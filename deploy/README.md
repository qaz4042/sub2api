# Sub2API 部署目录

本目录保留部署入口、配置样例和少量当前运维说明。过时服务器和历史排障材料已移除。

## 当前生产入口

- 目标机器：`my4g`
- 服务目录：`/opt/sub2api`
- 当前版本软链：`/opt/sub2api/current`
- systemd 服务：`sub2api.service`
- 访问域名：`https://codex.lizubin.online`
- 发布健康检查：远端本机 `http://127.0.0.1:8080/health`

## 常用发布

```bash
# 前后端一起构建并发布到 my4g
make deploy-my4g

# 复用现有前端 dist，只发布后端
make deploy-my4g-backend-only

# 要求本地工作区干净后再发布
REQUIRE_CLEAN=1 make deploy-my4g
```

验证：

```bash
ssh my4g 'readlink -f /opt/sub2api/current'
ssh my4g 'systemctl status sub2api --no-pager'
```
公网访问可按需手工验证，不作为发布脚本失败条件：
```bash
curl -fsS https://codex.lizubin.online/health
```

回滚：

```bash
ssh my4g 'ls -1dt /opt/sub2api/releases/* | head'
ssh my4g 'sudo ln -sfn /opt/sub2api/releases/<old-release> /opt/sub2api/current && sudo systemctl restart sub2api'
```

发布脚本在健康检查失败时会尝试自动回滚到上一版本。更详细的当前发布步骤见 `部署指南.md`。

## 文件分组

| 类型 | 文件 |
| --- | --- |
| 当前发布 | `部署指南.md`, `deploy-my4g.sh`, `sub2api.service` |
| Docker 部署 | `DOCKER.md`, `docker-deploy.sh`, `docker-compose*.yml`, `Dockerfile`, `.env.example`, `docker-entrypoint.sh` |
| 配置样例 | `配置参考.md`, `config.example.yaml`, `Caddyfile`, `nginx.my4g.conf`, `cloudflared.my4g.*`, `sub2api.my4g-oauth-domains.conf` |
| 数据管理 | `DATAMANAGEMENTD_CN.md`, `install-datamanagementd.sh`, `sub2api-datamanagementd.service` |
| my4g 直连验证 | `setup-my4g-ip-direct.sh`, `check-my4g-ip-direct.sh`, `certbot-reload-nginx.sh` |

## Docker 快速部署

推荐使用本地目录版本，方便备份和迁移：

```bash
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/docker-deploy.sh | bash
docker compose -f docker-compose.local.yml up -d
docker compose -f docker-compose.local.yml logs -f sub2api
```

常用命令：

```bash
docker compose -f docker-compose.local.yml ps
docker compose -f docker-compose.local.yml restart sub2api
docker compose -f docker-compose.local.yml pull
docker compose -f docker-compose.local.yml up -d
```

完整 Docker 镜像与环境变量说明见 `DOCKER.md` 和 `.env.example`。

## 数据管理联动

如需启用管理后台“数据管理”功能，需要在宿主机额外部署 `datamanagementd`。主进程固定探测：

```text
/tmp/sub2api-datamanagement.sock
```

Docker 场景下需把宿主机 Socket 挂载到容器内同路径。步骤见 `DATAMANAGEMENTD_CN.md`。

## 目录整理规则

- 当前可执行入口和配置样例留在 `deploy/` 根目录。
- 新增部署文档优先更新 `README.md` 或 `部署指南.md`；只有需要长期追溯时再新增独立文档。
