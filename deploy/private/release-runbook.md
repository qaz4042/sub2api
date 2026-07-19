# 通用发布与回滚 Runbook

本文件是脱敏模板，适用于使用 `docker-compose.local.yml` 的单机部署。仓库不保存真实域名、主机、凭证或生产快照；执行前请替换占位符，并把 `.env` 保存在服务器安全目录。

systemd 二进制部署可使用同目录的 `deploy-systemd-release.sh`。该脚本要求远端已有
`releases/` 和指向有效版本目录的 `current` 软链，并要求 service 的 `WorkingDirectory`、
`ExecStart` 经由 `current` 指向版本目录；具体参数和首次目录准备见 `deploy/README.md`。

## 变量

```bash
export DEPLOY_HOST="deploy.example.invalid"
export DEPLOY_DIR="/srv/sub2api"
export RELEASE_ID="<release-id>"
export SUB2API_IMAGE="weishaw/sub2api:<release-tag-or-digest>"
export SERVICE_PORT="8080"
```

`DEPLOY_HOST`、`DEPLOY_DIR` 和 `SUB2API_IMAGE` 只用于说明格式，不要直接照搬到生产环境。

## 发布前检查

1. 在本地完成与变更范围匹配的测试、类型检查和 `git diff --check`。
2. 确认发布镜像使用不可变的版本标签或 digest，不使用 `latest` 作为回滚依据。
3. 确认服务器上的 `.env` 已备份到受限目录，且没有被提交或打包进公开产物。
4. 发布前停止应用并备份本地数据目录；数据库迁移只向前执行，回滚需要恢复备份或执行补偿 SQL。

```bash
cd "${DEPLOY_DIR}"
docker compose \
  -f docker-compose.local.yml \
  -f private/docker-compose.release.yml \
  down

tar --exclude='./.env' -czf \
  "../sub2api-data-${RELEASE_ID}.tar.gz" \
  data postgres_data redis_data

install -m 600 .env "../sub2api-env-${RELEASE_ID}.backup"
```

## 发布与验证

```bash
cd "${DEPLOY_DIR}"
export SUB2API_IMAGE

docker compose \
  -f docker-compose.local.yml \
  -f private/docker-compose.release.yml \
  config >/dev/null

docker compose \
  -f docker-compose.local.yml \
  -f private/docker-compose.release.yml \
  pull sub2api

docker compose \
  -f docker-compose.local.yml \
  -f private/docker-compose.release.yml \
  up -d

curl --fail --silent --show-error \
  --connect-timeout 5 --max-time 15 \
  "http://127.0.0.1:${SERVICE_PORT}/health"
```

若健康检查失败，先查看 `docker compose ... logs --tail=200 sub2api`，不要在没有确认数据状态前重复执行迁移或删除数据目录。

## 回滚

保留上一版本的 `SUB2API_IMAGE` 和数据备份后，重新执行发布步骤：

```bash
export SUB2API_IMAGE="weishaw/sub2api:<previous-release-tag-or-digest>"

cd "${DEPLOY_DIR}"
docker compose \
  -f docker-compose.local.yml \
  -f private/docker-compose.release.yml \
  up -d

curl --fail --silent --show-error \
  --connect-timeout 5 --max-time 15 \
  "http://127.0.0.1:${SERVICE_PORT}/health"
```

如果回滚版本无法兼容已执行的数据库迁移，停止服务后恢复对应的数据库备份，再启动旧版本；恢复前确认备份时间点和目标版本一致。

## 退出条件

- 健康接口连续通过至少两次。
- `docker compose ... ps` 中应用、PostgreSQL 和 Redis 均为运行状态。
- 登录、API Key 鉴权和一条最小 API 请求已由目标环境验证。
- 记录实际镜像 tag/digest、迁移结果和回滚点；不把 `.env`、备份包或调试日志提交回仓库。
