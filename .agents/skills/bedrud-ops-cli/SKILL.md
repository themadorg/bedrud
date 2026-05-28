---
name: bedrud-ops-cli
description: Operational tooling — Debian installer, CLI user management, utilities, TLS cert generation.
license: Apache License
---

# Bedrud Ops & CLI

Go module `bedrud`. `internal/install/` + `internal/usercli/` + `internal/utils/`.

---

## `internal/install/` — Installer Package

6 files:

| File | Purpose |
|------|---------|
| `linux.go` | Interactive Debian installer: copy binary, write configs, create systemd/OpenRC units, ACME/self-signed TLS, external LK support |
| `openrc.go` | OpenRC init script template (Alpine) |
| `sysv.go` | SysV init script template |
| `config.go` | Config file templates |
| `init.go` | Init system detection (systemd vs OpenRC vs SysV) |
| `secrets.go` | Secret generation for config |

`DebianInstall(...)` — Writes `/etc/bedrud/config.yaml` + LK yaml, creates service units.
`DebianUninstall()` — Stop services, remove units, binaries, configs, data dirs.

---

## `internal/usercli/usercli.go` — CLI User Management

| Fn | Purpose |
|----|---------|
| `PromoteUser(configPath, email)` | Add `superadmin` to Accesses |
| `DemoteUser(configPath, email)` | Remove `superadmin` |
| `CreateUser(configPath, email, password, name)` | bcrypt hash, insert |
| `DeleteUser(configPath, email)` | Full cleanup cascade: rooms → LiveKit → chat uploads → passkeys → preferences → user |

`withUser(configPath, email, fn)` — Load config + init DB + lookup user → call fn.

---

## `internal/utils/` — Utilities

### `net.go`

`OutboundIP() net.IP` — detect outbound IP via UDP dial to 8.8.8.8:80.
`DisplayAddr(host, port string) string` — format address for display.

### `keys.go`

`GenerateAPIKey(length int) string` — crypto-random hex string.
`GenerateSecret(length int) string` — crypto-random alphanumeric.

### `safeio.go`

`SafeOpenAppend(path string, perm os.FileMode) (*os.File, error)` — atomic file append with create-if-missing.

### `tls.go`

- `const CertWarnDays = 30` — days before expiry to warn and auto-renew
- `const SelfSignedCertDays = 1825` — self-signed cert validity (~5 years)
- `KeyAlgorithm` enum: `KeyEd25519` (default), `KeyECDSA256`, `KeyRSA2048`, `KeyRSA4096`
- `GenerateSelfSignedCert(certFile, keyFile, hosts...)` — Ed25519, PKCS8, DigitalSignature only
- `GenerateSelfSignedCertWithAlgo(certFile, keyFile, algo, hosts...)` — explicit algo
- `RenewSelfSignedCert(certFile, keyFile, hosts...)` — reads algo from existing cert, preserves it
- `RenewSelfSignedCertWithAlgo(certFile, keyFile, algo, hosts...)` — explicit algo override
- `detectCertAlgorithm(certFile)` — PEM-decode + parse x509 → maps to KeyAlgorithm
- `keyUsageForAlgo(algo)` — RSA → DigitalSignature | KeyEncipherment. Ed25519/ECDSA → DigitalSignature only
- `generateKey(algo)` — ed25519/ecdsa/rsa dispatch. All return `crypto.Signer`
- `ValidateTLSCertPair(certFile, keyFile)` — reads, decodes PEM, parses x509, checks expiry, verifies key match. Returns `(*CertInfo, error)`
- `CertInfo` — Subject, Issuer, NotBefore, NotAfter, DaysRemaining, SANs, Status
