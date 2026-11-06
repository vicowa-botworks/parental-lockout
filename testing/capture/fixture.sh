#!/usr/bin/env bash
# Shared fixture plumbing for the 360 px captures. Sourced, never run.
#
# A fixture is a throwaway /etc/plock: its own config, its own state directory, its own audit
# trail, on a port nothing else uses. Nothing here writes outside testing/capture/runs/, and
# nothing here needs root — the daemon is pointed at the command backend with /bin/true, so a
# capture cannot lock the machine it is taken on. That is the whole reason a screenshot can be
# taken by a script at all: the alternative is a test that locks the operator's screen.
#
# Each *-setup.sh keeps its own config JSON, because the config IS the fixture: which rows exist
# and how wide they are is the thing being measured. What is shared is the plumbing around it.

# Where things live, resolved from this file so a capture can be run from any directory.
CAPTURE_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CAPTURE_REPO="$(cd "$CAPTURE_DIR/../.." && pwd)"

# The binary under test. Default is the repo's own build; PLOCK_BIN points at another one, which
# is how the negative control runs a binary built with a clause removed.
PLOCK_BIN="${PLOCK_BIN:-$CAPTURE_REPO/plock}"

# The account the capture signs in as, and the account the POST-only pages are captured for.
#
# The two POST fixtures use the *current user's* name, not a name invented here, and that is
# load-bearing rather than convenient: the result page lists the sessions an account holds on
# this machine, and a config-only name holds none. The page would then render its zero-session
# sentence and the capture would photograph an edge case instead of the table — while still
# reporting 360 px, because an empty table fits at any width. The driver refuses an empty table;
# this is how the fixture avoids being the reason it fires.
#
# Run the capture as the desktop user, not through sudo: under sudo this resolves to root, which
# holds no desktop session, and the refusal is the correct answer.
CAPTURE_ADMIN="${PLOCK_CAPTURE_ADMIN:-parent}"
CAPTURE_PASS="${PLOCK_CAPTURE_PASS:-hunter2-hunter2}"
# Overridable for the one case where the account running the capture is not the account holding
# the desktop session -- a headless box, or an operator logged in under a different name. It is
# not a way to make the POST captures work without a session: the driver refuses an account with
# no rows, and that refusal is the correct answer, because a capture of the zero-session sentence
# would still report 360 px.
CAPTURE_TARGET="${PLOCK_CAPTURE_TARGET:-$(id -un)}"

# capture_begin <name> <port> <cdp-port>
#
# Prepares the run directory and exports the paths the fixture's own heredocs write to. The state
# directory and the audit trail are rebuilt every run — a stale trail changes what the audit page
# shows — but the Chrome profile is kept, because a fresh profile makes Chrome ask questions
# nobody is there to answer.
capture_begin() {
  local name="$1" port="$2" cdp="$3"
  RUN_NAME="$name"
  RUN_DIR="$CAPTURE_DIR/runs/$name"
  CONFIG="$RUN_DIR/config.json"
  STATE="$RUN_DIR/state"
  AUDIT="$RUN_DIR/audit.jsonl"
  ENVFILE="$RUN_DIR/fixture.env"
  RUN_PORT="$port"
  RUN_CDP="$cdp"

  [ -x "$PLOCK_BIN" ] || {
    echo "no binary at $PLOCK_BIN" >&2
    echo "run: go build -o plock ./cmd/plock   (in $CAPTURE_REPO)" >&2
    echo "or set PLOCK_BIN=/path/to/plock" >&2
    exit 1
  }

  # Tight before creating anything. The daemon refuses to work in a state directory whose own
  # directory or parent is group-writable — a directory somebody else can reach is a directory
  # somebody else can swap out from under a root daemon — and this machine's default umask creates
  # 0775 directories. The fixture satisfies the check rather than arguing with it.
  umask 077
  mkdir -p "$RUN_DIR"
  rm -rf "$STATE"
  mkdir -p "$STATE"
  echo "fixture: $RUN_DIR  binary: $PLOCK_BIN  listen: 127.0.0.1:$port  target: $CAPTURE_TARGET"
}

# capture_admin
#
# The admin password is stored as a PBKDF2 verifier, never as the word itself, so a throwaway
# config has to be given one the way a real install would.
capture_admin() {
  printf '%s\n' "$CAPTURE_PASS" | "$PLOCK_BIN" init --username "$CAPTURE_ADMIN" \
    --password-stdin --force --config "$CONFIG" --state-dir "$STATE"
}

