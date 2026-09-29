package gui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/cheese-cracker/tray/internal/core"
)

// escEntry is an Entry that knows how to be left. Fyne's does nothing on Escape, and
// a field you cannot back out of is a trap. onBlur says the focus went elsewhere, for a
// field that should get out of the way when it does.
type escEntry struct {
	widget.Entry
	onEscape func()
	onBlur   func()
}

func newEscEntry(onEscape func()) *escEntry {
	e := &escEntry{onEscape: onEscape}
	e.ExtendBaseWidget(e)
	return e
}

func (e *escEntry) FocusLost() {
	e.Entry.FocusLost()
	if e.onBlur != nil {
		e.onBlur()
	}
}

func (e *escEntry) TypedKey(k *fyne.KeyEvent) {
	if k.Name == fyne.KeyEscape {
		if e.onEscape != nil {
			e.onEscape()
		}
		return
	}
	e.Entry.TypedKey(k)
}

// dueEntry spends ←/→ on the value: a day back, a day on (100a). The caret has
// nothing to do in a date, so the keys are free for the thing you came to change.
type dueEntry struct {
	escEntry
	day *widget.Label // the same date as a person reads it
}

func newDueEntry(onEscape func()) *dueEntry {
	e := &dueEntry{day: widget.NewLabel("")}
	e.onEscape = onEscape
	e.OnChanged = func(s string) { e.day.SetText(core.Day(s)) }
	e.ExtendBaseWidget(e)
	return e
}

func (e *dueEntry) TypedKey(k *fyne.KeyEvent) {
	if d, ok := core.Date(e.Text); ok {
		switch k.Name {
		case fyne.KeyLeft:
			e.SetText(d.AddDate(0, 0, -1).Format(core.DateLayout))
			return
		case fyne.KeyRight:
			e.SetText(d.AddDate(0, 0, 1).Format(core.DateLayout))
			return
		}
	}
	e.escEntry.TypedKey(k)
}

func (e *dueEntry) withDay() fyne.CanvasObject {
	return container.NewBorder(nil, nil, nil, e.day, e)
}

// noteEntry saves on Enter and breaks a line on ctrl+j (104f): most notes are a
// sentence, so the common case is one keystroke and the rare one is two.
type noteEntry struct {
	escEntry
	onSave func()
}

func newNoteEntry(onEscape, onSave func()) *noteEntry {
	e := &noteEntry{onSave: onSave}
	e.onEscape = onEscape
	e.MultiLine = true
	e.Wrapping = fyne.TextWrapWord
	e.SetMinRowsVisible(3)
	e.ExtendBaseWidget(e)
	return e
}

func (e *noteEntry) TypedKey(k *fyne.KeyEvent) {
	if k.Name == fyne.KeyReturn || k.Name == fyne.KeyEnter {
		e.onSave()
		return
	}
	e.escEntry.TypedKey(k)
}

func (e *noteEntry) TypedShortcut(s fyne.Shortcut) {
	if cs, ok := s.(*desktop.CustomShortcut); ok && cs.KeyName == fyne.KeyJ && cs.Modifier == fyne.KeyModifierControl {
		e.Entry.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
		return
	}
	e.Entry.TypedShortcut(s)
}

// form is the three questions (33): the tray's whole shape, asked once, every field
// prefilled (25). Take, rewrite and add are the same form with a different landing.
type form struct {
	u     *ui
	tasks []core.Task // none: add a tray task
	take  bool

	title *escEntry
	pri   *widget.RadioGroup
	due   *dueEntry
	tags  *escEntry
	box   *widget.Form
}

func (u *ui) openForm(tasks []core.Task, take bool) {
	f := &form{u: u, tasks: tasks, take: take}
	batch := len(tasks) > 1
	f.title = newEscEntry(u.hide)
	f.pri = widget.NewRadioGroup([]string{"H", "M", "L"}, nil)
	f.pri.Horizontal = true
	f.due = newDueEntry(u.hide)
	f.tags = newEscEntry(u.hide)
	f.tags.SetPlaceHolder(u.tagHint())
	for _, e := range []*escEntry{f.title, &f.due.escEntry, f.tags} {
		e.OnSubmitted = func(string) { f.save() }
	}

	switch {
	case len(tasks) == 1:
		t := tasks[0]
		f.title.SetText(t.Text)
		f.due.SetText(t.Due)
		f.tags.SetText(strings.Join(t.Tags, " "))
		f.pri.Required = true
		f.pri.SetSelected(priorityOrM(t.Priority))
	case batch:
		// Only what you touch changes (25): nothing is preselected, and an empty field
		// leaves that rung alone on every task.
	default:
		f.pri.Required = true
		f.pri.SetSelected("M")
	}

	f.box = widget.NewForm()
	if !batch {
		f.box.Append("words", f.title)
	}
	f.box.Append("priority", f.pri)
	f.box.Append("due", f.due.withDay())
	f.box.Append("tags", f.tags)
	f.box.OnSubmit = f.save
	f.box.OnCancel = u.hide
	f.box.SubmitText = "save"
	if take {
		f.box.SubmitText = "take"
	}

	focus := fyne.Focusable(f.title)
	if batch {
		focus = f.due
	}
	u.form = f
	u.show(f.box, focus)
}

