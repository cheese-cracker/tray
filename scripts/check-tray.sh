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
  export TRAY_CONFIG="$TRAY_HOME/config.yaml"   # never the user's file, and never their environment either
  unset OPENROUTER_API_KEY OPENROUTER_MODEL
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

# The fixture plugins: three-line shell scripts that stand in for a real one.
FIXTURES="$ROOT/internal/sync/testdata/plugins"
install_plugin() { mkdir -p "$TRAY_HOME/plugins"; cp -R "$FIXTURES/$1" "$TRAY_HOME/plugins/"; }

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
# manifest. This asserts what tray is willing to believe about one.
head_ "F23 · plugin lists what is installed"
setup

out=$(tray plugin)
case $out in *"no plugins"*) pass "no plugins is not an error" ;; *) bad "got: $out" ;; esac

# Installing one is opt-in, so until you do, tray is the tray you already had: the
# help does not advertise a verb whose only possible answer is "no plugins".
tray help | grep -q "tray plugin" && bad "help advertises plugin with none installed" \
  || pass "help is untouched until a plugin exists"

mkdir -p "$TRAY_HOME/plugins/notion" "$TRAY_HOME/plugins/halfdone"
printf '#!/bin/sh\necho "{\\"pull\\":[]}"\n' > "$TRAY_HOME/plugins/notion/sync"; chmod +x "$TRAY_HOME/plugins/notion/sync"
printf '#!/bin/sh\n' > "$TRAY_HOME/plugins/halfdone/sync"   # deliberately not executable

out=$(tray plugin list)
case $out in *notion*) pass "an installed plugin is listed" ;; *) bad "got: $out" ;; esac
tray help | grep -q "tray plugin" && pass "and now the help says so" || bad "help still silent with a plugin installed"
case $out in *halfdone*half-installed*) pass "a folder without an executable sync is listed as half-installed" ;;
  *) bad "half-installed folder not shown as such: $out" ;; esac
case $(tray sync) in *halfdone*) bad "a half-installed folder was run" ;; *) pass "and is never run" ;; esac
[ "$(tray plugin --json | jq -r '.[] | select(.name=="notion") | .hooks | join(" ")')" = "manual" ] \
  && pass "a sync plugin joins the manual hook" || bad "hooks: $(tray plugin --json | jq -c '.[]|select(.name=="notion")|.hooks')"
case $out in *"never run"*) pass "a plugin never run says so" ;; *) bad "got: $out" ;; esac

# The garage a plugin owns is ordinary rows in the store, so it reads with no plugin
# involved at all — which is what keeps a deleted plugin from taking your tasks.
tray dump to:notion +infra ship the billing migration >/dev/null
tray garage --month notion list | grep -q "billing migration" \
  && pass "a plugin garage reads as a plain garage" || bad "not readable: $(tray garage --month notion list)"
[ "$(garage_json notion | rows 'ship the billing migration')" = "1" ] && pass "and the store counts it" || bad "not in the store"

tray "$(gid_of notion 'ship the billing migration')" take pri:H >/dev/null
[ "$(tray_json | field 'ship the billing migration' from)" = "notion" ] \
  && pass "take remembers the garage it left" || bad "from lost"
[ "$(garage_json notion | rows 'ship the billing migration')" = "0" ] \
  && pass "and the row moved rather than copied" || bad "the plugin garage kept a copy"

out=$(tray plugin run notion)
case $out in *"notion: nothing new"*) pass "run prints one plugin's plan and lands nothing" ;; *) bad "got: $out" ;; esac
[ "$(tray plugin --json | jq -r '.[] | select(.name=="notion") | .sync.hook + " " + (.sync.ok|tostring)')" = "manual true" ] \
  && pass "and the listing remembers the run" || bad "got: $(tray plugin)"
tray plugin run nope >/dev/null 2>&1 && bad "an unknown plugin ran" || pass "an unknown plugin is an error"

# The example settings are the form; tray writes the answers and never reads them.
printf '{"url":"","rules":"in progress only"}' > "$TRAY_HOME/plugins/notion/settings.example.json"
tray plugin set notion url=https://example.test >/dev/null
[ "$(jq -r .url "$TRAY_HOME/plugins/notion/settings.json")" = "https://example.test" ] \
  && pass "set writes the plugin's settings file" || bad "settings not written"
tray plugin set notion colour=red >/dev/null 2>&1 && bad "a key the example does not name was accepted" \
  || pass "a key the example does not name is refused"
