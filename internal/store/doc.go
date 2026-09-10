package store

import (
	"sort"
	"strconv"
	"strings"

	"github.com/cheese-cracker/tray/internal/core"
)

// Doc is a markdown file as tasks plus prose. Edits address lines by index, so
// anything unrecognised — headings, paragraphs, half-typed bullets — is untouched.
type Doc struct {
	Path     string
	Header   string
	Checkbox bool
	Lines    []string

	// Edits are keyed by the line a task was parsed at and applied by Save, never
	// to Lines in place. A task spans its bullet and its note, so a Set that grows a
	// note would shift every index below it — while a batch loop still holds the old
	// ones. Deferring keeps every index valid until the moment they are all spent.
	removed  map[int]bool
	replaced map[int][]string
}

func open(path, header string, checkbox bool) (*Doc, error) {
	if err := Ensure(path, header); err != nil {
		return nil, err
	}
	lines, err := Read(path)
	if err != nil {
		return nil, err
	}
	return &Doc{Path: path, Header: header, Checkbox: checkbox, Lines: lines,
		removed: map[int]bool{}, replaced: map[int][]string{}}, nil
}

func Tray() (*Doc, error) {
	return open(TrayPath(), TrayHeader, true)
}

func Garage(month string) (*Doc, error) {
	if month == "" {
		month = ThisMonth()
	}
	header := MonthHeader(month)
	if month == Someday {
		header = "# " + Someday
	}
	return open(MonthPath(month), header, false)
}

func (d *Doc) Tasks() []core.Task { return core.Tasks(d.Lines) }

func (d *Doc) Live() []core.Task {
	var out []core.Task
	for _, t := range d.Tasks() {
		if t.Live() {
			out = append(out, t)
		}
	}
	return out
}

// Texts is what dedupe compares against, so a repeated move is a no-op.
func (d *Doc) Texts() map[string]bool {
	seen := map[string]bool{}
	for _, t := range d.Tasks() {
		if t.Text != "" {
			seen[t.Text] = true
		}
	}
	return seen
}

// Revive clears the arrow on a departed line with this text, so a task handed back
// comes home instead of appearing twice — or, worse, nowhere.
// Reclaim puts a task back on the line it departed from, in whatever state it is
// in now. Revive restores what the garage line used to say, which loses everything
// the tray added — a finished task comes home open, and every carryover after that
// carries completed work forward again.
func (d *Doc) Reclaim(t core.Task) bool {
	for _, was := range d.Tasks() {
		if was.Text == t.Text && was.Moved != "" && !was.Terminal() {
			t.Index, t.Moved = was.Index, ""
			delete(t.Attrs, "from") // the inverse of take: it lives here again
			d.Set(t)
			return true
		}
	}
	return false
}

func (d *Doc) Revive(text string) bool {
	for _, t := range d.Tasks() {
		if t.Text == text && t.Moved != "" && !t.Terminal() {
			t.Moved = ""
			d.Set(t)
			return true
		}
	}
	return false
}

// LiveTexts is what dedupe should compare against: a departed line is history, not
// an occupant.
func (d *Doc) LiveTexts() map[string]bool {
	seen := map[string]bool{}
	for _, t := range d.Live() {
		if t.Text != "" {
			seen[t.Text] = true
		}
	}
	return seen
}

func (d *Doc) Set(t core.Task) {
	if t.Index >= 0 && t.Index < len(d.Lines) {
		d.replaced[t.Index] = core.Lines(t, d.Checkbox)
	}
}

// Add appends, which shifts nothing, so it can go straight into Lines.
func (d *Doc) Add(t core.Task) {
	d.Lines = append(d.Lines, core.Lines(t, d.Checkbox)...)
}

func (d *Doc) Remove(t core.Task) {
	for i := t.Index; i < t.Index+core.SpanAt(d.Lines, t.Index); i++ {
		d.removed[i] = true
	}
}

func (d *Doc) Save() error {
	kept := make([]string, 0, len(d.Lines))
	for i := 0; i < len(d.Lines); {
		if fresh, ok := d.replaced[i]; ok {
			kept = append(kept, fresh...)
			i += core.SpanAt(d.Lines, i) // the old bullet and its old note, together
			continue
		}
		if !d.removed[i] {
			kept = append(kept, d.Lines[i])
		}
		i++
	}
	if err := Write(d.Path, kept); err != nil {
		return err
	}
	d.Lines, d.removed, d.replaced = kept, map[int]bool{}, map[int][]string{}
	return nil
}

// Resolve turns id specs — 3, or 2,5-7 — into tasks, indexing the report just read.
func Resolve(items []core.Task, spec string) []core.Task {
	if spec == "" {
		return nil
	}
	var wanted []int
	for _, part := range strings.Split(spec, ",") {
		if lo, hi, ok := rangeOf(part); ok {
			for n := lo; n <= hi; n++ {
				wanted = append(wanted, n)
			}
			continue
		}
		if n, err := strconv.Atoi(part); err == nil {
			wanted = append(wanted, n)
		}
	}
	sort.Ints(wanted)

	var out []core.Task
	for _, n := range wanted {
		if n >= 1 && n <= len(items) {
			out = append(out, items[n-1])
		}
	}
	return out
}

func rangeOf(part string) (int, int, bool) {
	if part == "" {
		return 0, 0, false
	}
	dash := strings.Index(part[1:], "-")
	if dash < 0 {
		return 0, 0, false
	}
	lo, err := strconv.Atoi(part[:dash+1])
	if err != nil {
		return 0, 0, false
	}
	hi, err := strconv.Atoi(part[dash+2:])
	if err != nil {
		return 0, 0, false
	}
	return lo, hi, true
}