# The instant the fixture's document claims to have been built at, and the zone it claims to be
# in. Declared rather than read from the host, because a photograph is a function of the clock it
# was taken on: `datetime.now()` in a setup script puts the wall clock into the audit trail's `at`
# column, and `.astimezone()` with no argument puts the *host's zone* into it as well. Two honest
# runs then differ by the minutes between them and by whoever's machine ran them, which is
# issue #207's cause 1 — the reason `audit.png` could not be re-taken.
#
# 2026-03-18 21:34:05 at -04:00 is a Wednesday evening in America/New_York after that year's
# spring-forward, so it is a plain offset — the fixture does not want a fold or a jump in the frame
# — and an evening, which is when a curfew page means something.
#
# Overridable for exactly one reason: proving the capture is clock-independent. Two runs that set
# different PLOCK_CAPTURE_STAMP values and still produce identical PNGs are the claim; nothing
# should be run with these set to anything else.
CAPTURE_STAMP="${PLOCK_CAPTURE_STAMP:-2026-03-18T21:34:05}"
CAPTURE_ZONE="${PLOCK_CAPTURE_ZONE:--04:00}"

# How many rows the trail holds once the driver has signed in, and therefore how many the audit page
# has to render. Declared by the setup script that seeds a trail (pages-setup.sh); empty means this
# fixture photographs no trail. The driver refuses a page that does not match the number, because
# the alternative — measured, README'd — was a PNG whose height was a run counter: every sign-in
# appends an `auth.success` row to the very trail the page photographs, and the committed file had
# reached 360x15004 while a fixture built from nothing rendered 2936 px (#207's cause 2).
CAPTURE_AUDIT_ROWS="${PLOCK_CAPTURE_AUDIT_ROWS:-}"

# The census the daemon is allowed to see. Empty means "ask the machine", which is what the two POST
# fixtures do on purpose — lock.png and testlock.png are evidence about a real lock on a real
# session, and a stubbed census would photograph a fiction. A page fixture that wants its Sessions
# cell to be the fixture's names bin here; see testing/capture/bin/loginctl for what it answers and
# why one account holds a session and the other holds none.
CAPTURE_PATH_PREPEND="${PLOCK_CAPTURE_PATH_PREPEND:-}"

# The zone the *reader* is in (#207's seventh cause, measured here rather than assumed).
# internal/web/templates/audit.tmpl used to format each row with `MST`, and Go's `time.Parse` adopts
# the process's local Location when a row's offset matches it -- so one and the same trail rendered
# `2026-03-18 21:33:05 EDT` on a machine in America/New_York and `2026-03-18 21:33:05 -0400` on one
# in UTC or Asia/Kolkata. Three runs of a nine-line program under `TZ=` each, output verbatim in the
# issue. The instant and the offset survived all three; only the label moved, and #210 replaced the
# label with the offset the row itself carries. This variable therefore decides no text on the page
# any more -- the two paragraphs below are what survives of the reason it was introduced, and both
# were rewritten on #210 for claiming more than that.
#
# It stays declared, as a canary, and the reason is narrower than the one this block gave until #210.
# Nothing in the render path asks the host anything now: the audit page's When cell prints the
# value's own offset, every clock on the dashboard comes from the profile's own zone -- which both
# fixture profiles name in `pages-setup.sh` -- and `wallClock` in `internal/dashboard/banner.go`
# renders the instant in the location it was computed in. The one place that still falls back to
# `time.Local` is `schedule.Location`, for a profile whose zone is absent or unloadable, and that is
# a property of the rule data rather than of the machine running the driver. What a declared zone
# buys is therefore one thing: if a host-dependent clock ever comes back, this is the knob that makes
# the frame disagree with the manifest's declared state instead of quietly agreeing with whatever
# host happened to run it.
#
# It is still not validated, and now nothing downstream would notice either. Go falls back to UTC for
# a zone it cannot load, and a column that prints the value's own offset renders `-04:00` under that
# fallback exactly as it does under a zone that loaded -- that equality is what the UTC case of
# `TestTheAuditRowIsStampedTheSameWhoeverReadsIt` holds. So a misspelt `PLOCK_CAPTURE_TZ` is silent
# here, and no longer something the audit page's declared state can catch. What still refuses is the
# *instant*: `cdp.pin_appended_rows` re-stamps the sign-in's row to the fixture's declared time and
# refuses any other count or kind.
CAPTURE_TZ="${PLOCK_CAPTURE_TZ:-}"