tray plugin | grep '^notion' | grep -q "launch" && bad "launch claimed without the marker" || pass "manual only until the marker exists"
touch "$TRAY_HOME/plugins/notion/on-launch"
tray plugin | grep '^notion' | grep -q "launch" && pass "the marker opts a plugin into launch" || bad "marker ignored"

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
for id in "$ida" "$idb" "$idc"; do
  case $id in [0-9a-z][0-9a-z][0-9a-z][0-9a-z]) ;; *) bad "id $id is not four base36 characters" ;; esac
  case $id in *[0-9]*) ;; *) bad "id $id has no digit, so it could be a word" ;; esac
done
[ "$ida" != "$idb" ] && [ "$idb" != "$idc" ] && [ "$ida" != "$idc" ] && pass "three rows, three ids" || bad "ids collide: $ida $idb $idc"
tray "$idc" erase >/dev/null
[ "$(id_of a)" = "$ida" ] && [ "$(id_of b)" = "$idb" ] \
  && pass "erasing a neighbour changes nothing" || bad "a or b changed: $(tray_json | jq -c 'map(.id)')"
tray add d pri:M >/dev/null
[ "$(id_of d)" != "$ida" ] && [ "$(id_of d)" != "$idb" ] && pass "a new task takes an id of its own" || bad "d took a live id"
tray "$ida" done >/dev/null
[ "$(tray_json | field a status)" = "completed" ] && pass "an id names the same task after every change" || bad "wrong row marked"
[ "$(id_of a)" = "$ida" ] && pass "and finishing keeps it too" || bad "a changed id"
tray done list >/dev/null 2>&1; [ "$(tray_json | field b status)" = "pending" ] && pass "a four-letter word is a filter, not an id" || bad "'done' was read as an id"
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

# --- F27 · round trips ---------------------------------------------------------------
# Another tool's shape is the wire, not the disk: what leaves as Taskwarrior JSON or
# todo.txt comes back field for field, and where it lands is read off the row itself.
head_ "F27 · export and import round-trip"
setup
tray add --note "only on the first load" Exportable pri:H due:2026-08-12 +infra >/dev/null
tray add Plain pri:M >/dev/null
tray add Finished pri:L >/dev/null
tray "$(id_of Finished)" done >/dev/null
first=$TRAY_HOME
tray export --all > "$first/tw.json"
tray export --format todotxt --all > "$first/todo.txt" 2>/dev/null
grep -q '^(A) 2026-08-07 Exportable +infra due:2026-08-12$' "$first/todo.txt" \
  && pass "todo.txt: priority letter, creation date, +tag, due:" || bad "got: $(cat "$first/todo.txt")"
grep -q '^x 2026-08-07 2026-08-07 Finished pri:C$' "$first/todo.txt" \
  && pass "a finished line leads with x and its date" || bad "got: $(cat "$first/todo.txt")"
tray export --format todotxt --all 2>&1 >/dev/null | grep -q "note" \
  && pass "and says when a note was left out" || bad "notes dropped silently"

TRAY_HOME=$(mktemp -d); export TRAY_HOME; tray init >/dev/null
tray import --format tw "$first/tw.json" | grep -q "3 imported" && pass "tw import lands every row" \
  || bad "got: $(tray import --format tw "$first/tw.json")"
[ "$(tray_json | field Exportable priority)" = "H" ] && pass "on the tray, since it had structure" || bad "not on the tray"
[ "$(jq 'map(del(.id)) | sort_by(.description)' "$first/tw.json")" = "$(tray_json | jq 'map(del(.id)) | sort_by(.description)')" ] \
  && pass "and exports again field for field" || bad "round trip drifted: $(tray_json)"
# stdin is closed throughout (F16), so the wire arrives as a file here.
printf '[{"description":"filed under a project","status":"pending","project":"alpha","uuid":"u-1","priority":"H"}]' > "$TRAY_HOME/in.json"
tray import --format tw "$TRAY_HOME/in.json" >/dev/null
[ "$(tray_json | field 'filed under a project' tags)" = '["alpha"]' ] && pass "a project comes in as a tag" || bad "no tag"
[ "$(tray_json | field 'filed under a project' uuid)" = "u-1" ] && pass "and keeps its uuid" || bad "uuid lost"
printf '[{"description":"filed under a project, renamed","status":"pending","project":"alpha","uuid":"u-1","priority":"H"}]' > "$TRAY_HOME/in.json"
tray import --format tw "$TRAY_HOME/in.json" | grep -q "1 updated" && pass "the same uuid updates in place" || bad "did not update"
[ "$(tray_json | jq length)" = "4" ] && pass "rather than adding a row" || bad "$(tray_json | jq length) rows"
rm -rf "$TRAY_HOME"

