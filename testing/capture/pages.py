#!/usr/bin/env python3
"""Measure and photograph the five GET-rendered admin pages at 360 px.

The 360 px acceptance criterion is a measurement, not a reading of the stylesheet, so this
drives a real browser: it starts the daemon on a throwaway fixture, signs in, then asks Chrome
at a 360 px viewport for documentElement.scrollWidth and a screenshot of each page. A page that
fits reports scrollWidth == 360; anything larger is horizontal overflow, which is what the
criterion refuses.

These five are the easy pages: each has a URL, so a browser can be sent straight to it. The two
result pages cannot be reached this way and have their own drivers (lock.py, testlock.py), which
is where the click-the-shipped-button refusals live.

Each page is refused on its own, before its own PNG is written, so a run that stops on the audit
page leaves the three earlier pages freshly captured and says plainly which one it would not
take.

    ./pages-setup.sh && ./pages.py [--out DIR] [--overwrite]
    ./pages.py --self-test        # the refusal contract only: no browser, no fixture, no writes

Run it from anywhere; every path is resolved from this file's location. Output goes to
testing/capture/runs/pages/ unless --out names a directory inside the repository, so re-capturing
the committed artifacts is an explicit `--out docs/screenshots/360`.
"""
import contextlib
import importlib
import io
import os.path as op
import sys

sys.path.insert(0, op.dirname(op.abspath(__file__)))
import cdp  # noqa: E402

# How hard the Detail column has to be pushed before a width measurement means anything. 60 sits
# far below the seeded row (251 characters: the captured loginctl listing, named below) and far
# above any sentence a person writes, so a fixture reworded back into prose trips this rather than
# quietly stopping the check.
UNBREAKABLE_MIN = 60

# The seeded row's Event kind, exactly as pages-setup.sh writes it.
STRESS_KIND = "platform.error"

# The longest run without a space anywhere in a `table.rows` cell, as the browser sees it.
# textContent, not innerText: innerText is the RENDERED text, and a cell that wraps correctly is
# exactly the thing that could hand back a token broken at the wrap point -- the refusal would then
# fire against the fix.
#
# Spliced into the same measure() call as the cell and form census, which is what that key exists
# for. It used to be its own CDP round-trip, and the consequence was small but real: the
# transcript's `hardest=` came from one read while the guard compared another, so the number the
# transcript certifies was not the number the check used.
LONGEST_JS = ("longest:Math.max(0, ...[...document.querySelectorAll('table.rows tbody td')]"
              ".flatMap(td => (td.textContent.match(/\\S+/g) || [])).map(s => s.length))")

# Is the row this fixture seeded actually on the page? The floor below is a page-wide realism
# floor, and the two are not the same claim: on the machine #140 came from, the daemon's own failure
# row supplied 123 characters and cleared 60 by itself, so no floor can attribute the pressure to
# the row the authored. This one can, and it refuses when that row is not among the rendered cells.
STRESS_JS = ("stress:[...document.querySelectorAll('table.rows tbody td')"
             f"].some(td => td.textContent.trim() === {STRESS_KIND!r})")


def require_stress_row(got, what):
    """The #140 row has to be on the page, not merely possible.

    `require_unbreakable` asks how hard the page was pushed; this asks who pushed it. Without it
    the fixture can lose its stress row -- a reworded setup script, a kind renamed in one place, a
    page that stops rendering the Event column -- and the width check keeps reporting a comfortable
    number, because some other row happens to be long today. That is the same failure #140 opened
    with, arriving from the opposite direction: the measurement stayed green while the thing it was
    measuring left the building.
    """
    if not got["stress"]:
        raise SystemExit(
            f"{what}: no rendered cell is the seeded {STRESS_KIND!r} row, so whatever width "
            "pressure this page has came from the machine and not from the fixture. The audit "
            "capture would be evidence about this box, not about the stylesheet (#140).")


