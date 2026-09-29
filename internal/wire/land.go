package wire

import (
	"time"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
)

// A lander knows what is already in the store, so an import can run twice: a row a
// source already names is updated in place, and a row with the same layer, month and
// words is skipped.
type lander struct {
	seen     map[string]bool
	bySource map[string]core.Task
}

func newLander(s *store.Store) (*lander, error) {
	present, err := s.Tasks(store.Filter{All: true})
	if err != nil {
		return nil, err
	}
	l := &lander{seen: map[string]bool{}, bySource: map[string]core.Task{}}
	for _, t := range present {
		l.seen[key(t)] = true
		if t.Source != "" {
			l.bySource[t.Source] = t
		}
	}
	return l, nil
}

// land puts one imported task where the ladder says, unless the importer already
// placed it: a priority, a due date or a period is committed work and goes on the tray;
// a waiting one lies in the garage of its day; the rest are jottings for this month.
func (l *lander) land(tx *store.Store, t core.Task, today time.Time) (string, error) {
	if have, ok := l.bySource[t.Source]; ok && t.Source != "" {
		have.Text, have.Priority, have.Due, have.Wait, have.Recur = t.Text, t.Priority, t.Due, t.Wait, t.Recur
		have.Until, have.Done, have.Tags, have.Note = t.Until, t.Done, t.Tags, t.Note
		if t.Entry != "" {
			have.Entry = t.Entry
		}
		if err := tx.Put(&have); err != nil {
			return "", err
		}
		l.bySource[t.Source] = have
		return "updated", nil
	}
	if t.Layer == "" {
		switch {
		case t.Waiting(today):
			d, _ := core.Date(t.Wait)
			t.Layer, t.Month = core.LayerGarage, d.Format("2006-01")
		case t.Priority != "" || t.Due != "" || t.Recur != "":
			t.Layer = core.LayerTray
		default:
			t.Layer, t.Month = core.LayerGarage, today.Format("2006-01")
		}
	}
	if t.Text == "" || l.seen[key(t)] {
		return "skipped", nil
	}
	if err := tx.Put(&t); err != nil {
		return "", err
	}
	l.seen[key(t)] = true
	if t.Source != "" {
		l.bySource[t.Source] = t
	}
	return "added", nil
}

// Land brings imported tasks into the store in one transaction.
func Land(s *store.Store, tasks []core.Task, today time.Time) (added, updated, skipped int, err error) {
	l, err := newLander(s)
	if err != nil {
		return 0, 0, 0, err
	}
	err = s.Update(func(tx *store.Store) error {
		for _, t := range tasks {
			outcome, err := l.land(tx, t, today)
			if err != nil {
				return err
			}
			switch outcome {
			case "added":
				added++
			case "updated":
				updated++
			default:
				skipped++
			}
		}
		return nil
	})
	return added, updated, skipped, err
}

func key(t core.Task) string { return t.Layer + "\x00" + t.Month + "\x00" + t.Text }
