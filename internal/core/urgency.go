package core

import (
	"math"
	"time"
)

// Taskwarrior's coefficients, over the fields we keep, so an import orders comparably.
const (
	coefPriority = 6.0
	coefDue      = 12.0
	coefTags     = 1.0
	coefAge      = 2.0
	maxAgeDays   = 365.0
)

var priorityWeight = map[string]float64{"H": 1.0, "M": 0.65, "L": 0.3}

const DateLayout = "2006-01-02"

// Date parses YYYY-MM-DD. Anything else is no date, never an error.
func Date(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	d, err := time.ParseInLocation(DateLayout, value, time.UTC)
	if err != nil {
		return time.Time{}, false
	}
	return d, true
}

// Day is a stored date made readable. The weekday is what actually tells you
// whether something is soon; the file keeps plain ISO.
func Day(value string) string {
	d, ok := Date(value)
	if !ok {
		return value
	}
	return value + " " + d.Format("Mon")
}

func daysBetween(from, to time.Time) int {
	a := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
	b := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
	return int(b.Sub(a).Hours() / 24)
}

// DueRamp is 1.0 overdue, 0.8 at a week out, 0.4 at a fortnight, 0 beyond four weeks.
func DueRamp(due string, today time.Time) float64 {
	d, ok := Date(due)
	if !ok {
		return 0
	}
	days := daysBetween(today, d)
	switch {
	case days <= 0:
		return 1.0
	case days <= 7:
		return 1.0 - 0.2*(float64(days)/7)
	case days <= 14:
		return 0.8 - 0.4*(float64(days-7)/7)
	default:
		return math.Max(0, 0.4-0.4*(float64(days-14)/14))
	}
}

func tagDamp(n int) float64 {
	switch n {
	case 0:
		return 0
	case 1:
		return 0.8
	case 2:
		return 0.9
	default:
		return 1.0
	}
}

func Urgency(t Task, today time.Time) float64 {
	score := coefPriority * priorityWeight[t.Priority()]
	score += coefDue * DueRamp(t.Attrs["due"], today)
	score += coefTags * tagDamp(len(t.Tags))
	if entry, ok := Date(t.Attrs["entry"]); ok {
		age := math.Min(float64(daysBetween(entry, today)), maxAgeDays)
		score -= coefAge * age / maxAgeDays
	}
	return math.Round(score*100) / 100
}

// IsUrgent is due inside a week; no due date is never urgent.
func IsUrgent(t Task, today time.Time) bool {
	d, ok := Date(t.Attrs["due"])
	return ok && daysBetween(today, d) <= 7
}

func IsImportant(t Task) bool {
	p := t.Priority()
	return p == "H" || p == "M"
}

func Quadrant(t Task, today time.Time) string {
	urgent, important := IsUrgent(t, today), IsImportant(t)
	switch {
	case urgent && important:
		return "Q1"
	case important:
		return "Q2"
	case urgent:
		return "Q3"
	default:
		return "Q4"
	}
}
