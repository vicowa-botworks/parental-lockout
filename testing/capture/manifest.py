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


def template_for(artifact, page=None):
    """Which template a photograph is evidence about -- the PAGE decides, not the file name.

    `profiles-refused.png` is the `/profiles` page in the state a refused write answers with. It is
    a second STATE of one page, not a second template, and deriving the template from the stem sent
    `record` looking for `internal/web/templates/profiles-refused.tmpl` and refusing to write the
    manifest: "a PNG named after a page with no template is not evidence about anything" -- true,
    and the wrong conclusion, because the page has a template and the file name does not.

    An artifact the driver does not declare (`lock`, `testlock`, taken by their own drivers) keeps
    the stem rule, which is the only thing known about it.
    """
    if page is None:
        return artifact
    return page.strip("/").split("/")[0] or "dashboard"


def sources_for(artifact, page=None):
    """The files a page's photograph is evidence about: its template, and what wraps it."""
    return [f"internal/web/templates/{template_for(artifact, page)}.tmpl"] + list(SHARED_SOURCES)


def pages_by_artifact():
    """`{png stem: url}` for every artifact the capture driver says it takes.

    The driver is the only thing that knows what each photograph depicts, and it already declares
    both halves -- `Page("/profiles", "profiles-refused", ...)` -- so this reads the pair rather
    than re-deriving one from the other. Same import seam as `driver_pages()`, so there is still
    exactly one list of pages in this repository.
    """
    return {spec.artifact: spec.url for spec in driver_pages()}


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
    """The commit that last touched any of `sources` -- the thing a photograph ages against.

    An EMPTY answer is not "nothing has touched these files". It means no commit this checkout can
    see has, and in a depth-1 clone that is the normal answer for every file in the repository.
    Callers have to tell those apart, which is why this returns "" rather than refusing: the
    refusal belongs where the reader can be told the check did not run.
    """
    return run(["log", "-1", "--format=%H", "--"] + list(sources))


def history_is_shallow(run=git):
    """Is this a shallow clone, so the history the manifest ages against is not here at all?

    `actions/checkout@v4` defaults to `fetch-depth: 1`, and a depth-1 checkout does not merely make
    the clock coarse -- it removes every commit except one, so `newest_commit` answers with that
    one for some files and with nothing for the rest. Reported as drift, that is seven stale
    screenshots on a tree whose screenshots may all be current, and it sends the reader to the
    capture package instead of to the workflow. Same distinction `check-deps.sh` draws about its
    own scanners: an empty result is "did not scan", never "found nothing".
    """
    try:
        return run(["rev-parse", "--is-shallow-repository"]).strip() == "true"
    except SystemExit:
        # A git too old to answer (the flag is 2.15, from 2015). Not shallow as far as this can
        # tell; the empty-history refusal below still catches a real depth-1 checkout, so the
        # verdict stays honest on a machine that cannot answer the cheap question.
        return False


