<!-- SPDX-License-Identifier: Apache-2.0 OR MIT -->

# Contributing

## Scope

Keep changes focused on declarative custom libinput curves, safe Xorg runtime
configuration, verification, persistence, or a real compositor-owned backend.
Do not add input interception, shell evaluation, hidden auto-apply behavior,
or claims that an independent libinput context configures Wayland.

## Development

```console
$ make check
$ make test
$ make docs-check
$ make shellcheck
$ make build
```

New parsing, planning, matching, or rendering behavior needs table-driven
tests. Backend code must be testable with a fake command runner and must pass
arguments directly rather than constructing a shell command.

`make docs-check` requires `scdoc`, `groff`, Bash, Zsh, and Fish. The complete
validation set is available as `make check-all`. Generated man pages and
completions are build artifacts and must not be committed.

For a live test:

1. capture `libinput-curve devices` and `status`;
2. run `plan` and inspect every target and property;
3. use a disposable profile or keep a known rollback command;
4. run `apply`;
5. require the built-in readback verification;
6. test hot-unplug separately from initial application.

Do not add real usernames, device serials, private paths, or unsanitized logs
to fixtures.

## Compatibility

Treat output and schema changes as interfaces. Keep human output readable,
JSON deterministic enough for consumers, and `render-xorg` stable. New
configuration fields must remain strict and documented.

Wayland work must name the compositor and demonstrate that its native input
path changed. Xwayland-only success is not sufficient.

## Commits

Use focused commits and include a Developer Certificate of Origin sign-off:

```console
$ git commit --signoff
```
