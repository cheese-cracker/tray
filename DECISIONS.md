# tray — decision log

Every choice that isn't obvious from the code, one line each. A losing option is kept
only where it **still tempts** — 9 is the example: `project` is a reasonable thing to
ask for again, so the reason there isn't one has to stay written down. Reversals nobody
will re-propose are deleted rather than carried; `git log -p DECISIONS.md` is the audit
trail, and it is a better one than a row that has to be read past every time.

A row here is a decision, not a wall. Where one is genuinely open the roadmap says so,
and the two are meant to agree.

## Shape

| # | Decision | Why | Status |
|---|---|---|---|
| 1 | Two layers: **garage** (dump) and **tray** (worklist) | Classifying at capture costs you the thought you were having | live |
| 2 | Markdown files are the truth | You must be able to edit them with no tool in the loop | live |
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
| 14 | Full Go rewrite is safe because `check-tray.sh` is **black-box** | It drives the binary and reads markdown, so it ported unchanged | live |
| 15 | **One Go binary**, zero runtime dependencies | No interpreter, no fzf, no gum; `go` builds it under `--tui` | live |
| 16 | `ui` and `cmd` are clients of `core`; neither parses a line or writes a file | If the UI needs the grammar, the boundary is wrong | live |
| 17 | Attributes are read off the **end** of a line, known keys only | A colon mid-sentence must survive, or the garage can't hold prose | live |
| 18 | The tag vocabulary comes from what's already in use | No config, no registry | live |

## Interface

| # | Decision | Why | Status |
|---|---|---|---|
| 19 | The TUI is the product; the CLI is the agent surface | ~90% of use is the terminal UI | live |
| 20 | Bare `tray` is the TUI **only when both ends are a terminal** | Piped, an agent must never be handed a UI | live |
| 22 | bubbletea, with tabs and a real pane | A picker can select a line; it can't do in-place editing or panes | live |
| 24 | `enter` opens a menu whose letters **also work from the list** | Discoverable on day one, one keystroke by week two | live |
| 25 | `rewrite` is **one form, every field prefilled** | Only what you touch changes; no wizard | live |
| 26 | `d` hands back to the garage, `D` deletes | Your mapping | live |
| 27 | `D` strikes through rather than removing the line | Consistent with 5. Flagged as overridable | live |
| 28 | Two tabs day to day; **`tray carryover` opens the months as tabs** | The sweep is the one ritual that's about months | live, widened by 73 |
| 28a | Tabs **cycle** at either end | With two or three tabs, stopping just makes you reach for the other key | live |
| 30 | **Tags are typed**; the ones in use show as a hint | A picker was friction for the rarer case of a new tag | live |
| 32 | Priority is a **radio `H · M · L`, defaulting to M** | No "none": an unset task reads as medium | live |
| 33 | The garage form asks for the title alone; the tray form asks for everything | Structure is expected on the tray, nowhere else | live |
| 34 | `tray add`/`take` **warn** what's missing rather than prompting | The CLI is the agent surface; it can't start asking questions | live |
| 34a | The menu leads with the layer's primary action: `rewrite` on the tray, `take` in the garage | `enter enter` should do the obvious thing | live |
| 35 | Full-page layout, list windows around the cursor | Overflow makes the alt screen jitter | live |
| 36 | Dates **display** with the weekday (`2026-08-12 Wed`); files and JSON stay ISO | The day is what tells you whether something is soon | live |

## Testing

| # | Decision | Why | Status |
|---|---|---|---|
| 37 | Shell suite asserts **file contents**, not internals | It survived a whole-language rewrite unchanged | live |
| 38 | stdin closed throughout the shell suite | A prompt an agent could hit fails the suite instead of hanging it | live |
| 39 | Subprocess assertions run from `/` with a stripped environment | Two blank-screen bugs passed CI by inheriting my `PATH`, `PYTHONPATH` and cwd | live |
| 40 | UI: assert **model state and files**; frames only for structure | Golden frames break on every restyle and prove nothing about what was written | live, narrowed by 64 |
| 42 | A build failure **fails** the shell suite; only a missing toolchain skips | Skipping reported "all flows pass" for a repo that didn't compile | live |
| 43 | New assertions are mutation-checked: break the code, watch them fail | Two assertions have now passed for the wrong reason | live |

## Repo

Extracted from `super-utils` into its own repo, 2026-08-20. The `super-utils` worktree it
lived in was orphaned, so none of the history came with it — the import is one commit.

