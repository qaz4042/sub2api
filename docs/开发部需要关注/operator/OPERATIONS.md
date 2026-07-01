# 运维提示

## Nginx 反代

Nginx 反代 Sub2API 或 CRS，并配合 Codex CLI 使用时，需要在 `http` 块启用：

```nginx
underscores_in_headers on;
```

Nginx 默认可能丢弃 `session_id` 等含下划线的请求头，导致粘性会话异常。

## URL 与响应头安全

如果关闭 URL allowlist 或响应头过滤，需要在网络层补足限制：

- 限制允许访问的上游域名和 IP。
- 阻断私网、回环、链路本地地址。
- 生产环境强制 HTTPS/TLS 出站。
- 在代理层剥离敏感上游响应头。

允许 HTTP URL 仅建议用于本地开发或可信内网测试，不建议用于生产环境。

## 日志与升级

systemd 部署查看日志：

```bash
sudo journalctl -u sub2api -f
```

Docker Compose 部署查看日志：

```bash
docker compose logs -f sub2api
```

Docker Compose 升级：

```bash
docker compose pull
docker compose up -d
```
