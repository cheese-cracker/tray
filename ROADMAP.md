# Roadmap

Rough order, not commitments. Every choice already made — including the reversed ones —
is one line in [DECISIONS.md](DECISIONS.md).

## Next

- [ ] **The rewrite form drops every tag but the first** — `core` has held `Tags []string`
  all along and urgency already damps by count, so the grammar was never the limit. The
  form is what flattens it: it reads `first.Tags[0]` (`internal/ui/form.go:64`) and writes
  `[]string{tag}` back (`:223`, `:253`), so retaking a two-tag task silently loses one.
  Correctness rather than a feature — and the same hand-rolled form as the `textinput`
  item below (60).

- [ ] **`tray install`** — the nag has no home until this exists. See below.

- [ ] **`bubbles/textinput` in the rewrite form** — hand-rolled text editing is the thin ice:
  no cursor and no word motions. The blocker is that `←`/`→` on the `due` field shift by a
  day, which a text input would claim for cursor movement. Needs a decision before it needs
  code.

- [ ] **A visible cursor in the text fields** — `title` and `tag` render as a bare string, so
  while you type nothing marks where the next rune lands or which field has the focus.
  **A caret pinned to the end of the string is unblocked** and is the cheap half: the model
  has no insertion index at all — `typed` appends (`internal/ui/form.go:154`) and `backspace`
  lops the last rune (`:185`) — so end-of-string is the only place a caret could honestly sit,
  and drawing it there binds no keys. A caret you can *move* is the other half, and that is
  the item above: moving it means `←`/`→`, which `cycle` has already promised to the priority
  radio and the `due` day-shift (`:107`).

- [ ] **`u` undo, one level** — needs a snapshot in `store`; not free. Copy-forward keeps
  everything hand-recoverable meanwhile, so this is comfort rather than safety.
- [ ] **`tray install`** — self-installing subcommand in Go (`charmbracelet/huh` for the
  prompts, `--yes` for headless): PATH check, binary placement, shell init block. Lands
  behind a runtime feature flag (`internal/feature`, env + config), which is why that
  package doesn't exist yet — nothing to gate until this does.
  - it is also the only thing that would make a turn-of-the-month reminder work.
    `--nag` was deleted because nothing ever ran it; `tray status` carries the warning
    until something can put it in a shell profile.

## Later

- [ ] **Prebuilt binaries** — goreleaser. `go install` is the only path today, so a Go
  toolchain is a hard requirement for anyone who wants this.
- [ ] **CI** — `make check` is the whole suite; no workflow is wired up yet.
- [ ] **Journal seeding** (`- [ ]` scrape) — only if the recurring-item problem comes back.
- [ ] **`tray dump` asking for the month on a TTY** — the pre-Go version had a picker.
  `a` in a garage tab covers it, so this is re-addable rather than missing.

## Parked

Not vetted and not planned. Written down so each one stops being re-thought from scratch,
and so whatever it costs sits next to it rather than being rediscovered.

### Plugins

Nothing below is reachable until there is a plugin surface, and there is none. Decision 15
is one static binary with zero runtime dependencies, so a third-party integration is either
compiled in — which makes it not third-party — or a separate executable found on `PATH` and
driven through the CLI, the way git finds `git-foo`. **That fork is the first question, and
it is upstream of every integration here.**

- [ ] **Google Calendar** — the one that is actually wanted. Vetted as a want, not as a design.
- [ ] **`task export | tray import`** — the missing leg. Export already ships the other way
  (`tray export | task import`, 4), and the field names were kept aligned for exactly this.
- [ ] **todo.txt export** — one more shape of the one-line-per-task grammar tray already writes.
- [ ] **Notion**, **Linear** — hosted stores, so both keep a server-side id. Ids are the thing
  8 says tray does not hold.
- [ ] **Dictation into the garage** (ostt) — the garage is the layer with no schema, so it is
  where a transcript can land without having to claim it is a task yet.

### Features as plugins

The same surface question, but nothing here is third-party — these are ours, and "as a plugin"
is a guess about packaging rather than a requirement.

- [ ] **Eisenhower view** — half of it exists. `core.Quadrant`
  (`internal/core/urgency.go:103`) is written and tested, `tray export` already emits it
  (`internal/cli/report.go:181`), and the README describes the reading. What is missing is
  somewhere to look at it.
- [ ] **TagManager — priorities on tags** — urgency counts tags and damps the count
  (one 0.8, two 0.9, three or more 1.0); it never weighs *which*. Weighing them needs a tag
  registry, which is the thing 18 exists to avoid.

### Personal

- [ ] **Journal integration** — `tray print` already emits `- [ ]` bullets for pasting; the
  return leg, scraping `- [ ]` back out of a journal, is the Later item above and unbuilt.
  Whether it belongs in this repo at all is the open part: a script of yours outside it keeps
  personal shape out of a general tool.

### Shape questions

Not features. Each is a decision, and none of them is made.

- [ ] **A different task format?** — today it is Taskwarrior's field names on markdown lines
  (2, 4, 17).
- [ ] **`project`, `description` and other detail fields** — would this even match the ethos?
  `project` already has a row in Rejected below, and DECISIONS 9 is the live one. Reopening
  either is allowed — a rejected option is kept written down precisely because it can be
  argued again. `description` has never been ruled on.
- [ ] **`triggerAt`, so a task can act like a reminder** — a date to surface on, distinct from
  the date it is owed. Nothing in tray runs on its own, though; `--nag` was deleted for exactly
  that (76), so this wants `tray install` before it wants a field.
- [ ] **Nested task sets** — would this even match the ethos?
- [ ] **Actually deleting a task, and whether `u` undo is what makes that safe** — undecided,
  and the two halves are one question. Today `D` strikes through: 5 says nothing is ever
  deleted, and 27 is the one row that flags its own rule as overridable. Undo sits in Next
  described as comfort rather than safety — which is true exactly as long as no line can
  leave the file, and stops being true the moment one can.

## Rejected

| | Why |
|---|---|
| `fzf` shell-out for `/` | A runtime dependency and an alt-screen seam, for a keystroke `bubbles/list` already does in-process |
| Hand-rolled fuzzy matcher | Scoring heuristics are a rabbit hole |
| Taskwarrior as the store | TW 3.x is `taskchampion.sqlite3`; a binary store can't be hand-edited |
| `project` as a second axis | Tags are the only axis — one dimension, nothing to decide twice |
