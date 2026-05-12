package scheduler

import (
	"bedrud/config"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"
	"net/http"
	"testing"
	"time"

	"github.com/livekit/protocol/livekit"
)

func TestInitialize_DoesNotPanic(t *testing.T) {
	// Initialize should not panic with nil deps
	Initialize(nil, &config.LiveKitConfig{}, &config.ServerConfig{})
	// Stop should not panic either
	Stop()
}

func TestStop_BeforeInitialize(t *testing.T) {
	// Should not panic if called before Initialize
	Stop()
}

func TestCheckIdleRooms_NilRepo(t *testing.T) {
	// Should return early without panic
	checkIdleRooms(nil, &config.LiveKitConfig{}, nil)
}

func TestCheckIdleRooms_EmptyRooms(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	// No rooms in DB → should return without panic
	checkIdleRooms(roomRepo, &config.LiveKitConfig{}, nil)
}

func TestCheckIdleRooms_RoomsWithinGracePeriod(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	// Create a room that is brand new (within 5-minute grace period)
	room := &models.Room{
		ID:        "grace-room-1",
		Name:      "grace-room",
		CreatedBy: "user-1",
		IsActive:  true,
		CreatedAt: time.Now(), // just now → within grace
	}
	db.Create(room)

	// Should NOT call LiveKit nor mark idle; exits early due to grace period
	lkClient := livekit.NewRoomServiceProtobufClient("http://localhost:9999", http.DefaultClient)
	checkIdleRooms(roomRepo, &config.LiveKitConfig{Host: "http://localhost:9999"}, lkClient)

	// Room should still be active
	updated, _ := roomRepo.GetRoom("grace-room-1")
	if updated != nil && !updated.IsActive {
		t.Fatal("room within grace period should not be marked idle")
	}
}

func TestCheckIdleRooms_OldRoomLiveKitUnavailable(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	// Create a room older than 5 minutes
	room := &models.Room{
		ID:        "old-room-1",
		Name:      "old-room",
		CreatedBy: "user-1",
		IsActive:  true,
		CreatedAt: time.Now().Add(-10 * time.Minute),
	}
	db.Create(room)

	// LiveKit is unreachable — checkIdleRooms should handle this gracefully
	lkClient := livekit.NewRoomServiceProtobufClient("http://localhost:9999", http.DefaultClient)
	checkIdleRooms(roomRepo, &config.LiveKitConfig{
		Host: "http://localhost:9999", // nothing listening here
	}, lkClient)

	// Room stays active since LiveKit reported an error
	updated, _ := roomRepo.GetRoom("old-room-1")
	if updated != nil && !updated.IsActive {
		t.Fatal("room should stay active when LiveKit call fails")
	}
}

func TestCheckIdleRooms_PersistentRoomSkipped(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	room := &models.Room{
		ID:        "persistent-room-1",
		Name:      "persistent-room",
		CreatedBy: "user-1",
		IsActive:  true,
		CreatedAt: time.Now().Add(-10 * time.Minute),
		Settings:  models.RoomSettings{IsPersistent: true},
	}
	db.Create(room)

	lkClient := livekit.NewRoomServiceProtobufClient("http://localhost:9999", http.DefaultClient)
	checkIdleRooms(roomRepo, &config.LiveKitConfig{Host: "http://localhost:9999"}, lkClient)

	updated, _ := roomRepo.GetRoom("persistent-room-1")
	if updated == nil {
		t.Fatal("expected to find persistent room")
	}
	if !updated.IsActive {
		t.Fatal("persistent room should remain active regardless of participant count")
	}
}

func TestCheckIdleRooms_NonPersistentRoomUnchangedOnLKUnavailable(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	room := &models.Room{
		ID:        "normal-room-1",
		Name:      "normal-room",
		CreatedBy: "user-1",
		IsActive:  true,
		CreatedAt: time.Now().Add(-10 * time.Minute),
		Settings:  models.RoomSettings{IsPersistent: false},
	}
	db.Create(room)

	lkClient := livekit.NewRoomServiceProtobufClient("http://localhost:9999", http.DefaultClient)
	checkIdleRooms(roomRepo, &config.LiveKitConfig{Host: "http://localhost:9999"}, lkClient)

	updated, _ := roomRepo.GetRoom("normal-room-1")
	if updated != nil && !updated.IsActive {
		t.Fatal("non-persistent room should stay active when LiveKit is unavailable (scheduler returns early on LK error)")
	}
}

func TestCheckIdleRooms_PersistentSkipWorksOnLKUnavailable(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	persistentRoom := &models.Room{
		ID:        "mixed-persistent",
		Name:      "mixed-persistent",
		CreatedBy: "user-1",
		IsActive:  true,
		CreatedAt: time.Now().Add(-10 * time.Minute),
		Settings:  models.RoomSettings{IsPersistent: true},
	}
	normalRoom := &models.Room{
		ID:        "mixed-normal",
		Name:      "mixed-normal",
		CreatedBy: "user-1",
		IsActive:  true,
		CreatedAt: time.Now().Add(-10 * time.Minute),
		Settings:  models.RoomSettings{IsPersistent: false},
	}
	db.Create(persistentRoom)
	db.Create(normalRoom)

	lkClient := livekit.NewRoomServiceProtobufClient("http://localhost:9999", http.DefaultClient)
	checkIdleRooms(roomRepo, &config.LiveKitConfig{Host: "http://localhost:9999"}, lkClient)

	// Both rooms stay active because LiveKit is unavailable — scheduler returns early on LK error.
	// This test verifies the persistent skip doesn't panic and the flow completes.
	persisted, _ := roomRepo.GetRoom("mixed-persistent")
	if persisted == nil || !persisted.IsActive {
		t.Fatal("persistent room should remain active")
	}
	normal, _ := roomRepo.GetRoom("mixed-normal")
	if normal == nil || !normal.IsActive {
		t.Fatal("non-persistent room should stay active when LiveKit is unavailable")
	}
}
