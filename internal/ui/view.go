package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/cheese-cracker/tray/internal/style"
)

var (
	accent = style.Accent
	subtle = style.Subtle

	titleStyle  = lipgloss.NewStyle().Bold(true)
	faintStyle  = lipgloss.NewStyle().Foreground(subtle)
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
	markStyle   = lipgloss.NewStyle().Foreground(accent)
	keyStyle    = lipgloss.NewStyle().Bold(true).Foreground(accent)
	doneStyle   = lipgloss.NewStyle().Foreground(subtle).Strikethrough(true)

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
	b.WriteString("\n") // the tabs need air above them or the frame reads as clipped
	b.WriteString(tabs + "\n")
	b.WriteString(style.Render(body) + "\n")
	b.WriteString(footer)
	return b.String()
}

// rowRoom is how many task rows fit, leaving space for the chrome. Zero means the
// terminal hasn't told us its size yet, so nothing is clipped.
func (m Model) rowRoom() int {
	if m.height == 0 {
		return 0
	}
	// the top margin, tabs, the pane border, its padding, the column header, then the
	// footer as it renders — which grows when `?` is open or a status line is set.
	chrome := 1 + 3 + pane.GetVerticalBorderSize() + 2 + 1 // margin, tabs, border, padding, header
	chrome += lipgloss.Height(strings.TrimRight(m.renderFooter(), "\n"))
	if m.filtering() {
		chrome++ // the filter line takes a row from the list, not from the frame
	}
	switch m.mode {
	case acting:
		chrome += len(m.offered()) + 2
	case sending:
		chrome += len(m.dests) + 2
	}
	return max(1, m.height-chrome)
}

func (m Model) renderTabs() string {
	labels := make([]string, len(m.layers))
	width := 0
	for i, l := range m.layers {
		labels[i] = l.title
		// The prefix earns its place next to "tray" and nowhere else. In the sweep
		// every tab is a garage month, so it is four repetitions of the obvious —
		// and four prefixed labels do not fit an eighty-column terminal.
		if !l.isTray() && !m.sweep {
			labels[i] = "garage · " + l.title
		}
		width += lipgloss.Width(inactiveTab.Render(labels[i]))
	}
	// The tab row's bottom border is the pane's top edge. It only turns a corner
	// where the pane does — at the last tab if the tabs reach the edge, otherwise
	// it carries straight on into the gap.
	gap := m.width - width
	closes := gap <= 0

	var rendered []string
	for i, label := range labels {
		style := inactiveTab
		if i == m.active {
			style = activeTab
		}
		border, _, _, _, _ := style.GetBorder()
		if i == 0 {
			border.BottomLeft = "├"
			if i == m.active {
				border.BottomLeft = "│"
			}
		}
		if i == len(labels)-1 {
			switch {
			case i == m.active && closes:
				border.BottomRight = "│"
			case i == m.active:
				border.BottomRight = "└"
			case closes:
				border.BottomRight = "┤"
			default:
				border.BottomRight = "┴"
			}
		}
		rendered = append(rendered, style.Border(border).Render(label))
	}
	row := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	if gap > 0 {
		row = lipgloss.JoinHorizontal(lipgloss.Bottom, row,
			tabGap.Render(strings.Repeat(" ", gap)))
	}
	return row
}

func (m Model) renderBody() string {
	if m.form != nil {
		return m.form.view()
	}

	var b strings.Builder
	if m.filtering() {
		b.WriteString(m.renderFilter() + "\n")
	}
	switch {
	case len(m.list.Items()) == 0:
		b.WriteString(faintStyle.Render(m.emptyMessage()))
	case len(m.list.VisibleItems()) == 0:
		b.WriteString(faintStyle.Render("nothing here matches"))
	default:
		b.WriteString(m.deleg.header() + "\n")
		b.WriteString(m.list.View())
	}

	switch m.mode {
	case acting:
		b.WriteString("\n\n" + m.renderMenu())
	case sending:
		b.WriteString("\n\n" + m.renderDestinations())
	}
	return b.String()
}

// The filter is drawn here rather than by the list so it sits inside our frame, and
// so an applied filter keeps saying what it hid — a table that quietly shows four of
// forty rows is a table you will misread.
func (m Model) renderFilter() string {
	if m.list.SettingFilter() {
		return keyStyle.Render("/") + m.list.FilterInput.View()
	}
	return faintStyle.Render(fmt.Sprintf("/%s — %d of %d · esc clears",
		m.list.FilterValue(), len(m.list.VisibleItems()), len(m.list.Items())))
}

func (m Model) emptyMessage() string {
	if m.layer().isTray() {
		return "nothing on the tray — a to add one, or take something from the garage"
	}
	return "nothing here — a to dump a line"
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

// The footer and the `?` overlay are the same keymap rendered at two lengths, so
// neither can advertise a key the other doesn't.
func (m Model) renderFooter() string {
	if m.mode == editing {
		return "" // the form carries its own hint
	}
	h := m.help
	if m.width > 0 {
		h.Width = m.width - 1
	}
	footer := " " + h.View(m.keys())
	if m.status != "" {
		footer = " " + cursorStyle.Render(m.status) + "\n" + footer
	}
	if m.width > 0 {
		footer = lipgloss.NewStyle().MaxWidth(m.width).Render(footer)
	}
	return footer + "\n"
}
