package storage

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"bedrud/config"
	"bedrud/internal/models"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const (
	defaultChatUploadDir = "./data/uploads/chat"
	uploadBackendInline  = "inline"
	uploadBackendDisk    = "disk"
	uploadBackendS3      = "s3"
)

// ChatAttachment is the metadata returned after a successful upload.
type ChatAttachment struct {
	Kind   string `json:"kind"` // "image" | "file"
	URL    string `json:"url"`
	Mime   string `json:"mime"`
	Size   int64  `json:"size"`
	Width  int    `json:"w"`
	Height int    `json:"h"`
	// Name is the original filename for non-image file attachments (WebXDC sendToChat, etc.).
	Name string `json:"name,omitempty"`
	// StorageBackend is server-only for cleanup routing (disk/s3 both use /uploads/chat/).
	StorageBackend string `json:"-"`
}

// ChatUploadStore handles persisting uploaded chat images and generic files.
type ChatUploadStore interface {
	// Store writes image data under roomID/userID/{hash}{ext} (image types only).
	Store(roomID, userID string, data []byte) (*ChatAttachment, error)
	// StoreNamed stores an image when possible; otherwise a generic file attachment
	// with the sanitized original filename (for sendToChat / file-only chat).
	StoreNamed(roomID, userID string, data []byte, filename string) (*ChatAttachment, error)
}

// ChatObjectKey is the relative storage key / URL suffix: room/user/hash.ext
func ChatObjectKey(roomID, userID, hash, ext string) string {
	return roomID + "/" + userID + "/" + hash + ext
}

// ChatUploadRoomID extracts roomID from a relative /uploads/chat/* path.
// Expected: {roomID}/{userID}/{hash}.ext
func ChatUploadRoomID(path string) (roomID string, ok bool) {
	if path == "" || strings.Contains(path, "..") || strings.Contains(path, "\\") {
		return "", false
	}
	parts := strings.Split(path, "/")
	if len(parts) != 3 || parts[0] == "" || parts[1] == "" || parts[2] == "" {
		return "", false
	}
	return parts[0], true
}

