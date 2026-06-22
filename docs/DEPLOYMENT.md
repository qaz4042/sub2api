# 部署说明

本文只保留部署入口和常用命令，完整配置以 `deploy/` 下文件为准。

## 脚本安装

适合已有 PostgreSQL 15+ 与 Redis 7+ 的 Linux 服务器：

```bash
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/install.sh | sudo bash
```

安装后常用命令：

```bash
sudo systemctl start sub2api
sudo systemctl enable sub2api
sudo systemctl status sub2api
sudo journalctl -u sub2api -f
```

浏览器访问 `http://YOUR_SERVER_IP:8080` 完成初始化。

## Docker Compose

适合希望一并启动 PostgreSQL、Redis 和 Sub2API 的场景：

```bash
mkdir -p sub2api-deploy && cd sub2api-deploy
curl -sSL https://raw.githubusercontent.com/Wei-Shaw/sub2api/main/deploy/docker-deploy.sh | bash
docker compose up -d
```

常用命令：

```bash
docker compose ps
docker compose logs -f sub2api
docker compose pull
docker compose up -d
docker compose down
```

如需手动配置，参考 `deploy/.env.example`、`deploy/docker-compose.yml` 与 `deploy/docker-compose.local.yml`。

## 源码构建

```bash
cd frontend
pnpm install
pnpm run build
cd ../backend
go build -tags embed -o sub2api ./cmd/server
cp ../deploy/config.example.yaml ./config.yaml
./sub2api
```

`-tags embed` 会把前端构建产物嵌入后端二进制，否则后端不会直接提供前端 UI。
