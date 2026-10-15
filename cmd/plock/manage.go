package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"github.com/vicowa-botworks/parental-lockout/internal/cli"
	"github.com/vicowa-botworks/parental-lockout/internal/config"
)

const profileUsage = `usage: plock profile list [--json]
       plock profile show <id> [--json]
       plock profile add <id> [--name TEXT] [--timezone TZ] [--copy-from <id>]
       plock profile edit <id> [--name TEXT] [--timezone TZ]
       plock profile rm <id>
       plock profile rule add <id> <rule-id> --days LIST --start HH:MM --end HH:MM
                                [--name TEXT] [--all-day] [--disable]
       plock profile rule rm <id> <rule-id>
       plock profile rule enable|disable <id> <rule-id>

Profiles are the schedules; users are assigned to one. A headless machine is
administrable over SSH with these and the user subcommands.

--timezone must be an IANA zone name the system knows, such as Europe/London. A zone
that does not resolve is refused here rather than falling back to UTC at runtime,
because a silently-wrong zone moves every window in the profile.
`

const userUsage = `usage: plock user list [--json]
       plock user assign <user> <profile>
       plock user enable <user>
       plock user disable <user>
       plock user rm <user>

assigning a user to a profile replaces their previous assignment; the old profile
stays in the file until "plock profile rm" removes it.

disabling a user stops them being locked out entirely. It is the loud off switch, and
it is reported as such by "plock status".
`

func runProfile(stdout, stderr io.Writer, args []string) int {
	if len(args) == 0 {
		// A bare group name is a typo, not a request for help: it goes to stderr with the
		// usage error code. `-h` is the request, and it is not an error.
		writeAll(stderr, profileUsage)
		return cli.Usage
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return profileList(stdout, stderr, rest)
	case "show":
		return profileShow(stdout, stderr, rest)
	case "add":
		return profileAdd(stdout, stderr, rest)
	case "edit":
		return profileEdit(stdout, stderr, rest)
	case "rm":
		return profileRm(stdout, stderr, rest)
	case "rule":
		return profileRule(stdout, stderr, rest)
	case "-h", "--help", "help":
		return runHelp(stdout, stderr, []string{"profile"})
	default:
		fmt.Fprintf(stderr, "plock profile: unknown subcommand %q\n\n%s", sub, profileUsage)
		return cli.Usage
	}
}

func runUser(stdout, stderr io.Writer, args []string) int {
	if len(args) == 0 {
		writeAll(stderr, userUsage)
		return cli.Usage
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "list":
		return userList(stdout, stderr, rest)
	case "assign":
		return userAssign(stdout, stderr, rest)
	case "enable", "disable":
		return userEnabled(stdout, stderr, sub == "enable", rest)
	case "rm":
		return userRm(stdout, stderr, rest)
	case "-h", "--help", "help":
		return runHelp(stdout, stderr, []string{"user"})
	default:
		fmt.Fprintf(stderr, "plock user: unknown subcommand %q\n\n%s", sub, userUsage)
		return cli.Usage
	}
}

// newFlagSet is the shape every subcommand shares: a config path, an optional --json, and
// help that goes to stdout while a bad flag goes to stderr.
type cmdFlags struct {
	fs     *flag.FlagSet
	out    *bytes.Buffer
	path   *string
	asJSON *bool
}

// jsonWanted reports whether this invocation asked for --json. Nil-safe because the
// flag is only declared on the read commands: a write subcommand has no --json to read,
// and a helper that panicked on the nil would make the shared failure path unusable
// there -- which is how commands end up owning their own error handling again.
func (c cmdFlags) jsonWanted() bool { return c.asJSON != nil && *c.asJSON }

func newFlagSet(name, usage string, withJSON bool) cmdFlags {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	var buf bytes.Buffer
	fs.SetOutput(&buf)
	c := cmdFlags{fs: fs, out: &buf, path: fs.String("config", defaultConfigPath, "path to the config document")}
	if withJSON {
		c.asJSON = fs.Bool("json", false, "machine-readable output on stdout")
	}
	fs.Usage = func() { buf.WriteString(usage) }
	return c
}

