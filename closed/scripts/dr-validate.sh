#!/usr/bin/env bash
set -euo pipefail

if [[ "${ANIMUS_DR_VALIDATE:-}" != "1" ]]; then
  echo "dr-validate: ANIMUS_DR_VALIDATE not set; skipping."
  exit 0
fi

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "missing required env: ${name}" >&2
    exit 1
  fi
}

require_tool() {
  local name="$1"
  if ! command -v "${name}" >/dev/null 2>&1; then
    echo "missing required tool: ${name}" >&2
    exit 1
  fi
}

require_tool curl
require_tool python3

require_env ANIMUS_GATEWAY_URL
require_env ANIMUS_DR_TOKEN
require_env ANIMUS_DR_PROJECT_ID

gateway="${ANIMUS_GATEWAY_URL%/}"
dataset_api="${gateway}/api/dataset-registry"
experiments_api="${gateway}/api/experiments"
audit_api="${gateway}/api/audit"

timestamp="$(date -u +%Y%m%dT%H%M%SZ)"
dataset_name="${ANIMUS_DR_DATASET_NAME:-dr-validate-${timestamp}}"
report_path="${ANIMUS_DR_REPORT_PATH:-/tmp/animus-dr-validate-${timestamp}.md}"

auth_header="Authorization: Bearer ${ANIMUS_DR_TOKEN}"
project_header="X-Project-Id: ${ANIMUS_DR_PROJECT_ID}"

start_time="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

check_url() {
  local url="$1"
  curl -fsS "${url}" >/dev/null
}

check_url "${gateway}/healthz"
check_url "${gateway}/readyz"
check_url "${experiments_api}/readyz"
check_url "${dataset_api}/readyz"
check_url "${audit_api}/readyz"

tmp_resp="$(mktemp)"
create_code="$(curl -sS -o "${tmp_resp}" -w "%{http_code}" \
  -H "${auth_header}" -H "${project_header}" -H "Content-Type: application/json" \
  -d "{\"name\":\"${dataset_name}\"}" \
  "${dataset_api}/datasets")"

dataset_id=""
if [[ "${create_code}" == "201" ]]; then
  dataset_id="$(python3 - <<'PY' "${tmp_resp}"
import json,sys
data=json.load(open(sys.argv[1]))
print(data.get("dataset_id",""))
PY
)"
elif [[ "${create_code}" == "409" ]]; then
  list_resp="$(mktemp)"
  list_code="$(curl -sS -o "${list_resp}" -w "%{http_code}" \
    -H "${auth_header}" -H "${project_header}" \
    "${dataset_api}/datasets?limit=200")"
  if [[ "${list_code}" != "200" ]]; then
    echo "dataset list failed (http ${list_code})" >&2
    exit 1
  fi
  dataset_id="$(python3 - <<'PY' "${list_resp}" "${dataset_name}"
import json,sys
data=json.load(open(sys.argv[1]))
name=sys.argv[2]
for item in data.get("datasets", []):
    if item.get("name") == name:
        print(item.get("dataset_id",""))
        sys.exit(0)
sys.exit(1)
PY
)" || {
    echo "dataset not found after conflict" >&2
    exit 1
  }
  rm -f "${list_resp}"
else
  echo "dataset create failed (http ${create_code})" >&2
  exit 1
fi
rm -f "${tmp_resp}"

tmp_file="$(mktemp)"
echo "dr-validate ${dataset_name}" > "${tmp_file}"
upload_resp="$(mktemp)"
upload_code="$(curl -sS -o "${upload_resp}" -w "%{http_code}" \
  -H "${auth_header}" -H "${project_header}" \
  -F "file=@${tmp_file}" \
  "${dataset_api}/datasets/${dataset_id}/versions/upload")"

version_id=""
if [[ "${upload_code}" == "201" ]]; then
  version_id="$(python3 - <<'PY' "${upload_resp}"
import json,sys
data=json.load(open(sys.argv[1]))
print(data.get("version_id",""))
PY
)"
elif [[ "${upload_code}" == "409" ]]; then
  versions_resp="$(mktemp)"
  versions_code="$(curl -sS -o "${versions_resp}" -w "%{http_code}" \
    -H "${auth_header}" -H "${project_header}" \
    "${dataset_api}/datasets/${dataset_id}/versions?limit=200")"
  if [[ "${versions_code}" != "200" ]]; then
    echo "dataset versions list failed (http ${versions_code})" >&2
    exit 1
  fi
  version_id="$(python3 - <<'PY' "${versions_resp}"
import json,sys
data=json.load(open(sys.argv[1]))
versions=data.get("versions", [])
if not versions:
    sys.exit(1)
versions.sort(key=lambda v: v.get("ordinal", 0), reverse=True)
print(versions[0].get("version_id",""))
PY
)" || {
    echo "dataset version not found after conflict" >&2
    exit 1
  }
  rm -f "${versions_resp}"
else
  echo "dataset upload failed (http ${upload_code})" >&2
  exit 1
fi

download_path="$(mktemp)"
curl -fsS -H "${auth_header}" -H "${project_header}" \
  "${dataset_api}/dataset-versions/${version_id}/download" \
  -o "${download_path}"
if [[ ! -s "${download_path}" ]]; then
  echo "downloaded dataset is empty" >&2
  exit 1
fi

end_time="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

{
  echo "# DR Validate Report"
  echo
  echo "- started_at: ${start_time}"
  echo "- finished_at: ${end_time}"
  echo "- gateway_url: ${gateway}"
  echo "- project_id: ${ANIMUS_DR_PROJECT_ID}"
  echo "- dataset_id: ${dataset_id}"
  echo "- dataset_version_id: ${version_id}"
  echo "- status: pass"
} > "${report_path}"

rm -f "${tmp_file}" "${upload_resp}" "${download_path}"

echo "dr-validate report: ${report_path}"
