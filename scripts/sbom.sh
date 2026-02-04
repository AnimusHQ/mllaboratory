#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/go_env.sh"

SYFT_VERSION="${SYFT_VERSION:-v0.103.1}"
BIN_DIR="${ROOT_DIR}/.bin"
OUTPUT_DIR="${SUPPLY_CHAIN_OUT:-${ROOT_DIR}/.cache/supply-chain}"
SBOM_DIR="${OUTPUT_DIR}/sbom"

mkdir -p "${BIN_DIR}" "${SBOM_DIR}"

if ! command -v syft >/dev/null 2>&1; then
  GOBIN="${BIN_DIR}" go install "github.com/anchore/syft/cmd/syft@${SYFT_VERSION}"
  export PATH="${BIN_DIR}:${PATH}"
fi

mapfile -t IMAGES < <("${ROOT_DIR}/scripts/list_images.sh" "$@")
if [[ "${#IMAGES[@]}" -eq 0 ]]; then
  echo "no images found"
  exit 1
fi

for image in "${IMAGES[@]}"; do
  safe=$(echo "${image}" | tr '/:@' '____')
  out="${SBOM_DIR}/${safe}.spdx.json"
  echo "==> sbom ${image}"
  syft "${image}" -o spdx-json="${out}"
done

printf '%s\n' "${SBOM_DIR}" > "${OUTPUT_DIR}/sbom_dir.txt"