// parse handles flag's own reporting and the one distinction worth making in every
// command: asking for help is not a typo.
func (c cmdFlags) parse(stderr io.Writer, args []string) (ok bool, help bool) {
	err := c.fs.Parse(args)
	if err == nil {
		return true, false
	}
	if errors.Is(err, flag.ErrHelp) {
		// flag.ErrHelp means the caller asked for help. The usage text is already in
		// c.out because fs.Usage wrote it there; the caller decides that it belongs on
		// stdout and that the exit code is 0.
		return false, true
	}
	fmt.Fprintf(stderr, "plock %s: %v\n%s", c.fs.Name(), err, c.out.String())
	return false, false
}

func profileList(stdout, stderr io.Writer, args []string) int {
	c := newFlagSet("profile list", profileUsage, true)
	if ok, help := c.parse(stderr, args); !ok {
		if help {
			writeAll(stdout, c.out.String())
			return cli.OK
		}
		return cli.Usage
	}
	cfg, code := loadForEdit(c, stderr, *c.path, "profile list")
	if code != cli.OK {
		return code
	}

	type row struct {
		ID    string `json:"id"`
		Name  string `json:"name"`
		Rules int    `json:"rules"`
		// Assigned counts the users pointing at this profile. It is on the listing because
		// it is the number that decides whether `profile rm` can succeed, and finding out
		// by attempting the removal is the wrong order.
		Assigned int `json:"assigned"`
	}
	rows := make([]row, 0, len(cfg.Profiles))
	for _, p := range cfg.Profiles {
		rows = append(rows, row{ID: p.ID, Name: p.Name, Rules: len(p.Rules), Assigned: assignedCount(cfg, p.ID)})
	}
	if *c.asJSON {
		if err := cli.JSON(stdout, map[string]any{"profiles": rows}); err != nil {
			return opFail(stderr, true, "plock profile list", codeOutputFailed, err)
		}
		return cli.OK
	}
	r := &cli.Report{}
	if len(rows) == 0 {
		r.Line("no profiles")
		return mustWrite(r, stdout, stderr, "profile list")
	}
	for _, x := range rows {
		r.Line("%s  %s  %d rule(s)  %d user(s)", x.ID, x.Name, x.Rules, x.Assigned)
	}
	return mustWrite(r, stdout, stderr, "profile list")
}

func profileShow(stdout, stderr io.Writer, args []string) int {
	c := newFlagSet("profile show", profileUsage, true)
	id, flagArgs, _ := cli.TakeFirst(c.fs, args)
	if ok, help := c.parse(stderr, flagArgs); !ok {
		if help {
			writeAll(stdout, c.out.String())
			return cli.OK
		}
		return cli.Usage
	}
	if id == "" {
		fmt.Fprint(stderr, "plock profile show: a profile id is required\n\n")
		return cli.Usage
	}
	cfg, code := loadForEdit(c, stderr, *c.path, "profile show")
	if code != cli.OK {
		return code
	}
	p, ok := findProfile(cfg, id)
	if !ok {
		return opFail(stderr, c.jsonWanted(), "plock profile show", codeNoSuchProfile,
			fmt.Errorf("no profile %q", id))
	}

	if *c.asJSON {
		if err := cli.JSON(stdout, p); err != nil {
			return opFail(stderr, true, "plock profile show", codeOutputFailed, err)
		}
		return cli.OK
	}
	r := &cli.Report{}
	r.Field("id", p.ID)
	r.Field("name", p.Name)
	r.Field("timezone", p.Timezone)
	r.Section("rules")
	if len(p.Rules) == 0 {
		r.Line("  none — this profile never locks anybody out")
	}
	for _, rule := range p.Rules {
		state := "enabled"
		if !rule.Enabled {
			state = "disabled"
		}
		window := fmt.Sprintf("%s–%s", rule.Start, rule.End)
		if rule.AllDay {
			window = "all day"
		}
		r.Line("  %s  %s  %s  %s  (%s)", rule.ID, strings.Join(rule.Days, ","), window, rule.Name, state)
	}
	r.Field("assigned", assignedCount(cfg, id))
	return mustWrite(r, stdout, stderr, "profile show")
}

