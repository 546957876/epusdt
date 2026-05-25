package install

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/labstack/echo/v4"
)

func TestInstallDefaults(t *testing.T) {
	d := InstallDefaults()
	if d.AppName != "epusdt" {
		t.Errorf("AppName = %q, want epusdt", d.AppName)
	}
	if d.HttpBindAddr != "127.0.0.1" {
		t.Errorf("HttpBindAddr = %q, want 127.0.0.1", d.HttpBindAddr)
	}
	if d.HttpBindPort != 8000 {
		t.Errorf("HttpBindPort = %d, want 8000", d.HttpBindPort)
	}
	if d.OrderExpirationTime != 10 {
		t.Errorf("OrderExpirationTime = %d, want 10", d.OrderExpirationTime)
	}
}

func TestInstallDefaultsDockerOverrides(t *testing.T) {
	oldDocker := os.Getenv("EPUSDT_DOCKER")
	oldHost := os.Getenv("EPUSDT_POSTGRES_HOST")
	oldPort := os.Getenv("EPUSDT_POSTGRES_PORT")
	oldUser := os.Getenv("EPUSDT_POSTGRES_USER")
	oldPass := os.Getenv("EPUSDT_POSTGRES_PASSWORD")
	oldDB := os.Getenv("EPUSDT_POSTGRES_DB")
	defer func() {
		if oldDocker == "" {
			_ = os.Unsetenv("EPUSDT_DOCKER")
		} else {
			_ = os.Setenv("EPUSDT_DOCKER", oldDocker)
		}
		if oldHost == "" {
			_ = os.Unsetenv("EPUSDT_POSTGRES_HOST")
		} else {
			_ = os.Setenv("EPUSDT_POSTGRES_HOST", oldHost)
		}
		if oldPort == "" {
			_ = os.Unsetenv("EPUSDT_POSTGRES_PORT")
		} else {
			_ = os.Setenv("EPUSDT_POSTGRES_PORT", oldPort)
		}
		if oldUser == "" {
			_ = os.Unsetenv("EPUSDT_POSTGRES_USER")
		} else {
			_ = os.Setenv("EPUSDT_POSTGRES_USER", oldUser)
		}
		if oldPass == "" {
			_ = os.Unsetenv("EPUSDT_POSTGRES_PASSWORD")
		} else {
			_ = os.Setenv("EPUSDT_POSTGRES_PASSWORD", oldPass)
		}
		if oldDB == "" {
			_ = os.Unsetenv("EPUSDT_POSTGRES_DB")
		} else {
			_ = os.Setenv("EPUSDT_POSTGRES_DB", oldDB)
		}
	}()
	if err := os.Setenv("EPUSDT_DOCKER", "1"); err != nil {
		t.Fatalf("set EPUSDT_DOCKER: %v", err)
	}
	_ = os.Setenv("EPUSDT_POSTGRES_HOST", "postgres")
	_ = os.Setenv("EPUSDT_POSTGRES_PORT", "5432")
	_ = os.Setenv("EPUSDT_POSTGRES_USER", "postgres")
	_ = os.Setenv("EPUSDT_POSTGRES_PASSWORD", "546957876Qq")
	_ = os.Setenv("EPUSDT_POSTGRES_DB", "gmpay")

	d := InstallDefaults()
	if d.DBType != "postgres" {
		t.Errorf("DBType = %q, want postgres", d.DBType)
	}
	if d.HttpBindAddr != "0.0.0.0" {
		t.Errorf("HttpBindAddr = %q, want 0.0.0.0", d.HttpBindAddr)
	}
	if d.RuntimeRootPath != "/app/runtime" {
		t.Errorf("RuntimeRootPath = %q, want /app/runtime", d.RuntimeRootPath)
	}
	if d.LogSavePath != "./logs" {
		t.Errorf("LogSavePath = %q, want ./logs", d.LogSavePath)
	}
	if d.PostgresHost != "postgres" {
		t.Errorf("PostgresHost = %q, want postgres", d.PostgresHost)
	}
	if d.PostgresPort != "5432" {
		t.Errorf("PostgresPort = %q, want 5432", d.PostgresPort)
	}
	if d.PostgresUser != "postgres" {
		t.Errorf("PostgresUser = %q, want postgres", d.PostgresUser)
	}
	if d.PostgresPasswd != "546957876Qq" {
		t.Errorf("PostgresPasswd = %q, want 546957876Qq", d.PostgresPasswd)
	}
	if d.PostgresDatabase != "gmpay" {
		t.Errorf("PostgresDatabase = %q, want gmpay", d.PostgresDatabase)
	}
}

func TestWriteEnvFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".env")

	req := &InstallRequest{
		AppName:              "myapp",
		AppURI:               "http://1.2.3.4:8000",
		InitialAdminUsername: "owner",
		InitialAdminPassword: "Secret123",
		DBType:               "mysql",
		MySQLHost:            "127.0.0.1",
		MySQLPort:            "3306",
		MySQLUser:            "epusdt",
		MySQLPasswd:          "mysql-secret",
		MySQLDatabase:        "epusdt",
		MySQLTablePrefix:     "ep_",
		HttpBindAddr:         "0.0.0.0",
		HttpBindPort:         9000,
		RuntimeRootPath:      "./runtime",
		LogSavePath:          "./logs",
		OrderExpirationTime:  15,
		OrderNoticeMaxRetry:  3,
	}
	if err := writeEnvFile(path, req); err != nil {
		t.Fatalf("writeEnvFile: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read env: %v", err)
	}
	content := string(data)

	for _, want := range []string{
		"app_name=myapp",
		"app_uri=http://1.2.3.4:8000",
		"initial_admin_username=owner",
		"initial_admin_password=Secret123",
		"db_type=mysql",
		"mysql_host=127.0.0.1",
		"mysql_port=3306",
		"mysql_user=epusdt",
		"mysql_passwd=mysql-secret",
		"mysql_database=epusdt",
		"mysql_table_prefix=ep_",
		"http_listen=0.0.0.0:9000",
		"order_expiration_time=15",
		"order_notice_max_retry=3",
		"db_type=mysql",
		"install=false",
	} {
		if !strings.Contains(content, want) {
			t.Errorf("env file missing %q\ncontent:\n%s", want, content)
		}
	}
}

