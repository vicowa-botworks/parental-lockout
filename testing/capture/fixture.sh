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

  # The binary is a photograph of the tree it was built from, and the templates and stylesheet are
  # compiled into it (`internal/web/sources.go:21`, `go:embed`), so a binary that predates a markup
  # edit serves the page that binary was built from. The browser then measures what the daemon
  # actually said, `scrollWidth` agrees, the needles agree because the needles are about other cells,
  # and `manifest.py record` certifies the result against the sources on disk — which name the NEW
  # template. Every check downstream of the browser is downstream of the wrong page.
  #
  # Measured 5 Nov on #219: a binary built nineteen minutes before the commit that changed the
  # masthead re-took the committed set and left five of the seven page-fixture frames byte-identical
  # to the pre-change files, moving only the two whose fixture CONTENT had changed. That is a set
  # which reads as "the two this branch moved" and would have been committed as evidence about a page
  # nobody photographed. Existence was the check here; it is not the check that matters.
  #
  # `*_test.go` is excluded because it is not compiled into plock — a guard that fires on a test
  # edit demands a rebuild that changes nothing, and a check that cries wolf gets edited out inside a
  # week. `go.mod` is asked for because a dependency change moves a byte of the binary.
  local -a built_from=("$CAPTURE_REPO/cmd" "$CAPTURE_REPO/internal")
  # Both halves of that condition are load-bearing. This repository has no third-party dependency, so
  # there is no `go.sum` to ask about, and `find` exits 1 on a path that does not exist: under the
  # setup scripts' `set -euo pipefail` that ends the run AT THE ASSIGNMENT, one line before the
  # message that would have explained it. Measured on the first version of this guard, which exited 1
  # with nothing on stderr and looked, from the outside, like a fixture that had simply stopped.
  if [ -f "$CAPTURE_REPO/go.mod" ]; then built_from+=("$CAPTURE_REPO/go.mod"); fi
  if [ -f "$CAPTURE_REPO/go.sum" ]; then built_from+=("$CAPTURE_REPO/go.sum"); fi
  local stale
  stale="$(find "${built_from[@]}" -type f ! -name '*_test.go' -newer "$PLOCK_BIN" \
              -printf '  %p\n')"
  if [ -n "$stale" ]; then
    echo "REFUSED: the binary at $PLOCK_BIN predates the sources compiled into it:" >&2
    printf '%s\n' "$stale" | sed -n '1,4p' >&2
    echo "and the pages are embedded in it, so a capture taken against it photographs a page this" >&2
    echo "tree does not contain -- and manifest.py record would then certify the new sources for it." >&2
    echo "run: go build -o plock ./cmd/plock   (in $CAPTURE_REPO)" >&2
    exit 1
  fi

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

# The zone the *reader* is in, which the audit page's When column depends on (#207's seventh cause,
# measured here rather than assumed). internal/web/templates/audit.tmpl:86 formats each row with
# `MST`, and Go's `time.Parse` adopts the process's local Location when a row's offset matches it --
# so one and the same trail rendered `2026-03-18 21:33:05 EDT` on a machine in America/New_York and
# `2026-03-18 21:33:05 -0400` on one in UTC or Asia/Kolkata. Three runs of a nine-line program under
# `TZ=` each, output verbatim in the issue. The instant and the offset survive all three; it is the
# label that belongs to whoever is reading.
#
# Declaring it here is what makes the frame the same bytes on every host. It does not make the label
# correct: a trail row's zone abbreviation is the reader's, not the row's, and that is filed as its
# own finding rather than fixed by a fixture, because the fix belongs in the template and the
# decision about what a parent should see (an offset, or a name the writer chose) is not this
# folder's to make. An unparseable TZ is not diagnosed here -- Go falls back to UTC, which prints
# `-0400`, which the audit page's declared state then refuses. That is the right place to notice.
CAPTURE_TZ="${PLOCK_CAPTURE_TZ:-}"

# Whether the page's own JavaScript was running when the frame was taken (#211).
#
# `off` is not a preference being recorded here, it is what the driver does: cdp.py disables page
# scripts (`Emulation.setScriptExecutionDisabled`) after the sign-in and before the first measurement,
# because `static/app.js` rewrites every `time[data-unix]` cell from `Date.now()` — so a frame taken
# with the scripts running carries a live countdown rather than the server's sentence — and because
# this folder's criterion is "usable at 360 px with JavaScript off", which a frame taken with them on
# has never tested.
#
# So why write it down at all, when the driver allows only one answer? Because the manifest is now a
# committed claim about a frame rather than about a commit, and the one condition a reader cannot
# recover from the code is the one that has to travel with the bytes: the frames committed before
# #221's re-take were taken with the scripts RUNNING, and every check in this repository called them
# current. The driver refuses a fixture that declares anything else, which is what keeps this word
# from becoming a value the fixture asserts and the driver ignores. A run whose env predates the key
# declares nothing, and the manifest leaves `scripts` out rather than guessing.
CAPTURE_SCRIPTS="${PLOCK_CAPTURE_SCRIPTS:-off}"

# capture_env
#
# The fixture's own answers, in one file the driver reads. Keeping the port, the account and the
# password here rather than in both halves means a fixture and its driver cannot disagree about
# which daemon the browser should be pointed at — and a browser aimed at a daemon that is not the
# one just started photographs a different machine's admin UI.
#
# The five optional keys below are written only when declared. A driver reads a missing one as "this
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
    echo "PLOCK_CAPTURE_SCRIPTS=$CAPTURE_SCRIPTS"
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
