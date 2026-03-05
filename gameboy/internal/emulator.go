package internal

import (
	"yukudanshi/gameboy/internal/apu"
	"yukudanshi/gameboy/internal/cartridge"
	"yukudanshi/gameboy/internal/cpu"
	"yukudanshi/gameboy/internal/joypad"
	"yukudanshi/gameboy/internal/memory"
	"yukudanshi/gameboy/internal/ppu"
	"yukudanshi/gameboy/internal/timer"
)

const cyclesPerFrame = 70224 // T-cycles per frame (~59.7 FPS)

// Emulator represents the Game Boy emulator
type Emulator struct {
	Bus     *memory.Bus
	CPU     *cpu.CPU
	PPU     *ppu.PPU
	APU     *apu.APU
	Timer   *timer.Timer
	Joypad  *joypad.Joypad
	Cart    *cartridge.Cartridge
	running bool
}

// New creates a new emulator instance
func New() *Emulator {
	p := ppu.New()
	a := apu.New()
	t := timer.New()
	j := joypad.New()
	bus := memory.New(p, a, t, j)

	emu := &Emulator{
		Bus:    bus,
		CPU:    cpu.New(bus),
		PPU:    p,
		APU:    a,
		Timer:  t,
		Joypad: j,
	}

	return emu
}

// LoadCartridge loads a ROM file
func (e *Emulator) LoadCartridge(filename string) error {
	cart, err := cartridge.LoadFromFile(filename)
	if err != nil {
		return err
	}
	e.Cart = cart
	e.Bus.Cart = cart
	return nil
}

// LoadROM loads a ROM from bytes
func (e *Emulator) LoadROM(data []byte) error {
	cart, err := cartridge.LoadFromBytes(data)
	if err != nil {
		return err
	}
	e.Cart = cart
	e.Bus.Cart = cart
	return nil
}

// Reset resets the emulator
func (e *Emulator) Reset() {
	e.CPU.Reset()
	e.PPU.Reset()
	e.APU.Reset()
	e.Timer.Reset()
	e.Joypad.Reset()
	e.Bus.Reset()
	// Re-attach cartridge after bus reset
	if e.Cart != nil {
		e.Bus.Cart = e.Cart
	}
}

// Step executes one CPU instruction and updates all subsystems
func (e *Emulator) Step() int {
	cycles := e.CPU.Step()

	// Update timer
	if e.Timer.Step(cycles) {
		e.Bus.RequestInterrupt(0x04) // Timer interrupt (bit 2)
	}

	// Update PPU
	ppuIRQ := e.PPU.Step(cycles)
	if ppuIRQ&0x01 != 0 {
		e.Bus.RequestInterrupt(0x01) // VBlank interrupt (bit 0)
	}
	if ppuIRQ&0x02 != 0 {
		e.Bus.RequestInterrupt(0x02) // STAT interrupt (bit 1)
	}

	// Update APU
	e.APU.Step(cycles)

	return cycles
}

// RunFrame runs emulation for one frame (~70224 T-cycles)
func (e *Emulator) RunFrame() {
	cyclesThisFrame := 0
	for cyclesThisFrame < cyclesPerFrame {
		cyclesThisFrame += e.Step()
	}
}

// Run starts the emulator main loop
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

// GetFrameBuffer returns the RGBA frame buffer
func (e *Emulator) GetFrameBuffer() []uint8 {
	return e.PPU.FrameBuffer[:]
}

// IsFrameReady returns true when PPU has completed a frame
func (e *Emulator) IsFrameReady() bool {
	return e.PPU.IsFrameReady()
}

// GetAudioSamples returns pending audio samples
func (e *Emulator) GetAudioSamples() []float32 {
	return e.APU.GetSamples()
}

// KeyDown handles key press
func (e *Emulator) KeyDown(button joypad.Button) {
	if e.Joypad.Press(button) {
		e.Bus.RequestInterrupt(0x10) // Joypad interrupt (bit 4)
	}
}

// KeyUp handles key release
func (e *Emulator) KeyUp(button joypad.Button) {
	e.Joypad.Release(button)
}

// GetCartridgeTitle returns the loaded ROM title
func (e *Emulator) GetCartridgeTitle() string {
	if e.Cart != nil {
		return e.Cart.Title
	}
	return ""
}
