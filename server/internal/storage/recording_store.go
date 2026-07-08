// TODO oncoming feature
package storage

import (
	"bedrud/config"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog/log"
)

// RecordingAttachment is the result of storing a recording file.
type RecordingAttachment struct {
	URL  string `json:"url"`
	Size int64  `json:"size"`
}

// RecordingStore handles persisting recording files (MP4/MKV).
// Implementations must be safe for concurrent use.
type RecordingStore interface {
	// Store saves a recording file from src reader.
	// key is the storage path (e.g. "recordings/{roomID}/{egressID}.mp4").
	Store(ctx context.Context, key string, src io.Reader, size int64) (*RecordingAttachment, error)
	// Delete removes a recording file by key.
	Delete(ctx context.Context, key string) error
}

// NewRecordingStore creates the appropriate backend from config.
// Falls back to disk if S3 is not fully configured.
func NewRecordingStore(cfg *config.RecordingConfig, s3Cfg config.ChatUploadS3Config) RecordingStore {
	storageDir := cfg.StorageDir
	if storageDir == "" {
		storageDir = "./data/recordings"
	}

	maxBytes := int64(cfg.MaxFileSizeMB) * 1024 * 1024
	if maxBytes <= 0 {
		maxBytes = 0 // unlimited
	}

	if s3Cfg.Endpoint != "" && s3Cfg.Bucket != "" && s3Cfg.AccessKey != "" {
		log.Info().Str("endpoint", s3Cfg.Endpoint).Str("bucket", s3Cfg.Bucket).
			Int64("maxBytes", maxBytes).Msg("recording: using S3 storage")
		return &s3RecordingStore{
			endpoint:  s3Cfg.Endpoint,
			bucket:    s3Cfg.Bucket,
			region:    s3Cfg.Region,
			accessKey: s3Cfg.AccessKey,
			secretKey: s3Cfg.SecretKey,
			publicURL: s3Cfg.PublicBaseURL,
			maxBytes:  maxBytes,
			fallback: &diskRecordingStore{
				dir:      storageDir,
				maxBytes: maxBytes,
			},
		}
	}

	log.Info().Str("dir", storageDir).Int64("maxBytes", maxBytes).Msg("recording: using disk storage")
	return &diskRecordingStore{
		dir:      storageDir,
		maxBytes: maxBytes,
	}
}

// ─── Disk backend ─────────────────────────────────────────────────────────────

type diskRecordingStore struct {
	dir      string
	maxBytes int64
}

func (s *diskRecordingStore) Store(ctx context.Context, key string, src io.Reader, size int64) (*RecordingAttachment, error) {
	// Max file size check
	if s.maxBytes > 0 {
		if size > s.maxBytes {
			return nil, fmt.Errorf("recording file size %d exceeds max %d", size, s.maxBytes)
		}
	}

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, fmt.Errorf("create recording dir: %w", err)
	}

	// Sanitize key: strip leading slash, prevent path traversal
	cleanKey := strings.TrimPrefix(key, "/")
	dst := filepath.Join(s.dir, cleanKey)

	// Ensure destination is within storage dir
	absDst, _ := filepath.Abs(dst)
	absDir, _ := filepath.Abs(s.dir)
	if !strings.HasPrefix(absDst, absDir+string(os.PathSeparator)) && absDst != absDir {
		return nil, fmt.Errorf("path traversal detected: %s", key)
	}

	// Create parent dirs for the key
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return nil, fmt.Errorf("create key parent dir: %w", err)
	}

	// Write via temp file then rename (atomic)
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".tmp-*")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	var written int64
	if s.maxBytes > 0 && size <= 0 {
		// Unknown size — limit via io.LimitReader
		written, err = io.Copy(tmp, io.LimitReader(src, s.maxBytes+1))
		if written > s.maxBytes {
			_ = tmp.Close()
			_ = os.Remove(tmpName)
			return nil, fmt.Errorf("recording exceeds max size %d bytes", s.maxBytes)
		}
	} else {
		written, err = io.Copy(tmp, src)
	}
	if err != nil {
		_ = tmp.Close()
		_ = os.Remove(tmpName)
		return nil, fmt.Errorf("write recording: %w", err)
	}
	if err := tmp.Close(); err != nil {
		_ = os.Remove(tmpName)
		return nil, fmt.Errorf("close recording: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpName, dst); err != nil {
		_ = os.Remove(tmpName) // cleanup temp on rename failure
		return nil, fmt.Errorf("rename recording: %w", err)
	}
	// fsync directory to persist rename
	if dirF, err := os.Open(filepath.Dir(dst)); err == nil {
		_ = dirF.Sync()
		_ = dirF.Close()
	}

	url := "/recordings/" + cleanKey
	return &RecordingAttachment{URL: url, Size: written}, nil
}

