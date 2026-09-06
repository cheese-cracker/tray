package ui

import (
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/cursor"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

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

// Every text field is one of these, built the same way, so `title`, `due` and `tag`
// cannot drift into three slightly different editors. The label column is drawn by the
// form, so the input contributes no prompt of its own.
func newInput(value string) textinput.Model {
	in := textinput.New()
	in.Prompt = ""
	in.SetValue(value)
	in.CursorEnd()
	in.Width = inputWidth
	brand(&in)
	return in
}

// brand repaints an input in the palette. bubbles ships its own colours — a pink caret
// and a grey placeholder that are nobody's brand — and every input in the app goes
// through here so none of them keeps them. The list's filter input included, which is
// not one of ours to construct.
func brand(in *textinput.Model) {
	in.TextStyle = lipgloss.NewStyle()
	in.Cursor.Style = cursorStyle
	in.PlaceholderStyle = faintStyle
	in.PromptStyle = keyStyle
}

// Wide enough for a real task title, narrow enough to stay inside the pane on an
// eighty-column terminal. textinput scrolls within it rather than overflowing.
const inputWidth = 46

// liveValue is what the field you are on is drawn in — the same weight the table gives
// the row under its cursor, so the form and the list agree about what "here" looks like.
//
// It has to be the input's own TextStyle rather than a style wrapped around the row.
// textinput renders the text before the caret and the text after it as two separate
// Renders with the caret's escape between them, and that escape carries a reset — so an
// outer colour dies at the caret and the value comes out half one colour, half another.
var liveValue = titleStyle

// text is what a field currently holds. Enums are not in `inputs` and read as "".
func (f form) text(name field) string { return f.inputs[name].Value() }

// setText writes a field and marks it touched, which is what makes the form only ever
// write back what you actually changed.
func (f *form) setText(name field, value string) {
	in := f.inputs[name]
	in.SetValue(value)
	in.CursorEnd()
	f.inputs[name] = in
	f.touched[name] = true
}

// focus puts the caret in the field the cursor is on and takes it out of every other,
// so exactly one input is live at a time.
func (f *form) focus() {
	for name, in := range f.inputs {
		if name == f.at {
			in.Focus()
			// After Focus, or SetMode reads the field as unfocused and hides the
			// caret. Static rather than blinking: the form drops every tea.Cmd, so a
			// blink would never be scheduled — this says so instead of relying on it.
			in.Cursor.SetMode(cursor.CursorStatic)
			in.TextStyle = liveValue
		} else {
			in.Blur()
			in.TextStyle = lipgloss.NewStyle()
		}
		f.inputs[name] = in
	}
}

type form struct {
	tasks    []core.Task
	month    string // which layer these came from; "" is the tray
	creating bool   // a new line rather than an edit
	at       field
	inputs   map[field]textinput.Model // every free-text field; enums are not typed into
	prio     string
	touched  map[field]bool
	vocab    []string
	batch    bool    // several tasks: the title is skipped, one name for many is never the intent
	only     []field // when set, the whole form is these fields — see newTagger
	today    time.Time
}

func newForm(tasks []core.Task, month string, today time.Time) form {
	f := form{
		tasks: tasks, month: month, touched: map[field]bool{}, vocab: store.Tags(),
		batch: len(tasks) > 1, today: today,
	}
	first := tasks[0]
	f.prio = first.Priority()
	if f.prio == "" {
		f.prio = defaultPriority
	}
	// Every tag, space separated. It held `Tags[0]` for a long time, which meant
	// rewriting a two-tag task silently dropped one — the grammar was never the limit,
	// the form was.
	f.inputs = map[field]textinput.Model{
		fTitle: newInput(first.Text),
		fDue:   newInput(first.Attrs["due"]),
		fTag:   newInput(strings.Join(first.Tags, " ")),
	}
	f.at = fTitle
	if f.batch {
		f.at = fPriority
	}
	f.focus()
	return f
}

// The garage asks for nothing but the words — that is the whole point of it. The
// tray is where structure is expected, so a new task there gets the full form.
func newEntry(month string, today time.Time) form {
	f := form{
		month: month, creating: true, touched: map[field]bool{},
		vocab: store.Tags(), today: today, at: fTitle, prio: defaultPriority,
		inputs: map[field]textinput.Model{
			fTitle: newInput(""), fDue: newInput(""), fTag: newInput(""),
		},
	}
	f.focus()
	return f
}

// tagging opens the same form showing nothing but the tag field. `+` is meant to be a
// keystroke, not a form, so it does not ask about anything it was not asked about.
func newTagger(tasks []core.Task, month string, today time.Time) form {
	f := newForm(tasks, month, today)
	f.only, f.at = []field{fTag}, fTag
	f.focus()
	return f
}

func (f form) fields() []field {
	if len(f.only) > 0 {
		return f.only
	}
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
			break
		}
	}
	f.focus() // exactly one input is live, and it is the one you are on
}

