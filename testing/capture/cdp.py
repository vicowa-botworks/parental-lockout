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
import argparse
import base64
import datetime
import json
import os
import os.path as op
import re
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

# Captures are staged here and copied to their target only after every page has passed. Per
# process, so two drivers running at once cannot publish each other's frames.
_PENDING = []
_STAGE = None


def _stage_dir():
    global _STAGE
    if _STAGE is None:
        _STAGE = op.join(RUNS, f"_stage-{os.getpid()}")
    return _STAGE


def discard_stage():
    """A refusal throws the staged bytes away; it never deletes what is already at the target."""
    stage = _stage_dir()
    for name in os.listdir(stage) if op.isdir(stage) else []:
        os.remove(op.join(stage, name))
    _PENDING.clear()
    if op.isdir(stage):
        os.rmdir(stage)


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

    def set_width(self, width):
        """Move this connection to another viewport (#149).

        Only `width` moves. Every other emulation parameter is repeated from `__init__` on
        purpose: a criterion that compares two viewports has to vary one thing, and a desktop pass
        that also changed the scale factor or the mobile flag would report a difference whose cause
        nobody could name. The page is re-navigated by the caller, exactly as the artifact pass
        navigates it, so each viewport sees a fresh layout rather than a reflow of a page the
        previous pass already measured.
        """
        self.width = width
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
        """Stage the bytes; `publish` is what puts them where they will be read.

        A refusal has to leave the target exactly as it was, and the multi-page driver refuses
        partway through: with four pages and a failure on the audit page, writing as you go leaves
        three freshly captured PNGs and one stale one under committed filenames -- a half-replaced
        evidence set, which is the state this folder's own house rule forbids, and `git status`
        shows it as three ordinary modifications. So nothing reaches `--out` until every page has
        passed.
        """
        dest = op.abspath(path)
        stage = op.join(_stage_dir(), op.basename(dest))
        os.makedirs(op.dirname(stage), exist_ok=True)
        with open(stage, "wb") as fh:
            fh.write(self.screenshot())
        _PENDING.append((stage, dest))

    def publish(self):
        """Copy every staged capture to its final path. Only ever called once the last check on
        the last page has passed."""
        for stage, dest in _PENDING:
            os.makedirs(op.dirname(dest), exist_ok=True)
            with open(stage, "rb") as src:
                data = src.read()
            with open(dest, "wb") as out:
                out.write(data)
            os.remove(stage)
        _PENDING.clear()
        if op.isdir(_stage_dir()):
            os.rmdir(_stage_dir())


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


# The form census, spliced into the same measurement as the cell census so a page is read once
# and the two readings cannot come from different renders.
FORMS_JS = (
    "forms:document.querySelectorAll(\"form[method=post]\").length,"
    "fields:document.querySelectorAll(\"form[method=post] input[name], "
    "form[method=post] select[name], form[method=post] textarea[name]\").length"
)


