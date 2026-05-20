package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	LiveKit  LiveKitConfig  `yaml:"livekit"`
	Auth     AuthConfig     `yaml:"auth"`
	Logger   LoggerConfig   `yaml:"logger"`
	Cors     CorsConfig     `yaml:"cors"`
	Chat     ChatConfig     `yaml:"chat"`
	// TODO oncoming feature
	Recording RecordingConfig `yaml:"recording"`
	RateLimit RateLimitConfig `yaml:"rateLimit"`
	Queue     QueueConfig     `yaml:"queue"`
	Email     EmailConfig     `yaml:"email"`
}

type ServerConfig struct {
	Port           string    `yaml:"port"`
	HTTPPort       string    `yaml:"httpPort"`
	Host           string    `yaml:"host"`
	ReadTimeout    ConfigInt `yaml:"readTimeout"`
	WriteTimeout   ConfigInt `yaml:"writeTimeout"`
	EnableTLS      bool      `yaml:"enableTLS" env:"SERVER_ENABLE_TLS"`
	DisableTLS     bool      `yaml:"disableTLS"`
	CertFile       string    `yaml:"certFile" env:"SERVER_CERT_FILE"`
	KeyFile        string    `yaml:"keyFile" env:"SERVER_KEY_FILE"`
	Domain         string    `yaml:"domain" env:"SERVER_DOMAIN"`
	Email          string    `yaml:"email" env:"SERVER_EMAIL"`
	UseACME        bool      `yaml:"useACME" env:"SERVER_USE_ACME"`
	TrustedProxies []string  `yaml:"trustedProxies"`
	ProxyHeader    string    `yaml:"proxyHeader"`
	// BehindProxy enables trusted-proxy mode. Set to true when running
	// behind Cloudflare, nginx, or any reverse proxy that terminates TLS.
	BehindProxy bool `yaml:"behindProxy"`
	// CertAlgorithm selects the key algorithm for self-signed certificate generation.
	// Supported: "ed25519" (default), "ecdsa256", "rsa2048", or "rsa4096".
	// Renewal auto-detects and preserves the existing cert's algorithm.
	// Env: SERVER_CERT_ALGORITHM
	CertAlgorithm string `yaml:"certAlgorithm" env:"SERVER_CERT_ALGORITHM"`
	// MaxParticipantsLimit is the hard ceiling for room maxParticipants.
	// 0 means use internal default (1000).
	// Env: SERVER_MAX_PARTICIPANTS_LIMIT
	MaxParticipantsLimit int `yaml:"maxParticipantsLimit" env:"SERVER_MAX_PARTICIPANTS_LIMIT"`
	// MaxRoomsPerUser caps the number of active rooms a single user can create.
	// 0 means unlimited. Default when used as fallback: 100.
	// Env: SERVER_MAX_ROOMS_PER_USER
	MaxRoomsPerUser int `yaml:"maxRoomsPerUser" env:"SERVER_MAX_ROOMS_PER_USER"`
}

type DatabaseConfig struct {
	Host         string `yaml:"host"`
	Port         string `yaml:"port"`
	User         string `yaml:"user"`
	Password     string `yaml:"password"`
	DBName       string `yaml:"dbname"`
	SSLMode      string `yaml:"sslmode"`
	MaxIdleConns int    `yaml:"maxIdleConns"`
	MaxOpenConns int    `yaml:"maxOpenConns"`
	MaxLifetime  int    `yaml:"maxLifetime"` // in minutes
	Type         string `yaml:"type"`        // e.g., "postgres", "sqlite"
	Path         string `yaml:"path"`        // Path for SQLite, if used
}

type LiveKitConfig struct {
	Host          string `yaml:"host"`
	InternalHost  string `yaml:"internalHost"`
	APIKey        string `yaml:"apiKey"`
	APISecret     string `yaml:"apiSecret"`
	ConfigPath    string `yaml:"configPath"`
	SkipTLSVerify bool   `yaml:"skipTLSVerify"`
	// External skips the embedded LiveKit server and /livekit proxy.
	// Set to true when using a separate LiveKit deployment (e.g. lk.bedrud.org).
	External bool `yaml:"external"`
	// NodeIP is the explicit IP address for embedded LiveKit's RTC node_ip.
	// When set, LiveKit uses this IP directly (use_external_ip=false) instead of
	// STUN-based detection. Required for air-gapped, Docker, or firewalled environments.
	// Env: LIVEKIT_NODE_IP
	NodeIP string `yaml:"nodeIP"`
}

