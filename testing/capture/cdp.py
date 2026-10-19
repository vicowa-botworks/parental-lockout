#!/usr/bin/env python3
"""The parts every 360 px capture needs: a daemon, a browser, a signed-in session.

Three pages in this repo are accepted at 360 px by a photograph rather than by a test:
the four GET-rendered admin pages, and the two result pages that exist only as the response
to a POST. All three drivers used to be separate copies of this file's contents, sitting in a
scratch directory outside the repository — which is how `docs/screenshots/360/lock.png` ended
up depicting a run-on sentence the tests said was fixed (#108), and how a capture taken before
a fix can otherwise land in the repo looking like the fixed page.

So the shared machinery lives here, and the refusals live here too. A screenshot of this repo
is evidence only because the thing that took it refuses to take it in the wrong state: wrong
URL, empty banner, unlabelled cell, missing button, a click that did not navigate, a detail
cell that runs into the note beside it. A refusal nobody has watched fire is as unproven as an
assertion never run against the alternative, so each driver's README names the negative control
that proves its own refusals bite.

Nothing in here asserts anything about a page. Page-specific checks belong in the driver that
photographs that page, next to the sentence it is checking.
"""
import base64
import datetime
import json
import os
import os.path as op
import socket
import struct
import subprocess
import sys
import time
import urllib.request

HERE = op.dirname(op.abspath(__file__))
REPO = op.dirname(op.dirname(HERE))

# The width every acceptance screenshot in this repo is taken at. It is a contract, not a
# preference: docs/screenshots/360/ is named for it, and the narrow stylesheet's breakpoints
# are written against it.
WIDTH = 360

# The fixture directory a driver works in, and the binary it serves. Both overridable, because
# the default output location must never be the repo's own docs/ tree: a capture that silently
# overwrites a committed artifact is worse than one that has to be asked for.
RUNS = op.join(HERE, "runs")


def fixture(name):
    """The run directory for one capture, and the fixture's own answers."""
    d = op.join(RUNS, name)
    envfile = op.join(d, "fixture.env")
    if not op.exists(envfile):
        raise SystemExit(
            f"{envfile} is missing; run {name}-setup.sh first\n"
            f"(it builds the throwaway config, the state directory and the admin account "
            f"this capture signs in to)")
    env = {}
    with open(envfile) as fh:
        for line in fh:
            line = line.strip()
            if line and not line.startswith("#") and "=" in line:
                k, v = line.split("=", 1)
                env[k] = v.strip().strip('"')
    for key in ("PLOCK_CAPTURE_USER", "PLOCK_CAPTURE_PASS", "PLOCK_CAPTURE_TARGET",
                "PLOCK_CAPTURE_PORT", "PLOCK_CAPTURE_CDP"):
        if key not in env:
            raise SystemExit(f"{envfile} does not set {key}; the fixture is incomplete")
    return d, env


def binary():
    """The daemon to serve: an explicit $PLOCK_BIN, else the repo's own build."""
    b = os.environ.get("PLOCK_BIN")
    if b:
        if not op.exists(b):
            raise SystemExit(f"PLOCK_BIN={b} does not exist")
        return b
    built = op.join(REPO, "plock")
    if not op.exists(built):
        raise SystemExit(f"{built} is not built; run `go build -o plock ./cmd/plock` in {REPO} "
                         "or point PLOCK_BIN at a binary")
    return built


