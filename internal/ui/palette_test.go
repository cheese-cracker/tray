package ui

import (
	"fmt"
	"regexp"
	"sort"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/cheese-cracker/tray/internal/style"
)

// Every colour on screen comes from internal/style. bubbles ships its own — a pink
// filter caret, a green filter prompt, three greys in the help — and they are absent
// only because start() overwrites them, which is exactly what regresses unnoticed.
//
// This walks the screens rather than reading the source, because the colours were never
// wrong in our code. They were missing from it, and the widget supplied its own.
func TestEveryColourComesFromThePalette(t *testing.T) {
	was := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(was)

	rgb := regexp.MustCompile(`[34]8;2;(\d+;\d+;\d+)`)
	allowed := map[string]bool{}
	for _, c := range []lipgloss.TerminalColor{
		style.Accent, style.Subtle, style.Strong, style.Review,
		style.High, style.Medium, style.Low,
	} {
		for _, m := range rgb.FindAllStringSubmatch(
			lipgloss.NewStyle().Foreground(c).Render("x"), -1) {
			allowed[m[1]] = true
		}
	}

	sandbox(t, "- [ ] a thing priority:H due:2026-08-12 +infra")
	base, _ := New().Update(tea.WindowSizeMsg{Width: 84, Height: 20})

	for name, view := range map[string]string{
		"list":   base.(Model).View(),
		"filter": keys(base, "/", "a").(Model).View(),
		"form":   keys(base, "r").(Model).View(),
		"menu":   keys(base, "enter").(Model).View(),
		"review": keys(base, "v").(Model).View(),
		"help":   keys(base, "?").(Model).View(),
	} {
		var stray []string
		for _, m := range rgb.FindAllStringSubmatch(view, -1) {
			if !allowed[m[1]] {
				stray = append(stray, m[1])
			}
		}
		if len(stray) > 0 {
			sort.Strings(stray)
			t.Errorf("the %s screen uses colours that are not in the palette: %s",
				name, fmt.Sprint(stray))
		}
	}
}
