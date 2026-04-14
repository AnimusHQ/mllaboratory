#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/lib/paths.sh"
CACHE_DIR="${ROOT_DIR}/.cache"
STARTUP_RETRIES="${ANIMUS_SYSTEM_STARTUP_RETRIES:-3}"
DEPLOY_DIR="$(animus_deploy_dir)"

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

export ANIMUS_SYSTEM_ENABLE=1

CLUSTER_NAME="${ANIMUS_KIND_CLUSTER_NAME:-animus-fullstack}"
NAMESPACE="${ANIMUS_SYSTEM_NAMESPACE:-animus-system}"
DATAPILOT_RELEASE="${ANIMUS_SYSTEM_DATAPILOT_RELEASE:-animus-datapilot}"
DATAPLANE_RELEASE="${ANIMUS_SYSTEM_DATAPLANE_RELEASE:-animus-dataplane}"
DATAPILOT_FULLNAME="${ANIMUS_SYSTEM_DATAPILOT_FULLNAME:-${DATAPILOT_RELEASE}-animus-datapilot}"
DATAPLANE_FULLNAME="${ANIMUS_SYSTEM_DATAPLANE_FULLNAME:-${DATAPLANE_RELEASE}-animus-dataplane}"
INTERNAL_AUTH_SECRET="${ANIMUS_SYSTEM_INTERNAL_AUTH_SECRET:-animus-internal-e2e-secret}"
GATEWAY_PORT="${ANIMUS_SYSTEM_GATEWAY_PORT:-8080}"
POSTGRES_PORT="${ANIMUS_SYSTEM_POSTGRES_PORT:-15432}"
CONSOLE_PORT="${ANIMUS_CONSOLE_PORT:-3001}"
UI_ENABLED="${ANIMUS_SYSTEM_UI_ENABLED:-0}"
CONSOLE_DEV="${ANIMUS_CONSOLE_DEV:-1}"
BUILD_IMAGES="${ANIMUS_SYSTEM_BUILD_IMAGES:-1}"
IMAGE_TAG="${ANIMUS_IMAGE_TAG:-}"
AUTH_MODE="${ANIMUS_AUTH_MODE:-oidc}"
TRAINING_EXECUTOR="${ANIMUS_SYSTEM_TRAINING_EXECUTOR:-disabled}"
TRAINING_NAMESPACE="${ANIMUS_SYSTEM_TRAINING_NAMESPACE:-${NAMESPACE}}"
TRAINING_JOB_TTL="${ANIMUS_SYSTEM_TRAINING_JOB_TTL_SECONDS:-3600}"
TRAINING_JOB_SA="${ANIMUS_SYSTEM_TRAINING_JOB_SERVICE_ACCOUNT:-}"

TRAINING_EXECUTOR_LOWER="$(echo "${TRAINING_EXECUTOR}" | tr '[:upper:]' '[:lower:]')"
if [[ "${TRAINING_EXECUTOR_LOWER}" != "" && "${TRAINING_EXECUTOR_LOWER}" != "disabled" ]]; then
  echo "system-prod-up: ANIMUS_SYSTEM_TRAINING_EXECUTOR must be disabled for production profile" >&2
  exit 1
fi

HOST_IP="${ANIMUS_HOST_IP:-}"
if [[ -z "$HOST_IP" ]]; then
  HOST_IP="$(hostname -I | awk "{print \$1}")"
fi

OIDC_ISSUER_URL="${ANIMUS_OIDC_ISSUER_URL:-http://${HOST_IP}:18080/realms/animus}"
OIDC_CLIENT_ID="${ANIMUS_OIDC_CLIENT_ID:-animus-gateway}"
OIDC_CLIENT_SECRET="${ANIMUS_OIDC_CLIENT_SECRET:-animus-oidc-local-secret}"
OIDC_REDIRECT_URL="${ANIMUS_OIDC_REDIRECT_URL:-http://${HOST_IP}:8080/auth/callback}"
PUBLIC_BASE_URL="${ANIMUS_PUBLIC_BASE_URL:-http://${HOST_IP}:8080}"
CONSOLE_UPSTREAM_URL="${ANIMUS_CONSOLE_UPSTREAM_URL:-http://${HOST_IP}:3001}"

