package gui

import (
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
)

// Every flow here drives the real app through the test driver and asserts on the
// model and the store, never on a frame (40, 63). Goldens live in screens_test.go.

type harness struct {
	t *testing.T
	u *ui
	s *store.Store
	w fyne.Window
}

func open(t *testing.T, seed ...core.Task) *harness {
	t.Helper()
	t.Setenv("TRAY_TODAY", "2026-09-28")
	test.NewTempApp(t)
	s, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	for i := range seed {
		if err := s.Put(&seed[i]); err != nil {
			t.Fatal(err)
		}
	}
	// Not NewTempWindow: its cleanup closes the window again after a flow that quit,
	// and the test driver does not survive a second close.
	w := test.NewWindow(widget.NewLabel(""))
	u := newUI(s, w)
	t.Cleanup(func() {
		if !u.closed {
			w.Close()
		}
	})
	w.SetContent(u.root)
	w.Resize(fyne.NewSize(960, 620))
	u.focusList()
	return &harness{t: t, u: u, s: s, w: w}
}

func garage(text string, tags ...string) core.Task {
	return core.Task{Layer: core.LayerGarage, Month: "2026-09", Text: text, Tags: tags}
}

func tray(text string) core.Task { return core.Task{Layer: core.LayerTray, Text: text} }

func (h *harness) get(id int64) core.Task {
	h.t.Helper()
	t, ok, err := h.s.Get(id)
	if err != nil || !ok {
		h.t.Fatalf("task %d: ok=%v err=%v", id, ok, err)
	}
	return t
}

func (h *harness) focused() fyne.Focusable { return h.w.Canvas().Focused() }

func (h *harness) key(name fyne.KeyName) {
	h.t.Helper()
	f := h.focused()
	if f == nil {
		h.t.Fatal("nothing focused")
	}
	f.TypedKey(&fyne.KeyEvent{Name: name})
}

func (h *harness) form() *form {
	h.t.Helper()
	if h.u.form == nil || h.u.pop == nil {
		h.t.Fatal("no form is open")
	}
	return h.u.form
}

func TestFlowTakeOpensTheFormAndSaves(t *testing.T) {
	h := open(t, garage("the billing page is slow", "work"))
	test.Type(h.u.garage, "t")
	f := h.form()
	test.Type(f.due, "2026-10-03")
	f.tags.SetText("work infra")
	f.pri.SetSelected("H")
	f.due.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})

	got := h.get(1)
	if got.Layer != core.LayerTray || got.FromMonth != "2026-09" || got.Month != "" {
		t.Fatalf("take did not move the row: %+v", got)
	}
	if got.Priority != "H" || got.Due != "2026-10-03" || strings.Join(got.Tags, " ") != "work infra" {
		t.Fatalf("the form's answers did not land: %+v", got)
	}
	if h.u.tabs.SelectedIndex() != 1 || h.u.pop != nil {
		t.Fatalf("after take the tray tab should be up and the form gone: tab %d pop %v", h.u.tabs.SelectedIndex(), h.u.pop)
	}
}

func TestFlowBatchRewriteSkipsTheTitle(t *testing.T) {
	a, b := tray("alpha"), tray("beta")
	a.Due = "2026-10-05"
	h := open(t, a, b)
	h.u.tabs.SelectIndex(1)
	test.Type(h.u.tray, " j ") // mark both
	test.Type(h.u.tray, "r")
	f := h.form()
	if len(f.box.Items) != 3 {
		t.Fatalf("a batch form should skip the title: %d items", len(f.box.Items))
	}
	f.pri.SetSelected("L")
	f.box.OnSubmit()

	for id, text := range map[int64]string{1: "alpha", 2: "beta"} {
		got := h.get(id)
		if got.Priority != "L" || got.Text != text {
			t.Fatalf("batch rewrite: %+v", got)
		}
	}
	if h.get(1).Due != "2026-10-05" {
		t.Fatal("an empty due in a batch should leave the date alone")
	}
}

