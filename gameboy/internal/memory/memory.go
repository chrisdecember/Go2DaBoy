package memory

import (
	"go2daboy/gameboy/internal/apu"
	"go2daboy/gameboy/internal/cartridge"
	"go2daboy/gameboy/internal/joypad"
	"go2daboy/gameboy/internal/ppu"
	"go2daboy/gameboy/internal/timer"
)

// Bus is the central memory bus routing reads/writes to the appropriate component.
type Bus struct {
	Cart   *cartridge.Cartridge
	PPU    *ppu.PPU
	APU    *apu.APU
	Timer  *timer.Timer
	Joypad *joypad.Joypad

	wram  [0x2000]uint8 // Work RAM (8KB)
	hram  [0x7F]uint8   // High RAM (127 bytes)
	ie    uint8         // Interrupt Enable (0xFFFF)
	ifReg uint8         // Interrupt Flag (0xFF0F)

	serial [2]uint8 // 0xFF01-0xFF02 Serial transfer

	// Serial output callback (for test harness / link cable)
	SerialCallback func(byte)

	// DMA state
	dmaActive  bool
	dmaSource  uint16
	dmaIndex   uint8
	dmaCycles  int
}

func New(p *ppu.PPU, a *apu.APU, t *timer.Timer, j *joypad.Joypad) *Bus {
	return &Bus{
		PPU:    p,
		APU:    a,
		Timer:  t,
		Joypad: j,
		ifReg:  0xE1,
	}
}

func (b *Bus) Reset() {
	b.wram = [0x2000]uint8{}
	b.hram = [0x7F]uint8{}
	b.ie = 0
	b.ifReg = 0xE1
	b.serial = [2]uint8{}
	b.dmaActive = false
	b.dmaIndex = 0
	b.dmaCycles = 0
}

// RequestInterrupt sets a bit in the IF register
func (b *Bus) RequestInterrupt(bit uint8) {
	b.ifReg |= bit
}

// GetIF returns the interrupt flag register
func (b *Bus) GetIF() uint8 {
	return b.ifReg
}

// GetIE returns the interrupt enable register
func (b *Bus) GetIE() uint8 {
	return b.ie
}

// StepDMA advances DMA by the given T-cycles
func (b *Bus) StepDMA(cycles int) {
	if !b.dmaActive {
		return
	}

	b.dmaCycles += cycles
	for b.dmaCycles >= 4 && b.dmaIndex < 0xA0 {
		b.dmaCycles -= 4
		// Read from source (using direct read to bypass DMA restrictions)
		val := b.dmaRead(b.dmaSource + uint16(b.dmaIndex))
		b.PPU.DirectWriteOAM(b.dmaIndex, val)
		b.dmaIndex++
	}

	if b.dmaIndex >= 0xA0 {
		b.dmaActive = false
	}
}

// dmaRead reads bypassing DMA access restrictions
func (b *Bus) dmaRead(addr uint16) uint8 {
	switch {
	case addr < 0x8000:
		if b.Cart != nil {
			return b.Cart.MBC.ReadROM(addr)
		}
		return 0xFF
	case addr < 0xA000:
		return b.PPU.VRAM[addr-0x8000]
	case addr < 0xC000:
		if b.Cart != nil {
			return b.Cart.MBC.ReadRAM(addr)
		}
		return 0xFF
	case addr < 0xE000:
		return b.wram[addr-0xC000]
	default:
		return 0xFF
	}
}

func (b *Bus) Read(addr uint16) uint8 {
	// During DMA, CPU can only access HRAM (0xFF80-0xFFFE) and IE (0xFFFF)
	if b.dmaActive && addr < 0xFF80 {
		return 0xFF
	}

	switch {
	case addr < 0x8000:
		if b.Cart != nil {
			return b.Cart.MBC.ReadROM(addr)
		}
		return 0xFF

	case addr < 0xA000:
		return b.PPU.ReadVRAM(addr - 0x8000)

	case addr < 0xC000:
		if b.Cart != nil {
			return b.Cart.MBC.ReadRAM(addr)
		}
		return 0xFF

	case addr < 0xE000:
		return b.wram[addr-0xC000]

	case addr < 0xFE00:
		return b.wram[addr-0xE000]

	case addr < 0xFEA0:
		return b.PPU.ReadOAM(addr - 0xFE00)

	case addr < 0xFF00:
		return 0xFF

	case addr < 0xFF80:
		return b.readIO(addr)

	case addr < 0xFFFF:
		return b.hram[addr-0xFF80]

	default:
		return b.ie
	}
}

