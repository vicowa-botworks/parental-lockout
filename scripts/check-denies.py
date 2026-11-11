#!/usr/bin/env python3
"""check-denies.py -- that the factory agent's deny list still refuses what it says it refuses.

Why this is in `make verify` (#217 round four): the deny list in `.opencode/opencode.jsonc` is the
whole boundary, because `opencode run --auto` "auto-approve[s] permissions that are not explicitly
denied". Four rounds of that file were reviewed by reading it, and every round changed which
commands were refused without anyone being able to say which. At `869564a` seven of seventeen rules
were inert (patterns opening on a literal, which the whole-line glob could never reach); deleting
those literals at `37120a8` fixed the flag-inserted forms and re-opened the bare ones -- `rm -rf`,
`systemctl --user stop <the timer>`, `go get` went from denied to allowed in the commit that was
supposed to be the repair. Nothing in the repository could tell either of those apart from a green
tick.

What this checks, and what it does not. It reads the rules the CLI *resolves* for the agent --
`opencode debug agent <name>`, or `opencode debug agents` filtered, because v2.2.5 has only the
second -- and matches whole command strings against them the way the matcher described in H-0013
does: every rule in order, last match wins, `*` a glob. Two directions are asserted:

  * every denial in the config is refused in BOTH the bare form and the flag-inserted form, and
    every denial pattern is *witnessed* by at least one command here, so a pattern nothing probes
    is a failure rather than an unchecked line;
  * a short allow-list still runs, because an over-reaching deny list shuts the harness down and a
    check that only asserts denials passes a config that forbids `git commit`.

The honest limit, stated in the file whose behaviour is being modelled: this reads the resolved
ruleset, not a decision. `opencode run --auto --agent factory` still dies with `UnknownError` on
the reviewing box and has never executed a command inside these rules, so the matcher's semantics
remain an assumption this script shares with the probe that found the regression. What it removes is
the silent half: a pattern that stops matching is now a red step naming the command, without a live
tick and without a model.

Skipping is loud and is not a failure, on the same reasoning as `scripts/check-shell.sh`: opencode
is the binary ticks run, not something CI installs, and requiring it would make `make verify` fail
on a fresh clone, which teaches people to run `go test ./...` instead of the gate. CI therefore
prints one step that did not run. `CHECK_DENIES_REQUIRED=1` turns any non-run into a failure, and
`FACTORY_OPENCODE` names the binary -- the same variable `factory/common.sh` and the generated unit
use, so this checks the rules against the CLI a tick would actually meet.
"""

import fnmatch
import json
import os
import re
import subprocess
import sys

HERE = os.path.dirname(os.path.abspath(__file__))
ROOT = os.path.dirname(HERE)
CONFIG = os.path.join(ROOT, ".opencode", "opencode.jsonc")
MAKEFILE = os.path.join(ROOT, "Makefile")
AGENT = os.environ.get("FACTORY_AGENT", "factory")
BINARY = os.environ.get("FACTORY_OPENCODE", "opencode")
REQUIRED = os.environ.get("CHECK_DENIES_REQUIRED", "")
# The shell tool has two names across releases -- v2.2.5 resolves it as `shell`, 1.18.5 as `bash`, the
# same word the config writes -- so a reader keyed on one of them reads the other release's ruleset as
# empty. Both are accepted, and an unrecognised tool name is reported, never skipped.
TOOLS = ("shell", "bash")
# What this reader knows how to parse, quoted in the fault message so a third shape says so out loud.
_VERSION = None
SHAPES = ('{"action": <tool>, "resource": <glob>, "effect": <verdict>} and '
          '{"permission": <tool>, "pattern": <glob>, "action": <verdict>}')

