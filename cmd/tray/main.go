package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/x/term"

	"github.com/cheese-cracker/tray/internal/cli"
	"github.com/cheese-cracker/tray/internal/gui"
	"github.com/cheese-cracker/tray/internal/store"
)

func main() {
	if len(os.Args) == 1 && wantsWindow() {
		os.Exit(window())
	}
	os.Exit(cli.Run(os.Args[1:]))
}

// wantsWindow is bare tray at a terminal that has somewhere to draw. Piped, or with
// nothing to draw on, it stays text — an agent must never be handed a UI (20).
func wantsWindow() bool {
	return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()) &&
		(os.Getenv("DISPLAY") != "" || os.Getenv("WAYLAND_DISPLAY") != "")
}

func window() int {
	s, err := store.Open(store.Home())
	if err != nil {
		fmt.Fprintln(os.Stderr, "tray: "+err.Error())
		return 2
	}
	defer s.Close()
	if err := gui.Run(s); err != nil {
		fmt.Fprintln(os.Stderr, "tray: "+err.Error())
		return 2
	}
	return 0
}
