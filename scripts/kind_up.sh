#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/lib/paths.sh"
CACHE_DIR="${ROOT_DIR}/.cache"
DEPLOY_DIR="$(animus_deploy_dir)"

if [[ "${ANIMUS_SYSTEM_ENABLE:-}" != "1" ]]; then
  echo "system-up: ANIMUS_SYSTEM_ENABLE not set; skipping."
  exit 0
fi

require_bin() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required tool: $1" >&2
    exit 1
  fi
}

require_bin kind
require_bin kubectl
require_bin helm
require_bin docker

CLUSTER_NAME="${ANIMUS_KIND_CLUSTER_NAME:-animus-fullstack}"
NAMESPACE="${ANIMUS_SYSTEM_NAMESPACE:-animus-system}"
DATAPILOT_RELEASE="${ANIMUS_SYSTEM_DATAPILOT_RELEASE:-animus-datapilot}"
DATAPLANE_RELEASE="${ANIMUS_SYSTEM_DATAPLANE_RELEASE:-animus-dataplane}"
INTERNAL_AUTH_SECRET="${ANIMUS_SYSTEM_INTERNAL_AUTH_SECRET:-animus-internal-e2e-secret}"
GATEWAY_PORT="${ANIMUS_SYSTEM_GATEWAY_PORT:-18080}"
POSTGRES_PORT="${ANIMUS_SYSTEM_POSTGRES_PORT:-15432}"

DATAPILOT_IMAGE="${ANIMUS_SYSTEM_CP_IMAGE:-${ANIMUS_SYSTEM_IMAGE:-}}"
DATAPLANE_IMAGE="${ANIMUS_SYSTEM_DP_IMAGE:-${ANIMUS_SYSTEM_IMAGE:-}}"

parse_image() {
  local image="$1"
  local repo tag digest
  repo="$image"
  tag="latest"
  digest=""
  if [[ "$image" == *@* ]]; then
    repo="${image%@*}"
    digest="${image#*@}"
  else
    local last="${image##*/}"
    if [[ "$last" == *:* ]]; then
      repo="${image%:*}"
      tag="${image##*:}"
    fi
  fi
  echo "$repo" "$tag" "$digest"
}

helm_image_args() {
  local image="$1"
  if [[ -z "$image" ]]; then
    echo ""
    return
  fi
  read -r repo tag digest <<<"$(parse_image "$image")"
  local args="--set image.repository=${repo} --set image.tag=${tag}"
  if [[ -n "$digest" ]]; then
    args="${args} --set image.digest=${digest}"
  fi
  echo "$args"
}

if ! kind get clusters | grep -qx "$CLUSTER_NAME"; then
  if [[ -n "${ANIMUS_KIND_IMAGE:-}" ]]; then
    kind create cluster --name "$CLUSTER_NAME" --image "${ANIMUS_KIND_IMAGE}"
  else
    kind create cluster --name "$CLUSTER_NAME"
  fi
fi

kubectl config use-context "kind-${CLUSTER_NAME}" >/dev/null
kubectl create namespace "$NAMESPACE" --dry-run=client -o yaml | kubectl apply -f - >/dev/null

if [[ "${ANIMUS_SYSTEM_LOAD_IMAGES:-}" == "1" ]]; then
  for img in "$DATAPILOT_IMAGE" "$DATAPLANE_IMAGE"; do
    if [[ -n "$img" ]]; then
      if docker image inspect "$img" >/dev/null 2>&1; then
        kind load docker-image "$img" --name "$CLUSTER_NAME"
      else
        echo "image not found locally: $img" >&2
        exit 1
      fi
    fi
  done
fi

DATAPILOT_ARGS="$(helm_image_args "$DATAPILOT_IMAGE")"
DATAPLANE_ARGS="$(helm_image_args "$DATAPLANE_IMAGE")"

# shellcheck disable=SC2086
helm upgrade --install "$DATAPILOT_RELEASE" "${DEPLOY_DIR}/helm/animus-datapilot" \
  --namespace "$NAMESPACE" \
  --create-namespace \
  -f "$ROOT_DIR/scripts/system_values.yaml" \
  --set auth.internalAuthSecret="$INTERNAL_AUTH_SECRET" \
  $DATAPILOT_ARGS

