// Package install provides a first-run setup REST API.
//
// When the .env config file is absent (or has install=true) the HTTP start
// command calls RunInstallServer, which listens on the same address the main
// server will eventually use (default :8000) and mounts two JSON endpoints
// under /install consumed by the frontend install UI:
//
//	GET  /install/defaults  — default field values for the form
//	POST /install           — validate + write .env, then shut down
//
// The HTTP listen address is submitted as two separate fields (http_bind_addr
// and http_bind_port) and combined internally as "ADDR:PORT" before writing
// the http_listen key in .env.  This makes the form easier for users who only
// want to change the port without touching the bind address.
//
// Once the .env is written the install server stops and normal bootstrap
// proceeds on the same port without a restart.
package install

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"
	"time"

	luluHttp "github.com/GMWalletApp/epusdt/util/http"
	"github.com/gookit/color"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DefaultInstallAddr is the listen address used by the install API.
// Matches the default http_listen so no extra port is needed.
const DefaultInstallAddr = ":8000"

// InstallRequest is the payload submitted by the install form.
// All fields are optional except AppURI; omitted fields fall back to InstallDefaults().
type InstallRequest struct {
	// Application display name (default: epusdt)
	AppName string `json:"app_name" form:"app_name" example:"epusdt"`
	// Public base URL of the service, e.g. https://pay.example.com (required)
	AppURI string `json:"app_uri" form:"app_uri" example:"https://pay.example.com"`
	// Initial admin username created on first bootstrap (default: admin)
	InitialAdminUsername string `json:"initial_admin_username" form:"initial_admin_username" example:"admin"`
	// Initial admin password created on first bootstrap; when omitted a random password is generated
	InitialAdminPassword string `json:"initial_admin_password" form:"initial_admin_password" example:"ChangeMe123!"`
	// When true, create the primary database automatically if it does not exist.
	CreateDatabaseIfMissing bool `json:"create_database_if_missing" form:"create_database_if_missing"`
	// Primary business database type: sqlite, mysql, postgres
	DBType string `json:"db_type" form:"db_type" example:"sqlite"`
	// Optional custom SQLite filename for the primary database
	SQLiteDatabaseFilename string `json:"sqlite_database_filename" form:"sqlite_database_filename" example:"epusdt.db"`
	// Optional table prefix for the SQLite primary database
	SQLiteTablePrefix string `json:"sqlite_table_prefix" form:"sqlite_table_prefix" example:""`
	// MySQL primary database connection fields
	MySQLHost        string `json:"mysql_host" form:"mysql_host" example:"127.0.0.1"`
	MySQLPort        string `json:"mysql_port" form:"mysql_port" example:"3306"`
	MySQLUser        string `json:"mysql_user" form:"mysql_user" example:"epusdt"`
	MySQLPasswd      string `json:"mysql_passwd" form:"mysql_passwd" example:"secret"`
	MySQLDatabase    string `json:"mysql_database" form:"mysql_database" example:"epusdt"`
	MySQLTablePrefix string `json:"mysql_table_prefix" form:"mysql_table_prefix" example:""`
	// PostgreSQL primary database connection fields
	PostgresHost        string `json:"postgres_host" form:"postgres_host" example:"127.0.0.1"`
	PostgresPort        string `json:"postgres_port" form:"postgres_port" example:"5432"`
	PostgresUser        string `json:"postgres_user" form:"postgres_user" example:"epusdt"`
	PostgresPasswd      string `json:"postgres_passwd" form:"postgres_passwd" example:"secret"`
	PostgresDatabase    string `json:"postgres_database" form:"postgres_database" example:"epusdt"`
	PostgresTablePrefix string `json:"postgres_table_prefix" form:"postgres_table_prefix" example:""`
	// Bind address for the HTTP server (default: 127.0.0.1)
	HttpBindAddr string `json:"http_bind_addr" form:"http_bind_addr" example:"127.0.0.1"`
	// Bind port for the HTTP server (default: 8000)
	HttpBindPort int `json:"http_bind_port" form:"http_bind_port" example:"8000"`
	// Runtime directory for SQLite DB and temp files (default: ./runtime)
	RuntimeRootPath string `json:"runtime_root_path" form:"runtime_root_path" example:"./runtime"`
	// Directory for application log files (default: ./logs)
	LogSavePath string `json:"log_save_path" form:"log_save_path" example:"./logs"`
	// Minutes before an unpaid order expires (default: 10)
	OrderExpirationTime int `json:"order_expiration_time" form:"order_expiration_time" example:"10"`
	// Maximum webhook retry attempts (default: 1)
	OrderNoticeMaxRetry int `json:"order_notice_max_retry" form:"order_notice_max_retry" example:"1"`
}

