package gui

import (
	"image/color"
	"sort"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
	"github.com/cheese-cracker/tray/internal/style"
)

// screen is a set of lists under tabs — home's two layers, review's two, the sweep's
// months — with the loader that fills them. The window shows one at a time.
type screen struct {
	tabs  *container.AppTabs
	lists []*taskList
	load  func(u *ui) error
}

// enter swaps the window's root for a mode's and gives it the keys. A mode is never
// stacked on another, so any popup closes first.
func (u *ui) enter(m mode, root fyne.CanvasObject) {
	u.hide()
	u.mode = m
	u.win.SetContent(root)
	u.reload()
	u.focusList()
}

// leave puts home back and forgets the mode's screen; marks made there were for it.
func (u *ui) leave() {
	u.hide()
	u.mode = modeHome
	u.rv, u.sw, u.view, u.pane, u.page = nil, nil, nil, nil, nil
	u.marks = map[int64]bool{}
	u.win.SetContent(u.root)
	u.reload()
	u.focusList()
}

// banner names the mode and the way out, above the table, where you look on arriving
// (92f). Its colour says which mode before a word is read (92g).
func banner(text string, tint color.RGBA) fyne.CanvasObject {
	tint.A = 0x60 // a wash, so the label keeps the theme's own contrast
	bg := canvas.NewRectangle(tint)
	label := widget.NewLabelWithStyle(text, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	return container.NewStack(bg, label)
}

// rule is the frame: a line in the mode's colour above and below the table.
func rule(tint color.RGBA) fyne.CanvasObject {
	r := canvas.NewRectangle(tint)
	r.SetMinSize(fyne.NewSize(0, 3))
	return r
}

// openReview widens each layer to everything on it and narrows the keys to restore and
// erase (92): a review pass shows what you finished next to what you did not, and the
// two verbs you reach for monthly live here and nowhere else. Amber, because those two
// are a correction and a removal — not danger, not the daily flow either.
func (u *ui) openReview() {
	amber := style.RGBA(style.Review, dark())
	garage := newTaskList(u, core.LayerGarage, reviewVerbs)
	garage.month, garage.review = store.ThisMonth(), true
	tray := newTaskList(u, core.LayerTray, reviewVerbs)
	tray.review = true
	tabs := container.NewAppTabs(
		container.NewTabItem("garage · "+store.ThisMonth(), garage),
		container.NewTabItem("tray", tray),
	)
	tabs.OnSelected = u.tabChanged
	tabs.SelectIndex(u.tabs.SelectedIndex()) // review the layer you were looking at
	u.rv = &screen{tabs: tabs, lists: []*taskList{garage, tray}, load: loadReview}

	root := container.NewBorder(
		container.NewVBox(banner("review — everything on the layer · R restore · E erase · v or esc leaves", amber), u.top, rule(amber)),
		container.NewVBox(rule(amber), u.bottom), nil, nil, u.split(tabs))
	u.enter(modeReview, root)
}

// loadReview is everything on the layer, in review order (92d).
func loadReview(u *ui) error {
	today := store.Today()
	for _, l := range u.rv.lists {
		f := store.Filter{Layer: l.layer, Month: l.month, All: true}
		rows, err := u.s.Tasks(f)
		if err != nil {
			return err
		}
		sort.SliceStable(rows, func(i, j int) bool {
			ri, rj := rank(rows[i], today), rank(rows[j], today)
			if ri != rj {
				return ri < rj
			}
			return ri == 0 && l.layer == core.LayerTray &&
				core.Urgency(rows[i], today) > core.Urgency(rows[j], today)
		})
		l.load(rows)
	}
	return nil
}

// rank is the review order: the work you have left, then what waits for its day, then
// what you finished, then the templates that make more of it. A finished H task still
// computes an urgency, so ranking on that alone would float it over the work left (92d).
func rank(t core.Task, today time.Time) int {
	switch {
	case t.Recur != "":
		return 3
	case t.Done != "":
		return 2
	case t.Waiting(today):
		return 1
	}
	return 0
}
