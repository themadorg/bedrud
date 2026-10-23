package config

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDatabaseConfig_GetDSN(t *testing.T) {
	cfg := &DatabaseConfig{
		User:     "bedrud",
		Password: "secret",
		Host:     "localhost",
		Port:     "5432",
		DBName:   "bedrud_db",
		SSLMode:  "disable",
	}

	expected := "postgresql://bedrud:secret@localhost:5432/bedrud_db?sslmode=disable"
	got := cfg.GetDSN()
	if got != expected {
		t.Fatalf("expected DSN '%s', got '%s'", expected, got)
	}
}

func TestDatabaseConfig_GetDSN_SpecialChars(t *testing.T) {
	cfg := &DatabaseConfig{
		User:     "admin",
		Password: "p@ss:w0rd",
		Host:     "db.example.com",
		Port:     "5433",
		DBName:   "mydb",
		SSLMode:  "require",
	}

	expected := "postgresql://admin:p@ss:w0rd@db.example.com:5433/mydb?sslmode=require"
	got := cfg.GetDSN()
	if got != expected {
		t.Fatalf("expected DSN '%s', got '%s'", expected, got)
	}
}

func TestConfig_StructDefaults(t *testing.T) {
	cfg := Config{}

	// All sub-structs should be zero-valued
	if cfg.Server.Port != "" {
		t.Fatal("expected empty default Port")
	}
	if cfg.Server.EnableTLS {
		t.Fatal("expected EnableTLS to default to false")
	}
	if cfg.Server.UseACME {
		t.Fatal("expected UseACME to default to false")
	}
	if cfg.Database.Type != "" {
		t.Fatal("expected empty default database type")
	}
	if cfg.LiveKit.Host != "" {
		t.Fatal("expected empty default livekit host")
	}
	if cfg.Auth.TokenDuration != 0 {
		t.Fatal("expected token duration default to 0")
	}
	if cfg.Cors.AllowCredentials {
		t.Fatal("expected AllowCredentials default to false")
	}
	if cfg.Cors.MaxAge != 0 {
		t.Fatal("expected MaxAge default to 0")
	}
}

func TestServerConfig_Fields(t *testing.T) {
	s := ServerConfig{
		Port:      "8080",
		Host:      "0.0.0.0",
		EnableTLS: true,
		CertFile:  "/etc/cert.pem",
		KeyFile:   "/etc/key.pem",
		Domain:    "example.com",
		Email:     "admin@example.com",
		UseACME:   true,
	}

	if s.Port != "8080" {
		t.Fatalf("expected Port '8080', got '%s'", s.Port)
	}
	if s.Host != "0.0.0.0" {
		t.Fatalf("expected Host '0.0.0.0', got '%s'", s.Host)
	}
	if !s.EnableTLS {
		t.Fatal("expected EnableTLS true")
	}
	if !s.UseACME {
		t.Fatal("expected UseACME true")
	}
}

func TestLiveKitConfig_Fields(t *testing.T) {
	lk := LiveKitConfig{
		Host:          "https://livekit.example.com",
		InternalHost:  "http://localhost:7880",
		APIKey:        "key123",
		APISecret:     "secret456",
		SkipTLSVerify: true,
	}

	if lk.Host != "https://livekit.example.com" {
		t.Fatalf("unexpected Host: %s", lk.Host)
	}
	if !lk.SkipTLSVerify {
		t.Fatal("expected SkipTLSVerify true")
	}
}

func TestCorsConfig_Fields(t *testing.T) {
	c := CorsConfig{
		AllowedOrigins:   "http://localhost:3000",
		AllowedHeaders:   "Content-Type,Authorization",
		AllowedMethods:   "GET,POST",
		AllowCredentials: true,
		ExposeHeaders:    "X-Custom",
		MaxAge:           3600,
	}

	if c.AllowedOrigins != "http://localhost:3000" {
		t.Fatalf("unexpected AllowedOrigins: %s", c.AllowedOrigins)
	}
	if !c.AllowCredentials {
		t.Fatal("expected AllowCredentials true")
	}
	if c.ExposeHeaders != "X-Custom" {
		t.Fatalf("unexpected ExposeHeaders: %s", c.ExposeHeaders)
	}
	if c.MaxAge != 3600 {
		t.Fatalf("expected MaxAge 3600, got %d", c.MaxAge)
	}
}

