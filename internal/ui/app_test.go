package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cheese-cracker/tray/internal/store"
)

// Behaviour is asserted on model state and on what landed in the file. Frames are
// only checked for structure, so restyling doesn't break the suite.
func sandbox(t *testing.T, lines ...string) {
	t.Helper()
	t.Setenv("TRAY_HOME", t.TempDir())
	t.Setenv("TRAY_TODAY", "2026-08-07")
	doc, err := store.Tray()
	if err != nil {
		t.Fatal(err)
	}
	doc.Lines = append([]string{store.TrayHeader, ""}, lines...)
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}
}

func trayFile(t *testing.T) string {
	t.Helper()
	lines, err := store.Read(store.TrayPath())
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(lines, "\n")
}

func garageFile(t *testing.T) string {
	t.Helper()
	lines, err := store.Read(store.MonthPath(""))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(lines, "\n")
}

func keys(m tea.Model, presses ...string) tea.Model {
	for _, k := range presses {
		m, _ = m.Update(keyMsg(k))
	}
	return m
}

func keyMsg(k string) tea.KeyMsg {
	switch k {
	case " ":
		return tea.KeyMsg{Type: tea.KeySpace}
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "backspace":
		return tea.KeyMsg{Type: tea.KeyBackspace}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "shift+tab":
		return tea.KeyMsg{Type: tea.KeyShiftTab}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
	}
}

func TestListOrdersByUrgency(t *testing.T) {
	sandbox(t,
		"- [ ] low thing priority:L",
		"- [ ] urgent thing priority:H due:2026-08-08",
		"- [ ] middle thing priority:M",
	)
	m := New()
	if len(m.items()) != 3 {
		t.Fatalf("got %d items", len(m.items()))
	}
	if m.items()[0].Text != "urgent thing" {
		t.Errorf("first row = %q, want the most urgent", m.items()[0].Text)
	}
}

func TestTerminalTasksAreNotListed(t *testing.T) {
	sandbox(t,
		"- [ ] open thing",
		"- [x] ~~done thing~~ done:2026-08-01",
		"- ~~dropped thing~~ dropped:2026-08-01",
	)
	if got := len(New().items()); got != 1 {
		t.Errorf("listed %d items, want only the live one", got)
	}
}

func TestCursorMoves(t *testing.T) {
	sandbox(t, "- [ ] one priority:H", "- [ ] two priority:M", "- [ ] three priority:L")
	m := keys(New(), "j", "j").(Model)
	if m.list.Index() != 2 {
		t.Errorf("cursor = %d, want 2", m.list.Index())
	}
	m = keys(m, "k").(Model)
	if m.list.Index() != 1 {
		t.Errorf("cursor = %d, want 1", m.list.Index())
	}
	// It must not run off either end.
	m = keys(m, "k", "k", "k").(Model)
	if m.list.Index() != 0 {
		t.Errorf("cursor = %d, want 0", m.list.Index())
	}
	// ↑↓ and h j k l are the whole of moving now: g, G, home, end and the page keys
	// were four more ways to do one thing.
	for _, dead := range []string{"g", "G", "home", "end", "pgup", "pgdown"} {
		if got := keys(m, dead).(Model).list.Index(); got != m.list.Index() {
			t.Errorf("%q should not move the cursor, went to %d", dead, got)
		}
	}
}

// tab is the only way across and ⇧tab the only way back. ←→ and h l are dead: ↑↓
// move within a layer and nothing here moves sideways.
func TestOnlyTabSwitchesLayers(t *testing.T) {
	sandbox(t, "- [ ] on the tray priority:H")
	garage(t, "2026-08", "- in the garage")

	m := New()
	for _, dead := range []string{"left", "right", "h", "l"} {
		if got := keys(m, dead).(Model).layer().title; got != "tray" {
			t.Errorf("%q should not switch layers, landed on %q", dead, got)
		}
	}
	if got := keys(m, "tab").(Model).layer().month; got != "2026-08" {
		t.Errorf("tab should switch, landed on %q", got)
	}
	if got := keys(m, "shift+tab").(Model).layer().month; got != "2026-08" {
		t.Errorf("⇧tab should switch back round, landed on %q", got)
	}
}

