# my2g 二进制 + systemd 部署方案

目标：把 Sub2API 应用从“Mac/Colima 构建 Docker 镜像并上传”改为“Mac 原生构建前端和 Linux Go 二进制，my2g 用 systemd 运行应用”。PostgreSQL、Redis、Mihomo 可以继续用 Docker Compose 保持现状。

## 结论

推荐形态：

```text
Mac 本地:
  pnpm build
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags embed

my2g:
  systemd 运行 /opt/sub2api/current/sub2api
  Docker Compose 继续运行 postgres / redis / mihomo
  Nginx / Cloudflare Tunnel 继续回源到 127.0.0.1:8080
```

这个方案保留 Go 的优势：构建快、产物小、服务器只做运行，不做镜像构建。它也避开当前最慢的链路：Colima 里 `docker build --platform linux/amd64`、`docker save`、`gzip`、`scp`、远端 `docker load`。

## 不变与变化

不变：

- 公网入口仍是 Cloudflare Tunnel + Nginx。
- 对外服务地址仍是 `https://portal.lizubin.online/`。
- PostgreSQL、Redis、Mihomo 不随应用更新重启。
- `/opt/sub2api/.env` 仍保留密钥来源，不提交 Git。
- 健康检查仍使用 `https://portal.lizubin.online/health`。

变化：

- `sub2api` 应用容器停止使用。
- 应用由 systemd 管理，进程直接跑在宿主机。
- 应用更新只上传 Linux 二进制和 `resources/`，不再上传 Docker 镜像。
- Postgres/Redis/Mihomo 需要绑定到宿主机 `127.0.0.1`，供 systemd 应用访问。
- 首次切换会调整 Compose 端口映射，可能重建 Postgres/Redis/Mihomo 容器；日常应用发布才是不重启这些依赖。

## 较小闭环原则

本方案按“小闭环”执行，每个闭环只改变一类状态，并且执行后立刻验证：

1. 预检闭环：只检查当前服务、端口、工具和配置，不改状态。
2. 备份闭环：先完成可校验的数据库和文件备份，再继续。
3. 依赖暴露闭环：只把 Postgres/Redis/Mihomo 暴露到 `127.0.0.1`，验证宿主机可连。
4. 配置迁移闭环：迁移旧 `/app/data` 和 `.env`，验证 `config.yaml`、`.installed`、密钥存在。
5. 发布闭环：上传 release 并验证二进制、资源目录、版本输出。
6. 切换闭环：先停止旧应用容器，再启动 systemd 应用。
7. 验证闭环：先本机健康检查，再公网健康检查，再做关键业务探测。
8. 回滚闭环：任何闭环失败，优先恢复到上一个已验证状态。

## 推荐目录

服务器目录建议：

```text
/opt/sub2api/
  current -> /opt/sub2api/releases/20260622-001
  releases/
    20260622-001/
      sub2api
      resources/
  shared/
    data/
      config.yaml
      .installed
  sub2api.env
  docker-compose.yml
  compose.my2g.yml
  compose.mihomo.yml
  compose.binary-deps.yml
  backups/
```

说明：

- `current` 是当前运行版本的软链接。
- `shared/data` 存放运行期配置和安装锁，升级应用时不覆盖。
- `sub2api.env` 是 systemd 的 `EnvironmentFile`，权限应为 `root:root 600`。
- 每次发布生成一个新 `releases/<tag>`，回滚只切回旧 `current`。

## 构建命令

在 Mac 仓库根目录执行：

```bash
TAG="${TAG:-$(date +%Y%m%d-%H%M%S)}"
COMMIT="$(git rev-parse --short HEAD)"
DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

pnpm --dir frontend install --frozen-lockfile
pnpm --dir frontend run build

cd backend
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -tags embed \
  -trimpath \
  -ldflags="-s -w -X main.Commit=${COMMIT} -X main.Date=${DATE} -X main.BuildType=release" \
  -o "sub2api-${TAG}" \
  ./cmd/server
```

