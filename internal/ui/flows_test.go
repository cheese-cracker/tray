package ui

import (
	"strings"
	"testing"
)

// The flows that are only true end to end: several keystrokes, a mode change in the
// middle, and a file on the other side. Each one is a row in FLOWS.md, and the
// linkage test in internal/flows fails if a row here has no row there.
//
// All of them drive the real bubbletea program through the harness in tui_test.go,
// which bounds every wait. Assertions are on the final model and on what landed on
// disk — never on a frame, which is what decision 40 is about.

func has(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Errorf("missing %q in:\n%s", want, got)
	}
}

func hasNot(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Errorf("unexpected %q in:\n%s", want, got)
	}
}

// T1 · take is the one moment structure gets paid for, so it has to open the form
// with the line already on the tray and every field ready to be left alone.
func TestFlowTakeOpensTheFormAndSaves(t *testing.T) {
	sandbox(t)
	garage(t, "2026-08", "- add retries to the sync job")

	u := drive(t, New()).waitFor("tray")
	u.press("l").waitFor("add retries") // to the garage tab
	u.press("t").waitFor("retake")      // take, which opens the form
	u.press("down", "left")             // priority field, step it up toward H
	u.press("enter")
	// The status is the form's, not the move's: take runs first, then the form
	// saves over it. The arrow it left behind is in the file, which is the record.
	m := u.waitFor("retook 1").final()

	if m.mode != browsing {
		t.Errorf("the form should have closed, mode = %v", m.mode)
	}
	has(t, trayFile(t), "add retries to the sync job")
	has(t, trayFile(t), "priority:H")
	has(t, monthFile(t, "2026-08"), "→ tray")
}

// T2 · with several marked the form skips the title: one name for many is never the
// intent. The batch still has to reach every task.
func TestFlowBatchRetakeSkipsTheTitle(t *testing.T) {
	sandbox(t, "- [ ] one priority:L", "- [ ] two priority:L")

	u := drive(t, New()).waitFor("one")
	u.press(" ", "j", " ") // mark both
	u.press("r").waitFor("retake 2 tasks")
	u.press("left", "enter") // priority L -> M, save
	m := u.waitFor("retook 2").final()

	if len(m.marked) != 0 {
		t.Errorf("marks should clear after the form, got %v", m.marked)
	}
	tray := trayFile(t)
	if n := strings.Count(tray, "priority:M"); n != 2 {
		t.Errorf("both tasks should be M, got %d:\n%s", n, tray)
	}
	has(t, tray, "- [ ] one")
	has(t, tray, "- [ ] two")
}

// T3 · a filter is only useful if you can act on what it left. The action has to hit
// the visible row, not the row that was under the cursor before you filtered.
func TestFlowFilterThenActOnAFilteredRow(t *testing.T) {
	sandbox(t,
		"- [ ] rotate the api keys priority:H",
		"- [ ] book the flights priority:L",
		"- [ ] renew the passport priority:M",
	)

	u := drive(t, New()).waitFor("rotate")
	u.press("/").typeIn("passport")
	u.press("enter").waitFor("1 of 3") // filter applied, one row left
	u.press("x")                       // done, straight from the list
	m := u.waitFor("1 done").final()

	if got := len(m.items()); got != 0 {
		t.Errorf("the finished row should leave the filtered list, %d left", got)
	}
	tray := trayFile(t)
	has(t, tray, "~~renew the passport~~")
	hasNot(t, tray, "~~rotate")
	hasNot(t, tray, "~~book")
}

// T4 · hiding a row is not deselecting it. Filter, mark, filter again, and every mark
// still counts — otherwise a filtered sweep silently drops half your selection.
func TestFlowMarksSurviveAFilter(t *testing.T) {
	sandbox(t,
		"- [ ] alpha task priority:M",
		"- [ ] beta task priority:M",
		"- [ ] gamma task priority:M",
	)

	u := drive(t, New()).waitFor("alpha")
	u.press("/").typeIn("alpha")
	u.press("enter").waitFor("1 of 3")
	u.press(" ")   // mark alpha while beta and gamma are hidden
	u.press("esc") // clear the filter
	u.waitFor("beta")
	u.press("/").typeIn("beta")
	u.press("enter").waitFor("1 of 3")
	u.press(" ") // mark beta too
	u.press("esc").waitFor("gamma")
	u.press("x") // done applies to both marks, not to the row under the cursor
	m := u.waitFor("2 done").final()

	if len(m.marked) != 0 {
		t.Errorf("marks should clear after the action, got %v", m.marked)
	}
	tray := trayFile(t)
	has(t, tray, "~~alpha task~~")
	has(t, tray, "~~beta task~~")
	hasNot(t, tray, "~~gamma task~~")
}

