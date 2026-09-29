package core

import (
	"strconv"
	"strings"
	"time"
)

// Period reads a recur value into a step: daily, weekly, monthly, yearly, or a count
// of days or weeks like 3d or 2w. Anything else is not a period, and a template
// carrying one is left alone rather than guessed at.
func Period(recur string) (func(time.Time) time.Time, bool) {
	switch recur {
	case "daily":
		return days(1), true
	case "weekly":
		return days(7), true
	case "monthly":
		return func(d time.Time) time.Time { return d.AddDate(0, 1, 0) }, true
	case "yearly":
		return func(d time.Time) time.Time { return d.AddDate(1, 0, 0) }, true
	}
	for suffix, unit := range map[string]int{"d": 1, "w": 7} {
		if n, err := strconv.Atoi(strings.TrimSuffix(recur, suffix)); err == nil && n > 0 && strings.HasSuffix(recur, suffix) {
			return days(n * unit), true
		}
	}
	return nil, false
}

func days(n int) func(time.Time) time.Time {
	return func(d time.Time) time.Time { return d.AddDate(0, 0, n) }
}

// ChildOf is the source a template's children carry.
func ChildOf(template Task) string { return "recur:" + template.ID }

// Materialize is the recurrence step: for every template with no live child, one
// child, due on the first occurrence on or after today — and after the last child's
// due, so a period is never served twice. Missed periods are skipped: a backlog of
// weeklies you did not do is noise, and the tray is the small deliberate list.
// children is every child, live or finished; a template that has ended gets none.
func Materialize(templates, children []Task, today time.Time) []Task {
	live := map[string]bool{}
	last := map[string]time.Time{}
	for _, c := range children {
		if !c.Terminal() {
			live[c.Source] = true
		}
		if d, ok := Date(c.Due); ok && d.After(last[c.Source]) {
			last[c.Source] = d
		}
	}
	var out []Task
	for _, tpl := range templates {
		if tpl.Recur == "" || live[ChildOf(tpl)] {
			continue
		}
		step, ok := Period(tpl.Recur)
		if !ok {
			continue
		}
		next, ok := Date(tpl.Due)
		if !ok {
			next = today
		}
		for next.Before(today) || !next.After(last[ChildOf(tpl)]) {
			next = step(next)
		}
		if until, ended := Date(tpl.Until); ended && next.After(until) {
			continue
		}
		out = append(out, Task{
			Layer: LayerTray, Text: tpl.Text, Priority: tpl.Priority, Tags: append([]string{}, tpl.Tags...),
			Note: tpl.Note, Due: next.Format(DateLayout), Entry: today.Format(DateLayout), Source: ChildOf(tpl),
		})
	}
	return out
}
