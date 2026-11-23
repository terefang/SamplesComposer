// cmd/sampler/main.go
// Fully commented implementation of a Sample Composer and Unison synthesizer GUI in Fyne,
package main

import (
    "SamplesComposer/pkg"
    "SamplesComposer/pkg/xfyne"
    "SamplesComposer/pkg/xoscillator"
    "encoding/binary"
    "encoding/json"
    "fmt"
    "image/color"
    "math"
    "math/cmplx"
    "os"
    "path/filepath"
    "strconv"
    "strings"
    "sync"
    "time"

    "fyne.io/fyne/v2/storage"
    "github.com/gen2brain/malgo"
    "github.com/terefang/gommons/pkg/xdg"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/app"
    "fyne.io/fyne/v2/canvas"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/layout"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"

    "SamplesComposer/pkg/xaudio"
)

// customSynthTheme implements fyne.Theme to allow instant runtime switching
// between dark and light appearance modes while delegating base metrics/colors to fyne.DefaultTheme().
type customSynthTheme struct {
    dark bool
}

func (t *customSynthTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
    v := theme.VariantLight
    if t.dark {
        v = theme.VariantDark
    }
    return theme.DefaultTheme().Color(name, v)
}

func (t *customSynthTheme) Font(style fyne.TextStyle) fyne.Resource {
    return theme.DefaultTheme().Font(style)
}

func (t *customSynthTheme) Icon(name fyne.ThemeIconName) fyne.Resource {
    return theme.DefaultTheme().Icon(name)
}

func (t *customSynthTheme) Size(name fyne.ThemeSizeName) float32 {
    return theme.DefaultTheme().Size(name)
}

// autoFitAccordion wraps widget.Accordion so opening or closing an item
// immediately recalculates and fits the window size without scrollbars.
type autoFitAccordion struct {
    widget.Accordion
    onToggle func()
}

func newAutoFitAccordion(onToggle func(), title string, content fyne.CanvasObject) *autoFitAccordion {
    acc := &autoFitAccordion{onToggle: onToggle}
    item := widget.NewAccordionItem(title, content)
    acc.Items = []*widget.AccordionItem{item}
    acc.ExtendBaseWidget(acc)
    return acc
}

func (a *autoFitAccordion) Open(index int) {
    a.Accordion.Open(index)
    if a.onToggle != nil {
        fyne.Do(a.onToggle)
    }
}

func (a *autoFitAccordion) Close(index int) {
    a.Accordion.Close(index)
    if a.onToggle != nil {
        fyne.Do(a.onToggle)
    }
}

// stretchVizLayout resizes the visualization background and triggers redraws whenever
// the visualization container changes size.
type stretchVizLayout struct {
    ui *synthUI
}

func (l *stretchVizLayout) MinSize(objects []fyne.CanvasObject) fyne.Size {
    return fyne.NewSize(300, 160)
}

func (l *stretchVizLayout) Layout(objects []fyne.CanvasObject, size fyne.Size) {
    if size.Width <= 1 || size.Height <= 1 {
        return
    }
    prevW := l.ui.vizLastSize.Width
    prevH := l.ui.vizLastSize.Height
    l.ui.vizBg.Move(fyne.NewPos(0, 0))
    l.ui.vizBg.Resize(size)

    // Only re-render waveforms/bars when container size meaningfully changes to avoid layout loops.
    if math.Abs(float64(size.Width-prevW)) > 1.0 || math.Abs(float64(size.Height-prevH)) > 1.0 {
        l.ui.vizLastSize = size
        fyne.Do(func() {
            l.ui.redrawVisualization()
        })
    }
}

// SynthParams holds every parametric value as an explicit configuration field.
type SynthParams struct {
    Name             string
    SampleRate       int     // Sample rate in Hz (8000, 16000, 32000, 48000)
    LengthMs         float64 // Total single-note duration in milliseconds (1000 to 30000)
    Tune             float64 // Base oscillator frequency in Hz
    Tone             float64 // Tone filter / modulation index balance (0.0 to 1.0)
    Snappy           float64 // Noise transient mix and attack scaling (0.0 to 1.0)
    NormalizePct     int64   // Peak normalization target in % (0 = off, 1..100 = target peak %)
    BaseWave         string  // Oscillator waveform shape (Sine, Sawtooth, SawtoothPoly, Square, SquarePoly, Triangle, FM)
    Voices           int     // Unison voice count (>= 1)
    Detune           float64 // Pitch spread across voices (0.0 to 1.0)
    DetuneCurve      string  // Voice detune distribution curve
    DetuneMix        float64 // Balance between side voices and center voice (0.0 to 1.0)
    StereoWidth      float64 // Stereo pan spread (0.0 to 1.0)
    SubOctave        float64 // Enable -1 octave sub-bass reinforcement
    AnalogDrift      float64 // LFO pitch wander per voice (0.0 to 1.0)
    HardSync         bool    // Lock oscillator phases to master cycle
    PulseWidth       float64 // Square wave duty cycle (0.05 to 0.95)
    ModRatio         float64 // FM modulator-to-carrier frequency ratio
    ModIndex         float64 // FM modulation depth
    Attack           float64 // Attack time in seconds
    Decay            float64 // Decay time in seconds
    Sustain          float64 // Sustain amplitude level (0.0 to 1.0)
    Release          float64 // Release tail in seconds
    Seed             int32
    AsdrEnable       bool
    Delay            float64
    Hold             float64
    TenvEnable       bool
    TenvInterp       string
    Tenv             []float64
    DtoStage         string
    Fm1Wave          string
    Fm2Wave          string
    CenterDelayMs    float64
    DtoRate          float64
    AnalogDriftDepth float64
    DbGain           float64 // player loudness
}

// defaultSynthParams returns the initial default configuration for all synthesizer parameters.
func defaultSynthParams() SynthParams {
    return SynthParams{
        SampleRate:       48000,
        LengthMs:         1500.0,
        Tune:             65.41,
        Tone:             0.65,
        Snappy:           0.50,
        NormalizePct:     0,
        BaseWave:         "fm",
        Voices:           7,
        Detune:           0.35,
        DetuneCurve:      "linear",
        DetuneMix:        0.75,
        StereoWidth:      0.80,
        SubOctave:        0,
        AnalogDrift:      0.15,
        HardSync:         false,
        PulseWidth:       0.50,
        ModRatio:         2.0,
        ModIndex:         1.0,
        Attack:           0.01,
        Decay:            0.35,
        Sustain:          0.60,
        Release:          0.25,
        Tenv:             make([]float64, 16),
        DtoStage:         "off",
        Fm1Wave:          "sine",
        Fm2Wave:          "sine",
        CenterDelayMs:    0.,
        DtoRate:          0.,
        AnalogDriftDepth: 0.0022,
    }
}

// applySampleNormalization scales []int16 samples so the peak reaches normPct% of full scale.
// When normPct <= 0, normalization is off and samples are returned unchanged.
func applySampleNormalization(samples []int16, normPct int64) []int16 {
    if normPct <= 0 || len(samples) == 0 {
        return samples
    }
    if normPct > 100 {
        normPct = 100
    }

    var peak float64
    for _, s := range samples {
        v := math.Abs(float64(s))
        if v > peak {
            peak = v
        }
    }
    if peak <= 0 {
        return samples
    }

    targetPeak := (float64(normPct) / 101.0) * float64(math.MaxInt16)
    gain := targetPeak / peak

    out := make([]int16, len(samples))
    for i, s := range samples {
        scaled := math.Round(float64(s) * gain)
        if scaled > float64(math.MaxInt16) {
            scaled = float64(math.MaxInt16)
        } else if scaled < float64(math.MinInt16) {
            scaled = float64(math.MinInt16)
        }
        out[i] = int16(scaled)
    }
    return out
}

func lookupWave(p SynthParams, bw string, ifm bool) xoscillator.Waveform {
    effectiveModIndex := p.ModIndex + (p.Tone * 2.5)
    baseWave, ok := xoscillator.LookupWaveform(bw)
    if ok {
        return baseWave
    }
    spl, ok2 := xoscillator.LookupWaveSampler(bw, 0x1337, 2, .5, false, 2, 2.0, .5, .5, .5)
    if ok2 {
        return spl
    }

    if ifm || bw == "fm" {
        fm2 := lookupWave(p, p.Fm2Wave, false)
        fm2.SetPulseWidth(1.0)
        fm := xoscillator.NewFM2Waveform(p.ModRatio, effectiveModIndex, fm2, p.PulseWidth)
        return fm
    } else if bw == "fm2" {
        fm1 := lookupWave(p, p.Fm1Wave, false)
        fm1.SetPulseWidth(1.0)
        fm2 := lookupWave(p, p.Fm2Wave, false)
        fm2.SetPulseWidth(1.0)
        fm := xoscillator.NewFMWaveform(p.ModRatio, effectiveModIndex, fm2, fm1, false)
        fm.SetPulseWidth(p.PulseWidth)
        return fm
    }

    return xoscillator.NewPulseWaveform(.5)
}

// SynthesizeParams renders a single NoteEvent using xaudio.RenderNote.
func SynthesizeParams(p SynthParams, forSave bool) ([]int16, error) {
    effectiveMix := math.Min(1.0, math.Max(0.0, math.Pow(p.DetuneMix, 2.0)*(0.5+0.5*p.Tone)))
    effectiveAttack := math.Max(0.001, p.Attack*(1.0-0.7*p.Snappy))
    baseWave := lookupWave(p, p.BaseWave, true)
    engine := xoscillator.NewOscillatorEngine(p.Tune, 0.0, baseWave, nil, nil, nil, false, p.PulseWidth, p.Seed)

    samples := engine.RenderSamples(float64(p.SampleRate), p.LengthMs/1000.0)
    senv := xoscillator.NewSequenceEnvelope()
    if p.AsdrEnable {
        env := xoscillator.NewADSR(effectiveAttack, p.Decay, p.Sustain, p.Release)
        env.HoldSec = p.Hold
        env.DelaySec = p.Delay
        senv.AddEnvelope(env)
    }
    if p.TenvEnable {
        interp, _ := xoscillator.LookupEnvelopeInterpolator(p.TenvInterp)
        env := xoscillator.NewTableEnvelope(p.LengthMs/15000., [16]float64(p.Tenv), interp)
        senv.AddEnvelope(env)
    }
    senv.ApplyBuffer(samples, float64(p.SampleRate))
    if xoscillator.HasAnalogSynthDetunePreset(p.DtoStage) {
        ssamples := xoscillator.ConvertSamplesMonoToInterleavedStereo(samples)
        dcurve, _ := xoscillator.LookupDetuneCurve(p.DetuneCurve)
        dto, _ := xoscillator.LookupAnalogSynthDetunePreset(p.DtoStage)
        dto.SetVoices(p.Voices)
        dto.SetDetune(p.Detune)
        dto.SetDetuneCurve(dcurve)
        dto.SetMix(effectiveMix)
        dto.SetStereoWidth(p.StereoWidth)
        dto.SetAnalogDrift(p.AnalogDrift)
        dto.SetAnalogDriftDepth(p.AnalogDriftDepth)
        dto.SetRate(p.DtoRate)
        dto.SetCenterDelayMs(p.CenterDelayMs)
        dto.SetSubOctave(p.SubOctave)
        dto.DetuneBufferStereo(ssamples, float64(p.SampleRate))
        return xoscillator.ConvertSamplesFtoI(ssamples), nil
    }

    if !forSave {
        proc := xoscillator.NewDBGainProcessor(p.DbGain)
        proc.ProcessBuffer(samples)
    }
    return xoscillator.ConvertSamplesFtoI2(samples), nil
}

