package xoscillator

import (
    "slices"
    "strings"
)

// Waveform defines a named audio wave generator evaluated over a normalized
// oscillator phase in [0.0, 1.0).
//
// Implementations produce bipolar audio samples in [-1.0, 1.0] and may expose
// mutable synthesis parameters such as pulse width and deterministic seed
// state. Callers should not assume concurrency safety unless documented by a
// concrete implementation.
type Waveform interface {
    // Name returns a human-readable label identifying the waveform and any
    // active mode variants (such as pulse width modulation, ring modulation,
    // or noise source configuration).
    Name() string

    // Sample computes a single bipolar audio sample in [-1.0, 1.0] for the
    // given normalized oscillator phase in [0.0, 1.0) (values outside this
    // range are typically wrapped or clamped by implementations).
    //
    // crossedHalf indicates whether the oscillator phase crossed 0.5 during the
    // current step, which clock-driven generators (such as LFSR or clocked
    // noise waveforms) can use to advance internal state.
    //
    // modMSB supplies a ring-modulation polarity sign (typically -1.0 or +1.0)
    // from an external modulator for waveforms that support ring modulation.
    Sample(phase float64, crossedHalf bool, modMSB float64) float64

    // SetPulseWidth updates the waveform's pulse width parameter (typically
    // normalized to [0.0, 1.0] or clamped by the implementation). Waveforms
    // that do not use pulse width may treat this as a no-op.
    SetPulseWidth(width float64)

    // GetPulseWidth returns the waveform's current pulse width setting.
    // Waveforms that do not use pulse width may return a constant default (such
    // as 0.5).
    GetPulseWidth() float64

    // SetSeed sets the 32-bit deterministic seed used by seeded or
    // noise-backed waveforms to initialize or reset pseudo-random/lattice
    // state. Non-seeded waveforms may treat this as a no-op.
    SetSeed(s int32)

    // GetSeed returns the waveform's current 32-bit seed value, or 0 if the
    // waveform does not maintain seed state.
    GetSeed() int32
}

// WaveformFactory constructs a fresh Waveform instance for a registered name.
type WaveformFactory func() Waveform

var waveformFactories = map[string]WaveformFactory{
    "sawtooth":     func() Waveform { return NewSawtoothWaveform() },
    "pulse/square": func() Waveform { return NewPulseWaveform(.50) },
    "triangle":     func() Waveform { return NewTriangleWaveform(.5, false) },
    "sine":         func() Waveform { return &SineWaveform{} },
    "snoise":       func() Waveform { return NewSnoiseWaveform() },
}

type WaveSamplerFactory func(seed int, period int, gain float64, ringMod bool, octaves int, lacunarity, persistance, shape, rgain float64) Waveform

var waveSamplerFactories = map[string]WaveSamplerFactory{
    "simplex": func(seed int, period int, gain float64, ringMod bool, octaves int, lacunarity, persistance, shape, rgain float64) Waveform {
        return NewNoiseSamplerWaveform(
            NewSimplexNoiseSampler(seed), period, gain, ringMod)
    },
    "sine-simplex": func(seed int, period int, gain float64, ringMod bool, octaves int, lacunarity, persistance, shape, rgain float64) Waveform {
        return NewNoiseSamplerWaveform(
            NewSineSimplexNoiseSampler(seed), period, gain, ringMod)
    },
    "cosine-simplex": func(seed int, period int, gain float64, ringMod bool, octaves int, lacunarity, persistance, shape, rgain float64) Waveform {
        return NewNoiseSamplerWaveform(
            NewCosineSimplexNoiseSampler(seed), period, gain, ringMod)
    },
    "x2x-simplex": func(seed int, period int, gain float64, ringMod bool, octaves int, lacunarity, persistance, shape, rgain float64) Waveform {
        return NewNoiseSamplerWaveform(
            NewCosineSimplexNoiseSampler(seed), period, gain, ringMod)
    },
    "billow": func(seed int, period int, gain float64, ringMod bool, octaves int, lacunarity, persistance, shape, rgain float64) Waveform {
        return NewNoiseSamplerWaveform(NewBillowNoiseSampler(
            NewSimplexNoiseSampler(seed), octaves, lacunarity, persistance, shape), period, gain, ringMod)
    },
    "fbm": func(seed int, period int, gain float64, ringMod bool, octaves int, lacunarity, persistance, shape, rgain float64) Waveform {
        return NewNoiseSamplerWaveform(
            NewFbmNoiseSampler(
                NewSimplexNoiseSampler(seed), octaves, lacunarity, persistance), period, gain, ringMod)
    },
    "ridged": func(seed int, period int, gain float64, ringMod bool, octaves int, lacunarity, persistance, shape, rgain float64) Waveform {
        return NewNoiseSamplerWaveform(
            NewRidgedNoiseSampler(
                NewSimplexNoiseSampler(seed), octaves, lacunarity, persistance, shape, rgain), period, gain, ringMod)
    },
    "turbulence": func(seed int, period int, gain float64, ringMod bool, octaves int, lacunarity, persistance, shape, rgain float64) Waveform {
        return NewNoiseSamplerWaveform(
            NewTurbulenceNoiseSampler(
                NewSimplexNoiseSampler(seed), octaves, lacunarity, persistance), period, gain, ringMod)
    },
}

// WaveformNames returns all registered waveform keys sorted lexicographically.
func WaveformNames() []string {
    names := make([]string, 0, len(waveformFactories))
    for name := range waveformFactories {
        names = append(names, name)
    }
    slices.Sort(names)
    return names
}

// WaveSamplerNames returns all registered waveform keys sorted lexicographically.
func WaveSamplerNames() []string {
    names := make([]string, 0, len(waveSamplerFactories))
    for name := range waveSamplerFactories {
        names = append(names, name)
    }
    slices.Sort(names)
    return names
}

// HasWaveform reports whether name is registered in the waveform registry
// (case-insensitive, trimmed).
func HasWaveform(name string) bool {
    key := strings.ToLower(strings.TrimSpace(name))
    _, ok := waveformFactories[key]
    return ok
}

// HasWaveSampler reports whether name is registered in the waveSampler registry
// (case-insensitive, trimmed).
func HasWaveSampler(name string) bool {
    key := strings.ToLower(strings.TrimSpace(name))
    _, ok := waveSamplerFactories[key]
    return ok
}

// LookupWaveform returns a new Waveform instance for name (case-insensitive,
// trimmed) and whether it was found.
func LookupWaveform(name string) (Waveform, bool) {
    key := strings.ToLower(strings.TrimSpace(name))
    factory, ok := waveformFactories[key]
    if !ok {
        return nil, false
    }
    return factory(), true
}

// LookupWaveSampler returns a new WaveSampler instance for name (case-insensitive,
// trimmed) and whether it was found.
func LookupWaveSampler(name string, seed int, period int, gain float64, ringMod bool, octaves int, lacunarity, persistance, shape, rgain float64) (Waveform, bool) {
    key := strings.ToLower(strings.TrimSpace(name))
    factory, ok := waveSamplerFactories[key]
    if !ok {
        return nil, false
    }
    return factory(seed, period, gain, ringMod, octaves, lacunarity, persistance, shape, rgain), true
}
