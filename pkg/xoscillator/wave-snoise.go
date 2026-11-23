package xoscillator

import (
    "math"
)

// --- Snoise Waveform ---

// SnoiseWaveform generates a two-level rectangular pulse wave whose duty cycle
// is controlled by PulseWidth.
type SnoiseWaveform struct {
    // PulseWidth sets the fraction of the cycle spent at +1.0 before dropping
    // to -1.0, clamped internally to [0.02, 0.98]. At 0.5 it is a square wave.
    PulseWidth float64
}

func (w *SnoiseWaveform) SetSeed(s int32) {
    //igonre
}

func (w *SnoiseWaveform) GetSeed() int32 {
    return 0
}

func (w *SnoiseWaveform) SetPulseWidth(width float64) {
    w.PulseWidth = width
}

func (w *SnoiseWaveform) GetPulseWidth() float64 {
    return w.PulseWidth
}

// NewSnoiseWaveform constructs a PulseWaveform with the initial duty cycle.
func NewSnoiseWaveform() *SnoiseWaveform {
    return &SnoiseWaveform{}
}

// Name returns "Pulse".
func (w *SnoiseWaveform) Name() string {
    return "Snoise"
}

// Sample evaluates the pulse wave at normalized phase in [0.0, 1.0).
func (w *SnoiseWaveform) Sample(phase float64, _ bool, _ float64) float64 {
    out := math.Sin(math.Pow(phase, 3) * w.PulseWidth)
    return out
}
