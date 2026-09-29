#!/usr/bin/env bash
# User-flow tests for tray. Drives the real binary against a sandboxed TRAY_HOME and
# reads the store back the way an agent would: `list --json` and `export`. stdin is
# closed throughout, so any prompt fails here rather than hanging.
set -u

ROOT=$(cd -P "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
BIN="$ROOT/build/tray"
fail=0

pass() { printf '  \033[32m✓\033[0m %s\n' "$1"; }
bad()  { printf '  \033[31m✗\033[0m %s\n' "$1"; fail=1; }
head_() { printf '\n\033[1m%s\033[0m\n' "$1"; }

# Every flow gets a clean home and a frozen today.
setup() {
  TRAY_HOME=$(mktemp -d)
  export TRAY_HOME
  export TRAY_TODAY=2026-08-07
  tray() { "$BIN" "$@" </dev/null; }
  tray init >/dev/null
}
teardown() { rm -rf "$TRAY_HOME"; }

# A layer as JSON, every state included. Rows are found by description: ids are
# permanent, so the words are the one thing that stays between typing and reading.
tray_json()   { tray list --all --json; }
garage_json() { tray garage --month "$1" list --all --json; }
field()   { jq -rc --arg t "$1" --arg f "$2" 'first(.[] | select(.description==$t)) | .[$f] // "null"'; }
note_of() { jq -r --arg t "$1" 'first(.[] | select(.description==$t)) | .annotations[0].description // "null"'; }
rows()    { jq -r --arg t "$1" '[.[] | select(.description==$t)] | length'; }
id_of()   { tray_json | field "$1" id; }
gid_of()  { garage_json "$1" | field "$2" id; }

# A build failure is a failure, not a skip: skipping would report "all flows pass"
# for a repo that doesn't compile. Only a missing toolchain is a legitimate skip.
if command -v go >/dev/null 2>&1; then
  (cd "$ROOT" && go build -o "$BIN" ./cmd/tray) || {
    printf '  \033[31m✗\033[0m tray does not build\n'
    exit 1
  }
elif [ ! -x "$BIN" ]; then
  echo "no go toolchain and no built binary — skipping"
  exit 0
fi

command -v jq >/dev/null 2>&1 || {
  printf '  \033[31m✗\033[0m jq is required: every assertion reads the store back through it\n'
  exit 1
}
valid_json() { jq -e . >/dev/null 2>&1; }

# macOS ships no timeout(1); coreutils installs it prefixed. Neither is required.
if command -v timeout >/dev/null 2>&1; then limit() { timeout 5 "$@"; }
elif command -v gtimeout >/dev/null 2>&1; then limit() { gtimeout 5 "$@"; }
else limit() { "$@"; }
fi

# --- F1 · init then dump ------------------------------------------------------
head_ "F1 · capture"
setup
tray init | grep -qF "$TRAY_HOME" && pass "init says where the data lives" || bad "init: $(tray init)"
out=$(tray dump add metrics to the worker)
[ "$(garage_json 2026-08 | rows 'add metrics to the worker')" = "1" ] \
  && pass "lands in this month's garage, verbatim" || bad "got: $(garage_json 2026-08)"
[ "$(garage_json 2026-08 | field 'add metrics to the worker' entry)" = "20260807T000000Z" ] \
  && pass "entry: stamped" || bad "no entry"
case $out in *2026-08*) pass "reports the month" ;; *) bad "unexpected: $out" ;; esac

# --- F2 · dump with month and tag --------------------------------------------
head_ "F2 · month + tag"
tray dump to:2026-11 +infra add metrics to the worker >/dev/null
[ "$(garage_json 2026-11 | field 'add metrics to the worker' tags)" = '["infra"]' ] \
  && pass "lands in 2026-11 with +infra" || bad "got: $(garage_json 2026-11)"

# --- F3 · arbitrary text is valid --------------------------------------------
head_ "F3 · jottpad tolerance"
tray dump '?? the billing page feels slow — worth a look: probably' >/dev/null
[ "$(garage_json 2026-08 | rows '?? the billing page feels slow — worth a look: probably')" = "1" ] \
  && pass "colons, dashes and ?? survive" || bad "prose was parsed"
teardown