// InstallDefaults returns sensible default values for the install form.
func InstallDefaults() InstallRequest {
	defaults := InstallRequest{
		AppName:                "epusdt",
		AppURI:                 "",
		InitialAdminUsername:   "admin",
		InitialAdminPassword:   "",
		DBType:                 "sqlite",
		SQLiteDatabaseFilename: "",
		SQLiteTablePrefix:      "",
		MySQLHost:              "127.0.0.1",
		MySQLPort:              "3306",
		MySQLUser:              "gmpay",
		MySQLPasswd:            "",
		MySQLDatabase:          "gmpay",
		MySQLTablePrefix:       "",
		PostgresHost:           "127.0.0.1",
		PostgresPort:           "5432",
		PostgresUser:           "postgres",
		PostgresPasswd:         "",
		PostgresDatabase:       "gmpay",
		PostgresTablePrefix:    "",
		HttpBindAddr:           "127.0.0.1",
		HttpBindPort:           8000,
		RuntimeRootPath:        "./runtime",
		LogSavePath:            "./logs",
		OrderExpirationTime:    10,
		OrderNoticeMaxRetry:    1,
	}
	if strings.TrimSpace(os.Getenv("EPUSDT_DOCKER")) != "" {
		defaults.DBType = "postgres"
		defaults.HttpBindAddr = "0.0.0.0"
		defaults.RuntimeRootPath = "/app/runtime"
		defaults.LogSavePath = "./logs"
		defaults.PostgresHost = envOrDefault("EPUSDT_POSTGRES_HOST", "postgres")
		defaults.PostgresPort = envOrDefault("EPUSDT_POSTGRES_PORT", "5432")
		defaults.PostgresUser = envOrDefault("EPUSDT_POSTGRES_USER", "postgres")
		defaults.PostgresPasswd = envOrDefault("EPUSDT_POSTGRES_PASSWORD", "546957876Qq")
		defaults.PostgresDatabase = envOrDefault("EPUSDT_POSTGRES_DB", "gmpay")
	}
	return defaults
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

// installHandler holds the per-invocation state shared between handlers.
type installHandler struct {
	envFilePath string
	done        chan struct{}
}

func installOnlyRouteMiddleware(wwwRoot string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			if req.Method != http.MethodGet && req.Method != http.MethodHead {
				return next(c)
			}

			path := req.URL.Path
			if path == "/" {
				return c.Redirect(http.StatusFound, "/install")
			}
			if path == "/install" || strings.HasPrefix(path, "/install/") {
				return next(c)
			}
			if luluHttp.ShouldSkipSPAFallback(path) {
				return next(c)
			}
			if isInstallStaticAssetRequest(wwwRoot, path) {
				return next(c)
			}

			return c.Redirect(http.StatusFound, "/install")
		}
	}
}

func isInstallStaticAssetRequest(wwwRoot, requestPath string) bool {
	resolvedPath, tryStat := luluHttp.ResolveSPAFilePath(wwwRoot, requestPath)
	if !tryStat {
		return false
	}

	info, err := os.Stat(resolvedPath)
	if err != nil {
		return false
	}

	return !info.IsDir()
}

func resolveInstallWWWRoot() string {
	// Resolve www/ relative to the executable so SPA routes work regardless
	// of the working directory. main.go extracts www/ next to the binary.
	wwwRoot := "./www"
	if exePath, err := os.Executable(); err == nil {
		if exePath, err = filepath.EvalSymlinks(exePath); err == nil {
			wwwRoot = filepath.Join(filepath.Dir(exePath), "www")
		}
	}
	return wwwRoot
}

