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
	return p
}

func TestMissingFileIsTheDefault(t *testing.T) {
	scratch(t, "")
	c, err := Load()
	if err != nil || c.Dates.Format != "" {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestTheFileIsRead(t *testing.T) {
	scratch(t, "dates:\n  format: 2 Jan\n")
	c, err := Load()
	if err != nil || c.Dates.Format != "2 Jan" {
		t.Fatalf("got %+v, %v", c, err)
	}
}

// Sections from older files — a db: that was never a setting, an openrouter: whose keys
// now live in a plugin's settings.json — are not errors: yaml ignores what the struct does
// not name, and they change nothing.
func TestUnknownKeysAreIgnored(t *testing.T) {
	scratch(t, "db:\n  url: libsql://somewhere\nopenrouter:\n  api_key: k\ndates:\n  format: d\n")
	c, err := Load()
	if err != nil || c.Dates.Format != "d" {
		t.Fatalf("got %+v, %v", c, err)
	}
}

func TestMalformedFileNamesItself(t *testing.T) {
	p := scratch(t, "dates: [\n")
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
	if err := os.WriteFile(p, []byte("dates:\n  format: keep\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if created, _ := WriteTemplate(); created {
		t.Fatal("overwrote an existing file")
	}
	scratch(t, Template)
	if c, err := Load(); err != nil || c.Dates.Format != "" {
		t.Fatalf("template: %+v %v", c, err)
	}
}
