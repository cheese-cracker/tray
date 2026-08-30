package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
)

// rewrite is one form with every field already at its current value, so only what
// you touch changes. No sequence, no question you must answer to reach another.

type field int

const (
	fTitle field = iota
	fPriority
	fDue
	fTag
)

var fieldNames = map[field]string{
	fTitle: "title", fPriority: "priority", fDue: "due", fTag: "tag",
}

// Left to right, exactly as the radio draws them. Stepping and drawing must read
// off the same slice: when they were two separate literals in opposite orders, h
// and l moved the dot the wrong way.
var priorities = []string{"H", "M", "L"}

const defaultPriority = "M"

type form struct {
	tasks    []core.Task
	month    string // which layer these came from; "" is the tray
	creating bool   // a new line rather than an edit
	at       field
	title    string
	prio     string
	due      string
	tag      string
	touched  map[field]bool
	vocab    []string
	batch    bool // several tasks: the title is skipped, one name for many is never the intent
	today    time.Time
}

func newForm(tasks []core.Task, month string, today time.Time) form {
	f := form{
		tasks: tasks, month: month, touched: map[field]bool{}, vocab: store.Tags(),
		batch: len(tasks) > 1, today: today,
	}
	first := tasks[0]
	f.title, f.prio, f.due = first.Text, first.Priority(), first.Attrs["due"]
	if f.prio == "" {
		f.prio = defaultPriority
	}
	if len(first.Tags) > 0 {
		f.tag = first.Tags[0]
	}
	f.at = fTitle
	if f.batch {
		f.at = fPriority
	}
	return f
}

// The garage asks for nothing but the words — that is the whole point of it. The
// tray is where structure is expected, so a new task there gets the full form.
func newEntry(month string, today time.Time) form {
	f := form{
		month: month, creating: true, touched: map[field]bool{},
		vocab: store.Tags(), today: today, at: fTitle, prio: defaultPriority,
	}
	return f
}

func (f form) fields() []field {
	// The garage asks for the words and nothing else — when a line is new, and just
	// as much when it is rewritten. A garage line *may* carry a priority, because one
	// handed back from the tray keeps what it was given; there is simply no way to
	// set one here. Wanting to is what `take` is for.
	if f.month != "" {
		return []field{fTitle}
	}
	if f.batch {
		return []field{fPriority, fDue, fTag}
	}
	return []field{fTitle, fPriority, fDue, fTag}
}

func (f *form) move(by int) {
	all := f.fields()
	for i, name := range all {
		if name == f.at {
			next := i + by
			if next >= 0 && next < len(all) {
				f.at = all[next]
			}
			return
		}
	}
}

// cycle is h/l. Enum fields step through their values; a date shifts by a day.
func (f *form) cycle(by int) {
	switch f.at {
	case fPriority:
		f.prio = clamp(priorities, f.prio, by) // an ordered scale clamps: l must never wrap L round to H
		f.touched[fPriority] = true
	case fDue:
		day, ok := core.Date(f.due)
		if !ok {
			day = f.today
		} else {
			day = day.AddDate(0, 0, by)
		}
		f.due = day.Format(core.DateLayout)
		f.touched[fDue] = true
	}
}

func at(options []string, current string) int {
	for i, o := range options {
		if o == current {
			return i
		}
	}
	return 0
}

func clamp(options []string, current string, by int) string {
	next := at(options, current) + by
	if next < 0 {
		next = 0
	}
	if next >= len(options) {
		next = len(options) - 1
	}
	return options[next]
}

// typing edits the free-text fields; enums are picked, never typed. A paste lands
// here too, and a pasted block can carry newlines and control characters — a task is
// one line of a markdown file, so those collapse rather than splitting it in two.
func (f *form) typed(runes []rune) {
	text := oneLine(runes)
	if text == "" {
		return
	}
	switch f.at {
	case fTitle:
		f.title += text
		f.touched[fTitle] = true
	case fDue:
		f.due += text
		f.touched[fDue] = true
	case fTag:
		f.tag += text
		f.touched[fTag] = true
	}
}

