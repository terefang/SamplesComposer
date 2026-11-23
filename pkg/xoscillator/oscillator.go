package xoscillator

import "math"

// Oscillator abstracts a SID-style audio oscillator capable of generating
// synthesized mono and interleaved stereo samples across offline buffer
// rendering, as well as updating runtime synthesis and hard-sync parameters.
//
// Implementations typically combine a Waveform source with frequency, sync
// frequency, phase, pulse width, envelope, filter, or detune state to produce
// bipolar audio samples in [-1.0, 1.0]. Callers should not assume concurrency
// safety unless documented by a concrete implementation.
type Oscillator interface {
    // RenderSamples renders durationSec seconds of mono audio at sampleRate Hz
    // into a newly allocated slice of bipolar samples in [-1.0, 1.0].
    //
    // If sampleRate is less than or equal to 0 or durationSec is less than or
    // equal to 0, RenderSamples returns nil without allocating a buffer.
    RenderSamples(sampleRate float64, durationSec float64) []float64

    // RenderStereoSamples renders durationSec seconds of interleaved stereo
    // audio ([L0, R0, L1, R1, ...]) at sampleRate Hz into a newly allocated
    // slice of bipolar samples in [-1.0, 1.0].
    //
    // If sampleRate is less than or equal to 0 or durationSec is less than or
    // equal to 0, RenderStereoSamples returns nil without allocating a buffer.
    RenderStereoSamples(sampleRate float64, durationSec float64) []float64

    // SetFreq updates the target oscillation frequency in Hertz.
    SetFreq(freq float64)

    // SetSyncFreq updates the master hard-sync cycle frequency in Hertz.
    SetSyncFreq(freq float64)

    // SetWaveform updates the active Waveform generator (may be nil).
    SetWaveform(wave Waveform)

    // SetPulseWidth updates the pulse width parameter (typically in [0.0, 1.0]).
    SetPulseWidth(pw float64)

    // SetSeed updates the 32-bit deterministic seed forwarded to the waveform.
    SetSeed(seed int32)
}

// --- Concrete Oscillator Engine Implementation ---

// OscillatorEngine is a phase-accumulating SID-style oscillator engine that
// implements the Oscillator interface and delegates sample generation to a
// pluggable Waveform implementation, optional Envelope, optional Filter, and
// optional DetuneStage.
//
// Callers should not assume concurrency safety unless external synchronization
// is provided.
type OscillatorEngine struct {
    // Freq is the target oscillation frequency in Hertz.
    Freq float64

    // SyncFreq is the optional master hard-sync cycle frequency in Hertz. When
    // greater than 0 and HardSync is enabled, phase resets are driven at
    // SyncFreq rather than Freq.
    SyncFreq float64

    // Wave is the active waveform generator used to produce raw samples. If nil,
    // raw sample output defaults to 0.0.
    Wave Waveform

    // Filter is an optional audio filter applied in-place to the rendered sample
    // buffer after envelope shaping.
    Filter Filter

    // Envelope is an optional amplitude envelope applied in-place to the
    // rendered sample buffer prior to filtering.
    Envelope Envelope

    // Detune is an optional post-processing detune stage applied after
    // envelope and filter stages during rendering.
    Detune DetuneStage

    // HardSync resets oscillator phase to the cycle fraction whenever the
    // master sync cycle wraps across an integer boundary.
    HardSync bool

    // PulseWidth is the pulse width parameter forwarded to Wave on each sample
    // step (typically normalized to [0.0, 1.0]).
    PulseWidth float64

    // Seed is the 32-bit deterministic seed forwarded to Wave on each sample
    // step (defaults to 0x1337 in constructors).
    Seed int32
}

// SetSyncFreq updates the master hard-sync frequency in Hertz. Non-finite or
// negative frequencies are clamped to 0.0 (falling back to Freq during sync).
func (o *OscillatorEngine) SetSyncFreq(freq float64) {
    if math.IsNaN(freq) || math.IsInf(freq, 0) || freq < 0.0 {
        o.SyncFreq = 0.0
        return
    }
    o.SyncFreq = freq
}

// RenderSamples renders durationSec seconds of mono audio at sampleRate Hz into a
// newly allocated slice of float64 samples. It returns nil if sampleRate or
// durationSec is not positive, or if the rounded sample count is not positive.
func (o *OscillatorEngine) RenderSamples(sampleRate float64, durationSec float64) []float64 {
    if sampleRate <= 0 || durationSec <= 0 {
        return nil
    }

    totalSamples := int(math.Round(sampleRate * durationSec))
    if totalSamples <= 0 {
        return nil
    }

    samples := make([]float64, totalSamples)

    prevPhase := 0.0
    phase := 0.0
    modMSB := 1.0
    inc := o.Freq / sampleRate

    syncFreq := o.SyncFreq
    if syncFreq <= 0.0 {
        syncFreq = o.Freq
    }
    syncInc := syncFreq / sampleRate
    prevSyncCycle := 0.0

    for i := 0; i < totalSamples; i++ {
        syncCycle := float64(i) * syncInc
        if o.HardSync && i > 0 && syncInc > 0.0 {
            if math.Floor(syncCycle) > math.Floor(prevSyncCycle) {
                phase = syncCycle - math.Floor(syncCycle)
            }
        }
        prevSyncCycle = syncCycle

        prevPhase = phase
        phase += inc

        effectiveModMSB := modMSB

        // Approximate the SID's mid-cycle bit transition to clock LFSR noise generators.
        crossedHalf := prevPhase < 0.5 && phase >= 0.5

        out := 0.0
        if o.Wave != nil {
            o.Wave.SetSeed(o.Seed)
            o.Wave.SetPulseWidth(o.PulseWidth)
            out = o.Wave.Sample(phase, crossedHalf, effectiveModMSB)
        }
        samples[i] = out
    }
    if o.Envelope != nil {
        o.Envelope.ApplyBuffer(samples, sampleRate)
    }
    if o.Filter != nil {
        o.Filter.ProcessBuffer(samples, sampleRate)
    }
    if o.Detune != nil {
        o.Detune.DetuneBuffer(samples, sampleRate)
    }
    return samples
}

