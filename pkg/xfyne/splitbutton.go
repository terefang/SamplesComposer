// Package xfyne provides extended composite widgets built on top of Fyne.
package xfyne

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// SplitButton pairs a primary action button with a secondary dropdown trigger
// button in a horizontal layout. Clicking the primary button invokes OnTapped,
// while clicking the dropdown button opens Menu anchored beneath the widget.
//
// Embeds widget.BaseWidget and relies on ExtendBaseWidget during construction.
type SplitButton struct {
	widget.BaseWidget

	// Text is the label displayed on the primary action button.
	// If modified directly rather than via SetText, call Refresh to update the UI.
	Text string

	// Icon is an optional resource displayed alongside Text on the primary button.
	// If modified directly rather than via SetIcon, call Refresh to update the UI.
	Icon fyne.Resource

	// Importance controls the visual emphasis (e.g., HighImportance, DangerImportance)
	// applied to both the primary button and the dropdown button on Refresh.
	Importance widget.Importance

	// Menu is the popup menu shown when the dropdown button is pressed or ShowMenu
	// is called. If nil or empty, ShowMenu does nothing.
	Menu *fyne.Menu

	// OnTapped is called when the primary button is clicked, if non-nil.
	OnTapped func()

	// disabled tracks whether the split button and its actions are disabled.
	disabled bool

	// primaryBtn is the main left-hand action button.
	primaryBtn *widget.Button

	// menuBtn is the right-hand dropdown arrow button that opens Menu.
	menuBtn *widget.Button
}

// NewSplitButton constructs a SplitButton with the given primary label, primary
// click callback, and optional popup menu items. It initializes the underlying
// buttons and registers the widget via ExtendBaseWidget.
func NewSplitButton(label string, onTapped func(), items ...*fyne.MenuItem) *SplitButton {
	return NewSplitButtonWithIcon(label, theme.MenuDropDownIcon(), onTapped, items...)
}

func NewSplitButtonWithDDD(label string, onTapped func(), items ...*fyne.MenuItem) *SplitButton {
	return NewSplitButtonWithIcon(label, theme.MoreHorizontalIcon(), onTapped, items...)
}

func NewSplitButtonWithIcon(label string, icon fyne.Resource, onTapped func(), items ...*fyne.MenuItem) *SplitButton {
	sb := &SplitButton{
		Text:     label,
		OnTapped: onTapped,
		Menu:     fyne.NewMenu("", items...),
		disabled: false,
	}

	// Delegate primary button taps to OnTapped if set and enabled.
	sb.primaryBtn = widget.NewButton(label, func() {
		if !sb.disabled && sb.OnTapped != nil {
			sb.OnTapped()
		}
	})

	// Delegate dropdown button taps to ShowMenu when enabled.
	sb.menuBtn = widget.NewButtonWithIcon("", icon, func() {
		//if !sb.disabled {
		sb.ShowMenu()
		//}
	})

	sb.ExtendBaseWidget(sb)
	return sb
}

// Disable disables the SplitButton so neither the primary action nor the
// dropdown menu can be triggered. It updates both underlying buttons and
// refreshes the widget.
func (sb *SplitButton) Disable() {
	sb.disabled = true
	if sb.primaryBtn != nil {
		sb.primaryBtn.Disable()
	}
	if sb.menuBtn != nil {
		sb.menuBtn.Disable()
	}
	sb.Refresh()
}

// Enable re-enables the SplitButton so both the primary action and the
// dropdown menu can be triggered. It updates both underlying buttons and
// refreshes the widget.
func (sb *SplitButton) Enable() {
	sb.disabled = false
	if sb.primaryBtn != nil {
		sb.primaryBtn.Enable()
	}
	if sb.menuBtn != nil {
		sb.menuBtn.Enable()
	}
	sb.Refresh()
}

// Disabled reports whether the SplitButton is currently disabled.
func (sb *SplitButton) Disabled() bool {
	return sb.disabled
}

