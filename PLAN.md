# Pixel FIFO + M-Cycle Stepping Implementation Plan

## Overview

Two tightly coupled changes that together fix:
- **Screen tearing/ripple** on vertical scrolling (FIFO gives per-tile SCX/SCY sampling)
- **mem_timing Blargg failures** (M-cycle stepping interleaves CPU and PPU correctly)
- **Mode 3 timing accuracy** (FIFO duration emerges naturally, no formula)

## Current Architecture (what we have)

```
Emulator.Step():
  1. CPU executes ENTIRE instruction → returns N T-cycles
  2. Timer.Step(N)    ← all N cycles at once
  3. PPU.Step(N)      ← all N cycles at once (ticks internally 1-by-1)
  4. APU.Step(N)      ← all N cycles at once
  5. Bus.StepDMA(N)   ← all N cycles at once
```

Problem: A 12-cycle instruction does its memory read on cycle 4 and write on cycle 8,
but PPU/Timer don't see those intermediate states. They get all 12 cycles after the
instruction is fully done.

## New Architecture (what we need)

```
CPU.Step():
  For each M-cycle (4 T-cycles) within an instruction:
    1. Do this M-cycle's work (fetch/read/write/internal)
    2. Call bus.Tick(4) → advances PPU, Timer, APU, DMA by 4 T-cycles
  Return total cycles consumed (unchanged API for RunFrame)
```

The CPU calls `bus.Tick(4)` after every M-cycle boundary. This means when the CPU
writes to SCY on M-cycle 3 of an instruction, the PPU sees that write immediately
and uses it for the next tile fetch. This is M-cycle accurate.

---

## Phase 1: M-Cycle Stepping (CPU + Bus)

### 1a. Add `Tick` to the Bus

Add a method `Bus.Tick(cycles int)` that advances all subsystems:

```go
// Tick advances all subsystems by the given T-cycles (always 4 for M-cycle stepping).
// Called by the CPU between each M-cycle of an instruction.
func (b *Bus) Tick(cycles int) {
    // Timer
    if b.Timer.Step(cycles) {
        b.ifReg |= 0x04 // Timer interrupt
    }
    // PPU
    ppuIRQ := b.PPU.Step(cycles)
    if ppuIRQ&0x01 != 0 {
        b.ifReg |= 0x01 // VBlank
    }
    if ppuIRQ&0x02 != 0 {
        b.ifReg |= 0x02 // STAT
    }
    // APU
    b.APU.Step(cycles)
    // DMA
    b.StepDMA(cycles)
}
```

### 1b. Refactor CPU to tick after each M-cycle

The CPU currently does all work then returns a cycle count. We need to insert
`c.tick(4)` calls at M-cycle boundaries. The CPU gets a `tick` function reference
set during init.

**Pattern for instruction types:**