func newInstallServer(envFilePath, wwwRoot string) (*echo.Echo, *installHandler) {
	h := &installHandler{
		envFilePath: envFilePath,
		done:        make(chan struct{}),
	}

	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	// api routes for the install frontend
	api := e.Group("/api")

	api.GET("/install/defaults", h.GetDefaults)
	api.POST("/install", h.Submit)
	api.POST("/install/test-db", h.TestDBConnection)
	api.POST("/install/ensure-db", h.EnsureDatabase)

	// While the install server is running, only /install, install APIs, and
	// real static assets should be reachable. Any other browser route must
	// be redirected back to /install so users cannot enter the login/admin
	// SPA before initialisation completes.
	e.Use(installOnlyRouteMiddleware(wwwRoot))

	e.Use(middleware.StaticWithConfig(middleware.StaticConfig{
		Skipper: func(c echo.Context) bool {
			return luluHttp.ShouldSkipSPAFallback(c.Request().URL.Path)
		},
		HTML5: true,
		Index: "index.html",
		Root:  wwwRoot,
	}))

	return e, h
}

// GetDefaults returns default values for the install form.
//
// @Summary      Install — get default values
// @Description  Returns sensible default field values for the first-run install form.
//
//	Available only before the .env config file has been written.
//	After installation completes this route is no longer served.
//
// @Tags         Install
// @Produce      json
// @Success      200 {object} InstallRequest "Default install values"
// @Router       /api/install/defaults [get]
func (h *installHandler) GetDefaults(c echo.Context) error {
	return c.JSON(http.StatusOK, InstallDefaults())
}

// TestDBConnection validates the submitted database configuration and probes
// the primary database connection without persisting any install state.
func (h *installHandler) TestDBConnection(c echo.Context) error {
	req := new(InstallRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
	}
	if err := normalizeInstallRequest(req, false); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
	}
	if err := preparePrimaryDatabase(req, filepath.Dir(h.envFilePath), false); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"message": "database connection ok"})
}

// EnsureDatabase creates the configured primary database when supported and
// missing, then verifies the connection.
func (h *installHandler) EnsureDatabase(c echo.Context) error {
	req := new(InstallRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
	}
	if err := normalizeInstallRequest(req, false); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
	}
	if err := preparePrimaryDatabase(req, filepath.Dir(h.envFilePath), req.CreateDatabaseIfMissing); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
	}
	return c.JSON(http.StatusOK, map[string]interface{}{"message": "database is ready"})
}

// Submit validates the install payload, writes the .env file, and signals
// the install server to shut down so the main bootstrap can proceed.
// http_bind_addr and http_bind_port are combined as "ADDR:PORT" to produce
// the http_listen config key (e.g. 0.0.0.0:8000).
//
// @Summary      Install — submit configuration
// @Description  Validates the submitted configuration and writes the .env file.
//
//	http_bind_addr + http_bind_port are joined internally as "ADDR:PORT" for
//	the http_listen config key (defaults: 127.0.0.1 and 8000 respectively).
//	http_bind_port must be in the range 1–65535 if provided.
//	app_uri is required. All other fields are optional and fall back to
//	the defaults returned by GET /api/install/defaults.
//	Sets install=false in the written .env, then shuts down the install
//	server so that normal application bootstrap starts on the same port.
//	After installation completes this route is no longer served.
//
// @Tags         Install
// @Accept       json
// @Produce      json
// @Param        body body     InstallRequest true "Install configuration"
// @Success      200  {object} map[string]string "message"
// @Failure      400  {object} map[string]string "error"
// @Failure      500  {object} map[string]string "error"
// @Router       /api/install [post]
func (h *installHandler) Submit(c echo.Context) error {
	req := new(InstallRequest)
	if err := c.Bind(req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
	}
	if err := normalizeInstallRequest(req, true); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
	}
	if err := preparePrimaryDatabase(req, filepath.Dir(h.envFilePath), true); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]interface{}{"error": err.Error()})
	}

	if err := writeEnvFile(h.envFilePath, req); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
	}
	if err := setInstallFlagAtPath(h.envFilePath, false); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{"error": err.Error()})
	}

	// Give the HTTP response a brief moment to flush before the install server
	// is asked to shut down; otherwise some clients observe a 502/connection
	// reset during the handoff to the real app server.
	go func() {
		time.Sleep(250 * time.Millisecond)
		close(h.done)
	}()
	return c.JSON(http.StatusOK, map[string]interface{}{"message": "install complete, starting server…"})
}

