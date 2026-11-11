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
    ./pages.py --self-test        # the refusal contract only: no browser, no daemon, and no
                                  # write anywhere except a temporary box of its own

Run it from anywhere; every path is resolved from this file's location. Output goes to
testing/capture/runs/pages/ unless --out names a directory inside the repository, so re-capturing
the committed artifacts is an explicit `--out docs/screenshots/360`.
"""
import contextlib
import datetime
import re
import importlib
import inspect
import io
import json
import os
import os.path as op
import shutil
import sys
import tempfile

sys.path.insert(0, op.dirname(op.abspath(__file__)))
import cdp  # noqa: E402
import manifest as capture_manifest  # noqa: E402

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


# The row census, in the shape the state checks below read: one entry per `table.rows` body row,
# each `[row-head, [cell text, ...]]`. `textContent`, not `innerText`, for the same reason the token
# census above takes it — innerText is the rendered text, so a cell that wraps correctly is exactly
# the thing that could hand back something the declaration does not match. Whitespace is collapsed
# because the templates indent their markup: the dashboard's `<time>` element sits on its own line,
# and the When cell is `\n 2026-03-18 21:34:05 -04:00\n`, which is the same *text* and not the same
# string. The head is empty for a table with no `<th>` per row, which is what the audit page is.
ROWS_JS = ("rows:[...document.querySelectorAll('table.rows tbody tr')].map(tr=>["
           "[...tr.querySelectorAll('th')].map(e=>e.textContent.trim())[0]||'',"
           "[...tr.querySelectorAll('td')].map(e=>e.textContent.replace(/\\s+/g,' ').trim())])")


def require_declared_state(got, spec, what):
    """Every cell this artifact is a photograph *of* has to say what the fixture declares.

    The dashboard's row text is a branch: `locked out until`, `scheduled, not enforced`, `not now;
    next at`, `nothing scheduled` (internal/web/templates/dashboard.tmpl:36-55), chosen by
    `schedule.Evaluate(p, now)`. Before #207 nothing said which branch had been taken, so the same
    fixture photographed a different *page* on Sunday at 22:00 than on Wednesday at 14:00 and both
    runs published a file named dashboard.png that was current and honest and different. That is not
    a flaky test — it is a criterion that was never pinned to a state, and the frame was the thing
    that quietly varied.

    So the state is declared here, in the same table as the URL and the artifact name, and a page
    that renders anything else is refused rather than committed. The refusal is the point: a capture
    taken outside the state this fixture can hold has to report, because the alternative is a third
    page arriving under a name that already means something else.

    A pair is `("row to look at", "a cell in that row, exactly").` The first half is matched against
    any cell or the row head, because the account name is a `<th>` on the dashboard and a `<td>`
    nowhere; the second half has to equal a cell in the same row, which is what makes it a claim
    about *that account's* clock rather than about the page containing both strings somewhere.
    """
    rows = got.get("rows")
    if not rows:
        raise SystemExit(
            f"{what}: no row census came back, so nothing can be checked against the state this "
            "artifact declares. An empty table fits at any width, which is exactly why a fit is not "
            "the check being made here (#207).")
    for needle, want in spec.state:
        hit = next((r for r in rows if needle in r[0] or any(needle == c for c in r[1])), None)
        if hit is None:
            raise SystemExit(
                f"{what}: no row mentions {needle!r}, so this page is not the page this artifact "
                f"declares. Rendered rows: {[r[0] or r[1][:1] for r in rows]} (#207).")
        if want not in hit[1]:
            raise SystemExit(
                f"{what}: the {needle!r} row says {hit[1]!r}, and this artifact is a photograph of "
                f"the state where it says {want!r}. The fixture or the clock has moved; either way "
                "the photograph would be of a state nothing declares (#207).")


def require_declared_rows(got, spec, declared, what):
    """The audit page shows the number of rows the fixture declared, and not one more.

    The mechanism this closes is a run counter wearing a photograph's clothes. Signing in over HTTP
    writes `auth.success` into the same trail the page photographs (internal/web/handlers.go:150), so
    every capture appends a row, and a row is ~254 px: the committed audit.png had reached 360x15004
    while a fixture built from nothing rendered 2936 px, which is 92% of Chromium's capture surface
    and nothing to do with the stylesheet (#207 cause 2, and #144's height bound).

    `cdp.pin_appended_rows` checks the file. This checks the *page*, which is the thing in the
    photograph: a reader of the artifact is counting rows on paper, and the two can disagree when the
    reader and the writer disagree about how many rows a trail holds — a Tail limit, a malformed line
    the reader drops, a template that starts repeating a row. Checking only the file would let that
    through with a caption about the trail.

    `declared` comes from the fixture, not from this file: the number of rows is a property of the
    document that was seeded, and a second home for it here would be a number that goes stale the day
    somebody seeds a row.
    """
    shown = len(got.get("rows") or [])
    if shown != declared:
        raise SystemExit(
            f"{what}: the page renders {shown} rows and the fixture declared {declared}. The height "
            "of this artifact is that number, which is how it came to be a run counter in the first "
            "place — so the mismatch is refused rather than photographed (#207).")


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
DEFAULT_FIXTURE = "pages"


class Page:
    def __init__(self, url, artifact, has_rows, *, nav, stress=False, refuse=None,
                 fixture=DEFAULT_FIXTURE, needles=(), state=(), declares_rows=False):
        self.url, self.artifact = url, artifact
        self.has_rows, self.stress = has_rows, stress
        self.refuse = refuse
        # Which entry in the shell's navigation this page is standing in, declared by the fixture
        # rather than worked out by the driver (#204).
        #
        # Keyword-only and with no default on purpose: every page has to say which entry is its own,
        # and a page added without one fails at import, in the file where it was written, instead of
        # quietly photographing a navigation nobody asked about. The alternative -- letting the
        # driver derive it from the URL -- would put a second copy of the "which section is this
        # path in" rule in Python, one segment behind the rule in `internal/web/sources.go` that
        # already decides it, and D-0025 refused exactly that duplication for JavaScript. A declared
        # string is not a rule: when the Go rule changes, this disagrees with the page and the
        # capture refuses, which is a finding a person reads, not a second implementation that
        # drifts silently.
        #
        # It is also what makes #204's criterion 1 askable. Criterion 1 is about "the nav link whose
        # href resolves to that page", and a guard that only compared properties could not tell a
        # correct mark from a stylesheet that marks the FIRST entry on every page -- which would
        # then be photographed, marked, on all nine frames.
        self.nav = nav
        # The state this artifact is a photograph *of*, as `(row, exact cell text)` pairs, and whether
        # the page's row count is checked against the fixture's declared count. Both exist because a
        # page is a branch and not a constant: the same fixture, honestly built, renders a different
        # document at another hour, and a driver with nothing to say about which one it took will
        # commit either (#207 causes 1, 2, 5 and 6). `needles` above asks whether a page is the right
        # *document*; these ask whether it is the right *reading* of it.
        self.state = tuple(state)
        self.declares_rows = declares_rows
        # Which FIXTURE this artifact is evidence about. Most pages are photographed from the one
        # document `pages-setup.sh` builds; `pages-noprof` is a different daemon on a different
        # document, because its page is a property of the document and not of a POST (#198). The
        # The value is the run directory's name, which is the only handle there is: `cdp.fixture`
        # reads `<run dir>/fixture.env`, so asking for a fixture that was never set up is already a
        # refusal. What it does NOT do is notice a page declared against the wrong fixture while both
        # directories exist -- hence the check at the bottom of this constructor, and the needles.
        self.fixture = fixture
        # Strings the rendered page HAS to contain for this artifact to be what it is named for.
        # `Refusal.about` does this job for a refused write; this is the same idea for a page whose
        # subject is a branch of a template. Without it, running this driver against a fixture that
        # DOES have profiles produces a perfectly well-formed, 360-px-fitting, entirely wrong
        # `users-no-profiles.png`, and every other check in this file passes on it.
        self.needles = tuple(needles)
        # The coupling between the two fields above, refused here rather than at capture time.
        #
        # A needle says "this artifact is evidence that the page said a particular thing". Only a
        # fixture that withholds something can produce that sentence, and the default fixture is the
        # one that has everything -- so a Page declaring needles and `fixture="pages"` is a page whose
        # subject is a branch the document it names cannot render. It would be caught by the needle
        # check at run time, as a refusal after a daemon has been started, a browser launched and four
        # pages photographed; and in the self-test it would be caught by nothing at all, because the
        # fake serves whatever body a rule hands it.
        #
        # Constructed pages are imported, so this raises at import: the mistake dies where it is made.
        # (Not `__post_init__`: `Page` is a plain class with an `__init__`, and a hook by that name is
        # a dataclass-only name that would never fire -- measured by declaring exactly such a page and
        # watching this raise.)
        if self.needles and fixture == DEFAULT_FIXTURE:
            raise ValueError(
                f"{artifact}: a page with needles must name the fixture that can withhold what they "
                f"check for, not the default {DEFAULT_FIXTURE!r} -- the ordinary fixture renders the "
                "populated branch, so the artifact could only ever be a photograph of the wrong state")


class Refusal:
    """A page photographed AFTER a refused write rather than on a plain GET (#138).

    `action` is matched as a suffix because the form's action carries the profile's id, which
    belongs to the fixture and not to this file. The driver reads the real action off the DOM,
    fills the boxes, and clicks the button that is in the page -- the same discipline the two POST
    drivers hold to, for the same reason: a driver that assembled its own post body would keep
    passing on a page whose shipped form posted a field no handler reads.

    `fields` are the boxes to fill before the click, and `checks` how many boxes of a repeated name
    to tick. `keep` names the one whose value has to still be in the box in the photograph, and
    `keep_value` is what was put there: a screenshot of a refusal sentence alone cannot show the
    half of #138 that is actually new, which is that the parent's own text came back with it.

    `about` is a needle the refusal sentence has to contain. It exists because the two measurements
    above can both pass on the wrong refusal: if the checkbox fill silently stopped ticking, the
    route answers 422 about the days instead, the echo puts `25:99` back anyway -- it is the same
    closure -- and the run publishes a `profiles-refused.png` that is a photograph of a different
    bug than the one its filename claims.
    """

    def __init__(self, action, fields, checks=None, keep="", keep_value="", about=""):
        self.action, self.fields = action, fields
        self.checks = checks or {}
        self.keep, self.keep_value = keep, keep_value
        self.about = about


PAGES = [
    # The state, declared in the same breath as the URL and the artifact name. Every cell below was
    # read off a fresh fixture on this branch, and each is a branch that would otherwise be decided by
    # the hour the capture happened to run:
    #
    #   casey   `locked out until 00:00`  -- the held branch, from an `allDay` rule on all seven days,
    #                                       at the midnight the rule was written with (needs #209's
    #                                       fix to say `00:00` rather than `05:00` on a day within 27h
    #                                       of a clock change; on a tree without it this refuses)
    #   casey   `no desktop session`      -- an enabled, held account with no session on purpose: a
    #                                       session here would be locked by the next tick and the lock
    #                                       would append a row to the trail the audit page photographs
    #   casey   `Every day, every hour`   -- the same row's Why cell: the rule the verdict *names*. It is
    #                                       declared beside the clock because `EndsAt` is a max over the
    #                                       open rules and the named rule is whichever one produced that
    #                                       max (internal/schedule/schedule.go:171-178), so a second
    #                                       enabled rule can move the row's words at one hour of the day
    #                                       and not another -- #220, three hours a day, measured. The
    #                                       fixture's curfew now releases at the all-day rule's own
    #                                       instant and a tie names the first rule in the profile
    #                                       (schedule_test.go:294). That test pins the tie on a synthetic
    #                                       two-rule profile; this cell is the only place that checks it
    #                                       on the document the photographs are made from. A guard that
    #                                       read the clock alone would have passed a frame whose row
    #                                       changed which rule it blamed at 21:00.
    #   alex    `nothing scheduled`       -- a profile with no enabled rule, so it says this at every
    #                                       hour of every day; a live curfew would say `next at 21:00`
    #                                       by day and `nothing scheduled` at 21:30 (#207 cause 5)
    #   alex    `x11 · seat0 · locked`    -- the census from testing/capture/bin/loginctl, not from
    #                                       this machine's logind (#207 cause 6)
    Page("/", "dashboard", True, nav="/", state=(
        ("casey", "locked out until 00:00"),
        ("casey", "Every day, every hour"),
        ("casey", "no desktop session"),
        ("alex", "nothing scheduled"),
        ("alex", "x11 · seat0 · locked"),
    )),
    Page("/users", "users", True, nav="/users"),
    Page("/profiles", "profiles", True, nav="/profiles"),
    # The refused profile form, as its own artifact. It is a different page from /profiles as of
    # #138 -- a sentence above the list and a form full of typed text -- and the page with more
    # prose in it is the one a 360 px criterion has to be measured on, not the tidy one.
    Page("/profiles", "profiles-refused", True, nav="/profiles",
         refuse=Refusal("/rules",
                        {"name": "School night", "start": "21:00", "end": "25:99"},
                        checks={"days": 2}, keep="end", keep_value="25:99",
                        about="HH:MM")),
    # The same URL as the `/users` row above, from a different document, showing what #192 made it
    # say when there is nothing to assign. It is declared beside its sibling rather than in a table
    # of its own so that "which page is this evidence about" still has one answer: manifest.py's
    # template_for asks this list, gets `/users`, and pins the artifact to `users.tmpl` + `page.tmpl`
    # exactly like `users.png` -- the second STATE of a page is not a second template (#138's rule,
    # #198's second use of it).
    Page("/users", "users-no-profiles", True, nav="/users", fixture="pages-noprof",
         needles=("There is no profile to assign yet", "Create a profile")),
    # `declares_rows` makes the page's own row count a claim the fixture vouches for: eight rows, the
    # seven seeded by pages-setup.sh plus the one this run's sign-in appends (cdp.pin_appended_rows
    # checks the file, require_declared_rows checks the photograph). Without it the height of this
    # artifact was a tally of how many times anyone had run a driver -- 360x15004 committed against
    # 2936 px from a fixture built from nothing (#207 cause 2).
    #
    # The pair pins the newest row: the event the sign-in wrote, at the instant the fixture declares,
    # in the zone the fixture declares the reader to be in. That last part is not decoration. The When
    # column used to be formatted with `MST` (internal/web/templates/audit.tmpl), and Go's
    # time.Parse hands a row whose offset matches the local zone that zone's abbreviation, so the
    # same trail printed `... 21:34:05 EDT` on one host and `... 21:34:05 -0400` on another (measured
    # under TZ= for America/New_York, UTC and Asia/Kolkata). #210 made the column print the stored
    # offset, so the text below is now host-independent and the declared state would hold without
    # PLOCK_CAPTURE_TZ. The zone stays declared because a daemon's local zone is a condition of a
    # frame whatever the template does with it, and this pair is the thing that would notice if a
    # host-dependent label ever came back -- which is exactly how it earned its place here.
    Page("/audit", "audit", True, nav="/audit", stress=True, declares_rows=True, state=(
        ("auth.success", "2026-03-18 21:34:05 -04:00"),
    )),
    Page("/settings", "settings", False, nav="/settings"),
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
    if spec.refuse:
        # After the landing check and before the measurement: the page has to be the one this entry
        # declares before anything is posted to it, and the width has to be measured on the response
        # to that post rather than on the form as first rendered.
        action = refusal_form(session, spec)
        session.settle()
        landed = session.where()
        if landed != action:
            raise SystemExit(f"{page}: the refused write was supposed to answer at {action} and "
                             f"the browser is at {landed}; nothing was measured")
    # One read per page. `extra` is what measure() takes for exactly this: the census keys a page
    # needs are spliced into the same object, so the number printed in the transcript, the number
    # the width refusal quotes and the number the stress guards compare are all from one render.
    extra = cdp.FORMS_JS
    # On every page and at every viewport, with no condition on it: the control census is a claim
    # about the stylesheet, and the stylesheet is the same file on /audit as on /settings (#202).
    extra += "," + cdp.CONTROLS_JS
    # The same argument for the navigation (#204): #204's table is five links read on five pages,
    # and the only reason nobody had ever read them is that nothing in this folder asked a browser
    # for a computed style at all. Unconditional, because the shell is one template and a page whose
    # navigation was never censused would not fail the guard -- it would answer `nav=0 links`.
    extra += "," + cdp.NAV_JS
    # And the state vocabulary (#199), unconditional for the same reason: the classes are on four
    # of the five pages and the stylesheet is one file, so a page whose state words were never
    # read would not fail the guard -- it would answer `states=0` and be believed.
    extra += "," + cdp.STATE_JS
    if spec.has_rows:
        extra += "," + LONGEST_JS
    if spec.stress:
        extra += "," + STRESS_JS
    if spec.refuse:
        extra += refusal_extra(action, spec.refuse.keep)
    if spec.needles:
        extra += "," + NEEDLE_JS
    if spec.state or spec.declares_rows:
        extra += "," + ROWS_JS
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
    # After the width refusal, like the fixture guards: a page that overflows should report the
    # overflow, which is the alarming fact. Before any write, because a photograph of a refusal
    # that is not a refusal is the wrong evidence filed under the right filename.
    if spec.refuse:
        require_refusal(got, spec, page)
    if spec.needles:
        # Before the width refusal and before any write: a page from the wrong fixture has nothing
        # worth measuring, and withholding the photograph is the whole point.
        require_needles(got, spec, name if width != ARTIFACT_WIDTH else spec.artifact)
    # The state and the row count, on the artifact pass only, for the reason the #140 guards two lines
    # below give for theirs: both are claims about the document the fixture built, not about the
    # viewport it is later rendered at, and running them on every pass prints one wrong fixture twice
    # and reads as two findings. Before the width refusal because a page in the wrong state overflowing
    # or not, it is the wrong state that has to be reported first; and before any write, because a
    # photograph of a state nothing declares is the thing this whole section exists to withhold.
    if spec.state and width == ARTIFACT_WIDTH:
        require_declared_state(got, spec, page)
    if spec.declares_rows and width == ARTIFACT_WIDTH:
        require_declared_rows(got, spec, int(cap.declared_rows()), page)
    if spec.stress and width == ARTIFACT_WIDTH:
        # Retyped rather than raised directly: these two state their own sentences and this adds no
        # words to them, it only records that the sentence is about the fixture. The type is what
        # lets main() carry the outcome across passes without re-reading prose.
        try:
            require_stress_row(got, page)
            require_unbreakable(got, page)
        except SystemExit as exc:
            raise FixtureRefusal(str(exc)) from None
    # Every control on the page has to agree with the page: the page's own typeface, one width
    # model, and a tick box that cannot be laid out away from the words that name it (#202).
    #
    # After the width refusal, like the fixture guards: a page that overflows should report the
    # overflow first, because that is the alarming fact and the width number is the one that stops
    # existing once the run refuses. Before the write, like every other check, because a photograph
    # of a page whose controls are in three fonts is the wrong evidence under a committed filename.
    # At both viewports, because none of the three claims is about a phone.
    cdp.require_control_font(got, page)
    cdp.require_control_width(got, page)
    cdp.require_choice_groups(got, page)
    # The navigation, at both viewports and on every page, for the same two reasons as the three
    # guards above: the shell is one template served by one stylesheet, and a page that overflows
    # should report its width before it reports its links.
    #
    # `spec.nav` is what makes the first of these #204's criterion 1 rather than a property
    # comparison: it names which entry this page is standing in, so a stylesheet that marks the
    # wrong one -- the first, always, which is the bug a naive mark would have -- is refused here
    # instead of being photographed nine times.
    cdp.require_nav_marked(got, page, spec.nav)
    cdp.require_nav_in_palette(got, page)
    cdp.require_nav_distinguishable(got, page, spec.nav)
    # The state vocabulary (#199), at both viewports and on every page, for the same two reasons as
    # the six guards above: the words are rendered by four of the five templates out of one
    # stylesheet, and a page that overflows reports its width first.
    cdp.require_state_legible(got, page)
    # Hoisted above the split instead of repeated in both arms: each arm needs a browser that is
    # still up, and one call cannot drift from its twin when someone moves the split.
    cap.require_running()
    if fixture_untested:
        # Measured, and reported as measuring nothing. The page failed the fixture guard on the
        # artifact pass, so the number below is a width without a stress row -- true, and not
        # evidence. Counting it as fitting is what the reviewer drove and named.
        print(f"  {page:<10} @{width:<5} scrollWidth={got['scroll']:>4} "
              f"innerWidth={got['inner']:>4} height={got['height']:>5} "
              f"{proof_of(got, spec)} {cdp.census_line(got)} {cdp.nav_census_line(got)} {cdp.state_census_line(got)}"
              f"{hard} UNTESTED (fixture)", flush=True)
        return "untested"
    if width != ARTIFACT_WIDTH:
        # Measured, refused, printed -- and no bytes. The committed set belongs to the artifact
        # viewport; a desktop PNG staged under the same basename would be published by whichever
        # viewport happened to run last.
        print(f"  {page:<10} @{width:<5} scrollWidth={got['scroll']:>4} "
              f"innerWidth={got['inner']:>4} height={got['height']:>5} "
              f"{proof_of(got, spec)} {cdp.census_line(got)} {cdp.nav_census_line(got)} {cdp.state_census_line(got)}"
              f"{hard} FITS", flush=True)
        return
    session.write_png(name)
    print(f"  {page:<10} @{width:<5} scrollWidth={got['scroll']:>4} "
          f"innerWidth={got['inner']:>4} height={got['height']:>5} "
          f"{proof_of(got, spec)} {cdp.census_line(got)} {cdp.nav_census_line(got)} {cdp.state_census_line(got)}"
              f"{hard} FITS  {name}", flush=True)


# Read after a refused write. Both keys are the photograph's proof that the refusal happened in
# the browser and not only in this file: `refusal` is the sentence the parent is shown, and `kept`
# is what is still in the box they typed into.
REFUSAL_JS = ("'refusal':(document.querySelector('#refusal')||{}).innerText||'',"
              "'kept':(function(){const f=document.querySelector('form[action=\"%s\"]');"
              "if(!f)return null;const i=f.querySelector('[name=\"%s\"]');"
              "return i?i.value:null})()")


def fill_script(action, fields, checks):
    """The JavaScript that types into the shipped form, given the action it was found under.

    Two bugs lived in this string, and both were reported as something else:

      * `querySelectorAll(...)[:2]` is not a slice. It is a parse error, measured against the
        same `google-chrome --headless=new` these drivers drive (Chrome 156): the reply carries
        `exceptionDetails.exception.description = "SyntaxError: Unexpected token ':'"`, and
        `result` holds the error object with no `value` key in it. Nothing is undefined and no
        `TypeError` is thrown at runtime, so the `try` below cannot see it -- what actually
        happened was that `value()` read `["result"].get("value")`, dropped `exceptionDetails`
        and answered `None`, and the driver printed "the form vanished" about a form that was
        sitting right there. `Array.from(...).slice(0, n)` is the same intent in a language that
        has it, and the parse error now reaches the `threw:` string from the other side.
      * the whole script is one expression the browser runs verbatim, so a wrong selector here is
        not a wrong measurement, it is a wrong page. Hence the `try`: a script that throws at
        RUNTIME returns `threw: <message>` and the caller says which of the two failures it is. A
        script that never parses is the same shape now, but by way of `cdp.value`, not this `try`.
    """
    return ("(() => { try { const f = document.querySelector('form[action=\"%s\"]'); if (!f) "
            "return 'gone'; " % action
            + "".join("f.querySelector('[name=\"%s\"]').value = %s; " % (k, json.dumps(v))
                      for k, v in fields.items())
            + "".join("Array.from(f.querySelectorAll('[name=\"%s\"]')).slice(0, %d)"
                      ".forEach(i => i.checked = true); " % (k, n) for k, n in checks.items())
            + "return 'filled'; } catch (e) { return 'threw: ' + e.message; } })()")


def refusal_extra(resolved_action, keep):
    """The two keys the refused page is measured with, read out of the form that answered.

    `resolved_action` is what the probe found (`/profiles/school/rules`), never the suffix the spec
    declares to FIND it (`/rules`). Splicing the suffix yields `form[action="/rules"]`, which
    matches nothing: `kept` comes back null on a page that echoed the typed value correctly, and the
    driver refuses the capture by accusing the product of the bug this branch closes. The sentence
    check still passes, because `#refusal` is global -- which is what made the pair read like a
    product failure instead of a selector one.
    """
    return "," + (REFUSAL_JS % (resolved_action, keep))


def refusal_form(session, spec):
    """Fill the shipped form and press its own button, returning the action it posted to.

    The probe runs before anything is typed and refuses a form that cannot post. A page whose
    button is missing, or whose CSRF field is empty, answers 422 for a reason that has nothing to
    do with the field under test, and the photograph would be of the wrong refusal -- and would
    look like a refusal in the transcript, because a refusal is what gets printed either way.
    """
    ref = spec.refuse
    probe = ("(() => { const f = document.querySelector('form[action$=\"%s\"]');"
             "if (!f) return JSON.stringify({missing: true});"
             "const b = f.querySelector('button[type=submit]');"
             "const c = f.querySelector('input[name=csrf]');"
             "return JSON.stringify({action: f.getAttribute('action'), button: !!b,"
             "csrf: !!(c && c.value)}); })()") % ref.action
    found = json.loads(session.value(probe) or "{}")
    if found.get("missing"):
        raise SystemExit(f"{spec.url} carries no form whose action ends in {ref.action!r}, so "
                         "there is nothing to refuse and no refusal to photograph")
    if not found.get("button"):
        raise SystemExit(f"the form at {found.get('action')!r} has no submit button; pressing it "
                         "would be a script posting a form, not the shipped control")
    if not found.get("csrf"):
        raise SystemExit("the refusal form posts no CSRF token -- the click would be refused as a "
                         "forged request, and the photograph would be of the wrong refusal")
    action = found["action"]

    fill = fill_script(action, ref.fields, ref.checks)
    got = session.value(fill)
    if got != "filled":
        # Two different failures print the same sentence otherwise. A form that is gone and a form
        # whose fields the driver got wrong are fixed in different files, and the second one is a
        # bug in this driver rather than in the page.
        if got and str(got).startswith("threw"):
            raise SystemExit(f"{spec.url}: the refusal form at {action!r} is present but could not "
                             f"be filled ({got}). A form that is missing says 'gone'; this is the "
                             f"driver asking for a field that is not on the form, or setting one "
                             f"wrong.")
        raise SystemExit(f"{spec.url}: the refusal form at {action!r} vanished before it could be "
                         "filled")
    clicked = ("(() => { const f = document.querySelector('form[action=\"%s\"]');"
               "const b = f && f.querySelector('button[type=submit]');"
               "if (!b) return 'no button'; b.click(); return 'clicked'; })()") % action
    if session.value(clicked) != "clicked":
        raise SystemExit(f"{spec.url}: the button at {action!r} was never clicked, so nothing was "
                         "refused and the photograph would be a plain GET")
    return action


def require_refusal(got, spec, what):
    """Refuse a photograph that does not show what it is named for (#138).

    Both halves are refusals rather than notes because the artifact IS the claim: a PNG called
    `profiles-refused` that shows an accepted write, or a refusal that threw the typed value away
    again, is evidence for the bug rather than against it.
    """
    if not (got.get("refusal") or "").strip():
        raise SystemExit(f"{what} shows no refusal sentence, so this is a photograph of a page "
                         "that accepted the write. The artifact the criterion wants is the refused "
                         "one, and nothing is published either way")
    want = spec.refuse.keep_value
    if got.get("kept") != want:
        raise SystemExit(f"{what} refused, but the {spec.refuse.keep!r} box holds "
                         f"{got.get('kept')!r} instead of {want!r}: the write was refused and the "
                         "typed value thrown away, which is the behaviour #138 closed. A sentence "
                         "over an empty form is not the fix")
    needle = spec.refuse.about
    if needle and needle not in (got.get("refusal") or ""):
        raise SystemExit(f"{what} was refused for something else -- the sentence does not say "
                         f"{needle!r}, it says {got.get('refusal')!r}. The artifact is named for "
                         "the refusal this page is evidence about, and a 422 about a different "
                         "field photographs the same way")



# The page's own text, read in the same pass as every other measurement (one navigation, one read --
# a second `Runtime.evaluate` would be a second render, and the width, the census and the sentence
# would then not be from the same page). Only pages that declare needles pay for it.
NEEDLE_JS = "'body':(document.body||{}).innerText||''"


def require_needles(got, spec, what):
    """Refuse a photograph of the wrong document (#198).

    This is the only check that can tell `users-no-profiles.png` taken from the fixture it names from
    `users-no-profiles.png` taken from the ordinary one. Width, census and fit all pass on the wrong
    fixture -- the ordinary accounts page is a fine page, it is simply not this one -- and the file
    would be committed as evidence about a branch nobody looked at.
    """
    body = got.get("body") or ""
    missing = [n for n in spec.needles if n not in body]
    if missing:
        raise FixtureRefusal(
            f"{what} does not say {missing[0]!r}, so this is not the page the artifact is named "
            f"for. The document being served is the wrong fixture: run "
            f"{spec.fixture}-setup.sh and pass --fixture {spec.fixture}. Rendered text begins "
            f"{body[:160]!r}")


def proof_of(got, spec):
    """The number that says this page was really rendered, in the shape that page can give."""
    return (f"tables={got['tables']} cells={got['cells']}" if spec.has_rows
            else f"forms={got['forms']} fields={got['fields']}")


def fixture_mode(argv=None):
    """Which fixture this run photographs, defaulting to the long-standing one.

    Parsed from argv rather than the environment: `--out` already works this way, and a mode that
    arrives silently from the environment would let a stale export decide which document a committed
    artifact was taken from. Both `--fixture NAME` and `--fixture=NAME` are accepted because `--out`
    accepts both, and a mistyped `--fixturex` is refused for the same reason `--outt` is.
    """
    argv = list(sys.argv[1:] if argv is None else argv)
    mode = "pages"
    i = 0
    while i < len(argv):
        a = argv[i]
        if a == "--fixture":
            if i + 1 >= len(argv):
                raise SystemExit("--fixture wants a fixture name, e.g. --fixture pages-noprof")
            mode, i = argv[i + 1], i + 2
            continue
        if a.startswith("--fixture="):
            mode, i = a.split("=", 1)[1], i + 1
            continue
        if a.startswith("--fixture"):
            raise SystemExit(f"{a} is not a flag; the fixture selector is --fixture NAME")
        i += 1
    return mode


def select_pages(mode, pages=None):
    """The artifacts one fixture can produce, and a refusal when the pair does not exist.

    Filtering is what makes a second fixture safe to add: without it, running the driver against
    `pages-noprof` would photograph the dashboard, the audit page and the settings form against a
    document built for none of them, and each would either refuse for an unrelated reason or, worse,
    fit at 360 px and be reported as a measurement.
    """
    pages = PAGES if pages is None else pages
    chosen = [p for p in pages if p.fixture == mode]
    if not chosen:
        named = sorted({p.fixture for p in pages})
        raise SystemExit(
            f"no page declares fixture {mode!r}; the fixtures this driver knows are {named}. "
            f"Run {mode}-setup.sh and name it with --fixture {mode}")
    return chosen


def fixture_envs(pages):
    """`runs/<fixture>/fixture.env` for each fixture the given pages belong to, in page order.

    This is the whole plumbing of #211: the conditions a frame was taken under are declared by a
    setup script, written to the run directory by `capture_env`, and the recorder can only state them
    if the driver hands over the files that hold them.

    Only fixtures whose file EXISTS are returned. `capture_env` is what writes it, so a missing file
    means that fixture's setup script never ran in this clone, and inventing conditions for it is the
    defect #211 closes -- the recorder then writes a manifest that claims nothing (`capture-manifest/1`)
    and says so out loud, which is a weaker claim rather than a false one.

    One path per fixture, in first-appearance order: two identical `--fixture-env` arguments make
    `manifest.record` refuse, because a file that states the same fixture twice is either a copy left
    in the wrong run directory or a fixture renamed halfway through.
    """
    seen, paths = set(), []
    for spec in pages:
        if spec.fixture in seen:
            continue
        seen.add(spec.fixture)
        path = op.join(cdp.RUNS, spec.fixture, "fixture.env")
        if op.exists(path):
            paths.append(path)
    return paths


def _refused_frame(cap, keep, out, spec, width, exc, notes):
    """One refusal's evidence: the sentence, the frames measured so far, and a frame of this page.

    Called from inside the handler, in this order, for two reasons. `keep_refused` opens the
    directory first because opening is what clears a previous refusal's frames -- writing the frame
    before that would delete the frame. And the frame is of the page AS IT NOW STANDS at the
    viewport that refused, which is the one thing the run cannot describe in words: #220's own
    refusal said `locked out until 18h 16m` where the fixture declares `00:00`, and the picture of
    a page in a state nothing declares is the evidence for the *next* such refusal, even though it
    can never be published as the evidence for this one.

    `notes` is the driver's running log of refusals, which becomes REFUSED.txt beside the frames.
    A frame with no sentence next to it is the thing #211 says a frame is not.
    """
    notes.append(f"  refused at {width} px: {spec.url} ({spec.artifact}.png)\n"
                 f"    {exc}\n")
    if cdp.keep_refused(keep, out=out) is None:
        # The keep was declined (nowhere safe to write), so the frames are gone and there is no
        # directory to put a diagnostic frame in. `keep_refused` has already said why.
        return
    cdp.capture_refused(cap, op.join(keep, f"{spec.artifact}@{width}.png"))


def _refusal_note(mode, out, envs, notes):
    """REFUSED.txt: what the run refused, under what conditions, and what it left next to this file.

    The frames in the refused directory are worthless on their own. A PNG with no sentence under it
    is the exact thing #211 was filed about -- a frame that looks current and states nothing about
    the state it was taken in -- and these frames state less than most, because the whole reason
    they exist is that the state was NOT the declared one. So the sentences and the conditions come
    here, in the directory, where they travel with the bytes.

    The conditions are named by their files rather than copied out. `fixture.env` is still on disk
    beside the run directory and is the only thing that can change; a copy made here would be a
    second answer to the same question, and #213 is what those two do when they drift.
    """
    when = datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")
    lines = [
        "# What this run refused, and what it left here.\n",
        "#\n",
        "# These are the frames a run measured and then declined to publish, and they are\n",
        "# not evidence: nothing records them, --out was left untouched, and the files named\n",
        "# on this run's FITS lines do not exist at those paths. Read them to see what the\n",
        "# page actually rendered; never quote them as current.\n",
        f"\nfixture: {mode}\nrefused at: {when}\n--out (untouched): {out}\n",
        "conditions stated in:\n"
        + ("".join(f"  {p}\n" for p in envs) if envs
           else "  (none: no setup script has run, so no fixture.env declares anything here)\n"),
        f"\nrefusals ({len(notes)}):\n" + ("".join(notes) if notes else "  (none recorded)\n"),
    ]
    return "".join(lines)


def main(shoot_fn=None, record_fn=None):
    # Resolved at call time rather than as a default argument. `def main(shoot_fn=shoot)` binds
    # the function object when this module is imported, so a caller that patches `pages.shoot`
    # would be ignored -- the exact shape that once made a hermeticity fix here a silent no-op.
    if shoot_fn is None:
        shoot_fn = shoot
    # Same reason, and it matters more here: writing the manifest is the step that turns a
    # directory of PNGs into evidence, and the self-test has to be able to watch it happen
    # without a git repository under it (#144).
    if record_fn is None:
        record_fn = capture_manifest.record
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
    mode = fixture_mode()
    selected = select_pages(mode)
    # The two declaration guards above deliberately read PAGES and not `selected`: they ask whether
    # this FILE declares a stress row somewhere, which is true in every fixture mode, and narrowing
    # them to the selection would let a mode with one quiet page delete the audit page's insurance.
    # What the selection changes is what gets MEASURED, and a run in pages-noprof says "1/1 pages
    # fit" -- a claim about one page, which is the only page it looked at.
    # Named by the MODE, not a constant: two fixtures writing one scratch directory means the second
    # run's manifest certifies the first run's PNGs as just-captured, with a `captured_at` and
    # `sources_rev` that describe a run which photographed one page. Separate directories make each
    # manifest the record of the run that wrote it.
    out = cdp.capture_out(mode, fixture_flag=True)
    # One entry per viewport, each holding the pages that failed THERE. Collapsing this into one
    # list would make "4/5 pages fit" a sentence about no particular width, which is the exact
    # ambiguity #149 was filed for: on the page that started it, 360 px and 1024 px have different
    # answers, and the honest transcript has to say which is which.
    refused = []
    # Pages refused for something about their fixture, on whichever pass ran the guard. Carried
    # across the viewport loop because the judgement is not viewport-scoped: a page whose stress row
    # was never rendered cannot demonstrate anything about 1024 px either.
    fixture_failed = set()
    # What a refusal leaves behind, and where. `out` is the contract -- it gets nothing -- and
    # `keep` is the honesty: the frames this run measured and a frame of the page that refused, so
    # that "the fixture or the clock has moved" is a claim the next pair of hands can check instead
    # of re-argue (#220). Built from `cdp.RUNS`, never from `out`; see `cdp.refused_dir`.
    keep = cdp.refused_dir(mode)
    notes = []
    with cdp.Capture(mode, CHROME_FLAGS) as cap:
        for width in WIDTHS:
            refused_here = []
            untested_here = []
            for spec in selected:
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
                    _refused_frame(cap, keep, out, spec, width, exc, notes)
                    refused_here.append(spec.url)
                    fixture_failed.add(spec.url)
                except SystemExit as exc:
                    # `report_refusal` and not a raising helper: this handler is the only thing
                    # standing between one bad page and the four that were measured fine, and until
                    # this head the helper raised from inside it, so both `append` lines below were
                    # unreachable and the run died on the first refusal while looking like a refusal.
                    cdp.report_refusal(name, exc)
                    _refused_frame(cap, keep, out, spec, width, exc, notes)
                    refused_here.append(spec.url)
                except Exception as exc:  # a traceback is not an answer; a refusal is
                    cdp.report_refusal(name, f"{type(exc).__name__}: {exc}")
                    _refused_frame(cap, keep, out, spec, width, f"{type(exc).__name__}: {exc}",
                                   notes)
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
            # NOT `discard_stage()`. The frames this run measured are moved to `keep` and the
            # refusals written beside them, because destroying them made a refusal unfalsifiable:
            # the transcript printed `FITS .../runs/pages/users.png` five times and then deleted
            # all five, so the only record of what the pages actually rendered was a sentence
            # nobody could look at. What is NOT here is a partial publish -- `out` still gets
            # nothing, and the nine committed frames still come from one fixture in one state or
            # they come from nowhere (#222 is the report on what a half-replaced set does).
            kept = cdp.keep_refused(keep, note=_refusal_note(mode, out, fixture_envs(selected),
                                                             notes), out=out)
            for width, pages, untested in refused:
                # A fixture-untested page is in neither the numerator nor the refused list: it did
                # not fail this viewport, and it did not prove anything about it either. The third
                # clause is what keeps those two apart in the sentence.
                # `selected`, never PAGES: the denominator is "pages this run looked at". A fixture
                # mode photographs one page, and `1/7 pages fit` would be a sentence about a sweep
                # that did not happen -- while `7/7` after a one-page run would be worse. The mode is
                # in the line so a one-page transcript cannot be read as the whole set.
                line = (f"{len(selected) - len(pages) - len(untested)}/{len(selected)} pages fit "
                        f"at {width} px (fixture {mode})")
                if pages:
                    line += f"; refused: {', '.join(pages)}"
                if untested:
                    line += f"; untested (fixture): {', '.join(untested)}"
                print(line, file=sys.stderr, flush=True)
            print(f"Nothing was written to {out}.", file=sys.stderr, flush=True)
            if kept is not None:
                left = sorted(f for f in os.listdir(keep) if f.endswith(".png"))
                print(f"  {len(left)} frame(s) this run measured are in {keep}, with "
                      f"{cdp.REFUSED_NOTE} beside them"
                      + (f": {', '.join(left)}" if left else ""), file=sys.stderr, flush=True)
                # The retracting sentence, and the reason this block is not just a `cp`. Every FITS
                # line above printed a path under `out`, and `out` does not hold those files and
                # never held them this run: the bytes were staged, then moved here. Left alone, the
                # transcript of a refused run is a list of filenames that do not exist.
                print("  the paths the FITS lines above print are files this run did NOT write. "
                      f"They are in {keep}, and they are not evidence: no manifest names them and "
                      "nothing certifies them, which is where a refused page belongs.",
                      file=sys.stderr, flush=True)
            return 1
        cap.session.publish()
    for width, pages, untested in refused:
        # Unreachable while anything was refused, and it has to stay that way: this is the success
        # sentence, and it would print a full count over a page that was never tested.
        if pages or untested:
            raise SystemExit(f"the success path printed a fit count for {width} px while "
                             f"{len(pages)} refused and {len(untested)} were untested")
        print(f"{len(selected)}/{len(selected)} pages fit at {width} px (fixture {mode})",
              flush=True)
    # Last, after every sentence that has to be true about this run. The manifest is what makes
    # these PNGs evidence about the pages they depict (#144): it names the sources each page was
    # rendered from and the commit that last moved them, and `make verify` refuses when that has
    # drifted. Written here rather than beside `publish()` so the guard above -- the one that
    # refuses to call a refused page a fit page -- cannot be stepped over by a manifest that
    # already landed. And never on the refusal path: a manifest over a run that wrote nothing
    # would certify photographs this run did not take.
    # The envs of the fixtures this run actually photographed, so each entry is stamped from the
    # conditions its own setup script declared and not from whichever fixture happened to be asked
    # for last. Empty when no setup script has run, which records a manifest without conditions.
    # `shot` is this run's own list, not the directory's: the frames it photographed are the frames
    # whose conditions are true today, and a frame whose bytes happened not to move is still a frame
    # taken under today's fixture (#211).
    record_fn(out, envs=fixture_envs(selected), shot=[spec.artifact for spec in selected])
    print(f"-> {op.join(out, capture_manifest.MANIFEST)}", flush=True)
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
# It starts no Chrome, spawns no daemon and writes nothing outside a temporary box: every rule
# drives the real `main()` through stubs for the three things that need a machine (`Capture`,
# `capture_out`, `discard_stage`) plus one for `shoot`. What they do not stub out is the logic under
# test -- the loop, the handlers, the counting and the exit code are all the real ones. Nor do they
# stub the file machinery a refusal uses (`keep_refused`, `capture_refused`) once #220 gave it a
# verdict of its own: a rule that watched a fake being handed the bytes would pass with the fake
# doing nothing, so those rules move `cdp.RUNS` into their own box (`_refusal_box`) and read the
# directory back.
# --------------------------------------------------------------------------------------

# The count of rules registered below, and the reason the count is a number in the source rather
# than `len(ran)`: a `return` inserted in the middle of self_test() leaves both lists complete for
# whatever ran, so only a number written somewhere else can tell a full pass from a truncated one.
# 11 from #140, plus the seven #149 adds for the second viewport, plus the four #138 adds for the
# page that has to be photographed refused (the refusal itself, the click that produces it, and the
# two measurements that read the refused page back), plus two on the review round: how a script the
# page refused to run gets reported, and the needle that makes the refused artifact mean the refusal
# its filename names. Plus two from #211 for the conditions a run hands the recorder: the envs of the
# fixtures it photographed, and the `envs=` kwarg at the manifest step. Plus seven from #202 for the
# controls on the page: the census being asked for at both viewports, each of the three guards
# refusing the one defect it names and accepting the fixed page, the zeros in the transcript, and the
# measurement that was never taken. Plus two from #207 for the state behind the page: the declared
# reading of the dashboard, and the audit row count that is the fixture's. Plus one for the scripts
# condition arriving in the browser rather than only in the record -- the #213 finding was that a
# declared condition and an unconditional driver agree with each other in the file and disagree with
# the frame, and nothing that drives `pages.py` can see that. Plus ten from #204 for the navigation
# in the palette and marked: the census asked for on every page, the mark present and tied to the
# page's own href, a link still on the user-agent's blue or underline, the marked entry differing
# from its siblings on a computed property, the mark inside the header's box, and the five that
# refuse the defect each one names rather than photographing it. Plus two from #229's review of those
# same guards, both about the box instead of the ink: a mark on an entry 26 px tall, and a mark painted
# on a border the browser is not drawing. Plus six from #220 for what a refusal leaves behind: where
# its frames go and what says so, the fixture's own refused directory, the frame leaving the publish
# queue rather than being copied beside it, the once-only clear of a refused directory, the two places
# a kept frame could be mistaken for evidence, and a diagnostic that must not outrank the refusal it
# was called from.
# The two canaries are registered like every other rule and deliberately not counted: they exist to
# prove the helper is not hollow.
EXPECTED_RULES = 76


class _FakeSession:
    def __init__(self, log):
        self.log = log

    def publish(self):
        self.log.append("publish")

    def screenshot(self):
        """Bytes, so `cdp.capture_refused` has something to write on the refusal path.

        Not a PNG -- nothing in the self-test decodes these, and the property under test is where
        the bytes land and what says so, not their contents. Real bytes would mean shipping a
        picture in a source file to prove a path is written.
        """
        self.log.append("screenshot")
        return b"selftest frame, not a PNG on purpose"


class _FakeCap:
    """Stands in for cdp.Capture: a context manager whose session records being published.

    `fixture_name` is echoed back and nothing else, because that is all the real Capture can do
    either: the run directory's name is where it reads fixture.env from, so no fake can produce a
    directory that disagrees with the name it was asked for. An earlier version let `loaded` be set
    apart from it, and the rule built on that pair pinned a state production cannot reach.
    """

    def __init__(self, log, fixture_name="pages"):
        self.log = log
        self.session = _FakeSession(log)
        self.base = "http://127.0.0.1:0"
        self.name = fixture_name

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
def _stubbed(log, out=None):
    """The three things that need a machine, replaced for the duration of one rule.

    `capture_out` matters most and not for convenience: it parses sys.argv, so a rule that lets
    main() reach it gets a refusal for the `--self-test` flag sitting in argv. Two of the rules
    below were satisfied by exactly that and passed while the refusal they guard was deleted --
    measured. Stubbing it means the only refusal a rule can see is the one the rule is about.

    `out` is where the rule wants the artifact directory -- rules that read back what a refusal
    left on disk point it inside their own box -- and None is this package's own scratch path.
    What is NOT stubbed, on purpose, is `keep_refused` and `capture_refused`: the whole claim a
    refusal makes is about files, and a rule that fakes the file writer can only ever pin that the
    driver called it. With `cdp.RUNS` moved into the box (see `_refusal_box`) the real ones run and
    the rule reads the directory back.
    """
    out = out or "/tmp/opencode/selftest-pages"
    saved = (cdp.Capture, cdp.capture_out, cdp.discard_stage)
    cdp.Capture = lambda name="pages", *a, **k: _FakeCap(log, name)
    cdp.capture_out = lambda name, fixture_flag=False: (
        log.append("capture_out:%s" % fixture_flag), out)[1]
    cdp.discard_stage = lambda: log.append("discard")
    try:
        yield
    finally:
        cdp.Capture, cdp.capture_out, cdp.discard_stage = saved


@contextlib.contextmanager
def _refusal_box(out_name="pages"):
    """A scratch tree for one rule: `runs/`, the stage, and the keep pointer, all ours and all temp.

    `cdp.RUNS` is what `refused_dir` and `_stage_dir` both read, so moving it moves the whole
    capture tree and the real `keep_refused`/`capture_refused` can run without a browser and
    without writing inside the repository. Every global the keep machinery touches is restored --
    leaving `_REFUSED` set would hand the next rule a directory it never cleared, which is the one
    property that keeps a kept frame from being read as this run's.
    """
    box = tempfile.mkdtemp(prefix="plock-capture-selftest-")
    saved = (cdp.RUNS, cdp._STAGE, cdp._REFUSED)
    pending = list(cdp._PENDING)
    cdp.RUNS = box
    cdp._STAGE = op.join(box, "_stage-selftest")
    cdp._REFUSED = None
    cdp._PENDING.clear()
    out = op.join(box, out_name)
    try:
        yield box, out
    finally:
        cdp.RUNS, cdp._STAGE, cdp._REFUSED = saved
        cdp._PENDING[:] = pending
        shutil.rmtree(box, ignore_errors=True)


# What a clean run owes, per fixture, written out by hand.
#
# This is deliberately NOT derived from PAGES. `_expected_pass` below has to agree with the driver's
# own selection or every census rule becomes a tautology -- and it already was one: deleting a Page
# from PAGES shrinks the derived expectation and the rule passes, so the oldest self-test in this file
# could not notice a page silently stopping being photographed. Held literally, a dropped page breaks
# the equality in `census_matches_the_declaration` instead, and that rule is the one that fails.
CENSUS = {
    "pages": [("/", "dashboard"), ("/users", "users"), ("/profiles", "profiles"),
              ("/profiles", "profiles-refused"), ("/audit", "audit"), ("/settings", "settings")],
    "pages-noprof": [("/users", "users-no-profiles")],
}


def _expected_pass(mode="pages"):
    """The transcript a clean run has to produce: every page of that fixture, at every viewport."""
    return [(p.url, w) for w in WIDTHS for p in select_pages(mode)]


def _run_main(refuse=(), refuse_at=(), fixture_refuse=(), crash=None, sink=None, argv=(),
              out=None, box=None):
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
    finished its own bookkeeping. `out` relocates the artifact directory (see `_stubbed`) and `box`
    names the scratch tree it lives in; a rule that reads a refused run's leftovers off disk passes
    both, from its own `_refusal_box`, so the run's `runs/` and the rule's assertions are the same
    directory.
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
    def go(box_dir, out_dir):
        nonlocal rc
        # Where this run's scratch tree is, for a rule that reads what the refusal left on disk.
        # The log already carries `publish` and `discard`; the frames a refusal keeps are files,
        # and a rule cannot assert about a directory it has not been told the name of.
        log.append(("box", box_dir))
        with _stubbed(log, out_dir):
            saved_argv = list(sys.argv)
            sys.argv = ["pages.py"] + list(argv)
            try:
                with contextlib.redirect_stdout(console), contextlib.redirect_stderr(err):
                    # The manifest step is stubbed for the same reason the browser is: it asks git
                    # what it thinks, and a self-test must not inherit an answer from whatever
                    # repository happens to sit above the temp directory. `sink` hands the caller the
                    # log, so a rule can assert the step ran -- or did not.
                    # The fake recorder takes `envs` because the driver hands it the fixture envs (#211).
                    # A recorder that refuses the kwarg would be a TypeError on the success path, which is
                    # loud; the silent failure is the other direction -- main dropping `envs=` -- and that
                    # is what the self-test rule about this call site reads the source for.
                    rc = main(stub, lambda out, envs=(), shot=(): log.append("record"))
            except SystemExit as exc:
                rc = f"raised:{exc.code}"
            finally:
                sys.argv = saved_argv

    # Every run gets a scratch tree, not only the rules that ask for one: the refusal path writes
    # frames, and a self-test whose summary line promises "nothing written outside a temp box" has
    # to mean it for the rules that never thought about files. A rule that reads the leftovers back
    # opens its own `_refusal_box` and passes `box` so the two are the same directory.
    if box is None:
        with _refusal_box() as (box_dir, auto_out):
            go(box_dir, out or auto_out)
    else:
        go(box, out or op.join(box, "pages"))
    if sink is not None:
        sink.extend(log)
    return rc, entered, err.getvalue(), console.getvalue(), "publish" in log, log.count("discard")


_LAST_EXTRA = []

# The census values a page has to report for the #202 guards to pass, transcribed from a real run of
# this driver against the stylesheet being fixed (`/settings` at 360 px: page font `system-ui,
# sans-serif` at 16px, a 22ch field computing 196.062px, a wrapping label at `inline-flex`). They are
# the browser's numbers rather than this file's guesses, so a rule that passes on them is a rule that
# passes on what Chrome says.
PAGE_FONT = "system-ui, sans-serif"
PAGE_SIZE = "16px"
FIELD_MAX = "196.062px"

# The types the census marks as "not typed into", repeated from cdp.CONTROLS_JS's NOVALUE list. A
# rule below compares the two so this transcription cannot drift from the one in the browser.
NOVALUE = ("hidden", "radio", "checkbox", "submit", "button", "reset", "image", "range", "color",
           "select-one", "select-multiple")


def _control(name="reason", kind="text", **over):
    """One census row, defaulted to a control that satisfies every #202 guard.

    A rule that wants a defect overrides the one field it is about, so each rule states its own
    drift and nothing else. `tag` follows `kind` because a census row is identified by both.
    """
    item = {"tag": "button" if kind in ("submit", "button", "reset") else "input",
            "type": kind, "name": name, "family": PAGE_FONT, "size": PAGE_SIZE, "max": FIELD_MAX,
            "typed": kind not in NOVALUE, "toggle": kind in ("radio", "checkbox"), "label": ""}
    if item["toggle"]:
        item["label"] = "inline-flex"
    elif kind == "select-one":
        item["tag"] = "select"
    item.update(over)
    return item


_CLEAN_ITEMS = [_control("reason"), _control("pollSeconds", kind="number"),
                _control("profile", kind="select-one"), _control("enabled", kind="checkbox"),
                _control("", kind="submit")]

CLEAN_CENSUS = {"family": PAGE_FONT, "size": PAGE_SIZE, "count": len(_CLEAN_ITEMS),
                "items": _CLEAN_ITEMS}

# A page that passes every other guard in shoot() at the artifact viewport, so a rule about the
# census can only be refused BY the census.
_BASE_PAGE = {"scroll": 360, "inner": 360, "height": 700, "tables": 1, "cells": 12,
              "forms": 2, "fields": 4, "longest": 14}

# --------------------------------------------------------------------------- #
# The navigation, transcribed rather than invented (#204).
#
# Same discipline as CLEAN_CENSUS: these are the strings a real Chrome reported on the dashboard
# at 360 px on 6 Nov, off the stylesheet this change writes -- the marked entry `rgb(26, 28, 32)`
# at weight 600 with a `rgb(216, 219, 226)` rule under it, its siblings `rgb(91, 98, 112)` at 500
# with nothing under them, every box 40 px tall because the padding is on all five and not only on
# the marked one. A fixture built from guesses would let a guard pass on a page no browser can
# produce, which is the failure mode every rule in this file exists to close.
# --------------------------------------------------------------------------- #
NAV_INK = "rgb(26, 28, 32)"        # app.css --ink, the marked entry and the site's name
NAV_MUTED = "rgb(91, 98, 112)"     # app.css --muted, every entry that is somewhere else
NAV_MARK = "rgb(216, 219, 226)"    # app.css --line, the rule under the current entry
NAV_CLEAR = "rgba(0, 0, 0, 0)"
CUR = "page"                     # the only value aria-current takes that means "this page"
NAV_HEIGHT = 40                    # 23.6 px of text, 14.4 px of padding, 2 px of reserved border
#                                   # -- and no longer only a transcription: `cdp.NAV_MIN_TARGET` is
#                                   # the floor `require_nav_marked` refuses under (#229), so this
#                                   # number is what the page measures AND what it has to be


def _nav_link(text, href, **over):
    """One navigation entry, defaulted to a link the stylesheet has already claimed."""
    item = {"text": text, "href": href, "cur": None, "color": NAV_MUTED, "weight": "500",
            "dec": "none", "bw": "2px", "bc": NAV_CLEAR, "bg": NAV_CLEAR,
            "x": 0, "y": 72, "w": 60, "h": NAV_HEIGHT, "vis": True}
    item.update(over)
    return item


def _nav_current(text, href, **over):
    """The same entry as the stylesheet renders the one for the page you are on."""
    return _nav_link(text, href, cur=CUR, color=NAV_INK, weight="600", bc=NAV_MARK, **over)


# The dashboard's own row at 360 px: four entries on the first line, the fifth with the sign-out
# form on the second, and the header's box spanning both -- measured `x=24 y=24 w=312 h=131`, with
# the nav itself at `y=72 h=82`.
NAV_HEADER = {"x": 24, "y": 24, "w": 312, "h": 131}
# The five entries as the shell writes them, at the boxes Chrome reported -- none of them marked.
# `clean_nav` puts the mark on whichever entry the page being faked declares as its own.
_NAV_ITEMS = [_nav_link("Status", "/", x=24),
              _nav_link("Accounts", "/users", x=97, w=78),
              _nav_link("Audit", "/audit", x=188, w=49),
              _nav_link("Profiles", "/profiles", x=250, w=66),
              _nav_link("Settings", "/settings", x=24, y=114, w=70)]


def clean_nav(href):
    """A navigation row, in the measured geometry, with `href` as the current entry.

    Why a function and not a constant: a canned "the current entry is Status" would make every
    rule that runs `shoot()` against a page other than the dashboard fail on a #204 guard it knows
    nothing about -- and the tempting repair for that failure is the one this file refuses
    everywhere, which is to weaken the guard. The geometry is transcribed once, from the dashboard;
    which of the five is marked is a fact about the page a rule is faking, and `_fake_browser`
    takes it from the spec the way the real driver takes it from the fixture.

    The href has to be one of the five. Asserting that here rather than defaulting to "mark nothing"
    is the same reasoning as the `Page(nav=...)` field being keyword-only and undeclarable-by-
    omission: a fixture that names a section the shell does not offer is a finding, not a default.
    """
    items = [dict(i) for i in _NAV_ITEMS]
    hit = False
    for item in items:
        cur = item["href"] == href
        hit = hit or cur
        item.update(cur=CUR if cur else None,
                    color=NAV_INK if cur else NAV_MUTED,
                    weight="600" if cur else "500",
                    bc=NAV_MARK if cur else NAV_CLEAR)
    if not hit:
        raise AssertionError(f"clean_nav asked for {href!r} and the shell offers "
                             f"{[i['href'] for i in _NAV_ITEMS]}")
    return [{"display": "flex", "overflow": False, "items": items}]


def _nav(*items, **over):
    """A measurement whose navigation is exactly the entries one rule is talking about.

    The mirror of `_census`: every other key is the ordinary passing page, so a refusal produced by
    a #204 rule can only have been produced by the navigation.
    """
    rows = over.pop("rows", None)
    nav = rows if rows is not None else [{"display": "flex", "overflow": False,
                                          "items": list(items)}]
    base = dict(_BASE_PAGE, nav=nav, header=dict(NAV_HEADER))
    base.update(over)
    return base


@contextlib.contextmanager
def _fake_browser(got, nav="/"):
    """The CDP reads, replaced, so the real `shoot()` can be called with no Chrome.

    Only the reads are faked. `shoot` itself -- the order it sets the viewport in, the checks it
    runs, whether it stages bytes -- is the code under test, and every one of those is invisible
    to the rules that drive `main()` through a stubbed `shoot_fn`.
    """
    saved = (cdp.measure, cdp.require_records, cdp.require_labelled_cells, cdp.require_form)
    # The `extra` string is recorded because a guard that reads a census key the measurement never
    # asked for is a guard that always fails, and from outside this fake the two are identical.
    #
    # The control census is DEFAULTED here, not added to every rule's canned `got`, for one reason:
    # the three #202 guards are deliberately left unstubbed -- stubbing a guard is how a guard stops
    # existing -- so they run for real against whatever this hands back. A rule that wants a defect
    # passes its own "controls" key and wins; a rule about something else gets a census that passes,
    # transcribed from a measured page rather than invented. `the_control_census_is_asked_for` is
    # what pins that the real shoot() actually asks the browser for it.
    cdp.measure = lambda session, extra="": (
        _LAST_EXTRA.append(extra),
        dict({"controls": CLEAN_CENSUS, "nav": clean_nav(nav), "header": dict(NAV_HEADER),
              "states": cdp.CLEAN_STATES},
             **got))[1]
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

    def value(self, js):
        """The three scripts `refusal_form` runs, answered the way a real page answers.

        Each script gets its own reply rather than one canned string for all of them: the driver's
        whole contract is that it probes the form, fills it, and then clicks it, and a session that
        answered every script identically would let step two and step three go unrun while the rule
        still saw a refusal-free `shoot()`. An unrecognised script fails loudly for the same
        reason. The action is the URL this session reports, because `shoot()` pins the landing
        against it.
        """
        self.log.append(("value", js[:20]))
        if "action$=" in js:
            return json.dumps({"action": self._where, "button": True, "csrf": True})
        if "'filled'" in js:
            return "filled"
        if "'clicked'" in js:
            return "clicked"
        raise AssertionError("unexpected script from the refusal driver: " + js[:70])


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
    spec = Page("/users", "users", True, nav="/users") if spec is None else spec
    with _fake_browser(got, spec.nav), contextlib.redirect_stdout(out or io.StringIO()):
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
            # Named with the rule's own `why`. Without it a crashing rule contributes a line that
            # says `FileNotFoundError: [Errno 2] No such file or directory: '<tmp>/pages-refused'`
            # and nothing about which claim broke -- seen on the mutation that hollowed
            # `keep_refused`, where four rules did exactly that and finding them meant bisecting.
            problems.append(f"{why}: {type(exc).__name__}: {exc}")
        (ran_to if ran_to is not None else ran).append((why, len(problems)))
        (bad_to if bad_to is not None else bad).extend(problems)

    # From the literal CENSUS, not from PAGES. Every count below is a claim about how many pages a
    # clean default-mode run photographed, and deriving it from the list the driver iterates makes
    # each one unfalsifiable: drop a Page from PAGES and the expectation shrinks with it.
    urls = [u for u, _ in CENSUS["pages"]]

    # Both rules below take their spec from the table rather than building one, for the reason
    # #140 established: a rule that invented its own Page would keep passing the day somebody
    # drops `refuse=` off PAGES, and the artifact would stop being taken while its guard stayed
    # green.
    refused_spec = next((p for p in PAGES if p.refuse), None)

    def refusal_photograph_proves_a_refusal(problems):
        """A page with no sentence on it is not a refusal, and its PNG is not published."""
        if refused_spec is None:
            problems.append("no page in PAGES carries a Refusal, so the #138 artifact is not being "
                            "taken and the rules that police it are checking nothing")
            return
        keep = refused_spec.refuse.keep_value
        base = {"inner": 360, "scroll": 300, "height": 900, "tables": 1, "cells": 4,
                "forms": 3, "fields": 9, "longest": 12,
                "refusal": "the clock must be written as HH:MM", "kept": keep}
        # The control first. A guard that refused every page would pass the two arms below, and
        # "the refused page is refused" is not a property anyone wants pinned.
        log, refusal = _run_shoot(dict(base), ARTIFACT_WIDTH, spec=refused_spec)
        if refusal is not None:
            problems.append(f"a page that shows a sentence and keeps {keep!r} was refused: "
                            f"{refusal}")
        if not any(k == "png" for k, _ in log):
            problems.append("the refused-write page fitted and no PNG was staged")
        for key in ("refusal", "kept"):
            got = dict(base)
            got[key] = ""
            log, refusal = _run_shoot(got, ARTIFACT_WIDTH, spec=refused_spec)
            if refusal is None:
                problems.append(f"{key}='' was photographed without a refusal, so a PNG named for "
                                "a refusal would be published over a page that is not one")
            if any(k == "png" for k, _ in log):
                problems.append(f"{key}='' still staged bytes after refusing")

    def refusal_form_is_clicked_not_reimplemented(problems):
        """The driver fills the shipped form and presses its own button.

        Three ways that click stops being a click, each of which still produces a 422 for a reason
        unrelated to the field under test: no form, no button, no CSRF token. Two of the three look
        like a successful refusal in the transcript, because a refusal is what gets printed either
        way, so the driver refuses them rather than photographing whatever comes back.
        """
        if refused_spec is None:
            problems.append("no page carries a Refusal, so there is no form to click")
            return

        class _Form:
            """A session that answers each script with the reply that script would get."""

            def __init__(self, probe):
                self.probe = probe
                self.seen = []

            def value(self, js):
                self.seen.append(js)
                if "action$=" in js:
                    return self.probe
                if "'filled'" in js:
                    return "filled"
                if "'clicked'" in js:
                    return "clicked"
                raise AssertionError("unexpected script: " + js[:70])

        # The control: a form that is really there gets filled, clicked, and reports the action the
        # page carries, because that action is what `shoot()` pins the landing against.
        good = '{"action": "/profiles/school/rules", "button": true, "csrf": true}'
        session = _Form(good)
        with contextlib.redirect_stdout(io.StringIO()):
            action = refusal_form(session, refused_spec)
        if action != "/profiles/school/rules":
            problems.append(f"a present form reported action {action!r}, want the action on the "
                            "page: the click has to be aimed at the form that was filled")
        if not any("'clicked'" in js for js in session.seen):
            problems.append("the driver never clicked a button")
        for probe, want in (
            ('{"missing": true}', "no form whose action ends in"),
            ('{"action": "/profiles/school/rules", "button": false, "csrf": true}',
             "no submit button"),
            ('{"action": "/profiles/school/rules", "button": true, "csrf": false}',
             "no CSRF token"),
        ):
            session = _Form(probe)
            refusal = None
            with contextlib.redirect_stdout(io.StringIO()):
                try:
                    refusal_form(session, refused_spec)
                except SystemExit as exc:
                    refusal = str(exc)
            if refusal is None or want not in refusal:
                problems.append(f"{probe} produced {refusal!r}, want a refusal naming {want!r}")

    def refused_run(**kw):
        """Run the driver in a scratch tree that outlives the call, and hand back what it left.

        `_run_main` deletes its own box on the way out, which is right for a rule that reads the
        transcript and useless for one that reads the frames a refusal left -- they would be gone
        with it. So a rule about files holds the box open itself and this closes it after the
        reading is done. Returns (rc, entered, stderr, published, discarded, kept, note); `kept` is
        None when there is no refused directory at all, and `note` is None when nothing was written
        beside the frames.

        Both come off the disk, from the real `cdp.keep_refused`/`capture_refused`. Neither is
        counted through a stub, because every claim these rules makes is about files -- a rule that
        watched a fake being called would still pass with `keep_refused` hollowed to `return []`.
        """
        with _refusal_box() as (box, out):
            rec = []
            rc, entered, err, _out, published, discarded = _run_main(box=box, out=out,
                                                                     sink=rec, **kw)
            d = op.join(box, "pages-refused")
            kept = (sorted(f for f in os.listdir(d) if f.endswith(".png"))
                    if op.isdir(d) else None)
            note = None
            if op.isfile(op.join(d, cdp.REFUSED_NOTE)):
                with open(op.join(d, cdp.REFUSED_NOTE), encoding="utf-8") as fh:
                    note = fh.read()
            return rc, entered, err, published, discarded, kept, note

    def all_refuse(problems):
        rc, entered, err, published, discarded, kept, _note = refused_run(refuse=urls)
        want = _expected_pass()
        if len(entered) != len(want):
            problems.append(f"a refusal on every page measured {len(entered)} of {len(want)} "
                            f"({entered}); the loop has to finish for the count it prints to be "
                            "true, at every viewport")
        if rc != 1:
            problems.append(f"every page refused -> main() returned {rc!r}, want 1")
        if discarded:
            problems.append(f"every page refused -> discard_stage() ran {discarded} times. A "
                            "refusal keeps the frames it measured now (#220); destroying them is "
                            "what left an operator with a sentence about a page and no way to "
                            "look at it")
        if published:
            problems.append("every page refused -> the stage was published anyway")
        if kept is None:
            problems.append("every page refused -> no refused directory at all, so the run "
                            "destroyed the twelve frames it had measured")
        elif len(kept) != 2 * len(urls):
            problems.append(f"every page refused at both viewports kept {len(kept)} frame(s) "
                            f"{kept}, want {2 * len(urls)}: one per page per viewport")
        if f"0/{len(urls)}" not in err:
            problems.append(f"the summary does not report 0/{len(urls)}: {err.strip()[-160:]!r}")

    def one_refuses(problems):
        victim = urls[-1]
        rc, entered, err, published, discarded, kept, note = refused_run(refuse=[victim])
        if entered != _expected_pass():
            problems.append(f"one refusal stopped the loop at {entered}, "
                            f"want {_expected_pass()}")
        if rc != 1:
            problems.append(f"one page refused -> main() returned {rc!r}, want 1")
        if victim not in err:
            problems.append(f"the summary does not name the refused page {victim!r}")
        if f"{len(urls) - 1}/{len(urls)}" not in err:
            problems.append(f"the summary does not report {len(urls) - 1}/{len(urls)}")
        if discarded or published:
            problems.append(f"one refusal -> discarded={discarded} published={published}, want "
                            "0 and False: the frames the other pages measured are kept, neither "
                            "destroyed nor published")
        # Only the refusing page is photographed. The five that measured fine have nothing to show
        # that they did not already stage, and a frame of a page that passed a refused run is the
        # exact ambiguity this folder refuses hardest -- it would also make `kept` a list a reader
        # cannot tell apart from the artifact set.
        want = sorted(f"{a}@{w}.png" for u, a in CENSUS["pages"] if u == victim for w in WIDTHS)
        if kept != want:
            problems.append(f"refusing {victim} kept {kept}, want {want} -- the page that refused, "
                            "at each viewport that refused it")
        if note is None or victim not in note:
            problems.append("nothing was written beside the frames naming what was refused, so "
                            "the bytes travel without the sentence that makes them mean anything "
                            "(#211's objection, applied to a frame nobody certified)")

    def none_refuses(problems):
        rc, entered, _err, published, discarded, kept, _note = refused_run()
        if rc != 0:
            problems.append(f"a clean run -> main() returned {rc!r}, want 0")
        if not published:
            problems.append("a clean run did not publish the stage")
        if discarded:
            problems.append("a clean run discarded the stage")
        if entered != _expected_pass():
            problems.append(f"a clean run measured {entered}, want {_expected_pass()}")
        if kept is not None:
            problems.append(f"a clean run left a refused directory ({kept}) behind, so the next "
                            "refusal would open a directory holding frames that are not its own")

    def crash_is_a_refusal(problems):
        # A bug inside shoot() must not cost the other four pages either: `except Exception` is
        # the only thing standing between a TypeError on page 1 and a run that measured nothing.
        rc, entered, err, published, discarded, kept, _note = refused_run(
            crash=TypeError("a real bug"))
        if len(entered) != len(_expected_pass()):
            problems.append(f"an exception on every page stopped the loop at {len(entered)}, "
                            f"want {len(_expected_pass())}")
        if rc != 1:
            problems.append(f"an exception -> rc={rc!r}, want 1")
        if "TypeError" not in err:
            problems.append("the refusal does not name the exception type")
        if discarded or not kept:
            problems.append(f"an exception -> discarded={discarded} kept={kept}: a bug is a "
                            "refusal, and a refusal keeps what it measured (#220)")

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
            PAGES[:] = [Page("/audit", "audit", False, nav="/audit", stress=True)]
            msg = refusal_from_main(problems)
            if msg and "has_rows" not in msg:
                problems.append(f"the refusal does not say how to fix it: {msg}")
        finally:
            PAGES[:] = saved

    def stress_must_be_carried(problems):
        saved = list(PAGES)
        try:
            PAGES[:] = [Page("/", "dashboard", True, nav="/")]
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
        rc, _entered, err, published, discarded, kept, _note = refused_run(
            refuse_at=[f"{victim}@{DESKTOP_WIDTH}"])
        if rc != 1:
            problems.append(f"a refusal at {DESKTOP_WIDTH} px only -> main() returned {rc!r}, "
                            "want 1")
        if published:
            problems.append(f"a refusal at {DESKTOP_WIDTH} px only published the stage anyway")
        if discarded:
            problems.append(f"a refusal at {DESKTOP_WIDTH} px only discarded the stage "
                            f"{discarded} times, want 0 -- it is kept (#220)")
        # The frame has to be the one that shows the defect. Photographing `/audit` at 360 px,
        # where it fits, would hand the operator a picture of the page that is fine while the
        # 1471 px document stayed unphotographed -- so the name carries the viewport.
        if kept != [f"audit@{DESKTOP_WIDTH}.png"]:
            problems.append(f"a refusal at {DESKTOP_WIDTH} px only kept {kept}, want "
                            f"[{DESKTOP_WIDTH!r} px frame of /audit] -- the phone frame of the "
                            "same page fits and proves nothing about the desktop")
        # Both viewports have to appear, with the right answer for each. Reading only "did it
        # fail" would let a driver refuse the phone pass and report the desktop as fitting --
        # the same sentence, inverted, and still a lie about the desktop.
        if f"{len(CENSUS["pages"])}/{len(CENSUS["pages"])} pages fit at {ARTIFACT_WIDTH} px" not in err:
            problems.append(f"the {ARTIFACT_WIDTH} px viewport is not reported as fitting: "
                            f"{err.strip()[-200:]!r}")
        if f"{len(CENSUS["pages"]) - 1}/{len(CENSUS["pages"])} pages fit at {DESKTOP_WIDTH} px" not in err:
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
            if f"{len(CENSUS["pages"])}/{len(CENSUS["pages"])} pages fit at {width} px" not in out_text:
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
        # Written from len(PAGES) rather than from the numbers this rule was first written against:
        # a hardcoded 5/5 silently stops being a claim the day a sixth page joins PAGES, and this is
        # the rule whose whole job is to notice a count that means less than it looks like.
        if f"{len(CENSUS["pages"])}/{len(CENSUS["pages"])} pages fit at {DESKTOP_WIDTH} px" in err:
            problems.append(f"the summary still certifies every page at {DESKTOP_WIDTH} px: "
                            f"{err!r}")
        if f"{len(CENSUS["pages"]) - 1}/{len(CENSUS["pages"])} pages fit at {DESKTOP_WIDTH} px" not in err:
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
                                   spec=Page(victim, "audit", True, nav=victim, stress=True),
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
        if f"{len(CENSUS["pages"])}/{len(CENSUS["pages"])} pages fit" not in out or rc != 0 or not published:
            problems.append(f"the control run stopped counting every page: rc={rc!r} {out!r}")

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
        spec = Page("/audit", "audit", True, nav="/audit", stress=True)
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

    def kept_box_read_from_the_form_that_posted(problems):
        """The kept-value selector names the form that answered, not the suffix that found it.

        Measured on the real page: splicing the declared suffix asks the browser for
        `form[action="/rules"]`, which is no form at all, so `kept` comes back null on a page whose
        box still held the typed value -- and `require_refusal` then refuses the capture for
        discarding a value the product had just put back. The sentence half of the same measurement
        passes either way, because `#refusal` is global, which is what made the pair read like a
        product failure rather than a selector one.
        """
        if refused_spec is None:
            problems.append("no page carries a Refusal, so there is no kept box to read")
            return
        resolved = "/profiles/school/rules"
        splice = refusal_extra(resolved, "end")
        if 'form[action="%s"]' % resolved not in splice:
            problems.append("the kept-value selector does not name the form that posted, so it "
                            "reads a box that is not on the page")
        control = "," + (REFUSAL_JS % (refused_spec.refuse.action, refused_spec.refuse.keep))
        if control == splice:
            problems.append("nothing distinguishes the declared suffix from the resolved action, "
                            "so this rule cannot tell the two splices apart")
        if 'form[action="%s"]' % resolved in control:
            problems.append("the control is not a control: the declared suffix already resolves to "
                            "the posted form")

    def fill_script_slices_no_node_list(problems):
        """`NodeList` is not an array and does not slice, so the checkbox fill has to copy it.

        The script is handed to the browser verbatim, which makes this string the behaviour rather
        than a proxy for it. The failure it pins is a misdiagnosis rather than a crash: the bad
        parse was reported to the operator as "the refusal form vanished", about a form that was
        there, because the CDP error field was dropped before anyone could read it.

        Honest about its own reach: without a browser this can only be a text pin, so it cannot see
        the fill hollowed out in place. `.forEach(i => i.checked = true)` becoming
        `.forEach(i => i.value = true)` keeps every substring checked here and ticks nothing; the
        sentence needle in `Refusal.about` is what turns that hollowing red on a real run.
        """
        script = fill_script("/profiles/school/rules", {"end": "25:99"}, {"days": 2})
        if "querySelectorAll('[name=\"days\"]')[:2]" in script:
            problems.append("the fill script slices a NodeList, which is undefined in a browser")
        if "Array.from(f.querySelectorAll('[name=\"days\"]')).slice(0, 2)" not in script:
            problems.append("the checkbox fill no longer copies the NodeList before slicing it, so "
                            "it throws and the driver blames the page")
        if "catch" not in script:
            problems.append("the fill script has no catch, so a script that throws is reported to "
                            "the operator as a form that is missing")

    def a_script_that_never_parsed_is_not_a_vanished_form(problems):
        """A script with a syntax error never runs, so the `try` written inside it cannot report it.

        Driven, not text-pinned: the first version of this rule grepped cdp.py for the word
        `exceptionDetails`, and stayed green with the code that reads it deleted, because the word
        is in the docstring above that code. So the real `Session.value` is called here with the
        two replies Chrome actually sends -- the one carrying a parse error, and a normal one --
        and what it returns is what gets asserted. Dropping the field is what turned a bad fill
        script into "the refusal form vanished", and the misreport cost more time than the bug.
        """

        def evaluate(reply):
            class _Transport:
                def call(self, method, **params):
                    return reply

            _Transport.value = cdp.Session.value
            return _Transport().value("document.querySelectorAll('x')[:2]")

        thrown = evaluate({"exceptionDetails": {
            "text": "Uncaught", "lineNumber": 0, "columnNumber": 43,
            "exception": {"type": "object", "subtype": "error", "className": "SyntaxError",
                          "description": "SyntaxError: Unexpected token ':'"}},
            "result": {"type": "object", "subtype": "error",
                       "className": "SyntaxError"}})
        if not (isinstance(thrown, str) and thrown.startswith("threw")):
            problems.append(f"a script the page refused to run answered {thrown!r}; the callers "
                            'test startswith("threw"), so anything else is read as a form that '
                            "is not there")
        elif "SyntaxError" not in thrown:
            problems.append("a refused script is reported as a shape rather than a reason: the "
                            f"reply was {thrown!r}, with no SyntaxError in it")
        if evaluate({"result": {"type": "string", "value": "filled"}}) != "filled":
            problems.append("a script that ran no longer hands back its value, so every caller "
                            "that compares against a string fails on a page that worked")

    def the_refused_artifact_is_refused_about_its_own_refusal(problems):
        """A 422 about the wrong field photographs exactly like a 422 about the right one.

        Behavioural rather than a text pin, because `require_refusal` is pure and can be driven
        here. The hole is a fill that silently ticks nothing: the route then refuses the days, the
        echo still puts `25:99` back because it is the same closure, and both measurements the
        driver already makes pass on a photograph of a different bug than the filename claims.
        """
        if refused_spec is None:
            problems.append("no page carries a Refusal, so no artifact is named for a refusal")
            return
        needle = refused_spec.refuse.about
        if not needle:
            problems.append("the refused artifact declares no needle, so any 422 on this page "
                            "publishes a photograph named for the clock")
            return
        clock = {"refusal": '"25:99" is not HH:MM on a 24-hour clock', "kept": "25:99"}
        other = {"refusal": "pick at least one day for the window", "kept": "25:99"}
        if needle in other["refusal"]:
            problems.append("the control sentence already contains the needle, so this rule "
                            "cannot tell a clock refusal from any other")
            return
        for got, want_refused in ((clock, False), (other, True)):
            try:
                require_refusal(dict(got), refused_spec, "profiles-refused")
                said = None
            except SystemExit as exc:
                said = str(exc)
            if (said is not None) != want_refused:
                problems.append(f"require_refusal {'refused' if said else 'accepted'} "
                                f"{got['refusal']!r}; the needle {needle!r} is the only thing that "
                                "makes the filename mean the refusal it is named for")

    def census_matches_the_declaration(problems):
        """Every artifact is photographed by exactly one fixture, and the literal list says which.

        This is the rule that makes the counts elsewhere real. `urls` and `_expected_pass` both come
        from here now, so without a literal on the other side of them, deleting a Page from PAGES
        would shrink the expectation and every census rule would keep passing -- including the one
        whose whole job is noticing a page stopped being photographed.
        """
        declared = {mode: [(q.url, q.artifact) for q in PAGES if q.fixture == mode]
                    for mode in CENSUS}
        for mode, want in CENSUS.items():
            got = declared.get(mode, [])
            if got != want:
                problems.append(f"fixture {mode}: CENSUS says {want} and PAGES declares {got}. "
                                "One of them is stale, and the counts in this file are derived from "
                                "CENSUS so a stale entry moves the numbers with the mistake")
        seen = [a for rows in CENSUS.values() for _, a in rows]
        dupes = {a for a in seen if seen.count(a) > 1}
        if dupes:
            problems.append(f"artifacts photographed by more than one fixture: {sorted(dupes)}")
        missing = [p.artifact for p in PAGES if p.artifact not in seen]
        if missing:
            problems.append(f"pages declared but in no fixture's CENSUS: {missing} -- they would "
                            "be measured and then excluded from every count this file prints")

    def a_fixture_mode_selects_its_own_pages(problems):
        for mode, want in (("pages", CENSUS["pages"]), ("pages-noprof", CENSUS["pages-noprof"])):
            log = []
            rc, entered, _err, _out, published, _disc = _run_main(
                argv=["--fixture", mode], sink=log)
            # argparse owns the whole command line, so a driver that stops declaring the flag makes
            # the real CLI die on "unrecognized arguments" while this fake -- which never parses --
            # keeps measuring every page. The flag has to be asserted, not assumed.
            if "capture_out:True" not in log:
                problems.append(f"--fixture {mode}: the driver did not ask capture_out to accept "
                                f"--fixture (log: {log}); the real CLI would reject the flag")
            if rc != 0:
                problems.append(f"--fixture {mode} returned {rc!r}, want 0")
            want_pass = [(u, w) for w in WIDTHS for u, _ in want]
            if entered != want_pass:
                problems.append(f"--fixture {mode} measured {entered}, want {want_pass}. A mode "
                                "that photographs the pages of another fixture writes artifacts "
                                "whose names promise a document that was never served")

    def an_unknown_fixture_refuses(problems):
        rc, entered, err, _out, published, _disc = _run_main(argv=["--fixture", "pages-nosuch"])
        # A declaration guard, so it raises SystemExit with the message rather than reporting a
        # per-page refusal and returning 1 -- the message lands in the exit code, not on stderr, and
        # that is the right shape: nothing has started yet, so there is no run to summarise.
        if not str(rc).startswith("raised:"):
            problems.append(f"--fixture pages-nosuch returned {rc!r}, want a refusal")
        if entered:
            problems.append(f"an unknown fixture still measured {entered}")
        if published:
            problems.append("an unknown fixture published a stage")
        if "pages-nosuch" not in str(rc):
            problems.append(f"the refusal does not name the fixture it refused: {str(rc)[:120]!r}")

    def capture_out_returns_one_directory(problems):
        """`--out` resolves inside the repo and comes back as ONE string, through the real function.

        Review round 4 asserted the opposite -- that `capture_out` returns two values while every
        caller unpacks one, so any `--out` invocation raises TypeError. It does not: the function has
        two returns, `op.join(RUNS, default_name)` and `target`, both single. The argument was still
        worth ending, because the file's own README documents the `--out docs/screenshots/360
        --overwrite` flow and NOTHING in this self-test had ever called the real `capture_out`. A claim
        about an unexercised call can be made again next week, so this calls it.
        """
        saved = list(sys.argv)
        try:
            sys.argv = ["pages.py", "--out", "docs/screenshots/360", "--overwrite"]
            got = cdp.capture_out("pages", fixture_flag=True)
        except BaseException as exc:  # a refusal here is a finding, not a pass
            problems.append(f"the real capture_out raised {type(exc).__name__} on the documented "
                            f"`--out docs/screenshots/360 --overwrite`: {exc}")
            return
        finally:
            sys.argv = saved
        if not isinstance(got, str):
            problems.append(f"capture_out returned {type(got).__name__}, and every caller unpacks one "
                            "value from it -- lock.py:162, testlock.py:147, pages.py:669")
            return
        if not got.endswith("docs/screenshots/360"):
            problems.append(f"capture_out sent the capture to {got!r} instead of the directory "
                            "--out named")
        # The flag is part of the contract: --out says where, --overwrite says the files there are
        # the ones you mean to replace. Without the second, a bare --out would replace committed
        # evidence, which is the harm the two-flag rule exists to gate.
        sys.argv = ["pages.py", "--out", "docs/screenshots/360"]
        try:
            cdp.capture_out("pages", fixture_flag=True)
            problems.append("--out without --overwrite was accepted, so one flag would replace "
                            "committed artifacts")
        except SystemExit as exc:
            if "overwrite" not in str(exc).lower():
                problems.append(f"--out without --overwrite refused for the wrong reason: {exc}")
        finally:
            sys.argv = saved
        # And the flag it does not give out: a driver that never asked must not accept --fixture.
        sys.argv = ["pages.py", "--fixture", "pages"]
        err = io.StringIO()
        try:
            with contextlib.redirect_stderr(err):
                cdp.capture_out("lock")
            problems.append("capture_out accepted --fixture for a driver that never passed "
                            "fixture_flag, so lock.py would photograph an unset-up fixture")
        except SystemExit as exc:
            # The refusal is argparse's, so the sentence is on stderr and the exception carries only
            # the exit code. Asserting on str(exc) would be asserting on "2"; the text is what names
            # the flag, so the text is what gets read. Measured: str(exc) == "2", stderr carries
            # "unrecognized arguments: --fixture pages".
            if "fixture" not in err.getvalue() or exc.code == 0:
                problems.append(f"--fixture refused for the wrong reason (code {exc.code}, stderr "
                                f"{err.getvalue().strip()!r})")
        finally:
            sys.argv = saved

    def a_needle_page_naming_the_default_fixture_is_refused(problems):
        """The invariant, asserted by constructing the page that breaks it.

        Two reviews asked for this in the same words, and neither could know the mechanism: `Page` is
        not a dataclass, so there is no `__post_init__` to hold the rule even if one were written. The
        only place a plain class can refuse is its `__init__`, which is where it now lives -- and this
        rule is the difference between that being a comment and being a guard, because a `raise` nobody
        constructs is indistinguishable from no `raise` at all.
        """
        try:
            Page("/audit", "audit-no-events", True, nav="/audit",
                 needles=("Nothing has been recorded yet",))
        except ValueError as exc:
            if "audit-no-events" not in str(exc) or DEFAULT_FIXTURE not in str(exc):
                problems.append(f"the refusal does not name the artifact or the fixture: {exc}")
            return
        except Exception as exc:  # a different exception is a different bug, not a pass
            problems.append(f"constructing the bad page raised {type(exc).__name__}, want ValueError")
            return
        problems.append("a needles-bearing Page declaring the default fixture was constructed: it "
                        "would photograph the populated branch and refuse on its own needle only "
                        "after a daemon and a browser are up, and nothing in this self-test would "
                        "notice at all")

    def the_declared_state_is_the_thing_photographed(problems):
        """#207: a page in another state is refused, and the same check passes on the real one.

        The discrimination is the whole rule. Both readings are handed to the same `spec` -- the
        declared `locked out until 00:00` and the `until 05:00` that #209's absent fix produces from
        the same fixture -- so this cannot be satisfied by a check that refuses everything or by one
        that refuses nothing. The third case is the one that used to be silently fine: a row that has
        moved or vanished, which is what an undeclared state looks like when the fixture changes
        under the declaration.
        """
        spec = next(p for p in PAGES if p.url == "/" and p.state)
        good = {"rows": [["casey", ["locked out until 00:00", "Every day, every hour",
                                    "no desktop session"]],
                         ["alex", ["nothing scheduled", "x11 · seat0 · locked"]]]}
        try:
            require_declared_state(good, spec, "/")
        except SystemExit as exc:
            problems.append(f"the declared state refused the page it declares: {exc}")
        drifted = {"rows": [["casey", ["locked out until 05:00", "Every day, every hour",
                                       "no desktop session"]],
                            ["alex", ["nothing scheduled", "x11 · seat0 · locked"]]]}
        try:
            require_declared_state(drifted, spec, "/")
            problems.append("a dashboard saying `until 05:00` for a rule that says midnight passed "
                            "the declared state -- which is #208's defect arriving as an artifact")
        except SystemExit as exc:
            if "05:00" not in str(exc) or "00:00" not in str(exc):
                problems.append(f"the state refusal names neither the cell it saw nor the one "
                                f"declared: {exc}")
        # #220's arm: the clock stays right and the *rule* moves. `EndsAt` is a max over the open
        # rules and the named rule is the one that produced that max, so a second enabled rule that
        # releases later than the all-day one re-decides the Why cell at one hour of the day and not
        # another -- three hours a day, on the very fixture this file photographs. A declaration that
        # reads the clock alone passes on this row, and the artifact would blame a rule that the frame
        # taken at 18:44 does not mention.
        renamed = {"rows": [["casey", ["locked out until 00:00",
                                       "Weeknight curfew including the weekend",
                                       "no desktop session"]],
                            ["alex", ["nothing scheduled", "x11 · seat0 · locked"]]]}
        try:
            require_declared_state(renamed, spec, "/")
            problems.append("a casey row that names the curfew while the clock still says 00:00 "
                            "passed the declared state, which is #220's three hours arriving as an "
                            "artifact nothing refuses")
        except SystemExit as exc:
            if "Weeknight curfew" not in str(exc) or "Every day" not in str(exc):
                problems.append(f"the named-rule refusal names neither the rule it saw nor the one "
                                f"declared: {exc}")
        try:
            require_declared_state({"rows": [["alex", ["nothing scheduled"]]]}, spec, "/")
            problems.append("a dashboard with no casey row passed the declared state")
        except SystemExit as exc:
            if "casey" not in str(exc):
                problems.append(f"the missing-row refusal does not name the row it wanted: {exc}")
        try:
            require_declared_state({"rows": []}, spec, "/")
            problems.append("an empty row census passed the declared state; an empty table fits at "
                            "any width, which is exactly the case that must not be quiet")
        except SystemExit as exc:
            if "no row census" not in str(exc):
                problems.append(f"the empty-census refusal says something else: {exc}")
        # And the declaration is not vacuous: every pair has to be a pair, or a typo in PAGES turns
        # the check off for that page while the page still photographs.
        for page in PAGES:
            for pair in page.state:
                if len(pair) != 2 or not all(isinstance(x, str) and x for x in pair):
                    problems.append(f"{page.artifact}: a state pair must be two non-empty strings, "
                                    f"got {pair!r}")

    def the_audit_row_count_is_the_fixtures(problems):
        """#207: the height of audit.png is a declared number, so it cannot be a run counter.

        Seven plus the sign-in is what pages-setup.sh declares today; the rule does not restate that
        number, it asks whether a page that disagrees is refused and by how much -- 7, 8 and 9 are the
        three ways this has actually been wrong (a dropped seed row, the fixture's own count, a second
        sign-in in one run). The committed artifact was the 92%-of-the-capture-surface end of that
        sequence: 15 004 px from a fixture that renders 2 936.
        """
        spec = next(p for p in PAGES if p.declares_rows)
        row = ["", ["2026-03-18 21:34:05 -04:00", "auth.success", "nobody", "", "parent"]]
        for want, count, should_refuse in ((8, 8, False), (8, 7, True), (8, 9, True)):
            got = {"rows": [list(row) for _ in range(count)]}
            try:
                require_declared_rows(got, spec, want, "/audit")
                if should_refuse:
                    problems.append(f"a page rendering {count} rows passed against {want} declared")
            except SystemExit as exc:
                if not should_refuse:
                    problems.append(f"the declared count refused its own number: {exc}")
                elif str(count) not in str(exc) or str(want) not in str(exc):
                    problems.append(f"the count refusal names neither number: {exc}")
        # The wiring, because a check nobody splices in is a comment with a name. `declares_rows` is
        # what makes shoot() ask the browser for the census; without it `rows` is absent and the
        # refusal below is the difference between a red run and a green one that measured nothing.
        if not spec.declares_rows:
            problems.append("/audit no longer declares its row count")
        try:
            require_declared_rows({}, spec, 8, "/audit")
            problems.append("a page with no row census at all passed the row-count check")
        except SystemExit as exc:
            if "renders 0 rows" not in str(exc):
                problems.append(f"the missing-census case reports something odd: {exc}")

    def a_page_is_recorded_where_it_is_photographed(problems):
        """The manifest's stem->url map is a projection of THIS table, in the right orientation.

        What this rule can and cannot see, stated because the first version of its comment got it
        wrong in a way a reviewer could not tell from the code. `manifest.pages_by_artifact()` is
        `{spec.artifact: spec.url for spec in driver_pages()}` and `driver_pages()` IMPORTS this same
        `PAGES` -- one list, deliberately, so there is no second table to fall out of agreement. So the
        loop below cannot catch a page declared wrongly (both sides would be wrong together); what it
        catches is the projection turning around or keying on something else. Measured: keying
        `pages_by_artifact` by `spec.url` makes every artifact report `None` and the count rule report
        `5 artifacts ... and 7 to the driver`. Two artifacts sharing a stem is the other way the same
        projection fails -- one photograph standing in for two pages -- and giving a second page the
        stem `users` turns it red.

        What it does NOT pin, and the reason there is no rule for it: that `out` is the directory the
        manifest is later pointed at. `capture_out` realpath()s its target and `main` hands the same
        value to write and to record, so the binding is the pin.
        """
        declared = capture_manifest.pages_by_artifact()
        for spec in PAGES:
            written = f"{spec.artifact}.png"
            if declared.get(spec.artifact) != spec.url:
                problems.append(f"{written}: the driver writes that name but the manifest maps "
                                f"{spec.artifact!r} to {declared.get(spec.artifact)!r}, so record "
                                "either refuses the run or certifies a different page")
        if len(declared) != len(PAGES):
            problems.append(f"{len(declared)} artifacts are declared to the manifest and {len(PAGES)} "
                            "to the driver: two artifacts share a stem, and one photograph would stand "
                            "for two pages")
        # What this rule does NOT pin, stated rather than implied: that `out` is the directory the
        # manifest is later pointed at. `capture_out` realpath()s its target, so `out` is not a symlink
        # standing for somewhere else, and `main` hands the same value to both write and record. A
        # rule here could only re-derive what `main` already passes by one binding, so the binding is
        # the pin and this comment is the reason there is no second one.

    def a_page_from_the_wrong_document_refuses(problems):
        """The needles, run through the real shoot().

        Nothing else in this file can tell `users-no-profiles.png` taken from the fixture it names
        from the same file taken from the ordinary one. The ordinary accounts page fits at 360 px,
        has rows, and has a label in every cell -- it is a perfectly good photograph of the wrong
        thing, which is what makes it committable.
        """
        spec = next((p for p in PAGES if p.artifact == "users-no-profiles"), None)
        if spec is None:
            problems.append("no Page declares users-no-profiles, so the artifact has no needle "
                            "and no driver; it cannot be evidence about anything")
            return
        if not spec.needles:
            problems.append("users-no-profiles declares no needles, so any /users render publishes "
                            "a photograph named for the no-profiles state")
            return
        base = {"scroll": 360, "inner": 360, "height": 700, "tables": 1, "cells": 12,
                "forms": 2, "fields": 4, "longest": 14}
        body = "\n".join(spec.needles)
        # The measurement has to actually ask for the text. A guard reading a key the census never
        # took is the failure `stress_must_be_carried` exists for, in the other direction.
        _LAST_EXTRA.clear()
        try:
            _log, refusal = _run_shoot(dict(base, body=body), ARTIFACT_WIDTH, spec=spec)
            if refusal is not None:
                problems.append(f"the right document was refused: {refusal!r}")
            # NEEDLE_JS by identity, not "body" as a substring: the other fragments already mention
            # document.body, so a substring test stays true when the needle read is deleted. Measured.
            if not any(NEEDLE_JS in e for e in _LAST_EXTRA):
                problems.append("shoot() never asked the browser for the page text, so the needle "
                                "guard compares a key the measurement never took")
            _log, refusal = _run_shoot(dict(base, body="Accounts  casey  school  Save"),
                                       ARTIFACT_WIDTH, spec=spec)
            if refusal is None or "wrong fixture" not in refusal:
                problems.append(f"an accounts page with profiles was accepted as "
                                f"users-no-profiles: {refusal!r}")
        finally:
            _LAST_EXTRA.clear()

    # --- The controls on the page, not only the page's tables (#202) -------------------------

    def _census(*items, **over):
        """A measurement whose only content is the controls one rule is talking about.

        Every other key is the ordinary passing one, so a refusal produced by these rules can only
        have been refused BY the census: the width and cell guards are fed the same numbers they
        accept in every rule above. `inner` is the exception -- a rule that walks `WIDTHS` has to set
        it, or the viewport guard (which compares the page's reported layout against what was asked)
        refuses first and the rule reports its own guard as absent.
        """
        base = dict(_BASE_PAGE, controls={"family": PAGE_FONT, "size": PAGE_SIZE,
                                          "count": len(items), "items": list(items)})
        base.update(over)
        return base

    def the_control_census_is_asked_for(problems):
        """shoot() splices cdp.CONTROLS_JS in, at both viewports, whatever kind of page it is.

        Pinned by identity, the way NEEDLE_JS is: a substring like "controls" is already true of the
        other fragments, so a substring test would stay green with the splice deleted.
        """
        for width in WIDTHS:
            _LAST_EXTRA.clear()
            try:
                _run_shoot(_census(_control("reason")), width)
                if not any(cdp.CONTROLS_JS in e for e in _LAST_EXTRA):
                    problems.append(f"at {width} px shoot() never asked the browser for the control "
                                    "census, so the #202 guards compare a key nobody measured")
            finally:
                _LAST_EXTRA.clear()
        # This file marks typed/not-typed off its own copy of the browser's NOVALUE list, so the two
        # transcriptions have to agree or a rule builds a census the page would never return.
        marker = "NOVALUE=["
        if marker not in cdp.CONTROLS_JS:
            problems.append("cdp.CONTROLS_JS no longer declares a NOVALUE list, so this file's copy "
                            "of it cannot be checked against the one the browser runs")
        else:
            head = cdp.CONTROLS_JS.split(marker, 1)[1].split("]", 1)[0]
            want = tuple(s.strip().strip("'\"") for s in head.split(","))
            if want != NOVALUE:
                problems.append(f"this file marks {NOVALUE} as not-typed and the census in the "
                                f"browser marks {want}; `_control` derives typed/toggle from the "
                                "first, so the two would disagree about the same control")

    def a_control_in_the_browsers_own_font_refuses(problems):
        """The census Chrome reported before the fix, refused at both viewports, and no bytes."""
        drifted = _control("currentPassword", kind="password", family="Arial", size="13.3333px")
        for width in WIDTHS:
            _log, refusal = _run_shoot(_census(_control("reason"), drifted, inner=width), width)
            if refusal is None or "font of the browser's choosing" not in refusal:
                problems.append(f"an input still in Arial was accepted at {width} px: {refusal!r}")
            elif "currentPassword" not in refusal:
                problems.append(f"the font refusal does not name the control that drifted: "
                                f"{refusal!r}")
        _log, _refusal = _run_shoot(_census(_control("reason"), drifted), ARTIFACT_WIDTH)
        if any(k[0] == "png" for k in _log):
            problems.append("a page with a control in the browser's font still staged a photograph")
        # The same page with the rule on it passes, or the loop above could be satisfied by a guard
        # that refuses every page in existence.
        _log, refusal = _run_shoot(_census(_control("reason"),
                                           _control("currentPassword", kind="password")),
                                   ARTIFACT_WIDTH)
        if refusal is not None:
            problems.append(f"the fixed stylesheet was refused: {refusal!r}")

    def two_field_widths_on_one_page_refuse(problems):
        """Two typed controls sized from two places is what #202 was filed about.

        The pair is the /settings page: the two path boxes at 22ch from the file's only input rule,
        the password boxes at the browser's default. Measured before the fix, 163 px and 185 px.
        """
        for width in WIDTHS:
            _log, refusal = _run_shoot(_census(_control("certPath"),
                                               _control("pollSeconds", kind="number",
                                                        max="none"), inner=width), width)
            if refusal is None or "source" not in refusal:
                problems.append(f"two field widths on one page were accepted at {width} px: "
                                f"{refusal!r}")
        # A `select` is outside the width model deliberately -- its longest option is what sizes it,
        # and 22ch would cut a profile's name in half -- so its `max-width: none` must not be read
        # as a second source. This is the arm that pins the exclusion.
        _log, refusal = _run_shoot(_census(_control("certPath"),
                                           _control("profile", kind="select-one")),
                                   ARTIFACT_WIDTH)
        if refusal is not None:
            problems.append(f"a select's intrinsic width was counted as a second field width: "
                            f"{refusal!r}")

    def a_field_sized_by_nothing_refuses(problems):
        """All the values agreeing is not the same as a rule having decided one.

        The arm a guard that only counted DISTINCT `max-width` values would never fire on. Delete
        the width rule whole and every field agrees with every other: they all report `none`, which
        is the user-agent's twenty characters and the same defect, not a shared model.
        """
        _log, refusal = _run_shoot(_census(_control("name", max="none"),
                                           _control("start", max="none")), ARTIFACT_WIDTH)
        if refusal is None or "user-agent" not in refusal:
            problems.append(f"a page whose fields are sized by nothing was accepted: {refusal!r}")

    def a_tick_box_outside_its_words_refuses(problems):
        """`:has()` applied, or it did not -- read off the browser, not off the stylesheet.

        A rule that greps `app.css` for `label:has(` passes the day a browser stops understanding
        `:has()` and drops the selector without a word. Computed `display` is the only thing in the
        page that says whether a choice group became one box.
        """
        loose = _control("enforcement", kind="radio", label="inline")
        for width in WIDTHS:
            _log, refusal = _run_shoot(_census(_control("reason"), loose, inner=width), width)
            if refusal is None or "not one box with the words" not in refusal:
                problems.append(f"a radio that can wrap away from its label was accepted at "
                                f"{width} px: {refusal!r}")
        # A page with no choice group is not this refusal, or the loop above would be satisfied by a
        # guard that refuses every page.
        _log, refusal = _run_shoot(_census(_control("reason")), ARTIFACT_WIDTH)
        if refusal is not None:
            problems.append(f"a page with no choice group was refused: {refusal!r}")
        # A toggle with no wrapping label at all is refused too: nothing holds that box to its words.
        _log, refusal = _run_shoot(_census(_control("enabled", kind="checkbox", label="")),
                                   ARTIFACT_WIDTH)
        if refusal is None or "no wrapping label" not in refusal:
            problems.append(f"a checkbox outside any label was accepted: {refusal!r}")

    def a_census_is_printed_for_every_page(problems):
        """The transcript says what the #202 guards were shown.

        The house rule this folder learned from #108: a rule that examined nothing must not report
        success silently. /users has nothing typed into it, so `require_control_width` returns
        without saying anything, and the line has to show `typed=0` rather than leave a reader to
        work out afterwards which guards had work to do.
        """
        out = io.StringIO()
        _run_shoot(_census(_control("reason")), ARTIFACT_WIDTH, out=out)
        if "controls=1 typed=1 groups=0" not in out.getvalue():
            problems.append(f"the artifact line does not print the census it checked: "
                            f"{out.getvalue()!r}")
        out = io.StringIO()
        _run_shoot(_census(_control("enabled", kind="checkbox")), ARTIFACT_WIDTH, out=out)
        if "controls=1 typed=0 groups=1" not in out.getvalue():
            problems.append(f"a page with nothing to type into prints no zeros: "
                            f"{out.getvalue()!r}")

    def a_census_that_was_never_taken_refuses(problems):
        """A missing measurement is a refusal, at all three guards, naming its own absence.

        Driven straight rather than through shoot(), because the fake browser supplies a census so
        the rules above can be about something else: the only way to hand a guard the nothing it has
        to refuse on is to call it with nothing. `{}` with a `scroll` key is exactly the shape a
        deleted splice produces.
        """
        for guard, name in ((cdp.require_control_font, "font"),
                            (cdp.require_control_width, "width"),
                            (cdp.require_choice_groups, "group")):
            try:
                guard({"scroll": 360}, "/users")
                problems.append(f"the {name} guard passed a page with no census at all")
            except SystemExit as exc:
                if "census" not in str(exc):
                    problems.append(f"the {name} guard refused for a reason that does not name the "
                                    f"missing census: {exc}")
            except Exception as exc:
                problems.append(f"the {name} guard raised {type(exc).__name__} instead of refusing; "
                                "a traceback is not an answer")


    def _run_width(got, width):
        """The same fixture, laid out at `width`. shoot() refuses a page whose browser reports a
        different viewport from the one it was asked for -- a real guard, `viewport_is_verified_not_
        assumed`, and one a both-viewports rule has to satisfy before its own claim is tested."""
        fresh = dict(got)
        fresh["inner"] = width
        return fresh

    # --------------------------------------------------------------------------- #
    # #204: the navigation, its mark, and the palette the mark is drawn in.
    #
    # The fixtures below are Chrome's, transcribed off the fixture on 6 Nov -- the defect rows from
    # before `nav a` existed, the passing rows from after -- for the same reason CLEAN_CENSUS is a
    # transcription: a fixture written from what the fix was *supposed* to produce lets a guard pass
    # on a page no browser can render. `UA_BLUE` and its neighbours are the four strings #204's
    # table has in it for all five photographed pages, and they are what a browser answers when no
    # stylesheet has claimed an anchor, so a rule that survives them is a rule about a stylesheet.
    # --------------------------------------------------------------------------- #
    UA_BLUE = cdp.UA_LINK_BLUE          # Chrome's `-webkit-link`, what every link computed to
    UA_WEIGHT = "400"                   # the UA does not pick a weight; the page's font did
    UA_BORDER = "0px"                   # no border at all, so a border comparison saw only absence
    UA_TRANSPARENT = "rgba(0, 0, 0, 0)"

    def _ua_link(text, href, **over):
        """One navigation entry as it computed before the stylesheet said anything about `a`.

        26 px tall as well, which is the other half of what the report measured and the reason the
        row needed a rule rather than a colour: a target a thumb cannot hit is not distinguishable
        from its siblings in the way that matters on a phone, whatever its ink says.
        """
        return _nav_link(text, href, color=UA_BLUE, weight=UA_WEIGHT, dec=cdp.UA_UNDERLINE,
                         bw=UA_BORDER, bc=UA_TRANSPARENT, h=26, **over)

    def _nav_users(*extra, marked=True, **over):
        """The row the shell renders on /users: five entries, the accounts one current.

        The default page for a shoot() rule is /users, and #204's criterion 1 is about the entry
        whose href resolves to the page -- so the passing fixture has to be marked where the page
        declares, not merely marked. Every rule below that wants a defect moves one fact off this
        row and leaves the rest alone, which is what keeps each refusal attributable to one guard.
        """
        items = [_nav_link("Status", "/"), _nav_link("Audit", "/audit", x=189, w=49),
                 _nav_link("Profiles", "/profiles", x=251, w=66),
                 _nav_link("Settings", "/settings", x=24, y=114, w=70)]
        head = _nav_current("Accounts", "/users", x=93, w=83) if marked else \
            _nav_link("Accounts", "/users", x=93, w=83)
        return _nav(head, *items, *extra, **over)

    def the_navigation_census_is_asked_for(problems):
        """shoot() splices cdp.NAV_JS in, at both viewports, whatever kind of page it is.

        Pinned by identity like NEEDLE_JS and CONTROLS_JS, for the same reason: a substring like
        "nav" is already true of the row's own contents, so only the fragment itself says the
        measurement was taken. Without the splice the three guards refuse every page with "no
        navigation census was taken", which is a refusal -- and a run that refuses for the wrong
        reason on every page looks from the outside like a run that found the defect.
        """
        for width in WIDTHS:
            _LAST_EXTRA.clear()
            try:
                _run_shoot(_run_width(_nav_users(), width), width)
                if not any(cdp.NAV_JS in e for e in _LAST_EXTRA):
                    problems.append(f"at {width} px shoot() never asked the browser for the "
                                    "navigation census, so the #204 guards compare a key nobody "
                                    "measured")
            finally:
                _LAST_EXTRA.clear()

    def an_unmarked_navigation_refuses(problems):
        """The reported defect, at both viewports, with no bytes staged.

        This is the row #204 measured: five links, none carrying `aria-current`, on all five
        photographed pages. The canary beside it is the same row with the attribute on the entry
        the page declares -- a guard that refuses every navigation in existence would satisfy the
        first half and fail the second.
        """
        unmarked = _nav_users(marked=False)
        for width in WIDTHS:
            _log, refusal = _run_shoot(_run_width(unmarked, width), width)
            if refusal is None or "aria-current" not in refusal:
                problems.append(f"a navigation that marks nothing was accepted at {width} px: "
                                f"{refusal!r}")
            elif "5 links" not in refusal:
                problems.append(f"the refusal does not say how many links it counted: {refusal!r}")
        _log, refusal = _run_shoot(unmarked, ARTIFACT_WIDTH)
        if any(k[0] == "png" for k in _log):
            problems.append("a navigation with no current entry still staged a photograph")
        _log, refusal = _run_shoot(_nav_users(), ARTIFACT_WIDTH)
        if refusal is not None:
            problems.append(f"the marked navigation was refused: {refusal!r}")

    def a_navigation_the_stylesheet_never_claimed_refuses(problems):
        """Criterion 2: a link still computing to the browser's own values is not in the palette.

        The first arm is #204's table verbatim -- `rgb(0, 0, 238)`, 400, underlined -- on a row
        where the mark itself is correct, so the refusal cannot be about the attribute. The second
        arm is the case the blue alone would miss: a stylesheet that sets a colour and forgets
        `text-decoration: none` leaves the user-agent's underline on the row, which is the visible
        half of the same complaint.
        """
        blue = _nav_users(_ua_link("Support", "/support", x=104, y=114, w=62))
        for width in WIDTHS:
            _log, refusal = _run_shoot(_run_width(blue, width), width)
            if refusal is None or UA_BLUE not in refusal:
                problems.append(f"a link still in the browser's own blue was accepted at "
                                f"{width} px: {refusal!r}")
            elif "Support" not in refusal:
                problems.append(f"the palette refusal does not name the link that drifted: "
                                f"{refusal!r}")
        _log, refusal = _run_shoot(blue, ARTIFACT_WIDTH)
        if any(k[0] == "png" for k in _log):
            problems.append("a page with a user-agent link still staged a photograph")
        underlined = _nav_users(_nav_link("Support", "/support", x=104, y=114, w=62,
                                          dec=cdp.UA_UNDERLINE))
        _log, refusal = _run_shoot(underlined, ARTIFACT_WIDTH)
        if refusal is None or "underline" not in refusal:
            problems.append(f"a link still underlined by the user-agent was accepted: {refusal!r}")

    def a_navigation_marking_two_entries_refuses(problems):
        """The second route to the root, marked as well as the entry for it.

        #204's secondary fact was that the header held two anchors with `href="/"` on the dashboard
        -- the title and Status -- differing only in weight. D-0026 took the title out of the row;
        this is the guard that keeps it out, because the failure mode of a fix like that is someone
        putting the link back "for the logo" and the page then answering criterion 1 with two yeses.
        """
        both = _nav(_nav_current("Status", "/"), _nav_current("Accounts", "/users", x=93, w=83),
                    _nav_link("Audit", "/audit", x=189, w=49))
        _log, refusal = _run_shoot(both, ARTIFACT_WIDTH)
        if refusal is None or "2 navigation entries" not in refusal:
            problems.append(f"two entries marked current were accepted: {refusal!r}")
        elif "'Status'" not in refusal or "'Accounts'" not in refusal:
            problems.append(f"the refusal does not name both marked entries: {refusal!r}")

    def a_navigation_marking_the_wrong_entry_refuses(problems):
        """Criterion 1's href tie: a mark on an entry is about that entry's page and no other.

        The bug this exists for is the one a naive fix ships -- the shell marks the first entry, or
        marks from a template name rather than a path -- and it is invisible to a guard that only
        counts marks. Nine photographs of a plausibly marked row are nine photographs of the wrong
        page, which is what makes this worth a refusal of its own rather than part of the count.
        """
        wrong = _nav(_nav_current("Status", "/"), _nav_link("Accounts", "/users", x=93, w=83),
                     _nav_link("Audit", "/audit", x=189, w=49))
        for width in WIDTHS:
            _log, refusal = _run_shoot(_run_width(wrong, width), width)
            if refusal is None or "marks '/'" not in refusal:
                problems.append(f"a mark on the wrong entry was accepted at {width} px: "
                                f"{refusal!r}")
            elif "/users" not in refusal:
                problems.append(f"the refusal does not name the section the page is in: "
                                f"{refusal!r}")
        gone = _nav(_nav_current("Status", "/"), _nav_link("Audit", "/audit", x=60, w=49))
        _log, refusal = _run_shoot(gone, ARTIFACT_WIDTH)
        if refusal is None or "no such href" not in refusal:
            problems.append(f"a page whose own section is missing from the row was accepted: "
                            f"{refusal!r}")

    def a_mark_no_property_can_see_refuses(problems):
        """Criterion 1's other half: the attribute, and nothing a reader can measure.

        Both rows are marked, correctly, on the right entry. In the first, the marked link's five
        visible properties are its siblings'; in the second only `border-bottom-color` differs,
        which is what this repository's stylesheet actually does -- the rule reserves 2 px
        transparent on every link so the mark cannot move the row, and colours it on the current
        one. #204's four properties as written would call that second page unmarked, which is why
        the fifth is read, and why the second arm has to pass or the guard is refusing the fix.
        """
        quiet = _nav_users()
        quiet["nav"][0]["items"][0] = _nav_link("Accounts", "/users", x=93, w=83, cur=CUR)
        _log, refusal = _run_shoot(quiet, ARTIFACT_WIDTH)
        if refusal is None or "looks exactly like" not in refusal:
            problems.append(f"a mark no property can see was accepted: {refusal!r}")
        elif "border-bottom" not in refusal:
            problems.append(f"the refusal does not name the properties it compared: {refusal!r}")
        border_only = _nav_users()
        border_only["nav"][0]["items"][0] = _nav_link("Accounts", "/users", x=93, w=83, cur=CUR,
                                                      bc=NAV_MARK)
        _log, refusal = _run_shoot(border_only, ARTIFACT_WIDTH)
        if refusal is not None:
            problems.append(f"a mark drawn only in the reserved border was refused: {refusal!r}")

    def a_mark_on_an_unhittable_target_refuses(problems):
        """The right mark, at the right colour, on a box a thumb cannot hit (#199's other half).

        Both rows are the marked navigation on the entry the page declares; only the box's height
        moves. The defect arm is the measured 26 px, not an invented 10: that is what this stylesheet
        answers with `nav a`'s padding deleted, and a fixture written from what a defect "would" look
        like is how a guard ends up refusing a page no browser can render. The canary is the same row
        with the OTHER entries small, because the guard is about the box the reader is aiming at and a
        rule that refused the whole row would be enforcing a claim #199 never made. #229's review
        measured the padding line as deletable with everything green, in Go and in a full capture run.
        """
        small = _nav_users()
        small["nav"][0]["items"][0]["h"] = 26
        _log, refusal = _run_shoot(small, ARTIFACT_WIDTH)
        if refusal is None or "26 px" not in refusal:
            problems.append(f"a mark on a 26 px target was accepted: {refusal!r}")
        elif "Accounts" not in refusal:
            problems.append(f"the height refusal does not name the entry it measured: {refusal!r}")
        hittable = _nav_users()
        for item in hittable["nav"][0]["items"][1:]:
            item["h"] = 26
        _log, refusal = _run_shoot(hittable, ARTIFACT_WIDTH)
        if refusal is not None:
            problems.append(f"a page was refused for the height of entries nobody marks: {refusal!r}")

    def a_mark_the_browser_never_draws_refuses(problems):
        """The reserved border the mark is painted on, measured rather than assumed (#204's fifth).

        Three arms, and the third is the one that keeps this guard honest about the shipped
        stylesheet. First: every entry computes `0px`, so the current page's `border-bottom-color` is
        a colour on a border Chrome never draws -- and the five-property comparison above still passes
        it, which is exactly what #229's review found by deleting `border-bottom: 2px solid
        transparent`. Second: the widths differ between siblings, which is the row-moving design
        D-0035 refused and this is the only place that refusal is measurable. Third: the transcribed
        row -- 2 px everywhere, the current one merely coloured -- passes, or the guard below is
        refusing the fix instead of the defect.
        """
        undrawn = _nav_users()
        for item in undrawn["nav"][0]["items"]:
            item["bw"] = "0px"
        _log, refusal = _run_shoot(undrawn, ARTIFACT_WIDTH)
        if refusal is None or "draws no border" not in refusal:
            problems.append(f"a mark on a border of zero width was accepted: {refusal!r}")
        elif "border-bottom-width" not in refusal:
            problems.append(f"the border refusal does not name the property it read: {refusal!r}")
        moved = _nav_users()
        moved["nav"][0]["items"][0]["bw"] = "4px"
        _log, refusal = _run_shoot(moved, ARTIFACT_WIDTH)
        if refusal is None or "do not compute it the same" not in refusal:
            problems.append(f"entries with unequal reserved borders were accepted: {refusal!r}")
        _log, refusal = _run_shoot(_nav_users(), ARTIFACT_WIDTH)
        if refusal is not None:
            problems.append(f"the measured row's reserved border was refused: {refusal!r}")

    def a_mark_outside_the_header_refuses(problems):
        """Criterion 4: the mark is the header's current-page mark, not one from anywhere.

        A navigation moved below the header -- into a footer, or a div after it -- keeps the right
        attribute on the right href and satisfies every property comparison in the file. The claim
        the report made is about where the mark is, so the boxes are compared, and the refusal
        quotes both.
        """
        moved = _nav_users()
        for item in moved["nav"][0]["items"]:
            item["y"] = 1200
        _log, refusal = _run_shoot(moved, ARTIFACT_WIDTH)
        if refusal is None or "not the header" not in refusal:
            problems.append(f"a mark outside the header was accepted: {refusal!r}")
        # The same refusal has to come from a page that renders no header at all: criterion 4 has no
        # answer against nothing, and `None` read as "no constraint" is how a guard passes by
        # measuring less.
        noheader = _nav_users()
        noheader["header"] = None
        _log, refusal = _run_shoot(noheader, ARTIFACT_WIDTH)
        if refusal is None or "no <header>" not in refusal:
            problems.append(f"a mark measured against no header was accepted: {refusal!r}")

    def a_navigation_row_of_one_link_refuses(problems):
        """A row of one cannot be marked out of a set, and fits at any width.

        #140's lesson aimed at the header: a page that lost its content passes every property check
        and photographs an empty region. Two links is the smallest set in which "which one is this
        page" has a meaning, and one is the shape a template guard leaves behind when it decides
        nobody may navigate anywhere.
        """
        alone = _nav(_nav_current("Accounts", "/users", x=93, w=83))
        _log, refusal = _run_shoot(alone, ARTIFACT_WIDTH)
        if refusal is None or "nothing in it can be marked" not in refusal:
            problems.append(f"a navigation of one link was accepted: {refusal!r}")

    def a_navigation_that_was_never_censused_refuses(problems):
        """A missing measurement is a refusal, at all three guards, naming its own absence.

        Driven straight rather than through shoot(), because the fake browser supplies a navigation
        so the rules above can be about something else -- the only way to hand a guard the nothing
        it has to refuse on is to call it with nothing.
        """
        # Arity is part of the contract here: the palette guard is section-independent -- it refuses
        # any navigation link the stylesheet never claimed, wherever it sits in the row -- and the
        # other two take the declared section because they compare the current entry against its
        # siblings. Passing three arguments to a two-argument guard would be a TypeError, which the
        # arm below reports as exactly that rather than as a pass.
        for guard, what, args in ((cdp.require_nav_marked, "marked", ("/users",)),
                                  (cdp.require_nav_in_palette, "palette", ()),
                                  (cdp.require_nav_distinguishable, "distinguishable",
                                   ("/users",))):
            try:
                guard({"scroll": 360}, "/users", *args)
                problems.append(f"the {what} guard passed a page with no navigation census at all")
            except SystemExit as exc:
                if "navigation census" not in str(exc):
                    problems.append(f"the {what} guard refused for a reason that does not name the "
                                    f"missing census: {exc}")
            except Exception as exc:
                problems.append(f"the {what} guard raised {type(exc).__name__} instead of refusing; "
                                "a traceback is not an answer")

    def the_mark_is_printed_for_every_page(problems):
        """The transcript names which entry it accepted, so a wrong mark is visible after the fact.

        `controls=15 typed=2` says what the census saw without refusing anything; the navigation
        gets the same treatment for the same reason -- a run that marked the wrong entry on a page
        nobody re-read is recoverable from the log line if the line says which entry it marked.
        """
        line = io.StringIO()
        _run_shoot(_nav_users(), ARTIFACT_WIDTH, out=line)
        printed = line.getvalue()
        if "nav=5 links marked=Accounts" not in printed:
            problems.append(f"the printed line does not name the entry the guards accepted: "
                            f"{printed!r}")

    # --------------------------------------------------------------------------- #
    # #199: the words the pages use to report a state, and whether the browser draws them as states.

    def _state_row(cls, text="locked out", **over):
        """One state element as the browser reported it -- in its parent's voice by default.

        Every own property starts equal to the parent's, because the shape this section guards is the
        one #199 measured: a `<span>` carrying a class the stylesheet never mentioned, computing
        exactly the prose around it. A rule that wants an accepted row moves ONE property, which is
        what keeps each canary attributable to the one channel it is about.

        `pbw` and `pbg` -- the parent's own border and background -- are carried although no guard
        reads them now. Deliberate: the first version of `require_state_legible` compared them, and
        on that version the `.problem` paragraph in `/settings`, which sits inside a bordered
        `fieldset`, computed a different border-left-width from its parent and was therefore called
        drawn on a stylesheet that had never heard of the class. Measured, at both viewports, on the
        pre-fix tree. Keeping the keys is what makes a revert to that comparison fail here rather
        than pass quietly.
        """
        row = {"cls": cls, "text": text,
               "color": "rgb(26, 28, 32)", "weight": "400", "style": "normal", "dec": "none",
               "bw": "0px", "bg": "rgba(0, 0, 0, 0)",
               "pcolor": "rgb(26, 28, 32)", "pweight": "400", "pstyle": "normal", "pdec": "none",
               "pbw": "0px", "pbg": "rgba(0, 0, 0, 0)", "vis": True}
        row.update(over)
        return row

    def _states_fixture(*rows):
        """The /users page, whose navigation already passes, showing these state elements."""
        return dict(_nav_users(), states=list(rows))

    def the_state_census_is_asked_for(problems):
        """shoot() splices cdp.STATE_JS in, at both viewports, whatever kind of page it is.

        Pinned by identity like NEEDLE_JS, CONTROLS_JS and NAV_JS: a substring like "state" is
        already true of the page's own contents. Without the splice the guard refuses every page with
        "no state census was taken", and a run that refuses for the wrong reason on every page looks
        from the outside like a run that found the defect.
        """
        for width in WIDTHS:
            _LAST_EXTRA.clear()
            try:
                _run_shoot(_run_width(_nav_users(), width), width)
                if not any(cdp.STATE_JS in e for e in _LAST_EXTRA):
                    problems.append(f"at {width} px shoot() never asked the browser for the state "
                                    "census, so the #199 guard compares a key nobody measured")
            finally:
                _LAST_EXTRA.clear()

    def every_driver_that_measures_the_census_prints_it(problems):
        """`states=N drawn=M` on every driver's line, not only on the pages driver's.

        The guard ran in three drivers and the line printed in one, which is how a pull-request body
        came to say "every driver line prints it" and be false for `lock.py` and `testlock.py`. A
        refusal is loud and a pass is silent, so the printed pair is the only thing a reader can audit
        after the fact: `/lock` renders `.override` (lock.tmpl:93) and the probe page renders
        `.problem`, and a frame taken before a stylesheet change is indistinguishable from a frame
        where nothing was asked unless the two counts are on the line.
        """
        for name in ("pages.py", "lock.py", "testlock.py"):
            with open(op.join(cdp.HERE, name), encoding="utf-8") as fh:
                src = fh.read()
            if "cdp.STATE_JS" in src and "state_census_line" not in src:
                problems.append(f"{name} splices cdp.STATE_JS into its measurement and never prints "
                                "cdp.state_census_line: the guard's verdict is in the refusal and "
                                "nowhere else, so a frame it accepted cannot be told apart from a "
                                "page that has no state words at all")

    def a_state_the_stylesheet_never_drew_refuses(problems):
        """The reported defect at both viewports, with no bytes staged -- and the drawn row beside it.

        Two lookalikes and one drawn verdict is the dashboard as #199 found it: a red bold "locked
        out" next to a "no desktop session" that computed nothing of its own. The count in the
        refusal is part of the claim -- `2 of 3` says the guard found the drawn one and refused the
        rest, where `3` would be a guard that reads nothing, and `0` would be a page with no states
        at all.
        """
        rows = [_state_row("state-unknown", "no desktop session"),
                _state_row("banner", "3 accounts are on this machine and not managed"),
                _state_row("disabled", "account switched off", color="rgb(132, 86, 0)")]
        for width in WIDTHS:
            _log, refusal = _run_shoot(_run_width(_states_fixture(*rows), width), width)
            if refusal is None or "2 of 3" not in refusal:
                problems.append(f"the page was accepted at {width} px with two undrawn states on it: "
                                f"{refusal!r}")
            elif "state-unknown" not in refusal or "banner" not in refusal:
                problems.append(f"the refusal does not name the classes it refused: {refusal!r}")
            elif "disabled" in refusal:
                # The drawn row is `disabled`, whose name appears nowhere in the guard's own prose --
                # `state-held` would not do, because the refusal quotes #199's sentence about it.
                problems.append(f"the refusal blamed the row that was drawn: {refusal!r}")
        _log, refusal = _run_shoot(_states_fixture(*rows), ARTIFACT_WIDTH)
        if any(k[0] == "png" for k in _log):
            problems.append("a page whose state words are prose still staged a photograph")
        _log, refusal = _run_shoot(_states_fixture(*cdp.CLEAN_STATES), ARTIFACT_WIDTH)
        if refusal is not None:
            problems.append(f"the drawn vocabulary was refused: {refusal!r}")

    def a_state_marked_only_by_its_parents_box_refuses(problems):
        """The difference has to be the element's own, not the box it happens to be sitting in.

        Inherited properties -- colour, weight, style, decoration -- travel from the surrounding
        text, so the parent is the right thing to compare them against. A border and a background are
        not inherited; they belong to a box, and the honest question about them is what THIS element
        carries. The row below is the `/settings` help paragraph exactly: four properties equal to
        its parent, no border and no background of its own, inside a `fieldset` that has a border. On
        the first version of the guard it passed, because the guard compared the parent's 2px border
        against the child's 0px and called that a mark.
        """
        row = _state_row("problem", "A change here takes effect the next time the daemon reads this",
                         pbw="2px", pbg="rgb(240, 240, 240)")
        _log, refusal = _run_shoot(_states_fixture(row), ARTIFACT_WIDTH)
        if refusal is None or "problem" not in refusal:
            problems.append(f"an undrawn state was accepted because its parent has a border: "
                            f"{refusal!r}")

    def one_visible_channel_is_enough(problems):
        """Any one thing a parent can see is a mark, and the guard prescribes none of the six.

        This is the half that keeps the first rule honest: a guard that refuses every state element
        in existence satisfies "the undrawn ones refuse" and fails here. It is also the half that
        keeps the guard from becoming a design: colour alone, weight alone, italic alone, an
        underline alone, a 3px rule, a tinted background -- a stylesheet may pick any of them and the
        driver's only business is that the reader can tell the word from the prose.
        """
        for name, over in (("colour", {"color": "rgb(179, 38, 30)"}),
                           ("weight", {"weight": "600"}),
                           ("style", {"style": "italic"}),
                           ("decoration", {"dec": "underline"}),
                           ("border", {"bw": "3px"}),
                           ("background", {"bg": "rgb(255, 246, 224)"})):
            _log, refusal = _run_shoot(
                _states_fixture(_state_row("state-gap", "not managed", **over)), ARTIFACT_WIDTH)
            if refusal is not None:
                problems.append(f"a state marked only by its {name} was refused: {refusal!r}")

    def an_invisible_state_is_skipped_and_its_neighbour_still_refuses(problems):
        """A state the browser is not showing cannot be looked at -- and hiding it is not a pass.

        Same reasoning as #204's `a_mark_the_browser_never_draws_refuses`, pointed the other way:
        `display: none` cannot be judged for contrast, and the next template that renders a row
        conditionally would have to style the case nobody sees. The canary is the visible row in the
        same census, which still refuses and still says how many rows it skipped -- because a guard
        that skips the invisible ones has to be shown to keep reading the rest.
        """
        hidden = _state_row("state-unknown", "no desktop session", vis=False)
        _log, refusal = _run_shoot(_states_fixture(hidden), ARTIFACT_WIDTH)
        if refusal is not None:
            problems.append(f"a state the browser does not show was refused: {refusal!r}")
        _log, refusal = _run_shoot(
            _states_fixture(hidden, _state_row("banner", "Enforcement is on")), ARTIFACT_WIDTH)
        if refusal is None or "banner" not in refusal:
            problems.append(f"the visible lookalike beside a hidden row was accepted: {refusal!r}")
        elif "1 element(s) carried a state class the browser was not showing" not in refusal:
            problems.append(f"the refusal does not say how many rows it skipped: {refusal!r}")

    def a_state_census_that_was_never_taken_refuses(problems):
        """A missing measurement is a refusal, and so is the shape a deleted splice produces.

        Driven straight as well as through shoot(), for the reason #202's and #204's versions give:
        the fake browser supplies a census so the rules above can be about something else, and the
        only way to hand the guard the nothing it has to refuse on is to call it with nothing.
        `{"states": None}` through shoot() is what a splice that was commented out, or a page whose
        measurement failed to include the fragment, actually looks like from the inside.
        """
        try:
            cdp.require_state_legible({"scroll": 360}, "/users")
            problems.append("the state guard passed a page with no census at all")
        except SystemExit as exc:
            if "state census" not in str(exc):
                problems.append(f"the state guard refused for a reason that does not name the "
                                f"missing census: {exc}")
        except Exception as exc:
            problems.append(f"the state guard raised {type(exc).__name__} instead of refusing; "
                            "a traceback is not an answer")
        _log, refusal = _run_shoot(dict(_nav_users(), states=None), ARTIFACT_WIDTH)
        if refusal is None or "state census" not in refusal:
            problems.append(f"a page measured without the state fragment was accepted: {refusal!r}")
        if any(k[0] == "png" for k in _log):
            problems.append("a page with no state census still staged a photograph")

    def the_state_census_is_printed_for_every_page(problems):
        """The transcript says how many states it saw and how many it accepted, zeros included.

        `/audit` with rows in it renders no state word at all, so `states=0 drawn=0` is a real
        reading of a real page -- and the only way a reader can tell that from `states=6 drawn=0`,
        which is /users before the fix, is the number after `drawn=`.
        """
        line = io.StringIO()
        _run_shoot(_states_fixture(*cdp.CLEAN_STATES), ARTIFACT_WIDTH, out=line)
        if "states=4 drawn=4" not in line.getvalue():
            problems.append(f"the printed line does not report the census it accepted: "
                            f"{line.getvalue()!r}")
        line = io.StringIO()
        _run_shoot(_states_fixture(), ARTIFACT_WIDTH, out=line)
        if "states=0 drawn=0" not in line.getvalue():
            problems.append(f"a page with no state word prints no zeros: {line.getvalue()!r}")

    def every_state_class_the_pages_render_is_in_the_census(problems):
        """The census list is the vocabulary the templates use, in both directions.

        #199 happened because the vocabulary had no single home: nine class names are spread over
        five templates, one of them was ever styled, and nothing anywhere asked the question. The
        browser guard asks the effect; this rule asks the COVERAGE, which is the half that retires
        silently. A class renamed in a template leaves `STATE_CLASSES` measuring a name nobody
        renders -- and a guard measuring nothing is a guard that passes, exactly the failure
        `require_state_legible` itself refuses to be.

        Read off the templates on disk rather than the rendered pages on purpose: the nine names are
        all written literally in the markup, so the cheapest possible read is a complete one, and the
        rendered check would need Chrome, which `--self-test` exists to avoid.
        """
        templates = os.path.normpath(os.path.join(cdp.HERE, "..", "..", "internal", "web",
                                                  "templates"))
        try:
            names = sorted(f for f in os.listdir(templates) if f.endswith(".tmpl"))
        except OSError as exc:
            problems.append(f"the templates cannot be listed, so the census is unchecked: {exc}")
            return
        if len(names) < 5:
            problems.append(f"only {len(names)} templates found under {templates}; the coverage "
                            "claim below is only an answer about the pages if all of them are here")
            return
        emitted, where = set(), {}
        for name in names:
            with open(os.path.join(templates, name), encoding="utf-8") as fh:
                text = fh.read()
            # Every `{{...}}` action is cut out of the file FIRST, and the words left inside a class
            # attribute are the names. The order matters twice over. It is what lets this reader see
            # the four composed row names at all -- `dashboard.tmpl:33` writes
            # `class="{{if .LockedOutNow}}held{{end}}{{if .CannotEvaluate}}unreadable{{end}}"` and
            # `users.tmpl:43` the stale/unmanaged pair, so those names exist only as the text between
            # two actions; a reader that accepts a whole literal attribute calls them unreadable and
            # stops counting, which is exactly how all four stayed out of `app.css` until this rule
            # said so. And cutting before splitting on `"` is what keeps `page.tmpl:39` from poisoning
            # the census: its `class="page-{{template "pagename" .}}"` holds a quoted string INSIDE the
            # action, so splitting first cuts the attribute at that inner quote and leaves a half
            # action behind. A name built out of a value is erased by the cut and then leaves only a
            # `page-` stub, which is dropped: it is not a state and no static reader can resolve it.
            stripped = re.sub(r"\{\{.*?\}\}", " ", text)
            for attr in stripped.split('class="')[1:]:
                for token in attr.split('"', 1)[0].split():
                    if token.endswith("-"):
                        continue
                    emitted.add(token)
                    where.setdefault(token, name)
        if not emitted:
            problems.append(f"no class attribute was read under {templates}: the coverage rule is "
                            "reading nothing, which is the failure it exists to catch")
            return
        for cls in cdp.STATE_CLASSES:
            if cls not in emitted:
                problems.append(f"cdp.STATE_CLASSES measures .{cls} and no template renders it, so "
                                "the guard is asking Chrome about a class that is never on a page -- "
                                "a rename in the markup is what this looks like (#199)")
        for cls in cdp.ROW_STATE_CLASSES:
            if cls not in cdp.STATE_CLASSES:
                problems.append(f"the row state .{cls} is not in cdp.STATE_CLASSES: the four row "
                                "classes are composed inside template actions and carry no `state-` "
                                "prefix, so nothing but this list would notice one being dropped")
        for cls in sorted(emitted):
            if cls.startswith("state-") and cls not in cdp.STATE_CLASSES:
                problems.append(f"{where[cls]} puts .{cls} on a word and cdp.STATE_CLASSES does not "
                                "measure it, so the state it names is never asked about in the "
                                "browser -- add it to the census (or delete the word)")
        # The documented exception, asserted instead of remembered: the one state the stylesheet
        # draws and no page says. If it starts being rendered it belongs in the census, and the
        # sentence in cdp.py that excuses it has to go with the exemption.
        if "state-free" in emitted:
            problems.append(f"{where['state-free']} renders .state-free, which cdp.STATE_CLASSES "
                            "omits on the stated ground that no page says it -- either put it in "
                            "the census or take it back out of the markup")

    # #220's six. The clause this issue carries that #219's fixture fix did not touch: a refused
    # run published nothing, which is right, and destroyed everything it had measured to get there,
    # which left the refusal itself uncheckable. Three of these rules read the disk rather than a
    # stub for the same reason `manifest.py`'s self-test reads files: the claim is about bytes, and
    # a rule that watched a fake being handed the bytes passes with the fake doing nothing.
    def a_refused_run_says_where_its_frames_are(problems):
        with _refusal_box() as (box, out):
            rec = []
            rc, _entered, err, _o, published, _discarded = _run_main(box=box, out=out, sink=rec,
                                                                     refuse=["/audit"])
            keep = cdp.refused_dir("pages")
            if rc != 1 or published:
                problems.append(f"refusing /audit -> rc={rc!r} published={published}")
            if op.isdir(out) and any(f.endswith(".png") for f in os.listdir(out)):
                problems.append(f"a refused run wrote a PNG into {out}. Keeping the frames is "
                                "#220; publishing them is #222, and this run must do the first and "
                                "never the second")
            if keep not in err:
                problems.append(f"the summary never names {keep}, so the frames are kept somewhere "
                                f"nobody is told about: {err.strip()[-160:]!r}")
            if "Nothing was written to" not in err:
                problems.append("the summary stopped saying that --out got nothing")
            # The retraction. Every FITS line above the summary prints a path under --out, and on a
            # refused run those files do not exist: measured 6 Nov, one transcript named five PNGs
            # and the directory held none. A count and a directory are only honest together.
            if "did NOT write" not in err:
                problems.append("the transcript still prints FITS paths for files this run did not "
                                "write, without saying so")
            note_path = op.join(keep, cdp.REFUSED_NOTE)
            if not op.isfile(note_path):
                problems.append("no REFUSED.txt beside the kept frames -- a frame with no sentence "
                                "under it is what #211 says a frame is not")
                return
            with open(note_path, encoding="utf-8") as fh:
                note = fh.read()
            for want in ("/audit", "not evidence", "--out (untouched)", "conditions stated in:"):
                if want not in note:
                    problems.append(f"REFUSED.txt does not say {want!r}: {note[:160]!r}")
            # The conditions are named by their files, not copied out, so that the answer cannot
            # drift from `fixture.env` the way #213's declared condition drifted from the frame.
            # A run in a scratch box has no `fixture.env` -- the honest sentence is then the one
            # that says none has run, and it has to be there rather than the line being absent.
            if ("fixture.env" not in note
                    and "none: no setup script has run" not in note):
                problems.append(f"REFUSED.txt names no conditions and does not say there are "
                                f"none: {note[:400]!r}")

    def each_fixture_keeps_its_refused_frames_apart(problems):
        # Two fixtures, one refused directory, and the frames of a `pages` refusal are sitting
        # there when `pages-noprof` refuses next -- under names that look like the second run's.
        # This is `capture_out`'s own argument (#207's scratch-directory note) applied to the
        # place a refusal writes, and the conditions in `fixture.env` are what makes mixing them
        # a lie rather than a convenience.
        for mode, victim in (("pages", "/audit"), ("pages-noprof", "/users")):
            with _refusal_box(mode) as (box, out):
                _run_main(box=box, out=out, refuse=[victim],
                          argv=["--fixture", mode] if mode != "pages" else [])
                mine, other = (f"{mode}-refused",
                               "pages-noprof-refused" if mode == "pages" else "pages-refused")
                if not op.isdir(op.join(box, mine)):
                    problems.append(f"{mode}: no {mine}/ -- the refused directory is not named "
                                    "for the fixture, so two fixtures share one drawer")
                if op.isdir(op.join(box, other)):
                    problems.append(f"{mode}: wrote into {other}/ as well")

    def a_kept_frame_is_moved_out_of_the_publish_queue(problems):
        with _refusal_box() as (box, out):
            stage = cdp._stage_dir()
            os.makedirs(stage, exist_ok=True)
            for name in ("dashboard.png", "users.png"):
                with open(op.join(stage, name), "wb") as fh:
                    fh.write(b"staged bytes")
                cdp._PENDING.append((op.join(stage, name), op.join(out, name)))
            keep = cdp.refused_dir("pages")
            moved = cdp.keep_refused(keep, note="one refusal\n", out=out)
            if sorted(op.basename(p) for p in (moved or [])) != ["dashboard.png", "users.png"]:
                problems.append(f"keep_refused returned {moved}, want both staged frames moved")
            if not op.isdir(keep):
                # Reported and stopped here: every clause below reads the directory, and a rule
                # that dies with a FileNotFoundError instead of naming the missing directory is a
                # worse diagnostic than a wrong one. (Seen on the mutation that hollowed
                # `keep_refused` to `return []`: five rules reported a traceback, none a sentence.)
                problems.append(f"keep_refused created no {keep} at all")
                return
            if sorted(os.listdir(keep)) != [cdp.REFUSED_NOTE, "dashboard.png", "users.png"]:
                problems.append(f"the refused directory holds {sorted(os.listdir(keep))}, want the "
                                "two frames and the note")
            if op.isdir(stage) and os.listdir(stage):
                problems.append("the stage still holds the frames, so a later publish() could "
                                "copy bytes that were refused")
            if cdp._PENDING:
                problems.append(f"_PENDING still holds {cdp._PENDING}: the frames were copied "
                                "beside the queue instead of out of it")
            if op.isdir(out) and os.listdir(out):
                problems.append(f"{out} holds files -- a keep must never write to --out")
            with open(op.join(keep, cdp.REFUSED_NOTE), encoding="utf-8") as fh:
                if fh.read() != "one refusal\n":
                    problems.append("the note was not written beside the frames")

    def a_refused_directory_is_opened_once_and_cleared_once(problems):
        with _refusal_box() as (box, out):
            keep = cdp.refused_dir("pages")
            os.makedirs(keep, exist_ok=True)
            for name in ("yesterday.png", cdp.REFUSED_NOTE):
                with open(op.join(keep, name), "w", encoding="utf-8") as fh:
                    fh.write("from an earlier refusal")
            said = io.StringIO()
            with contextlib.redirect_stderr(said):
                cdp.keep_refused(keep, out=out)
            if "cleared 2 file(s)" not in said.getvalue():
                problems.append("the run cleared yesterday's frames without saying it did, so a "
                                "reader who expected to find something there has no record of when "
                                "it went")
            # What this run writes after opening is its own; a second keep that cleared again
            # would delete the frames the refusals between the two calls had just photographed.
            today = op.join(keep, "dashboard@360.png")
            with open(today, "wb") as fh:
                fh.write(b"this run")
            with contextlib.redirect_stderr(io.StringIO()):
                cdp.keep_refused(keep, note="today\n", out=out)
            left = sorted(os.listdir(keep))
            if "yesterday.png" in left:
                problems.append(f"{keep} still holds a frame from before this run: {left}. A "
                                "refusal's directory read as current is the lie this folder "
                                "exists to refuse, and it is why clearing is not optional")
            if not op.exists(today):
                problems.append(f"the second keep_refused deleted {op.basename(today)}: the "
                                "directory is cleared when it is opened, not on every call")
            with open(op.join(keep, cdp.REFUSED_NOTE), encoding="utf-8") as fh:
                if fh.read() != "today\n":
                    problems.append("REFUSED.txt still says what the earlier run said, or mixes "
                                    "two runs in one file")

    def a_keep_that_could_be_read_as_evidence_is_refused(problems):
        # Declining destroys the frames. That is the old behaviour, and it is the right fallback:
        # bytes in a place a person might commit are worth less than bytes nobody can misread.
        #
        # Both targets are made fresh by the rule itself. The first version asked for a name outside
        # the scratch tree (`dirname(box)/"elsewhere"`) and asserted the name did not exist
        # afterwards -- and the mutation that lets the guard through created that directory in the
        # system temp, so every run since reported writing into a directory an EARLIER run had left
        # there. A rule that a piece of litter can redden is a rule that gets ignored, so the target
        # is a fresh empty directory and the claim is about what is in it.
        for outside, why, make in (
                (True, "outside the scratch tree",
                 lambda box, out: tempfile.mkdtemp(prefix="plock-not-a-keep-")),
                (False, "the directory this run publishes to", lambda box, out: out)):
            with _refusal_box() as (box, out):
                target = make(box, out)
                try:
                    stage = cdp._stage_dir()
                    os.makedirs(stage, exist_ok=True)
                    with open(op.join(stage, "audit.png"), "wb") as fh:
                        fh.write(b"staged bytes")
                    cdp._PENDING.append((op.join(stage, "audit.png"), op.join(out, "audit.png")))
                    err = io.StringIO()
                    with contextlib.redirect_stderr(err):
                        got = cdp.keep_refused(target, note="x\n", out=out)
                    if got is not None:
                        problems.append(f"keep_refused accepted a directory {why}: {target}")
                    wrote = [f for f in os.listdir(target)
                             if f.endswith(".png") or f == cdp.REFUSED_NOTE] \
                        if op.isdir(target) else []
                    if wrote:
                        problems.append(f"{why}: it wrote {wrote} into {target} anyway")
                    if cdp._PENDING or (op.isdir(stage) and os.listdir(stage)):
                        problems.append(f"{why}: the frames were left staged, so a later publish "
                                        "could reach them from a directory nothing owns")
                    if "REFUSED-KEEP" not in err.getvalue():
                        problems.append(f"{why}: declining silently. A run that loses its frames "
                                        "has to say it lost them")
                finally:
                    if outside:
                        shutil.rmtree(target, ignore_errors=True)

    def a_dead_browser_still_leaves_the_refusal_standing(problems):
        # The diagnostic runs from inside the handler that reports the real cause, and the browser
        # is exactly as likely to be gone at that moment as still up -- `require_running` raising
        # is one of the refusals that gets here. A diagnostic that dies would replace the finding
        # with its own, which is the failure mode `report_refusal` documents at its own shape.
        class Gone:
            class session:  # noqa: N801 -- attribute name is the real Capture's
                @staticmethod
                def screenshot():
                    raise AssertionError("the dead browser was asked for a screenshot")

            @staticmethod
            def require_running():
                raise SystemExit("chrome exited during the capture")

        with _refusal_box() as (box, out):
            path = op.join(cdp.refused_dir("pages"), "dashboard@360.png")
            err = io.StringIO()
            try:
                with contextlib.redirect_stderr(err):
                    got = cdp.capture_refused(Gone(), path)
            except BaseException as exc:  # noqa: BLE001 -- the whole claim is that this never fires
                problems.append(f"capture_refused raised {type(exc).__name__} out of a refusal "
                                f"handler: {exc}")
                return
            if got is not None:
                problems.append(f"capture_refused returned {got!r} for a browser that was gone")
            if op.exists(path):
                problems.append("it wrote a frame it never took")
            if "no frame of the refused page" not in err.getvalue():
                problems.append("the run says nothing about the frame it could not take, so the "
                                "operator reads a silent gap as a page that never rendered")

    rule("a refusal on every page still measures every page", all_refuse)
    rule("the literal census matches the page declaration", census_matches_the_declaration)
    rule("a fixture mode selects only its own pages", a_fixture_mode_selects_its_own_pages)
    rule("an unknown fixture refuses before the browser starts", an_unknown_fixture_refuses)
    rule("a page from the wrong document refuses", a_page_from_the_wrong_document_refuses)
    rule("a needle page must name a non-default fixture",
         a_needle_page_naming_the_default_fixture_is_refused)
    rule("capture_out resolves --out and returns one directory", capture_out_returns_one_directory)
    rule("a page is recorded where it is photographed", a_page_is_recorded_where_it_is_photographed)
    rule("one refusal names it and counts the rest", one_refuses)
    rule("a clean run publishes and exits 0", none_refuses)
    rule("a crash in shoot is a refusal, not an abort", crash_is_a_refusal)
    rule("a refused run says where its frames are", a_refused_run_says_where_its_frames_are)
    rule("each fixture keeps its refused frames apart",
         each_fixture_keeps_its_refused_frames_apart)
    rule("a kept frame is moved out of the publish queue",
         a_kept_frame_is_moved_out_of_the_publish_queue)
    rule("a refused directory is opened once and cleared once",
         a_refused_directory_is_opened_once_and_cleared_once)
    rule("a keep that could be read as evidence is refused",
         a_keep_that_could_be_read_as_evidence_is_refused)
    rule("a dead browser still leaves the refusal standing",
         a_dead_browser_still_leaves_the_refusal_standing)
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
    rule("a photographed refusal has to be a refusal", refusal_photograph_proves_a_refusal)
    rule("the refusal is produced by clicking the shipped form",
         refusal_form_is_clicked_not_reimplemented)
    rule("the kept box is read out of the form that posted", kept_box_read_from_the_form_that_posted)
    rule("the fill script slices a NodeList like an array", fill_script_slices_no_node_list)
    rule("a script that never parsed is not a vanished form",
         a_script_that_never_parsed_is_not_a_vanished_form)
    rule("the refused artifact is refused about its own refusal",
         the_refused_artifact_is_refused_about_its_own_refusal)
    rule("the control census is asked for, at both viewports", the_control_census_is_asked_for)
    rule("a control in the browser's own font refuses", a_control_in_the_browsers_own_font_refuses)
    rule("two field widths on one page refuse", two_field_widths_on_one_page_refuse)
    rule("a field sized by nothing refuses", a_field_sized_by_nothing_refuses)
    rule("a tick box outside its words refuses", a_tick_box_outside_its_words_refuses)
    rule("the census a page was shown is printed", a_census_is_printed_for_every_page)
    rule("a census that was never taken refuses", a_census_that_was_never_taken_refuses)

    def a_frame_over_the_wrong_census_refuses(problems):
        """#213: the two POST frames refuse a page whose session census is not the declared one.

        The guard lives in cdp.py because both POST drivers share it, and it is pinned here because
        this is the suite `make verify` runs for the capture contract. Three arms, and the third is
        the one this page shape makes easy to get wrong: `testlock` draws TWO `table.rows` tables, so
        a census that counts `tbody tr` over the page counts the backend table too and moves when a
        backend is added — a condition that changes for a reason that has nothing to do with logins.
        Rows are transcribed from the two pages as measured on 10 Nov (2 lockable sessions; 1 backend).
        """
        def row(labels):
            return {"result": list(labels), "vals": ["x"] * len(labels)}
        sessions = [row(["Where", "Result", "Detail"]), row(["Where", "Result", "Detail"])]
        backends = [row(["Can lock", "Detail"])]
        probe = backends + [row(["Where", "Would lock", "Detail"])]
        if cdp.require_declared_sessions(sessions, 2, "the lock result page") != 2:
            problems.append("a matching census did not pass, and did not return the count the "
                            "driver prints with it")
        try:
            cdp.require_declared_sessions(sessions, 1, "the lock result page")
            problems.append("a 2-session page passed against a census of 1 — the exact shape #213 "
                            "measured: the committed frame is a 2-session page the README describes "
                            "as 1 session")
        except SystemExit as exc:
            said = str(exc)
            if "2 lockable session row(s)" not in said or "declared 1" not in said:
                problems.append(f"the refusal does not name both numbers, so the reader cannot "
                                f"tell which side moved: {said[:160]!r}")
        if cdp.session_census(probe) != 1:
            problems.append(f"the probe page's census counted {cdp.session_census(probe)} of "
                            f"{len(probe)} rows: the backend table has a row per backend and is not "
                            "part of the session census")
        try:
            cdp.require_declared_sessions(probe, 2, "the probe result page")
            problems.append("a probe page with one session row passed a census of 2, so the census "
                            "is counting backend rows")
        except SystemExit:
            pass
        if cdp.require_declared_sessions([], 0, "the lock result page") != 0:
            problems.append("an empty census refused: 0 is a count the fixture may declare, and a "
                            "refusal here would be indistinguishable from the guard not running")

    rule("a frame photographed over the wrong session census refuses",
         a_frame_over_the_wrong_census_refuses)
    # #204's nine. The first is the splice; the next six are one refusal each for one of the
    # report's criteria; the last two are the census's own absence and its own line in the log.
    rule("the navigation census is asked for, at both viewports",
         the_navigation_census_is_asked_for)
    rule("a navigation that marks nothing refuses", an_unmarked_navigation_refuses)
    rule("a link still in the browser's own values refuses",
         a_navigation_the_stylesheet_never_claimed_refuses)
    rule("two entries marked current refuse", a_navigation_marking_two_entries_refuses)
    rule("a mark on the wrong entry refuses", a_navigation_marking_the_wrong_entry_refuses)
    rule("a mark no property can see refuses", a_mark_no_property_can_see_refuses)
    # #229's two, both about the box rather than the ink: what a thumb can hit, and whether the
    # border the mark is painted on is drawn at all.
    rule("a mark on a target nobody can hit refuses", a_mark_on_an_unhittable_target_refuses)
    rule("a mark the browser never draws refuses", a_mark_the_browser_never_draws_refuses)
    rule("a mark outside the header refuses", a_mark_outside_the_header_refuses)
    rule("a navigation row of one link refuses", a_navigation_row_of_one_link_refuses)
    rule("a navigation census that was never taken refuses",
         a_navigation_that_was_never_censused_refuses)
    rule("the accepted mark is printed", the_mark_is_printed_for_every_page)

    # #199: the state vocabulary.
    rule("the state census is asked for, at both viewports", the_state_census_is_asked_for)
    rule("a driver that measures the state census prints it",
         every_driver_that_measures_the_census_prints_it)
    rule("a state the stylesheet never drew refuses", a_state_the_stylesheet_never_drew_refuses)
    rule("a state marked only by its parent's box refuses",
         a_state_marked_only_by_its_parents_box_refuses)
    rule("any one visible channel counts as drawn", one_visible_channel_is_enough)
    rule("an invisible state is skipped, its neighbour still refuses",
         an_invisible_state_is_skipped_and_its_neighbour_still_refuses)
    rule("a state census that was never taken refuses",
         a_state_census_that_was_never_taken_refuses)
    rule("the state census is printed, zeros included",
         the_state_census_is_printed_for_every_page)
    rule("the census is the vocabulary the templates use",
         every_state_class_the_pages_render_is_in_the_census)

    def manifest_accompanies_the_artifacts(problems):
        # The manifest is the only thing that ties a committed PNG to the page it depicts (#144).
        # A run that publishes bytes and no manifest leaves `make verify` nothing to compare, and
        # that is the state every screenshot on main is in today: `profiles.png` predates #141.
        rec = []
        rc, _entered, _err, _out, published, _discarded = _run_main(sink=rec)
        if rc != 0 or not published:
            problems.append(f"a clean run -> rc={rc!r} published={published}, want 0 and True")
        if "record" not in rec:
            problems.append("a clean run published PNGs without recording a manifest beside "
                            "them, so nothing can tell when they stop being current (#144)")

    def manifest_never_over_a_refusal(problems):
        # The other half, and the half that decides whether the manifest means anything: a
        # manifest written over a run that wrote no PNGs certifies photographs this run did not
        # take, which is the failure #144 was filed against.
        rec = []
        rc, _entered, _err, _out, published, discarded = _run_main(refuse=["/audit"], sink=rec)
        if rc == 0:
            problems.append("a refused run exited 0")
        if "record" in rec:
            problems.append("a refused run recorded a manifest over a run that published nothing")
        if discarded and not published:
            # Kept, not destroyed, is the new contract (#220) -- but the invariant this rule owns
            # is that the staged frames stop being a live queue either way, so that no later
            # `publish()` can reach a run's leftovers.
            problems.append("a refused run neither moved its frames out of the stage nor "
                            "destroyed them, so a later publish could reach them")

    rule("a published set of PNGs is recorded in the manifest", manifest_accompanies_the_artifacts)
    rule("a refused run records no manifest", manifest_never_over_a_refusal)
    # #207's two. Both are cheap and both are the difference between a check that exists and a check
    # that runs: each hands the same spec the declared reading and the drifted one, so a rule that
    # refuses everything cannot pass either.
    rule("the declared state is the thing photographed",
         the_declared_state_is_the_thing_photographed)
    rule("the audit row count is the fixture's", the_audit_row_count_is_the_fixtures)

    # #211's two: the conditions a frame was taken under only reach the manifest if this driver hands
    # over the files that state them, and both ways of losing that are silent.

    def the_driver_hands_over_the_envs_of_the_fixtures_it_photographed(problems):
        """Per-fixture, in page order, existing files only -- the three properties #211 needs."""
        box = tempfile.mkdtemp(prefix="capture-envs-")
        saved = cdp.RUNS
        try:
            cdp.RUNS = box
            both = [next(p for p in PAGES if p.fixture == "pages"),
                    next(p for p in PAGES if p.fixture != "pages"),
                    next(p for p in PAGES if p.fixture == "pages")]
            # Nothing on disk yet: a fixture whose setup script never ran contributes no path, so the
            # recorder writes a manifest that claims no conditions instead of asserting some.
            if fixture_envs(both) != []:
                problems.append("fixture_envs invented a path for a fixture that never ran its "
                                "setup script -- the recorder would then state conditions nobody "
                                "declared, which is #211's whole objection")
            for fixture in ("pages-noprof", "pages"):
                d = op.join(box, fixture)
                os.makedirs(d, exist_ok=True)
                with open(op.join(d, "fixture.env"), "w", encoding="utf-8") as fh:
                    fh.write("PLOCK_CAPTURE_STAMP=2026-03-18T21:34:05\n")
            got = fixture_envs(both)
            # One path per fixture even though `pages` appears twice in the run, and the fixture that
            # photographed the first page comes first.
            if [op.basename(op.dirname(x)) for x in got] != ["pages", "pages-noprof"]:
                problems.append(f"fixture_envs returned {got}; want one path per fixture, in the "
                                f"order the pages were selected")
            if len(set(got)) != len(got):
                problems.append("the same fixture env was handed over twice, and manifest.record "
                                "refuses that on purpose")
        finally:
            cdp.RUNS = saved
            shutil.rmtree(box, ignore_errors=True)

    rule("the driver hands the recorder the envs of the fixtures it photographed",
         the_driver_hands_over_the_envs_of_the_fixtures_it_photographed)

    def the_conditions_are_handed_over_at_the_manifest_step(problems):
        """The kwarg is one line long and no other rule here would notice it being deleted.

        Drop `envs=` from the call and every rule above still passes: the run photographs nine pages,
        refuses what must be refused, writes a manifest, and that manifest silently stops saying what
        any frame was taken under -- back to the state #211 was filed about, with a green self-test.
        So the call site is read, in the same spirit as manifest.py's Makefile-clause rule: a clause
        whose deletion is invisible to behaviour has to be pinned where it is written.
        """
        body = inspect.getsource(main)
        if "record_fn(out, envs=fixture_envs(" not in body:
            problems.append("main no longer passes fixture_envs(...) to record_fn, so the manifest "
                            "records no conditions for the frames this driver took")
        if "shot=[spec.artifact for spec in selected]" not in body:
            problems.append("main no longer passes shot=..., so a frame this run re-took under moved "
                            "conditions keeps the conditions in the file instead of the ones it was "
                            "just taken under -- and a fixture change would refuse its own capture")
        if "capture-manifest/1" in body and "envs=fixture_envs" not in body:
            problems.append("main mentions the /1 schema without handing over envs")

    rule("the conditions are handed over at the manifest step",
         the_conditions_are_handed_over_at_the_manifest_step)

    def the_scripts_condition_reaches_the_browser(problems):
        """What the fixture declares has to be what the browser is told, not a comment about it.

        #213's finding was only half in the manifest. `fixture.sh` declares `PLOCK_CAPTURE_SCRIPTS`,
        `manifest.py` records it, and `cdp.py` called `Emulation.setScriptExecutionDisabled` with
        `value=True` whatever the fixture said -- so the recorded value described this file rather
        than the frame, and the discriminating run the notes told a reader to perform
        (`PLOCK_CAPTURE_SCRIPTS=on ./lock-setup.sh && ./lock.py`) could only produce the pixels it
        already had. No rule above could see that, because every one of them drives `pages.py`.

        So this rule reads the sibling module, in the same spirit as manifest.py's Makefile-clause
        rule: a control whose removal is invisible to every rule in its own file has to be pinned
        where it is exercised. Two shapes are refused: the helper going away, and the switch going
        back to a literal.
        """
        try:
            helper = inspect.getsource(cdp.scripts_disabled)
        except (AttributeError, OSError, TypeError):
            problems.append("cdp.py has no scripts_disabled(), so nothing turns the fixture's "
                            "PLOCK_CAPTURE_SCRIPTS into a browser setting and the manifest is "
                            "recording a constant again (#213)")
            return
        if 'env.get("PLOCK_CAPTURE_SCRIPTS")' not in helper:
            problems.append("cdp.scripts_disabled no longer reads PLOCK_CAPTURE_SCRIPTS out of the "
                            "fixture env, so the condition a fixture declares decides nothing about "
                            "its frames (the name surviving in this file's prose is not the read)")
        try:
            call = inspect.getsource(cdp.Capture)
        except (AttributeError, OSError, TypeError):
            problems.append("cdp.Capture could not be read, so nothing pins the scripts condition "
                            "being applied to the session that takes the frame")
            return
        if "scripts_disabled(self.env" not in call:
            problems.append("cdp.Capture no longer asks the fixture's env about page scripts, so "
                            "PLOCK_CAPTURE_SCRIPTS is prose again and the manifest's `scripts` field "
                            "describes cdp.py, not the frame")
        if ('setScriptExecutionDisabled", value=True' in call
                or 'setScriptExecutionDisabled", value=False' in call):
            problems.append("the scripts switch is driven by a literal again, so every frame is "
                            "taken under one condition whatever the fixture declares")

    rule("the scripts condition reaches the browser, not just the manifest",
         the_scripts_condition_reaches_the_browser)

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
    print(f"capture self-test: {EXPECTED_RULES} refusal rules hold (no browser, no fixture, "
          f"nothing written outside a temp box -- #211's env rule needs a fixture.env on disk to "
          f"recognise, and it is built in a directory under the system temp)")
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
