package xoscillator

import "math"

// --- ADSR Envelope Generator ---

// ADSR implements an envelope generator with optional pre-attack delay and
// post-attack hold that can either drive an OscillatorEngine during synthesis or
// post-process existing sample slices.
type ADSR struct {
    // DelaySec is the duration in seconds to hold 0.0 after trigger before
    // attack begins.
    DelaySec float64

    // AttackSec is the attack ramp duration in seconds (0.0 jumps immediately to 1.0).
    AttackSec float64

    // HoldSec is the duration in seconds to hold peak level (1.0) after attack
    // completes and before decay begins.
    HoldSec float64

    // DecaySec is the decay ramp duration in seconds from 1.0 to SustainLevel.
    DecaySec float64

    // SustainLevel is the held amplitude multiplier in [0.0, 1.0] during sustain.
    SustainLevel float64

    // ReleaseSec is the release ramp duration in seconds back to 0.0 after Gate is released.
    ReleaseSec float64
}

// NewADSR constructs an ADSR envelope with the specified stage timings and
// sustain level (with DelaySec and HoldSec set to 0.0), starting with Gate set
// to true so it begins attacking immediately.
func NewADSR(attackSec, decaySec, sustainLevel, releaseSec float64) *ADSR {
    return NewADSRWithDelay(0.0, attackSec, 0.0, decaySec, sustainLevel, releaseSec)
}

// NewADSRWithHold constructs an ADSR envelope including a HoldSec stage right
// after attack, starting with Gate set to true.
func NewADSRWithHold(attackSec, holdSec, decaySec, sustainLevel, releaseSec float64) *ADSR {
    return NewADSRWithDelay(0.0, attackSec, holdSec, decaySec, sustainLevel, releaseSec)
}

// NewADSRWithDelay constructs an ADSR envelope including a pre-attack DelaySec
// stage and post-attack HoldSec stage, starting with Gate set to true.
func NewADSRWithDelay(delaySec, attackSec, holdSec, decaySec, sustainLevel, releaseSec float64) *ADSR {
    return &ADSR{
        DelaySec:     delaySec,
        AttackSec:    attackSec,
        HoldSec:      holdSec,
        DecaySec:     decaySec,
        SustainLevel: sustainLevel,
        ReleaseSec:   releaseSec,
    }
}

func Lerp(a, b, t float64) float64 {
    return a + (b-a)*t
}

// ApplyBuffer post-processes an existing sample slice in place at sampleRate Hz.
func (e *ADSR) ApplyBuffer(samples []float64, sampleRate float64) {
    sLen := len(samples)
    dSec := float64(sLen) / sampleRate

    if sLen == 0 || sampleRate <= 0 {
        return
    }

    for i := range samples {
        sec := float64(i) / sampleRate
        if sec < e.DelaySec {
            samples[i] = 0
        } else if sec < e.DelaySec+e.AttackSec {
            t := (sec - e.DelaySec) / e.AttackSec
            samples[i] *= Lerp(0.0, 1.0, t)
        } else if sec < e.DelaySec+e.AttackSec+e.HoldSec {
            // samples[i] *= 1.0
        } else if sec < e.DelaySec+e.AttackSec+e.HoldSec+e.DecaySec {
            t := (sec - (e.DelaySec + e.AttackSec + e.HoldSec)) / e.DecaySec
            samples[i] *= Lerp(1.0, e.SustainLevel, t)
        } else if sec < dSec-e.ReleaseSec {
            samples[i] *= e.SustainLevel
        } else {
            t := (sec - (dSec - e.ReleaseSec)) / e.ReleaseSec
            samples[i] *= Lerp(e.SustainLevel, 0, t)
        }
    }
}

// GetCycleWidth returns the minimum duration in seconds required for a full ADSR
// cycle (delay + attack + hold + decay + release) at sampleRate Hz.
func (e *ADSR) GetCycleWidth() float64 {
    width := math.Max(0.0, e.DelaySec) +
        math.Max(0.0, e.AttackSec) +
        math.Max(0.0, e.HoldSec) +
        math.Max(0.0, e.DecaySec) +
        math.Max(0.0, e.ReleaseSec)
    return width
}
