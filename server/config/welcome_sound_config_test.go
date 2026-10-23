package config

import (
	"os"
	"testing"
)

func TestLoad_WelcomeSound(t *testing.T) {
	writeConfig := func(t *testing.T, body string) string {
		t.Helper()
		path := t.TempDir() + "/config.yaml"
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}
		return path
	}
	const yamlBody = `
auth:
  jwtSecret: "test-jwt-secret-at-least-32-chars!!"
  sessionSecret: "test-session-secret-32chars-long!"
database:
  type: "sqlite"
  path: ":memory:"
welcomeSound:
  maxBytes: 40000
  maxDurationSeconds: 5
  maxPeakDbfs: -3
  maxLoudnessLufs: -24
`

	t.Run("should read welcome sound limits from yaml", func(t *testing.T) {
		ResetLoadForTest()
		t.Cleanup(ResetLoadForTest)

		cfg, err := Load(writeConfig(t, yamlBody))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		got := cfg.WelcomeSound
		if got.MaxBytes != 40000 || got.MaxDurationSeconds != 5 || got.MaxPeakDbfs != -3 || got.MaxLoudnessLufs != -24 {
			t.Fatalf("unexpected welcome sound config: %+v", got)
		}
	})

	t.Run("should let environment variables override yaml", func(t *testing.T) {
		ResetLoadForTest()
		t.Cleanup(ResetLoadForTest)
		t.Setenv("WELCOME_SOUND_MAX_BYTES", "20480")
		t.Setenv("WELCOME_SOUND_MAX_DURATION_SECONDS", "4")
		t.Setenv("WELCOME_SOUND_MAX_PEAK_DBFS", "-2.5")
		t.Setenv("WELCOME_SOUND_MAX_LOUDNESS_LUFS", "-22")

		cfg, err := Load(writeConfig(t, yamlBody))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		got := cfg.WelcomeSound
		if got.MaxBytes != 20480 || got.MaxDurationSeconds != 4 || got.MaxPeakDbfs != -2.5 || got.MaxLoudnessLufs != -22 {
			t.Fatalf("unexpected welcome sound config: %+v", got)
		}
	})

	t.Run("should ignore invalid environment values", func(t *testing.T) {
		ResetLoadForTest()
		t.Cleanup(ResetLoadForTest)
		t.Setenv("WELCOME_SOUND_MAX_BYTES", "lots")
		t.Setenv("WELCOME_SOUND_MAX_PEAK_DBFS", "quiet")

		cfg, err := Load(writeConfig(t, yamlBody))
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if cfg.WelcomeSound.MaxBytes != 40000 || cfg.WelcomeSound.MaxPeakDbfs != -3 {
			t.Fatalf("expected yaml values to survive invalid env, got %+v", cfg.WelcomeSound)
		}
	})
}
