// Package style is the palette, and nothing else. It exists because the terminal
// header and the window are drawn by different packages and must not drift apart.
//
// Every colour is adaptive. The light half is Modus Operandi Tinted and the dark half
// Modus Vivendi Tinted — one family, so the app on a dark desktop and `tray head` on a
// dark terminal wear the same scheme as the site and the rest of imarobot.
package style

import (
	"fmt"
	"image/color"

	"github.com/charmbracelet/lipgloss"
)

var (
	// Surfaces. A category the header never needed: it prints on your prompt's paper.
	Paper = lipgloss.AdaptiveColor{Light: "#fbf7f0", Dark: "#0d0e1c"} // the window
	Card  = lipgloss.AdaptiveColor{Light: "#efe9dd", Dark: "#1d2235"} // panes, cards, hover
	Line  = lipgloss.AdaptiveColor{Light: "#cfc8bb", Dark: "#4a4f69"} // rules, borders, hollow rungs
	Ink   = lipgloss.AdaptiveColor{Light: "#1a1a1a", Dark: "#ffffff"} // text
	Ink2  = lipgloss.AdaptiveColor{Light: "#595959", Dark: "#c6daff"} // captions, labels

	Accent     = lipgloss.AdaptiveColor{Light: "#0031a9", Dark: "#2fafff"} // focus, links, filled rungs
	AccentSoft = lipgloss.AdaptiveColor{Light: "#e2e3e9", Dark: "#1a2b56"} // selection, the cursor row, chips
	Subtle     = lipgloss.AdaptiveColor{Light: "#6b6b6b", Dark: "#989898"} // hints

	// The high-contrast pole, for whatever the eye should land on first. Accent is
	// already spent on the cursor bar and the mark, so the row under the cursor is
	// carried by contrast rather than hue.
	Strong = Ink

	// The frame in review mode, and the whole of how that mode announces itself
	// before you have read a word of it. Modus' warm "hot": the two verbs that live
	// there are a correction and a removal — not danger, but not the daily flow either.
	Review     = lipgloss.AdaptiveColor{Light: "#6f5500", Dark: "#fec43f"}
	ReviewSoft = lipgloss.AdaptiveColor{Light: "#eae4d3", Dark: "#3a2e00"}

	// Priority, warm to cool. These carry meaning, so they stay distinguishable from
	// each other — and from Review, which Medium sits next to on the wheel.
	High   = lipgloss.AdaptiveColor{Light: "#a60000", Dark: "#ff5f59"}
	Medium = lipgloss.AdaptiveColor{Light: "#884900", Dark: "#f0c526"}
	Low    = lipgloss.AdaptiveColor{Light: "#006800", Dark: "#44bc44"}

	// A date reads in two states and no more: it is on you now, or it is not yet.
	// These name a meaning rather than adding a hue — one red means urgent everywhere.
	Now   = High   // due today, or already past
	Later = Subtle // due after today
)

// Priority is the colour for H, M or L. An unset priority reads as medium but was
// never chosen, so it gets the quiet treatment rather than medium's.
func Priority(p string) lipgloss.AdaptiveColor {
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

// RGBA is the palette entry as the app draws it. One palette for the header and the
// window (78g, 102): the hex strings above are the truth and this only reads them.
func RGBA(c lipgloss.AdaptiveColor, dark bool) color.RGBA {
	hex := c.Light
	if dark {
		hex = c.Dark
	}
	var r, g, b uint8
	fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	return color.RGBA{R: r, G: g, B: b, A: 0xff}
}
