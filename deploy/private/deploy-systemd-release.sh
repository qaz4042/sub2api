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
DRY_RUN="${DRY_RUN:-0}"

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
  printf 'release=%s\nssh_target=%s\nremote_release=%s\nservice=%s\nhealth_url=%s\ntarget=%s/%s\nbuild_frontend=%s\n' \
    "${TAG}" "${SSH_TARGET}" "${REMOTE_RELEASE}" "${SERVICE_NAME}" "${HEALTH_URL}" \
    "${TARGET_GOOS}" "${TARGET_GOARCH}" "${BUILD_FRONTEND}"
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

cleanup() {
  local status=$?
  rm -rf "${TMP_DIR}"
  if [[ "${status}" -ne 0 && "${REMOTE_PREPARED}" == "true" ]]; then
    ssh "${SSH_TARGET}" "rm -rf '${REMOTE_INCOMING}'" >/dev/null 2>&1 || true
  fi
  exit "${status}"
}
trap cleanup EXIT

ssh "${SSH_TARGET}" bash -s -- "${REMOTE_DIR}" "${REMOTE_RELEASE}" "${SERVICE_NAME}" <<'REMOTE_PREFLIGHT'
set -Eeuo pipefail
remote_dir="$1"
remote_release="$2"
service_name="$3"
for command in systemctl curl rsync readlink gzip; do
  command -v "${command}" >/dev/null
done
test -d "${remote_dir}/releases"
test -L "${remote_dir}/current"
test -d "$(readlink -f "${remote_dir}/current")"
systemctl cat "${service_name}" >/dev/null
test ! -e "${remote_release}"
REMOTE_PREFLIGHT

if [[ "${BUILD_FRONTEND}" == "1" ]]; then
  pnpm --dir "${ROOT_DIR}/frontend" install --frozen-lockfile
  pnpm --dir "${ROOT_DIR}/frontend" run build
else
  test -f "${ROOT_DIR}/backend/internal/web/dist/index.html"
fi

BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
(
  cd "${ROOT_DIR}/backend"
  CGO_ENABLED=0 GOOS="${TARGET_GOOS}" GOARCH="${TARGET_GOARCH}" go build \
    -tags embed \
    -trimpath \
    -ldflags="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.Date=${BUILD_DATE} -X main.BuildType=release" \
    -o "${ARTIFACT}" \
    ./cmd/server
)

ssh "${SSH_TARGET}" "mkdir -p '${REMOTE_INCOMING}/resources'"
REMOTE_PREPARED=true
gzip -1 -c "${ARTIFACT}" >"${COMPRESSED_ARTIFACT}"
scp -q "${COMPRESSED_ARTIFACT}" "${SSH_TARGET}:${REMOTE_INCOMING}/sub2api.gz"
rsync -a --delete "${ROOT_DIR}/backend/resources/" "${SSH_TARGET}:${REMOTE_INCOMING}/resources/"

ssh "${SSH_TARGET}" bash -s -- \
  "${REMOTE_DIR}" "${REMOTE_INCOMING}" "${REMOTE_RELEASE}" "${SERVICE_NAME}" \
  "${HEALTH_URL}" "${HEALTH_TIMEOUT}" <<'REMOTE_DEPLOY'
set -Eeuo pipefail
remote_dir="$1"
incoming="$2"
release="$3"
service_name="$4"
health_url="$5"
health_timeout="$6"
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

rollback() {
  local status=$?
  trap - ERR
  if [[ "${switched}" == "true" && -d "${previous}" ]]; then
    ln -sfn "${previous}" "${current}"
    systemctl restart "${service_name}" || true
    wait_for_health || true
  fi
  journalctl -u "${service_name}" -n 100 --no-pager >&2 || true
  exit "${status}"
}
trap rollback ERR

gzip -dc "${incoming}/sub2api.gz" >"${incoming}/sub2api"
rm -f "${incoming}/sub2api.gz"
chmod 0755 "${incoming}/sub2api"
mv "${incoming}" "${release}"
ln -sfn "${release}" "${current}"
switched=true
systemctl restart "${service_name}"
wait_for_health
trap - ERR
REMOTE_DEPLOY

REMOTE_PREPARED=false
printf '发布完成: %s:%s\n' "${SSH_TARGET}" "${REMOTE_RELEASE}"
