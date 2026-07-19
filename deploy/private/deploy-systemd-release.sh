#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SSH_TARGET="${SSH_TARGET:-}"
REMOTE_DIR="${REMOTE_DIR:-/opt/sub2api}"
SERVICE_NAME="${SERVICE_NAME:-sub2api}"
HEALTH_URL="${HEALTH_URL:-http://127.0.0.1:8080/health}"
HEALTH_TIMEOUT="${HEALTH_TIMEOUT:-60}"
REQUIRE_CLEAN="${REQUIRE_CLEAN:-1}"
BUILD_FRONTEND="${BUILD_FRONTEND:-1}"
TARGET_GOOS="${TARGET_GOOS:-linux}"
TARGET_GOARCH="${TARGET_GOARCH:-amd64}"
REMOTE_REQUIRED_FILE="${REMOTE_REQUIRED_FILE:-}"
REMOTE_OWNER="${REMOTE_OWNER:-}"
DRY_RUN="${DRY_RUN:-0}"

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S %z')" "$*"
}

fail() {
  echo "$*" >&2
  exit 1
}

require_boolean() {
  local name="$1"
  local value="$2"
  [[ "${value}" == "0" || "${value}" == "1" ]] || fail "${name} 只能是 0 或 1。"
}

[[ -n "${SSH_TARGET}" ]] || fail "必须设置 SSH_TARGET，例如 SSH_TARGET=deploy.example.invalid。"
[[ "${REMOTE_DIR}" == /* ]] || fail "REMOTE_DIR 必须是绝对路径。"
[[ "${REMOTE_DIR}" =~ ^/[A-Za-z0-9._/-]+$ && "/${REMOTE_DIR#/}/" != *"/../"* ]] || \
  fail "REMOTE_DIR 包含不安全的路径字符或 .. 段。"
[[ "${SERVICE_NAME}" =~ ^[A-Za-z0-9@_.-]+$ ]] || fail "SERVICE_NAME 包含非法字符。"
[[ "${HEALTH_URL}" == http://127.0.0.1:*/* || "${HEALTH_URL}" == http://localhost:*/* ]] || \
  fail "HEALTH_URL 必须是远端本机 HTTP 地址。"
[[ "${HEALTH_TIMEOUT}" =~ ^[1-9][0-9]*$ ]] || fail "HEALTH_TIMEOUT 必须是正整数。"
[[ "${TARGET_GOOS}" =~ ^[a-z0-9]+$ ]] || fail "TARGET_GOOS 包含非法字符。"
[[ "${TARGET_GOARCH}" =~ ^[a-z0-9]+$ ]] || fail "TARGET_GOARCH 包含非法字符。"
require_boolean REQUIRE_CLEAN "${REQUIRE_CLEAN}"
require_boolean BUILD_FRONTEND "${BUILD_FRONTEND}"
require_boolean DRY_RUN "${DRY_RUN}"
if [[ -n "${REMOTE_REQUIRED_FILE}" ]]; then
  [[ "${REMOTE_REQUIRED_FILE}" == /* ]] || fail "REMOTE_REQUIRED_FILE 必须是绝对路径。"
  [[ "${REMOTE_REQUIRED_FILE}" =~ ^/[A-Za-z0-9._/-]+$ && "/${REMOTE_REQUIRED_FILE#/}/" != *"/../"* ]] || \
    fail "REMOTE_REQUIRED_FILE 包含不安全的路径字符或 .. 段。"
fi
if [[ -n "${REMOTE_OWNER}" ]]; then
  [[ "${REMOTE_OWNER}" =~ ^[A-Za-z0-9_.-]+:[A-Za-z0-9_.-]+$ ]] || fail "REMOTE_OWNER 必须是 user:group。"
fi

VERSION="$(tr -d '\r\n' < "${ROOT_DIR}/backend/cmd/server/VERSION")"
COMMIT="$(git -C "${ROOT_DIR}" rev-parse --short HEAD)"
if [[ -n "$(git -C "${ROOT_DIR}" status --porcelain --untracked-files=normal)" ]]; then
  [[ "${REQUIRE_CLEAN}" == "0" ]] || fail "工作区存在未提交改动，设置 REQUIRE_CLEAN=0 可显式允许。"
  COMMIT="${COMMIT}-dirty"
fi

TAG="${TAG:-$(date +%Y%m%d-%H%M%S)-${COMMIT}}"
[[ "${TAG}" =~ ^[A-Za-z0-9._-]+$ ]] || fail "TAG 包含非法字符。"

REMOTE_RELEASE="${REMOTE_DIR}/releases/${TAG}"
REMOTE_INCOMING="${REMOTE_DIR}/releases/.incoming-${TAG}"

if [[ "${DRY_RUN}" == "1" ]]; then
  printf 'release=%s\nssh_target=%s\nremote_release=%s\nservice=%s\nhealth_url=%s\ntarget=%s/%s\nbuild_frontend=%s\nremote_required_file=%s\nremote_owner=%s\n' \
    "${TAG}" "${SSH_TARGET}" "${REMOTE_RELEASE}" "${SERVICE_NAME}" "${HEALTH_URL}" \
    "${TARGET_GOOS}" "${TARGET_GOARCH}" "${BUILD_FRONTEND}" "${REMOTE_REQUIRED_FILE}" "${REMOTE_OWNER}"
  exit 0
fi

for command in git go ssh scp rsync curl mktemp gzip; do
  command -v "${command}" >/dev/null 2>&1 || fail "缺少本地命令: ${command}"
done
if [[ "${BUILD_FRONTEND}" == "1" ]]; then
  command -v pnpm >/dev/null 2>&1 || fail "缺少本地命令: pnpm"
fi

TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/sub2api-release.XXXXXX")"
ARTIFACT="${TMP_DIR}/sub2api"
COMPRESSED_ARTIFACT="${TMP_DIR}/sub2api.gz"
REMOTE_PREPARED=false

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

cleanup() {
  local status=$?
  rm -rf "${TMP_DIR}"
  if [[ "${status}" -ne 0 && "${REMOTE_PREPARED}" == "true" ]]; then
    ssh "${SSH_TARGET}" "rm -rf '${REMOTE_INCOMING}'" >/dev/null 2>&1 || true
  fi
  exit "${status}"
}
trap cleanup EXIT

DEPLOY_STARTED=${SECONDS}
log "发布开始: ${TAG} -> ${SSH_TARGET}:${REMOTE_RELEASE}"
if [[ "${COMMIT}" == *-dirty ]]; then
  log "提示: 工作区有未提交改动，版本=${COMMIT}"
fi

step_start "1/5 远端环境"
ssh "${SSH_TARGET}" bash -s -- \
  "${REMOTE_DIR}" "${REMOTE_RELEASE}" "${SERVICE_NAME}" "${REMOTE_REQUIRED_FILE}" <<'REMOTE_PREFLIGHT'
set -Eeuo pipefail
remote_dir="$1"
remote_release="$2"
service_name="$3"
required_file="$4"
for command in systemctl curl rsync readlink gzip; do
  command -v "${command}" >/dev/null
done
test -d "${remote_dir}/releases"
test -L "${remote_dir}/current"
test -d "$(readlink -f "${remote_dir}/current")"
systemctl cat "${service_name}" >/dev/null
if [[ -n "${required_file}" ]]; then
  test -f "${required_file}"
fi
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
else
  step_start "2/5 复用前端 dist"
  test -f "${ROOT_DIR}/backend/internal/web/dist/index.html"
fi
step_done

BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
step_start "3/5 后端构建 ${TARGET_GOOS}/${TARGET_GOARCH}"
run_quiet "后端构建" bash -c '
  cd "$1"
  CGO_ENABLED=0 GOOS="$2" GOARCH="$3" go build \
    -tags embed \
    -trimpath \
    -ldflags="$4" \
    -o "$5" \
    ./cmd/server
' _ "${ROOT_DIR}/backend" "${TARGET_GOOS}" "${TARGET_GOARCH}" \
  "-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.Date=${BUILD_DATE} -X main.BuildType=release" \
  "${ARTIFACT}"
step_done

step_start "4/5 上传 release"
ssh "${SSH_TARGET}" "mkdir -p '${REMOTE_INCOMING}/resources'"
REMOTE_PREPARED=true
gzip -1 -c "${ARTIFACT}" >"${COMPRESSED_ARTIFACT}"
scp -q "${COMPRESSED_ARTIFACT}" "${SSH_TARGET}:${REMOTE_INCOMING}/sub2api.gz"
rsync -a --delete "${ROOT_DIR}/backend/resources/" "${SSH_TARGET}:${REMOTE_INCOMING}/resources/"
step_done

step_start "5/5 切换重启 + 本机健康"
ssh "${SSH_TARGET}" bash -s -- \
  "${REMOTE_DIR}" "${REMOTE_INCOMING}" "${REMOTE_RELEASE}" "${SERVICE_NAME}" \
  "${HEALTH_URL}" "${HEALTH_TIMEOUT}" "${REMOTE_OWNER}" <<'REMOTE_DEPLOY'
set -Eeuo pipefail
remote_dir="$1"
incoming="$2"
release="$3"
service_name="$4"
health_url="$5"
health_timeout="$6"
remote_owner="$7"
current="${remote_dir}/current"
previous="$(readlink -f "${current}")"
switched=false

wait_for_health() {
  local attempt
  for ((attempt = 0; attempt < health_timeout; attempt++)); do
    if curl -fsS --connect-timeout 1 --max-time 2 "${health_url}" >/dev/null 2>&1; then
      return 0
    fi
    sleep 1
  done
  return 1
}

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S %z')" "$*"
}

rollback() {
  local status=$?
  trap - ERR
  if [[ "${switched}" == "true" && -d "${previous}" ]]; then
    log "新 release 健康检查失败，回滚到 ${previous}" >&2
    ln -sfn "${previous}" "${current}"
    systemctl restart "${service_name}" || true
    wait_for_health || true
  fi
  journalctl -u "${service_name}" -n 100 --no-pager >&2 || true
  exit "${status}"
}
trap rollback ERR

test -f "${incoming}/sub2api.gz"
test -d "${incoming}/resources"
gzip -dc "${incoming}/sub2api.gz" >"${incoming}/sub2api"
rm -f "${incoming}/sub2api.gz"
chmod 0755 "${incoming}/sub2api"
if [[ -n "${remote_owner}" ]]; then
  chown -R "${remote_owner}" "${incoming}"
fi
mv "${incoming}" "${release}"
ln -sfn "${release}" "${current}"
switched=true
systemctl restart "${service_name}"
wait_for_health
trap - ERR
REMOTE_DEPLOY

REMOTE_PREPARED=false
step_done
log "发布完成 ($((SECONDS - DEPLOY_STARTED))s)"
log "release: ${REMOTE_RELEASE}"
log "health:  ${HEALTH_URL} (remote local check)"
