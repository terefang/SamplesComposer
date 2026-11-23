package xoscillator

import "math"

// BillowNoiseSampler implements NoiseSampler by layering octaves of an underlying
// NoiseSampler with rectified/rounded lobes for billowy noise shapes.
type BillowNoiseSampler struct {
    // Sampler is the base noise source evaluated per octave. If nil, a default
    // PerlinNoise1DSampler is used.
    Sampler NoiseSampler

    // Octaves sets how many successive billow layers are combined (defaults to 4
    // when <= 0).
    Octaves int

    // Lacunarity is the frequency multiplier per octave (defaults to 2.0 when
    // <= 0).
    Lacunarity float64

    // Persistence is the amplitude multiplier per octave (defaults to 0.5 when
    // <= 0).
    Persistence float64

    // BillowShape is an optional exponent applied to each rectified lobe in
    // [0.0, 1.0] (defaults to 1.0 when <= 0).
    BillowShape float64
}

func (s *BillowNoiseSampler) SetSeed(seed int32) {
    s.Sampler.SetSeed(seed)
}

var _ NoiseSampler = (*BillowNoiseSampler)(nil)

// NewBillowNoiseSampler constructs a BillowNoiseSampler wrapping the given base
// NoiseSampler with billowy multifractal parameters.
func NewBillowNoiseSampler(
    sampler NoiseSampler,
    octaves int,
    lacunarity float64,
    persistence float64,
    billowShape float64,
) *BillowNoiseSampler {
    if octaves <= 0 {
        octaves = 4
    }
    if lacunarity <= 0 {
        lacunarity = 2.0
    }
    if persistence <= 0 {
        persistence = 0.5
    }
    if billowShape <= 0 {
        billowShape = 1.0
    }
    return &BillowNoiseSampler{
        Sampler:     sampler,
        Octaves:     octaves,
        Lacunarity:  lacunarity,
        Persistence: persistence,
        BillowShape: billowShape,
    }
}

func (s *BillowNoiseSampler) Name() string {
    return "Billow " + s.Sampler.Name()
}

// SampleNoise evaluates billowy multifractal noise over normalized phase in
// [0.0, 1.0) and base lattice period, returning a value in [-1.0, 1.0].
func (s *BillowNoiseSampler) SampleNoise(phase float64, period int) float64 {
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

    billowShape := s.BillowShape
    if billowShape <= 0 {
        billowShape = 1.0
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

        raw := sampler.SampleNoise(octavePhase, octavePeriod)
        lobe := clamp01(math.Abs(raw))
        if billowShape != 1.0 {
            lobe = math.Pow(lobe, billowShape)
        }

        sum += lobe * amp
        norm += amp

        amp *= persistence
        freq *= lacunarity
    }

    if norm <= 0 {
        return 0.0
    }

    // Map [0.0, 1.0] billow energy into bipolar [-1.0, 1.0].
    bipolar := (sum/norm)*2.0 - 1.0
    return math.Max(-1.0, math.Min(1.0, bipolar))
}
