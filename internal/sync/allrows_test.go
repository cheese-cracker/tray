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

// A replica's add lands where the row sat on the other machine — its garage, its id,
// its columns — and a row that sat on the tray waits in the replica's garage (T45, T42).
func TestReplicaAddLandsWhereItSat(t *testing.T) {
	s := sandbox(t)
	taken := core.Task{Layer: core.LayerGarage, Month: "2026-08", Text: "already here"}
	if err := s.Put(&taken); err != nil {
		t.Fatal(err)
	}
	open := ""
	adds := []Row{
		{Key: "j4ne", ID: "j4ne", Layer: core.LayerGarage, Month: "2026-06", Text: "sat in June", Done: &open, Wait: "2026-09-01", Entry: "2026-06-02"},
		{Key: "c1au", ID: "c1au", Layer: core.LayerGarage, Month: "claude", Text: "another plugin's row", Done: &open, Source: "claude:session-9"},
		{Key: "t4ay", ID: "t4ay", Layer: core.LayerTray, Text: "sat on the tray", Done: &open, Priority: "H", FromMonth: "2026-07"},
		{Key: "elsewhere", ID: taken.ID, Layer: core.LayerGarage, Month: "2026-06", Text: "its id is taken here", Done: &open},
		{Key: "phone1", Text: "says nothing of where", Done: &open},
	}
	replica := Result{Plugin: "replica", AllRows: true, Diff: Diff{Adds: adds}}
	if _, err := Apply(s, replica, nil, time.Second); err != nil {
		t.Fatal(err)
	}
	by := map[string]core.Task{}
	for _, task := range rows(t, s, store.Filter{All: true}) {
		by[task.Text] = task
	}
	if got := by["sat in June"]; got.ID != "j4ne" || got.Layer != core.LayerGarage || got.Month != "2026-06" || got.Wait != "2026-09-01" || got.Entry != "2026-06-02" || got.Source != "" {
		t.Errorf("a garage row should land in its garage under its id with its columns: %+v", got)
	}
	if got := by["another plugin's row"]; got.ID != "c1au" || got.Month != "claude" || got.Source != "claude:session-9" {
		t.Errorf("another plugin's row should keep its garage and its source: %+v", got)
	}
	if got := by["sat on the tray"]; got.Layer != core.LayerGarage || got.Month != "replica" || got.FromMonth != "2026-07" || got.Priority != "H" {
		t.Errorf("a sync never writes to the tray: the row waits in the replica's garage (T42): %+v", got)
	}
	if got := by["its id is taken here"]; got.ID == taken.ID || got.Month != "2026-06" || got.Source != "replica:elsewhere" {
		t.Errorf("a taken id is never reused: the row gets a fresh one and the replica's source: %+v", got)
	}
	if got := by["already here"]; got.ID != taken.ID || got.Month != "2026-08" {
		t.Errorf("the row that held the id is untouched: %+v", got)
	}
	if got := by["says nothing of where"]; got.Month != "replica" || got.Source != "replica:phone1" {
		t.Errorf("a row that names no garage lands in the replica's: %+v", got)
	}

	// Where a row sat is a replica's to say. A plugin that reads only its own rows
	// lands everything in its own garage, whatever it reports.
	board := Result{Plugin: "board", Diff: Diff{Adds: []Row{{Key: "b1", ID: "b0rd", Layer: core.LayerGarage, Month: "2026-06", Text: "from a board", Done: &open}}}}
	if _, err := Apply(s, board, nil, time.Second); err != nil {
		t.Fatal(err)
	}
	for _, task := range rows(t, s, store.Filter{All: true}) {
		if task.Text == "from a board" && (task.Month != "board" || task.ID == "b0rd" || task.Source != "board:b1") {
			t.Errorf("without all-rows a plugin lands in its own garage: %+v", task)
		}
	}
}
