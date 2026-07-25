<!-- SPDX-License-Identifier: Apache-2.0 OR MIT -->

# systemd user service

The optional user unit runs `libinput-curve watch` only inside an X11 graphical
session:

```console
$ install -Dm0644 contrib/systemd/libinput-curve-watch.service \
    ~/.config/systemd/user/libinput-curve-watch.service
$ systemctl --user daemon-reload
$ systemctl --user enable --now libinput-curve-watch.service
```

This requires a desktop session that activates `graphical-session.target` and
imports `DISPLAY`, `XAUTHORITY`, and `XDG_SESSION_TYPE` into the user manager.
Some Xfce releases do not activate that target. In that case, leave the unit
disabled and start it from the desktop's ordinary autostart:

```text
systemctl --user start --no-block libinput-curve-watch.service
```

The service intentionally does not use `PrivateTmp=yes`: filesystem-backed
X11 sockets normally live below `/tmp/.X11-unix`. It receives no capabilities,
block-device access, writable system or home view, or network socket families
other than `AF_UNIX`.

Use:

```console
$ systemctl --user status libinput-curve-watch.service
$ journalctl --user -u libinput-curve-watch.service
```

Do not run a second watcher from desktop autostart. The autostart command
should start this unit rather than invoke `libinput-curve watch` directly.