def require_unbreakable(got, what):
    """#140's rule: the audit page is only evidence about width if it was handed something that
    cannot be wrapped.

    The refusal that opened the issue was real -- `893 px wide at a 360 px viewport`, printed twice,
    on two trees -- and it came from a row nobody authored. The daemon had failed to enumerate this
    machine's sessions and had written the loginctl JSON it could not parse straight into the Detail
    column. That is why the check exists: the stress was emergent, so it left without any edit. On
    today's machine the same fixture, unmodified, produced a 123-character token, the page fitted,
    and the driver reported `5/5 pages fit at 360 px` having asked for nothing. #146 replaced the
    invented payload with the captured listing and the page stopped fitting: 914 px.

    What this check is: a floor on the fixture's realism. It refuses a fixture that has quietly
    become prose, which is the failure it can actually prevent.

    What it is not: a statement about overflow. The sentence that used to sit here said a long token
    is not by itself an overflow at this width, on the strength of 320 characters of unbreakable hex
    supposedly fitting at 360 px. That measurement does not reproduce. In the audit Detail cell at a
    360 px viewport, 320 characters of hex are 2848 px of content in a 312 px box. Giving a cell the
    page's width sizes the box; it says nothing about the text inside it.

    What sets the width is the longest run between the browser's break opportunities, and those are
    not only spaces: a `[{…},{…}]` blob breaks between objects, a single object does not break at all
    (measured: 117 characters, 808 px). The rule that fixes it is `overflow-wrap` on `table.rows td`
    in `app.css` -- `anywhere`, with `break-word` under it, because `break-word` fixes the narrow
    layout and not the wide one. And the pin on that rule is the width refusal above, not a grep:
    delete the rule and this page overflows, so its photograph is not taken.

    It takes the measured object rather than the session, and lives here rather than in cdp.py, for
    the same reason its sibling checks do: it has one caller, it names the audit page, and the
    shared file's contract is that it holds no page-specific assertions.
    """
    longest = got.get("longest")
    if longest is None:
        # `stress` and `has_rows` are independent arguments of Page(), but the measurement this
        # reads is spliced on `has_rows`. A page declared `stress=True, has_rows=False` therefore
        # used to reach `got["longest"]` and raise KeyError -- a traceback, which this file's own
        # rule in main() says is not an answer. main() now refuses that combination before the
        # browser starts; this is the second net, because the check must not depend on being
        # called only from one place.
        raise SystemExit(
            f"{what}: asked how unbreakable the table is when no longest-token measurement was "
            "taken for this page. The token census is spliced only for pages declared with "
            "has_rows, so a page carrying the #140 stress row has to be one of them (#140).")
    if longest < UNBREAKABLE_MIN:
        raise SystemExit(
            f"{what}: the hardest token anywhere in the table is {longest} characters, under the "
            f"{UNBREAKABLE_MIN} this check needs to call a width measurement evidence. Nothing on "
            "the page can overflow, so 360 px here proves nothing about a detail that quotes a "
            "path or a JSON blob (#140).")


# Each entry is a URL, the committed artifact it becomes, whether the fixture puts a table on the
# page, and whether it carries the #140 stress row.
#
# `has_rows` decides which refusal proves the page was really rendered, and it exists because the
# settings page has no table. Dropping require_records for it would have been the lazy edit, and it
# is the wrong one: a /settings that rendered nothing but its navigation also fits at 360 px, and
# that is exactly the photograph nobody would notice is not evidence. A table page is refused for
# having no rows; a form page is refused for having nothing to submit.
#
# `stress` is a trait on the row rather than an `if page == "/audit"` inside shoot(), because a rule
# that hangs off a URL string stops running the day somebody edits the string -- and stops running
# quietly, which is the exact failure #140 is about. Here the rule and the page that carries it are
# one line of this table, and main() refuses to run at all if no line carries it.
class Page:
    def __init__(self, url, artifact, has_rows, stress=False):
        self.url, self.artifact = url, artifact
        self.has_rows, self.stress = has_rows, stress


PAGES = [
    Page("/", "dashboard", True),
    Page("/users", "users", True),
    Page("/profiles", "profiles", True),
    Page("/audit", "audit", True, stress=True),
    Page("/settings", "settings", False),
]

