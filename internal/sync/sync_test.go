package sync

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
)

// sandbox is a fresh home holding the named fixture plugins, copied so a test may
// rewrite a plan or leave an applied.json behind.
func sandbox(t *testing.T, plugins ...string) *store.Store {
	t.Helper()
	home := t.TempDir()
	t.Setenv("TRAY_HOME", home)
	// These tests exec fixture plugins, and plugin.Run loads the config to hand keys
	// to the child. Without this the suite ships the maintainer's real api key into
	// those processes — AGENTS.md promises `go test ./...` never reads the real file.
	t.Setenv("TRAY_CONFIG", filepath.Join(t.TempDir(), "none.yaml"))
	t.Setenv("TRAY_TODAY", "2026-08-07")
	if err := os.MkdirAll(filepath.Join(home, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range plugins {
		src := filepath.Join("testdata", "plugins", name)
		if err := exec.Command("cp", "-R", src, filepath.Join(home, "plugins", name)).Run(); err != nil {
			t.Fatal(err)
		}
	}
	s, err := store.Open(home)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func rows(t *testing.T, s *store.Store, f store.Filter) []core.Task {
	t.Helper()
	out, err := s.Tasks(f)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// did is one hook's number out of a summary, or -1 when the hook did not run.
func did(sum Summary, hook, key string) int {
	for _, r := range sum {
		if r.Hook == hook {
			return r.Counts[key]
		}
	}
	return -1
}

func TestTickMaterializesAndLiftsWithoutReview(t *testing.T) {
	s := sandbox(t)
	tpl := core.Task{Layer: core.LayerTray, Text: "Weekly review", Recur: "weekly", Due: "2026-08-08", Priority: "M"}
	waiting := core.Task{Layer: core.LayerGarage, Month: "2026-08", Text: "Call mom", Wait: "2026-08-07", Priority: "H"}
	later := core.Task{Layer: core.LayerGarage, Month: "2026-08", Text: "Not yet", Wait: "2026-08-20"}
	for _, task := range []*core.Task{&tpl, &waiting, &later} {
		if err := s.Put(task); err != nil {
			t.Fatal(err)
		}
	}

	sum, err := Fire(s, Manual, store.Today())
	if err != nil || did(sum, "recur", "materialized") != 1 || did(sum, "lift", "lifted") != 1 {
		t.Fatalf("Fire = %+v, %v", sum, err)
	}
	tray := rows(t, s, store.Filter{Layer: core.LayerTray})
	if len(tray) != 2 {
		t.Fatalf("tray = %v", tray)
	}
	for _, task := range tray {
		if task.Text == "Call mom" && (task.Wait != "" || task.Priority != "H" || task.FromMonth != "2026-08") {
			t.Errorf("lifted row = %+v", task)
		}
		if task.Source == "recur:"+tpl.ID && task.Due != "2026-08-08" {
			t.Errorf("child = %+v", task)
		}
	}
	if sum, _ = Fire(s, Manual, store.Today()); did(sum, "recur", "materialized") != 0 || did(sum, "lift", "lifted") != 0 {
		t.Errorf("a second tick did something: %+v", sum)
	}
	if got := rows(t, s, store.Filter{Layer: core.LayerGarage, Month: "2026-08"}); len(got) != 1 || got[0].Text != "Not yet" {
		t.Errorf("garage = %v", got)
	}
}

// echo reports two rows and one push; nothing lands until Apply, then it lands whole,
// and a second plan finds the rows it already knows.
func TestPlansThenApply(t *testing.T) {
	s := sandbox(t, "echo")
	results, err := Plans(s, Manual, "", 5*time.Second)
	if err != nil || len(results) != 1 {
		t.Fatalf("Plans = %v, %v", results, err)
	}
	r := results[0]
	if r.Err != nil || len(r.Diff.Adds) != 2 || len(r.Push) != 1 || r.Message != "2 adds · 0 updates · 1 push" {
		t.Fatalf("result = %+v", r)
	}
	if got := rows(t, s, store.Filter{All: true}); len(got) != 0 {
		t.Fatalf("a plan landed rows: %v", got)
	}
	runs, _ := s.Runs()
	if run := runs["echo"]; !run.OK || run.Event != "manual" {
		t.Errorf("run = %+v", run)
	}

	a, err := Apply(s, r, r.Push, 5*time.Second)
	if err != nil || a.Added != 2 || len(a.PushOK) != 1 || a.PushOK[0] != "n0" {
		t.Fatalf("Apply = %+v, %v", a, err)
	}
	garage := rows(t, s, store.Filter{Layer: core.LayerGarage, Month: "echo", All: true})
	if len(garage) != 2 {
		t.Fatalf("garage = %v", garage)
	}
	for _, task := range garage {
		if task.Text == "Renew the cert" && (task.Done != "2026-08-01" || task.Priority != "H" || task.Source != "echo:n2") {
			t.Errorf("landed row = %+v", task)
		}
	}
	if applied, err := os.ReadFile(filepath.Join(store.Home(), "plugins", "echo", "applied.json")); err != nil || !contains(string(applied), `"n0"`) {
		t.Errorf("push did not reach the plugin: %s %v", applied, err)
	}

	results, _ = Plans(s, Manual, "", 5*time.Second)
	if r := results[0]; !r.Diff.Empty() || len(r.Diff.Gone) != 0 {
		t.Errorf("second plan = %+v", r.Diff)
	}
}

// A plugin that fails, times out, or asks for you leaves the store untouched and the
// others unaffected; only the ones with the marker run at launch.
func TestFailuresAreIsolated(t *testing.T) {
	s := sandbox(t, "echo", "fail", "slow", "ask")
	results, err := Plans(s, Manual, "", time.Second)
	if err != nil || len(results) != 4 {
		t.Fatalf("Plans = %v, %v", results, err)
	}
	by := map[string]Result{}
	for _, r := range results {
		by[r.Plugin] = r
	}
	if by["echo"].Err != nil || len(by["echo"].Diff.Adds) != 2 {
		t.Errorf("echo suffered: %+v", by["echo"])
	}
	if by["fail"].Err == nil || by["fail"].Message != "boom" {
		t.Errorf("fail = %+v", by["fail"])
	}
	if by["slow"].Err == nil || by["slow"].Message != "timed out" {
		t.Errorf("slow = %+v", by["slow"])
	}
	if !by["ask"].NeedsYou() || by["ask"].Message != "login needed — sign in and run again" {
		t.Errorf("ask = %+v", by["ask"])
	}
	if got := rows(t, s, store.Filter{All: true}); len(got) != 0 {
		t.Errorf("something landed: %v", got)
	}
	runs, _ := s.Runs()
	if runs["fail"].OK || !runs["echo"].OK {
		t.Errorf("runs = %+v", runs)
	}

	if err := os.WriteFile(filepath.Join(store.Home(), "plugins", "echo", "on-launch"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	results, _ = Plans(s, Launch, "", time.Second)
	if len(results) != 1 || results[0].Plugin != "echo" {
		t.Errorf("launch ran %v, want echo alone", results)
	}
	if _, err := Plans(s, Manual, "nope", time.Second); err == nil {
		t.Error("an unknown plugin name should be an error")
	}
}

// Apply is one transaction: a plan whose second row the store refuses lands nothing.
func TestApplyLandsWholeOrNotAtAll(t *testing.T) {
	s := sandbox(t, "echo")
	bad := `{"pull":[{"key":"a","text":"good row"},{"key":"b","text":"bad row","priority":"Z"}],"push":[]}`
	if err := os.WriteFile(filepath.Join(store.Home(), "plugins", "echo", "plan.json"), []byte(bad), 0o644); err != nil {
		t.Fatal(err)
	}
	results, _ := Plans(s, Manual, "echo", 5*time.Second)
	if _, err := Apply(s, results[0], nil, 5*time.Second); err == nil {
		t.Fatal("Apply accepted a priority the store rejects")
	}
	if got := rows(t, s, store.Filter{All: true}); len(got) != 0 {
		t.Errorf("the good row landed alone: %v", got)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
