package core

import "time"

// Move is take, hand back and carry forward in one (7): a row changes layer or month
// and nothing is copied. Leaving the garage remembers the month, so an unload with no
// destination can bring the task home; arriving back forgets it, since it lives there
// again.
func Move(t *Task, layer, month string) {
	if layer == LayerTray {
		if t.Layer == LayerGarage && t.FromMonth == "" {
			t.FromMonth = t.Month
		}
		t.Layer, t.Month = LayerTray, ""
		return
	}
	if month == "" {
		month = t.FromMonth
	}
	t.Layer, t.Month, t.FromMonth = LayerGarage, month, ""
}

// Finish marks a task done in place, dated. A template cannot be finished, only
// stopped: done on one ends the recurrence today and leaves its children alone.
func Finish(t *Task, today time.Time) {
	when := today.Format(DateLayout)
	if t.Recur != "" {
		t.Until = when
		return
	}
	t.Done = when
}

// Restore is the inverse of Finish. No trace is kept: the overwhelming reason to reach
// for this is a mis-key, and a row stamped with every fumble is worse than one that is
// simply correct.
func Restore(t *Task) { t.Done = "" }
