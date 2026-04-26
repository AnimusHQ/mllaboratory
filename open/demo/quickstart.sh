#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
COMPOSE_FILE="${ROOT_DIR}/open/demo/docker-compose.yml"
COMPOSE_PROJECT_NAME="${ANIMUS_DEMO_COMPOSE_PROJECT:-animus-demo}"
GATEWAY_PORT="${ANIMUS_GATEWAY_PORT:-8080}"
USERSPACE_PORT="${ANIMUS_USERSPACE_PORT:-8090}"
HEALTH_TIMEOUT_SECONDS="${HEALTH_TIMEOUT_SECONDS:-30}"
HEALTH_POLL_SECONDS="${HEALTH_POLL_SECONDS:-1}"
LOG_TAIL_LINES="${LOG_TAIL_LINES:-200}"
DEMO_MODE="full"
SKIP_CLEANUP=0
GOLDEN_MODE=0
TRANSCRIPT_FILE=""
TRANSCRIPT_TMP=""
GOLDEN_FAILED=0

export ANIMUS_GATEWAY_URL="${ANIMUS_GATEWAY_URL:-http://localhost:${GATEWAY_PORT}}"
export ANIMUS_USERSPACE_URL="${ANIMUS_USERSPACE_URL:-http://localhost:${USERSPACE_PORT}}"
export ANIMUS_DEV_SKIP_UI="${ANIMUS_DEV_SKIP_UI:-1}"
export AUTH_MODE="${AUTH_MODE:-dev}"
export AUTH_SESSION_COOKIE_SECURE="${AUTH_SESSION_COOKIE_SECURE:-false}"
export ANIMUS_INTERNAL_AUTH_SECRET="${ANIMUS_INTERNAL_AUTH_SECRET:-animus-demo-internal-secret}"
export ANIMUS_MINIO_ACCESS_KEY="${ANIMUS_MINIO_ACCESS_KEY:-animus}"
export ANIMUS_MINIO_SECRET_KEY="${ANIMUS_MINIO_SECRET_KEY:-animusminio}"
export ANIMUS_MINIO_BUCKET_DATASETS="${ANIMUS_MINIO_BUCKET_DATASETS:-datasets}"
export ANIMUS_MINIO_BUCKET_ARTIFACTS="${ANIMUS_MINIO_BUCKET_ARTIFACTS:-artifacts}"
export DATABASE_URL="${DATABASE_URL:-postgres://animus:animus@postgres:5432/animus?sslmode=disable}"

COMPOSE_CMD=()

usage() {
  cat <<USAGE
Usage: $0 [--smoke|--down]

Options:
  --smoke  Run smoke check mode (health checks only).
  --down   Tear down demo compose stack.
USAGE
}

require_http_tooling() {
  if ! command -v curl >/dev/null 2>&1 && ! command -v python3 >/dev/null 2>&1; then
    echo "curl (preferred) or python3 is required" >&2
    exit 1
  fi
}

require_docker() {
  if ! command -v docker >/dev/null 2>&1; then
    echo "docker is required for the full demo" >&2
    exit 1
  fi
}

enable_golden_mode() {
  if [ "${GOLDEN_MODE}" = "1" ]; then
    return
  fi
  if [ "${DEMO_GOLDEN:-0}" != "1" ]; then
    return
  fi
  GOLDEN_MODE=1
  local transcript_dir="${ROOT_DIR}/open/demo/golden"
  if [ "${DEMO_MODE}" = "smoke" ] && [ "${DEMO_NO_DOCKER:-0}" = "1" ]; then
    TRANSCRIPT_FILE="${transcript_dir}/demo-smoke-transcript.txt"
  else
    TRANSCRIPT_FILE="${transcript_dir}/demo-transcript.txt"
  fi
  TRANSCRIPT_TMP="${TRANSCRIPT_FILE}.tmp"
  mkdir -p "${transcript_dir}"
  : > "${TRANSCRIPT_TMP}"
  exec > >(tee "${TRANSCRIPT_TMP}")
}

