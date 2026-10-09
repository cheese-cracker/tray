package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
)

// installSync writes a plugin whose plan is the JSON given; `apply` records what it
// was handed beside the script.
func installSync(t *testing.T, name, plan string) string {
	t.Helper()
	t.Setenv("TRAY_CONFIG", filepath.Join(t.TempDir(), "none.yaml")) // never the user's file
	dir := filepath.Join(os.Getenv("TRAY_HOME"), "plugins", name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	script := "#!/bin/sh\ncase $1 in plan) cat plan.json ;; apply) cat > applied.json; echo '{\"ok\":[\"x\"],\"failed\":[]}' ;; esac\n"
	if err := os.WriteFile(filepath.Join(dir, "sync"), []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plan.json"), []byte(plan), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func garageOf(t *testing.T, month string) []core.Task {
	t.Helper()
	out, err := ts.Tasks(store.Filter{Layer: core.LayerGarage, Month: month})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func applied(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "applied.json"))
	return err == nil
}

// A row coming in is shown before it lands, and enter lands it in the plugin's garage.
func TestSyncShowsWhatComesInAndEnterLandsIt(t *testing.T) {
	sandbox(t, "- [ ] a thing priority:M")
	dir := installSync(t, "remote", `{"pull":[{"key":"r1","text":"typed on the phone","done":"","tags":["phone"]}],"push":[]}`)

	m := keys(New(ts), "S").(Model)
	if m.mode != reviewing || len(m.plans) != 1 {
		t.Fatalf("S should hold the plan for review: mode=%v plans=%d status=%q", m.mode, len(m.plans), m.status)
	}
	if v := m.View(); !strings.Contains(v, "1 coming in") || !strings.Contains(v, "typed on the phone") {
		t.Errorf("the review should name the row:\n%s", v)
	}
	if len(garageOf(t, "remote")) != 0 {
		t.Error("nothing lands before enter")
	}

	m = keys(m, "enter").(Model)
	if m.mode != browsing || len(m.plans) != 0 {
		t.Errorf("enter should close the review: mode=%v", m.mode)
	}
	if rows := garageOf(t, "remote"); len(rows) != 1 || rows[0].Text != "typed on the phone" || rows[0].Source != "remote:r1" {
		t.Errorf("the row should land in the plugin's garage with its source: %+v", rows)
	}
	if !strings.Contains(m.status, "remote: +1") {
		t.Errorf("status = %q", m.status)
	}
	if applied(dir) {
		t.Error("no pushes, so apply should not have been called")
	}
}

// esc keeps local: nothing comes in, and the pushes in the same plan still go out.
func TestSyncEscKeepsLocalButStillPushes(t *testing.T) {
	sandbox(t, "- [ ] a thing priority:M")
	id := New(ts).items()[0].ID
	dir := installSync(t, "remote", `{"pull":[{"key":"r1","text":"typed on the phone","done":""}],"push":[{"key":"`+id+`","set":{"done":"2026-08-07"}}]}`)

	m := keys(New(ts), "S", "esc").(Model)
	if m.mode != browsing {
		t.Fatalf("esc should close the review, mode=%v", m.mode)
	}
	if len(garageOf(t, "remote")) != 0 {
		t.Error("esc must land nothing")
	}
	if !applied(dir) {
		t.Error("the push should still have gone out — local is the source of truth")
	}
	if !strings.Contains(m.status, "kept local") {
		t.Errorf("status = %q", m.status)
	}
}

// A plan with only pushes has nothing to decide: it lands with no screen at all.
func TestSyncPushOnlyLandsWithoutAScreen(t *testing.T) {
	sandbox(t, "- [ ] a thing priority:M")
	id := New(ts).items()[0].ID
	dir := installSync(t, "remote", `{"pull":[],"push":[{"key":"`+id+`","set":{"done":"2026-08-07"}}]}`)

	m := keys(New(ts), "S").(Model)
	if m.mode != browsing || len(m.plans) != 0 {
		t.Fatalf("pushes alone need no review: mode=%v", m.mode)
	}
	if !applied(dir) {
		t.Error("the push should have been handed to the plugin")
	}
	if !strings.Contains(m.status, "remote: +0 ~0 ↑1") {
		t.Errorf("status = %q", m.status)
	}
}

// A plugin that fails is one line in the status and nothing else changes.
func TestSyncNamesAFailedPluginAndCarriesOn(t *testing.T) {
	sandbox(t, "- [ ] a thing priority:M")
	dir := installSync(t, "remote", `not json`)
	m := keys(New(ts), "S").(Model)
	if m.mode != browsing || !strings.Contains(m.status, "remote:") {
		t.Errorf("mode=%v status=%q", m.mode, m.status)
	}
	if applied(dir) {
		t.Error("a failed plan must not be applied")
	}
}

// The tray is manual: a remote change to a tray row opens no review and lands nothing.
func TestSyncNeverTouchesTheTray(t *testing.T) {
	sandbox(t, "- [ ] a thing priority:M")
	id := New(ts).items()[0].ID
	dir := installSync(t, "remote", `{"pull":[{"key":"`+id+`","text":"renamed on the phone","done":""}],"push":[]}`)
	os.WriteFile(filepath.Join(dir, "all-rows"), nil, 0o644) // keyed by id, like a replica
	m := keys(New(ts), "S").(Model)
	if m.mode != browsing {
		t.Fatalf("no review for a tray row, mode=%v", m.mode)
	}
	if got := New(ts).items()[0].Text; got != "a thing" {
		t.Errorf("the tray row changed to %q", got)
	}
	if !strings.Contains(m.status, "remote: 1 on the tray, kept") {
		t.Errorf("status = %q", m.status)
	}
}
