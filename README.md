<!-- SPDX-License-Identifier: Apache-2.0 OR MIT -->

# libinput-curve

`libinput-curve` validates, previews, applies, verifies, and persists custom
libinput acceleration curves. It focuses on the configuration gap between
libinput's low-level custom profile API and the practical Xorg interfaces
exposed by `xf86-input-libinput`.

The project is in early development. The JSON schema and command names may
change before the first stable release.

## What it does

- strictly validates reusable fallback, motion, and scroll curves;
- matches devices by product substring, regular expression, USB vendor/product
  ID, or a conjunction of those criteria;
- inventories current XInput support and state;
- computes an idempotent property plan without changing the session;
- refuses the complete plan before writing if any matched device is ambiguous
  or lacks a required property;
- applies only drifted properties and then reads them back for verification;
- watches for XInput hotplug with a bounded polling interval;
- renders deterministic `xorg.conf.d` `InputClass` sections for persistence;
- scales output points at runtime for display-PPI or other physical
  calibration workflows.

The tool does not capture input events, install a kernel module, replace
libinput, or run commands through a shell.

## Backend boundary

libinput configuration belongs to the process that owns the libinput context.
Under Xorg, `xf86-input-libinput` exposes custom curves through XInput
properties and `xorg.conf.d` options, so this tool can configure the active
server.

Under Wayland, the compositor owns the relevant libinput context. `xinput`
sees at most Xwayland devices and cannot configure native Wayland input.
`libinput-curve` therefore rejects live commands in a Wayland session by
default. `--allow-xwayland` is an explicit diagnostic escape hatch, not a
Wayland solution. A future compositor backend must use that compositor's real
configuration interface.

See [docs/backends.md](docs/backends.md) for the exact boundary and upstream
references.

## Build

Requirements:

- Go 1.23 or newer;
- `xinput` and an Xorg session for live commands;
- libinput 1.23 or newer plus a sufficiently recent
  `xf86-input-libinput` for custom profiles.

```console
$ go build -o libinput-curve ./cmd/libinput-curve
$ ./libinput-curve version
0.1.0-dev
```

The binary has no external Go dependencies.

## Configuration

The default path is
`${XDG_CONFIG_HOME:-$HOME/.config}/libinput-curve/config.json`.

```json
{
  "schema": "io.github.mr-tao.libinput-curve/v1",
  "profiles": {
    "example": {
      "motion": {
        "step": 0.5,
        "points": [0, 0.5, 2, 5]
      },
      "scroll": {
        "step": 1,
        "points": [0, 0.4, 1, 3]
      }
    }
  },
  "devices": [
    {
      "id": "example-mouse",
      "match": {
        "product_contains": "Example Mouse",
        "vendor_id": "1234",
        "product_id": "5678"
      },
      "profile": "example"
    }
  ]
}
```

Unknown JSON fields, duplicate rule IDs, unsafe identifiers, non-finite
numbers, unsupported Xorg curve sizes, and incomplete hardware IDs are
rejected. Omitted `motion_scale` and `scroll_scale` default to `1`.

`product_contains` can be represented by Xorg's `MatchProduct`. Arbitrary
`name_regex` works for live commands but intentionally makes `render-xorg`
fail because it cannot be translated without broadening the persistent match.

The complete schema and curve semantics are documented in
[docs/configuration.md](docs/configuration.md).

Wheel acceleration, event resolution, and application-side visual
interpolation are separate mechanisms. See
[docs/scrolling.md](docs/scrolling.md) before trying to make a detented mouse
wheel behave like a touchpad.

## Workflow

Start read-only:

```console
$ libinput-curve validate
$ libinput-curve devices
$ libinput-curve plan
$ libinput-curve status
```

`plan` reports exact drift but always remains read-only. `status` exits `0`
when every configured and present device is in sync, `1` for drift or
unmatched rules, and `2` for an ambiguous or unsupported matched device.

Apply only after reviewing the plan:

```console
$ libinput-curve apply
```

`apply` preflights every matched device before the first write, sets only
drifted properties, re-reads all devices, and fails if any drift remains.
XInput properties are not transactional; a device disappearing during apply
can still leave an incomplete update, which the final verification reports.
An advisory lock below `$XDG_RUNTIME_DIR` prevents concurrent `apply` and
`watch` writers.

To keep profiles across XInput hotplug:

```console
$ libinput-curve watch --interval 2s
```

The watcher reloads the configuration on every interval, emits output only
when state changes, and exits on backend, validation, ambiguity, or
verification failure so a service manager can restart or alert.

## Physical scaling

libinput's x-axis is device speed in device units per millisecond and the
y-axis is output pointer speed. Device CPI and display PPI therefore matter.
Runtime multipliers scale y values without changing the sampled x step:

```console
$ libinput-curve plan --motion-scale 0.67
$ libinput-curve apply --motion-scale 0.67
```

Per-rule factors in JSON are multiplied by command-line factors. Fallback and
motion curves use the motion factor; scroll curves use the scroll factor.
Inspect the resulting plan before applying a calibration factor.

The included
[Windows EPP reference example](examples/windows-epp-reference.json) contains
the 1000-CPI motion curve and Microsoft Sculpt scroll curve developed on one
real system. It is a reproducible starting point, not a claim that Windows
uses one universal curve for every control-panel setting, device CPI, display,
or registry configuration.

## Persistent Xorg configuration

Generate a file first:

```console
$ libinput-curve render-xorg \
    --output ./90-libinput-curve.conf
$ sudo install -Dm0644 ./90-libinput-curve.conf \
    /etc/X11/xorg.conf.d/90-libinput-curve.conf
```

The renderer requires USB vendor/product IDs and rejects duplicate hardware
matches. Generated files use `MatchDriver`, `MatchIsPointer`, `MatchUSBID`,
optional `MatchProduct`, `AccelProfile`, and the corresponding custom curve
options. They take effect when Xorg creates the device, normally at the next
login or server restart.

## Related projects

`libinput-curve` complements rather than replaces:

- [libinput-custom-points-gen](https://github.com/Gnarus-G/libinput-custom-points-gen),
  which generates points for one family of formulas;
- [rawaccel_convert](https://github.com/Kuuuube/rawaccel_convert), which
  converts Raw Accel curve settings;
- [maccel](https://github.com/Gnarus-G/maccel), a separate kernel-module input
  stack.

This project's narrower role is declarative matching, read-only planning,
safe live Xorg application, verification, hotplug convergence, and persistent
Xorg rendering for arbitrary already-sampled curves.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). Do not file input captures, serial
numbers, usernames, or private device inventories without sanitizing them.

## License

Licensed under either Apache License 2.0 or the MIT license, at your option.
See [LICENSE](LICENSE).
