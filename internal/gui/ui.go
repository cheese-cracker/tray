// Package gui is the desktop app: a client of store and core that owns no rule of its
// own. Simple actions are one step from a row; anything that asks a question opens a
// form; anything that destroys, bulk-moves or brings data in gets a mode of its own.
// The TUI's letters survive as shortcuts, not as the design.
package gui

import (
	"fmt"
	"image/color"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/cheese-cracker/tray/internal/cli"
	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
	"github.com/cheese-cracker/tray/internal/style"
	"github.com/cheese-cracker/tray/internal/sync"
)

// mode is which screen owns the window. Each one past home is a root of its own, with
// its own verbs and an exit named in its banner (T11).
type mode int

const (
	modeHome mode = iota
	modeReview
	modeSweep
	modeSyncReview
	modePlugins
)

// syncTimeout bounds one plugin run from the app; the CLI's --timeout defaults the same.
const syncTimeout = 10 * time.Minute

type ui struct {
	s    *store.Store
	win  fyne.Window
	mode mode

	garage, tray *taskList
	tabs         *tabs
	home         *screen
	rv, sw       *screen // review and the sweep, built on entering
	search       *escEntry
	hidden       *canvas.Text
	top          *fyne.Container
	capture      *escEntry
	status       *widget.Label
	notice       *link
	bottom       *fyne.Container
	details      *details
	root         fyne.CanvasObject

	filter   string
	marks    map[int64]bool
	pop      *widget.PopUp
	form     *form // the open form, so a test can fill it in
	closed   bool
	flash    string // one line about the last action, shown until the next
	flashNew bool   // the next save is a capture: light its row on the way in
	flashID  int64

	pending  []sync.Result // what plugins reported at launch, waiting for a review
	syncedAt string
	syncing  bool
	view     *syncView    // the open sync review
	pane     *pluginsPane // the open plugins pane
	page     *page        // holds the keys in a mode that has no list
}

func newUI(s *store.Store, w fyne.Window) *ui {
	u := &ui{s: s, win: w, marks: map[int64]bool{}}
	u.details = newDetails(u)
	u.garage = newTaskList(u, core.LayerGarage, garageVerbs)
	u.garage.month = store.ThisMonth()
	u.tray = newTaskList(u, core.LayerTray, trayVerbs)

	// Capture costs nothing: words, Enter, done. The bar reads to: and +tag off the
	// front exactly as `tray dump` does, and the rest is literal.
	u.capture = newEscEntry(u.focusList)
	u.capture.SetPlaceHolder("dump a line — to:2026-11 +tag the words…")
	u.capture.OnSubmitted = func(text string) {
		u.dump(text)
		u.capture.SetText("")
	}

	add := newLink("+ add", func() { u.do("a", u.tray) })
	sweep := newLink("carry forward…", u.openSweep).hush()
	unload := newLink("hand the tray back…", u.openUnload).hush()

	u.tabs = newTabs(
		container.NewTabItem("garage · "+store.ThisMonth(),
			container.NewBorder(nil, container.NewPadded(container.NewBorder(nil, nil, nil, sweep, u.capture)), nil, nil, u.garage.view())),
		container.NewTabItem("tray",
			container.NewBorder(nil, container.NewPadded(container.NewBorder(nil, nil, add, unload)), nil, nil, u.tray.view())),
	)
	u.tabs.OnSelected = u.tabChanged
	u.home = &screen{tabs: u.tabs, lists: []*taskList{u.garage, u.tray}, load: loadHome}

	u.search = newEscEntry(func() {
		u.clearFilter()
		u.focusList()
	})
	u.search.SetPlaceHolder("/ filter by words or tag")
	u.search.OnChanged = func(text string) {
		u.filter = text
		u.refilter()
	}
	u.hidden = caption("", style.Subtle)
	u.status = widget.NewLabel("")
	u.status.SizeName = theme.SizeNameCaptionText
	u.status.Importance = widget.LowImportance
	u.notice = newLink("", u.openPending)
	u.notice.Hide()
	u.bottom = container.NewPadded(container.NewBorder(nil, nil, nil, u.notice, u.status))

	u.top = container.NewPadded(container.NewBorder(nil, nil, nil, container.NewCenter(u.hidden), u.search))
	u.root = u.frame(container.NewBorder(u.top, u.bottom, nil, nil, u.split(u.tabs)))
	u.reload()
	return u
}

