#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/go_env.sh"

COSIGN_VERSION="${COSIGN_VERSION:-v2.4.1}"
BIN_DIR="${ROOT_DIR}/.bin"
IMAGES_JSON="${ANIMUS_IMAGES_JSON:-${ROOT_DIR}/artifacts/images.json}"
INSTALL_TOOLS="${ANIMUS_INSTALL_TOOLS:-0}"
COSIGN_PUBLIC_KEY="${COSIGN_PUBLIC_KEY:-}"
IDENTITY_RE="${COSIGN_CERT_IDENTITY_REGEXP:-https://github.com/.+}"
OIDC_ISSUER="${COSIGN_CERT_OIDC_ISSUER:-https://token.actions.githubusercontent.com}"

mkdir -p "${BIN_DIR}"

if [[ ! -f "${IMAGES_JSON}" ]]; then
  echo "verify-images: images manifest missing: ${IMAGES_JSON}" >&2
  exit 1
fi

if ! command -v cosign >/dev/null 2>&1; then
  if [[ "${INSTALL_TOOLS}" == "1" ]]; then
    GOBIN="${BIN_DIR}" go install "github.com/sigstore/cosign/v2/cmd/cosign@${COSIGN_VERSION}"
    export PATH="${BIN_DIR}:${PATH}"
  else
    echo "verify-images: cosign not found; set ANIMUS_INSTALL_TOOLS=1 or install cosign manually" >&2
    exit 1
  fi
fi

while IFS= read -r line; do
  reference="$(echo "${line}" | sed -n 's/.*"reference":"\([^"]*\)".*/\1/p')"
  if [[ -z "${reference}" ]]; then
    continue
  fi

  echo "verify-images: ${reference}"
  if [[ -n "${COSIGN_PUBLIC_KEY}" ]]; then
    cosign verify --key "${COSIGN_PUBLIC_KEY}" "${reference}" >/dev/null
    cosign verify-attestation --key "${COSIGN_PUBLIC_KEY}" --type slsaprovenance "${reference}" >/dev/null
  else
    cosign verify \
      --certificate-identity-regexp "${IDENTITY_RE}" \
      --certificate-oidc-issuer "${OIDC_ISSUER}" \
      "${reference}" >/dev/null
    cosign verify-attestation \
      --type slsaprovenance \
      --certificate-identity-regexp "${IDENTITY_RE}" \
      --certificate-oidc-issuer "${OIDC_ISSUER}" \
      "${reference}" >/dev/null
  fi

done < <(rg '^[[:space:]]*\{"name":"' "${IMAGES_JSON}")

echo "verify-images: ok"