func TestSpaceMarksAndUnmarks(t *testing.T) {
	sandbox(t, "- [ ] one priority:H", "- [ ] two priority:M")
	m := keys(New(), " ").(Model)
	if len(m.marked) != 1 || !m.marked["one"] {
		t.Errorf("marked = %v", m.marked)
	}
	m = keys(m, " ").(Model)
	if len(m.marked) != 0 {
		t.Errorf("space should unmark: %v", m.marked)
	}
}

func TestDoneFromTheList(t *testing.T) {
	sandbox(t, "- [ ] one priority:H", "- [ ] two priority:M")
	m := keys(New(), "x").(Model) // acts on the cursor when nothing is marked

	if got := trayFile(t); !strings.Contains(got, "~~one~~") {
		t.Errorf("file did not record done:\n%s", got)
	}
	if len(m.items()) != 1 {
		t.Errorf("the finished task should leave the list, got %d", len(m.items()))
	}
}

func TestActionAppliesToEveryMark(t *testing.T) {
	sandbox(t, "- [ ] one priority:H", "- [ ] two priority:M", "- [ ] three priority:L")
	m := keys(New(), " ", "j", " ", "x").(Model) // mark two, finish both

	got := trayFile(t)
	if !strings.Contains(got, "~~one~~") || !strings.Contains(got, "~~two~~") {
		t.Errorf("both marks should be done:\n%s", got)
	}
	if strings.Contains(got, "~~three~~") {
		t.Error("an unmarked task was touched")
	}
	if len(m.marked) != 0 {
		t.Error("marks should clear after an action")
	}
}

func TestHandBackMovesToTheGarage(t *testing.T) {
	sandbox(t, "- [ ] one priority:H +infra")
	keys(New(), "d")

	if got := garageFile(t); !strings.Contains(got, "one priority:H") {
		t.Errorf("garage did not receive it:\n%s", got)
	}
	if got := trayFile(t); strings.Contains(got, "one") {
		t.Errorf("tray should have released it:\n%s", got)
	}
}

func TestDeleteIsAbandonedNotRemoved(t *testing.T) {
	sandbox(t, "- [ ] one priority:H")
	keys(New(), "D")

	got := trayFile(t)
	if !strings.Contains(got, "~~one~~") || !strings.Contains(got, "dropped:2026-08-07") {
		t.Errorf("want struck and dated as dropped:\n%s", got)
	}
	if strings.Contains(got, "done:") {
		t.Error("delete must not read as completed")
	}
}

func TestMenuOpensAndActs(t *testing.T) {
	sandbox(t, "- [ ] one priority:H")
	m := keys(New(), "enter").(Model)
	if m.mode != acting {
		t.Fatal("enter should open the menu")
	}
	if !strings.Contains(m.View(), "hand back") {
		t.Error("the menu should list its actions")
	}

	for m.offered()[m.menuAt].key != "d" { // walk to hand back, wherever it sits
		m = keys(m, "j").(Model)
	}
	m = keys(m, "enter").(Model)
	if m.mode != browsing {
		t.Error("the menu should close after acting")
	}
	if got := garageFile(t); !strings.Contains(got, "one") {
		t.Errorf("menu action did not apply:\n%s", got)
	}
}

func TestEscClosesTheMenuWithoutActing(t *testing.T) {
	sandbox(t, "- [ ] one priority:H")
	m := keys(New(), "enter", "esc").(Model)
	if m.mode != browsing {
		t.Error("esc should close the menu")
	}
	if got := trayFile(t); strings.Contains(got, "~~") {
		t.Errorf("nothing should have been applied:\n%s", got)
	}
}

