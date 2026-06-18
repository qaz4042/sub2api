# my2g 部署方案

检查日期：2026-06-18  
服务器：`ssh my2g`
代码基线：`qaz4042/sub2api@0389a6d4caead78311a3fe4f14ec855af7f827d6`

## 部署原则：较小闭环较佳实践

本文长期采用“较小闭环较佳实践”：

1. 每个阶段只解决一个主要问题。
2. 每个阶段必须有明确验收命令。
3. 验收通过后再进入下一阶段。
4. 失败时先在当前阶段修复或回滚，不叠加新变量。
5. 镜像、配置和数据都必须可追踪、可备份、可回滚。
6. 首次部署只启用必要组件；监控证明有需要后再扩容或调优。

部署闭环：

```text
发布固定镜像
  → 准备服务器
  → 验证镜像拉取
  → 启动内网服务
  → 配置 HTTPS
  → 建立备份
  → 小流量运行与观察
```

## 推荐架构

```text
GitHub Actions → GHCR 固定版本镜像
                       ↓
Internet → Nginx HTTPS → Sub2API
                            ├── PostgreSQL
                            └── Redis
```

- Sub2API、PostgreSQL、Redis 使用 Docker Compose。
- Nginx 安装在宿主机，只开放 `80/443`。
- Sub2API 的 `8080` 只绑定 `127.0.0.1`。
- PostgreSQL 和 Redis 只使用 Compose 内部网络。
- 数据存放在 `/opt/sub2api` 下的本地目录，方便备份和迁移。

## 已确认现状

| 项目 | 结果 |
| --- | --- |
| 系统 | Alibaba Cloud Linux 3，x86_64 |
| CPU / 内存 | 2 vCPU / 1.8 GiB |
| Swap | 无，部署前需增加 2 GiB |
| 磁盘 | 40 GiB，剩余约 31 GiB |
| 当前端口 | 仅监听 `22` |
| Docker / Compose | 未安装 |
| Nginx | 未安装 |
| GHCR | 可访问 |
| Docker Hub | 连接超时 |
| OOM | 近 30 天未发现 |

结论：可以承载初期低并发业务，但内存余量较小。需要限制连接池、图片并发和容器内存。业务量增加后优先升级到 4 GiB，或迁移 PostgreSQL。

## 部署前置清单

- [ ] 域名已解析到 my2g。
- [ ] 阿里云安全组只开放 `22`、`80`、`443`。
- [ ] 当前代码已发布为固定 GHCR 镜像。
- [ ] PostgreSQL、Redis 镜像具备可用拉取源。
- [ ] 密钥和 `.env` 有安全的离线副本。
- [ ] 备份可复制到服务器之外。

---

## 闭环 1：发布固定镜像

当前 fork 没有 Release，且仓库默认的 `weishaw/sub2api:latest` 不是当前项目代码，不能直接部署。

从确定的提交创建版本标签：

```bash
git status
git log -1 --oneline

git tag -a v0.0.0-my2g.20260618.1 \
  0389a6d4caead78311a3fe4f14ec855af7f827d6 \
  -m "my2g deployment build"
git push origin v0.0.0-my2g.20260618.1
```

等待 GitHub Actions 的 `Release` workflow 完成。

预期镜像：

```text
ghcr.io/qaz4042/sub2api:0.0.0-my2g.20260618.1
```

验收：

```bash
docker pull ghcr.io/qaz4042/sub2api:0.0.0-my2g.20260618.1
docker inspect ghcr.io/qaz4042/sub2api:0.0.0-my2g.20260618.1 \
  --format '{{index .RepoDigests 0}}'
```

通过标准：能够拉取，并记录镜像 digest。首次启动即使用 digest 固定镜像：

```yaml
image: ghcr.io/qaz4042/sub2api@sha256:实际摘要
```

如果 package 为私有：

```bash
read -rsp 'GHCR token: ' CR_PAT
echo
printf '%s' "$CR_PAT" | docker login ghcr.io -u qaz4042 --password-stdin
unset CR_PAT
```

PAT 只授予 `read:packages`，不要写入项目 `.env`。

