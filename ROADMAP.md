# Roadmap

Rough order, not commitments. Every choice already made — including the reversed ones —
is one line in [DECISIONS.md](DECISIONS.md).

## Now

- [ ] **`/` fuzzy filter** — `bubbles/list` filtering, in-process. No `fzf`, no
  `tea.ExecProcess`, no alt-screen seam to worry about. The month sweep needs it once a
  month file has real volume.
- [ ] **`?` help** — `bubbles/help` over the same keymap the footer renders, so the
  overlay and the footer cannot drift apart.

## Next

- [ ] **`u` undo, one level** — needs a snapshot in `store`; not free. Copy-forward keeps
  everything hand-recoverable meanwhile, so this is comfort rather than safety.
- [ ] **`tray install`** — self-installing subcommand in Go (`charmbracelet/huh` for the
  prompts, `--yes` for headless): PATH check, binary placement, shell init block. Lands
  behind a runtime feature flag (`internal/feature`, env + config), which is why that
  package doesn't exist yet — nothing to gate until this does.

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
