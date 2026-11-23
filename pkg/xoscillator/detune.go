package xoscillator

import (
    "slices"
    "strings"
)

// DetuneCurve determines how pitch detune spread is distributed across stacked
// voices from center to outer edges.
type DetuneCurve int

const (
    // DetuneCurveLinear distributes detune linearly across voice offsets.
    DetuneCurveLinear DetuneCurve = iota
    // DetuneCurveExponential concentrates voices closer to center and pushes
    // outer voices wider.
    DetuneCurveExponential
    // DetuneCurveSCurve applies a smooth S-curve distribution across voices.
    DetuneCurveSCurve
)

var detuneCurves = map[string]DetuneCurve{
    "linear":      DetuneCurveLinear,
    "exponential": DetuneCurveExponential,
    "scurve":      DetuneCurveSCurve,
}

func normalizeDetuneCurveName(name string) string {
    return strings.ToLower(strings.TrimSpace(name))
}

// DetuneCurveNames returns all registered detune curve keys sorted
// lexicographically.
func DetuneCurveNames() []string {
    names := make([]string, 0, len(detuneCurves))
    for name := range detuneCurves {
        names = append(names, name)
    }
    slices.Sort(names)
    return names
}

// HasDetuneCurve reports whether name is registered in the detune curve
// registry (case-insensitive, trimmed).
func HasDetuneCurve(name string) bool {
    _, ok := detuneCurves[normalizeDetuneCurveName(name)]
    return ok
}

// LookupDetuneCurve returns the DetuneCurve for name (case-insensitive,
// trimmed) and whether it was found.
func LookupDetuneCurve(name string) (DetuneCurve, bool) {
    curve, ok := detuneCurves[normalizeDetuneCurveName(name)]
    return curve, ok
}

// DetuneStage defines an audio detuning stage that modifies sample buffers in
// place at a given sample rate for both mono and interleaved stereo
// configurations, with common multi-voice controls.
type DetuneStage interface {
    // Name returns a human-readable label identifying the detune stage and
    // its configuration.
    Name() string

    // DetuneBuffer processes a mono slice of audio samples in place at
    // sampleRate Hz.
    DetuneBuffer(samples []float64, sampleRate float64)

    // DetuneBufferStereo processes an interleaved stereo slice ([L0, R0, L1, R1, ...])
    // in place at sampleRate Hz, applying stereo spread/panning where supported.
    DetuneBufferStereo(interleavedSamples []float64, sampleRate float64)

    // SetVoices updates the number of stacked/detuned voices.
    SetVoices(voices int)

    // SetDetune updates the detune spread amount (typically normalized to [0.0, 1.0]).
    SetDetune(detune float64)

    // SetDetuneCurve updates how detune spread is distributed across voices.
    SetDetuneCurve(curve DetuneCurve)

    // SetMix updates the wet/dry mix balance (typically normalized to [0.0, 1.0]).
    SetMix(mix float64)

    // SetAnalogDrift updates the organic analog drift amount, clamped to [0.0, 1.0].
    SetAnalogDrift(drift float64)

    // SetStereoWidth updates stereo spread across left and right channels
    // (typically normalized to [0.0, 1.0]).
    SetStereoWidth(width float64)

    // SetRate updates the modulation or cycle rate in Hertz.
    SetRate(rate float64)

    // SetCenterDelayMs updates the nominal delay offset in milliseconds.
    SetCenterDelayMs(delayMs float64)
}
