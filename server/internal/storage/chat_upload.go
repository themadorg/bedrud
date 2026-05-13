package storage

import (
	"bedrud/config"
	"bedrud/internal/models"
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
	"time"

	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ChatAttachment is the metadata returned after a successful upload.
type ChatAttachment struct {
	URL    string `json:"url"`
	Mime   string `json:"mime"`
	Size   int64  `json:"size"`
	Width  int    `json:"w"`
	Height int    `json:"h"`
}

// ChatUploadStore handles persisting uploaded chat images.
type ChatUploadStore interface {
	Store(data []byte) (*ChatAttachment, error)
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
func sniffMime(data []byte) (string, error) {
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

// contentHash returns a hex-encoded SHA256 of the data — used as filename.
func contentHash(data []byte) string {
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
		diskDir = "./data/uploads/chat"
	}

	switch strings.ToLower(cfg.Backend) {
	case "s3":
		return &s3Store{
			cfg:            cfg.S3,
			inlineMaxBytes: inlineMax,
			diskFallback:   &diskStore{dir: diskDir},
		}
	case "inline":
		// Always inline regardless of size.
		return &inlineStore{}
	default: // "disk" or empty
		return &hybridStore{
			inlineMaxBytes: inlineMax,
			disk:           &diskStore{dir: diskDir},
		}
	}
}

// ─── Disk backend ─────────────────────────────────────────────────────────────

type diskStore struct{ dir string }

func (s *diskStore) Store(data []byte) (*ChatAttachment, error) {
	mime, err := sniffMime(data)
	if err != nil {
		return nil, err
	}
	ext := allowedMimeTypes[mime]
	hash := contentHash(data)
	filename := hash + ext

	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create upload dir: %w", err)
	}

	path := filepath.Join(s.dir, filename)
	// Write only if not already present (content-addressed = idempotent).
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			return nil, fmt.Errorf("failed to write upload: %w", err)
		}
	}

	w, h := imageDimensions(data)
	return &ChatAttachment{
		URL:    "/uploads/chat/" + filename,
		Mime:   mime,
		Size:   int64(len(data)),
		Width:  w,
		Height: h,
	}, nil
}

// ─── Inline (base64 data URI) backend ─────────────────────────────────────────

type inlineStore struct{}

func (s *inlineStore) Store(data []byte) (*ChatAttachment, error) {
	mime, err := sniffMime(data)
	if err != nil {
		return nil, err
	}
	encoded := base64.StdEncoding.EncodeToString(data)
	dataURI := "data:" + mime + ";base64," + encoded
	w, h := imageDimensions(data)
	return &ChatAttachment{
		URL:    dataURI,
		Mime:   mime,
		Size:   int64(len(data)),
		Width:  w,
		Height: h,
	}, nil
}

// ─── Hybrid backend (inline if small, disk otherwise) ─────────────────────────

type hybridStore struct {
	inlineMaxBytes int64
	disk           *diskStore
}

