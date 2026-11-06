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
TOOL = "shell"

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
    for argv in attempts:
        try:
            proc = subprocess.run(argv, capture_output=True, text=True, encoding="utf-8",
                                  errors="replace", timeout=90, cwd=ROOT)
        except (OSError, subprocess.SubprocessError):
            continue
        try:
            data = json.loads(proc.stdout)
        except (json.JSONDecodeError, ValueError):
            continue
        for entry in data if isinstance(data, list) else [data]:
            if not isinstance(entry, dict):
                continue
            if entry.get("id") == AGENT or entry.get("name") == AGENT:
                return entry.get("permissions") or [], "--standalone" in argv
    return None, False


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
    """Every rule in order, last match wins -- which is why `* allow` is written first in the config."""
    effect = None
    for rule in rules:
        if rule.get("action") not in (tool, "*"):
            continue
        if fnmatch.fnmatchcase(command, rule.get("resource", "")):
            effect = rule.get("effect")
    return effect or "allow"


def invocation_problems(recipe):
    """Given the text of a Makefile, say whether `verify` still runs this script.

    Copied in shape from `check-decisions.py`'s guard of the same name, including the two things it
    learned the hard way: a bare filename substring is satisfied by a comment and by an `echo` that
    names this file while running something else, and a comment has to be tested on the *line* before
    splitting on `;`, because a commented step that continues with a backslash still puts the
    invocation in a later chunk. Asked and answered in round three of #217; the reason it is restated
    here rather than imported is that `check-decisions` is not an importable module name.

    This is asked before the CLI is resolved, so that a box without opencode -- where the rules
    themselves are skipped -- still cannot lose the step that would have caught it.
    """
    for raw in recipe.splitlines():
        if raw.lstrip().startswith("#"):
            continue
        for chunk in raw.split(";"):
            if chunk.strip().startswith("#"):
                continue
            if re.match(r"python3[ \t]+scripts/check-denies\.py(?![^\s])", chunk.strip()):
                return []
    return ["Makefile: no recipe command runs `python3 scripts/check-denies.py`, so the agent's deny "
            "list is not checked by the gate -- and a deny list nobody checks is how seven inert "
            "rules and three re-opened commands survived four reviews"]


def main():
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

    rules, standalone = resolved_rules()
    if rules is None:
        # The binary is absent (CI, a fresh clone) or answers neither debug verb. Say which step did
        # not run rather than printing a tick with a smaller number under it.
        why = "did not resolve the %r agent, so its debug surface changed" % AGENT
        try:
            subprocess.run([BINARY, "--version"], capture_output=True, text=True, timeout=30)
        except (OSError, subprocess.SubprocessError):
            why = "is not on PATH"
        line = ("check-denies: SKIPPED -- %s %s. The deny list was not checked; it is the boundary "
                "every tick runs inside, and nothing else here can see it. CHECK_DENIES_REQUIRED=1 "
                "makes a skipped run a failure." % (BINARY, why))
        print(line, file=sys.stderr if REQUIRED else sys.stdout)
        return 1 if REQUIRED else 0

    denies = config_denials()
    shell_rules = [r for r in rules if r.get("action") == TOOL and r.get("effect") == "deny"]

    # The ruleset has to be the one this file writes. In one direction that is H-0013's mis-keying
    # (a pattern the matcher never sees); in the other it is a stale answer, which is what happens
    # without --standalone when a server is running: it reports the config it loaded at startup, and
    # every row below then passes against rules that are not in the tree.
    resolved = {r.get("resource") for r in shell_rules}
    if shell_rules and not denies:
        # Without this line the two loops below still fail loudly, but as N messages about N
        # patterns, which reads as a broken config rather than as a reader that matched nothing --
        # the exact failure this script was written to catch, and the one it made on its own first
        # run: a character class typed as `[ \t]` instead of `[ \t]*` matched no pattern at all, and
        # every config-side assertion became vacuously true.
        problems.append("%s yielded no deny patterns to this reader while %s resolves %d, so the "
                        "reader is broken, not the config"
                        % (os.path.relpath(CONFIG, ROOT), BINARY, len(shell_rules)))
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
        got = verdict(rules, TOOL, command)
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
    question = [r for r in rules if r.get("action") == "question"]
    if not any(r.get("effect") == "deny" for r in question):
        problems.append('the `question` tool is not denied (%s), so a tick can still stall on an '
                        'answer nobody will give' % (question or "no rule for it"))

    if problems:
        print("check-denies: %d problem(s) against the ruleset %s resolves for agent %r "
              "(%d shell deny rules, %d commands, %d config patterns):"
              % (len(problems), BINARY, AGENT, len(shell_rules), len(COMMANDS), len(denies)),
              file=sys.stderr)
        for p in problems:
            print("  " + p, file=sys.stderr)
        return 1
    print("check-denies: %d deny rules resolved%s for agent %r, %d commands refused or allowed as "
          "declared, every pattern witnessed"
          % (len(shell_rules), " from disk (--standalone)" if standalone else " WITHOUT --standalone",
             AGENT, len(COMMANDS)))
    return 0


if __name__ == "__main__":
    sys.exit(main())
