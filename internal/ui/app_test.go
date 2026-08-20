package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"

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
		var msg tea.KeyMsg
		switch k {
		case " ":
			msg = tea.KeyMsg{Type: tea.KeySpace}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		case "esc":
			msg = tea.KeyMsg{Type: tea.KeyEsc}
		case "backspace":
			msg = tea.KeyMsg{Type: tea.KeyBackspace}
		case "left":
			msg = tea.KeyMsg{Type: tea.KeyLeft}
		case "right":
			msg = tea.KeyMsg{Type: tea.KeyRight}
		case "up":
			msg = tea.KeyMsg{Type: tea.KeyUp}
		case "down":
			msg = tea.KeyMsg{Type: tea.KeyDown}
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "shift+tab":
			msg = tea.KeyMsg{Type: tea.KeyShiftTab}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		m, _ = m.Update(msg)
	}
	return m
}

func TestListOrdersByUrgency(t *testing.T) {
	sandbox(t,
		"- [ ] low thing priority:L",
		"- [ ] urgent thing priority:H due:2026-08-08",
		"- [ ] middle thing priority:M",
	)
	m := New()
	if len(m.items) != 3 {
		t.Fatalf("got %d items", len(m.items))
	}
	if m.items[0].Text != "urgent thing" {
		t.Errorf("first row = %q, want the most urgent", m.items[0].Text)
	}
}

func TestTerminalTasksAreNotListed(t *testing.T) {
	sandbox(t,
		"- [ ] open thing",
		"- [x] ~~done thing~~ done:2026-08-01",
		"- ~~dropped thing~~ dropped:2026-08-01",
	)
	if got := len(New().items); got != 1 {
		t.Errorf("listed %d items, want only the live one", got)
	}
}

func TestCursorMoves(t *testing.T) {
	sandbox(t, "- [ ] one priority:H", "- [ ] two priority:M", "- [ ] three priority:L")
	m := keys(New(), "j", "j").(Model)
	if m.cursor != 2 {
		t.Errorf("cursor = %d, want 2", m.cursor)
	}
	m = keys(m, "k").(Model)
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want 1", m.cursor)
	}
	// It must not run off either end.
	m = keys(m, "k", "k", "k").(Model)
	if m.cursor != 0 {
		t.Errorf("cursor = %d, want 0", m.cursor)
	}
	m = keys(m, "G").(Model)
	if m.cursor != 2 {
		t.Errorf("G should go last, got %d", m.cursor)
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
	if len(m.items) != 1 {
		t.Errorf("the finished task should leave the list, got %d", len(m.items))
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

	for _, want := range []string{"tray", "one", "two", "●", "space mark", "enter act"} {
		if !strings.Contains(view, want) {
			t.Errorf("frame missing %q:\n%s", want, view)
		}
	}
}

// Drives a real tea.Program, so the wiring itself is covered rather than Update alone.
func TestProgramRunsAndQuits(t *testing.T) {
	sandbox(t, "- [ ] one priority:H")
	tm := teatest.NewTestModel(t, New(), teatest.WithInitialTermSize(80, 24))

	teatest.WaitFor(t, tm.Output(), func(out []byte) bool {
		return strings.Contains(string(out), "one")
	}, teatest.WithDuration(3*time.Second))

	tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	tm.WaitFinished(t, teatest.WithFinalTimeout(3*time.Second))
}