finalize_golden() {
  if [ "${GOLDEN_MODE}" = "1" ] && [ -n "${TRANSCRIPT_TMP}" ]; then
    mv -f "${TRANSCRIPT_TMP}" "${TRANSCRIPT_FILE}"
  fi
}

mark_golden_failed() {
  if [ "${GOLDEN_MODE}" = "1" ]; then
    GOLDEN_FAILED=1
  fi
}

banner() {
  if [ "${GOLDEN_MODE}" = "1" ]; then
    echo "=== $1 ==="
  fi
}

display_gateway_url() {
  echo "${ANIMUS_GATEWAY_URL}"
}

display_userspace_url() {
  echo "${ANIMUS_USERSPACE_URL}"
}

detect_compose_cmd() {
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    COMPOSE_CMD=(docker compose)
    return 0
  fi
  if command -v docker-compose >/dev/null 2>&1; then
    COMPOSE_CMD=(docker-compose)
    return 0
  fi
  return 1
}

compose() {
  "${COMPOSE_CMD[@]}" -f "${COMPOSE_FILE}" --project-name "${COMPOSE_PROJECT_NAME}" "$@"
}

http_get_status() {
  local url="$1"
  if command -v curl >/dev/null 2>&1; then
    curl -s -o /dev/null -w "%{http_code}" "${url}" || true
    return 0
  fi
  python3 - <<PY || true
import sys
import urllib.request
try:
    with urllib.request.urlopen("${url}") as resp:
        sys.stdout.write(str(resp.status))
except Exception:
    sys.stdout.write("000")
PY
}

wait_for_health() {
  local url="$1"
  local timeout_s="$2"
  local poll_s="$3"
  local deadline=$((SECONDS + timeout_s))
  local status=""
  while [ "${SECONDS}" -lt "${deadline}" ]; do
    status="$(http_get_status "${url}")"
    if [ "${status}" = "200" ]; then
      return 0
    fi
    sleep "${poll_s}"
  done
  return 1
}

headers_json() {
  local json="{"
  local first=1
  local header
  for header in "$@"; do
    local key="${header%%:*}"
    local value="${header#*: }"
    if [ "${first}" -eq 0 ]; then
      json+=","
    fi
    json+="\"${key}\":\"${value}\""
    first=0
  done
  json+="}"
  echo "${json}"
}

http_request() {
  local method="$1"
  local url="$2"
  local body="${3:-}"
  local content_type="${4:-}"
  shift 4
  local headers=("$@")
  local resp status payload

  if command -v curl >/dev/null 2>&1; then
    local args=(-sS -X "${method}" -w "\n%{http_code}")
    if [ -n "${content_type}" ]; then
      args+=( -H "Content-Type: ${content_type}" )
    fi
    local header
    for header in "${headers[@]}"; do
      args+=( -H "${header}" )
    done
    if [ -n "${body}" ]; then
      args+=( -d "${body}" )
    fi
    resp=$(curl "${args[@]}" "${url}")
    status="$(printf "%s" "${resp}" | tail -n1)"
    payload="$(printf "%s" "${resp}" | sed '$d')"
  else
    resp=$(METHOD="${method}" URL="${url}" BODY="${body}" CONTENT_TYPE="${content_type}" HEADERS_JSON="$(headers_json "${headers[@]}")" \
      python3 - <<'PY'
import json
import os
import sys
import urllib.request

method = os.environ.get("METHOD", "GET")
url = os.environ.get("URL", "")
body = os.environ.get("BODY", "")
headers = json.loads(os.environ.get("HEADERS_JSON", "{}"))
content_type = os.environ.get("CONTENT_TYPE", "")
if content_type:
    headers["Content-Type"] = content_type

data = body.encode() if body else None
req = urllib.request.Request(url, data=data, headers=headers, method=method)
try:
    resp = urllib.request.urlopen(req)
    status = resp.status
    payload = resp.read()
except urllib.error.HTTPError as exc:
    status = exc.code
    payload = exc.read()
print(status)
sys.stdout.buffer.write(payload)
PY
    )
    status="$(printf "%s" "${resp}" | head -n1)"
    payload="$(printf "%s" "${resp}" | tail -n +2)"
  fi

  if [ "${status}" -lt 200 ] || [ "${status}" -ge 300 ]; then
    echo "request failed: ${method} ${url} status=${status}" >&2
    if [ -n "${payload}" ]; then
      echo "response: ${payload}" >&2
    fi
    return 1
  fi
  printf "%s" "${payload}"
}

