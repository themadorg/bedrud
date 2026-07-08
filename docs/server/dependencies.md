# Go Dependencies

Module: `bedrud`  
Go version: **1.26** (`server/go.mod`)

---

## Direct dependencies

| Package | Version | Purpose |
|---------|---------|---------|
| `github.com/gofiber/fiber/v2` | v2.52.13 | HTTP framework |
| `github.com/gofiber/swagger` | v1.1.1 | Swagger UI middleware |
| `github.com/swaggo/swag` | v1.16.6 | OpenAPI code generation |
| `gorm.io/gorm` | v1.31.1 | ORM |
| `gorm.io/driver/sqlite` | v1.6.0 | SQLite driver |
| `gorm.io/driver/postgres` | v1.6.0 | PostgreSQL driver |
| `github.com/golang-jwt/jwt/v5` | v5.3.1 | JWT tokens |
| `github.com/go-passkeys/go-passkeys` | v0.4.1 | WebAuthn / FIDO2 |
| `github.com/markbates/goth` | v1.82.0 | OAuth2 (Google, GitHub, Twitter) |
| `github.com/gorilla/sessions` | v1.4.0 | OAuth session store |
| `github.com/livekit/protocol` | v1.46.0 | LiveKit RoomService client |
| `github.com/twitchtv/twirp` | v8.1.3 | Twirp RPC (LiveKit) |
| `github.com/rs/zerolog` | v1.35.1 | Structured logging |
| `github.com/spf13/cobra` | v1.10.2 | CLI framework |
| `github.com/spf13/viper` | v1.21.0 | CLI config/env binding |
| `github.com/go-co-op/gocron` | v1.37.0 | Background scheduler |
| `github.com/magefile/mage` | v1.17.2 | Build tasks |
| `github.com/google/uuid` | v1.6.0 | UUID generation |
| `golang.org/x/crypto` | v0.52.0 | bcrypt, TLS, ACME |
| `golang.org/x/net` | v0.54.0 | Network utilities |
| `golang.org/x/text` | v0.37.0 | Text processing |
| `google.golang.org/protobuf` | v1.36.11 | Protobuf (LiveKit) |
| `gopkg.in/yaml.v3` | v3.0.1 | Config YAML parsing |

---

## Embedded binaries (not Go modules)

| Binary | Location | Purpose |
|--------|----------|---------|
| `livekit-server` | `internal/livekit/bin/` | Embedded media server (`//go:embed`) |
| React SPA | `frontend/` | Embedded web UI (`ui.go`) |

---

## Dev tooling (not in go.mod)

| Tool | Config file | Purpose |
|------|-------------|---------|
| Air | `.air.toml` | Hot-reload for `cmd/server` |
| golangci-lint | `.golangci.yml` | Static analysis |
| swag CLI | `.swaggo` | Swagger type overrides |
| Mage | `magefile.go` | `mage Build`, `mage Swagger` |

---

## Updating dependencies

```bash
cd server
go mod tidy
go get -u ./...    # upgrade (use with care)
```