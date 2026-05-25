1. 创建和进入部署目录
```shell
mkdir epusdt && cd epusdt
```

2. 创建配置和运行时目录
```shell
mkdir -p conf runtime
```

3. 可选：先准备自定义 PostgreSQL 账号信息
```shell
export EPUSDT_POSTGRES_USER=postgres
export EPUSDT_POSTGRES_PASSWORD='546957876Qq'
export EPUSDT_POSTGRES_DB=gmpay
```

4. 如果你想跳过安装向导，手动把配置文件放到 `conf/.env`

app_url

tron_grid_api_key

api_rate_url

```shell
cat <<EOF > conf/.env
app_name=epusdt
app_uri=https://dujiaoka.com
log_level=info
http_access_log=false
sql_debug=false
http_listen=0.0.0.0:8000

static_path=/static
runtime_root_path=/app/runtime

log_save_path=./logs
log_max_size=32
log_max_age=7
max_backups=3

# supported values: postgres,mysql,sqlite
db_type=postgres

# sqlite primary database config
sqlite_database_filename=
sqlite_table_prefix=

# postgres config
postgres_host=postgres
postgres_port=5432
postgres_user=${EPUSDT_POSTGRES_USER}
postgres_passwd=${EPUSDT_POSTGRES_PASSWORD}
postgres_database=${EPUSDT_POSTGRES_DB}
postgres_table_prefix=
postgres_max_idle_conns=10
postgres_max_open_conns=100
postgres_max_life_time=6

# mysql config
mysql_host=127.0.0.1
mysql_port=3306
mysql_user=mysql_user
mysql_passwd=mysql_password
mysql_database=database_name
mysql_table_prefix=
mysql_max_idle_conns=10
mysql_max_open_conns=100
mysql_max_life_time=6

# sqlite runtime store config
runtime_sqlite_filename=epusdt-runtime.db

# background scheduler config
queue_concurrency=10
queue_poll_interval_ms=1000
callback_retry_base_seconds=5

tg_bot_token=
tg_proxy=
tg_manage=

api_auth_token=

order_expiration_time=10
order_notice_max_retry=0
api_rate_url=https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api@latest/v1/currencies/
tron_grid_api_key=
EOF
```
5. docker compose 创建
```shell
cat <<EOF > docker-compose.yaml
services:
  postgres:
    image: postgres:16-alpine
    restart: always
    environment:
      POSTGRES_USER: \${EPUSDT_POSTGRES_USER:-postgres}
      POSTGRES_PASSWORD: \${EPUSDT_POSTGRES_PASSWORD:-546957876Qq}
      POSTGRES_DB: \${EPUSDT_POSTGRES_DB:-gmpay}
    volumes:
      - postgres_data:/var/lib/postgresql/data
    healthcheck:
      test: ["CMD-SHELL", "pg_isready -U \${EPUSDT_POSTGRES_USER:-postgres} -d \${EPUSDT_POSTGRES_DB:-gmpay}"]
      interval: 10s
      timeout: 5s
      retries: 10

  epusdt:
    image: gmwallet/epusdt:latest
    restart: always
    build:
      context: .
      dockerfile: Dockerfile
    depends_on:
      postgres:
        condition: service_healthy
    environment:
      EPUSDT_CONFIG: /app/conf/.env
      EPUSDT_DOCKER: "1"
      EPUSDT_POSTGRES_HOST: postgres
      EPUSDT_POSTGRES_PORT: "5432"
      EPUSDT_POSTGRES_USER: \${EPUSDT_POSTGRES_USER:-postgres}
      EPUSDT_POSTGRES_PASSWORD: \${EPUSDT_POSTGRES_PASSWORD:-546957876Qq}
      EPUSDT_POSTGRES_DB: \${EPUSDT_POSTGRES_DB:-gmpay}
    volumes:
      - ./conf:/app/conf
      - ./runtime:/app/runtime
    ports:
      - "8000:8000"

volumes:
  postgres_data:
EOF
```
6. 运行
```shell
docker compose up -d
```
7. 如果你走安装向导，数据库相关这样填

- 数据库类型：`PostgreSQL`
- `postgres_host`：`postgres`
- `postgres_port`：`5432`
- 用户名 / 密码 / 数据库名：和 Compose 里的变量保持一致

⚠️ **Docker 环境下不要填写 `127.0.0.1` 或 `localhost`**

因为在容器里：

- `127.0.0.1` 指向的是 `epusdt` 容器自己
- 不是 PostgreSQL 容器

所以 Docker Compose 内置 PostgreSQL 时，主机名必须填写：

```text
postgres
```

8. 配置独角兽后台

商户密钥： http://your_domain/payments/epusdt/v1/order/create-transaction
