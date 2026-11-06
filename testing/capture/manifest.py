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
      "schema": "capture-manifest/2",
      "viewport": 360,
      "head": "<HEAD at capture>",
      "captured_at": "<UTC>",
      "pages": [{"page": "/profiles", "png": "profiles.png", "png_sha256": "...",
                 "sources": [...], "sources_rev": "<commit>",
                 "capture": {"fixture": "pages", "stamp": "2026-03-18T21:34:05-04:00",
                             "tz": "America/New_York", "rows": 8, "scripts": "off",
                             "census": "testing/capture/bin",
                             "state": [[row, cell], ...]}}]
    }

`sources_rev` is `git log -1 --format=%H` over the files that page is rendered from, and the check
recomputes it. A commit that touches a template or the stylesheet therefore retires the
photographs of every page that reads them -- which is the truth, and the reason the shared layout
and `app.css` are in every page's source set rather than in none.

The `capture` block (#211) is the other half of the claim. `sources_rev` answers "which commit does
this PNG depict" and nothing about **how the frame was taken**, so a reader with the manifest in
front of them cannot tell a frame taken at the fixture's declared instant, with page scripts off and
the fixture's own census, from one taken off a host clock with the scripts running. #207 was exactly
that inference: `manifest.py check` was never wrong, it was asked a narrower question than anyone
heard. Each entry therefore carries what the fixture that photographed it *declared* --
`record --fixture-env runs/<name>/fixture.env`, repeatable, one call per fixture, each scoped to the
pages that fixture photographs -- and a key that is absent means "this fixture declared nothing about
it", never a guessed default. `state` is read out of the driver's `Page.state` rather than typed here,
for the same reason `pages_by_artifact()` is a projection: a second copy of a claim is a second thing
to forget to update.

Two things the check refuses that a "compare the dates" design would miss:

  - **mtimes are not evidence.** A checkout's mtimes are the checkout's, not the capture's; a
    clone made today would find every PNG "newer" than every template. The comparison is against
    git history, which is the only clock here that means what it says.
  - **an uncommitted edit is the common case.** `git log` cannot see a template you changed five
    minutes ago, so the working tree is asked as well. That is what makes `make verify` refuse
    while you are still holding the edit, instead of after it has merged.

Run it two ways:

    python3 testing/capture/manifest.py record --dir docs/screenshots/360 \
        --fixture-env testing/capture/runs/pages/fixture.env \
        --fixture-env testing/capture/runs/pages-noprof/fixture.env \
        --fixture-env testing/capture/runs/lock/fixture.env \
        --fixture-env testing/capture/runs/testlock/fixture.env       # after a capture
    python3 testing/capture/manifest.py check                               # in `make verify`
    python3 testing/capture/manifest.py --self-test                         # the contract

Without `--fixture-env` a `record` still writes what it always wrote -- a manifest with no `capture`
blocks, and schema `capture-manifest/1` unless the manifest it is replacing already declared `2`. It
keeps the schema of what it replaces on purpose: a capture run's own record step (pages.py, which
knows its fixture and not the other three) must not be able to retire a `/2` manifest down to a `/1`
one and have `check` go quiet about conditions it used to insist on. The schema is what says whether
this file carries conditions, so nothing but a `--fixture-env` run may widen or narrow that claim.
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
# `/2` is the shape that carries a `capture` block (#211). `/1` is still READ -- nine frames on
# `main` were recorded before this field existed, and refusing them would be a refusal about a
# format, not about a photograph. It is never WRITTEN by a run that names its fixture: an entry that
# declares nothing is what #207 was, and a freshly recorded manifest must not be allowed to say
# nothing. Hence two names rather than one, and `check` accepting both while only insisting on the
# block for the schema that knows about it.
SCHEMA = "capture-manifest/2"
SCHEMA_V1 = "capture-manifest/1"
SCHEMAS = (SCHEMA, SCHEMA_V1)
ARTIFACT_WIDTH = 360

# The fixture's declared conditions, and the key each one is recorded under. This is an ALLOWLIST
# read from `fixture.env`, not a copy of it: that file also holds `PLOCK_CAPTURE_PASS`, in cleartext,
# because a throwaway config needs an admin credential the same way a real one does (#see the
# security note in testing/capture/README.md). The manifest is COMMITTED. A recorder that copied the
# env through would put this repository's evidence and someone's password in the same file, and the
# password would be the part that never came out.
#   env key            -> capture key   how it is read
CAPTURE_KEYS = (("PLOCK_CAPTURE_STAMP", "stamp", "instant"),
                ("PLOCK_CAPTURE_TZ", "tz", "text"),
                ("PLOCK_CAPTURE_AUDIT_ROWS", "rows", "int"),
                ("PLOCK_CAPTURE_SCRIPTS", "scripts", "text"),
                ("PLOCK_CAPTURE_PATH_PREPEND", "census", "path"))

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


def fixtures_from_specs(specs):
    """`{artifact: fixture}` for a list of driver page specs, with the stem rule folded in.

    A spec carrying no `fixture` attribute answers with its own stem. That is not a default invented
    to keep a test green: `lock` and `testlock` are photographed by the run directory of the same
    name, so the stem IS the answer for every artifact the table does not scope, and a driver that
    has not adopted `Page.fixture` yet therefore still gets the truth rather than a guess.
    """
    return {s.artifact: getattr(s, "fixture", None) or s.artifact for s in specs}


def pages_by_artifact():
    """`{png stem: url}` for every artifact the capture driver says it takes.

    The driver is the only thing that knows what each photograph depicts, and it already declares
    both halves -- `Page("/profiles", "profiles-refused", ...)` -- so this reads the pair rather
    than re-deriving one from the other. Same import seam as `driver_pages()`, so there is still
    exactly one list of pages in this repository.
    """
    return {spec.artifact: spec.url for spec in driver_pages()}


