package cpu

// execute dispatches a base opcode. Timing is handled by c.read/c.write/c.idle
// calls within each instruction; the opcode fetch M-cycle is already ticked by fetchByte.
func (c *CPU) execute(opcode uint8) {
	switch opcode {
	// === NOP === (4T: fetch only)
	case 0x00:

	// === LD rr, d16 === (12T: fetch + imm_lo + imm_hi)
	case 0x01:
		c.Regs.SetBC(c.fetchWord())
	case 0x11:
		c.Regs.SetDE(c.fetchWord())
	case 0x21:
		c.Regs.SetHL(c.fetchWord())
	case 0x31:
		c.Regs.SP = c.fetchWord()

	// === LD (rr), A === (8T: fetch + write)
	case 0x02:
		c.write(c.Regs.GetBC(), c.Regs.A)
	case 0x12:
		c.write(c.Regs.GetDE(), c.Regs.A)
	case 0x22: // LD (HL+), A
		c.write(c.Regs.GetHL(), c.Regs.A)
		c.Regs.SetHL(c.Regs.GetHL() + 1)
	case 0x32: // LD (HL-), A
		c.write(c.Regs.GetHL(), c.Regs.A)
		c.Regs.SetHL(c.Regs.GetHL() - 1)

	// === INC rr === (8T: fetch + idle)
	case 0x03:
		c.Regs.SetBC(c.Regs.GetBC() + 1)
		c.idle()
	case 0x13:
		c.Regs.SetDE(c.Regs.GetDE() + 1)
		c.idle()
	case 0x23:
		c.Regs.SetHL(c.Regs.GetHL() + 1)
		c.idle()
	case 0x33:
		c.Regs.SP++
		c.idle()

	// === INC r === (4T: fetch only)
	case 0x04:
		c.Regs.B = c.inc(c.Regs.B)
	case 0x0C:
		c.Regs.C = c.inc(c.Regs.C)
	case 0x14:
		c.Regs.D = c.inc(c.Regs.D)
	case 0x1C:
		c.Regs.E = c.inc(c.Regs.E)
	case 0x24:
		c.Regs.H = c.inc(c.Regs.H)
	case 0x2C:
		c.Regs.L = c.inc(c.Regs.L)
	case 0x34: // INC (HL) (12T: fetch + read + write)
		val := c.read(c.Regs.GetHL())
		c.write(c.Regs.GetHL(), c.inc(val))
	case 0x3C:
		c.Regs.A = c.inc(c.Regs.A)

	// === DEC r === (4T: fetch only)
	case 0x05:
		c.Regs.B = c.dec(c.Regs.B)
	case 0x0D:
		c.Regs.C = c.dec(c.Regs.C)
	case 0x15:
		c.Regs.D = c.dec(c.Regs.D)
	case 0x1D:
		c.Regs.E = c.dec(c.Regs.E)
	case 0x25:
		c.Regs.H = c.dec(c.Regs.H)
	case 0x2D:
		c.Regs.L = c.dec(c.Regs.L)
	case 0x35: // DEC (HL) (12T: fetch + read + write)
		val := c.read(c.Regs.GetHL())
		c.write(c.Regs.GetHL(), c.dec(val))
	case 0x3D:
		c.Regs.A = c.dec(c.Regs.A)

	// === LD r, d8 === (8T: fetch + imm)
	case 0x06:
		c.Regs.B = c.fetchByte()
	case 0x0E:
		c.Regs.C = c.fetchByte()
	case 0x16:
		c.Regs.D = c.fetchByte()
	case 0x1E:
		c.Regs.E = c.fetchByte()
	case 0x26:
		c.Regs.H = c.fetchByte()
	case 0x2E:
		c.Regs.L = c.fetchByte()
	case 0x36: // LD (HL), d8 (12T: fetch + imm + write)
		val := c.fetchByte()
		c.write(c.Regs.GetHL(), val)
	case 0x3E:
		c.Regs.A = c.fetchByte()

	// === Rotate A instructions === (4T: fetch only)
	case 0x07: // RLCA
		carry := c.Regs.A >> 7
		c.Regs.A = (c.Regs.A << 1) | carry
		c.Regs.SetFlag(FlagZ, false)
		c.Regs.SetFlag(FlagN, false)
		c.Regs.SetFlag(FlagH, false)
		c.Regs.SetFlag(FlagC, carry != 0)
	case 0x0F: // RRCA
		carry := c.Regs.A & 1
		c.Regs.A = (c.Regs.A >> 1) | (carry << 7)
		c.Regs.SetFlag(FlagZ, false)
		c.Regs.SetFlag(FlagN, false)
		c.Regs.SetFlag(FlagH, false)
		c.Regs.SetFlag(FlagC, carry != 0)
	case 0x17: // RLA
		oldCarry := uint8(0)
		if c.Regs.GetFlag(FlagC) {
			oldCarry = 1
		}
		carry := c.Regs.A >> 7
		c.Regs.A = (c.Regs.A << 1) | oldCarry
		c.Regs.SetFlag(FlagZ, false)
		c.Regs.SetFlag(FlagN, false)
		c.Regs.SetFlag(FlagH, false)
		c.Regs.SetFlag(FlagC, carry != 0)
	case 0x1F: // RRA
		oldCarry := uint8(0)
		if c.Regs.GetFlag(FlagC) {
			oldCarry = 1
		}
		carry := c.Regs.A & 1
		c.Regs.A = (c.Regs.A >> 1) | (oldCarry << 7)
		c.Regs.SetFlag(FlagZ, false)
		c.Regs.SetFlag(FlagN, false)
		c.Regs.SetFlag(FlagH, false)
		c.Regs.SetFlag(FlagC, carry != 0)

	// === LD (d16), SP === (20T: fetch + imm_lo + imm_hi + write_lo + write_hi)
	case 0x08:
		addr := c.fetchWord()
		c.write(addr, uint8(c.Regs.SP&0xFF))
		c.write(addr+1, uint8(c.Regs.SP>>8))

	// === ADD HL, rr === (8T: fetch + idle)
	case 0x09:
		c.addHL(c.Regs.GetBC())
		c.idle()
	case 0x19:
		c.addHL(c.Regs.GetDE())
		c.idle()
	case 0x29:
		c.addHL(c.Regs.GetHL())
		c.idle()
	case 0x39:
		c.addHL(c.Regs.SP)
		c.idle()

	// === LD A, (rr) === (8T: fetch + read)
	case 0x0A:
		c.Regs.A = c.read(c.Regs.GetBC())
	case 0x1A:
		c.Regs.A = c.read(c.Regs.GetDE())
	case 0x2A: // LD A, (HL+)
		c.Regs.A = c.read(c.Regs.GetHL())
		c.Regs.SetHL(c.Regs.GetHL() + 1)
	case 0x3A: // LD A, (HL-)
		c.Regs.A = c.read(c.Regs.GetHL())
		c.Regs.SetHL(c.Regs.GetHL() - 1)

	// === DEC rr === (8T: fetch + idle)
	case 0x0B:
		c.Regs.SetBC(c.Regs.GetBC() - 1)
		c.idle()
	case 0x1B:
		c.Regs.SetDE(c.Regs.GetDE() - 1)
		c.idle()
	case 0x2B:
		c.Regs.SetHL(c.Regs.GetHL() - 1)
		c.idle()
	case 0x3B:
		c.Regs.SP--
		c.idle()

	// === Misc ===
	case 0x10: // STOP (consume next byte)
		c.fetchByte()
		c.stopped = true

	case 0x18: // JR e8 (12T: fetch + imm + idle)
		offset := int8(c.fetchByte())
		c.Regs.PC = uint16(int32(c.Regs.PC) + int32(offset))
		c.idle()

	case 0x20: // JR NZ, e8 (12T taken, 8T not taken)
		offset := int8(c.fetchByte())
		if !c.Regs.GetFlag(FlagZ) {
			c.Regs.PC = uint16(int32(c.Regs.PC) + int32(offset))
			c.idle()
		}

	case 0x28: // JR Z, e8
		offset := int8(c.fetchByte())
		if c.Regs.GetFlag(FlagZ) {
			c.Regs.PC = uint16(int32(c.Regs.PC) + int32(offset))
			c.idle()
		}

	case 0x30: // JR NC, e8
		offset := int8(c.fetchByte())
		if !c.Regs.GetFlag(FlagC) {
			c.Regs.PC = uint16(int32(c.Regs.PC) + int32(offset))
			c.idle()
		}

	case 0x38: // JR C, e8
		offset := int8(c.fetchByte())
		if c.Regs.GetFlag(FlagC) {
			c.Regs.PC = uint16(int32(c.Regs.PC) + int32(offset))
			c.idle()
		}

	case 0x27: // DAA (4T)
		c.daa()

	case 0x2F: // CPL (4T)
		c.Regs.A = ^c.Regs.A
		c.Regs.SetFlag(FlagN, true)
		c.Regs.SetFlag(FlagH, true)

	case 0x37: // SCF (4T)
		c.Regs.SetFlag(FlagN, false)
		c.Regs.SetFlag(FlagH, false)
		c.Regs.SetFlag(FlagC, true)

	case 0x3F: // CCF (4T)
		c.Regs.SetFlag(FlagN, false)
		c.Regs.SetFlag(FlagH, false)
		c.Regs.SetFlag(FlagC, !c.Regs.GetFlag(FlagC))

	// === LD r, r' (0x40-0x7F) === (4T: fetch only; 8T for (HL) variants)
	case 0x40: // LD B,B
	case 0x41:
		c.Regs.B = c.Regs.C
	case 0x42:
		c.Regs.B = c.Regs.D
	case 0x43:
		c.Regs.B = c.Regs.E
	case 0x44:
		c.Regs.B = c.Regs.H
	case 0x45:
		c.Regs.B = c.Regs.L
	case 0x46:
		c.Regs.B = c.read(c.Regs.GetHL())
	case 0x47:
		c.Regs.B = c.Regs.A
	case 0x48:
		c.Regs.C = c.Regs.B
	case 0x49: // LD C,C
	case 0x4A:
		c.Regs.C = c.Regs.D
	case 0x4B:
		c.Regs.C = c.Regs.E
	case 0x4C:
		c.Regs.C = c.Regs.H
	case 0x4D:
		c.Regs.C = c.Regs.L
	case 0x4E:
		c.Regs.C = c.read(c.Regs.GetHL())
	case 0x4F:
		c.Regs.C = c.Regs.A
	case 0x50:
		c.Regs.D = c.Regs.B
	case 0x51:
		c.Regs.D = c.Regs.C
	case 0x52: // LD D,D
	case 0x53:
		c.Regs.D = c.Regs.E
	case 0x54:
		c.Regs.D = c.Regs.H
	case 0x55:
		c.Regs.D = c.Regs.L
	case 0x56:
		c.Regs.D = c.read(c.Regs.GetHL())
	case 0x57:
		c.Regs.D = c.Regs.A
	case 0x58:
		c.Regs.E = c.Regs.B
	case 0x59:
		c.Regs.E = c.Regs.C
	case 0x5A:
		c.Regs.E = c.Regs.D
	case 0x5B: // LD E,E
	case 0x5C:
		c.Regs.E = c.Regs.H
	case 0x5D:
		c.Regs.E = c.Regs.L
	case 0x5E:
		c.Regs.E = c.read(c.Regs.GetHL())
	case 0x5F:
		c.Regs.E = c.Regs.A
	case 0x60:
		c.Regs.H = c.Regs.B
	case 0x61:
		c.Regs.H = c.Regs.C
	case 0x62:
		c.Regs.H = c.Regs.D
	case 0x63:
		c.Regs.H = c.Regs.E
	case 0x64: // LD H,H
	case 0x65:
		c.Regs.H = c.Regs.L
	case 0x66:
		c.Regs.H = c.read(c.Regs.GetHL())
	case 0x67:
		c.Regs.H = c.Regs.A
	case 0x68:
		c.Regs.L = c.Regs.B
	case 0x69:
		c.Regs.L = c.Regs.C
	case 0x6A:
		c.Regs.L = c.Regs.D
	case 0x6B:
		c.Regs.L = c.Regs.E
	case 0x6C:
		c.Regs.L = c.Regs.H
	case 0x6D: // LD L,L
	case 0x6E:
		c.Regs.L = c.read(c.Regs.GetHL())
	case 0x6F:
		c.Regs.L = c.Regs.A

	// LD (HL), r (8T: fetch + write)
	case 0x70:
		c.write(c.Regs.GetHL(), c.Regs.B)
	case 0x71:
		c.write(c.Regs.GetHL(), c.Regs.C)
	case 0x72:
		c.write(c.Regs.GetHL(), c.Regs.D)
	case 0x73:
		c.write(c.Regs.GetHL(), c.Regs.E)
	case 0x74:
		c.write(c.Regs.GetHL(), c.Regs.H)
	case 0x75:
		c.write(c.Regs.GetHL(), c.Regs.L)
	case 0x76: // HALT (4T)
		if c.ime {
			c.halted = true
		} else {
			if c.Bus.GetIF()&c.Bus.GetIE()&0x1F != 0 {
				c.haltBug = true
			} else {
				c.halted = true
			}
		}
	case 0x77:
		c.write(c.Regs.GetHL(), c.Regs.A)

	// LD A, r (4T: fetch only)
	case 0x78:
		c.Regs.A = c.Regs.B
	case 0x79:
		c.Regs.A = c.Regs.C
	case 0x7A:
		c.Regs.A = c.Regs.D
	case 0x7B:
		c.Regs.A = c.Regs.E
	case 0x7C:
		c.Regs.A = c.Regs.H
	case 0x7D:
		c.Regs.A = c.Regs.L
	case 0x7E:
		c.Regs.A = c.read(c.Regs.GetHL())
	case 0x7F: // LD A,A

	// === ALU A, r (0x80-0xBF) === (4T reg, 8T (HL))
	// ADD A, r
	case 0x80:
		c.add(c.Regs.B)
	case 0x81:
		c.add(c.Regs.C)
	case 0x82:
		c.add(c.Regs.D)
	case 0x83:
		c.add(c.Regs.E)
	case 0x84:
		c.add(c.Regs.H)
	case 0x85:
		c.add(c.Regs.L)
	case 0x86:
		c.add(c.read(c.Regs.GetHL()))
	case 0x87:
		c.add(c.Regs.A)

	// ADC A, r
	case 0x88:
		c.adc(c.Regs.B)
	case 0x89:
		c.adc(c.Regs.C)
	case 0x8A:
		c.adc(c.Regs.D)
	case 0x8B:
		c.adc(c.Regs.E)
	case 0x8C:
		c.adc(c.Regs.H)
	case 0x8D:
		c.adc(c.Regs.L)
	case 0x8E:
		c.adc(c.read(c.Regs.GetHL()))
	case 0x8F:
		c.adc(c.Regs.A)

	// SUB r
	case 0x90:
		c.sub(c.Regs.B)
	case 0x91:
		c.sub(c.Regs.C)
	case 0x92:
		c.sub(c.Regs.D)
	case 0x93:
		c.sub(c.Regs.E)
	case 0x94:
		c.sub(c.Regs.H)
	case 0x95:
		c.sub(c.Regs.L)
	case 0x96:
		c.sub(c.read(c.Regs.GetHL()))
	case 0x97:
		c.sub(c.Regs.A)

	// SBC A, r
	case 0x98:
		c.sbc(c.Regs.B)
	case 0x99:
		c.sbc(c.Regs.C)
	case 0x9A:
		c.sbc(c.Regs.D)
	case 0x9B:
		c.sbc(c.Regs.E)
	case 0x9C:
		c.sbc(c.Regs.H)
	case 0x9D:
		c.sbc(c.Regs.L)
	case 0x9E:
		c.sbc(c.read(c.Regs.GetHL()))
	case 0x9F:
		c.sbc(c.Regs.A)

	// AND r
	case 0xA0:
		c.and(c.Regs.B)
	case 0xA1:
		c.and(c.Regs.C)
	case 0xA2:
		c.and(c.Regs.D)
	case 0xA3:
		c.and(c.Regs.E)
	case 0xA4:
		c.and(c.Regs.H)
	case 0xA5:
		c.and(c.Regs.L)
	case 0xA6:
		c.and(c.read(c.Regs.GetHL()))
	case 0xA7:
		c.and(c.Regs.A)

	// XOR r
	case 0xA8:
		c.xor(c.Regs.B)
	case 0xA9:
		c.xor(c.Regs.C)
	case 0xAA:
		c.xor(c.Regs.D)
	case 0xAB:
		c.xor(c.Regs.E)
	case 0xAC:
		c.xor(c.Regs.H)
	case 0xAD:
		c.xor(c.Regs.L)
	case 0xAE:
		c.xor(c.read(c.Regs.GetHL()))
	case 0xAF:
		c.xor(c.Regs.A)

	// OR r
	case 0xB0:
		c.or(c.Regs.B)
	case 0xB1:
		c.or(c.Regs.C)
	case 0xB2:
		c.or(c.Regs.D)
	case 0xB3:
		c.or(c.Regs.E)
	case 0xB4:
		c.or(c.Regs.H)
	case 0xB5:
		c.or(c.Regs.L)
	case 0xB6:
		c.or(c.read(c.Regs.GetHL()))
	case 0xB7:
		c.or(c.Regs.A)

	// CP r
	case 0xB8:
		c.cp(c.Regs.B)
	case 0xB9:
		c.cp(c.Regs.C)
	case 0xBA:
		c.cp(c.Regs.D)
	case 0xBB:
		c.cp(c.Regs.E)
	case 0xBC:
		c.cp(c.Regs.H)
	case 0xBD:
		c.cp(c.Regs.L)
	case 0xBE:
		c.cp(c.read(c.Regs.GetHL()))
	case 0xBF:
		c.cp(c.Regs.A)

	// === RET cc === (20T taken: fetch + idle + pop_lo + pop_hi + idle; 8T not taken: fetch + idle)
	case 0xC0: // RET NZ
		c.idle()
		if !c.Regs.GetFlag(FlagZ) {
			c.Regs.PC = c.pop()
			c.idle()
		}
	case 0xC8: // RET Z
		c.idle()
		if c.Regs.GetFlag(FlagZ) {
			c.Regs.PC = c.pop()
			c.idle()
		}
	case 0xD0: // RET NC
		c.idle()
		if !c.Regs.GetFlag(FlagC) {
			c.Regs.PC = c.pop()
			c.idle()
		}
	case 0xD8: // RET C
		c.idle()
		if c.Regs.GetFlag(FlagC) {
			c.Regs.PC = c.pop()
			c.idle()
		}

	// === POP rr === (12T: fetch + pop_lo + pop_hi)
	case 0xC1:
		c.Regs.SetBC(c.pop())
	case 0xD1:
		c.Regs.SetDE(c.pop())
	case 0xE1:
		c.Regs.SetHL(c.pop())
	case 0xF1:
		c.Regs.SetAF(c.pop())

	// === JP cc, d16 === (16T taken, 12T not taken)
	case 0xC2: // JP NZ, d16
		addr := c.fetchWord()
		if !c.Regs.GetFlag(FlagZ) {
			c.Regs.PC = addr
			c.idle()
		}
	case 0xCA: // JP Z, d16
		addr := c.fetchWord()
		if c.Regs.GetFlag(FlagZ) {
			c.Regs.PC = addr
			c.idle()
		}
	case 0xD2: // JP NC, d16
		addr := c.fetchWord()
		if !c.Regs.GetFlag(FlagC) {
			c.Regs.PC = addr
			c.idle()
		}
	case 0xDA: // JP C, d16
		addr := c.fetchWord()
		if c.Regs.GetFlag(FlagC) {
			c.Regs.PC = addr
			c.idle()
		}

	// === JP d16 === (16T: fetch + imm_lo + imm_hi + idle)
	case 0xC3:
		c.Regs.PC = c.fetchWord()
		c.idle()

	// === CALL cc, d16 === (24T taken, 12T not taken)
	case 0xC4: // CALL NZ, d16
		addr := c.fetchWord()
		if !c.Regs.GetFlag(FlagZ) {
			c.idle()
			c.push(c.Regs.PC)
			c.Regs.PC = addr
		}
	case 0xCC: // CALL Z, d16
		addr := c.fetchWord()
		if c.Regs.GetFlag(FlagZ) {
			c.idle()
			c.push(c.Regs.PC)
			c.Regs.PC = addr
		}
	case 0xD4: // CALL NC, d16
		addr := c.fetchWord()
		if !c.Regs.GetFlag(FlagC) {
			c.idle()
			c.push(c.Regs.PC)
			c.Regs.PC = addr
		}
	case 0xDC: // CALL C, d16
		addr := c.fetchWord()
		if c.Regs.GetFlag(FlagC) {
			c.idle()
			c.push(c.Regs.PC)
			c.Regs.PC = addr
		}

	// === PUSH rr === (16T: fetch + idle + push_hi + push_lo)
	case 0xC5:
		c.idle()
		c.push(c.Regs.GetBC())
	case 0xD5:
		c.idle()
		c.push(c.Regs.GetDE())
	case 0xE5:
		c.idle()
		c.push(c.Regs.GetHL())
	case 0xF5:
		c.idle()
		c.push(c.Regs.GetAF())

	// === ALU A, d8 === (8T: fetch + imm)
	case 0xC6:
		c.add(c.fetchByte())
	case 0xCE:
		c.adc(c.fetchByte())
	case 0xD6:
		c.sub(c.fetchByte())
	case 0xDE:
		c.sbc(c.fetchByte())
	case 0xE6:
		c.and(c.fetchByte())
	case 0xEE:
		c.xor(c.fetchByte())
	case 0xF6:
		c.or(c.fetchByte())
	case 0xFE:
		c.cp(c.fetchByte())

	// === RST === (16T: fetch + idle + push_hi + push_lo)
	case 0xC7:
		c.idle()
		c.push(c.Regs.PC)
		c.Regs.PC = 0x00
	case 0xCF:
		c.idle()
		c.push(c.Regs.PC)
		c.Regs.PC = 0x08
	case 0xD7:
		c.idle()
		c.push(c.Regs.PC)
		c.Regs.PC = 0x10
	case 0xDF:
		c.idle()
		c.push(c.Regs.PC)
		c.Regs.PC = 0x18
	case 0xE7:
		c.idle()
		c.push(c.Regs.PC)
		c.Regs.PC = 0x20
	case 0xEF:
		c.idle()
		c.push(c.Regs.PC)
		c.Regs.PC = 0x28
	case 0xF7:
		c.idle()
		c.push(c.Regs.PC)
		c.Regs.PC = 0x30
	case 0xFF:
		c.idle()
		c.push(c.Regs.PC)
		c.Regs.PC = 0x38

	// === RET / RETI === (16T: fetch + pop_lo + pop_hi + idle)
	case 0xC9: // RET
		c.Regs.PC = c.pop()
		c.idle()
	case 0xD9: // RETI
		c.Regs.PC = c.pop()
		c.idle()
		c.ime = true

	// === CB prefix === (fetchByte ticks for the CB opcode)
	case 0xCB:
		cbOpcode := c.fetchByte()
		c.executeCB(cbOpcode)

	// === CALL d16 === (24T: fetch + imm_lo + imm_hi + idle + push_hi + push_lo)
	case 0xCD:
		addr := c.fetchWord()
		c.idle()
		c.push(c.Regs.PC)
		c.Regs.PC = addr

	// === LDH (a8), A / LDH A, (a8) === (12T: fetch + imm + read/write)
	case 0xE0:
		addr := 0xFF00 + uint16(c.fetchByte())
		c.write(addr, c.Regs.A)
	case 0xF0:
		addr := 0xFF00 + uint16(c.fetchByte())
		c.Regs.A = c.read(addr)

	// === LD (C), A / LD A, (C) === (8T: fetch + read/write)
	case 0xE2:
		c.write(0xFF00+uint16(c.Regs.C), c.Regs.A)
	case 0xF2:
		c.Regs.A = c.read(0xFF00 + uint16(c.Regs.C))

	// === DI / EI === (4T: fetch only)
	case 0xF3: // DI
		c.ime = false
		c.eiDelay = false
	case 0xFB: // EI
		c.eiDelay = true

	// === LD (d16), A / LD A, (d16) === (16T: fetch + imm_lo + imm_hi + read/write)
	case 0xEA:
		addr := c.fetchWord()
		c.write(addr, c.Regs.A)
	case 0xFA:
		addr := c.fetchWord()
		c.Regs.A = c.read(addr)

	// === JP (HL) === (4T: fetch only)
	case 0xE9:
		c.Regs.PC = c.Regs.GetHL()

	// === LD SP, HL === (8T: fetch + idle)
	case 0xF9:
		c.Regs.SP = c.Regs.GetHL()
		c.idle()

	// === ADD SP, e8 === (16T: fetch + imm + idle + idle)
	case 0xE8:
		offset := int8(c.fetchByte())
		c.Regs.SP = c.addSPSigned(offset)
		c.idle()
		c.idle()

	// === LD HL, SP+e8 === (12T: fetch + imm + idle)
	case 0xF8:
		offset := int8(c.fetchByte())
		c.Regs.SetHL(c.addSPSigned(offset))
		c.idle()

	// === Undefined opcodes === (4T: fetch only)
	case 0xD3, 0xDB, 0xDD, 0xE3, 0xE4, 0xEB, 0xEC, 0xED, 0xF4, 0xFC, 0xFD:

	default:
	}
}