// allowedMimeTypes are the only MIME types accepted for chat image uploads.
var allowedMimeTypes = map[string]string{
	"image/png":  ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// imageDimensions extracts width/height from image bytes without fully decoding.
// Supports WebP with manual magic byte parsing (Go stdlib doesn't decode WebP).
func imageDimensions(data []byte) (width, height int) {
	// Manual WebP detection: RIFF + size + WEBP magic
	if len(data) >= 12 &&
		data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' &&
		data[8] == 'W' && data[9] == 'E' && data[10] == 'B' && data[11] == 'P' {
		return webpDimensions(data)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// webpDimensions parses WebP image dimensions from RIFF container.
func webpDimensions(data []byte) (width, height int) {
	if len(data) < 30 {
		return 0, 0
	}
	chunkType := string(data[12:16])
	switch chunkType {
	case "VP8 ": // Lossy WebP
		// VP8: 4 bytes header, then 3 bytes width, 3 bytes height (13-18)
		// Width: ((data[14] << 8) | data[13]) & 0x3FFF
		// Height: ((data[16] << 8) | data[15]) & 0x3FFF
		if len(data) < 19 {
			return 0, 0
		}
		w := int(data[14])<<8 | int(data[13])
		h := int(data[16])<<8 | int(data[15])
		return w & 0x3FFF, h & 0x3FFF
	case "VP8L": // Lossless WebP
		// VP8L: 5 bytes signature, then 14 bits width, 14 bits height
		if len(data) < 21 {
			return 0, 0
		}
		w := int(data[17]) | int(data[18])<<8
		h := int(data[19]) | int(data[20])<<8
		return (w & 0x3FFF) + 1, (h & 0x3FFF) + 1
	case "VP8X": // Extended WebP
		// VP8X: 8 bytes header, then width (3 bytes), height (3 bytes)
		// Width/height stored as 24-bit little-endian, each +1
		if len(data) < 24 {
			return 0, 0
		}
		w := int(data[20]) | int(data[21])<<8 | int(data[22])<<16
		h := int(data[23]) | int(data[24])<<8 | int(data[25])<<16
		return (w & 0xFFFFFF) + 1, (h & 0xFFFFFF) + 1
	}
	return 0, 0
}

// sniffMime returns the content type of the data, restricted to allowed image types.
// Returns an error if the type is not a permitted image format.
func SniffMime(data []byte) (string, error) {
	// WebP detection: RIFF....WEBP (http.DetectContentType does not detect WebP)
	if len(data) >= 12 &&
		data[0] == 'R' && data[1] == 'I' && data[2] == 'F' && data[3] == 'F' &&
		data[8] == 'W' && data[9] == 'E' && data[10] == 'B' && data[11] == 'P' {
		return "image/webp", nil
	}
	mime := http.DetectContentType(data)
	// DetectContentType can return "image/jpeg" or "image/png" etc.
	// Strip parameters like "; charset=utf-8" if present.
	if i := strings.Index(mime, ";"); i != -1 {
		mime = strings.TrimSpace(mime[:i])
	}
	if _, ok := allowedMimeTypes[mime]; !ok {
		return "", fmt.Errorf("unsupported image type: %s", mime)
	}
	return mime, nil
}

// SanitizeChatFilename keeps a safe display name for file attachments.
func SanitizeChatFilename(filename string) string {
	base := filepath.Base(strings.ReplaceAll(filename, "\\", "/"))
	base = strings.TrimSpace(base)
	base = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, base)
	if base == "" || base == "." || base == ".." {
		return "file.bin"
	}
	if len(base) > 120 {
		// keep extension when truncating
		ext := filepath.Ext(base)
		stem := strings.TrimSuffix(base, ext)
		if len(stem) > 100 {
			stem = stem[:100]
		}
		base = stem + ext
	}
	return base
}

// SafeChatFileExt returns a short alphanumeric extension (with leading dot) or ".bin".
func SafeChatFileExt(filename string) string {
	ext := strings.ToLower(filepath.Ext(SanitizeChatFilename(filename)))
	if ext == "" || len(ext) > 12 {
		return ".bin"
	}
	for _, r := range ext[1:] {
		if (r < 'a' || r > 'z') && (r < '0' || r > '9') {
			return ".bin"
		}
	}
	return ext
}

// SniffFileMime returns a content-type for generic file storage (not image-restricted).
func SniffFileMime(data []byte, filename string) string {
	if mime, err := SniffMime(data); err == nil {
		return mime
	}
	mime := http.DetectContentType(data)
	if i := strings.Index(mime, ";"); i != -1 {
		mime = strings.TrimSpace(mime[:i])
	}
	if mime == "" || mime == "application/octet-stream" {
		// Prefer a better guess from extension for common text types.
		switch SafeChatFileExt(filename) {
		case ".txt", ".md", ".csv", ".log":
			return "text/plain"
		case ".json":
			return "application/json"
		case ".pdf":
			return "application/pdf"
		case ".zip", ".xdc":
			return "application/zip"
		case ".html", ".htm":
			return "text/html"
		case ".wasm":
			return "application/wasm"
		}
	}
	if mime == "" {
		return "application/octet-stream"
	}
	return mime
}

// ContentHash returns a hex-encoded SHA256 of the data — used as filename.
func ContentHash(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// NewChatUploadStore creates the appropriate backend from config.
// Selection rule: if data size < InlineMaxBytes → inline base64; else use configured backend.
func NewChatUploadStore(cfg *config.ChatUploadConfig) ChatUploadStore {
	inlineMax := cfg.InlineMaxBytes.Int64()
	if inlineMax == 0 {
		inlineMax = 512_000 // 500 KB default
	}

	diskDir := cfg.DiskDir
	if diskDir == "" {
		diskDir = defaultChatUploadDir
	}

	switch strings.ToLower(cfg.Backend) {
	case uploadBackendS3:
		return &s3Store{
			cfg:            cfg.S3,
			inlineMaxBytes: inlineMax,
			diskFallback:   &diskStore{dir: diskDir},
		}
	case uploadBackendInline:
		// Always inline regardless of size.
		return &inlineStore{}
	default: // disk or empty
		return &hybridStore{
			inlineMaxBytes: inlineMax,
			disk:           &diskStore{dir: diskDir},
		}
	}
}

// ─── Disk backend ─────────────────────────────────────────────────────────────

type diskStore struct{ dir string }

func (s *diskStore) writeBytes(roomID, userID, ext string, data []byte) (key string, err error) {
	hash := ContentHash(data)
	key = ChatObjectKey(roomID, userID, hash, ext)
	path := filepath.Join(s.dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("failed to create upload dir: %w", err)
	}
	// Write only if not already present (content-addressed = idempotent).
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return "", fmt.Errorf("failed to write upload: %w", err)
		}
	}
	return key, nil
}

func (s *diskStore) Store(roomID, userID string, data []byte) (*ChatAttachment, error) {
	mime, err := SniffMime(data)
	if err != nil {
		return nil, err
	}
	ext := allowedMimeTypes[mime]
	key, err := s.writeBytes(roomID, userID, ext, data)
	if err != nil {
		return nil, err
	}
	w, h := imageDimensions(data)
	return &ChatAttachment{
		Kind:           "image",
		URL:            "/uploads/chat/" + key,
		Mime:           mime,
		Size:           int64(len(data)),
		Width:          w,
		Height:         h,
		StorageBackend: uploadBackendDisk,
	}, nil
}

func (s *diskStore) StoreNamed(roomID, userID string, data []byte, filename string) (*ChatAttachment, error) {
	if _, err := SniffMime(data); err == nil {
		return s.Store(roomID, userID, data)
	}
	name := SanitizeChatFilename(filename)
	ext := SafeChatFileExt(name)
	mime := SniffFileMime(data, name)
	key, err := s.writeBytes(roomID, userID, ext, data)
	if err != nil {
		return nil, err
	}
	return &ChatAttachment{
		Kind:           "file",
		URL:            "/uploads/chat/" + key,
		Mime:           mime,
		Size:           int64(len(data)),
		Name:           name,
		StorageBackend: uploadBackendDisk,
	}, nil
}

// ─── Inline (base64 data URI) backend ─────────────────────────────────────────

type inlineStore struct{}

func (s *inlineStore) Store(roomID, userID string, data []byte) (*ChatAttachment, error) {
	_, _ = roomID, userID
	mime, err := SniffMime(data)
	if err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	dataURI := "data:" + mime + ";base64," + encoded
	w, h := imageDimensions(data)
	return &ChatAttachment{
		Kind:           "image",
		URL:            dataURI,
		Mime:           mime,
		Size:           int64(len(data)),
		Width:          w,
		Height:         h,
		StorageBackend: uploadBackendInline,
	}, nil
}

func (s *inlineStore) StoreNamed(roomID, userID string, data []byte, filename string) (*ChatAttachment, error) {
	if _, err := SniffMime(data); err == nil {
		return s.Store(roomID, userID, data)
	}
	_, _ = roomID, userID
	name := SanitizeChatFilename(filename)
	mime := SniffFileMime(data, name)
	encoded := base64.StdEncoding.EncodeToString(data)
	dataURI := "data:" + mime + ";base64," + encoded
	return &ChatAttachment{
		Kind:           "file",
		URL:            dataURI,
		Mime:           mime,
		Size:           int64(len(data)),
		Name:           name,
		StorageBackend: uploadBackendInline,
	}, nil
}

// ─── Hybrid backend (inline if small, disk otherwise) ─────────────────────────

type hybridStore struct {
	inlineMaxBytes int64
	disk           *diskStore
}

func (s *hybridStore) Store(roomID, userID string, data []byte) (*ChatAttachment, error) {
	if int64(len(data)) < s.inlineMaxBytes {
		return (&inlineStore{}).Store(roomID, userID, data)
	}
	return s.disk.Store(roomID, userID, data)
}

func (s *hybridStore) StoreNamed(roomID, userID string, data []byte, filename string) (*ChatAttachment, error) {
	if int64(len(data)) < s.inlineMaxBytes {
		return (&inlineStore{}).StoreNamed(roomID, userID, data, filename)
	}
	return s.disk.StoreNamed(roomID, userID, data, filename)
}

// ─── S3-compatible backend ─────────────────────────────────────────────────────

// s3Store uploads to an S3/R2-compatible endpoint using AWS Signature V4.
// Falls back to disk when S3 config is incomplete (so the server still works
// even without S3 credentials configured).
type s3Store struct {
	cfg            config.ChatUploadS3Config
	inlineMaxBytes int64
	diskFallback   *diskStore
}

func (s *s3Store) Store(roomID, userID string, data []byte) (*ChatAttachment, error) {
	if s.cfg.Endpoint == "" || s.cfg.Bucket == "" || s.cfg.AccessKey == "" {
		// Fall back to disk when S3 is not fully configured.
		return s.diskFallback.Store(roomID, userID, data)
	}

	// Inline if small.
	if s.inlineMaxBytes > 0 && int64(len(data)) < s.inlineMaxBytes {
		return (&inlineStore{}).Store(roomID, userID, data)
	}

	mime, err := SniffMime(data)
	if err != nil {
		return nil, err
	}
	ext := allowedMimeTypes[mime]
	hash := ContentHash(data)
	key := ChatObjectKey(roomID, userID, hash, ext)

	if err := s.putObject(key, mime, data); err != nil {
		return nil, fmt.Errorf("s3 upload failed: %w", err)
	}

	// Serve via /uploads/chat/* which presigns a short-lived GET URL.
	w, h := imageDimensions(data)
	return &ChatAttachment{
		Kind:           "image",
		URL:            "/uploads/chat/" + key,
		Mime:           mime,
		Size:           int64(len(data)),
		Width:          w,
		Height:         h,
		StorageBackend: uploadBackendS3,
	}, nil
}

func (s *s3Store) StoreNamed(roomID, userID string, data []byte, filename string) (*ChatAttachment, error) {
	if s.cfg.Endpoint == "" || s.cfg.Bucket == "" || s.cfg.AccessKey == "" {
		return s.diskFallback.StoreNamed(roomID, userID, data, filename)
	}
	if s.inlineMaxBytes > 0 && int64(len(data)) < s.inlineMaxBytes {
		return (&inlineStore{}).StoreNamed(roomID, userID, data, filename)
	}
	if _, err := SniffMime(data); err == nil {
		return s.Store(roomID, userID, data)
	}
	name := SanitizeChatFilename(filename)
	ext := SafeChatFileExt(name)
	mime := SniffFileMime(data, name)
	hash := ContentHash(data)
	key := ChatObjectKey(roomID, userID, hash, ext)
	if err := s.putObject(key, mime, data); err != nil {
		return nil, fmt.Errorf("s3 upload failed: %w", err)
	}
	return &ChatAttachment{
		Kind:           "file",
		URL:            "/uploads/chat/" + key,
		Mime:           mime,
		Size:           int64(len(data)),
		Name:           name,
		StorageBackend: uploadBackendS3,
	}, nil
}

// putObject performs an AWS SigV4-signed PUT request to the S3 endpoint.
func (s *s3Store) putObject(key, contentType string, data []byte) error {
	return s3PutObject(s.cfg.Endpoint, s.cfg.Bucket, s.cfg.Region, s.cfg.AccessKey, s.cfg.SecretKey, key, contentType, data)
}

// s3PutObject uploads data to an S3-compatible bucket using AWS SigV4.
func s3PutObject(endpoint, bucket, region, accessKey, secretKey, key, contentType string, data []byte) error {
	if region == "" {
		region = "auto"
	}
	endpoint = strings.TrimRight(endpoint, "/")
	url := endpoint + "/" + bucket + "/" + key

	now := time.Now().UTC()
	datestamp := now.Format("20060102")
	amzdate := now.Format("20060102T150405Z")

	payloadHash := fmt.Sprintf("%x", sha256.Sum256(data))

	req, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("x-amz-date", amzdate)

	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := fmt.Sprintf(
		"content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		contentType, req.URL.Host, payloadHash, amzdate,
	)
	canonicalURI := "/" + bucket + "/" + key
	canonicalRequest := strings.Join([]string{
		"PUT", canonicalURI, "", canonicalHeaders, signedHeaders, payloadHash,
	}, "\n")

	credScope := datestamp + "/" + region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256", amzdate, credScope,
		fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalRequest))),
	}, "\n")

	signingKey := s3DeriveSigningKey(secretKey, datestamp, region)
	signature := fmt.Sprintf("%x", s3HMACSHA256(signingKey, stringToSign))

	authHeader := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, credScope, signedHeaders, signature,
	)
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("s3 returned status %d", resp.StatusCode)
	}
	return nil
}

