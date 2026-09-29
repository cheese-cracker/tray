package gui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/plugin"
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
	home := t.TempDir()
	t.Setenv("TRAY_HOME", home) // plugins are found under the home, like the store
	// A sync runs in the background in the app; a flow wants the answer before its
	// next line, so the hand-off runs inline here.
	background = func(work, then func()) {
		work()
		then()
	}
	test.NewTempApp(t)
	s, err := store.Open(home)
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

// install copies a fixture plugin from the sync suite into this home's plugins folder.
func (h *harness) install(name string) string {
	h.t.Helper()
	dst := filepath.Join(store.Home(), "plugins", name)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		h.t.Fatal(err)
	}
	if err := os.CopyFS(dst, os.DirFS(filepath.Join("..", "sync", "testdata", "plugins", name))); err != nil {
		h.t.Fatal(err)
	}
	return dst
}

func ids(rows []core.Task) []int64 {
	out := make([]int64, len(rows))
	for i, t := range rows {
		out[i] = t.ID
	}
	return out
}

func TestFlowSweepOpensTheMonthsAndCarriesForward(t *testing.T) {
	old := garage("left in august")
	old.Month, old.Due = "2026-08", "2026-08-15"
	h := open(t, old, garage("this month"))
	h.u.openSweep()
	if h.u.mode != modeSweep {
		t.Fatal("the sweep is a mode of its own")
	}
	tabs := h.u.sw.tabs
	for i, want := range []string{"2026-08", "2026-09", "2026-10", "someday"} {
		if tabs.Items[i].Text != want {
			t.Fatalf("tab %d is %q, want %q", i, tabs.Items[i].Text, want)
		}
	}
	if tabs.SelectedIndex() != 1 {
		t.Fatal("the sweep opens on this month (73a)")
	}
	tabs.SelectIndex(0)
	l := h.u.current()
	if len(l.rows) != 1 || l.rows[0].ID != 1 {
		t.Fatalf("the august tab shows august: %v", ids(l.rows))
	}
	test.Type(l, ">")
	h.focused().TypedRune('4')
	if got := h.get(1); got.Month != store.Someday {
		t.Fatalf("> reaches every tab on screen (73d): %+v", got)
	}

	back := h.get(1)
	core.Move(&back, core.LayerGarage, "2026-08")
	if err := h.s.Put(&back); err != nil {
		t.Fatal(err)
	}
	h.u.carry("2026-08")
	if got := h.get(1); got.Month != "2026-09" || got.Due != "" {
		t.Fatalf("carry forward moves the named month on and drops a due that passed (75): %+v", got)
	}
	if !strings.Contains(h.u.status.Text, "1 2026-08 to 2026-09") {
		t.Fatalf("the status names what moved: %q", h.u.status.Text)
	}
	h.key(fyne.KeyEscape)
	if h.u.mode != modeHome || h.u.closed {
		t.Fatal("esc leaves the sweep and does not quit")
	}
}

func TestFlowReviewShowsEverythingAndOffersTheRareVerbs(t *testing.T) {
	done := tray("beta")
	done.Done = "2026-09-20"
	tmpl := tray("weekly review")
	tmpl.Recur, tmpl.Due = "weekly", "2026-10-02"
	h := open(t, done, tmpl, tray("alpha"))
	h.u.tabs.SelectIndex(1)
	if len(h.u.tray.rows) != 1 {
		t.Fatalf("home shows the live rows alone: %v", ids(h.u.tray.rows))
	}
	test.Type(h.u.tray, "v")
	if h.u.mode != modeReview {
		t.Fatal("v opens review")
	}
	l := h.u.current()
	if got := ids(l.rows); len(got) != 3 || got[0] != 3 || got[1] != 1 || got[2] != 2 {
		t.Fatalf("review is live first, then finished, then templates: %v", got)
	}
	test.Type(l, "a")
	if h.u.pop != nil || h.focused() != fyne.Focusable(l) {
		t.Fatal("a writes nothing in review (92e)")
	}
	test.Type(l, "x")
	if h.get(3).Done != "" {
		t.Fatal("done is not a review verb")
	}
	l.pick(1)
	test.Type(l, "R")
	if h.get(1).Done != "" {
		t.Fatal("R says it was not finished after all")
	}
	test.Type(h.u.current(), "v")
	if h.u.mode != modeHome {
		t.Fatal("v leaves review")
	}
}