func TestFlowFilterThenActOnAFilteredRow(t *testing.T) {
	h := open(t, tray("alpha"), tray("beta"))
	h.u.tabs.SelectIndex(1)
	test.Type(h.u.search, "beta")
	if len(h.u.tray.rows) != 1 || h.u.hidden.Text != "1 hidden" {
		t.Fatalf("filter: rows %d hidden %q", len(h.u.tray.rows), h.u.hidden.Text)
	}
	test.Type(h.u.tray, "x")
	if h.get(2).Done == "" || h.get(1).Done != "" {
		t.Fatal("done should hit the row the filter left, not the pre-filter cursor")
	}
}

func TestFlowMarksSurviveAFilter(t *testing.T) {
	h := open(t, tray("alpha"), tray("beta"))
	h.u.tabs.SelectIndex(1)
	test.Type(h.u.tray, " ")
	test.Type(h.u.search, "beta")
	test.Type(h.u.tray, " ")
	h.u.clearFilter()
	test.Type(h.u.tray, "x")
	if h.get(1).Done == "" || h.get(2).Done == "" {
		t.Fatal("both marks should have been acted on")
	}
}

func TestFlowTabsCycleBothWays(t *testing.T) {
	h := open(t, garage("a"), tray("b"))
	h.key(fyne.KeyTab)
	if h.u.tabs.SelectedIndex() != 1 {
		t.Fatal("tab should reach the tray")
	}
	h.key(fyne.KeyTab)
	if h.u.tabs.SelectedIndex() != 0 {
		t.Fatal("tab should cycle back to the garage rather than stopping")
	}
	if h.focused() != fyne.Focusable(h.u.garage) {
		t.Fatal("the shown list should hold the keys")
	}
}

func TestFlowMoveToAMonth(t *testing.T) {
	h := open(t, tray("alpha"))
	h.u.tabs.SelectIndex(1)
	test.Type(h.u.tray, ">")
	h.focused().TypedRune('3')
	got := h.get(1)
	if got.Layer != core.LayerGarage || got.Month != store.Someday {
		t.Fatalf("move to someday: %+v", got)
	}
}

func TestFlowHandBackKeepsWhatItLearned(t *testing.T) {
	taken := tray("the billing page is slow")
	taken.FromMonth, taken.Priority, taken.Due = "2026-09", "H", "2026-10-01"
	h := open(t, taken)
	h.u.tabs.SelectIndex(1)
	test.Type(h.u.tray, "d")
	got := h.get(1)
	if got.Layer != core.LayerGarage || got.Month != "2026-09" || got.FromMonth != "" {
		t.Fatalf("hand back should revive the garage row: %+v", got)
	}
	if got.Priority != "H" || got.Due != "2026-10-01" {
		t.Fatalf("hand back should keep what the tray added (88a): %+v", got)
	}
}

func TestFlowCaptureBarWritesWordsOnly(t *testing.T) {
	h := open(t)
	test.Type(h.u.capture, "to:2026-11 +infra fix the ?? thing: now")
	h.u.capture.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	got := h.get(1)
	if got.Layer != core.LayerGarage || got.Month != "2026-11" || strings.Join(got.Tags, " ") != "infra" {
		t.Fatalf("the front of the line is read as dump reads it: %+v", got)
	}
	if got.Text != "fix the ?? thing: now" || got.Priority != "" || got.Due != "" {
		t.Fatalf("the rest is literal and nothing else is asked: %+v", got)
	}
	if h.u.capture.Text != "" {
		t.Fatal("the bar should be empty for the next line")
	}
}

func TestFlowTrayAddTakesTheWholeForm(t *testing.T) {
	h := open(t)
	h.u.tabs.SelectIndex(1)
	test.Type(h.u.tray, "a")
	f := h.form()
	test.Type(f.title, "Renew the cert")
	test.Type(f.due, "2026-10-03")
	test.Type(f.tags, "infra")
	f.pri.SetSelected("H")
	f.box.OnSubmit()
	got := h.get(1)
	if got.Layer != core.LayerTray || got.Text != "Renew the cert" || got.Priority != "H" ||
		got.Due != "2026-10-03" || strings.Join(got.Tags, " ") != "infra" || got.Entry == "" {
		t.Fatalf("add should land every field: %+v", got)
	}
}

