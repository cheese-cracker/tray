package sync

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
)

// An all-rows plugin sees the whole store keyed by id, and a pulled row keyed by an id
// is an update of that row — never an add, never gone.
func TestAllRowsPluginReadsEveryRowAndKeysByID(t *testing.T) {
	s := sandbox(t, "allrows", "own")
	garage := core.Task{Layer: core.LayerGarage, Month: "2026-08", Text: "a garage line"}
	tray := core.Task{Layer: core.LayerTray, Text: "a tray task", Priority: "H"}
	finished := core.Task{Layer: core.LayerGarage, Month: "2026-08", Text: "finished", Done: "2026-08-01"}
	for _, task := range []*core.Task{&garage, &tray, &finished} {
		if err := s.Put(task); err != nil {
			t.Fatal(err)
		}
	}

	if _, err := Plans(s, Manual, "", 5*time.Second); err != nil {
		t.Fatal(err)
	}
	var seen struct{ Tasks []Row }
	raw, err := os.ReadFile(filepath.Join(os.Getenv("TRAY_HOME"), "plugins", "allrows", "seen.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &seen); err != nil {
		t.Fatal(err)
	}
	if len(seen.Tasks) != 3 {
		t.Fatalf("all-rows saw %d rows, want 3", len(seen.Tasks))
	}
	for _, row := range seen.Tasks {
		if row.Key != row.ID || len(row.ID) != 4 || row.Layer == "" {
			t.Errorf("row %+v is not keyed by its id with its layer", row)
		}
	}
	raw, err = os.ReadFile(filepath.Join(os.Getenv("TRAY_HOME"), "plugins", "own", "seen.json"))
	if err != nil {
		t.Fatal(err)
	}
	var own struct{ Tasks []Row }
	if err := json.Unmarshal(raw, &own); err != nil || len(own.Tasks) != 0 {
		t.Fatalf("a plugin without the marker saw %d rows, want 0 (%v)", len(own.Tasks), err)
	}

	open := ""
	d := compare([]Row{{Key: garage.ID, Text: "a garage line, renamed", Done: &open}, {Key: tray.ID, Text: "a tray task, renamed", Done: &open},
		{Key: "phone1", Text: "typed on a phone", Done: &open}}, rows(t, s, store.Filter{All: true}), "allrows", true)
	if len(d.Updates) != 1 || d.Updates[0].Old.ID != garage.ID || len(d.Updates[0].Fields) != 1 || d.Updates[0].Fields[0] != "text" {
		t.Fatalf("an id-keyed garage row should update that row's text: %+v", d.Updates)
	}
	if len(d.OnTray) != 1 || d.OnTray[0].Old.ID != tray.ID {
		t.Fatalf("the tray is manual: its row is kept, not updated (T42): %+v", d.OnTray)
	}
	if len(d.Adds) != 1 || d.Adds[0].Key != "phone1" {
		t.Fatalf("an unknown key should be an add: %+v", d.Adds)
	}
	if len(d.Gone) != 0 {
		t.Fatalf("an all-rows plugin owns nothing, so nothing is gone: %+v", d.Gone)
	}
}