# ---------------------------------------------------------------- minimal CDP websocket
class WS:
    """Just enough of a websocket client for CDP, with no third-party dependency.

    This repo takes no modules (scripts/check-deps.sh fails the build if go.mod grows a
    require line, and there is no Python environment at all to grow one in), so the framing
    is written out: masked client frames, 16- and 64-bit length forms, and a reader that
    returns the next data frame and skips pings.
    """

    def __init__(self, url):
        rest = url.split("//", 1)[1]
        hostport, self.path = rest.split("/", 1)
        self.path = "/" + self.path
        host, port = hostport.split(":")
        self.s = socket.create_connection((host, int(port)), timeout=30)
        key = base64.b64encode(os.urandom(16)).decode()
        self.s.sendall((f"GET {self.path} HTTP/1.1\r\nHost: {hostport}\r\n"
                        f"Upgrade: websocket\r\nConnection: Upgrade\r\n"
                        f"Sec-WebSocket-Key: {key}\r\nSec-WebSocket-Version: 13\r\n\r\n").encode())
        buf = b""
        while b"\r\n\r\n" not in buf:
            buf += self.s.recv(4096)
        if b"101" not in buf.split(b"\r\n")[0]:
            raise RuntimeError("websocket handshake refused: " + buf.split(b"\r\n")[0].decode())

    def send(self, obj):
        data = json.dumps(obj).encode()
        hdr = bytearray([0x81])
        n = len(data)
        if n < 126:
            hdr.append(0x80 | n)
        elif n < 65536:
            hdr.append(0x80 | 126)
            hdr += struct.pack(">H", n)
        else:
            hdr.append(0x80 | 127)
            hdr += struct.pack(">Q", n)
        mask = os.urandom(4)
        self.s.sendall(bytes(hdr) + mask + bytes(b ^ mask[i % 4] for i, b in enumerate(data)))

    def _recv(self, n):
        buf = b""
        while len(buf) < n:
            chunk = self.s.recv(n - len(buf))
            if not chunk:
                raise RuntimeError("websocket closed")
            buf += chunk
        return buf

    def recv(self):
        while True:
            b1, b2 = self._recv(2)
            fin, op = b1 & 0x80, b1 & 0x0F
            n = b2 & 0x7F
            if n == 126:
                n = struct.unpack(">H", self._recv(2))[0]
            elif n == 127:
                n = struct.unpack(">Q", self._recv(8))[0]
            payload = self._recv(n)
            if op == 8:
                raise RuntimeError("websocket closed by chrome")
            if op in (1, 2) and fin:
                return json.loads(payload)

    def call(self, mid, method, params=None):
        self.send({"id": mid, "method": method, "params": params or {}})
        while True:
            msg = self.recv()
            if msg.get("id") == mid:
                if "error" in msg:
                    raise RuntimeError(f"{method}: {msg['error']}")
                return msg.get("result", {})


class Session:
    """One CDP connection, at one viewport, with the counters kept out of the drivers."""

    def __init__(self, ws, width):
        self._ws = ws
        self._mid = 0
        self.width = width
        self.call("Page.enable")
        self.call("Runtime.enable")
        self.call("Emulation.setDeviceMetricsOverride", width=width, height=900,
                  deviceScaleFactor=1, mobile=True)

    def call(self, method, **params):
        self._mid += 1
        return self._ws.call(self._mid, method, params)

    def goto(self, url):
        self.call("Page.navigate", url=url)
        for _ in range(80):
            time.sleep(0.1)
            if self.value("document.readyState") == "complete":
                return
        raise SystemExit(f"{url} never finished loading")

    def where(self):
        return self.value("location.pathname")

    def value(self, expr):
        return self.call("Runtime.evaluate", expression=expr,
                         returnByValue=True)["result"].get("value")

    def json(self, expr):
        return json.loads(self.value(expr))

    def settle(self, seconds=0.4):
        """One more beat for the stylesheet, then measure. A page measured mid-layout reports
        whatever width the pre-CSS document happens to have."""
        time.sleep(seconds)

    def screenshot(self):
        # The whole page, not the viewport: a screenshot cut at the fold is not evidence that
        # a page fits, because the part that was cut off is exactly where a wide element tends
        # to sit.
        return base64.b64decode(self.call("Page.captureScreenshot", format="png",
                                          captureBeyondViewport=True)["data"])

    def write_png(self, path):
        os.makedirs(op.dirname(op.abspath(path)), exist_ok=True)
        with open(path, "wb") as fh:
            fh.write(self.screenshot())


WIDTH_JS = ("JSON.stringify({scroll:document.documentElement.scrollWidth,"
            "height:document.documentElement.scrollHeight,"
            "body:document.body.scrollWidth,inner:window.innerWidth,"
            "tables:document.querySelectorAll('table.rows').length,"
            "cells:document.querySelectorAll('table.rows td:not([colspan])').length,"
            "unlabelled:document.querySelectorAll("
            "'table.rows td:not([colspan]):not([data-label])').length})")


def measure(session, extra=""):
    """scrollWidth plus the cell census. `extra` splices page-specific keys into the same
    object so a driver measures once, not twice, and the two readings cannot disagree."""
    js = WIDTH_JS[:-2] + ("," + extra if extra else "") + "})"
    return session.json(js)


def require_labelled_cells(got, what):
    """Every non-colspan cell in a `table.rows` table has to carry a data-label: that attribute
    is what the narrow stylesheet uses to name a cell when the header row has been stacked
    away. A missing one is a page that renders an unlabelled value at 360 px."""
    if got["unlabelled"]:
        raise SystemExit(f"{what} has {got['unlabelled']} cells with no data-label")


def require_records(got, what):
    """A page with no table was not measured, whatever its scrollWidth says: the empty-state
    sentence fits at any width. Every fixture here is built to have rows."""
    if got["tables"] == 0 or got["cells"] == 0:
        raise SystemExit(f"{what} rendered {got['tables']} tables and {got['cells']} cells; "
                         "the fixture is empty, so a fit here would prove nothing")


