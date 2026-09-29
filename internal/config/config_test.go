package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func scratch(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	if body != "" {
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("TRAY_CONFIG", p)
	for _, k := range []string{"OPENROUTER_API_KEY", "OPENROUTER_MODEL"} {
		t.Setenv(k, "")
	}
	return p
}

func TestMissingFileIsTheDefault(t *testing.T) {
	scratch(t, "")
	c, err := Load()
	if err != nil || c.OpenRouter.APIKey != "" || c.OpenRouter.Model != "" || len(c.FromEnv) != 0 {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestFileThenEnvWins(t *testing.T) {
	scratch(t, "openrouter:\n  api_key: filekey1234\n  model: m1\n")
	t.Setenv("OPENROUTER_API_KEY", "envkey9999")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.OpenRouter.APIKey != "envkey9999" || c.OpenRouter.Model != "m1" {
		t.Fatalf("precedence: %+v", c)
	}
	if strings.Join(c.FromEnv, ",") != "openrouter.api_key" {
		t.Fatalf("FromEnv = %v", c.FromEnv)
	}
}

// A stray db: section from an older file is not an error — yaml ignores what the struct
// does not name — and it changes nothing: the store is not a setting.
func TestUnknownKeysAreIgnored(t *testing.T) {
	scratch(t, "db:\n  url: libsql://somewhere\nopenrouter:\n  model: m2\n")
	c, err := Load()
	if err != nil || c.OpenRouter.Model != "m2" {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestMalformedFileNamesItself(t *testing.T) {
	p := scratch(t, "openrouter: [\n")
	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), p) || !strings.Contains(err.Error(), "line") {
		t.Fatalf("err = %v", err)
	}
}

func TestTemplateParsesAndIsWrittenOnce(t *testing.T) {
	p := scratch(t, "")
	if created, err := WriteTemplate(); err != nil || !created {
		t.Fatalf("first write: %v %v", created, err)
	}
	if err := os.WriteFile(p, []byte("openrouter:\n  model: keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if created, _ := WriteTemplate(); created {
		t.Fatal("overwrote an existing file")
	}
	scratch(t, Template)
	if c, err := Load(); err != nil || c.OpenRouter.Model != "" {
		t.Fatalf("template: %+v %v", c, err)
	}
}

func TestPluginEnv(t *testing.T) {
	var c Config
	if got := c.PluginEnv(); len(got) != 0 {
		t.Fatalf("empty config gave %v", got)
	}
	c.OpenRouter.APIKey, c.OpenRouter.Model = "k", "m"
	if got := strings.Join(c.PluginEnv(), " "); got != "TRAY_OPENROUTER_API_KEY=k TRAY_OPENROUTER_MODEL=m" {
		t.Fatalf("got %q", got)
	}
}

func TestMask(t *testing.T) {
	for in, want := range map[string]string{"": "(unset)", "abc": "****", "abcd": "****", "eyJhbGciOi1234": "…1234"} {
		if got := Mask(in); got != want {
			t.Errorf("Mask(%q) = %q, want %q", in, got, want)
		}
	}
}
