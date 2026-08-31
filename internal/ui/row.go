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

// column is one column of data: its heading, and how to fill it from a task.
type column struct {
	head string // the heading, and the width the column starts from
	pad  int    // air to its right
	tray bool   // tray only — the garage has none of these decided yet
	desc bool   // the description: carries the row's state, and gives way when narrow
	cell func(d *rowDelegate, t core.Task) string
}

// Every column the table knows how to draw.
var (
	colTask = column{head: "task", pad: 2, desc: true,
		cell: func(_ *rowDelegate, t core.Task) string { return t.Text }}

	colUrg = column{head: "urg", pad: 2, tray: true,
		cell: func(d *rowDelegate, t core.Task) string {
			return fmt.Sprintf("%.1f", core.Urgency(t, d.today))
		}}

	colPri = column{head: "pri", pad: 2, tray: true,
		cell: func(_ *rowDelegate, t core.Task) string { return t.Priority() }}

	colDue = column{head: "due", pad: 2, tray: true,
		cell: func(_ *rowDelegate, t core.Task) string { return core.Day(t.Attrs["due"]) }}

	colTags = column{head: "tags",
		cell: func(_ *rowDelegate, t core.Task) string {
			if len(t.Tags) == 0 {
				return ""
			}
			return "+" + strings.Join(t.Tags, " +")
		}}
)

// columns is the table as drawn, and it is the setting: adding or removing a line
// here is the whole of changing what the table shows. Widths, headings and styling
// are all read off whatever is listed, and nothing else in the package names one.
//
// colUrg is left out. Urgency earns its keep by deciding the order, and the order is
// already on screen — the row above the other says everything 17.1 beside 8.1 does,
// in no width at all. Put it back in this list for Taskwarrior's report.
var columns = []column{colTask, colPri, colDue, colTags}

// The gutter, left of the data: cursor, selection, checkbox. Not part of `columns` —
// it is chrome rather than fields, and there is nothing to configure about it.
const (
	gPoint = iota
	gMark
	gBox
	nGutter
)

var gutterPad = [nGutter]int{gMark: 1, gBox: 1}

// rowDelegate renders a task as a table row. bubbles/list owns scrolling, paging and
// filtering; the columns stay ours, so adopting it did not cost the table.
//
// The Model copies itself on every Update, so this is held by pointer and `marked` is
// the very same map the Model marks into — never reassigned, only cleared.
type rowDelegate struct {
	tray   bool
	today  time.Time
	marked map[string]bool

	// cols and widths are set together by measure and are always the same length, so
	// a row can never index past the widths it was measured for.
	gutter [nGutter]int
	cols   []column
	widths []int
}

func (d *rowDelegate) Height() int                         { return 1 }
func (d *rowDelegate) Spacing() int                        { return 0 }
func (d *rowDelegate) Update(tea.Msg, *list.Model) tea.Cmd { return nil }

func (d *rowDelegate) Render(w io.Writer, m list.Model, i int, item list.Item) {
	r, ok := item.(row)
	if !ok {
		return
	}
	// Selection and state used to share one cell, so a selected row that was also
	// finished lost its dot. They are separate columns now.
	marked := d.marked[r.Text]
	g := [nGutter]string{gPoint: " ", gMark: " ", gBox: d.state(r.Task)}
	if i == m.Index() {
		g[gPoint] = "▸"
	}
	if marked {
		g[gMark] = "●"
	}
	cells := make([]string, len(d.cols))
	for j, c := range d.cols {
		cells[j] = c.cell(d, r.Task)
	}
	fmt.Fprint(w, d.line(g, cells, i == m.Index(), marked, false, r.Terminal()))
}

func (d *rowDelegate) header() string {
	heads := make([]string, len(d.cols))
	for j, c := range d.cols {
		heads[j] = c.head
	}
	return d.line([nGutter]string{}, heads, false, false, true, false)
}

// state is the checkbox the tray file already writes — `[x]` done, `[ ]` open. Two
// states, because that is what markdown has and what the model has.
//
// Ballot glyphs (☐ ☑) were tried and read worse: a box drawn from brackets looks like
// the file it came from, and is not ambiguous-width under a CJK locale. The garage has
// no checkbox in its file, so it keeps a one-character mark instead.
func (d *rowDelegate) state(t core.Task) string {
	if !d.tray {
		if t.Done {
			return "✓"
		}
		return " "
	}
	if t.Done {
		return "[x]"
	}
	return "[ ]"
}