// computeSpectrum performs an FFT on left-channel samples to yield normalized magnitudes [0.0, 1.0].
func computeSpectrum(stereoSamples []int16, numBins int) []float64 {
    fftSize := 1024
    x := make([]complex128, fftSize)
    for i := 0; i < fftSize && (i*2) < len(stereoSamples); i++ {
        window := 0.5 * (1.0 - math.Cos(2.0*math.Pi*float64(i)/float64(fftSize-1)))
        sample := (float64(stereoSamples[i*2]) / math.MaxInt16) * window
        x[i] = complex(sample, 0)
    }

    var fft func([]complex128) []complex128
    fft = func(a []complex128) []complex128 {
        n := len(a)
        if n <= 1 {
            return a
        }
        even := make([]complex128, n/2)
        odd := make([]complex128, n/2)
        for i := 0; i < n/2; i++ {
            even[i] = a[2*i]
            odd[i] = a[2*i+1]
        }
        even = fft(even)
        odd = fft(odd)
        out := make([]complex128, n)
        for k := 0; k < n/2; k++ {
            t := cmplx.Exp(complex(0, -2.0*math.Pi*float64(k)/float64(n))) * odd[k]
            out[k] = even[k] + t
            out[k+n/2] = even[k] - t
        }
        return out
    }

    spectrum := fft(x)
    half := fftSize / 2
    if numBins > half {
        numBins = half
    }
    magnitudes := make([]float64, numBins)
    maxMag := 1e-9
    step := half / numBins
    for b := 0; b < numBins; b++ {
        mag := cmplx.Abs(spectrum[b*step])
        magnitudes[b] = mag
        if mag > maxMag {
            maxMag = mag
        }
    }
    for b := range magnitudes {
        magnitudes[b] /= maxMag
    }
    return magnitudes
}

// resolveEnvelopeCurve translates curve string selections into xaudio.EnvelopeCurve callbacks.
func resolveEnvelopeCurve(name string, cycles, depth float64) xaudio.EnvelopeCurve {
    switch name {
    case "Exponential":
        return xaudio.EnvExponential
    case "Logarithmic":
        return xaudio.EnvLogarithmic
    case "Hermite":
        return xaudio.EnvHermite
    case "Radian":
        return xaudio.EnvRadian
    case "BicubicWobble":
        return xaudio.NewBicubicWobbleCurve(cycles, depth)
    default:
        return xaudio.EnvLinear
    }
}

// resolveDetuneCurve translates detune curve string selections into xaudio.DetuneCurve callbacks.
func resolveDetuneCurve(name string) xoscillator.DetuneCurve {
    switch name {
    case "Scurve":
        return xoscillator.DetuneCurveSCurve
    case "Exponential":
        return xoscillator.DetuneCurveExponential
    default:
        return xoscillator.DetuneCurveLinear
    }
}

// synthUI consolidates all Fyne application state, widgets, and layout containers into one struct.
type synthUI struct {
    app            fyne.App
    window         fyne.Window
    params         SynthParams
    currentSample  []int16
    currentSampleN []int16
    vizMode        string
    vizLastSize    fyne.Size
    iniPath        string
    iniPresets     map[string]SynthParams
    applyingState  bool

    // Status & visualization widgets
    statusLabel    *widget.Label
    vizContainer   *fyne.Container
    vizBg          *canvas.Rectangle
    vizGroupHeader *fyne.Container
    vizContent     *fyne.Container
    vizAccordion   *autoFitAccordion

    // Selectors, combobox & checkboxes
    modeSelect         *widget.Select
    presetSaveBtn      *widget.Button
    presetLoadBtn      *widget.Button
    srSelect           *widget.Select
    waveSelect         *widget.Select
    detuneCurveSelect  *widget.Select
    subOctCheck        *widget.Check
    hardSyncCheck      *widget.Check
    attackCurveSelect  *widget.Select
    decayCurveSelect   *widget.Select
    releaseCurveSelect *widget.Select

    // Sliders and real-time value readouts
    lengthSlider     *widget.Slider
    lengthValLbl     *widget.Label
    tuneSlider       *widget.Slider
    tuneValLbl       *widget.Label
    toneSlider       *widget.Slider
    toneValLbl       *widget.Label
    snappySlider     *widget.Slider
    snappyValLbl     *widget.Label
    modRatioSlider   *widget.Slider
    modRatioValLbl   *widget.Label
    modIndexSlider   *widget.Slider
    modIndexValLbl   *widget.Label
    voicesSlider     *widget.Slider
    voicesValLbl     *widget.Label
    detuneSlider     *widget.Slider
    detuneValLbl     *widget.Label
    mixSlider        *widget.Slider
    mixValLbl        *widget.Label
    widthSlider      *widget.Slider
    widthValLbl      *widget.Label
    driftSlider      *widget.Slider
    driftValLbl      *widget.Label
    pulseWidthSlider *widget.Slider
    pulseWidthValLbl *widget.Label
    attackSlider     *widget.Slider
    attackValLbl     *widget.Label
    decaySlider      *widget.Slider
    decayValLbl      *widget.Label
    sustainSlider    *widget.Slider
    sustainValLbl    *widget.Label
    releaseSlider    *widget.Slider
    releaseValLbl    *widget.Label

    // Action buttons & metadata entries
    playBtn     *widget.Button
    saveBtn     *widget.Button
    resetBtn    *widget.Button
    titleEntry  *widget.Entry
    artistEntry *widget.Entry
    descEntry   *widget.Entry

    metaContent   *fyne.Container
    metaAccordion *autoFitAccordion
    colComposer   *fyne.Container
    colUnison     *fyne.Container
    topBar        *fyne.Container

    vizRateMu     sync.Mutex
    vizLastRender time.Time
    vizPending    bool
    vizTimer      *time.Timer

    // Audio player state & progress tracker (malgo)
    malgoCtx        *malgo.AllocatedContext
    malgoDev        *malgo.Device
    playMu          sync.Mutex
    playPCM         []byte // Little-endian 16-bit stereo PCM buffer
    playByteOffset  int    // Current read cursor in playPCM (aligned to 4 bytes per stereo frame)
    playTotalFrames int
    playSampleRate  int
    playSource      string // "Note" or "Sequence"
    isPlaying       bool
    isPaused        bool
    playTicker      *time.Ticker
    playStopCh      chan struct{}

    // Player UI widgets
    playerPlayPauseBtn *widget.Button
    playerStopBtn      *widget.Button
    playerProgressBar  *widget.ProgressBar
    playerSeekSlider   *widget.Slider
    playerTimeLabel    *widget.Label
    playerBar          *fyne.Container
    seekingUI          bool
    loopPlayback       bool
    playerLoopCheck    *widget.Check
    normValLbl         *widget.Label
    normSlider         *widget.Slider
    userCurrentPath    string
    baseWaveComposer   *fyne.Container
    noiseSeedValLbl    *widget.Label
    noiseSeedSlider    *widget.Slider
    asdrCheck          *widget.Check
    delayValLbl        *widget.Label
    delaySlider        *widget.Slider
    holdValLbl         *widget.Label
    holdSlider         *widget.Slider
    tenvCheck          *widget.Check
    tenvSelect         *widget.Select
    tenvValLbl         []*widget.Label
    tenvSlider         []*widget.Slider
    colTENV            *autoFitAccordion
    colADSR            *autoFitAccordion
    tenvBtn1           *widget.Button
    tenvBtn2           *widget.Button
    tenvBtn3           *widget.Button
    dtoSelect          *widget.Select
    colDTO             *autoFitAccordion
    fmWaveComposer     *fyne.Container
    fm1Select          *widget.Select
    fm2Select          *widget.Select
    cdmValLbl          *widget.Label
    cdmSlider          *widget.Slider
    rateValLbl         *widget.Label
    rateSlider         *widget.Slider
    soctValLbl         *widget.Label
    soctSlider         *widget.Slider
    driftDepthValLbl   *widget.Label
    driftDepthSlider   *widget.Slider
    dbgValLbl          *widget.Label
    dbgSlider          *widget.Slider
}

func (ui *synthUI) syncParamsToUI(params SynthParams) {
    if ui == nil {
        return
    }

    prevApplying := ui.applyingState
    ui.applyingState = true
    defer func() {
        ui.applyingState = prevApplying
    }()

    ui.params = params

    // Selectors & checkboxes
    setSelectValue(ui.srSelect, formatSampleRateOption(params.SampleRate))
    setSelectValue(ui.waveSelect, params.BaseWave)
    setSelectValue(ui.detuneCurveSelect, params.DetuneCurve)
    setCheckValue(ui.subOctCheck, params.SubOctave > 0)
    setCheckValue(ui.hardSyncCheck, params.HardSync)
    setCheckValue(ui.asdrCheck, params.AsdrEnable)
    setCheckValue(ui.tenvCheck, params.TenvEnable)
    setSelectValue(ui.tenvSelect, params.TenvInterp)
    setSelectValue(ui.dtoSelect, params.DtoStage)
    setSelectValue(ui.fm1Select, params.Fm1Wave)
    setSelectValue(ui.fm2Select, params.Fm2Wave)

    // Sliders
    setSliderValue(ui.lengthSlider, params.LengthMs)
    setSliderValue(ui.tuneSlider, params.Tune)
    setSliderValue(ui.toneSlider, params.Tone)
    setSliderValue(ui.snappySlider, params.Snappy)
    setSliderValue(ui.normSlider, float64(params.NormalizePct))
    setSliderValue(ui.voicesSlider, float64(params.Voices))
    setSliderValue(ui.detuneSlider, params.Detune)
    setSliderValue(ui.mixSlider, params.DetuneMix)
    setSliderValue(ui.widthSlider, params.StereoWidth)
    setSliderValue(ui.driftSlider, params.AnalogDrift)
    setSliderValue(ui.driftDepthSlider, params.AnalogDriftDepth)
    setSliderValue(ui.soctSlider, params.SubOctave)
    setSliderValue(ui.cdmSlider, params.CenterDelayMs)
    setSliderValue(ui.rateSlider, params.DtoRate)
    setSliderValue(ui.pulseWidthSlider, params.PulseWidth)
    setSliderValue(ui.modRatioSlider, params.ModRatio)
    setSliderValue(ui.modIndexSlider, params.ModIndex)
    setSliderValue(ui.noiseSeedSlider, float64(params.Seed))

    setSliderValue(ui.delaySlider, params.Delay)
    setSliderValue(ui.attackSlider, params.Attack)
    setSliderValue(ui.holdSlider, params.Hold)
    setSliderValue(ui.decaySlider, params.Decay)
    setSliderValue(ui.sustainSlider, params.Sustain)
    setSliderValue(ui.releaseSlider, params.Release)

    for i, s := range ui.tenvSlider {
        var val float64
        if i < len(params.Tenv) {
            val = params.Tenv[i]
        }
        setSliderValue(s, val)
    }
}

