package core

import (
	"reflect"
	"testing"
)

func TestParseGarageProseIsVerbatim(t *testing.T) {
	// The jottpad has to hold a sentence. A colon mid-prose is not an attribute.
	raw := "- ?? that config thing — does it even matter: probably not"
	got, ok := Parse(raw, 0)
	if !ok {
		t.Fatal("a bullet must parse")
	}
	want := "?? that config thing — does it even matter: probably not"
	if got.Text != want {
		t.Errorf("text = %q, want %q", got.Text, want)
	}
	if len(got.Attrs) != 0 {
		t.Errorf("attrs = %v, want none", got.Attrs)
	}
}

func TestParseTrayLine(t *testing.T) {
	raw := "- [ ] Rotate the api keys priority:H due:2026-08-12 project:alpha entry:2026-08-07 +infra"
	got, _ := Parse(raw, 3)
	if got.Text != "Rotate the api keys" {
		t.Errorf("text = %q", got.Text)
	}
	if got.Attrs["priority"] != "H" || got.Attrs["due"] != "2026-08-12" {
		t.Errorf("attrs = %v", got.Attrs)
	}
	if !reflect.DeepEqual(got.Tags, []string{"infra"}) {
		t.Errorf("tags = %v", got.Tags)
	}
	if got.Done || got.Dropped || !got.Live() {
		t.Error("should be live")
	}
	if got.Index != 3 {
		t.Errorf("index = %d, want 3", got.Index)
	}
}

func TestRoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		checkbox bool
	}{
		{"tray open", "- [ ] Rotate the api keys priority:H due:2026-08-12 project:alpha", true},
		{"tray done", "- [x] ~~Renew the passport~~ priority:H done:2026-08-06", true},
		{"garage plain", "- add metrics to the worker +infra", false},
		{"garage moved", "- add retries to the sync job → 2026-09", false},
		{"garage taken", "- Fix alerts priority:H → tray", false},
		{"dropped", "- ~~the notes script~~ dropped:2026-08-07", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			parsed, ok := Parse(c.line, 0)
			if !ok {
				t.Fatalf("did not parse: %q", c.line)
			}
			if got := Line(parsed, c.checkbox); got != c.line {
				t.Errorf("round trip\n got %q\nwant %q", got, c.line)
			}
		})
	}
}

func TestTerminalStates(t *testing.T) {
	done, _ := Parse("- [x] ~~Passport~~ done:2026-08-06", 0)
	if !done.Done || done.Dropped {
		t.Error("done: want done, not dropped")
	}
	dropped, _ := Parse("- ~~Dead idea~~ dropped:2026-08-07", 0)
	if dropped.Done || !dropped.Dropped {
		t.Error("dropped: want dropped, not done")
	}
	if dropped.Live() {
		t.Error("a dropped task is not live")
	}
	// A strike with no date is still terminal — someone edited it by hand.
	struck, _ := Parse("- ~~gave up on this~~", 0)
	if !struck.Dropped {
		t.Error("a bare strike is abandoned")
	}
}

func TestProseIsNotATask(t *testing.T) {
	for _, raw := range []string{"## notes to self", "this is a paragraph", "", "   "} {
		if _, ok := Parse(raw, 0); ok {
			t.Errorf("%q must not parse as a task", raw)
		}
	}
	// A star bullet is a task, because Obsidian writes them.
	if _, ok := Parse("* a star bullet", 0); !ok {
		t.Error("star bullets are bullets")
	}
}

func TestSplitMods(t *testing.T) {
	got := SplitMods([]string{"Fix", "alerts", "pri:h", "due:2026-08-12", "+infra", "-old", "wat:xx"})
	if got.Attrs["priority"] != "H" {
		t.Errorf("pri alias + uppercase failed: %v", got.Attrs)
	}
	if got.Attrs["due"] != "2026-08-12" {
		t.Errorf("due = %v", got.Attrs)
	}
	if !reflect.DeepEqual(got.AddTags, []string{"infra"}) {
		t.Errorf("add = %v", got.AddTags)
	}
	if !reflect.DeepEqual(got.DelTags, []string{"old"}) {
		t.Errorf("del = %v", got.DelTags)
	}
	// An unknown key stays in the description rather than becoming an attribute.
	if !reflect.DeepEqual(got.Words, []string{"Fix", "alerts", "wat:xx"}) {
		t.Errorf("words = %v", got.Words)
	}
}

func TestApplyModsEmptyValueRemoves(t *testing.T) {
	task, _ := Parse("- [ ] Something priority:H +infra +old", 0)
	ApplyMods(&task, SplitMods([]string{"priority:", "-old", "+new"}))
	if _, still := task.Attrs["priority"]; still {
		t.Error("empty value must remove the attribute")
	}
	if !reflect.DeepEqual(task.Tags, []string{"infra", "new"}) {
		t.Errorf("tags = %v", task.Tags)
	}
}

func TestCopyIsDetached(t *testing.T) {
	src, _ := Parse("- Fix alerts priority:H +infra → tray", 0)
	fresh := src.Copy()
	fresh.Attrs["priority"] = "L"
	fresh.Tags = append(fresh.Tags, "extra")
	if src.Attrs["priority"] != "H" {
		t.Error("copy shares its attrs map with the source")
	}
	if len(src.Tags) != 1 {
		t.Error("copy shares its tag slice with the source")
	}
	if fresh.Moved != "" || fresh.Index != -1 {
		t.Error("a travelling copy carries no arrow and no line")
	}
}

// Handing a finished task back to the garage must arrive struck through, not open.
func TestCopyKeepsTerminalState(t *testing.T) {
	done, _ := Parse("- [x] ~~Renew the passport~~ priority:H done:2026-08-06", 0)
	if got := Line(done.Copy(), false); got != "- ~~Renew the passport~~ priority:H done:2026-08-06" {
		t.Errorf("done copy = %q", got)
	}
	dropped, _ := Parse("- ~~Dead idea~~ dropped:2026-08-07", 0)
	if !dropped.Copy().Dropped {
		t.Error("an abandoned copy is still abandoned")
	}
}