# --- F4 · add, then urgency ordering -----------------------------------------
head_ "F4 · tray order is urgency, not insertion"
setup
tray add Low thing pri:L >/dev/null
tray add Urgent thing pri:H due:2026-08-08 >/dev/null
tray add Middle thing pri:M >/dev/null
first=$(tray list | sed -n '2p')
case $first in *"Urgent thing"*) pass "highest urgency first" ;; *) bad "got: $first" ;; esac
[ -n "$(id_of 'Urgent thing')" ] && pass "every row has an id" || bad "no id to type"
[ "$(tray_json | field 'Urgent thing' entry)" = "20260807T000000Z" ] && pass "entry: stamped" || bad "no entry"
tray list | grep -q "Sat Aug 8" \
  && pass "reports lead with the weekday" || bad "no weekday: $(tray list | sed -n 2p)"
tray list | grep -qE "20[0-9][0-9]" && bad "a date printed its year" || pass "no year on screen"
[ "$(tray_json | field 'Urgent thing' due)" = "20260808T000000Z" ] \
  && pass "the store keeps the year the screen dropped" || bad "due lost its year"

# --- F5 · take ---------------------------------------------------------------
head_ "F5 · take is a transformation"
tray dump add retries to the sync job >/dev/null
rid=$(gid_of 2026-08 'add retries to the sync job')
tray "$rid" take pri:H >/dev/null
[ "$(tray_json | rows 'add retries to the sync job')" = "1" ] && pass "lands on the tray" || bad "not on the tray"
[ "$(tray_json | field 'add retries to the sync job' priority)" = "H" ] \
  && pass "with the structure take gave it" || bad "priority lost"
[ "$(tray_json | field 'add retries to the sync job' from)" = "2026-08" ] \
  && pass "from: remembers the month it left" || bad "no from"
[ "$(garage_json 2026-08 | rows 'add retries to the sync job')" = "0" ] \
  && pass "moved, not copied: the garage row is gone" || bad "still in the garage"
[ "$(id_of 'add retries to the sync job')" = "$rid" ] && pass "and it kept its id" || bad "id changed"
out=$(tray "$rid" take)
case $out in *"already on the tray"*) pass "cannot be taken twice" ;; *) bad "got: $out" ;; esac
out=$(tray take 2>&1); code=$?
case $out in *"which one"*) pass "bare take asks which one" ;; *) bad "got: $out" ;; esac
[ "$code" = "0" ] && pass "bare take exits cleanly" || bad "bare take exited $code"
case $(tray add Structureless thing) in
  *"no pri"*) pass "a bare tray task says what it still wants" ;;
  *) bad "no nudge when structure is missing" ;;
esac

# --- F6 · done marks, it does not move ---------------------------------------
head_ "F6 · done strikes in place"
tray add Renew the TLS certificate pri:H >/dev/null
tray "$(id_of 'Renew the TLS certificate')" done >/dev/null
[ "$(tray_json | field 'Renew the TLS certificate' status)" = "completed" ] && pass "finished" || bad "not finished"
[ "$(tray_json | field 'Renew the TLS certificate' end)" = "20260807T000000Z" ] && pass "dated" || bad "no end date"
[ "$(tray_json | field 'Renew the TLS certificate' priority)" = "H" ] \
  && pass "in place, keeping what it had" || bad "attrs lost"
tray | grep -q "Renew the TLS certificate" && bad "a done item still in the default report" \
  || pass "a done item leaves the report"

# --- F21 · erase is the one verb that removes a row ----------------------------
head_ "F21 · erase removes the line"
tray add Typed it twice pri:L >/dev/null
tray "$(id_of 'Typed it twice')" erase >/dev/null
[ "$(tray_json | rows 'Typed it twice')" = "0" ] && pass "erase removes the row outright" || bad "still there"
[ "$(tray_json | rows 'Renew the TLS certificate')" = "1" ] && pass "and touches nothing else" || bad "took a neighbour"
tray add Erase me once done >/dev/null
eid=$(id_of 'Erase me once done')
tray "$eid" done >/dev/null
tray "$eid" erase >/dev/null
[ "$(tray_json | rows 'Erase me once done')" = "0" ] \
  && pass "a finished row is reachable by the same id" || bad "a finished row could not be erased"

# --- F22 · a note is the lines under a task ------------------------------------
head_ "F22 · note is the indented lines under a task"
tray add --note "expires on the 12th" Rotate the keys pri:H >/dev/null
[ "$(tray_json | note_of 'Rotate the keys')" = "expires on the 12th" ] \
  && pass "--note lands under the task" || bad "no note: $(tray_json | note_of 'Rotate the keys')"
