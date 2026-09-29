package store

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/cheese-cracker/tray/internal/core"
)

// File is the store: one SQLite file in the home, and never anywhere else (T36). A save
// waits on this disk and nothing more; a copy of the rows on some server is a plugin's
// job, and it arrives through the reviewed plan like anything from outside (T37).
const File = "tray.db"

// Ids are four random base36 characters (core.NewID), checked against the table on the
// way in — an agent that remembers one across runs must never find a different task
// behind it. Insertion order is the rowid, which a text key keeps.
const schema = `
CREATE TABLE IF NOT EXISTS task (
  id         TEXT PRIMARY KEY,
  layer      TEXT NOT NULL CHECK (layer IN ('tray','garage')),
  month      TEXT,
  text       TEXT NOT NULL,
  priority   TEXT CHECK (priority IN ('H','M','L')),
  due        TEXT,
  wait       TEXT,
  recur      TEXT,
  until      TEXT,
  entry      TEXT NOT NULL,
  done       TEXT,
  from_month TEXT,
  tags       TEXT NOT NULL DEFAULT '',
  note       TEXT NOT NULL DEFAULT '',
  source     TEXT
);
CREATE INDEX IF NOT EXISTS task_layer ON task (layer, month, done);
CREATE INDEX IF NOT EXISTS task_source ON task (source);
CREATE TABLE IF NOT EXISTS plugin_run (
  name TEXT PRIMARY KEY, hook TEXT, at TEXT, ok INTEGER, message TEXT
);
CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL);
`

const columns = "id, layer, month, text, priority, due, wait, recur, until, entry, done, from_month, tags, note, source"

// querier is what *sql.DB and *sql.Tx share, so every method is written once and a
// transaction is just a Store bound to a Tx.
type querier interface {
	Exec(string, ...any) (sql.Result, error)
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}

type Store struct {
	db *sql.DB
	q  querier
}

// Open creates the home and opens its database, making the schema if this is the first
// time.
func Open(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(dir, File)
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1) // one process, one writer; WAL readers never wait on it
	for _, stmt := range []string{"PRAGMA journal_mode = WAL", "PRAGMA busy_timeout = 5000", schema, "PRAGMA user_version = 1"} {
		if _, err := db.Exec(stmt); err != nil {
			db.Close()
			return nil, fmt.Errorf("%s: %w", path, err)
		}
	}
	return &Store{db: db, q: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }

// Update runs fn in one transaction: a batch verb lands whole or not at all.
func (s *Store) Update(fn func(*Store) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(&Store{db: s.db, q: tx}); err != nil {
		tx.Rollback()
		return err
	}
	return tx.Commit()
}

// Filter narrows Tasks. The zero Filter is every live row in the working set.
type Filter struct {
	Layer        string
	Month        string
	All          bool // finished rows and templates too
	IDs          []string
	Tags         []string          // every one must be present
	Attrs        map[string]string // key:value on a wire name, case-insensitive
	Text         string            // case-insensitive substring of the words or a tag
	SourcePrefix string            // rows a plugin owns: `<name>:`
}

