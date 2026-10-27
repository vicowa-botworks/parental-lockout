#!/usr/bin/env python3
"""Pin a committed 360 px screenshot to the page it depicts (#144).

`docs/screenshots/360/*.png` are acceptance evidence for "every page is usable at 360 px with
JavaScript off". Until now nothing in the repository tied a committed PNG to the page it shows, so
a screenshot could sit in `docs/` describing a page that had since grown two forms and `make
verify` stayed green. That is not a hypothetical: `profiles.png` on `main` predates #141, which
changed `profiles.tmpl` and committed no new photograph.

The driver cannot catch this and never could. `pages.py` refuses to write anything when any page
overflows, so the failure mode is not a bad write -- it is NO write, in a run whose PR still
merges. #108's lesson was "a screenshot with no driver behind it is an assertion nobody can
re-run"; this is the same failure one step later, a driver that ran, on a different page, leaving
the old picture where the new one belongs.

So the manifest records what a photograph was taken OF, and `check` refuses when that has moved:

    {
      "schema": "capture-manifest/1",
      "viewport": 360,
      "head": "<HEAD at capture>",
      "captured_at": "<UTC>",
      "pages": [{"page": "/profiles", "png": "profiles.png", "png_sha256": "...",
                 "scroll_width": 360, "sources": [...], "sources_rev": "<commit>"}]
    }

`sources_rev` is `git log -1 --format=%H` over the files that page is rendered from, and the check
recomputes it. A commit that touches a template or the stylesheet therefore retires the
photographs of every page that reads them -- which is the truth, and the reason the shared layout
and `app.css` are in every page's source set rather than in none.

Two things the check refuses that a "compare the dates" design would miss:

  - **mtimes are not evidence.** A checkout's mtimes are the checkout's, not the capture's; a
    clone made today would find every PNG "newer" than every template. The comparison is against
    git history, which is the only clock here that means what it says.
  - **an uncommitted edit is the common case.** `git log` cannot see a template you changed five
    minutes ago, so the working tree is asked as well. That is what makes `make verify` refuse
    while you are still holding the edit, instead of after it has merged.

Run it two ways:

    python3 testing/capture/manifest.py record --dir docs/screenshots/360   # after a capture
    python3 testing/capture/manifest.py check                               # in `make verify`
    python3 testing/capture/manifest.py --self-test                         # the contract
"""

import argparse
import hashlib
import json
import os
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(os.path.dirname(HERE))
MANIFEST = "manifest.json"
SCHEMA = "capture-manifest/1"
ARTIFACT_WIDTH = 360

# Every page is rendered from its own template plus the shared shell and the stylesheet. The
# shared files are in every page's set on purpose: a rule in `app.css` or a change to `page.tmpl`
# moves the rendering of all five pages, and a manifest that pretended otherwise would keep
# calling four stale photographs current.
SHARED_SOURCES = ("internal/web/templates/page.tmpl",
                  "internal/web/static/app.css",
                  "internal/web/static/app.js")


def sources_for(artifact):
    """The files a page's photograph is evidence about: its template, and what wraps it."""
    return [f"internal/web/templates/{artifact}.tmpl"] + list(SHARED_SOURCES)


def git(args, cwd=None):
    """One git call, as text. Raises OSError when there is no git to ask."""
    out = subprocess.run(["git"] + args, cwd=cwd or ROOT, capture_output=True, text=True,
                         encoding="utf-8", timeout=60)
    if out.returncode != 0:
        raise SystemExit(f"git {' '.join(args)} failed: {out.stderr.strip()}")
    # Newlines only. A bare strip() also eats the leading space of ` M path`, which is the first
    # of the three characters `git status --porcelain` puts before a filename, and a caller that
    # counts on them then reports the source as "nternal/web/static/app.css" -- a sentence with a
    # path in it that names nothing, printed by the one check whose job is to name the file.
    return out.stdout.strip("\r\n")


def newest_commit(sources, run=git):
    """The commit that last touched any of `sources` -- the thing a photograph ages against."""
    return run(["log", "-1", "--format=%H", "--"] + list(sources))