http_post_multipart() {
  local url="$1"
  local file_path="$2"
  local metadata="$3"
  shift 3
  local headers=("$@")

  if command -v curl >/dev/null 2>&1; then
    local args=(-sS -X POST -w "\n%{http_code}")
    local header
    for header in "${headers[@]}"; do
      args+=( -H "${header}" )
    done
    args+=( -F "file=@${file_path}" )
    args+=( -F "metadata=${metadata}" )
    local resp status payload
    resp=$(curl "${args[@]}" "${url}")
    status="$(printf "%s" "${resp}" | tail -n1)"
    payload="$(printf "%s" "${resp}" | sed '$d')"
    if [ "${status}" -lt 200 ] || [ "${status}" -ge 300 ]; then
      echo "request failed: POST ${url} status=${status}" >&2
      if [ -n "${payload}" ]; then
        echo "response: ${payload}" >&2
      fi
      return 1
    fi
    printf "%s" "${payload}"
    return 0
  fi

  python3 - <<PY
import json
import os
import sys
import urllib.request
import uuid

url = "${url}"
file_path = "${file_path}"
metadata = "${metadata}"
headers = json.loads('''${headers_json "${headers[@]}"}''')

boundary = uuid.uuid4().hex
parts = []

def add_field(name, value, filename=None, content_type=None):
    disposition = f'form-data; name="{name}"'
    if filename:
        disposition += f'; filename="{filename}"'
    parts.append(f'--{boundary}'.encode())
    parts.append(f'Content-Disposition: {disposition}'.encode())
    if content_type:
        parts.append(f'Content-Type: {content_type}'.encode())
    parts.append(b'')
    parts.append(value)

add_field("metadata", metadata.encode(), None, None)
with open(file_path, "rb") as f:
    data = f.read()
add_field("file", data, os.path.basename(file_path), "text/csv")
parts.append(f'--{boundary}--'.encode())
body = b"\r\n".join(parts) + b"\r\n"

headers["Content-Type"] = f"multipart/form-data; boundary={boundary}"
req = urllib.request.Request(url, data=body, headers=headers, method="POST")
try:
    resp = urllib.request.urlopen(req)
    status = resp.status
    payload = resp.read()
except urllib.error.HTTPError as exc:
    status = exc.code
    payload = exc.read()

if status < 200 or status >= 300:
    sys.stderr.write(f"request failed: POST {url} status={status}\n")
    sys.stderr.write(payload.decode(errors="ignore") + "\n")
    sys.exit(1)

sys.stdout.buffer.write(payload)
PY
}

http_put_file() {
  local url="$1"
  local file_path="$2"
  local content_type="$3"
  if command -v curl >/dev/null 2>&1; then
    curl -sS -X PUT -H "Content-Type: ${content_type}" --data-binary "@${file_path}" "${url}" >/dev/null
    return 0
  fi
  python3 - <<PY
import sys
import urllib.request

url = "${url}"
file_path = "${file_path}"
content_type = "${content_type}"
with open(file_path, "rb") as f:
    data = f.read()
req = urllib.request.Request(url, data=data, headers={"Content-Type": content_type}, method="PUT")
try:
    resp = urllib.request.urlopen(req)
    if resp.status < 200 or resp.status >= 300:
        sys.exit(1)
except Exception:
    sys.exit(1)
PY
}