// measure picks the columns for this layer and sizes them to the rows actually on
// screen, so a filter tightens the table instead of leaving it padded for rows that
// are no longer there.
func (d *rowDelegate) measure(items []list.Item, avail int) {
	d.gutter = [nGutter]int{gPoint: 1, gMark: 1}
	for _, box := range []string{d.state(core.Task{}), d.state(core.Task{Done: true})} {
		d.gutter[gBox] = max(d.gutter[gBox], lipgloss.Width(box))
	}

	d.cols = d.cols[:0]
	for _, c := range columns {
		if c.tray && !d.tray {
			continue // what `take` adds; the garage has not decided any of it
		}
		d.cols = append(d.cols, c)
	}

	w := make([]int, len(d.cols))
	for j, c := range d.cols {
		w[j] = lipgloss.Width(c.head)
	}
	for _, item := range items {
		r, ok := item.(row)
		if !ok {
			continue
		}
		for j, c := range d.cols {
			w[j] = max(w[j], lipgloss.Width(c.cell(d, r.Task)))
		}
	}

	// The description is what gives way when the table would overflow. Every other
	// column is as wide as its widest cell or it is not worth drawing.
	if avail > 0 {
		fixed, grow := 0, -1
		for i := 0; i < nGutter; i++ {
			fixed += d.gutter[i] + gutterPad[i]
		}
		for j, c := range d.cols {
			if c.desc {
				grow = j
			}
			if !c.desc {
				fixed += w[j]
			}
			fixed += c.pad
		}
		if grow >= 0 {
			if room := avail - fixed; room > 0 && w[grow] > room {
				w[grow] = room
			}
		}
	}
	d.widths = w
}

func (d *rowDelegate) line(g [nGutter]string, cells []string, selected, marked, header, finished bool) string {
	var b strings.Builder
	for i := 0; i < nGutter; i++ {
		cell := pad(g[i], d.gutter[i], gutterPad[i])
		switch {
		// The cursor and the mark are what you look for first, so nothing else gets
		// to claim their cells. A finished row used to draw its own ▸ faint, which
		// is the one row where finding it matters most.
		case header, i == gBox:
			cell = faintStyle.Render(cell)
		case i == gPoint:
			cell = cursorStyle.Render(cell)
		case i == gMark:
			cell = markStyle.Render(cell)
		}
		b.WriteString(cell)
	}
	for j, c := range d.cols {
		cell := pad(cells[j], d.widths[j], c.pad)
		switch {
		case header:
			cell = faintStyle.Render(cell)
		case c.desc:
			cell = taskStyle(selected, marked, finished).Render(cell)
		case selected && !finished: // the attributes on the row you are on stay full
		default: // the attribute columns stay quiet
			cell = faintStyle.Render(cell)
		}
		b.WriteString(cell)
	}
	return strings.TrimRight(b.String(), " ")
}

// taskStyle is the one cell that carries the state of the whole row, because it is the
// cell you are reading anyway. Struck through when the task is finished, bold while it
// is marked — a mark is worth a dot in its own column and the weight of the line, since
// a five-row selection you cannot see the shape of is one you will misapply an action
// to. The row under the cursor goes to full strength on top of either.
//
// A finished row stays dull even when marked: the colour is what says "done", and only
// the cursor is allowed to lift it.
func taskStyle(selected, marked, finished bool) lipgloss.Style {
	s := lipgloss.NewStyle()
	switch {
	case finished && selected:
		s = doneCursorStyle
	case finished:
		s = doneStyle
	case selected:
		s = titleStyle
	}
	if marked {
		s = s.Bold(true)
	}
	return s
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
	var b strings.Builder
	used := 0
	for _, r := range s {
		w := lipgloss.Width(string(r))
		if used+w+1 > width { // +1 for the ellipsis
			break
		}
		b.WriteRune(r)
		used += w
	}
	return b.String() + "…"
}
