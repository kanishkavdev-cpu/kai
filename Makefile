.PHONY: build run test

build:
	go build -o bin/kai ./cmd/kai

run:
	go run ./cmd/kai

test:
	go test ./...
