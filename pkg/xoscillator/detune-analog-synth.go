package xoscillator

import (
    "fmt"
    "math"
    "slices"
    "strings"
)

const defaultAnalogDriftDepth = 0.0022

// AnalogSynthDetuneStage implements DetuneStage using a configurable
// multi-voice analog-style detune architecture with curve-shaped pitch/delay
// spread, slow voice modulation, optional organic analog drift, sub-octave
// reinforcement, and stereo width panning.
//
// Callers should not assume concurrency safety unless external synchronization
// is provided.
type AnalogSynthDetuneStage struct {
    // Voices sets the number of detuned voices (clamped to [1, 16], defaulting
    // to 7 when <= 0).
    Voices int

    // VoiceOffsets specifies normalized voice positions (typically in
    // [-1.0, 1.0]) from left/lower to right/upper voices. When shorter than
    // Voices, remaining offsets are generated symmetrically.
    VoiceOffsets []float64

    // Detune controls detune spread from 0.0 (unison) to 1.0 (wide spread,
    // clamped to [0.0, 1.0]).
    Detune float64

    // DetuneCurve controls how detune spread is shaped across voices.
    DetuneCurve DetuneCurve

    // Mix controls the balance between the dry signal and the stacked detuned
    // voices from 0.0 to 1.0 (clamped to [0.0, 1.0]).
    Mix float64

    // StereoWidth sets stereo spread from 0.0 (Mono) to 1.0 (full L/R pan,
    // clamped to [0.0, 1.0]).
    StereoWidth float64

    // Rate is the slow LFO modulation rate in Hertz (defaults to 0.85 Hz when
    // <= 0).
    Rate float64

    // CenterDelayMs is the nominal delay offset in milliseconds (defaults to
    // 11.0 ms when < 0).
    CenterDelayMs float64

    // AnalogDrift controls slow, organic per-voice pitch/delay drift from 0.0
    // (none) to 1.0 (pronounced analog wander, clamped to [0.0, 1.0]).
    AnalogDrift float64

    // AnalogDriftDepth sets the maximum drift delay excursion in seconds per
    // unit of AnalogDrift (defaults to 0.0022 when < 0 or non-finite).
    AnalogDriftDepth float64

    // SubOctave controls sub-octave reinforcement from 0.0 (none) to 1.0
    // (full sub-octave blend, clamped to [0.0, 1.0]).
    SubOctave float64
}

var _ DetuneStage = (*AnalogSynthDetuneStage)(nil)

func defaultAnalogSynthVoiceOffsets() []float64 {
    return []float64{-1.0, -0.55, -0.22, 0.0, 0.22, 0.55, 1.0}
}

// NewDefaultAnalogSynthDetuneStage constructs an AnalogSynthDetuneStage using
// SetAnalogSynthDefaults (7 voices, classic symmetric voice offsets, default
// LFO rate, default center delay, zero analog drift, default drift depth, and
// zero sub-octave) with detune spread, mix, and stereo width clamped to
// [0.0, 1.0].
func NewDefaultAnalogSynthDetuneStage(
    detune float64,
    detuneCurve DetuneCurve,
    mix float64,
    stereoWidth float64,
) *AnalogSynthDetuneStage {
    stage := &AnalogSynthDetuneStage{
        Detune:      clamp01(detune),
        Mix:         clamp01(mix),
        StereoWidth: clamp01(stereoWidth),
    }
    stage.SetAnalogSynthDefaults()
    stage.SetDetuneCurve(detuneCurve)
    return stage
}

// NewAnalogSynthDetuneStageCommon constructs an AnalogSynthDetuneStage with variable
// voice count, custom voice offsets, detune amount, detune curve, wet/dry
// mix, and stereo width.
func NewAnalogSynthDetuneStageCommon(
    voices int,
    voiceOffsets []float64,
    detune float64,
    detuneCurve DetuneCurve,
    mix float64,
    stereoWidth float64,
) *AnalogSynthDetuneStage {
    if voices <= 0 {
        voices = 7
    } else if voices > 16 {
        voices = 16
    }

    var offsetsCopy []float64
    if len(voiceOffsets) > 0 {
        offsetsCopy = make([]float64, len(voiceOffsets))
        copy(offsetsCopy, voiceOffsets)
    }

    stage := &AnalogSynthDetuneStage{
        Voices:           voices,
        VoiceOffsets:     offsetsCopy,
        Detune:           clamp01(detune),
        Mix:              clamp01(mix),
        StereoWidth:      clamp01(stereoWidth),
        Rate:             0.85,
        CenterDelayMs:    11.0,
        AnalogDrift:      0.0,
        AnalogDriftDepth: defaultAnalogDriftDepth,
        SubOctave:        0.0,
    }
    stage.SetDetuneCurve(detuneCurve)
    return stage
}

