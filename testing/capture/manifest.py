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
                 "capture": {"fixture": "pages", "stamp": "2026-03-18T21:34:05",
                             "zone": "-04:00", "tz": "America/New_York", "rows": 8,
                             "census": "testing/capture/bin", "scripts": "off",
                             "state": [["casey", "no profile"]]}}]
    }

`sources_rev` is `git log -1 --format=%H` over the files that page is rendered from, and the check
recomputes it. A commit that touches a template or the stylesheet therefore retires the
photographs of every page that reads them -- which is the truth, and the reason the shared layout
and `app.css` are in every page's source set rather than in none.

The `capture` block (#211) is the second half of "evidence": the same page rendered under a
different clock, zone, row count, session census, or with page scripts on, is a different
photograph. `capture-manifest/1` -- every manifest committed so far -- answers "which page is this"
and nothing about the conditions, so a /1 manifest can be read as current by someone with no way to
know the frame was taken with page scripts running, on the capturing machine's clock, from a census
of whatever `loginctl` reported that day. That is #207's finding, one layer down from the one it
fixed. /1 therefore stays *readable* (a filed document does not become a finding because the tool
grew a field) while /2 promises a block per entry and `check` holds every entry to it.

`schema` describes the file, not the program that wrote it: the value is /2 only when **every**
entry carries a block. The nine committed artifacts come from three drivers and only `pages.py`
records, so a re-take of one page cannot say what `lock.png` was taken under; that directory stays
/1, keeps the blocks it really has, and `check` says how many of the frames state their conditions.
An absent condition is never filled in with a default -- `scripts: "off"` recorded because the driver
disables scripts today would be #211 repeated one layer down. The two POST artifacts are the case study
in doing it twice: their value was inferred first from commit dates and then from nine reproduced
digests, and neither inference could have been checked, because the driver did not read the declaration
until D-0025 and neither page renders anything the page's own scripts could change. Their `off` is true
now because the frames were re-taken (#213) by a driver that honours it -- which is a statement about the
run, the most the field can mean; see the scripts section of testing/capture/README.md.

Two things the check refuses that a "compare the dates" design would miss:

  - **mtimes are not evidence.** A checkout's mtimes are the checkout's, not the capture's; a
    clone made today would find every PNG "newer" than every template. The comparison is against
    git history, which is the only clock here that means what it says.
  - **an uncommitted edit is the common case.** `git log` cannot see a template you changed five
    minutes ago, so the working tree is asked as well. That is what makes `make verify` refuse
    while you are still holding the edit, instead of after it has merged.

Run it two ways:

    python3 testing/capture/manifest.py record --dir docs/screenshots/360 \\
        --fixture-env testing/capture/runs/pages/fixture.env          # after a capture
    python3 testing/capture/manifest.py check                               # in `make verify`
    python3 testing/capture/manifest.py --self-test                         # the contract

`pages.py` passes the env of every fixture it photographed, so an ordinary capture records the
conditions without being told to. `--fixture-env` is for recording a directory the drivers did not
all record themselves -- the committed set, which takes one env per fixture involved -- and omitting
it is what keeps a manifest at /1 rather than making it claim something.
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
# Two schemas, because the difference between them is a fact a reader can use.
#
# `capture-manifest/1` says which page a PNG depicts and what it was rendered from. That is what
# #144 needed and it is still true of every entry written before #211 -- and it is also exactly how
# far behind the capture itself had fallen: a manifest that names no conditions can be read as "this
# PNG is current" by someone with no way to know the frame was taken with page scripts running, under
# a host clock, from a census of whatever machine ran the driver (#207). So /1 stays *readable* -- a
# committed manifest does not become a finding because the tool grew a field -- while /2 adds a
# `capture` block per entry and `check` then requires one.
SCHEMA = "capture-manifest/2"
SCHEMA_PLAIN = "capture-manifest/1"
SCHEMAS_READABLE = (SCHEMA, SCHEMA_PLAIN)
ARTIFACT_WIDTH = 360
# The file a capture run leaves beside its own `fixture.env`, naming the frames it photographed and
# the bytes it wrote for each (`cdp.write_receipt`). `record` reads it from the directory the
# `--fixture-env` it was handed sits in, which is why there is no flag for it: the same file that
# says which fixture took a frame is the one that says that fixture ran at all (#302).
RECEIPT = "capture.json"

# The fixture.env keys that state a CONDITION on the frame, and the field each one takes in a manifest
# entry. Deliberately not the whole env: PLOCK_CAPTURE_USER, _PASS, _TARGET, _PORT and _CDP say how to
# reach a daemon, which is gone the moment the setup script exits, and a password has no business in a
# file that is committed.
#
# A key the fixture did not write produces no field. Absent means "this fixture declared nothing about
# it" and NEVER a default: the whole defect #211 closes is a reader filling in a condition nobody
# stated, and a `scripts: "off"` written by the recorder because the driver disables scripts today would
# be the same mistake one layer down. `cdp.scripts_disabled` is what makes the difference visible: the
# declaration decides the browser now, so an `off` in a block reports a run, not this file's preference.
CONDITION_KEYS = (
    ("PLOCK_CAPTURE_STAMP", "stamp"),
    ("PLOCK_CAPTURE_ZONE", "zone"),
    ("PLOCK_CAPTURE_TZ", "tz"),
    ("PLOCK_CAPTURE_AUDIT_ROWS", "rows"),
    ("PLOCK_CAPTURE_PATH_PREPEND", "census"),
    ("PLOCK_CAPTURE_SESSIONS", "sessions"),
    ("PLOCK_CAPTURE_SCRIPTS", "scripts"),
)
# Values a manifest is allowed to assert about the page's own scripts, read out of the driver that
# acts on them. Written here as well until 6 Nov, and the failure that buys is #211 in miniature: the
# driver renames or adds a spelling, the recorder goes on certifying the old one, and a manifest full
# of a condition nothing can produce stays green. Read the same lazy way as `driver_pages()`, for the
# same reason -- `check` still answers when the browser half cannot be imported.


def script_values():
    """`("on", "off")`, from `cdp.SCRIPT_VALUES` -- the one table of the two spellings."""
    sys.path.insert(0, HERE)
    import cdp
    return cdp.SCRIPT_VALUES

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


def post_captures():
    """`(artifact, fixture)` for the artifacts whose drivers are not `pages.py`.

    `lock.png` and `testlock.png` are photographed by `lock.py` and `testlock.py`, so the page table
    cannot say which fixture their frames are evidence about -- and a `capture` block guessed from the
    file name would attribute the wrong fixture's clock to them, which is the failure this whole field
    exists to prevent. Each of those drivers therefore declares the pair beside the code that uses it
    (`ARTIFACT` and `FIXTURE`, in the same words `Page.fixture` uses).

    Read lazily and forgivingly, in the same spirit as `driver_pages()`: if a driver cannot be
    imported, its artifact simply has no binding, and `record` refuses to invent one rather than
    pretending the question was never asked.
    """
    sys.path.insert(0, HERE)
    pairs = []
    for name in ("lock", "testlock"):
        try:
            mod = __import__(name)
        except ImportError:
            continue
        artifact = getattr(mod, "ARTIFACT", None)
        fixture = getattr(mod, "FIXTURE", None)
        if artifact and fixture:
            pairs.append((artifact, fixture))
    return pairs


def captures_by_artifact():
    """`{png stem: fixture name}` -- which fixture each frame is evidence about.

    One projection of the drivers' own declarations, for the same reason `pages_by_artifact` is a
    projection: a second table of these pairs is a second thing to forget to update, and the update
    that gets forgotten is the one that silently attributes a condition to the wrong frame.
    """
    binding = {spec.artifact: spec.fixture for spec in driver_pages()}
    for artifact, fixture in post_captures():
        binding.setdefault(artifact, fixture)
    return binding


def _source_of(fn):
    """The source of a function in this file, for the rules that can only check a clause is present.

    A rule that calls a function cannot see the caller. Both uses below are CLI wiring -- a flag, and
    the argument that carries it -- whose deletion leaves every behavioural rule green while the
    feature stops existing for the person who runs it from a shell. `inspect` is stdlib, and this
    repository's rule is about third-party dependencies (`scripts/check-deps.sh`, Go side).
    """
    import inspect
    return inspect.getsource(fn)


def read_fixture_env(path):
    """One fixture's declared conditions, as `(fixture name, {field: value})`.

    The name is the run directory's name, which is what `Page.fixture` names and what `cdp.fixture`
    resolves to a `fixture.env` -- so a caller can hand over `runs/pages/fixture.env` and the recorder
    knows, without being told twice, which entries those conditions belong to.

    The parse is `cdp.fixture`'s, not `cdp.fixture`: this runs inside `make verify` against a tree that
    may have no run directory at all, and the five keys that half is strict about (the account, the
    password, the port) are how to reach a daemon that stopped existing when the setup script exited.
    Asking an env for a *condition* it never declared is a refusal; asking it for credentials it
    correctly withholds from a committed file would be nonsense.
    """
    directory, name = os.path.split(os.path.normpath(path))
    parent = os.path.basename(directory)
    if name != "fixture.env" or not parent:
        raise SystemExit(
            f"{path} is not a fixture env. Name the file a setup script wrote -- "
            f"<repo>/testing/capture/runs/<fixture>/fixture.env -- because the directory's name is "
            f"the only thing that says which pages these conditions belong to.")
    try:
        with open(path, encoding="utf-8") as fh:
            lines = fh.readlines()
    except OSError as exc:
        raise SystemExit(f"{path} cannot be read: {exc}. Run {parent}-setup.sh first, or name the "
                         f"fixture whose conditions you mean to record.")
    env = {}
    for line in lines:
        line = line.strip()
        if line and not line.startswith("#") and "=" in line:
            key, value = line.split("=", 1)
            env[key] = value.strip().strip('"')
    conditions = {}
    for key, field in CONDITION_KEYS:
        value = env.get(key)
        if not value:
            continue
        if field == "rows":
            conditions[field] = int(value)
        elif field == "census":
            conditions[field] = repo_relative(value)
        else:
            conditions[field] = value
    return parent, conditions


def read_receipt(path, fixture):
    """One capture run's own list of the frames it photographed, or None when there is none.

    The file is `cdp.write_receipt`'s: `{fixture, head, at, frames: {artifact: {sha256, at, head}}}`.
    It is what lets `record` ask a frame's date of the run that took the frame instead of of the
    commit it happens to stand in (#302), and it is read here rather than through `cdp` for the same
    reason `read_fixture_env` parses the env itself: this runs inside `make verify` against a tree
    that may hold no run directory at all, and a recorder that imported the browser machinery to
    read one JSON file could not be imported by `check`.

    A missing file is None, and that is a different answer from a hollow one. Missing is the
    ordinary state of a tree where nothing has been captured, and `record` reaches for the file only
    when it is about to write a date history contradicts; a file that names no frames, or names a
    fixture other than the directory it sits in, is a run that cannot say what it took, and that is
    refused where it is read rather than minted around. The second arm is also the dodge #302's M1
    measurement runs: copy a `fixture.env` somewhere and the receipt beside it is not there, and
    copying the receipt too gets it refused by the name inside it.
    """
    try:
        with open(path, encoding="utf-8") as fh:
            doc = json.load(fh)
    except FileNotFoundError:
        return None
    except (OSError, ValueError) as exc:
        raise SystemExit(
            f"{path} cannot be read: {exc}. That file is the capture receipt: re-run the driver that "
            f"writes it, or record only the frames whose bytes history already agrees with (#302).")
    named = doc.get("fixture")
    if named and named != fixture:
        raise SystemExit(
            f"{path} says it was written by fixture {named!r} and it sits in a directory named "
            f"{fixture!r}. A receipt moved next to another fixture's env vouches for frames its own "
            f"run never photographed (#302). Re-run {fixture}'s driver, which writes both files in "
            f"one directory.")
    frames = doc.get("frames")
    if not isinstance(frames, dict):
        raise SystemExit(
            f"{path} carries no `frames` object, so it says nothing about what that run photographed "
            f"-- and the recorder will not guess on its behalf (#302). Re-run the driver that wrote "
            f"it.")
    for artifact, claim in frames.items():
        if not isinstance(claim, dict) or not claim.get("sha256"):
            raise SystemExit(
                f"{path} names {artifact} without the bytes it wrote for it, so the claim cannot be "
                f"attached to a file -- which is the whole point of a receipt (#302). Re-run the "
                f"driver that wrote it.")
    return {"fixture": fixture, "head": doc.get("head") or "", "at": doc.get("at") or "",
            "frames": {str(k): dict(v) for k, v in frames.items()}}


def repo_relative(path):
    """A path the manifest is allowed to carry: the repository's, not the capturing machine's.

    The env holds `$CAPTURE_DIR/bin`, absolute, because it is fed to the daemon's PATH and a PATH
    entry has to be one. The manifest is a committed file, and a committed file that says
    `/home/rodney/prj/…/testing/capture/bin` is a claim about one checkout on one machine -- it makes
    the manifest differ per clone for a condition that is the same everywhere, and it publishes a
    local path. Measured on the first end-to-end run: `census` came out
    `/tmp/opencode/wt-211/testing/capture/bin`, which is both problems in one string.

    A path outside the repository is passed through rather than rewritten with `..` segments: a
    daemon whose PATH really does start somewhere else has that somewhere else as a condition, and
    inventing a repo-relative spelling for it is the mistake this function exists to stop.
    """
    if not path.startswith(os.sep):
        return path
    try:
        inside = os.path.commonpath([os.path.realpath(ROOT), os.path.realpath(path)]) \
            == os.path.realpath(ROOT)
    except ValueError:
        # Paths on different drives, or otherwise not comparable to ROOT.
        return path
    if not inside:
        return path
    return os.path.relpath(os.path.realpath(path), os.path.realpath(ROOT))


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


def record(directory, head=None, run=git, now=None, envs=(), shot=()):
    """Write the manifest for the PNGs sitting in `directory`, then return it.

    Called by `pages.py` on the path that has already published its staged files. It is not
    called on the refusal path, and it must not be: a manifest written over a run that refused
    would certify photographs this run did not take.

    `envs` names the `fixture.env` files whose conditions describe these frames, one per fixture
    (#211). Each file says which fixture it belongs to by the directory it sits in, and the driver
    says which pages that fixture photographs, so neither half has to be restated -- which is the
    only reason a nine-entry manifest can carry per-fixture conditions without a per-row guess.

    `shot` names the artifacts (PNG stems, no extension) THIS run photographed. It is what makes the
    conditions describe the truth rather than the bytes: a frame the driver just re-took was taken
    under this run's conditions even when those conditions left the pixels alone -- move the fixture's
    declared clock and `settings.png` comes back byte-identical, and the honest statement about it is
    today's clock, not the one in the file. Digest comparison alone cannot see that, and getting it
    the other way round makes a fixture change refuse its own capture.

    So the precedence is: photographed by this run -> stamped from the env this run was given;
    untouched -> keeps the block the manifest already had, because the manifest is the only remaining
    evidence about how those bytes were taken and today's fixture says nothing about yesterday's
    frame. An untouched frame whose env now contradicts the block it carries is said out loud rather
    than quietly reconciled either way -- usually the fixture moved and the frame is right, sometimes
    (measured, 4 Nov: `scripts` on the two POST fixtures) the old declaration was the bug.

    `shot` is also what makes an unclaimed frame REFUSABLE instead of guessable (#294). A frame whose
    bytes differ from the entry the manifest carries, and that this run did not name, is refused:
    there is no prior rev worth carrying and no honest rev to mint, so every other outcome is the
    invented date #222 was filed to stop -- and `check` cannot catch it afterwards, because the value
    it compares is the one this function just wrote.

    `shot` is also what makes a frame's date ASKABLE rather than merely written (#302, retiring the
    residue `D-0055` disclosed in its own words: "a *named* frame is still taken on the driver's
    word … the ordering obligation now lives in the notes instead of in a check"). Naming a frame
    used to be the whole of it, and `7e65d3b` is what that costs: nine names typed eighteen seconds
    after a page commit, no PNG in the commit, seven rows dated at a commit their own pixels predate.
    A second reading of history cannot repair that either -- measured twice, on #301's delivered set
    and on the arm this issue first sketched -- because a re-take whose pixels do not change never
    produces a commit touching the PNG, so `newest_commit(png)` can never catch up with a page commit
    that moved no pixels, and the rows it would refuse are the ones whose stylesheet edit happened not
    to move any. So the answer comes from the run instead: each driver's publish writes
    `runs/<fixture>/capture.json`, naming the frames it photographed with the bytes it wrote and the
    commit it stood in, and a frame that is both claimed and contradicted by its own history is
    refused unless that receipt vouches for its exact bytes from a commit at or after the page's
    newest one. The claim then travels in the entry as `photographed`, and `check` re-asks the
    ordering from it instead of asking the blob history a question it cannot answer. A frame nothing
    contradicts still needs no receipt: `newest_commit` answering empty is a capture not committed
    yet, which is the ordering this folder tells people to use.

    With no `envs` at all this writes what it used to, under `capture-manifest/1`. Recording
    conditions is a choice the caller makes by handing over the files that state them. What no `envs`
    does NOT buy is silence about the frames: both #294 refusals below fire from `shot` and the
    manifest being read, and the bare command's honesty is a property of the set it is pointed at,
    not of the flags it was given.
    """
    shot = set(shot)
    fixtures = {}
    # Each fixture's own account of what its run photographed (#302), read from beside the env that
    # names the fixture. Held even when there is no receipt -- `{}` -- because "this fixture left no
    # receipt" is the answer a refusal has to be able to print, and it has to name the directory it
    # looked in.
    claims, claim_dirs = {}, {}
    for path in envs:
        name, conditions = read_fixture_env(path)
        if name in fixtures:
            raise SystemExit(
                f"{path} states conditions for fixture {name!r}, which an earlier --fixture-env "
                f"already stated. One env per fixture: two files claiming one fixture is either a "
                f"copy left in the wrong run directory or a fixture that was renamed halfway.")
        fixtures[name] = conditions
        directory_of_env = os.path.dirname(os.path.normpath(path))
        claim_dirs[name] = directory_of_env
        receipt = read_receipt(os.path.join(directory_of_env, RECEIPT), name)
        claims[name] = (receipt or {}).get("frames", {})

    present = {name[:-len(".png")] for name in pngs_in(directory)}
    unknown = sorted(shot - present)
    if unknown:
        raise SystemExit(
            f"--shot names {', '.join(unknown)}, and {directory} holds no such PNG. A driver that "
            f"reports photographing a frame it did not write would have its conditions recorded "
            f"against a file that is not there.")

    entries = []
    declared = pages_by_artifact()
    specs = {spec.artifact: spec for spec in driver_pages()}
    binding = captures_by_artifact()
    prior_doc = read_manifest(directory) or {}
    prior = {e.get("png"): e for e in prior_doc.get("pages", []) if e.get("png")}
    used = set()
    notes = []
    replaced = []
    unclaimed = []
    uncertified = []
    for name in pngs_in(directory):
        artifact = name[:-len(".png")]
        page = declared.get(artifact)
        sources = sources_for(artifact, page)
        missing = [s for s in sources if not os.path.exists(os.path.join(ROOT, s))]
        if missing:
            raise SystemExit(f"{name}: its manifest cannot be written, because {missing[0]} "
                             f"does not exist. A PNG named after a page with no template is not "
                             f"evidence about anything.")
        digest = sha256(os.path.join(directory, name))
        before = prior.get(name)
        current = newest_commit(sources, run=run)
        # "Unchanged" is a byte comparison with the entry the manifest already carries, not a guess
        # from timestamps: the only thing that makes re-attribution safe is knowing these exact pixels
        # were certified before.
        unchanged = bool(before) and before.get("png_sha256") == digest
        # The photograph's own history, asked once per frame and read by three arms below.
        #
        # An EMPTY answer is not "these bytes are old". It means no commit this checkout can see
        # touches the PNG, which is the ordinary state of frames a run has photographed and not
        # committed yet -- the ordering every note in this folder tells people to use (`pages.py`
        # records before anything of its run is committed, `fixture.sh` says commit last). So the
        # date a run is about to write is CONTRADICTED only when there is a pixel history and it
        # sits strictly behind the page's. Keying on the absence instead is #144's trap, and the
        # rule that keeps that open is the one naming `lock.png`'s two shapes of "this run's own
        # frames" below.
        pixels = newest_commit([os.path.relpath(os.path.join(directory, name), ROOT)], run=run)
        contradicted = strictly_behind(pixels, current, run=run)
        # #294, the substitution half. A frame whose bytes are NOT the ones this manifest certified,
        # and that this run did not name in `shot`, has no honest `sources_rev` to write at all: the
        # pixels came from somewhere no declaration covers. The mint below would write the commit it
        # is standing in, and `recorded == current` is exactly the arm `check` short-circuits on, so
        # the mint does not merely mis-date the frame -- it switches the refusal off. Measured 11 Nov
        # at d7a5149 through real git over a throwaway repository: nine frames recorded at `A`, the
        # shared masthead moved at `B` with `profiles.png` re-taken in that commit, `audit.png` then
        # replaced with another page's frame, one `record --shot profiles`. `check` refused the swap on
        # identity before the record and said nothing about `audit.png` after it, its rev now the
        # current commit; with the env supplied the frame also gained this run's `capture` block,
        # conditions this run never applied to those pixels (#211 reopened for one frame). Nothing
        # downstream can undo it either: the swap's own commit is the newest commit touching that PNG
        # now, so `check`'s ancestry arm reads a substituted frame as a re-take. `--shot` is the only
        # input that separates the two, which is why the refusal is here and cannot live in `check`.
        # A refusal and not a carry, because there is no prior rev worth keeping -- the entry's old
        # rev describes the file that used to be there, and pointing new bytes at it would be the same
        # invented date with a longer pedigree.
        if before and not unchanged and artifact not in shot:
            replaced.append(name)
        # #294, the other half: a frame the manifest has NEVER seen, that this run does not claim
        # either. Nothing is here to carry, so the only value available is again the commit the run
        # stands in -- and minting it also disarms the one gate that does see a frame like this:
        # `check`'s `is committed with no entry in` arm stops firing the moment an entry exists, and
        # the arm that replaces it opens `if recorded == current: continue`.
        #
        # It has to be a COMPARISON, not the absence, because "no entry" is also the ordinary shape of
        # an honest run: every frame of an uncommitted capture is new to the manifest, and that is the
        # documented ordering (`pages.py` records before anything of its run is committed; `fixture.sh`
        # says "Commit last"). Refusing the absence is refusing `make capture`, which is #144's trap --
        # see the rule below, which is written to fail if this clause ever stops being conditioned.
        #
        # The live specimen of this mint, measured 11 Nov here, has an entry already and so is the
        # clause above's class rather than this one's. Its mint is `git show 7e65d3b`'s tree, not the
        # published file: there all nine entries dated `2088043cb` and none carried -- `carried_from`
        # arrived a commit later, in `7ff2a67`. In `git show 7ff2a67`'s tree, the blob `main` publishes
        # today, `lock.png` and `testlock.png` were the only two of the nine with no `carried_from`. At
        # both trees the mint reads the same: `7e65d3b` re-dated every row from `3bcbfdd` -- the newest
        # commit over six of the nine PNGs' pixels, an ANCESTOR of `2088043` -- to `2088043cb`, eighteen
        # seconds after `2088043` landed (16:35:37 to 16:35:55), so seven of the nine frames predated
        # their page and `check` on THAT tree answered `9 photographs current`, rc 0: the rev compared
        # is the one the record wrote. `lock.py` and `testlock.py` write no manifest, so this clause is
        # the same mint one step earlier: a frame nobody had recorded, from pixels older than its page.
        #
        # #301 re-took all nine on 11 Nov and recorded them named (D-0066), so nothing carries now. What
        # it did NOT change is the value this clause feeds: the entry below takes `sources_rev` from
        # `current` = `newest_commit(sources)`, the PAGE's rev -- so a `--shot` frame is still dated on
        # the driver's word. #302 owns that half, and the clause below is where it starts: the same
        # comparison, computed once per frame as `contradicted` above, now also stands behind the two
        # claims this function is about to write (#302).
        if before is None and artifact not in shot and contradicted:
            unclaimed.append(f"{name} (its pixels were last committed at {pixels[:9]}..., "
                             f"its page at {current[:9]}...)")
        # `sources_rev` names the commit these pixels were rendered from, so it belongs to the frame
        # and not to whoever ran `record` last (#222) -- the same rule `shot` already applies to the
        # conditions block, extended to the field the gate reads. Without it a run that photographs
        # one frame writes the CURRENT commit over every frame in the directory, and `check`'s drift
        # arm opens `if recorded == current: continue`, so the re-dating is the one value that
        # short-circuits the refusal: measured on this file's own code, five frames, one page moved
        # and re-taken, one bare `record`, and `check` answered 0 problems over four frames whose
        # bytes predate the page. Carrying the prior rev instead turns that into 4 refusals, and it
        # is the only spelling under which the field still means what its name says after a partial
        # re-capture -- the property D-0023, D-0028 and D-0030 each lost a sentence to.
        #
        # `shot` is the way out for the honest case, and it is the flag the recipe already passes: a
        # frame re-taken under moved conditions can come back byte-identical (`settings.png` does,
        # D-0028), and only the driver's word says so. A carried frame whose page really has moved is
        # refused by the drift arm with the carried rev in the sentence, which is the evidence, not a
        # cover-up. Nothing here can make a stale frame look current; the worst this does is refuse a
        # frame that was re-taken and not declared, and the message names the flag.
        if unchanged and artifact not in shot and before.get("sources_rev"):
            entry = {"sources_rev": before["sources_rev"]}
            if prior_doc.get("head"):
                # Which manifest these pixels were certified at, so a reader can ask that commit what
                # the frame was rendered from. Omitted rather than null when the file has no `head`:
                # a provenance field that is always the same placeholder proves nothing, and there is
                # such a placeholder in this file's own self-test (`_fake_git` used to answer
                # `rev-parse HEAD` with the shallow verdict).
                entry["carried_from"] = prior_doc["head"]
            if strictly_behind(pixels, before["sources_rev"], run=run):
                # #302's third ask, and a sentence rather than a refusal: the rev being carried was
                # not written by this run, and refusing now would make the honest repair (#301's
                # re-take) impossible for a frame this run has no business re-taking. What it does
                # say out loud is that `carried_from` cannot do the job its name claims for these
                # rows -- asking the commit they point at what the frame was rendered from answers
                # with a manifest whose own `head` is the mint.
                from_manifest = (str(prior_doc.get("head") or "")[:9] or "this manifest")
                notes.append(f"{name} carries {str(before['sources_rev'])[:9]}..., which is newer "
                             f"than its own pixels ({pixels[:9]}...): that rev was minted by whoever "
                             f"last recorded this frame, not measured against it, and the manifest "
                             f"it was carried from ({from_manifest}) names that same commit as its "
                             f"own head -- so `carried_from` leads to the invention, not to the "
                             f"photograph (#302). Re-take it: a receipt is the only thing that can "
                             f"date it honestly from here.")
        else:
            entry = {"sources_rev": current}
            # `current` is the newest commit over the PAGE. Writing it for a frame whose own pixels
            # were last committed behind that commit is a claim that somebody looked at this page
            # after the page last moved -- and #302 measured, twice and on the delivered set, that
            # no later reading of history can confirm or refute the claim, because a re-take whose
            # pixels do not change never produces a commit touching the PNG. `7e65d3b` is the shape:
            # nine `--shot` names typed eighteen seconds after `2088043cb` landed, no PNG in the
            # commit, seven rows moved onto a date their pixels predate. So the claim is asked for
            # where it is made, from the one source that is not the typing: the run's own receipt
            # (`cdp.write_receipt`), read from beside the `--fixture-env` that names its fixture.
            if artifact in shot:
                wanted = binding.get(artifact)
                holders = sorted(f for f, frames in claims.items() if artifact in frames)
                complaint, held = None, None
                if not holders:
                    complaint = (f"{name} (pixels {pixels[:9]}..., page {current[:9]}..., "
                                 f"no receipt naming it"
                                 + (f" -- looked in {claim_dirs[wanted]}" if wanted in claim_dirs
                                    else f" -- {wanted!r}'s env was not handed over, so its "
                                         f"runs/{wanted}/capture.json was not read") + ")")
                elif wanted and holders != [wanted]:
                    complaint = (f"{name} (pixels {pixels[:9]}..., page {current[:9]}..., the only "
                                 f"receipt naming it is {holders[0]!r}'s, and the driver photographs "
                                 f"this artifact from {wanted!r})")
                else:
                    held = claims[holders[0]][artifact]
                    if held.get("sha256") != digest:
                        complaint = (f"{name} (the bytes on disk are {digest[:9]}... and "
                                     f"{holders[0]}'s receipt says this run photographed "
                                     f"{str(held.get('sha256'))[:9]}... -- the file has changed since "
                                     f"the photograph)")
                    elif not is_ancestor(current, held.get("head") or "", run=run):
                        complaint = (f"{name} (its own capture stood at "
                                     f"{str(held.get('head') or 'no commit')[:9]}..., which is behind "
                                     f"this page's newest commit {current[:9]}... -- the frame was "
                                     f"photographed before the page moved, and the run typed --shot "
                                     f"after)")
                if complaint:
                    # The receipt is demanded only by a contradiction, never by the absence of a
                    # receipt: keyed on the missing file the gate refuses the ordering this folder
                    # prescribes -- capture, record, commit -- which is #302's own M1 measurement and
                    # the over-refusal #144 already named for the same field.
                    if contradicted:
                        uncertified.append(complaint)
                else:
                    # The date stays `current`, because that is what the page's newest commit IS;
                    # what the block adds is the evidence for it, and `check` re-asks the ordering
                    # from it on every later run. Written whenever a receipt covers the frame, not
                    # only when one was needed to clear it: the claim is what turns "re-take within
                    # one round trip", the note D-0055 could only promise in prose, into something a
                    # gate can say out loud the next time the page moves.
                    entry["photographed"] = {"fixture": holders[0],
                                             "at": held.get("at") or "",
                                             "head": held.get("head") or "",
                                             "sha256": held["sha256"]}
            elif contradicted and before is not None and unchanged:
                uncertified.append(
                    f"{name} (pixels {pixels[:9]}..., page {current[:9]}..., and the entry this "
                    f"manifest carries for it names no `sources_rev`, so there is nothing to carry "
                    f"and this run did not claim it either)")
        entry = {
            "page": page if page is not None
            else ("/" if artifact == "dashboard" else "/" + artifact),
            "png": name,
            "png_sha256": digest,
            "sources": sources,
            **entry,
        }
        carried = before.get("capture") if unchanged and before.get("capture") else None
        fixture = binding.get(artifact)
        if artifact in shot and fixtures:
            # Photographed by this run: its conditions are this run's, whatever the bytes did.
            if not fixture or fixture not in fixtures:
                raise SystemExit(
                    f"{name}: this run photographed it and no env handed over states the conditions "
                    f"{'its fixture ' + repr(fixture) + ' was taken' if fixture else 'it was taken'} "
                    f"under. Recording it from another fixture's env is the confident wrong answer; "
                    f"recording it from nothing is #207. Run {fixture or 'the matching'}-setup.sh and "
                    f"pass its runs/{fixture or '<fixture>'}/fixture.env.")
            carried = None  # this run's env outranks what the file said about these pixels
        if carried and carried.get("fixture") == fixture:
            disagree = sorted((k, carried[k], v) for k, v in (fixtures.get(fixture) or {}).items()
                              if k in carried and carried[k] != v)
            for key, was, now_value in disagree:
                # Said, not reconciled. The frame was not taken by this run, so the manifest's block
                # is the only evidence about it and today's fixture cannot overrule that evidence --
                # but a declaration that has moved is worth a line, because sometimes the thing that
                # moved is a correction rather than a change.
                notes.append(f"{name} keeps the {key} the manifest recorded for it ({was!r}) while "
                             f"{fixture}'s fixture.env now declares {now_value!r}: this run did not "
                             f"photograph that frame, so the file describes it and not today's "
                             f"fixture. Re-take it if the old declaration was the bug.")
            entry["capture"] = carried
            used.add(fixture)
        elif fixture and fixture in fixtures:
            block = {"fixture": fixture}
            block.update(fixtures[fixture])
            spec = specs.get(artifact)
            if spec is not None and getattr(spec, "state", ()):
                # The driver's own words, not a transcription of them: #211's acceptance is that the
                # state strings cannot be retyped into a manifest, and the same reason `record` reads
                # the page list out of the driver instead of copying it.
                block["state"] = [[row, cell] for row, cell in spec.state]
            # Judged on what the ENV contributed, not on how big the block came out: the driver's
            # own `state` cells go in here too, so a block can look populated while the fixture said
            # nothing about the clock, the census or the scripts.
            if not fixtures[fixture]:
                raise SystemExit(
                    f"{name}: {fixture}/fixture.env was handed over and declares no condition at "
                    f"all, so recording it would add a block that says nothing except which fixture "
                    f"was involved -- the exact state #211 was filed about. Declare the clock and "
                    f"the scripts condition in {fixture}-setup.sh, or record without --fixture-env "
                    f"and leave this manifest at {SCHEMA_PLAIN}.")
            entry["capture"] = block
            used.add(fixture)
        elif carried:
            entry["capture"] = carried
        elif unchanged:
            # A frame this run did not touch, from a fixture this run was not given an env for, with
            # no block in the manifest it inherited: it predates the field. States nothing, refuses
            # nothing -- and the document stays /1, so `check` cannot promise what two of its entries
            # do not say. This is the committed set's situation today, and the reason a re-take of one
            # page must not demand conditions for frames nobody re-took.
            pass
        elif fixtures:
            raise SystemExit(
                f"{name}: this frame is new or re-taken, and nothing states the conditions it was "
                f"taken under. {artifact!r} belongs to fixture {fixture!r}, which is not among the "
                f"envs supplied ({sorted(fixtures)}). Run {fixture}-setup.sh and pass its "
                f"runs/{fixture}/fixture.env, or the frame will be committed as evidence about "
                f"nothing (#207).")
        entries.append(entry)

    # One message, three halves. The set is the finding: a run that left three frames unclaimed has
    # three frames to re-take, and a refusal that reports one of them sends the operator through the
    # loop three times. Raised before the notes are printed and before anything is written -- the
    # manifest on disk stays the one from before this run, which is what keeps `check`'s findings
    # about these frames alive over a refused record. All three fire whatever the caller was given:
    # they are about a frame nobody can evidence, and no `--fixture-env` describes one.
    if replaced or unclaimed or uncertified:
        parts = []
        if replaced:
            parts.append(
                f"{len(replaced)} frame(s) in {directory} are not the files this manifest recorded, "
                f"and this run did not say it photographed them: {', '.join(replaced)}. Those bytes "
                f"came from somewhere no declaration covers, so no commit honestly names what they "
                f"were rendered from, and the only value this command could write is the commit it is "
                f"standing in -- the value `check`'s drift arm `if recorded == current: continue` "
                f"accepts without asking history anything. That is how a swapped frame stopped being "
                f"a finding after one bare `record` (#222's mechanism, #294's measurement): `check` "
                f"refuses these bytes on identity right now, and recording them spends that finding. "
                f"Re-take them, or name every frame this run really photographed with `--shot "
                f"<artifact>` and hand over the `--fixture-env` of the fixture that took it.")
        if unclaimed:
            parts.append(
                f"{len(unclaimed)} frame(s) in {directory} have no entry in {MANIFEST}, this run did "
                f"not name them, and each one's pixels are older than its page: {'; '.join(unclaimed)}"
                f". Nothing is here to carry, so the only value this command could write is again the "
                f"commit it is standing in -- #222's sentence with a different hand on the pen -- and "
                f"recording them also removes the one gate that does see a frame like this, `check`'s "
                f"`is committed with no entry in`, in favour of the drift arm that compares a "
                f"fabricated rev against nothing (#294). This is the standing shape of the frames "
                f"whose drivers write no manifest: `lock.py` and `testlock.py` photograph and leave "
                f"the recording to whoever notices. Re-take them and name them (`--shot lock "
                f"--fixture-env runs/lock/fixture.env`, and the same for `testlock`), or record them "
                f"in the commit that photographs them -- a frame with no history yet is honestly dated "
                f"at this commit, and a frame whose pixels are this old is not.")
        if uncertified:
            parts.append(
                f"{len(uncertified)} frame(s) in {directory} are claimed by this run and about to be "
                f"dated at their page's newest commit while their own pixels were last committed "
                f"behind it: {'; '.join(uncertified)}. That date is the driver's word, and #302 "
                f"measured on the delivered set that nothing can check it afterwards: a re-take whose "
                f"pixels do not change never produces a commit touching the PNG, so "
                f"`newest_commit(png)` can never catch a page commit that moved no pixels, and "
                f"`check`'s `if recorded == current: continue` accepts the value this function just "
                f"wrote. `7e65d3b` is the shape this refuses -- nine `--shot` names typed eighteen "
                f"seconds after `2088043cb` landed, no PNG in that commit, seven rows moved onto a "
                f"date their pixels predate. What stands behind the claim now is the receipt the "
                f"driver's own publish writes at `runs/<fixture>/capture.json`, beside the "
                f"`--fixture-env` you handed over. So: run "
                f"the driver that photographs these frames and record again in that same sitting, or "
                f"drop `--shot` and let each frame keep the rev its own bytes were certified at -- "
                f"which is nothing to carry for a frame the manifest has never seen, and that one has "
                f"to be photographed rather than named.")
        parts.append("Nothing was written.")
        raise SystemExit("\n".join(parts))

    for note in notes:
        print(f"note: {note}", file=sys.stderr)

    unused = sorted(set(fixtures) - used)
    if unused:
        raise SystemExit(
            f"--fixture-env named {', '.join(unused)} and none of the PNGs in {directory} belongs to "
            f"it. A conditions file that describes nothing recorded here is either a typo, the wrong "
            f"run directory, or a fixture whose pages are photographed into another directory -- and "
            f"recording it anyway is how a manifest grows a field nobody can trace to a frame.")
    # The schema describes what the file contains, not which version of this program wrote it, and
    # the difference is whether the promise is universal: /2 says EVERY frame here states what it was
    # taken under, so `check` may hold every entry to it.
    #
    # `all`, not `any`, because of the way the committed set is actually assembled. The nine artifacts
    # in docs/screenshots/360 are photographed by three drivers -- `pages.py` per fixture, `lock.py`,
    # `testlock.py` -- and only the first of them records. So the honest re-take of one page hands the
    # recorder one fixture's env over a directory holding two frames from fixtures that were not
    # involved, and a schema chosen on `any` would call that directory /2 and then have `check` refuse
    # the two frames the run could not have known anything about. That is the trap #144 was: the
    # workflow that does the right thing, made red. A partial manifest is therefore /1, it carries the
    # blocks it really has, and `check` reports the ratio -- "7 of them state ...", which is a sentence
    # an operator reads -- instead of a refusal that sends them to run a fixture they did not touch.
    complete = bool(entries) and all("capture" in e for e in entries)
    doc = {"schema": SCHEMA if complete else SCHEMA_PLAIN, "viewport": ARTIFACT_WIDTH,
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

    if doc.get("schema") not in SCHEMAS_READABLE:
        problems.append(f"{MANIFEST} declares schema {doc.get('schema')!r}, want one of "
                        f"{' or '.join(repr(s) for s in SCHEMAS_READABLE)}. {SCHEMA_PLAIN} is the "
                        f"form recorded before #211 and stays readable; a name outside both is a "
                        f"filed-away format, not an old manifest.")
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
    specs = {spec.artifact: spec for spec in (pages if pages is not None else driver_pages())}
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
        # What the frame was taken under (#211). Checked before the source claims and before any of
        # them `continue`s, because an entry that says nothing about its conditions is a problem even
        # when its template turns out to be current -- that is the whole finding.
        cap = e.get("capture")
        if doc.get("schema") == SCHEMA and not cap:
            problems.append(f"{name} is recorded under {SCHEMA} with no `capture` block, so the "
                            f"manifest answers \"which page is this\" and not \"under what "
                            f"conditions\": it can be read as current by a reader with no way to know "
                            f"the frame was taken with page scripts running, on a host clock, from a "
                            f"census of the capturing machine. That is #207, and re-record it from "
                            f"the fixture's own env rather than hand-writing a block.")
        elif cap:
            fixture = cap.get("fixture")
            if not fixture:
                problems.append(f"{name}'s capture block names no fixture, so its conditions belong "
                                f"to no setup script and cannot be re-declared by anyone")
            scripts = cap.get("scripts")
            if scripts is not None and scripts not in script_values():
                problems.append(f"{name} records scripts={scripts!r}, which is not one of "
                                f"{' or '.join(repr(v) for v in script_values())}. A condition with a "
                                f"typo in it is a lie with a plausible shape, and the two POST frames "
                                f"are the reason this one is worth refusing: they were taken with "
                                f"scripts on, under a driver that now turns them off (#213)")
            elif scripts == "on":
                # Legal, and not committable. `cdp.scripts_disabled` honours the declaration, so
                # taking a frame with the page's scripts running is one environment variable away --
                # which is how you measure what the switch is worth. What it buys is a measurement,
                # not evidence: this folder's criterion is "usable at 360 px with JavaScript off", and
                # a frame whose own entry says the scripts were running has never tested that
                # sentence, so it must not land in docs/screenshots/360 next to the frames that did.
                problems.append(f"{name} records scripts='on': the page's own scripts were running "
                                f"when this frame was taken, so it has never tested this folder's "
                                f"criterion -- usable at 360 px with JavaScript off "
                                f"(testing/capture/README.md). Keep that run's PNG as a measurement; "
                                f"what is committed here is the frame taken with the declaration at "
                                f"its default")
            sessions = cap.get("sessions")
            if sessions is not None and not (isinstance(sessions, str) and sessions.isdigit()):
                problems.append(f"{name} records sessions={sessions!r}, which is not a count. The "
                                f"census is the number of lockable sessions the fixture asked the "
                                f"machine for and the driver counted on the page, so a value with a "
                                f"typo in it is a condition that reads as a number and matches "
                                f"nothing (#213). Re-record the frame from its fixture's own env.")
            spec = specs.get(name[:-len(".png")])
            if spec is not None:
                want_fixture = getattr(spec, "fixture", None)
                if fixture and want_fixture and fixture != want_fixture:
                    problems.append(f"{name} records conditions from {fixture!r} while the driver "
                                    f"photographs {spec.url} from {want_fixture!r}. The conditions "
                                    f"belong to the fixture that produced the frame; a block copied "
                                    f"from another fixture's env is a confident wrong answer, and "
                                    f"that is the failure mode #211's shape note warned about")
                want_state = [[row, cell] for row, cell in getattr(spec, "state", ())]
                if want_state and cap.get("state") != want_state:
                    problems.append(f"{name} records the state {cap.get('state')!r} and the driver "
                                    f"now declares {want_state!r} for {spec.url}. The state strings "
                                    f"are the driver's, so this manifest describes a reading of the "
                                    f"page that the fixture no longer builds -- re-take the frame "
                                    f"rather than editing this entry to match")

        # #302: a frame's own account of the run that photographed it, verified before it is used.
        #
        # `photographed` is the one field in an entry that says something about a RUN rather than
        # about bytes or a page, and it is what `check` now dates a claimed frame from -- so none of
        # it is trusted because `record` wrote it. Four shapes are refused: a block that is not a
        # block, one that names no commit (nothing to date the frame from), bytes that are not the
        # file on disk (the claim belongs to a photograph somebody has since replaced), and a fixture
        # that does not photograph this artifact -- the moved-receipt dodge, which is the
        # conditions-half failure #211 already names, one field over. Each leaves the frame to the
        # history arms below: an unusable claim is not a pass, it is the absence of a claim.
        claim = e.get("photographed")
        claim_rev = ""
        if claim is not None:
            spec = specs.get(name[:-len(".png")])
            want_fixture = getattr(spec, "fixture", None) if spec is not None else None
            if not isinstance(claim, dict):
                problems.append(f"{name} carries `photographed` as {claim!r}, and only the block a "
                                f"capture run's receipt turns into is a date this file may use "
                                f"(#302). Re-run the driver and re-record it.")
            else:
                claim_fixture = claim.get("fixture")
                claim_sha = claim.get("sha256")
                claim_head = claim.get("head") or ""
                if claim_fixture and want_fixture and claim_fixture != want_fixture:
                    problems.append(f"{name} claims it was photographed by {claim_fixture!r}, and "
                                    f"the driver photographs {page} from {want_fixture!r}. A receipt "
                                    f"that belongs to another fixture's run says nothing about these "
                                    f"bytes (#302).")
                elif not claim_sha:
                    problems.append(f"{name}'s `photographed` block names no `sha256`, so its claim "
                                    f"is about a filename rather than about these pixels (#302).")
                elif claim_sha != got:
                    problems.append(f"{name} claims it was photographed as {str(claim_sha)[:12]}... "
                                    f"and the file on disk is {got[:12]}...: the bytes the receipt "
                                    f"vouches for are not the bytes committed, which is #294's "
                                    f"substitution wearing #302's provenance block")
                elif len(claim_head) < 7:
                    problems.append(f"{name}'s `photographed` block names commit {claim_head!r}, "
                                    f"which is not enough of a commit to date a frame against "
                                    f"(#302). A capture writes the receipt with `git rev-parse "
                                    f"HEAD` in the checkout it ran in; re-run the driver here.")
                else:
                    claim_rev = claim_head

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
        # The frame's OWN newest commit, asked once and read by two arms: the claim arm right below,
        # where it is the witness a capture cannot supply out of its own transcript (#302's atomic
        # case), and the drift arm, where it is D-0056's comparison.
        shot = newest_commit([os.path.relpath(path, ROOT)], run=run)
        if claim_rev:
            # #302, the other half: a frame whose entry carries the run's own word is dated by that
            # word, not by its blob history -- and it is dated AHEAD of the `recorded == current`
            # short-circuit below, which is the arm that made the mint invisible: it compares the
            # manifest against the value `record` just wrote, so a frame dated at its own page's
            # commit never reaches the history that could contradict it. A claim consulted after
            # that arm would inherit the blindness, and a row whose `sources_rev` says `current`
            # while its own receipt says the capture stood behind -- the `7e65d3b` state with a
            # receipt bolted on, and the shape a hand-edited entry takes -- would be skipped by it.
            # Ordering the claim first costs nothing where it agrees: a covered claim says exactly
            # what the short-circuit says, so no row that passed before is refused now.
            #
            # The history cannot answer the question for a frame like this, and the arm this issue
            # first sketched was measured against the delivered set before it was dropped: the
            # pixels' newest commit stays at whatever commit last MOVED the bytes, because a re-take
            # whose pixels do not change never produces a commit touching the PNG -- so every page
            # commit after the last byte-change reads as drift, forever. That check refused 7 of the
            # 9 committed rows and named the two whose bytes had genuinely moved in `7e65d3b`, which
            # is #338's unsatisfiable advice seen from the other side.
            #
            # The claim answers the same question in one direction, and answers it honestly: the
            # capture stood at C, so these bytes cannot have seen a page commit after C. A frame is
            # therefore current against a page at or before C and stale against any commit after it.
            # That is the re-take deadline `record`'s note used to promise in prose and could not
            # enforce -- D-0055's "the ordering obligation now lives in the notes instead of in a
            # check", retired -- and it is enforced only on rows that carry the run's word, which is
            # why #338's nine unclaimed rows keep the answer they have.
            try:
                covered = is_ancestor(current, claim_rev, run=run)
            except SystemExit:
                # `git()` refuses a rev it cannot resolve, and a claim whose commit a rebase retired
                # is not a finding about the photograph -- it is this checkout failing to reach the
                # clone the capture ran in. Say which, and do not let one row abort the gate.
                covered = None
            if covered:
                continue
            # And the frame's own bytes get the same vote the drift arm gives them, because the claim
            # is one commit CONSERVATIVE by construction and the recipe in README.md is what makes it
            # so: the driver photographs and records BEFORE the commit that carries its output, so the
            # commit a receipt can name is the PARENT of the commit that lands the photograph -- for
            # every honest capture, every time. Measured 13 Nov with real git over a throwaway
            # repository (/tmp/opencode/302b/probe.py): the one commit carrying `profiles.tmpl`, the
            # re-taken `profiles.png` and the re-recorded manifest is refused here on this branch by
            # the sentence below, and accepted on `origin/main` by D-0056's comparison -- the
            # regression D-0056 exists to prevent ("...or in that same commit; what it may not be is
            # older than the page"), which is also #302's own list of orderings that must stay legal:
            # "A frame re-taken and committed in the same commit the page moved in."
            #
            # What it does NOT buy is the frame that did not travel. Bytes that did not change
            # produce no commit touching the PNG, so `shot` stays behind, the claim is the only
            # witness left, and a claim one commit conservative cannot tell a byte-identical re-take
            # in its page's own commit from a frame nobody photographed at all. That residue is
            # #338's, it is red on `origin/main` as well (same probe: 9 problems on both trees, the
            # #144 arm there and this arm here), and what this branch changes about it is the
            # SENTENCE -- the repair named below is reachable, re-run the driver and re-record
            # (measured green, same probe), where main's advice to re-take "in that same commit"
            # could not be satisfied by anyone.
            if shot and is_ancestor(current, shot, run=run):
                continue
            if covered is None:
                problems.append(f"{name}'s capture claim names commit {claim_rev[:12]}, which this "
                                f"checkout cannot order against {page}'s newest commit "
                                f"{current[:12]}: the claim was written where this history does not "
                                f"reach -- a rebase, or a capture run in another clone -- so it "
                                f"neither dates nor clears the frame (#302). Fetch, or re-run "
                                f"testing/capture in this checkout and re-record.")
                continue
            witness = ("no commit in this checkout touches the photograph, so " if not shot else
                       f"the newest commit touching the photograph is {shot[:12]}..., "
                       + ("behind it too, " if strictly_behind(shot, current, run=run)
                          else "and this checkout cannot place it at or after the page, ")
                       + "so ")
            problems.append(f"{name} was last photographed at commit {claim_rev[:12]} and {page}'s "
                            f"sources have moved to {current[:12]} since: the run that took these "
                            f"bytes stood behind the page, and {witness}neither the run's word nor "
                            f"its pixels say anybody has looked at this page since the frame was "
                            f"taken (#302). Re-run testing/capture and re-record -- the receipt the "
                            f"new run leaves beside its fixture.env is what re-dates this row.")
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
        # `shot` was asked once above, before the claim arm, and read here for the comparison D-0056
        # introduced.
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


def build_parser():
    """The CLI, as a function, so a self-test can hand it tokens without running the verb."""
    ap = argparse.ArgumentParser(prog="manifest.py", description=__doc__.splitlines()[0])
    ap.add_argument("verb", nargs="?", default="check", choices=("check", "record"))
    ap.add_argument("--dir", default=os.path.join(ROOT, "docs", "screenshots", "360"))
    ap.add_argument("--fixture-env", action="append", default=[], metavar="PATH", dest="envs",
                    help="a fixture's runs/<fixture>/fixture.env, repeatable: one per fixture "
                         "photographed into --dir. Each file names its own fixture by its directory, "
                         "so conditions are never applied to a page they were not declared for "
                         "(#211). The run's receipt (capture.json) is read from the same directory, "
                         "because the same run writes both (#302). Omit it and the manifest is "
                         "recorded at " + SCHEMA_PLAIN + ".")
    ap.add_argument("--shot", action="append", default=[], metavar="ARTIFACT", dest="shot",
                    help="an artifact THIS run photographed, named without .png, repeatable. Its "
                         "conditions are taken from --fixture-env rather than carried over from the "
                         "manifest, because a frame re-taken under moved conditions is a frame "
                         "taken under those conditions even when its bytes did not move. Drivers "
                         "pass it themselves; use it when recording a frame a driver did not record "
                         "(lock.py and testlock.py write no manifest). A name is not the whole "
                         "claim: a frame whose own pixels were last committed behind its page is "
                         "refused unless that run's receipt vouches for its exact bytes from a "
                         "commit at or after the page's newest one (#302).")
    ap.add_argument("--self-test", action="store_true")
    return ap


def strictly_behind(older, newer, run=git):
    """Is `older` an ANCESTOR of `newer` and not the same commit -- can this checkout PROVE it?

    `is_ancestor` alone cannot answer that. It is true of two equal revs, and it is false of two
    revs this checkout cannot order -- a git that answers `?`, a manifest carrying a rev a rebase
    retired, a depth-1 fetch. Both call sites below ask for the forward direction and take "no
    verdict" as "no finding", for the two failures #302 measured on the way: keyed on the absence
    of a pixel history the check refuses the ordering every note in this folder tells people to use
    (M1), and keyed on an ordering it cannot establish it refuses a set whose history merely happens
    to be short (the same shape #144 draws for an empty scan).

    A rev git cannot resolve makes `git()` refuse the whole command, so the question is asked
    inside a guard: "this checkout cannot see the commit the manifest names" is a sentence about one
    frame, never a stack trace over nine.
    """
    if not older or not newer or older == newer or len(older) < 7 or len(newer) < 7:
        return False
    try:
        return is_ancestor(older, newer, run=run)
    except SystemExit:
        return False


def summary(doc):
    """The sentence `check` prints when it found nothing -- naming the frames nobody re-took.

    A bare count reads as "this capture photographed and re-dated all nine", which is the claim #222
    is about: after a one-fixture run against the committed directory, the untouched entries are
    carried and the sentence must not imply otherwise. It is a function rather than an f-string in
    `main` so the self-test can pin the clause -- the clause is the only thing a reader of a green
    `make verify` has to tell a whole set apart from a subset, and a check whose summary lies is
    worse than one that refuses.
    """
    entries = doc.get("pages", [])
    stated = sum(1 for e in entries if e.get("capture"))
    carried = [e for e in entries if e.get("carried_from")]
    line = (f"screenshot manifest: {len(entries)} photographs current against their sources "
            f"(each pinned to {doc.get('viewport')} px and to the commit its page was rendered "
            f"from), {stated} of them also stating the fixture and conditions they were taken under")
    if not stated:
        line += (f" -- {SCHEMA_PLAIN} recorded before conditions were kept, so nothing here says "
                 f"what any frame was taken under (#211)")
    if carried:
        from_rev = ", ".join(sorted({str(e["carried_from"])[:9] for e in carried}))
        line += (f"; {len(carried)} of them are carried -- their `sources_rev` is the one certified in "
                 f"{from_rev}, not this run's, because this run did not re-take them -- so this line "
                 f"says nothing about those frames beyond their own history")
    # #302's share of the same honesty. A date `record` minted is arithmetic on the page's history;
    # a date a receipt backs is a measurement a run made. The reader of a green `make verify` cannot
    # tell those apart from a count, so the count says which is which -- and while no page commit
    # contradicts an unbacked row, `check` has nothing to refuse it with, which is exactly the state
    # `7e65d3b` left the committed set in.
    unbacked = [e for e in entries if not e.get("photographed")]
    if unbacked and entries:
        line += (f"; {len(unbacked)} of them are dated at a commit no capture run vouched for, so "
                 f"their date stands on this file's arithmetic rather than on a receipt (#302)")
    return line


def main(argv=None):
    ap = build_parser()
    a = ap.parse_args(argv)
    if a.self_test:
        return self_test()
    if a.verb == "record":
        doc = record(a.dir, envs=a.envs, shot=a.shot)
        with_conditions = sum(1 for e in doc["pages"] if e.get("capture"))
        print(f"recorded {len(doc['pages'])} page(s) at {doc['viewport']} px into "
              f"{manifest_path(a.dir)}")
        if with_conditions == len(doc["pages"]) and doc["pages"]:
            print(f"  all {with_conditions} state the fixture and conditions they were taken under "
                  f"({SCHEMA})")
        elif with_conditions:
            print(f"  {with_conditions} of {len(doc['pages'])} state the fixture and conditions they "
                  f"were taken under, so this stays {SCHEMA_PLAIN}: the others belong to fixtures "
                  f"this run was not given an env for. {SCHEMA} is written only when every frame "
                  f"here says what it was taken under, because that is what `check` promises")
        else:
            print(f"  {SCHEMA_PLAIN}: no --fixture-env was given, so nothing here says under what "
                  f"conditions any frame was taken. That is what #211 is about.")
        return 0
    problems = check(a.dir)
    if problems:
        print(f"screenshot manifest: {len(problems)} problem(s):")
        for p in problems:
            print(f"  {p}")
        return 1
    doc = read_manifest(a.dir)
    print(summary(doc))
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
            # `rev-parse HEAD` is `record`'s question -- "which commit is this manifest being written
            # at" -- and it used to get the shallow verdict back, because this branch was written for
            # one caller and never looked at the argument. A box whose manifest has `head: "false"`
            # cannot carry a frame honestly: `carried_from` would name a string that is not a commit,
            # which is every provenance field in this file's own self-tests.
            if len(args) > 1 and args[1] == "HEAD":
                return rev
            return "true" if shallow else "false"
        return "HEAD"
    return run





class _Page:
    """A stand-in for `pages.Page` that carries the three fields the conditions rules read."""

    def __init__(self, url, artifact, fixture=None, state=()):
        self.url, self.artifact = url, artifact
        self.fixture = fixture
        self.state = tuple(state)


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
        """Run `fn(box)` with ROOT pointed at a fresh box, and hand back its complaint.

        A `SystemExit` that reaches here is a `record`/`check` call failing while the rule was setting
        up or asking its question, which is a finding about the tree -- but a traceback that escapes
        the loop aborts the run before the remaining rules speak, so the count it leaves behind is
        wrong and the message reads as "something is wrong somewhere" rather than "this rule went
        red". Rules that WANT a refusal catch it themselves with `refused`, so this only ever turns a
        crash into a named failure.
        """
        global ROOT
        box = _box()
        saved = ROOT
        ROOT = box
        try:
            return fn(box)
        except SystemExit as exc:
            return ("a record/check call this rule needs in order to ask anything refused instead: "
                    f"{str(exc)[:220]}")
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

    # ---- #211: the conditions a frame was taken under ------------------------------
    #
    # #207 taught the capture to refuse a page whose clock, census, zone, row count or scripts
    # condition is not the one its fixture declares. This file is the artifact of record, and it
    # carried none of it -- so the manifest could still be read as "this PNG is current" by someone
    # with no way to know the frame was taken with page scripts running, under a host clock, from a
    # census of the capturing machine. Every rule below exists because a field that can be wrong in
    # one specific way has to be refused in that specific way.

    def env_file(box, fixture, **overrides):
        """One fixture's declared conditions, at the path that names the fixture."""
        d = os.path.join(box, "runs", fixture)
        os.makedirs(d, exist_ok=True)
        path = os.path.join(d, "fixture.env")
        values = {"PLOCK_CAPTURE_USER": "admin", "PLOCK_CAPTURE_PASS": "hunter2",
                  "PLOCK_CAPTURE_TARGET": "casey", "PLOCK_CAPTURE_PORT": "8099",
                  "PLOCK_CAPTURE_CDP": "9222", "PLOCK_CAPTURE_STAMP": "2026-03-18T21:34:05",
                  "PLOCK_CAPTURE_ZONE": "-04:00", "PLOCK_CAPTURE_TZ": "America/New_York",
                  "PLOCK_CAPTURE_AUDIT_ROWS": "8",
                  "PLOCK_CAPTURE_PATH_PREPEND": "testing/capture/bin",
                  "PLOCK_CAPTURE_SCRIPTS": "off"}
        values.update(overrides)
        with open(path, "w", encoding="utf-8") as fh:
            for key, value in values.items():
                fh.write(f"{key}={value}\n")
        return path

    def receipt(box, fixture, frames, head=RECORDED, at="2026-01-01T00:00:00Z", write=True):
        """The file a capture run leaves beside its own `fixture.env`, planted by hand (#302).

        Planted and not produced: this box has no browser and no run directory, and the claim under
        test is what `record` and `check` make of a receipt, not how `cdp` came to write one. The
        shape is `cdp.write_receipt`'s and `a_receipt_is_the_shape_the_driver_writes` below is what
        keeps the two files from drifting apart -- the same reasoning that makes the state strings
        and the script values in this suite be read from the driver instead of typed.

        The digest is of the PNG as it stands when the receipt is planted, because that is what a
        publish does: the run hashes the bytes it wrote and nothing that happens afterwards. Re-plant
        after re-writing a frame, which is what an operator with a replaced file has to do too.

        `write=False` returns the document without saving it, for the rules that hand a recorder a
        file it cannot read.
        """
        doc = {"fixture": fixture, "head": head, "at": at, "frames": {}}
        for stem in frames:
            doc["frames"][stem] = {
                "sha256": sha256(os.path.join(box, "docs", "screenshots", "360", stem + ".png")),
                "at": at, "head": head}
        if write:
            d = os.path.join(box, "runs", fixture)
            os.makedirs(d, exist_ok=True)
            with open(os.path.join(d, RECEIPT), "w", encoding="utf-8") as fh:
                json.dump(doc, fh, indent=2, sort_keys=True)
                fh.write("\n")
        return doc

    def recorded(box, envs=(), pngs=("dashboard", "users", "profiles", "audit", "settings")):
        d = os.path.join(box, "docs", "screenshots", "360")
        for a in pngs:
            _png(box, a + ".png")
        record(d, run=_fake_git(), now="2026-01-01T00:00:00Z", envs=envs)
        return read_manifest(d)

    def refused(fn):
        """Run `fn`, wanting a SystemExit, and hand back the complaint -- message included."""
        try:
            fn()
        except SystemExit as exc:
            return str(exc) or None
        return "REFUSED-NOTHING"

    def conditions_land_on_the_pages_that_fixture_photographs(box):
        """One fixture's env must not stamp another fixture's page, and a fixture with no page in
        this directory must not be recorded at all."""
        good = recorded(box, envs=[env_file(box, "pages")])
        if good["schema"] != SCHEMA:
            return f"recording conditions still wrote {good['schema']!r}"
        for e in good["pages"]:
            cap = e.get("capture")
            if not cap:
                return f"{e['png']} got no capture block from the pages fixture's env"
            if cap.get("fixture") != "pages":
                return f"{e['png']} was stamped from {cap.get('fixture')!r}, not 'pages'"
            if cap.get("stamp") != "2026-03-18T21:34:05" or cap.get("scripts") != "off":
                return f"{e['png']}'s conditions are not what the env declared: {cap}"
            if "password" in json.dumps(cap) or "PLOCK_CAPTURE_PASS" in json.dumps(cap):
                return f"{e['png']}'s capture block carries a credential key"
        if any(k not in cap for k in ("zone", "tz", "rows", "census")):
            return f"{e['png']}'s block dropped a declared condition: {cap}"

        # The other half of the binding question, and the one a per-row flag would get wrong: an env
        # whose pages are not in this directory is a mistake, not a no-op.
        def mismatched():
            d = os.path.join(box, "docs", "screenshots", "360")
            record(d, run=_fake_git(), now="2026-01-01T00:00:00Z",
                   envs=[env_file(box, "pages-noprof")])
        message = refused(mismatched)
        if message == "REFUSED-NOTHING" or not message:
            return ("an env for a fixture with no PNG in the directory recorded happily, so "
                    "conditions can be filed against frames they never touched")
        if "pages-noprof" not in message:
            return f"the refusal does not name the offending fixture: {message[:120]!r}"

        # And the credentials the env also holds stay out of the committed file.
        if "hunter2" in json.dumps(good):
            return "the manifest carries the fixture's password"
        return None

    rule("conditions land on the pages their fixture photographs, and nowhere else",
         in_box(conditions_land_on_the_pages_that_fixture_photographs))

    def a_declared_state_comes_from_the_driver(box):
        """The `state` strings are the driver's, so retyping them into a manifest is impossible."""
        real = {spec.artifact: spec for spec in driver_pages()}
        doc = recorded(box, envs=[env_file(box, "pages")])
        for e in doc["pages"]:
            spec = real.get(e["png"][:-4])
            want = [[row, cell] for row, cell in getattr(spec, "state", ())]
            got = e["capture"].get("state")
            if bool(want) != bool(got):
                return (f"{e['png']}: the driver declares {len(want)} state cells and the manifest "
                        f"records {len(got or [])}")
            if want and got != want:
                return f"{e['png']}: recorded {got!r}, the driver declares {want!r}"
        if not any(e["capture"].get("state") for e in doc["pages"]):
            return ("no page's state was recorded at all, so this rule is reading nothing -- the "
                    "driver would have to have lost every declared state cell")
        return None

    rule("the state strings are the driver's, not a transcription",
         in_box(a_declared_state_comes_from_the_driver))

    def an_env_that_declares_nothing_is_refused(box):
        """#207 was a capture with undeclared conditions; an empty block is the same claim with a
        field name on it."""
        bare = {"PLOCK_CAPTURE_STAMP": "", "PLOCK_CAPTURE_ZONE": "", "PLOCK_CAPTURE_TZ": "",
                "PLOCK_CAPTURE_AUDIT_ROWS": "", "PLOCK_CAPTURE_PATH_PREPEND": "",
                "PLOCK_CAPTURE_SCRIPTS": ""}

        def go():
            d = os.path.join(box, "docs", "screenshots", "360")
            _png(box, "dashboard.png")
            record(d, run=_fake_git(), now="2026-01-01T00:00:00Z",
                   envs=[env_file(box, "pages", **bare)])
        message = refused(go)
        if message == "REFUSED-NOTHING" or not message:
            return ("an env that declares no condition recorded a block anyway, which is the "
                    "undeclared-conditions state wearing /2's clothes")
        if "#211" not in message and "condition" not in message:
            return f"the refusal does not say what is wrong: {message[:120]!r}"
        return None

    rule("an env that declares no condition is refused rather than recorded",
         in_box(an_env_that_declares_nothing_is_refused))

    def conditions_follow_the_frame_not_the_recorder(box):
        """An unchanged PNG keeps the conditions it was taken under; a re-taken one is stamped from
        the env that just described it."""
        first = recorded(box, envs=[env_file(box, "pages", PLOCK_CAPTURE_STAMP="2026-03-18T21:34:05")])
        d = os.path.join(box, "docs", "screenshots", "360")
        # The fixture's declaration moves. The committed frame did not move, so its provenance may
        # not: re-recording must not re-attribute a clock to pixels taken under another one. The note
        # that this provokes is asserted by the rule below; here it is swallowed so that `make verify`
        # prints the self-test's own line and nothing else.
        import contextlib
        import io
        with contextlib.redirect_stderr(io.StringIO()):
            record(d, run=_fake_git(), now="2026-02-01T00:00:00Z",
                   envs=[env_file(box, "pages", PLOCK_CAPTURE_STAMP="2027-01-01T00:00:00")])
        again = read_manifest(d)
        moved = [e for e in again["pages"] if e["capture"]["stamp"] != "2026-03-18T21:34:05"]
        if moved:
            return (f"{moved[0]['png']} was re-stamped to {moved[0]['capture']['stamp']!r} while its "
                    f"bytes were unchanged, so a frame taken under one clock now claims another")
        # Now the frame really is new, and the fresh declaration is the honest one -- and since #294
        # it has to be DECLARED: a changed frame this run does not name is refused rather than
        # re-dated, which is the same rule this file's recipes follow (`--shot` for every frame the
        # run took). Silence here would be the refusal firing on the honest workflow.
        _png(box, "users.png", body=b"a re-taken frame")
        with contextlib.redirect_stderr(io.StringIO()):
            record(d, run=_fake_git(), now="2026-03-01T00:00:00Z",
                   envs=[env_file(box, "pages", PLOCK_CAPTURE_STAMP="2027-01-01T00:00:00")],
                   shot=["users"])
        retaken = read_manifest(d)
        fresh = [e for e in retaken["pages"] if e["png"] == "users.png"][0]
        if fresh["capture"]["stamp"] != "2027-01-01T00:00:00":
            return ("a re-taken frame kept the old conditions, so the block describes a capture "
                    "that no longer exists on disk")
        kept = [e for e in retaken["pages"] if e["png"] == "audit.png"][0]
        if kept["capture"]["stamp"] != "2026-03-18T21:34:05":
            return "a frame nobody re-took was re-stamped by the run that re-took another page"
        return None

    rule("conditions belong to the frame, not to whoever ran record last",
         in_box(conditions_follow_the_frame_not_the_recorder))

    def a_second_new_frame_needs_its_own_conditions(box):
        """A frame that is new and has no env is the #207 shape, and recording the OTHER fixtures'
        envs must not let it through on a guess."""
        def go():
            d = os.path.join(box, "docs", "screenshots", "360")
            for a in ("dashboard", "users-no-profiles"):
                _png(box, a + ".png")
            record(d, run=_fake_git(), now="2026-01-01T00:00:00Z",
                   envs=[env_file(box, "pages")])
        message = refused(go)
        if message == "REFUSED-NOTHING" or not message:
            return ("a new frame whose fixture declared nothing recorded anyway -- one env per "
                    "directory is not a licence to stamp an unrelated fixture's clock on it")
        if "users-no-profiles" not in message:
            return f"the refusal does not name the unbound artifact: {message[:120]!r}"
        return None

    rule("a new frame with no conditions for its own fixture is refused",
         in_box(a_second_new_frame_needs_its_own_conditions))

    def schema_decides_whether_silence_is_a_finding(box):
        """/1 predates conditions and stays readable; /2 promises them, so an entry without a block
        is a hole in a promise and has to be named."""
        _png(box, "profiles.png")
        d = os.path.join(box, "docs", "screenshots", "360")
        entry = {"page": "/profiles", "png": "profiles.png",
                 "png_sha256": sha256(os.path.join(d, "profiles.png")),
                 "sources": sources_for("profiles", "/profiles"), "sources_rev": RECORDED}
        for schema, want_problem in ((SCHEMA_PLAIN, False), (SCHEMA, True)):
            doc = {"schema": schema, "viewport": ARTIFACT_WIDTH, "head": RECORDED,
                   "captured_at": "2026-01-01T00:00:00Z", "pages": [dict(entry)]}
            with open(os.path.join(d, MANIFEST), "w", encoding="utf-8") as fh:
                json.dump(doc, fh)
            problems = check(d, run=_fake_git(), pages=[_Page("/profiles", "profiles")])
            named = any("no `capture` block" in p for p in problems)
            if named != want_problem:
                return (f"{schema}: an entry with no capture block produced "
                        f"{'a finding' if named else 'silence'}, want "
                        f"{'a finding' if want_problem else 'silence'}")
        # And a schema name from neither generation is its own sentence, not a silent pass.
        doc["schema"] = "capture-manifest/0"
        with open(os.path.join(d, MANIFEST), "w", encoding="utf-8") as fh:
            json.dump(doc, fh)
        if not any("capture-manifest/0" in p for p in
                   check(d, run=_fake_git(), pages=[_Page("/profiles", "profiles")])):
            return "a manifest from an unknown generation was not named as one"
        return None

    rule("a /1 manifest stays readable and a /2 manifest is held to its promise",
         in_box(schema_decides_whether_silence_is_a_finding))

    def a_condition_that_cannot_be_true_is_refused(box):
        """The two shapes that make a recorded condition worse than an absent one: a value nobody
        declares, and conditions copied from another fixture's env."""
        _png(box, "users-no-profiles.png")
        d = os.path.join(box, "docs", "screenshots", "360")
        spec = _Page("/users", "users-no-profiles", fixture="pages-noprof",
                     state=(("casey", "no profile"),))
        good = {"page": "/users", "png": "users-no-profiles.png",
                "png_sha256": sha256(os.path.join(d, "users-no-profiles.png")),
                "sources": sources_for("users-no-profiles", "/users"), "sources_rev": RECORDED,
                "capture": {"fixture": "pages-noprof", "stamp": "2026-03-18T21:34:05",
                            "scripts": "off", "state": [["casey", "no profile"]]}}

        def check_with(capture):
            entry = dict(good)
            entry["capture"] = capture
            doc = {"schema": SCHEMA, "viewport": ARTIFACT_WIDTH, "head": RECORDED,
                   "captured_at": "2026-01-01T00:00:00Z", "pages": [entry]}
            with open(os.path.join(d, MANIFEST), "w", encoding="utf-8") as fh:
                json.dump(doc, fh)
            return check(d, run=_fake_git(), pages=[spec])

        if check_with(dict(good["capture"])):
            return f"the honest block was refused: {check_with(dict(good['capture']))[:1]}"
        wrong_fixture = dict(good["capture"], fixture="pages")
        if not any("photographs /users from" in p for p in check_with(wrong_fixture)):
            return ("conditions from one fixture recorded against a page another fixture "
                    "photographs, and nothing was refused -- a confident wrong answer")
        typo = dict(good["capture"], scripts="maybe")
        if not any("scripts=" in p for p in check_with(typo)):
            return "scripts=maybe is not a condition anyone declares, and it passed"
        # #213's key, in the same two shapes: a census that is not a count, and a census that is a
        # count of nothing. `pages` records no `sessions` at all and must stay green — the pages
        # fixture gets its census from the stub loginctl and says so through `census`, so a rule that
        # demanded the new key from every frame would refuse five honest entries.
        if any("sessions=" in p for p in check_with(dict(good["capture"]))):
            return ("an entry with no `sessions` key was refused; the pages fixture declares no "
                    "census of its own and must stay recordable without one (#213)")
        for bad_census in ("two", "1 ", ""):
            if not any("sessions=" in p for p in check_with(dict(good["capture"],
                                                                sessions=bad_census))):
                return (f"sessions={bad_census!r} is not a count, and it passed: a census with a "
                        f"typo in it reads as a condition and matches no page")
        if any("sessions=" in p for p in check_with(dict(good["capture"], sessions="8"))):
            return "a well-formed census was refused, so the rule refuses the shape and not the defect"
        stale = dict(good["capture"], state=[["casey", "assigned"]])
        if not any("no longer builds" in p for p in check_with(stale)):
            return ("an entry describing a state the driver no longer declares passed, so the "
                    "manifest can certify a reading of the page the fixture cannot produce")
        no_fixture = dict(good["capture"])
        del no_fixture["fixture"]
        if not any("names no fixture" in p for p in check_with(no_fixture)):
            return "a block that belongs to no setup script passed"
        return None

    rule("a condition that cannot be true is refused, in each shape it takes",
         in_box(a_condition_that_cannot_be_true_is_refused))

    def a_frame_taken_with_the_scripts_running_is_refused(box):
        """A legal condition can still disqualify the artifact.

        `cdp.scripts_disabled` honours `PLOCK_CAPTURE_SCRIPTS`, so a frame taken with the page's
        scripts running is one environment variable away -- that knob is what makes the condition
        measurable at all. What it buys is a measurement, not evidence for this folder's criterion,
        and the guard here is what keeps the two apart: without it the cheapest way to satisfy
        `scripts` is to commit whichever frame the last run happened to leave in the directory.
        Built as a pair, because a guard that refuses both values is a ban on the field and would
        pass for the wrong reason.
        """
        _png(box, "users-no-profiles.png")
        d = os.path.join(box, "docs", "screenshots", "360")
        spec = _Page("/users", "users-no-profiles", fixture="pages-noprof",
                     state=(("casey", "no profile"),))
        cap = {"fixture": "pages-noprof", "stamp": "2026-03-18T21:34:05", "scripts": "on",
               "state": [["casey", "no profile"]]}
        entry = {"page": "/users", "png": "users-no-profiles.png",
                 "png_sha256": sha256(os.path.join(d, "users-no-profiles.png")),
                 "sources": sources_for("users-no-profiles", "/users"), "sources_rev": RECORDED,
                 "capture": cap}
        doc = {"schema": SCHEMA, "viewport": ARTIFACT_WIDTH, "head": RECORDED,
               "captured_at": "2026-01-01T00:00:00Z", "pages": [entry]}

        def write_and_check(capture):
            entry["capture"] = capture
            with open(os.path.join(d, MANIFEST), "w", encoding="utf-8") as fh:
                json.dump(doc, fh)
            return check(d, run=_fake_git(), pages=[spec])

        problems = write_and_check(dict(cap))
        if not any("JavaScript off" in p for p in problems):
            return ("a frame whose entry says the page's scripts were running committed green, so "
                    f"the artifact certifies a criterion it never tested: {problems[:1]}")
        off = write_and_check(dict(cap, scripts="off"))
        if off:
            return ("the same block with scripts='off' was refused too, so this is a ban on naming "
                    f"the condition rather than a guard on the criterion: {off[:1]}")
        return None

    rule("a frame taken with the page's scripts running is not evidence for the criterion",
         in_box(a_frame_taken_with_the_scripts_running_is_refused))

    def a_fixture_env_path_that_names_nothing_is_refused(box):
        """The directory name IS the binding, so a path that cannot say it is not a conditions file."""
        def go(path):
            return refused(lambda: read_fixture_env(path))
        for path, want in ((os.path.join(box, "pages", "env"), "fixture.env"),
                           (os.path.join(box, "fixture.env"), "directory")):
            message = go(path)
            if message == "REFUSED-NOTHING" or not message:
                return f"{path} was accepted as a fixture env"
            if want not in message:
                return f"{path}: the refusal does not say why: {message[:120]!r}"
        missing = refused(lambda: read_fixture_env(os.path.join(box, "gone", "fixture.env")))
        if missing == "REFUSED-NOTHING" or not missing:
            return "an unreadable fixture env recorded nothing and said nothing"
        if "gone-setup.sh" not in missing:
            return f"the refusal does not name the setup script to run: {missing[:120]!r}"
        return None

    rule("a fixture env is found by the directory that names its fixture",
         in_box(a_fixture_env_path_that_names_nothing_is_refused))

    def the_operators_door_is_open(box):
        """The CLI is tested by running it, because three textual checks in a row were satisfied
        by prose.

        Version one grepped `main`'s source for `--fixture-env` and survived the flag being renamed,
        because main's closing sentence -- "no --fixture-env was given" -- contains the string.
        Version two grepped the rendered `--help`, and survived the flag being DELETED, because the
        help of the flag next to it says "its conditions are taken from --fixture-env". A CLI's own
        narration is the worst possible place to look for evidence that a flag exists: it is written
        to describe the flag whether or not the flag is there.

        So: hand the parser the tokens, and check they arrive at `record`. `record` is swapped for a
        stub, which also means nothing is written and no git is consulted, and `--help` is never
        passed -- argparse acts on `--help` the moment it sees it and never validates the rest, which
        is how version two's successor stayed green on a flag that no longer parsed.
        """
        import contextlib
        import io

        seen = {}

        def stub(directory, head=None, run=git, now=None, envs=(), shot=()):
            seen["dir"], seen["envs"], seen["shot"] = directory, list(envs), list(shot)
            return {"schema": SCHEMA_PLAIN, "viewport": ARTIFACT_WIDTH, "pages": []}

        def cli(argv):
            """`record`'s kwargs, or None when the CLI refused to take the tokens at all."""
            seen.clear()
            try:
                with contextlib.redirect_stdout(io.StringIO()):
                    main(argv)
            except SystemExit:
                return None
            return dict(seen)

        saved, env = record, env_file(box, "pages")
        globals()["record"] = stub
        try:
            got = cli(["record", "--dir", box, "--fixture-env", env, "--shot", "profiles"])
            if got is None:
                return ("record --fixture-env/--shot no longer parse, so the conditions this file "
                        "keeps can only be recorded from Python -- while this file's prose goes on "
                        "describing both flags to whoever types them")
            if got.get("envs") != [env]:
                return (f"--fixture-env parsed but reached record as {got.get('envs')!r}: the flag "
                        f"would be accepted and dropped, which is worse than a missing flag because "
                        f"nothing complains")
            if got.get("shot") != ["profiles"]:
                return (f"--shot parsed but reached record as {got.get('shot')!r}, so a frame this "
                        f"run photographed would keep the conditions in the file instead of the ones "
                        f"it was just taken under")
            # Repeatable, both of them: the committed set is recorded with one env per fixture.
            second = env_file(box, "lock")
            got = cli(["record", "--dir", box, "--fixture-env", env, "--fixture-env", second,
                       "--shot", "profiles", "--shot", "lock"])
            if got is None or got.get("envs") != [env, second] or got.get("shot") != ["profiles",
                                                                                       "lock"]:
                return ("neither flag is repeatable any more, so the committed set -- nine artifacts "
                        "from three fixtures -- cannot be recorded in one command")
        finally:
            globals()["record"] = saved
        return None

    rule("record --fixture-env is still the operator's door", in_box(the_operators_door_is_open))

    def a_retake_of_one_page_demands_nothing_of_the_frames_it_never_touched(box):
        """The committed set's real shape: nine artifacts, three drivers, one recorder.

        `docs/screenshots/360` holds five frames from the pages fixture, one from pages-noprof, and
        `lock.png` / `testlock.png` from the two POST drivers -- which record nothing, because only
        `pages.py` calls `record`. Re-take one page and the recorder is handed one fixture's env over a
        directory containing two frames from fixtures that were not involved. If those two were treated
        as unaccounted-for, the honest workflow would go red -- the #144 trap wearing #211's clothes --
        and if the document were called /2 anyway, `check` would then refuse the very frames it cannot
        know anything about. So: an untouched frame with no block keeps stating nothing, the manifest
        stays /1, and the schema only becomes /2 when every frame in the directory says what it was
        taken under.
        """
        d = os.path.join(box, "docs", "screenshots", "360")
        os.makedirs(d, exist_ok=True)
        for a in ("profiles", "profiles-refused", "lock"):
            _png(box, a + ".png")
        # The manifest as committed today: every entry, no conditions anywhere, schema /1.
        entry = lambda a, page: {"page": page, "png": a + ".png",
                                 "png_sha256": sha256(os.path.join(d, a + ".png")),
                                 "sources": sources_for(a, page), "sources_rev": RECORDED}
        doc = {"schema": SCHEMA_PLAIN, "viewport": ARTIFACT_WIDTH, "head": RECORDED,
               "captured_at": "2026-01-01T00:00:00Z",
               "pages": [entry("profiles", "/profiles"),
                         entry("profiles-refused", "/profiles"),
                         entry("lock", "/lock-now")]}
        with open(os.path.join(d, MANIFEST), "w", encoding="utf-8") as fh:
            json.dump(doc, fh)

        # One page is re-taken; only its fixture's env is in hand, as pages.py hands it -- with its
        # `--shot` list too, because that is what `pages.py:1097` actually passes
        # (`shot=[spec.artifact for spec in selected]`), and since #294 a changed frame that stays
        # unnamed is a refusal rather than a re-dating.
        _png(box, "profiles.png", body=b"a re-taken frame")
        # `refused` and not a bare call: a rule that lets the SystemExit escape crashes the whole
        # self-test with a traceback instead of naming the rule, and a gate that crashes is read as
        # "something is wrong somewhere" rather than "this workflow went red".
        complaint = refused(lambda: record(d, run=_fake_git(), now="2026-02-01T00:00:00Z",
                                           envs=[env_file(box, "pages")], shot=["profiles"]))
        # `refused` reports silence as the sentinel string, because "nothing was refused" is the
        # answer a refusal-wanting rule needs to distinguish from a real message.
        if complaint and complaint != "REFUSED-NOTHING":
            return f"the honest re-take was refused: {str(complaint)[:160]}"
        after = read_manifest(d)
        by_png = {e["png"]: e for e in after["pages"]}
        if after["schema"] != SCHEMA_PLAIN:
            return (f"the manifest became {after['schema']!r} while lock.png still states nothing, "
                    f"so check would hold an entry to a promise nobody made for it")
        if "capture" not in by_png["profiles.png"]:
            return "the re-taken frame got no conditions, so the run that took it stated nothing"
        if by_png["profiles.png"]["capture"].get("fixture") != "pages":
            return f"the re-taken frame was stamped {by_png['profiles.png']['capture']!r}"
        if "capture" in by_png["lock.png"]:
            return ("lock.png acquired conditions from the pages fixture's env -- a block nobody "
                    "declared for it, which is the confident wrong answer #211 refuses")
        shot = [_Page("/profiles", "profiles"), _Page("/profiles", "profiles-refused")]
        if check(d, run=_fake_git(), pages=shot):
            return (f"the honest re-take was refused: {check(d, run=_fake_git(), pages=shot)[:1]}")

        # And when every fixture involved hands over its env, the directory really is /2.
        record(d, run=_fake_git(), now="2026-03-01T00:00:00Z",
               envs=[env_file(box, "pages"), env_file(box, "lock")])
        full = read_manifest(d)
        if full["schema"] != SCHEMA:
            return f"every frame stated its conditions and the document stayed {full['schema']!r}"
        if not all("capture" in e for e in full["pages"]):
            return "a /2 manifest with an entry that states nothing"
        if check(d, run=_fake_git(), pages=shot):
            return f"a complete /2 manifest was refused: {check(d, run=_fake_git(), pages=shot)[:1]}"
        return None

    rule("a re-take of one page demands nothing of the frames it never touched",
         in_box(a_retake_of_one_page_demands_nothing_of_the_frames_it_never_touched))

    def a_path_in_the_manifest_is_the_repositorys(box):
        """`census` is the one condition whose env value is a filesystem path."""
        inside = os.path.join(ROOT, "testing", "capture", "bin")
        if repo_relative(inside) != os.path.join("testing", "capture", "bin"):
            return (f"an in-repo absolute path recorded as {repo_relative(inside)!r}; the manifest is "
                    f"a committed file and that string is one checkout on one machine")
        outside = os.path.join(os.path.dirname(os.path.realpath(ROOT)), "elsewhere", "bin")
        if repo_relative(outside) != outside:
            return (f"a path outside the repository was rewritten to {repo_relative(outside)!r}; a "
                    f"daemon whose PATH starts elsewhere has that as a condition, and a `..` "
                    f"spelling invents a location the fixture never had")
        relative = "testing/capture/bin"
        if repo_relative(relative) != relative:
            return f"an already-relative value was rewritten to {repo_relative(relative)!r}"
        # And through the env, which is where the absolute value actually arrives.
        path = env_file(box, "pages", PLOCK_CAPTURE_PATH_PREPEND=inside)
        name, conditions = read_fixture_env(path)
        if conditions.get("census") != os.path.join("testing", "capture", "bin"):
            return f"the env's absolute census recorded as {conditions.get('census')!r}"
        blob = json.dumps(recorded(box, envs=[path]))
        if os.path.realpath(ROOT) in blob:
            return "the manifest carries the repository root in it"
        return None

    rule("a path in the manifest is the repository's, not the capturing machine's",
         in_box(a_path_in_the_manifest_is_the_repositorys))

    def a_frame_this_run_took_is_stamped_from_this_run(box):
        """Bytes are not provenance.

        Move the fixture's declared clock and `settings.png` comes back byte-identical -- nothing on
        that page prints a time. A recorder that decides by digest alone would then keep the old clock
        on a frame that was just taken under the new one, and the failure runs the other way as well:
        a fixture change would refuse its own capture, because the honest re-run looks like a
        contradiction. So the driver says what it photographed and the env it was run under wins.
        """
        env_a = env_file(box, "pages", PLOCK_CAPTURE_STAMP="2026-03-18T21:34:05")
        recorded(box, envs=[env_a])
        d = os.path.join(box, "docs", "screenshots", "360")
        env_b = env_file(box, "pages", PLOCK_CAPTURE_STAMP="2027-01-01T00:00:00")

        # The same bytes, re-photographed: no _png() call, so no digest changed. The notes this
        # provokes for the frames left alone are asserted by the rule below; swallowed here so the
        # self-test prints its own line and nothing else.
        import contextlib
        import io
        with contextlib.redirect_stderr(io.StringIO()):
            record(d, run=_fake_git(), now="2026-02-01T00:00:00Z", envs=[env_b], shot=["settings"])
        after = {e["png"]: e for e in read_manifest(d)["pages"]}
        if after["settings.png"]["capture"]["stamp"] != "2027-01-01T00:00:00":
            return ("a frame re-photographed under a moved clock kept the old one, so the manifest "
                    "describes a capture that no longer exists")
        if after["audit.png"]["capture"]["stamp"] != "2026-03-18T21:34:05":
            return ("a frame this run did not photograph was re-stamped anyway -- the driver's word "
                    "is the only thing separating 'taken under today' from 'certified before'")

        # A frame this run took, with no env for its fixture, is the #207 shape and refuses.
        message = refused(lambda: record(d, run=_fake_git(), now="2026-03-01T00:00:00Z",
                                        envs=[env_file(box, "lock")], shot=["settings"]))
        if message == "REFUSED-NOTHING" or not message:
            return ("a photographed frame with no conditions for its own fixture recorded anyway, "
                    "so `shot` turned a refusal into a guess")
        if "photographed it" not in message:
            return f"the refusal does not say what happened to the frame: {message[:120]!r}"
        return None

    rule("a frame this run photographed is stamped from this run, not from the file",
         in_box(a_frame_this_run_took_is_stamped_from_this_run))

    # ---- #222: the same rule applied to the field the GATE reads ---------------------
    #
    # Everything above is about `capture`, and the closure of #222 read as though it settled the
    # issue. It did not: `sources_rev` is written from the commit and the clock for every PNG in the
    # directory, and `check`'s drift arm opens `if recorded == current: continue`, so a record that
    # photographs one frame mints the certificate for the frames it left alone. Two rules, because
    # the finding has two halves -- what the field must say, and what the gate must then do.

    def an_untouched_frame_keeps_the_rev_it_was_rendered_from(box):
        import contextlib
        import io
        env = env_file(box, "pages")
        first = recorded(box, envs=[env])
        if first["head"] == "false":
            return ("`record` still answers `rev-parse HEAD` with the shallow verdict, so `head` is a "
                    "placeholder and `carried_from` would name a string that is not a commit")
        d = os.path.join(box, "docs", "screenshots", "360")
        _png(box, "profiles.png", body=b"re-taken under the moved page")
        moved = _fake_git(rev="b" * 40)
        # The re-take's own word, planted before the record (#302). This is the shape `7e65d3b` was
        # NOT: a frame named with `--shot` while its committed pixels sit at the older commit. The
        # rule's own claim -- that the four frames this run left alone keep their rev -- is only
        # testable if the one frame it did take is allowed to be recorded, and the only thing that
        # now lets it be recorded is the receipt its driver would have left.
        receipt(box, "pages", ["profiles"], head="b" * 40, at="2026-01-02T00:00:00Z")
        with contextlib.redirect_stderr(io.StringIO()):
            record(d, run=moved, now="2026-01-02T00:00:00Z", envs=[env], shot=["profiles"])
        after = {e["png"]: e for e in read_manifest(d)["pages"]}
        lied = sorted(n for n, e in after.items()
                      if n != "profiles.png" and e.get("sources_rev") != RECORDED)
        if lied:
            return (f"{lied[0]} was re-stamped to the current commit by a run that did not photograph "
                    f"it, and `recorded == current` is the arm `check` short-circuits on -- the "
                    f"re-stamp is not a wrong sentence about the frame, it is the refusal switched "
                    f"off (#222)")
        if after["profiles.png"].get("sources_rev") != "b" * 40:
            return ("the frame this run DID photograph kept the old rev, so `shot` decides nothing "
                    "and the honest re-take is indistinguishable from the stale frame")
        if after["audit.png"].get("carried_from") != first["head"]:
            return ("a carried frame does not name the manifest its rev came from, so the field that "
                    "says 'this one is older' cannot be audited")
        problems = check(d, run=moved, pages=DRIVER)
        if not problems:
            return ("a partial record left four frames checking clean over a page that had moved: "
                    "0 problems where there are four, which is what #222 measured on this file")
        missing = [a for a in ("audit", "dashboard", "settings", "users")
                   if not any(p.startswith(a + ".png") and "predates" in p for p in problems)]
        if missing:
            return f"carried frames that did not travel with the page were not refused: {missing}"
        return None

    rule("a frame this run did not take keeps the rev its bytes were rendered from",
         in_box(an_untouched_frame_keeps_the_rev_it_was_rendered_from))

    def the_green_sentence_names_the_frames_it_did_not_take(box):
        import contextlib
        import io
        env = env_file(box, "pages")
        recorded(box, envs=[env])
        d = os.path.join(box, "docs", "screenshots", "360")
        _png(box, "profiles.png", body=b"re-taken")
        with contextlib.redirect_stderr(io.StringIO()):
            record(d, run=_fake_git(), now="2026-01-02T00:00:00Z", envs=[env], shot=["profiles"])
        doc = read_manifest(d)
        quiet = check(d, run=_fake_git(), pages=DRIVER)
        if quiet:
            return f"the carried set did not check clean before the sentence was read: {quiet[0]}"
        line = summary(doc)
        if "carried" not in line or "did not re-take them" not in line:
            return (f"the green sentence counts {len(doc['pages'])} photographs without saying which "
                    f"of them this run did not take: {line}")
        # The other direction, because it is the same claim from the reader's side: a run that really
        # does take the set clears the carry instead of adding to it.
        with contextlib.redirect_stderr(io.StringIO()):
            record(d, run=_fake_git(), now="2026-01-03T00:00:00Z", envs=[env],
                   shot=["dashboard", "users", "profiles", "audit", "settings"])
        after = read_manifest(d)
        if any(e.get("carried_from") for e in after["pages"]):
            return "a record that took the whole set still carries frames, so the clause never clears"
        if "carried" in summary(after):
            return "the whole-set sentence still claims frames were carried"
        return None

    rule("the green sentence names the frames the record did not take, and a full set clears it",
         in_box(the_green_sentence_names_the_frames_it_did_not_take))

    # ---- #294: the frame nobody claimed ------------------------------------------------
    #
    # #222 stopped `record` moving a rev for a frame it did not take by CARRYING the prior entry's
    # value, and the carry needs three things at once: a prior entry, identical bytes, and a rev to
    # carry. Two inputs still reached the mint (issue #294, both measured at d7a5149 through real
    # git over a throwaway repository, not read off this file): bytes swapped in from another page,
    # and a frame the manifest has never seen. Both are refused at record time now, because `check`
    # cannot be the last line -- by the time it runs, the value its drift arm compares is the one
    # `record` invented, and the swap's own commit is the newest commit touching the PNG.

    def a_changed_frame_nobody_named_is_refused(box):
        """Arm A: bytes the manifest does not recognise, and no `--shot` claiming them."""
        d = os.path.join(box, "docs", "screenshots", "360")
        env = env_file(box, "pages")
        before = recorded(box, envs=[env])
        _png(box, "audit.png", body=b"another page's frame, swapped in")
        message = refused(lambda: record(d, run=_fake_git(), now="2026-02-01T00:00:00Z",
                                         envs=[env], shot=["profiles"]))
        if message == "REFUSED-NOTHING":
            return ("a frame whose bytes are not the ones the manifest recorded was re-dated by a run "
                    "that did not claim it, and `recorded == current` is the arm `check` "
                    "short-circuits on -- the re-dating is the identity finding deleted (#294)")
        if "audit.png" not in message:
            return f"the refusal does not name the frame it is refusing: {message[:120]!r}"
        if "--shot" not in message:
            return f"the refusal does not name the flag that resolves it: {message[:120]!r}"
        # A refusal that wrote anything is the bug with extra steps: the manifest on disk has to stay
        # the one that still describes the swap as a finding.
        if read_manifest(d)["pages"] != before["pages"]:
            return "the refused record rewrote the manifest, which is the thing #294 refuses to do"
        problems = check(d, run=_fake_git(), pages=DRIVER)
        if not any(p.startswith("audit.png") and "not the file" in p for p in problems):
            return (f"after the refusal `check` no longer names the swap, so the finding is gone "
                    f"either way: {problems[:1]}")
        # The way out, because a refusal with no route through it is a dead directory: the frame is
        # honest as soon as the run says it took it, and its conditions are then this run's.
        again = refused(lambda: record(d, run=_fake_git(), now="2026-02-02T00:00:00Z",
                                       envs=[env], shot=["profiles", "audit"]))
        if again != "REFUSED-NOTHING":
            return (f"naming the frame in `--shot` did not clear the refusal, so an honest re-take "
                    f"has no route through it: {str(again)[:120]!r}")
        after = {e["png"]: e for e in read_manifest(d)["pages"]}
        if (after["audit.png"].get("capture") or {}).get("stamp") != "2026-03-18T21:34:05":
            return ("the named frame did not take this run's conditions, so the declaration was "
                    "believed for the rev and ignored for the block")
        return None

    rule("a frame whose bytes moved and nobody claimed is refused, and a named re-take clears it",
         in_box(a_changed_frame_nobody_named_is_refused))

    def a_frame_new_to_the_manifest_with_older_pixels_is_refused(box):
        """Arm B: nothing to carry, and the only rev available is the one the run stands in.

        `lock.py` and `testlock.py` write no manifest, so their frames reach the committed set with no
        entry and pixels from whatever commit last touched them (#294 names `lock.png` and
        `testlock.png` at `26f19f181`/`99099f982`, both behind the commit that moved the shared page).
        The refusal is a comparison, not the absence: an uncommitted capture makes EVERY frame new to
        the manifest, and that ordering is what the notes and `fixture.sh` tell people to do.
        """
        d = os.path.join(box, "docs", "screenshots", "360")
        env = env_file(box, "pages")
        before = recorded(box, envs=[env])
        _png(box, "lock.png", body=b"a frame lock.py photographed and nobody recorded")
        moved = _fake_git(rev="b" * 40)          # the page has moved since those pixels were taken
        message = refused(lambda: record(d, run=moved, now="2026-02-01T00:00:00Z",
                                         shot=["profiles"]))
        if message == "REFUSED-NOTHING":
            return ("a frame with no prior entry was dated at the current commit by a run that did "
                    "not claim it, while its own pixels were last committed a commit earlier -- the "
                    "mint writes #222's sentence and the drift arm then compares a fabricated rev "
                    "against nothing (#294)")
        if "lock.png" not in message:
            return f"the refusal does not name the frame it is refusing: {message[:120]!r}"
        if "have no entry in" not in message:
            return (f"the refusal is not the unrecorded-frame arm, so something else fired first: "
                    f"{message[:160]!r}")
        if "--shot" not in message or "--fixture-env" not in message:
            return f"the refusal names neither flag that resolves it: {message[:120]!r}"
        if read_manifest(d)["pages"] != before["pages"]:
            return "the refused record rewrote the manifest"
        problems = check(d, run=moved, pages=DRIVER)
        if not any(p.startswith("lock.png") and "no entry" in p for p in problems):
            return (f"after the refusal `check` no longer calls the frame unrecorded, so the "
                    f"finding is gone either way: {problems[:1]}")
        # The honest ordering this file already tells people to use: record the frame in the commit
        # that photographs it, bare. Then its pixels have nothing to disagree with and there is
        # nothing to refuse -- which is why an absence of history is not the trigger above, and why a
        # refusal keyed on "no entry" instead of on the comparison is #144's trap: `record` over a
        # directory of frames nobody has committed yet is the normal thing to do, and `pages.py`
        # records before anything of its run is committed. Both spellings of "this run's own frames"
        # have to survive: untracked (`pages.py`'s ordering) and committed in the very commit the
        # manifest is written at (the notes' ordering).
        for label, honest in (("untracked", _fake_git(rev="b" * 40, png_rev=None)),
                              ("committed in this very commit", _fake_git(rev="b" * 40,
                                                                         png_rev="b" * 40))):
            again = refused(lambda: record(d, run=honest, now="2026-02-02T00:00:00Z"))
            if again != "REFUSED-NOTHING":
                return (f"a frame with nothing to disagree with ({label}) was refused, so the "
                        f"documented ordering -- record the run's own frames, bare -- is red: "
                        f"{str(again)[:120]!r}")
        entry = [e for e in read_manifest(d)["pages"] if e["png"] == "lock.png"]
        if not entry or entry[0]["sources_rev"] != "b" * 40:
            return f"the frame with no history was not recorded at the current commit: {entry[:1]}"
        return None

    rule("a frame new to the manifest with pixels older than its page is refused, and a frame with "
         "no history at all is not",
         in_box(a_frame_new_to_the_manifest_with_older_pixels_is_refused))

    def a_moved_declaration_is_said_not_reconciled(box):
        """An untouched frame whose env now disagrees keeps its block, and the file says so."""
        import contextlib
        import io
        d = os.path.join(box, "docs", "screenshots", "360")
        recorded(box, envs=[env_file(box, "pages", PLOCK_CAPTURE_SCRIPTS="on")])
        stream = io.StringIO()
        with contextlib.redirect_stderr(stream):
            record(d, run=_fake_git(), now="2026-02-01T00:00:00Z",
                   envs=[env_file(box, "pages", PLOCK_CAPTURE_SCRIPTS="off")])
        said = stream.getvalue()
        if "scripts" not in said or "'on'" not in said or "'off'" not in said:
            return (f"a declaration that moved was said silently: {said[:160]!r} -- measured on "
                    f"the two POST fixtures, where the value that moved was a correction")
        after = {e["png"]: e for e in read_manifest(d)["pages"]}
        if after["settings.png"]["capture"]["scripts"] != "on":
            return ("an untouched frame was rewritten to today's declaration, which is the silent "
                    "re-attribution #211 refuses: this run did not take that frame")
        return None

    rule("a declaration that moved is said out loud, not reconciled",
         in_box(a_moved_declaration_is_said_not_reconciled))

    def a_shot_with_no_png_is_refused(box):
        """The driver's word is evidence, so it has to be checkable against the directory."""
        def go():
            d = os.path.join(box, "docs", "screenshots", "360")
            _png(box, "profiles.png")
            record(d, run=_fake_git(), now="2026-01-01T00:00:00Z", envs=[env_file(box, "pages")],
                   shot=["profiles", "profiles-nosuch"])
        message = refused(go)
        if message == "REFUSED-NOTHING" or not message:
            return ("a run claimed to have photographed a frame that is not in the directory and "
                    "nothing was refused, so `shot` is a licence to attach conditions to nothing")
        if "profiles-nosuch" not in message:
            return f"the refusal does not name the phantom artifact: {message[:120]!r}"
        return None

    rule("a run cannot report photographing a frame that is not there",
         in_box(a_shot_with_no_png_is_refused))

    # ---- #302: the driver's word, made checkable --------------------------------------
    #
    # `--shot` dates a frame at its PAGE's newest commit and never at its own pixels, and
    # `check`'s `recorded == current` arm compares the value `record` just wrote -- so the mint is
    # invisible from the other end. `7e65d3b` is the specimen, measured on the delivered set: nine
    # names typed eighteen seconds after `2088043cb`, no PNG in the commit, seven rows on a date
    # their pixels predate, `check` answering `9 photographs current` with rc 0. The repair is the
    # run's own receipt (`cdp.write_receipt`, at `runs/<fixture>/capture.json`, read from beside the
    # `--fixture-env` that names the fixture). Seven rules, because the repair has three parts and
    # two boundaries: a contradicted mint is refused without a receipt and cleared by one; a receipt
    # that cannot be attached to these exact bytes is worth nothing; the writer and the reader are
    # one shape; the pre-commit ordering stays open (#302's own M1 measurement, which is the
    # over-refusal this file's #144 lesson forbids); and a claim that goes stale goes red without
    # becoming the unsatisfiable advice #338 is filed against.

    def a_claimed_frame_behind_its_page_is_refused_without_a_receipt(box):
        env = env_file(box, "pages")
        first = recorded(box, envs=[env])
        d = os.path.join(box, "docs", "screenshots", "360")
        _png(box, "profiles.png", body=b"re-taken, and not committed with the page")
        moved = _fake_git(rev="b" * 40)
        message = refused(lambda: record(d, run=moved, now="2026-01-02T00:00:00Z", envs=[env],
                                        shot=["profiles"]))
        if message in (None, "REFUSED-NOTHING"):
            return ("`--shot profiles` re-dated a frame whose committed pixels sit at the older "
                    "commit and nothing was refused. That mint is the value `check`'s "
                    "`recorded == current` arm accepts, and #302 measured the whole delivered set "
                    "green over it")
        if RECEIPT not in message:
            return (f"the refusal does not name the file that answers it: {message[:160]!r}. An "
                    "operator told only that a date is wrong has no door to knock on")
        if read_manifest(d)["head"] != first["head"]:
            return "the refused record wrote the manifest anyway"

        # The same run, holding the word its driver would have left.
        receipt(box, "pages", ["profiles"], head="b" * 40, at="2026-01-02T00:00:00Z")
        record(d, run=moved, now="2026-01-02T00:00:00Z", envs=[env], shot=["profiles"])
        after = {e["png"]: e for e in read_manifest(d)["pages"]}
        claim = after["profiles.png"].get("photographed")
        if not claim or claim.get("head") != "b" * 40:
            return (f"a receipt covering the page did not clear the frame: {claim!r}. Refusing the "
                    "honest re-take with the dishonest one is the mistake #338 is filed against")
        if claim.get("sha256") != sha256(os.path.join(d, "profiles.png")):
            return "the block does not name the bytes it just cleared"
        if after["profiles.png"].get("sources_rev") != "b" * 40:
            return "the cleared frame did not get the date the receipt proves"
        # And the boundary the whole design rests on: a frame nobody claims is untouched by this.
        # `newest_commit(png)` answering empty -- or answering behind the page, for a set recorded
        # before its run is committed -- is the ordering every note in this folder tells people to
        # use, and a gate that refuses it is #338's dead end with a stricter lock on it.
        bare = refused(lambda: record(d, run=_fake_git(rev="c" * 40), now="2026-01-03T00:00:00Z",
                                      envs=[env]))
        if bare != "REFUSED-NOTHING":
            return (f"a bare record over a set whose page had moved was refused: {bare[:160]!r}. "
                    "Nothing claimed a new date there, so the frames carry and the ordering the "
                    "README prescribes has to stay open (#302 M1)")
        return None

    rule("a frame named with `--shot` while its own pixels are behind its page is refused without a "
         "capture receipt, and cleared by one",
         in_box(a_claimed_frame_behind_its_page_is_refused_without_a_receipt))

    def a_receipt_from_behind_the_page_dates_nothing(box):
        env = env_file(box, "pages")
        recorded(box, envs=[env])
        d = os.path.join(box, "docs", "screenshots", "360")
        _png(box, "profiles.png", body=b"photographed while the page was still at the old commit")
        # The shape `7e65d3b` actually is: a run whose capture stood at `A`, a page whose newest
        # commit is `B`, and a name typed afterwards. The receipt is real, and it says the opposite
        # of what the date claims -- so it must refuse rather than satisfy.
        receipt(box, "pages", ["profiles"], head=RECORDED, at="2026-01-02T00:00:00Z")
        message = refused(lambda: record(d, run=_fake_git(rev="b" * 40), now="2026-01-02T00:00:00Z",
                                         envs=[env], shot=["profiles"]))
        if message in (None, "REFUSED-NOTHING"):
            return ("a receipt naming a commit BEHIND the page cleared the frame: the check is "
                    "only worth what it refuses, and this is the exact ordering of the commit this "
                    "issue is filed against")
        if "stood at" not in message:
            return f"the refusal does not say which way the two commits fall: {message[:160]!r}"
        return None

    rule("a receipt whose capture stood behind the page refuses the frame instead of clearing it",
         in_box(a_receipt_from_behind_the_page_dates_nothing))

    def a_receipt_that_is_not_about_these_bytes_is_refused(box):
        env = env_file(box, "pages")
        other = env_file(box, "lock")
        recorded(box, envs=[env])
        d = os.path.join(box, "docs", "screenshots", "360")
        # (a) The receipt from another fixture's run, sitting in that fixture's own directory -- so
        # this is not the copy-a-file dodge, it is a real receipt about different frames.
        _png(box, "profiles.png", body=b"someone else's frame")
        receipt(box, "lock", ["profiles"], head="b" * 40)
        message = refused(lambda: record(d, run=_fake_git(rev="b" * 40), now="2026-01-02T00:00:00Z",
                                         envs=[env, other], shot=["profiles"]))
        if message in (None, "REFUSED-NOTHING") or "lock" not in message:
            return (f"a receipt belonging to another fixture's run cleared the frame: "
                    f"{message[:160]!r}")
        # (b) These frames' own receipt, and bytes that are not the ones it names: the file moved
        # after the run that photographed it, which is #294's substitution inside #302's provenance.
        os.remove(os.path.join(box, "runs", "lock", RECEIPT))
        _png(box, "profiles.png", body=b"re-taken under the moved page")
        receipt(box, "pages", ["profiles"], head="b" * 40)
        _png(box, "profiles.png", body=b"and then replaced, after the run that claimed it")
        message = refused(lambda: record(d, run=_fake_git(rev="b" * 40), now="2026-01-02T00:00:00Z",
                                         envs=[env], shot=["profiles"]))
        if message in (None, "REFUSED-NOTHING") or "receipt says" not in message:
            return (f"bytes that are not the ones the receipt vouches for cleared the frame: "
                    f"{message[:160]!r}")
        return None

    rule("a receipt is worth only the bytes and the fixture it was written for",
         in_box(a_receipt_that_is_not_about_these_bytes_is_refused))

    def the_receipt_the_driver_writes_is_the_one_record_reads(box):
        sys.path.insert(0, HERE)
        import cdp
        env = env_file(box, "pages")            # box/runs/pages/fixture.env, the env `record` reads
        d = os.path.join(box, "docs", "screenshots", "360")
        stage_root = os.path.join(box, "runs")
        stage = os.path.join(stage_root, "_stage-selftest")
        os.makedirs(stage, exist_ok=True)
        # `publish` removes the stage directory when it drains it, so each cell puts its own frame
        # in just before it runs -- the way two real runs would.
        with open(os.path.join(stage, "users.png"), "wb") as fh:
            fh.write(b"a second frame, from a run whose fixture declared nothing")
        saved = (cdp.RUNS, cdp._STAGE, list(cdp._PENDING), cdp.git_head)
        try:
            cdp.RUNS = stage_root
            cdp._STAGE = stage
            cdp._PENDING.clear()
            # The commit the capture stood in is the one thing this box cannot get from git: the
            # fake history below knows two revs and no real sha. Stubbed at the probe only, so the
            # bytes of the receipt are `cdp.write_receipt`'s, unchanged.
            cdp.git_head = lambda: "b" * 40
            # Cell one, first because it consumes the conditions file: a run where no fixture
            # declared anything. #211's rule for `fixture.env`, applied to the file whose whole
            # purpose is to be trusted -- a receipt written here would be a claim about a run nobody
            # set up, and the recorder reads it as that fixture's word. The capture must still have
            # RUN: a gate that stops a photograph because a bookkeeping file is missing is how a
            # check becomes an outage, and the frames are owed to `--out` either way.
            os.remove(env)
            # Published somewhere other than the artifact directory: this cell is about the driver's
            # moment, and a frame left in `d` would be a second unclaimed frame for cell two's
            # `record` to refuse -- correctly, but for the wrong reason.
            other = os.path.join(box, "shots-nofixture")
            os.makedirs(other, exist_ok=True)
            cdp._PENDING.append((os.path.join(stage, "users.png"), os.path.join(other, "users.png")))
            cdp.Session.publish(None, "pages")
            where = cdp.receipt_path("pages")
            silent = os.path.exists(where)
            landed, strays = os.path.exists(os.path.join(other, "users.png")), \
                [f for f in os.listdir(other) if f == RECEIPT]
            # The recorder needs that file back: the cell below is the reader's half, and `record`
            # refuses a conditions file it cannot open.
            env_file(box, "pages")
            # Cell two, the round trip: the driver's own publish, then the recorder's own read.
            #
            # `publish` reads the module's own `_PENDING` and `_stage_dir()` and nothing on the
            # instance, and it is the only moment that knows both the bytes and the fixture -- so
            # driving it is the difference between pinning a helper and pinning the run.
            os.makedirs(stage, exist_ok=True)
            with open(os.path.join(stage, "profiles.png"), "wb") as fh:
                fh.write(b"the bytes this run photographed")
            cdp._PENDING.append((os.path.join(stage, "profiles.png"),
                                 os.path.join(d, "profiles.png")))
            cdp.Session.publish(None, "pages")
            if not os.path.exists(where):
                return ("the publish put the frame on disk and wrote no receipt beside it, so the "
                        "run's account of what it photographed exists nowhere and `record` has "
                        "nothing to read (#302)")
            try:
                produced = json.load(open(where, encoding="utf-8"))
            except (OSError, ValueError) as exc:
                return f"the receipt the publish wrote cannot be read: {exc}"
            record(d, run=_fake_git(rev="b" * 40), now="2026-01-02T00:00:00Z", envs=[env],
                   shot=["profiles"])
            entry = {e["png"]: e for e in read_manifest(d)["pages"]}["profiles.png"]
        finally:
            cdp.RUNS, cdp._STAGE, cdp.git_head = saved[0], saved[1], saved[3]
            cdp._PENDING[:] = saved[2]
        if silent:
            return ("a receipt was written where no `fixture.env` exists. The recorder reads that "
                    "file as the fixture's word about its frames, and no fixture ran here (#211's "
                    "lock, one field over)")
        if not landed:
            return ("a run with no conditions file refused to publish the frame it photographed: the "
                    "receipt is the weaker statement, never the capture")
        if strays:
            return (f"{RECEIPT} landed in the artifact directory the driver publishes to. That "
                    "directory is committed, and a transcript of one run on one machine is not "
                    "evidence about the frames beside it")
        if cdp.RECEIPT != RECEIPT:
            return (f"the driver writes {cdp.RECEIPT!r} and the recorder reads {RECEIPT!r}: two "
                    "names for one file is a claim nobody can find")
        if produced.get("fixture") != "pages":
            return f"the receipt does not name its own fixture: {produced!r}"
        frame = produced.get("frames", {}).get("profiles")
        if not frame or frame.get("sha256") != sha256(os.path.join(d, "profiles.png")):
            return (f"the receipt does not name the bytes the publish wrote: {frame!r} vs "
                    f"{sha256(os.path.join(d, 'profiles.png'))[:12]}...")
        if not entry.get("photographed"):
            return ("`record` cannot read what the driver wrote -- the file is on disk and the "
                    f"entry carries only {entry.get('sources_rev')!r}")
        if entry["photographed"].get("head") != "b" * 40:
            return f"the claim survived the round trip wrong: {entry['photographed']!r}"
        return None

    rule("the receipt a driver's publish writes is the one `record` reads, key for key",
         in_box(the_receipt_the_driver_writes_is_the_one_record_reads))

    def an_unusable_claim_is_checked_before_check_uses_it(box):
        env = env_file(box, "pages")
        d = os.path.join(box, "docs", "screenshots", "360")
        doc = recorded(box, envs=[env])
        # `_Page` carries the fixture `check` compares a claim's fixture against. `DRIVER` does not:
        # its rows are built with url and artifact only, so a rule that used it here could not tell
        # a claim naming the right fixture from one naming another -- the wrong-fixture claim would
        # pass on a None, which is #211's "absent is not a verdict" in the reader's seat.
        one = [_Page("/profiles", "profiles", fixture="pages")]

        def with_claim(artifact, **overrides):
            pages = [dict(e) for e in doc["pages"]]
            for e in pages:
                if e["png"] == artifact + ".png":
                    claim = dict(e.get("photographed") or
                                 {"fixture": "pages", "at": "2026-01-01T00:00:00Z",
                                  "head": RECORDED,
                                  "sha256": sha256(os.path.join(d, artifact + ".png"))})
                    claim.update(overrides)
                    e["photographed"] = claim
            with open(os.path.join(d, MANIFEST), "w", encoding="utf-8") as fh:
                json.dump(dict(doc, pages=pages), fh, indent=2, sort_keys=True)
            return check(d, run=_fake_git(), pages=one)

        wrong_bytes = [p for p in with_claim("profiles", sha256="0" * 64)
                       if p.startswith("profiles.png")]
        if not any("the file on disk" in p for p in wrong_bytes):
            return (f"a claim naming bytes that are not the file on disk passed: {wrong_bytes[:1]!r} "
                    "-- that is #294's substitution wearing a provenance block")
        wrong_fixture = [p for p in with_claim("profiles", fixture="lock")
                         if p.startswith("profiles.png")]
        if not any("lock" in p for p in wrong_fixture):
            return f"a claim from another fixture's run passed: {wrong_fixture[:1]!r}"
        no_head = [p for p in with_claim("profiles", head="") if p.startswith("profiles.png")]
        if not any("not enough of a commit" in p for p in no_head):
            return f"a claim naming no commit passed as a date: {no_head[:1]!r}"
        good = [p for p in with_claim("profiles") if p.startswith("profiles.png")]
        if good:
            return f"a well-formed claim about a current frame was refused: {good[:1]!r}"
        return None

    rule("a `photographed` block is verified before `check` is willing to date a frame by it",
         in_box(an_unusable_claim_is_checked_before_check_uses_it))

    def a_claim_the_page_has_moved_past_goes_red_and_a_new_run_clears_it(box):
        env = env_file(box, "pages")
        d = os.path.join(box, "docs", "screenshots", "360")
        _png(box, "profiles.png")
        receipt(box, "pages", ["profiles"], head=RECORDED)
        record(d, run=_fake_git(), now="2026-01-01T00:00:00Z", envs=[env], shot=["profiles"])
        one = [_Page("/profiles", "profiles")]
        # The deadline the mint used to escape: the frame is current for the commit it was written
        # at, the page moves to `b`, and nothing about the file changes. Before #302 this is where
        # the note said "re-take within one round trip" and no check could tell.
        moved = _fake_git(rev="b" * 40)
        stale = [p for p in check(d, run=moved, pages=one) if p.startswith("profiles.png")]
        if not any("was last photographed at commit" in p for p in stale):
            return (f"a page that moved past the commit its own capture named produced {stale[:1]!r}; "
                    "the claim is a date, and a date that stops being true has to go red (#302)")
        # The repair the sentence promises, run through the real path rather than asserted: a new
        # capture, a new receipt, a re-record. #338 is the lesson that a refusal with no reachable
        # green is a lock, so this cell is the rule's other half.
        _png(box, "profiles.png", body=b"re-taken after the page moved")
        receipt(box, "pages", ["profiles"], head="b" * 40, at="2026-01-02T00:00:00Z")
        record(d, run=moved, now="2026-01-02T00:00:00Z", envs=[env], shot=["profiles"])
        clean = [p for p in check(d, run=moved, pages=one) if p.startswith("profiles.png")]
        if clean:
            return (f"the repair the refusal names left the frame still refusing: {clean[:1]!r} -- "
                    "an unsatisfiable refusal is the defect #338 files, not a guard")
        return None

    rule("a claimed frame goes red when its page moves past the commit its capture named, and a "
         "re-take through the receipt clears it",
         in_box(a_claim_the_page_has_moved_past_goes_red_and_a_new_run_clears_it))

    def a_frame_carried_into_its_page_s_own_commit_is_not_refused_by_its_own_receipt(box):
        """The receipt is one commit CONSERVATIVE, and the recipe in README.md is what makes it so.

        That recipe runs the driver and the recorder BEFORE the commit that carries their output --
        `./pages.py --out docs/screenshots/360 --overwrite`, then `manifest.py record`, then
        `git commit`. So the commit a receipt can honestly name is the PARENT of the commit that
        lands the photograph: every fixture, every capture, no exceptions. A `check` that dates a
        claimed frame from the claim alone therefore refuses the one ordering D-0056 was written to
        protect -- "Re-take it and re-run `manifest.py record`, in a commit after the one that
        changed the page **or in that same commit**" -- and #338 files that exact over-refusal
        against this file in general. #302's own list of orderings that must stay legal names it:
        "A frame re-taken and committed in the same commit the page moved in."

        Measured 13 Nov against real git rather than this fake, because the fake cannot say what a
        capture standing in a commit's parent looks like: `/tmp/opencode/302b/probe.py`, case 1, run
        against `origin/main` (`9d15457`) and against this branch. Both trees accept the `record`;
        `main` accepts the commit, and this branch's first cut refused it -- `profiles.png was last
        photographed at commit c9baf3cbe6b5 and /profiles's sources have moved to 473bc5cd0805
        since` -- a sentence false about the frame it printed, invisible to D-0056's rule above
        because that box never held a receipt.

        Two cells, and the second is what keeps the first from being a bypass: a frame whose bytes
        did NOT travel into the page's commit has nothing left to vouch for it, because a claim one
        commit conservative cannot tell a byte-identical re-take in the page's own commit from a
        frame nobody photographed at all. That one stays red -- the residue #338 names, red on
        `origin/main` as well (9 problems on both trees under the same probe), with the reachable
        repair in the rule above.
        """
        env = env_file(box, "pages")
        d = os.path.join(box, "docs", "screenshots", "360")
        one = [_Page("/profiles", "profiles")]
        _png(box, "profiles.png")
        receipt(box, "pages", ["profiles"], head=RECORDED)
        record(d, run=_fake_git(), now="2026-01-01T00:00:00Z", envs=[env], shot=["profiles"])
        if not read_manifest(d)["pages"][0].get("photographed"):
            return "the setup wrote no claim, so this rule would prove nothing"
        # The page moved into `b`, and so did the photograph: one commit, both of them.
        atomic = _fake_git(rev="b" * 40, png_rev="b" * 40)
        travelled = [p for p in check(d, run=atomic, pages=one) if p.startswith("profiles.png")]
        if travelled:
            return (f"a frame re-taken in the commit that moved its page was refused: {travelled[:1]!r}. "
                    "The receipt names the parent because the driver ran before the commit -- that is "
                    "the recipe -- and the frame's own bytes are in the commit that moved the page "
                    "(#302 refuses an ordering D-0056 protects)")
        stayed = [p for p in check(d, run=_fake_git(rev="b" * 40), pages=one)
                  if p.startswith("profiles.png")]
        if not any("was last photographed at commit" in p for p in stayed):
            return (f"a claim whose frame did not move with the page passed: {stayed[:1]!r} -- "
                    "accepting the atomic case on the claim alone would accept the mint too")
        if not any("the newest commit touching the photograph is " + RECORDED[:12] in p
                   for p in stayed):
            return (f"the refusal prints the claim but not the pixels that contradict it, so the "
                    f"reader cannot tell the two witnesses apart: {stayed[:1]!r}")
        return None

    rule("a frame re-taken in the commit that moved its page stays current, and one whose bytes did "
         "not travel still goes red",
         in_box(a_frame_carried_into_its_page_s_own_commit_is_not_refused_by_its_own_receipt))

    def the_carried_note_says_what_carried_from_cannot_reach(box):
        import contextlib
        import io
        env = env_file(box, "pages")
        d = os.path.join(box, "docs", "screenshots", "360")
        # Recorded once where the page and its photograph share a commit -- the atomic case #144
        # exists to allow, and the honest counterpart of `7e65d3b` -- then seen again from a history
        # in which the photograph's newest commit is the OLDER of the two: a rebase, or the retired
        # rev `carried_from` itself points at.
        for a in ("dashboard", "users", "profiles", "audit", "settings"):
            _png(box, a + ".png")
        record(d, run=_fake_git(rev="b" * 40, png_rev="b" * 40), now="2026-01-01T00:00:00Z",
               envs=[env])
        first = read_manifest(d)
        if first["pages"][0].get("sources_rev") != "b" * 40:
            return f"the setup did not mint what it claims: {first['pages'][0]!r}"
        rewrite = _fake_git(rev="b" * 40, png_rev=RECORDED)
        stream = io.StringIO()
        with contextlib.redirect_stderr(stream):
            record(d, run=rewrite, now="2026-01-03T00:00:00Z", envs=[env])
        said = stream.getvalue()
        if "carries" not in said or "own pixels" not in said:
            return (f"a rev newer than its own frame's pixels was carried in silence: {said[:160]!r}. "
                    "`carried_from` is the field a reader audits the carry through, and here it "
                    "names the commit the rev was invented at (#302)")
        if first["head"][:9] not in said:
            return ("the note does not name the manifest it was carried from, so the reader cannot "
                    f"follow it to the mint: {said[:160]!r}")
        if not read_manifest(d)["pages"]:
            return "the note turned into a refusal: a frame nobody claims must still be recordable"
        return None

    rule("a carried rev newer than its own frame's pixels is said out loud, with the mint it came "
         "from", in_box(the_carried_note_says_what_carried_from_cannot_reach))

    def a_mint_is_invisible_to_the_history_and_visible_to_its_own_claim(box):
        """The blind spot, pinned as a decision -- and the one field that reaches past it.

        Three cells, because the ticket's claim is about what `check` CANNOT see and the repair is
        about what it now can. Deleting `if recorded == current: continue` from `check` goes red on
        the second cell, which is the only way to keep an accepted limit from silently becoming an
        untested arm: this file has already shipped one green check that compared nothing.
        """
        env = env_file(box, "pages")
        d = os.path.join(box, "docs", "screenshots", "360")
        _png(box, "profiles.png")
        receipt(box, "pages", ["profiles"], head="b" * 40, at="2026-01-01T00:00:00Z")
        # The mint's own shape: the row's `sources_rev` IS the page's newest commit, and the frame's
        # pixels were last committed behind it. `7e65d3b`, one frame at a time.
        moved = _fake_git(rev="b" * 40, png_rev=RECORDED)
        record(d, run=moved, now="2026-01-01T00:00:00Z", envs=[env], shot=["profiles"])
        one = [_Page("/profiles", "profiles", fixture="pages")]
        doc = read_manifest(d)
        entry = doc["pages"][0]
        if entry.get("sources_rev") != "b" * 40 or not entry.get("photographed"):
            return f"the setup is not the mint: {entry!r}"
        covered = check(d, run=moved, pages=one)
        if covered:
            return (f"a frame the run's receipt covers was refused: {covered[:1]!r}. The whole point "
                    "of the claim is that a byte-identical re-take stays current")

        def with_claim(**overrides):
            pages = [dict(e) for e in doc["pages"]]
            claim = dict(pages[0].get("photographed"))
            claim.update(overrides)
            pages[0]["photographed"] = claim
            with open(os.path.join(d, MANIFEST), "w", encoding="utf-8") as fh:
                json.dump(dict(doc, pages=pages), fh, indent=2, sort_keys=True)
            return [p for p in check(d, run=moved, pages=one) if p.startswith("profiles.png")]

        # The residue, stated rather than hidden: with no receipt in the manifest -- which is every
        # row #338 lists -- the mint is a row `check` reads as current, because the only thing that
        # could say otherwise is the blob history that cannot see a re-take whose bytes did not
        # move. Refusing it is #338's dead end; not saying it is a green that means less than it
        # reads. So the suite pins that the answer here is silence, and `summary()` says the count.
        # Deleting `if recorded == current: continue` lands here as a refusal and goes red.
        pages = [dict(e) for e in doc["pages"]]
        pages[0].pop("photographed", None)
        with open(os.path.join(d, MANIFEST), "w", encoding="utf-8") as fh:
            json.dump(dict(doc, pages=pages), fh, indent=2, sort_keys=True)
        blind = [p for p in check(d, run=moved, pages=one) if p.startswith("profiles.png")]
        if blind:
            return (f"an unclaimed row dated at its page's own commit was refused: {blind[:1]!r}. "
                    "That is the check #302 measured as unsatisfiable -- its pixels cannot move "
                    "without a page change -- and `check`'s `recorded == current` arm is what holds "
                    "it open on purpose; the limit belongs in the summary, not in a refusal")
        # And the row that carries a claim contradicting its own date: the short-circuit used to
        # skip it before anything read the claim.
        lying = with_claim(head=RECORDED)
        if not any("was last photographed at commit" in p for p in lying):
            return (f"a row whose date says its page's commit while its receipt says the capture "
                    f"stood behind it passed: {lying[:1]!r}")
        return None

    rule("a row dated at its own page's commit is invisible to the blob history by decision, and a "
         "claim contradicting it is read before that arm can skip it",
         in_box(a_mint_is_invisible_to_the_history_and_visible_to_its_own_claim))

    def a_claimed_frame_whose_bytes_did_not_move_is_still_dated_at_the_page(box):
        """D-0028's case, with the date it always needed: a re-take that changed nothing.

        `settings.png` comes back byte-identical after its page moves, so `unchanged` is true and the
        carry rule is the one that decides its date. Without `--shot` that is right -- nobody looked
        at it. With `--shot` and a receipt, the frame WAS photographed and the carry would write a rev
        from before the run that took it, which is the mint's mirror image.
        """
        env = env_file(box, "pages")
        recorded(box, envs=[env])
        d = os.path.join(box, "docs", "screenshots", "360")
        moved = _fake_git(rev="b" * 40)
        receipt(box, "pages", ["profiles"], head="b" * 40, at="2026-01-02T00:00:00Z")
        record(d, run=moved, now="2026-01-02T00:00:00Z", envs=[env], shot=["profiles"])
        after = {e["png"]: e for e in read_manifest(d)["pages"]}
        if after["profiles.png"].get("sources_rev") != "b" * 40:
            return ("a byte-identical re-take its own receipt vouches for kept the rev from before "
                    "it was photographed, so `--shot` decides nothing for the frames whose pixels did "
                    "not move -- the ones D-0028 is about")
        if not after["profiles.png"].get("photographed"):
            return "the re-dated frame carries no evidence for the date it was given"
        for other in ("audit.png", "dashboard.png", "settings.png", "users.png"):
            if after[other].get("sources_rev") != RECORDED:
                return f"{other} was re-dated by a run that did not photograph it (#222)"
        return None

    rule("a claimed frame whose bytes came back identical is dated at the page it was re-taken from",
         in_box(a_claimed_frame_whose_bytes_did_not_move_is_still_dated_at_the_page))

    EXPECTED = 54
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
