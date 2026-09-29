package gui

import (
	"flag"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"fyne.io/fyne/v2/test"

	"github.com/cheese-cracker/tray/internal/core"
)

var update = flag.Bool("update", false, "rewrite the screen goldens, then read the diff")

// TestScreens keeps one golden per distinct screen (64). They hold no behaviour: they
// catch the class of thing a flow cannot see — a pane that lost its border, a column
// that stopped aligning. `make golden` rewrites them.
func seedScreens() []core.Task {
	a := tray("Rotate the api keys")
	a.Priority, a.Due, a.Tags = "H", "2026-09-25", []string{"infra"}
	b := tray("Book the return flight")
	b.Due, b.Note = "2026-10-02", "window seat"
	c := garage("the billing page is slow", "work")
	d := garage("ask whether the offsite dates are fixed yet")
	d.Wait = "2026-10-10"
	return []core.Task{a, b, c, d}
}

func TestScreens(t *testing.T) {
	seed := seedScreens
	for _, sc := range []struct {
		name  string
		setup func(h *harness)
	}{
		{"garage.png", func(h *harness) {}},
		{"tray.png", func(h *harness) { h.u.tabs.SelectIndex(1) }},
		{"take.png", func(h *harness) { test.Type(h.u.garage, "t") }},
		{"help.png", func(h *harness) { test.Type(h.u.garage, "?") }},
	} {
		t.Run(sc.name, func(t *testing.T) {
			h := open(t, seed()...)
			sc.setup(h)
			if *update {
				write(t, sc.name, h)
			}
			test.AssertRendersToImage(t, sc.name, h.w.Canvas())
		})
	}
}

func write(t *testing.T, name string, h *harness) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, h.w.Canvas().Capture()); err != nil {
		t.Fatal(err)
	}
}
