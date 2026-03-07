package test

import (
	"fmt"
	"testing"

	"go2daboy/emulator/internal/debug"
	"go2daboy/emulator/internal/ppu"
)

// dmgColor returns the expected RGBA for a DMG palette shade (0-3).
// Matches the default palette in ppu.go.
var dmgColors = [4][4]uint8{
	{0x9B, 0xBC, 0x0F, 0xFF}, // 0 = lightest
	{0x8B, 0xAC, 0x0F, 0xFF}, // 1
	{0x30, 0x62, 0x30, 0xFF}, // 2
	{0x0F, 0x38, 0x0F, 0xFF}, // 3 = darkest
}

// assertPixelColor checks that pixel (x,y) matches the expected shade.
func assertPixelColor(t *testing.T, fb []uint8, x, y int, shade int, label string) {
	t.Helper()
	i := (y*ppu.ScreenWidth + x) * 4
	expected := dmgColors[shade]
	got := [4]uint8{fb[i], fb[i+1], fb[i+2], fb[i+3]}
	if got != expected {
		t.Errorf("%s: pixel (%d,%d) expected shade %d %v, got %v", label, x, y, shade, expected, got)
	}
}

// assertRegionColor checks all pixels in a rectangle match a shade.
func assertRegionColor(t *testing.T, fb []uint8, x0, y0, w, h, shade int, label string) {
	t.Helper()
	expected := dmgColors[shade]
	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			i := (y*ppu.ScreenWidth + x) * 4
			got := [4]uint8{fb[i], fb[i+1], fb[i+2], fb[i+3]}
			if got != expected {
				t.Errorf("%s: pixel (%d,%d) expected shade %d %v, got %v", label, x, y, shade, expected, got)
				return // report first failure only per region
			}
		}
	}
}

// ---------- Test ROM builders ----------

// buildSolidFillROM creates a ROM that fills the entire screen with the darkest color.
// Tile 0 gets all-0xFF data (color index 3), tile map is already all zeros (tile 0).
// With BGP=0xFC: color index 3 → shade 3 (darkest).
func buildSolidFillROM() []byte {
	r := debug.NewROMBuilder()
	r.DI()
	r.WaitVBlank()
	r.DisableLCD()

	// Write tile 0: all 0xFF (every pixel = color index 3)
	r.FillVRAM(0x0000, 0xFF, 16) // tile 0 at VRAM 0x0000

	// Tile map is already all 0 (pointing to tile 0) — VRAM initialized to 0

	// BGP=0xFC (default): index 0→shade 0, index 1→shade 3, index 2→shade 3, index 3→shade 3
	r.SetBGP(0xFC)

	// Enable LCD: BG on, LCD on
	r.EnableLCD(0x91)

	r.InfiniteLoop()
	return r.Build()
}

// buildLightFillROM creates a ROM that fills screen with lightest color.
// Tile 0 = all zeros (default), map = all zeros. BGP=0xFC: index 0→shade 0.
func buildLightFillROM() []byte {
	r := debug.NewROMBuilder()
	r.DI()
	r.WaitVBlank()
	r.DisableLCD()
	// Everything already zero/default
	r.SetBGP(0xFC)
	r.EnableLCD(0x91)
	r.InfiniteLoop()
	return r.Build()
}

// buildCheckerboardROM creates a ROM with an 8x8 tile checkerboard.
// Tile 0 = all lightest, Tile 1 = all darkest.
// Tile map alternates tile 0 and tile 1 in a checkerboard pattern.
func buildCheckerboardROM() []byte {
	r := debug.NewROMBuilder()
	r.DI()
	r.WaitVBlank()
	r.DisableLCD()

	// Tile 0 = all zeros (lightest) — already default
	// Tile 1 = all 0xFF (darkest)
	r.FillVRAM(0x0010, 0xFF, 16) // tile 1 starts at VRAM offset 0x10

	// Fill tile map with checkerboard: rows 0-17, cols 0-19 (visible area)
	// VRAM tile map at 0x9800 = VRAM offset 0x1800
	for row := 0; row < 18; row++ {
		for col := 0; col < 20; col++ {
			tileIdx := uint8((row + col) & 1) // 0 or 1
			if tileIdx == 1 {
				r.WriteTileMap(row, col, 1, false)
			}
			// tileIdx 0 is already default (VRAM zeroed)
		}
	}

	r.SetBGP(0xFC)
	r.SetScroll(0, 0)
	r.EnableLCD(0x91)
	r.InfiniteLoop()
	return r.Build()
}

