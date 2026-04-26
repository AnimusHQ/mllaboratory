#!/usr/bin/env bash
set -euo pipefail

_animus_paths_error() {
  echo "animus paths: $*" >&2
}

_animus_paths_root_dir() {
  local root_dir
  if [[ -n "${ROOT_DIR:-}" ]]; then
    if [[ "${ROOT_DIR}" == /* ]]; then
      root_dir="${ROOT_DIR}"
    else
      root_dir="$(pwd -P)/${ROOT_DIR}"
    fi
    if [[ ! -d "${root_dir}" ]]; then
      _animus_paths_error "ROOT_DIR does not exist: ${root_dir}"
      return 1
    fi
    (cd "${root_dir}" && pwd -P)
    return 0
  fi
  local script_dir
  script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
  printf '%s\n' "${script_dir}"
}

_animus_paths_resolve() {
  local value="$1"
  if [[ "${value}" == /* ]]; then
    printf '%s\n' "${value}"
    return
  fi
  printf '%s/%s\n' "$(_animus_paths_root_dir)" "${value}"
}

_animus_paths_validate_contracts_dir() {
  local dir="$1"
  [[ -d "${dir}" ]] || return 1
  [[ -d "${dir}/openapi" ]] || return 1
  [[ -d "${dir}/baseline" ]] || return 1
  [[ -f "${dir}/pipeline_spec.yaml" ]] || return 1
  return 0
}

_animus_paths_validate_deploy_dir() {
  local dir="$1"
  [[ -d "${dir}" ]] || return 1
  [[ -d "${dir}/helm" ]] || return 1
  [[ -d "${dir}/docker" ]] || return 1
  return 0
}

_animus_paths_validate_sdk_dir() {
  local dir="$1"
  [[ -d "${dir}" ]] || return 1
  # Require real SDK content so scaffolding-only dirs do not shadow legacy path.
  [[ -f "${dir}/python/pyproject.toml" ]] || return 1
  return 0
}

animus_contracts_dir() {
  local root_dir
  local override_dir
  local canonical_dir
  local legacy_dir
  root_dir="$(_animus_paths_root_dir)"
  if [[ -n "${ANIMUS_CONTRACTS_DIR:-}" ]]; then
    override_dir="$(_animus_paths_resolve "${ANIMUS_CONTRACTS_DIR}")"
    if ! _animus_paths_validate_contracts_dir "${override_dir}"; then
      _animus_paths_error "invalid ANIMUS_CONTRACTS_DIR=${ANIMUS_CONTRACTS_DIR} (resolved: ${override_dir}); expected {openapi/, baseline/, pipeline_spec.yaml}"
      return 1
    fi
    printf '%s\n' "${override_dir}"
    return
  fi

  canonical_dir="${root_dir}/core/contracts"
  if _animus_paths_validate_contracts_dir "${canonical_dir}"; then
    printf '%s\n' "${canonical_dir}"
    return
  fi

  legacy_dir="${root_dir}/open/api"
  if _animus_paths_validate_contracts_dir "${legacy_dir}"; then
    printf '%s\n' "${legacy_dir}"
    return
  fi

  _animus_paths_error "no valid contracts directory found; checked ${canonical_dir} and ${legacy_dir}"
  return 1
}

animus_deploy_dir() {
  local root_dir
  local override_dir
  local canonical_dir
  local legacy_dir
  root_dir="$(_animus_paths_root_dir)"
  if [[ -n "${ANIMUS_DEPLOY_DIR:-}" ]]; then
    override_dir="$(_animus_paths_resolve "${ANIMUS_DEPLOY_DIR}")"
    if ! _animus_paths_validate_deploy_dir "${override_dir}"; then
      _animus_paths_error "invalid ANIMUS_DEPLOY_DIR=${ANIMUS_DEPLOY_DIR} (resolved: ${override_dir}); expected {helm/, docker/}"
      return 1
    fi
    printf '%s\n' "${override_dir}"
    return
  fi

  canonical_dir="${root_dir}/deploy"
  if _animus_paths_validate_deploy_dir "${canonical_dir}"; then
    printf '%s\n' "${canonical_dir}"
    return
  fi

  legacy_dir="${root_dir}/closed/deploy"
  if _animus_paths_validate_deploy_dir "${legacy_dir}"; then
    printf '%s\n' "${legacy_dir}"
    return
  fi

  _animus_paths_error "no valid deploy directory found; checked ${canonical_dir} and ${legacy_dir}"
  return 1
}

animus_sdk_dir() {
  local root_dir
  local override_dir
  local canonical_dir
  local legacy_dir
  root_dir="$(_animus_paths_root_dir)"

  if [[ -n "${ANIMUS_SDK_DIR:-}" ]]; then
    override_dir="$(_animus_paths_resolve "${ANIMUS_SDK_DIR}")"
    if _animus_paths_validate_sdk_dir "${override_dir}"; then
      printf '%s\n' "${override_dir}"
      return
    fi
    # Keep default ANIMUS_SDK_DIR=sdk compatible during migration by allowing fallback.
    if [[ "${ANIMUS_SDK_DIR}" != "sdk" ]]; then
      _animus_paths_error "invalid ANIMUS_SDK_DIR=${ANIMUS_SDK_DIR} (resolved: ${override_dir}); expected python/pyproject.toml"
      return 1
    fi
  fi

  canonical_dir="${root_dir}/sdk"
  if _animus_paths_validate_sdk_dir "${canonical_dir}"; then
    printf '%s\n' "${canonical_dir}"
    return
  fi

  legacy_dir="${root_dir}/open/sdk"
  if _animus_paths_validate_sdk_dir "${legacy_dir}"; then
    printf '%s\n' "${legacy_dir}"
    return
  fi

  _animus_paths_error "no valid SDK directory found; checked ${canonical_dir} and ${legacy_dir}. Run: git submodule update --init --recursive sdk."
  return 1
}

_animus_paths_validate_enterprise_dir() {
  local dir="$1"
  local required_scripts=(
    "airgap-bundle.sh"
    "backup.sh"
    "restore.sh"
    "verify-restore.sh"
    "dr-validate.sh"
  )
  [[ -d "${dir}" ]] || return 1
  local script
  for script in "${required_scripts[@]}"; do
    [[ -f "${dir}/${script}" ]] || return 1
    [[ -x "${dir}/${script}" ]] || return 1
  done
  return 0
}

animus_enterprise_scripts_dir() {
  local root_dir
  root_dir="$(_animus_paths_root_dir)"
  if [[ -n "${ANIMUS_ENTERPRISE_SCRIPTS_DIR:-}" ]]; then
    local resolved_override
    resolved_override="$(_animus_paths_resolve "${ANIMUS_ENTERPRISE_SCRIPTS_DIR}")"
    if ! _animus_paths_validate_enterprise_dir "${resolved_override}"; then
      _animus_paths_error "invalid ANIMUS_ENTERPRISE_SCRIPTS_DIR=${ANIMUS_ENTERPRISE_SCRIPTS_DIR} (resolved: ${resolved_override}); expected executable scripts {airgap-bundle.sh, backup.sh, restore.sh, verify-restore.sh, dr-validate.sh}"
      return 1
    fi
    printf '%s\n' "${resolved_override}"
    return
  fi
  if _animus_paths_validate_enterprise_dir "${root_dir}/enterprise/scripts"; then
    printf '%s\n' "${root_dir}/enterprise/scripts"
    return
  fi

  _animus_paths_error "no valid enterprise scripts directory found; checked ${root_dir}/enterprise/scripts"
  return 1
}
