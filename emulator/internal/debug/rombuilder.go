package debug

// ROMBuilder generates minimal Game Boy test ROMs programmatically.
// Code is placed at the entry point (0x0100) with an optional jump past
// the header area. The resulting ROM is 32KB (minimum for MBC0).
type ROMBuilder struct {
	rom [0x8000]byte
	pc  int // current write position
}

// NewROMBuilder creates a builder with the write cursor at 0x0100.
func NewROMBuilder() *ROMBuilder {
	r := &ROMBuilder{pc: 0x0100}
	// Set header defaults so cartridge loader accepts it
	r.rom[0x0148] = 0x00 // 32KB ROM
	r.rom[0x0149] = 0x00 // No RAM
	return r
}

// Build returns the final 32KB ROM.
func (r *ROMBuilder) Build() []byte {
	out := make([]byte, len(r.rom))
	copy(out, r.rom[:])
	return out
}

// PC returns the current program counter.
func (r *ROMBuilder) PC() int { return r.pc }

// --- Raw byte emission ---

func (r *ROMBuilder) emit(b ...byte) {
	for _, v := range b {
		r.rom[r.pc] = v
		r.pc++
	}
}

// --- LR35902 instructions ---

// NOP: 0x00
func (r *ROMBuilder) NOP() { r.emit(0x00) }

// DI: disable interrupts
func (r *ROMBuilder) DI() { r.emit(0xF3) }

// EI: enable interrupts
func (r *ROMBuilder) EI() { r.emit(0xFB) }

// HALT: halt until interrupt
func (r *ROMBuilder) HALT() { r.emit(0x76) }

// LDImm8A: LD A, n
func (r *ROMBuilder) LDImm8A(n uint8) { r.emit(0x3E, n) }

// LDHLDA: LD (HL-), A — write A to (HL) then decrement HL
func (r *ROMBuilder) LDHLDA() { r.emit(0x32) }

// LDHLIA: LD (HL+), A — write A to (HL) then increment HL
func (r *ROMBuilder) LDHLIA() { r.emit(0x22) }

// LDHnA: LDH (n), A — write A to 0xFF00+n
func (r *ROMBuilder) LDHnA(n uint8) { r.emit(0xE0, n) }

// LDHAn: LDH A, (n) — read 0xFF00+n into A
func (r *ROMBuilder) LDHAn(n uint8) { r.emit(0xF0, n) }

// LDAddr16A: LD (a16), A
func (r *ROMBuilder) LDAddr16A(addr uint16) {
	r.emit(0xEA, byte(addr&0xFF), byte(addr>>8))
}

// LDAAddr16: LD A, (a16)
func (r *ROMBuilder) LDAAddr16(addr uint16) {
	r.emit(0xFA, byte(addr&0xFF), byte(addr>>8))
}

// LDHLImm16: LD HL, nn
func (r *ROMBuilder) LDHLImm16(nn uint16) {
	r.emit(0x21, byte(nn&0xFF), byte(nn>>8))
}

// LDBCImm16: LD BC, nn
func (r *ROMBuilder) LDBCImm16(nn uint16) {
	r.emit(0x01, byte(nn&0xFF), byte(nn>>8))
}

// LDDEImm16: LD DE, nn
func (r *ROMBuilder) LDDEImm16(nn uint16) {
	r.emit(0x11, byte(nn&0xFF), byte(nn>>8))
}

// LDHLmemA: LD (HL), A
func (r *ROMBuilder) LDHLmemA() { r.emit(0x77) }

// LDAHLmem: LD A, (HL)
func (r *ROMBuilder) LDAHLmem() { r.emit(0x7E) }

// LDAImm: alias for LDImm8A
func (r *ROMBuilder) LDAImm(n uint8) { r.emit(0x3E, n) }

// LDB: LD B, n
func (r *ROMBuilder) LDB(n uint8) { r.emit(0x06, n) }

