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
		frame(t, New(), "tab")
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

	// `?` is a page in its own right, so it gets a screen of its own.
	t.Run("help_page", func(t *testing.T) {
		sandbox(t, full...)
		out, _ := New().Update(tea.WindowSizeMsg{Width: 80, Height: 26})
		golden.RequireEqual(t, []byte(ansi.ReplaceAllString(
			keys(out, "?").(Model).View(), "")))
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

	// The sweep is its own screen: four month tabs, no tray, no `garage ·` prefix
	// repeated four times, opening on the current month.
	t.Run("sweep", func(t *testing.T) {
		sandbox(t)
		garage(t, "2026-07", "- left over from july", "- chase the deposit +chore")
		garage(t, "2026-08", "- dumped this month", "- another jotting +infra")
		frame(t, NewSweep(""))
	})

	// `v` reveals what you finished. It is the only way to reach a restore.
	t.Run("showing_done", func(t *testing.T) {
		sandbox(t,
			"- [ ] Rotate the api keys priority:H due:2026-08-12 entry:2026-08-01 +infra",
			"- [x] ~~Renew the passport~~ priority:H entry:2026-08-02 done:2026-08-06",
			"- [ ] Chase the invoice priority:M due:2026-08-25 entry:2026-08-05 +admin",
			"- [x] ~~Book the flights~~ priority:L entry:2026-08-03 dropped:2026-08-05",
		)
		frame(t, New(), "v")
	})

	t.Run("done_row_menu", func(t *testing.T) {
		sandbox(t,
			"- [ ] Rotate the api keys priority:H entry:2026-08-01",
			"- [x] ~~Renew the passport~~ priority:H entry:2026-08-02 done:2026-08-06",
		)
		frame(t, New(), "v", "j", "enter")
	})

	t.Run("month_picker", func(t *testing.T) {
		sandbox(t)
		golden.RequireEqual(t, []byte(ansi.ReplaceAllString(
			picker{months: pickable(), title: "unload the tray to", at: 1}.View(), "")))
	})

	// A task wider than the terminal must be truncated, not wrapped.
	t.Run("narrow_truncates", func(t *testing.T) {
		sandbox(t, "- [ ] a task with a very long description that will not fit priority:H +infra")
		out, _ := New().Update(tea.WindowSizeMsg{Width: 46, Height: 12})
		golden.RequireEqual(t, []byte(ansi.ReplaceAllString(out.(Model).View(), "")))
	})
}
