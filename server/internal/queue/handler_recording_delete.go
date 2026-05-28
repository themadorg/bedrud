// TODO oncoming feature
package queue

import (
	"context"
	"encoding/json"
	"fmt"

	"bedrud/internal/models"
	"bedrud/internal/repository"

	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

// NewRecordingDeleteHandler creates a handler that hard-deletes a recording.
func NewRecordingDeleteHandler(recordingRepo *repository.RecordingRepository) Handler {
	return func(ctx context.Context, db *gorm.DB, job *models.Job) error {
		var payload RecordingDeletePayload
		if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
			return fmt.Errorf("unmarshal recording_delete payload: %w", err)
		}

		recording, err := recordingRepo.GetByID(payload.RecordingID)
		if err != nil {
			if err == repository.ErrRecordingNotFound {
				log.Warn().Str("recordingID", payload.RecordingID).
					Msg("recording_delete: already gone")
				return nil
			}
			return fmt.Errorf("fetch recording %s: %w", payload.RecordingID, err)
		}

		// Hard delete the DB record
		if err := recordingRepo.DeleteRecording(payload.RecordingID); err != nil {
			return fmt.Errorf("delete recording %s: %w", payload.RecordingID, err)
		}

		log.Info().Str("recordingID", payload.RecordingID).
			Str("roomID", payload.RoomID).
			Str("status", string(recording.Status)).
			Msg("recording_delete: completed")

		return nil
	}
}
