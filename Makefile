.PHONY: build clean serve

GOROOT := $(shell go env GOROOT)
WASM_EXEC := $(GOROOT)/lib/wasm/wasm_exec.js

build:
	@mkdir -p build
	@echo "Building WASM..."
	GOOS=js GOARCH=wasm go build -o build/main.wasm ./gameboy/wasm/
	@cp $(WASM_EXEC) build/wasm_exec.js
	@echo "Build complete: build/main.wasm"

serve: build
	@echo "Starting server on http://localhost:8080"
	go run ./gameboy/cmd/ -serve

clean:
	rm -rf build/