tray "$(id_of 'Rotate the keys')" note "replaced whole" >/dev/null
[ "$(tray_json | note_of 'Rotate the keys')" = "replaced whole" ] \
  && pass "note replaces the note, not stacks" || bad "note did not replace"
tray "$(id_of 'Rotate the keys')" note | grep -q "replaced whole" \
  && pass "note with nothing to set prints it" || bad "note did not print"
tray export | grep -q '"annotations"' && pass "exports as a Taskwarrior annotation" \
  || bad "no annotation in export"
tray dump --note "why it matters" a jotting with a note >/dev/null
[ "$(garage_json 2026-08 | note_of 'a jotting with a note')" = "why it matters" ] \
  && pass "dump reads --note as a leading token" || bad "dump lost the note"
tray dump this --note is literal mid-sentence >/dev/null
[ "$(garage_json 2026-08 | rows 'this --note is literal mid-sentence')" = "1" ] \
  && pass "and past the first word the tail stays literal" || bad "tail was parsed"

# --- F7 · unload, twice ------------------------------------------------------
head_ "F7 · unload is idempotent"
tray unload --to 2026-08 >/dev/null
[ "$(garage_json 2026-08 | field 'Renew the TLS certificate' status)" = "completed" ] \
  && pass "done items land finished and dated" || bad "done item came home open"
[ "$(garage_json 2026-08 | field 'Urgent thing' priority)" = "H" ] \
  && pass "open items keep their attrs" || bad "attrs dropped"
[ "$(tray_json | jq length)" = "0" ] && pass "tray emptied" || bad "tray not empty"
snap=$(garage_json 2026-08)
out=$(tray unload --to 2026-08)
[ "$snap" = "$(garage_json 2026-08)" ] && pass "second unload is a no-op" || bad "second unload changed the month"
case $out in *"tray empty"*) pass "and says so" ;; *) bad "got: $out" ;; esac
teardown

# --- F8 · carryover ----------------------------------------------------------
head_ "F8 · carryover moves forward"
setup
TRAY_TODAY=2026-07-15 tray dump July leftover one >/dev/null
TRAY_TODAY=2026-07-15 tray dump July leftover two >/dev/null
TRAY_TODAY=2026-07-15 tray dump July dated thing >/dev/null
tray "$(gid_of 2026-07 'July dated thing')" rewrite due:2026-07-20 >/dev/null
tray add 'still working on this' pri:H >/dev/null
tray carryover --run --month 2026-07 >/dev/null
[ "$(garage_json 2026-08 | rows 'July leftover one')" = "1" ] && pass "moved into August" || bad "not moved"
[ "$(garage_json 2026-07 | rows 'July leftover one')" = "0" ] \
  && pass "and out of July: nothing is copied" || bad "July kept a copy"
[ "$(tray_json | rows 'still working on this')" = "1" ] \
  && pass "the tray is left alone — unload is its own ritual" \
  || bad "carryover emptied the tray behind your back"
[ "$(garage_json 2026-08 | field 'July dated thing' due)" = "null" ] \
  && pass "a due date that already passed is not carried" || bad "carried a stale due"
out=$(tray carryover --run --month 2026-07)
case $out in *"nothing to carry"*) pass "second run finds nothing live" ;; *) bad "got: $out" ;; esac

# --- F10 · export ------------------------------------------------------------
head_ "F10 · export"
tray add Exportable pri:H due:2026-08-12 +infra >/dev/null
tray export | valid_json && pass "valid JSON" || bad "invalid JSON: $(tray export | head -3)"
tray export | grep -q '"status": "pending"' && pass "TW status field" || bad "no TW status"
tray export | grep -q '"due": "20260812T000000Z"' && pass "TW date stamps" || bad "date not TW-shaped"
tray export | grep -q '"tags"' && pass "tags exported" || bad "tags missing"
tray export | grep -q '"id"' && pass "and the id, which is what an agent addresses" || bad "no id"

# --- F11 · filters -----------------------------------------------------------
head_ "F11 · filters"
tray +infra list | grep -q Exportable && pass "+tag filter" || bad "+tag filter broken"
tray +nope list | grep -q Exportable && bad "+tag filter matched wrongly" || pass "non-matching tag excludes"
tray due:2026-08-12 list | grep -q Exportable && pass "key:value filter" || bad "due: filter broken"
tray --json list | valid_json && pass "--json on a report" || bad "--json broken"

