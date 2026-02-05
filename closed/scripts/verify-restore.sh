#!/usr/bin/env bash
set -euo pipefail

export ANIMUS_DR_VALIDATE=1
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec "${script_dir}/dr-validate.sh"
