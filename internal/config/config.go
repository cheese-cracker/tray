// Package config is the one file tray reads about itself: where the database is and
// the keys plugins need. Everything in it is optional — tray with no file at all is the
// tray of every earlier release. Tags and the row format stay out of it (18): those
// live in the data, not beside it.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	DB struct {
		URL       string `yaml:"url"`
		AuthToken string `yaml:"auth_token"`
	} `yaml:"db"`
	OpenRouter struct {
		APIKey string `yaml:"api_key"`
		Model  string `yaml:"model"`
	} `yaml:"openrouter"`
	Dates struct {
		Format string `yaml:"format"`
	} `yaml:"dates"`

	// FromEnv names the keys an environment variable supplied, so `tray config` can
	// say where a value came from without the file having to be re-read.
	FromEnv []string `yaml:"-"`
}

// Template is what `tray init` writes when there is no file: every key present, every
// value empty, the comment saying what fills it.
const Template = `# tray — every key is optional; delete this file and tray behaves as before.
db:
  url: ""            # empty → $TRAY_HOME/tray.db. A path or file: URL → local SQLite. libsql://<db>.turso.io → Turso
  auth_token: ""     # Turso token; or TRAY_DB_TOKEN
openrouter:
  api_key: ""        # or OPENROUTER_API_KEY. Not needed to install tray — plugins read it
  model: ""
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

// Load reads the file, then lets the environment override it. A missing file is the
// default; a file that does not parse is an error that names it, because a silently
// ignored typo is how a token ends up in the wrong place.
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
	for _, o := range []struct {
		env string
		dst *string
		key string
	}{
		{"TRAY_DB_URL", &c.DB.URL, "db.url"},
		{"TRAY_DB_TOKEN", &c.DB.AuthToken, "db.auth_token"},
		{"OPENROUTER_API_KEY", &c.OpenRouter.APIKey, "openrouter.api_key"},
		{"OPENROUTER_MODEL", &c.OpenRouter.Model, "openrouter.model"},
	} {
		if v := os.Getenv(o.env); v != "" {
			*o.dst = v
			c.FromEnv = append(c.FromEnv, o.key)
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

// Mask keeps the last four characters of a secret, which is enough to tell two apart
// and not enough to use.
func Mask(s string) string {
	switch {
	case s == "":
		return "(unset)"
	case len(s) <= 4:
		return "****"
	default:
		return "…" + s[len(s)-4:]
	}
}

// PluginEnv is what a plugin may read of this: the keys, under tray's own names.
func (c Config) PluginEnv() []string {
	var env []string
	if c.OpenRouter.APIKey != "" {
		env = append(env, "TRAY_OPENROUTER_API_KEY="+c.OpenRouter.APIKey)
	}
	if c.OpenRouter.Model != "" {
		env = append(env, "TRAY_OPENROUTER_MODEL="+c.OpenRouter.Model)
	}
	return env
}
