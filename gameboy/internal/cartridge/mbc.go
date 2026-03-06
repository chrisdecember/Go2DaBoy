package cartridge

// MBC is the interface for memory bank controllers
type MBC interface {
	ReadROM(addr uint16) uint8
	WriteROM(addr uint16, value uint8)
	ReadRAM(addr uint16) uint8
	WriteRAM(addr uint16, value uint8)
}

// --- MBC0: ROM Only (no banking) ---

type MBC0 struct {
	rom []byte
}

func NewMBC0(rom []byte) *MBC0 {
	return &MBC0{rom: rom}
}

func (m *MBC0) ReadROM(addr uint16) uint8 {
	if int(addr) < len(m.rom) {
		return m.rom[addr]
	}
	return 0xFF
}

func (m *MBC0) WriteROM(addr uint16, value uint8) {
	// ROM only - writes are ignored
}

func (m *MBC0) ReadRAM(addr uint16) uint8 {
	return 0xFF
}

func (m *MBC0) WriteRAM(addr uint16, value uint8) {
	// No external RAM
}

// --- MBC1: up to 2MB ROM / 32KB RAM ---

type MBC1 struct {
	rom        []byte
	ram        []byte
	romBank    uint8
	ramBank    uint8
	ramEnabled bool
	bankMode   uint8 // 0=ROM banking, 1=RAM banking
	hasRAM     bool
}

func NewMBC1(rom []byte, ramSize int) *MBC1 {
	m := &MBC1{
		rom:     rom,
		romBank: 1,
		hasRAM:  ramSize > 0,
	}
	if ramSize > 0 {
		m.ram = make([]byte, ramSize)
	}
	return m
}

func (m *MBC1) ReadROM(addr uint16) uint8 {
	if addr < 0x4000 {
		// Bank 0 (or adjusted bank in mode 1)
		bank := 0
		if m.bankMode == 1 {
			bank = int(m.ramBank) << 5
		}
		offset := bank*0x4000 + int(addr)
		if offset < len(m.rom) {
			return m.rom[offset]
		}
		return 0xFF
	}
	// Bank N (0x4000-0x7FFF)
	bank := int(m.romBank) | (int(m.ramBank) << 5)
	offset := bank*0x4000 + int(addr-0x4000)
	offset %= len(m.rom)
	return m.rom[offset]
}

func (m *MBC1) WriteROM(addr uint16, value uint8) {
	switch {
	case addr < 0x2000:
		// RAM enable
		m.ramEnabled = (value & 0x0F) == 0x0A
	case addr < 0x4000:
		// ROM bank number (lower 5 bits)
		bank := value & 0x1F
		if bank == 0 {
			bank = 1
		}
		m.romBank = bank
	case addr < 0x6000:
		// RAM bank / upper ROM bank bits
		m.ramBank = value & 0x03
	default:
		// Banking mode select
		m.bankMode = value & 0x01
	}
}

func (m *MBC1) ReadRAM(addr uint16) uint8 {
	if !m.ramEnabled || !m.hasRAM {
		return 0xFF
	}
	bank := 0
	if m.bankMode == 1 {
		bank = int(m.ramBank)
	}
	offset := bank*0x2000 + int(addr-0xA000)
	if offset < len(m.ram) {
		return m.ram[offset]
	}
	return 0xFF
}

func (m *MBC1) WriteRAM(addr uint16, value uint8) {
	if !m.ramEnabled || !m.hasRAM {
		return
	}
	bank := 0
	if m.bankMode == 1 {
		bank = int(m.ramBank)
	}
	offset := bank*0x2000 + int(addr-0xA000)
	if offset < len(m.ram) {
		m.ram[offset] = value
	}
}

// --- MBC2: up to 256KB ROM / 512x4-bit RAM ---

type MBC2 struct {
	rom        []byte
	ram        [512]uint8 // 512 x 4-bit RAM (only lower nibble used)
	romBank    uint8
	ramEnabled bool
}

func NewMBC2(rom []byte) *MBC2 {
	return &MBC2{
		rom:     rom,
		romBank: 1,
	}
}

func (m *MBC2) ReadROM(addr uint16) uint8 {
	if addr < 0x4000 {
		if int(addr) < len(m.rom) {
			return m.rom[addr]
		}
		return 0xFF
	}
	offset := int(m.romBank)*0x4000 + int(addr-0x4000)
	if len(m.rom) > 0 {
		offset %= len(m.rom)
		return m.rom[offset]
	}
	return 0xFF
}

func (m *MBC2) WriteROM(addr uint16, value uint8) {
	// MBC2 uses bit 8 of address to distinguish RAM enable vs ROM bank
	if addr < 0x4000 {
		if addr&0x0100 == 0 {
			// RAM enable (bit 8 = 0)
			m.ramEnabled = (value & 0x0F) == 0x0A
		} else {
			// ROM bank (bit 8 = 1), 4-bit register
			bank := value & 0x0F
			if bank == 0 {
				bank = 1
			}
			m.romBank = bank
		}
	}
}

func (m *MBC2) ReadRAM(addr uint16) uint8 {
	if !m.ramEnabled {
		return 0xFF
	}
	// MBC2 has 512 addresses, mapped to A000-A1FF, with echo up to BFFF
	offset := int(addr-0xA000) & 0x1FF
	return m.ram[offset] | 0xF0 // Upper nibble reads as 1
}

