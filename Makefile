.PHONY: build-cgo test-cgo build-worker build-evolve clean

# Build version info
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

build-cgo:
	cd src/gosim/cgo && \
	go build -buildmode=c-shared -o ../../../libcardsim.so bridge.go

build-worker:
	mkdir -p bin
	cd src/gosim && go build -o ../../bin/gosim-worker ./cmd/worker

build-evolve:
	mkdir -p bin
	cd src/gosim && go build -ldflags "$(LDFLAGS)" -o ../../bin/darwindeck-evolve ./cmd/evolve

test-cgo: build-cgo
	uv run pytest tests/integration/test_cgo_bridge.py -v

clean:
	rm -f libcardsim.so libcardsim.h bin/gosim-worker bin/darwindeck-evolve