# The one flag the GET pages need and the result pages do not: these four are the pages whose
# whole problem is a table wider than the viewport, and a scrollbar that is hidden changes what
# the compositor has to fit. The committed artifacts were taken with it off, so it stays off --
# a capture that differs from the artifact by a rendering flag is not evidence about the
# artifact.
CHROME_FLAGS = ("--hide-scrollbars=false",)


def shoot(cap, spec, name):
    page, session, base = spec.url, cap.session, cap.base
    session.goto(base + page)
    here = session.where()
    if here != page:
        raise SystemExit(f"{page} answered {here} instead; nothing was measured")
    session.settle()
    # One read per page. `extra` is what measure() takes for exactly this: the census keys a page
    # needs are spliced into the same object, so the number printed in the transcript, the number
    # the width refusal quotes and the number the stress guards compare are all from one render.
    extra = cdp.FORMS_JS
    if spec.has_rows:
        extra += "," + LONGEST_JS
    if spec.stress:
        extra += "," + STRESS_JS
    got = cdp.measure(session, extra)
    if spec.has_rows:
        cdp.require_records(got, page)
        cdp.require_labelled_cells(got, page)
    else:
        cdp.require_form(got, page)
        cdp.require_labelled_cells(got, page)
    # Before the write, not after. A photograph of an overflowing page is the artifact the
    # criterion refuses, and a driver that writes it and then exits 1 has left the wrong evidence
    # on disk under the right filename -- with --out pointing at the docs tree it has overwritten
    # the committed artifact and only an exit code says so.
    # Measured BEFORE the width refusal so the refusal can name it. The 893 px refusal that opened
    # #140 said only how wide the page was, and by the time anyone read it the row that may have
    # caused it was gone -- the number that would have answered is one CDP call away at the moment of
    # refusal and unrecoverable after it.
    longest = got.get("longest")
    hard = "" if longest is None else f" hardest={longest}ch"
    if got["scroll"] > cdp.WIDTH:
        cause = ("" if longest is None else
                 f", and its hardest unbreakable token is {longest} characters")
        raise SystemExit(f"{page} is {got['scroll']} px wide at a {cdp.WIDTH} px viewport{cause}; "
                         "the criterion refuses this page, so its photograph is not taken")
    # After the width refusal, before the write. A page that overflows should report the overflow,
    # which is the alarming fact; a page that fits is then asked whether fitting meant anything.
    # #140's whole lesson is that the audit page can fit at 360 px having been asked for nothing,
    # because the row that broke it came from the daemon failing on this machine rather than from
    # the fixture.
    if spec.stress:
        require_stress_row(got, page)
        require_unbreakable(got, page)
    cap.require_running()
    session.write_png(name)
    proof = f"tables={got['tables']} cells={got['cells']}" if spec.has_rows else \
        f"forms={got['forms']} fields={got['fields']}"
    print(f"  {page:<10} scrollWidth={got['scroll']:>4} innerWidth={got['inner']:>4} "
          f"height={got['height']:>5} {proof}{hard} "
          f"FITS  {name}", flush=True)