TRAY_HOME=$(mktemp -d); export TRAY_HOME; tray init >/dev/null
tray import --format todotxt "$first/todo.txt" >/dev/null
tray export --format todotxt --all 2>/dev/null | diff -q "$first/todo.txt" - >/dev/null \
  && pass "todo.txt round-trips line for line" || bad "todo.txt drifted: $(tray export --format todotxt --all 2>/dev/null)"
printf 'a bare jotting\n(B) committed thing due:2026-08-20 @work\n' > "$TRAY_HOME/in.txt"
tray import --format todotxt "$TRAY_HOME/in.txt" >/dev/null
[ "$(garage_json 2026-08 | rows 'a bare jotting')" = "1" ] \
  && pass "a line with no structure is a jotting for this month" || bad "jotting went elsewhere"
[ "$(tray_json | field 'committed thing' tags)" = '["work"]' ] \
  && pass "@context is a tag, and a priority puts the line on the tray" || bad "got: $(tray_json)"
rm -rf "$first"
teardown

# --- F32 · context ---------------------------------------------------------------------
# What you hand an agent is the report you read plus what it cannot see from a list.
head_ "F32 · context is the report with its notes"
setup
tray add --note "expires on the 12th" Rotate the keys pri:H +infra >/dev/null
tray add Plain pri:M >/dev/null
out=$(tray context)
case $out in *"**infra**"*) pass "grouped like the report" ;; *) bad "got: $out" ;; esac
case $out in *"$(id_of 'Rotate the keys')  Rotate the keys"*) pass "with ids" ;; *) bad "no id: $out" ;; esac
case $out in *"expires on the 12th"*) pass "and the note under its task" ;; *) bad "no note: $out" ;; esac
out=$(tray "$(id_of Plain)" context)
case $out in *Rotate*) bad "an id should narrow it: $out" ;; *Plain*) pass "an id narrows it to that task" ;; *) bad "got: $out" ;; esac
case $(tray +infra context) in *Plain*) bad "a filter should narrow it" ;; *Rotate*) pass "a filter narrows it too" ;; *) bad "filter broke it" ;; esac
case $(tray zz9z context) in "nothing to copy") pass "an unknown id says so" ;; *) bad "got: $(tray zz9z context)" ;; esac
teardown

# --- F25 · waiting ---------------------------------------------------------------------
# A waiting task lies in the garage of its month — a line that waits for a day, not for
# you to look — and the one event lifts it onto the tray when the day comes.
head_ "F25 · a waiting task lies in the garage until its day"
setup
tray add Call mom wait:2026-08-10 due:2026-08-11 pri:H >/dev/null
[ "$(garage_json 2026-08 | rows 'Call mom')" = "1" ] && pass "it lands in the garage of its month" || bad "not in the garage"
[ "$(garage_json 2026-08 | field 'Call mom' status)" = "waiting" ] && pass "and reads as waiting" || bad "status: $(garage_json 2026-08 | field 'Call mom' status)"
tray garage list | grep -q "WAIT" && pass "the garage shows the day" || bad "no WAIT column"
tray status | grep -q "waiting 1" && pass "status counts it" || bad "got: $(tray status)"
case $(tray sync) in *"lifted 0"*) pass "before its day, sync leaves it there" ;; *) bad "lifted early" ;; esac
case $(TRAY_TODAY=2026-08-10 tray sync) in *"lifted 1"*) pass "on its day, sync lifts it" ;; *) bad "not lifted" ;; esac
[ "$(tray_json | field 'Call mom' priority)" = "H" ] && [ "$(tray_json | field 'Call mom' due)" = "20260811T000000Z" ] \
  && pass "onto the tray with what it carried" || bad "attrs lost: $(tray_json)"
[ "$(tray_json | field 'Call mom' wait)" = "null" ] && pass "its wait spent" || bad "wait lingered"
tray "$(id_of 'Call mom')" unload >/dev/null
case $(TRAY_TODAY=2026-08-12 tray sync) in *"lifted 0"*) pass "handed back, it is not lifted again" ;; *) bad "lifted twice" ;; esac
teardown

