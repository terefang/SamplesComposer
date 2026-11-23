package xoscillator

import (
    "fmt"
    "math"
)

// SequenceEnvelope composes multiple Envelope implementations and applies them
// one after another in order to a sample buffer.
type SequenceEnvelope struct {
    Envelopes []Envelope
}

func (s *SequenceEnvelope) GetCycleWidth() float64 {
    return 0.0
}

var _ Envelope = (*SequenceEnvelope)(nil)

// NewSequenceEnvelope constructs a SequenceEnvelope.
func NewSequenceEnvelope() *SequenceEnvelope {
    seq := &SequenceEnvelope{}
    return seq
}

// NewSequenceEnvelopes constructs a SequenceEnvelope from the given envelopes.
func NewSequenceEnvelopes(envelopes ...Envelope) *SequenceEnvelope {
    seq := &SequenceEnvelope{}
    seq.SetEnvelopes(envelopes)
    return seq
}

// SetEnvelopes replaces the chained envelopes with a defensive copy of
// envelopes.
func (s *SequenceEnvelope) SetEnvelopes(envelopes []Envelope) {
    if len(envelopes) == 0 {
        s.Envelopes = nil
        return
    }
    copied := make([]Envelope, len(envelopes))
    copy(copied, envelopes)
    s.Envelopes = copied
}

// AddEnvelope appends an Envelope to the end of the sequence.
func (s *SequenceEnvelope) AddEnvelope(env Envelope) {
    if env == nil {
        return
    }
    s.Envelopes = append(s.Envelopes, env)
}

// Name returns a human-readable identifier for the chained envelope.
func (s *SequenceEnvelope) Name() string {
    return fmt.Sprintf("SequenceEnvelope(count=%d)", len(s.Envelopes))
}

// ApplyBuffer applies each non-nil envelope in Envelopes sequentially in-place
// to samples at sampleRate Hz.
func (s *SequenceEnvelope) ApplyBuffer(samples []float64, sampleRate float64) {
    if len(samples) == 0 || sampleRate <= 0.0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
        return
    }
    for _, env := range s.Envelopes {
        if env != nil {
            env.ApplyBuffer(samples, sampleRate)
        }
    }
}