// s3DeleteObject removes an object from an S3-compatible bucket using AWS SigV4.
func s3DeleteObject(endpoint, bucket, region, accessKey, secretKey, key string) error {
	if endpoint == "" || bucket == "" || accessKey == "" {
		return fmt.Errorf("s3 not configured")
	}
	if region == "" {
		region = "auto"
	}
	endpoint = strings.TrimRight(endpoint, "/")
	url := endpoint + "/" + bucket + "/" + key

	now := time.Now().UTC()
	datestamp := now.Format("20060102")
	amzdate := now.Format("20060102T150405Z")
	emptyPayloadHash := fmt.Sprintf("%x", sha256.Sum256([]byte("")))

	req, err := http.NewRequest(http.MethodDelete, url, http.NoBody)
	if err != nil {
		return err
	}
	req.Header.Set("x-amz-content-sha256", emptyPayloadHash)
	req.Header.Set("x-amz-date", amzdate)

	signedHeaders := "host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := fmt.Sprintf(
		"host:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		req.URL.Host, emptyPayloadHash, amzdate,
	)
	canonicalURI := "/" + bucket + "/" + key
	canonicalRequest := strings.Join([]string{
		"DELETE", canonicalURI, "", canonicalHeaders, signedHeaders, emptyPayloadHash,
	}, "\n")

	credScope := datestamp + "/" + region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256", amzdate, credScope,
		fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalRequest))),
	}, "\n")

	signingKey := s3DeriveSigningKey(secretKey, datestamp, region)
	signature := fmt.Sprintf("%x", s3HMACSHA256(signingKey, stringToSign))

	authHeader := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		accessKey, credScope, signedHeaders, signature,
	)
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("s3 delete returned status %d", resp.StatusCode)
	}
	return nil
}