// TestParse_UserConfig verifies that the exact config the user runs parses
// without errors — this is the backward-compat test for string-typed numerics.
func TestParse_UserConfig(t *testing.T) {
	y := `
server:
  port: "8443"
  httpPort: "8440"
  enableTLS: true
  certFile: "crt.pem"
  keyFile: "key.pem"
  host: "192.168.70.159"
  readTimeout: 30
  writeTimeout: 30

database:
  type: "sqlite"
  path: "./bedrud-local.db"

logger:
  level: "debug"
  outputPath: ""

livekit:
  host: "https://192.168.70.159:8443/livekit"
  internalHost: "http://127.0.0.1:7880"
  apiKey: "devkey"
  apiSecret: "ce66a6fc4e6c0c961202035f58f710f377f42d9d8416e670a62a1b3515d6aec63bd23e56d1836aca630d499067022e7e"

auth:
  jwtSecret: "6615786f478eb6a3a2e57afb6b10886b38a3b38eea7834180323f07339e512cb"
  sessionSecret: "6615786f478eb6a3a2e57afb6b10886b38a3b38eea7834180323f07339e512cb"
  tokenDuration: 24
  frontendURL: "https://192.168.70.159:8443"
  google:
    clientId: "CHANGE_ME_GOOGLE_CLIENT_ID"
    clientSecret: "CHANGE_ME_GOOGLE_CLIENT_SECRET"
    redirectUrl: "http://localhost:8090/api/auth/google/callback"
  github:
    clientId: "CHANGE_ME_GITHUB_CLIENT_ID"
    clientSecret: "CHANGE_ME_GITHUB_CLIENT_SECRET"
    redirectUrl: "http://localhost:8090/api/auth/github/callback"
  twitter:
    clientId: ""
    clientSecret: ""
    redirectUrl: "http://localhost:8090/api/auth/twitter/callback"

cors:
  allowedOrigins: "https://192.168.70.159:8443,http://192.168.70.159:8440,http://localhost:8090,http://localhost:3000,http://localhost:5173"
  allowedHeaders: "Origin, Content-Type, Accept, Authorization"
  allowedMethods: "GET, POST, PUT, DELETE, OPTIONS"
  allowCredentials: true
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(y), &cfg); err != nil {
		t.Fatalf("failed to parse user config: %v", err)
	}

	tests := []struct {
		name string
		got  any
		want any
	}{
		{"server.port", cfg.Server.Port, "8443"},
		{"server.httpPort", cfg.Server.HTTPPort, "8440"},
		{"server.host", cfg.Server.Host, "192.168.70.159"},
		{"server.readTimeout", cfg.Server.ReadTimeout.Int(), 30},
		{"server.writeTimeout", cfg.Server.WriteTimeout.Int(), 30},
		{"server.enableTLS", cfg.Server.EnableTLS, true},
		{"server.certFile", cfg.Server.CertFile, "crt.pem"},
		{"server.keyFile", cfg.Server.KeyFile, "key.pem"},
		{"database.type", cfg.Database.Type, "sqlite"},
		{"database.path", cfg.Database.Path, "./bedrud-local.db"},
		{"livekit.host", cfg.LiveKit.Host, "https://192.168.70.159:8443/livekit"},
		{"livekit.apiKey", cfg.LiveKit.APIKey, "devkey"},
		{"auth.tokenDuration", cfg.Auth.TokenDuration.Int(), 24},
		{"auth.frontendURL", cfg.Auth.FrontendURL, "https://192.168.70.159:8443"},
		{"cors.allowCredentials", cfg.Cors.AllowCredentials, true},
	}
	for _, tt := range tests {
		if tt.got != tt.want {
			t.Errorf("field %s = %v, want %v", tt.name, tt.got, tt.want)
		}
	}
}

// TestParse_StringNumerics asserts that numeric fields accept quoted strings.
// This is the core backward-compat requirement — the user's binary failed
// because the old uint32 fields rejected "7880".
func TestParse_StringNumerics(t *testing.T) {
	y := `
server:
  readTimeout: "30"
  writeTimeout: "30"

auth:
  tokenDuration: "24"

cors:
  maxAge: "3600"

chat:
  uploads:
    maxBytes: "10485760"
    inlineMaxBytes: "512000"
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(y), &cfg); err != nil {
		t.Fatalf("failed to parse string numerics: %v", err)
	}

	if cfg.Server.ReadTimeout.Int() != 30 {
		t.Fatalf("ReadTimeout: got %d, want 30", cfg.Server.ReadTimeout.Int())
	}
	if cfg.Server.WriteTimeout.Int() != 30 {
		t.Fatalf("WriteTimeout: got %d, want 30", cfg.Server.WriteTimeout.Int())
	}
	if cfg.Auth.TokenDuration.Int() != 24 {
		t.Fatalf("TokenDuration: got %d, want 24", cfg.Auth.TokenDuration.Int())
	}
	if cfg.Cors.MaxAge.Int() != 3600 {
		t.Fatalf("MaxAge: got %d, want 3600", cfg.Cors.MaxAge.Int())
	}
	if cfg.Chat.Uploads.MaxBytes.Int64() != 10485760 {
		t.Fatalf("MaxBytes: got %d, want 10485760", cfg.Chat.Uploads.MaxBytes.Int64())
	}
	if cfg.Chat.Uploads.InlineMaxBytes.Int64() != 512000 {
		t.Fatalf("InlineMaxBytes: got %d, want 512000", cfg.Chat.Uploads.InlineMaxBytes.Int64())
	}
}

