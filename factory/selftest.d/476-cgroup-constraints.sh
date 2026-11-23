# shellcheck shell=bash
# shellcheck disable=SC2016,SC2034,SC2154
# 476-cgroup-constraints.sh -- the fence lines #476's Option 1 puts in the [Service] section.
#
# The class: whatever a tick starts stands in the unit's cgroup. `H-0312`'s MCP fence removed
# one family of it (the servers nobody asked for), and `TimeoutStopSec=` bounds what one
# grandchild that ignores SIGTERM costs the queue (60 s, the #246 lesson) -- but nothing bounded
# what may STAND there at all until a maintainer answered #390's open half on 21 Nov with
# Option 1: constrain (`gh issue view 476` comment of 2026-11-21T01:29:43Z; decision `H-0403`).
# The shipped generator now renders `TasksMax=` and a `MemoryHigh=`/`MemoryMax=` pair, sized
# from this unit's OWN cgroup read on 23 Nov with a tick in flight: `pids.peak=113` against an
# inherited `pids.max=212828`, `memory.peak=571 MiB` on a 186 GiB machine.
#
# What the legs pin, and the polarities:
#   (a) the fence: the rendered unit carries an un-commented `TasksMax=` that is a number inside
#       the pinned margin 256..4096 (113 is what one real pass peaked at; below 256 starves a
#       legitimate `make verify`, above 4096 fences nothing), and `MemoryHigh=`/`MemoryMax=` are
#       integer percentages with High >= 10, Max <= 100 and Max > High -- a High of 100 is
#       `max` in clothes, and Max <= High means the throttle is the kill;
#   (b) the absence controls: delete either line from the GENERATOR and the same check refuses
#       -- the comment above the lines cannot be what the leg is accidentally matching;
#   (c) the drift control: shrink the generator's TasksMax to 64 and the margin check refuses,
#       so the ok above is a range, not the check certifying whatever digits shipped;
#   (d) the parser itself, and its own control: `systemd-analyze verify` must print no
#       `, ignoring` complaint about the shipped rendering -- an ignored setting is a
#       no-fence that reads green, the very failure mode the `SuccessExitStatus=` comma was
#       measured into this template. The control and the grep had to be MEASURED, not assumed:
#       on systemd 259.9 a first plant of `TasksMax=not-a-number` parsed SILENTLY (14-value
#       probe, 23 Nov -- garbage words and suffixed spellings are all quiet, and the only value
#       that drew a complaint was the out-of-range `TasksMax=-5`, which prints
#       `Invalid maximum tasks value '-5', ignoring:` -- a different message family from the
#       comma's `Failed to parse value, ignoring`). So the plant is `-5` and the grep is the
#       lowercase `ignoring` that both families share (the machine's `/etc` noise is
#       `Ignoring.` with a capital and never matches). Two consequences, both kept honest:
#       systemd ITSELF would swallow a words-shaped typo in TasksMax, which is why arm (a) is
#       the strong check and this arm only proves the parser read the line; and "systemd is
#       happy" without this control is no evidence at all -- that false lead is H-0403's.
#       The leg SKIPs where `systemd-analyze` is not installed, saying so.

it476_work=$work/cgroup476
it476_saved_tree=$gen_tree

it476_say() { printf '%s\n' "$*" | sed 's/^/            /' >&2; }

# it476_pct_num <value> -> the integer of a `<digits>%` value on stdout; refusal on anything
# else, so `max`, `infinity`, bytes and `25.5%` are all equally refused: the fence is written in
# percents because the unit file is rendered per machine, and a non-integer percent is a
# systemd-version coin-flip nobody needs to settle on a queue machine.
it476_pct_num() { # <value>
	local n=$1
	case "$n" in
	*%) n=${n%\%} ;;
	*) return 1 ;;
	esac
	case "$n" in
	'' | *[!0-9]*) return 1 ;;
	esac
	printf '%s' "$n"
}

