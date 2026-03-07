package test

import (
	"fmt"
	"testing"

	"go2daboy/emulator/internal"
	"go2daboy/emulator/internal/debug"
)

// TestROMBuilderExecution verifies the generated ROM bytes and execution.
func TestROMBuilderExecution(t *testing.T) {
	// Build the simplest possible ROM: just write 0xFF to VRAM 0x8000
	r := debug.NewROMBuilder()
	r.DI()                     // F3
	r.LDAImm(0xFF)             // 3E FF
	r.LDAddr16A(0x8000)        // EA 00 80
	r.InfiniteLoop()           // 18 FE

	rom := r.Build()

	// Dump bytes at 0x100
	t.Logf("ROM bytes at 0x100: % 02X", rom[0x100:0x110])

	emu := internal.New()
	if err := emu.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	emu.Reset()

	// Check initial state
	t.Logf("Initial PC=%04X A=%02X LCDC=%02X", emu.CPU.Regs.PC, emu.CPU.Regs.A, emu.PPU.LCDC)
	t.Logf("VRAM[0]=%02X", emu.PPU.VRAM[0])

	// Step a few instructions
	for i := 0; i < 20; i++ {
		pc := emu.CPU.Regs.PC
		cycles := emu.Step()
		t.Logf("Step %d: PC=%04X->%04X A=%02X cycles=%d VRAM[0]=%02X",
			i, pc, emu.CPU.Regs.PC, emu.CPU.Regs.A, cycles, emu.PPU.VRAM[0])
	}
}

// TestROMBuilderFillVRAM tests FillVRAM generates correct code.
func TestROMBuilderFillVRAM(t *testing.T) {
	r := debug.NewROMBuilder()
	r.DI()
	r.WaitVBlank()
	r.DisableLCD()
	r.FillVRAM(0x0000, 0xFF, 16) // fill tile 0 with 0xFF
	r.InfiniteLoop()

	rom := r.Build()
	t.Logf("ROM bytes at 0x100: % 02X", rom[0x100:0x130])

	emu := internal.New()
	if err := emu.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	emu.Reset()

	// Run for a frame
	emu.RunFrame()

	t.Logf("After 1 frame: VRAM[0..15]: % 02X", emu.PPU.VRAM[0:16])
	t.Logf("PC=%04X HL=%04X BC=%04X", emu.CPU.Regs.PC,
		uint16(emu.CPU.Regs.H)<<8|uint16(emu.CPU.Regs.L),
		uint16(emu.CPU.Regs.B)<<8|uint16(emu.CPU.Regs.C))

	// Check VRAM was written
	allFF := true
	for i := 0; i < 16; i++ {
		if emu.PPU.VRAM[i] != 0xFF {
			allFF = false
			break
		}
	}
	if !allFF {
		t.Error("VRAM[0..15] should all be 0xFF")
	}
}

// TestROMBuilderDisableLCD tests that DisableLCD produces working code.
func TestROMBuilderDisableLCD(t *testing.T) {
	r := debug.NewROMBuilder()
	r.DI()

	// Wait for VBlank then disable LCD
	r.WaitVBlank()
	r.DisableLCD()
	r.InfiniteLoop()

	rom := r.Build()
	t.Logf("ROM at 0x100: % 02X", rom[0x100:0x120])

	emu := internal.New()
	if err := emu.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	emu.Reset()

	// Run a few frames — should eventually reach VBlank and disable LCD
	for i := 0; i < 5; i++ {
		emu.RunFrame()
	}

	lcdOn := emu.PPU.LCDC&0x80 != 0
	t.Logf("After 5 frames: LCDC=%02X LCD on=%v PC=%04X", emu.PPU.LCDC, lcdOn, emu.CPU.Regs.PC)

	if lcdOn {
		t.Error("LCD should be disabled after WaitVBlank + DisableLCD")
	}
}

func TestROMSolidFillDebug(t *testing.T) {
	rom := buildSolidFillROM()

	// Dump the ROM entry point
	t.Logf("ROM at 0x100: % 02X", rom[0x100:0x140])

	emu := internal.New()
	if err := emu.LoadROM(rom); err != nil {
		t.Fatal(err)
	}
	emu.Reset()

	// Run several frames
	for frame := 0; frame < 10; frame++ {
		emu.RunFrame()
	}

	t.Logf("After 10 frames:")
	t.Logf("  PC=%04X LCDC=%02X LY=%d", emu.CPU.Regs.PC, emu.PPU.LCDC, emu.PPU.LY)
	t.Logf("  VRAM[0..15]: % 02X", emu.PPU.VRAM[0:16])
	t.Logf("  BGP=%02X SCX=%d SCY=%d", emu.PPU.BGP, emu.PPU.SCX, emu.PPU.SCY)

	// Check framebuffer
	fb := emu.GetFrameBuffer()
	t.Logf("  FB[0..3]: %v (pixel 0,0)", fb[0:4])

	colors := debug.UniqueColors(fb)
	t.Logf("  Unique colors: %d", colors)
	fmt.Printf("  Blank ranges: %v\n", debug.BlankRowRanges(fb))
}
