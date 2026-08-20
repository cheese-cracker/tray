package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"

	"github.com/cheese-cracker/tray/internal/core"
)

var (
	accent = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	subtle = lipgloss.AdaptiveColor{Light: "#909090", Dark: "#626262"}

	titleStyle  = lipgloss.NewStyle().Bold(true)
	faintStyle  = lipgloss.NewStyle().Foreground(subtle)
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
	markStyle   = lipgloss.NewStyle().Foreground(accent)
	keyStyle    = lipgloss.NewStyle().Bold(true).Foreground(accent)

	// Tabs, as bubbletea's own tabs example does it: the active tab's bottom border
	// is opened so it reads as one shape with the pane below.
	inactiveTabBorder = tabBorder("┴", "─", "┴")
	activeTabBorder   = tabBorder("┘", " ", "└")

	inactiveTab = lipgloss.NewStyle().
			Border(inactiveTabBorder, true).BorderForeground(accent).
			Foreground(subtle).Padding(0, 2)
	activeTab = inactiveTab.Border(activeTabBorder, true).
			Foreground(lipgloss.NoColor{}).Bold(true)

	pane = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).BorderForeground(accent).
		UnsetBorderTop().Padding(1, 2)

	// Carries the tab row's bottom border out to the edge of the pane; without it
	// the frame stops wherever the last tab ended.
	tabGap = inactiveTab.BorderTop(false).BorderLeft(false).BorderRight(false).
		UnsetPadding()
)

func tabBorder(left, middle, right string) lipgloss.Border {
	border := lipgloss.RoundedBorder()
	border.BottomLeft, border.Bottom, border.BottomRight = left, middle, right
	return border
}

func (m Model) View() string {
	if m.err != nil {
		return "tray: " + m.err.Error() + "\n"
	}

	tabs := m.renderTabs()
	body := m.renderBody()
	footer := m.renderFooter()

	style := pane
	switch {
	case m.width > 0:
		// Fill the terminal. lipgloss counts padding inside Width and Height but adds
		// borders outside, so only the border is subtracted here.
		style = pane.Width(m.width - pane.GetHorizontalBorderSize())
		if fill := m.height - lipgloss.Height(tabs) - lipgloss.Height(footer) -
			pane.GetVerticalBorderSize(); fill > 0 {
			style = style.Height(fill)
		}
	case lipgloss.Width(tabs)-pane.GetHorizontalFrameSize() > lipgloss.Width(body):
		// No size yet: widen to meet the tab bar, but never narrow below the content.
		style = pane.Width(lipgloss.Width(tabs) - pane.GetHorizontalFrameSize())
	}

	var b strings.Builder
	b.WriteString(tabs + "\n")
	b.WriteString(style.Render(body) + "\n")
	b.WriteString(footer)
	return b.String()
}

// rowRoom is how many task rows fit, leaving space for the chrome. Zero means no
// limit, which is what happens before the terminal has told us its size.
func (m Model) rowRoom() int {
	if m.height == 0 {
		return 0
	}
	chrome := 3 + pane.GetVerticalBorderSize() + 2 + 1 + 1 // tabs, border, padding, header, footer
	switch m.mode {
	case acting:
		chrome += len(m.offered()) + 2
	case sending:
		chrome += len(m.dests) + 2
	}
	return max(1, m.height-chrome)
}

// window keeps the cursor on screen without keeping any scroll state: the offset is
// derived, so it can never drift out of step with the list.
func (m Model) window() (int, int) {
	room := m.rowRoom()
	if room == 0 || len(m.items) <= room {
		return 0, len(m.items)
	}
	room = max(1, room-1) // the "… more" line needs a row of its own
	start := m.cursor - room/2
	if start < 0 {
		start = 0
	}
	if start > len(m.items)-room {
		start = len(m.items) - room
	}
	return start, start + room
}

func (m Model) renderTabs() string {
	var rendered []string
	for i, l := range m.layers {
		style := inactiveTab
		if i == m.active {
			style = activeTab
		}
		border, _, _, _, _ := style.GetBorder()
		switch {
		case i == 0 && i == m.active:
			border.BottomLeft = "│"
		case i == 0:
			border.BottomLeft = "├"
		case i == len(m.layers)-1 && i == m.active:
			border.BottomRight = "│"
		case i == len(m.layers)-1:
			border.BottomRight = "┤"
		}
		label := l.title
		if !l.isTray() {
			label = "garage · " + l.title
		}
		rendered = append(rendered, style.Border(border).Render(label))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	if gap := m.width - lipgloss.Width(row); m.width > 0 && gap > 0 {
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, row,
			tabGap.Render(strings.Repeat(" ", gap)))
	}
	return row
}

