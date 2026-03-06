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
    const FRAME_DURATION = 1000 / 59.7273; // ~16.74ms - exact refresh rate
    let lastFrameTime = 0;
    let frameTimeAccumulator = 0;

    // Fast-forward
    let fastForwardHeld = false;  // spacebar hold
    let fastForwardToggle = false; // button toggle
    let wasFastForward = false;   // track transitions
    const FF_SPEED = 4;

    function isFastForward() {
        return fastForwardHeld || fastForwardToggle;
    }

    // ── Audio ring buffer (typed Float32Array, zero GC) ──────────────
    const AUDIO_RING_SIZE = 65536;
    const AUDIO_RING_MASK = AUDIO_RING_SIZE - 1;
    let audioRing = null;
    let audioWritePos = 0;
    let audioReadPos = 0;

    function audioRingAvailable() {
        return (audioWritePos - audioReadPos) & AUDIO_RING_MASK;
    }

    function audioRingFree() {
        return (AUDIO_RING_SIZE - 1) - audioRingAvailable();
    }

    function audioRingClear() {
        audioWritePos = 0;
        audioReadPos = 0;
    }

    // ── Rebindable keyboard controls ─────────────────────────────────

    var DEFAULT_BINDINGS = {
        up:     'ArrowUp',
        down:   'ArrowDown',
        left:   'ArrowLeft',
        right:  'ArrowRight',
        a:      'KeyX',
        b:      'KeyZ',
        start:  'Enter',
        select: 'ShiftLeft',
        ff:     'Space'
    };

    var ACTION_TO_BTN = {
        up: 6, down: 7, left: 5, right: 4,
        a: 0, b: 1, start: 3, select: 2
        // ff is special — not an emulated button
    };

    var BINDING_DISPLAY = {
        up:     'D-Pad Up',
        down:   'D-Pad Down',
        left:   'D-Pad Left',
        right:  'D-Pad Right',
        a:      'A Button',
        b:      'B Button',
        start:  'Start',
        select: 'Select',
        ff:     'Fast Forward'
    };

    var BINDING_ORDER = ['up','down','left','right','a','b','start','select','ff'];

    var keyBindings = {};
    var keyMap = {};        // code → action reverse lookup
    var modalOpen = false;
    var rebinding = false;
    var rebindAction = null;

    function loadBindings() {
        try {
            var saved = localStorage.getItem('g2db-keybindings');
            if (saved) {
                keyBindings = JSON.parse(saved);
                // Ensure all actions have a binding
                for (var i = 0; i < BINDING_ORDER.length; i++) {
                    var k = BINDING_ORDER[i];
                    if (!(k in keyBindings)) keyBindings[k] = DEFAULT_BINDINGS[k];
                }
                return;
            }
        } catch(e) {}
        keyBindings = {};
        for (var key in DEFAULT_BINDINGS) keyBindings[key] = DEFAULT_BINDINGS[key];
    }

    function saveBindings() {
        try {
            localStorage.setItem('g2db-keybindings', JSON.stringify(keyBindings));
        } catch(e) {}
    }

    function buildKeyMap() {
        var map = {};
        for (var action in keyBindings) {
            map[keyBindings[action]] = action;
        }
        return map;
    }

    function keyDisplayName(code) {
        if (!code) return '\u2014';
        if (code.startsWith('Key')) return code.slice(3);
        if (code.startsWith('Digit')) return code.slice(5);
        var names = {
            'ArrowUp': '\u2191', 'ArrowDown': '\u2193',
            'ArrowLeft': '\u2190', 'ArrowRight': '\u2192',
            'Space': 'SPACE', 'Enter': 'ENTER',
            'ShiftLeft': 'L-SHIFT', 'ShiftRight': 'R-SHIFT',
            'ControlLeft': 'L-CTRL', 'ControlRight': 'R-CTRL',
            'AltLeft': 'L-ALT', 'AltRight': 'R-ALT',
            'Backspace': 'BKSP', 'Tab': 'TAB',
            'CapsLock': 'CAPS', 'Escape': 'ESC',
            'MetaLeft': 'L-META', 'MetaRight': 'R-META',
            'Semicolon': ';', 'Equal': '=', 'Comma': ',',
            'Minus': '-', 'Period': '.', 'Slash': '/',
            'Backquote': '`', 'BracketLeft': '[', 'BracketRight': ']',
            'Backslash': '\\', 'Quote': "'",
            'Delete': 'DEL', 'Insert': 'INS',
            'Home': 'HOME', 'End': 'END',
            'PageUp': 'PGUP', 'PageDown': 'PGDN',
            'NumpadEnter': 'NUM-ENTER',
            'NumpadAdd': 'NUM+', 'NumpadSubtract': 'NUM-',
            'NumpadMultiply': 'NUM*', 'NumpadDivide': 'NUM/',
            'NumpadDecimal': 'NUM.',
            'Numpad0': 'NUM0', 'Numpad1': 'NUM1', 'Numpad2': 'NUM2',
            'Numpad3': 'NUM3', 'Numpad4': 'NUM4', 'Numpad5': 'NUM5',
            'Numpad6': 'NUM6', 'Numpad7': 'NUM7', 'Numpad8': 'NUM8',
            'Numpad9': 'NUM9',
            'F1':'F1','F2':'F2','F3':'F3','F4':'F4','F5':'F5','F6':'F6',
            'F7':'F7','F8':'F8','F9':'F9','F10':'F10','F11':'F11','F12':'F12'
        };
        return names[code] || code;
    }

    // ── Initialize WASM ──────────────────────────────────────────────

    async function init() {
        var go = new Go();
        var result;
        try {
            result = await WebAssembly.instantiateStreaming(
                fetch('main.wasm'),
                go.importObject
            );
        } catch (e) {
            var resp = await fetch('main.wasm');
            var bytes = await resp.arrayBuffer();
            result = await WebAssembly.instantiate(bytes, go.importObject);
        }

        go.run(result.instance);
        await new Promise(function(r) { setTimeout(r, 100); });

        // Setup canvas with double-buffering
        canvas = document.getElementById('screen');
        ctx = canvas.getContext('2d');

        offCanvas = document.createElement('canvas');
        offCanvas.width = 160;
        offCanvas.height = 144;
        offCtx = offCanvas.getContext('2d');
        imageData = offCtx.createImageData(160, 144);

        frameBuffer = new Uint8Array(160 * 144 * 4);

        // Fill with default green
        for (var i = 0; i < frameBuffer.length; i += 4) {
            frameBuffer[i]   = 0x9B;
            frameBuffer[i+1] = 0xBC;
            frameBuffer[i+2] = 0x0F;
            frameBuffer[i+3] = 0xFF;
        }
        renderFrame();

        setupControls();
        setupFileInput();
        setupKeyboard();
        setupModal();
        setupColorsModal();

        // iOS: create AudioContext on first user gesture so it isn't born suspended
        var unlockAudio = function() {
            ensureAudioCtx();
            if (audioCtx && audioCtx.state === 'suspended') {
                audioCtx.resume();
            }
            document.removeEventListener('touchstart', unlockAudio, true);
            document.removeEventListener('touchend', unlockAudio, true);
            document.removeEventListener('click', unlockAudio, true);
        };
        document.addEventListener('touchstart', unlockAudio, true);
        document.addEventListener('touchend', unlockAudio, true);
        document.addEventListener('click', unlockAudio, true);

        // Load saved palette and apply
        loadPalette();
        applyPalette(currentPalette);

        document.getElementById('loading-screen').classList.add('hidden');
        document.getElementById('console').classList.remove('hidden');
    }

    // ── File input ───────────────────────────────────────────────────

    function setupFileInput() {
        var input = document.getElementById('rom-input');
        input.addEventListener('change', function(e) {
            var file = e.target.files[0];
            if (!file) return;

            var reader = new FileReader();
            reader.onload = function(ev) {
                var data = new Uint8Array(ev.target.result);
                var err = gbLoadROM(data);
                if (err && err !== '') {
                    alert('Failed to load ROM: ' + err);
                    return;
                }

                romLoaded = true;
                var title = gbGetTitle();
                document.getElementById('game-title').textContent = title || file.name;

                // Light up power LED
                var led = document.getElementById('power-led');
                if (led) led.classList.add('on');

                initAudio();
                startEmulation();
            };
            reader.readAsArrayBuffer(file);
            input.value = '';
        });
    }

    // ── Emulation loop ───────────────────────────────────────────────

    function startEmulation() {
        if (running) return;
        running = true;
        lastFrameTime = performance.now();
        frameTimeAccumulator = 0;
        emulationLoop();
    }

    function emulationLoop() {
        if (!running || !romLoaded) return;

        var now = performance.now();
        var elapsed = now - lastFrameTime;
        lastFrameTime = now;

        frameTimeAccumulator += Math.min(elapsed, FRAME_DURATION * 8);

        var ff = isFastForward();
        var framesRun = 0;

        if (wasFastForward && !ff) {
            // Leaving FF: reset timing so we don't lurch.
            // Don't clear the ring buffer — it was never filled during FF,
            // so the consumer drains naturally with no pop/click.
            frameTimeAccumulator = 0;
            lastFrameTime = now;
        }
        wasFastForward = ff;

        if (ff) {
            frameTimeAccumulator += (FF_SPEED - 1) * Math.min(elapsed, FRAME_DURATION * 2);
        }

        while (frameTimeAccumulator >= FRAME_DURATION) {
            gbRunFrame();
            framesRun++;
            frameTimeAccumulator -= FRAME_DURATION;

            // During FF, only queue every FF_SPEED-th frame to keep the
            // ring buffer from overflowing while still producing audio.
            if (soundEnabled && audioCtx && audioCtx.state === 'running') {
                var samples = gbGetAudio();
                if (samples && samples.length > 0 && (!ff || framesRun % FF_SPEED === 1)) {
                    queueAudio(samples);
                }
            } else {
                gbGetAudio();
            }

            if (framesRun >= FF_SPEED) {
                if (ff) frameTimeAccumulator = 0;
                break;
            }
        }

        if (framesRun > 0) {
            gbGetFrame(frameBuffer);
            renderFrame();
        }

        updateFastForwardIndicator(ff);
        animFrameId = requestAnimationFrame(emulationLoop);
    }

    function renderFrame() {
        imageData.data.set(frameBuffer);
        offCtx.putImageData(imageData, 0, 0);
        ctx.drawImage(offCanvas, 0, 0);
    }

    function updateFastForwardIndicator(active) {
        var indicator = document.getElementById('ff-indicator');
        if (indicator) indicator.classList.toggle('active', active);
    }

    // ── Audio engine ─────────────────────────────────────────────────

    // iOS Safari requires AudioContext creation inside a direct user gesture.
    // We create it on the very first touch/click, then connect the processor
    // when a ROM is loaded.
    function ensureAudioCtx() {
        if (audioCtx) return;
        try {
            audioCtx = new (window.AudioContext || window.webkitAudioContext)({
                sampleRate: 44100
            });
            audioRing = new Float32Array(AUDIO_RING_SIZE);
            audioRingClear();
        } catch (e) {
            console.warn('AudioContext creation failed:', e);
        }
    }

    function initAudio() {
        ensureAudioCtx();
        if (!audioCtx || audioScriptNode) return;
        try {
            var bufferSize = 2048;
            audioScriptNode = audioCtx.createScriptProcessor(bufferSize, 0, 2);
            audioScriptNode.onaudioprocess = function(e) {
                var left = e.outputBuffer.getChannelData(0);
                var right = e.outputBuffer.getChannelData(1);
                var avail = audioRingAvailable();
                var samplePairs = Math.min(bufferSize, avail >>> 1);
                var i = 0;
                var lastL = 0, lastR = 0;
                for (; i < samplePairs; i++) {
                    lastL = left[i] = audioRing[audioReadPos];
                    audioReadPos = (audioReadPos + 1) & AUDIO_RING_MASK;
                    lastR = right[i] = audioRing[audioReadPos];
                    audioReadPos = (audioReadPos + 1) & AUDIO_RING_MASK;
                }
                // Fade out over 32 samples on underrun to prevent clicks
                if (i < bufferSize && i > 0) {
                    var fadeLen = Math.min(32, bufferSize - i);
                    for (var f = 0; f < fadeLen; f++) {
                        var t = 1 - (f + 1) / fadeLen;
                        left[i] = lastL * t;
                        right[i] = lastR * t;
                        i++;
                    }
                }
                for (; i < bufferSize; i++) {
                    left[i] = 0;
                    right[i] = 0;
                }
            };
            audioScriptNode.connect(audioCtx.destination);
            // Force resume in case iOS suspended it
            if (audioCtx.state === 'suspended') {
                audioCtx.resume();
            }
        } catch (e) {
            console.warn('Audio init failed:', e);
        }
    }

    function queueAudio(samples) {
        if (!audioRing) return;
        var len = samples.length;
        var free = audioRingFree();
        if (len > free) {
            // Buffer full — drop this batch entirely rather than advancing
            // the read cursor (which skips audio mid-stream → audible click).
            // At normal speed this shouldn't happen; it's a safety valve.
            return;
        }
        for (var i = 0; i < len; i++) {
            audioRing[audioWritePos] = samples[i];
            audioWritePos = (audioWritePos + 1) & AUDIO_RING_MASK;
        }
    }

    function resumeAudio() {
        ensureAudioCtx();
        if (audioCtx && audioCtx.state === 'suspended') {
            audioCtx.resume();
        }
    }

    // ── Touch / mouse controls ───────────────────────────────────────

    function setupControls() {
        var buttons = document.querySelectorAll('[data-btn]');
        buttons.forEach(function(btn) {
            var code = parseInt(btn.dataset.btn);

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

        // Reset
        document.getElementById('btn-reset').addEventListener('click', function() {
            if (romLoaded) gbReset();
        });

        // Sound toggle
        document.getElementById('btn-sound').addEventListener('click', function() {
            soundEnabled = !soundEnabled;
            this.textContent = 'SND: ' + (soundEnabled ? 'ON' : 'OFF');
            if (!soundEnabled && audioCtx) {
                audioCtx.suspend();
            } else if (soundEnabled && audioCtx) {
                audioCtx.resume();
            }
        });

        // Fast-forward toggle
        document.getElementById('btn-ff').addEventListener('click', function() {
            fastForwardToggle = !fastForwardToggle;
            this.textContent = fastForwardToggle ? 'FF: ON' : 'FF';
        });

        // FF touch hold
        var ffBtn = document.getElementById('btn-ff');
        ffBtn.addEventListener('touchstart', function(e) {
            if (fastForwardToggle) return;
            e.preventDefault();
            fastForwardHeld = true;
        }, { passive: false });
        ffBtn.addEventListener('touchend', function() {
            if (fastForwardToggle) return;
            fastForwardHeld = false;
        });
        ffBtn.addEventListener('touchcancel', function() {
            fastForwardHeld = false;
        });

        // Controls modal button
        document.getElementById('btn-controls').addEventListener('click', function() {
            openControlsModal();
        });

        // Prevent context menu on long press
        document.addEventListener('contextmenu', function(e) {
            e.preventDefault();
        });
    }

    // ── Keyboard ─────────────────────────────────────────────────────

    function setupKeyboard() {
        loadBindings();
        keyMap = buildKeyMap();

        document.addEventListener('keydown', function(e) {
            if (modalOpen) return;

            var action = keyMap[e.code];
            if (!action) return;
            e.preventDefault();

            if (action === 'ff') {
                fastForwardHeld = true;
                return;
            }

            var btn = ACTION_TO_BTN[action];
            if (btn !== undefined && romLoaded) {
                resumeAudio();
                gbKeyDown(btn);
                // Visual feedback on on-screen button
                var el = document.querySelector('[data-btn="' + btn + '"]');
                if (el) el.classList.add('pressed');
            }
        });

        document.addEventListener('keyup', function(e) {
            if (modalOpen) return;

            var action = keyMap[e.code];
            if (!action) return;
            e.preventDefault();

            if (action === 'ff') {
                fastForwardHeld = false;
                return;
            }

            var btn = ACTION_TO_BTN[action];
            if (btn !== undefined && romLoaded) {
                gbKeyUp(btn);
                var el = document.querySelector('[data-btn="' + btn + '"]');
                if (el) el.classList.remove('pressed');
            }
        });
    }

    // ── Controls modal ───────────────────────────────────────────────

    function setupModal() {
        document.getElementById('modal-overlay').addEventListener('click', closeControlsModal);
        document.getElementById('btn-close-modal').addEventListener('click', closeControlsModal);
        document.getElementById('btn-reset-bindings').addEventListener('click', function() {
            keyBindings = {};
            for (var key in DEFAULT_BINDINGS) keyBindings[key] = DEFAULT_BINDINGS[key];
            saveBindings();
            keyMap = buildKeyMap();
            renderBindings();
        });
    }

    function openControlsModal() {
        modalOpen = true;
        document.getElementById('controls-modal').classList.remove('hidden');
        renderBindings();
    }

    function closeControlsModal() {
        modalOpen = false;
        rebinding = false;
        rebindAction = null;
        document.getElementById('controls-modal').classList.add('hidden');
    }

    function renderBindings() {
        var list = document.getElementById('bindings-list');
        list.innerHTML = '';

        for (var i = 0; i < BINDING_ORDER.length; i++) {
            var action = BINDING_ORDER[i];
            var row = document.createElement('div');
            row.className = 'binding-row';

            var label = document.createElement('span');
            label.className = 'binding-label';
            label.textContent = BINDING_DISPLAY[action];

            var keyBtn = document.createElement('button');
            keyBtn.className = 'binding-key';
            keyBtn.textContent = keyDisplayName(keyBindings[action]);
            keyBtn.setAttribute('data-action', action);
            keyBtn.addEventListener('click', (function(act, btn) {
                return function() { startRebind(act, btn); };
            })(action, keyBtn));

            row.appendChild(label);
            row.appendChild(keyBtn);
            list.appendChild(row);
        }
    }

    function startRebind(action, btn) {
        // Cancel any previous rebind
        var prev = document.querySelector('.binding-key.listening');
        if (prev) {
            prev.classList.remove('listening');
            prev.textContent = keyDisplayName(keyBindings[prev.getAttribute('data-action')]);
        }

        rebinding = true;
        rebindAction = action;
        btn.classList.add('listening');
        btn.textContent = 'Press a key\u2026';

        function onKey(e) {
            e.preventDefault();
            e.stopPropagation();

            // Escape cancels
            if (e.code === 'Escape') {
                btn.classList.remove('listening');
                btn.textContent = keyDisplayName(keyBindings[action]);
                rebinding = false;
                rebindAction = null;
                document.removeEventListener('keydown', onKey, true);
                return;
            }

            // If this key is already bound to another action, swap
            for (var other in keyBindings) {
                if (other !== action && keyBindings[other] === e.code) {
                    keyBindings[other] = keyBindings[action];
                }
            }

            keyBindings[action] = e.code;
            saveBindings();
            keyMap = buildKeyMap();

            btn.classList.remove('listening');
            rebinding = false;
            rebindAction = null;
            document.removeEventListener('keydown', onKey, true);

            renderBindings();
        }

        document.addEventListener('keydown', onKey, true);
    }

    // ── Color palette ────────────────────────────────────────────────

    var PALETTE_PRESETS = {
        'Classic':    ['#9BBC0F','#8BAC0F','#306230','#0F380F'],
        'Grayscale':  ['#FFFFFF','#AAAAAA','#555555','#000000'],
        'Pocket':     ['#C4CFA1','#8B956D','#4D533C','#1F1F1F'],
        'Light':      ['#E0F8D0','#88C070','#346856','#081820'],
        'Nostalgia':  ['#F8E8C0','#D0A870','#785838','#201808'],
        'Crimson':    ['#FFD0D0','#E06060','#802020','#300000'],
        'Ocean':      ['#C0E8F8','#5090C0','#204868','#081828'],
        'Lavender':   ['#E8D0F8','#A070D0','#583880','#180830']
    };
    var PRESET_NAMES = Object.keys(PALETTE_PRESETS);

    var DEFAULT_PALETTE = PALETTE_PRESETS['Classic'];
    var currentPalette = DEFAULT_PALETTE.slice();

    function hexToRgb(hex) {
        var c = parseInt(hex.slice(1), 16);
        return [(c >> 16) & 0xFF, (c >> 8) & 0xFF, c & 0xFF];
    }

    function rgbToHex(r, g, b) {
        return '#' + ((1 << 24) | (r << 16) | (g << 8) | b).toString(16).slice(1).toUpperCase();
    }

    function applyPalette(colors) {
        currentPalette = colors.slice();
        var args = [];
        for (var i = 0; i < 4; i++) {
            var rgb = hexToRgb(colors[i]);
            args.push(rgb[0], rgb[1], rgb[2]);
        }
        if (typeof gbSetPalette !== 'undefined') {
            gbSetPalette.apply(null, args);
        }
        // Update screen background and well to match lightest/darkest
        var screen = document.getElementById('screen');
        if (screen) screen.style.background = colors[0];
        var well = document.getElementById('screen-well');
        if (well) well.style.background = colors[3];
        // Update the initial framebuffer fill color if no ROM loaded
        if (!romLoaded && frameBuffer) {
            var rgb0 = hexToRgb(colors[0]);
            for (var j = 0; j < frameBuffer.length; j += 4) {
                frameBuffer[j]   = rgb0[0];
                frameBuffer[j+1] = rgb0[1];
                frameBuffer[j+2] = rgb0[2];
                frameBuffer[j+3] = 0xFF;
            }
            renderFrame();
        }
        savePalette();
        updateSwatches();
    }

    function savePalette() {
        try {
            localStorage.setItem('g2db-palette', JSON.stringify(currentPalette));
        } catch(e) {}
    }

    function loadPalette() {
        try {
            var saved = localStorage.getItem('g2db-palette');
            if (saved) {
                var colors = JSON.parse(saved);
                if (colors && colors.length === 4) {
                    currentPalette = colors;
                    return;
                }
            }
        } catch(e) {}
        currentPalette = DEFAULT_PALETTE.slice();
    }

    function updateSwatches() {
        for (var i = 0; i < 4; i++) {
            var sw = document.getElementById('swatch-' + i);
            if (sw) sw.style.background = currentPalette[i];
        }
    }

    function updatePickerInputs() {
        for (var i = 0; i < 4; i++) {
            var ci = document.getElementById('color-' + i);
            var hi = document.getElementById('hex-' + i);
            if (ci) ci.value = currentPalette[i];
            if (hi) hi.value = currentPalette[i];
        }
        updateSwatches();
        highlightActivePreset();
    }

    function highlightActivePreset() {
        var btns = document.querySelectorAll('.preset-btn');
        btns.forEach(function(btn) { btn.classList.remove('active'); });

        for (var name in PALETTE_PRESETS) {
            var p = PALETTE_PRESETS[name];
            var match = true;
            for (var i = 0; i < 4; i++) {
                if (currentPalette[i].toUpperCase() !== p[i].toUpperCase()) {
                    match = false;
                    break;
                }
            }
            if (match) {
                var el = document.querySelector('.preset-btn[data-preset="' + name + '"]');
                if (el) el.classList.add('active');
                break;
            }
        }
    }

    function setupColorsModal() {
        // Build preset buttons
        var presetsDiv = document.getElementById('palette-presets');
        for (var pi = 0; pi < PRESET_NAMES.length; pi++) {
            var name = PRESET_NAMES[pi];
            var colors = PALETTE_PRESETS[name];
            var btn = document.createElement('button');
            btn.className = 'preset-btn';
            btn.setAttribute('data-preset', name);

            var dots = document.createElement('span');
            dots.className = 'preset-dots';
            for (var di = 0; di < 4; di++) {
                var dot = document.createElement('span');
                dot.className = 'preset-dot';
                dot.style.background = colors[di];
                dots.appendChild(dot);
            }
            btn.appendChild(dots);

            var label = document.createElement('span');
            label.textContent = name;
            btn.appendChild(label);

            btn.addEventListener('click', (function(n) {
                return function() {
                    applyPalette(PALETTE_PRESETS[n]);
                    updatePickerInputs();
                };
            })(name));

            presetsDiv.appendChild(btn);
        }

        // Wire up color pickers and hex inputs
        for (var i = 0; i < 4; i++) {
            (function(idx) {
                var ci = document.getElementById('color-' + idx);
                var hi = document.getElementById('hex-' + idx);

                ci.addEventListener('input', function() {
                    hi.value = ci.value.toUpperCase();
                    currentPalette[idx] = ci.value.toUpperCase();
                    applyPalette(currentPalette);
                    highlightActivePreset();
                });

                hi.addEventListener('change', function() {
                    var v = hi.value.trim();
                    if (/^#[0-9A-Fa-f]{6}$/.test(v)) {
                        ci.value = v;
                        currentPalette[idx] = v.toUpperCase();
                        applyPalette(currentPalette);
                        highlightActivePreset();
                    } else {
                        hi.value = currentPalette[idx];
                    }
                });
            })(i);
        }

        // Open/close
        document.getElementById('btn-colors').addEventListener('click', function() {
            modalOpen = true;
            updatePickerInputs();
            document.getElementById('colors-modal').classList.remove('hidden');
        });

        document.getElementById('colors-overlay').addEventListener('click', closeColorsModal);
        document.getElementById('btn-close-colors').addEventListener('click', closeColorsModal);

        document.getElementById('btn-reset-colors').addEventListener('click', function() {
            applyPalette(DEFAULT_PALETTE);
            updatePickerInputs();
        });

        // ── Scanline toggle ──────────────────────────────────
        var screenWell = document.getElementById('screen-well');
        var scanBtns = document.querySelectorAll('.scan-opt');
        var savedScan = localStorage.getItem('g2db-scanline') || '';

        // Apply saved setting
        if (savedScan) {
            screenWell.classList.add(savedScan);
        }
        scanBtns.forEach(function(btn) {
            if (btn.dataset.scan === savedScan) {
                btn.classList.add('active');
            } else {
                btn.classList.remove('active');
            }
        });

        scanBtns.forEach(function(btn) {
            btn.addEventListener('click', function() {
                // Remove all scan classes
                screenWell.classList.remove('scan-dmg', 'scan-clean', 'scan-heavy', 'scan-lcd');
                // Remove active from all buttons
                scanBtns.forEach(function(b) { b.classList.remove('active'); });
                // Apply selected
                var mode = btn.dataset.scan;
                if (mode) {
                    screenWell.classList.add(mode);
                }
                btn.classList.add('active');
                localStorage.setItem('g2db-scanline', mode);
            });
        });
    }

    function closeColorsModal() {
        modalOpen = false;
        document.getElementById('colors-modal').classList.add('hidden');
    }

    // ── Boot ─────────────────────────────────────────────────────────

    init().catch(function(err) {
        console.error('Failed to initialize:', err);
        document.getElementById('loading-screen').innerHTML =
            '<p style="color: #ff6666">Failed to load emulator.<br>Make sure main.wasm is built.</p>';
    });
})();