前端产物会输出到 `backend/internal/web/dist`，再被 `go build -tags embed` 嵌入二进制。服务器不需要 Node、pnpm、Go，也不需要 Docker 构建应用镜像。

## 上传产物

```bash
TAG="${TAG:-$(date +%Y%m%d-%H%M%S)}"
REMOTE_RELEASE="/opt/sub2api/releases/${TAG}"

ssh my2g "sudo mkdir -p '${REMOTE_RELEASE}'"
scp "backend/sub2api-${TAG}" "my2g:/tmp/sub2api"
rsync -a --delete backend/resources/ "my2g:/tmp/sub2api-resources/"

ssh my2g "sudo install -m 0755 /tmp/sub2api '${REMOTE_RELEASE}/sub2api' && \
  sudo mkdir -p '${REMOTE_RELEASE}/resources' && \
  sudo rsync -a --delete /tmp/sub2api-resources/ '${REMOTE_RELEASE}/resources/' && \
  sudo chown -R root:root '${REMOTE_RELEASE}' && \
  rm -f /tmp/sub2api && rm -rf /tmp/sub2api-resources"
```

如果服务器没有 `rsync`，可以用 `tar` 替代，但 `rsync` 对小改动更快。

## systemd 服务

建议新建或替换 `/etc/systemd/system/sub2api.service`：

```ini
[Unit]
Description=Sub2API - AI API Gateway Platform
Documentation=https://github.com/Wei-Shaw/sub2api
After=network.target docker.service
Wants=docker.service

[Service]
Type=simple
User=sub2api
Group=sub2api
WorkingDirectory=/opt/sub2api/current
EnvironmentFile=/opt/sub2api/sub2api.env
ExecStart=/opt/sub2api/current/sub2api
Restart=always
RestartSec=5
StandardOutput=journal
StandardError=journal
SyslogIdentifier=sub2api

NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=/opt/sub2api

[Install]
WantedBy=multi-user.target
```

`/opt/sub2api/sub2api.env` 示例：

```bash
GIN_MODE=release
SERVER_HOST=127.0.0.1
SERVER_PORT=8080
DATA_DIR=/opt/sub2api/shared/data
TZ=Asia/Shanghai

AUTO_SETUP=true
RUN_MODE=standard

DATABASE_HOST=127.0.0.1
DATABASE_PORT=5432
DATABASE_USER=sub2api
DATABASE_PASSWORD=change_this_secure_password
DATABASE_DBNAME=sub2api
DATABASE_SSLMODE=disable
DATABASE_MAX_OPEN_CONNS=50
DATABASE_MAX_IDLE_CONNS=10
DATABASE_CONN_MAX_LIFETIME_MINUTES=30
DATABASE_CONN_MAX_IDLE_TIME_MINUTES=5

REDIS_HOST=127.0.0.1
REDIS_PORT=6379
REDIS_PASSWORD=
REDIS_DB=0
REDIS_POOL_SIZE=1024
REDIS_MIN_IDLE_CONNS=10
REDIS_ENABLE_TLS=false

JWT_SECRET=change_this_fixed_secret
JWT_EXPIRE_HOUR=24
TOTP_ENCRYPTION_KEY=change_this_fixed_key

ADMIN_EMAIL=admin@sub2api.local
ADMIN_PASSWORD=
```

注意：

- `DATABASE_PASSWORD`、`JWT_SECRET`、`TOTP_ENCRYPTION_KEY` 应从当前 `/opt/sub2api/.env` 迁移，不要重新生成导致登录或 2FA 异常。
- `ADMIN_PASSWORD` 只在首次空库初始化时有意义；已有用户数据时不会覆盖现有管理员。
- `SERVER_HOST=127.0.0.1` 更适合当前 Cloudflare Tunnel/Nginx 回源形态，避免应用端口直接暴露公网。
- 上面只是最小示例，不是完整生产配置。迁移时应以当前 `/opt/sub2api/.env` 为准，完整保留日志、H2C、网关超时、OAuth client secret、调度、支付、Webhook 等已经启用的变量。
- 二进制直跑后，后台备份/恢复功能会调用宿主机的 `pg_dump` 和 `psql`。my2g 上需要安装 PostgreSQL client，建议与 Postgres 容器主版本一致。

