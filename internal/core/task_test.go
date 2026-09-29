package core

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseGarageProseIsVerbatim(t *testing.T) {
	// The jottpad has to hold a sentence. A colon mid-prose is not an attribute.
	raw := "- ?? the billing page feels slow — worth a look: probably"
	got, ok := Parse(raw, day("2026-08-07"))
	if !ok {
		t.Fatal("a bullet must parse")
	}
	want := "?? the billing page feels slow — worth a look: probably"
	if got.Text != want {
		t.Errorf("text = %q, want %q", got.Text, want)
	}
	if got.Priority != "" || got.Due != "" || got.Done != "" {
		t.Errorf("fields = %+v, want none set", got)
	}
}

func TestParseTrayLine(t *testing.T) {
	raw := "- [ ] Rotate the api keys priority:H due:2026-08-12 project:alpha entry:2026-08-07 +infra"
	got, _ := Parse(raw, day("2026-08-07"))
	if got.Text != "Rotate the api keys" {
		t.Errorf("text = %q — project: must be consumed, not kept as words (9)", got.Text)
	}
	if got.Priority != "H" || got.Due != "2026-08-12" || got.Entry != "2026-08-07" {
		t.Errorf("fields = %+v", got)
	}
	// A project is a tag (9): the one axis, so nothing another tool files under a
	// project is lost on the way in.
	if !reflect.DeepEqual(got.Tags, []string{"infra", "alpha"}) {
		t.Errorf("tags = %v, want project: read as a tag", got.Tags)
	}
	if got.Terminal() || !got.Live() {
		t.Error("should be live")
	}
}

func TestRoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		line     string
		checkbox bool
	}{
		{"tray open", "- [ ] Rotate the api keys priority:H due:2026-08-12", true},
		{"tray done", "- [x] ~~Renew the TLS certificate~~ priority:H done:2026-08-06", true},
		{"tray waiting", "- [ ] Call mom priority:H wait:2027-03-13", true},
		{"template", "- [ ] Weekly review priority:M due:2026-10-03 recur:weekly until:2027-01-01", true},
		{"garage plain", "- add metrics to the worker +infra", false},
		{"garage moved", "- add retries to the sync job → 2026-09", false},
		{"garage taken", "- Fix alerts priority:H → tray", false},
		{"garage done", "- ~~the notes script~~ done:2026-08-07", false},
		{"garage from", "- [ ] Fix alerts priority:H from:2026-06", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			parsed, ok := Parse(c.line, day("2026-08-07"))
			if !ok {
				t.Fatalf("did not parse: %q", c.line)
			}
			if got := Line(parsed, c.checkbox); got != c.line {
				t.Errorf("round trip\n got %q\nwant %q", got, c.line)
			}
		})
	}
}

// Both spellings are read; the wire is written Taskwarrior-style, because 4 aligns the
// field names with Taskwarrior and `tray export | task import` rests on that.
func TestATagIsReadEitherWayAndWrittenAsTaskwarrior(t *testing.T) {
	for _, line := range []string{
		"- add metrics to the worker +infra",
		"- add metrics to the worker #infra",
	} {
		parsed, ok := Parse(line, day("2026-08-07"))
		if !ok {
			t.Fatalf("did not parse: %q", line)
		}
		if len(parsed.Tags) != 1 || parsed.Tags[0] != "infra" {
			t.Errorf("%q gave tags %v", line, parsed.Tags)
		}
		if got := Line(parsed, false); got != "- add metrics to the worker +infra" {
			t.Errorf("%q wrote back as %q, want the wire spelling", line, got)
		}
	}
}

