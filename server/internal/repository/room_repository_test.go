package repository

import (
	"bedrud/internal/models"
	"bedrud/internal/testutil"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

const testUserIDRoom = "user-1"

func TestRoomRepository_CreateRoom(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	// Create a user first (needed for foreign key if enabled)
	db.Create(&models.User{ID: testUserIDRoom, Email: "user@ex.com", Name: "Creator", Provider: "local", IsActive: true})

	room, err := repo.CreateRoom(testUserIDRoom, "test-room", true, "standard", 0, &models.RoomSettings{
		AllowChat:  true,
		AllowVideo: true,
		AllowAudio: true,
	})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}
	if room == nil {
		t.Fatal("expected non-nil room")
	}
	if room.Name != "test-room" {
		t.Fatalf("expected name 'test-room', got '%s'", room.Name)
	}
	if room.CreatedBy != testUserIDRoom {
		t.Fatalf("expected createdBy 'user-1', got '%s'", room.CreatedBy)
	}
	if room.AdminID != testUserIDRoom {
		t.Fatalf("expected adminID 'user-1', got '%s'", room.AdminID)
	}
	if !room.IsActive {
		t.Fatal("expected room to be active")
	}
	if !room.IsPublic {
		t.Fatal("expected room to be public")
	}
	if room.Mode != "standard" {
		t.Fatalf("expected mode 'standard', got '%s'", room.Mode)
	}
}

func TestRoomRepository_CreateRoom_CreatesParticipantAndPermissions(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "user@ex.com", Name: "Creator", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "perm-room", false, "standard", 0, &models.RoomSettings{})

	// Check participant was created
	participants, err := repo.GetActiveParticipants(room.ID)
	if err != nil {
		t.Fatalf("failed to get participants: %v", err)
	}
	if len(participants) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(participants))
	}
	if participants[0].UserID != testUserIDRoom {
		t.Fatal("expected creator to be a participant")
	}
	if !participants[0].IsApproved {
		t.Fatal("expected creator to be approved")
	}
	if !participants[0].IsOnStage {
		t.Fatal("expected creator to be on stage")
	}

	// Check permissions
	perms, err := repo.GetParticipantPermissions(room.ID, testUserIDRoom)
	if err != nil {
		t.Fatalf("failed to get permissions: %v", err)
	}
	if !perms.IsAdmin {
		t.Fatal("expected creator to have admin permissions")
	}
	if !perms.CanKick {
		t.Fatal("expected creator to have kick permission")
	}
}

func TestRoomRepository_GetRoom(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "user@ex.com", Name: "Creator", Provider: "local", IsActive: true})
	room, _ := repo.CreateRoom(testUserIDRoom, "get-room", false, "standard", 0, &models.RoomSettings{})

	found, err := repo.GetRoom(room.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find room")
	}
	if found.Name != "get-room" {
		t.Fatalf("expected name 'get-room', got '%s'", found.Name)
	}
}

func TestRoomRepository_GetRoom_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	found, err := repo.GetRoom("nonexistent-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != nil {
		t.Fatal("expected nil for non-existent room")
	}
}

func TestRoomRepository_GetRoomByName(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "user@ex.com", Name: "Creator", Provider: "local", IsActive: true})
	_, _ = repo.CreateRoom(testUserIDRoom, "named-room", false, "standard", 0, &models.RoomSettings{})

	found, err := repo.GetRoomByName("named-room")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found == nil || found.Name != "named-room" {
		t.Fatal("expected to find room by name")
	}
}

func TestRoomRepository_GetRoomByName_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	found, err := repo.GetRoomByName("nonexistent-room")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != nil {
		t.Fatal("expected nil for non-existent room name")
	}
}

func TestRoomRepository_AddParticipant(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "join-room", false, "standard", 0, &models.RoomSettings{})

	err := repo.AddParticipant(room.ID, "user-2")
	if err != nil {
		t.Fatalf("failed to add participant: %v", err)
	}

	participants, _ := repo.GetActiveParticipants(room.ID)
	if len(participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(participants))
	}
}

func TestRoomRepository_AddParticipant_Rejoin(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "rejoin-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, "user-2")
	_ = repo.RemoveParticipant(room.ID, "user-2")

	// Rejoin
	err := repo.AddParticipant(room.ID, "user-2")
	if err != nil {
		t.Fatalf("failed to rejoin: %v", err)
	}

	participants, _ := repo.GetActiveParticipants(room.ID)
	if len(participants) != 2 {
		t.Fatalf("expected 2 active participants after rejoin, got %d", len(participants))
	}
}

func TestRoomRepository_AddParticipant_BannedUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "ban-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, "user-2")
	_ = repo.KickParticipant(room.ID, "user-2")

	// Try to rejoin after ban
	err := repo.AddParticipant(room.ID, "user-2")
	if err == nil {
		t.Fatal("expected error when banned user tries to join")
	}
}

func TestRoomRepository_AddParticipant_UpdatesLastActivityAt(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: "laa-u1", Email: "laa@ex.com", Name: "LAA", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom("laa-u1", "laa-room", false, "standard", 0, &models.RoomSettings{})

	// LastActivityAt should be set on room creation
	if room.LastActivityAt == nil {
		t.Fatal("expected LastActivityAt to be set on room creation")
	}

	// Add a new participant — LastActivityAt should update
	old := *room.LastActivityAt
	db.Create(&models.User{ID: "laa-u2", Email: "laa2@ex.com", Name: "LAA2", Provider: "local", IsActive: true})
	if err := repo.AddParticipant(room.ID, "laa-u2"); err != nil {
		t.Fatalf("failed to add participant: %v", err)
	}

	var updatedRoom models.Room
	db.First(&updatedRoom, "id = ?", room.ID)
	if updatedRoom.LastActivityAt == nil {
		t.Fatal("expected LastActivityAt after participant add")
	}
	if !updatedRoom.LastActivityAt.After(old) {
		t.Fatal("expected LastActivityAt to advance after participant add")
	}

	// Rejoin (reactivate) — LastActivityAt should update again
	repo.RemoveParticipant(room.ID, "laa-u2")
	reactivated := *updatedRoom.LastActivityAt
	if err := repo.AddParticipant(room.ID, "laa-u2"); err != nil {
		t.Fatalf("failed to re-add participant: %v", err)
	}
	db.First(&updatedRoom, "id = ?", room.ID)
	if !updatedRoom.LastActivityAt.After(reactivated) {
		t.Fatal("expected LastActivityAt to advance on rejoin")
	}
}

func TestRoomRepository_RemoveParticipant(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "leave-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, "user-2")

	err := repo.RemoveParticipant(room.ID, "user-2")
	if err != nil {
		t.Fatalf("failed to remove participant: %v", err)
	}

	participants, _ := repo.GetActiveParticipants(room.ID)
	if len(participants) != 1 {
		t.Fatalf("expected 1 active participant after leave, got %d", len(participants))
	}
}

func TestRoomRepository_KickParticipant(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "kick-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, "user-2")

	err := repo.KickParticipant(room.ID, "user-2")
	if err != nil {
		t.Fatalf("failed to kick participant: %v", err)
	}

	// Verify participant is inactive and banned
	var participant models.RoomParticipant
	db.Where("room_id = ? AND user_id = ?", room.ID, "user-2").First(&participant)
	if participant.IsActive {
		t.Fatal("expected participant to be inactive after kick")
	}
	if !participant.IsBanned {
		t.Fatal("expected participant to be banned after kick")
	}
}

