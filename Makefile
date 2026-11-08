BINARY    := stacklit
CMD       := ./cmd/stacklit
VERSION   ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed "s/^v//" || echo "dev")
LDFLAGS   := -ldflags "-X github.com/glincker/stacklit/internal/cli.Version=$(VERSION)"

.PHONY: build run test clean install scale

install:
	go install $(LDFLAGS) $(CMD)

build:
	go build $(LDFLAGS) -o $(BINARY) $(CMD)

run: build
	./$(BINARY)

test:
	go test ./...

clean:
	rm -f $(BINARY)
	rm -rf dist/

# Scale metrics on big public repos. Slow (hours) and needs network and ~4 GB of temp disk.
# Use ONLY=django,react to pick repos; results land in bench/SCALE.md.
scale:
	go run ./bench/scale $(if $(ONLY),-only $(ONLY),) -json bench/scale/results.json -write bench/SCALE.md
