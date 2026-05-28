package models

import "time"

type SystemSettings struct {
	ID                    uint `gorm:"primaryKey;autoIncrement" json:"id"`
	RegistrationEnabled   bool `gorm:"not null;default:true" json:"registrationEnabled"`
	TokenRegistrationOnly bool `gorm:"not null;default:false" json:"tokenRegistrationOnly"`

	// Auth
	PasskeysEnabled     bool   `gorm:"not null;default:true" json:"passkeysEnabled"`
	GoogleClientID      string `gorm:"size:512" json:"googleClientId"`
	GoogleClientSecret  string `gorm:"size:512" json:"googleClientSecret"`
	GoogleRedirectURL   string `gorm:"size:512" json:"googleRedirectUrl"`
	GithubClientID      string `gorm:"size:512" json:"githubClientId"`
	GithubClientSecret  string `gorm:"size:512" json:"githubClientSecret"`
	GithubRedirectURL   string `gorm:"size:512" json:"githubRedirectUrl"`
	TwitterClientID     string `gorm:"size:512" json:"twitterClientId"`
	TwitterClientSecret string `gorm:"size:512" json:"twitterClientSecret"`
	TwitterRedirectURL  string `gorm:"size:512" json:"twitterRedirectUrl"`
	JWTSecret           string `gorm:"size:512" json:"jwtSecret"`
	TokenDuration       int    `gorm:"default:24" json:"tokenDuration"`
	SessionSecret       string `gorm:"size:512" json:"sessionSecret"`
	FrontendURL         string `gorm:"size:512" json:"frontendUrl"`

	// Server
	ServerPort      string `gorm:"size:20" json:"serverPort"`
	ServerHost      string `gorm:"size:255" json:"serverHost"`
	ServerDomain    string `gorm:"size:255" json:"serverDomain"`
	ServerEnableTLS bool   `json:"serverEnableTls"`
	ServerCertFile  string `gorm:"size:512" json:"serverCertFile"`
	ServerKeyFile   string `gorm:"size:512" json:"serverKeyFile"`
	ServerUseACME   bool   `json:"serverUseAcme"`
	ServerEmail     string `gorm:"size:255" json:"serverEmail"`
	BehindProxy     bool   `json:"behindProxy"`

	// Instance
	ServerName        string `gorm:"size:255" json:"serverName"`
	GuestLoginEnabled bool   `gorm:"not null;default:true" json:"guestLoginEnabled"`

	// LiveKit
	LiveKitHost      string `gorm:"size:255" json:"livekitHost"`
	LiveKitAPIKey    string `gorm:"size:255" json:"livekitApiKey"`
	LiveKitAPISecret string `gorm:"size:255" json:"livekitApiSecret"`
	LiveKitExternal  bool   `json:"livekitExternal"`

	// CORS
	CORSAllowedOrigins   string `gorm:"size:1024" json:"corsAllowedOrigins"`
	CORSAllowedHeaders   string `gorm:"size:1024" json:"corsAllowedHeaders"`
	CORSAllowedMethods   string `gorm:"size:255" json:"corsAllowedMethods"`
	CORSAllowCredentials bool   `json:"corsAllowCredentials"`
	CORSMaxAge           int    `json:"corsMaxAge"`

	// Chat uploads
	ChatUploadBackend     string `gorm:"size:20" json:"chatUploadBackend"`
	ChatUploadMaxBytes    int64  `json:"chatUploadMaxBytes"`
	ChatUploadInlineMax   int64  `json:"chatUploadInlineMax"`
	ChatUploadDiskDir     string `gorm:"size:512" json:"chatUploadDiskDir"`
	ChatUploadS3Endpoint  string `gorm:"size:255" json:"chatUploadS3Endpoint"`
	ChatUploadS3Bucket    string `gorm:"size:255" json:"chatUploadS3Bucket"`
	ChatUploadS3Region    string `gorm:"size:50" json:"chatUploadS3Region"`
	ChatUploadS3AccessKey string `gorm:"size:255" json:"chatUploadS3AccessKey"`
	ChatUploadS3SecretKey string `gorm:"size:255" json:"chatUploadS3SecretKey"`
	ChatUploadS3PublicURL string `gorm:"size:512" json:"chatUploadS3PublicUrl"`

	// Room limits
	MaxParticipantsLimit int `gorm:"default:1000" json:"maxParticipantsLimit"`
	MaxRoomsPerUser      int `gorm:"default:100" json:"maxRoomsPerUser"`

	// Upload quotas
	MaxUploadBytesPerUser    int64 `gorm:"default:524288000" json:"maxUploadBytesPerUser"`
	GlobalDiskThresholdBytes int64 `gorm:"default:0" json:"globalDiskThresholdBytes"`

	// Chat message retention
	ChatMaxMessageCount int `gorm:"default:10000" json:"chatMaxMessageCount"`
	ChatMessageTTLHours int `gorm:"default:2160" json:"chatMessageTTLHours"`

	// TODO oncoming feature: recordings
	// RecordingsEnabled, RecordingMaxDurationMins, RecordingMaxFileSizeMB
	RecordingsEnabled        bool `gorm:"default:false" json:"recordingsEnabled"`
	RecordingMaxDurationMins int  `gorm:"default:60" json:"recordingMaxDurationMins"` // 0 = unlimited
	RecordingMaxFileSizeMB   int  `gorm:"default:2048" json:"recordingMaxFileSizeMB"` // 0 = unlimited

	// Email branding
	EmailInstanceName string `gorm:"size:255" json:"emailInstanceName"`
	EmailSupportEmail string `gorm:"size:255" json:"emailSupportEmail"`
	EmailInstanceURL  string `gorm:"size:512" json:"emailInstanceUrl"`
	EmailHeaderBg     string `gorm:"size:7" json:"emailHeaderBg"`
	EmailButtonBg     string `gorm:"size:7" json:"emailButtonBg"`

	// Per-template subject line overrides (empty = use config.yaml or hardcoded default)
	EmailSubjectVerify  string `gorm:"size:255" json:"emailSubjectVerify"`
	EmailSubjectWelcome string `gorm:"size:255" json:"emailSubjectWelcome"`
	EmailSubjectReset   string `gorm:"size:255" json:"emailSubjectReset"`
	EmailSubjectChanged string `gorm:"size:255" json:"emailSubjectChanged"`
	EmailSubjectInvite  string `gorm:"size:255" json:"emailSubjectInvite"`

	// Per-template preheader text overrides
	EmailPreheaderVerify  string `gorm:"size:512" json:"emailPreheaderVerify"`
	EmailPreheaderWelcome string `gorm:"size:512" json:"emailPreheaderWelcome"`
	EmailPreheaderReset   string `gorm:"size:512" json:"emailPreheaderReset"`
	EmailPreheaderChanged string `gorm:"size:512" json:"emailPreheaderChanged"`
	EmailPreheaderInvite  string `gorm:"size:512" json:"emailPreheaderInvite"`

	// SMTP settings in DB (empty = fall back to config.yaml)
	EmailSMTPHost      string `gorm:"size:255" json:"emailSmtpHost"`
	EmailSMTPPort      int    `gorm:"default:0" json:"emailSmtpPort"`
	EmailUsername      string `gorm:"size:255" json:"emailUsername"`
	EmailPassword      string `gorm:"size:512" json:"emailPassword"`
	EmailFromAddress   string `gorm:"size:255" json:"emailFromAddress"`
	EmailFromName      string `gorm:"size:255" json:"emailFromName"`
	EmailTLSSkipVerify bool   `json:"emailTlsSkipVerify"`
	EmailSMTPSMode     bool   `json:"emailSmtpsMode"`

	// Logger
	LogLevel string `gorm:"size:20" json:"logLevel"`

	UpdatedAt time.Time `json:"updatedAt"`
}

// IsOAuthProviderConfigured returns true if the given provider has both
// client ID and client secret set.
func (s *SystemSettings) IsOAuthProviderConfigured(provider string) bool {
	switch provider {
	case "google":
		return s.GoogleClientID != "" && s.GoogleClientSecret != ""
	case "github":
		return s.GithubClientID != "" && s.GithubClientSecret != ""
	case "twitter":
		return s.TwitterClientID != "" && s.TwitterClientSecret != ""
	}
	return false
}

// ConfiguredOAuthProviders returns a list of provider IDs that have credentials.
func (s *SystemSettings) ConfiguredOAuthProviders() []string {
	var providers []string
	for _, p := range []string{"google", "github", "twitter"} {
		if s.IsOAuthProviderConfigured(p) {
			providers = append(providers, p)
		}
	}
	return providers
}
