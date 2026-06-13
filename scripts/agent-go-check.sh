#!/usr/bin/env sh
set -eu

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$repo_root"

# Exit code 2 feeds stderr back to the agent (Claude Code / Codex) so it can
# read the failure and self-correct. Any other non-zero exit code is only
# shown to the user and never reaches the agent.
fail_with_feedback() {
	{
		echo "Go fast gate failed. Fix the issues below before continuing."
		printf '%s\n' "$1"
	} >&2
	exit 2
}

# Run every check even if an earlier one fails so the agent sees all issues
# in a single pass. Failure output goes to stdout for capture.
run_fast_gate() {
	gate_status=0
	if command -v lefthook >/dev/null 2>&1; then
		lefthook run agent-check 2>&1 || gate_status=$?
	else
		./scripts/lint.sh 2>&1 || gate_status=$?
		./scripts/test-small.sh 2>&1 || gate_status=$?
	fi
	return "$gate_status"
}

if ! command -v jq >/dev/null 2>&1; then
	echo "jq is not installed; running the Go fast gate without targeted formatting." >&2
	if ! gate_output=$(run_fast_gate); then
		fail_with_feedback "$gate_output"
	fi
	exit 0
fi

hook_input=$(mktemp "${TMPDIR:-/tmp}/agent-go-check.input.XXXXXX")
changed_paths=$(mktemp "${TMPDIR:-/tmp}/agent-go-check.paths.XXXXXX")
trap 'rm -f "$hook_input" "$changed_paths"' EXIT

cat > "$hook_input"

jq -r '
	[
		.. | strings | scan("[A-Za-z0-9_./-]+\\.go|[A-Za-z0-9_./-]*go\\.mod|[A-Za-z0-9_./-]*go\\.sum")
	]
	| unique
	| .[]
' "$hook_input" > "$changed_paths" 2>/dev/null || exit 0

if [ ! -s "$changed_paths" ]; then
	exit 0
fi

relevant=0

while IFS= read -r path; do
	case "$path" in
		/*)
			case "$path" in
				"$repo_root"/*)
					clean_path=${path#"$repo_root"/}
					;;
				*)
					continue
					;;
			esac
			;;
		*)
			clean_path=${path#./}
			;;
	esac

	case "$clean_path" in
		*.go)
			if [ -f "$clean_path" ]; then
				if ! fmt_output=$(./scripts/fmt.sh "$clean_path" 2>&1); then
					fail_with_feedback "$fmt_output"
				fi
				relevant=1
			fi
			;;
		go.mod | go.sum | */go.mod | */go.sum)
			if [ -f "$clean_path" ]; then
				relevant=1
			fi
			;;
	esac
done < "$changed_paths"

if [ "$relevant" -ne 1 ]; then
	exit 0
fi

if ! gate_output=$(run_fast_gate); then
	fail_with_feedback "$gate_output"
fi
