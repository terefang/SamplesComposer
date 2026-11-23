package xoscillator

import "math"

type SimplexNoiseSampler struct {
    Seed int
}

func (s *SimplexNoiseSampler) SetSeed(seed int32) {
    s.Seed = int(seed)
}

var _ NoiseSampler = (*SimplexNoiseSampler)(nil)

func NewSimplexNoiseSampler(
    seed int,
) *SimplexNoiseSampler {
    return &SimplexNoiseSampler{
        Seed: seed,
    }
}

func (s *SimplexNoiseSampler) Name() string {
    return "Simplex"
}

func (s *SimplexNoiseSampler) SampleNoise(phase float64, period int) float64 {
    bipolar := singleSimplex(false, s.Seed, phase*float64(period), 0.5)
    return math.Max(-1.0, math.Min(1.0, bipolar))
}

type SineSimplexNoiseSampler struct {
    Seed int
}

func (s *SineSimplexNoiseSampler) SetSeed(seed int32) {
    s.Seed = int(seed)
}

var _ NoiseSampler = (*SineSimplexNoiseSampler)(nil)

func NewSineSimplexNoiseSampler(
    seed int,
) *SineSimplexNoiseSampler {
    return &SineSimplexNoiseSampler{
        Seed: seed,
    }
}

func (s *SineSimplexNoiseSampler) Name() string {
    return "SineSimplex"
}

func (s *SineSimplexNoiseSampler) SampleNoise(phase float64, period int) float64 {
    bipolar := singleSimplex(false, s.Seed, phase*float64(period), 0.5)
    return math.Sin(bipolar * math.Pi)
}

type CosineSimplexNoiseSampler struct {
    Seed int
}

func (s *CosineSimplexNoiseSampler) SetSeed(seed int32) {
    s.Seed = int(seed)
}

var _ NoiseSampler = (*CosineSimplexNoiseSampler)(nil)

func NewCosineSimplexNoiseSampler(
    seed int,
) *CosineSimplexNoiseSampler {
    return &CosineSimplexNoiseSampler{
        Seed: seed,
    }
}

func (s *CosineSimplexNoiseSampler) Name() string {
    return "CosineSimplex"
}

func (s *CosineSimplexNoiseSampler) SampleNoise(phase float64, period int) float64 {
    bipolar := singleSimplex(false, s.Seed, phase*float64(period), 0.5)
    return math.Cos(bipolar * math.Pi)
}

type X2xSimplexNoiseSampler struct {
    Seed int
}

func (s *X2xSimplexNoiseSampler) SetSeed(seed int32) {
    s.Seed = int(seed)
}

var _ NoiseSampler = (*X2xSimplexNoiseSampler)(nil)

func NewX2xSimplexNoiseSampler(
    seed int,
) *X2xSimplexNoiseSampler {
    return &X2xSimplexNoiseSampler{
        Seed: seed,
    }
}

func (s *X2xSimplexNoiseSampler) Name() string {
    return "X2xSimplex"
}

func (s *X2xSimplexNoiseSampler) SampleNoise(phase float64, period int) float64 {
    bipolar := singleSimplex(false, s.Seed, phase*float64(period), 0.5)
    bipolar = math.Pow(bipolar, 2) - (2 * bipolar)
    return math.Max(-1.0, math.Min(1.0, bipolar))
}
