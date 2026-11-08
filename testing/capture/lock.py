#!/usr/bin/env python3
"""Photograph the LOCK RESULT page at 360 px, and measure it.

This page has no URL you may navigate to. It is the response to a POST from the dashboard, so
the only honest capture is a browser that signs in, clicks the dashboard's own `Lock now` button,
and photographs what comes back: the real form, the real CSRF field, the real handler, the real
rendered page.

So the script refuses to fake any of that. It stops, without writing a file, when:

* the dashboard has no `lock-now` form for the target account, or that form has no submit
  button, or no CSRF field with a value in it, or is not method=post -- a hand-built request
  body would prove nothing about what the shipped template sends, and a refused POST photographs
  a refusal;
* the click does not navigate to the result page;
* the result page renders no banner sentence, or no session rows (the empty case fits at any
  width and says nothing);
* any cell lacks a `data-label`;
* the self-session note does not appear in any Detail cell, because then the #108 rule below
  would have had nothing to look at and a rule that examined nothing reports success for free;
* a row claims the lock happened while its own Result cell says otherwise (#109);
* the Detail cell's sentence runs into the self-session note (#108, in cdp.py, because the probe
  page copies that cell and needs the same rule);
* the page overflows 360 px -- checked before the write, because a photograph of an overflowing
  page is the artifact the criterion refuses, and writing it and then exiting 1 leaves the wrong
  evidence on disk with a green-looking filename on it;
* the click wrote no `lock.requested` line to the audit trail.

The trail check counts only the lines this run added. A fixture can be photographed twice, and a
membership test against the whole file passes on the previous run's line -- which is exactly the
case where this run's click did nothing.

    ./lock-setup.sh && ./lock.py [--out DIR] [--overwrite]

The negative control is in README.md: rebuild with the terminator removed and this script must
refuse, and say what happened to the PNG that is still lying there from the last run.
"""
import os.path as op
import sys
import time

sys.path.insert(0, op.dirname(op.abspath(__file__)))
import cdp  # noqa: E402


# Which fixture this frame is evidence about, and what the manifest calls the file.
#
# `manifest.py` learns the pairing by importing these two names from the driver that takes the shot.
# A second table of the same pairs somewhere else is the thing to avoid: the update someone forgets
# is the one that attributes one fixture's clock to another fixture's pixels, which is the failure
# #211 refuses rather than records. The name is the run directory's name -- the one `capture_begin`
# is given in lock-setup.sh, which is also what `cdp.fixture("lock")` resolves to a
# `fixture.env` from -- so there is exactly one place that says what this fixture is called.
#
# It is NOT the same fixture as the five GET pages. This one starts with an empty audit trail and
# declares neither audit rows nor a reader zone, so its manifest block carries no `rows` and no `tz`;
# the pages fixture seeds 8 rows and declares both. Binding these two artifacts to `pages` because
# the state looks similar would have the manifest assert a condition these pixels never saw.
#
# What this frame was taken under, and how to check it. The scripts condition is `off`, and the frame
# that carries it was photographed by THIS driver on this branch, at the commit the manifest's `head`
# names -- not inferred from an older one. The discriminating test, both arms:
#
#   PLOCK_CAPTURE_SCRIPTS=on ./lock-setup.sh && \
#     ./lock.py --out testing/capture/runs/lock-on --overwrite      # -> b59bb2be…
#   ./lock-setup.sh && \
#     ./lock.py --out testing/capture/runs/lock-off --overwrite     # -> b59bb2be…, the same file
#
# Measured 6 Nov: one PNG from both arms, equal to the committed frame. That is what this page's
# pixels can witness, which is nothing -- /lock renders no `time[data-unix]`, and rewriting those
# nodes is the only thing `static/app.js` does on load, so there is no element for the page's scripts
# to change. The control is `/dashboard`, where the same switch moves the casey row from the server's
# `locked out until 00:00` to the browser's `locked out until 7h 46m` and `pages.py` refuses the
# frame. So `scripts: "off"` in this frame's block is a statement about the run that produced these
# bytes, which is all the field can mean on a page the pixels cannot tell apart; `cdp.scripts_disabled`
# is what makes the declaration above true rather than decorative, and `manifest.py check` refuses to
# commit an entry that says `on`, because such a frame has never tested this folder's criterion.
FIXTURE = "lock"
ARTIFACT = "lock"


