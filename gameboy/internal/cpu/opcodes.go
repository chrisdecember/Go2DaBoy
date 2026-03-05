package cpu

// execute dispatches a base opcode and returns T-cycles consumed
func (c *CPU) execute(opcode uint8) int {
	switch opcode {
	// === NOP ===
	case 0x00:
		return 4

	// === LD rr, d16 ===
	case 0x01: // LD BC, d16
		c.Regs.SetBC(c.fetchWord())
		return 12
	case 0x11: // LD DE, d16
		c.Regs.SetDE(c.fetchWord())
		return 12
	case 0x21: // LD HL, d16
		c.Regs.SetHL(c.fetchWord())
		return 12
	case 0x31: // LD SP, d16
		c.Regs.SP = c.fetchWord()
		return 12

	// === LD (rr), A ===
	case 0x02: // LD (BC), A
		c.Bus.Write(c.Regs.GetBC(), c.Regs.A)
		return 8
	case 0x12: // LD (DE), A
		c.Bus.Write(c.Regs.GetDE(), c.Regs.A)
		return 8
	case 0x22: // LD (HL+), A
		c.Bus.Write(c.Regs.GetHL(), c.Regs.A)
		c.Regs.SetHL(c.Regs.GetHL() + 1)
		return 8
	case 0x32: // LD (HL-), A
		c.Bus.Write(c.Regs.GetHL(), c.Regs.A)
		c.Regs.SetHL(c.Regs.GetHL() - 1)
		return 8

	// === INC rr ===
	case 0x03:
		c.Regs.SetBC(c.Regs.GetBC() + 1)
		return 8
	case 0x13:
		c.Regs.SetDE(c.Regs.GetDE() + 1)
		return 8
	case 0x23:
		c.Regs.SetHL(c.Regs.GetHL() + 1)
		return 8
	case 0x33:
		c.Regs.SP++
		return 8

	// === INC r ===
	case 0x04:
		c.Regs.B = c.inc(c.Regs.B)
		return 4
	case 0x0C:
		c.Regs.C = c.inc(c.Regs.C)
		return 4
	case 0x14:
		c.Regs.D = c.inc(c.Regs.D)
		return 4
	case 0x1C:
		c.Regs.E = c.inc(c.Regs.E)
		return 4
	case 0x24:
		c.Regs.H = c.inc(c.Regs.H)
		return 4
	case 0x2C:
		c.Regs.L = c.inc(c.Regs.L)
		return 4
	case 0x34: // INC (HL)
		val := c.Bus.Read(c.Regs.GetHL())
		c.Bus.Write(c.Regs.GetHL(), c.inc(val))
		return 12
	case 0x3C:
		c.Regs.A = c.inc(c.Regs.A)
		return 4

	// === DEC r ===
	case 0x05:
		c.Regs.B = c.dec(c.Regs.B)
		return 4
	case 0x0D:
		c.Regs.C = c.dec(c.Regs.C)
		return 4
	case 0x15:
		c.Regs.D = c.dec(c.Regs.D)
		return 4
	case 0x1D:
		c.Regs.E = c.dec(c.Regs.E)
		return 4
	case 0x25:
		c.Regs.H = c.dec(c.Regs.H)
		return 4
	case 0x2D:
		c.Regs.L = c.dec(c.Regs.L)
		return 4
	case 0x35: // DEC (HL)
		val := c.Bus.Read(c.Regs.GetHL())
		c.Bus.Write(c.Regs.GetHL(), c.dec(val))
		return 12
	case 0x3D:
		c.Regs.A = c.dec(c.Regs.A)
		return 4

	// === LD r, d8 ===
	case 0x06:
		c.Regs.B = c.fetchByte()
		return 8
	case 0x0E:
		c.Regs.C = c.fetchByte()
		return 8
	case 0x16:
		c.Regs.D = c.fetchByte()
		return 8
	case 0x1E:
		c.Regs.E = c.fetchByte()
		return 8
	case 0x26:
		c.Regs.H = c.fetchByte()
		return 8
	case 0x2E:
		c.Regs.L = c.fetchByte()
		return 8
	case 0x36: // LD (HL), d8
		c.Bus.Write(c.Regs.GetHL(), c.fetchByte())
		return 12
	case 0x3E:
		c.Regs.A = c.fetchByte()
		return 8

	// === Rotate A instructions ===
	case 0x07: // RLCA
		carry := c.Regs.A >> 7
		c.Regs.A = (c.Regs.A << 1) | carry
		c.Regs.SetFlag(FlagZ, false)
		c.Regs.SetFlag(FlagN, false)
		c.Regs.SetFlag(FlagH, false)
		c.Regs.SetFlag(FlagC, carry != 0)
		return 4
	case 0x0F: // RRCA
		carry := c.Regs.A & 1
		c.Regs.A = (c.Regs.A >> 1) | (carry << 7)
		c.Regs.SetFlag(FlagZ, false)
		c.Regs.SetFlag(FlagN, false)
		c.Regs.SetFlag(FlagH, false)
		c.Regs.SetFlag(FlagC, carry != 0)
		return 4
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
		return 4
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
		return 4

	// === LD (d16), SP ===
	case 0x08:
		addr := c.fetchWord()
		c.Bus.WriteWord(addr, c.Regs.SP)
		return 20

	// === ADD HL, rr ===
	case 0x09:
		c.addHL(c.Regs.GetBC())
		return 8
	case 0x19:
		c.addHL(c.Regs.GetDE())
		return 8
	case 0x29:
		c.addHL(c.Regs.GetHL())
		return 8
	case 0x39:
		c.addHL(c.Regs.SP)
		return 8

	// === LD A, (rr) ===
	case 0x0A:
		c.Regs.A = c.Bus.Read(c.Regs.GetBC())
		return 8
	case 0x1A:
		c.Regs.A = c.Bus.Read(c.Regs.GetDE())
		return 8
	case 0x2A: // LD A, (HL+)
		c.Regs.A = c.Bus.Read(c.Regs.GetHL())
		c.Regs.SetHL(c.Regs.GetHL() + 1)
		return 8
	case 0x3A: // LD A, (HL-)
		c.Regs.A = c.Bus.Read(c.Regs.GetHL())
		c.Regs.SetHL(c.Regs.GetHL() - 1)
		return 8

	// === DEC rr ===
	case 0x0B:
		c.Regs.SetBC(c.Regs.GetBC() - 1)
		return 8
	case 0x1B:
		c.Regs.SetDE(c.Regs.GetDE() - 1)
		return 8
	case 0x2B:
		c.Regs.SetHL(c.Regs.GetHL() - 1)
		return 8
	case 0x3B:
		c.Regs.SP--
		return 8

	// === Misc ===
	case 0x10: // STOP
		c.fetchByte() // Consume next byte
		c.stopped = true
		return 4

	case 0x18: // JR e8
		offset := int8(c.fetchByte())
		c.Regs.PC = uint16(int32(c.Regs.PC) + int32(offset))
		return 12

	case 0x20: // JR NZ, e8
		offset := int8(c.fetchByte())
		if !c.Regs.GetFlag(FlagZ) {
			c.Regs.PC = uint16(int32(c.Regs.PC) + int32(offset))
			return 12
		}
		return 8

	case 0x28: // JR Z, e8
		offset := int8(c.fetchByte())
		if c.Regs.GetFlag(FlagZ) {
			c.Regs.PC = uint16(int32(c.Regs.PC) + int32(offset))
			return 12
		}
		return 8

	case 0x30: // JR NC, e8
		offset := int8(c.fetchByte())
		if !c.Regs.GetFlag(FlagC) {
			c.Regs.PC = uint16(int32(c.Regs.PC) + int32(offset))
			return 12
		}
		return 8

	case 0x38: // JR C, e8
		offset := int8(c.fetchByte())
		if c.Regs.GetFlag(FlagC) {
			c.Regs.PC = uint16(int32(c.Regs.PC) + int32(offset))
			return 12
		}
		return 8

	case 0x27: // DAA
		c.daa()
		return 4

	case 0x2F: // CPL
		c.Regs.A = ^c.Regs.A
		c.Regs.SetFlag(FlagN, true)
		c.Regs.SetFlag(FlagH, true)
		return 4

	case 0x37: // SCF
		c.Regs.SetFlag(FlagN, false)
		c.Regs.SetFlag(FlagH, false)
		c.Regs.SetFlag(FlagC, true)
		return 4

	case 0x3F: // CCF
		c.Regs.SetFlag(FlagN, false)
		c.Regs.SetFlag(FlagH, false)
		c.Regs.SetFlag(FlagC, !c.Regs.GetFlag(FlagC))
		return 4

	// === LD r, r' (0x40-0x7F) ===
	// LD B, r
	case 0x40:
		return 4 // LD B,B = NOP
	case 0x41:
		c.Regs.B = c.Regs.C
		return 4
	case 0x42:
		c.Regs.B = c.Regs.D
		return 4
	case 0x43:
		c.Regs.B = c.Regs.E
		return 4
	case 0x44:
		c.Regs.B = c.Regs.H
		return 4
	case 0x45:
		c.Regs.B = c.Regs.L
		return 4
	case 0x46:
		c.Regs.B = c.Bus.Read(c.Regs.GetHL())
		return 8
	case 0x47:
		c.Regs.B = c.Regs.A
		return 4

	// LD C, r
	case 0x48:
		c.Regs.C = c.Regs.B
		return 4
	case 0x49:
		return 4
	case 0x4A:
		c.Regs.C = c.Regs.D
		return 4
	case 0x4B:
		c.Regs.C = c.Regs.E
		return 4
	case 0x4C:
		c.Regs.C = c.Regs.H
		return 4
	case 0x4D:
		c.Regs.C = c.Regs.L
		return 4
	case 0x4E:
		c.Regs.C = c.Bus.Read(c.Regs.GetHL())
		return 8
	case 0x4F:
		c.Regs.C = c.Regs.A
		return 4

	// LD D, r
	case 0x50:
		c.Regs.D = c.Regs.B
		return 4
	case 0x51:
		c.Regs.D = c.Regs.C
		return 4
	case 0x52:
		return 4
	case 0x53:
		c.Regs.D = c.Regs.E
		return 4
	case 0x54:
		c.Regs.D = c.Regs.H
		return 4
	case 0x55:
		c.Regs.D = c.Regs.L
		return 4
	case 0x56:
		c.Regs.D = c.Bus.Read(c.Regs.GetHL())
		return 8
	case 0x57:
		c.Regs.D = c.Regs.A
		return 4

	// LD E, r
	case 0x58:
		c.Regs.E = c.Regs.B
		return 4
	case 0x59:
		c.Regs.E = c.Regs.C
		return 4
	case 0x5A:
		c.Regs.E = c.Regs.D
		return 4
	case 0x5B:
		return 4
	case 0x5C:
		c.Regs.E = c.Regs.H
		return 4
	case 0x5D:
		c.Regs.E = c.Regs.L
		return 4
	case 0x5E:
		c.Regs.E = c.Bus.Read(c.Regs.GetHL())
		return 8
	case 0x5F:
		c.Regs.E = c.Regs.A
		return 4

	// LD H, r
	case 0x60:
		c.Regs.H = c.Regs.B
		return 4
	case 0x61:
		c.Regs.H = c.Regs.C
		return 4
	case 0x62:
		c.Regs.H = c.Regs.D
		return 4
	case 0x63:
		c.Regs.H = c.Regs.E
		return 4
	case 0x64:
		return 4
	case 0x65:
		c.Regs.H = c.Regs.L
		return 4
	case 0x66:
		c.Regs.H = c.Bus.Read(c.Regs.GetHL())
		return 8
	case 0x67:
		c.Regs.H = c.Regs.A
		return 4

	// LD L, r
	case 0x68:
		c.Regs.L = c.Regs.B
		return 4
	case 0x69:
		c.Regs.L = c.Regs.C
		return 4
	case 0x6A:
		c.Regs.L = c.Regs.D
		return 4
	case 0x6B:
		c.Regs.L = c.Regs.E
		return 4
	case 0x6C:
		c.Regs.L = c.Regs.H
		return 4
	case 0x6D:
		return 4
	case 0x6E:
		c.Regs.L = c.Bus.Read(c.Regs.GetHL())
		return 8
	case 0x6F:
		c.Regs.L = c.Regs.A
		return 4

	// LD (HL), r
	case 0x70:
		c.Bus.Write(c.Regs.GetHL(), c.Regs.B)
		return 8
	case 0x71:
		c.Bus.Write(c.Regs.GetHL(), c.Regs.C)
		return 8
	case 0x72:
		c.Bus.Write(c.Regs.GetHL(), c.Regs.D)
		return 8
	case 0x73:
		c.Bus.Write(c.Regs.GetHL(), c.Regs.E)
		return 8
	case 0x74:
		c.Bus.Write(c.Regs.GetHL(), c.Regs.H)
		return 8
	case 0x75:
		c.Bus.Write(c.Regs.GetHL(), c.Regs.L)
		return 8
	case 0x76: // HALT
		if c.ime {
			c.halted = true
		} else {
			// Check for HALT bug: IME=0 but interrupts pending
			if c.Bus.GetIF()&c.Bus.GetIE()&0x1F != 0 {
				// HALT bug: don't enter HALT, next fetch doesn't increment PC
				c.haltBug = true
			} else {
				// Normal HALT with IME=0: halt until interrupt pending
				c.halted = true
			}
		}
		return 4
	case 0x77:
		c.Bus.Write(c.Regs.GetHL(), c.Regs.A)
		return 8

	// LD A, r
	case 0x78:
		c.Regs.A = c.Regs.B
		return 4
	case 0x79:
		c.Regs.A = c.Regs.C
		return 4
	case 0x7A:
		c.Regs.A = c.Regs.D
		return 4
	case 0x7B:
		c.Regs.A = c.Regs.E
		return 4
	case 0x7C:
		c.Regs.A = c.Regs.H
		return 4
	case 0x7D:
		c.Regs.A = c.Regs.L
		return 4
	case 0x7E:
		c.Regs.A = c.Bus.Read(c.Regs.GetHL())
		return 8
	case 0x7F:
		return 4 // LD A,A = NOP

	// === ALU A, r (0x80-0xBF) ===
	// ADD A, r
	case 0x80:
		c.add(c.Regs.B)
		return 4
	case 0x81:
		c.add(c.Regs.C)
		return 4
	case 0x82:
		c.add(c.Regs.D)
		return 4
	case 0x83:
		c.add(c.Regs.E)
		return 4
	case 0x84:
		c.add(c.Regs.H)
		return 4
	case 0x85:
		c.add(c.Regs.L)
		return 4
	case 0x86:
		c.add(c.Bus.Read(c.Regs.GetHL()))
		return 8
	case 0x87:
		c.add(c.Regs.A)
		return 4

	// ADC A, r
	case 0x88:
		c.adc(c.Regs.B)
		return 4
	case 0x89:
		c.adc(c.Regs.C)
		return 4
	case 0x8A:
		c.adc(c.Regs.D)
		return 4
	case 0x8B:
		c.adc(c.Regs.E)
		return 4
	case 0x8C:
		c.adc(c.Regs.H)
		return 4
	case 0x8D:
		c.adc(c.Regs.L)
		return 4
	case 0x8E:
		c.adc(c.Bus.Read(c.Regs.GetHL()))
		return 8
	case 0x8F:
		c.adc(c.Regs.A)
		return 4

	// SUB r
	case 0x90:
		c.sub(c.Regs.B)
		return 4
	case 0x91:
		c.sub(c.Regs.C)
		return 4
	case 0x92:
		c.sub(c.Regs.D)
		return 4
	case 0x93:
		c.sub(c.Regs.E)
		return 4
	case 0x94:
		c.sub(c.Regs.H)
		return 4
	case 0x95:
		c.sub(c.Regs.L)
		return 4
	case 0x96:
		c.sub(c.Bus.Read(c.Regs.GetHL()))
		return 8
	case 0x97:
		c.sub(c.Regs.A)
		return 4

	// SBC A, r
	case 0x98:
		c.sbc(c.Regs.B)
		return 4
	case 0x99:
		c.sbc(c.Regs.C)
		return 4
	case 0x9A:
		c.sbc(c.Regs.D)
		return 4
	case 0x9B:
		c.sbc(c.Regs.E)
		return 4
	case 0x9C:
		c.sbc(c.Regs.H)
		return 4
	case 0x9D:
		c.sbc(c.Regs.L)
		return 4
	case 0x9E:
		c.sbc(c.Bus.Read(c.Regs.GetHL()))
		return 8
	case 0x9F:
		c.sbc(c.Regs.A)
		return 4

	// AND r
	case 0xA0:
		c.and(c.Regs.B)
		return 4
	case 0xA1:
		c.and(c.Regs.C)
		return 4
	case 0xA2:
		c.and(c.Regs.D)
		return 4
	case 0xA3:
		c.and(c.Regs.E)
		return 4
	case 0xA4:
		c.and(c.Regs.H)
		return 4
	case 0xA5:
		c.and(c.Regs.L)
		return 4
	case 0xA6:
		c.and(c.Bus.Read(c.Regs.GetHL()))
		return 8
	case 0xA7:
		c.and(c.Regs.A)
		return 4

	// XOR r
	case 0xA8:
		c.xor(c.Regs.B)
		return 4
	case 0xA9:
		c.xor(c.Regs.C)
		return 4
	case 0xAA:
		c.xor(c.Regs.D)
		return 4
	case 0xAB:
		c.xor(c.Regs.E)
		return 4
	case 0xAC:
		c.xor(c.Regs.H)
		return 4
	case 0xAD:
		c.xor(c.Regs.L)
		return 4
	case 0xAE:
		c.xor(c.Bus.Read(c.Regs.GetHL()))
		return 8
	case 0xAF:
		c.xor(c.Regs.A)
		return 4

	// OR r
	case 0xB0:
		c.or(c.Regs.B)
		return 4
	case 0xB1:
		c.or(c.Regs.C)
		return 4
	case 0xB2:
		c.or(c.Regs.D)
		return 4
	case 0xB3:
		c.or(c.Regs.E)
		return 4
	case 0xB4:
		c.or(c.Regs.H)
		return 4
	case 0xB5:
		c.or(c.Regs.L)
		return 4
	case 0xB6:
		c.or(c.Bus.Read(c.Regs.GetHL()))
		return 8
	case 0xB7:
		c.or(c.Regs.A)
		return 4

	// CP r
	case 0xB8:
		c.cp(c.Regs.B)
		return 4
	case 0xB9:
		c.cp(c.Regs.C)
		return 4
	case 0xBA:
		c.cp(c.Regs.D)
		return 4
	case 0xBB:
		c.cp(c.Regs.E)
		return 4
	case 0xBC:
		c.cp(c.Regs.H)
		return 4
	case 0xBD:
		c.cp(c.Regs.L)
		return 4
	case 0xBE:
		c.cp(c.Bus.Read(c.Regs.GetHL()))
		return 8
	case 0xBF:
		c.cp(c.Regs.A)
		return 4

	// === RET cc ===
	case 0xC0: // RET NZ
		if !c.Regs.GetFlag(FlagZ) {
			c.Regs.PC = c.pop()
			return 20
		}
		return 8
	case 0xC8: // RET Z
		if c.Regs.GetFlag(FlagZ) {
			c.Regs.PC = c.pop()
			return 20
		}
		return 8
	case 0xD0: // RET NC
		if !c.Regs.GetFlag(FlagC) {
			c.Regs.PC = c.pop()
			return 20
		}
		return 8
	case 0xD8: // RET C
		if c.Regs.GetFlag(FlagC) {
			c.Regs.PC = c.pop()
			return 20
		}
		return 8

	// === POP rr ===
	case 0xC1:
		c.Regs.SetBC(c.pop())
		return 12
	case 0xD1:
		c.Regs.SetDE(c.pop())
		return 12
	case 0xE1:
		c.Regs.SetHL(c.pop())
		return 12
	case 0xF1:
		c.Regs.SetAF(c.pop())
		return 12

	// === JP cc, d16 ===
	case 0xC2: // JP NZ, d16
		addr := c.fetchWord()
		if !c.Regs.GetFlag(FlagZ) {
			c.Regs.PC = addr
			return 16
		}
		return 12
	case 0xCA: // JP Z, d16
		addr := c.fetchWord()
		if c.Regs.GetFlag(FlagZ) {
			c.Regs.PC = addr
			return 16
		}
		return 12
	case 0xD2: // JP NC, d16
		addr := c.fetchWord()
		if !c.Regs.GetFlag(FlagC) {
			c.Regs.PC = addr
			return 16
		}
		return 12
	case 0xDA: // JP C, d16
		addr := c.fetchWord()
		if c.Regs.GetFlag(FlagC) {
			c.Regs.PC = addr
			return 16
		}
		return 12

	// === JP d16 ===
	case 0xC3:
		c.Regs.PC = c.fetchWord()
		return 16

	// === CALL cc, d16 ===
	case 0xC4: // CALL NZ, d16
		addr := c.fetchWord()
		if !c.Regs.GetFlag(FlagZ) {
			c.push(c.Regs.PC)
			c.Regs.PC = addr
			return 24
		}
		return 12
	case 0xCC: // CALL Z, d16
		addr := c.fetchWord()
		if c.Regs.GetFlag(FlagZ) {
			c.push(c.Regs.PC)
			c.Regs.PC = addr
			return 24
		}
		return 12
	case 0xD4: // CALL NC, d16
		addr := c.fetchWord()
		if !c.Regs.GetFlag(FlagC) {
			c.push(c.Regs.PC)
			c.Regs.PC = addr
			return 24
		}
		return 12
	case 0xDC: // CALL C, d16
		addr := c.fetchWord()
		if c.Regs.GetFlag(FlagC) {
			c.push(c.Regs.PC)
			c.Regs.PC = addr
			return 24
		}
		return 12

	// === PUSH rr ===
	case 0xC5:
		c.push(c.Regs.GetBC())
		return 16
	case 0xD5:
		c.push(c.Regs.GetDE())
		return 16
	case 0xE5:
		c.push(c.Regs.GetHL())
		return 16
	case 0xF5:
		c.push(c.Regs.GetAF())
		return 16

	// === ALU A, d8 ===
	case 0xC6: // ADD A, d8
		c.add(c.fetchByte())
		return 8
	case 0xCE: // ADC A, d8
		c.adc(c.fetchByte())
		return 8
	case 0xD6: // SUB d8
		c.sub(c.fetchByte())
		return 8
	case 0xDE: // SBC A, d8
		c.sbc(c.fetchByte())
		return 8
	case 0xE6: // AND d8
		c.and(c.fetchByte())
		return 8
	case 0xEE: // XOR d8
		c.xor(c.fetchByte())
		return 8
	case 0xF6: // OR d8
		c.or(c.fetchByte())
		return 8
	case 0xFE: // CP d8
		c.cp(c.fetchByte())
		return 8

	// === RST ===
	case 0xC7:
		c.push(c.Regs.PC)
		c.Regs.PC = 0x00
		return 16
	case 0xCF:
		c.push(c.Regs.PC)
		c.Regs.PC = 0x08
		return 16
	case 0xD7:
		c.push(c.Regs.PC)
		c.Regs.PC = 0x10
		return 16
	case 0xDF:
		c.push(c.Regs.PC)
		c.Regs.PC = 0x18
		return 16
	case 0xE7:
		c.push(c.Regs.PC)
		c.Regs.PC = 0x20
		return 16
	case 0xEF:
		c.push(c.Regs.PC)
		c.Regs.PC = 0x28
		return 16
	case 0xF7:
		c.push(c.Regs.PC)
		c.Regs.PC = 0x30
		return 16
	case 0xFF:
		c.push(c.Regs.PC)
		c.Regs.PC = 0x38
		return 16

	// === RET / RETI ===
	case 0xC9: // RET
		c.Regs.PC = c.pop()
		return 16
	case 0xD9: // RETI
		c.Regs.PC = c.pop()
		c.ime = true
		return 16

	// === CB prefix ===
	case 0xCB:
		cbOpcode := c.fetchByte()
		return c.executeCB(cbOpcode) + 4 // +4 for the CB prefix fetch

	// === CALL d16 ===
	case 0xCD:
		addr := c.fetchWord()
		c.push(c.Regs.PC)
		c.Regs.PC = addr
		return 24

	// === LD (0xFF00+d8), A / LD A, (0xFF00+d8) ===
	case 0xE0: // LDH (a8), A
		c.Bus.Write(0xFF00+uint16(c.fetchByte()), c.Regs.A)
		return 12
	case 0xF0: // LDH A, (a8)
		c.Regs.A = c.Bus.Read(0xFF00 + uint16(c.fetchByte()))
		return 12

	// === LD (0xFF00+C), A / LD A, (0xFF00+C) ===
	case 0xE2:
		c.Bus.Write(0xFF00+uint16(c.Regs.C), c.Regs.A)
		return 8
	case 0xF2:
		c.Regs.A = c.Bus.Read(0xFF00 + uint16(c.Regs.C))
		return 8

	// === DI / EI ===
	case 0xF3: // DI
		c.ime = false
		c.eiDelay = false
		return 4
	case 0xFB: // EI
		c.eiDelay = true
		return 4

	// === LD (d16), A / LD A, (d16) ===
	case 0xEA:
		c.Bus.Write(c.fetchWord(), c.Regs.A)
		return 16
	case 0xFA:
		c.Regs.A = c.Bus.Read(c.fetchWord())
		return 16

	// === JP (HL) ===
	case 0xE9:
		c.Regs.PC = c.Regs.GetHL()
		return 4

	// === LD SP, HL ===
	case 0xF9:
		c.Regs.SP = c.Regs.GetHL()
		return 8

	// === ADD SP, e8 ===
	case 0xE8:
		offset := int8(c.fetchByte())
		c.Regs.SP = c.addSPSigned(offset)
		return 16

	// === LD HL, SP+e8 ===
	case 0xF8:
		offset := int8(c.fetchByte())
		c.Regs.SetHL(c.addSPSigned(offset))
		return 12

	// === Undefined opcodes ===
	case 0xD3, 0xDB, 0xDD, 0xE3, 0xE4, 0xEB, 0xEC, 0xED, 0xF4, 0xFC, 0xFD:
		// These are undefined on the Game Boy and effectively act as NOPs or hangs
		return 4

	default:
		return 4
	}
}
