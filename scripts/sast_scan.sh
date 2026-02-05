#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [ "${ANIMUS_SAST_SCAN:-}" != "1" ]; then
  echo "sast-scan: ANIMUS_SAST_SCAN not set; skipping."
  exit 0
fi

GOSEC_VERSION="${GOSEC_VERSION:-v2.21.2}"
BIN_DIR="${ROOT_DIR}/.bin"
OUTPUT_DIR="${SAST_OUT:-${ROOT_DIR}/.cache/supply-chain/sast}"

mkdir -p "${BIN_DIR}" "${OUTPUT_DIR}"

if ! command -v gosec >/dev/null 2>&1; then
  if [ "${ANIMUS_SAST_INSTALL:-}" = "1" ]; then
    GOBIN="${BIN_DIR}" go install "github.com/securego/gosec/v2/cmd/gosec@${GOSEC_VERSION}"
    export PATH="${BIN_DIR}:${PATH}"
  else
    echo "sast-scan: gosec not found; set ANIMUS_SAST_INSTALL=1 or install manually."
    exit 1
  fi
fi

echo "==> gosec"
gosec -fmt json -out "${OUTPUT_DIR}/gosec.json" ./...

printf '%s\n' "${OUTPUT_DIR}" > "${OUTPUT_DIR}/sast_dir.txt"
