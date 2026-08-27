// Package style is the palette, and nothing else. It exists because the terminal
// header and the TUI are drawn by different packages and must not drift apart.
//
// Every colour is adaptive: the header prints above whatever prompt you already have,
// so it has to be legible on a light background as well as a dark one.
package style

import "github.com/charmbracelet/lipgloss"

var (
	Accent = lipgloss.AdaptiveColor{Light: "#874BFD", Dark: "#7D56F4"}
	Subtle = lipgloss.AdaptiveColor{Light: "#909090", Dark: "#626262"}

	// Priority, warm to cool. These carry meaning, so they stay distinguishable
	// from each other rather than being three shades of the accent.
	High   = lipgloss.AdaptiveColor{Light: "#D20F39", Dark: "#F38BA8"}
	Medium = lipgloss.AdaptiveColor{Light: "#FE640B", Dark: "#FAB387"}
	Low    = lipgloss.AdaptiveColor{Light: "#1E66F5", Dark: "#89B4FA"}
)

// Priority is the colour for H, M or L. An unset priority reads as medium but was
// never chosen, so it gets the quiet treatment rather than medium's.
func Priority(p string) lipgloss.TerminalColor {
	switch p {
	case "H":
		return High
	case "M":
		return Medium
	case "L":
		return Low
	default:
		return Subtle
	}
}
