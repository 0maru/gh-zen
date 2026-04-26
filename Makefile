SHELL := /bin/sh
OUTPUT ?= gh-zen

.DEFAULT_GOAL := help

.PHONY: help
help:
	@printf '%s\n' 'Available targets:'
	@printf '  %-18s %s\n' 'setup' 'Install local Git hooks'
	@printf '  %-18s %s\n' 'build' 'Build the gh-zen extension binary'
	@printf '  %-18s %s\n' 'install-extension' 'Build and install this checkout as a local gh extension'
	@printf '  %-18s %s\n' 'run-extension' 'Run the installed gh extension'
	@printf '  %-18s %s\n' 'fmt' 'Format source files'
	@printf '  %-18s %s\n' 'fmt-check' 'Check source formatting'
	@printf '  %-18s %s\n' 'lint' 'Run lint checks'
	@printf '  %-18s %s\n' 'test-small' 'Run the fast deterministic test gate'
	@printf '  %-18s %s\n' 'test-medium' 'Run the default local integration test gate'
	@printf '  %-18s %s\n' 'test-large' 'Run explicitly enabled large tests'
	@printf '  %-18s %s\n' 'check' 'Run the normal local validation gate'
	@printf '  %-18s %s\n' 'agent-check' 'Run the fast agent validation gate'
	@printf '  %-18s %s\n' 'clean' 'Remove local build artifacts'

.PHONY: setup
setup:
	lefthook install

.PHONY: build
build:
	./scripts/build.sh "$(OUTPUT)"

.PHONY: install-extension
install-extension: build
	gh extension install . --force

.PHONY: run-extension
run-extension:
	gh zen

.PHONY: fmt
fmt:
	./scripts/fmt.sh

.PHONY: fmt-check
fmt-check:
	./scripts/fmt-check.sh

.PHONY: lint
lint:
	./scripts/lint.sh

.PHONY: test-small
test-small:
	./scripts/test-small.sh

.PHONY: test-medium test
test-medium test:
	./scripts/test-medium.sh

.PHONY: test-large
test-large:
	GH_ZEN_LARGE_TESTS=1 ./scripts/test-large.sh

.PHONY: check
check:
	./scripts/check.sh

.PHONY: agent-check
agent-check:
	lefthook run agent-check

.PHONY: clean
clean:
	rm -f gh-zen
