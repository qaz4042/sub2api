# Sub2API 从 my2g 迁移到 my4g 结果

迁移日期：2026-06-23。

## 当前结论

Sub2API 已从北京 `my2g` 迁移到香港 `my4g`，当前正式访问域名为：

- 主页：<https://codex.lizubin.online/>
- 健康检查：<https://codex.lizubin.online/health>

公网首页、健康检查和公开设置 API 均已验证返回 HTTP 200。

## my4g 当前部署

- SSH 别名：`my4g`
- 公网 IP：`152.32.190.110`
- 系统：Debian 12（bookworm）
- 架构：Linux amd64
- 配置：4 GiB 内存、80 GiB 系统盘
- 应用 release：`/opt/sub2api/releases/20260623-053550-64b6d9a2-dirty`
- 当前软链接：`/opt/sub2api/current`

运行结构：

```text
Cloudflare
  -> codex.lizubin.online
  -> my4g:443
  -> Nginx
  -> 127.0.0.1:8080
  -> sub2api.service

Docker Compose:
  sub2api-postgres -> 127.0.0.1:5432
  sub2api-redis    -> 127.0.0.1:6379
  sub2api-mihomo   -> 127.0.0.1:7890/7891
```

应用继续使用“前端嵌入 Go 二进制 + systemd”的部署方式。PostgreSQL、Redis 和 Mihomo 由 Docker Compose 运行。

## 数据迁移

迁移期间没有停止 `my2g`。按照迁移前约定，使用迁移时点的 PostgreSQL 在线快照，忽略切换窗口的数据差异。

- 快照文件：`/opt/sub2api/backups/postgres-my4g-migration-20260623.dump`
- SHA-256：`5a54cffa8845ef4cba1a8e2538643f2993876992883cf0df8ba4c3fdc19b1f56`
- 恢复后 `public` schema：75 张表

以下运行内容也已复制：

- 当前应用 release 和 `resources/`
- systemd 环境变量
- 共享运行数据
- Redis 数据
- Mihomo 配置
- Docker Compose 配置

数据库中的域名相关设置已更新：

```text
api_base_url=https://codex.lizubin.online
frontend_url=https://codex.lizubin.online
google_oauth_redirect_url=https://codex.lizubin.online/api/v1/auth/oauth/google/callback
balance_low_notify_recharge_url=https://codex.lizubin.online
```

## Cloudflare 与 HTTPS

`codex.lizubin.online` 原本仍指向 `codex-my2g` Cloudflare Tunnel。迁移时已完成以下调整：

1. 删除旧 Tunnel CNAME。
2. 创建已代理的 A 记录，指向 `152.32.190.110`。
3. 在 Cloudflare 创建 Origin Certificate。
4. 将证书安装到 `my4g` Nginx。
5. Cloudflare SSL/TLS 模式设置为 Full (strict)。
6. HTTP 入口重定向到 HTTPS。

`my4g` 不安装或依赖 cloudflared，公网链路为 Cloudflare 代理直连 Nginx。

Nginx 配置模板：

```text
deploy/nginx.my4g.conf
```

## 中国大陆直连测试入口

为绕过 Cloudflare 全球网络在中国大陆的线路波动，已新增灰度测试域名：

- 主页：<https://codex-direct.lizubin.online/>
- 健康检查：<https://codex-direct.lizubin.online/health>
- 中性域名对照：<https://edge.lizubin.online/>

链路如下：

```text
codex-direct.lizubin.online / edge.lizubin.online
  -> Cloudflare DNS only
  -> 152.32.190.110:443
  -> my4g Nginx
  -> 127.0.0.1:8080
```

该域名使用 Let's Encrypt 公共证书，不经过 Cloudflare 代理。证书由
`certbot.timer` 自动续期，并通过以下 deploy hook 检查和重载 Nginx：

```text
/etc/letsencrypt/renewal-hooks/deploy/reload-nginx.sh
```

对应仓库脚本：

```text
deploy/certbot-reload-nginx.sh
```

2026-06-23 从北京 `my2g` 实测：

- 直连域名连续 5 次返回 HTTP 200。
- DNS 缓存后总耗时约 0.15 秒。
- 同一节点访问 Cloudflare 正式域名总耗时约 0.96–1.44 秒。

同日从实际家庭宽带绕过本机 FlClash、强制使用 Wi-Fi 接口复测：

