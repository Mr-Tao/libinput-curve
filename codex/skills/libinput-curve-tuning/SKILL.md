---
name: libinput-curve-tuning
description: Diagnose, design, validate, apply, and persist custom libinput fallback, pointer-motion, or scroll acceleration curves. Use when Codex is asked to change mouse or touchpad feel, reproduce a Windows or Raw Accel curve, tune wheel acceleration, explain XInput custom properties, make profiles survive hotplug/login, or distinguish libinput behavior from toolkit/application scrolling under Xorg or Wayland.
---

# Libinput Curve Tuning

Use `libinput-curve` as the evidence and application layer. Keep device
matching, physical calibration, live Xorg state, persistent Xorg state, and
application scrolling as separate concerns.

## Establish the backend

1. Read `XDG_SESSION_TYPE`, `DISPLAY`, installed libinput and
   `xf86-input-libinput` versions, and the actual device inventory.
2. Under Xorg, use XInput properties and `xorg.conf.d`.
3. Under Wayland, identify the compositor's native configuration API. Do not
   claim that `xinput` or a second libinput context changes native Wayland
   input.
4. Read [references/diagnostic-workflow.md](references/diagnostic-workflow.md)
   when device matching, wheel layers, or physical scaling is involved.

## Capture state first

Run read-only commands:

```console
libinput-curve devices --format json
libinput-curve status --format json
libinput-curve plan --format json
```

Preserve the plan before applying it: each operation contains both current and
desired property values. Also inspect the exact target with
`xinput list-props ID` when diagnosing a property outside the tool's scope.

Do not infer device CPI, polling rate, display PPI, or high-resolution wheel
support from a product family. Measure or read the current hardware setting.

## Design the narrow rule

- Match a specific device with both USB vendor/product IDs and a product
  substring when available.
- Use `name_regex` only for live matching that genuinely needs it. It cannot
  be rendered faithfully to Xorg.
- Keep touchpads, trackpoints, mice, and composite receiver interfaces in
  separate rules.
- Define only the movement types being changed. An omitted curve retains the
  existing configured value during live apply and uses libinput's fallback
  behavior in a generated profile.
- Keep at least two points, a positive step, and a measured reason for the
  final extrapolation slope.

Remember that curve points are output speeds, not acceleration factors. The
factor at nonzero input speed `x` is `f(x)/x`.

## Validate and preview

```console
libinput-curve validate --config PATH
libinput-curve plan --config PATH
```

Treat any ambiguous match or missing property as a blocker. An unmatched rule
is normal for a disconnected peripheral, but confirm it is expected before
applying.

For display-aware calibration, preview the multiplier explicitly:

```console
libinput-curve plan --config PATH --motion-scale FACTOR
```

Do not change curve shape and physical scaling in the same experiment unless
the user asked for both.

## Apply and verify

Only apply after the user requested a change and the plan targets the expected
device:

```console
libinput-curve apply --config PATH
```

Require the built-in readback verification. Then test:

1. slow precision movement or one wheel detent;
2. medium movement;
3. fast movement;
4. hot-unplug/replug;
5. the actual application where the problem was noticed.

Change one dimension at a time. For scroll complaints, compare a browser and a
GTK file manager before changing the curve again; toolkit quantization can be
the limiting layer.

## Persist

Generate persistent configuration into a reviewable file:

```console
libinput-curve render-xorg --config PATH --output ./90-libinput-curve.conf
```

Diff it against existing `xorg.conf.d` rules. Back up the exact old file,
install the new file atomically, and explain that it takes effect when Xorg
recreates the device.

Use one watcher owner. A desktop autostart may start a systemd user service,
but must not also run `libinput-curve watch` directly.

## Roll back

Live XInput state normally resets when the device or X server is recreated.
For immediate rollback, use the `current` values captured in the JSON plan,
or reapply the previous known configuration. For persistent rollback, restore
the backed-up `xorg.conf.d` file and recreate the device/session.

Never delete an existing persistent rule before preserving it.
