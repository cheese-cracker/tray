package gui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/plugin"
	"github.com/cheese-cracker/tray/internal/store"
	"github.com/cheese-cracker/tray/internal/style"
	"github.com/cheese-cracker/tray/internal/sync"
)

// pluginsPane is what is installed and how each one is doing: the garage it keeps, how
// its last run went, whether it runs at launch, the settings its example names, and a
// copy of it for another site. Nothing here interprets a setting (T7).
type pluginsPane struct {
	u   *ui
	box *fyne.Container
}

func (u *ui) openPlugins() {
	pane := &pluginsPane{u: u, box: container.NewVBox()}
	accent := style.RGBA(style.Accent, dark())
	p := newPage(container.NewBorder(banner("plugins — esc leaves", accent), u.bottom, nil, nil,
		container.NewVScroll(pane.box)))
	p.onKey = func(k *fyne.KeyEvent) {
		if k.Name == fyne.KeyEscape {
			u.leave()
		}
	}
	p.onRune = func(r rune) {
		if r == 'q' {
			u.quit()
		}
	}
	u.pane, u.page = pane, p
	pane.fill()
	u.enter(modePlugins, p)
}

func (pane *pluginsPane) fill() {
	pane.box.Objects = nil
	found := plugin.List()
	runs, err := pane.u.s.Runs()
	if err != nil {
		pane.u.fail(err)
	}
	if len(found) == 0 {
		pane.box.Add(widget.NewLabel("no plugins — one is a folder in " + plugin.Dir() +
			" holding an executable `" + plugin.SyncFile + "` or a verb under `" + plugin.ActionsDir + "/`"))
	}
	for _, p := range found {
		pane.box.Add(pane.card(p, runs[p.Name]))
	}
	pane.box.Refresh()
}

func (pane *pluginsPane) card(p plugin.Plugin, last store.Run) fyne.CanvasObject {
	u := pane.u
	var parts []string
	if p.Sync != "" {
		rows, err := u.s.Tasks(store.Filter{Layer: core.LayerGarage, Month: p.Garage(), All: true})
		if err != nil {
			u.fail(err)
		}
		parts = append(parts, fmt.Sprintf("garage %d rows", len(rows)))
	}
	if len(p.Verbs) > 0 {
		parts = append(parts, "enter → "+strings.Join(p.Verbs, ", "))
	}
	lines := container.NewVBox()
	if p.Sync != "" {
		lines.Add(lastRun(last))
		launch := widget.NewCheck("run at launch", func(on bool) {
			if err := setOnLaunch(p, on); err != nil {
				u.fail(err)
			}
		})
		launch.Checked = p.OnLaunch
		lines.Add(container.NewHBox(launch, lowButton("run now", func() { pane.run(p.Name) })))
	}
	if len(p.SettingsKeys()) > 0 {
		lines.Add(container.NewHBox(
			lowButton("settings…", func() { pane.settings(p) }),
			lowButton("duplicate…", func() { pane.duplicate(p) })))
	}
	return widget.NewCard(p.Name, strings.Join(parts, " · "), lines)
}

func lastRun(last store.Run) fyne.CanvasObject {
	switch {
	case last.Name == "":
		return grey("never run")
	case last.OK:
		return grey(fmt.Sprintf("last %s %s ok — %s", last.Hook, last.At, last.Message))
	}
	return warn(fmt.Sprintf("last %s %s failed — %s", last.Hook, last.At, last.Message))
}

// setOnLaunch writes or removes the marker: the folder states the fact (T8).
func setOnLaunch(p plugin.Plugin, on bool) error {
	path := filepath.Join(p.Dir, plugin.OnLaunchMarker)
	if on {
		return os.WriteFile(path, nil, 0o644)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// run plans one plugin now and opens the review for it alone; no tick, no others.
func (pane *pluginsPane) run(name string) {
	u := pane.u
	u.runSync(func() (sync.Summary, []sync.Result, error) {
		results, err := sync.Plans(u.s, sync.Manual, name, syncTimeout)
		return sync.Summary{}, results, err
	}, u.openSyncReview)
}

// settings is the form the plugin's example names, filled with what the file holds.
func (pane *pluginsPane) settings(p plugin.Plugin) {
	u := pane.u
	current := p.Settings()
	keys := p.SettingsKeys()
	f := widget.NewForm()
	entries := make([]*escEntry, len(keys))
	for i, k := range keys {
		e := newEscEntry(u.hide)
		e.SetText(current[k])
		entries[i] = e
		f.Append(k, e)
	}
	f.OnSubmit = func() {
		values := map[string]string{}
		for i, k := range keys {
			values[k] = entries[i].Text
		}
		if err := p.SetSettings(values); err != nil {
			u.fail(err)
			return
		}
		u.hide()
		u.say("settings saved for " + p.Name)
	}
	f.OnCancel = u.hide
	f.SubmitText = "save"
	var focus fyne.Focusable
	if len(entries) > 0 {
		focus = entries[0]
	}
	u.show(container.NewVBox(widget.NewLabelWithStyle(p.Name+" settings", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}), f), focus)
}

// duplicate copies the folder under a new name: a copy is an install, and the settings,
// evidence and log stay behind because they were the original's (T7).
func (pane *pluginsPane) duplicate(p plugin.Plugin) {
	u := pane.u
	name := newEscEntry(u.hide)
	name.SetPlaceHolder("a name for the copy, e.g. web-linear")
	name.OnSubmitted = func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || strings.ContainsAny(s, "/ ") {
			return
		}
		dst := filepath.Join(plugin.Dir(), s)
		if _, err := os.Stat(dst); err == nil {
			u.fail(fmt.Errorf("%s already exists", s))
			return
		}
		if err := os.CopyFS(dst, os.DirFS(p.Dir)); err != nil {
			u.fail(err)
			return
		}
		os.Remove(filepath.Join(dst, plugin.SettingsFile))
		os.Remove(filepath.Join(dst, plugin.LogFile))
		os.RemoveAll(filepath.Join(dst, plugin.EvidenceDir))
		u.hide()
		pane.fill()
		if copy, ok := plugin.Find(s); ok && len(copy.SettingsKeys()) > 0 {
			pane.settings(copy)
		}
	}
	f := widget.NewForm(widget.NewFormItem("copy "+p.Name+" as", name))
	f.OnSubmit = func() { name.OnSubmitted(name.Text) }
	f.OnCancel = u.hide
	u.show(f, name)
}
