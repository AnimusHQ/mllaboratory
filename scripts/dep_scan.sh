#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [ "${ANIMUS_DEP_SCAN:-}" != "1" ]; then
  echo "dep-scan: ANIMUS_DEP_SCAN not set; skipping."
  exit 0
fi

GRYPE_VERSION="${GRYPE_VERSION:-v0.78.0}"
BIN_DIR="${ROOT_DIR}/.bin"
OUTPUT_DIR="${DEP_SCAN_OUT:-${ROOT_DIR}/.cache/supply-chain/dep-scan}"
FAIL_ON="${DEP_SCAN_FAIL_ON:-}"

mkdir -p "${BIN_DIR}" "${OUTPUT_DIR}"

if ! command -v grype >/dev/null 2>&1; then
  if [ "${ANIMUS_DEP_SCAN_INSTALL:-}" = "1" ]; then
    GOBIN="${BIN_DIR}" go install "github.com/anchore/grype/cmd/grype@${GRYPE_VERSION}"
    export PATH="${BIN_DIR}:${PATH}"
  else
    echo "dep-scan: grype not found; set ANIMUS_DEP_SCAN_INSTALL=1 or install manually."
    exit 1
  fi
fi

mapfile -t IMAGES < <("${ROOT_DIR}/scripts/list_images.sh" "$@")
if [[ "${#IMAGES[@]}" -eq 0 ]]; then
  echo "dep-scan: no images found"
  exit 1
fi

for image in "${IMAGES[@]}"; do
  safe=$(echo "${image}" | tr '/:@' '____')
  out="${OUTPUT_DIR}/${safe}.json"
  echo "==> dep scan ${image}"
  if [[ -n "${FAIL_ON}" ]]; then
    grype "${image}" -o json --fail-on "${FAIL_ON}" > "${out}"
  else
    grype "${image}" -o json > "${out}" || true
  fi
done

printf '%s\n' "${OUTPUT_DIR}" > "${OUTPUT_DIR}/dep_scan_dir.txt"
