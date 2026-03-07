package debug

import (
	"fmt"
	"strings"

	"go2daboy/emulator/internal/ppu"
)

// Snapshot captures the full emulator state at a point in time.
type Snapshot struct {
	// CPU registers
	A, F, B, C, D, E, H, L uint8
	SP, PC                  uint16
	IME                     bool

	// PPU state
	LCDC, STAT, SCY, SCX   uint8
	LY, LYC                uint8
	BGP, OBP0, OBP1        uint8
	WY, WX                 uint8
	PPUMode                uint8
	PPUModeClock           int
	WindowLine             int

	// Timer state
	DIV              uint8
	TIMA, TMA, TAC   uint8

	// Interrupt registers
	IE, IF uint8

	// Frame hash
	FrameHash string

	// Frame number when snapshot was taken
	Frame int
}

// String returns a human-readable dump of the snapshot.
func (s *Snapshot) String() string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("=== Snapshot (Frame %d) ===\n", s.Frame))

	// CPU
	b.WriteString(fmt.Sprintf("CPU: A=%02X F=%02X B=%02X C=%02X D=%02X E=%02X H=%02X L=%02X\n",
		s.A, s.F, s.B, s.C, s.D, s.E, s.H, s.L))
	b.WriteString(fmt.Sprintf("     SP=%04X PC=%04X IME=%v\n", s.SP, s.PC, s.IME))

	// Flags
	flags := ""
	if s.F&0x80 != 0 {
		flags += "Z"
	} else {
		flags += "-"
	}
	if s.F&0x40 != 0 {
		flags += "N"
	} else {
		flags += "-"
	}
	if s.F&0x20 != 0 {
		flags += "H"
	} else {
		flags += "-"
	}
	if s.F&0x10 != 0 {
		flags += "C"
	} else {
		flags += "-"
	}
	b.WriteString(fmt.Sprintf("     Flags: %s\n", flags))

	// PPU
	modeNames := [4]string{"HBlank", "VBlank", "OAM", "Transfer"}
	modeName := "Unknown"
	if s.PPUMode < 4 {
		modeName = modeNames[s.PPUMode]
	}
	b.WriteString(fmt.Sprintf("PPU: LCDC=%02X STAT=%02X Mode=%s(%d) Clock=%d\n",
		s.LCDC, s.STAT, modeName, s.PPUMode, s.PPUModeClock))
	b.WriteString(fmt.Sprintf("     LY=%d LYC=%d SCX=%d SCY=%d WX=%d WY=%d\n",
		s.LY, s.LYC, s.SCX, s.SCY, s.WX, s.WY))
	b.WriteString(fmt.Sprintf("     BGP=%02X OBP0=%02X OBP1=%02X WindowLine=%d\n",
		s.BGP, s.OBP0, s.OBP1, s.WindowLine))

	// LCDC bit breakdown
	b.WriteString(fmt.Sprintf("     LCDC bits: LCD=%v BG=%v OBJ=%v OBJSize=%v BGMap=%v TileData=%v WIN=%v WINMap=%v\n",
		s.LCDC&0x80 != 0, s.LCDC&0x01 != 0, s.LCDC&0x02 != 0,
		map[bool]string{true: "8x16", false: "8x8"}[s.LCDC&0x04 != 0],
		map[bool]string{true: "9C00", false: "9800"}[s.LCDC&0x08 != 0],
		map[bool]string{true: "8000", false: "8800"}[s.LCDC&0x10 != 0],
		s.LCDC&0x20 != 0,
		map[bool]string{true: "9C00", false: "9800"}[s.LCDC&0x40 != 0]))

	// Timer
	b.WriteString(fmt.Sprintf("Timer: DIV=%02X TIMA=%02X TMA=%02X TAC=%02X\n",
		s.DIV, s.TIMA, s.TMA, s.TAC))

	// Interrupts
	b.WriteString(fmt.Sprintf("IRQ: IE=%02X IF=%02X\n", s.IE, s.IF))
	b.WriteString(fmt.Sprintf("     Enabled: VBlank=%v STAT=%v Timer=%v Serial=%v Joypad=%v\n",
		s.IE&0x01 != 0, s.IE&0x02 != 0, s.IE&0x04 != 0, s.IE&0x08 != 0, s.IE&0x10 != 0))
	b.WriteString(fmt.Sprintf("     Pending: VBlank=%v STAT=%v Timer=%v Serial=%v Joypad=%v\n",
		s.IF&0x01 != 0, s.IF&0x02 != 0, s.IF&0x04 != 0, s.IF&0x08 != 0, s.IF&0x10 != 0))

	b.WriteString(fmt.Sprintf("Frame: %s\n", s.FrameHash))

	return b.String()
}