def require_form(got, what):
    """A form page with nothing to submit was not measured.

    This is the sibling of require_records, not a relaxation of it. A page like /settings has no
    `table.rows` at all, so require_records would refuse it as an empty fixture -- and the wrong
    repair is to drop the check, because a /settings that rendered only its navigation and no
    form also fits at 360 px and would be committed as evidence that the page fits. What makes a
    form page's fit mean something is that there is a form on it, with fields, posting somewhere.
    A signed-out run lands on /login, which does have a form -- the caller's own URL check is what
    stops that page being photographed as /settings.
    """
    if got["forms"] == 0 or got["fields"] == 0:
        raise SystemExit(f"{what} rendered {got['forms']} POST forms and {got['fields']} fields; "
                         "there is nothing to submit here, so a fit would prove nothing")


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

    Two counts come back, and the difference between them is the honest statement of what was
    covered: `seen` is how many cells held the note at all, `seams` how many of those had a second
    sentence in front of it for the rule to check. Requiring `seen` above zero is what stops the
    rule from reporting success on a page where the note never rendered -- if the templates ever
    reword it, that is the line which turns a silent loss of coverage into a refusal. But `seams`
    cannot be required: on the probe page a lockable session's cell is the note alone, so a
    correct page can legitimately have no seam at all, and a driver that demanded one would refuse
    correct code. It is printed instead, so the transcript says whether #108 was exercised.
    """
    seen = 0
    seams = 0
    for r in rows:
        labels = [k.lower() for k in r["result"]]
        if "detail" not in labels:
            continue
        cell = r["vals"][labels.index("detail")]
        if SELF_SESSION_NOTE not in cell:
            continue
        seen += 1
        before = cell.split(SELF_SESSION_NOTE)[0].rstrip()
        if before:
            seams += 1
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
    return seen, seams


class Capture:
    """The daemon, the browser and the signed-in session, with one teardown path.

    Use it as a context manager. Teardown has to happen on the failure path too: a driver that
    refuses and leaves a headless Chrome and a root-owned daemon serving a throwaway config
    behind it teaches whoever runs it next that this folder is unsafe to run.
    """

    def __init__(self, fixture_name, extra_chrome_flags=()):
        self.dir, self.env = fixture(fixture_name)
        self.port = int(self.env["PLOCK_CAPTURE_PORT"])
        self.cdp = int(self.env["PLOCK_CAPTURE_CDP"])
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

            # The DevTools endpoint is a shared address exactly like the daemon's port, and the
            # same mistake applies to it: asking who is there, rather than insisting it is the one
            # we started. Measured: with a stranger Chrome holding the fixture's CDP port, this
            # driver's own chrome.log recorded `bind() failed: Address already in use`, and the run
            # went on to sign in, measure and screenshot a tab in a browser it never started, in a
            # profile it never made -- then left that stranger holding the port for the next run.
            # If the stranger is a headed browser, the capture has hijacked a real window.
            if _port_answers(f"http://127.0.0.1:{self.cdp}/json/version"):
                raise SystemExit(
                    f"something already answers the DevTools endpoint on 127.0.0.1:{self.cdp}, so "
                    "this capture would drive a browser it did not start. Close whatever holds the "
                    "port (a leftover chrome --headless, or a real Chrome started with "
                    "--remote-debugging-port) or move CDP_PORT; the fixture's own chrome.log will "
                    "say `Address already in use`.")
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

            # The endpoint answered, so a browser is there -- which is not the same as *our*
            # browser being there. Chrome that loses the port does not exit: it falls back to ::1
            # on the same port number and stays up, so require_running's pid check cannot see it
            # and the pre-launch probe above is a race that a stranger arriving half a second late
            # wins. Chrome's own first line, printed into this fixture's chrome.log, names the
            # address it bound and the id of the browser behind it -- a file a stranger cannot have
            # written. Compare both with what the endpoint advertises, and refuse unless they agree.
            addr, self.browser_id = _logged_browser(op.join(self.dir, "chrome.log"))
            if self.browser_id is None:
                raise SystemExit(
                    f"chrome never logged the DevTools endpoint it opened, so there is no way to "
                    f"tell whether 127.0.0.1:{self.cdp} is the browser this fixture started; see "
                    f"{self.dir}/chrome.log")
            want_addr = f"127.0.0.1:{self.cdp}"
            if addr != want_addr:
                raise SystemExit(
                    f"chrome opened its DevTools endpoint on {addr}, not {want_addr} -- it lost the "
                    "port to something already there and fell back rather than exiting. The capture "
                    "would drive that other browser. Close it (chrome.log says `Address already in "
                    "use`) or move CDP_PORT.")
            served = _browser_id(f"http://127.0.0.1:{self.cdp}/json/version")
            if served != self.browser_id:
                raise SystemExit(
                    f"the DevTools endpoint on 127.0.0.1:{self.cdp} serves browser {served!r} while "
                    f"this fixture's chrome.log recorded {self.browser_id!r}, so the capture would "
                    "drive a browser it did not start.")

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
        """The daemon and the browser this fixture started must still be the ones in use.

        Called once before the browser touches anything and once immediately before the PNG is
        written. If either died, whatever answered the requests in between was not started by this
        harness -- a different config, a different audit trail, a different browser window -- and
        the measurement would be about that machine while the transcript names this one. The
        fixture's own logs are named because that is where `address already in use` is written;
        the process exit code alone does not say who took the port.
        """
        for who, proc, hint in (
                ("daemon", self.server,
                 "something else was answering the port, and serve.log will say "
                 "`bind: address already in use`"),
                ("browser", self.chrome,
                 "something else was answering the DevTools endpoint, and chrome.log will say "
                 "`Address already in use`")):
            if proc is not None and proc.poll() is not None:
                raise SystemExit(
                    f"the {who} this fixture started is no longer running (code "
                    f"{proc.returncode}) -- {hint}. Free the port or move it in the setup script; "
                    "a capture of somebody else's daemon or browser window is not evidence about "
                    "this fixture.")
        # A live pid is not ownership -- the Chrome that lost the port is alive as well, on ::1.
        # Ask the endpoint which browser it is and compare with what our own profile recorded.
        expected = getattr(self, "browser_id", None)
        if expected is not None:
            served = _browser_id(f"http://127.0.0.1:{self.cdp}/json/version")
            if served != expected:
                raise SystemExit(
                    f"the DevTools endpoint on 127.0.0.1:{self.cdp} now serves browser "
                    f"{served!r}, not the {expected!r} this fixture started, so the measurement "
                    "since the last check was taken in a browser this harness did not open.")

    def close(self):
        for p in (self.chrome, self.server):
            if p and p.poll() is None:
                p.terminate()
        time.sleep(1)
        for p in (self.chrome, self.server):
            if p and p.poll() is None:
                p.kill()


def capture_out(default_name):
    """Where the PNGs land: inside this repository, and only under a flag you meant to type.

    Never the repo's own docs/ tree unless it is asked for by name -- an overwrite is exactly the
    state a stale screenshot can fake -- and never outside the repository either. Three separate
    holes, all measured on this branch:

    * A relative --out resolved against the shell's directory, so `--out docs/screenshots/360`
      typed from testing/capture created testing/capture/docs/screenshots/360/, which the runs/
      ignore rule does not cover: the operator believed they had re-taken the evidence, the
      committed files were untouched, and a later `git add -A` would commit four PNGs nobody
      meant to make. So a relative --out resolves against the repository root.
    * The path was normalised, not resolved, so a symlink under runs/ walked straight out of the
      repository with the guard silent. Hence realpath on both sides.
    * argv was read by hand, so `--out=docs/screenshots/360` and a mistyped `--outt` were both
      ignored and the driver exited 0 having written to runs/ -- the same harm, arriving from the
      other direction. Hence argparse: an unrecognised flag is an error, not a no-op.
    """
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--out", metavar="DIR",
                    help="directory for the PNGs, inside the repository; default "
                         "testing/capture/runs/" + default_name)
    ap.add_argument("--overwrite", action="store_true",
                    help="required to replace PNGs that already exist in --out")
    a = ap.parse_args()
    if not a.out:
        return op.join(RUNS, default_name)
    target = op.realpath(a.out if op.isabs(a.out) else op.join(REPO, a.out))
    if not (target + op.sep).startswith(op.realpath(REPO) + op.sep):
        raise SystemExit(f"--out {a.out!r} resolves to {target}, outside the repository "
                         f"({op.realpath(REPO)}); a capture writes inside the repo or into its "
                         "own ignored runs/ directory")
    if op.exists(target) and not op.isdir(target):
        raise SystemExit(f"--out {a.out!r} is a file, not a directory")
    # Replacing a committed artifact is a different act from taking a picture, and it is the one
    # that can quietly destroy evidence. Writing this folder's own README and then testing the
    # invocation it names replaced two committed PNGs with no error and no warning -- overwriting
    # is what the flag is for. So the flag is doubled: --out says where, --overwrite says the
    # files already there are the ones you mean to replace.
    #
    # The test is where the target IS, not whether it happens to hold PNGs yet. Keying on the
    # contents let `--out docs/screenshots/360/..` and `--out .` through -- both resolve inside
    # the repo, neither contains a *.png at the top -- and the run then wrote four new PNGs into
    # tracked directories with no flag and no warning, which is the exact harm the flag exists to
    # gate. Anything that is not this package's own ignored runs/ directory takes the flag.
    stage_root = op.realpath(RUNS) + op.sep
    if not (target + op.sep).startswith(stage_root) and not a.overwrite:
        existing = sorted(f for f in os.listdir(target) if f.endswith(".png")) if op.isdir(target) else []
        holds = f" already holds {len(existing)} PNG(s) ({', '.join(existing)})" if existing else ""
        raise SystemExit(
            f"{target}{holds} is not this package's scratch directory ({RUNS}), so writing there "
            "means replacing evidence. Pass --overwrite as well to say you mean it; a re-capture "
            "belongs in the same branch as the change that made the old one stale")
    global _STAGE
    _STAGE = op.join(RUNS, f"_stage-{os.getpid()}")
    return target


def _browser_id(url):
    """The id in the `webSocketDebuggerUrl` a DevTools endpoint advertises, or None.

    This is the only thing that names *which* browser an endpoint belongs to. A second Chrome that
    loses the port race does not exit: it logs `bind() failed: Address already in use`, falls back
    to `::1` on the SAME port number, and stays alive -- so a live pid and an answering port are
    both equally consistent with talking to a stranger.
    """
    try:
        with urllib.request.urlopen(url, timeout=5) as r:
            version = json.load(r)
    except Exception:
        return None
    ws = version.get("webSocketDebuggerUrl") or ""
    marker = "/devtools/browser/"
    if marker not in ws:
        return None
    return ws.split(marker, 1)[1].strip("/")


def _logged_browser(log_path):
    """The address and browser id Chrome said it opened, read from the fixture's own log.

    `--headless=new` does not write DevToolsActivePort into --user-data-dir (measured: a
    successful run leaves no such file anywhere under the profile), but it prints the endpoint it
    opened to stdout, and stdout is a file this fixture made -- which a stranger cannot have
    written. The address matters as much as the id: a Chrome that lost the port logs
    `bind() failed: Address already in use` and then `DevTools listening on ws://[::1]:9333/...`,
    so it stays alive on the same port number at a different address and both the pid check and a
    bare id comparison can still read it as ours.
    """
    for _ in range(20):
        try:
            with open(log_path, errors="replace") as f:
                text = f.read()
        except OSError:
            text = ""
        m = re.search(r"DevTools listening on ws://([^/]+)/devtools/browser/(\S+)", text)
        if m:
            return m.group(1), m.group(2).strip("/")
        time.sleep(0.25)
    return None, None


def _port_answers(url):
    """True if anything at all replies. A refused connection is the answer we want."""
    try:
        with urllib.request.urlopen(url, timeout=1):
            return True
    except urllib.error.HTTPError:
        return True
    except Exception:
        return False


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


def report_refusal(path, exc):
    """Every refusal comes through here, and it says what happened to the file.

    'exit 1, nothing written' is true and still misleading: a PNG from an earlier run is sitting
    right there, and the next pair of hands to reach for it has no way to tell it predates the
    run that refused. Deleting it is not the answer either -- with --out pointing at the docs
    tree, the file it would delete is the committed evidence. So the refusal names the stale
    file, with the mtime and size that say when it really came from.

    It reports and does NOT exit, and that is the whole point of this function's shape. It used
    to end in `raise SystemExit(1)`, which made every statement after every call to it in the
    package dead code, because the raise happened inside the caller's own `except` handler:

      * `pages.py` collected refused URLs to print one honest `N/5 pages fit ... refused: ...`
        sentence. Measured with a stubbed `shoot()` that refuses: the loop stopped after page 1
        of 5, `discard_stage()` ran 0 times, and the sentence never printed.
      * `lock.py` and `testlock.py` each called `discard_stage()` on the next line. Dead.

    The exit code laundered all of it, because an uncaught exception is also exit 1 -- the code
    a refusal is supposed to produce. So the reporting stays here and the decision stays with
    the caller, which is the only place that knows whether one more page can be measured. There
    is deliberately no raising variant sitting next to it: one verb, so the next statement after
    a refusal cannot be a silent casualty.
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