# --- F26 · recurrence -------------------------------------------------------------------
# A template keeps one live child at a time. The next one is due on the first occurrence
# on or after today and after the last — never the same period twice, never a backlog.
head_ "F26 · recurrence serves each period once"
setup
out=$(tray add Weekly review recur:weekly due:2026-08-08 pri:M +ops)
case $out in *template*next*"Sat Aug 8"*) pass "adding a template names the next occurrence" ;; *) bad "got: $out" ;; esac
[ "$(tray_json | rows 'Weekly review')" = "2" ] && pass "a template and one child exist" || bad "rows: $(tray_json | rows 'Weekly review')"
[ "$(tray list | grep -c 'Weekly review')" = "1" ] && pass "the default list shows the child alone" || bad "got: $(tray list)"
tray list --all | grep -q "↻" && pass "the template is marked under --all" || bad "no mark"
tray status | grep -q "templates 1" && pass "status counts it" || bad "got: $(tray status)"
case $(tray sync) in *"materialized 0"*) pass "a live child blocks a second" ;; *) bad "materialized again" ;; esac
child() { tray_json | jq -r 'first(.[] | select(.description=="Weekly review" and .status=="pending")) | .id'; }
child_due() { tray_json | jq -r 'first(.[] | select(.description=="Weekly review" and .status=="pending")) | .due'; }
tray "$(child)" done >/dev/null
case $(tray sync) in *"materialized 1"*) pass "once the child is done, the next period gets one" ;; *) bad "nothing materialized" ;; esac
[ "$(child_due)" = "20260815T000000Z" ] && pass "due one period on — the same period is never served twice" || bad "due: $(child_due)"
tray "$(child)" done >/dev/null
TRAY_TODAY=2026-09-20 tray sync >/dev/null
[ "$(child_due)" = "20260926T000000Z" ] && pass "missed periods are skipped, not stacked" || bad "due: $(child_due)"
tray add Odd recur:fortnightly due:2026-08-08 >/dev/null 2>&1 && bad "an unknown period was accepted" || pass "an unknown period is refused"
tpl=$(tray_json | jq -r 'first(.[] | select(.recur=="weekly")) | .id')
tray "$tpl" done >/dev/null
tray "$(child)" done >/dev/null
case $(TRAY_TODAY=2026-10-01 tray sync) in *"materialized 0"*) pass "done on a template ends it" ;; *) bad "an ended template materialized" ;; esac
teardown

# --- F28 · sync plans --------------------------------------------------------------------
# What a plugin reports is shown, never landed, until you say so.
head_ "F28 · sync prints a plan and lands nothing without --apply"
setup
install_plugin echo
out=$(tray sync)
case $out in "materialized 0 · lifted 0"*) pass "the summary leads" ;; *) bad "got: $out" ;; esac
case $out in *"echo: 2 adds · 0 updates · 1 push"*) pass "then each plugin's plan" ;; *) bad "got: $out" ;; esac
case $out in *"+ Ship the notes  +work"*) pass "adds are listed with their tags" ;; *) bad "no add line: $out" ;; esac
case $out in *"↑ n0 done=2026-08-07"*) pass "and what the plugin would push" ;; *) bad "no push line: $out" ;; esac
case $out in *"evidence: evidence/shot.png"*) pass "with the evidence it gathered" ;; *) bad "no evidence" ;; esac
case $out in *"tray sync --apply"*) pass "and names the way to land it" ;; *) bad "no hint" ;; esac
[ "$(garage_json echo | jq length)" = "0" ] && pass "nothing landed" || bad "rows landed: $(garage_json echo)"
tray sync --json | valid_json && pass "--json is valid" || bad "--json broken"
[ "$(tray sync --json | jq -r '.plugins[0].adds | length')" = "2" ] && pass "and carries the plan" || bad "json plan wrong"
teardown

# --- F29 · apply ---------------------------------------------------------------------------
head_ "F29 · apply lands a plugin's plan whole or not at all"
setup
install_plugin echo
tray sync --apply | grep -q "applied: 2 adds" && pass "--apply lands it" || bad "got: $(tray sync --apply)"
[ "$(garage_json echo | rows 'Ship the notes')" = "1" ] && pass "adds arrive in the plugin's garage" || bad "not in the garage"
[ "$(garage_json echo | field 'Renew the cert' status)" = "completed" ] \
  && [ "$(garage_json echo | field 'Renew the cert' priority)" = "H" ] \
  && pass "with what the plugin reported" || bad "fields lost: $(garage_json echo)"
