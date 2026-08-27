#!/usr/bin/env bash
# User-flow tests for tray. Drives the real binary against a sandboxed TRAY_HOME
# and asserts file contents. stdin is closed throughout, so any prompt fails here
# rather than hanging.
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

has()   { grep -qF -- "$2" "$TRAY_HOME/$1"; }
count() { grep -cF -- "$2" "$TRAY_HOME/$1" 2>/dev/null || true; }
# Ids come off the default view, which is what a user would be reading.
id_of() { tray | grep -F -- "$1" | awk '{print $1}' | head -1; }

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

# jq is a core dependency; python3 is the fallback for a bare checkout.
if command -v jq >/dev/null 2>&1; then valid_json() { jq -e . >/dev/null 2>&1; }
elif command -v python3 >/dev/null 2>&1; then valid_json() { python3 -m json.tool >/dev/null 2>&1; }
else valid_json() { grep -q .; }
fi

# macOS ships no timeout(1); coreutils installs it prefixed. Neither is required.
if command -v timeout >/dev/null 2>&1; then limit() { timeout 5 "$@"; }
elif command -v gtimeout >/dev/null 2>&1; then limit() { gtimeout 5 "$@"; }
else limit() { "$@"; }
fi

# --- F1 · init then dump ------------------------------------------------------
head_ "F1 · capture"
setup
out=$(tray dump add metrics to the worker)
[ -f "$TRAY_HOME/2026-08.md" ] && pass "month file created" || bad "no month file"
has 2026-08.md "- add metrics to the worker" \
  && pass "line is verbatim" || bad "line mangled: $(cat "$TRAY_HOME/2026-08.md")"
head -1 "$TRAY_HOME/2026-08.md" | grep -q '^# 2026-08$' \
  && pass "header written" || bad "no header"
case $out in *2026-08*) pass "reports the month" ;; *) bad "unexpected: $out" ;; esac

# --- F2 · dump with month and tag --------------------------------------------
head_ "F2 · month + tag"
tray dump to:2026-11 +infra add metrics to the worker >/dev/null
has 2026-11.md "- add metrics to the worker +infra" \
  && pass "lands in 2026-11 with +infra" || bad "got: $(cat "$TRAY_HOME/2026-11.md")"

# --- F3 · arbitrary text is valid --------------------------------------------
head_ "F3 · jottpad tolerance"
tray dump '?? that config thing — does it even matter: probably not' >/dev/null
has 2026-08.md '?? that config thing — does it even matter: probably not' \
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
[ "$(id_of 'Urgent thing')" = "1" ] && pass "id 1 is the most urgent" || bad "ids not canonical"
has tray.md "entry:2026-08-07" && pass "entry: stamped" || bad "no entry:"
tray list | grep -q "2026-08-08 Sat" \
  && pass "reports show the weekday" || bad "no weekday: $(tray list | sed -n 2p)"
grep -q "Sat" "$TRAY_HOME/tray.md" && bad "the weekday leaked into the file" \
  || pass "the file stays plain ISO"
has tray.md "- [ ] Urgent thing priority:H due:2026-08-08" \
  && pass "attrs serialise in a stable order" || bad "got: $(grep Urgent "$TRAY_HOME/tray.md")"

# --- F5 · take ---------------------------------------------------------------
head_ "F5 · take is a transformation"
tray dump add retries to the sync job >/dev/null
tray 1 take pri:H >/dev/null
has tray.md "add retries to the sync job" && pass "lands in the tray" || bad "not in tray"
has tray.md "from:2026-08" && pass "from: stamped" || bad "no from:"
has 2026-08.md "add retries to the sync job → tray" \
  && pass "source annotated → tray" || bad "source not annotated"
tray 1 take >/dev/null 2>&1
[ "$(count tray.md 'add retries to the sync job')" = "1" ] \
  && pass "cannot be taken twice" || bad "duplicated in the tray"
# `take` with no id used to panic on an empty id spec.
out=$(tray take 2>&1); code=$?
case $out in *"which one"*) pass "bare take asks which one" ;; *) bad "got: $out" ;; esac
[ "$code" = "0" ] && pass "bare take exits cleanly" || bad "bare take exited $code"
case $(tray add Structureless thing) in
  *"no pri"*) pass "a bare tray task says what it still wants" ;;
  *) bad "no nudge when structure is missing" ;;
esac

# --- F6 · done and drop ------------------------------------------------------
head_ "F6 · done · drop"
tray add Renew the passport pri:H >/dev/null
tray "$(id_of 'Renew the passport')" "done" >/dev/null
has tray.md "~~Renew the passport~~" && pass "struck through in place" || bad "not struck"
has tray.md "done:2026-08-07" && pass "done: dated" || bad "no done:"
tray add Dead idea >/dev/null
tray "$(id_of 'Dead idea')" drop >/dev/null
has tray.md "dropped:2026-08-07" && pass "drop is terminal, not done" || bad "no dropped:"
tray | grep -q "Dead idea" && bad "dropped item still in the default report" \
  || pass "dropped item leaves the report"

