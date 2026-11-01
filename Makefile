.PHONY: docs generate test run build

docs:
	./scripts/sync-docs.sh

generate:
	go generate ./...

test: docs
	go test ./...

run: docs
	go run .

build:
	go build -o gothic.run .