func TestFlowEscClearsTheFilterBeforeItQuits(t *testing.T) {
	h := open(t, tray("alpha"))
	h.u.tabs.SelectIndex(1)
	test.Type(h.u.search, "zzz")
	h.u.focusList()
	h.key(fyne.KeyEscape)
	if h.u.filter != "" || h.u.search.Text != "" || h.u.closed {
		t.Fatalf("first esc clears the filter and nothing else: filter %q closed %v", h.u.filter, h.u.closed)
	}
	h.key(fyne.KeyEscape)
	if !h.u.closed {
		t.Fatal("second esc quits")
	}
}

func TestFlowHelpPageToggles(t *testing.T) {
	h := open(t, tray("alpha"))
	h.u.tabs.SelectIndex(1)
	test.Type(h.u.tray, "?")
	if h.u.pop == nil {
		t.Fatal("? should open the help page")
	}
	if _, ok := h.focused().(*page); !ok {
		t.Fatalf("the page should hold the keys, not %T", h.focused())
	}
	h.focused().TypedRune('x')
	if h.u.pop != nil {
		t.Fatal("any key should close the page")
	}
	if h.get(1).Done != "" {
		t.Fatal("the key that closed the page must be spent doing so (86e)")
	}
	if h.focused() != fyne.Focusable(h.u.tray) {
		t.Fatal("the list should have the keys back")
	}
}

func TestFlowPasteIntoTheTitle(t *testing.T) {
	h := open(t)
	h.u.tabs.SelectIndex(1)
	test.Type(h.u.tray, "a")
	f := h.form()
	cb := h.w.Clipboard()
	cb.SetContent("first line\nsecond line")
	f.title.TypedShortcut(&fyne.ShortcutPaste{Clipboard: cb})
	f.box.OnSubmit()
	if got := h.get(1).Text; got != "first line second line" {
		t.Fatalf("a pasted newline collapses: %q", got)
	}
}

func TestFlowNoteIsTheLinesUnderATask(t *testing.T) {
	h := open(t, tray("the billing page is slow"))
	h.u.tabs.SelectIndex(1)
	test.Type(h.u.tray, "n")
	e, ok := h.focused().(*noteEntry)
	if !ok {
		t.Fatalf("n should open the note alone, focused: %T", h.focused())
	}
	test.Type(e, "only on the first load")
	e.TypedShortcut(&desktop.CustomShortcut{KeyName: fyne.KeyJ, Modifier: fyne.KeyModifierControl})
	test.Type(e, "the second is fine")
	e.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if got := h.get(1).Note; got != "only on the first load\nthe second is fine" {
		t.Fatalf("enter saves and ctrl+j breaks the line: %q", got)
	}
	if r := h.u.tray.items[0]; r == nil || !r.note.Visible() {
		t.Fatal("the row should show ≡ once a note exists")
	}
}

func TestFlowDetailsPaneHintsTheMissingRungs(t *testing.T) {
	h := open(t, tray("alpha"))
	h.u.tabs.SelectIndex(1)
	if h.u.details.hint.Text != "no priority · no due" {
		t.Fatalf("hint: %q", h.u.details.hint.Text)
	}
	test.Type(h.u.details.due, "2026-10-03")
	h.u.details.due.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if h.u.details.hint.Text != "no priority" {
		t.Fatalf("hint after a due: %q", h.u.details.hint.Text)
	}
	h.u.details.pri.SetSelected("H")
	if h.u.details.hint.Text != "" || h.get(1).Priority != "H" {
		t.Fatalf("hint after a priority: %q, %+v", h.u.details.hint.Text, h.get(1))
	}
}

func TestFlowShortcutsAreDeadWhileAnEntryHasFocus(t *testing.T) {
	h := open(t, tray("alpha"))
	h.u.tabs.SelectIndex(1)
	h.w.Canvas().Focus(h.u.search)
	h.focused().TypedRune('x')
	if h.get(1).Done != "" {
		t.Fatal("a letter typed into a field is text, not a verb")
	}
	if h.u.search.Text != "x" {
		t.Fatalf("the field should have taken it: %q", h.u.search.Text)
	}
}
