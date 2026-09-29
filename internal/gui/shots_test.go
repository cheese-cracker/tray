package gui

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"

	"github.com/cheese-cracker/tray/internal/core"
)

// TestShots writes full-size captures of every screen in both variants to TRAY_SHOTS,
// for looking at the app without a display. It asserts nothing and is skipped otherwise.
func TestShots(t *testing.T) {
	dir := os.Getenv("TRAY_SHOTS")
	if dir == "" {
		t.Skip("set TRAY_SHOTS=<dir> to write screen captures")
	}
	seed := func() []core.Task {
		a := garage("the billing page is slow on first load", "work")
		b := garage("ask whether the offsite dates are fixed yet")
		b.Note = "ask in the standup"
		c := garage("Call mom")
		c.Wait, c.Priority = "2026-09-30", "H"
		d := tray("Rotate the api keys")
		d.Priority, d.Due, d.Tags = "H", "2026-09-28", []string{"infra"}
		e := tray("Book the return flight")
		e.Priority, e.Due, e.Tags = "L", "2026-11-20", []string{"travel"}
		f := tray("Write the quarterly summary")
		f.Tags = []string{"work"}
		g := tray("Weekly review")
		g.Recur, g.Due, g.Priority, g.Tags = "weekly", "2026-10-03", "M", []string{"ops"}
		h := tray("Renew the TLS certificate")
		h.Priority, h.Done = "H", "2026-09-20"
		return []core.Task{a, b, c, d, e, f, g, h}
	}
	screens := []struct {
		name  string
		setup func(h *harness)
	}{
		{"home-garage", func(h *harness) {}},
		{"home-tray", func(h *harness) { h.u.tabs.SelectIndex(1); h.u.tray.pick(len(h.u.tray.rows) - 1) }},
		{"take", func(h *harness) { h.u.garage.pick(0); h.u.do("t", h.u.garage) }},
		{"review", func(h *harness) { h.u.tabs.SelectIndex(1); h.u.do("v", h.u.tray) }},
		{"review-template", func(h *harness) {
			h.u.tabs.SelectIndex(1)
			h.u.do("v", h.u.tray)
			l := h.u.current()
			l.pick(len(l.rows) - 1)
		}},
		{"sweep", func(h *harness) { h.u.openSweep() }},
		{"syncreview", func(h *harness) { h.u.openSyncReview(reviewResults()) }},
		{"plugins", func(h *harness) { h.install("echo"); h.u.openPlugins() }},
		{"help", func(h *harness) { h.u.do("?", h.u.garage) }},
	}
	for _, v := range []struct {
		name    string
		variant fyne.ThemeVariant
	}{{"light", theme.VariantLight}, {"dark", theme.VariantDark}} {
		for _, sc := range screens {
			t.Run(sc.name+"-"+v.name, func(t *testing.T) {
				testVariant = v.variant
				h := open(t, seed()...)
				sc.setup(h)
				path := filepath.Join(dir, sc.name+"-"+v.name+".png")
				f, err := os.Create(path)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				if err := png.Encode(f, h.w.Canvas().Capture()); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
	testVariant = theme.VariantLight
}
