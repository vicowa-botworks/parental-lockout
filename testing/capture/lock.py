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
* a row claims the lock happened while its own Result cell says otherwise (#109);
* the Detail cell's sentence runs into the self-session note (#108, in cdp.py, because the probe
  page copies that cell and needs the same rule).

    ./lock-setup.sh && ./lock.py [--out DIR]

The negative control is in README.md: rebuild with the terminator clause removed and this script
must refuse and leave the previous PNG untouched. A refusal nobody watched fire is as unproven
as an assertion never run against the alternative wording.
"""
import json
import os.path as op
import sys
import time

sys.path.insert(0, op.dirname(op.abspath(__file__)))
import cdp  # noqa: E402


def main():
    out = cdp.out_dir("lock")
    name = op.join(out, "lock.png")
    with cdp.Capture("lock") as cap:
        s = cap.session
        target = cap.env["PLOCK_CAPTURE_TARGET"]

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
                                 "'rows':document.querySelectorAll('table.rows tbody tr').length")
        # The page has to be the verdict page, not an error page wearing its URL: the banner is
        # the sentence the feature exists to render, and an empty one means nothing was captured
        # no matter what scrollWidth says.
        if not g["banner"].strip():
            raise SystemExit("the result page rendered no banner sentence; nothing was measured")
        if g["rows"] == 0:
            raise SystemExit("the result page has no session rows; the account holds no session "
                             "here, so the capture would show the empty case")
        cdp.require_labelled_cells(g, "the result page")

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
        cdp.require_detail_terminated(rows, "the lock result page")

        s.write_png(name)
        print(f"viewport {cdp.WIDTH} px  page={want}", flush=True)
        print(f"  scrollWidth={g['scroll']} innerWidth={g['inner']} height={g['height']}",
              flush=True)
        print(f"  rows={g['rows']} cells={g['cells']} unlabelled={g['unlabelled']}", flush=True)
        print(f"  banner: {g['banner'].strip()[:110]}", flush=True)
        if g["flag"]:
            print(f"  flag:   {g['flag'].strip()[:110]}...", flush=True)
        fits = g["scroll"] <= cdp.WIDTH
        print(f"  {'FITS' if fits else 'OVERFLOWS'} at {cdp.WIDTH} px  -> {name}", flush=True)

        # The audit trail is the other half of the claim: an accepted lock writes a line, so a
        # page that looked right and had quietly done nothing is still caught.
        with open(op.join(cap.dir, "audit.jsonl")) as fh:
            lines = [l for l in fh if l.strip()]
        print(f"audit trail: {len(lines)} event(s)", flush=True)
        for l in lines[-3:]:
            d = json.loads(l)
            print(f"  {d.get('kind')} user={d.get('username')!r} "
                  f"detail={str(d.get('detail'))[:80]!r}", flush=True)
        return 0 if fits else 1


if __name__ == "__main__":
    sys.exit(main())
