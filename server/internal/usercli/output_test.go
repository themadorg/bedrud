package usercli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"bedrud/internal/clioutput"
)

func captureCLI(t *testing.T, jsonMode bool) *bytes.Buffer {
	t.Helper()
	var out bytes.Buffer
	clioutput.SetWriters(&out, &bytes.Buffer{})
	clioutput.SetJSON(jsonMode)
	t.Cleanup(func() {
		clioutput.ResetWriters()
		clioutput.SetJSON(false)
	})
	return &out
}

func TestReportPasswordChangePrintsGeneratedPassword(t *testing.T) {
	out := captureCLI(t, false)

	if err := reportPasswordChange("ops@example.com", "generated-secret-9876", true); err != nil {
		t.Fatal(err)
	}

	text := out.String()
	if !strings.Contains(text, "generated-secret-9876") {
		t.Fatalf("the generated password was not shown, the account would be locked out:\n%s", text)
	}
	if !strings.Contains(text, "ops@example.com") {
		t.Fatalf("no confirmation line:\n%s", text)
	}
}

func TestReportPasswordChangeDoesNotEchoSuppliedPassword(t *testing.T) {
	out := captureCLI(t, false)

	if err := reportPasswordChange("ops@example.com", "operator-chosen-secret", false); err != nil {
		t.Fatal(err)
	}

	text := out.String()
	if strings.Contains(text, "operator-chosen-secret") {
		t.Fatalf("a password the operator already holds was echoed:\n%s", text)
	}
	if !strings.Contains(text, "Password updated for ops@example.com") {
		t.Fatalf("no confirmation line:\n%s", text)
	}
}

func TestReportPasswordChangeJSONCarriesGeneratedPassword(t *testing.T) {
	out := captureCLI(t, true)

	if err := reportPasswordChange("ops@example.com", "generated-secret-9876", true); err != nil {
		t.Fatal(err)
	}

	var result clioutput.Result
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, out.String())
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected a data object, got %T", result.Data)
	}
	if data["password"] != "generated-secret-9876" {
		t.Fatalf("password missing from JSON data: %+v", data)
	}
	if strings.Contains(out.String(), "New password:") {
		t.Fatalf("text output leaked into JSON mode:\n%s", out.String())
	}
}

func TestReportPasswordChangeJSONOmitsSuppliedPassword(t *testing.T) {
	out := captureCLI(t, true)

	if err := reportPasswordChange("ops@example.com", "operator-chosen-secret", false); err != nil {
		t.Fatal(err)
	}

	if strings.Contains(out.String(), "operator-chosen-secret") {
		t.Fatalf("a password the operator already holds was echoed:\n%s", out.String())
	}
}