// frame is the window's floor: the paper, and a size it never shrinks under.
func (u *ui) frame(content fyne.CanvasObject) fyne.CanvasObject {
	floor := canvas.NewRectangle(color.Transparent)
	floor.SetMinSize(fyne.NewSize(800, 520))
	return container.NewStack(floor, content)
}

// split is the list beside the pane; every screen with a list gets the same pair. The
// pane sits on card, one step up from the paper, so the ladder reads as its own place.
func (u *ui) split(lists fyne.CanvasObject) fyne.CanvasObject {
	pane := container.NewStack(rounded(style.Card, 0), container.NewVScroll(container.NewPadded(u.details.box)))
	sp := container.NewHSplit(lists, pane)
	sp.Offset = 0.62
	return sp
}

func (u *ui) tabChanged(*container.TabItem) {
	u.focusList()
	u.details.show(u.current().cursorTask())
}

// screen is whichever set of lists the mode shows; a mode with no list stands on home.
func (u *ui) screen() *screen {
	switch u.mode {
	case modeReview:
		return u.rv
	case modeSweep:
		return u.sw
	}
	return u.home
}

func (u *ui) current() *taskList {
	sc := u.screen()
	return sc.lists[sc.tabs.SelectedIndex()]
}

// switchLayer cycles at either end (28a): with two tabs forward and back are one move.
func (u *ui) switchLayer() {
	sc := u.screen()
	sc.tabs.SelectIndex((sc.tabs.SelectedIndex() + 1) % len(sc.lists))
}

func (u *ui) focusList() {
	if u.page != nil && (u.mode == modeSyncReview || u.mode == modePlugins) {
		u.win.Canvas().Focus(u.page)
		return
	}
	u.win.Canvas().Focus(u.current())
}

// reload reads the shown lists again, and home's when another screen is up, so the
// counts in the status line never lag what a mode changed.
func (u *ui) reload() {
	if err := u.screen().load(u); err != nil {
		u.fail(err)
		return
	}
	if u.mode != modeHome {
		if err := u.home.load(u); err != nil {
			u.fail(err)
			return
		}
	}
	u.refilter()
	u.status.SetText(u.statusText())
	u.flash = ""
	u.details.show(u.current().cursorTask())
}

// loadHome is the daily screen: this month's garage oldest first, the tray by urgency.
func loadHome(u *ui) error {
	today := store.Today()
	garage, err := u.s.Tasks(store.Filter{Layer: core.LayerGarage, Month: store.ThisMonth()})
	if err != nil {
		return err
	}
	tray, err := u.s.Tasks(store.Filter{Layer: core.LayerTray})
	if err != nil {
		return err
	}
	sort.SliceStable(tray, func(i, j int) bool {
		return core.Urgency(tray[i], today) > core.Urgency(tray[j], today)
	})
	u.garage.load(garage)
	u.tray.load(tray)
	return nil
}

// refilter narrows every shown list and says what the filter hid, so a list quietly
// showing 3 of 17 rows is never misread (57a).
func (u *ui) refilter() {
	for _, l := range u.screen().lists {
		l.apply(u.filter)
	}
	hid := len(u.current().all) - len(u.current().rows)
	if u.filter == "" || hid == 0 {
		u.hidden.Text = ""
	} else {
		u.hidden.Text = fmt.Sprintf("%d hidden", hid)
	}
	u.hidden.Refresh()
	u.top.Refresh() // the label's width changed; the search field takes up the rest
}

func (u *ui) clearFilter() {
	u.filter = ""
	u.search.SetText("")
	u.refilter()
}

