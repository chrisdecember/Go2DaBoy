package cpu

import "go2daboy/emulator/internal/memory"

// CPU represents the Sharp LR35902 processor
type CPU struct {
	Regs    Registers
	Bus     *memory.Bus
	halted  bool
	stopped bool
	ime     bool // Interrupt Master Enable
	eiDelay bool // EI enables IME after next instruction
	haltBug bool // HALT bug: next PC increment is skipped
	cycles  int  // T-cycle accumulator for current Step
}

func New(bus *memory.Bus) *CPU {
	return &CPU{
		Regs: NewRegisters(),
		Bus:  bus,
	}
}

func (c *CPU) Reset() {
	c.Regs = NewRegisters()
	c.halted = false
	c.stopped = false
	c.ime = false
	c.eiDelay = false
	c.haltBug = false
	c.cycles = 0
}

// tick advances all subsystems by 1 M-cycle (4 T-cycles)
func (c *CPU) tick() {
	c.cycles += 4
	c.Bus.Tick(4)
}

// read performs a bus read and ticks 1 M-cycle
func (c *CPU) read(addr uint16) uint8 {
	val := c.Bus.Read(addr)
	c.tick()
	return val
}

// write performs a bus write and ticks 1 M-cycle
func (c *CPU) write(addr uint16, val uint8) {
	c.Bus.Write(addr, val)
	c.tick()
}

// idle ticks 1 M-cycle with no bus activity
func (c *CPU) idle() {
	c.tick()
}

// Step executes one instruction and returns T-cycles consumed.
// Subsystems (PPU, Timer, APU, DMA) are advanced inline via Bus.Tick
// after each M-cycle, giving M-cycle-accurate interleaving.
func (c *CPU) Step() int {
	c.cycles = 0

	// Handle interrupts BEFORE EI takes effect
	if c.handleInterrupts() {
		return c.cycles
	}

	// If halted, consume 1 M-cycle doing nothing (bus still advances)
	if c.halted {
		c.tick()
		return c.cycles
	}

	// Fetch and execute opcode (fetchByte ticks 1 M-cycle for the fetch)
	opcode := c.fetchByte()
	c.execute(opcode)

	// Apply pending EI AFTER instruction execution
	if c.eiDelay {
		c.eiDelay = false
		c.ime = true
	}

	return c.cycles
}

func (c *CPU) handleInterrupts() bool {
	ifReg := c.Bus.GetIF()
	ieReg := c.Bus.GetIE()
	pending := ifReg & ieReg & 0x1F

	if pending == 0 {
		return false
	}

	// Any pending interrupt wakes from HALT regardless of IME
	if c.halted {
		c.halted = false
		c.tick() // wake-up M-cycle
	}

	if !c.ime {
		return false
	}

	// Service highest priority interrupt
	// Dispatch timing: 2 idle + push_hi + push_lo + 1 idle = 5 M-cycles (20T)
	c.ime = false
	for bit := uint8(0); bit < 5; bit++ {
		mask := uint8(1 << bit)
		if pending&mask != 0 {
			c.Bus.ClearInterruptBit(mask)

			c.idle() // internal M-cycle 1
			c.idle() // internal M-cycle 2
			c.Regs.SP--
			c.write(c.Regs.SP, uint8(c.Regs.PC>>8)) // push PC high
			c.Regs.SP--
			c.write(c.Regs.SP, uint8(c.Regs.PC&0xFF)) // push PC low
			c.Regs.PC = 0x0040 + uint16(bit)*8
			c.idle() // internal M-cycle 5
			return true
		}
	}
	return false
}

// fetchByte reads a byte at PC, increments PC, and ticks 1 M-cycle
func (c *CPU) fetchByte() uint8 {
	val := c.Bus.Read(c.Regs.PC)
	if c.haltBug {
		c.haltBug = false
	} else {
		c.Regs.PC++
	}
	c.tick()
	return val
}

// fetchWord reads a 16-bit word at PC (little endian), ticks 2 M-cycles
func (c *CPU) fetchWord() uint16 {
	lo := uint16(c.fetchByte())
	hi := uint16(c.fetchByte())
	return hi<<8 | lo
}