func (s *hybridStore) Store(data []byte) (*ChatAttachment, error) {
	if int64(len(data)) < s.inlineMaxBytes {
		return (&inlineStore{}).Store(data)
	}
	return s.disk.Store(data)
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

func (s *s3Store) Store(data []byte) (*ChatAttachment, error) {
	if s.cfg.Endpoint == "" || s.cfg.Bucket == "" || s.cfg.AccessKey == "" {
		// Fall back to disk when S3 is not fully configured.
		return s.diskFallback.Store(data)
	}

	// Inline if small.
	if s.inlineMaxBytes > 0 && int64(len(data)) < s.inlineMaxBytes {
		return (&inlineStore{}).Store(data)
	}

	mime, err := sniffMime(data)
	if err != nil {
		return nil, err
	}
	ext := allowedMimeTypes[mime]
	hash := contentHash(data)
	key := hash + ext

	if err := s.putObject(key, mime, data); err != nil {
		return nil, fmt.Errorf("s3 upload failed: %w", err)
	}

	publicBase := strings.TrimRight(s.cfg.PublicBaseURL, "/")
	if publicBase == "" {
		publicBase = strings.TrimRight(s.cfg.Endpoint, "/") + "/" + s.cfg.Bucket
	}
	url := publicBase + "/" + key

	w, h := imageDimensions(data)
	return &ChatAttachment{
		URL:    url,
		Mime:   mime,
		Size:   int64(len(data)),
		Width:  w,
		Height: h,
	}, nil
}

// putObject performs an AWS SigV4-signed PUT request to the S3 endpoint.
func (s *s3Store) putObject(key, contentType string, data []byte) error {
	region := s.cfg.Region
	if region == "" {
		region = "auto"
	}
	endpoint := strings.TrimRight(s.cfg.Endpoint, "/")
	url := endpoint + "/" + s.cfg.Bucket + "/" + key

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

	// Build canonical request.
	signedHeaders := "content-type;host;x-amz-content-sha256;x-amz-date"
	canonicalHeaders := fmt.Sprintf(
		"content-type:%s\nhost:%s\nx-amz-content-sha256:%s\nx-amz-date:%s\n",
		contentType, req.URL.Host, payloadHash, amzdate,
	)
	canonicalURI := "/" + s.cfg.Bucket + "/" + key
	canonicalRequest := strings.Join([]string{
		"PUT", canonicalURI, "", canonicalHeaders, signedHeaders, payloadHash,
	}, "\n")

	credScope := datestamp + "/" + region + "/s3/aws4_request"
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256", amzdate, credScope,
		fmt.Sprintf("%x", sha256.Sum256([]byte(canonicalRequest))),
	}, "\n")

	signingKey := s.deriveSigningKey(datestamp, region)
	signature := fmt.Sprintf("%x", s.hmacSHA256(signingKey, stringToSign))

	authHeader := fmt.Sprintf(
		"AWS4-HMAC-SHA256 Credential=%s/%s, SignedHeaders=%s, Signature=%s",
		s.cfg.AccessKey, credScope, signedHeaders, signature,
	)
	req.Header.Set("Authorization", authHeader)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		_ = body // body is intentionally discarded to avoid leaking it to the client; logged upstream
		return fmt.Errorf("s3 returned status %d", resp.StatusCode)
	}
	return nil
}

func (s *s3Store) hmacSHA256(key []byte, data string) []byte {
	h := hmac.New(sha256.New, key)
	h.Write([]byte(data))
	return h.Sum(nil)
}

func (s *s3Store) deriveSigningKey(datestamp, region string) []byte {
	kDate := s.hmacSHA256([]byte("AWS4"+s.cfg.SecretKey), datestamp)
	kRegion := s.hmacSHA256(kDate, region)
	kService := s.hmacSHA256(kRegion, "s3")
	return s.hmacSHA256(kService, "aws4_request")
}

type ChatUploadTracker struct {
	db      *gorm.DB
	chatDir string
}

func NewChatUploadTracker(db *gorm.DB, chatDir string) *ChatUploadTracker {
	if chatDir == "" {
		chatDir = "./data/uploads/chat"
	}
	return &ChatUploadTracker{db: db, chatDir: chatDir}
}

func (t *ChatUploadTracker) Record(roomID, fileHash, ext string) error {
	upload := &models.ChatUpload{
		ID:        uuid.New().String(),
		RoomID:    roomID,
		FileHash:  fileHash,
		Extension: ext,
	}
	return t.db.Create(upload).Error
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
	for _, u := range deleted {
		// Only delete the file if no other room references it (cross-room safety)
		var remaining int64
		t.db.Model(&models.ChatUpload{}).Where("file_hash = ? AND id != ?", u.FileHash, u.ID).Count(&remaining)
		if remaining > 0 {
			continue
		}
		path := filepath.Join(t.chatDir, u.FileHash+u.Extension)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			log.Warn().Err(err).Str("path", path).Msg("orphan chat upload file on disk")
		}
	}
	return nil
}