// T5 · with two or three tabs, stopping at either end just makes you reach for the
// other key. Decision 28a.
func TestFlowTabsCycleBothWays(t *testing.T) {
	sandbox(t, "- [ ] on the tray priority:M")
	garage(t, "2026-08", "- in the garage")

	u := drive(t, New()).waitFor("on the tray")
	u.press("l").waitFor("in the garage")
	u.press("l").waitFor("on the tray") // wrapped forward off the last tab
	u.press("h").waitFor("in the garage")
	m := u.press("h").waitFor("on the tray").final() // and back off the first

	if m.active != 0 {
		t.Errorf("should be back on the tray, active = %d", m.active)
	}
}

// T6 · `>` is the month sweep. The source line stays as a record with an arrow, and
// the live copy lands where you sent it — decision 6, and the reason `find` works.
func TestFlowMoveToCopiesForwardWithAnArrow(t *testing.T) {
	sandbox(t)
	garage(t, "2026-08", "- not this month after all")

	u := drive(t, New()).waitFor("tray")
	u.press("l").waitFor("not this month")
	u.press(">").waitFor("move 1 to")
	u.press("j").press("enter") // past the tray, to a month
	m := u.waitFor("→").final()

	if m.mode != browsing {
		t.Errorf("the picker should have closed, mode = %v", m.mode)
	}
	has(t, monthFile(t, "2026-08"), "→ ")
}

// T7 · the bug 34b was written for: handing back used to leave the task on neither
// layer, because dedupe counted the departed line as still being there.
func TestFlowHandBackRevivesTheGarageLine(t *testing.T) {
	sandbox(t)
	garage(t, "2026-08", "- fix the sync job")

	u := drive(t, New()).waitFor("tray")
	u.press("l").waitFor("fix the sync job")
	u.press("t").waitFor("retake") // take it, and give it a priority on the way
	u.press("down", "left")
	u.press("enter").waitFor("retook 1")
	u.press("h").waitFor("fix the sync job")
	u.press("d") // hand it straight back
	u.final()

	month := monthFile(t, "2026-08")
	hasNot(t, trayFile(t), "fix the sync job")
	has(t, month, "fix the sync job")
	hasNot(t, month, "→ tray") // the arrow is gone: the line came home, not a copy
	if n := strings.Count(month, "fix the sync job"); n != 1 {
		t.Errorf("handing back should revive one line, not add a copy:\n%s", month)
	}
	// Coming home must not undo what the tray added, or taking it again costs the
	// same structuring twice.
	has(t, month, "fix the sync job priority:H")
}

// T8 · the garage asks for the words and nothing else — that is the whole point of
// it. Decision 33.
func TestFlowGarageAddAsksOnlyForATitle(t *testing.T) {
	sandbox(t)
	garage(t, "2026-08")

	u := drive(t, New()).waitFor("tray")
	u.press("l").waitFor("nothing here")
	u.press("a").waitFor("nothing else needed")
	u.typeIn("that config thing")
	u.press("enter")
	u.waitFor("added").final()

	month := monthFile(t, "2026-08")
	has(t, month, "- that config thing")
	hasNot(t, month, "priority:") // nothing was asked for, so nothing was written
}

// T9 · the tray is where structure is expected, so a new task there gets every
// field. Decision 33, the other half.
func TestFlowTrayAddTakesTheWholeForm(t *testing.T) {
	sandbox(t)

	u := drive(t, New()).waitFor("nothing on the tray")
	u.press("a").waitFor("add to the tray")
	u.typeIn("rotate the api keys")
	u.press("down", "left")           // priority M -> H, leftward: the radio reads H · M · L
	u.press("down", "right", "right") // due: the first → lands on today, the next steps a day
	u.press("down").typeIn("infra")   // tag
	u.press("enter")
	u.waitFor("added").final()

	tray := trayFile(t)
	has(t, tray, "- [ ] rotate the api keys")
	has(t, tray, "priority:H")
	has(t, tray, "due:2026-08-08")
	has(t, tray, "+infra")
}

// T10 · esc has two jobs and they have to happen in that order, or clearing a filter
// drops you out of the program.
func TestFlowEscClearsTheFilterBeforeItQuits(t *testing.T) {
	sandbox(t, "- [ ] alpha task priority:M", "- [ ] beta task priority:M")

	u := drive(t, New()).waitFor("alpha")
	u.press("/").typeIn("alpha")
	u.press("enter").waitFor("1 of 2")
	u.press("esc").waitFor("beta") // still running, filter gone
	m := u.press("q").final()

	if m.filtering() {
		t.Error("esc should have cleared the filter")
	}
	if len(m.items()) != 2 {
		t.Errorf("both rows should be back, got %d", len(m.items()))
	}
}

