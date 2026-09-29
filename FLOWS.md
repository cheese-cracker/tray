# Flows

**What must keep working.** Not a test list — a list of promises, each with the test
that holds it. `internal/flows` fails the build if a row here has no test, or a flow
test has no row here. Neither can rot quietly.

Two kinds:

| | Driven by | Asserts on | Lives in |
|---|---|---|---|
| **F** | the real binary, from a shell | what `list --json` and `export` report | `scripts/check-tray.sh` |
| **G** | the real app, via `fyne.io/fyne/v2/test` | final model **and** the store | `internal/gui/flows_test.go` |

F flows survived a whole-language rewrite and then a whole-store rewrite: they read
the binary's own output and nothing else, which is why they are still bash in a repo
that is otherwise Go. G flows are the ones that are only true end to end: several
actions, a mode change in the middle, and a row on the far side.

Everything else — parsing, urgency, the store, single-key handling — is a unit test,
and does not belong here.

## F · the agent surface

`tray` piped is text, and an agent must never be handed a UI. **stdin is closed
throughout the F suite**, so a prompt appearing where an agent could hit it fails the
suite rather than hanging it.

| # | Must keep working | Held by |
|---|---|---|
| F1 | `dump` writes this month's garage and the words survive verbatim | `F1 · capture` |
| F2 | A leading `to:` and `+tag` are the only things `dump` parses | `F2 · month + tag` |
| F3 | Arbitrary text is a valid garage line — half-sentences, `??`, colons mid-prose | `F3 · jottpad tolerance` |
| F4 | The tray reports in urgency order, not insertion order | `F4 · tray order is urgency, not insertion` |
| F5 | `take` moves a row onto the tray with the structure you gave it, remembers the month it left, keeps its id, and never takes it twice | `F5 · take is a transformation` |
| F6 | `done` marks in place, dated, keeping what the row had; the default report hides it | `F6 · done strikes in place` |
| F7 | `unload` is idempotent — running it twice changes nothing | `F7 · unload is idempotent` |
| F8 | `carryover` moves a month's live rows to the next month, leaves the tray alone, and drops a due date that already passed | `F8 · carryover moves forward` |
| F10 | `export` is valid JSON in Taskwarrior's import shape, plus the id | `F10 · export` |
| F11 | `+tag` and `key:value` filters select the right rows | `F11 · filters` |
| F12 | `status` names any earlier month still holding live rows, and the command that clears it | `F12 · status names the month left behind` |
| F13 | The piped default view is plain bullets with ids, no attributes | `F13 · the default view is the print, with ids` |
| F15 | `find` reaches every layer and every month at once, finished rows only under `--all` | `F15 · find reaches every layer and month` |
| F16 | Every report runs headless without ever prompting | `F16 · headless` |
| F17 | `unload` brings a task home to the month it left — finished ones finished, open ones keeping what the tray gave them — and one task alone needs no `--to` | `F17 · unload brings the tray home whole` |
| F18 | Nothing is inferred headlessly: `carryover`, `unload` and `import` refuse to guess, and an unknown flag is an error rather than a silence | `F18 · nothing is inferred headlessly` |
| F19 | `head` prints the top few compactly, says nothing at all on an empty tray, and renders dates in the one format the rest of the tool uses | `F19 · head is the terminal header` |
| F20 | `restore` un-finishes the task you named and leaves no trace | `F20 · restore says a task was not finished after all` |
| F21 | `erase` removes a row outright — the one verb that does — leaves its neighbours alone, and reaches a finished row by the same id | `F21 · erase removes the line` |
| F22 | A note is the lines under a task: `--note` on `dump` and `add`, `tray <id> note <text>` replaces it whole, it exports as one Taskwarrior annotation | `F22 · note is the indented lines under a task` |
| F23 | `plugin` lists what is installed — the garage a plugin keeps, as rows, and the verbs under its `actions/` — counts a file without the exec bit as half an install, and refuses to sync | `F23 · plugin lists what is installed` |
| F24 | Ids are permanent: erasing or finishing a neighbour renumbers nothing, and an erased id is never handed out again | `F24 · ids are permanent` |
| F27 | `export` and `import` round-trip Taskwarrior JSON and todo.txt field for field; a project comes in as a tag, a uuid updates the row it names, and a line with no structure lands in this month's garage | `F27 · export and import round-trip` |
| F32 | `context` is the grouped report with ids and every note under its task, narrowed by ids or a filter | `F32 · context is the report with its notes` |
| F33 | `import --format md` brings a markdown home in whole — layers by filename, notes, struck lines finished, `→` lines skipped — and twice adds nothing | `F33 · import migrates the markdown home` |

## G · the app

The G suite arrives with the app. Until then this half of the table is empty, and
`internal/flows` treats a missing `internal/gui/flows_test.go` as zero rows.

## Adding one

1. Write the test. `TestFlow…` in `internal/gui/flows_test.go`, or a `head_ "F… · …"`
   block in `scripts/check-tray.sh`.
2. Add the row here, with the test name in backticks in the last column.
3. `make check`. Missing either half fails `internal/flows`.

New assertions are mutation-checked before they count: break the code, watch the
assertion fail. Two have now passed for the wrong reason (decision 43).
