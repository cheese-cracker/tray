// Package plugin finds the third-party plugins installed under the tray home and runs
// them as processes. It knows where they are, whether they look runnable, which verbs
// they offer and which hooks they opted into; it never reads a garage — a plugin garage
// is ordinary rows in the store — and never reads the settings a plugin keeps, so it
// stays free of both the grammar (16) and anyone else's meaning.
package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cheese-cracker/tray/internal/config"
	"github.com/cheese-cracker/tray/internal/store"
)

// The folder is the manifest. Each name here is a fact a plugin states by having the
// file: `sync` says it keeps a garage, a file under actions/ is a menu verb, the
// example settings are the form it wants filled, the on-launch marker is its consent
// to run when the app opens, and the all-rows marker asks to see the whole store rather
// than its own rows (T38). Nothing is declared and nothing is parsed (18).
const (
	SyncFile        = "sync"
	ActionsDir      = "actions"
	SettingsExample = "settings.example.json"
	SettingsFile    = "settings.json"
	OnLaunchMarker  = "on-launch"
	AllRowsMarker   = "all-rows"
	LogFile         = "log"
	EvidenceDir     = "evidence"
)

// A Plugin is a directory. Name is the folder, which is also the garage it owns.
// A folder counts once it has a runnable sync or at least one verb.
type Plugin struct {
	Name     string
	Dir      string
	Sync     string   // "" when the plugin keeps no garage
	Verbs    []string // the executables under ActionsDir, in name order
	OnLaunch bool     // run at launch too, not only on a manual sync
	AllRows  bool     // `sync plan` reads every row, not only the ones it keyed
}

// An Action is one verb the interface can offer. tray hands the executable the picked
// ids as arguments and the layer they sit on in TRAY_LAYER, and otherwise knows
// nothing about what it does.
type Action struct {
	Plugin string
	Verb   string
	Path   string
}

// Garage is the month this plugin's rows sit in. It equals Name, spelled out because
// the two being the same string is a decision and not a coincidence — `someday` is the
// precedent for a month that is not a month.
func (p Plugin) Garage() string { return p.Name }

// SettingsKeys are the fields the plugin wants filled, read off its example file. No
// file, no keys: the plugin has nothing to ask.
func (p Plugin) SettingsKeys() []string {
	raw, err := os.ReadFile(filepath.Join(p.Dir, SettingsExample))
	if err != nil {
		return nil
	}
	var example map[string]any
	if json.Unmarshal(raw, &example) != nil {
		return nil
	}
	keys := make([]string, 0, len(example))
	for k := range example {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// SetSettings merges values into the plugin's settings file. tray never interprets
// what is there; it only refuses a key the example does not name, so a typo cannot
// become a setting nothing reads.
func (p Plugin) SetSettings(values map[string]string) error {
	if keys := p.SettingsKeys(); keys != nil {
		for k := range values {
			if !contains(keys, k) {
				return fmt.Errorf("%s has no setting %q — it asks for %s", p.Name, k, strings.Join(keys, ", "))
			}
		}
	}
	settings := map[string]any{}
	if raw, err := os.ReadFile(filepath.Join(p.Dir, SettingsFile)); err == nil {
		_ = json.Unmarshal(raw, &settings)
	}
	for k, v := range values {
		settings[k] = v
	}
	blob, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(p.Dir, SettingsFile), append(blob, '\n'), 0o600)
}

// Dir is where plugins live, beside the database.
func Dir() string { return filepath.Join(store.Home(), "plugins") }

// List is every installed plugin, in name order. A folder with neither a runnable
// sync nor a verb is not a plugin but half an install, and counting it would promise
// something that cannot happen. No plugins at all is the usual case and is not an
// error.
func List() []Plugin {
	entries, err := os.ReadDir(Dir()) // already name-sorted
	if err != nil {
		return nil
	}
	var found []Plugin
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(Dir(), e.Name())
		p := Plugin{Name: e.Name(), Dir: dir, Verbs: verbs(dir)}
		if sync := filepath.Join(dir, SyncFile); runnable(sync) {
			p.Sync = sync
		}
		if p.Sync == "" && len(p.Verbs) == 0 {
			continue
		}
		_, err := os.Stat(filepath.Join(dir, OnLaunchMarker))
		p.OnLaunch = err == nil
		_, err = os.Stat(filepath.Join(dir, AllRowsMarker))
		p.AllRows = err == nil
		found = append(found, p)
	}
	return found
}

