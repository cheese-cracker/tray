// Package store knows where things live and how to write them, never what they mean.
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	DefaultHome = "tray"
	TrayHeader  = "# tray"
	Someday     = "someday"
	monthLayout = "2006-01"
)

var monthName = regexp.MustCompile(`^\d{4}-\d{2}\.md$`)

func Home() string {
	if set := os.Getenv("TRAY_HOME"); set != "" {
		return expand(set)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return DefaultHome
	}
	return filepath.Join(home, DefaultHome)
}

func expand(path string) string {
	if !strings.HasPrefix(path, "~") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~"))
}

// Today honours TRAY_TODAY so the month turn is testable without freezing the clock.
func Today() time.Time {
	if set := os.Getenv("TRAY_TODAY"); set != "" {
		if d, err := time.ParseInLocation("2006-01-02", set, time.UTC); err == nil {
			return d
		}
	}
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
}

func ThisMonth() string { return Today().Format(monthLayout) }

func NextMonth(month string) string { return shiftMonth(month, 1) }
func PrevMonth(month string) string { return shiftMonth(month, -1) }

func shiftMonth(month string, by int) string {
	year, mon, err := splitMonth(month)
	if err != nil {
		return month
	}
	return time.Date(year, time.Month(mon)+time.Month(by), 1, 0, 0, 0, 0, time.UTC).Format(monthLayout)
}

func splitMonth(month string) (int, int, error) {
	parts := strings.SplitN(month, "-", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("not a month: %q", month)
	}
	year, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, err
	}
	mon, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, err
	}
	return year, mon, nil
}

func TrayPath() string { return filepath.Join(Home(), "tray.md") }

func MonthPath(month string) string {
	if month == "" {
		month = ThisMonth()
	}
	if month == Someday {
		return filepath.Join(Home(), Someday+".md")
	}
	return filepath.Join(Home(), month+".md")
}

func MonthHeader(month string) string { return "# " + month }

// Months lists the month files present, oldest first.
func Months() []string {
	entries, err := os.ReadDir(Home())
	if err != nil {
		return nil
	}
	var found []string
	for _, e := range entries {
		if !e.IsDir() && monthName.MatchString(e.Name()) {
			found = append(found, strings.TrimSuffix(e.Name(), ".md"))
		}
	}
	sort.Strings(found)
	return found
}
