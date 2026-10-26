#!/usr/bin/env python3
"""Measure the five GET-rendered admin pages at every acceptance viewport, and photograph them at
the artifact one.

The acceptance criterion is a measurement, not a reading of the stylesheet, so this drives a real
browser: it starts the daemon on a throwaway fixture, signs in, then asks Chrome for
`documentElement.scrollWidth` at each viewport in `WIDTHS` and a screenshot at the artifact width.
A page that fits reports `scrollWidth == width`; anything larger is horizontal overflow, which is
what the criterion refuses.

Two viewports, because one is not a criterion (#149). `overflow-wrap: break-word` alone measures
360 px at a 360 px viewport and 1471 px at a 1024 px one, so a driver that looks only at the phone
certifies a fix that leaves the desktop broken -- and `break-word` is the value a reasonable person
reaches for first. Each viewport is refused on its own and named in its own summary line, so "the
audit page fits" is never a sentence that quietly means "at one of the widths".

These five are the easy pages: each has a URL, so a browser can be sent straight to it. The two
result pages cannot be reached this way and have their own drivers (lock.py, testlock.py), which
is where the click-the-shipped-button refusals live.

Nothing reaches `--out` until every page has passed at every viewport. Each page is measured and
refused before its own bytes are staged, and staging is not publishing: a run that refuses the
audit page leaves the committed artifacts exactly as they were, rather than four fresh PNGs and
one stale one under filenames that all look current.

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

# The viewports a page has to fit, and which of them produces an artifact.
#
# `cdp.WIDTH` is the artifact contract: docs/screenshots/360/ is named for it and the committed
# PNGs are taken at it. The second entry is #149, and it is here because the first one alone was
# not a criterion. Measured on the audit page while fixing #140:
#
#   viewport        as shipped   `overflow-wrap: break-word`   `overflow-wrap: anywhere`
#   360 px               914 px                       360 px                        360 px
#   1024 px             1471 px                      1471 px                       1024 px
#
# The middle column is the trap: the value a reasonable person reaches for first passes the phone
# criterion and leaves the desktop a 1471 px document, and before #149 nothing here looked at a
# desktop width at all. So a width is refused on its own terms and named in its own summary line;
# "the audit page fits" is only ever said about one viewport.
ARTIFACT_WIDTH = cdp.WIDTH
DESKTOP_WIDTH = 1024
WIDTHS = (ARTIFACT_WIDTH, DESKTOP_WIDTH)

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
    (measured: 117 characters, 808 px). The rule that fixes it is `overflow-wrap` on `body` in
    `app.css` -- `anywhere`, with `break-word` under it, because `break-word` fixes the narrow
    layout and not the wide one. What the width refusal above pins is that *some* wrap rule is
    there: delete both declarations and this page overflows, so its photograph is not taken. It
    cannot pin which one. `break-word` on its own still yields a 360 px document at a 360 px
    viewport, so the value that carries the desktop half is watched only by the grep in
    `TestTheStylesheetPrintsTheNarrowLabels`, and that grep pins presence rather than effect --
    until some criterion measures a second width, which is #149.

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


class FixtureRefusal(SystemExit):
    """A refusal about the FIXTURE, not about the viewport: the page was not pushed hard enough.

    It is a `SystemExit` so every handler and every rule that already treats a refusal as a
    `SystemExit` keeps working unchanged. The type exists for one reason: a page refused this way is
    not width evidence at ANY viewport, and `main()` has to be able to carry that to the pass that
    never ran the guard, rather than printing `5/5 pages fit at 1024 px` about a page it just
    refused as non-evidence -- #140's claim, printed in the opposite direction.
    """


def shoot(cap, spec, name, width=ARTIFACT_WIDTH, fixture_untested=False):
    """Measure one page at one viewport, and photograph it if this is the artifact viewport.

    `width` is a parameter and not a read of `cdp.WIDTH`, because the caller walks `WIDTHS`: a
    refusal has to name the viewport it was measured at, and a helper that printed the constant
    would report every overflow as a phone problem -- which is how a 1471 px desktop document
    would keep arriving labelled 360.

    `fixture_untested` says this page was already refused on the artifact pass for a reason about
    its fixture. Returns "untested" for such a page so the caller can keep it out of the count of
    pages that fitted; returns None otherwise.
    """
    page, session, base = spec.url, cap.session, cap.base
    session.set_width(width)
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
    # The viewport is a claim this function has to earn, not one its caller gets to make. `width` is
    # what was asked for; `inner` is what the browser says it laid out at. Compared here, at the
    # first moment both exist, because the failure it catches is the one this criterion exists to
    # close: an emulation call that stops reaching the browser leaves both passes measuring a 360 px
    # document, and the transcript prints two viewports agreeing. Nothing in the self-test can see
    # it from above -- `_ShootSession` stands where `set_width` is, so every rule there verifies that
    # `shoot()` ASKED. This is the one assertion below the fake, and it is also what makes comparing
    # `scroll` against the requested `width` legitimate: the two are pinned to agree here or the run
    # refuses there.
    if got["inner"] != width:
        raise SystemExit(f"{page} rendered at {got['inner']} px when asked for {width} px; the "
                         "viewport never moved, so this measurement is about neither width")
    # Asked at every viewport, not just the artifact one. A table a wider media query hides, or a
    # form that stops rendering above 40rem, fits its viewport perfectly and reports a comfortable
    # width while having rendered nothing worth photographing. #140 was a page that fitted nothing;
    # #149 is about a page that fits at the width nobody looked at, and an empty page fits at both.
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
    if got["scroll"] > width:
        cause = ("" if longest is None else
                 f", and its hardest unbreakable token is {longest} characters")
        # The consequence differs by viewport, and saying the wrong one is a small lie with a
        # deadline on it: only the artifact pass has a photograph to withhold.
        consequence = ("its photograph is not taken" if width == ARTIFACT_WIDTH else
                       "nothing is published, though this viewport takes no photograph of its own")
        raise SystemExit(f"{page} is {got['scroll']} px wide at a {width} px viewport{cause}; "
                         f"the criterion refuses this page, so {consequence}")
    # After the width refusal, before the write. A page that overflows should report the overflow,
    # which is the alarming fact; a page that fits is then asked whether fitting meant anything.
    # #140's whole lesson is that the audit page can fit at 360 px having been asked for nothing,
    # because the row that broke it came from the daemon failing on this machine rather than from
    # the fixture.
    #
    # The artifact pass only. These two ask whether the FIXTURE was pushed hard enough for a width
    # measurement to mean anything -- a question about the row the daemon was handed, not about the
    # viewport it is later rendered at. Running them on every pass would print one broken fixture
    # twice and read as two findings.
    if spec.stress and width == ARTIFACT_WIDTH:
        # Retyped rather than raised directly: these two state their own sentences and this adds no
        # words to them, it only records that the sentence is about the fixture. The type is what
        # lets main() carry the outcome across passes without re-reading prose.
        try:
            require_stress_row(got, page)
            require_unbreakable(got, page)
        except SystemExit as exc:
            raise FixtureRefusal(str(exc)) from None
    # Hoisted above the split instead of repeated in both arms: each arm needs a browser that is
    # still up, and one call cannot drift from its twin when someone moves the split.
    cap.require_running()
    if fixture_untested:
        # Measured, and reported as measuring nothing. The page failed the fixture guard on the
        # artifact pass, so the number below is a width without a stress row -- true, and not
        # evidence. Counting it as fitting is what the reviewer drove and named.
        print(f"  {page:<10} @{width:<5} scrollWidth={got['scroll']:>4} "
              f"innerWidth={got['inner']:>4} height={got['height']:>5} "
              f"{proof_of(got, spec)}{hard} UNTESTED (fixture)", flush=True)
        return "untested"
    if width != ARTIFACT_WIDTH:
        # Measured, refused, printed -- and no bytes. The committed set belongs to the artifact
        # viewport; a desktop PNG staged under the same basename would be published by whichever
        # viewport happened to run last.
        print(f"  {page:<10} @{width:<5} scrollWidth={got['scroll']:>4} "
              f"innerWidth={got['inner']:>4} height={got['height']:>5} "
              f"{proof_of(got, spec)}{hard} FITS", flush=True)
        return
    session.write_png(name)
    print(f"  {page:<10} @{width:<5} scrollWidth={got['scroll']:>4} "
          f"innerWidth={got['inner']:>4} height={got['height']:>5} "
          f"{proof_of(got, spec)}{hard} FITS  {name}", flush=True)


def proof_of(got, spec):
    """The number that says this page was really rendered, in the shape that page can give."""
    return (f"tables={got['tables']} cells={got['cells']}" if spec.has_rows
            else f"forms={got['forms']} fields={got['fields']}")


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
    # One entry per viewport, each holding the pages that failed THERE. Collapsing this into one
    # list would make "4/5 pages fit" a sentence about no particular width, which is the exact
    # ambiguity #149 was filed for: on the page that started it, 360 px and 1024 px have different
    # answers, and the honest transcript has to say which is which.
    refused = []
    # Pages refused for something about their fixture, on whichever pass ran the guard. Carried
    # across the viewport loop because the judgement is not viewport-scoped: a page whose stress row
    # was never rendered cannot demonstrate anything about 1024 px either.
    fixture_failed = set()
    with cdp.Capture("pages", CHROME_FLAGS) as cap:
        for width in WIDTHS:
            refused_here = []
            untested_here = []
            for spec in PAGES:
                # Only the artifact viewport owns a filename. The desktop pass is given a label so a
                # refusal can be attributed to it, and never a path it could stage bytes into.
                name = (op.join(out, spec.artifact + ".png") if width == ARTIFACT_WIDTH
                        else f"{spec.url} at {width} px")
                # Set before the try: an exception arm below must not leave the caller reading a
                # name that was never bound -- which is exactly how this line first failed, as an
                # UnboundLocalError inside the handler for someone else's refusal.
                outcome = None
                try:
                    outcome = shoot_fn(cap, spec, name, width,
                                       fixture_untested=spec.url in fixture_failed)
                except FixtureRefusal as exc:
                    # Before the SystemExit arm, which would otherwise swallow it. Reported exactly
                    # as any other refusal is, and additionally remembered.
                    cdp.report_refusal(name, exc)
                    refused_here.append(spec.url)
                    fixture_failed.add(spec.url)
                except SystemExit as exc:
                    # `report_refusal` and not a raising helper: this handler is the only thing
                    # standing between one bad page and the four that were measured fine, and until
                    # this head the helper raised from inside it, so both `append` lines below were
                    # unreachable and the run died on the first refusal while looking like a refusal.
                    cdp.report_refusal(name, exc)
                    refused_here.append(spec.url)
                except Exception as exc:  # a traceback is not an answer; a refusal is
                    cdp.report_refusal(name, f"{type(exc).__name__}: {exc}")
                    refused_here.append(spec.url)
                if outcome == "untested":
                    untested_here.append(spec.url)
            refused.append((width, refused_here, untested_here))
        # Every viewport is walked even when the first one refuses, and that is a decision rather
        # than a loop nesting. Stopping at the first failing width would report a page that is
        # broken at both as broken at one, and the second number is the one that tells you whether
        # the fix is a wrap rule or a column that nobody sized. The cost of finishing is one more
        # navigation per page; the cost of not finishing is a lie about the other viewport.
        if any(pages or untested for _, pages, untested in refused):
            # The count is the exit status, so it has to be the true one. This line used to print
            # `4/4 pages fit` and return 0 whatever had happened, because a refusal was caught
            # per page and the loop simply carried on -- a run that photographed three pages and
            # refused the fourth reported complete success, which is the failure this whole
            # package exists to make impossible. The opposite failure then replaced it: the
            # refusal helper raised, so nothing was ever counted and the sentence below was
            # decoration. `--self-test` now pins both directions -- every page measured even when
            # all five refuse, and exit 1 whenever any one of them does.
            cdp.discard_stage()
            for width, pages, untested in refused:
                # A fixture-untested page is in neither the numerator nor the refused list: it did
                # not fail this viewport, and it did not prove anything about it either. The third
                # clause is what keeps those two apart in the sentence.
                line = (f"{len(PAGES) - len(pages) - len(untested)}/{len(PAGES)} pages fit at "
                        f"{width} px")
                if pages:
                    line += f"; refused: {', '.join(pages)}"
                if untested:
                    line += f"; untested (fixture): {', '.join(untested)}"
                print(line, file=sys.stderr, flush=True)
            print(f"Nothing was written to {out}.", file=sys.stderr, flush=True)
            return 1
        cap.session.publish()
    for width, pages, untested in refused:
        # Unreachable while anything was refused, and it has to stay that way: this is the success
        # sentence, and it would print a full count over a page that was never tested.
        if pages or untested:
            raise SystemExit(f"the success path printed a fit count for {width} px while "
                             f"{len(pages)} refused and {len(untested)} were untested")
        print(f"{len(PAGES)}/{len(PAGES)} pages fit at {width} px", flush=True)
    print(f"-> {out}", flush=True)
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

# The count of rules registered below, and the reason the count is a number in the source rather
# than `len(ran)`: a `return` inserted in the middle of self_test() leaves both lists complete for
# whatever ran, so only a number written somewhere else can tell a full pass from a truncated one.
# 11 from #140, plus the seven #149 adds for the second viewport.
EXPECTED_RULES = 21


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


def _expected_pass():
    """The transcript a clean run has to produce: every page, at every viewport, in `WIDTHS` order."""
    return [(p.url, w) for w in WIDTHS for p in PAGES]


def _run_main(refuse=(), refuse_at=(), fixture_refuse=(), crash=None):
    """Call the real main() with the machine stubbed out; report what the driver actually did.

    `refuse` names URLs that fail at EVERY viewport. `refuse_at` names `"url@width"` pairs that
    fail at one viewport only, and that is the shape #149 is about: a page that fits at 360 px and
    is a 1471 px document at 1024 px. A harness that could only refuse a URL everywhere could not
    tell a driver that walks both viewports from one that walks the phone and calls it a day.

    `fixture_refuse` names URLs that fail the FIXTURE guards, which the real driver runs on the
    artifact pass only -- the shape the cross-pass carry has to be tested against, because a stub
    that refused at both widths would prove nothing about carrying anything.

    Returns (rc, entered, stderr_text, stdout_text, published, discard_calls). `rc` is the string
    "raised" when main() let an exception escape instead of returning a code -- itself a finding,
    because an escaping exception and a refusal both exit 1 and only the return proves the driver
    finished its own bookkeeping.
    """
    log, entered = [], []
    urls = set(refuse)
    at = set(refuse_at)
    fx = set(fixture_refuse)

    def stub(cap, spec, name, width, fixture_untested=False):
        entered.append((spec.url, width))
        if spec.url in urls or f"{spec.url}@{width}" in at:
            raise SystemExit(f"{spec.url} is 9999 px wide at a {width} px viewport")
        if spec.url in fx and width == ARTIFACT_WIDTH:
            raise FixtureRefusal(f"{spec.url}: the hardest token anywhere in the table is 3 "
                                 "characters, under the 40 this check needs to call a width "
                                 "measurement evidence (#140).")
        if crash is not None:
            raise crash
        return "untested" if fixture_untested else None

    rc = "raised"
    err = io.StringIO()
    console = io.StringIO()
    with _stubbed(log):
        try:
            with contextlib.redirect_stdout(console), contextlib.redirect_stderr(err):
                rc = main(stub)
        except SystemExit as exc:
            rc = f"raised:{exc.code}"
    return rc, entered, err.getvalue(), console.getvalue(), "publish" in log, log.count("discard")


@contextlib.contextmanager
def _fake_browser(got):
    """The CDP reads, replaced, so the real `shoot()` can be called with no Chrome.

    Only the reads are faked. `shoot` itself -- the order it sets the viewport in, the checks it
    runs, whether it stages bytes -- is the code under test, and every one of those is invisible
    to the rules that drive `main()` through a stubbed `shoot_fn`.
    """
    saved = (cdp.measure, cdp.require_records, cdp.require_labelled_cells, cdp.require_form)
    cdp.measure = lambda session, extra="": dict(got)
    cdp.require_records = lambda g, w: None
    cdp.require_labelled_cells = lambda g, w: None
    cdp.require_form = lambda g, w: None
    try:
        yield
    finally:
        cdp.measure, cdp.require_records, cdp.require_labelled_cells, cdp.require_form = saved


class _ShootSession:
    """The session `shoot()` talks to: it records, and it remembers nothing else.

    Distinct from the `_FakeSession` above, which stands in for the session `main()` publishes.
    Two different contracts, and one class quietly serving both would let a rule pass on the
    other's methods.
    """

    def __init__(self, log, where="/users"):
        self.log, self._where = log, where

    def set_width(self, width):
        self.log.append(("set_width", width))

    def goto(self, url):
        self.log.append(("goto", url))

    def where(self):
        return self._where

    def settle(self, seconds=0.4):
        pass

    def write_png(self, path):
        self.log.append(("png", path))


class _ShootCap:
    """A Capture whose session records what `shoot()` asked it to do."""

    def __init__(self, log, where="/users"):
        self.session = _ShootSession(log, where)
        self.base = "http://127.0.0.1:0"

    def require_running(self):
        pass


def _run_shoot(got, width, log=None, where=None, spec=None, fixture_untested=False, out=None):
    """The real shoot() against a fake browser; returns (log, refusal_text_or_None).

    The fake session answers with the spec's own URL by default: `shoot` refuses a page that
    answers something else, and a rule about the fixture guards must not be satisfied -- or
    defeated -- by that check.

    `out` collects what `shoot()` printed, for the rules whose claim IS the printed line; left
    None, the output is discarded as it always was. `fixture_untested` drives the carried flag, so
    a rule can ask what the desktop pass says about a page the artifact pass refused for its
    fixture.
    """
    log = [] if log is None else log
    spec = Page("/users", "users", True) if spec is None else spec
    with _fake_browser(got), contextlib.redirect_stdout(out or io.StringIO()):
        try:
            shoot(_ShootCap(log, spec.url if where is None else where), spec,
                  "/tmp/opencode/selftest-users.png", width,
                  fixture_untested=fixture_untested)
        except SystemExit as exc:
            return log, str(exc)
    return log, None


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
        rc, entered, err, _out, published, discarded = _run_main(refuse=urls)
        want = _expected_pass()
        if len(entered) != len(want):
            problems.append(f"a refusal on every page measured {len(entered)} of {len(want)} "
                            f"({entered}); the loop has to finish for the count it prints to be "
                            "true, at every viewport")
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
        rc, entered, err, _out, published, discarded = _run_main(refuse=[victim])
        if entered != _expected_pass():
            problems.append(f"one refusal stopped the loop at {entered}, "
                            f"want {_expected_pass()}")
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
        rc, entered, err, _out, published, discarded = _run_main()
        if rc != 0:
            problems.append(f"a clean run -> main() returned {rc!r}, want 0")
        if not published:
            problems.append("a clean run did not publish the stage")
        if discarded:
            problems.append("a clean run discarded the stage")
        if entered != _expected_pass():
            problems.append(f"a clean run measured {entered}, want {_expected_pass()}")

    def crash_is_a_refusal(problems):
        # A bug inside shoot() must not cost the other four pages either: `except Exception` is
        # the only thing standing between a TypeError on page 1 and a run that measured nothing.
        rc, entered, err, _out, published, discarded = _run_main(crash=TypeError("a real bug"))
        if len(entered) != len(_expected_pass()):
            problems.append(f"an exception on every page stopped the loop at {len(entered)}, "
                            f"want {len(_expected_pass())}")
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

    def desktop_only_refusal(problems):
        # The trap #149 exists for: a page that fits at 360 px and is a 1471 px document at
        # 1024 px -- which is exactly what `overflow-wrap: break-word` alone produces. A driver
        # that measures one viewport calls that run green, so the contract being pinned here is
        # that the phone's 5/5 buys the run nothing.
        victim = "/audit"
        rc, _entered, err, _out, published, discarded = _run_main(
            refuse_at=[f"{victim}@{DESKTOP_WIDTH}"])
        if rc != 1:
            problems.append(f"a refusal at {DESKTOP_WIDTH} px only -> main() returned {rc!r}, "
                            "want 1")
        if published:
            problems.append(f"a refusal at {DESKTOP_WIDTH} px only published the stage anyway")
        if discarded != 1:
            problems.append(f"a refusal at {DESKTOP_WIDTH} px only discarded the stage "
                            f"{discarded} times, want 1")
        # Both viewports have to appear, with the right answer for each. Reading only "did it
        # fail" would let a driver refuse the phone pass and report the desktop as fitting --
        # the same sentence, inverted, and still a lie about the desktop.
        if f"{len(PAGES)}/{len(PAGES)} pages fit at {ARTIFACT_WIDTH} px" not in err:
            problems.append(f"the {ARTIFACT_WIDTH} px viewport is not reported as fitting: "
                            f"{err.strip()[-200:]!r}")
        if f"{len(PAGES) - 1}/{len(PAGES)} pages fit at {DESKTOP_WIDTH} px" not in err:
            problems.append(f"the {DESKTOP_WIDTH} px viewport is not reported as refusing: "
                            f"{err.strip()[-200:]!r}")
        if victim not in err:
            problems.append(f"the refusal does not name {victim!r}")

    def every_viewport_measured(problems):
        # `for width in WIDTHS[:1]` keeps every other rule in this file green: the phone still
        # measures five pages, publishes, and exits 0. This is the rule that notices a viewport
        # was dropped, and it is written as an exact transcript rather than a count so a pass
        # that measured the same page twice at 360 px cannot satisfy it.
        rc, entered, _err, _out, published, _discarded = _run_main()
        want = _expected_pass()
        if entered != want:
            problems.append(f"a clean run measured {entered}, want {want}")
        if rc != 0 or not published:
            problems.append(f"a clean run -> rc={rc!r} published={published}, want 0 and True")

    def summary_names_each_viewport(problems):
        # "5/5 pages fit" is not a criterion until it says at what width. #140's lesson was that a
        # fitting page can mean nothing; #149's is that it can mean a different viewport than the
        # one you are reading about.
        _rc, _entered, _err, out_text, _published, _discarded = _run_main()
        for width in WIDTHS:
            if f"{len(PAGES)}/{len(PAGES)} pages fit at {width} px" not in out_text:
                problems.append(f"the success transcript has no line naming {width} px: "
                                f"{out_text!r}")

    def viewport_set_before_navigation(problems):
        # Deleting `session.set_width(width)` is invisible to every rule above: main() would still
        # walk both viewports, print two honest summary lines and publish, while both passes
        # measured the same 360 px layout and reported it as two viewports agreeing. So this rule
        # calls the real shoot() and reads the order of what it asked for.
        log, refusal = _run_shoot({"scroll": 1024, "inner": 1024, "height": 900,
                                   "tables": 1, "cells": 4, "longest": 251}, DESKTOP_WIDTH)
        if refusal is not None:
            problems.append(f"the desktop pass refused a page that fits: {refusal}")
        kinds = [k for k, _ in log]
        if ("set_width", DESKTOP_WIDTH) not in log:
            problems.append(f"shoot() never moved the session to {DESKTOP_WIDTH} px ({log}); both "
                            "passes would measure the artifact viewport and be reported as two")
        if "goto" not in kinds or kinds.index("set_width") > kinds.index("goto"):
            problems.append(f"the viewport is moved after the page is loaded ({log}); the render "
                            "being measured is the one the previous pass left behind")

    def only_the_artifact_viewport_writes(problems):
        # A desktop PNG staged under the same basename would be published by whichever viewport ran
        # last, and docs/screenshots/360/ would start carrying a 1024 px screenshot with nothing in
        # the transcript saying so.
        log, refusal = _run_shoot({"scroll": 1024, "inner": 1024, "height": 900,
                                   "tables": 1, "cells": 4, "longest": 251}, DESKTOP_WIDTH)
        if refusal is not None or any(k == "png" for k, _ in log):
            problems.append(f"the {DESKTOP_WIDTH} px pass wrote bytes "
                            f"({log}, refusal={refusal!r})")
        log, refusal = _run_shoot({"scroll": ARTIFACT_WIDTH, "inner": ARTIFACT_WIDTH,
                                   "height": 900, "tables": 1, "cells": 4, "longest": 251},
                                  ARTIFACT_WIDTH)
        if refusal is not None or not any(k == "png" for k, _ in log):
            problems.append(f"the {ARTIFACT_WIDTH} px pass did not stage its artifact "
                            f"({log}, refusal={refusal!r})")

    def refusal_names_the_viewport_that_failed(problems):
        # The 1471 px document has to arrive labelled 1024. A message that prints the constant
        # instead of the viewport sends the next reader to the narrow stylesheet to fix a desktop
        # problem, which is where #149's middle column went unnoticed in the first place.
        wide = {"scroll": 1471, "inner": DESKTOP_WIDTH, "height": 900,
                "tables": 1, "cells": 4, "longest": 251}
        _log, refusal = _run_shoot(wide, DESKTOP_WIDTH)
        if refusal is None:
            problems.append("a 1471 px document at a 1024 px viewport was accepted")
        elif f"{DESKTOP_WIDTH} px viewport" not in refusal:
            problems.append(f"the refusal does not name {DESKTOP_WIDTH} px: {refusal!r}")
        elif "photograph is not taken" in refusal:
            problems.append("the desktop refusal withholds a photograph that does not exist: "
                            f"{refusal!r}")
        _log, refusal = _run_shoot(dict(wide, scroll=400, inner=ARTIFACT_WIDTH), ARTIFACT_WIDTH)
        if refusal is None or "photograph is not taken" not in (refusal or ""):
            problems.append("the artifact refusal does not say the photograph was withheld: "
                            f"{refusal!r}")


    def viewport_is_verified_not_assumed(problems):
        """The pass has to prove the browser moved, not that `set_width` was called.

        Every other rule here stands above `_ShootSession`, the fake that answers where
        `set_width` is, so all of them can only see that `shoot()` ASKED for a width. Two mutants
        live below that line and both were measured surviving the eighteen: delete the
        `Emulation.setDeviceMetricsOverride` call but keep `self.width = width`, and ignore the
        argument and re-send 360. Each leaves both passes measuring a 360 px document while the
        transcript prints two viewports agreeing -- #149's own failure, with the self-test green and
        no mutation set over `testing/capture` to catch it. `inner == width`, asserted in `shoot()`
        and driven here, is the one pin under the fake.
        """
        wide = {"scroll": 1024, "inner": 1024, "height": 900, "tables": 0, "cells": 0,
                "forms": 1, "fields": 3}
        # The control first: a page that really did lay out at 1024 px must pass, or every refusal
        # below is a fixture nobody can satisfy.
        _log, refusal = _run_shoot(wide, DESKTOP_WIDTH)
        if refusal is not None:
            problems.append(f"a page measured at the width it was asked for was refused: {refusal!r}")
        # Asked for 1024, laid out at 360: the emulation call that never reached the browser.
        _log, refusal = _run_shoot(dict(wide, inner=ARTIFACT_WIDTH, scroll=ARTIFACT_WIDTH),
                                   DESKTOP_WIDTH)
        if refusal is None or "never moved" not in refusal:
            problems.append(f"a {DESKTOP_WIDTH} px pass measuring {ARTIFACT_WIDTH} px was accepted: "
                            f"{refusal!r}")
        for want in (str(ARTIFACT_WIDTH), str(DESKTOP_WIDTH)):
            if want not in (refusal or ""):
                problems.append(f"the refusal does not name {want} px: {refusal!r}")
        # The stale direction too: a pass that kept the artifact width after being moved back would
        # publish a 1024 px photograph under a 360 px filename.
        _log, refusal = _run_shoot(wide, ARTIFACT_WIDTH)
        if refusal is None or "never moved" not in refusal:
            problems.append(f"the artifact pass measuring {DESKTOP_WIDTH} px was accepted: "
                            f"{refusal!r}")

    def fixture_refusal_is_carried_to_the_other_pass(problems):
        """A page refused for its fixture at 360 px is not evidence at 1024 px either.

        The guards run on the artifact pass only -- right, because one broken fixture is one finding.
        The consequence was not: the desktop pass then measured the same page and the summary printed
        `5/5 pages fit at 1024 px` about a page the run had just refused as non-evidence. That is
        #140's claim -- a page that could not have overflowed is not width evidence -- arriving in
        the opposite direction, and it contradicts this file's own sentence that a transcript
        printing one line is not evidence about the other.
        """
        victim = "/audit"
        rc, entered, err, out, published, _discarded = _run_main(fixture_refuse=[victim])
        for width in WIDTHS:
            if (victim, width) not in entered:
                problems.append(f"{victim} was not measured at {width} px; a carried refusal is "
                                "not a skipped page")
        if f"untested (fixture): {victim}" not in err:
            problems.append(f"the summary counts {victim} as fitting: {err!r}")
        if "5/5 pages fit at 1024 px" in err:
            problems.append(f"the summary still certifies all five at {DESKTOP_WIDTH} px: {err!r}")
        if f"4/5 pages fit at {DESKTOP_WIDTH} px" not in err:
            problems.append(f"the desktop line does not count {victim} out: {err!r}")
        # The artifact pass keeps its own verdict: refused, not merely untested.
        if f"refused: {victim}" not in err:
            problems.append(f"the artifact pass stopped refusing {victim}: {err!r}")
        if rc != 1:
            problems.append(f"a fixture refusal -> main() returned {rc!r}, want 1")
        if published:
            problems.append("a fixture refusal published anyway")
        # The printed line is `shoot()`'s half and the stub cannot show it -- the stub reports an
        # outcome, it does not print a transcript. So the label is driven through the real `shoot()`
        # against the fake browser, at the viewport that never ran the guard.
        line = io.StringIO()
        pushed = {"scroll": 1024, "inner": 1024, "height": 900, "tables": 1, "cells": 4,
                  "longest": 251, "stress": True}
        _log, refusal = _run_shoot(pushed, DESKTOP_WIDTH,
                                   spec=Page(victim, "audit", True, stress=True),
                                   fixture_untested=True, out=line)
        printed = line.getvalue()
        if refusal is not None:
            problems.append(f"an untested page was refused at {DESKTOP_WIDTH} px instead of "
                            f"reported: {refusal!r}")
        if "UNTESTED (fixture)" not in printed:
            problems.append(f"the desktop line does not say untested: {printed!r}")
        if "FITS" in printed:
            problems.append(f"an untested page still printed FITS: {printed!r}")
        if "scrollWidth=1024" not in printed:
            problems.append(f"the untested line stopped reporting what it measured: {printed!r}")

        # Control: with every fixture intact, nothing is untested and both widths say five.
        rc, _e, err, out, published, _d = _run_main()
        if "untested" in out + err:
            problems.append(f"a clean run reported something untested: {out!r} {err!r}")
        if "5/5 pages fit" not in out or rc != 0 or not published:
            problems.append(f"the control run stopped counting all five: rc={rc!r} {out!r}")

    def an_untested_page_is_not_a_refusal_at_the_other_viewport(problems):
        """The two buckets stay two. Folding them together is the tempting simplification.

        A fixture-untested page did not fail the desktop viewport -- the page rendered and fitted.
        It also did not prove anything about it. Printing it under `refused:` would tell an operator
        to go fix a width that is not broken; printing it as fitting is the failure the rule above
        pins. So the desktop line names it in a clause of its own and the artifact line keeps the
        refusal, and this rule is what stops either clause being merged into the other.
        """
        victim = "/audit"
        _rc, _entered, err, _out, _published, _d = _run_main(fixture_refuse=[victim])
        desktop = [ln for ln in err.splitlines() if f"at {DESKTOP_WIDTH} px" in ln]
        artifact = [ln for ln in err.splitlines() if f"at {ARTIFACT_WIDTH} px" in ln]
        if len(desktop) != 1 or len(artifact) != 1:
            problems.append(f"expected one summary line per viewport, got {err!r}")
            return
        if f"refused: {victim}" in desktop[0]:
            problems.append(f"the desktop pass calls a fixture gap a width refusal: {desktop[0]!r}")
        if f"untested (fixture): {victim}" not in desktop[0]:
            problems.append(f"the desktop clause is missing: {desktop[0]!r}")
        if f"refused: {victim}" not in artifact[0]:
            problems.append(f"the artifact pass lost its refusal: {artifact[0]!r}")
        if "untested" in artifact[0]:
            problems.append(f"the artifact pass calls its own refusal untested: {artifact[0]!r}")

    def fixture_guards_run_once(problems):
        # require_stress_row and require_unbreakable ask whether the FIXTURE was pushed hard enough
        # for a width number to mean anything. Asked at both viewports they turn one broken fixture
        # into two refusals, and the summary counts the same page twice -- which is the failure this
        # file's own counting rules exist to prevent, arriving from the other direction.
        spec = Page("/audit", "audit", True, stress=True)
        pushed = {"scroll": 1024, "inner": 1024, "height": 900, "tables": 1, "cells": 4,
                  "longest": 251, "stress": False}
        _log, refusal = _run_shoot(pushed, DESKTOP_WIDTH, spec=spec)
        if refusal is not None:
            problems.append(f"the {DESKTOP_WIDTH} px pass ran the fixture guards ({refusal!r}); "
                            "one broken fixture would be counted as two refusals")
        _log, refusal = _run_shoot(dict(pushed, scroll=ARTIFACT_WIDTH, inner=ARTIFACT_WIDTH),
                                   ARTIFACT_WIDTH, spec=spec)
        if refusal is None or "seeded" not in refusal:
            problems.append("the artifact pass accepted a page with no seeded stress row: "
                            f"{refusal!r}")

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
    rule("a desktop-only refusal fails the run", desktop_only_refusal)
    rule("every page is measured at every viewport", every_viewport_measured)
    rule("each summary line names its viewport", summary_names_each_viewport)
    rule("the viewport is moved before the page is loaded", viewport_set_before_navigation)
    rule("only the artifact viewport writes bytes", only_the_artifact_viewport_writes)
    rule("a refusal names the viewport that failed", refusal_names_the_viewport_that_failed)
    rule("the fixture guards run at the artifact viewport only", fixture_guards_run_once)
    rule("the viewport is verified, not assumed", viewport_is_verified_not_assumed)
    rule("a fixture refusal carries to the other viewport", fixture_refusal_is_carried_to_the_other_pass)
    rule("an untested page is not a refusal at the other viewport",
         an_untested_page_is_not_a_refusal_at_the_other_viewport)

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
