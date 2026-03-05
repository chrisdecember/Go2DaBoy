package apu

const sampleRate = 44100
const cpuFreq = 4194304
const samplesPerFrame = sampleRate / 60

// APU implements the Game Boy audio processing unit with 4 channels.
type APU struct {
	enabled bool

	// Channel 1: Square wave with sweep
	ch1 squareChannel
	// Channel 2: Square wave
	ch2 squareChannel
	// Channel 3: Wave
	ch3 waveChannel
	// Channel 4: Noise
	ch4 noiseChannel

	// Master volume / VIN
	nr50 uint8
	// Sound panning
	nr51 uint8
	// Master control
	nr52 uint8

	// Frame sequencer
	frameSeqCounter int
	frameSeqStep    uint8

	// Sample generation
	sampleCounter float64
	sampleBuffer  []float32
	bufferPos     int
}

type squareChannel struct {
	enabled     bool
	dacEnabled  bool
	length      int
	lengthEn    bool
	duty        uint8
	dutyPos     uint8
	freqTimer   int
	frequency   uint16
	volume      uint8
	volumeInit  uint8
	envDir      uint8
	envPeriod   uint8
	envTimer    uint8
	sweepPeriod uint8
	sweepDir    uint8
	sweepShift  uint8
	sweepTimer  uint8
	sweepEn     bool
	sweepFreq   uint16
}

type waveChannel struct {
	enabled    bool
	dacEnabled bool
	length     int
	lengthEn   bool
	volume     uint8
	freqTimer  int
	frequency  uint16
	pos        uint8
	waveRAM    [16]uint8
}

type noiseChannel struct {
	enabled    bool
	dacEnabled bool
	length     int
	lengthEn   bool
	volume     uint8
	volumeInit uint8
	envDir     uint8
	envPeriod  uint8
	envTimer   uint8
	freqTimer  int
	divisor    uint8
	widthMode  uint8
	clockShift uint8
	lfsr       uint16
}

var dutyTable = [4][8]uint8{
	{0, 0, 0, 0, 0, 0, 0, 1}, // 12.5%
	{1, 0, 0, 0, 0, 0, 0, 1}, // 25%
	{1, 0, 0, 0, 0, 1, 1, 1}, // 50%
	{0, 1, 1, 1, 1, 1, 1, 0}, // 75%
}

var noiseDivisors = [8]int{8, 16, 32, 48, 64, 80, 96, 112}

func New() *APU {
	a := &APU{
		sampleBuffer: make([]float32, samplesPerFrame*2), // stereo
	}
	a.ch4.lfsr = 0x7FFF
	return a
}

func (a *APU) Reset() {
	a.enabled = true
	a.nr50 = 0x77
	a.nr51 = 0xF3
	a.nr52 = 0xF1
	a.ch1 = squareChannel{}
	a.ch2 = squareChannel{}
	a.ch3 = waveChannel{}
	a.ch4 = noiseChannel{lfsr: 0x7FFF}
	a.frameSeqCounter = 0
	a.frameSeqStep = 0
	a.sampleCounter = 0
	a.bufferPos = 0
}