func (b *Bus) Write(addr uint16, value uint8) {
	// During DMA, CPU can only access HRAM (0xFF80-0xFFFE) and IE (0xFFFF)
	if b.dmaActive && addr < 0xFF80 {
		return
	}

	switch {
	case addr < 0x8000:
		if b.Cart != nil {
			b.Cart.MBC.WriteROM(addr, value)
		}

	case addr < 0xA000:
		b.PPU.WriteVRAM(addr-0x8000, value)

	case addr < 0xC000:
		if b.Cart != nil {
			b.Cart.MBC.WriteRAM(addr, value)
		}

	case addr < 0xE000:
		b.wram[addr-0xC000] = value

	case addr < 0xFE00:
		b.wram[addr-0xE000] = value

	case addr < 0xFEA0:
		b.PPU.WriteOAM(addr-0xFE00, value)

	case addr < 0xFF00:
		// Unusable

	case addr < 0xFF80:
		b.writeIO(addr, value)

	case addr < 0xFFFF:
		b.hram[addr-0xFF80] = value

	default:
		b.ie = value
	}
}

func (b *Bus) readIO(addr uint16) uint8 {
	switch {
	case addr == 0xFF00:
		return b.Joypad.Read()

	case addr == 0xFF01:
		return b.serial[0]
	case addr == 0xFF02:
		return b.serial[1]

	case addr >= 0xFF04 && addr <= 0xFF07:
		return b.Timer.Read(addr)

	case addr == 0xFF0F:
		return b.ifReg | 0xE0

	case addr >= 0xFF10 && addr <= 0xFF3F:
		return b.APU.Read(addr)

	case addr >= 0xFF40 && addr <= 0xFF4B:
		return b.PPU.Read(addr)

	default:
		return 0xFF
	}
}

func (b *Bus) writeIO(addr uint16, value uint8) {
	switch {
	case addr == 0xFF00:
		b.Joypad.Write(value)

	case addr == 0xFF01:
		b.serial[0] = value
	case addr == 0xFF02:
		b.serial[1] = value
		// When bit 7 is set (transfer start) with internal clock (bit 0),
		// auto-complete the transfer for Blargg test compatibility
		if value&0x81 == 0x81 {
			if b.SerialCallback != nil {
				b.SerialCallback(b.serial[0])
			}
			b.serial[0] = 0xFF // No link cable connected
			b.serial[1] &= 0x7F // Clear transfer start flag
			b.ifReg |= 0x08     // Request serial interrupt (bit 3)
		}

	case addr >= 0xFF04 && addr <= 0xFF07:
		b.Timer.Write(addr, value)

	case addr == 0xFF0F:
		b.ifReg = value & 0x1F

	case addr >= 0xFF10 && addr <= 0xFF3F:
		b.APU.Write(addr, value)

	case addr >= 0xFF40 && addr <= 0xFF4B:
		b.PPU.Write(addr, value)
		if addr == 0xFF46 {
			b.startDMA(value)
		}

	case addr == 0xFF50:
		// Boot ROM disable
	}
}

// startDMA begins an OAM DMA transfer (160 bytes, 640 T-cycles + 8 T-cycle startup delay)
func (b *Bus) startDMA(value uint8) {
	b.dmaActive = true
	b.dmaSource = uint16(value) << 8
	b.dmaIndex = 0
	b.dmaCycles = -8 // 2 M-cycle startup delay before first byte transfer
}

// ReadWord reads a 16-bit word (little endian)
func (b *Bus) ReadWord(addr uint16) uint16 {
	lo := uint16(b.Read(addr))
	hi := uint16(b.Read(addr + 1))
	return hi<<8 | lo
}

// WriteWord writes a 16-bit word (little endian)
func (b *Bus) WriteWord(addr uint16, value uint16) {
	b.Write(addr, uint8(value&0xFF))
	b.Write(addr+1, uint8(value>>8))
}
