package ppu

const (
	ScreenWidth  = 160
	ScreenHeight = 144
	// RGBA bytes per frame
	FrameBufferSize = ScreenWidth * ScreenHeight * 4
)

// PPU modes
const (
	ModeHBlank  = 0
	ModeVBlank  = 1
	ModeOAM     = 2
	ModeTransfer = 3
)

// Scanline timing (T-cycles)
const (
	oamCycles      = 80
	transferCycles = 172 // Minimum; actual varies with sprites
	hblankCycles   = 204 // Maximum; fills remainder of 456
	scanlineCycles = 456
	vblankLines    = 10
	totalLines     = 154
)

// DMG palette colors (classic green)
var dmgColors = [4][4]uint8{
	{0x9B, 0xBC, 0x0F, 0xFF}, // Lightest
	{0x8B, 0xAC, 0x0F, 0xFF},
	{0x30, 0x62, 0x30, 0xFF},
	{0x0F, 0x38, 0x0F, 0xFF}, // Darkest
}

// PPU implements the Game Boy pixel processing unit
type PPU struct {
	// VRAM and OAM
	VRAM [0x2000]uint8
	OAM  [0xA0]uint8

	// Registers
	LCDC uint8 // 0xFF40 LCD Control
	STAT uint8 // 0xFF41 LCD Status
	SCY  uint8 // 0xFF42 Scroll Y
	SCX  uint8 // 0xFF43 Scroll X
	LY   uint8 // 0xFF44 LCD Y Coordinate
	LYC  uint8 // 0xFF45 LY Compare
	DMA  uint8 // 0xFF46 DMA Transfer
	BGP  uint8 // 0xFF47 BG Palette
	OBP0 uint8 // 0xFF48 OBJ Palette 0
	OBP1 uint8 // 0xFF49 OBJ Palette 1
	WY   uint8 // 0xFF4A Window Y
	WX   uint8 // 0xFF4B Window X

	// Internal state
	mode          uint8
	modeClock     int
	windowLine    int // Internal window line counter
	frameReady    bool
	statIRQ       bool
	prevStatLine  bool

	// Frame buffer (RGBA)
	FrameBuffer [FrameBufferSize]uint8
}

func New() *PPU {
	return &PPU{}
}

func (p *PPU) Reset() {
	p.LCDC = 0x91
	p.STAT = 0x82 // Mode 2 (OAM) initially, matching p.mode
	p.SCY = 0
	p.SCX = 0
	p.LY = 0
	p.LYC = 0
	p.BGP = 0xFC
	p.OBP0 = 0xFF
	p.OBP1 = 0xFF
	p.WY = 0
	p.WX = 0
	p.mode = ModeOAM
	p.modeClock = 0
	p.windowLine = 0
	p.frameReady = false
}

// Read handles PPU register reads
func (p *PPU) Read(addr uint16) uint8 {
	switch addr {
	case 0xFF40:
		return p.LCDC
	case 0xFF41:
		stat := p.STAT | 0x80 // Bit 7 always reads as 1
		return stat
	case 0xFF42:
		return p.SCY
	case 0xFF43:
		return p.SCX
	case 0xFF44:
		return p.LY
	case 0xFF45:
		return p.LYC
	case 0xFF46:
		return p.DMA
	case 0xFF47:
		return p.BGP
	case 0xFF48:
		return p.OBP0
	case 0xFF49:
		return p.OBP1
	case 0xFF4A:
		return p.WY
	case 0xFF4B:
		return p.WX
	}
	return 0xFF
}

// Write handles PPU register writes
func (p *PPU) Write(addr uint16, value uint8) {
	switch addr {
	case 0xFF40:
		wasEnabled := p.LCDC&0x80 != 0
		p.LCDC = value
		if wasEnabled && value&0x80 == 0 {
			// LCD turned off
			p.LY = 0
			p.modeClock = 0
			p.mode = ModeHBlank
			p.STAT = p.STAT & 0xFC
			p.windowLine = 0
			p.prevStatLine = false
		}
	case 0xFF41:
		p.STAT = (value & 0xF8) | (p.STAT & 0x07) // Lower 3 bits read-only
	case 0xFF42:
		p.SCY = value
	case 0xFF43:
		p.SCX = value
	case 0xFF44:
		// LY is read-only
	case 0xFF45:
		p.LYC = value
	case 0xFF46:
		p.DMA = value
		// DMA is handled by the memory bus
	case 0xFF47:
		p.BGP = value
	case 0xFF48:
		p.OBP0 = value
	case 0xFF49:
		p.OBP1 = value
	case 0xFF4A:
		p.WY = value
	case 0xFF4B:
		p.WX = value
	}
}

