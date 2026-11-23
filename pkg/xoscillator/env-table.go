package xoscillator

import (
    "math"
    "slices"
    "strings"
)

// --- 16-Step Table Envelope Generator ---

// EnvelopeInterpolator computes an interpolated envelope value from four
// neighboring control levels (l0, l1, l2, l3) and a local segment phase in
// [0.0, 1.0), where l1 is the start of the active segment and l2 is the end.
type EnvelopeInterpolator func(l0, l1, l2, l3, phase float64) float64

// LinearInterpolate performs linear interpolation between l1 and l2 over phase
// in [0.0, 1.0].
func LinearInterpolate(_, l1, l2, _, phase float64) float64 {
    t := math.Max(0.0, math.Min(1.0, phase))
    return l1 + (l2-l1)*t
}

func SquareInterpolate(_, l1, l2, _, phase float64) float64 {
    t := math.Max(0.0, math.Min(1.0, phase))
    return l1 + (l2-l1)*t*t
}

// CosineInterpolate performs smooth cosine easing between l1 and l2 over phase
// in [0.0, 1.0].
func CosineInterpolate(_, l1, l2, _, phase float64) float64 {
    t := math.Max(0.0, math.Min(1.0, phase))
    mu := 0.5 * (1.0 - math.Cos(t*math.Pi))
    return l1*(1.0-mu) + l2*mu
}

// CatmullRomInterpolate performs Catmull-Rom cubic spline interpolation across
// four evenly spaced control levels (l0, l1, l2, l3) for phase in [0.0, 1.0],
// where l1 is at phase=0.0 and l2 is at phase=1.0.
func CatmullRomInterpolate(l0, l1, l2, l3, phase float64) float64 {
    t := math.Max(0.0, math.Min(1.0, phase))
    t2 := t * t
    t3 := t2 * t

    return 0.5 * (
        (2.0 * l1) +
            (-l0+l2)*t +
            (2.0*l0-5.0*l1+4.0*l2-l3)*t2 +
            (-l0+3.0*l1-3.0*l2+l3)*t3)
}

// HermiteInterpolate performs cubic Hermite interpolation across four evenly
// spaced control levels (l0, l1, l2, l3) for phase in [0.0, 1.0] with zero
// tension and zero bias.
func HermiteInterpolate(l0, l1, l2, l3, phase float64) float64 {
    t := math.Max(0.0, math.Min(1.0, phase))
    t2 := t * t
    t3 := t2 * t

    // Tangents at l1 and l2 estimated from neighboring points.
    m1 := 0.5 * ((l2 - l1) + (l1 - l0))
    m2 := 0.5 * ((l3 - l2) + (l2 - l1))

    h00 := 2.0*t3 - 3.0*t2 + 1.0
    h10 := t3 - 2.0*t2 + t
    h01 := -2.0*t3 + 3.0*t2
    h11 := t3 - t2

    return h00*l1 + h10*m1 + h01*l2 + h11*m2
}

func min4(a, b, c, d float64) float64 {
    return math.Min(math.Min(a, b), math.Min(c, d))
}

func max4(a, b, c, d float64) float64 {
    return math.Max(math.Max(a, b), math.Max(c, d))
}

func avg4(a, b, c, d float64) float64 {
    return (a + b + c + d) * 0.25
}

func median4(a, b, c, d float64) float64 {
    s := [4]float64{a, b, c, d}
    for i := 1; i < 4; i++ {
        key := s[i]
        j := i - 1
        for j >= 0 && s[j] > key {
            s[j+1] = s[j]
            j--
        }
        s[j+1] = key
    }
    return 0.5 * (s[1] + s[2])
}

// MinInterpolate computes the minimum of all four levels (l0, l1, l2, l3) and
// interpolates from l1 toward that minimum across phase in [0.0, 1.0].
func MinInterpolate(l0, l1, l2, l3, phase float64) float64 {
    t := math.Max(0.0, math.Min(1.0, phase))
    target := min4(l0, l1, l2, l3)
    return l1 + (target-l1)*t
}

// MaxInterpolate computes the maximum of all four levels (l0, l1, l2, l3) and
// interpolates from l1 toward that maximum across phase in [0.0, 1.0].
func MaxInterpolate(l0, l1, l2, l3, phase float64) float64 {
    t := math.Max(0.0, math.Min(1.0, phase))
    target := max4(l0, l1, l2, l3)
    return l1 + (target-l1)*t
}

// AvgInterpolate computes the arithmetic average of all four levels
// (l0, l1, l2, l3) and interpolates from l1 toward that average across phase
// in [0.0, 1.0].
func AvgInterpolate(l0, l1, l2, l3, phase float64) float64 {
    t := math.Max(0.0, math.Min(1.0, phase))
    target := avg4(l0, l1, l2, l3)
    return l1 + (target-l1)*t
}