grep -q '"n0"' "$TRAY_HOME/plugins/echo/applied.json" && pass "the push reached the plugin" || bad "no applied.json"
tray plugin | grep -q "echo.*apply .*applied" && pass "the listing remembers the apply" || bad "got: $(tray plugin)"
case $(tray sync) in *"echo: 0 adds"*) pass "a second sync finds the rows it knows" ;; *) bad "got: $(tray sync)" ;; esac
printf '{"pull":[{"key":"a","text":"good row"},{"key":"b","text":"bad row","priority":"Z"}],"push":[]}' > "$TRAY_HOME/plugins/echo/plan.json"
tray sync --apply >/dev/null 2>&1 && bad "a plan the store rejects was applied" || pass "a row the store refuses fails the apply"
[ "$(garage_json echo | rows 'good row')" = "0" ] && pass "and the good row did not land alone" || bad "half a plan landed"
teardown

# --- F30 · failure isolation ---------------------------------------------------------------
head_ "F30 · a failing plugin fails alone"
setup
install_plugin echo; install_plugin fail; install_plugin ask
out=$(tray sync)
case $out in *"fail: failed — boom"*) pass "a failure is named with its first stderr line" ;; *) bad "got: $out" ;; esac
case $out in *"ask: needs you — login needed"*) pass "exit 2 is a question, not a failure" ;; *) bad "got: $out" ;; esac
case $out in *"echo: 2 adds"*) pass "the others still plan" ;; *) bad "echo suffered: $out" ;; esac
grep -q "and more detail" "$TRAY_HOME/plugins/fail/log" && pass "the whole of stderr is in the log" || bad "log missing"
tray sync --apply >/dev/null
[ "$(garage_json echo | jq length)" = "2" ] && pass "--apply lands the ones that planned" || bad "echo did not land"
[ "$(garage_json fail | jq length)" = "0" ] && pass "and a failed one landed nothing" || bad "fail landed rows"
tray plugin | grep -q "fail.*failed — boom" && pass "the listing names the failure" || bad "got: $(tray plugin)"
tray status | grep -q "fail failed — boom" && pass "so does status" || bad "got: $(tray status)"
teardown

# --- F31 · timeout ---------------------------------------------------------------------------
head_ "F31 · a slow plugin times out alone"
setup
install_plugin echo; install_plugin slow
out=$(limit "$BIN" sync --timeout 1s </dev/null)
case $out in *"slow: failed — timed out"*) pass "the slow one is cut off and named" ;; *) bad "got: $out" ;; esac
case $out in *"echo: 2 adds"*) pass "echo's plan still prints" ;; *) bad "echo suffered: $out" ;; esac
tray sync --timeout soon >/dev/null 2>&1 && bad "a bad timeout was accepted" || pass "a bad --timeout is an error"
teardown

# --- F34 · the whole event in one run --------------------------------------------------------
# Three plugins installed at once, one sync: the good plan prints, the failure is named,
# the slow one is cut off — then one --apply lands exactly the good plan and hands its push
# back. The pieces are F28–F31; this is the run you actually do.
head_ "F34 · one sync runs every plugin and lands one plan whole"
setup
install_plugin echo; install_plugin fail; install_plugin slow
out=$(limit "$BIN" sync --timeout 1s </dev/null)
case $out in *"echo: 2 adds · 0 updates · 1 push"*) pass "echo plans" ;; *) bad "got: $out" ;; esac
case $out in *"fail: failed — boom"*) pass "fail is named" ;; *) bad "got: $out" ;; esac
case $out in *"slow: failed — timed out"*) pass "slow is cut off" ;; *) bad "got: $out" ;; esac
[ "$(garage_json echo | jq length)" = "0" ] && pass "and nothing has landed yet" || bad "rows landed before --apply"
out=$(limit "$BIN" sync --apply --plugin echo --timeout 1s </dev/null)
case $out in *"applied: 2 adds"*) pass "--apply --plugin lands the one plan" ;; *) bad "got: $out" ;; esac
[ "$(garage_json echo | rows 'Ship the notes')" = "1" ] && [ "$(garage_json echo | rows 'Renew the cert')" = "1" ] \
  && pass "both rows are in echo's garage" || bad "garage: $(garage_json echo)"
[ "$(garage_json echo | field 'Ship the notes' tags)" = '["work"]' ] && pass "with their tags" || bad "tags lost"
grep -q '"n0"' "$TRAY_HOME/plugins/echo/applied.json" && pass "the push reached the plugin" || bad "no applied.json"
[ "$(garage_json fail | jq length)" = "0" ] && [ "$(garage_json slow | jq length)" = "0" ] \
  && pass "the others landed nothing" || bad "a failed plugin landed rows"
tray status | grep -q "fail failed — boom" && pass "status still names the failure" || bad "got: $(tray status)"
teardown