| Instruction type | M-cycles | Pattern |
|---|---|---|
| `NOP` | 1 | fetch(1M) |
| `LD r,r'` | 1 | fetch(1M) |
| `LD r,d8` | 2 | fetch(1M) + read_imm(1M) |
| `LD r,(HL)` | 2 | fetch(1M) + read_mem(1M) |
| `LD (HL),r` | 2 | fetch(1M) + write_mem(1M) |
| `LD (HL),d8` | 3 | fetch(1M) + read_imm(1M) + write_mem(1M) |
| `INC (HL)` | 3 | fetch(1M) + read_mem(1M) + write_mem(1M) |
| `LD rr,d16` | 3 | fetch(1M) + read_lo(1M) + read_hi(1M) |
| `PUSH rr` | 4 | fetch(1M) + internal(1M) + write_hi(1M) + write_lo(1M) |
| `POP rr` | 3 | fetch(1M) + read_lo(1M) + read_hi(1M) |
| `JP d16` | 4 | fetch(1M) + read_lo(1M) + read_hi(1M) + internal(1M) |
| `JP cc,d16 (taken)` | 4 | same as JP |
| `JP cc,d16 (not taken)` | 3 | fetch(1M) + read_lo(1M) + read_hi(1M) |
| `JR e8` | 3 | fetch(1M) + read_imm(1M) + internal(1M) |
| `JR cc,e8 (not taken)` | 2 | fetch(1M) + read_imm(1M) |
| `CALL d16` | 6 | fetch(1M) + read_lo(1M) + read_hi(1M) + internal(1M) + push_hi(1M) + push_lo(1M) |
| `RET` | 4 | fetch(1M) + pop_lo(1M) + pop_hi(1M) + internal(1M) |
| `RET cc (taken)` | 5 | fetch(1M) + internal(1M) + pop_lo(1M) + pop_hi(1M) + internal(1M) |
| `RET cc (not taken)` | 2 | fetch(1M) + internal(1M) |
| `RST` | 4 | fetch(1M) + internal(1M) + push_hi(1M) + push_lo(1M) |
| `ADD SP,e8` | 4 | fetch(1M) + read_imm(1M) + internal(1M) + internal(1M) |
| `LD (d16),A` | 4 | fetch(1M) + read_lo(1M) + read_hi(1M) + write_mem(1M) |
| `CB prefix` | +1M | fetch_CB(1M) then depends on operation |
| `INC/DEC rr` | 2 | fetch(1M) + internal(1M) |
| `ADD HL,rr` | 2 | fetch(1M) + internal(1M) |
| `LD SP,HL` | 2 | fetch(1M) + internal(1M) |
| `HALT` | 1 | fetch(1M) |
| Interrupt dispatch | 5 | 2x internal(1M) + push_hi(1M) + push_lo(1M) + internal(1M) |

**Implementation approach:**

Instead of returning cycle counts, each instruction calls `c.tick(4)` at each
M-cycle boundary. The fetch of the opcode itself is the first M-cycle, ticked
by the Step() wrapper. Each `Bus.Read()` and `Bus.Write()` represents one M-cycle.

We introduce thin wrappers:

```go
// read does a bus read and ticks 1 M-cycle
func (c *CPU) read(addr uint16) uint8 {
    val := c.Bus.Read(addr)
    c.Bus.Tick(4)
    return val
}

// write does a bus write and ticks 1 M-cycle
func (c *CPU) write(addr uint16, val uint8) {
    c.Bus.Write(addr, val)
    c.Bus.Tick(4)
}

// idle ticks 1 M-cycle with no bus activity
func (c *CPU) idle() {
    c.Bus.Tick(4)
}
```

Then every instruction uses `c.read()`, `c.write()`, `c.idle()` instead of
`c.Bus.Read()`, `c.Bus.Write()`. The opcode fetch M-cycle is ticked in `Step()`.

**fetchByte and fetchWord also tick:**

```go
func (c *CPU) fetchByte() uint8 {
    val := c.Bus.Read(c.Regs.PC)
    c.Regs.PC++
    c.Bus.Tick(4)
    return val
}
```

**Total cycles are tracked by counting ticks:**

```go
func (c *CPU) Step() int {
    c.cycles = 0
    // The opcode fetch is the first M-cycle
    opcode := c.fetchByte()  // ticks 4
    c.execute(opcode)        // ticks remaining M-cycles
    return c.cycles
}
```

Where `Tick(4)` also increments `c.cycles += 4`.

### 1c. Update Emulator.Step()

Simplify to just call CPU.Step() — the bus tick now handles everything:

```go
func (e *Emulator) Step() int {
    return e.CPU.Step()
}
```

Timer, PPU, APU, DMA are all advanced inside `Bus.Tick()`.

### 1d. Update opcodes.go and cb_opcodes.go

Every instruction stops returning a cycle count and instead uses `c.read()`,
`c.write()`, `c.idle()` which self-tick. Examples:

```go
// Before:
case 0x46: // LD B, (HL)
    c.Regs.B = c.Bus.Read(c.Regs.GetHL())
    return 8

// After:
case 0x46: // LD B, (HL)
    c.Regs.B = c.read(c.Regs.GetHL())
    // fetchByte ticked 4, read ticked 4 = 8 total
```

```go
// Before:
case 0x34: // INC (HL)
    val := c.Bus.Read(c.Regs.GetHL())
    c.Bus.Write(c.Regs.GetHL(), c.inc(val))
    return 12

// After:
case 0x34: // INC (HL)
    val := c.read(c.Regs.GetHL())   // +4 (total 8 with fetch)
    c.write(c.Regs.GetHL(), c.inc(val)) // +4 (total 12)
```

