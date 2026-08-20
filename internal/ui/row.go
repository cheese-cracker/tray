package ui

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/cheese-cracker/tray/internal/core"
)

// row is one task as bubbles/list sees it. Filtering matches the words and the tags
// — what you wrote — never `priority:H`, which you never typed.
type row struct{ core.Task }

func (r row) FilterValue() string {
	if len(r.Tags) == 0 {
		return r.Text
	}
	return r.Text + " +" + strings.Join(r.Tags, " +")
}

// The columns, and how much air each one leaves to its right. The cursor sits tight
// against the mark; the task column is the one that gives way when space is short.
const (
	cPoint = iota
	cMark
	cTask
	cUrg
	cPri
	cDue
	cTags
	nCols
)

var colPad = [nCols]int{cMark: 1, cTask: 2, cUrg: 2, cPri: 2, cDue: 2}

// rowDelegate renders a task as a table row. bubbles/list owns scrolling, paging and
// filtering; the columns stay ours, so adopting it did not cost the table.
//
// The Model copies itself on every Update, so this is held by pointer and `marked` is
// the very same map the Model marks into — never reassigned, only cleared.
type rowDelegate struct {
	tray   bool
	today  time.Time
	marked map[string]bool
	widths [nCols]int
}

func (d *rowDelegate) Height() int                         { return 1 }
func (d *rowDelegate) Spacing() int                        { return 0 }
func (d *rowDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }
func (d *rowDelegate) Render(w io.Writer, m list.Model, i int, item list.Item) {
	r, ok := item.(row)
	if !ok {
		return
	}
	cells := d.cells(r.Task)
	if i == m.Index() {
		cells[cPoint] = "▸"
	}
	if d.marked[r.Text] {
		cells[cMark] = "●"
	}
	fmt.Fprint(w, d.render(cells, i == m.Index(), false))
}

func (d *rowDelegate) headers() [nCols]string {
	h := [nCols]string{cTask: "task", cTags: "tags"}
	if d.tray {
		h[cUrg], h[cPri], h[cDue] = "urg", "pri", "due"
	}
	return h
}

func (d *rowDelegate) header() string { return d.render(d.headers(), false, true) }

// A garage month has no urgency, priority or due date to show: those are what `take`
// adds, and the whole point of the garage is that they haven't been decided yet.
func (d *rowDelegate) cells(t core.Task) [nCols]string {
	var c [nCols]string
	c[cPoint], c[cMark] = " ", " "
	c[cTask] = t.Text
	if d.tray {
		c[cUrg] = fmt.Sprintf("%.1f", core.Urgency(t, d.today))
		c[cPri] = t.Priority()
		c[cDue] = core.Day(t.Attrs["due"])
	}
	var tags []string
	for _, g := range t.Tags {
		tags = append(tags, "+"+g)
	}
	c[cTags] = strings.Join(tags, " ")
	return c
}

// measure sizes the columns to the rows actually on screen, so a filter tightens the
// table instead of leaving it padded for rows that are no longer there.
func (d *rowDelegate) measure(items []list.Item, avail int) {
	w := [nCols]int{cPoint: 1, cMark: 1}
	h := d.headers()
	for i := cTask; i < nCols; i++ {
		w[i] = lipgloss.Width(h[i])
	}
	for _, item := range items {
		r, ok := item.(row)
		if !ok {
			continue
		}
		c := d.cells(r.Task)
		for i := cTask; i < nCols; i++ {
			w[i] = max(w[i], lipgloss.Width(c[i]))
		}
	}
	if avail > 0 {
		fixed := colPad[cTask]
		for i := range w {
			if i != cTask {
				fixed += w[i] + colPad[i]
			}
		}
		if room := avail - fixed; room > 0 && w[cTask] > room {
			w[cTask] = room
		}
	}
	d.widths = w
}

func (d *rowDelegate) render(cells [nCols]string, selected, header bool) string {
	var b strings.Builder
	for i := 0; i < nCols; i++ {
		cell := pad(cells[i], d.widths[i], colPad[i])
		switch {
		case header:
			cell = faintStyle.Render(cell)
		case i == cPoint:
			cell = cursorStyle.Render(cell)
		case i == cMark:
			cell = markStyle.Render(cell)
		case selected:
			cell = titleStyle.Render(cell)
		case i >= cUrg: // the attribute columns stay quiet
			cell = faintStyle.Render(cell)
		}
		b.WriteString(cell)
	}
	return strings.TrimRight(b.String(), " ")
}

// pad truncates first so lipgloss pads rather than wraps: a wrapped row would push
// every row below it down by one and make the whole pane jitter.
func pad(s string, width, right int) string {
	return lipgloss.NewStyle().Width(width + right).Render(truncate(s, width))
}

func truncate(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	r := []rune(s)
	for len(r) > 0 && lipgloss.Width(string(r))+1 > width {
		r = r[:len(r)-1]
	}
	return string(r) + "…"
}
