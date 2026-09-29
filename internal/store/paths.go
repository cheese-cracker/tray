// Package store knows where the database lives and how to read and write it, never
// what a task means. It is the only package that speaks SQL.
package store

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	Someday     = "someday"
	monthLayout = "2006-01"
)

var monthRe = regexp.MustCompile(`^\d{4}-\d{2}$`)

// Home is $TRAY_HOME, else the XDG data directory. Hidden rather than in `~`, by 53's
// own rule: the convention splits on ownership, and nobody is meant to open a sqlite
// file by hand.
func Home() string {
	if set := os.Getenv("TRAY_HOME"); set != "" {
		return expand(set)
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "tray")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "tray"
	}
	return filepath.Join(home, ".local", "share", "tray")
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

// IsMonth tells a calendar month apart from someday and a plugin's garage.
func IsMonth(name string) bool { return monthRe.MatchString(name) }

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