func s3HMACSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func s3DeriveSigningKey(secretKey, datestamp, region string) []byte {
	kDate := s3HMACSHA256([]byte("AWS4"+secretKey), datestamp)
	kRegion := s3HMACSHA256(kDate, region)
	kService := s3HMACSHA256(kRegion, "s3")
	return s3HMACSHA256(kService, "aws4_request")
}

// deleteObject removes an object from the S3 bucket using AWS Signature V4.
// Errors are returned to the caller (ChatUploadTracker logs a warning and continues).
func (s *s3Store) deleteObject(key string) error {
	return s3DeleteObject(s.cfg.Endpoint, s.cfg.Bucket, s.cfg.Region, s.cfg.AccessKey, s.cfg.SecretKey, key)
}

// DeleteObject implements ObjectDeleter.
func (s *s3Store) DeleteObject(key string) error { return s.deleteObject(key) }

// ObjectDeleter removes an object from an S3-compatible bucket by key.
type ObjectDeleter interface {
	DeleteObject(key string) error
}

// ObjectPresigner issues time-limited GET URLs for private S3 objects.
type ObjectPresigner interface {
	PresignGet(key string, expiry time.Duration) (string, error)
}

// NewS3Deleter creates an ObjectDeleter backed by the S3 configuration.
// Also implements ObjectPresigner when fully configured.
func NewS3Deleter(cfg *config.ChatUploadS3Config) ObjectDeleter {
	if cfg == nil {
		return &s3Store{}
	}
	return &s3Store{cfg: *cfg}
}

