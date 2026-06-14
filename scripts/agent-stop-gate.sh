#!/usr/bin/env sh
set -eu

# Stop-gate hook for coding agents (Claude Code / Codex).
#
# When the agent tries to finish a turn while uncommitted Go changes exist,
# run the normal validation gate (fmt-check + lint + test-medium). On failure,
# block the stop and feed the output back so the agent keeps fixing until the
# gate passes. A retry counter caps the loop at max_attempts to avoid running
# forever when the agent cannot fix the failure.

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$repo_root"

max_attempts=3

hook_input=""
if [ ! -t 0 ]; then
	hook_input=$(cat 2>/dev/null || true)
fi

session_id=""
if command -v jq >/dev/null 2>&1 && [ -n "$hook_input" ]; then
	session_id=$(printf '%s' "$hook_input" | jq -r '.session_id // empty' 2>/dev/null || true)
fi
session_id=$(printf '%s' "$session_id" | tr -cd 'A-Za-z0-9_-')
counter_file="${TMPDIR:-/tmp}/gh-zen-stop-gate-${session_id:-default}.count"

clear_counter() {
	rm -f "$counter_file"
}

# Only gate uncommitted Go-related changes; lefthook pre-commit and pre-push
# already gate committed work.
changed=$(git status --porcelain -- '*.go' 'go.mod' 'go.sum' 2>/dev/null || true)
if [ -z "$changed" ]; then
	clear_counter
	exit 0
fi

# Run every check even if an earlier one fails so the agent sees all issues
# in a single pass. Failure output goes to stdout for capture.
run_gate() {
	gate_status=0
	./scripts/fmt-check.sh 2>&1 || gate_status=$?
	./scripts/lint.sh 2>&1 || gate_status=$?
	./scripts/test-medium.sh 2>&1 || gate_status=$?
	return "$gate_status"
}

if gate_output=$(run_gate); then
	clear_counter
	exit 0
fi

attempts=0
if [ -f "$counter_file" ]; then
	attempts=$(cat "$counter_file" 2>/dev/null || echo 0)
fi
case "$attempts" in
	'' | *[!0-9]*) attempts=0 ;;
esac

if [ "$attempts" -ge "$max_attempts" ]; then
	clear_counter
	{
		echo "Stop gate still failing after ${max_attempts} attempts; allowing stop."
		echo "Run 'make check' and fix the remaining issues manually."
	} >&2
	exit 1
fi

attempts=$((attempts + 1))
printf '%s' "$attempts" > "$counter_file"

summary=$(printf '%s\n' "$gate_output" | tail -n 120)
reason="Stop gate failed (attempt ${attempts}/${max_attempts}): uncommitted Go changes do not pass the normal gate (fmt-check + lint + test-medium). Fix the issues below, then finish again.

${summary}"

if command -v jq >/dev/null 2>&1; then
	# JSON decision output: blocks the stop and shows the reason to the agent.
	jq -n --arg reason "$reason" '{decision: "block", reason: $reason}'
	exit 0
fi

# Fallback without jq: exit code 2 also blocks and feeds stderr to the agent.
printf '%s\n' "$reason" >&2
exit 2
