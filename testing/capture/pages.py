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

Run it from anywhere; every path is resolved from this file's location. Output goes to
testing/capture/runs/pages/ unless --out names a directory inside the repository, so re-capturing
the committed artifacts is an explicit `--out docs/screenshots/360`.
"""
import os.path as op
import sys

sys.path.insert(0, op.dirname(op.abspath(__file__)))
import cdp  # noqa: E402

# Each entry is (URL, committed artifact name, whether the fixture puts a table on the page).
#
# The last field decides which refusal proves the page was really rendered, and it exists because
# the settings page has no table. Dropping require_records for it would have been the lazy edit,
# and it is the wrong one: a /settings that rendered nothing but its navigation also fits at
# 360 px, and that is exactly the photograph nobody would notice is not evidence. A table page is
# refused for having no rows; a form page is refused for having nothing to submit.
PAGES = [
    ("/", "dashboard", True),
    ("/users", "users", True),
    ("/profiles", "profiles", True),
    ("/audit", "audit", True),
    ("/settings", "settings", False),
]

# The one flag the GET pages need and the result pages do not: these four are the pages whose
# whole problem is a table wider than the viewport, and a scrollbar that is hidden changes what
# the compositor has to fit. The committed artifacts were taken with it off, so it stays off --
# a capture that differs from the artifact by a rendering flag is not evidence about the
# artifact.
CHROME_FLAGS = ("--hide-scrollbars=false",)


def shoot(cap, page, name, has_rows):
    session, base = cap.session, cap.base
    session.goto(base + page)
    here = session.where()
    if here != page:
        raise SystemExit(f"{page} answered {here} instead; nothing was measured")
    session.settle()
    got = cdp.measure(session, cdp.FORMS_JS)
    if has_rows:
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
    longest = cdp.longest_token(session) if has_rows else None
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
    if page == "/audit":
        cdp.require_unbreakable_detail(session, "the audit page")
    cap.require_running()
    session.write_png(name)
    proof = f"tables={got['tables']} cells={got['cells']}" if has_rows else \
        f"forms={got['forms']} fields={got['fields']}"
    print(f"  {page:<10} scrollWidth={got['scroll']:>4} innerWidth={got['inner']:>4} "
          f"height={got['height']:>5} {proof}{hard} "
          f"FITS  {name}", flush=True)


def main():
    out = cdp.capture_out("pages")
    refused_pages = []
    with cdp.Capture("pages", CHROME_FLAGS) as cap:
        for page, artifact, has_rows in PAGES:
            name = op.join(out, artifact + ".png")
            try:
                shoot(cap, page, name, has_rows)
            except SystemExit as exc:
                cdp.refused(name, exc)
                refused_pages.append(page)
            except Exception as exc:  # a traceback is not an answer; a refusal is
                cdp.refused(name, f"{type(exc).__name__}: {exc}")
                refused_pages.append(page)
        if refused_pages:
            # The count is the exit status, so it has to be the true one. This line used to print
            # `4/4 pages fit` and return 0 whatever had happened, because a refusal was caught
            # per page and the loop simply carried on -- a run that photographed three pages and
            # refused the fourth reported complete success, which is the failure this whole
            # package exists to make impossible.
            cdp.discard_stage()
            print(f"\n{len(PAGES) - len(refused_pages)}/{len(PAGES)} pages fit at "
                  f"{cdp.WIDTH} px; refused: {', '.join(refused_pages)}. Nothing was written to "
                  f"{out}.", file=sys.stderr, flush=True)
            return 1
        cap.session.publish()
    print(f"\n{len(PAGES)}/{len(PAGES)} pages fit at {cdp.WIDTH} px -> {out}", flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