// MedianInterpolate computes the median of all four levels (l0, l1, l2, l3)
// and interpolates from l1 toward that median across phase in [0.0, 1.0].
func MedianInterpolate(l0, l1, l2, l3, phase float64) float64 {
    t := math.Max(0.0, math.Min(1.0, phase))
    target := median4(l0, l1, l2, l3)
    return l1 + (target-l1)*t
}

// Interpolator registry

var envelopeInterpolators = map[string]EnvelopeInterpolator{
    "linear":      LinearInterpolate,
    "square":      SquareInterpolate,
    "cosine":      CosineInterpolate,
    "catmull-rom": CatmullRomInterpolate,
    "hermite":     HermiteInterpolate,
    "min":         MinInterpolate,
    "max":         MaxInterpolate,
    "median":      MedianInterpolate,
    "average":     AvgInterpolate,
}

func normalizeEnvelopeInterpolatorName(name string) string {
    return strings.ToLower(strings.TrimSpace(name))
}

// EnvelopeInterpolatorNames returns all registered interpolator keys sorted
// lexicographically.
func EnvelopeInterpolatorNames() []string {
    names := make([]string, 0, len(envelopeInterpolators))
    for name := range envelopeInterpolators {
        names = append(names, name)
    }
    slices.Sort(names)
    return names
}

// HasEnvelopeInterpolator reports whether name is registered in the
// interpolator registry (case-insensitive, trimmed).
func HasEnvelopeInterpolator(name string) bool {
    _, ok := envelopeInterpolators[normalizeEnvelopeInterpolatorName(name)]
    return ok
}

// LookupEnvelopeInterpolator returns the EnvelopeInterpolator for name
// (case-insensitive, trimmed) and whether it was found.
func LookupEnvelopeInterpolator(name string) (EnvelopeInterpolator, bool) {
    interp, ok := envelopeInterpolators[normalizeEnvelopeInterpolatorName(name)]
    return interp, ok
}

// --- Parameterized Pairwise EnvelopeInterpolator Functions ---

// PairwiseInterpolator interpolates between two levels a and b at phase t.
type PairwiseInterpolator func(a, b, t float64) float64

func defaultPairwiseLerp(a, b, t float64) float64 {
    return a + (b-a)*t
}

func pairwiseSixWith(l0, l1, l2, l3, phase float64, fn PairwiseInterpolator) [6]float64 {
    if fn == nil {
        fn = defaultPairwiseLerp
    }

    p01 := phase + 1.0
    p02 := (phase + 1.0) / 2.0
    p03 := (phase + 1.0) / 3.0
    p12 := phase
    p13 := phase / 2.0
    p23 := phase - 1.0

    return [6]float64{
        fn(l0, l1, p01),
        fn(l0, l2, p02),
        fn(l0, l3, p03),
        fn(l1, l2, p12),
        fn(l1, l3, p13),
        fn(l2, l3, p23),
    }
}

// MinPairwiseInterpolate evaluates fn across all six pairs of (l0, l1, l2, l3)
// using adjusted phase parameters p for pairs outside l1 and l2, and returns
// the lowest value. If fn is nil, linear interpolation is used.
func MinPairwiseInterpolate(l0, l1, l2, l3, phase float64, fn PairwiseInterpolator) float64 {
    return min6(pairwiseSixWith(l0, l1, l2, l3, phase, fn))
}

// MaxPairwiseInterpolate evaluates fn across all six pairs of (l0, l1, l2, l3)
// using adjusted phase parameters p for pairs outside l1 and l2, and returns
// the highest value. If fn is nil, linear interpolation is used.
func MaxPairwiseInterpolate(l0, l1, l2, l3, phase float64, fn PairwiseInterpolator) float64 {
    return max6(pairwiseSixWith(l0, l1, l2, l3, phase, fn))
}

// AvgPairwiseInterpolate evaluates fn across all six pairs of (l0, l1, l2, l3)
// using adjusted phase parameters p for pairs outside l1 and l2, and returns
// their average value. If fn is nil, linear interpolation is used.
func AvgPairwiseInterpolate(l0, l1, l2, l3, phase float64, fn PairwiseInterpolator) float64 {
    return avg6(pairwiseSixWith(l0, l1, l2, l3, phase, fn))
}

