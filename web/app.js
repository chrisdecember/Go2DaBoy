(function() {
    'use strict';

    let romLoaded = false;
    let running = false;
    let soundEnabled = true;
    let audioCtx = null;
    let audioScriptNode = null;
    let frameBuffer = null;
    let canvas, ctx, imageData;
    let animFrameId = null;

    // Double-buffering: render to offscreen, then blit to visible canvas
    let offCanvas = null;
    let offCtx = null;

    // Frame timing
    const FRAME_DURATION = 1000 / 59.7273; // ~16.74ms - Game Boy's exact refresh rate
    let lastFrameTime = 0;
    let frameTimeAccumulator = 0;

    // Fast-forward
    let fastForwardHeld = false;  // spacebar hold
    let fastForwardToggle = false; // button toggle
    let wasFastForward = false;   // track transitions
    const FF_SPEED = 4; // run up to 4 frames per RAF tick when fast-forwarding

    function isFastForward() {
        return fastForwardHeld || fastForwardToggle;
    }

    // ── Audio ring buffer (typed Float32Array, zero GC) ──────────────
    const AUDIO_RING_SIZE = 65536; // power of 2, ~1.5s at 44100
    const AUDIO_RING_MASK = AUDIO_RING_SIZE - 1;
    let audioRing = null;   // Float32Array
    let audioWritePos = 0;  // producer cursor
    let audioReadPos = 0;   // consumer cursor

    function audioRingAvailable() {
        return (audioWritePos - audioReadPos) & AUDIO_RING_MASK;
    }

    function audioRingFree() {
        // Leave 1 slot empty to distinguish full vs empty
        return (AUDIO_RING_SIZE - 1) - audioRingAvailable();
    }

    function audioRingClear() {
        audioWritePos = 0;
        audioReadPos = 0;
    }

    // Initialize WASM
    async function init() {
        const go = new Go();
        let result;
        try {
            result = await WebAssembly.instantiateStreaming(
                fetch('main.wasm'),
                go.importObject
            );
        } catch (e) {
            // Fallback for servers that don't set correct MIME type
            const resp = await fetch('main.wasm');
            const bytes = await resp.arrayBuffer();
            result = await WebAssembly.instantiate(bytes, go.importObject);
        }

        go.run(result.instance);

        // Wait for Go to initialize
        await new Promise(r => setTimeout(r, 100));

        // Setup canvas with double-buffering
        canvas = document.getElementById('screen');
        ctx = canvas.getContext('2d');

        offCanvas = document.createElement('canvas');
        offCanvas.width = 160;
        offCanvas.height = 144;
        offCtx = offCanvas.getContext('2d');
        imageData = offCtx.createImageData(160, 144);

        frameBuffer = new Uint8Array(160 * 144 * 4);

        // Fill with default GB green
        for (let i = 0; i < frameBuffer.length; i += 4) {
            frameBuffer[i] = 0x9B;
            frameBuffer[i+1] = 0xBC;
            frameBuffer[i+2] = 0x0F;
            frameBuffer[i+3] = 0xFF;
        }
        renderFrame();

        // Setup UI
        setupControls();
        setupFileInput();
        setupKeyboard();

        // Show emulator
        document.getElementById('loading-screen').classList.add('hidden');
        document.getElementById('emulator').classList.remove('hidden');
    }

    function setupFileInput() {
        const input = document.getElementById('rom-input');
        input.addEventListener('change', function(e) {
            const file = e.target.files[0];
            if (!file) return;

            const reader = new FileReader();
            reader.onload = function(ev) {
                const data = new Uint8Array(ev.target.result);
                const err = gbLoadROM(data);
                if (err && err !== '') {
                    alert('Failed to load ROM: ' + err);
                    return;
                }

                romLoaded = true;
                const title = gbGetTitle();
                document.getElementById('game-title').textContent = title || file.name;

                initAudio();
                startEmulation();
            };
            reader.readAsArrayBuffer(file);
            // Reset input so same file can be reloaded
            input.value = '';
        });
    }

    function startEmulation() {
        if (running) return;
        running = true;
        lastFrameTime = performance.now();
        frameTimeAccumulator = 0;
        emulationLoop();
    }

    function emulationLoop() {
        if (!running || !romLoaded) return;

        const now = performance.now();
        const elapsed = now - lastFrameTime;
        lastFrameTime = now;

        // Cap accumulated time to prevent spiral of death (e.g. after tab switch)
        frameTimeAccumulator += Math.min(elapsed, FRAME_DURATION * 8);

        const ff = isFastForward();
        let framesRun = 0;

        // Handle FF ↔ normal transitions
        if (wasFastForward && !ff) {
            // Leaving FF: flush audio ring + reset timing so we don't
            // lurch through banked-up accumulator time
            audioRingClear();
            frameTimeAccumulator = 0;
            lastFrameTime = now;
        }
        wasFastForward = ff;

        // During fast-forward, multiply the effective elapsed time so the
        // accumulator fills up enough for FF_SPEED frames per tick.
        if (ff) {
            frameTimeAccumulator += (FF_SPEED - 1) * Math.min(elapsed, FRAME_DURATION * 2);
        }

        while (frameTimeAccumulator >= FRAME_DURATION) {
            gbRunFrame();
            framesRun++;
            frameTimeAccumulator -= FRAME_DURATION;

            // Queue audio from every frame (including FF — gives sped-up music)
            if (soundEnabled && audioCtx && audioCtx.state === 'running') {
                const samples = gbGetAudio();
                if (samples && samples.length > 0) {
                    queueAudio(samples);
                }
            } else {
                gbGetAudio();
            }

            // Hard cap to avoid blocking the browser
            if (framesRun >= FF_SPEED) {
                if (ff) frameTimeAccumulator = 0; // don't bank leftover time
                break;
            }
        }

        // Only render the latest frame (skip rendering intermediate catch-up frames)
        if (framesRun > 0) {
            gbGetFrame(frameBuffer);
            renderFrame();
        }

        // Update FF indicator
        updateFastForwardIndicator(ff);

        animFrameId = requestAnimationFrame(emulationLoop);
    }

    function renderFrame() {
        // Write to offscreen canvas, then blit — the browser composites
        // drawImage atomically, preventing mid-scanline tearing.
        imageData.data.set(frameBuffer);
        offCtx.putImageData(imageData, 0, 0);
        ctx.drawImage(offCanvas, 0, 0);
    }

    // Fast-forward indicator
    function updateFastForwardIndicator(active) {
        const indicator = document.getElementById('ff-indicator');
        if (indicator) {
            indicator.classList.toggle('active', active);
        }
    }

    // ── Audio engine (ring-buffer backed) ────────────────────────────

    function initAudio() {
        if (audioCtx) return;

        try {
            audioCtx = new (window.AudioContext || window.webkitAudioContext)({
                sampleRate: 44100
            });

            // Allocate ring buffer
            audioRing = new Float32Array(AUDIO_RING_SIZE);
            audioRingClear();

            // ScriptProcessorNode — reads from ring buffer
            var bufferSize = 2048;
            audioScriptNode = audioCtx.createScriptProcessor(bufferSize, 0, 2);
            audioScriptNode.onaudioprocess = function(e) {
                var left = e.outputBuffer.getChannelData(0);
                var right = e.outputBuffer.getChannelData(1);
                var avail = audioRingAvailable();
                // We need 2 floats per stereo sample (L + R)
                var samplePairs = Math.min(bufferSize, avail >>> 1);

                var i = 0;
                for (; i < samplePairs; i++) {
                    left[i] = audioRing[audioReadPos];
                    audioReadPos = (audioReadPos + 1) & AUDIO_RING_MASK;
                    right[i] = audioRing[audioReadPos];
                    audioReadPos = (audioReadPos + 1) & AUDIO_RING_MASK;
                }
                // Fill remainder with silence
                for (; i < bufferSize; i++) {
                    left[i] = 0;
                    right[i] = 0;
                }
            };
            audioScriptNode.connect(audioCtx.destination);
        } catch (e) {
            console.warn('Audio init failed:', e);
        }
    }

    function queueAudio(samples) {
        if (!audioRing) return;

        var len = samples.length;
        var free = audioRingFree();

        if (len > free) {
            // Buffer full — drop oldest samples by advancing read cursor.
            // Advance by just enough to make room, keeps audio mostly intact
            // instead of hard-chopping half the buffer.
            var drop = len - free;
            audioReadPos = (audioReadPos + drop) & AUDIO_RING_MASK;
        }

        for (var i = 0; i < len; i++) {
            audioRing[audioWritePos] = samples[i];
            audioWritePos = (audioWritePos + 1) & AUDIO_RING_MASK;
        }
    }

    // Resume audio on user interaction (required by browsers)
    function resumeAudio() {
        if (audioCtx && audioCtx.state === 'suspended') {
            audioCtx.resume();
        }
    }

    // Controls - touch and mouse
    function setupControls() {
        const buttons = document.querySelectorAll('[data-btn]');
        buttons.forEach(function(btn) {
            const code = parseInt(btn.dataset.btn);

            // Touch events
            btn.addEventListener('touchstart', function(e) {
                e.preventDefault();
                resumeAudio();
                btn.classList.add('pressed');
                if (romLoaded) gbKeyDown(code);
            }, { passive: false });

            btn.addEventListener('touchend', function(e) {
                e.preventDefault();
                btn.classList.remove('pressed');
                if (romLoaded) gbKeyUp(code);
            }, { passive: false });

            btn.addEventListener('touchcancel', function(e) {
                btn.classList.remove('pressed');
                if (romLoaded) gbKeyUp(code);
            });

            // Mouse events (for desktop testing)
            btn.addEventListener('mousedown', function(e) {
                e.preventDefault();
                resumeAudio();
                btn.classList.add('pressed');
                if (romLoaded) gbKeyDown(code);
            });

            btn.addEventListener('mouseup', function(e) {
                btn.classList.remove('pressed');
                if (romLoaded) gbKeyUp(code);
            });

            btn.addEventListener('mouseleave', function(e) {
                if (btn.classList.contains('pressed')) {
                    btn.classList.remove('pressed');
                    if (romLoaded) gbKeyUp(code);
                }
            });
        });

        // Reset button
        document.getElementById('btn-reset').addEventListener('click', function() {
            if (romLoaded) {
                gbReset();
            }
        });

        // Sound toggle
        document.getElementById('btn-sound').addEventListener('click', function() {
            soundEnabled = !soundEnabled;
            this.textContent = 'SOUND: ' + (soundEnabled ? 'ON' : 'OFF');
            if (!soundEnabled && audioCtx) {
                audioCtx.suspend();
            } else if (soundEnabled && audioCtx) {
                audioCtx.resume();
            }
        });

        // Fast-forward toggle button
        document.getElementById('btn-ff').addEventListener('click', function() {
            fastForwardToggle = !fastForwardToggle;
            this.textContent = fastForwardToggle ? 'FF: ON' : 'FF';
        });

        // Fast-forward touch support (hold to FF)
        var ffBtn = document.getElementById('btn-ff');
        ffBtn.addEventListener('touchstart', function(e) {
            // If toggle is already on, let the click handler manage it
            if (fastForwardToggle) return;
            e.preventDefault();
            fastForwardHeld = true;
        }, { passive: false });
        ffBtn.addEventListener('touchend', function(e) {
            if (fastForwardToggle) return;
            fastForwardHeld = false;
        });
        ffBtn.addEventListener('touchcancel', function() {
            fastForwardHeld = false;
        });

        // Prevent context menu on long press
        document.addEventListener('contextmenu', function(e) {
            e.preventDefault();
        });
    }

    // Keyboard mapping for desktop
    function setupKeyboard() {
        const keyMap = {
            'ArrowUp': 6,
            'ArrowDown': 7,
            'ArrowLeft': 5,
            'ArrowRight': 4,
            'z': 1, 'Z': 1,       // B
            'x': 0, 'X': 0,       // A
            'Enter': 3,            // Start
            'Shift': 2,            // Select
            'Backspace': 2,        // Select (alt)
            'a': 1, 'A': 1,       // B (alt)
            's': 0, 'S': 0,       // A (alt)
        };

        document.addEventListener('keydown', function(e) {
            // Spacebar = hold fast-forward
            if (e.code === 'Space') {
                e.preventDefault();
                fastForwardHeld = true;
                return;
            }

            const btn = keyMap[e.key];
            if (btn !== undefined && romLoaded) {
                e.preventDefault();
                resumeAudio();
                gbKeyDown(btn);
            }
        });

        document.addEventListener('keyup', function(e) {
            if (e.code === 'Space') {
                e.preventDefault();
                fastForwardHeld = false;
                return;
            }

            const btn = keyMap[e.key];
            if (btn !== undefined && romLoaded) {
                e.preventDefault();
                gbKeyUp(btn);
            }
        });
    }

    // Boot
    init().catch(function(err) {
        console.error('Failed to initialize:', err);
        document.getElementById('loading-screen').innerHTML =
            '<p style="color: #ff6666">Failed to load emulator.<br>Make sure main.wasm is built.</p>';
    });
})();