// TestParse_IntNumerics asserts that unquoted ints still work.
func TestParse_IntNumerics(t *testing.T) {
	y := `
server:
  readTimeout: 30
  writeTimeout: 30

auth:
  tokenDuration: 24

cors:
  maxAge: 3600

chat:
  uploads:
    maxBytes: 10485760
    inlineMaxBytes: 512000
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(y), &cfg); err != nil {
		t.Fatalf("failed to parse int numerics: %v", err)
	}

	if cfg.Server.ReadTimeout.Int() != 30 {
		t.Fatalf("ReadTimeout: got %d, want 30", cfg.Server.ReadTimeout.Int())
	}
	if cfg.Server.WriteTimeout.Int() != 30 {
		t.Fatalf("WriteTimeout: got %d, want 30", cfg.Server.WriteTimeout.Int())
	}
	if cfg.Auth.TokenDuration.Int() != 24 {
		t.Fatalf("TokenDuration: got %d, want 24", cfg.Auth.TokenDuration.Int())
	}
	if cfg.Cors.MaxAge.Int() != 3600 {
		t.Fatalf("MaxAge: got %d, want 3600", cfg.Cors.MaxAge.Int())
	}
	if cfg.Chat.Uploads.MaxBytes.Int64() != 10485760 {
		t.Fatalf("MaxBytes: got %d, want 10485760", cfg.Chat.Uploads.MaxBytes.Int64())
	}
	if cfg.Chat.Uploads.InlineMaxBytes.Int64() != 512000 {
		t.Fatalf("InlineMaxBytes: got %d, want 512000", cfg.Chat.Uploads.InlineMaxBytes.Int64())
	}
}

// TestParse_MixedNumerics confirms mixing quoted and unquoted works.
func TestParse_MixedNumerics(t *testing.T) {
	y := `
server:
  readTimeout: "30"
  writeTimeout: 30
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(y), &cfg); err != nil {
		t.Fatalf("failed to parse mixed numerics: %v", err)
	}

	if cfg.Server.ReadTimeout.Int() != 30 {
		t.Fatalf("ReadTimeout: got %d, want 30", cfg.Server.ReadTimeout.Int())
	}
	if cfg.Server.WriteTimeout.Int() != 30 {
		t.Fatalf("WriteTimeout: got %d, want 30", cfg.Server.WriteTimeout.Int())
	}
}

