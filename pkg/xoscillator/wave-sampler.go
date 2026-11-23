package xoscillator

import "math"

// --- Noise Sampler ---

// NoiseSampler defines an interface for noise sources evaluated over normalized
// oscillator phase in [0.0, 1.0), lattice period, and deterministic seed.
type NoiseSampler interface {
    // SampleNoise evaluates a normalized noise value for the given phase in
    // [0.0, 1.0), and lattice period.
    SampleNoise(phase float64, period int) float64
    SetSeed(seed int32)
    Name() string
}

// --- NoiseSampler Waveform ---

// NoiseSamplerWaveform generates a Waveform by delegating sample evaluation over
// normalized phase in [0.0, 1.0) and Period to a pluggable NoiseSampler.
type NoiseSamplerWaveform struct {
    // Sampler provides noise values over normalized phase and Period. If nil,
    // a default PerlinNoise1DSampler is used.
    Sampler NoiseSampler

    // Period sets the number of noise lattice cells per oscillator cycle
    // (defaults to 64 when <= 0).
    Period int

    // Gain scales the sampled noise value before clamping to [-1.0, 1.0].
    Gain float64

    // RingMod inverts the output polarity whenever modMSB is negative.
    RingMod bool

    Seed       int32
    PulseWidth float64
}

func (w *NoiseSamplerWaveform) SetSeed(s int32) {
    w.Seed = s
}

func (w *NoiseSamplerWaveform) GetSeed() int32 {
    return w.Seed
}

var _ Waveform = (*NoiseSamplerWaveform)(nil)

func (w *NoiseSamplerWaveform) SetPulseWidth(width float64) {
    w.PulseWidth = width
}

func (w *NoiseSamplerWaveform) GetPulseWidth() float64 {
    return w.PulseWidth
}

// NewNoiseSamplerWaveform constructs a NoiseSamplerWaveform with the given
// NoiseSampler, cycle period, gain, and ring-modulation setting.
func NewNoiseSamplerWaveform(sampler NoiseSampler, period int, gain float64, ringMod bool) *NoiseSamplerWaveform {
    return &NoiseSamplerWaveform{
        Sampler: sampler,
        Period:  period,
        Gain:    gain,
        RingMod: ringMod,
    }
}

// Name returns "Noise Sampler (Ring Mod)" when ring modulation is active,
// or "Noise Sampler".
func (w *NoiseSamplerWaveform) Name() string {
    if w.RingMod {
        return "Noise Sampler (Ring Mod) " + w.Sampler.Name()
    }
    return "Noise Sampler " + w.Sampler.Name()
}

// Sample evaluates the NoiseSampler at normalized phase in [0.0, 1.0),
// applying Gain, optional RingMod, and clamping to [-1.0, 1.0].
func (w *NoiseSamplerWaveform) Sample(phase float64, _ bool, modMSB float64) float64 {
    sampler := w.Sampler

    period := w.Period
    if period <= 0 {
        period = 64
    }
    sampler.SetSeed(w.Seed)
    phaseCycle := phase - math.Trunc(phase)
    if phaseCycle > w.PulseWidth {
        return 0
    }

    sample := sampler.SampleNoise(phase, period)

    gain := w.Gain
    if gain == 0 && w.Period == 0 && !w.RingMod {
        gain = 1.0
    }
    out := sample * gain

    if w.RingMod && modMSB < 0 {
        out = -out
    }

    return math.Max(-1.0, math.Min(1.0, out))
}