- `152.32.190.110:22` 可以连接。
- `152.32.190.110:80/443` 在请求进入 Nginx 前被重置。
- `edge.lizubin.online` 与 `codex-direct.lizubin.online` 结果相同，排除 `codex` 域名关键字影响。
- Cloudflare 的 `104.21.91.130`、`172.67.219.205` 也在 TLS 阶段前被该宽带线路重置。
- 北京 `my2g` 直连入口返回 `Non-compliance ICP Filing`，未备案时不能作为国内反向代理入口。

因此当前问题是家庭运营商到现有 Cloudflare/UCloud 公网 IP 的线路限制，修改 DNS、证书或 Nginx
无法解决。下一步需要更换 `my4g` 公网 IP/香港线路，或完成备案后使用中国大陆入口。

`edge.lizubin.online` 不含 `codex` 关键字，用于区分运营商线路/IP 问题和 TLS SNI 干扰。
正式域名 `codex.lizubin.online` 暂不切换。

### 当前阶段结论

项目仍处于初创阶段，当前选择低成本的小闭环方案：

- 不购买高成本精品 BGP，不将未备案的 `my2g` 用作公开 Web 反向代理。
- 正式服务继续运行在 `my4g`，国内公开访问暂时仍可能需要可用代理。
- 管理员和少量内部用户可通过 `my2g:22` 建立 SSH SOCKS5 私有桥接，只转发
  `codex-direct.lizubin.online`、`edge.lizubin.online` 等 Sub2API 流量。
- 本地隧道建议绑定 `127.0.0.1`，使用独立低权限 SSH 用户、密钥登录、PAC 分流和自动重连；
  不开放 `my2g` 的 80、443 或 8443 作为公共入口。
- 当国内访问问题已经明确影响付费转化时，再按顺序评估低成本香港优化线路边缘节点、
  UCloud 精品 BGP，以及备案后的中国大陆入口。

私有桥接示例：

```bash
ssh -NT \
  -D 127.0.0.1:1088 \
  -o ExitOnForwardFailure=yes \
  -o ServerAliveInterval=30 \
  -o ServerAliveCountMax=3 \
  my2g
```

该方案不依赖第三方 VPN，但仍是面向内部用户的 SSH 私有代理，不能作为普通用户无需配置即可访问的正式入口。

## 后续发布

在本地仓库根目录执行：

```bash
make deploy-my4g
```

该命令执行：

1. 检查远端 systemd 环境。
2. 构建前端。
3. 构建嵌入前端的 Linux amd64 Go 二进制。
4. 上传独立 release。
5. 原子切换 `/opt/sub2api/current`。
6. 重启 `sub2api.service`。
7. 执行本机及公网健康检查。
8. 本机健康检查失败时自动回滚。

如需禁止发布未提交的工作区：

```bash
REQUIRE_CLEAN=1 make deploy-my4g
```

## my2g 退役状态

`my2g` 上的 Sub2API 后端已经停止并禁用：

```text
ActiveState=inactive
SubState=dead
UnitFileState=disabled
```

`127.0.0.1:8080` 已关闭。

旧域名 <https://portal.lizubin.online/> 继续通过原 Cloudflare Tunnel 访问 `my2g` Nginx，但只返回静态迁移提示页：

```text
站点域名已迁移
https://codex.lizubin.online/
```

静态页和 Nginx 配置模板：

```text
deploy/my2g-retired-index.html
deploy/nginx.my2g-retired.conf
```

退役前配置备份：

```text
/opt/sub2api/backups/retirement-20260623-153357
```

为便于回查和短期回滚，`my2g` 的以下容器暂时保留运行：

- PostgreSQL
- Redis
- Mihomo

确认不再需要旧数据后，可以另行停止并清理这些容器及服务器。

## 验证命令

```bash
# 新站公网健康检查
curl -fsS https://codex.lizubin.online/health

# 中国大陆直连测试入口
curl -fsS https://codex-direct.lizubin.online/health

# my4g 服务状态
ssh my4g 'systemctl status sub2api nginx docker'

# Let's Encrypt 自动续期
ssh my4g 'systemctl status certbot.timer'
ssh my4g 'certbot renew --dry-run --no-random-sleep-on-renew'

# my4g 依赖容器
ssh my4g 'docker ps --format "{{.Names}}|{{.Status}}"'

# 旧后端应保持停止
ssh my2g 'systemctl show sub2api -p ActiveState -p SubState -p UnitFileState'

# 旧站迁移提示
curl -fsS https://portal.lizubin.online/
```