def session_rows(session):
    """Every row of every `table.rows` table, as (data-label, text) pairs.

    The data-label matters: at 360 px the header row is stacked away and the label attribute is
    the only thing that says which cell is which, so a check that reads cells by position is
    reading a different column on a narrow page than on a wide one.
    """
    return session.json(
        "JSON.stringify([...document.querySelectorAll('table.rows tbody tr')].map(tr => {"
        "  const c = [...tr.querySelectorAll('td:not([colspan])')];"
        "  return {result: c.map(x => x.getAttribute('data-label')),"
        "          vals: c.map(x => x.innerText.trim())};}))")


# The first words of the self-session note, as one string in one place. Both result pages render
# the same Detail cell, and the check below is about the seam between that cell's two sentences.
SELF_SESSION_NOTE = "This is the session the daemon runs"


def require_detail_terminated(rows, what):
    """#108's rule, on the rendered page: the Detail cell's own sentence has to end before the
    self-session note begins. Returns how many cells the note actually appeared in.

    The cell can hold two sentences at once -- what the lock produced, and the note -- and Go
    trims the whitespace around every template action, so the only thing between them is what the
    code chose to put there. For #103 that was a space, and the page read
    "...accepted the request but cannot confirm it This is the session the daemon runs inside."
    The PNG committed for #103 was exactly that: it passed every other refusal in this file --
    right URL, right banner, 360 px, every cell labelled -- and still showed the defect the repo
    claimed was fixed.

    This lives here rather than in each driver because the cell was copied between pages, and a
    copied cell is a copied rule: the check that only exists on one page is the check that fails
    to notice when the other one regresses.

    The return value is not decoration. Every branch below can skip, and a rule that skips
    everything still reports success -- so a driver has to require a count above zero, or it is
    asserting nothing while appearing to assert #108. If the note's wording is ever changed in the
    templates, this is the line that turns that from a silent loss of coverage into a refusal.
    """
    seen = 0
    for r in rows:
        labels = [k.lower() for k in r["result"]]
        if "detail" not in labels:
            continue
        cell = r["vals"][labels.index("detail")]
        if SELF_SESSION_NOTE not in cell:
            continue
        seen += 1
        before = cell.split(SELF_SESSION_NOTE)[0].rstrip()
        if not before:
            # The note is the whole cell. There is no second sentence here to run into, so there
            # is no seam to check -- and demanding a terminator anyway is a refusal that fires
            # against a correct page. Measured, not theorised: the probe page's session rows carry
            # no lock detail when the session is lockable (a probe never locks, and an unlockable
            # session does get one, which is why the count above is what a driver asserts on), so
            # its cell is note-only, and the first version of this check refused the probe capture
            # on a tree where the page was right.
            continue
        if before[-1] not in ".!?":
            raise SystemExit(
                f"{what}: the detail runs into the self-session note ({before[-32:]!r} then "
                f"{SELF_SESSION_NOTE!r}); the page still renders a run-on, so this capture is "
                "the pre-fix page")
    return seen