// Delete removes a recording file from disk.
func (s *diskRecordingStore) Delete(ctx context.Context, key string) error {
	cleanKey := strings.TrimPrefix(key, "/")
	dst := filepath.Join(s.dir, cleanKey)

	// Path traversal guard
	absDst, _ := filepath.Abs(dst)
	absDir, _ := filepath.Abs(s.dir)
	if !strings.HasPrefix(absDst, absDir+string(os.PathSeparator)) && absDst != absDir {
		return fmt.Errorf("path traversal: %s", key)
	}

	if err := os.Remove(dst); err != nil {
		if os.IsNotExist(err) {
			return nil // already gone
		}
		return fmt.Errorf("delete recording file: %w", err)
	}

	// Remove empty parent dirs up to storage dir
	for dir := filepath.Dir(dst); strings.HasPrefix(dir, absDir+string(os.PathSeparator)) && dir != absDir; dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil {
			break // dir not empty or can't remove — stop
		}
	}

	return nil
}

// ─── S3 backend ────────────────────────────────────────────────────────────────

type s3RecordingStore struct {
	endpoint  string
	bucket    string
	region    string
	accessKey string
	secretKey string
	publicURL string
	maxBytes  int64
	fallback  *diskRecordingStore
}

func (s *s3RecordingStore) Store(ctx context.Context, key string, src io.Reader, size int64) (*RecordingAttachment, error) {
	// Max file size check (upfront if size known)
	if s.maxBytes > 0 && size > s.maxBytes {
		return nil, fmt.Errorf("recording file size %d exceeds max %d", size, s.maxBytes)
	}

	// Read into buffer for S3 upload (V4 signing needs full payload for SHA256)
	data, err := io.ReadAll(src)
	if err != nil {
		return nil, fmt.Errorf("read recording data: %w", err)
	}

	// Size check after read if not known upfront
	if s.maxBytes > 0 && int64(len(data)) > s.maxBytes {
		return nil, fmt.Errorf("recording size %d exceeds max %d", len(data), s.maxBytes)
	}

	// Upload via shared SigV4 helper
	contentType := "video/mp4"
	if strings.HasSuffix(key, ".webm") {
		contentType = "video/webm"
	}

	if err := s3PutObject(s.endpoint, s.bucket, s.region, s.accessKey, s.secretKey, key, contentType, data); err != nil {
		return nil, fmt.Errorf("s3 upload: %w", err)
	}

	baseURL := strings.TrimRight(s.publicURL, "/")
	if baseURL == "" {
		baseURL = strings.TrimRight(s.endpoint, "/") + "/" + s.bucket
	}
	url := baseURL + "/" + key

	return &RecordingAttachment{URL: url, Size: int64(len(data))}, nil
}

// Delete removes a recording file from S3.
func (s *s3RecordingStore) Delete(ctx context.Context, key string) error {
	return s3DeleteObject(s.endpoint, s.bucket, s.region, s.accessKey, s.secretKey, key)
}

// ExtractStorageKey converts a recording FileURL back to the storage key.
// Disk: /recordings/{createdBy}/{roomID}/{file}.mp4 → recordings/{createdBy}/{roomID}/{file}.mp4
// S3:   https://s3.example.com/bucket/recordings/{path} → recordings/{path}
func ExtractStorageKey(fileURL string) string {
	u, err := url.Parse(fileURL)
	if err != nil {
		return fileURL
	}
	key := strings.TrimPrefix(u.Path, "/")
	if strings.HasPrefix(key, "recordings/") {
		return key
	}
	// S3: strip bucket name prefix
	parts := strings.SplitN(key, "/", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return key
}
