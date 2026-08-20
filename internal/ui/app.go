// Package ui is the terminal interface. It is a client of core and store, and
// never parses a line or writes a file itself.
package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
)

// mode is what the keyboard is talking to.
type mode int

const (
	browsing mode = iota
	acting        // the enter menu
	sending       // choosing where `>` moves things
	editing       // the retake form
)

// action is one row of the enter menu. Its key also works straight from the list.
type action struct {
	key   string
	label string
	tray  bool // offered on the tray
	rest  bool // offered on a garage month
	apply func(*Model, []core.Task) string
}

var actions = []action{
	{key: "t", label: "take", rest: true, apply: (*Model).take},
	{key: "x", label: "done", tray: true, rest: true,
		apply: func(m *Model, p []core.Task) string { return m.finish(p, "done") }},
	{key: "d", label: "hand back", tray: true, apply: (*Model).handBack},
	{key: ">", label: "move to", tray: true, rest: true, apply: (*Model).chooseDestination},
	{key: "r", label: "retake", tray: true, rest: true, apply: (*Model).openForm},
	{key: "D", label: "delete", tray: true, rest: true,
		apply: func(m *Model, p []core.Task) string { return m.finish(p, "dropped") }},
}

type Model struct {
	layers []layer
	active int
	items  []core.Task
	cursor int
	marked map[string]bool // by text, so it survives a reload

	mode   mode
	menuAt int
	dests  []layer
	destAt int
	form   *form

	status  string
	today   time.Time
	err     error
	sweep   bool   // the month-turn ritual: months get the tabs, not the tray
	closing string // which month is being swept
	width   int    // 0 until the terminal tells us, so the view must cope without
	height  int
}

func New() Model {
	m := Model{marked: map[string]bool{}, today: store.Today()}
	m.reload()
	return m
}

// NewSweep is `tray carryover`: the closing month, this one, and someday.
func NewSweep(closing string) Model {
	m := Model{marked: map[string]bool{}, today: store.Today(), sweep: true, closing: closing}
	m.reload()
	return m
}

func (m Model) layer() layer {
	if m.active < len(m.layers) {
		return m.layers[m.active]
	}
	return layer{title: "tray"}
}

func (m *Model) reload() {
	m.layers = layers(m.sweep, m.closing)
	if m.active >= len(m.layers) {
		m.active = 0
	}
	doc, err := m.layer().open()
	if err != nil {
		m.err = err
		return
	}
	var live []core.Task
	for _, t := range doc.Tasks() {
		if t.Live() && t.Parsed() {
			live = append(live, t)
		}
	}
	if m.layer().isTray() {
		sortByUrgency(live, m.today)
	}
	m.items = live
	if m.cursor >= len(m.items) {
		m.cursor = max(0, len(m.items)-1)
	}
}

func sortByUrgency(items []core.Task, today time.Time) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && core.Urgency(items[j], today) > core.Urgency(items[j-1], today); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

// picked is what an action applies to: everything marked, or the row under the cursor.
func (m *Model) picked() []core.Task {
	var out []core.Task
	for _, t := range m.items {
		if m.marked[t.Text] {
			out = append(out, t)
		}
	}
	if len(out) == 0 && m.cursor < len(m.items) {
		out = append(out, m.items[m.cursor])
	}
	return out
}

// The first row is what enter-enter does, so each layer leads with its own primary
// action: restructure what you're working on, take what you jotted.
var order = map[bool][]string{
	true:  {"r", "x", "d", ">", "D"}, // the tray
	false: {"t", "r", ">", "x", "D"}, // a garage month
}

func (m Model) offered() []action {
	byKey := map[string]action{}
	for _, a := range actions {
		if (m.layer().isTray() && a.tray) || (!m.layer().isTray() && a.rest) {
			byKey[a.key] = a
		}
	}
	var out []action
	for _, key := range order[m.layer().isTray()] {
		if a, ok := byKey[key]; ok {
			out = append(out, a)
		}
	}
	return out
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if size, ok := msg.(tea.WindowSizeMsg); ok {
		m.width, m.height = size.Width, size.Height
		return m, nil
	}
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch m.mode {
	case editing:
		return m.updateForm(key)
	case acting:
		return m.updateMenu(key)
	case sending:
		return m.updateDestinations(key)
	default:
		return m.updateList(key)
	}
}

// Tabs cycle: there are two or three of them, so stopping at the end just makes you
// reach for the other key.
func (m *Model) switchTab(by int) {
	if len(m.layers) == 0 {
		return
	}
	m.active = (m.active + by + len(m.layers)) % len(m.layers)
	m.cursor = 0
	m.marked = map[string]bool{} // marks belong to a layer, not to the session
	m.status = ""
	m.reload()
}

