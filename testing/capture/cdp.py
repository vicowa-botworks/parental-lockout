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


# The directory a refused run leaves its frames in, and the note beside them. See `keep_refused`
# for why the bytes are kept and `refused_dir` for why they can only ever land here.
REFUSED_NOTE = "REFUSED.txt"
_REFUSED = None


def refused_dir(mode):
    """`testing/capture/runs/<mode>-refused` -- the only directory a refusal may write into.

    Named from the fixture and from `RUNS`, never from `--out`, because `--out` is where the
    evidence lives and a refusal's whole contract is that nothing lands there (#222 is what a
    half-replaced evidence set costs). `--out runs/pages-refused` is the one way an operator
    could point the two at the same directory, so `keep_refused` asks about the run's own target
    as well and declines the keep when they overlap.
    """
    return op.join(RUNS, f"{mode}-refused")


def keep_refused(keep, note=None, out=None):
    """Move what this run has staged into `keep`; write `note` beside it; touch nothing at `--out`.

    Deleting the frames a refusal had already measured used to be the contract, and it made a
    refusal an argument nobody could check. Measured 6 Nov on `main` with the condition the capture
    notes already document as refusing this fixture
    (`PLOCK_CAPTURE_SCRIPTS=on ./pages-setup.sh && ./pages.py`): one page refused, five measured
    `FITS` and printed their artifact paths, and the run then deleted all five --

        REFUSED: /: the 'casey' row says ['locked out until 18h 16m', ...]
          no file was written
          /users     @360   ... FITS  .../runs/pages/users.png
          ... four more like it
        Nothing was written to .../runs/pages.

    Six sentences about files, and the directory held none of them. The operator had the refusal's
    prose and nothing else, including no picture of the page that refused -- every guard runs
    before `write_png`, which is correct for a publish and was silently correct for a look.

    So the frames come here instead. Two properties are enforced below rather than trusted:

      * **Moved, not copied from a live stage.** `_PENDING` is emptied, so `publish()` has no
        entry left to copy even if a future caller reaches it with a broken refusal count. The
        bytes are no longer addressed by a staged path at all.
      * **Staleness cleared once, on opening, never again.** A frame from yesterday's refusal
        sitting beside today's is the same lie in a new costume, so whatever is in the directory
        when this run first opens it is deleted; what this run writes after that is left alone.

    Safe to call more than once: it opens the directory on the first call and moves whatever has
    been staged since the last. Returns the frames it moved this time, or None when it declined
    to write there -- and declining destroys them, the old behaviour, because a keep directory
    outside `runs/` or overlapping `--out` is a frame about to be read as evidence, which is worth
    less than the bytes.
    """
    global _REFUSED
    real_runs = op.realpath(RUNS)
    real_keep = op.realpath(keep)
    inside = real_keep != real_runs and (real_keep + op.sep).startswith(real_runs + op.sep)
    real_out = op.realpath(out) if out else None
    overlaps = real_out is not None and (real_keep == real_out
                                         or real_keep.startswith(real_out + op.sep)
                                         or real_out.startswith(real_keep + op.sep))
    if not inside or overlaps:
        why = (f"{keep} is not inside {real_runs}" if not inside else
               f"{keep} is the directory this run publishes to (--out {out})")
        operator_print(f"REFUSED-KEEP: {why}, so a frame left here could be read as the evidence "
                       "this run refused to write. The frames it measured are destroyed instead.")
        discard_stage()
        return None
    if _REFUSED != real_keep:
        os.makedirs(real_keep, exist_ok=True)
        stale = sorted(f for f in os.listdir(real_keep)
                       if f.endswith(".png") or f == REFUSED_NOTE)
        for name in stale:
            os.remove(op.join(real_keep, name))
        _REFUSED = real_keep
        if stale:
            operator_print(f"  {op.basename(real_keep)}/ cleared {len(stale)} file(s) from an "
                           "earlier refusal; everything in it now came from this run")
    stage = _stage_dir()
    moved = []
    for name in sorted(os.listdir(stage)) if op.isdir(stage) else []:
        dest = op.join(real_keep, name)
        os.replace(op.join(stage, name), dest)
        moved.append(dest)
    _PENDING.clear()
    if op.isdir(stage):
        # An empty stage is what `publish` would walk; leave nothing behind to walk.
        try:
            os.rmdir(stage)
        except OSError:
            pass
    if note:
        with open(op.join(real_keep, REFUSED_NOTE), "a") as fh:
            fh.write(note)
    return moved


def operator_print(text):
    """One sentence to the operator, spoken from inside a refusal handler, that cannot raise.

    Every disposal in this package tells the operator what happened with `print`, and every one of
    those sentences is spoken while the run is already standing inside somebody else's failure. A
    `print` can die -- a closed stderr, an `EPIPE` from `| head`, a device that stopped taking writes
    -- and when it dies the ordinary way the exception lands in the handler that was catching the
    REFUSAL. #317 measures what that costs on `origin/main` at `5be86e5`: both single-frame drivers'
    fixture arms end in `raised:OSError: [Errno 9]` with an empty stderr, and one dying `REFUSED:`
    print in the sweep escapes `main()` at the first refused page, taking every page after it, the
    count sentence, and `REFUSED.txt`. All three exit 1, because an uncaught exception exits 1 --
    the same code a refusal was supposed to produce, which is what launders it.

    So the rule `refuse_single_shot`'s docstring has always stated -- "`report_refusal` and
    `capture_refused` both report and never raise" -- is enforced here rather than by intent. That
    docstring called it "a wish about a `print`", and it was.

    One refused write is not a dead stream: measured for #317, the run whose `REFUSED:` headline died
    still printed the sentence after it, and the run whose pointer sentence died had already landed
    two lines above it. So this does not conclude "stderr is gone" and give up -- the NEXT sentence
    gets its own attempt at stderr, which is what keeps a single failed write from costing the whole
    report. Only the line that died is moved, and it is repeated on stdout WHOLE rather than
    summarised or dropped: an operator reading a transcript should not have to notice which sentence
    is missing, and a refusal that arrives on the wrong stream is still a refusal. If stdout is gone
    as well the line is lost and nothing is raised, because a diagnostic never outranks the refusal it
    is reporting -- the same reason `capture_refused` returns None instead of propagating.

    Returns True when the line reached stderr, False when it had to be moved or was lost. That is all
    a caller can usefully do with the answer; the point of this function is that the caller is never
    interrupted by it.
    """
    try:
        print(text, file=sys.stderr, flush=True)
        return True
    except (Exception, SystemExit) as died:  # SystemExit is not under Exception, and it is a real one
        why = f"{type(died).__name__}: {died}"
    try:
        print(f"  stderr refused the line below ({why}); it is repeated here, whole, because a run "
              "that cannot say out loud what it refused is the failure this package exists to end",
              file=sys.stdout, flush=True)
        print(text, file=sys.stdout, flush=True)
    except (Exception, SystemExit):
        pass  # both streams are gone: the refusal still stands, and it still exits 1
    return False


def capture_refused(cap, path):
    """Photograph the page a refusal just rejected, straight to a path nothing publishes from.

    Every guard in a driver runs BEFORE the write, and that is right -- a photograph of a page in
    a state nothing declares must not be published -- but it also meant the refusing page was
    never photographed at all. The refusal is the only thing a driver says about a page, so that
    sentence should travel with the picture it is a sentence about. Measured 6 Nov: `/` was
    refused for reading `locked out until 18h 16m` where the fixture declares `locked out until
    00:00`, and no amount of rerunning the recipe would have shown anyone the page.

    This is deliberately NOT `session.write_png`: that stages into `_PENDING`, which is the queue
    `publish()` copies into the artifact directory. These bytes go to the path named, full stop,
    and the path comes from `refused_dir`.

    It reports and does not raise, for the reason `report_refusal` gives for its own shape. This
    runs from inside a refusal handler, and a diagnostic that dies replaces the real cause with
    the diagnostic's own -- the browser is exactly as likely to be gone as still up at this
    moment, since `require_running` raising is one of the refusals that gets here.
    """
    try:
        cap.require_running()
        data = cap.session.screenshot()
        with open(path, "wb") as fh:
            fh.write(data)
    except (Exception, SystemExit) as exc:  # noqa: BLE001 -- a diagnostic never outranks the refusal
        # `SystemExit` is spelled out because it is not under `Exception`, and it is the arm this
        # call is most likely to take: `require_running` raises it, and a dead browser is one of the
        # refusals that reaches here. The rule that found this (`a dead browser still leaves the
        # refusal standing`) is the reason it is not left as a bare `except Exception`.
        operator_print(f"  no frame of the refused page: {type(exc).__name__}: {exc}")
        return None
    operator_print(f"  what the page actually renders is at {path} -- this run refused to publish "
                   "it, so it is not evidence; read it with REFUSED.txt beside it")
    return path


def refusal_note(mode, out, envs, notes):
    """REFUSED.txt: what the run refused, under what conditions, and what it left next to this file.

    The frames in the refused directory are worthless on their own. A PNG with no sentence next to it
    is the exact thing #211 was filed about -- a frame that looks current and states nothing about
    the state it was taken in -- and these frames state less than most, because the whole reason
    they exist is that the state was NOT the declared one. So the sentences and the conditions come
    here, in the directory, where they travel with the bytes.

    The conditions are named by their files rather than copied out. `fixture.env` is still on disk
    beside the run directory and is the only thing that can change; a copy made here would be a
    second answer to the same question, and #213 is what those two do when they drift.

    This used to be `pages.py`'s `_refusal_note`. It moved here when #296 wired the two single-frame
    drivers into the same keeping, because the text is the thing that makes a kept frame mean
    something and three drivers writing three versions of it is how one of them ends up saying less
    than the others.
    """
    when = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    lines = [
        "# What this run refused, and what it left here.\n",
        "#\n",
        "# These are the frames a run measured and then declined to publish, and they are\n",
        "# not evidence: nothing records them, --out was left untouched, and any artifact\n",
        "# path this run printed names a file that is not there. Read them to see what the\n",
        "# page actually rendered; never quote them as current.\n",
        f"\nfixture: {mode}\nrefused at: {when}\n--out (untouched): {out}\n",
        "conditions stated in:\n"
        + ("".join(f"  {p}\n" for p in envs) if envs
           else "  (none: no setup script has run, so no fixture.env declares anything here)\n"),
        f"\nrefusals ({len(notes)}):\n" + ("".join(notes) if notes else "  (none recorded)\n"),
    ]
    return "".join(lines)


def refused_frame(cap, keep, out, note=None, frame=None):
    """Move what this run staged into `keep` with `note` beside it, then photograph the refused page.

    The order is the whole function: `keep_refused` opening the directory is what clears a previous
    refusal's frames, so a frame written before that open would be deleted by the run that wrote it.
    Then the page is photographed, because the refusal is the only sentence a driver says about a
    page and it should travel with the picture it is a sentence about.

    One call per refusal is the property its name keeps: this is the disposal of the stage on the
    refusal path, and `pages.py --self-test` counts it at exactly one for every refusal arm. Before
    #220 the disposal was `discard_stage()` and the count was pinned the same way, which is how
    `lock.py` and `testlock.py` were found to have been running theirs as dead code -- an exit code
    laundered the fact that the statement after a raising helper never ran. A keep that never runs
    leaves nothing on disk, so the count and the directory together are what make the statement live.

    Returns what `keep_refused` returned: the frames it moved, or None when it declined to write
    there and destroyed them instead -- in which case there is no directory to photograph into.
    """
    kept = keep_refused(keep, note=note, out=out)
    if kept is None or cap is None or not frame:
        return kept
    capture_refused(cap, op.join(keep, frame))
    return kept


