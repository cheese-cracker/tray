package sync

import (
	"testing"
	"time"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
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

// A pulled row's note lands with it, and reported again differently it is an update like
// any field. A row that reports no note keeps yours (T44).
func TestAPulledNoteLandsAndUpdates(t *testing.T) {
	s := sandbox(t)
	open := ""
	first := []Row{{Key: "s1", Text: "a session", Done: &open, Note: "began with X"}}
	if _, err := Apply(s, Result{Plugin: "claude", Diff: compare(first, nil, "claude", false)}, nil, time.Second); err != nil {
		t.Fatal(err)
	}
	got := rows(t, s, store.Filter{Layer: core.LayerGarage, Month: "claude"})
	if len(got) != 1 || got[0].Note != "began with X" {
		t.Fatalf("landed = %+v", got)
	}
	if d := compare(first, got, "claude", false); !d.Empty() {
		t.Errorf("the same note is no change, got %+v", d.Updates)
	}
	if d := compare([]Row{{Key: "s1", Text: "a session"}}, got, "claude", false); !d.Empty() {
		t.Errorf("a note left out is not a change, got %+v", d.Updates)
	}

	d := compare([]Row{{Key: "s1", Text: "a session", Note: "then did Y, and Z is open"}}, got, "claude", false)
	if len(d.Updates) != 1 || d.Updates[0].String() != "note began with X → then did Y, and Z is open" {
		t.Fatalf("updates = %+v", d.Updates)
	}
	if old, new := d.Updates[0].Values("note"); old != "began with X" || new != "then did Y, and Z is open" {
		t.Errorf("values = %q → %q, want the whole notes, commas and all", old, new)
	}
	if _, err := Apply(s, Result{Plugin: "claude", Diff: d}, nil, time.Second); err != nil {
		t.Fatal(err)
	}
	if got = rows(t, s, store.Filter{Layer: core.LayerGarage, Month: "claude"}); got[0].Note != "then did Y, and Z is open" {
		t.Errorf("note after update = %q", got[0].Note)
	}
}
