# shellcheck shell=bash
# shellcheck disable=SC2016,SC2034,SC2154
# 469-pinned-fetch-retry.sh -- the verify job's pinned fetches must retry the error
# class that actually happens, and the tz fetch must fall back across hosts (#469).
#
# #469 measured three attempts dead in `Install the pinned tz database` on 19 Nov --
# `curl: (56) Recv failure: Connection reset by peer`, step `Verify` never executed --
# while the step's `--retry 3` could not reach that failure shape: --retry alone
# re-scores HTTP transients, a mid-body RST is transport (fixture rows A/B on curl
# 8.18.0: rc 56 on ONE connection with the old flags, FOUR with --retry-all-errors
# added, and a 503 retried four ways under the old flags, so the machinery was live
# and the error class is the whole story). And the degraded thing was one object on
# one hostname: us.archive.ubuntu.com served both debs byte-identical to the pinned
# digests. The step now fetches each half through a host list behind the unchanged
# digest gate. The ticket left one thing NOT established: the runner's curl build
# was never measured. This leg measures it on every run: it extracts the shipped
# fetch_pinned from .github/workflows/ci.yml and points it at a mid-body-RST
# fixture, and prints the connection count that number IS.
#
# What the legs pin, and the polarities:
#   (a) the extraction premise and the string pins: ci.yml carries a fetch_pinned
#       function that parses; --retry-all-errors on all three pinned fetch lines;
#       the four digest gates still in place (the sha256 check is what decides a
#       fetch -- no fallback may ever be able to fetch a different version);
#   (b) the fixture control FIRST: the step's OLD flags (`-sSfL --retry 3`) against
#       the mid-body-RST fixture spend exactly ONE connection -- if this goes red
#       the fixture stopped reproducing the ticket's shape and leg (c) is vacuous;
#   (c) the runner-build measurement: the shipped function, verbatim flags, at the
#       same fixture -- failure reported, more than one connection spent (the count
#       is printed), and no partial body left behind for the digest check to chew;
#   (d) the fallback: host one RSTs mid-body, host two serves -- rc 0, the bytes are
#       host two's, and the served-by line names the host that served (option 3 of
#       the ticket: a future red run names the degraded host instead of guessing);
#   (e) a healthy first host serves directly -- the fallback costs nothing when the
#       named host is fine;
#   (f) both hosts dead: rc non-zero, stdout stays silent about a served file, the
#       refusal names the last curl exit, and no output stump survives.
#
# Cost note: legs (b) and (d)-(f) call curl through a wrapper that appends `--retry 2
# --retry-delay 1` (last occurrence of a valued option wins in curl) -- these legs
# are about WHICH errors retry and in what host order, not about the sleep value. Leg
# (c) is deliberately NOT wrapped: it is the exact shipped shape, ~25 s of
# retry-delay sleeps on purpose. Every fixture server self-expires after 120 s, so a
# leg that dies mid-run leaves no listener behind.
pfr_work=$work/pf469
pfr_ci=$HERE/../.github/workflows/ci.yml
pfr_say() { printf '            %s\n' "$1" >&2; }

# The two fixture servers, written at run time into $pfr_work (the suite's scratch).
pfr_write_servers() {
	mkdir -p -- "$pfr_work" || return 1
	cat >"$pfr_work/reset_server.py" <<'PYEOF' || return 1
# Answers 200 with Content-Length: 5000, sends 200 body bytes, then RSTs (SO_LINGER 0).
import socket, struct, sys, time
pf, secs = sys.argv[1], float(sys.argv[2])
s = socket.socket(); s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
s.bind(("127.0.0.1", 0)); s.listen(8)
open(pf, "w").write(str(s.getsockname()[1]))
end = time.time() + secs
n = 0
while time.time() < end:
    try:
        s.settimeout(2.0); c, _ = s.accept()
    except socket.timeout:
        continue
    n += 1
    print(f"CONN {n}", flush=True)
    try:
        c.recv(65536)
        c.sendall(b"HTTP/1.1 200 OK\r\nContent-Length: 5000\r\n\r\n" + b"x" * 200)
        c.setsockopt(socket.SOL_SOCKET, socket.SO_LINGER, struct.pack("ii", 1, 0))
        c.close()
    except OSError:
        pass
PYEOF
	cat >"$pfr_work/good_server.py" <<'PYEOF' || return 1
# Answers 200 with fixed bytes for any request.
import socket, sys, time
pf, secs = sys.argv[1], float(sys.argv[2])
body = b"PINNED-TZ-BYTES" * 64
s = socket.socket(); s.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
s.bind(("127.0.0.1", 0)); s.listen(8)
open(pf, "w").write(str(s.getsockname()[1]))
end = time.time() + secs
n = 0
while time.time() < end:
    try:
        s.settimeout(2.0); c, _ = s.accept()
    except socket.timeout:
        continue
    n += 1
    print(f"CONN {n}", flush=True)
    try:
        c.recv(65536)
        c.sendall(b"HTTP/1.1 200 OK\r\nContent-Length: %d\r\n\r\n" % len(body) + body)
        c.close()
    except OSError:
        pass
PYEOF
}