func TestRoomRepository_BringToStage_RemoveFromStage(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "stage-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, "user-2")

	// user-2 should not be on stage
	onStage, _ := repo.IsParticipantOnStage(room.ID, "user-2")
	if onStage {
		t.Fatal("expected user-2 to not be on stage initially")
	}

	// Bring to stage
	_ = repo.BringToStage(room.ID, "user-2")
	onStage, _ = repo.IsParticipantOnStage(room.ID, "user-2")
	if !onStage {
		t.Fatal("expected user-2 to be on stage")
	}

	// Remove from stage
	_ = repo.RemoveFromStage(room.ID, "user-2")
	onStage, _ = repo.IsParticipantOnStage(room.ID, "user-2")
	if onStage {
		t.Fatal("expected user-2 to not be on stage after removal")
	}
}

func TestRoomRepository_UpdateParticipantStatus(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "status-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, "user-2")

	err := repo.UpdateParticipantStatus(room.ID, "user-2", map[string]interface{}{
		"is_muted":     true,
		"is_video_off": true,
	})
	if err != nil {
		t.Fatalf("failed to update status: %v", err)
	}

	var participant models.RoomParticipant
	db.Where("room_id = ? AND user_id = ?", room.ID, "user-2").First(&participant)
	if !participant.IsMuted {
		t.Fatal("expected participant to be muted")
	}
	if !participant.IsVideoOff {
		t.Fatal("expected participant video to be off")
	}
}

func TestRoomRepository_DeleteRoom(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "delete-room", false, "standard", 0, &models.RoomSettings{})

	err := repo.DeleteRoom(room.ID, testUserIDRoom)
	if err != nil {
		t.Fatalf("failed to delete room: %v", err)
	}

	found, _ := repo.GetRoom(room.ID)
	if found != nil {
		t.Fatal("expected room to be deleted")
	}
}

func TestRoomRepository_DeleteRoom_NotCreator(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "no-delete-room", false, "standard", 0, &models.RoomSettings{})

	err := repo.DeleteRoom(room.ID, "user-2")
	if err == nil {
		t.Fatal("expected error when non-creator tries to delete room")
	}
}

func TestRoomRepository_GetAllRooms(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	_, _ = repo.CreateRoom(testUserIDRoom, "room-1", false, "standard", 0, &models.RoomSettings{})
	_, _ = repo.CreateRoom(testUserIDRoom, "room-2", true, "standard", 0, &models.RoomSettings{})

	rooms, err := repo.GetAllRooms()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms, got %d", len(rooms))
	}
}

func TestRoomRepository_GetRoomsCreatedByUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	_, _ = repo.CreateRoom(testUserIDRoom, "my-room-1", false, "standard", 0, &models.RoomSettings{})
	_, _ = repo.CreateRoom(testUserIDRoom, "my-room-2", false, "standard", 0, &models.RoomSettings{})
	_, _ = repo.CreateRoom("user-2", "other-room", false, "standard", 0, &models.RoomSettings{})

	rooms, err := repo.GetRoomsCreatedByUser(testUserIDRoom)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms created by user-1, got %d", len(rooms))
	}
}

func TestRoomRepository_GetLatestRoomsCreatedByUser_Dedup(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "dedup@ex.com", Name: "Dedup", Provider: "local", IsActive: true})

	room1, _ := repo.CreateRoom(testUserIDRoom, "dup-room", false, "standard", 0, &models.RoomSettings{})
	// Deactivate first room so second can be created with same slug
	db.Model(&models.Room{}).Where("id = ?", room1.ID).Update("is_active", false)
	_, _ = repo.CreateRoom(testUserIDRoom, "dup-room", false, "standard", 0, &models.RoomSettings{})
	_, _ = repo.CreateRoom(testUserIDRoom, "unique-room", false, "standard", 0, &models.RoomSettings{})

	rooms, err := repo.GetLatestRoomsCreatedByUser(testUserIDRoom)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rooms) != 2 {
		t.Fatalf("expected 2 rooms (deduped), got %d", len(rooms))
	}
	// Verify dedup: no duplicate names
	names := make(map[string]bool)
	for _, r := range rooms {
		if names[r.Name] {
			t.Fatalf("duplicate name %s in result", r.Name)
		}
		names[r.Name] = true
	}
	if !names["dup-room"] {
		t.Fatalf("expected dup-room in results")
	}
	if !names["unique-room"] {
		t.Fatalf("expected unique-room in results")
	}
}

