package platform

import (
	"fmt"
	"strconv"
	"strings"
)

// tableColumns are the columns parseSessionTable knows how to read. A column that is
// not in this set is ignored, which is what a future loginctl that adds one needs.
var tableColumns = map[string]tableCol{
	"SESSION": colSession,
	"UID":     colUID,
	"USER":    colUser,
	"SEAT":    colSeat,
	"LEADER":  colLeader,
	"CLASS":   colClass,
	"TTY":     colTTY,
	"IDLE":    colIdle,
	"SINCE":   colSince,
}

type tableCol int

const (
	colSession tableCol = iota
	colUID
	colUser
	colSeat
	colLeader
	colClass
	colTTY
	colIdle
	colSince
)

// requiredColumns must all be present in the header. SINCE and IDLE are not required
// because neither is used for a decision; the rest are, and reading a username out of
// the CLASS column because a build reordered the header is exactly the kind of wrong
// that produces a lock applied to the wrong account.
var requiredColumns = []tableCol{colSession, colUID, colUser, colSeat, colLeader, colClass}

// parseSessionTable reads loginctl's plain table.
//
// Columns are resolved by header NAME, not by position. The old code checked that the
// header began "SESSION UID" and then indexed fields 0..8, so a build that inserted a
// column anywhere after UID would have parsed successfully with every field from USER
// onward shifted by one — a session whose Class read as its TTY and whose username read
// as its seat, with nothing failing. Checking the header cannot prevent that, because
// the header is exactly what changed; reading it is the only thing that can.
func parseSessionTable(out []byte) ([]listEntry, error) {
	lines := strings.Split(string(out), "\n")

	header := -1
	var cols []tableCol // cols[i] is the column found at field index i
	for i, line := range lines {
		fields := strings.Fields(line)
		if !looksLikeHeader(fields) {
			continue
		}
		cols = cols[:0]
		seen := map[tableCol]bool{}
		for _, name := range fields {
			c, ok := tableColumns[name]
			if !ok {
				// An unknown column gets a slot so the positions after it still line
				// up, but nothing reads it.
				cols = append(cols, -1)
				continue
			}
			cols = append(cols, c)
			seen[c] = true
		}
		var missing []string
		for _, req := range requiredColumns {
			if !seen[req] {
				missing = append(missing, tableName[req])
			}
		}
		if len(missing) > 0 {
			// This is a header-shaped line that this code cannot read. Say which
			// columns are gone rather than falling through to a later line and guessing.
			return nil, fmt.Errorf("loginctl table header is missing %s (header: %s)",
				strings.Join(missing, ", "), strings.Join(fields, " "))
		}
		header = i
		break
	}
	if header < 0 {
		return nil, fmt.Errorf("no column header in the table output")
	}

	var entries []listEntry
	for _, line := range lines[header+1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if strings.HasSuffix(line, "listed.") {
			// loginctl's trailer: "2 sessions listed."
			continue
		}
		fields := strings.Fields(line)
		e, err := parseTableRow(fields, cols)
		if err != nil {
			// A row this code cannot read is a session that would go unenforced. The
			// whole sweep stops, because "some of the children are locked" is not a
			// state to enter quietly.
			return nil, fmt.Errorf("loginctl table row %q: %w", line, err)
		}
		entries = append(entries, e)
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("table output had no session rows")
	}
	return entries, nil
}

// tableName is the reverse of tableColumns, for reporting which columns went missing.
var tableName = map[tableCol]string{
	colSession: "SESSION", colUID: "UID", colUser: "USER", colSeat: "SEAT",
	colLeader: "LEADER", colClass: "CLASS", colTTY: "TTY", colIdle: "IDLE", colSince: "SINCE",
}

// looksLikeHeader decides whether a line is the column header.
//
// It cannot simply test that the line starts with SESSION, because that is the same
// position-locked assumption the rest of this file exists to remove: a build that
// reorders the columns would put the header itself out of reach. Instead a header is a
// line that names SESSION and whose every field is an all-caps token. A data row always
// contains at least a numeric UID, so it can never look like this.
func looksLikeHeader(fields []string) bool {
	if len(fields) < 2 {
		return false
	}
	sawSession := false
	for _, f := range fields {
		if f == "SESSION" {
			sawSession = true
		}
		for _, r := range f {
			if !(r >= 'A' && r <= 'Z' || r == '_') {
				return false
			}
		}
	}
	return sawSession
}

func parseTableRow(fields []string, cols []tableCol) (listEntry, error) {
	var e listEntry
	for i, f := range fields {
		if i >= len(cols) {
			break // extra trailing field in a row the header did not describe
		}
		switch cols[i] {
		case colSession:
			e.Session = tableStr(f)
		case colUID:
			n, err := strconv.ParseUint(f, 10, 32)
			if err != nil {
				return e, fmt.Errorf("UID column is %q, not a number", f)
			}
			e.UID = uint32(n)
		case colUser:
			e.User = tableStr(f)
		case colSeat:
			e.Seat = tableField(f)
		case colLeader:
			n, err := strconv.ParseUint(f, 10, 32)
			if err != nil {
				return e, fmt.Errorf("LEADER column is %q, not a process id", f)
			}
			e.Leader = int(n)
		case colClass:
			e.Class = tableStr(f)
		case colTTY:
			e.TTY = tableField(f)
		case colIdle:
			e.Idle = strings.EqualFold(f, "yes") || strings.EqualFold(f, "true")
		case colSince:
			// Only the first token. SINCE is "Thu 2026-10-01 15:47:23 UTC", which
			// strings.Fields splits into three, and reconstructing it would mean
			// special-casing "whatever is left when this is the last column". Nothing
			// decides on SINCE, so the first token is recorded for the sake of having
			// read the column and is not a value to build anything on.
			e.Since = tableField(f)
		case -1:
			// unknown column
		}
	}
	if len(fields) < len(cols) {
		// A short row is a truncated row: the columns past the end were never read, so
		// the session would be dropped or half-populated. Silently skipping it is how a
		// child stops being enforced with no error anywhere.
		return e, fmt.Errorf("row has %d fields, the header describes %d", len(fields), len(cols))
	}
	if e.Session == "" {
		return e, fmt.Errorf("no session id (SESSION column is %q)", fields[0])
	}
	return e, nil
}
