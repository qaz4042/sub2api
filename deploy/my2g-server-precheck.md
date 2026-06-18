# my2g 服务器部署方案与前置检查

检查日期：2026-06-18  
目标服务器：`ssh my2g`  
目标代码：`qaz4042/sub2api` 的 `main` 分支，检查时提交为 `0389a6d4caead78311a3fe4f14ec855af7f827d6`

## 结论

推荐采用：

> GitHub Actions 构建固定版本镜像 → GHCR → my2g 上使用 Docker Compose 运行 Sub2API、PostgreSQL、Redis → 宿主机 Nginx 终止 HTTPS。

不建议直接运行仓库默认的 `weishaw/sub2api:latest`，原因如下：

1. 当前仓库是 `qaz4042/sub2api` fork，默认镜像不是当前项目代码。
2. 当前提交没有 Git tag，GitHub 仓库也没有 Release；部署前必须先产出当前代码对应的镜像。
3. `latest` 不可审计且回滚目标不确定，生产环境应固定版本标签或镜像 digest。
4. my2g 当前访问 GHCR 正常，但访问 Docker Hub 超时；默认 Compose 中的应用、PostgreSQL、Redis 镜像不能保证直接拉取成功。
5. 服务器只有 1.8 GiB 内存，必须增加 Swap、收紧连接池和并发，并给容器设置资源上限。

这台服务器适合低到中低并发的初期部署。若持续并发、图片生成请求或数据量明显增加，应优先升级到至少 `2 vCPU / 4 GiB`，或将 PostgreSQL 迁移到托管数据库。

## 已确认的服务器现状

| 项目 | 结果 | 判断 |
| --- | --- | --- |
| 系统 | Alibaba Cloud Linux 3.2104 U12.3，x86_64 | 可部署 |
| 内核 | Linux 5.10.134 | 可部署 |
| CPU | 2 vCPU | 初期够用 |
| 内存 | 1.8 GiB，可用约 1.5 GiB | 偏紧 |
| Swap | 0 | 部署前必须补充 |
| 根磁盘 | 40 GiB，已用 6.3 GiB，剩余约 31 GiB | 初期够用 |
| 时区/NTP | Asia/Shanghai，时钟已同步 | 正常 |
| SELinux | Disabled | 无额外策略阻塞 |
| firewalld | 已安装但未运行 | 依赖阿里云安全组；建议同时启用主机防火墙 |
| 监听端口 | 仅 `22/tcp` | `80/443` 可用 |
| Docker | 未安装 | 待安装 |
| Docker Compose | 未安装 | 待安装 Compose plugin |
| Nginx/Caddy | 未安装 | 推荐安装 Nginx |
| Git | 未安装 | Docker 部署非必需 |
| OOM | 近 30 天未发现 OOM | 当前正常 |
| 失败服务 | 仅 `kdump.service` | 不阻塞本项目 |
| Docker Hub | 连接超时 | 部署阻塞项 |
| GHCR | 可连接 | 推荐作为镜像仓库 |

当前常驻系统进程约占 `300–400 MiB`，其中阿里云安全组件约占 `100 MiB`。部署后三个容器、Docker daemon、Nginx 和系统本身将接近物理内存上限。

## 部署前必须确认

- [ ] 已准备域名，例如 `api.example.com`，A/AAAA 记录正确指向 my2g。
- [ ] 阿里云安全组只开放 `22`、`80`、`443`；不开放 `5432`、`6379`、`8080`。
- [ ] 当前提交已经构建并推送到 GHCR，且获得不可变镜像标签。
- [ ] my2g 能拉取 Sub2API、PostgreSQL、Redis 三个镜像。
- [ ] 已增加 2 GiB Swap。
- [ ] 已安装 Docker Engine 和 Compose plugin。
- [ ] 已生成并离线保存 PostgreSQL、JWT、TOTP、管理员密码。
- [ ] 已确定备份目录或远端对象存储。
- [ ] 已确认使用本项目和上游账号/API 的授权、合规及数据处理责任。

## 阻塞项 1：先发布当前项目镜像

仓库已有 `.github/workflows/release.yml`，会在推送 `v*` tag 后构建 GHCR 镜像。建议从准备部署的提交创建语义化预发布标签，例如：

```bash
git status
git log -1 --oneline

# 示例；实际版本号应按项目版本策略确定
git tag -a v0.0.0-my2g.20260618.1 0389a6d4caead78311a3fe4f14ec855af7f827d6 \
  -m "my2g deployment build"
git push origin v0.0.0-my2g.20260618.1
```

