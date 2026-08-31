# Roadmap

Rough order, not commitments. Every choice already made — including the reversed ones —
is one line in [DECISIONS.md](DECISIONS.md).

## Legend

- **Next**, **Later** — ordered by intent. Neither is a promise.
- **Parked** — not vetted, not planned. Written down so each one stops being re-thought from
  scratch, and so whatever it costs sits next to it rather than being rediscovered.
- **Shipped** — kept collapsed, so a thing that was once a question does not read as open.
- `[plugin]` — third-party, and none of it is reachable until a plugin surface exists.
  15 is one static binary with zero runtime dependencies, so an integration is either
  compiled in — which makes it not third-party — or a separate executable found on `PATH`
  and driven through the CLI, the way git finds `git-foo`. **That fork is the first
  question, and it is upstream of every one of them.**
- `[ours]` — the same surface question, but nothing here is third-party. "As a plugin" is a
  guess about packaging rather than a requirement.
- `[personal]` — whether it belongs in a general tool at all is part of the question.
- `[shape]` — a decision, not a feature. None of them is made.

## Next

- [ ] **The rewrite form drops every tag but the first** — `core` has held `Tags []string`
  all along and urgency already damps by count, so the grammar was never the limit. The form
  is what flattens it: it reads `first.Tags[0]` (`internal/ui/form.go:64`) and writes
  `[]string{tag}` back (`:223`, `:253`), so retaking a two-tag task silently loses one.
  Correctness rather than a feature, and the same hand-rolled form as the item below (60).
- [ ] **`bubbles/textinput` in the rewrite form** — hand-rolled text editing is the thin ice:
  no cursor and no word motions. Blocked on `←`/`→` in the `due` field shifting by a day,
  which a text input would claim for cursor movement. Needs a decision before it needs code.
- [ ] **A visible cursor in the text fields** — `title` and `tag` render as a bare string, so
  nothing marks where the next rune lands or which field has focus. **A caret pinned to the
  end is unblocked**: the model has no insertion index at all — `typed` appends
  (`internal/ui/form.go:154`), `backspace` lops the last rune (`:185`) — so end-of-string is
  the only place one could honestly sit, and drawing it there binds no keys. A caret you can
  *move* is the item above; moving it means `←`/`→`, already promised to the priority radio
  and the `due` day-shift (`:107`).
- [ ] **`u` undo, one level** — needs a snapshot in `store`, so not free. It was comfort
  rather than safety while nothing could leave the file; `E` erase changed that. The status
  line naming what was erased is the stand-in until this exists — enough to retype a line,
  not enough to undo one. Everything else stays recoverable by hand, because copy-forward
  leaves the source line where it was.
- [ ] **`tray install`** — self-installing subcommand in Go (`charmbracelet/huh` for the
  prompts, `--yes` for headless): PATH check, binary placement, shell init block. Lands
  behind a runtime feature flag (`internal/feature`, env + config), which is why that package
  doesn't exist yet — nothing to gate until this does.
  - Also the only thing that would make a turn-of-the-month reminder work. `--nag` was
    deleted because nothing ever ran it; `tray status` carries the warning until something
    can put it in a shell profile.

## Later

- [ ] **Prebuilt binaries** — goreleaser. `go install` is the only path today, so a Go
  toolchain is a hard requirement for anyone who wants this.
- [ ] **CI** — `make check` is the whole suite; no workflow is wired up yet.
- [ ] **Journal seeding** (`- [ ]` scrape) — only if the recurring-item problem comes back.
- [ ] **`tray dump` asking for the month on a TTY** — the pre-Go version had a picker. `a` in
  a garage tab covers it, so this is re-addable rather than missing.

## Parked

- [ ] `[plugin]` **Google Calendar** — the one that is actually wanted. Vetted as a want, not
  as a design.
- [ ] `[plugin]` **`task export | tray import`** — the missing leg. Export already ships the
  other way (`tray export | task import`, 4), and the field names were kept aligned for this.
- [ ] `[plugin]` **todo.txt export** — one more shape of the one-line-per-task grammar tray
  already writes.
- [ ] `[plugin]` **Notion**, **Linear** — hosted stores, so both keep a server-side id. Ids
  are the thing 8 says tray does not hold.
