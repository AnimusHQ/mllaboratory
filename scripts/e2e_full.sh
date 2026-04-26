#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/go_env.sh"

if [[ -z "${ANIMUS_E2E_GATEWAY_URL:-}" && -f "${ROOT_DIR}/.cache/system_env" ]]; then
  # shellcheck source=/dev/null
  source "${ROOT_DIR}/.cache/system_env"
fi

if [[ -z "${ANIMUS_E2E_GATEWAY_URL:-}" ]]; then
  echo "e2e-full: ANIMUS_E2E_GATEWAY_URL not set; skipping."
  exit 0
fi

ARTIFACTS_DIR="${ANIMUS_ARTIFACTS_DIR:-}"
if [[ -n "$ARTIFACTS_DIR" ]]; then
  mkdir -p "$ARTIFACTS_DIR"
  export ANIMUS_E2E_ARTIFACTS_DIR="$ARTIFACTS_DIR"
fi

export ANIMUS_E2E_FAILURES="${ANIMUS_E2E_FAILURES:-1}"

ARGS=("-tags=e2e" "./closed/e2e" "-v")
if [[ "$#" -gt 0 ]]; then
  ARGS+=("$@")
fi

if [[ -n "$ARTIFACTS_DIR" ]]; then
  echo "==> go test ${ARGS[*]} (artifacts: $ARTIFACTS_DIR/go-test-e2e.json)"
  go test -json "${ARGS[@]}" | tee "$ARTIFACTS_DIR/go-test-e2e.json"
else
  echo "==> go test ${ARGS[*]}"
  go test "${ARGS[@]}"
fi
