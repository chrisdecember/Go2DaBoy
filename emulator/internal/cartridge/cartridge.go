package cartridge

import (
	"fmt"
	"os"
)

// Cartridge represents a ROM cartridge
type Cartridge struct {
	Data    []byte
	Title   string
	Type    uint8
	ROMSize uint
	RAMSize uint
	MBC     MBC
}

// LoadFromFile loads a cartridge ROM from a file
func LoadFromFile(filename string) (*Cartridge, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read ROM file: %w", err)
	}
	return LoadFromBytes(data)
}

// LoadFromBytes loads a cartridge ROM from a byte slice
func LoadFromBytes(data []byte) (*Cartridge, error) {
	if len(data) < 0x150 {
		return nil, fmt.Errorf("ROM too small, not a valid ROM")
	}

	cart := &Cartridge{
		Data: data,
	}
	cart.readHeader()
	cart.initMBC()

	return cart, nil
}

func (c *Cartridge) readHeader() {
	// Title (0x134-0x143)
	titleEnd := 0x143
	for i := 0x134; i < titleEnd; i++ {
		if c.Data[i] == 0 {
			titleEnd = i
			break
		}
	}
	c.Title = string(c.Data[0x134:titleEnd])

	// Cartridge type
	c.Type = c.Data[0x147]

	// ROM size
	switch c.Data[0x148] {
	case 0x00:
		c.ROMSize = 32 * 1024
	case 0x01:
		c.ROMSize = 64 * 1024
	case 0x02:
		c.ROMSize = 128 * 1024
	case 0x03:
		c.ROMSize = 256 * 1024
	case 0x04:
		c.ROMSize = 512 * 1024
	case 0x05:
		c.ROMSize = 1024 * 1024
	case 0x06:
		c.ROMSize = 2 * 1024 * 1024
	case 0x07:
		c.ROMSize = 4 * 1024 * 1024
	case 0x08:
		c.ROMSize = 8 * 1024 * 1024
	default:
		c.ROMSize = 32 * 1024
	}

	// RAM size
	switch c.Data[0x149] {
	case 0x00:
		c.RAMSize = 0
	case 0x01:
		c.RAMSize = 2 * 1024
	case 0x02:
		c.RAMSize = 8 * 1024
	case 0x03:
		c.RAMSize = 32 * 1024
	case 0x04:
		c.RAMSize = 128 * 1024
	case 0x05:
		c.RAMSize = 64 * 1024
	default:
		c.RAMSize = 0
	}
}

func (c *Cartridge) initMBC() {
	ramSize := int(c.RAMSize)

	switch c.Type {
	case 0x00: // ROM ONLY
		c.MBC = NewMBC0(c.Data)
	case 0x01: // MBC1
		c.MBC = NewMBC1(c.Data, 0)
	case 0x02: // MBC1+RAM
		c.MBC = NewMBC1(c.Data, ramSize)
	case 0x03: // MBC1+RAM+BATTERY
		c.MBC = NewMBC1(c.Data, ramSize)
	case 0x05: // MBC2
		c.MBC = NewMBC2(c.Data)
	case 0x06: // MBC2+BATTERY
		c.MBC = NewMBC2(c.Data)
	case 0x08: // ROM+RAM
		c.MBC = NewMBC0RAM(c.Data, ramSize)
	case 0x09: // ROM+RAM+BATTERY
		c.MBC = NewMBC0RAM(c.Data, ramSize)
	case 0x0F, 0x10, 0x11, 0x12, 0x13: // MBC3 variants
		c.MBC = NewMBC3(c.Data, ramSize)
	case 0x19, 0x1A, 0x1B, 0x1C, 0x1D, 0x1E: // MBC5 variants
		c.MBC = NewMBC5(c.Data, ramSize)
	default:
		// Default to MBC0 for unknown types
		c.MBC = NewMBC0(c.Data)
	}
}
