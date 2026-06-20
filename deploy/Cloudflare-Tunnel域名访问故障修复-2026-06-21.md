# Cloudflare Tunnel 域名访问故障修复（2026-06-21）

## 结论

`https://portal.lizubin.online/` 是为解决原域名连接被链路重置而新增的备用访问入口。

最终方案不是继续调整 Nginx 或证书，而是通过 **Cloudflare Tunnel** 建立服务器到 Cloudflare 的主动出站连接，避免公网直接回源和入站 TLS 链路干扰。

## 故障现象

原地址 `https://codex.lizubin.online/` 先后出现：

- Cloudflare `525 SSL handshake failed`
- 静态 JS 文件加载时报 `net::ERR_CONNECTION_CLOSED`
- TLS ClientHello 发出后连接被直接重置

排查确认：

- Nginx 配置正常。
- Let's Encrypt 源站证书有效。
- 应用和静态资源在服务器本机均返回 `200`。
- 故障发生在客户端或 Cloudflare 到源站的网络链路，不是应用文件缺失。
- `codex.lizubin.online` 主机名在部分网络中会触发连接重置。

## 对 portal.lizubin.online 所做的配置

### 1. 创建 Cloudflare Tunnel

在服务器 `my2g` 安装 `cloudflared 2026.6.1`，创建命名隧道：

```text
codex-my2g
```

隧道由服务器主动连接 Cloudflare Edge，避免 Cloudflare 通过公网 `443` 端口直接访问源站。

### 2. 添加 DNS 路由

在 Cloudflare 中新增：

```text
portal.lizubin.online -> Cloudflare Tunnel
```

域名继续由 Cloudflare 提供公网 HTTPS 和边缘接入。

### 3. 配置本机回源

隧道收到 `portal.lizubin.online` 请求后，直接转发到服务器本机应用：

```yaml
ingress:
  - hostname: portal.lizubin.online
    service: http://127.0.0.1:8080
    originRequest:
      httpHostHeader: portal.lizubin.online
  - service: http_status:404
```

该方式绕过公网回源链路，应用端口仍只绑定 `127.0.0.1:8080`。

### 4. 配置常驻服务

将 `cloudflared` 注册为 systemd 服务：

- 已设置开机自动启动。
- 服务异常退出后由 systemd 管理。
- 使用 HTTP/2 与 Cloudflare 建立多条持久连接。
- 配置文件位于 `/etc/cloudflared/config.yml`。

## 验证结果

完成配置后验证：

- `https://portal.lizubin.online/` 连续访问返回 `HTTP 200`。
- Cloudflare HTTPS 证书校验正常。
- 首页和应用 API 可访问。
- 三个曾报错的静态资源均返回 `200`：
  - `vendor-vue-DdvVI69T.js`
  - `vendor-i18n-DY-5nrdT.js`
  - `vendor-misc-DJoKcLuU.js`
- 重启 `cloudflared` 后隧道可以自动恢复。

## 当前访问建议

- 正式使用：`https://portal.lizubin.online/`
- 不建议继续使用：`https://codex.lizubin.online/`

旧域名的应用和资源本身没有问题，但其主机名在部分网络链路中仍可能被重置。Cloudflare Tunnel 能解决公网回源问题，不能修复访问者网络对特定域名的干扰，因此最终通过更换入口域名解决。

## 运维检查

```bash
systemctl status cloudflared
cloudflared tunnel info codex-my2g
cloudflared --config /etc/cloudflared/config.yml tunnel ingress validate
curl -I https://portal.lizubin.online/
```
