package ui

import (
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"

	"github.com/cheese-cracker/tray/internal/style"
)

// A due date reads in two states: on you now, or not yet. Sandbox time is 2026-08-07.
func TestTheDueColumnHasTwoStates(t *testing.T) {
	was := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(was)

	sandbox(t,
		"- [ ] gone by priority:H due:2026-08-01",
		"- [ ] owed today priority:H due:2026-08-07",
		"- [ ] not yet priority:H due:2026-09-20",
		"- [ ] no date at all priority:H",
	)
	out, _ := New().Update(tea.WindowSizeMsg{Width: 84, Height: 20})
	view := out.(Model).View()

	paint := func(c lipgloss.TerminalColor) string {
		m := regexp.MustCompile(`\[38;2;[0-9;]+`).
			FindString(lipgloss.NewStyle().Foreground(c).Render("x"))
		return m
	}
	now, later := paint(style.Now), paint(style.Later)
	if now == later {
		t.Fatal("the two states must be distinguishable")
	}

	for _, c := range []struct {
		row  string
		want string
		name string
	}{
		{"Sat Aug 1", now, "already past"},
		{"Fri Aug 7", now, "today"},
		{"Sun Sep 20", later, "not yet"},
	} {
		var line string
		for _, l := range strings.Split(view, "\n") {
			if strings.Contains(regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(l, ""), c.row) {
				line = l
			}
		}
		if line == "" {
			t.Fatalf("no row rendered for %s:\n%s", c.row, view)
		}
		if !strings.Contains(line, c.want) {
			t.Errorf("%s (%s) is not painted %q:\n%q", c.name, c.row, c.want, line)
		}
	}
}

// A finished row is dull throughout — the colour is what says "done", and a red due
// date on a line you already closed is shouting about nothing.
func TestAFinishedRowKeepsNoDueColour(t *testing.T) {
	was := lipgloss.ColorProfile()
	lipgloss.SetColorProfile(termenv.TrueColor)
	defer lipgloss.SetColorProfile(was)

	sandbox(t, "- [x] ~~long done~~ priority:H due:2026-08-01 done:2026-08-02")
	out, _ := New().Update(tea.WindowSizeMsg{Width: 84, Height: 20})
	view := keys(out, "v").(Model).View()

	now := regexp.MustCompile(`\[38;2;[0-9;]+`).
		FindString(lipgloss.NewStyle().Foreground(style.Now).Render("x"))
	for _, l := range strings.Split(view, "\n") {
		plain := regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(l, "")
		if strings.Contains(plain, "long done") && strings.Contains(l, now) {
			t.Errorf("a finished row should carry no due colour:\n%q", l)
		}
	}
}
