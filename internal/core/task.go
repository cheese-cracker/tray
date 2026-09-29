// Package core holds the rules — grammar, urgency, moves. It never touches a file or
// a table: the grammar here is for the wire (the CLI's mods, import and export), and
// the store keeps columns.
package core

import (
	"regexp"
	"strings"
	"time"
	"unicode"
)

const (
	LayerTray   = "tray"
	LayerGarage = "garage"
)

// KnownAttrs is the wire order: what follows a task's words on a line, and the keys
// the CLI accepts as key:value. Each is a column; `from` is spelled from_month there.
var KnownAttrs = []string{"priority", "due", "wait", "recur", "until", "entry", "from", "done"}

// Read, never written: a project is a tag here (9), so project: on the wire becomes
// one; and a file written by an older build may still carry dropped:, which reads as
// done. Both have to be recognised, or the key is swallowed into the words — an unknown
// key off the end of a line is just more sentence (17).
const (
	legacyDropped = "dropped"
	legacyProject = "project"
)

// TagMark is what a tag is written with on the wire. Taskwarrior's spelling, which is
// what 4 aligns with; both spellings are read (see tagRe).
const TagMark = "+"

var aliases = map[string]string{"pri": "priority", "p": "priority"}

var (
	bulletRe = regexp.MustCompile(`^(\s*)[-*]\s+(?:\[([ xX])\]\s+)?(.*)$`)
	attrRe   = regexp.MustCompile(`^([a-z]+):(\S*)$`)
	tagRe    = regexp.MustCompile(`^[+#](\w[\w/-]*)$`)
	movedRe  = regexp.MustCompile(`\s*→\s*(tray|\d{4}-\d{2})\s*$`)
)

type Task struct {
	ID    int64
	Layer string // LayerTray or LayerGarage
	Month string // garage only: 2026-09, someday, or the plugin whose garage it is
	Text  string

	Priority string // H, M or L; "" reads as M (32)
	Due      string // YYYY-MM-DD
	Wait     string // a garage row that surfaces onto the tray on this day
	Recur    string // a period; set, the row is a template rather than a task
	Until    string // when a template stops
	Entry    string // created
	Done     string // finished on; "" is live
	// FromMonth is the garage month a tray task was taken from, so handing it back
	// needs no destination. Cleared on the way home: it lives there again.
	FromMonth string

	Tags   []string
	Note   string
	Source string // <plugin>:<key>, recur:<id>, tw:<uuid> — where a row came from

	// Moved is read off a line's trailing `→ 2026-09` and never stored. To an importer
	// it says the line is history whose live copy went elsewhere.
	Moved string
}

func New(text string, tags []string) Task {
	return Task{Layer: LayerGarage, Text: text, Tags: tags}
}

func (t Task) Terminal() bool { return t.Done != "" }

// Live is work you can pick up: not finished, not a template, not a line that moved.
func (t Task) Live() bool { return !t.Terminal() && t.Recur == "" && t.Moved == "" }

// Waiting is a row whose day has not come.
func (t Task) Waiting(today time.Time) bool {
	d, ok := Date(t.Wait)
	return ok && d.After(today)
}

// Attr reads a field by its wire name.
func (t Task) Attr(key string) string {
	switch key {
	case "priority":
		return t.Priority
	case "due":
		return t.Due
	case "wait":
		return t.Wait
	case "recur":
		return t.Recur
	case "until":
		return t.Until
	case "entry":
		return t.Entry
	case "from":
		return t.FromMonth
	case "done":
		return t.Done
	case "month":
		return t.Month
	}
	return ""
}

// SetAttr writes a field by its wire name; an empty value clears it. Unknown keys
// are ignored, which is what lets `to:` ride along in Mods without becoming a field.
func (t *Task) SetAttr(key, value string) {
	switch key {
	case "priority":
		t.Priority = strings.ToUpper(value)
	case "due":
		t.Due = value
	case "wait":
		t.Wait = value
	case "recur":
		t.Recur = value
	case "until":
		t.Until = value
	case "entry":
		t.Entry = value
	case "from":
		t.FromMonth = value
	case "done":
		t.Done = value
	}
}

func canonical(key string) string {
	if full, ok := aliases[key]; ok {
		return full
	}
	return key
}

// known is wider than KnownAttrs, which is the write order: a retired key still has to
// be recognised on the way in or it becomes part of the words.
func known(key string) bool {
	if key == legacyDropped || key == legacyProject {
		return true
	}
	for _, k := range KnownAttrs {
		if k == key {
			return true
		}
	}
	return false
}

