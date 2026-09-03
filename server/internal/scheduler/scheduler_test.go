package scheduler

import (
	"context"
	"errors"
	"testing"
	"time"

	"bedrud/config"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/livekit/protocol/livekit"
)

// lkConfig is the config every test that means to reach the room loop needs.
//
// checkIdleRooms mints a JWT before it calls the client, and
// auth.AccessToken.ToJWT returns ErrKeysMissing when either credential is
// empty — so a config without these two returns at the token step and never
// touches LiveKit at all, whatever client it was handed. Several tests here
// used to do exactly that while claiming to cover the loop below it.
func lkConfig() *config.LiveKitConfig {
	return &config.LiveKitConfig{Host: "mock", APIKey: "key", APISecret: "secret"}
}

// activeRoom builds a room the idle check will consider: active, and old
// enough to be past the five-minute grace period unless told otherwise.
func activeRoom(id, name string, age time.Duration, persistent bool) *models.Room {
	return &models.Room{
		ID:        id,
		Name:      name,
		CreatedBy: "user-1",
		IsActive:  true,
		CreatedAt: time.Now().Add(-age),
		Settings:  models.RoomSettings{IsPersistent: persistent},
	}
}

// assertActive reads the room back and fails if it is missing or its IsActive
// does not match. Reading with `updated != nil && ...` instead would turn a
// missing row into a pass, which is how these assertions used to be written.
func assertActive(t *testing.T, roomRepo *repository.RoomRepository, id string, want bool, msg string) {
	t.Helper()

	updated, err := roomRepo.GetRoom(id)
	if err != nil {
		t.Fatalf("GetRoom(%s): %v", id, err)
	}
	if updated == nil {
		t.Fatalf("GetRoom(%s): room is gone", id)
	}
	if updated.IsActive != want {
		t.Fatalf("%s: IsActive=%v, want %v", msg, updated.IsActive, want)
	}
}

func TestInitialize_DoesNotPanic(t *testing.T) {
	// Initialize should not panic with nil deps
	Initialize(nil, nil, nil, nil, &config.LiveKitConfig{}, &config.ServerConfig{}, nil, nil)
	// Stop should not panic either
	Stop()
}