def refuse_single_shot(cap, mode, out, name, frame, why):
    """The whole refusal arm of a single-frame driver: the sentence, the kept frames, the page.

    `lock.py` and `testlock.py` photograph one page each, so a refusal is one sentence about one
    file -- and it used to be a sentence about a file that had just been deleted, with the only other
    picture it named being the previous run's `lock.png` sitting at `--out`, which the same sentence
    declares stale (#296). Both halves of what #251 ended for `pages.py` are therefore this one call:

      * the staged bytes and `REFUSED.txt` go to `runs/<mode>-refused/`. On the ordinary path nothing
        is staged -- every guard runs before `write_png` -- so what lands is the frame of the page
        that refused, named `lock@360.png` rather than `lock.png` so it cannot be opened as the
        artifact;
      * `--out` is left exactly as it was, and `report_refusal` names the file standing there with
        its mtime instead of deleting it (#222: with `--out` pointing at the docs tree, that file is
        the committed evidence).

    Reported, then kept, then photographed, in the order the transcript is read: the refusal, the
    stale file it did not write, the frame it did take, the count of what is in the directory.
    `report_refusal` and `capture_refused` both report and never raise, because this runs from inside
    a refusal handler and a diagnostic that dies replaces the real cause with the diagnostic's own.

    That rule used to be a wish about a `print`, and a `print` can die: a closed stderr, an `EPIPE`
    from `| head`, and the sentence after the note is on disk raises `OSError`. Two things now hold it
    up instead of intent. `cdp.operator_print` is how every sentence below is spoken, so a stream that
    refuses a write moves that line to stdout rather than raising into the caller's own `except`
    handler; and `SingleRefusal` below wraps the whole disposal, so a step that can raise for a
    non-print reason -- `keep_refused` moving bytes with `os.replace`, `refused_dir` creating the
    drawer -- cannot escape `main()` either. #317 measures the version with neither: the fixture arm
    of both drivers ending `raised:OSError: [Errno 9]` with nothing on stderr at all.

    `SingleRefusal` also keeps #304's symptom out: a disposal that raises inside a nested handler used
    to be caught by the outer one and read as a second refusal, appending a second `refusals (1):`
    block whose sentence quoted the diagnostic instead of the page.
    """
    keep = refused_dir(mode)
    report_refusal(name, why)
    env = op.join(RUNS, mode, "fixture.env")
    kept = refused_frame(cap, keep, out,
                         note=refusal_note(mode, out, [env] if op.exists(env) else [],
                                            [f"  refused: {why}\n"]),
                         frame=frame)
    if kept is None:
        # The keep was declined (nowhere safe to write), so the frames are gone and there is no
        # directory to count. `keep_refused` has already said why this one was destroyed.
        return None
    left = sorted(f for f in os.listdir(keep) if f.endswith(".png")) if op.isdir(keep) else []
    if left:
        operator_print(f"  {len(left)} frame(s) this run kept are in {keep}, with {REFUSED_NOTE} "
                       f"beside them; nothing was written to {out}, and none of these frames is "
                       "evidence")
    else:
        # The refusal came from the fixture, before there was a page to photograph. "0 frame(s)" would
        # be a true sentence that answers the wrong question -- the operator is looking for a picture
        # and is owed the reason there is none, not a zero. The reason has TWO cases though, and the
        # sentence used to name only one of them: `capture_refused` reports a failed photograph and
        # returns None instead of raising, so an empty drawer also means the page WAS up and the
        # browser is what failed. Measured 11 Nov with the fake session refusing one screenshot: the
        # transcript told an operator whose browser had just answered a page that it "was refused
        # before it had a page in a browser to photograph", which sends them to `lock-setup.sh` for a
        # websocket problem. `cap is None` is the fixture, and nothing else -- which is true of these
        # two drivers only because `SingleRefusal` suppresses the retry: the outer arm also disposes
        # with `cap=None`, and before #304 that second call reached this same sentence for a run that
        # had a live browser in its first attempt.
        why_none = ("this run was refused before it had a page in a browser to photograph"
                    if cap is None else
                    "the page was up and the photograph of it is what failed -- the line above this "
                    "one is what the browser answered")
        operator_print(f"  no frame could be taken, so {keep} holds none: {why_none}. "
                       f"{REFUSED_NOTE} is there with the refusal and the conditions; nothing was "
                       f"written to {out}")
    return kept


def refusal_reason(exc):
    """The one sentence a refusal is stated in, from either exception shape a driver catches.

    `SystemExit` already carries the operator's sentence, so it is printed as written; anything else
    has to name its type, because a bare `the connection was reset` in `REFUSED.txt` is a `RuntimeError`
    or an `OSError` and the difference is the diagnosis. Both single-frame drivers catch the two shapes
    in two nested handlers and now a third time in the outer one, so six call sites used to spell this
    expression out -- and a note whose sentence depends on which handler happened to catch the page is
    the same drift `refusal_note` was moved here to stop.
    """
    return str(exc) if isinstance(exc, SystemExit) else f"{type(exc).__name__}: {exc}"


class SingleRefusal:
    """One disposal per refusal for a driver that photographs one page, decided in one place.

    `lock.py` and `testlock.py` catch their refusal twice, and they have to: the inner arms sit
    inside the `with` because `capture_refused` needs the browser this run opened, and the outer arm
    catches `Exception` as well because a fixture that refuses before a browser exists still owes the
    operator the note and the stale-file sentence. Nested handlers mean one more thing, though --
    anything that raises *inside* the first disposal is caught by the outer arm and read as a second
    refusal. Measured 11 Nov on `origin/main` at `1e31cac`, driving the real `lock.py` `main()` with a
    faked browser and a stderr that dies once (the pointer sentence `capture_refused` prints after the
    note has already landed): `REFUSED.txt` came back with two `# What this run refused` headers and
    two `refusals (1):` blocks, the second block's sentence was `OSError: [Errno 9] Bad file
    descriptor` rather than the page's, `keep_refused` had run twice against a drawer holding one
    frame, and the last thing the operator read was a refusal of the diagnostic. #304.

    So this object holds the flag the outer arm checks, and it lives here rather than in each driver
    for the reason #296 moved the note into `refusal_note`: the two drivers are one shape, and a rule
    written twice is a rule one of them can stop meaning.

    The flag is set BEFORE the call, never after. A disposal that died is still a disposal -- it
    wrote the note and moved the stage, and what killed it was its last sentence -- so the retry is
    the thing being refused, not something to be earned by a clean exit. A driver that wants to know
    whether it was refused reads `disposed`; nothing else in the package does.
    """

    def __init__(self, mode, out, name, frame):
        self.mode = mode
        self.out = out
        self.name = name
        self.frame = frame
        self.disposed = False

    def refuse(self, cap, why):
        """Dispose of this run once, and never let the disposal leave the driver.

        Two properties, one flag and one `try`.

        **Once.** Returns what `refuse_single_shot` returned on the one call this makes, and None on
        every call after it. On those, `why` is no longer the page refusing -- it is whatever else
        raised between the two handlers, and since #317 a dying report is not one of those, because
        the report cannot raise. The sentence is still chosen from what is actually on disk rather
        than from the flag: a disposal that died before the note never wrote it, and telling the
        operator that a note exists which does not is the same invention in the opposite direction.
        (`pages.py --self-test` keeps both branches honest: one where the note is there and one where
        the disposal wrote nothing, both reached by a second event after the first disposal.)

        The failed disposal is not retried when it wrote nothing either. Whatever stopped it -- a
        drawer that cannot be created, a directory that will not open -- stops the retry in the same
        place, and a second block in a note that does not exist buys nothing. Saying what is missing
        costs nothing and cannot fail twice.

        **Never leaves the driver.** The call is wrapped, so an `OSError` from the file machinery
        below becomes a sentence and a `None`, not a traceback out of `main()`. That is the half
        #304's once-flag could not do: the flag decides whether a disposal runs twice, and it says
        nothing about a disposal that escapes. #317 measured both drivers' fixture arms -- the arm
        with a single handler, so nothing was left downstream to catch it -- ending
        `raised:OSError: [Errno 9] Bad file descriptor`, and the sweep losing every page after the
        first refused one. `pages.py --self-test`'s rules "a refusal whose report cannot be written
        still finishes its disposal" and "a refusal in a fixture still disposes of itself once" grade
        it, one per arm.
        """
        if self.disposed:
            note = op.join(refused_dir(self.mode), REFUSED_NOTE)
            if op.isfile(note):
                operator_print(f"  this refusal was already disposed of: something raised after it "
                               f"({why}), and a second disposal would append a second block to "
                               f"{note} for one refusal. {note} holds the one note this run wrote, "
                               f"and nothing was appended to it.")
            else:
                operator_print(f"  this run's refusal was disposed of once already, and that "
                               f"disposal wrote nothing: the second refusal-shaped failure reached "
                               f"the outer handler ({why}) with no {REFUSED_NOTE} on disk. There is "
                               f"no {REFUSED_NOTE} for this refusal and it will not be retried, "
                               f"because whatever stopped the first disposal stops the second; "
                               f"anything this run staged is still in the stage, and nothing was "
                               f"written to {self.out}.")
            return None
        self.disposed = True
        # The disposal is the last thing a refusal does, and it is allowed to die. `keep_refused`
        # moves bytes with `os.replace`, `refused_dir` creates a directory, and the drawer is counted
        # with `os.listdir` -- three ways an `OSError` reaches the handler that caught the REFUSAL,
        # none of them a print. Before #317 every one of them escaped `main()`: an uncaught exception
        # exits 1, the code a refusal was supposed to produce, and the driver's own `return 1` -- the
        # one statement proving it finished deciding -- never ran.
        try:
            return refuse_single_shot(cap, self.mode, self.out, self.name, self.frame, why)
        except (Exception, SystemExit) as exc:  # noqa: BLE001 -- a dying disposal is not a second refusal
            died = refusal_reason(exc)
            note = op.join(refused_dir(self.mode), REFUSED_NOTE)
            # Read off the disk, never off the attempt: a disposal can die before the note or after
            # it, and D-0065's lesson is that the sentence has to say which. `op.exists` rather than
            # `op.isfile` here is not a spelling preference -- the check above is an attack anchor and
            # a second identical line would make it match twice, which this repository's attack suite
            # refuses rather than resolving.
            told = (f"{note} was written before it died, and the refusal is in it"
                    if op.exists(note) else
                    f"no {REFUSED_NOTE} was written, so this refusal has no note beside its frames")
            operator_print(f"  this run's refusal was disposed of ONCE and that disposal died "
                           f"({died}) part way through: {told}, {self.out} was not written, and "
                           f"nothing this run staged was published. The refusal stands, the driver "
                           f"returns 1, and a report that dies is neither a second refusal nor a "
                           f"traceback.")
            return None


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