# --- F35 · the mirror -------------------------------------------------------------------------
# Two markdown files beside the database, one bullet per open task with its id, rewritten
# after every write. The lightest view there is: drop the folder in a vault and read it
# from a phone.
head_ "F35 · the mirror is rewritten after every write"
setup
tray dump 'from the shell' >/dev/null
tray add 'on the tray' pri:H >/dev/null
gid=$(gid_of 2026-08 'from the shell'); tid=$(id_of 'on the tray')
grep -q "^- ($gid) from the shell\$" "$TRAY_HOME/garage.md" && pass "garage.md has the line with its id" || bad "garage.md: $(cat "$TRAY_HOME/garage.md")"
grep -q "^## 2026-08\$" "$TRAY_HOME/garage.md" && pass "under its month" || bad "no month heading"
grep -q "^- ($tid) on the tray\$" "$TRAY_HOME/tray.md" && pass "tray.md has the tray task" || bad "tray.md: $(cat "$TRAY_HOME/tray.md")"
grep -q "pri\|+" "$TRAY_HOME/garage.md" "$TRAY_HOME/tray.md" && bad "the mirror carries more than ids and words" || pass "ids and words, nothing else"
tray "$tid" done >/dev/null
grep -q "on the tray" "$TRAY_HOME/tray.md" && bad "a finished task is still in tray.md" || pass "a finished task leaves tray.md"
tray "$gid" take >/dev/null
grep -q "from the shell" "$TRAY_HOME/garage.md" && bad "a taken line is still in garage.md" || pass "take moves the line from garage.md to tray.md"
grep -q "^- ($gid) from the shell\$" "$TRAY_HOME/tray.md" && pass "with the same id" || bad "tray.md: $(cat "$TRAY_HOME/tray.md")"
before=$(stat -c %Y "$TRAY_HOME/tray.md" 2>/dev/null || stat -f %m "$TRAY_HOME/tray.md")
sleep 1; tray list >/dev/null
after=$(stat -c %Y "$TRAY_HOME/tray.md" 2>/dev/null || stat -f %m "$TRAY_HOME/tray.md")
[ "$before" = "$after" ] && pass "a read rewrites nothing" || bad "list touched the mirror"
teardown

# --- F36 · garage.md read back ---------------------------------------------------------------
# What you type into garage.md on a phone comes in on the next sync: a new bullet is a new
# line, changed words are a rename. Nothing is deleted from here, and tray.md is never read.
head_ "F36 · sync reads garage.md back: new lines and renames, nothing else"
setup
tray dump 'already here' >/dev/null
tray add 'sensitive tray task' pri:H >/dev/null
gid=$(gid_of 2026-08 'already here'); tid=$(id_of 'sensitive tray task')
printf '# garage\n\n## 2026-08\n- from the phone\n- (%s) already here, renamed\n\n## 2026-11\n- for november\n\n## someday\n- (zz9z) with an id tray never gave\n' "$gid" > "$TRAY_HOME/garage.md"
printf '# tray\n\n- (%s) sensitive tray task, edited on the phone\n- a tray line typed on the phone\n' "$tid" > "$TRAY_HOME/tray.md"
out=$(tray sync)
case $out in *"garage.md +2 ~1 ?1"*) pass "the summary counts new, renamed and unknown" ;; *) bad "got: $out" ;; esac
[ "$(garage_json 2026-08 | rows 'from the phone')" = "1" ] && pass "a new bullet is a new line in its month" || bad "phone line missing"
[ "$(garage_json 2026-11 | rows 'for november')" = "1" ] && pass "a heading names the month" || bad "november line missing"
[ "$(garage_json 2026-08 | field 'already here, renamed' id)" = "$gid" ] && pass "changed words rename the task, id kept" || bad "rename lost: $(garage_json 2026-08 | jq -c 'map(.description)')"
[ "$(garage_json someday | jq length)" = "0" ] && pass "an unknown id lands nothing" || bad "unknown id was imported"
[ "$(tray_json | field 'sensitive tray task' id)" = "$tid" ] && [ "$(tray_json | jq length)" = "1" ] \
  && pass "tray.md is never read" || bad "tray.md was read: $(tray_json | jq -c 'map(.description)')"
