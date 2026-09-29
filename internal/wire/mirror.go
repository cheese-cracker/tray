package wire

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
)

// The mirror: two markdown files beside the database, one bullet per open task. tray.md
// is written and never read — the tray is not for adding to on a whim. garage.md is
// read back on sync: a new bullet is a new line, changed words are a rename, a missing
// bullet deletes nothing. Drop the folder into a vault and a phone can dump into it.
const (
	TrayMirror   = "tray.md"
	GarageMirror = "garage.md"
)

// garageKey is where the store remembers the garage.md it last wrote or read, so a
// file that differs from it is one somebody edited — and the mirror will not write over
// an edit it has not read. tray.md has no such key: it is never read, so nothing there
// is ever lost that was meant to be kept.
const garageKey = "mirror.garage.md"

// Mirror rewrites both files from the store. Unchanged content is left alone, so a
// vault watching the folder sees a write only when something moved. garage.md with
// edits tray has not absorbed is skipped, and Mirror says so; the next sync reads it.
func Mirror(s *store.Store, dir string, today time.Time) (skipped bool, err error) {
	tray, err := s.Tasks(store.Filter{Layer: core.LayerTray})
	if err != nil {
		return false, err
	}
	sort.SliceStable(tray, func(i, j int) bool {
		return core.Urgency(tray[i], today) > core.Urgency(tray[j], today)
	})
	var tb strings.Builder
	tb.WriteString("# tray\n\n")
	for _, t := range tray {
		tb.WriteString(core.MirrorLine(t) + "\n")
	}

	garage, err := s.Tasks(store.Filter{Layer: core.LayerGarage})
	if err != nil {
		return false, err
	}
	byMonth := map[string][]core.Task{}
	for _, t := range garage {
		if store.IsMonth(t.Month) || t.Month == store.Someday { // a plugin's garage is the plugin's
			byMonth[t.Month] = append(byMonth[t.Month], t)
		}
	}
	months := make([]string, 0, len(byMonth))
	for m := range byMonth {
		if m != store.Someday {
			months = append(months, m)
		}
	}
	sort.Strings(months)
	if _, ok := byMonth[store.Someday]; ok {
		months = append(months, store.Someday)
	}
	var gb strings.Builder
	gb.WriteString("# garage\n")
	for _, m := range months {
		gb.WriteString("\n" + core.MirrorHeading(m) + "\n")
		for _, t := range byMonth[m] {
			gb.WriteString(core.MirrorLine(t) + "\n")
		}
	}

	if err := writeIfChanged(filepath.Join(dir, TrayMirror), tb.String()); err != nil {
		return false, err
	}
	path := filepath.Join(dir, GarageMirror)
	last, err := s.Meta(garageKey)
	if err != nil {
		return false, err
	}
	if cur, ok := digest(path); ok && cur != last {
		return true, nil // edited since we last touched it: absorb first, never clobber
	}
	if err := writeIfChanged(path, gb.String()); err != nil {
		return false, err
	}
	return false, s.SetMeta(garageKey, sum(gb.String()))
}

func digest(path string) (string, bool) {
	body, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return sum(string(body)), true
}

func sum(body string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(body))) }

// writeIfChanged replaces a file atomically, and not at all when it already says this.
func writeIfChanged(path, body string) error {
	if old, err := os.ReadFile(path); err == nil && string(old) == body {
		return nil
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tray-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.WriteString(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Absorbed is what reading garage.md back did.
type Absorbed struct{ Added, Renamed, Unknown int }

// Absorb reads garage.md: a bullet with no id becomes a new line in the month it sits
// under (this month above any heading); a bullet whose words changed renames its task; a
// bullet with an id tray does not know is left alone and counted. Nothing is deleted
// and nothing is finished from here — the file is a lightweight view, not a second
// interface. One transaction.
func Absorb(s *store.Store, dir string, today time.Time) (Absorbed, error) {
	var got Absorbed
	body, err := os.ReadFile(filepath.Join(dir, GarageMirror))
	if os.IsNotExist(err) {
		return got, nil
	}
	if err != nil {
		return got, err
	}

	garage, err := s.Tasks(store.Filter{Layer: core.LayerGarage})
	if err != nil {
		return got, err
	}
	byID := map[string]core.Task{}
	seen := map[string]bool{} // month + words already live there, so a re-read adds nothing
	for _, t := range garage {
		byID[t.ID] = t
		seen[t.Month+"\n"+t.Text] = true
	}

	month := today.Format("2006-01")
	var renames, adds []core.Task
	for _, line := range strings.Split(string(body), "\n") {
		if h, ok := core.ParseMirrorHeading(line); ok {
			month = today.Format("2006-01")
			if store.IsMonth(h) || h == store.Someday {
				month = h
			}
			continue
		}
		id, text, ok := core.ParseMirror(line)
		if !ok || text == "" {
			continue
		}
		if id != "" {
			t, known := byID[id]
			switch {
			case !known:
				got.Unknown++
			case t.Text != text:
				t.Text = text
				renames = append(renames, t)
			}
			continue
		}
		if seen[month+"\n"+text] {
			continue
		}
		seen[month+"\n"+text] = true
		t := core.New(text, nil)
		t.Month, t.Entry = month, today.Format(core.DateLayout)
		adds = append(adds, t)
	}
	err = s.Update(func(tx *store.Store) error {
		// Read is consumed: whatever the file says now, the mirror may write over it.
		if err := tx.SetMeta(garageKey, sum(string(body))); err != nil {
			return err
		}
		for _, t := range renames {
			if err := tx.Put(&t); err != nil {
				return err
			}
			got.Renamed++
		}
		for _, t := range adds {
			if err := tx.Put(&t); err != nil {
				return err
			}
			got.Added++
		}
		return nil
	})
	return got, err
}