type AuthConfig struct {
	JWTSecret           string       `yaml:"jwtSecret"`
	TokenDuration       ConfigInt    `yaml:"tokenDuration"` // in hours
	Google              OAuth2Config `yaml:"google"`
	Github              OAuth2Config `yaml:"github"`
	Twitter             OAuth2Config `yaml:"twitter"`
	FrontendURL         string       `yaml:"frontendURL"`
	SessionSecret       string       `yaml:"sessionSecret"`
	PasskeyChallengeTTL int          `yaml:"passkeyChallengeTTL"` // minutes, default 5. Env: AUTH_PASSKEY_CHALLENGE_TTL
	// RequireEmailVerification gates all local registration + login behind email verification.
	// When true, users must verify their email before they can access the app.
	// Default: false (backward compatible). Env: AUTH_REQUIRE_EMAIL_VERIFICATION
	RequireEmailVerification bool `yaml:"requireEmailVerification" env:"AUTH_REQUIRE_EMAIL_VERIFICATION"`
	// VerificationEmailCooldownMins is the minimum time between verification email resends.
	// Default: 2 minutes. Env: AUTH_VERIFICATION_COOLDOWN_MINS
	VerificationEmailCooldownMins int `yaml:"verificationEmailCooldownMins" env:"AUTH_VERIFICATION_COOLDOWN_MINS"`
	// VerificationTokenTTLHours controls how long verification links remain valid.
	// Default: 24 hours. Env: AUTH_VERIFICATION_TOKEN_TTL_HOURS
	VerificationTokenTTLHours int `yaml:"verificationTokenTTLHours" env:"AUTH_VERIFICATION_TOKEN_TTL_HOURS"`
	// UnverifiedAccountTTLHours controls automatic deletion of local/passkey accounts
	// that registered but never verified their email. 0 = disabled. Default: 48 hours.
	// Env: AUTH_UNVERIFIED_ACCOUNT_TTL_HOURS
	UnverifiedAccountTTLHours int `yaml:"unverifiedAccountTTLHours" env:"AUTH_UNVERIFIED_ACCOUNT_TTL_HOURS"`
	// ResetTokenTTLHours controls how long password reset links remain valid.
	// 0 means use default (1 hour). Env: AUTH_RESET_TOKEN_TTL_HOURS
	ResetTokenTTLHours int `yaml:"resetTokenTTLHours" env:"AUTH_RESET_TOKEN_TTL_HOURS"`
}

type OAuth2Config struct {
	ClientID     string `yaml:"clientId"`
	ClientSecret string `yaml:"clientSecret"`
	RedirectURL  string `yaml:"redirectUrl"`
}

type LoggerConfig struct {
	Level      string `yaml:"level"`
	OutputPath string `yaml:"outputPath"`
}

type CorsConfig struct {
	AllowedOrigins   string    `yaml:"allowedOrigins"`
	AllowedHeaders   string    `yaml:"allowedHeaders"`
	AllowedMethods   string    `yaml:"allowedMethods"`
	AllowCredentials bool      `yaml:"allowCredentials"`
	ExposeHeaders    string    `yaml:"exposeHeaders"`
	MaxAge           ConfigInt `yaml:"maxAge"`
}