// NewAnalogSynthDetuneStage constructs an AnalogSynthDetuneStage with all
// voice, detune, mix, stereo width, modulation rate, and delay parameters
// explicitly specified and normalized via setters.
func NewAnalogSynthDetuneStage(
    voices int,
    voiceOffsets []float64,
    detune float64,
    detuneCurve DetuneCurve,
    mix float64,
    stereoWidth float64,
    rate float64,
    centerDelayMs float64,
    subOctave float64,
) *AnalogSynthDetuneStage {
    stage := &AnalogSynthDetuneStage{}
    stage.SetVoices(voices)
    stage.SetVoiceOffsets(voiceOffsets)
    stage.SetDetune(detune)
    stage.SetDetuneCurve(detuneCurve)
    stage.SetMix(mix)
    stage.SetStereoWidth(stereoWidth)
    stage.SetRate(rate)
    stage.SetCenterDelayMs(centerDelayMs)
    stage.SetAnalogDriftDepth(defaultAnalogDriftDepth)
    stage.SetSubOctave(subOctave)
    return stage
}

// SetAnalogSynthDefaults restores the default 7-voice count, symmetric voice
// offsets, modulation rate, center delay, zero analog drift, default drift
// depth, and zero sub-octave.
func (d *AnalogSynthDetuneStage) SetAnalogSynthDefaults() {
    d.Voices = 7
    d.VoiceOffsets = defaultAnalogSynthVoiceOffsets()
    d.Rate = 0.85
    d.CenterDelayMs = 11.0
    d.AnalogDrift = 0.0
    d.AnalogDriftDepth = defaultAnalogDriftDepth
    d.SubOctave = 0.0
}

// SetVoices updates the number of detuned voices, clamping to [1, 16] and
// defaulting to 7 when <= 0.
func (d *AnalogSynthDetuneStage) SetVoices(voices int) {
    if voices <= 0 {
        d.Voices = 7
        return
    }
    if voices > 16 {
        d.Voices = 16
        return
    }
    d.Voices = voices
}

// SetVoiceOffsets updates the per-voice normalized offset slice, copying the
// input slice so external mutations do not affect stage state. Passing nil or
// an empty slice clears custom offsets in favor of generated offsets.
func (d *AnalogSynthDetuneStage) SetVoiceOffsets(offsets []float64) {
    if len(offsets) == 0 {
        d.VoiceOffsets = nil
        return
    }
    d.VoiceOffsets = make([]float64, len(offsets))
    copy(d.VoiceOffsets, offsets)
}

// SetDetune updates the detune spread amount, clamped to [0.0, 1.0].
func (d *AnalogSynthDetuneStage) SetDetune(detune float64) {
    d.Detune = clamp01(detune)
}

// SetDetuneCurve updates the detune distribution curve used across voices,
// defaulting unrecognized values to DetuneCurveLinear.
func (d *AnalogSynthDetuneStage) SetDetuneCurve(curve DetuneCurve) {
    switch curve {
    case DetuneCurveLinear, DetuneCurveExponential, DetuneCurveSCurve:
        d.DetuneCurve = curve
    default:
        d.DetuneCurve = DetuneCurveLinear
    }
}

// SetMix updates the wet/dry balance between detuned voices and the dry
// signal, clamped to [0.0, 1.0].
func (d *AnalogSynthDetuneStage) SetMix(mix float64) {
    d.Mix = clamp01(mix)
}

