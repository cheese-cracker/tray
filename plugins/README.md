# Plugins

A plugin is a folder under `$TRAY_HOME/plugins/`. The folder is the manifest: each file
below is a fact the plugin states by having it. Nothing is declared and nothing is
parsed. You run `sync`; the plugin does the rest.

```
$TRAY_HOME/plugins/<name>/
  sync                    executable. `sync plan` and `sync apply` (below). Present ⇒ the plugin keeps a garage
  actions/<verb>          executable → a row in the enter menu; gets the picked ids as arguments and TRAY_LAYER
  settings.example.json   {"url": "", "rules": "…"} — the keys the plugin wants filled; the app renders them as a form
  settings.json           written by the form or `tray plugin set`; tray never reads it
  on-launch               marker: run `sync plan` when the app opens too. Absent ⇒ manual only
  all-rows                marker: `sync plan` reads the whole store, not only your rows (below)
  health                  executable, optional: the probe `tray plugin check` runs (below)
  evidence/               plugin-written; the newest file is shown beside the plan
  log                     tray-written: stderr of the last run
```

The plugin's garage is the month named after it: rows it pulls land in
`layer=garage, month=<name>` with `source=<name>:<key>`, and climb like any other line.
A row you have already taken stays where it is and is updated in place.

**Local is the source of truth.** Whatever you bring back — a remote replica, a phone, a
board — lands through the reviewed diff and never overwrites a local row silently; your
pushes carry local state outward. Offline is therefore the normal case, not a failure:
tray keeps working, and your plan waits for the next sync that can reach you.

## Events and hooks

Nothing in tray runs on its own. It acts on two **events**:

| Event | Fired by | What answers it |
|---|---|---|
| `write` | any change to the store — the CLI's, the interface's, a plugin's landing | the mirror |
| `sync` | you: `tray sync`, `S` in the interface (`manual`); or an interface opening (`launch`) | the built-in hooks, then every plugin |

A **hook** is one thing that answers an event, and `internal/sync/hooks.go` is the whole
table, in the order they run:

| Hook | On | Does |
|---|---|---|
| `recur` | sync | gives every template its next child |
| `lift` | sync | moves a garage line whose day has come onto the tray |
| `garage.md` | sync | reads the garage mirror back — new bullets, renamed bullets |
| `mirror` | write, sync | rewrites `tray.md` and `garage.md` from the store |
| *a plugin* | sync | its `sync plan`, reviewed; `sync apply` when you land it |

The built-in hooks are tray's own data, so they land directly and report a phrase each
(`materialized 1 · lifted 0 · garage.md +2 ~1`). A plugin is a hook too, on the same
event — it runs after the built-ins, in its own process, and its plan is shown before
anything lands (T5). On `launch` only plugins that left an `on-launch` file run; the
built-ins always do. Adding a built-in is one row in the table; adding a plugin is a
folder — nothing is registered, and neither knows about the other.

## `sync plan`

Print what you see. stdin is the rows tray already holds for you, keyed the way you key
them:

```json
{"tasks": [{"key": "ABC-12", "text": "…", "done": "", "tags": ["work"], "priority": "H", "due": "2026-10-01"}]}
```

stdout is your plan:

```json
{"pull":  [{"key": "ABC-12", "text": "…", "done": null, "tags": ["work"], "due": "2026-10-01",
            "evidence": "evidence/2026-09-28T10-00.png"}],
 "push":  [{"key": "ABC-9", "set": {"done": "2026-09-28"}}],
 "evidence": "evidence/2026-09-28T10-00.png"}
```

- `key` is stable for the same item across runs, and every pulled row has one and a `text`.
- A field you leave out is one you did not report: `"done": null`, no `tags`, an empty
  `priority` or `due`. tray never clears what you could not see. `"done": ""` says open;
  a date says finished.
- `push` is what you would change on your side, read off tray's rows — a `done` tray
  has that the site does not. Nothing to push is `[]`.
- Zero rows is a valid plan. Never guess.
- Exit `2` with one line on stderr when you need the user — a login, a setting. Exit
  non-zero for anything else; the first stderr line is what tray shows, the rest is in
  `log`.

tray diffs `pull` against the rows you own: a new key is an add, a known key an update
on exactly the fields you reported differently, a key you stopped reporting is noted as
gone and left alone. Nothing lands until the user applies your plan, and then it lands
whole or not at all.

### Every row

A replica has to see everything to know what changed. Leave an `all-rows` file in your
folder and stdin carries the whole store — every layer, month and state, finished rows
too — each task keyed by its **id** and carrying every column:

```json
{"tasks": [{"key": "k79l", "id": "k79l", "layer": "tray", "text": "…", "done": "", "tags": ["work"],
            "priority": "H", "due": "2026-10-01", "wait": "", "recur": "", "until": "", "entry": "2026-09-28",
            "from_month": "2026-09", "note": "", "source": ""}]}
```

Your pushes may then name any task by its id; tray does nothing with a push but hand the
confirmed list back to you on `apply`. A pulled row keyed by an id tray holds is an
update of that row (reviewed, only the fields you report); any other key is a new line in
your garage with `source=<name>:<key>`. Nothing is ever *gone* for you — you own no rows.

## `sync apply`

stdin is the confirmed pushes: `{"push": [...]}`. Do them and answer:

```json
{"ok": ["ABC-9"], "failed": [{"key": "ABC-7", "error": "…"}]}
```

A plugin with nothing to push is never asked to apply.

## `health`, and what `tray plugin` shows

`health` is optional and takes no input. Exit **0**: well. Exit **2**: it needs you — write
one line to stderr saying what (`token expired`, `not signed in`). Exit **1**: failed.
`tray plugin check [name]` runs each probe with a ten-second limit and remembers the
verdict apart from the last sync run, so neither hides the other. A plugin without a
probe is reported as `no probe`, which is not a failure. The reference plugins — `gcal`
and `turso`, each with a probe — live in the `tray-plugins` repo, one folder each.

`tray plugin` is the health view, one row per folder:

```
NAME        STATE                          HOOKS                    SETTINGS  LAST
turso       ok                             manual · all-rows        ok        check 2026-09-29 12:01 — ok
gcal        never run                      verbs: schedule          —         —
web-linear  unconfigured                   launch · manual          missing   —
halfdone    half-installed                 —                        —         —
```

`state` reads the folder before the history: a folder none of whose files carries the
exec bit is `half-installed`; an example settings file with no `settings.json` beside it
is `unconfigured`; otherwise the last probe, else the last sync run, else `never run`.
`hooks` is what the plugin joins — `launch`, `manual`, `verbs: …`, `all-rows`. Nothing is
listed that is not a folder. Whatever a plugin needs to run — credentials, a token, a
model provider's key — lives in its own `settings.json`, named by its
`settings.example.json`; tray's own environment and config file hand nothing across.
`--json` gives the same rows to an agent.

## Environment and limits

Every run gets `TRAY_HOME` and `TRAY_PLUGIN_DIR`; a verb also gets `TRAY_LAYER` and
`TRAY_IDS` — the picked tasks' four-character ids, comma separated. `sync plan` and `sync apply` have ten minutes by default (`--timeout`);
a verb owns the screen and has none. A plugin runs as you; the exec bit is the consent.

## Trying one

`internal/sync/testdata/plugins/` holds four three-line plugins — `echo`, `fail`,
`slow`, `ask` — that the suite runs against. Copy `echo` into your plugins folder and
`tray sync` to see a plan; `tray sync --apply` to see it land.
