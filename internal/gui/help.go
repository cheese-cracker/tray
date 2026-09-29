package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/cheese-cracker/tray/internal/style"
)

// page is content that takes the keyboard: a help screen any key dismisses, a picker a
// digit answers. It accepts Tab so the driver hands that over too.
type page struct {
	widget.BaseWidget
	content fyne.CanvasObject
	onKey   func(*fyne.KeyEvent)
	onRune  func(rune)
	onTap   func()
}

func newPage(content fyne.CanvasObject) *page {
	p := &page{content: content}
	p.ExtendBaseWidget(p)
	return p
}

func (p *page) CreateRenderer() fyne.WidgetRenderer { return widget.NewSimpleRenderer(p.content) }
func (p *page) AcceptsTab() bool                    { return true }
func (p *page) FocusGained()                        {}
func (p *page) FocusLost()                          {}

func (p *page) TypedKey(k *fyne.KeyEvent) {
	if p.onKey != nil {
		p.onKey(k)
	}
}

func (p *page) TypedRune(r rune) {
	if p.onRune != nil {
		p.onRune(r)
	}
}

func (p *page) Tapped(*fyne.PointEvent) {
	if p.onTap != nil {
		p.onTap()
	}
}

// openHelp is a page, not a keymap strip (86): what needs explaining is why there
// are two layers at all, so the prose comes first, the picture second, and the
// shortcuts last under a rule. Any key or click closes it.
func (u *ui) openHelp() {
	title := semibold("Two layers, and a task that gets clearer in steps", style.Ink)
	title.TextSize = theme.Size(theme.SizeNameHeadingText)

	prose := widget.NewRichText(
		&widget.TextSegment{Style: widget.RichTextStyle{SizeName: sizeProse}, Text: "Write anything into the garage — a few words, no questions. " +
			"Take a line onto the tray when you are ready to work on it; that is when it asks for a priority, a date and tags. " +
			"Everything else — a note, a schedule, where it came from — is a rung it climbs later, each one when you are ready and never before."},
	)
	prose.Wrapping = fyne.TextWrapWord

	// The picture: three places and the two moves between them.
	box := func(name, what string) fyne.CanvasObject {
		return container.NewStack(rounded(style.Card, 8),
			container.NewPadded(container.NewVBox(semibold(name, style.Ink), caption(what, style.Ink2))))
	}
	arrow := func(verb string) fyne.CanvasObject {
		return container.NewCenter(container.NewVBox(caption(verb, style.Accent), text("→", style.Accent)))
	}
	pic := container.NewVBox(
		container.NewHBox(box("garage", "any words"), arrow("take"), box("tray", "priority · due · tags"), arrow("done"), box("finished", "a date, where it sits")),
		caption("← hand back returns a line to the garage, keeping what it learned", style.Ink2),
	)

	// Two pairs a line: twenty keys in ten rows fit under the picture on a laptop screen.
	keys := container.NewGridWithColumns(4)
	for _, k := range [][2]string{
		{"↑ ↓  j k", "move"}, {"tab", "switch layer"}, {"space", "select"}, {"l", "open the pane"},
		{"enter  :", "the command palette — every action, typed at"}, {"ctrl+shift+p", "the palette, from anywhere"},
		{"t", "take"}, {"x", "done"}, {"d", "hand back"}, {">", "move to a month"},
		{"r", "rewrite"}, {"#", "tag"}, {"n", "note"}, {"a", "add"},
		{"v", "review — R restore and E erase live there"}, {"s", "sync"}, {"p", "plugins"}, {"c", "copy context"},
		{"/", "filter — hidden until you ask"}, {"esc", "clear the filter, leave the mode, then quit"}, {"?", "this page"}, {"q", "quit"},
	} {
		keys.Add(mono(k[0], style.Ink))
		keys.Add(text(k[1], style.Ink2))
	}

	body := container.NewVBox(title, prose, pic, vrule(), keys, caption("any key closes this", style.Subtle))
	p := newPage(container.NewStack(rounded(style.Paper, 8), container.NewPadded(body)))
	p.onKey = func(*fyne.KeyEvent) { u.hide() }
	p.onRune = func(rune) { u.hide() }
	p.onTap = u.hide
	u.show(p, p)
}
