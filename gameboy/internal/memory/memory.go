package memory

// MemoryMap represents the Game Boy's memory map
type MemoryMap struct {
	// ROM Banks
	rom [0x8000]uint8 // 32KB ROM
	// Video RAM
	vram [0x2000]uint8 // 8KB Video RAM
	// External RAM
	eram [0x2000]uint8 // 8KB External RAM
	// Work RAM
	wram [0x2000]uint8 // 8KB Work RAM
	// OAM
	oam [0x100]uint8 // Object Attribute Memory
	// High RAM
	hram [0x80]uint8 // High RAM
	// I/O Registers
	io [0x80]uint8 // I/O Registers
}

// New creates a new memory map
func New() *MemoryMap {
	return &MemoryMap{}
}

// Read reads a byte from memory
func (m *MemoryMap) Read(addr uint16) uint8 {
	switch {
	case addr < 0x8000:
		return m.rom[addr]
	case addr < 0xA000:
		return m.vram[addr-0x8000]
	case addr < 0xC000:
		return m.eram[addr-0xA000]
	case addr < 0xE000:
		return m.wram[addr-0xC000]
	case addr < 0xFE00:
		return m.wram[addr-0xE000] // Echo RAM
	case addr < 0xFEA0:
		return m.oam[addr-0xFE00]
	case addr < 0xFF00:
		return 0 // Unusable memory
	case addr < 0xFF80:
		return m.io[addr-0xFF00]
	case addr < 0xFFFF:
		return m.hram[addr-0xFF80]
	default:
		return m.io[0x7F] // 0xFFFF - Interrupt Enable register
	}
}

// Write writes a byte to memory
func (m *MemoryMap) Write(addr uint16, value uint8) {
	switch {
	case addr < 0x8000:
		m.rom[addr] = value // Note: ROM shouldn't be writable in final implementation
	case addr < 0xA000:
		m.vram[addr-0x8000] = value
	case addr < 0xC000:
		m.eram[addr-0xA000] = value
	case addr < 0xE000:
		m.wram[addr-0xC000] = value
	case addr < 0xFE00:
		m.wram[addr-0xE000] = value // Echo RAM
	case addr < 0xFEA0:
		m.oam[addr-0xFE00] = value
	case addr < 0xFF00:
		return // Unusable memory
	case addr < 0xFF80:
		m.io[addr-0xFF00] = value
	case addr < 0xFFFF:
		m.hram[addr-0xFF80] = value
	default:
		m.io[0x7F] = value // 0xFFFF - Interrupt Enable register
	}
}

// ReadWord reads a 16-bit word from memory (little endian)
func (m *MemoryMap) ReadWord(addr uint16) uint16 {
	low := uint16(m.Read(addr))
	high := uint16(m.Read(addr + 1))
	return (high << 8) | low
}

// WriteWord writes a 16-bit word to memory (little endian)
func (m *MemoryMap) WriteWord(addr uint16, value uint16) {
	m.Write(addr, uint8(value&0xFF))
	m.Write(addr+1, uint8(value>>8))
}
