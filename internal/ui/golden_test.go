package ui

import (
	"fmt"
	"regexp"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/golden"

	"github.com/cheese-cracker/tray/internal/store"
)

// One golden per distinct screen, and no more. Decision 40 rejected golden frames
// because a suite full of them breaks on every restyle and proves nothing about what
// was written — which is still true, and is why nothing here asserts behaviour. What
// these catch is the class of bug that behaviour tests cannot see: a frame that lost
// its border, a column that stopped aligning, a footer that overflowed into `…`.
//
// Colour is stripped, so a CI runner that forces colour on doesn't rewrite them all.
// Regenerate with `go test ./internal/ui -run TestScreens -update`.

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)

func frame(t *testing.T, m tea.Model, presses ...string) {
	t.Helper()
	out, _ := m.Update(tea.WindowSizeMsg{Width: 84, Height: 20})
	shot(t, keys(out, presses...).(Model).View())
}

// shot records a frame with its colour stripped and its ids made stable. Ids are random
// and a golden must not be, so every id on screen is renamed to its row's insertion
// order — `id01` on — which keeps the column's width and says which row is which.
func shot(t *testing.T, view string) {
	t.Helper()
	view = ansiRe.ReplaceAllString(view, "")
	if rows, err := ts.Tasks(store.Filter{All: true}); err == nil {
		for i, r := range rows {
			view = strings.ReplaceAll(view, r.ID, fmt.Sprintf("id%02d", i+1))
		}
	}
	golden.RequireEqual(t, []byte(view))
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
		frame(t, New(ts))
	})

	t.Run("tray_empty", func(t *testing.T) {
		sandbox(t)
		frame(t, New(ts))
	})

	t.Run("tray_marked", func(t *testing.T) {
		sandbox(t, full...)
		frame(t, New(ts), " ", "j", " ")
	})

	t.Run("garage", func(t *testing.T) {
		sandbox(t)
		garage(t, "2026-08",
			"- ?? the billing page feels slow on first load",
			"- add metrics to the worker +infra",
			"- chase the landlord about the boiler",
		)
		frame(t, New(ts), "tab")
	})

	t.Run("action_menu", func(t *testing.T) {
		sandbox(t, full...)
		frame(t, New(ts), "enter")
	})

	t.Run("destinations", func(t *testing.T) {
		sandbox(t, full...)
		frame(t, New(ts), ">")
	})

	t.Run("rewrite_form", func(t *testing.T) {
		sandbox(t, full...)
		frame(t, New(ts), "r")
	})

	// `?` takes the whole screen, so it gets a golden of its own — and a second at a
	// size where the diagram has to be dropped.
	t.Run("help_page", func(t *testing.T) {
		sandbox(t, full...)
		out, _ := New(ts).Update(tea.WindowSizeMsg{Width: 80, Height: 26})
		shot(t, keys(out, "?").(Model).View())
	})

	t.Run("help_page_narrow", func(t *testing.T) {
		sandbox(t, full...)
		out, _ := New(ts).Update(tea.WindowSizeMsg{Width: 60, Height: 20})
		shot(t, keys(out, "?").(Model).View())
	})

	t.Run("filter_typing", func(t *testing.T) {
		sandbox(t, full...)
		frame(t, New(ts), "/", "i", "n", "v")
	})

	// A long list has to page inside the frame rather than push it past the
	// terminal — the jitter decision 35 exists to prevent.
	t.Run("long_list_pages", func(t *testing.T) {
		var lines []string
		for i := 0; i < 30; i++ {
			lines = append(lines, "- [ ] task "+string(rune('a'+i%26))+" priority:M")
		}
		sandbox(t, lines...)
		frame(t, New(ts))
	})

	// The sweep is its own screen: four month tabs, no tray, no `garage ·` prefix
	// repeated four times, opening on the current month.
	t.Run("sweep", func(t *testing.T) {
		sandbox(t)
		garage(t, "2026-07", "- left over from july", "- chase the deposit +chore")
		garage(t, "2026-08", "- dumped this month", "- another jotting +infra")
		frame(t, NewSweep(ts, ""))
	})

	// `v` is a mode: everything on the layer, live first, and only the rare verbs.
	t.Run("review", func(t *testing.T) {
		sandbox(t,
			"- [ ] Rotate the api keys priority:H due:2026-08-12 entry:2026-08-01 +infra",
			"- [x] ~~Renew the TLS certificate~~ priority:H entry:2026-08-02 done:2026-08-06",
			"- [ ] Chase the invoice priority:M due:2026-08-25 entry:2026-08-05 +admin",
			"- [x] ~~Book the flights~~ priority:L entry:2026-08-03 done:2026-08-05",
		)
		frame(t, New(ts), "v")
	})

	t.Run("review_menu", func(t *testing.T) {
		sandbox(t,
			"- [ ] Rotate the api keys priority:H entry:2026-08-01",
			"- [x] ~~Renew the TLS certificate~~ priority:H entry:2026-08-02 done:2026-08-06",
		)
		frame(t, New(ts), "v", "j", "enter")
	})

	t.Run("sync_review", func(t *testing.T) {
		sandbox(t, full...)
		installSync(t, "phone", `{"pull":[{"key":"r1","text":"Call the landlord","done":"","tags":["home"]},{"key":"r2","text":"Buy a kettle","done":""}],"push":[]}`)
		frame(t, New(ts), "S")
	})

	t.Run("month_picker", func(t *testing.T) {
		sandbox(t)
		shot(t, picker{months: pickable(), title: "unload the tray to", at: 1}.View())
	})

	// A task wider than the terminal must be truncated, not wrapped.
	t.Run("narrow_truncates", func(t *testing.T) {
		sandbox(t, "- [ ] a task with a very long description that will not fit priority:H +infra")
		out, _ := New(ts).Update(tea.WindowSizeMsg{Width: 46, Height: 12})
		shot(t, out.(Model).View())
	})
}