// priorityOrM is what the radio shows: unset reads as medium (32), and taking a row
// writes the priority it showed you.
func priorityOrM(p string) string {
	if p == "" {
		return "M"
	}
	return p
}

func (f *form) save() {
	words := flatten(f.title.Text)
	batch := len(f.tasks) > 1
	var out []core.Task
	if len(f.tasks) == 0 {
		if words == "" {
			return
		}
		t := core.New(words, nil)
		t.Layer = core.LayerTray
		out = append(out, t)
	}
	for _, t := range f.tasks {
		if f.take {
			core.Move(&t, core.LayerTray, "")
			t.Wait = "" // you took it; its day is now
		}
		if !batch && words != "" {
			t.Text = words
		}
		out = append(out, t)
	}
	for i := range out {
		if p := f.pri.Selected; p != "" {
			out[i].Priority = p
		}
		if due := strings.TrimSpace(f.due.Text); due != "" || !batch {
			out[i].Due = due
		}
		if tags := strings.Fields(f.tags.Text); len(tags) > 0 || !batch {
			out[i].Tags = tags
		}
	}
	f.u.hide()
	f.u.save(out)
	if f.take || len(f.tasks) == 0 {
		f.u.tabs.SelectIndex(1) // the row went to the tray; so do you (T1)
	}
}

// flatten keeps a task on one line: a pasted newline collapses rather than being
// trusted (100d).
func flatten(s string) string { return strings.Join(strings.Fields(s), " ") }

// openTags is the tag field alone (99b): a keystroke, not the whole form.
func (u *ui) openTags(tasks []core.Task) {
	e := newEscEntry(u.hide)
	e.SetPlaceHolder(u.tagHint())
	if len(tasks) == 1 {
		e.SetText(strings.Join(tasks[0].Tags, " "))
	}
	e.OnSubmitted = func(s string) {
		tags := strings.Fields(s)
		for i := range tasks {
			tasks[i].Tags = tags
		}
		u.hide()
		u.save(tasks)
	}
	f := widget.NewForm(widget.NewFormItem("tags", e))
	f.OnSubmit = func() { e.OnSubmitted(e.Text) }
	f.OnCancel = u.hide
	u.show(f, e)
}

// openNote is one note per task, replaced whole (104).
func (u *ui) openNote(tasks []core.Task) {
	var e *noteEntry
	e = newNoteEntry(u.hide, func() {
		for i := range tasks {
			tasks[i].Note = strings.TrimSpace(e.Text)
		}
		u.hide()
		u.save(tasks)
	})
	if len(tasks) == 1 {
		e.SetText(tasks[0].Note)
	}
	f := widget.NewForm(widget.NewFormItem("note", e))
	f.OnSubmit = e.onSave
	f.OnCancel = u.hide
	u.show(f, e)
}

// openMove offers the months on screen and nothing further (73d, 73e): every tab of
// the sweep, or the daily screen's this month, next month and someday.
func (u *ui) openMove(tasks []core.Task) {
	dests := u.destinations()
	move := func(month string) {
		for i := range tasks {
			core.Move(&tasks[i], core.LayerGarage, month)
		}
		u.hide()
		u.save(tasks)
	}
	box := container.NewVBox(widget.NewLabel("move to"))
	for i, month := range dests {
		month := month
		box.Add(widget.NewButton(string(rune('1'+i))+"  garage · "+month, func() { move(month) }))
	}
	p := newPage(box)
	p.onKey = func(k *fyne.KeyEvent) {
		if k.Name == fyne.KeyEscape {
			u.hide()
		}
	}
	p.onRune = func(r rune) {
		if i := int(r - '1'); i >= 0 && i < len(dests) {
			move(dests[i])
		}
	}
	u.show(p, p)
}