func setSelectValue(sel *widget.Select, val string) {
    if sel == nil || val == "" {
        return
    }
    for _, opt := range sel.Options {
        if opt == val {
            sel.SetSelected(val)
            return
        }
    }
}

func setCheckValue(chk *widget.Check, checked bool) {
    if chk != nil {
        chk.SetChecked(checked)
    }
}

func setSliderValue(s *widget.Slider, val float64) {
    if s != nil {
        s.SetValue(val)
    }
}

func formatSampleRateOption(sr int) string {
    if sr <= 0 {
        return ""
    }
    return fmt.Sprintf("%d kHz", sr/1000)
}

// newSynthUI initializes defaults, loads XDG INI presets if present, and constructs all widgets.
func newSynthUI() *synthUI {
    a := app.New()
    t := &customSynthTheme{dark: true}
    a.Settings().SetTheme(t)

    w := a.NewWindow("SamplesComposer & Unison Synthesizer " + pkg.PkgVersion)
    w.SetFixedSize(false)

    ui := &synthUI{
        app:     a,
        window:  w,
        vizMode: "Waveform",
        params:  defaultSynthParams(),
    }

    ui.initWidgets()
    ui.buildLayout()
    ui.regenerateNote()
    return ui
}

// applyParamsToControls synchronizes all slider, selector, and checkbox widgets to p.
func (ui *synthUI) applyParamsToControls(p SynthParams) {
    ui.applyingState = true
    defer func() { ui.applyingState = false }()

    ui.params = p

    switch p.SampleRate {
    case 8000:
        ui.srSelect.SetSelected("8 kHz")
    case 16000:
        ui.srSelect.SetSelected("16 kHz")
    case 32000:
        ui.srSelect.SetSelected("32 kHz")
    default:
        ui.srSelect.SetSelected("48 kHz")
    }

    ui.lengthSlider.SetValue(p.LengthMs)
    ui.lengthValLbl.SetText(fmt.Sprintf("%.0f ms", p.LengthMs))

    ui.tuneSlider.SetValue(p.Tune)
    ui.tuneValLbl.SetText(fmt.Sprintf("%.2f Hz", p.Tune))

    ui.toneSlider.SetValue(p.Tone)
    ui.toneValLbl.SetText(fmt.Sprintf("%.2f", p.Tone))

    ui.snappySlider.SetValue(p.Snappy)
    ui.snappyValLbl.SetText(fmt.Sprintf("%.2f", p.Snappy))

    ui.modRatioSlider.SetValue(p.ModRatio)
    ui.modRatioValLbl.SetText(fmt.Sprintf("%.2f", p.ModRatio))

    ui.modIndexSlider.SetValue(p.ModIndex)
    ui.modIndexValLbl.SetText(fmt.Sprintf("%.2f", p.ModIndex))

    ui.waveSelect.SetSelected(p.BaseWave)
    //ui.detuneCurveSelect.SetSelected(formatDetuneCurveName(&p.DetuneCurve))

    ui.voicesSlider.SetValue(float64(p.Voices))
    ui.voicesValLbl.SetText(fmt.Sprintf("%d", p.Voices))

    ui.detuneSlider.SetValue(p.Detune)
    ui.detuneValLbl.SetText(fmt.Sprintf("%.2f", p.Detune))

    ui.mixSlider.SetValue(p.DetuneMix)
    ui.mixValLbl.SetText(fmt.Sprintf("%.2f", p.DetuneMix))

    ui.widthSlider.SetValue(p.StereoWidth)
    ui.widthValLbl.SetText(fmt.Sprintf("%.2f", p.StereoWidth))

    ui.driftSlider.SetValue(p.AnalogDrift)
    ui.driftValLbl.SetText(fmt.Sprintf("%.2f", p.AnalogDrift))

    ui.pulseWidthSlider.SetValue(p.PulseWidth)
    ui.pulseWidthValLbl.SetText(fmt.Sprintf("%.2f", p.PulseWidth))

    ui.attackSlider.SetValue(p.Attack)
    ui.attackValLbl.SetText(fmt.Sprintf("%.3fs", p.Attack))

    ui.decaySlider.SetValue(p.Decay)
    ui.decayValLbl.SetText(fmt.Sprintf("%.2fs", p.Decay))

    ui.sustainSlider.SetValue(p.Sustain)
    ui.sustainValLbl.SetText(fmt.Sprintf("%.2f", p.Sustain))

    ui.releaseSlider.SetValue(p.Release)
    ui.releaseValLbl.SetText(fmt.Sprintf("%.2fs", p.Release))

}

// applyPreset updates synthesizer parameters from either a hardcoded preset or an INI preset.
func (ui *synthUI) applyPreset(name string) {
    if ui.applyingState {
        return
    }
    trimmed := strings.TrimSpace(name)
    if trimmed == "" {
        return
    }

    p := ui.params
    var sourceLabel string

    ui.applyParamsToControls(p)
    ui.statusLabel.SetText("Loaded " + sourceLabel)
    ui.regenerateNote()
}

// fitWindowToContent sizes the window so all visible widgets fit without scrollbars.
func (ui *synthUI) fitWindowToContent() {
    if ui.window == nil || ui.window.Content() == nil {
        return
    }
    min := ui.window.Content().MinSize()
    cur := ui.window.Content().Size()
    targetW := cur.Width
    if min.Width > targetW {
        targetW = min.Width
    }
    ui.window.Resize(fyne.NewSize(targetW, min.Height))
}

// resetAllToDefaults restores all SynthParams, widget controls, and timeline steps to initial defaults.
func (ui *synthUI) resetAllToDefaults() {
    ui.applyParamsToControls(defaultSynthParams())
    ui.regenerateNote()
    ui.fitWindowToContent()
    ui.statusLabel.SetText("Reset all synthesizer parameters and timeline steps to defaults")
}

// requestVisualizationRedraw rate-limits visualizer re-renders to at most 5 times per second
// (every 200ms) while ensuring the latest parameter state is drawn after rapid slider changes.
func (ui *synthUI) requestVisualizationRedraw() {
    const minInterval = 200 * time.Millisecond

    ui.vizRateMu.Lock()
    now := time.Now()
    elapsed := now.Sub(ui.vizLastRender)

    if elapsed >= minInterval {
        ui.vizLastRender = now
        ui.vizPending = false
        if ui.vizTimer != nil {
            ui.vizTimer.Stop()
            ui.vizTimer = nil
        }
        ui.vizRateMu.Unlock()
        fyne.Do(func() {
            ui.redrawVisualization()
        })
        return
    }

    ui.vizPending = true
    if ui.vizTimer == nil {
        delay := minInterval - elapsed
        ui.vizTimer = time.AfterFunc(delay, func() {
            ui.vizRateMu.Lock()
            if !ui.vizPending {
                ui.vizTimer = nil
                ui.vizRateMu.Unlock()
                return
            }
            ui.vizLastRender = time.Now()
            ui.vizPending = false
            ui.vizTimer = nil
            ui.vizRateMu.Unlock()
            fyne.Do(func() {
                ui.redrawVisualization()
            })
        })
    }
    ui.vizRateMu.Unlock()
}

// detachMalgoDevice atomically clears playback state under playMu and returns the
// active device so Stop()/Uninit() can be called OUTSIDE playMu (preventing audio-thread deadlock).
func (ui *synthUI) detachMalgoDevice() *malgo.Device {
    ui.playMu.Lock()
    if ui.playStopCh != nil {
        close(ui.playStopCh)
        ui.playStopCh = nil
    }
    if ui.playTicker != nil {
        ui.playTicker.Stop()
        ui.playTicker = nil
    }
    dev := ui.malgoDev
    ui.malgoDev = nil
    ui.isPlaying = false
    ui.isPaused = false
    ui.playMu.Unlock()
    return dev
}

func stopAndUninitDevice(dev *malgo.Device) {
    if dev != nil {
        _ = dev.Stop()
        dev.Uninit()
    }
}

