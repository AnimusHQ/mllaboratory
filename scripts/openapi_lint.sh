#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/go_env.sh"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/lib/paths.sh"

if [[ -n "${GOFLAGS:-}" ]]; then
  export GOFLAGS="${GOFLAGS} -mod=vendor"
else
  export GOFLAGS="-mod=vendor"
fi

CONTRACTS_DIR="$(animus_contracts_dir)"
SPECS=(
  "${CONTRACTS_DIR}/openapi/experiments.yaml"
  "${CONTRACTS_DIR}/openapi/dataplane_internal.yaml"
  "${CONTRACTS_DIR}/openapi/dataset-registry.yaml"
  "${CONTRACTS_DIR}/openapi/quality.yaml"
  "${CONTRACTS_DIR}/openapi/lineage.yaml"
  "${CONTRACTS_DIR}/openapi/audit.yaml"
  "${CONTRACTS_DIR}/openapi/gateway.yaml"
)

for spec in "${SPECS[@]}"; do
  if [[ ! -f "${spec}" ]]; then
    echo "missing spec: ${spec}"
    exit 1
  fi
done

FILES_CSV="$(IFS=,; echo "${SPECS[*]}")"
echo "==> openapi lint ${FILES_CSV}"
go run "${ROOT_DIR}/cmd/openapi-lint" --files "${FILES_CSV}"
