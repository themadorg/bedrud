package auth

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"bedrud/internal/storage"
)

func testWelcomeSoundLimits() storage.WelcomeSoundLimits {
	return storage.WelcomeSoundLimits{MaxBytes: 32 * 1024, MaxDurationSeconds: 7, MaxPeakDbfs: -1, MaxLoudnessLufs: -20}
}

func readWelcomeSoundFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "storage", "testdata", "welcome-sound", name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func setupWelcomeSoundService(t *testing.T) (*AuthService, string, string) {
	t.Helper()
	dir := t.TempDir()
	t.Cleanup(storage.SetWelcomeSoundDirForTest(dir))
	svc, _ := setupAuthService(t)
	user, err := svc.Register(testEmail, "password123", "Test User")
	if err != nil {
		t.Fatalf("register: %v", err)
	}
	return svc, user.ID, dir
}

func TestAuthService_SetWelcomeSound(t *testing.T) {
	morning := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)

	t.Run("should store the clip and set the URL", func(t *testing.T) {
		svc, userID, dir := setupWelcomeSoundService(t)

		user, err := svc.SetWelcomeSound(userID, readWelcomeSoundFixture(t, "valid.mp3"), testWelcomeSoundLimits(), morning)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(user.WelcomeSoundURL, "/uploads/welcome-sounds/"+userID+".mp3?v=") {
			t.Fatalf("unexpected URL %q", user.WelcomeSoundURL)
		}
		if _, err := os.Stat(filepath.Join(dir, userID+".mp3")); err != nil {
			t.Fatalf("expected the clip on disk: %v", err)
		}
	})

	t.Run("should allow two changes a day and refuse the third", func(t *testing.T) {
		svc, userID, _ := setupWelcomeSoundService(t)
		clip := readWelcomeSoundFixture(t, "valid.mp3")

		for change := 1; change <= 2; change++ {
			if _, err := svc.SetWelcomeSound(userID, clip, testWelcomeSoundLimits(), morning); err != nil {
				t.Fatalf("change %d: unexpected error: %v", change, err)
			}
		}
		_, err := svc.SetWelcomeSound(userID, clip, testWelcomeSoundLimits(), morning.Add(time.Hour))

		if !errors.Is(err, ErrWelcomeSoundDailyLimit) {
			t.Fatalf("expected ErrWelcomeSoundDailyLimit, got %v", err)
		}
	})

	t.Run("should allow changes again on the next UTC day", func(t *testing.T) {
		svc, userID, _ := setupWelcomeSoundService(t)
		clip := readWelcomeSoundFixture(t, "valid.mp3")
		for change := 1; change <= 2; change++ {
			if _, err := svc.SetWelcomeSound(userID, clip, testWelcomeSoundLimits(), morning); err != nil {
				t.Fatalf("change %d: unexpected error: %v", change, err)
			}
		}

		nextDay := time.Date(2026, 10, 24, 0, 0, 1, 0, time.UTC)
		if _, err := svc.SetWelcomeSound(userID, clip, testWelcomeSoundLimits(), nextDay); err != nil {
			t.Fatalf("expected a fresh allowance, got %v", err)
		}
	})

	t.Run("should not count a rejected clip", func(t *testing.T) {
		svc, userID, _ := setupWelcomeSoundService(t)

		for attempt := 1; attempt <= 3; attempt++ {
			_, err := svc.SetWelcomeSound(userID, readWelcomeSoundFixture(t, "too-loud.mp3"), testWelcomeSoundLimits(), morning)
			if !errors.Is(err, storage.ErrWelcomeSoundTooLoud) {
				t.Fatalf("attempt %d: expected ErrWelcomeSoundTooLoud, got %v", attempt, err)
			}
		}
		if _, err := svc.SetWelcomeSound(userID, readWelcomeSoundFixture(t, "valid.mp3"), testWelcomeSoundLimits(), morning); err != nil {
			t.Fatalf("expected the allowance untouched, got %v", err)
		}
	})
}

func TestAuthService_ClearWelcomeSound(t *testing.T) {
	morning := time.Date(2026, 10, 23, 9, 0, 0, 0, time.UTC)

	t.Run("should remove the clip and the URL without using a change", func(t *testing.T) {
		svc, userID, dir := setupWelcomeSoundService(t)
		clip := readWelcomeSoundFixture(t, "valid.mp3")
		if _, err := svc.SetWelcomeSound(userID, clip, testWelcomeSoundLimits(), morning); err != nil {
			t.Fatalf("set: %v", err)
		}

		user, err := svc.ClearWelcomeSound(userID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user.WelcomeSoundURL != "" {
			t.Fatalf("expected an empty URL, got %q", user.WelcomeSoundURL)
		}
		if _, err := os.Stat(filepath.Join(dir, userID+".mp3")); !os.IsNotExist(err) {
			t.Fatalf("expected the clip to be gone, got %v", err)
		}
		if _, err := svc.SetWelcomeSound(userID, clip, testWelcomeSoundLimits(), morning); err != nil {
			t.Fatalf("expected the second change still available, got %v", err)
		}
	})

	t.Run("should report a missing user", func(t *testing.T) {
		svc, _, _ := setupWelcomeSoundService(t)

		if _, err := svc.ClearWelcomeSound("no-such-user"); err == nil {
			t.Fatal("expected an error")
		}
	})
}
