package gui

import (
	"fmt"
	"image/color"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/test"

	"github.com/charmbracelet/lipgloss"

	"github.com/cheese-cracker/tray/internal/style"
)

// TestEveryColourIsThePalettes walks every screen and fails on a shape or a text drawn
// in a colour the palette does not own (102b). The colours were never wrong in code
// before — they were absent from it, which is what a review misses and a regression
// restores. Alpha is ignored: a shade or a fade is the palette's colour, thinner.
func TestEveryColourIsThePalettes(t *testing.T) {
	owned := map[[3]uint8]string{}
	for name, c := range map[string]lipgloss.AdaptiveColor{
		"Paper": style.Paper, "Card": style.Card, "Line": style.Line, "Ink": style.Ink, "Ink2": style.Ink2,
		"Accent": style.Accent, "AccentSoft": style.AccentSoft, "Subtle": style.Subtle,
		"Review": style.Review, "ReviewSoft": style.ReviewSoft,
		"High": style.High, "Medium": style.Medium, "Low": style.Low,
	} {
		for _, dark := range []bool{false, true} {
			c := style.RGBA(c, dark)
			owned[[3]uint8{c.R, c.G, c.B}] = name
		}
	}
	check := func(t *testing.T, what string, c color.Color) {
		if c == nil {
			return
		}
		n := color.NRGBAModel.Convert(c).(color.NRGBA)
		if n.A == 0 {
			return
		}
		if _, ok := owned[[3]uint8{n.R, n.G, n.B}]; !ok {
			t.Errorf("%s wears #%02x%02x%02x, which the palette does not own", what, n.R, n.G, n.B)
		}
	}
	for _, sc := range []struct {
		name  string
		setup func(h *harness)
	}{
		{"garage", func(h *harness) {}},
		{"tray", func(h *harness) { h.u.tabs.SelectIndex(1) }},
		{"take", func(h *harness) { test.Type(h.u.garage, "t") }},
		{"help", func(h *harness) { test.Type(h.u.garage, "?") }},
		{"review", func(h *harness) { h.u.tabs.SelectIndex(1); test.Type(h.u.tray, "v") }},
		{"sweep", func(h *harness) { h.u.openSweep() }},
		{"syncreview", func(h *harness) { h.u.openSyncReview(reviewResults()) }},
		{"plugins", func(h *harness) { h.install("echo"); h.u.openPlugins() }},
	} {
		t.Run(sc.name, func(t *testing.T) {
			h := open(t, seedScreens()...)
			sc.setup(h)
			for _, o := range test.LaidOutObjects(h.w.Canvas().Content()) {
				what := fmt.Sprintf("%T", o)
				switch v := o.(type) {
				case *canvas.Text:
					check(t, what+" "+v.Text, v.Color)
				case *canvas.Rectangle:
					check(t, what, v.FillColor)
					check(t, what+" stroke", v.StrokeColor)
				case *canvas.Circle:
					check(t, what, v.FillColor)
					check(t, what+" stroke", v.StrokeColor)
				case *canvas.Line:
					check(t, what, v.StrokeColor)
				}
			}
			for _, o := range h.w.Canvas().Overlays().List() {
				for _, in := range test.LaidOutObjects(o) {
					if v, ok := in.(*canvas.Rectangle); ok {
						check(t, fmt.Sprintf("overlay %T", o), v.FillColor)
					}
				}
			}
		})
	}
}

var _ fyne.CanvasObject // keep the import honest if the walk changes shape
