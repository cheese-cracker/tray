package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cheese-cracker/tray/internal/store"
)

// PickMonth asks where the tray should land. Unload never guesses the month, and the
// CLI can never prompt — so this is the terminal half of that rule, and the reason
// bare `tray unload` is an error when anything is piped.
//
// It returns "" if you back out, which unload reports as cancelled rather than
// treating as a default.
func PickMonth() (string, error) {
	m := picker{months: pickable(), title: "unload the tray to"}
	out, err := tea.NewProgram(m).Run()
	if err != nil {
		return "", err
	}
	return out.(picker).chosen, nil
}

// The months worth offering: the one that is closing, the one you are in, the one
// coming, and someday. Anything older is a hand-edit or an explicit --to.
func pickable() []layer {
	this := store.ThisMonth()
	return []layer{
		{title: monthTitle(store.PrevMonth(this)), month: store.PrevMonth(this)},
		{title: monthTitle(this), month: this},
		{title: monthTitle(store.NextMonth(this)), month: store.NextMonth(this)},
		{title: store.Someday, month: store.Someday},
	}
}

type picker struct {
	months []layer
	title  string
	at     int
	chosen string
}

func (p picker) Init() tea.Cmd { return nil }

func (p picker) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	key, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	switch key.String() {
	case "q", "esc", "ctrl+c":
		return p, tea.Quit
	case "j", "down":
		if p.at < len(p.months)-1 {
			p.at++
		}
	case "k", "up":
		if p.at > 0 {
			p.at--
		}
	case "enter":
		p.chosen = p.months[p.at].month
		return p, tea.Quit
	}
	return p, nil
}

func (p picker) View() string {
	var b strings.Builder
	b.WriteString("\n  " + titleStyle.Render(p.title) + "\n\n")
	for i, l := range p.months {
		row := "    " + l.title
		if i == p.at {
			row = "  " + cursorStyle.Render("▸") + " " + titleStyle.Render(l.title)
		}
		b.WriteString(row + "\n")
	}
	b.WriteString("\n" + faintStyle.Render("  j k choose · enter unload · esc cancel") + "\n")
	return b.String()
}