等待 GitHub Actions 的 `Release` workflow 成功后，预期镜像名称类似：

```text
ghcr.io/qaz4042/sub2api:0.0.0-my2g.20260618.1
```

发布后检查：

```bash
docker pull ghcr.io/qaz4042/sub2api:0.0.0-my2g.20260618.1
docker inspect ghcr.io/qaz4042/sub2api:0.0.0-my2g.20260618.1 \
  --format '{{index .RepoDigests 0}}'
```

将得到的 digest 记录下来。正式 Compose 最稳妥的写法是：

```yaml
image: ghcr.io/qaz4042/sub2api@sha256:实际摘要
```

如果 GHCR package 是私有的，在服务器使用仅有 `read:packages` 权限的 GitHub PAT 登录：

```bash
read -rsp 'GHCR token: ' CR_PAT
echo
printf '%s' "$CR_PAT" | docker login ghcr.io -u qaz4042 --password-stdin
unset CR_PAT
```

不要把 PAT 写入 `.env`、Compose 文件或 shell history。公开仓库不代表其 GHCR package 自动公开，需在 GitHub package 设置中单独确认可见性。

## 阻塞项 2：解决 Docker Hub 拉取

`docker-compose.local.yml` 还依赖：

```text
postgres:18-alpine
redis:8-alpine
```

检查时 my2g 访问 Docker Hub 超时。部署前必须选择以下一种方案：

### 方案 A：配置阿里云容器镜像加速器

在阿里云容器镜像服务控制台取得当前账号专属的加速地址，再写入 `/etc/docker/daemon.json`。不要直接复制其他账号的加速地址。

```json
{
  "registry-mirrors": [
    "https://你的专属加速地址"
  ],
  "log-driver": "local",
  "log-opts": {
    "max-size": "20m",
    "max-file": "5"
  }
}
```

应用并验证：

```bash
systemctl restart docker
docker info
docker pull postgres:18-alpine
docker pull redis:8-alpine
```

### 方案 B：将依赖镜像同步到自己的 GHCR

如果加速器不稳定，在可访问 Docker Hub 的 CI 中将以下镜像同步到 GHCR，并在 Compose overlay 中替换镜像地址：

```text
docker.io/library/postgres:18-alpine
docker.io/library/redis:8-alpine
```

生产部署应固定 digest。不要使用来源不明的公共镜像代理。

## 服务器准备步骤

以下命令均在 `ssh my2g` 后执行。

### 1. 更新基础包

```bash
dnf makecache
dnf update -y
dnf install -y ca-certificates curl openssl yum-utils nginx \
  certbot python3-certbot-nginx
```

如果更新了内核，先重启并重新登录：

```bash
reboot
```

### 2. 增加 2 GiB Swap

```bash
fallocate -l 2G /swapfile
chmod 600 /swapfile
mkswap /swapfile
swapon /swapfile
printf '/swapfile none swap sw 0 0\n' >> /etc/fstab

cat >/etc/sysctl.d/99-sub2api.conf <<'EOF'
vm.swappiness=10
vm.overcommit_memory=1
EOF

sysctl --system
free -h
swapon --show
```

检查时 `vm.swappiness=0` 且没有 Swap；上述设置用于缓冲突发内存压力，不代表可以长期依赖 Swap 承载业务。

### 3. 安装 Docker Engine 和 Compose plugin

Alibaba Cloud Linux 3 的 `$releasever` 与 Docker CentOS 仓库目录不一致。检查时 Docker 官方 CDN 间歇性 SSL 失败，而阿里云 Docker CE 镜像站的 CentOS 8 仓库可稳定读取，并可提供 Docker Engine 26.1.3、Buildx 0.14.0 和 Compose 2.27.0。因此这里固定使用阿里云镜像站的 CentOS 8 stable 仓库：

```bash
yum-config-manager --add-repo \
  https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo

# Alibaba Cloud Linux 3 兼容 el8 包，但其 releasever 不是 8
sed -i 's#\\$releasever#8#g' /etc/yum.repos.d/docker-ce.repo

dnf makecache
dnf list --showduplicates docker-ce docker-compose-plugin
dnf install -y docker-ce docker-ce-cli containerd.io \
  docker-buildx-plugin docker-compose-plugin

systemctl enable --now docker
docker version
docker compose version
```

安装时应核对镜像站提供的 GPG key 是否与 Docker 官方 key 一致。Docker 官方文档当前列出的指纹为：

```text
060A 61C5 1B55 8A7F 742B 77AA C52F EB6B 621E 9F35
```

