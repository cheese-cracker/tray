package ui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// `?` is a page, not a keymap strip. A keymap tells you which letter does a thing you
// already understand; the thing most people do not understand here is why there are
// two layers at all. So the diagram comes first and the keys come last.
//
// It replaces the list rather than pushing it up, which is what makes it fit on a
// short terminal and lets it be read as one thing.

var (
	helpBox = lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).BorderForeground(subtle).
		Padding(0, 1).Width(26)
	helpHead = lipgloss.NewStyle().Bold(true).Foreground(accent)
)

func (m Model) helpPage() string {
	garage := helpBox.Render(strings.Join([]string{
		helpHead.Render("garage"),
		faintStyle.Render("anything, half-formed."),
		faintStyle.Render("nothing is asked of you"),
		"",
		keyStyle.Render("a") + faintStyle.Render("  dump a line"),
	}, "\n"))

	tray := helpBox.Render(strings.Join([]string{
		helpHead.Render("tray"),
		faintStyle.Render("three to seven things"),
		faintStyle.Render("you are doing now"),
		faintStyle.Render("priority · due · tags"),
		keyStyle.Render("a") + faintStyle.Render("  add, with the form"),
	}, "\n"))

	// The gutter is the whole point of the picture: `take` is the one moment
	// structure gets paid for, and `d` is the way back.
	gutter := strings.Join([]string{
		"", "", // clear the box's top border and title row
		"  " + keyStyle.Render("t") + faintStyle.Render(" take"),
		"  " + cursorStyle.Render("─────▶"),
		"  " + cursorStyle.Render("◀─────"),
		"  " + keyStyle.Render("d") + faintStyle.Render(" back"),
	}, "\n")

	var b strings.Builder
	b.WriteString(faintStyle.Render(
		"Dump anything into the garage; take a few things onto the tray.") + "\n\n")
	b.WriteString(lipgloss.JoinHorizontal(lipgloss.Top, garage, gutter, tray) + "\n\n")
	b.WriteString(faintStyle.Render(
		"Structure is paid for once, when something graduates — never at capture.") + "\n\n")
	b.WriteString(helpKeys() + "\n")
	return b.String()
}

// One section, four columns. The widths are measured from the labels rather than
// picked by hand: hand-picked ones broke three times running — the last column wrapped
// at 22, "show done" collided with its neighbour at 18, and "tab  h l" was clipped to
// "tab  h…" by a key column sized for a shorter key.
func helpKeys() string {
	cols := [][][2]string{
		{{"", "moving"}, {"↑↓  j k", "move"}, {"tab  h l", "switch"}},
		{{"", "choosing"}, {"space", "select"}, {"enter", "the menu"}, {"/", "filter"}, {"v", "show done"}},
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
