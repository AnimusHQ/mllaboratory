#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/scripts/integration-compose.yml"

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required for integration harness" >&2
  exit 1
fi

if ! docker compose version >/dev/null 2>&1; then
  echo "docker compose is required for integration harness" >&2
  exit 1
fi

docker compose -f "${COMPOSE_FILE}" up -d

"${ROOT_DIR}/scripts/wait_port.sh" localhost 55432 40
"${ROOT_DIR}/scripts/wait_port.sh" localhost 59000 40

echo "integration services ready"
cat <<ENV
export ANIMUS_INTEGRATION=1
export ANIMUS_TEST_DATABASE_URL=postgres://animus:animus@localhost:55432/animus?sslmode=disable
export ANIMUS_TEST_MINIO_ENDPOINT=localhost:59000
export ANIMUS_TEST_MINIO_ACCESS_KEY=animus
export ANIMUS_TEST_MINIO_SECRET_KEY=animusminio
export ANIMUS_TEST_MINIO_BUCKET_DATASETS=datasets
export ANIMUS_TEST_MINIO_BUCKET_ARTIFACTS=artifacts
ENV