class Capture:
    """The daemon, the browser and the signed-in session, with one teardown path.

    Use it as a context manager. Teardown has to happen on the failure path too: a driver that
    refuses and leaves a headless Chrome and a root-owned daemon serving a throwaway config
    behind it teaches whoever runs it next that this folder is unsafe to run.
    """

    def __init__(self, fixture_name, extra_chrome_flags=()):
        self.dir, self.env = fixture(fixture_name)
        self.port = int(self.env["PLOCK_CAPTURE_PORT"])
        self.base = f"http://127.0.0.1:{self.port}"
        self.bin = binary()
        self.server = None
        self.chrome = None
        self.session = None
        self._flags = tuple(extra_chrome_flags)

    def __enter__(self):
        self._start()
        return self

    def __exit__(self, *exc):
        self.close()
        return False

    def http(self, path, data=None, headers=None, method=None):
        req = urllib.request.Request(self.base + path, data=data, headers=headers or {},
                                     method=method)
        with urllib.request.urlopen(req, timeout=20) as r:
            return r.status, r.headers, r.read()

    def _start(self):
        self.server = subprocess.Popen(
            [self.bin, "serve", "--config", op.join(self.dir, "config.json"),
             "--state-dir", op.join(self.dir, "state"),
             "--audit-path", op.join(self.dir, "audit.jsonl")],
            stdout=open(op.join(self.dir, "serve.log"), "wb"), stderr=subprocess.STDOUT)
        try:
            # The port answering is not evidence that *our* daemon is answering. Measured: with a
            # second `plock serve` already holding the fixture's port, this fixture's daemon dies
            # on `bind: address already in use`, the stranger answers the readiness probe, and the
            # capture signs in, clicks, measures and writes a PNG -- about a different config, a
            # different audit trail and a different set of accounts than the fixture just built.
            # A daemon can only answer once it has bound, so watching the process here is enough.
            for _ in range(60):
                if self.server.poll() is not None:
                    raise SystemExit(
                        f"the daemon this fixture started exited (code {self.server.returncode}) "
                        f"before it answered anything: something else already holds "
                        f"127.0.0.1:{self.port}, and a capture aimed at that process photographs a "
                        "different daemon than the fixture built. See serve.log; free the port or "
                        "move it in the setup script.")
                try:
                    self.http("/login")
                    break
                except Exception:
                    time.sleep(0.25)
            else:
                raise SystemExit("the daemon never answered; see serve.log")
            # A successful probe is not proof that our daemon answered it. A stranger on the port
            # replies instantly, so the loop can break while our own process is still on its way to
            # failing its bind -- which is how the first version of this check missed: the capture
            # went on to sign in at the stranger's login form. Our daemon fails the bind inside the
            # first ~100 ms of starting, so looking again shortly after the first answer sees it.
            time.sleep(0.4)
            self.require_running()

            self.chrome = subprocess.Popen(
                ["google-chrome", "--headless=new",
                 f"--remote-debugging-port={self.env['PLOCK_CAPTURE_CDP']}",
                 f"--user-data-dir={self.dir}/chrome", "--no-first-run",
                 "--no-default-browser-check", "--disable-gpu", *self._flags, "about:blank"],
                stdout=open(op.join(self.dir, "chrome.log"), "wb"), stderr=subprocess.STDOUT)
            targets = None
            for _ in range(60):
                try:
                    targets = json.load(urllib.request.urlopen(
                        f"http://127.0.0.1:{self.env['PLOCK_CAPTURE_CDP']}/json/list", timeout=5))
                    if any(t.get("type") == "page" for t in targets):
                        break
                except Exception:
                    time.sleep(0.25)
            else:
                raise SystemExit("chrome's devtools endpoint never came up; see chrome.log")
            target = next(t for t in targets if t.get("type") == "page")

            # Sign in inside this browser, the way a person does. A cookie captured by a second
            # client would never reach Chrome, and an unauthenticated Chrome quietly photographs
            # the login page at every address while still reporting 360 px -- a green run that
            # measured nothing.
            self.session = Session(WS(target["webSocketDebuggerUrl"]), WIDTH)
            self.session.goto(self.base + "/login")
            filled = self.session.value(f"""
                (() => {{
                  const u = document.querySelector('input[name=username]');
                  const p = document.querySelector('input[name=password]');
                  if (!u || !p) return 'no form';
                  u.value = {json.dumps(self.env['PLOCK_CAPTURE_USER'])};
                  p.value = {json.dumps(self.env['PLOCK_CAPTURE_PASS'])};
                  u.form.submit(); return 'submitted';
                }})()""")
            if filled != "submitted":
                raise SystemExit(f"the login form could not be filled: {filled!r}")
            for _ in range(80):
                time.sleep(0.1)
                if self.session.where() != "/login":
                    break
            else:
                raise SystemExit("login did not navigate away from /login; the password is wrong "
                                 "or the account was never created")
            print(f"signed in as {self.env['PLOCK_CAPTURE_USER']} at {self.base}", flush=True)
        except BaseException:
            self.close()
            raise

    def require_running(self):
        """The daemon this fixture started must still be the process serving the port.

        Called once before the browser touches anything and once immediately before the PNG is
        written. If it died, whatever answered the requests in between was somebody else's daemon
        -- a different config, a different audit trail, a different set of accounts -- and the
        measurement would be about that machine while the transcript names this one.
        """
        if self.server is not None and self.server.poll() is not None:
            raise SystemExit(
                f"the daemon this fixture started is no longer running (code "
                f"{self.server.returncode}). Something else was answering 127.0.0.1:{self.port} "
                "-- most likely another plock serve already holds the port, and the fixture's own "
                "serve.log will say `bind: address already in use`. Free the port or move it in "
                "the setup script; a capture of a stranger's daemon is not evidence about this "
                "fixture.")

    def close(self):
        for p in (self.chrome, self.server):
            if p and p.poll() is None:
                p.terminate()
        time.sleep(1)
        for p in (self.chrome, self.server):
            if p and p.poll() is None:
                p.kill()