func TestEscAndQBothQuit(t *testing.T) {
	sandbox(t, "- [ ] one")
	quits := map[string]tea.KeyMsg{
		"q":   {Type: tea.KeyRunes, Runes: []rune("q")},
		"esc": {Type: tea.KeyEsc},
	}
	for name, msg := range quits {
		if _, cmd := New().Update(msg); cmd == nil {
			t.Errorf("%s should quit", name)
		}
	}
}

func TestBothAAndNAdd(t *testing.T) {
	sandbox(t)
	for _, key := range []string{"a", "n"} {
		m := keys(New(), key).(Model)
		if m.form == nil || !m.form.creating {
			t.Errorf("%q should start a new entry", key)
		}
	}
}

func TestEmptyTraySaysSo(t *testing.T) {
	sandbox(t)
	view := New().View()
	if !strings.Contains(view, "nothing on the tray") {
		t.Errorf("view = %q", view)
	}
}

// The frame is checked for structure only — rows, marks, the keymap — so styling
// changes don't break this.
func TestFrameShowsRowsMarksAndKeymap(t *testing.T) {
	sandbox(t, "- [ ] one priority:H", "- [ ] two priority:M")
	view := keys(New(), " ").(Model).View()

	for _, want := range []string{"tray", "one", "two", "●", "urg", "pri", "enter act", "? help"} {
		if !strings.Contains(view, want) {
			t.Errorf("frame missing %q:\n%s", want, view)
		}
	}
}

// The footer wraps rather than truncating, so every key it knows about is on screen.
// It used to shed keys for width — and then hid `v` unless it was already on, which
// is no way to find out a key exists.
func TestFooterNamesEveryKeyItKnows(t *testing.T) {
	sandbox(t, "- [ ] one priority:H")
	for _, width := range []int{80, 120} {
		out, _ := New().Update(tea.WindowSizeMsg{Width: width, Height: 20})
		m := out.(Model)
		view := m.View()
		for _, b := range m.keys().ShortHelp() {
			if !strings.Contains(view, b.Help().Key+" "+b.Help().Desc) {
				t.Errorf("at %d cols the footer omits %q:\n%s", width, b.Help().Key, view)
			}
		}
		// No key may be lost to an ellipsis, which is what truncation did.
		if strings.Contains(view, "…\n") {
			t.Errorf("at %d cols the footer truncated:\n%s", width, view)
		}
	}
}

// `?` is a screen of its own. A keymap tells you which letter does a thing you
// already understand; what needs explaining here is why there are two layers at all,
// so the diagram leads and the keys follow.
func TestHelpScreenExplainsTheTwoLayers(t *testing.T) {
	sandbox(t, "- [ ] a task worth seeing behind the dialog priority:H")
	m := keys(New(), "?").(Model)
	if !m.help.ShowAll {
		t.Fatal("? should open the dialog")
	}
	out, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 26})
	view := out.(Model).View()

	for _, want := range []string{
		"garage", "tray", // the two layers, as boxes
		"take", "hand back", // and the move between them
		"a line to the layer", "onto the tray, structured", // what each move does
		"keys", "acting", // one keymap section, headed
		"j k",                          // the alias the footer doesn't name
		"restore", "rewrite", "delete", // the letters only the menu shows
	} {
		if !strings.Contains(view, want) {
			t.Errorf("the help dialog never mentions %q:\n%s", want, view)
		}
	}
	// It takes the screen: the list, the tabs and the footer are all gone while it
	// is open, which is what lets it use the full width.
	for _, gone := range []string{"↑↓ move · space select", "garage · August", "urg"} {
		if strings.Contains(view, gone) {
			t.Errorf("the interface should be gone behind the help, %q showing:\n%s", gone, view)
		}
	}
	if !strings.Contains(view, "any key to go back") {
		t.Errorf("it should say how to leave:\n%s", view)
	}
	// It has to say what tray is, not only how to drive it.
	for _, want := range []string{"task tracker in two layers", "half-thought", "pick it up"} {
		if !strings.Contains(view, want) {
			t.Errorf("the dialog should explain the idea, %q missing:\n%s", want, view)
		}
	}
	// The letter sits inside the verb, so it reads as a word and marks the key at
	// once. `(d)` has no d-word to sit in: `D` already owns delete.
	for _, want := range []string{"(a)dd", "(t)ake", "(d)", "(enter)"} {
		if !strings.Contains(view, want) {
			t.Errorf("the help should show %q:\n%s", want, view)
		}
	}
	// The diagram says what the layers are and nothing else. Keys live under it: a
	// picture carrying its own keybindings is doing two jobs and neither well.
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "─ garage ") || strings.Contains(line, "any month") {
			if strings.Contains(line, "(") {
				t.Errorf("the diagram should carry no keys:\n%s", line)
			}
		}
	}
	// Progressive disclosure: prose, then the picture, then the full keymap below a
	// rule. The keymap must come last, or a newcomer meets twenty letters first.
	if strings.Index(view, "two layers") > strings.Index(view, "hand back") {
		t.Error("the explanation should come before the keymap")
	}
}