// T11 · `tray carryover` opens the same interface with the months as tabs, because
// that ritual is the one time the months matter more than the tray. It opens on the
// current month: which month is "closing" depends on whether you sweep on the 30th or
// the 10th, so there is nothing honest to guess.
func TestFlowSweepOpensTheMonthsAsTabs(t *testing.T) {
	sandbox(t, "- [ ] on the tray priority:M")
	garage(t, "2026-07", "- left over from july")
	garage(t, "2026-08", "- dumped this month")

	u := drive(t, NewSweep("2026-07")).waitFor("dumped this month")
	m := u.press("q").final()

	if m.layer().month != "2026-08" {
		t.Errorf("the sweep should open on the current month, got %q", m.layer().month)
	}
	var months []string
	for _, tab := range m.layers {
		if tab.isTray() {
			t.Error("the tray does not get a tab during the sweep")
		}
		months = append(months, tab.month)
	}
	want := []string{"2026-07", "2026-08", "2026-09", "someday"}
	if strings.Join(months, " ") != strings.Join(want, " ") {
		t.Errorf("tabs = %v, want %v", months, want)
	}
	hasNot(t, m.View(), "on the tray")

	// The closing month is one `h` away, and it is a real tab, not a destination.
	back := keys(m, "h").(Model)
	if back.layer().month != "2026-07" {
		t.Errorf("h should reach the closing month, got %q", back.layer().month)
	}
	has(t, back.View(), "left over from july")
}

// T12 · `?` is the only place some keys are named, so it has to open, list them, and
// close again without disturbing the list underneath.
func TestFlowHelpOverlayToggles(t *testing.T) {
	sandbox(t, "- [ ] one priority:H")

	u := drive(t, New()).waitFor("one")
	u.press("?").waitFor("hand back")
	m := u.press("?").press("q").final()

	if m.help.ShowAll {
		t.Error("the second ? should have closed the overlay")
	}
	if len(m.items()) != 1 {
		t.Errorf("the list should be untouched, got %d rows", len(m.items()))
	}
}

// T13 · pasting a title. The form is hand-rolled, and it used to accept only
// single-rune messages — so a paste, which arrives as one message carrying all of
// it, was dropped silently. A pasted block can also carry newlines, and a task is
// one line of a markdown file.
func TestFlowPasteIntoTheTitle(t *testing.T) {
	sandbox(t)

	u := drive(t, New()).waitFor("nothing on the tray")
	u.press("a").waitFor("add to the tray")
	u.paste("rotate the api keys\nand the certs")
	u.typeIn("!")
	u.press("enter")
	u.waitFor("added").final()

	tray := trayFile(t)
	has(t, tray, "rotate the api keys and the certs!") // collapsed to a space, not split

	// Counting bullets would not catch this: a split leaves the tail with no bullet
	// at all, so the count stays at one while half the task sits on a line the
	// parser will never recognise.
	for _, line := range strings.Split(tray, "\n") {
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "- ") {
			continue
		}
		t.Errorf("a pasted newline left the orphan line %q in:\n%s", line, tray)
	}
}

// T14 · a finished task is invisible until you ask for it, and the only thing you can
// say about one is that it wasn't finished after all.
func TestFlowViewDoneThenRestore(t *testing.T) {
	sandbox(t,
		"- [ ] still open priority:H entry:2026-08-01",
		"- [x] ~~done by mistake~~ priority:M entry:2026-08-02 done:2026-08-06",
	)

	u := drive(t, New()).waitFor("still open")
	if got := len(u.press("q").final().items()); got != 1 {
		t.Fatalf("a finished task should be hidden by default, saw %d rows", got)
	}

	u = drive(t, New()).waitFor("still open")
	u.press("v").waitFor("done by mistake")
	u.press("j").press("enter").waitFor("restore")
	m := u.press("R").waitFor("1 restored").final()

	tray := trayFile(t)
	has(t, tray, "- [ ] done by mistake")
	hasNot(t, tray, "~~done by mistake~~")
	hasNot(t, tray, "done:2026-08-06")
	has(t, tray, "priority:M") // restoring un-finishes it, it does not strip it
	if got := len(m.items()); got != 2 {
		t.Errorf("both should be live now, saw %d", got)
	}
}

// T15 · the menu on a finished row offers restore and nothing else — `x` on a done
// task or `d` handing one back are meaningless or quietly destructive.
func TestFlowFinishedRowOffersOnlyRestore(t *testing.T) {
	sandbox(t,
		"- [ ] still open priority:H entry:2026-08-01",
		"- [x] ~~already done~~ priority:M entry:2026-08-02 done:2026-08-06",
	)

	m := keys(New(), "v", "j").(Model)
	offered := m.offered()
	if len(offered) != 1 || offered[0].key != "R" {
		var keys []string
		for _, a := range offered {
			keys = append(keys, a.key)
		}
		t.Errorf("a finished row offered %v, want just R", keys)
	}
	// The cursor back on a live row gets the normal menu again.
	if got := len(keys(m, "k").(Model).offered()); got < 4 {
		t.Errorf("a live row should keep its full menu, got %d actions", got)
	}
	// A mixed selection has no single sensible verb, so it keeps the normal menu.
	mixed := keys(New(), "v", " ", "j", " ").(Model)
	if mixed.allFinished() {
		t.Error("a selection spanning both kinds is not all finished")
	}
}