---

## 闭环 2：准备服务器基础环境

以下命令在 `ssh my2g` 后执行。

本节应可安全重复执行；已有配置先检查或备份，不重复追加、不直接覆盖。

### 2.1 安装基础包

```bash
dnf makecache
dnf update -y
dnf install -y ca-certificates curl openssl yum-utils nginx \
  certbot python3-certbot-nginx
```

如果更新了内核，重启后再继续。

### 2.2 增加 2 GiB Swap

```bash
if [ ! -f /swapfile ]; then
  fallocate -l 2G /swapfile
  chmod 600 /swapfile
  mkswap /swapfile
fi

swapon --show=NAME | grep -qx /swapfile || swapon /swapfile
grep -qF '/swapfile none swap sw 0 0' /etc/fstab ||
  printf '/swapfile none swap sw 0 0\n' >> /etc/fstab

cat >/etc/sysctl.d/99-sub2api.conf <<'EOF'
vm.swappiness=10
vm.overcommit_memory=1
EOF

sysctl --system
```

### 2.3 安装 Docker

服务器访问 Docker 官方 CDN 不稳定，使用阿里云 Docker CE 镜像站的 CentOS 8 兼容仓库：

```bash
test -f /etc/yum.repos.d/docker-ce.repo ||
  yum-config-manager --add-repo \
    https://mirrors.aliyun.com/docker-ce/linux/centos/docker-ce.repo
sed -i 's#\\$releasever#8#g' /etc/yum.repos.d/docker-ce.repo

dnf makecache
dnf install -y docker-ce docker-ce-cli containerd.io \
  docker-buildx-plugin docker-compose-plugin

systemctl enable --now docker
```

验收：

```bash
free -h
swapon --show
docker version
docker compose version
```

通过标准：Swap 为 2 GiB，Docker 和 Compose 均正常。

---

## 闭环 3：打通所有镜像拉取

当前 Docker Hub 连接超时，而基础 Compose 依赖：

```text
postgres:18-alpine
redis:8-alpine
```

优先在阿里云容器镜像服务获取专属加速地址，配置 `/etc/docker/daemon.json`：

如果该文件已存在，先备份并合并原配置，不要直接覆盖：

```bash
test ! -f /etc/docker/daemon.json ||
  test -f /etc/docker/daemon.json.bak ||
  cp -a /etc/docker/daemon.json /etc/docker/daemon.json.bak
```

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

```bash
dockerd --validate --config-file=/etc/docker/daemon.json
systemctl restart docker

docker pull postgres:18-alpine
docker pull redis:8-alpine
docker pull ghcr.io/qaz4042/sub2api:0.0.0-my2g.20260618.1

docker image inspect postgres:18-alpine redis:8-alpine \
  ghcr.io/qaz4042/sub2api:0.0.0-my2g.20260618.1 \
  --format '{{index .RepoDigests 0}}'
```

通过标准：三个镜像全部拉取成功，并记录各自 digest。首次启动前将 Compose 中三个镜像都改为 `镜像名@sha256:摘要`。

如果 Docker Hub 加速仍不稳定，应在 CI 中把 PostgreSQL 和 Redis 同步到自己的 GHCR，再替换 Compose 镜像地址。不要使用来源不明的公共代理镜像。

---

## 闭环 4：启动本机服务

### 4.1 准备部署文件

本地执行：

```bash
ssh my2g 'install -d -m 0750 /opt/sub2api'
scp deploy/docker-compose.local.yml my2g:/opt/sub2api/docker-compose.yml
scp deploy/.env.example my2g:/opt/sub2api/.env.example
```

不在 my2g 上构建源码：机器内存较小，且服务器构建不利于产物追踪。

### 4.2 创建生产配置

服务器执行：

