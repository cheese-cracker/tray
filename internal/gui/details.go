package gui

import (
	"fmt"
	"image/color"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/style"
)

// details is the ladder drawn for one task, top rung down: words, tags, then what the
// tray asks for, the note, the schedule behind a fold, where it came from, and the id
// in grey at the bottom. A rung that holds something is a filled marker on the rail; an
// empty one is hollow with a hint, never a prompt; the first empty one after the last
// filled is the next step, and says so in the accent (T12).
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
	note   *noteEntry
	recur  *escEntry
	wait   *escEntry
	until  *escEntry
	more   *fold
	source *canvas.Text
	id     *canvas.Text

	rWords, rTags, rTray, rNote, rMore, rSource, rID *rung
	rungs                                            []*rung
	fields                                           *fyne.Container
	box                                              *fyne.Container
}

func newDetails(u *ui) *details {
	d := &details{u: u}
	d.empty = grey("Nothing to show — land on a row.")
	d.words = d.entry(func(t *core.Task, s string) {
		if s = flatten(s); s != "" {
			t.Text = s
		}
	})
	d.tags = d.entry(func(t *core.Task, s string) { t.Tags = strings.Fields(s) })
	d.tags.SetPlaceHolder("space separated")
	d.pri = widget.NewRadioGroup([]string{"H", "M", "L"}, func(p string) {
		if !d.filling && p != "" {
			d.commit(func(t *core.Task) { t.Priority = p })
		}
	})
	d.pri.Horizontal = true
	d.due = newDueEntry(u.focusList)
	d.due.OnSubmitted = func(s string) { d.commit(func(t *core.Task) { t.Due = strings.TrimSpace(s) }) }
	d.hint = hint("")
	d.note = newNoteEntry(u.focusList, func() {
		d.commit(func(t *core.Task) { t.Note = strings.TrimSpace(d.note.Text) })
	})
	d.recur = d.entry(func(t *core.Task, s string) { t.Recur = strings.TrimSpace(s) })
	d.recur.SetPlaceHolder("weekly · monthly · 2w")
	d.wait = d.entry(func(t *core.Task, s string) { t.Wait = strings.TrimSpace(s) })
	d.wait.SetPlaceHolder("the day it lands on the tray")
	d.until = d.entry(func(t *core.Task, s string) { t.Until = strings.TrimSpace(s) })
	d.until.SetPlaceHolder("when it stops")
	d.source = mono("", style.Subtle)
	d.id = mono("", style.Subtle)

	schedule := container.NewVBox(labelled("recur", d.recur), labelled("wait", d.wait), labelled("until", d.until))
	d.more = newFold(schedule, func() {
		if d.fields != nil {
			d.fields.Refresh()
		}
	})

	d.rWords = newRung("words", nil, d.words, nil)
	d.rTags = newRung("tags", nil, d.tags, hint("no tags yet — press #"))
	d.rTray = newRung("the tray asks", nil,
		container.NewVBox(labelled("priority", d.pri), labelled("due", d.due.withDay())), d.hint)
	d.rNote = newRung("note", nil, d.note, hint("no note — press n"))
	d.rMore = newRung("schedule", d.more.head("schedule"), d.more.box, hint("not scheduled — repeat, or wait for a day"))
	d.rSource = newRung("source", nil, d.source, nil)
	d.rID = newRung("", container.NewHBox(container.NewCenter(d.id), newLink("copy context", d.copyContext).hush()), nil, nil)
	d.rID.rail.Hide() // the ladder ends here
	d.rungs = []*rung{d.rWords, d.rTags, d.rTray, d.rNote, d.rMore, d.rSource, d.rID}

	d.fields = container.NewVBox()
	for _, r := range d.rungs {
		d.fields.Add(r.box)
	}
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
	same := d.has && ok && d.t.ID == t.ID // the same task after a change: rungs may fill
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
	setShown(d.rTray.box, tray)
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
	scheduled := t.Recur != "" || t.Wait != "" || t.Until != ""
	if !same {
		d.more.setOpen(scheduled, false)
	}
	d.source.Text = t.Source
	d.source.Refresh()
	setShown(d.rSource.box, t.Source != "")
	d.id.Text = fmt.Sprintf("#%d", t.ID)
	d.id.Refresh()

	filled := map[*rung]bool{
		d.rWords: true, d.rTags: len(t.Tags) > 0, d.rTray: t.Priority != "" && t.Due != "",
		d.rNote: t.Note != "", d.rMore: scheduled, d.rSource: t.Source != "", d.rID: true,
	}
	var next *rung
	for _, r := range d.rungs {
		if r != d.rID && r.box.Visible() && !filled[r] {
			next = r
			break
		}
	}
	for _, r := range d.rungs {
		r.paint(filled[r], r == next, same)
	}
	d.rID.marker.FillColor, d.rID.marker.StrokeColor = rgba(style.Subtle), rgba(style.Subtle)
	d.rID.marker.Refresh()
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

// hint is an empty rung's line: quiet, italic, never a prompt.
func hint(s string) *canvas.Text {
	t := grey(s)
	t.TextStyle.Italic = true
	return t
}

func labelled(name string, o fyne.CanvasObject) fyne.CanvasObject {
	return container.NewBorder(nil, nil, container.NewCenter(fixed(caption(name, style.Ink2), 52, 16)), nil, o)
}

// rung is one step of the ladder: a marker on the rail, its name, what it holds, and
// the hint shown while it holds nothing.
type rung struct {
	label  *canvas.Text
	marker *canvas.Circle
	rail   *canvas.Rectangle
	hint   *canvas.Text
	box    *fyne.Container
	filled bool
	next   bool
}

func newRung(name string, head, body fyne.CanvasObject, hint *canvas.Text) *rung {
	r := &rung{hint: hint}
	r.label = caption(strings.ToUpper(name), style.Ink2)
	r.label.TextStyle.Bold = true
	r.marker = dot(12, color.Transparent, rgba(style.Line), 1.5)
	r.rail = canvas.NewRectangle(rgba(style.Line))
	if head == nil {
		head = r.label
	}
	items := []fyne.CanvasObject{head}
	if body != nil {
		items = append(items, body)
	}
	if hint != nil {
		items = append(items, hint)
	}
	rail := container.New(&railLayout{}, r.marker, r.rail)
	r.box = container.NewBorder(nil, nil, rail, nil, container.NewPadded(container.NewVBox(items...)))
	return r
}

// paint draws the rung's state. On the same task a rung that just filled eases from
// the paper to the accent, and the new next step fades in, so a change reads as a climb.
func (r *rung) paint(filled, next, animate bool) {
	wasFilled, wasNext := r.filled, r.next
	r.filled, r.next = filled, next
	label := style.Ink2
	if next {
		label = style.Accent
	}
	switch {
	case filled:
		r.marker.StrokeColor, r.marker.StrokeWidth = rgba(style.Accent), 1.5
		if animate && !wasFilled {
			canvas.NewColorRGBAAnimation(rgba(style.Paper), rgba(style.Accent), settle, func(c color.Color) {
				r.marker.FillColor = c
				r.marker.Refresh()
			}).Start()
		} else {
			r.marker.FillColor = rgba(style.Accent)
		}
	case next:
		r.marker.FillColor, r.marker.StrokeColor, r.marker.StrokeWidth = color.Transparent, rgba(style.Accent), 2
	default:
		r.marker.FillColor, r.marker.StrokeColor, r.marker.StrokeWidth = color.Transparent, rgba(style.Line), 1.5
	}
	r.marker.Refresh()
	if animate && next && !wasNext {
		fade(r.label, rgba(style.Paper), rgba(label), unfold)
	} else {
		r.label.Color = rgba(label)
		r.label.Refresh()
	}
	if r.hint != nil {
		setShown(r.hint, !filled)
	}
}

// railLayout is the marker at the top of a rung and the line running down from it to the
// next one, in a column 24 wide.
type railLayout struct{}

func (railLayout) MinSize([]fyne.CanvasObject) fyne.Size { return fyne.NewSize(24, 20) }

func (railLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	marker, rail := objs[0], objs[1]
	marker.Resize(fyne.NewSize(12, 12))
	marker.Move(fyne.NewPos(6, 8))
	rail.Resize(fyne.NewSize(2, max(size.Height-22, 0)))
	rail.Move(fyne.NewPos(11, 22))
}

// fold is a section behind a chevron: closed, it costs one line; open, it unfolds over
// a few frames rather than jumping.
type fold struct {
	chev    *glyph
	content fyne.CanvasObject
	scroll  *container.Scroll
	box     *fyne.Container
	shown   float32 // 0 closed, 1 open
	open    bool
	onGrow  func()
}

func newFold(content fyne.CanvasObject, onGrow func()) *fold {
	f := &fold{content: content, onGrow: onGrow}
	f.chev = newGlyph(glyphChevron, rgba(style.Ink2))
	// A scroll clips to its bounds, which is the whole reason it is here: while the fold
	// opens the fields are cut at the moving edge rather than drawn over the next rung.
	f.scroll = container.NewVScroll(content)
	f.box = container.New(&foldLayout{f}, f.scroll)
	return f
}

// head is the rung's label with the chevron beside it; tapping either turns the fold.
func (f *fold) head(name string) fyne.CanvasObject {
	label := caption(strings.ToUpper(name), style.Ink2)
	label.TextStyle.Bold = true
	return newTap(container.NewHBox(container.NewCenter(label), fixed(f.chev, 14, 14)), f.toggle)
}

func (f *fold) toggle() { f.setOpen(!f.open, true) }

func (f *fold) setOpen(open, animate bool) {
	f.open = open
	to := float32(0)
	if open {
		to = 1
	}
	if !animate {
		f.chev.open = to
		f.chev.Refresh()
		f.grow(to)
		return
	}
	f.chev.turn(open)
	from := f.shown
	fyne.NewAnimation(unfold, func(p float32) { f.grow(lerp(from, to, p)) }).Start()
}

func (f *fold) grow(p float32) {
	f.shown = p
	// A clip of zero height is no clip at all to the painter, so a closed fold hides
	// its fields outright rather than trusting a zero-sized scroll to cut them.
	setShown(f.scroll, p > 0)
	f.box.Refresh()
	if f.onGrow != nil {
		f.onGrow()
	}
}

type foldLayout struct{ f *fold }

func (l *foldLayout) MinSize([]fyne.CanvasObject) fyne.Size {
	min := l.f.content.MinSize()
	return fyne.NewSize(min.Width, min.Height*l.f.shown)
}

func (l *foldLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	objs[0].Resize(size)
	objs[0].Move(fyne.NewPos(0, 0))
}

// tap makes any content tappable.
type tap struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onTap   func()
}

func newTap(content fyne.CanvasObject, onTap func()) *tap {
	t := &tap{content: content, onTap: onTap}
	t.ExtendBaseWidget(t)
	return t
}

func (t *tap) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(t.content) }
func (t *tap) Tapped(*fyne.PointEvent) {
	if t.onTap != nil {
		t.onTap()
	}
}
