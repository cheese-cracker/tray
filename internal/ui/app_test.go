package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

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
	m = keys(m, "G").(Model)
	if m.list.Index() != 2 {
		t.Errorf("G should go last, got %d", m.list.Index())
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

// The footer sheds keys to stay on one line, so `?` has to be the place they turn
// up. If a key is in neither, it doesn't exist as far as a new user is concerned.
func TestHelpOverlayCarriesWhatTheFooterDrops(t *testing.T) {
	sandbox(t, "- [ ] one priority:H")
	m := keys(New(), "?").(Model)
	if !m.help.ShowAll {
		t.Fatal("? should open the overlay")
	}
	view := m.View()
	for _, want := range []string{"space", "mark", "retake", "hand back", "filter", "quit"} {
		if !strings.Contains(view, want) {
			t.Errorf("overlay missing %q:\n%s", want, view)
		}
	}
	if keys(m, "?").(Model).help.ShowAll {
		t.Error("? should close it again")
	}
}

// Drives a real tea.Program, so the wiring itself is covered rather than Update alone.
func TestProgramRunsAndQuits(t *testing.T) {
	sandbox(t, "- [ ] one priority:H")
	drive(t, New()).waitFor("one").press("q").final()
}
