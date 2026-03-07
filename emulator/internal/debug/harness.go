package debug

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go2daboy/emulator/internal"
)

// Harness wraps an emulator instance with debug instrumentation.
type Harness struct {
	Emu          *internal.Emulator
	SerialOutput strings.Builder

	frameCount   int
	snapshots    []Snapshot
	frameHashes  []string
	outputDir    string
}

// NewHarness creates a debug harness. outputDir is where PNGs and logs are saved.
// Pass "" to disable file output.
func NewHarness(outputDir string) *Harness {
	emu := internal.New()
	h := &Harness{
		Emu:       emu,
		outputDir: outputDir,
	}
	emu.Bus.SerialCallback = func(b byte) {
		h.SerialOutput.WriteByte(b)
	}
	return h
}

// LoadROM loads a ROM file into the harness.
func (h *Harness) LoadROM(path string) error {
	if err := h.Emu.LoadCartridge(path); err != nil {
		return err
	}
	h.Emu.Reset()
	// Re-attach serial callback after reset
	h.Emu.Bus.SerialCallback = func(b byte) {
		h.SerialOutput.WriteByte(b)
	}
	return nil
}

// LoadROMBytes loads ROM from bytes.
func (h *Harness) LoadROMBytes(data []byte) error {
	if err := h.Emu.LoadROM(data); err != nil {
		return err
	}
	h.Emu.Reset()
	h.Emu.Bus.SerialCallback = func(b byte) {
		h.SerialOutput.WriteByte(b)
	}
	return nil
}

// RunFrames runs N frames, capturing snapshots at specified intervals.
// snapshotEvery=0 means no snapshots. Returns the final framebuffer copy.
func (h *Harness) RunFrames(n int, snapshotEvery int) []uint8 {
	for i := 0; i < n; i++ {
		h.Emu.RunFrame()
		h.frameCount++

		hash := FrameHash(h.Emu.GetFrameBuffer())
		h.frameHashes = append(h.frameHashes, hash)

		if snapshotEvery > 0 && h.frameCount%snapshotEvery == 0 {
			h.snapshots = append(h.snapshots, h.TakeSnapshot())
		}
	}

	fb := make([]uint8, len(h.Emu.GetFrameBuffer()))
	copy(fb, h.Emu.GetFrameBuffer())
	return fb
}

// RunUntilSerial runs frames until the serial output contains the target string,
// or until maxFrames is reached. Returns whether the target was found.
func (h *Harness) RunUntilSerial(target string, maxFrames int) bool {
	for i := 0; i < maxFrames; i++ {
		h.Emu.RunFrame()
		h.frameCount++
		h.frameHashes = append(h.frameHashes, FrameHash(h.Emu.GetFrameBuffer()))

		if strings.Contains(h.SerialOutput.String(), target) {
			return true
		}
	}
	return false
}

// RunUntilStable runs frames until the framebuffer hash stabilizes
// (same hash for stableCount consecutive frames), or until maxFrames.
func (h *Harness) RunUntilStable(stableCount, maxFrames int) (bool, int) {
	var lastHash string
	consecutive := 0

	for i := 0; i < maxFrames; i++ {
		h.Emu.RunFrame()
		h.frameCount++
		hash := FrameHash(h.Emu.GetFrameBuffer())
		h.frameHashes = append(h.frameHashes, hash)

		if hash == lastHash {
			consecutive++
			if consecutive >= stableCount {
				return true, h.frameCount
			}
		} else {
			consecutive = 1
			lastHash = hash
		}
	}
	return false, h.frameCount
}

