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

.PHONY: bootstrap fmt test integrations-test lint build openapi-lint dev demo demo-smoke demo-down e2e sbom vuln-scan supply-chain helm-images

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
	@./scripts/go_test.sh $(GO_PACKAGES)
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
	@./scripts/go_test.sh ./closed/...


build:
	@mkdir -p "$(GOCACHE)" "$(GOMODCACHE)" "$(GOTMPDIR)"
	@echo "==> go build"
	@$(GO) build $(GO_PACKAGES)

openapi-lint:
	@./scripts/openapi_lint.sh

helm-images:
	@./scripts/list_images.sh

sbom:
	@./scripts/sbom.sh

vuln-scan:
	@./scripts/vuln_scan.sh

supply-chain:
	@./scripts/supply_chain.sh

e2e:
	@./scripts/e2e.sh

dev:
	@COMPOSE_BIN="$(COMPOSE_BIN)" ./closed/scripts/dev.sh

demo:
	@ANIMUS_DEV_SKIP_UI=1 bash ./open/demo/quickstart.sh

demo-smoke:
	@ANIMUS_DEV_SKIP_UI=1 bash ./open/demo/quickstart.sh --smoke

demo-down:
	@bash ./open/demo/quickstart.sh --down