def capture(name):
    with cdp.Capture(ARTIFACT) as cap:
        s = cap.session
        target = cap.env["PLOCK_CAPTURE_TARGET"]
        trail = op.join(cap.dir, "audit.jsonl")
        # Where this run starts in the trail. Everything before it belongs to whoever last
        # photographed this fixture, and is not evidence about this click.
        first_new = len(cdp.audit_lines(trail))

        # The dashboard, and the button the operator actually has.
        s.goto(cap.base + "/")
        if s.where() != "/":
            raise SystemExit(f"the dashboard answered {s.where()} instead; nothing was measured")
        s.settle()
        form = s.json(f"""
            (() => {{
              const f = document.querySelector('form[action="/users/{target}/lock-now"]');
              if (!f) return JSON.stringify({{err: 'no lock-now form for {target}'}});
              const b = f.querySelector('button[type=submit]');
              const c = f.querySelector('input[name=csrf]');
              return JSON.stringify({{button: !!b, csrf: !!(c && c.value),
                                      method: f.method, action: f.getAttribute('action')}});
            }})()""")
        if form.get("err"):
            raise SystemExit(form["err"])
        if not form["button"]:
            raise SystemExit("the lock-now form has no submit button")
        if not form["csrf"]:
            raise SystemExit("the lock-now form posts no CSRF token — the click would be refused "
                             "and the capture would photograph a refusal")
        if form["method"].lower() != "post":
            raise SystemExit(f"the lock-now form is method={form['method']}, not post")
        print(f"dashboard form: action={form['action']} method={form['method']} "
              f"csrf=yes button=yes", flush=True)

        # A real click on the real button, so the request is the one the template produces.
        clicked = s.value("(() => { const b = document.querySelector("
                          "'form[action=\"/users/" + target + "/lock-now\"] button[type=submit]'); "
                          "if (!b) return 'no button'; b.click(); return 'clicked'; })()")
        if clicked != "clicked":
            raise SystemExit(f"the button was never clicked: {clicked!r}")
        want = f"/users/{target}/lock-now"
        for _ in range(120):
            time.sleep(0.1)
            if s.where() == want:
                break
        else:
            raise SystemExit(f"the click never produced {want}; the browser is at {s.where()}")
        s.settle(0.5)

        g = cdp.measure(s, extra="'banner':(document.querySelector('p.banner')||{}).innerText||'',"
                                 "'flag':(document.querySelector('p.flag')||{}).innerText||'',"
                                 "'rows':document.querySelectorAll('table.rows tbody tr').length"
                                 "," + cdp.CONTROLS_JS + "," + cdp.NAV_JS + "," + cdp.STATE_JS)
        # The page has to be the verdict page, not an error page wearing its URL: the banner is
        # the sentence the feature exists to render, and an empty one means nothing was captured
        # no matter what scrollWidth says.
        if not g["banner"].strip():
            raise SystemExit("the result page rendered no banner sentence; nothing was measured")
        if g["rows"] == 0:
            raise SystemExit("the result page has no session rows; the account holds no session "
                             "here, so the capture would show the empty case")
        cdp.require_labelled_cells(g, "the result page")
        # #202, on one of the two pages pages.py does not photograph. What either of them carries
        # today is the chrome's own Sign out button -- not typed into, not a choice -- so all three
        # guards pass having found nothing, and the `controls=1 typed=0 groups=0` on the line below
        # is what says so out loud. They are here anyway because a verdict page is exactly where a
        # note box or a tick box gets added next, and then the claim has to be measured rather than
        # assumed from the day this line was written.
        cdp.require_control_font(g, "the lock result page")
        cdp.require_control_width(g, "the lock result page")
        cdp.require_choice_groups(g, "the lock result page")
        # #204, on one of the two pages pages.py does not photograph -- and the case its five GET
        # frames would never have caught: this URL is `/users/<name>/lock-now`, a child of a
        # section, so the accounts entry is the one that has to carry the mark. A shell that marks
        # only an exact path match renders this page with nothing current and every property guard
        # still green, because there would be no current entry to compare. `"/users"` is declared
        # here rather than derived from `s.where()` for the reason D-0025 gives for the same choice
        # in JavaScript: deriving it would be a second copy of the section rule, and it would agree
        # with any bug the shell has.
        cdp.require_nav_marked(g, "the lock result page", "/users")
        cdp.require_nav_in_palette(g, "the lock result page")
        cdp.require_nav_distinguishable(g, "the lock result page", "/users")
        # #199, and this is the page the issue's answer is actually about. `state-held` is the red
        # "locked" and `state-unknown` is the plain "not locked" in the same Result column: two
        # opposite verdicts, one of them drawn and one of them left as prose, on the one page an
        # operator quotes as evidence. The GET frames cannot see this because they never click.
        cdp.require_state_legible(g, "the lock result page")

        rows = cdp.session_rows(s)
        for r in rows:
            try:
                i = [k.lower() for k in r["result"]].index("result")
            except ValueError:
                raise SystemExit("a session row has no Result cell; the capture is not the table "
                                 "this page is supposed to show")
            verdict = r["vals"][i]
            detail = " ".join(r["vals"])
            if "so it happened" in detail and not verdict.lower().startswith("locked"):
                raise SystemExit(f"a {verdict!r} row still claims the lock happened; this is the "
                                 "pre-fix page, not the one being shipped")
        if not any("the column beside it says what came of it" in " ".join(r["vals"])
                   for r in rows):
            raise SystemExit("no row carries the gated clause; the self-session note did not "
                             "render, so this capture proves nothing about it")
        seen, seams = cdp.require_detail_terminated(rows, "the lock result page")
        if seen == 0:
            raise SystemExit("no Detail cell on the page holds the self-session note, so the "
                             "#108 seam rule examined nothing -- a rule that checked nothing is "
                             "not a reason to trust this capture")

        # The trail is the other half of the claim, and it is read before the PNG is written: an
        # accepted lock writes a line, so a page that looked right and had quietly done nothing
        # is caught here rather than photographed.
        cdp.require_trail(cdp.audit_lines(trail)[first_new:], "lock.requested",
                          {"lock.requested"}, "the lock-now click")

        print(f"  #108 seam: note in {seen} Detail cell(s), {seams} with a sentence in front "
              "of it", flush=True)

        fits = g["scroll"] <= cdp.WIDTH
        if not fits:
            raise SystemExit(f"the page is {g['scroll']} px wide at a {cdp.WIDTH} px viewport; "
                             "the criterion refuses this page, so its photograph is not taken")

        cap.require_running()
        s.write_png(name)
        # Every check above has passed, and only now do the bytes land. A refusal anywhere earlier
        # leaves the target exactly as it was, staged frames and all.
        s.publish()
        print(f"viewport {cdp.WIDTH} px  page={want}", flush=True)
        print(f"  scrollWidth={g['scroll']} innerWidth={g['inner']} height={g['height']}",
              flush=True)
        print(f"  rows={g['rows']} cells={g['cells']} unlabelled={g['unlabelled']} "
              f"{cdp.census_line(g)} {cdp.nav_census_line(g)}", flush=True)
        print(f"  banner: {g['banner'].strip()[:110]}", flush=True)
        if g["flag"]:
            print(f"  flag:   {g['flag'].strip()[:110]}...", flush=True)
        print(f"  FITS at {cdp.WIDTH} px  -> {name}", flush=True)
        return 0


def main():
    name = op.join(cdp.capture_out("lock"), "lock.png")
    try:
        return capture(name)
    except SystemExit as exc:
        cdp.report_refusal(name, exc)
    except Exception as exc:  # a traceback is not an answer; a refusal is
        cdp.report_refusal(name, f"{type(exc).__name__}: {exc}")
    # Both refusals land here, and the exit code is decided once, on the outside. It used to come
    # from the refusal helper raising -- which also made the discard on the next line dead code, so
    # a refused run left its staged bytes sitting in the way of the next one. Discarding and
    # returning 1 from the same place is the only shape where a refusal cannot turn into a pass.
    cdp.discard_stage()
    return 1


if __name__ == "__main__":
    sys.exit(main())
