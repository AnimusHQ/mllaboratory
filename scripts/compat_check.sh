#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
# shellcheck source=/dev/null
source "${ROOT_DIR}/scripts/lib/paths.sh"

fail=0
enterprise_mode="${ANIMUS_COMPAT_ENTERPRISE_MODE:-allow_stub}"
if [[ "${enterprise_mode}" != "allow_stub" && "${enterprise_mode}" != "require_initialized" ]]; then
  echo "compat-check: unsupported ANIMUS_COMPAT_ENTERPRISE_MODE=${enterprise_mode} (expected allow_stub|require_initialized)" >&2
  exit 1
fi

contracts_dir="$(animus_contracts_dir)"
deploy_dir="$(animus_deploy_dir)"
canonical_contracts="${ROOT_DIR}/core/contracts"
legacy_contracts_stub="${ROOT_DIR}/open/api"
canonical_deploy="${ROOT_DIR}/deploy"
legacy_deploy_stub="${ROOT_DIR}/closed/deploy"
canonical_enterprise_scripts="${ROOT_DIR}/enterprise/scripts"
legacy_enterprise_scripts="${ROOT_DIR}/closed/scripts"
legacy_sdk_stub="${ROOT_DIR}/open/sdk"
required_enterprise_scripts=(
  "airgap-bundle.sh"
  "backup.sh"
  "restore.sh"
  "verify-restore.sh"
  "dr-validate.sh"
)

check_stub_only_stub_dir() {
  local dir="$1"
  local label="$2"
  local marker=".stub-marker"
  local has_readme=0
  local entry
  local -a entries=()
  local -a unexpected=()

  if [[ ! -d "${dir}" ]]; then
    echo "compat-check: ${label} stub dir missing: ${dir}" >&2
    fail=1
    return
  fi

  # Legacy stub directories may contain README.md and an optional marker file.
  mapfile -t entries < <(find "${dir}" -mindepth 1 -printf '%P\n' | sort)
  for entry in "${entries[@]}"; do
    if [[ "${entry}" == "README.md" ]]; then
      has_readme=1
      continue
    fi
    if [[ "${entry}" == "${marker}" ]]; then
      continue
    fi
    unexpected+=("${entry}")
  done

  if [[ "${has_readme}" -ne 1 ]]; then
    echo "compat-check: ${label} stub must include README.md: ${dir}" >&2
    fail=1
  fi

  if [[ "${#unexpected[@]}" -gt 0 ]]; then
    echo "compat-check: ${label} stub must contain only README.md (and optional ${marker}): ${dir}" >&2
    echo "compat-check: unexpected entries under ${dir}:" >&2
    printf '  - %s\n' "${unexpected[@]}" >&2
    fail=1
  fi
}

check_non_empty_dir() {
  local dir="$1"
  local label="$2"
  if [[ ! -d "${dir}" ]]; then
    echo "compat-check: ${label} dir missing: ${dir}" >&2
    fail=1
    return
  fi
  if ! find "${dir}" -mindepth 1 -maxdepth 1 | grep -q .; then
    echo "compat-check: ${label} dir must be non-empty: ${dir}" >&2
    fail=1
  fi
}

is_valid_enterprise_scripts_dir() {
  local dir="$1"
  local script
  for script in "${required_enterprise_scripts[@]}"; do
    [[ -f "${dir}/${script}" ]] || return 1
    [[ -x "${dir}/${script}" ]] || return 1
  done
  return 0
}

