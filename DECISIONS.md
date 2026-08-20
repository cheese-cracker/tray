# tray — decision log

Every choice that isn't obvious from the code, one line each. **Reversed** rows are
kept rather than deleted; a decision log that only shows the winners can't be audited.

## Shape

| # | Decision | Why | Status |
|---|---|---|---|
| 1 | Two layers: **garage** (dump) and **tray** (worklist) | Classifying at capture costs you the thought you were having | live |
| 2 | Markdown files are the truth | You must be able to edit them with no tool in the loop | live |
| 3 | **Not** Taskwarrior as the store | TW 3.x is `taskchampion.sqlite3` — a binary store can't be hand-edited | live |
| 4 | Keep TW's field names and urgency coefficients | `tray export \| task import` stays a one-way hatch | live |
| 5 | **Nothing is ever deleted.** `done`/`drop` strike through in place | The file is the record; that's what makes `find` a rot detector | live |
| 6 | Copy-forward with an arrow: the source line stays, annotated `→ 2026-09` | Month files stay a record, and nothing can move twice | live |
| 7 | take · hand back · carry forward are **one operation** | Three rituals were three code paths for the same move | live |
| 8 | Ids are positional and ephemeral, never stored | A hand-edit can then never desync them | live |
| 9 | `project` dropped; **tags are the only axis** | One dimension, nothing to decide twice | live |
| 10 | Someday and old months have no tab | Reachable via `>`; they don't earn standing room | live |

## Implementation

| # | Decision | Why | Status |
|---|---|---|---|
| 11 | stdlib **Python**, fzf + gum | Fastest thing that could work | **reversed → 15** |
| 12 | Python core + Go TUI over a JSON contract | Thought the tests were tied to Python | **reversed → 15** |
| 13 | Shell-only core | Rejected: floats, BSD/GNU date math, hand-built JSON | rejected |
| 14 | Full Go rewrite is safe because `check-tray.sh` is **black-box** | It drives the binary and reads markdown, so it ported unchanged | live |
| 15 | **One Go binary**, zero runtime dependencies | No interpreter, no fzf, no gum; `go` builds it under `--tui` | live |
| 16 | `ui` and `cmd` are clients of `core`; neither parses a line or writes a file | If the UI needs the grammar, the boundary is wrong | live |
| 17 | Attributes are read off the **end** of a line, known keys only | A colon mid-sentence must survive, or the garage can't hold prose | live |
| 18 | The tag vocabulary comes from what's already in use | No config, no registry | live |
| 18a | Widgets are **hand-rolled** — radio, text fields, table, tabs | Only `bubbletea` + `lipgloss`; no `bubbles`. Cheap so far, but text editing is the thin ice — see 44a | live, watch |

## Interface

| # | Decision | Why | Status |
|---|---|---|---|
| 19 | The TUI is the product; the CLI is the agent surface | ~90% of use is the terminal UI | live |
| 20 | Bare `tray` is the TUI **only when both ends are a terminal** | Piped, an agent must never be handed a UI | live |
| 21 | fzf list bound like ranger | Cheap, and fzf was already a dependency | **reversed → 22** |
| 22 | bubbletea, with tabs and a real pane | fzf can't do in-place editing or panes | live |
| 23 | Six keys: `j k` · `space` · `enter` · `tab` · `a`/`n` · `q`/`esc` | Everything else lives in the menu | live |
| 24 | `enter` opens a menu whose letters **also work from the list** | Discoverable on day one, one keystroke by week two | live |
| 25 | `retake` is **one form, every field prefilled** | Only what you touch changes; no wizard | live |
| 26 | `d` hands back to the garage, `D` deletes | Your mapping | live |
| 27 | `D` strikes through rather than removing the line | Consistent with 5. Flagged as overridable | live |
| 28 | Two tabs day to day; **`tray carryover` opens the months as tabs** | The sweep is the one ritual that's about months | live |
| 28a | Tabs **cycle** at either end | With two or three tabs, stopping just makes you reach for the other key | live |
| 29 | Tags are picked from a closed list, never typed | Feared vocabulary sprawl | **reversed → 30** |
| 30 | **Tags are typed**; the ones in use show as a hint | A picker was friction for the rarer case of a new tag | live |
| 31 | Priority cycles `none · L · M · H`, clamped at the ends | Wrapping H round to none wiped priorities in a batch | **reversed → 32** |
| 32 | Priority is a **radio `H · M · L`, defaulting to M** | No "none": an unset task reads as medium | live |
| 33 | The garage form asks for the title alone; the tray form asks for everything | Structure is expected on the tray, nowhere else | live |
| 34 | `tray add`/`take` **warn** what's missing rather than prompting | The CLI is the agent surface; it can't start asking questions | live |
| 34a | The menu leads with the layer's primary action: `retake` on the tray, `take` in the garage | `enter enter` should do the obvious thing | live |
| 34b | Handing back **revives the line it came from** rather than adding a copy | Dedupe counted departed lines, so the task left the tray and lived nowhere | live |
| 35 | Full-page layout, list windows around the cursor | Overflow makes the alt screen jitter | live |
| 36 | Dates **display** with the weekday (`2026-08-12 Wed`); files and JSON stay ISO | The day is what tells you whether something is soon | live |