func profileAdd(stdout, stderr io.Writer, args []string) int {
	c := newFlagSet("profile add", profileUsage, false)
	id, flagArgs, _ := cli.TakeFirst(c.fs, args)
	name := c.fs.String("name", "", "display name")
	tz := c.fs.String("timezone", "UTC", "IANA timezone")
	copyFrom := c.fs.String("copy-from", "", "start from an existing profile's rules")
	if ok, help := c.parse(stderr, flagArgs); !ok {
		if help {
			writeAll(stdout, c.out.String())
			return cli.OK
		}
		return cli.Usage
	}
	if id == "" {
		fmt.Fprint(stderr, "plock profile add: a profile id is required\n\n")
		return cli.Usage
	}
	cfg, code := loadForEdit(c, stderr, *c.path, "profile add")
	if code != cli.OK {
		return code
	}
	if _, ok := findProfile(cfg, id); ok {
		// add does not overwrite. Silently replacing a profile would replace every child
		// assigned to its schedule, and `edit` exists for changing one on purpose.
		fmt.Fprintf(stderr, "plock profile add: profile %q already exists (use `plock profile edit`)\n", id)
		return cli.Usage
	}

	p := config.Profile{ID: id, Name: *name, Timezone: *tz}
	if p.Name == "" {
		p.Name = id
	}
	if *copyFrom != "" {
		src, ok := findProfile(cfg, *copyFrom)
		if !ok {
			fmt.Fprintf(stderr, "plock profile add: no profile %q to copy from\n", *copyFrom)
			return cli.Failure
		}
		// The rules and the timezone travel together. A rule is a wall-clock time, so its
		// meaning depends on the zone it is read in: taking a profile's rules but leaving
		// the copy on a different zone would shift every window by the offset between them
		// while the rule text looked identical to the source. --timezone still wins, below.
		// The identity does not come across: id and name are the copy's own.
		p.Rules = append([]config.Rule(nil), src.Rules...)
		p.Timezone = src.Timezone
		if *name == "" {
			p.Name = src.Name
		}
		if *tz != "UTC" {
			// An explicit --timezone beats the source's, and only an explicit one: "UTC" is
			// the flag's default, so treating it as a choice would make it impossible to
			// copy a profile's zone without also naming it.
			p.Timezone = *tz
		}
	}
	if err := checkTimezone(p.Timezone); err != nil {
		fmt.Fprintf(stderr, "plock profile add: %v\n", err)
		return cli.Usage
	}

	cfg.UpsertProfile(p)
	return saveEdited(cfg, *c.path, stdout, stderr, "profile add", func(r *cli.Report) {
		r.Field("profile", p.ID)
		r.Field("name", p.Name)
		r.Field("timezone", p.Timezone)
		r.Field("rules", len(p.Rules))
	})
}

func profileEdit(stdout, stderr io.Writer, args []string) int {
	c := newFlagSet("profile edit", profileUsage, false)
	id, flagArgs, _ := cli.TakeFirst(c.fs, args)
	name := c.fs.String("name", "", "display name")
	tz := c.fs.String("timezone", "", "IANA timezone")
	if ok, help := c.parse(stderr, flagArgs); !ok {
		if help {
			writeAll(stdout, c.out.String())
			return cli.OK
		}
		return cli.Usage
	}
	if id == "" {
		fmt.Fprint(stderr, "plock profile edit: a profile id is required\n\n")
		return cli.Usage
	}
	if *name == "" && *tz == "" {
		fmt.Fprint(stderr, "plock profile edit: nothing to change (give --name or --timezone)\n")
		return cli.Usage
	}
	cfg, code := loadForEdit(c, stderr, *c.path, "profile edit")
	if code != cli.OK {
		return code
	}
	p, ok := findProfile(cfg, id)
	if !ok {
		fmt.Fprintf(stderr, "plock profile edit: no profile %q\n", id)
		return cli.Failure
	}
	if *tz != "" {
		if err := checkTimezone(*tz); err != nil {
			fmt.Fprintf(stderr, "plock profile edit: %v\n", err)
			return cli.Usage
		}
		p.Timezone = *tz
	}
	if *name != "" {
		p.Name = *name
	}
	cfg.UpsertProfile(p)
	return saveEdited(cfg, *c.path, stdout, stderr, "profile edit", func(r *cli.Report) {
		r.Field("profile", p.ID)
		r.Field("name", p.Name)
		r.Field("timezone", p.Timezone)
	})
}