// LDC: LD C, n
func (r *ROMBuilder) LDC(n uint8) { r.emit(0x0E, n) }

// LDD: LD D, n
func (r *ROMBuilder) LDD(n uint8) { r.emit(0x16, n) }

// LDE: LD E, n
func (r *ROMBuilder) LDE(n uint8) { r.emit(0x1E, n) }

// XORA: XOR A (sets A=0, Z flag)
func (r *ROMBuilder) XORA() { r.emit(0xAF) }

// IncHL: INC HL
func (r *ROMBuilder) IncHL() { r.emit(0x23) }

// DecBC: DEC BC
func (r *ROMBuilder) DecBC() { r.emit(0x0B) }

// DecB: DEC B
func (r *ROMBuilder) DecB() { r.emit(0x05) }

// LDABReg: LD A, B
func (r *ROMBuilder) LDABReg() { r.emit(0x78) }

// ORC: OR C
func (r *ROMBuilder) ORC() { r.emit(0xB1) }

// ORB: OR B
func (r *ROMBuilder) ORB() { r.emit(0xB0) }

// CPn: CP n (compare A with immediate)
func (r *ROMBuilder) CPn(n uint8) { r.emit(0xFE, n) }

// JRnz: JR NZ, offset (signed)
func (r *ROMBuilder) JRnz(offset int8) { r.emit(0x20, byte(offset)) }

// JRz: JR Z, offset
func (r *ROMBuilder) JRz(offset int8) { r.emit(0x28, byte(offset)) }

// JR: JR offset (unconditional)
func (r *ROMBuilder) JR(offset int8) { r.emit(0x18, byte(offset)) }

// JPnn: JP nn (unconditional jump to absolute address)
func (r *ROMBuilder) JPnn(addr uint16) {
	r.emit(0xC3, byte(addr&0xFF), byte(addr>>8))
}

// CALL: CALL nn
func (r *ROMBuilder) CALL(addr uint16) {
	r.emit(0xCD, byte(addr&0xFF), byte(addr>>8))
}

// RET: return from call
func (r *ROMBuilder) RET() { r.emit(0xC9) }

// PUSH AF
func (r *ROMBuilder) PushAF() { r.emit(0xF5) }

// POP AF
func (r *ROMBuilder) PopAF() { r.emit(0xF1) }

// PUSH BC
func (r *ROMBuilder) PushBC() { r.emit(0xC5) }

// POP BC
func (r *ROMBuilder) PopBC() { r.emit(0xC1) }

// --- High-level helpers ---

// WaitVBlank emits a loop that polls LY (0xFF44) until it reaches 144 (VBlank).
// Uses A register.
func (r *ROMBuilder) WaitVBlank() {
	// .loop: LDH A, (0x44)  ; 2 bytes
	//        CP 144          ; 2 bytes
	//        JR NZ, .loop    ; 2 bytes — offset is relative to PC after JR
	r.LDHAn(0x44)
	r.CPn(144)
	r.JRnz(-6) // back to LDH: (PC+2) + (-6) = PC-4 = start of LDH
}

// DisableLCD turns off the LCD (bit 7 of LCDC).
// Should be done during VBlank to avoid damage on real hardware.
func (r *ROMBuilder) DisableLCD() {
	r.LDHAn(0x40) // read LCDC
	// AND 0x7F (clear bit 7): 0xE6, 0x7F
	r.emit(0xE6, 0x7F)
	r.LDHnA(0x40) // write LCDC
}

// EnableLCD turns on the LCD with the given LCDC flags OR'd with 0x80.
func (r *ROMBuilder) EnableLCD(lcdc uint8) {
	r.LDAImm(lcdc | 0x80)
	r.LDHnA(0x40)
}

// SetBGP sets the background palette register.
func (r *ROMBuilder) SetBGP(bgp uint8) {
	r.LDAImm(bgp)
	r.LDHnA(0x47)
}