```go
// Before:
case 0xC5: // PUSH BC
    c.push(c.Regs.GetBC())
    return 16

// After:
case 0xC5: // PUSH BC
    c.idle()                          // internal cycle
    c.write(c.Regs.SP-1, c.Regs.B)   // push high byte first
    c.write(c.Regs.SP-2, c.Regs.C)
    c.Regs.SP -= 2
    // fetch(4) + idle(4) + write(4) + write(4) = 16
```

### 1e. Interrupt dispatch timing

```go
func (c *CPU) handleInterrupts() int {
    // ... find pending interrupt ...
    c.cycles = 0
    c.idle()  // 2 internal M-cycles
    c.idle()
    c.write(c.Regs.SP-1, uint8(c.Regs.PC>>8))  // push PC high
    c.write(c.Regs.SP-2, uint8(c.Regs.PC))      // push PC low
    c.Regs.SP -= 2
    c.Regs.PC = vector
    c.idle()  // final internal M-cycle
    return c.cycles  // = 20
}
```

---

## Phase 2: Pixel FIFO PPU Rewrite

### 2a. Data Structures

```go
// FIFOPixel holds one pixel in the FIFO
type FIFOPixel struct {
    Color    uint8 // 0-3 color index (pre-palette)
    Palette  uint8 // 0=BGP, 1=OBP0, 2=OBP1
    BgPrio   bool  // OBJ-to-BG priority bit (from sprite attribute bit 7)
    IsSprite bool  // true if this came from a sprite
}

// PixelFIFO is an 8-16 entry FIFO backed by a fixed array
type PixelFIFO struct {
    pixels [16]FIFOPixel
    head   int
    size   int
}

func (f *PixelFIFO) Push(px FIFOPixel) { ... }  // append
func (f *PixelFIFO) Pop() FIFOPixel { ... }      // remove from front
func (f *PixelFIFO) Clear() { ... }
func (f *PixelFIFO) Size() int { return f.size }

// FetcherStep enum
const (
    FetchTile     = 0
    FetchDataLow  = 1
    FetchDataHigh = 2
    FetchSleep    = 3
    FetchPush     = 4
)

// Fetcher reads tiles from VRAM and pushes rows of 8 pixels to the BG FIFO
type Fetcher struct {
    step       int    // current step (0-4)
    ticks      int    // dots spent on current step (0-1, steps take 2 dots)
    tileIndex  uint8  // tile number from tilemap
    tileDataLo uint8  // low byte of tile row
    tileDataHi uint8  // high byte of tile row
    mapX       uint8  // current tilemap X position (0-31)
    pixelY     uint8  // Y position within the tile row (0-7)
    fetchingWindow bool // true if currently fetching window tiles
}
```

### 2b. OAM Scan (Mode 2) — 80 dots

During Mode 2, the PPU scans all 40 OAM entries (2 per M-cycle = 80 dots)
and builds a buffer of up to 10 sprites that overlap the current scanline:

```go
type SpriteEntry struct {
    Y      uint8
    X      uint8
    Tile   uint8
    Flags  uint8
    OAMIdx uint8 // position in OAM (0-39), for priority
}

// In PPU struct:
scanlineSprites [10]SpriteEntry  // sprites found during OAM scan
spriteCount     int              // number found (0-10)
oamScanIndex    int              // current OAM entry being scanned (0-39)
```

Each tick during Mode 2: advance oamScanIndex, check if sprite overlaps LY,
add to buffer if so (up to 10). When modeClock reaches 80, transition to Mode 3.

### 2c. Pixel Transfer (Mode 3) — variable duration

Mode 3 drives the fetcher and FIFO. Each dot:

1. **Advance fetcher** (if not waiting on push):
   - Steps 0-3: increment fetcher tick. On tick 1, execute the step's action.
   - Step 4 (push): attempt to push 8 pixels to BG FIFO. Only succeeds if FIFO
     is empty. If success, reset fetcher to step 0. If fail, retry next dot.

