SHELL := /bin/bash

GO ?= go
PY ?= python3
COMPOSE_BIN ?= docker compose -f open/demo/docker-compose.yml
CANONICAL_UI_DIR ?= closed/ui

# Repository layout documentation (path resolution is implemented in scripts/lib/paths.sh).
CANONICAL_CONTRACTS_DIR ?= core/contracts
CANONICAL_DEPLOY_DIR ?= deploy
CANONICAL_ENTERPRISE_SCRIPTS_DIR ?= enterprise/scripts

ANIMUS_CONTRACTS_DIR ?= $(CANONICAL_CONTRACTS_DIR)
ANIMUS_DEPLOY_DIR ?= $(CANONICAL_DEPLOY_DIR)
ANIMUS_ENTERPRISE_SCRIPTS_DIR ?= $(CANONICAL_ENTERPRISE_SCRIPTS_DIR)
ANIMUS_SDK_DIR ?= sdk
ANIMUS_SDK_REQUIRED ?= 0

GO_PACKAGES ?= ./...
LINT_BIN_DIR := $(CURDIR)/.bin
GOLANGCI_LINT_VERSION ?= v1.64.8
GOLANGCI_LINT_VERSION_STR ?= 1.64.8
GOLANGCI_LINT ?= $(LINT_BIN_DIR)/golangci-lint
GO_FILES := $(shell find . -type f -name '*.go' -not -path './.cache/*' -not -path './.git/*' -not -path './vendor/*' -not -path './*/node_modules/*' -not -path './*/dist-test/*')

CACHE_DIR := $(CURDIR)/.cache
export GOCACHE := $(CACHE_DIR)/go-build
export GOMODCACHE := $(CACHE_DIR)/go-mod
export GOTMPDIR := $(CACHE_DIR)/go-tmp

.PHONY: bootstrap fmt test integrations-test dr-validate lint build ci-baseline openapi-lint openapi-compat openapi-check openapi-baseline-update compat-check legacy-scan arch-check repro-check base-image-check policy-validate hygiene-check submodule-check sdk-path guardrails-check dev demo demo-smoke demo-down e2e e2e-full system-test sbom sbom-check sign-images verify-images vuln-scan supply-chain helm-images sast-scan dep-scan integration-up integration-down system-up system-down system-prod-up system-prod-down system-prod-health make-up-prod make-prod-up ui-build ui-test full-stack artifacts-collect images-build

bootstrap:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)" "$(GOTMPDIR)"
	@$(GO) mod tidy

fmt:
	@if [ -z "$(GO_FILES)" ]; then \
		echo "No Go files found."; \
		exit 0; \
	fi
	@$(GO) fmt ./...
	@gofmt -w $(GO_FILES)

lint:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)" "$(GOTMPDIR)"
	@echo "==> gofmt (check)"
	@unformatted="$$(gofmt -l $(GO_FILES))"; \
	if [ -n "$$unformatted" ]; then \
		echo "$$unformatted"; \
		echo "gofmt check failed (run: gofmt -w <files>)"; \
		exit 1; \
	fi
	@echo "==> go vet"
	@$(GO) vet $(GO_PACKAGES)
	@echo "==> golangci-lint"
	@mkdir -p "$(LINT_BIN_DIR)"
	@if [ ! -x "$(GOLANGCI_LINT)" ] || ! "$(GOLANGCI_LINT)" version 2>/dev/null | grep -q "$(GOLANGCI_LINT_VERSION_STR)"; then \
		echo "==> installing golangci-lint $(GOLANGCI_LINT_VERSION)"; \
		GOBIN="$(LINT_BIN_DIR)" $(GO) install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION); \
	fi
	@$(GOLANGCI_LINT) run
	@echo "==> python compileall"
	@sdk_dir="$$(ANIMUS_SDK_DIR="$(ANIMUS_SDK_DIR)" $(MAKE) -s sdk-path 2>/dev/null || true)"; \
	if [ -z "$$sdk_dir" ]; then \
		if [ "$(ANIMUS_SDK_REQUIRED)" = "1" ]; then \
			echo "SDK path resolution failed."; \
			echo "Run: git submodule update --init --recursive sdk"; \
			exit 1; \
		fi; \
		echo "==> python compileall skipped (SDK path unresolved)"; \
		exit 0; \
	fi; \
	py_sdk_dir="$$sdk_dir/python"; \
	if [ ! -d "$$py_sdk_dir/src" ]; then \
		if [ "$(ANIMUS_SDK_REQUIRED)" = "1" ]; then \
			echo "Python SDK source not found at $$py_sdk_dir/src."; \
			echo "Run: git submodule update --init --recursive sdk"; \
			exit 1; \
		fi; \
		echo "==> python compileall skipped (SDK not initialized at $$py_sdk_dir/src)"; \
		exit 0; \
	fi; \
	PYTHONPATH="$$py_sdk_dir/src" $(PY) -m compileall -q "$$py_sdk_dir/src"

test:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)" "$(GOTMPDIR)"
	@echo "==> go test"
	@unit_json=""; \
	if [ -n "$$ANIMUS_GO_TEST_JSON_DIR" ]; then \
		unit_json="$${ANIMUS_GO_TEST_JSON_DIR}/go-test-unit.json"; \
	fi; \
	ANIMUS_GO_TEST_JSON="$$unit_json" ./scripts/go_test.sh $(GO_PACKAGES)
	@if [ "$$ANIMUS_INTEGRATION" = "1" ]; then \
		echo "==> integration tests"; \
		integration_json=""; \
		if [ -n "$$ANIMUS_GO_TEST_JSON_DIR" ]; then \
			integration_json="$${ANIMUS_GO_TEST_JSON_DIR}/go-test-integration.json"; \
		fi; \
		ANIMUS_INTEGRATION=1 ANIMUS_GO_TEST_JSON="$$integration_json" ./scripts/go_test.sh -tags=integration ./closed/...; \
	else \
		echo "==> integration tests skipped (set ANIMUS_INTEGRATION=1)"; \
	fi
	@echo "==> python tests"
	@sdk_dir="$$(ANIMUS_SDK_DIR="$(ANIMUS_SDK_DIR)" $(MAKE) -s sdk-path 2>/dev/null || true)"; \
	if [ -z "$$sdk_dir" ]; then \
		if [ "$(ANIMUS_SDK_REQUIRED)" = "1" ]; then \
			echo "SDK path resolution failed."; \
			echo "Run: git submodule update --init --recursive sdk"; \
			exit 1; \
		fi; \
		echo "==> python tests skipped (SDK path unresolved)"; \
		exit 0; \
	fi; \
	py_sdk_dir="$$sdk_dir/python"; \
	if [ ! -d "$$py_sdk_dir/tests" ]; then \
		if [ "$(ANIMUS_SDK_REQUIRED)" = "1" ]; then \
			echo "Python SDK tests not found at $$py_sdk_dir/tests."; \
			echo "Run: git submodule update --init --recursive sdk"; \
			exit 1; \
		fi; \
		echo "==> python tests skipped (SDK not initialized at $$py_sdk_dir/tests)"; \
		exit 0; \
	fi; \
	PYTHONPATH="$$py_sdk_dir/src" $(PY) -m unittest discover -s "$$py_sdk_dir/tests" -p 'test_*.py'

integrations-test:
	@json_out=""; \
	if [ -n "$$ANIMUS_GO_TEST_JSON_DIR" ]; then \
		json_out="$${ANIMUS_GO_TEST_JSON_DIR}/go-test-integrations.json"; \
	fi; \
	ANIMUS_GO_TEST_JSON="$$json_out" ./scripts/go_test.sh ./closed/...

integration-up:
	@./scripts/integration_up.sh

integration-down:
	@./scripts/integration_down.sh

ui-build:
	@cd "$(CANONICAL_UI_DIR)" && npm ci --no-audit && npm run build

ui-test:
	@cd "$(CANONICAL_UI_DIR)" && npm ci --no-audit && npm run test

images-build:
	@ANIMUS_DEPLOY_DIR="$(ANIMUS_DEPLOY_DIR)" ./scripts/build_images.sh

system-up:
	@ANIMUS_DEPLOY_DIR="$(ANIMUS_DEPLOY_DIR)" ./scripts/kind_up.sh

system-down:
	@./scripts/kind_down.sh

system-prod-up:
	@ANIMUS_DEPLOY_DIR="$(ANIMUS_DEPLOY_DIR)" ./scripts/system_prod_up.sh

system-prod-down:
	@./scripts/system_prod_down.sh

system-prod-health:
	@./scripts/system_prod_health.sh

make-up-prod:
	@$(MAKE) system-prod-up

make-prod-up:
	@$(MAKE) system-prod-up

full-stack:
	@./scripts/full_stack.sh

artifacts-collect:
	@./scripts/artifacts_collect.sh


