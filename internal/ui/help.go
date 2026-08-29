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

var (
	helpBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(subtle).
		Padding(0, 1).Width(22)
	helpHead = lipgloss.NewStyle().Bold(true).Foreground(accent)
)

func (m Model) helpPage() string {
	garage := helpBox.Render(strings.Join([]string{
		helpHead.Render("garage"),
		faintStyle.Render("anything, unformed"),
		faintStyle.Render("nothing is asked"),
		keyStyle.Render("a") + faintStyle.Render("  dump a line"),
	}, "\n"))

	tray := helpBox.Render(strings.Join([]string{
		helpHead.Render("tray"),
		faintStyle.Render("three to seven, now"),
		faintStyle.Render("priority, due, tags"),
		keyStyle.Render("a") + faintStyle.Render("  add, structured"),
	}, "\n"))

	// The gutter is the whole point of the picture: `take` is the one moment
	// structure gets paid for, and `d` is the way back.
	gutter := strings.Join([]string{
		"", // the box's top border
		"  " + keyStyle.Render("t") + faintStyle.Render(" take"),
		"  " + cursorStyle.Render("─────▶"),
		"  " + cursorStyle.Render("◀─────"),
		"  " + keyStyle.Render("d") + faintStyle.Render(" back"),
	}, "\n")

	// What the thing is, before how it works. The diagram answers "why two layers";
	// nothing answered "what am I even looking at".
	what := helpHead.Render("tray") + faintStyle.Render(" — a task list kept in plain markdown, in") +
		"\n" + faintStyle.Render("~/task-garage. Yours to edit in any editor.")

	concept := strings.Join([]string{
		what,
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, garage, gutter, tray),
		faintStyle.Render("Structure is paid for once, when something graduates."),
	}, "\n")

	keys := helpKeys()
	rule := faintStyle.Render(strings.Repeat("─",
		max(lipgloss.Width(concept), lipgloss.Width(keys))))

	return strings.Join([]string{concept, rule, keys}, "\n")
}

// The lower section: keys, in three columns. Four spanned the whole terminal, and a
// popup that fills the screen is not a popup. Widths are measured from the labels
// rather than picked by hand — hand-picked ones broke three times running.
func helpKeys() string {
	cols := [][][2]string{
		{
			{"", "keys"},
			{"↑↓  j k", "move"}, {"tab", "switch layer"}, {"space", "select"},
			{"enter", "the menu"}, {"/", "filter"}, {"v", "show done"},
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
		rows[row] = left + line + ansi.TruncateLeft(rows[row], x+topW, "")
	}
	return strings.Join(rows, "\n")
}
