#!/usr/bin/env python3
"""Measure and photograph the four GET-rendered admin pages at 360 px.

The 360 px acceptance criterion is a measurement, not a reading of the stylesheet, so this
drives a real browser: it starts the daemon on a throwaway fixture, signs in, then asks Chrome
at a 360 px viewport for documentElement.scrollWidth and a screenshot of each page. A page that
fits reports scrollWidth == 360; anything larger is horizontal overflow, which is what the
criterion refuses.

These four are the easy pages: each has a URL, so a browser can be sent straight to it. The two
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

PAGES = ["/", "/users", "/profiles", "/audit"]

# The one flag the GET pages need and the result pages do not: these four are the pages whose
# whole problem is a table wider than the viewport, and a scrollbar that is hidden changes what
# the compositor has to fit. The committed artifacts were taken with it off, so it stays off --
# a capture that differs from the artifact by a rendering flag is not evidence about the
# artifact.
CHROME_FLAGS = ("--hide-scrollbars=false",)


def shoot(session, base, page, name):
    session.goto(base + page)
    here = session.where()
    if here != page:
        raise SystemExit(f"{page} answered {here} instead; nothing was measured")
    session.settle()
    got = cdp.measure(session)
    cdp.require_records(got, page)
    cdp.require_labelled_cells(got, page)
    # Before the write, not after. A photograph of an overflowing page is the artifact the
    # criterion refuses, and a driver that writes it and then exits 1 has left the wrong evidence
    # on disk under the right filename -- with --out pointing at the docs tree it has overwritten
    # the committed artifact and only an exit code says so.
    if got["scroll"] > cdp.WIDTH:
        raise SystemExit(f"{page} is {got['scroll']} px wide at a {cdp.WIDTH} px viewport; the "
                         "criterion refuses this page, so its photograph is not taken")
    session.write_png(name)
    print(f"  {page:<10} scrollWidth={got['scroll']:>4} innerWidth={got['inner']:>4} "
          f"height={got['height']:>5} tables={got['tables']} cells={got['cells']} "
          f"FITS  {name}", flush=True)


def main():
    out = cdp.out_dir("pages")
    with cdp.Capture("pages", CHROME_FLAGS) as cap:
        for page in PAGES:
            name = op.join(out, (page.strip("/") or "dashboard") + ".png")
            try:
                shoot(cap.session, cap.base, page, name)
            except SystemExit as exc:
                cdp.refused(name, exc)
    print(f"\n{len(PAGES)}/{len(PAGES)} pages fit at {cdp.WIDTH} px", flush=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
