#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
TMP_DIR="$(mktemp -d "${TMPDIR:-/tmp}/sub2api-deploy-test.XXXXXX")"
trap 'rm -rf "${TMP_DIR}"' EXIT

MOCK_BIN="${TMP_DIR}/bin"
mkdir -p "${MOCK_BIN}"

make_mock() {
  local name="$1"
  shift
  printf '%s\n' '#!/usr/bin/env bash' 'set -Eeuo pipefail' "$@" >"${MOCK_BIN}/${name}"
  chmod +x "${MOCK_BIN}/${name}"
}

make_mock ssh 'if [[ ! -t 0 ]]; then cat >/dev/null; fi'
make_mock scp ':'
make_mock rsync ':'
make_mock pnpm 'echo "unexpected raw pnpm output"'
make_mock go '
output=""
while (($#)); do
  if [[ "$1" == "-o" ]]; then
    output="$2"
    break
  fi
  shift
done
[[ -n "${output}" ]]
: >"${output}"
'

OUTPUT="${TMP_DIR}/output"
PATH="${MOCK_BIN}:${PATH}" \
SSH_TARGET=test-host \
REMOTE_DIR=/srv/sub2api \
SERVICE_NAME=sub2api \
HEALTH_URL=http://127.0.0.1:8080/health \
REMOTE_REQUIRED_FILE=/srv/sub2api/sub2api.env \
REMOTE_OWNER=root:root \
REQUIRE_CLEAN=0 \
TAG=test-release \
"${ROOT_DIR}/deploy/private/deploy-systemd-release.sh" >"${OUTPUT}"

for expected in \
  '开始 1/5 远端环境' \
  '开始 2/5 前端构建' \
  '开始 3/5 后端构建 linux/amd64' \
  '开始 4/5 上传 release' \
  '开始 5/5 切换重启 + 本机健康' \
  'release: /srv/sub2api/releases/test-release' \
  'health:  http://127.0.0.1:8080/health (remote local check)'; do
  grep -Fq "${expected}" "${OUTPUT}"
done

if grep -Fq 'unexpected raw pnpm output' "${OUTPUT}"; then
  echo "成功构建时不应输出 pnpm 详情" >&2
  exit 1
fi

DRY_OUTPUT="${TMP_DIR}/dry-output"
SSH_TARGET=my4g \
REMOTE_REQUIRED_FILE=/opt/sub2api/sub2api.env \
REMOTE_OWNER=root:root \
BUILD_FRONTEND=0 \
REQUIRE_CLEAN=0 \
DRY_RUN=1 \
TAG=dry-release \
"${ROOT_DIR}/deploy/private/deploy-systemd-release.sh" >"${DRY_OUTPUT}"

grep -Fqx 'build_frontend=0' "${DRY_OUTPUT}"
grep -Fqx 'remote_required_file=/opt/sub2api/sub2api.env' "${DRY_OUTPUT}"
grep -Fqx 'remote_owner=root:root' "${DRY_OUTPUT}"