func profileRm(stdout, stderr io.Writer, args []string) int {
	c := newFlagSet("profile rm", profileUsage, false)
	id, flagArgs, _ := cli.TakeFirst(c.fs, args)
	if ok, help := c.parse(stderr, flagArgs); !ok {
		if help {
			writeAll(stdout, c.out.String())
			return cli.OK
		}
		return cli.Usage
	}
	if id == "" {
		fmt.Fprint(stderr, "plock profile rm: a profile id is required\n\n")
		return cli.Usage
	}
	cfg, code := loadForEdit(c, stderr, *c.path, "profile rm")
	if code != cli.OK {
		return code
	}
	if _, ok := findProfile(cfg, id); !ok {
		fmt.Fprintf(stderr, "plock profile rm: no profile %q\n", id)
		return cli.Failure
	}

	// The safety rule of this whole file. A user whose profile id resolves to nothing has
	// no rules, and no rules means no lockout: deleting a profile that children are
	// assigned to releases those children silently, on the same tick the daemon next reads
	// the file. That is the fail-open direction, and it is reachable by one plausible
	// command.
	// There is deliberately no --force here. config.DeleteProfile refuses an assignment
	// still pointing at the profile, so a flag promising to override it would either lie --
	// which is the `plock lock --force` mistake of promising something no code keeps -- or
	// would have to decide what happens to those users, and every answer there is a policy
	// the operator did not ask for: silently unmanaging a child is its own silent release.
	// Reassigning first is one extra command and it is the one that says what you meant.
	if names := assignedUsers(cfg, id); len(names) > 0 {
		fmt.Fprintf(stderr, "plock profile rm: %d user(s) are assigned to %q: %s\n"+
			"Removing it would leave them with no schedule, which means no lockout.\n"+
			"Reassign them first: plock user assign <user> <profile>\n", len(names), id, strings.Join(names, ", "))
		return cli.Usage
	}

	// The error is reported rather than discarded. DeleteProfile re-checks the assignment
	// for the same reason this command does, and a refusal from the layer that writes the
	// file is the one answer that must reach the operator.
	if err := cfg.DeleteProfile(id); err != nil {
		fmt.Fprintf(stderr, "plock profile rm: %v\n", err)
		return cli.Failure
	}
	return saveEdited(cfg, *c.path, stdout, stderr, "profile rm", func(r *cli.Report) {
		r.Field("removed", id)
	})
}

func profileRule(stdout, stderr io.Writer, args []string) int {
	if len(args) == 0 {
		writeAll(stderr, profileUsage)
		return cli.Usage
	}
	sub, rest := args[0], args[1:]
	switch sub {
	case "-h", "--help", "help":
		writeAll(stdout, profileUsage)
		return cli.OK
	case "add":
		return ruleAdd(stdout, stderr, rest)
	case "rm":
		return ruleRm(stdout, stderr, rest)
	case "enable", "disable":
		return ruleEnabled(stdout, stderr, sub == "enable", rest)
	default:
		fmt.Fprintf(stderr, "plock profile rule: unknown subcommand %q\n\n%s", sub, profileUsage)
		return cli.Usage
	}
}

