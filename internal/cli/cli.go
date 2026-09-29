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

	"github.com/cheese-cracker/tray/internal/config"
	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/plugin"
	"github.com/cheese-cracker/tray/internal/store"
	"github.com/cheese-cracker/tray/internal/sync"
	"github.com/cheese-cracker/tray/internal/ui"
)

const Version = "0.3.0"

var verbs = []string{
	"init", "config", "dump", "add", "take", "rewrite", "edit", "note", "done", "erase",
	"unload", "carryover", "list", "head", "find", "print", "export", "import", "context",
	"sync", "status", "restore", "plugin", "help",
}

// An id token is one id or a comma list of them; every part must be shaped like one, so
// a four-letter word is a filter and a four-character id is an id.
var idSpec = regexp.MustCompile(`^[0-9a-z]{4}(,[0-9a-z]{4})*$`)

func isIDList(tok string) bool {
	if !idSpec.MatchString(tok) {
		return false
	}
	for _, part := range strings.Split(tok, ",") {
		if !core.IsID(part) {
			return false
		}
	}
	return true
}

const usage = `tray — two layers, one database. Dump to the garage, take onto the tray.

  tray                              the tray, grouped by tag, ids on the left
  tray list                         the dense table: urgency, priority, due
  tray head [n]                     the top few, compactly. Silent when empty
  tray dump <text>                  → this month's garage; the tail is literal
  tray dump to:2026-11 +infra <text>
  tray add <desc> pri:H due:2026-08-12
  tray add <desc> wait:2027-03-13   → the garage of that month, until that day
  tray k79l take [pri:H due:...]     garage → tray, the structuring step
  tray k79l done  ·  tray k79l,79ya done  ·  tray k79l erase   erase removes the row
  tray k79l note <text>              a few lines of context under a task; --note on dump/add
  tray --all list  ·  tray k79l restore    see the finished; say one wasn't
  tray k79l rewrite pri:M            every field — recur: wait: until: too
  tray k79l edit <new text>          the words alone
  tray unload --to 2026-09           hand the tray back to a month, whole
  tray k79l unload                   one task, back to the month it came from
  tray carryover --run --month 2026-08     that month's leftovers move to the next
  tray garage list  ·  tray +infra list  ·  tray list --all (with the finished)
  tray find <text>                   every layer, every month
  tray print  ·  tray status
  tray config                        where the config file is and what it says, secrets masked
  tray export [--format tw|todotxt|md] [--all]    Taskwarrior JSON by default
  tray import --format tw|todotxt [file|-]        from a file or stdin; --format md ~/tray for the old home
  tray context [ids]                 the report with ids and every note, for pasting to an agent
  tray add <desc> recur:weekly due:2026-10-03    a template; sync keeps one live child of it
  tray sync [--plugin <name>] [--apply] [--json]  the one event: recurrence and waiting rows land,
                                     garage.md is read back, plugin plans print — and land only under --apply

Ids are permanent, four characters. Filters: ids (k79l, k79l,79ya), +tag, key:value, and
` + "`garage`" + ` to switch layer. tray.md and garage.md beside the database mirror the open
tasks; add a bullet to garage.md and the next sync brings it in.`

type options struct {
	json, all, run, apply, help, version     bool
	month, to, note, format, plugin, timeout string
	unknown                                  []string // rejected, not ignored
}

var valueFlags = map[string]bool{
	"--month": true, "--to": true, "--note": true, "--format": true, "--plugin": true, "--timeout": true,
}

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
		case arg == "--apply":
			opts.apply = true
		case arg == "--help" || arg == "-h":
			opts.help = true
		case arg == "--version":
			opts.version = true
		case arg == "--plain" || arg == "--yes":
			// accepted and ignored: output is plain, and nothing here deletes
		case valueFlags[arg] && i+1 < len(args):
			switch arg {
			case "--month":
				opts.month = args[i+1]
			case "--to":
				opts.to = args[i+1]
			case "--note":
				opts.note = args[i+1]
			case "--format":
				opts.format = args[i+1]
			case "--plugin":
				opts.plugin = args[i+1]
			case "--timeout":
				opts.timeout = args[i+1]
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
		case isIDList(tok):
			req.ids = tok
		default:
			req.filters = append(req.filters, tok)
		}
	}
	return req
}

func merge(into *options, from options) {
	if from.note != "" {
		into.note = from.note
	}
	if from.format != "" {
		into.format = from.format
	}
	if from.plugin != "" {
		into.plugin = from.plugin
	}
	if from.timeout != "" {
		into.timeout = from.timeout
	}
	into.json = into.json || from.json
	into.all = into.all || from.all
	into.run = into.run || from.run
	into.apply = into.apply || from.apply
	into.unknown = append(into.unknown, from.unknown...)
	into.help = into.help || from.help
	into.version = into.version || from.version
	if from.month != "" {
		into.month = from.month
	}
	if from.to != "" {
		into.to = from.to
	}
}