// Tasks is every row the filter admits, oldest first. Layer, month, state and ids are
// SQL; tags, attrs and text are matched in Go over what is left.
// ponytail: tags is a space-separated column and filters parse it here; a join table
// if that ever shows up on a profile.
func (s *Store) Tasks(f Filter) ([]core.Task, error) {
	where, args := []string{"1 = 1"}, []any{}
	if f.Layer != "" {
		where, args = append(where, "layer = ?"), append(args, f.Layer)
	}
	if f.Month != "" {
		where, args = append(where, "month = ?"), append(args, f.Month)
	}
	if !f.All {
		where = append(where, "done IS NULL AND recur IS NULL")
	}
	if len(f.IDs) > 0 {
		marks := make([]string, len(f.IDs))
		for i, id := range f.IDs {
			marks[i], args = "?", append(args, id)
		}
		where = append(where, "id IN ("+strings.Join(marks, ",")+")")
	}
	if f.SourcePrefix != "" {
		escaped := strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(f.SourcePrefix)
		where, args = append(where, `source LIKE ? ESCAPE '\'`), append(args, escaped+"%")
	}
	rows, err := s.q.Query("SELECT "+columns+" FROM task WHERE "+strings.Join(where, " AND ")+" ORDER BY rowid", args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []core.Task
	for rows.Next() {
		t, err := scan(rows)
		if err != nil {
			return nil, err
		}
		if matches(t, f) {
			out = append(out, t)
		}
	}
	return out, rows.Err()
}

func matches(t core.Task, f Filter) bool {
	for _, g := range f.Tags {
		if !has(t.Tags, g) {
			return false
		}
	}
	for key, val := range f.Attrs {
		if !strings.EqualFold(t.Attr(key), val) {
			return false
		}
	}
	if f.Text != "" {
		needle := strings.ToLower(f.Text)
		if !strings.Contains(strings.ToLower(t.Text), needle) &&
			!strings.Contains(strings.ToLower(strings.Join(t.Tags, " ")), needle) {
			return false
		}
	}
	return true
}

func has(tags []string, g string) bool {
	for _, have := range tags {
		if have == g {
			return true
		}
	}
	return false
}

func (s *Store) Get(id string) (core.Task, bool, error) {
	rows, err := s.q.Query("SELECT "+columns+" FROM task WHERE id = ?", id)
	if err != nil {
		return core.Task{}, false, err
	}
	defer rows.Close()
	if !rows.Next() {
		return core.Task{}, false, rows.Err()
	}
	t, err := scan(rows)
	return t, err == nil, err
}

// Put inserts a task with no id and gives it one; otherwise it rewrites every column.
func (s *Store) Put(t *core.Task) error {
	if t.Entry == "" {
		t.Entry = Today().Format(core.DateLayout)
	}
	vals := []any{
		t.Layer, null(t.Month), t.Text, null(t.Priority), null(t.Due), null(t.Wait), null(t.Recur),
		null(t.Until), t.Entry, null(t.Done), null(t.FromMonth), strings.Join(t.Tags, " "), t.Note, null(t.Source),
	}
	if t.ID == "" {
		id, err := s.fresh()
		if err != nil {
			return err
		}
		_, err = s.q.Exec(`INSERT INTO task (id, layer, month, text, priority, due, wait, recur, until, entry, done, from_month, tags, note, source)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`, append([]any{id}, vals...)...)
		if err != nil {
			return err
		}
		t.ID = id
		return nil
	}
	_, err := s.q.Exec(`UPDATE task SET layer = ?, month = ?, text = ?, priority = ?, due = ?, wait = ?, recur = ?,
		until = ?, entry = ?, done = ?, from_month = ?, tags = ?, note = ?, source = ? WHERE id = ?`, append(vals, t.ID)...)
	return err
}

// fresh is an id no row holds. A collision is one in a million and costs one more roll.
func (s *Store) fresh() (string, error) {
	for {
		id := core.NewID()
		var n int
		if err := s.q.QueryRow("SELECT count(*) FROM task WHERE id = ?", id).Scan(&n); err != nil {
			return "", err
		}
		if n == 0 {
			return id, nil
		}
	}
}

func (s *Store) Delete(id string) error {
	_, err := s.q.Exec("DELETE FROM task WHERE id = ?", id)
	return err
}

// Months is every calendar month a garage row sits in, oldest first. Someday and the
// plugins' garages are not months.
func (s *Store) Months() ([]string, error) {
	rows, err := s.q.Query("SELECT DISTINCT month FROM task WHERE layer = ? AND month IS NOT NULL", core.LayerGarage)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		if IsMonth(m) {
			out = append(out, m)
		}
	}
	sort.Strings(out)
	return out, rows.Err()
}

// A Run is the last thing a plugin did: one row per plugin, because the plugins pane
// and `status` ask "how did it go last time", never for a history.
type Run struct {
	Name, Event, At string
	OK              bool
	Message         string
}

func (s *Store) RecordRun(r Run) error {
	_, err := s.q.Exec(`INSERT OR REPLACE INTO plugin_run (name, hook, at, ok, message) VALUES (?, ?, ?, ?, ?)`,
		r.Name, r.Event, r.At, r.OK, r.Message)
	return err
}

// Runs is every plugin's last run, by name.
func (s *Store) Runs() (map[string]Run, error) {
	rows, err := s.q.Query("SELECT name, hook, at, ok, message FROM plugin_run")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]Run{}
	for rows.Next() {
		var r Run
		if err := rows.Scan(&r.Name, &r.Event, &r.At, &r.OK, &r.Message); err != nil {
			return nil, err
		}
		out[r.Name] = r
	}
	return out, rows.Err()
}

// Meta is one remembered fact — the mirror's last write — or "" when none.
func (s *Store) Meta(key string) (string, error) {
	var v string
	err := s.q.QueryRow("SELECT value FROM meta WHERE key = ?", key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func (s *Store) SetMeta(key, value string) error {
	_, err := s.q.Exec("INSERT OR REPLACE INTO meta (key, value) VALUES (?, ?)", key, value)
	return err
}

func scan(rows *sql.Rows) (core.Task, error) {
	var t core.Task
	var month, priority, due, wait, recur, until, done, from, source sql.NullString
	var tags string
	err := rows.Scan(&t.ID, &t.Layer, &month, &t.Text, &priority, &due, &wait, &recur, &until,
		&t.Entry, &done, &from, &tags, &t.Note, &source)
	if err != nil {
		return t, err
	}
	t.Month, t.Priority, t.Due, t.Wait, t.Recur = month.String, priority.String, due.String, wait.String, recur.String
	t.Until, t.Done, t.FromMonth, t.Source = until.String, done.String, from.String, source.String
	t.Tags = strings.Fields(tags)
	return t, nil
}

// null keeps an unset field NULL rather than "", so the CHECKs and indexes see one
// shape of absence.
func null(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// ParseIDs turns an id list — `k79l`, or `k79l,79ya` — into ids. Anything that is not
// shaped like one is ignored, not an error.
func ParseIDs(spec string) []string {
	var out []string
	for _, part := range strings.Split(spec, ",") {
		if core.IsID(part) {
			out = append(out, part)
		}
	}
	return out
}
