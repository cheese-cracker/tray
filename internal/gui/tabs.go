package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/cheese-cracker/tray/internal/style"
)

// tabs is a segmented control: the layers as pills, the chosen one on a soft accent
// fill, and one content shown under them. It keeps AppTabs' shape — Items, SelectIndex,
// SelectedIndex, OnSelected — so a screen and its tests read the same either way.
type tabs struct {
	widget.BaseWidget
	Items      []*container.TabItem
	OnSelected func(*container.TabItem)
	selected   int
	pills      []*pill
	bar        *fyne.Container
	body       *fyne.Container
}

func newTabs(items ...*container.TabItem) *tabs {
	t := &tabs{Items: items, bar: container.NewHBox(), body: container.NewStack()}
	for i, it := range items {
		i := i
		p := newPill(it.Text, func() { t.SelectIndex(i) })
		t.pills = append(t.pills, p)
		t.bar.Add(p)
		t.body.Add(it.Content)
	}
	t.ExtendBaseWidget(t)
	t.show()
	return t
}

func (t *tabs) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewBorder(container.NewPadded(t.bar), nil, nil, nil, t.body))
}

func (t *tabs) SelectedIndex() int { return t.selected }

// SelectIndex shows a tab and tells the owner, as AppTabs does, whether or not it changed.
func (t *tabs) SelectIndex(i int) {
	if i < 0 || i >= len(t.Items) {
		return
	}
	t.selected = i
	t.show()
	if t.OnSelected != nil {
		t.OnSelected(t.Items[i])
	}
}

func (t *tabs) show() {
	for i, it := range t.Items {
		setShown(it.Content, i == t.selected)
		t.pills[i].active = i == t.selected
		t.pills[i].Refresh()
	}
}

// pill is one tab's button.
type pill struct {
	widget.BaseWidget
	label  *canvas.Text
	bg     fyne.CanvasObject
	active bool
	onTap  func()
}

func newPill(text string, tap func()) *pill {
	p := &pill{onTap: tap}
	p.label = semibold(text, style.Ink2)
	p.bg = rounded(style.AccentSoft, 12)
	p.ExtendBaseWidget(p)
	return p
}

func (p *pill) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(container.NewStack(p.bg, container.NewPadded(container.NewCenter(p.label))))
}

func (p *pill) MinSize() fyne.Size {
	s := p.label.MinSize()
	return fyne.NewSize(s.Width+20, s.Height+10)
}

func (p *pill) Refresh() {
	if p.active {
		p.label.Color = rgba(style.Ink)
		p.bg.Show()
	} else {
		p.label.Color = rgba(style.Ink2)
		p.bg.Hide()
	}
	p.label.Refresh()
	p.BaseWidget.Refresh()
}

func (p *pill) Tapped(*fyne.PointEvent) {
	if p.onTap != nil {
		p.onTap()
	}
}