// SetStereoWidth updates stereo spread across left and right channels,
// clamped to [0.0, 1.0].
func (d *AnalogSynthDetuneStage) SetStereoWidth(width float64) {
    d.StereoWidth = clamp01(width)
}

// SetRate updates the slow LFO modulation rate in Hertz. Non-positive or
// non-finite values default to 0.85 Hz.
func (d *AnalogSynthDetuneStage) SetRate(rate float64) {
    if math.IsNaN(rate) || math.IsInf(rate, 0) || rate <= 0.0 {
        d.Rate = 0.85
        return
    }
    d.Rate = rate
}

// SetCenterDelayMs updates the nominal delay offset in milliseconds. Negative
// or non-finite values default to 11.0 ms.
func (d *AnalogSynthDetuneStage) SetCenterDelayMs(delayMs float64) {
    if math.IsNaN(delayMs) || math.IsInf(delayMs, 0) || delayMs < 0.0 {
        d.CenterDelayMs = 11.0
        return
    }
    d.CenterDelayMs = delayMs
}

// SetAnalogDrift updates the organic analog drift amount, clamped to [0.0, 1.0].
func (d *AnalogSynthDetuneStage) SetAnalogDrift(drift float64) {
    d.AnalogDrift = clamp01(drift)
}

// SetAnalogDriftDepth updates the analog drift delay depth in seconds. Negative
// or non-finite values default to 0.0022.
func (d *AnalogSynthDetuneStage) SetAnalogDriftDepth(depth float64) {
    if math.IsNaN(depth) || math.IsInf(depth, 0) || depth < 0.0 {
        d.AnalogDriftDepth = defaultAnalogDriftDepth
        return
    }
    d.AnalogDriftDepth = depth
}

// SetSubOctave updates the sub-octave reinforcement amount, clamped to [0.0, 1.0].
func (d *AnalogSynthDetuneStage) SetSubOctave(subOctave float64) {
    d.SubOctave = clamp01(subOctave)
}

// Name returns a human-readable label identifying the analog synth detune stage.
func (d *AnalogSynthDetuneStage) Name() string {
    voices := d.Voices
    if voices <= 0 {
        voices = 7
    } else if voices > 16 {
        voices = 16
    }
    return fmt.Sprintf(
        "Analog Synth Detune (%dV, detune=%.2f, drift=%.2f, sub=%.2f, mix=%.2f)",
        voices,
        clamp01(d.Detune),
        clamp01(d.AnalogDrift),
        clamp01(d.SubOctave),
        clamp01(d.Mix),
    )
}

func (d *AnalogSynthDetuneStage) shapePos(normPos float64) float64 {
    p := clamp01(normPos)
    var base float64
    switch d.DetuneCurve {
    case DetuneCurveExponential:
        base = p * p
    case DetuneCurveSCurve:
        base = p * p * (3.0 - 2.0*p)
    case DetuneCurveLinear:
        fallthrough
    default:
        base = p
    }
    return 0.65*base + 0.35*math.Pow(p, 2.3)
}

func (d *AnalogSynthDetuneStage) resolvedOffsets(voices int) []float64 {
    offsets := make([]float64, voices)
    for v := 0; v < voices; v++ {
        if v < len(d.VoiceOffsets) {
            val := d.VoiceOffsets[v]
            if val < -1.0 {
                val = -1.0
            } else if val > 1.0 {
                val = 1.0
            }
            offsets[v] = val
        } else if voices == 1 {
            offsets[v] = 0.0
        } else {
            offsets[v] = (float64(v)/float64(voices-1))*2.0 - 1.0
        }
    }
    return offsets
}

// DetuneBuffer processes mono samples in place at sampleRate Hz.
func (d *AnalogSynthDetuneStage) DetuneBuffer(samples []float64, sampleRate float64) {
    if len(samples) == 0 || sampleRate <= 0.0 {
        return
    }

    interleaved := ConvertSamplesMonoToInterleavedStereo(samples)
    d.DetuneBufferStereo(interleaved, sampleRate)

    for i := range samples {
        samples[i] = 0.5 * (interleaved[i*2] + interleaved[i*2+1])
    }
}