// TestParse_EmptyStringNumerics verifies empty strings resolve to 0.
func TestParse_EmptyStringNumerics(t *testing.T) {
	y := `
server:
  readTimeout: ""
  writeTimeout: ""
`
	var cfg Config
	if err := yaml.Unmarshal([]byte(y), &cfg); err != nil {
		t.Fatalf("failed to parse empty string numerics: %v", err)
	}

	if cfg.Server.ReadTimeout.Int() != 0 {
		t.Fatalf("ReadTimeout: got %d, want 0", cfg.Server.ReadTimeout.Int())
	}
	if cfg.Server.WriteTimeout.Int() != 0 {
		t.Fatalf("WriteTimeout: got %d, want 0", cfg.Server.WriteTimeout.Int())
	}
}

// TestParse_MissingNumerics confirms omitted fields stay 0.
func TestParse_MissingNumerics(t *testing.T) {
	y := `server: {}`
	var cfg Config
	if err := yaml.Unmarshal([]byte(y), &cfg); err != nil {
		t.Fatalf("failed to parse empty server: %v", err)
	}
	if cfg.Server.ReadTimeout.Int() != 0 {
		t.Fatalf("ReadTimeout: got %d, want 0", cfg.Server.ReadTimeout.Int())
	}
}

func TestLoad_NewEnvOverrides(t *testing.T) {
	ResetLoadForTest()
	t.Cleanup(ResetLoadForTest)

	dir := t.TempDir()
	path := dir + "/config.yaml"
	yamlBody := `
server:
  port: "8090"
  maxParticipantsLimit: 100
  maxRoomsPerUser: 10
livekit:
  host: "ws://localhost:7880"
  external: false
  skipTLSVerify: false
auth:
  jwtSecret: "test-jwt-secret-at-least-32-chars!!"
  sessionSecret: "test-session-secret-32chars-long!"
  verificationTokenTTLHours: 24
  unverifiedAccountTTLHours: 48
database:
  type: "sqlite"
  path: ":memory:"
`
	if err := os.WriteFile(path, []byte(yamlBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("SERVER_MAX_PARTICIPANTS_LIMIT", "250")
	t.Setenv("AUTH_VERIFICATION_TOKEN_TTL_HOURS", "12")
	t.Setenv("AUTH_UNVERIFIED_ACCOUNT_TTL_HOURS", "72")
	t.Setenv("LIVEKIT_EXTERNAL", "true")
	t.Setenv("LIVEKIT_SKIP_TLS_VERIFY", "true")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.MaxParticipantsLimit != 250 {
		t.Fatalf("MaxParticipantsLimit: got %d, want 250", cfg.Server.MaxParticipantsLimit)
	}
	if cfg.Auth.VerificationTokenTTLHours != 12 {
		t.Fatalf("VerificationTokenTTLHours: got %d, want 12", cfg.Auth.VerificationTokenTTLHours)
	}
	if cfg.Auth.UnverifiedAccountTTLHours != 72 {
		t.Fatalf("UnverifiedAccountTTLHours: got %d, want 72", cfg.Auth.UnverifiedAccountTTLHours)
	}
	if !cfg.LiveKit.External {
		t.Fatal("expected LiveKit.External true")
	}
	if !cfg.LiveKit.SkipTLSVerify {
		t.Fatal("expected LiveKit.SkipTLSVerify true")
	}
}

func TestLoad_NewEnvOverrides_InvalidIgnored(t *testing.T) {
	ResetLoadForTest()
	t.Cleanup(ResetLoadForTest)

	dir := t.TempDir()
	path := dir + "/config.yaml"
	yamlBody := `
server:
  port: "8090"
  maxParticipantsLimit: 100
livekit:
  host: "ws://localhost:7880"
  external: true
  skipTLSVerify: true
auth:
  jwtSecret: "test-jwt-secret-at-least-32-chars!!"
  sessionSecret: "test-session-secret-32chars-long!"
  verificationTokenTTLHours: 24
  unverifiedAccountTTLHours: 48
database:
  type: "sqlite"
  path: ":memory:"
`
	if err := os.WriteFile(path, []byte(yamlBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("SERVER_MAX_PARTICIPANTS_LIMIT", "not-a-number")
	t.Setenv("AUTH_VERIFICATION_TOKEN_TTL_HOURS", "xx")
	t.Setenv("AUTH_UNVERIFIED_ACCOUNT_TTL_HOURS", "yy")
	t.Setenv("LIVEKIT_EXTERNAL", "notbool")
	t.Setenv("LIVEKIT_SKIP_TLS_VERIFY", "maybe")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Server.MaxParticipantsLimit != 100 {
		t.Fatalf("MaxParticipantsLimit should stay YAML 100, got %d", cfg.Server.MaxParticipantsLimit)
	}
	if cfg.Auth.VerificationTokenTTLHours != 24 {
		t.Fatalf("VerificationTokenTTLHours should stay 24, got %d", cfg.Auth.VerificationTokenTTLHours)
	}
	if cfg.Auth.UnverifiedAccountTTLHours != 48 {
		t.Fatalf("UnverifiedAccountTTLHours should stay 48, got %d", cfg.Auth.UnverifiedAccountTTLHours)
	}
	if !cfg.LiveKit.External {
		t.Fatal("LiveKit.External should stay YAML true")
	}
	if !cfg.LiveKit.SkipTLSVerify {
		t.Fatal("LiveKit.SkipTLSVerify should stay YAML true")
	}
}

func TestLoad_LiveKitExternal_FalseOverride(t *testing.T) {
	ResetLoadForTest()
	t.Cleanup(ResetLoadForTest)

	dir := t.TempDir()
	path := dir + "/config.yaml"
	yamlBody := `
server:
  port: "8090"
livekit:
  host: "ws://localhost:7880"
  external: true
  skipTLSVerify: true
auth:
  jwtSecret: "test-jwt-secret-at-least-32-chars!!"
  sessionSecret: "test-session-secret-32chars-long!"
database:
  type: "sqlite"
  path: ":memory:"
`
	if err := os.WriteFile(path, []byte(yamlBody), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	t.Setenv("LIVEKIT_EXTERNAL", "false")
	t.Setenv("LIVEKIT_SKIP_TLS_VERIFY", "0")

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.LiveKit.External {
		t.Fatal("expected LiveKit.External false from env")
	}
	if cfg.LiveKit.SkipTLSVerify {
		t.Fatal("expected LiveKit.SkipTLSVerify false from env")
	}
}

func TestLoad_TokenDurationDefault(t *testing.T) {
	cases := []struct {
		name string
		line string
		want int
	}{
		{name: "missing", line: "", want: DefaultTokenDurationHours},
		{name: "zero", line: "  tokenDuration: 0\n", want: DefaultTokenDurationHours},
		{name: "negative", line: "  tokenDuration: -5\n", want: DefaultTokenDurationHours},
		{name: "explicit", line: "  tokenDuration: 48\n", want: 48},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ResetLoadForTest()
			t.Cleanup(ResetLoadForTest)

			path := t.TempDir() + "/config.yaml"
			yamlBody := "auth:\n  jwtSecret: \"test-jwt-secret-at-least-32-chars!!\"\n" + testCase.line
			if err := os.WriteFile(path, []byte(yamlBody), 0o600); err != nil {
				t.Fatalf("write config: %v", err)
			}

			cfg, err := Load(path)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			if got := cfg.Auth.TokenDuration.Int(); got != testCase.want {
				t.Fatalf("TokenDuration: got %d, want %d", got, testCase.want)
			}
		})
	}
}
