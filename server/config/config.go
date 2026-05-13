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
	Server    ServerConfig    `yaml:"server"`
	Database  DatabaseConfig  `yaml:"database"`
	LiveKit   LiveKitConfig   `yaml:"livekit"`
	Auth      AuthConfig      `yaml:"auth"`
	Logger    LoggerConfig    `yaml:"logger"`
	Cors      CorsConfig      `yaml:"cors"`
	Chat      ChatConfig      `yaml:"chat"`
	RateLimit RateLimitConfig `yaml:"rateLimit"`
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
	// Supported: "ed25519" (default), "ecdsa256", "rsa2048", "rsa4096".
	// Renewal auto-detects and preserves the existing cert's algorithm.
	// Env: SERVER_CERT_ALGORITHM
	CertAlgorithm string `yaml:"certAlgorithm" env:"SERVER_CERT_ALGORITHM"`
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

// ChatConfig holds settings for in-room chat, including image upload storage.
type ChatConfig struct {
	Uploads ChatUploadConfig `yaml:"uploads"`
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

// RateLimitConfig controls rate limiting for auth and guest endpoints.
// Nil fields = use defaults. Set to 0 to disable.
type RateLimitConfig struct {
	AuthMaxRequests  *int `yaml:"authMaxRequests"`
	AuthWindowSecs   *int `yaml:"authWindowSecs"`
	GuestMaxRequests *int `yaml:"guestMaxRequests"`
	GuestWindowSecs  *int `yaml:"guestWindowSecs"`
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
	})

	return config, loadErr
}

// Get returns the loaded configuration
func Get() *Config {
	if config == nil {
		panic("Config not loaded")
	}
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
