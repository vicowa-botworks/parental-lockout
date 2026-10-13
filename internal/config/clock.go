package config

import (
	"regexp"
	"strconv"
)

// timeOfDay is the documented wall-clock format. It is deliberately strict:
// "9:00" and "24:00" are what a hand edit produces, and both are ambiguous once
// they are in the file.
//
// The two groups are for ParseClock. Nothing else may read the digits out of a
// time, so that the format and the arithmetic cannot disagree about what a legal
// time is.
var timeOfDay = regexp.MustCompile(`^([01]\d|2[0-3]):([0-5]\d)$`)

// ParseClock converts a documented "HH:MM" wall-clock string into minutes past
// midnight.
//
// It lives here, next to the format it implements, because the format has two
// readers with different jobs: Validate decides whether a document may be saved,
// and the schedule evaluator decides whether a child is locked out right now. If
// those two ever disagree about what a legal time is, the disagreement is a rule
// that saves cleanly and then never fires — a lockout that silently does not
// happen, which is the failure this product cannot have.
func ParseClock(s string) (int, bool) {
	m := timeOfDay.FindStringSubmatch(s)
	if m == nil {
		return 0, false
	}

	hour, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	minute, err := strconv.Atoi(m[2])
	if err != nil {
		return 0, false
	}
	return hour*60 + minute, true
}
