#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
IMAGES_JSON="${ANIMUS_IMAGES_JSON:-${ROOT_DIR}/artifacts/images.json}"
OUTPUT_DIR="${SUPPLY_CHAIN_OUT:-${ROOT_DIR}/.cache/supply-chain}"
SBOM_DIR="${SBOM_DIR:-${OUTPUT_DIR}/sbom}"
SBOM_INDEX="${SBOM_INDEX:-${SBOM_DIR}/index.json}"

if [[ ! -f "${IMAGES_JSON}" ]]; then
  echo "sbom-check: images manifest missing: ${IMAGES_JSON}" >&2
  exit 1
fi
if [[ ! -f "${SBOM_INDEX}" ]]; then
  echo "sbom-check: sbom index missing: ${SBOM_INDEX}" >&2
  exit 1
fi

fail=0

while IFS= read -r line; do
  name="$(echo "${line}" | sed -n 's/.*"name":"\([^"]*\)".*/\1/p')"
  digest="$(echo "${line}" | sed -n 's/.*"digest":"\([^"]*\)".*/\1/p')"
  reference="$(echo "${line}" | sed -n 's/.*"reference":"\([^"]*\)".*/\1/p')"
  if [[ -z "${name}" || -z "${digest}" || -z "${reference}" ]]; then
    continue
  fi

  index_line="$(rg -m1 "\"name\":\"${name}\".*\"digest\":\"${digest}\"" "${SBOM_INDEX}" || true)"
  if [[ -z "${index_line}" ]]; then
    echo "sbom-check: index entry missing for ${name} (${digest})" >&2
    fail=1
    continue
  fi

  sbom_path="$(echo "${index_line}" | sed -n 's/.*"sbom":"\([^"]*\)".*/\1/p')"
  if [[ -z "${sbom_path}" ]]; then
    echo "sbom-check: sbom path missing for ${name}" >&2
    fail=1
    continue
  fi
  if [[ "${sbom_path}" != /* ]]; then
    sbom_path="${ROOT_DIR}/${sbom_path}"
  fi
  if [[ ! -f "${sbom_path}" ]]; then
    echo "sbom-check: sbom file not found for ${name}: ${sbom_path}" >&2
    fail=1
    continue
  fi

  if ! grep -Fq "${digest}" "${sbom_path}"; then
    echo "sbom-check: digest ${digest} not found in sbom metadata for ${name}" >&2
    fail=1
  fi

done < <(rg '^[[:space:]]*\{"name":"' "${IMAGES_JSON}")

if [[ "${fail}" -ne 0 ]]; then
  exit 1
fi

echo "sbom-check: ok"
