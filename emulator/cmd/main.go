package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	romFile := flag.String("rom", "", "Path to ROM file (for headless testing)")
	serve := flag.Bool("serve", false, "Start web server for the emulator")
	port := flag.String("port", "8080", "Port to serve on")
	flag.Parse()

	if *serve {
		startServer(*port)
		return
	}

	if *romFile == "" {
		fmt.Println("Go2DaBoy - retro handheld emulator")
		fmt.Println()
		fmt.Println("Usage:")
		fmt.Println("  go2daboy -serve              Start web server on port 8080")
		fmt.Println("  go2daboy -serve -port 3000   Start web server on port 3000")
		fmt.Println("  go2daboy -rom game.gb        Load ROM (headless test)")
		os.Exit(0)
	}

	// Headless test mode - just verify ROM loads
	data, err := os.ReadFile(*romFile)
	if err != nil {
		log.Fatalf("Failed to read ROM: %v", err)
	}

	fmt.Printf("ROM loaded: %d bytes\n", len(data))
	fmt.Printf("Title bytes: %s\n", string(data[0x134:0x143]))
	fmt.Printf("Cartridge type: 0x%02X\n", data[0x147])
	fmt.Printf("ROM size code: 0x%02X\n", data[0x148])
	fmt.Printf("RAM size code: 0x%02X\n", data[0x149])
}

func startServer(port string) {
	// Find the web directory
	webDir := findWebDir()
	wasmDir := findWasmDir()

	mux := http.NewServeMux()

	// Serve WASM file
	mux.HandleFunc("/main.wasm", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/wasm")
		http.ServeFile(w, r, filepath.Join(wasmDir, "main.wasm"))
	})

	// Serve wasm_exec.js from Go installation
	mux.HandleFunc("/wasm_exec.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript")
		http.ServeFile(w, r, filepath.Join(wasmDir, "wasm_exec.js"))
	})

	// Serve static files
	mux.Handle("/", http.FileServer(http.Dir(webDir)))

	fmt.Printf("Starting Go2DaBoy server on http://localhost:%s\n", port)
	fmt.Println("Open this URL on your phone or browser to play!")
	log.Fatal(http.ListenAndServe(":"+port, mux))
}

func findWebDir() string {
	// Try relative paths
	candidates := []string{"web", "../web", "../../web"}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	log.Fatal("Cannot find web directory. Run from project root.")
	return ""
}

func findWasmDir() string {
	candidates := []string{"build", "../build", "../../build"}
	for _, c := range candidates {
		if info, err := os.Stat(c); err == nil && info.IsDir() {
			abs, _ := filepath.Abs(c)
			return abs
		}
	}
	log.Fatal("Cannot find build directory. Run 'make build' first.")
	return ""
}
