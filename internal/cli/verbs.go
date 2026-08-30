package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

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
	return " — no " + strings.Join(wants, " or ") + "; add with `tray 1 rewrite " +
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

// restore resolves ids against the same rows `list --all` prints, in the same order.
// Numbering the finished ones separately would have been tidier to implement and a
// trap to use: you read "2✓" off the screen and 2 would have meant something else.
func cmdRestore(req request) (string, error) {
	doc, items, err := view(req, true)
	if err != nil {
		return "", err
	}
	var names []string
	for _, t := range store.Resolve(items, req.ids) {
		if !t.Terminal() {
			continue // already open; restoring it would be a no-op worth not claiming
		}
		core.Restore(&t)
		doc.Set(t)
		names = append(names, t.Text)
	}
	if len(names) == 0 {
		return "nothing finished at those ids — tray list --all", nil
	}
	if err := doc.Save(); err != nil {
		return "", err
	}
	return "restored: " + strings.Join(names, " · "), nil
}

func cmdRewrite(req request) (string, error) {
	doc, items, err := view(req, false)
	if err != nil {
		return "", err
	}
	picked := store.Resolve(items, req.ids)
	if len(picked) == 0 {
		return "no match", nil
	}
	if len(req.tail) == 0 {
		return "the pickers land with the TUI — for now: tray N rewrite pri:M due:2026-08-20", nil
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
	return fmt.Sprintf("rewrote %d", len(picked)), nil
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
	return cmdRewrite(req)
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

// Emptying the whole tray is the largest single action here, so where it lands is
// never guessed. On a terminal that means asking; piped, it means saying so and
// stopping, because the agent surface must not start a conversation.
func cmdUnload(req request) (string, error) {
	to := req.opts.to
	if to == "" {
		if !interactive() {
			return "", fmt.Errorf("unload needs a month — tray unload --to %s", store.ThisMonth())
		}
		chosen, err := ui.PickMonth()
		if err != nil {
			return "", err
		}
		if chosen == "" {
			return "cancelled", nil
		}
		to = chosen
	}
	tray, items, err := view(request{scope: "tray"}, true)
	if err != nil {
		return "", err
	}
	picked := items
	if req.ids != "" {
		picked = store.Resolve(items, req.ids)
	}
	return unload(tray, picked, to)
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

// carryover is month → month and nothing else. It used to drain the whole tray into
// the closing month first, which is how running it mid-August emptied the tray into a
// July that never existed. Unload is its own ritual now, run first and by name.
//
// The source is never inferred: "the closing month" is not a fact about the calendar.
// Sweeping August into September is the same job whether you do it on the 30th, when
// August is the current month, or on the 10th, when it is the previous one.
func cmdCarryover(req request) (string, error) {
	if !req.opts.run && !req.opts.draft {
		if interactive() {
			return "", ui.RunSweep(req.opts.month)
		}
		return "", fmt.Errorf("not a terminal — tray carryover --run --month %s",
			store.PrevMonth(store.ThisMonth()))
	}
	source := req.opts.month
	if source == "" {
		return "", fmt.Errorf("carryover --run needs a month — try --month %s",
			store.PrevMonth(store.ThisMonth()))
	}
	target := store.NextMonth(source)

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
		return source + ": nothing to carry", nil
	}
	dst, err := store.Garage(target)
	if err != nil {
		return "", err
	}
	seen := dst.Texts()
	for _, t := range live {
		if !seen[t.Text] {
			dst.Add(carried(t, store.Today()))
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
	note := fmt.Sprintf("%d %s → %s", len(live), source, target)
	if req.opts.draft {
		note += "\nediting " + dst.Path + " — delete a line to drop it"
		_ = openEditor(dst.Path)
	}
	return note, nil
}

// carried is the copy that goes forward. A due date that has already passed does not:
// carrying a line forward is admitting the date did not hold, and keeping it means
// every re-take starts overdue with a junk urgency. The source keeps the original, so
// the record is still there.
func carried(t core.Task, today time.Time) core.Task {
	forward := t.Copy()
	if due, ok := core.Date(forward.Attrs["due"]); ok && due.Before(today) {
		delete(forward.Attrs, "due")
	}
	return forward
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
				"%s unresolved: %d items — tray carryover --run --month %s", month, live, month))
		}
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
