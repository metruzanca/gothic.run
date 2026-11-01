.PHONY: docs generate test run build

docs:
	./scripts/sync-docs.sh

generate:
	go generate ./...

test: generate docs
	go test ./...

run: generate docs
	go run .

build: generate
	go build -o gothic.run .
