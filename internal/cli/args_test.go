package cli

import (
	"flag"
	"strings"
	"testing"
)

// newSet is the unlock command's flag shape: string flags, an int, a bool. The tests are
// written against a realistic set because the behaviour under test is exactly "is this
// token a flag's value or a positional", and that depends on the declared types.
func newSet() *flag.FlagSet {
	fs := flag.NewFlagSet("unlock", flag.ContinueOnError)
	fs.SetOutput(discard{})
	_ = fs.String("reason", "", "")
	_ = fs.String("by", "", "")
	_ = fs.String("config", "", "")
	_ = fs.Int("minutes", 0, "")
	_ = fs.Bool("json", false, "")
	return fs
}

func TestTakeFirstPullsTheUsernameOutOfTheDocumentedShape(t *testing.T) {
	// `plock unlock casey --minutes 30 --reason x` is the shape the manual shows and the
	// shape every human copies. Go's flag package stops parsing at "casey", so without
	// this the command sees five positionals and reports that --minutes is missing.
	fs := newSet()
	first, rest, found := TakeFirst(fs, []string{"casey", "--minutes", "30", "--reason", "film night"})
	if !found || first != "casey" {
		t.Fatalf("first = %q found=%v, want casey/true", first, found)
	}
	if err := fs.Parse(rest); err != nil {
		t.Fatalf("the remaining arguments do not parse: %v (%v)", err, rest)
	}
	if got := fs.Lookup("minutes").Value.String(); got != "30" {
		t.Errorf("--minutes = %s after parsing %v, want 30", got, rest)
	}
	if got := fs.Lookup("reason").Value.String(); got != "film night" {
		t.Errorf("--reason = %q, want the whole quoted value", got)
	}
	if fs.NArg() != 0 {
		t.Errorf("%d arguments still positional after parsing: %v", fs.NArg(), fs.Args())
	}
}

func TestTakeFirstDoesNotEatAFlagValueThatLooksLikeAName(t *testing.T) {
	// `--by casey` has a value that is exactly what a username looks like. Taking the
	// first non-flag token would hand back "casey" from the wrong place and leave the real
	// username behind as an extra.
	fs := newSet()
	first, rest, _ := TakeFirst(fs, []string{"--by", "casey", "alex", "--minutes", "5"})
	if first != "alex" {
		t.Fatalf("first = %q, want alex: --by's value is not the positional", first)
	}
	if err := fs.Parse(rest); err != nil {
		t.Fatalf("%v (%v)", err, rest)
	}
	// Both facts have to hold at once: "alex" came out, and "casey" is still --by's.
	if got := fs.Lookup("by").Value.String(); got != "casey" {
		t.Errorf("--by = %q, want casey", got)
	}
}

func TestTakeFirstLeavesAnUnquotedReasonTailForFlagToComplainAbout(t *testing.T) {
	// `--reason film night` with the space unquoted arrives as three arguments. "night"
	// looks exactly like a positional, and taking it as one would silently record "film"
	// as the reason -- a different reason than the one typed, with no complaint.
	fs := newSet()
	first, rest, _ := TakeFirst(fs, []string{"casey", "--reason", "film", "night"})
	if first != "casey" {
		t.Fatalf("first = %q, want casey", first)
	}
	if err := fs.Parse(rest); err == nil && fs.NArg() == 0 {
		t.Errorf("the stray word disappeared: rest=%v args=%v -- a word of the reason was dropped", rest, fs.Args())
	}
}

func TestTakeFirstRespectsTheDoubleDashSeparator(t *testing.T) {
	// A user literally named "-x" is reachable, and `unlock --minutes 5 -- -h` must not
	// turn into help.
	fs := newSet()
	first, rest, found := TakeFirst(fs, []string{"--minutes", "5", "--", "-h"})
	if found {
		t.Errorf("first = %q, want nothing: after -- every token is a value", first)
	}
	if err := fs.Parse(rest); err != nil {
		t.Fatalf("%v (%v)", err, rest)
	}
	if fs.NArg() != 1 || fs.Arg(0) != "-h" {
		t.Errorf("positionals after parsing = %v, want [-h]", fs.Args())
	}

	fs2 := newSet()
	first, rest, _ = TakeFirst(fs2, []string{"--", "-x", "--minutes", "5"})
	if err := fs2.Parse(rest); err != nil {
		t.Fatalf("%v", err)
	}
	if first != "" || fs2.NArg() != 3 {
		t.Errorf("first=%q args=%v, want everything after -- kept as positionals", first, fs2.Args())
	}
}

func TestTakeFirstAcceptsEqualsFormAndBoolFlags(t *testing.T) {
	// --minutes=5 carries its value already, and a bool never consumes the next token, so
	// the token after either one is a genuine positional.
	fs := newSet()
	first, rest, _ := TakeFirst(fs, []string{"--minutes=5", "--json", "casey"})
	if first != "casey" {
		t.Fatalf("first = %q, want casey", first)
	}
	if err := fs.Parse(rest); err != nil {
		t.Fatalf("%v (%v)", err, rest)
	}
	if fs.Lookup("json").Value.String() != "true" {
		t.Error("--json was not parsed")
	}
	if fs.Lookup("minutes").Value.String() != "5" {
		t.Error("--minutes=5 was not parsed")
	}
}

func TestTakeFirstAssumesAnUnknownFlagTakesAValue(t *testing.T) {
	// An unknown flag has unknowable arity. Assuming it takes a value means a typo'd
	// flag's value is never mistaken for the username: the worst outcome is that flag
	// refuses the line, which it was going to do anyway.
	fs := newSet()
	first, rest, _ := TakeFirst(fs, []string{"--minuts", "5", "casey"})
	if first != "casey" {
		t.Errorf("first = %q, want casey: an unknown flag must not hand its value back as the account to unlock", first)
	}
	if err := fs.Parse(rest); err == nil {
		t.Errorf("an unknown flag parsed cleanly: %v", rest)
	}
}

func TestTakeFirstOnNoPositionalReturnsNotFound(t *testing.T) {
	fs := newSet()
	first, rest, found := TakeFirst(fs, []string{"--minutes", "5"})
	if found || first != "" {
		t.Errorf("first = %q found = %v, want empty/false", first, found)
	}
	if strings.Join(rest, " ") != "--minutes 5" {
		t.Errorf("rest = %v, want the arguments unchanged", rest)
	}
}

func TestTakeFirstOnEmptyArgsIsNotAPanic(t *testing.T) {
	fs := newSet()
	if _, rest, found := TakeFirst(fs, nil); found || len(rest) != 0 {
		t.Errorf("nil args produced %v / %v", rest, found)
	}
}

func TestTakeFirstKeepsASingleDashAsAPositional(t *testing.T) {
	// "-" is not a flag by any shell convention.
	fs := newSet()
	first, _, found := TakeFirst(fs, []string{"-", "--minutes", "5"})
	if !found || first != "-" {
		t.Errorf("first = %q found=%v, want -/true", first, found)
	}
}

type discard struct{}

func (discard) Write(p []byte) (int, error) { return len(p), nil }
