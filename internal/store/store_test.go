package store

import (
	"errors"
	"reflect"
	"testing"

	"github.com/cheese-cracker/tray/internal/core"
)

func sandbox(t *testing.T) *Store {
	t.Helper()
	t.Setenv("TRAY_TODAY", "2026-08-07")
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func put(t *testing.T, s *Store, task core.Task) core.Task {
	t.Helper()
	if err := s.Put(&task); err != nil {
		t.Fatal(err)
	}
	return task
}

func texts(tasks []core.Task) []string {
	var out []string
	for _, t := range tasks {
		out = append(out, t.Text)
	}
	return out
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
	for name, want := range map[string]bool{"2026-08": true, Someday: false, "notion": false} {
		if IsMonth(name) != want {
			t.Errorf("IsMonth(%s) = %v", name, !want)
		}
	}
}

func TestTodayHonoursOverride(t *testing.T) {
	t.Setenv("TRAY_TODAY", "2026-08-07")
	if got := Today().Format(core.DateLayout); got != "2026-08-07" {
		t.Errorf("Today() = %s", got)
	}
	if got := ThisMonth(); got != "2026-08" {
		t.Errorf("ThisMonth() = %s", got)
	}
}

// The home is hidden, by 53's own rule: nobody is meant to open a sqlite file.
func TestDefaultHomeIsTheXDGDataDirectory(t *testing.T) {
	t.Setenv("HOME", "/tmp/somewhere")
	t.Setenv("TRAY_HOME", "")
	t.Setenv("XDG_DATA_HOME", "")
	if got, want := Home(), "/tmp/somewhere/.local/share/tray"; got != want {
		t.Errorf("Home() = %s, want %s", got, want)
	}
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg")
	if got := Home(); got != "/tmp/xdg/tray" {
		t.Errorf("XDG_DATA_HOME should be honoured, got %s", got)
	}
	t.Setenv("TRAY_HOME", "/tmp/elsewhere")
	if got := Home(); got != "/tmp/elsewhere" {
		t.Errorf("TRAY_HOME should still win, got %s", got)
	}
}

// An erased id is never reused: an agent holding it across runs must not find another
// task behind it.
func TestIdsArePermanent(t *testing.T) {
	s := sandbox(t)
	a := put(t, s, core.New("a", nil))
	b := put(t, s, core.New("b", nil))
	if a.ID == 0 || b.ID <= a.ID {
		t.Fatalf("ids = %d, %d — want climbing from 1", a.ID, b.ID)
	}
	if err := s.Delete(b.ID); err != nil {
		t.Fatal(err)
	}
	c := put(t, s, core.New("c", nil))
	if c.ID <= b.ID {
		t.Errorf("c took %d, which b (%d) had", c.ID, b.ID)
	}
	if _, ok, _ := s.Get(b.ID); ok {
		t.Error("an erased row is still there")
	}
}

func TestPutRoundTripsEveryColumn(t *testing.T) {
	s := sandbox(t)
	want := core.Task{
		Layer: core.LayerTray, Text: "Rotate the keys", Priority: "H", Due: "2026-08-12",
		Wait: "2026-08-10", Recur: "weekly", Until: "2027-01-01", Entry: "2026-08-01",
		Done: "2026-08-07", FromMonth: "2026-07", Tags: []string{"infra", "work"},
		Note: "two\nlines", Source: "tw:abc",
	}
	want = put(t, s, want)
	got, ok, err := s.Get(want.ID)
	if err != nil || !ok {
		t.Fatalf("Get = %v, %v", ok, err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}

	// An update rewrites every column, and a cleared field reads back as "".
	want.Priority, want.Tags, want.Done = "", nil, ""
	put(t, s, want)
	got, _, _ = s.Get(want.ID)
	if got.Priority != "" || len(got.Tags) != 0 || got.Done != "" {
		t.Errorf("cleared fields came back: %+v", got)
	}
}

func TestPutStampsEntry(t *testing.T) {
	s := sandbox(t)
	got := put(t, s, core.New("fresh", nil))
	if got.Entry != "2026-08-07" {
		t.Errorf("entry = %q, want today", got.Entry)
	}
}

func TestTasksFilters(t *testing.T) {
	s := sandbox(t)
	live := put(t, s, core.Task{Layer: core.LayerTray, Text: "Urgent thing", Priority: "H", Due: "2026-08-08", Tags: []string{"infra"}})
	put(t, s, core.Task{Layer: core.LayerTray, Text: "Finished thing", Done: "2026-08-06"})
	put(t, s, core.Task{Layer: core.LayerTray, Text: "Weekly review", Recur: "weekly", Due: "2026-08-14"})
	put(t, s, core.Task{Layer: core.LayerGarage, Month: "2026-08", Text: "a jotting", Tags: []string{"Infra"}})
	put(t, s, core.Task{Layer: core.LayerGarage, Month: "2026-09", Text: "later"})

	cases := []struct {
		name string
		f    Filter
		want []string
	}{
		{"live tray hides finished and templates", Filter{Layer: core.LayerTray}, []string{"Urgent thing"}},
		{"all tray", Filter{Layer: core.LayerTray, All: true}, []string{"Urgent thing", "Finished thing", "Weekly review"}},
		{"a garage month", Filter{Layer: core.LayerGarage, Month: "2026-08"}, []string{"a jotting"}},
		{"tag, exact", Filter{Tags: []string{"infra"}}, []string{"Urgent thing"}},
		{"attr, case-insensitive", Filter{Attrs: map[string]string{"priority": "h"}}, []string{"Urgent thing"}},
		{"attr on a field nothing has", Filter{Attrs: map[string]string{"due": "2027-01-01"}}, nil},
		{"text over words", Filter{Text: "THING"}, []string{"Urgent thing"}},
		{"text over tags", Filter{Text: "inf", All: true}, []string{"Urgent thing", "a jotting"}},
		{"ids reach every state", Filter{IDs: []int64{live.ID, live.ID + 1}, All: true}, []string{"Urgent thing", "Finished thing"}},
	}
	for _, c := range cases {
		got, err := s.Tasks(c.f)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(texts(got), c.want) {
			t.Errorf("%s: got %v, want %v", c.name, texts(got), c.want)
		}
	}
}

func TestUpdateIsAllOrNothing(t *testing.T) {
	s := sandbox(t)
	boom := errors.New("boom")
	err := s.Update(func(tx *Store) error {
		if err := tx.Put(&core.Task{Layer: core.LayerTray, Text: "half"}); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("Update swallowed the error: %v", err)
	}
	got, _ := s.Tasks(Filter{All: true})
	if len(got) != 0 {
		t.Errorf("a rolled-back row landed: %v", texts(got))
	}
}

func TestMonthsAreCalendarMonthsOnly(t *testing.T) {
	s := sandbox(t)
	for _, m := range []string{"2026-08", Someday, "2026-07", "notion"} {
		put(t, s, core.Task{Layer: core.LayerGarage, Month: m, Text: "in " + m})
	}
	put(t, s, core.Task{Layer: core.LayerTray, Text: "on the tray"})
	got, err := s.Months()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, []string{"2026-07", "2026-08"}) {
		t.Errorf("Months() = %v", got)
	}
}

func TestParseIDs(t *testing.T) {
	cases := []struct {
		spec string
		want []int64
	}{
		{"3", []int64{3}},
		{"2,5-7", []int64{2, 5, 6, 7}},
		{"1-3", []int64{1, 2, 3}},
		{"7-5", nil}, // a backwards range is nothing, not a crash
		{"bogus", nil},
	}
	for _, c := range cases {
		if got := ParseIDs(c.spec); !reflect.DeepEqual(got, c.want) {
			t.Errorf("ParseIDs(%q) = %v, want %v", c.spec, got, c.want)
		}
	}
}