2. **Check for sprite**: if a sprite's X position matches the current screen X + 8,
   pause the BG fetcher and fetch sprite tile data (6-11 dot penalty).

3. **Pop pixel from FIFO** (if FIFO has > 8 pixels or push just succeeded):
   - First `SCX & 7` pixels are discarded (fine X scroll)
   - After that, each popped pixel is written to the framebuffer
   - If a sprite FIFO pixel is available at this X, mix it in

4. **Increment screen X**. When X reaches 160, transition to Mode 0 (HBlank).

### 2d. Fetcher State Machine Detail

**Step 0 — Get Tile (2 dots):**
```
tileMapAddr = bgMapBase + ((mapX) & 31) + (((LY + SCY) & 0xFF) / 8) * 32
tileIndex = VRAM[tileMapAddr]
mapX++
```
Note: SCY is read HERE, per tile fetch. SCX upper bits determine initial mapX.
SCX low 3 bits only affect fine scroll pixel discard.

If fetching window:
```
tileMapAddr = winMapBase + (windowFetchX & 31) + (windowLine / 8) * 32
windowFetchX++
```

**Step 1 — Get Tile Data Low (2 dots):**
```
tileDataAddr = tileDataBase + tileIndex * 16 + (pixelY % 8) * 2
tileDataLo = VRAM[tileDataAddr]
```
(Uses signed/unsigned addressing based on LCDC.4)

**Step 2 — Get Tile Data High (2 dots):**
```
tileDataHi = VRAM[tileDataAddr + 1]
```

**Step 3 — Sleep (2 dots):**
No-op.

**Step 4 — Push (attempt every dot):**
If BG FIFO is empty, push 8 pixels constructed from tileDataLo/Hi.
Pixel color = `((hi >> (7-bit)) & 1) << 1 | ((lo >> (7-bit)) & 1)`

### 2e. Sprite Fetching

When the pixel output X matches a sprite's X position (adjusted for the +8 offset):

1. If BG FIFO has pixels and fetcher isn't at step 4: advance fetcher to complete
   current step (penalty varies 0-5 dots depending on progress)
2. Fetch sprite tile data (6 dots: get tile, get data low, get data high, each 2 dots)
3. Mix sprite pixels into sprite FIFO (overlapping existing sprite pixels by priority)
4. Resume BG fetcher

### 2f. Window Trigger

When all these are true:
- LCDC bit 5 is set (window enabled)
- WY condition triggered (LY == WY was true at some point this frame)
- Current pixel X + 7 >= WX

Then:
1. Clear BG FIFO
2. Reset fetcher to step 0
3. Switch fetcher to window mode (uses window tilemap, window line counter)
4. Window line counter increments at end of scanline (only if window was active)
5. +6 dot penalty (fetcher restart)

### 2g. Pixel Mixing (FIFO output)

Each dot that produces a visible pixel:

```go
bgPixel := bgFIFO.Pop()
finalColor := bgPixel.Color

// Apply BG palette
if LCDC bit 0 clear {
    finalColor = 0  // BG disabled: all pixels are color 0
}

// Check sprite FIFO
if spriteFIFO.Size() > 0 {
    sprPixel := spriteFIFO.Pop()
    if sprPixel.Color != 0 && LCDC bit 1 set {
        // Sprite is visible
        if !sprPixel.BgPrio || finalColor == 0 {
            // Sprite wins: use sprite color + sprite palette
            paletteReg = OBP0 or OBP1 based on sprPixel.Palette
            paletteColor = (paletteReg >> (sprPixel.Color * 2)) & 0x03
            // Write to framebuffer using paletteColor
            goto writePixel
        }
    }
}

// Use BG/Window color
paletteColor := (BGP >> (finalColor * 2)) & 0x03
writePixel:
    framebuffer[LY * 160 + screenX] = dmgColors[paletteColor]
```

### 2h. HBlank (Mode 0)

No rendering work. Duration = 456 - 80 - (Mode 3 duration).
Transitions to Mode 2 of next scanline (LY++), or Mode 1 if LY reaches 144.

### 2i. STAT Interrupt Handling