# The conditions a frame can be taken under, as a fixture declares one. One table, here, because
# this is the file that acts on it: `manifest.py` reads it out of here the same way it reads the
# page list out of `pages.py`, so a renamed or added spelling cannot survive in the recorder as a
# condition no driver is able to produce.
SCRIPT_VALUES = ("on", "off")


def scripts_disabled(env, where):
    """Does this fixture ask for the PAGE's own scripts to be switched off while it is photographed?

    The answer used to be a constant — `Emulation.setScriptExecutionDisabled` was called with
    `value=True` whatever the fixture declared — which made `PLOCK_CAPTURE_SCRIPTS` a declaration
    nothing read. The manifest then recorded a fact about this file rather than about the frame, and
    the knob `post-fixture.sh` advertised (`PLOCK_CAPTURE_SCRIPTS=on` to test a frame taken with
    scripts running) took exactly the same pixels as the default. That is how #213's finding survived
    four gates: an assertion resting on a control with no power cannot be checked by anyone told to
    go and check it.

    Absent means `off`, and that is the strict direction: this folder's criterion is "usable at 360 px
    with JavaScript off", so a fixture that names nothing gets the frame the criterion names. The
    recorder still writes no `scripts` key for a fixture that declared none (#211's
    absent-is-not-a-default rule is the recorder's business, not the driver's).

    Anything outside the two values is refused rather than guessed, for the reason the Chrome flag
    below is refused: a capture that looks configured is worse than one that fails.
    """
    raw = env.get("PLOCK_CAPTURE_SCRIPTS")
    value = (raw or "off").strip().lower()
    if value not in SCRIPT_VALUES:
        raise SystemExit(
            f"{where} declares PLOCK_CAPTURE_SCRIPTS={raw!r}, which is not one of "
            f"{' or '.join(repr(v) for v in SCRIPT_VALUES)}. Refusing rather than defaulting: the "
            "manifest would record the typo as written and the browser would photograph the default, "
            "so one run would produce a file and a frame that disagree.")
    return value != "on"


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

        `mobile=True` travels with it, so at 1024 px this is a *wide mobile viewport* rather than a
        desktop browser. That is the price of varying one thing, and it is right for this
        stylesheet, which branches only on `max-width: 40rem`, `prefers-color-scheme` and
        `prefers-reduced-motion`. A future rule keyed on `(hover: hover)` or `(pointer: fine)` would
        land outside what this criterion measures, and would need its own emulation parameter rather
        than a wider `width`.
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
        """Evaluate in the page and hand back the value, or `threw: <what>` when it did not run.

        The `threw:` shape is what the callers already test for, and it has to cover the failure
        that never throws. A script with a syntax error is rejected before a single node is looked
        up, so there is no exception object and no `result.value` -- Chrome reports it in
        `exceptionDetails`, which used to be dropped on the floor here. `value()` then answered
        `None`, a caller asking `got.startswith("threw")` got a clean False, and the driver printed
        "the refusal form vanished" about a form that was sitting in the page. The parse error is
        the finding, so it goes into the same string the runtime throws go into.
        """
        out = self.call("Runtime.evaluate", expression=expr, returnByValue=True)
        bad = out.get("exceptionDetails")
        if bad:
            exc = bad.get("exception") or {}
            return "threw: " + (exc.get("description") or exc.get("value") or bad.get("text")
                                or "the page refused to evaluate the script")
        return out["result"].get("value")

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


# The control census, spliced into the same measurement as the cell and form censuses so a page is
# read once and the readings cannot come from different renders (#202).
#
# It reports the page's own font beside every control's, because the claim being checked is
# agreement, and agreement needs both sides: `#202` measured every typed control at
# `Arial 13.3333px` while the prose around it was `system-ui 16px`, and a rule that compared each
# control against a font this file hardcoded would keep passing the day the page's font changed.
#
# `typed` is decided by *excluding* the controls that draw no value (`radio`, `range`, a
# `select-one`...), which is the opposite of how `app.css` lists them -- the stylesheet lists the
# types you type into, positively, so that a type added to a template and missing from the rule
# fails loudly instead of silently inheriting `width: 100%`. The two lists are deliberately
# asymmetric: the place they agree is measured, and the place they drift apart is a refusal.
#
# `label` carries the computed `display` of the label wrapping a toggle, or "" when there is none.
# It is how `:has()` gets measured rather than trusted: a selector the engine has never heard of is
# dropped without a word and leaves the page exactly as it was, and `display` is the only thing in
# the browser that says whether the rule applied.
#
# Hidden controls are skipped, and so is anything the page did not lay out at all (zero by zero):
# the census is about what the photograph shows.
CONTROLS_JS = (
    "'controls':(function(){"
    "var NOVALUE=['hidden','radio','checkbox','submit','button','reset','image','range','color',"
    "'select-one','select-multiple'];"
    "var page=getComputedStyle(document.body);"
    "var items=[];"
    "Array.prototype.forEach.call(document.querySelectorAll('input,select,textarea,button'),"
    "function(el){"
    "var t=(el.type||'').toLowerCase();"
    "if(t==='hidden'){return;}"
    "var r=el.getBoundingClientRect();"
    "if(r.width===0&&r.height===0){return;}"
    "var s=getComputedStyle(el);"
    "var lab=el.closest('label');"
    "items.push({tag:el.tagName.toLowerCase(),type:t,name:el.name||'',"
    "family:s.fontFamily,size:s.fontSize,max:s.maxWidth,"
    "typed:(el.tagName==='TEXTAREA'||NOVALUE.indexOf(t)<0),"
    "toggle:(t==='radio'||t==='checkbox'),"
    "label:lab?getComputedStyle(lab).display:''});});"
    "return {family:page.fontFamily,size:page.fontSize,count:items.length,items:items};})()"
)


def control_name(item):
    """One control, named the way a refusal has to name it: type, then the field it belongs to."""
    label = item["tag"] + (f"[type={item['type']}]" if item["type"] else "")
    return f"{label} {item['name']!r}" if item["name"] else label


def _listed(items, limit=4):
    """Up to `limit` names, then a count, because a refusal that scrolls is a refusal nobody reads."""
    head = ", ".join(control_name(i) for i in items[:limit])
    return head if len(items) <= limit else f"{head}, and {len(items) - limit} more"


def _count(n, singular):
    """`1 control` / `2 controls`. A refusal that says `1 control(s)` reads like a template."""
    return f"{n} {singular}" if n == 1 else f"{n} {singular}s"


def census_line(got):
    """What the #202 guards were shown, in the form every driver prints on its measurement line.

    This folder's house rule is that a rule which examined nothing must not report success silently,
    and a passing `require_control_width` on a page with nothing typed into it is exactly that: true,
    and about nothing. Printing `controls=15 typed=7 groups=2` is what tells the next reader which
    kind of pass they were looking at -- the same reason the #108 check prints how many Detail cells
    carried the note rather than only saying that it terminated cleanly. It lives here rather than in
    pages.py because the two single-page drivers print the same census on their own line, zeros and
    all: the verdict page and the probe page each carry one control today, the chrome's Sign out
    button, and a reader has to be able to see that the guards were shown nothing.
    """
    page = (got or {}).get("controls") or {}
    items = page.get("items") or []
    return (f"controls={len(items)} typed={sum(1 for i in items if i['typed'])} "
            f"groups={sum(1 for i in items if i['toggle'])}")


def require_control_font(got, what):
    """Every control that draws text is set in the page's own typeface (#202).

    The user-agent stylesheets give a form control the browser's own font, not the document's, so a
    stylesheet that never says otherwise renders an input in Arial 13.33 px in the middle of a page
    written in system-ui 16 px. That is what #202 measured on every photographed page: `text`,
    `password` and `number` boxes at `Arial 13.3333px` beside buttons at `system-ui 16px`.

    The check compares each control with the *page*, not with a constant, so it says nothing about
    which typeface the page chose -- only that a control did not choose one for itself.

    The second half of the harm is why this is a refusal and not a note: `ch` resolves against the
    element's own font, so `max-width: 22ch` is 163 px in Arial 13.33 px and 196 px in the page's
    16 px. A control in the wrong font is also measuring its own declared width in the wrong unit,
    which is how a stylesheet can say one width and render another.
    """
    page = got.get("controls")
    if page is None:
        raise SystemExit(f"{what}: asked whether the controls match the page when no control census "
                         "was taken for it. The census has to be spliced into the measurement "
                         "(cdp.CONTROLS_JS), and a check that reads a key nobody measured is a "
                         "check that always passes (#202).")
    bad = [i for i in page["items"] if i["family"] != page["family"] or i["size"] != page["size"]]
    if bad:
        raise SystemExit(
            f"{what} sets {len(bad)} of its {page['count']} visible controls in a font of the "
            f"browser's choosing -- {_listed(bad)} -- while the page's own prose is "
            f"{page['family']} at {page['size']}. `font: inherit` is on the control, or it is not "
            "(#202).")


def require_control_width(got, what):
    """Every control you type into is sized by one rule, not by whichever one matches its type.

    The observable that says *which rule* made a width is the computed `max-width`, and it is the
    one available without reading the stylesheet: no user-agent sheet puts a `max-width` on an
    input, so a control reporting `none` is sitting at the browser's default width of about twenty
    characters with nothing in `app.css` deciding it. Two controls reporting two different values
    means two rules are sizing fields on the same page.

    #202 measured both shapes at once on /settings: the number box at 65 px, the two path boxes at
    163 px from the file's only input rule, and three password boxes at 185 px from the browser --
    three widths on one page, which is the misalignment #199 had reported as happening "in some
    places".

    A page with nothing typed into it is not a pass, it is a page with nothing to say; the count is
    printed on every page's line (`controls=15 typed=0 groups=0`) so a transcript shows whether
    this ran on anything rather than leaving a reader to guess which kind of pass it was.
    """
    page = got.get("controls")
    if page is None:
        raise SystemExit(f"{what}: asked how the fields are sized when no control census was taken "
                         "for it. The census has to be spliced into the measurement "
                         "(cdp.CONTROLS_JS) (#202).")
    typed = [i for i in page["items"] if i["typed"]]
    if not typed:
        return
    by_max = {}
    for item in typed:
        by_max.setdefault(item["max"], []).append(item)
    if len(by_max) == 1 and "none" not in by_max:
        return
    loose = by_max.get("none") or []
    strict = {m: v for m, v in by_max.items() if m != "none"}
    if loose and not strict:
        problem = (f"no `max-width` on {_listed(typed)}, so each sits at the user-agent's own "
                   "default of about twenty characters -- this stylesheet deciding nothing")
    elif loose:
        sized = next(iter(strict.values()))
        problem = (f"{_count(len(loose), 'control')} carry no `max-width` and sit at the "
                   f"user-agent's default of about twenty characters ({_listed(loose)}) while "
                   f"{_count(len(sized), 'control')} are sized by a rule ({_listed(sized)})")
    else:
        detail = "; ".join(f"`{m}` on {_listed(v)}" for m, v in sorted(by_max.items()))
        problem = f"their `max-width` resolves to {len(by_max)} different values -- {detail}"
    raise SystemExit(
        f"{what} sizes the {_count(len(typed), 'control')} you can type into from "
        f"{len(by_max)} {'source' if len(by_max) == 1 else 'sources'}: {problem}. One width model "
        "is what #202 asks for; three field widths on one page are the defect.")