def main(shoot_fn=None):
    # Resolved at call time rather than as a default argument. `def main(shoot_fn=shoot)` binds
    # the function object when this module is imported, so a caller that patches `pages.shoot`
    # would be ignored -- the exact shape that once made a hermeticity fix here a silent no-op.
    if shoot_fn is None:
        shoot_fn = shoot
    # The trait has to be carried by somebody. A table where every `stress` flag has been dropped
    # measures five pages that could not have overflowed and reports them as fitting, which is the
    # silence #140 was filed for; refusing here means the omission is a refusal rather than a
    # transcript.
    if not any(p.stress for p in PAGES):
        raise SystemExit(
            "no page in PAGES carries the #140 stress row, so no width measurement this driver "
            "makes can be evidence about overflow")
    # The other way the trait can be inert: carried by a page whose table census is never spliced,
    # so both stress checks would be reading keys the measurement never took. Refusing the
    # declaration here, before Chrome starts, beats a KeyError from inside one page's handler --
    # and beats the quieter version, where a future edit flips has_rows and the audit page starts
    # reporting a comfortable width again.
    for p in PAGES:
        if p.stress and not p.has_rows:
            raise SystemExit(
                f"{p.url} carries the #140 stress row but is declared as a page with no table, so "
                "the checks that attribute width pressure to the fixture have nothing to read. "
                "Declare it with has_rows=True.")
    out = cdp.capture_out("pages")
    refused_pages = []
    with cdp.Capture("pages", CHROME_FLAGS) as cap:
        for spec in PAGES:
            name = op.join(out, spec.artifact + ".png")
            try:
                shoot_fn(cap, spec, name)
            except SystemExit as exc:
                # `report_refusal` and not a raising helper: this handler is the only thing
                # standing between one bad page and the four that were measured fine, and until
                # this head the helper raised from inside it, so both `append` lines below were
                # unreachable and the run died on the first refusal while looking like a refusal.
                cdp.report_refusal(name, exc)
                refused_pages.append(spec.url)
            except Exception as exc:  # a traceback is not an answer; a refusal is
                cdp.report_refusal(name, f"{type(exc).__name__}: {exc}")
                refused_pages.append(spec.url)
        if refused_pages:
            # The count is the exit status, so it has to be the true one. This line used to print
            # `4/4 pages fit` and return 0 whatever had happened, because a refusal was caught
            # per page and the loop simply carried on -- a run that photographed three pages and
            # refused the fourth reported complete success, which is the failure this whole
            # package exists to make impossible. The opposite failure then replaced it: the
            # refusal helper raised, so nothing was ever counted and the sentence below was
            # decoration. `--self-test` now pins both directions -- every page measured even when
            # all five refuse, and exit 1 whenever any one of them does.
            cdp.discard_stage()
            print(f"\n{len(PAGES) - len(refused_pages)}/{len(PAGES)} pages fit at "
                  f"{cdp.WIDTH} px; refused: {', '.join(refused_pages)}. Nothing was written to "
                  f"{out}.", file=sys.stderr, flush=True)
            return 1
        cap.session.publish()
    print(f"\n{len(PAGES)}/{len(PAGES)} pages fit at {cdp.WIDTH} px -> {out}", flush=True)
    return 0


# --------------------------------------------------------------------------------------
# The self-test: the refusal contract, with no browser and no fixture.
#
# Why this exists at all. Nothing else in the repository can see these files: the mutants target
# `.go` and `.tmpl` only, nothing imports `pages.py`, and shellcheck does not read Python. So a
# refusal helper that raised from inside the caller's own `except` handler -- which made the loop
# stop after 1 page of 5 while still exiting 1, and made `discard_stage()` dead in all three
# drivers -- survived every gate this repository has. A `--self-test` is the only shape of pin
# available to a file no compiler or test binary ever opens, and `make verify` runs it the way it
# runs the two mutation self-tests.
#
# It starts no Chrome, spawns no daemon and writes no file: every rule drives the real `main()`
# through stubs for the three things that need a machine (`Capture`, `capture_out`,
# `discard_stage`) plus one for `shoot`. What they do not stub out is the logic under test -- the
# loop, the handlers, the counting and the exit code are all the real ones.
# --------------------------------------------------------------------------------------

EXPECTED_RULES = 11


class _FakeSession:
    def __init__(self, log):
        self.log = log

    def publish(self):
        self.log.append("publish")


class _FakeCap:
    """Stands in for cdp.Capture: a context manager whose session records being published."""

    def __init__(self, log):
        self.log = log
        self.session = _FakeSession(log)
        self.base = "http://127.0.0.1:0"

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        return False

    def require_running(self):
        pass


def _raiser(exc):
    def boom(*args):
        raise exc
    return boom


