package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
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

const diagram = `  garage  ──take──▶  tray  ──done──▶  finished
  any words          priority · due · tags     a date, where it sits
     ▲                  │
     └────hand back─────┘`

// openHelp is a page, not a keymap strip (86): what needs explaining is why there
// are two layers at all, so the prose comes first, the picture second, and the
// shortcuts last under a rule. Any key or click closes it.
func (u *ui) openHelp() {
	prose := widget.NewLabel("Write anything into the garage — a few words, no questions.\n" +
		"Take a line onto the tray when you are ready to work on it; that is when\n" +
		"it asks for a priority, a date and tags. A task gets clearer in steps, each\n" +
		"one when you are ready and never before.")
	pic := widget.NewLabelWithStyle(diagram, fyne.TextAlignLeading, fyne.TextStyle{Monospace: true})

	keys := container.NewGridWithColumns(2)
	for _, k := range [][2]string{
		{"↑ ↓  j k", "move"}, {"tab", "switch layer"}, {"space", "select"}, {"enter  l", "open the pane"},
		{"t", "take"}, {"x", "done"}, {"d", "hand back"}, {">", "move to a month"},
		{"r", "rewrite"}, {"#", "tag"}, {"n", "note"}, {"a", "add"},
		{"/", "filter"}, {"esc", "clear the filter, then quit"}, {"?", "this page"}, {"q", "quit"},
	} {
		keys.Add(widget.NewLabelWithStyle(k[0], fyne.TextAlignLeading, fyne.TextStyle{Monospace: true}))
		keys.Add(widget.NewLabel(k[1]))
	}

	p := newPage(container.NewVBox(prose, pic, widget.NewSeparator(), keys, grey("any key closes this")))
	p.onKey = func(*fyne.KeyEvent) { u.hide() }
	p.onRune = func(rune) { u.hide() }
	p.onTap = u.hide
	u.show(p, p)
}
