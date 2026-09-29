package plugin

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// install writes a plugin the way an installer would, or without the exec bit when
// mode says so — which is the case worth testing.
func install(t *testing.T, name string, mode os.FileMode) {
	t.Helper()
	dir := filepath.Join(Dir(), name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, SyncFile), []byte("#!/bin/sh\n"), mode); err != nil {
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
		t.Errorf("List() = %v, want none — sync is not executable", names(got))
	}
}

func TestListSkipsAFolderWithNoSync(t *testing.T) {
	sandbox(t)
	if err := os.MkdirAll(filepath.Join(Dir(), "notion"), 0o755); err != nil {
		t.Fatal(err)
	}
	if got := List(); len(got) != 0 {
		t.Errorf("List() = %v, want none — no %s", names(got), SyncFile)
	}
}

// The folder name is the whole manifest: it names the plugin and the garage month its
// rows sit in, and nothing declares that.
func TestGarageComesFromTheFolderName(t *testing.T) {
	sandbox(t)
	install(t, "notion", 0o755)

	p, ok := Find("notion")
	if !ok {
		t.Fatal("Find(notion) = false, want the installed plugin")
	}
	if p.Garage() != "notion" {
		t.Errorf("Garage() = %s, want notion", p.Garage())
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
// bit is the same consent it is for sync, so an unmarked one is not offered.
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

// A plugin that only adds verbs keeps no garage and has no sync — and is a plugin
// all the same. Listing it says what it adds rather than a garage it never wrote.
func TestAPluginMayHaveVerbsAndNoSync(t *testing.T) {
	sandbox(t)
	installVerb(t, "gcal", "schedule", 0o755)

	p, ok := Find("gcal")
	if !ok {
		t.Fatal("Find(gcal) = false, want the verb-only plugin")
	}
	if p.Sync != "" || len(p.Verbs) != 1 || p.Verbs[0] != "schedule" {
		t.Errorf("got Sync=%q Verbs=%v, want no sync and [schedule]", p.Sync, p.Verbs)
	}
}

// The marker is the plugin's consent to run at launch; the example settings are the
// form it asks to have filled, and a key the example does not name is refused.
func TestMarkerAndSettingsAreFilesInTheFolder(t *testing.T) {
	sandbox(t)
	install(t, "web", 0o755)
	p, _ := Find("web")
	if p.OnLaunch {
		t.Error("OnLaunch without the marker")
	}
	if err := os.WriteFile(filepath.Join(p.Dir, OnLaunchMarker), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if p, _ = Find("web"); !p.OnLaunch {
		t.Error("OnLaunch = false with the marker present")
	}

	if p.SettingsKeys() != nil {
		t.Error("keys without an example file")
	}
	if err := os.WriteFile(filepath.Join(p.Dir, SettingsExample), []byte(`{"url":"","rules":"in progress only"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if keys := p.SettingsKeys(); len(keys) != 2 || keys[0] != "rules" || keys[1] != "url" {
		t.Errorf("SettingsKeys = %v", keys)
	}
	if err := p.SetSettings(map[string]string{"colour": "red"}); err == nil {
		t.Error("a key the example does not name was accepted")
	}
	if err := p.SetSettings(map[string]string{"url": "https://x"}); err != nil {
		t.Fatal(err)
	}
	if err := p.SetSettings(map[string]string{"rules": "all"}); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(p.Dir, SettingsFile))
	if got := string(raw); !contains([]string{got}, got) || len(got) == 0 || !containsAll(got, `"url": "https://x"`, `"rules": "all"`) {
		t.Errorf("settings.json = %s", got)
	}
}

// Run hands back stdout, keeps stderr in the log, and turns an exit code into an
// ExitError carrying stderr's first line — exit 2 being the plugin asking for you.
func TestRunReportsExitCodesAndKeepsTheLog(t *testing.T) {
	sandbox(t)
	install(t, "ask", 0o755)
	p, _ := Find("ask")
	script := "#!/bin/sh\necho '{\"pull\":[]}'\necho 'login needed — sign in and run again' >&2\necho second >&2\nexit 2\n"
	if err := os.WriteFile(p.Sync, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	_, err := Run(context.Background(), Exec{Dir: p.Dir, Path: p.Sync, Args: []string{"plan"}})
	var exit *ExitError
	if !asExit(err, &exit) || !exit.NeedsYou() || exit.Message != "login needed — sign in and run again" {
		t.Fatalf("err = %v", err)
	}
	if log, _ := os.ReadFile(filepath.Join(p.Dir, LogFile)); !containsAll(string(log), "login needed", "second") {
		t.Errorf("log = %q", log)
	}

	if err := os.WriteFile(p.Sync, []byte("#!/bin/sh\ncat\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	out, err := Run(context.Background(), Exec{Dir: p.Dir, Path: p.Sync, Stdin: []byte("echoed")})
	if err != nil || string(out) != "echoed" {
		t.Errorf("out = %q, err = %v", out, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := os.WriteFile(p.Sync, []byte("#!/bin/sh\nsleep 3\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	_, err = Run(ctx, Exec{Dir: p.Dir, Path: p.Sync})
	if !asExit(err, &exit) || exit.Message != "timed out" {
		t.Errorf("err = %v, want timed out", err)
	}
	if time.Since(start) > 2*time.Second {
		t.Errorf("a timed-out plugin held Run for %s", time.Since(start))
	}
}

func asExit(err error, target **ExitError) bool {
	e, ok := err.(*ExitError)
	if ok {
		*target = e
	}
	return ok
}

func containsAll(s string, parts ...string) bool {
	for _, p := range parts {
		if !containsStr(s, p) {
			return false
		}
	}
	return true
}

func containsStr(s, sub string) bool {
	return len(sub) == 0 || len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
