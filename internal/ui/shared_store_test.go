package ui_test

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/cheese-cracker/tray/internal/cli"
	"github.com/cheese-cracker/tray/internal/store"
	"github.com/cheese-cracker/tray/internal/ui"
)

// T21 · one store, two hands. What the CLI writes the interface shows; what the
// interface writes the CLI reads back by the same id. This is the promise the whole
// split rests on, so it goes through both real surfaces: cli.Run for the agent half,
// the bubbletea model for yours.
//
// It lives outside package ui because cli imports ui, and a test inside ui could not
// import cli back.
func TestFlowTheCLIAndTheTUIShareOneStore(t *testing.T) {
	t.Setenv("TRAY_HOME", t.TempDir())
	t.Setenv("TRAY_TODAY", "2026-08-07")

	run(t, "dump", "fix the sync job")
	run(t, "dump", "book the flights")

	s, err := store.Open(store.Home())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	// The interface: take the first line and accept the form, then finish it; take the
	// second, then hand it straight back.
	m := press(ui.New(s), "tab", "t", "enter") // garage → take → save M
	m = press(m, "shift+tab", "x")             // on the tray: done
	m = press(m, "tab", "t", "enter")          // the remaining garage line → tray
	press(m, "shift+tab", "d")                 // and back home

	ids := map[string]string{}
	all, _ := s.Tasks(store.Filter{All: true})
	for _, task := range all {
		ids[task.Text] = task.ID
	}

	var tray, garage []map[string]any
	unmarshal(t, run(t, "list", "--all", "--json"), &tray)
	unmarshal(t, run(t, "garage", "list", "--json"), &garage)

	if len(tray) != 1 || tray[0]["description"] != "fix the sync job" || tray[0]["status"] != "completed" {
		t.Fatalf("the CLI should read one finished tray task, got %v", tray)
	}
	if tray[0]["id"] != ids["fix the sync job"] || tray[0]["end"] != "20260807T000000Z" {
		t.Errorf("the id and the day the interface wrote must read back: %v", tray[0])
	}
	if len(garage) != 1 || garage[0]["description"] != "book the flights" || garage[0]["id"] != ids["book the flights"] {
		t.Fatalf("the handed-back line should be the CLI's garage row, got %v", garage)
	}
	if garage[0]["priority"] != "M" {
		t.Errorf("handing back keeps what the tray gave (88a): %v", garage[0])
	}
}

// run drives the CLI in-process and returns what it printed.
func run(t *testing.T, args ...string) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	was := os.Stdout
	os.Stdout = w
	code := cli.Run(args)
	w.Close()
	os.Stdout = was
	out, _ := io.ReadAll(r)
	if code != 0 {
		t.Fatalf("tray %s exited %d:\n%s", strings.Join(args, " "), code, out)
	}
	return string(out)
}

func press(m tea.Model, keys ...string) tea.Model {
	for _, k := range keys {
		var msg tea.KeyMsg
		switch k {
		case "tab":
			msg = tea.KeyMsg{Type: tea.KeyTab}
		case "shift+tab":
			msg = tea.KeyMsg{Type: tea.KeyShiftTab}
		case "enter":
			msg = tea.KeyMsg{Type: tea.KeyEnter}
		default:
			msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(k)}
		}
		m, _ = m.Update(msg)
	}
	return m
}

func unmarshal(t *testing.T, raw string, into any) {
	t.Helper()
	if err := json.Unmarshal([]byte(raw), into); err != nil {
		t.Fatalf("not JSON: %v\n%s", err, raw)
	}
}
