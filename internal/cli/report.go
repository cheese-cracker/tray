package cli

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/term"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
	"github.com/cheese-cracker/tray/internal/style"
	"github.com/cheese-cracker/tray/internal/wire"
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

// mark rides on the id, so a finished row or a template is never mistaken for work
// still to do — which matters most under --all, the only place they show.
func mark(t core.Task) string {
	switch {
	case t.Terminal():
		return "✓"
	case t.Recur != "":
		return "↻"
	default:
		return ""
	}
}

func id(t core.Task) string { return t.ID + mark(t) }

func trayTable(items []core.Task, today time.Time) string {
	if len(items) == 0 {
		return "tray empty"
	}
	var rows [][]string
	for _, t := range items {
		rows = append(rows, []string{
			id(t), fmt.Sprintf("%.1f", core.Urgency(t, today)),
			dash(t.Priority), dash(core.Day(t.Due)), t.Text,
		})
	}
	return table(rows, []string{"ID", "URG", "PRI", "DUE", "DESCRIPTION"})
}

// The garage table grows a WAIT column only once a row has a day to surface on; the
// table is unchanged for anyone who never writes one (104e's rule).
func garageTable(items []core.Task, month string) string {
	if len(items) == 0 {
		return month + " empty"
	}
	waits := false
	for _, t := range items {
		waits = waits || t.Wait != ""
	}
	headers := []string{"ID", "DESCRIPTION", "TAGS"}
	if waits {
		headers = append(headers, "WAIT")
	}
	var rows [][]string
	for _, t := range items {
		var tags []string
		for _, g := range t.Tags {
			tags = append(tags, core.TagMark+g)
		}
		row := []string{id(t), t.Text, strings.Join(tags, " ")}
		if waits {
			row = append(row, core.Day(t.Wait))
		}
		rows = append(rows, row)
	}
	return table(rows, headers)
}

// byTag buckets rows under their first tag, each bucket in urgency order, names sorted.
func byTag(items []core.Task, today time.Time) ([]string, map[string][]core.Task) {
	groups := map[string][]core.Task{}
	for _, t := range items {
		key := untagged
		if len(t.Tags) > 0 {
			key = t.Tags[0]
		}
		groups[key] = append(groups[key], t)
	}
	names := make([]string, 0, len(groups))
	for name, rows := range groups {
		names = append(names, name)
		sort.SliceStable(rows, func(i, j int) bool {
			return core.Urgency(rows[i], today) > core.Urgency(rows[j], today)
		})
	}
	sort.Strings(names)
	return names, groups
}

// grouped is the default view and the journal print: bullets by tag, no attributes.
// Numbered keeps ids on screen, so what you read is addressable.
func grouped(items []core.Task, today time.Time, numbered bool) string {
	if len(items) == 0 {
		return "tray empty"
	}
	names, groups := byTag(items, today)
	var out []string
	for _, name := range names {
		out = append(out, "**"+name+"**")
		for _, t := range groups[name] {
			switch {
			case numbered:
				out = append(out, fmt.Sprintf("  %s  %s", t.ID, t.Text))
			case t.Terminal():
				out = append(out, "- [x] ~~"+t.Text+"~~")
			default:
				out = append(out, "- [ ] "+t.Text)
			}
			if !numbered {
				out = append(out, indented(t.Note, "  ")...)
			}
		}
		out = append(out, "")
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

// contextReport is what you hand an agent: the grouped report with ids, and under each
// task the note it carries — the whole of what tray knows about it, in plain text.
func contextReport(items []core.Task, today time.Time) string {
	names, groups := byTag(items, today)
	var out []string
	for _, name := range names {
		out = append(out, "**"+name+"**")
		for _, t := range groups[name] {
			out = append(out, fmt.Sprintf("  %s  %s", t.ID, t.Text))
			out = append(out, indented(t.Note, "      ")...)
		}
		out = append(out, "")
	}
	return strings.TrimRight(strings.Join(out, "\n"), "\n")
}

func indented(note, prefix string) []string {
	var out []string
	for _, l := range strings.Split(note, "\n") {
		if l != "" {
			out = append(out, prefix+l)
		}
	}
	return out
}

func dash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

// --all is the one switch for "show me what I finished too", on either layer.
func cmdReport(s *store.Store, req request, dense bool) (string, error) {
	items, err := view(s, req, req.opts.all)
	if err != nil {
		return "", err
	}
	today := store.Today()
	if req.opts.json {
		return wire.ExportTaskwarrior(items, today)
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

func cmdPrint(s *store.Store, req request) (string, error) {
	items, err := view(s, req, false)
	if err != nil {
		return "", err
	}
	return grouped(items, store.Today(), false), nil
}

// head is the terminal-header report: the top of the tray, and nothing at all when
// there is nothing on it.
//
// It exists because a shell profile runs it on every new terminal. That changes what
// good output is — ids you didn't ask for are clutter, an urgency figure is noise at a
// glance, and a "nothing to do" line is one you stop reading in a week.
func cmdHead(s *store.Store, req request) (string, error) {
	items, err := view(s, request{scope: "tray", filters: req.filters}, false)
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
		whens[i] = core.Day(t.Due)
		textW = max(textW, lipgloss.Width(t.Text))
		whenW = max(whenW, lipgloss.Width(whens[i]))
	}
	// 1 letter + 2 + text + 2 + when, inside a border and a space either side.
	if over := (1 + 2 + textW + 2 + whenW + 4) - headRoom(); over > 0 {
		textW = max(12, textW-over)
	}

	rows := make([]string, len(shown))
	for i, t := range shown {
		tint := lipgloss.NewStyle().Foreground(style.Priority(t.Priority))
		rows[i] = tint.Bold(true).Render(letter(t)) + "  " +
			tint.Render(fill(clip(t.Text, textW), textW)) + "  " +
			whenStyle(shown[i].Due, today).Render(rightFill(whens[i], whenW))
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
	if t.Priority != "" {
		return t.Priority
	}
	return "·"
}

func daysUntil(d, today time.Time) int {
	return int(d.Sub(today).Hours() / 24)
}

// Overdue is the only thing here allowed to shout. This takes the date rather than the
// rendered string: sniffing the string only worked because it carried the words `over`
// and `today` — change the format and every row silently renders quiet.
func whenStyle(due string, today time.Time) lipgloss.Style {
	d, ok := core.Date(due)
	if !ok {
		return lipgloss.NewStyle().Foreground(style.Subtle)
	}
	switch days := daysUntil(d, today); {
	case days < 0:
		return lipgloss.NewStyle().Foreground(style.High).Bold(true)
	case days <= 1:
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
