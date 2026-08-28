// Package ui is the terminal interface. It is a client of core and store, and
// never parses a line or writes a file itself.
package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
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
	{key: "R", label: "restore", tray: true, rest: true, apply: (*Model).restore},
}

type Model struct {
	layers []layer
	active int
	list   list.Model
	deleg  *rowDelegate
	help   help.Model
	marked map[string]bool // by text, so it survives a reload and a filter

	mode   mode
	menuAt int
	dests  []layer
	destAt int
	form   *form

	status   string
	today    time.Time
	err      error
	showDone bool   // v: the finished rows are hidden until you ask for them
	sweep    bool   // the month-turn ritual: months get the tabs, not the tray
	closing  string // which month is being swept
	width    int    // 0 until the terminal tells us, so the view must cope without
	height   int
}

func New() Model {
	return start(Model{marked: map[string]bool{}, today: store.Today()})
}

// NewSweep is `tray carryover`: the closing month, this one, and someday.
func NewSweep(closing string) Model {
	return start(Model{
		marked: map[string]bool{}, today: store.Today(), sweep: true, closing: closing,
	})
}

// start wires bubbles/list down to the part we want: scrolling, paging and the
// fuzzy filter behind `/`. Everything it would otherwise draw is switched off,
// because the frame, the tabs, the column header and the footer are ours.
func start(m Model) Model {
	m.deleg = &rowDelegate{marked: m.marked, today: m.today}
	m.list = list.New(nil, m.deleg, 0, 0)
	m.list.SetShowTitle(false)
	m.list.SetShowStatusBar(false)
	m.list.SetShowPagination(false)
	m.list.SetShowHelp(false)
	m.list.SetShowFilter(false) // drawn by renderFilter, so the frame owns the line
	m.list.SetFilteringEnabled(true)
	m.list.DisableQuitKeybindings() // q and esc mean tray things first
	m.list.FilterInput.Prompt = ""

	// Four ways to move a cursor is three too many. Paging follows the cursor on its
	// own, so ↑↓ (and j k) are the whole of it.
	m.list.KeyMap.PrevPage = key.NewBinding()
	m.list.KeyMap.NextPage = key.NewBinding()
	m.list.KeyMap.GoToStart = key.NewBinding()
	m.list.KeyMap.GoToEnd = key.NewBinding()
	// `?` is ours. list's own help would advertise list's keymap, not tray's.
	m.list.KeyMap.ShowFullHelp = key.NewBinding()
	m.list.KeyMap.CloseFullHelp = key.NewBinding()

	m.help = help.New()
	m.help.ShortSeparator = " · "
	m.help.FullSeparator = "   "

	m.reload() // no filter can be set yet, so there is no command to run
	if m.sweep {
		m.active = sweepStart(m.layers)
		m.reload()
	}
	return m
}

// items is what is on screen: filtered, in the order the layer sorts them.
func (m Model) items() []core.Task {
	visible := m.list.VisibleItems()
	out := make([]core.Task, 0, len(visible))
	for _, item := range visible {
		if r, ok := item.(row); ok {
			out = append(out, r.Task)
		}
	}
	return out
}

func (m Model) filtering() bool { return m.list.FilterState() != list.Unfiltered }

func (m Model) layer() layer {
	if m.active < len(m.layers) {
		return m.layers[m.active]
	}
	return layer{title: "tray"}
}

// reload re-reads the layer from disk. It returns a command because a filter that
// is still applied has to be re-run against the new rows, and that is async.
func (m *Model) reload() tea.Cmd {
	m.layers = layers(m.sweep, m.closing)
	if m.active >= len(m.layers) {
		m.active = 0
	}
	doc, err := m.layer().open()
	if err != nil {
		m.err = err
		return nil
	}
	var live []core.Task
	for _, t := range doc.Tasks() {
		if t.Parsed() && (t.Live() || (m.showDone && t.Terminal())) {
			live = append(live, t)
		}
	}
	if m.layer().isTray() {
		sortByUrgency(live, m.today)
	}
	items := make([]list.Item, len(live))
	for i, t := range live {
		items[i] = row{t}
	}
	at := m.list.Index()
	cmd := m.list.SetItems(items)
	m.list.Select(min(at, max(0, len(items)-1)))
	m.resize()
	return cmd
}

// resize keeps the list inside the frame. The filter line and the help overlay both
// take rows away from it, so this runs whenever either could have changed — an
// overflowing pane is what makes the alt screen jitter.
func (m *Model) resize() {
	width := m.width - pane.GetHorizontalFrameSize()
	if width < 1 {
		width = 60 // before the terminal has said anything, pick a sane table width
	}
	height := m.rowRoom()
	if height < 1 {
		height = max(1, len(m.list.Items()))
	}
	m.list.SetSize(width, height)
	m.deleg.tray = m.layer().isTray()
	m.deleg.today = m.today
	m.deleg.measure(m.list.VisibleItems(), width)
}

func sortByUrgency(items []core.Task, today time.Time) {
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && core.Urgency(items[j], today) > core.Urgency(items[j-1], today); j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}
}

// picked is what an action applies to: everything marked, or the row under the
// cursor. A mark on a row a filter is currently hiding still counts — hiding a row
// is not the same as deselecting it.
func (m *Model) picked() []core.Task {
	var out []core.Task
	for _, item := range m.list.Items() {
		r, ok := item.(row)
		if ok && m.marked[r.Text] {
			out = append(out, r.Task)
		}
	}
	if len(out) == 0 {
		if r, ok := m.list.SelectedItem().(row); ok {
			out = append(out, r.Task)
		}
	}
	return out
}

