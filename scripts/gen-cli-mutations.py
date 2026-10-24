#!/usr/bin/env python3
"""Build scripts/mutations/cli.json for internal/cli, a package that had no set at all.

Every `old` is taken by locating ONE line whose text contains a distinctive fragment and
using that whole line, so nothing here is transcribed by hand -- a hand-typed pattern is
how a mutant ends up NOT APPLIED. The transform asserts it actually changed the line.
"""
import json, sys

SPECS = [
    # ---- args.go: TakeFirst, the guard that fixed a data-loss bug (memory 1054).
    ("CI1", "internal/cli/args.go", 'if a == "--" {',
     lambda l: l.replace('if a == "--" {', 'if false {'),
     'the "--" separator stops being special, so it falls into the flag branch and swallows '
     'the token after it: the documented `unlock -- -x` can no longer name an account called '
     '-x, which is the only reason the separator exists'),

    ("CI2", "internal/cli/args.go", "if !found && len(after) > 0 {",
     lambda l: l.replace("if !found && len(after) > 0 {", "if !found {"),
     "the length check on `after` is dropped, so `TakeFirst` indexes after[0] when the "
     "separator is the last token -- `plock unlock --` panics instead of reporting that no "
     "account was named. Deliberately NOT cmd.json's TS3, which mutates the SAME "
     "line by dropping `!found` instead: that one overwrites a positional already taken "
     "(`unlock casey -- anna` acts on anna) and already runs ./internal/cli/ as well as "
     "./cmd/plock/, so re-deriving it here would be one claim counted twice. This is the "
     "other half of the condition, and only this package's suite sees it crash."),

    ("CI3", "internal/cli/args.go", "return after[0], append(rest, after[1:]...), true",
     lambda l: l.replace(", true", ", false"),
     'the adopted token is returned with found=false -- the exact bug the comment above it '
     'records: every caller reads a missing positional plus a non-empty fs.Args() as the '
     'user making a mistake, so all 17 call sites refuse a valid line at once'),

    ("CI4", "internal/cli/args.go", '\t\t\trest = append(rest, a)\n\t\t\tif !found && len(after) > 0 {',
     lambda l: l.replace("\t\t\trest = append(rest, a)\n", "\t\t\t_ = a\n"),
     'the separator is dropped from the remainder instead of copied into it, so flag.Parse '
     'sees no separator and `unlock -- casey --minutes 5` adopts casey and then parses '
     '--minutes as a flag -- the opposite of what a separator promises '
     '(anchored on the following line because `rest = append(rest, a)` also appears in the '
     'positional branch below, at the same indent)'),

    ("CI5", "internal/cli/args.go", 'if !strings.HasPrefix(a, "-") || a == "-" {',
     lambda l: l.replace(' || a == "-"', ''),
     'a lone "-" stops counting as a positional and is treated as a flag instead'),

    ("CI6", "internal/cli/args.go", "\t\t\tif !found {",
     lambda l: l.replace("if !found {", "if true {"),
     'the first positional rule becomes "last one wins", so a second bare token silently '
     'replaces the account the operator named'),

    ("CI7", "internal/cli/args.go", "if i+1 < len(args) && flagWantsValue(fs, a, name) {",
     lambda l: l.replace("i+1 < len(args) && ", ""),
     'the bounds check goes, so a trailing flag with no value reads past the end of args -- '
     'a panic in the argument parser of a command the operator runs by hand'),

    ("CI8", "internal/cli/args.go", 'name := strings.TrimLeft(a, "-")',
     lambda l: l.replace('strings.TrimLeft(a, "-")', "a"),
     'the lookup is done with the dashes still on, so every known flag looks unknown and '
     'inherits the unknown-flag rule -- a boolean flag then eats the following positional'),

    ("CI9", "internal/cli/args.go", 'if strings.Contains(arg, "=") {',
     lambda l: l.replace('strings.Contains(arg, "=")', "false"),
     '--minutes=5 is no longer recognised as carrying its own value, so it consumes the '
     'next token as a value it already has and the positional after it disappears'),

    ("CI10", "internal/cli/args.go", "\t\treturn true",
     lambda l: l.replace("return true", "return false"),
     'an unknown flag is assumed valueless instead, which is the direction the comment '
     'refuses on purpose: a typo\'d flag\'s value becomes the username, turning a clear '
     'complaint about the flag into a confident wrong answer about a child\'s account'),

    ("CI11", "internal/cli/args.go", "_, isBool := f.Value.(boolFlag)",
     lambda l: l.replace("f.Value.(boolFlag)", "any(nil).(boolFlag)"),
     'the bool-flag detector never fires, so every flag looks like it takes a value and '
     '`--verbose casey` loses casey to the flag -- flag itself never consumes a value after '
     'a bool. Mutating the assertion rather than `return !isBool` is deliberate: replacing '
     'the return orphans isBool, which is a build failure and not a catch'),

    # ---- cli.go: the shared output contract.
    ("CI12", "internal/cli/cli.go", "\tif asJSON {",
     lambda l: l.replace("if asJSON {", "if false {"),
     '--json failures print as a human line, so a script reading the failure parses nothing; '
     'this is the one implementation of the contract every command routes through'),

    ("CI13", "internal/cli/cli.go", "\t\treturn Failure",
     lambda l: l.replace("return Failure", "return 0"),
     'a --json failure exits 0 while stdout is empty, so `plock ... --json` looks like a '
     'successful run to anything that checks the exit status'),

    ("CI14", "internal/cli/cli.go", "}{Error: code, Message: err.Error()})",
     lambda l: l.replace("{Error: code,", "{Error: \"\","),
     'the machine-readable error code is dropped from the failure document, leaving only '
     'prose to branch on'),

    ("CI15", "internal/cli/cli.go", "enc.SetEscapeHTML(false)",
     lambda l: l.replace("false)", "true)"),
     'HTML escaping is turned back on, so a value containing < or > round-trips as \\u003c '
     'and the machine-readable format stops being the identity on its own bytes'),

    ("CI16", "internal/cli/cli.go", "if pad < len(label) {",
     lambda l: l.replace("pad < len(label)", "pad > len(label)"),
     'the alignment column is computed the wrong way round, so labels are not aligned to the '
     'widest one seen so far'),

    ("CI17", "internal/cli/cli.go", "if r.b.Len() > 0 {",
     lambda l: l.replace("if r.b.Len() > 0 {", "if true {"),
     'the blank line before a section is written unconditionally, so a report that starts '
     'with a section starts with a stray blank line'),

    ("CI18", "internal/cli/cli.go", 'if !strings.HasSuffix(format, "\\n") {',
     lambda l: l.replace('!strings.HasSuffix(format, "\\n")', "true"),
     'a line that already ends in a newline gets a second one, so every table in a report '
     'grows a blank row'),

    ("CI19", "internal/cli/cli.go", "\tif d < 0 {",
     lambda l: l.replace("if d < 0 {", "if false {"),
     'a negative remaining time is no longer clamped, so a lockout that has already ended '
     'renders as a negative wait'),

    ("CI20", "internal/cli/cli.go", "\tsort.Strings(out)",
     lambda l: l.replace("sort.Strings(out)", "sort.Strings(nil)"),
     'Keys stops sorting, and because Go randomises map iteration a machine-readable field '
     'built from a map changes between two runs of the same command on the same file'),
]

