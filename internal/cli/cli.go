// Package cli is the command surface — tray [filter] <verb> [mods] — and a client
// of core and store, holding no rules of its own.
package cli

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/charmbracelet/x/term"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
	"github.com/cheese-cracker/tray/internal/ui"
)

const Version = "0.2.0"

var verbs = []string{
	"init", "dump", "add", "take", "rewrite", "edit", "done", "drop",
	"unload", "carryover", "list", "head", "find", "print", "export", "status",
	"restore", "help",
}

var idSpec = regexp.MustCompile(`^\d+([,-]\d+)*$`)

const usage = `tray — two layers of markdown. Dump to the garage, take onto the tray.

  tray                              the tray, grouped by tag, ids on the left
  tray list                         the dense table: urgency, priority, due
  tray head [n]                     the top few, compactly. Silent when empty
  tray dump <text>                  → this month's garage; the tail is literal
  tray dump to:2026-11 +infra <text>
  tray add <desc> pri:H due:2026-08-12
  tray 3 take [pri:H due:...]        garage → tray, the structuring step
  tray 1 done  ·  tray 3 drop  ·  tray 2,5-7 done
  tray --all list  ·  tray 4 restore       see the finished; say one wasn't
  tray 2 rewrite pri:M               what the TUI runs on r
  tray 2 edit <new text>  ·  tray edit      one line, or the file in $EDITOR
  tray unload --to 2026-09           hand the tray back to a month, whole
  tray unload                        ... picks the month on a terminal
  tray carryover                     the sweep: prev · this · next · someday
  tray carryover --run --month 2026-08     ... headless; the month is required
  tray carryover --draft --month 2026-08   ... then hand-edit the target
  tray garage list  ·  tray +infra list  ·  tray list --all (with the finished)
  tray find <text>                   every layer, every month — repeats are rot
  tray print  ·  tray export  ·  tray status

Filters: bare ids (3, 2,5-7), +tag, and ` + "`garage`" + ` to switch layer.`

type options struct {
	json, all, run, draft, help, version bool
	month, to                            string
	unknown                              []string // rejected, not ignored
}

var valueFlags = map[string]bool{"--month": true, "--to": true}

func takeFlags(args []string) (options, []string) {
	var opts options
	var rest []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			opts.json = true
		case arg == "--all":
			opts.all = true
		case arg == "--run":
			opts.run = true
		case arg == "--draft":
			opts.draft = true
		case arg == "--help" || arg == "-h":
			opts.help = true
		case arg == "--version":
			opts.version = true
		case arg == "--plain" || arg == "--yes":
			// accepted and ignored: output is plain, and nothing here deletes
		case valueFlags[arg] && i+1 < len(args):
			if arg == "--month" {
				opts.month = args[i+1]
			} else {
				opts.to = args[i+1]
			}
			i++
		case strings.HasPrefix(arg, "--"):
			opts.unknown = append(opts.unknown, arg)
		default:
			rest = append(rest, arg)
		}
	}
	return opts, rest
}

type request struct {
	scope   string // "tray" or "garage"
	ids     string
	filters []string
	verb    string
	tail    []string
	opts    options
}

func parse(args []string) request {
	at := -1
	for i, a := range args {
		if contains(verbs, a) {
			at = i
			break
		}
	}
	req := request{scope: "tray"}
	var head []string
	if at < 0 {
		head = args
	} else {
		req.verb = args[at]
		head = args[:at]
		req.tail = args[at+1:]
	}

	req.opts, head = takeFlags(head)
	if req.verb != "dump" { // dump's tail is literal, flags and all
		tailOpts, tail := takeFlags(req.tail)
		req.tail = tail
		merge(&req.opts, tailOpts)
	}

	for _, tok := range head {
		switch {
		case tok == "garage":
			req.scope = "garage"
		case idSpec.MatchString(tok):
			req.ids = tok
		default:
			req.filters = append(req.filters, tok)
		}
	}
	return req
}

