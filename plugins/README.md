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
  evidence/               plugin-written; the newest file is shown beside the plan
  log                     tray-written: stderr of the last run
```

The plugin's garage is the month named after it: rows it pulls land in
`layer=garage, month=<name>` with `source=<name>:<key>`, and climb like any other line.
A row you have already taken stays where it is and is updated in place.

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

## `sync apply`

stdin is the confirmed pushes: `{"push": [...]}`. Do them and answer:

```json
{"ok": ["ABC-9"], "failed": [{"key": "ABC-7", "error": "…"}]}
```

A plugin with nothing to push is never asked to apply.

## Environment and limits

Every run gets `TRAY_HOME` and `TRAY_PLUGIN_DIR`; a verb also gets `TRAY_LAYER` and
`TRAY_IDS`. `sync plan` and `sync apply` have ten minutes by default (`--timeout`);
a verb owns the screen and has none. A plugin runs as you; the exec bit is the consent.

## Trying one

`internal/sync/testdata/plugins/` holds four three-line plugins — `echo`, `fail`,
`slow`, `ask` — that the suite runs against. Copy `echo` into your plugins folder and
`tray sync` to see a plan; `tray sync --apply` to see it land.
