#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/go_env.sh"

GRYPE_VERSION="${GRYPE_VERSION:-v0.78.0}"
BIN_DIR="${ROOT_DIR}/.bin"
OUTPUT_DIR="${SUPPLY_CHAIN_OUT:-${ROOT_DIR}/.cache/supply-chain}"
VULN_DIR="${OUTPUT_DIR}/vuln"
FAIL_ON="${VULN_FAIL_ON:-}" # e.g., critical, high, medium

mkdir -p "${BIN_DIR}" "${VULN_DIR}"

if ! command -v grype >/dev/null 2>&1; then
  GOBIN="${BIN_DIR}" go install "github.com/anchore/grype/cmd/grype@${GRYPE_VERSION}"
  export PATH="${BIN_DIR}:${PATH}"
fi

mapfile -t IMAGES < <("${ROOT_DIR}/scripts/list_images.sh" "$@")
if [[ "${#IMAGES[@]}" -eq 0 ]]; then
  echo "no images found"
  exit 1
fi

for image in "${IMAGES[@]}"; do
  safe=$(echo "${image}" | tr '/:@' '____')
  out="${VULN_DIR}/${safe}.json"
  echo "==> vuln scan ${image}"
  if [[ -n "${FAIL_ON}" ]]; then
    grype "${image}" -o json --fail-on "${FAIL_ON}" > "${out}"
  else
    grype "${image}" -o json > "${out}" || true
  fi
done

printf '%s\n' "${VULN_DIR}" > "${OUTPUT_DIR}/vuln_dir.txt"
