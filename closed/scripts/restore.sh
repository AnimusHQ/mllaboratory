#!/usr/bin/env bash
set -euo pipefail

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "missing required env: ${name}" >&2
    exit 1
  fi
}

require_env BACKUP_DIR
require_env DATABASE_URL

backup_dir="${BACKUP_DIR}"
dump_path="${backup_dir}/postgres/animus.dump"

if [[ ! -f "${dump_path}" ]]; then
  echo "postgres dump not found: ${dump_path}" >&2
  exit 1
fi

pg_restore --clean --if-exists --no-owner --no-privileges --dbname "${DATABASE_URL}" "${dump_path}"

if [[ -n "${ANIMUS_MINIO_ENDPOINT:-}" ]]; then
  require_env ANIMUS_MINIO_ACCESS_KEY
  require_env ANIMUS_MINIO_SECRET_KEY
  buckets="${ANIMUS_MINIO_BUCKETS:-datasets artifacts}"
  endpoint="${ANIMUS_MINIO_ENDPOINT}"
  scheme="https"
  if [[ "${endpoint}" == http://* ]]; then
    scheme="http"
    endpoint="${endpoint#http://}"
  elif [[ "${endpoint}" == https://* ]]; then
    scheme="https"
    endpoint="${endpoint#https://}"
  fi

  if command -v mc >/dev/null 2>&1; then
    export MC_HOST_animus="${scheme}://${ANIMUS_MINIO_ACCESS_KEY}:${ANIMUS_MINIO_SECRET_KEY}@${endpoint}"
    for bucket in ${buckets}; do
      mc mb --ignore-existing "animus/${bucket}" >/dev/null 2>&1 || true
      mc mirror --overwrite --quiet "${backup_dir}/minio/${bucket}" "animus/${bucket}"
    done
  elif command -v aws >/dev/null 2>&1; then
    export AWS_ACCESS_KEY_ID="${ANIMUS_MINIO_ACCESS_KEY}"
    export AWS_SECRET_ACCESS_KEY="${ANIMUS_MINIO_SECRET_KEY}"
    export AWS_EC2_METADATA_DISABLED=true
    endpoint_url="${scheme}://${endpoint}"
    for bucket in ${buckets}; do
      aws s3 mb --endpoint-url "${endpoint_url}" "s3://${bucket}" >/dev/null 2>&1 || true
      aws s3 sync --only-show-errors --endpoint-url "${endpoint_url}" "${backup_dir}/minio/${bucket}" "s3://${bucket}"
    done
  else
    echo "minio endpoint set but neither mc nor aws cli found" >&2
    exit 1
  fi
fi

echo "restore complete: ${backup_dir}"
