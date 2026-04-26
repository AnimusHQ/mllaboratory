#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ARTIFACTS_ROOT="${ROOT_DIR}/artifacts"
TIMESTAMP="$(date -u +%Y%m%dT%H%M%SZ)"
ARTIFACTS_DIR="${ANIMUS_ARTIFACTS_DIR:-${ARTIFACTS_ROOT}/${TIMESTAMP}}"

mkdir -p "$ARTIFACTS_DIR"
export ANIMUS_ARTIFACTS_DIR="$ARTIFACTS_DIR"
export ANIMUS_GO_TEST_JSON_DIR="${ANIMUS_GO_TEST_JSON_DIR:-$ARTIFACTS_DIR}"
export ANIMUS_E2E_FAILURES="${ANIMUS_E2E_FAILURES:-1}"
export ANIMUS_INTEGRATION="${ANIMUS_INTEGRATION:-1}"
export ANIMUS_TEST_DATABASE_URL="${ANIMUS_TEST_DATABASE_URL:-postgres://animus:animus@localhost:55432/animus?sslmode=disable}"
export ANIMUS_TEST_MINIO_ENDPOINT="${ANIMUS_TEST_MINIO_ENDPOINT:-localhost:59000}"
export ANIMUS_TEST_MINIO_ACCESS_KEY="${ANIMUS_TEST_MINIO_ACCESS_KEY:-animus}"
export ANIMUS_TEST_MINIO_SECRET_KEY="${ANIMUS_TEST_MINIO_SECRET_KEY:-animusminio}"
export ANIMUS_TEST_MINIO_BUCKET_DATASETS="${ANIMUS_TEST_MINIO_BUCKET_DATASETS:-datasets}"
export ANIMUS_TEST_MINIO_BUCKET_ARTIFACTS="${ANIMUS_TEST_MINIO_BUCKET_ARTIFACTS:-artifacts}"
export ANIMUS_SYSTEM_ENABLE="${ANIMUS_SYSTEM_ENABLE:-1}"

cleanup() {
  make -s artifacts-collect || true
  make -s system-down || true
  make -s integration-down || true
}
trap cleanup EXIT

make -s guardrails-check
make -s openapi-lint
make -s integration-up
make -s test
make -s integrations-test
make -s ui-build
make -s ui-test
make -s system-up
make -s system-test
make -s dr-validate