func normalizeInstallRequest(req *InstallRequest, requireAppURI bool) error {
	d := InstallDefaults()
	req.AppURI = strings.TrimSpace(req.AppURI)
	if requireAppURI && req.AppURI == "" {
		return fmt.Errorf("应用地址不能为空")
	}
	req.InitialAdminUsername = strings.ToLower(strings.TrimSpace(req.InitialAdminUsername))
	if req.InitialAdminUsername == "" {
		req.InitialAdminUsername = d.InitialAdminUsername
	}
	if len(req.InitialAdminUsername) < 3 {
		return fmt.Errorf("初始管理员账号至少需要 3 个字符")
	}
	if strings.ContainsAny(req.InitialAdminUsername, " \t\r\n") {
		return fmt.Errorf("初始管理员账号不能包含空格")
	}
	req.InitialAdminPassword = strings.TrimSpace(req.InitialAdminPassword)
	if req.InitialAdminPassword != "" && len(req.InitialAdminPassword) < 6 {
		return fmt.Errorf("初始管理员密码至少需要 6 个字符")
	}
	req.DBType = strings.ToLower(strings.TrimSpace(req.DBType))
	if req.DBType == "" {
		req.DBType = d.DBType
	}
	switch req.DBType {
	case "sqlite":
		req.SQLiteDatabaseFilename = strings.TrimSpace(req.SQLiteDatabaseFilename)
		req.SQLiteTablePrefix = strings.TrimSpace(req.SQLiteTablePrefix)
	case "mysql":
		req.MySQLHost = strings.TrimSpace(req.MySQLHost)
		req.MySQLPort = strings.TrimSpace(req.MySQLPort)
		req.MySQLUser = strings.TrimSpace(req.MySQLUser)
		req.MySQLPasswd = strings.TrimSpace(req.MySQLPasswd)
		req.MySQLDatabase = strings.TrimSpace(req.MySQLDatabase)
		req.MySQLTablePrefix = strings.TrimSpace(req.MySQLTablePrefix)
		if isDockerLocalDatabaseHost(req.MySQLHost) {
			return fmt.Errorf("Docker 部署下 MySQL 地址不能填写 127.0.0.1 / localhost，请填写可访问的 MySQL 主机名")
		}
		if req.MySQLHost == "" || req.MySQLPort == "" || req.MySQLUser == "" || req.MySQLDatabase == "" {
			return fmt.Errorf("选择 MySQL 时，地址、端口、用户名、数据库名不能为空")
		}
	case "postgres":
		req.PostgresHost = strings.TrimSpace(req.PostgresHost)
		req.PostgresPort = strings.TrimSpace(req.PostgresPort)
		req.PostgresUser = strings.TrimSpace(req.PostgresUser)
		req.PostgresPasswd = strings.TrimSpace(req.PostgresPasswd)
		req.PostgresDatabase = strings.TrimSpace(req.PostgresDatabase)
		req.PostgresTablePrefix = strings.TrimSpace(req.PostgresTablePrefix)
		if isDockerLocalDatabaseHost(req.PostgresHost) {
			return fmt.Errorf("Docker 部署下 PostgreSQL 地址不能填写 127.0.0.1 / localhost，请填写 Compose 服务名 postgres")
		}
		if req.PostgresHost == "" || req.PostgresPort == "" || req.PostgresUser == "" || req.PostgresDatabase == "" {
			return fmt.Errorf("选择 PostgreSQL 时，地址、端口、用户名、数据库名不能为空")
		}
	default:
		return fmt.Errorf("数据库类型必须是 sqlite、mysql 或 postgres")
	}
	if req.HttpBindPort != 0 && (req.HttpBindPort < 1 || req.HttpBindPort > 65535) {
		return fmt.Errorf("端口必须在 1 到 65535 之间")
	}
	if strings.TrimSpace(req.AppName) == "" {
		req.AppName = d.AppName
	}
	if strings.TrimSpace(req.HttpBindAddr) == "" {
		req.HttpBindAddr = d.HttpBindAddr
	}
	if req.HttpBindPort <= 0 {
		req.HttpBindPort = d.HttpBindPort
	}
	if strings.TrimSpace(req.RuntimeRootPath) == "" {
		req.RuntimeRootPath = d.RuntimeRootPath
	}
	if strings.TrimSpace(req.LogSavePath) == "" {
		req.LogSavePath = d.LogSavePath
	}
	if req.OrderExpirationTime <= 0 {
		req.OrderExpirationTime = d.OrderExpirationTime
	}
	if req.OrderNoticeMaxRetry < 0 {
		req.OrderNoticeMaxRetry = d.OrderNoticeMaxRetry
	}
	return nil
}

