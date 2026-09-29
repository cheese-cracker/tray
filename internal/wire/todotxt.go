package wire

import (
	"bufio"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/cheese-cracker/tray/internal/core"
)

// todo.txt has three priorities worth a letter to us. A fourth and beyond is still a
// priority, so it reads as low rather than as none.
var (
	letters    = map[string]string{"H": "A", "M": "B", "L": "C"}
	priorities = map[string]string{"A": "H", "B": "M"}
	dateRe     = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	priRe      = regexp.MustCompile(`^\(([A-Z])\)$`)
)

// ExportTodotxt writes one task per line in todo.txt's grammar: `x` and the dates for
// a finished one, `(A)` for a priority, `+tag`, then `key:value` extensions. A note has
// no line to live on, so it is left out and counted for the caller to say so.
func ExportTodotxt(items []core.Task) (text string, dropped int) {
	var lines []string
	for _, t := range items {
		var parts []string
		if t.Terminal() {
			parts = append(parts, "x", t.Done)
		} else if l := letters[t.Priority]; l != "" {
			parts = append(parts, "("+l+")")
		}
		if t.Entry != "" {
			parts = append(parts, t.Entry)
		}
		parts = append(parts, t.Text)
		for _, g := range t.Tags {
			parts = append(parts, "+"+g)
		}
		// The grammar drops the letter on completion; pri: is how todo.txt tools keep it.
		if l := letters[t.Priority]; t.Terminal() && l != "" {
			parts = append(parts, "pri:"+l)
		}
		for _, kv := range [][2]string{{"due", t.Due}, {"wait", t.Wait}, {"until", t.Until}, {"rec", t.Recur}} {
			if kv[1] != "" {
				parts = append(parts, kv[0]+":"+kv[1])
			}
		}
		if t.Note != "" {
			dropped++
		}
		lines = append(lines, strings.Join(parts, " "))
	}
	return strings.Join(lines, "\n"), dropped
}

// ImportTodotxt reads a todo.txt file. Both `+project` and `@context` are tags (9), and
// `t:` — the threshold most todo.txt tools use — is our wait.
func ImportTodotxt(r io.Reader, today time.Time) ([]core.Task, error) {
	var out []core.Task
	sc := bufio.NewScanner(r)
	for sc.Scan() {
		tokens := strings.Fields(sc.Text())
		if len(tokens) == 0 {
			continue
		}
		var t core.Task
		if tokens[0] == "x" {
			tokens = tokens[1:]
			if t.Done = takeDate(&tokens); t.Done == "" {
				t.Done = today.Format(core.DateLayout)
			}
		} else if m := priRe.FindStringSubmatch(tokens[0]); m != nil {
			t.Priority = letter(m[1])
			tokens = tokens[1:]
		}
		t.Entry = takeDate(&tokens)

		var words []string
		for _, tok := range tokens {
			if len(tok) > 1 && (tok[0] == '+' || tok[0] == '@') {
				t.Tags = appendTag(t.Tags, tok[1:])
				continue
			}
			if key, val, ok := strings.Cut(tok, ":"); ok && val != "" {
				switch key {
				case "due":
					t.Due = val
					continue
				case "wait", "t":
					t.Wait = val
					continue
				case "until":
					t.Until = val
					continue
				case "rec":
					t.Recur = val
					continue
				case "pri":
					t.Priority = letter(strings.ToUpper(val))
					continue
				}
			}
			words = append(words, tok)
		}
		if t.Text = strings.Join(words, " "); t.Text != "" {
			out = append(out, t)
		}
	}
	return out, sc.Err()
}

func letter(l string) string {
	if p, ok := priorities[l]; ok {
		return p
	}
	return "L"
}

// takeDate consumes a leading date token, if the next token is one.
func takeDate(tokens *[]string) string {
	if len(*tokens) == 0 || !dateRe.MatchString((*tokens)[0]) {
		return ""
	}
	d := (*tokens)[0]
	*tokens = (*tokens)[1:]
	return d
}
