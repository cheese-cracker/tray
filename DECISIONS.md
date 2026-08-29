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
| 7 | take · hand back · carry forward are **one operation** | Three rituals were three code paths for the same move | live — but see 71: sharing the *move* is not sharing the *command* |
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
| 18a | Widgets are **hand-rolled** — radio, text fields, table, tabs | Only `bubbletea` + `lipgloss`; no `bubbles`. Cheap so far, but text editing is the thin ice | **partly reversed → 56** |

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
| 28 | Two tabs day to day; **`tray carryover` opens the months as tabs** | The sweep is the one ritual that's about months | live, widened by 73 |
| 28a | Tabs **cycle** at either end | With two or three tabs, stopping just makes you reach for the other key | live |
| 29 | Tags are picked from a closed list, never typed | Feared vocabulary sprawl | **reversed → 30** |
| 30 | **Tags are typed**; the ones in use show as a hint | A picker was friction for the rarer case of a new tag | live |
| 31 | Priority cycles `none · L · M · H`, clamped at the ends | Wrapping H round to none wiped priorities in a batch | **reversed → 32** |
| 32 | Priority is a **radio `H · M · L`, defaulting to M** | No "none": an unset task reads as medium | live |
| 33 | The garage form asks for the title alone; the tray form asks for everything | Structure is expected on the tray, nowhere else | live |
| 34 | `tray add`/`take` **warn** what's missing rather than prompting | The CLI is the agent surface; it can't start asking questions | live |
| 34a | The menu leads with the layer's primary action: `retake` on the tray, `take` in the garage | `enter enter` should do the obvious thing | live |
| 34b | Handing back **revives the line it came from** rather than adding a copy | Dedupe counted departed lines, so the task left the tray and lived nowhere | live, corrected by 68 |
| 35 | Full-page layout, list windows around the cursor | Overflow makes the alt screen jitter | live |
| 36 | Dates **display** with the weekday (`2026-08-12 Wed`); files and JSON stay ISO | The day is what tells you whether something is soon | live |

## Testing

| # | Decision | Why | Status |
|---|---|---|---|
| 37 | Shell suite asserts **file contents**, not internals | It survived a whole-language rewrite unchanged | live |
| 38 | stdin closed throughout the shell suite | A prompt an agent could hit fails the suite instead of hanging it | live |
| 39 | Subprocess assertions run from `/` with a stripped environment | Two blank-screen bugs passed CI by inheriting my `PATH`, `PYTHONPATH` and cwd | live |
| 40 | UI: assert **model state and files**; frames only for structure | Golden frames break on every restyle and prove nothing about what was written | live, narrowed by 64 |
| 41 | pty scraping of the real binary | Hung twice and gave a false negative once | abandoned → 62 |
| 42 | A build failure **fails** the shell suite; only a missing toolchain skips | Skipping reported "all flows pass" for a repo that didn't compile | live |
| 43 | New assertions are mutation-checked: break the code, watch them fail | Two assertions have now passed for the wrong reason | live |

## Not built

| # | Item | Plan | Status |
|---|---|---|---|
| 44 | `/` search | **Hand off to `fzf` via `tea.ExecProcess`** — no matching code of ours. Check the alt-screen seam first | **reversed → 57** |
| 45 | Hand-rolled fuzzy matcher, or `sahilm/fuzzy` | Scoring heuristics are a rabbit hole; a dependency for one keystroke | **half stands → 57** |
| 46 | `u` undo, one level | Copy-forward keeps everything hand-recoverable meanwhile | open |
| 47 | `?` help | The footer already carries the live keymap | **reversed → 59, then 86** |
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

## The list layer