// It is modal: anything at all dismisses it, and the key is spent doing so — you
// should never have to work out which key closes a thing that is in your way.
func TestAnyKeyClosesTheHelpDialog(t *testing.T) {
	sandbox(t, "- [ ] one priority:H", "- [ ] two priority:M")
	open := keys(New(), "?").(Model)

	for _, k := range []string{"?", "esc", "j", "x", "q", "a", "tab", " ", "/", "R"} {
		after := keys(open, k).(Model)
		if after.help.ShowAll {
			t.Errorf("%q should close the dialog", k)
		}
		// Spent closing, not also acted on: `x` must not finish anything, `a` must
		// not open a form, `j` must not move.
		if after.form != nil || after.mode != browsing || after.list.Index() != 0 {
			t.Errorf("%q did something besides closing", k)
		}
		if got := trayFile(t); strings.Contains(got, "~~") {
			t.Fatalf("%q acted on a task while dismissing:\n%s", k, got)
		}
	}
	// ctrl+c still means what it always means.
	if _, cmd := open.Update(keyMsg("ctrl+c")); cmd == nil {
		t.Error("ctrl+c should still quit from the dialog")
	}
}

// The dialog's layout has broken three times in a row — a wrapped column, two
// labels colliding, a key clipped to "tab  h…". Its widths are measured now, but the
// measuring is only as good as this check: nothing may exceed the terminal, and no
// key may be truncated into an ellipsis.
// The layout has broken four times now — a wrapped column, two labels colliding, a
// key clipped to "tab  h…", and a frame that grew while its contents did not. It is
// measured from the content, and this is what checks the measuring.
func TestHelpScreenFitsTheTerminal(t *testing.T) {
	sandbox(t, "- [ ] one priority:H")
	for _, size := range [][2]int{{80, 24}, {80, 30}, {100, 26}, {110, 34}, {60, 20}} {
		width, height := size[0], size[1]
		out, _ := New().Update(tea.WindowSizeMsg{Width: width, Height: height})
		view := keys(out, "?").(Model).View()
		lines := strings.Split(strings.TrimRight(view, "\n"), "\n")
		for _, line := range lines {
			if got := lipgloss.Width(line); got > width {
				t.Errorf("at %dx%d a line is %d wide: %q", width, height, got, line)
			}
		}
		if len(lines) > height {
			t.Errorf("at %dx%d the view is %d rows", width, height, len(lines))
		}
		if strings.Contains(view, "…") {
			t.Errorf("at %dx%d something was truncated:\n%s", width, height, view)
		}
		// It grows with the terminal but never fills it — a box spanning every
		// column is not floating over anything.
		// It grows with the terminal: the prose reflows and the boxes spread, so a
		// wider screen shows a wider help rather than the same block with more air.
		if width >= minDiagramWidth {
			wide := 0
			for _, line := range lines {
				wide = max(wide, lipgloss.Width(line))
			}
			if wide < width-8 {
				t.Errorf("at %dx%d the help only used %d columns", width, height, wide)
			}
		}
	}
}

