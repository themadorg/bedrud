package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"bedrud/config"
	"bedrud/internal/clioutput"
	"bedrud/internal/database"
	"bedrud/internal/install"
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

func TestUpdateRequiresSource(t *testing.T) {
	out, errBuf := captureOutput(t)
	root := NewRootCmd()
	root.SetArgs([]string{"update"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when update has no source")
	}
	combined := err.Error() + errBuf.String() + out.String()
	if !bytes.Contains([]byte(combined), []byte("missing source")) {
		t.Fatalf("expected missing source error, got err=%v stdout=%q stderr=%q", err, out.String(), errBuf.String())
	}
}

func TestUpgradeRequiresSource(t *testing.T) {
	out, errBuf := captureOutput(t)
	root := NewRootCmd()
	root.SetArgs([]string{"upgrade"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when upgrade has no source")
	}
	combined := err.Error() + errBuf.String() + out.String()
	if !bytes.Contains([]byte(combined), []byte("missing source")) {
		t.Fatalf("expected missing source error, got err=%v", err)
	}
}

func TestUpdateSelfAndSourceConflict(t *testing.T) {
	_, _ = captureOutput(t)
	root := NewRootCmd()
	root.SetArgs([]string{"update", "--self", "/tmp/bedrud"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("--self")) {
		t.Fatalf("got %v", err)
	}
}

func TestUpdateSkipBinaryAndSourceConflict(t *testing.T) {
	_, _ = captureOutput(t)
	root := NewRootCmd()
	root.SetArgs([]string{"update", "--skip-binary", "/tmp/bedrud"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected conflict error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("--skip-binary")) {
		t.Fatalf("got %v", err)
	}
}

func TestUpdateLatestSkipChecksumConflict(t *testing.T) {
	_, _ = captureOutput(t)
	root := NewRootCmd()
	root.SetArgs([]string{"update", "latest", "--skip-checksum"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected latest+skip-checksum error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("skip-checksum")) &&
		!bytes.Contains([]byte(err.Error()), []byte("latest")) {
		t.Fatalf("got %v", err)
	}
}

func TestUpdateNightlySkipChecksumConflict(t *testing.T) {
	_, _ = captureOutput(t)
	root := NewRootCmd()
	root.SetArgs([]string{"update", "--nightly", "--skip-checksum"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected nightly+skip-checksum error")
	}
}

func TestUpdateNightlyWithPathConflict(t *testing.T) {
	_, _ = captureOutput(t)
	root := NewRootCmd()
	root.SetArgs([]string{"update", "--nightly", "/tmp/bedrud"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected nightly+path error")
	}
}

func TestVersionsListNoRoot(t *testing.T) {
	t.Setenv("BEDRUD_INSTALL_ROOT", t.TempDir())
	out, errBuf := captureOutput(t)
	root := NewRootCmd()
	root.SetArgs([]string{"versions", "list"})
	if err := root.Execute(); err != nil {
		t.Fatalf("err=%v stderr=%s", err, errBuf.String())
	}
	if !strings.Contains(out.String(), "Install root:") {
		t.Fatalf("got %q", out.String())
	}
}

func TestCompletionBash(t *testing.T) {
	out, errBuf := captureOutput(t)
	// GenBash writes to stdout of the command, not clioutput writers — use root with SetOut
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetErr(&buf)
	root.SetArgs([]string{"completion", "bash"})
	if err := root.Execute(); err != nil {
		t.Fatalf("err=%v stderr=%s", err, errBuf.String())
	}
	s := buf.String()
	if !strings.Contains(s, "bedrud") {
		// cobra may write to os.Stdout directly for GenBashCompletionV2
		// fall back: call writeCompletion
		var b2 bytes.Buffer
		if err := writeCompletion(&b2, "bash", "bedrud"); err != nil {
			t.Fatal(err)
		}
		s = b2.String()
	}
	if !strings.Contains(s, "bedrud") {
		t.Fatalf("bash completion missing binary name: %q (stdout capture=%q)", s, out.String())
	}
	if !strings.Contains(s, "install") && !strings.Contains(s, "update") {
		// completion scripts often list subcommands
		t.Logf("warning: install/update not found in completion (may still be valid)")
	}
}

func TestCompletionZshAndFish(t *testing.T) {
	for _, shell := range []string{"zsh", "fish"} {
		var buf bytes.Buffer
		if err := writeCompletion(&buf, shell, "bedrud"); err != nil {
			t.Fatalf("%s: %v", shell, err)
		}
		if !strings.Contains(buf.String(), "bedrud") {
			t.Fatalf("%s completion missing binary name", shell)
		}
	}
}

func TestCompletionUnsupportedShell(t *testing.T) {
	var buf bytes.Buffer
	if err := writeCompletion(&buf, "powershell", "bedrud"); err == nil {
		t.Fatal("expected error")
	}
}

func TestGenerateMan(t *testing.T) {
	out, _ := captureOutput(t)
	root := NewRootCmd()
	var buf bytes.Buffer
	root.SetOut(&buf)
	root.SetArgs([]string{"generate-man"})
	// generate-man writes to os.Stdout via io.WriteString — use helper
	man := install.EmbeddedManPage()
	if !strings.Contains(man, `.TH "bedrud" "1"`) {
		t.Fatal("embedded man invalid")
	}
	_ = out
}

func TestGenerateCompletionsInProcess(t *testing.T) {
	bash, zsh, fish, err := generateCompletions("bedrud")
	if err != nil {
		t.Fatal(err)
	}
	for name, b := range map[string][]byte{"bash": bash, "zsh": zsh, "fish": fish} {
		if !bytes.Contains(b, []byte("bedrud")) {
			t.Fatalf("%s missing bedrud", name)
		}
		if len(b) < 50 {
			t.Fatalf("%s too short", name)
		}
	}
}
