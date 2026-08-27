package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/store"
	"github.com/cheese-cracker/tray/internal/ui"
)

func cmdInit() (string, error) {
	if err := store.Ensure(store.TrayPath(), store.TrayHeader); err != nil {
		return "", err
	}
	month := store.ThisMonth()
	if err := store.Ensure(store.MonthPath(month), store.MonthHeader(month)); err != nil {
		return "", err
	}
	if err := store.Ensure(store.MonthPath(store.Someday), "# "+store.Someday); err != nil {
		return "", err
	}
	return "ready: " + store.Home(), nil
}

// cmdDump is capture. Only a leading to: and +tag are read; the rest is literal.
func cmdDump(req request) (string, error) {
	tail := req.tail
	month, tags := "", []string{}
	for len(tail) > 0 {
		if rest, ok := strings.CutPrefix(tail[0], "to:"); ok && rest != "" {
			month = rest
			tail = tail[1:]
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
	doc, err := store.Garage(month)
	if err != nil {
		return "", err
	}
	doc.Add(core.New(text, tags))
	if err := doc.Save(); err != nil {
		return "", err
	}
	if month == "" {
		month = store.ThisMonth()
	}
	return "→ " + month, nil
}

func cmdAdd(req request) (string, error) {
	mods := core.SplitMods(req.tail)
	delete(mods.Attrs, "to")
	text := strings.Join(mods.Words, " ")
	if text == "" {
		return "nothing to add", nil
	}
	task := core.New(text, mods.AddTags)
	core.ApplyMods(&task, core.Mods{Attrs: mods.Attrs})
	if task.Attrs["entry"] == "" {
		task.Attrs["entry"] = store.Today().Format(core.DateLayout)
	}

	doc, err := store.Tray()
	if err != nil {
		return "", err
	}
	doc.Add(task)
	if err := doc.Save(); err != nil {
		return "", err
	}
	return "added: " + text + missing(task), nil
}

// missing names what a tray task still wants. The garage asks for nothing; the tray
// is the layer where structure is the point.
func missing(t core.Task) string {
	var wants []string
	if t.Priority() == "" {
		wants = append(wants, "pri:H")
	}
	if t.Attrs["due"] == "" {
		wants = append(wants, "due:2026-08-20")
	}
	if len(wants) == 0 {
		return ""
	}
	return " — no " + strings.Join(wants, " or ") + "; add with `tray 1 modify " +
		strings.Join(wants, " ") + "`"
}

// cmdTake is the structuring step: garage → tray, source annotated, never twice.
func cmdTake(req request) (string, error) {
	garage, items, err := view(request{scope: "garage", opts: req.opts}, false)
	if err != nil {
		return "", err
	}
	if req.ids == "" {
		return "which one? `tray garage list` for the ids, then `tray 3 take`", nil
	}
	picked := store.Resolve(items, req.ids)
	if len(picked) == 0 {
		return "no match — `tray garage list` for the ids", nil
	}

	tray, err := store.Tray()
	if err != nil {
		return "", err
	}
	month := req.opts.month
	if month == "" {
		month = store.ThisMonth()
	}
	mods := core.SplitMods(req.tail)
	for _, t := range picked {
		fresh := core.Arrive(t, month, store.Today())
		if len(mods.Words) > 0 {
			fresh.Text = strings.Join(mods.Words, " ")
		}
		core.ApplyMods(&fresh, mods)
		tray.Add(fresh)

		core.Depart(&t, core.DestTray)
		garage.Set(t)
	}
	if err := tray.Save(); err != nil {
		return "", err
	}
	if err := garage.Save(); err != nil {
		return "", err
	}
	note := ""
	if len(picked) == 1 {
		note = missing(core.Arrive(picked[0], month, store.Today()))
	}
	return fmt.Sprintf("took %d", len(picked)) + note, nil
}

func cmdFinish(req request, as string) (string, error) {
	doc, items, err := view(req, false)
	if err != nil {
		return "", err
	}
	picked := store.Resolve(items, req.ids)
	if len(picked) == 0 {
		return "no match", nil
	}
	var names []string
	for _, t := range picked {
		core.Finish(&t, as, store.Today())
		doc.Set(t)
		names = append(names, t.Text)
	}
	if err := doc.Save(); err != nil {
		return "", err
	}
	return as + ": " + strings.Join(names, " · "), nil
}

func cmdModify(req request) (string, error) {
	doc, items, err := view(req, false)
	if err != nil {
		return "", err
	}
	picked := store.Resolve(items, req.ids)
	if len(picked) == 0 {
		return "no match", nil
	}
	if len(req.tail) == 0 {
		return "the pickers land with the TUI — for now: tray N modify pri:M due:2026-08-20", nil
	}
	mods := core.SplitMods(req.tail)
	for _, t := range picked {
		core.ApplyMods(&t, mods)
		if len(mods.Words) > 0 {
			t.Text = strings.Join(mods.Words, " ")
		}
		doc.Set(t)
	}
	if err := doc.Save(); err != nil {
		return "", err
	}
	return fmt.Sprintf("modified %d", len(picked)), nil
}

// cmdEdit hands the file to your editor — Hand B, made explicit.
func cmdEdit(req request) (string, error) {
	if req.ids == "" {
		path := store.TrayPath()
		if req.scope == "garage" {
			path = store.MonthPath(req.opts.month)
		}
		return "", openEditor(path)
	}
	if len(req.tail) == 0 {
		return "nothing to write", nil
	}
	return cmdModify(req)
}

func openEditor(path string) error {
	editor := os.Getenv("VISUAL")
	if editor == "" {
		editor = os.Getenv("EDITOR")
	}
	if editor == "" {
		editor = "vi"
	}
	parts := strings.Fields(editor)
	cmd := exec.Command(parts[0], append(parts[1:], path)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	return cmd.Run()
}

// cmdUnload hands the tray back to a garage month. Runnable any time.
func cmdUnload(req request) (string, error) {
	tray, items, err := view(request{scope: "tray"}, true)
	if err != nil {
		return "", err
	}
	picked := items
	if req.ids != "" {
		picked = store.Resolve(items, req.ids)
	}
	return unload(tray, picked, req.opts.to)
}

func unload(tray *store.Doc, picked []core.Task, to string) (string, error) {
	if len(picked) == 0 {
		return "tray empty", nil
	}
	month := to
	if month == "" {
		month = store.ThisMonth()
	}
	garage, err := store.Garage(month)
	if err != nil {
		return "", err
	}
	seen := garage.LiveTexts()
	moved := 0
	for _, t := range picked {
		switch {
		case t.Text == "" || seen[t.Text]: // already there: a second run is a no-op
		case garage.Reclaim(t): // it came from here, so bring that line home as it is
			seen[t.Text] = true
			moved++
		default:
			garage.Add(t.Copy())
			seen[t.Text] = true
			moved++
		}
		tray.Remove(t) // the tray always releases, whatever the garage did
	}
	if err := garage.Save(); err != nil {
		return "", err
	}
	if err := tray.Save(); err != nil {
		return "", err
	}
	return fmt.Sprintf("%d → %s", moved, month), nil
}

// cmdCarryover copies a closing month's live lines forward, annotating the source.
func cmdCarryover(req request) (string, error) {
	if !req.opts.all && !req.opts.draft {
		if interactive() {
			return "", ui.RunSweep(req.opts.month)
		}
		return "not a terminal — use --all or --draft", nil
	}
	source := req.opts.month
	if source == "" {
		source = store.PrevMonth(store.ThisMonth())
	}
	target := store.NextMonth(source)

	var notes []string
	tray, err := store.Tray()
	if err != nil {
		return "", err
	}
	if len(tray.Tasks()) > 0 { // the tray drains into the month it lived in
		note, err := unload(tray, tray.Tasks(), source)
		if err != nil {
			return "", err
		}
		notes = append(notes, note)
	}

	src, err := store.Garage(source)
	if err != nil {
		return "", err
	}
	var live []core.Task
	for _, t := range src.Live() {
		if t.Parsed() {
			live = append(live, t)
		}
	}
	if len(live) == 0 {
		return strings.Join(append(notes, source+": nothing to carry"), "\n"), nil
	}

	dst, err := store.Garage(target)
	if err != nil {
		return "", err
	}
	seen := dst.Texts()
	for _, t := range live {
		if !seen[t.Text] {
			dst.Add(t.Copy())
			seen[t.Text] = true
		}
		core.Depart(&t, target)
		src.Set(t)
	}
	if err := dst.Save(); err != nil {
		return "", err
	}
	if err := src.Save(); err != nil {
		return "", err
	}

	notes = append(notes, fmt.Sprintf("%d %s → %s", len(live), source, target))
	if req.opts.draft {
		notes = append(notes, "editing "+dst.Path+" — delete a line to drop it")
		if err := openEditor(dst.Path); err != nil {
			return strings.Join(notes, "\n"), nil
		}
	}
	return strings.Join(notes, "\n"), nil
}

func cmdStatus(req request) (string, error) {
	var stale []string
	for _, month := range store.Months() {
		if month >= store.ThisMonth() {
			continue
		}
		doc, err := store.Garage(month)
		if err != nil {
			continue
		}
		live := 0
		for _, t := range doc.Live() {
			if t.Parsed() {
				live++
			}
		}
		if live > 0 {
			stale = append(stale, fmt.Sprintf(
				"%s unresolved: %d items — run tray carryover --month %s", month, live, month))
		}
	}
	if req.opts.nag {
		return strings.Join(stale, "\n"), nil
	}
	_, items, err := view(request{scope: "tray"}, false)
	if err != nil {
		return "", err
	}
	line := fmt.Sprintf("tray: %d live · garage: %s", len(items), store.ThisMonth())
	return strings.Join(append(stale, line), "\n"), nil
}

func tagName(token string) (string, bool) {
	mods := core.SplitMods([]string{token})
	if len(mods.AddTags) == 1 {
		return mods.AddTags[0], true
	}
	return "", false
}
