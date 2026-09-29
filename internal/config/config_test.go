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
	for _, k := range []string{"TRAY_DB_URL", "TRAY_DB_TOKEN", "OPENROUTER_API_KEY", "OPENROUTER_MODEL"} {
		t.Setenv(k, "")
	}
	return p
}

func TestMissingFileIsTheDefault(t *testing.T) {
	scratch(t, "")
	c, err := Load()
	if err != nil || c.DB.URL != "" || c.OpenRouter.APIKey != "" || len(c.FromEnv) != 0 {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestFileThenEnvWins(t *testing.T) {
	scratch(t, "db:\n  url: file.db\n  auth_token: filetoken\nopenrouter:\n  model: m1\n")
	t.Setenv("TRAY_DB_TOKEN", "envtoken")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.DB.URL != "file.db" || c.DB.AuthToken != "envtoken" || c.OpenRouter.Model != "m1" {
		t.Fatalf("precedence: %+v", c)
	}
	if strings.Join(c.FromEnv, ",") != "db.auth_token" {
		t.Fatalf("FromEnv = %v", c.FromEnv)
	}
}

func TestMalformedFileNamesItself(t *testing.T) {
	p := scratch(t, "db: [\n")
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
	if err := os.WriteFile(p, []byte("db:\n  url: keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if created, _ := WriteTemplate(); created {
		t.Fatal("overwrote an existing file")
	}
	scratch(t, Template)
	if c, err := Load(); err != nil || c.DB.URL != "" {
		t.Fatalf("template: %+v %v", c, err)
	}
}

func TestMask(t *testing.T) {
	for in, want := range map[string]string{"": "(unset)", "abc": "****", "abcd": "****", "eyJhbGciOi1234": "…1234"} {
		if got := Mask(in); got != want {
			t.Errorf("Mask(%q) = %q, want %q", in, got, want)
		}
	}
}