// DetuneBufferStereo processes interleaved stereo samples ([L0, R0, L1, R1, ...])
// in place at sampleRate Hz using configured voices and offsets with
// StereoWidth panning, optional AnalogDrift, and optional SubOctave. If
// len(interleavedSamples) is odd, the trailing partial sample is left untouched.
func (d *AnalogSynthDetuneStage) DetuneBufferStereo(interleavedSamples []float64, sampleRate float64) {
    frames := len(interleavedSamples) / 2
    if frames == 0 || sampleRate <= 0.0 {
        return
    }

    mix := clamp01(d.Mix)
    if mix == 0.0 {
        return
    }

    voices := d.Voices
    if voices <= 0 {
        voices = 7
    } else if voices > 16 {
        voices = 16
    }

    offsets := d.resolvedOffsets(voices)
    detune := clamp01(d.Detune)
    width := clamp01(d.StereoWidth)
    drift := clamp01(d.AnalogDrift)
    driftDepth := d.AnalogDriftDepth
    if math.IsNaN(driftDepth) || math.IsInf(driftDepth, 0) || driftDepth < 0.0 {
        driftDepth = defaultAnalogDriftDepth
    }
    subOctave := clamp01(d.SubOctave)

    rate := d.Rate
    if rate <= 0.0 {
        rate = 0.85
    }
    centerMs := d.CenterDelayMs
    if centerMs < 0.0 {
        centerMs = 11.0
    }

    dryL := make([]float64, frames)
    dryR := make([]float64, frames)
    for i := 0; i < frames; i++ {
        dryL[i] = interleavedSamples[i*2]
        dryR[i] = interleavedSamples[i*2+1]
    }

    centerSamples := (centerMs / 1000.0) * sampleRate
    maxSpreadSamples := 0.009 * sampleRate * detune
    lfoDepthSamples := 0.0015 * sampleRate * (0.25 + 0.75*detune)
    driftDepthSamples := driftDepth * sampleRate * drift
    subGain := 0.45 * subOctave

    voicePhase := make([]float64, voices)
    driftPhase := make([]float64, voices)
    centerIdx := voices / 2
    invVoices := 1.0 / float64(voices)

    for i := 0; i < frames; i++ {
        wetLSum := 0.0
        wetRSum := 0.0

        for v := 0; v < voices; v++ {
            offset := offsets[v]
            absPos := math.Abs(offset)
            sign := 0.0
            if offset < 0.0 {
                sign = -1.0
            } else if offset > 0.0 {
                sign = 1.0
            }

            shaped := d.shapePos(absPos)
            voiceRate := rate * (0.75 + 0.5*absPos)
            voicePhase[v] += voiceRate / sampleRate
            lfo := math.Sin(2.0*math.Pi*voicePhase[v] + float64(v)*0.9)

            var driftOffset float64
            if drift > 0.0 {
                driftRate := rate * (0.11 + 0.037*float64(v))
                driftPhase[v] += driftRate / sampleRate
                p := 2.0*math.Pi*driftPhase[v] + float64(v)*1.7
                driftOffset = (0.75*math.Sin(p) + 0.25*math.Sin(2.0*p+1.3)) * driftDepthSamples
            }

            delaySamples := centerSamples + sign*shaped*maxSpreadSamples + lfo*lfoDepthSamples + driftOffset
            if delaySamples < 0.5 {
                delaySamples = 0.5
            }

            tapL := readInterpolatedTap(dryL, float64(i)-delaySamples)
            tapR := readInterpolatedTap(dryR, float64(i)-delaySamples)

            if subGain > 0.0 {
                subDelaySamples := 2.0 * delaySamples
                if subDelaySamples < 1.0 {
                    subDelaySamples = 1.0
                }
                subTapL := readInterpolatedTap(dryL, float64(i)-subDelaySamples)
                subTapR := readInterpolatedTap(dryR, float64(i)-subDelaySamples)
                tapL += subTapL * subGain
                tapR += subTapR * subGain
            }

            pan := offset * width
            angle := (pan + 1.0) * 0.25 * math.Pi
            gainL := math.Cos(angle) * math.Sqrt2
            gainR := math.Sin(angle) * math.Sqrt2

            centerBoost := 1.0
            if v == centerIdx && voices%2 == 1 {
                centerBoost = 1.15
            }

            wetLSum += tapL * gainL * centerBoost
            wetRSum += tapR * gainR * centerBoost
        }

        wetL := wetLSum * invVoices
        wetR := wetRSum * invVoices

        outL := dryL[i]*(1.0-mix) + wetL*mix
        outR := dryR[i]*(1.0-mix) + wetR*mix

        interleavedSamples[i*2] = math.Max(-1.0, math.Min(1.0, outL))
        interleavedSamples[i*2+1] = math.Max(-1.0, math.Min(1.0, outR))
    }
}

