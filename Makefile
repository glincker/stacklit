BINARY    := stacklit
CMD       := ./cmd/stacklit
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed "s/^v//" || echo "dev")
LDFLAGS   := -ldflags "-X github.com/glincker/stacklit/internal/cli.Version=$(VERSION)"

.PHONY: build run test clean install bench

install:
	go install $(LDFLAGS) $(CMD)

build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

run: build
	./$(BINARY)

test:
	go test ./...

# Reproducible retrieval benchmark: clones pinned repos to a temp dir, needs git and network.
bench:
	go run ./bench -write bench/RESULTS.md

clean:
	rm -f $(BINARY)
	rm -rf dist/
