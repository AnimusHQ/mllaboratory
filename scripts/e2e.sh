#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/go_env.sh"

ARGS=("-tags=e2e" "./closed/e2e" "-v")
if [[ "$#" -gt 0 ]]; then
  ARGS+=("$@")
fi

echo "==> go test ${ARGS[*]}"
exec go test "${ARGS[@]}"