// NewSubOctaveAnalogSynthDetuneStage constructs a 4-voice detune stage with
// sub-octave reinforcement, narrow-to-moderate stereo width, and tight delay
// for heavy sub-anchored synth stacks.
func NewSubOctaveAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    stage := NewAnalogSynthDetuneStage(
        4,
        []float64{-0.85, -0.30, 0.30, 0.85},
        0.24,
        DetuneCurveSCurve,
        0.55,
        0.45,
        0.70,
        9.0,
        0.6,
    )
    stage.SetSubOctave(0.75)
    return stage
}

// NewAnalogDriftAnalogSynthDetuneStage constructs a 6-voice detune stage
// emphasizing slow organic analog drift for warm, living pitch wander.
func NewAnalogDriftAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    stage := NewAnalogSynthDetuneStage(
        6,
        []float64{-1.0, -0.6, -0.2, 0.2, 0.6, 1.0},
        0.36,
        DetuneCurveSCurve,
        0.56,
        0.78,
        0.40,
        14.5,
        0,
    )
    stage.SetAnalogDrift(0.75)
    return stage
}

// NewTightLeadAnalogSynthDetuneStage constructs a focused 5-voice detune
// stage tuned for cutting leads and plucks with symmetric offsets, moderate
// spread, and controlled stereo width.
func NewTightLeadAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        5,
        []float64{-1.0, -0.5, 0.0, 0.5, 1.0},
        0.25,
        DetuneCurveSCurve,
        0.45,
        0.65,
        0.95,
        9.5,
        0,
    )
}

// NewClassicSupersawAnalogSynthDetuneStage constructs a 7-voice detune stage
// modeled on classic Supersaw voicing with exponential spread, prominent
// detune stack, and wide stereo imaging.
func NewClassicSupersawAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        7,
        []float64{-1.0, -0.55, -0.22, 0.0, 0.22, 0.55, 1.0},
        0.68,
        DetuneCurveExponential,
        0.60,
        0.85,
        0.85,
        11.0,
        0,
    )
}

// NewWidePadAnalogSynthDetuneStage constructs a lush 9-voice detune stage
// with slow LFO drift, generous spread, and broad stereo width suited to pads.
func NewWidePadAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        9,
        nil,
        0.78,
        DetuneCurveExponential,
        0.70,
        0.95,
        0.55,
        13.5,
        0,
    )
}

// NewFatBassAnalogSynthDetuneStage constructs a 3-voice detune stage with
// narrow spread, low stereo width, and subtle mix to keep low end punchy and
// mono-compatible.
func NewFatBassAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        3,
        []float64{-0.65, 0.0, 0.65},
        0.14,
        DetuneCurveLinear,
        0.32,
        0.25,
        0.65,
        7.5,
        0,
    )
}

// NewSubtleEnsembleAnalogSynthDetuneStage constructs a 4-voice gentle detune
// stage for delicate string or electric-piano thickening.
func NewSubtleEnsembleAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        4,
        []float64{-0.85, -0.30, 0.30, 0.85},
        0.20,
        DetuneCurveSCurve,
        0.38,
        0.72,
        0.70,
        12.0,
        0,
    )
}

// NewTranceHypersawAnalogSynthDetuneStage constructs an 11-voice high-spread
// detune stage for dense, soaring electronic leads and anthems.
func NewTranceHypersawAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        11,
        nil,
        0.86,
        DetuneCurveExponential,
        0.72,
        0.92,
        1.15,
        11.5,
        0,
    )
}