// TileMapDump reads the background tile map from VRAM and returns tile indices
// as a 32x32 grid. mapSelect: false = 0x9800 (VRAM 0x1800), true = 0x9C00 (VRAM 0x1C00).
func TileMapDump(vram []uint8, mapSelect bool) [32][32]uint8 {
	var result [32][32]uint8
	base := uint16(0x1800)
	if mapSelect {
		base = 0x1C00
	}
	for row := 0; row < 32; row++ {
		for col := 0; col < 32; col++ {
			result[row][col] = vram[base+uint16(row)*32+uint16(col)]
		}
	}
	return result
}

// TileMapString formats the visible portion (20x18 tiles) of a tile map as a hex grid.
func TileMapString(vram []uint8, mapSelect bool, scx, scy uint8) string {
	tileMap := TileMapDump(vram, mapSelect)
	var b strings.Builder
	startTileY := int(scy) / 8
	startTileX := int(scx) / 8

	b.WriteString("Tile Map (visible 20x18):\n")
	for row := 0; row < 18; row++ {
		tileY := (startTileY + row) & 31
		for col := 0; col < 20; col++ {
			tileX := (startTileX + col) & 31
			if col > 0 {
				b.WriteByte(' ')
			}
			b.WriteString(fmt.Sprintf("%02X", tileMap[tileY][tileX]))
		}
		b.WriteByte('\n')
	}
	return b.String()
}

// OAMDump returns sprite entries from OAM memory.
type SpriteEntry struct {
	Y, X, Tile, Flags uint8
	Index             int
}

func OAMDump(oam []uint8) []SpriteEntry {
	sprites := make([]SpriteEntry, 40)
	for i := 0; i < 40; i++ {
		sprites[i] = SpriteEntry{
			Y:     oam[i*4],
			X:     oam[i*4+1],
			Tile:  oam[i*4+2],
			Flags: oam[i*4+3],
			Index: i,
		}
	}
	return sprites
}

// VisibleSprites returns only sprites that would be visible on screen.
func VisibleSprites(oam []uint8, lcdc uint8) []SpriteEntry {
	height := 8
	if lcdc&0x04 != 0 {
		height = 16
	}
	var visible []SpriteEntry
	for i := 0; i < 40; i++ {
		y := int(oam[i*4]) - 16
		x := int(oam[i*4+1]) - 8
		if y > -height && y < ppu.ScreenHeight && x > -8 && x < ppu.ScreenWidth {
			visible = append(visible, SpriteEntry{
				Y:     oam[i*4],
				X:     oam[i*4+1],
				Tile:  oam[i*4+2],
				Flags: oam[i*4+3],
				Index: i,
			})
		}
	}
	return visible
}

// SpritesString formats visible sprite data as a table.
func SpritesString(oam []uint8, lcdc uint8) string {
	sprites := VisibleSprites(oam, lcdc)
	if len(sprites) == 0 {
		return "No visible sprites\n"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("Visible Sprites (%d):\n", len(sprites)))
	b.WriteString("  # | Y   X  | Tile | Flags (Priority|YFlip|XFlip|Palette)\n")
	b.WriteString("----+--------+------+----------------------------------------\n")
	for _, s := range sprites {
		b.WriteString(fmt.Sprintf(" %2d | %3d %3d | 0x%02X | P=%v Y=%v X=%v PAL=%d\n",
			s.Index, int(s.Y)-16, int(s.X)-8, s.Tile,
			s.Flags&0x80 != 0, s.Flags&0x40 != 0, s.Flags&0x20 != 0,
			(s.Flags>>4)&1))
	}
	return b.String()
}