func ruleAdd(stdout, stderr io.Writer, args []string) int {
	c := newFlagSet("profile rule add", profileUsage, false)
	// Two positionals, so TakeFirst is used twice: the profile, then the rule.
	profileID, rest, _ := cli.TakeFirst(c.fs, args)
	ruleID, flagArgs, _ := cli.TakeFirst(c.fs, rest)

	days := c.fs.String("days", "mon,tue,wed,thu,fri,sat,sun", "comma-separated weekdays")
	start := c.fs.String("start", "", "window start HH:MM (required)")
	end := c.fs.String("end", "", "window end HH:MM (required)")
	name := c.fs.String("name", "", "display name")
	allDay := c.fs.Bool("all-day", false, "hold the whole day, ignoring --start and --end")
	disable := c.fs.Bool("disable", false, "add the rule switched off")

	if ok, help := c.parse(stderr, flagArgs); !ok {
		if help {
			writeAll(stdout, c.out.String())
			return cli.OK
		}
		return cli.Usage
	}
	if profileID == "" || ruleID == "" {
		fmt.Fprint(stderr, "plock profile rule add: a profile id and a rule id are required\n\n")
		return cli.Usage
	}
	if !*allDay && (*start == "" || *end == "") {
		// Both or neither. A rule with one endpoint is the shape the evaluator has to widen
		// to the whole day, which is the opposite of what a half-filled flag line means.
		fmt.Fprint(stderr, "plock profile rule add: --start and --end are both required (or --all-day)\n")
		return cli.Usage
	}

	cfg, code := loadForEdit(c, stderr, *c.path, "profile rule add")
	if code != cli.OK {
		return code
	}
	p, ok := findProfile(cfg, profileID)
	if !ok {
		fmt.Fprintf(stderr, "plock profile rule add: no profile %q\n", profileID)
		return cli.Failure
	}
	dayList, err := parseDays(*days)
	if err != nil {
		fmt.Fprintf(stderr, "plock profile rule add: %v\n", err)
		return cli.Usage
	}

	rule := config.Rule{
		ID: ruleID, Name: *name, Days: dayList, Start: *start, End: *end,
		AllDay: *allDay, Enabled: !*disable,
	}
	if rule.Name == "" {
		rule.Name = ruleID
	}
	for _, existing := range p.Rules {
		if existing.ID == ruleID {
			// Same refusal as `profile add`: a rule id is what `rule rm` and `rule disable`
			// take, so replacing one silently means those commands later act on a rule the
			// operator never saw.
			fmt.Fprintf(stderr, "plock profile rule add: profile %q already has a rule %q\n", profileID, ruleID)
			return cli.Usage
		}
	}
	p.Rules = append([]config.Rule{}, append(p.Rules, rule)...)
	cfg.UpsertProfile(p)
	return saveEdited(cfg, *c.path, stdout, stderr, "profile rule add", func(r *cli.Report) {
		r.Field("profile", profileID)
		r.Field("rule", ruleID)
		if rule.AllDay {
			r.Field("window", "all day")
		} else {
			r.Field("window", fmt.Sprintf("%s–%s", rule.Start, rule.End))
		}
		r.Field("days", strings.Join(rule.Days, ","))
	})
}

func ruleRm(stdout, stderr io.Writer, args []string) int {
	c := newFlagSet("profile rule rm", profileUsage, false)
	profileID, rest, _ := cli.TakeFirst(c.fs, args)
	ruleID, flagArgs, _ := cli.TakeFirst(c.fs, rest)
	if ok, help := c.parse(stderr, flagArgs); !ok {
		if help {
			writeAll(stdout, c.out.String())
			return cli.OK
		}
		return cli.Usage
	}
	if profileID == "" || ruleID == "" {
		fmt.Fprint(stderr, "plock profile rule rm: a profile id and a rule id are required\n\n")
		return cli.Usage
	}
	cfg, code := loadForEdit(c, stderr, *c.path, "profile rule rm")
	if code != cli.OK {
		return code
	}
	p, ok := findProfile(cfg, profileID)
	if !ok {
		fmt.Fprintf(stderr, "plock profile rule rm: no profile %q\n", profileID)
		return cli.Failure
	}
	kept := make([]config.Rule, 0, len(p.Rules))
	var found bool
	for _, r := range p.Rules {
		if r.ID == ruleID {
			found = true
			continue
		}
		kept = append(kept, r)
	}
	if !found {
		fmt.Fprintf(stderr, "plock profile rule rm: profile %q has no rule %q\n", profileID, ruleID)
		return cli.Failure
	}
	p.Rules = kept
	cfg.UpsertProfile(p)
	return saveEdited(cfg, *c.path, stdout, stderr, "profile rule rm", func(r *cli.Report) {
		r.Field("profile", profileID)
		r.Field("removed rule", ruleID)
		r.Field("rules left", len(p.Rules))
	})
}

