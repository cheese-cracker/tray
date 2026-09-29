package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/plugin"
	"github.com/cheese-cracker/tray/internal/store"
	"github.com/cheese-cracker/tray/internal/wire"
)

// init is a receipt: opening the store already made the home and the database, so
// what it uniquely gives is a line saying where the data lives (97b).
func cmdInit() (string, error) {
	return "ready: " + store.Home(), nil
}

// cmdDump is capture. Only a leading to:, --note and +tag are read; the rest is literal.
func cmdDump(s *store.Store, req request) (string, error) {
	tail := req.tail
	month, tags := "", []string{}
	note := req.opts.note
	for len(tail) > 0 {
		if rest, ok := strings.CutPrefix(tail[0], "to:"); ok && rest != "" {
			month = rest
			tail = tail[1:]
			continue
		}
		// The tail is literal (F3), so this is read here rather than as a flag — one
		// more leading token, alongside to: and the tag, and nothing past the first
		// word of text is ever parsed.
		if tail[0] == "--note" && len(tail) > 1 {
			note = tail[1]
			tail = tail[2:]
			continue
		}
		if name, ok := tagName(tail[0]); ok {
			tags = append(tags, name)
			tail = tail[1:]
			continue
		}
		break
	}

	text := strings.Join(tail, " ")
	if text == "" {
		return "nothing to dump", nil
	}
	if month == "" {
		month = store.ThisMonth()
	}
	task := core.New(text, tags)
	task.Month, task.Note = month, note
	if err := s.Put(&task); err != nil {
		return "", err
	}
	return "→ " + month, nil
}

func cmdAdd(s *store.Store, req request) (string, error) {
	mods := core.SplitMods(req.tail)
	delete(mods.Attrs, "to")
	text := strings.Join(mods.Words, " ")
	if text == "" {
		return "nothing to add", nil
	}
	task := core.New(text, mods.AddTags)
	task.Note = req.opts.note
	core.ApplyMods(&task, core.Mods{Attrs: mods.Attrs})
	// A waiting task lies in the garage of its day until sync lifts it, so adding one
	// is a dump with a date rather than a tray task you cannot see.
	if d, ok := core.Date(task.Wait); ok {
		task.Layer, task.Month = core.LayerGarage, d.Format("2006-01")
	} else {
		task.Layer = core.LayerTray
	}
	if err := s.Put(&task); err != nil {
		return "", err
	}
	if task.Layer == core.LayerGarage {
		return fmt.Sprintf("added: %s → %s, waiting until %s", text, task.Month, core.Day(task.Wait)), nil
	}
	return "added: " + text + missing(task), nil
}

// missing names what a tray task still wants. The garage asks for nothing; the tray
// is the layer where structure is the point.
func missing(t core.Task) string {
	var wants []string
	if t.Priority == "" {
		wants = append(wants, "pri:H")
	}
	if t.Due == "" {
		wants = append(wants, "due:2026-08-20")
	}
	if len(wants) == 0 {
		return ""
	}
	return fmt.Sprintf(" — no %s; add with `tray %d rewrite %s`",
		strings.Join(wants, " or "), t.ID, strings.Join(wants, " "))
}

