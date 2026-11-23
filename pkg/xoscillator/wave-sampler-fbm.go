package xoscillator

import "math"

// FbmNoiseSampler implements NoiseSampler by layering octaves of an underlying
// NoiseSampler using fractal Brownian motion (fBm).
type FbmNoiseSampler struct {
    // Sampler is the base noise source evaluated per octave. If nil, a default
    // PerlinNoise1DSampler is used.
    Sampler NoiseSampler

    // Octaves sets how many successive fBm layers are combined (defaults to 4
    // when <= 0).
    Octaves int

    // Lacunarity is the frequency multiplier per octave (defaults to 2.0 when
    // <= 0).
    Lacunarity float64

    // Persistence is the amplitude multiplier per octave (defaults to 0.5 when
    // <= 0).
    Persistence float64
}

func (s *FbmNoiseSampler)  SetSeed(seed int32) {
    s.Sampler.SetSeed(seed)
}

var _ NoiseSampler = (*FbmNoiseSampler)(nil)

func (s *FbmNoiseSampler) Name() string {
    return "Fbm " + s.Sampler.Name()
}

// NewFbmNoiseSampler constructs an FbmNoiseSampler wrapping the given base
// NoiseSampler with explicit octave, lacunarity, and persistence settings.
func NewFbmNoiseSampler(
    sampler NoiseSampler,
    octaves int,
    lacunarity float64,
    persistence float64,
) *FbmNoiseSampler {

    if octaves <= 0 {
        octaves = 4
    }
    if lacunarity <= 0 {
        lacunarity = 2.0
    }
    if persistence <= 0 {
        persistence = 0.5
    }
    return &FbmNoiseSampler{
        Sampler:     sampler,
        Octaves:     octaves,
        Lacunarity:  lacunarity,
        Persistence: persistence,
    }
}

// SampleNoise evaluates multi-octave fBm noise over normalized phase in [0.0, 1.0)
// and base lattice period, returning a value in [-1.0, 1.0].
func (s *FbmNoiseSampler) SampleNoise(phase float64, period int) float64 {
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

        sum += sampler.SampleNoise(octavePhase, octavePeriod) * amp
        norm += amp

        amp *= persistence
        freq *= lacunarity
    }

    if norm <= 0 {
        return 0.0
    }
    return math.Max(-1.0, math.Min(1.0, sum/norm))
}