func ruleEnabled(stdout, stderr io.Writer, enable bool, args []string) int {
	c := newFlagSet("profile rule "+verb(enable), profileUsage, false)
	profileID, rest, _ := cli.TakeFirst(c.fs, args)
	ruleID, flagArgs, _ := cli.TakeFirst(c.fs, rest)
	if ok, help := c.parse(stderr, flagArgs); !ok {
		if help {
			writeAll(stdout, c.out.String())
			return cli.OK
		}
		return cli.Usage
	}
	if profileID == "" || ruleID == "" {
		fmt.Fprintf(stderr, "plock profile rule %s: a profile id and a rule id are required\n\n", verb(enable))
		return cli.Usage
	}
	cfg, code := loadForEdit(c, stderr, *c.path, "profile rule "+verb(enable))
	if code != cli.OK {
		return code
	}
	p, ok := findProfile(cfg, profileID)
	if !ok {
		fmt.Fprintf(stderr, "plock profile rule %s: no profile %q\n", verb(enable), profileID)
		return cli.Failure
	}
	var found bool
	next := make([]config.Rule, len(p.Rules))
	for i, r := range p.Rules {
		if r.ID == ruleID {
			r.Enabled = enable
			found = true
		}
		next[i] = r
	}
	if !found {
		fmt.Fprintf(stderr, "plock profile rule %s: profile %q has no rule %q\n", verb(enable), profileID, ruleID)
		return cli.Failure
	}
	p.Rules = next
	cfg.UpsertProfile(p)
	return saveEdited(cfg, *c.path, stdout, stderr, "profile rule "+verb(enable), func(r *cli.Report) {
		r.Field("profile", profileID)
		r.Field("rule", ruleID)
		r.Field("enabled", enable)
	})
}

func userList(stdout, stderr io.Writer, args []string) int {
	c := newFlagSet("user list", userUsage, true)
	if ok, help := c.parse(stderr, args); !ok {
		if help {
			writeAll(stdout, c.out.String())
			return cli.OK
		}
		return cli.Usage
	}
	cfg, code := loadForEdit(c, stderr, *c.path, "user list")
	if code != cli.OK {
		return code
	}
	type row struct {
		Username string `json:"username"`
		Profile  string `json:"profile"`
		Enabled  bool   `json:"enabled"`
		// ProfileExists is the difference between "assigned to a profile with no rules" and
		// "assigned to a profile that is not in the file". Both mean no lockout; only one
		// is a mistake someone made.
		ProfileExists bool `json:"profileExists"`
	}
	users := append([]config.User(nil), cfg.Users...)
	sort.Slice(users, func(i, j int) bool { return users[i].Username < users[j].Username })
	rows := make([]row, 0, len(users))
	for _, u := range users {
		_, exists := findProfile(cfg, u.ProfileID)
		rows = append(rows, row{Username: u.Username, Profile: u.ProfileID, Enabled: u.Enabled, ProfileExists: exists})
	}
	if *c.asJSON {
		if err := cli.JSON(stdout, map[string]any{"users": rows}); err != nil {
			return opFail(stderr, true, "plock user list", codeOutputFailed, err)
		}
		return cli.OK
	}
	r := &cli.Report{}
	if len(rows) == 0 {
		r.Line("no managed users")
		return mustWrite(r, stdout, stderr, "user list")
	}
	for _, x := range rows {
		state := "enabled"
		if !x.Enabled {
			state = "DISABLED — never locked out"
		}
		suffix := ""
		if !x.ProfileExists {
			suffix = "  WARNING no such profile"
		}
		r.Line("%s  %s  %s%s", x.Username, x.Profile, state, suffix)
	}
	return mustWrite(r, stdout, stderr, "user list")
}

