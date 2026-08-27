package cli

import (
	"os"
	"strconv"

	"encoding/json"
	"fmt"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"

	"github.com/cheese-cracker/tray/internal/style"
	"sort"
	"strings"
	"time"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
)

const untagged = "untagged"

func table(rows [][]string, headers []string) string {
	widths := make([]int, len(headers))
	for i, h := range headers {
		widths[i] = len(h)
	}
	for _, row := range rows {
		for i, cell := range row {
			if w := len([]rune(cell)); w > widths[i] {
				widths[i] = w
			}
		}
	}
	pad := func(cells []string) string {
		var b strings.Builder
		for i, cell := range cells {
			b.WriteString(cell)
			if i < len(cells)-1 {
				b.WriteString(strings.Repeat(" ", widths[i]-len([]rune(cell))+2))
			}
		}
		return strings.TrimRight(b.String(), " ")
	}

	out := []string{pad(headers)}
	for _, row := range rows {
		out = append(out, pad(row))
	}
	return strings.Join(out, "\n")
}

// mark rides on the id, so a finished line is never mistaken for work still to do —
// which matters most on the garage, where --all is the only way to see one at all.
func mark(t core.Task) string {
	switch {
	case t.Done:
		return "✓"
	case t.Dropped:
		return "✗"
	case !t.Parsed():
		return "?"
	default:
		return ""
	}
}

func trayTable(items []core.Task, today time.Time) string {
	if len(items) == 0 {
		return "tray empty"
	}
	var rows [][]string
	for n, t := range items {
		urgency := "—"
		if t.Parsed() {
			urgency = fmt.Sprintf("%.1f", core.Urgency(t, today))
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d%s", n+1, mark(t)), urgency,
			dash(t.Priority()), dash(core.Day(t.Attrs["due"])), text(t),
		})
	}
	return table(rows, []string{"ID", "URG", "PRI", "DUE", "DESCRIPTION"})
}

