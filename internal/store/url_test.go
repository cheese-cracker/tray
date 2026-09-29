package store

import (
	"os"
	"path/filepath"
	"testing"
)

// The scheme picks the engine, and the token is never in the DSN. No network: nothing
// here opens anything.
func TestDriverAndDSNByScheme(t *testing.T) {
	cases := []struct{ url, driver, dsn string }{
		{"", "sqlite", filepath.Join("home", File)},
		{"/elsewhere/other.db", "sqlite", "/elsewhere/other.db"},
		{"file:/elsewhere/other.db", "sqlite", "/elsewhere/other.db"},
		{"libsql://tray-me.turso.io", "libsql", "libsql://tray-me.turso.io"},
		{"https://tray-me.turso.io", "libsql", "https://tray-me.turso.io"},
		{"wss://tray-me.turso.io", "libsql", "wss://tray-me.turso.io"},
	}
	for _, c := range cases {
		if got := Driver(c.url); got != c.driver {
			t.Errorf("Driver(%q) = %q, want %q", c.url, got, c.driver)
		}
		if got := DSN("home", c.url); got != c.dsn {
			t.Errorf("DSN(%q) = %q, want %q", c.url, got, c.dsn)
		}
	}
}

func TestOpenAtAnotherFile(t *testing.T) {
	home, other := t.TempDir(), filepath.Join(t.TempDir(), "other.db")
	s, err := OpenAt(home, other, "")
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err := s.Tasks(Filter{}); err != nil {
		t.Fatal(err)
	}
	if fileExists(filepath.Join(home, File)) {
		t.Fatal("the home's tray.db was created although the URL named another file")
	}
	if !fileExists(other) {
		t.Fatal("the named file was not created")
	}
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }
