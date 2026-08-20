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
	t.Dropped = as == "dropped"
}

func setDefault(attrs map[string]string, key, value string) {
	if attrs[key] == "" {
		attrs[key] = value
	}
}