// startMalgoPlayback streams samples through malgo starting at startFrame without blocking the UI thread.
func (ui *synthUI) startMalgoPlayback(sourceLabel string, samples []int16, sampleRate int, startFrame int) {
    if len(samples) < 2 || sampleRate <= 0 {
        ui.statusLabel.SetText("No rendered audio samples to play")
        return
    }

    // Copy samples snapshot so concurrent synth parameter edits don't race with playback.
    pcmCopy := samplesToPCMBytes(samples)
    totalFrames := len(samples) / 2
    if startFrame < 0 || startFrame >= totalFrames {
        startFrame = 0
    }

    go func() {
        // 1. Stop any previous device OUTSIDE playMu so onSamples can never deadlock.
        stopAndUninitDevice(ui.detachMalgoDevice())

        // 2. Initialize malgo context OUTSIDE playMu.
        ui.playMu.Lock()
        ctx := ui.malgoCtx
        ui.playMu.Unlock()
        if ctx == nil {
            newCtx, err := malgo.InitContext(nil, malgo.ContextConfig{}, nil)
            if err != nil {
                fyne.Do(func() {
                    ui.statusLabel.SetText("malgo context error: " + err.Error())
                })
                return
            }
            ui.playMu.Lock()
            if ui.malgoCtx == nil {
                ui.malgoCtx = newCtx
                ctx = newCtx
            } else {
                ctx = ui.malgoCtx
                _ = newCtx.Uninit()
                newCtx.Free()
            }
            ui.playMu.Unlock()
        }

        const bytesPerFrame = 4 // 2 channels * 2 bytes (int16)

        ui.playMu.Lock()
        ui.playPCM = pcmCopy
        ui.playByteOffset = startFrame * bytesPerFrame
        ui.playTotalFrames = totalFrames
        ui.playSampleRate = sampleRate
        ui.playSource = sourceLabel
        ui.isPlaying = true
        ui.isPaused = false
        ui.playMu.Unlock()

        deviceConfig := malgo.DefaultDeviceConfig(malgo.Playback)
        deviceConfig.Playback.Format = malgo.FormatS16
        deviceConfig.Playback.Channels = 2
        deviceConfig.SampleRate = uint32(sampleRate)
        deviceConfig.Alsa.NoMMap = 1

        onSamples := func(pOutputSample, pInputSamples []byte, framecount uint32) {
            bytesNeeded := int(framecount) * bytesPerFrame
            if bytesNeeded > len(pOutputSample) {
                bytesNeeded = len(pOutputSample)
            }
            // Align to full stereo frames so channel phase/order is preserved across loop boundaries.
            bytesNeeded = (bytesNeeded / bytesPerFrame) * bytesPerFrame
            if bytesNeeded <= 0 {
                return
            }

            ui.playMu.Lock()
            if !ui.isPlaying || ui.isPaused || len(ui.playPCM) < bytesPerFrame {
                ui.playMu.Unlock()
                clear(pOutputSample[:bytesNeeded])
                return
            }

            written := 0
            for written < bytesNeeded {
                if ui.playByteOffset >= len(ui.playPCM) {
                    if !ui.loopPlayback {
                        break
                    }
                    ui.playByteOffset = 0
                }
                remaining := len(ui.playPCM) - ui.playByteOffset
                toCopy := bytesNeeded - written
                if remaining < toCopy {
                    toCopy = remaining
                }
                copy(pOutputSample[written:written+toCopy], ui.playPCM[ui.playByteOffset:ui.playByteOffset+toCopy])
                ui.playByteOffset += toCopy
                written += toCopy
            }
            looping := ui.loopPlayback
            if ui.playByteOffset >= len(ui.playPCM) && looping {
                ui.playByteOffset = 0
            }
            ui.playMu.Unlock()

            if written < bytesNeeded {
                clear(pOutputSample[written:bytesNeeded])
            }
        }

        // 3. Init and Start device OUTSIDE playMu.
        dev, err := malgo.InitDevice(ctx.Context, deviceConfig, malgo.DeviceCallbacks{
            Data: onSamples,
        })
        if err != nil {
            ui.playMu.Lock()
            ui.isPlaying = false
            ui.playMu.Unlock()
            fyne.Do(func() {
                ui.statusLabel.SetText("malgo device init error: " + err.Error())
            })
            return
        }

        if err := dev.Start(); err != nil {
            dev.Uninit()
            ui.playMu.Lock()
            ui.isPlaying = false
            ui.playMu.Unlock()
            fyne.Do(func() {
                ui.statusLabel.SetText("malgo start error: " + err.Error())
            })
            return
        }

        stopCh := make(chan struct{})
        ticker := time.NewTicker(80 * time.Millisecond)

        ui.playMu.Lock()
        ui.malgoDev = dev
        ui.playStopCh = stopCh
        ui.playTicker = ticker
        ui.playMu.Unlock()

        fyne.Do(func() {
            ui.refreshPlayerUI(startFrame, totalFrames, sampleRate, true, false)
        })

        // 4. Progress tracker loop.
        defer ticker.Stop()
        for {
            select {
            case <-stopCh:
                return
            case <-ticker.C:
                ui.playMu.Lock()
                if !ui.isPlaying || ui.malgoDev != dev {
                    ui.playMu.Unlock()
                    return
                }
                curFrames := ui.playByteOffset / bytesPerFrame
                totFrames := ui.playTotalFrames
                sr := ui.playSampleRate
                paused := ui.isPaused
                looping := ui.loopPlayback
                finished := !looping && ui.playByteOffset >= len(ui.playPCM)
                ui.playMu.Unlock()

                if finished {
                    stopAndUninitDevice(ui.detachMalgoDevice())
                    ui.playMu.Lock()
                    ui.playByteOffset = totFrames * bytesPerFrame
                    ui.playMu.Unlock()
                    fyne.Do(func() {
                        ui.refreshPlayerUI(totFrames, totFrames, sr, false, false)
                        ui.statusLabel.SetText("Finished playing " + sourceLabel)
                    })
                    return
                }

                fyne.Do(func() {
                    ui.refreshPlayerUI(curFrames, totFrames, sr, true, paused)
                })
            }
        }
    }()
}

// redrawVisualization updates the waveform or FFT spectrum canvas using the current container width.
func (ui *synthUI) redrawVisualization() {
    ui.vizBg.FillColor = color.RGBA{R: 18, G: 20, B: 26, A: 255}
    ui.vizContainer.Objects = []fyne.CanvasObject{ui.vizBg}

    width := ui.vizContainer.Size().Width
    height := ui.vizContainer.Size().Height
    if width <= 1 && ui.window.Content() != nil {
        width = ui.window.Content().Size().Width - 16
    }
    if width <= 1 {
        width = 1000
    }
    if height <= 1 {
        height = 160
    }
    ui.vizBg.Resize(fyne.NewSize(width, height))

    if len(ui.currentSampleN) == 0 {
        ui.vizContainer.Refresh()
        return
    }

    waveColor := color.RGBA{R: 0, G: 220, B: 180, A: 255}
    specColor := color.RGBA{R: 255, G: 140, B: 40, A: 255}

    if strings.HasPrefix(ui.vizMode, "Waveform") {
        numPoints := int(width)
        if numPoints < 60 {
            numPoints = 60
        }

        sLen := len(ui.currentSampleN)
        midY := height / 2.0
        //dx := width / float32(sLen)
        if ui.vizMode == "Waveform" {
            var prevX, prevY float32
            for i := 0; (i < sLen) && (i < int(width*10)); i++ {
                n := float32(ui.currentSampleN[i]) / float32(math.MaxInt16)
                x := float32(i / 10.0)
                y := midY - (n * (midY - 10))
                if i > 0 {
                    line := canvas.NewLine(waveColor)
                    line.StrokeWidth = 1
                    line.Position1 = fyne.NewPos(prevX, prevY)
                    line.Position2 = fyne.NewPos(x, y)
                    ui.vizContainer.Add(line)
                }
                prevX, prevY = x, y
            }
        } else {
            step := sLen / numPoints
            if step < 1 {
                step = 1
            }
            var prevX, prevY float32
            for i := 0; i < numPoints && (i*step) < sLen; i++ {
                norm := float32(ui.currentSampleN[i*step]) / float32(math.MaxInt16)
                x := float32(i) //* dx
                y := midY - (norm * (midY - 10))
                if i > 0 {
                    line := canvas.NewLine(waveColor)
                    line.StrokeWidth = 1
                    line.Position1 = fyne.NewPos(prevX, prevY)
                    line.Position2 = fyne.NewPos(x, y)
                    ui.vizContainer.Add(line)
                }
                prevX, prevY = x, y
            }
        }
    } else {
        bins := int(width / 16)
        if bins < 16 {
            bins = 16
        }
        if bins > 128 {
            bins = 128
        }
        mags := computeSpectrum(ui.currentSample, bins)
        barW := width / float32(bins)
        for b, mag := range mags {
            barH := float32(mag) * (height - 16)
            bar := canvas.NewRectangle(specColor)
            bar.Move(fyne.NewPos(float32(b)*barW+1, height-barH))
            bar.Resize(fyne.NewSize(barW-2, barH))
            ui.vizContainer.Add(bar)
        }
    }
    ui.vizContainer.Refresh()
}

func samplesToPCMBytes(samples []int16) []byte {
    buf := make([]byte, len(samples)*2)
    for i, s := range samples {
        binary.LittleEndian.PutUint16(buf[i*2:i*2+2], uint16(s))
    }
    return buf
}

func formatSeconds(sec float64) string {
    if sec < 0 {
        sec = 0
    }
    mins := int(sec) / 60
    rem := sec - float64(mins*60)
    return fmt.Sprintf("%02d:%04.1f", mins, rem)
}

func (ui *synthUI) togglePlayPause() {
    ui.playMu.Lock()
    dev := ui.malgoDev
    playing := ui.isPlaying
    paused := ui.isPaused
    curFrames := ui.playByteOffset / 4
    totFrames := ui.playTotalFrames
    sr := ui.playSampleRate
    src := ui.playSource
    ui.playMu.Unlock()

    if dev != nil && playing {
        if paused {
            go func() {
                _ = dev.Start()
                ui.playMu.Lock()
                ui.isPaused = false
                ui.playMu.Unlock()
                fyne.Do(func() {
                    ui.refreshPlayerUI(curFrames, totFrames, sr, true, false)
                    ui.statusLabel.SetText("Resumed playback")
                })
            }()
            return
        }
        go func() {
            _ = dev.Stop()
            ui.playMu.Lock()
            ui.isPaused = true
            ui.playMu.Unlock()
            fyne.Do(func() {
                ui.refreshPlayerUI(curFrames, totFrames, sr, true, true)
                ui.statusLabel.SetText("Paused playback")
            })
        }()
        return
    }

    if src == "" {
        src = "Note"
    }
    if len(ui.currentSample) == 0 {
        ui.regenerateNote()
    }
    ui.startMalgoPlayback(src, ui.currentSample, ui.params.SampleRate, 0)
}

// refreshPlayerUI updates Fyne widgets using pre-snapshotted values without acquiring playMu.
func (ui *synthUI) refreshPlayerUI(curFrames, totalFrames, sr int, playing, paused bool) {
    if ui.playerProgressBar == nil || ui.playerTimeLabel == nil || ui.playerPlayPauseBtn == nil {
        return
    }
    if totalFrames == 0 && len(ui.currentSample) >= 2 {
        totalFrames = len(ui.currentSample) / 2
        sr = ui.params.SampleRate
    }
    var posSec, totalSec, ratio float64
    if sr > 0 && totalFrames > 0 {
        posSec = float64(curFrames) / float64(sr)
        totalSec = float64(totalFrames) / float64(sr)
        ratio = math.Min(1.0, math.Max(0.0, float64(curFrames)/float64(totalFrames)))
    }
    ui.playerProgressBar.SetValue(ratio)
    if ui.playerSeekSlider != nil {
        ui.seekingUI = true
        ui.playerSeekSlider.SetValue(ratio)
        ui.seekingUI = false
    }
    ui.playerTimeLabel.SetText(fmt.Sprintf("%s / %s", formatSeconds(posSec), formatSeconds(totalSec)))
    if playing && !paused {
        ui.playerPlayPauseBtn.SetText("Pause")
    } else {
        ui.playerPlayPauseBtn.SetText("Play")
    }
}

func (ui *synthUI) stopPlayback() {
    go func() {
        stopAndUninitDevice(ui.detachMalgoDevice())
        ui.playMu.Lock()
        ui.playByteOffset = 0
        totFrames := ui.playTotalFrames
        sr := ui.playSampleRate
        ui.playMu.Unlock()
        fyne.Do(func() {
            ui.refreshPlayerUI(0, totFrames, sr, false, false)
            ui.statusLabel.SetText("Playback stopped")
        })
    }()
}

