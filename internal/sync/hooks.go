package sync

import (
	"fmt"
	"strings"
	"time"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
	"github.com/cheese-cracker/tray/internal/wire"
)

// Nothing in tray runs on its own (T4). It acts on two events: something wrote to the
// store, or you asked it to sync — by hand, or by opening an interface that asked to
// run then. Hooks answer events; this table is the whole of what happens, in order.
type Event string

const (
	Write  Event = "write"  // after any change to the store — the CLI's, the interface's, a plugin's
	Manual Event = "manual" // `tray sync`, `S` in the interface
	Launch Event = "launch" // an interface opening; only hooks and plugins that asked to run then
)

// A Hook is one thing that runs on an event. The built-in ones are rows in Hooks and
// run in that order; an installed plugin is a hook on the sync events too — its `sync`
// executable, run by Plans after these, because its plan is reviewed before it lands
// and these are not. Adding a built-in is one row: a name, the events it answers, and
// what it does.
type Hook struct {
	Name string
	On   []Event
	Run  func(s *store.Store, today time.Time) (Report, error)
}

// A Report is what one hook did: a phrase for the status line, numbers for `--json`.
type Report struct {
	Hook   string
	Line   string
	Counts map[string]int
}

// Hooks, in the order they run. The mirror is last on every event so the files always
// show the store as the other hooks left it.
var Hooks = []Hook{
	{Name: "recur", On: []Event{Manual, Launch}, Run: recur},
	{Name: "lift", On: []Event{Manual, Launch}, Run: lift},
	{Name: "garage.md", On: []Event{Manual, Launch}, Run: absorb},
	{Name: "mirror", On: []Event{Write, Manual, Launch}, Run: mirror},
}

// A Summary is every report of one event.
type Summary []Report

// String is the status line: each hook's phrase, quiet ones left out.
func (s Summary) String() string {
	var parts []string
	for _, r := range s {
		if r.Line != "" {
			parts = append(parts, r.Line)
		}
	}
	return strings.Join(parts, " · ")
}

// Fire runs every built-in hook that answers the event, in order, and stops at the
// first that fails. Plugins are not here: see Plans.
func Fire(s *store.Store, ev Event, today time.Time) (Summary, error) {
	var sum Summary
	for _, h := range Hooks {
		if !answers(h, ev) {
			continue
		}
		r, err := h.Run(s, today)
		r.Hook = h.Name
		sum = append(sum, r)
		if err != nil {
			return sum, fmt.Errorf("%s: %w", h.Name, err)
		}
	}
	return sum, nil
}

func answers(h Hook, ev Event) bool {
	for _, on := range h.On {
		if on == ev {
			return true
		}
	}
	return false
}

// recur gives every template its next child (T14). Inside data, so no review.
func recur(s *store.Store, today time.Time) (Report, error) {
	n := 0
	err := s.Update(func(tx *store.Store) error {
		all, err := tx.Tasks(store.Filter{All: true})
		if err != nil {
			return err
		}
		var templates, children []core.Task
		for _, t := range all {
			switch {
			case t.Recur != "":
				templates = append(templates, t)
			case strings.HasPrefix(t.Source, "recur:"):
				children = append(children, t)
			}
		}
		for _, c := range core.Materialize(templates, children, today) {
			if err := tx.Put(&c); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return Report{Line: fmt.Sprintf("materialized %d", n), Counts: map[string]int{"materialized": n}}, err
}

// lift moves a garage row whose day has come onto the tray with what it carries (T13).
func lift(s *store.Store, today time.Time) (Report, error) {
	n := 0
	err := s.Update(func(tx *store.Store) error {
		rows, err := tx.Tasks(store.Filter{Layer: core.LayerGarage})
		if err != nil {
			return err
		}
		for _, t := range rows {
			if t.Wait == "" || t.Waiting(today) {
				continue
			}
			core.Move(&t, core.LayerTray, "")
			t.Wait = "" // its day came; handed back later, it waits for nothing
			if err := tx.Put(&t); err != nil {
				return err
			}
			n++
		}
		return nil
	})
	return Report{Line: fmt.Sprintf("lifted %d", n), Counts: map[string]int{"lifted": n}}, err
}

// absorb reads garage.md back: new lines and renames, nothing else. Your own file, so
// it lands directly — the review is for what comes from outside (T5).
func absorb(s *store.Store, today time.Time) (Report, error) {
	got, err := wire.Absorb(s, store.Home(), today)
	line := fmt.Sprintf("garage.md +%d ~%d", got.Added, got.Renamed)
	if got.Unknown > 0 {
		line += fmt.Sprintf(" ?%d", got.Unknown)
	}
	return Report{Line: line, Counts: map[string]int{"added": got.Added, "renamed": got.Renamed, "unknown": got.Unknown}}, err
}

// mirror rewrites tray.md and garage.md. Quiet unless it fails, or unless garage.md
// holds edits it has not read — then it says so instead of writing over them.
func mirror(s *store.Store, today time.Time) (Report, error) {
	skipped, err := wire.Mirror(s, store.Home(), today)
	if skipped {
		return Report{Line: "garage.md has edits — sync brings them in"}, err
	}
	return Report{}, err
}
