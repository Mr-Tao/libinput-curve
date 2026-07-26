# SPDX-License-Identifier: Apache-2.0 OR MIT

GO ?= go
SCDOC ?= scdoc
VERSION ?= 0.1.0-dev
BUILD_DIR ?= build
DIST_DIR ?= dist

MANPAGE := $(BUILD_DIR)/man/libinput-curve.1

.PHONY: all build check check-all clean docs docs-check man release shellcheck test

all: check build

build:
	$(GO) build -ldflags="-X main.version=$(VERSION)" -o libinput-curve ./cmd/libinput-curve

test:
	$(GO) test -race ./...

check:
	test -z "$$(gofmt -l cmd internal)"
	$(GO) vet ./...
	$(GO) test ./...

docs: man

man:
	mkdir -p "$(dir $(MANPAGE))"
	$(SCDOC) < docs/man/libinput-curve.1.scd > "$(MANPAGE)"

docs-check:
	@set -eu; \
	tmpdir="$$(mktemp -d)"; \
	trap 'rm -rf "$$tmpdir"' EXIT; \
	$(MAKE) --no-print-directory man BUILD_DIR="$$tmpdir/build"; \
	test -s "$$tmpdir/build/man/libinput-curve.1"; \
	groff -man -z -ww "$$tmpdir/build/man/libinput-curve.1"; \
	$(GO) run ./cmd/libinput-curve completion bash > "$$tmpdir/libinput-curve.bash"; \
	$(GO) run ./cmd/libinput-curve completion zsh > "$$tmpdir/_libinput-curve"; \
	$(GO) run ./cmd/libinput-curve completion fish > "$$tmpdir/libinput-curve.fish"; \
	bash -n "$$tmpdir/libinput-curve.bash"; \
	zsh -n "$$tmpdir/_libinput-curve"; \
	fish -n "$$tmpdir/libinput-curve.fish"

shellcheck:
	shellcheck scripts/package-release.sh

check-all: check test docs-check shellcheck

release:
	@test "$(VERSION)" != "0.1.0-dev" || { \
		echo "VERSION must be a release version such as 0.1.0" >&2; \
		exit 2; \
	}
	scripts/package-release.sh "$(VERSION)" "$(DIST_DIR)"

clean:
	rm -rf libinput-curve build dist coverage.out