// ReadVRAM reads from video RAM
func (p *PPU) ReadVRAM(addr uint16) uint8 {
	if p.mode == ModeTransfer && p.LCDC&0x80 != 0 {
		return 0xFF // VRAM inaccessible during mode 3
	}
	return p.VRAM[addr&0x1FFF]
}

// WriteVRAM writes to video RAM
func (p *PPU) WriteVRAM(addr uint16, value uint8) {
	if p.mode == ModeTransfer && p.LCDC&0x80 != 0 {
		return // VRAM inaccessible during mode 3
	}
	p.VRAM[addr&0x1FFF] = value
}

// ReadOAM reads from object attribute memory
func (p *PPU) ReadOAM(addr uint16) uint8 {
	if (p.mode == ModeOAM || p.mode == ModeTransfer) && p.LCDC&0x80 != 0 {
		return 0xFF
	}
	offset := addr & 0xFF
	if offset >= 0xA0 {
		return 0xFF
	}
	return p.OAM[offset]
}

// WriteOAM writes to object attribute memory
func (p *PPU) WriteOAM(addr uint16, value uint8) {
	if (p.mode == ModeOAM || p.mode == ModeTransfer) && p.LCDC&0x80 != 0 {
		return
	}
	offset := addr & 0xFF
	if offset >= 0xA0 {
		return
	}
	p.OAM[offset] = value
}

// DirectWriteOAM writes to OAM bypassing mode checks (for DMA)
func (p *PPU) DirectWriteOAM(offset uint8, value uint8) {
	if int(offset) < len(p.OAM) {
		p.OAM[offset] = value
	}
}

// Step advances the PPU by the given number of T-cycles.
// Returns a bitmask of interrupts: bit 0 = VBlank, bit 1 = STAT
func (p *PPU) Step(cycles int) uint8 {
	if p.LCDC&0x80 == 0 {
		return 0 // LCD disabled
	}

	var interrupts uint8

	for i := 0; i < cycles; i++ {
		p.modeClock++
		irq := p.tick()
		interrupts |= irq
	}

	return interrupts
}

func (p *PPU) tick() uint8 {
	var interrupts uint8

	switch p.mode {
	case ModeOAM:
		if p.modeClock >= oamCycles {
			p.modeClock -= oamCycles
			p.mode = ModeTransfer
		}

	case ModeTransfer:
		if p.modeClock >= transferCycles {
			p.modeClock -= transferCycles
			p.mode = ModeHBlank
			p.renderScanline()
		}

	case ModeHBlank:
		if p.modeClock >= hblankCycles {
			p.modeClock -= hblankCycles
			p.LY++

			if p.LY >= ScreenHeight {
				p.mode = ModeVBlank
				interrupts |= 0x01 // VBlank interrupt
				p.frameReady = true
			} else {
				p.mode = ModeOAM
			}
		}

	case ModeVBlank:
		if p.modeClock >= scanlineCycles {
			p.modeClock -= scanlineCycles
			p.LY++

			if p.LY >= totalLines {
				p.LY = 0
				p.mode = ModeOAM
				p.windowLine = 0
			}
		}
	}

	// Update STAT mode and coincidence flag
	p.STAT = (p.STAT & 0xFC) | p.mode
	if p.LY == p.LYC {
		p.STAT |= 0x04
	} else {
		p.STAT &^= 0x04
	}

	// Evaluate STAT IRQ line once per tick after all state changes
	interrupts |= p.checkStatIRQ()

	return interrupts
}

func (p *PPU) checkStatIRQ() uint8 {
	statLine := false
	if p.STAT&0x20 != 0 && p.mode == ModeOAM {
		statLine = true
	}
	if p.STAT&0x10 != 0 && p.mode == ModeVBlank {
		statLine = true
	}
	if p.STAT&0x08 != 0 && p.mode == ModeHBlank {
		statLine = true
	}
	if p.STAT&0x40 != 0 && p.LY == p.LYC {
		statLine = true
	}

	// Rising edge only
	if statLine && !p.prevStatLine {
		p.prevStatLine = statLine
		return 0x02 // STAT interrupt
	}
	p.prevStatLine = statLine
	return 0
}

