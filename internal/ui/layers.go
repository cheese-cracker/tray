package ui

import (
	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
)

// A layer is one tab. Tray plus the garage months you can actually act on — last
// month appears only while it still holds live lines, so the month turn announces
// itself by a tab showing up.
type layer struct {
	title string
	month string // "" is the tray; otherwise a month or "someday"
}

func (l layer) isTray() bool { return l.month == "" }

func (l layer) open() (*store.Doc, error) {
	if l.isTray() {
		return store.Tray()
	}
	return store.Garage(l.month)
}

// Two tabs is the whole day-to-day shape: what you're doing, and what you dumped.
// Someday and other months are still reachable through `>`; they just don't earn
// standing room. The sweep is the exception — that ritual is about months.
func layers(sweep bool, closing string) []layer {
	this := store.ThisMonth()
	if !sweep {
		return []layer{
			{title: "tray"},
			{title: monthTitle(this), month: this},
		}
	}
	// Four months and no tray. `--month` replaces the closing tab so a month further
	// back is still reachable; without it the tab is simply the previous one.
	prev := store.PrevMonth(this)
	if closing != "" {
		prev = closing
	}
	out := []layer{{title: monthTitle(prev), month: prev}}
	if prev != this {
		out = append(out, layer{title: monthTitle(this), month: this})
	}
	next := store.NextMonth(this)
	return append(out,
		layer{title: monthTitle(next), month: next},
		layer{title: store.Someday, month: store.Someday})
}

// sweepStart is the tab the sweep opens on: the current month, always. Which month is
// "closing" depends on whether you sweep on the 30th or the 10th, so there is no
// honest way to guess it — and landing somewhere predictable beats landing somewhere
// clever that is sometimes wrong.
func sweepStart(tabs []layer) int {
	for i, l := range tabs {
		if l.month == store.ThisMonth() {
			return i
		}
	}
	return 0
}

func monthTitle(month string) string {
	if d, ok := core.Date(month + "-01"); ok {
		return d.Format("January")
	}
	return month
}

func liveIn(month string) int {
	doc, err := store.Garage(month)
	if err != nil {
		return 0
	}
	n := 0
	for _, t := range doc.Live() {
		if t.Parsed() {
			n++
		}
	}
	return n
}

// destinations are where `>` can send the selection: every other layer, plus next
// month, which is what carrying forward means.
func (m Model) destinations() []layer {
	this := store.ThisMonth()
	all := []layer{
		{title: "tray"},
		{title: monthTitle(this), month: this},
		{title: monthTitle(store.NextMonth(this)), month: store.NextMonth(this)},
		{title: store.Someday, month: store.Someday},
	}
	if last := store.PrevMonth(this); liveIn(last) > 0 {
		all = append(all, layer{title: monthTitle(last), month: last})
	}

	var out []layer
	seen := map[string]bool{m.layer().month: true}
	for _, l := range all {
		if seen[l.month] {
			continue
		}
		seen[l.month] = true
		out = append(out, l)
	}
	return out
}

// move is take, hand back and carry forward at once: the source line stays with an
// arrow to where its copy went.
func (m *Model) move(picked []core.Task, to layer) string {
	from, err := m.layer().open()
	if err != nil {
		return err.Error()
	}
	dst, err := to.open()
	if err != nil {
		return err.Error()
	}

	provenance := m.layer().month
	seen := dst.LiveTexts()
	moved := 0
	for _, t := range picked {
		switch {
		case t.Text == "" || seen[t.Text]: // already live there
		case !to.isTray() && dst.Reclaim(t): // it came from there: bring it home as it is
			seen[t.Text] = true
			moved++
		case to.isTray():
			dst.Add(core.Arrive(t, provenance, m.today))
			seen[t.Text] = true
			moved++
		default:
			dst.Add(t.Copy()) // between garage months nothing is being structured
			seen[t.Text] = true
			moved++
		}

		// The source is dealt with either way — skipping this is what stranded
		// tasks on the tray with the garage still pointing at them.
		if m.layer().isTray() {
			from.Remove(t) // the tray is a worklist, not a record: it releases
		} else {
			core.Depart(&t, arrow(to))
			from.Set(t)
		}
	}
	if err := dst.Save(); err != nil {
		return err.Error()
	}
	if err := from.Save(); err != nil {
		return err.Error()
	}
	return plural(moved, "→ "+to.title)
}

func arrow(to layer) string {
	if to.isTray() {
		return core.DestTray
	}
	return to.month
}
