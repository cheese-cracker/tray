package wire

import (
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/cheese-cracker/tray/internal/core"
)

const stampLayout = "20060102T150405Z"

// Stamp is a date in Taskwarrior's shape; "" when there is no date.
func Stamp(value string) string {
	d, ok := core.Date(value)
	if !ok {
		return ""
	}
	return d.Format("20060102T000000Z")
}

// date reads a Taskwarrior stamp, or a plain ISO date, back into the shape the store
// keeps. Anything else is no date.
func date(stamp string) string {
	if d, err := time.Parse(stampLayout, stamp); err == nil {
		return d.Format(core.DateLayout)
	}
	if _, ok := core.Date(stamp); ok {
		return stamp
	}
	return ""
}

func status(t core.Task, today time.Time) string {
	switch {
	case t.Terminal():
		return "completed"
	case t.Recur != "":
		return "recurring"
	case t.Waiting(today):
		return "waiting"
	}
	return "pending"
}

// ExportTaskwarrior is Taskwarrior's import shape, so `tray export | task import` works,
// plus the id — which is what an agent addresses a task by — and the fields Taskwarrior
// does not have but a tray reader wants back.
func ExportTaskwarrior(items []core.Task, today time.Time) (string, error) {
	out := make([]map[string]any, 0, len(items))
	for _, t := range items {
		row := map[string]any{"id": t.ID, "description": t.Text, "status": status(t, today)}
		if t.Priority != "" {
			row["priority"] = t.Priority
		}
		for field, value := range map[string]string{
			"due": t.Due, "wait": t.Wait, "until": t.Until, "entry": t.Entry, "end": t.Done,
		} {
			if stamp := Stamp(value); stamp != "" {
				row[field] = stamp
			}
		}
		if t.Recur != "" {
			row["recur"] = t.Recur
		}
		if t.FromMonth != "" {
			row["from"] = t.FromMonth
		}
		if uuid, ok := strings.CutPrefix(t.Source, "tw:"); ok {
			row["uuid"] = uuid
		}
		if len(t.Tags) > 0 {
			row["tags"] = t.Tags
		}
		// Taskwarrior's name for a note. One entry: the note is one thing, not a log.
		if t.Note != "" {
			row["annotations"] = []map[string]string{{
				"entry": Stamp(t.Entry), "description": t.Note,
			}}
		}
		row["urgency"] = core.Urgency(t, today)
		row["quadrant"] = core.Quadrant(t, today)
		out = append(out, row)
	}
	blob, err := json.MarshalIndent(out, "", "  ")
	return string(blob), err
}

type twTask struct {
	Description string   `json:"description"`
	Status      string   `json:"status"`
	Priority    string   `json:"priority"`
	Due         string   `json:"due"`
	Wait        string   `json:"wait"`
	Until       string   `json:"until"`
	Entry       string   `json:"entry"`
	End         string   `json:"end"`
	Recur       string   `json:"recur"`
	Project     string   `json:"project"`
	Tags        []string `json:"tags"`
	UUID        string   `json:"uuid"`
	Annotations []struct {
		Description string `json:"description"`
	} `json:"annotations"`
}

// ImportTaskwarrior reads what `task export` prints. A project is a tag here (9), the
// annotations fold into the one note, and the uuid rides along as the source so a
// second import finds the same row. Deleted tasks are not tasks.
func ImportTaskwarrior(r io.Reader, today time.Time) ([]core.Task, error) {
	var in []twTask
	if err := json.NewDecoder(r).Decode(&in); err != nil {
		return nil, err
	}
	var out []core.Task
	for _, tw := range in {
		if tw.Status == "deleted" || tw.Description == "" {
			continue
		}
		t := core.Task{
			Text: tw.Description, Priority: priority(tw.Priority), Due: date(tw.Due), Wait: date(tw.Wait),
			Until: date(tw.Until), Entry: date(tw.Entry), Recur: tw.Recur, Tags: tw.Tags,
		}
		if tw.Status == "completed" {
			if t.Done = date(tw.End); t.Done == "" {
				t.Done = today.Format(core.DateLayout)
			}
		}
		if tw.Project != "" {
			t.Tags = appendTag(t.Tags, tw.Project)
		}
		var notes []string
		for _, a := range tw.Annotations {
			if a.Description != "" {
				notes = append(notes, a.Description)
			}
		}
		t.Note = strings.Join(notes, "\n")
		if tw.UUID != "" {
			t.Source = "tw:" + tw.UUID
		}
		out = append(out, t)
	}
	return out, nil
}

// priority keeps only what the store's CHECK admits; another tool's E is no priority.
func priority(p string) string {
	switch p = strings.ToUpper(p); p {
	case "H", "M", "L":
		return p
	}
	return ""
}

func appendTag(tags []string, g string) []string {
	for _, have := range tags {
		if have == g {
			return tags
		}
	}
	return append(tags, g)
}
