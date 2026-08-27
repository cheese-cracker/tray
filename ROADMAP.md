# Roadmap

Rough order, not commitments. Every choice already made — including the reversed ones —
is one line in [DECISIONS.md](DECISIONS.md).

## Next

- [ ] **`tray install`** — the nag has no home until this exists. See below.

- [ ] **`bubbles/textinput` in the retake form** — 18a called hand-rolled text editing
  the thin ice and it still is: no cursor, no word motions, no paste. The blocker is
  that `←`/`→` on the `due` field shift by a day, which a text input would claim for
  cursor movement. Needs a decision before it needs code.

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

## Rejected

| | Why |
|---|---|
| `fzf` shell-out for `/` | A runtime dependency and an alt-screen seam, for a keystroke `bubbles/list` already does in-process |
| Hand-rolled fuzzy matcher | Scoring heuristics are a rabbit hole |
| Taskwarrior as the store | TW 3.x is `taskchampion.sqlite3`; a binary store can't be hand-edited |
| `project` as a second axis | Tags are the only axis — one dimension, nothing to decide twice |
