SHELL := /bin/bash

GO ?= go
PY ?= python3
COMPOSE_BIN ?= docker compose -f closed/deploy/docker-compose.yml

PY_SDK_DIR ?= open/sdk/python
PY_SDK_SUBMODULE ?= open/sdk
GO_PACKAGES ?= ./...
LINT_BIN_DIR := $(CURDIR)/.bin
GOLANGCI_LINT_VERSION ?= v1.64.8
GOLANGCI_LINT_VERSION_STR ?= 1.64.8
GOLANGCI_LINT ?= $(LINT_BIN_DIR)/golangci-lint
GO_FILES := $(shell find . -type f -name '*.go' -not -path './.cache/*' -not -path './.git/*')

CACHE_DIR := $(CURDIR)/.cache
export GOCACHE := $(CACHE_DIR)/go-build
export GOMODCACHE := $(CACHE_DIR)/go-mod
export GOTMPDIR := $(CACHE_DIR)/go-tmp

.PHONY: bootstrap fmt test integrations-test dr-validate lint build openapi-lint openapi-compat guardrails-check dev demo demo-smoke demo-down e2e e2e-full system-test sbom vuln-scan supply-chain helm-images sast-scan dep-scan integration-up integration-down system-up system-down system-prod-up system-prod-down system-prod-health make-up-prod make-prod-up ui-build ui-test full-stack artifacts-collect

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
	@if [ ! -d "$(PY_SDK_DIR)/src" ]; then \
		echo "Python SDK not found at $(PY_SDK_DIR)."; \
		if [ "$(PY_SDK_DIR)" = "$(PY_SDK_SUBMODULE)/python" ] && [ -e .git ] && command -v git >/dev/null 2>&1; then \
			echo "==> init submodule $(PY_SDK_SUBMODULE)"; \
			git submodule update --init --recursive "$(PY_SDK_SUBMODULE)"; \
		else \
			echo "If you're using the submodule, run: git submodule update --init --recursive $(PY_SDK_SUBMODULE)"; \
			exit 1; \
		fi; \
	fi
	@PYTHONPATH="$(PY_SDK_DIR)/src" $(PY) -m compileall -q "$(PY_SDK_DIR)/src"

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
	@if [ ! -d "$(PY_SDK_DIR)/tests" ]; then \
		echo "Python SDK tests not found at $(PY_SDK_DIR)/tests."; \
		if [ "$(PY_SDK_DIR)" = "$(PY_SDK_SUBMODULE)/python" ] && [ -e .git ] && command -v git >/dev/null 2>&1; then \
			echo "==> init submodule $(PY_SDK_SUBMODULE)"; \
			git submodule update --init --recursive "$(PY_SDK_SUBMODULE)"; \
		else \
			echo "If you're using the submodule, run: git submodule update --init --recursive $(PY_SDK_SUBMODULE)"; \
			exit 1; \
		fi; \
	fi
	@PYTHONPATH="$(PY_SDK_DIR)/src" $(PY) -m unittest discover -s "$(PY_SDK_DIR)/tests" -p 'test_*.py'

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
	@cd closed/frontend_console && NEXT_PUBLIC_SITE_URL="$${NEXT_PUBLIC_SITE_URL:-http://localhost:3001}" npm run build

ui-test:
	@cd closed/frontend_console && NEXT_PUBLIC_SITE_URL="$${NEXT_PUBLIC_SITE_URL:-http://localhost:3001}" npm run test

system-up:
	@./scripts/kind_up.sh

system-down:
	@./scripts/kind_down.sh

system-prod-up:
	@./scripts/system_prod_up.sh

system-prod-down:
	@./scripts/system_prod_down.sh

system-prod-health:
	@./scripts/system_prod_health.sh

make-up-prod:
	@./scripts/system_prod_up.sh

make-prod-up:
	@./scripts/system_prod_up.sh

full-stack:
	@./scripts/full_stack.sh

artifacts-collect:
	@./scripts/artifacts_collect.sh


dr-validate:
	@if [ "$$ANIMUS_DR_VALIDATE" != "1" ]; then \
		echo "dr-validate: ANIMUS_DR_VALIDATE not set; skipping."; \
		exit 0; \
	fi
	@./closed/scripts/dr-validate.sh

build:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)" "$(GOTMPDIR)"
	@echo "==> go build"
	@$(GO) build $(GO_PACKAGES)

openapi-lint:
	@./scripts/openapi_lint.sh

openapi-compat:
	@./scripts/openapi_breaking_check.sh

guardrails-check:
	@./scripts/precommit_guardrails.sh

helm-images:
	@./scripts/list_images.sh

sbom:
	@./scripts/sbom.sh

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
	@COMPOSE_BIN="$(COMPOSE_BIN)" ./closed/scripts/dev.sh

demo:
	@ANIMUS_DEV_SKIP_UI=1 bash ./open/demo/quickstart.sh

demo-smoke:
	@ANIMUS_DEV_SKIP_UI=1 bash ./open/demo/quickstart.sh --smoke

demo-down:
	@bash ./open/demo/quickstart.sh --down