## 依赖访问方式

systemd 应用在宿主机上运行，不能使用 Docker Compose 内部 DNS 名称 `postgres`、`redis`、`mihomo`。需要把 Postgres、Redis、Mihomo 仅绑定到本机回环地址。

在服务器 `/opt/sub2api/compose.binary-deps.yml` 增加：

```yaml
services:
  postgres:
    ports:
      - "127.0.0.1:5432:5432"

  redis:
    ports:
      - "127.0.0.1:6379:6379"

  mihomo:
    ports:
      - "127.0.0.1:7890:7890"
      - "127.0.0.1:7891:7891"
```

说明：

- Postgres/Redis 的应用连接地址从 `postgres`、`redis` 改为 `127.0.0.1`。
- Mihomo 代理地址从 `http://mihomo:7890` 改为 `http://127.0.0.1:7890`，或从 `socks5://mihomo:7891` 改为 `socks5://127.0.0.1:7891`。
- 如果数据库里的代理表已经保存了 `host=mihomo`，切换前需要更新为 `host=127.0.0.1`。
- `compose.binary-deps.yml` 首次生效时可能重建依赖容器，必须先完成数据库备份。

依赖切换后，Compose 命令使用：

```bash
docker compose \
  -f docker-compose.yml \
  -f compose.my2g.yml \
  -f compose.mihomo.yml \
  -f compose.binary-deps.yml \
  up -d postgres redis mihomo
```

如果宿主机已经占用端口，改成 `127.0.0.1:5433:5432`、`127.0.0.1:6380:6379`、`127.0.0.1:7892:7890`，同时修改 `sub2api.env` 和代理配置里的端口。

## 首次切换步骤

按下面闭环顺序执行。每个闭环通过后再进入下一步；失败时先停下，不要继续叠加改动。

### 闭环 0：预检当前状态

只读检查当前容器、端口、健康检查和必要工具：

```bash
ssh my2g 'cd /opt/sub2api && \
  sudo docker compose -f docker-compose.yml -f compose.my2g.yml -f compose.mihomo.yml ps && \
  sudo docker inspect --format "{{.State.Health.Status}}" sub2api sub2api-postgres sub2api-redis sub2api-mihomo 2>/dev/null || true && \
  ss -lntp | grep -E ":(8080|5432|6379|7890|7891)\\b" || true && \
  command -v rsync && command -v systemctl && command -v docker'
```

通过标准：

- 当前 `sub2api`、Postgres、Redis、Mihomo 正常。
- `127.0.0.1:8080` 当前由 Docker 应用容器提供服务。
- `5432`、`6379`、`7890`、`7891` 没有被非本方案进程占用；如已占用，先确定替代端口。

### 闭环 1：备份当前配置和数据库

先备份文件，再做可校验的 PostgreSQL dump：

```bash
ssh my2g 'sudo bash -lc '"'"'
  cd /opt/sub2api
  set -e
  stamp="$(date +%Y%m%d-%H%M%S)" && \
  mkdir -p backups && \
  cp .env "backups/.env-before-binary-${stamp}" && \
  cp docker-compose.yml "backups/docker-compose-before-binary-${stamp}.yml" && \
  cp compose.my2g.yml "backups/compose.my2g-before-binary-${stamp}.yml" && \
  cp compose.mihomo.yml "backups/compose.mihomo-before-binary-${stamp}.yml" && \
  set -a && . ./.env && set +a && \
  tmp="/tmp/sub2api-postgres-${stamp}.dump.tmp" && \
  backup="backups/postgres-${stamp}.dump" && \
  docker compose -f docker-compose.yml -f compose.my2g.yml -f compose.mihomo.yml exec -T postgres \
    pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc > "$tmp" && \
  test -s "$tmp" && \
  docker compose -f docker-compose.yml -f compose.my2g.yml -f compose.mihomo.yml exec -T postgres \
    pg_restore --list < "$tmp" >/dev/null && \
  install -m 0600 -o root -g root "$tmp" "$backup" && \
  rm -f "$tmp" && \
  ls -lh "$backup"
'"'"''
```