func merge(into *options, from options) {
	into.json = into.json || from.json
	into.all = into.all || from.all
	into.run = into.run || from.run
	into.unknown = append(into.unknown, from.unknown...)
	into.draft = into.draft || from.draft
	into.help = into.help || from.help
	into.version = into.version || from.version
	if from.month != "" {
		into.month = from.month
	}
	if from.to != "" {
		into.to = from.to
	}
}

// Run dispatches one invocation and returns an exit code.
func Run(args []string) int {
	req := parse(args)

	if req.opts.help || req.verb == "help" {
		fmt.Println(usage)
		return 0
	}
	if req.opts.version {
		fmt.Println("tray " + Version)
		return 0
	}
	if len(req.opts.unknown) > 0 {
		fmt.Fprintln(os.Stderr, "tray: unknown flag "+strings.Join(req.opts.unknown, " "))
		return 2
	}

	out, err := dispatch(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tray: "+err.Error())
		return 2
	}
	if out != "" {
		fmt.Println(out)
	}
	return 0
}

func dispatch(req request) (string, error) {
	switch req.verb {
	case "init":
		return cmdInit()
	case "dump":
		return cmdDump(req)
	case "add":
		return cmdAdd(req)
	case "take":
		return cmdTake(req)
	case "done":
		return cmdFinish(req, "done")
	case "restore":
		return cmdRestore(req)
	case "drop":
		return cmdFinish(req, "dropped")
	case "rewrite":
		return cmdRewrite(req)
	case "edit":
		return cmdEdit(req)
	case "unload":
		return cmdUnload(req)
	case "carryover":
		return cmdCarryover(req)
	case "find":
		return cmdFind(req)
	case "print":
		return cmdPrint(req)
	case "export":
		req.opts.json = true
		return cmdReport(req, true)
	case "status":
		return cmdStatus(req)
	case "list":
		return cmdReport(req, true)
	case "head":
		return cmdHead(req)
	default:
		// Bare tray on a terminal is the interface; piped, it stays text so an
		// agent can never be handed a UI.
		if req.verb == "" && req.ids == "" && len(req.filters) == 0 && interactive() {
			return "", ui.Run()
		}
		return cmdReport(req, false)
	}
}

// view returns the document and the canonical order every id indexes into.
func view(req request, everything bool) (*store.Doc, []core.Task, error) {
	if req.scope == "garage" {
		doc, err := store.Garage(req.opts.month)
		if err != nil {
			return nil, nil, err
		}
		var items []core.Task
		for _, t := range doc.Tasks() {
			if t.Parsed() && (everything || t.Live()) && matches(t, req.filters) {
				items = append(items, t)
			}
		}
		return doc, items, nil
	}

	doc, err := store.Tray()
	if err != nil {
		return nil, nil, err
	}
	var items []core.Task
	for _, t := range doc.Tasks() {
		if (everything || t.Live()) && matches(t, req.filters) {
			items = append(items, t)
		}
	}
	today := store.Today()
	sort.SliceStable(items, func(i, j int) bool {
		return core.Urgency(items[i], today) > core.Urgency(items[j], today)
	})
	return doc, items, nil
}

func matches(t core.Task, filters []string) bool {
	for _, f := range filters {
		switch {
		case strings.HasPrefix(f, "+"), strings.HasPrefix(f, "#"):
			if !contains(t.Tags, f[1:]) {
				return false
			}
		case strings.Contains(f, ":"):
			key, val, _ := strings.Cut(f, ":")
			if !strings.EqualFold(t.Attrs[key], val) {
				return false
			}
		default:
			if !strings.Contains(strings.ToLower(t.Text), strings.ToLower(f)) {
				return false
			}
		}
	}
	return true
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// interactive is both ends. Stat-and-check-chardevice is not enough: /dev/null is
// a character device too, so redirected output would have looked like a terminal.
func interactive() bool {
	return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd())
}
