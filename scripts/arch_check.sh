#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/go_env.sh"

fail=0

require_no_match() {
  local pattern="$1"
  shift
  local message="$1"
  shift
  if rg -n --glob '!**/*_test.go' "${pattern}" "$@" >/dev/null 2>&1; then
    echo "arch-check: ${message}" >&2
    rg -n --glob '!**/*_test.go' "${pattern}" "$@" >&2 || true
    fail=1
  fi
}

# Control-plane must not import runtime execution primitives.
require_no_match '"github.com/animus-labs/animus-go/closed/internal/runtimeexec"' \
  'control-plane code must not import runtime execution primitives' \
  "${ROOT_DIR}/closed/experiments" \
  "${ROOT_DIR}/closed/internal" \
  "${ROOT_DIR}/closed/audit"

# Control-plane packages must not perform direct runtime secret fetch requests.
require_no_match 'secrets\.Request\{' \
  'control-plane code must not construct raw secrets.Request (use secrets.FetchIntegrationSecret)' \
  "${ROOT_DIR}/closed/experiments" \
  "${ROOT_DIR}/closed/internal/integrations" \
  "${ROOT_DIR}/closed/internal/auditexport" \
  "${ROOT_DIR}/closed/audit"

# Control-plane packages must not import dataplane package directly.
require_no_match '"github.com/animus-labs/animus-go/closed/dataplane"' \
  'control-plane code must not import dataplane package directly' \
  "${ROOT_DIR}/closed/experiments" \
  "${ROOT_DIR}/closed/internal" \
  "${ROOT_DIR}/closed/audit"

require_no_match 'ANIMUS_CP_EXECUTION_ENABLE|ANIMUS_TRAINING_EXECUTOR' \
  'control-plane must not include legacy CP execution env toggles' \
  "${ROOT_DIR}/closed/experiments"

if [[ "${fail}" -ne 0 ]]; then
  exit 1
fi

echo "arch-check: ok"
