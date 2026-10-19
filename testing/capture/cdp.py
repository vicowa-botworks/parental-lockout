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
    self-session note begins.

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
    """
    for r in rows:
        labels = [k.lower() for k in r["result"]]
        if "detail" not in labels:
            continue
        cell = r["vals"][labels.index("detail")]
        if SELF_SESSION_NOTE not in cell:
            continue
        before = cell.split(SELF_SESSION_NOTE)[0].rstrip()
        if not before:
            # The note is the whole cell. There is no second sentence here to run into, so there
            # is no seam to check -- and demanding a terminator anyway is a refusal that fires
            # against a correct page. Measured, not theorised: the probe page's session rows carry
            # no lock detail at all (a probe never locks), so its cell is note-only, and the first
            # version of this check refused the probe capture on a tree where the page was right.
            continue
        if before[-1] not in ".!?":
            raise SystemExit(
                f"{what}: the detail runs into the self-session note ({before[-32:]!r} then "
                f"{SELF_SESSION_NOTE!r}); the page still renders a run-on, so this capture is "
                "the pre-fix page")


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
            for _ in range(60):
                try:
                    self.http("/login")
                    break
                except Exception:
                    time.sleep(0.25)
            else:
                raise SystemExit("the daemon never answered; see serve.log")

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

    def close(self):
        for p in (self.chrome, self.server):
            if p and p.poll() is None:
                p.terminate()
        time.sleep(1)
        for p in (self.chrome, self.server):
            if p and p.poll() is None:
                p.kill()


def out_dir(default_name):
    """Where the PNGs land. Never the repo's own docs/ tree unless it is asked for by name:
    a capture that silently overwrites a committed artifact is worse than one that has to be
    requested, because an overwrite is exactly the state a stale screenshot can fake. Pass
    `--out docs/screenshots/360` to re-capture the committed artifacts themselves; each driver
    writes the file name the repo already uses for that page."""
    if "--out" in sys.argv:
        i = sys.argv.index("--out")
        if i + 1 >= len(sys.argv) or sys.argv[i + 1].startswith("--"):
            raise SystemExit("--out needs a directory")
        return sys.argv[i + 1]
    return op.join(RUNS, default_name)