def is_ancestor(older, newer, run=git):
    """Is `older` at or before `newer` in history?

    Answered with `rev-list --count newer..older`, which counts what `older` has that `newer` does
    not: zero means `older` brings nothing the other side lacks, so it is at or before `newer` in
    history and a photograph taken at `newer` saw the page as of `older`.

    The range, not the `newer ^older` spelling, and the difference is the whole answer: `--count
    newer ^older` is zero for an ANCESTOR and also zero for a DESCENDANT, because both have
    nothing the other lacks in that direction. Written that way the check accepts exactly the case
    it exists to refuse -- a page that moved while its photograph stayed put -- and stays green
    doing it. Measured: with the caret spelling, the drift rule below stops firing.

    Not `merge-base --is-ancestor` either, whose answer IS a non-zero exit code -- and `git()`
    turns a non-zero exit into a refusal, correctly, for every command where silence would be a
    lie. A helper that raised on "no, it is not an ancestor" would be worse than useless here,
    because "no" is the finding this file exists to print.
    """
    if not older or not newer or len(older) < 7 or len(newer) < 7:
        return False
    return run(["rev-list", "--count", f"{newer}..{older}"]).strip() == "0"


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
    declared = pages_by_artifact()
    for name in pngs_in(directory):
        artifact = name[:-len(".png")]
        page = declared.get(artifact)
        sources = sources_for(artifact, page)
        missing = [s for s in sources if not os.path.exists(os.path.join(ROOT, s))]
        if missing:
            raise SystemExit(f"{name}: its manifest cannot be written, because {missing[0]} "
                             f"does not exist. A PNG named after a page with no template is not "
                             f"evidence about anything.")
        entries.append({
            "page": page if page is not None
            else ("/" if artifact == "dashboard" else "/" + artifact),
            "png": name,
            "png_sha256": sha256(os.path.join(directory, name)),
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

    # Before any per-page claim: a checkout without history cannot produce a verdict at all, and
    # the difference between "these photographs are old" and "this check did not run" is the whole
    # difference between sending someone to the capture package and sending them to the workflow.
    if history_is_shallow(run=run):
        return [f"{MANIFEST} was not checked: this is a shallow clone, so the history that says "
                f"when a page last moved is not in this checkout. Nothing here is a finding about "
                f"the photographs. Fetch the full history (`git fetch --unshallow`; in CI, "
                f"`fetch-depth: 0` on the job that runs `make verify`) and run it again."]

    entries = doc.get("pages", [])
    seen = set()
    for e in entries:
        name = e.get("png", "")
        if not name:
            # Refused rather than allowed to fall through: `os.path.join(directory, "")` is the
            # artifact directory, and hashing a directory raises IsADirectoryError. The gate still
            # exits non-zero on a traceback, but the one manifest-shaped input that should say "an
            # entry has no PNG" would say that in a Python stack trace.
            problems.append(f"an entry in {MANIFEST} names no PNG (page "
                            f"{e.get('page', '?')!r}): nothing to compare the recorded sources "
                            f"against, so the entry certifies nothing")
            continue
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
        sources = e.get("sources") or sources_for(name[:-len(".png")], e.get("page"))
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
        if not current:
            problems.append(f"{MANIFEST} was not checked against {page}'s sources: no commit this "
                            f"checkout can see touches {sources[0]}. That is a history problem, "
                            f"not a photograph problem -- a shallow or filtered fetch says the "
                            f"same thing about every page in the repository. Nothing here is a "
                            f"finding about {name}.")
            continue
        if recorded == current:
            continue
        # The page has moved since the manifest was recorded. That is only drift if the photograph
        # did not move with it, and the first version of this check could not tell the two apart:
        # it compared the manifest's own commit against the page's, so a commit that changed a
        # template AND re-took its photograph -- the one commit in which both are true -- was
        # refused by the very change that made the photograph current, with a sentence that was
        # false about it.
        #
        # So ask the history what it knows instead of what the manifest remembers. The PNG has a
        # newest commit too, and the photograph is current exactly when the page's newest change is
        # at or before the photograph's newest change. The bytes are pinned by `png_sha256`
        # regardless, so accepting the atomic case cannot smuggle in a swapped file; and the
        # failure #144 is about -- the page moved and nobody re-took -- still refuses, because then
        # the PNG's newest commit is genuinely older than the source's.
        shot = newest_commit([os.path.relpath(path, ROOT)], run=run)
        if not shot:
            problems.append(f"{MANIFEST} was not checked against {name}: no commit this checkout "
                            f"can see touches the photograph, so there is nothing to date it "
                            f"against. That is a fetch problem, not a finding about {name}.")
            continue
        if is_ancestor(current, shot, run=run):
            continue
        problems.append(f"{name} predates the newest change to {page}'s sources: the manifest was "
                        f"taken at {str(recorded)[:12]}..., {sources[0]} (or the shared shell) has "
                        f"moved to {str(current)[:12]}..., and the newest commit touching the "
                        f"photograph is {str(shot)[:12]}... -- before that change. The page grew "
                        f"and the photograph did not (#144). Re-take it and re-run "
                        f"`manifest.py record`, in a commit after the one that changed the page "
                        f"or in that same commit; what it may not be is older than the page.")

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


# The commit a manifest recorded against is written with the default fake, so it is a name both
# the fake and the rules below have to agree on.
RECORDED = "a" * 40


def _fake_git(rev=RECORDED, dirty=(), png_rev=RECORDED, shallow=False, visible=True):
    """A git that says `rev` is the newest commit touching the sources and `dirty` is uncommitted.

    `png_rev` is the newest commit touching the photographs, and it defaults to the commit the
    manifest was recorded at: that is what a page that moved without a re-capture looks like, so
    the drift rules below stay drift rules. Set it to `rev` to ask the other question -- a page
    change and its re-take in one commit -- and to a commit after `rev` for a re-take with no page
    change at all.

    `shallow` and `visible` are the two ways a checkout can fail to have the history this check
    reads. They are separate because they refuse differently: a shallow repository is refused up
    front, before any page is looked at, while an invisible source is refused per page.
    """
    # Oldest first. A photograph left at the commit the manifest was recorded at is the older of
    # the two -- that is what drift looks like -- while an explicitly named png_rev is a re-take,
    # which sits after the page's last change.
    history = [rev] if png_rev == rev else (
        [png_rev, rev] if png_rev == RECORDED else [rev, png_rev])

    def run(args, cwd=None):
        if args[0] == "status":
            # Raw porcelain, leading space and all -- what `git` hands back for real. A fake that
            # returned the tidied version would hide exactly the parsing bug this file shipped
            # with, where the status column was sliced at an offset and the path lost a letter.
            return "\n".join(f" M {p}" for p in dirty)
        if args[0] == "log":
            paths = args[args.index("--") + 1:] if "--" in args else []
            if not visible:
                return ""
            if paths and paths[0].endswith(".png"):
                return png_rev
            return rev
        if args[0] == "rev-list":
            # `rev-list --count NEWER..OLDER`: how much OLDER has that NEWER does not. Zero is
            # "older is already behind newer", which is the answer the caller wants.
            newer, older = args[2].split("..")
            if older not in history or newer not in history:
                return "?"
            return str(max(0, history.index(older) - history.index(newer)))
        if args[0] == "rev-parse":
            return "true" if shallow else "false"
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

    def a_second_state_of_one_page_shares_its_template():
        """`profiles-refused` is a STATE of `/profiles`, so its evidence is `profiles.tmpl`.

        The stem was the only rule, and it made a second state of a page unrecordable: `record`
        asked for `profiles-refused.tmpl`, found no such file, and refused to write the manifest at
        all -- "a PNG named after a page with no template is not evidence about anything". True, and
        the wrong conclusion: the page has a template and the file name does not. The artifact #138
        exists to produce was the one this tool could not certify.
        """
        if sources_for("profiles-refused", "/profiles") != sources_for("profiles", "/profiles"):
            return ("the second state does not share the page's source set, so a change to "
                    "/profiles would leave its other photograph certified as current")
        if any("profiles-refused" in s for s in sources_for("profiles-refused", "/profiles")):
            return ("the template is still read out of the file name, so `record` refuses a second "
                    "state of a page instead of manifesting it")
        # The control, and the other half of the rule: `lock.png` and `testlock.png` are taken by
        # their own drivers and are not in the page table. Resolving everything through the page
        # would leave them with no sources at all -- a manifest that checks nothing.
        if not any(s.endswith("lock.tmpl") for s in sources_for("lock")):
            return ("an artifact the driver does not declare no longer resolves to its own "
                    "template, so its photograph is pinned to nothing")
        return None

    rule("a second state of a page shares its template",
         a_second_state_of_one_page_shares_its_template())
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

    # The clock, round 2. Both of these are the check describing the checkout rather than the
    # photographs: a depth-1 fetch and an invisible source each produce a confident sentence about
    # stale screenshots on a tree that may be perfectly current, and the reader follows the sentence
    # to the wrong file. Measured by the reviewer on `refs/pull/174/merge` at depth 1: seven
    # refusals, all of them about CI's fetch.
    def shallow_checkout(box):
        for a in ("dashboard", "users", "profiles", "audit", "settings"):
            _png(box, a + ".png")
        record(os.path.join(box, "docs", "screenshots", "360"), run=_fake_git(),
               now="2026-01-01T00:00:00Z")
        d = os.path.join(box, "docs", "screenshots", "360")
        # The control first: the same tree with history visible and the page unmoved is current.
        if check(d, run=_fake_git(), pages=DRIVER):
            return "the control: a full checkout with an unmoved page was refused"
        problems = check(d, run=_fake_git(shallow=True, rev="b" * 40), pages=DRIVER)
        if not any("was not checked" in p for p in problems):
            return f"a shallow clone produced no 'did not run' sentence: {problems}"
        if any("predates" in p for p in problems):
            return (f"a shallow clone was reported as stale photographs: {problems}. The whole "
                    "point of the distinction is that the reader should not go looking for a page "
                    "that moved")
        if len(problems) != 1:
            return f"a shallow clone should produce one sentence, not {len(problems)}: {problems}"
        return None

    rule("a shallow checkout is reported as no verdict, not as stale screenshots",
         in_box(shallow_checkout))

    def invisible_sources(box):
        for a in ("dashboard", "users", "profiles", "audit", "settings"):
            _png(box, a + ".png")
        record(os.path.join(box, "docs", "screenshots", "360"), run=_fake_git(),
               now="2026-01-01T00:00:00Z")
        d = os.path.join(box, "docs", "screenshots", "360")
        problems = check(d, run=_fake_git(visible=False), pages=DRIVER)
        if not any("history problem" in p for p in problems):
            return f"an empty history produced no 'this did not run' sentence: {problems[:1]}"
        if any("predates" in p for p in problems):
            return f"an empty history was reported as drift: {problems[:1]}"
        # The control: the same manifest against a git that can see the page's last change, and
        # the drift sentence is back -- so the rule above is about visibility and not about the
        # comparison having been deleted.
        moved = check(d, run=_fake_git(rev="b" * 40), pages=DRIVER)
        if not any("predates" in p for p in moved):
            return f"a page that moved without a re-take stopped being refused: {moved[:1]}"
        return None

    rule("a source no commit can be seen to touch is a fetch problem, not drift",
         in_box(invisible_sources))

    # Blocking 2: a page change and its re-taken photograph in one commit used to be refused by the
    # commit that made the photograph current, with a sentence that was false about it.
    def page_and_photograph_together(box):
        for a in ("dashboard", "users", "profiles", "audit", "settings"):
            _png(box, a + ".png")
        record(os.path.join(box, "docs", "screenshots", "360"), run=_fake_git(),
               now="2026-01-01T00:00:00Z")
        d = os.path.join(box, "docs", "screenshots", "360")
        # The page moved to b..., and so did the photograph: one commit carrying both.
        if check(d, run=_fake_git(rev="b" * 40, png_rev="b" * 40), pages=DRIVER):
            return ("a page and its re-take committed together were refused -- the trap that makes "
                    "the honest workflow red")
        # A re-take with no page change is current too (the photograph is newer than the page).
        if check(d, run=_fake_git(rev="b" * 40, png_rev="c" * 40), pages=DRIVER):
            return "a re-take with no page change was refused"
        # And the failure #144 is about still refuses: the page moved, the photograph did not.
        if not any("predates" in p for p in check(d, run=_fake_git(rev="b" * 40), pages=DRIVER)):
            return "the page moved and the photograph did not, and nothing was refused"
        return None

    rule("a page change and its re-take may share a commit", in_box(page_and_photograph_together))

    def entry_with_no_png(box):
        _png(box, "profiles.png")
        d = os.path.join(box, "docs", "screenshots", "360")
        with open(os.path.join(d, MANIFEST), "w", encoding="utf-8") as fh:
            json.dump({"schema": SCHEMA, "viewport": ARTIFACT_WIDTH, "pages": [{}]}, fh)
        problems = check(d, run=_fake_git(), pages=[_Page("/profiles", "profiles")])
        if not any("names no PNG" in p for p in problems):
            return f"an entry with no PNG produced {problems[:1]!r}, want a sentence about the entry"
        return None

    rule("an entry with no PNG is a refusal, not a traceback", in_box(entry_with_no_png))

    # `scroll_width` was the viewport wearing a measurement's name: check() never read it, so the
    # manifest testified to a number nobody took. Now the capture hands it over and a manifest
    # without one says so.
    def no_unmeasured_claims(box):
        _png(box, "profiles.png")
        d = os.path.join(box, "docs", "screenshots", "360")
        record(d, run=_fake_git(), now="2026-01-01T00:00:00Z")
        doc = read_manifest(d)
        if "scroll_width" in doc["pages"][0]:
            return ("the manifest carries scroll_width again. It was the viewport wearing a "
                    "measurement's name, and it cannot be fixed by plumbing: record() is called "
                    "once, by pages.py, which has never measured lock.png or testlock.png -- two "
                    "of the seven entries would still be assertions. A width belongs in the "
                    "capture transcript, where it is measured and refused, not copied into a file "
                    "that can only ever be older than the measurement it claims to record.")
        return None

    rule("the manifest claims nothing it did not measure", in_box(no_unmeasured_claims))

    EXPECTED = 25
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
