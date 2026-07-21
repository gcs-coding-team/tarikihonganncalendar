.PHONY: run build test vet

run:
	go run ./cmd/api

build:
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker

test:
	go test ./...

vet:
	go vet ./...