// buildHorizontalStripesROM creates a tile where rows alternate between
// darkest and lightest, producing 1-pixel-tall horizontal stripes.
func buildHorizontalStripesROM() []byte {
	r := debug.NewROMBuilder()
	r.DI()
	r.WaitVBlank()
	r.DisableLCD()

	// Tile 0: alternating rows
	// Each tile row is 2 bytes. 0xFF,0xFF = color index 3 (darkest)
	// 0x00,0x00 = color index 0 (lightest)
	tile := [16]byte{
		0xFF, 0xFF, // row 0: darkest
		0x00, 0x00, // row 1: lightest
		0xFF, 0xFF, // row 2: darkest
		0x00, 0x00, // row 3: lightest
		0xFF, 0xFF, // row 4: darkest
		0x00, 0x00, // row 5: lightest
		0xFF, 0xFF, // row 6: darkest
		0x00, 0x00, // row 7: lightest
	}
	r.WriteTileData(0, tile)

	r.SetBGP(0xFC)
	r.EnableLCD(0x91)
	r.InfiniteLoop()
	return r.Build()
}

// buildScrollROM renders a distinctive pattern and scrolls by (SCY=8, SCX=4).
// Tile 0 = lightest, Tile 1 = darkest. Row 0 of tile map = all tile 1,
// other rows = tile 0. With SCY=8, the visible area starts at tile row 1,
// so the first visible row should be lightest (tile 0).
func buildScrollROM() []byte {
	r := debug.NewROMBuilder()
	r.DI()
	r.WaitVBlank()
	r.DisableLCD()

	// Tile 1 = all darkest
	r.FillVRAM(0x0010, 0xFF, 16)

	// Fill tile map row 0 (cols 0-31) with tile 1
	for col := 0; col < 32; col++ {
		r.WriteTileMap(0, col, 1, false)
	}
	// All other rows remain tile 0 (lightest) — default

	// Scroll: SCY=8 (skip tile row 0), SCX=4 (half-tile offset)
	r.SetScroll(8, 4)
	r.SetBGP(0xFC)
	r.EnableLCD(0x91)
	r.InfiniteLoop()
	return r.Build()
}

// buildSpriteROM places a single 8x8 sprite at screen position (40, 40).
// The sprite uses tile 1 (all darkest), BG is all lightest.
func buildSpriteROM() []byte {
	r := debug.NewROMBuilder()
	r.DI()
	r.WaitVBlank()
	r.DisableLCD()

	// Tile 1 = all darkest (for sprite)
	r.FillVRAM(0x0010, 0xFF, 16)

	// OBP0 palette: 0xE4 = identity (0→0, 1→1, 2→2, 3→3)
	r.SetOBP0(0xE4)
	r.SetBGP(0xFC)

	// Sprite 0: Y=40+16=56, X=40+8=48, Tile=1, Flags=0
	r.WriteOAMEntry(0, 56, 48, 1, 0x00)

	// Enable LCD with BG + OBJ enabled
	r.EnableLCD(0x93) // bit 7=LCD on, bit 4=tile data 8000, bit 1=OBJ on, bit 0=BG on
	r.InfiniteLoop()
	return r.Build()
}

