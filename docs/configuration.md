<!-- SPDX-License-Identifier: Apache-2.0 OR MIT -->

# Configuration

## Curves

Each curve contains a positive `step` and between 2 and 64 output-speed
`points`. Values must be finite and within the current
`xf86-input-libinput` range `0..10000`.

For point index `i`, the sampled coordinate is:

```text
(i * step, points[i])
```

libinput linearly interpolates between samples and extrapolates beyond the
last sample using the last segment. A final flat segment therefore caps output
speed; a rising final segment continues rising.

Profiles can define any combination of:

- `fallback`: catch-all movement not covered by a specific curve;
- `motion`: regular pointer motion;
- `scroll`: mouse or touchpad scrolling.

The custom profile uses libinput's flat fallback for movement types without a
configured curve. Keep an explicit fallback curve constant unless there is a
measured reason to do otherwise.

## Device rules

Rules are evaluated as conjunctions: every specified criterion must match.

- `product_contains`: literal case-sensitive product-name substring. It maps
  to Xorg `MatchProduct`.
- `name_regex`: Go regular expression for live XInput matching. It cannot be
  rendered to Xorg.
- `vendor_id` plus `product_id`: exactly four hexadecimal digits each. They
  must be specified together.

Every rule has a unique safe `id`, references one profile, and may define
positive `motion_scale` and `scroll_scale` factors. Omitted factors default to
`1`.

If one live device matches multiple rules, the entire plan is rejected before
any property is changed. A rule may match multiple equivalent live devices.
An absent rule is reported but is not an apply failure, allowing profiles for
occasionally disconnected peripherals.

## Runtime scale

Command-line `--motion-scale` and `--scroll-scale` values multiply rule
factors. The motion factor scales fallback and motion y values. The scroll
factor scales scroll y values. Steps are unchanged.

Scaling is rejected if a result is non-finite or exceeds the current Xorg
property range.

## Strict decoding

The v1 schema identifier is:

```text
io.github.mr-tao.libinput-curve/v1
```

The decoder accepts exactly one JSON object and rejects unknown fields and
trailing values. Byte-for-byte preservation is not promised; the schema
describes behavior rather than formatting.
