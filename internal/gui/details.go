package gui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/cheese-cracker/tray/internal/core"
)

// details is the ladder drawn for one task, top rung down: words, tags, then what the
// tray asks for, the note, the schedule behind a fold, where it came from, and the id
// in grey at the bottom. An empty rung is a hint, never a prompt.
type details struct {
	u       *ui
	t       core.Task
	has     bool
	filling bool

	empty  *canvas.Text
	words  *escEntry
	tags   *escEntry
	pri    *widget.RadioGroup
	due    *dueEntry
	hint   *canvas.Text
	tray   *fyne.Container
	note   *noteEntry
	recur  *escEntry
	wait   *escEntry
	until  *escEntry
	more   *widget.Accordion
	source *canvas.Text
	id     *canvas.Text
	fields *fyne.Container
	box    *fyne.Container
}

func newDetails(u *ui) *details {
	d := &details{u: u}
	d.empty = grey("nothing here yet")
	d.words = d.entry(func(t *core.Task, s string) {
		if s = flatten(s); s != "" {
			t.Text = s
		}
	})
	d.tags = d.entry(func(t *core.Task, s string) { t.Tags = strings.Fields(s) })
	d.pri = widget.NewRadioGroup([]string{"H", "M", "L"}, func(p string) {
		if !d.filling && p != "" {
			d.commit(func(t *core.Task) { t.Priority = p })
		}
	})
	d.pri.Horizontal = true
	d.due = newDueEntry(u.focusList)
	d.due.OnSubmitted = func(s string) { d.commit(func(t *core.Task) { t.Due = strings.TrimSpace(s) }) }
	d.hint = grey("")
	d.tray = container.NewVBox(labelled("priority", d.pri), labelled("due", d.due.withDay()), d.hint)

	d.note = newNoteEntry(u.focusList, func() {
		d.commit(func(t *core.Task) { t.Note = strings.TrimSpace(d.note.Text) })
	})
	d.recur = d.entry(func(t *core.Task, s string) { t.Recur = strings.TrimSpace(s) })
	d.wait = d.entry(func(t *core.Task, s string) { t.Wait = strings.TrimSpace(s) })
	d.until = d.entry(func(t *core.Task, s string) { t.Until = strings.TrimSpace(s) })
	d.more = widget.NewAccordion(widget.NewAccordionItem("more", container.NewVBox(
		labelled("recur", d.recur), labelled("wait", d.wait), labelled("until", d.until))))
	d.source = grey("")
	d.id = grey("")

	copyBtn := widget.NewButton("copy context", d.copyContext)
	copyBtn.Importance = widget.LowImportance

	d.fields = container.NewVBox(
		labelled("words", d.words), labelled("tags", d.tags), d.tray,
		labelled("note", d.note), d.more, d.source, container.NewHBox(d.id, copyBtn))
	d.box = container.NewVBox(d.empty, d.fields)
	d.show(core.Task{}, false)
	return d
}

func (d *details) entry(set func(*core.Task, string)) *escEntry {
	e := newEscEntry(d.u.focusList)
	e.OnSubmitted = func(s string) { d.commit(func(t *core.Task) { set(t, s) }) }
	return e
}

func (d *details) show(t core.Task, ok bool) {
	d.t, d.has = t, ok
	d.filling = true
	defer func() { d.filling = false }()
	setShown(d.empty, !ok)
	setShown(d.fields, ok)
	if !ok {
		d.box.Refresh()
		return
	}
	d.words.SetText(t.Text)
	d.tags.SetText(strings.Join(t.Tags, " "))
	tray := t.Layer == core.LayerTray
	setShown(d.tray, tray)
	if tray {
		d.pri.SetSelected(t.Priority)
		d.due.SetText(t.Due)
		d.hint.Text = missing(t)
		d.hint.Refresh()
	}
	d.note.SetText(t.Note)
	d.recur.SetText(t.Recur)
	d.wait.SetText(t.Wait)
	d.until.SetText(t.Until)
	d.source.Text = t.Source
	setShown(d.source, t.Source != "")
	d.id.Text = fmt.Sprintf("#%d", t.ID)
	d.id.Refresh()
	// A child that was hidden has no size until its parent lays out again; painting
	// it before that is a rectangle of negative width.
	d.fields.Refresh()
	d.box.Refresh()
}

// missing names the rungs a tray task has not climbed, the way `tray add` does (34).
func missing(t core.Task) string {
	var wants []string
	if t.Priority == "" {
		wants = append(wants, "no priority")
	}
	if t.Due == "" {
		wants = append(wants, "no due")
	}
	return strings.Join(wants, " · ")
}

func (d *details) commit(change func(*core.Task)) {
	if !d.has {
		return
	}
	t := d.t
	change(&t)
	d.u.save([]core.Task{t})
}

func (d *details) focus() {
	if d.has {
		d.u.win.Canvas().Focus(d.words)
	}
}

// copyContext is what an agent would be handed: this row as `tray context` prints it.
func (d *details) copyContext() {
	if d.has {
		d.u.copyContext([]core.Task{d.t})
	}
}

func labelled(name string, o fyne.CanvasObject) fyne.CanvasObject {
	return container.NewBorder(nil, nil, grey(name), nil, o)
}