def require_choice_groups(got, what):
    """A tick box and the words that name it are one box (#202).

    Every radio and checkbox in this UI is written `<label><input> On — enforce the schedule</label>`
    and `app.css` makes such a label an atomic inline-level box, because an inline label breaks at
    any space and a box can therefore be photographed at the end of one line with its sentence on
    the next.

    This is the effect of a `:has()` selector, measured rather than grepped. The stylesheet reaches
    these labels by what is inside them -- the templates carry no class that says "choice group" --
    and a `:has()` rule a browser does not implement is discarded without a word, leaving the page
    exactly as it was before it and every grep in the repository still satisfied. Computed
    `display` is what the browser actually applied, so that is what is compared.

    A toggle with no wrapping label at all is refused with the others rather than skipped: there is
    nothing holding that box to its words, which is the defect, not an exemption from the check.
    """
    page = got.get("controls")
    if page is None:
        raise SystemExit(f"{what}: asked whether the choice groups are grouped when no control "
                         "census was taken for it. The census has to be spliced into the measurement "
                         "(cdp.CONTROLS_JS) (#202).")
    toggles = [i for i in page["items"] if i["toggle"]]
    loose = [i for i in toggles if i["label"] != "inline-flex"]
    if loose:
        naked = [i for i in loose if not i["label"]]
        wrapped = [i for i in loose if i["label"]]
        parts = []
        if naked:
            parts.append(f"{_listed(naked)}: no wrapping label at all")
        if wrapped:
            parts.append(f"{_listed(wrapped)}: its label is left as `display: "
                         f"{wrapped[0]['label']}`")
        raise SystemExit(
            f"{what} has {_count(len(toggles), 'radio/checkbox control')}, and these are not one "
            f"box with the words that name them ({' | '.join(parts)}). A choice group that is still "
            "an inline run of text lets the box wrap away from the words it belongs to (#202).")


# The navigation census, spliced into the same measurement as the control census (#204).
#
# One row per `<nav>` element, and every anchor inside it with the four properties #204's
# criterion names -- `color`, `font-weight`, `text-decoration-line`, `border-bottom-width` -- plus
# the background it names as acceptable instead, the `aria-current` value the shell writes, and the
# box. The box is here because criterion 4 is a position claim ("the marked entry is inside the
# header on all five pages"), and a property comparison cannot answer a position claim; the
# header's own rect comes back with it so "inside" is a comparison rather than an opinion.
#
# `vis` is `checkVisibility`, not `offsetParent`: an anchor can be legitimately hidden by the page
# (a template that renders a link only when there is something to link to) and #204 counted what a
# reader sees. Chrome's own answer, with the `visibility` property included so a link hidden by
# the stylesheet is not counted as a link the palette has to explain.
NAV_JS = (
    "'nav':(function(){"
    "var rows=[];"
    "Array.prototype.forEach.call(document.querySelectorAll('nav'),function(n){"
    "var ncs=getComputedStyle(n);"
    "var items=[];"
    "Array.prototype.forEach.call(n.querySelectorAll('a'),function(a){"
    "var s=getComputedStyle(a);var r=a.getBoundingClientRect();"
    "items.push({text:(a.textContent||'').replace(/\\s+/g,' ').trim(),"
    "href:a.getAttribute('href'),cur:a.getAttribute('aria-current'),"
    "color:s.color,weight:s.fontWeight,dec:s.textDecorationLine,"
    "bw:s.borderBottomWidth,bc:s.borderBottomColor,bg:s.backgroundColor,"
    "x:Math.round(r.x),y:Math.round(r.y),w:Math.round(r.width),h:Math.round(r.height),"
    "vis:a.checkVisibility===undefined?true:a.checkVisibility({visibilityProperty:true})});});"
    "rows.push({display:ncs.display,overflow:n.scrollWidth>n.clientWidth+1,items:items});});"
    "return rows;})(),"
    "'header':(function(){var h=document.querySelector('header');"
    "if(!h){return null;}var r=h.getBoundingClientRect();"
    "return {x:Math.round(r.x),y:Math.round(r.y),w:Math.round(r.width),"
    "h:Math.round(r.height)};})()"
)

# The two values a link has when no stylesheet has touched it. Chrome's UA sheet puts
# `color: -webkit-link` and `text-decoration: underline` on every anchor, and `-webkit-link`
# resolves to this. #204 measured exactly these two on all five photographed pages: every link
# `rgb(0, 0, 238)`, weight 400, underlined.
UA_LINK_BLUE = "rgb(0, 0, 238)"
UA_UNDERLINE = "underline"

# The smallest box that is a target rather than a glyph, in pixels.
#
# #199 is a complaint about a hand as much as about a colour: "when accessing from a phone ... the
# text is so small ... it is difficult to navigate". Measured on this stylesheet, a navigation entry
# is 26 px tall with `nav a`'s padding gone and 40 px with it -- 23.6 px of line box, 14.4 px of
# `.45rem` padding, 2 px of reserved border. `pages.py` transcribes the same 40 as its fixture's
# `NAV_HEIGHT`; this is the floor the guard refuses under, and the two agree on the number while
# doing different jobs, one a measurement of what the page renders and one a claim about what a thumb
# can hit. #229's review is why it is a number at all: deleting the padding line was green in every
# Go guard, every other capture guard, and a full nine-page capture run.
NAV_MIN_TARGET = 40


def _border_px(value):
    """The pixel width a computed `border-bottom-width` reports, or None when it is not a length.

    Chrome answers `2px`. A census from anything else could answer `2 px`, or a keyword like
    `medium`, and a parser that read those as 0 would turn "the browser is not drawing this border"
    into "it is drawing it at zero width" -- two different refusals collapsed into one wrong number.
    So a value this cannot read comes back as None and the caller says it could not read it.
    """
    text = str(value or "").strip().lower()
    if not text.endswith("px"):
        return None
    try:
        return float(text[:-2])
    except ValueError:
        return None


def _nav_rows(got, what):
    """The navigation rows, or a refusal naming the census that was never taken.

    The same shape as the three #202 guards, for the same reason: a check that reads a key nobody
    measured does not pass because the page is fine, it passes because nothing was ever compared,
    and the transcript cannot tell the two apart.
    """
    rows = (got or {}).get("nav")
    if rows is None:
        raise SystemExit(f"{what}: asked whether the navigation marks the current page when no "
                         "navigation census was taken for it. The census has to be spliced into "
                         "the measurement (cdp.NAV_JS), and a check that reads a key nobody "
                         "measured is a check that always passes (#204).")
    return rows


def nav_census_line(got):
    """What the #204 guards were shown, on the same line as the #202 census.

    `nav=5 links marked=Status overflow=0` -- the marked entry named by its own text rather than by
    an index, because the text is the only thing a reader of the transcript can check against the
    photograph below it.

    Where there is not exactly one mark the field is a COUNT, not a name: an empty navigation prints
    `nav=0 links marked=0` and a doubly-marked one prints `marked=2`. It said `marked=none` for the
    first of those until #229's review read the line the code actually writes, and the difference is
    not cosmetic -- `none` is a word this file uses for a state it did not measure, and a transcript
    reading `marked=none` on a page that measured zero marks would be the one place in the run where
    a reader could not tell an absence from a gap. Zero is the case this folder refuses to let pass
    silently (see `census_line`), and the refusal is the guard's job; this line's job is to report
    what the census saw, in the units it counted in.
    """
    rows = (got or {}).get("nav") or []
    items = [i for row in rows for i in row["items"]]
    marked = [i for i in items if i["cur"] == "page"]
    return (f"nav={len(items)} links marked={marked[0]['text'] if len(marked) == 1 else len(marked)}"
            f" overflow={sum(1 for row in rows if row['overflow'])}")


def nav_names(items, limit=4):
    """Navigation entries by their own text, the way a refusal has to name them.

    Not `_listed`, which names a *control* by its tag and `name` attribute: an anchor has neither
    in the way a reader of the transcript could check, and the artifact below the refusal line
    shows these links by their words. Falls back to the href for an anchor with no text, because
    the refusal still has to point at something.
    """
    def one(item):
        return repr(item["text"]) if item["text"] else f"href={item['href']!r} (no text)"
    head = ", ".join(one(i) for i in items[:limit])
    return head if len(items) <= limit else f"{head}, and {len(items) - limit} more"


