#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GITMODULES="${ROOT_DIR}/.gitmodules"
POLICY_FILE="${ANIMUS_GUARDRAIL_POLICY_FILE:-${ROOT_DIR}/scripts/guardrail_policy.env}"
FAIL=0

if [[ ! -f "${GITMODULES}" ]]; then
  echo "submodule-check: missing ${GITMODULES}" >&2
  exit 1
fi

if [[ -z "${ANIMUS_SUBMODULE_CHECK_ENFORCE:-}" ]]; then
  if [[ -f "${POLICY_FILE}" ]]; then
    # shellcheck source=/dev/null
    source "${POLICY_FILE}"
  fi
fi

ENFORCE="${ANIMUS_SUBMODULE_CHECK_ENFORCE:-${ANIMUS_PR_SUBMODULE_ENFORCE:-0}}"
REQUIRE_INIT="${ANIMUS_SUBMODULE_CHECK_REQUIRE_INIT:-0}"

is_placeholder_url() {
  local url="$1"
  [[ "${url}" == TODO_* ]]
}

is_valid_submodule_url() {
  local url="$1"
  [[ "${url}" == TODO_* ]] || [[ "${url}" == *"://"* ]] || [[ "${url}" == git@*:* ]] || [[ "${url}" == ./* ]] || [[ "${url}" == ../* ]]
}

read_submodule_url() {
  local key="$1"
  git config -f "${GITMODULES}" --get "${key}" || true
}

require_gitlink() {
  local path="$1"
  local entry
  entry="$(git -C "${ROOT_DIR}" ls-files -s -- "${path}" | awk '{print $1}')"
  if [[ "${entry}" != "160000" ]]; then
    echo "submodule-check: ${path} must be tracked as gitlink (mode 160000)" >&2
    FAIL=1
  fi
}

check_stub_only_dir() {
  local dir="$1"
  local label="$2"
  local has_readme=0
  local entry
  local -a entries=()
  local -a unexpected=()

  if [[ ! -d "${dir}" ]]; then
    return
  fi

  mapfile -t entries < <(find "${dir}" -mindepth 1 -printf '%P\n' | sort)
  for entry in "${entries[@]}"; do
    if [[ "${entry}" == "README.md" ]]; then
      has_readme=1
      continue
    fi
    if [[ "${entry}" == ".stub-marker" ]]; then
      continue
    fi
    unexpected+=("${entry}")
  done

  if [[ "${has_readme}" -ne 1 ]]; then
    echo "submodule-check: ${label} must include README.md: ${dir}" >&2
    FAIL=1
  fi

  if [[ "${#unexpected[@]}" -gt 0 ]]; then
    echo "submodule-check: ${label} must be stub-only (README.md + optional .stub-marker): ${dir}" >&2
    printf '  - %s\n' "${unexpected[@]}" >&2
    FAIL=1
  fi
}

enterprise_url="$(read_submodule_url 'submodule.enterprise.url')"
sdk_url="$(read_submodule_url 'submodule.sdk.url')"
legacy_sdk_url="$(read_submodule_url 'submodule.open/sdk.url')"

if [[ -n "${legacy_sdk_url}" ]]; then
  echo "submodule-check: legacy submodule entry is forbidden: submodule.open/sdk" >&2
  FAIL=1
fi

if [[ -z "${enterprise_url}" ]]; then
  echo "submodule-check: missing submodule.enterprise.url in .gitmodules" >&2
  FAIL=1
fi
if [[ -z "${sdk_url}" ]]; then
  echo "submodule-check: missing submodule.sdk.url in .gitmodules" >&2
  FAIL=1
fi

if [[ -n "${enterprise_url}" ]] && ! is_valid_submodule_url "${enterprise_url}"; then
  echo "submodule-check: invalid enterprise URL format: ${enterprise_url}" >&2
  FAIL=1
fi
if [[ -n "${sdk_url}" ]] && ! is_valid_submodule_url "${sdk_url}"; then
  echo "submodule-check: invalid sdk URL format: ${sdk_url}" >&2
  FAIL=1
fi

require_gitlink "enterprise"
require_gitlink "sdk"

if git -C "${ROOT_DIR}" ls-files --error-unmatch enterprise/* >/dev/null 2>&1; then
  echo "submodule-check: enterprise/* must not be tracked in open-core repo (gitlink only)" >&2
  FAIL=1
fi
if git -C "${ROOT_DIR}" ls-files --error-unmatch sdk/* >/dev/null 2>&1; then
  echo "submodule-check: sdk/* must not be tracked in open-core repo (gitlink only)" >&2
  FAIL=1
fi

check_stub_only_dir "${ROOT_DIR}/open/sdk" "legacy sdk"

if is_placeholder_url "${enterprise_url}"; then
  if [[ "${enterprise_url}" != "TODO_ENTERPRISE_REPO_URL" ]]; then
    echo "submodule-check: enterprise placeholder must be TODO_ENTERPRISE_REPO_URL (got ${enterprise_url})" >&2
    FAIL=1
  fi
  if [[ "${ENFORCE}" == "1" ]]; then
    echo "submodule-check: enterprise submodule URL placeholder detected: ${enterprise_url}" >&2
    FAIL=1
  else
    echo "submodule-check: warning: enterprise URL is placeholder (${enterprise_url})"
  fi
fi

if is_placeholder_url "${sdk_url}"; then
  if [[ "${sdk_url}" != "TODO_SDK_REPO_URL" ]]; then
    echo "submodule-check: sdk placeholder must be TODO_SDK_REPO_URL (got ${sdk_url})" >&2
    FAIL=1
  fi
  if [[ "${ENFORCE}" == "1" ]]; then
    echo "submodule-check: sdk submodule URL placeholder detected: ${sdk_url}" >&2
    FAIL=1
  else
    echo "submodule-check: warning: sdk URL is placeholder (${sdk_url})"
  fi
fi

if [[ "${ENFORCE}" == "1" && "${REQUIRE_INIT}" == "1" ]]; then
  if [[ ! -x "${ROOT_DIR}/enterprise/scripts/airgap-bundle.sh" ]]; then
    echo "submodule-check: enforce mode requires initialized enterprise/scripts content" >&2
    FAIL=1
  fi
  if [[ ! -f "${ROOT_DIR}/sdk/python/pyproject.toml" ]]; then
    echo "submodule-check: enforce mode requires initialized sdk/python/pyproject.toml" >&2
    FAIL=1
  fi
fi

if [[ "${FAIL}" -ne 0 ]]; then
  exit 1
fi

echo "submodule-check: ok (enforce=${ENFORCE} require_init=${REQUIRE_INIT})"