@contextlib.contextmanager
def _stubbed(log):
    """The three things that need a machine, replaced for the duration of one rule.

    `capture_out` matters most and not for convenience: it parses sys.argv, so a rule that lets
    main() reach it gets a refusal for the `--self-test` flag sitting in argv. Two of the rules
    below were satisfied by exactly that and passed while the refusal they guard was deleted --
    measured. Stubbing it means the only refusal a rule can see is the one the rule is about.
    """
    saved = (cdp.Capture, cdp.capture_out, cdp.discard_stage)
    cdp.Capture = lambda *a, **k: _FakeCap(log)
    cdp.capture_out = lambda name: "/tmp/opencode/selftest-pages"
    cdp.discard_stage = lambda: log.append("discard")
    try:
        yield
    finally:
        cdp.Capture, cdp.capture_out, cdp.discard_stage = saved


def _run_main(refuse=(), crash=None):
    """Call the real main() with the machine stubbed out; report what the driver actually did.

    Returns (rc, entered, stderr_text, published, discard_calls). `rc` is the string "raised"
    when main() let an exception escape instead of returning a code -- itself a finding, because
    an escaping exception and a refusal both exit 1 and only the return proves the driver
    finished its own bookkeeping.
    """
    log, entered = [], []
    urls = set(refuse)

    def stub(cap, spec, name):
        entered.append(spec.url)
        if spec.url in urls:
            raise SystemExit(f"{spec.url} is 9999 px wide at a {cdp.WIDTH} px viewport")
        if crash is not None:
            raise crash

    rc = "raised"
    err = io.StringIO()
    with _stubbed(log):
        try:
            with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(err):
                rc = main(stub)
        except SystemExit as exc:
            rc = f"raised:{exc.code}"
    return rc, entered, err.getvalue(), "publish" in log, log.count("discard")


def _run_single_shot(mod, exc):
    """Drive lock.py / testlock.py main() with its capture stubbed to refuse."""
    log = []
    saved = (mod.capture, cdp.capture_out, cdp.discard_stage)
    mod.capture = _raiser(exc)
    cdp.capture_out = lambda name: "/tmp/opencode/selftest-single"
    cdp.discard_stage = lambda: log.append("discard")
    rc = "raised"
    try:
        with contextlib.redirect_stdout(io.StringIO()), contextlib.redirect_stderr(io.StringIO()):
            rc = mod.main()
    except SystemExit as se:
        rc = f"raised:{se.code}"
    finally:
        mod.capture, cdp.capture_out, cdp.discard_stage = saved
    return rc, log.count("discard")


