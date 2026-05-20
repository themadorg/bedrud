package repository

import (
	"bedrud/internal/models"
	"bedrud/internal/testutil"
	"testing"
	"time"
)

func TestUserRepository_CreateUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user := &models.User{
		ID:       "user-1",
		Email:    "test@example.com",
		Name:     "Test User",
		Provider: "local",
		Accesses: models.StringArray{"user"},
		IsActive: true,
	}

	err := repo.CreateUser(user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Verify user was created
	found, err := repo.GetUserByID("user-1")
	if err != nil {
		t.Fatalf("failed to get user: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find user")
	}
	if found.Email != "test@example.com" {
		t.Fatalf("expected email 'test@example.com', got '%s'", found.Email)
	}
}

func TestUserRepository_CreateUser_DuplicateEmail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user1 := &models.User{ID: "user-1", Email: "dup@example.com", Name: "User 1", Provider: "local", IsActive: true}
	user2 := &models.User{ID: "user-2", Email: "dup@example.com", Name: "User 2", Provider: "local", IsActive: true}

	_ = repo.CreateUser(user1)
	err := repo.CreateUser(user2)
	if err == nil {
		t.Fatal("expected error for duplicate email")
	}
}

func TestUserRepository_GetUserByEmail(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user := &models.User{ID: "user-1", Email: "find@example.com", Name: "Found", Provider: "local", IsActive: true}
	_ = repo.CreateUser(user)

	found, err := repo.GetUserByEmail("find@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find user")
	}
	if found.Name != "Found" {
		t.Fatalf("expected name 'Found', got '%s'", found.Name)
	}
}

func TestUserRepository_GetUserByEmail_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	found, err := repo.GetUserByEmail("nonexistent@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != nil {
		t.Fatal("expected nil for non-existent user")
	}
}

func TestUserRepository_GetUserByID(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user := &models.User{ID: "user-42", Email: "id@example.com", Name: "ID User", Provider: "local", IsActive: true}
	_ = repo.CreateUser(user)

	found, err := repo.GetUserByID("user-42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found == nil || found.ID != "user-42" {
		t.Fatal("expected to find user by ID")
	}
}

func TestUserRepository_GetUserByID_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	found, err := repo.GetUserByID("nonexistent-id")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != nil {
		t.Fatal("expected nil for non-existent user")
	}
}

func TestUserRepository_UpdateRefreshToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user := &models.User{ID: "user-1", Email: "refresh@example.com", Name: "Refresh", Provider: "local", IsActive: true}
	_ = repo.CreateUser(user)

	err := repo.UpdateRefreshToken("user-1", "new-refresh-token")
	if err != nil {
		t.Fatalf("failed to update refresh token: %v", err)
	}

	matched, err := repo.MatchRefreshToken("user-1", "new-refresh-token")
	if err != nil {
		t.Fatalf("MatchRefreshToken failed: %v", err)
	}
	if !matched {
		t.Fatal("expected MatchRefreshToken to return true for the stored token")
	}
}

func TestUserRepository_UpdateUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user := &models.User{ID: "user-1", Email: "update@example.com", Name: "Original", Provider: "local", IsActive: true}
	_ = repo.CreateUser(user)

	user.Name = "Updated Name"
	err := repo.UpdateUser(user)
	if err != nil {
		t.Fatalf("failed to update user: %v", err)
	}

	found, _ := repo.GetUserByID("user-1")
	if found.Name != "Updated Name" {
		t.Fatalf("expected name 'Updated Name', got '%s'", found.Name)
	}
}

func TestUserRepository_DeleteUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user := &models.User{ID: "user-to-delete", Email: "delete@example.com", Name: "Delete Me", Provider: "local", IsActive: true}
	_ = repo.CreateUser(user)

	err := repo.DeleteUser("user-to-delete")
	if err != nil {
		t.Fatalf("failed to delete user: %v", err)
	}

	found, _ := repo.GetUserByID("user-to-delete")
	if found != nil {
		t.Fatal("expected user to be deleted")
	}
}

func TestUserRepository_GetAllUsers(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	_ = repo.CreateUser(&models.User{ID: "u1", Email: "u1@ex.com", Name: "U1", Provider: "local", IsActive: true})
	_ = repo.CreateUser(&models.User{ID: "u2", Email: "u2@ex.com", Name: "U2", Provider: "local", IsActive: true})
	_ = repo.CreateUser(&models.User{ID: "u3", Email: "u3@ex.com", Name: "U3", Provider: "google", IsActive: true})

	users, _, err := repo.GetAllUsers(PaginationParams{Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(users))
	}
}

