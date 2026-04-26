#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CACHE_DIR="${ROOT_DIR}/.cache"

export ANIMUS_SYSTEM_ENABLE=1

NAMESPACE="${ANIMUS_SYSTEM_NAMESPACE:-animus-system}"
DATAPILOT_RELEASE="${ANIMUS_SYSTEM_DATAPILOT_RELEASE:-animus-datapilot}"
DATAPLANE_RELEASE="${ANIMUS_SYSTEM_DATAPLANE_RELEASE:-animus-dataplane}"

stop_pf() {
  local pid_file="$1"
  if [[ -f "$pid_file" ]]; then
    local pid
    pid="$(cat "$pid_file")"
    if kill -0 "$pid" >/dev/null 2>&1; then
      kill "$pid" >/dev/null 2>&1 || true
    fi
    rm -f "$pid_file"
  fi
}

stop_pf "$CACHE_DIR/system_gateway_pf.pid"
stop_pf "$CACHE_DIR/system_postgres_pf.pid"
stop_pf "$CACHE_DIR/console-dev.pid"

if [[ "${ANIMUS_SYSTEM_PRESERVE:-}" == "1" ]]; then
  echo "system-prod-down: preserving releases (set ANIMUS_SYSTEM_PRESERVE=0 to uninstall)"
  exit 0
fi

helm uninstall "$DATAPILOT_RELEASE" -n "$NAMESPACE" || true
helm uninstall "$DATAPLANE_RELEASE" -n "$NAMESPACE" || true

if [[ "${ANIMUS_SYSTEM_DELETE_NAMESPACE:-}" == "1" ]]; then
  kubectl delete namespace "$NAMESPACE" || true
fi

echo "system-prod-down: releases removed"