func (u *ui) statusText() string {
	all, err := u.s.Tasks(store.Filter{All: true})
	if err != nil {
		return err.Error()
	}
	today := store.Today()
	live, waiting, templates := 0, 0, 0
	for _, t := range all {
		switch {
		case t.Layer == core.LayerGarage && t.Waiting(today):
			waiting++
		case t.Recur != "":
			if until, ended := core.Date(t.Until); !(ended && until.Before(today)) {
				templates++
			}
		case t.Layer == core.LayerTray && t.Live():
			live++
		}
	}
	text := fmt.Sprintf("tray %d · waiting %d · templates %d · garage %s", live, waiting, templates, store.ThisMonth())
	switch {
	case u.syncing:
		text += " · syncing…"
	case u.syncedAt != "":
		text += " · synced " + u.syncedAt
	}
	if u.flash != "" {
		text += " · " + u.flash
	}
	return text
}

func (u *ui) fail(err error) { u.say("error: " + err.Error()) }

// say puts one line about what just happened in the status, until the next reload.
func (u *ui) say(msg string) {
	u.flash = msg
	u.status.SetText(u.statusText())
}

// targets is what an action applies to: the marks, or the row under the cursor.
// Marks are kept by id across the whole layer, so a filter hides a row without
// unmarking it (57b).
func (u *ui) targets(l *taskList) []core.Task {
	var out []core.Task
	for _, t := range l.all {
		if u.marks[t.ID] {
			out = append(out, t)
		}
	}
	if len(out) == 0 {
		if t, ok := l.cursorTask(); ok {
			out = []core.Task{t}
		}
	}
	return out
}

func (u *ui) setMark(id int64, on bool) {
	if on {
		u.marks[id] = true
	} else {
		delete(u.marks, id)
	}
}

// offers is the mode's keymap (T11): review keeps the two rare verbs, the sweep the two
// that move a line, home everything but those two. A letter a mode does not offer is
// dead there, so a key pressed in the wrong room does nothing rather than something.
func (u *ui) offers(verb string) bool {
	const always = " l/?qc"
	switch u.mode {
	case modeHome:
		return !strings.Contains("RE", verb)
	case modeReview:
		return strings.Contains(always+"REv", verb)
	case modeSweep:
		return strings.Contains(always+"t>", verb)
	}
	return false
}

// do is the one dispatcher, so a letter, a row button and a menu can never disagree
// about what a verb does (24).
func (u *ui) do(verb string, l *taskList) {
	if !u.offers(verb) {
		return
	}
	picked := u.targets(l)
	switch verb {
	case " ":
		if t, ok := l.cursorTask(); ok {
			u.setMark(t.ID, !u.marks[t.ID])
			l.Refresh()
		}
	case "t":
		if l.layer == core.LayerGarage && len(picked) > 0 {
			u.openForm(picked, true)
		}
	case "x":
		today := store.Today()
		for i := range picked {
			core.Finish(&picked[i], today)
		}
		u.save(picked)
	case "d":
		if l.layer != core.LayerTray {
			return
		}
		for i := range picked {
			// Home is the month it left, else this one; what it learned stays (88a).
			month := picked[i].FromMonth
			if month == "" {
				month = store.ThisMonth()
			}
			core.Move(&picked[i], core.LayerGarage, month)
		}
		u.save(picked)
	case ">":
		if len(picked) > 0 {
			u.openMove(picked)
		}
	case "r":
		if len(picked) > 0 {
			u.openForm(picked, false)
		}
	case "#":
		if len(picked) > 0 {
			u.openTags(picked)
		}
	case "n":
		if len(picked) > 0 {
			u.openNote(picked)
		}
	case "a":
		if l.layer == core.LayerGarage {
			u.win.Canvas().Focus(u.capture)
		} else {
			u.openForm(nil, false)
		}
	case "R":
		var out []core.Task
		for _, t := range picked {
			if t.Done != "" {
				core.Restore(&t)
				out = append(out, t)
			}
		}
		if len(out) > 0 {
			u.save(out)
		}
	case "E":
		u.erase(picked)
	case "v":
		if u.mode == modeReview {
			u.leave()
		} else {
			u.openReview()
		}
	case "s":
		u.syncNow()
	case "p":
		u.openPlugins()
	case "c":
		u.copyContext(picked)
	case "l":
		u.details.focus()
	case "/":
		u.win.Canvas().Focus(u.search)
	case "?":
		u.openHelp()
	case "q":
		u.quit()
	}
}