# (command, expected effect, why this spelling is on the list). `deny` rows are the commands the
# comments in the config claim are refused; `allow` rows are the ones a deny that over-reached would
# take away. Both forms of every denial -- bare, where the denied program opens the line, and
# flag-inserted, where something precedes it -- because at both heads one of the two was missing.
COMMANDS = [
    ("rm -rf /tmp/plock-scratch", "deny", "the bare form, and the one a model types first"),
    ("rm -r /tmp/trees/x", "deny", "recursive without force still deletes a tree"),
    ("rm --recursive --force /tmp/trees/x", "deny", "the long-flag spelling, bare"),
    ("cd /tmp && rm -rf /home/hermes", "deny", "the form only a leading wildcard reaches"),
    ("cd /tmp && rm --recursive --force /tmp/trees/x", "deny", "long flags, inserted form"),
    ("systemctl --user restart parental-lockout-factory.timer", "deny",
     "a stopped supervisor is a bigger incident than a stalled ticket"),
    ("systemctl --user stop parental-lockout-factory.timer", "deny", "the same, another verb"),
    ("sudo systemctl stop docker", "deny", "the form only a leading wildcard reaches"),
    ("go get github.com/example/dep", "deny", "a require line is the user's decision, not a tick's"),
    ("GOTOOLCHAIN=go1.24 go get github.com/example/dep", "deny", "the env-prefixed form"),
    ("gh -R vicowa-botworks/parental-lockout pr merge 218", "deny",
     "the --repo form, which is what round three fixed"),
    ("gh pr merge 218 --admin", "deny", "the bare merge spelling"),
    ("gh pr review 217 --approve", "deny", "a tick cannot approve its own pull request"),
    ("git push --force origin feat", "deny", "rewrites history a reviewer has read"),
    ("git -C /trees/feat push --force origin feat", "deny", "the -C form this harness uses constantly"),
    ("git push -f origin feat", "deny", "the short flag"),
    ("git push --delete origin issue-199", "deny", "deletes someone else's branch"),
    ("git push origin main", "deny", "the protected branch"),
    ("git push origin master", "deny", "the other name for it"),
    ("gh api -X DELETE refs/tags/v1", "deny", "the raw call, denied whole rather than by method"),
    ("gh api", "deny", "the bare form of the same call"),
    ("gh release create v1.0.0", "deny", "publishing is a public act"),
    ("gh release delete v1.0.0", "deny", "un-publishing is worse"),
    ("gh release edit v1.0.0", "deny", "and editing one is neither of those but is still public"),
    ("git tag v9.9.9", "deny", "this repository publishes from a tag"),
    ("git worktree remove ../issue-199", "deny", "another session's fence, with work in it"),
    ("git worktree move ../issue-199 ../elsewhere", "deny", "the same fence, moved"),
    ("git describe --tags --always", "allow", "the packaging gate runs this exact command"),
    ("git tag", "allow", "with no argument it lists, and a tick has to be able to look"),
    ("gh release list", "allow", "reading a release is not publishing one"),
    ("git push origin feat", "allow", "how a tick's work survives the tick"),
    ("go build ./...", "allow", "the build"),
    ("go vet ./...", "allow", "the vet"),
    ("git commit -m 'Re-take the five frames the masthead moved'", "allow", "the ordinary act"),
]

DENY_KEY = re.compile(r'^[ \t]*"((?:[^"\\]|\\.)*)"[ \t]*:[ \t]*"deny"[ \t]*,?[ \t]*$', re.M)


def cli_version():
    """What `--version` calls the binary, or None. Asked once, and never fatal.

    The pass line and every skip line name it. Pattern semantics are the one thing about this check
    that plausibly moves between releases, and the reader has now been wrong about two of them, so a
    verdict without the version underneath it is the citation D-0015 refuses.
    """
    global _VERSION
    if _VERSION is None:
        try:
            proc = subprocess.run([BINARY, "--version"], capture_output=True, text=True,
                                  encoding="utf-8", errors="replace", timeout=30)
            _VERSION = (proc.stdout or proc.stderr).strip().splitlines()[0] if (
                proc.stdout or proc.stderr).strip() else None
        except (OSError, subprocess.SubprocessError, IndexError):
            _VERSION = None
    return _VERSION


