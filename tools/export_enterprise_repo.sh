#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SRC_DIR="${ROOT_DIR}/enterprise"
OUT_DIR="/tmp/animus-enterprise-export"

usage() {
  cat <<'USAGE' >&2
usage: tools/export_enterprise_repo.sh [--src <enterprise-dir>] [--out <output-dir>]

Creates a standalone git repository containing enterprise content.
It does not push to any remote.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --src)
      SRC_DIR="${2:-}"
      shift 2
      ;;
    --out)
      OUT_DIR="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown arg: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [[ ! -d "${SRC_DIR}" ]]; then
  echo "export-enterprise: source directory not found: ${SRC_DIR}" >&2
  exit 1
fi

if [[ ! -d "${SRC_DIR}/scripts" ]]; then
  echo "export-enterprise: source must contain scripts/: ${SRC_DIR}" >&2
  exit 1
fi

if [[ -e "${OUT_DIR}" ]] && [[ -n "$(find "${OUT_DIR}" -mindepth 1 -maxdepth 1 2>/dev/null)" ]]; then
  echo "export-enterprise: output dir is not empty: ${OUT_DIR}" >&2
  echo "export-enterprise: choose another --out path or clean it first." >&2
  exit 1
fi

rm -rf "${OUT_DIR}"
mkdir -p "${OUT_DIR}"
cp -a "${SRC_DIR}/." "${OUT_DIR}/"

(
  cd "${OUT_DIR}"
  git init -q
  git config user.name "Animus Export Bot"
  git config user.email "animus-export-bot@example.com"
  git add .
  git commit -q -m "chore: initial enterprise repo export from open-core tree"
)

cat <<EOF
export-enterprise: created repository at ${OUT_DIR}

Next steps:
  1) cd ${OUT_DIR}
  2) git remote add origin <PRIVATE_ENTERPRISE_REPO_URL>
  3) git branch -M main
  4) git push -u origin main
EOF
