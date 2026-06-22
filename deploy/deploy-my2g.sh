#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SSH_TARGET="${SSH_TARGET:-my2g}"
REMOTE_DIR="${REMOTE_DIR:-/opt/sub2api}"
TAG="${TAG:-0.0.0-my2g.$(date +%Y%m%d-%H%M%S)}"
COMMIT="${COMMIT:-$(git -C "${ROOT_DIR}" rev-parse HEAD 2>/dev/null || printf 'unknown')}"
BUILD_DATE="${BUILD_DATE:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}"
IMAGE="sub2api:${TAG}"
ARCHIVE="${TMPDIR:-/tmp}/sub2api-${TAG}.tar.gz"
REMOTE_ARCHIVE="/tmp/sub2api-${TAG}.tar.gz"

cleanup() {
  rm -f "${ARCHIVE}"
}
trap cleanup EXIT

for command in docker ssh scp gzip; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    echo "缺少命令: ${command}" >&2
    exit 1
  fi
done

if ! docker info >/dev/null 2>&1; then
  echo "Docker Desktop 未运行。" >&2
  exit 1
fi

echo "[1/5] 构建 ${IMAGE} (linux/amd64)"
docker build \
  --platform linux/amd64 \
  --build-arg "VERSION=${TAG}" \
  --build-arg "COMMIT=${COMMIT}" \
  --build-arg "DATE=${BUILD_DATE}" \
  -t "${IMAGE}" \
  "${ROOT_DIR}"

echo "[2/5] 导出并压缩镜像"
docker save "${IMAGE}" | gzip -1 > "${ARCHIVE}"

echo "[3/5] 上传到 ${SSH_TARGET}"
scp "${ARCHIVE}" "${SSH_TARGET}:${REMOTE_ARCHIVE}"

echo "[4/5] 更新远端应用"
ssh "${SSH_TARGET}" bash -s -- "${TAG}" "${REMOTE_ARCHIVE}" "${REMOTE_DIR}" <<'REMOTE_SCRIPT'
set -Eeuo pipefail

TAG="$1"
ARCHIVE="$2"
REMOTE_DIR="$3"
IMAGE="sub2api:${TAG}"
COMPOSE_FILES=(-f docker-compose.yml -f compose.my2g.yml -f compose.mihomo.yml)
BACKUP="${REMOTE_DIR}/backups/compose.my2g-before-${TAG}.yml"
DEPLOY_STARTED=false

rollback() {
  status=$?
  trap - ERR
  rm -f "${ARCHIVE}"
  if [[ "${DEPLOY_STARTED}" == "true" && -f "${BACKUP}" ]]; then
    echo "部署失败，恢复上一个应用镜像..." >&2
    cp "${BACKUP}" compose.my2g.yml
    docker compose "${COMPOSE_FILES[@]}" up -d --no-deps sub2api || true
  fi
  exit "${status}"
}
trap rollback ERR

cd "${REMOTE_DIR}"
mkdir -p backups
gzip -dc "${ARCHIVE}" | docker load
docker image inspect "${IMAGE}" >/dev/null

cp compose.my2g.yml "${BACKUP}"
DEPLOY_STARTED=true

sed -i "/^  sub2api:$/,/^  [[:alnum:]_-]*:$/ s|^    image:.*$|    image: ${IMAGE}|" compose.my2g.yml
grep -F "    image: ${IMAGE}" compose.my2g.yml >/dev/null

docker compose "${COMPOSE_FILES[@]}" config >/dev/null
docker compose "${COMPOSE_FILES[@]}" up -d --no-deps sub2api

for _ in $(seq 1 36); do
  health="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' sub2api)"
  if [[ "${health}" == "healthy" ]]; then
    break
  fi
  if [[ "${health}" == "unhealthy" || "${health}" == "exited" || "${health}" == "dead" ]]; then
    echo "容器状态异常: ${health}" >&2
    exit 1
  fi
  sleep 5
done

health="$(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' sub2api)"
if [[ "${health}" != "healthy" ]]; then
  echo "等待健康检查超时，当前状态: ${health}" >&2
  exit 1
fi

curl -fsS --connect-timeout 10 --max-time 30 https://portal.lizubin.online/health >/dev/null
rm -f "${ARCHIVE}"
trap - ERR

echo "远端部署成功: ${IMAGE}"
echo "配置备份: ${BACKUP}"
REMOTE_SCRIPT

echo "[5/5] 部署完成"
echo "镜像: ${IMAGE}"
echo "地址: https://portal.lizubin.online/purchase"
