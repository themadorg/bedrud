# Utilities (`internal/utils`)

Standalone helper functions with no application dependencies.

---

## TLS (`tls.go`)

Self-signed certificate generation, renewal, and validation.

### Constants

| Constant | Value | Purpose |
|----------|-------|---------|
| `CertWarnDays` | 30 | Days before expiry to warn and auto-renew |
| `SelfSignedCertDays` | 1825 | Self-signed validity (~5 years) |

### Key algorithms

```go
type KeyAlgorithm string

const (
    KeyEd25519  KeyAlgorithm = "ed25519"   // default
    KeyECDSA256 KeyAlgorithm = "ecdsa256"
    KeyRSA2048  KeyAlgorithm = "rsa2048"
    KeyRSA4096  KeyAlgorithm = "rsa4096"
)
```

### Functions

| Function | Purpose |
|----------|---------|
| `GenerateSelfSignedCert(certFile, keyFile, hosts...)` | Generate Ed25519 cert (default) |
| `GenerateSelfSignedCertWithAlgo(certFile, keyFile, algo, hosts...)` | Explicit algorithm |
| `RenewSelfSignedCert(certFile, keyFile, hosts...)` | Auto-detect existing algo, renew |
| `RenewSelfSignedCertWithAlgo(certFile, keyFile, algo, hosts...)` | Explicit algo override |
| `ValidateTLSCertPair(certFile, keyFile)` | Parse, check expiry, verify key match |
| `detectCertAlgorithm(certFile)` | Read existing cert's public key type |

### CertInfo

```go
type CertInfo struct {
    Subject       string
    Issuer        string
    NotBefore     time.Time
    NotAfter      time.Time
    DaysRemaining int
    SANs          []string
    Status        string
}
```

Default SANs: `localhost`, `127.0.0.1`, `::1`. CN: `localhost`. Org: "Bedrud Open Source".

Renewal uses atomic `.new` temp files + `os.Rename`.

---

## Email (`email.go`)

SMTP send helper used by queue email handler and auth verification.

Supports:
- STARTTLS (default, port 587)
- SMTPS direct TLS (port 465, `email.smtpsMode`)
- TLS skip verify (`email.tlsSkipVerify`)

---

## Keys (`keys.go`)

Cryptographic key and secret generation for installer and CLI.

---

## Network (`net.go`)

Network utilities including outbound IP detection (used by LiveKit `ResolveNodeIP`).

---

## Safe I/O (`safeio.go`)

Safe file read/write with size limits to prevent unbounded memory usage.

| Function | Purpose |
|----------|---------|
| `ReadFileLimited(path, maxBytes)` | Read file with size cap |
| `WriteFileAtomic(path, data)` | Atomic write via temp + rename |