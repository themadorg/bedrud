package storage

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const welcomeSoundFixtureDir = "testdata/welcome-sound"

// Matches ffmpeg's EBU R128 readings in testdata/welcome-sound/README.md within MP3 decoder differences.
const loudnessToleranceLu = 0.5

func testWelcomeSoundLimits() WelcomeSoundLimits {
	return WelcomeSoundLimits{
		MaxBytes:           32 * 1024,
		MaxDurationSeconds: 7,
		MaxPeakDbfs:        -1,
		MaxLoudnessLufs:    -20,
	}
}

func readWelcomeSoundFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(welcomeSoundFixtureDir, name))
	if err != nil {
		t.Fatalf("read fixture %s: %v", name, err)
	}
	return data
}

func TestAnalyzeWelcomeSound(t *testing.T) {
	t.Run("should accept a quiet mono clip and match the reference reading", func(t *testing.T) {
		analysis, err := AnalyzeWelcomeSound(readWelcomeSoundFixture(t, "valid.mp3"), testWelcomeSoundLimits())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if math.Abs(analysis.LoudnessLufs-(-30.0)) > loudnessToleranceLu {
			t.Fatalf("expected about -30.0 LUFS, got %.2f", analysis.LoudnessLufs)
		}
		if math.Abs(analysis.Duration.Seconds()-2) > 0.1 {
			t.Fatalf("expected about 2 s, got %s", analysis.Duration)
		}
	})

	t.Run("should measure true stereo as two channels", func(t *testing.T) {
		analysis, err := AnalyzeWelcomeSound(readWelcomeSoundFixture(t, "valid-stereo.mp3"), testWelcomeSoundLimits())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if math.Abs(analysis.LoudnessLufs-(-27.1)) > loudnessToleranceLu {
			t.Fatalf("expected about -27.1 LUFS, got %.2f", analysis.LoudnessLufs)
		}
	})

	rejections := []struct {
		name    string
		fixture string
		limits  func(WelcomeSoundLimits) WelcomeSoundLimits
		want    error
	}{
		{name: "should reject a clip louder than the loudness limit", fixture: "too-loud.mp3", want: ErrWelcomeSoundTooLoud},
		{name: "should reject a clip whose peak reaches the peak limit", fixture: "peaky.mp3", want: ErrWelcomeSoundPeakTooHigh},
		{name: "should reject a silent clip", fixture: "silent.mp3", want: ErrWelcomeSoundSilent},
		{name: "should reject a 22.05 kHz clip the decoder measures wrongly", fixture: "unsupported-rate.mp3", want: ErrWelcomeSoundSampleRate},
		{name: "should reject a clip longer than the duration limit", fixture: "too-long.mp3", want: ErrWelcomeSoundTooLong,
			limits: func(limits WelcomeSoundLimits) WelcomeSoundLimits { limits.MaxBytes = 64 * 1024; return limits }},
		{name: "should reject a clip larger than the size limit", fixture: "valid.mp3", want: ErrWelcomeSoundTooLarge,
			limits: func(limits WelcomeSoundLimits) WelcomeSoundLimits { limits.MaxBytes = 4096; return limits }},
	}
	for _, rejection := range rejections {
		t.Run(rejection.name, func(t *testing.T) {
			limits := testWelcomeSoundLimits()
			if rejection.limits != nil {
				limits = rejection.limits(limits)
			}

			_, err := AnalyzeWelcomeSound(readWelcomeSoundFixture(t, rejection.fixture), limits)

			if !errors.Is(err, rejection.want) {
				t.Fatalf("expected %v, got %v", rejection.want, err)
			}
		})
	}

	t.Run("should reject data that is not MP3", func(t *testing.T) {
		pngHeader := []byte("\x89PNG\r\n\x1a\n" + strings.Repeat("x", 64))

		_, err := AnalyzeWelcomeSound(pngHeader, testWelcomeSoundLimits())

		if !errors.Is(err, ErrWelcomeSoundUnsupported) {
			t.Fatalf("expected ErrWelcomeSoundUnsupported, got %v", err)
		}
	})

	t.Run("should reject an empty upload", func(t *testing.T) {
		_, err := AnalyzeWelcomeSound(nil, testWelcomeSoundLimits())

		if !errors.Is(err, ErrWelcomeSoundUnsupported) {
			t.Fatalf("expected ErrWelcomeSoundUnsupported, got %v", err)
		}
	})
}