// defaultChatUploadPresignTTL is how long a signed GET URL stays valid.
const defaultChatUploadPresignTTL = time.Hour

// PresignGet implements ObjectPresigner.
func (s *s3Store) PresignGet(key string, expiry time.Duration) (string, error) {
	if s.cfg.Endpoint == "" || s.cfg.Bucket == "" || s.cfg.AccessKey == "" || s.cfg.SecretKey == "" {
		return "", fmt.Errorf("s3 not configured")
	}
	if expiry <= 0 {
		expiry = defaultChatUploadPresignTTL
	}
	return s3PresignGetURL(s.cfg.Endpoint, s.cfg.Bucket, s.cfg.Region, s.cfg.AccessKey, s.cfg.SecretKey, key, expiry)
}

// s3PresignGetURL builds an AWS SigV4 query-string presigned GET URL (path-style).
func s3PresignGetURL(endpoint, bucket, region, accessKey, secretKey, key string, expiry time.Duration) (string, error) {
	if region == "" {
		region = "auto"
	}
	endpoint = strings.TrimRight(endpoint, "/")
	parsed, err := http.NewRequest(http.MethodGet, endpoint+"/"+bucket+"/"+key, http.NoBody)
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()
	datestamp := now.Format("20060102")
	amzdate := now.Format("20060102T150405Z")
	expires := int(expiry.Seconds())
	if expires < 1 {
		expires = 1
	}
	if expires > 604800 { // AWS max 7 days
		expires = 604800
	}

	credScope := datestamp + "/" + region + "/s3/aws4_request"
	credential := accessKey + "/" + credScope

	// Query params sorted by name for the canonical request.
	query := fmt.Sprintf(
		"X-Amz-Algorithm=AWS4-HMAC-SHA256&X-Amz-Credential=%s&X-Amz-Date=%s&X-Amz-Expires=%d&X-Amz-SignedHeaders=host",
		s3URIEncode(credential, true), amzdate, expires,
	)

	canonicalURI := "/" + bucket + "/" + key
	canonicalHeaders := "host:" + parsed.URL.Host + "\n"
	signedHeaders := "host"
	canonicalRequest := strings.Join([]string{
		"GET", canonicalURI, query, canonicalHeaders, signedHeaders, "UNSIGNED-PAYLOAD",
	}, "\n")

	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256", amzdate, credScope,
		fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalRequest))),
	}, "\n")

	signingKey := s3DeriveSigningKey(secretKey, datestamp, region)
	signature := fmt.Sprintf("%x", s3HMACSHA256(signingKey, stringToSign))

	return endpoint + "/" + bucket + "/" + key + "?" + query + "&X-Amz-Signature=" + signature, nil
}