dr-validate:
	@if [ "$$ANIMUS_DR_VALIDATE" != "1" ]; then \
		echo "dr-validate: ANIMUS_DR_VALIDATE not set; skipping."; \
		exit 0; \
	fi
	@ANIMUS_ENTERPRISE_SCRIPTS_DIR="$(ANIMUS_ENTERPRISE_SCRIPTS_DIR)" ./enterprise/scripts/dr-validate.sh

build:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)" "$(GOTMPDIR)"
	@echo "==> go build"
	@$(GO) build $(GO_PACKAGES)

ci-baseline: submodule-check hygiene-check lint build test helm-images openapi-check

openapi-lint:
	@ANIMUS_CONTRACTS_DIR="$(ANIMUS_CONTRACTS_DIR)" ./scripts/openapi_lint.sh

openapi-compat:
	@ANIMUS_CONTRACTS_DIR="$(ANIMUS_CONTRACTS_DIR)" ./scripts/openapi_breaking_check.sh

openapi-check: openapi-lint openapi-compat

openapi-baseline-update:
	@if [ "$$OPENAPI_BASELINE_UPDATE" != "1" ]; then \
		echo "openapi-baseline-update: set OPENAPI_BASELINE_UPDATE=1"; \
		exit 1; \
	fi
	@ANIMUS_CONTRACTS_DIR="$(ANIMUS_CONTRACTS_DIR)" OPENAPI_BASELINE_UPDATE=1 ./scripts/openapi_breaking_check.sh

compat-check:
	@ANIMUS_CONTRACTS_DIR="$(ANIMUS_CONTRACTS_DIR)" \
	 ANIMUS_DEPLOY_DIR="$(ANIMUS_DEPLOY_DIR)" \
	 ANIMUS_ENTERPRISE_SCRIPTS_DIR="$(ANIMUS_ENTERPRISE_SCRIPTS_DIR)" \
	 ./scripts/compat_check.sh

legacy-scan:
	@./scripts/legacy_scan.sh

arch-check:
	@./scripts/arch_check.sh

repro-check:
	@CANONICAL_UI_DIR="$(CANONICAL_UI_DIR)" \
	 ANIMUS_DEPLOY_DIR="$(ANIMUS_DEPLOY_DIR)" \
	 ./scripts/repro_check.sh

base-image-check:
	@ANIMUS_DEPLOY_DIR="$(ANIMUS_DEPLOY_DIR)" ./scripts/base_image_check.sh

policy-validate:
	@GOFLAGS=-mod=vendor go run ./cmd/policy-validate \
		--policy ./deploy/policy/kyverno-signed-images.yaml \
		--signed ./deploy/policy/samples/pod-signed.yaml \
		--unsigned ./deploy/policy/samples/pod-unsigned.yaml

hygiene-check:
	@./scripts/hygiene_check.sh

submodule-check:
	@./scripts/submodule_check.sh

sdk-path:
	@ANIMUS_SDK_DIR="$(ANIMUS_SDK_DIR)" bash -c 'source ./scripts/lib/paths.sh; animus_sdk_dir'

guardrails-check:
	@./scripts/precommit_guardrails.sh

helm-images:
	@ANIMUS_DEPLOY_DIR="$(ANIMUS_DEPLOY_DIR)" ./scripts/list_images.sh

sbom:
	@./scripts/sbom.sh

sbom-check:
	@./scripts/sbom_check.sh

sign-images:
	@./scripts/sign_images.sh

verify-images:
	@./scripts/verify_images.sh

vuln-scan:
	@./scripts/vuln_scan.sh

sast-scan:
	@./scripts/sast_scan.sh

dep-scan:
	@./scripts/dep_scan.sh

supply-chain:
	@./scripts/supply_chain.sh

e2e:
	@./scripts/e2e.sh

e2e-full:
	@./scripts/e2e_full.sh

system-test: e2e-full

dev:
	@COMPOSE_BIN="$(COMPOSE_BIN)" ./scripts/dev.sh

demo:
	@echo "DEPRECATED: use 'make dev' (demo target will be removed after 2 releases)" >&2
	@$(MAKE) dev

demo-smoke:
	@echo "DEPRECATED: use 'make dev DEV_ARGS=--smoke' (demo-smoke target will be removed after 2 releases)" >&2
	@DEV_ARGS="--smoke" $(MAKE) dev

demo-down:
	@echo "DEPRECATED: use 'make dev DEV_ARGS=--down' (demo-down target will be removed after 2 releases)" >&2
	@DEV_ARGS="--down" $(MAKE) dev
