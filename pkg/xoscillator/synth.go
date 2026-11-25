package xoscillator

// SynthParams holds every parametric value as an explicit configuration field.
type SynthParams struct {
	// samplpe parameters
	Name       string
	SampleRate int     // Sample rate in Hz (8000, 16000, 32000, 48000)
	LengthMs   float64 // Total single-note duration in milliseconds (1000 to 30000)
	// basewave parameters
	Tune       float64 // Base oscillator frequency in Hz
	Seed       int32   // fractal basewave seed
	BaseWave   string  // Oscillator waveform shape (Sine, Sawtooth, SawtoothPoly, Square, SquarePoly, Triangle, FM)
	PulseWidth float64 // Square wave duty cycle (0.05 to 0.95)
	HardSync   bool    // Lock oscillator phases to master cycle
	// FM basewave parameters
	Fm1Wave  string
	Fm2Wave  string
	Tone     float64 // Tone filter / modulation index balance (0.0 to 1.0)
	ModRatio float64 // FM modulator-to-carrier frequency ratio
	ModIndex float64 // FM modulation depth
	// Detuen/oversampling stage
	DtoStage         string
	DtoRate          float64 // detune amount
	Voices           int     // Unison voice count (>= 1)
	StereoWidth      float64 // Stereo pan spread (0.0 to 1.0)
	Detune           float64 // Pitch spread across voices (0.0 to 1.0)
	DetuneCurve      string  // Voice detune distribution curve
	DetuneMix        float64 // Balance between side voices and center voice (0.0 to 1.0)
	SubOctave        float64 // Enable -1 octave sub-bass reinforcement
	AnalogDrift      float64 // LFO pitch wander per voice (0.0 to 1.0)
	AnalogDriftDepth float64
	CenterDelayMs    float64

	//ADSR stage parameters
	AsdrEnable bool
	Delay      float64
	Attack     float64 // Attack time in seconds
	Snappy     float64 // Noise transient mix and attack scaling (0.0 to 1.0)
	Hold       float64 // Hold time in seconds
	Decay      float64 // Decay time in seconds
	Sustain    float64 // Sustain amplitude level (0.0 to 1.0)
	Release    float64 // Release tail in seconds
	// Table Envelope stage parameters
	TenvEnable bool
	TenvInterp string    // interpolator function
	Tenv       []float64 // 16 equally spaced envelope modifiers [-1 .. +1]
	// post-process stage
	NormalizePct int64   // Peak normalization target in % (0 = off, 1..100 = target peak %)
	DbGain       float64 // player loudness
}
