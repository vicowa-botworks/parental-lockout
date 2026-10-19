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

    ./testlock-setup.sh && ./testlock.py [--out DIR]
"""
import json
import os.path as op
import sys
import time

sys.path.insert(0, op.dirname(op.abspath(__file__)))
import cdp  # noqa: E402


def main():
    out = cdp.out_dir("testlock")
    name = op.join(out, "testlock.png")
    with cdp.Capture("testlock") as cap:
        s = cap.session
        target = cap.env["PLOCK_CAPTURE_TARGET"]

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
        # regression on the other one.
        cdp.require_detail_terminated(cdp.session_rows(s), "the probe result page")

        # The trail first, the file second: see the module docstring.
        with open(op.join(cap.dir, "audit.jsonl")) as fh:
            kinds = [json.loads(l).get("kind", "") for l in fh if l.strip()]
        lockish = [k for k in kinds if k.startswith("lock.") and k != "lock.probed"]
        print(f"audit trail: {len(kinds)} event(s): {sorted(set(kinds))}", flush=True)
        if "lock.probed" not in kinds:
            raise SystemExit("the probe wrote no lock.probed line")
        if lockish:
            raise SystemExit(f"the probe wrote lock lines {lockish}; it must never lock")

        s.write_png(name)
        print(f"viewport {cdp.WIDTH} px  page={want}", flush=True)
        print(f"  scrollWidth={g['scroll']} innerWidth={g['inner']} height={g['height']}",
              flush=True)
        print(f"  tables={g['tables']} backendRows={g['backendRows']} cells={g['cells']} "
              f"unlabelled={g['unlabelled']}", flush=True)
        print(f"  banner: {g['banner'].strip()[:110]}", flush=True)
        fits = g["scroll"] <= cdp.WIDTH
        print(f"  {'FITS' if fits else 'OVERFLOWS'} at {cdp.WIDTH} px  -> {name}", flush=True)
        return 0 if fits else 1


if __name__ == "__main__":
    sys.exit(main())