| # | Decision | Why | Status |
|---|---|---|---|
| 51 | `tray` is **its own repo**, module `github.com/cheese-cracker/tray` | It was `--tui` in a coreutils installer; nothing about it is a shell alias | live |
| 52 | The `bin/tray` bash wrapper is **dropped** | It existed to rebuild-on-stale from a symlink in `~/.local/bin`; `go install` is the same thing without the shell | live |
| 53 | The default home is **visible, in `~`** | 2 says you must be able to edit these with no tool in the loop, and XDG buries them where nobody looks. The convention splits on ownership, not platform: Taskwarrior hides `~/.task/` because you are not meant to open a sqlite file; org-mode, Obsidian and todo.txt stay visible because you are | live |
| 53a | It is **`~/tray`**, not `~/task-garage` | The directory holds `tray.md` as well as the months, so the old name announced one of the two layers and not the pair. `~/tray` matches the binary and is a word shorter | live |
| 54 | `scripts/check-tray.sh` **stays bash**, and builds the binary itself | Decision 37: it survived a whole-language rewrite unchanged. That property is worth one non-Go file | live |
| 55 | No `internal/feature` until `tray install` needs it | A flag package with zero consumers is speculative code you can't test | live |

## The list layer

| # | Decision | Why | Status |
|---|---|---|---|
| 56 | **`bubbles/list`** is the list layer | Scrolling, paging and filtering, none of it ours. The table and the tabs survive intact; only the machinery under them changed | live |
| 56a | The table is a **custom `ItemDelegate`**, not `list`'s default | A delegate renders whatever string you hand it, so the aligned columns, the `●`/`▸` glyphs and the header row all came across unchanged. Adopting `list` did not cost the table | live |
| 56b | Columns are measured over the **visible** rows | A filter tightens the table instead of leaving it padded for rows that are no longer there | live |
| 56c | Cells are truncated before they are padded | `lipgloss` wraps rather than clips, and one wrapped row pushes every row below it down — which is the jitter 35 exists to prevent | live |
| 57 | `/` is **`bubbles/list`'s fuzzy filter**, in process | In process: no `fzf`, no `tea.ExecProcess`, no alt-screen seam, no runtime dependency. No matcher of ours either — scoring heuristics are a rabbit hole, and `sahilm/fuzzy` arrives as `list`'s transitive dependency rather than as a choice | live |
| 57a | An applied filter states what it hid | A table quietly showing 3 of 17 rows is a table you will misread | live |
| 57b | **A mark survives a filter** | Hiding a row is not deselecting it. Filter, mark, filter again, act on all of it | live |
| 58 | The list's `h`/`l`/`d` paging keys are unbound, and its quit keys disabled | `h`/`l` are the tabs and `d` hands back. The cursor keys page on their own | live |
| 59 | `?` is **`bubbles/help`** over one `keyMap` that also renders the footer | The overlay and the footer cannot drift apart, because they are the same value rendered at two lengths | live |
| 60 | The form stays **hand-rolled** — no `bubbles/textinput` yet | `←`/`→` on the `due` field shift by a day (25, and documented). A text input would take those keys for cursor movement — and it already sanitises a pasted rune the way T13 requires, which hand-rolled editing had to be taught | open |

## Flows

| # | Decision | Why | Status |
|---|---|---|---|
| 61 | **FLOWS.md is enforced**, not prose | `internal/flows` fails the build on a promise with no test or a flow test with no promise. A flow document nobody checks is a comment | live |
| 61a | It parses the Go suite with `go/ast`, not `grep` | A test name inside a comment or a string must not satisfy a row | live |
| 62 | Complex flows run under **`teatest`**, driving the real program | teatest drives the real program without a pty, and the one place it can block is `WaitFinished` with no timeout | live |
| 62a | **No test touches `teatest` directly** — everything goes through the harness in `tui_test.go` | That is the only way "every wait is bounded" can be a property of the suite rather than a habit. `go test -timeout` in the Makefile is the backstop under it | live |
| 62b | `waitFor` matches **every frame so far**, not just the newest | bubbletea rewrites changed lines only, so text already on screen may never be sent again | live |
| 62c | Keystrokes need no wait; commands do | Keys and Quit share one ordered message channel. The filter re-runs as a command, so that one does need waiting on | live |
| 63 | teatest asserts on the **final model and the files**, never on a frame | 40 still holds. teatest buys a real program, not a new thing to assert on | live |
| 64 | **One golden per screen**, colour stripped, and none of them assert behaviour | Narrows 40 rather than reversing it: goldens catch a lost border, a column that stopped aligning, a footer that overflowed into `…` — none of which a behaviour test can see | live |
| 65 | The F suite builds the binary itself | The `bin/tray` wrapper that used to do it went with super-utils (52) | live |

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
| 84 | The footer **wraps**; it does not truncate | `bubbles/help` cuts the line with an ellipsis, which silently hides whichever keys sort last — on an eighty-column terminal that was most of them. Wrapping costs one row and drops nothing | live |
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
| 86b | Help column widths are **measured from the labels**, not chosen | Hand-picked widths broke three times running, and a test now asserts the dialog fits in both directions with nothing truncated — goldens record what happened, they do not object to it | live |
| 86e | **Any key dismisses it**, and the key is spent doing so | You should never have to work out which key closes a thing that is in your way. `ctrl+c` still quits | live |
| 86n | On a terminal too small for it, the **diagram is what goes** | Something has to give, and losing the keymap or clipping mid-sentence would both be worse. It is the decorative half | live |
| 86m | `?` takes **the whole screen** | As an overlay it grew a frame but never grew its contents, so it read the same size however big the terminal was. Full screen means the prose reflows, the boxes spread, and there is no ceiling to design the content against — and any key still goes back, so it costs nothing to open | live |
| 86k | **Progressive disclosure**: prose, then the picture, then the keymap under a rule | A newcomer who meets twenty letters first has learnt nothing. A test asserts the order, because it is the kind of thing an edit quietly reverses | live |