// NewLushStringPadAnalogSynthDetuneStage constructs an 8-voice warm pad stage
// with smooth S-curve spread and slow modulation.
func NewLushStringPadAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        8,
        nil,
        0.58,
        DetuneCurveSCurve,
        0.68,
        0.88,
        0.45,
        14.0,
        0,
    )
}

// NewPunchyPluckAnalogSynthDetuneStage constructs a 4-voice crisp pluck stage
// with fast subtle modulation and tight delay timing.
func NewPunchyPluckAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        4,
        []float64{-0.9, -0.35, 0.35, 0.9},
        0.22,
        DetuneCurveExponential,
        0.42,
        0.60,
        1.30,
        8.0,
        0,
    )
}

// NewDetunedBrassAnalogSynthDetuneStage constructs a 6-voice brassy stack
// with linear spread and medium-wide stereo placement.
func NewDetunedBrassAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        6,
        []float64{-1.0, -0.6, -0.2, 0.2, 0.6, 1.0},
        0.40,
        DetuneCurveLinear,
        0.52,
        0.78,
        0.80,
        10.5,
        0,
    )
}

// NewVintageDriftAnalogSynthDetuneStage constructs a 5-voice analog-drift
// stage with slow LFO rate and slightly longer delay for warm instability.
func NewVintageDriftAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    stage := NewAnalogSynthDetuneStage(
        5,
        []float64{-0.8, -0.4, 0.0, 0.4, 0.8},
        0.34,
        DetuneCurveSCurve,
        0.55,
        0.75,
        0.35,
        15.0,
        0,
    )
    stage.SetAnalogDrift(0.45)
    return stage
}

// NewWideStereoChorusStackAnalogSynthDetuneStage constructs a 6-voice wide
// stereo stack emphasizing L/R separation and balanced detune.
func NewWideStereoChorusStackAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        6,
        []float64{-1.0, -0.65, -0.25, 0.25, 0.65, 1.0},
        0.48,
        DetuneCurveSCurve,
        0.58,
        1.00,
        0.90,
        12.5,
        0,
    )
}

// NewMassiveWallAnalogSynthDetuneStage constructs a 16-voice maximum-density
// stack for full-spectrum walls of sound.
func NewMassiveWallAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        16,
        nil,
        0.92,
        DetuneCurveExponential,
        0.80,
        1.00,
        1.05,
        13.0,
        0,
    )
}

// NewCinematicDroneAnalogSynthDetuneStage constructs a 12-voice evolving
// detune stage with slow modulation and long delay for dark or ambient drones.
func NewCinematicDroneAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    stage := NewAnalogSynthDetuneStage(
        12,
        nil,
        0.74,
        DetuneCurveSCurve,
        0.76,
        0.90,
        0.28,
        16.5,
        0,
    )
    stage.SetAnalogDrift(0.35)
    return stage
}

// NewArpSparkleAnalogSynthDetuneStage constructs a 5-voice bright detune stage
// with lively modulation and crisp delay for arpeggios and sequences.
func NewArpSparkleAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        5,
        []float64{-1.0, -0.5, 0.0, 0.5, 1.0},
        0.32,
        DetuneCurveLinear,
        0.48,
        0.76,
        1.45,
        8.5,
        0,
    )
}

// NewMonoCenterStackAnalogSynthDetuneStage constructs a 5-voice centered stack
// with zero stereo width for mono-summed leads and sound effects.
func NewMonoCenterStackAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        5,
        []float64{-0.9, -0.45, 0.0, 0.45, 0.9},
        0.42,
        DetuneCurveSCurve,
        0.55,
        0.00,
        0.80,
        10.0,
        0,
    )
}

// NewLoFiTapeWobbleAnalogSynthDetuneStage constructs a 3-voice warm detune
// stage with slow drift and moderate delay for vintage tape character.
func NewLoFiTapeWobbleAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    stage := NewAnalogSynthDetuneStage(
        3,
        []float64{-0.75, 0.0, 0.75},
        0.28,
        DetuneCurveSCurve,
        0.50,
        0.55,
        0.40,
        15.5,
        0,
    )
    stage.SetAnalogDrift(0.60)
    return stage
}