it476_leg_fence() {
	local t h m hn mn
	if [ ! -s "$gen_svc" ]; then
		it476_say "the render is empty -- the generator wrote no [Service] section for"
		it476_say "this leg to read, so every line below is absence, not verdict."
		return 1
	fi
	if grep -q '@' "$gen_svc"; then
		it476_say "a @TOKEN@ survived render: $(grep -n '@' "$gen_svc" | head -n 1)"
		it476_say "systemd would be handed the token, not a value."
		return 1
	fi
	t=$(deadline_value "$gen_svc" TasksMax)
	if [ -z "$t" ]; then
		it476_say "no un-commented TasksMax= line in the rendered unit. A commented-out one"
		it476_say "does not fence anything; #476 Option 1 is a line systemd loads."
		return 1
	fi
	case "$t" in
	'' | *[!0-9]*)
		it476_say "TasksMax=$t is not a plain number -- 'max' is the same word as absent here."
		return 1
		;;
	esac
	if [ "$t" -lt 256 ] || [ "$t" -gt 4096 ]; then
		it476_say "TasksMax=$t is outside the pinned margin 256..4096. This unit's own cgroup"
		it476_say "peaked at 113 processes with one real tick in flight (pids.peak, 23 Nov):"
		it476_say "below 256 a legitimate make verify is starved, above 4096 the number fences"
		it476_say "nothing a grandchild could reach. Change the decision, then the number."
		return 1
	fi
	h=$(deadline_value "$gen_svc" MemoryHigh)
	m=$(deadline_value "$gen_svc" MemoryMax)
	if [ -z "$h" ] || [ -z "$m" ]; then
		it476_say "MemoryHigh= or MemoryMax= is missing from the rendered unit (High=[$h] Max=[$m])."
		return 1
	fi
	if ! hn=$(it476_pct_num "$h") || ! mn=$(it476_pct_num "$m"); then
		it476_say "the memory pair is not two integer percentages (High=[$h] Max=[$m]): percents"
		it476_say "scale with the machine this installer runs on, which bytes would not."
		return 1
	fi
	if [ "$hn" -lt 10 ] || [ "$mn" -gt 100 ] || [ "$mn" -le "$hn" ]; then
		it476_say "the memory pair is out of shape (High=${hn}% Max=${mn}%): High below 10% throttles"
		it476_say "a legitimate go build, Max above High means the kernel kills before it ever"
		it476_say "throttles, and High at 100 is max in clothes."
		return 1
	fi
	return 0
}

# it476_leg_parses_clean <unit-file>: ok iff `systemd-analyze verify` prints no lowercase
# `ignoring` complaint for it -- both message families this systemd uses for a setting it is
# about to swallow end in `, ignoring`, and the machine's own /etc noise is the capital-I
# `Ignoring.` form, which never matches. 2 = the parser is not installed; gen_leg reports SKIP.
it476_leg_parses_clean() { # <unit file>
	local out
	command -v systemd-analyze >/dev/null 2>&1 || return 2
	mkdir -p -- "$it476_work/verify" || return 1
	cp -- "$1" "$it476_work/verify/probe.service" || return 1
	out=$(systemd-analyze verify "$it476_work/verify/probe.service" 2>&1 || true)
	if printf '%s\n' "$out" | grep -q 'ignoring'; then
		it476_say "systemd is about to swallow a line of the rendering it was handed:"
		printf '%s\n' "$out" | grep 'ignoring' | sed 's/^/              /' >&2
		it476_say "an ignored setting is a no-fence that reads green -- the SuccessExitStatus"
		it476_say "comma is the precedent measured on this very template."
		return 1
	fi
	return 0
}

if [ ! -f "$HERE/install-timer.sh" ]; then
	printf '  SKIP  %-40s install-timer.sh is not in this tree\n' "the unit cgroup fence"
elif ! gen_setup "$it476_work/genrepo" 1; then
	printf '  SKIP  %-40s the throwaway generator could not be built in %s\n' \
		"the unit cgroup fence" "$it476_work"