check_legacy_references() {
  local hits
  # Enforce canonical paths across active build/release surfaces while keeping
  # explicit migration shims and historical docs excluded.
  hits="$(rg -n --hidden '(open/api|closed/deploy|closed/scripts|open/sdk)' "${ROOT_DIR}" \
    --glob '!.git/*' \
    --glob '!docs/**' \
    --glob '!open/api/README.md' \
    --glob '!open/sdk/README.md' \
    --glob '!closed/deploy/README.md' \
    --glob '!closed/scripts/README.md' \
    --glob '!closed/scripts/.stub-marker' \
    --glob '!scripts/compat_check.sh' \
    --glob '!scripts/legacy_scan.sh' \
    --glob '!scripts/submodule_check.sh' \
    --glob '!scripts/lib/paths.sh' \
    --glob '!.gitignore' \
    --glob '!README.md' \
    --glob '!open/demo/quickstart.sh' \
    --glob '!closed/frontend_console/lib/gateway-openapi.ts' \
    --glob '!closed/frontend_console/dist-test/**' || true)"
  if [[ -n "${hits}" ]]; then
    echo "compat-check: hardcoded legacy paths are forbidden outside approved shims" >&2
    echo "${hits}" >&2
    fail=1
  fi
}

if [[ ! -d "${contracts_dir}/openapi" ]]; then
  echo "compat-check: contracts openapi dir missing: ${contracts_dir}/openapi" >&2
  fail=1
fi
if [[ ! -d "${contracts_dir}/baseline" ]]; then
  echo "compat-check: contracts baseline dir missing: ${contracts_dir}/baseline" >&2
  fail=1
fi
if [[ ! -f "${contracts_dir}/pipeline_spec.yaml" ]]; then
  echo "compat-check: contracts pipeline spec missing: ${contracts_dir}/pipeline_spec.yaml" >&2
  fail=1
fi
check_non_empty_dir "${canonical_contracts}/openapi" "canonical contracts openapi"
check_non_empty_dir "${canonical_contracts}/baseline" "canonical contracts baseline"
if [[ ! -f "${canonical_contracts}/pipeline_spec.yaml" ]]; then
  echo "compat-check: canonical contracts pipeline spec missing: ${canonical_contracts}/pipeline_spec.yaml" >&2
  fail=1
fi

if [[ -d "${canonical_contracts}/openapi" && -d "${canonical_contracts}/baseline" && -f "${canonical_contracts}/pipeline_spec.yaml" ]]; then
  if [[ "${contracts_dir}" != "${canonical_contracts}" ]]; then
    echo "compat-check: resolver must prefer canonical contracts dir (${canonical_contracts}), got ${contracts_dir}" >&2
    fail=1
  fi
fi

if [[ ! -d "${deploy_dir}/helm" ]]; then
  echo "compat-check: deploy helm dir missing: ${deploy_dir}/helm" >&2
  fail=1
fi
if [[ ! -d "${deploy_dir}/docker" ]]; then
  echo "compat-check: deploy docker dir missing: ${deploy_dir}/docker" >&2
  fail=1
fi
check_non_empty_dir "${canonical_deploy}/helm" "canonical deploy helm"
check_non_empty_dir "${canonical_deploy}/docker" "canonical deploy docker"
if [[ -d "${canonical_deploy}/helm" && -d "${canonical_deploy}/docker" ]]; then
  if [[ "${deploy_dir}" != "${canonical_deploy}" ]]; then
    echo "compat-check: resolver must prefer canonical deploy dir (${canonical_deploy}), got ${deploy_dir}" >&2
    fail=1
  fi
fi

canonical_enterprise_ready=0
if is_valid_enterprise_scripts_dir "${canonical_enterprise_scripts}"; then
  canonical_enterprise_ready=1
fi

if [[ "${canonical_enterprise_ready}" -eq 1 ]]; then
  enterprise_scripts_dir="$(animus_enterprise_scripts_dir)"
  for script in "${required_enterprise_scripts[@]}"; do
    if [[ ! -f "${enterprise_scripts_dir}/${script}" ]]; then
      echo "compat-check: enterprise script missing: ${enterprise_scripts_dir}/${script}" >&2
      fail=1
    fi
    if [[ ! -x "${enterprise_scripts_dir}/${script}" ]]; then
      echo "compat-check: enterprise script must be executable: ${enterprise_scripts_dir}/${script}" >&2
      fail=1
    fi
  done
  if [[ "${enterprise_scripts_dir}" != "${canonical_enterprise_scripts}" ]]; then
    echo "compat-check: resolver must prefer canonical enterprise scripts dir (${canonical_enterprise_scripts}), got ${enterprise_scripts_dir}" >&2
    fail=1
  fi
