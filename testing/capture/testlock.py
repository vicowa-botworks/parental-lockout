#!/usr/bin/env python3
"""Photograph the BACKEND-PROBE result page at 360 px, and measure it.

Same shape as lock.py and for the same reason: `/users/{name}/test-lock` is the response to a
POST, so there is no URL to navigate to. The browser signs in, clicks the dashboard's own
`Check backends` button, and photographs what the shipped form's own request produced.

The second half of this page's claim is in the audit trail rather than in the pixels. A probe is
supposed to write `lock.probed` and write nothing that claims a lock. A page that looked right
and had actually locked the operator's screen would otherwise be indistinguishable in this
artifact, so the trail is checked before the PNG is written, not after: a refusal that fires
after the file exists has already produced the thing it was meant to withhold.

Two things make that check mean something:

* **An allowlist, not a denylist.** Refusing kinds that start with `lock.` misses the one that
  matters most, because `lockout.entered` does not start with `lock.` -- it starts with `locko`.
  So the rule is inverted: the only line this click may write is `lock.probed`, and anything
  else stops the capture. A future benign line will therefore be refused until somebody decides
  it is benign, which is the correct direction for a check whose subject is whether the screen
  got locked.
* **Only the lines this run added.** A fixture can be photographed twice, and a membership test
  against the whole file passes on the previous run's `lock.probed` -- demonstrated, a second
  run with no intervening setup reported two of them and passed. So the driver remembers where
  the trail stood before the click and looks only after that mark.

    ./testlock-setup.sh && ./testlock.py [--out DIR] [--overwrite]
"""
import json
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
# is given in testlock-setup.sh, which is also what `cdp.fixture("testlock")` resolves to a
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
#   PLOCK_CAPTURE_SCRIPTS=on ./testlock-setup.sh && \
#     ./testlock.py --out testing/capture/runs/testlock-on --overwrite     # -> d285d1cd0d0389b6…
#   ./testlock-setup.sh && \
#     ./testlock.py --out testing/capture/runs/testlock-off --overwrite    # -> d285d1cd0d0389b6…,
#                                                                            the same file
#
# Measured 6 Nov, and again 10 Nov: one PNG from both arms, equal to the committed frame. That is what this page's
# pixels can witness, which is nothing -- the probe page renders no `time[data-unix]`, and rewriting
# those nodes is the only thing `static/app.js` does on load, so there is no element for the page's
# scripts to change. The control is `/dashboard`, where the same switch moves the casey row from the
# server's `locked out until 00:00` to the browser's `locked out until 7h 46m` and `pages.py` refuses
# the frame. So `scripts: "off"` in this frame's block is a statement about the run that produced
# these bytes, which is all the field can mean on a page the pixels cannot tell apart;
# `cdp.scripts_disabled` is what makes the declaration above true rather than decorative, and
# `manifest.py check` refuses to commit an entry that says `on`, because such a frame has never
# tested this folder's criterion.
FIXTURE = "testlock"
ARTIFACT = "testlock"

# The one event this click is allowed to write. Everything the login and the daemon's own
# startup write lands before the mark, so it is not part of this set.
ALLOWED = {"lock.probed"}