def normalise_rule(entry):
    """One resolved rule from either release, as `{"tool", "pattern", "effect"}`, or None.

    Two vocabularies, one meaning. v2.2.5 answers
    `{"action": "shell", "resource": "rm -r*", "effect": "deny"}`; 1.18.5 answers
    `{"permission": "bash", "pattern": "rm -r*", "action": "deny"}`. The word `action` is the *tool* in
    one and the *verdict* in the other, which is why the branch is on which verdict key is present and
    not on the key's name: reading `action` as the tool on 1.18.5 yields a ruleset of zero bash rules,
    and reading `effect` as the verdict yields `None` for every one of them. Measured on the reviewer's
    box (1.18.5) against this tree: 23 config patterns each reported "does not appear in the resolved
    ruleset", 27 commands each reported ALLOW, and the `question` tool reported "no rule for it" -- 51
    problems about a config that was correct, from a reader that understood neither shape.
    """
    if not isinstance(entry, dict):
        return None
    if entry.get("effect") is not None:
        tool, effect = entry.get("action"), entry.get("effect")
    else:
        tool, effect = entry.get("permission"), entry.get("action")
    pattern = entry.get("resource")
    if pattern is None:
        pattern = entry.get("pattern", "*")
    if not isinstance(tool, str) or not isinstance(effect, str) or not isinstance(pattern, str):
        return None
    return {"tool": tool, "pattern": pattern, "effect": effect}


def rules_from(container):
    """The rule list inside an agent entry, whichever shape the entry holds it in.

    A list of rule objects is what both releases print today. The dict forms are what the config itself
    uses (`{"bash": {"rm -r*": "deny"}}`, or `{"question": "deny"}` for a whole tool) and cost four
    lines; a release that echoes the config back is a shape worth parsing rather than a surprise.
    """
    found = []
    if isinstance(container, list):
        found = [r for r in (normalise_rule(item) for item in container) if r]
    elif isinstance(container, dict):
        for tool, value in container.items():
            if isinstance(value, str):
                found.append({"tool": tool, "pattern": "*", "effect": value})
            elif isinstance(value, dict):
                for pattern, effect in value.items():
                    if isinstance(effect, str):
                        found.append({"tool": tool, "pattern": pattern, "effect": effect})
                    elif isinstance(effect, dict):
                        probe = normalise_rule(dict(effect, **{"resource": effect.get("resource", pattern)}))
                        if probe:
                            found.append(dict(probe, tool=probe["tool"] or tool))
            elif isinstance(value, list):
                found += [r for r in (normalise_rule(item) for item in value) if r]
    return found


def agent_entry(data):
    """The `factory` entry inside whatever `debug agent`/`debug agents` printed, or None."""
    for entry in data if isinstance(data, list) else [data]:
        if not isinstance(entry, dict):
            continue
        if entry.get("id") not in (None, AGENT) or entry.get("name") not in (None, AGENT):
            if entry.get("id") != AGENT and entry.get("name") != AGENT:
                continue
        for key in ("permissions", "permission"):
            if key in entry:
                return rules_from(entry[key])
    return None


def resolved_rules():
    """The agent's rules as the CLI resolves them, or None if the CLI would not say.

    `--standalone` first, on every verb, because it is the only form that resolves the config from
    disk: without it the CLI asks a running server, and a server answers from the config it loaded at
    its own start. Measured on 6 Nov, writing the pre-fix list and running the check in the same
    command still returned the fixed list -- 23 rules, all green -- because the desktop app's server
    was holding the file as it had been a minute earlier. A check that reads yesterday's ruleset is
    worse than no check, because it prints a number.

    `debug agent <name>` is what the reviewing box has; v2.2.5 answers only `debug agents`, which
    prints every agent and needs the filter, and it is not known whether `--standalone` exists on
    every release. Each combination is tried in order for the reason this whole file exists: a
    citation, like a pattern, has to resolve on somebody else's machine. If only the server-backed
    form answers, `main` compares the answer against this file and refuses to call it a pass.
    """
    attempts = []
    for extra in (["--standalone"], []):
        attempts.append([BINARY, "debug", "agent", AGENT] + extra)
        attempts.append([BINARY, "debug", "agents"] + extra)
    answered = False
    for argv in attempts:
        try:
            proc = subprocess.run(argv, capture_output=True, text=True, encoding="utf-8",
                                  errors="replace", timeout=90, cwd=ROOT)
        except (OSError, subprocess.SubprocessError):
            continue
        if proc.stdout.strip().startswith("{") or proc.stdout.strip().startswith("["):
            answered = True
        try:
            data = json.loads(proc.stdout)
        except (json.JSONDecodeError, ValueError):
            continue
        rules = agent_entry(data)
        if rules is not None:
            return rules, "--standalone" in argv, True
    return None, False, answered