# --- F12 · status ------------------------------------------------------------
head_ "F12 · status names the month left behind"
tray dump to:2026-06 stale June thing >/dev/null
out=$(TRAY_TODAY=2026-06-20 tray status)
case $out in *unresolved*) bad "warned inside the month: $out" ;; *) pass "quiet inside the month" ;; esac
out=$(tray status)
case $out in *2026-06*unresolved*carryover*--run*--month*)
  pass "names the month, and the command that fixes it" ;;
  *) bad "no warning: $out" ;; esac
case $out in *"tray:"*live*) pass "and still says where you stand" ;; *) bad "no summary: $out" ;; esac
tray status --nag >/dev/null 2>&1 && bad "--nag should be gone" || pass "--nag is gone"

# --- F13 · print -------------------------------------------------------------
head_ "F13 · the default view is the print, with ids"
out=$(tray print)
case $out in *"- [ ] Exportable"*) pass "plain bullets" ;; *) bad "got: $out" ;; esac
case $out in *priority:*|*due:*) bad "print leaked attrs" ;; *) pass "no attrs in print" ;; esac
case $out in *"**infra**"*) pass "grouped by tag" ;; *) bad "not grouped: $out" ;; esac
bare=$(tray)
case $bare in *"**infra**"*) pass "bare tray groups the same way" ;; *) bad "got: $bare" ;; esac
case $bare in *priority:*|*URG*) bad "bare tray leaked the table" ;; *) pass "no table, no attrs" ;; esac
case $bare in *"- [ ]"*) bad "bare tray has journal checkboxes" ;; *) pass "ids instead of checkboxes" ;; esac
[ -n "$(id_of Exportable)" ] && pass "ids are on screen" || bad "no id to type"
teardown

# --- F15 · find across layers ------------------------------------------------
head_ "F15 · find reaches every layer and month"
setup
for m in 2026-05 2026-06 2026-07; do
  tray dump "to:$m" add retries to the sync job >/dev/null
done
out=$(tray find retries)
case $out in *2026-05*2026-06*2026-07*) pass "hits every month, oldest first" ;; *) bad "got: $out" ;; esac
tray add add retries to the sync job pri:H >/dev/null
tray find retries | grep -q "^tray" && pass "searches the tray too" || bad "tray layer missed"
tray "$(id_of 'add retries to the sync job')" done >/dev/null
tray find retries | grep -q "^tray" && bad "a finished row surfaced without --all" \
  || pass "the finished are out unless asked"
tray --all find retries | grep -q "^tray" && pass "--all reaches them" || bad "--all missed the finished"
case $(tray find nothingmatchesthis) in "no match") pass "empty search" ;; *) bad "no-match wrong" ;; esac
teardown

# --- F16 · agent surface never prompts ---------------------------------------
head_ "F16 · headless"
setup
for verb in "" "list" "garage list" "status" "export" "print" "--json list" "plugin"; do
  # shellcheck disable=SC2086
  limit "$BIN" $verb </dev/null >/dev/null 2>&1
  code=$?
  [ "$code" -le 1 ] || bad "\`tray $verb\` exited $code"
done
pass "every report runs headless without prompting"
tray --version | grep -q "^tray " && pass "--version" || bad "--version broken"
tray help | grep -q "two layers" && pass "help" || bad "help broken"
teardown

# --- F17 · the round trip home ----------------------------------------------
# F7 covers a tray whose tasks never came from this month. This is the common one:
# dump here, take it, hand it back — it goes home to the month it left, keeping what
# the tray added, and forgets where home was once it is there.
head_ "F17 · unload brings the tray home whole"
setup
tray dump ship the release notes >/dev/null
tray dump finish the migration >/dev/null
tray "$(gid_of 2026-08 'ship the release notes')" take pri:H due:2026-08-20 +work >/dev/null
tray "$(gid_of 2026-08 'finish the migration')" take pri:L >/dev/null
tray "$(id_of 'ship the release notes')" done >/dev/null
tray unload --to 2026-08 >/dev/null

[ "$(garage_json 2026-08 | rows 'ship the release notes')" = "1" ] \
  && pass "one row home, not a copy beside the one it left" || bad "duplicated"
[ "$(garage_json 2026-08 | field 'ship the release notes' status)" = "completed" ] \
  && pass "a finished task lands finished" || bad "finished task came home open"