// SetText updates the primary button label and triggers a widget refresh.
func (sb *SplitButton) SetText(text string) {
	sb.Text = text
	if sb.primaryBtn != nil {
		sb.primaryBtn.SetText(text)
	}
	sb.Refresh()
}

// SetIcon updates the primary button icon and triggers a widget refresh.
func (sb *SplitButton) SetIcon(icon fyne.Resource) {
	sb.Icon = icon
	if sb.primaryBtn != nil {
		sb.primaryBtn.SetIcon(icon)
	}
	sb.Refresh()
}

// ShowMenu displays Menu in a popup positioned immediately below the SplitButton
// in canvas coordinates. It is a no-op if the widget is disabled, if Menu has
// no items, or if the widget is not yet attached to a canvas.
func (sb *SplitButton) ShowMenu() {
	if sb.disabled || sb.Menu == nil || len(sb.Menu.Items) == 0 {
		return
	}

	// Resolve the canvas and absolute position so the popup aligns beneath the widget.
	driver := fyne.CurrentApp().Driver()
	canvas := driver.CanvasForObject(sb)
	if canvas == nil {
		return
	}

	pos := driver.AbsolutePositionForObject(sb)
	size := sb.Size()

	popup := widget.NewPopUpMenu(sb.Menu, canvas)
	popup.ShowAtPosition(fyne.NewPos(pos.X, pos.Y+size.Height))
}

// CreateRenderer builds and returns the WidgetRenderer for SplitButton, arranging
// primaryBtn and menuBtn side-by-side inside a horizontal box container.
func (sb *SplitButton) CreateRenderer() fyne.WidgetRenderer {
	sb.syncButtonState()
	row := container.NewHBox(sb.primaryBtn, sb.menuBtn)
	return &splitButtonRenderer{
		sb:      sb,
		content: row,
		objects: []fyne.CanvasObject{row},
	}
}

// syncButtonState copies exported configuration fields and disabled state onto
// primaryBtn and menuBtn prior to rendering or refreshing.
func (sb *SplitButton) syncButtonState() {
	if sb.primaryBtn != nil {
		sb.primaryBtn.Text = sb.Text
		sb.primaryBtn.Icon = sb.Icon
		sb.primaryBtn.Importance = sb.Importance
		if sb.disabled {
			sb.primaryBtn.Disable()
		} else {
			sb.primaryBtn.Enable()
		}
	}
	if sb.menuBtn != nil {
		sb.menuBtn.Importance = sb.Importance
		// Preserve any custom icon assigned to menuBtn; default to the dropdown arrow.
		if sb.menuBtn.Icon == nil {
			sb.menuBtn.SetIcon(theme.MenuDropDownIcon())
		}
		if sb.disabled {
			sb.menuBtn.Disable()
		} else {
			sb.menuBtn.Enable()
		}
	}
}

// splitButtonRenderer manages layout, sizing, and refresh lifecycle for SplitButton.
type splitButtonRenderer struct {
	sb      *SplitButton
	content *fyne.Container
	objects []fyne.CanvasObject
}

// Layout sizes and positions the horizontal button row to fill the widget bounds.
func (r *splitButtonRenderer) Layout(size fyne.Size) {
	r.content.Resize(size)
	r.content.Move(fyne.NewPos(0, 0))
}

// MinSize returns the minimum size required by the horizontal button row.
func (r *splitButtonRenderer) MinSize() fyne.Size {
	return r.content.MinSize()
}

// Refresh synchronizes child button state and refreshes both buttons and the row.
func (r *splitButtonRenderer) Refresh() {
	r.sb.syncButtonState()
	if r.sb.primaryBtn != nil {
		r.sb.primaryBtn.Refresh()
	}
	if r.sb.menuBtn != nil {
		r.sb.menuBtn.Refresh()
	}
	r.content.Refresh()
}

// Objects returns the top-level canvas objects managed by this renderer.
func (r *splitButtonRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

// Destroy cleans up renderer resources (no-op for this composite widget).
func (r *splitButtonRenderer) Destroy() {}