def config_denials():
    """The deny patterns this repository writes down, read from the config rather than assumed.

    The table above is the claim; this is the thing being claimed about. Read from inside the `bash`
    object only: a tool's own rule (`"question": "deny"`) is a bare effect, not a command pattern,
    and it is asserted as a tool rule at the end of `main` instead. Scoping matters for the same
    reason the config scopes it -- a top-level pattern is read as an action, so a check that conflates
    the two would pass a config whose patterns are all inert.
    """
    with open(CONFIG, encoding="utf-8") as handle:
        text = handle.read()
    start = text.find('"bash"')
    if start < 0:
        return []
    brace = text.find("{", start)
    if brace < 0:
        return []
    depth, end = 0, len(text)
    for i in range(brace, len(text)):
        if text[i] == "{":
            depth += 1
        elif text[i] == "}":
            depth -= 1
            if depth == 0:
                end = i
                break
    return DENY_KEY.findall(text[brace:end])


def verdict(rules, tool, command):
    """Every rule in order, last match wins -- which is why `* allow` is written first in the config.

    Takes normalised rules (see `normalise_rule`) and accepts either name of the shell tool, because a
    verdict computed against the wrong tool name is a silent ALLOW for every command in the table.
    """
    effect = None
    for rule in rules:
        if rule["tool"] not in (tool, "*") and not (tool in TOOLS and rule["tool"] in TOOLS):
            continue
        if fnmatch.fnmatchcase(command, rule["pattern"]):
            effect = rule["effect"]
    return effect or "allow"


INVOCATION = re.compile(r"python3[ \t]+scripts/check-denies\.py(?![^\s])")
SELFTEST_INVOCATION = re.compile(r"python3[ \t]+scripts/check-denies\.py[ \t]+--self-test(?![^\s])")


def _status_reaches_shell(chunk, end):
    """Whether a non-zero exit from the invocation ending at `end` can end the recipe.

    Round seven, measured on `b5fd0ca`: `A --self-test && B;` under `set -e` prints nothing and exits
    0 when A fails, because errexit ignores every command of an AND-OR list but the last. So "the text
    is in the recipe" and "the recipe fails when this fails" are two different claims, and only the
    second one is worth a gate. Anything after the invocation in the same chunk -- `&&`, `||`, a pipe
    -- means something else decides the status; anything before it means the invocation is last, which
    is the position that counts.
    """
    tail, head = chunk[end:], chunk[:end]
    if "||" in tail or "&&" in tail or "|" in tail:
        return False
    if "|" in head:
        return False
    return True


def invocation_problems(recipe):
    """Given the text of a Makefile, say whether `verify` still runs this script -- and whether a
    failure of it could actually fail `verify`.

    Copied in shape from `check-decisions.py`'s guard of the same name, including the two things it
    learned the hard way: a bare filename substring is satisfied by a comment and by an `echo` that
    names this file while running something else, and a comment has to be tested on the *line* before
    splitting on `;`, because a commented step that continues with a backslash still puts the
    invocation in a later chunk. Asked and answered in round three of #217; the reason it is restated
    here rather than imported is that `check-decisions` is not an importable module name.

    This is asked before the CLI is resolved, so that a box without opencode -- where the rules
    themselves are skipped -- still cannot lose the step that would have caught it.
    """
    problems = []
    found = {"plain": False, "self-test": False}
    for raw in recipe.splitlines():
        if raw.lstrip().startswith("#"):
            continue
        for chunk in raw.split(";"):
            text = chunk.strip()
            if text.startswith("#"):
                continue
            kind = "self-test" if SELFTEST_INVOCATION.match(text) else None
            if kind is None and INVOCATION.match(text):
                kind = "plain"
            if kind is None:
                continue
            match = (SELFTEST_INVOCATION if kind == "self-test" else INVOCATION).match(text)
            if not _status_reaches_shell(text, match.end()):
                problems.append("Makefile: `%s` runs where its exit status cannot reach the shell -- a "
                                "command in an && or || list, or a pipeline, can fail and the recipe "
                                "still ends 0, so this step can never redden `verify`" % text)
                continue
            found[kind] = True
    if not found["plain"]:
        problems.append("Makefile: no recipe command runs `python3 scripts/check-denies.py`, so the "
                        "agent's deny list is not checked by the gate -- and a deny list nobody checks "
                        "is how seven inert rules and three re-opened commands survived four reviews")
    if not found["self-test"]:
        problems.append("Makefile: no recipe command runs `python3 scripts/check-denies.py --self-test` "
                        "in a position that can fail the gate, so CI -- where there is no CLI and every "
                        "live path skips -- checks nothing about the deny list")
    return problems