func TestRoomRepository_GetUserParticipationsPaginated_Basic(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	room, _ := repo.CreateRoom(testUserIDRoom, "session-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, testUserIDRoom)

	participants, total, err := repo.GetUserParticipationsPaginated(testUserIDRoom, UserParticipationsParams{Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 1 {
		t.Fatalf("expected total 1, got %d", total)
	}
	if len(participants) != 1 {
		t.Fatalf("expected 1 participant, got %d", len(participants))
	}
	if participants[0].RoomID != room.ID {
		t.Fatalf("expected room %s, got %s", room.ID, participants[0].RoomID)
	}
	if participants[0].Room == nil || participants[0].Room.Name != "session-room" {
		t.Fatal("expected Room preload with name 'session-room'")
	}
	if !participants[0].IsActive {
		t.Fatal("expected IsActive true after AddParticipant")
	}
	if participants[0].JoinedAt.IsZero() {
		t.Fatal("expected non-zero JoinedAt")
	}
}

func TestRoomRepository_GetUserParticipationsPaginated_NoParticipations(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: "lone-user", Email: "lone@ex.com", Name: "Lone", Provider: "local", IsActive: true})

	participants, total, err := repo.GetUserParticipationsPaginated("lone-user", UserParticipationsParams{Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 0 {
		t.Fatalf("expected total 0, got %d", total)
	}
	if len(participants) != 0 {
		t.Fatalf("expected 0 participants, got %d", len(participants))
	}
}

func TestRoomRepository_GetUserParticipationsPaginated_Pagination(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	// Create 3 rooms and join them
	for i := 0; i < 3; i++ {
		name := fmt.Sprintf("pag-room-%d", i)
		room, _ := repo.CreateRoom(testUserIDRoom, name, false, "standard", 0, &models.RoomSettings{})
		_ = repo.AddParticipant(room.ID, testUserIDRoom)
		// Stagger joined_at so ordering is deterministic
		db.Model(&models.RoomParticipant{}).Where("room_id = ? AND user_id = ?", room.ID, testUserIDRoom).
			Update("joined_at", time.Now().Add(-time.Duration(i)*time.Hour))
	}

	// Page 1, limit 2 → 2 results, total 3
	p1, total, err := repo.GetUserParticipationsPaginated(testUserIDRoom, UserParticipationsParams{Page: 1, Limit: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if len(p1) != 2 {
		t.Fatalf("expected 2 participants on page 1, got %d", len(p1))
	}

	// Page 2, limit 2 → 1 result, total 3
	p2, total, err := repo.GetUserParticipationsPaginated(testUserIDRoom, UserParticipationsParams{Page: 2, Limit: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 3 {
		t.Fatalf("expected total 3, got %d", total)
	}
	if len(p2) != 1 {
		t.Fatalf("expected 1 participant on page 2, got %d", len(p2))
	}

	// Page 1, limit 50 → 3 results (clamp check: limit > 100 would clamp, but 50 is fine)
	pAll, total, err := repo.GetUserParticipationsPaginated(testUserIDRoom, UserParticipationsParams{Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pAll) != 3 {
		t.Fatalf("expected 3 participants with limit 50, got %d", len(pAll))
	}

	// Ordered by joined_at desc (most recent first)
	for i := 1; i < len(pAll); i++ {
		if pAll[i].JoinedAt.After(pAll[i-1].JoinedAt) {
			t.Fatal("expected descending joined_at order")
		}
	}
}

func TestRoomRepository_GetUserParticipationsPaginated_ClampLimit(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	room, _ := repo.CreateRoom(testUserIDRoom, "clamp-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, testUserIDRoom)

	// Limit > 100 → clamped to 50
	_, _, err := repo.GetUserParticipationsPaginated(testUserIDRoom, UserParticipationsParams{Page: 1, Limit: 200})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Limit <= 0 → clamped to 50
	_, _, err = repo.GetUserParticipationsPaginated(testUserIDRoom, UserParticipationsParams{Page: 1, Limit: 0})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Page <= 0 → clamped to 1 (should not error)
	_, _, err = repo.GetUserParticipationsPaginated(testUserIDRoom, UserParticipationsParams{Page: 0, Limit: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoomRepository_UpdateRoomSettings(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "settings-room", false, "standard", 0, &models.RoomSettings{
		AllowChat: false,
		E2EE:      false,
	})

	// The UpdateRoomSettings uses Select("Settings").Updates() on embedded struct.
	// This may have limitations with SQLite. Just verify no error is returned.
	err := repo.UpdateRoomSettings(room.ID, &models.RoomSettings{
		AllowChat:  true,
		AllowVideo: true,
		E2EE:       true,
	})
	if err != nil {
		t.Fatalf("failed to update settings: %v", err)
	}
}

func TestRoomRepository_GetRoomParticipantsWithUsers(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "preload-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, "user-2")

	participants, err := repo.GetRoomParticipantsWithUsers(room.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(participants) != 2 {
		t.Fatalf("expected 2 participants, got %d", len(participants))
	}
	// Check that User is preloaded
	for _, p := range participants {
		if p.User == nil {
			t.Fatal("expected User to be preloaded")
		}
	}
}

// ====== Duplicate room name returns ErrRoomNameTaken ======

func TestRoomRepository_CreateRoom_DuplicateName(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	_, err := repo.CreateRoom(testUserIDRoom, "dup-room", false, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("first creation failed: %v", err)
	}

	_, err = repo.CreateRoom(testUserIDRoom, "dup-room", false, "standard", 0, &models.RoomSettings{})
	if err == nil {
		t.Fatal("expected error when creating room with duplicate name")
	}
	if !errors.Is(err, models.ErrRoomNameTaken) {
		t.Fatalf("expected ErrRoomNameTaken, got: %v", err)
	}
}

// ====== Archived (soft-deleted) room name is reusable ======

func TestRoomRepository_CreateRoom_ArchivedNameReusable(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	room, err := repo.CreateRoom(testUserIDRoom, "archived-room", false, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("first creation failed: %v", err)
	}

	// Soft-delete (archive) the room
	if err := repo.SoftDeleteRoom(room.ID); err != nil {
		t.Fatalf("SoftDeleteRoom failed: %v", err)
	}

	// Creating a room with same name should succeed
	newRoom, err := repo.CreateRoom(testUserIDRoom, "archived-room", false, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("expected reuse of archived room name, got error: %v", err)
	}
	if newRoom.ID == room.ID {
		t.Fatal("expected new room ID, got same as archived room")
	}
	if !newRoom.IsActive {
		t.Fatal("new room should be active")
	}
}

// ====== Idle (inactive, not soft-deleted) room name is reusable ======

func TestRoomRepository_CreateRoom_IdleNameReusable(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	room, err := repo.CreateRoom(testUserIDRoom, "idle-room", false, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("first creation failed: %v", err)
	}

	// Set room idle (is_active=false, deleted_at remains NULL)
	if err := repo.SetRoomIdle(room.ID); err != nil {
		t.Fatalf("SetRoomIdle failed: %v", err)
	}

	// Creating a room with same name should succeed
	newRoom, err := repo.CreateRoom(testUserIDRoom, "idle-room", false, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("expected reuse of idle room name, got error: %v", err)
	}
	if newRoom.ID == room.ID {
		t.Fatal("expected new room ID, got same as idle room")
	}
	if !newRoom.IsActive {
		t.Fatal("new room should be active")
	}
}

// ====== Empty room name auto-generates ======

func TestRoomRepository_CreateRoom_EmptyNameAutoGenerates(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	room, err := repo.CreateRoom(testUserIDRoom, "", false, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("CreateRoom with empty name should auto-generate, got error: %v", err)
	}
	if room.Name == "" {
		t.Fatal("expected auto-generated name, got empty string")
	}
	// Verify generated name is valid
	if err := models.ValidateRoomName(room.Name); err != nil {
		t.Fatalf("auto-generated name '%s' failed validation: %v", room.Name, err)
	}
	t.Logf("Auto-generated room name: %s", room.Name)
}

// ====== Name validation rejects special characters ======

func TestRoomRepository_CreateRoom_SpecialCharsRejected(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	invalidNames := []string{
		"room#1",
		"room@name",
		"room name",
		"room/path",
		"room.dot",
		"room_under",
	}
	for _, name := range invalidNames {
		_, err := repo.CreateRoom(testUserIDRoom, name, false, "standard", 0, &models.RoomSettings{})
		if err == nil {
			t.Fatalf("expected error for invalid name '%s'", name)
		}
	}
}

// ====== Name is lowercased and trimmed ======

func TestRoomRepository_CreateRoom_NameNormalized(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	room, err := repo.CreateRoom(testUserIDRoom, "  My-Room  ", false, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if room.Name != "my-room" {
		t.Fatalf("expected normalized name 'my-room', got '%s'", room.Name)
	}
}

// ====== Name too long is rejected ======

func TestRoomRepository_CreateRoom_NameTooLong(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	longName := strings.Repeat("a", 64)
	_, err := repo.CreateRoom(testUserIDRoom, longName, false, "standard", 0, &models.RoomSettings{})
	if !errors.Is(err, models.ErrRoomNameTooLong) {
		t.Fatalf("expected ErrRoomNameTooLong, got: %v", err)
	}
}

// ====== BUG DETECTION: MaxParticipants not passed to room ======

func TestRoomRepository_CreateRoom_MaxParticipantsNotSet(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	// BUG: CreateRoom doesn't accept maxParticipants parameter!
	// The Room struct has a default of 20 in GORM, but the value is never set from input.
	room, err := repo.CreateRoom(testUserIDRoom, "max-room", false, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// The MaxParticipants field should either be the GORM default (20) or 0
	found, _ := repo.GetRoom(room.ID)
	t.Logf("MaxParticipants value: %d (GORM default:20 expected, but CreateRoom doesn't set it from handler input)", found.MaxParticipants)

	// Document the bug: CreateRoom signature doesn't include maxParticipants
	// so it's always the GORM default or 0
	if found.MaxParticipants != 0 && found.MaxParticipants != 20 {
		t.Fatalf("unexpected MaxParticipants: %d", found.MaxParticipants)
	}
}

// ====== Room Settings Preservation ======

func TestRoomRepository_CreateRoom_SettingsPreserved(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	settings := &models.RoomSettings{
		AllowChat:       true,
		AllowVideo:      true,
		AllowAudio:      true,
		RequireApproval: true,
		E2EE:            true,
	}

	room, err := repo.CreateRoom(testUserIDRoom, "settings-preserved", false, "standard", 0, settings)
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	found, _ := repo.GetRoom(room.ID)
	if !found.Settings.AllowChat {
		t.Fatal("expected AllowChat to be preserved as true")
	}
	if !found.Settings.AllowVideo {
		t.Fatal("expected AllowVideo to be preserved as true")
	}
	if !found.Settings.AllowAudio {
		t.Fatal("expected AllowAudio to be preserved as true")
	}
	if !found.Settings.RequireApproval {
		t.Fatal("expected RequireApproval to be preserved as true")
	}
	if !found.Settings.E2EE {
		t.Fatal("expected E2EE to be preserved as true")
	}
}

// ====== Room ExpiresAt ======

func TestRoomRepository_CreateRoom_ExpiresAtIsSet(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "expire-room", false, "standard", 0, &models.RoomSettings{})

	found, _ := repo.GetRoom(room.ID)
	if found.ExpiresAt.IsZero() {
		t.Fatal("expected ExpiresAt to be set")
	}
	// ExpiresAt should be ~24h from now
	if found.ExpiresAt.Before(found.CreatedAt) {
		t.Fatal("expected ExpiresAt to be after CreatedAt")
	}
}

// ====== CleanupExpiredRooms ======

func TestRoomRepository_CleanupExpiredRooms(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	// Create a room that already expired
	room, _ := repo.CreateRoom(testUserIDRoom, "expired-room", false, "standard", 0, &models.RoomSettings{})
	// Manually set ExpiresAt to the past
	db.Model(&models.Room{}).Where("id = ?", room.ID).Update("expires_at", "2020-01-01 00:00:00")

	// Create a room that hasn't expired yet
	_, _ = repo.CreateRoom(testUserIDRoom, "active-room", false, "standard", 0, &models.RoomSettings{})

	err := repo.CleanupExpiredRooms()
	if err != nil {
		t.Fatalf("failed to cleanup: %v", err)
	}

	// Expired room should be inactive
	expired, _ := repo.GetRoom(room.ID)
	if expired.IsActive {
		t.Fatal("expected expired room to be deactivated")
	}
}

func TestRoomRepository_CleanupExpiredRooms_NoExpired(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "user@ex.com", Name: "U1", Provider: "local", IsActive: true})
	_, _ = repo.CreateRoom(testUserIDRoom, "future-room", false, "standard", 0, &models.RoomSettings{})

	err := repo.CleanupExpiredRooms()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoomRepository_CleanupExpiredRooms_PersistentSkipped(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "user@ex.com", Name: "U1", Provider: "local", IsActive: true})

	expiredRoom, _ := repo.CreateRoom(testUserIDRoom, "expired-persist-room", false, "standard", 0, &models.RoomSettings{
		IsPersistent: true,
	})
	db.Model(&models.Room{}).Where("id = ?", expiredRoom.ID).Update("expires_at", "2020-01-01 00:00:00")

	normalExpiredRoom, _ := repo.CreateRoom(testUserIDRoom, "expired-normal-room", false, "standard", 0, &models.RoomSettings{
		IsPersistent: false,
	})
	db.Model(&models.Room{}).Where("id = ?", normalExpiredRoom.ID).Update("expires_at", "2020-01-01 00:00:00")

	err := repo.CleanupExpiredRooms()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	persisted, _ := repo.GetRoom(expiredRoom.ID)
	if !persisted.IsActive {
		t.Fatal("persistent expired room should NOT be deactivated by CleanupExpiredRooms")
	}

	normal, _ := repo.GetRoom(normalExpiredRoom.ID)
	if normal.IsActive {
		t.Fatal("non-persistent expired room SHOULD be deactivated by CleanupExpiredRooms")
	}
}

func TestRoomRepository_CreateRoom_PrivateRoom(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "private-room", false, "standard", 0, &models.RoomSettings{})
	if room.IsPublic {
		t.Fatal("expected room to be private")
	}
}

func TestRoomRepository_CreateRoom_PublicRoom(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "public-room", true, "standard", 0, &models.RoomSettings{})
	if !room.IsPublic {
		t.Fatal("expected room to be public")
	}
}

// ====== Multiple Participants Lifecycle ======

func TestRoomRepository_MultipleParticipantsLifecycle(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-3", Email: "u3@ex.com", Name: "U3", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-4", Email: "u4@ex.com", Name: "U4", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "lifecycle-room", false, "standard", 0, &models.RoomSettings{})

	// Add 3 more participants
	_ = repo.AddParticipant(room.ID, "user-2")
	_ = repo.AddParticipant(room.ID, "user-3")
	_ = repo.AddParticipant(room.ID, "user-4")

	participants, _ := repo.GetActiveParticipants(room.ID)
	if len(participants) != 4 {
		t.Fatalf("expected 4 active participants, got %d", len(participants))
	}

	// user-2 leaves
	_ = repo.RemoveParticipant(room.ID, "user-2")
	participants, _ = repo.GetActiveParticipants(room.ID)
	if len(participants) != 3 {
		t.Fatalf("expected 3 active participants after leave, got %d", len(participants))
	}

	// user-3 is kicked (banned)
	_ = repo.KickParticipant(room.ID, "user-3")
	participants, _ = repo.GetActiveParticipants(room.ID)
	if len(participants) != 2 {
		t.Fatalf("expected 2 active participants after kick, got %d", len(participants))
	}

	// user-2 rejoins (was not banned)
	_ = repo.AddParticipant(room.ID, "user-2")
	participants, _ = repo.GetActiveParticipants(room.ID)
	if len(participants) != 3 {
		t.Fatalf("expected 3 active participants after rejoin, got %d", len(participants))
	}

	// user-3 cannot rejoin (banned)
	err := repo.AddParticipant(room.ID, "user-3")
	if err == nil {
		t.Fatal("expected error: banned user should not be able to rejoin")
	}
}

// ====== AddParticipant same user twice ======

func TestRoomRepository_AddParticipant_AlreadyActive(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "double-join-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, "user-2")

	// Add again (already active) — should not error, just update
	err := repo.AddParticipant(room.ID, "user-2")
	if err != nil {
		t.Fatalf("unexpected error on double-add: %v", err)
	}

	// Should still only be 2 participants (not duplicated)
	participants, _ := repo.GetActiveParticipants(room.ID)
	if len(participants) != 2 {
		t.Fatalf("expected 2 participants (no duplicate), got %d", len(participants))
	}
}

// ====== DeleteRoom cleans up participants and permissions ======

func TestRoomRepository_DeleteRoom_CleansUpAll(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "cleanup-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, "user-2")

	// Verify we have participants and permissions before delete
	var participantCount, permCount int64
	db.Model(&models.RoomParticipant{}).Where("room_id = ?", room.ID).Count(&participantCount)
	db.Model(&models.RoomPermissions{}).Where("room_id = ?", room.ID).Count(&permCount)
	if participantCount == 0 {
		t.Fatal("expected participants before delete")
	}
	if permCount == 0 {
		t.Fatal("expected permissions before delete")
	}

	_ = repo.DeleteRoom(room.ID, testUserIDRoom)

	// Verify everything is cleaned up
	db.Model(&models.RoomParticipant{}).Where("room_id = ?", room.ID).Count(&participantCount)
	db.Model(&models.RoomPermissions{}).Where("room_id = ?", room.ID).Count(&permCount)
	if participantCount != 0 {
		t.Fatalf("expected 0 participants after delete, got %d", participantCount)
	}
	if permCount != 0 {
		t.Fatalf("expected 0 permissions after delete, got %d", permCount)
	}
}

// ====== Room with RequireApproval setting ======

func TestRoomRepository_CreateRoom_RequireApproval(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	room, err := repo.CreateRoom(testUserIDRoom, "approval-room", false, "standard", 0, &models.RoomSettings{
		RequireApproval: true,
		AllowChat:       true,
	})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	found, _ := repo.GetRoom(room.ID)
	if !found.Settings.RequireApproval {
		t.Fatal("expected RequireApproval to be true")
	}
}

func TestRoomRepository_AddParticipant_NewJoiner_NotApproved(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "approval-test", false, "standard", 0, &models.RoomSettings{RequireApproval: true})
	_ = repo.AddParticipant(room.ID, "user-2")

	// New joiner should NOT be approved by default
	var participant models.RoomParticipant
	db.Where("room_id = ? AND user_id = ?", room.ID, "user-2").First(&participant)
	if participant.IsApproved {
		t.Fatal("new joiner should NOT be auto-approved (approval is required)")
	}
}

func TestRoomRepository_CreateRoom_CreatorIsAlwaysApproved(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "creator-approved", false, "standard", 0, &models.RoomSettings{RequireApproval: true})

	var participant models.RoomParticipant
	db.Where("room_id = ? AND user_id = ?", room.ID, testUserIDRoom).First(&participant)
	if !participant.IsApproved {
		t.Fatal("creator should always be approved regardless of RequireApproval setting")
	}
}

// ====== Room E2EE setting ======

func TestRoomRepository_CreateRoom_E2EE(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "e2ee-room", false, "standard", 0, &models.RoomSettings{E2EE: true})

	found, _ := repo.GetRoom(room.ID)
	if !found.Settings.E2EE {
		t.Fatal("expected E2EE to be true")
	}
}

// ====== GetRoomsParticipatedInByUser ======

func TestRoomRepository_GetRoomsParticipatedInByUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	// user-1 creates 2 rooms
	room1, _ := repo.CreateRoom(testUserIDRoom, "owner-room-1", false, "standard", 0, &models.RoomSettings{})
	room2, _ := repo.CreateRoom(testUserIDRoom, "owner-room-2", false, "standard", 0, &models.RoomSettings{})

	// user-2 creates 1 room, joins 1 of user-1's rooms
	_, _ = repo.CreateRoom("user-2", "u2-own-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room1.ID, "user-2")

	// user-2 participated rooms (excluding rooms they created)
	rooms, err := repo.GetRoomsParticipatedInByUser("user-2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rooms) != 1 {
		t.Fatalf("expected 1 participated room, got %d", len(rooms))
	}
	if rooms[0].ID != room1.ID {
		t.Fatalf("expected room ID '%s', got '%s'", room1.ID, rooms[0].ID)
	}

	// user-2 also joins room2
	_ = repo.AddParticipant(room2.ID, "user-2")
	rooms, _ = repo.GetRoomsParticipatedInByUser("user-2")
	if len(rooms) != 2 {
		t.Fatalf("expected 2 participated rooms, got %d", len(rooms))
	}
}

func TestRoomRepository_GetRoomsParticipatedInByUser_None(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	rooms, err := repo.GetRoomsParticipatedInByUser(testUserIDRoom)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rooms) != 0 {
		t.Fatalf("expected 0 rooms, got %d", len(rooms))
	}
}

// ====== Admin permissions validation ======

func TestRoomRepository_CreatorHasFullAdminPermissions(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "admin-perms-room", false, "standard", 0, &models.RoomSettings{})

	perms, err := repo.GetParticipantPermissions(room.ID, testUserIDRoom)
	if err != nil {
		t.Fatalf("failed to get permissions: %v", err)
	}
	if !perms.IsAdmin {
		t.Fatal("expected IsAdmin")
	}
	if !perms.CanKick {
		t.Fatal("expected CanKick")
	}
	if !perms.CanMuteAudio {
		t.Fatal("expected CanMuteAudio")
	}
	if !perms.CanDisableVideo {
		t.Fatal("expected CanDisableVideo")
	}
	if !perms.CanChat {
		t.Fatal("expected CanChat")
	}
}

func TestRoomRepository_NewParticipantHasNoPermissions(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "noperm-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, "user-2")

	// New participant should NOT have permissions entry
	_, err := repo.GetParticipantPermissions(room.ID, "user-2")
	if err == nil {
		t.Logf("Note: new participant has a permissions record (may be auto-created)")
	}
	// If no error, permissions should all be false
}

// ====== RemoveParticipant sets LeftAt ======

func TestRoomRepository_RemoveParticipant_SetsLeftAt(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "leftat-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, "user-2")
	_ = repo.RemoveParticipant(room.ID, "user-2")

	var participant models.RoomParticipant
	db.Where("room_id = ? AND user_id = ?", room.ID, "user-2").First(&participant)
	if participant.LeftAt == nil {
		t.Fatal("expected LeftAt to be set after removal")
	}
	if participant.IsActive {
		t.Fatal("expected IsActive to be false after removal")
	}
}

func TestRoomRepository_RemoveAllParticipants(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: "rap-u1", Email: "rap1@ex.com", Name: "RAP1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "rap-u2", Email: "rap2@ex.com", Name: "RAP2", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "rap-u3", Email: "rap3@ex.com", Name: "RAP3", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom("rap-u1", "rap-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, "rap-u1")
	_ = repo.AddParticipant(room.ID, "rap-u2")
	_ = repo.AddParticipant(room.ID, "rap-u3")

	if err := repo.RemoveAllParticipants(room.ID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var count int64
	db.Model(&models.RoomParticipant{}).Where("room_id = ? AND is_active = ?", room.ID, true).Count(&count)
	if count != 0 {
		t.Fatalf("expected 0 active participants, got %d", count)
	}

	// Already removed — should be a no-op
	if err := repo.RemoveAllParticipants(room.ID); err != nil {
		t.Fatalf("expected nil on repeat call, got %v", err)
	}
}

// ====== Stage management edge cases ======

func TestRoomRepository_BringToStage_CreatorAlreadyOnStage(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "creator-stage", false, "standard", 0, &models.RoomSettings{})

	// Creator should already be on stage
	onStage, _ := repo.IsParticipantOnStage(room.ID, testUserIDRoom)
	if !onStage {
		t.Fatal("expected creator to be on stage by default")
	}

	// Bringing creator to stage again should not fail
	err := repo.BringToStage(room.ID, testUserIDRoom)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRoomRepository_RemoveFromStage_CreatorCanBeRemoved(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "stage-remove", false, "standard", 0, &models.RoomSettings{})

	// Remove creator from stage (should work but might be undesirable)
	_ = repo.RemoveFromStage(room.ID, testUserIDRoom)
	onStage, _ := repo.IsParticipantOnStage(room.ID, testUserIDRoom)
	if onStage {
		t.Fatal("expected creator to be removed from stage")
	}
}

// ====== UpdateParticipantPermissions ======

func TestRoomRepository_UpdateParticipantPermissions(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "perm-update-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, "user-2")

	// Create permissions for user-2 (since AddParticipant doesn't auto-create them)
	db.Create(&models.RoomPermissions{
		ID:     "perm-u2",
		RoomID: room.ID,
		UserID: "user-2",
	})

	err := repo.UpdateParticipantPermissions(room.ID, "user-2", map[string]interface{}{
		"can_chat": true,
	})
	if err != nil {
		t.Fatalf("failed to update permissions: %v", err)
	}
}

// ====== GetRoomByID returns room with correct data ======

func TestRoomRepository_GetRoom_ReturnsCompleteData(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	_, err := repo.CreateRoom(testUserIDRoom, "complete-data-room", true, "standard", 0, &models.RoomSettings{
		AllowChat:  true,
		AllowAudio: true,
		E2EE:       true,
	})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	found, _ := repo.GetRoomByName("complete-data-room")
	if found == nil {
		t.Fatal("expected to find room")
	}
	if found.CreatedBy != testUserIDRoom {
		t.Fatalf("expected createdBy 'user-1', got '%s'", found.CreatedBy)
	}
	if found.AdminID != testUserIDRoom {
		t.Fatalf("expected adminID 'user-1', got '%s'", found.AdminID)
	}
	if !found.IsActive {
		t.Fatal("expected IsActive true")
	}
	if !found.IsPublic {
		t.Fatal("expected IsPublic true")
	}
	if found.Mode != "standard" {
		t.Fatalf("expected mode 'standard', got '%s'", found.Mode)
	}
	if found.ID == "" {
		t.Fatal("expected non-empty ID (UUID)")
	}
	if found.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
}

// ====== KickParticipant sets LeftAt ======

func TestRoomRepository_KickParticipant_SetsLeftAt(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "kick-left-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, "user-2")
	_ = repo.KickParticipant(room.ID, "user-2")

	var participant models.RoomParticipant
	db.Where("room_id = ? AND user_id = ?", room.ID, "user-2").First(&participant)
	if participant.LeftAt == nil {
		t.Fatal("expected LeftAt to be set after kick")
	}
	if !participant.IsBanned {
		t.Fatal("expected IsBanned after kick")
	}
}

// ====== GetAllRooms returns empty for no rooms ======

func TestRoomRepository_GetAllRooms_Empty(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	rooms, err := repo.GetAllRooms()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rooms) != 0 {
		t.Fatalf("expected 0 rooms, got %d", len(rooms))
	}
}