grep -q "^- ($gid) already here, renamed\$" "$TRAY_HOME/garage.md" && pass "the mirror shows the rename" || bad "garage.md: $(cat "$TRAY_HOME/garage.md")"
grep -q "^- ([0-9a-z][0-9a-z][0-9a-z][0-9a-z]) from the phone\$" "$TRAY_HOME/garage.md" && pass "and the new line has an id now" || bad "phone line has no id"
grep -q "zz9z" "$TRAY_HOME/garage.md" && bad "the unknown line was kept" || pass "the unknown line is gone from the view"
grep -q "^- ($tid) sensitive tray task\$" "$TRAY_HOME/tray.md" && pass "tray.md is rewritten from the store" || bad "tray.md: $(cat "$TRAY_HOME/tray.md")"
grep -v "already here" "$TRAY_HOME/garage.md" > "$TRAY_HOME/g.tmp" && mv "$TRAY_HOME/g.tmp" "$TRAY_HOME/garage.md"
tray sync >/dev/null
[ "$(garage_json 2026-08 | rows 'already here, renamed')" = "1" ] && pass "a missing bullet deletes nothing" || bad "the row was deleted"
grep -q "already here, renamed" "$TRAY_HOME/garage.md" && pass "and comes back into the file" || bad "the row did not return"
case $(tray sync) in *"garage.md +0 ~0"*) pass "a second sync finds nothing new" ;; *) bad "got: $(tray sync)" ;; esac
teardown

# --- F37 · the config file ------------------------------------------------------------------
# Nothing live is in the file yet, and nothing in it is a secret: a plugin's keys live in
# that plugin's settings.json. So the promises are about the file itself — read, tolerant
# of what older files held, honest when malformed — and about the one key that must never
# cross into a plugin from here: a provider key in tray's own environment.
head_ "F37 · the config file is read and tolerant, tray config names it, and no provider key crosses into a plugin"
setup
CFG="$TRAY_HOME/cfg.yaml"
printf 'db:\n  url: "libsql://not-a-setting.example"\nopenrouter:\n  api_key: "sk-or-v1-abcdef9876"\ndates:\n  format: "2 Jan"\n' > "$CFG"
install_plugin envp
TRAY_CONFIG="$CFG" tray dump 'stays in the home' >/dev/null
[ "$(garage_json 2026-08 | rows 'stays in the home')" = "1" ] && pass "a db.url in the file changes nothing: the row is in the home's tray.db" || bad "the row is not in the home"
out=$(TRAY_CONFIG="$CFG" tray config)
case $out in *"$CFG"*) pass "config names the file" ;; *) bad "got: $out" ;; esac
case $out in *"dates.format 2 Jan"*) pass "and reads the one key it has" ;; *) bad "got: $out" ;; esac
case $out in *"abcdef9876"*|*openrouter*|*"db."*) bad "config speaks of keys that are not its own: $out" ;; *) pass "an older file's db and openrouter sections are ignored" ;; esac
seen=$(TRAY_CONFIG="$CFG" OPENROUTER_API_KEY=k OPENROUTER_MODEL=m tray plugin run envp)
# Anchored: `key=` is a prefix of `key=set`, so a glob would pass either way.
printf '%s\n' "$seen" | grep -qx '  + model=unset key=' && pass "a provider key in tray's environment never reaches a plugin" || bad "got: $seen"
printf 'dates: [\n' > "$CFG"
TRAY_CONFIG="$CFG" tray status >/dev/null 2>"$TRAY_HOME/err" && bad "a malformed config was ignored" || pass "a malformed config is an error"
grep -q "$CFG" "$TRAY_HOME/err" && pass "and the error names the file" || bad "error: $(cat "$TRAY_HOME/err")"
teardown

# --- F39 · a plugin may ask for every row ---------------------------------------------------
head_ "F39 · an all-rows plugin reads the whole store; without the marker a plugin reads only its own rows"
setup
install_plugin allrows; install_plugin own
tray dump 'a garage line' >/dev/null
tray add 'a tray task' pri:H >/dev/null
tray dump 'finished already' >/dev/null
tray "$(garage_json 2026-08 | field 'finished already' id)" done >/dev/null
tray plugin run allrows >/dev/null
SEEN="$TRAY_HOME/plugins/allrows/seen.json"
[ "$(jq '.tasks | length' "$SEEN")" = "3" ] && pass "all three rows arrived" || bad "got $(jq -c '.tasks | map(.text)' "$SEEN")"
[ "$(jq -r '.tasks[] | select(.text=="a tray task") | .layer' "$SEEN")" = "tray" ] && pass "the tray row came with its layer" || bad "layer missing"
[ "$(jq -r '.tasks[] | select(.text=="finished already") | .done' "$SEEN")" = "2026-08-07" ] && pass "the done row came with its date" || bad "done row missing or undated"
[ "$(jq -r '[.tasks[] | select(.id != null and (.id|length)==4)] | length' "$SEEN")" = "3" ] && pass "every row carries its id" || bad "ids missing"
[ "$(jq -r '.tasks[] | select(.text=="a garage line") | .key == .id' "$SEEN")" = "true" ] && pass "and is keyed by it" || bad "key is not the id"
tray plugin run own >/dev/null
[ "$(jq '.tasks | length' "$TRAY_HOME/plugins/own/seen.json")" = "0" ] && pass "without the marker a plugin sees only rows it keyed — none" || bad "own saw $(jq '.tasks | length' "$TRAY_HOME/plugins/own/seen.json") rows"
teardown