[ "$(garage_json 2026-08 | field 'ship the release notes' end)" = "20260807T000000Z" ] && pass "and dated" || bad "no done date"
[ "$(garage_json 2026-08 | field 'ship the release notes' priority)" = "H" ] \
  && [ "$(garage_json 2026-08 | field 'ship the release notes' due)" = "20260820T000000Z" ] \
  && pass "a finished task keeps what the tray gave it" || bad "attrs dropped"
[ "$(garage_json 2026-08 | field 'finish the migration' priority)" = "L" ] \
  && pass "an open task keeps its attrs, so taking it again is free" || bad "open task came home bare"
[ "$(garage_json 2026-08 | field 'finish the migration' from)" = "null" ] \
  && pass "from: dropped on the way home" || bad "from: is noise on a row that lives here"
[ "$(tray_json | jq length)" = "0" ] && pass "the tray is empty" || bad "something stayed on the tray"

tray "$(gid_of 2026-08 'finish the migration')" take >/dev/null
tray "$(id_of 'finish the migration')" unload >/dev/null
[ "$(garage_json 2026-08 | rows 'finish the migration')" = "1" ] \
  && pass "one task alone knows the month it came from" || bad "a single unload needed --to"
teardown

# --- F18 · nothing is inferred headlessly -------------------------------------
head_ "F18 · nothing is inferred headlessly"
setup
tray carryover >/dev/null 2>&1 && bad "bare carryover should fail" || pass "bare carryover refuses to run"
tray carryover --run >/dev/null 2>&1 && bad "--run should need a month" || pass "--run refuses to guess the month"
tray unload >/dev/null 2>&1 && bad "bare unload should fail" || pass "unload refuses to guess the month"
tray garage list --nope >/dev/null 2>&1 && bad "an unknown flag was swallowed" \
  || pass "an unknown flag is an error, not a silence"
tray import "$TRAY_HOME" >/dev/null 2>&1 && bad "import guessed a format" || pass "import refuses to guess a format"
teardown

# --- F19 · the terminal header ------------------------------------------------
# Runs in a shell profile on every new terminal, so the empty case matters more
# than the full one: it must cost a fresh terminal nothing at all.
head_ "F19 · head is the terminal header"
setup
out=$(tray head)
[ -z "$out" ] && pass "an empty tray prints nothing at all" || bad "printed: $out"

tray add 'first thing' pri:H due:2026-08-04 >/dev/null
tray add 'second thing' pri:M due:2026-08-08 >/dev/null
tray add 'third thing' pri:L >/dev/null
tray add 'fourth thing' pri:L >/dev/null

out=$(tray head)
[ "$(printf '%s' "$out" | grep -c .)" = "5" ] \
  && pass "a framed box of three rows by default" || bad "got:\n$out"
case $out in "╭─ tray "*) pass "titled top edge" ;; *) bad "no title: $out" ;; esac
case $out in *"╰─"*) pass "and it closes" ;; *) bad "unclosed box: $out" ;; esac
case $out in *"of 4"*) bad "a count was asked to go: $out" ;; *) pass "no count" ;; esac
printf '%s' "$out" | grep -q "$(printf '\033')" \
  && bad "escape codes survived a pipe" || pass "plain when piped, coloured on a terminal"
case $out in *"Sat Aug 8"*) pass "head uses the one date format" ;;
  *) bad "head must render dates like everything else:\n$out" ;; esac
case $out in *"3d over"*|*tomorrow*) bad "a second date vocabulary came back:\n$out" ;;
  *) pass "and carries no relative wording" ;; esac
case $out in *[0-9][0-9].[0-9]*) bad "urgency numbers are noise here: $out" ;;
  *) pass "no urgency figures" ;; esac

out=$(tray head 2)
[ "$(printf '%s' "$out" | grep -c .)" = "4" ] && pass "the count is honoured" || bad "got:\n$out"

tray add 'a task with a description far too long to fit inside any sensible terminal window at all' pri:H due:2026-08-01 >/dev/null
out=$(tray head 1)
longest=$(printf '%s' "$out" | awk '{ print length }' | sort -rn | head -1)
[ "$longest" -le 80 ] && pass "clamps to the terminal (80 when piped)" || bad "line was $longest wide"
out=$(tray +nope head)
[ -z "$out" ] && pass "a filter that matches nothing is silent too" || bad "printed: $out"
teardown

