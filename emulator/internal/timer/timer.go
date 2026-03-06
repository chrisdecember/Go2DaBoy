package timer

// Timer implements the timer subsystem.
// DIV (0xFF04) is the upper byte of a 16-bit internal counter.
// TIMA (0xFF05) increments at a rate specified by TAC and triggers
// an interrupt on overflow, reloading from TMA.
type Timer struct {
	div          uint16 // Internal 16-bit counter; upper 8 bits = DIV register
	tima         uint8  // Timer counter (0xFF05)
	tma          uint8  // Timer modulo (0xFF06)
	tac          uint8  // Timer control (0xFF07)
	overflowTick bool   // TIMA overflow delay (reloads on next cycle)
	reloaded     bool   // Whether TIMA was just reloaded from TMA
}

func New() *Timer {
	return &Timer{}
}

func (t *Timer) Reset() {
	t.div = 0xABCC // Post-boot value
	t.tima = 0
	t.tma = 0
	t.tac = 0
	t.overflowTick = false
	t.reloaded = false
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
		// Writing any value resets DIV to 0.
		// If the selected TAC bit was high, resetting causes a falling edge
		// which can trigger a TIMA increment.
		if t.tac&0x04 != 0 {
			bit := t.tacBit()
			if t.div&bit != 0 {
				t.incrementTIMA()
			}
		}
		t.div = 0
	case 0xFF05:
		// Writing to TIMA during the overflow cycle cancels the reload
		if !t.reloaded {
			t.tima = value
		}
		t.overflowTick = false
	case 0xFF06:
		t.tma = value
		// If TIMA was just reloaded from TMA, update it
		if t.reloaded {
			t.tima = value
		}
	case 0xFF07:
		oldTac := t.tac
		t.tac = value & 0x07

		// Changing TAC can cause a falling edge if the old selected bit was 1
		// and either timer is now disabled or new selected bit is 0
		if oldTac&0x04 != 0 {
			oldBit := t.tacBitForClock(oldTac & 0x03)
			oldSelected := t.div&oldBit != 0

			if value&0x04 != 0 {
				newBit := t.tacBitForClock(value & 0x03)
				newSelected := t.div&newBit != 0
				if oldSelected && !newSelected {
					t.incrementTIMA()
				}
			} else if oldSelected {
				// Timer was disabled while selected bit was high
				t.incrementTIMA()
			}
		}
	}
}

func (t *Timer) incrementTIMA() {
	t.tima++
	if t.tima == 0 {
		t.overflowTick = true
	}
}

// tacBit returns the bit position in DIV for the current TAC clock select
func (t *Timer) tacBit() uint16 {
	return t.tacBitForClock(t.tac & 0x03)
}

func (t *Timer) tacBitForClock(clock uint8) uint16 {
	switch clock {
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
		t.reloaded = false

		// Handle TIMA overflow (delayed by one cycle)
		if t.overflowTick {
			t.overflowTick = false
			t.tima = t.tma
			t.reloaded = true
			interrupt = true
		}

		prevDiv := t.div
		t.div++

		if t.tac&0x04 != 0 {
			bit := t.tacBit()
			// Falling edge detector on the AND of the selected bit and timer enable
			if prevDiv&bit != 0 && t.div&bit == 0 {
				t.incrementTIMA()
			}
		}
	}
	return interrupt
}
