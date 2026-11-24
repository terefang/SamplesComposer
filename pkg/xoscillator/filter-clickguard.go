package xoscillator

import (
    "fmt"
    "math"
)

const (
    defaultEndClickFadeMs   = 8.0
    defaultStartClickFadeMs = 0.0
)

// ClickGuardFilter applies smooth raised-cosine fades at the start and/or
// end of a sample buffer to prevent clicks and pops at buffer boundaries.
type ClickGuardFilter struct {
    // EndFadeMs sets the fade-out duration at the tail of the buffer in
    // milliseconds (defaults to 8.0 ms when <= 0 or non-finite).
    EndFadeMs float64

    // StartFadeMs sets the fade-in duration at the start of the buffer in
    // milliseconds (0.0 disables start fade; negative or non-finite values
    // default to 0.0).
    StartFadeMs float64
}

var _ Filter = (*ClickGuardFilter)(nil)

// NewEndClickGuardFilter constructs an ClickGuardFilter with the specified
// tail fade duration in milliseconds. Non-positive or non-finite values default
// to 8.0 ms.
func NewEndClickGuardFilter(fadeMs float64) *ClickGuardFilter {
    f := &ClickGuardFilter{}
    f.SetEndFadeMs(fadeMs)
    f.SetStartFadeMs(defaultStartClickFadeMs)
    return f
}

// NewBoundaryClickGuardFilter constructs an ClickGuardFilter configured with
// both start fade-in and end fade-out durations in milliseconds.
func NewBoundaryClickGuardFilter(fadeMs, startFadeMs float64) *ClickGuardFilter {
    f := &ClickGuardFilter{}
    f.SetEndFadeMs(fadeMs)
    f.SetStartFadeMs(startFadeMs)
    return f
}

// SetEndFadeMs updates the tail fade duration in milliseconds. Non-positive or
// non-finite values default to 8.0 ms.
func (f *ClickGuardFilter) SetEndFadeMs(fadeMs float64) {
    if f == nil {
        return
    }
    if math.IsNaN(fadeMs) || math.IsInf(fadeMs, 0) || fadeMs <= 0.0 {
        f.EndFadeMs = defaultEndClickFadeMs
        return
    }
    f.EndFadeMs = fadeMs
}

// SetStartFadeMs updates the head fade-in duration in milliseconds. Negative or
// non-finite values default to 0.0 (disabled).
func (f *ClickGuardFilter) SetStartFadeMs(startFadeMs float64) {
    if f == nil {
        return
    }
    if math.IsNaN(startFadeMs) || math.IsInf(startFadeMs, 0) || startFadeMs < 0.0 {
        f.StartFadeMs = defaultStartClickFadeMs
        return
    }
    f.StartFadeMs = startFadeMs
}

// Name returns a human-readable identifier for the filter.
func (f *ClickGuardFilter) Name() string {
    endMs := defaultEndClickFadeMs
    if f != nil && !math.IsNaN(f.EndFadeMs) && !math.IsInf(f.EndFadeMs, 0) && f.EndFadeMs > 0.0 {
        endMs = f.EndFadeMs
    }

    startMs := defaultStartClickFadeMs
    if f != nil && !math.IsNaN(f.StartFadeMs) && !math.IsInf(f.StartFadeMs, 0) && f.StartFadeMs > 0.0 {
        startMs = f.StartFadeMs
    }

    if startMs > 0.0 {
        return fmt.Sprintf("EndClickGuard(start=%.1f ms, end=%.1f ms)", startMs, endMs)
    }
    return fmt.Sprintf("EndClickGuard(%.1f ms)", endMs)
}

// ProcessBuffer applies smooth fade-in (when StartFadeMs > 0) and fade-out at
// the buffer boundaries in place at sampleRate Hz.
func (f *ClickGuardFilter) ProcessBuffer(samples []float64, sampleRate float64) {
    n := len(samples)
    if f == nil || n == 0 || sampleRate <= 0.0 || math.IsNaN(sampleRate) || math.IsInf(sampleRate, 0) {
        return
    }

    endMs := f.EndFadeMs
    if math.IsNaN(endMs) || math.IsInf(endMs, 0) || endMs <= 0.0 {
        endMs = defaultEndClickFadeMs
    }

    startMs := f.StartFadeMs
    if math.IsNaN(startMs) || math.IsInf(startMs, 0) || startMs < 0.0 {
        startMs = defaultStartClickFadeMs
    }

    endSamples := int(math.Round((endMs / 1000.0) * sampleRate))
    if endSamples <= 0 {
        endSamples = 1
    }

    startSamples := 0
    if startMs > 0.0 {
        startSamples = int(math.Round((startMs / 1000.0) * sampleRate))
        if startSamples <= 0 {
            startSamples = 1
        }
    }

    totalRequested := startSamples + endSamples
    if totalRequested > n {
        scale := float64(n) / float64(totalRequested)
        if startSamples > 0 {
            startSamples = int(math.Floor(float64(startSamples) * scale))
            if startSamples == 0 && startMs > 0.0 {
                startSamples = 1
            }
        }
        endSamples = int(math.Floor(float64(endSamples) * scale))
        if endSamples == 0 {
            endSamples = 1
        }
        if startSamples+endSamples > n {
            if startSamples >= n {
                startSamples = n
                endSamples = 0
            } else {
                endSamples = n - startSamples
            }
        }
    }

    if startSamples > 0 {
        if startSamples == 1 {
            samples[0] = 0.0
        } else {
            denom := float64(startSamples - 1)
            for i := 0; i < startSamples; i++ {
                progress := float64(i) / denom
                gain := 0.5 * (1.0 - math.Cos(math.Pi*progress))
                samples[i] *= gain
            }
        }
    }

    if endSamples > 0 {
        startIdx := n - endSamples
        if endSamples == 1 {
            samples[n-1] = 0.0
        } else {
            denom := float64(endSamples - 1)
            for i := 0; i < endSamples; i++ {
                progress := float64(i) / denom
                gain := 0.5 * (1.0 + math.Cos(math.Pi*progress))
                samples[startIdx+i] *= gain
            }
        }
    }
}
