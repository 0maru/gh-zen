#!/usr/bin/env sh
set -eu

if [ "${GH_ZEN_LARGE_TESTS:-}" != "1" ]; then
	echo "Large tests are opt-in. Run with GH_ZEN_LARGE_TESTS=1." >&2
	exit 1
fi

go test -tags=large ./...
