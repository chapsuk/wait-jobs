APP_NAME=wait-jobs
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS=-X github.com/chapsuk/wait-jobs/internal/buildinfo.Version=$(VERSION) -X github.com/chapsuk/wait-jobs/internal/buildinfo.Commit=$(COMMIT) -X github.com/chapsuk/wait-jobs/internal/buildinfo.Date=$(DATE)

.PHONY: build test lint tidy

build:
	go build -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME) .

test:
	go test ./...

lint:
	go test ./...

tidy:
	go mod tidy
