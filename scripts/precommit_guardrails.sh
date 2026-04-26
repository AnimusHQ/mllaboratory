#!/usr/bin/env bash
set -euo pipefail

blocked=$(git diff --cached --name-only -- README.md docs/README_ENTERPRISE_CHECKLIST.md || true)
if [ -n "$blocked" ]; then
  echo "ERROR: guardrail files staged. Unstage before committing."
  echo "$blocked"
  exit 1
fi
