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

## 备注

- DNS 和 HTTPS 已满足当前访问要求。
- 当前公网 HTTPS 访问依赖 Cloudflare Tunnel 服务，需保持 `cloudflared` 正常运行。
- 如需登录后台，请从 `/opt/sub2api/.env` 查看 `ADMIN_EMAIL` 和 `ADMIN_PASSWORD`。
- 国内服务器访问 OpenAI 的代理方案见：[my2g-openai-proxy-runbook.md](./my2g-openai-proxy-runbook.md)。
