package ui

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
)

// Decision 41 abandoned pty scraping because it hung twice. teatest is not a pty,
// but it can still hang in exactly one place: WaitFinished with no timeout blocks on
// a channel forever. So no test calls teatest directly — everything goes through this
// harness, and every wait it performs is bounded:
//
//	press/typeIn   send a key, cannot block
//	waitFor        teatest.WaitFor, always given WithDuration
//	final          WaitFinished, always given WithFinalTimeout
//	Cleanup        quits the program even if the test never did
//
// `go test -timeout` in the Makefile is the backstop under all of that.
const patience = 3 * time.Second

// tui drives the real bubbletea program. Assertions belong on the final model and on
// the files it wrote — decision 40 — so waitFor is for synchronising before the next
// keystroke, not for checking that something is true.
type tui struct {
	t    *testing.T
	tm   *teatest.TestModel
	seen bytes.Buffer // teatest drains its reader, so frames are kept here

	done  bool
	model Model
}

func drive(t *testing.T, m tea.Model) *tui {
	t.Helper()
	u := &tui{t: t, tm: teatest.NewTestModel(t, m, teatest.WithInitialTermSize(90, 24))}
	t.Cleanup(func() { u.quit() })
	return u
}

func (u *tui) press(presses ...string) *tui {
	u.t.Helper()
	for _, k := range presses {
		u.tm.Send(keyMsg(k))
	}
	return u
}

func (u *tui) typeIn(s string) *tui {
	u.t.Helper()
	for _, r := range s {
		u.tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return u
}

// paste is what the terminal sends for ctrl+shift+v: one message, every rune at
// once, Paste set. bubbletea turns bracketed paste into exactly this.
func (u *tui) paste(s string) *tui {
	u.t.Helper()
	u.tm.Send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s), Paste: true})
	return u
}

// waitFor blocks until the given text has appeared in some frame, or fails the test.
// It can never block longer than `patience`.
//
// Keystrokes do not need it: they go down the same message channel as everything
// else, so the program handles them in the order they were sent, and Quit is just
// another message behind them. Use it where a *command* is in the way — the filter
// re-runs asynchronously — and where a named frame makes a failure readable.
//
// It matches against every frame so far, not only the newest, because bubbletea
// rewrites changed lines only: text that is already on screen may never be sent
// again.
func (u *tui) waitFor(want string) *tui {
	u.t.Helper()
	teatest.WaitFor(u.t, io.TeeReader(u.tm.Output(), &u.seen), func([]byte) bool {
		return strings.Contains(u.seen.String(), want)
	}, teatest.WithDuration(patience), teatest.WithCheckInterval(10*time.Millisecond))
	return u
}

// final quits the program and hands back the model it ended on.
func (u *tui) final() Model {
	u.t.Helper()
	u.quit()
	return u.model
}

func (u *tui) quit() {
	if u.done {
		return
	}
	u.done = true
	_ = u.tm.Quit()
	u.model, _ = u.tm.FinalModel(u.t, teatest.WithFinalTimeout(patience)).(Model)
}