// buildWindowROM enables the window at WY=72, WX=7 (left edge).
// BG = all lightest, window tiles = all darkest.
// Bottom half of screen should be darkest.
func buildWindowROM() []byte {
	r := debug.NewROMBuilder()
	r.DI()
	r.WaitVBlank()
	r.DisableLCD()

	// Tile 1 = all darkest
	r.FillVRAM(0x0010, 0xFF, 16)

	// Fill window tile map (0x9C00) with tile 1
	r.FillTileMap(1, true) // true = 0x9C00

	// Window at Y=72, X=7 (WX=7 means window starts at screen X=0)
	r.SetWindow(72, 7)
	r.SetBGP(0xFC)

	// LCDC: LCD on, window tile map=9C00, window enable, BG tile data=8000, BG map=9800, BG on
	// Bit 7=LCD, bit 6=window map 9C00, bit 5=window enable, bit 4=tile data 8000, bit 0=BG
	r.EnableLCD(0xF1)
	r.InfiniteLoop()
	return r.Build()
}

// buildFourShadesROM creates 4 tiles (one per shade) and arranges them
// in vertical columns: left quarter = shade 0, etc.
func buildFourShadesROM() []byte {
	r := debug.NewROMBuilder()
	r.DI()
	r.WaitVBlank()
	r.DisableLCD()

	// BGP = 0xE4: identity mapping (index 0→shade 0, 1→1, 2→2, 3→3)
	r.SetBGP(0xE4)

	// Tile 0: color index 0 (both planes 0x00) — already default
	// Tile 1: color index 1 (low plane 0xFF, high plane 0x00)
	tile1 := [16]byte{
		0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00,
		0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00,
	}
	r.WriteTileData(1, tile1)

	// Tile 2: color index 2 (low plane 0x00, high plane 0xFF)
	tile2 := [16]byte{
		0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF,
		0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF,
	}
	r.WriteTileData(2, tile2)

	// Tile 3: color index 3 (both planes 0xFF)
	tile3 := [16]byte{
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
		0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF,
	}
	r.WriteTileData(3, tile3)

	// Fill tile map: columns 0-4 = tile 0, 5-9 = tile 1, 10-14 = tile 2, 15-19 = tile 3
	for row := 0; row < 18; row++ {
		for col := 0; col < 20; col++ {
			tileIdx := uint8(col / 5)
			if tileIdx > 3 {
				tileIdx = 3
			}
			if tileIdx > 0 { // tile 0 is default
				r.WriteTileMap(row, col, tileIdx, false)
			}
		}
	}

	r.EnableLCD(0x91)
	r.InfiniteLoop()
	return r.Build()
}

// ---------- Pixel-perfect verification tests ----------

func runTestROM(t *testing.T, rom []byte, frames int) []uint8 {
	t.Helper()
	h := debug.NewHarness("")
	if err := h.LoadROMBytes(rom); err != nil {
		t.Fatal(err)
	}
	return h.RunFrames(frames, 0)
}

func TestSolidFillDark(t *testing.T) {
	fb := runTestROM(t, buildSolidFillROM(), 5)

	// Every pixel should be shade 3 (darkest)
	for y := 0; y < ppu.ScreenHeight; y++ {
		for x := 0; x < ppu.ScreenWidth; x++ {
			i := (y*ppu.ScreenWidth + x) * 4
			got := [4]uint8{fb[i], fb[i+1], fb[i+2], fb[i+3]}
			if got != dmgColors[3] {
				t.Fatalf("pixel (%d,%d) expected darkest %v, got %v", x, y, dmgColors[3], got)
			}
		}
	}
	t.Log("All 23,040 pixels are darkest shade — PASS")
}

func TestSolidFillLight(t *testing.T) {
	fb := runTestROM(t, buildLightFillROM(), 5)

	for y := 0; y < ppu.ScreenHeight; y++ {
		for x := 0; x < ppu.ScreenWidth; x++ {
			i := (y*ppu.ScreenWidth + x) * 4
			got := [4]uint8{fb[i], fb[i+1], fb[i+2], fb[i+3]}
			if got != dmgColors[0] {
				t.Fatalf("pixel (%d,%d) expected lightest %v, got %v", x, y, dmgColors[0], got)
			}
		}
	}
	t.Log("All 23,040 pixels are lightest shade — PASS")
}

