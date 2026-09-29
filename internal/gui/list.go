package gui

import (
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
	"github.com/cheese-cracker/tray/internal/style"
)

// taskList is one layer's rows. It owns the keyboard while you browse: the letters
// are shortcuts for the controls on the rows, and Tab is the layer switch rather than
// focus traversal, which is why it accepts the key itself.
type taskList struct {
	widget.List
	u     *ui
	layer string
	all   []core.Task // the layer, live
	rows  []core.Task // what the filter left
	cur   int
	focus bool
	// items is the row widget last shown for an index, for tests that read a row.
	// ponytail: List recycles item widgets on scroll, so this is only true while every
	// row fits on screen; a proper lookup if lists ever grow past a screen.
	items map[widget.ListItemID]*row
}

func newTaskList(u *ui, layer string) *taskList {
	l := &taskList{u: u, layer: layer, items: map[widget.ListItemID]*row{}}
	l.Length = func() int { return len(l.rows) }
	l.CreateItem = func() fyne.CanvasObject { return newRow(l) }
	l.UpdateItem = func(id widget.ListItemID, o fyne.CanvasObject) {
		r := o.(*row)
		r.set(id, l.rows[id])
		l.items[id] = r
	}
	l.OnSelected = func(id widget.ListItemID) {
		l.cur = id
		l.u.details.show(l.cursorTask())
	}
	l.HideSeparators = true
	l.ExtendBaseWidget(l)
	return l
}

func (l *taskList) load(rows []core.Task) { l.all = rows }

// apply keeps the cursor on the task it was on, if the filter still shows it.
func (l *taskList) apply(filter string) {
	was, had := l.cursorTask()
	l.rows = l.rows[:0:0]
	for _, t := range l.all {
		if filter == "" || fuzzy(filter, t.Text+" "+strings.Join(t.Tags, " ")) {
			l.rows = append(l.rows, t)
		}
	}
	l.cur = 0
	for i, t := range l.rows {
		if had && t.ID == was.ID {
			l.cur = i
		}
	}
	l.Refresh()
	if len(l.rows) > 0 {
		l.Select(l.cur)
	} else {
		l.UnselectAll()
	}
}

func (l *taskList) cursorTask() (core.Task, bool) {
	if l.cur < 0 || l.cur >= len(l.rows) {
		return core.Task{}, false
	}
	return l.rows[l.cur], true
}

func (l *taskList) pick(i int) {
	if i < 0 || i >= len(l.rows) {
		return
	}
	l.cur = i
	l.Select(i)
	l.u.details.show(l.cursorTask())
	l.Refresh()
}

func (l *taskList) move(by int) {
	next := l.cur + by
	if next < 0 || next >= len(l.rows) {
		return
	}
	l.pick(next)
}

func (l *taskList) AcceptsTab() bool { return true }

func (l *taskList) FocusGained() {
	l.focus = true
	l.List.FocusGained()
	l.Refresh()
}

func (l *taskList) FocusLost() {
	l.focus = false
	l.List.FocusLost()
	l.Refresh()
}

func (l *taskList) TypedKey(ev *fyne.KeyEvent) {
	switch ev.Name {
	case fyne.KeyDown:
		l.move(1)
	case fyne.KeyUp:
		l.move(-1)
	case fyne.KeyTab:
		l.u.switchLayer()
	case fyne.KeyReturn, fyne.KeyEnter:
		l.u.do("l", l)
	case fyne.KeyEscape:
		// A filter is the thing most recently put in your way, so it goes first.
		if l.u.filter != "" {
			l.u.clearFilter()
			return
		}
		l.u.quit()
	}
}

func (l *taskList) TypedRune(r rune) {
	switch r {
	case 'j':
		l.move(1)
	case 'k':
		l.move(-1)
	default:
		l.u.do(string(r), l)
	}
}

// fuzzy is the needle's runes in order somewhere in the hay, case-insensitively —
// over words and tags only, never the note.
func fuzzy(needle, hay string) bool {
	n := []rune(strings.ToLower(needle))
	i := 0
	for _, r := range strings.ToLower(hay) {
		if i < len(n) && r == n[i] {
			i++
		}
	}
	return i == len(n)
}

