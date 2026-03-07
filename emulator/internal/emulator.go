package internal

import (
	"go2daboy/emulator/internal/apu"
	"go2daboy/emulator/internal/cartridge"
	"go2daboy/emulator/internal/cpu"
	"go2daboy/emulator/internal/joypad"
	"go2daboy/emulator/internal/memory"
	"go2daboy/emulator/internal/ppu"
	"go2daboy/emulator/internal/timer"
)

const cyclesPerFrame = 70224 // T-cycles per frame (~59.7 FPS)

// Emulator represents the emulator core
type Emulator struct {
	Bus            *memory.Bus
	CPU            *cpu.CPU
	PPU            *ppu.PPU
	APU            *apu.APU
	Timer          *timer.Timer
	Joypad         *joypad.Joypad
	Cart           *cartridge.Cartridge
	running        bool
	cycleOvershoot int // carry-over cycles from previous RunFrame to prevent drift
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

// Reset resets the emulator to post-boot ROM state
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

	// Set post-boot I/O register values (what the boot ROM leaves behind)
	e.Bus.Write(0xFF00, 0xCF) // P1/JOYP
	e.Bus.Write(0xFF01, 0x00) // SB
	e.Bus.Write(0xFF02, 0x7E) // SC
	e.Bus.Write(0xFF05, 0x00) // TIMA
	e.Bus.Write(0xFF06, 0x00) // TMA
	e.Bus.Write(0xFF07, 0xF8) // TAC
	e.Bus.Write(0xFF0F, 0xE1) // IF
	e.Bus.Write(0xFF10, 0x80) // NR10
	e.Bus.Write(0xFF11, 0xBF) // NR11
	e.Bus.Write(0xFF12, 0xF3) // NR12
	e.Bus.Write(0xFF14, 0xBF) // NR14
	e.Bus.Write(0xFF16, 0x3F) // NR21
	e.Bus.Write(0xFF17, 0x00) // NR22
	e.Bus.Write(0xFF19, 0xBF) // NR24
	e.Bus.Write(0xFF1A, 0x7F) // NR30
	e.Bus.Write(0xFF1B, 0xFF) // NR31
	e.Bus.Write(0xFF1C, 0x9F) // NR32
	e.Bus.Write(0xFF1E, 0xBF) // NR34
	e.Bus.Write(0xFF20, 0xFF) // NR41
	e.Bus.Write(0xFF21, 0x00) // NR42
	e.Bus.Write(0xFF22, 0x00) // NR43
	e.Bus.Write(0xFF23, 0xBF) // NR44
	e.Bus.Write(0xFF24, 0x77) // NR50
	e.Bus.Write(0xFF25, 0xF3) // NR51
	e.Bus.Write(0xFF26, 0xF1) // NR52
	e.Bus.Write(0xFF40, 0x91) // LCDC
	e.Bus.Write(0xFF41, 0x85) // STAT
	e.Bus.Write(0xFF42, 0x00) // SCY
	e.Bus.Write(0xFF43, 0x00) // SCX
	e.Bus.Write(0xFF45, 0x00) // LYC
	e.Bus.Write(0xFF47, 0xFC) // BGP
	e.Bus.Write(0xFF48, 0xFF) // OBP0
	e.Bus.Write(0xFF49, 0xFF) // OBP1
	e.Bus.Write(0xFF4A, 0x00) // WY
	e.Bus.Write(0xFF4B, 0x00) // WX
	e.Bus.Write(0xFFFF, 0x00) // IE
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

	// Update DMA
	e.Bus.StepDMA(cycles)

	return cycles
}

// RunFrame runs emulation for one frame (~70224 T-cycles).
// Excess cycles from the last instruction are carried over to the next frame
// to prevent drift that causes the frame boundary to cross VBlank, which
// would leave a partially-rendered next frame in the buffer (visible as tearing).
func (e *Emulator) RunFrame() {
	cyclesThisFrame := e.cycleOvershoot
	for cyclesThisFrame < cyclesPerFrame {
		cyclesThisFrame += e.Step()
	}
	e.cycleOvershoot = cyclesThisFrame - cyclesPerFrame
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

// SetPalette replaces the 4-shade palette at runtime.
func (e *Emulator) SetPalette(colors [4][4]uint8) {
	ppu.SetPalette(colors)
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
