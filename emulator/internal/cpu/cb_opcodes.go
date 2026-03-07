package cpu

// executeCB handles CB-prefixed opcodes. The CB prefix fetch and CB opcode fetch
// are already ticked by the caller. This function ticks for any memory operations.
//
// Timing (including prefix + opcode fetches done by caller):
//   - CB reg op:       8T  (prefix fetch + opcode fetch)
//   - CB (HL) BIT:     12T (prefix + opcode + read)
//   - CB (HL) rot/res/set: 16T (prefix + opcode + read + write)
func (c *CPU) executeCB(opcode uint8) {
	reg := opcode & 0x07
	isHL := reg == 6

	// Read the value (ticks 1 M-cycle for (HL))
	var val uint8
	if isHL {
		val = c.read(c.Regs.GetHL())
	} else {
		val = c.cbReadReg(reg)
	}

	op := opcode >> 6
	switch op {
	case 0: // Rotates and shifts
		subOp := (opcode >> 3) & 0x07
		switch subOp {
		case 0:
			val = c.rlc(val)
		case 1:
			val = c.rrc(val)
		case 2:
			val = c.rl(val)
		case 3:
			val = c.rr(val)
		case 4:
			val = c.sla(val)
		case 5:
			val = c.sra(val)
		case 6:
			val = c.swap(val)
		case 7:
			val = c.srl(val)
		}
		if isHL {
			c.write(c.Regs.GetHL(), val)
		} else {
			c.cbWriteReg(reg, val)
		}

	case 1: // BIT b, r (no write back)
		bit := (opcode >> 3) & 0x07
		c.bit(bit, val)

	case 2: // RES b, r
		bit := (opcode >> 3) & 0x07
		val &^= 1 << bit
		if isHL {
			c.write(c.Regs.GetHL(), val)
		} else {
			c.cbWriteReg(reg, val)
		}

	case 3: // SET b, r
		bit := (opcode >> 3) & 0x07
		val |= 1 << bit
		if isHL {
			c.write(c.Regs.GetHL(), val)
		} else {
			c.cbWriteReg(reg, val)
		}
	}
}

func (c *CPU) cbReadReg(reg uint8) uint8 {
	switch reg {
	case 0:
		return c.Regs.B
	case 1:
		return c.Regs.C
	case 2:
		return c.Regs.D
	case 3:
		return c.Regs.E
	case 4:
		return c.Regs.H
	case 5:
		return c.Regs.L
	case 7:
		return c.Regs.A
	}
	return 0
}

func (c *CPU) cbWriteReg(reg uint8, val uint8) {
	switch reg {
	case 0:
		c.Regs.B = val
	case 1:
		c.Regs.C = val
	case 2:
		c.Regs.D = val
	case 3:
		c.Regs.E = val
	case 4:
		c.Regs.H = val
	case 5:
		c.Regs.L = val
	case 7:
		c.Regs.A = val
	}
}
