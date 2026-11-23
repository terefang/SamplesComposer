package xoscillator

import (
    "fmt"
    "math"
)

// --- FM Waveform ---

// FMWaveform generates a frequency-modulated (phase-modulated) audio wave over a
// normalized oscillator phase in [0.0, 1.0), implementing Waveform.
//
// It evaluates a carrier phase modulated by a modulator wave (or an optional
// pluggable modulator Waveform) scaled by a modulation index. When ModWave is
// nil, it uses a default sine modulator at ModRatio times the carrier cycle.
type FMWaveform struct {
    // ModRatio sets the modulator frequency ratio relative to the carrier
    // frequency (defaults to 1.0 when <= 0).
    ModRatio float64

    // ModIndex controls the peak phase-modulation depth (beta), where higher
    // values add richer sideband harmonics (defaults to 1.0 when < 0).
    ModIndex float64

    // ModWave is an optional pluggable Waveform used as the modulator source.
    // When nil, a sine wave is evaluated at wrapped modulator phase.
    ModWave Waveform

    // CarrierWave is an optional pluggable Waveform used as the carrier wave
    // after phase modulation. When nil, a sine wave is evaluated.
    CarrierWave Waveform

    // RingMod inverts the output polarity whenever modMSB is negative.
    RingMod bool

    pulseWidth float64
    seed       int32
}

var _ Waveform = (*FMWaveform)(nil)

// NewFMWaveform constructs an FMWaveform with all configuration parameters:
// modulator frequency ratio, modulation index, optional modulator and carrier
// Waveforms, and ring-modulation mode.
func NewFMWaveform(
    modRatio float64,
    modIndex float64,
    modWave Waveform,
    carrierWave Waveform,
    ringMod bool,
) *FMWaveform {
    if modRatio <= 0.0 {
        modRatio = 1.0
    }
    if modIndex < 0.0 {
        modIndex = 1.0
    }
    return &FMWaveform{
        ModRatio:    modRatio,
        ModIndex:    modIndex,
        ModWave:     modWave,
        CarrierWave: carrierWave,
        RingMod:     ringMod,
        pulseWidth:  0.5,
        seed:        0x1337,
    }
}

// Name returns a human-readable label identifying the FM waveform and its
// active modulator and carrier subwaves (defaulting to "Sine" when nil),
// including "(Ring Mod)" when ring modulation is enabled.
func (w *FMWaveform) Name() string {
    modName := "Sine"
    if w.ModWave != nil && w.ModWave.Name() != "" {
        modName = w.ModWave.Name()
    }

    carrierName := "Sine"
    if w.CarrierWave != nil && w.CarrierWave.Name() != "" {
        carrierName = w.CarrierWave.Name()
    }

    base := fmt.Sprintf("FM (%s -> %s)", modName, carrierName)
    if w.RingMod {
        return base + " (Ring Mod)"
    }
    return base
}

// Sample evaluates the FM waveform at normalized oscillator phase in [0.0, 1.0),
// returning a bipolar sample clamped to [-1.0, 1.0].
func (w *FMWaveform) Sample(phase float64, crossedHalf bool, modMSB float64) float64 {
    p := phase - math.Floor(phase)
    if p < 0.0 {
        p += 1.0
    }

    ratio := w.ModRatio
    if ratio <= 0.0 {
        ratio = 1.0
    }

    index := w.ModIndex
    if index < 0.0 {
        index = 0.0
    }

    modPhase := p * ratio
    modPhaseWrapped := modPhase - math.Floor(modPhase)
    if modPhaseWrapped < 0.0 {
        modPhaseWrapped += 1.0
    }

    var modVal float64
    if w.ModWave != nil {
        w.ModWave.SetSeed(w.seed)
        //w.ModWave.SetPulseWidth(w.pulseWidth)
        modVal = w.ModWave.Sample(modPhaseWrapped, crossedHalf, modMSB)
    } else {
        modVal = math.Sin(2.0 * math.Pi * modPhaseWrapped)
    }

    // Phase modulation: carrier phase in cycles shifted by (index / 2pi) * modVal,
    // so multiplying by 2pi yields carrierAngle = 2pi * p + index * modVal.
    carrierAngle := 2.0*math.Pi*p + index*modVal

    var out float64
    if w.CarrierWave != nil {
        w.CarrierWave.SetSeed(w.seed)
        //w.CarrierWave.SetPulseWidth(w.pulseWidth)
        normCarrierPhase := carrierAngle / (2.0 * math.Pi)
        normCarrierPhase -= math.Floor(normCarrierPhase)
        if normCarrierPhase < 0.0 {
            normCarrierPhase += 1.0
        }
        out = w.CarrierWave.Sample(normCarrierPhase, crossedHalf, modMSB)
    } else {
        out = math.Sin(carrierAngle)
    }

    if w.RingMod && modMSB < 0.0 {
        out = -out
    }

    return math.Max(-1.0, math.Min(1.0, out))
}

// SetPulseWidth updates the pulse width forwarded to any nested Waveforms and
// stored on the FMWaveform.
func (w *FMWaveform) SetPulseWidth(width float64) {
    w.pulseWidth = width
    if w.ModWave != nil {
        w.ModWave.SetPulseWidth(width)
    }
    if w.CarrierWave != nil {
        w.CarrierWave.SetPulseWidth(width)
    }
}

// GetPulseWidth returns the current pulse width setting.
func (w *FMWaveform) GetPulseWidth() float64 {
    return w.pulseWidth
}

// SetSeed sets the 32-bit deterministic seed forwarded to any nested Waveforms
// and stored on the FMWaveform.
func (w *FMWaveform) SetSeed(s int32) {
    w.seed = s
    if w.ModWave != nil {
        w.ModWave.SetSeed(s)
    }
    if w.CarrierWave != nil {
        w.CarrierWave.SetSeed(s)
    }
}

// GetSeed returns the current 32-bit seed value.
func (w *FMWaveform) GetSeed() int32 {
    return w.seed
}