# --- F40 · plugin is the health view --------------------------------------------------------
# A plugin that fails at sync time, ten minutes in, is a plugin nobody asked. `check`
# asks first, with the exit codes a sync already speaks, and the listing reads the
# folder before the history: an unfinished install or an unfilled form is the state.
head_ "F40 · plugin is the health view"
setup
install_plugin echo; install_plugin allrows; install_plugin healthy; install_plugin ask; install_plugin broken
mkdir -p "$TRAY_HOME/plugins/unconf" "$TRAY_HOME/plugins/halfway" "$TRAY_HOME/plugins/gcal/actions"
printf '#!/bin/sh\necho "{\\"pull\\":[]}"\n' > "$TRAY_HOME/plugins/unconf/sync"; chmod +x "$TRAY_HOME/plugins/unconf/sync"
printf '{"url":""}' > "$TRAY_HOME/plugins/unconf/settings.example.json"
printf '#!/bin/sh\n' > "$TRAY_HOME/plugins/halfway/sync"   # no exec bit
printf '#!/bin/sh\n' > "$TRAY_HOME/plugins/gcal/actions/schedule"; chmod +x "$TRAY_HOME/plugins/gcal/actions/schedule"
row() { tray plugin --json | jq -r --arg n "$1" ".[] | select(.name==\$n) | $2"; }
tray plugin --json | jq -e 'type=="array"' >/dev/null && pass "--json is an array of rows" || bad "not JSON: $(tray plugin --json | head -c 80)"
[ "$(row unconf .state)" = "unconfigured" ] && [ "$(row unconf .settings)" = "missing" ] && pass "an example with no settings.json is unconfigured" || bad "unconf: $(row unconf .state) / $(row unconf .settings)"
[ "$(row halfway .state)" = "half-installed" ] && pass "a folder with no exec bit is half-installed" || bad "halfway: $(row halfway .state)"
[ "$(row gcal '.hooks|join(" ")')" = "verbs: schedule" ] && pass "a verb-only plugin joins the menu and nothing else" || bad "gcal hooks: $(row gcal '.hooks|join(" ")')"
[ "$(row allrows '.hooks|join(" ")')" = "manual all-rows" ] && pass "hooks name manual and all-rows" || bad "allrows hooks: $(row allrows '.hooks|join(" ")')"
ghosts=$(tray plugin --json | jq -r '.[].name' | while read -r n; do [ -d "$TRAY_HOME/plugins/$n" ] || echo "$n"; done)
[ -z "$ghosts" ] && pass "every row is a folder, and nothing else is listed" || bad "listed without a folder: $ghosts"
tray plugin run echo >/dev/null
out=$(tray plugin check)
case $out in *echo*"no probe"*) pass "a plugin without a probe says so" ;; *) bad "got: $out" ;; esac
[ "$(row healthy .state)" = "ok" ] && [ "$(row healthy .check.ok)" = "true" ] && pass "exit 0 reads as ok" || bad "healthy: $(row healthy .state)"
[ "$(row ask .state)" = "needs you — login needed" ] && pass "exit 2 reads as needs you, with the plugin's line" || bad "ask: $(row ask .state)"
[ "$(row broken .state)" = "failed — boom" ] && pass "exit 1 reads as failed, with the plugin's line" || bad "broken: $(row broken .state)"
[ "$(row echo .sync.hook)" = "manual" ] && [ "$(row echo .sync.ok)" = "true" ] && pass "check left the last sync run in place" || bad "echo sync run lost: $(row echo .sync)"
case $(row echo .last) in check*) pass "and the newest thing it did is the check" ;; *) bad "last: $(row echo .last)" ;; esac
tray plugin check nope >/dev/null 2>&1 && bad "checking an unknown plugin passed" || pass "an unknown plugin is an error"
teardown

printf '\n'
[ "$fail" = 0 ] && printf '\033[32mtray flows pass\033[0m\n' || printf '\033[31mtray flows FAILED\033[0m\n'
exit "$fail"