// Drives a real tea.Program, so the wiring itself is covered rather than Update alone.
func TestProgramRunsAndQuits(t *testing.T) {
	sandbox(t, "- [ ] one priority:H")
	drive(t, New()).waitFor("one").press("q").final()
}

// The delegate renders marks by reading the Model's own map, so the two must stay
// the same map. Clearing marks has to be clear(), never a fresh map: reassigning it
// leaves the delegate pointing at the old one and marks silently stop drawing.
func TestMarksKeepRenderingAfterTheyAreCleared(t *testing.T) {
	sandbox(t, "- [ ] one priority:H", "- [ ] two priority:M")
	garage(t, "2026-08", "- something jotted")

	marked := func(m Model) bool { return strings.Contains(m.View(), "●") }

	m := keys(New(), " ").(Model)
	if !marked(m) {
		t.Fatalf("a mark should draw a ●:\n%s", m.View())
	}
	if !m.deleg.marked["one"] {
		t.Error("the delegate is not reading the model's mark map")
	}

	// Both paths that clear marks: a tab switch, and an action.
	m = keys(m, "tab", "shift+tab").(Model)
	if marked(m) {
		t.Error("switching tabs should clear the marks")
	}
	m = keys(m, " ").(Model)
	if !marked(m) {
		t.Errorf("marks must still draw after a tab switch cleared them:\n%s", m.View())
	}

	m = keys(m, "x").(Model) // done clears the marks too
	m = keys(m, " ").(Model)
	if !marked(m) {
		t.Errorf("marks must still draw after an action cleared them:\n%s", m.View())
	}
}

// The tray file writes a checkbox, so the tray rows show one. `[-]` for dropped is
// Obsidian's spelling of a state markdown has no box for. The garage file has no
// checkbox, so its rows keep a one-character mark instead.
func TestTrayShowsCheckboxesAndTheGarageDoesNot(t *testing.T) {
	sandbox(t,
		"- [ ] still open priority:M",
		"- [x] ~~finished~~ priority:M done:2026-08-06",
		"- [ ] ~~abandoned~~ priority:M dropped:2026-08-06",
	)
	garage(t, "2026-08", "- a jotting", "- ~~a finished jotting~~ done:2026-08-06")

	tray := keys(New(), "v").(Model).View()
	for _, want := range []string{"[ ] still open", "[x] finished", "[-] abandoned"} {
		if !strings.Contains(tray, want) {
			t.Errorf("the tray should show %q:\n%s", want, tray)
		}
	}

	garageView := keys(New(), "tab", "v").(Model).View()
	if strings.Contains(garageView, "[ ]") || strings.Contains(garageView, "[x]") {
		t.Errorf("the garage file has no checkbox, so its rows must not draw one:\n%s", garageView)
	}
	for _, want := range []string{"✓ a finished jotting", "a jotting"} {
		if !strings.Contains(garageView, want) {
			t.Errorf("the garage should show %q:\n%s", want, garageView)
		}
	}
}

// Selection and state used to share a cell, so a selected row that was also finished
// lost its dot. They are separate columns now.
func TestASelectedFinishedRowShowsBoth(t *testing.T) {
	sandbox(t, "- [x] ~~finished~~ priority:M done:2026-08-06")
	view := keys(New(), "v", " ").(Model).View()
	if !strings.Contains(view, "● [x]") {
		t.Errorf("a selected finished row should show both:\n%s", view)
	}
}
