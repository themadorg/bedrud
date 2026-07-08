# File Storage (`internal/storage`)

Chat image uploads and recording file storage.

---

## Chat Uploads (`chat_upload.go`)

### ChatUploadStore interface

```go
type ChatUploadStore interface {
    Store(data []byte) (*ChatAttachment, error)
}
```

### Backends

| Backend | Behavior |
|---------|----------|
| `disk` (default) | Files under `chat.uploads.diskDir` (default `./data/uploads/chat`) |
| `inline` | Always returns base64 data URLs |
| `hybrid` | Inline for small files, disk for larger |
| `s3` | Raw AWS SigV4 upload (no SDK dependency) |

Factory: `NewChatUploadStore(cfg *config.ChatUploadConfig)`

### ChatAttachment

```go
type ChatAttachment struct {
    URL    string
    MIME   string
    Size   int
    Width  int
    Height int
}
```

### Validation

- MIME: png, jpeg, gif, webp only
- SHA-256 content hash used as filename
- Max size from `chat.uploads.maxBytes` (default 10 MB)
- Inline threshold: `chat.uploads.inlineMaxBytes` (default 500 KB)

### ChatUploadTracker

Tracks disk-backed uploads in the `chat_uploads` DB table.

| Method | Purpose |
|--------|---------|
| `Record(roomID, fileHash, extension)` | Insert tracking row |
| `DeleteByRoom(roomID)` | Delete files + DB rows for a room |
| `GetUserUsage(userID)` | Total bytes per user |
| `GetGlobalUsage()` | Total bytes across all users |

Quotas enforced by handlers:
- Per-user: `chat.maxUploadBytesPerUser` (default 500 MB)
- Global: `chat.globalDiskThresholdBytes`

S3 and inline uploads are not tracked.

### ObjectDeleter / S3Deleter

```go
type ObjectDeleter interface {
    DeleteObject(key string) error
}
```

`NewS3Deleter(cfg)` — raw AWS SigV4 delete for S3-compatible backends. Wired into `ChatUploadTracker` when `chat.uploads.backend == "s3"` so room/user cleanup deletes remote objects too.

---

## Recording Storage (`recording_store.go`)

### RecordingStore

Disk-backed storage for recording files.

| Method | Purpose |
|--------|---------|
| `Store(roomID, egressID, data)` | Write recording file |
| `GetPath(recordingID)` | Resolve file path |
| `Delete(recordingID)` | Remove file from disk |
| `GetSize(recordingID)` | File size in bytes |

Storage directory: `recording.storageDir` (default `./data/recordings`).

Retention controlled by `recording.retentionHours` (default 720 = 30 days). Scheduler runs cleanup at `recording.cleanupIntervalHours` (default 24).