json_get() {
  local body="$1"
  local key="$2"
  if command -v python3 >/dev/null 2>&1; then
    printf "%s" "${body}" | python3 - <<PY
import json
import sys
try:
    data = json.load(sys.stdin)
    value = data.get("${key}")
    if value is None:
        sys.exit(1)
    if isinstance(value, (dict, list)):
        print(json.dumps(value))
    else:
        print(value)
except Exception:
    sys.exit(1)
PY
    return 0
  fi
  printf "%s" "${body}" | tr -d '\n' | sed -n "s/.*\"${key}\"[[:space:]]*:[[:space:]]*\"\([^\"]*\)\".*/\1/p"
}

sha256_file() {
  local file_path="$1"
  if command -v python3 >/dev/null 2>&1; then
    python3 - <<PY
import hashlib
with open("${file_path}", "rb") as f:
    h = hashlib.sha256()
    while True:
        chunk = f.read(8192)
        if not chunk:
            break
        h.update(chunk)
print(h.hexdigest())
PY
    return 0
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "${file_path}" | awk '{print $1}'
    return 0
  fi
  if command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "${file_path}" | awk '{print $1}'
    return 0
  fi
  echo "sha256 helper missing" >&2
  return 1
}

tail_logs() {
  local svc
  for svc in "$@"; do
    echo "==> logs: ${svc}" >&2
    compose logs --no-color --tail "${LOG_TAIL_LINES}" "${svc}" >&2 || true
  done
}

cleanup() {
  local code=$?
  if [ "${code}" -ne 0 ]; then
    mark_golden_failed
  fi
  if [ "${SKIP_CLEANUP}" = "1" ]; then
    if [ "${GOLDEN_FAILED}" = "1" ]; then
      echo "golden_transcript_not_finalized=true"
    fi
    exit "${code}"
  fi
  if [ "${#COMPOSE_CMD[@]}" -gt 0 ]; then
    if [ "${code}" -ne 0 ]; then
      tail_logs postgres minio dataset-registry experiments audit gateway userspace-runner || true
    fi
    compose down -v >/dev/null 2>&1 || true
  fi
  if [ "${GOLDEN_FAILED}" = "1" ]; then
    echo "golden_transcript_not_finalized=true"
  fi
  exit "${code}"
}
trap cleanup EXIT INT TERM

if [ "${1:-}" = "--smoke" ]; then
  DEMO_MODE="smoke"
  shift
elif [ "${1:-}" = "--down" ]; then
  DEMO_MODE="down"
  shift
elif [ -n "${1:-}" ]; then
  usage
  exit 2
fi

enable_golden_mode

if [ "${DEMO_MODE}" = "down" ]; then
  SKIP_CLEANUP=1
  if detect_compose_cmd; then
    if ! compose ps >/dev/null 2>&1; then
      echo "Docker compose not found; nothing to tear down."
      exit 0
    fi
    compose down -v
    exit $?
  fi
  echo "Docker compose not found; nothing to tear down."
  exit 0
fi

enable_golden_mode
require_http_tooling

if [ "${DEMO_NO_DOCKER:-0}" = "1" ]; then
  if [ "${DEMO_MODE}" != "smoke" ]; then
    echo "Full demo requires Docker. Use --smoke with DEMO_NO_DOCKER=1 or install Docker." >&2
    mark_golden_failed
    exit 1
  fi
  base_url="${DEMO_BASE_URL:-}"
  if [ -z "${base_url}" ] && [ "${GOLDEN_MODE}" != "1" ]; then
    base_url="${ANIMUS_GATEWAY_URL}"
  fi
  if [ -z "${base_url}" ]; then
    echo "DEMO_BASE_URL is required for no-docker smoke mode" >&2
    mark_golden_failed
    exit 1
  fi
  echo "==> smoke check (no docker)"
  if ! wait_for_health "${base_url}/api/experiments/healthz" "${HEALTH_TIMEOUT_SECONDS}" "${HEALTH_POLL_SECONDS}"; then
    echo "experiments health check failed" >&2
    mark_golden_failed
    exit 1
  fi
  if ! wait_for_health "${base_url}/api/dataset-registry/healthz" "${HEALTH_TIMEOUT_SECONDS}" "${HEALTH_POLL_SECONDS}"; then
    echo "dataset-registry health check failed" >&2
    mark_golden_failed
    exit 1
  fi
  if ! wait_for_health "${base_url}/api/audit/healthz" "${HEALTH_TIMEOUT_SECONDS}" "${HEALTH_POLL_SECONDS}"; then
    echo "audit health check failed" >&2
    mark_golden_failed
    exit 1
  fi
  echo "==> smoke check ok"
  finalize_golden
  exit 0
