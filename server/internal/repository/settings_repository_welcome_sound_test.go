package repository

import (
	"testing"

	"bedrud/config"
	"bedrud/internal/models"
	"bedrud/internal/testutil"
)

func TestSettingsRepository_WelcomeSound(t *testing.T) {
	configuredLimits := func() *config.Config {
		return &config.Config{WelcomeSound: config.WelcomeSoundConfig{
			MaxBytes:           40000,
			MaxDurationSeconds: 5,
			MaxPeakDbfs:        -3,
			MaxLoudnessLufs:    -24,
		}}
	}

	t.Run("should keep the feature disabled by default", func(t *testing.T) {
		repo := NewSettingsRepository(testutil.SetupTestDB(t))

		settings, err := repo.GetEffectiveSettings()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if settings.WelcomeSoundEnabled {
			t.Fatal("expected WelcomeSoundEnabled to default to false")
		}
	})

	t.Run("should fill built-in limits when neither database nor config sets them", func(t *testing.T) {
		repo := NewSettingsRepository(testutil.SetupTestDB(t))

		settings, err := repo.GetEffectiveSettings()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if settings.WelcomeSoundMaxBytes != config.DefaultWelcomeSoundMaxBytes ||
			settings.WelcomeSoundMaxDurationSeconds != config.DefaultWelcomeSoundMaxDurationSeconds ||
			settings.WelcomeSoundMaxPeakDbfs != config.DefaultWelcomeSoundMaxPeakDbfs ||
			settings.WelcomeSoundMaxLoudnessLufs != config.DefaultWelcomeSoundMaxLoudnessLufs {
			t.Fatalf("expected built-in defaults, got %+v", welcomeSoundFields(settings))
		}
	})

	t.Run("should fill empty limits from config", func(t *testing.T) {
		settings := &models.SystemSettings{}
		mergeFromConfig(settings, configuredLimits())

		if settings.WelcomeSoundMaxBytes != 40000 ||
			settings.WelcomeSoundMaxDurationSeconds != 5 ||
			settings.WelcomeSoundMaxPeakDbfs != -3 ||
			settings.WelcomeSoundMaxLoudnessLufs != -24 {
			t.Fatalf("expected config limits, got %+v", welcomeSoundFields(settings))
		}
	})

	t.Run("should prefer limits saved by an admin over config", func(t *testing.T) {
		settings := &models.SystemSettings{
			WelcomeSoundMaxBytes:           10000,
			WelcomeSoundMaxDurationSeconds: 3,
			WelcomeSoundMaxPeakDbfs:        -6,
			WelcomeSoundMaxLoudnessLufs:    -30,
		}
		mergeFromConfig(settings, configuredLimits())

		if settings.WelcomeSoundMaxBytes != 10000 ||
			settings.WelcomeSoundMaxDurationSeconds != 3 ||
			settings.WelcomeSoundMaxPeakDbfs != -6 ||
			settings.WelcomeSoundMaxLoudnessLufs != -30 {
			t.Fatalf("expected admin limits to win, got %+v", welcomeSoundFields(settings))
		}
	})
}

// welcomeSoundFields collects the welcome sound fields for readable failure output.
func welcomeSoundFields(settings *models.SystemSettings) map[string]any {
	return map[string]any{
		"enabled":            settings.WelcomeSoundEnabled,
		"maxBytes":           settings.WelcomeSoundMaxBytes,
		"maxDurationSeconds": settings.WelcomeSoundMaxDurationSeconds,
		"maxPeakDbfs":        settings.WelcomeSoundMaxPeakDbfs,
		"maxLoudnessLufs":    settings.WelcomeSoundMaxLoudnessLufs,
	}
}