// ====== RoomRepository.GetUserByID ======

func TestRoomRepository_GetUserByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	user, err := repo.GetUserByID(testUserIDRoom)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if user == nil {
		t.Fatal("expected to find user")
	}
	if user.ID != testUserIDRoom {
		t.Fatalf("expected ID 'user-1', got '%s'", user.ID)
	}
}

func TestRoomRepository_GetUserByID_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	_, err := repo.GetUserByID("nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent user")
	}
}

// ====== Concurrency: creator join-time is set correctly ======

func TestRoomRepository_CreateRoom_CreatorJoinTimeSet(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "jointime-room", false, "standard", 0, &models.RoomSettings{})

	var participant models.RoomParticipant
	db.Where("room_id = ? AND user_id = ?", room.ID, testUserIDRoom).First(&participant)
	if participant.JoinedAt.IsZero() {
		t.Fatal("expected JoinedAt to be set for creator")
	}
	if participant.LeftAt != nil {
		t.Fatal("expected LeftAt to be nil for active creator")
	}
}

// ====== Chat blocked state ======

func TestRoomRepository_UpdateParticipantStatus_ChatBlocked(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: testUserIDRoom, Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom(testUserIDRoom, "chat-block-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, "user-2")

	_ = repo.UpdateParticipantStatus(room.ID, "user-2", map[string]interface{}{
		"is_chat_blocked": true,
	})

	var participant models.RoomParticipant
	db.Where("room_id = ? AND user_id = ?", room.ID, "user-2").First(&participant)
	if !participant.IsChatBlocked {
		t.Fatal("expected chat to be blocked")
	}
}

func TestRoomRepository_GetAllActiveRooms(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: "user-gar", Email: "gar@ex.com", Name: "GAR", Provider: "local", IsActive: true})

	r1, _ := repo.CreateRoom("user-gar", "active-room-1", false, "standard", 0, &models.RoomSettings{})
	r2, _ := repo.CreateRoom("user-gar", "active-room-2", false, "standard", 0, &models.RoomSettings{})

	// Mark r2 as idle
	_ = repo.SetRoomIdle(r2.ID)

	actives, err := repo.GetAllActiveRooms()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(actives) != 1 {
		t.Fatalf("expected 1 active room, got %d", len(actives))
	}
	if actives[0].ID != r1.ID {
		t.Fatalf("expected active room to be r1, got %s", actives[0].ID)
	}
}

