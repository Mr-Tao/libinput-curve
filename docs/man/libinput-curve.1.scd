libinput-curve(1)
; SPDX-License-Identifier: Apache-2.0 OR MIT

# NAME

libinput-curve - plan, apply, and persist custom libinput acceleration curves

# SYNOPSIS

*libinput-curve* *validate* [*--config* _path_]

*libinput-curve* *devices* [*--format* _human|json_] [*--xinput* _command_] [*--allow-xwayland*]

*libinput-curve* _plan|status|apply_ [*--config* _path_] [*--format* _human|json_] [*--xinput* _command_] [*--motion-scale* _factor_] [*--scroll-scale* _factor_] [*--allow-xwayland*]

*libinput-curve* *watch* [*--config* _path_] [*--format* _human|json_] [*--xinput* _command_] [*--motion-scale* _factor_] [*--scroll-scale* _factor_] [*--allow-xwayland*] [*--interval* _duration_]

*libinput-curve* *render-xorg* [*--config* _path_] [*--motion-scale* _factor_] [*--scroll-scale* _factor_] [*--output* _path|-_]

*libinput-curve* *completion* _bash|zsh|fish_

*libinput-curve* *version*

# DESCRIPTION

*libinput-curve* validates declarative custom libinput curves, matches them to
Xorg pointer devices, previews the exact XInput property changes, applies
preflighted changes, and verifies the resulting state. It can also render
deterministic xorg.conf.d InputClass sections for persistent configuration.

Live commands use the XInput properties exposed by xf86-input-libinput. Under
Wayland, the compositor owns the native libinput context, so XInput can affect
only Xwayland devices. Live commands therefore reject Wayland sessions unless
*--allow-xwayland* is explicitly supplied.

No command changes device state except *apply* and *watch*. The *plan*,
*status*, and *devices* commands are read-only. *render-xorg* writes only its
requested output file.

# COMMANDS

*validate*
	Strictly decode and validate the configuration. Print its schema and the
	number of profiles and device rules.

*devices*
	List Xorg pointer devices that expose libinput acceleration profiles,
	including whether a custom profile is available and enabled.

*plan*
	Show the property changes required to bring connected matched devices into
	the configured state. Do not change device state.

*status*
	Show the same plan as *plan*, but return status 1 when drift or unmatched
	rules are present.

*apply*
	Preflight the complete plan, apply only drifted properties, read all
	properties back, and fail if verification finds remaining drift.

*watch*
	Poll for XInput hotplug, reload the configuration, and converge matched
	devices. Output is emitted only when the resulting state changes. A
	process-wide advisory lock prevents concurrent *apply* and *watch*
	writers.

*render-xorg*
	Render xorg.conf.d InputClass sections. Device rules must contain USB
	vendor and product IDs. Rules using only a regular expression cannot be
	translated and are rejected.

*completion*
	Write a completion definition for Bash, Zsh, or Fish to standard output.

*version*
	Print the build version.

# OPTIONS

*--config* _path_
	Read configuration from _path_. The default is
	_$XDG_CONFIG_HOME/libinput-curve/config.json_, or
	_$HOME/.config/libinput-curve/config.json_ when XDG_CONFIG_HOME is unset.
	If neither base directory is available, use _config.json_ in the current
	directory.

*--format* _human|json_
	Select human-readable or JSON output. The default is _human_.

*--xinput* _command_
	Use _command_ as the xinput executable. The default is _xinput_. The
	executable is invoked directly without a shell.

*--motion-scale* _factor_
	Multiply fallback and motion curve output points by a finite factor greater
	than zero. The default is 1.

*--scroll-scale* _factor_
	Multiply scroll curve output points by a finite factor greater than zero.
	The default is 1.

*--allow-xwayland*
	Allow a live command in a Wayland session. This affects only Xwayland
	devices and does not configure the compositor-owned native input path.

*--interval* _duration_
	Set the *watch* polling interval using Go duration syntax, for example
	_500ms_ or _2s_. Values below 250ms are rejected. The default is 2s.

*--output* _path|-_
	Write rendered Xorg configuration atomically to _path_. Use _-_ for
	standard output, which is the default.

*-h*, *--help*
	Show top-level help, or help for commands that accept options.

# CONFIGURATION

The configuration is strict JSON with schema
_io.github.mr-tao.libinput-curve/v1_. It defines named profiles and device
rules. Unknown fields, duplicate or unsafe rule IDs, incomplete USB IDs,
non-finite values, invalid curve lengths, and ambiguous matches are rejected.

Curve points are output speeds sampled at the configured input-speed step;
they are not acceleration factors. Device CPI, display PPI, wheel resolution,
and application-side scrolling behavior remain separate concerns.

# ENVIRONMENT

*XDG_CONFIG_HOME*
	Overrides the base directory for the default configuration path.

*HOME*
	Provides the default configuration base when XDG_CONFIG_HOME is unset.

*XDG_RUNTIME_DIR*
	Contains the advisory mutation lock. If unset, a per-user lock is placed
	in the system temporary directory.

*XDG_SESSION_TYPE*
	Used to reject unsupported native Wayland configuration by default.

# EXIT STATUS

*0*
	The command completed successfully. For *status*, all present matched
	devices are in sync and no rules are unmatched.

*1*
	*status* found drift or unmatched rules.

*2*
	A runtime operation, preflight, application, or verification failed.

*64*
	Command-line usage was invalid.

*65*
	Configuration was invalid or could not be decoded.

*69*
	The requested live backend was unavailable.

*74*
	Output could not be written.

# FILES

_$XDG_CONFIG_HOME/libinput-curve/config.json_
	Default configuration file.

_$XDG_RUNTIME_DIR/libinput-curve.lock_
	Advisory lock used by *apply* and *watch*.

# EXAMPLES

Inspect and apply a configuration:

```
libinput-curve validate
libinput-curve devices
libinput-curve plan
libinput-curve apply
```

Generate persistent Xorg configuration:

```
libinput-curve render-xorg --output 90-libinput-curve.conf
```

Install Fish completion for the current user:

```
libinput-curve completion fish >~/.config/fish/completions/libinput-curve.fish
```

# SEE ALSO

*xinput*(1), *Xorg*(1), *systemd.service*(5)