| # | Decision | Why | Status |
|---|---|---|---|
| 56 | **`bubbles/list`** is the list layer | Scrolling, paging and filtering, none of it ours. 18a's table and tabs survive intact; only the machinery under them changed | live |
| 56a | The table is a **custom `ItemDelegate`**, not `list`'s default | A delegate renders whatever string you hand it, so the aligned columns, the `●`/`▸` glyphs and the header row all came across unchanged. Adopting `list` did not cost the table | live |
| 56b | Columns are measured over the **visible** rows | A filter tightens the table instead of leaving it padded for rows that are no longer there | live |
| 56c | Cells are truncated before they are padded | `lipgloss` wraps rather than clips, and one wrapped row pushes every row below it down — which is the jitter 35 exists to prevent | live |
| 57 | `/` is **`bubbles/list`'s fuzzy filter**, in process | Reverses 44: no `fzf`, no `tea.ExecProcess`, no alt-screen seam, no runtime dependency. 45's real point — don't hand-roll a matcher — still stands; `sahilm/fuzzy` arrives as `list`'s transitive dependency rather than as a choice of ours | live |
| 57a | An applied filter states what it hid | A table quietly showing 3 of 17 rows is a table you will misread | live |
| 57b | **A mark survives a filter** | Hiding a row is not deselecting it. Filter, mark, filter again, act on all of it | live |
| 58 | The list's `h`/`l`/`d` paging keys are unbound, and its quit keys disabled | `h`/`l` are the tabs and `d` hands back. The cursor keys page on their own | live |
| 59 | `?` is **`bubbles/help`** over one `keyMap` that also renders the footer | Reverses 47. The overlay and the footer cannot drift apart, because they are the same value rendered at two lengths | live |
| 59a | The footer drops `space mark`; `?` carries it | It is the least guessable key, and the footer has to stay inside eighty columns on either layer | **reversed → 84** |
| 60 | The form stays **hand-rolled** — no `bubbles/textinput` yet | `←`/`→` on the `due` field shift by a day (25, and documented). A text input would take those keys for cursor movement. 18a's thin ice is still thin — and 67 is what fell through it | open |

## Flows

| # | Decision | Why | Status |
|---|---|---|---|
| 61 | **FLOWS.md is enforced**, not prose | `internal/flows` fails the build on a promise with no test or a flow test with no promise. A flow document nobody checks is a comment | live |
| 61a | It parses the Go suite with `go/ast`, not `grep` | A test name inside a comment or a string must not satisfy a row | live |
| 62 | Complex flows run under **`teatest`**, driving the real program | Reopens 41: teatest is not a pty, and the one place it can block is `WaitFinished` with no timeout | live |
| 62a | **No test touches `teatest` directly** — everything goes through the harness in `tui_test.go` | That is the only way "every wait is bounded" can be a property of the suite rather than a habit. `go test -timeout` in the Makefile is the backstop under it | live |
| 62b | `waitFor` matches **every frame so far**, not just the newest | bubbletea rewrites changed lines only, so text already on screen may never be sent again | live |
| 62c | Keystrokes need no wait; commands do | Keys and Quit share one ordered message channel. The filter re-runs as a command, so that one does need waiting on | live |
| 63 | teatest asserts on the **final model and the files**, never on a frame | 40 still holds. teatest buys a real program, not a new thing to assert on | live |
| 64 | **One golden per screen**, colour stripped, and none of them assert behaviour | Narrows 40 rather than reversing it: goldens catch a lost border, a column that stopped aligning, a footer that overflowed into `…` — none of which a behaviour test can see | live |
| 65 | The F suite builds the binary itself | The `bin/tray` wrapper that used to do it went with super-utils (52) | live |

## Two bugs the hand-rolled form had

| # | Decision | Why | Status |
|---|---|---|---|
| 66 | The priority radio **draws and steps off one slice** | It was two literals in opposite orders — `{"L","M","H"}` stepped, `{"H","M","L"}` drawn — so `l` and `→` moved the dot left. One slice is the only fix that cannot drift again | live |
| 66a | The test asserts the **rendered dot** moves the way the key points | Asserting `prio == "H"` would have passed the whole time the keys were backwards. Order-agnostic, so it survives a reordering of the scale | live |
| 67 | A `KeyRunes` message with **many runes is always text** | Bracketed paste is on by default and delivers a paste as one message with every rune. The form only handled single-rune messages, so pastes vanished silently | live |
| 67a | Pasted newlines collapse to a space; other control characters are dropped | A task is one line of markdown. A raw `\n` split it in two — and the attributes went with the tail, so the surviving half silently lost its priority | live |
| 67b | `bubbles/textinput` already did both | It sanitises pasted runes the same way, which is why `/` never had this bug and the form did. Evidence for 60, not against it | noted |

## Coming home

Found by using it: dumping a checklist into a month, taking part of it, then
unloading. Both promises TRAY.md makes about `unload` were false on that path.