安装失败时不要改用未经审核的一键脚本；优先检查仓库兼容性、镜像站同步状态和网络。

### 4. 配置 Docker 日志轮转

若第 2 个阻塞项中尚未创建 `/etc/docker/daemon.json`，至少配置：

```json
{
  "log-driver": "local",
  "log-opts": {
    "max-size": "20m",
    "max-file": "5"
  }
}
```

```bash
systemctl restart docker
docker info --format '{{.LoggingDriver}}'
```

日志配置只对重建后的容器生效。

### 5. 创建部署目录

```bash
install -d -m 0750 /opt/sub2api
cd /opt/sub2api
```

将当前仓库的以下文件复制到服务器：

```bash
scp deploy/docker-compose.local.yml my2g:/opt/sub2api/docker-compose.yml
scp deploy/.env.example my2g:/opt/sub2api/.env.example
```

不要在服务器上从 `main` 临时构建。2 GiB 机器不适合同时承担 Node、Go 和 Docker 多阶段构建，且会让部署产物不可追踪。

## 推荐的 my2g Compose overlay

在 `/opt/sub2api/compose.my2g.yml` 创建以下内容。将应用镜像替换为实际固定标签，生产稳定后进一步替换为 digest。

```yaml
services:
  sub2api:
    image: ghcr.io/qaz4042/sub2api:0.0.0-my2g.20260618.1
    mem_limit: 768m
    mem_reservation: 384m
    cpus: 1.75
    pids_limit: 512
    environment:
      GOMEMLIMIT: 600MiB
      GOMAXPROCS: "2"

      DATABASE_MAX_OPEN_CONNS: "30"
      DATABASE_MAX_IDLE_CONNS: "5"
      REDIS_POOL_SIZE: "64"
      REDIS_MIN_IDLE_CONNS: "4"

      SERVER_MAX_REQUEST_BODY_SIZE: "67108864"
      GATEWAY_MAX_BODY_SIZE: "67108864"
      SERVER_H2C_MAX_CONCURRENT_STREAMS: "20"

      GATEWAY_MAX_CONNS_PER_HOST: "128"
      GATEWAY_MAX_IDLE_CONNS: "256"
      GATEWAY_MAX_IDLE_CONNS_PER_HOST: "64"

      GATEWAY_IMAGE_CONCURRENCY_ENABLED: "true"
      GATEWAY_IMAGE_CONCURRENCY_MAX_CONCURRENT_REQUESTS: "1"
      GATEWAY_IMAGE_CONCURRENCY_OVERFLOW_MODE: "reject"
      GATEWAY_IMAGE_CONCURRENCY_MAX_WAITING_REQUESTS: "0"

      LOG_OUTPUT_TO_STDOUT: "true"
      LOG_OUTPUT_TO_FILE: "false"

  postgres:
    mem_limit: 448m
    mem_reservation: 256m
    cpus: 1.0
    pids_limit: 256
    command:
      - postgres
      - -c
      - max_connections=80
      - -c
      - shared_buffers=128MB
      - -c
      - effective_cache_size=384MB
      - -c
      - maintenance_work_mem=32MB
      - -c
      - work_mem=2MB

  redis:
    mem_limit: 192m
    mem_reservation: 64m
    cpus: 0.5
    pids_limit: 128
    command: >
      sh -c '
        exec redis-server
        --save 60 1
        --appendonly yes
        --appendfsync everysec
        --maxmemory 96mb
        --maxmemory-policy noeviction
        --maxclients 1000
        ${REDIS_PASSWORD:+--requirepass "$REDIS_PASSWORD"}'
```

说明：

- 基础 Compose 已把应用端口映射到 `${BIND_HOST}:${SERVER_PORT}`；`.env` 中设置 `BIND_HOST=127.0.0.1`，避免公网绕过 Nginx。
- PostgreSQL 和 Redis 没有映射宿主机端口，保持现状。
- `noeviction` 比随机淘汰业务锁、队列和状态更安全；达到上限时会显式报错，届时应扩容或排查异常数据。
- 默认请求体上限从 256 MiB 收紧到 64 MiB。确有大图上传需求时再按监控结果调整。
- 图片生成暂时限制单并发，防止 base64 图片和上游响应造成瞬时 OOM。
- Compose overlay 的 `environment` 会覆盖基础文件中的同名变量。

先检查最终合并配置，重点确认镜像、端口、目录和密码没有被错误展开：

```bash
cd /opt/sub2api
docker compose -f docker-compose.yml -f compose.my2g.yml config
```