// Parse reads one markdown line. A bullet is a task; anything else is prose. A struck
// or ticked line is finished, and one that carries no date is dated today — that is
// what strikethrough means to someone reading the file with no tray in the loop.
func Parse(raw string, today time.Time) (Task, bool) {
	m := bulletRe.FindStringSubmatch(raw)
	if m == nil {
		return Task{}, false
	}
	box, body := m[2], m[3]

	var moved string
	if mv := movedRe.FindStringSubmatchIndex(body); mv != nil {
		moved = body[mv[2]:mv[3]]
		body = body[:mv[0]]
	}

	// Attributes and tags are read off the END only, so a colon mid-prose survives.
	tokens := strings.Fields(body)
	attrs := map[string]string{}
	var tags []string
	for len(tokens) > 0 {
		last := tokens[len(tokens)-1]
		if g := tagRe.FindStringSubmatch(last); g != nil {
			tags = append([]string{g[1]}, tags...)
			tokens = tokens[:len(tokens)-1]
			continue
		}
		g := attrRe.FindStringSubmatch(last)
		if g == nil {
			break
		}
		key := canonical(g[1])
		if !known(key) || g[2] == "" {
			break
		}
		if _, seen := attrs[key]; !seen {
			attrs[key] = g[2]
		}
		tokens = tokens[:len(tokens)-1]
	}

	text := strings.Join(tokens, " ")
	struck := len(text) > 4 && strings.HasPrefix(text, "~~") && strings.HasSuffix(text, "~~")
	if struck {
		text = strings.TrimSpace(text[2 : len(text)-2])
	}

	t := Task{Text: text, Tags: tags, Moved: moved}
	for key, value := range attrs {
		t.SetAttr(key, value)
	}
	if when, was := attrs[legacyDropped]; was && t.Done == "" {
		t.Done = when
	}
	if p, was := attrs[legacyProject]; was && !contains(t.Tags, p) {
		t.Tags = append(t.Tags, p)
	}
	if (strings.EqualFold(box, "x") || struck) && t.Done == "" {
		t.Done = today.Format(DateLayout)
	}
	return t, true
}

// Tasks parses every bullet in a document, each with the indented lines under it.
func Tasks(lines []string, today time.Time) []Task {
	var out []Task
	for i := 0; i < len(lines); {
		t, ok := Parse(lines[i], today)
		if !ok {
			i++
			continue
		}
		span := SpanAt(lines, i)
		var note []string
		for _, raw := range lines[i+1 : i+span] {
			note = append(note, strings.TrimSpace(raw))
		}
		t.Note = strings.Join(note, "\n")
		out = append(out, t)
		i += span
	}
	return out
}

// SpanAt is how many lines the task at i occupies: its bullet, then every following
// line that is indented and is not itself a bullet. A blank line ends it, which is
// the escape hatch for prose that sits under a task without belonging to it.
func SpanAt(lines []string, i int) int {
	n := 1
	for i+n < len(lines) && isNoteLine(lines[i+n]) {
		n++
	}
	return n
}

func isNoteLine(raw string) bool {
	if strings.TrimSpace(raw) == "" || !unicode.IsSpace(rune(raw[0])) {
		return false
	}
	return !bulletRe.MatchString(raw)
}

// Lines is Line and then the note, each note line under a two-space indent — the
// shape any markdown editor nests, and the one SpanAt reads back.
func Lines(t Task, checkbox bool) []string {
	out := []string{Line(t, checkbox)}
	if t.Note == "" {
		return out
	}
	for _, l := range strings.Split(t.Note, "\n") {
		out = append(out, "  "+strings.TrimSpace(l))
	}
	return out
}

// Line serialises a task to markdown.
func Line(t Task, checkbox bool) string {
	body := t.Text
	if t.Terminal() {
		body = "~~" + t.Text + "~~"
	}
	parts := []string{body}
	for _, key := range KnownAttrs {
		if v := t.Attr(key); v != "" {
			parts = append(parts, key+":"+v)
		}
	}
	for _, g := range t.Tags {
		parts = append(parts, TagMark+g)
	}

	box := ""
	if checkbox {
		box = "[ ] "
		if t.Terminal() {
			box = "[x] "
		}
	}
	line := "- " + box + strings.Join(parts, " ")
	if t.Moved != "" {
		line += " → " + t.Moved
	}
	return line
}

type Mods struct {
	Attrs   map[string]string
	AddTags []string
	DelTags []string
	Words   []string
}

// SplitMods pulls attributes, +tags and -tags out of a token list; the rest is description.
func SplitMods(tokens []string) Mods {
	mods := Mods{Attrs: map[string]string{}}
	for _, tok := range tokens {
		if strings.HasPrefix(tok, "-") && tagRe.MatchString("+"+tok[1:]) {
			mods.DelTags = append(mods.DelTags, tok[1:])
			continue
		}
		if g := tagRe.FindStringSubmatch(tok); g != nil {
			mods.AddTags = append(mods.AddTags, g[1])
			continue
		}
		if g := attrRe.FindStringSubmatch(tok); g != nil {
			key := canonical(g[1])
			if known(key) || key == "to" {
				mods.Attrs[key] = g[2]
				continue
			}
		}
		mods.Words = append(mods.Words, tok)
	}
	if p, ok := mods.Attrs["priority"]; ok {
		mods.Attrs["priority"] = strings.ToUpper(p)
	}
	return mods
}

// ApplyMods writes mods onto a task. An empty value removes the attribute.
func ApplyMods(t *Task, mods Mods) {
	for key, val := range mods.Attrs {
		if key == legacyProject {
			if val != "" && !contains(mods.AddTags, val) {
				mods.AddTags = append(mods.AddTags, val)
			}
			continue
		}
		t.SetAttr(key, val)
	}
	if len(mods.DelTags) > 0 {
		kept := t.Tags[:0:0]
		for _, g := range t.Tags {
			if !contains(mods.DelTags, g) {
				kept = append(kept, g)
			}
		}
		t.Tags = kept
	}
	for _, g := range mods.AddTags {
		if !contains(t.Tags, g) {
			t.Tags = append(t.Tags, g)
		}
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
