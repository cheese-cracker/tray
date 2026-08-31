package core

import "testing"

func TestDepartLeavesTheLineAndMarksIt(t *testing.T) {
	src, _ := Parse("- add retries to the sync job", 4)
	Depart(&src, "2026-09")
	if src.Text != "add retries to the sync job" {
		t.Error("departing must not touch the text")
	}
	if src.Live() {
		t.Error("a departed line is no longer live, so it can't move twice")
	}
	want := "- add retries to the sync job → 2026-09"
	if got := Line(src, false); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestArriveStamps(t *testing.T) {
	src, _ := Parse("- add metrics to the worker +infra", 0)
	fresh := Arrive(src, "2026-08", day("2026-08-07"))

	if fresh.Attrs["from"] != "2026-08" {
		t.Errorf("from = %q", fresh.Attrs["from"])
	}
	if fresh.Attrs["entry"] != "2026-08-07" {
		t.Errorf("entry = %q", fresh.Attrs["entry"])
	}
	if fresh.Moved != "" {
		t.Error("the copy that travels carries no arrow")
	}
	if len(fresh.Tags) != 1 || fresh.Tags[0] != "infra" {
		t.Errorf("tags should survive the move: %v", fresh.Tags)
	}
}

func TestArriveKeepsExistingProvenance(t *testing.T) {
	// Handing back and taking again must not rewrite where it originally came from.
	src, _ := Parse("- [ ] Fix alerts priority:H from:2026-06 entry:2026-06-01", 0)
	fresh := Arrive(src, "2026-08", day("2026-08-07"))
	if fresh.Attrs["from"] != "2026-06" {
		t.Errorf("from = %q, want the original 2026-06", fresh.Attrs["from"])
	}
	if fresh.Attrs["entry"] != "2026-06-01" {
		t.Errorf("entry = %q, want the original", fresh.Attrs["entry"])
	}
}

func TestFinishIsTerminalInPlace(t *testing.T) {
	task, _ := Parse("- [ ] Renew the TLS certificate priority:H", 2)
	Finish(&task, "done", day("2026-08-07"))
	if !task.Done || task.Live() {
		t.Error("want done and not live")
	}
	want := "- [x] ~~Renew the TLS certificate~~ priority:H done:2026-08-07"
	if got := Line(task, true); got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	other, _ := Parse("- [ ] Dead idea", 3)
	Restore(&other)
	if other.Done || !other.Live() {
		t.Error("restoring an open task leaves it open")
	}
}
