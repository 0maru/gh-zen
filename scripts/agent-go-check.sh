#!/usr/bin/env sh
set -eu

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$repo_root"

if ! command -v jq >/dev/null 2>&1; then
	echo "jq is not installed; running the Go fast gate without targeted formatting." >&2
	./scripts/lint.sh
	./scripts/test-small.sh
	exit 0
fi

hook_input=$(mktemp)
changed_paths=$(mktemp)
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
				./scripts/fmt.sh "$clean_path"
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

if command -v lefthook >/dev/null 2>&1; then
	lefthook run agent-check
else
	./scripts/lint.sh
	./scripts/test-small.sh
fi
