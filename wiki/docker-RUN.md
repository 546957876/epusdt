1. 创建和进入部署目录
```shell
mkdir epusdt && cd epusdt
```

2. 创建配置和运行时目录
```shell
mkdir -p conf runtime
```

3. 把配置文件放到 `conf/.env`

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
db_type=sqlite

# sqlite primary database config
sqlite_database_filename=
sqlite_table_prefix=

# postgres config
postgres_host=127.0.0.1
postgres_port=3306
postgres_user=mysql_user
postgres_passwd=mysql_password
postgres_database=database_name
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
4. docker compose 创建
```shell
cat <<EOF > docker-compose.yaml
services:
  epusdt:
    image: gmwallet/epusdt:latest
    restart: always
    build:
      context: .
      dockerfile: Dockerfile
    environment:
      EPUSDT_CONFIG: /app/conf/.env
      EPUSDT_DOCKER: "1"
    volumes:
      - ./conf:/app/conf
      - ./runtime:/app/runtime
    ports:
      - "8000:8000"
EOF
```
5. 运行
```shell
docker compose up -d
```
6. 配置独角兽后台

商户密钥： http://your_domain/payments/epusdt/v1/order/create-transaction