func (m Model) updateList(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "q", "esc", "ctrl+c":
		return m, tea.Quit
	case "tab", "l", "right":
		m.switchTab(1)
	case "shift+tab", "h", "left":
		m.switchTab(-1)
	case "j", "down":
		if m.cursor < len(m.items)-1 {
			m.cursor++
		}
	case "k", "up":
		if m.cursor > 0 {
			m.cursor--
		}
	case "g":
		m.cursor = 0
	case "G":
		m.cursor = max(0, len(m.items)-1)
	case " ":
		if m.cursor < len(m.items) {
			text := m.items[m.cursor].Text
			if m.marked[text] {
				delete(m.marked, text)
			} else {
				m.marked[text] = true
			}
		}
	case "a", "n":
		f := newEntry(m.layer().month, m.today)
		m.form, m.mode = &f, editing
	case "enter":
		if len(m.items) > 0 {
			m.mode, m.menuAt = acting, 0
		}
	default:
		if act, found := m.lookup(key.String()); found {
			m.run(act)
		}
	}
	return m, nil
}

func (m Model) updateMenu(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	offered := m.offered()
	switch key.String() {
	case "esc", "q":
		m.mode = browsing
	case "j", "down":
		if m.menuAt < len(offered)-1 {
			m.menuAt++
		}
	case "k", "up":
		if m.menuAt > 0 {
			m.menuAt--
		}
	case "enter":
		m.run(offered[m.menuAt])
	default:
		if act, found := m.lookup(key.String()); found {
			m.run(act)
		}
	}
	return m, nil
}

func (m Model) updateDestinations(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch key.String() {
	case "esc", "q":
		m.mode = browsing
	case "j", "down":
		if m.destAt < len(m.dests)-1 {
			m.destAt++
		}
	case "k", "up":
		if m.destAt > 0 {
			m.destAt--
		}
	case "enter":
		m.status = m.move(m.picked(), m.dests[m.destAt])
		m.mode = browsing
		m.marked = map[string]bool{}
		m.reload()
	}
	return m, nil
}

func (m Model) updateForm(key tea.KeyMsg) (tea.Model, tea.Cmd) {
	next, save, done := m.form.update(key)
	m.form = &next
	if !done {
		return m, nil
	}
	if save {
		status, err := next.apply()
		if err != nil {
			m.status = err.Error()
		} else {
			m.status = status
		}
	}
	m.form, m.mode = nil, browsing
	m.marked = map[string]bool{}
	m.reload()
	return m, nil
}

func (m Model) lookup(key string) (action, bool) {
	for _, a := range m.offered() {
		if a.key == key {
			return a, true
		}
	}
	return action{}, false
}

// run applies an action, then re-reads from disk so ids and urgency stay honest.
func (m *Model) run(a action) {
	picked := m.picked()
	if len(picked) == 0 {
		return
	}
	m.status = a.apply(m, picked)
	if m.mode == editing || m.mode == sending {
		return // those modes own the selection until they are done
	}
	m.mode = browsing
	m.marked = map[string]bool{}
	m.reload()
}

func (m *Model) chooseDestination(picked []core.Task) string {
	m.dests, m.destAt, m.mode = m.destinations(), 0, sending
	return ""
}

// openForm hands the selection to the retake form, on whichever layer it came from.
func (m *Model) openForm(picked []core.Task) string {
	f := newForm(picked, m.layer().month, m.today)
	m.form, m.mode = &f, editing
	return ""
}

// take is a move onto the tray followed by the form — structure paid for once, at
// the moment something graduates.
func (m *Model) take(picked []core.Task) string {
	tray := layer{title: "tray"}
	status := m.move(picked, tray)

	doc, err := tray.open()
	if err != nil {
		return status
	}
	var landed []core.Task
	wanted := map[string]bool{}
	for _, t := range picked {
		wanted[t.Text] = true
	}
	for _, t := range doc.Tasks() {
		if wanted[t.Text] && t.Live() {
			landed = append(landed, t)
		}
	}
	if len(landed) == 0 {
		return status
	}
	f := newForm(landed, "", m.today)
	m.form, m.mode = &f, editing
	return status
}

func (m *Model) finish(picked []core.Task, as string) string {
	doc, err := m.layer().open()
	if err != nil {
		return err.Error()
	}
	for _, t := range picked {
		core.Finish(&t, as, m.today)
		doc.Set(t)
	}
	if err := doc.Save(); err != nil {
		return err.Error()
	}
	return plural(len(picked), as)
}

func (m *Model) handBack(picked []core.Task) string {
	return m.move(picked, layer{title: monthTitle(store.ThisMonth()), month: store.ThisMonth()})
}

func plural(n int, what string) string {
	return fmt.Sprintf("%d %s", n, what)
}

// Run takes over the terminal until the user quits.
func Run() error { return run(New()) }

// RunSweep opens the same interface with the month tabs the ritual needs.
func RunSweep(closing string) error { return run(NewSweep(closing)) }

func run(m Model) error {
	_, err := tea.NewProgram(m, tea.WithAltScreen()).Run()
	return err
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
