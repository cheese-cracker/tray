package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/plugin"
	"github.com/cheese-cracker/tray/internal/store"
	"github.com/cheese-cracker/tray/internal/sync"
)

const defaultTimeout = 10 * time.Minute

func timeoutOf(opts options) (time.Duration, error) {
	if opts.timeout == "" {
		return defaultTimeout, nil
	}
	d, err := time.ParseDuration(opts.timeout)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("--timeout wants a duration like 30s or 10m, not %q", opts.timeout)
	}
	return d, nil
}

// sync is the one event. Recurrence and lifting apply on the spot; what a plugin
// reports is printed as a plan and lands only under --apply, whole per plugin.
func cmdSync(s *store.Store, req request) (string, error) {
	timeout, err := timeoutOf(req.opts)
	if err != nil {
		return "", err
	}
	sum, results, err := sync.Sync(s, sync.Manual, req.opts.plugin, timeout)
	if err != nil {
		return "", err
	}
	applied := map[string]sync.Applied{}
	if req.opts.apply {
		for _, r := range results {
			if r.Err != nil || (r.Diff.Empty() && len(r.Push) == 0) {
				continue
			}
			a, err := sync.Apply(s, r, r.Push, timeout)
			if err != nil {
				return "", fmt.Errorf("%s: %w", r.Plugin, err)
			}
			applied[r.Plugin] = a
		}
	}
	if req.opts.json {
		return syncJSON(sum, results, applied)
	}
	lines := []string{fmt.Sprintf("materialized %d · lifted %d", sum.Materialized, sum.Lifted)}
	lines = append(lines, renderPlans(results, applied)...)
	if pending := pendingRows(results); pending > 0 && !req.opts.apply {
		lines = append(lines, fmt.Sprintf("run `tray sync --apply` to land %d %s", pending, plural(pending, "change")))
	}
	return strings.Join(lines, "\n"), nil
}

func pendingRows(results []sync.Result) int {
	n := 0
	for _, r := range results {
		if r.Err == nil {
			n += len(r.Diff.Adds) + len(r.Diff.Updates) + len(r.Push)
		}
	}
	return n
}

func renderPlans(results []sync.Result, applied map[string]sync.Applied) []string {
	var out []string
	for _, r := range results {
		switch {
		case r.NeedsYou():
			out = append(out, fmt.Sprintf("%s: needs you — %s", r.Plugin, r.Message))
			continue
		case r.Err != nil:
			out = append(out, fmt.Sprintf("%s: failed — %s", r.Plugin, r.Message))
			continue
		}
		out = append(out, fmt.Sprintf("%s: %s", r.Plugin, r.Message))
		for _, row := range r.Diff.Adds {
			out = append(out, "  + "+row.Text+tagsOf(row.Tags))
		}
		for _, u := range r.Diff.Updates {
			out = append(out, fmt.Sprintf("  ~ %s %s: %s", u.Old.ID, u.Old.Text, changes(u)))
		}
		for _, p := range r.Push {
			out = append(out, "  ↑ "+p.Key+" "+setOf(p.Set))
		}
		for _, g := range r.Diff.Gone {
			out = append(out, fmt.Sprintf("  gone from source: %s %s (kept)", g.ID, g.Text))
		}
		if r.Evidence != "" {
			out = append(out, "  evidence: "+r.Evidence)
		}
		if a, ok := applied[r.Plugin]; ok {
			out = append(out, fmt.Sprintf("  applied: %d adds · %d updates · pushed %d (%d failed)",
				a.Added, a.Updated, len(a.PushOK), len(a.PushFailed)))
		}
	}
	return out
}

func changes(u sync.Update) string {
	var parts []string
	for _, f := range u.Fields {
		old, new := u.Old.Attr(f), ""
		switch f {
		case "text":
			old, new = u.Old.Text, u.New.Text
		case "done":
			old, new = orOpen(u.Old.Done), orOpen(*u.New.Done)
		case "tags":
			old, new = strings.Join(u.Old.Tags, " "), strings.Join(u.New.Tags, " ")
		case "priority":
			new = u.New.Priority
		case "due":
			new = u.New.Due
		}
		parts = append(parts, fmt.Sprintf("%s %s → %s", f, old, new))
	}
	return strings.Join(parts, ", ")
}

func orOpen(done string) string {
	if done == "" {
		return "open"
	}
	return "done " + done
}

func tagsOf(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return "  " + core.TagMark + strings.Join(tags, " "+core.TagMark)
}

func setOf(set map[string]string) string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"="+set[k])
	}
	return strings.Join(parts, " ")
}

func plural(n int, noun string) string {
	if n == 1 {
		return noun
	}
	return noun + "s"
}

type planJSON struct {
	Plugin   string        `json:"plugin"`
	Adds     []sync.Row    `json:"adds"`
	Updates  []updateJSON  `json:"updates"`
	Push     []sync.Push   `json:"push"`
	Gone     []goneJSON    `json:"gone"`
	Evidence string        `json:"evidence,omitempty"`
	Error    string        `json:"error,omitempty"`
	NeedsYou bool          `json:"needs_you,omitempty"`
	Message  string        `json:"message"`
	Applied  *sync.Applied `json:"applied,omitempty"`
}

