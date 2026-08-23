package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"
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
	for _, status := range []models.JobStatus{models.JobDone, models.JobFailed} {
		t.Run(string(status), func(t *testing.T) {
			db := testutil.SetupTestDB(t)
			ctx := context.Background()

			if err := enqueueRoomDelete(ctx, db, "room-1"); err != nil {
				t.Fatalf("first enqueue: %v", err)
			}
			if err := db.Model(&models.Job{}).
				Where("type = ? AND dedupe_key = ?", "room_delete", "room-1").
				Update("status", status).Error; err != nil {
				t.Fatalf("mark %s: %v", status, err)
			}

			// The whole point of a partial index: a finished job must not hold
			// the target forever, which is what an in-memory marker did.
			if err := enqueueRoomDelete(ctx, db, "room-1"); err != nil {
				t.Fatalf("enqueue after the previous job reached %s: %v", status, err)
			}
			if n := countJobs(t, db, "room_delete"); n != 2 {
				t.Errorf("want the finished job plus a fresh one, got %d rows", n)
			}
		})
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

// The pre-check in Enqueue loses to a concurrent enqueue: both callers can read
// "no unfinished job" before either inserts. What actually decides the winner is
// idx_jobs_active_dedupe, and the branch that translates its violation back into
// ErrDuplicateJob is the only reason the loser gets a 409 rather than a 500.
// Nothing else in the suite reaches that branch — the sequential tests above are
// all caught by the pre-check first.
//
// SQLite only, deliberately. The race is reachable here even though the test
// pool is pinned to one connection, because the pre-check and the Create are
// separate statements and goroutines interleave between them. On Postgres the
// network round-trip is slower than goroutine scheduling, so the first insert
// commits before any other caller runs its pre-check and the fallback is never
// exercised — a Postgres subtest would pass green while asserting nothing.
func TestEnqueue_ConcurrentCallersCollapseToOneJob(t *testing.T) {
	db := testutil.SetupTestDB(t)
	ctx := context.Background()

	const callers = 16

	// Repeated over several keys on purpose. Whether any caller actually reaches
	// the fallback is up to the scheduler: measured against a build with the
	// fallback deleted, a single burst left it untouched roughly one run in five,
	// and the assertions below hold either way — so one burst would be an 80%
	// regression guard that looks like a 100% one. Rounds are independent, so the
	// chance of never reaching it falls off geometrically.
	for round := 0; round < 5; round++ {
		key := fmt.Sprintf("room-%d", round)

		var wg sync.WaitGroup
		start := make(chan struct{})
		errs := make([]error, callers)

		for i := 0; i < callers; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				<-start
				errs[i] = enqueueRoomDelete(ctx, db, key)
			}(i)
		}
		close(start) // release them together
		wg.Wait()

		var accepted, duplicate int
		for _, err := range errs {
			switch {
			case err == nil:
				accepted++
			case errors.Is(err, ErrDuplicateJob):
				duplicate++
			default:
				// A raw driver error here means the violation was not translated,
				// which surfaces to the caller as a 500 instead of a 409.
				t.Errorf("%s: caller got neither nil nor ErrDuplicateJob: %v", key, err)
			}
		}

		if accepted != 1 {
			t.Errorf("%s: want exactly 1 accepted enqueue, got %d", key, accepted)
		}
		if duplicate != callers-1 {
			t.Errorf("%s: want %d callers refused as duplicates, got %d", key, callers-1, duplicate)
		}
	}

	// One row per key, so every burst collapsed rather than only the last.
	if n := countJobs(t, db, "room_delete"); n != 5 {
		t.Errorf("want 1 job row per key, got %d", n)
	}
}