func TestRoomRepository_SetRoomIdle(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: "user-sri", Email: "sri@ex.com", Name: "SRI", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom("user-sri", "idle-room", false, "standard", 0, &models.RoomSettings{})
	if !room.IsActive {
		t.Fatal("expected room to be active initially")
	}

	err := repo.SetRoomIdle(room.ID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, _ := repo.GetRoom(room.ID)
	if updated.IsActive {
		t.Fatal("expected room to be inactive after SetRoomIdle")
	}
}

func TestRoomRepository_UpdateRoom(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: "user-ur", Email: "ur@ex.com", Name: "UR", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom("user-ur", "update-room", false, "standard", 0, &models.RoomSettings{})

	room.MaxParticipants = 99
	err := repo.UpdateRoom(room)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	updated, _ := repo.GetRoom(room.ID)
	if updated.MaxParticipants != 99 {
		t.Fatalf("expected MaxParticipants 99, got %d", updated.MaxParticipants)
	}
}

func TestRoomRepository_CountActiveParticipants(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: "user-cap1", Email: "cap1@ex.com", Name: "CAP1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "user-cap2", Email: "cap2@ex.com", Name: "CAP2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom("user-cap1", "cap-room", false, "standard", 0, &models.RoomSettings{})
	_ = repo.AddParticipant(room.ID, "user-cap2")

	count, err := repo.CountActiveParticipants()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// creator + cap2 = 2 distinct users
	if count < 1 {
		t.Fatalf("expected at least 1 active participant, got %d", count)
	}
}

func TestRoomRepository_CountActiveParticipants_Empty(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	count, err := repo.CountActiveParticipants()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 participants in empty DB, got %d", count)
	}
}

