package repository

import (
	"bedrud/config"
	"bedrud/internal/models"

	"gorm.io/gorm"
)

type SettingsRepository struct {
	db  *gorm.DB
	cfg *config.Config
}

func NewSettingsRepository(db *gorm.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

// SetConfig stores the loaded config for fallback merge.
func (r *SettingsRepository) SetConfig(cfg *config.Config) {
	r.cfg = cfg
}

func (r *SettingsRepository) GetSettings() (*models.SystemSettings, error) {
	var s models.SystemSettings
	err := r.db.Attrs(models.SystemSettings{RegistrationEnabled: true, GuestLoginEnabled: true, PasskeysEnabled: true, TokenDuration: 24}).FirstOrCreate(&s, models.SystemSettings{ID: 1}).Error
	return &s, err
}

func (r *SettingsRepository) SaveSettings(s *models.SystemSettings) error {
	s.ID = 1
	return r.db.Save(s).Error
}

// GetEffectiveSettings returns settings with DB values overlaid on config.yaml defaults.
// For each field: if the DB value is non-empty/non-zero, it wins; otherwise config.yaml is used.
func (r *SettingsRepository) GetEffectiveSettings() (*models.SystemSettings, error) {
	s, err := r.GetSettings()
	if err != nil {
		return nil, err
	}
	if r.cfg == nil {
		return s, nil
	}
	mergeFromConfig(s, r.cfg)
	return s, nil
}

// mergeFromConfig fills in zero/empty fields from the config file.
// NOTE: Boolean fields from config can override explicit DB false values (e.g.
// setting ServerEnableTLS=false in admin settings then config's enableTls:true wins).
// This is a known limitation — a full design change would track which fields
// have been explicitly set via admin API vs loaded from config.yaml.
func mergeFromConfig(s *models.SystemSettings, cfg *config.Config) {
	// Auth
	if s.GoogleClientID == "" && cfg.Auth.Google.ClientID != "" {
		s.GoogleClientID = cfg.Auth.Google.ClientID
	}
	if s.GoogleClientSecret == "" && cfg.Auth.Google.ClientSecret != "" {
		s.GoogleClientSecret = cfg.Auth.Google.ClientSecret
	}
	if s.GoogleRedirectURL == "" && cfg.Auth.Google.RedirectURL != "" {
		s.GoogleRedirectURL = cfg.Auth.Google.RedirectURL
	}
	if s.GithubClientID == "" && cfg.Auth.Github.ClientID != "" {
		s.GithubClientID = cfg.Auth.Github.ClientID
	}
	if s.GithubClientSecret == "" && cfg.Auth.Github.ClientSecret != "" {
		s.GithubClientSecret = cfg.Auth.Github.ClientSecret
	}
	if s.GithubRedirectURL == "" && cfg.Auth.Github.RedirectURL != "" {
		s.GithubRedirectURL = cfg.Auth.Github.RedirectURL
	}
	if s.TwitterClientID == "" && cfg.Auth.Twitter.ClientID != "" {
		s.TwitterClientID = cfg.Auth.Twitter.ClientID
	}
	if s.TwitterClientSecret == "" && cfg.Auth.Twitter.ClientSecret != "" {
		s.TwitterClientSecret = cfg.Auth.Twitter.ClientSecret
	}
	if s.TwitterRedirectURL == "" && cfg.Auth.Twitter.RedirectURL != "" {
		s.TwitterRedirectURL = cfg.Auth.Twitter.RedirectURL
	}
	if s.JWTSecret == "" {
		s.JWTSecret = cfg.Auth.JWTSecret
	}
	if s.TokenDuration == 0 && cfg.Auth.TokenDuration != 0 {
		s.TokenDuration = cfg.Auth.TokenDuration.Int()
	}
	if s.SessionSecret == "" {
		s.SessionSecret = cfg.Auth.SessionSecret
	}
	if s.FrontendURL == "" {
		s.FrontendURL = cfg.Auth.FrontendURL
	}

	// Server
	if s.ServerPort == "" {
		s.ServerPort = cfg.Server.Port
	}
	if s.ServerHost == "" {
		s.ServerHost = cfg.Server.Host
	}
	if s.ServerDomain == "" {
		s.ServerDomain = cfg.Server.Domain
	}
	if !s.ServerEnableTLS && cfg.Server.EnableTLS {
		s.ServerEnableTLS = cfg.Server.EnableTLS
	}
	if s.ServerCertFile == "" {
		s.ServerCertFile = cfg.Server.CertFile
	}
	if s.ServerKeyFile == "" {
		s.ServerKeyFile = cfg.Server.KeyFile
	}
	if !s.ServerUseACME && cfg.Server.UseACME {
		s.ServerUseACME = cfg.Server.UseACME
	}
	if s.ServerEmail == "" {
		s.ServerEmail = cfg.Server.Email
	}
	if !s.BehindProxy && cfg.Server.BehindProxy {
		s.BehindProxy = cfg.Server.BehindProxy
	}

	// WebXDC gallery (DB non-empty/non-false wins; config fills defaults)
	if s.WebxdcGalleryRemoteCatalogURL == "" && cfg.Webxdc.Gallery.RemoteCatalogURL != "" {
		s.WebxdcGalleryRemoteCatalogURL = cfg.Webxdc.Gallery.RemoteCatalogURL
	}
	if s.WebxdcGallerySource == "" {
		if cfg.Webxdc.Gallery.Source != "" {
			s.WebxdcGallerySource = cfg.Webxdc.Gallery.Source
		} else {
			s.WebxdcGallerySource = "local"
		}
	}
	// Note: booleans — if admin never saved gallery flags, seed from config when config enables gallery
	if !s.WebxdcGalleryEnabled && cfg.Webxdc.Gallery.Enabled {
		s.WebxdcGalleryEnabled = true
	}
	if !s.WebxdcGalleryAllowRemoteDownload && cfg.Webxdc.Gallery.AllowRemoteDownload {
		s.WebxdcGalleryAllowRemoteDownload = true
	}
	// Size limits: 0 in DB means "use config.yaml". Surface config as MiB for the admin UI.
	const mib = int64(1 << 20)
	if s.WebxdcMaxArchiveMB <= 0 && cfg.Webxdc.MaxArchiveBytes > 0 {
		s.WebxdcMaxArchiveMB = int(cfg.Webxdc.MaxArchiveBytes / mib)
		if s.WebxdcMaxArchiveMB <= 0 {
			s.WebxdcMaxArchiveMB = 10
		}
	}
	if s.WebxdcMaxUncompressedMB <= 0 && cfg.Webxdc.MaxUncompressedTotal > 0 {
		s.WebxdcMaxUncompressedMB = int(cfg.Webxdc.MaxUncompressedTotal / mib)
		if s.WebxdcMaxUncompressedMB <= 0 {
			s.WebxdcMaxUncompressedMB = 30
		}
	}
	if s.WebxdcMaxSingleFileMB <= 0 && cfg.Webxdc.MaxSingleFileBytes > 0 {
		s.WebxdcMaxSingleFileMB = int(cfg.Webxdc.MaxSingleFileBytes / mib)
		if s.WebxdcMaxSingleFileMB <= 0 {
			s.WebxdcMaxSingleFileMB = 5
		}
	}
	if s.WebxdcMaxEntries <= 0 {
		if cfg.Webxdc.MaxEntries > 0 {
			s.WebxdcMaxEntries = cfg.Webxdc.MaxEntries
		} else {
			s.WebxdcMaxEntries = 500
		}
	}

	// LiveKit
	if s.LiveKitHost == "" {
		s.LiveKitHost = cfg.LiveKit.Host
	}
	if s.LiveKitAPIKey == "" {
		s.LiveKitAPIKey = cfg.LiveKit.APIKey
	}
	if s.LiveKitAPISecret == "" {
		s.LiveKitAPISecret = cfg.LiveKit.APISecret
	}
	if !s.LiveKitExternal && cfg.LiveKit.External {
		s.LiveKitExternal = cfg.LiveKit.External
	}

	// CORS
	if s.CORSAllowedOrigins == "" {
		s.CORSAllowedOrigins = cfg.Cors.AllowedOrigins
	}
	if s.CORSAllowedHeaders == "" {
		s.CORSAllowedHeaders = cfg.Cors.AllowedHeaders
	}
	if s.CORSAllowedMethods == "" {
		s.CORSAllowedMethods = cfg.Cors.AllowedMethods
	}
	if !s.CORSAllowCredentials && cfg.Cors.AllowCredentials {
		s.CORSAllowCredentials = cfg.Cors.AllowCredentials
	}
	if s.CORSMaxAge == 0 && cfg.Cors.MaxAge != 0 {
		s.CORSMaxAge = cfg.Cors.MaxAge.Int()
	}

	// Chat uploads
	if s.ChatUploadBackend == "" {
		s.ChatUploadBackend = cfg.Chat.Uploads.Backend
	}
	if s.ChatUploadMaxBytes == 0 && cfg.Chat.Uploads.MaxBytes != 0 {
		s.ChatUploadMaxBytes = cfg.Chat.Uploads.MaxBytes.Int64()
	}
	if s.ChatUploadMaxDimension == 0 && cfg.Chat.Uploads.MaxDimension != 0 {
		s.ChatUploadMaxDimension = cfg.Chat.Uploads.MaxDimension.Int()
	}
	if s.ChatUploadInlineMax == 0 && cfg.Chat.Uploads.InlineMaxBytes != 0 {
		s.ChatUploadInlineMax = cfg.Chat.Uploads.InlineMaxBytes.Int64()
	}
	if s.ChatUploadDiskDir == "" {
		s.ChatUploadDiskDir = cfg.Chat.Uploads.DiskDir
	}
	if s.ChatUploadS3Endpoint == "" {
		s.ChatUploadS3Endpoint = cfg.Chat.Uploads.S3.Endpoint
	}
	if s.ChatUploadS3Bucket == "" {
		s.ChatUploadS3Bucket = cfg.Chat.Uploads.S3.Bucket
	}
	if s.ChatUploadS3Region == "" {
		s.ChatUploadS3Region = cfg.Chat.Uploads.S3.Region
	}
	if s.ChatUploadS3AccessKey == "" {
		s.ChatUploadS3AccessKey = cfg.Chat.Uploads.S3.AccessKey
	}
	if s.ChatUploadS3SecretKey == "" {
		s.ChatUploadS3SecretKey = cfg.Chat.Uploads.S3.SecretKey
	}
	if s.ChatUploadS3PublicURL == "" {
		s.ChatUploadS3PublicURL = cfg.Chat.Uploads.S3.PublicBaseURL
	}

	// Room limits
	if s.MaxParticipantsLimit == 0 && cfg.Server.MaxParticipantsLimit != 0 {
		s.MaxParticipantsLimit = cfg.Server.MaxParticipantsLimit
	}
	if s.MaxRoomsPerUser == 0 && cfg.Server.MaxRoomsPerUser != 0 {
		s.MaxRoomsPerUser = cfg.Server.MaxRoomsPerUser
	}

	// Upload quotas
	if s.MaxUploadBytesPerUser == 0 && cfg.Chat.MaxUploadBytesPerUser != 0 {
		s.MaxUploadBytesPerUser = cfg.Chat.MaxUploadBytesPerUser
	}
	if s.GlobalDiskThresholdBytes == 0 && cfg.Chat.GlobalDiskThresholdBytes != 0 {
		s.GlobalDiskThresholdBytes = cfg.Chat.GlobalDiskThresholdBytes
	}

	// Chat message retention
	if s.ChatMaxMessageCount == 0 && cfg.Chat.MaxMessageCount != 0 {
		s.ChatMaxMessageCount = cfg.Chat.MaxMessageCount
	}
	if s.ChatMessageTTLHours == 0 && cfg.Chat.MessageTTLHours != 0 {
		s.ChatMessageTTLHours = cfg.Chat.MessageTTLHours
	}

	// Logger
	if s.LogLevel == "" {
		s.LogLevel = cfg.Logger.Level
	}

	// Email branding
	if s.EmailInstanceName == "" {
		s.EmailInstanceName = cfg.Email.Templates.InstanceName
	}
	if s.EmailSupportEmail == "" {
		s.EmailSupportEmail = cfg.Email.Templates.SupportEmail
	}
	if s.EmailInstanceURL == "" {
		s.EmailInstanceURL = cfg.Email.Templates.InstanceURL
	}
	if s.EmailHeaderBg == "" {
		s.EmailHeaderBg = cfg.Email.Templates.HeaderBgColor
	}
	if s.EmailButtonBg == "" {
		s.EmailButtonBg = cfg.Email.Templates.ButtonBgColor
	}

	// Per-template subject overrides
	if s.EmailSubjectVerify == "" {
		s.EmailSubjectVerify = cfg.Email.Templates.SubjectLines["verify_email"]
	}
	if s.EmailSubjectWelcome == "" {
		s.EmailSubjectWelcome = cfg.Email.Templates.SubjectLines["welcome"]
	}
	if s.EmailSubjectReset == "" {
		s.EmailSubjectReset = cfg.Email.Templates.SubjectLines["password_reset"]
	}
	if s.EmailSubjectChanged == "" {
		s.EmailSubjectChanged = cfg.Email.Templates.SubjectLines["password_changed"]
	}
	if s.EmailSubjectInvite == "" {
		s.EmailSubjectInvite = cfg.Email.Templates.SubjectLines["room_invite"]
	}

	// Per-template preheader overrides
	if s.EmailPreheaderVerify == "" {
		s.EmailPreheaderVerify = cfg.Email.Templates.PreheaderText["verify_email"]
	}
	if s.EmailPreheaderWelcome == "" {
		s.EmailPreheaderWelcome = cfg.Email.Templates.PreheaderText["welcome"]
	}
	if s.EmailPreheaderReset == "" {
		s.EmailPreheaderReset = cfg.Email.Templates.PreheaderText["password_reset"]
	}
	if s.EmailPreheaderChanged == "" {
		s.EmailPreheaderChanged = cfg.Email.Templates.PreheaderText["password_changed"]
	}
	if s.EmailPreheaderInvite == "" {
		s.EmailPreheaderInvite = cfg.Email.Templates.PreheaderText["room_invite"]
	}

	// SMTP settings from DB
	if s.EmailSMTPHost == "" {
		s.EmailSMTPHost = cfg.Email.SMTPHost
	}
	if s.EmailSMTPPort == 0 {
		s.EmailSMTPPort = cfg.Email.SMTPPort
	}
	if s.EmailUsername == "" {
		s.EmailUsername = cfg.Email.Username
	}
	if s.EmailPassword == "" {
		s.EmailPassword = cfg.Email.Password
	}
	if s.EmailFromAddress == "" {
		s.EmailFromAddress = cfg.Email.FromAddress
	}
	if s.EmailFromName == "" {
		s.EmailFromName = cfg.Email.FromName
	}
	if !s.EmailTLSSkipVerify && cfg.Email.TLSSkipVerify {
		s.EmailTLSSkipVerify = cfg.Email.TLSSkipVerify
	}
	if !s.EmailSMTPSMode && cfg.Email.SMTPSMode {
		s.EmailSMTPSMode = cfg.Email.SMTPSMode
	}
}
