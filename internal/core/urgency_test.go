package core

import (
	"math"
	"testing"
	"time"
)

func day(s string) time.Time {
	d, _ := time.ParseInLocation(DateLayout, s, time.UTC)
	return d
}

func TestDueRamp(t *testing.T) {
	today := day("2026-08-07")
	cases := []struct {
		due  string
		want float64
	}{
		{"", 0},
		{"2026-08-01", 1.0}, // overdue
		{"2026-08-07", 1.0}, // today
		{"2026-08-14", 0.8}, // a week
		{"2026-08-21", 0.4}, // a fortnight
		{"2026-09-04", 0.0}, // four weeks
		{"2027-01-01", 0.0}, // far out
		{"not-a-date", 0},
	}
	for _, c := range cases {
		if got := DueRamp(c.due, today); math.Abs(got-c.want) > 0.001 {
			t.Errorf("DueRamp(%q) = %v, want %v", c.due, got, c.want)
		}
	}
}

func TestUrgencyOrdering(t *testing.T) {
	today := day("2026-08-07")
	urgent, _ := Parse("- [ ] Urgent thing priority:H due:2026-08-08 entry:2026-08-07", 0)
	middle, _ := Parse("- [ ] Middle thing priority:M entry:2026-08-07", 0)
	low, _ := Parse("- [ ] Low thing priority:L entry:2026-08-07", 0)

	if !(Urgency(urgent, today) > Urgency(middle, today)) {
		t.Errorf("H+due must outrank M: %v vs %v", Urgency(urgent, today), Urgency(middle, today))
	}
	if !(Urgency(middle, today) > Urgency(low, today)) {
		t.Errorf("M must outrank L: %v vs %v", Urgency(middle, today), Urgency(low, today))
	}
}

func TestUrgencyTerms(t *testing.T) {
	today := day("2026-08-07")

	// Priority alone: 6.0 * 1.0, no tags, no due, no project, no age.
	bare, _ := Parse("- [ ] Thing priority:H", 0)
	if got := Urgency(bare, today); math.Abs(got-6.0) > 0.001 {
		t.Errorf("priority H alone = %v, want 6.0", got)
	}

	// Tags are count-damped: one tag is worth 0.8, two 0.9, three or more 1.0.
	for _, c := range []struct {
		line string
		want float64
	}{
		{"- [ ] Thing +a", 0.8},
		{"- [ ] Thing +a +b", 0.9},
		{"- [ ] Thing +a +b +c", 1.0},
	} {
		task, _ := Parse(c.line, 0)
		if got := Urgency(task, today); math.Abs(got-c.want) > 0.001 {
			t.Errorf("%q = %v, want %v", c.line, got, c.want)
		}
	}

	// Age subtracts, capped at a year.
	old, _ := Parse("- [ ] Thing entry:2025-08-07", 0)
	if got := Urgency(old, today); math.Abs(got-(-2.0)) > 0.01 {
		t.Errorf("a year old = %v, want -2.0", got)
	}
	ancient, _ := Parse("- [ ] Thing entry:2020-01-01", 0)
	if got := Urgency(ancient, today); math.Abs(got-(-2.0)) > 0.01 {
		t.Errorf("age is capped at a year: %v", got)
	}
}

func TestQuadrant(t *testing.T) {
	today := day("2026-08-07")
	cases := []struct {
		line string
		want string
	}{
		{"- [ ] a priority:H due:2026-08-08", "Q1"},
		{"- [ ] a priority:H", "Q2"},
		{"- [ ] a priority:M", "Q2"},
		{"- [ ] a due:2026-08-08", "Q3"},
		{"- [ ] a priority:L due:2026-08-08", "Q3"},
		{"- [ ] a", "Q4"},
		{"- [ ] a priority:L", "Q4"},
		{"- [ ] a due:2026-12-01", "Q4"}, // a far due date is not urgent
	}
	for _, c := range cases {
		task, _ := Parse(c.line, 0)
		if got := Quadrant(task, today); got != c.want {
			t.Errorf("%q = %s, want %s", c.line, got, c.want)
		}
	}
}

func TestDayAddsTheWeekday(t *testing.T) {
	if got := Day("2026-08-12"); got != "2026-08-12 Wed" {
		t.Errorf("Day = %q, want the weekday alongside", got)
	}
	// Anything unparseable is handed back untouched — a hand-typed line must show
	// as typed rather than vanish.
	for _, odd := range []string{"", "next tuesday", "2026-13-99"} {
		if got := Day(odd); got != odd {
			t.Errorf("Day(%q) = %q, want it unchanged", odd, got)
		}
	}
}