- [ ] `[plugin]` **Dictation into the garage** (ostt) — the garage is the layer with no
  schema, so it is where a transcript can land without having to claim it is a task yet.
- [ ] `[ours]` **Eisenhower view** — half of it exists. `core.Quadrant`
  (`internal/core/urgency.go:103`) is written and tested, `tray export` already emits it
  (`internal/cli/report.go:181`), and the README describes the reading. What is missing is
  somewhere to look at it.
- [ ] `[ours]` **TagManager — priorities on tags** — urgency counts tags and damps the count
  (one 0.8, two 0.9, three or more 1.0); it never weighs *which*. Weighing them needs a tag
  registry, which is the thing 18 exists to avoid.
- [ ] `[personal]` **Journal integration** — `tray print` already emits `- [ ]` bullets for
  pasting; the return leg, scraping them back out, is the Later item above and unbuilt. A
  script of yours outside this repo keeps personal shape out of a general tool.
- [ ] `[shape]` **A config file, so the row format is a choice** — todo.txt or the default,
  which today is Taskwarrior's field names on markdown lines (2, 4, 17). Two things sit on
  it. There is deliberately no config file at all — 18 keeps the tag vocabulary in the files
  rather than in a registry, and a config is the first crack in that. And todo.txt is a
  different grammar, not a flag: `x ` for done, `(A)` for priority, `@context` alongside
  `+project`, dates in positional slots. So `core.Line` and the parser both fork, and a
  directory holding rows in both shapes cannot be read back unambiguously — which is 17's
  whole job.
- [ ] `[shape]` **A store that isn't markdown** — Taskwarrior driven through its own CLI,
  say. 2 keeps applying to the markdown store, which stays the default; a second backend is
  something you opt into knowing what it costs. It costs `store` as an interface, an encoding
  for the garage, a parser that wants the opposite of 17, latency measured before anything
  else, and a second FLOWS suite — and it gives up `find` as a rot detector and month files
  as an immutable record (5, 6), both properties of having files per month rather than of the
  data.
- [ ] `[shape]` **`project`, `description` and other detail fields** — would this even match
  the ethos? `project` already has a row in Rejected and 9 is the live one. Reopening either
  is allowed: a rejected option is kept written down precisely because it can be argued
  again. `description` has never been ruled on.
- [ ] `[shape]` **`triggerAt`, so a task can act like a reminder** — a date to surface on,
  distinct from the date it is owed. Nothing in tray runs on its own though, and `--nag` was
  deleted for exactly that (76), so this wants `tray install` before it wants a field.
- [ ] `[shape]` **Nested task sets** — would this even match the ethos?

## Shipped

- [x] **`~/tray` as the default directory** — `.config` was never it: XDG splits config from
  data and these are data. Visible-in-home held, because the convention splits on who owns
  the bytes — Taskwarrior hides `~/.task/` since you are not meant to open a sqlite file,
  while org-mode, Obsidian and todo.txt stay visible because you are. The name was the live
  part: the directory holds `tray.md` too, so `task-garage` announced one layer, not the
  pair. Decisions 53, 53a. **No migration path** — an existing `~/task-garage` has to be
  moved by hand.
- [x] **Markdown checkboxes in the tray rows** — `[ ]` and `[x]`, the box the tray file
  already writes. Garage rows keep a plain mark; its file has no checkbox. Decisions 89–89e,
  including the ballot glyphs that were tried and reverted.
- [x] **The dropped state is gone, and `E` erases for real** — `dropped:` was a second way to
  say finished that no report ever told apart, and it left a line you could neither act on nor
  remove. One terminal state now, and one verb that actually removes a line — reachable in
  review mode alone. Decisions 90–94.
- [x] **`v` is a mode, not a filter** — it widens the list to everything on the layer and
  narrows the keymap to the two rare verbs. The split is by how often you reach for a verb:
  a monthly one stays out of the flow you drive daily. Decisions 92–94.

## Rejected

| | Why |
|---|---|
| `fzf` shell-out for `/` | A runtime dependency and an alt-screen seam, for a keystroke `bubbles/list` already does in-process |
| Hand-rolled fuzzy matcher | Scoring heuristics are a rabbit hole |
| `project` as a second axis | Tags are the only axis — one dimension, nothing to decide twice |
