#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/go_env.sh"

SYFT_VERSION="${SYFT_VERSION:-v0.103.1}"
BIN_DIR="${ROOT_DIR}/.bin"
OUTPUT_DIR="${SUPPLY_CHAIN_OUT:-${ROOT_DIR}/.cache/supply-chain}"
SBOM_DIR="${SBOM_DIR:-${OUTPUT_DIR}/sbom}"
IMAGES_JSON="${ANIMUS_IMAGES_JSON:-${ROOT_DIR}/artifacts/images.json}"
SBOM_INDEX="${SBOM_INDEX:-${SBOM_DIR}/index.json}"
INSTALL_TOOLS="${ANIMUS_INSTALL_TOOLS:-0}"

mkdir -p "${BIN_DIR}" "${SBOM_DIR}"

if ! command -v syft >/dev/null 2>&1; then
  if [[ "${INSTALL_TOOLS}" == "1" ]]; then
    GOBIN="${BIN_DIR}" go install "github.com/anchore/syft/cmd/syft@${SYFT_VERSION}"
    export PATH="${BIN_DIR}:${PATH}"
  else
    echo "sbom: syft not found; set ANIMUS_INSTALL_TOOLS=1 or install syft manually" >&2
    exit 1
  fi
fi

declare -a image_names
declare -a image_refs
declare -a image_digests

if [[ -f "${IMAGES_JSON}" ]]; then
  while IFS= read -r line; do
    name="$(echo "${line}" | sed -n 's/.*"name":"\([^"]*\)".*/\1/p')"
    reference="$(echo "${line}" | sed -n 's/.*"reference":"\([^"]*\)".*/\1/p')"
    digest="$(echo "${line}" | sed -n 's/.*"digest":"\([^"]*\)".*/\1/p')"
    if [[ -z "${name}" || -z "${reference}" || -z "${digest}" ]]; then
      continue
    fi
    image_names+=("${name}")
    image_refs+=("${reference}")
    image_digests+=("${digest}")
  done < <(rg '^[[:space:]]*\{"name":"' "${IMAGES_JSON}")
else
  mapfile -t fallback_images < <("${ROOT_DIR}/scripts/list_images.sh" "$@")
  for image in "${fallback_images[@]}"; do
    safe="$(echo "${image}" | tr '/:@' '____')"
    digest=""
    if [[ "${image}" == *@sha256:* ]]; then
      digest="${image##*@}"
    fi
    image_names+=("${safe}")
    image_refs+=("${image}")
    image_digests+=("${digest}")
  done
fi

if [[ "${#image_refs[@]}" -eq 0 ]]; then
  echo "sbom: no images found" >&2
  exit 1
fi

declare -a index_entries
for i in "${!image_refs[@]}"; do
  name="${image_names[${i}]}"
  reference="${image_refs[${i}]}"
  digest="${image_digests[${i}]}"
  out="${SBOM_DIR}/${name}.spdx.json"
  out_rel="${out#${ROOT_DIR}/}"
  echo "==> sbom ${reference}"
  syft "${reference}" -o spdx-json="${out}"
  index_entries+=("{\"name\":\"${name}\",\"reference\":\"${reference}\",\"digest\":\"${digest}\",\"sbom\":\"${out_rel}\"}")
done

{
  echo "{"
  echo "  \"schema_version\": \"v1\","
  echo "  \"generated_at\": \"$(date -u +%Y-%m-%dT%H:%M:%SZ)\","
  echo "  \"entries\": ["
  for i in "${!index_entries[@]}"; do
    comma=","
    if [[ "${i}" -eq "$(( ${#index_entries[@]} - 1 ))" ]]; then
      comma=""
    fi
    echo "    ${index_entries[${i}]}${comma}"
  done
  echo "  ]"
  echo "}"
} > "${SBOM_INDEX}"

printf '%s\n' "${SBOM_DIR}" > "${OUTPUT_DIR}/sbom_dir.txt"
printf '%s\n' "${SBOM_INDEX}" > "${OUTPUT_DIR}/sbom_index.txt"
