package queue

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/services"
	"bedrud/internal/storage"
	"bedrud/internal/testutil"
)

func TestUserDeleteHandler(t *testing.T) {
	t.Run("should remove the user's welcome sound file", func(t *testing.T) {
		soundDir := t.TempDir()
		t.Cleanup(storage.SetWelcomeSoundDirForTest(soundDir))
		soundPath := filepath.Join(soundDir, "leaving-user.mp3")
		if err := os.WriteFile(soundPath, []byte("clip"), 0o644); err != nil {
			t.Fatal(err)
		}

		db := testutil.SetupTestDB(t)
		userRepo := repository.NewUserRepository(db)
		roomRepo := repository.NewRoomRepository(db)
		if err := userRepo.CreateUser(&models.User{ID: "leaving-user", Email: "leaving@ex.com", Name: "Leaving", Provider: "local", IsActive: true}); err != nil {
			t.Fatal(err)
		}
		cleanupSvc := services.NewRoomCleanupService(roomRepo, nil, testutil.NewMockRoomService(), nil, "test", "secret", nil)
		handler := NewUserDeleteHandler(cleanupSvc, userRepo, repository.NewPasskeyRepository(db), repository.NewUserPreferencesRepository(db), roomRepo)
		payload, _ := json.Marshal(UserDeletePayload{UserID: "leaving-user", Email: "leaving@ex.com"})

		if err := handler(context.Background(), db, &models.Job{Payload: string(payload)}); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(soundPath); !os.IsNotExist(err) {
			t.Fatalf("expected the welcome sound file to be gone, got %v", err)
		}
	})
}
