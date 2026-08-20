package ui

import (
	"strings"
	"testing"

	"github.com/cheese-cracker/tray/internal/store"
)

func openRetake(t *testing.T, presses ...string) Model {
	t.Helper()
	m := keys(New(), append([]string{"r"}, presses...)...).(Model)
	if m.form == nil {
		t.Fatal("r should open the form")
	}
	return m
}

func TestFormOpensPrefilled(t *testing.T) {
	sandbox(t, "- [ ] rotate the api keys priority:M due:2026-08-12 +infra")
	f := openRetake(t).form

	if f.title != "rotate the api keys" {
		t.Errorf("title = %q", f.title)
	}
	if f.prio != "M" || f.due != "2026-08-12" || f.tag != "infra" {
		t.Errorf("prefill = %q %q %q", f.prio, f.due, f.tag)
	}
	if len(f.touched) != 0 {
		t.Error("opening the form must not count as editing")
	}
}

func TestUntouchedFieldsAreLeftAlone(t *testing.T) {
	sandbox(t, "- [ ] rotate the api keys priority:M due:2026-08-12 +infra")
	keys(openRetake(t), "enter") // save immediately, having changed nothing

	got := trayFile(t)
	for _, want := range []string{"rotate the api keys", "priority:M", "due:2026-08-12", "+infra"} {
		if !strings.Contains(got, want) {
			t.Errorf("lost %q:\n%s", want, got)
		}
	}
}

func TestCyclePriority(t *testing.T) {
	sandbox(t, "- [ ] a thing priority:M")
	m := openRetake(t)
	m.form.at = fPriority

	m = keys(m, "l").(Model) // M → H
	if m.form.prio != "H" {
		t.Errorf("after l = %q, want H", m.form.prio)
	}
	m = keys(m, "h", "h").(Model) // H → M → L
	if m.form.prio != "L" {
		t.Errorf("after h h = %q, want L", m.form.prio)
	}
	keys(m, "enter")
	if got := trayFile(t); !strings.Contains(got, "priority:L") {
		t.Errorf("not saved:\n%s", got)
	}
}

// Priority is a radio over three values — there is no "none", and an unset task
// reads as medium.
func TestPriorityHasNoNoneAndDefaultsToMedium(t *testing.T) {
	sandbox(t, "- [ ] no priority yet")
	m := openRetake(t)
	if m.form.prio != "M" {
		t.Errorf("an unset priority should read as M, got %q", m.form.prio)
	}

	m.form.at = fPriority
	m = keys(m, "h", "h", "h").(Model) // walk to the bottom and stay there
	if m.form.prio != "L" {
		t.Errorf("h should clamp at L, got %q", m.form.prio)
	}
	if !strings.Contains(m.View(), "(•) L") {
		t.Errorf("the radio should show the choice:\n%s", m.View())
	}
}

func TestNewTrayTaskIsMediumByDefault(t *testing.T) {
	sandbox(t)
	keys(New(), "a", "d", "o", " ", "i", "t", "enter")
	if got := trayFile(t); !strings.Contains(got, "priority:M") {
		t.Errorf("a new tray task should land at medium:\n%s", got)
	}
}

func TestTypingRenames(t *testing.T) {
	sandbox(t, "- [ ] ab priority:H")
	m := openRetake(t)
	m = keys(m, "c", "d").(Model) // title field is first
	if m.form.title != "abcd" {
		t.Errorf("title = %q", m.form.title)
	}
	m = keys(m, "backspace").(Model)
	if m.form.title != "abc" {
		t.Errorf("backspace failed: %q", m.form.title)
	}
	keys(m, "enter")
	if got := trayFile(t); !strings.Contains(got, "- [ ] abc priority:H") {
		t.Errorf("rename not saved:\n%s", got)
	}
}

// On a text field the vim keys are just letters — that is the whole reason enums
// are picked rather than typed.
func TestVimKeysTypeIntoTheTitle(t *testing.T) {
	sandbox(t, "- [ ] x")
	m := openRetake(t, "h", "j", "k", "l")
	if m.form.title != "xhjkl" {
		t.Errorf("title = %q, want the letters typed", m.form.title)
	}
}

func TestDueShiftsByADay(t *testing.T) {
	sandbox(t, "- [ ] a thing due:2026-08-12")
	m := openRetake(t)
	m.form.at = fDue
	m = keys(m, "right").(Model)
	if got := m.form.due; got != "2026-08-13" {
		t.Errorf("due = %q, want 2026-08-13", got)
	}
	m = keys(m, "left", "left").(Model)
	if got := m.form.due; got != "2026-08-11" {
		t.Errorf("due = %q, want 2026-08-11", got)
	}
}

func TestEmptyDueShiftsFromToday(t *testing.T) {
	sandbox(t, "- [ ] a thing")
	m := openRetake(t)
	m.form.at = fDue
	m = keys(m, "right").(Model)
	if got := m.form.due; got != "2026-08-07" {
		t.Errorf("due = %q, want today", got)
	}
}

// Tags are typed, and the tags already in use are shown as a hint rather than a
// menu — so a new one costs nothing but is still an act, not an accident.
func TestTagIsTyped(t *testing.T) {
	sandbox(t, "- [ ] a thing +infra", "- [ ] another +ops")
	m := openRetake(t)
	m.form.at = fTag
	m.form.tag = ""

	m = keys(m, "b", "i", "l", "l", "i", "n", "g").(Model)
	if m.form.tag != "billing" {
		t.Errorf("tag = %q, want the letters typed", m.form.tag)
	}
	m = keys(m, "backspace").(Model)
	if m.form.tag != "billin" {
		t.Errorf("backspace failed: %q", m.form.tag)
	}

	if hint := m.View(); !strings.Contains(hint, "in use:") {
		t.Errorf("the tags already in use should be offered as a hint:\n%s", hint)
	}

	m.form.tag = "billing"
	keys(m, "enter")
	if got := trayFile(t); !strings.Contains(got, "+billing") {
		t.Errorf("typed tag not saved:\n%s", got)
	}
}