func TestCheckerboard(t *testing.T) {
	fb := runTestROM(t, buildCheckerboardROM(), 5)

	for tileRow := 0; tileRow < 18; tileRow++ {
		for tileCol := 0; tileCol < 20; tileCol++ {
			expectedShade := 0 // lightest
			if (tileRow+tileCol)&1 == 1 {
				expectedShade = 3 // darkest
			}
			// Check center pixel of each tile
			px := tileCol*8 + 4
			py := tileRow*8 + 4
			assertPixelColor(t, fb, px, py, expectedShade,
				fmt.Sprintf("tile(%d,%d)", tileRow, tileCol))
		}
	}
	t.Log("Checkerboard pattern verified — PASS")
}

func TestHorizontalStripes(t *testing.T) {
	fb := runTestROM(t, buildHorizontalStripesROM(), 5)

	for y := 0; y < ppu.ScreenHeight; y++ {
		expectedShade := 3 // darkest (even tile-row pixels)
		if (y%8)%2 == 1 {
			expectedShade = 0 // lightest (odd tile-row pixels)
		}
		// Check a few pixels across the row
		for _, x := range []int{0, 40, 80, 120, 159} {
			assertPixelColor(t, fb, x, y, expectedShade,
				fmt.Sprintf("stripe y=%d", y))
		}
	}
	t.Log("Horizontal stripes verified — PASS")
}

func TestScrollOffset(t *testing.T) {
	fb := runTestROM(t, buildScrollROM(), 5)

	// SCY=8 means tile row 0 (darkest) is scrolled out of view.
	// Tile row 1+ are all tile 0 (lightest). So entire visible area should be lightest.
	// SCX=4 means we're shifted 4 pixels right, but all tiles are the same (lightest),
	// so every pixel is still lightest.
	assertRegionColor(t, fb, 0, 0, ppu.ScreenWidth, ppu.ScreenHeight, 0, "scrolled view")
	t.Log("Scroll offset verified (darkest row scrolled out) — PASS")
}

func TestScrollVisible(t *testing.T) {
	// Variation: SCY=0, darkest row IS visible at top
	r := debug.NewROMBuilder()
	r.DI()
	r.WaitVBlank()
	r.DisableLCD()
	r.FillVRAM(0x0010, 0xFF, 16) // tile 1 = darkest
	for col := 0; col < 32; col++ {
		r.WriteTileMap(0, col, 1, false) // row 0 = tile 1
	}
	r.SetScroll(0, 0)
	r.SetBGP(0xFC)
	r.EnableLCD(0x91)
	r.InfiniteLoop()

	fb := runTestROM(t, r.Build(), 5)

	// Top 8 rows should be darkest (tile row 0 = tile 1)
	assertRegionColor(t, fb, 0, 0, ppu.ScreenWidth, 8, 3, "darkest top row")
	// Row 8+ should be lightest (tile 0)
	assertRegionColor(t, fb, 0, 8, ppu.ScreenWidth, 8, 0, "lightest second row")
	t.Log("Scroll visible row verified — PASS")
}

func TestSpriteRendering(t *testing.T) {
	fb := runTestROM(t, buildSpriteROM(), 5)

	// Sprite at screen position (40, 40), 8x8, tile 1 (all darkest)
	// BG is lightest everywhere.

	// Check BG pixel outside sprite
	assertPixelColor(t, fb, 0, 0, 0, "BG corner")
	assertPixelColor(t, fb, 100, 100, 0, "BG center-ish")

	// Check sprite pixels: sprite covers (40,40) to (47,47)
	// OBP0=0xE4: index 3→shade 3. Tile is all 0xFF = index 3 = shade 3.
	for y := 40; y < 48; y++ {
		for x := 40; x < 48; x++ {
			assertPixelColor(t, fb, x, y, 3, "sprite body")
		}
	}

	// Check pixel just outside sprite is BG
	assertPixelColor(t, fb, 39, 40, 0, "left of sprite")
	assertPixelColor(t, fb, 48, 40, 0, "right of sprite")
	assertPixelColor(t, fb, 40, 39, 0, "above sprite")
	assertPixelColor(t, fb, 40, 48, 0, "below sprite")

	t.Log("Sprite rendering verified (8x8 at 40,40) — PASS")
}

