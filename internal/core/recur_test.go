package core

import "testing"

func TestPeriodVocabulary(t *testing.T) {
	anchor := day("2026-08-08")
	for recur, want := range map[string]string{
		"daily": "2026-08-09", "weekly": "2026-08-15", "monthly": "2026-09-08", "yearly": "2027-08-08",
		"3d": "2026-08-11", "2w": "2026-08-22",
	} {
		step, ok := Period(recur)
		if !ok {
			t.Fatalf("Period(%s) not recognised", recur)
		}
		if got := step(anchor).Format(DateLayout); got != want {
			t.Errorf("%s from %s = %s, want %s", recur, "2026-08-08", got, want)
		}
	}
	for _, bad := range []string{"fortnightly", "0d", "w", "", "2m"} {
		if _, ok := Period(bad); ok {
			t.Errorf("Period(%q) accepted", bad)
		}
	}
}

// A template with a live child is left alone; one whose child is done gets the next
// occurrence — after the last due, never the same period again, and never a backlog.
func TestMaterializeServesEachPeriodOnce(t *testing.T) {
	tpl := Task{ID: 7, Layer: LayerTray, Text: "Weekly review", Recur: "weekly", Due: "2026-08-08", Priority: "M", Tags: []string{"ops"}}
	today := day("2026-08-07")

	first := Materialize([]Task{tpl}, nil, today)
	if len(first) != 1 || first[0].Due != "2026-08-08" || first[0].Source != "recur:7" || first[0].Priority != "M" {
		t.Fatalf("first child = %+v", first)
	}
	if got := Materialize([]Task{tpl}, first, today); len(got) != 0 {
		t.Errorf("a live child should block a second: %+v", got)
	}

	Finish(&first[0], today) // done before its due, still inside the period
	if got := Materialize([]Task{tpl}, first, today); len(got) != 1 || got[0].Due != "2026-08-15" {
		t.Errorf("same-period child again: %+v", got)
	}

	late := day("2026-09-20") // six weeks of nothing
	if got := Materialize([]Task{tpl}, first, late); len(got) != 1 || got[0].Due != "2026-09-26" {
		t.Errorf("missed periods should be skipped, got %+v", got)
	}

	ended := tpl
	ended.Until = "2026-08-20"
	if got := Materialize([]Task{ended}, first, late); len(got) != 0 {
		t.Errorf("an ended template materialized: %+v", got)
	}
	odd := tpl
	odd.Recur = "fortnightly"
	if got := Materialize([]Task{odd}, nil, today); len(got) != 0 {
		t.Errorf("an unknown period materialized: %+v", got)
	}
}
