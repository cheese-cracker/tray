package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

// `?` is a page, not a keymap strip. A keymap tells you which letter does a thing you
// already understand; the thing most people do not understand here is why there are
// two layers at all. So the diagram comes first and the keys come last.
//
// It replaces the list rather than pushing it up, which is what makes it fit on a
// short terminal and lets it be read as one thing.

// dialog is the frame `?` floats in. It is drawn over the list rather than replacing
// it, so the interface stays on screen behind what is explaining it.
var dialog = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).BorderForeground(accent).
	Padding(0, 2)

var helpHead = lipgloss.NewStyle().Bold(true).Foreground(accent)

// titled draws a box with its name set into the top edge, which buys back the row a
// name-as-first-line costs. The dialog has to fit twenty rows to stay clear of the
// footer on a short terminal, and every row is spent.
func titled(name string, lines []string, inner int) string {
	edge := lipgloss.NewStyle().Foreground(subtle)
	out := []string{
		edge.Render("╭─ ") + helpHead.Render(name) +
			edge.Render(" "+strings.Repeat("─", inner-lipgloss.Width(name)-1)+"╮"),
	}
	for _, l := range lines {
		out = append(out, edge.Render("│")+" "+pad(l, inner, 0)+" "+edge.Render("│"))
	}
	return strings.Join(append(out,
		edge.Render("╰"+strings.Repeat("─", inner+2)+"╯")), "\n")
}

func (m Model) helpPage() string {
	// The picture says what the two layers *are*. Keys live under it, not inside it:
	// a diagram carrying its own keybindings is doing two jobs and neither well.
	garage := titled("garage", []string{
		faintStyle.Render("anything, any month"),
		faintStyle.Render("no structure asked"),
	}, 19)

	tray := titled("tray", []string{
		faintStyle.Render("what you do now"),
		faintStyle.Render("priority, due, tags"),
	}, 19)

	gutter := strings.Join([]string{"", "  " + cursorStyle.Render("────▶"),
		"  " + cursorStyle.Render("◀────"), ""}, "\n")

	what := strings.Join([]string{
		helpHead.Render("tray") + faintStyle.Render(" is a task manager in two layers. Dump any task,"),
		faintStyle.Render("or a half-thought, into the garage for the month you"),
		faintStyle.Render("expect to pick it up. Take it onto the tray when ready."),
	}, "\n")

	// The four moves, in the order you meet them.
	moves := strings.Join([]string{
		verb("(a)dd", "a line to the layer you are on"),
		verb("(t)ake", "a garage line onto the tray, structured"),
		verb("(d)", "hand a tray task back to the garage"),
		verb("(enter)", "act on the selection; the menu lists the rest"),
	}, "\n")

	diagram := lipgloss.JoinHorizontal(lipgloss.Top, garage, gutter, tray)
	keys := helpKeys()
	rule := faintStyle.Render(strings.Repeat("─", max(
		max(lipgloss.Width(what), lipgloss.Width(diagram)),
		max(lipgloss.Width(moves), lipgloss.Width(keys)))))

	// No blank before the diagram: its top border already separates it, and the
	// dialog has exactly twenty rows before it starts covering the footer.
	return strings.Join([]string{what, diagram, moves, rule, keys}, "\n")
}

// helpDialog grows into the terminal. A modal fixed at its content's size reads as
// a tooltip on a big screen — but it keeps a margin, because a box filling every
// column is not floating over anything.
//
// The block inside stays left-aligned and is centred as a whole: centring each line
// would scatter the key columns.
func (m Model) helpDialog() string {
	content := m.helpPage()
	w, h := lipgloss.Width(content), lipgloss.Height(content)
	// Grow, but only into air the content can use. Uncapped it became a vast box
	// with a paragraph adrift in the middle — larger, and worse.
	if m.width > 0 {
		w = max(w, min(m.width*9/10-dialog.GetHorizontalFrameSize(), w+16))
	}
	if m.height > 0 {
		// The four rows the top margin and the footer need, or it covers them.
		w := m.height - 4 - dialog.GetVerticalFrameSize()
		h = max(h, min(w, h+4))
	}
	return dialog.Render(centre(content, w, h))
}