func (m Model) renderBody() string {
	if m.form != nil {
		return m.form.view()
	}
	if len(m.items) == 0 {
		return faintStyle.Render(m.emptyMessage())
	}

	body := m.renderTable()
	switch m.mode {
	case acting:
		body += "\n\n" + m.renderMenu()
	case sending:
		body += "\n\n" + m.renderDestinations()
	}
	return body
}

func (m Model) emptyMessage() string {
	if m.layer().isTray() {
		return "nothing on the tray — a to add one, or take something from the garage"
	}
	return "nothing here — a to dump a line"
}

// renderTable uses lipgloss's own table so columns align without hand-padding.
func (m Model) renderTable() string {
	tray := m.layer().isTray()

	headers := []string{"", "", "task", "urg", "pri", "due", "tags"}
	if !tray {
		headers = []string{"", "", "task", "", "", "", "tags"}
	}

	start, end := m.window()
	rows := make([][]string, 0, end-start)
	for i, t := range m.items[start:end] {
		i += start
		mark := " "
		if m.marked[t.Text] {
			mark = markStyle.Render("●")
		}
		point := " "
		if i == m.cursor {
			point = cursorStyle.Render("▸")
		}
		urgency, priority, due := "", "", ""
		if tray {
			urgency = fmt.Sprintf("%.1f", core.Urgency(t, m.today))
			priority = t.Priority()
			due = core.Day(t.Attrs["due"])
		}
		var tags []string
		for _, g := range t.Tags {
			tags = append(tags, "+"+g)
		}
		rows = append(rows, []string{
			point, mark, t.Text, urgency, priority, due, strings.Join(tags, " "),
		})
	}
	if hidden := len(m.items) - (end - start); hidden > 0 {
		rows = append(rows, []string{"", "", fmt.Sprintf("… %d more", hidden), "", "", "", ""})
	}

	return table.New().
		Border(lipgloss.HiddenBorder()).
		BorderTop(false).BorderBottom(false).BorderLeft(false).BorderRight(false).
		BorderHeader(false).BorderColumn(false).BorderRow(false).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			style := lipgloss.NewStyle().PaddingRight(2)
			switch col {
			case 0:
				style = style.PaddingRight(0) // the cursor sits tight against the mark
			case 1:
				style = style.PaddingRight(1)
			}
			if row == table.HeaderRow {
				return style.Foreground(subtle)
			}
			if start, _ := m.window(); row+start == m.cursor {
				return style.Bold(true)
			}
			if col >= 3 { // the attribute columns stay quiet
				return style.Foreground(subtle)
			}
			return style
		}).
		Render()
}

func (m Model) renderMenu() string {
	rows := []string{faintStyle.Render(fmt.Sprintf("%d selected", len(m.picked())))}
	for i, a := range m.offered() {
		row := fmt.Sprintf("  %s  %s", keyStyle.Render(a.key), a.label)
		if i == m.menuAt {
			row = cursorStyle.Render("▸") + row[1:]
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderDestinations() string {
	rows := []string{faintStyle.Render(fmt.Sprintf("move %d to", len(m.picked())))}
	for i, l := range m.dests {
		label := l.title
		if !l.isTray() {
			label = "garage · " + l.title
		}
		row := "    " + label
		if i == m.destAt {
			row = "  " + cursorStyle.Render("▸") + " " + label
		}
		rows = append(rows, row)
	}
	return strings.Join(rows, "\n")
}

func (m Model) renderFooter() string {
	var keys string
	switch m.mode {
	case editing:
		return ""
	case acting, sending:
		keys = "j k choose · enter apply · esc back"
	default:
		keys = "j k move · h l tab · space mark · enter act · a add · q quit"
		if !m.layer().isTray() {
			keys = "j k move · h l tab · space mark · enter act · a add · t take · q quit"
		}
	}
	footer := faintStyle.Render(" " + keys)
	if m.status != "" {
		footer = " " + cursorStyle.Render(m.status) + "\n" + footer
	}
	if m.width > 0 {
		footer = lipgloss.NewStyle().MaxWidth(m.width).Render(footer)
	}
	return footer + "\n"
}