func userAssign(stdout, stderr io.Writer, args []string) int {
	c := newFlagSet("user assign", userUsage, false)
	name, rest, _ := cli.TakeFirst(c.fs, args)
	profileID, flagArgs, _ := cli.TakeFirst(c.fs, rest)
	if ok, help := c.parse(stderr, flagArgs); !ok {
		if help {
			writeAll(stdout, c.out.String())
			return cli.OK
		}
		return cli.Usage
	}
	if name == "" || profileID == "" {
		fmt.Fprint(stderr, "plock user assign: a username and a profile id are required\n\n")
		return cli.Usage
	}
	cfg, code := loadForEdit(c, stderr, *c.path, "user assign")
	if code != cli.OK {
		return code
	}
	// The profile has to exist. Assigning a child to a name that is not in the file is the
	// same silent release as deleting the profile from under them, and it is easier to type.
	if _, ok := findProfile(cfg, profileID); !ok {
		fmt.Fprintf(stderr, "plock user assign: no profile %q\n", profileID)
		return cli.Failure
	}
	u, existed := cfg.UserFor(name)
	u.Username = name
	u.ProfileID = profileID
	if !existed {
		// A brand new managed user starts enforced. Defaulting a new account to unmanaged
		// would mean the first `user assign` on a fresh install did nothing.
		u.Enabled = true
	}
	cfg.UpsertUser(u)
	return saveEdited(cfg, *c.path, stdout, stderr, "user assign", func(r *cli.Report) {
		r.Field("user", name)
		r.Field("profile", profileID)
		if !existed {
			r.Field("note", "new managed user, enforcement on")
		}
	})
}

func userEnabled(stdout, stderr io.Writer, enable bool, args []string) int {
	c := newFlagSet("user "+verb(enable), userUsage, false)
	name, flagArgs, _ := cli.TakeFirst(c.fs, args)
	if ok, help := c.parse(stderr, flagArgs); !ok {
		if help {
			writeAll(stdout, c.out.String())
			return cli.OK
		}
		return cli.Usage
	}
	if name == "" {
		fmt.Fprintf(stderr, "plock user %s: a username is required\n\n", verb(enable))
		return cli.Usage
	}
	cfg, code := loadForEdit(c, stderr, *c.path, "user "+verb(enable))
	if code != cli.OK {
		return code
	}
	u, ok := cfg.UserFor(name)
	if !ok {
		fmt.Fprintf(stderr, "plock user %s: no managed user %q\n", verb(enable), name)
		return cli.Failure
	}
	u.Enabled = enable
	cfg.UpsertUser(u)
	return saveEdited(cfg, *c.path, stdout, stderr, "user "+verb(enable), func(r *cli.Report) {
		r.Field("user", name)
		r.Field("enabled", enable)
		if !enable {
			// Disabling is the one CLI action that stops enforcement for a child without
			// touching the schedule, so the confirmation says what it means rather than
			// echoing a boolean.
			r.Line("\n%s is no longer locked out at all. Re-enable with `plock user enable %s`.", name, name)
		}
	})
}

func userRm(stdout, stderr io.Writer, args []string) int {
	c := newFlagSet("user rm", userUsage, false)
	name, flagArgs, _ := cli.TakeFirst(c.fs, args)
	if ok, help := c.parse(stderr, flagArgs); !ok {
		if help {
			writeAll(stdout, c.out.String())
			return cli.OK
		}
		return cli.Usage
	}
	if name == "" {
		fmt.Fprint(stderr, "plock user rm: a username is required\n\n")
		return cli.Usage
	}
	cfg, code := loadForEdit(c, stderr, *c.path, "user rm")
	if code != cli.OK {
		return code
	}
	if _, ok := cfg.UserFor(name); !ok {
		fmt.Fprintf(stderr, "plock user rm: no managed user %q\n", name)
		return cli.Failure
	}
	cfg.RemoveUser(name)
	return saveEdited(cfg, *c.path, stdout, stderr, "user rm", func(r *cli.Report) {
		r.Field("removed", name)
		r.Line("\nThe account itself is untouched on this machine — only plock stops managing it.")
	})
}

