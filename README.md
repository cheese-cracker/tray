<div align="center">

# tray

**Two layers of markdown. Dump anything into the garage; take a few things onto the tray.**

</div>

---

Most task tools make you classify at the moment you have the least information — the moment you think of the thing. `tray` splits that in two:

| | **garage** — the jottpad | **tray** — the worklist |
|---|---|---|
| Voice | pointers to yourself | tasks for a doer |
| Structure | none required, any line is valid | priority, due, tags |
| Volume | everything, all month | three to seven things |
| Example | `?? that config thing — does it even matter` | `Rotate the api keys priority:H due:2026-08-12 +infra` |

**Structure is added at review time, not capture time.** `take` is the transformation between the layers — the one moment you pay for schema, once, for the handful of things that graduate.

## Install

```sh
go install github.com/cheese-cracker/tray/cmd/tray@latest
```

Or from a clone: `make install`. One static binary, nothing needed at runtime.

## Quick start

```sh
tray init                      # creates ~/task-garage/
tray dump that config thing    # capture, zero ceremony
tray                           # the TUI
```

## The three ways in

| | | |
|---|---|---|
| **Something occurs to you** | `tray dump <text>` · or `a` in a garage tab | Nothing else is asked. A line in this month's garage, verbatim |
| **Something is already live** | `tray add <desc> pri:H due:2026-08-12` · or `a` on the tray | Straight onto the tray. **Structure is expected here** — `tray add` names what's missing if you leave it out |
| **Promote what you jotted** | `tray 3 take` · or `t` in a garage tab | Garage → tray. In the TUI this opens the retake form, because taking *is* the moment structure gets paid for |

Everything else is reading, finishing, or moving things between those two layers.

> **Quote anything the shell would eat.** `tray dump ?? does this matter` fails in zsh —
> `??` is a glob. Use `tray dump '?? does this matter'`. Same for `!` and `*`.

## The verbs

### Capture and create

| | |
|---|---|
| `tray dump <text>` | Put a line in this month's garage. **The tail is literal** — colons, dashes and half-sentences all survive. |
| `tray dump to:2026-11 +infra <text>` | A leading `to:` and `+tag` are the only things parsed. |
| `tray dump` | With no text, says so. Use `a` in the TUI to type one. |
| `tray add <desc> pri:H due:2026-08-12 +infra` | Create straight on the tray, for something already live. Taskwarrior's verb and grammar. |

### Moving between layers

| | |
|---|---|
| `tray 3 take` | Garage → tray. This is where a jotted pointer becomes a real task. |
| `tray 3 take pri:H +infra` | Same, without the prompts. |
| `tray 2 retake` | Restructure something already on the tray. |
| `tray unload --to 2026-09` | Hand the whole tray back to a month's garage. Runnable any time, not just at the month turn. **The month is never guessed** — bare `tray unload` picks it on a terminal and is an error when piped. Done items land struck through; open ones keep what the tray gave them, so taking them again is free. |
| `tray 2 unload --to 2026-09` | One item. |

### Finishing

| | |
|---|---|
| `tray 1 done` | Struck through in place, dated. Never moved, never deleted. |
| `tray 2 restore` | Says it wasn't finished after all. Ids resolve against `tray list --all`, which is the view you read them from. |
| `tray 3 drop` | Also terminal, but abandoned rather than finished. |
| `tray 2,5-7 done` | Ranges, like Taskwarrior. |

### Editing

| | |
|---|---|
| `tray 2 modify pri:M +blocked -infra` | Exact and scriptable. What agents use. |
| `tray 2 edit <new text>` | Rewrite one line's text, attributes untouched. |
| `tray edit` | Open `tray.md` in `$EDITOR`. `tray garage edit` opens the month. |

### Reading

| | |
|---|---|
| `tray` | The TUI on a terminal; grouped bullets with ids when piped. |
| `tray list` | The dense table — urgency, priority, due. |
| `tray garage list` | This month's jottpad. |
| `tray +infra list` · `tray due:2026-08-12 list` | Filters. |
| `tray find <text>` | Every layer, every month at once. **A line that turns up in four months is a rot signal** — you get it free, with no counter to maintain. |
| `tray print` | Plain `- [ ]` bullets grouped by tag, for pasting into a journal. No priority, no dates. |
| `tray export` | JSON in Taskwarrior's import shape: `tray export \| task import` works. |
| `tray head [n]` | The top few, compactly — for a shell profile. **Silent on an empty tray.** |
| `tray list --all` | The finished lines too, `✓` done and `✗` dropped. Works on either layer. |
| `tray status` | Where you stand, and any earlier month still holding live lines. |

