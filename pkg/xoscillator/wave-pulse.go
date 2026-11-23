package xoscillator

import (
    "math"
)

// --- Pulse Waveform (with Pulse Width Modulation) ---

// PulseWaveform generates a two-level rectangular pulse wave whose duty cycle
// is controlled by PulseWidth.
type PulseWaveform struct {
    // PulseWidth sets the fraction of the cycle spent at +1.0 before dropping
    // to -1.0, clamped internally to [0.02, 0.98]. At 0.5 it is a square wave.
    PulseWidth float64
}

func (w *PulseWaveform) SetSeed(s int32) {
    //igonre
}

func (w *PulseWaveform) GetSeed() int32 {
    return 0
}

func (w *PulseWaveform) SetPulseWidth(width float64) {
    w.PulseWidth = width
}

func (w *PulseWaveform) GetPulseWidth() float64 {
    return w.PulseWidth
}

// NewPulseWaveform constructs a PulseWaveform with the initial duty cycle.
func NewPulseWaveform(pulseWidth float64) *PulseWaveform {
    return &PulseWaveform{PulseWidth: pulseWidth}
}

// Name returns "Pulse".
func (w *PulseWaveform) Name() string {
    return "Pulse/Square"
}

// Sample evaluates the pulse wave at normalized phase in [0.0, 1.0).
func (w *PulseWaveform) Sample(phase float64, _ bool, _ float64) float64 {
    if phase >= 1.0 {
        phase -= math.Floor(phase)
    }
    pw := math.Max(0.02, math.Min(0.98, w.PulseWidth))
    if phase < pw {
        return 1.0
    }
    return -1.0
}
