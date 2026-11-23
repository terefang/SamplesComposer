package xfyne

import (
    "math"

    "fyne.io/fyne/v2"
    "fyne.io/fyne/v2/container"
    "fyne.io/fyne/v2/theme"
    "fyne.io/fyne/v2/widget"
)

// StepperSlider combines a horizontal slider with "-" and "+" buttons for
// fine-grained step adjustments.
//
// Embeds widget.BaseWidget and relies on ExtendBaseWidget during construction.
type StepperSlider struct {
    widget.BaseWidget

    // Min is the minimum allowed slider value.
    Min float64

    // Max is the maximum allowed slider value.
    Max float64

    // Step is the increment/decrement amount used by the "+" and "-" buttons.
    // If <= 0, a default step of 1 is used.
    Step float64

    // Value is the current slider value, clamped to [Min, Max].
    Value float64

    // Orientation controls whether the slider is horizontal or vertical.
    // Defaults to horizontal.
    Orientation widget.Orientation

    // OnChanged is invoked whenever the value changes from user interaction
    // or SetValue, if non-nil.
    OnChanged func(float64)

    // disabled tracks whether the slider and stepper buttons are disabled.
    disabled bool

    // decBtn decrements Value by Step.
    decBtn *widget.Button

    // incBtn increments Value by Step.
    incBtn *widget.Button

    // slider is the underlying Fyne slider widget.
    slider *widget.Slider
}

// NewStepperSlider creates a StepperSlider with the given minimum, maximum,
// and step size, initialized at min.
func NewStepperSlider(min, max, step, dvalue float64, onchange func(float64)) *StepperSlider {
    if max < min {
        max = min
    }
    if step <= 0 {
        step = 1
    }

    ss := &StepperSlider{
        Min:         min,
        Max:         max,
        Step:        step,
        Value:       dvalue,
        Orientation: widget.Horizontal,
    }

    ss.decBtn = widget.NewButtonWithIcon("", theme.ContentRemoveIcon(), func() {
        ss.Decrement()
    })

    ss.incBtn = widget.NewButtonWithIcon("", theme.ContentAddIcon(), func() {
        ss.Increment()
    })

    ss.slider = widget.NewSlider(min, max)
    ss.slider.Step = step
    ss.slider.Value = min
    ss.slider.OnChanged = func(val float64) {
        if ss.disabled {
            // Revert any programmatic/interaction attempt if disabled.
            ss.slider.SetValue(ss.Value)
            return
        }
        clamped := ss.clamp(val)
        if clamped != val {
            ss.slider.SetValue(clamped)
            return
        }
        if ss.Value != clamped {
            ss.Value = clamped
            ss.updateButtonBounds()
            if ss.OnChanged != nil {
                ss.OnChanged(ss.Value)
            }
        }
    }

    ss.updateButtonBounds()
    ss.ExtendBaseWidget(ss)
    if onchange != nil {
        ss.OnChanged = onchange
    }
    return ss
}

// SetValue sets the slider value, clamping it to [Min, Max], updating the
// underlying slider, and invoking OnChanged if the value changed.
func (ss *StepperSlider) SetValue(val float64) {
    clamped := ss.clamp(val)
    changed := ss.Value != clamped
    ss.Value = clamped

    if ss.slider != nil && ss.slider.Value != clamped {
        ss.slider.SetValue(clamped)
    }
    ss.updateButtonBounds()
    ss.Refresh()

    if changed && ss.OnChanged != nil {
        ss.OnChanged(ss.Value)
    }
}

// Increment increases Value by Step (clamped to Max).
func (ss *StepperSlider) Increment() {
    if ss.disabled {
        return
    }
    ss.SetValue(ss.Value + ss.effectiveStep())
}

// Decrement decreases Value by Step (clamped to Min).
func (ss *StepperSlider) Decrement() {
    if ss.disabled {
        return
    }
    ss.SetValue(ss.Value - ss.effectiveStep())
}

// Disable disables the slider and both "+" and "-" buttons.
func (ss *StepperSlider) Disable() {
    ss.disabled = true
    if ss.slider != nil {
        ss.slider.Disable()
    }
    if ss.decBtn != nil {
        ss.decBtn.Disable()
    }
    if ss.incBtn != nil {
        ss.incBtn.Disable()
    }
    ss.Refresh()
}

// Enable re-enables the slider and stepper buttons according to current bounds.
func (ss *StepperSlider) Enable() {
    ss.disabled = false
    if ss.slider != nil {
        ss.slider.Enable()
    }
    ss.updateButtonBounds()
    ss.Refresh()
}

// Disabled reports whether the StepperSlider is currently disabled.
func (ss *StepperSlider) Disabled() bool {
    return ss.disabled
}

// CreateRenderer builds and returns the WidgetRenderer for StepperSlider,
// placing "-" on the left, the expanding slider in the middle, and "+" on the right.
func (ss *StepperSlider) CreateRenderer() fyne.WidgetRenderer {
    ss.syncState()
    row := container.NewBorder(nil, nil, ss.decBtn, ss.incBtn, ss.slider)
    return &stepperSliderRenderer{
        ss:      ss,
        content: row,
        objects: []fyne.CanvasObject{row},
    }
}

func (ss *StepperSlider) effectiveStep() float64 {
    if ss.Step <= 0 {
        return 1
    }
    return ss.Step
}

func (ss *StepperSlider) clamp(val float64) float64 {
    if math.IsNaN(val) {
        return ss.Min
    }
    if val < ss.Min {
        return ss.Min
    }
    if val > ss.Max {
        return ss.Max
    }
    return val
}

func (ss *StepperSlider) updateButtonBounds() {
    if ss.decBtn != nil {
        if ss.disabled || ss.Value <= ss.Min {
            ss.decBtn.Disable()
        } else {
            ss.decBtn.Enable()
        }
    }
    if ss.incBtn != nil {
        if ss.disabled || ss.Value >= ss.Max {
            ss.incBtn.Disable()
        } else {
            ss.incBtn.Enable()
        }
    }
}

func (ss *StepperSlider) syncState() {
    if ss.Max < ss.Min {
        ss.Max = ss.Min
    }
    ss.Value = ss.clamp(ss.Value)

    if ss.slider != nil {
        ss.slider.Min = ss.Min
        ss.slider.Max = ss.Max
        ss.slider.Step = ss.effectiveStep()
        ss.slider.Orientation = ss.Orientation
        if ss.slider.Value != ss.Value {
            ss.slider.SetValue(ss.Value)
        }
        if ss.disabled {
            ss.slider.Disable()
        } else {
            ss.slider.Enable()
        }
    }

    ss.updateButtonBounds()
}

type stepperSliderRenderer struct {
    ss      *StepperSlider
    content *fyne.Container
    objects []fyne.CanvasObject
}

func (r *stepperSliderRenderer) Layout(size fyne.Size) {
    r.content.Resize(size)
    r.content.Move(fyne.NewPos(0, 0))
}

func (r *stepperSliderRenderer) MinSize() fyne.Size {
    return r.content.MinSize()
}

func (r *stepperSliderRenderer) Refresh() {
    r.ss.syncState()
    if r.ss.decBtn != nil {
        r.ss.decBtn.Refresh()
    }
    if r.ss.incBtn != nil {
        r.ss.incBtn.Refresh()
    }
    if r.ss.slider != nil {
        r.ss.slider.Refresh()
    }
    r.content.Refresh()
}

func (r *stepperSliderRenderer) Objects() []fyne.CanvasObject {
    return r.objects
}

func (r *stepperSliderRenderer) Destroy() {}