// ChatConfig holds settings for in-room chat, including image upload storage and quotas.
type ChatConfig struct {
	Uploads ChatUploadConfig `yaml:"uploads"`
	// MaxUploadBytesPerUser caps total bytes a user can store via chat uploads.
	// 0 means unlimited. Default when used as fallback: 524288000 (500 MB).
	// Env: CHAT_MAX_UPLOAD_BYTES_PER_USER
	MaxUploadBytesPerUser int64 `yaml:"maxUploadBytesPerUser" env:"CHAT_MAX_UPLOAD_BYTES_PER_USER"`
	// GlobalDiskThresholdBytes is the total upload storage ceiling across all users.
	// When exceeded, all uploads are rejected until an admin frees space.
	// 0 means unlimited. Env: CHAT_GLOBAL_DISK_THRESHOLD_BYTES
	GlobalDiskThresholdBytes int64 `yaml:"globalDiskThresholdBytes" env:"CHAT_GLOBAL_DISK_THRESHOLD_BYTES"`
	// MaxMessageCount caps the number of chat messages kept per room.
	// 0 means unlimited. Default: 10000. Env: CHAT_MAX_MESSAGE_COUNT
	MaxMessageCount int `yaml:"maxMessageCount" env:"CHAT_MAX_MESSAGE_COUNT"`
	// MessageTTLHours is the max age of a chat message in hours.
	// Messages older than this are purged. 0 means forever. Default: 2160 (90 days).
	// Env: CHAT_MESSAGE_TTL_HOURS
	MessageTTLHours int `yaml:"messageTTLHours" env:"CHAT_MESSAGE_TTL_HOURS"`
}

// ChatUploadConfig controls where and how chat image uploads are stored.
// Backend choices: "disk" (default) stores files under DiskDir;
// "s3" uploads to an S3-compatible bucket (requires S3 fields);
// "inline" always returns base64 data URLs regardless of InlineMaxBytes.
type ChatUploadConfig struct {
	// Backend is "disk" (default), "s3", or "inline".
	Backend string `yaml:"backend"`
	// MaxBytes is the hard upload size limit (default 10 MB).
	MaxBytes ConfigInt `yaml:"maxBytes"`
	// InlineMaxBytes: images smaller than this are returned as data: URIs instead
	// of stored on disk / S3. Set to 0 to disable inline encoding. Default 512000 (500 KB).
	InlineMaxBytes ConfigInt `yaml:"inlineMaxBytes"`
	// DiskDir is the directory for disk-backend uploads. Default: ./data/uploads/chat
	DiskDir string `yaml:"diskDir"`
	// S3 holds connection info for the S3-compatible storage backend.
	S3 ChatUploadS3Config `yaml:"s3"`
}

type ChatUploadS3Config struct {
	Endpoint      string `yaml:"endpoint"`
	Bucket        string `yaml:"bucket"`
	Region        string `yaml:"region"`
	AccessKey     string `yaml:"accessKey"`
	SecretKey     string `yaml:"secretKey"`
	PublicBaseURL string `yaml:"publicBaseUrl"`
}

// TODO oncoming feature
// RecordingConfig controls recording storage.
type RecordingConfig struct {
	// MaxFileSizeMB caps each recording file. 0 = unlimited. Default 2048.
	MaxFileSizeMB int `yaml:"maxFileSizeMB"`
	// StorageDir is the directory for disk-backed recordings. Default: ./data/recordings
	StorageDir string `yaml:"storageDir"`
	// MaxRecordingsPerRoom caps total recordings per room. 0 = unlimited.
	// Applied to non-persistent rooms to prevent recording spam.
	MaxRecordingsPerRoom int `yaml:"maxRecordingsPerRoom"`
	// RetentionHours controls how long completed recordings are kept after room archive.
	// 0 = keep forever. Default: 720 (30 days). Env: RECORDING_RETENTION_HOURS
	RetentionHours int `yaml:"retentionHours" env:"RECORDING_RETENTION_HOURS"`
	// CleanupIntervalHours controls how often the scheduler checks for expired recordings.
	// 0 = disabled. Default: 24 (daily). Env: RECORDING_CLEANUP_INTERVAL_HOURS
	CleanupIntervalHours int `yaml:"cleanupIntervalHours" env:"RECORDING_CLEANUP_INTERVAL_HOURS"`
}

