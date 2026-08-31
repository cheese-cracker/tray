package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/cheese-cracker/tray/internal/style"
)

var (
	accent     = style.Accent
	subtle     = style.Subtle
	strong     = style.Strong
	reviewEdge = style.Review

	titleStyle  = lipgloss.NewStyle().Bold(true).Foreground(strong)
	faintStyle  = lipgloss.NewStyle().Foreground(subtle)
	cursorStyle = lipgloss.NewStyle().Bold(true).Foreground(accent)
	markStyle   = lipgloss.NewStyle().Foreground(accent)
	keyStyle    = lipgloss.NewStyle().Bold(true).Foreground(accent)
	doneStyle   = lipgloss.NewStyle().Foreground(subtle).Strikethrough(true)

	// A finished row under the cursor: still struck, no longer dull. It lifts to the
	// terminal's own foreground rather than to strong, because a done task should
	// never read louder than the live ones around it.
	doneCursorStyle = doneStyle.Foreground(lipgloss.NoColor{})

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

// frameColor is what the border and the tab row draw in. It is the one thing that
// says "you are not on the daily screen" without spending a word on saying it.
func (m Model) frameColor() lipgloss.TerminalColor {
	if m.viewing {
		return reviewEdge
	}
	return accent
}

func (m Model) View() string {
	if m.err != nil {
		return "tray: " + m.err.Error() + "\n"
	}
	if m.help.ShowAll {
		return "\n  " + strings.ReplaceAll(m.helpScreen(), "\n", "\n  ") + "\n"
	}

	tabs := m.renderTabs()
	body := m.renderBody()
	footer := m.renderFooter()

	frame := pane.BorderForeground(m.frameColor())
	style := frame
	switch {
	case m.width > 0:
		// Fill the terminal. lipgloss counts padding inside Width and Height but adds
		// borders outside, so only the border is subtracted here.
		style = frame.Width(m.width - pane.GetHorizontalBorderSize())
		if fill := m.height - lipgloss.Height(tabs) - lipgloss.Height(footer) -
			pane.GetVerticalBorderSize(); fill > 0 {
			style = style.Height(fill)
		}
	case lipgloss.Width(tabs)-pane.GetHorizontalFrameSize() > lipgloss.Width(body):
		// No size yet: widen to meet the tab bar, but never narrow below the content.
		style = frame.Width(lipgloss.Width(tabs) - pane.GetHorizontalFrameSize())
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
	if m.viewing {
		chrome++ // and so does the line naming the done view
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
	edge := m.frameColor()
	inactiveTab := inactiveTab.BorderForeground(edge)
	activeTab := activeTab.BorderForeground(edge)
	tabGap := tabGap.BorderForeground(edge)

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
	// Review mode changes both what is listed and what the keys do, so it says so
	// above the table. The tabs still name the layer; this names the mode within it —
	// and names the way out, because a mode you cannot see the exit from is a trap.
	if m.viewing {
		b.WriteString(cursorStyle.Render("review mode") +
			faintStyle.Render("  ·  ") + keyStyle.Render("v/esc") +
			faintStyle.Render(" to leave") + "\n")
	}
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

// shortHelp wraps instead of truncating. bubbles/help cuts the line off with an
// ellipsis, which silently hides whichever keys sort last — and on an eighty-column
// terminal that is most of them.
func (m Model) shortHelp() string {
	room := 78
	if m.width > 2 {
		room = m.width - 2
	}
	sep := m.help.Styles.ShortSeparator.Render(m.help.ShortSeparator)
	var lines []string
	line := ""
	for _, b := range m.keys().ShortHelp() {
		item := m.help.Styles.ShortKey.Render(b.Help().Key) + " " +
			m.help.Styles.ShortDesc.Render(b.Help().Desc)
		switch {
		case line == "":
			line = item
		case lipgloss.Width(line)+lipgloss.Width(sep)+lipgloss.Width(item) <= room:
			line += sep + item
		default:
			lines = append(lines, line)
			line = item
		}
	}
	return strings.Join(append(lines, line), "\n ")
}

func (m Model) emptyMessage() string {
	if m.viewing {
		return "nothing on this layer at all — v goes back"
	}
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

// The footer and the `?` screen are the same keymap rendered at two lengths, so
// neither can advertise a key the other doesn't.
func (m Model) renderFooter() string {
	if m.mode == editing {
		return "" // the form carries its own hint
	}
	footer := " " + m.shortHelp()
	if m.status != "" {
		footer = " " + cursorStyle.Render(m.status) + "\n" + footer
	}
	if m.width > 0 {
		footer = lipgloss.NewStyle().MaxWidth(m.width).Render(footer)
	}
	return footer + "\n"
}
