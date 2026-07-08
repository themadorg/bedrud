# `bedrud uninstall`

Remove Bedrud from the system: stop services, delete units, binaries, and config under `/etc/bedrud`.

**Source:** `server/internal/cli/install.go` → `internal/install.LinuxUninstall()`

---

## Usage

```bash
sudo bedrud uninstall
sudo bedrud --json uninstall
```

---

## JSON output

```json
{
  "ok": true,
  "message": "✓ Bedrud uninstalled successfully",
  "data": {
    "uninstalled": true
  }
}
```

---

## Related

- [install.md](./install.md)