# --- F7 · unload, twice ------------------------------------------------------
head_ "F7 · unload is idempotent"
tray unload --to 2026-08 >/dev/null
has 2026-08.md "~~Renew the passport~~" && has 2026-08.md "done:2026-08-07" \
  && pass "done items land struck and dated" || bad "done item lost its strike"
has 2026-08.md "priority:H" && pass "open items keep their attrs" || bad "attrs dropped"
[ "$(grep -c '^- ' "$TRAY_HOME/tray.md")" = "0" ] \
  && pass "tray emptied" || bad "tray not empty"
head -1 "$TRAY_HOME/tray.md" | grep -q "^# tray$" && pass "header survives" || bad "header lost"
cp "$TRAY_HOME/2026-08.md" "$TRAY_HOME/.snap"
tray unload --to 2026-08 >/dev/null
diff -q "$TRAY_HOME/.snap" "$TRAY_HOME/2026-08.md" >/dev/null \
  && pass "second unload is a no-op" || bad "second unload rewrote the month"
teardown

# --- F8 · carryover ----------------------------------------------------------
head_ "F8 · carryover copies forward"
setup
TRAY_TODAY=2026-07-15 tray dump July leftover one >/dev/null
TRAY_TODAY=2026-07-15 tray dump July leftover two >/dev/null
TRAY_TODAY=2026-07-15 tray dump 'July dated thing due:2026-07-20' >/dev/null
tray add 'still working on this' pri:H >/dev/null
tray carryover --run --month 2026-07 >/dev/null
has 2026-08.md "- July leftover one" && pass "copied into August" || bad "not copied"
has 2026-07.md "July leftover one → 2026-08" && pass "source annotated" || bad "source clean"
[ "$(count 2026-07.md 'July leftover one')" = "1" ] \
  && pass "source kept exactly once" || bad "source duplicated"
has tray.md "still working on this" \
  && pass "the tray is left alone — unload is its own ritual" \
  || bad "carryover emptied the tray behind your back"
has 2026-08.md "July dated thing" && ! has 2026-08.md "due:2026-07-20" \
  && pass "a due date that already passed is not carried" \
  || bad "carried a stale due:\n$(cat "$TRAY_HOME/2026-08.md")"
has 2026-07.md "due:2026-07-20" \
  && pass "the source still records what the date was" || bad "source lost the date"
out=$(tray carryover --run --month 2026-07)
case $out in *"nothing to carry"*) pass "second run finds nothing live" ;; *) bad "got: $out" ;; esac

# --- F9 · hand-edit tolerance ------------------------------------------------
head_ "F9 · one document, two hands"
{
  echo ""
  echo "## notes to self"
  echo "this paragraph is mine and must survive"
  echo "* a star bullet with weird: punctuation"
  echo "- "
} >> "$TRAY_HOME/2026-08.md"
cp "$TRAY_HOME/2026-08.md" "$TRAY_HOME/.snap"
tray add Something else pri:M >/dev/null
tray dump another line >/dev/null
tray unload --to 2026-08 >/dev/null
grep -q "this paragraph is mine and must survive" "$TRAY_HOME/2026-08.md" \
  && pass "prose survives a write cycle" || bad "prose lost"
grep -q "a star bullet with weird: punctuation" "$TRAY_HOME/2026-08.md" \
  && pass "star bullet and stray colon survive" || bad "star bullet mangled"
grep -q "^## notes to self$" "$TRAY_HOME/2026-08.md" \
  && pass "hand-written heading survives" || bad "heading lost"

# --- F10 · export ------------------------------------------------------------
head_ "F10 · export"
tray add Exportable pri:H due:2026-08-12 +infra >/dev/null
tray export | valid_json \
  && pass "valid JSON" || bad "invalid JSON: $(tray export | head -3)"
tray export | grep -q '"status": "pending"' && pass "TW status field" || bad "no TW status"
tray export | grep -q '"due": "20260812T000000Z"' \
  && pass "TW date stamps" || bad "date not TW-shaped"
tray export | grep -q '"tags"' && pass "tags exported" || bad "tags missing"

# --- F11 · filters -----------------------------------------------------------
head_ "F11 · filters"
tray +infra list | grep -q Exportable && pass "+tag filter" || bad "+tag filter broken"
tray +nope list | grep -q Exportable && bad "+tag filter matched wrongly" \
  || pass "non-matching tag excludes"
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
case $out in *priority:*|*due:*) bad "v1 print leaked attrs" ;; *) pass "no attrs in v1 output" ;; esac
case $out in *"**infra**"*) pass "grouped by tag" ;; *) bad "not grouped: $out" ;; esac
bare=$(tray)
case $bare in *"**infra**"*) pass "bare tray groups the same way" ;; *) bad "got: $bare" ;; esac
case $bare in *priority:*|*URG*) bad "bare tray leaked the table" ;; *) pass "no table, no attrs" ;; esac
case $bare in *"- [ ]"*) bad "bare tray has journal checkboxes" ;; *) pass "ids instead of checkboxes" ;; esac
[ -n "$(id_of Exportable)" ] && pass "ids are on screen" || bad "no id to type"

