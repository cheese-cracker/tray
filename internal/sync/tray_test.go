package sync

import (
	"testing"

	"github.com/cheese-cracker/tray/internal/core"
)

// The tray is manual: a change a plugin reports for a tray row is noted, never an
// update, so neither the review nor Apply can reach it. A garage row still is.
func TestAChangeAimedAtATrayRowIsKeptNotApplied(t *testing.T) {
	open := ""
	rows := []core.Task{
		{ID: "t1", Layer: core.LayerTray, Text: "on the tray"},
		{ID: "g1", Layer: core.LayerGarage, Month: "2026-08", Text: "in the garage"},
	}
	pull := []Row{
		{Key: "t1", Text: "renamed remotely", Done: &open},
		{Key: "g1", Text: "renamed remotely too", Done: &open},
	}
	d := compare(pull, rows, "remote", true)
	if len(d.Updates) != 1 || d.Updates[0].Old.ID != "g1" {
		t.Errorf("only the garage row should be an update, got %+v", d.Updates)
	}
	if len(d.OnTray) != 1 || d.OnTray[0].Old.ID != "t1" {
		t.Errorf("the tray row should be noted, got %+v", d.OnTray)
	}
	if s := summarize(Result{Diff: d}); s != "0 adds · 1 update · 0 pushes · 1 on the tray, kept" {
		t.Errorf("summary = %q", s)
	}
}