// row draws one task. Its actions appear when the pointer is on it or the cursor is,
// so the list reads clean and every verb is still one click away.
type row struct {
	widget.BaseWidget
	l   *taskList
	idx int
	t   core.Task

	hovered bool
	mark    *widget.Check
	pri     *canvas.Text
	text    *canvas.Text
	tags    *canvas.Text
	when    *canvas.Text
	note    *canvas.Text
	verbs   map[string]*widget.Button
	actions *fyne.Container
	box     *fyne.Container
}

var (
	garageVerbs = []string{"t", "#", "l"}
	trayVerbs   = []string{"x", "d", ">", "l"}
	verbLabels  = map[string]string{"t": "take", "#": "tag", "x": "done", "d": "hand back", ">": "move", "l": "open"}
)

func newRow(l *taskList) *row {
	r := &row{l: l, verbs: map[string]*widget.Button{}}
	r.mark = widget.NewCheck("", func(on bool) { r.l.u.setMark(r.t.ID, on) })
	r.pri = grey("")
	r.pri.TextStyle.Bold = true
	r.text = plain("")
	r.tags = grey("")
	r.when = grey("")
	r.note = grey("≡")
	r.actions = container.NewHBox()
	// Creation order is reading order: the layer's own verbs first, open last on both.
	for _, key := range []string{"x", "d", ">", "t", "#", "l"} {
		key := key
		b := widget.NewButton(verbLabels[key], func() {
			r.l.pick(r.idx)
			r.l.u.do(key, r.l)
		})
		b.Importance = widget.LowImportance
		r.verbs[key] = b
		r.actions.Add(b)
	}
	r.box = container.NewHBox(r.mark, r.pri, r.text, r.tags, r.when, r.note, layout.NewSpacer(), r.actions)
	r.ExtendBaseWidget(r)
	return r
}

func (r *row) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(r.box) }

func (r *row) set(idx int, t core.Task) {
	r.idx, r.t = idx, t
	r.Refresh()
}

func (r *row) Refresh() {
	tray := r.l.layer == core.LayerTray
	isDark := dark()
	r.mark.Checked = r.l.u.marks[r.t.ID]
	r.mark.Refresh()

	// An unset priority reads as medium but was never chosen, so it prints a dot (78c).
	r.pri.Text = "·"
	if r.t.Priority != "" {
		r.pri.Text = r.t.Priority
	}
	r.pri.Color = style.RGBA(style.Priority(r.t.Priority), isDark)
	setShown(r.pri, tray)

	r.text.Text = r.t.Text
	r.text.Refresh()

	var tags []string
	for _, g := range r.t.Tags {
		tags = append(tags, "#"+g) // the screen draws #, the wire keeps + (101)
	}
	r.tags.Text = strings.Join(tags, " ")
	setShown(r.tags, len(tags) > 0)

	// A date is the one thing about a row that is about today: on you now, or later.
	r.when.Text = ""
	r.when.Color = style.RGBA(style.Later, isDark)
	if tray && r.t.Due != "" {
		r.when.Text = core.Day(r.t.Due)
		if d, ok := core.Date(r.t.Due); ok && !d.After(store.Today()) {
			r.when.Color = style.RGBA(style.Now, isDark)
		}
	}
	if !tray && r.t.Wait != "" {
		r.when.Text = "waits " + core.Day(r.t.Wait)
	}
	setShown(r.when, r.when.Text != "")
	setShown(r.note, r.t.Note != "")

	verbs := garageVerbs
	if tray {
		verbs = trayVerbs
	}
	for key, b := range r.verbs {
		setShown(b, contains(verbs, key))
	}
	setShown(r.actions, r.hovered || (r.l.focus && r.l.cur == r.idx))
	r.BaseWidget.Refresh()
}

func (r *row) MouseIn(*desktop.MouseEvent)    { r.hovered = true; r.Refresh() }
func (r *row) MouseMoved(*desktop.MouseEvent) {}
func (r *row) MouseOut()                      { r.hovered = false; r.Refresh() }

// Tapped puts the cursor here and shows the row in the pane; the list keeps the keys.
func (r *row) Tapped(*fyne.PointEvent) {
	r.l.pick(r.idx)
	r.l.u.focusList()
}

func setShown(o fyne.CanvasObject, on bool) {
	if on {
		o.Show()
	} else {
		o.Hide()
	}
	o.Refresh()
}

func contains(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}
