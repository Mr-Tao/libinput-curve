# Repository Guidance

- Keep the tool read-only unless the user explicitly runs `apply`.
- Never invoke commands through a shell; pass arguments directly.
- Treat Wayland compositor configuration as unsupported unless a real,
  compositor-owned backend exists.
- Keep config parsing and planning testable without an X server.
- Preserve stable machine-readable output and deterministic Xorg rendering.
- Use SPDX identifier `Apache-2.0 OR MIT` in source files.
