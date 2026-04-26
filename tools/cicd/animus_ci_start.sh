#!/usr/bin/env bash
set -euo pipefail

script_name="$(basename "$0")"

fail() {
  echo "${script_name}: $*" >&2
  exit 1
}

require_cmd() {
  command -v "$1" >/dev/null 2>&1 || fail "missing required command: $1"
}

require_env() {
  local name="$1"
  if [ -z "${!name:-}" ]; then
    fail "missing required env var: ${name}"
  fi
}

urlencode() {
  python3 -c 'import sys,urllib.parse; print(urllib.parse.quote(sys.argv[1], safe=""))' "$1"
}

api_request() {
  local method="$1"
  local url="$2"
  local data="${3:-}"
  local tmp_body
  local code

  tmp_body="$(mktemp)"
  if [ "$method" = "GET" ]; then
    code="$(curl -sS -o "$tmp_body" -w "%{http_code}" -H "$AUTH_HEADER" "$url")"
  else
    code="$(curl -sS -o "$tmp_body" -w "%{http_code}" -H "$AUTH_HEADER" -H "Content-Type: application/json" -X "$method" -d "$data" "$url")"
  fi

  if [ "$code" -lt 200 ] || [ "$code" -ge 300 ]; then
    echo "request failed ($code) for $method $url" >&2
    cat "$tmp_body" >&2
    rm -f "$tmp_body"
    exit 1
  fi

  cat "$tmp_body"
  rm -f "$tmp_body"
}

api_get_allow_404() {
  local url="$1"
  local tmp_body
  local code

  tmp_body="$(mktemp)"
  code="$(curl -sS -o "$tmp_body" -w "%{http_code}" -H "$AUTH_HEADER" "$url")"
  if [ "$code" = "404" ]; then
    rm -f "$tmp_body"
    return 1
  fi
  if [ "$code" -lt 200 ] || [ "$code" -ge 300 ]; then
    echo "request failed ($code) for GET $url" >&2
    cat "$tmp_body" >&2
    rm -f "$tmp_body"
    exit 1
  fi
  cat "$tmp_body"
  rm -f "$tmp_body"
}

require_cmd curl
require_cmd python3

ANIMUS_BASE_URL="${ANIMUS_BASE_URL:-${ANIMUS_GATEWAY_URL:-}}"
ANIMUS_RUN_TOKEN="${ANIMUS_RUN_TOKEN:-${ANIMUS_AUTH_TOKEN:-}}"

require_env ANIMUS_BASE_URL
require_env ANIMUS_RUN_TOKEN
require_env ANIMUS_PROJECT
require_env ANIMUS_DATASET_REF
require_env GIT_COMMIT_SHA
require_env IMAGE_DIGEST

if [ "${ANIMUS_DRY_RUN:-}" = "1" ]; then
  run_id="run_dry_$(date +%s)"
  mkdir -p .animus
  printf 'RUN_ID=%s\n' "$run_id" > .animus/run.env
  echo "dry run: wrote .animus/run.env"
  exit 0
fi

base_url="${ANIMUS_BASE_URL%/}"
AUTH_HEADER="Authorization: Bearer ${ANIMUS_RUN_TOKEN}"

image_ref="${ANIMUS_IMAGE_REF:-}"
if [ -z "$image_ref" ]; then
  image_repo="${ANIMUS_IMAGE_REPO:-}"
  if [ -n "$image_repo" ]; then
    image_ref="${image_repo}@${IMAGE_DIGEST}"
  else
    encoded_digest="$(urlencode "$IMAGE_DIGEST")"
    model_resp="$(api_get_allow_404 "${base_url}/api/experiments/model-images/${encoded_digest}" || true)"
    if [ -n "$model_resp" ]; then
      image_repo="$(printf '%s' "$model_resp" | python3 - <<'PY'
import json,sys
data = json.load(sys.stdin)
print(data.get("repo", ""))
PY
)"
      if [ -n "$image_repo" ]; then
        image_ref="${image_repo}@${IMAGE_DIGEST}"
      fi
    fi
  fi
fi

if [ -z "$image_ref" ]; then
  fail "unable to resolve image ref; set ANIMUS_IMAGE_REF or ANIMUS_IMAGE_REPO (or register the digest via /ci/report)"
fi

project_name="${ANIMUS_PROJECT}"
encoded_project="$(urlencode "$project_name")"
experiment_resp="$(api_request "GET" "${base_url}/api/experiments/experiments?limit=1&name=${encoded_project}")"
experiment_id="$(printf '%s' "$experiment_resp" | python3 - <<'PY'
import json,sys
data = json.load(sys.stdin)
experiments = data.get("experiments") or []
print(experiments[0].get("experiment_id", "") if experiments else "")
PY
)"

if [ -z "$experiment_id" ]; then
  create_payload="$(python3 - <<'PY'
import json,os
payload = {
    "name": os.environ["ANIMUS_PROJECT"],
    "description": "CI integration run",
    "metadata": {"source": "ci"},
}
print(json.dumps(payload))
PY
)"
  created="$(api_request "POST" "${base_url}/api/experiments/experiments" "$create_payload")"
  experiment_id="$(printf '%s' "$created" | python3 - <<'PY'
import json,sys
data = json.load(sys.stdin)
print(data.get("experiment_id", ""))
PY
)"
fi

if [ -z "$experiment_id" ]; then
  fail "unable to resolve experiment id for ANIMUS_PROJECT=${ANIMUS_PROJECT}"
fi

git_repo="${GIT_REPO_URL:-${CI_PROJECT_URL:-${GIT_URL:-}}}"
git_ref="${GIT_REF:-${CI_COMMIT_REF_NAME:-${GIT_BRANCH:-}}}"

payload="$(python3 - <<'PY'
import json,os
payload = {
    "experiment_id": os.environ["EXPERIMENT_ID"],
    "dataset_version_id": os.environ["ANIMUS_DATASET_REF"],
    "image_ref": os.environ["IMAGE_REF"],
    "git_commit": os.environ["GIT_COMMIT_SHA"],
}
repo = os.environ.get("GIT_REPO", "").strip()
ref = os.environ.get("GIT_REF", "").strip()
if repo:
    payload["git_repo"] = repo
if ref:
    payload["git_ref"] = ref
print(json.dumps(payload))
PY
)"

resp="$(EXPERIMENT_ID="$experiment_id" IMAGE_REF="$image_ref" GIT_REPO="$git_repo" GIT_REF="$git_ref" \
  api_request "POST" "${base_url}/api/experiments/experiments/runs:execute" "$payload")"

run_id="$(printf '%s' "$resp" | python3 - <<'PY'
import json,sys
data = json.load(sys.stdin)
print(data.get("run_id", ""))
PY
)"

if [ -z "$run_id" ]; then
  fail "run_id missing from response"
fi

mkdir -p .animus
printf 'RUN_ID=%s\n' "$run_id" > .animus/run.env
echo "run registered: ${run_id}"