// centre indents the block by one amount, rather than centring each line.
//
// lipgloss.Place looks like the call for this and is not: for Center it pads by
// `gap + short`, where short is how far that line falls below the widest — so every
// line is centred on its own and the key columns come apart. Checked, not assumed.
func centre(content string, w, h int) string {
	lines := strings.Split(content, "\n")
	if gap := (w - lipgloss.Width(content)) / 2; gap > 0 {
		indent := strings.Repeat(" ", gap)
		for i := range lines {
			lines[i] = indent + lines[i]
		}
	}
	if gap := (h - len(lines)) / 2; gap > 0 {
		lines = append(make([]string, gap), lines...)
		for len(lines) < h {
			lines = append(lines, "")
		}
	}
	return strings.Join(lines, "\n")
}

func verb(key, meaning string) string {
	return keyStyle.Render(pad(key, 8, 1)) + faintStyle.Render(meaning)
}

// The lower section: keys, in three columns. Four spanned the whole terminal, and a
// popup that fills the screen is not a popup. Widths are measured from the labels
// rather than picked by hand — hand-picked ones broke three times running.
func helpKeys() string {
	cols := [][][2]string{
		{
			{"", "keys"},
			{"↑↓  j k", "move"}, {"tab", "switch layer"}, {"space", "select"},
			{"/", "filter"}, {"v", "show done"},
		},
		{{"", "acting"}, {"a", "add"}, {"t", "take"}, {"r", "retake"}, {"x", "done"}},
		{{"", ""}, {"d", "hand back"}, {">", "move to"}, {"D", "delete"}, {"R", "restore"}},
	}

	keyW := 0
	for _, col := range cols {
		for _, pair := range col[1:] {
			keyW = max(keyW, lipgloss.Width(pair[0]))
		}
	}

	rendered := make([]string, len(cols))
	for i, col := range cols {
		descW := 0
		var rows []string
		for j, pair := range col {
			if j == 0 {
				rows = append(rows, helpHead.Render(pair[1]))
				continue
			}
			descW = max(descW, lipgloss.Width(pair[1]))
			rows = append(rows, keyStyle.Render(pad(pair[0], keyW, 1))+faintStyle.Render(pair[1]))
		}
		width := keyW + 1 + descW
		if i < len(cols)-1 {
			width += 2 // a gutter, except after the last column
		}
		rendered[i] = lipgloss.NewStyle().Width(width).Render(strings.Join(rows, "\n"))
	}
	return lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
}

// overlay splices top over bg, centred, keeping whatever of bg still shows at the
// edges. lipgloss has no compositor, so the slicing is done cell-wise with ansi —
// cutting a styled line by byte offset would shred the escape sequences.
func overlay(bg, top string) string {
	rows := strings.Split(bg, "\n")
	over := strings.Split(top, "\n")
	width, height := lipgloss.Width(bg), len(rows)
	topW := lipgloss.Width(top)

	x := max(0, (width-topW)/2)
	y := max(0, (height-len(over))/2)

	for i, line := range over {
		row := y + i
		if row < 0 || row >= len(rows) {
			continue
		}
		left := ansi.Truncate(rows[row], x, "")
		if gap := x - ansi.StringWidth(left); gap > 0 {
			left += strings.Repeat(" ", gap)
		}
		// Clipped to the background: the help has a minimum width it cannot go below,
		// and on a terminal narrower than that an unclipped row would wrap and
		// corrupt every line beneath it. Cut off beats scrambled.
		rows[row] = ansi.Truncate(left+line+ansi.TruncateLeft(rows[row], x+topW, ""), width, "")
	}
	return strings.Join(rows, "\n")
}
