package cli

import (
	"encoding/json"
	"fmt"
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

func trayTable(items []core.Task, today time.Time) string {
	if len(items) == 0 {
		return "tray empty"
	}
	var rows [][]string
	for n, t := range items {
		mark := ""
		switch {
		case t.Done:
			mark = "✓"
		case t.Dropped:
			mark = "✗"
		case !t.Parsed():
			mark = "?"
		}
		urgency := "—"
		if t.Parsed() {
			urgency = fmt.Sprintf("%.1f", core.Urgency(t, today))
		}
		rows = append(rows, []string{
			fmt.Sprintf("%d%s", n+1, mark), urgency,
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
		rows = append(rows, []string{fmt.Sprint(n + 1), text(t), strings.Join(tags, " ")})
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

func cmdReport(req request, dense bool) (string, error) {
	_, items, err := view(req, dense)
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