## Testing

| # | Decision | Why | Status |
|---|---|---|---|
| 37 | Shell suite asserts **file contents**, not internals | It survived a whole-language rewrite unchanged | live |
| 38 | stdin closed throughout the shell suite | A prompt an agent could hit fails the suite instead of hanging it | live |
| 39 | Subprocess assertions run from `/` with a stripped environment | Two blank-screen bugs passed CI by inheriting my `PATH`, `PYTHONPATH` and cwd | live |
| 40 | UI: assert **model state and files**; frames only for structure | Golden frames break on every restyle and prove nothing about what was written | live |
| 41 | pty scraping of the real binary | Hung twice and gave a false negative once; `teatest` covers it properly | abandoned |
| 42 | A build failure **fails** the shell suite; only a missing toolchain skips | Skipping reported "all flows pass" for a repo that didn't compile | live |
| 43 | New assertions are mutation-checked: break the code, watch them fail | Two assertions have now passed for the wrong reason | live |

## Not built

| # | Item | Plan | Status |
|---|---|---|---|
| 44 | `/` search | **Hand off to `fzf` via `tea.ExecProcess`** — no matching code of ours. Check the alt-screen seam first | open |
| 45 | Hand-rolled fuzzy matcher, or `sahilm/fuzzy` | Scoring heuristics are a rabbit hole; a dependency for one keystroke | rejected |
| 46 | `u` undo, one level | Copy-forward keeps everything hand-recoverable meanwhile | open |
| 47 | `?` help | The footer already carries the live keymap | open |
| 48 | Shipping: `go` in `Brewfile.tui`, built at install | Nothing vendored, no release pipeline. Prebuilt binaries if it ever ships wider | live, wants review |
| 49 | Journal seeding (`- [ ]` scrape) | Only if the recurring-item problem comes back | deferred |
| 50 | `tray dump` asking for the month on a TTY | The Python version's gum picker; the Go rewrite dropped it. `a` in a garage tab covers it | dropped, re-addable |

## Repo

Extracted from `super-utils` into its own repo, 2026-08-20. The `super-utils` worktree it
lived in was orphaned, so none of the history came with it — the import is one commit.

| # | Decision | Why | Status |
|---|---|---|---|
| 51 | `tray` is **its own repo**, module `github.com/cheese-cracker/tray` | It was `--tui` in a coreutils installer; nothing about it is a shell alias | live |
| 52 | The `bin/tray` bash wrapper is **dropped** | It existed to rebuild-on-stale from a symlink in `~/.local/bin`; `go install` is the same thing without the shell | live |
| 53 | `~/task-garage` stays the default home | Decision 2 says you must be able to edit these with no tool in the loop, and XDG buries them where nobody looks | live |
| 54 | `scripts/check-tray.sh` **stays bash**, and builds the binary itself | Decision 37: it survived a whole-language rewrite unchanged. That property is worth one non-Go file | live |
| 55 | No `internal/feature` until `tray install` needs it | A flag package with zero consumers is speculative code you can't test | live |
