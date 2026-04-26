#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/go_env.sh"

if [[ "$#" -eq 0 ]]; then
  set -- ./closed/...
fi

if [[ -n "${ANIMUS_GO_TEST_JSON:-}" ]]; then
  mkdir -p "$(dirname "${ANIMUS_GO_TEST_JSON}")"
  echo "==> go test -json $* (json: ${ANIMUS_GO_TEST_JSON})"
  go test -json "$@" | tee "${ANIMUS_GO_TEST_JSON}"
else
  echo "==> go test $*"
  exec go test "$@"
fi
