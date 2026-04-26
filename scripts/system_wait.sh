#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

if [[ "${ANIMUS_SYSTEM_ENABLE:-}" != "1" ]]; then
  echo "system-wait: ANIMUS_SYSTEM_ENABLE not set; skipping."
  exit 0
fi

require_bin() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required tool: $1" >&2
    exit 1
  fi
}

require_bin kubectl

NAMESPACE="${ANIMUS_SYSTEM_NAMESPACE:-animus-system}"
DATAPILOT_RELEASE="${ANIMUS_SYSTEM_DATAPILOT_RELEASE:-animus-datapilot}"
DATAPLANE_RELEASE="${ANIMUS_SYSTEM_DATAPLANE_RELEASE:-animus-dataplane}"
TIMEOUT="${ANIMUS_SYSTEM_WAIT_TIMEOUT:-180s}"

wait_for() {
  local kind="$1"
  local name="$2"
  if kubectl -n "$NAMESPACE" get "$kind" "$name" >/dev/null 2>&1; then
    kubectl -n "$NAMESPACE" wait --for=condition=available "$kind"/"$name" --timeout="$TIMEOUT" >/dev/null
  fi
}

wait_for_statefulset() {
  local name="$1"
  if kubectl -n "$NAMESPACE" get statefulset "$name" >/dev/null 2>&1; then
    kubectl -n "$NAMESPACE" rollout status statefulset "$name" --timeout="$TIMEOUT" >/dev/null
  fi
}

wait_for_job() {
  local name="$1"
  if kubectl -n "$NAMESPACE" get job "$name" >/dev/null 2>&1; then
    kubectl -n "$NAMESPACE" wait --for=condition=complete job "$name" --timeout="$TIMEOUT" >/dev/null
  fi
}

wait_for_job "${DATAPILOT_RELEASE}-migrate"
wait_for_statefulset "${DATAPILOT_RELEASE}-postgres"
wait_for_statefulset "${DATAPILOT_RELEASE}-minio"

for svc in gateway dataset-registry quality experiments lineage audit; do
  wait_for deployment "${DATAPILOT_RELEASE}-${svc}"
  done

wait_for deployment "${DATAPLANE_RELEASE}"

echo "system-wait: all components ready"
