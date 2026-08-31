package core

import "time"

// Take, hand back and carry forward are one operation: the source line stays with
// an arrow to where its live copy went, and a copy travels. Nothing moves twice.

const DestTray = "tray"

// Depart annotates the source. The line stays exactly where it is.
func Depart(src *Task, dest string) {
	src.Moved = dest
}

// Arrive is the copy that travels, stamped with where it came from and when.
func Arrive(src Task, from string, today time.Time) Task {
	fresh := src.Copy()
	if from != "" {
		setDefault(fresh.Attrs, "from", from)
	}
	setDefault(fresh.Attrs, "entry", today.Format(DateLayout))
	return fresh
}

// Finish marks a task terminal in place — done, or abandoned. Nothing is removed.
func Finish(t *Task, as string, today time.Time) {
	if t.Attrs == nil {
		t.Attrs = map[string]string{}
	}
	t.Attrs[as] = today.Format(DateLayout)
	t.Done = as == "done"
}

// Restore is the inverse of Finish: the line stops being struck through and reads as
// open again. Decision 5 is about never deleting a *line* — this deletes nothing, it
// just says the thing was not finished after all.
//
// No trace is kept. The overwhelming reason to reach for this is a mis-key, and a
// line stamped with every time you fumbled is worse than one that is simply correct.
func Restore(t *Task) {
	if t.Attrs != nil {
		delete(t.Attrs, "done")
	}
	t.Done = false
}

func setDefault(attrs map[string]string, key, value string) {
	if attrs[key] == "" {
		attrs[key] = value
	}
}
