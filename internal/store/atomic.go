package store

import (
	"os"
	"path/filepath"
	"strings"
)

// Read returns a file's lines. A missing file is empty, not an error.
func Read(path string) ([]string, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	text := strings.TrimSuffix(string(raw), "\n")
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

// Write replaces a file atomically, so a crash can never leave it truncated.
func Write(path string, lines []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tray-")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())

	body := strings.TrimRight(strings.Join(lines, "\n"), "\n") + "\n"
	if _, err := tmp.WriteString(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Ensure creates a layer with just its header if it isn't there yet.
func Ensure(path, header string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	return Write(path, []string{header, ""})
}