// TakeSnapshot captures current emulator state.
func (h *Harness) TakeSnapshot() Snapshot {
	emu := h.Emu
	return Snapshot{
		A: emu.CPU.Regs.A, F: emu.CPU.Regs.F,
		B: emu.CPU.Regs.B, C: emu.CPU.Regs.C,
		D: emu.CPU.Regs.D, E: emu.CPU.Regs.E,
		H: emu.CPU.Regs.H, L: emu.CPU.Regs.L,
		SP: emu.CPU.Regs.SP, PC: emu.CPU.Regs.PC,

		LCDC: emu.PPU.LCDC, STAT: emu.PPU.STAT,
		SCY: emu.PPU.SCY, SCX: emu.PPU.SCX,
		LY: emu.PPU.LY, LYC: emu.PPU.LYC,
		BGP: emu.PPU.BGP, OBP0: emu.PPU.OBP0, OBP1: emu.PPU.OBP1,
		WY: emu.PPU.WY, WX: emu.PPU.WX,
		PPUMode: emu.PPU.GetMode(),
		PPUModeClock: emu.PPU.GetModeClock(),
		WindowLine: emu.PPU.GetWindowLine(),

		DIV:  emu.Timer.Read(0xFF04),
		TIMA: emu.Timer.Read(0xFF05),
		TMA:  emu.Timer.Read(0xFF06),
		TAC:  emu.Timer.Read(0xFF07),

		IE: emu.Bus.GetIE(),
		IF: emu.Bus.GetIF(),

		FrameHash: FrameHash(emu.GetFrameBuffer()),
		Frame:     h.frameCount,
	}
}

// SaveFrame saves the current framebuffer as a PNG.
func (h *Harness) SaveFrame(name string) error {
	if h.outputDir == "" {
		return nil
	}
	if err := os.MkdirAll(h.outputDir, 0755); err != nil {
		return err
	}
	path := filepath.Join(h.outputDir, name+".png")
	return SaveFramePNG(h.Emu.GetFrameBuffer(), path)
}

// Report generates a full diagnostic report string.
func (h *Harness) Report() string {
	var b strings.Builder

	b.WriteString("=== Debug Harness Report ===\n")
	b.WriteString(fmt.Sprintf("Total frames run: %d\n", h.frameCount))
	b.WriteString(fmt.Sprintf("ROM: %s\n", h.Emu.GetCartridgeTitle()))
	b.WriteString(fmt.Sprintf("Serial output: %q\n\n", h.SerialOutput.String()))

	// Frame hash uniqueness
	hashSet := make(map[string]int)
	for _, hash := range h.frameHashes {
		hashSet[hash]++
	}
	b.WriteString(fmt.Sprintf("Unique frames: %d / %d total\n", len(hashSet), len(h.frameHashes)))

	// Current frame analysis
	fb := h.Emu.GetFrameBuffer()
	b.WriteString(fmt.Sprintf("Unique colors: %d\n", UniqueColors(fb)))

	blankRanges := BlankRowRanges(fb)
	if len(blankRanges) > 0 {
		b.WriteString("Blank row ranges:\n")
		for _, r := range blankRanges {
			b.WriteString(fmt.Sprintf("  rows %d-%d (%d rows)\n", r[0], r[1], r[1]-r[0]+1))
		}
	} else {
		b.WriteString("No blank row ranges (all rows have content)\n")
	}

	// Current snapshot
	b.WriteString("\n")
	snap := h.TakeSnapshot()
	b.WriteString(snap.String())

	// Tile map
	b.WriteString("\n")
	bgMapSelect := h.Emu.PPU.LCDC&0x08 != 0
	b.WriteString(TileMapString(h.Emu.PPU.VRAM[:], bgMapSelect, h.Emu.PPU.SCX, h.Emu.PPU.SCY))

	// Sprites
	b.WriteString("\n")
	b.WriteString(SpritesString(h.Emu.PPU.OAM[:], h.Emu.PPU.LCDC))

	return b.String()
}

// FrameCount returns the total number of frames run.
func (h *Harness) FrameCount() int {
	return h.frameCount
}

// Snapshots returns all captured snapshots.
func (h *Harness) Snapshots() []Snapshot {
	return h.snapshots
}

// FrameHashes returns all frame hashes in order.
func (h *Harness) FrameHashes() []string {
	return h.frameHashes
}