```bash
cd /opt/sub2api
cp .env.example .env
chmod 600 .env

POSTGRES_PASSWORD="$(openssl rand -hex 32)"
REDIS_PASSWORD="$(openssl rand -hex 32)"
JWT_SECRET="$(openssl rand -hex 32)"
TOTP_ENCRYPTION_KEY="$(openssl rand -hex 32)"
ADMIN_PASSWORD="$(openssl rand -hex 24)"

sed -i "s/^POSTGRES_PASSWORD=.*/POSTGRES_PASSWORD=${POSTGRES_PASSWORD}/" .env
sed -i "s/^REDIS_PASSWORD=.*/REDIS_PASSWORD=${REDIS_PASSWORD}/" .env
sed -i "s/^JWT_SECRET=.*/JWT_SECRET=${JWT_SECRET}/" .env
sed -i "s/^TOTP_ENCRYPTION_KEY=.*/TOTP_ENCRYPTION_KEY=${TOTP_ENCRYPTION_KEY}/" .env
sed -i "s/^ADMIN_PASSWORD=.*/ADMIN_PASSWORD=${ADMIN_PASSWORD}/" .env
sed -i 's/^BIND_HOST=.*/BIND_HOST=127.0.0.1/' .env

unset POSTGRES_PASSWORD REDIS_PASSWORD JWT_SECRET \
  TOTP_ENCRYPTION_KEY ADMIN_PASSWORD
```

手工修改 `.env`：

```dotenv
ADMIN_EMAIL=实际管理员邮箱
TZ=Asia/Shanghai
RUN_MODE=standard
```

### 4.3 添加 my2g 资源限制

创建 `/opt/sub2api/compose.my2g.yml`：

```yaml
services:
  sub2api:
    image: ghcr.io/qaz4042/sub2api@sha256:实际摘要
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
    image: postgres@sha256:实际摘要
    mem_limit: 448m
    mem_reservation: 256m
    cpus: 1.0
    command:
      - postgres
      - -c
      - max_connections=80
      - -c
      - shared_buffers=128MB
      - -c
      - work_mem=2MB

  redis:
    image: redis@sha256:实际摘要
    mem_limit: 192m
    mem_reservation: 64m
    cpus: 0.5
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

### 4.4 启动并验收

```bash
cd /opt/sub2api
mkdir -p data postgres_data redis_data backups
chmod 700 backups

docker compose -f docker-compose.yml -f compose.my2g.yml config >/dev/null
docker compose -f docker-compose.yml -f compose.my2g.yml up -d

docker compose -f docker-compose.yml -f compose.my2g.yml ps
curl -fsS http://127.0.0.1:8080/health
ss -lntp
docker stats --no-stream
```

通过标准：

- 三个容器均为 `healthy`。
- `8080` 只监听 `127.0.0.1`。
- 宿主机不监听 `5432`、`6379`。
- 没有容器持续重启或 OOM。

失败回滚：

```bash
docker compose -f docker-compose.yml -f compose.my2g.yml down
```

此时不要删除数据目录。

---

## 闭环 5：接入 HTTPS

先确认域名已解析，并在阿里云安全组开放 `80/443`。

建议同时启用宿主机防火墙，只允许 SSH、HTTP 和 HTTPS：

```bash
dnf install -y firewalld
systemctl enable --now firewalld
firewall-cmd --permanent --add-service={ssh,http,https}
firewall-cmd --reload
firewall-cmd --list-all
```

保持当前 SSH 会话，在另一个终端确认仍能登录后再继续。

先创建最小 HTTP 配置，再用 Certbot 签发证书：

```bash
cat >/etc/nginx/conf.d/sub2api.conf <<'EOF'
server {
    listen 80;
    server_name api.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
    }
}
EOF

nginx -t
systemctl enable --now nginx
certbot --nginx -d api.example.com
```

在 Nginx 的 HTTPS `server` 中确认包含：

```nginx
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
    proxy_read_timeout 1800s;
    proxy_send_timeout 1800s;
}
```

`underscores_in_headers on` 是项目要求，否则 `session_id` 等请求头可能被丢弃。

验收：

```bash
nginx -t
systemctl reload nginx
curl -fsS https://api.example.com/health
certbot renew --dry-run
systemctl list-timers '*certbot*'
```

再从服务器之外验证：

```bash
nmap -Pn -p 22,80,443,8080,5432,6379 api.example.com
```

通过标准：

- HTTPS 健康检查和证书续期测试成功。
- 公网仅开放 `22/80/443`，不能访问 `8080/5432/6379`。
- 阿里云安全组与宿主机防火墙规则一致。

---

## 闭环 6：建立备份与恢复能力

### 6.1 PostgreSQL 日常备份

使用临时文件，校验成功后再改为正式文件：

```bash
cd /opt/sub2api
set -e
set -a
. ./.env
set +a