func TestUserRepository_GetAllUsers_Empty(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	users, _, err := repo.GetAllUsers(PaginationParams{Page: 1, Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(users) != 0 {
		t.Fatalf("expected 0 users, got %d", len(users))
	}
}

func TestUserRepository_CreateOrUpdateUser(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user := &models.User{
		ID:       "user-1",
		Email:    "oauth@example.com",
		Name:     "OAuth User",
		Provider: "google",
		IsActive: true,
		Accesses: models.StringArray{"user"},
	}

	// First call should create
	err := repo.CreateOrUpdateUser(user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Second call should update
	user.Name = "Updated OAuth User"
	err = repo.CreateOrUpdateUser(user)
	if err != nil {
		t.Fatalf("failed to update user: %v", err)
	}

	found, _ := repo.GetUserByEmail("oauth@example.com")
	if found == nil {
		t.Fatal("expected to find user")
	}
}

func TestUserRepository_GetUserByEmailAndProvider(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	_ = repo.CreateUser(&models.User{ID: "u1", Email: "local@ex.com", Name: "Local", Provider: "local", IsActive: true})
	_ = repo.CreateUser(&models.User{ID: "u2", Email: "google@ex.com", Name: "Google", Provider: "google", IsActive: true})

	found, err := repo.GetUserByEmailAndProvider("google@ex.com", "google")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found == nil {
		t.Fatal("expected to find user")
	}
	if found.Provider != "google" {
		t.Fatalf("expected provider 'google', got '%s'", found.Provider)
	}

	// Also verify provider mismatch returns nil
	notFound, err := repo.GetUserByEmailAndProvider("local@ex.com", "google")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if notFound != nil {
		t.Fatal("expected nil for email+provider mismatch")
	}
}

func TestUserRepository_GetUserByEmailAndProvider_NotFound(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	found, err := repo.GetUserByEmailAndProvider("nonexist@ex.com", "local")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if found != nil {
		t.Fatal("expected nil for non-existent user")
	}
}

func TestUserRepository_BlockRefreshToken(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user := &models.User{ID: "user-1", Email: "block@ex.com", Name: "Block", Provider: "local", IsActive: true}
	_ = repo.CreateUser(user)

	err := repo.BlockRefreshToken("user-1", "some-token", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("failed to block refresh token: %v", err)
	}

	blocked, err := repo.IsRefreshTokenBlocked("some-token")
	if err != nil {
		t.Fatalf("IsRefreshTokenBlocked failed: %v", err)
	}
	if !blocked {
		t.Fatal("expected token to be blocked")
	}
}

func TestUserRepository_IsRefreshTokenBlocked_NotBlocked(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	blocked, err := repo.IsRefreshTokenBlocked("nonexistent-token")
	if err != nil {
		t.Fatalf("IsRefreshTokenBlocked failed: %v", err)
	}
	if blocked {
		t.Fatal("expected token to not be blocked")
	}
}

func TestUserRepository_IsRefreshTokenBlocked_Expired(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user := &models.User{ID: "user-1", Email: "exp@ex.com", Name: "Exp", Provider: "local", IsActive: true}
	_ = repo.CreateUser(user)

	// Block with past expiration
	_ = repo.BlockRefreshToken("user-1", "expired-token", time.Now().Add(-1*time.Hour))

	blocked, err := repo.IsRefreshTokenBlocked("expired-token")
	if err != nil {
		t.Fatalf("IsRefreshTokenBlocked failed: %v", err)
	}
	if blocked {
		t.Fatal("expected expired blocked token to not be considered blocked")
	}
}

func TestUserRepository_CleanupBlockedTokens(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	user := &models.User{ID: "user-1", Email: "cleanup@ex.com", Name: "Cleanup", Provider: "local", IsActive: true}
	_ = repo.CreateUser(user)

	_ = repo.BlockRefreshToken("user-1", "expired-1", time.Now().Add(-2*time.Hour))
	_ = repo.BlockRefreshToken("user-1", "expired-2", time.Now().Add(-1*time.Hour))
	_ = repo.BlockRefreshToken("user-1", "active-1", time.Now().Add(1*time.Hour))

	err := repo.CleanupBlockedTokens()
	if err != nil {
		t.Fatalf("failed to cleanup: %v", err)
	}

	// Only the active one should remain
	var count int64
	db.Model(&models.BlockedRefreshToken{}).Count(&count)
	if count != 1 {
		t.Fatalf("expected 1 remaining blocked token, got %d", count)
	}
}

func TestUserRepository_UpdateUserAccesses(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	_ = repo.CreateUser(&models.User{ID: "acc-user", Email: "acc@ex.com", Name: "Acc", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})

	err := repo.UpdateUserAccesses("acc-user", []string{"admin", "user"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	found, _ := repo.GetUserByID("acc-user")
	if len(found.Accesses) != 2 {
		t.Fatalf("expected 2 accesses, got %d", len(found.Accesses))
	}
}

// Note: GetUsersByAccess uses PostgreSQL-specific ANY() syntax and cannot be tested
// with the in-memory SQLite test DB.

func TestUserRepository_DeleteUser_CascadesCleanup(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	roomRepo := NewRoomRepository(db)

	// Create user
	user := &models.User{ID: "user-cascade", Email: "cascade@ex.com", Name: "Cascade", Provider: "local", IsActive: true}
	_ = repo.CreateUser(user)

	// Create room with this user
	_, _ = roomRepo.CreateRoom("user-cascade", "test-room", false, "standard", 0, &models.RoomSettings{})

	// Block a token
	_ = repo.BlockRefreshToken("user-cascade", "some-token", time.Now().Add(time.Hour))

	// Delete user should cascade
	err := repo.DeleteUser("user-cascade")
	if err != nil {
		t.Fatalf("failed to delete user: %v", err)
	}

	// Verify user is gone
	found, _ := repo.GetUserByID("user-cascade")
	if found != nil {
		t.Fatal("expected user to be deleted")
	}

	// Verify participants are cleaned up
	var participantCount int64
	db.Model(&models.RoomParticipant{}).Where("user_id = ?", "user-cascade").Count(&participantCount)
	if participantCount != 0 {
		t.Fatalf("expected 0 participant records, got %d", participantCount)
	}
}

func TestUserRepository_GetRecentUsers(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	now := time.Now().UTC()
	user1 := &models.User{ID: "gru-1", Email: "gru1@ex.com", Name: "User1", Provider: "local", IsActive: true, CreatedAt: now.Add(-2 * time.Hour)}
	user2 := &models.User{ID: "gru-2", Email: "gru2@ex.com", Name: "User2", Provider: "local", IsActive: true, CreatedAt: now.Add(-1 * time.Hour)}
	user3 := &models.User{ID: "gru-3", Email: "gru3@ex.com", Name: "User3", Provider: "local", IsActive: true, CreatedAt: now}
	_ = repo.CreateUser(user1)
	_ = repo.CreateUser(user2)
	_ = repo.CreateUser(user3)

	recent, err := repo.GetRecentUsers(2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(recent) != 2 {
		t.Fatalf("expected 2 users, got %d", len(recent))
	}
	if recent[0].ID != "gru-3" {
		t.Fatalf("expected most recent 'gru-3', got '%s'", recent[0].ID)
	}
	if recent[1].ID != "gru-2" {
		t.Fatalf("expected second 'gru-2', got '%s'", recent[1].ID)
	}
}

func TestUserRepository_GetRecentUsers_Empty(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	users, err := repo.GetRecentUsers(5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if users == nil {
		t.Fatal("expected non-nil empty slice")
	}
	if len(users) != 0 {
		t.Fatalf("expected 0 users, got %d", len(users))
	}
}

func TestUserRepository_CountUsersByDay(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	now := time.Now().UTC()

	// Create users with specific CreatedAt
	u1 := &models.User{ID: "cud-1", Email: "cud1@ex.com", Name: "CUD1", Provider: "local", IsActive: true}
	u2 := &models.User{ID: "cud-2", Email: "cud2@ex.com", Name: "CUD2", Provider: "local", IsActive: true}
	u3 := &models.User{ID: "cud-3", Email: "cud3@ex.com", Name: "CUD3", Provider: "local", IsActive: true}
	_ = repo.CreateUser(u1)
	_ = repo.CreateUser(u2)
	_ = repo.CreateUser(u3)

	day0 := now.Add(-24 * time.Hour)
	day1 := now.Add(-48 * time.Hour)
	db.Model(&models.User{}).Where("id = ?", "cud-1").Update("created_at", day0)
	db.Model(&models.User{}).Where("id = ?", "cud-2").Update("created_at", day0)
	db.Model(&models.User{}).Where("id = ?", "cud-3").Update("created_at", day1)

	counts, err := repo.CountUsersByDay(7)
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
		t.Fatalf("expected 2 users on %s, got %d", day0Key, day0Count)
	}
	if day1Count != 1 {
		t.Fatalf("expected 1 user on %s, got %d", day1Key, day1Count)
	}
}

func TestUserRepository_CountUsersFiltered(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	// Seed users with different providers
	db.Create(&models.User{ID: "cuf-1", Email: "cuf1@ex.com", Name: "CUF1", Provider: "local", IsActive: true})
	db.Create(&models.User{ID: "cuf-2", Email: "cuf2@ex.com", Name: "CUF2", Provider: "github", IsActive: true})
	db.Create(&models.User{ID: "cuf-3", Email: "cuf3@ex.com", Name: "CUF3", Provider: "guest", IsActive: true})
	db.Create(&models.User{ID: "cuf-4", Email: "cuf4@ex.com", Name: "CUF4", Provider: "guest", IsActive: true})

	// No filter — count all
	all, err := repo.CountUsersFiltered(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if all != 4 {
		t.Fatalf("expected 4 total users, got %d", all)
	}

	// Exclude guests
	withoutGuests, err := repo.CountUsersFiltered([]string{"guest"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if withoutGuests != 2 {
		t.Fatalf("expected 2 non-guest users, got %d", withoutGuests)
	}

	// Exclude multiple providers
	withoutLocal, err := repo.CountUsersFiltered([]string{"local", "guest"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if withoutLocal != 1 {
		t.Fatalf("expected 1 user (github only), got %d", withoutLocal)
	}

	// Empty filter — same as no filter
	empty, err := repo.CountUsersFiltered([]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if empty != 4 {
		t.Fatalf("expected 4 with empty filter, got %d", empty)
	}
}

func TestUserRepository_CountUsersSinceFiltered(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)
	now := time.Now().UTC()

	// Seed users with different providers at different times
	db.Create(&models.User{ID: "cusf-1", Email: "cusf1@ex.com", Name: "CUSF1", Provider: "local", IsActive: true, CreatedAt: now.Add(-72 * time.Hour)})
	db.Create(&models.User{ID: "cusf-2", Email: "cusf2@ex.com", Name: "CUSF2", Provider: "github", IsActive: true, CreatedAt: now.Add(-48 * time.Hour)})
	db.Create(&models.User{ID: "cusf-3", Email: "cusf3@ex.com", Name: "CUSF3", Provider: "guest", IsActive: true, CreatedAt: now.Add(-24 * time.Hour)})

	// Count all since 96h ago (all users)
	all, err := repo.CountUsersSinceFiltered(now.Add(-96*time.Hour), nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if all != 3 {
		t.Fatalf("expected 3 users, got %d", all)
	}

	// Exclude guests
	withoutGuests, err := repo.CountUsersSinceFiltered(now.Add(-96*time.Hour), []string{"guest"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if withoutGuests != 2 {
		t.Fatalf("expected 2 non-guest users, got %d", withoutGuests)
	}

	// Since 36h ago: only the guest (24h ago) should match
	recent, err := repo.CountUsersSinceFiltered(now.Add(-36*time.Hour), []string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recent != 1 {
		t.Fatalf("expected 1 recent user, got %d", recent)
	}

	// Since 36h ago excluding guests: zero
	recentNoGuest, err := repo.CountUsersSinceFiltered(now.Add(-36*time.Hour), []string{"guest"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recentNoGuest != 0 {
		t.Fatalf("expected 0 recent non-guest users, got %d", recentNoGuest)
	}
}

func TestUserRepository_DeleteUnverifiedUsers_DeletesEligible(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	// Create an old unverified user — eligible for deletion
	oldUnverified := &models.User{
		ID:       "del-eligible-1",
		Email:    "old@test.com",
		Password: "hash",
		Name:     "Old",
		Provider: "local",
	}
	if err := db.Create(oldUnverified).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	// Backdate updated_at so it looks old
	past := time.Now().Add(-72 * time.Hour)
	db.Model(oldUnverified).Update("updated_at", past)

	// Create a verified user — should NOT be deleted
	now := time.Now()
	verified := &models.User{
		ID:              "del-verified-1",
		Email:           "verified@test.com",
		Password:        "hash",
		Name:            "Verified",
		Provider:        "local",
		EmailVerifiedAt: &now,
	}
	if err := db.Create(verified).Error; err != nil {
		t.Fatalf("failed to create verified user: %v", err)
	}

	// Create a recently changed user (EmailVerifiedAt=nil, updated_at recent)
	recentChanged := &models.User{
		ID:       "del-changed-1",
		Email:    "changed@test.com",
		Password: "hash",
		Name:     "Changed",
		Provider: "local",
	}
	if err := db.Create(recentChanged).Error; err != nil {
		t.Fatalf("failed to create changed user: %v", err)
	}
	// Backdate created_at to old, but keep updated_at recent (like ChangeEmail did)
	db.Model(recentChanged).Update("created_at", past)

	cutoff := time.Now().Add(-24 * time.Hour)
	deleted, err := repo.DeleteUnverifiedUsers(cutoff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Only oldUnverified should be deleted (old updated_at + nil EmailVerifiedAt)
	if deleted != 1 {
		t.Fatalf("expected 1 deleted, got %d", deleted)
	}

	// Verify oldUnverified is gone (GetUserByID returns nil, nil for not found)
	deletedUser, err := repo.GetUserByID(oldUnverified.ID)
	if err != nil {
		t.Fatalf("unexpected error checking deleted user: %v", err)
	}
	if deletedUser != nil {
		t.Fatal("expected oldUnverified to be deleted")
	}

	// Verify verified user still exists
	v, err := repo.GetUserByID(verified.ID)
	if err != nil || v == nil {
		t.Fatal("expected verified user to remain")
	}

	// Verify recently changed user still exists
	c, err := repo.GetUserByID(recentChanged.ID)
	if err != nil || c == nil {
		t.Fatal("expected recently changed user to remain")
	}
}

func TestUserRepository_DeleteUnverifiedUsers_NoEligible(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	// Create a verified user only
	now := time.Now()
	verified := &models.User{
		ID:              "del-only-1",
		Email:           "only@test.com",
		Password:        "hash",
		Name:            "Only",
		Provider:        "local",
		EmailVerifiedAt: &now,
	}
	if err := db.Create(verified).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	cutoff := time.Now().Add(-1 * time.Hour)
	deleted, err := repo.DeleteUnverifiedUsers(cutoff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if deleted != 0 {
		t.Fatalf("expected 0 deleted, got %d", deleted)
	}
}

func TestUserRepository_DeleteUnverifiedUsers_PasskeyProvider(t *testing.T) {
	db := testutil.SetupTestDB(t)
	repo := NewUserRepository(db)

	// Create old unverified passkey user — eligible
	passkeyUser := &models.User{
		ID:       "del-passkey-1",
		Email:    "passkey@test.com",
		Name:     "Passkey",
		Provider: "passkey",
	}
	if err := db.Create(passkeyUser).Error; err != nil {
		t.Fatalf("failed to create passkey user: %v", err)
	}
	past := time.Now().Add(-72 * time.Hour)
	db.Model(passkeyUser).Update("updated_at", past)

	// Create old unverified OAuth user — NOT eligible (different provider)
	oauthUser := &models.User{
		ID:       "del-oauth-1",
		Email:    "oauth@test.com",
		Name:     "OAuth",
		Provider: "google",
	}
	if err := db.Create(oauthUser).Error; err != nil {
		t.Fatalf("failed to create oauth user: %v", err)
	}
	db.Model(oauthUser).Update("updated_at", past)

	cutoff := time.Now().Add(-24 * time.Hour)
	deleted, err := repo.DeleteUnverifiedUsers(cutoff)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if deleted != 1 {
		t.Fatalf("expected 1 deleted (passkey), got %d", deleted)
	}

	// Verify OAuth user still exists
	o, err := repo.GetUserByID(oauthUser.ID)
	if err != nil || o == nil {
		t.Fatal("expected OAuth user to remain")
	}
}
