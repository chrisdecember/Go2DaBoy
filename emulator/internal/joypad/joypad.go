package joypad

// Button represents an emulated button
type Button uint8

const (
	ButtonA      Button = 0
	ButtonB      Button = 1
	ButtonSelect Button = 2
	ButtonStart  Button = 3
	ButtonRight  Button = 4
	ButtonLeft   Button = 5
	ButtonUp     Button = 6
	ButtonDown   Button = 7
)

// Joypad handles button input via register 0xFF00.
// Uses active-low logic: bit=0 means pressed.
type Joypad struct {
	buttons         uint8 // 8 buttons packed: low=pressed
	selectAction    bool  // Bit 5 of P1 (0=select)
	selectDirection bool  // Bit 4 of P1 (0=select)
}

func New() *Joypad {
	return &Joypad{
		buttons: 0xFF, // All released
	}
}

func (j *Joypad) Reset() {
	j.buttons = 0xFF
	j.selectAction = false
	j.selectDirection = false
}

// Read returns the joypad register value (0xFF00)
func (j *Joypad) Read() uint8 {
	result := uint8(0xFF)

	if j.selectAction {
		result &^= 0x20
		if j.buttons&(1<<uint(ButtonA)) == 0 {
			result &^= 0x01
		}
		if j.buttons&(1<<uint(ButtonB)) == 0 {
			result &^= 0x02
		}
		if j.buttons&(1<<uint(ButtonSelect)) == 0 {
			result &^= 0x04
		}
		if j.buttons&(1<<uint(ButtonStart)) == 0 {
			result &^= 0x08
		}
	}

	if j.selectDirection {
		result &^= 0x10
		if j.buttons&(1<<uint(ButtonRight)) == 0 {
			result &^= 0x01
		}
		if j.buttons&(1<<uint(ButtonLeft)) == 0 {
			result &^= 0x02
		}
		if j.buttons&(1<<uint(ButtonUp)) == 0 {
			result &^= 0x04
		}
		if j.buttons&(1<<uint(ButtonDown)) == 0 {
			result &^= 0x08
		}
	}

	return result
}

// Write handles writes to the joypad register (0xFF00)
func (j *Joypad) Write(value uint8) {
	j.selectAction = value&0x20 == 0
	j.selectDirection = value&0x10 == 0
}

// Press marks a button as pressed. Returns true if state changed (for interrupt).
func (j *Joypad) Press(button Button) bool {
	old := j.buttons
	j.buttons &^= 1 << uint(button)
	return old != j.buttons
}

// Release marks a button as released.
func (j *Joypad) Release(button Button) {
	j.buttons |= 1 << uint(button)
}
