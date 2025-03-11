package cpu

import "yukudanshi/gameboy/internal/memory"

// CPU represents the Game Boy's CPU (Sharp LR35902)
type CPU struct {
	Registers Registers
	Memory    *memory.MemoryMap
	halted    bool
	stopped   bool
	ime       bool // Interrupt Master Enable flag
}

// New creates a new CPU instance
func New(memory *memory.MemoryMap) *CPU {
	return &CPU{
		Registers: NewRegisters(),
		Memory:    memory,
	}
}

// Reset resets the CPU to its initial state
func (c *CPU) Reset() {
	c.Registers = NewRegisters()
	c.halted = false
	c.stopped = false
	c.ime = false
}

// FetchByte fetches the next byte from memory at the program counter and increments PC
func (c *CPU) FetchByte() uint8 {
	value := c.Memory.Read(c.Registers.PC)
	c.Registers.PC++
	return value
}

// FetchWord fetches the next word (2 bytes) from memory at the program counter and increments PC
func (c *CPU) FetchWord() uint16 {
	value := c.Memory.ReadWord(c.Registers.PC)
	c.Registers.PC += 2
	return value
}

// Step executes one instruction and returns the number of cycles taken
func (c *CPU) Step() int {
	// We'll implement this with all instruction handling later
	opcode := c.FetchByte()
	// Simple placeholder
	return 4 // Most instructions take 4 cycles
}
