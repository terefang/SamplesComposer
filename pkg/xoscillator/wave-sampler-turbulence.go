package xoscillator

import "math"

// TurbulenceNoiseSampler implements NoiseSampler by summing absolute-folded
// octaves of an underlying NoiseSampler for turbulent noise.
type TurbulenceNoiseSampler struct {
    // Sampler is the base noise source evaluated per octave. If nil, a default
    // PerlinNoise1DSampler is used.
    Sampler NoiseSampler

    // Octaves sets how many successive turbulence layers are summed (defaults
    // to 4 when <= 0).
    Octaves int

    // Lacunarity is the frequency multiplier per octave (defaults to 2.0 when
    // <= 0).
    Lacunarity float64

    // Persistence is the amplitude multiplier per octave (defaults to 0.5 when
    // <= 0).
    Persistence float64
}

func (s *TurbulenceNoiseSampler) SetSeed(seed int32) {
    s.Sampler.SetSeed(seed)
}

var _ NoiseSampler = (*TurbulenceNoiseSampler)(nil)

func (s *TurbulenceNoiseSampler) Name() string {
    return "Turbulence " + s.Sampler.Name()
}

// NewTurbulenceNoiseSampler constructs a TurbulenceNoiseSampler wrapping the
// given base NoiseSampler with octave, lacunarity, and persistence settings.
func NewTurbulenceNoiseSampler(
    sampler NoiseSampler,
    octaves int,
    lacunarity float64,
    persistence float64,
) *TurbulenceNoiseSampler {

    if octaves <= 0 {
        octaves = 4
    }
    if lacunarity <= 0 {
        lacunarity = 2.0
    }
    if persistence <= 0 {
        persistence = 0.5
    }
    return &TurbulenceNoiseSampler{
        Sampler:     sampler,
        Octaves:     octaves,
        Lacunarity:  lacunarity,
        Persistence: persistence,
    }
}

// SampleNoise evaluates turbulent noise over normalized phase in [0.0, 1.0) and
// base lattice period, returning a value in [-1.0, 1.0].
func (s *TurbulenceNoiseSampler) SampleNoise(phase float64, period int) float64 {
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

    p := phase - math.Floor(phase)

    amp := 1.0
    freq := 1.0
    sum := 0.0
    norm := 0.0

    for o := 0; o < octaves; o++ {
        octavePeriod := int(math.Round(float64(period) * freq))
        octavePhase := p * freq
        octavePhase -= math.Floor(octavePhase)

        turb := clamp01(math.Abs(sampler.SampleNoise(octavePhase, octavePeriod)))
        sum += turb * amp
        norm += amp

        amp *= persistence
        freq *= lacunarity
    }

    if norm <= 0 {
        return 0.0
    }

    // Map [0.0, 1.0] turbulence energy into bipolar [-1.0, 1.0].
    bipolar := (sum/norm)*2.0 - 1.0
    return math.Max(-1.0, math.Min(1.0, bipolar))
}