通过标准：

- `backups/postgres-<stamp>.dump` 非空。
- `pg_restore --list` 校验通过。
- 重要备份最好再复制到 my2g 之外，只留在同一台机器不是完整备份。

### 闭环 2：准备宿主机用户、目录和客户端工具

准备 systemd 用户、release 目录、共享 data 目录，并安装与 Postgres 容器主版本一致的 PostgreSQL client：

```bash
ssh my2g 'sudo useradd --system --home /opt/sub2api --shell /usr/sbin/nologin sub2api 2>/dev/null || true && \
  sudo mkdir -p /opt/sub2api/releases /opt/sub2api/shared/data /opt/sub2api/backups && \
  sudo chown -R sub2api:sub2api /opt/sub2api/shared && \
  pg_major="$(sudo docker exec sub2api-postgres postgres -V | sed -E "s/.* ([0-9]+).*/\\1/")" && \
  host_pg_major="$(pg_dump --version 2>/dev/null | sed -E "s/.* ([0-9]+).*/\\1/" || true)" && \
  if [ "$host_pg_major" != "$pg_major" ] || ! command -v psql >/dev/null 2>&1 || ! command -v redis-cli >/dev/null 2>&1; then \
    sudo apt-get update && sudo apt-get install -y "postgresql-client-${pg_major}" redis-tools; \
  fi && \
  test "$(pg_dump --version | sed -E "s/.* ([0-9]+).*/\\1/")" = "$pg_major" && \
  pg_dump --version && psql --version && redis-cli --version'
```

通过标准：

- `id sub2api` 存在。
- `/opt/sub2api/shared/data` 属主为 `sub2api:sub2api`。
- 如果后续需要应用进程写本地备份或临时导出文件，提前把对应目录单独授权给 `sub2api`；否则 `/opt/sub2api/backups` 只作为人工 root 备份目录使用。
- `pg_dump`、`psql` 和 `redis-cli` 可执行。
- `pg_dump` 主版本与 `sub2api-postgres` 容器主版本一致；如果 apt 源没有对应的 `postgresql-client-<major>`，先配置 PostgreSQL 官方 apt 源或等价安装源，不要带着旧版 `pg_dump` 继续切换。

### 闭环 3：迁移旧容器数据目录

把当前容器里的 `/app/data` 迁移到宿主机固定目录，保留 `config.yaml`、`.installed`、日志和其他运行期文件：

```bash
ssh my2g 'cd /opt/sub2api && \
  set -e && \
  sudo docker cp sub2api:/app/data/. /opt/sub2api/shared/data/ && \
  sudo chown -R sub2api:sub2api /opt/sub2api/shared/data && \
  sudo test -f /opt/sub2api/shared/data/config.yaml && \
  sudo test -f /opt/sub2api/shared/data/.installed && \
  sudo ls -la /opt/sub2api/shared/data | sed -n "1,40p"'
```

通过标准：

- `/opt/sub2api/shared/data/config.yaml` 存在。
- `/opt/sub2api/shared/data/.installed` 存在。
- 不触发重新初始化，不覆盖旧数据库配置。

### 闭环 4：创建 systemd 环境文件

不要只照抄本文最小样例。以当前 `/opt/sub2api/.env` 为基础生成 `/opt/sub2api/sub2api.env`，并至少调整这些项：

```bash
SERVER_HOST=127.0.0.1
SERVER_PORT=8080
DATA_DIR=/opt/sub2api/shared/data
DATABASE_HOST=127.0.0.1
DATABASE_USER=<当前 POSTGRES_USER>
DATABASE_PASSWORD=<当前 POSTGRES_PASSWORD>
DATABASE_DBNAME=<当前 POSTGRES_DB>
REDIS_HOST=127.0.0.1
```

