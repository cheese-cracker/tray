package gui

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/cheese-cracker/tray/internal/plugin"
	"github.com/cheese-cracker/tray/internal/style"
	"github.com/cheese-cracker/tray/internal/sync"
)

// background runs work off the UI thread and hands what it made back on it. Tests swap
// it for a synchronous one, so a flow asserts without a scheduler in the way.
var background = func(work func(), then func()) {
	go func() {
		work()
		fyne.Do(then)
	}()
}

// syncNow is `s`: with plans already waiting it opens them; otherwise it runs the event
// and opens whatever the plugins reported. Recurrence and lifting land on the spot.
func (u *ui) syncNow() {
	if len(u.pending) > 0 {
		u.openPending()
		return
	}
	u.runSync(func() (sync.Summary, []sync.Result, error) {
		return sync.Sync(u.s, sync.Manual, "", syncTimeout)
	}, u.openSyncReview)
}

// launch is the launch hook: the event runs once the window is up, and what the plugins
// found waits in the status line rather than in your face (T4). Nothing here prompts.
func (u *ui) launch() {
	u.runSync(func() (sync.Summary, []sync.Result, error) {
		return sync.Sync(u.s, sync.Launch, "", syncTimeout)
	}, u.hold)
}

// runSync runs one event in the background and, back on the UI thread, reads the world
// again and hands the plugins' results to whoever asked. One at a time.
func (u *ui) runSync(run func() (sync.Summary, []sync.Result, error), then func([]sync.Result)) {
	if u.syncing {
		return
	}
	u.syncing = true
	u.status.SetText(u.statusText())
	var (
		sum     sync.Summary
		results []sync.Result
		err     error
	)
	background(func() { sum, results, err = run() }, func() {
		u.syncing = false
		u.syncedAt = time.Now().Format("15:04")
		if err != nil {
			u.fail(err)
			return
		}
		if sum.Materialized+sum.Lifted > 0 {
			u.flash = fmt.Sprintf("materialized %d · lifted %d", sum.Materialized, sum.Lifted)
		}
		u.reload()
		then(results)
	})
}

// hold keeps what the plugins reported for later and says so in one line you can click.
func (u *ui) hold(results []sync.Result) {
	u.pending = u.pending[:0]
	changes, failed := 0, 0
	for _, r := range results {
		switch {
		case r.Err != nil:
			failed++
		case !r.Diff.Empty() || len(r.Push) > 0:
			changes++
		default:
			continue
		}
		u.pending = append(u.pending, r)
	}
	if len(u.pending) == 0 {
		u.notice.Hide()
		return
	}
	var parts []string
	if changes == 1 {
		parts = append(parts, "1 plugin has changes")
	} else if changes > 1 {
		parts = append(parts, fmt.Sprintf("%d plugins have changes", changes))
	}
	if failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", failed))
	}
	u.notice.SetText(strings.Join(parts, " · ") + " — review")
	u.notice.Show()
	u.bottom.Refresh()
}

func (u *ui) openPending() {
	if len(u.pending) > 0 {
		u.openSyncReview(u.pending)
	}
}

// syncView is where outside data waits for you (T5): one card per plugin — what it
// would add and change, what it would push back, what it no longer sees, its evidence
// — and a button that lands the plan whole or leaves it.
type syncView struct {
	u     *ui
	cards []*syncCard
}

type syncCard struct {
	r     sync.Result
	box   fyne.CanvasObject
	state *widget.Label
	apply *widget.Button
	done  bool
}

func (u *ui) openSyncReview(results []sync.Result) {
	v := &syncView{u: u}
	box := container.NewVBox()
	for _, r := range results {
		c := v.card(r)
		v.cards = append(v.cards, c)
		box.Add(c.box)
	}
	if len(results) == 0 {
		box.Add(container.NewPadded(grey("No plugin keeps a garage — nothing to review.")))
	}
	p := newPage(container.NewBorder(
		banner("sync review", "y applies the next plan whole · esc leaves and lands nothing", style.AccentSoft, style.Accent),
		u.bottom, nil, nil, container.NewVScroll(container.NewPadded(box))))
	p.onKey = func(k *fyne.KeyEvent) {
		if k.Name == fyne.KeyEscape {
			u.leave()
		}
	}
	p.onRune = func(r rune) {
		switch r {
		case 'y':
			v.applyNext()
		case 'q':
			u.quit()
		}
	}
	u.view, u.page = v, p
	u.pending, u.notice.Text = nil, ""
	u.notice.Hide()
	u.enter(modeSyncReview, p)
}

