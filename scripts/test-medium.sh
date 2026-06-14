#!/usr/bin/env sh
set -eu

# Git exports repository-specific environment variables to hook processes
# (e.g. lefthook pre-push). Drop them so tests that create temporary git
# repositories never operate on this repository instead.
unset GIT_DIR GIT_WORK_TREE GIT_INDEX_FILE GIT_COMMON_DIR GIT_OBJECT_DIRECTORY GIT_PREFIX

go test ./...