# shellcheck disable=SC2086
helm upgrade --install "$DATAPLANE_RELEASE" "${DEPLOY_DIR}/helm/animus-dataplane" \
  --namespace "$NAMESPACE" \
  --create-namespace \
  -f "$ROOT_DIR/scripts/system_dataplane_values.yaml" \
  --set auth.internalAuthSecret="$INTERNAL_AUTH_SECRET" \
  --set controlPlane.baseURL="http://${DATAPILOT_RELEASE}-gateway:8080" \
  $DATAPLANE_ARGS

if [[ "${ANIMUS_SYSTEM_SIEM_MOCK:-1}" == "1" ]]; then
  kubectl apply -n "$NAMESPACE" -f "$ROOT_DIR/scripts/system_siem_mock.yaml" >/dev/null
fi

if [[ "${ANIMUS_SYSTEM_VAULT_DEV:-}" == "1" ]]; then
  kubectl apply -n "$NAMESPACE" -f "$ROOT_DIR/scripts/system_vault_dev.yaml" >/dev/null
fi

kubectl set env deployment/"${DATAPILOT_RELEASE}"-experiments \
  ANIMUS_DATAPLANE_URL="http://${DATAPLANE_RELEASE}:8086" \
  ANIMUS_WEBHOOK_POLL_INTERVAL="${ANIMUS_SYSTEM_WEBHOOK_POLL_INTERVAL:-1s}" \
  ANIMUS_WEBHOOK_RETRY_BASE="${ANIMUS_SYSTEM_WEBHOOK_RETRY_BASE:-1s}" \
  ANIMUS_WEBHOOK_RETRY_MAX="${ANIMUS_SYSTEM_WEBHOOK_RETRY_MAX:-3s}" \
  ANIMUS_WEBHOOK_HTTP_TIMEOUT="${ANIMUS_SYSTEM_WEBHOOK_HTTP_TIMEOUT:-2s}" \
  ANIMUS_WEBHOOK_MAX_ATTEMPTS="${ANIMUS_SYSTEM_WEBHOOK_MAX_ATTEMPTS:-3}" \
  ANIMUS_DEVENV_TTL="${ANIMUS_SYSTEM_DEVENV_TTL:-30m}" \
  ANIMUS_DEVENV_ACCESS_TTL="${ANIMUS_SYSTEM_DEVENV_ACCESS_TTL:-5m}" \
  ANIMUS_DEVENV_ACCESS_AUDIT_INTERVAL="${ANIMUS_SYSTEM_DEVENV_ACCESS_AUDIT_INTERVAL:-30s}" \
  ANIMUS_DEVENV_CODE_SERVER_PORT="${ANIMUS_SYSTEM_DEVENV_CODE_SERVER_PORT:-80}" \
  ANIMUS_REGISTRY_POLICY_MODE="${ANIMUS_SYSTEM_REGISTRY_POLICY_MODE:-allow_unsigned}" \
  ANIMUS_REGISTRY_POLICY_PROVIDER="${ANIMUS_SYSTEM_REGISTRY_POLICY_PROVIDER:-noop}" >/dev/null

kubectl set env deployment/"${DATAPILOT_RELEASE}"-audit \
  AUDIT_EXPORT_DESTINATION="${ANIMUS_SYSTEM_AUDIT_DESTINATION:-webhook}" \
  AUDIT_EXPORT_WEBHOOK_URL="${ANIMUS_SYSTEM_AUDIT_WEBHOOK_URL:-http://siem-mock.${NAMESPACE}.svc.cluster.local:18081}" \
  AUDIT_EXPORT_SYSLOG_ADDR="${ANIMUS_SYSTEM_AUDIT_SYSLOG_ADDR:-siem-mock.${NAMESPACE}.svc.cluster.local:1514}" \
  AUDIT_EXPORT_SYSLOG_PROTOCOL="${ANIMUS_SYSTEM_AUDIT_SYSLOG_PROTOCOL:-tcp}" \
  AUDIT_EXPORT_POLL_INTERVAL="${ANIMUS_SYSTEM_AUDIT_POLL_INTERVAL:-1s}" \
  AUDIT_EXPORT_RETRY_BASE="${ANIMUS_SYSTEM_AUDIT_RETRY_BASE:-1s}" \
  AUDIT_EXPORT_RETRY_MAX="${ANIMUS_SYSTEM_AUDIT_RETRY_MAX:-3s}" \
  AUDIT_EXPORT_HTTP_TIMEOUT="${ANIMUS_SYSTEM_AUDIT_HTTP_TIMEOUT:-2s}" \
  AUDIT_EXPORT_MAX_ATTEMPTS="${ANIMUS_SYSTEM_AUDIT_MAX_ATTEMPTS:-2}" >/dev/null