| # | Decision | Why | Status |
|---|---|---|---|
| 68 | Coming home **reclaims** the departed line — it writes the task's current state onto it, rather than restoring what the line used to say | 34b was right that the line must come home rather than be copied, but `Revive` restored the *garage's* old text. So a task finished on the tray came home **open**, and every carryover after that carried completed work forward again. Open ones came home bare, so "taking them again is free" was false too | live |
| 68a | `from:` is dropped on the way home | It records which month a task graduated from; on a line living in that month it is tautology. The exact inverse of what take adds | live |
| 68b | F7 passed throughout | Its tray never came from the month it unloaded into, so it only ever exercised the copy path, where nothing was wrong. F17 is the round trip — dump here, take, hand back — which is the common case and was untested | live |

## The month turn

Designed by running the tool for real: two checklists dumped into two months, then
asking what the turn actually looks like. Almost every question turned out to be about
what is *guessed*.

| # | Decision | Why | Status |
|---|---|---|---|
| 69 | **The closing month is not a fact about the calendar** | Sweeping August into September is the same job on Aug 30, when August is current, and on Sep 10, when it is previous. No date offset gets both, so nothing infers it | live |
| 70 | `carryover --run` **requires `--month`** | The one rule that cannot cascade. "Oldest month with live lines" is right until you run it twice — the second run marches the month you just carried *into* forward again | live |
| 71 | `carryover` **no longer drains the tray** | It used to unload the whole tray into the closing month first. Run mid-August that meant a fabricated `2026-07.md` holding your live work. Two rituals, two names, run in order | live |
| 72 | `unload` **requires `--to`**, picks it on a terminal, errors when piped | Emptying the tray is the largest single action here. Decision 20 says a bare command on a terminal is a UI and 34 says the CLI must not prompt — a picker on a TTY and an error otherwise is both at once | live |
| 73 | The sweep is **four tabs**: prev · this · next · someday | You need somewhere to put things as much as somewhere to take them from. `--month` replaces the closing tab, so a month you skipped stays reachable | live |
| 73a | It opens on **this** month, always | Landing somewhere predictable beats landing somewhere clever that is sometimes wrong — and by 69, "clever" has nothing honest to aim at | live |
| 73b | The `garage ·` prefix is dropped in the sweep | It earns its place next to "tray" and nowhere else. Four prefixed labels also overflow an eighty-column terminal | live |
| 74 | **Quitting the sweep carries nothing**, and the docs now say so | The docs claimed otherwise for as long as the sweep has existed. Triage and carry are separate acts; the sweep is the first | live |
| 75 | A **due date that has already passed is not carried forward** | Carrying a line forward is admitting the date did not hold. Keeping it means every re-take starts overdue and urgency is junk. The source line keeps it | live |
| 76 | **`--nag` deleted** | Nothing ever ran it. It was built to live in a shell profile, super-utils' installer is gone, and `tray install` is not written — so it was a nag you had to remember to trigger. `tray status` carries the warning until something can install it | live |
| 77 | **`--all` means one thing everywhere**: show the finished too | It was aliased to `dense`, so the tray table quietly included finished work while the garage could never show it at all. `carryover` uses `--run` now | live |
| 77a | An unknown `--flag` is an **error** | `garage list --all` was silently swallowed, which is how 77 went unnoticed. Unknown flags fell into the filter list and vanished | live |

## The terminal header

| # | Decision | Why | Status |
|---|---|---|---|
| 78 | `tray head [n]` is its own report, not `list \| head` | A shell profile runs it on every terminal, which changes what good output is: ids you didn't ask for are clutter, and the urgency figure is noise at a glance | live |
| 78a | **Silent on an empty tray** | A header that says "nothing to do" is one you stop reading in a week, and a clear day should cost a new terminal nothing | live |
| 78b | Dates are **relative**, and the past says so | `Mon` for a task due last Monday reads as upcoming. Getting that wrong in a header is worse than omitting the date | live |
| 78d | The header is a **titled box**, rows tinted by priority | Your call over my recommendation: I argued a rule was quieter for something seen fifty times a day, you wanted the frame. It matches the TUI's own pane, which is the better consistency argument | live |
| 78e | The box is **hand-drawn**, not `lipgloss.Border` | The title sits *in* the top edge, and splicing it into an already-coloured border is guesswork about where the escape codes fall | live |
| 78f | No count in the title | It was answering a question nobody asks at a prompt | live |
| 78g | `internal/style` holds the palette | The header and the TUI are drawn by different packages. Two copies of the accent colour is a drift waiting to happen | live |
| 78c | An unset priority prints `·`, not `M` | 32 says unset *reads* as medium; printing the letter would claim you chose it | live |

## Undoing a finish

