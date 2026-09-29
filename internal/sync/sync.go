// Package sync is the one event (T4). It materializes due recurrences and lifts
// waiting rows — inside data, applied at once — then runs every plugin that keeps a
// garage and turns what each reports into a diff. Nothing a plugin says lands until
// Apply, and each plugin lands whole or not at all (T5).
package sync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/plugin"
	"github.com/cheese-cracker/tray/internal/store"
)

// A Row is one item as a plugin sees it. A nil Done, nil Tags or empty field is one the
// plugin did not report, so a site that cannot see priority never clears one; "" for
// Done says open, a date says finished.
type Row struct {
	Key      string   `json:"key"`
	Text     string   `json:"text"`
	Done     *string  `json:"done"`
	Tags     []string `json:"tags"`
	Priority string   `json:"priority,omitempty"`
	Due      string   `json:"due,omitempty"`
	Evidence string   `json:"evidence,omitempty"`

	// The rest travels only to a plugin that asked for every row (all-rows): the whole
	// task, so a replica can tell what changed. Key is then the id.
	ID        string `json:"id,omitempty"`
	Layer     string `json:"layer,omitempty"`
	Month     string `json:"month,omitempty"`
	Wait      string `json:"wait,omitempty"`
	Recur     string `json:"recur,omitempty"`
	Until     string `json:"until,omitempty"`
	Entry     string `json:"entry,omitempty"`
	FromMonth string `json:"from_month,omitempty"`
	Note      string `json:"note,omitempty"`
	Source    string `json:"source,omitempty"`
}

// A Push is a change the plugin would make on its side, for you to confirm.
type Push struct {
	Key string            `json:"key"`
	Set map[string]string `json:"set"`
}

// A Plan is what `sync plan` prints: the plugin's view of its items, what it would push
// back, and the evidence it gathered.
type Plan struct {
	Pull     []Row  `json:"pull"`
	Push     []Push `json:"push"`
	Evidence string `json:"evidence"`
}

// An Update is one row the plugin reports differently from how tray holds it.
type Update struct {
	Old    core.Task
	New    Row
	Fields []string
}

type Diff struct {
	Adds    []Row
	Updates []Update
	Gone    []core.Task // keys the plugin no longer reports; kept, never deleted
}

func (d Diff) Empty() bool { return len(d.Adds) == 0 && len(d.Updates) == 0 }

// A Result is one plugin's plan against the store, or why there is none.
type Result struct {
	Plugin   string
	Diff     Diff
	Push     []Push
	Evidence string
	Err      error
	Message  string
}

// NeedsYou is a plugin that stopped to ask — a login, a setting — rather than failed.
func (r Result) NeedsYou() bool {
	var exit *plugin.ExitError
	return errors.As(r.Err, &exit) && exit.NeedsYou()
}

// Plans runs every plugin the event admits — or the one named — each in its own
// process with its own deadline, and diffs what it reports against the rows it owns.
// The store is read before anything runs and written after everything has, so the
// plugins never contend for it. A plugin is a hook on the sync events (see Hooks); it
// runs after the built-in ones because its plan is reviewed before it lands.
func Plans(s *store.Store, ev Event, only string, timeout time.Duration) ([]Result, error) {
	type job struct {
		p    plugin.Plugin
		rows []core.Task
	}
	var jobs []job
	for _, p := range plugin.List() {
		if p.Sync == "" || (only != "" && p.Name != only) || (ev == Launch && !p.OnLaunch) {
			continue
		}
		f := store.Filter{All: true, SourcePrefix: p.Name + ":"}
		if p.AllRows {
			f.SourcePrefix = "" // the whole store: every layer, month and state
		}
		rows, err := s.Tasks(f)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, job{p, rows})
	}
	if only != "" && len(jobs) == 0 {
		return nil, fmt.Errorf("no plugin named %s keeps a garage — tray plugin", only)
	}

	results := make(chan Result, len(jobs))
	for _, j := range jobs {
		go func(j job) { results <- plan(j.p, j.rows, timeout) }(j)
	}
	out := make([]Result, 0, len(jobs))
	for range jobs {
		out = append(out, <-results)
	}
	sort.Slice(out, func(i, k int) bool { return out[i].Plugin < out[k].Plugin })

	at := time.Now().Format("2006-01-02 15:04")
	for _, r := range out {
		if err := s.RecordRun(store.Run{Name: r.Plugin, Event: string(ev), At: at, OK: r.Err == nil, Message: r.Message}); err != nil {
			return out, err
		}
	}
	return out, nil
}

