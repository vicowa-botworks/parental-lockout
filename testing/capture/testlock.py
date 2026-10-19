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

# The one event this click is allowed to write. Everything the login and the daemon's own
# startup write lands before the mark, so it is not part of this set.
ALLOWED = {"lock.probed"}


def capture(name):
    with cdp.Capture("testlock") as cap:
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
                                 ".querySelectorAll('tbody tr').length")
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
        # The probe page copies the lock page's Detail cell, so it inherits the #108 rule. A
        # check that exists on one of two pages sharing a cell is the check that misses the
        # regression on the other one. The count matters for the same reason it does on lock.py:
        # whether a seam exists here depends on whether the account's sessions are lockable, and
        # a rule that found nothing to look at must not be quoted as a pass.
        seen, seams = cdp.require_detail_terminated(cdp.session_rows(s),
                                                  "the probe result page")
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
        s.publish()
        print(f"viewport {cdp.WIDTH} px  page={want}", flush=True)
        print(f"  scrollWidth={g['scroll']} innerWidth={g['inner']} height={g['height']}",
              flush=True)
        print(f"  tables={g['tables']} backendRows={g['backendRows']} cells={g['cells']} "
              f"unlabelled={g['unlabelled']}", flush=True)
        print(f"  banner: {g['banner'].strip()[:110]}", flush=True)
        print(f"  FITS at {cdp.WIDTH} px  -> {name}", flush=True)
        return 0


def main():
    name = op.join(cdp.capture_out("testlock"), "testlock.png")
    try:
        return capture(name)
    except SystemExit as exc:
        cdp.refused(name, exc)
        cdp.discard_stage()
    except Exception as exc:  # a traceback is not an answer; a refusal is
        cdp.refused(name, f"{type(exc).__name__}: {exc}")
        cdp.discard_stage()


if __name__ == "__main__":
    sys.exit(main())
