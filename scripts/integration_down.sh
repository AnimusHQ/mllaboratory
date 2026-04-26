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

docker compose -f "${COMPOSE_FILE}" down -v