// cycle is h/l. Enum fields step through their values; a date shifts by a day.
func (f *form) cycle(by int) {
	switch f.at {
	case fPriority:
		f.prio = clamp(priorities, f.prio, by) // an ordered scale clamps: l must never wrap L round to H
		f.touched[fPriority] = true
	case fDue:
		day, ok := core.Date(f.text(fDue))
		if !ok {
			day = f.today
		} else {
			day = day.AddDate(0, 0, by)
		}
		f.setText(fDue, day.Format(core.DateLayout))
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

// edit hands the keystroke to whichever input has the caret. textinput owns insertion,
// deletion, and the arrow keys inside a line; the form owns only which field is live.
//
// A paste can carry newlines, and a task is one line of a markdown file — so the value
// is flattened afterwards rather than trusting what arrived.
func (f *form) edit(msg tea.KeyMsg) {
	f.focus() // `at` is the truth; focus follows it rather than the other way round
	in, ok := f.inputs[f.at]
	if !ok {
		return
	}
	// A space reaches textinput as runes. Some senders set only the type, and an
	// empty-runed message would insert nothing at all.
	if msg.Type == tea.KeySpace && len(msg.Runes) == 0 {
		msg.Runes = []rune{' '}
	}
	before := in.Value()
	in, _ = in.Update(msg)
	if flat := oneLine([]rune(in.Value())); flat != in.Value() {
		in.SetValue(flat)
	}
	f.inputs[f.at] = in
	if in.Value() != before {
		f.touched[f.at] = true
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
		if f.touched[fTitle] && !f.batch && strings.TrimSpace(f.text(fTitle)) != "" {
			t.Text = strings.TrimSpace(f.text(fTitle))
		}
		if f.touched[fPriority] {
			set(&t, "priority", f.prio)
		}
		if f.touched[fDue] {
			set(&t, "due", strings.TrimSpace(f.text(fDue)))
		}
		if f.touched[fTag] {
			t.Tags = strings.Fields(f.text(fTag)) // nil when empty, which clears them
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
	title := strings.TrimSpace(f.text(fTitle))
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
		set(&task, "due", strings.TrimSpace(f.text(fDue)))
	}
	task.Tags = strings.Fields(f.text(fTag))
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
	case tea.KeyUp:
		f.move(-1)
		return f, false, false
	case tea.KeyDown, tea.KeyTab:
		f.move(1)
		return f, false, false
	}

	// ←/→ do whatever the field they are in is for. On the two fields that hold a
	// choice they change it; on a text field they move the caret, which is textinput's
	// job and is why nothing here claims them globally. The hint line says which.
	if key.Type == tea.KeyLeft || key.Type == tea.KeyRight {
		by := 1
		if key.Type == tea.KeyLeft {
			by = -1
		}
		if f.at == fPriority || f.at == fDue {
			f.cycle(by)
			return f, false, false
		}
	}

	// On an enum field the vim keys navigate. There is no input to type into there,
	// so they cannot be letters; everywhere else they are.
	if f.at == fPriority && key.Type == tea.KeyRunes && len(key.Runes) == 1 {
		switch key.Runes[0] {
		case 'j':
			f.move(1)
		case 'k':
			f.move(-1)
		case 'h':
			f.cycle(-1)
		case 'l':
			f.cycle(1)
		}
		return f, false, false
	}

	f.edit(key)
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

	for _, name := range f.fields() {
		label := fmt.Sprintf("%-9s", fieldNames[name])
		if name == f.at {
			label = cursorStyle.Render(label)
		}
		row := "  " + label + " " + f.value(name)
		if f.touched[name] {
			row += " " + faintStyle.Render("edited")
		}
		b.WriteString(row + "\n")
	}

	hint := "← → move · type to edit"
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

// value is what a row shows. A live field draws its input, caret and all; a quiet one
// draws its text, and `due` takes the chance to read as a date rather than an ISO stamp.
func (f form) value(name field) string {
	if name == fPriority {
		return paint(name == f.at, radio(f.prio))
	}
	live := name == f.at
	// `due` is the one text field whose arrows are spent on the value rather than the
	// caret, so a caret there would not move. It reads as a date instead, live or not.
	if name == fDue {
		return paint(live, dashed(core.Day(f.text(fDue))))
	}
	if live {
		return f.inputs[name].View() // carries its own colour; see liveValue
	}
	return dashed(f.text(name))
}

// paint gives a plain value the same weight an input gives its own text, so a row does
// not change colour depending on whether it happens to be typed into.
func paint(live bool, s string) string {
	if live {
		return liveValue.Render(s)
	}
	return s
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
