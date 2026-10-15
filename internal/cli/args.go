package cli

import (
	"flag"
	"strings"
)

// TakeFirst pulls the leading positional argument out of args and returns it with the
// remaining arguments in their original order, ready for flag.Parse.
//
// It exists because Go's flag package stops parsing at the first argument that is not a
// flag, and that makes the documented shape of `plock unlock <user> --minutes N --reason
// TEXT` parse as a command with five positional arguments and no flags. The username comes
// first because that is how every command line a human copies from a manual is written;
// `plock unlock --minutes 30 --reason x casey` is not a thing anyone types.
//
// The rule is deliberately narrow: the first token that is neither a flag nor a flag's
// VALUE becomes the positional, and everything else is handed back untouched so flag
// reports its own errors about its own arguments.
//
// Distinguishing the two needs the FlagSet, which is why this takes one rather than being
// a string helper. Without it `--reason film night` is ambiguous: "night" looks exactly
// like a positional, and treating it as one would silently drop the tail of a reason
// instead of reporting that the reason had an unquoted word in it.
//
// A "--" ends flag parsing in the ordinary way, and everything after it is left where it
// is for flag to turn into positionals — so `unlock -- -x` still means a user literally
// named "-x", and `unlock --minutes 5 -- -h` does not become help.
func TakeFirst(fs *flag.FlagSet, args []string) (first string, rest []string, found bool) {
	rest = make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		a := args[i]

		if a == "--" {
			// Everything after the separator is positional by definition. Copy it through
			// and stop looking: a token that looks like a flag after "--" is a value, and
			// a value is never the username the caller is asking for.
			rest = append(rest, args[i:]...)
			return "", rest, false
		}

		if !strings.HasPrefix(a, "-") || a == "-" {
			if !found {
				first, found = a, true
				continue
			}
			rest = append(rest, a)
			continue
		}

		// A flag. Copy it, and consume its value when it takes one, so the value is never
		// mistaken for the positional.
		rest = append(rest, a)
		name := strings.TrimLeft(a, "-")
		if i+1 < len(args) && flagWantsValue(fs, a, name) {
			i++
			rest = append(rest, args[i])
		}
	}
	return first, rest, found
}

// flagWantsValue reports whether args[i] is a non-boolean flag written without "=", and
// so is about to consume args[i+1] as its value.
func flagWantsValue(fs *flag.FlagSet, arg, name string) bool {
	if strings.Contains(arg, "=") {
		// --minutes=5 carries its value already.
		return false
	}
	f := fs.Lookup(name)
	if f == nil {
		// An unknown flag has unknowable arity. Assume it takes a value: flag.Parse is
		// going to refuse the line either way ("flag provided but not defined"), and the
		// only question is what happens on the way there. Assuming valueless would let a
		// typo'd flag's value be taken as the username, which turns a clear complaint about
		// the flag into a confident wrong answer about a child's account.
		return true
	}
	// flag's own definition of a boolean flag: it does not consume a following value.
	_, isBool := f.Value.(boolFlag)
	return !isBool
}

// boolFlag is the interface flag itself uses to decide whether a flag takes a value.
// Repeating it here is the only way to ask the question without parsing.
type boolFlag interface {
	IsBoolFlag() bool
}
