package gui

import (
	"context"
	"fmt"
	"image/color"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"

	"github.com/cheese-cracker/tray/internal/core"
	"github.com/cheese-cracker/tray/internal/plugin"
	"github.com/cheese-cracker/tray/internal/store"
	"github.com/cheese-cracker/tray/internal/style"
)

// The command palette is the action menu (24): everything that applies right now, one
// list, typed at. A verb keeps its letter beside it, so the palette teaches the shortcut
// it is standing in for; a plugin verb has no letter and lives here alone (105a). Text
// that matches nothing is a capture, so the palette is also the fastest way to dump.

type command struct {
	label string
	key   string // the shortcut, right-aligned; "" for a verb that has none
	words string // what else the search may hit: "finish complete" for done
	run   func()
}

// commands is what applies to the current layer in the current mode, then the doors out
// of it, then the plugins' verbs. Nothing here decides — every entry calls the one
// dispatcher, so the palette can never disagree with a letter or a row button.
func (u *ui) commands(l *taskList) []command {
	var out []command
	add := func(label, key, words string, run func()) {
		out = append(out, command{label, key, words, run})
	}
	verb := func(key, label, words string) {
		add(label, key, words, func() { u.do(key, l) })
	}
	switch u.mode {
	case modeHome:
		if l.layer == core.LayerGarage {
			verb("t", "take onto the tray", "structure priority due")
			verb("#", "tag", "label")
			verb("n", "note", "context lines")
			verb("r", "rewrite", "edit words change")
			verb(">", "move to a month", "garage someday next")
		} else {
			verb("x", "done", "finish complete close")
			verb("d", "hand back to the garage", "return revive")
			verb(">", "move to a month", "garage someday next")
			verb("r", "rewrite", "edit change priority due")
			verb("#", "tag", "label")
			verb("n", "note", "context lines")
		}
		verb("l", "open the pane", "details ladder")
		verb("c", "copy context", "clipboard agent")
		verb("v", "review", "finished restore erase")
		if l.layer == core.LayerGarage {
			add("carry forward…", "", "sweep month turn carryover", u.openSweep)
			add("go to the tray", "tab", "switch layer", u.switchLayer)
		} else {
			add("hand the tray back…", "", "unload empty month", u.openUnload)
			add("go to the garage", "tab", "switch layer", u.switchLayer)
		}
		verb("s", "sync", "plugins recurrence refresh pull")
		verb("p", "plugins", "installed settings")
		verb("/", "filter", "search find")
		// Offered on both layers, never in review (105c).
		for _, a := range plugin.Actions() {
			a := a
			add("plugin · "+a.Verb, "", a.Plugin+" "+a.Verb, func() { u.runVerb(a, l) })
		}
	case modeReview:
		if t, ok := l.cursorTask(); ok && t.Done != "" {
			verb("R", "restore", "unfinish undo")
		}
		verb("E", "erase", "delete remove")
		verb("l", "open the pane", "details")
		verb("c", "copy context", "clipboard agent")
		add("leave review", "v", "esc back home", u.leave)
	case modeSweep:
		verb("t", "take onto the tray", "structure")
		verb(">", "move to a month", "garage someday next")
		verb("l", "open the pane", "details")
		add("leave the sweep", "esc", "back home", u.leave)
	}
	verb("?", "help", "keys shortcuts page")
	verb("q", "quit", "exit close")
	return out
}

// runVerb hands a plugin's verb the picked ids, as the menu did (105): the verb owns
// what happens next, and tray reads the world again when it is back. It runs off the UI
// thread with no clock on it — a verb may open a browser and wait for you.
func (u *ui) runVerb(a plugin.Action, l *taskList) {
	var args []string
	for _, t := range u.targets(l) {
		args = append(args, strconv.FormatInt(t.ID, 10))
	}
	u.say("running " + a.Plugin + " · " + a.Verb + "…")
	var err error
	background(func() {
		_, err = plugin.Run(context.Background(), plugin.Exec{
			Dir: filepath.Dir(filepath.Dir(a.Path)), Path: a.Path, Args: args,
			Env: []string{"TRAY_LAYER=" + l.layer, "TRAY_IDS=" + strings.Join(args, ",")},
		})
	}, func() {
		if err != nil {
			u.fail(fmt.Errorf("%s · %s: %w", a.Plugin, a.Verb, err))
			return
		}
		u.flash = a.Plugin + " · " + a.Verb + ": done"
		u.reload()
	})
}

