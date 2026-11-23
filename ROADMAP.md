# Roadmap

Rough order, not commitments. **Next** is vetted. **Parked** is everything else — written
down so it stops being re-thought from scratch.

One line per item. Shipped items are deleted rather than archived: every choice already
made, including the reversed ones, is one line in [DECISIONS.md](DECISIONS.md).

Tags: `[plugin]` on the contract in `plugins/README.md` · `[shell]` a snippet calling the
CLI · `[ours]` ours to build · `[personal]` may not belong in a general tool · `[shape]` a
decision, not a feature.

Ids in brackets are tray task ids. Those tasks are marked done on the tray to clear the garage; the work is not done until the line here is deleted.

## Next

- [ ] **Slide older tasks to today** (aq2s) — one action (a key, or a verb in the `enter` menu) that
  moves every open task due before today onto today, instead of re-dating them one by one.
- [ ] `[shape]` **A cron garage** (9d9t) — a TUI view listing every task set up as a cron or habit,
  with its rule and next fire. The UX is still being refined; the agent that sets these up
  lives in traytable.
- [ ] `[plugin]` **A web garage** — `settings.example.json` asks for a URL and a ruleset
  ("only rows marked in progress"); `sync plan` drives the site with `web-agent` (login
  happens in the browser it opens), screenshots it as evidence, and reads rows off the page
  under the ruleset; `sync apply` marks done where the site allows. Done when two plans on
  an unchanged page produce an empty diff, and the review's rows match the screenshot on
  one real list.
- [x] `[plugin]` **Google Calendar** — `tray-plugins/gcal`: verb-only, on the contract, with a
  `health` probe that asks Google whether the credentials still answer.
- [x] `[plugin]` **Turso** — `tray-plugins/turso`: an `all-rows` plugin that keeps a
  copy of every task in a Turso database and merges three ways against the rows it last
  pushed, local winning every conflict; a row typed on another device comes back as a
  garage line for review (T37). Python, stdlib only, the HTTP pipeline API.
- [ ] `[plugin]` **A board's API** — Linear or Jira over `sync plan|apply`; the board's
  project is a tag (9).
- [ ] `[plugin]` **Claude conversations** — an `actions/attach` that compacts a transcript
  into the task's note, replaced whole (104), with its model key in its own `settings.json`
  (T40). Needs one input, the session; how a verb asks for one is decided when this is built.
- [ ] `[shape]` **Which fields cross the plugin boundary** — `asRows` sends more of a row than
  `compare` and `Apply` handle, so the rest of a pulled row is dropped. Decide the set once.
- [ ] **Bare states in `tray plugin --json`** — `state` carries prose (`"failed — boom"`), so
  an agent cannot `select(.state=="failed")`; the prose belongs in its own `message`.
- [ ] **A sync review in the TUI** — `S` runs the built-in hooks; a plugin's plan still needs
  `tray sync --apply`, because the interface has no screen for the diff yet.
- [ ] **The mirror in a vault** — `tray.md` and `garage.md` are written beside the database;
  pointing them elsewhere (an Obsidian folder) wants the one setting there is no file for
  yet. A symlink does it today.
- [ ] **Prebuilt binaries** — goreleaser. `go install` is the only path today, so a Go
  toolchain is a hard requirement for anyone who wants this.
- [ ] **CI** — `make check` is the whole suite and no workflow runs it.
- [ ] **A second demo take** — the recording predates the id column, and never shows `/`
  or review mode. Kit is in `~/tray-demo/`.

## Parked

- [ ] `[shape]` **A desktop app in place of the TUI** — Fyne, archived as trayfyne (branch `traytable`,
  draft #5), stale against `main`. The TUI stays.
- [ ] **Simpler key sequences** — the letters and their chords are a lot to learn; fewer,
  more obvious ones would help new users.
- [ ] `[ours]` **A tray server** — one database, many clients, for more than one machine.
  The CLI is already the API; what is missing is a listener and a story for the phone.
- [ ] **`u` undo, one level** — needs a snapshot in `store`. `E` erase is the only action that
  leaves nothing to recover by hand.
- [ ] `[ours]` **Eisenhower view** — `core.Quadrant` is written and `tray export` emits it;
  what is missing is somewhere to look at it.
- [ ] `[ours]` **Priorities on tags** — urgency counts tags and never weighs which. Weighing
  needs a tag registry, the thing 18 exists to avoid.
- [ ] `[personal]` **Journal integration** — `tray print` emits the bullets; scraping them back
  is unbuilt, and a script outside this repo keeps personal shape out of a general tool.
- [ ] **Revisit the colours `tray head` uses** — it is the one surface that spends four
  palette entries at once. Every one is from the palette, so this is taste rather than
  consistency.
- [ ] `[shape]` **A preferences file** — the date format is the one preference left now that
  todo.txt is an export rather than a row format. T7 covers a plugin's settings and nothing
  of yours; 18 still keeps the tag vocabulary in the rows.
- [ ] `[shape]` **`project` and other detail fields** — `project` was ruled out (9) and now
  arrives as a tag from every wire. Reopening a settled one is allowed.
- [ ] `[shape]` **Nested task sets** — would this even match the ethos?