### The month turn

| | |
|---|---|
| `tray carryover` | The sweep, on a terminal: four month tabs, described below. |
| `tray carryover --run --month 2026-08` | Headless. Every live line in that month is copied into the next one. |
| `tray carryover --draft --month 2026-08` | Same, then opens the target so you can delete what's dead. |

**Carryover is month → month and nothing else.** It does not touch your tray — `unload`
is its own ritual, run first and by name. At a turn that is two commands:

```sh
tray unload --to 2026-08                 # close the tray out into the month that ended
tray carryover --run --month 2026-08     # its leftovers move to September
```

**The month is never inferred.** "The closing month" is not a fact about the calendar:
sweeping August into September is the same job whether you do it on the 30th, when
August is the current month, or on the 10th, when it is the previous one. So `--run`
requires `--month`, and `tray status` prints the exact line to run.

Carryover is **copy-forward**: the source line stays where it was, annotated `→ 2026-09`. Month files are immutable records of what you were considering, which is what makes `tray find` a rot detector.

A **due date that has already passed is not carried**. Carrying a line forward is
admitting the date didn't hold; keeping it would mean every re-take starts overdue.
The source line keeps it, so the record is intact.

Dropped lines are struck through in place. Nothing is ever deleted.

## The TUI

Bare `tray` on a terminal opens it; piped, `tray` stays text, so an agent is never
handed a UI. Built with [bubbletea](https://github.com/charmbracelet/bubbletea),
[bubbles](https://github.com/charmbracelet/bubbles) and
[lipgloss](https://github.com/charmbracelet/lipgloss).

```
╭────────╮╭───────────────────╮
│  tray  ││  garage · August  │
│        └┴───────────────────┴────────────────────────────────────────┤
│                                                                      │
│     task                         urg   pri  due         tags         │
│   ● Rotate the api keys          17.1  H    2026-08-12 Wed  +infra   │
│  ▸  Book the flights             7.3   L    2026-08-20 Thu           │
│     Review the deploy checklist  4.7   M                +infra       │
│                                                                      │
└──────────────────────────────────────────────────────────────────────┘
 ↑↓ move · space select · ←→ tab · enter act · a add · v show done · / filter
 ? help · q quit
```

| | |
|---|---|
| `↑` `↓` | move — `j` `k` also work |
| `←` `→` · `tab` `⇧tab` | switch tab — cycles round at either end. `h` `l` also work |
| `space` | select. Every action applies to the selection, or to the row under the cursor |
| `enter` | the action menu |
| `a` `n` | add — a title alone in a garage tab, the whole form on the tray |
| `/` | filter. Fuzzy, over the text and the tags; `esc` clears it |
| `v` | show what you finished, `✓` done and `✗` dropped, struck through |
| `?` | the help page — what the two layers are, and every key |
| `q` `esc` | quit |

**The footer names the arrows only.** `h j k l` work and are written down in `?`; a
footer that lists two ways to do one thing teaches neither.

**Two tabs, day to day**: what you're doing, and what you dumped this month. Someday
and other months are still reachable through `>`; they just don't earn standing room.

**`tray carryover` opens the same interface with the months as tabs** — the previous
month, this one, the next, and someday — because that ritual is the one time the
months matter more than the tray. It opens on **this** month, because which month is
"closing" depends on the day you happen to sweep.

The sweep is **triage only: quitting it carries nothing.** Mark what deserves to
survive and `>` it where it belongs; `x` and `D` finish or drop what doesn't. When
you're done, `tray carryover --run --month <m>` moves everything still live.

### Acting

`enter` opens a menu over the selection, and each row's letter also works straight
from the list — so the menu teaches itself out of a job.

| On the tray | | On a garage month | |
|---|---|---|---|
| `x` | done | `t` | take — onto the tray, then the form |
| `d` | hand back to this month | `>` | move to another month |
| `>` | move to | `x` | done, without ever reaching the tray |
| `r` | retake | `r` | retake |
| `D` | delete | `D` | delete |
| `R` | restore — **only offered on a finished row** | `R` | restore |

`>` **is the month sweep.** Open last month, mark what matters, and move it — to the
tray, to next month, or to someday. Anything you leave is carried forward as usual.

### retake — one form, everything prefilled

```
  retake

  title     Rotate the api keys
  priority  ( ) H  (•) M  ( ) L
  due       2026-08-12 Wed
  tag       infra

  ↑↓ field · h l choose · enter save · esc cancel
```

**Every field starts at its current value, so only what you touch changes.** No
sequence, no question you have to answer to reach the one you wanted.

- `↑↓` moves between fields; `h`/`l` move the priority radio; typing edits the rest
- **priority is `H · M · L` with no "none"** — an unset task reads as medium, and a
  new tray task lands there
- on `due`, `←`/`→` shift by a day
- **tags are typed**; the ones already in use are shown as a hint, not a menu
- with several marked the title is skipped — one name for many is never the intent

Every decision behind this, including the ones since reversed, is a row in
[DECISIONS.md](DECISIONS.md).

`u` undo is **not implemented** — see ROADMAP.md. Copy-forward keeps everything
recoverable by hand meanwhile.

Bare `tray unload` on a terminal opens a month picker rather than guessing. Piped, it
is an error: the CLI is the agent surface and must never start a conversation.

### Filtering

`/` filters the rows in front of you, fuzzily, over the text and the tags — never over
`priority:H`, which you never typed. It is `bubbles/list` doing the work in process:
no `fzf`, nothing handed the terminal, nothing to flicker.

An applied filter says what it hid (`/infra — 3 of 17 · esc clears`), because a table
quietly showing three of seventeen rows is a table you will misread. **A mark survives
a filter**: hiding a row is not the same as deselecting it, so you can filter, mark,
filter again, and act on everything you marked.

## The files

```
~/task-garage/          $TRAY_HOME overrides this
  tray.md               the worklist
  2026-08.md            this month's garage
  2026-09.md
  someday.md            explicitly undated
```

```markdown
# tray.md
- [ ] Rotate the api keys priority:H due:2026-08-12 entry:2026-08-07
- [x] ~~Renew the passport~~ priority:H done:2026-08-06

# 2026-08.md
- ?? that config thing — does it even matter now
- add metrics to the worker +infra
- add retries to the sync job → 2026-09
- Rotate the api keys priority:H → tray
```

| Field | Meaning |
|---|---|
| `priority:` | `H` / `M` / `L`. `pri:` also works |
| `due:` | `YYYY-MM-DD` on disk; shown with the weekday |
| `entry:` | created, feeds the age term in urgency |
| `from:` | which garage month it graduated from |
| `done:` / `dropped:` | terminal, with the date |
| `+tag` | `#tag` is read too, `+tag` is written |
| `→ 2026-09` / `→ tray` | this line's live copy moved elsewhere; the line itself is history |

**Attributes are read off the end of a line only, and only for known keys.** A colon in the middle of a sentence is never touched — that's what lets the garage hold prose.

### One document, two hands

The files are the truth, and you are expected to edit them directly. Anything `tray` doesn't recognise — headings, paragraphs, `*` bullets, a half-typed line — is preserved byte-for-byte through every operation. Two flow tests exist purely to hold that promise.

Ids are positional and computed per report, never stored, so reordering `tray.md` by hand can't desync anything.

### Urgency

Taskwarrior's polynomial, over the fields we keep, so the numbers stay comparable after an import:

```
urgency = 6.0·priority + 12.0·due_proximity + 1.0·tags − 2.0·age
```

The Eisenhower view reads `urgent` as due within seven days and `important` as priority `H` or `M`.

### In your shell

`tray head` is built for a profile: it prints the top of the tray and **nothing at all**
when the tray is empty, so a fresh terminal costs nothing on a clear day.

```zsh
# ~/.zshrc
[[ -o interactive && -t 1 ]] && command -v tray >/dev/null && tray head
```

```
╭─ tray ───────────────────────────────────────────────────╮
│ H  I'll discuss a deposit with Vishal.           3d over │
│ H  Wire the transfer money to carta                  Mon │
│ M  Reach out to the people mentioned by Prannay  3d over │
╰──────────────────────────────────────────────────────────╯
```

Rows are tinted by priority — H red, M amber, L blue — with overdue in red and every
other date faint. Piped, it is the plain box above and nothing else, so `tray head` is
still safe to read from a script.

No ids and no urgency figures — you can't act on an id you didn't ask for, and the
number means nothing at a glance. Dates are relative, because a task due last Monday
shown as `Mon` reads as upcoming, which is the one thing a header must not get wrong.

## Requirements

**Nothing at runtime** — one static binary. Building it needs Go 1.24 or newer.

## Tests

```sh
make check          # fmt · vet · go test · flows
```

`scripts/check-tray.sh` drives the real binary through sixteen user flows against a
sandboxed `$TRAY_HOME`, asserting on file contents. **stdin is closed throughout**, so a
prompt appearing where an agent could hit it fails the suite rather than hanging it.

`go test ./...` covers the core, the store and the TUI. Every flow that must keep working
is a row in [FLOWS.md](FLOWS.md), and every row names the test that holds it.
