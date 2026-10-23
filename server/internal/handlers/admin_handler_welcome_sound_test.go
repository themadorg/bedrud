package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"bedrud/config"
)

func TestApplySettingsFields_WelcomeSound(t *testing.T) {
	t.Run("should apply the switch and every limit", func(t *testing.T) {
		existing := defaultSettings()
		raw := map[string]json.RawMessage{
			"welcomeSoundEnabled":            json.RawMessage(`true`),
			"welcomeSoundMaxBytes":           json.RawMessage(`20480`),
			"welcomeSoundMaxDurationSeconds": json.RawMessage(`5`),
			"welcomeSoundMaxPeakDbfs":        json.RawMessage(`-2.5`),
			"welcomeSoundMaxLoudnessLufs":    json.RawMessage(`-23`),
		}
		if err := applySettingsFields(&existing, raw); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !existing.WelcomeSoundEnabled ||
			existing.WelcomeSoundMaxBytes != 20480 ||
			existing.WelcomeSoundMaxDurationSeconds != 5 ||
			existing.WelcomeSoundMaxPeakDbfs != -2.5 ||
			existing.WelcomeSoundMaxLoudnessLufs != -23 {
			t.Fatalf("fields not applied: %+v", existing)
		}
	})

	t.Run("should reject values outside their range", func(t *testing.T) {
		cases := map[string]string{
			"welcomeSoundMaxBytes":           `2000000`,
			"welcomeSoundMaxDurationSeconds": `31`,
			"welcomeSoundMaxPeakDbfs":        `1`,
			"welcomeSoundMaxLoudnessLufs":    `-80`,
		}
		for key, value := range cases {
			existing := defaultSettings()
			raw := map[string]json.RawMessage{key: json.RawMessage(value)}
			if err := applySettingsFields(&existing, raw); err == nil {
				t.Errorf("%s=%s: expected a range error", key, value)
			}
		}
	})

	t.Run("should reject a limit sent with the wrong type", func(t *testing.T) {
		existing := defaultSettings()
		raw := map[string]json.RawMessage{"welcomeSoundMaxPeakDbfs": json.RawMessage(`"loud"`)}
		if err := applySettingsFields(&existing, raw); err == nil {
			t.Fatal("expected a type error")
		}
	})
}

func TestAdminHandler_GetPublicSettings_WelcomeSound(t *testing.T) {
	readWelcomeSound := func(t *testing.T, appResponse *http.Response) map[string]any {
		t.Helper()
		body, _ := io.ReadAll(appResponse.Body)
		var result map[string]any
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatal(err)
		}
		welcomeSound, ok := result["welcomeSound"].(map[string]any)
		if !ok {
			t.Fatalf("expected a welcomeSound object, got %s", string(body))
		}
		return welcomeSound
	}

	t.Run("should expose the switch off and the built-in limits by default", func(t *testing.T) {
		app, _, _ := setupAdminTestApp(t)

		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/public/settings", http.NoBody), -1)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		welcomeSound := readWelcomeSound(t, resp)

		if welcomeSound["enabled"] != false {
			t.Fatalf("expected enabled false, got %v", welcomeSound["enabled"])
		}
		if welcomeSound["maxBytes"] != float64(config.DefaultWelcomeSoundMaxBytes) {
			t.Fatalf("expected default maxBytes, got %v", welcomeSound["maxBytes"])
		}
		if welcomeSound["maxDurationSeconds"] != float64(config.DefaultWelcomeSoundMaxDurationSeconds) {
			t.Fatalf("expected default maxDurationSeconds, got %v", welcomeSound["maxDurationSeconds"])
		}
	})

	t.Run("should reflect limits saved by an admin", func(t *testing.T) {
		app, _, _ := setupAdminTestApp(t)
		update, _ := json.Marshal(map[string]any{
			"welcomeSoundEnabled":            true,
			"welcomeSoundMaxBytes":           20480,
			"welcomeSoundMaxDurationSeconds": 4,
		})
		putRequest := httptest.NewRequest(http.MethodPut, "/admin/settings", bytes.NewReader(update))
		putRequest.Header.Set("Content-Type", "application/json")
		putResponse, err := app.Test(putRequest, -1)
		if err != nil {
			t.Fatal(err)
		}
		putResponse.Body.Close()
		if putResponse.StatusCode != http.StatusOK {
			t.Fatalf("expected 200 from update, got %d", putResponse.StatusCode)
		}

		resp, err := app.Test(httptest.NewRequest(http.MethodGet, "/public/settings", http.NoBody), -1)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		welcomeSound := readWelcomeSound(t, resp)

		if welcomeSound["enabled"] != true || welcomeSound["maxBytes"] != float64(20480) || welcomeSound["maxDurationSeconds"] != float64(4) {
			t.Fatalf("unexpected welcomeSound block: %v", welcomeSound)
		}
	})
}
