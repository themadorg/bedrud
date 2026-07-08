# `bedrud version`

Print the Bedrud binary version (set at build time via `-ldflags`).

**Source:** `server/internal/cli/root.go`

---

## Usage

```bash
bedrud version
bedrud --json version
bedrud --version
bedrud -v
bedrud --json --version
```

---

## Text output

```
bedrud 1.2.3
```

---

## JSON output

```json
{
  "ok": true,
  "data": {
    "name": "bedrud",
    "version": "1.2.3"
  }
}
```