#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/go_env.sh"

COSIGN_VERSION="${COSIGN_VERSION:-v2.4.1}"
BIN_DIR="${ROOT_DIR}/.bin"
IMAGES_JSON="${ANIMUS_IMAGES_JSON:-${ROOT_DIR}/artifacts/images.json}"
ARTIFACTS_DIR="${ANIMUS_ARTIFACTS_DIR:-${ROOT_DIR}/artifacts}"
ATTEST_DIR="${ANIMUS_ATTEST_DIR:-${ARTIFACTS_DIR}/attestations}"
INSTALL_TOOLS="${ANIMUS_INSTALL_TOOLS:-0}"
COSIGN_KEY="${COSIGN_KEY:-}"

mkdir -p "${BIN_DIR}" "${ATTEST_DIR}"

if [[ ! -f "${IMAGES_JSON}" ]]; then
  echo "sign-images: images manifest missing: ${IMAGES_JSON}" >&2
  exit 1
fi

if ! command -v cosign >/dev/null 2>&1; then
  if [[ "${INSTALL_TOOLS}" == "1" ]]; then
    GOBIN="${BIN_DIR}" go install "github.com/sigstore/cosign/v2/cmd/cosign@${COSIGN_VERSION}"
    export PATH="${BIN_DIR}:${PATH}"
  else
    echo "sign-images: cosign not found; set ANIMUS_INSTALL_TOOLS=1 or install cosign manually" >&2
    exit 1
  fi
fi

while IFS= read -r line; do
  name="$(echo "${line}" | sed -n 's/.*"name":"\([^"]*\)".*/\1/p')"
  reference="$(echo "${line}" | sed -n 's/.*"reference":"\([^"]*\)".*/\1/p')"
  digest="$(echo "${line}" | sed -n 's/.*"digest":"\([^"]*\)".*/\1/p')"
  if [[ -z "${name}" || -z "${reference}" || -z "${digest}" ]]; then
    continue
  fi

  predicate="${ATTEST_DIR}/${name}-provenance.json"
  cat > "${predicate}" <<EOF_PRED
{
  "buildType": "https://animus.dev/build/container/v1",
  "builder": {
    "id": "animus-go"
  },
  "invocation": {
    "configSource": {
      "uri": "${GITHUB_SERVER_URL:-local}/${GITHUB_REPOSITORY:-animus-go}",
      "digest": {
        "sha1": "${GITHUB_SHA:-$(git -C "${ROOT_DIR}" rev-parse HEAD 2>/dev/null || echo unknown)}"
      }
    }
  },
  "metadata": {
    "name": "${name}",
    "digest": "${digest}",
    "createdAt": "$(date -u +%Y-%m-%dT%H:%M:%SZ)"
  }
}
EOF_PRED

  echo "sign-images: signing ${reference}"
  if [[ -n "${COSIGN_KEY}" ]]; then
    tlog_upload="${COSIGN_TLOG_UPLOAD:-false}"
    cosign sign --yes --key "${COSIGN_KEY}" --tlog-upload="${tlog_upload}" "${reference}"
    cosign attest --yes --key "${COSIGN_KEY}" --type slsaprovenance --predicate "${predicate}" --tlog-upload="${tlog_upload}" "${reference}"
  else
    cosign sign --yes "${reference}"
    cosign attest --yes --type slsaprovenance --predicate "${predicate}" "${reference}"
  fi
done < <(rg '^[[:space:]]*\{"name":"' "${IMAGES_JSON}")

echo "sign-images: ok"