def require_nav_marked(got, what, nav):
    """Exactly one entry per navigation row says "this is the page you are on" (#204), and it is the
    entry this page belongs to.

    `nav` is the href the fixture declares for this page's section, and it is what turns a count
    into criterion 1. Without it the strongest thing this function could say is "one of these five
    is marked", which a stylesheet gets right on every page by marking the first -- and nine
    photographs of a wrong mark are nine photographs of the wrong page. The alternative source for
    that string, deriving the section from the URL here, would be a second copy of the segment rule
    `internal/web/sources.go` already owns; D-0025 refused that duplication for the browser, and
    the same reason applies a layer out.

    The count is the whole of #199's first sentence -- "indicate clearly which page is currently
    selected" -- and #204 measured it as zero everywhere: five links per page, none carrying
    `aria-current`, on all five photographed pages.

    Zero and two are different failures and both are refused by name. Zero is the reported defect.
    Two is what happens when the site's name stays an anchor to the root while the row also carries
    an entry for the root: two elements marked current on one page turns the mark into decoration,
    which is the belief the issue was filed against.

    Then the position, because criterion 4 is a position claim and property comparisons cannot
    answer it: the marked box has to be inside the header's box. A navigation moved out of the
    header -- into a footer, or into a `<div>` after `</header>` -- would keep marking the right
    entry and would satisfy every property check in this file while photographing something the
    report did not ask for.

    Then the box's own height, because #199's first sentence is about a thumb and not only about an
    eye. `NAV_JS` has always recorded `h` per entry and the header check has always read `y`, and
    until #229's review nothing in the repository read `h`: deleting `nav a`'s padding left every Go
    guard green, left `require_nav_in_palette` and `require_nav_distinguishable` green (padding moves
    no colour, weight, decoration or border), and let a full nine-page capture exit 0 while the
    profiles frame genuinely moved 17 px. A census that measures a box and refuses nothing about it
    is a census that was never asked the question. The floor is the marked entry's own -- the one a
    reader is aiming at -- and `NAV_MIN_TARGET` says why 40.

    What this does NOT ask is whether the marked entry's *name* is the page's name. The row's first
    entry reads "Status", its `<title>` reads "Status", and no heading on the page says either word
    (#219). That is a page-identity question and a census of computed style cannot settle it.
    """
    rows = _nav_rows(got, what)
    header = (got or {}).get("header")
    for row in rows:
        items = [i for i in row["items"] if i["vis"]]
        marks = [i for i in items if i["cur"] == "page"]
        if len(items) < 2:
            raise SystemExit(
                f"{what} has a <nav> holding {len(items)} visible links, so nothing in it can be "
                "marked out of a set. #204 asks which entry is the current page *of the row*; a row "
                "of one has no other entry, and a page that lost its links fits at any width "
                "(#140's lesson, applied to the header).")
        if not marks:
            raise SystemExit(
                f"{what} has {len(items)} links in its navigation and marks none of them current. "
                "No entry carries `aria-current=\"page\"`, so the row cannot be told which page it "
                "is on by anything except reading the URL -- which is the defect #204 was filed "
                "about, measured as zero marks on all five photographed pages.")
        if len(marks) > 1:
            raise SystemExit(
                f"{what} marks {len(marks)} navigation entries current "
                f"({', '.join(repr(i['text']) for i in marks)}). `aria-current=\"page\"` is one "
                "fact about one location; two of them is the header containing a second route to "
                "the root, and the reader cannot tell which one the page is.")
        marked = marks[0]
        if not marked["text"]:
            raise SystemExit(
                f"{what} marks a navigation entry with no text as the current page. A screen reader "
                "announces the mark and nothing to announce with it, and the parent looking at the "
                "photograph sees a mark on blank space.")
        if not any(i["href"] == nav for i in items):
            raise SystemExit(
                f"{what} declares its section as {nav!r} and the rendered row offers no such href "
                f"({nav_names(items, limit=8)}). Either the shell lost an entry or this page's "
                "declaration is stale -- and with no entry for this page, there is nothing the mark "
                "could correctly be on, so criterion 1 cannot be answered at all.")
        if marked["href"] != nav:
            raise SystemExit(
                f"{what} is photographed in the {nav!r} section and marks {marked['href']!r} "
                f"({marked['text']!r}) current. Criterion 1 is about the link whose href resolves to "
                "the page being photographed; this is what a stylesheet that marks the first entry "
                "on every page looks like, and it would have been photographed, plausibly, nine times "
                "over.")
        if header is None:
            raise SystemExit(
                f"{what} marks an entry current and renders no <header> for the mark to be inside. "
                "#204's criterion 4 is that the mark is the header's current-page mark; measured "
                "against no header, that claim is about nothing.")
        if not (header["x"] <= marked["x"] and header["y"] <= marked["y"]
                and marked["y"] <= header["y"] + header["h"]):
            raise SystemExit(
                f"{what} marks {marked['text']!r} current at ({marked['x']},{marked['y']}) while "
                f"the header spans y={header['y']}..{header['y'] + header['h']}, so the mark is not "
                "the header's. A navigation moved out of the header keeps marking the right entry "
                "and photographs a different page than the one #204 asked about.")
        # The target, measured on the one entry the reader is aiming at. The comparison is on the
        # rounded number NAV_JS records -- the same one the census prints -- because a floor is a
        # claim about a thumb, and a thumb does not care about the tenth of a pixel the browser
        # rounded away.
        if marked["h"] < NAV_MIN_TARGET:
            raise SystemExit(
                f"{what} marks {marked['text']!r} current in a box {marked['h']} px tall, under the "
                f"{NAV_MIN_TARGET} px this row is required to offer. #199 is a complaint about "
                "navigating a phone with a thumb: the mark being on the right link, in the right "
                "colour, at the right weight, says nothing about the link being hittable. That is "
                "`nav a`'s padding -- 26 px without it, 40 with it, measured on this stylesheet -- "
                "and no other guard in this repository reads a box's height, which is how deleting "
                "it stayed green through nine photographs (#229).")


def require_nav_in_palette(got, what):
    """No link in the navigation is still the browser's own link (#204's first sentence).

    The scope is the navigation, and it is worth saying because the sentence above it does not carry
    it: #204's table is the five links in `<nav>` on five pages, so the loop below walks the rows
    `NAV_JS` censused and nothing else. A link in the page's own body that still computes to the
    user-agent's blue is outside this census -- `NAV_JS` selects `nav`, and a guard can only refuse
    what it was shown. Widening it would mean widening the census to every anchor on nine pages,
    which is a different claim about a different set of links than the one the report measured.

    #199 opens with "it doesn't match the color palette of the system" and #204 measured why: the
    stylesheet's only rule for `a` was `:focus-visible`, so every anchor kept Chrome's user-agent
    `color: -webkit-link` -- `rgb(0, 0, 238)` -- and its underline, next to a page that says
    `--ink: #1a1c20` in its first lines. The first committed artifact even shows the visited half:
    the site's name rendered `rgb(100, 0, 100)` because the capture browser had already been to the
    root and the UA's visited rule was the only rule about that anchor's color.

    Two properties, both the UA's signature, checked on every link in every navigation row. This
    refuses on the value, not on the presence of a rule: a stylesheet that reaches these links by
    any selector at all passes, and one that greps beautifully while a typo keeps the UA value on
    the page fails.

    The visited state is NOT observable here and this says so rather than pretending: a fresh
    profile has no history, so `a:visited` never matches in the capture browser. What protects it
    is the same author declaration -- an author `color` on the anchor outranks the UA's visited rule
    for the same reason it outranks the UA's unvisited one -- and the artifact above is why the
    check is worth having even so.
    """
    rows = _nav_rows(got, what)
    blue = []
    underlined = []
    for row in rows:
        for item in row["items"]:
            if not item["vis"]:
                continue
            if item["color"] == UA_LINK_BLUE:
                blue.append(item)
            elif item["dec"] == UA_UNDERLINE:
                underlined.append(item)
    if blue or underlined:
        parts = []
        if blue:
            parts.append(f"{nav_names(blue)} still user-agent blue {UA_LINK_BLUE}")
        if underlined:
            parts.append(f"{nav_names(underlined)} still carries the user-agent's underline")
        raise SystemExit(
            f"{what} has {len(blue) + len(underlined)} navigation link(s) the stylesheet never "
            f"claimed ({' | '.join(parts)}). The palette in `app.css` says `--ink: #1a1c20` and "
            "`--muted: #5b6270`; a link reading neither is not in the palette (#204, #199).")


def require_nav_distinguishable(got, what, nav):
    """The entry for this page differs from every sibling on something a browser agrees is visible.

    The first four properties are the issue's own list -- `color`, `font-weight`,
    `text-decoration-line`, `border-bottom-width` -- and the reason all of them are read is the
    failure this rule exists for: a stylesheet can mark the current page with a rule the engine
    throws away, or with a rule whose effect is invisible, and every grep in the repository stays
    green while the reader sees the same row. #204's wording is that the current link "does not
    differ from its siblings in any property a reader can see", so that is exactly the comparison.

    `border-bottom-color` is the fifth and it is here because the four as written cannot see the
    design this stylesheet chose: every link reserves a 2px border so marking the current one does
    not move the row, so `border-bottom-width` is `2px` everywhere and only its COLOR changes. A
    comparison limited to the width would call this page unmarked. Adding a property makes the
    guard more accepting, never less: it still refuses the case where nothing at all differs, which
    is the regression that actually ships.

    The obvious cheap compliance -- `text-decoration: underline` on the current page alone --
    satisfies "at least one of them" and is the worst answer available, because the row's links
    just stopped being underlined when they stopped being plain anchors. That is not written as a
    rule here: a rule saying "the mark must not be an underline" is a rule about one design, and
    the next designer argues with it and wins. It is written as the comment on the stylesheet rule
    instead, where the person editing that rule reads it.

    The reservation itself is the exception to that paragraph, and it is measured here rather than
    left to the comment, because #229's review showed what leaving it costs: deleting
    `border-bottom: 2px solid transparent` from `nav a` was green in the whole `internal/web`
    package AND green through a full capture run. With the reservation gone every link computes
    `0px`, the current-page rule's `border-bottom-color` is a colour on a border the browser never
    draws, and the comparison above still passes on colour and weight alone -- so the third visible
    part of the mark silently stops existing while every property named in the issue's own list
    still differs. That is not a sixth property added to the comparison, it is the fifth one made
    honest: `bc` only means anything on a border that is drawn.

    So two things are asserted about the width, on the whole row rather than on the marked entry
    alone. It is non-zero -- otherwise nothing is drawn -- and every entry computes the same value,
    because "marking the current entry cannot move the row's baseline" is a claim about the row: a
    width that differs between siblings is the design D-0035 refused, and it would have put 2 px of
    vertical jump on every page the shell renders.
    """
    rows = _nav_rows(got, what)
    for row in rows:
        items = [i for i in row["items"] if i["vis"]]
        # The entry this page belongs to, by href -- not "the marked one", which is the same row
        # only after require_nav_marked has run. Criterion 1 names the link "whose href resolves to
        # that page", so this guard asks about that link; in `shoot()` the marked guard runs first
        # and has already refused the case where the two disagree, and a caller that runs this one
        # alone gets no comparison rather than a second, weaker opinion about the wrong element.
        marks = [i for i in items if i["href"] == nav]
        if len(marks) != 1:
            continue  # require_nav_marked owns the count and the identity, and says more about both
        marked = marks[0]
        same = [i for i in items if i is not marked and all(
            marked[k] == i[k] for k in ("color", "weight", "dec", "bw", "bc"))]
        if same:
            raise SystemExit(
                f"{what} is in the {nav!r} section and that entry ({marked['text']!r}) looks exactly "
                f"like {len(same)} of its siblings ({nav_names(same)}) -- "
                f"color {marked['color']}, weight {marked['weight']}, decoration "
                f"{marked['dec']}, border-bottom {marked['bw']} {marked['bc']}. The attribute is in "
                "the DOM and nothing in the stylesheet turns it into something a parent can see, "
                "which is the half of #204 a grep of `app.css` cannot tell apart from a rule that "
                "works.")
        # The reservation the mark is drawn on, asserted on every entry rather than on the marked one.
        # `reserved` is None when the census reports a width this parser cannot read at all, and that
        # is said as itself -- a comparison that read `medium` as 0 would refuse with a sentence about
        # a border nobody drew.
        reserved = _border_px(marked["bw"])
        others = sorted({i["bw"] for i in items if i is not marked and i["bw"] != marked["bw"]})
        if reserved is None:
            raise SystemExit(
                f"{what} marks {marked['text']!r} current with a border-bottom-width the capture "
                f"driver cannot read ({marked['bw']!r}). The mark in this repository is drawn as the "
                "COLOUR of a reserved border, so an unreadable width is not a harmless extra digit: "
                "it means the census stopped reporting the number this guard compares.")
        if reserved <= 0 or others:
            why = (f"its border-bottom-width computes {marked['bw']!r}, so the browser draws no "
                   "border at all and the current page's `border-bottom-color` is a colour on "
                   "nothing" if reserved <= 0 else
                   f"the entries do not compute it the same: {marked['bw']!r} on "
                   f"{marked['text']!r} against "
                   f"{', '.join(repr(w) for w in others)} on its siblings")
            raise SystemExit(
                f"{what} marks the current entry on a border that is not the row's reservation: "
                f"{why}. `nav a` reserves one border on every link precisely so that marking "
                "the current entry cannot move the row's baseline, and this is the only place that "
                "reservation is measured: deleting `border-bottom: 2px solid transparent` was green "
                "package-wide and green through nine photographs, and a future rule that marks by "
                "width would put a vertical jump in the header of every page the shell renders "
                "(#229, D-0035).")


