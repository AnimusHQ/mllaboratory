#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/lib/paths.sh"

require_bin() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required tool: $1" >&2
    exit 1
  fi
}

require_bin docker

if ! docker buildx version >/dev/null 2>&1; then
  echo "docker buildx is required" >&2
  exit 1
fi

export DOCKER_CONFIG="${DOCKER_CONFIG:-${ROOT_DIR}/.cache/docker-config}"
mkdir -p "${DOCKER_CONFIG}"

DEPLOY_DIR="$(animus_deploy_dir)"
GO_VERSION="${ANIMUS_GO_VERSION:-1.25}"
IMAGE_REPO="${ANIMUS_IMAGE_REPO:-animus}"
IMAGE_TAG="${ANIMUS_IMAGE_TAG:-}"
IMAGE_PLATFORM="${ANIMUS_IMAGE_PLATFORM:-linux/amd64}"
IMAGE_PUSH="${ANIMUS_IMAGE_PUSH:-0}"
BUILD_UI="${ANIMUS_BUILD_UI:-0}"
DOCKERFILE="${DEPLOY_DIR}/docker/Dockerfile.service"
UI_DOCKERFILE="${DEPLOY_DIR}/docker/Dockerfile.ui"
ARTIFACTS_DIR="${ANIMUS_ARTIFACTS_DIR:-${ROOT_DIR}/artifacts}"
IMAGES_JSON="${ANIMUS_IMAGES_JSON:-${ARTIFACTS_DIR}/images.json}"
IMAGES_TXT="${ANIMUS_IMAGES_TXT:-${ARTIFACTS_DIR}/images.txt}"
CP_VALUES_OUT="${ANIMUS_CP_VALUES_OUT:-${ARTIFACTS_DIR}/helm/animus-datapilot-values-production.generated.yaml}"
DP_VALUES_OUT="${ANIMUS_DP_VALUES_OUT:-${ARTIFACTS_DIR}/helm/animus-dataplane-values-production.generated.yaml}"
BUILDX_BUILDER="${ANIMUS_BUILDX_BUILDER:-animus-builder}"

mkdir -p "$(dirname "${IMAGES_JSON}")" "$(dirname "${IMAGES_TXT}")" "$(dirname "${CP_VALUES_OUT}")" "$(dirname "${DP_VALUES_OUT}")"

if [[ -z "${IMAGE_TAG}" ]]; then
  if command -v git >/dev/null 2>&1 && git -C "${ROOT_DIR}" rev-parse --short HEAD >/dev/null 2>&1; then
    IMAGE_TAG="$(git -C "${ROOT_DIR}" describe --tags --always --dirty)"
    IMAGE_TAG="${IMAGE_TAG//\//-}"
  else
    IMAGE_TAG="local-$(date +%Y%m%d%H%M%S)"
  fi
fi

if [[ "${IMAGE_TAG}" == "latest" ]]; then
  echo "build-images: mutable tag 'latest' is not allowed" >&2
  exit 1
fi

ANIMUS_VERSION="${ANIMUS_VERSION:-${IMAGE_TAG}}"
VCS_REF="$(git -C "${ROOT_DIR}" rev-parse --short=12 HEAD 2>/dev/null || echo "unknown")"
BUILD_DATE="$(git -C "${ROOT_DIR}" show -s --format=%cI HEAD 2>/dev/null || date -u +%Y-%m-%dT%H:%M:%SZ)"
SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH:-$(git -C "${ROOT_DIR}" show -s --format=%ct HEAD 2>/dev/null || date -u +%s)}"

if ! docker buildx inspect "${BUILDX_BUILDER}" >/dev/null 2>&1; then
  docker buildx create --name "${BUILDX_BUILDER}" --driver docker-container >/dev/null
fi
docker buildx use "${BUILDX_BUILDER}" >/dev/null

declare -a entries
declare -a references
declare -A digest_by_name

build_service() {
  local name="$1"
  local image_tag_ref="${IMAGE_REPO}/${name}:${IMAGE_TAG}"
  local digest=""

  echo "build-images: ${name}"
  docker buildx build \
    --platform "${IMAGE_PLATFORM}" \
    --build-arg GO_VERSION="${GO_VERSION}" \
    --build-arg SERVICE="${name}" \
    --build-arg SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH}" \
    --label "org.opencontainers.image.version=${ANIMUS_VERSION}" \
    --label "org.opencontainers.image.revision=${VCS_REF}" \
    --label "org.opencontainers.image.created=${BUILD_DATE}" \
    -f "${DOCKERFILE}" \
    -t "${image_tag_ref}" \
    $( [[ "${IMAGE_PUSH}" == "1" ]] && echo --push || echo --load ) \
    "${ROOT_DIR}"

  if [[ "${IMAGE_PUSH}" == "1" ]]; then
    digest="$(docker buildx imagetools inspect "${image_tag_ref}" | awk '/Digest:/ {print $2; exit}')"
  else
    digest="$(docker image inspect --format '{{.Id}}' "${image_tag_ref}")"
  fi
  if [[ ! "${digest}" =~ ^sha256:[0-9a-f]{64}$ ]]; then
    echo "build-images: failed to resolve digest for ${name} (${digest})" >&2
    exit 1
  fi

  local reference="${IMAGE_REPO}/${name}@${digest}"
  digest_by_name["${name}"]="${digest}"
  references+=("${reference}")
  entries+=("{\"name\":\"${name}\",\"tag\":\"${image_tag_ref}\",\"digest\":\"${digest}\",\"reference\":\"${reference}\",\"platform\":\"${IMAGE_PLATFORM}\"}")
}