func (m *MBC2) WriteRAM(addr uint16, value uint8) {
	if !m.ramEnabled {
		return
	}
	offset := int(addr-0xA000) & 0x1FF
	m.ram[offset] = value & 0x0F // Only lower nibble is writable
}

// --- MBC3: up to 2MB ROM / 32KB RAM + RTC ---

type MBC3 struct {
	rom        []byte
	ram        []byte
	romBank    uint8
	ramBank    uint8
	ramEnabled bool
	hasRAM     bool
	// RTC registers (simplified)
	rtcRegisters [5]uint8
	rtcMapped    bool
}

func NewMBC3(rom []byte, ramSize int) *MBC3 {
	m := &MBC3{
		rom:     rom,
		romBank: 1,
		hasRAM:  ramSize > 0,
	}
	if ramSize > 0 {
		m.ram = make([]byte, ramSize)
	}
	return m
}

func (m *MBC3) ReadROM(addr uint16) uint8 {
	if addr < 0x4000 {
		if int(addr) < len(m.rom) {
			return m.rom[addr]
		}
		return 0xFF
	}
	bank := int(m.romBank)
	if bank == 0 {
		bank = 1
	}
	offset := bank*0x4000 + int(addr-0x4000)
	if len(m.rom) > 0 {
		offset %= len(m.rom)
		return m.rom[offset]
	}
	return 0xFF
}

func (m *MBC3) WriteROM(addr uint16, value uint8) {
	switch {
	case addr < 0x2000:
		m.ramEnabled = (value & 0x0F) == 0x0A
	case addr < 0x4000:
		bank := value & 0x7F
		if bank == 0 {
			bank = 1
		}
		m.romBank = bank
	case addr < 0x6000:
		if value <= 0x03 {
			m.ramBank = value
			m.rtcMapped = false
		} else if value >= 0x08 && value <= 0x0C {
			m.rtcMapped = true
			m.ramBank = value
		}
	default:
		// RTC latch - simplified
	}
}

func (m *MBC3) ReadRAM(addr uint16) uint8 {
	if !m.ramEnabled {
		return 0xFF
	}
	if m.rtcMapped {
		idx := int(m.ramBank) - 0x08
		if idx >= 0 && idx < 5 {
			return m.rtcRegisters[idx]
		}
		return 0xFF
	}
	if !m.hasRAM {
		return 0xFF
	}
	offset := int(m.ramBank)*0x2000 + int(addr-0xA000)
	if offset < len(m.ram) {
		return m.ram[offset]
	}
	return 0xFF
}

func (m *MBC3) WriteRAM(addr uint16, value uint8) {
	if !m.ramEnabled {
		return
	}
	if m.rtcMapped {
		idx := int(m.ramBank) - 0x08
		if idx >= 0 && idx < 5 {
			m.rtcRegisters[idx] = value
		}
		return
	}
	if !m.hasRAM {
		return
	}
	offset := int(m.ramBank)*0x2000 + int(addr-0xA000)
	if offset < len(m.ram) {
		m.ram[offset] = value
	}
}

// --- MBC5: up to 8MB ROM / 128KB RAM ---

type MBC5 struct {
	rom        []byte
	ram        []byte
	romBankLo  uint8
	romBankHi  uint8
	ramBank    uint8
	ramEnabled bool
	hasRAM     bool
}

func NewMBC5(rom []byte, ramSize int) *MBC5 {
	m := &MBC5{
		rom:    rom,
		hasRAM: ramSize > 0,
	}
	if ramSize > 0 {
		m.ram = make([]byte, ramSize)
	}
	return m
}

func (m *MBC5) romBank() int {
	return int(m.romBankHi)<<8 | int(m.romBankLo)
}

func (m *MBC5) ReadROM(addr uint16) uint8 {
	if addr < 0x4000 {
		if int(addr) < len(m.rom) {
			return m.rom[addr]
		}
		return 0xFF
	}
	offset := m.romBank()*0x4000 + int(addr-0x4000)
	if len(m.rom) > 0 {
		offset %= len(m.rom)
	}
	if offset < len(m.rom) {
		return m.rom[offset]
	}
	return 0xFF
}

func (m *MBC5) WriteROM(addr uint16, value uint8) {
	switch {
	case addr < 0x2000:
		m.ramEnabled = (value & 0x0F) == 0x0A
	case addr < 0x3000:
		m.romBankLo = value
	case addr < 0x4000:
		m.romBankHi = value & 0x01
	case addr < 0x6000:
		m.ramBank = value & 0x0F
	}
}

func (m *MBC5) ReadRAM(addr uint16) uint8 {
	if !m.ramEnabled || !m.hasRAM {
		return 0xFF
	}
	offset := int(m.ramBank)*0x2000 + int(addr-0xA000)
	if offset < len(m.ram) {
		return m.ram[offset]
	}
	return 0xFF
}

func (m *MBC5) WriteRAM(addr uint16, value uint8) {
	if !m.ramEnabled || !m.hasRAM {
		return
	}
	offset := int(m.ramBank)*0x2000 + int(addr-0xA000)
	if offset < len(m.ram) {
		m.ram[offset] = value
	}
}
