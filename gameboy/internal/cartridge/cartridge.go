package cartridge

import (
	"fmt"
	"os"
)

// CartridgeType holds information about the cartridge type
type CartridgeType uint8

// Cartridge types
const (
	ROMOnly CartridgeType = 0x00
	MBC1    CartridgeType = 0x01
	MBC1RAM CartridgeType = 0x02
	// Add more as needed
)

// Cartridge represents a Game Boy cartridge
type Cartridge struct {
	Data    []byte
	Title   string
	Type    CartridgeType
	ROMSize uint
	RAMSize uint
}

// LoadFromFile loads a cartridge ROM from a file
func LoadFromFile(filename string) (*Cartridge, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read ROM file: %w", err)
	}

	// Validate minimal ROM size
	if len(data) < 0x150 {
		return nil, fmt.Errorf("ROM file too small, not a valid GB ROM")
	}

	cart := &Cartridge{
		Data: data,
	}

	// Extract cartridge information from header
	cart.readHeader()

	return cart, nil
}

// readHeader reads the cartridge header information
func (c *Cartridge) readHeader() {
	// Title (older carts use 16 bytes, newer use 15)
	c.Title = string(c.Data[0x134:0x143])

	// Cartridge type
	c.Type = CartridgeType(c.Data[0x147])

	// ROM size
	switch c.Data[0x148] {
	case 0x00:
		c.ROMSize = 32 * 1024 // 32KB (2 banks)
	case 0x01:
		c.ROMSize = 64 * 1024 // 64KB (4 banks)
	case 0x02:
		c.ROMSize = 128 * 1024 // 128KB (8 banks)
		// Add more as needed
	}

	// RAM size
	switch c.Data[0x149] {
	case 0x00:
		c.RAMSize = 0
	case 0x01:
		c.RAMSize = 2 * 1024 // 2KB
	case 0x02:
		c.RAMSize = 8 * 1024 // 8KB
		// Add more as needed
	}
}

// GetROMBank returns a specific ROM bank
func (c *Cartridge) GetROMBank(bank int) []byte {
	start := bank * 0x4000
	end := start + 0x4000

	if end > len(c.Data) {
		end = len(c.Data)
	}

	return c.Data[start:end]
}
