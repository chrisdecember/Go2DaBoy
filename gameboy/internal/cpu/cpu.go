package cpu

import "go2daboy/gameboy/internal/memory"

// CPU represents the Sharp LR35902 processor
type CPU struct {
	Regs    Registers
	Bus     *memory.Bus
	halted  bool
	stopped bool
	ime     bool // Interrupt Master Enable
	eiDelay bool // EI enables IME after next instruction
	haltBug bool // HALT bug: next PC increment is skipped
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
}

// Step executes one instruction and returns T-cycles consumed
func (c *CPU) Step() int {
	// Handle interrupts BEFORE EI takes effect
	// This ensures the instruction after EI executes before interrupts fire
	if cycles := c.handleInterrupts(); cycles > 0 {
		return cycles
	}

	// If halted, consume 4 cycles doing nothing
	if c.halted {
		return 4
	}

	// Fetch and execute opcode
	opcode := c.fetchByte()
	cycles := c.execute(opcode)

	// Apply pending EI AFTER instruction execution
	// EI delays IME enable by one instruction per Pan Docs
	if c.eiDelay {
		c.eiDelay = false
		c.ime = true
	}

	return cycles
}

func (c *CPU) handleInterrupts() int {
	ifReg := c.Bus.GetIF()
	ieReg := c.Bus.GetIE()
	pending := ifReg & ieReg & 0x1F

	if pending == 0 {
		return 0
	}

	// Any pending interrupt wakes from HALT regardless of IME
	if c.halted {
		c.halted = false
	}

	if !c.ime {
		return 0
	}

	// Service highest priority interrupt
	c.ime = false
	for bit := uint8(0); bit < 5; bit++ {
		mask := uint8(1 << bit)
		if pending&mask != 0 {
			// Clear this interrupt flag bit
			c.Bus.Write(0xFF0F, ifReg&^mask)

			// Push PC and jump to interrupt vector
			c.push(c.Regs.PC)
			c.Regs.PC = 0x0040 + uint16(bit)*8
			return 20
		}
	}
	return 0
}

// fetchByte reads a byte at PC and increments PC
func (c *CPU) fetchByte() uint8 {
	val := c.Bus.Read(c.Regs.PC)
	if c.haltBug {
		// HALT bug: PC fails to increment, causing the next byte to be read twice
		c.haltBug = false
	} else {
		c.Regs.PC++
	}
	return val
}

// fetchWord reads a 16-bit word at PC (little endian) and increments PC by 2
func (c *CPU) fetchWord() uint16 {
	lo := uint16(c.fetchByte())
	hi := uint16(c.fetchByte())
	return hi<<8 | lo
}

// Stack operations - push writes high byte first (SP-1), then low byte (SP-2)
func (c *CPU) push(value uint16) {
	c.Regs.SP--
	c.Bus.Write(c.Regs.SP, uint8(value>>8)) // High byte
	c.Regs.SP--
	c.Bus.Write(c.Regs.SP, uint8(value&0xFF)) // Low byte
}

func (c *CPU) pop() uint16 {
	val := c.Bus.ReadWord(c.Regs.SP)
	c.Regs.SP += 2
	return val
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
	// Half carry and carry are based on lower byte addition
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
		// After addition: correct low nibble FIRST, then high nibble
		// Low nibble must be checked on original value before high nibble adjustment
		if c.Regs.GetFlag(FlagH) || (a&0x0F) > 0x09 {
			a += 0x06
		}
		if c.Regs.GetFlag(FlagC) || a > 0x99 {
			a += 0x60
			c.Regs.SetFlag(FlagC, true)
		}
	} else {
		// After subtraction
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