// IsFrameReady returns true if a new frame is ready
func (p *PPU) IsFrameReady() bool {
	if p.frameReady {
		p.frameReady = false
		return true
	}
	return false
}

// renderScanline renders the current scanline to the frame buffer
func (p *PPU) renderScanline() {
	if p.LY >= ScreenHeight {
		return
	}

	// Clear scanline first
	lineOffset := int(p.LY) * ScreenWidth * 4
	for x := 0; x < ScreenWidth; x++ {
		idx := lineOffset + x*4
		// Default to lightest color
		p.FrameBuffer[idx] = dmgColors[0][0]
		p.FrameBuffer[idx+1] = dmgColors[0][1]
		p.FrameBuffer[idx+2] = dmgColors[0][2]
		p.FrameBuffer[idx+3] = dmgColors[0][3]
	}

	// Background priority array for sprite rendering
	var bgPriority [ScreenWidth]uint8 // Stores color index for BG priority

	if p.LCDC&0x01 != 0 {
		p.renderBackground(&bgPriority)
	}

	if p.LCDC&0x20 != 0 {
		p.renderWindow(&bgPriority)
	}

	if p.LCDC&0x02 != 0 {
		p.renderSprites(bgPriority)
	}
}

func (p *PPU) renderBackground(bgPriority *[ScreenWidth]uint8) {
	// Which tile map to use
	tileMapBase := uint16(0x1800) // 0x9800
	if p.LCDC&0x08 != 0 {
		tileMapBase = 0x1C00 // 0x9C00
	}

	// Which tile data to use
	useSigned := p.LCDC&0x10 == 0

	y := uint16(p.SCY) + uint16(p.LY)
	tileRow := (y / 8) & 31
	lineOffset := int(p.LY) * ScreenWidth * 4

	for x := 0; x < ScreenWidth; x++ {
		px := (uint16(p.SCX) + uint16(x)) & 0xFF
		tileCol := (px / 8) & 31

		// Get tile index from tile map
		tileAddr := tileMapBase + tileRow*32 + tileCol
		tileIdx := p.VRAM[tileAddr]

		// Get tile data address
		var tileDataAddr uint16
		if useSigned {
			// Signed addressing: base $9000 (VRAM 0x1000), tile index is signed
			tileDataAddr = uint16(int(0x1000) + int(int8(tileIdx))*16)
		} else {
			// Unsigned addressing: 0x8000 base
			tileDataAddr = uint16(tileIdx) * 16
		}

		// Get the two bytes for this row of the tile
		row := (y % 8) * 2
		b1 := p.VRAM[tileDataAddr+row]
		b2 := p.VRAM[tileDataAddr+row+1]

		// Get the bit for this pixel (7 - pixel position within tile)
		bit := 7 - (px % 8)
		colorIdx := ((b2 >> bit) & 1 << 1) | ((b1 >> bit) & 1)

		// Apply palette
		paletteColor := (p.BGP >> (colorIdx * 2)) & 0x03

		bgPriority[x] = colorIdx

		idx := lineOffset + x*4
		p.FrameBuffer[idx] = dmgColors[paletteColor][0]
		p.FrameBuffer[idx+1] = dmgColors[paletteColor][1]
		p.FrameBuffer[idx+2] = dmgColors[paletteColor][2]
		p.FrameBuffer[idx+3] = dmgColors[paletteColor][3]
	}
}

func (p *PPU) renderWindow(bgPriority *[ScreenWidth]uint8) {
	if p.WY > p.LY {
		return
	}

	// Window X is offset by 7
	wx := int(p.WX) - 7
	if wx >= ScreenWidth {
		return
	}

	tileMapBase := uint16(0x1800)
	if p.LCDC&0x40 != 0 {
		tileMapBase = 0x1C00
	}

	useSigned := p.LCDC&0x10 == 0

	y := uint16(p.windowLine)
	tileRow := (y / 8) & 31
	lineOffset := int(p.LY) * ScreenWidth * 4

	rendered := false

	for x := 0; x < ScreenWidth; x++ {
		if x < wx {
			continue
		}

		rendered = true
		px := uint16(x - wx)
		tileCol := (px / 8) & 31

		tileAddr := tileMapBase + tileRow*32 + tileCol
		tileIdx := p.VRAM[tileAddr]

		var tileDataAddr uint16
		if useSigned {
			tileDataAddr = uint16(int(0x1000) + int(int8(tileIdx))*16)
		} else {
			tileDataAddr = uint16(tileIdx) * 16
		}

		row := (y % 8) * 2
		b1 := p.VRAM[tileDataAddr+row]
		b2 := p.VRAM[tileDataAddr+row+1]

		bit := 7 - (px % 8)
		colorIdx := ((b2 >> bit) & 1 << 1) | ((b1 >> bit) & 1)

		paletteColor := (p.BGP >> (colorIdx * 2)) & 0x03

		bgPriority[x] = colorIdx

		idx := lineOffset + x*4
		p.FrameBuffer[idx] = dmgColors[paletteColor][0]
		p.FrameBuffer[idx+1] = dmgColors[paletteColor][1]
		p.FrameBuffer[idx+2] = dmgColors[paletteColor][2]
		p.FrameBuffer[idx+3] = dmgColors[paletteColor][3]
	}

	if rendered {
		p.windowLine++
	}
}