func (a *APU) Read(addr uint16) uint8 {
	switch addr {
	// Channel 1
	case 0xFF10:
		return a.ch1.sweepPeriod<<4 | a.ch1.sweepDir<<3 | a.ch1.sweepShift | 0x80
	case 0xFF11:
		return a.ch1.duty<<6 | 0x3F
	case 0xFF12:
		return a.ch1.volumeInit<<4 | a.ch1.envDir<<3 | a.ch1.envPeriod
	case 0xFF13:
		return 0xFF // Write-only
	case 0xFF14:
		v := uint8(0xBF)
		if a.ch1.lengthEn {
			v |= 0x40
		}
		return v
	// Channel 2
	case 0xFF16:
		return a.ch2.duty<<6 | 0x3F
	case 0xFF17:
		return a.ch2.volumeInit<<4 | a.ch2.envDir<<3 | a.ch2.envPeriod
	case 0xFF18:
		return 0xFF
	case 0xFF19:
		v := uint8(0xBF)
		if a.ch2.lengthEn {
			v |= 0x40
		}
		return v
	// Channel 3
	case 0xFF1A:
		if a.ch3.dacEnabled {
			return 0xFF
		}
		return 0x7F
	case 0xFF1B:
		return 0xFF
	case 0xFF1C:
		return a.ch3.volume<<5 | 0x9F
	case 0xFF1D:
		return 0xFF
	case 0xFF1E:
		v := uint8(0xBF)
		if a.ch3.lengthEn {
			v |= 0x40
		}
		return v
	// Channel 4
	case 0xFF20:
		return 0xFF
	case 0xFF21:
		return a.ch4.volumeInit<<4 | a.ch4.envDir<<3 | a.ch4.envPeriod
	case 0xFF22:
		return a.ch4.clockShift<<4 | a.ch4.widthMode<<3 | a.ch4.divisor
	case 0xFF23:
		v := uint8(0xBF)
		if a.ch4.lengthEn {
			v |= 0x40
		}
		return v
	// Master
	case 0xFF24:
		return a.nr50
	case 0xFF25:
		return a.nr51
	case 0xFF26:
		v := uint8(0x70)
		if a.enabled {
			v |= 0x80
		}
		if a.ch1.enabled {
			v |= 0x01
		}
		if a.ch2.enabled {
			v |= 0x02
		}
		if a.ch3.enabled {
			v |= 0x04
		}
		if a.ch4.enabled {
			v |= 0x08
		}
		return v
	}
	// Wave RAM (0xFF30-0xFF3F)
	if addr >= 0xFF30 && addr <= 0xFF3F {
		return a.ch3.waveRAM[addr-0xFF30]
	}
	return 0xFF
}

func (a *APU) Write(addr uint16, value uint8) {
	if !a.enabled && addr != 0xFF26 && !(addr >= 0xFF30 && addr <= 0xFF3F) {
		return
	}

	switch addr {
	// Channel 1
	case 0xFF10:
		a.ch1.sweepPeriod = (value >> 4) & 0x07
		a.ch1.sweepDir = (value >> 3) & 0x01
		a.ch1.sweepShift = value & 0x07
	case 0xFF11:
		a.ch1.duty = (value >> 6) & 0x03
		a.ch1.length = 64 - int(value&0x3F)
	case 0xFF12:
		a.ch1.volumeInit = (value >> 4) & 0x0F
		a.ch1.envDir = (value >> 3) & 0x01
		a.ch1.envPeriod = value & 0x07
		a.ch1.dacEnabled = value&0xF8 != 0
		if !a.ch1.dacEnabled {
			a.ch1.enabled = false
		}
	case 0xFF13:
		a.ch1.frequency = (a.ch1.frequency & 0x700) | uint16(value)
	case 0xFF14:
		a.ch1.frequency = (a.ch1.frequency & 0xFF) | (uint16(value&0x07) << 8)
		a.ch1.lengthEn = value&0x40 != 0
		if value&0x80 != 0 {
			a.triggerCh1()
		}
	// Channel 2
	case 0xFF16:
		a.ch2.duty = (value >> 6) & 0x03
		a.ch2.length = 64 - int(value&0x3F)
	case 0xFF17:
		a.ch2.volumeInit = (value >> 4) & 0x0F
		a.ch2.envDir = (value >> 3) & 0x01
		a.ch2.envPeriod = value & 0x07
		a.ch2.dacEnabled = value&0xF8 != 0
		if !a.ch2.dacEnabled {
			a.ch2.enabled = false
		}
	case 0xFF18:
		a.ch2.frequency = (a.ch2.frequency & 0x700) | uint16(value)
	case 0xFF19:
		a.ch2.frequency = (a.ch2.frequency & 0xFF) | (uint16(value&0x07) << 8)
		a.ch2.lengthEn = value&0x40 != 0
		if value&0x80 != 0 {
			a.triggerCh2()
		}
	// Channel 3
	case 0xFF1A:
		a.ch3.dacEnabled = value&0x80 != 0
		if !a.ch3.dacEnabled {
			a.ch3.enabled = false
		}
	case 0xFF1B:
		a.ch3.length = 256 - int(value)
	case 0xFF1C:
		a.ch3.volume = (value >> 5) & 0x03
	case 0xFF1D:
		a.ch3.frequency = (a.ch3.frequency & 0x700) | uint16(value)
	case 0xFF1E:
		a.ch3.frequency = (a.ch3.frequency & 0xFF) | (uint16(value&0x07) << 8)
		a.ch3.lengthEn = value&0x40 != 0
		if value&0x80 != 0 {
			a.triggerCh3()
		}
	// Channel 4
	case 0xFF20:
		a.ch4.length = 64 - int(value&0x3F)
	case 0xFF21:
		a.ch4.volumeInit = (value >> 4) & 0x0F
		a.ch4.envDir = (value >> 3) & 0x01
		a.ch4.envPeriod = value & 0x07
		a.ch4.dacEnabled = value&0xF8 != 0
		if !a.ch4.dacEnabled {
			a.ch4.enabled = false
		}
	case 0xFF22:
		a.ch4.clockShift = (value >> 4) & 0x0F
		a.ch4.widthMode = (value >> 3) & 0x01
		a.ch4.divisor = value & 0x07
	case 0xFF23:
		a.ch4.lengthEn = value&0x40 != 0
		if value&0x80 != 0 {
			a.triggerCh4()
		}
	// Master
	case 0xFF24:
		a.nr50 = value
	case 0xFF25:
		a.nr51 = value
	case 0xFF26:
		wasEnabled := a.enabled
		a.enabled = value&0x80 != 0
		if wasEnabled && !a.enabled {
			// Turning off APU clears all registers
			a.ch1 = squareChannel{}
			a.ch2 = squareChannel{}
			a.ch3.enabled = false
			a.ch3.dacEnabled = false
			a.ch4 = noiseChannel{lfsr: 0x7FFF}
			a.nr50 = 0
			a.nr51 = 0
		}
	}
	// Wave RAM
	if addr >= 0xFF30 && addr <= 0xFF3F {
		a.ch3.waveRAM[addr-0xFF30] = value
	}
}

