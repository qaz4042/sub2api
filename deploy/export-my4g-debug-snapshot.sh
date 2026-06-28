#!/usr/bin/env bash

set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SSH_TARGET="${SSH_TARGET:-my4g}"
REMOTE_DIR="${REMOTE_DIR:-/opt/sub2api}"
OUTPUT_BASE="${OUTPUT_BASE:-${ROOT_DIR}/.local-data/debug-snapshots}"
SNAPSHOT_TAG="${SNAPSHOT_TAG:-$(date +%Y%m%d-%H%M%S)}"
OUTPUT_DIR="${OUTPUT_DIR:-${OUTPUT_BASE}/${SSH_TARGET}-${SNAPSHOT_TAG}}"
DRY_RUN="${DRY_RUN:-1}"
CONFIRM_VALUE="I_UNDERSTAND_READONLY_REDACTED"

LIMIT_USERS="${LIMIT_USERS:-50}"
LIMIT_API_KEYS="${LIMIT_API_KEYS:-100}"
LIMIT_ACCOUNTS="${LIMIT_ACCOUNTS:-100}"
LIMIT_USAGE="${LIMIT_USAGE:-300}"
USAGE_DAYS="${USAGE_DAYS:-7}"

log() {
  printf '[%s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S %z')" "$*" >&2
}

require_int() {
  local name="$1"
  local value="$2"
  if [[ ! "${value}" =~ ^[1-9][0-9]*$ ]]; then
    echo "${name} 必须是正整数: ${value}" >&2
    exit 1
  fi
}

require_int LIMIT_USERS "${LIMIT_USERS}"
require_int LIMIT_API_KEYS "${LIMIT_API_KEYS}"
require_int LIMIT_ACCOUNTS "${LIMIT_ACCOUNTS}"
require_int LIMIT_USAGE "${LIMIT_USAGE}"
require_int USAGE_DAYS "${USAGE_DAYS}"

if [[ "${DRY_RUN}" == "1" ]]; then
  cat >&2 <<EOF
将从 ${SSH_TARGET}:${REMOTE_DIR} 只读导出脱敏调试快照。

不会做的事：
- 不下载 ${REMOTE_DIR}/sub2api.env
- 不 pg_dump 全库
- 不导出明文 password/token/secret/api key/credentials
- 不写入生产数据库

将导出的内容：
- 数据库表估算行数
- settings 的 key 与脱敏后的 value 摘要
- users/api_keys/accounts/groups/usage_logs 的小样本

确认执行：
  DRY_RUN=0 CONFIRM_MY4G_DEBUG_EXPORT=${CONFIRM_VALUE} ./deploy/export-my4g-debug-snapshot.sh

可选参数：
  LIMIT_USERS=50 LIMIT_API_KEYS=100 LIMIT_ACCOUNTS=100 LIMIT_USAGE=300 USAGE_DAYS=7
EOF
  exit 0
fi

if [[ "${CONFIRM_MY4G_DEBUG_EXPORT:-}" != "${CONFIRM_VALUE}" ]]; then
  echo "拒绝执行：请设置 CONFIRM_MY4G_DEBUG_EXPORT=${CONFIRM_VALUE}" >&2
  exit 1
fi

for command in ssh tar mkdir date; do
  if ! command -v "${command}" >/dev/null 2>&1; then
    echo "缺少本地命令: ${command}" >&2
    exit 1
  fi
done

mkdir -p "${OUTPUT_DIR}"
ARCHIVE="${OUTPUT_DIR}/snapshot.tar.gz"

log "开始导出脱敏调试快照: ${SSH_TARGET}:${REMOTE_DIR}"
ssh "${SSH_TARGET}" bash -s -- \
  "${REMOTE_DIR}" \
  "${LIMIT_USERS}" \
  "${LIMIT_API_KEYS}" \
  "${LIMIT_ACCOUNTS}" \
  "${LIMIT_USAGE}" \
  "${USAGE_DAYS}" <<'REMOTE_SCRIPT' >"${ARCHIVE}"
set -Eeuo pipefail

remote_dir="$1"
limit_users="$2"
limit_api_keys="$3"
limit_accounts="$4"
limit_usage="$5"
usage_days="$6"

log() {
  printf '[remote %s] %s\n' "$(date '+%Y-%m-%d %H:%M:%S %z')" "$*" >&2
}