func (p *PPU) renderSprites(bgPriority [ScreenWidth]uint8) {
	spriteHeight := 8
	if p.LCDC&0x04 != 0 {
		spriteHeight = 16
	}

	// Collect sprites on this scanline (max 10)
	type spriteEntry struct {
		y, x, tile, flags uint8
		oamIdx            int
	}
	var sprites [10]spriteEntry
	spriteCount := 0

	for i := 0; i < 40 && spriteCount < 10; i++ {
		oamAddr := i * 4
		sy := int(p.OAM[oamAddr]) - 16

		if int(p.LY) >= sy && int(p.LY) < sy+spriteHeight {
			sprites[spriteCount] = spriteEntry{
				y:      p.OAM[oamAddr],
				x:      p.OAM[oamAddr+1],
				tile:   p.OAM[oamAddr+2],
				flags:  p.OAM[oamAddr+3],
				oamIdx: i,
			}
			spriteCount++
		}
	}

	// Sort by X position (lower X = higher priority, stable by OAM order)
	for i := 0; i < spriteCount-1; i++ {
		for j := i + 1; j < spriteCount; j++ {
			if sprites[j].x < sprites[i].x ||
				(sprites[j].x == sprites[i].x && sprites[j].oamIdx < sprites[i].oamIdx) {
				sprites[i], sprites[j] = sprites[j], sprites[i]
			}
		}
	}

	lineOffset := int(p.LY) * ScreenWidth * 4

	// Render in reverse order (lowest priority first, so highest priority overwrites)
	for i := spriteCount - 1; i >= 0; i-- {
		sprite := sprites[i]
		sy := int(sprite.y) - 16
		sx := int(sprite.x) - 8

		row := int(p.LY) - sy

		// Y flip
		if sprite.flags&0x40 != 0 {
			row = spriteHeight - 1 - row
		}

		tileIdx := sprite.tile
		if spriteHeight == 16 {
			tileIdx &= 0xFE // In 8x16 mode, bit 0 is ignored
			if row >= 8 {
				tileIdx++
				row -= 8
			}
		}

		tileAddr := uint16(tileIdx)*16 + uint16(row)*2
		b1 := p.VRAM[tileAddr]
		b2 := p.VRAM[tileAddr+1]

		for px := 0; px < 8; px++ {
			screenX := sx + px
			if screenX < 0 || screenX >= ScreenWidth {
				continue
			}

			bit := uint8(7 - px)
			// X flip
			if sprite.flags&0x20 != 0 {
				bit = uint8(px)
			}

			colorIdx := ((b2 >> bit) & 1 << 1) | ((b1 >> bit) & 1)
			if colorIdx == 0 {
				continue // Transparent
			}

			// BG priority: if set, sprite is behind BG colors 1-3
			if sprite.flags&0x80 != 0 && bgPriority[screenX] != 0 {
				continue
			}

			// Select palette
			palette := p.OBP0
			if sprite.flags&0x10 != 0 {
				palette = p.OBP1
			}

			paletteColor := (palette >> (colorIdx * 2)) & 0x03

			idx := lineOffset + screenX*4
			p.FrameBuffer[idx] = dmgColors[paletteColor][0]
			p.FrameBuffer[idx+1] = dmgColors[paletteColor][1]
			p.FrameBuffer[idx+2] = dmgColors[paletteColor][2]
			p.FrameBuffer[idx+3] = dmgColors[paletteColor][3]
		}
	}
}