# pfr_start <good|reset> <tag> -- echo the port; the pid lives in $pfr_work/pid.<tag>.
pfr_start() {
	local mode=$1 tag=$2
	local pf="$pfr_work/pf.$tag" log="$pfr_work/srv.$tag.log" i
	python3 "$pfr_work/${mode}_server.py" "$pf" 120 >"$log" 2>&1 &
	echo "$!" >"$pfr_work/pid.$tag"
	for i in $(seq 60); do [ -s "$pf" ] && break; sleep 0.1; done
	[ -s "$pf" ] || { pfr_say "$mode server never wrote a port file"; return 1; }
	cat "$pf"
}

pfr_stop() { # <tag...>
	local tag
	for tag in "$@"; do
		[ -f "$pfr_work/pid.$tag" ] || continue
		kill "$(cat "$pfr_work/pid.$tag")" 2>/dev/null || true
		wait "$(cat "$pfr_work/pid.$tag")" 2>/dev/null || true
	done
	return 0
}

# pfr_extract -- the shipped fetch_pinned (+ its pool_hosts default), byte-for-byte
# from ci.yml, into $pfr_work/func.sh. The awk range opens at the array line and
# closes at the function's own closing brace (10 spaces: 8-space YAML block scalar
# + 2). The leg reads ci.yml, not itself: the contract #428 allows, and the point of
# the leg is that the YAML ships what the retry story claims.
pfr_extract() {
	pfr_write_servers || { pfr_say 'could not write the fixture servers'; return 1; }
	awk '/pool_hosts=\(/{f=1} f{print} f&&/^          \}$/{exit}' "$pfr_ci" >"$pfr_work/func.sh" || return 1
	[ -s "$pfr_work/func.sh" ] || { pfr_say 'ci.yml carries no fetch_pinned block to extract'; return 1; }
	grep -q 'fetch_pinned()' "$pfr_work/func.sh" || { pfr_say 'the extraction found no fetch_pinned function'; return 1; }
	bash -n "$pfr_work/func.sh" || { pfr_say 'the extracted block does not parse'; return 1; }
	return 0
}

pfr_pins() {
	local n_retry n_gate
	n_retry=$(grep -c -- '--retry-all-errors' "$pfr_ci")
	n_gate=$(grep -c 'sum --check --status' "$pfr_ci")
	[ "$n_retry" -ge 3 ] || { pfr_say "--retry-all-errors on $n_retry pinned fetch line(s), want >= 3"; return 1; }
	[ "$n_gate" -ge 4 ] || { pfr_say "digest gates in ci.yml: $n_gate, want >= 4 -- the sha256 check is what decides a fetch"; return 1; }
	grep -q -- '--retry-delay 5' "$pfr_ci" || { pfr_say 'no pinned fetch carries --retry-delay 5'; return 1; }
	grep -q 'us\.archive\.ubuntu\.com' "$pfr_ci" || { pfr_say 'the tz host list lost its sibling entry'; return 1; }
	return 0
}

claim_leg '469: ci.yml carries a parseable fetch_pinned' pfr_extract
claim_leg '469: all pinned fetches retry all errors; digest gates intact' pfr_pins

# pfr_control -- #469's row A against the shipped fixture: the step's OLD flags
# spend exactly one connection. If this goes red the fixture no longer has the
# ticket's precondition and every verdict above it is vacuous.
pfr_control() {
	local port rc conns
	port=$(pfr_start reset ctl) || return 1
	curl -sSfL --retry 3 -o "$pfr_work/ctl.out" "http://127.0.0.1:$port/x" >"$pfr_work/ctl.curl" 2>&1
	rc=$?
	conns=$(grep -c '^CONN' "$pfr_work/srv.ctl.log")
	pfr_stop ctl
	pfr_say "old-flags control: rc=$rc conns=$conns, want rc!=0 conns=1 (row A of #469)"
	[ "$rc" -ne 0 ] && [ "$conns" -eq 1 ]
}
claim_leg '469: fixture control -- old flags spend ONE connection' pfr_control

