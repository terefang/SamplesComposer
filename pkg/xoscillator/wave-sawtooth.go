package xoscillator

import (
    "math"
)

// --- Sawtooth Waveform (with Pulse Width Gating) ---

// SawtoothWaveform generates a rising sawtooth wave from -1.0 to +1.0 over
// phase < PulseWidth and stays silent (0.0) for phase >= PulseWidth.
type SawtoothWaveform struct {
    // PulseWidth sets the normalized phase window [0.02, 0.98] during which
    // the rising ramp is active before staying silent (0.0) for the rest of the cycle.
    PulseWidth float64
}

func (w *SawtoothWaveform) SetSeed(s int32) {
    //ignore
}

func (w *SawtoothWaveform) GetSeed() int32 {
    return 0
}

// NewSawtoothWaveform constructs a SawtoothWaveform with a default PulseWidth of 0.5.
func NewSawtoothWaveform() *SawtoothWaveform {
    return &SawtoothWaveform{PulseWidth: 0.5}
}

// NewSawtoothWaveformWithPulseWidth constructs a SawtoothWaveform with the given PulseWidth.
func NewSawtoothWaveformWithPulseWidth(pulseWidth float64) *SawtoothWaveform {
    return &SawtoothWaveform{PulseWidth: pulseWidth}
}

func (w *SawtoothWaveform) SetPulseWidth(width float64) {
    w.PulseWidth = width
}

func (w *SawtoothWaveform) GetPulseWidth() float64 {
    return w.PulseWidth
}

// Name returns "Sawtooth".
func (w *SawtoothWaveform) Name() string {
    return "Saw(tooth)"
}

// Sample evaluates the rising sawtooth wave at normalized phase in [0.0, 1.0),
// returning 0.0 when phase >= PulseWidth.
func (w *SawtoothWaveform) Sample(phase float64, _ bool, _ float64) float64 {
    if phase >= 1.0 {
        phase -= math.Floor(phase)
    }
    pw := math.Max(0.02, math.Min(1.0, w.PulseWidth))
    if phase < pw {
        return 2.0*(phase/pw) - 1.0
    }
    return 0.0
}