// Sync is the whole event as `tray sync` runs it: the built-in hooks, then the plugins.
func Sync(s *store.Store, ev Event, only string, timeout time.Duration) (Summary, []Result, error) {
	sum, err := Fire(s, ev, store.Today())
	if err != nil {
		return sum, nil, err
	}
	results, err := Plans(s, ev, only, timeout)
	return sum, results, err
}

func plan(p plugin.Plugin, rows []core.Task, timeout time.Duration) Result {
	r := Result{Plugin: p.Name}
	in, _ := json.Marshal(map[string]any{"tasks": asRows(p.Name, rows, p.AllRows)})
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	out, err := plugin.Run(ctx, plugin.Exec{Dir: p.Dir, Path: p.Sync, Args: []string{"plan"}, Stdin: in})
	if err != nil {
		r.Err, r.Message = err, err.Error()
		return r
	}
	var pl Plan
	if err := json.Unmarshal(out, &pl); err != nil {
		r.Err, r.Message = fmt.Errorf("plan is not JSON: %w", err), "plan is not JSON"
		return r
	}
	for _, row := range pl.Pull {
		if row.Key == "" || row.Text == "" {
			r.Err, r.Message = errors.New("a pulled row has no key or no text"), "a pulled row has no key or no text"
			return r
		}
	}
	r.Diff, r.Push, r.Evidence = compare(pl.Pull, rows, p.Name, p.AllRows), pl.Push, pl.Evidence
	r.Message = summarize(r)
	return r
}

// asRows is what the plugin gets on stdin: the rows it owns, keyed the way it keys
// them — or, for a plugin that asked for every row, the whole task keyed by its id.
func asRows(name string, rows []core.Task, all bool) []Row {
	out := make([]Row, 0, len(rows))
	for _, t := range rows {
		done := t.Done
		row := Row{
			Key: strings.TrimPrefix(t.Source, name+":"), Text: t.Text, Done: &done,
			Tags: append([]string{}, t.Tags...), Priority: t.Priority, Due: t.Due,
		}
		if all {
			row.Key, row.ID, row.Layer, row.Month = t.ID, t.ID, t.Layer, t.Month
			row.Wait, row.Recur, row.Until, row.Entry = t.Wait, t.Recur, t.Until, t.Entry
			row.FromMonth, row.Note, row.Source = t.FromMonth, t.Note, t.Source
		}
		out = append(out, row)
	}
	return out
}

// compare is the diff: a key tray has not seen is an add, a key it has is an update on
// exactly the fields the plugin reported differently, and a key the plugin stopped
// reporting is gone — noted, never deleted or finished, because a board hiding done
// work is not finishing it. A plugin that reads every row keys an update by the task's
// id and owns nothing, so for it nothing is ever gone.
func compare(pull []Row, rows []core.Task, name string, all bool) Diff {
	have := map[string]core.Task{}
	for _, t := range rows {
		if all {
			have[t.ID] = t
			continue
		}
		have[strings.TrimPrefix(t.Source, name+":")] = t
	}
	var d Diff
	seen := map[string]bool{}
	for _, row := range pull {
		seen[row.Key] = true
		old, ok := have[row.Key]
		if !ok {
			d.Adds = append(d.Adds, row)
			continue
		}
		var fields []string
		if row.Text != "" && row.Text != old.Text {
			fields = append(fields, "text")
		}
		if row.Done != nil && *row.Done != old.Done {
			fields = append(fields, "done")
		}
		if row.Tags != nil && strings.Join(row.Tags, " ") != strings.Join(old.Tags, " ") {
			fields = append(fields, "tags")
		}
		if row.Priority != "" && row.Priority != old.Priority {
			fields = append(fields, "priority")
		}
		if row.Due != "" && row.Due != old.Due {
			fields = append(fields, "due")
		}
		if len(fields) > 0 {
			d.Updates = append(d.Updates, Update{Old: old, New: row, Fields: fields})
		}
	}
	if all {
		return d
	}
	for _, t := range rows {
		if !seen[strings.TrimPrefix(t.Source, name+":")] {
			d.Gone = append(d.Gone, t)
		}
	}
	sort.Slice(d.Gone, func(i, k int) bool { return d.Gone[i].ID < d.Gone[k].ID })
	return d
}

