// Package wire reads and writes the shapes tray shares with other tools — markdown
// bullets, todo.txt, Taskwarrior JSON. The grammar is core's; this package only knows
// which file is which layer.
package wire

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
)

// ImportMarkdown brings a markdown home — or one file of it — into the store. tray.md
// is the tray; a month, someday or a plugin's name is that garage. A `→` line is
// history whose live copy went elsewhere, so it is skipped; a struck or ticked line
// arrives finished. A row already present (same layer, month and words) is skipped,
// which is what lets the import run twice.
func ImportMarkdown(s *store.Store, path string, today time.Time) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", err
	}
	files := []string{path}
	if info.IsDir() {
		if files, err = filepath.Glob(filepath.Join(path, "*.md")); err != nil {
			return "", err
		}
		sort.Strings(files)
	}
	if len(files) == 0 {
		return "", fmt.Errorf("%s: no markdown files", path)
	}

	present, err := s.Tasks(store.Filter{All: true})
	if err != nil {
		return "", err
	}
	seen := map[string]bool{}
	for _, t := range present {
		seen[key(t)] = true
	}

	var report []string
	err = s.Update(func(tx *store.Store) error {
		for _, file := range files {
			raw, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			layer, month := place(file)
			added, skipped := 0, 0
			for _, t := range core.Tasks(strings.Split(strings.TrimRight(string(raw), "\n"), "\n"), today) {
				t.Layer, t.Month = layer, month
				if t.Moved != "" || t.Text == "" || seen[key(t)] {
					skipped++
					continue
				}
				if err := tx.Put(&t); err != nil {
					return err
				}
				seen[key(t)] = true
				added++
			}
			report = append(report, fmt.Sprintf("%s: %d imported, %d skipped", filepath.Base(file), added, skipped))
		}
		return nil
	})
	return strings.Join(report, "\n"), err
}

func place(file string) (layer, month string) {
	stem := strings.TrimSuffix(filepath.Base(file), ".md")
	if stem == "tray" {
		return core.LayerTray, ""
	}
	return core.LayerGarage, stem
}

func key(t core.Task) string { return t.Layer + "\x00" + t.Month + "\x00" + t.Text }