func Find(name string) (Plugin, bool) {
	for _, p := range List() {
		if p.Name == name {
			return p, true
		}
	}
	return Plugin{}, false
}

// Actions is every verb every plugin offers, in plugin then verb order.
func Actions() []Action {
	var out []Action
	for _, p := range List() {
		for _, v := range p.Verbs {
			out = append(out, Action{Plugin: p.Name, Verb: v, Path: filepath.Join(p.Dir, ActionsDir, v)})
		}
	}
	return out
}

// verbs are the runnable files under actions/. The exec bit is the same consent it is
// for sync: a verb you have not marked is not yet one tray may put in front of you.
func verbs(dir string) []string {
	entries, err := os.ReadDir(filepath.Join(dir, ActionsDir))
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if runnable(filepath.Join(dir, ActionsDir, e.Name())) {
			out = append(out, e.Name())
		}
	}
	return out
}

// runnable checks the exec bit rather than assuming it. tray executes this file, so a
// plugin you have not marked executable is one you have not finished installing — and
// marking it is the only consent tray gets to ask for.
func runnable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode().Perm()&0o111 != 0
}

// An Exec is one run of a plugin's executable: which file, with what arguments, what
// arrives on stdin, and anything beyond TRAY_HOME and TRAY_PLUGIN_DIR in its environment.
type Exec struct {
	Dir   string
	Path  string
	Args  []string
	Stdin []byte
	Env   []string
}

// An ExitError is the plugin saying no. Code 2 means it needs you — a login, a setting
// — and Message is the first line it wrote to stderr, which is all a status line has
// room for. The whole of stderr is in the plugin's log.
type ExitError struct {
	Code    int
	Message string
}

func (e *ExitError) Error() string {
	if e.Message == "" {
		return fmt.Sprintf("exit %d", e.Code)
	}
	return e.Message
}

// NeedsYou is exit 2: not a failure, a question tray cannot answer.
func (e *ExitError) NeedsYou() bool { return e.Code == 2 }

// Run executes one plugin file and returns its stdout. stderr goes to the plugin's log
// whatever happens, so a failure can be read after the fact. The context bounds the
// run: a plugin that outlives it is killed and reported as timed out, and WaitDelay
// keeps a child it left behind from holding the pipes open.
func Run(ctx context.Context, e Exec) ([]byte, error) {
	cmd := exec.CommandContext(ctx, e.Path, e.Args...)
	cmd.Dir = e.Dir
	cmd.Env = append(os.Environ(), "TRAY_HOME="+store.Home(), "TRAY_PLUGIN_DIR="+e.Dir)
	// The keys from the config file, under tray's names — a plugin never reads the file.
	if cfg, err := config.Load(); err == nil {
		cmd.Env = append(cmd.Env, cfg.PluginEnv()...)
	}
	cmd.Env = append(cmd.Env, e.Env...)
	cmd.Stdin = bytes.NewReader(e.Stdin)
	var out, errs bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errs
	cmd.WaitDelay = time.Second

	err := cmd.Run()
	_ = os.WriteFile(filepath.Join(e.Dir, LogFile), errs.Bytes(), 0o600)

	switch {
	case err == nil:
		return out.Bytes(), nil
	case errors.Is(ctx.Err(), context.DeadlineExceeded):
		return nil, &ExitError{Code: -1, Message: "timed out"}
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return nil, &ExitError{Code: exit.ExitCode(), Message: firstLine(errs.String())}
	}
	return nil, err
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(s), "\n")
	return line
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
