package ui

import (
	"os"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/plugin"
)

// A plugin verb is a row of the enter menu with no letter of its own: the letters are
// tray's, and a verb that arrived by being installed is reached by enter, then a
// choice (105a). It is offered on both layers — a reminder is as much for a tray task
// as for a jotted one — and never in review, which has its own two verbs (92c).
func pluginActions() []action {
	var out []action
	for _, a := range plugin.Actions() {
		out = append(out, action{label: a.Verb, tray: true, rest: true, apply: handoff(a)})
	}
	return out
}

// handoff gives the verb the terminal for as long as it runs. It is an interface of
// its own — a prompt, a picker, a browser consent — and tray has nothing to say while
// it is up. What it did to the files is what the reload shows when it is back, and
// tray says nothing else (105b).
func handoff(a plugin.Action) func(*Model, []core.Task) string {
	return func(m *Model, picked []core.Task) string {
		m.exec = tea.ExecProcess(command(a, arrow(m.layer()), picked), func(err error) tea.Msg {
			return pluginDone{a, err}
		})
		return ""
	}
}

// command is the whole contract: the picked lines as arguments, the layer they sit
// on in TRAY_LAYER — `tray`, or a month — and the terminal for everything else.
func command(a plugin.Action, layer string, picked []core.Task) *exec.Cmd {
	args := make([]string, 0, len(picked))
	for _, t := range picked {
		args = append(args, t.Text)
	}
	cmd := exec.Command(a.Path, args...)
	cmd.Env = append(os.Environ(), "TRAY_LAYER="+layer)
	return cmd
}

// pluginDone is the terminal coming back. Only a failure is worth a line.
type pluginDone struct {
	action plugin.Action
	err    error
}