// SetOBP0 sets sprite palette 0.
func (r *ROMBuilder) SetOBP0(obp uint8) {
	r.LDAImm(obp)
	r.LDHnA(0x48)
}

// SetOBP1 sets sprite palette 1.
func (r *ROMBuilder) SetOBP1(obp uint8) {
	r.LDAImm(obp)
	r.LDHnA(0x49)
}

// SetScroll sets SCY and SCX.
func (r *ROMBuilder) SetScroll(scy, scx uint8) {
	r.LDAImm(scy)
	r.LDHnA(0x42) // SCY
	r.LDAImm(scx)
	r.LDHnA(0x43) // SCX
}

// SetWindow sets WY and WX.
func (r *ROMBuilder) SetWindow(wy, wx uint8) {
	r.LDAImm(wy)
	r.LDHnA(0x4A) // WY
	r.LDAImm(wx)
	r.LDHnA(0x4B) // WX
}

// FillVRAM writes `value` to `count` bytes starting at VRAM address
// (0x8000 + vramOffset). Uses A, B, C, E, H, L registers.
func (r *ROMBuilder) FillVRAM(vramOffset uint16, value uint8, count uint16) {
	addr := 0x8000 + vramOffset
	r.LDHLImm16(addr)
	r.LDBCImm16(count)
	r.LDE(value)
	// .loop:
	loopPC := r.pc
	r.emit(0x73) // LD (HL), E
	r.IncHL()    // INC HL
	r.DecBC()    // DEC BC
	r.LDABReg()  // LD A, B
	r.ORC()      // OR C
	r.JRnz(int8(loopPC - (r.pc + 2)))
}

// WriteTileData writes raw tile data (16 bytes per tile) to VRAM.
// tileIndex is the tile number (0-255), data is 16 bytes.
func (r *ROMBuilder) WriteTileData(tileIndex uint8, data [16]byte) {
	addr := 0x8000 + uint16(tileIndex)*16
	r.LDHLImm16(addr)
	for _, b := range data {
		r.LDAImm(b)
		r.LDHLIA() // LD (HL+), A
	}
}

// WriteTileMap writes a single tile index to the BG tile map.
// row, col are tile coordinates (0-31). mapSelect: false=0x9800, true=0x9C00.
func (r *ROMBuilder) WriteTileMap(row, col int, tileIdx uint8, mapSelect bool) {
	base := uint16(0x9800)
	if mapSelect {
		base = 0x9C00
	}
	addr := base + uint16(row)*32 + uint16(col)
	r.LDAImm(tileIdx)
	r.LDAddr16A(addr)
}

// FillTileMap fills the entire 32x32 tile map with a single tile index.
func (r *ROMBuilder) FillTileMap(tileIdx uint8, mapSelect bool) {
	base := uint16(0x1800) // VRAM offset
	if mapSelect {
		base = 0x1C00
	}
	r.FillVRAM(base, tileIdx, 32*32)
}

// WriteOAMEntry writes a sprite entry. oamIndex is 0-39.
// Does a direct write to OAM memory (0xFE00+).
func (r *ROMBuilder) WriteOAMEntry(oamIndex int, y, x, tile, flags uint8) {
	base := uint16(0xFE00) + uint16(oamIndex)*4
	r.LDAImm(y)
	r.LDAddr16A(base)
	r.LDAImm(x)
	r.LDAddr16A(base + 1)
	r.LDAImm(tile)
	r.LDAddr16A(base + 2)
	r.LDAImm(flags)
	r.LDAddr16A(base + 3)
}

// InfiniteLoop emits JR -2 (spin forever).
func (r *ROMBuilder) InfiniteLoop() {
	r.JR(-2)
}

// WriteRawAt writes raw bytes at a specific ROM address without moving the cursor.
func (r *ROMBuilder) WriteRawAt(addr int, data []byte) {
	copy(r.rom[addr:], data)
}
