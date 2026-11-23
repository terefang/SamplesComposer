package xoscillator

import (
    "fmt"
    "math"
)

const (
    defaultFM2ModRatio = 2.0
    defaultFM2ModIndex = 1.8
)

// FM2Waveform implements Waveform as a 2-operator frequency modulation
// oscillator with an independent modulator phase accumulator.
type FM2Waveform struct {
    // ModRatio sets the modulator-to-carrier frequency ratio (defaults to 2.0
    // when <= 0 or non-finite).
    ModRatio float64

    // ModIndex sets the phase modulation depth (non-negative; negative or
    // non-finite values default to 0.0).
    ModIndex float64

    // ModWave is an optional pluggable Waveform used as the modulator source.
    // When nil, a sine wave is evaluated at wrapped modulator phase.
    ModWave Waveform

    PulseWidth float64
}

func (w *FM2Waveform) SetPulseWidth(pulseWidth float64) {
    w.PulseWidth = pulseWidth
}

func (w *FM2Waveform) GetPulseWidth() float64 {
    return w.PulseWidth
}

func (w *FM2Waveform) SetSeed(s int32) {
}

func (w *FM2Waveform) GetSeed() int32 {
    return 0
}

var _ Waveform = (*FM2Waveform)(nil)

// NewFM2Waveform constructs an FM2Waveform with normalized ModRatio and ModIndex.
func NewFM2Waveform(modRatio, modIndex float64, modWave Waveform, pulseWidth float64) *FM2Waveform {
    w := &FM2Waveform{
        ModRatio:   modRatio,
        ModIndex:   modIndex,
        ModWave:    modWave,
        PulseWidth: pulseWidth,
    }
    return w
}

func (w *FM2Waveform) SetModWave(_ModWave Waveform) {
    w.ModWave = _ModWave
}

// SetModRatio updates the modulator-to-carrier frequency ratio. Non-positive
// or non-finite values default to 2.0.
func (w *FM2Waveform) SetModRatio(ratio float64) {
    if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio <= 0.0 {
        w.ModRatio = defaultFM2ModRatio
        return
    }
    w.ModRatio = ratio
}

// SetModIndex updates the FM modulation index. Negative or non-finite values
// default to 0.0.
func (w *FM2Waveform) SetModIndex(index float64) {
    if math.IsNaN(index) || math.IsInf(index, 0) || index < 0.0 {
        w.ModIndex = 0.0
        return
    }
    w.ModIndex = index
}

// Name returns a human-readable identifier for the FM2 waveform.
func (w *FM2Waveform) Name() string {
    ratio := w.ModRatio
    if math.IsNaN(ratio) || math.IsInf(ratio, 0) || ratio <= 0.0 {
        ratio = defaultFM2ModRatio
    }
    index := w.ModIndex
    if math.IsNaN(index) || math.IsInf(index, 0) || index < 0.0 {
        index = 0.0
    }
    return fmt.Sprintf("FM2 (ratio=%.2f, index=%.2f)", ratio, index)
}

// Sample computes the next FM2 sample for normalized carrier phase in [0.0, 1.0).
func (w *FM2Waveform) Sample(phase float64, _ bool, modMSB float64) float64 {
    effectivePhase := phase - math.Floor(phase)
    if effectivePhase < 0.0 {
        effectivePhase += 1.0
    }

    modPhase := effectivePhase * w.ModRatio
    modPhase = modPhase - math.Floor(modPhase)
    if modPhase < 0.0 {
        modPhase += 1.0
    }

    modSignal := math.Sin(2.0 * math.Pi * modPhase)
    if w.ModWave != nil {
        modSignal = w.ModWave.Sample(modPhase, (modPhase > .5), 1.0)
    }
    if modMSB < 0.0 {
        modSignal = -modSignal
    }
    //sample := math.Sin((2.0*math.Pi*effectivePhase/w.PulseWidth) + w.ModIndex*modSignal)
    sample := math.Sin(2.0*math.Pi*effectivePhase + w.ModIndex*modSignal)

    if effectivePhase > w.PulseWidth {
        return 0
    }

    return sample
}
