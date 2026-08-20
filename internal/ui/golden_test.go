package ui

import (
	"regexp"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/golden"
)

// One golden per distinct screen, and no more. Decision 40 rejected golden frames
// because a suite full of them breaks on every restyle and proves nothing about what
// was written — which is still true, and is why nothing here asserts behaviour. What
// these catch is the class of bug that behaviour tests cannot see: a frame that lost
// its border, a column that stopped aligning, a footer that overflowed into `…`.
//
// Colour is stripped, so a CI runner that forces colour on doesn't rewrite them all.
// Regenerate with `go test ./internal/ui -run TestScreens -update`.

var ansi = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func frame(t *testing.T, m tea.Model, presses ...string) {
	t.Helper()
	out, _ := m.Update(tea.WindowSizeMsg{Width: 84, Height: 20})
	golden.RequireEqual(t, []byte(ansi.ReplaceAllString(keys(out, presses...).(Model).View(), "")))
}

func TestScreens(t *testing.T) {
	full := []string{
		"- [ ] Rotate the api keys priority:H due:2026-08-12 entry:2026-08-01 +infra",
		"- [ ] Book the flights priority:L due:2026-08-20 entry:2026-08-02",
		"- [ ] Review the deploy checklist priority:M entry:2026-08-03 +infra",
		"- [ ] Chase the invoice priority:M due:2026-08-25 entry:2026-08-05 +admin",
	}

	t.Run("tray", func(t *testing.T) {
		sandbox(t, full...)
		frame(t, New())
	})

	t.Run("tray_empty", func(t *testing.T) {
		sandbox(t)
		frame(t, New())
	})

	t.Run("tray_marked", func(t *testing.T) {
		sandbox(t, full...)
		frame(t, New(), " ", "j", " ")
	})

	t.Run("garage", func(t *testing.T) {
		sandbox(t)
		garage(t, "2026-08",
			"- ?? that config thing — does it even matter now",
			"- add metrics to the worker +infra",
			"- chase the landlord about the boiler",
		)
		frame(t, New(), "l")
	})

	t.Run("action_menu", func(t *testing.T) {
		sandbox(t, full...)
		frame(t, New(), "enter")
	})

	t.Run("destinations", func(t *testing.T) {
		sandbox(t, full...)
		frame(t, New(), ">")
	})

	t.Run("retake_form", func(t *testing.T) {
		sandbox(t, full...)
		frame(t, New(), "r")
	})

	t.Run("help_overlay", func(t *testing.T) {
		sandbox(t, full...)
		frame(t, New(), "?")
	})

	t.Run("filter_typing", func(t *testing.T) {
		sandbox(t, full...)
		frame(t, New(), "/", "i", "n", "v")
	})

	// A long list has to page inside the frame rather than push it past the
	// terminal — the jitter decision 35 exists to prevent.
	t.Run("long_list_pages", func(t *testing.T) {
		var lines []string
		for i := 0; i < 30; i++ {
			lines = append(lines, "- [ ] task "+string(rune('a'+i%26))+" priority:M")
		}
		sandbox(t, lines...)
		frame(t, New())
	})

	// A task wider than the terminal must be truncated, not wrapped.
	t.Run("narrow_truncates", func(t *testing.T) {
		sandbox(t, "- [ ] a task with a very long description that will not fit priority:H +infra")
		out, _ := New().Update(tea.WindowSizeMsg{Width: 46, Height: 12})
		golden.RequireEqual(t, []byte(ansi.ReplaceAllString(out.(Model).View(), "")))
	})
}
