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
pg_dir="${backup_dir}/postgres"
minio_dir="${backup_dir}/minio"
manifest="${backup_dir}/manifest.env"

mkdir -p "${pg_dir}" "${minio_dir}"

dump_path="${pg_dir}/animus.dump"
pg_dump --format=custom --file="${dump_path}" "${DATABASE_URL}"

dump_sha="$(sha256sum "${dump_path}" | awk '{print $1}')"

{
  echo "BACKUP_TIMESTAMP_UTC=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  echo "BACKUP_DIR=${backup_dir}"
  echo "POSTGRES_DUMP_SHA256=${dump_sha}"
} > "${manifest}"

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
      mkdir -p "${minio_dir}/${bucket}"
      mc mirror --overwrite --quiet "animus/${bucket}" "${minio_dir}/${bucket}"
    done
  elif command -v aws >/dev/null 2>&1; then
    export AWS_ACCESS_KEY_ID="${ANIMUS_MINIO_ACCESS_KEY}"
    export AWS_SECRET_ACCESS_KEY="${ANIMUS_MINIO_SECRET_KEY}"
    export AWS_EC2_METADATA_DISABLED=true
    endpoint_url="${scheme}://${endpoint}"
    for bucket in ${buckets}; do
      mkdir -p "${minio_dir}/${bucket}"
      aws s3 sync --only-show-errors --endpoint-url "${endpoint_url}" "s3://${bucket}" "${minio_dir}/${bucket}"
    done
  else
    echo "minio endpoint set but neither mc nor aws cli found" >&2
    exit 1
  fi
  echo "MINIO_BUCKETS=${buckets}" >> "${manifest}"
fi

echo "backup complete: ${backup_dir}"
