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
| `tray unload` | Hand the whole tray back to this month's garage. Runnable any time, not just at the month turn. Done items land struck through; open ones keep their attributes, so taking them again is free. |
| `tray 2 unload --to 2026-09` | One item, to a month you choose. |

### Finishing

| | |
|---|---|
| `tray 1 done` | Struck through in place, dated. Never moved, never deleted. |
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
| `tray status --nag` | One line if a month has turned with things left in it. Silent otherwise. |

### The month turn

| | |
|---|---|
| `tray carryover --all` | Every live line in the closing month is copied into the next one. |
| `tray carryover --draft` | Same, then opens next month so you can delete what's dead. |
| `tray carryover --month 2026-06` | A specific month. |

Carryover is **copy-forward**: the source line stays where it was, annotated `→ 2026-09`. Month files are immutable records of what you were considering, which is what makes `tray find` a rot detector.

Dropped lines are struck through in place. Nothing is ever deleted.

## The TUI

Bare `tray` on a terminal opens it; piped, `tray` stays text, so an agent is never
handed a UI. Built with [bubbletea](https://github.com/charmbracelet/bubbletea).

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
 j k move · h l tab · space mark · enter act · q quit
```

| | |
|---|---|
| `j` `k` `↑↓` | move |
| `h` `l` · `tab` `⇧tab` | switch tab — cycles round at either end |
| `space` | mark. Every action applies to the marks, or to the row under the cursor |
| `enter` | the action menu |
| `a` `n` | add — a title alone in a garage tab, the whole form on the tray |
| `q` `esc` | quit |

**Two tabs, day to day**: what you're doing, and what you dumped this month. Someday
and other months are still reachable through `>`; they just don't earn standing room.

**`tray carryover` opens the same interface with the months as tabs** — the closing
month, this one, and someday — because that ritual is the one time the months matter
more than the tray. Mark what deserves to survive, `>` it where it belongs, and
anything you leave is carried forward as usual.

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

`/` search, `u` undo and `?` help are **not implemented** — see ROADMAP.md, where
`/` is planned as a hand-off to `fzf` rather than a fuzzy matcher of our own.

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
