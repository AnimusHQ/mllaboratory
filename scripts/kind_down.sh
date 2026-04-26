#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CACHE_DIR="${ROOT_DIR}/.cache"

if [[ "${ANIMUS_SYSTEM_ENABLE:-}" != "1" ]]; then
  echo "system-down: ANIMUS_SYSTEM_ENABLE not set; skipping."
  exit 0
fi

CLUSTER_NAME="${ANIMUS_KIND_CLUSTER_NAME:-animus-fullstack}"

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

rm -f "$CACHE_DIR/system_env"

if kind get clusters | grep -qx "$CLUSTER_NAME"; then
  if [[ "${ANIMUS_SYSTEM_PRESERVE:-}" == "1" ]]; then
    echo "system-down: preserving kind cluster ${CLUSTER_NAME}"
  else
    kind delete cluster --name "$CLUSTER_NAME"
  fi
fi