fi

require_docker
if ! detect_compose_cmd; then
  cat <<MSG >&2
Docker Compose is required for the full demo.
Install Docker Desktop (includes 'docker compose') or the legacy 'docker-compose' CLI.
If you only need a health-check smoke test, use:
  DEMO_NO_DOCKER=1 make demo-smoke DEMO_BASE_URL=http://localhost:8080
MSG
  mark_golden_failed
  exit 1
fi

echo "==> starting demo stack"
compose up -d --build

echo "==> waiting for gateway $(display_gateway_url)/healthz"
if ! wait_for_health "${ANIMUS_GATEWAY_URL}/healthz" "${HEALTH_TIMEOUT_SECONDS}" "${HEALTH_POLL_SECONDS}"; then
  echo "gateway did not become healthy within ${HEALTH_TIMEOUT_SECONDS}s" >&2
  mark_golden_failed
  exit 1
fi

echo "==> waiting for userspace $(display_userspace_url)/healthz"
if ! wait_for_health "${ANIMUS_USERSPACE_URL}/healthz" "${HEALTH_TIMEOUT_SECONDS}" "${HEALTH_POLL_SECONDS}"; then
  echo "userspace runner did not become healthy within ${HEALTH_TIMEOUT_SECONDS}s" >&2
  mark_golden_failed
  exit 1
fi

echo "==> migrations are managed by service startup in demo compose"

REQUEST_ID="${ANIMUS_DEMO_REQUEST_ID:-demo-$(date -u +%Y%m%dT%H%M%SZ)}"
NAME_SUFFIX="${ANIMUS_DEMO_SUFFIX:-$(date -u +%Y%m%d%H%M%S)}"
API_BASE="${ANIMUS_GATEWAY_URL}/api"

if [ "${DEMO_MODE}" = "smoke" ]; then
  banner "Create project"
  echo "==> create project"
  project_body=$(http_request "POST" "${API_BASE}/dataset-registry/projects" \
    "{\"name\":\"demo-project-${NAME_SUFFIX}\",\"description\":\"Animus smoke demo\",\"metadata\":{\"source\":\"demo\"}}" \
    "application/json" "X-Request-Id: ${REQUEST_ID}")
  PROJECT_ID="$(json_get "${project_body}" "project_id")"
  if [ -z "${PROJECT_ID}" ]; then
    echo "project_id missing" >&2
    mark_golden_failed
    exit 1
  fi
  if [ "${GOLDEN_MODE}" = "1" ]; then
    echo "project_id=${PROJECT_ID}"
  fi
  echo "==> smoke check ok"
  finalize_golden
  exit 0
fi

echo "==> manual steps"
echo "  1) POST /api/dataset-registry/projects"
echo "  2) POST /api/dataset-registry/datasets (X-Project-Id)"
echo "  3) POST /api/dataset-registry/datasets/{id}/versions/upload"
echo "  4) POST /api/dataset-registry/projects/{project_id}/artifacts"
echo "  5) POST /api/experiments/projects/{project_id}/runs"
echo "  6) POST /api/experiments/projects/{project_id}/runs/{run_id}:plan"
echo "  7) POST /api/experiments/projects/{project_id}/runs/{run_id}:dry-run"
echo "  8) GET  /api/experiments/projects/{project_id}/runs/{run_id}"
echo "  9) POST /api/audit/export"