else
	gen_tree=$it476_work/genrepo
	if ! gen_dry_run "$HERE/install-timer.sh"; then
		fail=$((fail + 1))
		printf '  FAIL  %-40s the generator refused its own fixture,\n' \
			"the unit cgroup fence"
		printf '          so nothing was rendered to ask: %s\n' "$(sed -n '1,1p' "$gen_err")"
	else
		gen_split
		gen_leg 'fence lines and margins' \
			'TasksMax numeric in 256..4096, Memory pair integer percents, High < Max' \
			it476_leg_fence
		gen_control 'fence: TasksMax line deleted' "$it476_work/plant-notasks" \
			'/^TasksMax=@TASKS_MAX@$/d' it476_leg_fence
		gen_control 'fence: MemoryMax line deleted' "$it476_work/plant-nomem" \
			'/^MemoryMax=@MEM_MAX@$/d' it476_leg_fence
		gen_control 'fence: TasksMax shrunk to 64' "$it476_work/plant-tiny" \
			's/^TASKS_MAX=[0-9][0-9]*$/TASKS_MAX=64/' it476_leg_fence

		# The parser arms. The controls above clobbered $gen_svc with a planted rendering, so
		# re-run the shipped generator before asking systemd about "the shipped unit" -- asking
		# the last plant and calling it the ship is the #1180 shape this file refuses.
		if gen_dry_run "$HERE/install-timer.sh"; then
			gen_split
			gen_leg 'fence parses clean in systemd' \
				'no , ignoring complaint for the shipped rendering' \
				it476_leg_parses_clean "$gen_svc"
			if command -v systemd-analyze >/dev/null 2>&1; then
				# The plant is an out-of-range NUMBER because a words-shaped garbage value
				# parses silently on this systemd (measured 23 Nov, 14 values: only `-5` drew
				# the `Invalid maximum tasks value '-5', ignoring:` line). A plant the parser
				# never complains about would certify nothing, so the plant's own refusal is
				# what this arm asserts. Output is CAPTURED then grepped, not piped into
				# `grep -q`: that is the neighbouring precedent's shape (`unit_exit_status_codes`
				# at `factory/selftest.sh:16081` "good=$(systemd-analyze verify"), and the first
				# suite run of this leg red the control through a `| grep -q` pipe that a
				# standalone mini-run of the same bytes greened -- the capture form is the one
				# that matches the precedent under audit. The plant line is asserted in the
				# rendered unit before systemd is asked, so the control can only go red because
				# of what systemd said, never because a plant silently failed to land.
				it476_bad=$(gen_planted "$it476_work/plant-badparse" \
					's/^TASKS_MAX=[0-9][0-9]*$/TASKS_MAX=-5/' || true)
				if [ -n "$it476_bad" ] && gen_dry_run "$it476_bad"; then
					gen_split
					it476_hit=$(deadline_value "$gen_svc" TasksMax)
					if [ "$it476_hit" != "-5" ]; then
						printf '  SKIP  %-40s the plant did not land (TasksMax=%s)\n' \
							"control: TasksMax=-5 out of range" "$it476_hit"
					else
						mkdir -p -- "$it476_work/verify" || true
						cp -- "$gen_svc" "$it476_work/verify/probe.service" || true
						it476_out=$(systemd-analyze verify \
							"$it476_work/verify/probe.service" 2>&1 || true)
						if printf '%s\n' "$it476_out" | grep -q 'ignoring'; then
							printf '  ok    %-40s the same parser refuses it\n' \
								"control: TasksMax=-5 out of range"
						else
							fail=$((fail + 1))
							printf '  FAIL  %-40s systemd parsed it quietly, so the ok\n' \
								"control: TasksMax=-5 out of range"
							printf '          above never proved the parser read the line.\n'
							printf '%s\n' "$it476_out" | sed 's/^/              /' >&2
						fi
					fi
				else
					printf '  SKIP  %-40s the plant could not be rendered\n' \
						"control: TasksMax=-5 out of range"
				fi
			fi
		else
			fail=$((fail + 1))
			printf '  FAIL  %-40s re-rendering the shipped generator failed\n' \
				"the unit cgroup fence (parser)"
		fi
	fi
	gen_tree=$it476_saved_tree
fi
