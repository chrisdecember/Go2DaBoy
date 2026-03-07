package ppu

const (
	ScreenWidth  = 160
	ScreenHeight = 144
	FrameBufferSize = ScreenWidth * ScreenHeight * 4
)

const (
	ModeHBlank   = 0
	ModeVBlank   = 1
	ModeOAM      = 2
	ModeTransfer = 3
)

const (
	scanlineCycles = 456
	totalLines     = 154
)

var dmgColors = [4][4]uint8{
	{0x9B, 0xBC, 0x0F, 0xFF},
	{0x8B, 0xAC, 0x0F, 0xFF},
	{0x30, 0x62, 0x30, 0xFF},
	{0x0F, 0x38, 0x0F, 0xFF},
}

func SetPalette(colors [4][4]uint8) {
	dmgColors = colors
}

// Fetcher states
const (
	fetchGetTile     = 0
	fetchGetDataLow  = 1
	fetchGetDataHigh = 2
	fetchPush        = 3
)

type fifoPixel struct {
	color   uint8 // 0-3 color index before palette
	palette uint8 // 0=BGP, 1=OBP0, 2=OBP1
	bgPrio  bool  // sprite OBJ-to-BG priority (attr bit 7)
	isSpr   bool
}

type pixelFIFO struct {
	buf  [16]fifoPixel
	head uint8
	size uint8
}

func (f *pixelFIFO) push(px fifoPixel) {
	idx := (f.head + f.size) & 15
	f.buf[idx] = px
	f.size++
}

func (f *pixelFIFO) pop() fifoPixel {
	px := f.buf[f.head]
	f.head = (f.head + 1) & 15
	f.size--
	return px
}

func (f *pixelFIFO) clear() {
	f.head = 0
	f.size = 0
}

type spriteEntry struct {
	y, x, tile, flags uint8
}

type fetcher struct {
	step       uint8
	ticks      uint8
	tileIndex  uint8
	tileDataLo uint8
	tileDataHi uint8
	mapX       uint8
	tileY      uint8
	fetchWin   bool
}

type PPU struct {
	VRAM [0x2000]uint8
	OAM  [0xA0]uint8

	LCDC uint8
	STAT uint8
	SCY  uint8
	SCX  uint8
	LY   uint8
	LYC  uint8
	DMA  uint8
	BGP  uint8
	OBP0 uint8
	OBP1 uint8
	WY   uint8
	WX   uint8

	mode      uint8
	modeClock int

	// Window internal state
	windowLine    int
	wyTriggered   bool
	windowActive  bool // true if window was rendered on this scanline

	// FIFO state
	bgFIFO  pixelFIFO
	sprFIFO pixelFIFO
	fetch   fetcher
	screenX int
	discardPixels int // SCX & 7 fine scroll discard

	// OAM scan
	scanSprites [10]spriteEntry
	spriteCount int
	oamScanIdx  int

	// Sprite fetch
	spriteFetching   bool
	spriteFetchStep  uint8
	sprFetchDotCount uint8
	currentSprite    spriteEntry
	sprTileDataLo   uint8
	sprTileDataHi   uint8

	// Frame
	frameReady   bool
	prevStatLine bool

	FrameBuffer [FrameBufferSize]uint8
}

func New() *PPU {
	return &PPU{}
}

func (p *PPU) Reset() {
	p.LCDC = 0x91
	p.STAT = 0x82
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
	p.wyTriggered = false
	p.windowActive = false
	p.frameReady = false
	p.prevStatLine = false
	p.screenX = 0
	p.discardPixels = 0
	p.spriteCount = 0
	p.oamScanIdx = 0
	p.spriteFetching = false
	p.bgFIFO.clear()
	p.sprFIFO.clear()
}