func TestFlowEraseIsReachableOnlyInReview(t *testing.T) {
	h := open(t, tray("alpha"), tray("beta"))
	h.u.tabs.SelectIndex(1)
	test.Type(h.u.tray, "E")
	if _, ok, _ := h.s.Get(1); !ok {
		t.Fatal("E is dead outside review (93)")
	}
	test.Type(h.u.tray, "v")
	l := h.u.current()
	l.pick(1)
	test.Type(l, "E")
	if _, ok, _ := h.s.Get(2); ok {
		t.Fatal("E removes the row")
	}
	if _, ok, _ := h.s.Get(1); !ok {
		t.Fatal("its neighbour stays")
	}
	if !strings.Contains(h.u.status.Text, "erased: beta") {
		t.Fatalf("the status names what went: %q", h.u.status.Text)
	}
}

func TestFlowSyncReviewAppliesAPluginPlanWholeOrNotAtAll(t *testing.T) {
	h := open(t)
	dir := h.install("echo")
	test.Type(h.u.garage, "s")
	if h.u.mode != modeSyncReview || h.u.view == nil || len(h.u.view.cards) != 1 {
		t.Fatalf("s opens the review of what echo reported: mode %d", h.u.mode)
	}
	h.focused().TypedRune('y')
	rows, err := h.s.Tasks(store.Filter{Layer: core.LayerGarage, Month: "echo", All: true})
	if err != nil || len(rows) != 2 {
		t.Fatalf("echo's plan lands whole, in its garage: %d rows, %v", len(rows), err)
	}
	if rows[0].Source != "echo:n1" || rows[1].Done == "" {
		t.Fatalf("adds carry their key and what the plugin reported: %+v", rows)
	}
	if _, err := os.Stat(filepath.Join(dir, "applied.json")); err != nil {
		t.Fatal("the confirmed push should reach the plugin")
	}
	h.key(fyne.KeyEscape)
	if h.u.mode != modeHome {
		t.Fatal("esc leaves the review")
	}

	// A plan the store refuses lands nothing, not half.
	bad := `{"pull":[{"key":"n3","text":"fine"},{"key":"n4","text":"no such priority","priority":"Z"}],"push":[]}`
	if err := os.WriteFile(filepath.Join(dir, "plan.json"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	test.Type(h.u.garage, "s")
	h.focused().TypedRune('y')
	rows, _ = h.s.Tasks(store.Filter{Layer: core.LayerGarage, Month: "echo", All: true})
	if len(rows) != 2 {
		t.Fatalf("a refused plan must land nothing: %d rows", len(rows))
	}
	if state := h.u.view.cards[0].state.Text; !strings.HasPrefix(state, "not applied") {
		t.Fatalf("the card says why: %q", state)
	}
}

func TestFlowAFailedPluginFailsAlone(t *testing.T) {
	h := open(t)
	dir := h.install("fail")
	// Only a plugin that opted in runs at launch (T8); this one did, and breaks.
	if err := os.WriteFile(filepath.Join(dir, plugin.OnLaunchMarker), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	h.u.launch()
	if h.u.notice.Hidden || !strings.Contains(h.u.notice.Text, "1 failed") {
		t.Fatalf("the status line names the failure: %q hidden %v", h.u.notice.Text, h.u.notice.Hidden)
	}
	if h.u.mode != modeHome || h.u.pop != nil {
		t.Fatal("a failure at launch is a line, never a modal")
	}
	test.Type(h.u.capture, "still works")
	h.u.capture.TypedKey(&fyne.KeyEvent{Name: fyne.KeyReturn})
	if h.get(1).Text != "still works" {
		t.Fatal("the app still takes a dump")
	}
	test.Type(h.u.garage, "s")
	if h.u.mode != modeSyncReview || len(h.u.view.cards) != 1 || !h.u.view.cards[0].done {
		t.Fatal("s opens what waited, and a failed plan has nothing to apply")
	}
}

func TestFlowLaunchLiftsAWaitingRowWithoutAsking(t *testing.T) {
	w := garage("call mom")
	w.Wait, w.Priority = "2026-09-28", "H"
	h := open(t, w)
	if len(h.u.garage.rows) != 1 {
		t.Fatal("until sync, the row lies in the garage")
	}
	h.u.launch()
	got := h.get(1)
	if got.Layer != core.LayerTray || got.Wait != "" || got.Priority != "H" {
		t.Fatalf("its day came, and it kept what it carried: %+v", got)
	}
	if h.u.pop != nil || h.u.mode != modeHome || !h.u.notice.Hidden {
		t.Fatal("nothing prompts at launch")
	}
	if !strings.Contains(h.u.status.Text, "synced") {
		t.Fatalf("the status says when: %q", h.u.status.Text)
	}
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