Unchanged from current implementation. The rising-edge detection on the STAT
line is already correct. The mode transitions happen naturally from the FIFO timing.

**STAT write bug (DMG):** When CPU writes to FF41, the STAT interrupt line is
briefly asserted for one M-cycle (as if $FF were written). This causes a spurious
STAT interrupt on DMG hardware. Implement as:

```go
func (p *PPU) Write(addr uint16, value uint8) {
    if addr == 0xFF41 {
        // DMG STAT write bug: briefly assert all conditions
        p.statWriteBug = true  // checked on next tick
    }
    ...
}
```

---

## Phase 3: Performance Optimization for WASM

### 3a. Zero-allocation hot path

The FIFO, fetcher, and OAM scan use fixed-size arrays (no slices, no maps).
All pixel structures are value types (no pointers). The FIFOPixel struct is
4 bytes, fitting in a single uint32 if needed for cache efficiency.

### 3b. Inline the hot loop

The `tick()` function is called 70,224 times per frame. It must be as lean as
possible. Use a switch on the mode (OAM/Transfer/HBlank/VBlank) as the outer
branch, with the fetcher state machine as a nested switch only during Mode 3.

### 3c. Avoid function call overhead

Go's WASM target doesn't inline as aggressively as native. Keep the FIFO
push/pop/size as direct field access where possible rather than method calls.
The fetcher steps can be a flat switch rather than a function-per-step dispatch.

### 3d. Benchmark

Target: RunFrame() completes in < 8ms on a mid-range phone (2020-era).
Current scanline-based PPU runs in ~2-3ms. The FIFO adds ~160x more work per
scanline (per-dot vs per-scanline), but each dot's work is trivial (a few
comparisons and array accesses). Expected overhead: 2-4x → ~6-10ms. Acceptable
for 16.7ms frame budget.

If needed, the fallback is to run 4 dots per tick instead of 1 (M-cycle granularity
rather than dot granularity). This loses sub-M-cycle accuracy but is still far
better than scanline-based rendering and passes all practical game compatibility.

---

## Phase 4: Verification

### 4a. Re-run Blargg tests
- cpu_instrs: should still pass 11/11 (CPU logic unchanged)
- instr_timing: should still pass (cycle counts unchanged, just distributed differently)
- mem_timing: should now pass (M-cycle stepping)
- halt_bug: should still pass

### 4b. Game testing
- Pokemon Red/Blue: vertical scrolling should be clean (no ripple)
- Zelda Link's Awakening: window-based HUD should be stable
- Tetris: basic rendering
- Dr. Mario: sprites and scrolling
- Kirby's Dream Land: parallax scrolling (uses mid-frame SCY changes)

### 4c. Performance testing
- Run on mobile browser (Chrome Android, Safari iOS)
- Verify 60fps sustained with no frame drops
- Check memory usage (should be unchanged — no new allocations)

---

## File Change Summary

| File | Change |
|------|--------|
| `memory/memory.go` | Add `Tick(cycles int)` method |
| `cpu/cpu.go` | Add `cycles` field, `read/write/idle` helpers, refactor `Step()` |
| `cpu/opcodes.go` | Replace all `Bus.Read/Write` + return with `read/write/idle` |
| `cpu/cb_opcodes.go` | Same pattern |
| `ppu/ppu.go` | Complete rewrite: FIFO, fetcher, OAM scan, pixel mixing |
| `internal/emulator.go` | Simplify `Step()` to just call `CPU.Step()` |

No changes to: APU, Timer, Joypad, Cartridge, WASM bridge, web frontend.

---

## Implementation Order

1. **Phase 1 first** (M-cycle stepping) — this is a mechanical refactor of the CPU.
   Each instruction gets `c.read()`/`c.write()`/`c.idle()` calls replacing
   `c.Bus.Read()`/`c.Bus.Write()` + cycle count returns. Test with existing
   scanline PPU to ensure nothing breaks.

2. **Phase 2** (FIFO) — replace the PPU rendering. The M-cycle architecture from
   Phase 1 means the PPU sees register changes at the right time.

3. **Phase 3** (optimization) — only if needed after Phase 2.

4. **Phase 4** (verification) — re-run all tests, game testing.
