package internal

import (
	"yukudanshi/gameboy/internal/cartridge"
	"yukudanshi/gameboy/internal/cpu"
	"yukudanshi/gameboy/internal/gpu"
	"yukudanshi/gameboy/internal/memory"
)

// Emulator represents the Game Boy emulator
type Emulator struct {
	Memory    *memory.MemoryMap
	CPU       *cpu.CPU
	GPU       *gpu.GPU
	Cartridge *cartridge.Cartridge
	running   bool
}

// New creates a new emulator instance
func New() *Emulator {
	mem := memory.New()

	emu := &Emulator{
		Memory:  mem,
		CPU:     cpu.New(mem),
		GPU:     gpu.New(mem),
		running: false,
	}

	return emu
}

// LoadCartridge loads a Game Boy ROM
func (e *Emulator) LoadCartridge(filename string) error {
	cart, err := cartridge.LoadFromFile(filename)
	if err != nil {
		return err
	}

	e.Cartridge = cart

	// Load the first ROM bank
	romBank := cart.GetROMBank(0)
	for i, val := range romBank {
		// Write to memory - needs proper banking later
		e.Memory.Write(uint16(i), val)
	}

	return nil
}

// Reset resets the emulator to its initial state
func (e *Emulator) Reset() {
	e.CPU.Reset()
	e.GPU.Reset()
	// Reset other components as needed
}

// Step executes one CPU instruction
func (e *Emulator) Step() int {
	cycles := e.CPU.Step()
	e.GPU.Step(cycles)
	// Update other components
	return cycles
}

// Run starts the emulator
func (e *Emulator) Run() {
	e.running = true
	for e.running {
		e.Step()
	}
}

// Stop stops the emulator
func (e *Emulator) Stop() {
	e.running = false
}