def out_dir(default_name):
    """Where the PNGs land: inside this repository, or nowhere.

    Never the repo's own docs/ tree unless it is asked for by name -- an overwrite is exactly
    the state a stale screenshot can fake -- and never outside the repository either. A relative
    `--out` is resolved against the repo root rather than the shell's directory, because the
    obvious invocation is the one that goes wrong: `--out docs/screenshots/360` typed from
    testing/capture otherwise creates testing/capture/docs/screenshots/360/, which the runs/
    ignore rule does not cover, so the operator believes they re-took the evidence, the
    committed files are untouched, and a later `git add -A` commits four PNGs nobody meant to
    make. Measured, not theorised.
    """
    if "--out" in sys.argv:
        i = sys.argv.index("--out")
        if i + 1 >= len(sys.argv) or sys.argv[i + 1].startswith("--"):
            raise SystemExit("--out needs a directory")
        raw = sys.argv[i + 1]
        target = op.normpath(raw if op.isabs(raw) else op.join(REPO, raw))
        if not (target + op.sep).startswith(REPO + op.sep):
            raise SystemExit(f"--out {raw!r} resolves to {target}, outside the repository "
                             f"({REPO}); a capture writes inside the repo or into its own "
                             "ignored runs/ directory")
        # Replacing a committed artifact is a different act from taking a picture, and it is the
        # one that can quietly destroy evidence. Measured on this branch: the sentence in the
        # README telling you to pass `--out docs/screenshots/360` was tested by passing exactly
        # that, and two committed PNGs were gone before anything said a word -- no error, because
        # that is what the flag is for. So the flag has to be doubled: --out says where, and
        # --overwrite says the files already there are the ones you mean to replace.
        existing = sorted(f for f in os.listdir(target) if f.endswith(".png")) if op.isdir(target) else []
        if existing and "--overwrite" not in sys.argv:
            raise SystemExit(f"{target} already holds {len(existing)} PNG(s) "
                             f"({', '.join(existing)}). Pass --overwrite as well to replace "
                             "them; a re-capture belongs in the same branch as the change that "
                             "made the old one stale")
        return target
    return op.join(RUNS, default_name)


def audit_lines(path):
    """The trail as it stands right now, so a driver can remember where this run starts."""
    try:
        with open(path) as fh:
            return [l for l in fh if l.strip()]
    except FileNotFoundError:
        return []


def require_trail(lines, must, allowed, what):
    """The audit trail half of a capture's claim: this click wrote `must`, and nothing else.

    An allowlist, not a denylist. Refusing kinds that start with `lock.` misses the one that
    matters most, because `lockout.entered` starts with `locko` -- and on the probe page the whole
    point is that nothing which claims a lock may appear. So the rule is inverted: anything the
    driver was not told to expect stops the capture, and a future benign line has to be admitted
    on purpose rather than slipping through a prefix.

    `lines` is the part of the trail this run added, not the whole file: a fixture can be
    photographed twice, and a membership test against the whole file passes on the previous run's
    line.
    """
    kinds = [json.loads(l).get("kind", "") for l in lines]
    print(f"audit trail from this click: {sorted(set(kinds)) or 'nothing'}", flush=True)
    if must not in kinds:
        raise SystemExit(f"{what} wrote no {must!r} line; the page shows a verdict this run did "
                         "not ask for")
    unexpected = sorted(set(kinds) - set(allowed))
    if unexpected:
        raise SystemExit(f"{what} wrote {unexpected}; the only line it may write is "
                         f"{sorted(set(allowed))[0]!r} -- if a new line is expected now, widen "
                         "the allowlist deliberately and say what it is")


def refused(path, exc):
    """Every refusal comes through here, and it says what happened to the file.

    'exit 1, nothing written' is true and still misleading: a PNG from an earlier run is sitting
    right there, and the next pair of hands to reach for it has no way to tell it predates the
    run that refused. Deleting it is not the answer either -- with --out pointing at the docs
    tree, the file it would delete is the committed evidence. So the refusal names the stale
    file, with the mtime and size that say when it really came from.
    """
    print(f"REFUSED: {exc}", file=sys.stderr)
    if path and op.exists(path):
        st = os.stat(path)
        when = datetime.datetime.fromtimestamp(st.st_mtime, datetime.timezone.utc)
        print(f"  a previous {op.basename(path)} is still at {path} -- mtime "
              f"{when.strftime('%Y-%m-%dT%H:%M:%SZ')}, {st.st_size} bytes -- and it is NOT from "
              "this run. Do not quote it as current.", file=sys.stderr)
    else:
        print("  no file was written", file=sys.stderr)
    raise SystemExit(1)

