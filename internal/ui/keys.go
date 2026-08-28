package ui

import "github.com/charmbracelet/bubbles/key"

// keyMap is what the footer renders and what `?` expands. Both read the same value,
// so the one-line hint and the help overlay cannot drift apart — which is the only
// reason a help screen is worth having when the footer already carries the keymap.
//
// These bindings are for display. The list's own keymap does the matching for
// movement and filtering; everything else is matched by updateList.
type keyMap struct {
	short []key.Binding
	full  [][]key.Binding
}

func (k keyMap) ShortHelp() []key.Binding  { return k.short }
func (k keyMap) FullHelp() [][]key.Binding { return k.full }

func bind(keys, help string) key.Binding {
	return key.NewBinding(key.WithKeys(keys), key.WithHelp(keys, help))
}

func (m Model) keys() keyMap {
	if m.mode == acting || m.mode == sending {
		picking := []key.Binding{bind("↑↓", "choose"), bind("enter", "apply"), bind("esc", "back")}
		return keyMap{short: picking, full: [][]key.Binding{picking}}
	}

	// The footer teaches the arrows, because they are the keys someone opening this
	// for the first time will already try. The vim aliases all work and are named in
	// `?` — a footer that lists two ways to do one thing teaches neither.
	//
	// Every key it names is on screen: the footer wraps rather than truncating.
	short := []key.Binding{
		bind("↑↓", "move"), bind("space", "select"), bind("←→", "tab"),
		bind("enter", "act"), bind("a", "add"),
	}
	if !m.layer().isTray() {
		short = append(short, bind("t", "take"))
	}
	done := bind("v", "show done")
	if m.showDone {
		done = bind("v", "hide done")
	}
	short = append(short, done, bind("/", "filter"), bind("?", "help"), bind("q", "quit"))

	// The action letters come from the layer, so the overlay teaches exactly the
	// menu you would have got from enter.
	var acts []key.Binding
	for _, a := range m.offered() {
		acts = append(acts, bind(a.key, a.label))
	}

	rest := []key.Binding{bind("/", "filter")}
	if m.filtering() {
		rest = append(rest, bind("esc", "clear filter"))
	}
	rest = append(rest, bind("?", "help"), bind("q", "quit"))

	return keyMap{short: short, full: [][]key.Binding{short, acts, rest}}
}
