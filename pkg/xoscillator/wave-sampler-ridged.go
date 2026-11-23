package xoscillator

import "math"

// RidgedNoiseSampler implements NoiseSampler by layering octaves of an underlying
// NoiseSampler folded into sharp ridges (ridged multifractal noise).
type RidgedNoiseSampler struct {
    // Sampler is the base noise source evaluated per octave. If nil, a default
    // PerlinNoise1DSampler is used.
    Sampler NoiseSampler

    // Octaves sets how many successive ridged layers are summed (defaults to 4
    // when <= 0).
    Octaves int

    // Lacunarity is the frequency multiplier per octave (defaults to 2.0 when
    // <= 0).
    Lacunarity float64

    // Persistence is the base amplitude decay per octave (defaults to 0.5 when
    // <= 0).
    Persistence float64

    // Sharpness controls ridge exponent sharpening (defaults to 2.0 when <= 0).
    Sharpness float64

    // RidgeGain controls how strongly each ridge weights subsequent octaves
    // (defaults to 2.0 when <= 0).
    RidgeGain float64
}

func (s *RidgedNoiseSampler) SetSeed(seed int32) {
    s.Sampler.SetSeed(seed)
}

var _ NoiseSampler = (*RidgedNoiseSampler)(nil)

func (s *RidgedNoiseSampler) Name() string {
    return "Ridged " + s.Sampler.Name()
}

// NewRidgedNoiseSampler constructs a RidgedNoiseSampler wrapping the given base
// NoiseSampler with ridged multifractal parameters.
func NewRidgedNoiseSampler(
    sampler NoiseSampler,
    octaves int,
    lacunarity float64,
    persistence float64,
    sharpness float64,
    ridgeGain float64,
) *RidgedNoiseSampler {

    if octaves <= 0 {
        octaves = 4
    }
    if lacunarity <= 0 {
        lacunarity = 2.0
    }
    if persistence <= 0 {
        persistence = 0.5
    }
    if sharpness <= 0 {
        sharpness = 2.0
    }
    if ridgeGain <= 0 {
        ridgeGain = 2.0
    }
    return &RidgedNoiseSampler{
        Sampler:     sampler,
        Octaves:     octaves,
        Lacunarity:  lacunarity,
        Persistence: persistence,
        Sharpness:   sharpness,
        RidgeGain:   ridgeGain,
    }
}

func clamp01(v float64) float64 {
    if v < 0.0 {
        return 0.0
    }
    if v > 1.0 {
        return 1.0
    }
    return v
}

// SampleNoise evaluates ridged multifractal noise over normalized phase in
// [0.0, 1.0) and base lattice period, returning a value in [-1.0, 1.0].
func (s *RidgedNoiseSampler) SampleNoise(phase float64, period int) float64 {
    sampler := s.Sampler

    octaves := s.Octaves
    if octaves <= 0 {
        octaves = 4
    }

    lacunarity := s.Lacunarity
    if lacunarity <= 0 {
        lacunarity = 2.0
    }

    persistence := s.Persistence
    if persistence <= 0 {
        persistence = 0.5
    }

    sharpness := s.Sharpness
    if sharpness <= 0 {
        sharpness = 2.0
    }

    ridgeGain := s.RidgeGain
    if ridgeGain <= 0 {
        ridgeGain = 2.0
    }

    p := phase - math.Floor(phase)

    amp := 1.0
    freq := 1.0
    weight := 1.0
    sum := 0.0
    norm := 0.0

    for o := 0; o < octaves; o++ {
        octavePeriod := int(math.Round(float64(period) * freq))
        octavePhase := p * freq
        octavePhase -= math.Floor(octavePhase)

        raw := sampler.SampleNoise(octavePhase, octavePeriod)
        signal := 1.0 - math.Abs(raw)
        signal = math.Pow(clamp01(signal), sharpness)
        signal *= weight

        weight = clamp01(signal * ridgeGain)

        sum += signal * amp
        norm += amp

        amp *= persistence
        freq *= lacunarity
    }

    if norm <= 0 {
        return 0.0
    }

    // Map [0.0, 1.0] multifractal energy into bipolar [-1.0, 1.0].
    bipolar := (sum/norm)*2.0 - 1.0
    return math.Max(-1.0, math.Min(1.0, bipolar))
}
