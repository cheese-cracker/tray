<div align="center">

# 🗒️ tray

**A task tracker in two layers.**
One for quick one-line thoughts, jotted the moment they occur.
One for the few tasks actually on the plate.

</div>

---

Most task apps ask you to file a thought the moment you have it — a project, a
priority, a due date — which is exactly when you know least about it. So you either
answer three questions you can't answer yet, or you don't write it down at all.

tray keeps those two jobs apart:

**🗑️ The garage** — for quick one-line task thoughts, filed under whatever month
feels right. Nothing else is asked: a half-sentence is a valid entry.

**🍽️ The tray** — for the three to seven tasks actually picked up and on the plate.

Moving a line from one to the other is the one moment it gets organised, and only the
few that graduate ever need it.

```sh
go install github.com/cheese-cracker/tray/cmd/tray@latest

tray init                      # creates ~/task-garage/
tray dump that config thing    # capture, zero ceremony
tray                           # open it
```

## 🎯 What it's for

- **🖥️ One tool, two interfaces.** A full-screen TUI for you; a plain-text CLI for your
  agents. Same files, same rules, neither one a second-class citizen.
- **📅 Structure only when it helps.** A line starts as a scribble and becomes a real
  task — priority, due date, tags — at the moment you pick it up. Never before.
- **🔌 Plays with what you already use.** Calendars, trackers, other task tools, via
  plugins. *Not built yet — see [ROADMAP.md](ROADMAP.md).*

## The interface

Bare `tray` on a terminal opens it.

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
 ↑↓ move · space select · tab switch · enter act · a add · v show done
 / filter · ? help · q quit
```

**Two tabs, day to day:** what you're doing, and what you dumped this month.

| | |
|---|---|
| `↑` `↓` | move — `j` `k` also work |
| `tab` | switch layer, cycling at either end. `⇧tab` goes back |
| `space` | select. Actions apply to your selection, or to the row under the cursor |
| `enter` | the action menu — take, retake, done, hand back, move, delete |
| `a` | add — a bare line in the garage, the full form on the tray |
| `t` | take a garage line onto the tray, and give it structure |
| `/` | filter · `v` show what you finished · `?` help · `q` quit |

Press **`?`** for a full-screen explainer: what the two layers are, and every key.

## Features

- 🗂️ **Two layers** — a garage that asks nothing of you, a tray that expects structure.
- 📝 **Plain markdown**, one task per line, in `~/task-garage/`. Edit it in any editor;
  tray preserves anything it doesn't recognise, byte for byte.
- 🗓️ **A garage per month** — dump into November in August, if that's when you'll do it.
- 🔁 **A month-turn sweep** — `tray carryover` opens the months as tabs so you can triage
  what's left before it rolls forward.
- ♻️ **Nothing is ever deleted.** Finishing strikes a line through in place; `v` shows
  them, `R` restores one you finished by accident.
- 🔍 **`/` fuzzy filter** over text and tags, and `tray find` across every month at once —
  a line that keeps reappearing is a rot signal you get for free.
- ✍️ **One form to restructure**, every field prefilled, so only what you touch changes.
- 📊 **Taskwarrior's urgency formula and field names**, so `tray export | task import`
  just works.
- 🖥️ **`tray head` for your shell profile** — the top few tasks on every new terminal,
  and completely silent when the tray is empty.
- 📦 **One static binary.** Nothing needed at runtime.

## The files

```
~/task-garage/          $TRAY_HOME overrides this
  tray.md               the worklist
  2026-08.md            this month's garage
  someday.md            explicitly undated
```

```markdown
# tray.md
- [ ] Rotate the api keys priority:H due:2026-08-12 entry:2026-08-07
- [x] ~~Renew the passport~~ priority:H done:2026-08-06