// NewRazorSawLeadAnalogSynthDetuneStage constructs a 7-voice aggressive lead
// stage with high detune and tight delay for cutting synth hooks.
func NewRazorSawLeadAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        7,
        []float64{-1.0, -0.6, -0.25, 0.0, 0.25, 0.6, 1.0},
        0.76,
        DetuneCurveLinear,
        0.64,
        0.82,
        1.25,
        7.0,
        0,
    )
}

// NewGlassBellPadAnalogSynthDetuneStage constructs a 6-voice airy detune stage
// with gentle spread and broad stereo image for crystalline chords.
func NewGlassBellPadAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        6,
        []float64{-0.95, -0.5, -0.15, 0.15, 0.5, 0.95},
        0.24,
        DetuneCurveExponential,
        0.54,
        0.92,
        0.60,
        13.0,
        0,
    )
}

// NewSubBassGrowlAnalogSynthDetuneStage constructs a 2-voice tight detune
// stage for subtle low-mid growl without washing out sub weight.
func NewSubBassGrowlAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        2,
        []float64{-0.5, 0.5},
        0.10,
        DetuneCurveLinear,
        0.25,
        0.15,
        0.50,
        6.5,
        0,
    )
}

// NewSynthwaveChordsAnalogSynthDetuneStage constructs an 8-voice nostalgic
// chord voicing with balanced detune and warm stereo width.
func NewSynthwaveChordsAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        8,
        nil,
        0.46,
        DetuneCurveSCurve,
        0.58,
        0.84,
        0.75,
        12.0,
        0,
    )
}

// NewHyperSawAnthemAnalogSynthDetuneStage constructs a 14-voice wide-spread
// detune stage for big festival leads and chords.
func NewHyperSawAnthemAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        14,
        nil,
        0.90,
        DetuneCurveExponential,
        0.78,
        0.98,
        1.20,
        12.0,
        0,
    )
}

// NewIntimateVocalPadAnalogSynthDetuneStage constructs a 5-voice close-knit
// detune stage with low spread for breathy, intimate synth textures.
func NewIntimateVocalPadAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        5,
        []float64{-0.7, -0.3, 0.0, 0.3, 0.7},
        0.18,
        DetuneCurveSCurve,
        0.48,
        0.68,
        0.50,
        14.5,
        0,
    )
}

// NewPercussiveStabAnalogSynthDetuneStage constructs a 4-voice snappy detune
// stage with short delay and brisk modulation for stabs and hits.
func NewPercussiveStabAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    return NewAnalogSynthDetuneStage(
        4,
        []float64{-1.0, -0.4, 0.4, 1.0},
        0.30,
        DetuneCurveLinear,
        0.46,
        0.70,
        1.50,
        6.0,
        0,
    )
}

// NewAmbientShimmerWashAnalogSynthDetuneStage constructs a 10-voice expansive
// wash with high mix and wide stereo spread for atmospheric beds.
func NewAmbientShimmerWashAnalogSynthDetuneStage() *AnalogSynthDetuneStage {
    stage := NewAnalogSynthDetuneStage(
        10,
        nil,
        0.82,
        DetuneCurveSCurve,
        0.85,
        1.00,
        0.32,
        17.0,
        0,
    )
    stage.SetAnalogDrift(0.40)
    return stage
}

// AnalogSynthDetunePresetFactory constructs a fresh *AnalogSynthDetuneStage
// instance for a named preset.
type AnalogSynthDetunePresetFactory func() *AnalogSynthDetuneStage