func oneLine(runes []rune) string {
	var b strings.Builder
	for _, r := range runes {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteRune(' ')
		case unicode.IsControl(r): // dropped: nothing sane to render for it
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func (f *form) backspace() {
	edit := func(s string) string {
		if s == "" {
			return s
		}
		r := []rune(s)
		return string(r[:len(r)-1])
	}
	switch f.at {
	case fTitle:
		f.title = edit(f.title)
		f.touched[fTitle] = true
	case fDue:
		f.due = edit(f.due)
		f.touched[fDue] = true
	case fTag:
		f.tag = edit(f.tag)
		f.touched[fTag] = true
	}
}

// apply writes only the fields that were touched, across every task in the form.
func (f form) apply() (string, error) {
	doc, err := layer{month: f.month}.open()
	if err != nil {
		return "", err
	}
	if f.creating {
		return f.create(doc)
	}
	for _, t := range f.tasks {
		if f.touched[fTitle] && !f.batch && strings.TrimSpace(f.title) != "" {
			t.Text = strings.TrimSpace(f.title)
		}
		if f.touched[fPriority] {
			set(&t, "priority", f.prio)
		}
		if f.touched[fDue] {
			set(&t, "due", strings.TrimSpace(f.due))
		}
		if f.touched[fTag] {
			if tag := strings.TrimSpace(f.tag); tag == "" {
				t.Tags = nil
			} else {
				t.Tags = []string{tag}
			}
		}
		doc.Set(t)
	}
	if err := doc.Save(); err != nil {
		return "", err
	}
	if len(f.touched) == 0 {
		return "unchanged", nil
	}
	return fmt.Sprintf("rewrote %d", len(f.tasks)), nil
}

func (f form) create(doc *store.Doc) (string, error) {
	title := strings.TrimSpace(f.title)
	if title == "" {
		return "", nil // nothing typed: the same as cancelling
	}
	task := core.New(title, nil)
	if f.month == "" {
		task.Attrs["entry"] = f.today.Format(core.DateLayout)
		priority := f.prio
		if priority == "" {
			priority = defaultPriority
		}
		set(&task, "priority", priority)
		set(&task, "due", strings.TrimSpace(f.due))
	}
	if tag := strings.TrimSpace(f.tag); tag != "" {
		task.Tags = []string{tag}
	}
	doc.Add(task)
	if err := doc.Save(); err != nil {
		return "", err
	}
	return "added: " + title, nil
}

func set(t *core.Task, key, value string) {
	if value == "" {
		delete(t.Attrs, key)
		return
	}
	t.Attrs[key] = value
}

func (f form) update(key tea.KeyMsg) (form, bool, bool) {
	switch key.Type {
	case tea.KeyEsc:
		return f, false, true // cancelled
	case tea.KeyEnter:
		return f, true, true // save
	case tea.KeyBackspace:
		f.backspace()
		return f, false, false
	case tea.KeyUp:
		f.move(-1)
		return f, false, false
	case tea.KeyDown:
		f.move(1)
		return f, false, false
	case tea.KeyLeft:
		f.cycle(-1)
		return f, false, false
	case tea.KeyRight:
		f.cycle(1)
		return f, false, false
	case tea.KeyTab:
		f.move(1)
		return f, false, false
	}

	// A paste — or a burst the terminal coalesced — is one message carrying many
	// runes. It can never be a vim key, so it is always text. The len == 1 gate below
	// used to be the only path, which dropped every paste on the floor.
	if key.Type == tea.KeyRunes && (key.Paste || len(key.Runes) > 1) {
		f.typed(key.Runes)
		return f, false, false
	}

	// On an enum field the vim keys navigate; on a text field they are just letters.
	if key.Type == tea.KeyRunes && len(key.Runes) == 1 {
		enum := f.at == fPriority
		switch {
		case enum && key.Runes[0] == 'j':
			f.move(1)
		case enum && key.Runes[0] == 'k':
			f.move(-1)
		case enum && key.Runes[0] == 'h':
			f.cycle(-1)
		case enum && key.Runes[0] == 'l':
			f.cycle(1)
		default:
			f.typed(key.Runes)
		}
		return f, false, false
	}
	if key.Type == tea.KeySpace {
		f.typed([]rune{' '})
	}
	return f, false, false
}

func (f form) view() string {
	var b strings.Builder
	title := "rewrite"
	switch {
	case f.creating && f.month != "":
		title = "dump — a line for later, nothing else needed"
	case f.creating:
		title = "add to the tray"
	case f.batch:
		title = fmt.Sprintf("rewrite %d tasks", len(f.tasks))
	}
	b.WriteString("\n  " + titleStyle.Render(title) + "\n\n")

	due := dashed(f.due)
	if day := core.Day(f.due); day != f.due {
		due = day
	}
	values := map[field]string{
		fTitle: f.title, fPriority: radio(f.prio), fDue: due, fTag: f.tag,
	}
	for _, name := range f.fields() {
		value := values[name]
		row := fmt.Sprintf("  %-9s %s", fieldNames[name], value)
		if name == f.at {
			row = cursorStyle.Render(row)
			if f.touched[name] {
				row += " " + faintStyle.Render("edited")
			}
		} else if f.touched[name] {
			row += " " + faintStyle.Render("edited")
		}
		b.WriteString(row + "\n")
	}

	hint := "type to edit"
	switch f.at {
	case fPriority:
		hint = "h l choose"
	case fDue:
		hint = "← → by a day · type a date"
	case fTag:
		hint = "type a tag"
		if len(f.vocab) > 0 {
			hint += " · in use: " + strings.Join(f.vocab, " ")
		}
	}
	b.WriteString("\n" + faintStyle.Render("  ↑↓ field · "+hint+" · enter save · esc cancel") + "\n")
	return b.String()
}

// radio spells the choice out rather than hiding two thirds of it behind a cycle.
func radio(current string) string {
	if current == "" {
		current = defaultPriority
	}
	var out []string
	for _, p := range priorities {
		dot := "( )"
		if p == current {
			dot = "(•)"
		}
		out = append(out, dot+" "+p)
	}
	return strings.Join(out, "  ")
}

func dashed(s string) string {
	if s == "" {
		return "none"
	}
	return s
}
