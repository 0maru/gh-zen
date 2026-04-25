#!/usr/bin/env sh
set -eu

if [ "$#" -gt 0 ]; then
	for path in "$@"; do
		case "$path" in
			*.go)
				if [ -f "$path" ]; then
					gofmt -w "$path"
				fi
				;;
		esac
	done
	exit 0
fi

find . \
	-name '*.go' \
	-not -path './.git/*' \
	-not -path './vendor/*' \
	-exec gofmt -w {} +
