package queue

import (
	"bedrud/internal/models"
	"bedrud/internal/storage"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// NewChatUploadS3Handler creates a handler that processes async S3 chat uploads.
// Only enqueued for S3-backed uploadBackends; small files are always inline-sync.
func NewChatUploadS3Handler(
	uploadStore storage.ChatUploadStore,
	uploadTracker *storage.ChatUploadTracker,
) Handler {
	return func(ctx context.Context, db *gorm.DB, job *models.Job) error {
		var payload ChatUploadS3Payload
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			return fmt.Errorf("unmarshal chat_upload_s3 payload: %w", err)
		}

		data, err := base64.StdEncoding.DecodeString(payload.Data)
		if err != nil {
			return fmt.Errorf("decode base64: %w", err)
		}

		attachment, err := uploadStore.Store(data)
		if err != nil {
			return fmt.Errorf("store upload: %w", err)
		}

		if uploadTracker != nil {
			// Derive fileHash from content and ext from MIME type
			fileHash := storage.ContentHash(data)
			ext := mimeToExt(attachment.Mime)
			uploadTracker.Record(payload.RoomID, payload.UserID, fileHash, ext, attachment.Size, "s3")
		}

		log.Info().Str("roomID", payload.RoomID).Str("userID", payload.UserID).
			Str("mime", attachment.Mime).Int64("size", attachment.Size).
			Msg("queue: chat upload stored to S3")
		return nil
	}
}

// mimeToExt maps a MIME type to its file extension. Defaults to ".bin".
func mimeToExt(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	default:
		return ".bin"
	}
}