yaml_get() {
  local section="$1"
  local key="$2"
  local file="$3"
  [[ -r "${file}" ]] || return 0
  awk -v section="${section}" -v key="${key}" '
    $0 ~ "^[[:space:]]*" section ":[[:space:]]*$" { in_section=1; next }
    in_section && $0 ~ "^[^[:space:]#][^:]*:" { in_section=0 }
    in_section {
      line=$0
      sub(/#.*/, "", line)
      pattern="^[[:space:]]+" key ":[[:space:]]*"
      if (line ~ pattern) {
        sub(pattern, "", line)
        gsub(/^[ \t"'\''"]+|[ \t"'\''"]+$/, "", line)
        print line
        exit
      }
    }
  ' "${file}"
}

if ! command -v psql >/dev/null 2>&1; then
  echo "远端缺少 psql，无法导出只读快照。" >&2
  exit 1
fi

env_file="${remote_dir}/sub2api.env"
if [[ -r "${env_file}" ]]; then
  set -a
  # shellcheck disable=SC1090
  source "${env_file}"
  set +a
fi

config_file=""
for candidate in \
  "${DATA_DIR:-}/config.yaml" \
  "${remote_dir}/config.yaml" \
  "${remote_dir}/current/config.yaml" \
  "/app/data/config.yaml" \
  "/etc/sub2api/config.yaml"; do
  if [[ -n "${candidate}" && -r "${candidate}" ]]; then
    config_file="${candidate}"
    break
  fi
done

DATABASE_HOST="${DATABASE_HOST:-$(yaml_get database host "${config_file}")}"
DATABASE_PORT="${DATABASE_PORT:-$(yaml_get database port "${config_file}")}"
DATABASE_USER="${DATABASE_USER:-$(yaml_get database user "${config_file}")}"
DATABASE_PASSWORD="${DATABASE_PASSWORD:-$(yaml_get database password "${config_file}")}"
DATABASE_DBNAME="${DATABASE_DBNAME:-$(yaml_get database dbname "${config_file}")}"
DATABASE_SSLMODE="${DATABASE_SSLMODE:-$(yaml_get database sslmode "${config_file}")}"

export PGHOST="${DATABASE_HOST:-127.0.0.1}"
export PGPORT="${DATABASE_PORT:-5432}"
export PGUSER="${DATABASE_USER:-postgres}"
export PGPASSWORD="${DATABASE_PASSWORD:-}"
export PGDATABASE="${DATABASE_DBNAME:-sub2api}"
export PGSSLMODE="${DATABASE_SSLMODE:-disable}"

tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

psql_cmd=(psql -X -v ON_ERROR_STOP=1 -qAt)

copy_query() {
  local file="$1"
  local sql
  sql="$(cat)"
  "${psql_cmd[@]}" -c "COPY (${sql}) TO STDOUT" >"${tmp}/${file}"
}

write_shape() {
  {
    printf '生成时间: %s\n' "$(date -Is)"
    printf '远端目录: %s\n' "${remote_dir}"
    printf '配置来源: %s\n' "${config_file:-env/defaults}"
    printf '数据库连接: host=%s port=%s user=%s dbname=%s sslmode=%s password_present=%s\n' \
      "${PGHOST}" "${PGPORT}" "${PGUSER}" "${PGDATABASE}" "${PGSSLMODE}" \
      "$([[ -n "${PGPASSWORD}" ]] && echo yes || echo no)"
    printf '\n只记录配置形状，不记录任何明文密钥。\n'
    printf 'env_present DATABASE_HOST=%s DATABASE_PORT=%s DATABASE_USER=%s DATABASE_PASSWORD=%s DATABASE_DBNAME=%s REDIS_HOST=%s REDIS_PASSWORD=%s JWT_SECRET=%s\n' \
      "$([[ -n "${DATABASE_HOST:-}" ]] && echo yes || echo no)" \
      "$([[ -n "${DATABASE_PORT:-}" ]] && echo yes || echo no)" \
      "$([[ -n "${DATABASE_USER:-}" ]] && echo yes || echo no)" \
      "$([[ -n "${DATABASE_PASSWORD:-}" ]] && echo yes || echo no)" \
      "$([[ -n "${DATABASE_DBNAME:-}" ]] && echo yes || echo no)" \
      "$([[ -n "${REDIS_HOST:-}" ]] && echo yes || echo no)" \
      "$([[ -n "${REDIS_PASSWORD:-}" ]] && echo yes || echo no)" \
      "$([[ -n "${JWT_SECRET:-}" ]] && echo yes || echo no)"
  } >"${tmp}/configuration_shape.txt"
}

write_readme() {
  cat >"${tmp}/README.txt" <<EOF
这是从生产环境生成的只读、脱敏、小样本调试快照。

使用约束：
- 不要把本目录提交到 Git。
- 不要把样本当作完整备份。
- users/api_keys/accounts 的敏感字段已经替换为占位或摘要。
- 如仍需复现具体问题，继续按最小范围补充对应记录，避免全量同步生产数据。
EOF
}

log "连接 PostgreSQL: ${PGHOST}:${PGPORT}/${PGDATABASE}"
"${psql_cmd[@]}" -c "SELECT 1" >/dev/null

write_shape
write_readme

copy_query metadata.jsonl <<SQL
SELECT row_to_json(t)
FROM (
  SELECT
    now() AS exported_at,
    current_database() AS database_name,
    current_user AS database_user,
    inet_server_addr()::text AS server_addr,
    version() AS postgres_version
) t
SQL

copy_query table_counts.jsonl <<SQL
SELECT row_to_json(t)
FROM (
  SELECT
    relname AS table_name,
    n_live_tup AS estimated_rows
  FROM pg_stat_user_tables
  ORDER BY relname
) t
SQL

copy_query settings_summary.jsonl <<SQL
SELECT row_to_json(t)
FROM (
  SELECT
    key,
    length(value) AS value_length,
    CASE
      WHEN lower(key) ~ '(secret|token|password|credential|client_secret|api_key|apikey|private|webhook|smtp|oauth|payment|notify|sign|salt)' THEN '[redacted]'
      WHEN length(value) > 200 THEN left(value, 200) || '...[truncated]'
      ELSE value
    END AS value_preview,
    updated_at
  FROM settings
  ORDER BY key
) t
SQL

copy_query groups_sample.jsonl <<SQL
SELECT row_to_json(t)
FROM (
  SELECT
    id,
    name,
    description IS NOT NULL AND description <> '' AS has_description,
    rate_multiplier,
    is_exclusive,
    status,
    created_at,
    updated_at,
    deleted_at IS NOT NULL AS deleted
  FROM groups
  ORDER BY id
) t
SQL

copy_query users_sample.jsonl <<SQL
SELECT row_to_json(t)
FROM (
  SELECT
    id,
    'user_' || id || '@redacted.local' AS email,
    role,
    balance,
    concurrency,
    status,
    username <> '' AS has_username,
    notes <> '' AS has_notes,
    totp_enabled,
    signup_source,
    last_login_at,
    last_active_at,
    balance_notify_enabled,
    balance_notify_threshold_type,
    balance_notify_threshold,
    total_recharged,
    rpm_limit,
    created_at,
    updated_at,
    deleted_at IS NOT NULL AS deleted
  FROM users
  ORDER BY created_at DESC
  LIMIT ${limit_users}
) t
SQL

copy_query api_keys_sample.jsonl <<SQL
SELECT row_to_json(t)
FROM (
  SELECT
    id,
    user_id,
    'sk-redacted-' || id AS key_preview,
    md5(coalesce(name, '')) AS name_hash,
    group_id,
    status,
    last_used_at,
    jsonb_array_length(coalesce(ip_whitelist, '[]'::jsonb)) AS ip_whitelist_count,
    jsonb_array_length(coalesce(ip_blacklist, '[]'::jsonb)) AS ip_blacklist_count,
    quota,
    quota_used,
    expires_at,
    rate_limit_5h,
    rate_limit_1d,
    rate_limit_7d,
    usage_5h,
    usage_1d,
    usage_7d,
    created_at,
    updated_at,
    deleted_at IS NOT NULL AS deleted
  FROM api_keys
  ORDER BY created_at DESC
  LIMIT ${limit_api_keys}
) t
SQL

copy_query accounts_sample.jsonl <<SQL
SELECT row_to_json(t)
FROM (
  SELECT
    id,
    md5(coalesce(name, '')) AS name_hash,
    notes IS NOT NULL AND notes <> '' AS has_notes,
    platform,
    type,
    (
      SELECT coalesce(array_agg(k ORDER BY k), ARRAY[]::text[])
      FROM jsonb_object_keys(coalesce(credentials, '{}'::jsonb)) AS k
    ) AS credential_keys,
    (
      SELECT coalesce(array_agg(k ORDER BY k), ARRAY[]::text[])
      FROM jsonb_object_keys(coalesce(extra, '{}'::jsonb)) AS k
    ) AS extra_keys,
    proxy_id IS NOT NULL AS has_proxy,
    concurrency,
    load_factor,
    priority,
    rate_multiplier,
    status,
    CASE WHEN error_message IS NULL OR error_message = '' THEN NULL ELSE md5(error_message) END AS error_hash,
    last_used_at,
    expires_at,
    auto_pause_on_expired,
    schedulable,
    rate_limited_at,
    rate_limit_reset_at,
    overload_until,
    temp_unschedulable_until,
    temp_unschedulable_reason IS NOT NULL AND temp_unschedulable_reason <> '' AS has_temp_unschedulable_reason,
    created_at,
    updated_at,
    deleted_at IS NOT NULL AS deleted
  FROM accounts
  ORDER BY updated_at DESC
  LIMIT ${limit_accounts}
) t
SQL

copy_query usage_logs_sample.jsonl <<SQL
SELECT row_to_json(t)
FROM (
  SELECT
    id,
    user_id,
    api_key_id,
    account_id,
    CASE WHEN request_id IS NULL OR request_id = '' THEN NULL ELSE md5(request_id) END AS request_hash,
    model,
    input_tokens,
    output_tokens,
    cache_creation_tokens,
    cache_read_tokens,
    cache_creation_5m_tokens,
    cache_creation_1h_tokens,
    input_cost,
    output_cost,
    cache_creation_cost,
    cache_read_cost,
    total_cost,
    actual_cost,
    stream,
    duration_ms,
    created_at
  FROM usage_logs
  WHERE created_at >= now() - (${usage_days} || ' days')::interval
  ORDER BY created_at DESC
  LIMIT ${limit_usage}
) t
SQL

tar -czf - -C "${tmp}" .
REMOTE_SCRIPT

tar -xzf "${ARCHIVE}" -C "${OUTPUT_DIR}"
log "完成: ${OUTPUT_DIR}"
