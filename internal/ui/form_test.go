package ui

import (
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"regexp"
	"strings"
	"testing"

	"github.com/cheese-cracker/tray/internal/store"
)

func openRewrite(t *testing.T, presses ...string) Model {
	t.Helper()
	m := keys(New(), append([]string{"r"}, presses...)...).(Model)
	if m.form == nil {
		t.Fatal("r should open the form")
	}
	return m
}

func TestFormOpensPrefilled(t *testing.T) {
	sandbox(t, "- [ ] rotate the api keys priority:M due:2026-08-12 +infra")
	f := openRewrite(t).form

	if f.text(fTitle) != "rotate the api keys" {
		t.Errorf("title = %q", f.text(fTitle))
	}
	if f.prio != "M" || f.text(fDue) != "2026-08-12" || f.text(fTag) != "infra" {
		t.Errorf("prefill = %q %q %q", f.prio, f.text(fDue), f.text(fTag))
	}
	if len(f.touched) != 0 {
		t.Error("opening the form must not count as editing")
	}
}

func TestUntouchedFieldsAreLeftAlone(t *testing.T) {
	sandbox(t, "- [ ] rotate the api keys priority:M due:2026-08-12 +infra")
	keys(openRewrite(t), "enter") // save immediately, having changed nothing

	got := trayFile(t)
	for _, want := range []string{"rotate the api keys", "priority:M", "due:2026-08-12", "+infra"} {
		if !strings.Contains(got, want) {
			t.Errorf("lost %q:\n%s", want, got)
		}
	}
}

