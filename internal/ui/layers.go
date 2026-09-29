package ui

import (
	"sort"

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

// name is the layer as the store and a plugin spell it: `tray`, or the month.
func (l layer) name() string {
	if l.isTray() {
		return core.LayerTray
	}
	return l.month
}

// filter is the rows this layer holds. Zero `All` is the working set: live rows only.
func (l layer) filter(everything bool) store.Filter {
	if l.isTray() {
		return store.Filter{Layer: core.LayerTray, All: everything}
	}
	return store.Filter{Layer: core.LayerGarage, Month: l.month, All: everything}
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
	// No tray, and the months this sweep is about: the one being closed, this one, and
	// somewhere later. `--month` replaces the closing tab, so a month further back —
	// or further forward — is reachable without quitting.
	//
	// `next` is only ever the forward slot, so it is dropped when the named month is
	// already forward of this one: sweeping November in September needs September and
	// November, and October is a month you did not come here to think about.
	closingMonth := store.PrevMonth(this)
	if closing != "" {
		closingMonth = closing
	}
	months := []string{closingMonth, this}
	if closingMonth <= this {
		months = append(months, store.NextMonth(this))
	}

	// Chronological, so the tabs read as a timeline whichever month was named. someday
	// is not a date and sits at the end.
	sort.Strings(months)
	var out []layer
	seen := map[string]bool{}
	for _, m := range append(months, store.Someday) {
		if seen[m] {
			continue
		}
		seen[m] = true
		out = append(out, layer{title: monthTitle(m), month: m})
	}
	return out
}

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

func liveIn(s *store.Store, month string) int {
	rows, err := s.Tasks(store.Filter{Layer: core.LayerGarage, Month: month})
	if err != nil {
		return 0
	}
	return len(rows)
}

// destinations are where `>` can send the selection: every other layer, plus next
// month, which is what carrying forward means.
func (m Model) destinations() []layer {
	// Every tab on screen comes first, in tab order. Anything you can see is somewhere
	// you can send a line — during a sweep the closing month is a tab and was not a
	// destination, so a line could be carried out of it and never back.
	var all []layer
	all = append(all, m.layers...)

	// The tray is always reachable, tab or not: the sweep has no tray tab, and triage
	// is exactly when you decide something is for now rather than for later.
	all = append(all, layer{title: "tray"})

	// Beyond that the two screens want opposite things. The sweep is a closed world —
	// the months it opened are the months it is about, and a fifth one in the picker is
	// a month you did not come here to think about. The daily screen has only two tabs,
	// so `>` is the whole of how someday and next month are reached at all (10).
	if !m.sweep {
		this := store.ThisMonth()
		all = append(all,
			layer{title: monthTitle(this), month: this},
			layer{title: monthTitle(store.NextMonth(this)), month: store.NextMonth(this)},
			layer{title: store.Someday, month: store.Someday},
		)
		if last := store.PrevMonth(this); liveIn(m.s, last) > 0 {
			all = append(all, layer{title: monthTitle(last), month: last})
		}
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

// move is take, hand back and carry forward at once (7): the row changes layer or
// month and nothing is copied. Arriving on the tray remembers the month it left, so
// handing back needs no destination; a row already where it is going is left alone.
func (m *Model) move(picked []core.Task, to layer) string {
	moved := 0
	err := m.s.Update(func(tx *store.Store) error {
		for _, t := range picked {
			if to.isTray() && t.Layer == core.LayerTray {
				continue
			}
			if !to.isTray() && t.Layer == core.LayerGarage && t.Month == to.month {
				continue
			}
			core.Move(&t, layerOf(to), to.month)
			if to.isTray() {
				t.Wait = "" // you took it; its day is now
			}
			if err := tx.Put(&t); err != nil {
				return err
			}
			moved++
		}
		return nil
	})
	if err != nil {
		return err.Error()
	}
	return plural(moved, "→ "+to.title)
}

func layerOf(l layer) string {
	if l.isTray() {
		return core.LayerTray
	}
	return core.LayerGarage
}