如果 Postgres/Redis 改用了替代端口，也同步调整：

```bash
DATABASE_PORT=5433
REDIS_PORT=6380
```

如果有 OpenAI/Gemini 账号绑定 Mihomo 代理，把代理配置从 `mihomo:7890` 改成 `127.0.0.1:7890`。

```bash
ssh my2g 'sudo bash -lc '"'"'
  set -e
  set -a
  . /opt/sub2api/.env
  set +a
  install -m 0600 -o root -g root /opt/sub2api/.env /opt/sub2api/sub2api.env

  ensure_env() {
    key="$1"
    value="$2"
    file="/opt/sub2api/sub2api.env"
    tmp="$(mktemp)"
    awk -v k="$key" -v v="$value" '"'"'
      BEGIN { found = 0 }
      index($0, k "=") == 1 { print k "=" v; found = 1; next }
      { print }
      END { if (!found) print k "=" v }
    '"'"' "$file" > "$tmp"
    cat "$tmp" > "$file"
    rm -f "$tmp"
  }

  ensure_env SERVER_HOST 127.0.0.1
  ensure_env SERVER_PORT "${SERVER_PORT:-8080}"
  ensure_env DATA_DIR /opt/sub2api/shared/data
  ensure_env DATABASE_HOST 127.0.0.1
  ensure_env DATABASE_PORT "${DATABASE_PORT:-5432}"
  ensure_env DATABASE_USER "${DATABASE_USER:-${POSTGRES_USER:-sub2api}}"
  ensure_env DATABASE_PASSWORD "${DATABASE_PASSWORD:-${POSTGRES_PASSWORD:?POSTGRES_PASSWORD is required}}"
  ensure_env DATABASE_DBNAME "${DATABASE_DBNAME:-${POSTGRES_DB:-sub2api}}"
  ensure_env DATABASE_SSLMODE "${DATABASE_SSLMODE:-disable}"
  ensure_env REDIS_HOST 127.0.0.1
  ensure_env REDIS_PORT "${REDIS_PORT:-6379}"

  grep -E "^(SERVER_HOST|SERVER_PORT|DATA_DIR|DATABASE_HOST|DATABASE_PORT|DATABASE_USER|DATABASE_DBNAME|REDIS_HOST|REDIS_PORT)=" /opt/sub2api/sub2api.env
  for key in DATABASE_PASSWORD JWT_SECRET TOTP_ENCRYPTION_KEY; do
    grep -q "^${key}=.\+" /opt/sub2api/sub2api.env
    printf "%s=<set>\n" "$key"
  done
'"'"''
```

通过标准：

- `DATABASE_PASSWORD`、`JWT_SECRET`、`TOTP_ENCRYPTION_KEY` 仍是旧值，不为空。
- 生产环境已用到的 OAuth secret、支付、Webhook、网关调度等变量没有丢。
- `/opt/sub2api/sub2api.env` 权限为 `root:root 600`。
- `sub2api.env` 只能依赖 systemd `EnvironmentFile` 支持的简单 `KEY=value` 语义；不要保留 Docker Compose 专用的 `${VAR:-default}`、`${VAR:?required}` 展开写法。

### 闭环 5：暴露依赖到宿主机回环地址

创建 `compose.binary-deps.yml`，让 Postgres/Redis/Mihomo 绑定到 `127.0.0.1`。先渲染 Compose 配置，确认 Postgres、Redis 的 volume 名称和挂载路径没有变化：

```bash
ssh my2g 'cd /opt/sub2api && \
  sudo tee compose.binary-deps.yml >/dev/null <<'"'"'EOF'"'"'
services:
  postgres:
    ports:
      - "127.0.0.1:5432:5432"

  redis:
    ports:
      - "127.0.0.1:6379:6379"

  mihomo:
    ports:
      - "127.0.0.1:7890:7890"
      - "127.0.0.1:7891:7891"
EOF
  sudo docker compose -f docker-compose.yml -f compose.my2g.yml -f compose.mihomo.yml -f compose.binary-deps.yml config | sed -n "/services:/,/networks:/p"'
```

