package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// `?` is a screen of its own, not a panel inside the interface.
//
// It began as an overlay floating over the list, which grew a frame but never grew
// its contents — so it read the same size however big the terminal was. Taking the
// whole screen means the prose can reflow, the boxes can spread, and there is no
// twenty-row ceiling to design against. Any key goes back.
var helpHead = lipgloss.NewStyle().Bold(true).Foreground(accent)

// titled draws a box with its name set into the top edge.
func titled(name string, lines []string, inner int) string {
	edge := lipgloss.NewStyle().Foreground(subtle)
	out := []string{
		edge.Render("╭─ ") + helpHead.Render(name) +
			edge.Render(" "+strings.Repeat("─", max(0, inner-lipgloss.Width(name)-1))+"╮"),
	}
	for _, l := range lines {
		out = append(out, edge.Render("│")+" "+pad(l, inner, 0)+" "+edge.Render("│"))
	}
	return strings.Join(append(out,
		edge.Render("╰"+strings.Repeat("─", inner+2)+"╯")), "\n")
}

// Below this the two boxes and their gutter cannot sit side by side.
const minDiagramWidth = 72

func (m Model) helpScreen() string {
	width := 76
	if m.width > 8 {
		width = m.width - 4
	}

	title := helpHead.Render("tray") +
		faintStyle.Render(" — a task manager in two layers")
	if gap := width - lipgloss.Width(title) - 20; gap > 0 {
		title += strings.Repeat(" ", gap) + faintStyle.Render("any key to go back")
	}

	// Reflows: on a wide terminal this is two lines, on a narrow one four.
	prose := lipgloss.NewStyle().Width(width).Foreground(subtle).Render(
		"Dump any task, or a half-thought, into the garage for the month you expect " +
			"to pick it up. Take it onto the tray when you are ready to work on it.")

	// The boxes share the width, so the picture spreads instead of sitting in a
	// corner of a big screen.
	gutter := 12
	inner := max(17, (width-gutter)/2-4)
	garage := titled("garage", []string{
		faintStyle.Render("anything, any month"),
		faintStyle.Render("no structure asked"),
	}, inner)
	tray := titled("tray", []string{
		faintStyle.Render("what you do now"),
		faintStyle.Render("priority, due, tags"),
	}, inner)
	arrows := strings.Join([]string{
		"", "  " + cursorStyle.Render("────▶"), "  " + cursorStyle.Render("◀────"), "",
	}, "\n")

	moves := strings.Join([]string{
		verb("(a)dd", "a line to the layer you are on"),
		verb("(t)ake", "a garage line onto the tray, structured"),
		verb("(d)", "hand a tray task back to the garage"),
		verb("(enter)", "act on the selection; the menu lists the rest"),
	}, "\n")

	words := []string{title, "", prose, "", moves, "",
		faintStyle.Render(strings.Repeat("─", width)), "", helpKeys()}

	// The picture is the part that can go. On a narrow terminal the two boxes and
	// their gutter do not fit, and on a short one something has to give — losing the
	// keymap or clipping mid-sentence would both be worse.
	diagram := lipgloss.JoinHorizontal(lipgloss.Top, garage, arrows, tray)
	full := append([]string{title, "", prose, "", diagram}, words[3:]...)
	if width+4 >= minDiagramWidth && (m.height == 0 || lipgloss.Height(strings.Join(full, "\n"))+2 <= m.height) {
		return strings.Join(full, "\n")
	}
	return strings.Join(words, "\n")
}

func verb(key, meaning string) string {
	return keyStyle.Render(pad(key, 8, 1)) + faintStyle.Render(meaning)
}

// The lower section: keys, in three columns. Widths are measured from the labels
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
