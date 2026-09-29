package core

import "testing"

func TestTakeRemembersTheMonthItLeft(t *testing.T) {
	src := New("add retries to the sync job", []string{"infra"})
	src.Month = "2026-08"
	Move(&src, LayerTray, "")
	if src.Layer != LayerTray || src.Month != "" {
		t.Errorf("a taken task is on the tray and in no month: %+v", src)
	}
	if src.FromMonth != "2026-08" {
		t.Errorf("from = %q, want the garage month it left", src.FromMonth)
	}
	if len(src.Tags) != 1 {
		t.Error("tags survive the move")
	}
}

// Handing back with no destination goes home, and forgets home: it lives there again.
func TestHandBackGoesHomeAndForgets(t *testing.T) {
	task := Task{Layer: LayerTray, Text: "Fix alerts", Priority: "H", FromMonth: "2026-06"}
	Move(&task, LayerGarage, "")
	if task.Layer != LayerGarage || task.Month != "2026-06" || task.FromMonth != "" {
		t.Errorf("got %+v", task)
	}
	if task.Priority != "H" {
		t.Error("what the tray added comes home with it (88a)")
	}
	// Taking it again stamps where it lives now, not where it once was.
	Move(&task, LayerTray, "")
	if task.FromMonth != "2026-06" {
		t.Errorf("from = %q", task.FromMonth)
	}
}

func TestCarryForwardIsAMonthChange(t *testing.T) {
	task := Task{Layer: LayerGarage, Month: "2026-08", Text: "leftover"}
	Move(&task, LayerGarage, "2026-09")
	if task.Month != "2026-09" || task.Layer != LayerGarage || task.FromMonth != "" {
		t.Errorf("got %+v", task)
	}
}

func TestFinishIsTerminalInPlace(t *testing.T) {
	task, _ := Parse("- [ ] Renew the TLS certificate priority:H", day("2026-08-01"))
	Finish(&task, day("2026-08-07"))
	if !task.Terminal() || task.Live() {
		t.Error("want done and not live")
	}
	want := "- [x] ~~Renew the TLS certificate~~ priority:H done:2026-08-07"
	if got := Line(task, true); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	Restore(&task)
	if task.Terminal() || !task.Live() {
		t.Error("restore reopens it, with no trace")
	}
}

// Done on a template ends the recurrence rather than striking a template through.
func TestFinishOnATemplateStopsIt(t *testing.T) {
	tpl := Task{Layer: LayerTray, Text: "Weekly review", Recur: "weekly", Due: "2026-10-03"}
	Finish(&tpl, day("2026-08-07"))
	if tpl.Done != "" || tpl.Until != "2026-08-07" {
		t.Errorf("got %+v", tpl)
	}
	if tpl.Live() {
		t.Error("a template is never live work")
	}
}