// card is one plugin's plan on a card: a status dot and its message up top, then what it
// would add, change, push back and no longer sees — each as a section with its count —
// its evidence at the right, and the two buttons that decide it.
func (v *syncView) card(r sync.Result) *syncCard {
	c := &syncCard{r: r}
	lines := container.NewVBox()
	section := func(name string, n int, rows ...fyne.CanvasObject) {
		if n == 0 {
			return
		}
		head := caption(fmt.Sprintf("%s · %d", strings.ToUpper(name), n), style.Ink2)
		head.TextStyle.Bold = true
		lines.Add(container.NewVBox(append([]fyne.CanvasObject{head}, rows...)...))
	}
	state := style.Low
	switch {
	case r.NeedsYou():
		state = style.Review
		lines.Add(text("needs you — "+r.Message, style.Review))
	case r.Err != nil:
		state = style.High
		lines.Add(warn("failed — " + r.Message))
	default:
		var adds, ups, pushes, gone []fyne.CanvasObject
		for _, row := range r.Diff.Adds {
			line := container.NewHBox(plain("+ " + row.Text))
			for _, g := range row.Tags {
				line.Add(chip("#" + g))
			}
			adds = append(adds, line)
		}
		for _, up := range r.Diff.Updates {
			ups = append(ups, plain(fmt.Sprintf("%s: %s", up.Old.Text, changesOf(up))))
		}
		for _, p := range r.Push {
			pushes = append(pushes, plain(p.Key+"  "+setOf(p.Set)))
		}
		for _, g := range r.Diff.Gone {
			gone = append(gone, grey(g.Text+" — kept"))
		}
		section("adds", len(adds), adds...)
		section("updates", len(ups), ups...)
		section("pushes", len(pushes), pushes...)
		section("gone from source", len(gone), gone...)
	}
	c.state = widget.NewLabel("")
	c.state.SizeName = theme.SizeNameCaptionText
	c.apply = primary("Apply all", func() { v.apply(c) })
	discard := lowButton("Discard", func() { v.discard(c) })
	if r.Err != nil || (r.Diff.Empty() && len(r.Push) == 0) {
		c.done = true // nothing to land
		c.apply.Disable()
	}
	lines.Add(container.NewHBox(c.apply, discard, c.state))

	head := container.NewHBox(fixed(dot(8, rgba(state), color.Transparent, 0), 8, 8),
		container.NewCenter(semibold(r.Plugin, style.Ink)), container.NewCenter(caption(r.Message, style.Ink2)))
	body := container.NewVBox(head, lines)
	var content fyne.CanvasObject = body
	if img := evidence(r); img != nil {
		content = container.NewBorder(nil, nil, nil, img, body)
	}
	c.box = container.NewPadded(container.NewStack(rounded(style.Card, 8), container.NewPadded(content)))
	return c
}

// apply lands one plugin's plan: one transaction in the store, so an error means
// nothing landed, then the confirmed pushes go back to the plugin off the UI thread.
func (v *syncView) apply(c *syncCard) {
	if c.done {
		return
	}
	c.done = true
	c.apply.Disable()
	c.state.SetText("applying…")
	var (
		a   sync.Applied
		err error
	)
	background(func() { a, err = sync.Apply(v.u.s, c.r, c.r.Push, syncTimeout) }, func() {
		if err != nil {
			c.state.SetText("not applied — " + err.Error())
			c.done = false
			c.apply.Enable()
			return
		}
		c.state.SetText(fmt.Sprintf("applied %d adds · %d updates · pushed %d (%d failed)",
			a.Added, a.Updated, len(a.PushOK), len(a.PushFailed)))
		v.u.reload()
	})
}

func (v *syncView) discard(c *syncCard) {
	c.done = true
	c.apply.Disable()
	c.state.SetText("discarded")
}

// applyNext is `y`: the first plan still waiting lands.
func (v *syncView) applyNext() {
	for _, c := range v.cards {
		if !c.done {
			v.apply(c)
			return
		}
	}
}

// evidence is the newest thing the plugin saw, when the file it named is there.
func evidence(r sync.Result) fyne.CanvasObject {
	if r.Evidence == "" {
		return nil
	}
	p, ok := plugin.Find(r.Plugin)
	if !ok {
		return nil
	}
	path := filepath.Join(p.Dir, r.Evidence)
	if _, err := os.Stat(path); err != nil {
		return nil
	}
	img := canvas.NewImageFromFile(path)
	img.FillMode = canvas.ImageFillContain
	img.SetMinSize(fyne.NewSize(160, 120))
	return img
}

func changesOf(up sync.Update) string {
	var parts []string
	for _, f := range up.Fields {
		old, new := up.Old.Attr(f), ""
		switch f {
		case "text":
			old, new = up.Old.Text, up.New.Text
		case "done":
			old, new = orOpen(up.Old.Done), orOpen(*up.New.Done)
		case "tags":
			old, new = strings.Join(up.Old.Tags, " "), strings.Join(up.New.Tags, " ")
		case "priority":
			new = up.New.Priority
		case "due":
			new = up.New.Due
		}
		parts = append(parts, fmt.Sprintf("%s %s → %s", f, old, new))
	}
	return strings.Join(parts, ", ")
}

func orOpen(done string) string {
	if done == "" {
		return "open"
	}
	return "done " + done
}

func tagsOf(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return "  #" + strings.Join(tags, " #")
}

func setOf(set map[string]string) string {
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	for _, k := range keys {
		parts = append(parts, k+"="+set[k])
	}
	return strings.Join(parts, " ")
}
