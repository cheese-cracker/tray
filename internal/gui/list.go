package gui

import (
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/widget"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
	"github.com/cheese-cracker/tray/internal/style"
)

// rowHeight is dense enough to read a list as a list and still a finger's target.
const rowHeight = 34

// taskList is one layer's rows. It owns the keyboard while you browse: the letters
// are shortcuts for the controls on the rows, and Tab is the layer switch rather than
// focus traversal, which is why it accepts the key itself.
type taskList struct {
	widget.List
	u        *ui
	layer    string
	month    string   // the garage month shown, where the layer is the garage
	verbs    []string // the actions a row here offers, in reading order
	review   bool     // everything on the layer: finished and templates draw as such
	all      []core.Task
	rows     []core.Task // what the filter left
	cur      int
	focus    bool
	empty    *canvas.Text // what an empty layer says, centred over the list
	emptyMsg string
	// items is the row widget last shown for an index, for tests that read a row.
	// ponytail: List recycles item widgets on scroll, so this is only true while every
	// row fits on screen; a proper lookup if lists ever grow past a screen.
	items map[widget.ListItemID]*row
}

var (
	garageVerbs = []string{"t", "#", "l"}
	trayVerbs   = []string{"x", "d", ">", "l"}
	reviewVerbs = []string{"R", "E", "l"}
	sweepVerbs  = []string{"t", ">", "l"}
	verbLabels  = map[string]string{
		"t": "take", "#": "tag", "x": "done", "d": "hand back", ">": "move", "l": "open",
		"R": "restore", "E": "erase",
	}
)

func newTaskList(u *ui, layer string, verbs []string) *taskList {
	l := &taskList{u: u, layer: layer, verbs: verbs, items: map[widget.ListItemID]*row{}}
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
	// Empty states say what to do next rather than that there is nothing (T12: the
	// garage's next step is typing, the tray's is taking).
	msg := "Nothing here."
	switch layer {
	case core.LayerGarage:
		msg = "Nothing here yet — type below."
	case core.LayerTray:
		msg = "Nothing on the tray. Take a line from the garage (t)."
	}
	l.emptyMsg = msg
	l.empty = grey(msg)
	l.ExtendBaseWidget(l)
	return l
}