func (a *APU) triggerCh1() {
	a.ch1.enabled = a.ch1.dacEnabled
	if a.ch1.length == 0 {
		a.ch1.length = 64
	}
	a.ch1.freqTimer = (2048 - int(a.ch1.frequency)) * 4
	a.ch1.volume = a.ch1.volumeInit
	a.ch1.envTimer = a.ch1.envPeriod
	a.ch1.sweepFreq = a.ch1.frequency
	a.ch1.sweepTimer = a.ch1.sweepPeriod
	if a.ch1.sweepTimer == 0 {
		a.ch1.sweepTimer = 8
	}
	a.ch1.sweepEn = a.ch1.sweepPeriod > 0 || a.ch1.sweepShift > 0
}

func (a *APU) triggerCh2() {
	a.ch2.enabled = a.ch2.dacEnabled
	if a.ch2.length == 0 {
		a.ch2.length = 64
	}
	a.ch2.freqTimer = (2048 - int(a.ch2.frequency)) * 4
	a.ch2.volume = a.ch2.volumeInit
	a.ch2.envTimer = a.ch2.envPeriod
}

func (a *APU) triggerCh3() {
	a.ch3.enabled = a.ch3.dacEnabled
	if a.ch3.length == 0 {
		a.ch3.length = 256
	}
	a.ch3.freqTimer = (2048 - int(a.ch3.frequency)) * 2
	a.ch3.pos = 0
}

func (a *APU) triggerCh4() {
	a.ch4.enabled = a.ch4.dacEnabled
	if a.ch4.length == 0 {
		a.ch4.length = 64
	}
	a.ch4.freqTimer = noiseDivisors[a.ch4.divisor] << a.ch4.clockShift
	a.ch4.volume = a.ch4.volumeInit
	a.ch4.envTimer = a.ch4.envPeriod
	a.ch4.lfsr = 0x7FFF
}

// Step advances the APU by the given number of T-cycles
func (a *APU) Step(cycles int) {
	if !a.enabled {
		return
	}

	for i := 0; i < cycles; i++ {
		// Frame sequencer (ticks at 512 Hz = every 8192 T-cycles)
		a.frameSeqCounter++
		if a.frameSeqCounter >= 8192 {
			a.frameSeqCounter = 0
			a.clockFrameSequencer()
		}

		// Clock channel frequency timers
		a.clockCh1()
		a.clockCh2()
		a.clockCh3()
		a.clockCh4()

		// Generate sample
		a.sampleCounter += float64(sampleRate)
		if a.sampleCounter >= float64(cpuFreq) {
			a.sampleCounter -= float64(cpuFreq)
			a.generateSample()
		}
	}
}