# The words the pages use to report a state the daemon reached (#199).
#
# This is a census of the templates, not a wish list: every name here is one a `.tmpl` file puts on
# an element, and `every_state_class_the_pages_render_is_in_the_census` below refuses the day one of
# them stops being rendered -- a guard that measures a class nobody emits passes on nothing. The
# matching seam on the Go side is `TestEveryStateClassTheTemplatesNameIsDrawn` in internal/web, which
# refuses the other direction: a word the pages say and app.css never mentions. Since #239 that seam
# reads the whole vocabulary named here, not only the `state-` prefixed half of it.
#
# `state-free` is deliberately absent. app.css has drawn it since before this folder existed and no
# template has ever rendered it, so it is the one entry in the stylesheet's state block a browser
# could not be asked about; the Go test asserts both halves of that sentence stay true.
#
# #239 moved two names in and moved no pixels out. `.problem` was carrying six jobs at once -- a
# refused write, a per-account fact, a backend's detail, the standing "enforcement is switched off"
# sentence, one paragraph of help prose and a missing value -- and `.override` carried both the open
# hold and the self-session sentence whose whole content is that nothing is held. Two of those jobs had
# no name, so they are in this census: `refusal` (the write that did not happen, the evaluator's
# complaint) and `exempt` (the session the daemon runs inside, which no schedule will lock). The other
# three went to names already listed here or to no state at all -- the mode notice to `disabled`, the
# missing value to `state-gap`, the help prose to `muted`, which is not a state and stays out. A name
# this list gains is a name Chrome is now asked about on every photographed page, which is the half of
# the guard a rename cannot quietly opt out of.
# The four row-level state names, named separately because nothing outside this file can find them.
#
# They are the finding the coverage rule in `pages.py --self-test` produced the first time it was run,
# and the reason that rule's first version was wrong. `dashboard.tmpl:33` writes
# `class="{{if .LockedOutNow}}held{{end}}{{if .CannotEvaluate}}unreadable{{end}}"` and `users.tmpl:43`
# writes `class="{{if .Stale}}stale{{end}}{{if not .Configured}}unmanaged{{end}}"`, so the four names
# exist only as the text BETWEEN template actions and carry none of the `state-` prefix. A reader that
# scans class attributes for literal words finds them; a reader that requires the prefix does not; and
# `app.css` on the head this branch started from had no rule for any of the four -- measured with
# `grep -n "tr\.\|tbody tr" internal/web/static/app.css`, which answered nothing at all. So a row whose
# account is switched off, unreadable, stale or unmanaged was marked only by whichever word one of its
# cells happened to hold, and the row -- the thing a parent's eye travels down when it is not reading --
# said nothing. That is #199's report one level above the sentence. The list is here by name because the
# browser is the only reader that can see a composed class on a page, and a census that silently omits
# four names is a census that passes on them.
ROW_STATE_CLASSES = ("held", "unreadable", "stale", "unmanaged")

STATE_CLASSES = ("state-held", "state-unknown", "state-gap", "problem", "refusal", "override",
                 "override-expired", "exempt", "disabled", "flag", "banner") + ROW_STATE_CLASSES


# The state census (#199): every element on the page carrying one of those classes, with the four
# properties that travel to it from the sentence it sits in -- computed for the element AND for its
# own parent -- plus the two that belong to its box.
#
# The parent is the whole point. #199's report is that the UI "is very basic", and the located cause
# on the pages a parent actually reads is that the stylesheet wrote a rule for one of these classes
# and left the rest. Measured in Chrome on the pre-fix tree, `/users` at both viewports: six state
# words on the page, ZERO drawn differently from the prose around them -- "in the configuration, not
# on the machine" (`state-unknown`), "not managed" (`state-gap`), "the account no longer exists"
# (`problem`) and the page's own banner paragraph. `/` drew one of its four, the red bold "locked
# out", beside a "no desktop session" in the same ink and weight as the sentence next to it.
#
# A class can be in the DOM, and named in the stylesheet's prose, and do nothing at all -- so the
# question is not "does app.css mention `.state-gap`", it is "does the browser draw this word
# differently from the sentence it is sitting in".
STATE_JS = (
    "'states':(function(){var out=[];"
    "[" + ",".join(f"'{c}'" for c in STATE_CLASSES) + "].forEach(function(c){"
    "Array.prototype.forEach.call(document.querySelectorAll('.'+c),function(e){"
    "var s=getComputedStyle(e),p=e.parentElement,ps=p?getComputedStyle(p):s;"
    "out.push({cls:c,text:(e.textContent||'').replace(/\\s+/g,' ').trim().slice(0,48),"
    "color:s.color,weight:s.fontWeight,style:s.fontStyle,dec:s.textDecorationLine,"
    "bw:s.borderLeftWidth,bg:s.backgroundColor,"
    "pcolor:ps.color,pweight:ps.fontWeight,pstyle:ps.fontStyle,pdec:ps.textDecorationLine,"
    "vis:e.checkVisibility===undefined?true:e.checkVisibility({visibilityProperty:true})});});});"
    "return out;})()"
)

# The four properties that travel from the text an element sits in, as
# (the name a refusal prints, the element's own key, its parent's key), and the two that are the
# element's own paint.
#
# The split is the whole rule. Colour, weight, style and decoration are INHERITED: an element that
# does not change them is being drawn in the voice of the sentence around it, and the parent's value
# is the right thing to compare against. A border and a background are not inherited -- they belong
# to the box -- so the honest question about them is what THIS element has, never what its parent
# has. Comparing them against the parent was the first version of this guard and it was wrong in the
# direction that matters: the `.problem` paragraph in `/settings` sits inside a `fieldset`, whose own
# border makes the child's `border-left-width` differ from it, so the guard called a rule-less
# `<p class="problem">` "drawn as a state" on a stylesheet that never mentioned the class. Measured
# on the pre-fix tree, 360 px and 1024 px alike.
STATE_PROPS = (("color", "color", "pcolor"), ("font-weight", "weight", "pweight"),
               ("font-style", "style", "pstyle"),
               ("text-decoration-line", "dec", "pdec"))

# What a box reports when nothing paints it: Chrome's answer for `background-color` at its initial
# value, and a border whose width is zero is not drawn whatever colour it is given.
STATE_NO_PAINT = ("rgba(0, 0, 0, 0)", "transparent")


# What a page that has its states drawn answers, transcribed from the measured `/` frame at 360 px
# after #199's block went into app.css (the values are Chrome's `getComputedStyle` output for the
# stylesheet as shipped, not a design sketch). `_fake_browser` in pages.py defaults this key the way
# it defaults `CLEAN_CENSUS` and `clean_nav`, so a rule about something else is not defeated by a
# guard it was never about -- and a rule that wants a defect passes its own `states` and wins.
CLEAN_STATES = [{"cls": "state-held", "text": "locked out", "color": "rgb(179, 38, 30)",
                 "weight": "600", "style": "normal", "dec": "none", "bw": "0px",
                 "bg": "rgba(0, 0, 0, 0)", "pcolor": "rgb(26, 28, 32)", "pweight": "400",
                 "pstyle": "normal", "pdec": "none", "vis": True},
                {"cls": "state-unknown", "text": "no desktop session", "color": "rgb(91, 98, 112)",
                 "weight": "400", "style": "italic", "dec": "none", "bw": "0px",
                 "bg": "rgba(0, 0, 0, 0)", "pcolor": "rgb(26, 28, 32)", "pweight": "400",
                 "pstyle": "normal", "pdec": "none", "vis": True},
                {"cls": "disabled", "text": "account switched off", "color": "rgb(132, 86, 0)",
                 "weight": "400", "style": "normal", "dec": "none", "bw": "0px",
                 "bg": "rgba(0, 0, 0, 0)", "pcolor": "rgb(26, 28, 32)", "pweight": "400",
                 "pstyle": "normal", "pdec": "none", "vis": True},
                {"cls": "banner", "text": "casey is locked out now", "color": "rgb(26, 28, 32)",
                 "weight": "600", "style": "normal", "dec": "none", "bw": "0px",
                 "bg": "rgba(0, 0, 0, 0)", "pcolor": "rgb(26, 28, 32)", "pweight": "400",
                 "pstyle": "normal", "pdec": "none", "vis": True}]


def _state_painted(r):
    """True when the element carries a mark of its own -- a border drawn, or a background painted."""
    return (_border_px(r["bw"]) or 0) > 0 or r["bg"].strip().lower() not in STATE_NO_PAINT


def _state_rows(got, what):
    """The state census, or a refusal naming the census that was never taken.

    The same shape as `_nav_rows` and the #202 guards: a check that reads a key nobody measured does
    not pass because the page is fine, it passes because nothing was compared.
    """
    rows = (got or {}).get("states")
    if rows is None:
        raise SystemExit(f"{what}: asked whether the page's states are drawn as states when no "
                         "state census was taken for it. The census has to be spliced into the "
                         "measurement (cdp.STATE_JS), and a check that reads a key nobody measured "
                         "is a check that always passes (#199).")
    return rows


def state_census_line(got):
    """`states=4 drawn=1` on the driver's measurement line, zeros included.

    `states=0` is a legitimate reading of a page that renders no state word -- `/audit` with rows in
    it is one -- and this folder's house rule is that a guard which examined nothing says so on the
    line instead of reporting a silence as a pass (see `census_line`). `drawn` is the number the
    guard was willing to accept, so `states=6 drawn=0` reads as what it is: six words reporting six
    states, none of them drawn differently from the prose around them.
    """
    rows = (got or {}).get("states") or []
    seen = [r for r in rows if r.get("vis")]
    return f"states={len(seen)} drawn={len(seen) - len(_state_lookalikes(seen))}"


def _state_lookalikes(rows):
    """The state elements the browser draws in exactly the voice of the text they sit in.

    An element passes when one inherited property differs from its parent's, or when it carries a
    border or a background of its own. Any one of those is something a parent can see, and the point
    of the rule is that a reader can tell the word apart from the sentence -- so this refuses the
    case where the stylesheet said nothing about the class, and prescribes nothing about which of
    the four channels a design should move.
    """
    bad = []
    for r in rows:
        inherited = [name for name, mine, parent in STATE_PROPS if r[mine] != r[parent]]
        if not inherited and not _state_painted(r):
            bad.append(r)
    return bad


