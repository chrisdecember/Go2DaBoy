package cpu

// CPU flags
const (
	FlagZ uint8 = 1 << 7 // Zero
	FlagN uint8 = 1 << 6 // Subtract
	FlagH uint8 = 1 << 5 // Half Carry
	FlagC uint8 = 1 << 4 // Carry
)

// Registers holds CPU register values
type Registers struct {
	A, F uint8  // Accumulator & Flags
	B, C uint8  // BC pair
	D, E uint8  // DE pair
	H, L uint8  // HL pair
	SP   uint16 // Stack pointer
	PC   uint16 // Program counter
}

// NewRegisters creates a register set with initial values
func NewRegisters() Registers {
	return Registers{
		A:  0x01,
		F:  0xB0,
		B:  0x00,
		C:  0x13,
		D:  0x00,
		E:  0xD8,
		H:  0x01,
		L:  0x4D,
		SP: 0xFFFE,
		PC: 0x0100,
	}
}

// GetAF returns the combined AF register
func (r *Registers) GetAF() uint16 {
	return uint16(r.A)<<8 | uint16(r.F)
}

// SetAF sets the combined AF register
func (r *Registers) SetAF(value uint16) {
	r.A = uint8(value >> 8)
	r.F = uint8(value) & 0xF0 // Lower 4 bits are always 0
}

// GetBC returns the combined BC register
func (r *Registers) GetBC() uint16 {
	return uint16(r.B)<<8 | uint16(r.C)
}

// SetBC sets the combined BC register
func (r *Registers) SetBC(value uint16) {
	r.B = uint8(value >> 8)
	r.C = uint8(value)
}

// GetDE returns the combined DE register
func (r *Registers) GetDE() uint16 {
	return uint16(r.D)<<8 | uint16(r.E)
}

// SetDE sets the combined DE register
func (r *Registers) SetDE(value uint16) {
	r.D = uint8(value >> 8)
	r.E = uint8(value)
}

// GetHL returns the combined HL register
func (r *Registers) GetHL() uint16 {
	return uint16(r.H)<<8 | uint16(r.L)
}

// SetHL sets the combined HL register
func (r *Registers) SetHL(value uint16) {
	r.H = uint8(value >> 8)
	r.L = uint8(value)
}

// GetFlag returns the state of a specific flag
func (r *Registers) GetFlag(flag uint8) bool {
	return r.F&flag != 0
}

// SetFlag sets or clears a specific flag
func (r *Registers) SetFlag(flag uint8, value bool) {
	if value {
		r.F |= flag
	} else {
		r.F &^= flag
	}
}