func garageTable(items []core.Task, month string) string {
	if len(items) == 0 {
		return month + " empty"
	}
	var rows [][]string
	for n, t := range items {
		var tags []string
		for _, g := range t.Tags {
			tags = append(tags, "+"+g)
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d%s", n+1, mark(t)), text(t), strings.Join(tags, " ")})
	}
	return table(rows, []string{"ID", "DESCRIPTION", "TAGS"})
}

// grouped is the default view and the journal print: bullets by tag, no attributes.
// Numbered keeps ids on screen, so what you read is addressable.
func grouped(items []core.Task, today time.Time, numbered bool) string {
	if len(items) == 0 {
		return "tray empty"
	}
	type row struct {
		id   int
		task core.Task
	}
	groups := map[string][]row{}
	for n, t := range items {
		key := untagged
		if len(t.Tags) > 0 {
			key = t.Tags[0]
		}
		groups[key] = append(groups[key], row{n + 1, t})
	}

	names := make([]string, 0, len(groups))
	for name := range groups {
		names = append(names, name)
	}
	sort.Strings(names)

	var out []string
	for _, name := range names {
		out = append(out, "**"+name+"**")
		rows := groups[name]
		sort.SliceStable(rows, func(i, j int) bool {
			return core.Urgency(rows[i].task, today) > core.Urgency(rows[j].task, today)
		})
		for _, r := range rows {
			if numbered {
				out = append(out, fmt.Sprintf("  %d  %s", r.id, r.task.Text))
			} else {
				out = append(out, "- [ ] "+r.task.Text)
			}
		}
		out = append(out, "")
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

func findReport(hits []store.Hit) string {
	if len(hits) == 0 {
		return "no match"
	}
	var rows [][]string
	for _, h := range hits {
		line := strings.TrimPrefix(h.Line, "- ")
		line = strings.TrimPrefix(line, "[ ] ")
		line = strings.TrimPrefix(line, "[x] ")
		rows = append(rows, []string{h.Where, line})
	}
	out := table(rows, []string{"WHERE", "LINE"})
	if months := store.MonthsWith(hits); months > 2 {
		out += fmt.Sprintf("\n\n%d months — rot signal", months)
	}
	return out
}

// asJSON is Taskwarrior's import shape, so `tray export | task import` works.
func asJSON(items []core.Task, today time.Time) (string, error) {
	out := make([]map[string]any, 0, len(items))
	for _, t := range items {
		row := map[string]any{"description": t.Text, "status": status(t)}
		if p := t.Priority(); p != "" {
			row["priority"] = p
		}
		for field, key := range map[string]string{"due": "due", "entry": "entry", "end": "done"} {
			if stamp := twStamp(t.Attrs[key]); stamp != "" {
				row[field] = stamp
			}
		}
		if len(t.Tags) > 0 {
			row["tags"] = t.Tags
		}
		row["urgency"] = core.Urgency(t, today)
		row["quadrant"] = core.Quadrant(t, today)
		out = append(out, row)
	}
	blob, err := json.MarshalIndent(out, "", "  ")
	return string(blob), err
}

func status(t core.Task) string {
	switch {
	case t.Done:
		return "completed"
	case t.Dropped:
		return "deleted"
	default:
		return "pending"
	}
}

func twStamp(value string) string {
	d, ok := core.Date(value)
	if !ok {
		return ""
	}
	return d.Format("20060102T000000Z")
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func text(t core.Task) string {
	if t.Text != "" {
		return t.Text
	}
	return strings.TrimSpace(t.Raw)
}

// --all is the one switch for "show me what I finished too", on either layer. It
// used to be aliased to `dense`, so the tray table quietly included finished work
// while the garage could never show it at all.
func cmdReport(req request, dense bool) (string, error) {
	_, items, err := view(req, req.opts.all)
	if err != nil {
		return "", err
	}
	today := store.Today()
	if req.opts.json {
		return asJSON(items, today)
	}
	if req.scope == "garage" {
		month := req.opts.month
		if month == "" {
			month = store.ThisMonth()
		}
		return garageTable(items, month), nil
	}
	if dense {
		return trayTable(items, today), nil
	}
	return grouped(items, today, true), nil
}

func cmdPrint(req request) (string, error) {
	_, items, err := view(req, false)
	if err != nil {
		return "", err
	}
	return grouped(items, store.Today(), false), nil
}

func cmdFind(req request) (string, error) {
	needle := strings.Join(req.tail, " ")
	if needle == "" {
		return "nothing to find", nil
	}
	return findReport(store.Grep(needle)), nil
}

// head is the terminal-header report: the top of the tray, and nothing at all when
// there is nothing on it.
//
// It exists because a shell profile runs it on every new terminal. That changes what
// good output is — ids you didn't ask for are clutter, an urgency figure is noise at a
// glance, and a "nothing to do" line is one you stop reading in a week.
func cmdHead(req request) (string, error) {
	_, items, err := view(request{scope: "tray", filters: req.filters}, false)
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", nil // an empty tray costs a new terminal nothing
	}

	n := 3
	if len(req.tail) > 0 {
		if want, err := strconv.Atoi(req.tail[0]); err == nil && want > 0 {
			n = want
		}
	}
	shown := items
	if len(shown) > n {
		shown = shown[:n]
	}

	today := store.Today()
	whens := make([]string, len(shown))
	textW, whenW := 0, 0
	for i, t := range shown {
		whens[i] = when(t.Attrs["due"], today)
		textW = max(textW, lipgloss.Width(text(t)))
		whenW = max(whenW, lipgloss.Width(whens[i]))
	}
	// 1 letter + 2 + text + 2 + when, inside a border and a space either side.
	if over := (1 + 2 + textW + 2 + whenW + 4) - headRoom(); over > 0 {
		textW = max(12, textW-over)
	}

	rows := make([]string, len(shown))
	for i, t := range shown {
		tint := lipgloss.NewStyle().Foreground(style.Priority(t.Priority()))
		rows[i] = tint.Bold(true).Render(letter(t)) + "  " +
			tint.Render(fill(clip(text(t), textW), textW)) + "  " +
			whenStyle(whens[i]).Render(rightFill(whens[i], whenW))
	}
	return box("tray", rows, 1+2+textW+2+whenW), nil
}

// box is hand-drawn rather than lipgloss's border, because the title sits *in* the
// top edge and splicing it into an already-coloured border is guesswork about where
// the escape codes fall.
func box(title string, rows []string, inner int) string {
	edge := lipgloss.NewStyle().Foreground(style.Accent)
	name := lipgloss.NewStyle().Foreground(style.Accent).Bold(true)

	top := edge.Render("╭" + strings.Repeat("─", inner+2) + "╮")
	if fillW := inner - lipgloss.Width(title) - 1; fillW >= 0 {
		top = edge.Render("╭─ ") + name.Render(title) +
			edge.Render(" "+strings.Repeat("─", fillW)+"╮")
	}
	out := []string{top}
	for _, row := range rows {
		out = append(out, edge.Render("│")+" "+row+" "+edge.Render("│"))
	}
	return strings.Join(append(out,
		edge.Render("╰"+strings.Repeat("─", inner+2)+"╯")), "\n")
}

// An unset priority reads as medium (decision 32), but writing "M" would claim you
// chose it. The dot says the column is empty without breaking the alignment.
func letter(t core.Task) string {
	if p := t.Priority(); p != "" {
		return p
	}
	return "·"
}

// when is honest about the past. A task due last Monday rendered as "Mon" reads as
// upcoming, which is the one thing a header must never get wrong.
func when(due string, today time.Time) string {
	d, ok := core.Date(due)
	if !ok {
		return ""
	}
	days := int(d.Sub(today).Hours() / 24)
	switch {
	case days < 0:
		return fmt.Sprintf("%dd over", -days)
	case days == 0:
		return "today"
	case days == 1:
		return "tomorrow"
	case days < 7:
		return d.Format("Mon")
	default:
		return d.Format(core.DateLayout)
	}
}

// Overdue is the only thing here allowed to shout.
func whenStyle(w string) lipgloss.Style {
	switch {
	case strings.HasSuffix(w, "over"):
		return lipgloss.NewStyle().Foreground(style.High).Bold(true)
	case w == "today" || w == "tomorrow":
		return lipgloss.NewStyle().Foreground(style.Medium)
	default:
		return lipgloss.NewStyle().Foreground(style.Subtle)
	}
}

func fill(s string, width int) string {
	if pad := width - lipgloss.Width(s); pad > 0 {
		return s + strings.Repeat(" ", pad)
	}
	return s
}

// Dates read as a column of deadlines, so they line up on the right.
func rightFill(s string, width int) string {
	if pad := width - lipgloss.Width(s); pad > 0 {
		return strings.Repeat(" ", pad) + s
	}
	return s
}

func clip(s string, width int) string {
	r := []rune(s)
	if len(r) <= width || width < 2 {
		return s
	}
	return string(r[:width-1]) + "…"
}

// headRoom is the terminal's width, or a sane fallback when there isn't one — the
// header is often the first thing a profile runs, before anything has a size.
func headRoom() int {
	if w, _, err := term.GetSize(os.Stdout.Fd()); err == nil && w > 20 {
		return w
	}
	return 80
}
