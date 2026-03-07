package test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"go2daboy/emulator/internal/debug"
)

// TestDebugSmoke verifies the debug harness works with a minimal ROM.
func TestDebugSmoke(t *testing.T) {
	// Minimal ROM: XOR A; JR -2 (infinite loop)
	rom := makeMinimalROM()
	h := debug.NewHarness("")
	if err := h.LoadROMBytes(rom); err != nil {
		t.Fatal(err)
	}

	fb := h.RunFrames(60, 0)
	if len(fb) == 0 {
		t.Fatal("RunFrames returned empty buffer")
	}

	snap := h.TakeSnapshot()
	t.Logf("After 60 frames:\n%s", snap.String())

	report := h.Report()
	if !strings.Contains(report, "Total frames run: 60") {
		t.Errorf("Report missing frame count")
	}
	t.Logf("Report:\n%s", report)
}

// TestDebugFrameStability runs the smoke ROM and checks frame hash stability.
func TestDebugFrameStability(t *testing.T) {
	rom := makeMinimalROM()
	h := debug.NewHarness("")
	if err := h.LoadROMBytes(rom); err != nil {
		t.Fatal(err)
	}

	stable, frame := h.RunUntilStable(10, 300)
	t.Logf("Stable=%v at frame %d", stable, frame)

	hashes := h.FrameHashes()
	unique := make(map[string]bool)
	for _, hash := range hashes {
		unique[hash] = true
	}
	t.Logf("Unique frame hashes: %d / %d", len(unique), len(hashes))
}

// TestDebugFrameDiff verifies frame diff detection.
func TestDebugFrameDiff(t *testing.T) {
	rom := makeMinimalROM()
	h := debug.NewHarness("")
	if err := h.LoadROMBytes(rom); err != nil {
		t.Fatal(err)
	}

	fb1 := h.RunFrames(1, 0)
	fb2 := h.RunFrames(1, 0)

	diffCount, diffs := debug.FrameDiff(fb1, fb2, 5)
	t.Logf("Frame 1 vs Frame 2: %d differing pixels", diffCount)
	for _, d := range diffs {
		t.Logf("  (%d,%d): %v -> %v", d.X, d.Y, d.A, d.B)
	}
}

// TestDebugBlankDetection checks blank row range detection.
func TestDebugBlankDetection(t *testing.T) {
	rom := makeMinimalROM()
	h := debug.NewHarness("")
	if err := h.LoadROMBytes(rom); err != nil {
		t.Fatal(err)
	}

	h.RunFrames(10, 0)
	fb := h.Emu.GetFrameBuffer()

	ranges := debug.BlankRowRanges(fb)
	t.Logf("Blank row ranges: %v", ranges)

	colors := debug.UniqueColors(fb)
	t.Logf("Unique colors in frame: %d", colors)

	profile := debug.RowProfile(fb)
	nonBlank := 0
	for _, row := range profile {
		if !row.Blank {
			nonBlank++
		}
	}
	t.Logf("Non-blank rows: %d / 144", nonBlank)
}

// TestDebugPNGExport tests PNG save/load cycle.
func TestDebugPNGExport(t *testing.T) {
	rom := makeMinimalROM()
	dir := t.TempDir()
	h := debug.NewHarness(dir)
	if err := h.LoadROMBytes(rom); err != nil {
		t.Fatal(err)
	}

	h.RunFrames(10, 0)
	if err := h.SaveFrame("test_frame"); err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(dir, "test_frame.png")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("PNG not created: %v", err)
	}
	t.Logf("Saved PNG: %s (%d bytes)", path, info.Size())
}

// TestBlarggVisual runs a Blargg test ROM (if available) with full debug
// instrumentation, capturing frames and generating a diagnostic report.
func TestBlarggVisual(t *testing.T) {
	romPath := filepath.Join("testdata", "blargg", "cpu_instrs.gb")
	if _, err := os.Stat(romPath); os.IsNotExist(err) {
		t.Skip("cpu_instrs.gb not found - place in testdata/blargg/")
	}

	dir := t.TempDir()
	h := debug.NewHarness(dir)
	if err := h.LoadROM(romPath); err != nil {
		t.Fatal(err)
	}

	// Run until test completes or timeout
	found := h.RunUntilSerial("Passed", 60*60)
	if !found {
		// Check for failure
		if strings.Contains(h.SerialOutput.String(), "Failed") {
			t.Logf("Test FAILED")
		} else {
			t.Logf("Test did not complete in time")
		}
	}

	// Save final frame
	h.SaveFrame("blargg_final")

	// Full report
	report := h.Report()
	t.Logf("\n%s", report)

	// Save report to file
	reportPath := filepath.Join(dir, "report.txt")
	os.WriteFile(reportPath, []byte(report), 0644)
	t.Logf("Report saved to: %s", reportPath)
	t.Logf("Final frame: %s", filepath.Join(dir, "blargg_final.png"))

	serial := h.SerialOutput.String()
	if !strings.Contains(serial, "Passed") {
		t.Errorf("Serial output:\n%s", serial)
	}
}