// pluginUsage advertises `tray plugin` only once you have one. Installing a plugin is
// opt-in, so until you do, the help is the help tray has always printed — a verb that
// can only ever answer "no plugins" has not earned a line in a usage block this short.
func pluginUsage() string {
	if len(plugin.List()) == 0 {
		return ""
	}
	return "\n\n  tray plugin  ·  tray plugin check     what is installed and how it is: state, hooks, settings, last run; check runs each probe"
}

// Run dispatches one invocation and returns an exit code.
func Run(args []string) int {
	req := parse(args)

	if req.opts.help || req.verb == "help" {
		fmt.Println(usage + pluginUsage())
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

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintln(os.Stderr, "tray: "+err.Error())
		return 2
	}
	if req.verb == "config" {
		fmt.Println(cmdConfig(cfg))
		return 0
	}
	s, err := store.Open(store.Home())
	if err != nil {
		fmt.Fprintln(os.Stderr, "tray: "+err.Error())
		return 2
	}
	defer s.Close()

	out, err := dispatch(s, req)
	if err != nil {
		fmt.Fprintln(os.Stderr, "tray: "+err.Error())
		return 2
	}
	if out != "" {
		fmt.Println(out)
	}
	// Whatever the verb did, the mirror shows it (the write event, see sync.Hooks). A
	// read fires it too, which costs a compare and writes nothing.
	if _, err := sync.Fire(s, sync.Write, store.Today()); err != nil {
		fmt.Fprintln(os.Stderr, "tray: "+err.Error())
		return 2
	}
	return 0
}

func dispatch(s *store.Store, req request) (string, error) {
	switch req.verb {
	case "init":
		return cmdInit()
	case "dump":
		return cmdDump(s, req)
	case "add":
		return cmdAdd(s, req)
	case "take":
		return cmdTake(s, req)
	case "done":
		return cmdFinish(s, req)
	case "restore":
		return cmdRestore(s, req)
	case "erase":
		return cmdErase(s, req)
	case "note":
		return cmdNote(s, req)
	case "rewrite":
		return cmdRewrite(s, req)
	case "edit":
		return cmdEdit(s, req)
	case "unload":
		return cmdUnload(s, req)
	case "carryover":
		return cmdCarryover(s, req)
	case "find":
		return cmdFind(s, req)
	case "print":
		return cmdPrint(s, req)
	case "export":
		return cmdExport(s, req)
	case "import":
		return cmdImport(s, req)
	case "context":
		return cmdContext(s, req)
	case "sync":
		return cmdSync(s, req)
	case "status":
		return cmdStatus(s)
	case "plugin":
		return cmdPlugin(s, req)
	case "list":
		return cmdReport(s, req, true)
	case "head":
		return cmdHead(s, req)
	default:
		// Bare tray on a terminal is the interface; piped, it stays text so an
		// agent can never be handed a UI (20).
		if req.verb == "" && req.ids == "" && len(req.filters) == 0 && interactive() {
			return "", ui.Run(s)
		}
		return cmdReport(s, req, false)
	}
}

// interactive is both ends. Stat-and-check-chardevice is not enough: /dev/null is
// a character device too, so redirected output would have looked like a terminal.
func interactive() bool {
	return term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd())
}

// view is the rows a report shows: the layer the request names, live unless asked for
// everything, narrowed by its filters. The tray reads in urgency order, live rows above
// finished ones; a garage reads oldest first.
func view(s *store.Store, req request, everything bool) ([]core.Task, error) {
	f := store.Filter{Layer: core.LayerTray, All: everything}
	if req.scope == "garage" {
		f.Layer, f.Month = core.LayerGarage, req.opts.month
		if f.Month == "" {
			f.Month = store.ThisMonth()
		}
	}
	var words []string
	for _, tok := range req.filters {
		switch {
		case strings.HasPrefix(tok, "+"), strings.HasPrefix(tok, "#"):
			f.Tags = append(f.Tags, tok[1:])
		case strings.Contains(tok, ":"):
			key, val, _ := strings.Cut(tok, ":")
			if f.Attrs == nil {
				f.Attrs = map[string]string{}
			}
			f.Attrs[key] = val
		default:
			words = append(words, tok)
		}
	}
	f.Text = strings.Join(words, " ")

	items, err := s.Tasks(f)
	if err != nil || req.scope == "garage" {
		return items, err
	}
	today := store.Today()
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Terminal() != items[j].Terminal() {
			return !items[i].Terminal()
		}
		return core.Urgency(items[i], today) > core.Urgency(items[j], today)
	})
	return items, nil
}

// pick resolves the request's ids against every row, whatever its layer or state. An
// id is permanent, so there is no report to index into and no second id space for the
// finished — 82 and 93b went with positional ids.
func pick(s *store.Store, req request) ([]core.Task, error) {
	ids := store.ParseIDs(req.ids)
	if len(ids) == 0 {
		return nil, nil
	}
	return s.Tasks(store.Filter{IDs: ids, All: true})
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
