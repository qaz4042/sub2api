#!/usr/bin/env bash

set -Eeuo pipefail

SSH_TARGET="${SSH_TARGET:-my2g}"
REMOTE_DIR="${REMOTE_DIR:-/opt/sub2api}"
STAMP="${STAMP:-$(date +%Y%m%d-%H%M%S)}"
LOCAL_ROOT="${LOCAL_ROOT:-${HOME}/Documents/sub2api-recovery}"
LOCAL_DIR="${LOCAL_ROOT}/${STAMP}"
REMOTE_BUNDLE="${REMOTE_DIR}/backups/sub2api-reinstall-${STAMP}.tar.gz"
LOCAL_PLAIN="${LOCAL_DIR}/sub2api-reinstall-${STAMP}.tar.gz"
LOCAL_ENCRYPTED="${LOCAL_PLAIN}.enc"
KEY_FILE="${LOCAL_DIR}/sub2api-reinstall-${STAMP}.key"

for command in ssh scp openssl shasum tar mktemp; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    echo "缺少命令: ${command}" >&2
    exit 1
  fi
done

umask 077
mkdir -p "${LOCAL_DIR}"

echo "[1/6] 在 ${SSH_TARGET} 生成 PostgreSQL dump 和配置包"
ssh "${SSH_TARGET}" 'sudo bash -s' -- "${STAMP}" "${REMOTE_DIR}" <<'REMOTE_SCRIPT'
set -Eeuo pipefail

STAMP="$1"
REMOTE_DIR="$2"
STAGING="${REMOTE_DIR}/backups/reinstall-${STAMP}"
BUNDLE="${REMOTE_DIR}/backups/sub2api-reinstall-${STAMP}.tar.gz"
ROOTFS_ARCHIVE="${STAGING}/rootfs.tar.gz"
DATABASE_DUMP="${STAGING}/postgres.dump"

cleanup() {
  rm -rf "${STAGING}"
}
trap cleanup EXIT

mkdir -p "${STAGING}"
chmod 0700 "${REMOTE_DIR}/backups" "${STAGING}"

docker exec sub2api-postgres sh -c \
  'pg_dump -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Fc' > "${DATABASE_DUMP}"
test -s "${DATABASE_DUMP}"
docker exec -i sub2api-postgres pg_restore --list < "${DATABASE_DUMP}" >/dev/null

declare -a backup_paths=(
  opt/sub2api/.env
  opt/sub2api/data
  opt/sub2api/docker-compose.yml
  opt/sub2api/compose.my2g.yml
  opt/sub2api/compose.mihomo.yml
  opt/sub2api/mihomo
  etc/cloudflared/config.yml
  etc/systemd/system/cloudflared.service
  etc/nginx/conf.d/sub2api.conf
  root/.ssh/authorized_keys
)

credentials_file="$(awk -F: '/^[[:space:]]*credentials-file:/ {sub(/^[[:space:]]+/, "", $2); print $2; exit}' /etc/cloudflared/config.yml)"
if [[ -n "${credentials_file}" && -f "${credentials_file}" ]]; then
  backup_paths+=("${credentials_file#/}")
fi

for path in "${backup_paths[@]}"; do
  test -e "/${path}"
done

tar --acls --xattrs -czpf "${ROOTFS_ARCHIVE}" -C / "${backup_paths[@]}"
tar -tvzf "${ROOTFS_ARCHIVE}" > "${STAGING}/rootfs-manifest.txt"

{
  printf 'created_at=%s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  printf 'source_host=%s\n' "$(hostname)"
  printf 'source_os=%s\n' "$(. /etc/os-release && printf '%s' "${PRETTY_NAME}")"
  printf 'postgres_version=%s\n' "$(docker exec sub2api-postgres postgres -V)"
  printf 'sub2api_image=%s\n' "$(docker inspect --format '{{.Config.Image}}' sub2api)"
} > "${STAGING}/metadata.txt"

(
  cd "${STAGING}"
  sha256sum postgres.dump rootfs.tar.gz rootfs-manifest.txt metadata.txt > SHA256SUMS
)

tar -czpf "${BUNDLE}" -C "${STAGING}" \
  postgres.dump rootfs.tar.gz rootfs-manifest.txt metadata.txt SHA256SUMS
chmod 0600 "${BUNDLE}"
(
  cd "$(dirname "${BUNDLE}")"
  sha256sum "$(basename "${BUNDLE}")" > "$(basename "${BUNDLE}").sha256"
)
chmod 0600 "${BUNDLE}.sha256"

tar -tzf "${BUNDLE}" >/dev/null
stat -c '%n %s bytes %U:%G %a' "${BUNDLE}" "${BUNDLE}.sha256"
REMOTE_SCRIPT

echo "[2/6] 下载明文备份到 Mac 临时目录"
scp "${SSH_TARGET}:${REMOTE_BUNDLE}" "${LOCAL_PLAIN}"
scp "${SSH_TARGET}:${REMOTE_BUNDLE}.sha256" "${LOCAL_PLAIN}.remote.sha256"

remote_sha="$(awk '{print $1}' "${LOCAL_PLAIN}.remote.sha256")"
local_sha="$(shasum -a 256 "${LOCAL_PLAIN}" | awk '{print $1}')"
if [[ "${remote_sha}" != "${local_sha}" ]]; then
  echo "远端与本地备份 SHA-256 不一致" >&2
  exit 1
fi
tar -tzf "${LOCAL_PLAIN}" >/dev/null

echo "[3/6] 生成本地恢复密钥"
openssl rand -base64 48 > "${KEY_FILE}"
chmod 0600 "${KEY_FILE}"

echo "[4/6] 使用 AES-256-CBC + PBKDF2 加密"
openssl enc -aes-256-cbc -salt -pbkdf2 -iter 600000 \
  -in "${LOCAL_PLAIN}" -out "${LOCAL_ENCRYPTED}" -pass "file:${KEY_FILE}"
chmod 0600 "${LOCAL_ENCRYPTED}"
(
  cd "${LOCAL_DIR}"
  shasum -a 256 "$(basename "${LOCAL_ENCRYPTED}")" > "$(basename "${LOCAL_ENCRYPTED}").sha256"
)

echo "[5/6] 解密回读并校验"
verify_file="$(mktemp "${LOCAL_DIR}/verify.XXXXXX.tar.gz")"
trap 'rm -f "${verify_file}"' EXIT
openssl enc -d -aes-256-cbc -pbkdf2 -iter 600000 \
  -in "${LOCAL_ENCRYPTED}" -out "${verify_file}" -pass "file:${KEY_FILE}"
verify_sha="$(shasum -a 256 "${verify_file}" | awk '{print $1}')"
if [[ "${verify_sha}" != "${local_sha}" ]]; then
  echo "加密包解密回读 SHA-256 不一致" >&2
  exit 1
fi
tar -tzf "${verify_file}" >/dev/null
rm -f "${verify_file}" "${LOCAL_PLAIN}" "${LOCAL_PLAIN}.remote.sha256"
trap - EXIT

echo "[6/6] 备份完成"
printf 'STAMP=%s\n' "${STAMP}"
printf 'ENCRYPTED=%s\n' "${LOCAL_ENCRYPTED}"
printf 'CHECKSUM=%s\n' "${LOCAL_ENCRYPTED}.sha256"
printf 'KEY_FILE=%s\n' "${KEY_FILE}"
