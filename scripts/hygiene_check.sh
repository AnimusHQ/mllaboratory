#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ALLOWLIST_FILE="${ANIMUS_HYGIENE_ALLOW_FILE:-${ROOT_DIR}/scripts/hygiene.allow}"
INCLUDE_IGNORED="${ANIMUS_HYGIENE_CHECK_INCLUDE_IGNORED:-0}"

# Generated/junk paths that must not be tracked or appear as untracked files.
JUNK_PATTERN='(^artifacts/|^\.cache/|^\.bin/|(^|/)dist-test/|(^|/)node_modules/|(^|/)\.next/|(^|/)__pycache__/|\.pyc$|(^|/)build/|(^|/)dist/)'

allowed_count=0
allow_patterns=()

validate_allowlist_metadata() {
  local file="$1"
  local line
  local line_no=0
  local metadata_seen=0
  local fail=0

  while IFS= read -r line || [[ -n "${line}" ]]; do
    line_no=$((line_no + 1))
    if [[ -z "${line}" || "${line}" =~ ^[[:space:]]*$ ]]; then
      continue
    fi
    if [[ "${line}" =~ ^[[:space:]]*# ]]; then
      if [[ "${line}" =~ owner= && "${line}" =~ expiry= ]]; then
        metadata_seen=1
      fi
      continue
    fi
    if [[ "${metadata_seen}" -ne 1 ]]; then
      echo "hygiene-check: allowlist entry missing metadata (owner=..., expiry=YYYY-MM-DD): ${file}:${line_no}" >&2
      fail=1
    fi
    metadata_seen=0
  done < "${file}"

  return "${fail}"
}

if [[ -f "${ALLOWLIST_FILE}" ]]; then
  validate_allowlist_metadata "${ALLOWLIST_FILE}"
  while IFS= read -r pattern; do
    if [[ -z "${pattern}" || "${pattern}" =~ ^[[:space:]]*# ]]; then
      continue
    fi
    allow_patterns+=("${pattern}")
  done < "${ALLOWLIST_FILE}"
fi

filter_allowlisted() {
  local -n in_ref=$1
  local -n out_ref=$2
  local item
  local pattern
  local excluded
  out_ref=()
  for item in "${in_ref[@]}"; do
    excluded=0
    for pattern in "${allow_patterns[@]}"; do
      if [[ "${item}" =~ ${pattern} ]]; then
        excluded=1
        allowed_count=$((allowed_count + 1))
        break
      fi
    done
    if [[ "${excluded}" -eq 0 ]]; then
      out_ref+=("${item}")
    fi
  done
}

mapfile -t tracked_matches < <(git -C "${ROOT_DIR}" ls-files | rg "${JUNK_PATTERN}" || true)
mapfile -t untracked_matches < <(git -C "${ROOT_DIR}" ls-files --others --exclude-standard | rg "${JUNK_PATTERN}" || true)

if [[ "${INCLUDE_IGNORED}" == "1" ]]; then
  mapfile -t ignored_matches < <(git -C "${ROOT_DIR}" ls-files --others -i --exclude-standard | rg "${JUNK_PATTERN}" || true)
  if [[ "${#ignored_matches[@]}" -gt 0 ]]; then
    untracked_matches+=("${ignored_matches[@]}")
  fi
fi

if [[ "${#untracked_matches[@]}" -gt 0 ]]; then
  mapfile -t untracked_matches < <(printf '%s\n' "${untracked_matches[@]}" | awk 'NF' | sort -u)
fi

filter_allowlisted tracked_matches tracked_filtered
filter_allowlisted untracked_matches untracked_filtered

if [[ -f "${ALLOWLIST_FILE}" ]]; then
  echo "hygiene-check: allowlist=${ALLOWLIST_FILE}"
else
  echo "hygiene-check: allowlist not found: ${ALLOWLIST_FILE}"
fi
echo "hygiene-check: tracked=${#tracked_matches[@]} untracked=${#untracked_matches[@]} allowed=${allowed_count}"

if [[ "${#tracked_filtered[@]}" -gt 0 ]]; then
  echo "hygiene-check: tracked generated/junk paths detected:" >&2
  printf '%s\n' "${tracked_filtered[@]}" >&2
  echo "hygiene-check: remove with git rm or move behind reviewed allowlist entry." >&2
  exit 1
fi

if [[ "${#untracked_filtered[@]}" -gt 0 ]]; then
  echo "hygiene-check: untracked generated/junk paths detected:" >&2
  printf '%s\n' "${untracked_filtered[@]}" >&2
  echo "hygiene-check: remove files or add a narrow, temporary allowlist entry." >&2
  exit 1
fi

echo "hygiene-check: ok"