func (a *APU) clockFrameSequencer() {
	switch a.frameSeqStep {
	case 0:
		a.clockLength()
	case 2:
		a.clockLength()
		a.clockSweep()
	case 4:
		a.clockLength()
	case 6:
		a.clockLength()
		a.clockSweep()
	case 7:
		a.clockEnvelope()
	}
	a.frameSeqStep = (a.frameSeqStep + 1) & 7
}

func (a *APU) clockLength() {
	if a.ch1.lengthEn && a.ch1.length > 0 {
		a.ch1.length--
		if a.ch1.length == 0 {
			a.ch1.enabled = false
		}
	}
	if a.ch2.lengthEn && a.ch2.length > 0 {
		a.ch2.length--
		if a.ch2.length == 0 {
			a.ch2.enabled = false
		}
	}
	if a.ch3.lengthEn && a.ch3.length > 0 {
		a.ch3.length--
		if a.ch3.length == 0 {
			a.ch3.enabled = false
		}
	}
	if a.ch4.lengthEn && a.ch4.length > 0 {
		a.ch4.length--
		if a.ch4.length == 0 {
			a.ch4.enabled = false
		}
	}
}

func (a *APU) clockSweep() {
	if !a.ch1.sweepEn || a.ch1.sweepPeriod == 0 {
		return
	}
	a.ch1.sweepTimer--
	if a.ch1.sweepTimer == 0 {
		a.ch1.sweepTimer = a.ch1.sweepPeriod
		if a.ch1.sweepTimer == 0 {
			a.ch1.sweepTimer = 8
		}
		newFreq := a.calcSweepFreq()
		if newFreq <= 2047 && a.ch1.sweepShift > 0 {
			a.ch1.sweepFreq = uint16(newFreq)
			a.ch1.frequency = uint16(newFreq)
			// Check again for overflow
			if a.calcSweepFreq() > 2047 {
				a.ch1.enabled = false
			}
		}
		if newFreq > 2047 {
			a.ch1.enabled = false
		}
	}
}

func (a *APU) calcSweepFreq() int {
	freq := int(a.ch1.sweepFreq) >> a.ch1.sweepShift
	if a.ch1.sweepDir != 0 {
		freq = int(a.ch1.sweepFreq) - freq
	} else {
		freq = int(a.ch1.sweepFreq) + freq
	}
	return freq
}

func (a *APU) clockEnvelope() {
	// Channel 1
	if a.ch1.envPeriod > 0 {
		a.ch1.envTimer--
		if a.ch1.envTimer == 0 {
			a.ch1.envTimer = a.ch1.envPeriod
			if a.ch1.envDir != 0 && a.ch1.volume < 15 {
				a.ch1.volume++
			} else if a.ch1.envDir == 0 && a.ch1.volume > 0 {
				a.ch1.volume--
			}
		}
	}
	// Channel 2
	if a.ch2.envPeriod > 0 {
		a.ch2.envTimer--
		if a.ch2.envTimer == 0 {
			a.ch2.envTimer = a.ch2.envPeriod
			if a.ch2.envDir != 0 && a.ch2.volume < 15 {
				a.ch2.volume++
			} else if a.ch2.envDir == 0 && a.ch2.volume > 0 {
				a.ch2.volume--
			}
		}
	}
	// Channel 4
	if a.ch4.envPeriod > 0 {
		a.ch4.envTimer--
		if a.ch4.envTimer == 0 {
			a.ch4.envTimer = a.ch4.envPeriod
			if a.ch4.envDir != 0 && a.ch4.volume < 15 {
				a.ch4.volume++
			} else if a.ch4.envDir == 0 && a.ch4.volume > 0 {
				a.ch4.volume--
			}
		}
	}
}

func (a *APU) clockCh1() {
	a.ch1.freqTimer--
	if a.ch1.freqTimer <= 0 {
		a.ch1.freqTimer = (2048 - int(a.ch1.frequency)) * 4
		a.ch1.dutyPos = (a.ch1.dutyPos + 1) & 7
	}
}

func (a *APU) clockCh2() {
	a.ch2.freqTimer--
	if a.ch2.freqTimer <= 0 {
		a.ch2.freqTimer = (2048 - int(a.ch2.frequency)) * 4
		a.ch2.dutyPos = (a.ch2.dutyPos + 1) & 7
	}
}

