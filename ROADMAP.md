# Roadmap

Rough order, not commitments. **Next** is vetted. **Parked** is everything else — written
down so it stops being re-thought from scratch.

One line per item. Shipped items are deleted rather than archived: every choice already
made, including the reversed ones, is one line in [DECISIONS.md](DECISIONS.md).

Tags: `[plugin]` third-party · `[shell]` a snippet calling the CLI · `[ours]` ours to
build · `[personal]` may not belong in a general tool · `[shape]` a decision, not a feature.

## Next

- [ ] **A key for tags in the garage** — `+`, no form. `tray dump +infra` already writes
  tags there (88c), so this is the interface catching up with the grammar, not new structure.
- [ ] **The rewrite form drops every tag but the first** — reads `first.Tags[0]`
  (`internal/ui/form.go:64`) and writes one back (`:223`, `:253`). Correctness, not a feature.
- [ ] **`bubbles/textinput` in the rewrite form** — hand-rolled editing has no cursor and no
  word motions. Blocked on `←`/`→` in `due`, which shifts by a day and a text input would claim.
- [ ] **A visible cursor in the text fields** — a caret pinned to the end is unblocked and
  binds no keys; a movable one is the item above.
- [ ] **`u` undo, one level** — needs a snapshot in `store`. `E` erase is the only action that
  leaves nothing to recover by hand.
- [ ] **`tray init` prompts on a TTY** — offers to write the `tray head` line into your shell
  profile, `--yes` for headless. Same shape as 72: a picker on a terminal, an error when piped.
  Absorbs what `tray install` was for; packaging is goreleaser's job, not ours.
- [ ] **Prebuilt binaries** — goreleaser. `go install` is the only path today, so a Go
  toolchain is a hard requirement for anyone who wants this.
- [ ] **CI** — `make check` is the whole suite and no workflow runs it. Also what a required
  status check on `main` would need.
- [ ] **A second demo take** — the recording never shows `/` or review mode. Kit is in
  `~/tray-demo/`; the gap is not described in the README.

## Parked

- [ ] `[plugin]` **Google Calendar** — the one actually wanted, the one least settled. Pull
  *and* push, which is what a garage is, but a calendar is a grid of times, not a dump of lines.
- [ ] `[plugin]` **`task export | tray import`** — the missing leg; field names were kept
  aligned for it (4).
- [ ] `[plugin]` **todo.txt export** — one more shape of the grammar tray already writes.
- [ ] `[plugin]` **Notion, Linear, Jira** — hosted stores keep a server-side id, which 8 says
  tray does not hold. Pull-only as a 3P garage is the vetted first step. The cron'd agent
  below is the cheaper answer to the same question, and does not need a plugin surface.
- [ ] `[shell]` **A cron'd agent harness syncing the hosted stores** — Claude Code, Codex or
  opencode on a schedule, reading Notion/Linear/Jira through their own connectors and calling
  `tray dump`. Nothing to build but a prompt and a crontab: 19 already makes the CLI the agent
  surface, so this sidesteps the plugin-surface fork that blocks every `[plugin]` item.
- [ ] `[shell]` **Dictation into the garage** (ostt) — the garage has no schema, so a transcript
  can land without claiming to be a task yet. Needs only a way to call `tray dump`.
- [ ] `[ours]` **Eisenhower view** — `core.Quadrant` is written and `tray export` emits it;
  what is missing is somewhere to look at it.
- [ ] `[ours]` **Priorities on tags** — urgency counts tags and never weighs which. Weighing
  needs a tag registry, the thing 18 exists to avoid.
- [ ] `[personal]` **Journal integration** — `tray print` emits the bullets; scraping them back
  is unbuilt, and a script outside this repo keeps personal shape out of a general tool.
- [ ] **Journal seeding** (`- [ ]` scrape) — only if the recurring-item problem comes back.
- [ ] **`tray dump` asking for the month on a TTY** — `a` in a garage tab covers it, so
  re-addable rather than missing.
- [ ] `[shape]` **A config file, so the row format is a choice** — 18 keeps the tag vocabulary
  in the files rather than a registry, and a config is the first crack in that. todo.txt is a
  different grammar, not a flag.
- [ ] `[shape]` **A store that isn't markdown** — costs `store` as an interface, a second FLOWS
  suite, and gives up `find` as a rot detector and month files as a record (5, 6).
- [ ] `[shape]` **`project`, `description` and other detail fields** — `project` was already
  ruled out (9); `description` never has been. Reopening a settled one is allowed.
- [ ] `[shape]` **`triggerAt`, so a task can act like a reminder** — nothing in tray runs on its
  own, so this wants a shell profile line before it wants a field (76).
- [ ] `[shape]` **Nested task sets** — would this even match the ethos?