Reported from use: two tasks went done when one was meant to.

| # | Decision | Why | Status |
|---|---|---|---|
| 79 | `v` reveals the finished rows, struck through and marked `✓`/`✗` | They were unreachable from the TUI entirely — the only way to see one was `tray list --all` or opening the file | live |
| 80 | A finished row offers **restore and nothing else** | The only sane thing to say about a record is that it isn't one. `x` on a done task re-stamps it and `d` hands a finished task back — meaningless or quietly destructive | live |
| 80a | **All** finished, not any | A selection spanning both kinds has no single sensible verb, so it keeps the normal menu | live |
| 81 | Restore leaves **no trace** | Decision 5 is about never deleting a *line*; this deletes nothing. The overwhelming reason to reach for it is a mis-key, and a line stamped with every fumble is worse than one that is simply correct | live |
| 82 | `tray <ids> restore` resolves ids against **`list --all`** | Numbering the finished rows separately was tidier to implement and a trap to use: you read `2✓` off the screen and 2 meant something else. Caught by trying it once | live |
| 83 | Nothing was done about the mis-select itself | Considered a footer mark count, confirm-on-batch, and `u` undo. Your call: not a problem worth solving, and simpler is better. Marks still silently win over the cursor row, and a letter pressed from the list still skips the menu's count | open by choice |

## The footer

| # | Decision | Why | Status |
|---|---|---|---|
| 84 | The footer **wraps**; it does not truncate | Reverses 59a. `bubbles/help` cuts the line with an ellipsis, which silently hides whichever keys sort last — on an eighty-column terminal that was most of them. Wrapping costs one row and drops nothing | live |
| 84a | So `space mark` is back, and `v` is named whether it is on or off | Both had been sacrificed to the one-line budget. `v` was worse: it appeared only once already enabled, so the footer could never tell you the key existed | live |
| 84b | The test asserts **every binding the keymap knows** appears on screen | Asserting specific labels would not have caught this: the footer was correct, it was just cut off. Mutation-checked against the truncating version | live |

## Teaching the interface

| # | Decision | Why | Status |
|---|---|---|---|
| 85 | The footer names **the arrows**; `h j k l` work but are written down only in `?` | They are the keys someone opening this for the first time already tries. A footer that lists two ways to do one thing teaches neither | live |
| 85c | **`tab` alone** switches layers; `←→` and `h` `l` are unbound | ↑↓ move within a layer and nothing here moves sideways, so every sideways key was another idiom for a job `tab` already names. `⇧tab` goes back and is not advertised. `j` `k` survive as the one alias, because moving is the thing you do constantly | live |
| 85a | `g` `G` `home` `end` `pgup` `pgdn` are **unbound** | Four more ways to move one cursor. Paging follows the cursor on its own | live |
| 85b | `mark` is called **select** everywhere now | The menu had said "N selected" since the beginning while the footer said "mark". One of them had to give | live |
| 86 | `?` is a **page**, not a keymap strip | Narrows 59. A keymap tells you which letter does a thing you already understand; what needs explaining here is why there are two layers at all. So the diagram comes first and the keys come last, as one section | live |
| 86a | It **floats over** the list as a dialog | Replacing the list read as the pane changing rather than something opening. lipgloss has no compositor, so the splice is cell-wise via `ansi.Truncate`/`TruncateLeft` — cutting a styled line by byte offset shreds the escapes | live |
| 86b | Help column widths are **measured from the labels**, not chosen | Hand-picked widths broke three times running: the last column wrapped at 22, "show done" collided at 18, and "tab  h l" was clipped to "tab  h…" by a key column sized for a shorter key | live |
| 86c | A test asserts the dialog fits the terminal in both directions and nothing is truncated | The measuring is only as good as the check. Three layout regressions reached a golden before this existed — goldens record what happened, they do not object to it | live |
| 86d | The dialog is sized so the frame shows around it | A popup spanning the whole terminal is not a popup. That cost the fourth keymap column and two words per box line | live |
| 86e | **Any key dismisses it**, and the key is spent doing so | You should never have to work out which key closes a thing that is in your way. `ctrl+c` still quits | live |
| 86f | It opens by saying **what tray is** — two lines, before the diagram | The diagram answers "why two layers"; nothing answered "what am I looking at". It names `~/task-garage`, which is the other thing a newcomer needs | live |
| 86g | Keys are a **separate lower section**, under a rule | The concept half and the reference half are read at different times and by different people | live |