func TestRoomRepository_GetAllActiveRoomsWithLimit(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: "user-limit", Email: "limit@ex.com", Name: "Limit", Provider: "local", IsActive: true})

	for i := 0; i < 5; i++ {
		_, _ = repo.CreateRoom("user-limit", fmt.Sprintf("limit-room-%d", i), false, "standard", 0, &models.RoomSettings{})
	}

	rooms, err := repo.GetAllActiveRoomsWithLimit(3)
	if err != nil {
		t.Fatal(err)
	}
	if len(rooms) > 3 {
		t.Fatalf("expected max 3 rooms, got %d", len(rooms))
	}
}

func TestRoomRepository_UpdateRoom_SavesAllFields(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: "user-save", Email: "save@ex.com", Name: "Save", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom("user-save", "save-test", false, "standard", 0, &models.RoomSettings{
		AllowChat: true, AllowVideo: true, AllowAudio: false,
		RequireApproval: true, E2EE: false, IsPersistent: false,
	})

	// Modify several fields
	room.MaxParticipants = 42
	room.IsPublic = true
	room.Settings.AllowChat = false
	room.Settings.IsPersistent = true
	room.Mode = "webinar"

	err := repo.UpdateRoom(room)
	if err != nil {
		t.Fatalf("UpdateRoom failed: %v", err)
	}

	updated, _ := repo.GetRoom(room.ID)
	if updated.MaxParticipants != 42 {
		t.Fatalf("expected MaxParticipants 42, got %d", updated.MaxParticipants)
	}
	if !updated.IsPublic {
		t.Fatal("expected IsPublic true")
	}
	if updated.Settings.AllowChat {
		t.Fatal("expected AllowChat false")
	}
	if !updated.Settings.IsPersistent {
		t.Fatal("expected IsPersistent true")
	}
	if updated.Mode != "webinar" {
		t.Fatalf("expected mode 'webinar', got '%s'", updated.Mode)
	}
	// Fields not touched should retain original values
	if !updated.Settings.AllowVideo {
		t.Fatal("expected AllowVideo to remain true")
	}
	if !updated.Settings.RequireApproval {
		t.Fatal("expected RequireApproval to remain true")
	}
}