else
  if [[ "${enterprise_mode}" == "allow_stub" ]]; then
    echo "compat-check: enterprise/scripts not initialized; allow_stub mode enabled"
  else
    echo "compat-check: enterprise/scripts must be initialized in mode=${enterprise_mode}" >&2
    fail=1
  fi
fi

check_stub_only_stub_dir "${legacy_contracts_stub}" "legacy contracts"
check_stub_only_stub_dir "${legacy_deploy_stub}" "legacy deploy"
check_stub_only_stub_dir "${legacy_enterprise_scripts}" "legacy enterprise scripts"
check_stub_only_stub_dir "${legacy_sdk_stub}" "legacy sdk"

saved_contracts_override="${ANIMUS_CONTRACTS_DIR-}"
saved_contracts_set=0
if [[ "${ANIMUS_CONTRACTS_DIR+x}" == "x" ]]; then
  saved_contracts_set=1
fi
ANIMUS_CONTRACTS_DIR="core/contracts"
override_resolved="$(animus_contracts_dir)"
if [[ "${override_resolved}" != "${canonical_contracts}" ]]; then
  echo "compat-check: ANIMUS_CONTRACTS_DIR override failed, got ${override_resolved}" >&2
  fail=1
fi
if [[ "${saved_contracts_set}" -eq 1 ]]; then
  ANIMUS_CONTRACTS_DIR="${saved_contracts_override}"
else
  unset ANIMUS_CONTRACTS_DIR
fi

saved_deploy_override="${ANIMUS_DEPLOY_DIR-}"
saved_deploy_set=0
if [[ "${ANIMUS_DEPLOY_DIR+x}" == "x" ]]; then
  saved_deploy_set=1
fi
ANIMUS_DEPLOY_DIR="deploy"
deploy_override_resolved="$(animus_deploy_dir)"
if [[ "${deploy_override_resolved}" != "${canonical_deploy}" ]]; then
  echo "compat-check: ANIMUS_DEPLOY_DIR override failed, got ${deploy_override_resolved}" >&2
  fail=1
fi
if [[ "${saved_deploy_set}" -eq 1 ]]; then
  ANIMUS_DEPLOY_DIR="${saved_deploy_override}"
else
  unset ANIMUS_DEPLOY_DIR
fi

saved_enterprise_override="${ANIMUS_ENTERPRISE_SCRIPTS_DIR-}"
saved_enterprise_set=0
if [[ "${ANIMUS_ENTERPRISE_SCRIPTS_DIR+x}" == "x" ]]; then
  saved_enterprise_set=1
fi
if [[ "${canonical_enterprise_ready}" -eq 1 ]]; then
  ANIMUS_ENTERPRISE_SCRIPTS_DIR="enterprise/scripts"
  enterprise_override_resolved="$(animus_enterprise_scripts_dir)"
  if [[ "${enterprise_override_resolved}" != "${canonical_enterprise_scripts}" ]]; then
    echo "compat-check: ANIMUS_ENTERPRISE_SCRIPTS_DIR override failed, got ${enterprise_override_resolved}" >&2
    fail=1
  fi
fi
if ANIMUS_ENTERPRISE_SCRIPTS_DIR="enterprise/missing" animus_enterprise_scripts_dir >/dev/null 2>&1; then
  echo "compat-check: invalid ANIMUS_ENTERPRISE_SCRIPTS_DIR must fail" >&2
  fail=1
fi
if [[ "${saved_enterprise_set}" -eq 1 ]]; then
  ANIMUS_ENTERPRISE_SCRIPTS_DIR="${saved_enterprise_override}"
else
  unset ANIMUS_ENTERPRISE_SCRIPTS_DIR
fi

check_legacy_references

if [[ "${fail}" -ne 0 ]]; then
  exit 1
fi

echo "compat-check: ok"
