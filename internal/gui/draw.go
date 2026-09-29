package gui

import (
	"image/color"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/charmbracelet/lipgloss"

	"github.com/cheese-cracker/tray/internal/style"
)

// The small vocabulary every screen is drawn with. Glyphs the house font lacks — a
// check, a chevron, the note sign, a rung — are lines and circles, so nothing here
// depends on which fonts a machine happens to have.

const (
	flick  = 100 * time.Millisecond // a row's actions arriving
	quick  = 120 * time.Millisecond // a modal, a bar, the palette
	settle = 200 * time.Millisecond // a rung filling
	unfold = 160 * time.Millisecond // a section opening
	linger = 400 * time.Millisecond // a new row's fill fading out
)

func text(s string, c lipgloss.AdaptiveColor) *canvas.Text {
	t := canvas.NewText(s, rgba(c))
	t.TextSize = theme.Size(theme.SizeNameText)
	return t
}

// caption is the small label: a rung's name, a date's weekday, a last run.
func caption(s string, c lipgloss.AdaptiveColor) *canvas.Text {
	t := text(s, c)
	t.TextSize = theme.Size(theme.SizeNameCaptionText)
	return t
}

// mono is Fira Code: ids, dates, keys — anything read as a value rather than a phrase.
func mono(s string, c lipgloss.AdaptiveColor) *canvas.Text {
	t := text(s, c)
	t.TextStyle.Monospace = true
	return t
}

func semibold(s string, c lipgloss.AdaptiveColor) *canvas.Text {
	t := text(s, c)
	t.TextStyle.Bold = true
	return t
}

// grey is a line that informs without asking to be read first.
func grey(s string) *canvas.Text { return text(s, style.Subtle) }

func plain(s string) *canvas.Text { return text(s, style.Ink) }

// warn is the one line allowed to shout: a plugin that failed or stopped to ask.
func warn(s string) *canvas.Text { return text(s, style.High) }

// rounded is a filled rectangle with soft corners: a chip, a card, a pill, a banner.
func rounded(c lipgloss.AdaptiveColor, radius float32) *canvas.Rectangle {
	r := canvas.NewRectangle(rgba(c))
	r.CornerRadius = radius
	return r
}

// dot is a circle of a fixed size, filled or hollow.
func dot(size float32, fill, stroke color.Color, width float32) *canvas.Circle {
	c := canvas.NewCircle(fill)
	c.StrokeColor, c.StrokeWidth = stroke, width
	c.Resize(fyne.NewSize(size, size))
	return c
}

// fixed pins an object to one size inside any layout.
func fixed(o fyne.CanvasObject, w, h float32) fyne.CanvasObject {
	return container.New(&fixedLayout{fyne.NewSize(w, h)}, o)
}

type fixedLayout struct{ size fyne.Size }

func (l *fixedLayout) MinSize([]fyne.CanvasObject) fyne.Size { return l.size }
func (l *fixedLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Resize(l.size)
		o.Move(fyne.NewPos((size.Width-l.size.Width)/2, (size.Height-l.size.Height)/2))
	}
}

// vrule is a one-pixel line across; vline the same, down. Borders are lines here, never
// filled bars: the paper stays one surface with a few seams.
func vrule() fyne.CanvasObject {
	r := canvas.NewRectangle(rgba(style.Line))
	r.SetMinSize(fyne.NewSize(0, 1))
	return r
}

func vline() fyne.CanvasObject {
	r := canvas.NewRectangle(rgba(style.Line))
	r.SetMinSize(fyne.NewSize(1, 0))
	return r
}

// inset pads an object by exact amounts, where the theme's one padding is too much or
// too little: a chip, a header, a status bar.
func inset(o fyne.CanvasObject, x, y float32) fyne.CanvasObject {
	return container.New(&insetLayout{x, y}, o)
}

type insetLayout struct{ x, y float32 }

func (l *insetLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	s := objs[0].MinSize()
	return fyne.NewSize(s.Width+2*l.x, s.Height+2*l.y)
}

