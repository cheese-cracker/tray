package plugin

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/cheese-cracker/tray/internal/config"
)

// A Folder is any directory under plugins/ that has the files of a plugin, runnable or
// not — what `tray plugin` shows. Half is a folder none of whose entry points carries
// the exec bit (105): an install someone has not finished, listed so the silence has
// a name. List() keeps skipping these; nothing here may run.
type Folder struct {
	Plugin
	Half bool
}

// Folders is every plugin folder, in name order, half-installed ones included.
func Folders() []Folder {
	entries, err := os.ReadDir(Dir())
	if err != nil {
		return nil
	}
	var out []Folder
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		p := read(e.Name())
		if !hasPluginFiles(p.Dir) {
			continue
		}
		out = append(out, Folder{Plugin: p, Half: p.Sync == "" && p.Health == "" && len(p.Verbs) == 0})
	}
	return out
}

func hasPluginFiles(dir string) bool {
	for _, f := range []string{SyncFile, HealthFile} {
		if _, err := os.Stat(filepath.Join(dir, f)); err == nil {
			return true
		}
	}
	actions, _ := os.ReadDir(filepath.Join(dir, ActionsDir))
	return len(actions) > 0
}

// Configured says whether a plugin that asks for settings has been given any. A plugin
// with no example asks for nothing, and is configured by definition.
func (p Plugin) Configured() bool {
	if _, err := os.Stat(filepath.Join(p.Dir, SettingsExample)); err != nil {
		return true
	}
	_, err := os.Stat(filepath.Join(p.Dir, SettingsFile))
	return err == nil
}

// AsksForSettings is the example file being present at all.
func (p Plugin) AsksForSettings() bool {
	_, err := os.Stat(filepath.Join(p.Dir, SettingsExample))
	return err == nil
}

// NoProbe is what a check records for a plugin without a `health` file: an absence,
// not a verdict, so the listing's state looks past it to the last sync run.
const NoProbe = "no probe"

// Check runs the probe. Exit 0 is well, 2 is a question for you, anything else failed;
// no probe is not a failure but an absence, and says so. Probed is false only then.
func Check(ctx context.Context, p Plugin) (ok bool, message string, probed bool) {
	if p.Health == "" {
		return false, NoProbe, false
	}
	_, err := Run(ctx, Exec{Dir: p.Dir, Path: p.Health})
	if err == nil {
		return true, "ok", true
	}
	var exit *ExitError
	if errors.As(err, &exit) && exit.NeedsYou() {
		return false, "needs you — " + exit.Error(), true
	}
	return false, "failed — " + err.Error(), true
}

// A Core plugin is a capability shipped inside tray and gated by the config file. It is
// listed whether or not it is on, so its absence is visible rather than silent, and it
// never runs as a process.
type Core struct {
	Name     string
	Enabled  func(config.Config) bool
	Provides string
	TurnOn   string
}

var Cores = []Core{{
	Name:     "openrouter",
	Enabled:  func(c config.Config) bool { return c.OpenRouter.APIKey != "" },
	Provides: "TRAY_OPENROUTER_API_KEY / MODEL to plugins; agent verbs later",
	TurnOn:   "set openrouter.api_key in config.yaml",
}}
