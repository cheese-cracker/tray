// Package plugin finds the third-party garage plugins installed under the tray home.
// It knows where they are and whether they look runnable; it never runs one, and it
// never reads a garage — a plugin garage is an ordinary markdown file, so that stays
// store's job and this package stays free of the grammar (16).
package plugin

import (
	"os"
	"path/filepath"

	"github.com/cheese-cracker/tray/internal/store"
)

// Runner is the only filename this package knows. There is no manifest: a plugin
// called notion owns notion.md and is run by plugins/notion/run, so both facts are
// the folder name and there is nothing to declare and nothing to parse. That is 18's
// argument — the vocabulary is whatever is already in use — one level down.
const Runner = "run"

// A Plugin is a directory. Name is the folder, which is also the garage it owns.
type Plugin struct {
	Name string
	Dir  string
	Run  string
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

// List is every installed plugin, in name order. A folder without a runnable `run` is
// not a plugin but half an install, and counting it would promise a sync that cannot
// happen. No plugins at all is the usual case and is not an error.
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
		run := filepath.Join(dir, Runner)
		if !runnable(run) {
			continue
		}
		found = append(found, Plugin{Name: e.Name(), Dir: dir, Run: run})
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