# 2026-08.md
- ?? that config thing — does it even matter now
- add metrics to the worker +infra
- Rotate the api keys priority:H → tray
```

The files are the truth and you are meant to edit them. Attributes are read off the
**end** of a line and only for known keys, so a colon mid-sentence survives — which is
what lets the garage hold prose. `→ tray` marks a line whose live copy moved elsewhere;
the line itself stays as history.

<details>
<summary><b>🤖 The CLI — the surface for agents</b></summary>

<br>

Piped, `tray` never opens a UI and never prompts, so an agent can drive every part of
it. This is the same tool and the same files — just the half you don't have to look at.

### Capture and create

| | |
|---|---|
| `tray dump <text>` | A line in this month's garage. **The tail is literal** — colons, dashes and half-sentences all survive. |
| `tray dump to:2026-11 +infra <text>` | A leading `to:` and `+tag` are the only things parsed. |
| `tray add <desc> pri:H due:2026-08-12 +infra` | Straight onto the tray, for something already live. |

> **Quote anything the shell would eat.** `tray dump ?? does this matter` fails in zsh —
> `??` is a glob. Use `tray dump '?? does this matter'`. Same for `!` and `*`.

### Moving between layers

| | |
|---|---|
| `tray 3 take [pri:H +infra]` | Garage → tray. Where a jotted pointer becomes a real task. |
| `tray 2 retake` | Restructure something already on the tray. |
| `tray unload --to 2026-09` | Hand the whole tray back to a month. **The month is never guessed** — bare `tray unload` picks it on a terminal and errors when piped. Finished items land struck through; open ones keep what the tray gave them. |
| `tray 2 unload --to 2026-09` | One item. |

### Finishing

| | |
|---|---|
| `tray 1 done` · `tray 3 drop` | Struck through in place, dated. Never moved, never deleted. |
| `tray 2,5-7 done` | Ranges, like Taskwarrior. |
| `tray 2 restore` | Says it wasn't finished after all. Ids resolve against `tray list --all`. |

### Editing

| | |
|---|---|
| `tray 2 modify pri:M +blocked -infra` | Exact and scriptable. What agents use. |
| `tray 2 edit <new text>` | Rewrite one line's text, attributes untouched. |
| `tray edit` · `tray garage edit` | Open the file in `$EDITOR`. |

### Reading

| | |
|---|---|
| `tray` | Grouped bullets with ids when piped. |
| `tray list` · `tray list --all` | The dense table; `--all` includes `✓` done and `✗` dropped. |
| `tray garage list` | This month's jottpad. |
| `tray +infra list` · `tray due:2026-08-12 list` | Filters. |
| `tray find <text>` | Every layer, every month at once. |
| `tray print` | Plain `- [ ]` bullets grouped by tag, for a journal. |
| `tray export` | JSON in Taskwarrior's import shape. |
| `tray head [n]` | The top few, compactly. Silent on an empty tray. |
| `tray status` | Where you stand, and any earlier month still holding live lines. |

### The month turn

Two commands, in order — `carryover` is month → month and never touches your tray:

```sh
tray unload --to 2026-08                 # close the tray out into the month that ended
tray carryover --run --month 2026-08     # its leftovers move to September
```

**The month is never inferred.** Sweeping August into September is the same job whether
you do it on the 30th, when August is current, or on the 10th, when it is previous — so
`--run` requires `--month`, and `tray status` prints the exact line to run.

Carryover is **copy-forward**: the source line stays, annotated `→ 2026-09`, which is
what makes month files a record and `tray find` a rot detector. A due date that has
already passed is not carried.

### Field reference

| Field | Meaning |
|---|---|
| `priority:` | `H` / `M` / `L`. `pri:` also works |
| `due:` | `YYYY-MM-DD` on disk; shown with the weekday |
| `entry:` | created, feeds the age term in urgency |
| `from:` | which garage month it graduated from |
| `done:` / `dropped:` | terminal, with the date |
| `+tag` | `#tag` is read too, `+tag` is written |
| `→ 2026-09` / `→ tray` | this line's live copy moved elsewhere; the line itself is history |

Ids are positional and computed per report, never stored, so reordering a file by hand
can't desync anything.

</details>

## In your shell

`tray head` prints the top of your tray and **nothing at all** when it's empty, so a
clear day costs a fresh terminal no lines.

```zsh
# ~/.zshrc
[[ -o interactive && -t 1 ]] && command -v tray >/dev/null && tray head
```

```
╭─ tray ───────────────────────────────────────────╮
│ H  Rotate the api keys                   3d over │
│ H  Book the flights                          Mon │
│ M  Review the deploy checklist           3d over │
╰──────────────────────────────────────────────────╯
```

---

[FLOWS.md](FLOWS.md) is what must keep working, and the test that holds each promise.
[DECISIONS.md](DECISIONS.md) is why things are the way they are.
[ROADMAP.md](ROADMAP.md) is what's next.
