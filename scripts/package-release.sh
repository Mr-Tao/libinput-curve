#!/bin/sh
# SPDX-License-Identifier: Apache-2.0 OR MIT

set -eu
umask 022

usage() {
	printf 'Usage: %s VERSION [OUTPUT-DIRECTORY]\n' "$0" >&2
}

if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
	usage
	exit 64
fi

version=$1
output_argument=${2:-dist}
if ! printf '%s\n' "$version" |
	grep -Eq '^(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$'; then
	printf 'package-release: VERSION must be MAJOR.MINOR.PATCH\n' >&2
	exit 64
fi

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repo_root=$(CDPATH='' cd -- "$script_dir/.." && pwd)
cd "$repo_root"

mkdir -p "$output_argument"
output_dir=$(CDPATH='' cd -- "$output_argument" && pwd)

if [ -z "${SOURCE_DATE_EPOCH:-}" ]; then
	SOURCE_DATE_EPOCH=$(git log -1 --format=%ct)
	export SOURCE_DATE_EPOCH
fi

archive_root=libinput-curve-"$version"-linux-amd64
archive_name=$archive_root.tar.gz
temporary=$(mktemp -d "${TMPDIR:-/tmp}/libinput-curve-release.XXXXXX")
trap 'rm -rf "$temporary"' EXIT HUP INT TERM
stage=$temporary/$archive_root
document_root=$stage/share/doc/libinput-curve
license_root=$stage/share/licenses/libinput-curve

mkdir -p \
	"$stage/bin" \
	"$stage/share/man/man1" \
	"$stage/share/bash-completion/completions" \
	"$stage/share/zsh/site-functions" \
	"$stage/share/fish/vendor_completions.d" \
	"$stage/share/systemd/user" \
	"$document_root/examples" \
	"$license_root"

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
	go build -trimpath -ldflags "-s -w -X main.version=$version" \
	-o "$stage/bin/libinput-curve" ./cmd/libinput-curve

embedded_version=$("$stage/bin/libinput-curve" version)
if [ "$embedded_version" != "$version" ]; then
	printf 'package-release: embedded version is %s, expected %s\n' \
		"$embedded_version" "$version" >&2
	exit 70
fi

make --no-print-directory man BUILD_DIR="$temporary/generated"
cp "$temporary/generated/man/libinput-curve.1" \
	"$stage/share/man/man1/libinput-curve.1"
"$stage/bin/libinput-curve" completion bash \
	> "$stage/share/bash-completion/completions/libinput-curve"
"$stage/bin/libinput-curve" completion zsh \
	> "$stage/share/zsh/site-functions/_libinput-curve"
"$stage/bin/libinput-curve" completion fish \
	> "$stage/share/fish/vendor_completions.d/libinput-curve.fish"

cp README.md CONTRIBUTING.md \
	docs/backends.md docs/configuration.md docs/scrolling.md \
	docs/man/libinput-curve.1.scd "$document_root/"
cp examples/windows-epp-reference.json "$document_root/examples/"
cp contrib/systemd/README.md "$document_root/systemd.md"
cp contrib/systemd/libinput-curve-watch.service "$stage/share/systemd/user/"
cp LICENSE LICENSE-APACHE LICENSE-MIT "$license_root/"

tar --sort=name \
	--mtime="@$SOURCE_DATE_EPOCH" \
	--owner=0 --group=0 --numeric-owner \
	-C "$temporary" -cf - "$archive_root" |
	gzip -n > "$output_dir/$archive_name"

(
	cd "$output_dir"
	sha256sum "$archive_name" > SHA256SUMS
)

printf '%s\n' "$output_dir/$archive_name" "$output_dir/SHA256SUMS"