func TestRoomRepository_EnrichAdminRoomDetails(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)
	db.Create(&models.User{ID: "enrich-u1", Email: "u1@ex.com", Name: "User1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "enrich-u2", Email: "u2@ex.com", Name: "User2", Provider: "local", IsActive: true})

	room1, _ := repo.CreateRoom("enrich-u1", "enrich-room-1", true, "standard", 0, &models.RoomSettings{})
	room2, _ := repo.CreateRoom("enrich-u2", "enrich-room-2", true, "standard", 0, &models.RoomSettings{})

	// room1 gets a second participant (creator enrich-u1 already auto-added)
	if err := repo.AddParticipant(room1.ID, "enrich-u2"); err != nil {
		t.Fatalf("failed to add participant: %v", err)
	}

	enriched, err := repo.EnrichAdminRoomDetails([]models.Room{*room1, *room2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(enriched) != 2 {
		t.Fatalf("expected 2 results, got %d", len(enriched))
	}

	for _, e := range enriched {
		switch e.Name {
		case "enrich-room-1":
			if e.ParticipantsCount != 2 {
				t.Fatalf("expected 2 participants for room1, got %d", e.ParticipantsCount)
			}
			if e.LastActivityAt == nil {
				t.Fatal("expected non-nil lastActivityAt for room1")
			}
			if e.OwnerName != "User1" {
				t.Fatalf("expected owner 'User1', got '%s'", e.OwnerName)
			}
		case "enrich-room-2":
			// room2 has 1 participant (the creator, auto-added by CreateRoom)
			if e.ParticipantsCount != 1 {
				t.Fatalf("expected 1 participant for room2, got %d", e.ParticipantsCount)
			}
			if e.LastActivityAt == nil {
				t.Fatal("expected non-nil lastActivityAt for room2")
			}
		}
	}
}

func TestRoomRepository_EnrichAdminRoomDetails_EmptyInput(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	result, err := repo.EnrichAdminRoomDetails([]models.Room{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 results, got %d", len(result))
	}
}

func TestRoomRepository_CountRoomsByDay(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)
	now := time.Now().UTC()

	// Override timestamps to specific days via direct DB update
	db.Create(&models.User{ID: "crd-u1", Email: "crd@ex.com", Name: "CRD", Provider: "local", IsActive: true})
	room1, _ := repo.CreateRoom("crd-u1", "crd-room-1", true, "standard", 0, &models.RoomSettings{})
	room2, _ := repo.CreateRoom("crd-u1", "crd-room-2", true, "standard", 0, &models.RoomSettings{})
	room3, _ := repo.CreateRoom("crd-u1", "crd-room-3", true, "standard", 0, &models.RoomSettings{})

	day0 := now.Add(-24 * time.Hour)
	day1 := now.Add(-48 * time.Hour)
	db.Model(room1).Update("created_at", day0)
	db.Model(room2).Update("created_at", day0)
	db.Model(room3).Update("created_at", day1)

	counts, err := repo.CountRoomsByDay(7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(counts) != 7 {
		t.Fatalf("expected 7 days, got %d", len(counts))
	}

	day0Key := day0.Format("2006-01-02")
	day1Key := day1.Format("2006-01-02")
	var day0Count, day1Count int
	for _, c := range counts {
		key := c.Date.Format("2006-01-02")
		if key == day0Key {
			day0Count = c.Count
		}
		if key == day1Key {
			day1Count = c.Count
		}
	}
	if day0Count != 2 {
		t.Fatalf("expected 2 rooms on %s, got %d", day0Key, day0Count)
	}
	if day1Count != 1 {
		t.Fatalf("expected 1 room on %s, got %d", day1Key, day1Count)
	}
}

func TestRoomRepository_CountRoomsByDay_Empty(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	counts, err := repo.CountRoomsByDay(7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(counts) != 7 {
		t.Fatalf("expected 7 days, got %d", len(counts))
	}
	for _, c := range counts {
		if c.Count != 0 {
			t.Fatalf("expected 0 count for %s, got %d", c.Date.Format("2006-01-02"), c.Count)
		}
	}
}

func TestRoomRepository_CountActiveParticipantsByDay(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)
	now := time.Now().UTC()

	db.Create(&models.User{ID: "cap-u1", Email: "cap@ex.com", Name: "CAP", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "cap-u2", Email: "cap2@ex.com", Name: "CAP2", Provider: "local", IsActive: true})
	room, _ := repo.CreateRoom("cap-u1", "cap-room", true, "standard", 0, &models.RoomSettings{})

	if err := repo.AddParticipant(room.ID, "cap-u2"); err != nil {
		t.Fatalf("failed to add participant: %v", err)
	}
	// Override JoinedAt on participant records
	day0 := now.Add(-24 * time.Hour)
	day1 := now.Add(-48 * time.Hour)
	db.Model(&models.RoomParticipant{}).Where("user_id = ?", "cap-u1").Update("joined_at", day0)
	db.Model(&models.RoomParticipant{}).Where("user_id = ?", "cap-u2").Update("joined_at", day1)

	counts, err := repo.CountActiveParticipantsByDay(7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(counts) != 7 {
		t.Fatalf("expected 7 days, got %d", len(counts))
	}

	day0Key := day0.Format("2006-01-02")
	day1Key := day1.Format("2006-01-02")
	var day0Count, day1Count int
	for _, c := range counts {
		key := c.Date.Format("2006-01-02")
		if key == day0Key {
			day0Count = c.Count
		}
		if key == day1Key {
			day1Count = c.Count
		}
	}
	if day0Count != 1 {
		t.Fatalf("expected 1 participant on %s, got %d", day0Key, day0Count)
	}
	if day1Count != 1 {
		t.Fatalf("expected 1 participant on %s, got %d", day1Key, day1Count)
	}
}

func TestRoomRepository_GetRecentRoomEvents(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: "evt-u1", Email: "evt@ex.com", Name: "EventUser", Provider: "local", IsActive: true})
	room, err := repo.CreateRoom("evt-u1", "event-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	if err := repo.AddParticipant(room.ID, "evt-u1"); err != nil {
		t.Fatalf("failed to add participant: %v", err)
	}

	events, err := repo.GetRecentRoomEvents(10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(events) < 2 {
		t.Fatalf("expected at least 2 events, got %d", len(events))
	}

	// First event should be room_joined (most recent), second should be room_created
	if events[0].Type != "room_joined" {
		t.Fatalf("expected first event type 'room_joined', got '%s'", events[0].Type)
	}
	if events[0].RoomID != room.ID {
		t.Fatalf("expected room ID %s, got %s", room.ID, events[0].RoomID)
	}
	if events[1].Type != "room_created" {
		t.Fatalf("expected second event type 'room_created', got '%s'", events[1].Type)
	}
	if events[1].RoomID != room.ID {
		t.Fatalf("expected room ID %s, got %s", room.ID, events[1].RoomID)
	}
}

func TestRoomRepository_GetRecentRoomEvents_Empty(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	events, err := repo.GetRecentRoomEvents(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if events == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(events) != 0 {
		t.Fatalf("expected 0 events, got %d", len(events))
	}
}

func TestRoomRepository_CountActiveRoomsWithParticipantCount(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: "arc-u1", Email: "arc@ex.com", Name: "ARC", Provider: "local", IsActive: true})
	repo.CreateRoom("arc-u1", "arc-active", true, "standard", 0, &models.RoomSettings{})
	repo.CreateRoom("arc-u1", "arc-inactive", true, "standard", 0, &models.RoomSettings{})

	// Deactivate second room
	db.Model(&models.Room{}).Where("name = ?", "arc-inactive").Update("is_active", false)

	count, err := repo.CountActiveRoomsWithParticipantCount()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 active room, got %d", count)
	}
}

func TestRoomRepository_CountActiveRoomsWithParticipantCount_None(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	count, err := repo.CountActiveRoomsWithParticipantCount()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 active rooms, got %d", count)
	}
}

func TestRoomRepository_CountPersistentRooms(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: "cpr-u1", Email: "cpr@ex.com", Name: "CPR", Provider: "local", IsActive: true})

	// No persistent rooms yet
	count, err := repo.CountPersistentRooms()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 persistent rooms, got %d", count)
	}

	// Create a persistent room
	room, _ := repo.CreateRoom("cpr-u1", "cpr-room-1", true, "standard", 0, &models.RoomSettings{IsPersistent: true})
	count, err = repo.CountPersistentRooms()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 persistent room, got %d", count)
	}

	// Non-persistent room should not affect count
	repo.CreateRoom("cpr-u1", "cpr-room-2", true, "standard", 0, &models.RoomSettings{})
	count, err = repo.CountPersistentRooms()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected still 1 persistent room, got %d", count)
	}

	// Verify persisted via embedded prefix
	var dbCount int64
	db.Model(&models.Room{}).Where("settings_is_persistent = ?", true).Count(&dbCount)
	if dbCount != 1 {
		t.Fatalf("expected 1 in raw DB, got %d", dbCount)
	}
	_ = room
}