func TestEscCancelsEverything(t *testing.T) {
	sandbox(t, "- [ ] keep me priority:M")
	m := openRetake(t, "z", "z", "z")
	m = keys(m, "esc").(Model)
	if m.form != nil {
		t.Error("esc should close the form")
	}
	if got := trayFile(t); !strings.Contains(got, "- [ ] keep me priority:M") {
		t.Errorf("esc must change nothing:\n%s", got)
	}
}

func TestPriorityClampsAtTheTop(t *testing.T) {
	sandbox(t, "- [ ] a thing priority:H")
	m := openRetake(t)
	m.form.at = fPriority
	m = keys(m, "l", "l").(Model)
	if m.form.prio != "H" {
		t.Errorf("l must not wrap H round to none, got %q", m.form.prio)
	}
}

func TestBatchSkipsTheTitle(t *testing.T) {
	sandbox(t, "- [ ] one priority:M", "- [ ] two priority:M")
	m := keys(New(), " ", "j", " ", "r").(Model) // mark both, retake
	if m.form == nil || !m.form.batch {
		t.Fatal("two marks should open a batch form")
	}
	for _, name := range m.form.fields() {
		if name == fTitle {
			t.Error("a batch form must not offer the title")
		}
	}
	m = keys(m, "l").(Model) // priority is the first field in a batch
	keys(m, "enter")

	got := trayFile(t)
	if strings.Count(got, "priority:H") != 2 {
		t.Errorf("both should have moved to H:\n%s", got)
	}
	if !strings.Contains(got, "one") || !strings.Contains(got, "two") {
		t.Errorf("titles must survive a batch:\n%s", got)
	}
}

func TestFormViewShowsFieldsAndHint(t *testing.T) {
	sandbox(t, "- [ ] a thing priority:M due:2026-08-12 +infra")
	view := openRetake(t).View()
	for _, want := range []string{"retake", "title", "priority", "due", "tag", "enter save", "esc cancel"} {
		if !strings.Contains(view, want) {
			t.Errorf("form view missing %q:\n%s", want, view)
		}
	}
}

// The three ways in. The garage asks for nothing; the tray expects structure.
func TestAddToGarageAsksOnlyForTheWords(t *testing.T) {
	sandbox(t)
	m := keys(New(), "tab", "a").(Model) // garage tab
	if m.form == nil || !m.form.creating {
		t.Fatal("a should open a new entry")
	}
	if got := m.form.fields(); len(got) != 1 || got[0] != fTitle {
		t.Errorf("the garage should ask for the title alone, got %v", got)
	}

	m = keys(m, "n", "e", "w", " ", "l", "i", "n", "e").(Model)
	m = keys(m, "enter").(Model)
	if got := monthFile(t, "2026-08"); !strings.Contains(got, "- new line") {
		t.Errorf("garage did not receive it:\n%s", got)
	}
}

func TestAddToTrayOffersTheWholeForm(t *testing.T) {
	sandbox(t)
	m := keys(New(), "a").(Model) // tray tab
	if got := m.form.fields(); len(got) != 4 {
		t.Errorf("the tray should expect structure, got %v", got)
	}

	m = keys(m, "d", "o", " ", "i", "t").(Model)
	m.form.at = fPriority
	m = keys(m, "l", "l", "l").(Model) // none → H
	m = keys(m, "enter").(Model)

	got := trayFile(t)
	if !strings.Contains(got, "do it") || !strings.Contains(got, "priority:H") {
		t.Errorf("tray task not created with its priority:\n%s", got)
	}
	if !strings.Contains(got, "entry:2026-08-07") {
		t.Errorf("a new tray task should be dated:\n%s", got)
	}
}

func TestAddCreatesNothingWhenAbandoned(t *testing.T) {
	sandbox(t)
	keys(New(), "a", "x", "esc")
	if got := trayFile(t); strings.Contains(got, "x") && strings.Count(got, "- ") > 0 {
		t.Errorf("esc must create nothing:\n%s", got)
	}
	keys(New(), "a", "enter") // saved with an empty title
	lines, _ := store.Read(store.TrayPath())
	for _, line := range lines {
		if strings.HasPrefix(line, "- ") {
			t.Errorf("an empty title must not create a task: %q", line)
		}
	}
}

// The weekday is decoration on the way out. It must never end up in the buffer you
// are editing, or the next keystroke would append to it.
func TestWeekdayIsShownButNotEdited(t *testing.T) {
	sandbox(t, "- [ ] a thing due:2026-08-12")
	m := openRetake(t)
	m.form.at = fDue

	if !strings.Contains(m.View(), "2026-08-12 Wed") {
		t.Errorf("the weekday should be shown:\n%s", m.View())
	}
	if m.form.due != "2026-08-12" {
		t.Errorf("buffer = %q, want the stored date alone", m.form.due)
	}

	m = keys(m, "right").(Model) // a day later
	if m.form.due != "2026-08-13" {
		t.Errorf("buffer = %q after ←→", m.form.due)
	}
	keys(m, "enter")
	if got := trayFile(t); !strings.Contains(got, "due:2026-08-13") || strings.Contains(got, "Thu") {
		t.Errorf("the file must stay plain ISO:\n%s", got)
	}
}
