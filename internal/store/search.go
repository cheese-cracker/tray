package store

import (
	"path/filepath"
	"sort"
	"strings"
)

type Hit struct {
	Where string // "2026-08", "tray", "someday"
	Line  string
}

// Grep searches every layer at once, in order.
func Grep(needle string) []Hit {
	paths, err := filepath.Glob(filepath.Join(Home(), "*.md"))
	if err != nil || needle == "" {
		return nil
	}
	sort.Strings(paths)

	needle = strings.ToLower(needle)
	var hits []Hit
	for _, path := range paths {
		lines, err := Read(path)
		if err != nil {
			continue
		}
		where := strings.TrimSuffix(filepath.Base(path), ".md")
		for _, line := range lines {
			if strings.Contains(strings.ToLower(line), needle) {
				hits = append(hits, Hit{Where: where, Line: strings.TrimSpace(line)})
			}
		}
	}
	return hits
}

// Months a needle appears in — a line in four months is a rot signal.
func MonthsWith(hits []Hit) int {
	seen := map[string]bool{}
	for _, h := range hits {
		if len(h.Where) > 0 && h.Where[0] >= '0' && h.Where[0] <= '9' {
			seen[h.Where] = true
		}
	}
	return len(seen)
}

// Tags is the vocabulary in use across every layer, for the tag picker.
func Tags() []string {
	seen := map[string]bool{}
	collect := func(d *Doc, err error) {
		if err != nil {
			return
		}
		for _, t := range d.Tasks() {
			for _, g := range t.Tags {
				seen[g] = true
			}
		}
	}
	collect(Tray())
	collect(Garage(Someday))
	for _, month := range Months() {
		collect(Garage(month))
	}
	out := make([]string, 0, len(seen))
	for g := range seen {
		out = append(out, g)
	}
	sort.Strings(out)
	return out
}
