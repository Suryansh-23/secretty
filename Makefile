BINARY := secretty
CMD := ./cmd/secretty
GO_PACKAGES := ./cmd/... ./internal/...

.PHONY: build test lint fmt vet smoke

build:
	go build -o bin/$(BINARY) $(CMD)

test:
	go test $(GO_PACKAGES)

lint:
	golangci-lint run $(GO_PACKAGES)

vet:
	go vet $(GO_PACKAGES)

fmt:
	gofmt -w cmd internal

smoke: build
	./scripts/smoke.sh ./bin/$(BINARY)
