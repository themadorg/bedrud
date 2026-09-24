package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"bedrud/config"

	"gopkg.in/yaml.v3"
)

const commentedConfig = `# bedrud configuration
server:
  port: "8443"
  httpPort: "8080"
  enableTLS: true
auth:
  # Signing key for issued JWTs.
  jwtSecret: "test-jwt-secret-with-enough-length-32"
  sessionSecret: "test-session-secret-long-enough-32"
  tokenDuration: 24
logger:
  level: info
`

func writeConfigFixture(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func readConfigFixture(t *testing.T, path string) (string, *config.Config) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg config.Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		t.Fatalf("config no longer parses: %v\n%s", err, data)
	}
	return string(data), &cfg
}

func TestConfigSetKeepsOtherKeysLoadable(t *testing.T) {
	path := writeConfigFixture(t, commentedConfig)
	_, _ = captureOutput(t)

	if err := executeRoot([]string{"--config", path, "config", "set", "logger.level", "debug"}); err != nil {
		t.Fatalf("config set: %v", err)
	}

	raw, cfg := readConfigFixture(t, path)
	if cfg.Auth.JWTSecret == "" || cfg.Auth.SessionSecret == "" {
		t.Fatalf("camelCase keys were lost:\n%s", raw)
	}
	if cfg.Auth.TokenDuration.Int() != 24 {
		t.Fatalf("tokenDuration lost: %d\n%s", cfg.Auth.TokenDuration.Int(), raw)
	}
	if cfg.Server.Port != "8443" || cfg.Server.HTTPPort != "8080" || !cfg.Server.EnableTLS {
		t.Fatalf("server section changed:\n%s", raw)
	}
	if cfg.Logger.Level != "debug" {
		t.Fatalf("logger.level not applied: %q", cfg.Logger.Level)
	}
	if !strings.Contains(raw, "# bedrud configuration") || !strings.Contains(raw, "# Signing key for issued JWTs.") {
		t.Fatalf("comments were dropped:\n%s", raw)
	}
}

func TestConfigSetCanonicalisesKeyCase(t *testing.T) {
	path := writeConfigFixture(t, commentedConfig)
	_, _ = captureOutput(t)

	if err := executeRoot([]string{"--config", path, "config", "set", "auth.jwtsecret", "another-secret-long-enough-for-use"}); err != nil {
		t.Fatalf("config set: %v", err)
	}

	raw, cfg := readConfigFixture(t, path)
	if cfg.Auth.JWTSecret != "another-secret-long-enough-for-use" {
		t.Fatalf("jwtSecret not applied: %q\n%s", cfg.Auth.JWTSecret, raw)
	}
	if strings.Contains(raw, "jwtsecret:") {
		t.Fatalf("wrote a key the loader ignores:\n%s", raw)
	}
}

func TestConfigSetRepairsLowercasedKey(t *testing.T) {
	path := writeConfigFixture(t, "auth:\n  jwtsecret: \"old-secret-that-the-loader-ignores\"\n")
	_, _ = captureOutput(t)

	if err := executeRoot([]string{"--config", path, "config", "set", "auth.jwtSecret", "new-secret-long-enough-for-use-32"}); err != nil {
		t.Fatalf("config set: %v", err)
	}

	raw, cfg := readConfigFixture(t, path)
	if cfg.Auth.JWTSecret != "new-secret-long-enough-for-use-32" {
		t.Fatalf("jwtSecret not readable after repair: %q\n%s", cfg.Auth.JWTSecret, raw)
	}
	if strings.Count(raw, "wtSecret") != 1 || strings.Contains(raw, "jwtsecret") {
		t.Fatalf("expected the mangled key to be replaced, got:\n%s", raw)
	}
}

func TestConfigSetTypesValuesForTheirField(t *testing.T) {
	path := writeConfigFixture(t, commentedConfig)
	_, _ = captureOutput(t)

	for _, tc := range []struct{ key, value string }{
		{"auth.tokenDuration", "48"},
		{"server.enableTLS", "false"},
		{"server.port", "9443"},
		{"server.trustedProxies", "10.0.0.1, 10.0.0.2"},
	} {
		if err := executeRoot([]string{"--config", path, "config", "set", tc.key, tc.value}); err != nil {
			t.Fatalf("set %s: %v", tc.key, err)
		}
	}

	raw, cfg := readConfigFixture(t, path)
	if cfg.Auth.TokenDuration.Int() != 48 {
		t.Fatalf("tokenDuration: %d\n%s", cfg.Auth.TokenDuration.Int(), raw)
	}
	if cfg.Server.EnableTLS {
		t.Fatalf("enableTLS not cleared:\n%s", raw)
	}
	if cfg.Server.Port != "9443" {
		t.Fatalf("port: %q\n%s", cfg.Server.Port, raw)
	}
	if len(cfg.Server.TrustedProxies) != 2 || cfg.Server.TrustedProxies[1] != "10.0.0.2" {
		t.Fatalf("trustedProxies: %v\n%s", cfg.Server.TrustedProxies, raw)
	}
	if !strings.Contains(raw, `port: "9443"`) {
		t.Fatalf("numeric string field lost its quotes:\n%s", raw)
	}
}

func TestConfigSetCreatesMissingSection(t *testing.T) {
	path := writeConfigFixture(t, commentedConfig)
	_, _ = captureOutput(t)

	if err := executeRoot([]string{"--config", path, "config", "set", "queue.concurrency", "4"}); err != nil {
		t.Fatalf("config set: %v", err)
	}

	raw, cfg := readConfigFixture(t, path)
	if cfg.Queue.Concurrency != 4 {
		t.Fatalf("queue.concurrency: %d\n%s", cfg.Queue.Concurrency, raw)
	}
}

func TestConfigSetRejectsBadInput(t *testing.T) {
	cases := map[string][]string{
		"unknown key":      {"auth.jwtSecrets", "x"},
		"section as value": {"auth", "x"},
		"non-integer":      {"auth.tokenDuration", "soon"},
		"non-boolean":      {"server.enableTLS", "maybe"},
	}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			path := writeConfigFixture(t, commentedConfig)
			_, _ = captureOutput(t)

			err := executeRoot(append([]string{"--config", path, "config", "set"}, args...))
			if err == nil {
				t.Fatal("expected an error")
			}
			raw, _ := readConfigFixture(t, path)
			if raw != commentedConfig {
				t.Fatalf("config was modified despite the error:\n%s", raw)
			}
		})
	}
}

func TestConfigSetPreservesFileMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes are not enforced on Windows")
	}
	path := writeConfigFixture(t, commentedConfig)
	if err := os.Chmod(path, 0o640); err != nil {
		t.Fatal(err)
	}
	_, _ = captureOutput(t)

	if err := executeRoot([]string{"--config", path, "config", "set", "logger.level", "warn"}); err != nil {
		t.Fatalf("config set: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("mode changed to %o", info.Mode().Perm())
	}
}
