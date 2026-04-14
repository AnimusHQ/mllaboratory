#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
QUICKSTART="${ROOT_DIR}/open/demo/quickstart.sh"

if [[ ! -f "${QUICKSTART}" ]]; then
  echo "dev: quickstart script not found: ${QUICKSTART}" >&2
  exit 1
fi

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  cat <<'USAGE'
Usage: make dev [DEV_ARGS="--smoke|--down"]

Canonical local developer path for this repository.
Delegates to open/demo/quickstart.sh.
USAGE
  exit 0
fi

if [[ -n "${DEV_ARGS:-}" ]]; then
  # shellcheck disable=SC2086
  set -- ${DEV_ARGS}
fi

exec bash "${QUICKSTART}" "$@"