def self_test():
    """Every rule below is a claim about what a refusal does. Eleven of them were false when
    written, and the file still exited 1, which is exactly why a count and an exit code are not
    evidence that a driver did its job."""
    ran, bad = [], []

    def rule(why, body, ran_to=None, bad_to=None):
        problems = []
        try:
            body(problems)
        except Exception as exc:  # a rule that throws has not passed
            problems.append(f"{type(exc).__name__}: {exc}")
        (ran_to if ran_to is not None else ran).append((why, len(problems)))
        (bad_to if bad_to is not None else bad).extend(problems)

    urls = [p.url for p in PAGES]

    def all_refuse(problems):
        rc, entered, err, published, discarded = _run_main(refuse=urls)
        if len(entered) != len(urls):
            problems.append(f"a refusal on every page measured {len(entered)} of {len(urls)} "
                            f"({entered}); the loop has to finish for the count it prints to be "
                            "true")
        if rc != 1:
            problems.append(f"every page refused -> main() returned {rc!r}, want 1")
        if discarded != 1:
            problems.append(f"every page refused -> discard_stage() ran {discarded} times, want 1")
        if published:
            problems.append("every page refused -> the stage was published anyway")
        if f"0/{len(urls)}" not in err:
            problems.append(f"the summary does not report 0/{len(urls)}: {err.strip()[-160:]!r}")

    def one_refuses(problems):
        victim = urls[-1]
        rc, entered, err, published, discarded = _run_main(refuse=[victim])
        if entered != urls:
            problems.append(f"one refusal stopped the loop at {entered}, want all {urls}")
        if rc != 1:
            problems.append(f"one page refused -> main() returned {rc!r}, want 1")
        if victim not in err:
            problems.append(f"the summary does not name the refused page {victim!r}")
        if f"{len(urls) - 1}/{len(urls)}" not in err:
            problems.append(f"the summary does not report {len(urls) - 1}/{len(urls)}")
        if discarded != 1 or published:
            problems.append(f"one refusal -> discarded={discarded} published={published}, "
                            "want 1 and False")

    def none_refuses(problems):
        rc, entered, err, published, discarded = _run_main()
        if rc != 0:
            problems.append(f"a clean run -> main() returned {rc!r}, want 0")
        if not published:
            problems.append("a clean run did not publish the stage")
        if discarded:
            problems.append("a clean run discarded the stage")
        if entered != urls:
            problems.append(f"a clean run measured {entered}, want {urls}")

    def crash_is_a_refusal(problems):
        # A bug inside shoot() must not cost the other four pages either: `except Exception` is
        # the only thing standing between a TypeError on page 1 and a run that measured nothing.
        rc, entered, err, published, discarded = _run_main(crash=TypeError("a real bug"))
        if len(entered) != len(urls):
            problems.append(f"an exception on every page stopped the loop at {len(entered)}")
        if rc != 1 or discarded != 1:
            problems.append(f"an exception -> rc={rc!r} discarded={discarded}, want 1 and 1")
        if "TypeError" not in err:
            problems.append("the refusal does not name the exception type")

    def report_returns(problems):
        # The root cause of the whole class: this used to end in `raise SystemExit(1)`, so it died
        # at the first line of every handler that called it.
        try:
            with contextlib.redirect_stderr(io.StringIO()):
                cdp.report_refusal("/tmp/opencode/selftest-not-here.png", SystemExit("x"))
        except BaseException as exc:
            problems.append(f"report_refusal raised {type(exc).__name__}; it reports and returns, "
                            "or every statement after every call to it is dead code")

    def census_missing(problems):
        # The key is ABSENT, not None. That is the real case: LONGEST_JS is spliced on `has_rows`,
        # so a stress page declared without a table never gets a `longest` key at all. A fixture
        # carrying `longest: None` tests the wrong branch and let the `got["longest"]` indexing
        # survive a full probe battery -- measured, and the reason this line says what it says.
        try:
            require_unbreakable({"stress": True}, "/audit")
            problems.append("a measurement with no token census was accepted")
        except KeyError:
            problems.append("a missing token census raised KeyError, and this file's own rule is "
                            "that a traceback is not an answer")
        except SystemExit:
            pass

    def census_floor(problems):
        try:
            require_unbreakable({"stress": True, "longest": UNBREAKABLE_MIN - 1}, "/audit")
            problems.append("a token under the floor was accepted")
        except SystemExit:
            pass

    def row_absent(problems):
        try:
            require_stress_row({"stress": False}, "/audit")
            problems.append("a page with no seeded stress row was accepted")
        except SystemExit:
            pass

    def refusal_from_main(problems):
        """Run main() with the machine stubbed and a shoot that succeeds; return the refusal text.

        The shoot does nothing on purpose. If the declaration check under test has been deleted,
        main() measures its one page happily, publishes and returns 0 -- which is recorded as a
        problem here rather than being mistaken for a refusal.
        """
        log = []
        with _stubbed(log):
            try:
                rc = main(lambda *a: None)
            except SystemExit as exc:
                return str(exc)
            except BaseException as exc:
                problems.append(f"main() raised {type(exc).__name__} instead of refusing: {exc}")
                return ""
        problems.append(f"main() returned {rc!r} instead of refusing the declaration")
        return ""

    def stress_needs_table(problems):
        saved = list(PAGES)
        try:
            PAGES[:] = [Page("/audit", "audit", False, stress=True)]
            msg = refusal_from_main(problems)
            if msg and "has_rows" not in msg:
                problems.append(f"the refusal does not say how to fix it: {msg}")
        finally:
            PAGES[:] = saved

    def stress_must_be_carried(problems):
        saved = list(PAGES)
        try:
            PAGES[:] = [Page("/", "dashboard", True)]
            msg = refusal_from_main(problems)
            if msg and "stress row" not in msg:
                problems.append(f"the refusal does not name the missing trait: {msg}")
        finally:
            PAGES[:] = saved

    def single_shot(problems):
        # The same dead-statement class in the two single-page drivers: each called
        # discard_stage() on the line after a helper that raised.
        for mod_name in ("lock", "testlock"):
            mod = importlib.import_module(mod_name)
            for exc in (SystemExit("the page overflows"), RuntimeError("a real bug")):
                rc, discarded = _run_single_shot(mod, exc)
                kind = type(exc).__name__
                if rc != 1:
                    problems.append(f"{mod_name}: a {kind} refusal -> main() returned {rc!r}, "
                                    "want 1 (an escaping exception exits 1 as well, so only the "
                                    "return proves the driver finished)")
                if discarded != 1:
                    problems.append(f"{mod_name}: a {kind} refusal discarded the stage "
                                    f"{discarded} times, want 1")

    rule("a refusal on every page still measures all five", all_refuse)
    rule("one refusal names it and counts the rest", one_refuses)
    rule("a clean run publishes and exits 0", none_refuses)
    rule("a crash in shoot is a refusal, not an abort", crash_is_a_refusal)
    rule("report_refusal reports and returns", report_returns)
    rule("a missing token census refuses", census_missing)
    rule("a token under the floor refuses", census_floor)
    rule("an absent stress row refuses", row_absent)
    rule("a stress row needs a table", stress_needs_table)
    rule("somebody must carry the stress row", stress_must_be_carried)
    rule("lock.py and testlock.py discard and return 1", single_shot)

    # Canary pair, through the same `rule` helper against its own lists: without it,
    # `rule = lambda w, b, *a: ran.append((w, 0))` reports 11 rules holding while calling none
    # of them. Same hollowing the mutation self-test probes for, same reason it exists.
    probe_ran, probe_bad = [], []
    rule("canary", lambda problems: problems.append("a problem"), probe_ran, probe_bad)
    rule("canary", lambda problems: None, probe_ran, probe_bad)
    # The pair has to agree on BOTH lists the helper writes. Checking only the problem list let
    # `ran.append((why, 0))` through the whole probe battery: the problem still reached `bad`, so
    # the canary looked satisfied while every real rule was being recorded as passing. Measured.
    counts = [n for _, n in probe_ran]
    if counts != [1, 0] or len(probe_bad) != 1:
        bad.append(f"the rule helper is hollow: the canary pair recorded counts {counts} and "
                   f"{len(probe_bad)} problems, want [1, 0] and 1")

    return ran, bad


