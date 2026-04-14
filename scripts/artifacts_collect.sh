#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARTIFACTS_ROOT="${ROOT_DIR}/artifacts"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ARTIFACTS_DIR="${ANIMUS_ARTIFACTS_DIR:-${ARTIFACTS_ROOT}/${TIMESTAMP}}"

mkdir -p "${ARTIFACTS_DIR}"

echo "artifacts-collect: ${ARTIFACTS_DIR}"

if [[ -f "${ROOT_DIR}/.cache/system_env" ]]; then
  cp -f "${ROOT_DIR}/.cache/system_env" "${ARTIFACTS_DIR}/system_env.sh"
  # shellcheck source=/dev/null
  source "${ROOT_DIR}/.cache/system_env"
fi

if [[ -d "${ROOT_DIR}/closed/ui/test-results" ]]; then
  mkdir -p "${ARTIFACTS_DIR}/ui"
  cp -R "${ROOT_DIR}/closed/ui/test-results" "${ARTIFACTS_DIR}/ui/"
fi
if [[ -d "${ROOT_DIR}/closed/ui/coverage" ]]; then
  mkdir -p "${ARTIFACTS_DIR}/ui"
  cp -R "${ROOT_DIR}/closed/ui/coverage" "${ARTIFACTS_DIR}/ui/"
fi

GATEWAY_URL="${ANIMUS_E2E_GATEWAY_URL:-}"
if [[ -n "$GATEWAY_URL" ]] && command -v curl >/dev/null 2>&1; then
  curl -fsS "${GATEWAY_URL}/readyz" -o "${ARTIFACTS_DIR}/gateway_readyz.json" || true
  curl -fsS "${GATEWAY_URL}/metrics" -o "${ARTIFACTS_DIR}/gateway_metrics.txt" || true
  curl -fsS "${GATEWAY_URL}/api/audit/events?limit=200" -o "${ARTIFACTS_DIR}/audit_events.json" || true
  if [[ -s "${ARTIFACTS_DIR}/audit_events.json" ]] && command -v python3 >/dev/null 2>&1; then
    python3 - <<'PY' "${ARTIFACTS_DIR}/audit_events.json" "${ARTIFACTS_DIR}/audit_events.ndjson" || true
import json
import sys
src = sys.argv[1]
dst = sys.argv[2]
with open(src, "r", encoding="utf-8") as fh:
    data = json.load(fh)
items = data.get("events", []) if isinstance(data, dict) else []
with open(dst, "w", encoding="utf-8") as out:
    for item in items:
        out.write(json.dumps(item, ensure_ascii=False))
        out.write("\n")
PY
  fi
fi

if [[ -n "$GATEWAY_URL" ]] && [[ -f "${ARTIFACTS_DIR}/e2e_ids.json" ]] && command -v python3 >/dev/null 2>&1; then
  read -r PROJECT_ID RUN_ID <<<"$(python3 - <<'PY' "${ARTIFACTS_DIR}/e2e_ids.json" || true
import json
import sys
with open(sys.argv[1], "r", encoding="utf-8") as fh:
    data = json.load(fh)
print(data.get("project_id", ""), data.get("run_id", ""))
PY
)"
  if [[ -n "$PROJECT_ID" && -n "$RUN_ID" ]] && command -v curl >/dev/null 2>&1; then
    curl -fsS -H "X-Project-Id: ${PROJECT_ID}" \
      "${GATEWAY_URL}/api/experiments/projects/${PROJECT_ID}/runs/${RUN_ID}/reproducibility-bundle" \
      -o "${ARTIFACTS_DIR}/reproducibility_bundle.json" || true
    if [[ -s "${ARTIFACTS_DIR}/reproducibility_bundle.json" ]] && command -v sha256sum >/dev/null 2>&1; then
      sha256sum "${ARTIFACTS_DIR}/reproducibility_bundle.json" > "${ARTIFACTS_DIR}/reproducibility_bundle.sha256" || true
    fi
  fi
fi

if command -v kubectl >/dev/null 2>&1; then
  NAMESPACE="${ANIMUS_E2E_NAMESPACE:-${ANIMUS_SYSTEM_NAMESPACE:-animus-system}}"
  if kubectl get namespace "${NAMESPACE}" >/dev/null 2>&1; then
    K8S_DIR="${ARTIFACTS_DIR}/k8s"
    mkdir -p "${K8S_DIR}/logs"
    kubectl -n "${NAMESPACE}" get pods -o wide > "${K8S_DIR}/pods.txt" || true
    kubectl -n "${NAMESPACE}" get deployments -o wide > "${K8S_DIR}/deployments.txt" || true
    kubectl -n "${NAMESPACE}" get svc -o wide > "${K8S_DIR}/services.txt" || true
    kubectl -n "${NAMESPACE}" get events --sort-by=.metadata.creationTimestamp > "${K8S_DIR}/events.txt" || true

    DATAPILOT_RELEASE="${ANIMUS_SYSTEM_DATAPILOT_RELEASE:-animus-datapilot}"
    DATAPLANE_RELEASE="${ANIMUS_SYSTEM_DATAPLANE_RELEASE:-animus-dataplane}"
    for deploy in \
      "${DATAPILOT_RELEASE}-gateway" \
      "${DATAPILOT_RELEASE}-experiments" \
      "${DATAPILOT_RELEASE}-audit" \
      "${DATAPLANE_RELEASE}" \
      "siem-mock" \
      "vault-dev"; do
      if kubectl -n "${NAMESPACE}" get deployment "${deploy}" >/dev/null 2>&1; then
        kubectl -n "${NAMESPACE}" logs "deployment/${deploy}" --all-containers --since=2h > "${K8S_DIR}/logs/${deploy}.log" 2>&1 || true
      fi
    done
  fi
fi