func summarize(r Result) string {
	if r.Diff.Empty() && len(r.Push) == 0 && len(r.Diff.Gone) == 0 {
		return "nothing new"
	}
	parts := []string{
		count(len(r.Diff.Adds), "add"), count(len(r.Diff.Updates), "update"), count(len(r.Push), "push"),
	}
	if n := len(r.Diff.Gone); n > 0 {
		parts = append(parts, fmt.Sprintf("%d gone", n))
	}
	return strings.Join(parts, " · ")
}

func count(n int, noun string) string {
	if n == 1 {
		return "1 " + noun
	}
	if strings.HasSuffix(noun, "sh") {
		return fmt.Sprintf("%d %ses", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// Applied is what landed: rows in the store, and what the plugin said about each push.
type Applied struct {
	Added      int           `json:"added"`
	Updated    int           `json:"updated"`
	PushOK     []string      `json:"ok"`
	PushFailed []PushFailure `json:"failed"`
}

type PushFailure struct {
	Key   string `json:"key"`
	Error string `json:"error"`
}

// Apply lands one plugin's plan: adds into its garage and updates on the fields it
// reported, in a single transaction; then the confirmed pushes go back to the plugin,
// whose answer is recorded. Adds arrive at the bottom of the ladder (T9) — a row
// already taken is updated where it is.
func Apply(s *store.Store, r Result, push []Push, timeout time.Duration) (Applied, error) {
	var a Applied
	if r.Err != nil {
		return a, r.Err
	}
	today := store.Today().Format(core.DateLayout)
	err := s.Update(func(tx *store.Store) error {
		for _, row := range r.Diff.Adds {
			t := core.Task{
				Layer: core.LayerGarage, Month: r.Plugin, Text: row.Text, Tags: row.Tags,
				Priority: row.Priority, Due: row.Due, Entry: today, Source: r.Plugin + ":" + row.Key,
			}
			if row.Done != nil {
				t.Done = *row.Done
			}
			if err := tx.Put(&t); err != nil {
				return err
			}
			a.Added++
		}
		for _, u := range r.Diff.Updates {
			t, ok, err := tx.Get(u.Old.ID)
			if err != nil || !ok {
				return err
			}
			for _, f := range u.Fields {
				switch f {
				case "text":
					t.Text = u.New.Text
				case "done":
					t.Done = *u.New.Done
				case "tags":
					t.Tags = u.New.Tags
				case "priority":
					t.Priority = u.New.Priority
				case "due":
					t.Due = u.New.Due
				}
			}
			if err := tx.Put(&t); err != nil {
				return err
			}
			a.Updated++
		}
		return nil
	})
	if err != nil {
		return a, err
	}

	message := fmt.Sprintf("applied %s · %s", count(a.Added, "add"), count(a.Updated, "update"))
	if len(push) > 0 {
		p, ok := plugin.Find(r.Plugin)
		if !ok {
			return a, fmt.Errorf("%s is no longer installed", r.Plugin)
		}
		in, _ := json.Marshal(map[string]any{"push": push})
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		out, err := plugin.Run(ctx, plugin.Exec{Dir: p.Dir, Path: p.Sync, Args: []string{"apply"}, Stdin: in})
		if err != nil {
			_ = s.RecordRun(store.Run{Name: r.Plugin, Event: "apply", At: now(), OK: false, Message: message + " · push failed: " + err.Error()})
			return a, fmt.Errorf("push: %w", err)
		}
		if err := json.Unmarshal(out, &a); err != nil {
			return a, fmt.Errorf("push answer is not JSON: %w", err)
		}
		message += fmt.Sprintf(" · pushed %d (%d failed)", len(a.PushOK), len(a.PushFailed))
	}
	return a, s.RecordRun(store.Run{Name: r.Plugin, Event: "apply", At: now(), OK: true, Message: message})
}

func now() string { return time.Now().Format("2006-01-02 15:04") }