def dirty_sources(sources, run=git):
    """Which of `sources` have an uncommitted change, so a live edit retires the PNG too."""
    out = run(["status", "--porcelain", "--"] + list(sources))
    paths = []
    for line in out.splitlines():
        if not line.strip():
            continue
        # Split rather than sliced: the status column is two characters and a space, and a parser
        # that counts them is one leading-space difference away from naming a file that does not
        # exist. `-> ` is a rename, and the page is rendered from where the file went.
        path = line.split(maxsplit=1)[1] if " " in line.strip() else ""
        paths.append(path.rsplit(" -> ", 1)[-1].strip())
    return [p for p in paths if p]


def sha256(path):
    h = hashlib.sha256()
    with open(path, "rb") as fh:
        for chunk in iter(lambda: fh.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


def manifest_path(directory):
    return os.path.join(directory, MANIFEST)


def read_manifest(directory):
    try:
        with open(manifest_path(directory), encoding="utf-8") as fh:
            return json.load(fh)
    except FileNotFoundError:
        return None
    except (OSError, ValueError) as exc:
        raise SystemExit(f"{manifest_path(directory)} cannot be read: {exc}")


def pngs_in(directory):
    return sorted(f for f in os.listdir(directory) if f.endswith(".png"))


def record(directory, head=None, run=git, now=None):
    """Write the manifest for the PNGs sitting in `directory`, then return it.

    Called by `pages.py` on the path that has already published its staged files. It is not
    called on the refusal path, and it must not be: a manifest written over a run that refused
    would certify photographs this run did not take.
    """
    entries = []
    for name in pngs_in(directory):
        artifact = name[:-len(".png")]
        sources = sources_for(artifact)
        missing = [s for s in sources if not os.path.exists(os.path.join(ROOT, s))]
        if missing:
            raise SystemExit(f"{name}: its manifest cannot be written, because {missing[0]} "
                             f"does not exist. A PNG named after a page with no template is not "
                             f"evidence about anything.")
        entries.append({
            "page": "/" if artifact == "dashboard" else "/" + artifact,
            "png": name,
            "png_sha256": sha256(os.path.join(directory, name)),
            "scroll_width": ARTIFACT_WIDTH,
            "sources": sources,
            "sources_rev": newest_commit(sources, run=run),
        })
    doc = {"schema": SCHEMA, "viewport": ARTIFACT_WIDTH,
           # Through the same seam as everything else that asks git a question: a record path
           # that reached for the real git could not be driven, and an undrivable path is one
           # the self-test cannot promise anything about.
           "head": head if head is not None else run(["rev-parse", "HEAD"]),
           "captured_at": now or _utc_now(), "pages": entries}
    tmp = manifest_path(directory) + ".tmp"
    with open(tmp, "w", encoding="utf-8") as fh:
        json.dump(doc, fh, indent=2, sort_keys=True)
        fh.write("\n")
    os.replace(tmp, manifest_path(directory))
    return doc


def verify_clause_problems(text):
    """Is this check still wired into `make verify`? (#144's second criterion.)

    Read from the Makefile's text rather than from a run of `make`, because the thing to notice is
    the deletion of a clause, and a recipe that stopped calling this check is indistinguishable
    from a repository that has no stale screenshots -- both are a green `verify`. Same argument
    `check-mutations.py` makes about its own summary line, one file over: the claim has to be
    pinned to the source, or the cheapest way to pass it is to stop making it.
    """
    if not text:
        return ("the Makefile could not be read, so nothing here can promise `make verify` still "
                "runs this check.")
    if "manifest.py check" not in text:
        return ("`make verify` no longer runs `manifest.py check`. A gate with no pin is a "
                "suggestion: the check exists to refuse a screenshot that predates its page, and "
                "deleting the clause is the way that failure comes back without anyone editing "
                "the check.")
    return None


# Read at import, before any self-test points ROOT at a temp box. The Makefile under test is the
# one this file ships in, not whatever directory a rule happens to be standing in.
try:
    with open(os.path.join(ROOT, "Makefile"), encoding="utf-8") as _fh:
        _MAKEFILE = _fh.read()
except OSError:  # reported as a problem by the rule below, never as a crash
    _MAKEFILE = ""


def _utc_now():
    import datetime
    return datetime.datetime.now(datetime.timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def check(directory, run=git, pages=None):
    """Every way a committed photograph can stop being evidence, as a list of sentences."""
    problems = []
    doc = read_manifest(directory)
    on_disk = pngs_in(directory)
    if doc is None:
        if on_disk:
            return [f"{len(on_disk)} committed screenshot(s) in {directory} and no {MANIFEST}: "
                    f"nothing says which page each one shows or what it was rendered from, so "
                    f"none of them is evidence about the page it claims (#144)."]
        return [f"{directory} holds no screenshots and no manifest."]

    if doc.get("schema") != SCHEMA:
        problems.append(f"{MANIFEST} declares schema {doc.get('schema')!r}, want {SCHEMA!r}")
    if doc.get("viewport") != ARTIFACT_WIDTH:
        problems.append(f"{MANIFEST} was recorded at viewport {doc.get('viewport')!r}, not "
                        f"{ARTIFACT_WIDTH} -- the acceptance width. A manifest taken at another "
                        f"width is not evidence about this one.")

    entries = doc.get("pages", [])
    seen = set()
    for e in entries:
        name = e.get("png", "")
        seen.add(name)
        page = e.get("page", "?")
        path = os.path.join(directory, name)
        if not os.path.exists(path):
            problems.append(f"{MANIFEST} describes {name} for {page} and no such file is "
                            f"committed -- the manifest and the artifacts have come apart")
            continue
        want = e.get("png_sha256")
        got = sha256(path)
        if want != got:
            problems.append(f"{name} is not the file this manifest recorded ({got[:12]}... on "
                            f"disk, {str(want)[:12]}... recorded): it was replaced without "
                            f"re-running the capture, so nothing here says it still fits")
        sources = e.get("sources") or sources_for(name[:-len(".png")])
        missing = [s for s in sources if not os.path.exists(os.path.join(ROOT, s))]
        if missing:
            problems.append(f"{page}'s manifest names {missing[0]}, which does not exist. A page "
                            f"whose template was renamed or deleted cannot be photographed, so "
                            f"the photograph is not evidence about anything")
            continue
        live = dirty_sources(sources, run=run)
        if live:
            problems.append(f"{name} is held against {page}'s sources, and {live[0]} has an "
                            f"uncommitted change: the page on disk is not the page in the "
                            f"photograph. Re-run testing/capture before committing.")
            continue
        recorded = e.get("sources_rev")
        current = newest_commit(sources, run=run)
        if recorded != current:
            problems.append(f"{name} predates the newest change to {page}'s sources: the "
                            f"manifest was taken at {str(recorded)[:12]}... and "
                            f"{sources[0]} (or the shared shell) has moved to "
                            f"{str(current)[:12]}... since. The page grew and the photograph "
                            f"did not (#144).")

    for name in on_disk:
        if name not in seen:
            problems.append(f"{name} is committed with no entry in {MANIFEST}: nothing records "
                            f"what it was rendered from, so nothing can tell when it stops being "
                            f"current (#144)")

    # The other direction: a page added to the driver with no re-capture leaves four current
    # photographs and one page nobody has ever photographed, and every check above passes.
    for spec in (pages if pages is not None else driver_pages()):
        if spec.artifact + ".png" not in seen:
            problems.append(f"the capture driver photographs {spec.url} but {MANIFEST} has no "
                            f"entry for it: that page has never been committed as evidence")
    return problems


def driver_pages():
    """The pages the driver is contracted to photograph, read from the driver itself.

    Imported here rather than at module scope so `check` still answers when the browser driver
    cannot be imported, and so the list is never copied into this file -- a second list of the
    same five pages is a second thing to forget to update.
    """
    sys.path.insert(0, HERE)
    import pages
    return pages.PAGES


def main(argv=None):
    ap = argparse.ArgumentParser(prog="manifest.py", description=__doc__.splitlines()[0])
    ap.add_argument("verb", nargs="?", default="check", choices=("check", "record"))
    ap.add_argument("--dir", default=os.path.join(ROOT, "docs", "screenshots", "360"))
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args(argv)
    if a.self_test:
        return self_test()
    if a.verb == "record":
        doc = record(a.dir)
        print(f"recorded {len(doc['pages'])} page(s) at {doc['viewport']} px into "
              f"{manifest_path(a.dir)}")
        return 0
    problems = check(a.dir)
    if problems:
        print(f"screenshot manifest: {len(problems)} problem(s):")
        for p in problems:
            print(f"  {p}")
        return 1
    doc = read_manifest(a.dir)
    print(f"screenshot manifest: {len(doc['pages'])} photographs current against their sources "
          f"(each pinned to {doc['viewport']} px and to the commit its page was rendered from)")
    return 0


# --------------------------------------------------------------------------------------
# The contract, with no browser and no repository.
#
# Same shape as pages.py's self-test: a rule is a complaint that must be None, a canary proves a
# hollowed rule still fails, and the tally is pinned because a deleted rule is invisible to every
# rule that survives it. `run=` is the git seam -- the same reason mutset takes `git=` and
# mutate.py takes `tool=`: the behaviour under test is what the tool does when git says the
# template moved, and that cannot be made to happen by waiting for a commit.
# --------------------------------------------------------------------------------------


def _box():
    """A directory that looks like the repo: templates, static, and an empty artifact dir.

    No git and no browser: every rule below answers what the tool does when git says a source
    moved, and that is driven through `run=` rather than by waiting for a commit.
    """
    import tempfile
    box = tempfile.mkdtemp(prefix="plock-manifest-")
    for rel in list(SHARED_SOURCES) + [f"internal/web/templates/{a}.tmpl"
                                       for a in ("dashboard", "users", "profiles", "audit",
                                                 "settings", "lock", "testlock")]:
        full = os.path.join(box, rel)
        os.makedirs(os.path.dirname(full), exist_ok=True)
        with open(full, "w", encoding="utf-8") as fh:
            fh.write("x\n")
    shots = os.path.join(box, "docs", "screenshots", "360")
    os.makedirs(shots, exist_ok=True)
    return box


def _png(box, name, body=b"png"):
    with open(os.path.join(box, "docs", "screenshots", "360", name), "wb") as fh:
        fh.write(body)


def _fake_git(rev="a" * 40, dirty=()):
    """A git that says `rev` is the newest commit and `dirty` is uncommitted."""

    def run(args, cwd=None):
        if args[0] == "status":
            # Raw porcelain, leading space and all -- what `git` hands back for real. A fake that
            # returned the tidied version would hide exactly the parsing bug this file shipped
            # with, where the status column was sliced at an offset and the path lost a letter.
            return "\n".join(f" M {p}" for p in dirty)
        if args[0] == "log":
            return rev
        return "HEAD"
    return run


class _Page:
    def __init__(self, url, artifact):
        self.url, self.artifact = url, artifact


DRIVER = [_Page("/", "dashboard"), _Page("/users", "users"), _Page("/profiles", "profiles"),
          _Page("/audit", "audit"), _Page("/settings", "settings")]


def self_test():
    global ROOT
    ran, bad = [], []

    def rule(name, complaint):
        ran.append(name)
        if complaint:
            bad.append(f"{name}: {complaint}")

    def in_box(fn):
        """Run `fn(box)` with ROOT pointed at a fresh box, and hand back its complaint."""
        global ROOT
        box = _box()
        saved = ROOT
        ROOT = box
        try:
            return fn(box)
        finally:
            ROOT = saved

    def drive(setup=None, run=None, pages=DRIVER, verb="check"):
        """Run one verb over a fresh box. Returns the complaint list, or a harness error."""

        def go(box):
            if setup:
                setup(box)
            d = os.path.join(box, "docs", "screenshots", "360")
            if verb == "check":
                return check(d, run=run or _fake_git(), pages=pages)
            record(d, run=run or _fake_git(), now="2026-01-01T00:00:00Z")
            return None
        return in_box(go)

    def with_manifest(box):
        for a in ("dashboard", "users", "profiles", "audit", "settings"):
            _png(box, a + ".png")
        record(os.path.join(box, "docs", "screenshots", "360"), run=_fake_git(),
               now="2026-01-01T00:00:00Z")

    def one_png(box):
        _png(box, "profiles.png")

    def stray_png(box):
        with_manifest(box)
        _png(box, "secrets.png")

    def swapped_png(box):
        with_manifest(box)
        _png(box, "profiles.png", body=b"something else entirely")

    def emptied(box):
        d = os.path.join(box, "docs", "screenshots", "360")
        os.makedirs(d, exist_ok=True)

    def manifest_only(box):
        d = os.path.join(box, "docs", "screenshots", "360")
        os.makedirs(d, exist_ok=True)
        with open(os.path.join(d, MANIFEST), "w", encoding="utf-8") as fh:
            json.dump({"schema": SCHEMA, "viewport": ARTIFACT_WIDTH, "pages": []}, fh)

    def wrong_width(box):
        with_manifest(box)
        p = os.path.join(box, "docs", "screenshots", "360", MANIFEST)
        doc = json.load(open(p, encoding="utf-8"))
        doc["viewport"] = 1024
        json.dump(doc, open(p, "w", encoding="utf-8"))

    def says(name, problems):
        return None if any(name in p for p in problems) else f"no problem named {name!r}"

    rule("a tree whose PNGs have no manifest is refused",
         says("no manifest", drive(one_png)))
    rule("an empty artifact directory is refused rather than passed",
         says("no screenshots", drive(emptied)))
    rule("a manifest with no pages in it is refused",
         says("never been committed as evidence", drive(manifest_only)))
    rule("a consistent tree passes, and says what it checked", drive(with_manifest))
    rule("a PNG that is not the file the manifest recorded is refused",
         says("replaced without", drive(swapped_png)))
    rule("a PNG committed with no entry is refused",
         says("no entry in", drive(stray_png)))
    rule("a source with an uncommitted change retires the photograph",
         says("uncommitted change",
              drive(with_manifest, run=_fake_git(dirty=["internal/web/templates/page.tmpl"]))))
    rule("a newer commit over a source retires the photograph",
         says("predates the newest change",
              drive(with_manifest, run=_fake_git(rev="b" * 40))))
    rule("a page the driver photographs and the manifest omits is refused",
         says("never been committed as evidence",
              drive(with_manifest, pages=DRIVER + [_Page("/secrets", "secrets")])))
    rule("a manifest recorded at another viewport is refused",
         says("not 360", drive(wrong_width)))

    # The status parser, on the two shapes that are not a plain modification. Both are the same
    # class of bug as the one this file shipped with: the sentence names a path, and the path is
    # wrong, and nothing else in the check notices.
    rule("a renamed source is named by where it went, not by the arrow",
         None if dirty_sources(["a.tmpl"], run=lambda a, cwd=None: "R  old/a.tmpl -> a.tmpl")
         == ["a.tmpl"] else "the rename line was not parsed into the file that exists now")
    def git_keeps_the_status_column():
        """The real `git()`, over a child that answers like git does.

        This is the bug this file shipped with: `git()` called a bare `strip()`, which took the
        leading space off ` M path`, and the parser that counted three characters then named the
        source as "nternal/web/static/app.css". The fake above cannot see it -- it hands the
        parser what it is told to -- so this drives the function that talks to the child.
        """
        import subprocess as sp
        saved = sp.run

        class _Out:
            returncode, stdout, stderr = 0, " M internal/a.tmpl\n", ""

        sp.run = lambda *a, **k: _Out()
        try:
            got = git(["status", "--porcelain", "--", "internal/a.tmpl"])
            if got != " M internal/a.tmpl":
                return f"git() returned {got!r}; the status column is what says WHICH file changed"
            found = dirty_sources(["internal/a.tmpl"])
            if found != ["internal/a.tmpl"]:
                return f"dirty_sources read that line as {found!r}"
        finally:
            sp.run = saved
        return None

    rule("git() hands the status line over whole, and the parser names the file",
         git_keeps_the_status_column())

    # The record side, because a manifest nobody can produce is a gate that ships once.
    def recorded(box):
        _png(box, "profiles.png")
        record(os.path.join(box, "docs", "screenshots", "360"), run=_fake_git(),
               now="2026-01-01T00:00:00Z")
        doc = read_manifest(os.path.join(box, "docs", "screenshots", "360"))
        if doc["viewport"] != ARTIFACT_WIDTH or doc["captured_at"] != "2026-01-01T00:00:00Z":
            return "the manifest does not carry the viewport and timestamp it was handed"
        e = doc["pages"][0]
        if e["page"] != "/profiles" or e["sources"][0] != "internal/web/templates/profiles.tmpl":
            return f"profiles.png was recorded as {e['page']} from {e['sources']}"
        if not any(src.endswith("page.tmpl") for src in e["sources"]):
            return "the shared shell is missing from the source set, so a layout change would " \
                   "leave every other photograph looking current"
        return None

    rule("record writes the entry the check reads back", in_box(recorded))

    def recorded_then_checked(box):
        _png(box, "profiles.png")
        record(os.path.join(box, "docs", "screenshots", "360"), run=_fake_git(),
               now="2026-01-01T00:00:00Z")
        d = os.path.join(box, "docs", "screenshots", "360")
        return [p for p in check(d, run=_fake_git(), pages=[_Page("/profiles", "profiles")])]

    rule("what record writes, check accepts on the same tree",
         in_box(recorded_then_checked))

    # A PNG named after a page that has no template cannot be recorded. It is a refusal rather
    # than an entry with a missing source, because the alternative is a manifest that certifies a
    # photograph of nothing.
    def orphan(box):
        _png(box, "secrets.png")
        try:
            record(os.path.join(box, "docs", "screenshots", "360"), run=_fake_git())
        except SystemExit as exc:
            return None if "does not exist" in str(exc) else f"refused for the wrong reason: {exc}"
        return "record certified a PNG whose template does not exist"

    rule("a PNG named after a page with no template cannot be recorded", in_box(orphan))

    # Controls: each proves the rule above it can still fail. Without them a rule that always
    # returns None is indistinguishable from a rule that always passes.
    def control_hollow():
        """The canary: a rule whose complaint is a constant must be reported as broken."""
        got = says("anything at all", [])
        return None if got else "the harness reported a missing problem as a pass"

    rule("canary: a rule that looks for a sentence and finds none reports it",
         control_hollow())
    rule("control: a consistent tree is not refused by the swap check",
               None if not any("replaced without" in p for p in drive(with_manifest))
               else "the swap check fires on a tree it was handed clean")

    # The gate has to be pinned to the recipe that runs it, or the cheapest way to pass it is to
    # delete the line. This reads the Makefile this file ships in -- see _MAKEFILE above.
    rule("the Makefile still runs this check inside `make verify`",
         verify_clause_problems(_MAKEFILE))
    rule("control: a Makefile with the clause deleted is caught by the pin",
         None if verify_clause_problems('verify:\n\t@set -e; \\\n\techo "==> gofmt"; \\\n')
         else "the pin did not notice the clause being deleted")

    EXPECTED = 19
    if len(ran) != EXPECTED:
        bad.append(f"expected {EXPECTED} rules, ran {len(ran)} -- one was deleted, or never "
                   f"reached (a `return` in self_test() skips everything below it)")
    if bad:
        print("capture manifest self-test FAILED:")
        for p in bad:
            print(f"  {p}")
        return 1
    print(f"capture manifest self-test: {len(ran)} rules hold (no browser, no git, no writes "
          f"outside a temp box)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