// s3URIEncode percent-encodes for AWS SigV4 (encodeSlash=true for query values).
func s3URIEncode(s string, encodeSlash bool) string {
	var b strings.Builder
	b.Grow(len(s) * 2)
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '.' || c == '~' || (c == '/' && !encodeSlash) {
			b.WriteByte(c)
			continue
		}
		fmt.Fprintf(&b, "%%%02X", c)
	}
	return b.String()
}

type ChatUploadTracker struct {
	db      *gorm.DB
	chatDir string
	deleter ObjectDeleter

	mu                 sync.Mutex
	totalBytesCache    int64
	totalBytesCachedAt time.Time
}

func NewChatUploadTracker(db *gorm.DB, chatDir string, deleter ObjectDeleter) *ChatUploadTracker {
	if chatDir == "" {
		chatDir = defaultChatUploadDir
	}
	return &ChatUploadTracker{db: db, chatDir: chatDir, deleter: deleter}
}

// ResolveChatUpload maps /uploads/chat/{room}/{user}/{hash}.ext to local file or presigned S3 GET.
// Prefer local file if present (disk backend). Else presign when ObjectPresigner is set.
// Returns (filePath, redirectURL, error). Caller: if redirectURL != "" → 302, else SendFile(filePath).
// Caller must authorize room access separately via ChatUploadRoomID(path).
func ResolveChatUpload(path, uploadDir string, presigner ObjectPresigner) (filePath, redirect string, err error) {
	if _, ok := ChatUploadRoomID(path); !ok {
		return "", "", fmt.Errorf("invalid path")
	}
	cleanDir := filepath.Clean(uploadDir)
	resolved := filepath.Join(uploadDir, filepath.FromSlash(path))
	if !strings.HasPrefix(resolved, cleanDir+string(os.PathSeparator)) && resolved != cleanDir {
		return "", "", fmt.Errorf("invalid path")
	}
	if st, statErr := os.Stat(resolved); statErr == nil && !st.IsDir() {
		return resolved, "", nil
	}
	if presigner != nil {
		url, pErr := presigner.PresignGet(path, defaultChatUploadPresignTTL)
		if pErr != nil {
			return "", "", pErr
		}
		return "", url, nil
	}
	return "", "", fmt.Errorf("not found")
}