func TestCyclePriority(t *testing.T) {
	sandbox(t, "- [ ] a thing priority:M")
	m := openRewrite(t)
	m.form.at = fPriority

	m = keys(m, "h").(Model) // M → H, leftward, because the radio reads H · M · L
	if m.form.prio != "H" {
		t.Errorf("after h = %q, want H", m.form.prio)
	}
	m = keys(m, "l", "l").(Model) // H → M → L
	if m.form.prio != "L" {
		t.Errorf("after l l = %q, want L", m.form.prio)
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
	m := openRewrite(t)
	if m.form.prio != "M" {
		t.Errorf("an unset priority should read as M, got %q", m.form.prio)
	}

	m.form.at = fPriority
	m = keys(m, "l", "l", "l").(Model) // walk to the far end and stay there
	if m.form.prio != "L" {
		t.Errorf("l should clamp at L, got %q", m.form.prio)
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
	m := openRewrite(t)
	m = keys(m, "c", "d").(Model) // title field is first
	if m.form.text(fTitle) != "abcd" {
		t.Errorf("title = %q", m.form.text(fTitle))
	}
	m = keys(m, "backspace").(Model)
	if m.form.text(fTitle) != "abc" {
		t.Errorf("backspace failed: %q", m.form.text(fTitle))
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
	m := openRewrite(t, "h", "j", "k", "l")
	if m.form.text(fTitle) != "xhjkl" {
		t.Errorf("title = %q, want the letters typed", m.form.text(fTitle))
	}
}

func TestDueShiftsByADay(t *testing.T) {
	sandbox(t, "- [ ] a thing due:2026-08-12")
	m := openRewrite(t)
	m.form.at = fDue
	m = keys(m, "right").(Model)
	if got := m.form.text(fDue); got != "2026-08-13" {
		t.Errorf("due = %q, want 2026-08-13", got)
	}
	m = keys(m, "left", "left").(Model)
	if got := m.form.text(fDue); got != "2026-08-11" {
		t.Errorf("due = %q, want 2026-08-11", got)
	}
}

func TestEmptyDueShiftsFromToday(t *testing.T) {
	sandbox(t, "- [ ] a thing")
	m := openRewrite(t)
	m.form.at = fDue
	m = keys(m, "right").(Model)
	if got := m.form.text(fDue); got != "2026-08-07" {
		t.Errorf("due = %q, want today", got)
	}
}

// Tags are typed, and the tags already in use are shown as a hint rather than a
// menu — so a new one costs nothing but is still an act, not an accident.
func TestTagIsTyped(t *testing.T) {
	sandbox(t, "- [ ] a thing +infra", "- [ ] another +ops")
	m := openRewrite(t)
	m.form.at = fTag
	m.form.setText(fTag, "")

	m = keys(m, "b", "i", "l", "l", "i", "n", "g").(Model)
	if m.form.text(fTag) != "billing" {
		t.Errorf("tag = %q, want the letters typed", m.form.text(fTag))
	}
	m = keys(m, "backspace").(Model)
	if m.form.text(fTag) != "billin" {
		t.Errorf("backspace failed: %q", m.form.text(fTag))
	}

	if hint := m.View(); !strings.Contains(hint, "in use:") {
		t.Errorf("the tags already in use should be offered as a hint:\n%s", hint)
	}

	m.form.setText(fTag, "billing")
	keys(m, "enter")
	if got := trayFile(t); !strings.Contains(got, "+billing") {
		t.Errorf("typed tag not saved:\n%s", got)
	}
}

func TestEscCancelsEverything(t *testing.T) {
	sandbox(t, "- [ ] keep me priority:M")
	m := openRewrite(t, "z", "z", "z")
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
	m := openRewrite(t)
	m.form.at = fPriority
	m = keys(m, "h", "h").(Model)
	if m.form.prio != "H" {
		t.Errorf("h must not wrap H round to L, got %q", m.form.prio)
	}
}

func TestBatchSkipsTheTitle(t *testing.T) {
	sandbox(t, "- [ ] one priority:M", "- [ ] two priority:M")
	m := keys(New(), " ", "j", " ", "r").(Model) // mark both, rewrite
	if m.form == nil || !m.form.batch {
		t.Fatal("two marks should open a batch form")
	}
	for _, name := range m.form.fields() {
		if name == fTitle {
			t.Error("a batch form must not offer the title")
		}
	}
	m = keys(m, "h").(Model) // priority is the first field in a batch
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
	view := openRewrite(t).View()
	for _, want := range []string{"rewrite", "title", "priority", "due", "tag", "enter save", "esc cancel"} {
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
	m = keys(m, "h", "h", "h").(Model) // M → H, clamped
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
	m := openRewrite(t)
	m.form.at = fDue

	if !strings.Contains(m.View(), "Wed Aug 12") {
		t.Errorf("the readable date should be shown:\n%s", m.View())
	}
	if m.form.text(fDue) != "2026-08-12" {
		t.Errorf("buffer = %q, want the stored date alone", m.form.text(fDue))
	}

	m = keys(m, "right").(Model) // a day later
	if m.form.text(fDue) != "2026-08-13" {
		t.Errorf("buffer = %q after ←→", m.form.text(fDue))
	}
	keys(m, "enter")
	if got := trayFile(t); !strings.Contains(got, "due:2026-08-13") || strings.Contains(got, "Thu") {
		t.Errorf("the file must stay plain ISO:\n%s", got)
	}
}

// The radio reads H · M · L left to right. Whichever way the keys step, the dot has
// to move the way the key points — the two used to be separate literals in opposite
// orders, so l moved the dot left.
func TestPriorityStepsTheWayTheRadioReads(t *testing.T) {
	sandbox(t, "- [ ] a thing priority:M")
	m := openRewrite(t)
	m.form.at = fPriority

	dot := func(m Model) int { return strings.Index(m.View(), "(•)") }
	mid := dot(m)
	if mid < 0 {
		t.Fatalf("no radio drawn:\n%s", m.View())
	}
	for _, step := range []struct {
		key   string
		wants string
	}{{"l", "right"}, {"right", "right"}, {"h", "left"}, {"left", "left"}} {
		got := dot(keys(m, step.key).(Model))
		if (step.wants == "right") != (got > mid) {
			t.Errorf("%q should move the dot %s, %d -> %d:\n%s",
				step.key, step.wants, mid, got, keys(m, step.key).(Model).View())
		}
	}
}

// The point of textinput: a caret you can move, so a typo in the middle of a title is
// a fix rather than a retype. Hand-rolled editing only ever appended and backspaced.
func TestArrowsMoveTheCaretInTextFields(t *testing.T) {
	sandbox(t, "- [ ] hello world priority:M")
	m := openRewrite(t)

	// Left five puts the caret before "world"; typing lands there, not at the end.
	m = keys(m, "left", "left", "left", "left", "left").(Model)
	m = keys(m, "b", "i", "g", " ").(Model)
	if got := m.form.text(fTitle); got != "hello big world" {
		t.Errorf("title = %q, want the text inserted at the caret", got)
	}

	// And backspace deletes at the caret rather than off the end.
	m = keys(m, "backspace").(Model)
	if got := m.form.text(fTitle); got != "hello bigworld" {
		t.Errorf("title = %q, want the caret's character removed", got)
	}
}

// The arrows belong to the field they are in. On priority and due they change the
// value, which is why nothing claims them for the caret globally.
func TestArrowsStillPickAValueOnChoiceFields(t *testing.T) {
	sandbox(t, "- [ ] a thing priority:M due:2026-08-12")
	m := openRewrite(t)

	m.form.at = fPriority
	if m = keys(m, "left").(Model); m.form.prio != "H" {
		t.Errorf("priority = %q, want ← to pick", m.form.prio)
	}
	m.form.at = fDue
	if m = keys(m, "right").(Model); m.form.text(fDue) != "2026-08-13" {
		t.Errorf("due = %q, want → to shift a day", m.form.text(fDue))
	}
}

// The value on the live row must be one colour from end to end. textinput renders the
// text either side of the caret as two separate styled runs with a reset between them,
// so a colour wrapped around the whole row dies at the caret — which is how this looked
// half accent and half default until the style moved inside the input.
func TestTheLiveRowIsOneColourEitherSideOfTheCaret(t *testing.T) {
	was := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(was)

	sandbox(t, "- [ ] hello world priority:M")
	m := keys(openRewrite(t), "left", "left", "left").(Model)

	var line string
	plain := regexp.MustCompile(`\x1b\[[0-9;]*m`)
	for _, l := range strings.Split(m.View(), "\n") {
		if strings.Contains(plain.ReplaceAllString(l, ""), "hello world") {
			line = l
		}
	}
	if line == "" {
		t.Fatalf("no title row rendered:\n%s", m.View())
	}

	// Anchor on the caret — its run is the reversed one — and compare the runs either
	// side of it. Everything else on the line is border and padding.
	runs := regexp.MustCompile(`\x1b\[([0-9;]+)m([^\x1b]*)`).FindAllStringSubmatch(line, -1)
	caret := -1
	for i, r := range runs {
		if strings.Contains(";"+r[1]+";", ";7;") {
			caret = i
		}
	}
	if caret <= 0 || caret+1 >= len(runs) {
		t.Fatalf("no caret run found in %q", line)
	}
	before, after := runs[caret-1], runs[caret+1]
	if before[1] != after[1] {
		t.Errorf("text either side of the caret differs:\n  %q %q\n  %q %q",
			before[1], before[2], after[1], after[2])
	}
}