// push writes a 16-bit value to the stack (high byte first), ticks 2 M-cycles
func (c *CPU) push(value uint16) {
	c.Regs.SP--
	c.write(c.Regs.SP, uint8(value>>8)) // High byte
	c.Regs.SP--
	c.write(c.Regs.SP, uint8(value&0xFF)) // Low byte
}

// pop reads a 16-bit value from the stack, ticks 2 M-cycles
func (c *CPU) pop() uint16 {
	lo := uint16(c.read(c.Regs.SP))
	c.Regs.SP++
	hi := uint16(c.read(c.Regs.SP))
	c.Regs.SP++
	return hi<<8 | lo
}

// --- ALU Helpers ---

func (c *CPU) add(val uint8) {
	a := c.Regs.A
	result := uint16(a) + uint16(val)
	c.Regs.A = uint8(result)
	c.Regs.SetFlag(FlagZ, c.Regs.A == 0)
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, (a&0x0F)+(val&0x0F) > 0x0F)
	c.Regs.SetFlag(FlagC, result > 0xFF)
}

func (c *CPU) adc(val uint8) {
	carry := uint8(0)
	if c.Regs.GetFlag(FlagC) {
		carry = 1
	}
	a := c.Regs.A
	result := uint16(a) + uint16(val) + uint16(carry)
	c.Regs.A = uint8(result)
	c.Regs.SetFlag(FlagZ, c.Regs.A == 0)
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, (a&0x0F)+(val&0x0F)+carry > 0x0F)
	c.Regs.SetFlag(FlagC, result > 0xFF)
}

func (c *CPU) sub(val uint8) {
	a := c.Regs.A
	result := a - val
	c.Regs.A = result
	c.Regs.SetFlag(FlagZ, result == 0)
	c.Regs.SetFlag(FlagN, true)
	c.Regs.SetFlag(FlagH, (a&0x0F) < (val&0x0F))
	c.Regs.SetFlag(FlagC, a < val)
}

func (c *CPU) sbc(val uint8) {
	carry := uint8(0)
	if c.Regs.GetFlag(FlagC) {
		carry = 1
	}
	a := c.Regs.A
	result := int16(a) - int16(val) - int16(carry)
	c.Regs.A = uint8(result)
	c.Regs.SetFlag(FlagZ, uint8(result) == 0)
	c.Regs.SetFlag(FlagN, true)
	c.Regs.SetFlag(FlagH, int16(a&0x0F)-int16(val&0x0F)-int16(carry) < 0)
	c.Regs.SetFlag(FlagC, result < 0)
}

func (c *CPU) and(val uint8) {
	c.Regs.A &= val
	c.Regs.SetFlag(FlagZ, c.Regs.A == 0)
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, true)
	c.Regs.SetFlag(FlagC, false)
}

func (c *CPU) xor(val uint8) {
	c.Regs.A ^= val
	c.Regs.SetFlag(FlagZ, c.Regs.A == 0)
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, false)
	c.Regs.SetFlag(FlagC, false)
}

func (c *CPU) or(val uint8) {
	c.Regs.A |= val
	c.Regs.SetFlag(FlagZ, c.Regs.A == 0)
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, false)
	c.Regs.SetFlag(FlagC, false)
}

func (c *CPU) cp(val uint8) {
	a := c.Regs.A
	result := a - val
	c.Regs.SetFlag(FlagZ, result == 0)
	c.Regs.SetFlag(FlagN, true)
	c.Regs.SetFlag(FlagH, (a&0x0F) < (val&0x0F))
	c.Regs.SetFlag(FlagC, a < val)
}

func (c *CPU) inc(val uint8) uint8 {
	result := val + 1
	c.Regs.SetFlag(FlagZ, result == 0)
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, (val&0x0F)+1 > 0x0F)
	return result
}

func (c *CPU) dec(val uint8) uint8 {
	result := val - 1
	c.Regs.SetFlag(FlagZ, result == 0)
	c.Regs.SetFlag(FlagN, true)
	c.Regs.SetFlag(FlagH, val&0x0F == 0)
	return result
}

func (c *CPU) addHL(val uint16) {
	hl := c.Regs.GetHL()
	result := uint32(hl) + uint32(val)
	c.Regs.SetHL(uint16(result))
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, (hl&0x0FFF)+(val&0x0FFF) > 0x0FFF)
	c.Regs.SetFlag(FlagC, result > 0xFFFF)
}

