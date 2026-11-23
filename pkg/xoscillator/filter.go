package xoscillator

import "math"

// Filter processes audio samples at a given sample rate and supports resetting
// internal filter state.
type Filter interface {
    Name() string
    // ProcessBuffer post-processes an existing sample slice in place at
    // sampleRate Hz.
    ProcessBuffer(samples []float64, sampleRate float64)
}

// --- Resonant Filter ---

// FilterMode selects the response of a ResonantFilter.
type FilterMode int

const (
    // FilterOff bypasses filtering and passes input samples through unchanged.
    FilterOff FilterMode = iota

    // FilterLowPass attenuates frequencies above Cutoff with resonance emphasis.
    FilterLowPass

    // FilterBandPass passes frequencies centered around Cutoff.
    FilterBandPass

    // FilterHighPass attenuates frequencies below Cutoff with resonance emphasis.
    FilterHighPass
)

// ResonantFilter implements a two-pole state-variable resonant filter inspired
// by the analog filter section of the SID chip.
type ResonantFilter struct {
    // Cutoff is the target filter cutoff/center frequency in Hertz.
    Cutoff float64

    // Resonance controls filter feedback/peak around Cutoff in [0.0, 1.0],
    // where 0.0 is unresonant and values near 1.0 approach self-oscillation.
    Resonance float64

    // Mode selects FilterOff, FilterLowPass, FilterBandPass, or FilterHighPass.
    Mode FilterMode
}

func (f *ResonantFilter) Name() string {
    switch f.Mode {
    case FilterLowPass:
        return "Low Pass"
    case FilterBandPass:
        return "Band Pass"
    case FilterHighPass:
        return "High Pass"
    }
    return "Off"
}

// NewResonantFilter constructs a ResonantFilter with the given cutoff frequency
// in Hz, resonance in [0.0, 1.0], and FilterMode.
func NewResonantFilter(cutoff, resonance float64, mode FilterMode) *ResonantFilter {
    return &ResonantFilter{
        Cutoff:    cutoff,
        Resonance: resonance,
        Mode:      mode,
    }
}

// ProcessBuffer post-processes an existing sample slice in place at sampleRate Hz.
func (f *ResonantFilter) ProcessBuffer(samples []float64, sampleRate float64) {
    if len(samples) == 0 || f.Mode == FilterOff || sampleRate <= 0 {
        return
    }

    // Clamp cutoff below Nyquist to keep the state-variable integrators stable.
    nyquistLimit := sampleRate * 0.45
    fc := math.Max(10.0, math.Min(nyquistLimit, f.Cutoff))
    freqCoeff := 2.0 * math.Sin(math.Pi*fc/sampleRate)

    // Map resonance [0.0, 1.0] to damping factor q in [1.0, 0.05].
    res := math.Max(0.0, math.Min(1.0, f.Resonance))
    q := 1.0 - 0.95*res

    _low := 0.
    _band := 0.
    for i := range samples {

        // Two-pass oversampled state-variable step for stability at higher cutoffs.
        for pass := 0; pass < 2; pass++ {
            high := samples[i] - _low - q*_band
            _band += freqCoeff * high
            _low += freqCoeff * _band
        }

        switch f.Mode {
        case FilterLowPass:
            samples[i] = _low
        case FilterBandPass:
            samples[i] = _band
        case FilterHighPass:
            samples[i] = samples[i] - _low - q*_band
        }
    }
}
