package gui

import (
	"fmt"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
	"github.com/cheese-cracker/tray/internal/style"
)

// openSweep is the month turn as a screen (73): the months it is about, chronological,
// opened on this one (73a), with the two verbs the job is made of and, under each real
// month, a button that carries that month forward. The month is on the button, never
// inferred (70).
func (u *ui) openSweep() {
	this := store.ThisMonth()
	months := []string{store.PrevMonth(this), this, store.NextMonth(this), store.Someday}
	var lists []*taskList
	var items []*container.TabItem
	for _, m := range months {
		m := m
		l := newTaskList(u, core.LayerGarage, sweepVerbs)
		l.month = m
		lists = append(lists, l)
		content := l.view()
		if store.IsMonth(m) {
			carry := newLink("carry forward "+m+" → "+store.NextMonth(m), func() { u.carry(m) })
			content = container.NewBorder(nil, container.NewPadded(container.NewHBox(carry)), nil, nil, content)
		}
		items = append(items, container.NewTabItem(m, content))
	}
	tabs := newTabs(items...)
	tabs.OnSelected = u.tabChanged
	tabs.SelectIndex(1)
	u.sw = &screen{tabs: tabs, lists: lists, load: loadSweep}

	root := container.NewBorder(
		container.NewVBox(banner("sweep", "> move to a month · t take · esc leaves", style.AccentSoft, style.Accent), u.top),
		u.bottom, nil, nil, u.split(tabs))
	u.enter(modeSweep, root)
}

func loadSweep(u *ui) error {
	for _, l := range u.sw.lists {
		rows, err := u.s.Tasks(store.Filter{Layer: core.LayerGarage, Month: l.month})
		if err != nil {
			return err
		}
		l.load(rows)
	}
	return nil
}

// carry is `tray carryover --run --month` from the sweep: the live rows of the named
// month move to the next, and a due date already passed does not come along — carrying
// a line forward is admitting the date did not hold (75).
func (u *ui) carry(source string) {
	rows, err := u.s.Tasks(store.Filter{Layer: core.LayerGarage, Month: source})
	if err != nil {
		u.fail(err)
		return
	}
	target := store.NextMonth(source)
	today := store.Today()
	err = u.s.Update(func(tx *store.Store) error {
		for _, t := range rows {
			if due, ok := core.Date(t.Due); ok && due.Before(today) {
				t.Due = ""
			}
			core.Move(&t, core.LayerGarage, target)
			if err := tx.Put(&t); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		u.fail(err)
		return
	}
	u.flash = fmt.Sprintf("%d %s → %s", len(rows), source, target)
	u.reload()
}

// destinations is where > can send a line: every tab on screen (73d). The daily screen
// has two, so there it is this month, the next and someday — the whole of how those
// are reached (73e).
func (u *ui) destinations() []string {
	if u.mode == modeSweep {
		var out []string
		for _, l := range u.sw.lists {
			out = append(out, l.month)
		}
		return out
	}
	this := store.ThisMonth()
	return []string{this, store.NextMonth(this), store.Someday}
}

// openUnload hands the whole tray back. Emptying the tray is the largest single action
// here, so where it lands is never guessed (72): a picker that says how many rows will
// move, and moves them only on a choice. Templates are the tray's furniture and stay.
func (u *ui) openUnload() {
	rows, err := u.s.Tasks(store.Filter{Layer: core.LayerTray})
	if err != nil {
		u.fail(err)
		return
	}
	home := 0
	for _, t := range rows {
		if t.FromMonth != "" {
			home++
		}
	}
	unload := func(month string) {
		moved, kept := 0, 0
		err := u.s.Update(func(tx *store.Store) error {
			for _, t := range rows {
				if month == "" && t.FromMonth == "" {
					kept++ // never came from a month, so there is no home to send it to
					continue
				}
				core.Move(&t, core.LayerGarage, month)
				if err := tx.Put(&t); err != nil {
					return err
				}
				moved++
			}
			return nil
		})
		if err != nil {
			u.fail(err)
			return
		}
		u.hide()
		where := month
		if where == "" {
			where = "home"
		}
		u.flash = fmt.Sprintf("%d to %s", moved, where)
		if kept > 0 {
			u.flash += fmt.Sprintf(" · %d never came from a month, kept", kept)
		}
		u.marks = map[int64]bool{}
		u.reload()
	}

	this := store.ThisMonth()
	dests := []struct{ label, month string }{
		{"garage · " + this, this},
		{"garage · " + store.NextMonth(this), store.NextMonth(this)},
		{store.Someday, store.Someday},
		{fmt.Sprintf("where they came from (%d have a home)", home), ""},
	}
	title := semibold(fmt.Sprintf("hand the tray back — %d rows will move", len(rows)), style.Ink)
	box := container.NewVBox(title)
	for i, d := range dests {
		d := d
		box.Add(lowButton(string(rune('1'+i))+"  "+d.label, func() { unload(d.month) }))
	}
	typed := newEscEntry(u.hide)
	typed.SetPlaceHolder("or a month, 2026-11")
	typed.OnSubmitted = func(s string) {
		if s = strings.TrimSpace(s); store.IsMonth(s) {
			unload(s)
		}
	}
	box.Add(typed)
	p := newPage(box)
	p.onKey = func(k *fyne.KeyEvent) {
		if k.Name == fyne.KeyEscape {
			u.hide()
		}
	}
	p.onRune = func(r rune) {
		if i := int(r - '1'); i >= 0 && i < len(dests) {
			unload(dests[i].month)
		}
	}
	u.show(p, p)
}
