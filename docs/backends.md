<!-- SPDX-License-Identifier: Apache-2.0 OR MIT -->

# Backend model

## libinput ownership

libinput does not provide a global settings daemon. Configuration methods
change a device in the caller's libinput context. A short-lived utility that
opens its own context cannot reconfigure the context already owned by a
Wayland compositor.

The upstream custom acceleration API was introduced in libinput 1.23. A
custom configuration combines each movement type's step and sampled points
before applying it to the device:

- [libinput pointer acceleration](https://wayland.freedesktop.org/libinput/doc/latest/pointer-acceleration.html)
- [libinput device configuration API](https://wayland.freedesktop.org/libinput/doc/latest/api/group__config.html)
- [libinput configuration ownership](https://wayland.freedesktop.org/libinput/doc/latest/configuration.html)

## Xorg

`xf86-input-libinput` owns the libinput context and exposes supported settings
as XInput properties. It also parses equivalent `xorg.conf.d` options.

`libinput-curve` uses only these public interfaces:

- `xinput list --short`;
- `xinput list-props ID`;
- `xinput set-prop ID PROPERTY VALUES...` during explicit apply/watch;
- generated `InputClass` configuration.

No input event is intercepted and no `/dev/input` node is opened.

Runtime XInput property updates are not atomic because points, step, and
profile selection are separate properties. The tool first validates every
required property, writes curve points and steps, enables the custom profile
last, and then re-reads the complete state. A hot-unplug during the write can
still cause partial state and a failed verification. Inventory collection is
retried briefly when a hotplug makes the XInput device list unstable, and an
advisory per-user lock serializes mutating commands.

## Wayland

Native Wayland support requires a backend implemented through the compositor
that owns the input context. Generic `xinput` calls affect only Xwayland and do
not change native application input. The live CLI rejects
`XDG_SESSION_TYPE=wayland` unless the diagnostic `--allow-xwayland` flag is
provided.

Do not implement a backend by opening a second libinput context and claiming
success. That context does not own the events delivered by the compositor.