def capture(cap, name):
    s = cap.session
    target = cap.env["PLOCK_CAPTURE_TARGET"]
    trail = op.join(cap.dir, "audit.jsonl")
    first_new = len(cdp.audit_lines(trail))

    # The dashboard, and the button the operator actually has.
    s.goto(cap.base + "/")
    if s.where() != "/":
        raise SystemExit(f"the dashboard answered {s.where()} instead; nothing was measured")
    s.settle()
    form = s.json(f"""
        (() => {{
          const f = document.querySelector('form[action="/users/{target}/test-lock"]');
          if (!f) return JSON.stringify({{err: 'no probe form for {target}'}});
          const b = f.querySelector('button[type=submit]');
          const c = f.querySelector('input[name=csrf]');
          return JSON.stringify({{button: !!b, csrf: !!(c && c.value),
                                  label: b ? b.textContent.trim() : '',
                                  method: f.method, action: f.getAttribute('action')}});
        }})()""")
    if form.get("err"):
        raise SystemExit(form["err"])
    if not form["button"]:
        raise SystemExit("the probe form has no submit button")
    if not form["csrf"]:
        raise SystemExit("the probe form posts no CSRF token -- the click would be refused "
                         "and the capture would photograph a refusal")
    if form["method"].lower() != "post":
        raise SystemExit(f"the probe form is method={form['method']}, not post")
    print(f"dashboard form: action={form['action']} method={form['method']} "
          f"label={form['label']!r} csrf=yes button=yes", flush=True)

    clicked = s.value("(() => { const b = document.querySelector("
                      "'form[action=\"/users/" + target + "/test-lock\"] button[type=submit]'); "
                      "if (!b) return 'no button'; b.click(); return 'clicked'; })()")
    if clicked != "clicked":
        raise SystemExit(f"the button was never clicked: {clicked!r}")
    want = f"/users/{target}/test-lock"
    for _ in range(120):
        time.sleep(0.1)
        if s.where() == want:
            break
    else:
        raise SystemExit(f"the click never produced {want}; the browser is at {s.where()}")
    s.settle(0.5)

    g = cdp.measure(s, extra="'banner':(document.querySelector('p.banner')||{}).innerText||'',"
                             "'muted':(document.querySelector('p.muted')||{}).innerText||'',"
                             "'backendRows':document.querySelectorAll('table.rows')[0]"
                             ".querySelectorAll('tbody tr').length" + "," + cdp.CONTROLS_JS
                             + "," + cdp.NAV_JS + "," + cdp.STATE_JS)
    # The page has to be the probe result, not an error page wearing its URL. The banner
    # counts the backends, so an empty one means nothing was measured whatever scrollWidth
    # says; two tables is the page's own shape, backend table then sessions.
    if not g["banner"].strip():
        raise SystemExit("the result page rendered no banner sentence; nothing was measured")
    if "Nothing was locked" not in g["muted"]:
        raise SystemExit("the result page does not say nothing was locked; this is not the "
                         f"probe page: {g['muted'][:120]!r}")
    if g["tables"] < 2 or g["backendRows"] < 1:
        raise SystemExit(f"the result page has {g['tables']} tables and {g['backendRows']} "
                         "backend rows; the capture would show an empty diagnostic")
    cdp.require_labelled_cells(g, "the result page")
    # #202, on the other page pages.py does not photograph. The transport radios its name
    # suggests live on /settings -- a pages.py frame, where the choice-group guard has real work
    # to do -- so this page too carries only the chrome's Sign out button, and the zeros on the
    # line below are the record of that. The guard belongs here anyway: a diagnostic page is
    # where a "use this backend" tick box gets added next, and on the day it does, a `:has()`
    # rule the browser dropped would leave this frame looking exactly as it did before it.
    cdp.require_control_font(g, "the probe result page")
    cdp.require_control_width(g, "the probe result page")
    cdp.require_choice_groups(g, "the probe result page")
    # #204, and the same argument as lock.py's: `/users/<name>/test-lock` is a child of a
    # section, so the mark belongs to the accounts entry. Both POST verdict pages are covered
    # because between them and pages.py's five GET frames every page the shell renders is
    # asked, and this is the only pair where the answer is not the page's own href.
    cdp.require_nav_marked(g, "the probe result page", "/users")
    cdp.require_nav_in_palette(g, "the probe result page")
    cdp.require_nav_distinguishable(g, "the probe result page", "/users")
    # #199, on the page where the vocabulary is the answer. Both verdict columns here are two
    # spans, `state-held` for "yes" and `state-gap` for "no": one red and bold, the other the
    # same ink and weight as the sentence above it, so "no, this backend cannot lock" read as
    # an ordinary line of the explanation.
    cdp.require_state_legible(g, "the probe result page")
    rows = cdp.session_rows(s)
    # #213, the same rule as lock.py's. This page draws two tables and the census is only the
    # second one, which is why the count goes through `cdp.session_census` rather than counting
    # `tbody tr`: the backend table above it has a row per configured backend, and on 8 Nov that
    # distinction is the difference between a check and a number that moves for an unrelated
    # reason.
    census = cdp.require_declared_sessions(rows, cap.declared_sessions(),
                                           "the probe result page")
    # The probe page copies the lock page's Detail cell, so it inherits the #108 rule. A
    # check that exists on one of two pages sharing a cell is the check that misses the
    # regression on the other one. The count matters for the same reason it does on lock.py:
    # whether a seam exists here depends on whether the account's sessions are lockable, and
    # a rule that found nothing to look at must not be quoted as a pass.
    seen, seams = cdp.require_detail_terminated(rows, "the probe result page")
    if seen == 0:
        raise SystemExit("no Detail cell on the page holds the self-session note, so the "
                         "#108 seam rule examined nothing -- a rule that checked nothing is "
                         "not a reason to trust this capture")

    cdp.require_trail(cdp.audit_lines(trail)[first_new:], "lock.probed", ALLOWED,
                      "the test-lock probe")

    # Reported, never required: a lockable session's cell on this page is the note alone, so
    # seams=0 is a correct page, not a passed check. The transcript has to say which it was.
    print(f"  #108 seam: note in {seen} Detail cell(s), {seams} with a sentence in front "
          "of it", flush=True)

    fits = g["scroll"] <= cdp.WIDTH
    if not fits:
        raise SystemExit(f"the page is {g['scroll']} px wide at a {cdp.WIDTH} px viewport; "
                         "the criterion refuses this page, so its photograph is not taken")

    cap.require_running()
    s.write_png(name)
    # Every check above has passed, and only now do the bytes land: a refusal earlier in this
    # function leaves the target exactly as it was, staged frames and all.
    #
    # `FIXTURE` names this run's receipt (#302), for the reason written out in `lock.py`: this is
    # the second of the two drivers that record nothing themselves, and `testlock.png` is the other
    # row whose rev was typed rather than measured.
    s.publish(FIXTURE)
    print(f"viewport {cdp.WIDTH} px  page={want}", flush=True)
    print(f"  scrollWidth={g['scroll']} innerWidth={g['inner']} height={g['height']}",
          flush=True)
    print(f"  tables={g['tables']} backendRows={g['backendRows']} cells={g['cells']} "
          f"unlabelled={g['unlabelled']} {cdp.census_line(g)} "
          f"{cdp.nav_census_line(g)} {cdp.state_census_line(g)}", flush=True)
    print(f"  banner: {g['banner'].strip()[:110]}", flush=True)
    print(f"  FITS at {cdp.WIDTH} px  -> {name}", flush=True)
    return 0