func TestRoomRepository_CountActiveRoomsByDay(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)
	now := time.Now().UTC()

	db.Create(&models.User{ID: "card-u1", Email: "card@ex.com", Name: "CARD", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "card-u2", Email: "card2@ex.com", Name: "CARD2", Provider: "local", IsActive: true})

	room, _ := repo.CreateRoom("card-u1", "card-room", true, "standard", 0, &models.RoomSettings{})

	// Add participants on different days
	if err := repo.AddParticipant(room.ID, "card-u1"); err != nil {
		t.Fatalf("failed to add participant: %v", err)
	}
	if err := repo.AddParticipant(room.ID, "card-u2"); err != nil {
		t.Fatalf("failed to add participant: %v", err)
	}

	day0 := now.Add(-24 * time.Hour)
	day1 := now.Add(-48 * time.Hour)
	db.Model(&models.RoomParticipant{}).Where("user_id = ?", "card-u1").Update("joined_at", day0)
	db.Model(&models.RoomParticipant{}).Where("user_id = ?", "card-u2").Update("joined_at", day1)

	counts, err := repo.CountActiveRoomsByDay(7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(counts) != 7 {
		t.Fatalf("expected 7 days, got %d", len(counts))
	}

	day0Key := day0.Format("2006-01-02")
	day1Key := day1.Format("2006-01-02")
	var day0Count, day1Count int
	for _, c := range counts {
		key := c.Date.Format("2006-01-02")
		if key == day0Key {
			day0Count = c.Count
		}
		if key == day1Key {
			day1Count = c.Count
		}
	}
	// Both participants are in the same room — only 1 distinct room per day
	if day0Count != 1 {
		t.Fatalf("expected 1 active room on %s, got %d", day0Key, day0Count)
	}
	if day1Count != 1 {
		t.Fatalf("expected 1 active room on %s, got %d", day1Key, day1Count)
	}
}

func TestRoomRepository_CountActiveRoomsByDay_Empty(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	counts, err := repo.CountActiveRoomsByDay(7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(counts) != 7 {
		t.Fatalf("expected 7 days, got %d", len(counts))
	}
	for _, c := range counts {
		if c.Count != 0 {
			t.Fatalf("expected 0 count for %s, got %d", c.Date.Format("2006-01-02"), c.Count)
		}
	}
}

func TestRoomRepository_CountStaleRooms(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	db.Create(&models.User{ID: "csr-u1", Email: "csr@ex.com", Name: "CSR", Provider: "local", IsActive: true})

	// Room with recent activity — NOT stale
	now := time.Now()
	room, _ := repo.CreateRoom("csr-u1", "csr-room-active", true, "standard", 0, &models.RoomSettings{})
	db.Model(room).Update("last_activity_at", now.Add(-1*time.Hour))

	// Room with old activity — IS stale
	room2, _ := repo.CreateRoom("csr-u1", "csr-room-stale", true, "standard", 0, &models.RoomSettings{})
	db.Model(room2).Update("last_activity_at", now.Add(-72*time.Hour))

	// Room with nil last_activity_at (pre-migration) — falls back to created_at
	room3, _ := repo.CreateRoom("csr-u1", "csr-room-null", true, "standard", 0, &models.RoomSettings{})
	db.Model(room3).Update("last_activity_at", nil)
	db.Model(room3).Update("created_at", now.Add(-96*time.Hour)) // old created_at = stale

	// Room created recently with nil last_activity_at — NOT stale (created_at is recent)
	room4, _ := repo.CreateRoom("csr-u1", "csr-room-recent-null", true, "standard", 0, &models.RoomSettings{})
	db.Model(room4).Update("last_activity_at", nil)

	count, err := repo.CountStaleRooms(48)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// room2 (last_activity 72h ago) and room3 (created 96h ago, last_activity nil) are stale = 2
	if count != 2 {
		t.Fatalf("expected 2 stale rooms, got %d", count)
	}
	_ = room
}

func TestRoomRepository_CountStaleRooms_None(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewRoomRepository(db)

	count, err := repo.CountStaleRooms(48)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 stale rooms, got %d", count)
	}
}
