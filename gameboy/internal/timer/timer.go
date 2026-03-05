package timer

// Timer implements the Game Boy timer subsystem.
// DIV (0xFF04) is the upper byte of a 16-bit internal counter.
// TIMA (0xFF05) increments at a rate specified by TAC and triggers
// an interrupt on overflow, reloading from TMA.
type Timer struct {
	div      uint16 // Internal 16-bit counter; upper 8 bits = DIV register
	tima     uint8  // Timer counter (0xFF05)
	tma      uint8  // Timer modulo (0xFF06)
	tac      uint8  // Timer control (0xFF07)
	overflow bool
}

func New() *Timer {
	return &Timer{}
}

func (t *Timer) Reset() {
	t.div = 0xABCC // Post-boot value
	t.tima = 0
	t.tma = 0
	t.tac = 0
	t.overflow = false
}

func (t *Timer) Read(addr uint16) uint8 {
	switch addr {
	case 0xFF04:
		return uint8(t.div >> 8)
	case 0xFF05:
		return t.tima
	case 0xFF06:
		return t.tma
	case 0xFF07:
		return t.tac | 0xF8
	}
	return 0xFF
}

func (t *Timer) Write(addr uint16, value uint8) {
	switch addr {
	case 0xFF04:
		// Writing any value resets DIV to 0
		t.div = 0
	case 0xFF05:
		t.tima = value
		t.overflow = false
	case 0xFF06:
		t.tma = value
	case 0xFF07:
		t.tac = value & 0x07
	}
}

// tacBit returns the bit position in DIV that triggers TIMA increment
func (t *Timer) tacBit() uint16 {
	switch t.tac & 0x03 {
	case 0:
		return 1 << 9 // 4096 Hz
	case 1:
		return 1 << 3 // 262144 Hz
	case 2:
		return 1 << 5 // 65536 Hz
	case 3:
		return 1 << 7 // 16384 Hz
	}
	return 0
}

// Step advances the timer by the given number of T-cycles.
// Returns true if a timer interrupt should be requested.
func (t *Timer) Step(cycles int) bool {
	interrupt := false
	for i := 0; i < cycles; i++ {
		prevDiv := t.div
		t.div++

		if t.tac&0x04 != 0 {
			bit := t.tacBit()
			// Falling edge detector
			if prevDiv&bit != 0 && t.div&bit == 0 {
				t.tima++
				if t.tima == 0 {
					t.tima = t.tma
					interrupt = true
				}
			}
		}
	}
	return interrupt
}