// paletteWidth is the card's width: wide enough for a label and its key, never a bar.
const paletteWidth = 560

// paletteRows is how many commands show before the list scrolls.
const paletteRows = 9

type cmdPalette struct {
	u     *ui
	l     *taskList
	input *paletteEntry
	all   []command
	shown []command
	cur   int
	rows  *fyne.Container
	list  *container.Scroll
}

func (u *ui) openPalette(l *taskList) {
	p := &cmdPalette{u: u, l: l, all: u.commands(l), rows: container.NewVBox()}
	p.input = newPaletteEntry(u.hide, p.move, p.run)
	p.input.SetPlaceHolder("type a command…")
	p.input.OnChanged = func(string) { p.refresh() }
	p.list = container.NewVScroll(p.rows)
	width := canvas.NewRectangle(color.Transparent)
	width.SetMinSize(fyne.NewSize(paletteWidth, 0))
	body := container.NewVBox(width, p.input, vrule(), p.list)
	p.refresh()
	u.showAt(body, p.input, true) // closes whatever was up first, and forgets it
	u.pal = p
}

// refresh narrows the list to what the text matches, best first, and puts the cursor
// on the top row. No match is not nothing: the text becomes a line to capture.
func (p *cmdPalette) refresh() {
	needle := strings.TrimSpace(p.input.Text)
	type hit struct {
		c    command
		rank int
	}
	var hits []hit
	for _, c := range p.all {
		if r := score(needle, c); r >= 0 {
			hits = append(hits, hit{c, r})
		}
	}
	sort.SliceStable(hits, func(i, j int) bool { return hits[i].rank < hits[j].rank })
	p.shown = p.shown[:0:0]
	for _, h := range hits {
		p.shown = append(p.shown, h.c)
	}
	if len(p.shown) == 0 && needle != "" {
		p.shown = append(p.shown, p.capture(needle))
	}
	if p.cur >= len(p.shown) {
		p.cur = 0
	}
	p.draw(needle)
}

// capture is the palette's floor: words that name no command are a line for the layer you
// are looking at — the garage takes them as dump does, the tray as add does.
func (p *cmdPalette) capture(words string) command {
	if p.u.mode == modeHome && p.l.layer == core.LayerTray {
		return command{label: fmt.Sprintf("add “%s” to the tray", words), run: func() {
			t := core.New(words, nil)
			t.Layer = core.LayerTray
			p.u.flashNew = true
			p.u.save([]core.Task{t})
		}}
	}
	return command{label: fmt.Sprintf("dump “%s” into the garage", words), run: func() { p.u.dump(words) }}
}

// rank orders the matches: a label starting with the text, then one containing it, then
// the letters in order somewhere in it, then a hit among its other words. -1 is no match.
func score(needle string, c command) int {
	if needle == "" {
		return 0
	}
	n, label := strings.ToLower(needle), strings.ToLower(c.label)
	switch {
	case strings.HasPrefix(label, n):
		return 0
	case strings.Contains(label, n):
		return 1
	case fuzzy(n, label):
		return 2
	case fuzzy(n, c.words):
		return 3
	}
	return -1
}

func (p *cmdPalette) draw(needle string) {
	p.rows.Objects = nil
	for i, c := range p.shown {
		p.rows.Add(p.row(i, c, needle))
	}
	p.rows.Refresh()
	n := min(len(p.shown), paletteRows)
	if n == 0 {
		n = 1
	}
	p.list.SetMinSize(fyne.NewSize(paletteWidth, float32(n)*paletteRowHeight))
	p.list.Refresh()
	// The card follows the list: a popup keeps the size it opened at unless told.
	if pop := p.u.pop; pop != nil {
		pop.Resize(pop.Content.MinSize())
	}
}

const paletteRowHeight = 30