func (l *insetLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	objs[0].Move(fyne.NewPos(l.x, l.y))
	objs[0].Resize(fyne.NewSize(size.Width-2*l.x, size.Height-2*l.y))
}

// card is the one shell every modal wears — the palette, a form, a picker: a soft
// rectangle with a hairline edge, and it arrives rather than appears (slideIn).
func card(content fyne.CanvasObject) fyne.CanvasObject {
	bg := rounded(style.Card, 8)
	bg.StrokeColor, bg.StrokeWidth = rgba(style.Line), 1
	return container.NewStack(bg, inset(content, 14, 12))
}

// slideIn drops content eight pixels into place while its shell fades up from nothing.
// Under 150 ms, eased, so it reads as arriving, not bouncing.
func slideIn(content fyne.CanvasObject) fyne.CanvasObject {
	l := &slideLayout{dy: -8}
	box := container.New(l, content)
	a := fyne.NewAnimation(quick, func(f float32) {
		l.dy = -8 * (1 - f)
		box.Refresh()
	})
	a.Curve = fyne.AnimationEaseOut
	a.Start()
	return box
}

type slideLayout struct{ dy float32 }

func (l *slideLayout) MinSize(objs []fyne.CanvasObject) fyne.Size { return objs[0].MinSize() }
func (l *slideLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	objs[0].Resize(size)
	objs[0].Move(fyne.NewPos(0, l.dy))
}

// fade eases a text from one colour to another. The test driver ticks an animation to
// its end at once, so a flow and a golden see the final colour.
func fade(t *canvas.Text, from, to color.Color, d time.Duration) {
	canvas.NewColorRGBAAnimation(from, to, d, func(c color.Color) {
		t.Color = c
		t.Refresh()
	}).Start()
}

// glyph is a check, a chevron or the note sign, drawn with lines.
type glyph struct {
	widget.BaseWidget
	kind  glyphKind
	color color.Color
	open  float32 // chevron: 0 points right, 1 points down
	lines []*canvas.Line
}

type glyphKind int

const (
	glyphCheck glyphKind = iota
	glyphChevron
	glyphNote
)

func newGlyph(kind glyphKind, c color.Color) *glyph {
	g := &glyph{kind: kind, color: c}
	n := 2
	if kind == glyphNote {
		n = 3
	}
	for i := 0; i < n; i++ {
		l := canvas.NewLine(c)
		l.StrokeWidth = 1.6
		g.lines = append(g.lines, l)
	}
	g.ExtendBaseWidget(g)
	return g
}

func (g *glyph) MinSize() fyne.Size { return fyne.NewSize(12, 12) }

func (g *glyph) CreateRenderer() fyne.WidgetRenderer {
	objs := make([]fyne.CanvasObject, len(g.lines))
	for i, l := range g.lines {
		objs[i] = l
	}
	return &glyphRenderer{g: g, objs: objs}
}

type glyphRenderer struct {
	g    *glyph
	objs []fyne.CanvasObject
}

func (r *glyphRenderer) MinSize() fyne.Size           { return r.g.MinSize() }
func (r *glyphRenderer) Objects() []fyne.CanvasObject { return r.objs }
func (r *glyphRenderer) Destroy()                     {}
func (r *glyphRenderer) Refresh() {
	for _, l := range r.g.lines {
		l.StrokeColor = r.g.color
		l.Refresh()
	}
}