func (c *CPU) addSPSigned(val int8) uint16 {
	sp := c.Regs.SP
	offset := uint16(val)
	result := sp + offset
	c.Regs.SetFlag(FlagZ, false)
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, (sp&0x0F)+(offset&0x0F) > 0x0F)
	c.Regs.SetFlag(FlagC, (sp&0xFF)+(offset&0xFF) > 0xFF)
	return result
}

// --- Rotation/Shift helpers ---

func (c *CPU) rlc(val uint8) uint8 {
	carry := val >> 7
	result := (val << 1) | carry
	c.Regs.SetFlag(FlagZ, result == 0)
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, false)
	c.Regs.SetFlag(FlagC, carry != 0)
	return result
}

func (c *CPU) rrc(val uint8) uint8 {
	carry := val & 1
	result := (val >> 1) | (carry << 7)
	c.Regs.SetFlag(FlagZ, result == 0)
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, false)
	c.Regs.SetFlag(FlagC, carry != 0)
	return result
}

func (c *CPU) rl(val uint8) uint8 {
	oldCarry := uint8(0)
	if c.Regs.GetFlag(FlagC) {
		oldCarry = 1
	}
	carry := val >> 7
	result := (val << 1) | oldCarry
	c.Regs.SetFlag(FlagZ, result == 0)
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, false)
	c.Regs.SetFlag(FlagC, carry != 0)
	return result
}

func (c *CPU) rr(val uint8) uint8 {
	oldCarry := uint8(0)
	if c.Regs.GetFlag(FlagC) {
		oldCarry = 1
	}
	carry := val & 1
	result := (val >> 1) | (oldCarry << 7)
	c.Regs.SetFlag(FlagZ, result == 0)
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, false)
	c.Regs.SetFlag(FlagC, carry != 0)
	return result
}

func (c *CPU) sla(val uint8) uint8 {
	carry := val >> 7
	result := val << 1
	c.Regs.SetFlag(FlagZ, result == 0)
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, false)
	c.Regs.SetFlag(FlagC, carry != 0)
	return result
}

func (c *CPU) sra(val uint8) uint8 {
	carry := val & 1
	result := (val >> 1) | (val & 0x80)
	c.Regs.SetFlag(FlagZ, result == 0)
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, false)
	c.Regs.SetFlag(FlagC, carry != 0)
	return result
}

func (c *CPU) srl(val uint8) uint8 {
	carry := val & 1
	result := val >> 1
	c.Regs.SetFlag(FlagZ, result == 0)
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, false)
	c.Regs.SetFlag(FlagC, carry != 0)
	return result
}

func (c *CPU) swap(val uint8) uint8 {
	result := (val>>4)&0x0F | (val<<4)&0xF0
	c.Regs.SetFlag(FlagZ, result == 0)
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, false)
	c.Regs.SetFlag(FlagC, false)
	return result
}

func (c *CPU) bit(b uint8, val uint8) {
	c.Regs.SetFlag(FlagZ, val&(1<<b) == 0)
	c.Regs.SetFlag(FlagN, false)
	c.Regs.SetFlag(FlagH, true)
}

func (c *CPU) daa() {
	a := c.Regs.A
	if !c.Regs.GetFlag(FlagN) {
		if c.Regs.GetFlag(FlagC) || a > 0x99 {
			a += 0x60
			c.Regs.SetFlag(FlagC, true)
		}
		if c.Regs.GetFlag(FlagH) || (a&0x0F) > 0x09 {
			a += 0x06
		}
	} else {
		if c.Regs.GetFlag(FlagC) {
			a -= 0x60
		}
		if c.Regs.GetFlag(FlagH) {
			a -= 0x06
		}
	}
	c.Regs.A = a
	c.Regs.SetFlag(FlagZ, a == 0)
	c.Regs.SetFlag(FlagH, false)
}

// IsHalted returns whether the CPU is in HALT state
func (c *CPU) IsHalted() bool {
	return c.halted
}

// IsStopped returns whether the CPU is in STOP state
func (c *CPU) IsStopped() bool {
	return c.stopped
}