// row is one command: its label with the letters the text hit in the accent, and its key
// at the right in Fira Code. The cursor row sits on the soft accent.
func (p *cmdPalette) row(i int, c command, needle string) fyne.CanvasObject {
	// The runs sit flush: a box layout would put its spacing between them and break the word.
	label := container.New(&tightLayout{})
	for _, seg := range highlight(c.label, needle) {
		col := style.Ink
		if seg.hit {
			col = style.Accent
		}
		label.Add(text(seg.text, col))
	}
	var key fyne.CanvasObject = canvas.NewRectangle(color.Transparent)
	if c.key != "" {
		k := mono(c.key, style.Subtle)
		k.TextSize = 12
		key = container.NewCenter(k)
	}
	bg := rounded(style.AccentSoft, 6)
	setShown(bg, i == p.cur)
	line := container.NewBorder(nil, nil, container.NewCenter(label), key)
	return newTap(container.NewStack(bg, inset(fixed(line, paletteWidth-20, paletteRowHeight-6), 8, 3)), func() {
		p.cur = i
		p.run()
	})
}

type segment struct {
	text string
	hit  bool
}

// tightLayout lays texts left to right with nothing between them.
type tightLayout struct{}

func (tightLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	var s fyne.Size
	for _, o := range objs {
		m := o.MinSize()
		s.Width += m.Width
		s.Height = max(s.Height, m.Height)
	}
	return s
}

func (tightLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	x := float32(0)
	for _, o := range objs {
		m := o.MinSize()
		o.Resize(m)
		o.Move(fyne.NewPos(x, (size.Height-m.Height)/2))
		x += m.Width
	}
}

// highlight splits a label into the runs the needle's letters landed on and the rest,
// matching the way fuzzy does: each letter of the text, in order, at its first chance.
func highlight(label, needle string) []segment {
	n := []rune(strings.ToLower(needle))
	var out []segment
	i := 0
	for _, r := range label {
		hit := i < len(n) && strings.ToLower(string(r)) == string(n[i])
		if hit {
			i++
		}
		if len(out) > 0 && out[len(out)-1].hit == hit {
			out[len(out)-1].text += string(r)
			continue
		}
		out = append(out, segment{string(r), hit})
	}
	if i < len(n) { // the letters were not all in the label: a hit among the words, no highlight
		return []segment{{label, false}}
	}
	return out
}

func (p *cmdPalette) move(by int) {
	next := p.cur + by
	if next < 0 || next >= len(p.shown) {
		return
	}
	p.cur = next
	p.draw(strings.TrimSpace(p.input.Text))
}

// run closes the palette first, so what the command opens is not under it.
func (p *cmdPalette) run() {
	if p.cur < 0 || p.cur >= len(p.shown) {
		return
	}
	c := p.shown[p.cur]
	p.u.hide()
	c.run()
}

// paletteEntry is the search field: ↑↓ and ctrl+n/p move the cursor, Enter runs, Escape
// leaves — the keys a palette is expected to have, on a field that would otherwise keep
// them for a caret with nowhere to go.
type paletteEntry struct {
	escEntry
	onMove func(int)
	onRun  func()
}

func newPaletteEntry(onEscape func(), onMove func(int), onRun func()) *paletteEntry {
	e := &paletteEntry{onMove: onMove, onRun: onRun}
	e.onEscape = onEscape
	e.ExtendBaseWidget(e)
	return e
}

func (e *paletteEntry) TypedKey(k *fyne.KeyEvent) {
	switch k.Name {
	case fyne.KeyDown:
		e.onMove(1)
	case fyne.KeyUp:
		e.onMove(-1)
	case fyne.KeyReturn, fyne.KeyEnter:
		e.onRun()
	default:
		e.escEntry.TypedKey(k)
	}
}

func (e *paletteEntry) TypedShortcut(s fyne.Shortcut) {
	if cs, ok := s.(*desktop.CustomShortcut); ok && cs.Modifier == fyne.KeyModifierControl {
		switch cs.KeyName {
		case fyne.KeyN:
			e.onMove(1)
			return
		case fyne.KeyP:
			e.onMove(-1)
			return
		}
	}
	e.Entry.TypedShortcut(s)
}

// paletteShortcut is ctrl+shift+p, the key every editor opens its palette with.
func paletteShortcut(s fyne.Shortcut) bool {
	cs, ok := s.(*desktop.CustomShortcut)
	return ok && cs.KeyName == fyne.KeyP && cs.Modifier == fyne.KeyModifierControl|fyne.KeyModifierShift
}

var _ = store.Today // the palette reads dates through the lists; the import keeps the layer honest
