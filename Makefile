.PHONY: download-specs test lint build

download-specs:
	./scripts/download-specs.sh

test:
	go test ./...

lint:
	golangci-lint run

build:
	go build ./...
