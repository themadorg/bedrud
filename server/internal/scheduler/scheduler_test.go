package scheduler

import (
	"bedrud/config"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/livekit/protocol/livekit"
	"google.golang.org/protobuf/proto"
)

// mockLkNoRooms returns an httptest.Server that responds to
// ListRooms with an empty room list (simulating 0 participants).
func mockLkNoRooms() *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/twirp/livekit.RoomService/ListRooms" {
			resp := &livekit.ListRoomsResponse{}
			data, _ := proto.Marshal(resp)
			w.Header().Set("Content-Type", "application/protobuf")
			w.Write(data)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

// mockLkRoom returns an httptest.Server that responds to
// ListRooms with a single room having the given participant count.
func mockLkRoom(roomName string, numParticipants uint32) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/twirp/livekit.RoomService/ListRooms" {
			resp := &livekit.ListRoomsResponse{
				Rooms: []*livekit.Room{
					{Name: roomName, NumParticipants: numParticipants},
				},
			}
			data, _ := proto.Marshal(resp)
			w.Header().Set("Content-Type", "application/protobuf")
			w.Write(data)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
}

func TestInitialize_DoesNotPanic(t *testing.T) {
	// Initialize should not panic with nil deps
	Initialize(nil, nil, &config.LiveKitConfig{}, &config.ServerConfig{})
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

func TestCheckIdleRooms_PersistentRoomStaysActive_WhenLKReportsEmpty(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	room := &models.Room{
		ID:        "persist-empty-lk",
		Name:      "persist-empty-lk",
		CreatedBy: "user-1",
		IsActive:  true,
		CreatedAt: time.Now().Add(-10 * time.Minute),
		Settings:  models.RoomSettings{IsPersistent: true},
	}
	db.Create(room)

	mockLK := mockLkNoRooms()
	defer mockLK.Close()

	lkClient := livekit.NewRoomServiceProtobufClient(mockLK.URL, http.DefaultClient)
	checkIdleRooms(roomRepo, &config.LiveKitConfig{Host: mockLK.URL, APIKey: "key", APISecret: "secret"}, lkClient)

	updated, _ := roomRepo.GetRoom("persist-empty-lk")
	if updated == nil || !updated.IsActive {
		t.Fatal("persistent room should remain active when LK reports 0 participants")
	}
}

func TestCheckIdleRooms_NonPersistentMarkedIdle_WhenLKReportsEmpty(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	room := &models.Room{
		ID:        "normal-empty-lk",
		Name:      "normal-empty-lk",
		CreatedBy: "user-1",
		IsActive:  true,
		CreatedAt: time.Now().Add(-10 * time.Minute),
		Settings:  models.RoomSettings{IsPersistent: false},
	}
	db.Create(room)

	mockLK := mockLkNoRooms()
	defer mockLK.Close()

	lkClient := livekit.NewRoomServiceProtobufClient(mockLK.URL, http.DefaultClient)
	checkIdleRooms(roomRepo, &config.LiveKitConfig{Host: mockLK.URL, APIKey: "key", APISecret: "secret"}, lkClient)

	updated, _ := roomRepo.GetRoom("normal-empty-lk")
	if updated == nil {
		t.Fatal("expected room to still exist after being marked idle")
	}
	if updated.IsActive {
		t.Fatal("non-persistent room should be marked idle when LK reports 0 participants")
	}
}

func TestCheckIdleRooms_NonPersistentNotMarked_WhenLKHasParticipants(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	room := &models.Room{
		ID:        "active-lk-room",
		Name:      "active-lk-room",
		CreatedBy: "user-1",
		IsActive:  true,
		CreatedAt: time.Now().Add(-10 * time.Minute),
		Settings:  models.RoomSettings{IsPersistent: false},
	}
	db.Create(room)

	mockLK := mockLkRoom("active-lk-room", 1)
	defer mockLK.Close()

	lkClient := livekit.NewRoomServiceProtobufClient(mockLK.URL, http.DefaultClient)
	checkIdleRooms(roomRepo, &config.LiveKitConfig{Host: mockLK.URL, APIKey: "key", APISecret: "secret"}, lkClient)

	updated, _ := roomRepo.GetRoom("active-lk-room")
	if updated == nil || !updated.IsActive {
		t.Fatal("non-persistent room with active participants should NOT be marked idle")
	}
}

func TestScheduler_CleanupExpiredRooms_Integration(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	// Create expired non-persistent room
	room := &models.Room{
		ID: "expired-room", Name: "expired-room", CreatedBy: "user",
		IsActive: true, ExpiresAt: time.Now().Add(-1 * time.Hour),
		Settings: models.RoomSettings{IsPersistent: false},
	}
	db.Create(room)

	// Call the same logic scheduler would
	_ = roomRepo.CleanupExpiredRooms()

	updated, _ := roomRepo.GetRoom("expired-room")
	if updated != nil && updated.IsActive {
		t.Fatal("expired room should be marked inactive")
	}
}