var analogSynthDetunePresetFactories = map[string]AnalogSynthDetunePresetFactory{
    "sub-octave":               NewSubOctaveAnalogSynthDetuneStage,
    "analog-drift":             NewAnalogDriftAnalogSynthDetuneStage,
    "tight-lead":               NewTightLeadAnalogSynthDetuneStage,
    "classic-supersaw":         NewClassicSupersawAnalogSynthDetuneStage,
    "wide-pad":                 NewWidePadAnalogSynthDetuneStage,
    "fat-bass":                 NewFatBassAnalogSynthDetuneStage,
    "subtle-ensemble":          NewSubtleEnsembleAnalogSynthDetuneStage,
    "trance-hypersaw":          NewTranceHypersawAnalogSynthDetuneStage,
    "lush-string-pad":          NewLushStringPadAnalogSynthDetuneStage,
    "punchy-pluck":             NewPunchyPluckAnalogSynthDetuneStage,
    "detuned-brass":            NewDetunedBrassAnalogSynthDetuneStage,
    "vintage-drift":            NewVintageDriftAnalogSynthDetuneStage,
    "wide-stereo-chorus-stack": NewWideStereoChorusStackAnalogSynthDetuneStage,
    "massive-wall":             NewMassiveWallAnalogSynthDetuneStage,
    "cinematic-drone":          NewCinematicDroneAnalogSynthDetuneStage,
    "arp-sparkle":              NewArpSparkleAnalogSynthDetuneStage,
    "mono-center-stack":        NewMonoCenterStackAnalogSynthDetuneStage,
    "lo-fi-tape-wobble":        NewLoFiTapeWobbleAnalogSynthDetuneStage,
    "razor-saw-lead":           NewRazorSawLeadAnalogSynthDetuneStage,
    "glass-bell-pad":           NewGlassBellPadAnalogSynthDetuneStage,
    "sub-bass-growl":           NewSubBassGrowlAnalogSynthDetuneStage,
    "synthwave-chords":         NewSynthwaveChordsAnalogSynthDetuneStage,
    "hyper-saw-anthem":         NewHyperSawAnthemAnalogSynthDetuneStage,
    "intimate-vocal-pad":       NewIntimateVocalPadAnalogSynthDetuneStage,
    "percussive-stab":          NewPercussiveStabAnalogSynthDetuneStage,
    "ambient-shimmer-wash":     NewAmbientShimmerWashAnalogSynthDetuneStage,
}

// AnalogSynthDetunePresetNames returns all registered preset keys sorted
// lexicographically.
func AnalogSynthDetunePresetNames() []string {
    names := make([]string, 0, len(analogSynthDetunePresetFactories))
    for name := range analogSynthDetunePresetFactories {
        names = append(names, name)
    }
    slices.Sort(names)
    return names
}

// HasAnalogSynthDetunePreset reports whether name is registered in the preset
// registry (case-insensitive, trimmed).
func HasAnalogSynthDetunePreset(name string) bool {
    key := strings.ToLower(strings.TrimSpace(name))
    _, ok := analogSynthDetunePresetFactories[key]
    return ok
}

// LookupAnalogSynthDetunePreset returns a new *AnalogSynthDetuneStage for the
// given preset name (case-insensitive, trimmed) and whether it was found.
func LookupAnalogSynthDetunePreset(name string) (*AnalogSynthDetuneStage, bool) {
    key := strings.ToLower(strings.TrimSpace(name))
    factory, ok := analogSynthDetunePresetFactories[key]
    if !ok {
        return nil, false
    }
    return factory(), true
}

// NewAnalogSynthDetunePreset returns a *AnalogSynthDetuneStage for name, or
// falls back to NewDefaultAnalogSynthDetuneStage(0.5, DetuneCurveLinear, 0.5, 0.75)
// when no matching preset exists.
func NewAnalogSynthDetunePreset(name string) *AnalogSynthDetuneStage {
    if stage, ok := LookupAnalogSynthDetunePreset(name); ok {
        return stage
    }
    return NewDefaultAnalogSynthDetuneStage(0.5, DetuneCurveLinear, 0.5, 0.75)
}

// readInterpolatedTap reads a sample from buf at fractional index pos using
// linear interpolation, clamping out-of-range positions to the buffer ends.
func readInterpolatedTap(buf []float64, pos float64) float64 {
    n := len(buf)
    if n == 0 || math.IsNaN(pos) || math.IsInf(pos, 0) {
        return 0.0
    }
    if pos <= 0.0 {
        return buf[0]
    }
    maxIdx := n - 1
    if pos >= float64(maxIdx) {
        return buf[maxIdx]
    }

    idx := int(math.Floor(pos))
    if idx < 0 {
        return buf[0]
    }
    if idx >= maxIdx {
        return buf[maxIdx]
    }

    frac := pos - float64(idx)
    return buf[idx]*(1.0-frac) + buf[idx+1]*frac
}