注意：该命令会输出环境变量值，不要把输出发到公开日志。

## 创建生产 `.env`

```bash
cd /opt/sub2api
cp .env.example .env
chmod 600 .env

POSTGRES_PASSWORD="$(openssl rand -hex 32)"
JWT_SECRET="$(openssl rand -hex 32)"
TOTP_ENCRYPTION_KEY="$(openssl rand -hex 32)"
ADMIN_PASSWORD="$(openssl rand -hex 24)"

sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=${POSTGRES_PASSWORD}/" .env
sed -i "s/^JWT_SECRET=.*/JWT_SECRET=${JWT_SECRET}/" .env
sed -i "s/^TOTP_ENCRYPTION_KEY=.*/TOTP_ENCRYPTION_KEY=${TOTP_ENCRYPTION_KEY}/" .env
sed -i "s/^ADMIN_PASSWORD=.*/ADMIN_PASSWORD=${ADMIN_PASSWORD}/" .env

sed -i 's/^BIND_HOST=.*/BIND_HOST=127.0.0.1/' .env
sed -i 's/^SERVER_PORT=.*/SERVER_PORT=8080/' .env
sed -i 's/^DATABASE_MAX_OPEN_CONNS=.*/DATABASE_MAX_OPEN_CONNS=30/' .env
sed -i 's/^DATABASE_MAX_IDLE_CONNS=.*/DATABASE_MAX_IDLE_CONNS=5/' .env
sed -i 's/^REDIS_POOL_SIZE=.*/REDIS_POOL_SIZE=64/' .env
sed -i 's/^REDIS_MIN_IDLE_CONNS=.*/REDIS_MIN_IDLE_CONNS=4/' .env

unset POSTGRES_PASSWORD JWT_SECRET TOTP_ENCRYPTION_KEY ADMIN_PASSWORD
```

还需手工修改：

```dotenv
ADMIN_EMAIL=实际管理员邮箱
REDIS_PASSWORD=另一个随机强密码
TZ=Asia/Shanghai
RUN_MODE=standard
```

将 `.env` 安全地离线备份。JWT 和 TOTP key 丢失会分别导致现有登录态失效和 2FA 数据不可恢复。

## 首次启动

```bash
cd /opt/sub2api
mkdir -p data postgres_data redis_data backups
chmod 700 backups

docker compose -f docker-compose.yml -f compose.my2g.yml pull
docker compose -f docker-compose.yml -f compose.my2g.yml up -d
docker compose -f docker-compose.yml -f compose.my2g.yml ps
docker compose -f docker-compose.yml -f compose.my2g.yml logs --tail=200 sub2api
```

本机验证：

```bash
curl -fsS http://127.0.0.1:8080/health
docker stats --no-stream
free -h
df -h /
```

必须确认：

- 三个容器均为 `healthy`。
- `8080` 只监听 `127.0.0.1`。
- PostgreSQL `5432` 和 Redis `6379` 未监听宿主机公网地址。
- 没有持续重启、OOM、数据库连接耗尽或 Redis 内存错误。

## Nginx 与 HTTPS

先确认域名已经解析到服务器。下列命令中的 `api.example.com` 必须先替换为实际域名，然后创建 ACME 目录，并用仅监听 HTTP 的临时配置签发证书：

```bash
mkdir -p /var/www/certbot

cat >/etc/nginx/conf.d/sub2api.conf <<'EOF'
server {
    listen 80;
    listen [::]:80;
    server_name api.example.com;

    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    location / {
        proxy_pass http://127.0.0.1:8080;
    }
}
EOF

nginx -t
systemctl enable --now nginx

certbot certonly --webroot \
  -w /var/www/certbot \
  -d api.example.com
```

证书签发成功后，把 `/etc/nginx/conf.d/sub2api.conf` 替换为：

```nginx
server {
    listen 80;
    listen [::]:80;
    server_name api.example.com;

    location /.well-known/acme-challenge/ {
        root /var/www/certbot;
    }

    location / {
        return 301 https://$host$request_uri;
    }
}

server {
    listen 443 ssl http2;
    listen [::]:443 ssl http2;
    server_name api.example.com;

    ssl_certificate /etc/letsencrypt/live/api.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/api.example.com/privkey.pem;

    client_max_body_size 64m;
    underscores_in_headers on;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_set_header Connection "";
        proxy_buffering off;
        proxy_request_buffering off;
        proxy_cache off;

        proxy_connect_timeout 30s;
        proxy_send_timeout 1800s;
        proxy_read_timeout 1800s;
    }
}
```