// view is the list with its empty state over it.
func (l *taskList) view() fyne.CanvasObject {
	return container.NewStack(l, container.NewCenter(l.empty))
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
	l.empty.Text = l.emptyMsg
	if filter != "" {
		l.empty.Text = "Nothing matches."
	}
	setShown(l.empty, len(l.rows) == 0)
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

// FocusGained keeps the focus flag here and never tells the List: it would paint its own
// focus ring on an item of its choosing, beside the cursor row this widget draws.
func (l *taskList) FocusGained() {
	l.focus = true
	l.Refresh()
}

func (l *taskList) FocusLost() {
	l.focus = false
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
		l.u.do(":", l) // the palette is the action menu (24)
	case fyne.KeyEscape:
		// A filter is the thing most recently put in your way, so it goes first; then
		// the mode you are in (92f); only home quits.
		switch {
		case l.u.filter != "":
			l.u.clearFilter()
		case l.u.mode != modeHome:
			l.u.leave()
		default:
			l.u.quit()
		}
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

// TypedShortcut is ctrl+shift+p, the palette's other key; the List's own shortcuts are
// none of ours.
func (l *taskList) TypedShortcut(s fyne.Shortcut) {
	if paletteShortcut(s) {
		l.u.do(":", l)
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

// row draws one task: the cursor bar, the mark, a priority dot and letter, the words,
// its tags as chips, the date in Fira Code, the note sign, and — when the pointer or
// the cursor is on it — its actions, so the list reads clean and every verb is still
// one click away.
type row struct {
	widget.BaseWidget
	l   *taskList
	idx int
	t   core.Task

	hovered bool
	acting  bool // the actions are up; appear() runs on the way up only
	bg      *canvas.Rectangle
	bar     *canvas.Rectangle
	mark    *markDot
	priDot  *canvas.Circle
	pri     *canvas.Text
	text    *canvas.Text
	chips   *fyne.Container
	when    *canvas.Text
	note    *glyph
	verbs   map[string]*link
	actions *fyne.Container
	box     fyne.CanvasObject
}

func newRow(l *taskList) *row {
	r := &row{l: l, verbs: map[string]*link{}}
	r.bg = canvas.NewRectangle(color.Transparent)
	r.bg.SetMinSize(fyne.NewSize(0, rowHeight))
	r.bar = canvas.NewRectangle(rgba(style.Accent))
	r.mark = newMarkDot(func() {
		r.l.u.setMark(r.t.ID, !r.l.u.marks[r.t.ID])
		r.Refresh()
	})
	r.priDot = dot(8, rgba(style.Subtle), color.Transparent, 0)
	r.pri = mono("·", style.Subtle)
	r.text = plain("")
	r.chips = container.NewHBox()
	r.when = mono("", style.Later)
	r.when.TextSize = 12
	r.note = newGlyph(glyphNote, rgba(style.Subtle))
	r.actions = container.NewHBox()
	// Creation order is reading order: the layer's own verbs first, open last on both.
	for _, key := range []string{"x", "d", ">", "t", "#", "R", "E", "l"} {
		key := key
		a := newLink(verbLabels[key], func() {
			r.l.pick(r.idx)
			r.l.u.do(key, r.l)
		})
		r.verbs[key] = a
		r.actions.Add(a)
	}
	left := container.NewHBox(fixed(r.bar, 3, rowHeight), fixed(r.mark, 28, rowHeight),
		container.NewCenter(container.NewHBox(fixed(r.priDot, 8, 8), r.pri)))
	right := container.NewHBox(container.NewCenter(r.when), fixed(r.note, 16, rowHeight),
		container.NewCenter(r.actions), fixed(canvas.NewRectangle(color.Transparent), 6, 1))
	middle := container.NewHBox(container.NewCenter(r.text), container.NewCenter(r.chips))
	r.box = container.NewStack(r.bg, container.NewBorder(nil, nil, left, right, middle))
	r.ExtendBaseWidget(r)
	return r
}

func (r *row) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(r.box) }

func (r *row) set(idx int, t core.Task) {
	r.idx, r.t = idx, t
	r.Refresh()
	// A line you just captured lands with a soft accent that fades to the paper, so the
	// eye finds where it went without a word said.
	if r.l.u.flashID != 0 && r.l.u.flashID == t.ID {
		r.l.u.flashID = 0
		// Opaque to opaque, then clear: a fade to alpha zero would pass through colours
		// RGBA cannot hold, and the list's own highlight has to show through at the end.
		from, to := rgba(style.AccentSoft), rgba(style.Paper)
		canvas.NewColorRGBAAnimation(from, to, linger, func(c color.Color) {
			r.bg.FillColor = c
			if c == color.Color(to) {
				r.bg.FillColor = color.Transparent
			}
			r.bg.Refresh()
		}).Start()
	}
}

func (r *row) Refresh() {
	tray := r.l.layer == core.LayerTray
	finished := r.t.Done != ""
	cursor := r.l.cur == r.idx && len(r.l.rows) > 0
	setShown(r.bar, cursor)
	r.mark.set(r.l.u.marks[r.t.ID], finished)

	// An unset priority reads as medium but was never chosen, so it prints a dot (78c).
	r.pri.Text = "·"
	tint := style.Priority(r.t.Priority)
	if r.t.Priority != "" {
		r.pri.Text = r.t.Priority
	}
	if finished {
		tint = style.Subtle
	}
	r.pri.Color = rgba(tint)
	r.priDot.FillColor = rgba(tint)
	r.priDot.Refresh()
	setShown(r.priDot, tray && r.t.Priority != "" && !finished)
	setShown(r.pri, tray)

	// A finished row keeps none of its colour (103b): the mark is what says done.
	r.text.Text = r.t.Text
	r.text.Color = rgba(style.Ink)
	if finished {
		r.text.Color = rgba(style.Subtle)
	}
	r.text.Refresh()

	r.chips.Objects = nil
	for _, g := range r.t.Tags {
		r.chips.Add(chip("#" + g)) // the screen draws #, the wire keeps + (101)
	}
	r.chips.Refresh()

	// A date is the one thing about a row that is about today: on you now, or later.
	// A template says how often it makes more of itself; a waiting line says when.
	r.when.Text = ""
	r.when.Color = rgba(style.Later)
	switch {
	case r.t.Recur != "":
		r.when.Text = "every " + strings.TrimSuffix(r.t.Recur, "ly")
		if r.t.Recur == "daily" {
			r.when.Text = "every day"
		}
	case tray && r.t.Due != "":
		r.when.Text = core.Day(r.t.Due)
		if d, ok := core.Date(r.t.Due); ok && !d.After(store.Today()) && !finished {
			r.when.Color = rgba(style.Now)
		}
	case !tray && r.t.Wait != "":
		r.when.Text = "waits " + core.Day(r.t.Wait)
	}
	setShown(r.when, r.when.Text != "")
	setShown(r.note, r.t.Note != "")

	// Restore is for a finished row and nothing else (80): the only sane thing to say
	// about a record is that it isn't one.
	for key, a := range r.verbs {
		setShown(a, contains(r.l.verbs, key) && (key != "R" || finished))
	}
	up := r.hovered || (r.l.focus && cursor)
	if up && !r.acting {
		for _, a := range r.verbs {
			if a.Visible() {
				a.appear()
			}
		}
	}
	r.acting = up
	setShown(r.actions, up)
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

// markDot is the selection mark: a hollow circle that fills with the accent when the row
// is marked, and carries a check when the row is finished.
type markDot struct {
	widget.BaseWidget
	circle *canvas.Circle
	check  *glyph
	onTap  func()
}

func newMarkDot(tap func()) *markDot {
	m := &markDot{onTap: tap}
	m.circle = dot(14, color.Transparent, rgba(style.Line), 1.5)
	m.check = newGlyph(glyphCheck, rgba(style.Subtle))
	m.ExtendBaseWidget(m)
	return m
}

func (m *markDot) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewCenter(container.NewStack(fixed(m.circle, 14, 14), fixed(m.check, 14, 14))))
}

func (m *markDot) MinSize() fyne.Size { return fyne.NewSize(20, 20) }

func (m *markDot) set(marked, finished bool) {
	switch {
	case marked:
		m.circle.FillColor, m.circle.StrokeColor = rgba(style.Accent), rgba(style.Accent)
	default:
		m.circle.FillColor, m.circle.StrokeColor = color.Transparent, rgba(style.Line)
	}
	m.circle.Refresh()
	setShown(m.check, finished && !marked)
	m.Refresh()
}

func (m *markDot) Tapped(*fyne.PointEvent) {
	if m.onTap != nil {
		m.onTap()
	}
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