func TestSaveUserWelcomeSound(t *testing.T) {
	t.Run("should store a valid clip and return a versioned URL", func(t *testing.T) {
		dir := t.TempDir()
		t.Cleanup(SetWelcomeSoundDirForTest(dir))

		url, err := SaveUserWelcomeSound("user-1", readWelcomeSoundFixture(t, "valid.mp3"), testWelcomeSoundLimits())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(url, "/uploads/welcome-sounds/user-1.mp3?v=") {
			t.Fatalf("unexpected URL %q", url)
		}
		if _, err := os.Stat(filepath.Join(dir, "user-1.mp3")); err != nil {
			t.Fatalf("expected the clip on disk: %v", err)
		}
	})

	t.Run("should write nothing when the clip is rejected", func(t *testing.T) {
		dir := t.TempDir()
		t.Cleanup(SetWelcomeSoundDirForTest(dir))

		if _, err := SaveUserWelcomeSound("user-1", readWelcomeSoundFixture(t, "too-loud.mp3"), testWelcomeSoundLimits()); err == nil {
			t.Fatal("expected a rejection")
		}
		if _, err := os.Stat(filepath.Join(dir, "user-1.mp3")); !os.IsNotExist(err) {
			t.Fatalf("expected no file, got %v", err)
		}
	})

	t.Run("should require a user id", func(t *testing.T) {
		t.Cleanup(SetWelcomeSoundDirForTest(t.TempDir()))

		if _, err := SaveUserWelcomeSound("", readWelcomeSoundFixture(t, "valid.mp3"), testWelcomeSoundLimits()); err == nil {
			t.Fatal("expected an error for an empty user id")
		}
	})
}

func TestDeleteUserWelcomeSoundFile(t *testing.T) {
	t.Run("should remove the stored clip", func(t *testing.T) {
		dir := t.TempDir()
		t.Cleanup(SetWelcomeSoundDirForTest(dir))
		if _, err := SaveUserWelcomeSound("user-1", readWelcomeSoundFixture(t, "valid.mp3"), testWelcomeSoundLimits()); err != nil {
			t.Fatalf("save: %v", err)
		}

		if err := DeleteUserWelcomeSoundFile("user-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if _, err := os.Stat(filepath.Join(dir, "user-1.mp3")); !os.IsNotExist(err) {
			t.Fatalf("expected the clip to be gone, got %v", err)
		}
	})

	t.Run("should succeed when there is nothing to remove", func(t *testing.T) {
		t.Cleanup(SetWelcomeSoundDirForTest(t.TempDir()))

		if err := DeleteUserWelcomeSoundFile("user-1"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})
}

func TestResolveWelcomeSoundFile(t *testing.T) {
	t.Run("should resolve a clip inside the welcome sound folder and drop the version", func(t *testing.T) {
		dir := t.TempDir()
		t.Cleanup(SetWelcomeSoundDirForTest(dir))

		resolved, err := ResolveWelcomeSoundFile("user-1.mp3?v=123")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if resolved != filepath.Join(dir, "user-1.mp3") {
			t.Fatalf("unexpected path %q", resolved)
		}
	})

	t.Run("should refuse paths that leave the folder", func(t *testing.T) {
		t.Cleanup(SetWelcomeSoundDirForTest(t.TempDir()))

		for _, path := range []string{"../secret.mp3", "", "a/../../b.mp3"} {
			if _, err := ResolveWelcomeSoundFile(path); err == nil {
				t.Errorf("expected %q to be refused", path)
			}
		}
	})
}