kubectl set env deployment/"${DATAPLANE_RELEASE}" \
  ANIMUS_DP_EGRESS_MODE="${ANIMUS_SYSTEM_DP_EGRESS_MODE:-allow}" \
  ANIMUS_DATAPLANE_STATUS_POLL_INTERVAL="${ANIMUS_SYSTEM_DP_STATUS_POLL_INTERVAL:-1h}" \
  ANIMUS_DEVENV_CODE_SERVER_PORT="${ANIMUS_SYSTEM_DEVENV_CODE_SERVER_PORT:-80}" >/dev/null

if [[ -n "${ANIMUS_SYSTEM_DEVENV_GIT_IMAGE:-}" ]]; then
  kubectl set env deployment/"${DATAPLANE_RELEASE}" ANIMUS_DEVENV_GIT_IMAGE="${ANIMUS_SYSTEM_DEVENV_GIT_IMAGE}" >/dev/null
fi
if [[ -n "${ANIMUS_SYSTEM_DEVENV_CODE_SERVER_CMD:-}" ]]; then
  kubectl set env deployment/"${DATAPLANE_RELEASE}" ANIMUS_DEVENV_CODE_SERVER_CMD="${ANIMUS_SYSTEM_DEVENV_CODE_SERVER_CMD}" >/dev/null
fi

"$ROOT_DIR/scripts/system_wait.sh"

mkdir -p "$CACHE_DIR"

start_port_forward() {
  local name="$1"
  local target="$2"
  local port="$3"
  local pid_file="$4"
  local log_file="$5"
  if [[ -f "$pid_file" ]]; then
    if kill -0 "$(cat "$pid_file")" >/dev/null 2>&1; then
      return
    fi
  fi
  kubectl -n "$NAMESPACE" port-forward "$target" "$port" >"$log_file" 2>&1 &
  echo $! >"$pid_file"
}

start_port_forward "gateway" "svc/${DATAPILOT_RELEASE}-gateway" "${GATEWAY_PORT}:8080" \
  "$CACHE_DIR/system_gateway_pf.pid" "$CACHE_DIR/system_gateway_pf.log"
"$ROOT_DIR/scripts/wait_port.sh" 127.0.0.1 "$GATEWAY_PORT" 30 >/dev/null

start_port_forward "postgres" "svc/${DATAPILOT_RELEASE}-postgres" "${POSTGRES_PORT}:5432" \
  "$CACHE_DIR/system_postgres_pf.pid" "$CACHE_DIR/system_postgres_pf.log"
"$ROOT_DIR/scripts/wait_port.sh" 127.0.0.1 "$POSTGRES_PORT" 30 >/dev/null

cat >"$CACHE_DIR/system_env" <<ENVEOF
export ANIMUS_E2E_GATEWAY_URL="http://127.0.0.1:${GATEWAY_PORT}"
export ANIMUS_E2E_DATABASE_URL="postgres://animus:animus@127.0.0.1:${POSTGRES_PORT}/animus?sslmode=disable"
export ANIMUS_E2E_NAMESPACE="${NAMESPACE}"
export ANIMUS_E2E_SIEM_WEBHOOK_URL="http://siem-mock.${NAMESPACE}.svc.cluster.local:18080"
export ANIMUS_E2E_SIEM_SYSLOG_ADDR="siem-mock.${NAMESPACE}.svc.cluster.local:1514"
export ANIMUS_E2E_SIEM_SYSLOG_PROTOCOL="tcp"
ENVEOF

echo "system-up: gateway port-forward on :${GATEWAY_PORT}, env file: ${CACHE_DIR}/system_env"