def fixtures_by_artifact():
    """`{png stem: fixture}` for every artifact the driver declares (#211).

    The third projection out of `PAGES`, beside `pages_by_artifact()`, and with the same reason for
    existing: the nine committed frames come from four fixtures (`pages`, `pages-noprof`, `lock`,
    `testlock`), and a recorder that could not say which fixture photographed which frame could only
    have stamped every entry with one fixture's conditions. That is not a narrower claim, it is a
    false one -- the `pages` fixture declares eight audit rows and a stubbed census, and neither is
    true of `lock.png`.

    Artifacts the driver does not declare keep the stem rule, exactly as `template_for` does: the
    POST drivers name their run directory after the page they photograph (`lock-setup.sh` writes
    `runs/lock/`, `lock.py` writes `lock.png`), which is the only statement anywhere that ties the
    two together.
    """
    return fixtures_from_specs(driver_pages())


def states_by_artifact():
    """`{png stem: [(row, cell), ...]}` -- the state each artifact is a photograph OF.

    Read out of `Page.state` rather than typed into this file. #211's acceptance names the reason:
    a manifest that quoted the cell text would be a second copy of a claim the driver already owns,
    and the second copy is the one that silently stops agreeing. The self-test refuses any state
    string appearing in this file's own text, which is what makes "impossible to retype" a checked
    property rather than an intention.
    """
    return {spec.artifact: list(spec.state) for spec in driver_pages()}


def fixture_env(path):
    """One fixture's declared conditions, read through the driver's own seam.

    Returns `(fixture name, env)`. The name is the run directory's name, because that is the handle
    the driver uses (`cdp.fixture(name)` reads `<runs dir>/name/fixture.env`) and the value
    `Page.fixture` carries; the file is parsed by the same function the driver parses it with, so the
    manifest and the driver cannot come to disagree about what `fixture.env` says.

    A path that is not `<runs>/<name>/fixture.env` is refused rather than read. The fixture name is
    taken from the directory, so an operator aiming this at `runs/pages/config.json`, or at a copy
    they tidied into `fixture.env.bak`, would otherwise get a `capture` block naming a fixture that
    no setup script ever wrote -- a conditions record with a made-up provenance is worse than none.
    """
    sys.path.insert(0, HERE)
    import cdp
    full = os.path.abspath(path if os.path.isabs(path) else os.path.join(ROOT, path))
    name = os.path.basename(os.path.dirname(full))
    if os.path.basename(full) != "fixture.env" or not name or name in (".", ".."):
        raise SystemExit(
            f"{path} is not a fixture env. Name the file the driver reads, "
            f"<runs>/<fixture>/fixture.env, so the recorded fixture name is the run directory's and "
            f"not something this command invented.")
    try:
        env = cdp.fixture_env_at(full)
    except FileNotFoundError:
        raise SystemExit(f"{full} does not exist; run {name}-setup.sh first -- the manifest records "
                         f"the conditions a fixture declared, and a fixture that was never set up "
                         f"declared nothing.")
    return name, env


def repo_relative(value):
    """An absolute declared path, by its location inside the repository when it is inside it.

    `PLOCK_CAPTURE_PATH_PREPEND` names a directory on the machine that took the photographs, and the
    manifest is committed. Recorded verbatim it puts `/home/whoever/prj/wt/<worktree>/...` into the
    evidence -- a fact about the capture box's home directory and worktree name, not about any frame,
    which is the same over-trust #211 is about. A path outside the repository is kept exactly as
    declared, because that is the condition and there is nothing truer to say about it.

    A *relative* declaration is returned untouched, and that is not laziness: a relative path is
    relative to the daemon's working directory, which is not this file's working directory and not
    necessarily the repository's. Resolving it here would be a guess about where the daemon was
    started, and the guess would be written into committed evidence. As it happens `pages-setup.sh`
    declares this key as `testing/capture/bin`, which is already the answer.
    """
    if not os.path.isabs(value):
        return value
    here = os.path.realpath(ROOT)
    there = os.path.realpath(value)
    if there == here or there.startswith(here + os.sep):
        return os.path.relpath(there, here)
    return value


def conditions(name, env):
    """The fixture half of one artifact's `capture` block: only what this fixture declared.

    An absent env key means an absent capture key. That is the whole design: `pages-noprof` says
    nothing about row counts because its page has no table, and recording `rows: 0` for it would be a
    number nobody declared -- the same mistake as the `scroll_width` this file already refuses, one
    field wearing a measurement's name.

    `stamp` is the instant the fixture says its trail was stamped at -- the pair
    `PLOCK_CAPTURE_STAMP` and `PLOCK_CAPTURE_ZONE`, the same two halves `cdp.declared_stamp`
    concatenates and validates. It is what the fixture *declared*, not a measurement of what the
    browser did, and a fixture with no trail still declares the clock it handed the daemon: `rows` is
    absent for those, and the two keys together are what tells a reader which case an entry is. A pair
    the driver would refuse to boot on is refused here rather than committed as evidence.

    The page half -- `state` -- is added by the caller, because it belongs to the artifact and not to
    the fixture: one fixture photographs six pages and each declares its own cells.
    """
    import datetime
    block = {"fixture": name}
    stamp, zone = env.get("PLOCK_CAPTURE_STAMP", ""), env.get("PLOCK_CAPTURE_ZONE", "")
    for env_key, key, kind in CAPTURE_KEYS:
        value = env.get(env_key, "")
        if not value:
            continue
        if key == "stamp":
            if not zone:
                raise SystemExit(
                    f"fixture {name!r} declares PLOCK_CAPTURE_STAMP={stamp!r} with no "
                    f"PLOCK_CAPTURE_ZONE, so there is no instant to record. Name both, as the driver "
                    f"insists on, or neither.")
            try:
                datetime.datetime.fromisoformat(stamp + zone)
            except ValueError as exc:
                raise SystemExit(
                    f"fixture {name!r} declares {stamp!r} with {zone!r}, which is not an instant "
                    f"({exc}). The driver refuses to photograph that fixture; if the driver has "
                    f"changed, this manifest would be recording an unreadable clock.")
            block["stamp"] = stamp + zone
            continue
        if kind == "int":
            try:
                block[key] = int(value)
            except ValueError:
                raise SystemExit(f"fixture {name!r} declares {env_key}={value!r}, which is not a "
                                 f"number of rows. The manifest records it as a count; the driver "
                                 f"refuses a page that renders any other.")
            continue
        if kind == "path":
            block[key] = repo_relative(value)
            continue
        block[key] = value
    return block


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


