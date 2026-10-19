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

# capture_env
#
# The fixture's own answers, in one file the driver reads. Keeping the port, the account and the
# password here rather than in both halves means a fixture and its driver cannot disagree about
# which daemon the browser should be pointed at — and a browser aimed at a daemon that is not the
# one just started photographs a different machine's admin UI.
capture_env() {
  {
    echo "# written by $RUN_NAME-setup.sh; the driver reads this and refuses without it"
    echo "PLOCK_CAPTURE_USER=$CAPTURE_ADMIN"
    echo "PLOCK_CAPTURE_PASS=$CAPTURE_PASS"
    echo "PLOCK_CAPTURE_TARGET=$CAPTURE_TARGET"
    echo "PLOCK_CAPTURE_PORT=$RUN_PORT"
    echo "PLOCK_CAPTURE_CDP=$RUN_CDP"
  } > "$ENVFILE"
}

capture_done() {
  local events=0
  [ -f "$AUDIT" ] && events="$(wc -l < "$AUDIT")"
  echo "wrote $CONFIG and $AUDIT ($events events); fixture.env names port $RUN_PORT"
}
