package xoscillator

import (
    "math"
)

// --- Triangle Waveform (with Variable Peak / Pulse Width & Optional Ring Mod) ---

// TriangleWaveform generates a piecewise-linear triangle wave whose peak
// inflection point can be shifted via PulseWidth, with optional ring modulation.
type TriangleWaveform struct {
    // PulseWidth sets the normalized phase position [0.02, 0.98] where the
    // rising ramp reaches +1.0 and transitions into the falling ramp. A value
    // of 0.5 produces a symmetric triangle wave.
    PulseWidth float64

    // RingMod inverts the output polarity whenever modMSB is negative,
    // producing metallic/inharmonic ring-modulated spectra characteristic of the SID.
    RingMod bool
}

func (w *TriangleWaveform) SetSeed(s int32) {
    //none
}

func (w *TriangleWaveform) GetSeed() int32 {
    return 0
}

func (w *TriangleWaveform) SetPulseWidth(width float64) {
    w.PulseWidth = width
}

func (w *TriangleWaveform) GetPulseWidth() float64 {
    return w.PulseWidth
}

// NewTriangleWaveform constructs a TriangleWaveform with the given peak/pulse
// width and ring-modulation setting.
func NewTriangleWaveform(pulseWidth float64, ringMod bool) *TriangleWaveform {
    return &TriangleWaveform{
        PulseWidth: pulseWidth,
        RingMod:    ringMod,
    }
}

// Name returns "Triangle (Ring Mod)" when ring modulation is active, or "Triangle".
func (w *TriangleWaveform) Name() string {
    if w.RingMod {
        return "Triangle (Ring Mod)"
    }
    return "Triangle"
}

// Sample evaluates the triangle wave at normalized phase in [0.0, 1.0),
// applying PulseWidth skew and optional ring modulation.
func (w *TriangleWaveform) Sample(phase float64, _ bool, modMSB float64) float64 {
    if phase >= 1.0 {
        phase -= math.Floor(phase)
    }
    // Clamp to avoid division by zero at extreme phase boundaries.
    pw := math.Max(0.02, math.Min(0.98, w.PulseWidth))

    var tri float64
    if phase < pw {
        // Rise from -1.0 at phase=0 to +1.0 at phase=pw.
        tri = 2.0*(phase/pw) - 1.0
    } else {
        // Fall from +1.0 at phase=pw to -1.0 at phase=1.0.
        tri = 1.0 - 2.0*((phase-pw)/(1.0-pw))
    }

    if w.RingMod && modMSB < 0 {
        tri = -tri
    }
    return tri
}