# pfr_probe -- the runner-build measurement #469 could not make: the shipped
# function, verbatim flags (retry-delay 5 included, ~25 s on purpose), pointed at
# the RST fixture by ONE host. Failure must be reported, retries must have been
# spent (the count is printed), and the partial body curl left behind must not
# survive the refusal -- a stump must never reach the digest check.
pfr_probe() {
	local port rc conns outdir="$pfr_work/probe"
	port=$(pfr_start reset probe) || return 1
	mkdir -p "$outdir"
	(
		cd "$outdir" || exit 1
		( set -euo pipefail
		  # shellcheck source=/dev/null
		  source "$pfr_work/func.sh"
		  pool_hosts=("http://127.0.0.1:$port")
		  fetch_pinned tzdata.deb tzdata_X_all.deb )
	) >"$pfr_work/probe.out" 2>"$pfr_work/probe.err"
	rc=$?
	pfr_stop probe
	conns=$(grep -c '^CONN' "$pfr_work/srv.probe.log")
	pfr_say "runner curl $(curl --version | head -1 | cut -d' ' -f2): rc=$rc conns=$conns -- want rc!=0, conns>1"
	[ "$rc" -ne 0 ] && [ "$conns" -gt 1 ] && [ ! -e "$outdir/tzdata.deb" ]
}
claim_leg '469: shipped flags retry a mid-body RST on THIS curl build' pfr_probe

# pfr_run_func <dir> <host1> <host2> -- fetch through the extracted function with
# curl wrapped to rewrite only the timing flags (appended last, so they win).
pfr_run_func() {
	local dir=$1 h1=$2 h2=$3
	(
		cd "$dir" || exit 1
		( set -euo pipefail
		  # shellcheck disable=SC2317  # invoked indirectly, through the sourced function
		  curl() { command curl "$@" --retry 2 --retry-delay 1; }
		  # shellcheck source=/dev/null
		  source "$pfr_work/func.sh"
		  pool_hosts=("http://127.0.0.1:$h1" "http://127.0.0.1:$h2")
		  fetch_pinned tzdata.deb tzdata_X_all.deb )
	)
}

# pfr_fallback -- host one RSTs mid-body, host two serves: the fetch succeeds, the
# bytes are host two's, and the step line names the host that served.
pfr_fallback() {
	local p1 p2 rc sz
	p1=$(pfr_start reset fb1) || return 1
	p2=$(pfr_start good fb2) || { pfr_stop fb1; return 1; }
	mkdir -p "$pfr_work/fb"
	pfr_run_func "$pfr_work/fb" "$p1" "$p2" >"$pfr_work/fb.out" 2>"$pfr_work/fb.err"
	rc=$?
	pfr_stop fb1 fb2
	sz=$(stat -c%s "$pfr_work/fb/tzdata.deb" 2>/dev/null || echo 0)
	pfr_say "fallback: rc=$rc bytes=$sz out: $(cat "$pfr_work/fb.out")"
	[ "$rc" -eq 0 ] && [ "$sz" -gt 0 ] && grep -q "served by http://127.0.0.1:$p2" "$pfr_work/fb.out"
}
claim_leg '469: a mid-body RST on host one falls back to host two' pfr_fallback

# pfr_first -- a healthy first host serves directly: the fallback costs no round
# trip to a good fetch.
pfr_first() {
	local p1 rc
	p1=$(pfr_start good f1) || return 1
	mkdir -p "$pfr_work/first"
	pfr_run_func "$pfr_work/first" "$p1" 1 >"$pfr_work/first.out" 2>"$pfr_work/first.err"
	rc=$?
	pfr_stop f1
	pfr_say "first-host: rc=$rc out: $(cat "$pfr_work/first.out")"
	[ "$rc" -eq 0 ] && grep -q "served by http://127.0.0.1:$p1" "$pfr_work/first.out"
}
claim_leg '469: a healthy first host serves without the fallback' pfr_first

# pfr_both -- both hosts dead: loud refusal naming the last curl exit, stdout
# silent, no output stump left by the failed attempts.
pfr_both() {
	local rc
	mkdir -p "$pfr_work/both"
	pfr_run_func "$pfr_work/both" 1 2 >"$pfr_work/both.out" 2>"$pfr_work/both.err"
	rc=$?
	pfr_say "both-dead: rc=$rc stdout=$(wc -c <"$pfr_work/both.out")B err: $(tail -1 "$pfr_work/both.err")"
	[ "$rc" -ne 0 ] && [ ! -s "$pfr_work/both.out" ] &&
		grep -q 'no host served' "$pfr_work/both.err" && [ ! -e "$pfr_work/both/tzdata.deb" ]
}
claim_leg '469: both hosts dead refuses loudly and leaves no stump' pfr_both
