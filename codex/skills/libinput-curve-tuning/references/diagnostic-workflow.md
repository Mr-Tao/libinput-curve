<!-- SPDX-License-Identifier: Apache-2.0 OR MIT -->

# Diagnostic workflow

## Inventory

Collect:

```console
printf 'session=%s display=%s\n' "$XDG_SESSION_TYPE" "$DISPLAY"
libinput --version
pacman -Q libinput xf86-input-libinput xorg-xinput 2>/dev/null
libinput-curve devices --format json
xinput list --short
```

On non-Arch systems, replace the package query with the native package
manager. Query only relevant device properties after identifying the ID.

## Property map

- `libinput Accel Profiles Available`: adaptive, flat, custom support.
- `libinput Accel Profile Enabled`: adaptive, flat, custom selection.
- `libinput Accel Custom Fallback Points/Step`: catch-all function.
- `libinput Accel Custom Motion Points/Step`: pointer-motion function.
- `libinput Accel Custom Scroll Points/Step`: scroll function.
- `libinput Accel Speed`: retained but behaviorally irrelevant while the
  custom profile is active.
- `libinput High Resolution Wheel Scroll Enabled`: whether the Xorg driver
  forwards high-resolution wheel events.
- `libinput Scrolling Pixel Distance`: conversion distance for smooth
  two-finger, edge, or button scrolling; it is not the custom scroll curve.
- `libinput Natural Scrolling Enabled`: direction, not acceleration.

Solaar or device firmware may independently control wheel mode, diversion,
DPI/CPI, and high-resolution behavior.

## Curve semantics

For points `p[i]` and step `s`, sample `i` is `(i*s, p[i])`.
libinput linearly interpolates and extrapolates. Consequences:

- `[0, 1]`, step `1` is identity;
- a constant positive y value produces a fixed output speed near that region;
- a flat last segment caps output speed;
- a steep last segment keeps accelerating beyond the sampled range;
- scaling all y values changes output distance without moving input-speed
  breakpoints.

The input axis uses device units per millisecond. CPI and event timing affect
which part of the curve is exercised. Display PPI affects the physical
distance represented by an output delta.

## Scroll layers

Diagnose these independently:

1. hardware detents/free-spin and firmware;
2. libinput high-resolution event production;
3. custom scroll acceleration;
4. Xorg smooth-scroll valuators;
5. toolkit behavior;
6. application settings such as Firefox wheel overrides.

A wheel that emits only whole detents cannot acquire touchpad-like continuous
physical samples from a curve. A custom curve can reduce the first output step
and accelerate later events, while a toolkit may still render each delivered
increment discretely.

Measure rather than infer the second and fourth layers:

```console
sudo evtest /dev/input/eventX
xinput test-xi2 --root DEVICE_ID
```

On `REL_WHEEL_HI_RES`, `120` is one logical detent. Fractional values such as
`40` or `60` demonstrate useful sub-detent information. Merely advertising
the axis, enabling `libinput High Resolution Wheel Scroll Enabled`, or seeing
an XI2 smooth-scroll valuator does not prove that fractional samples exist.

Likewise, "smooth" at the event-protocol layer means fractional-capable, not
visually animated. Browsers may interpolate each wheel delta over several
frames while a GTK widget directly updates its adjustment. The environment
variable that enables Firefox's XInput2 path does not enable animation in GTK
applications.

`libinput Scrolling Pixel Distance` is not a wheel-detent-size control. It
applies to continuous two-finger, edge, or button scrolling.

See the project-level
[wheel scrolling guide](../../../../docs/scrolling.md) for the full boundary
and upstream references.

## Experiment discipline

- Capture the current JSON plan.
- Confirm one rule and one device.
- Change one curve or scale.
- Apply and require readback.
- Test slow, medium, fast, and hotplug behavior.
- Record device CPI, display PPI, relevant application, and exact curve.
- Revert before testing a different hypothesis.

Avoid global `MatchIsPointer` rules unless every pointer was measured and the
user explicitly wants identical behavior.