// QueueConfig controls the internal job queue worker.
type QueueConfig struct {
	PollInterval ConfigInt `yaml:"pollInterval"` // ms between polls, default 500. Env: QUEUE_POLL_INTERVAL
	MaxAttempts  int       `yaml:"maxAttempts"`  // max retries before failed, default 3. Env: QUEUE_MAX_ATTEMPTS
	Concurrency  int       `yaml:"concurrency"`  // worker goroutines, default 1. Env: QUEUE_CONCURRENCY
}

// EmailConfig controls SMTP settings and email template configuration.
type EmailConfig struct {
	SMTPHost      string              `yaml:"smtpHost"`      // Env: EMAIL_SMTP_HOST
	SMTPPort      int                 `yaml:"smtpPort"`      // Env: EMAIL_SMTP_PORT
	Username      string              `yaml:"username"`      // Env: EMAIL_USERNAME
	Password      string              `yaml:"password"`      // Env: EMAIL_PASSWORD
	FromAddress   string              `yaml:"fromAddress"`   // Env: EMAIL_FROM_ADDRESS
	FromName      string              `yaml:"fromName"`      // Env: EMAIL_FROM_NAME
	TLSSkipVerify bool                `yaml:"tlsSkipVerify"` // Skip TLS cert validation. Env: EMAIL_TLS_SKIP_VERIFY
	SMTPSMode     bool                `yaml:"smtpsMode"`     // Direct TLS (SMTPS, port 465). Env: EMAIL_SMTPS_MODE
	Templates     EmailTemplateConfig `yaml:"templates"`     // Email branding and per-template overrides
}

// EmailTemplateConfig controls email branding and per-template text overrides.
// Values can be overridden per-instance via the admin panel SystemSettings.
type EmailTemplateConfig struct {
	InstanceName  string            `yaml:"instanceName"` // Default "Bedrud"
	SupportEmail  string            `yaml:"supportEmail"`
	InstanceURL   string            `yaml:"instanceUrl"`
	HeaderBgColor string            `yaml:"headerBgColor"` // Hex color, default "#1a1a2e"
	ButtonBgColor string            `yaml:"buttonBgColor"` // Hex color, default "#e11d48"
	SubjectLines  map[string]string `yaml:"subjectLines"`  // Per-template subject override
	PreheaderText map[string]string `yaml:"preheaderText"` // Per-template preheader text
}

// RateLimitConfig controls rate limiting for auth and guest endpoints.
// Nil fields = use defaults. Set to 0 to disable.
type RateLimitConfig struct {
	AuthMaxRequests       *int `yaml:"authMaxRequests"`
	AuthWindowSecs        *int `yaml:"authWindowSecs"`
	AuthResendMaxRequests *int `yaml:"authResendMaxRequests"` // separate limit for verification resend. Env: RATELIMIT_AUTH_RESEND_MAX
	AuthResendWindowSecs  *int `yaml:"authResendWindowSecs"`  // Env: RATELIMIT_AUTH_RESEND_WINDOW
	GuestMaxRequests      *int `yaml:"guestMaxRequests"`
	GuestWindowSecs       *int `yaml:"guestWindowSecs"`
	APIMaxRequests        *int `yaml:"apiMaxRequests"`
	APIWindowSecs         *int `yaml:"apiWindowSecs"`
}

// ConfigInt accepts YAML int or quoted-string numeric values.
// Logs warning when string path triggered to encourage migration to bare int.
type ConfigInt int64

func (f *ConfigInt) UnmarshalYAML(unmarshal func(any) error) error {
	var i int64
	if err := unmarshal(&i); err == nil {
		*f = ConfigInt(i)
		return nil
	}

	var s string
	if err := unmarshal(&s); err != nil {
		return err
	}
	if s == "" {
		*f = 0
		return nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return err
	}
	log.Warn().Str("value", s).Msg("config field uses quoted string, use bare integer instead")
	*f = ConfigInt(n)
	return nil
}

func (f ConfigInt) MarshalYAML() (any, error) { return int64(f), nil }
func (f ConfigInt) Int() int                  { return int(f) }
func (f ConfigInt) Int64() int64              { return int64(f) }

var (
	config *Config
	once   sync.Once
)

