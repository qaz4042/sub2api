#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SSH_TARGET="${SSH_TARGET:-my2g}"
REMOTE_DIR="${REMOTE_DIR:-/opt/sub2api}"
PUBLIC_HEALTH_URL="${PUBLIC_HEALTH_URL:-https://portal.lizubin.online/health}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-60}"
REQUIRE_CLEAN="${REQUIRE_CLEAN:-0}"

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S %z')" "$*"
}

STEP_STARTED=0
step_start() {
  STEP_STARTED=${SECONDS}
  log "$1"
}

step_done() {
  log "$1 完成（耗时 $((SECONDS - STEP_STARTED))s）"
}

for command in git go pnpm ssh scp rsync curl mktemp; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    echo "缺少本地命令: ${command}" >&2
    exit 1
  fi
done

VERSION="$(tr -d '\r\n' < "${ROOT_DIR}/backend/cmd/server/VERSION")"
COMMIT="$(git -C "${ROOT_DIR}" rev-parse --short HEAD)"
DIRTY=false
if [[ -n "$(git -C "${ROOT_DIR}" status --porcelain --untracked-files=normal)" ]]; then
  DIRTY=true
  COMMIT="${COMMIT}-dirty"
fi

if [[ "${DIRTY}" == "true" && "${REQUIRE_CLEAN}" == "1" ]]; then
  echo "工作区存在未提交改动，REQUIRE_CLEAN=1 禁止发布。" >&2
  exit 1
fi

TAG="${TAG:-$(date +%Y%m%d-%H%M%S)-${COMMIT}}"
if [[ ! "${TAG}" =~ ^[A-Za-z0-9._-]+$ ]]; then
  echo "TAG 只能包含字母、数字、点、下划线和连字符: ${TAG}" >&2
  exit 1
fi
if [[ ! "${HEALTH_TIMEOUT}" =~ ^[1-9][0-9]*$ ]]; then
  echo "HEALTH_TIMEOUT 必须是正整数: ${HEALTH_TIMEOUT}" >&2
  exit 1
fi

BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
REMOTE_RELEASE="${REMOTE_DIR}/releases/${TAG}"
REMOTE_INCOMING="${REMOTE_DIR}/releases/.incoming-${TAG}"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/sub2api-deploy.XXXXXX")"
ARTIFACT="${TMP_DIR}/sub2api"
REMOTE_PREPARED=false

cleanup() {
  status=$?
  rm -rf "${TMP_DIR}"
  if [[ "${status}" -ne 0 && "${REMOTE_PREPARED}" == "true" ]]; then
    ssh "${SSH_TARGET}" "rm -rf '${REMOTE_INCOMING}'" >/dev/null 2>&1 || true
  fi
  exit "${status}"
}
trap cleanup EXIT

if [[ "${DIRTY}" == "true" ]]; then
  log "提示: 正在发布未提交的工作区，版本标记为 ${COMMIT}。"
fi

DEPLOY_STARTED=${SECONDS}
step_start "[1/6] 检查远端 systemd 部署环境"
ssh "${SSH_TARGET}" bash -s -- "${REMOTE_DIR}" "${REMOTE_RELEASE}" <<'REMOTE_PREFLIGHT'
set -Eeuo pipefail
remote_dir="$1"
remote_release="$2"
for command in systemctl curl rsync readlink; do
  command -v "${command}" >/dev/null
done
test -d "${remote_dir}/releases"
test -L "${remote_dir}/current"
test -f "${remote_dir}/sub2api.env"
if [[ -e "${remote_release}" ]]; then
  echo "远端 release 已存在: ${remote_release}" >&2
  exit 1
fi
REMOTE_PREFLIGHT
step_done "[1/6] 远端环境检查"

step_start "[2/6] 构建前端"
pnpm --dir "${ROOT_DIR}/frontend" install --frozen-lockfile
pnpm --dir "${ROOT_DIR}/frontend" run build
step_done "[2/6] 前端构建"

step_start "[3/6] 构建 linux/amd64 后端二进制（已嵌入前端）"
(
  cd "${ROOT_DIR}/backend"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -tags embed \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.Date=${BUILD_DATE} -X main.BuildType=release" \
    -o "${ARTIFACT}" \
    ./cmd/server
)
step_done "[3/6] 后端二进制构建"

step_start "[4/6] 上传 release ${TAG}"
ssh "${SSH_TARGET}" "mkdir -p '${REMOTE_INCOMING}/resources'"
REMOTE_PREPARED=true
scp "${ARTIFACT}" "${SSH_TARGET}:${REMOTE_INCOMING}/sub2api"
rsync -a --delete "${ROOT_DIR}/backend/resources/" "${SSH_TARGET}:${REMOTE_INCOMING}/resources/"
step_done "[4/6] release 上传"

step_start "[5/6] 原子切换、重启并执行本机健康检查"
ssh "${SSH_TARGET}" bash -s -- \
  "${REMOTE_DIR}" "${REMOTE_INCOMING}" "${REMOTE_RELEASE}" "${HEALTH_TIMEOUT}" <<'REMOTE_DEPLOY'
set -Eeuo pipefail

remote_dir="$1"
incoming="$2"
release="$3"
health_timeout="$4"
current="${remote_dir}/current"
previous="$(readlink -f "${current}")"
switched=false

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S %z')" "$*"
}

wait_for_health() {
  local attempt
  for ((attempt = 0; attempt < health_timeout; attempt++)); do
    if curl -fsS --connect-timeout 1 --max-time 2 http://127.0.0.1:8080/health >/dev/null; then
      return 0
    fi
    sleep 1
  done
  return 1
}

rollback() {
  local status=$?
  trap - ERR
  if [[ "${switched}" == "true" && -n "${previous}" && -d "${previous}" ]]; then
    log "新 release 健康检查失败，回滚到 ${previous}" >&2
    ln -sfn "${previous}" "${current}"
    systemctl restart sub2api || true
    wait_for_health || true
  fi
  journalctl -u sub2api -n 100 --no-pager >&2 || true
  exit "${status}"
}
trap rollback ERR

test -f "${incoming}/sub2api"
test -d "${incoming}/resources"
chmod 0755 "${incoming}/sub2api"
chown -R root:root "${incoming}"
mv "${incoming}" "${release}"
ln -sfn "${release}" "${current}"
switched=true
systemctl restart sub2api
wait_for_health

trap - ERR
log "远端应用已健康: $(readlink -f "${current}")"
REMOTE_DEPLOY
REMOTE_PREPARED=false
step_done "[5/6] 远端切换与健康检查"

step_start "[6/6] 检查公网入口"
public_healthy=false
for _ in 1 2 3 4 5; do
  if curl -fsS --connect-timeout 5 --max-time 15 "${PUBLIC_HEALTH_URL}" >/dev/null; then
    public_healthy=true
    break
  fi
  sleep 2
done
if [[ "${public_healthy}" != "true" ]]; then
  echo "应用本机健康，但公网健康检查失败: ${PUBLIC_HEALTH_URL}" >&2
  exit 1
fi
step_done "[6/6] 公网入口检查"

log "部署成功（总耗时 $((SECONDS - DEPLOY_STARTED))s）: ${REMOTE_RELEASE}"
log "健康检查: ${PUBLIC_HEALTH_URL}"
