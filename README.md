<p align="center">
  <br>
  <strong style="font-size: 2em;">Go2DaBoy</strong>
  <br>
  <em>A retro handheld console emulator built with Go + WebAssembly</em>
  <br><br>
  <a href="#features">Features</a> &middot;
  <a href="#quick-start">Quick Start</a> &middot;
  <a href="#deployment">Deployment</a> &middot;
  <a href="#controls">Controls</a> &middot;
  <a href="#architecture">Architecture</a>
</p>

---

Go2DaBoy is a dot-matrix handheld console emulator that runs entirely in the browser. The emulator core is written in Go, compiled to WebAssembly, and wrapped in a responsive frontend that works on desktop and mobile — no install required.

## Features

- **Accurate emulation** — CPU, PPU, APU, and timer subsystems with cycle-accurate timing
- **Full audio** — 4-channel sound synthesis with high-pass filtering, ring buffer, and zero-GC audio pipeline
- **6 cartridge types** — MBC0, MBC1, MBC2, MBC3, MBC5, and ROM+RAM
- **Mobile-first UI** — touch D-pad, A/B buttons, responsive layout with landscape support
- **Fast-forward** — hold Space or tap the FF button for 4x speed
- **8 color palettes** — Classic, Grayscale, Pocket, Light, Nostalgia, Crimson, Ocean, Lavender — plus a custom color picker
- **Rebindable controls** — remap every key, persisted in localStorage
- **Zero dependencies** — no frameworks, no npm, no bundler. Vanilla JS + Go + Make

## Quick Start

**Prerequisites:** [Go 1.21+](https://go.dev/dl/) and Make.

```bash
# Clone the repo
git clone https://github.com/chrisdecember/Go2DaBoy.git
cd Go2DaBoy

# Build the WASM binary and start the server
make serve
```

Open `http://localhost:8080` in your browser. Click **LOAD** to pick a `.gb` ROM file. That's it.

### Build only (no server)

```bash
make build
```

This compiles `build/main.wasm` and copies `wasm_exec.js`. You can then serve the `web/` and `build/` directories with any static file server.

## Controls

| Action | Keyboard | Touch |
|--------|----------|-------|
| D-Pad | Arrow keys | On-screen D-pad |
| A | X | On-screen A button |
| B | Z | On-screen B button |
| Start | Enter | On-screen START |
| Select | Left Shift | On-screen SELECT |
| Fast Forward | Space (hold) | FF button (toggle) |

All keyboard bindings can be remapped via the **KEYS** button in the toolbar.

## Color Palettes

Switch palettes on the fly via the **COLORS** button. Includes 8 presets:

| Palette | Description |
|---------|-------------|
| Classic | The original dot-matrix green |
| Grayscale | Clean monochrome |
| Pocket | Muted green-gray |
| Light | Bright and vibrant |
| Nostalgia | Warm sepia tones |
| Crimson | Red theme |
| Ocean | Cool blue theme |
| Lavender | Purple theme |

You can also create custom palettes with the built-in color picker and hex input.

## Deployment

Go2DaBoy is a static site once built. The WASM binary + a few HTML/JS/CSS files are all you need.

### Option 1: GitHub Pages

```bash
make build
```

Copy these files into your GitHub Pages root (or a `docs/` folder):

```
web/index.html
web/style.css
web/app.js
build/main.wasm
build/wasm_exec.js
```

Make sure `main.wasm` and `wasm_exec.js` are served from the same directory as `index.html`, or adjust the paths in `index.html` and `app.js`.

### Option 2: Any static host (Netlify, Vercel, Cloudflare Pages, etc.)

```bash
make build

# Copy everything into one deploy directory
mkdir -p dist
cp web/* dist/
cp build/main.wasm dist/
cp build/wasm_exec.js dist/
```

Deploy the `dist/` folder. Ensure your host serves `.wasm` files with the `application/wasm` MIME type (most modern hosts do this by default).

### Option 3: Self-hosted with the built-in Go server

```bash
make serve
# or with a custom port:
go run ./emulator/cmd/ -serve -port 3000
```

The Go server handles WASM MIME types and serves everything from `web/` and `build/`.

### Option 4: Docker (minimal)

```dockerfile
FROM golang:1.21 AS build
WORKDIR /app
COPY . .
RUN make build

FROM nginx:alpine
COPY --from=build /app/web /usr/share/nginx/html
COPY --from=build /app/build/main.wasm /usr/share/nginx/html/
COPY --from=build /app/build/wasm_exec.js /usr/share/nginx/html/
```

```bash
docker build -t go2daboy .
docker run -p 8080:80 go2daboy
```

## Architecture

```
Go2DaBoy/
├── emulator/
│   ├── cmd/              Server entry point
│   ├── wasm/             WASM bridge (Go ↔ JS)
│   └── internal/
│       ├── cpu/          Sharp LR35902 — all opcodes + CB prefix
│       ├── ppu/          Pixel processing — BG, window, sprites
│       ├── apu/          4-channel audio synthesis + high-pass filter
│       ├── timer/        DIV/TIMA with falling-edge detection
│       ├── joypad/       Button input with interrupt support
│       ├── cartridge/    ROM loading + MBC0/1/2/3/5
│       └── memory/       Memory bus with DMA
├── web/
│   ├── index.html        Console UI shell
│   ├── app.js            Emulation loop, audio, controls, palettes
│   └── style.css         Responsive console styling
└── Makefile
```

The emulator core is pure Go with no external dependencies. It compiles to ~2MB of WASM. The frontend is vanilla JavaScript — no build step, no framework.

### How it works

1. Go compiles to WASM and exposes functions via `syscall/js`
2. The JS emulation loop calls `gbRunFrame()` at ~59.7 Hz using `requestAnimationFrame`
3. Frame data is bulk-copied from Go memory to a canvas via `ImageData`
4. Audio samples are transferred as `Float32Array` through a shared ring buffer
5. Input flows from touch/keyboard events → JS → Go via `gbKeyDown`/`gbKeyUp`

## Cartridge Support

| Type | Variants | Max ROM | Max RAM |
|------|----------|---------|---------|
| MBC0 | ROM only, ROM+RAM | 32 KB | 8 KB |
| MBC1 | Standard, +RAM, +Battery | 2 MB | 32 KB |
| MBC2 | Standard, +Battery | 256 KB | 512 bytes |
| MBC3 | Standard, +RAM, +Battery, +RTC | 2 MB | 32 KB |
| MBC5 | Standard, +RAM, +Battery | 8 MB | 128 KB |

## Testing

The project includes a test suite based on the Blargg test ROMs:

```bash
# Run all tests
go test ./emulator/test/

# Run with verbose output
go test -v ./emulator/test/
```

Test ROMs go in `emulator/test/testdata/blargg/`.

---

<p align="center">
  Designed by <strong>Chris Maloney</strong>, assisted by <strong>Claude Code</strong>
</p>
