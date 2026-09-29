package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/plugin"
)

func installVerb(t *testing.T, name, verb string) {
	t.Helper()
	dir := filepath.Join(os.Getenv("TRAY_HOME"), "plugins", name, plugin.ActionsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, verb), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
}

func offers(m Model, label string) bool {
	for _, a := range m.offered() {
		if a.label == label {
			return true
		}
	}
	return false
}

// A verb an installed plugin adds sits in the menu on both layers, after tray's own,
// and stays out of review — which offers its two verbs and nothing else (92c).
func TestAPluginVerbIsInTheMenuOnBothLayersAndNotInReview(t *testing.T) {
	sandbox(t, "- [ ] a thing priority:M")
	garage(t, "2026-08", "- something jotted")
	installVerb(t, "gcal", "schedule")

	m := New()
	if !offers(m, "schedule") {
		t.Error("the tray menu should offer the plugin verb")
	}
	if last := m.offered()[len(m.offered())-1]; last.label != "schedule" || last.key != "" {
		t.Errorf("the verb should come last and carry no letter, got %+v", last)
	}
	m = keys(m, "tab").(Model)
	if !offers(m, "schedule") {
		t.Error("the garage menu should offer it too")
	}
	m = keys(m, "v").(Model)
	if offers(m, "schedule") {
		t.Error("review offers restore and erase alone")
	}
}

// Without a plugin the menu is the menu it always was: nothing is added, and no
// letterless row appears in `?`.
func TestNoPluginMeansNoExtraRow(t *testing.T) {
	sandbox(t, "- [ ] a thing priority:M")
	m := New()
	for _, a := range m.offered() {
		if a.key == "" {
			t.Errorf("a letterless row with no plugin installed: %+v", a)
		}
	}
	if n := len(m.actionKeys()); n != len(m.offered()) {
		t.Errorf("? lists %d keys for %d menu rows", n, len(m.offered()))
	}
}

// Choosing the verb hands over the terminal rather than reloading, and says nothing
// when it comes back clean — the reload is the report. A failure is one line.
func TestChoosingAPluginVerbHandsOverTheTerminal(t *testing.T) {
	sandbox(t, "- [ ] a thing priority:M")
	installVerb(t, "gcal", "schedule")

	m := keys(New(), "enter").(Model)
	for m.offered()[m.menuAt].label != "schedule" {
		m = keys(m, "j").(Model)
	}
	next, cmd := m.Update(keyMsg("enter"))
	m = next.(Model)
	if cmd == nil {
		t.Fatal("choosing a plugin verb should return the handover command")
	}
	if m.mode != browsing || m.exec != nil {
		t.Errorf("the menu should close and the handover be spent: mode=%v exec=%v", m.mode, m.exec != nil)
	}
	if got := fmt.Sprintf("%T", cmd()); !strings.Contains(got, "execMsg") {
		t.Errorf("the command should be a terminal handover, got %s", got)
	}

	a := plugin.Action{Plugin: "gcal", Verb: "schedule"}
	next, _ = m.Update(pluginDone{action: a})
	if got := next.(Model).status; got != "" {
		t.Errorf("a clean return says nothing, got %q", got)
	}
	next, _ = m.Update(pluginDone{action: a, err: errors.New("exit status 1")})
	if got := next.(Model).status; got != "schedule: exit status 1" {
		t.Errorf("a failure names the verb and the error, got %q", got)
	}
}

// The contract, in full: the picked lines are the arguments, the layer is TRAY_LAYER.
func TestThePluginCommandCarriesTheLinesAndTheLayer(t *testing.T) {
	a := plugin.Action{Plugin: "gcal", Verb: "schedule", Path: "/p/actions/schedule"}
	picked := []core.Task{{Text: "chase the SOC2 questionnaire"}, {Text: "ship the billing migration"}}

	cmd := command(a, "2026-09", picked)
	want := []string{"/p/actions/schedule", "chase the SOC2 questionnaire", "ship the billing migration"}
	if strings.Join(cmd.Args, "|") != strings.Join(want, "|") {
		t.Errorf("Args = %v, want %v", cmd.Args, want)
	}
	if !slicesContains(cmd.Env, "TRAY_LAYER=2026-09") {
		t.Errorf("Env lacks TRAY_LAYER=2026-09: %v", cmd.Env[len(cmd.Env)-1])
	}
	if !slicesContains(command(a, "tray", picked).Env, "TRAY_LAYER=tray") {
		t.Error("on the tray the layer is named `tray`")
	}
}

func slicesContains(xs []string, x string) bool {
	for _, s := range xs {
		if s == x {
			return true
		}
	}
	return false
}