func (ui *synthUI) seekPlaybackRatio(ratio float64) {
    if ratio < 0 {
        ratio = 0
    }
    if ratio > 1 {
        ratio = 1
    }
    const bytesPerFrame = 4

    ui.playMu.Lock()
    if len(ui.playPCM) >= bytesPerFrame && ui.playTotalFrames > 0 {
        targetFrame := int(ratio * float64(ui.playTotalFrames))
        if targetFrame >= ui.playTotalFrames {
            targetFrame = ui.playTotalFrames - 1
        }
        ui.playByteOffset = targetFrame * bytesPerFrame
        totFrames := ui.playTotalFrames
        sr := ui.playSampleRate
        playing := ui.isPlaying
        paused := ui.isPaused
        ui.playMu.Unlock()
        ui.refreshPlayerUI(targetFrame, totFrames, sr, playing, paused)
        return
    }
    ui.playMu.Unlock()

    if len(ui.currentSample) >= 2 && ui.params.SampleRate > 0 {
        targetFrame := int(ratio * float64(len(ui.currentSample)/2))
        src := ui.playSource
        if src == "" {
            src = "Note"
        }
        ui.startMalgoPlayback(src, ui.currentSample, ui.params.SampleRate, targetFrame)
    }
}

// regenerateNote renders a single note via SynthesizeParams and redraws the visualization.
func (ui *synthUI) regenerateNote() {
    samples, err := SynthesizeParams(ui.params, false)
    if err != nil {
        ui.statusLabel.SetText("Synthesis error: " + err.Error())
        return
    }
    ui.currentSampleN = applySampleNormalization(samples, 100)
    ui.currentSample = applySampleNormalization(samples, ui.params.NormalizePct)
    ui.syncPlaybackWithRenderedSamples("Note", ui.currentSample, ui.params.SampleRate)
    ui.statusLabel.SetText(fmt.Sprintf("Rendered %d stereo frames @ %d Hz", len(ui.currentSample)/2, ui.params.SampleRate))
    ui.redrawVisualization()
}

