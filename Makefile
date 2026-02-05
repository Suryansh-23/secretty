BINARY := secretty
CMD := ./cmd/secretty
GO_PACKAGES := $(shell go list -f '{{.ImportPath}}' ./... 2>/dev/null | grep -v '/context/' || true)
GO_DIRS := $(shell go list -f '{{.Dir}}' ./... 2>/dev/null | sed 's|^$(CURDIR)/||' | grep -v '^context/' || true)

.PHONY: build test lint fmt smoke vet

build:
	go build -o bin/$(BINARY) $(CMD)

test:
	go test $(GO_PACKAGES)

lint:
	golangci-lint run $(GO_DIRS)

vet:
	go vet $(GO_PACKAGES)

fmt:
	gofmt -w cmd internal

smoke: build
	./scripts/smoke.sh ./bin/$(BINARY)