OUT = "scripts/mutations/cli.json"
mutants, problems = [], []
cache = {}
for mid, f, frag, fn, note in SPECS:
    if f not in cache:
        cache[f] = open(f).read()
    raw = cache[f]
    if "\n" in frag:
        # A multi-line anchor: the fragment IS the old text, matched against the raw file.
        if raw.count(frag) != 1:
            problems.append(f"{mid}: block matched {raw.count(frag)} times in {f}")
            continue
        old = frag
    else:
        lines = raw.split("\n")
        hits = [i for i, line in enumerate(lines) if frag in line]
        if len(hits) != 1:
            problems.append(f"{mid}: fragment {frag!r} matched {len(hits)} lines in {f}")
            continue
        old = lines[hits[0]]
    new = fn(old)
    if new == old:
        problems.append(f"{mid}: transform did not change the line")
        continue
    mutants.append({"id": mid, "file": f, "old": old, "new": new, "note": note})

if problems:
    print("REFUSING to write -- unresolved patterns:")
    for p in problems:
        print("  " + p)
    sys.exit(1)

doc = {
    "description": "internal/cli: the shared CLI plumbing. The package had no set at all until "
                   "#76's CI job made an untested mutant visible as a class of problem; TakeFirst "
                   "is here because it is what makes `plock unlock <user> --minutes N` parse at all "
                   "(the flag-order data-loss bug), and Fail/JSON/Keys are here because they are the "
                   "single implementation of the --json contract every command routes through.",
    "package": "./internal/cli/",
    "mutants": mutants,
}
# Both keywords are load-bearing, and #148 is the hunk that proves it. `ensure_ascii` defaults to
# True, so regenerating this set re-serialises every em dash and arrow in a note as `\u2014` /
# `\u2192` and mints a diff hunk on a mutant nobody changed -- which is what #133 carried into
# review on a line of `enforce.json` its own PR did not touch. `encoding` defaults to the
# operator's locale, so two machines can write different bytes from the same generator; that
# failure is silent in the opposite direction, which is worse. A committed set file is UTF-8 by
# declaration, not by whoever happened to run the generator last.
with open(OUT, "w", encoding="utf-8") as fh:
    json.dump(doc, fh, indent=2, ensure_ascii=False)
    fh.write("\n")
print(f"wrote {OUT} with {len(mutants)} mutants, all patterns verified against the source")