func isDockerLocalDatabaseHost(host string) bool {
	if strings.TrimSpace(os.Getenv("EPUSDT_DOCKER")) == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(host)) {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}

func preparePrimaryDatabase(req *InstallRequest, configDir string, createIfMissing bool) error {
	switch req.DBType {
	case "sqlite":
		sqlitePath := resolvePrimarySQLitePath(configDir, req.SQLiteDatabaseFilename)
		if err := os.MkdirAll(filepath.Dir(sqlitePath), 0o755); err != nil {
			return fmt.Errorf("SQLite 目录不可用：%w", err)
		}
		info, statErr := os.Stat(sqlitePath)
		created := false
		if statErr != nil && !os.IsNotExist(statErr) {
			return fmt.Errorf("SQLite 文件路径无效：%w", statErr)
		}
		f, err := os.OpenFile(sqlitePath, os.O_RDWR|os.O_CREATE, 0o600)
		if err != nil {
			return fmt.Errorf("SQLite 文件不可用：%w", err)
		}
		_ = f.Close()
		if os.IsNotExist(statErr) {
			created = true
		} else if info == nil {
			created = true
		}
		if created {
			_ = os.Remove(sqlitePath)
		}
		return nil
	case "mysql":
		if createIfMissing {
			if err := ensureMySQLDatabaseExists(req); err != nil {
				return err
			}
		}
		return pingMySQLDatabase(req)
	case "postgres":
		if createIfMissing {
			if err := ensurePostgresDatabaseExists(req); err != nil {
				return err
			}
		}
		return pingPostgresDatabase(req)
	default:
		return fmt.Errorf("数据库类型必须是 sqlite、mysql 或 postgres")
	}
}

func resolvePrimarySQLitePath(configDir, filename string) string {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return filepath.Join(configDir, "epusdt.db")
	}
	if filepath.IsAbs(filename) {
		return filename
	}
	filename = strings.TrimPrefix(strings.TrimPrefix(filename, "/"), "\\")
	return filepath.Join(configDir, filepath.FromSlash(filename))
}

var safeDatabaseIdentifier = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func validateDatabaseIdentifier(name string) error {
	if !safeDatabaseIdentifier.MatchString(name) {
		return fmt.Errorf("数据库名 %q 含有不支持的字符", name)
	}
	return nil
}

func pingMySQLDatabase(req *InstallRequest) error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		req.MySQLUser,
		req.MySQLPasswd,
		req.MySQLHost,
		req.MySQLPort,
		req.MySQLDatabase,
	)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("MySQL 连接失败：%w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("MySQL 连接句柄获取失败：%w", err)
	}
	defer sqlDB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("MySQL 连通性检测失败：%w", err)
	}
	return nil
}