# Whether the PAGE's own scripts ran while the frame was taken (#211).
#
# Recorded here rather than read off the driver, because it is a property of the frame and not of the
# code: what the driver does today is not evidence about pixels somebody took earlier, and a fixture
# that stayed silent about it would let a reader fill the gap in from `cdp.py` -- which is the defect
# #211 closes, one layer down.
#
# The first version of this note claimed the two POST artifacts predated #207 and had to be declared
# `on` until #213 re-took them. The second version claimed the opposite from nine reproduced digests.
# Both outran their evidence, in opposite directions: the pixels in question cannot distinguish the two
# values, because neither POST result page renders a `time[data-unix]` element and rewriting those is
# the only load-time write in `static/app.js`. Measured 6 Nov with the declaration wired through to the
# browser (`cdp.scripts_disabled`, D-0025 — it was `value=True` before that, whatever a fixture
# declared): `on` and `off` give one PNG apiece for both frames. What decides which condition a frame
# was taken under is the run that took it, so the fix was to take the frames again rather than to pick
# a value — and the lesson that governs the whole field is the narrower one: a digest can show a frame
# is *consistent with* a condition, which is not the same as its witnessing it.
#
# A fixture that declares nothing about this gets no `scripts` key in its manifest entry. Absent means
# "this fixture did not choose", exactly as it does for the four optional keys above; it never means
# `off` by default, because the whole defect #211 closes is a reader filling in a condition nobody
# stated. Say the half that asymmetry hides, because this is the one field where the two halves are
# reached by different routes: a fixture that names nothing still gets an `off` *frame*, because
# `${PLOCK_CAPTURE_SCRIPTS:-off}` below defaults it and `cdp.scripts_disabled` reads an absent
# declaration as the criterion's own condition — so `pages-noprof`'s frame was taken with the scripts
# off while its entry says nothing about it. `rows` and `tz` are absent there because nobody chose;
# `scripts` is absent because nobody wrote it down, not because anything was left running.
CAPTURE_SCRIPTS="${PLOCK_CAPTURE_SCRIPTS:-off}"

# capture_env
#
# The fixture's own answers, in one file the driver reads. Keeping the port, the account and the
# password here rather than in both halves means a fixture and its driver cannot disagree about
# which daemon the browser should be pointed at — and a browser aimed at a daemon that is not the
# one just started photographs a different machine's admin UI.
#
# The four optional keys below are written only when declared. A driver reads a missing one as "this
# fixture did not choose", never as zero or as the empty string: a fixture that says nothing about
# its clock must not be silently re-stamped to a date somebody else chose.
capture_env() {
  {
    echo "# written by $RUN_NAME-setup.sh; the driver reads this and refuses without it"
    echo "PLOCK_CAPTURE_USER=$CAPTURE_ADMIN"
    echo "PLOCK_CAPTURE_PASS=$CAPTURE_PASS"
    echo "PLOCK_CAPTURE_TARGET=$CAPTURE_TARGET"
    echo "PLOCK_CAPTURE_PORT=$RUN_PORT"
    echo "PLOCK_CAPTURE_CDP=$RUN_CDP"
    if [ -n "$CAPTURE_STAMP" ]; then echo "PLOCK_CAPTURE_STAMP=$CAPTURE_STAMP"; fi
    if [ -n "$CAPTURE_ZONE" ]; then echo "PLOCK_CAPTURE_ZONE=$CAPTURE_ZONE"; fi
    if [ -n "$CAPTURE_AUDIT_ROWS" ]; then echo "PLOCK_CAPTURE_AUDIT_ROWS=$CAPTURE_AUDIT_ROWS"; fi
    if [ -n "$CAPTURE_PATH_PREPEND" ]; then echo "PLOCK_CAPTURE_PATH_PREPEND=$CAPTURE_PATH_PREPEND"; fi
    if [ -n "$CAPTURE_TZ" ]; then echo "PLOCK_CAPTURE_TZ=$CAPTURE_TZ"; fi
    if [ -n "$CAPTURE_SCRIPTS" ]; then echo "PLOCK_CAPTURE_SCRIPTS=$CAPTURE_SCRIPTS"; fi
  } > "$ENVFILE"
  # The last line is not decoration: every line above is a condition, and the setup scripts run
  # under `set -euo pipefail` (fixture.sh is sourced, so it inherits that). A `[ -n ] && echo` list
  # that takes the false arm is a failing command list, which under -e ends the script one line
  # before it wrote a file nobody expected to be missing.
  return 0
}

capture_done() {
  local events=0
  [ -f "$AUDIT" ] && events="$(wc -l < "$AUDIT")"
  # The fixture's own arithmetic, checked at the bench rather than at the browser. Declared rows
  # include the one the sign-in appends, so a seeded trail one line short of the declaration is
  # either a dropped seed row or a number that was not updated when a row was removed — and either
  # way the driver's refusal would name the audit page instead of the heredoc that caused it.
  if [ -n "$CAPTURE_AUDIT_ROWS" ] && [ "$events" -ne $((CAPTURE_AUDIT_ROWS - 1)) ]; then
    echo "REFUSED: $AUDIT holds $events seeded rows, and this fixture declares $CAPTURE_AUDIT_ROWS" >&2
    echo "rows on the page including the one the sign-in appends. Fix $RUN_NAME-setup.sh: add the" >&2
    echo "row back, or lower the number it declares -- the page is photographed against it." >&2
    exit 1
  fi
  echo "wrote $CONFIG and $AUDIT ($events events); fixture.env names port $RUN_PORT"
  if [ -n "$CAPTURE_PATH_PREPEND" ]; then
    echo "census: $CAPTURE_PATH_PREPEND on the daemon's PATH, not this machine's loginctl"
  fi
}
