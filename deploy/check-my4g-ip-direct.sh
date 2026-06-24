#!/usr/bin/env bash

set -Eeuo pipefail

PUBLIC_IP="${PUBLIC_IP:-152.32.190.110}"
BASE_URL="${BASE_URL:-https://${PUBLIC_IP}}"
API_KEY="${API_KEY:-}"
MODEL_LIST_PATH="${MODEL_LIST_PATH:-/v1/models}"

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S %z')" "$*"
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || {
    echo "缺少命令: $1" >&2
    exit 1
  }
}

for command in curl openssl head; do
  require_command "${command}"
done

log "检查证书 SAN 与有效期: ${PUBLIC_IP}:443"
printf '' | openssl s_client -connect "${PUBLIC_IP}:443" -servername "${PUBLIC_IP}" 2>/dev/null \
  | openssl x509 -noout -subject -issuer -dates -ext subjectAltName

log "检查健康接口: ${BASE_URL}/health"
curl -fsS --connect-timeout 5 --max-time 15 "${BASE_URL}/health"
printf '\n'

if [[ -n "${API_KEY}" ]]; then
  log "检查真实 API 请求: ${BASE_URL}${MODEL_LIST_PATH}"
  response="$(curl -fsS --connect-timeout 5 --max-time 30 \
    -H "Authorization: Bearer ${API_KEY}" \
    "${BASE_URL}${MODEL_LIST_PATH}")"
  printf '%s' "${response}" | head -c 1000
  printf '\n'
else
  log "未设置 API_KEY，已跳过 /v1/models 真实 API 请求。"
fi

log "完成。ccSwitch 灰度入口填写: ${BASE_URL}"
