#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CACHE_ROOT="${CACHE_ROOT:-}"

if [[ -z "${CACHE_ROOT}" ]]; then
  if mkdir -p "${ROOT_DIR}/.cache" 2>/dev/null; then
    CACHE_ROOT="${ROOT_DIR}/.cache"
  else
    CACHE_ROOT="${TMPDIR:-/tmp}/animus-go-cache"
  fi
fi

export GOCACHE="${CACHE_ROOT}/go-build"
export GOMODCACHE="${CACHE_ROOT}/go-mod"
export GOTMPDIR="${CACHE_ROOT}/go-tmp"

mkdir -p "${GOCACHE}" "${GOMODCACHE}" "${GOTMPDIR}"