banner "Auth session"
echo "==> auth session"
if [ "$(http_get_status "${ANIMUS_GATEWAY_URL}/auth/session")" = "200" ]; then
  http_request "GET" "${ANIMUS_GATEWAY_URL}/auth/session" "" "" "X-Request-Id: ${REQUEST_ID}" >/dev/null
else
  echo "==> auth session not configured (dev stub in use)"
fi

banner "Create project"
echo "==> create project"
project_body=$(http_request "POST" "${API_BASE}/dataset-registry/projects" \
  "{\"name\":\"demo-project-${NAME_SUFFIX}\",\"description\":\"Animus full-surface demo\",\"metadata\":{\"source\":\"demo\"}}" \
  "application/json" "X-Request-Id: ${REQUEST_ID}")
PROJECT_ID="$(json_get "${project_body}" "project_id")"
if [ -z "${PROJECT_ID}" ]; then
  echo "project_id missing" >&2
  mark_golden_failed
  exit 1
fi
if [ "${GOLDEN_MODE}" = "1" ]; then
  echo "project_id=${PROJECT_ID}"
fi

PROJECT_HEADER="X-Project-Id: ${PROJECT_ID}"

banner "Create dataset"
echo "==> create dataset"
dataset_body=$(http_request "POST" "${API_BASE}/dataset-registry/datasets" \
  "{\"name\":\"demo-dataset-${NAME_SUFFIX}\",\"description\":\"Deterministic demo dataset\",\"metadata\":{\"source\":\"demo\"}}" \
  "application/json" "X-Request-Id: ${REQUEST_ID}" "${PROJECT_HEADER}")
DATASET_ID="$(json_get "${dataset_body}" "dataset_id")"

if [ -z "${DATASET_ID}" ]; then
  echo "dataset_id missing" >&2
  mark_golden_failed
  exit 1
fi
if [ "${GOLDEN_MODE}" = "1" ]; then
  echo "dataset_id=${DATASET_ID}"
fi

banner "Upload dataset version"
echo "==> upload dataset version"
DATASET_FILE="${ROOT_DIR}/open/demo/data/demo.csv"
version_body=$(http_post_multipart "${API_BASE}/dataset-registry/datasets/${DATASET_ID}/versions/upload" \
  "${DATASET_FILE}" "{\"source\":\"demo\"}" "X-Request-Id: ${REQUEST_ID}" "${PROJECT_HEADER}")
DATASET_VERSION_ID="$(json_get "${version_body}" "dataset_version_id")"

if [ -z "${DATASET_VERSION_ID}" ]; then
  echo "dataset_version_id missing" >&2
  mark_golden_failed
  exit 1
fi
if [ "${GOLDEN_MODE}" = "1" ]; then
  echo "dataset_version_id=${DATASET_VERSION_ID}"
fi

banner "Create artifact"
echo "==> create artifact (presigned upload)"
ARTIFACT_FILE="$(mktemp -t animus-demo-artifact.XXXXXX.txt)"
printf "animus demo artifact %s\n" "${REQUEST_ID}" > "${ARTIFACT_FILE}"
ARTIFACT_SHA256="$(sha256_file "${ARTIFACT_FILE}")"
ARTIFACT_SIZE="$(wc -c < "${ARTIFACT_FILE}" | tr -d ' ')"
artifact_body=$(http_request "POST" "${API_BASE}/dataset-registry/projects/${PROJECT_ID}/artifacts" \
  "{\"kind\":\"demo\",\"content_type\":\"text/plain\",\"size_bytes\":${ARTIFACT_SIZE},\"sha256\":\"${ARTIFACT_SHA256}\",\"metadata\":{\"run\":\"demo\"}}" \
  "application/json" "X-Request-Id: ${REQUEST_ID}" "${PROJECT_HEADER}")
UPLOAD_URL="$(json_get "${artifact_body}" "upload_url")"
if [ -z "${UPLOAD_URL}" ]; then
  echo "upload_url missing" >&2
  mark_golden_failed
  exit 1
fi
http_put_file "${UPLOAD_URL}" "${ARTIFACT_FILE}" "text/plain"

