GO ?= go
IMAGE ?= go-metin2-server

.PHONY: fmt test build build-authd build-gamed docker-build docker-build-debug

fmt:
	$(GO) fmt ./...

test:
	$(GO) test ./...

build: build-authd build-gamed

build-authd:
	mkdir -p bin
	$(GO) build -o bin/authd ./cmd/authd

build-gamed:
	mkdir -p bin
	$(GO) build -o bin/gamed ./cmd/gamed

docker-build:
	docker build --target runtime -t $(IMAGE):latest .

docker-build-debug:
	docker build --target runtime-debug -t $(IMAGE):debug .
