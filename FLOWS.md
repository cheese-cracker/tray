# Flows

**What must keep working.** Not a test list — a list of promises, each with the test
that holds it. `internal/flows` fails the build if a row here has no test, or a flow
test has no row here. Neither can rot quietly.

Two kinds:

| | Driven by | Asserts on | Lives in |
|---|---|---|---|
| **F** | the real binary, from a shell | file contents | `scripts/check-tray.sh` |
| **T** | the real bubbletea program, via `teatest` | final model **and** file contents | `internal/ui/flows_test.go` |

F flows survived a whole-language rewrite unchanged (decision 37), which is why they
are still bash in a repo that is otherwise Go. T flows are the ones that are only true
end to end: several keystrokes, a mode change in the middle, and a file on the far side.

Everything else — parsing, urgency, the store, single-key handling — is a unit test,
and does not belong here.

## F · the agent surface

`tray` piped is text, and an agent must never be handed a UI. **stdin is closed
throughout the F suite**, so a prompt appearing where an agent could hit it fails the
suite rather than hanging it.

| # | Must keep working | Held by |
|---|---|---|
| F1 | `dump` writes this month's file and the line survives verbatim | `F1 · capture` |
| F2 | A leading `to:` and `+tag` are the only things `dump` parses | `F2 · month + tag` |
| F3 | Arbitrary text is a valid garage line — half-sentences, `??`, colons mid-prose | `F3 · jottpad tolerance` |
| F4 | The tray reports in urgency order, not insertion order | `F4 · tray order is urgency, not insertion` |
| F5 | `take` is a transformation: structure is added, the source keeps an arrow | `F5 · take is a transformation` |
| F6 | `done` and `drop` strike through in place, dated, never moving the line | `F6 · done · drop` |
| F7 | `unload` is idempotent — running it twice does not duplicate the tray | `F7 · unload is idempotent` |
| F8 | `carryover` copies forward; the source month stays a record | `F8 · carryover copies forward` |
| F9 | Headings, prose and `*` bullets survive every operation byte-for-byte | `F9 · one document, two hands` |
| F10 | `export` is valid JSON in Taskwarrior's import shape | `F10 · export` |
| F11 | `+tag` and `due:` filters select the right lines | `F11 · filters` |
| F12 | `status --nag` is silent inside the month and speaks once it turns | `F12 · the nag` |
| F13 | The piped default view is plain bullets with ids, no attributes | `F13 · the default view is the print, with ids` |
| F14 | Ids are positional per report, so a hand reorder cannot desync them | `F14 · ids survive a hand reorder` |
| F15 | `find` reaches every layer and every month at once | `F15 · find is the rot signal` |
| F16 | Every report runs headless without ever prompting | `F16 · headless` |

## T · the terminal interface

| # | Must keep working | Held by |
|---|---|---|
| T1 | `take` moves the line onto the tray **and then** opens the form, prefilled | `TestFlowTakeOpensTheFormAndSaves` |
| T2 | With several marked, the form skips the title and still reaches every task | `TestFlowBatchRetakeSkipsTheTitle` |
| T3 | An action applies to the row a filter left visible, not to the pre-filter cursor | `TestFlowFilterThenActOnAFilteredRow` |
| T4 | **Marks survive a filter.** Filter, mark, filter again, act on all of them | `TestFlowMarksSurviveAFilter` |
| T5 | Tabs cycle at both ends rather than stopping | `TestFlowTabsCycleBothWays` |
| T6 | `>` copies forward and leaves an arrow on the source line | `TestFlowMoveToCopiesForwardWithAnArrow` |
| T7 | Handing back **revives** the garage line it came from — no copy, no orphan | `TestFlowHandBackRevivesTheGarageLine` |
| T8 | Adding in a garage tab asks for the words and writes nothing else | `TestFlowGarageAddAsksOnlyForATitle` |
| T9 | Adding on the tray takes the whole form: priority, due and tag all land | `TestFlowTrayAddTakesTheWholeForm` |
| T10 | `esc` clears an applied filter **before** it quits the program | `TestFlowEscClearsTheFilterBeforeItQuits` |
| T11 | `carryover` opens the same interface with the months as tabs, and no tray tab | `TestFlowSweepOpensTheMonthsAsTabs` |
| T12 | `?` opens and closes without disturbing the list underneath | `TestFlowHelpOverlayToggles` |
| T13 | A pasted title lands whole, and a pasted newline collapses rather than splitting the line | `TestFlowPasteIntoTheTitle` |

## Screens

`TestScreens` keeps one golden frame per distinct screen — tray, empty tray, marked
rows, garage, action menu, destination picker, retake form, help overlay, active
filter, a paged long list, and a narrow terminal that must truncate rather than wrap.

They are not flows and hold no behaviour. They catch the class of thing a behaviour
test cannot see: a frame that lost its border, a column that stopped aligning, a
footer that overflowed into `…`. Regenerate with:

```sh
go test ./internal/ui -run TestScreens -update
```

## Adding one

1. Write the test. `TestFlow…` in `internal/ui/flows_test.go`, or a `head_ "F… · …"`
   block in `scripts/check-tray.sh`.
2. Add the row here, with the test name in backticks in the last column.
3. `make check`. Missing either half fails `internal/flows`.

New assertions are mutation-checked before they count: break the code, watch the
assertion fail. Two have now passed for the wrong reason (decision 43).
