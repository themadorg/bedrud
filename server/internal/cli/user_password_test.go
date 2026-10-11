package cli

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/clioutput"
	"bedrud/internal/database"
	"bedrud/internal/repository"
)

var printedPassword = regexp.MustCompile(`New password: (\S+)`)

func createTestUser(t *testing.T, cfgPath, email string) {
	t.Helper()
	if err := executeRoot([]string{
		"--config", cfgPath, "user", "create",
		"--email", email, "--password", "initial-password-123", "--name", "Test User",
	}); err != nil {
		t.Fatalf("create user: %v", err)
	}
}

// assertPasswordWorks reopens the database the CLI wrote to and checks the
// stored hash against the password the command handed the operator.
func assertPasswordWorks(t *testing.T, cfgPath, email, password string) {
	t.Helper()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := database.Initialize(&cfg.Database); err != nil {
		t.Fatal(err)
	}
	defer database.Close()

	user, err := repository.NewUserRepository(database.GetDB()).GetUserByEmail(email)
	if err != nil {
		t.Fatal(err)
	}
	if user == nil {
		t.Fatalf("user %s is gone", email)
	}
	if err := auth.VerifyPassword(password, user.Password); err != nil {
		t.Fatalf("the password the CLI reported does not open the account: %v", err)
	}
}

func TestUserResetPasswordPrintsGeneratedPassword(t *testing.T) {
	cfgPath := writeTestConfig(t)
	out, errBuf := captureOutput(t)
	createTestUser(t, cfgPath, "reset-text@example.com")
	out.Reset()

	if err := executeRoot([]string{"--config", cfgPath, "user", "reset-password", "--email", "reset-text@example.com"}); err != nil {
		t.Fatalf("reset-password: %v\nstderr: %s", err, errBuf.String())
	}

	match := printedPassword.FindStringSubmatch(out.String())
	if match == nil {
		t.Fatalf("reset-password printed no password, the account would be locked out:\n%s", out.String())
	}
	assertPasswordWorks(t, cfgPath, "reset-text@example.com", match[1])
}

func TestUserResetPasswordJSONCarriesGeneratedPassword(t *testing.T) {
	cfgPath := writeTestConfig(t)
	out, errBuf := captureOutput(t)
	createTestUser(t, cfgPath, "reset-json@example.com")
	out.Reset()

	if err := executeRoot([]string{"--json", "--config", cfgPath, "user", "reset-password", "--email", "reset-json@example.com"}); err != nil {
		t.Fatalf("reset-password: %v\nstderr: %s", err, errBuf.String())
	}

	var result clioutput.Result
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, out.String())
	}
	data, _ := result.Data.(map[string]any)
	password, _ := data["password"].(string)
	if password == "" {
		t.Fatalf("no password in JSON output: %s", out.String())
	}
	if strings.Contains(out.String(), "New password:") {
		t.Fatalf("text output leaked into JSON mode: %s", out.String())
	}
	assertPasswordWorks(t, cfgPath, "reset-json@example.com", password)
}

func TestUserPasswordReadsStdin(t *testing.T) {
	cfgPath := writeTestConfig(t)
	_, errBuf := captureOutput(t)
	createTestUser(t, cfgPath, "stdin@example.com")

	root := NewRootCmd()
	root.SetIn(bytes.NewBufferString("piped-password-4567\n"))
	root.SetArgs([]string{"--config", cfgPath, "user", "password", "--email", "stdin@example.com", "--password-stdin"})
	if err := root.Execute(); err != nil {
		t.Fatalf("user password: %v\nstderr: %s", err, errBuf.String())
	}

	assertPasswordWorks(t, cfgPath, "stdin@example.com", "piped-password-4567")
}

func TestUserPasswordRejectsAmbiguousInput(t *testing.T) {
	cases := map[string][]string{
		"no password at all": {"--email", "x@example.com"},
		"both sources":       {"--email", "x@example.com", "--password", "a", "--password-stdin"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			cfgPath := writeTestConfig(t)
			_, _ = captureOutput(t)

			root := NewRootCmd()
			root.SetIn(bytes.NewBufferString("piped\n"))
			root.SetArgs(append([]string{"--config", cfgPath, "user", "password"}, args...))
			if err := root.Execute(); err == nil {
				t.Fatal("expected an error instead of a silent password change")
			}
		})
	}
}