// The first row is what enter-enter does, so each layer leads with its own primary
// action: restructure what you're working on, take what you jotted.
var order = map[bool][]string{
	true:  {"r", "x", "d", ">", "D"}, // the tray
	false: {"t", "r", ">", "x", "D"}, // a garage month
}

// A finished row is a record. The only sane thing to say about one is that it was not
// finished after all, so that is the only thing offered — `x` on a done task or `d`
// handing a done task back are either meaningless or quietly destructive.
func (m Model) offered() []action {
	if m.allFinished() {
		for _, a := range actions {
			if a.key == "R" {
				return []action{a}
			}
		}
	}
	byKey := map[string]action{}
	for _, a := range actions {
		if a.key == "R" {
			continue // reachable only on a finished row, above
		}
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

// allFinished, not anyFinished: a mixed selection keeps the normal menu, because
// there is no single sensible verb across both kinds.
func (m *Model) allFinished() bool {
	picked := m.picked()
	if len(picked) == 0 {
		return false
	}
	for _, t := range picked {
		if !t.Terminal() {
			return false
		}
	}
	return true
}

func (m *Model) restore(picked []core.Task) string {
	doc, err := m.layer().open()
	if err != nil {
		return err.Error()
	}
	n := 0
	for _, t := range picked {
		if !t.Terminal() {
			continue
		}
		core.Restore(&t)
		doc.Set(t)
		n++
	}
	if err := doc.Save(); err != nil {
		return err.Error()
	}
	return plural(n, "restored")
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.resize()
		return m, nil
	case tea.KeyMsg:
		switch m.mode {
		case editing:
			return m.updateForm(msg)
		case acting:
			return m.updateMenu(msg)
		case sending:
			return m.updateDestinations(msg)
		default:
			return m.updateList(msg)
		}
	}
	// Filter matches arrive as a message of their own, so everything else goes
	// to the list rather than being dropped on the floor.
	return m.toList(msg)
}

// toList hands a message to bubbles/list and re-measures, because anything the list
// acts on can change which rows are visible and therefore how wide the columns are.
func (m Model) toList(msg tea.Msg) (tea.Model, tea.Cmd) {
	l, cmd := m.list.Update(msg)
	m.list = l
	m.resize()
	return m, cmd
}

// Tabs cycle: there are two or three of them, so stopping at the end just makes you
// reach for the other key.
func (m *Model) switchTab(by int) tea.Cmd {
	if len(m.layers) == 0 {
		return nil
	}
	m.active = (m.active + by + len(m.layers)) % len(m.layers)
	m.list.ResetFilter() // a filter is about the rows you were looking at
	m.list.Select(0)
	clear(m.marked) // marks belong to a layer, not to the session
	m.status = ""
	return m.reload()
}

func (m Model) updateList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// With the filter box focused every keystroke is text, `q` and `j` included.
	if m.list.SettingFilter() {
		return m.toList(msg)
	}
	switch msg.String() {
	case "ctrl+c", "q":
		return m, tea.Quit
	case "esc":
		if m.help.ShowAll {
			m.help.ShowAll = false
			m.resize()
			return m, nil
		}
		if m.filtering() {
			return m.toList(msg) // esc drops the filter before it quits tray
		}
		return m, tea.Quit
	case "?":
		m.help.ShowAll = !m.help.ShowAll
		m.resize()
	case "v":
		m.showDone = !m.showDone
		return m, m.reload()
	case "tab", "l", "right":
		return m, m.switchTab(1)
	case "shift+tab", "h", "left":
		return m, m.switchTab(-1)
	case "j", "down", "k", "up", "/":
		return m.toList(msg)
	case " ":
		if r, ok := m.list.SelectedItem().(row); ok {
			if m.marked[r.Text] {
				delete(m.marked, r.Text)
			} else {
				m.marked[r.Text] = true
			}
		}
	case "a", "n":
		f := newEntry(m.layer().month, m.today)
		m.form, m.mode = &f, editing
		m.resize()
	case "enter":
		if len(m.list.VisibleItems()) > 0 {
			m.mode, m.menuAt = acting, 0
			m.resize()
		}
	default:
		if act, found := m.lookup(msg.String()); found {
			return m, m.run(act)
		}
	}
	return m, nil
}

func (m Model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	offered := m.offered()
	switch msg.String() {
	case "esc", "q":
		m.mode = browsing
		m.resize()
	case "j", "down":
		if m.menuAt < len(offered)-1 {
			m.menuAt++
		}
	case "k", "up":
		if m.menuAt > 0 {
			m.menuAt--
		}
	case "enter":
		return m, m.run(offered[m.menuAt])
	default:
		if act, found := m.lookup(msg.String()); found {
			return m, m.run(act)
		}
	}
	return m, nil
}

func (m Model) updateDestinations(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.mode = browsing
		m.resize()
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
		clear(m.marked)
		return m, m.reload()
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
	clear(m.marked)
	return m, m.reload()
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
func (m *Model) run(a action) tea.Cmd {
	picked := m.picked()
	if len(picked) == 0 {
		return nil
	}
	m.status = a.apply(m, picked)
	if m.mode == editing || m.mode == sending {
		m.resize()
		return nil // those modes own the selection until they are done
	}
	m.mode = browsing
	clear(m.marked)
	return m.reload()
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
