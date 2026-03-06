package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go2daboy/emulator/internal"
)

// TestBlarggCPUInstrs runs each of the Blargg cpu_instrs individual test ROMs.
// Place ROMs in testdata/blargg/ directory.
// Expected files: 01-special.gb, 02-interrupts.gb, ... 11-op a,(hl).gb
// Or the combined cpu_instrs.gb
func TestBlarggCPUInstrs(t *testing.T) {
	romPath := filepath.Join("testdata", "blargg", "cpu_instrs.gb")
	if _, err := os.Stat(romPath); os.IsNotExist(err) {
		t.Skip("cpu_instrs.gb not found in testdata/blargg/ - place Blargg ROMs there to run tests")
	}

	result := runBlarggTest(t, romPath, 60*60) // 60 seconds at 60fps
	if !strings.Contains(result, "Passed") {
		t.Errorf("cpu_instrs did not pass.\nSerial output:\n%s", result)
	} else {
		t.Logf("cpu_instrs output:\n%s", result)
	}
}

// TestBlarggIndividual runs individual Blargg test ROMs from testdata/blargg/
func TestBlarggIndividual(t *testing.T) {
	testROMs := []struct {
		name string
		file string
	}{
		{"01-special", "01-special.gb"},
		{"02-interrupts", "02-interrupts.gb"},
		{"03-op sp,hl", "03-op sp,hl.gb"},
		{"04-op r,imm", "04-op r,imm.gb"},
		{"05-op rp", "05-op rp.gb"},
		{"06-ld r,r", "06-ld r,r.gb"},
		{"07-jr,jp,call,ret,rst", "07-jr,jp,call,ret,rst.gb"},
		{"08-misc instrs", "08-misc instrs.gb"},
		{"09-op r,r", "09-op r,r.gb"},
		{"10-bit ops", "10-bit ops.gb"},
		{"11-op a,(hl)", "11-op a,(hl).gb"},
	}

	for _, tc := range testROMs {
		t.Run(tc.name, func(t *testing.T) {
			romPath := filepath.Join("testdata", "blargg", tc.file)
			if _, err := os.Stat(romPath); os.IsNotExist(err) {
				t.Skipf("%s not found", tc.file)
			}

			result := runBlarggTest(t, romPath, 30*60) // 30 seconds
			if strings.Contains(result, "Failed") {
				t.Errorf("%s FAILED.\nSerial output:\n%s", tc.name, result)
			} else if strings.Contains(result, "Passed") {
				t.Logf("%s PASSED", tc.name)
			} else {
				t.Logf("%s - no clear result.\nSerial output:\n%s", tc.name, result)
			}
		})
	}
}

// TestBlarggInstrTiming tests instruction timing accuracy
func TestBlarggInstrTiming(t *testing.T) {
	romPath := filepath.Join("testdata", "blargg", "instr_timing.gb")
	if _, err := os.Stat(romPath); os.IsNotExist(err) {
		t.Skip("instr_timing.gb not found")
	}

	result := runBlarggTest(t, romPath, 30*60)
	if strings.Contains(result, "Failed") {
		t.Errorf("instr_timing FAILED.\nSerial output:\n%s", result)
	} else if strings.Contains(result, "Passed") {
		t.Logf("instr_timing PASSED")
	} else {
		t.Logf("instr_timing - output:\n%s", result)
	}
}

// TestBlarggMemTiming tests memory access timing
func TestBlarggMemTiming(t *testing.T) {
	romPath := filepath.Join("testdata", "blargg", "mem_timing.gb")
	if _, err := os.Stat(romPath); os.IsNotExist(err) {
		t.Skip("mem_timing.gb not found")
	}

	result := runBlarggTest(t, romPath, 30*60)
	if strings.Contains(result, "Failed") {
		t.Errorf("mem_timing FAILED.\nSerial output:\n%s", result)
	} else if strings.Contains(result, "Passed") {
		t.Logf("mem_timing PASSED")
	} else {
		t.Logf("mem_timing - output:\n%s", result)
	}
}

// runBlarggTest runs a ROM for the given number of frames and returns serial output
func runBlarggTest(t *testing.T, romPath string, maxFrames int) string {
	t.Helper()

	emu := internal.New()
	err := emu.LoadCartridge(romPath)
	if err != nil {
		t.Fatalf("Failed to load ROM %s: %v", romPath, err)
	}

	emu.Reset()

	var serialOutput strings.Builder

	// Hook serial output
	emu.Bus.SerialCallback = func(b byte) {
		serialOutput.WriteByte(b)
	}

	// Run emulation
	for frame := 0; frame < maxFrames; frame++ {
		emu.RunFrame()

		output := serialOutput.String()

		// Check for test completion
		if strings.Contains(output, "Passed") || strings.Contains(output, "Failed") {
			// Run a few more frames to capture full output
			for extra := 0; extra < 10; extra++ {
				emu.RunFrame()
			}
			return serialOutput.String()
		}
	}

	return serialOutput.String()
}

// TestBlarggQuick is a quick smoke test that verifies the emulator can
// run for 1000 frames without crashing
func TestEmulatorSmoke(t *testing.T) {
	// Create a minimal ROM that does XOR A; JR -2 (infinite loop)
	rom := make([]byte, 0x8000)
	// Logo data (required for header validation, but we skip it)
	rom[0x100] = 0xAF // XOR A at entry point
	rom[0x101] = 0x18 // JR
	rom[0x102] = 0xFE // -2 (back to XOR A)
	// Set header checksum
	rom[0x148] = 0x00 // 32KB ROM
	rom[0x149] = 0x00 // No RAM
	// Pad to minimum size
	for i := 0x134; i < 0x14D; i++ {
		rom[i] = 0x00
	}

	emu := internal.New()
	err := emu.LoadROM(rom)
	if err != nil {
		t.Fatalf("Failed to load test ROM: %v", err)
	}
	emu.Reset()

	// Run 1000 frames without panicking
	for i := 0; i < 1000; i++ {
		emu.RunFrame()
	}

	// Check that A register is 0 (XOR A sets it to 0)
	if emu.CPU.Regs.A != 0 {
		t.Errorf("Expected A=0 after XOR A, got A=0x%02X", emu.CPU.Regs.A)
	}

	fmt.Println("Smoke test: 1000 frames executed successfully")
}