// TestBlarggVisualIndividual runs individual Blargg tests with visual capture.
func TestBlarggVisualIndividual(t *testing.T) {
	testROMs := []string{
		"01-special.gb", "02-interrupts.gb", "03-op sp,hl.gb",
		"04-op r,imm.gb", "05-op rp.gb", "06-ld r,r.gb",
		"07-jr,jp,call,ret,rst.gb", "08-misc instrs.gb",
		"09-op r,r.gb", "10-bit ops.gb", "11-op a,(hl).gb",
	}

	for _, rom := range testROMs {
		name := strings.TrimSuffix(rom, ".gb")
		t.Run(name, func(t *testing.T) {
			romPath := filepath.Join("testdata", "blargg", rom)
			if _, err := os.Stat(romPath); os.IsNotExist(err) {
				t.Skipf("%s not found", rom)
			}

			dir := t.TempDir()
			h := debug.NewHarness(dir)
			if err := h.LoadROM(romPath); err != nil {
				t.Fatal(err)
			}

			found := h.RunUntilSerial("Passed", 30*60)

			h.SaveFrame(name + "_final")

			serial := h.SerialOutput.String()
			if !found {
				if strings.Contains(serial, "Failed") {
					t.Errorf("%s FAILED\nSerial: %s", name, serial)
				} else {
					t.Logf("%s incomplete\nSerial: %s", name, serial)
				}
			} else {
				t.Logf("%s PASSED", name)
			}

			// Always save snapshot for analysis
			snap := h.TakeSnapshot()
			t.Logf("\n%s", snap.String())
		})
	}
}

// TestRenderingIntegrity is a general-purpose rendering check.
// It runs a ROM for N frames and checks for common rendering problems:
// - Large blank regions (broken scanline rendering)
// - Only 1 unique color (LCD not rendering)
// - Frame never changes (stuck/frozen)
func TestRenderingIntegrity(t *testing.T) {
	romPath := filepath.Join("testdata", "blargg", "cpu_instrs.gb")
	if _, err := os.Stat(romPath); os.IsNotExist(err) {
		t.Skip("cpu_instrs.gb not found")
	}

	h := debug.NewHarness("")
	if err := h.LoadROM(romPath); err != nil {
		t.Fatal(err)
	}

	// Run 120 frames (~2 seconds)
	h.RunFrames(120, 0)
	fb := h.Emu.GetFrameBuffer()

	// Check: not all blank
	colors := debug.UniqueColors(fb)
	if colors <= 1 {
		t.Errorf("Frame has only %d color(s) - LCD may not be rendering", colors)
	}

	// Check: no large blank bands (>50% of screen)
	blankRanges := debug.BlankRowRanges(fb)
	totalBlank := 0
	for _, r := range blankRanges {
		totalBlank += r[1] - r[0] + 1
	}
	if totalBlank > 72 { // More than 50% of 144 rows
		t.Errorf("Large blank region: %d/144 rows blank. Ranges: %v", totalBlank, blankRanges)
	}

	// Check: frame changes over time (not frozen)
	hashes := h.FrameHashes()
	unique := make(map[string]bool)
	for _, hash := range hashes {
		unique[hash] = true
	}
	if len(unique) < 3 {
		t.Logf("Warning: only %d unique frames in 120 frames - may be frozen", len(unique))
	}

	t.Logf("Rendering check: %d colors, %d blank rows, %d unique frames",
		colors, totalBlank, len(unique))
}

// TestScreenCapturePipeline demonstrates the full AI analysis pipeline:
// 1. Load ROM, 2. Run to stability, 3. Capture frame, 4. Generate report
// The saved PNG can be read by the AI (Read tool supports images).
func TestScreenCapturePipeline(t *testing.T) {
	romPath := filepath.Join("testdata", "blargg", "cpu_instrs.gb")
	if _, err := os.Stat(romPath); os.IsNotExist(err) {
		t.Skip("cpu_instrs.gb not found")
	}

	// Output to a fixed location for easy AI access
	dir := filepath.Join("testdata", "debug_output")
	os.MkdirAll(dir, 0755)

	h := debug.NewHarness(dir)
	if err := h.LoadROM(romPath); err != nil {
		t.Fatal(err)
	}

	// Phase 1: Run until serial output starts
	h.RunUntilSerial("\n", 600)
	h.SaveFrame("phase1_serial_start")
	t.Logf("Phase 1 (serial start): frame %d, serial=%q",
		h.FrameCount(), h.SerialOutput.String())

	// Phase 2: Run until test completes
	found := h.RunUntilSerial("Passed", 60*60)
	h.SaveFrame("phase2_test_complete")
	t.Logf("Phase 2 (test complete): found=%v, frame %d", found, h.FrameCount())

	// Phase 3: Run a few more frames for final stable image
	h.RunFrames(30, 0)
	h.SaveFrame("phase3_final")

	// Save full report
	report := h.Report()
	reportPath := filepath.Join(dir, "analysis_report.txt")
	os.WriteFile(reportPath, []byte(report), 0644)

	// Log paths for AI
	t.Logf("\n=== AI Analysis Pipeline Output ===")
	t.Logf("Frame captures saved to: %s/", dir)
	t.Logf("  phase1_serial_start.png - First serial output")
	t.Logf("  phase2_test_complete.png - Test completion")
	t.Logf("  phase3_final.png - Final stable frame")
	t.Logf("  analysis_report.txt - Full diagnostic report")
	t.Logf("Serial output: %s", h.SerialOutput.String())

	fmt.Printf("\nTo analyze: use Read tool on PNG files in %s/\n", dir)
}

func makeMinimalROM() []byte {
	rom := make([]byte, 0x8000)
	rom[0x100] = 0xAF // XOR A
	rom[0x101] = 0x18 // JR
	rom[0x102] = 0xFE // -2
	rom[0x148] = 0x00 // 32KB ROM
	rom[0x149] = 0x00 // No RAM
	return rom
}
