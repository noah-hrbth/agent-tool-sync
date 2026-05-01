BINARY := agentsync
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X main.version=$(VERSION)"

.PHONY: build test smoke sandbox-reset dev clean lint release-check release-snapshot

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/agentsync

test:
	go test ./...

smoke:
	go test -run TestSmoke ./internal/tui/...

sandbox-reset:
	bash scripts/reset-sandbox.sh

dev:
	go run $(LDFLAGS) ./cmd/agentsync --workspace ./examples/sandbox

clean:
	rm -f $(BINARY)
	rm -rf dist/

lint:
	go vet ./...

release-check:
	goreleaser check

release-snapshot:
	goreleaser release --snapshot --clean --skip=publish