func TestInstallAPIDefaults(t *testing.T) {
	h := &installHandler{done: make(chan struct{})}
	e := echo.New()
	e.GET("/install/defaults", h.GetDefaults)

	req := httptest.NewRequest(http.MethodGet, "/install/defaults", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["app_name"] != "epusdt" {
		t.Errorf("app_name = %v, want epusdt", body["app_name"])
	}
	if body["initial_admin_username"] != "admin" {
		t.Errorf("initial_admin_username = %v, want admin", body["initial_admin_username"])
	}
	if body["db_type"] != "sqlite" {
		t.Errorf("db_type = %v, want sqlite", body["db_type"])
	}
	if body["http_bind_addr"] != "127.0.0.1" {
		t.Errorf("http_bind_addr = %v, want 127.0.0.1", body["http_bind_addr"])
	}
	if body["http_bind_port"] != float64(8000) {
		t.Errorf("http_bind_port = %v, want 8000", body["http_bind_port"])
	}
}

func TestInstallServerRootRedirectsToInstall(t *testing.T) {
	dir := t.TempDir()
	wwwRoot := filepath.Join(dir, "www")
	if err := os.MkdirAll(wwwRoot, 0o755); err != nil {
		t.Fatalf("mkdir www root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wwwRoot, "index.html"), []byte("install-ui"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	e, _ := newInstallServer(filepath.Join(dir, ".env"), wwwRoot)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/install" {
		t.Fatalf("Location = %q, want /install", got)
	}
}

func TestInstallServerServesSPAOnInstallRoute(t *testing.T) {
	dir := t.TempDir()
	wwwRoot := filepath.Join(dir, "www")
	if err := os.MkdirAll(wwwRoot, 0o755); err != nil {
		t.Fatalf("mkdir www root: %v", err)
	}
	const wantBody = "install-ui"
	if err := os.WriteFile(filepath.Join(wwwRoot, "index.html"), []byte(wantBody), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	e, _ := newInstallServer(filepath.Join(dir, ".env"), wwwRoot)

	req := httptest.NewRequest(http.MethodGet, "/install", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != wantBody {
		t.Fatalf("body = %q, want %q", body, wantBody)
	}
}

func TestInstallServerRedirectsOtherSPARoutesToInstall(t *testing.T) {
	dir := t.TempDir()
	wwwRoot := filepath.Join(dir, "www")
	if err := os.MkdirAll(wwwRoot, 0o755); err != nil {
		t.Fatalf("mkdir www root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wwwRoot, "index.html"), []byte("install-ui"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}

	e, _ := newInstallServer(filepath.Join(dir, ".env"), wwwRoot)

	req := httptest.NewRequest(http.MethodGet, "/sign-in", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusFound {
		t.Fatalf("status = %d, want 302; body: %s", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Location"); got != "/install" {
		t.Fatalf("Location = %q, want /install", got)
	}
}

func TestInstallServerServesStaticAssetsWithoutRedirect(t *testing.T) {
	dir := t.TempDir()
	wwwRoot := filepath.Join(dir, "www")
	assetsRoot := filepath.Join(wwwRoot, "assets")
	if err := os.MkdirAll(assetsRoot, 0o755); err != nil {
		t.Fatalf("mkdir assets root: %v", err)
	}
	if err := os.WriteFile(filepath.Join(wwwRoot, "index.html"), []byte("install-ui"), 0o644); err != nil {
		t.Fatalf("write index.html: %v", err)
	}
	const wantBody = "console.log('install');"
	if err := os.WriteFile(filepath.Join(assetsRoot, "app.js"), []byte(wantBody), 0o644); err != nil {
		t.Fatalf("write app.js: %v", err)
	}

	e, _ := newInstallServer(filepath.Join(dir, ".env"), wwwRoot)

	req := httptest.NewRequest(http.MethodGet, "/assets/app.js", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	if body := rec.Body.String(); body != wantBody {
		t.Fatalf("body = %q, want %q", body, wantBody)
	}
}

func TestInstallAPISubmit(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	h := &installHandler{envFilePath: envPath, done: make(chan struct{})}
	e := echo.New()
	e.POST("/install", h.Submit)

	payload := `{"app_name":"testapp","app_uri":"http://10.0.0.1:8000","initial_admin_username":"owner","initial_admin_password":"Secret123","db_type":"sqlite","sqlite_database_filename":"epusdt-test.db","http_bind_addr":"0.0.0.0","http_bind_port":8000,"order_expiration_time":10,"order_notice_max_retry":1}`
	req := httptest.NewRequest(http.MethodPost, "/install", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
	// done channel should be closed after successful submit
	select {
	case <-h.done:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("handler did not close done channel within timeout")
	}
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("env file not written: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "app_uri=http://10.0.0.1:8000") {
		t.Errorf("env file missing app_uri; content:\n%s", content)
	}
	if !strings.Contains(content, "initial_admin_username=owner") {
		t.Errorf("env file missing initial_admin_username; content:\n%s", content)
	}
	if !strings.Contains(content, "initial_admin_password=Secret123") {
		t.Errorf("env file missing initial_admin_password; content:\n%s", content)
	}
	if !strings.Contains(content, "db_type=sqlite") {
		t.Errorf("env file missing db_type=sqlite; content:\n%s", content)
	}
	if !strings.Contains(content, "sqlite_database_filename=epusdt-test.db") {
		t.Errorf("env file missing sqlite_database_filename; content:\n%s", content)
	}
	if !strings.Contains(content, "http_listen=0.0.0.0:8000") {
		t.Errorf("env file missing http_listen; content:\n%s", content)
	}
}

func TestInstallAPISubmitInvalidInitialAdminPassword(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	h := &installHandler{envFilePath: envPath, done: make(chan struct{})}
	e := echo.New()
	e.POST("/install", h.Submit)

	payload := `{"app_uri":"http://example.com","initial_admin_password":"123"}`
	req := httptest.NewRequest(http.MethodPost, "/install", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

func TestInstallAPISubmitInvalidMySQLConfig(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	h := &installHandler{envFilePath: envPath, done: make(chan struct{})}
	e := echo.New()
	e.POST("/install", h.Submit)

	payload := `{"app_uri":"http://example.com","db_type":"mysql","mysql_host":"127.0.0.1"}`
	req := httptest.NewRequest(http.MethodPost, "/install", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
}

func TestNormalizeInstallRequestRejectsDockerLocalPostgresHost(t *testing.T) {
	oldDocker := os.Getenv("EPUSDT_DOCKER")
	defer func() {
		if oldDocker == "" {
			_ = os.Unsetenv("EPUSDT_DOCKER")
		} else {
			_ = os.Setenv("EPUSDT_DOCKER", oldDocker)
		}
	}()
	if err := os.Setenv("EPUSDT_DOCKER", "1"); err != nil {
		t.Fatalf("set EPUSDT_DOCKER: %v", err)
	}

	req := &InstallRequest{
		AppURI:           "http://example.com",
		DBType:           "postgres",
		InitialAdminUsername: "admin",
		InitialAdminPassword: "Secret123",
		PostgresHost:     "127.0.0.1",
		PostgresPort:     "5432",
		PostgresUser:     "postgres",
		PostgresDatabase: "gmpay",
	}

	err := normalizeInstallRequest(req, true, true)
	if err == nil {
		t.Fatal("expected docker-local postgres host validation error")
	}
	if !strings.Contains(err.Error(), "Compose 服务名 postgres") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNormalizeInstallRequestRequiresInitialAdminPasswordOnSubmit(t *testing.T) {
	req := &InstallRequest{
		AppURI:               "http://example.com",
		InitialAdminUsername: "admin",
		DBType:               "sqlite",
	}

	err := normalizeInstallRequest(req, true, true)
	if err == nil {
		t.Fatal("expected missing initial admin password validation error")
	}
	if !strings.Contains(err.Error(), "初始管理员密码不能为空") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInstallAPITestDBSQLite(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	h := &installHandler{envFilePath: envPath, done: make(chan struct{})}
	e := echo.New()
	e.POST("/install/test-db", h.TestDBConnection)

	payload := `{"db_type":"sqlite","sqlite_database_filename":"install-test.db"}`
	req := httptest.NewRequest(http.MethodPost, "/install/test-db", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestInstallAPIEnsureDBSQLite(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	h := &installHandler{envFilePath: envPath, done: make(chan struct{})}
	e := echo.New()
	e.POST("/install/ensure-db", h.EnsureDatabase)

	payload := `{"db_type":"sqlite","sqlite_database_filename":"install-test.db","create_database_if_missing":true}`
	req := httptest.NewRequest(http.MethodPost, "/install/ensure-db", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", rec.Code, rec.Body.String())
	}
}

func TestInstallAPISubmitMissingURI(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	h := &installHandler{envFilePath: envPath, done: make(chan struct{})}
	e := echo.New()
	e.POST("/install", h.Submit)

	req := httptest.NewRequest(http.MethodPost, "/install", strings.NewReader(`{"app_name":"x"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if _, err := os.Stat(envPath); err == nil {
		t.Error("env file should not have been written for invalid request")
	}
}

func TestInstallAPISubmitInvalidPort(t *testing.T) {
	dir := t.TempDir()
	envPath := filepath.Join(dir, ".env")

	h := &installHandler{envFilePath: envPath, done: make(chan struct{})}
	e := echo.New()
	e.POST("/install", h.Submit)

	payload := `{"app_uri":"http://example.com","http_bind_port":99999}`
	req := httptest.NewRequest(http.MethodPost, "/install", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body: %s", rec.Code, rec.Body.String())
	}
	if _, err := os.Stat(envPath); err == nil {
		t.Error("env file should not have been written for invalid port")
	}
}
