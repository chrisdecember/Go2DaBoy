.PHONY: build clean serve

WASM_EXEC := $(shell GOROOT= go env GOROOT)/lib/wasm/wasm_exec.js

build:
	@mkdir -p build
	@echo "Building WASM..."
	GOOS=js GOARCH=wasm go build -o build/main.wasm ./emulator/wasm/
	@cp $(WASM_EXEC) build/wasm_exec.js
	@echo "Build complete: build/main.wasm"

serve: build
	@echo "Starting server on http://localhost:8080"
	go run ./emulator/cmd/ -serve

clean:
	rm -rf build/
