package debug

import (
	"crypto/sha256"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"

	"go2daboy/emulator/internal/ppu"
)

// FrameHash returns a hex SHA-256 hash of the framebuffer contents.
// Useful for detecting whether two frames are identical.
func FrameHash(fb []uint8) string {
	h := sha256.Sum256(fb)
	return fmt.Sprintf("%x", h)
}

// SaveFramePNG writes the 160x144 RGBA framebuffer to a PNG file.
func SaveFramePNG(fb []uint8, path string) error {
	img := image.NewRGBA(image.Rect(0, 0, ppu.ScreenWidth, ppu.ScreenHeight))
	for y := 0; y < ppu.ScreenHeight; y++ {
		for x := 0; x < ppu.ScreenWidth; x++ {
			i := (y*ppu.ScreenWidth + x) * 4
			img.SetRGBA(x, y, color.RGBA{
				R: fb[i],
				G: fb[i+1],
				B: fb[i+2],
				A: fb[i+3],
			})
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// FrameDiff compares two framebuffers and returns the number of differing pixels
// and a list of (x,y) coordinates of the first N differences (up to maxDiffs).
func FrameDiff(a, b []uint8, maxDiffs int) (diffCount int, diffs []PixelDiff) {
	if len(a) != ppu.FrameBufferSize || len(b) != ppu.FrameBufferSize {
		return -1, nil
	}
	for y := 0; y < ppu.ScreenHeight; y++ {
		for x := 0; x < ppu.ScreenWidth; x++ {
			i := (y*ppu.ScreenWidth + x) * 4
			if a[i] != b[i] || a[i+1] != b[i+1] || a[i+2] != b[i+2] || a[i+3] != b[i+3] {
				diffCount++
				if len(diffs) < maxDiffs {
					diffs = append(diffs, PixelDiff{
						X:  x,
						Y:  y,
						A:  [4]uint8{a[i], a[i+1], a[i+2], a[i+3]},
						B:  [4]uint8{b[i], b[i+1], b[i+2], b[i+3]},
					})
				}
			}
		}
	}
	return diffCount, diffs
}

// PixelDiff describes a single pixel difference between two frames.
type PixelDiff struct {
	X, Y int
	A, B [4]uint8 // RGBA values in frame A and B
}

// SaveDiffPNG creates a visual diff image: identical pixels are black,
// differing pixels are highlighted in red.
func SaveDiffPNG(a, b []uint8, path string) error {
	img := image.NewRGBA(image.Rect(0, 0, ppu.ScreenWidth, ppu.ScreenHeight))
	for y := 0; y < ppu.ScreenHeight; y++ {
		for x := 0; x < ppu.ScreenWidth; x++ {
			i := (y*ppu.ScreenWidth + x) * 4
			if a[i] != b[i] || a[i+1] != b[i+1] || a[i+2] != b[i+2] {
				img.SetRGBA(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
			} else {
				img.SetRGBA(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 255})
			}
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// RegionBlank checks if a rectangular region of the framebuffer is all one color.
// Returns true if every pixel in the region matches the color at (x0,y0).
func RegionBlank(fb []uint8, x0, y0, w, h int) bool {
	if x0+w > ppu.ScreenWidth || y0+h > ppu.ScreenHeight {
		return false
	}
	refIdx := (y0*ppu.ScreenWidth + x0) * 4
	refR, refG, refB := fb[refIdx], fb[refIdx+1], fb[refIdx+2]

	for y := y0; y < y0+h; y++ {
		for x := x0; x < x0+w; x++ {
			i := (y*ppu.ScreenWidth + x) * 4
			if fb[i] != refR || fb[i+1] != refG || fb[i+2] != refB {
				return false
			}
		}
	}
	return true
}

// UniqueColors counts the number of distinct colors in the framebuffer.
func UniqueColors(fb []uint8) int {
	seen := make(map[[3]uint8]bool)
	for i := 0; i < len(fb); i += 4 {
		c := [3]uint8{fb[i], fb[i+1], fb[i+2]}
		seen[c] = true
	}
	return len(seen)
}

// RowProfile returns per-row unique color counts and whether each row
// is "blank" (single color). Useful for detecting rendering gaps.
func RowProfile(fb []uint8) []RowInfo {
	result := make([]RowInfo, ppu.ScreenHeight)
	for y := 0; y < ppu.ScreenHeight; y++ {
		seen := make(map[[3]uint8]bool)
		for x := 0; x < ppu.ScreenWidth; x++ {
			i := (y*ppu.ScreenWidth + x) * 4
			c := [3]uint8{fb[i], fb[i+1], fb[i+2]}
			seen[c] = true
		}
		result[y] = RowInfo{
			Y:            y,
			UniqueColors: len(seen),
			Blank:        len(seen) == 1,
		}
	}
	return result
}

// RowInfo holds per-row analysis data.
type RowInfo struct {
	Y            int
	UniqueColors int
	Blank        bool
}

// BlankRowRanges returns contiguous ranges of blank rows.
// Useful for detecting "black bands" from broken rendering.
func BlankRowRanges(fb []uint8) [][2]int {
	profile := RowProfile(fb)
	var ranges [][2]int
	start := -1
	for i, row := range profile {
		if row.Blank {
			if start < 0 {
				start = i
			}
		} else {
			if start >= 0 {
				ranges = append(ranges, [2]int{start, i - 1})
				start = -1
			}
		}
	}
	if start >= 0 {
		ranges = append(ranges, [2]int{start, ppu.ScreenHeight - 1})
	}
	return ranges
}
