#!/usr/bin/env sh
set -eu

repo_root=$(git rev-parse --show-toplevel 2>/dev/null || pwd)
cd "$repo_root"

output=${1:-gh-zen}
output_dir=$(dirname "$output")

if [ "$output_dir" != "." ]; then
	mkdir -p "$output_dir"
fi

go build -trimpath -o "$output" .