def require_state_legible(got, what):
    """Every word the page uses to report a state has to look like a state.

    The refusal is the defect #199 reported, measured rather than read out of the stylesheet. On the
    pre-fix tree, at both viewports: `/users` reported six state words and drew none of them --
    "in the configuration, not on the machine" (`state-unknown`), "not managed" (`state-gap`), "the
    account no longer exists" (`problem`) and the page's own banner paragraph each computed the same
    colour, weight, style and decoration as the prose they sat in, with no border or background of
    their own. `/` drew one of its four: the red bold `state-held` "locked out", beside an italic-less
    muted-less "no desktop session" that a parent reads as the same kind of sentence.

    Nothing here says WHICH treatment a state gets. Colour-blindness, dark mode, and the difference
    between "cannot be decided" and "locked out" are all arguments about specific properties, and the
    last time this repository put one designer's answer into a guard -- `nav a:hover` above the
    current-page rule, #229 -- the guard had to grow three more properties to stay honest. The choice
    lives in app.css, where the person editing it reads it; the guard only refuses the case where the
    browser was told nothing.

    An element the browser does not draw is skipped, for the same reason #204 counted only visible
    links: a state the page is not showing cannot be looked at, and refusing it would mean the next
    template that renders a row conditionally has to style the case nobody sees.
    """
    rows = _state_rows(got, what)
    visible = [r for r in rows if r.get("vis")]
    bad = _state_lookalikes(visible)
    if not bad:
        return
    shown = ", ".join(f"{r['cls']!r} {r['text']!r}" for r in bad[:4])
    more = "" if len(bad) <= 4 else f", and {len(bad) - 4} more"
    one = bad[0]
    raise SystemExit(
        f"{what} reports {len(bad)} of {len(visible)} state(s) the stylesheet never drew "
        f"({shown}{more}). {one['cls']!r} computes color {one['color']}, weight {one['weight']}, "
        f"style {one['style']} and decoration {one['dec']} -- every one of them the same as the text "
        f"it is sitting in, and it carries no border or background of its own (border-left "
        f"{one['bw']}, background {one['bg']}). The class is in the markup and nothing in app.css "
        "turns it into something a parent can see, which is what #199 measured: `.state-held` was "
        "red and bold and every other state word on the page was ordinary prose, so \"no desktop "
        "session\" and \"locked out\" read as the same kind of sentence. "
        f"{len(rows) - len(visible)} element(s) carried a state class the browser was not showing "
        "and were skipped."
    )


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


def session_census(rows):
    """How many of `rows` are the account's own session rows.

    Both POST pages draw the census as a `table.rows` table whose rows carry a `Where` cell, and
    `testlock` draws a second `table.rows` table above it — one row per configured backend, labelled
    `Can lock` and `Detail`. Counting `tbody tr` over the whole page therefore counts two different
    things on the two pages, which is the mistake this function exists to make impossible: the census
    is the rows with a `Where`, on both pages, whatever else the page has grown beside them.
    """
    return sum(1 for r in rows if "Where" in [str(x) for x in (r.get("result") or [])])


def require_declared_sessions(rows, declared, what):
    """The page shows exactly the number of lockable sessions the fixture declared (#213).

    The sibling of `pages.py`'s `require_declared_rows`, one layer down: that one pins a row count to
    a seeded file, this one pins a row count to a census the machine was asked for before the browser
    started. Both exist because the alternative is a frame whose shape belongs to whatever the
    capturing machine happened to be, and both are checks of the *page* rather than of the source
    they were declared from — the reader of the artifact is counting rows on paper.

    What this catches that `scrollWidth` never could: the census is what decides how tall these two
    pages are. `lock.png` was re-taken on 8 Nov on a machine holding two lockable sessions for the
    target — `tty` plus `x11`, both of them `isLockable` at `internal/platform/platform.go:241-258` —
    and the artifact changed shape by 394 px while every guard in this file stayed green, because
    nothing anywhere said how many rows the frame was supposed to have. The README that describes
    these two frames still quotes the 1-session transcript from 27 Oct.

    A refusal here is not a broken page. It is a different page: the same template over another
    machine's logins. Re-take it deliberately, on a machine whose census is the one the artifact is
    meant to show, and let the manifest say which that was.
    """
    shown = session_census(rows)
    if shown != declared:
        raise SystemExit(
            f"{what}: the page renders {shown} lockable session row(s) and the fixture declared "
            f"{declared}. The census decides this artifact's height, so a frame taken over a "
            f"different census is a different page — re-run the setup script on a machine whose "
            f"census is the one you mean to publish, or declare PLOCK_CAPTURE_SESSIONS={shown} if "
            f"this one is right and the fixture is out of date (#213).")
    return shown


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
        # The trail as the fixture left it, taken inside _start before the browser signs in. Empty
        # here so a caller that asks before _start ran gets an honest zero rather than an
        # AttributeError in the middle of a refusal message.
        self.trail_before = []
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
        # The daemon's PATH, when the fixture has an opinion about it.
        #
        # This exists because the dashboard's Sessions cell is not a property of the config. `GET /`
        # asks the platform layer, the command backend delegates listing to logind
        # (internal/platform/command.go:133), and `loginctl` is resolved through PATH
        # (internal/platform/exec.go:74-79 — bare name, `exec.LookPath`, no absolute fallback). So
        # the daemon asked *this machine*, and the committed dashboard.png reported `session list
        # unavailable` while three fresh runs on the same box reported `no desktop session` — two
        # honest captures of one fixture, two different pages, both of them current by every check
        # manifest.py has (#207 cause 6). A fixture that names bin/ puts its own census first; a
        # fixture that names nothing keeps asking the machine, which is what the two POST fixtures
        # want, because their subject is a real lock on a real session.
        #
        # Prepend, not replace: the daemon still has to find the things nobody stubbed. Absolute
        # only — `exec.LookPath` refuses a relative PATH entry (`exec.go:67-72`), so a relative name
        # here would mean `loginctl: not found in PATH` and a page saying `session list unavailable`
        # for a reason that looks like the product's.
        daemon_env = None
        prepend = self.env.get("PLOCK_CAPTURE_PATH_PREPEND")
        zone = self.env.get("PLOCK_CAPTURE_TZ")
        if prepend or zone:
            daemon_env = dict(os.environ)
        if prepend:
            if not op.isabs(prepend):
                raise SystemExit(
                    f"{self.dir}/fixture.env declares PLOCK_CAPTURE_PATH_PREPEND={prepend!r}, which "
                    "is not an absolute path; exec.LookPath refuses a relative PATH entry, so the "
                    "daemon would find no loginctl at all and the page would say `session list "
                    "unavailable` for a reason nobody chose")
            if not op.isdir(prepend):
                raise SystemExit(f"PLOCK_CAPTURE_PATH_PREPEND={prepend!r} is not a directory, so "
                                 "nothing could be prepended to the daemon's PATH")
            daemon_env["PATH"] = prepend + os.pathsep + os.environ.get("PATH", "")
            print(f"census: the daemon's PATH starts at {prepend}", flush=True)
        if zone:
            # The reader's zone. It was load-bearing for the When column: the template asked for
            # `MST`, and Go hands a parsed instant the local zone's abbreviation when its offset
            # matches (measured: one trail, three TZ values, `EDT` / `-0400` / `-0400`). #210 changed
            # the column to the stored offset, so nothing on a page reads the host's zone any more:
            # every dashboard clock comes from the profile's own declared zone and `wallClock`
            # (`internal/dashboard/banner.go`) renders the instant in the location it was computed
            # in. It stays set so a frame cannot inherit a zone by accident -- the canary for a
            # host-dependent clock coming back, not the cause of one. Set on the daemon, not the
            # browser, because the daemon is what renders the page.
            #
            # Not validated here, and now nothing downstream would notice either: Go falls back to
            # UTC for a zone it cannot load, and a column printing the value's own offset renders the
            # same `-04:00` under the fallback as under a zone that loaded. The declared state used to
            # be where a bad zone surfaced -- the `MST` layout printed `-0400` against an `EDT` needle
            # -- and that trip wire left with #210.
            daemon_env["TZ"] = zone
            print(f"reader zone: TZ={zone} for the daemon, not {os.environ.get('TZ') or 'the host default'}",
                  flush=True)

        self.server = subprocess.Popen(
            [self.bin, "serve", "--config", op.join(self.dir, "config.json"),
             "--state-dir", op.join(self.dir, "state"),
             "--audit-path", op.join(self.dir, "audit.jsonl")],
            stdout=open(op.join(self.dir, "serve.log"), "wb"), stderr=subprocess.STDOUT,
            env=daemon_env)
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

            # Where this run's own rows begin. Read after the daemon is confirmed and before the
            # browser is authenticated, because the sign-in is the first thing in this run that can
            # write to the trail; anything appended after this mark belongs to this run.
            self.trail_before = audit_lines(op.join(self.dir, "audit.jsonl"))

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

            # The trail's clock, owned before a single pixel is measured. Only a fixture that
            # declares its row count asks for this: the two POST fixtures photograph a trail that is
            # *supposed* to grow, because the row their page shows is the one the click wrote, and
            # pinning it to a declared instant would erase the finding.
            if self.env.get("PLOCK_CAPTURE_AUDIT_ROWS"):
                pin_appended_rows(op.join(self.dir, "audit.jsonl"), self.trail_before,
                                  self.declared_stamp(), int(self.declared_rows()),
                                  f"{op.basename(self.dir)} fixture")

            # The page's own scripts, off or on, from here to the last photograph. The login happened
            # above, so the session cookie is already in the jar whatever this decides.
            #
            # Two reasons the default is `off`, and the second is the one that decides it. The first:
            # static/app.js:76-104 selects every `time[data-unix]` and rewrites its textContent from
            # Date.now(), on load and then every second, so the "until" in a frame taken with scripts
            # running is a live countdown reading `58m 12s` rather than the `EndsAtClock` the server
            # put there — unrepeatable pixels, #207's cause 4, and the reason pinning the Go clock
            # alone would not have pinned the image. The second: this folder's own criterion is
            # "usable at 360 px with JavaScript off", and a photograph taken with the scripts on has
            # never tested that sentence. `manifest.py check` refuses to commit a frame recorded
            # `scripts: "on"` for that reason.
            #
            # Off is the default and not the rule, because `on` is the only way to measure what this
            # switch is worth: the page's scripts have to be switchable to show which pages they can
            # move. `dashboard.png` proves the switch has power, and a page with no `time[data-unix]`
            # in it proves the pixels are insensitive there — see the `scripts` section of README.md
            # for both measurements.
            #
            # Emulation.setScriptExecutionDisabled rather than a Chrome launch flag, and that choice
            # is measured, not stylistic. `--disable-javascript` is still *accepted* by this machine's
            # Chrome (156.0.8078.79 starts it without a word of complaint) and does nothing: a page
            # whose script sets document.title to "JS RAN" came back with that title at the flag and
            # with `orig` under the CDP call. A flag that is accepted and ignored is the worst shape
            # of refusal to build a criterion on, because the capture looks configured — and it is
            # also what the declaration in `fixture.env` was until this line read it. The driver's own
            # Runtime.evaluate calls are unaffected — measured, `1+1` still came back 2 with the
            # page's scripts disabled — because this switches off the page's world, not this client's.
            scripts_off = scripts_disabled(self.env, f"{op.basename(self.dir)} fixture")
            self.session.call("Emulation.setScriptExecutionDisabled", value=scripts_off)
            # Said out loud, in the transcript next to the census line, because a frame's conditions
            # are the thing a reader of the artifact cannot recover from the pixels afterwards.
            print(f"page scripts: {'off' if scripts_off else 'ON'} for this frame "
                  f"(PLOCK_CAPTURE_SCRIPTS in {op.basename(self.dir)}/fixture.env)", flush=True)
            # #213, in the same breath: the census is the condition these two frames' height comes
            # from, and the three things that decided it are named with it. Not the session bus and
            # not the timezone — `fixture.sh`'s note on `capture_declare_census` carries the
            # measurements that show why it is the system bus and the loginctl that matter here.
            if self.env.get("PLOCK_CAPTURE_SESSIONS"):
                uid = self.env.get("PLOCK_CAPTURE_CENSUS_UID")
                through = self.env.get("PLOCK_CAPTURE_CENSUS_LOGINCTL")
                if uid and through:
                    line = (f"session census: {self.env['PLOCK_CAPTURE_SESSIONS']} lockable "
                            f"session(s), as uid {uid} through {through}")
                    if self.env.get("PLOCK_CAPTURE_CENSUS_SYSTEM_BUS"):
                        line += (f" on DBUS_SYSTEM_BUS_ADDRESS="
                                 f"{self.env['PLOCK_CAPTURE_CENSUS_SYSTEM_BUS']}")
                else:
                    # No loginctl and no uid with it: nobody asked the machine, so the number came
                    # from a person. Said as plainly as the measured case, because the two support
                    # different sentences about the frame (#213).
                    line = (f"session census: {self.env['PLOCK_CAPTURE_SESSIONS']} lockable "
                            "session(s), declared by the operator rather than taken from the machine")
                print(f"{line} (PLOCK_CAPTURE_SESSIONS in "
                      f"{op.basename(self.dir)}/fixture.env)", flush=True)
        except BaseException:
            self.close()
            raise

    def declared_stamp(self):
        """The instant this fixture says the clock reads, as `<date>T<time><offset>`.

        Validated rather than pasted. The value goes verbatim into the `at` field of a trail row and
        comes back out as text in the When column, so a typo here — `2026-13-40`, `25:99`, an offset
        written `-4:00` where the ISO form wants `-04:00` — would otherwise surface as an unreadable
        row on a photographed page, or as a traceback from `int()` further down. Both are worse
        answers than this one, and the sentence names the file to edit.
        """
        stamp = self.env.get("PLOCK_CAPTURE_STAMP", "")
        zone = self.env.get("PLOCK_CAPTURE_ZONE", "")
        for key, value in (("PLOCK_CAPTURE_STAMP", stamp), ("PLOCK_CAPTURE_ZONE", zone)):
            if not value:
                raise SystemExit(
                    f"{op.basename(self.dir)} declares PLOCK_CAPTURE_AUDIT_ROWS but not {key}, so "
                    "there is no declared instant to stamp the sign-in's row with. Name both from "
                    "the same fixture, or name neither and let the trail keep the clock it wrote.")
        whole = stamp + zone
        try:
            datetime.datetime.fromisoformat(whole)
        except ValueError as exc:
            raise SystemExit(
                f"{op.basename(self.dir)} declares PLOCK_CAPTURE_STAMP={stamp!r} with "
                f"PLOCK_CAPTURE_ZONE={zone!r}, which is not an instant ({exc}). The concatenation is "
                "written straight into a trail row's `at` field and printed in the When column.")
        return whole

    def declared_rows(self):
        """The row count the fixture declares, or a refusal that says what it expected."""
        raw = self.env.get("PLOCK_CAPTURE_AUDIT_ROWS", "")
        if not raw.isdigit():
            raise SystemExit(
                f"{op.basename(self.dir)} declares PLOCK_CAPTURE_AUDIT_ROWS={raw!r}, which is not a "
                "count. The fixture's declared number is what the audit page's row count is compared "
                "against, so it has to be a plain integer.")
        return raw

    def declared_sessions(self):
        """The session census this fixture declares, or a refusal that says where the number comes from.

        #213, and the reason it is a refusal rather than a default: these two pages render one row per
        lockable session of the target account, so the count is the shape of the artifact — its height
        with it. `0` read out of an absent key would be a fixture confidently photographing the empty
        case, and a default of "whatever the page shows" is the check not happening, which is the state
        the two frames were in until now: the committed `lock.png` is a 2-session page and the README
        that describes it still quotes the 1-session transcript.

        There is one way for this to be missing legitimately: a `runs/<fixture>/fixture.env` written by
        a setup script older than the census. The message says so, because the alternative reading —
        that this file is where the number belongs — would put a second copy of a machine's logins in
        the driver.
        """
        raw = self.env.get("PLOCK_CAPTURE_SESSIONS", "")
        if not raw.isdigit():
            raise SystemExit(
                f"{op.basename(self.dir)} declares PLOCK_CAPTURE_SESSIONS={raw!r}, which is not a "
                "count. The census decides how many rows this page has and therefore how tall the "
                "artifact is, so it is declared before the browser starts and checked against the "
                "page. `capture_declare_census` in fixture.sh writes it into fixture.env from the "
                "machine's own census: re-run the setup script, or name the count yourself with "
                "PLOCK_CAPTURE_SESSIONS=<n> in front of it.")
        return int(raw)

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


