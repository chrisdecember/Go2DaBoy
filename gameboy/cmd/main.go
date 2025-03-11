package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"yukudanshi/gameboy/internal"
)

func main() {
	romFile := flag.String("rom", "", "Path to ROM file")
	flag.Parse()

	if *romFile == "" {
		fmt.Println("Please specify a ROM file with -rom flag")
		os.Exit(1)
	}

	emu := internal.New()
	err := emu.LoadCartridge(*romFile)
	if err != nil {
		log.Fatalf("Failed to load ROM: %v", err)
	}

	fmt.Println("Starting Game Boy emulator...")
	emu.Reset()
	emu.Run()
}
