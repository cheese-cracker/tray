package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cheese-cracker/tray/internal/plugin"
	"github.com/cheese-cracker/tray/internal/store"
	"github.com/cheese-cracker/tray/internal/sync"
)

const defaultTimeout = sync.DefaultTimeout

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
	lines := []string{sum.String()}
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
			out = append(out, "  + "+row.Text+sync.TagSuffix(row.Tags))
		}
		for _, u := range r.Diff.Updates {
			out = append(out, fmt.Sprintf("  ~ %s %s: %s", u.Old.ID, u.Old.Text, u))
		}
		for _, p := range r.Push {
			out = append(out, "  ↑ "+p.Key+" "+setOf(p.Set))
		}
		for _, g := range r.Diff.Gone {
			out = append(out, fmt.Sprintf("  gone from source: %s %s (kept)", g.ID, g.Text))
		}
		for _, u := range r.Diff.OnTray {
			out = append(out, fmt.Sprintf("  on the tray, kept: %s %s: %s", u.Old.ID, u.Old.Text, u))
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
			for _, part := range strings.Split(u.String(), ", ") {
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
	hooks := map[string]map[string]int{}
	for _, r := range sum {
		if r.Counts != nil {
			hooks[r.Hook] = r.Counts
		}
	}
	blob, err := json.MarshalIndent(map[string]any{"hooks": hooks, "plugins": plans}, "", "  ")
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
		return pluginView(s, req.opts.json)
	case "check":
		only := ""
		if len(req.tail) > 1 {
			only = req.tail[1]
		}
		if err := checkPlugins(s, only); err != nil {
			return "", err
		}
		return pluginView(s, req.opts.json)
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
	return "", fmt.Errorf("tray plugin [list | check [name] | run <name> | set <name> key=value …]")
}

// checkPlugins runs every probe — or one plugin's — and remembers each verdict apart
// from the last sync run. A plugin without a probe is noted as such, not failed.
func checkPlugins(s *store.Store, only string) error {
	found := false
	for _, f := range plugin.Folders() {
		if only != "" && f.Name != only {
			continue
		}
		found = true
		if f.Half {
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		ok, message, _ := plugin.Check(ctx, f.Plugin)
		cancel()
		if err := s.RecordCheck(store.Run{Name: f.Name, Event: "check", At: now(), OK: ok, Message: message}); err != nil {
			return err
		}
	}
	if only != "" && !found {
		return fmt.Errorf("no plugin named %s — tray plugin", only)
	}
	return nil
}

func now() string { return time.Now().Format("2006-01-02 15:04") }

// A pluginRow is one line of the health view: how a plugin is, what it joins, whether it
// has been told what it asked for, and the last thing it did.
type pluginRow struct {
	Name     string     `json:"name"`
	State    string     `json:"state"`
	Hooks    []string   `json:"hooks"`
	Settings string     `json:"settings"`
	Sync     *store.Run `json:"sync,omitempty"`
	Check    *store.Run `json:"check,omitempty"`
	Last     string     `json:"last"`
}

func pluginRows(s *store.Store) ([]pluginRow, error) {
	runs, err := s.Runs()
	if err != nil {
		return nil, err
	}
	checks, err := s.Checks()
	if err != nil {
		return nil, err
	}
	var rows []pluginRow
	for _, f := range plugin.Folders() {
		row := pluginRow{Name: f.Name, Hooks: hooksOf(f.Plugin), Settings: "—"}
		if f.AsksForSettings() {
			row.Settings = "ok"
			if !f.Configured() {
				row.Settings = "missing"
			}
		}
		if r, ok := runs[f.Name]; ok {
			r := r
			row.Sync = &r
		}
		if c, ok := checks[f.Name]; ok {
			c := c
			row.Check = &c
		}
		row.State = stateOf(f, row.Sync, row.Check)
		row.Last = lastOf(row.Sync, row.Check)
		rows = append(rows, row)
	}
	return rows, nil
}

// stateOf reads the folder before the history: an install you have not finished, or a
// form you have not filled, is the state, whatever the last run said.
func stateOf(f plugin.Folder, sync, check *store.Run) string {
	switch {
	case f.Half:
		return "half-installed"
	case !f.Configured():
		return "unconfigured"
	case check != nil && check.Message != plugin.NoProbe:
		return check.Message
	case sync == nil:
		return "never run"
	case sync.OK:
		return "ok"
	default:
		return "failed — " + sync.Message
	}
}

func hooksOf(p plugin.Plugin) []string {
	var hooks []string
	if p.OnLaunch {
		hooks = append(hooks, "launch")
	}
	if p.Sync != "" {
		hooks = append(hooks, "manual")
	}
	if len(p.Verbs) > 0 {
		hooks = append(hooks, "verbs: "+strings.Join(p.Verbs, ","))
	}
	if p.AllRows {
		hooks = append(hooks, "all-rows")
	}
	return hooks
}

// lastOf is the newer of the last sync run and the last probe; the timestamps sort as
// text because they are written to sort as text. Within the same minute the probe wins:
// it is the one you just asked for.
func lastOf(sync, check *store.Run) string {
	newest := sync
	if check != nil && (newest == nil || check.At >= newest.At) {
		newest = check
	}
	if newest == nil {
		return "—"
	}
	return fmt.Sprintf("%s %s — %s", newest.Event, newest.At, newest.Message)
}

// pluginView is the health view: one row per folder.
func pluginView(s *store.Store, asJSON bool) (string, error) {
	rows, err := pluginRows(s)
	if err != nil {
		return "", err
	}
	if asJSON {
		blob, err := json.MarshalIndent(rows, "", "  ")
		return string(blob), err
	}
	var cells [][]string
	for _, r := range rows {
		hooks := "—"
		if len(r.Hooks) > 0 {
			hooks = strings.Join(r.Hooks, " · ")
		}
		cells = append(cells, []string{r.Name, clip(r.State, 36), clip(hooks, 40), r.Settings, clip(r.Last, 48)})
	}
	out := table(cells, []string{"NAME", "STATE", "HOOKS", "SETTINGS", "LAST"})
	if len(plugin.Folders()) == 0 {
		out += "\n\nno plugins installed — one is a folder in " + plugin.Dir() +
			" holding an executable `" + plugin.SyncFile + "` or a verb under `" + plugin.ActionsDir + "/`"
	}
	return out, nil
}

func setKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
