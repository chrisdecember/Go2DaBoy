package cpu

// executeCB handles CB-prefixed opcodes. Returns T-cycles (not including CB prefix fetch).
func (c *CPU) executeCB(opcode uint8) int {
	// CB opcodes follow a regular pattern:
	// Bits 7-6: operation (00=rotate/shift, 01=BIT, 10=RES, 11=SET)
	// Bits 5-3: bit number (for BIT/RES/SET) or sub-operation (for rotate/shift)
	// Bits 2-0: register (B=0, C=1, D=2, E=3, H=4, L=5, (HL)=6, A=7)

	reg := opcode & 0x07
	isHL := reg == 6

	// Read the value
	val := c.cbRead(reg)

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
		c.cbWrite(reg, val)
		if isHL {
			return 12
		}
		return 4

	case 1: // BIT b, r
		bit := (opcode >> 3) & 0x07
		c.bit(bit, val)
		if isHL {
			return 8
		}
		return 4

	case 2: // RES b, r
		bit := (opcode >> 3) & 0x07
		val &^= 1 << bit
		c.cbWrite(reg, val)
		if isHL {
			return 12
		}
		return 4

	case 3: // SET b, r
		bit := (opcode >> 3) & 0x07
		val |= 1 << bit
		c.cbWrite(reg, val)
		if isHL {
			return 12
		}
		return 4
	}

	return 4
}

func (c *CPU) cbRead(reg uint8) uint8 {
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
	case 6:
		return c.Bus.Read(c.Regs.GetHL())
	case 7:
		return c.Regs.A
	}
	return 0
}

func (c *CPU) cbWrite(reg uint8, val uint8) {
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
	case 6:
		c.Bus.Write(c.Regs.GetHL(), val)
	case 7:
		c.Regs.A = val
	}
}
