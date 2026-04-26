BINARY := agentsync
MODULE  := github.com/noah-hrbth/agentsync
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-X $(MODULE)/cmd/agentsync.version=$(VERSION)"

.PHONY: build test smoke sandbox-reset dev clean lint

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