// RenderStereoSamples renders durationSec seconds of interleaved stereo audio
// ([L0, R0, L1, R1, ...]) at sampleRate Hz. When Detune is non-nil,
// DetuneBufferStereo is applied to the interleaved buffer; otherwise each mono
// sample is duplicated across left and right channels. It returns nil if
// sampleRate or durationSec is not positive.
func (o *OscillatorEngine) RenderStereoSamples(sampleRate float64, durationSec float64) []float64 {
    mono := o.RenderSamples(sampleRate, durationSec)
    if mono == nil {
        return nil
    }

    stereo := ConvertSamplesMonoToInterleavedStereo(mono)
    if o.Detune != nil {
        o.Detune.DetuneBufferStereo(stereo, sampleRate)
    }
    return stereo
}

// NewOscillatorEngine constructs an OscillatorEngine with all synthesis,
// modulation, hard-sync, and post-processing parameters explicitly specified.
func NewOscillatorEngine(
    freq float64,
    syncFreq float64,
    wave Waveform,
    filter Filter,
    envelope Envelope,
    detune DetuneStage,
    hardSync bool,
    pulseWidth float64,
    seed int32,
) *OscillatorEngine {
    engine := &OscillatorEngine{
        Wave:     wave,
        Filter:   filter,
        Envelope: envelope,
        Detune:   detune,
        HardSync: hardSync,
        Seed:     seed,
    }
    engine.SetFreq(freq)
    engine.SetSyncFreq(syncFreq)
    engine.SetPulseWidth(pulseWidth)
    return engine
}

var _ Oscillator = (*OscillatorEngine)(nil)

// SetFreq updates the target oscillation frequency in Hertz. Non-finite or
// negative frequencies are clamped to 0.0.
func (o *OscillatorEngine) SetFreq(freq float64) {
    if math.IsNaN(freq) || math.IsInf(freq, 0) || freq < 0.0 {
        o.Freq = 0.0
        return
    }
    o.Freq = freq
}

// SetWaveform updates the active Waveform generator used during rendering.
func (o *OscillatorEngine) SetWaveform(wave Waveform) {
    o.Wave = wave
}

// SetFilter updates the optional post-envelope Filter stage.
func (o *OscillatorEngine) SetFilter(filter Filter) {
    o.Filter = filter
}

// SetEnvelope updates the optional amplitude Envelope stage.
func (o *OscillatorEngine) SetEnvelope(env Envelope) {
    o.Envelope = env
}

// SetDetune updates the optional DetuneStage post-processing stage.
func (o *OscillatorEngine) SetDetune(detune DetuneStage) {
    o.Detune = detune
}

// SetPulseWidth updates the pulse width parameter clamped to [0.0, 1.0].
func (o *OscillatorEngine) SetPulseWidth(pw float64) {
    o.PulseWidth = clamp01(pw)
}

// SetSeed updates the 32-bit deterministic seed forwarded to Wave.
func (o *OscillatorEngine) SetSeed(seed int32) {
    o.Seed = seed
}

// SetHardSync updates whether hard-sync phase reset behavior is enabled.
func (o *OscillatorEngine) SetHardSync(enabled bool) {
    o.HardSync = enabled
}

// ConvertSamplesFtoI2 converts a slice of normalized float64 audio samples into
// an interleaved two-channel (stereo) []int16 buffer scaled by math.MaxInt16,
// duplicating each mono sample into both left and right channels.
func ConvertSamplesFtoI2(samples []float64) []int16 {
    sLen := len(samples)
    samplesInt := make([]int16, sLen*2)
    for i := 0; i < sLen; i++ {
        samplesInt[i*2] = int16(samples[i] * math.MaxInt16)
        samplesInt[i*2+1] = samplesInt[i*2]
    }
    return samplesInt
}

// ConvertSamplesFtoI converts a slice of normalized float64 audio samples into
// a mono []int16 buffer scaled by math.MaxInt16.
func ConvertSamplesFtoI(samples []float64) []int16 {
    sLen := len(samples)
    samplesInt := make([]int16, sLen)
    for i := 0; i < sLen; i++ {
        samplesInt[i] = int16(samples[i] * math.MaxInt16)
    }
    return samplesInt
}

// ConvertSamplesMonoToInterleavedStereo converts a mono slice of audio samples
// into a newly allocated interleaved stereo slice ([L0, R0, L1, R1, ...]) by
// duplicating each sample into both left and right channels. It returns nil if
// samples is nil.
func ConvertSamplesMonoToInterleavedStereo(samples []float64) []float64 {
    if samples == nil {
        return nil
    }
    stereo := make([]float64, len(samples)*2)
    for i, s := range samples {
        stereo[i*2] = s
        stereo[i*2+1] = s
    }
    return stereo
}
