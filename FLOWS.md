# Flows

**What must keep working.** Not a test list — a list of promises, each with the test
that holds it. `internal/flows` fails the build if a row here has no test, or a flow
test has no row here. Neither can rot quietly.

Two kinds:

| | Driven by | Asserts on | Lives in |
|---|---|---|---|
| **F** | the real binary, from a shell | what `list --json` and `export` report | `scripts/check-tray.sh` |
| **T** | the real bubbletea program, via `teatest` | final model **and** the store | `internal/ui/*_test.go` |

F flows survived a whole-language rewrite and then a whole-store rewrite: they read
the binary's own output and nothing else, which is why they are still bash in a repo
that is otherwise Go. T flows are the ones that are only true end to end: several
keystrokes, a mode change in the middle, and a row on the far side — read back from
the store, never from a frame.

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
| F23 | `plugin` lists what is installed — the garage a plugin keeps, as rows, the verbs under its `actions/`, the `on-launch` marker, how its last run went — counts a file without the exec bit as half an install; `plugin run` prints one plan and lands nothing; `plugin set` writes only the keys the example names | `F23 · plugin lists what is installed` |
| F24 | Ids are permanent: erasing or finishing a neighbour renumbers nothing, and an erased id is never handed out again | `F24 · ids are permanent` |
| F25 | A task with `wait:` lies in the garage of its month, reads as waiting, and the first `sync` on or after its day lifts it onto the tray with what it carried — once | `F25 · a waiting task lies in the garage until its day` |
| F26 | A template keeps one live child; `sync` materializes the next only once the last is done, due one period on, skipping missed periods; an unknown period is refused and `done` on a template ends it | `F26 · recurrence serves each period once` |
| F28 | `sync` prints the summary and every plugin's plan — adds, pushes, evidence — and lands nothing without `--apply`; `--json` carries the same | `F28 · sync prints a plan and lands nothing without --apply` |
| F29 | `--apply` lands a plugin's plan in its garage in one transaction — a row the store refuses lands nothing — and hands the confirmed pushes to the plugin | `F29 · apply lands a plugin's plan whole or not at all` |
| F30 | A plugin that exits non-zero is named with its first stderr line and lands nothing; exit 2 is a question; the others plan and land regardless; `plugin` and `status` say who failed | `F30 · a failing plugin fails alone` |
| F31 | A plugin that outlives `--timeout` is cut off and named while the others' plans still print | `F31 · a slow plugin times out alone` |
| F27 | `export` and `import` round-trip Taskwarrior JSON and todo.txt field for field; a project comes in as a tag, a uuid updates the row it names, and a line with no structure lands in this month's garage | `F27 · export and import round-trip` |
| F32 | `context` is the grouped report with ids and every note under its task, narrowed by ids or a filter | `F32 · context is the report with its notes` |
| F33 | `import --format md` brings a markdown home in whole — layers by filename, notes, struck lines finished, `→` lines skipped — and twice adds nothing | `F33 · import migrates the markdown home` |
| F34 | One `sync` runs every installed plugin — the good plan prints, a failure is named, a slow one is cut off — and one `--apply --plugin` lands exactly that plan, whole, and hands its push back | `F34 · one sync runs every plugin and lands one plan whole` |
| F35 | `tray.md` and `garage.md` beside the database hold one `- (id) words` bullet per open task, under month headings, rewritten after every write and left alone by a read | `F35 · the mirror is rewritten after every write` |
| F36 | `sync` reads `garage.md` back: a bullet with no id is a new line in the month it sits under, changed words rename the task, an unknown id and a missing bullet change nothing, and `tray.md` is never read | `F36 · sync reads garage.md back: new lines and renames, nothing else` |
| F37 | `~/.config/tray/config.yaml` (or `TRAY_CONFIG`) names the store: `db.url` pointing at another file writes there and leaves the home's `tray.db` alone; `tray config` prints the path and the effective values with secrets cut to their last four characters, says which came from the environment, and a malformed file is an error that names it | `F37 · the config file names the store, and tray config masks its secrets` |

## T · the terminal interface

| # | Must keep working | Held by |
|---|---|---|
| T1 | `take` moves the row onto the tray, remembers the month it left, **and then** opens the form, prefilled | `TestFlowTakeOpensTheFormAndSaves` |
| T2 | With several marked, the form skips the title and still reaches every task | `TestFlowBatchRewriteSkipsTheTitle` |
| T3 | An action applies to the row a filter left visible, not to the pre-filter cursor | `TestFlowFilterThenActOnAFilteredRow` |
| T4 | **Marks survive a filter.** Filter, mark, filter again, act on all of them | `TestFlowMarksSurviveAFilter` |
| T5 | Tabs cycle at both ends rather than stopping | `TestFlowTabsCycleBothWays` |
| T6 | `>` moves the row to the month you chose — one row, nothing copied, nothing left behind | `TestFlowMoveToMovesTheRow` |
| T7 | Handing back moves the row home to the month it came from — no copy, no orphan — and it keeps what the tray added | `TestFlowHandBackMovesTheRowHome` |
| T8 | Adding in a garage tab asks for the words and writes nothing else | `TestFlowGarageAddAsksOnlyForATitle` |
| T9 | Adding on the tray takes the whole form: priority, due and tag all land | `TestFlowTrayAddTakesTheWholeForm` |
| T10 | `esc` clears an applied filter **before** it quits the program | `TestFlowEscClearsTheFilterBeforeItQuits` |
| T11 | `carryover` opens the months it is about — the named one, this one, a forward slot, someday — no tray tab, focused on this month, and `>` reaches every one of them | `TestFlowSweepOpensTheMonthsAsTabs` |
| T12 | `?` opens and closes without disturbing the list underneath | `TestFlowHelpOverlayToggles` |
| T13 | A pasted title lands whole, and a pasted newline collapses rather than splitting the words | `TestFlowPasteIntoTheTitle` |
| T14 | A finished task is hidden until `v`, and `R` says it wasn't finished after all | `TestFlowViewDoneThenRestore` |
| T15 | `v` lists everything on the layer, live rows first, and offers restore and erase alone; `a` writes nothing there | `TestFlowReviewShowsEverythingAndOffersTheRareVerbs` |
| T16 | A garage rewrite edits the words alone, and keeps whatever the row already carries | `TestFlowGarageRewriteIsTextOnly` |
| T17 | A garage rewrite refuses a batch — there is nothing left for it to change | `TestFlowGarageRewriteRefusesABatch` |
| T18 | `E` removes a row outright and names it in the status, and is reachable only in review mode | `TestFlowEraseRemovesTheLineAndSaysWhatWent` |
| T19 | `n` opens the note alone on either layer, saves it on the row, and the row shows `≡` | `TestFlowNoteIsTheIndentedLinesUnderATask` |
| T20 | The id column reads the permanent id: erase the row above and the one below keeps its number | `TestFlowTheIdColumnReadsThePermanentId` |
| T21 | The CLI and the interface share one store: what `dump` writes the interface shows, and what the interface takes, finishes and hands back `list --json` reads by the same ids | `TestFlowTheCLIAndTheTUIShareOneStore` |
| T22 | `S` is the sync event by hand: a bullet added to `garage.md` lands in the garage, a renamed one is renamed, a removed one survives, `tray.md` edits are ignored, and both files are rewritten | `TestFlowSyncReadsTheGarageFileBack` |

## Adding one

1. Write the test. `TestFlow…` in `internal/ui/flows_test.go`, or a `head_ "F… · …"`
   block in `scripts/check-tray.sh`.
2. Add the row here, with the test name in backticks in the last column.
3. `make check`. Missing either half fails `internal/flows`.

New assertions are mutation-checked before they count: break the code, watch the
assertion fail. Two have now passed for the wrong reason (decision 43).