# --- F20 · restore --------------------------------------------------------------
head_ "F20 · restore says a task was not finished after all"
setup
tray add alpha pri:M >/dev/null
tray add beta pri:M >/dev/null
tray add gamma pri:M >/dev/null
tray "$(id_of beta),$(id_of gamma)" done >/dev/null

tray list | grep -q beta && bad "a finished task should be out of the default view" \
  || pass "finished tasks are hidden by default"
tray list --all | grep -q "✓" && pass "--all shows them, marked" || bad "no mark: $(tray list --all)"

tray "$(id_of beta)" restore >/dev/null
[ "$(tray_json | field beta status)" = "pending" ] && pass "restore reopens it" || bad "still done"
[ "$(tray_json | field beta end)" = "null" ] && pass "and leaves no trace" || bad "end date lingered"
[ "$(tray_json | field beta priority)" = "M" ] && pass "attributes are untouched" || bad "attrs lost"
[ "$(tray_json | field gamma status)" = "completed" ] \
  && pass "it restored the one you named, not a neighbour" || bad "restored the wrong task"

out=$(tray "$(id_of alpha)" restore)
case $out in *"nothing finished"*) pass "an open id says so rather than lying" ;; *) bad "got: $out" ;; esac
teardown

# --- F23 · plugin ----------------------------------------------------------------
# A plugin is a folder holding an executable, and the folder name is the whole
# manifest. Nothing here runs one: this asserts what tray is willing to believe.
head_ "F23 · plugin lists what is installed"
setup

out=$(tray plugin)
case $out in *"no plugins"*) pass "no plugins is not an error" ;; *) bad "got: $out" ;; esac

# Installing one is opt-in, so until you do, tray is the tray you already had: the
# help does not advertise a verb whose only possible answer is "no plugins".
tray help | grep -q "tray plugin" && bad "help advertises plugin with none installed" \
  || pass "help is untouched until a plugin exists"

mkdir -p "$TRAY_HOME/plugins/notion" "$TRAY_HOME/plugins/halfdone"
printf '#!/bin/sh\n' > "$TRAY_HOME/plugins/notion/run"; chmod +x "$TRAY_HOME/plugins/notion/run"
printf '#!/bin/sh\n' > "$TRAY_HOME/plugins/halfdone/run"   # deliberately not executable

out=$(tray plugin list)
case $out in *notion*) pass "an installed plugin is listed" ;; *) bad "got: $out" ;; esac
tray help | grep -q "tray plugin" && pass "and now the help says so" || bad "help still silent with a plugin installed"
case $out in *halfdone*) bad "a non-executable run counted as a plugin: $out" ;;
  *) pass "a folder without an executable run is half an install" ;; esac
case $out in *"garage empty"*) pass "a garage never written says so" ;; *) bad "got: $out" ;; esac

# The garage a plugin owns is ordinary rows in the store, so it reads with no plugin
# involved at all — which is what keeps a deleted plugin from taking your tasks.
tray dump to:notion +infra ship the billing migration >/dev/null
tray garage --month notion list | grep -q "billing migration" \
  && pass "a plugin garage reads as a plain garage" || bad "not readable: $(tray garage --month notion list)"
tray plugin | grep -q "garage 1 row" && pass "and the listing counts it" || bad "got: $(tray plugin)"

tray "$(gid_of notion 'ship the billing migration')" take pri:H >/dev/null
[ "$(tray_json | field 'ship the billing migration' from)" = "notion" ] \
  && pass "take remembers the garage it left" || bad "from lost"
[ "$(garage_json notion | rows 'ship the billing migration')" = "0" ] \
  && pass "and the row moved rather than copied" || bad "the plugin garage kept a copy"

out=$(tray plugin sync 2>&1)
case $out in *carryover*) pass "sync is refused, and says where syncing happens" ;; *) bad "got: $out" ;; esac

# A menu verb is an executable under actions/, named after itself (105). A plugin may
# be nothing but verbs — then it keeps no garage, and the listing says what it adds
# rather than a garage it never wrote.
mkdir -p "$TRAY_HOME/plugins/gcal/actions"
printf '#!/bin/sh\n' > "$TRAY_HOME/plugins/gcal/actions/schedule"; chmod +x "$TRAY_HOME/plugins/gcal/actions/schedule"
printf '#!/bin/sh\n' > "$TRAY_HOME/plugins/gcal/actions/half"   # deliberately not executable
line=$(tray plugin | grep '^gcal')
case $line in *"schedule"*) pass "a verb-only plugin is listed with its verb" ;; *) bad "got: $line" ;; esac
case $line in *half*) bad "a non-executable verb was offered: $line" ;;
  *) pass "a verb without the exec bit is half an install" ;; esac