banner "Create environment definition"
echo "==> create environment definition"
env_def_body=$(http_request "POST" "${API_BASE}/experiments/projects/${PROJECT_ID}/environment-definitions" \
  "{\"name\":\"demo-env-${NAME_SUFFIX}\",\"description\":\"Deterministic demo environment\",\"baseImages\":[{\"name\":\"runtime\",\"ref\":\"ghcr.io/animus/demo-runtime:1.0.0\"}]}" \
  "application/json" "X-Request-Id: ${REQUEST_ID}" "${PROJECT_HEADER}")
ENVIRONMENT_DEFINITION_ID="$(printf "%s" "${env_def_body}" | python3 -c 'import json,sys; data=json.load(sys.stdin); print(data.get("definition", {}).get("environmentDefinitionId", ""))' 2>/dev/null || true)"
if [ -z "${ENVIRONMENT_DEFINITION_ID}" ]; then
  ENVIRONMENT_DEFINITION_ID="$(printf "%s" "${env_def_body}" | tr -d '\n' | sed -n 's/.*"environmentDefinitionId"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
fi
if [ -z "${ENVIRONMENT_DEFINITION_ID}" ]; then
  echo "environmentDefinitionId missing" >&2
  mark_golden_failed
  exit 1
fi
if [ "${GOLDEN_MODE}" = "1" ]; then
  echo "environment_definition_id=${ENVIRONMENT_DEFINITION_ID}"
fi

banner "Create environment lock"
echo "==> create environment lock"
env_lock_body=$(http_request "POST" "${API_BASE}/experiments/projects/${PROJECT_ID}/environment-locks" \
  "{\"environmentDefinitionId\":\"${ENVIRONMENT_DEFINITION_ID}\",\"imageDigests\":{\"runtime\":\"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb\"}}" \
  "application/json" "X-Request-Id: ${REQUEST_ID}" "${PROJECT_HEADER}")
ENV_LOCK_ID="$(printf "%s" "${env_lock_body}" | python3 -c 'import json,sys; data=json.load(sys.stdin); print(data.get("lock", {}).get("lockId", ""))' 2>/dev/null || true)"
if [ -z "${ENV_LOCK_ID}" ]; then
  ENV_LOCK_ID="$(printf "%s" "${env_lock_body}" | tr -d '\n' | sed -n 's/.*"lockId"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')"
fi
if [ -z "${ENV_LOCK_ID}" ]; then
  echo "lockId missing" >&2
  mark_golden_failed
  exit 1
fi
if [ "${GOLDEN_MODE}" = "1" ]; then
  echo "environment_lock_id=${ENV_LOCK_ID}"
fi

PIPELINE_SPEC=$(cat <<'JSON'
{"apiVersion":"animus/v1alpha1","kind":"Pipeline","specVersion":"1.0","spec":{"steps":[{"name":"prepare","image":"ghcr.io/animus/demo@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","command":["/bin/true"],"args":[],"inputs":{"datasets":[],"artifacts":[]},"outputs":{"artifacts":[{"name":"prepared","type":"dataset"}]},"env":[],"resources":{"cpu":"1","memory":"512Mi","gpu":0},"retryPolicy":{"maxAttempts":1,"backoff":{"type":"fixed","initialSeconds":0,"maxSeconds":0,"multiplier":1}}},{"name":"train","image":"ghcr.io/animus/demo@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb","command":["/bin/true"],"args":[],"inputs":{"datasets":[{"name":"training","datasetRef":"training_data"}],"artifacts":[{"name":"prepared","fromStep":"prepare","artifact":"prepared"}]},"outputs":{"artifacts":[{"name":"model","type":"model"}]},"env":[{"name":"SEED","value":"1337"}],"resources":{"cpu":"2","memory":"1Gi","gpu":0},"retryPolicy":{"maxAttempts":2,"backoff":{"type":"fixed","initialSeconds":1,"maxSeconds":1,"multiplier":1}}}],"dependencies":[{"from":"prepare","to":"train"}]}}
JSON
)

