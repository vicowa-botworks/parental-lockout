package main

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// docs/cli.md promises "stable field names". A promise nobody checks is how a document
// becomes fiction, and it happened while writing this one: the first draft listed
// `configExists`, `profileCount` and a top-level `degraded` array for `status`, none of
// which the struct has, and it described `users[].override` as an object when it is a
// string holding the reason. Every one of those read plausibly. None would have failed
// anything.
//
// So: every field name the document puts in a table must exist as a JSON tag in this
// command's sources. The check is deliberately about existence rather than shape -- shape
// is what the per-command JSON tests already pin field by field -- because the failure this
// guards is the one nothing else can see: a name that was never in the code.
//
// It reads the tags rather than running the commands on purpose. `omitempty` means a real
// field is absent from any given sample, so a sample-based check would report a correct
// document as wrong whenever the fixture did not happen to populate a field -- and a test
// that cries wolf gets its assertions deleted.
func TestDocumentedJSONFieldsExist(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "cli.md"))
	if err != nil {
		t.Fatalf("read docs/cli.md: %v", err)
	}
	tags := jsonTagsInSources(t)
	if len(tags) == 0 {
		t.Fatal("no JSON tags found, so this test would pass on any document")
	}

	// Field tables are the ones whose header row is `| Field | Type | ... |`, and the check
	// is bounded by that header rather than by recognising a type name. The first version
	// skipped any row whose Type column it did not know, which meant a single row could
	// escape the check by typing itself `integer` instead of `int` -- verified, and it
	// passed clean. Inside a field table every row is a field, so an unrecognised type is
	// now a failure rather than an exemption.
	var (
		inFieldTable bool
		checked      int
	)
	for _, line := range strings.Split(string(doc), "\n") {
		cells := splitRow(line)
		if len(cells) == 0 {
			inFieldTable = false
			continue
		}
		if cells[0] == "Field" {
			inFieldTable = true
			continue
		}
		if !inFieldTable {
			continue
		}
		if strings.HasPrefix(cells[0], "-") {
			continue // the |---|---| separator row
		}
		name := strings.Trim(cells[0], "`")
		if name == cells[0] && !strings.Contains(name, "`") {
			t.Errorf("row in a field table has no field name in backticks: %q", line)
			continue
		}
		// The type is the first word of its cell: the document writes
		// "string — the stable key to branch on", and the prose is for the reader.
		typ := ""
		if len(cells) > 1 {
			typ = strings.TrimSpace(strings.SplitN(cells[1], " ", 2)[0])
			typ = strings.Trim(typ, ",;")
		}
		if !fieldTypes[typ] {
			t.Errorf("field %q has unrecognised type %q; add it to fieldTypes if it is real",
				name, cellOr(cells, 1))
			continue
		}
		// One row may name several fields of the same type -- `locked`, `unconfirmed`,
		// `refused` are all ints -- and each has to exist.
		for _, field := range backticked(cells[0]) {
			// The leaf is what has to exist as a tag; the prefix says which object holds
			// it, and the per-command JSON tests own that.
			leaf := field
			if i := strings.LastIndexAny(leaf, ".[]"); i >= 0 {
				leaf = leaf[i+1:]
			}
			if leaf == "" {
				continue
			}
			checked++
			if !tags[leaf] {
				t.Errorf("docs/cli.md documents %q (leaf %q); no such JSON tag exists in cmd/plock", field, leaf)
			}
		}
	}
	if checked < 20 {
		t.Fatalf("only %d documented fields parsed; the table format changed and this test is now vacuous", checked)
	}
}

// backticked returns the contents of every `code span` in a cell.
func backticked(s string) []string {
	var out []string
	for {
		i := strings.IndexByte(s, '`')
		if i < 0 {
			return out
		}
		s = s[i+1:]
		j := strings.IndexByte(s, '`')
		if j < 0 {
			return out
		}
		if name := strings.TrimSpace(s[:j]); name != "" {
			out = append(out, name)
		}
		s = s[j+1:]
	}
}

// fieldTypes are the type words docs/cli.md is allowed to use. The list is closed on
// purpose: a new spelling is either a real type worth adding here or a typo, and both
// answers should be a deliberate edit rather than a silent pass.
var fieldTypes = map[string]bool{
	"string": true, "int": true, "bool": true, "object": true,
	"[]string": true, "[]object": true,
}

// splitRow returns the trimmed cells of a markdown table row, or nil for a non-row. A
// blank line and any prose return nil, which is what ends a table.
func splitRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "|") || !strings.HasSuffix(trimmed, "|") {
		return nil
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(trimmed, "|"), "|")
	parts := strings.Split(inner, "|")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, strings.TrimSpace(p))
	}
	return out
}

func cellOr(cells []string, i int) string {
	if i < len(cells) {
		return cells[i]
	}
	return ""
}

// jsonTagsInSources collects every `json:"name"` in the package's non-test sources, plus
// internal/config's, because `profile show` emits config.Profile directly and its fields are
// documented here too.
func jsonTagsInSources(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	dirs := []string{".", filepath.Join("..", "..", "internal", "config")}
	tag := regexp.MustCompile("`json:\"([a-zA-Z][a-zA-Z0-9_]*)")
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		for _, e := range entries {
			n := e.Name()
			if !strings.HasSuffix(n, ".go") || strings.HasSuffix(n, "_test.go") {
				continue
			}
			raw, err := os.ReadFile(filepath.Join(dir, n))
			if err != nil {
				t.Fatalf("read %s: %v", n, err)
			}
			for _, m := range tag.FindAllStringSubmatch(string(raw), -1) {
				out[m[1]] = true
			}
		}
	}
	return out
}

// The document is the operator's entry point, so it has to exist and cover the commands
// that exist. A new command with no section is how a doc quietly stops being true.
func TestCLIDocCoversEveryCommand(t *testing.T) {
	doc, err := os.ReadFile(filepath.Join("..", "..", "docs", "cli.md"))
	if err != nil {
		t.Fatalf("read docs/cli.md: %v", err)
	}
	text := string(doc)
	for _, name := range topLevelCommands(t) {
		if !strings.Contains(text, "plock "+name) {
			t.Errorf("docs/cli.md never mentions `plock %s`", name)
		}
	}
}

// topLevelCommands reads the command list out of globalUsage, the same text `plock -h`
// prints, rather than restating it here -- a second list would drift from the first, which
// is the failure this file exists to catch.
func topLevelCommands(t *testing.T) []string {
	t.Helper()
	// The list is the indented "name<spaces>description" column, not the `plock <command>`
	// synopsis line: that is how globalUsage presents it, and matching the synopsis would
	// find the placeholder rather than the commands.
	var out []string
	row := regexp.MustCompile(`^\s+([a-z][a-z-]+)\s{2,}\S`)
	for _, line := range strings.Split(globalUsage, "\n") {
		if m := row.FindStringSubmatch(line); m != nil {
			out = append(out, m[1])
		}
	}
	sort.Strings(out)
	if len(out) < 5 {
		t.Fatalf("found %d commands in globalUsage: %v", len(out), out)
	}
	return out
}
