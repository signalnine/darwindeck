.PHONY: build-cgo test-cgo build-worker clean

build-cgo:
	cd src/gosim/cgo && \
	go build -buildmode=c-shared -o ../../../libcardsim.so bridge.go

build-worker:
	mkdir -p bin
	cd src/gosim && go build -o ../../bin/gosim-worker ./cmd/worker

test-cgo: build-cgo
	uv run pytest tests/integration/test_cgo_bridge.py -v

clean:
	rm -f libcardsim.so libcardsim.h bin/gosim-worker
