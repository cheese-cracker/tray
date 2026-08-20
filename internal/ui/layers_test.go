package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cheese-cracker/tray/internal/store"
)

func garage(t *testing.T, month string, lines ...string) {
	t.Helper()
	doc, err := store.Garage(month)
	if err != nil {
		t.Fatal(err)
	}
	doc.Lines = append([]string{store.MonthHeader(month), ""}, lines...)
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}
}

func monthFile(t *testing.T, month string) string {
	t.Helper()
	lines, err := store.Read(store.MonthPath(month))
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(lines, "\n")
}

// Day to day there are exactly two: what you're doing, and what you dumped.
func TestTabsAreTrayAndThisMonth(t *testing.T) {
	sandbox(t, "- [ ] on the tray")
	m := New()
	if len(m.layers) != 2 {
		t.Fatalf("tabs = %v, want just tray + this month", titles(m))
	}
	if !m.layers[0].isTray() || m.layers[1].month != "2026-08" {
		t.Errorf("tabs = %v", titles(m))
	}
}

// Neither someday nor an old month earns standing room, however much is in them.
func TestNoSomedayOrOldMonthTab(t *testing.T) {
	sandbox(t)
	garage(t, "2026-07", "- left over from july")
	garage(t, store.Someday, "- one day maybe")

	for _, l := range New().layers {
		if l.month == store.Someday || l.month == "2026-07" {
			t.Errorf("%q should not be a tab", l.title)
		}
	}
}

// The sweep is the exception: that ritual is about months, so months get the tabs.
func TestSweepTabsAreTheMonths(t *testing.T) {
	sandbox(t)
	garage(t, "2026-07", "- left over from july")

	m := NewSweep("")
	if len(m.layers) != 3 {
		t.Fatalf("sweep tabs = %v, want closing + this + someday", titles(m))
	}
	if m.layers[0].month != "2026-07" {
		t.Errorf("the sweep should open on the closing month, got %v", titles(m))
	}
	if m.layers[2].month != store.Someday {
		t.Errorf("someday should be reachable while sweeping: %v", titles(m))
	}
	if m.items[0].Text != "left over from july" {
		t.Errorf("items = %v", texts(m))
	}
}

func TestSweepHonoursAChosenMonth(t *testing.T) {
	sandbox(t)
	garage(t, "2026-05", "- ancient")
	m := NewSweep("2026-05")
	if m.layers[0].month != "2026-05" {
		t.Errorf("tabs = %v, want the month asked for", titles(m))
	}
}

// Someday has no tab, so `>` is the only way to reach it — it must be offered.
func TestSomedayIsStillADestination(t *testing.T) {
	sandbox(t, "- [ ] on the tray")
	var found bool
	for _, d := range New().destinations() {
		if d.month == store.Someday {
			found = true
		}
	}
	if !found {
		t.Error("someday must stay reachable through move-to")
	}
}

func TestTabSwitchesLayer(t *testing.T) {
	sandbox(t, "- [ ] on the tray")
	garage(t, "2026-08", "- in the garage")

	m := New()
	if m.items[0].Text != "on the tray" {
		t.Fatalf("first tab should be the tray, got %q", m.items[0].Text)
	}
	m = keys(m, "tab").(Model)
	if m.layer().isTray() {
		t.Fatal("tab should leave the tray")
	}
	if len(m.items) != 1 || m.items[0].Text != "in the garage" {
		t.Errorf("garage tab shows %v", texts(m))
	}
	m = keys(m, "shift+tab").(Model)
	if !m.layer().isTray() {
		t.Error("shift+tab should come back")
	}
	// l and h are the same movement, for people who never leave home row.
	if keys(m, "l").(Model).layer().isTray() {
		t.Error("l should move a tab right")
	}
}

func TestSwitchingTabsClearsMarks(t *testing.T) {
	sandbox(t, "- [ ] one", "- [ ] two")
	garage(t, "2026-08", "- in the garage")
	m := keys(New(), " ").(Model)
	if len(m.marked) != 1 {
		t.Fatal("setup: nothing marked")
	}
	m = keys(m, "tab").(Model)
	if len(m.marked) != 0 {
		t.Errorf("marks belong to a layer: %v", m.marked)
	}
}

