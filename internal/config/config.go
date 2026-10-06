// Package config is the one file tray reads about itself. Nothing live is in it yet: the
// store is $TRAY_HOME/tray.db and not a setting, a plugin keeps its own keys in its own
// settings.json, and tags and the row format stay in the data (18). What remains is room
// for a preference — `dates.format` is reserved — so the file, the path and `tray config`
// exist before the first key that tray itself needs does.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Dates struct {
		Format string `yaml:"format"`
	} `yaml:"dates"`
}

// Template is what `tray init` writes when there is no file: every key present, every
// value empty, the comment saying what fills it.
const Template = `# tray — every key is optional; delete this file and tray behaves as before.
# The database is $TRAY_HOME/tray.db and is not a setting; a copy elsewhere is a plugin,
# and a plugin keeps the keys it needs in its own settings.json.
dates:
  format: ""         # reserved; parsed, unused for now
`

// Path is $TRAY_CONFIG, else the XDG config directory — beside the other dotfiles you
// edit by hand, and not beside the database, which you don't.
func Path() string {
	if set := os.Getenv("TRAY_CONFIG"); set != "" {
		return set
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "tray", "config.yaml")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join("tray", "config.yaml")
	}
	return filepath.Join(home, ".config", "tray", "config.yaml")
}

// Load reads the file. A missing file is the default; a file that does not parse is an
// error that names it, because a silently ignored typo is how a key ends up in the wrong
// place. Keys the struct does not name are ignored, so an older file is not an error.
func Load() (Config, error) {
	var c Config
	raw, err := os.ReadFile(Path())
	switch {
	case errors.Is(err, os.ErrNotExist):
	case err != nil:
		return c, err
	default:
		if err := yaml.Unmarshal(raw, &c); err != nil {
			return c, fmt.Errorf("%s: %w", Path(), err)
		}
	}
	return c, nil
}

// WriteTemplate creates the file if it is not there. It never touches one that is.
func WriteTemplate() (created bool, err error) {
	if _, err := os.Stat(Path()); err == nil {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(Path()), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(Path(), []byte(Template), 0o600)
}
