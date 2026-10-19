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

    ./pages-setup.sh && ./pages.py [--out DIR]

Run it from anywhere; every path is resolved from this file's location. Output goes to
testing/capture/runs/pages/ unless --out names a directory, so re-capturing the committed
artifacts is an explicit `--out docs/screenshots/360`.
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


def main():
    out = cdp.out_dir("pages")
    bad = 0
    with cdp.Capture("pages", CHROME_FLAGS) as cap:
        s = cap.session
        for page in PAGES:
            s.goto(cap.base + page)
            here = s.where()
            if here != page:
                raise SystemExit(f"{page} answered {here} instead; nothing was measured")
            s.settle()
            got = cdp.measure(s)
            cdp.require_records(got, page)
            cdp.require_labelled_cells(got, page)
            name = op.join(out, (page.strip("/") or "dashboard") + ".png")
            s.write_png(name)
            fits = got["scroll"] <= cdp.WIDTH
            bad += 0 if fits else 1
            print(f"  {page:<10} scrollWidth={got['scroll']:>4} innerWidth={got['inner']:>4} "
                  f"height={got['height']:>5} tables={got['tables']} cells={got['cells']} "
                  f"{'FITS' if fits else 'OVERFLOWS'}  {name}", flush=True)
        print(f"\n{len(PAGES) - bad}/{len(PAGES)} pages fit at {cdp.WIDTH} px", flush=True)
    return 1 if bad else 0


if __name__ == "__main__":
    sys.exit(main())
