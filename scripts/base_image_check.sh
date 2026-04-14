#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/lib/paths.sh"

DEPLOY_DIR="$(animus_deploy_dir)"

DOCKERFILES=(
  "${DEPLOY_DIR}/docker/Dockerfile.service"
  "${DEPLOY_DIR}/docker/Dockerfile.ui"
)

fail=0

for dockerfile in "${DOCKERFILES[@]}"; do
  if [[ ! -f "${dockerfile}" ]]; then
    echo "base-image-check: missing Dockerfile: ${dockerfile}" >&2
    fail=1
    continue
  fi

  while IFS= read -r row; do
    line_no="${row%%:*}"
    line="${row#*:}"

    image_ref="$(echo "${line}" | awk '{
      image_idx=2
      if ($2 ~ /^--platform=/) {
        image_idx=3
      }
      if (NF >= image_idx) {
        print $image_idx
      }
    }')"

    if [[ -z "${image_ref}" ]]; then
      continue
    fi

    # Stage references like "FROM builder AS runtime" are allowed.
    if [[ "${image_ref}" != *":"* && "${image_ref}" != *"/"* && "${image_ref}" != *"@"* ]]; then
      continue
    fi

    if [[ ! "${image_ref}" =~ @sha256:[0-9a-f]{64}$ ]]; then
      echo "base-image-check: ${dockerfile}:${line_no} uses tag-only or non-digest base image: ${image_ref}" >&2
      fail=1
    fi
  done < <(awk 'toupper($1)=="FROM"{print NR ":" $0}' "${dockerfile}")
done

if [[ "${fail}" -ne 0 ]]; then
  exit 1
fi

echo "base-image-check: ok"
