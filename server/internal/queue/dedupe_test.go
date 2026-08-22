package queue

import (
	"context"
	"errors"
	"testing"

	"bedrud/internal/models"
	"bedrud/internal/testutil"

	"gorm.io/gorm"
)

// Deduplication is what stops a double-clicked delete from ending a meeting
// twice. It lives in the queue rather than in a handler's memory so it survives
// a restart, holds across server instances, and does not depend on what the
// job's target looks like afterwards — an archived room, a deleted user.

func countJobs(t *testing.T, db *gorm.DB, jobType string) int64 {
	t.Helper()

	var n int64
	if err := db.Model(&models.Job{}).Where("type = ?", jobType).Count(&n).Error; err != nil {
		t.Fatalf("count %s jobs: %v", jobType, err)
	}
	return n
}

func enqueueRoomDelete(ctx context.Context, db *gorm.DB, roomID string) error {
	return Enqueue(ctx, db, "room_delete", RoomDeletePayload{RoomID: roomID}, WithDedupeKey(roomID))
}

func TestEnqueue_RefusesASecondJobForTheSameTarget(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx := context.Background()

	if err := enqueueRoomDelete(ctx, db, "room-1"); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}

	err := enqueueRoomDelete(ctx, db, "room-1")
	if !errors.Is(err, ErrDuplicateJob) {
		t.Fatalf("want ErrDuplicateJob, got %v", err)
	}
	if n := countJobs(t, db, "room_delete"); n != 1 {
		t.Errorf("want 1 job, got %d — the duplicate was inserted", n)
	}
}

func TestEnqueue_AllowsDifferentTargets(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx := context.Background()

	if err := enqueueRoomDelete(ctx, db, "room-1"); err != nil {
		t.Fatalf("room-1: %v", err)
	}
	if err := enqueueRoomDelete(ctx, db, "room-2"); err != nil {
		t.Fatalf("room-2 should not be blocked by room-1: %v", err)
	}

	if n := countJobs(t, db, "room_delete"); n != 2 {
		t.Errorf("want 2 jobs, got %d", n)
	}
}

func TestEnqueue_SameKeyDifferentTypesDoNotCollide(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx := context.Background()

	// A room and a user can share an id; the index is on (type, dedupe_key).
	if err := Enqueue(ctx, db, "room_delete", RoomDeletePayload{RoomID: "x"}, WithDedupeKey("x")); err != nil {
		t.Fatalf("room_delete: %v", err)
	}
	if err := Enqueue(ctx, db, "user_delete", UserDeletePayload{UserID: "x"}, WithDedupeKey("x")); err != nil {
		t.Fatalf("user_delete should not be blocked by room_delete: %v", err)
	}
}

func TestEnqueue_FinishedJobStopsBlocking(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx := context.Background()

	if err := enqueueRoomDelete(ctx, db, "room-1"); err != nil {
		t.Fatalf("first enqueue: %v", err)
	}

	for _, status := range []models.JobStatus{models.JobDone, models.JobFailed} {
		if err := db.Model(&models.Job{}).
			Where("type = ? AND dedupe_key = ?", "room_delete", "room-1").
			Update("status", status).Error; err != nil {
			t.Fatalf("mark %s: %v", status, err)
		}

		// The whole point of a partial index: a finished job must not hold the
		// target forever, which is what an in-memory marker did.
		if err := enqueueRoomDelete(ctx, db, "room-1"); err != nil {
			t.Fatalf("enqueue after the previous job reached %s: %v", status, err)
		}
		if err := db.Where("type = ?", "room_delete").Delete(&models.Job{}).Error; err != nil {
			t.Fatalf("reset: %v", err)
		}
		if err := enqueueRoomDelete(ctx, db, "room-1"); err != nil {
			t.Fatalf("reseed: %v", err)
		}
	}
}

func TestEnqueue_WithoutAKeyNothingIsDeduplicated(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx := context.Background()

	// Job types that do not opt in must be unaffected, including several rows
	// that all carry an empty key — the index exempts those.
	for i := 0; i < 3; i++ {
		if err := Enqueue(ctx, db, "send_email", SendEmailPayload{To: "a@b.test"}); err != nil {
			t.Fatalf("enqueue %d: %v", i, err)
		}
	}

	if n := countJobs(t, db, "send_email"); n != 3 {
		t.Errorf("want 3 jobs, got %d — empty keys were treated as duplicates", n)
	}
}
