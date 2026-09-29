// Package plugin finds the third-party plugins installed under the tray home. It
// knows where they are, whether they look runnable, and which verbs they offer; it
// never runs one itself, and it never reads a garage — a plugin garage is an ordinary
// markdown file, so that stays store's job and this package stays free of the
// grammar (16).
package plugin

import (
	"os"
	"path/filepath"

	"github.com/cheese-cracker/tray/internal/store"
)

// Runner is the syncer's filename. There is no manifest: a plugin called notion owns
// notion.md and is run by plugins/notion/run, so both facts are the folder name and
// there is nothing to declare and nothing to parse. That is 18's argument — the
// vocabulary is whatever is already in use — one level down.
const Runner = "run"

// ActionsDir holds a plugin's menu verbs, one executable each, named after the verb:
// plugins/gcal/actions/schedule is the row "schedule" in the enter menu. The same
// rule as Runner, one level further down (105).
const ActionsDir = "actions"

// A Plugin is a directory. Name is the folder, which is also the garage it owns.
// A folder counts once it has a runnable Runner or at least one verb.
type Plugin struct {
	Name  string
	Dir   string
	Run   string   // "" when the plugin keeps no garage
	Verbs []string // the executables under ActionsDir, in name order
}

// An Action is one verb the interface can offer. tray hands the executable the picked
// lines as arguments and the layer they sit on in TRAY_LAYER, and otherwise knows
// nothing about what it does.
type Action struct {
	Plugin string
	Verb   string
	Path   string
}

// Garage is the layer this plugin keeps. It equals Name, spelled out because the two
// being the same string is a decision and not a coincidence.
func (p Plugin) Garage() string { return p.Name }

// Path is the markdown file the plugin writes and tray reads. MonthPath already
// serves a name that is not a month — `someday` is the precedent — so a plugin
// garage needs no new path rule.
func (p Plugin) Path() string { return store.MonthPath(p.Name) }

// Dir is where plugins live: beside the garage files they keep, not under a dot
// directory, for the reason 53 gives about the data itself.
func Dir() string { return filepath.Join(store.Home(), "plugins") }

// List is every installed plugin, in name order. A folder with neither a runnable
// `run` nor a verb is not a plugin but half an install, and counting it would promise
// something that cannot happen. No plugins at all is the usual case and is not an
// error.
func List() []Plugin {
	entries, err := os.ReadDir(Dir()) // already name-sorted
	if err != nil {
		return nil
	}
	var found []Plugin
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(Dir(), e.Name())
		p := Plugin{Name: e.Name(), Dir: dir, Verbs: verbs(dir)}
		if run := filepath.Join(dir, Runner); runnable(run) {
			p.Run = run
		}
		if p.Run == "" && len(p.Verbs) == 0 {
			continue
		}
		found = append(found, p)
	}
	return found
}

func Find(name string) (Plugin, bool) {
	for _, p := range List() {
		if p.Name == name {
			return p, true
		}
	}
	return Plugin{}, false
}

// Actions is every verb every plugin offers, in plugin then verb order.
func Actions() []Action {
	var out []Action
	for _, p := range List() {
		for _, v := range p.Verbs {
			out = append(out, Action{Plugin: p.Name, Verb: v, Path: filepath.Join(p.Dir, ActionsDir, v)})
		}
	}
	return out
}

// verbs are the runnable files under actions/. The exec bit is the same consent it is
// for run: a verb you have not marked is not yet one tray may put in front of you.
func verbs(dir string) []string {
	entries, err := os.ReadDir(filepath.Join(dir, ActionsDir))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if runnable(filepath.Join(dir, ActionsDir, e.Name())) {
			out = append(out, e.Name())
		}
	}
	return out
}

// runnable checks the exec bit rather than assuming it. tray executes this file, so a
// plugin you have not marked executable is one you have not finished installing — and
// marking it is the only consent tray gets to ask for.
func runnable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}