RUN_REQUEST=$(cat <<JSON
{"idempotencyKey":"${REQUEST_ID}","pipelineSpec":${PIPELINE_SPEC},"datasetBindings":{"training_data":"${DATASET_VERSION_ID}"},"codeRef":{"repoUrl":"https://example.local/animus/demo.git","commitSha":"0123456789abcdef0123456789abcdef01234567"},"envLock":{"lockId":"${ENV_LOCK_ID}"}}
JSON
)

banner "Create run"
echo "==> create run"
run_body=$(http_request "POST" "${API_BASE}/experiments/projects/${PROJECT_ID}/runs" "${RUN_REQUEST}" "application/json" "X-Request-Id: ${REQUEST_ID}" "${PROJECT_HEADER}")
RUN_ID="$(json_get "${run_body}" "runId")"
SPEC_HASH="$(json_get "${run_body}" "specHash")"

if [ -z "${RUN_ID}" ] || [ -z "${SPEC_HASH}" ]; then
  echo "runId/specHash missing" >&2
  mark_golden_failed
  exit 1
fi
if [ "${GOLDEN_MODE}" = "1" ]; then
  echo "run_id=${RUN_ID}"
  echo "spec_hash=${SPEC_HASH}"
fi

banner "Plan"
echo "==> plan run"
http_request "POST" "${API_BASE}/experiments/projects/${PROJECT_ID}/runs/${RUN_ID}:plan" "{}" "application/json" "X-Request-Id: ${REQUEST_ID}" "${PROJECT_HEADER}" >/dev/null

banner "Dry-run"
echo "==> dry-run"
dry_body=$(http_request "POST" "${API_BASE}/experiments/projects/${PROJECT_ID}/runs/${RUN_ID}:dry-run" "{}" "application/json" "X-Request-Id: ${REQUEST_ID}" "${PROJECT_HEADER}")
DRY_STATE="$(json_get "${dry_body}" "state")"
if [ "${GOLDEN_MODE}" = "1" ]; then
  echo "dry_run_state=${DRY_STATE}"
fi

banner "Derived state"
echo "==> derived state"
get_body=$(http_request "GET" "${API_BASE}/experiments/projects/${PROJECT_ID}/runs/${RUN_ID}" "" "" "X-Request-Id: ${REQUEST_ID}" "${PROJECT_HEADER}")
DERIVED_STATE="$(json_get "${get_body}" "state")"

if [ "${GOLDEN_MODE}" = "1" ]; then
  echo "derived_state=${DERIVED_STATE}"
else
  printf "==> run state: %s (dry-run=%s)\n" "${DERIVED_STATE}" "${DRY_STATE}"
fi

banner "Userspace execution"
echo "==> userspace execution (data plane surface)"
userspace_body=$(http_request "POST" "${ANIMUS_USERSPACE_URL}/execute-demo-step" \
  "{\"run_id\":\"${RUN_ID}\",\"step_name\":\"train\",\"attempt\":1,\"seed\":\"${SPEC_HASH}:${RUN_ID}\",\"artifact_bucket\":\"${ANIMUS_MINIO_BUCKET_ARTIFACTS}\",\"artifact_prefix\":\"demo/${RUN_ID}\"}" \
  "application/json" "X-Request-Id: ${REQUEST_ID}")
USERSPACE_STATUS="$(json_get "${userspace_body}" "status")"

if [ "${GOLDEN_MODE}" = "1" ]; then
  echo "userspace_status=${USERSPACE_STATUS}"
else
  printf "==> userspace status: %s\n" "${USERSPACE_STATUS}"
fi

banner "Audit export"
echo "==> audit export (first 3 lines)"
audit_body=$(http_request "POST" "${API_BASE}/audit/export" "{\"project_id\":\"${PROJECT_ID}\"}" "application/json" "X-Request-Id: ${REQUEST_ID}" "${PROJECT_HEADER}")
printf "%s\n" "${audit_body}" | head -n 3

echo "==> demo complete"
finalize_golden
exit 0
