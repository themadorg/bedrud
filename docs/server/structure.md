# Server Directory Structure

Complete map of the `server/` directory. Paths are relative to `server/`.

> **Full monorepo tree:** [docs/project-tree.md](../project-tree.md) includes `server/`, `apps/`, `packages/`, `agents/`, and more.

**Public docs:** [Code Structure](https://bedrud.org/en/docs/backend/structure) · [Server Architecture](https://bedrud.org/en/docs/architecture/server) — [`backend/structure.mdx`](../../apps/site/src/content/docs/en/backend/structure.mdx). Full map: [public-docs.md](./public-docs.md).

```
server/
├── cmd/                          # Application entrypoints
│   ├── bedrud/main.go            # Production CLI binary (cobra)
│   └── server/main.go            # Dev API server (Air hot-reload target)
│
├── config/                       # Configuration package
│   ├── config.go                 # Config struct, Load(), env overrides
│   ├── config_test.go
│   └── configint_test.go
│
├── internal/                     # Private application code
│   ├── auth/                     # JWT, OAuth, passkeys, email verification
│   ├── cli/                      # Cobra CLI commands (run, install, user, …)
│   ├── database/                 # GORM init, migrations
│   ├── handlers/                 # HTTP route handlers
│   ├── install/                # OS installer (systemd, OpenRC, SysV)
│   ├── livekit/                  # Embedded LiveKit binary management
│   │   └── bin/                  # livekit-server placeholder (CI: touch empty file)
│   ├── lkutil/                   # Shared LiveKit client helpers
│   ├── middleware/               # Auth, rate limiting, recordings gate
│   ├── models/                   # GORM database models
│   ├── queue/                    # Async job queue worker + handlers
│   │   └── templates/            # Email HTML templates (Cerberus)
│   ├── repository/               # Data access layer
│   ├── roomcli/                  # CLI room management helpers
│   ├── scheduler/                # Background tasks (idle rooms, cleanup)
│   ├── server/                   # Production bootstrap (Run())
│   ├── services/                 # Room cleanup, recording service
│   ├── storage/                  # Chat uploads, recording file storage
│   ├── templates/                # Legacy HTML (login.html, index.html — pre-React)
│   │   ├── login.html
│   │   └── index.html
│   ├── testutil/                 # Test DB setup, LiveKit mocks
│   ├── usercli/                  # CLI user management helpers
│   └── utils/                    # TLS certs, email, keys, safe I/O
│
├── docs/                         # Generated Swagger/OpenAPI (swag)
│   ├── docs.go
│   ├── swagger.json
│   └── swagger.yaml
│
├── frontend/                     # Embedded React build output (generated)
│   ├── index.html                # SPA entry
│   ├── shell.html                # Non-API route shell
│   ├── assets/                   # JS/CSS bundles
│   └── …                         # favicon, manifest, robots.txt
│
├── dist/                         # Mage build output (binary + config)
│
├── ui.go                         # //go:embed all:frontend → embed.FS
├── magefile.go                   # Mage build tasks (swagger, dist)
├── go.mod / go.sum               # Go module definition
├── README.md                     # Library references (minimal)
│
├── config.local.yaml.example     # Dev config template (copy → config.yaml)
├── config/livekit.yaml.example   # External LiveKit YAML example
├── .env.example                  # Environment variable reference
├── .air.toml                     # Air hot-reload config
├── .golangci.yml                 # Linter config
└── .swaggo                       # Swag generator config
```

---

## Top-Level Files

| File | Purpose |
|------|---------|
| `ui.go` | Embeds `frontend/` into the binary via `//go:embed all:frontend`. Populated by `make build`. |
| `magefile.go` | Mage tasks: `Build`, `Swagger`, `InstallDeps`. Builds `dist/bedrud` from `cmd/server`. |
| `go.mod` | Module `bedrud`. Go 1.24+. |
| `config.local.yaml.example` | Template for local dev. Copy to `config.local.yaml` or `config.yaml`. Never commit secrets. |

---

## `cmd/` — Entrypoints

### `cmd/bedrud/main.go`

Production binary. Delegates to `internal/cli.Execute(version)`. Supports all CLI subcommands and legacy flag-style invocations (`--run`, `--livekit`).

### `cmd/server/main.go`

Development API server. No CLI subcommands. Wires all subsystems inline, registers routes, serves Swagger/Scalar + SPA. Target for Air hot-reload (`make dev-server-hot`).

---

## `config/` — Configuration

| File | Exports |
|------|---------|
| `config.go` | `Config` struct with sections: `Server`, `Database`, `LiveKit`, `Auth`, `Logger`, `Cors`, `Chat`, `Recording`, `RateLimit`, `Queue`, `Email`. `Load(path)`, `Get()`, `GetSafe()`, `SetForTest()`. `ConfigInt` type for YAML int/string compat. |

See [Configuration](./configuration.md) for all fields and environment variables.

---

## `internal/auth/`

| File | Purpose |
|------|---------|
| `auth.go` | `AuthService` — register, login, guest, passkey, profile, password, logout |
| `jwt.go` | Token generation/validation, `Claims` struct, token pair |
| `session_store.go` | Gorilla CookieStore for Goth OAuth sessions |
| `challenge_store.go` | In-memory WebAuthn challenge store |
| `email.go` | Email verification and password reset token logic |
| `*_test.go` | Unit tests |

---

## `internal/cli/`

Cobra command tree. See [CLI](./cli.md).

| File | Commands |
|------|----------|
| `root.go` | Root command, `--config` flag, version |
| `run.go` | `bedrud run` |
| `livekit.go` | `bedrud livekit` |
| `install.go` | `bedrud install`, `bedrud uninstall` |
| `cert.go` | `bedrud cert renew`, `bedrud cert info` |
| `user.go` | `bedrud user create/delete/promote/demote/list/info/password/…` |
| `room.go` | `bedrud room list/info/close/suspend/reactivate/kick` |
| `config.go` | `bedrud config path/show/get/set/validate` |
| `settings.go` | `bedrud settings show/set/reset` |
| `invite.go` | `bedrud invite list/create/delete` |
| `db.go` | `bedrud db migrate`, `bedrud db status` |
| `legacy.go` | Backward-compat for `--run`, `--livekit`, `--version` flags |
| `runtime.go` | Shared runtime helpers for CLI commands |

---

## `internal/handlers/`

HTTP controllers. See [HTTP Handlers](./internal/handlers.md).

| File | Handler / Purpose |
|------|-------------------|
| `auth_handler.go` | Local auth, passkeys, email verification, password reset |
| `auth.go` | OAuth (Goth) begin/callback |
| `room.go` | Room CRUD, join, moderation, chat upload |
| `room_auth.go` | Room-level authorization helpers |
| `users.go` | Admin user management |
| `admin_handler.go` | System settings, invite tokens |
| `admin_overview.go` | Admin dashboard stats/widgets |
| `admin_queue.go` | Admin queue job management |
| `preferences_handler.go` | Per-user JSON preferences |
| `recording_handler.go` | Recording start/stop/list/download |
| `livekit_webhook.go` | LiveKit disconnect/webhook events |
| `cert_handler.go` | TLS certificate info endpoint |
| `cooldown.go` | Email resend cooldown tracking |
| `errors.go` | Standardized error responses |
| `models.go` | Shared response DTOs |

---

## `internal/models/`

GORM models. See [Database Models](./internal/models.md).

| File | Model |
|------|-------|
| `user.go` | `User`, `AccessLevel`, `StringArray` |
| `room.go` | `Room`, `RoomSettings`, `RoomParticipant`, `RoomPermissions` |
| `passkey.go` | `Passkey` |
| `refresh.go` | `BlockedRefreshToken` |
| `settings.go` | `SystemSettings` |
| `invite_token.go` | `InviteToken` |
| `user_preferences.go` | `UserPreferences` |
| `chat_upload.go` | `ChatUpload` |
| `job.go` | `Job`, `JobStatus` |
| `recording.go` | `Recording` |
| `webhook.go` | `Webhook`, `WebhookDelivery` |
| `verification_event.go` | `VerificationEvent` |
| `stats.go` | Dashboard stat DTOs |
| `queue_stats.go` | Queue statistics DTOs |

---

## `internal/repository/`

Data access. See [Repositories](./internal/repository.md).

| File | Repository |
|------|------------|
| `user_repository.go` | `UserRepository` |
| `room_repository.go` | `RoomRepository` |
| `passkey_repository.go` | `PasskeyRepository` |
| `settings_repository.go` | `SettingsRepository` |
| `invite_token_repository.go` | `InviteTokenRepository` |
| `user_preferences_repository.go` | `UserPreferencesRepository` |
| `recording_repository.go` | `RecordingRepository` |
| `webhook_repository.go` | `WebhookRepository` |
| `verification_event_repository.go` | `VerificationEventRepository` |

---

## `internal/queue/`

Async job queue. See [Job Queue](./internal/queue.md).

| File | Purpose |
|------|---------|
| `queue.go` | `Enqueue()`, `Worker`, `Handler` type |
| `worker.go` | Poll loop, claim, retry, backoff |
| `job.go` | Payload structs for all job types |
| `handler_user_delete.go` | `user_delete` job |
| `handler_room_delete.go` | `room_delete` job |
| `handler_room_suspend.go` | `room_suspend` job |
| `handler_chat_upload.go` | `chat_upload_s3` job |
| `handler_email.go` | `send_email` job |
| `handler_dispatch_webhook.go` | `dispatch_webhook` job |
| `handler_process_recording.go` | `process_recording` job |
| `handler_recording_delete.go` | `recording_delete` job |
| `handler_stubs.go` | Legacy stub handlers |

---

## `internal/livekit/`

| File | Purpose |
|------|---------|
| `embed.go` | `//go:embed bin/livekit-server` (build tag `!windows`) |
| `embed_windows.go` | Windows stub (no embedded binary) |
| `config.go` | `ConfigYAML` struct for LiveKit YAML generation |
| `server.go` | `ExportBinary`, `StartInternalServer`, `RunLiveKit`, TURN/TLS config |

---

## `internal/install/`

| File | Purpose |
|------|---------|
| `linux.go` | Debian/Ubuntu systemd installer |
| `openrc.go` | OpenRC init installer |
| `sysv.go` | SysV init installer |
| `init.go` | Init system detection and dispatch |
| `config.go` | Installer config generation |
| `secrets.go` | Secret generation during install |

---

## `internal/storage/`

| File | Purpose |
|------|---------|
| `chat_upload.go` | `ChatUploadStore` (disk/inline/s3/hybrid), `ChatUploadTracker` |
| `recording_store.go` | `RecordingStore` for recording file I/O |

---

## `internal/services/`

| File | Purpose |
|------|---------|
| `room_cleanup.go` | `RoomCleanupService` — cascade delete, suspend |
| `recording_service.go` | `RecordingService` — egress, processing |

---

## `internal/utils/`

| File | Purpose |
|------|---------|
| `tls.go` | Self-signed cert generation/renewal (Ed25519, ECDSA, RSA) |
| `email.go` | SMTP send helper |
| `keys.go` | Cryptographic key generation |
| `net.go` | Network utilities |
| `safeio.go` | Safe file read/write with size limits |

---

## `internal/templates/` (legacy)

Pre-React server-rendered HTML (`login.html`, `index.html`). Superseded by embedded React SPA. Not served in current bootstrap.

## `internal/queue/templates/`

Cerberus HTML + plain-text email templates. See [Job Queue](./internal/queue.md).

## Database migrations

No separate `migrations/` SQL folder. Schema managed by GORM `AutoMigrate` in `internal/database/migrations.go`. Run via startup or `bedrud db migrate`.

## `docs/` (within server)

Generated Swagger/OpenAPI files. Regenerate with `make swagger-gen` or `mage Swagger`.

| File | Purpose |
|------|---------|
| `docs.go` | Go package init for swag |
| `swagger.json` | OpenAPI 2.0 JSON |
| `swagger.yaml` | OpenAPI 2.0 YAML |

---

## `frontend/` (generated)

Built React assets from `apps/web/`. **Do not edit manually.** Populated by `make build`:

```
apps/web/build/* → server/frontend/ → //go:embed in ui.go
```

Contains `index.html` (SPA), `shell.html` (non-API fallback), hashed JS/CSS in `assets/`.

---

## Dependency Graph

```
cmd/bedrud → cli → server.Run()
cmd/server → handlers → repository → models
                    → auth → repository
                    → lkutil
                    → middleware → auth
                    → scheduler → repository + livekit + database
                    → services → repository + lkutil + storage
                    → queue → database + models + services
                    → storage (standalone)
                    → install → utils
                    → usercli / roomcli → repository + lkutil + services
                    → database → models (migrations)
```