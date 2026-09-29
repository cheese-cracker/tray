package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"

	"github.com/cheese-cracker/tray/internal/store"
)

// Run opens the window and returns when it closes.
func Run(s *store.Store) error {
	a := app.New()
	a.Settings().SetTheme(newTheme())
	w := a.NewWindow("tray")
	u := newUI(s, w)
	w.SetContent(u.root)
	w.Resize(fyne.NewSize(960, 620))
	u.focusList()
	w.ShowAndRun()
	return nil
}
