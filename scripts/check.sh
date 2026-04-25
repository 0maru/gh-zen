#!/usr/bin/env sh
set -eu

./scripts/fmt.sh
./scripts/lint.sh
./scripts/test-medium.sh