确认依赖容器的持久化挂载不变后，再重建依赖容器：

```bash
ssh my2g 'cd /opt/sub2api && \
  sudo docker compose -f docker-compose.yml -f compose.my2g.yml -f compose.mihomo.yml -f compose.binary-deps.yml up -d postgres redis mihomo'
```

验证宿主机访问：

```bash
ssh my2g 'sudo bash -lc '"'"'
  set -e
  set -a
  . /opt/sub2api/sub2api.env
  set +a
  PGPASSWORD="$DATABASE_PASSWORD" psql -h 127.0.0.1 -p "${DATABASE_PORT:-5432}" -U "$DATABASE_USER" -d "$DATABASE_DBNAME" -c "select 1;" && \
  redis-cli -h 127.0.0.1 -p "${REDIS_PORT:-6379}" ${REDIS_PASSWORD:+-a "$REDIS_PASSWORD"} ping && \
  curl -fsS --max-time 15 -x http://127.0.0.1:7890 https://api.openai.com/v1/models -o /dev/null -w "mihomo_http_code=%{http_code}\n"
'"'"''
```

通过标准：

- `psql` 返回 `1`。
- `redis-cli ping` 返回 `PONG`。
- Mihomo 代理请求能连通，HTTP 状态码不是本地连接错误。
- 如果本闭环失败，先移走 `compose.binary-deps.yml`，再用原 Compose 文件恢复依赖容器并复测旧应用，不要继续进入 systemd 切换闭环。

### 闭环 6：构建并上传 release

本地构建并上传一个 release，然后在服务器验证二进制和资源目录：

```bash
# 本地执行构建和上传命令，见上文“构建命令”和“上传产物”
```

```bash
ssh my2g "test -x /opt/sub2api/releases/${TAG}/sub2api && \
  test -f /opt/sub2api/releases/${TAG}/resources/model-pricing/model_prices_and_context_window.json && \
  /opt/sub2api/releases/${TAG}/sub2api --version"
```

通过标准：

- 二进制可执行。
- `resources/model-pricing/model_prices_and_context_window.json` 存在。
- `--version` 能输出版本信息。

### 闭环 7：安装 systemd 服务并 dry-run 配置

在服务器切换软链接：

```bash
ssh my2g "sudo ln -sfn /opt/sub2api/releases/${TAG} /opt/sub2api/current"
```

安装 `/etc/systemd/system/sub2api.service` 后验证：

```bash
ssh my2g 'sudo systemctl daemon-reload && \
  sudo systemd-analyze verify /etc/systemd/system/sub2api.service && \
  sudo systemctl cat sub2api'
```

通过标准：

- `systemd-analyze verify` 无错误。
- `EnvironmentFile`、`WorkingDirectory`、`ExecStart` 指向本文约定路径。

### 闭环 8：切换应用进程

停止应用容器，但保留 Postgres/Redis/Mihomo：

```bash
ssh my2g 'cd /opt/sub2api && \
  sudo docker compose -f docker-compose.yml -f compose.my2g.yml -f compose.mihomo.yml -f compose.binary-deps.yml stop sub2api'
```

确认 8080 空出来后启动 systemd 应用：

```bash
ssh my2g 'if [ "$(sudo docker inspect -f "{{.State.Running}}" sub2api 2>/dev/null || true)" = "true" ]; then \
    echo "sub2api container is still running"; exit 1; \
  fi && \
  if ss -lntp | grep ":8080\\b"; then \
    echo "port 8080 is still occupied"; exit 1; \
  fi && \
  sudo systemctl enable sub2api && \
  sudo systemctl restart sub2api'
```

### 闭环 9：分层验证

先验证本机，再验证公网：

```bash
ssh my2g 'systemctl --no-pager status sub2api'
ssh my2g 'curl -fsS http://127.0.0.1:8080/health'
curl -fsS https://portal.lizubin.online/health
```