func (a *APU) clockCh3() {
	a.ch3.freqTimer--
	if a.ch3.freqTimer <= 0 {
		a.ch3.freqTimer = (2048 - int(a.ch3.frequency)) * 2
		a.ch3.pos = (a.ch3.pos + 1) & 31
	}
}

func (a *APU) clockCh4() {
	a.ch4.freqTimer--
	if a.ch4.freqTimer <= 0 {
		a.ch4.freqTimer = noiseDivisors[a.ch4.divisor] << a.ch4.clockShift
		xorBit := (a.ch4.lfsr & 0x01) ^ ((a.ch4.lfsr >> 1) & 0x01)
		a.ch4.lfsr = (a.ch4.lfsr >> 1) | (xorBit << 14)
		if a.ch4.widthMode != 0 {
			a.ch4.lfsr &^= (1 << 6)
			a.ch4.lfsr |= xorBit << 6
		}
	}
}

func (a *APU) getCh1Sample() float32 {
	if !a.ch1.enabled || !a.ch1.dacEnabled {
		return 0
	}
	if dutyTable[a.ch1.duty][a.ch1.dutyPos] != 0 {
		return float32(a.ch1.volume) / 15.0
	}
	return 0
}

func (a *APU) getCh2Sample() float32 {
	if !a.ch2.enabled || !a.ch2.dacEnabled {
		return 0
	}
	if dutyTable[a.ch2.duty][a.ch2.dutyPos] != 0 {
		return float32(a.ch2.volume) / 15.0
	}
	return 0
}

func (a *APU) getCh3Sample() float32 {
	if !a.ch3.enabled || !a.ch3.dacEnabled {
		return 0
	}
	sampleByte := a.ch3.waveRAM[a.ch3.pos/2]
	var sample uint8
	if a.ch3.pos%2 == 0 {
		sample = (sampleByte >> 4) & 0x0F
	} else {
		sample = sampleByte & 0x0F
	}
	switch a.ch3.volume {
	case 0:
		sample >>= 4 // Mute
	case 1:
		// 100%
	case 2:
		sample >>= 1 // 50%
	case 3:
		sample >>= 2 // 25%
	}
	return float32(sample) / 15.0
}

func (a *APU) getCh4Sample() float32 {
	if !a.ch4.enabled || !a.ch4.dacEnabled {
		return 0
	}
	if a.ch4.lfsr&0x01 == 0 {
		return float32(a.ch4.volume) / 15.0
	}
	return 0
}

func (a *APU) generateSample() {
	if a.bufferPos >= len(a.sampleBuffer) {
		return
	}

	ch1 := a.getCh1Sample()
	ch2 := a.getCh2Sample()
	ch3 := a.getCh3Sample()
	ch4 := a.getCh4Sample()

	// Left channel
	var left float32
	if a.nr51&0x10 != 0 {
		left += ch1
	}
	if a.nr51&0x20 != 0 {
		left += ch2
	}
	if a.nr51&0x40 != 0 {
		left += ch3
	}
	if a.nr51&0x80 != 0 {
		left += ch4
	}

	// Right channel
	var right float32
	if a.nr51&0x01 != 0 {
		right += ch1
	}
	if a.nr51&0x02 != 0 {
		right += ch2
	}
	if a.nr51&0x04 != 0 {
		right += ch3
	}
	if a.nr51&0x08 != 0 {
		right += ch4
	}

	// Master volume
	leftVol := float32((a.nr50>>4)&0x07+1) / 8.0
	rightVol := float32((a.nr50&0x07)+1) / 8.0

	left = left * leftVol / 4.0
	right = right * rightVol / 4.0

	a.sampleBuffer[a.bufferPos] = left
	a.bufferPos++
	if a.bufferPos < len(a.sampleBuffer) {
		a.sampleBuffer[a.bufferPos] = right
		a.bufferPos++
	}
}

// GetSamples returns the current audio sample buffer and resets it
func (a *APU) GetSamples() []float32 {
	samples := make([]float32, a.bufferPos)
	copy(samples, a.sampleBuffer[:a.bufferPos])
	a.bufferPos = 0
	return samples
}

// GetSampleRate returns the audio sample rate
func (a *APU) GetSampleRate() int {
	return sampleRate
}