func TestStop_BeforeInitialize(t *testing.T) {
	// scheduler is a package-level var and TestInitialize_DoesNotPanic sets it,
	// so without this reset the nil branch in Stop is never taken and the test
	// proves nothing about the order it is named for.
	saved := scheduler
	scheduler = nil
	t.Cleanup(func() { scheduler = saved })

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

// TestCheckIdleRooms_MissingCredentialsStopBeforeLiveKit covers the token step
// on its own. It is the branch several tests reached by accident, so it gets
// one test that says so and asserts the client was never called.
func TestCheckIdleRooms_MissingCredentialsStopBeforeLiveKit(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	db.Create(activeRoom("no-creds", "no-creds", 10*time.Minute, false))

	lk := testutil.NewMockRoomService()
	checkIdleRooms(roomRepo, &config.LiveKitConfig{Host: "mock"}, lk)

	if n := lk.ListRoomsCalls.Load(); n != 0 {
		t.Fatalf("expected LiveKit to be untouched without credentials, got %d ListRooms calls", n)
	}
	assertActive(t, roomRepo, "no-creds", true, "room with no LiveKit credentials")
}

// TestCheckIdleRooms_ListRoomsErrorLeavesRoomsActive covers the early return
// when LiveKit answers with an error — deterministically, instead of by
// pointing a real client at a port nothing is expected to be listening on.
func TestCheckIdleRooms_ListRoomsErrorLeavesRoomsActive(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	db.Create(activeRoom("lk-down", "lk-down", 10*time.Minute, false))

	lk := testutil.NewMockRoomService()
	lk.OnListRooms = func(context.Context, *livekit.ListRoomsRequest) (*livekit.ListRoomsResponse, error) {
		return nil, errors.New("livekit unreachable")
	}

	checkIdleRooms(roomRepo, lkConfig(), lk)

	if n := lk.ListRoomsCalls.Load(); n != 1 {
		t.Fatalf("expected exactly 1 ListRooms call, got %d", n)
	}
	assertActive(t, roomRepo, "lk-down", true, "room while LiveKit is erroring")
}

// TestCheckIdleRooms_RoomsWithinGracePeriod pairs the young room with an old
// one in the same run. Without the control the test passes whenever the loop is
// not reached at all, which is what it used to do.
func TestCheckIdleRooms_RoomsWithinGracePeriod(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	db.Create(activeRoom("grace-room", "grace-room", 0, false))
	db.Create(activeRoom("old-room", "old-room", 10*time.Minute, false))

	lk := testutil.NewMockRoomService() // ListRooms returns an empty list

	checkIdleRooms(roomRepo, lkConfig(), lk)

	assertActive(t, roomRepo, "grace-room", true, "room inside the grace period")
	assertActive(t, roomRepo, "old-room", false, "control room past the grace period")
}

// TestCheckIdleRooms_PersistentRoomSkipped is the same shape: the persistent
// room is only meaningful next to a non-persistent one that does get marked.
func TestCheckIdleRooms_PersistentRoomSkipped(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	db.Create(activeRoom("persistent-room", "persistent-room", 10*time.Minute, true))
	db.Create(activeRoom("normal-room", "normal-room", 10*time.Minute, false))

	lk := testutil.NewMockRoomService()

	checkIdleRooms(roomRepo, lkConfig(), lk)

	assertActive(t, roomRepo, "persistent-room", true, "persistent room")
	assertActive(t, roomRepo, "normal-room", false, "control non-persistent room")
}

func TestCheckIdleRooms_NonPersistentMarkedIdle_WhenLKReportsEmpty(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	db.Create(activeRoom("normal-empty-lk", "normal-empty-lk", 10*time.Minute, false))

	lk := testutil.NewMockRoomService()

	checkIdleRooms(roomRepo, lkConfig(), lk)

	assertActive(t, roomRepo, "normal-empty-lk", false, "non-persistent room with no LiveKit participants")
}

func TestCheckIdleRooms_NonPersistentNotMarked_WhenLKHasParticipants(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	db.Create(activeRoom("active-lk-room", "active-lk-room", 10*time.Minute, false))

	lk := testutil.NewMockRoomService()
	lk.OnListRooms = func(context.Context, *livekit.ListRoomsRequest) (*livekit.ListRoomsResponse, error) {
		return &livekit.ListRoomsResponse{
			Rooms: []*livekit.Room{{Name: "active-lk-room", NumParticipants: 1}},
		}, nil
	}

	checkIdleRooms(roomRepo, lkConfig(), lk)

	assertActive(t, roomRepo, "active-lk-room", true, "room LiveKit reports as occupied")
}

// TestCheckIdleRooms_ReactivatesWhenParticipantJoinsDuringCheck covers the
// re-check after SetRoomIdle: ListRooms said the room was empty, but somebody
// joined before the write landed, so the room is put back and its participants
// are left alone. Nothing reached this branch before — mockLkNoRooms answered
// 404 to ListParticipants, so the re-check always errored out.
func TestCheckIdleRooms_ReactivatesWhenParticipantJoinsDuringCheck(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	db.Create(activeRoom("racing-room", "racing-room", 10*time.Minute, false))
	db.Create(&models.RoomParticipant{
		ID:       "racing-participant",
		RoomID:   "racing-room",
		UserID:   "user-1",
		IsActive: true,
	})

	lk := testutil.NewMockRoomService() // ListRooms: empty, so the room is marked idle
	lk.OnListParticipants = func(context.Context, *livekit.ListParticipantsRequest) (*livekit.ListParticipantsResponse, error) {
		return &livekit.ListParticipantsResponse{
			Participants: []*livekit.ParticipantInfo{{Identity: "user-1"}},
		}, nil
	}

	checkIdleRooms(roomRepo, lkConfig(), lk)

	if n := lk.ListParticipantsCalls.Load(); n != 1 {
		t.Fatalf("expected the idle write to be re-checked once, got %d ListParticipants calls", n)
	}
	assertActive(t, roomRepo, "racing-room", true, "room somebody joined during the idle check")

	// Reactivating returns before DeactivateRoomParticipants, so the participant
	// that caused it must still be active.
	var p models.RoomParticipant
	if err := db.First(&p, "id = ?", "racing-participant").Error; err != nil {
		t.Fatalf("load participant: %v", err)
	}
	if !p.IsActive {
		t.Fatal("participant was deactivated even though the room was reactivated")
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

	assertActive(t, roomRepo, "expired-room", false, "expired room")
}