def record(directory, head=None, run=git, now=None, envs=None):
    """Write the manifest for the PNGs sitting in `directory`, then return it.

    Called by `pages.py` on the path that has already published its staged files. It is not
    called on the refusal path, and it must not be: a manifest written over a run that refused
    would certify photographs this run did not take.

    `envs` is the list of fixtures that photographed this directory, as `(name, env)` pairs from
    `fixture_env()`, and it is what turns "current" into "current, and taken like this" (#211).
    Passing it is a claim about the WHOLE directory, so both directions refuse:

      * a PNG none of the passed fixtures photographs -- its `capture` block would be missing, and a
        newly recorded entry that declares nothing is the shape #207 was;
      * a passed fixture that photographs nothing in this directory -- the wrong env, or the wrong
        directory. A silent no-match is how a manifest acquires another run's provenance, and the
        operator would never be told which of the two was wrong.

    With no `envs` this writes what it always wrote and no `capture` block appears anywhere. The
    schema then follows the file it replaces instead of the newest shape this program can write: a
    `/2` manifest whose conditions have gone must stay visibly `/2`, so `check` keeps refusing it,
    rather than being quietly retired into a `/1` file that asserts nothing and passes. The capture
    drivers call this with no `envs` -- each knows its own fixture and not the other three -- so that
    downgrade is a run anyone can type.
    """
    entries = []
    declared = pages_by_artifact()
    fixtures = fixtures_by_artifact()
    states = states_by_artifact()
    blocks = {name: conditions(name, env) for name, env in (envs or [])}
    used = set()
    for name in pngs_in(directory):
        artifact = name[:-len(".png")]
        page = declared.get(artifact)
        sources = sources_for(artifact, page)
        missing = [s for s in sources if not os.path.exists(os.path.join(ROOT, s))]
        if missing:
            raise SystemExit(f"{name}: its manifest cannot be written, because {missing[0]} "
                             f"does not exist. A PNG named after a page with no template is not "
                             f"evidence about anything.")
        entry = {
            "page": page if page is not None
            else ("/" if artifact == "dashboard" else "/" + artifact),
            "png": name,
            "png_sha256": sha256(os.path.join(directory, name)),
            "sources": sources,
            "sources_rev": newest_commit(sources, run=run),
        }
        # The stem rule for the two POST artifacts, beside the driver's table for the seven it
        # declares: `runs/lock/fixture.env` is what `lock.py` read, and nothing else says so.
        fixture = fixtures.get(artifact, artifact)
        if fixture in blocks:
            used.add(fixture)
            block = dict(blocks[fixture])
            if states.get(artifact):
                block["state"] = [list(pair) for pair in states[artifact]]
            entry["capture"] = block
        entries.append(entry)

    if blocks:
        silent = [e["png"] for e in entries if "capture" not in e]
        if silent:
            raise SystemExit(
                f"{len(silent)} PNG(s) here belong to a fixture you did not name: "
                f"{', '.join(silent)}. --fixture-env was passed, so every entry in this directory "
                f"must record the conditions it was taken under, and these would record nothing -- "
                f"which is what #207 was. The driver's table names "
                f"{', '.join(sorted(set(fixtures.values())))}; a frame it does not declare is "
                f"photographed by the run directory of the same name (runs/lock takes lock.png). "
                f"Pass one env per fixture that contributed a frame.")
        spare = sorted(set(blocks) - used)
        if spare:
            raise SystemExit(
                f"fixture(s) {', '.join(repr(n) for n in spare)} photographed no PNG in "
                f"{directory}, so their conditions would be recorded against nothing. Either the env "
                f"belongs to a different directory or --out sent a capture somewhere else; a fixture "
                f"whose pages are all absent is not a spare flag.")

    previous = read_manifest(directory)
    schema = SCHEMA
    if not blocks and (not previous or previous.get("schema") != SCHEMA):
        schema = SCHEMA_V1
    doc = {"schema": schema, "viewport": ARTIFACT_WIDTH,
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


# The other two files the `capture` block depends on, read the same way (#211). Neither is this
# file's code, so neither can be driven from here: `capture_env` is bash and the driver's refusal is
# inside `cdp.Capture.__enter__`, which needs a browser and a daemon. They are pinned by their text
# because the failure worth catching is one half of the contract being deleted, and the shape of
# that failure is a green gate over a condition nobody honours -- the same reason the Makefile clause
# below is pinned rather than trusted.
try:
    with open(os.path.join(HERE, "fixture.sh"), encoding="utf-8") as _fh:
        _FIXTURE_SH = _fh.read()
except OSError:
    _FIXTURE_SH = ""
try:
    with open(os.path.join(HERE, "cdp.py"), encoding="utf-8") as _fh:
        _CDP = _fh.read()
except OSError:
    _CDP = ""


def condition_contract_errors(fixture_sh=None, cdp_py=None):
    """Is the `scripts` condition still written by the fixture AND honoured by the driver?

    `capture` records what a fixture declared. That is only evidence while the thing that declared it
    and the thing that photographs it agree about the word: `capture_env` must put it in `fixture.env`
    (or every future frame records nothing about the one condition that changed the pixels), and the
    driver must refuse a fixture that declares a condition it does not apply (or the manifest records
    a condition nobody honoured, which is a claim with a hash under it and no measurement).
    """
    sh = _FIXTURE_SH if fixture_sh is None else fixture_sh
    cdp = _CDP if cdp_py is None else cdp_py
    problems = []
    for name, text in (("fixture.sh", sh), ("cdp.py", cdp)):
        if not text:
            problems.append(f"{name} could not be read, so nothing here can promise the `scripts` "
                            f"condition is still declared and honoured (#211)")
    if problems:
        return problems
    if 'echo "PLOCK_CAPTURE_SCRIPTS=$CAPTURE_SCRIPTS"' not in sh:
        problems.append("`capture_env` no longer writes PLOCK_CAPTURE_SCRIPTS into fixture.env, so "
                        "every frame recorded from now on declares nothing about the page's scripts "
                        "-- the one condition a reader cannot recover from the code, because the "
                        "committed frames and the code that takes them now disagree about it")
    if "PLOCK_CAPTURE_SCRIPTS" not in cdp or '!= "off"' not in cdp:
        problems.append("cdp.py no longer refuses a fixture that declares PLOCK_CAPTURE_SCRIPTS "
                        "other than `off`, so the manifest would record a condition the driver does "
                        "not apply -- a fixture could assert scripts=on, be photographed with them "
                        "off, and the entry would testify to the run that did not happen")
    return problems


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

    # Both schemas are readable; only `/2` is insisted on. The nine frames on `main` predate the
    # `capture` field, and a refusal that is really about a format would send a reader to re-take
    # photographs that are current. What `check` will not do is treat a `/2` entry with no conditions
    # as if it were a `/1` one: `/2` is the schema that says "this file records how the frame was
    # taken", so an entry under it that says nothing is the omission #211 was filed for.
    declares_conditions = doc.get("schema") == SCHEMA
    if doc.get("schema") not in SCHEMAS:
        problems.append(f"{MANIFEST} declares schema {doc.get('schema')!r}, want one of "
                        f"{', '.join(SCHEMAS)}")
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
    # Which fixture photographed which frame, from the driver's table -- or from the list the caller
    # was already going to hand the completeness loop below, so the two rules cannot disagree about
    # what this directory is supposed to hold.
    specs = pages if pages is not None else driver_pages()
    fixtures = fixtures_from_specs(specs)
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
        artifact = name[:-len(".png")]
        # The two #211 rules, before anything is said about the file's bytes: a frame can be perfectly
        # current against its sources and still be an undocumented frame, and "which commit" was never
        # the question that went wrong.
        block = e.get("capture")
        if declares_conditions and not (isinstance(block, dict) and block.get("fixture")):
            problems.append(
                f"{name} is recorded under {SCHEMA} with no capture block: the manifest says which "
                f"commit {page} was rendered from and nothing about how the frame was taken -- no "
                f"fixture, no declared instant, no row count, no statement about the page's scripts. "
                f"An undeclared capture is what made #207 possible, so a newly recorded manifest may "
                f"not contain one. Re-record it: python3 testing/capture/manifest.py record "
                f"--fixture-env testing/capture/runs/<fixture>/fixture.env, one flag per fixture "
                f"that photographed this directory.")
        elif isinstance(block, dict) and block.get("fixture"):
            want = fixtures.get(artifact, artifact)
            if block["fixture"] != want:
                problems.append(
                    f"{name} records the conditions of fixture {block['fixture']!r}, which is not the "
                    f"fixture that photographs it: the driver's table says {want!r}. Those conditions "
                    f"were stamped from the wrong run directory -- its clock, its row count and its "
                    f"census describe a document this page was not rendered from.")
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
    for spec in specs:
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
    # Repeated once per fixture, because the committed set is photographed by four of them and a
    # single flag would either apply one fixture's clock to all nine frames or be a lie about seven.
    # A relative path resolves against the repository, the same rule `--out` follows in cdp.py: an
    # operator typing this from testing/capture and one typing it from the root must aim at the same
    # file, and the last time that was left to the shell's directory a whole set of PNGs landed
    # somewhere the ignore rule did not cover.
    ap.add_argument("--fixture-env", action="append", default=[], metavar="PATH",
                    help="runs/<fixture>/fixture.env, one flag per fixture that photographed --dir; "
                         "each entry is stamped with the conditions that fixture declared (#211)")
    ap.add_argument("--self-test", action="store_true")
    a = ap.parse_args(argv)
    if a.self_test:
        return self_test()
    if a.verb == "record":
        envs = [fixture_env(p) for p in a.fixture_env]
        doc = record(a.dir, envs=envs)
        print(f"recorded {len(doc['pages'])} page(s) at {doc['viewport']} px into "
              f"{manifest_path(a.dir)}")
        if envs:
            print(f"  {sum(1 for e in doc['pages'] if 'capture' in e)} of them under the conditions "
                  f"{doc['schema']} means: {', '.join(sorted(n for n, _ in envs))}")
        else:
            print(f"  no capture conditions recorded ({doc['schema']}), so this file answers \"which "
                  f"commit does each PNG depict\" and not \"how was it taken\". Pass --fixture-env "
                  f"once per fixture and it answers both (#211).")
        return 0
    problems = check(a.dir)
    if problems:
        print(f"screenshot manifest: {len(problems)} problem(s):")
        for p in problems:
            print(f"  {p}")
        return 1
    doc = read_manifest(a.dir)
    # The sentence a green gate prints is itself a claim, so it has to carry what is missing from the
    # file as well as what the check proved. Nine frames on `main` were recorded before the `capture`
    # field existed and `check` is deliberately silent about that; a summary that read "9 photographs
    # current" over them would be #211's complaint, printed by the gate that noticed it.
    plain = sum(1 for e in doc.get("pages", []) if not e.get("capture"))
    line = (f"screenshot manifest: {len(doc['pages'])} photographs current against their sources "
            f"(each pinned to {doc['viewport']} px and to the commit its page was rendered from")
    if plain:
        line += (f"; {plain} of them record no capture conditions, so nothing in this file says under "
                 f"what clock, census or script setting they were taken (#211)")
    print(line + ")")
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
        """Record one rule's verdict.

        A rule may hand in a CALLABLE instead of a complaint, and it is run here. That exists because
        a refusal is how this file says "no", and when one escapes a rule's own `try` the whole report
        ended at that line: the other thirty-five verdicts were never named. Measured 6 Nov, mutating
        ten of #211's rules one at a time -- three of the ten came back as a traceback or an early
        refusal and stopped there, so the report could not tell a one-rule defect from a broken
        harness. The refusal is still reported in full; what changed is that the rest of the report
        survives it. (The older rules pass their complaint directly and are unchanged.)
        """
        ran.append(name)
        if callable(complaint):
            try:
                complaint = complaint()
            except AssertionError as exc:
                complaint = str(exc)
            except SystemExit as exc:
                complaint = f"the rule died refusing something: {exc}"
        if complaint:
            bad.append(f"{name}: {complaint}")

    # The harness's own canary, run before any rule is recorded and then rolled back. `rule` is the
    # only thing standing between a complaint and the report, and #211 gave it a new job -- running a
    # callable, and turning a `SystemExit` into a verdict -- so a `rule` that quietly dropped what it
    # was handed would now be able to silence twelve rules from one line. That is the same floor #138
    # was about (a guard that stops calling what it guards), arriving from the harness rather than from
    # a check, so it gets the same answer: a probe that fails if the reporting is broken, and prints
    # nothing when it is not.
    def harness_reports_what_it_is_handed():
        """Each of the three shapes a complaint arrives in must reach the report."""
        def dies():
            raise SystemExit("a condition it could not read")
        saved = len(ran), len(bad)
        rule("canary: a complaint handed in is reported", "a direct complaint")
        rule("canary: a callable's complaint is reported", lambda: "a late complaint")
        rule("canary: a rule that dies is reported", dies)
        added = bad[saved[1]:]
        del ran[saved[0]:], bad[saved[1]:]
        if len(added) != 3:
            return (f"the harness reported {len(added)} of 3 problems; `rule` is the only path from a "
                    "complaint to the report, so a rule that drops one turns the tally into a number "
                    "nobody earned")
        for needle in ("a direct complaint", "a late complaint", "a condition it could not read"):
            if not any(needle in b for b in added):
                return f"the harness lost the complaint {needle!r} on its way to the report"
        return None

    rule("canary: the harness reports every complaint it is handed",
         harness_reports_what_it_is_handed())

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

    # ----------------------------------------------------------------------------------------
    # The `capture` block (#211). Everything above asks WHICH COMMIT a frame depicts; these ask HOW
    # it was taken, which is the question #207 was asked only after nine PNGs had been read as
    # current for years. The rules are written against the driver's real `PAGES` table wherever the
    # claim is about the table -- `fixture`, `state` -- because a fake that carries neither would
    # let a projection that stopped reading the table pass as a projection.
    # ----------------------------------------------------------------------------------------

    def runs_env(box, name, **over):
        """`runs/<name>/fixture.env` in the box, as `capture_env` writes it. Returns the path.

        The five keys the driver insists on are here because the recorder reads the same file, and
        `hunter2-hunter2` is here because that file really does hold a cleartext admin credential --
        one rule below is about it never reaching the committed manifest.
        """
        env = {"PLOCK_CAPTURE_USER": "parent", "PLOCK_CAPTURE_PASS": "hunter2-hunter2",
               "PLOCK_CAPTURE_TARGET": "operator", "PLOCK_CAPTURE_PORT": "8501",
               "PLOCK_CAPTURE_CDP": "9333", "PLOCK_CAPTURE_SCRIPTS": "off"}
        env.update(over)
        d = os.path.join(box, "testing", "capture", "runs", name)
        os.makedirs(d, exist_ok=True)
        path = os.path.join(d, "fixture.env")
        with open(path, "w", encoding="utf-8") as fh:
            fh.write("# written by a self-test rule; the recorder reads this and refuses without it\n")
            for k, v in sorted(env.items()):
                if v is None:
                    continue
                fh.write(f"{k}={v}\n")
        return path

    # Everything the `pages` fixture declares. A fixture that declares nothing is a different rule.
    FULL = {"PLOCK_CAPTURE_STAMP": "2026-03-18T21:34:05", "PLOCK_CAPTURE_ZONE": "-04:00",
            "PLOCK_CAPTURE_AUDIT_ROWS": "8", "PLOCK_CAPTURE_PATH_PREPEND": "testing/capture/bin",
            "PLOCK_CAPTURE_TZ": "America/New_York"}

    def shots(box, *stems):
        for s in stems:
            _png(box, s + ".png")

    def art(doc, stem):
        return next(e for e in doc["pages"] if e["png"] == stem + ".png")

    def joined(problems):
        return "; ".join(problems) or None

    def conditions_follow_the_fixture():
        """One env per fixture, and each frame carries only its own fixture's word (#211).

        The failure this refuses is the one the issue names: nine entries, four fixtures, and a
        recorder that could only apply one fixture's conditions to all of them. `pages` declares
        eight audit rows and the stubbed census; neither is a true thing to say about `lock.png`,
        which photographs this machine's real sessions off a fixture that declares no trail at all.
        """
        def go(box):
            shots(box, "dashboard", "users-no-profiles", "lock")
            envs = [fixture_env(runs_env(box, "pages", **FULL)),
                    fixture_env(runs_env(box, "pages-noprof")),
                    fixture_env(runs_env(box, "lock", PLOCK_CAPTURE_STAMP="2026-04-02T09:00:00",
                                         PLOCK_CAPTURE_ZONE="-04:00"))]
            doc = record(os.path.join(box, "docs", "screenshots", "360"), run=_fake_git(),
                         now="2026-01-01T00:00:00Z", envs=envs)
            problems = []
            d = art(doc, "dashboard")["capture"]
            if d.get("fixture") != "pages":
                problems.append(f"dashboard.png was stamped {d.get('fixture')!r}")
            if d.get("rows") != 8 or d.get("scripts") != "off":
                problems.append(f"the pages conditions never reached dashboard.png: {d}")
            if d.get("stamp") != "2026-03-18T21:34:05-04:00":
                problems.append(f"the declared pair was not recorded as the instant it is: "
                                f"{d.get('stamp')!r}")
            n = art(doc, "users-no-profiles")["capture"]
            if n.get("fixture") != "pages-noprof":
                problems.append(f"users-no-profiles.png was stamped {n.get('fixture')!r}")
            if "rows" in n or "census" in n or "tz" in n:
                problems.append(f"another fixture's conditions were stamped onto "
                                f"users-no-profiles.png: {n}")
            k = art(doc, "lock")["capture"]
            if k.get("fixture") != "lock":
                problems.append(f"lock.png is not scoped by the run directory that took it: {k}")
            if k.get("stamp") != "2026-04-02T09:00:00-04:00":
                problems.append(f"lock.png carries the pages clock instead of its own: "
                                f"{k.get('stamp')!r}")
            if "rows" in k:
                problems.append(f"the POST fixture declares no trail and recorded a row count: {k}")
            return joined(problems)
        return in_box(go)

    rule("each frame is stamped with its own fixture's conditions, and no other's",
         conditions_follow_the_fixture)

    def half_the_directory_unexplained():
        """`--fixture-env` is a claim about every frame in the directory, or it is nothing."""
        def go(box):
            shots(box, "users", "users-no-profiles")
            d = os.path.join(box, "docs", "screenshots", "360")
            envs = [fixture_env(runs_env(box, "pages", **FULL))]
            try:
                record(d, run=_fake_git(), now="2026-01-01T00:00:00Z", envs=envs)
            except SystemExit as exc:
                if "users-no-profiles.png" not in str(exc):
                    return f"it refused without naming the frame it could not explain: {exc}"
                if os.path.exists(manifest_path(d)):
                    return "it refused and wrote the manifest anyway"
                return None
            return "a record that could not account for every frame wrote a manifest"
        return in_box(go)

    rule("a frame whose fixture was not named cannot be recorded as if it were",
         half_the_directory_unexplained)

    def a_fixture_that_photographed_nothing():
        """The other direction: a spare `--fixture-env` means the wrong env or the wrong --dir."""
        def go(box):
            shots(box, "users")
            envs = [fixture_env(runs_env(box, "pages", **FULL)),
                    fixture_env(runs_env(box, "testlock"))]
            try:
                record(os.path.join(box, "docs", "screenshots", "360"), run=_fake_git(),
                       now="2026-01-01T00:00:00Z", envs=envs)
            except SystemExit as exc:
                return (None if "testlock" in str(exc)
                        else f"it refused without naming the fixture that matched nothing: {exc}")
            return "a fixture that photographed nothing here was accepted silently"
        return in_box(go)

    rule("a fixture env that matches no frame here is refused", a_fixture_that_photographed_nothing)

    def conditions_are_required_by_the_schema_that_promises_them():
        """`/2` says this file records conditions, so an entry under it that says nothing refuses.

        `/1` stays quiet. The nine frames on `main` were recorded before the field existed, and a
        refusal about a FORMAT would send a reader to re-take photographs that are current -- which is
        why the two schema names exist rather than one with an optional field.
        """
        def go(box):
            shots(box, "users")
            d = os.path.join(box, "docs", "screenshots", "360")
            record(d, run=_fake_git(), now="2026-01-01T00:00:00Z",
                   envs=[fixture_env(runs_env(box, "pages", **FULL))])
            p = manifest_path(d)
            doc = json.load(open(p, encoding="utf-8"))
            if doc["schema"] != SCHEMA:
                return f"a recorded manifest declares {doc['schema']!r}, not {SCHEMA!r}"
            stripped = json.loads(json.dumps(doc))
            stripped["pages"][0].pop("capture")
            json.dump(stripped, open(p, "w", encoding="utf-8"))
            problems = check(d, run=_fake_git(), pages=driver_pages())
            if not any("no capture block" in x for x in problems):
                return f"a /2 entry that declares nothing passed: {problems[:1]}"
            old = json.loads(json.dumps(stripped))
            old["schema"] = SCHEMA_V1
            json.dump(old, open(p, "w", encoding="utf-8"))
            if any("no capture block" in x for x in check(d, run=_fake_git(), pages=driver_pages())):
                return "a frame recorded before this field existed was refused for its format"
            return None
        return in_box(go)

    rule("a newly recorded entry that declares nothing is refused, an old one is not",
         conditions_are_required_by_the_schema_that_promises_them)

    def conditions_from_the_wrong_run():
        """A block naming a fixture that does not photograph this frame is a stamped-in lie."""
        def go(box):
            shots(box, "users")
            d = os.path.join(box, "docs", "screenshots", "360")
            record(d, run=_fake_git(), now="2026-01-01T00:00:00Z",
                   envs=[fixture_env(runs_env(box, "pages", **FULL))])
            p = manifest_path(d)
            doc = json.load(open(p, encoding="utf-8"))
            if any("not the fixture that photographs it" in x
                   for x in check(d, run=_fake_git(), pages=driver_pages())):
                return "the honest attribution was refused"
            doc["pages"][0]["capture"]["fixture"] = "testlock"
            json.dump(doc, open(p, "w", encoding="utf-8"))
            problems = check(d, run=_fake_git(), pages=driver_pages())
            if not any("not the fixture that photographs it" in x for x in problems):
                return f"conditions stamped from another fixture's run passed: {problems[:1]}"
            return None
        return in_box(go)

    rule("conditions stamped from another fixture's run directory are refused",
         conditions_from_the_wrong_run)

    def a_credential_never_reaches_the_committed_manifest():
        """`fixture.env` holds a cleartext admin password; the manifest is committed.

        This is why `record` reads an allowlist of keys rather than the env. A recorder that copied
        the file through would put this repository's evidence and somebody's credential in the same
        JSON, and the credential is the part that never comes out -- `docs/` is where the merge
        goes, and a password in a merged commit is in every clone forever.
        """
        def go(box):
            path = runs_env(box, "pages", **FULL)
            if "hunter2-hunter2" not in open(path, encoding="utf-8").read():
                return "the control is gone: the fixture env no longer holds the credential"
            shots(box, "users")
            record(os.path.join(box, "docs", "screenshots", "360"), run=_fake_git(),
                   now="2026-01-01T00:00:00Z", envs=[fixture_env(path)])
            text = open(manifest_path(os.path.join(box, "docs", "screenshots", "360")),
                        encoding="utf-8").read()
            if "hunter2" in text:
                return "the manifest was written with the fixture's password in it"
            if "PLOCK_CAPTURE_PASS" in text:
                return f"the manifest carries the credential's key name: {text[:200]}"
            return None
        return in_box(go)

    rule("the recorder copies conditions out of fixture.env, never the credential",
         a_credential_never_reaches_the_committed_manifest)

    def the_state_is_a_projection_not_a_transcription():
        """`state` comes from `Page.state`. The cells a frame proves are therefore typed once."""

        table = {s.artifact: [list(x) for x in s.state] for s in driver_pages()}

        def go(box):
            shots(box, "dashboard", "audit")
            doc = record(os.path.join(box, "docs", "screenshots", "360"), run=_fake_git(),
                         now="2026-01-01T00:00:00Z",
                         envs=[fixture_env(runs_env(box, "pages", **FULL))])
            problems = []
            for stem in ("dashboard", "audit"):
                got = art(doc, stem)["capture"].get("state")
                if got != table[stem]:
                    problems.append(f"{stem}: recorded {got}, the driver declares {table[stem]}")
            # The half a projection cannot show by itself: if the cell text were also typed in this
            # file, the copy above could be a copy of the copy. #207's whole family of defects is a
            # second statement of one claim that stopped agreeing with the first.
            here = open(os.path.join(HERE, "manifest.py"), encoding="utf-8").read()
            quoted = sorted({cell for pairs in table.values() for _, cell in pairs
                             if len(cell) > 8 and cell in here})
            if quoted:
                problems.append(f"this file types the cell text it should read from the driver: "
                                f"{quoted}")
            return joined(problems)
        return in_box(go)

    rule("a page's declared state is read from the driver, never retyped here",
         the_state_is_a_projection_not_a_transcription)

    def an_undeclared_condition_stays_absent():
        """No `rows`, no `rows: 0`. The `scroll_width` mistake, refused one field earlier.

        `pages-noprof` declares no trail because its page has no table, and `pages-noprof`-style
        silence about the census is the honest answer for a fixture that never chose one. A default
        here would be a number nobody declared, which is what this file already refuses once.
        """
        def go(box):
            shots(box, "users")
            d = os.path.join(box, "docs", "screenshots", "360")
            record(d, run=_fake_git(), now="2026-01-01T00:00:00Z",
                   envs=[fixture_env(runs_env(box, "pages", PLOCK_CAPTURE_SCRIPTS=None))])
            bare = art(read_manifest(d), "users")["capture"]
            if bare != {"fixture": "pages"}:
                return (f"a fixture that declared nothing but its name recorded {bare}. An absent "
                        "condition has to stay absent, or the manifest is inventing defaults")
            record(d, run=_fake_git(), now="2026-01-02T00:00:00Z",
                   envs=[fixture_env(runs_env(box, "pages"))])
            now_block = art(read_manifest(d), "users")["capture"]
            if now_block != {"fixture": "pages", "scripts": "off"}:
                return f"a declared condition did not reach the entry: {now_block}"
            return None
        return in_box(go)

    rule("a condition the fixture did not declare stays out of the entry",
         an_undeclared_condition_stays_absent)

    def an_unreadable_condition_is_refused():
        """A clock the driver would not boot on cannot become a recorded condition."""
        cases = (({"PLOCK_CAPTURE_STAMP": "2026-13-40", "PLOCK_CAPTURE_ZONE": "-04:00"},
                  "not an instant"),
                 ({"PLOCK_CAPTURE_STAMP": "2026-03-18T21:34:05"}, "PLOCK_CAPTURE_ZONE"),
                 ({"PLOCK_CAPTURE_AUDIT_ROWS": "eight"}, "not a number of rows"))

        def go(box):
            shots(box, "users")
            d = os.path.join(box, "docs", "screenshots", "360")
            problems = []
            for over, needle in cases:
                path = runs_env(box, "pages", **over)
                try:
                    record(d, run=_fake_git(), now="2026-01-01T00:00:00Z",
                           envs=[fixture_env(path)])
                    problems.append(f"{over} was recorded as a condition")
                except SystemExit as exc:
                    if needle not in str(exc):
                        problems.append(f"{over} refused for the wrong reason: {exc}")
            return joined(problems)
        return in_box(go)

    rule("a condition that is not readable is a refusal, not a field",
         an_unreadable_condition_is_refused)

    def a_record_without_conditions_keeps_the_schema_it_found():
        """The capture drivers record with no env, and must not be able to retire a `/2` file.

        Each driver knows its own fixture and not the other three, so `pages.py` still calls `record`
        empty-handed; if that call wrote the newest schema, the file would go on claiming it carries
        conditions while `check` fell silent about entries that no longer have them. Keeping `/2` is
        what makes the loss LOUD: the gate refuses the manifest until a run names its fixtures again.
        """
        def go(box):
            shots(box, "users")
            d = os.path.join(box, "docs", "screenshots", "360")
            record(d, run=_fake_git(), now="2026-01-01T00:00:00Z")
            doc = read_manifest(d)
            problems = []
            if doc["schema"] != SCHEMA_V1:
                problems.append(f"a fresh record declares {doc['schema']!r}, not {SCHEMA_V1!r}")
            if any("capture" in e for e in doc["pages"]):
                problems.append("a record with no fixture env wrote conditions it was not given")
            p = manifest_path(d)
            up = json.loads(json.dumps(doc))
            up["schema"] = SCHEMA
            json.dump(up, open(p, "w", encoding="utf-8"))
            record(d, run=_fake_git(), now="2026-01-02T00:00:00Z")
            after = read_manifest(d)
            if after["schema"] != SCHEMA:
                return (f"the drivers' record step retired {SCHEMA} to {after['schema']!r}: the file "
                        "no longer claims it carries conditions, so the gate went quiet about the "
                        "conditions it lost")
            if not any("no capture block" in x
                       for x in check(d, run=_fake_git(), pages=driver_pages())):
                problems.append("the lost conditions passed the gate anyway")
            return joined(problems)
        return in_box(go)

    rule("a record with no fixture env cannot downgrade a manifest that has conditions",
         a_record_without_conditions_keeps_the_schema_it_found)

    def the_recorder_reads_the_file_the_driver_reads():
        """One parser for `fixture.env`, and the fixture's name is its run directory's."""

        def go(box):
            path = runs_env(box, "pages", **FULL)
            name, env = fixture_env(path)
            problems = []
            if name != "pages":
                problems.append(f"the fixture was named {name!r}, not after its run directory")
            if env.get("PLOCK_CAPTURE_PORT") != "8501":
                problems.append(f"the env was not read as the driver reads it: {sorted(env)}")
            try:
                fixture_env(os.path.join(os.path.dirname(path), "config.json"))
                problems.append("a config.json was accepted as a fixture env, and named a fixture "
                                "no setup script ever wrote")
            except SystemExit as exc:
                if "not a fixture env" not in str(exc):
                    problems.append(f"refused for the wrong reason: {exc}")
            try:
                fixture_env(os.path.join(box, "testing", "capture", "runs", "ghost", "fixture.env"))
                problems.append("a fixture env that is not there was accepted")
            except SystemExit as exc:
                if "setup.sh" not in str(exc):
                    problems.append(f"a missing env should name the setup to run: {exc}")
            sys.path.insert(0, HERE)
            import cdp
            if fixture_env(path)[1] != cdp.fixture_env_at(path):
                problems.append("the recorder and the driver parsed one file to two different dicts")
            return joined(problems)
        return in_box(go)

    rule("the manifest reads fixture.env through the driver's own parser",
         the_recorder_reads_the_file_the_driver_reads)

    def a_path_is_recorded_by_where_it_is_in_the_repository():
        """The census directory is on the capture machine; the manifest is committed (#211).

        Three cases, because the key has three shapes and only one of them is a machine's path. Every
        rule here runs with `ROOT` at a temp box, so a census under the box is a census inside the
        repository and the answer has to be the box-relative one -- which is what the committed file
        has to carry. A census outside it is a real condition of the run and stays as declared, and a
        relative one is left alone because resolving it would mean guessing the daemon's directory.
        """
        def go(box):
            shots(box, "users")
            d = os.path.join(box, "docs", "screenshots", "360")
            cases = [(os.path.join(box, "testing", "capture", "bin"),
                      os.path.join("testing", "capture", "bin"),
                      "a census inside the repository was recorded as {} -- the committed manifest "
                      "names the capture machine's directory and worktree instead of the repository's"),
                     ("/opt/other/census", "/opt/other/census",
                      "a census outside the repository was rewritten to {} instead of being recorded "
                      "as the condition the fixture declared"),
                     ("testing/capture/bin", "testing/capture/bin",
                      "a relative census was resolved against somewhere other than where it was "
                      "written: {}")]
            problems = []
            for declared, want, complaint in cases:
                record(d, run=_fake_git(), now="2026-01-01T00:00:00Z",
                       envs=[fixture_env(runs_env(box, "pages",
                                                  PLOCK_CAPTURE_PATH_PREPEND=declared))])
                got = art(read_manifest(d), "users")["capture"]["census"]
                if got != want:
                    problems.append(complaint.format(got))
            return joined(problems)
        return in_box(go)

    rule("a declared census directory is recorded by its path in the repository",
         a_path_is_recorded_by_where_it_is_in_the_repository)

    # The two halves of the `scripts` condition that live in other files, pinned by their text: an
    # undrivable contract is still a contract, and the way it breaks is one side being edited away
    # while every runnable check downstream keeps passing.
    rule("the fixture still declares the scripts condition and the driver still honours it",
         None if not condition_contract_errors() else "; ".join(condition_contract_errors()))
    rule("control: either half deleted is caught by the pin",
         None if (condition_contract_errors(fixture_sh="") and condition_contract_errors(cdp_py=""))
         and "no longer writes" in "; ".join(condition_contract_errors(fixture_sh="x\n"))
         and "no longer refuses" in "; ".join(condition_contract_errors(cdp_py="x\n"))
         else "the pin does not notice one half being deleted")

    EXPECTED = 40
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