func TestGarageOffersTakeAndNotHandBack(t *testing.T) {
	sandbox(t)
	garage(t, "2026-08", "- in the garage")
	m := keys(New(), "tab").(Model)

	var keysOffered []string
	for _, a := range m.offered() {
		keysOffered = append(keysOffered, a.key)
	}
	joined := strings.Join(keysOffered, "")
	if !strings.Contains(joined, "t") {
		t.Errorf("garage should offer take, got %v", keysOffered)
	}
	if strings.Contains(joined, "d") && !strings.Contains(joined, "D") {
		t.Errorf("hand back makes no sense in the garage, got %v", keysOffered)
	}
}

// take is the structuring step: it moves the line and opens the form on what landed.
func TestTakeFromGarageMovesAndOpensTheForm(t *testing.T) {
	sandbox(t)
	garage(t, "2026-08", "- add retries to the sync job")
	m := keys(New(), "tab", "t").(Model)

	if got := trayFile(t); !strings.Contains(got, "add retries to the sync job") {
		t.Errorf("tray did not receive it:\n%s", got)
	}
	if got := monthFile(t, "2026-08"); !strings.Contains(got, "→ tray") {
		t.Errorf("source should be annotated, not removed:\n%s", got)
	}
	if m.mode != editing || m.form == nil {
		t.Error("take should open the retake form on what landed")
	}
	if m.form.month != "" {
		t.Errorf("the form should be editing the tray, got month %q", m.form.month)
	}
}

func TestMoveToCarriesForwardWithAnArrow(t *testing.T) {
	sandbox(t)
	garage(t, "2026-08", "- not this month after all")
	m := keys(New(), "tab", ">").(Model)
	if m.mode != sending {
		t.Fatal("> should ask where to")
	}
	if len(m.dests) == 0 {
		t.Fatal("no destinations offered")
	}

	// Walk to September and send it there.
	target := -1
	for i, d := range m.dests {
		if d.month == "2026-09" {
			target = i
		}
	}
	if target < 0 {
		t.Fatalf("next month should be a destination: %v", m.dests)
	}
	for i := 0; i < target; i++ {
		m = keys(m, "j").(Model)
	}
	m = keys(m, "enter").(Model)

	if got := monthFile(t, "2026-09"); !strings.Contains(got, "not this month after all") {
		t.Errorf("september did not receive it:\n%s", got)
	}
	if got := monthFile(t, "2026-08"); !strings.Contains(got, "→ 2026-09") {
		t.Errorf("august should keep the line, annotated:\n%s", got)
	}
	if m.mode != browsing {
		t.Error("should return to the list after moving")
	}
}

func TestDestinationsExcludeWhereYouAlreadyAre(t *testing.T) {
	sandbox(t, "- [ ] on the tray")
	m := New()
	for _, d := range m.destinations() {
		if d.month == m.layer().month {
			t.Errorf("destinations should not include the current layer: %v", d)
		}
	}
}

func TestTabBarShowsEveryLayer(t *testing.T) {
	sandbox(t, "- [ ] one")
	view := New().View()
	if strings.Contains(view, "someday") {
		t.Errorf("someday should not be on the tab bar:\n%s", view)
	}
	for _, want := range []string{"tray", "August"} {
		if !strings.Contains(view, want) {
			t.Errorf("tab bar missing %q:\n%s", want, view)
		}
	}
}

func titles(m Model) []string {
	var out []string
	for _, l := range m.layers {
		out = append(out, l.title)
	}
	return out
}

func texts(m Model) []string {
	var out []string
	for _, t := range m.items {
		out = append(out, t.Text)
	}
	return out
}

// Full-page layout. The dangerous direction is overshooting the terminal, which
// makes the alt screen jitter, so these assert we stay inside it.
func TestFullPageFitsTheTerminal(t *testing.T) {
	sandbox(t, "- [ ] one priority:H", "- [ ] two priority:M")
	m := New()
	out, _ := m.Update(tea.WindowSizeMsg{Width: 70, Height: 18})
	view := out.(Model).View()

	for i, line := range strings.Split(strings.TrimRight(view, "\n"), "\n") {
		if w := lipgloss.Width(line); w > 70 {
			t.Errorf("line %d is %d wide, past the 70-column terminal", i, w)
		}
	}
	if h := lipgloss.Height(strings.TrimRight(view, "\n")); h > 18 {
		t.Errorf("view is %d rows, past the 18-row terminal", h)
	}
}

