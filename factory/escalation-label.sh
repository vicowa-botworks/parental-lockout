#!/usr/bin/env bash
# escalation-label.sh -- print the name of the label that marks an artifact as owing a human a decision.
#
# Issue #315. The name used to be written out in full in five files -- `AGENTS.md`, the skill twice,
# `docs/factory.md`, and the tick instruction in `factory/factory.sh` -- with a leg in `selftest.sh`
# pinning one of them. Each place was a chance to drift, and #311 (what the name even is) is still an
# open human decision, so the name now lives on one line of `common.sh` and this script is how anything
# reads it: a tick in the middle of an escalation is not going to source 2,850 lines of configuration,
# and a tick that does not read the name is a tick that pastes a stale one into a write against GitHub.
#
# It prints the value and nothing else, so `--add-label "$(factory/escalation-label.sh)"` is the whole
# command. It is a read: it never touches GitHub, and it refuses rather than printing nothing. An empty
# label is worse than a missing script -- `gh --add-label ""` is a call aimed at no name at all, and the
# step it belongs to is the one whose whole job is to leave a visible marker.
#
# shellcheck shell=bash disable=SC2016
set -euo pipefail

_here=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=./common.sh
# shellcheck disable=SC1091
. "$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/common.sh"

if [ -z "${FACTORY_ESCALATION_LABEL:-}" ]; then
	printf 'escalation-label.sh: FACTORY_ESCALATION_LABEL is empty in %s/common.sh -- refusing to print a name that would hand `gh` a bare --add-label\n' "$_here" >&2
	exit 1
fi

printf '%s\n' "$FACTORY_ESCALATION_LABEL"
