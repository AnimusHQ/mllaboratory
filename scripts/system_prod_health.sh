#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CACHE_DIR="${ROOT_DIR}/.cache"

require_bin() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "missing required tool: $1" >&2
    exit 1
  fi
}

require_bin kubectl
require_bin curl

NAMESPACE="${ANIMUS_SYSTEM_NAMESPACE:-animus-system}"
GATEWAY_PORT="${ANIMUS_SYSTEM_GATEWAY_PORT:-8080}"
HOST_IP="${ANIMUS_HOST_IP:-}"
if [[ -z "$HOST_IP" ]]; then
  HOST_IP="$(hostname -I | awk '{print $1}')"
fi

PUBLIC_BASE_URL="${ANIMUS_PUBLIC_BASE_URL:-http://${HOST_IP}:${GATEWAY_PORT}}"
CONSOLE_UPSTREAM_URL="${ANIMUS_CONSOLE_UPSTREAM_URL:-http://${HOST_IP}:3001}"

fail() {
  echo "system-prod-health: $1" >&2
  exit 1
}

check_code() {
  local url="$1"
  local expected="$2"
  local code
  code="$(curl -s -o /dev/null -w '%{http_code}' "$url" || true)"
  if [[ "$code" != "$expected" ]]; then
    echo "health-check: expected ${expected}, got ${code} for ${url}" >&2
    return 1
  fi
  return 0
}

echo "health-check: gateway ${PUBLIC_BASE_URL}"
if ! check_code "${PUBLIC_BASE_URL}/" "200"; then
  echo "diagnostic: port-forward or gateway not reachable" >&2
  ss -ltnp | grep ":${GATEWAY_PORT}" || true
  kubectl -n "$NAMESPACE" get pods || true
  exit 1
fi

me_tmp="$(mktemp)"
me_code="$(curl -s -o "$me_tmp" -w '%{http_code}' "${PUBLIC_BASE_URL}/api/auth/me" || true)"
if [[ "$me_code" == "200" ]]; then
  if ! grep -q '"user_id"' "$me_tmp"; then
    if grep -q '"service":"gateway"' "$me_tmp"; then
      echo "health-check: /api/auth/me unexpected response (gateway image likely outdated)" >&2
    else
      echo "health-check: /api/auth/me unexpected payload" >&2
    fi
    cat "$me_tmp" >&2 || true
    rm -f "$me_tmp"
    exit 1
  fi
elif [[ "$me_code" != "401" ]]; then
  echo "health-check: /api/auth/me returned ${me_code} (expected 401 or 200)" >&2
  cat "$me_tmp" >&2 || true
  rm -f "$me_tmp"
  exit 1
fi
rm -f "$me_tmp"

console_code="$(curl -s -o /dev/null -w '%{http_code}' "${PUBLIC_BASE_URL}/console" || true)"
if [[ "$console_code" != "302" ]]; then
  echo "health-check: /console returned ${console_code} (expected 302)" >&2
  body="$(curl -s "${PUBLIC_BASE_URL}/console" || true)"
  if echo "$body" | grep -q '"service":"gateway"'; then
    echo "diagnostic: gateway console proxy not deployed; rebuild and redeploy gateway" >&2
  fi
  kubectl -n "$NAMESPACE" get pods || true
  exit 1
fi

echo "health-check: console upstream ${CONSOLE_UPSTREAM_URL}"
upstream_code="$(curl -s -o /dev/null -w '%{http_code}' "${CONSOLE_UPSTREAM_URL}" || true)"
if [[ "$upstream_code" != "200" && "$upstream_code" != "307" && "$upstream_code" != "308" ]]; then
  echo "health-check: console upstream returned ${upstream_code}" >&2
  if [[ -f "${CACHE_DIR}/console-dev.log" ]]; then
    tail -n 40 "${CACHE_DIR}/console-dev.log" || true
  fi
  exit 1
fi

echo "health-check: ok"