func (t *ChatUploadTracker) Record(roomID, userID, fileHash, ext string, fileSize int64, backend string) error {
	upload := &models.ChatUpload{
		ID:             uuid.New().String(),
		RoomID:         roomID,
		UploadedBy:     userID,
		FileHash:       fileHash,
		Extension:      ext,
		FileSize:       fileSize,
		StorageBackend: backend,
	}
	return t.db.Create(upload).Error
}

// GetUserUploadBytes returns the total bytes stored by a user via chat uploads.
func (t *ChatUploadTracker) GetUserUploadBytes(userID string) (int64, error) {
	var total int64
	err := t.db.Model(&models.ChatUpload{}).
		Where("uploaded_by = ?", userID).
		Select("COALESCE(SUM(file_size), 0)").
		Scan(&total).Error
	return total, err
}

// GetTotalUploadBytes returns the total bytes stored across all users.
// Results are cached for 60 seconds to avoid excessive DB queries.
func (t *ChatUploadTracker) GetTotalUploadBytes() (int64, error) {
	t.mu.Lock()
	if time.Since(t.totalBytesCachedAt) < 60*time.Second {
		cached := t.totalBytesCache
		t.mu.Unlock()
		return cached, nil
	}
	t.mu.Unlock()

	var total int64
	err := t.db.Model(&models.ChatUpload{}).
		Select("COALESCE(SUM(file_size), 0)").
		Scan(&total).Error
	if err != nil {
		return 0, err
	}

	t.mu.Lock()
	t.totalBytesCache = total
	t.totalBytesCachedAt = time.Now()
	t.mu.Unlock()
	return total, nil
}

func (t *ChatUploadTracker) DeleteByRoom(roomID string) error {
	var deleted []models.ChatUpload
	result := t.db.Clauses(clause.Returning{}).Where("room_id = ?", roomID).Delete(&deleted)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return nil
	}
	for i := range deleted {
		u := &deleted[i]
		// Objects are keyed room/user/hash — unique per upload path, always delete.
		key := ChatObjectKey(u.RoomID, u.UploadedBy, u.FileHash, u.Extension)
		switch u.StorageBackend {
		case uploadBackendS3:
			if t.deleter != nil {
				if err := t.deleter.DeleteObject(key); err != nil {
					log.Warn().Err(err).Str("key", key).Str("roomID", roomID).Msg("failed to delete S3 chat upload object")
				}
			} else {
				log.Warn().Str("key", key).Str("roomID", roomID).Msg("no S3 deleter configured, orphaned S3 object")
			}
		case uploadBackendInline:
		default: // disk or unknown
			path := filepath.Join(t.chatDir, filepath.FromSlash(key))
			if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
				log.Warn().Err(err).Str("path", path).Msg("orphan chat upload file on disk")
			}
		}
	}
	return nil
}
