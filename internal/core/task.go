// Package core holds the rules — grammar, urgency, moves. It never touches a file.
package core

import (
	"regexp"
	"strings"
)

// KnownAttrs is also the order attributes serialise in.
var KnownAttrs = []string{"priority", "due", "project", "entry", "from", "done", "dropped"}

var aliases = map[string]string{"pri": "priority", "p": "priority", "proj": "project"}

var (
	bulletRe = regexp.MustCompile(`^(\s*)[-*]\s+(?:\[([ xX])\]\s+)?(.*)$`)
	attrRe   = regexp.MustCompile(`^([a-z]+):(\S*)$`)
	tagRe    = regexp.MustCompile(`^[+#](\w[\w/-]*)$`)
	movedRe  = regexp.MustCompile(`\s*→\s*(tray|\d{4}-\d{2})\s*$`)
)

type Task struct {
	Index   int // line index in the file it came from
	Raw     string
	Text    string
	Attrs   map[string]string
	Tags    []string
	Done    bool
	Dropped bool
	Moved   string // "tray", "2026-09", or empty
}

func New(text string, tags []string) Task {
	return Task{Index: -1, Text: text, Attrs: map[string]string{}, Tags: tags}
}

func (t Task) Terminal() bool { return t.Done || t.Dropped }
func (t Task) Live() bool     { return !t.Terminal() && t.Moved == "" }
func (t Task) Parsed() bool   { return t.Text != "" }
func (t Task) Priority() string {
	return strings.ToUpper(t.Attrs["priority"])
}

// Copy is what travels during a move. Terminal state comes along, so a done task
// handed back to the garage arrives struck through rather than looking open.
func (t Task) Copy() Task {
	attrs := make(map[string]string, len(t.Attrs))
	for k, v := range t.Attrs {
		attrs[k] = v
	}
	return Task{
		Index: -1, Text: t.Text, Attrs: attrs, Tags: append([]string{}, t.Tags...),
		Done: t.Done, Dropped: t.Dropped,
	}
}

func canonical(key string) string {
	if full, ok := aliases[key]; ok {
		return full
	}
	return key
}

func known(key string) bool {
	for _, k := range KnownAttrs {
		if k == key {
			return true
		}
	}
	return false
}

// Parse reads one line. A bullet is a task; anything else is prose we leave alone.
func Parse(raw string, index int) (Task, bool) {
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

	_, hasDone := attrs["done"]
	done := strings.EqualFold(box, "x") || hasDone
	_, hasDropped := attrs["dropped"]

	return Task{
		Index: index, Raw: raw, Text: text, Attrs: attrs, Tags: tags,
		Done: done, Dropped: hasDropped || (struck && !done), Moved: moved,
	}, true
}

// Tasks parses every bullet in a file, keeping each one's line index.
func Tasks(lines []string) []Task {
	var out []Task
	for i, raw := range lines {
		if t, ok := Parse(raw, i); ok {
			out = append(out, t)
		}
	}
	return out
}

// Line serialises a task back to markdown.
func Line(t Task, checkbox bool) string {
	body := t.Text
	if t.Terminal() {
		body = "~~" + t.Text + "~~"
	}
	parts := []string{body}
	for _, key := range KnownAttrs {
		if v := t.Attrs[key]; v != "" {
			parts = append(parts, key+":"+v)
		}
	}
	for _, g := range t.Tags {
		parts = append(parts, "+"+g)
	}

	box := ""
	if checkbox {
		box = "[ ] "
		if t.Done {
			box = "[x] "
		}
	}
	line := "- " + box + strings.Join(nonEmpty(parts), " ")
	if t.Moved != "" {
		line += " → " + t.Moved
	}
	return line
}

func nonEmpty(in []string) []string {
	out := in[:0:0]
	for _, s := range in {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
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
	if t.Attrs == nil {
		t.Attrs = map[string]string{}
	}
	for key, val := range mods.Attrs {
		if val == "" {
			delete(t.Attrs, key)
			continue
		}
		t.Attrs[key] = val
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