// Load reads the configuration file and returns a Config struct
func Load(configPath string) (*Config, error) {
	var loadErr error
	once.Do(func() {
		config = &Config{}

		data, err := os.ReadFile(configPath)
		if err != nil {
			if os.IsNotExist(err) {
				loadErr = fmt.Errorf(
					"configuration file not found: %s\n\n"+
						"Create one by copying the example:\n"+
						"  cp config.local.yaml.example config.yaml\n"+
						"Or specify a custom path:\n"+
						"  CONFIG_PATH=/path/to/config.yaml bedrud run",
					configPath,
				)
			} else {
				loadErr = fmt.Errorf("failed to read configuration file %s: %w", configPath, err)
			}
			return
		}

		err = yaml.Unmarshal(data, config)
		if err != nil {
			loadErr = fmt.Errorf("invalid configuration in %s: %w", configPath, err)
			return
		}

		// Override with environment variables if they exist
		if envPort := os.Getenv("SERVER_PORT"); envPort != "" {
			config.Server.Port = envPort
		}
		if envHTTPPort := os.Getenv("SERVER_HTTP_PORT"); envHTTPPort != "" {
			config.Server.HTTPPort = envHTTPPort
		}
		if envEnableTLS := os.Getenv("SERVER_ENABLE_TLS"); envEnableTLS != "" {
			if b, err := strconv.ParseBool(envEnableTLS); err == nil {
				config.Server.EnableTLS = b
			}
		}
		if envCertFile := os.Getenv("SERVER_CERT_FILE"); envCertFile != "" {
			config.Server.CertFile = envCertFile
		}
		if envKeyFile := os.Getenv("SERVER_KEY_FILE"); envKeyFile != "" {
			config.Server.KeyFile = envKeyFile
		}
		if envDomain := os.Getenv("SERVER_DOMAIN"); envDomain != "" {
			config.Server.Domain = envDomain
		}
		if envEmail := os.Getenv("SERVER_EMAIL"); envEmail != "" {
			config.Server.Email = envEmail
		}
		if envUseACME := os.Getenv("SERVER_USE_ACME"); envUseACME != "" {
			if b, err := strconv.ParseBool(envUseACME); err == nil {
				config.Server.UseACME = b
			}
		}
		if envTrustedProxies := os.Getenv("SERVER_TRUSTED_PROXIES"); envTrustedProxies != "" {
			config.Server.TrustedProxies = strings.Split(envTrustedProxies, ",")
		}
		if envProxyHeader := os.Getenv("SERVER_PROXY_HEADER"); envProxyHeader != "" {
			config.Server.ProxyHeader = envProxyHeader
		}
		if envCertAlgorithm := os.Getenv("SERVER_CERT_ALGORITHM"); envCertAlgorithm != "" {
			config.Server.CertAlgorithm = envCertAlgorithm
		}
		if envMaxRoomsPerUser := os.Getenv("SERVER_MAX_ROOMS_PER_USER"); envMaxRoomsPerUser != "" {
			if i, err := strconv.Atoi(envMaxRoomsPerUser); err == nil {
				config.Server.MaxRoomsPerUser = i
			}
		}
		if dbHost := os.Getenv("DB_HOST"); dbHost != "" {
			config.Database.Host = dbHost
		}
		if dbPort := os.Getenv("DB_PORT"); dbPort != "" {
			config.Database.Port = dbPort
		}
		if dbUser := os.Getenv("DB_USER"); dbUser != "" {
			config.Database.User = dbUser
		}
		if dbPass := os.Getenv("DB_PASSWORD"); dbPass != "" {
			config.Database.Password = dbPass
		}
		if dbName := os.Getenv("DB_NAME"); dbName != "" {
			config.Database.DBName = dbName
		}
		if dbType := os.Getenv("DB_TYPE"); dbType != "" {
			config.Database.Type = dbType
		}
		if dbPath := os.Getenv("DB_PATH"); dbPath != "" {
			config.Database.Path = dbPath
		}
		if livekitHost := os.Getenv("LIVEKIT_HOST"); livekitHost != "" {
			config.LiveKit.Host = livekitHost
		}
		if livekitInternalHost := os.Getenv("LIVEKIT_INTERNAL_HOST"); livekitInternalHost != "" {
			config.LiveKit.InternalHost = livekitInternalHost
		}
		if livekitApiKey := os.Getenv("LIVEKIT_API_KEY"); livekitApiKey != "" {
			config.LiveKit.APIKey = livekitApiKey
		}
		if livekitApiSecret := os.Getenv("LIVEKIT_API_SECRET"); livekitApiSecret != "" {
			config.LiveKit.APISecret = livekitApiSecret
		}
		if livekitConfigPath := os.Getenv("LIVEKIT_CONFIG_PATH"); livekitConfigPath != "" {
			config.LiveKit.ConfigPath = livekitConfigPath
		}
		if livekitNodeIP := os.Getenv("LIVEKIT_NODE_IP"); livekitNodeIP != "" {
			config.LiveKit.NodeIP = livekitNodeIP
		}
		if jwtSecret := os.Getenv("JWT_SECRET"); jwtSecret != "" {
			config.Auth.JWTSecret = jwtSecret
		}
		if frontendURL := os.Getenv("AUTH_FRONTEND_URL"); frontendURL != "" {
			config.Auth.FrontendURL = frontendURL
		}
		if ttl := os.Getenv("AUTH_PASSKEY_CHALLENGE_TTL"); ttl != "" {
			if n, err := strconv.Atoi(ttl); err == nil {
				config.Auth.PasskeyChallengeTTL = n
			}
		}
		if v := os.Getenv("AUTH_REQUIRE_EMAIL_VERIFICATION"); v == "true" || v == "1" {
			config.Auth.RequireEmailVerification = true
		}
		if v := os.Getenv("AUTH_VERIFICATION_COOLDOWN_MINS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				config.Auth.VerificationEmailCooldownMins = n
			}
		}
		if v := os.Getenv("AUTH_RESET_TOKEN_TTL_HOURS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil {
				config.Auth.ResetTokenTTLHours = n
			}
		}

		// CORS environment variable overrides
		if corsAllowedOrigins := os.Getenv("CORS_ALLOWED_ORIGINS"); corsAllowedOrigins != "" {
			config.Cors.AllowedOrigins = corsAllowedOrigins
		}
		if corsAllowedHeaders := os.Getenv("CORS_ALLOWED_HEADERS"); corsAllowedHeaders != "" {
			config.Cors.AllowedHeaders = corsAllowedHeaders
		}
		if corsAllowedMethods := os.Getenv("CORS_ALLOWED_METHODS"); corsAllowedMethods != "" {
			config.Cors.AllowedMethods = corsAllowedMethods
		}
		if corsAllowCredentials := os.Getenv("CORS_ALLOW_CREDENTIALS"); corsAllowCredentials != "" {
			if b, err := strconv.ParseBool(corsAllowCredentials); err == nil {
				config.Cors.AllowCredentials = b
			}
		}
		if corsExposeHeaders := os.Getenv("CORS_EXPOSE_HEADERS"); corsExposeHeaders != "" {
			config.Cors.ExposeHeaders = corsExposeHeaders
		}
		if corsMaxAge := os.Getenv("CORS_MAX_AGE"); corsMaxAge != "" {
			if i, err := strconv.Atoi(corsMaxAge); err == nil {
				config.Cors.MaxAge = ConfigInt(i)
			}
		}

		// Chat upload quota environment variable overrides
		if v := os.Getenv("CHAT_MAX_UPLOAD_BYTES_PER_USER"); v != "" {
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				config.Chat.MaxUploadBytesPerUser = i
			}
		}
		if v := os.Getenv("CHAT_GLOBAL_DISK_THRESHOLD_BYTES"); v != "" {
			if i, err := strconv.ParseInt(v, 10, 64); err == nil {
				config.Chat.GlobalDiskThresholdBytes = i
			}
		}
		if v := os.Getenv("CHAT_MAX_MESSAGE_COUNT"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				config.Chat.MaxMessageCount = i
			}
		}
		if v := os.Getenv("CHAT_MESSAGE_TTL_HOURS"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				config.Chat.MessageTTLHours = i
			}
		}

		// Rate limit environment variable overrides
		if v := os.Getenv("RATELIMIT_AUTH_MAX"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				config.RateLimit.AuthMaxRequests = &i
			}
		}
		if v := os.Getenv("RATELIMIT_AUTH_WINDOW"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				config.RateLimit.AuthWindowSecs = &i
			}
		}
		if v := os.Getenv("RATELIMIT_GUEST_MAX"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				config.RateLimit.GuestMaxRequests = &i
			}
		}
		if v := os.Getenv("RATELIMIT_GUEST_WINDOW"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				config.RateLimit.GuestWindowSecs = &i
			}
		}
		if v := os.Getenv("RATELIMIT_AUTH_RESEND_MAX"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				config.RateLimit.AuthResendMaxRequests = &i
			}
		}
		if v := os.Getenv("RATELIMIT_AUTH_RESEND_WINDOW"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				config.RateLimit.AuthResendWindowSecs = &i
			}
		}

		// API rate limit environment variable overrides
		if v := os.Getenv("RATELIMIT_API_MAX"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				config.RateLimit.APIMaxRequests = &i
			}
		}
		if v := os.Getenv("RATELIMIT_API_WINDOW"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				config.RateLimit.APIWindowSecs = &i
			}
		}

		// Queue environment variable overrides
		if v := os.Getenv("QUEUE_POLL_INTERVAL"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				config.Queue.PollInterval = ConfigInt(i)
			}
		}
		if v := os.Getenv("QUEUE_MAX_ATTEMPTS"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				config.Queue.MaxAttempts = i
			}
		}
		if v := os.Getenv("QUEUE_CONCURRENCY"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				config.Queue.Concurrency = i
			}
		}

		// Email environment variable overrides
		if v := os.Getenv("EMAIL_SMTP_HOST"); v != "" {
			config.Email.SMTPHost = v
		}
		if v := os.Getenv("EMAIL_SMTP_PORT"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				config.Email.SMTPPort = i
			}
		}
		if v := os.Getenv("EMAIL_USERNAME"); v != "" {
			config.Email.Username = v
		}
		if v := os.Getenv("EMAIL_PASSWORD"); v != "" {
			config.Email.Password = v
		}
		if v := os.Getenv("EMAIL_FROM_ADDRESS"); v != "" {
			config.Email.FromAddress = v
		}
		if v := os.Getenv("EMAIL_FROM_NAME"); v != "" {
			config.Email.FromName = v
		}
		if v := os.Getenv("EMAIL_TLS_SKIP_VERIFY"); v == "true" || v == "1" {
			config.Email.TLSSkipVerify = true
		}
		if v := os.Getenv("EMAIL_SMTPS_MODE"); v == "true" || v == "1" {
			config.Email.SMTPSMode = true
		}

		// Recording retention environment variable overrides
		if v := os.Getenv("RECORDING_RETENTION_HOURS"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				config.Recording.RetentionHours = i
			}
		}
		if v := os.Getenv("RECORDING_CLEANUP_INTERVAL_HOURS"); v != "" {
			if i, err := strconv.Atoi(v); err == nil {
				config.Recording.CleanupIntervalHours = i
			}
		}
	})

	return config, loadErr
}

// Get returns the loaded configuration.
// Panics if config not loaded. Use GetSafe() for cases where config may not be initialized.
func Get() *Config {
	if config == nil {
		panic("Config not loaded")
	}
	return config
}

// GetSafe returns the loaded configuration or nil if Load() hasn't been called.
func GetSafe() *Config {
	return config
}

// SetForTest sets the global config for testing purposes only.
// This bypasses the sync.Once in Load and should only be used in tests.
func SetForTest(cfg *Config) {
	config = cfg
}

// GetDSN returns the PostgreSQL connection string
func (c *DatabaseConfig) GetDSN() string {
	return "postgresql://" + c.User + ":" + c.Password + "@" + c.Host + ":" + c.Port + "/" + c.DBName + "?sslmode=" + c.SSLMode
}
