package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"bedrud/config"
	"bedrud/internal/clioutput"
	"bedrud/internal/database"
	"bedrud/internal/models"
)

func TestVersionJSON(t *testing.T) {
	out, errBuf := captureOutput(t)
	root := NewRootCmd()
	root.SetArgs([]string{"version", "--json"})

	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v\nstderr: %s", err, errBuf.String())
	}

	var result clioutput.Result
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal stdout: %v\nraw: %s", err, out.String())
	}
	if !result.OK {
		t.Fatalf("expected ok result: %+v", result)
	}
	data, ok := result.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected data object, got %T", result.Data)
	}
	if data["name"] != "bedrud" || data["version"] == "" {
		t.Fatalf("unexpected version data: %+v", data)
	}
}

func TestVersionText(t *testing.T) {
	out, _ := captureOutput(t)
	root := NewRootCmd()
	root.SetArgs([]string{"version"})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}
	if out.String() != "bedrud dev\n" {
		t.Fatalf("got %q", out.String())
	}
}

func TestConfigPathJSON(t *testing.T) {
	out, _ := captureOutput(t)
	root := NewRootCmd()
	root.SetArgs([]string{"--json", "config", "path", "--config", "/tmp/custom.yaml"})

	if err := root.Execute(); err != nil {
		t.Fatal(err)
	}

	var result clioutput.Result
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data := result.Data.(map[string]any)
	if data["path"] != "/tmp/custom.yaml" {
		t.Fatalf("unexpected path: %+v", data)
	}
}

func TestUserCreateAndListJSON(t *testing.T) {
	cfgPath := writeTestConfig(t)
	out, errBuf := captureOutput(t)

	root := NewRootCmd()
	root.SetArgs([]string{
		"--json", "--config", cfgPath,
		"user", "create",
		"--email", "cli-json@example.com",
		"--password", "secure-password-123",
		"--name", "CLI JSON",
	})
	if err := root.Execute(); err != nil {
		t.Fatalf("create: %v\nstderr: %s", err, errBuf.String())
	}

	var createResult clioutput.Result
	if err := json.Unmarshal(out.Bytes(), &createResult); err != nil {
		t.Fatalf("create json: %v\nraw: %s", err, out.String())
	}
	if !createResult.OK {
		t.Fatalf("create not ok: %+v", createResult)
	}

	out.Reset()
	errBuf.Reset()
	root = NewRootCmd()
	root.SetArgs([]string{"--json", "--config", cfgPath, "user", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("list: %v\nstderr: %s", err, errBuf.String())
	}

	var listResult clioutput.Result
	if err := json.Unmarshal(out.Bytes(), &listResult); err != nil {
		t.Fatalf("list json: %v\nraw: %s", err, out.String())
	}
	data := listResult.Data.(map[string]any)
	total, _ := data["total"].(float64)
	if total < 1 {
		t.Fatalf("expected at least one user, got %+v", data)
	}
}

func TestUserCreateMissingEmailError(t *testing.T) {
	cfgPath := writeTestConfig(t)
	_, errBuf := captureOutput(t)

	err := executeRoot([]string{"--json", "--config", cfgPath, "user", "create", "--password", "x", "--name", "n"})
	if err == nil {
		t.Fatal("expected error")
	}

	var result clioutput.Result
	if err := json.Unmarshal(errBuf.Bytes(), &result); err != nil {
		t.Fatalf("error json: %v\nstderr: %s", err, errBuf.String())
	}
	if result.OK || result.Message == "" {
		t.Fatalf("expected failed JSON error, got %+v", result)
	}
}

func TestLegacyVersionJSON(t *testing.T) {
	out, _ := captureOutput(t)
	clioutput.SetJSON(true)

	if !dispatchLegacy([]string{"--json", "--version"}) {
		t.Fatal("expected legacy handler")
	}

	var result clioutput.Result
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, out.String())
	}
	if !result.OK {
		t.Fatalf("expected ok: %+v", result)
	}
	data := result.Data.(map[string]any)
	if data["name"] != "bedrud" {
		t.Fatalf("unexpected data: %+v", data)
	}
}

func TestDBStatusJSON(t *testing.T) {
	cfgPath := writeTestConfig(t)
	out, errBuf := captureOutput(t)

	root := NewRootCmd()
	root.SetArgs([]string{"--json", "--config", cfgPath, "db", "status"})
	if err := root.Execute(); err != nil {
		t.Fatalf("db status: %v\nstderr: %s", err, errBuf.String())
	}

	var result clioutput.Result
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, out.String())
	}
	if !result.OK {
		t.Fatalf("expected ok: %+v", result)
	}
	data := result.Data.(map[string]any)
	if data["type"] != "sqlite" || data["status"] != "ok" {
		t.Fatalf("unexpected status data: %+v", data)
	}
}

func executeRoot(args []string) error {
	root := NewRootCmd()
	root.SetArgs(args)
	err := root.Execute()
	if err != nil && clioutput.JSON() {
		clioutput.EmitError(err)
	}
	return err
}

func captureOutput(t *testing.T) (stdout, stderr *bytes.Buffer) {
	t.Helper()
	var out, errOut bytes.Buffer
	clioutput.SetWriters(&out, &errOut)
	clioutput.SetJSON(false)
	t.Cleanup(func() {
		clioutput.ResetWriters()
		clioutput.SetJSON(false)
		config.ResetLoadForTest()
		database.ResetForTest()
		configPath = ""
	})
	return &out, &errOut
}

func writeTestConfig(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	cfgPath := filepath.Join(dir, "config.yaml")
	content := `auth:
  jwtSecret: "test-jwt-secret-with-enough-length-32"
  sessionSecret: "test-session-secret-long-enough-32"
server:
  port: "8090"
  host: "localhost"
database:
  type: sqlite
  path: "` + dbPath + `"
livekit:
  apiKey: test-key
  apiSecret: "test-secret-12345678901234567890123456789012"
  host: "http://localhost:7880"
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfgPath
}

func TestRedactSettings(t *testing.T) {
	s := &models.SystemSettings{
		GoogleClientSecret:  "g",
		GithubClientSecret:  "h",
		TwitterClientSecret: "t",
		JWTSecret:           "j",
		SessionSecret:       "s",
		LiveKitAPIKey:       "k",
		LiveKitAPISecret:    "lk",
		ChatUploadS3AccessKey: "ak",
		ChatUploadS3SecretKey: "sk",
		EmailPassword:       "pw",
		ServerHost:          "keep-me",
	}
	redactSettings(s)
	for _, got := range []string{
		s.GoogleClientSecret, s.GithubClientSecret, s.TwitterClientSecret,
		s.JWTSecret, s.SessionSecret, s.LiveKitAPIKey, s.LiveKitAPISecret,
		s.ChatUploadS3AccessKey, s.ChatUploadS3SecretKey, s.EmailPassword,
	} {
		if got != "***redacted***" {
			t.Fatalf("secret not redacted: %q", got)
		}
	}
	if s.ServerHost != "keep-me" {
		t.Fatalf("non-secret mutated: %q", s.ServerHost)
	}
}