func TestWindowOverlay(t *testing.T) {
	fb := runTestROM(t, buildWindowROM(), 5)

	// Window starts at Y=72 (halfway down), X=0 (WX=7 → screen X=0)
	// Above window: BG = lightest (tile 0, all zeros)
	// At/below window: window = darkest (tile 1, all 0xFF)

	// Check BG above window
	assertRegionColor(t, fb, 0, 0, ppu.ScreenWidth, 8, 0, "BG above window")
	assertPixelColor(t, fb, 80, 60, 0, "BG mid-upper")

	// Check window area
	assertPixelColor(t, fb, 0, 72, 3, "window start row")
	assertPixelColor(t, fb, 80, 100, 3, "window mid")
	assertPixelColor(t, fb, 159, 143, 3, "window bottom-right")

	t.Log("Window overlay verified (WY=72) — PASS")
}

func TestFourShades(t *testing.T) {
	fb := runTestROM(t, buildFourShadesROM(), 5)

	// Columns: 0-39 = shade 0, 40-79 = shade 1, 80-119 = shade 2, 120-159 = shade 3
	// (5 tiles × 8px = 40px per shade column)
	for shade := 0; shade < 4; shade++ {
		x := shade*40 + 20 // center of each column
		y := 72            // middle of screen
		assertPixelColor(t, fb, x, y, shade,
			fmt.Sprintf("shade %d column", shade))
	}

	// Check boundaries
	assertPixelColor(t, fb, 39, 0, 0, "last pixel of shade 0")
	assertPixelColor(t, fb, 40, 0, 1, "first pixel of shade 1")

	t.Log("Four shades verified — PASS")
}

// TestRenderingRegression is a meta-test that runs all visual ROMs and reports
// a summary. Useful as a single "did rendering break?" check.
func TestRenderingRegression(t *testing.T) {
	tests := []struct {
		name  string
		rom   []byte
		check func(t *testing.T, fb []uint8)
	}{
		{"SolidDark", buildSolidFillROM(), func(t *testing.T, fb []uint8) {
			assertRegionColor(t, fb, 0, 0, 160, 144, 3, "entire screen darkest")
		}},
		{"SolidLight", buildLightFillROM(), func(t *testing.T, fb []uint8) {
			assertRegionColor(t, fb, 0, 0, 160, 144, 0, "entire screen lightest")
		}},
		{"Checkerboard", buildCheckerboardROM(), func(t *testing.T, fb []uint8) {
			// Just check a few tiles
			assertPixelColor(t, fb, 4, 4, 0, "tile(0,0) light")
			assertPixelColor(t, fb, 12, 4, 3, "tile(0,1) dark")
		}},
		{"Stripes", buildHorizontalStripesROM(), func(t *testing.T, fb []uint8) {
			assertPixelColor(t, fb, 80, 0, 3, "stripe row 0 dark")
			assertPixelColor(t, fb, 80, 1, 0, "stripe row 1 light")
		}},
		{"Sprite", buildSpriteROM(), func(t *testing.T, fb []uint8) {
			assertPixelColor(t, fb, 44, 44, 3, "sprite center")
			assertPixelColor(t, fb, 0, 0, 0, "bg corner")
		}},
		{"Window", buildWindowROM(), func(t *testing.T, fb []uint8) {
			assertPixelColor(t, fb, 80, 0, 0, "above window")
			assertPixelColor(t, fb, 80, 80, 3, "in window")
		}},
	}

	passed := 0
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fb := runTestROM(t, tc.rom, 5)
			tc.check(t, fb)
			if !t.Failed() {
				passed++
			}
		})
	}
	t.Logf("Rendering regression: %d/%d passed", passed, len(tests))
}