def self_test_verdict(ran, bad):
    """Turn what self_test() returned into an exit code, outside the code whose reach it measures.

    The count check lives here rather than at the tail of self_test() for the reason mutate.py
    learned the hard way: a bare `return` inserted above the tail skips every remaining rule, and
    both lists are already complete by then, so a reach check placed after them cannot see the
    block that never ran.
    """
    if len(ran) != EXPECTED_RULES:
        print(f"capture self-test: {len(ran)} rules ran, want {EXPECTED_RULES} "
              "(a `return` inside self_test() skips every rule below it)", file=sys.stderr)
        return 1
    if bad:
        for p in bad:
            print(f"  capture self-test: {p}", file=sys.stderr)
        print(f"capture self-test: {len([n for n, k in ran if k])} of {EXPECTED_RULES} "
              "rules failed", file=sys.stderr)
        return 1
    print(f"capture self-test: {EXPECTED_RULES} refusal rules hold "
          "(no browser, no fixture, nothing written)")
    return 0


if __name__ == "__main__":
    if "--self-test" in sys.argv[1:]:
        extra = [a for a in sys.argv[1:] if a != "--self-test"]
        if extra:
            raise SystemExit(
                f"--self-test takes no other arguments: it starts no browser and writes nothing, "
                f"so {extra[0]!r} would be silently ignored")
        sys.exit(self_test_verdict(*self_test()))
    sys.exit(main())
