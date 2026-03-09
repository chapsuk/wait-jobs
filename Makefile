APP_NAME=wait-jobs

.PHONY: build test lint tidy

build:
	go build -o bin/$(APP_NAME) ./...

test:
	go test ./...

lint:
	go test ./...

tidy:
	go mod tidy
