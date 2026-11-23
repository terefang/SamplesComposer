package xoscillator

import (
    "fmt"
    "math"
)

// DBGainProcessor adjusts the amplitude/loudness of a sample buffer in-place
// by a decibel offset (positive boosts, negative attenuates, 0.0 is unity).
type DBGainProcessor struct {
    // GainDB is the gain adjustment in decibels.
    GainDB float64
}

// NewDBGainProcessor constructs a DBGainProcessor with normalized GainDB.
func NewDBGainProcessor(gainDB float64) *DBGainProcessor {
    p := &DBGainProcessor{}
    p.SetGainDB(gainDB)
    return p
}

// SetGainDB updates the gain offset in decibels. NaN or +Inf values default
// to 0.0 dB; -Inf is preserved so samples attenuate to 0.0.
func (p *DBGainProcessor) SetGainDB(gainDB float64) {
    if math.IsNaN(gainDB) || math.IsInf(gainDB, 1) {
        p.GainDB = 0.0
        return
    }
    p.GainDB = gainDB
}

// LinearGain returns the linear amplitude multiplier corresponding to GainDB.
func (p *DBGainProcessor) LinearGain() float64 {
    db := p.GainDB
    if math.IsNaN(db) || math.IsInf(db, 1) {
        return 1.0
    }
    if math.IsInf(db, -1) {
        return 0.0
    }
    return math.Pow(10.0, db/20.0)
}

// Name returns a human-readable identifier for the processor.
func (p *DBGainProcessor) Name() string {
    return fmt.Sprintf("dB Gain (%+.2f dB)", p.GainDB)
}

// ProcessBuffer scales samples in-place by LinearGain().
func (p *DBGainProcessor) ProcessBuffer(samples []float64) {
    if len(samples) == 0 {
        return
    }
    gain := p.LinearGain()
    if gain == 1.0 {
        return
    }
    for i := range samples {
        samples[i] *= gain
    }
}
