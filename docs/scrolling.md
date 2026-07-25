<!-- SPDX-License-Identifier: Apache-2.0 OR MIT -->

# Wheel acceleration, resolution, and visual smoothness

Three independent mechanisms determine how wheel scrolling feels:

1. **Event resolution** is the physical information emitted by the device.
2. **Acceleration** maps input speed to output speed. This is the layer changed
   by a libinput custom scroll curve.
3. **Visual interpolation** animates an output delta over multiple frames. It
   belongs to the toolkit or application.

Changing one layer cannot synthesize information or behavior owned by another
layer. In particular, a custom curve can make slow detents smaller and fast
detents larger, but it cannot turn one hardware event into a timed stream of
touchpad-like events.

## Measure the event stream

Do not infer high-resolution behavior from the existence of
`REL_WHEEL_HI_RES` alone. Capture the device without grabbing it:

```console
$ sudo evtest /dev/input/eventX
```

For the high-resolution axis, `120` represents one logical detent. Values such
as `40` or `60` show useful sub-detent resolution. A device that repeatedly
emits `REL_WHEEL 1` together with `REL_WHEEL_HI_RES 120` still supplies one
whole step per detent, despite using the modern event code.

Under Xorg, inspect the server-side scroll classes and events as a separate
step:

```console
$ xinput test-xi2 --root DEVICE_ID
```

An XI2 smooth-scroll valuator means the protocol can carry fractional deltas.
It does not promise that the device produces fractions or that an application
will animate them.

## Interpret the XInput properties

- `libinput High Resolution Wheel Scroll Enabled` selects whether
  `xf86-input-libinput` forwards high-resolution wheel events. It is not a
  hardware-resolution detector.
- `libinput Accel Custom Scroll Points` and `Step` define the speed mapping.
  They affect output magnitude, not the number or timing of physical samples.
- `libinput Scrolling Pixel Distance` applies to continuous two-finger, edge,
  or button scrolling. It does not set the size of a physical wheel detent.

Disabling high-resolution forwarding usually discards information and is not
a visual-smoothing mechanism.

## Explain application differences

A touchpad naturally emits many small deltas over time, so direct adjustment
updates can look continuous. A low-resolution wheel may provide only one delta
per detent.

GTK calls these fractional-capable events "smooth" events, but that describes
their representation rather than frame-by-frame animation. In the GTK 3
`GtkScrolledWindow` path, ordinary wheel deltas are applied directly to the
scroll adjustment. Thunar delegates ordinary vertical scrolling to that GTK
path. Its visible movement can therefore remain stepped even when the same
device feels smooth in a browser.

Firefox has its own mouse-wheel smooth-scroll durations and animation
machinery. Enabling its XInput2 path can affect which events Firefox receives,
but the browser's interpolation is what makes a whole detent move over
multiple frames. That behavior does not transfer to GTK applications through
an environment variable.

## Choose the right fix

- Tune the custom scroll curve when the distance or acceleration is wrong.
- Fix firmware or device mode when useful high-resolution events exist but are
  not emitted.
- Fix the toolkit or application when event magnitudes are right but visual
  movement is abrupt.

An input-remapping daemon could split detents into synthetic events, but it
would add latency, device matching, wake/hotplug, and double-processing risks.
It is not a safe generic substitute for application-side interpolation.

## References

- [libinput high-resolution wheel API](https://wayland.freedesktop.org/libinput/doc/latest/wheel-api.html)
- [libinput scrolling behavior](https://wayland.freedesktop.org/libinput/doc/latest/scrolling.html)
- [Linux input event codes](https://docs.kernel.org/input/event-codes.html)
- [GTK 3 `GtkScrolledWindow`](https://docs.gtk.org/gtk3/class.ScrolledWindow.html)
- [GTK issue 702: interpolate low-resolution mouse-wheel scrolling](https://gitlab.gnome.org/GNOME/gtk/-/issues/702)
- [Firefox smooth-scroll preferences](https://searchfox.org/firefox-main/source/modules/libpref/init/StaticPrefList.yaml)
