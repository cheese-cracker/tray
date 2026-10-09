package ui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cheese-cracker/tray/internal/sync"
)

// runSync is `S`: the built-in hooks, then every plugin's plan. A plan that only
// pushes lands at once — local is the source of truth (T37), so there is nothing to
// decide. A plan with rows coming in is held and shown; it lands on enter and is
// kept out on esc, its pushes going either way (T41).
func (m *Model) runSync() tea.Cmd {
	sum, results, err := sync.Sync(m.s, sync.Manual, "", sync.DefaultTimeout)
	if err != nil {
		m.status = err.Error()
		return m.reload()
	}
	notes := []string{"synced · " + sum.String()}
	m.plans = nil
	for _, r := range results {
		switch {
		case r.Err != nil:
			notes = append(notes, r.Plugin+": "+r.Message)
		case !r.Diff.Empty():
			m.plans = append(m.plans, r)
		case len(r.Push) > 0:
			notes = append(notes, m.land(r))
		}
		if n := len(r.Diff.OnTray); n > 0 {
			notes = append(notes, fmt.Sprintf("%s: %d on the tray, kept", r.Plugin, n))
		}
	}
	m.status = strings.Join(notes, " · ")
	if len(m.plans) > 0 {
		m.mode = reviewing
	}
	return m.reload()
}

func (m *Model) land(r sync.Result) string {
	a, err := sync.Apply(m.s, r, r.Push, sync.DefaultTimeout)
	if err != nil {
		return r.Plugin + ": " + err.Error()
	}
	out := fmt.Sprintf("%s: +%d ~%d ↑%d", r.Plugin, a.Added, a.Updated, len(a.PushOK))
	if n := len(a.PushFailed); n > 0 {
		out += fmt.Sprintf(" (%d failed)", n)
	}
	return out
}

func (m Model) updateReview(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var notes []string
	switch msg.String() {
	case "enter", "y":
		for _, r := range m.plans {
			notes = append(notes, m.land(r))
		}
	case "esc", "q", "n":
		// Kept local: nothing comes in, and what was going out still goes.
		for _, r := range m.plans {
			r.Diff = sync.Diff{}
			notes = append(notes, m.land(r)+" · kept local")
		}
	default:
		return m, nil
	}
	m.plans, m.mode = nil, browsing
	m.status = strings.Join(notes, " · ")
	return m, m.reload()
}

// renderReview is the diff: what each plugin would bring in, one line a row. Pushes
// are not shown — they are local state going out, already decided.
func (m Model) renderReview() string {
	n := 0
	for _, r := range m.plans {
		n += len(r.Diff.Adds) + len(r.Diff.Updates)
	}
	rows := []string{faintStyle.Render(fmt.Sprintf("%d coming in", n))}
	for _, r := range m.plans {
		rows = append(rows, "  "+titleStyle.Render(r.Plugin))
		for _, row := range r.Diff.Adds {
			rows = append(rows, "    "+keyStyle.Render("+")+" "+row.Text+faintStyle.Render(sync.TagSuffix(row.Tags)))
		}
		for _, u := range r.Diff.Updates {
			rows = append(rows, "    "+keyStyle.Render("~")+" "+u.Old.Text+faintStyle.Render("  "+u.String()))
		}
	}
	return strings.Join(rows, "\n")
}
