package plugin

import (
	"os"
	"path/filepath"
	"testing"
)

// install writes a plugin the way an install wizard would, or without the exec bit
// when mode says so — which is the case worth testing.
func install(t *testing.T, name string, mode os.FileMode) {
	t.Helper()
	dir := filepath.Join(Dir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, Runner), []byte("#!/bin/sh\n"), mode); err != nil {
		t.Fatal(err)
	}
}

func sandbox(t *testing.T) {
	t.Helper()
	t.Setenv("TRAY_HOME", t.TempDir())
}

func names(ps []Plugin) []string {
	out := make([]string, 0, len(ps))
	for _, p := range ps {
		out = append(out, p.Name)
	}
	return out
}

// No plugins is the usual case, so it is the one that must not be an error: the
// plugins directory does not exist until something installs one.
func TestListIsEmptyWithNoPluginsDirectory(t *testing.T) {
	sandbox(t)
	if got := List(); len(got) != 0 {
		t.Errorf("List() = %v, want none", names(got))
	}
}

func TestListFindsInstalledPluginsInNameOrder(t *testing.T) {
	sandbox(t)
	install(t, "notion", 0o755)
	install(t, "linear", 0o755)

	got := names(List())
	if len(got) != 2 || got[0] != "linear" || got[1] != "notion" {
		t.Errorf("List() = %v, want [linear notion]", got)
	}
}

// A folder without the exec bit is half an install. Listing it would promise a sync
// tray cannot run, and marking the file is the only consent tray asks for.
func TestListSkipsAPluginThatIsNotExecutable(t *testing.T) {
	sandbox(t)
	install(t, "notion", 0o644)
	if got := List(); len(got) != 0 {
		t.Errorf("List() = %v, want none — run is not executable", names(got))
	}
}

func TestListSkipsAFolderWithNoRunner(t *testing.T) {
	sandbox(t)
	if err := os.MkdirAll(filepath.Join(Dir(), "notion"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := List(); len(got) != 0 {
		t.Errorf("List() = %v, want none — no %s", names(got), Runner)
	}
}

// The folder name is the whole manifest: it names the plugin and the garage it owns,
// so a plugin called notion reads and writes notion.md and nothing declares that.
func TestGarageAndPathComeFromTheFolderName(t *testing.T) {
	sandbox(t)
	install(t, "notion", 0o755)

	p, ok := Find("notion")
	if !ok {
		t.Fatal("Find(notion) = false, want the installed plugin")
	}
	if p.Garage() != "notion" {
		t.Errorf("Garage() = %s, want notion", p.Garage())
	}
	if want := filepath.Join(os.Getenv("TRAY_HOME"), "notion.md"); p.Path() != want {
		t.Errorf("Path() = %s, want %s", p.Path(), want)
	}
}

func TestFindMissesWhatIsNotInstalled(t *testing.T) {
	sandbox(t)
	install(t, "notion", 0o755)
	if _, ok := Find("jira"); ok {
		t.Error("Find(jira) = true, want false")
	}
}

// installVerb writes one menu verb the way a plugin's installer would.
func installVerb(t *testing.T, name, verb string, mode os.FileMode) {
	t.Helper()
	dir := filepath.Join(Dir(), name, ActionsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, verb), []byte("#!/bin/sh\n"), mode); err != nil {
		t.Fatal(err)
	}
}

// A verb is a file under actions/, and its name is the whole declaration. The exec
// bit is the same consent it is for run, so an unmarked one is not offered.
func TestActionsAreTheExecutablesUnderActions(t *testing.T) {
	sandbox(t)
	installVerb(t, "gcal", "schedule", 0o755)
	installVerb(t, "gcal", "half", 0o644)

	got := Actions()
	if len(got) != 1 || got[0].Plugin != "gcal" || got[0].Verb != "schedule" {
		t.Fatalf("Actions() = %v, want gcal/schedule alone", got)
	}
	if want := filepath.Join(Dir(), "gcal", ActionsDir, "schedule"); got[0].Path != want {
		t.Errorf("Path = %s, want %s", got[0].Path, want)
	}
}

// A plugin that only adds verbs keeps no garage and has no run — and is a plugin
// all the same. Listing it says what it adds rather than a garage it never wrote.
func TestAPluginMayHaveVerbsAndNoRunner(t *testing.T) {
	sandbox(t)
	installVerb(t, "gcal", "schedule", 0o755)

	p, ok := Find("gcal")
	if !ok {
		t.Fatal("Find(gcal) = false, want the verb-only plugin")
	}
	if p.Run != "" || len(p.Verbs) != 1 || p.Verbs[0] != "schedule" {
		t.Errorf("got Run=%q Verbs=%v, want no runner and [schedule]", p.Run, p.Verbs)
	}
}