func ensureMySQLDatabaseExists(req *InstallRequest) error {
	if err := validateDatabaseIdentifier(req.MySQLDatabase); err != nil {
		return err
	}
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/?charset=utf8mb4&parseTime=True&loc=Local",
		req.MySQLUser,
		req.MySQLPasswd,
		req.MySQLHost,
		req.MySQLPort,
	)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("MySQL 连接失败：%w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("MySQL 连接句柄获取失败：%w", err)
	}
	defer sqlDB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("MySQL 连通性检测失败：%w", err)
	}
	if err := db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`", req.MySQLDatabase)).Error; err != nil {
		return fmt.Errorf("MySQL 自动创建数据库失败：%w", err)
	}
	return nil
}

func pingPostgresDatabase(req *InstallRequest) error {
	dsn := fmt.Sprintf("user=%s password=%s host=%s port=%s dbname=%s sslmode=disable TimeZone=Asia/Shanghai",
		req.PostgresUser,
		req.PostgresPasswd,
		req.PostgresHost,
		req.PostgresPort,
		req.PostgresDatabase,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("PostgreSQL 连接失败：%w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("PostgreSQL 连接句柄获取失败：%w", err)
	}
	defer sqlDB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("PostgreSQL 连通性检测失败：%w", err)
	}
	return nil
}

func ensurePostgresDatabaseExists(req *InstallRequest) error {
	if err := validateDatabaseIdentifier(req.PostgresDatabase); err != nil {
		return err
	}
	dsn := fmt.Sprintf("user=%s password=%s host=%s port=%s dbname=postgres sslmode=disable TimeZone=Asia/Shanghai",
		req.PostgresUser,
		req.PostgresPasswd,
		req.PostgresHost,
		req.PostgresPort,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		return fmt.Errorf("PostgreSQL 默认库连接失败：%w", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("PostgreSQL 连接句柄获取失败：%w", err)
	}
	defer sqlDB.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("PostgreSQL 连通性检测失败：%w", err)
	}
	var exists bool
	row := sqlDB.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", req.PostgresDatabase)
	if err := row.Scan(&exists); err != nil {
		return fmt.Errorf("PostgreSQL 检查数据库是否存在失败：%w", err)
	}
	if exists {
		return nil
	}
	if err := db.Exec(fmt.Sprintf("CREATE DATABASE \"%s\"", req.PostgresDatabase)).Error; err != nil {
		return fmt.Errorf("PostgreSQL 自动创建数据库失败：%w", err)
	}
	return nil
}

// RunInstallServer starts the install REST API on listenAddr (default :8000)
// under the /install path and blocks until the .env file has been written.
// The caller should then proceed with normal app initialisation (bootstrap.InitApp).
func RunInstallServer(listenAddr, envFilePath string) {
	if listenAddr == "" {
		listenAddr = DefaultInstallAddr
	}

	e, h := newInstallServer(envFilePath, resolveInstallWWWRoot())

	// Build a human-readable URL for the console hint.
	installHost := listenAddr
	if strings.HasPrefix(installHost, ":") {
		installHost = "localhost" + installHost
	}
	color.Green.Printf("[install] no config found — install API available at http://%s/install\n", installHost)

	go func() {
		<-h.done
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = e.Shutdown(ctx)
	}()

	if err := e.Start(listenAddr); err != nil && err != http.ErrServerClosed {
		color.Red.Printf("[install] server error: %s\n", err)
		os.Exit(1)
	}

	color.Green.Printf("[install] configuration saved to %s, starting…\n", envFilePath)
}

// formControlledKeys are keys whose values always come from the install form
// (or are set unconditionally by the template). Existing config values for
// these keys must NOT be preserved — the form submission takes precedence.
var formControlledKeys = map[string]bool{
	"app_name":                 true,
	"app_uri":                  true,
	"initial_admin_username":   true,
	"initial_admin_password":   true,
	"db_type":                  true,
	"sqlite_database_filename": true,
	"sqlite_table_prefix":      true,
	"mysql_host":               true,
	"mysql_port":               true,
	"mysql_user":               true,
	"mysql_passwd":             true,
	"mysql_database":           true,
	"mysql_table_prefix":       true,
	"postgres_host":            true,
	"postgres_port":            true,
	"postgres_user":            true,
	"postgres_passwd":          true,
	"postgres_database":        true,
	"postgres_table_prefix":    true,
	"http_listen":              true,
	"runtime_root_path":        true,
	"log_save_path":            true,
	"order_expiration_time":    true,
	"order_notice_max_retry":   true,
	"install":                  true,
}

var preserveBlankFormKeys = map[string]bool{
	"mysql_passwd":    true,
	"postgres_passwd": true,
}

// writeEnvFile renders and writes a minimal .env file.
// If the file already exists, values for keys that are NOT controlled by the
// install form are preserved from the existing file so that operator-specific
// settings (tg_bot_token, db_type, etc.) survive a re-install.
// Keys that the form controls (app_uri, http_listen, …) always use the
// submitted values.
func writeEnvFile(path string, r *InstallRequest) error {
	// Collect existing non-empty key→value pairs for non-form keys.
	existingValues := map[string]string{}
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			if idx := strings.IndexByte(line, '='); idx >= 0 {
				k := strings.TrimSpace(line[:idx])
				v := strings.TrimSpace(line[idx+1:])
				if v != "" {
					existingValues[k] = v
				}
			}
		}
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}

	// Render the template into a buffer first.
	var buf bytes.Buffer
	if err := envTemplate.Execute(&buf, r); err != nil {
		return fmt.Errorf("render env template: %w", err)
	}

	// For non-form keys that already had a value, substitute the existing value
	// so the template default does not clobber operator configuration.
	lines := strings.Split(buf.String(), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if idx := strings.IndexByte(trimmed, '='); idx >= 0 {
			k := strings.TrimSpace(trimmed[:idx])
			renderedValue := strings.TrimSpace(trimmed[idx+1:])
			if existing, ok := existingValues[k]; ok && (!formControlledKeys[k] || (preserveBlankFormKeys[k] && renderedValue == "")) {
				lines[i] = k + "=" + existing
			}
		}
	}

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("open config file: %w", err)
	}
	defer f.Close()
	_, err = fmt.Fprint(f, strings.Join(lines, "\n"))
	return err
}

func setInstallFlagAtPath(path string, enabled bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	targetValue := "false"
	if enabled {
		targetValue = "true"
	}

	lines := strings.Split(string(data), "\n")
	found := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if !strings.HasPrefix(trimmed, "install=") {
			continue
		}
		lines[i] = "install=" + targetValue
		found = true
	}
	if !found {
		lines = append(lines, "install="+targetValue)
	}

	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600)
}

var envTemplate = template.Must(template.New("env").Parse(`app_name={{.AppName}}
app_uri={{.AppURI}}
initial_admin_username={{.InitialAdminUsername}}
initial_admin_password={{.InitialAdminPassword}}
log_level=info
http_access_log=false
sql_debug=false
http_listen={{.HttpBindAddr}}:{{.HttpBindPort}}