stamp="$(date +%Y%m%d-%H%M%S)"
tmp="backups/.postgres-${stamp}.dump.tmp"
backup="backups/postgres-${stamp}.dump"

docker compose -f docker-compose.yml -f compose.my2g.yml exec -T postgres \
  pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc > "$tmp"
test -s "$tmp"
docker compose -f docker-compose.yml -f compose.my2g.yml exec -T postgres \
  pg_restore --list < "$tmp" >/dev/null
mv "$tmp" "$backup"

tar -czf "backups/files-${stamp}.tar.gz" \
  .env docker-compose.yml compose.my2g.yml data

unset POSTGRES_PASSWORD REDIS_PASSWORD JWT_SECRET \
  TOTP_ENCRYPTION_KEY ADMIN_PASSWORD
```

通过标准：备份校验成功，并加密复制到服务器之外。只保存在 my2g 本机不算完整备份。

### 6.2 PostgreSQL 恢复演练

首次上线后单独完成一次恢复演练，不放入每日备份脚本。使用临时数据库恢复，确认业务表可读后删除。

通过标准：最新备份能够恢复，且不影响生产数据库。

### 6.3 Redis 数据策略

- 如果 Redis 仅保存可重建的缓存，不备份。
- 如果 Redis 保存会话、队列等不可重建状态，再单独建立一致性备份闭环。

---

## 闭环 7：小流量观察

首次上线后先小流量运行 24 小时，不立即增加并发或修改多个参数。

```bash
cd /opt/sub2api
docker compose -f docker-compose.yml -f compose.my2g.yml ps
docker compose -f docker-compose.yml -f compose.my2g.yml logs --since=30m
docker stats --no-stream
free -h
swapon --show
df -h /
journalctl -k --since today |
  grep -Ei 'out of memory|killed process' || true
```

需要处理的信号：

- 内存持续超过 85%。
- Swap 持续超过 512 MiB。
- 磁盘超过 75%。
- 容器重启或健康检查失败。
- Redis 接近 96 MiB。
- PostgreSQL 连接接近 30。
- API 5xx 或上游超时持续增加。

每次只调整一个主要参数，观察一个完整业务周期后再继续。这是“较小闭环较佳实践”在运行期的核心要求。

## 升级与回滚

升级也按小闭环执行：

1. 备份。
2. 记录旧镜像 digest。
3. 修改为新的固定镜像。
4. 只重建 Sub2API。
5. 验证健康检查和核心请求。
6. 失败立即改回旧 digest。

```bash
cd /opt/sub2api
docker compose -f docker-compose.yml -f compose.my2g.yml pull sub2api
docker compose -f docker-compose.yml -f compose.my2g.yml up -d sub2api
curl -fsS http://127.0.0.1:8080/health
```

如果版本包含不可逆数据库迁移，回滚镜像不足以恢复，必须同时使用数据库备份。

## 最终验收

- [ ] Sub2API、PostgreSQL、Redis 均固定到已记录的 digest。
- [ ] 三个容器均健康。
- [ ] 只有 `22/80/443` 对外开放。
- [ ] HTTPS、流式响应和核心 API 可用。
- [ ] Swap、内存和磁盘处于安全范围。
- [ ] 备份已复制到服务器之外并验证可恢复。
- [ ] 升级和回滚步骤已记录。

## 参考

- Docker Engine：<https://docs.docker.com/engine/install/centos/>
- Docker Compose：<https://docs.docker.com/compose/install/linux/>
- GHCR：<https://docs.github.com/packages/working-with-a-github-packages-registry/working-with-the-container-registry>
- 基础 Compose：`deploy/docker-compose.local.yml`
- 环境变量模板：`deploy/.env.example`