`underscores_in_headers on` 是项目明确要求，缺失时 Nginx 会丢弃 `session_id` 等带下划线的请求头，影响粘性会话。关闭代理缓冲并增加超时是为了支持 SSE/流式响应和长时间图片请求。

也可使用阿里云证书代替 Certbot。正式启用前：

```bash
nginx -t
systemctl enable --now nginx
curl -fsS https://api.example.com/health
systemctl status certbot-renew.timer --no-pager || true
```

在 HTTPS 可用前，不要把管理后台或 API key 暴露到明文公网。

## 防火墙和安全组

阿里云安全组入方向建议：

| 端口 | 来源 | 用途 |
| --- | --- | --- |
| 22/tcp | 固定管理 IP，避免 `0.0.0.0/0` | SSH |
| 80/tcp | `0.0.0.0/0`、`::/0` | ACME/HTTPS 跳转 |
| 443/tcp | `0.0.0.0/0`、`::/0` | 业务流量 |

主机防火墙可作为第二层保护：

```bash
systemctl enable --now firewalld
firewall-cmd --permanent --add-service=ssh
firewall-cmd --permanent --add-service=http
firewall-cmd --permanent --add-service=https
firewall-cmd --reload
firewall-cmd --list-all
```

启用前先确认当前 SSH 会话不会被阻断，并保留一个已登录终端。

## 备份

仅打包正在运行的 PostgreSQL 数据目录不能视为可靠逻辑备份。推荐每天使用 `pg_dump`，同时备份应用数据、Redis 持久化文件和配置。

```bash
cd /opt/sub2api
set -a
. ./.env
set +a

stamp="$(date +%Y%m%d-%H%M%S)"

docker compose -f docker-compose.yml -f compose.my2g.yml exec -T postgres \
  pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc \
  > "backups/postgres-${stamp}.dump"

docker compose -f docker-compose.yml -f compose.my2g.yml exec -T redis \
  redis-cli BGSAVE

tar --exclude='data/logs' \
  -czf "backups/files-${stamp}.tar.gz" \
  .env compose.my2g.yml data redis_data

unset POSTGRES_PASSWORD JWT_SECRET TOTP_ENCRYPTION_KEY ADMIN_PASSWORD REDIS_PASSWORD
find backups -type f -mtime +7 -delete
```

至少每天将备份复制到另一台机器或对象存储；只留在同一块系统盘不算灾备。每月执行一次恢复演练。

## 升级与回滚

升级前：

1. 创建 PostgreSQL 和文件备份。
2. 阅读目标版本迁移说明。
3. 拉取固定版本镜像。
4. 记录旧镜像标签和 digest。

```bash
cd /opt/sub2api

# 修改 compose.my2g.yml 中的固定镜像版本后
docker compose -f docker-compose.yml -f compose.my2g.yml pull sub2api
docker compose -f docker-compose.yml -f compose.my2g.yml up -d sub2api
docker compose -f docker-compose.yml -f compose.my2g.yml logs --tail=200 sub2api
curl -fsS http://127.0.0.1:8080/health
```

应用回滚时改回旧 digest 并重建容器：

```bash
docker compose -f docker-compose.yml -f compose.my2g.yml up -d sub2api
```

如果新版本执行了不可逆数据库迁移，仅回滚镜像可能无效，必须按该版本说明恢复数据库备份。

## 日常检查

```bash
cd /opt/sub2api
docker compose -f docker-compose.yml -f compose.my2g.yml ps
docker compose -f docker-compose.yml -f compose.my2g.yml logs --since=30m
docker stats --no-stream
free -h
swapon --show
df -h /
journalctl -k --since today | grep -Ei 'out of memory|killed process' || true
```

建议告警阈值：

- 内存持续超过 85%。
- Swap 持续使用超过 512 MiB，而不是短时峰值。
- 根磁盘超过 75%。
- 任一容器反复重启或不健康。
- PostgreSQL 连接长期接近 30。
- Redis `used_memory` 接近 96 MiB。
- API 5xx、上游超时或请求排队持续上升。

## 参考

- Docker Engine CentOS 安装文档：<https://docs.docker.com/engine/install/centos/>
- Docker Compose plugin 安装文档：<https://docs.docker.com/compose/install/linux/>
- GitHub Container Registry 登录与权限：<https://docs.github.com/packages/working-with-a-github-packages-registry/working-with-the-container-registry>
- 项目 Docker Compose：`deploy/docker-compose.local.yml`
- 项目环境变量模板：`deploy/.env.example`