// saveEdited validates and writes, then prints the confirmation built by describe.
//
// The gate is ValidateForEdit, not Validate. These commands author a profile or an
// assignment, so everything about what they changed is checked -- but the administrator
// credential is a field none of them touch, and refusing to add a child because the web UI
// has no login yet leaves that child unenforced.
func saveEdited(cfg *config.Config, path string, stdout, stderr io.Writer, what string, describe func(*cli.Report)) int {
	if err := cfg.ValidateForEdit(); err != nil {
		fmt.Fprintf(stderr, "plock %s: refusing to write: %v\n", what, err)
		return cli.Failure
	}
	if err := cfg.Save(path); err != nil {
		fmt.Fprintf(stderr, "plock %s: %v\n", what, err)
		return cli.Failure
	}
	r := &cli.Report{}
	describe(r)
	return mustWrite(r, stdout, stderr, what)
}

// loadForEdit reads the document a write will be based on.
//
// A config that will not load is refused rather than defaulted: writing on top of a
// document this build cannot read would replace bytes the operator wrote with a document
// assembled from defaults, and the daemon would then enforce something nobody configured.
func loadForEdit(c cmdFlags, stderr io.Writer, path, what string) (*config.Config, int) {
	cfg, err := config.Load(path)
	if err != nil {
		return nil, opFail(stderr, c.jsonWanted(), "plock "+what, codeConfigUnreadable, err)
	}
	return cfg, cli.OK
}

func findProfile(cfg *config.Config, id string) (config.Profile, bool) {
	for _, p := range cfg.Profiles {
		if p.ID == id {
			return p, true
		}
	}
	return config.Profile{}, false
}

func assignedCount(cfg *config.Config, id string) int {
	return len(assignedUsers(cfg, id))
}

func assignedUsers(cfg *config.Config, id string) []string {
	var out []string
	for _, u := range cfg.Users {
		if u.ProfileID == id {
			out = append(out, u.Username)
		}
	}
	sort.Strings(out)
	return out
}

// checkTimezone refuses a zone the system cannot resolve.
//
// The evaluator does not refuse: an unknown zone falls back to the default and reports
// itself degraded, which is right at read time -- a daemon must keep enforcing -- and wrong
// at write time, where the operator is standing there and can fix it. A silently-wrong
// zone moves every window in the profile by hours, and the schedule still looks correct.
func checkTimezone(name string) error {
	if name == "" {
		return errors.New("a timezone is required")
	}
	if _, err := time.LoadLocation(name); err != nil {
		return fmt.Errorf("timezone %q is not one this system knows: %v", name, err)
	}
	return nil
}

// parseDays accepts the three-letter and long weekday names, in any case.
func parseDays(spec string) ([]string, error) {
	want := map[string]string{
		"mon": "mon", "monday": "mon", "tue": "tue", "tuesday": "tue",
		"wed": "wed", "wednesday": "wed", "thu": "thu", "thursday": "thu",
		"fri": "fri", "friday": "fri", "sat": "sat", "saturday": "sat",
		"sun": "sun", "sunday": "sun",
	}
	var out []string
	for _, part := range strings.Split(spec, ",") {
		key := strings.ToLower(strings.TrimSpace(part))
		if key == "" {
			continue
		}
		v, ok := want[key]
		if !ok {
			return nil, fmt.Errorf("unknown weekday %q (use mon, tue, wed, thu, fri, sat, sun)", part)
		}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil, errors.New("--days named no weekday")
	}
	return out, nil
}

func verb(enable bool) string {
	if enable {
		return "enable"
	}
	return "disable"
}
