.PHONY: build-cgo test-cgo clean

build-cgo:
	cd src/gosim/cgo && \
	go build -buildmode=c-shared -o ../../../libcardsim.so bridge.go

test-cgo: build-cgo
	uv run pytest tests/integration/test_cgo_bridge.py -v

clean:
	rm -f libcardsim.so libcardsim.h