def main():
    out = cdp.capture_out(FIXTURE)
    name = op.join(out, f"{ARTIFACT}.png")
    # Same shape as lock.py, same reason: the frame of the page this run refused, named for the
    # viewport and never after the artifact. See `cdp.refuse_single_shot` for what a refusal leaves
    # where, and #222 for why the file standing at `--out` is named instead of deleted.
    frame = f"{ARTIFACT}@{cdp.WIDTH}.png"
    # Same shape as lock.py, same reason: one disposal per refusal, decided by `cdp.SingleRefusal`
    # because the two handlers below are nested and a raise from inside the first disposal otherwise
    # reaches the outer one as a second refusal -- which is #304, measured on both drivers.
    disposal = cdp.SingleRefusal(FIXTURE, out, name, frame)
    try:
        with cdp.Capture(FIXTURE) as cap:
            try:
                return capture(cap, name)
            except (Exception, SystemExit) as exc:  # a traceback is not an answer; a refusal is
                # Inside the `with`, because only the browser this run opened can show what the page
                # actually rendered while the run was refusing it. One arm for both shapes because
                # `refusal_reason` is what states a shape, not the handler that caught it.
                disposal.refuse(cap, cdp.refusal_reason(exc))
    except (Exception, SystemExit) as exc:
        # The probe fixture refused before a browser existed -- no `testlock-setup.sh`, no binary.
        # And the arm that sees a disposal raise on its own report, which it must not answer with a
        # second refusal.
        disposal.refuse(None, cdp.refusal_reason(exc))
    # One explicit 1, outside both handlers, exactly as in lock.py: reaching the end of this function
    # means a refusal happened, and the keeping has already run beside the live browser.
    return 1


if __name__ == "__main__":
    sys.exit(main())
