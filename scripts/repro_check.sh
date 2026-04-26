#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/lib/paths.sh"

DEPLOY_DIR="$(animus_deploy_dir)"
CANONICAL_UI_DIR="${CANONICAL_UI_DIR:-}"
if [[ -z "${CANONICAL_UI_DIR}" ]]; then
  CANONICAL_UI_DIR="$(awk -F'\?=' '/^CANONICAL_UI_DIR/ {gsub(/[[:space:]]/,"",$2); print $2; exit}' "${ROOT_DIR}/Makefile")"
fi
if [[ -z "${CANONICAL_UI_DIR}" ]]; then
  CANONICAL_UI_DIR="closed/ui"
fi

PROD_CP_VALUES="${ANIMUS_PROD_VALUES_CP:-${DEPLOY_DIR}/helm/animus-datapilot/values-production.yaml}"
PROD_DP_VALUES="${ANIMUS_PROD_VALUES_DP:-${DEPLOY_DIR}/helm/animus-dataplane/values-production.yaml}"
IMAGES_JSON="${ANIMUS_IMAGES_JSON:-${ROOT_DIR}/artifacts/images.json}"
REQUIRE_MANIFEST_MATCH="${ANIMUS_REPRO_REQUIRE_MANIFEST_MATCH:-0}"

fail=0

lockfile_found=0
for lock in package-lock.json npm-shrinkwrap.json pnpm-lock.yaml yarn.lock; do
  if [[ -f "${ROOT_DIR}/${CANONICAL_UI_DIR}/${lock}" ]]; then
    lockfile_found=1
    break
  fi
done
if [[ "${lockfile_found}" -ne 1 ]]; then
  echo "repro-check: missing lockfile in canonical UI dir: ${CANONICAL_UI_DIR}" >&2
  fail=1
fi

prod_values=("${PROD_CP_VALUES}" "${PROD_DP_VALUES}")
for values_file in "${prod_values[@]}"; do
  if [[ ! -f "${values_file}" ]]; then
    echo "repro-check: missing production values file: ${values_file}" >&2
    fail=1
    continue
  fi
  if rg -n '^[[:space:]]*tag:[[:space:]]*"?latest"?[[:space:]]*$' "${values_file}" >/dev/null 2>&1; then
    echo "repro-check: mutable image tag 'latest' is forbidden in ${values_file}" >&2
    fail=1
  fi
  if rg -n '^[[:space:]]*tag:[[:space:]]*"?[A-Za-z0-9_.-]+"?[[:space:]]*$' "${values_file}" >/dev/null 2>&1; then
    echo "repro-check: tag-only image references are forbidden in production values: ${values_file}" >&2
    fail=1
  fi
  if ! rg -n 'sha256:[0-9a-f]{64}' "${values_file}" >/dev/null 2>&1; then
    echo "repro-check: ${values_file} must include digest pins (sha256:...)" >&2
    fail=1
  fi
done

for svc in gateway experiments dataset-registry quality lineage audit; do
  if ! rg -n "^[[:space:]]+${svc}:[[:space:]]+sha256:[0-9a-f]{64}[[:space:]]*$" "${PROD_CP_VALUES}" >/dev/null 2>&1; then
    echo "repro-check: datapilot production values missing digest for service ${svc}" >&2
    fail=1
  fi
done

if ! rg -n '^[[:space:]]*digest:[[:space:]]+sha256:[0-9a-f]{64}[[:space:]]*$' "${PROD_DP_VALUES}" >/dev/null 2>&1; then
  echo "repro-check: dataplane production values missing image.digest" >&2
  fail=1
fi

if [[ "${REQUIRE_MANIFEST_MATCH}" == "1" ]]; then
  if [[ ! -f "${IMAGES_JSON}" ]]; then
    echo "repro-check: images manifest required but missing: ${IMAGES_JSON}" >&2
    fail=1
  else
    mapfile -t manifest_digests < <(rg --no-filename -o 'sha256:[0-9a-f]{64}' "${IMAGES_JSON}" | sort -u)
    mapfile -t prod_digests < <(rg --no-filename -o 'sha256:[0-9a-f]{64}' "${PROD_CP_VALUES}" "${PROD_DP_VALUES}" | sort -u)

    for digest in "${prod_digests[@]}"; do
      found=0
      for md in "${manifest_digests[@]}"; do
        if [[ "${digest}" == "${md}" ]]; then
          found=1
          break
        fi
      done
      if [[ "${found}" -ne 1 ]]; then
        echo "repro-check: production digest not present in images manifest: ${digest}" >&2
        fail=1
      fi
    done
  fi
fi

if ! rg -n 'ANIMUS_SYSTEM_TRAINING_EXECUTOR:-disabled' "${ROOT_DIR}/scripts/system_prod_up.sh" >/dev/null 2>&1; then
  echo "repro-check: scripts/system_prod_up.sh must default ANIMUS_SYSTEM_TRAINING_EXECUTOR to disabled" >&2
  fail=1
fi

if ! rg -n '^profile:[[:space:]]+production$' "${PROD_CP_VALUES}" >/dev/null 2>&1; then
  echo "repro-check: datapilot production values must set profile: production" >&2
  fail=1
fi
if ! rg -n '^profile:[[:space:]]+production$' "${PROD_DP_VALUES}" >/dev/null 2>&1; then
  echo "repro-check: dataplane production values must set profile: production" >&2
  fail=1
fi
if ! rg -n 'fail .*profile=production' "${DEPLOY_DIR}/helm/animus-datapilot/templates/_helpers.tpl" >/dev/null 2>&1; then
  echo "repro-check: datapilot chart must fail when production digest requirements are violated" >&2
  fail=1
fi
if ! rg -n 'fail .*profile=production' "${DEPLOY_DIR}/helm/animus-dataplane/templates/_helpers.tpl" >/dev/null 2>&1; then
  echo "repro-check: dataplane chart must fail when production digest requirements are violated" >&2
  fail=1
fi

if [[ "${fail}" -ne 0 ]]; then
  exit 1
fi

echo "repro-check: ok"
