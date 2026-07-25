# SPDX-License-Identifier: Apache-2.0 OR MIT

.PHONY: all build check clean test

all: check build

build:
	go build ./cmd/libinput-curve

test:
	go test -race ./...

check:
	test -z "$$(gofmt -l cmd internal)"
	go vet ./...
	go test ./...

clean:
	rm -rf libinput-curve dist coverage.out