## Rewriting a line

| # | Decision | Why | Status |
|---|---|---|---|
| 87 | `retake` is called **`rewrite`** | `retake` named the mechanism — the inverse of `take` — not the thing you wanted to do. `r` stays: it is where the vim hand already is | live |
| 87a | The old names are **gone**, not deprecated | Nothing has ever used them. A stub that says "modify is now rewrite" is a name kept alive to announce it is dead. `tray 1 modify pri:M` now parses `modify` as a filter and prints an empty report — but so does any word that is not a verb, so this is the general shape of the CLI rather than a hole this rename dug | live |
| 87b | **`modify` is gone; `rewrite` is the verb** | Two CLI names for one TUI action. TUI-first: the interface names the concept and the CLI takes the same word. Costs Taskwarrior's `modify` spelling — familiar, but verb names were never part of the import contract that 4 protects | live |
| 88 | A garage rewrite edits **the words alone** | Setting a priority in the garage is wanting a tray task, and `take` is how you say that. Same rule 33 already applies to `a`dd, now applied to `r` | live |
| 88a | It **keeps** whatever the line already carries | A line handed back from the tray arrives with a priority (68). Not offering the field is different from clearing it, and the form only ever writes what you touched | live |
| 88b | A garage batch rewrite is **refused** | The words are all there is, and one name for many is never the intent (25) — so a batch has nothing left to change. Better to say so than open a form with no fields | live |
| 88c | The **CLI does not enforce 88**; `rewrite` and `modify` will set a priority on a garage line | Your call. 19 says the TUI is the product and the CLI is the agent surface: the interface teaches a habit, the CLI stays the exact scriptable thing it is documented as. `dump +infra` already writes a tag to the garage, so "no structure here" was never quite true either | live |

## The checkbox

| # | Decision | Why | Status |
|---|---|---|---|
| 89 | The tray rows draw a **markdown checkbox**: `[ ]` `[x]` `[-]` | `- [x]` is the most understood task idiom there is, and the tray file already writes one — the screen was the only place the two disagreed about shape. The column width is measured from the box rather than assumed | live |
| 89a | The **garage keeps a one-character mark** | Its file writes `- a jotted line` with no box, and 2 says the file is the truth. Drawing one there would show something the line does not contain — and the absence is part of what says the garage asks nothing of you | live |
| 89b | Dropped is `[-]`, which markdown has no box for | Obsidian's spelling for cancelled, and it reads instantly beside `[x]`. Mirroring the file exactly would draw an empty box, leaving strikethrough — styling, which vanishes when piped — as the only difference from unfinished work | live |
| 89e | Ballot glyphs `☐ ☑ ☒` were tried and **reverted** | One column instead of three, and a third glyph for dropped that markdown lacks — but brackets look like the file they came from, and are not ambiguous-width under a CJK locale, where a terminal and `go-runewidth` can disagree and skew the table | live |
| 89c | State moved to **its own column**, beside selection | They shared one cell, so a row that was both selected and finished lost its dot. Found by giving the box somewhere to live | live |
| 89d | Purely visual: **`x` does not toggle** | A box invites ticking and unticking with one key, which would fold `R` into `x`. Worth considering, but changing what a key does is not the same change as changing how it looks | open |
