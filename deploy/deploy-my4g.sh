#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SSH_TARGET="${SSH_TARGET:-my4g}"
REMOTE_DIR="${REMOTE_DIR:-/opt/sub2api}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-60}"
REQUIRE_CLEAN="${REQUIRE_CLEAN:-0}"
BUILD_FRONTEND="${BUILD_FRONTEND:-1}"

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S %z')" "$*"
}

STEP_STARTED=0
STEP_NAME=""
step_start() {
  STEP_STARTED=${SECONDS}
  STEP_NAME="$1"
  log "开始 ${STEP_NAME}"
}

step_done() {
  local label="${1:-${STEP_NAME}}"
  log "完成 ${label} ($((SECONDS - STEP_STARTED))s)"
}

run_quiet() {
  local label="$1"
  shift
  local output
  output="$(mktemp "${TMP_DIR}/deploy-step.XXXXXX")"
  if ! "$@" >"${output}" 2>&1; then
    log "${label} 失败，输出如下：" >&2
    cat "${output}" >&2
    return 1
  fi
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
if [[ "${BUILD_FRONTEND}" != "0" && "${BUILD_FRONTEND}" != "1" ]]; then
  echo "BUILD_FRONTEND 只能是 0 或 1。" >&2
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

DEPLOY_STARTED=${SECONDS}
log "发布开始: ${TAG} -> ${SSH_TARGET}:${REMOTE_RELEASE}"
if [[ "${DIRTY}" == "true" ]]; then
  log "提示: 工作区有未提交改动，版本=${COMMIT}"
fi

step_start "1/5 远端环境"
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
step_done

if [[ "${BUILD_FRONTEND}" == "1" ]]; then
  step_start "2/5 前端构建"
  run_quiet "前端依赖安装" pnpm --dir "${ROOT_DIR}/frontend" install --frozen-lockfile
  run_quiet "前端构建" pnpm --dir "${ROOT_DIR}/frontend" run build
  step_done
else
  step_start "2/5 复用前端 dist"
  test -f "${ROOT_DIR}/backend/internal/web/dist/index.html"
  step_done
fi

step_start "3/5 后端构建 linux/amd64"
(
  cd "${ROOT_DIR}/backend"
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -tags embed \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.Date=${BUILD_DATE} -X main.BuildType=release" \
    -o "${ARTIFACT}" \
    ./cmd/server
)
step_done

step_start "4/5 上传 release"
ssh "${SSH_TARGET}" "mkdir -p '${REMOTE_INCOMING}/resources'"
REMOTE_PREPARED=true
scp -q "${ARTIFACT}" "${SSH_TARGET}:${REMOTE_INCOMING}/sub2api"
rsync -a --delete "${ROOT_DIR}/backend/resources/" "${SSH_TARGET}:${REMOTE_INCOMING}/resources/"
step_done

step_start "5/5 切换重启 + 本机健康"
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
    if curl -fs --connect-timeout 1 --max-time 2 http://127.0.0.1:8080/health >/dev/null 2>&1; then
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
REMOTE_DEPLOY
REMOTE_PREPARED=false
step_done

log "发布完成 ($((SECONDS - DEPLOY_STARTED))s)"
log "release: ${REMOTE_RELEASE}"
log "health:  http://127.0.0.1:8080/health (remote local check)"