// MedianPairwiseInterpolate evaluates fn across all six pairs of (l0, l1, l2, l3)
// using adjusted phase parameters p for pairs outside l1 and l2, and returns
// the median value. If fn is nil, linear interpolation is used.
func MedianPairwiseInterpolate(l0, l1, l2, l3, phase float64, fn PairwiseInterpolator) float64 {
    return median6(pairwiseSixWith(l0, l1, l2, l3, phase, fn))
}
func min6(v [6]float64) float64 {
    m := v[0]
    for i := 1; i < 6; i++ {
        if v[i] < m {
            m = v[i]
        }
    }
    return m
}

func max6(v [6]float64) float64 {
    m := v[0]
    for i := 1; i < 6; i++ {
        if v[i] > m {
            m = v[i]
        }
    }
    return m
}

func avg6(v [6]float64) float64 {
    sum := 0.0
    for i := 0; i < 6; i++ {
        sum += v[i]
    }
    return sum / 6.0
}

func median6(v [6]float64) float64 {
    s := v
    for i := 1; i < 6; i++ {
        key := s[i]
        j := i - 1
        for j >= 0 && s[j] > key {
            s[j+1] = s[j]
            j--
        }
        s[j+1] = key
    }
    return 0.5 * (s[2] + s[3])
}

// TableEnvelope implements Envelope using a total DurationSec, 16 evenly
// spaced control levels, and a custom 4-point interpolation function.
type TableEnvelope struct {
    // DurationSec is the total length of the 16-point envelope table in seconds.
    DurationSec float64

    // Levels holds the 16 evenly spaced envelope control levels across DurationSec.
    Levels [16]float64

    // Interpolate evaluates the envelope value given (l0, l1, l2, l3, phase).
    // If nil, linear interpolation between l1 and l2 is used.
    Interpolate EnvelopeInterpolator

    // Loop restarts playback from 0 when reaching DurationSec while Gate is true.
    Loop bool

    // Gate controls whether the envelope is actively held/playing. When false,
    // the output decays smoothly toward 0.0.
    Gate bool

    elapsed float64
    level   float64
    active  bool
}

// NewTableEnvelope constructs a TableEnvelope with the given total duration,
// 16 levels, and 4-point interpolation function, starting with Gate set to true.
func NewTableEnvelope(durationSec float64, levels [16]float64, interp EnvelopeInterpolator) *TableEnvelope {
    return &TableEnvelope{
        DurationSec: durationSec,
        Levels:      levels,
        Interpolate: interp,
        Gate:        true,
        active:      true,
    }
}

type ExtendedEnvelopeInterpolator func(l0, l1, l2, l3, phase float64, fn PairwiseInterpolator) float64

func NewTableExtendedEnvelope(durationSec float64, levels [16]float64, interp ExtendedEnvelopeInterpolator, pp PairwiseInterpolator) *TableEnvelope {
    return &TableEnvelope{
        DurationSec: durationSec,
        Levels:      levels,
        Interpolate: func(l0, l1, l2, l3, phase float64) float64 {
            return interp(l0, l1, l2, l3, phase, pp)
        },
        Gate:   true,
        active: true,
    }
}

// ApplyBuffer post-processes an existing sample slice in place at sampleRate Hz.
func (t *TableEnvelope) ApplyBuffer(samples []float64, sampleRate float64) {
    sLen := len(samples)
    if sLen == 0 || sampleRate <= 0 {
        return
    }
    for i := range samples {
        dt := float64(i) / sampleRate
        l0 := 0
        l1 := 0
        l2 := 0
        l3 := 0
        phase := (dt - (math.Trunc(dt/t.DurationSec) * t.DurationSec)) / t.DurationSec
        for x := 15; x >= 0; x-- {
            if dt < (float64(x)+1)*t.DurationSec {
                l0 = x - 1
                l1 = x
                l2 = x + 1
                l3 = x + 2
            }
        }

        if l0 < 0 {
            l0 = 0
        }
        if l1 < 0 {
            l1 = 0
        }
        if l2 < 0 {
            l2 = 0
        }
        if l3 < 0 {
            l3 = 0
        }
        if l0 > 15 {
            l0 = 15
        }
        if l1 > 15 {
            l1 = 15
        }
        if l2 > 15 {
            l2 = 15
        }
        if l3 > 15 {
            l3 = 15
        }

        //mod := Lerp(t.Levels[l1], t.Levels[l2], phase)
        mod := t.Interpolate(t.Levels[l0], t.Levels[l1], t.Levels[l2], t.Levels[l3], phase)
        samples[i] *= mod
    }
}

// GetCycleWidth returns the minimum duration in seconds required for a full TableEnvelope cycle.
func (t *TableEnvelope) GetCycleWidth() float64 {
    return math.Max(0.0, t.DurationSec)
}