func (p *PPU) Read(addr uint16) uint8 {
	switch addr {
	case 0xFF40:
		return p.LCDC
	case 0xFF41:
		return p.STAT | 0x80
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

func (p *PPU) Write(addr uint16, value uint8) {
	switch addr {
	case 0xFF40:
		wasEnabled := p.LCDC&0x80 != 0
		p.LCDC = value
		if wasEnabled && value&0x80 == 0 {
			p.LY = 0
			p.modeClock = 0
			p.mode = ModeHBlank
			p.STAT = p.STAT & 0xFC
			p.windowLine = 0
			p.wyTriggered = false
			p.prevStatLine = false
		}
	case 0xFF41:
		p.STAT = (value & 0xF8) | (p.STAT & 0x07)
	case 0xFF42:
		p.SCY = value
	case 0xFF43:
		p.SCX = value
	case 0xFF44:
	case 0xFF45:
		p.LYC = value
	case 0xFF46:
		p.DMA = value
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

func (p *PPU) ReadVRAM(addr uint16) uint8 {
	if p.mode == ModeTransfer && p.LCDC&0x80 != 0 {
		return 0xFF
	}
	return p.VRAM[addr&0x1FFF]
}

func (p *PPU) WriteVRAM(addr uint16, value uint8) {
	if p.mode == ModeTransfer && p.LCDC&0x80 != 0 {
		return
	}
	p.VRAM[addr&0x1FFF] = value
}

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

func (p *PPU) DirectWriteOAM(offset uint8, value uint8) {
	if int(offset) < len(p.OAM) {
		p.OAM[offset] = value
	}
}

func (p *PPU) Step(cycles int) uint8 {
	if p.LCDC&0x80 == 0 {
		return 0
	}
	var interrupts uint8
	for i := 0; i < cycles; i++ {
		interrupts |= p.tick()
	}
	return interrupts
}

func (p *PPU) tick() uint8 {
	var interrupts uint8
	p.modeClock++

	switch p.mode {
	case ModeOAM:
		// OAM scan: check 2 OAM entries per 4 dots (1 entry per 2 dots)
		if p.modeClock == 1 {
			// Start of OAM scan: reset
			p.spriteCount = 0
			p.oamScanIdx = 0
			// Check window Y trigger
			if p.LY == p.WY {
				p.wyTriggered = true
			}
		}
		if p.modeClock%2 == 0 && p.oamScanIdx < 40 {
			p.oamScanEntry()
		}
		if p.modeClock >= 80 {
			p.modeClock = 0
			p.mode = ModeTransfer
			p.startTransfer()
		}

	case ModeTransfer:
		p.transferTick()
		if p.screenX >= ScreenWidth {
			p.mode = ModeHBlank
			// modeClock continues counting for HBlank duration
		}

	case ModeHBlank:
		// Total scanline = 456 dots. modeClock now includes Mode 3 ticks.
		// We use modeClock to track dots since Mode 2 ended.
		// Mode 2 took 80 dots. We need total - 80 more dots for Mode 3 + HBlank.
		if p.modeClock >= (scanlineCycles - 80) {
			if p.windowActive {
				p.windowLine++
				p.windowActive = false
			}
			p.modeClock = 0
			p.LY++
			if p.LY >= ScreenHeight {
				p.mode = ModeVBlank
				interrupts |= 0x01
				p.frameReady = true
			} else {
				p.mode = ModeOAM
			}
		}

	case ModeVBlank:
		if p.modeClock >= scanlineCycles {
			p.modeClock = 0
			p.LY++
			if p.LY >= totalLines {
				p.LY = 0
				p.mode = ModeOAM
				p.windowLine = 0
				p.wyTriggered = false
			}
		}
	}

	// Update STAT
	p.STAT = (p.STAT & 0xFC) | p.mode
	if p.LY == p.LYC {
		p.STAT |= 0x04
	} else {
		p.STAT &^= 0x04
	}
	interrupts |= p.checkStatIRQ()
	return interrupts
}

func (p *PPU) oamScanEntry() {
	if p.oamScanIdx >= 40 || p.spriteCount >= 10 {
		p.oamScanIdx++
		return
	}
	base := p.oamScanIdx * 4
	sy := p.OAM[base]
	sx := p.OAM[base+1]
	tile := p.OAM[base+2]
	flags := p.OAM[base+3]
	p.oamScanIdx++

	spriteH := uint8(8)
	if p.LCDC&0x04 != 0 {
		spriteH = 16
	}
	top := int(sy) - 16
	if int(p.LY) >= top && int(p.LY) < top+int(spriteH) {
		p.scanSprites[p.spriteCount] = spriteEntry{y: sy, x: sx, tile: tile, flags: flags}
		p.spriteCount++
	}
}

func (p *PPU) startTransfer() {
	p.screenX = 0
	p.discardPixels = int(p.SCX & 7)
	p.bgFIFO.clear()
	p.sprFIFO.clear()
	p.spriteFetching = false

	// Init fetcher for background
	p.fetch.step = fetchGetTile
	p.fetch.ticks = 0
	p.fetch.fetchWin = false
	p.fetch.mapX = p.SCX / 8
	p.fetch.tileY = (p.LY + p.SCY) & 7
}

func (p *PPU) transferTick() {
	// Check for sprite fetch trigger
	if !p.spriteFetching && p.LCDC&0x02 != 0 {
		for i := 0; i < p.spriteCount; i++ {
			sx := int(p.scanSprites[i].x)
			// Sprite X=0 means off-screen left; visible at screenX when sx-8 == screenX
			targetX := sx - 8
			if targetX <= p.screenX && sx > 0 {
				// Check not already fetched (mark by zeroing x)
				if p.scanSprites[i].x != 0 {
					p.currentSprite = p.scanSprites[i]
					p.scanSprites[i].x = 0 // mark fetched
					p.spriteFetching = true
					p.spriteFetchStep = 0
					p.sprFetchDotCount = 0
					break
				}
			}
		}
	}

	if p.spriteFetching {
		p.sprFetchTick()
		return
	}

	// Advance BG/Win fetcher
	p.fetcherTick()

	// Try to pop a pixel
	if p.bgFIFO.size > 0 {
		p.popPixel()
	}
}

func (p *PPU) fetcherTick() {
	p.fetch.ticks++
	if p.fetch.ticks < 2 {
		return // Each step takes 2 dots
	}
	p.fetch.ticks = 0

	switch p.fetch.step {
	case fetchGetTile:
		var mapBase uint16
		var tileY uint8
		var mapX uint8

		if p.fetch.fetchWin {
			mapBase = 0x1800
			if p.LCDC&0x40 != 0 {
				mapBase = 0x1C00
			}
			tileY = uint8(p.windowLine) & 7
			mapX = p.fetch.mapX
		} else {
			mapBase = 0x1800
			if p.LCDC&0x08 != 0 {
				mapBase = 0x1C00
			}
			tileY = (p.LY + p.SCY) & 7
			mapX = p.fetch.mapX
		}

		var tileRow uint16
		if p.fetch.fetchWin {
			tileRow = uint16(p.windowLine) / 8
		} else {
			tileRow = (uint16(p.LY) + uint16(p.SCY)) / 8 & 31
		}

		tileAddr := mapBase + (tileRow&31)*32 + uint16(mapX&31)
		p.fetch.tileIndex = p.VRAM[tileAddr]
		p.fetch.tileY = tileY
		p.fetch.step = fetchGetDataLow

	case fetchGetDataLow:
		addr := p.tileDataAddr(p.fetch.tileIndex, p.fetch.tileY)
		p.fetch.tileDataLo = p.VRAM[addr]
		p.fetch.step = fetchGetDataHigh

	case fetchGetDataHigh:
		addr := p.tileDataAddr(p.fetch.tileIndex, p.fetch.tileY)
		p.fetch.tileDataHi = p.VRAM[addr+1]
		p.fetch.step = fetchPush

	case fetchPush:
		if p.bgFIFO.size > 0 {
			return // Wait until FIFO is empty
		}
		lo := p.fetch.tileDataLo
		hi := p.fetch.tileDataHi
		for bit := 7; bit >= 0; bit-- {
			color := ((hi >> uint(bit)) & 1) << 1
			color |= (lo >> uint(bit)) & 1
			p.bgFIFO.push(fifoPixel{color: color, palette: 0})
		}
		p.fetch.mapX++
		p.fetch.step = fetchGetTile
	}
}

func (p *PPU) tileDataAddr(tileIdx uint8, tileY uint8) uint16 {
	if p.LCDC&0x10 == 0 {
		// Signed addressing: base 0x1000 (0x9000 in memory map)
		return uint16(int(0x1000) + int(int8(tileIdx))*16 + int(tileY)*2)
	}
	// Unsigned addressing: base 0x0000 (0x8000 in memory map)
	return uint16(tileIdx)*16 + uint16(tileY)*2
}

func (p *PPU) popPixel() {
	bgPx := p.bgFIFO.pop()

	// Discard pixels for fine X scroll
	if p.discardPixels > 0 {
		p.discardPixels--
		// Also discard sprite pixel if present
		if p.sprFIFO.size > 0 {
			p.sprFIFO.pop()
		}
		return
	}

	if p.screenX >= ScreenWidth {
		return
	}

	// Check window trigger
	if !p.fetch.fetchWin && p.wyTriggered && p.LCDC&0x20 != 0 {
		wx := int(p.WX) - 7
		if p.screenX >= wx && wx < ScreenWidth {
			// Switch to window
			p.windowActive = true
			p.fetch.fetchWin = true
			p.fetch.mapX = 0
			p.fetch.step = fetchGetTile
			p.fetch.ticks = 0
			p.bgFIFO.clear()
			// Re-push this pixel's slot won't happen; we just return
			// and the fetcher will fill the FIFO with window tiles
			return
		}
	}

	// Determine BG color
	bgColor := bgPx.color
	if p.LCDC&0x01 == 0 {
		bgColor = 0 // BG/Window disabled
	}

	// Check sprite pixel
	var finalColor uint8
	var palette uint8 = 0 // BGP

	if p.sprFIFO.size > 0 {
		sprPx := p.sprFIFO.pop()
		if sprPx.isSpr && sprPx.color != 0 && p.LCDC&0x02 != 0 {
			if !sprPx.bgPrio || bgColor == 0 {
				// Sprite wins
				finalColor = sprPx.color
				palette = sprPx.palette
				p.writePixel(finalColor, palette)
				p.screenX++
				return
			}
		}
	}

	// Use BG/Window color
	finalColor = bgColor
	p.writePixel(finalColor, 0)
	p.screenX++
}

func (p *PPU) writePixel(colorIdx uint8, palette uint8) {
	var palReg uint8
	switch palette {
	case 0:
		palReg = p.BGP
	case 1:
		palReg = p.OBP0
	case 2:
		palReg = p.OBP1
	default:
		palReg = p.BGP
	}
	shade := (palReg >> (colorIdx * 2)) & 0x03
	idx := int(p.LY)*ScreenWidth*4 + p.screenX*4
	if idx+3 < FrameBufferSize {
		p.FrameBuffer[idx] = dmgColors[shade][0]
		p.FrameBuffer[idx+1] = dmgColors[shade][1]
		p.FrameBuffer[idx+2] = dmgColors[shade][2]
		p.FrameBuffer[idx+3] = dmgColors[shade][3]
	}
}

func (p *PPU) sprFetchTick() {
	p.sprFetchDotCount++
	if p.sprFetchDotCount < 2 {
		return
	}
	p.sprFetchDotCount = 0

	switch p.spriteFetchStep {
	case 0: // Get tile
		spr := p.currentSprite
		spriteH := uint8(8)
		if p.LCDC&0x04 != 0 {
			spriteH = 16
		}
		row := int(p.LY) - (int(spr.y) - 16)
		if spr.flags&0x40 != 0 { // Y flip
			row = int(spriteH) - 1 - row
		}
		tileIdx := spr.tile
		if spriteH == 16 {
			tileIdx &= 0xFE
			if row >= 8 {
				tileIdx++
				row -= 8
			}
		}
		addr := uint16(tileIdx)*16 + uint16(row)*2
		p.sprTileDataLo = p.VRAM[addr]
		p.sprTileDataHi = p.VRAM[addr+1]
		p.spriteFetchStep = 1

	case 1: // Data low (already read above, just a timing slot)
		p.spriteFetchStep = 2

	case 2: // Data high + push to sprite FIFO
		spr := p.currentSprite
		lo := p.sprTileDataLo
		hi := p.sprTileDataHi
		palID := uint8(1)
		if spr.flags&0x10 != 0 {
			palID = 2
		}
		bgPrio := spr.flags&0x80 != 0
		startX := int(spr.x) - 8
		for bit := 7; bit >= 0; bit-- {
			px := 7 - bit
			if spr.flags&0x20 != 0 { // X flip
				px = bit
			}
			color := ((hi >> uint(px)) & 1) << 1
			color |= (lo >> uint(px)) & 1

			fifoIdx := bit // position in FIFO corresponds to pixel column
			screenPos := startX + (7 - bit)
			if screenPos < 0 || screenPos >= ScreenWidth {
				continue
			}

			// Mix into sprite FIFO: only overwrite transparent (color 0) entries
			_ = fifoIdx
			sprFIFOPos := uint8(screenPos - p.screenX + p.discardPixels)
			if sprFIFOPos < p.sprFIFO.size {
				// Check existing entry
				realIdx := (p.sprFIFO.head + sprFIFOPos) & 15
				if p.sprFIFO.buf[realIdx].color == 0 && color != 0 {
					p.sprFIFO.buf[realIdx] = fifoPixel{
						color:   color,
						palette: palID,
						bgPrio:  bgPrio,
						isSpr:   true,
					}
				}
			} else {
				// Extend FIFO
				for p.sprFIFO.size <= sprFIFOPos && p.sprFIFO.size < 16 {
					p.sprFIFO.push(fifoPixel{})
				}
				if sprFIFOPos < 16 {
					realIdx := (p.sprFIFO.head + sprFIFOPos) & 15
					p.sprFIFO.buf[realIdx] = fifoPixel{
						color:   color,
						palette: palID,
						bgPrio:  bgPrio,
						isSpr:   true,
					}
				}
			}
		}
		p.spriteFetching = false
	}
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
	if statLine && !p.prevStatLine {
		p.prevStatLine = statLine
		return 0x02
	}
	p.prevStatLine = statLine
	return 0
}

func (p *PPU) IsFrameReady() bool {
	if p.frameReady {
		p.frameReady = false
		return true
	}
	return false
}
