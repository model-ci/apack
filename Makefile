BIN_DIR := bin
Apack_CMD := ./
VERSION := $(shell cat VERSION)
GIT_COMMIT ?= $(shell git rev-parse HEAD)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

TARGET_OS ?= linux
TARGET_ARCH ?= amd64

Apack_BIN := $(BIN_DIR)/$(TARGET_OS)_$(TARGET_ARCH)/apack

LDFLAGS := -X "github.com/model-ci/apack/internal/consts.version=$(VERSION)" \
           -X "github.com/model-ci/apack/internal/consts.gitCommit=$(GIT_COMMIT)" \
           -X "github.com/model-ci/apack/internal/consts.buildTime=$(BUILD_TIME)"

.PHONY: all build apack clean fmt lint test release help

all: build
build: apack

apack:
	mkdir -p $(BIN_DIR)/$(TARGET_OS)_$(TARGET_ARCH)
	GOOS=$(TARGET_OS) GOARCH=$(TARGET_ARCH) CGO_ENABLED=0 go build -ldflags '$(LDFLAGS)' -o $(Apack_BIN) $(Apack_CMD)

clean:
	rm -rf $(BIN_DIR)

fmt:
	go fmt ./...

lint:
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Please install golangci-lint: https://golangci-lint.run/usage/install/"; \
		exit 1; \
	fi
	golangci-lint run ./...

test:
	go test ./...

release: clean
	$(MAKE) build TARGET_OS=linux TARGET_ARCH=amd64
	$(MAKE) build TARGET_OS=linux TARGET_ARCH=arm64
	$(MAKE) build TARGET_OS=darwin TARGET_ARCH=amd64
	$(MAKE) build TARGET_OS=darwin TARGET_ARCH=arm64
	$(MAKE) build TARGET_OS=windows TARGET_ARCH=amd64
	$(MAKE) build TARGET_OS=windows TARGET_ARCH=arm64

help:
	@echo "Apack Project Build Tool"
	@echo ""
	@echo "Usage:"
	@echo "  make <target>"
	@echo ""
	@echo "Available targets:"
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)
	@echo ""
	@echo "Environment variables:"
	@echo "  TARGET_OS       Target operating system (default: linux)"
	@echo "  TARGET_ARCH     Target architecture (default: amd64)"
	@echo ""
	@echo "Examples:"
	@echo "  make build                    # Build default platform (linux/amd64)"
	@echo "  make build TARGET_OS=darwin   # Build macOS version"
	@echo "  make release                  # Cross-compile all platforms"