build_ui() {
  local name="ui"
  local image_tag_ref="${IMAGE_REPO}/${name}:${IMAGE_TAG}"
  local digest=""

  echo "build-images: ${name}"
  docker buildx build \
    --platform "${IMAGE_PLATFORM}" \
    --build-arg SOURCE_DATE_EPOCH="${SOURCE_DATE_EPOCH}" \
    --label "org.opencontainers.image.version=${ANIMUS_VERSION}" \
    --label "org.opencontainers.image.revision=${VCS_REF}" \
    --label "org.opencontainers.image.created=${BUILD_DATE}" \
    -f "${UI_DOCKERFILE}" \
    -t "${image_tag_ref}" \
    $( [[ "${IMAGE_PUSH}" == "1" ]] && echo --push || echo --load ) \
    "${ROOT_DIR}"

  if [[ "${IMAGE_PUSH}" == "1" ]]; then
    digest="$(docker buildx imagetools inspect "${image_tag_ref}" | awk '/Digest:/ {print $2; exit}')"
  else
    digest="$(docker image inspect --format '{{.Id}}' "${image_tag_ref}")"
  fi
  if [[ ! "${digest}" =~ ^sha256:[0-9a-f]{64}$ ]]; then
    echo "build-images: failed to resolve digest for ui (${digest})" >&2
    exit 1
  fi

  local reference="${IMAGE_REPO}/${name}@${digest}"
  digest_by_name["${name}"]="${digest}"
  references+=("${reference}")
  entries+=("{\"name\":\"${name}\",\"tag\":\"${image_tag_ref}\",\"digest\":\"${digest}\",\"reference\":\"${reference}\",\"platform\":\"${IMAGE_PLATFORM}\"}")
}

SERVICES=(
  gateway
  experiments
  dataset-registry
  quality
  lineage
  audit
  dataplane
)

for svc in "${SERVICES[@]}"; do
  build_service "${svc}"
done

if [[ "${BUILD_UI}" == "1" ]]; then
  build_ui
fi

{
  echo "{"
  echo "  \"schema_version\": \"v1\","
  echo "  \"repository\": \"${IMAGE_REPO}\","
  echo "  \"tag\": \"${IMAGE_TAG}\","
  echo "  \"version\": \"${ANIMUS_VERSION}\","
  echo "  \"revision\": \"${VCS_REF}\","
  echo "  \"platform\": \"${IMAGE_PLATFORM}\","
  echo "  \"created_at\": \"${BUILD_DATE}\","
  echo "  \"images\": ["
  for i in "${!entries[@]}"; do
    comma=","
    if [[ "${i}" -eq "$(( ${#entries[@]} - 1 ))" ]]; then
      comma=""
    fi
    echo "    ${entries[${i}]}${comma}"
  done
  echo "  ]"
  echo "}"
} > "${IMAGES_JSON}"

printf '%s\n' "${references[@]}" > "${IMAGES_TXT}"

cat > "${CP_VALUES_OUT}" <<EOF_CP
profile: production
image:
  repository: ${IMAGE_REPO}
  digests:
    gateway: ${digest_by_name[gateway]}
    experiments: ${digest_by_name[experiments]}
    dataset-registry: ${digest_by_name[dataset-registry]}
    quality: ${digest_by_name[quality]}
    lineage: ${digest_by_name[lineage]}
    audit: ${digest_by_name[audit]}
training:
  executor: disabled
ui:
  enabled: $( [[ -n "${digest_by_name[ui]:-}" ]] && echo true || echo false )
  image:
    digest: ${digest_by_name[ui]:-}
EOF_CP

cat > "${DP_VALUES_OUT}" <<EOF_DP
profile: production
image:
  repository: ${IMAGE_REPO}
  digest: ${digest_by_name[dataplane]}
EOF_DP

echo "build-images: wrote manifest ${IMAGES_JSON}"
echo "build-images: wrote references ${IMAGES_TXT}"
echo "build-images: wrote datapilot values ${CP_VALUES_OUT}"
echo "build-images: wrote dataplane values ${DP_VALUES_OUT}"