# --- F14 · ids after a hand reorder ------------------------------------------
head_ "F14 · ids survive a hand reorder"
tray add Alpha pri:L >/dev/null
tray add Beta pri:H >/dev/null
top=$(tray list | sed -n '2p')
awk '/^- /{ b[++n] = $0; next } { print } END { for (i = n; i >= 1; i--) print b[i] }' \
  "$TRAY_HOME/tray.md" > "$TRAY_HOME/.reordered" && mv "$TRAY_HOME/.reordered" "$TRAY_HOME/tray.md"
now=$(tray list | sed -n '2p')
[ "$top" = "$now" ] && pass "report order is urgency, not file order" || bad "order changed with the file"
tray "$(id_of Beta)" "done" >/dev/null
grep -q "~~Beta~~" "$TRAY_HOME/tray.md" && pass "id hits the right line" || bad "wrong line marked"
teardown

# --- F15 · find across layers ------------------------------------------------
head_ "F15 · find is the rot signal"
setup
for m in 2026-05 2026-06 2026-07; do
  tray dump "to:$m" add retries to the sync job >/dev/null
done
out=$(tray find retries)
case $out in *2026-05*2026-06*2026-07*) pass "hits every month" ;; *) bad "got: $out" ;; esac
case $out in *"3 months — rot signal"*) pass "flags the rot" ;; *) bad "no rot flag: $out" ;; esac
tray add add retries to the sync job pri:H >/dev/null
tray find retries | grep -q "^tray" && pass "searches the tray too" || bad "tray layer missed"
case $(tray find nothingmatchesthis) in "no match") pass "empty search" ;; *) bad "no-match wrong" ;; esac
teardown

# --- F16 · agent surface never prompts ---------------------------------------
head_ "F16 · headless"
setup
for verb in "" "list" "garage list" "status" "export" "print" "--json list"; do
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
# F7 covers a tray whose tasks never came from this month, so it only ever
# exercised the copy path. This is the common one: dump here, take it, hand it
# back. It goes home to the line it left, which must not undo what the tray added.
head_ "F17 · unload brings the tray home whole"
setup
tray dump ship the release notes >/dev/null
tray dump finish the migration >/dev/null
tray garage 1 take pri:H due:2026-08-20 +work >/dev/null
tray garage 1 take pri:L >/dev/null
tray 1 done >/dev/null
tray unload --to 2026-08 >/dev/null

[ "$(count 2026-08.md 'ship the release notes')" = "1" ] \
  && pass "one line home, not a copy beside the one it left" \
  || bad "duplicated: $(cat "$TRAY_HOME/2026-08.md")"
has 2026-08.md "~~ship the release notes~~" \
  && pass "a finished task lands struck through" \
  || bad "finished task came home open — carryover would carry it forever"
has 2026-08.md "done:2026-08-07" && pass "and dated" || bad "no done date"
grep -q "ship the release notes~~ priority:H due:2026-08-20" "$TRAY_HOME/2026-08.md" \
  && pass "a finished task keeps what the tray gave it" || bad "attrs dropped"
grep -q "^- finish the migration priority:L" "$TRAY_HOME/2026-08.md" \
  && pass "an open task keeps its attrs, so taking it again is free" \
  || bad "open task came home bare: $(cat "$TRAY_HOME/2026-08.md")"
has 2026-08.md "from:" && bad "from: is noise on a line that lives here" \
  || pass "from: dropped on the way home"
[ "$(count 2026-08.md '→ tray')" = "0" ] \
  && pass "no line still points at the tray" || bad "stale arrow left behind"
teardown


# Two flows here once drove the fzf keymap and gum's pickers. Both are gone: `/`
# is in-process now and the month picker is bubbletea. The interface is covered by
# the teatest flows in internal/ui — see FLOWS.md.

# --- F18 · nothing is inferred headlessly -------------------------------------
head_ "F18 · nothing is inferred headlessly"
setup
tray carryover >/dev/null 2>&1 && bad "bare carryover should fail when piped" \
  || pass "bare carryover refuses without a terminal"
tray carryover --run >/dev/null 2>&1 && bad "--run should need a month" \
  || pass "--run refuses to guess the month"
tray unload >/dev/null 2>&1 && bad "bare unload should fail when piped" \
  || pass "unload refuses to guess the month"
tray garage list --nope >/dev/null 2>&1 && bad "an unknown flag was swallowed" \
  || pass "an unknown flag is an error, not a silence"
teardown

printf '\n'
[ "$fail" = 0 ] && printf '\033[32mtray flows pass\033[0m\n' || printf '\033[31mtray flows FAILED\033[0m\n'
exit "$fail"
