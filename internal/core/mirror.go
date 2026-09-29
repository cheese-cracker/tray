package core

import (
	"regexp"
	"strings"
)

// The mirror is the lightest view of a layer there is: one bullet per open task, the id
// in parentheses, then the words — and nothing else, so a phone can add a line or fix
// one without knowing any grammar. Months are `## 2026-09` headings.
var (
	mirrorBulletRe  = regexp.MustCompile(`^\s*[-*]\s+(?:\[[ xX]\]\s+)?(.*)$`)
	mirrorIDRe      = regexp.MustCompile(`^\(([0-9a-z]{4})\)\s*(.*)$`)
	mirrorHeadingRe = regexp.MustCompile(`^##\s+(\S+)\s*$`)
)

func MirrorLine(t Task) string { return "- (" + t.ID + ") " + t.Text }

func MirrorHeading(month string) string { return "## " + month }

// ParseMirror reads one line as a bullet: its id when it carries a well-formed one, and
// its words. A parenthesised token that is not an id is just the start of the words.
func ParseMirror(line string) (id, text string, ok bool) {
	m := mirrorBulletRe.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	body := strings.TrimSpace(m[1])
	if g := mirrorIDRe.FindStringSubmatch(body); g != nil && IsID(g[1]) {
		return g[1], strings.TrimSpace(g[2]), true
	}
	return "", body, true
}

// ParseMirrorHeading is the month a run of bullets belongs to.
func ParseMirrorHeading(line string) (string, bool) {
	m := mirrorHeadingRe.FindStringSubmatch(line)
	if m == nil {
		return "", false
	}
	return m[1], true
}