mkdir -p "$CACHE_DIR"
VALUES_FILE="${CACHE_DIR}/system_prod_values.yaml"
CHART_DIR="${DEPLOY_DIR}/helm/animus-datapilot"
CHART_WORK_DIR="${CACHE_DIR}/animus-datapilot-chart"
MIGRATIONS_SRC="${ROOT_DIR}/closed/migrations"

if [[ -z "$IMAGE_TAG" ]]; then
  if command -v git >/dev/null 2>&1 && git -C "$ROOT_DIR" rev-parse --short HEAD >/dev/null 2>&1; then
    IMAGE_TAG="local-$(git -C "$ROOT_DIR" rev-parse --short HEAD)"
  else
    IMAGE_TAG="local-$(date +%Y%m%d%H%M%S)"
  fi
fi

if [[ "$BUILD_IMAGES" == "1" ]]; then
  require_bin make
  if [[ "$UI_ENABLED" == "1" ]]; then
    ANIMUS_BUILD_UI=1 ANIMUS_IMAGE_TAG="$IMAGE_TAG" make images-build
  else
    ANIMUS_BUILD_UI=0 ANIMUS_IMAGE_TAG="$IMAGE_TAG" make images-build
  fi
fi

sync_chart_migrations() {
  rm -rf "$CHART_WORK_DIR"
  cp -R "$CHART_DIR" "$CHART_WORK_DIR"
  mkdir -p "$CHART_WORK_DIR/migrations"
  rm -f "$CHART_WORK_DIR/migrations/"*.up.sql
  shopt -s nullglob
  local files=("${MIGRATIONS_SRC}"/*_*.up.sql)
  shopt -u nullglob
  if [[ "${#files[@]}" -eq 0 ]]; then
    echo "system-prod-up: no migrations found in ${MIGRATIONS_SRC}" >&2
    exit 1
  fi
  cp "${files[@]}" "$CHART_WORK_DIR/migrations/"
}

stop_pf() {
  local pid_file="$1"
  if [[ -f "$pid_file" ]]; then
    local pid
    pid="$(cat "$pid_file" || true)"
    if [[ -n "$pid" ]] && kill -0 "$pid" >/dev/null 2>&1; then
      if ps -p "$pid" -o comm= 2>/dev/null | grep -q kubectl; then
        kill "$pid" >/dev/null 2>&1 || true
      fi
    fi
    rm -f "$pid_file"
  fi
}

ensure_port_forward() {
  local target="$1"
  local port="$2"
  local pid_file="$3"
  local log_file="$4"
  local attempts=0
  while [[ "$attempts" -lt "$STARTUP_RETRIES" ]]; do
    attempts=$((attempts + 1))
    stop_pf "$pid_file"
    kubectl -n "$NAMESPACE" port-forward --address 0.0.0.0 "$target" "$port" >"$log_file" 2>&1 &
    echo $! >"$pid_file"
    if "$ROOT_DIR/scripts/wait_port.sh" 127.0.0.1 "$(echo "$port" | cut -d: -f1)" 30 >/dev/null; then
      return
    fi
  done
  echo "system-prod-up: port-forward failed for ${target} on ${port}" >&2
  exit 1
}

cat >"$VALUES_FILE" <<EOFVALUES
profile: production
auth:
  mode: "${AUTH_MODE}"
  sessionCookieSecure: false
image:
  tag: "${IMAGE_TAG}"
oidc:
  issuerURL: "${OIDC_ISSUER_URL}"
  clientID: "${OIDC_CLIENT_ID}"
  clientSecret: "${OIDC_CLIENT_SECRET}"
  redirectURL: "${OIDC_REDIRECT_URL}"
  scopes: "openid profile email"
  rolesClaim: "roles"
  emailClaim: "email"
  sessionCookieName: "animus_session"
  sessionMaxAgeSeconds: 3600
  sessionCookieSameSite: Lax
console:
  upstreamURL: "${CONSOLE_UPSTREAM_URL}"
  publicBaseURL: "${PUBLIC_BASE_URL}"
  allowedReturnToOrigins:
    - "${PUBLIC_BASE_URL}"
    - "${CONSOLE_UPSTREAM_URL}"
training:
  executor: "${TRAINING_EXECUTOR}"
  k8s:
    namespace: "${TRAINING_NAMESPACE}"
    jobTTLSeconds: ${TRAINING_JOB_TTL}
    jobServiceAccount: "${TRAINING_JOB_SA}"
ui:
  enabled: $( [[ "$UI_ENABLED" == "1" ]] && echo true || echo false )
postgres:
  persistence:
    enabled: false
minio:
  persistence:
    enabled: false
EOFVALUES

if [[ "${ANIMUS_SYSTEM_LOAD_IMAGES:-1}" == "1" ]]; then
  IMAGES=(
    "animus/gateway:${IMAGE_TAG}"
    "animus/experiments:${IMAGE_TAG}"
    "animus/dataset-registry:${IMAGE_TAG}"
    "animus/quality:${IMAGE_TAG}"
    "animus/lineage:${IMAGE_TAG}"
    "animus/audit:${IMAGE_TAG}"
    "animus/dataplane:${IMAGE_TAG}"
  )
  if [[ "$UI_ENABLED" == "1" ]]; then
    IMAGES+=("animus/ui:${IMAGE_TAG}")
  fi
  for img in "${IMAGES[@]}"; do
    if docker image inspect "$img" >/dev/null 2>&1; then
      kind load docker-image "$img" --name "$CLUSTER_NAME"
    else
      echo "image not found locally: $img" >&2
      echo "build it or set ANIMUS_SYSTEM_LOAD_IMAGES=0 to allow pull" >&2
      exit 1
    fi
  done
fi

sync_chart_migrations

helm upgrade --install "$DATAPILOT_RELEASE" "$CHART_WORK_DIR" \
  --namespace "$NAMESPACE" \
  --create-namespace \
  -f "$VALUES_FILE" \
  --set auth.internalAuthSecret="$INTERNAL_AUTH_SECRET"

helm upgrade --install "$DATAPLANE_RELEASE" "${DEPLOY_DIR}/helm/animus-dataplane" \
  --namespace "$NAMESPACE" \
  --create-namespace \
  --set auth.internalAuthSecret="$INTERNAL_AUTH_SECRET" \
  --set image.tag="$IMAGE_TAG" \
  --set controlPlane.baseURL="http://${DATAPILOT_FULLNAME}-gateway:8080"

kubectl -n "$NAMESPACE" set env deployment/"${DATAPILOT_FULLNAME}"-experiments \
  ANIMUS_DATAPLANE_URL="http://${DATAPLANE_FULLNAME}:8086" \
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

kubectl -n "$NAMESPACE" set env deployment/"${DATAPILOT_FULLNAME}"-audit \
  AUDIT_EXPORT_DESTINATION="${ANIMUS_SYSTEM_AUDIT_DESTINATION:-webhook}" \
  AUDIT_EXPORT_WEBHOOK_URL="${ANIMUS_SYSTEM_AUDIT_WEBHOOK_URL:-http://siem-mock.${NAMESPACE}.svc.cluster.local:18081}" \
  AUDIT_EXPORT_SYSLOG_ADDR="${ANIMUS_SYSTEM_AUDIT_SYSLOG_ADDR:-siem-mock.${NAMESPACE}.svc.cluster.local:1514}" \
  AUDIT_EXPORT_SYSLOG_PROTOCOL="${ANIMUS_SYSTEM_AUDIT_SYSLOG_PROTOCOL:-tcp}" \
  AUDIT_EXPORT_POLL_INTERVAL="${ANIMUS_SYSTEM_AUDIT_POLL_INTERVAL:-1s}" \
  AUDIT_EXPORT_RETRY_BASE="${ANIMUS_SYSTEM_AUDIT_RETRY_BASE:-1s}" \
  AUDIT_EXPORT_RETRY_MAX="${ANIMUS_SYSTEM_AUDIT_RETRY_MAX:-3s}" \
  AUDIT_EXPORT_HTTP_TIMEOUT="${ANIMUS_SYSTEM_AUDIT_HTTP_TIMEOUT:-2s}" \
  AUDIT_EXPORT_MAX_ATTEMPTS="${ANIMUS_SYSTEM_AUDIT_MAX_ATTEMPTS:-2}" >/dev/null

kubectl -n "$NAMESPACE" set env deployment/"${DATAPLANE_FULLNAME}" \
  ANIMUS_DP_EGRESS_MODE="${ANIMUS_SYSTEM_DP_EGRESS_MODE:-allow}" \
  ANIMUS_DATAPLANE_STATUS_POLL_INTERVAL="${ANIMUS_SYSTEM_DP_STATUS_POLL_INTERVAL:-1h}" \
  ANIMUS_DEVENV_CODE_SERVER_PORT="${ANIMUS_SYSTEM_DEVENV_CODE_SERVER_PORT:-80}" >/dev/null

"$ROOT_DIR/scripts/system_wait.sh"

ensure_port_forward "svc/${DATAPILOT_FULLNAME}-gateway" "${GATEWAY_PORT}:8080" \
  "$CACHE_DIR/system_gateway_pf.pid" "$CACHE_DIR/system_gateway_pf.log"

ensure_port_forward "svc/${DATAPILOT_FULLNAME}-postgres" "${POSTGRES_PORT}:5432" \
  "$CACHE_DIR/system_postgres_pf.pid" "$CACHE_DIR/system_postgres_pf.log"

if [[ "$UI_ENABLED" != "1" ]]; then
  kubectl -n "$NAMESPACE" delete deployment "${DATAPILOT_FULLNAME}-ui" --ignore-not-found >/dev/null 2>&1 || true
  kubectl -n "$NAMESPACE" delete service "${DATAPILOT_FULLNAME}-ui" --ignore-not-found >/dev/null 2>&1 || true
fi

start_console_dev() {
  if [[ "$CONSOLE_DEV" != "1" ]]; then
    return
  fi
  if [[ "$UI_ENABLED" == "1" ]]; then
    echo "console dev disabled (ui.enabled=1)" >&2
    return
  fi
  require_bin node
  require_bin npm
  if [[ ! -d "$ROOT_DIR/closed/ui/node_modules" ]]; then
    echo "console dev requires node_modules; run: (cd closed/ui && npm ci)" >&2
    exit 1
  fi
  local pid_file="$CACHE_DIR/console-dev.pid"
  local log_file="$CACHE_DIR/console-dev.log"
  if [[ -f "$pid_file" ]]; then
    if kill -0 "$(cat "$pid_file")" >/dev/null 2>&1; then
      return
    fi
  fi
  (cd "$ROOT_DIR/closed/ui" && \
    NEXT_PUBLIC_SITE_URL="${PUBLIC_BASE_URL}" \
    NEXT_PUBLIC_GATEWAY_URL="${PUBLIC_BASE_URL}" \
    nohup npm run dev >"$log_file" 2>&1 & echo $! >"$pid_file")
  "$ROOT_DIR/scripts/wait_port.sh" 127.0.0.1 "$CONSOLE_PORT" 60 >/dev/null
}

start_console_dev

cat >"$CACHE_DIR/system_prod_env" <<ENVEOF
export ANIMUS_HOST_IP="${HOST_IP}"
export ANIMUS_GATEWAY_URL="http://${HOST_IP}:${GATEWAY_PORT}"
export ANIMUS_PUBLIC_BASE_URL="${PUBLIC_BASE_URL}"
export ANIMUS_CONSOLE_UPSTREAM_URL="${CONSOLE_UPSTREAM_URL}"
export ANIMUS_IMAGE_TAG="${IMAGE_TAG}"
ENVEOF

echo "system-prod-up: gateway port-forward on :${GATEWAY_PORT}"
echo "system-prod-up: console upstream -> ${CONSOLE_UPSTREAM_URL}"
echo "system-prod-up: values file -> ${VALUES_FILE}"
echo "system-prod-up: env file -> ${CACHE_DIR}/system_prod_env"

if [[ "${ANIMUS_SYSTEM_HEALTH:-1}" == "1" ]]; then
  "$ROOT_DIR/scripts/system_prod_health.sh"
fi