// A list longer than the terminal must window rather than overflow, and the cursor
// has to stay on screen wherever it is.
func TestLongListWindowsAroundTheCursor(t *testing.T) {
	var lines []string
	for i := 0; i < 30; i++ {
		lines = append(lines, fmt.Sprintf("- [ ] task %02d priority:M", i))
	}
	sandbox(t, lines...)

	m := New()
	out, _ := m.Update(tea.WindowSizeMsg{Width: 60, Height: 14})
	m = out.(Model)

	for _, at := range []int{0, 15, 29} {
		m.cursor = at
		start, end := m.window()
		if at < start || at >= end {
			t.Errorf("cursor %d is outside the window %d..%d", at, start, end)
		}
		if end-start >= len(m.items) {
			t.Errorf("30 tasks should not all fit in 14 rows: %d..%d", start, end)
		}
		if h := lipgloss.Height(strings.TrimRight(m.View(), "\n")); h > 14 {
			t.Errorf("cursor %d: view is %d rows, past the terminal", at, h)
		}
	}
	if !strings.Contains(m.View(), "more") {
		t.Error("a windowed list should say how much is hidden")
	}
}

// The round trip that was broken: take something, then hand it straight back. It
// must leave the tray, come home to the garage line it came from, and not double up.
func TestHandBackReturnsTheLineItCameFrom(t *testing.T) {
	sandbox(t)
	garage(t, "2026-08", "- fix the sync job")

	m := keys(New(), "tab", "t").(Model) // take it
	m = keys(m, "esc").(Model)           // skip the form
	if got := trayFile(t); !strings.Contains(got, "fix the sync job") {
		t.Fatalf("setup: not taken:\n%s", got)
	}

	keys(New(), "d") // hand it back from the tray

	tray := trayFile(t)
	if strings.Contains(tray, "fix the sync job") {
		t.Errorf("it should have left the tray:\n%s", tray)
	}
	month := monthFile(t, "2026-08")
	if strings.Contains(month, "→ tray") {
		t.Errorf("the arrow should be cleared — it isn't on the tray any more:\n%s", month)
	}
	if n := strings.Count(month, "fix the sync job"); n != 1 {
		t.Errorf("want exactly one line, got %d:\n%s", n, month)
	}
	if len(New().items) != 0 {
		t.Error("the tray should be empty")
	}
}

func TestRetakeIsTheDefaultOnTheTray(t *testing.T) {
	sandbox(t, "- [ ] a thing priority:M")
	m := keys(New(), "enter").(Model)
	if got := m.offered()[m.menuAt].key; got != "r" {
		t.Errorf("the default action is %q, want retake", got)
	}
}

func TestTakeIsTheDefaultInTheGarage(t *testing.T) {
	sandbox(t)
	garage(t, "2026-08", "- something jotted")
	m := keys(New(), "tab", "enter").(Model)
	if got := m.offered()[m.menuAt].key; got != "t" {
		t.Errorf("the default action is %q, want take", got)
	}
}

func TestTabsCycle(t *testing.T) {
	sandbox(t, "- [ ] on the tray")
	m := New()
	if len(m.layers) != 2 {
		t.Fatalf("setup: %v", titles(m))
	}

	// Forwards off the end comes back to the start.
	m = keys(m, "tab").(Model)
	if m.layer().isTray() {
		t.Fatal("setup: tab did not move")
	}
	m = keys(m, "tab").(Model)
	if !m.layer().isTray() {
		t.Error("tab off the last tab should wrap to the first")
	}

	// And backwards off the front goes to the end.
	m = keys(m, "shift+tab").(Model)
	if m.layer().isTray() {
		t.Error("shift+tab off the first tab should wrap to the last")
	}

	// h and l are the same movement, so they wrap too.
	m = New()
	for i := 0; i < len(m.layers); i++ {
		m = keys(m, "l").(Model)
	}
	if !m.layer().isTray() {
		t.Error("l all the way round should land back on the tray")
	}
}