def config_tool_rule(tool):
    """The effect the config writes for a whole tool, or None if it writes no rule for it.

    `question` is denied as a tool, not as a command pattern, so it is invisible to every command row in
    this file -- and it is the denial the harness needs most, because a `question` in a headless run
    holds a worktree until the ceiling kills it with no reason recorded. Read from the file so the
    self-test can assert it on a box with no CLI at all.
    """
    with open(CONFIG, encoding="utf-8") as handle:
        text = handle.read()
    found = re.findall(r'"%s"[ \t]*:[ \t]*"(deny|allow|ask)"' % re.escape(tool), text)
    return found[-1] if found else None


def synthetic_rulesets():
    """The checked-in patterns as each release's resolved shape, so the table is testable without one.

    This is the part of the check CI can actually run: opencode is not installed in CI, so every live
    path here prints SKIPPED and the deny table -- the boundary every tick runs inside -- goes unexamined
    on the machine that does most of the gating. Patterns matched against commands is a property of the
    patterns, not of the CLI; keying (H-0013) still needs a CLI, and that stays a live-only claim.
    """
    denies = config_denials()
    shape_a = [{"action": "shell", "resource": p, "effect": "deny"} for p in denies]
    shape_b = [{"permission": "bash", "pattern": p, "action": "deny"} for p in denies]
    return denies, shape_a, shape_b


def table_problems(rules, label):
    """Every declared row of COMMANDS against `rules`, as one list of sentences."""
    out = []
    for command, want, why in COMMANDS:
        got = verdict(rules, "shell", command)
        if got != want:
            out.append("%s: %s resolves %s, wanted %s (%s)" % (label, command, got.upper(),
                                                               want.upper(), why))
    return out