case $line in *garage*) bad "a garage it never wrote: $line" ;;
  *) pass "no garage is claimed for a plugin that keeps none" ;; esac
teardown

# --- F24 · ids are permanent ------------------------------------------------------
# An agent remembers an id across runs. Whatever happens to the neighbours, it must
# still name the same task — and an erased id must never come back as a different one.
head_ "F24 · ids are permanent"
setup
tray add a pri:M >/dev/null
tray add b pri:M >/dev/null
tray add c pri:M >/dev/null
ida=$(id_of a); idb=$(id_of b); idc=$(id_of c)
# c holds the highest id, so this is the erase a plain rowid would silently reuse.
tray "$idc" erase >/dev/null
[ "$(id_of a)" = "$ida" ] && [ "$(id_of b)" = "$idb" ] \
  && pass "erasing a neighbour renumbers nothing" || bad "a or b moved: $(tray_json | jq -c 'map(.id)')"
tray add d pri:M >/dev/null
[ "$(id_of d)" -gt "$idc" ] && pass "a new task takes a new id, never an erased one" || bad "d took $(id_of d), c had $idc"
tray "$ida" done >/dev/null
[ "$(tray_json | field a status)" = "completed" ] && pass "an id names the same task after every change" || bad "wrong row marked"
[ "$(id_of a)" = "$ida" ] && pass "and finishing keeps it too" || bad "a changed id"
teardown

# --- F33 · import -----------------------------------------------------------------
# The markdown home tray used to keep comes in whole: layers by filename, notes as the
# lines under a bullet, struck lines finished, arrows skipped as history.
head_ "F33 · import migrates the markdown home"
setup
out=$(tray import --format md "$ROOT/scripts/testdata/migrate")
case $out in *"tray.md: 3 imported"*) pass "reports what each file gave" ;; *) bad "got: $out" ;; esac
[ "$(tray_json | field 'Rotate the api keys' priority)" = "H" ] && pass "tray.md is the tray, attrs intact" || bad "tray row lost attrs"
[ "$(tray_json | note_of 'Rotate the api keys')" = "$(printf 'The old keys expire on the 12th.\nRotate staging first.')" ] \
  && pass "the lines under a bullet are its note" || bad "note: $(tray_json | note_of 'Rotate the api keys')"
[ "$(tray_json | field 'Renew the TLS certificate' status)" = "completed" ] \
  && [ "$(tray_json | field 'Renew the TLS certificate' end)" = "20260829T000000Z" ] \
  && pass "a struck line arrives finished, on its date" || bad "struck line mishandled"
[ "$(tray_json | field 'the billing page feels slow on first load' from)" = "2026-08" ] \
  && pass "from: is the month it came from" || bad "from lost"
[ "$(garage_json 2026-08 | field 'add metrics to the sync worker' tags)" = '["infra"]' ] \
  && pass "a month file is that month's garage" || bad "month row lost"
[ "$(garage_json 2026-08 | rows 'the billing page feels slow on first load')" = "0" ] \
  && pass "a → line is history and is skipped" || bad "an arrow line was imported"
[ "$(garage_json 2026-08 | field 'gave up on this' status)" = "completed" ] \
  && pass "a bare strike arrives finished today" || bad "bare strike came in open"
[ "$(garage_json someday | rows 'learn to sail')" = "1" ] && pass "someday is a garage" || bad "someday lost"
[ "$(garage_json notion | rows 'ship the billing migration')" = "1" ] \
  && pass "any other file is a plugin's garage" || bad "plugin garage lost"
before=$(tray_json | jq length)
tray import --format md "$ROOT/scripts/testdata/migrate" >/dev/null
[ "$(tray_json | jq length)" = "$before" ] && pass "importing twice adds nothing" || bad "the second import duplicated rows"
teardown

printf '\n'
[ "$fail" = 0 ] && printf '\033[32mtray flows pass\033[0m\n' || printf '\033[31mtray flows FAILED\033[0m\n'
exit "$fail"
