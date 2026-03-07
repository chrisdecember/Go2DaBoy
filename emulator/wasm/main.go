//go:build js && wasm

package main

import (
	"syscall/js"
	"unsafe"

	"go2daboy/emulator/internal"
	"go2daboy/emulator/internal/joypad"
)

var emu *internal.Emulator

// Pre-allocated JS buffers for audio transfer (avoids per-frame allocation)
var audioJSBuf js.Value  // Uint8Array
var audioJSBufCap int    // capacity in bytes

func main() {
	emu = internal.New()

	js.Global().Set("gbLoadROM", js.FuncOf(loadROM))
	js.Global().Set("gbRunFrame", js.FuncOf(runFrame))
	js.Global().Set("gbGetFrame", js.FuncOf(getFrame))
	js.Global().Set("gbGetAudio", js.FuncOf(getAudio))
	js.Global().Set("gbKeyDown", js.FuncOf(keyDown))
	js.Global().Set("gbKeyUp", js.FuncOf(keyUp))
	js.Global().Set("gbReset", js.FuncOf(reset))
	js.Global().Set("gbGetTitle", js.FuncOf(getTitle))
	js.Global().Set("gbSetPalette", js.FuncOf(setPalette))
	js.Global().Set("gbSetAudioRate", js.FuncOf(setAudioRate))

	// Keep the Go runtime alive
	select {}
}

func loadROM(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf("no ROM data provided")
	}

	jsArray := args[0]
	length := jsArray.Length()
	if length == 0 {
		return js.ValueOf("ROM data is empty")
	}
	data := make([]byte, length)
	js.CopyBytesToGo(data, jsArray)

	err := emu.LoadROM(data)
	if err != nil {
		return js.ValueOf(err.Error())
	}

	emu.Reset()
	return js.ValueOf("")
}

func runFrame(this js.Value, args []js.Value) interface{} {
	emu.RunFrame()
	return nil
}

func getFrame(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return nil
	}
	dst := args[0]
	fb := emu.GetFrameBuffer()
	js.CopyBytesToJS(dst, fb)
	return nil
}

func getAudio(this js.Value, args []js.Value) interface{} {
	samples := emu.GetAudioSamples()
	n := len(samples)
	if n == 0 {
		return nil
	}

	// Reinterpret []float32 as []byte for bulk copy (WASM is always little-endian)
	byteLen := n * 4
	byteSlice := unsafe.Slice((*byte)(unsafe.Pointer(&samples[0])), byteLen)

	// Grow the shared JS buffer if needed
	if byteLen > audioJSBufCap {
		// Round up to next power of 2 to avoid frequent re-allocs
		newCap := byteLen
		newCap--
		newCap |= newCap >> 1
		newCap |= newCap >> 2
		newCap |= newCap >> 4
		newCap |= newCap >> 8
		newCap |= newCap >> 16
		newCap++

		audioJSBuf = js.Global().Get("Uint8Array").New(newCap)
		audioJSBufCap = newCap
	}

	// Single bulk copy instead of N individual SetIndex calls
	js.CopyBytesToJS(audioJSBuf, byteSlice)

	// Return a Float32Array view over the copied bytes
	return js.Global().Get("Float32Array").New(
		audioJSBuf.Get("buffer"),
		audioJSBuf.Get("byteOffset"),
		n,
	)
}

func keyDown(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return nil
	}
	btn := mapButton(args[0].Int())
	if btn >= 0 {
		emu.KeyDown(joypad.Button(btn))
	}
	return nil
}

func keyUp(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return nil
	}
	btn := mapButton(args[0].Int())
	if btn >= 0 {
		emu.KeyUp(joypad.Button(btn))
	}
	return nil
}

func reset(this js.Value, args []js.Value) interface{} {
	emu.Reset()
	return nil
}

func getTitle(this js.Value, args []js.Value) interface{} {
	return js.ValueOf(emu.GetCartridgeTitle())
}

// setPalette accepts 4 colors as 12 ints: [R0,G0,B0, R1,G1,B1, R2,G2,B2, R3,G3,B3]
// ordered lightest to darkest.
func setPalette(this js.Value, args []js.Value) interface{} {
	if len(args) < 12 {
		return nil
	}
	var colors [4][4]uint8
	for i := 0; i < 4; i++ {
		colors[i][0] = uint8(args[i*3].Int())
		colors[i][1] = uint8(args[i*3+1].Int())
		colors[i][2] = uint8(args[i*3+2].Int())
		colors[i][3] = 0xFF
	}
	emu.SetPalette(colors)
	return nil
}

// setAudioRate adjusts the APU output sample rate for dynamic rate control.
func setAudioRate(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return nil
	}
	emu.APU.SetOutputRate(args[0].Int())
	return nil
}

// mapButton maps JS button codes to joypad buttons
// 0=A, 1=B, 2=Select, 3=Start, 4=Right, 5=Left, 6=Up, 7=Down
func mapButton(code int) int {
	if code >= 0 && code <= 7 {
		return code
	}
	return -1
}