def self_test():
    """The matcher, both releases' vocabularies, and the two ways this list has actually been wrong.

    Every case is a made-up ruleset, so the assertions hold on a box with no CLI and on a box whose CLI
    speaks a third shape. Each is written to fail if the code it protects is removed -- the same
    discipline `check-decisions.py --self-test` applies to itself, for the same reason: a check nobody
    can turn off is a check nobody can trust was ever on.
    """
    fails = []
    denies, shape_a, shape_b = synthetic_rulesets()

    # 1 and 2: the checked-in patterns, in both shapes, must give the declared verdict for every row.
    for label, raw in (("shape A (v2.2.5)", shape_a), ("shape B (1.18.5)", shape_b)):
        parsed = rules_from(raw)
        if len(parsed) != len(raw):
            fails.append("%s: %d of %d rules could not be normalised" % (label, len(raw) - len(parsed),
                                                                        len(raw)))
        fails += table_problems(parsed, label)

    # 3: the `37120a8` regression, in one line -- a table whose denies all open with `* ` re-allowed
    # `rm -rf`, `systemctl` and `go get` in the commit that was sent to fix something else.
    wildcard_only = [r for r in shape_a if r["resource"].startswith("* ")]
    missed = [c for c, want, _ in COMMANDS
              if want == "deny" and not any(fnmatch.fnmatchcase(c, r["resource"]) for r in wildcard_only)]
    if "rm -rf /tmp/plock-scratch" not in missed:
        fails.append("wildcard-only: dropping every literal-first pattern still refused the bare "
                     "`rm -rf`, so this table cannot see the regression that happened")

    # 4: the `869564a` half -- literal-first only, which is what seven inert rules looked like.
    literal_only = [r for r in shape_a if not r["resource"].startswith("* ")]
    if any(fnmatch.fnmatchcase("gh -R vicowa-botworks/parental-lockout pr merge 218", r["resource"])
           for r in literal_only):
        fails.append("literal-only: a first-pattern-literal rule reached the `--repo` form, so this "
                     "case no longer describes the bug it was written for")

    # 5: a denial widened past its subcommand must be caught by an allow row, not celebrated as extra
    # coverage. The shipped rules are `* tag *` and `git tag *`, and the spaces are load-bearing: drop
    # them and the same rule swallows `git describe --tags --always`, which the packaging gate runs, and
    # `git tag` with no argument, which is how a tick reads the version it just built. Both halves are
    # asserted, because a check that only tests the widened form proves nothing about the shipped one.
    widened = shape_a + [{"action": "shell", "resource": "*tag*", "effect": "deny"}]
    swallowed = [c for c, want, _ in COMMANDS if want == "allow" and fnmatch.fnmatchcase(c, "*tag*")]
    if not swallowed:
        fails.append("widened-tag: no allow row is matched by `*tag*`, so widening the tag rule would "
                     "not be noticed by the allow-list")
    if verdict(rules_from(widened), "shell", "git describe --tags --always") != "deny":
        fails.append("widened-tag: `*tag*` did not refuse `git describe --tags --always`, so the "
                     "allow-list is not the thing keeping that command alive")
    for probe in ("git describe --tags --always", "git tag", "gh release list"):
        if verdict(rules_from(shape_a), "shell", probe) != "allow":
            fails.append("shaped-tag: the shipped tag rules refuse %r, so a legitimate read is denied "
                         "and the allow-list did not catch it because the row is not in this file" % probe)

    # 6: a reader that parses nothing is a reader fault. Asserted here as the parse, because the message
    # is asserted by the arm in `main` and the arm cannot run without a CLI to answer in the bad shape.
    junk = agent_entry([{"name": "factory",
                         "permission": [{"tool": "exec", "glob": "rm *", "verdict": "block"}]}])
    if junk:
        fails.append("reader-fault: a third shape parsed as %d rule(s), so an unreadable CLI is "
                     "reporting a verdict on the config again" % len(junk))
    if agent_entry([{"name": "factory", "permissions": []}]) is None:
        fails.append("reader-fault: an empty ruleset answered None, which is the same answer as no CLI "
                     "at all and loses the distinction between the two")
    # The dict form the config itself uses has to parse, or a release that echoes the config back reads
    # as a fault.
    if not rules_from({"bash": {"rm -r*": "deny"}, "question": "deny"}):
        fails.append("dict form: a config-shaped `permission` object parsed as no rules")

    # 7: the config's one tool-level denial.
    if config_tool_rule("question") != "deny":
        fails.append("the config does not deny the `question` tool (it reads %r), so a tick can stall "
                     "on an answer nobody will give" % config_tool_rule("question"))

    # 8: the Makefile guard, both ways -- a commented step with an intact continuation is the shape that
    # passed round three.
    real = open(MAKEFILE, encoding="utf-8").read()
    if invocation_problems(real):
        fails.append("verify-invocation: this repository's own Makefile no longer satisfies the guard "
                     "that checks it does -- %s" % invocation_problems(real))
    commented = "\n".join(("#\t" + line if "scripts/check-denies.py" in line and not
                            line.lstrip().startswith("#") else line) for line in real.splitlines())
    if not invocation_problems(commented):
        fails.append("verify-invocation: commenting out the step left the guard green, so the step that "
                     "catches this step being deleted is not real")
    # Deleting the self-test invocation has to redden the guard. The old assertion asked whether the
    # string `--self-test` appeared anywhere in the Makefile -- which it always does, because `verify`
    # runs `python3 scripts/check-decisions.py --self-test` two lines above -- so the guard could not
    # fail this way however badly the step was broken.
    dropped = "\n".join(
        (line.replace("check-denies.py --self-test;", "check-denies.py;")
             .replace("check-denies.py --self-test", "check-denies.py")
         if "scripts/check-denies.py --self-test" in line and not line.lstrip().startswith("#") else line)
        for line in real.splitlines())
    if "scripts/check-denies.py --self-test" in dropped:
        fails.append("verify-invocation: the --self-test mutation case found nothing to mutate, so it "
                     "asserts nothing -- the Makefile no longer has the invocation shape it documents")
    elif not invocation_problems(dropped):
        fails.append("verify-invocation: deleting the --self-test invocation left the guard green, so "
                     "nothing catches CI losing the only deny-table check it can ever run")

    # And so has putting it back the way it was: `A --self-test && B`, the shape that passed four
    # reviews while a red self-test printed nothing and `verify` said OK.
    chained = real.replace("check-denies.py --self-test; python3 scripts/check-denies.py",
                           "check-denies.py --self-test && python3 scripts/check-denies.py")
    if chained == real:
        fails.append("verify-invocation: the && mutation case found nothing to mutate, so it asserts "
                     "nothing -- the two invocations are no longer adjacent in `verify`")
    elif not invocation_problems(chained):
        fails.append("verify-invocation: joining the two invocations with && left the guard green -- a "
                     "command other than the last in an AND-OR list is invisible to `set -e`, so a red "
                     "self-test hides the live run, every gate behind it, and the verify: OK")

    # 9: the list must not be silently shrunk to the point of meaninglessness.
    if len(denies) < 10 or not any(want == "allow" for _, want, _ in COMMANDS):
        fails.append("the tables are too small to assert anything: %d config patterns, %d allow rows"
                     % (len(denies), sum(1 for _, w, _ in COMMANDS if w == "allow")))

    if fails:
        print("check-denies --self-test FAILED:")
        for f in fails:
            print("  " + f)
        return 1
    print("check-denies --self-test: %d patterns resolved in both releases' shapes, %d commands refused "
          "or allowed as declared, both one-shaped regressions refused, one widened pattern refused, the "
          "unreadable-CLI case reported as a reader fault, and this step's own Makefile invocation"
          % (len(denies), len(COMMANDS)))
    return 0


