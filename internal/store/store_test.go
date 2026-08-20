package store

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cheese-cracker/tray/internal/core"
)

func sandbox(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("TRAY_HOME", dir)
	t.Setenv("TRAY_TODAY", "2026-08-07")
	return dir
}

func TestMonthMath(t *testing.T) {
	cases := []struct{ in, next, prev string }{
		{"2026-08", "2026-09", "2026-07"},
		{"2026-12", "2027-01", "2026-11"},
		{"2026-01", "2026-02", "2025-12"},
	}
	for _, c := range cases {
		if got := NextMonth(c.in); got != c.next {
			t.Errorf("NextMonth(%s) = %s, want %s", c.in, got, c.next)
		}
		if got := PrevMonth(c.in); got != c.prev {
			t.Errorf("PrevMonth(%s) = %s, want %s", c.in, got, c.prev)
		}
	}
}

func TestTodayHonoursOverride(t *testing.T) {
	sandbox(t)
	if got := Today().Format(core.DateLayout); got != "2026-08-07" {
		t.Errorf("Today() = %s", got)
	}
	if got := ThisMonth(); got != "2026-08" {
		t.Errorf("ThisMonth() = %s", got)
	}
}

func TestPaths(t *testing.T) {
	dir := sandbox(t)
	if got := TrayPath(); got != filepath.Join(dir, "tray.md") {
		t.Errorf("TrayPath = %s", got)
	}
	if got := MonthPath(""); got != filepath.Join(dir, "2026-08.md") {
		t.Errorf("MonthPath(\"\") = %s", got)
	}
	if got := MonthPath(Someday); got != filepath.Join(dir, "someday.md") {
		t.Errorf("MonthPath(someday) = %s", got)
	}
}

func TestWriteIsAtomicAndLeavesNoTemp(t *testing.T) {
	dir := sandbox(t)
	path := filepath.Join(dir, "tray.md")
	if err := Write(path, []string{"# tray", "- [ ] a"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(path)
	if string(raw) != "# tray\n- [ ] a\n" {
		t.Errorf("content = %q", raw)
	}
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tray-") {
			t.Errorf("left a temp file behind: %s", e.Name())
		}
	}
}

// The contract that matters: we only own the lines we recognise.
func TestHandWrittenProseSurvives(t *testing.T) {
	sandbox(t)
	doc, err := Garage("2026-08")
	if err != nil {
		t.Fatal(err)
	}
	doc.Lines = []string{
		"# 2026-08",
		"",
		"- add metrics to the worker",
		"",
		"## notes to self",
		"this paragraph is mine and must survive",
		"* a star bullet with weird: punctuation",
		"- ",
	}
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}

	// A full edit cycle: add one, mark one terminal, drop one.
	doc, _ = Garage("2026-08")
	doc.Add(core.New("another line", nil))
	tasks := doc.Tasks()
	core.Finish(&tasks[0], "done", Today())
	doc.Set(tasks[0])
	if err := doc.Save(); err != nil {
		t.Fatal(err)
	}

	lines, _ := Read(MonthPath("2026-08"))
	body := strings.Join(lines, "\n")
	for _, must := range []string{
		"## notes to self",
		"this paragraph is mine and must survive",
		"* a star bullet with weird: punctuation",
		"- another line",
		"~~add metrics to the worker~~",
	} {
		if !strings.Contains(body, must) {
			t.Errorf("lost %q from:\n%s", must, body)
		}
	}
}

func TestRemoveOnlyDropsItsOwnLine(t *testing.T) {
	sandbox(t)
	doc, _ := Tray()
	doc.Lines = []string{"# tray", "- [ ] first", "keep me", "- [ ] second"}
	_ = doc.Save()

	doc, _ = Tray()
	tasks := doc.Tasks()
	doc.Remove(tasks[0])
	_ = doc.Save()

	lines, _ := Read(TrayPath())
	body := strings.Join(lines, "\n")
	if strings.Contains(body, "first") {
		t.Error("removed line is still there")
	}
	for _, must := range []string{"# tray", "keep me", "- [ ] second"} {
		if !strings.Contains(body, must) {
			t.Errorf("lost %q from:\n%s", must, body)
		}
	}
}

func TestResolveRanges(t *testing.T) {
	items := []core.Task{
		core.New("one", nil), core.New("two", nil), core.New("three", nil),
		core.New("four", nil), core.New("five", nil), core.New("six", nil),
		core.New("seven", nil),
	}
	cases := []struct {
		spec string
		want []string
	}{
		{"3", []string{"three"}},
		{"2,5-7", []string{"two", "five", "six", "seven"}},
		{"1-3", []string{"one", "two", "three"}},
		{"99", nil}, // out of range is ignored, not an error
		{"bogus", nil},
	}
	for _, c := range cases {
		got := Resolve(items, c.spec)
		if len(got) != len(c.want) {
			t.Errorf("Resolve(%q) gave %d items, want %d", c.spec, len(got), len(c.want))
			continue
		}
		for i, w := range c.want {
			if got[i].Text != w {
				t.Errorf("Resolve(%q)[%d] = %s, want %s", c.spec, i, got[i].Text, w)
			}
		}
	}
}

func TestEnsureCreatesWithHeader(t *testing.T) {
	sandbox(t)
	if _, err := Tray(); err != nil {
		t.Fatal(err)
	}
	lines, _ := Read(TrayPath())
	if len(lines) == 0 || lines[0] != TrayHeader {
		t.Errorf("lines = %v, want a header", lines)
	}
}

func TestGrepAcrossMonthsIsOrdered(t *testing.T) {
	sandbox(t)
	for _, month := range []string{"2026-05", "2026-06", "2026-07"} {
		doc, _ := Garage(month)
		doc.Add(core.New("add retries to the sync job", nil))
		_ = doc.Save()
	}
	hits := Grep("RETRIES") // case-insensitive
	if len(hits) != 3 {
		t.Fatalf("got %d hits, want 3: %v", len(hits), hits)
	}
	if hits[0].Where != "2026-05" || hits[2].Where != "2026-07" {
		t.Errorf("months out of order: %v", hits)
	}
	if got := MonthsWith(hits); got != 3 {
		t.Errorf("MonthsWith = %d, want 3", got)
	}
	if len(Grep("nothingmatchesthis")) != 0 {
		t.Error("a miss must return nothing")
	}
}

func TestMonthsListsOnlyMonthFiles(t *testing.T) {
	sandbox(t)
	for _, m := range []string{"2026-07", "2026-08"} {
		doc, _ := Garage(m)
		_ = doc.Save()
	}
	if _, err := Tray(); err != nil {
		t.Fatal(err)
	}
	doc, _ := Garage(Someday)
	_ = doc.Save()

	got := Months()
	if len(got) != 2 || got[0] != "2026-07" || got[1] != "2026-08" {
		t.Errorf("Months() = %v, want the two month files only", got)
	}
}
