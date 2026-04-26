#!/usr/bin/env bash
set -euo pipefail

HOST="${1:-}"
PORT="${2:-}"
TIMEOUT="${3:-30}"

if [[ -z "$HOST" || -z "$PORT" ]]; then
  echo "usage: wait_port.sh <host> <port> [timeout_seconds]" >&2
  exit 1
fi

start=$(date +%s)
while true; do
  if (echo >"/dev/tcp/${HOST}/${PORT}") >/dev/null 2>&1; then
    echo "port ${HOST}:${PORT} is ready"
    exit 0
  fi
  now=$(date +%s)
  if (( now - start >= TIMEOUT )); then
    echo "timeout waiting for ${HOST}:${PORT}" >&2
    exit 1
  fi
  sleep 1
done