def capture_out(default_name, fixture_flag=False):
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
    # `--fixture` belongs to the driver and not to this helper -- but argparse owns the whole command
    # line, so an undeclared flag dies here as "unrecognized arguments" before the driver gets to
    # refuse it for a better reason. It is therefore declared only by drivers that pass
    # fixture_flag=True and left undeclared everywhere else, so `lock.py --fixture x` stays an error
    # rather than an ignored word. The VALUE is read once, by the driver's own fixture_mode(); this
    # exists only so argparse does not reject an argument it has no use for.
    argv = sys.argv[1:]
    if fixture_flag:
        ap.add_argument("--fixture", metavar="NAME", default="pages",
                        help="which capture fixture to photograph; see the driver's PAGES table")
    a = ap.parse_args(argv)
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


# The one row a signed-in capture appends to the trail, and the emitter that writes it:
# `s.audit("auth.success", sess.Username)` at internal/web/handlers.go:150. Named here because two
# separate checks need the same name, and a third would be the fixture's.
SIGNIN_KIND = "auth.success"

# The `at` field, as the Go writer emits it (`"at":"..."`) and as a fixture's own seeder emits it
# (`"at": "..."`). Replacing the value and nothing else is deliberate: re-encoding the row through
# json.dumps would reformat the whole line, and a fixture's claim is about *when*, not about
# whether the trail is pretty.
_AT_FIELD = re.compile(r'("at"\s*:\s*")[^"]*(")')


def pin_appended_rows(path, seeded, stamp, declared_rows, what):
    """Make the rows this run appended carry the fixture's declared instant, or refuse.

    The fixture owns its clock (issue #207, cause 1): the trail is seeded at the instant the setup
    script declares instead of at `datetime.now()`, so the When column no longer depends on when the
    photograph was taken. One row is outside the seeder's reach — the sign-in has to happen over real
    HTTP, through the handler, because the driver authenticates the way a person does — and
    `audit.Writer` stamps it with `time.Now` (`cmd/plock/serve.go:224` sets no `At`, and
    `web.Options` has no clock to be given). This is the other half of owning the stamp: the row the
    product wrote is identified, checked, and re-stamped to the same declared instant.

    Checked, then rewritten, in that order, because the two refusals are the point:

      * the count has to be the fixture's declared number, so the run counter cannot come back. The
        committed audit.png had reached 360x15004 while a fixture built from nothing rendered 2936
        px — the height was a tally of how many times anyone had run a driver on their own machine
        (#207 cause 2). A fixture that declares 8 rows and finds 9 after one sign-in is a fixture
        whose sign-in writes more than it used to, and that is worth a refusal, not a shrug.
      * every appended row has to be the sign-in's own kind. This is the one that catches the
        enforcement loop having written to the trail during the capture — which is exactly what
        happens if a fixture hands a *held, enabled* account a session: the next tick locks it and
        appends a `lock.*` row to the page being photographed. Re-stamping a row like that would
        have hidden a real event behind a plausible timestamp, which is worse than a tall image.

    The seeded rows are written back unchanged and the file is replaced in place. In place matters:
    the daemon holds this path open with O_APPEND, and O_APPEND counts from the *current* end, so a
    truncate-and-write cannot be interleaved with a concurrent append on the same inode, while
    unlink-and-recreate would leave the daemon appending to an orphaned file the page never shows.
    """
    try:
        with open(path) as fh:
            lines = [l for l in fh if l.strip()]
    except FileNotFoundError:
        lines = []
    if len(lines) < len(seeded):
        raise SystemExit(
            f"{what}: the trail holds {len(lines)} rows and the fixture seeded {len(seeded)}, so "
            "the file was replaced underneath this run. Nothing was re-stamped: a trail that can "
            "shrink between the fixture and the photograph is not evidence about either.")
    appended = lines[len(seeded):]
    kinds = sorted({json.loads(l).get("kind", "") for l in appended})
    if len(lines) != declared_rows:
        raise SystemExit(
            f"{what}: this fixture declares {declared_rows} rows on the page and the trail holds "
            f"{len(lines)} ({kinds or 'nothing'} appended since it was seeded). The sign-in writes "
            f"exactly one {SIGNIN_KIND!r} row and nothing else may write during a capture — if "
            "either of those has changed, the fixture's declared number and this check both have to "
            "be updated on purpose, because the artifact's height is that number (#207).")
    wrong = sorted(set(kinds) - {SIGNIN_KIND})
    if wrong:
        raise SystemExit(
            f"{what}: the run appended {wrong}; the only row a capture may add to the trail it is "
            f"about to photograph is {SIGNIN_KIND!r}. Re-stamping that would put the fixture's "
            "clock on an event the fixture did not choose — which is the same lie as a tall image, "
            "with better manners.")
    if not appended:
        return 0
    pinned = [_AT_FIELD.sub(lambda m: m.group(1) + stamp + m.group(2), l) for l in appended]
    for original, rewritten in zip(appended, pinned):
        if rewritten == original:
            raise SystemExit(
                f"{what}: asked to stamp {stamp!r} on a row that does not change when the `at` "
                f"field is replaced: {original[:160]} — the row has no `at` field this function "
                "recognises, so the When column is not what this fixture thinks it is.")
    with open(path, "w") as fh:
        fh.writelines(l if l.endswith("\n") else l + "\n" for l in seeded + pinned)
    print(f"trail: {len(pinned)} appended row(s) re-stamped to {stamp}; "
          f"{len(seeded) + len(pinned)} rows, the fixture's declared count", flush=True)
    return len(pinned)


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

    The same argument runs the other way about its own output. All three sentences are spoken with
    `operator_print`, because every call site is inside a `except` handler in one of the three
    drivers, and a headline that raises on the way out is a refusal the operator never hears AND a
    traceback that exits 1: #317 measures the sweep losing 11 of its 12 measurements to one dead
    `print`, and both single-frame drivers' fixture arms printing nothing at all.
    """
    operator_print(f"REFUSED: {exc}")
    if path and op.exists(path):
        st = os.stat(path)
        when = datetime.datetime.fromtimestamp(st.st_mtime, datetime.timezone.utc)
        operator_print(f"  a previous {op.basename(path)} is still at {path} -- mtime "
                       f"{when.strftime('%Y-%m-%dT%H:%M:%SZ')}, {st.st_size} bytes -- and it is NOT "
                       "from this run. Do not quote it as current.")
    else:
        operator_print("  no file was written")