type updateJSON struct {
	ID     string               `json:"id"`
	Text   string               `json:"text"`
	Fields map[string][2]string `json:"fields"`
}

type goneJSON struct {
	ID   string `json:"id"`
	Text string `json:"text"`
}

func syncJSON(sum sync.Summary, results []sync.Result, applied map[string]sync.Applied) (string, error) {
	plans := make([]planJSON, 0, len(results))
	for _, r := range results {
		p := planJSON{Plugin: r.Plugin, Adds: r.Diff.Adds, Push: r.Push, Evidence: r.Evidence, Message: r.Message,
			Updates: []updateJSON{}, Gone: []goneJSON{}}
		if p.Adds == nil {
			p.Adds = []sync.Row{}
		}
		if p.Push == nil {
			p.Push = []sync.Push{}
		}
		if r.Err != nil {
			p.Error, p.NeedsYou = r.Err.Error(), r.NeedsYou()
		}
		for _, u := range r.Diff.Updates {
			fields := map[string][2]string{}
			for _, part := range strings.Split(changes(u), ", ") {
				name, rest, _ := strings.Cut(part, " ")
				old, new, _ := strings.Cut(rest, " → ")
				fields[name] = [2]string{old, new}
			}
			p.Updates = append(p.Updates, updateJSON{ID: u.Old.ID, Text: u.Old.Text, Fields: fields})
		}
		for _, g := range r.Diff.Gone {
			p.Gone = append(p.Gone, goneJSON{ID: g.ID, Text: g.Text})
		}
		if a, ok := applied[r.Plugin]; ok {
			p.Applied = &a
		}
		plans = append(plans, p)
	}
	blob, err := json.MarshalIndent(map[string]any{
		"materialized": sum.Materialized, "lifted": sum.Lifted, "plugins": plans,
	}, "", "  ")
	return string(blob), err
}

// plugin lists what is installed and how its last run went; `run` prints one plugin's
// plan without landing it; `set` fills the settings its example names.
func cmdPlugin(s *store.Store, req request) (string, error) {
	sub := "list"
	if len(req.tail) > 0 {
		sub = req.tail[0]
	}
	switch sub {
	case "list":
		return listPlugins(s)
	case "run":
		if len(req.tail) < 2 {
			return "", fmt.Errorf("which one? tray plugin run <name>")
		}
		timeout, err := timeoutOf(req.opts)
		if err != nil {
			return "", err
		}
		results, err := sync.Plans(s, sync.Manual, req.tail[1], timeout)
		if err != nil {
			return "", err
		}
		return strings.Join(renderPlans(results, nil), "\n"), nil
	case "set":
		if len(req.tail) < 3 {
			return "", fmt.Errorf("tray plugin set <name> key=value …")
		}
		p, ok := plugin.Find(req.tail[1])
		if !ok {
			return "", fmt.Errorf("no plugin named %s — tray plugin", req.tail[1])
		}
		values := map[string]string{}
		for _, kv := range req.tail[2:] {
			k, v, ok := strings.Cut(kv, "=")
			if !ok || k == "" {
				return "", fmt.Errorf("%q is not key=value", kv)
			}
			values[k] = v
		}
		if err := p.SetSettings(values); err != nil {
			return "", err
		}
		return fmt.Sprintf("set %s for %s", strings.Join(setKeys(values), ", "), p.Name), nil
	}
	return "", fmt.Errorf("tray plugin [list | run <name> | set <name> key=value …]")
}

func setKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func listPlugins(s *store.Store) (string, error) {
	found := plugin.List()
	if len(found) == 0 {
		return "no plugins — one is a folder in " + plugin.Dir() +
			" holding an executable `" + plugin.SyncFile + "` or a verb under `" + plugin.ActionsDir + "/`", nil
	}
	runs, err := s.Runs()
	if err != nil {
		return "", err
	}
	var rows []string
	for _, p := range found {
		desc, err := describe(s, p, runs[p.Name])
		if err != nil {
			return "", err
		}
		rows = append(rows, fmt.Sprintf("%-12s %s", p.Name, desc))
	}
	return strings.Join(rows, "\n"), nil
}

// describe says what a plugin is doing here: the garage it keeps, as rows in the
// store, the verbs it puts in the menu, whether it runs at launch, and how its last
// run went. A plugin with verbs and no sync keeps no garage, so none is claimed.
func describe(s *store.Store, p plugin.Plugin, last store.Run) (string, error) {
	var parts []string
	if p.Sync != "" {
		rows, err := s.Tasks(store.Filter{Layer: core.LayerGarage, Month: p.Garage(), All: true})
		if err != nil {
			return "", err
		}
		parts = append(parts, "garage "+count(len(rows), "row"))
	}
	if len(p.Verbs) > 0 {
		parts = append(parts, "enter → "+strings.Join(p.Verbs, ", "))
	}
	if p.OnLaunch {
		parts = append(parts, "on-launch")
	}
	if p.Sync != "" {
		switch {
		case last.Name == "":
			parts = append(parts, "never run")
		case last.OK:
			parts = append(parts, fmt.Sprintf("last %s %s ok — %s", last.Hook, last.At, last.Message))
		default:
			parts = append(parts, fmt.Sprintf("last %s %s failed — %s", last.Hook, last.At, last.Message))
		}
	}
	return strings.Join(parts, " · "), nil
}
