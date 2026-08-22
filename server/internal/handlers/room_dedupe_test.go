package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// Deduplication of room_delete is keyed on the operation, not just the room:
// an archive and a purge share the job type but do different things, so the key
// carries which one. These tests pin that distinction, because reducing the key
// back to a bare room id still passes every other test in the package while
// silently dropping admin purges.

func setupRoomDedupeApp(t *testing.T) (*fiber.App, *gorm.DB, *repository.RoomRepository) {
	t.Helper()

	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	userRepo := repository.NewUserRepository(db)
	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "k", APISecret: "s"}
	handler := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, roomRepo, userRepo, nil, nil, nil, nil, nil)

	db.Create(&models.User{
		ID: "creator-user", Email: "creator@ex.com", Name: "Creator",
		Provider: "local", IsActive: true, Accesses: models.StringArray{"user"},
	})

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", &auth.Claims{UserID: "creator-user", Accesses: []string{"user"}})
		return c.Next()
	})
	app.Delete("/room/:roomId", handler.DeleteRoom)
	app.Post("/admin/rooms/bulk/close", handler.BulkCloseRooms)

	return app, db, roomRepo
}

func roomDeleteJobs(t *testing.T, db *gorm.DB) []models.Job {
	t.Helper()

	var jobs []models.Job
	if err := db.Where("type = ?", "room_delete").Order("created_at").Find(&jobs).Error; err != nil {
		t.Fatalf("read room_delete jobs: %v", err)
	}
	return jobs
}

func bulkClose(t *testing.T, app *fiber.App, ids ...string) BulkResult {
	t.Helper()

	body, _ := json.Marshal(map[string][]string{"ids": ids})
	req := httptest.NewRequest(http.MethodPost, "/admin/rooms/bulk/close", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("bulk close: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("bulk close: want 202, got %d: %s", resp.StatusCode, b)
	}

	var result BulkResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode bulk result: %v", err)
	}
	return result
}

// An admin purge must survive a creator's archive still sitting in the queue.
// Before this, BulkCloseRooms passed no key at all, so the two jobs coexisted by
// accident; keying both on the bare room id would have fixed the accident by
// discarding the purge and reporting it as done.
func TestBulkCloseRooms_PurgeSurvivesAQueuedArchive(t *testing.T) {
	app, db, roomRepo := setupRoomDedupeApp(t)

	room, err := roomRepo.CreateRoom("creator-user", "overlap", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	req := httptest.NewRequest(http.MethodDelete, "/room/"+room.ID, http.NoBody)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("delete room: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("delete room: want 202, got %d: %s", resp.StatusCode, b)
	}

	result := bulkClose(t, app, room.ID)
	if item := result.Results[room.ID]; !item.Success {
		t.Errorf("the purge was refused by the queued archive: %+v", item)
	}

	jobs := roomDeleteJobs(t, db)
	if len(jobs) != 2 {
		t.Fatalf("want an archive job and a purge job, got %d", len(jobs))
	}

	// Pin the key shape itself. Collapsing these to one key is the regression
	// this test exists to catch, and it is invisible from job counts alone.
	if jobs[0].DedupeKey != room.ID {
		t.Errorf("archive key = %q, want the bare room id %q", jobs[0].DedupeKey, room.ID)
	}
	if want := room.ID + ":purge"; jobs[1].DedupeKey != want {
		t.Errorf("purge key = %q, want %q", jobs[1].DedupeKey, want)
	}

	var archive, purge struct {
		Purge bool `json:"purge"`
	}
	if err := json.Unmarshal([]byte(jobs[0].Payload), &archive); err != nil {
		t.Fatalf("unmarshal archive payload: %v", err)
	}
	if err := json.Unmarshal([]byte(jobs[1].Payload), &purge); err != nil {
		t.Fatalf("unmarshal purge payload: %v", err)
	}
	if archive.Purge {
		t.Error("the creator's delete should archive, not purge")
	}
	if !purge.Purge {
		t.Error("the admin's bulk close should purge, not archive")
	}
}

// A double-clicked Close button is the case the dedupe key exists for: both
// requests ask for the same purge, so the second must collapse into the first
// rather than end the meeting twice.
func TestBulkCloseRooms_RepeatCollapsesToOneJob(t *testing.T) {
	app, db, roomRepo := setupRoomDedupeApp(t)

	room, err := roomRepo.CreateRoom("creator-user", "repeat", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	for i := 1; i <= 2; i++ {
		result := bulkClose(t, app, room.ID)
		// The room is already queued for exactly this purge, so the request got
		// what it asked for — reporting it as failed would be a lie the UI would
		// not show anyway.
		if item := result.Results[room.ID]; !item.Success {
			t.Errorf("request %d reported failure: %+v", i, item)
		}
	}

	if jobs := roomDeleteJobs(t, db); len(jobs) != 1 {
		t.Errorf("want 1 purge job after two identical requests, got %d", len(jobs))
	}
}

// Duplicate ids inside a single request collapse the same way — the API accepts
// any list, and the UI is not the only caller.
func TestBulkCloseRooms_DuplicateIDsInOneRequest(t *testing.T) {
	app, db, roomRepo := setupRoomDedupeApp(t)

	room, err := roomRepo.CreateRoom("creator-user", "dupe-ids", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}

	bulkClose(t, app, room.ID, room.ID, room.ID)

	if jobs := roomDeleteJobs(t, db); len(jobs) != 1 {
		t.Errorf("want 1 purge job, got %d", len(jobs))
	}
}
