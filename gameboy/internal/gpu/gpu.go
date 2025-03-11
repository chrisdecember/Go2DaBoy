package gpu

import "yukudanshi/gameboy/internal/memory"

// Mode represents the current GPU mode
type Mode uint8

const (
	HBlank  Mode = 0
	VBlank  Mode = 1
	OAMScan Mode = 2
	Drawing Mode = 3
)

// GPU represents the Game Boy's Graphics Processing Unit
type GPU struct {
	Memory *memory.MemoryMap
	vram   [0x2000]uint8
	oam    [0xA0]uint8

	// LCD Control Register (0xFF40)
	lcdControl uint8

	// LCD Status Register (0xFF41)
	lcdStatus uint8

	// Scroll coordinates
	scrollX uint8
	scrollY uint8

	// Window position
	windowX uint8
	windowY uint8

	// LCD Y Coordinate (0xFF44)
	ly uint8

	// LY Compare (0xFF45)
	lyCompare uint8

	// Background Palette (0xFF47)
	bgPalette uint8

	// Object Palettes (0xFF48-0xFF49)
	objPalette0 uint8
	objPalette1 uint8

	// Internal counters
	mode      Mode
	modeClock int

	// Screen buffer (160x144 pixels)
	screen [160][144]uint8
}

// New creates a new GPU instance
func New(memory *memory.MemoryMap) *GPU {
	return &GPU{
		Memory: memory,
	}
}

// Reset resets the GPU to its initial state
func (g *GPU) Reset() {
	g.lcdControl = 0x91
	g.lcdStatus = 0
	g.scrollX = 0
	g.scrollY = 0
	g.ly = 0
	g.lyCompare = 0
	g.bgPalette = 0xFC
	g.objPalette0 = 0xFF
	g.objPalette1 = 0xFF
	g.windowX = 0
	g.windowY = 0
	g.mode = OAMScan
	g.modeClock = 0
}

// Step advances the GPU by the specified number of cycles
func (g *GPU) Step(cycles int) {
	// We'll implement this later with timing logic
}

// GetScreen returns the current screen buffer
func (g *GPU) GetScreen() [160][144]uint8 {
	return g.screen
}