func (r *glyphRenderer) Layout(size fyne.Size) {
	w, h := size.Width, size.Height
	cx, cy := w/2, h/2
	set := func(l *canvas.Line, x1, y1, x2, y2 float32) {
		l.Position1, l.Position2 = fyne.NewPos(x1, y1), fyne.NewPos(x2, y2)
		l.Refresh()
	}
	switch r.g.kind {
	case glyphCheck:
		set(r.g.lines[0], w*0.2, cy, w*0.42, h*0.75)
		set(r.g.lines[1], w*0.42, h*0.75, w*0.82, h*0.28)
	case glyphChevron:
		// Right-pointing at 0, down-pointing at 1; in between, the arms turn.
		o := r.g.open
		ax, ay := lerp(cx-3, cx-4, o), lerp(cy-4, cy-2, o)
		bx, by := lerp(cx+1, cx, o), lerp(cy, cy+2, o)
		ex, ey := lerp(cx-3, cx+4, o), lerp(cy+4, cy-2, o)
		set(r.g.lines[0], ax, ay, bx, by)
		set(r.g.lines[1], bx, by, ex, ey)
	case glyphNote:
		for i, l := range r.g.lines {
			y := h*0.3 + float32(i)*h*0.2
			end := w * 0.8
			if i == 2 {
				end = w * 0.55
			}
			set(l, w*0.2, y, end, y)
		}
	}
}

func lerp(a, b, t float32) float32 { return a + (b-a)*t }

// turn animates the chevron between its two directions.
func (g *glyph) turn(open bool) {
	to := float32(0)
	if open {
		to = 1
	}
	from := g.open
	fyne.NewAnimation(unfold, func(f float32) {
		g.open = lerp(from, to, f)
		g.Refresh()
	}).Start()
}

// link is a quiet action: accent text that does something when tapped, and darkens
// under the pointer. It is what a row's verbs, a pane's buttons and copy context wear.
type link struct {
	widget.BaseWidget
	Text   string
	label  *canvas.Text
	OnTap  func()
	hushed bool // drawn in ink2 until hovered, for actions that should not compete
}

func newLink(text string, tap func()) *link {
	l := &link{Text: text, OnTap: tap}
	l.label = canvas.NewText(text, rgba(style.Accent))
	l.label.TextSize = theme.Size(theme.SizeNameText)
	l.ExtendBaseWidget(l)
	return l
}

// hush draws the link in ink2 until hovered: an action that is there, not one that asks.
func (l *link) hush() *link {
	l.hushed = true
	l.label.Color = rgba(style.Ink2)
	return l
}

func (l *link) SetText(s string) {
	l.Text = s
	l.label.Text = s
	l.label.Refresh()
	l.Refresh()
}

func (l *link) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewCenter(l.label))
}

func (l *link) MinSize() fyne.Size {
	s := l.label.MinSize()
	return fyne.NewSize(s.Width+8, s.Height+4)
}

func (l *link) Tapped(*fyne.PointEvent) {
	if l.OnTap != nil {
		l.OnTap()
	}
}

func (l *link) Cursor() desktop.Cursor { return desktop.PointerCursor }

func (l *link) MouseIn(*desktop.MouseEvent)    { fade(l.label, l.label.Color, rgba(style.Ink), quick) }
func (l *link) MouseMoved(*desktop.MouseEvent) {}
func (l *link) MouseOut() {
	c := style.Accent
	if l.hushed {
		c = style.Ink2
	}
	fade(l.label, l.label.Color, rgba(c), quick)
}

// appear fades the link in from the paper, for actions that arrive on hover.
func (l *link) appear() {
	c := style.Accent
	if l.hushed {
		c = style.Ink2
	}
	fade(l.label, rgba(style.Paper), rgba(c), flick)
}

// primary is the one filled button a screen may have: apply, save, take.
func primary(label string, tap func()) *widget.Button {
	b := widget.NewButton(label, tap)
	b.Importance = widget.HighImportance
	return b
}

// lowButton is a bordered quiet button, for a form's cancel and a picker's rows.
func lowButton(label string, tap func()) *widget.Button {
	b := widget.NewButton(label, tap)
	b.Importance = widget.LowImportance
	return b
}

// chip is a tag as the screen draws it: #tag on a soft accent pill, no taller than the
// line it sits in.
func chip(label string) fyne.CanvasObject {
	t := caption(label, style.Ink2)
	bg := rounded(style.AccentSoft, 8)
	return container.NewStack(bg, inset(container.NewCenter(t), 6, 1))
}