// initWidgets instantiates all interactive controls on synthUI.
func (ui *synthUI) initWidgets() {
    ui.presetSaveBtn = widget.NewButton("Save Preset", func() { ui.savePresets() })
    ui.presetLoadBtn = widget.NewButton("Load Preset", func() { ui.loadPresets() })

    ui.dbgValLbl = widget.NewLabel(fmt.Sprintf("%+.1fdB", ui.params.DbGain))
    ui.dbgSlider = widget.NewSlider(-50.0, 0.0)
    ui.dbgSlider.Step = 1
    ui.dbgSlider.SetValue(ui.params.DbGain)
    ui.dbgSlider.OnChanged = func(v float64) {
        ui.params.DbGain = v
        ui.dbgValLbl.SetText(fmt.Sprintf("%+.1fdB", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.statusLabel = widget.NewLabel("Ready")

    ui.vizBg = canvas.NewRectangle(color.RGBA{R: 18, G: 20, B: 26, A: 255})
    ui.vizContainer = container.New(&stretchVizLayout{ui: ui}, ui.vizBg)

    ui.modeSelect = widget.NewSelect([]string{"Waveform", "Waveform2", "Spectrum"}, func(s string) {
        ui.vizMode = s
        ui.redrawVisualization()
    })
    ui.modeSelect.SetSelected("Waveform")

    ui.buildFmWaveUI()

    // detune oversampling
    ui.buildDetuneUI()

    //T-ENV
    ui.buildTenvUI()

    //ASDR
    ui.buildAsdrUI()

    ui.titleEntry = widget.NewEntry()
    ui.titleEntry.SetText("SamplesComposer Sample")
    ui.artistEntry = widget.NewEntry()
    ui.artistEntry.SetText("SamplesComposer Synth")
    ui.descEntry = widget.NewEntry()
    ui.descEntry.SetText("Generated with github.com/terefang/SamplesComposer")

    ui.playBtn = widget.NewButton("Play Note", func() {
        ui.regenerateNote()
        ui.startMalgoPlayback("Note", ui.currentSample, ui.params.SampleRate, 0)
        ui.statusLabel.SetText("Playing synthesized note via malgo")
    })

    if ui.userCurrentPath == "" {
        ui.userCurrentPath, _ = os.UserHomeDir()
    }
    ui.saveBtn = widget.NewButton("Save Sample", ui.saveWavFile())

    ui.resetBtn = widget.NewButton("Reset Defaults", func() {
        ui.resetAllToDefaults()
    })

    ui.playerPlayPauseBtn = widget.NewButton("Play", func() {
        ui.togglePlayPause()
    })
    ui.playerStopBtn = widget.NewButton("Stop", func() {
        ui.stopPlayback()
    })
    ui.playerProgressBar = widget.NewProgressBar()
    ui.playerSeekSlider = widget.NewSlider(0.0, 1.0)
    ui.playerSeekSlider.Step = 0.005
    ui.playerSeekSlider.OnChanged = func(v float64) {
        if ui.seekingUI {
            return
        }
        ui.seekPlaybackRatio(v)
    }
    ui.playerTimeLabel = widget.NewLabel("00:00.0 / 00:00.0")
    ui.playerLoopCheck = widget.NewCheck("Loop", func(checked bool) {
        ui.playMu.Lock()
        ui.loopPlayback = checked
        ui.playMu.Unlock()
    })
    ui.playerLoopCheck.SetChecked(ui.loopPlayback)
    ui.playerBar = container.NewBorder(
        nil, nil,
        container.NewVBox(
            ui.playerPlayPauseBtn,
            ui.playerStopBtn,
        ),
        container.NewVBox(
            ui.playerTimeLabel,
            ui.playerLoopCheck,
        ),
        container.NewVBox(ui.playerProgressBar, ui.playerSeekSlider),
    )

}

func (ui *synthUI) saveWavFile() func() {
    return func() {
        _dialog := xfyne.NewSelectDialog(
            "Select File to Save ...",
            ui.userCurrentPath,
            false,
            func(outPath string) {

                if strings.ToLower(filepath.Ext(outPath)) != ".wav" {
                    outPath += ".wav"
                }

                text, _ := json.Marshal(ui.params)
                meta := xaudio.WavMetadata{
                    Title:       ui.titleEntry.Text,
                    Artist:      ui.artistEntry.Text,
                    Description: ui.descEntry.Text,
                    Software:    "SampleComposer Fyne Synth",
                    Date:        "2026-11-19",
                    Text:        text,
                }

                outSamples, err := SynthesizeParams(ui.params, true)
                outSamples = applySampleNormalization(outSamples, ui.params.NormalizePct)

                if err == nil {
                    if err = xaudio.WriteWavWithMetadata(outPath, ui.params.SampleRate, 2, outSamples, meta); err != nil {
                        ui.statusLabel.SetText("Save error: " + err.Error())
                        return
                    }
                    ui.statusLabel.SetText("Saved " + outPath + " with RIFF INFO metadata")
                } else {
                    ui.statusLabel.SetText("Error: " + err.Error())
                }
            },
            func() {
                ui.statusLabel.SetText("Save dialog error: Dialog cancelled")
            },
        )
        _dialog.SetPreferredDirs(
            xdg.DocumentsPath("Samples"),
            xdg.DocumentsPath("Music"),
            xdg.DocumentsDir(),
            xdg.ProjectsPath("Samples"),
            xdg.ProjectsPath("Music"),
            xdg.ProjectsDir(),
            xdg.DownloadDir(),
            xdg.DesktopDir())

        _dialog.SetAction("Save")
        _dialog.SetFileName(fmt.Sprintf("%s_%dHz_%dpw_0x%08X_%s.wav", ui.params.BaseWave, int(ui.params.Tune), int(ui.params.PulseWidth*100), ui.params.Seed, ui.params.DtoStage))
        _dialog.AddFileTypeFilter("Wave Files (*.wav)", storage.NewExtensionFileFilter([]string{".wav"}))
        _dialog.Show()
    }
}

// syncPlaybackWithRenderedSamples updates the player's PCM buffer whenever parameters
// or sequence steps re-render, hot-swapping live playback or restarting if SampleRate changed.
func (ui *synthUI) syncPlaybackWithRenderedSamples(sourceLabel string, samples []int16, sampleRate int) {
    if len(samples) < 2 || sampleRate <= 0 {
        return
    }
    const bytesPerFrame = 4
    pcmCopy := samplesToPCMBytes(samples)
    totalFrames := len(samples) / 2

    ui.playMu.Lock()
    active := ui.malgoDev != nil && ui.isPlaying
    paused := ui.isPaused
    sameRate := ui.playSampleRate == sampleRate
    curFrames := ui.playByteOffset / bytesPerFrame

    if active && !sameRate {
        ui.playMu.Unlock()
        if curFrames >= totalFrames {
            curFrames = 0
        }
        ui.startMalgoPlayback(sourceLabel, samples, sampleRate, curFrames)
        return
    }

    ui.playPCM = pcmCopy
    ui.playTotalFrames = totalFrames
    ui.playSampleRate = sampleRate
    ui.playSource = sourceLabel

    if active {
        if ui.playByteOffset >= len(pcmCopy) {
            if ui.loopPlayback {
                ui.playByteOffset = 0
            } else {
                ui.playByteOffset = (totalFrames - 1) * bytesPerFrame
            }
        }
        curFrames = ui.playByteOffset / bytesPerFrame
        ui.playMu.Unlock()
        ui.refreshPlayerUI(curFrames, totalFrames, sampleRate, true, paused)
        return
    }

    ui.playByteOffset = 0
    ui.playMu.Unlock()
    ui.refreshPlayerUI(0, totalFrames, sampleRate, false, false)
}

// makeParameterRow places left label, stretching center input widget (with "-" and "+" buttons for sliders),
// and right numeric label in a Border container so the input widget expands across all available horizontal space.
func makeParameterRow(labelText string, input fyne.CanvasObject, valLabel *widget.Label) *fyne.Container {
    center := input
    if slider, ok := input.(*widget.Slider); ok {
        step := slider.Step
        if step <= 0 {
            step = 1.0
        }
        decBtn := widget.NewButton("-", func() {
            next := slider.Value - step
            if next < slider.Min {
                next = slider.Min
            }
            next = math.Round(next*1e6) / 1e6
            slider.SetValue(next)
        })
        incBtn := widget.NewButton("+", func() {
            next := slider.Value + step
            if next > slider.Max {
                next = slider.Max
            }
            next = math.Round(next*1e6) / 1e6
            slider.SetValue(next)
        })
        center = container.NewBorder(nil, nil, decBtn, incBtn, slider)
    }

    var right fyne.CanvasObject = layout.NewSpacer()
    if valLabel != nil {
        right = valLabel
    }
    return container.NewBorder(nil, nil, widget.NewLabel(labelText), right, center)
}

func makeHSliderPlusMinus(slider *widget.Slider) *fyne.Container {
    step := slider.Step
    if step <= 0 {
        step = 1.0
    }
    decBtn := widget.NewButton("-", func() {
        next := slider.Value - step
        if next < slider.Min {
            next = slider.Min
        }
        next = math.Round(next*1e6) / 1e6
        slider.SetValue(next)
    })
    incBtn := widget.NewButton("+", func() {
        next := slider.Value + step
        if next > slider.Max {
            next = slider.Max
        }
        next = math.Round(next*1e6) / 1e6
        slider.SetValue(next)
    })
    return container.NewBorder(nil, nil, decBtn, incBtn, slider)
}

func makeVSliderPlusMinus(slider *widget.Slider) *fyne.Container {
    step := slider.Step
    if step <= 0 {
        step = 1.0
    }
    decBtn := widget.NewButton("-", func() {
        next := slider.Value - step
        if next < slider.Min {
            next = slider.Min
        }
        next = math.Round(next*1e6) / 1e6
        slider.SetValue(next)
    })
    incBtn := widget.NewButton("+", func() {
        next := slider.Value + step
        if next > slider.Max {
            next = slider.Max
        }
        next = math.Round(next*1e6) / 1e6
        slider.SetValue(next)
    })
    return container.NewBorder(incBtn, decBtn, nil, nil, slider)
}

func makeVSliderPlusMinus2(slider *widget.Slider) *fyne.Container {
    step := slider.Step
    if step <= 0 {
        step = 1.0
    }
    decBtn := widget.NewButton("-", func() {
        next := slider.Value - step
        if next < slider.Min {
            next = slider.Min
        }
        next = math.Round(next*1e3) / 1e3
        slider.SetValue(next)
    })
    dec2Btn := widget.NewButton("--", func() {
        next := (slider.Value + slider.Min) / 2
        if next < slider.Min {
            next = slider.Min
        }
        next = math.Round(next*1e3) / 1e3
        slider.SetValue(next)
    })
    incBtn := widget.NewButton("+", func() {
        next := slider.Value + step
        if next > slider.Max {
            next = slider.Max
        }
        next = math.Round(next*1e3) / 1e3
        slider.SetValue(next)
    })
    inc2Btn := widget.NewButton("++", func() {
        next := (slider.Value + slider.Max) / 2
        if next > slider.Max {
            next = slider.Max
        }
        next = math.Round(next*1e3) / 1e3
        slider.SetValue(next)
    })
    x0Btn := widget.NewButton("0", func() {
        slider.SetValue(0)
    })
    x1Btn := widget.NewButton("M", func() {
        slider.SetValue(slider.Min)
    })
    x2Btn := widget.NewButton("X", func() {
        slider.SetValue(slider.Max)
    })
    return container.NewBorder(
        container.NewVBox(inc2Btn, incBtn),
        container.NewVBox(decBtn, dec2Btn, container.NewAdaptiveGrid(2, x0Btn, x1Btn, x2Btn)),
        nil, nil, slider)
}

// buildLayout arranges the toolbar, parameter columns, independently collapsible Sequence,
// Meta-Data, and Visualization accordions directly into the window without scrollbars.
func (ui *synthUI) buildLayout() {
    ui.buildBaseWaveUI()

    // Standalone Meta-Data accordion that auto-fits the window when opened or closed
    ui.metaContent = container.NewVBox(
        makeParameterRow("Title (INAM)", ui.titleEntry, nil),
        makeParameterRow("Artist (IART)", ui.artistEntry, nil),
        makeParameterRow("Description (ICMT)", ui.descEntry, nil),
    )
    ui.metaAccordion = newAutoFitAccordion(
        func() { ui.fitWindowToContent() },
        "Meta-Data",
        ui.metaContent,
    )
    ui.metaAccordion.Open(0)

    // Group label row paired directly with the visualization mode selector
    ui.vizGroupHeader = container.NewHBox(
        widget.NewLabelWithStyle("Visualization", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
        ui.modeSelect,
        layout.NewSpacer(),
    )

    // Collapsible Visualization accordion that auto-fits the window when opened or closed
    ui.vizContent = container.NewVBox(
        ui.vizGroupHeader,
        ui.vizContainer,
    )
    ui.vizAccordion = newAutoFitAccordion(
        func() { ui.fitWindowToContent() },
        "Visualization",
        ui.vizContent,
    )
    ui.vizAccordion.Open(0)

    ui.topBar = container.NewGridWithColumns(6,
        ui.playBtn,
        container.NewBorder(nil, nil, nil, ui.dbgValLbl, makeHSliderPlusMinus(ui.dbgSlider)),
        ui.resetBtn,
        ui.saveBtn,
        ui.presetSaveBtn,
        ui.presetLoadBtn,
    )

    scroller := container.NewVScroll(container.NewVBox(ui.baseWaveComposer,
        ui.fmWaveComposer,
        ui.colDTO,
        ui.colADSR,
        ui.colTENV,
        ui.metaAccordion,

    ),
    )
    scroller.SetMinSize(fyne.NewSize(600, 600))
    root := container.NewBorder(ui.topBar,
        container.NewVBox(ui.vizAccordion, ui.playerBar, ui.statusLabel),
        nil, nil, scroller)

    ui.window.SetContent(root)
    ui.window.SetOnClosed(func() {
        stopAndUninitDevice(ui.detachMalgoDevice())
        if ui.malgoCtx != nil {
            _ = ui.malgoCtx.Uninit()
            ui.malgoCtx.Free()
        }
    })
}
func (ui *synthUI) buildTenvUI() {
    ui.tenvCheck = widget.NewCheck("T-Env Enable", func(checked bool) {
        ui.params.TenvEnable = checked
        if !ui.applyingState {
            ui.regenerateNote()
        }
    })

    ui.tenvSelect = widget.NewSelect(xoscillator.EnvelopeInterpolatorNames(), func(s string) {
        ui.params.TenvInterp = s
        if !ui.applyingState {
            ui.regenerateNote()
        }
    })

    ui.tenvBtn1 = widget.NewButton("Zero All", func() {
        for i := 0; i < 16; i++ {
            ui.tenvSlider[i].SetValue(0)
        }
    })
    ui.tenvBtn2 = widget.NewButton("Max All", func() {
        for i := 0; i < 16; i++ {
            ui.tenvSlider[i].SetValue(1.0)
        }
    })
    ui.tenvBtn3 = widget.NewButton("Min All", func() {
        for i := 0; i < 16; i++ {
            ui.tenvSlider[i].SetValue(-1.0)
        }
    })

    ui.tenvValLbl = make([]*widget.Label, 16)
    ui.tenvSlider = make([]*widget.Slider, 16)
    for i := 0; i < 16; i++ {
        ui.tenvValLbl[i] = widget.NewLabel(fmt.Sprintf("%.3f", ui.params.Tenv[i]))
        ui.tenvSlider[i] = widget.NewSlider(-1.0, 1.0)
        ui.tenvSlider[i].Step = 0.005
        ui.tenvSlider[i].Orientation = widget.Vertical
        ui.tenvSlider[i].SetValue(ui.params.Tenv[i])
        ui.tenvSlider[i].OnChanged = func(v float64) {
            ui.params.Tenv[i] = v
            ui.tenvValLbl[i].SetText(fmt.Sprintf("%.3f", v))
            if !ui.applyingState {
                ui.regenerateNote()
            }
        }
    }

    envContent := container.NewVBox(
        container.NewHBox(ui.tenvCheck, ui.tenvSelect, ui.tenvBtn1, ui.tenvBtn2, ui.tenvBtn3),
        container.NewGridWithColumns(16,
            container.NewVBox(ui.tenvValLbl[0], makeVSliderPlusMinus2(ui.tenvSlider[0])),
            container.NewVBox(ui.tenvValLbl[1], makeVSliderPlusMinus2(ui.tenvSlider[1])),
            container.NewVBox(ui.tenvValLbl[2], makeVSliderPlusMinus2(ui.tenvSlider[2])),
            container.NewVBox(ui.tenvValLbl[3], makeVSliderPlusMinus2(ui.tenvSlider[3])),
            container.NewVBox(ui.tenvValLbl[4], makeVSliderPlusMinus2(ui.tenvSlider[4])),
            container.NewVBox(ui.tenvValLbl[5], makeVSliderPlusMinus2(ui.tenvSlider[5])),
            container.NewVBox(ui.tenvValLbl[6], makeVSliderPlusMinus2(ui.tenvSlider[6])),
            container.NewVBox(ui.tenvValLbl[7], makeVSliderPlusMinus2(ui.tenvSlider[7])),
            container.NewVBox(ui.tenvValLbl[8], makeVSliderPlusMinus2(ui.tenvSlider[8])),
            container.NewVBox(ui.tenvValLbl[9], makeVSliderPlusMinus2(ui.tenvSlider[9])),
            container.NewVBox(ui.tenvValLbl[10], makeVSliderPlusMinus2(ui.tenvSlider[10])),
            container.NewVBox(ui.tenvValLbl[11], makeVSliderPlusMinus2(ui.tenvSlider[11])),
            container.NewVBox(ui.tenvValLbl[12], makeVSliderPlusMinus2(ui.tenvSlider[12])),
            container.NewVBox(ui.tenvValLbl[13], makeVSliderPlusMinus2(ui.tenvSlider[13])),
            container.NewVBox(ui.tenvValLbl[14], makeVSliderPlusMinus2(ui.tenvSlider[14])),
            container.NewVBox(ui.tenvValLbl[15], makeVSliderPlusMinus2(ui.tenvSlider[15])),
        ),
    )
    ui.colTENV = newAutoFitAccordion(
        func() { ui.fitWindowToContent() },
        "T16-Envelope",
        envContent,
    )
}

func (ui *synthUI) buildDetuneUI() {
    dtonames := make([]string, 0)
    dtonames = append(dtonames, "off")
    dtonames = append(dtonames, xoscillator.AnalogSynthDetunePresetNames()...)
    ui.dtoSelect = widget.NewSelect(dtonames, func(s string) {
        ui.params.DtoStage = s
        if xoscillator.HasAnalogSynthDetunePreset(s) {
            dto, _ := xoscillator.LookupAnalogSynthDetunePreset(s)
            ui.voicesSlider.SetValue(float64(dto.Voices))
            ui.detuneSlider.SetValue(float64(dto.Detune))
            ui.mixSlider.SetValue(float64(dto.Mix))
            ui.cdmSlider.SetValue(float64(dto.CenterDelayMs))
            ui.rateSlider.SetValue(float64(dto.Rate))
            ui.driftSlider.SetValue(float64(dto.AnalogDrift))
            ui.driftDepthSlider.SetValue(float64(dto.AnalogDriftDepth))
            ui.detuneCurveSelect.SetSelectedIndex(int(dto.DetuneCurve))
            ui.widthSlider.SetValue(float64(dto.StereoWidth))
            ui.soctSlider.SetValue(float64(dto.SubOctave))
        }
        if !ui.applyingState {
            ui.regenerateNote()
        }
    })
    ui.dtoSelect.SetSelected(dtonames[0])

    ui.mixValLbl = widget.NewLabel(fmt.Sprintf("%.2f", ui.params.DetuneMix))
    ui.mixSlider = widget.NewSlider(0.0, 1.0)
    ui.mixSlider.Step = 0.01
    ui.mixSlider.SetValue(ui.params.DetuneMix)
    ui.mixSlider.OnChanged = func(v float64) {
        ui.params.DetuneMix = v
        ui.mixValLbl.SetText(fmt.Sprintf("%.2f", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.widthValLbl = widget.NewLabel(fmt.Sprintf("%.2f", ui.params.StereoWidth))
    ui.widthSlider = widget.NewSlider(0.0, 1.0)
    ui.widthSlider.Step = 0.01
    ui.widthSlider.SetValue(ui.params.StereoWidth)
    ui.widthSlider.OnChanged = func(v float64) {
        ui.params.StereoWidth = v
        ui.widthValLbl.SetText(fmt.Sprintf("%.2f", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.voicesValLbl = widget.NewLabel(fmt.Sprintf("%d", ui.params.Voices))
    ui.voicesSlider = widget.NewSlider(0, 16)
    ui.voicesSlider.Step = 1
    ui.voicesSlider.SetValue(float64(ui.params.Voices))
    ui.voicesSlider.OnChanged = func(v float64) {
        ui.params.Voices = int(v)
        ui.voicesValLbl.SetText(fmt.Sprintf("%d", ui.params.Voices))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.detuneCurveSelect = widget.NewSelect(xoscillator.DetuneCurveNames(), func(s string) {
        ui.params.DetuneCurve = s
        if !ui.applyingState {
            ui.regenerateNote()
        }
    })
    ui.detuneCurveSelect.SetSelected("Analog Trance Synth")

    ui.detuneValLbl = widget.NewLabel(fmt.Sprintf("%.2f", ui.params.Detune))
    ui.detuneSlider = widget.NewSlider(0.0, 2.0)
    ui.detuneSlider.Step = 0.01
    ui.detuneSlider.SetValue(ui.params.Detune)
    ui.detuneSlider.OnChanged = func(v float64) {
        ui.params.Detune = v
        ui.detuneValLbl.SetText(fmt.Sprintf("%.2f", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.driftValLbl = widget.NewLabel(fmt.Sprintf("%.2f", ui.params.AnalogDrift))
    ui.driftSlider = widget.NewSlider(0.0, 1.0)
    ui.driftSlider.Step = 0.01
    ui.driftSlider.SetValue(ui.params.AnalogDrift)
    ui.driftSlider.OnChanged = func(v float64) {
        ui.params.AnalogDrift = v
        ui.driftValLbl.SetText(fmt.Sprintf("%.2f", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.driftDepthValLbl = widget.NewLabel(fmt.Sprintf("%.2fppt", ui.params.AnalogDriftDepth*1000))
    ui.driftDepthSlider = widget.NewSlider(0.0, 1.0)
    ui.driftDepthSlider.Step = 0.0001
    ui.driftDepthSlider.SetValue(ui.params.AnalogDriftDepth)
    ui.driftDepthSlider.OnChanged = func(v float64) {
        ui.params.AnalogDriftDepth = v
        ui.driftDepthValLbl.SetText(fmt.Sprintf("%.2fppt", v*1000))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.cdmValLbl = widget.NewLabel(fmt.Sprintf("%.2fms", ui.params.CenterDelayMs))
    ui.cdmSlider = widget.NewSlider(0.0, 1.0)
    ui.cdmSlider.Step = 0.01
    ui.cdmSlider.SetValue(ui.params.CenterDelayMs)
    ui.cdmSlider.OnChanged = func(v float64) {
        ui.params.CenterDelayMs = v
        ui.cdmValLbl.SetText(fmt.Sprintf("%.2fms", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.rateValLbl = widget.NewLabel(fmt.Sprintf("%.2fHz", ui.params.DtoRate))
    ui.rateSlider = widget.NewSlider(0.0, 1.0)
    ui.rateSlider.Step = 0.01
    ui.rateSlider.SetValue(ui.params.DtoRate)
    ui.rateSlider.OnChanged = func(v float64) {
        ui.params.DtoRate = v
        ui.rateValLbl.SetText(fmt.Sprintf("%.2fHz", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.soctValLbl = widget.NewLabel(fmt.Sprintf("%.3f", ui.params.SubOctave))
    ui.soctSlider = widget.NewSlider(-10.0, 10.0)
    ui.soctSlider.Step = 0.01
    ui.soctSlider.SetValue(ui.params.SubOctave)
    ui.soctSlider.OnChanged = func(v float64) {
        ui.params.SubOctave = v
        ui.soctValLbl.SetText(fmt.Sprintf("%.3f", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    dtoContent := container.NewVBox(
        container.NewGridWithColumns(2,
            xfyne.NewVariableGrid([]float32{0.3, 0.7},
                widget.NewLabel("Detune Preset"), ui.dtoSelect,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Voices"), makeHSliderPlusMinus(ui.voicesSlider), ui.voicesValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Detune Amount"), makeHSliderPlusMinus(ui.detuneSlider), ui.detuneValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.7},
                widget.NewLabel("Detune Curve"), ui.detuneCurveSelect,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Detune Mix"), makeHSliderPlusMinus(ui.mixSlider), ui.mixValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Detune Rate"), makeHSliderPlusMinus(ui.rateSlider), ui.rateValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Stereo Width"), makeHSliderPlusMinus(ui.widthSlider), ui.widthValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Centre Delay"), makeHSliderPlusMinus(ui.cdmSlider), ui.cdmValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Analog Drift"), makeHSliderPlusMinus(ui.driftSlider), ui.driftValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Drift Depth"), makeHSliderPlusMinus(ui.driftDepthSlider), ui.driftDepthValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Sub-Octaves"), makeHSliderPlusMinus(ui.soctSlider), ui.soctValLbl,
            ),
        ),
    )
    ui.colDTO = newAutoFitAccordion(
        func() { ui.fitWindowToContent() },
        "Detune/Oversample",
        dtoContent,
    )

}

func (ui *synthUI) buildAsdrUI() {
    ui.asdrCheck = widget.NewCheck("ASDR Enable", func(checked bool) {
        ui.params.AsdrEnable = checked
        if !ui.applyingState {
            ui.regenerateNote()
        }
    })

    ui.delayValLbl = widget.NewLabel(fmt.Sprintf("%.3fs", ui.params.Delay))
    ui.delaySlider = widget.NewSlider(0.001, 10.0)
    ui.delaySlider.Step = 0.005
    ui.delaySlider.SetValue(ui.params.Delay)
    ui.delaySlider.OnChanged = func(v float64) {
        ui.params.Delay = v
        ui.delayValLbl.SetText(fmt.Sprintf("%.3fs", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.holdValLbl = widget.NewLabel(fmt.Sprintf("%.3fs", ui.params.Hold))
    ui.holdSlider = widget.NewSlider(0.001, 10.0)
    ui.holdSlider.Step = 0.005
    ui.holdSlider.SetValue(ui.params.Hold)
    ui.holdSlider.OnChanged = func(v float64) {
        ui.params.Hold = v
        ui.holdValLbl.SetText(fmt.Sprintf("%.3fs", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.attackValLbl = widget.NewLabel(fmt.Sprintf("%.3fs", ui.params.Attack))
    ui.attackSlider = widget.NewSlider(0.001, 2.0)
    ui.attackSlider.Step = 0.005
    ui.attackSlider.SetValue(ui.params.Attack)
    ui.attackSlider.OnChanged = func(v float64) {
        ui.params.Attack = v
        ui.attackValLbl.SetText(fmt.Sprintf("%.3fs", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.decayValLbl = widget.NewLabel(fmt.Sprintf("%.2fs", ui.params.Decay))
    ui.decaySlider = widget.NewSlider(0.01, 3.0)
    ui.decaySlider.Step = 0.01
    ui.decaySlider.SetValue(ui.params.Decay)
    ui.decaySlider.OnChanged = func(v float64) {
        ui.params.Decay = v
        ui.decayValLbl.SetText(fmt.Sprintf("%.2fs", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.sustainValLbl = widget.NewLabel(fmt.Sprintf("%.2f%%", (ui.params.Sustain * 100.0)))
    ui.sustainSlider = widget.NewSlider(0.0, 1.0)
    ui.sustainSlider.Step = 0.01
    ui.sustainSlider.SetValue(ui.params.Sustain)
    ui.sustainSlider.OnChanged = func(v float64) {
        ui.params.Sustain = v
        ui.sustainValLbl.SetText(fmt.Sprintf("%.2f%%", (v * 100.0)))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.releaseValLbl = widget.NewLabel(fmt.Sprintf("%.2fs", ui.params.Release))
    ui.releaseSlider = widget.NewSlider(0.01, 3.0)
    ui.releaseSlider.Step = 0.01
    ui.releaseSlider.SetValue(ui.params.Release)
    ui.releaseSlider.OnChanged = func(v float64) {
        ui.params.Release = v
        ui.releaseValLbl.SetText(fmt.Sprintf("%.2fs", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    adsrContent := container.NewVBox(
        ui.asdrCheck,
        container.NewGridWithColumns(2,
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Delay"), makeHSliderPlusMinus(ui.delaySlider), ui.delayValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Attack"), makeHSliderPlusMinus(ui.attackSlider), ui.attackValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Hold"), makeHSliderPlusMinus(ui.holdSlider), ui.holdValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Decay"), makeHSliderPlusMinus(ui.decaySlider), ui.decayValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Sustain"), makeHSliderPlusMinus(ui.sustainSlider), ui.sustainValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Release"), makeHSliderPlusMinus(ui.releaseSlider), ui.releaseValLbl,
            ),
        ),
    )

    ui.colADSR = newAutoFitAccordion(
        func() { ui.fitWindowToContent() },
        "ADSR/Envelope",
        adsrContent,
    )

}

func (ui *synthUI) buildFmWaveUI() {
    ui.toneValLbl = widget.NewLabel(fmt.Sprintf("%.2f", ui.params.Tone))
    ui.toneSlider = widget.NewSlider(0.0, 6.0)
    ui.toneSlider.Step = 0.01
    ui.toneSlider.SetValue(ui.params.Tone)
    ui.toneSlider.OnChanged = func(v float64) {
        ui.params.Tone = v
        ui.toneValLbl.SetText(fmt.Sprintf("%.2f", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.modRatioValLbl = widget.NewLabel(fmt.Sprintf("%.1f", ui.params.ModRatio))
    ui.modRatioSlider = widget.NewSlider(0.00, 5.0)
    ui.modRatioSlider.Step = 0.1
    ui.modRatioSlider.SetValue(ui.params.ModRatio)
    ui.modRatioSlider.OnChanged = func(v float64) {
        ui.params.ModRatio = v
        ui.modRatioValLbl.SetText(fmt.Sprintf("%.1f", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.modIndexValLbl = widget.NewLabel(fmt.Sprintf("%.2f", ui.params.ModIndex))
    ui.modIndexSlider = widget.NewSlider(0.0, 20.0)
    ui.modIndexSlider.Step = 0.01
    ui.modIndexSlider.SetValue(ui.params.ModIndex)
    ui.modIndexSlider.OnChanged = func(v float64) {
        ui.params.ModIndex = v
        ui.modIndexValLbl.SetText(fmt.Sprintf("%.2f", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    wfnames := make([]string, 0)
    wfnames = append(wfnames, xoscillator.WaveformNames()...)
    wfnames = append(wfnames, xoscillator.WaveSamplerNames()...)

    ui.fm1Select = widget.NewSelect(wfnames, func(s string) {
        ui.params.Fm1Wave = s
        if !ui.applyingState {
            ui.regenerateNote()
        }
    })
    ui.fm1Select.SetSelected(wfnames[0])

    ui.fm2Select = widget.NewSelect(wfnames, func(s string) {
        ui.params.Fm2Wave = s
        if !ui.applyingState {
            ui.regenerateNote()
        }
    })
    ui.fm2Select.SetSelected(wfnames[0])

    ui.fmWaveComposer = container.NewVBox(
        widget.NewLabelWithStyle("FM Waveform", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
        container.NewGridWithColumns(2,
            xfyne.NewVariableGrid([]float32{0.3, 0.7},
                widget.NewLabel("FM Carrier"), ui.fm1Select,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.7},
                widget.NewLabel("FM Modulator"), ui.fm2Select,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("FM Mod Ratio"), makeHSliderPlusMinus(ui.modRatioSlider), ui.modRatioValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("FM Mod Index"), makeHSliderPlusMinus(ui.modIndexSlider), ui.modIndexValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("FM Tone"), makeHSliderPlusMinus(ui.toneSlider), ui.toneValLbl,
            ),
        ),
    )

}

func (ui *synthUI) buildBaseWaveUI() {
    // waveforms
    wfnames := make([]string, 0)
    wfnames = append(wfnames, xoscillator.WaveformNames()...)
    wfnames = append(wfnames, xoscillator.WaveSamplerNames()...)
    wfnames = append(wfnames, "fm", "fm2")

    ui.waveSelect = widget.NewSelect(wfnames, func(s string) {
        ui.params.BaseWave = s
        if !ui.applyingState {
            ui.regenerateNote()
        }
    })
    ui.waveSelect.SetSelected(wfnames[0])

    // Sample rate
    ui.srSelect = widget.NewSelect([]string{"8 kHz", "16 kHz", "32 kHz", "48 kHz"}, func(s string) {
        khz, err := strconv.Atoi(strings.TrimSuffix(s, " kHz"))
        if err == nil {
            ui.params.SampleRate = khz * 1000
            if !ui.applyingState {
                ui.regenerateNote()
            }
        }
    })
    ui.srSelect.SetSelected("48 kHz")

    // sample length/duration
    ui.lengthValLbl = widget.NewLabel(fmt.Sprintf("%.0f ms", ui.params.LengthMs))
    ui.lengthSlider = widget.NewSlider(100, 30000)
    ui.lengthSlider.Step = 100
    ui.lengthSlider.SetValue(ui.params.LengthMs)
    ui.lengthSlider.OnChanged = func(v float64) {
        ui.params.LengthMs = v
        ui.lengthValLbl.SetText(fmt.Sprintf("%.0f ms", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    // tune/note
    ui.tuneValLbl = widget.NewLabel(fmt.Sprintf("%.2f Hz", ui.params.Tune))
    ui.tuneSlider = widget.NewSlider(5.0, 5000.0)
    ui.tuneSlider.SetValue(ui.params.Tune)
    ui.tuneSlider.OnChanged = func(v float64) {
        ui.params.Tune = v
        ui.tuneValLbl.SetText(fmt.Sprintf("%.2f Hz", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    // pulse width
    ui.pulseWidthValLbl = widget.NewLabel(fmt.Sprintf("%.2f", ui.params.PulseWidth))
    ui.pulseWidthSlider = widget.NewSlider(0.05, 1.0)
    ui.pulseWidthSlider.Step = 0.01
    ui.pulseWidthSlider.SetValue(ui.params.PulseWidth)
    ui.pulseWidthSlider.OnChanged = func(v float64) {
        ui.params.PulseWidth = v
        ui.pulseWidthValLbl.SetText(fmt.Sprintf("%.2f", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.snappyValLbl = widget.NewLabel(fmt.Sprintf("%.2f", ui.params.Snappy))
    ui.snappySlider = widget.NewSlider(0.0, 10.0)
    ui.snappySlider.Step = 0.01
    ui.snappySlider.SetValue(ui.params.Snappy)
    ui.snappySlider.OnChanged = func(v float64) {
        ui.params.Snappy = v
        ui.snappyValLbl.SetText(fmt.Sprintf("%.2f", v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    // normalization
    formatNormLabel := func(v float64) string {
        if v <= 0 {
            return "Off"
        }
        return fmt.Sprintf("%.0f%%", v)
    }

    ui.normValLbl = widget.NewLabel(formatNormLabel(float64(ui.params.NormalizePct)))
    ui.normSlider = widget.NewSlider(0, 100)
    ui.normSlider.Step = 1.0
    ui.normSlider.SetValue(float64(ui.params.NormalizePct))
    ui.normSlider.OnChanged = func(v float64) {
        ui.params.NormalizePct = int64(v)
        ui.normValLbl.SetText(formatNormLabel(v))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    // noise seed
    ui.noiseSeedValLbl = widget.NewLabel(fmt.Sprintf("0x%08X", ui.params.Seed))
    ui.noiseSeedSlider = widget.NewSlider(0, math.MaxInt32)
    ui.noiseSeedSlider.Step = 1.0
    ui.noiseSeedSlider.SetValue(float64(ui.params.Seed))
    ui.noiseSeedSlider.OnChanged = func(v float64) {
        ui.params.Seed = int32(v)
        ui.noiseSeedValLbl.SetText(fmt.Sprintf("0x%08X", int32(v)))
        if !ui.applyingState {
            ui.regenerateNote()
        }
    }

    ui.baseWaveComposer = container.NewVBox(
        widget.NewLabelWithStyle("Base Waveform", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
        container.NewGridWithColumns(2,
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Sample Rate"), ui.srSelect, widget.NewLabel(""),
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Waveform"), ui.waveSelect, widget.NewLabel(""),
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Sample Length"), makeHSliderPlusMinus(ui.lengthSlider), ui.lengthValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Tune"), makeHSliderPlusMinus(ui.tuneSlider), ui.tuneValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Pulse Width"), makeHSliderPlusMinus(ui.pulseWidthSlider), ui.pulseWidthValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Normalize"), makeHSliderPlusMinus(ui.normSlider), ui.normValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Seed"), makeHSliderPlusMinus(ui.noiseSeedSlider), ui.noiseSeedValLbl,
            ),
            xfyne.NewVariableGrid([]float32{0.3, 0.5, 0.2},
                widget.NewLabel("Snappyness"), makeHSliderPlusMinus(ui.snappySlider), ui.snappyValLbl,
            ),

        ),
    )
}

func (ui *synthUI) doFileDialog(title, path, deffile, extdescr, ext, action string, sel func(p string), nsel func()) {
    _dialog := xfyne.NewSelectDialog(title, path, false, sel, nsel)
    _dialog.SetPreferredDirs(
        xdg.DocumentsPath("Samples"),
        xdg.DocumentsPath("Music"),
        xdg.DocumentsDir(),
        xdg.ProjectsPath("Samples"),
        xdg.ProjectsPath("Music"),
        xdg.ProjectsDir(),
        xdg.DownloadDir(),
        xdg.DesktopDir())

    _dialog.SetAction(action)
    _dialog.SetFileName(deffile)
    filter := storage.NewExtensionFileFilter([]string{ext})
    _dialog.AddFileTypeFilter(extdescr, filter)
    _dialog.Show()
}

func (ui *synthUI) savePresets() {
    ui.doFileDialog("Select File to Save Preset ...",
        ui.userCurrentPath,
        fmt.Sprintf("%s_%dHz_%dpw_0x%08X_%s.preset", ui.params.BaseWave, int(ui.params.Tune), int(ui.params.PulseWidth*100), ui.params.Seed, ui.params.DtoStage),
        "Preset Files (*.preset)",
        ".preset",
        "Save",
        func(outPath string) {
            ui.userCurrentPath = filepath.Dir(outPath)

            if strings.ToLower(filepath.Ext(outPath)) != ".preset" {
                outPath += ".preset"
            }

            bytes, err := json.Marshal(ui.params)
            if err != nil {
                ui.statusLabel.SetText("Marshal error: " + err.Error())
                return
            }

            err = os.WriteFile(outPath, bytes, 0640)
            if err != nil {
                ui.statusLabel.SetText("Save error: " + err.Error())
                return
            }

            ui.statusLabel.SetText("Saved " + outPath)

        },
        func() {
            ui.statusLabel.SetText("Save dialog error: Dialog cancelled")
        },
    )
}

func (ui *synthUI) loadPresets() {
    ui.doFileDialog("Select File to Load Preset ...",
        ui.userCurrentPath,
        "",
        "Preset Files (*.preset)",
        ".preset",
        "Load",
        func(inPath string) {
            ui.userCurrentPath = filepath.Dir(inPath)
            bytes, err := os.ReadFile(inPath)
            if err != nil {
                ui.statusLabel.SetText("Load error: " + err.Error())
                return
            }

            err = json.Unmarshal(bytes, &ui.params)
            if err != nil {
                ui.statusLabel.SetText("UnMarshal error: " + err.Error())
                return
            }

            ui.syncParamsToUI(ui.params)
            if !ui.applyingState {
                ui.regenerateNote()
            }
            ui.statusLabel.SetText("Load " + inPath)
        },
        func() {
            ui.statusLabel.SetText("Load dialog error: Dialog cancelled")
        },
    )

}

func main() {
    ui := newSynthUI()
    ui.window.ShowAndRun()
}
