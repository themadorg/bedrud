# bedrud-gui

GTK4 + libadwaita UI. `make build` compiles **bedrud-host** and embeds it in the GUI. At runtime the CLI is unpacked to `~/.cache/bedrud-gui/bedrud-host` (the encryption key trailer is kept). Override with `BEDRUD_HOST_BIN` if needed.

## Needs

- GTK 4, libadwaita, pkg-config, C compiler (cgo)

## Run

```bash
make test
make run
make appimage   # dist/Bedrud_Host-x86_64.AppImage (bundles GTK 4 + libadwaita + bedrud-host)
```

The AppImage is standalone: no `BEDRUD_HOST_BIN`, no separate CLI install. Host is embedded in the GUI and unpacked to `~/.cache/bedrud-gui/` on first run.

Flow:

1. **Initialize** — credentials form (required before anything else).
2. **+** in the header — create a host (background; toast + details when done).
3. **Cards** — view, admin, open in browser, delete (Adwaita destructive confirm).
4. **Gear (top left)** — settings, including delete local database.
5. **System tray** — polybar/i3 XEmbed, or StatusNotifier elsewhere. Close hides to tray.

No API tokens live in this source tree. Example DNS names in docs use `example.com`.
