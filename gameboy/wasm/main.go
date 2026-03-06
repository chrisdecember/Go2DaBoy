//go:build js && wasm

package main

import (
	"syscall/js"

	"yukudanshi/gameboy/internal"
	"yukudanshi/gameboy/internal/joypad"
)

var emu *internal.Emulator

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

	// Keep the Go runtime alive
	select {}
}

func loadROM(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return js.ValueOf("no ROM data provided")
	}

	jsArray := args[0]
	length := jsArray.Length()
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
	if len(samples) == 0 {
		return nil
	}

	jsArray := js.Global().Get("Float32Array").New(len(samples))
	for i, s := range samples {
		jsArray.SetIndex(i, s)
	}
	return jsArray
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

// mapButton maps JS button codes to joypad buttons
// 0=A, 1=B, 2=Select, 3=Start, 4=Right, 5=Left, 6=Up, 7=Down
func mapButton(code int) int {
	if code >= 0 && code <= 7 {
		return code
	}
	return -1
}
