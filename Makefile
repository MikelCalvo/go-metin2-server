GO ?= go
IMAGE ?= go-metin2-server

# Prefer CI-provided identity when present so lab retention trees keyed by
# <commit12> match the GitHub Actions checkout even on shallow/detached SHAs.
# FreeBSD/bmake-compatible `!=` shell assignments (not GNU Make $(shell ...)).
VERSION != if [ -n "$${GITHUB_REF_NAME:-}" ]; then printf '%s' "$${GITHUB_REF_NAME}"; else git describe --tags --always --dirty 2>/dev/null || echo dev; fi
COMMIT != if [ -n "$${GITHUB_SHA:-}" ]; then printf '%s' "$${GITHUB_SHA}" | cut -c1-12; else git rev-parse --short=12 HEAD 2>/dev/null || echo none; fi
BUILD_DATE != date -u +%Y-%m-%dT%H:%M:%SZ
BUILDINFO_PKG = github.com/MikelCalvo/go-metin2-server/internal/buildinfo
LDFLAGS = -X $(BUILDINFO_PKG).Version=$(VERSION) -X $(BUILDINFO_PKG).Commit=$(COMMIT) -X $(BUILDINFO_PKG).BuildDate=$(BUILD_DATE)

.PHONY: fmt test build build-authd build-gamed build-metin2-migrate docker-build docker-build-debug

fmt:
	$(GO) fmt ./...

test:
	$(GO) test ./...

build: build-authd build-gamed build-metin2-migrate

build-authd:
	mkdir -p bin
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/authd ./cmd/authd

build-gamed:
	mkdir -p bin
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/gamed ./cmd/gamed

build-metin2-migrate:
	mkdir -p bin
	$(GO) build -ldflags "$(LDFLAGS)" -o bin/metin2-migrate ./cmd/metin2-migrate

docker-build:
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_DATE=$(BUILD_DATE) --target runtime -t $(IMAGE):latest .

docker-build-debug:
	docker build --build-arg VERSION=$(VERSION) --build-arg COMMIT=$(COMMIT) --build-arg BUILD_DATE=$(BUILD_DATE) --target runtime-debug -t $(IMAGE):debug .