func TestTerminalStates(t *testing.T) {
	today := day("2026-08-07")
	done, _ := Parse("- [x] ~~Ship it~~ done:2026-08-06", today)
	if done.Done != "2026-08-06" || done.Live() {
		t.Errorf("done: want finished on the 6th and not live, got %+v", done)
	}
	// A strike with no date is still finished — someone edited it by hand, and
	// strikethrough is what that means to a reader with no tray in the loop. The
	// importer has no better date than today.
	struck, _ := Parse("- ~~gave up on this~~", today)
	if struck.Done != "2026-08-07" || struck.Live() {
		t.Errorf("a bare strike is finished today, got %+v", struck)
	}
	// `dropped:` was a third terminal state, removed. A file written by an older
	// build has to read as done rather than have the attribute swallowed into the
	// task's own text.
	legacy, _ := Parse("- ~~Dead idea~~ dropped:2026-08-07", today)
	if legacy.Done != "2026-08-07" {
		t.Errorf("a legacy dropped line should read as done, got %+v", legacy)
	}
	if legacy.Text != "Dead idea" {
		t.Errorf("text = %q, want the attribute consumed", legacy.Text)
	}
	if got := Line(legacy, false); got != "- ~~Dead idea~~ done:2026-08-07" {
		t.Errorf("it should converge to done: on the next write, got %q", got)
	}
	moved, _ := Parse("- Fix alerts → tray", today)
	if moved.Live() || moved.Moved != "tray" {
		t.Errorf("a moved line is history, not live: %+v", moved)
	}
}

func TestProseIsNotATask(t *testing.T) {
	for _, raw := range []string{"## notes to self", "this is a paragraph", "", "   "} {
		if _, ok := Parse(raw, day("2026-08-07")); ok {
			t.Errorf("%q must not parse as a task", raw)
		}
	}
	// A star bullet is a task, because Obsidian writes them.
	if _, ok := Parse("* a star bullet", day("2026-08-07")); !ok {
		t.Error("star bullets are bullets")
	}
}

func TestSplitMods(t *testing.T) {
	got := SplitMods([]string{"Fix", "alerts", "pri:h", "due:2026-08-12", "wait:2026-09-01", "+infra", "-old", "wat:xx"})
	if got.Attrs["priority"] != "H" {
		t.Errorf("pri alias + uppercase failed: %v", got.Attrs)
	}
	if got.Attrs["due"] != "2026-08-12" || got.Attrs["wait"] != "2026-09-01" {
		t.Errorf("attrs = %v", got.Attrs)
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
	task, _ := Parse("- [ ] Something priority:H +infra +old", day("2026-08-07"))
	ApplyMods(&task, SplitMods([]string{"priority:", "-old", "+new", "to:2026-09"}))
	if task.Priority != "" {
		t.Error("empty value must remove the attribute")
	}
	if !reflect.DeepEqual(task.Tags, []string{"infra", "new"}) {
		t.Errorf("tags = %v", task.Tags)
	}
	if task.Month != "" {
		t.Error("to: is the CLI's, not a field")
	}
}

// A note is the indented lines under a bullet. One note, any length; a blank line ends
// it, and an indented bullet is still a task rather than part of the note above.
func TestANoteIsTheIndentedLinesUnderATask(t *testing.T) {
	lines := []string{
		"# tray",
		"- [ ] Rotate the api keys priority:H +infra",
		"  The old keys expire on the 12th.",
		"  Rotate staging first.",
		"- [ ] Book the flight priority:M",
		"  - an indented bullet is a task, not a note",
		"",
		"  prose after a blank line belongs to nobody",
		"- [ ] no note at all",
	}
	tasks := Tasks(lines, day("2026-08-07"))
	if len(tasks) != 4 {
		t.Fatalf("parsed %d tasks, want 4: %+v", len(tasks), tasks)
	}
	if got := tasks[0].Note; got != "The old keys expire on the 12th.\nRotate staging first." {
		t.Errorf("note = %q", got)
	}
	if tasks[1].Note != "" {
		t.Errorf("an indented bullet must not become a note: %+v", tasks[1])
	}
	if tasks[2].Text != "an indented bullet is a task, not a note" {
		t.Errorf("the indented bullet should parse as its own task, got %q", tasks[2].Text)
	}
	if tasks[3].Note != "" {
		t.Errorf("prose after a blank line is not a note: %q", tasks[3].Note)
	}

	// And it writes back the way it was read.
	got := Lines(tasks[0], true)
	want := []string{
		"- [ ] Rotate the api keys priority:H +infra",
		"  The old keys expire on the 12th.",
		"  Rotate staging first.",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("Lines =\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}
