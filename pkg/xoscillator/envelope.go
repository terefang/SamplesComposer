package xoscillator

// Envelope shapes audio amplitude over time both during live sample generation
// and as a post-processor over existing sample buffers.
type Envelope interface {
    // ApplyBuffer post-processes an existing sample slice in place at
    // sampleRate Hz.
    ApplyBuffer(samples []float64, sampleRate float64)

    // GetCycleWidth returns the minimum duration in seconds required for a full
    // envelope cycle.
    GetCycleWidth() float64

}
