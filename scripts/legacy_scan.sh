#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ALLOWLIST_FILE="${ANIMUS_LEGACY_SCAN_ALLOW_FILE:-${ROOT_DIR}/scripts/legacy_scan.allow}"
FAIL_MODE="${ANIMUS_LEGACY_SCAN_FAIL:-0}"
allowed_count=0

mapfile -t matches < <(rg -n --hidden '(open/api|closed/deploy|closed/scripts|open/sdk)' "${ROOT_DIR}" \
  --glob '!.git/*' \
  --glob '!open/api/README.md' \
  --glob '!open/sdk/README.md' \
  --glob '!closed/deploy/README.md' \
  --glob '!closed/scripts/README.md' \
  --glob '!closed/scripts/.stub-marker' \
  --glob '!scripts/compat_check.sh' \
  --glob '!scripts/submodule_check.sh' \
  --glob '!scripts/legacy_scan.sh' || true)

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
      echo "legacy-scan: allowlist entry missing metadata (owner=..., expiry=YYYY-MM-DD): ${file}:${line_no}" >&2
      fail=1
    fi
    metadata_seen=0
  done < "${file}"

  return "${fail}"
}

allow_patterns=()
if [[ -f "${ALLOWLIST_FILE}" ]]; then
  validate_allowlist_metadata "${ALLOWLIST_FILE}"
  while IFS= read -r pattern; do
    if [[ -z "${pattern}" || "${pattern}" =~ ^[[:space:]]*# ]]; then
      continue
    fi
    allow_patterns+=("${pattern}")
  done < "${ALLOWLIST_FILE}"
fi

filtered_matches=()
for match in "${matches[@]}"; do
  excluded=0
  for pattern in "${allow_patterns[@]}"; do
    if [[ "${match}" =~ ${pattern} ]]; then
      excluded=1
      allowed_count=$((allowed_count + 1))
      break
    fi
  done
  if [[ "${excluded}" -eq 0 ]]; then
    filtered_matches+=("${match}")
  fi
done

if [[ -f "${ALLOWLIST_FILE}" ]]; then
  echo "legacy-scan: allowlist=${ALLOWLIST_FILE}"
else
  echo "legacy-scan: allowlist not found: ${ALLOWLIST_FILE}"
fi
echo "legacy-scan: matched=${#matches[@]} allowed=${allowed_count} unallowed=${#filtered_matches[@]}"

if [[ "${#filtered_matches[@]}" -gt 0 ]]; then
  echo "legacy-scan: found legacy path references (non-blocking):"
  printf '%s\n' "${filtered_matches[@]}"
  if [[ "${FAIL_MODE}" == "1" ]]; then
    echo "legacy-scan: failing because ANIMUS_LEGACY_SCAN_FAIL=1" >&2
    exit 1
  fi
else
  echo "legacy-scan: no legacy path references found outside approved shims"
fi

exit 0
