BINARY := secretty
CMD := ./cmd/secretty

.PHONY: build test lint fmt smoke

build:
	go build -o bin/$(BINARY) $(CMD)

test:
	go test ./...

lint:
	golangci-lint run

fmt:
	gofmt -w cmd internal

smoke: build
	./scripts/smoke.sh ./bin/$(BINARY)
