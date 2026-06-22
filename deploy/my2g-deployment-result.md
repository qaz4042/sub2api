# my2g 部署结果摘要

部署目标：Sub2API 在 `my2g` 服务器完成部署，并通过 Cloudflare Tunnel 对外提供 HTTPS 访问。

## 访问地址

- 主页：<https://portal.lizubin.online/>
- 健康检查：<https://portal.lizubin.online/health>

## 当前状态

- HTTPS 已通过 Cloudflare Tunnel 接入完成。
- Nginx 当前保留 `portal.lizubin.online` 的 80 端口反代配置，不再绑定废弃入口的 443 证书。
- 主页已在浏览器验证可正常展示，标题为 `Sub2API - AI API Gateway`。
- `/health` 返回正常：`{"status":"ok"}`。
- WebSocket/长连接代理路径已验证，应用侧能正常返回认证拦截。
- Docker 服务状态正常：
  - `sub2api`：healthy
  - `sub2api-postgres`：healthy
  - `sub2api-redis`：healthy

## 服务器关键路径

- 部署目录：`/opt/sub2api/`
- Compose 主配置：`/opt/sub2api/docker-compose.yml`
- Compose 覆盖配置：`/opt/sub2api/compose.my2g.yml`
- 环境变量与密钥：`/opt/sub2api/.env`
- 备份目录：`/opt/sub2api/backups/`
- Nginx 站点配置：`/etc/nginx/conf.d/sub2api.conf`
- Cloudflare Tunnel 配置：`/etc/cloudflared/config.yml`

## 密钥查看

密钥没有写入本文档。需要查看时登录服务器读取：

```bash
ssh my2g 'sudo grep "^POSTGRES_PASSWORD=" /opt/sub2api/.env'
ssh my2g 'sudo grep "^ADMIN_PASSWORD=" /opt/sub2api/.env'
```

注意：`/opt/sub2api/.env` 权限已设置为 `root:root 600`，不要提交到 Git，也不要公开粘贴。

## 证书与入口

- 公网 HTTPS 证书由 Cloudflare 侧托管。
- `portal.lizubin.online` 通过 Cloudflare Tunnel 回源到服务器本机 `127.0.0.1:8080`。
- 服务器上的历史 Let's Encrypt 证书材料不参与当前 `portal.lizubin.online` 入口。

## 对外端口

服务器公网只需要开放：

- `22/tcp`：SSH
- `80/tcp`：Nginx HTTP 反代备用入口

PostgreSQL、Redis、应用 8080 端口不直接暴露公网。

## 镜像与部署方式

由于远端 Docker Hub 拉取不稳定，本次采用本地构建并传输镜像到服务器：

- 应用镜像：`sub2api:0.0.0-my2g.20260618.1`
- PostgreSQL：`postgres:18-alpine`
- Redis：`redis:8-alpine`

镜像在服务器上通过 `compose.my2g.yml` 固定为本地 image ID，避免运行时再次拉取外网镜像。

后续从当前 Mac 更新应用时，可在仓库根目录执行：

```bash
make deploy-my2g
```

该命令会构建 `linux/amd64` 前后端一体镜像、上传到 `my2g`、备份 Compose 配置、只重建 Sub2API 应用容器，并在健康检查失败时自动恢复上一镜像。PostgreSQL、Redis 和 Mihomo 不会随应用更新重启。

如需指定镜像版本：

```bash
TAG=0.0.0-my2g.20260622.2 make deploy-my2g
```

## 2026-06-22 部署执行结论

当前从 Mac 执行 `make deploy-my2g` 慢，主要原因不是 Makefile 本身，而是脚本内部在本地 Docker/Colima 中执行：

```bash
docker build --platform linux/amd64 ...
```

当前本机 Docker daemon 运行在 Colima，配置为 `x86_64 / 2 CPU / 4 GiB`。前端构建步骤 `vue-tsc -b && vite build` 在该环境中执行时会明显慢于 Mac 原生执行，尤其是在 Apple Silicon Mac 上构建 `linux/amd64` 镜像时更明显。

结论：

- 前端构建命令本身可以在 Mac 原生环境直接运行，不必进入 Colima。
- 但只要要在 Mac 本地构建 Linux Docker 镜像，就必须经过某种 Linux Docker daemon，例如 Colima、Docker Desktop、OrbStack、Podman machine 或远程 Linux Docker。
- 当前部署慢的是“本地 Docker 镜像构建”链路，不是前端代码或业务代码异常。
- 不建议改成直接复制 Mac 原生产物覆盖服务器，因为这会削弱 Docker 镜像部署的可追踪性和回滚能力。

较小闭环的推荐方向：

1. 保留 `make deploy-my2g` 作为一键部署入口。
2. 将 `make deploy-my2g` 的默认语义调整为“远程构建 + 部署”，即在 `my2g` 服务器上使用本机 Docker 构建目标镜像，再切换 Compose 镜像并做健康检查。
3. 可选保留 `make deploy-my2g-local` 作为备用路径，语义为“Mac 本地构建镜像、压缩上传、服务器加载并重启”。

两种路径取舍：

| 命令 | 语义 | 优点 | 缺点 | 建议用途 |
|---|---|---|---|---|
| `make deploy-my2g` | 远程构建并部署 | 不依赖 Mac/Colima 性能；构建环境与运行环境一致；省去镜像导出和上传 | 会占用 `my2g` 服务器 CPU/内存；服务器内存较小，极端情况下可能吃 swap | 日常默认路径 |
| `make deploy-my2g-local` | 本地构建、上传镜像、远端加载部署 | 不占用服务器构建资源；可作为远程构建失败时的退路 | 当前 Mac/Colima 环境较慢；链路更长；本地和远端环境差异更大 | 备用路径 |

部署过程对线上用户的影响边界：

- 构建阶段不影响线上旧容器继续服务。
- 真正影响用户的是最后 `docker compose up -d --no-deps sub2api` 切换应用容器时的一小段时间。
- PostgreSQL、Redis 和 Mihomo 不应随应用镜像更新一起重启。
- 部署脚本应保留 Compose 配置备份、健康检查和失败回滚。

当前建议：

- 短期：将 `make deploy-my2g` 改为远程构建，降低 Mac/Colima 对部署速度的影响。
- 中期：如果仍希望本地构建，可考虑用 OrbStack 或提升 Colima CPU/内存、启用更快的文件挂载方式。
- 后期：如果部署频率提高，再引入 GitHub Actions + 镜像仓库；服务器只负责拉取镜像和重启服务。

## 备注

- DNS 和 HTTPS 已满足当前访问要求。
- 当前公网 HTTPS 访问依赖 Cloudflare Tunnel 服务，需保持 `cloudflared` 正常运行。
- 如需登录后台，请从 `/opt/sub2api/.env` 查看 `ADMIN_EMAIL` 和 `ADMIN_PASSWORD`。
- 国内服务器访问 OpenAI 的代理方案见：[my2g-openai-proxy-runbook.md](./my2g-openai-proxy-runbook.md)。