// save lands every task in one transaction, then reads the world back. Marks are
// spent by the action they were for.
func (u *ui) save(tasks []core.Task) {
	err := u.s.Update(func(tx *store.Store) error {
		for i := range tasks {
			if err := tx.Put(&tasks[i]); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		u.fail(err)
		return
	}
	if u.flashNew && len(tasks) > 0 {
		u.flashID = tasks[0].ID
	}
	u.flashNew = false
	u.marks = map[int64]bool{}
	u.reload()
}

// erase removes rows outright, the one verb that does (91), with no prompt (91c): the
// status names what went, which is all the recovery a prompt ever bought.
func (u *ui) erase(tasks []core.Task) {
	if len(tasks) == 0 {
		return
	}
	var names []string
	err := u.s.Update(func(tx *store.Store) error {
		for _, t := range tasks {
			if err := tx.Delete(t.ID); err != nil {
				return err
			}
			names = append(names, t.Text)
		}
		return nil
	})
	if err != nil {
		u.fail(err)
		return
	}
	u.marks = map[int64]bool{}
	u.flash = "erased: " + strings.Join(names, " · ")
	u.reload()
}

// copyContext hands an agent what `tray context` prints: one shape, never a second one
// for the screen.
func (u *ui) copyContext(tasks []core.Task) {
	if len(tasks) == 0 {
		return
	}
	u.win.Clipboard().SetContent(cli.ContextText(tasks, store.Today()))
	u.say(fmt.Sprintf("copied %d", len(tasks)))
}

// dump is the capture bar's verb, with the CLI's reading of the front of the line.
func (u *ui) dump(text string) {
	tail := strings.Fields(text)
	month, tags := store.ThisMonth(), []string{}
	for len(tail) > 0 {
		if rest, ok := strings.CutPrefix(tail[0], "to:"); ok && rest != "" {
			month = rest
			tail = tail[1:]
			continue
		}
		if mods := core.SplitMods([]string{tail[0]}); len(mods.AddTags) == 1 {
			tags = append(tags, mods.AddTags[0])
			tail = tail[1:]
			continue
		}
		break
	}
	if len(tail) == 0 {
		return
	}
	t := core.New(strings.Join(tail, " "), tags)
	t.Month = month
	u.flashNew = true
	u.save([]core.Task{t})
}

// tagHint is the vocabulary in use, offered rather than picked from (30).
func (u *ui) tagHint() string {
	all, err := u.s.Tasks(store.Filter{All: true})
	if err != nil {
		return ""
	}
	seen := map[string]bool{}
	var tags []string
	for _, t := range all {
		for _, g := range t.Tags {
			if !seen[g] {
				seen[g] = true
				tags = append(tags, g)
			}
		}
	}
	if len(tags) == 0 {
		return "tags, space separated"
	}
	sort.Strings(tags)
	return "in use: " + strings.Join(tags, " ")
}

// show puts content over the screen and hands it the keyboard. One popup at a time: a
// form over a form is a question you cannot see.
func (u *ui) show(content fyne.CanvasObject, focus fyne.Focusable) {
	u.hide()
	u.pop = widget.NewModalPopUp(container.NewPadded(content), u.win.Canvas())
	u.pop.Show()
	if focus != nil {
		u.win.Canvas().Focus(focus)
	}
}

func (u *ui) hide() {
	if u.pop != nil {
		u.pop.Hide()
		u.pop = nil
	}
	u.focusList()
}

func (u *ui) quit() {
	u.closed = true
	u.win.Close()
}
