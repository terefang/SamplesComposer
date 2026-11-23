package xoscillator

import "math"

// --- Sine Waveform ---

// SineWaveform generates a standard sine wave over normalized oscillator phase in [0.0, 1.0).
type SineWaveform struct {
    // RingMod inverts the output polarity whenever modMSB is negative.
    RingMod    bool
    PulseWidth float64
}

func (w *SineWaveform) SetSeed(s int32) {
    //ignore
}

func (w *SineWaveform) GetSeed() int32 {
    return 0
}

var _ Waveform = (*SineWaveform)(nil)

func (w *SineWaveform) SetPulseWidth(width float64) {
    w.PulseWidth = width
}

func (w *SineWaveform) GetPulseWidth() float64 {
    return w.PulseWidth
}

// Name returns "Sine (Ring Mod)" when ring modulation is active, or "Sine".
func (w *SineWaveform) Name() string {
    if w.RingMod {
        return "Sine (Ring Mod)"
    }
    return "Sine"
}

// Sample evaluates the sine waveform at normalized phase in [0.0, 1.0), returning a value in [-1.0, 1.0].
func (w *SineWaveform) Sample(phase float64, _ bool, modMSB float64) float64 {
    if phase >= 1.0 {
        phase -= math.Floor(phase)
    }
    // Normalize phase to [0, 2π) radians.
    p := phase - math.Floor(phase)
    sinVal := math.Sin(p * 2.0 * math.Pi)

    if phase >= w.PulseWidth {
        sinVal = 0.0
    }

    if w.RingMod && modMSB < 0 {
        return -sinVal
    }
    return sinVal
}
