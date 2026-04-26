#!/usr/bin/env sh
set -eu

unformatted=$(
	find . \
		-name '*.go' \
		-not -path './.git/*' \
		-not -path './vendor/*' \
		-exec gofmt -l {} +
)

if [ -n "$unformatted" ]; then
	echo "Go files need formatting:" >&2
	printf '%s\n' "$unformatted" >&2
	echo "Run ./scripts/fmt.sh or make fmt." >&2
	exit 1
fi