def main():
    if "--self-test" in sys.argv[1:]:
        return self_test()
    problems = []
    try:
        with open(MAKEFILE, encoding="utf-8") as handle:
            problems += invocation_problems(handle.read())
    except OSError as exc:
        problems.append("Makefile: cannot be read (%s), so nothing can be claimed about the gate" % exc)
    if problems:
        print("check-denies: %d problem(s):" % len(problems), file=sys.stderr)
        for problem in problems:
            print("  " + problem, file=sys.stderr)
        return 1

    rules, standalone, answered = resolved_rules()
    if rules is None:
        # The binary is absent (CI, a fresh clone) or answers neither debug verb. Say which step did
        # not run rather than printing a tick with a smaller number under it.
        why = "did not resolve the %r agent, so its debug surface changed" % AGENT
        try:
            subprocess.run([BINARY, "--version"], capture_output=True, text=True,
                           encoding="utf-8", errors="replace", timeout=30)
        except (OSError, subprocess.SubprocessError):
            why = "is not on PATH"
        line = ("check-denies: SKIPPED -- %s (%s) %s. The deny list was not checked; it is the "
                "boundary every tick runs inside, and nothing else here can see it. "
                "CHECK_DENIES_REQUIRED=1 makes a skipped run a failure."
                % (BINARY, cli_version() or "version unknown", why))
        print(line, file=sys.stderr if REQUIRED else sys.stdout)
        return 1 if REQUIRED else 0

    denies = config_denials()
    shell_rules = [r for r in rules if r["tool"] in TOOLS and r["effect"] == "deny"]

    # A reader that parsed nothing is a reader fault, and it must never be reported as the config's
    # verdict. `if shell_rules and not denies` below cannot cover this case: it needs a non-empty
    # ruleset, so on a box whose CLI answers in a shape this file does not know, every config pattern
    # becomes an accusation ("does not appear in the resolved ruleset"), every command becomes ALLOW,
    # and the `question` tool becomes "no rule for it" -- 51 problems naming a correct file, in the
    # imperative voice of the rule that exists to catch mis-keying. That is the mirror image of the
    # `[ \t]` bug this script caught on its own first run, and it points at the wrong artefact. It is
    # also not free: `make -n verify` starts `set -e;` and this step is ninth, so a false verdict here
    # silences every gate behind it. Loud skip, like a missing binary; failure only when required.
    if not shell_rules:
        line = ("check-denies: SKIPPED (reader fault) -- %s (%s) answered for agent %r but %d of its "
                "%d rule(s) were a shell/bash denial this reader could parse (%s). %s lists %d deny "
                "pattern(s), so the rules exist and this file is the thing that cannot read them. This "
                "is not a verdict on the config. CHECK_DENIES_REQUIRED=1 makes it a failure."
                % (BINARY, cli_version() or "version unknown", AGENT, len(shell_rules), len(rules),
                   SHAPES, os.path.relpath(CONFIG, ROOT), len(denies)))
        print(line, file=sys.stderr if REQUIRED else sys.stdout)
        return 1 if REQUIRED else 0

    # The ruleset has to be the one this file writes. In one direction that is H-0013's mis-keying
    # (a pattern the matcher never sees); in the other it is a stale answer, which is what happens
    # without --standalone when a server is running: it reports the config it loaded at startup, and
    # every row below then passes against rules that are not in the tree.
    resolved = {r["pattern"] for r in shell_rules}
    if not denies:
        # The other direction of the same fault: the CLI answered with rules, and the config reader
        # matched none of them, so every assertion below would be vacuously true. This is what the
        # first run of this script did to itself -- a character class typed as `[ \t]` instead of
        # `[ \t]*` matched no pattern at all, and 0 patterns meant 0 problems meant a green gate.
        problems.append("%s yielded no deny patterns to this reader while %s (%s) resolves %d, so the "
                        "reader is broken, not the config"
                        % (os.path.relpath(CONFIG, ROOT), BINARY, cli_version() or "?",
                           len(shell_rules)))
    for pattern in denies:
        if pattern not in resolved:
            problems.append('config pattern "%s" does not appear in the resolved ruleset, so the '
                            'matcher never sees it (H-0013: a pattern outside the `bash` object is '
                            'read as an action, and no tool reports an action named after a command)'
                            % pattern)
    for pattern in sorted(resolved - set(denies)):
        problems.append('the resolved ruleset denies "%s", which %s does not contain -- the answer '
                        'came from something other than this file (a server that has not reloaded it), '
                        'so nothing here was actually checked%s'
                        % (pattern, os.path.relpath(CONFIG, ROOT),
                           "" if standalone else "; %s answered without --standalone" % BINARY))

    for command, want, why in COMMANDS:
        got = verdict(rules, "shell", command)
        if got != want:
            problems.append("%s resolves %s, wanted %s  (%s)\n      %s"
                            % (command, got.upper(), want.upper(), why,
                               "the config's deny patterns are the boundary; --auto allows the rest"))

    # A denial nothing probes is as blind as a pattern nothing evaluates.
    for pattern in denies:
        if not any(fnmatch.fnmatchcase(c, pattern) for c, want, _ in COMMANDS if want == "deny"):
            problems.append('config pattern "%s" is witnessed by no command in this file, so whether '
                            'it refuses anything is unknown' % pattern)

    if not any(want == "allow" for _, want, _ in COMMANDS):
        problems.append("the allow-list is empty, so an over-reaching deny list would pass here")

    # The one denial that is a tool rule rather than a command pattern. It is the denial the harness
    # depends on most -- a `question` in a headless run holds a worktree until the ceiling kills it,
    # with no reason recorded -- and it is invisible to everything above, because it never matches a
    # command string.
    question = [r for r in rules if r["tool"] == "question"]
    if not any(r.get("effect") == "deny" for r in question):
        problems.append('the `question` tool is not denied (%s), so a tick can still stall on an '
                        'answer nobody will give' % (question or "no rule for it"))

    if problems:
        print("check-denies: %d problem(s) against the ruleset %s (%s) resolves for agent %r "
              "(%d shell deny rules, %d commands, %d config patterns):"
              % (len(problems), BINARY, cli_version() or "version unknown", AGENT, len(shell_rules),
                 len(COMMANDS), len(denies)), file=sys.stderr)
        for p in problems:
            print("  " + p, file=sys.stderr)
        return 1
    # Binary and version on the line that says it passed. `23 deny rules resolved` is green on one box
    # and 51 problems on another; this push's own argument is that a citation without the binary and the
    # version is a claim nobody else can re-run, and a pass line is a citation of the thing it read.
    print("check-denies: %d deny rules resolved by %s (%s)%s for agent %r, %d commands refused or "
          "allowed as declared, every pattern witnessed"
          % (len(shell_rules), BINARY, cli_version() or "version unknown",
             " from disk (--standalone)" if standalone else " WITHOUT --standalone (a server may be "
             "answering from the config it loaded at startup)", AGENT, len(COMMANDS)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