static_path=/static
runtime_root_path={{.RuntimeRootPath}}

log_save_path={{.LogSavePath}}
log_max_size=32
log_max_age=7
max_backups=3

# supported values: postgres,mysql,sqlite
db_type={{.DBType}}

# sqlite primary database config
sqlite_database_filename={{.SQLiteDatabaseFilename}}
sqlite_table_prefix={{.SQLiteTablePrefix}}

# postgres config
postgres_host={{.PostgresHost}}
postgres_port={{.PostgresPort}}
postgres_user={{.PostgresUser}}
postgres_passwd={{.PostgresPasswd}}
postgres_database={{.PostgresDatabase}}
postgres_table_prefix={{.PostgresTablePrefix}}
postgres_max_idle_conns=10
postgres_max_open_conns=100
postgres_max_life_time=6

# mysql config
mysql_host={{.MySQLHost}}
mysql_port={{.MySQLPort}}
mysql_user={{.MySQLUser}}
mysql_passwd={{.MySQLPasswd}}
mysql_database={{.MySQLDatabase}}
mysql_table_prefix={{.MySQLTablePrefix}}
mysql_max_idle_conns=10
mysql_max_open_conns=100
mysql_max_life_time=6

# sqlite runtime store config
runtime_sqlite_filename=epusdt-runtime.db

# background scheduler config
queue_concurrency=10
queue_poll_interval_ms=1000
callback_retry_base_seconds=5

order_expiration_time={{.OrderExpirationTime}}
order_notice_max_retry={{.OrderNoticeMaxRetry}}

api_rate_url=

# Set to true to re-run the install wizard on next startup.
install=false
`))