再验证关键业务：

```bash
ssh my2g 'journalctl -u sub2api -n 120 --no-pager'
ssh my2g 'curl -fsS http://127.0.0.1:8080/api/v1/settings/public >/dev/null'
ssh my2g 'curl -fsS --max-time 15 -x http://127.0.0.1:7890 https://api.openai.com/v1/models -o /dev/null -w "mihomo_http_code=%{http_code}\n"'
```

通过标准：

- 本机和公网 `/health` 都成功。
- 日志没有循环重启、数据库连接失败、Redis 连接失败、读取 `config.yaml` 失败。
- 后台代理配置若使用 Mihomo，账号测试或等价探测成功。

## 日常发布流程

1. Mac 本地构建新二进制。
2. 上传到 `/opt/sub2api/releases/<tag>`。
3. 服务器执行：

```bash
sudo ln -sfn /opt/sub2api/releases/<tag> /opt/sub2api/current
sudo systemctl restart sub2api
```

4. 健康检查：

```bash
curl -fsS http://127.0.0.1:8080/health
curl -fsS https://portal.lizubin.online/health
```

发布期间 Postgres/Redis/Mihomo 不重启。用户影响只发生在 `systemctl restart sub2api` 的短暂窗口。

## 回滚

查看历史 release：

```bash
ssh my2g 'ls -1dt /opt/sub2api/releases/* | head'
```

切回上一版：

```bash
ssh my2g 'sudo ln -sfn /opt/sub2api/releases/<previous-tag> /opt/sub2api/current && \
  sudo systemctl restart sub2api && \
  curl -fsS http://127.0.0.1:8080/health'
```

如果 systemd 切换失败，可以临时恢复旧应用容器：

```bash
ssh my2g 'sudo systemctl stop sub2api || true && \
  cd /opt/sub2api && \
  sudo docker compose -f docker-compose.yml -f compose.my2g.yml -f compose.mihomo.yml up -d --no-deps sub2api'
```

如果失败发生在“依赖暴露闭环”，优先恢复依赖容器到旧 Compose 形态：

```bash
ssh my2g 'cd /opt/sub2api && \
  sudo mv compose.binary-deps.yml "backups/compose.binary-deps.failed.$(date +%Y%m%d-%H%M%S).yml" 2>/dev/null || true && \
  sudo docker compose -f docker-compose.yml -f compose.my2g.yml -f compose.mihomo.yml up -d postgres redis mihomo && \
  sudo docker compose -f docker-compose.yml -f compose.my2g.yml -f compose.mihomo.yml up -d --no-deps sub2api && \
  curl -fsS http://127.0.0.1:8080/health'
```

## 风险与边界

- 这个方案不再把应用封装成 Docker 镜像，镜像级别的可追踪性会降低；用 `releases/<tag>` 和 `current` 软链接补足版本追踪和回滚。
- 系统依赖更少，但 systemd 服务文件、环境变量和目录权限必须维护好。
- Postgres/Redis 只应绑定 `127.0.0.1`，不要绑定 `0.0.0.0`。
- 首次暴露 Postgres/Redis/Mihomo 到宿主机是一次依赖层变更，必须单独验证和单独回滚；不要和应用进程切换合并执行。
- `DATA_DIR` 必须固定到 `/opt/sub2api/shared/data`，否则配置文件和 `.installed` 可能落到 release 目录，升级时容易丢状态。
- `resources/` 需要随二进制一起发布，因为模型价格等运行期 fallback 数据会从 `./resources` 读取。

## 后续可自动化

可以新增一个 `make deploy-my2g-binary`，封装：

1. 前端构建。
2. Go 交叉编译。
3. 上传 release。
4. 服务器切 `current`。
5. `systemctl restart sub2api`。
6. 本地和公网健康检查。
7. 失败时自动切回上一 release。

当前阶段建议先按本文手动跑通一次，再把稳定命令固化成脚本。