// cmdTake is the structuring step: garage → tray. Nothing is copied and nothing is
// taken twice — a row already on the tray is left where it is.
func cmdTake(s *store.Store, req request) (string, error) {
	if req.ids == "" {
		return "which one? `tray garage list` for the ids, then `tray 12 take`", nil
	}
	picked, err := pick(s, req)
	if err != nil {
		return "", err
	}
	if len(picked) == 0 {
		return "no match — `tray garage list` for the ids", nil
	}
	mods := core.SplitMods(req.tail)
	took, last := 0, core.Task{}
	err = s.Update(func(tx *store.Store) error {
		for _, t := range picked {
			if t.Layer == core.LayerTray {
				continue
			}
			core.Move(&t, core.LayerTray, "")
			t.Wait = "" // you took it; its day is now
			if len(mods.Words) > 0 {
				t.Text = strings.Join(mods.Words, " ")
			}
			core.ApplyMods(&t, mods)
			if err := tx.Put(&t); err != nil {
				return err
			}
			took, last = took+1, t
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if took == 0 {
		return "already on the tray", nil
	}
	note := ""
	if took == 1 {
		note = missing(last)
	}
	return fmt.Sprintf("took %d", took) + note, nil
}

func cmdFinish(s *store.Store, req request) (string, error) {
	picked, err := pick(s, req)
	if err != nil {
		return "", err
	}
	if len(picked) == 0 {
		return "no match", nil
	}
	today := store.Today()
	var names []string
	err = s.Update(func(tx *store.Store) error {
		for _, t := range picked {
			core.Finish(&t, today)
			if err := tx.Put(&t); err != nil {
				return err
			}
			names = append(names, t.Text)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return "done: " + strings.Join(names, " · "), nil
}

func cmdRestore(s *store.Store, req request) (string, error) {
	picked, err := pick(s, req)
	if err != nil {
		return "", err
	}
	var names []string
	err = s.Update(func(tx *store.Store) error {
		for _, t := range picked {
			if !t.Terminal() {
				continue // already open; restoring it would be a no-op worth not claiming
			}
			core.Restore(&t)
			if err := tx.Put(&t); err != nil {
				return err
			}
			names = append(names, t.Text)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if len(names) == 0 {
		return "nothing finished at those ids — tray list --all", nil
	}
	return "restored: " + strings.Join(names, " · "), nil
}

// erase is the one verb that removes a row. Everything else marks. This is for a line
// that should not have been written — a typo, a duplicate.
func cmdErase(s *store.Store, req request) (string, error) {
	picked, err := pick(s, req)
	if err != nil {
		return "", err
	}
	if len(picked) == 0 {
		return "no match", nil
	}
	var names []string
	err = s.Update(func(tx *store.Store) error {
		for _, t := range picked {
			if err := tx.Delete(t.ID); err != nil {
				return err
			}
			names = append(names, t.Text)
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return "erased: " + strings.Join(names, " · "), nil
}

// note sets the lines under a task, or prints them when given nothing to set. One
// note per task, replaced whole: a bag of tasks keeps no history of one (104).
func cmdNote(s *store.Store, req request) (string, error) {
	picked, err := pick(s, req)
	if err != nil {
		return "", err
	}
	if len(picked) == 0 {
		return "no match", nil
	}
	text := strings.TrimSpace(strings.Join(req.tail, " "))
	if text == "" {
		var out []string
		for _, t := range picked {
			if t.Note == "" {
				out = append(out, t.Text+": no note")
			} else {
				out = append(out, t.Text+":\n  "+strings.ReplaceAll(t.Note, "\n", "\n  "))
			}
		}
		return strings.Join(out, "\n"), nil
	}
	err = s.Update(func(tx *store.Store) error {
		for _, t := range picked {
			t.Note = text
			if err := tx.Put(&t); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("noted %d", len(picked)), nil
}

func cmdRewrite(s *store.Store, req request) (string, error) {
	picked, err := pick(s, req)
	if err != nil {
		return "", err
	}
	if len(picked) == 0 {
		return "no match", nil
	}
	if len(req.tail) == 0 {
		return "nothing to change — tray 12 rewrite pri:M due:2026-08-20 +tag", nil
	}
	mods := core.SplitMods(req.tail)
	err = s.Update(func(tx *store.Store) error {
		for _, t := range picked {
			core.ApplyMods(&t, mods)
			if len(mods.Words) > 0 {
				t.Text = strings.Join(mods.Words, " ")
			}
			if err := tx.Put(&t); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("rewrote %d", len(picked)), nil
}

// edit is the words alone, attributes untouched. There is no file to hand to an editor
// any more, so a bare edit says so rather than guessing what you meant.
func cmdEdit(s *store.Store, req request) (string, error) {
	if req.ids == "" {
		return "", fmt.Errorf("there is no file to open — tray <id> edit <new text>")
	}
	if len(req.tail) == 0 {
		return "nothing to write", nil
	}
	picked, err := pick(s, req)
	if err != nil {
		return "", err
	}
	if len(picked) == 0 {
		return "no match", nil
	}
	text := strings.Join(req.tail, " ")
	err = s.Update(func(tx *store.Store) error {
		for _, t := range picked {
			t.Text = text
			if err := tx.Put(&t); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("edited %d", len(picked)), nil
}

// Emptying the whole tray is the largest single action here, so where it lands is
// never guessed (72): one task knows the month it came from; the whole tray needs --to.
func cmdUnload(s *store.Store, req request) (string, error) {
	to := req.opts.to
	var picked []core.Task
	var err error
	if req.ids != "" {
		picked, err = pick(s, req)
	} else {
		if to == "" {
			return "", fmt.Errorf("unload needs a month — tray unload --to %s", store.ThisMonth())
		}
		picked, err = s.Tasks(store.Filter{Layer: core.LayerTray, All: true})
	}
	if err != nil {
		return "", err
	}
	moved := 0
	err = s.Update(func(tx *store.Store) error {
		for _, t := range picked {
			if t.Layer != core.LayerTray || t.Recur != "" {
				continue // a template is the tray's own furniture, not work to hand back
			}
			if to == "" && t.FromMonth == "" {
				return fmt.Errorf("%d never came from a month — tray %d unload --to %s", t.ID, t.ID, store.ThisMonth())
			}
			core.Move(&t, core.LayerGarage, to)
			if err := tx.Put(&t); err != nil {
				return err
			}
			moved++
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if moved == 0 {
		return "tray empty", nil
	}
	if to == "" {
		return fmt.Sprintf("%d → home", moved), nil
	}
	return fmt.Sprintf("%d → %s", moved, to), nil
}

// carryover is month → month and nothing else (71). The source is never inferred (69,
// 70): sweeping August into September is the same job on the 30th, when August is the
// current month, and on the 10th, when it is the previous one.
func cmdCarryover(s *store.Store, req request) (string, error) {
	if !req.opts.run {
		return "", fmt.Errorf("carryover is headless — tray carryover --run --month %s",
			store.PrevMonth(store.ThisMonth()))
	}
	source := req.opts.month
	if source == "" {
		return "", fmt.Errorf("carryover --run needs a month — try --month %s",
			store.PrevMonth(store.ThisMonth()))
	}
	target := store.NextMonth(source)
	live, err := s.Tasks(store.Filter{Layer: core.LayerGarage, Month: source})
	if err != nil {
		return "", err
	}
	if len(live) == 0 {
		return source + ": nothing to carry", nil
	}
	today := store.Today()
	err = s.Update(func(tx *store.Store) error {
		for _, t := range live {
			// Carrying a line forward is admitting the date did not hold; keeping it
			// means every re-take starts overdue with a junk urgency (75).
			if due, ok := core.Date(t.Due); ok && due.Before(today) {
				t.Due = ""
			}
			core.Move(&t, core.LayerGarage, target)
			if err := tx.Put(&t); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%d %s → %s", len(live), source, target), nil
}

func cmdStatus(s *store.Store) (string, error) {
	months, err := s.Months()
	if err != nil {
		return "", err
	}
	var lines []string
	for _, month := range months {
		if month >= store.ThisMonth() {
			continue
		}
		live, err := s.Tasks(store.Filter{Layer: core.LayerGarage, Month: month})
		if err != nil {
			return "", err
		}
		if len(live) > 0 {
			lines = append(lines, fmt.Sprintf(
				"%s unresolved: %d items — tray carryover --run --month %s", month, len(live), month))
		}
	}
	tray, err := s.Tasks(store.Filter{Layer: core.LayerTray})
	if err != nil {
		return "", err
	}
	all, err := s.Tasks(store.Filter{All: true})
	if err != nil {
		return "", err
	}
	today := store.Today()
	waiting, templates := 0, 0
	for _, t := range all {
		if t.Layer == core.LayerGarage && t.Waiting(today) {
			waiting++
		}
		if until, ended := core.Date(t.Until); t.Recur != "" && !(ended && until.Before(today)) {
			templates++
		}
	}
	lines = append(lines, fmt.Sprintf("tray: %d live · waiting %d · templates %d · garage %s",
		len(tray), waiting, templates, store.ThisMonth()))
	return strings.Join(lines, "\n"), nil
}

// cmdPlugin lists what is installed, and that is deliberately all it does. A verb
// that pulls on demand is the shape --nag was deleted for (76): the sync belongs to
// opening the tab in the sweep, so there is one sync point and you cannot forget it.
func cmdPlugin(s *store.Store, req request) (string, error) {
	if len(req.tail) > 0 && req.tail[0] != "list" {
		return "", fmt.Errorf("tray plugin lists what is installed — syncing happens in `tray carryover`")
	}
	found := plugin.List()
	if len(found) == 0 {
		return "no plugins — one is a folder in " + plugin.Dir() +
			" holding an executable `" + plugin.Runner + "`", nil
	}
	var rows []string
	for _, p := range found {
		desc, err := describe(s, p)
		if err != nil {
			return "", err
		}
		rows = append(rows, fmt.Sprintf("%-12s %s", p.Name, desc))
	}
	return strings.Join(rows, "\n"), nil
}

// describe says what a plugin is doing here: the garage it keeps, as rows in the
// store, and the verbs it puts in the menu. A plugin with verbs and no runner keeps no
// garage, so none is claimed for it.
func describe(s *store.Store, p plugin.Plugin) (string, error) {
	var parts []string
	if p.Run != "" {
		rows, err := s.Tasks(store.Filter{Layer: core.LayerGarage, Month: p.Garage(), All: true})
		if err != nil {
			return "", err
		}
		parts = append(parts, "garage "+count(len(rows), "row"))
	}
	if len(p.Verbs) > 0 {
		parts = append(parts, "enter → "+strings.Join(p.Verbs, ", "))
	}
	return strings.Join(parts, " · "), nil
}

func count(n int, noun string) string {
	switch n {
	case 0:
		return "empty"
	case 1:
		return "1 " + noun
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

// find is search across every layer and every month. The finished are out unless you
// ask, as everywhere else.
func cmdFind(s *store.Store, req request) (string, error) {
	needle := strings.Join(req.tail, " ")
	if needle == "" {
		return "nothing to find", nil
	}
	hits, err := s.Tasks(store.Filter{Text: needle, All: req.opts.all})
	if err != nil {
		return "", err
	}
	if len(hits) == 0 {
		return "no match", nil
	}
	sort.SliceStable(hits, func(i, j int) bool { return where(hits[i]) < where(hits[j]) })
	var rows [][]string
	for _, t := range hits {
		rows = append(rows, []string{where(t), strings.TrimPrefix(core.Line(t, false), "- ")})
	}
	return table(rows, []string{"WHERE", "LINE"}), nil
}

func where(t core.Task) string {
	if t.Layer == core.LayerTray {
		return "tray"
	}
	return t.Month
}

// export is the tray in another tool's shape. Taskwarrior JSON by default, because
// `tray export | task import` is the promise 4 keeps; todo.txt and markdown bullets are
// the other two grammars tray already speaks.
func cmdExport(s *store.Store, req request) (string, error) {
	items, err := view(s, req, req.opts.all)
	if err != nil {
		return "", err
	}
	today := store.Today()
	switch req.opts.format {
	case "", "tw":
		return wire.ExportTaskwarrior(items, today)
	case "todotxt":
		text, dropped := wire.ExportTodotxt(items)
		if dropped > 0 {
			fmt.Fprintf(os.Stderr, "tray: %d note(s) have no line in todo.txt and were left out\n", dropped)
		}
		return text, nil
	case "md":
		return grouped(items, today, false), nil
	}
	return "", fmt.Errorf("unknown format %q — tw, todotxt or md", req.opts.format)
}

// import never guesses a format (18): the same line reads differently in each grammar.
func cmdImport(s *store.Store, req request) (string, error) {
	today := store.Today()
	switch req.opts.format {
	case "md":
		if len(req.tail) == 0 {
			return "", fmt.Errorf("import needs a path — tray import --format md ~/tray")
		}
		return wire.ImportMarkdown(s, req.tail[0], today)
	case "tw", "todotxt":
		r, name, err := input(req.tail)
		if err != nil {
			return "", err
		}
		defer r.Close()
		var tasks []core.Task
		if req.opts.format == "tw" {
			tasks, err = wire.ImportTaskwarrior(r, today)
		} else {
			tasks, err = wire.ImportTodotxt(r, today)
		}
		if err != nil {
			return "", fmt.Errorf("%s: %w", name, err)
		}
		added, updated, skipped, err := wire.Land(s, tasks, today)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s: %d imported, %d updated, %d skipped", name, added, updated, skipped), nil
	}
	return "", fmt.Errorf("import needs a format — tray import --format tw|todotxt|md [file|-]")
}

// input is the file named, or stdin when none is or `-` stands in for one.
func input(tail []string) (io.ReadCloser, string, error) {
	if len(tail) == 0 || tail[0] == "-" {
		return os.Stdin, "stdin", nil
	}
	f, err := os.Open(tail[0])
	return f, filepath.Base(tail[0]), err
}

// context is what you paste to an agent: the rows you named, or the report you would
// have read, with every note under its task.
func cmdContext(s *store.Store, req request) (string, error) {
	var items []core.Task
	var err error
	if req.ids != "" {
		items, err = pick(s, req)
	} else {
		items, err = view(s, req, req.opts.all)
	}
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "nothing to copy", nil
	}
	return contextReport(items, store.Today()), nil
}

func tagName(token string) (string, bool) {
	mods := core.SplitMods([]string{token})
	if len(mods.AddTags) == 1 {
		return mods.AddTags[0], true
	}
	return "", false
}
