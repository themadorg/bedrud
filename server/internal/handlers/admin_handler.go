package handlers

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/lkutil"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/utils"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/livekit/protocol/livekit"
	"github.com/rs/zerolog/log"
	"github.com/twitchtv/twirp"
)

// validateSettings checks that settings values are within acceptable ranges.
func validateSettings(s *models.SystemSettings) error {
	// Token duration
	if s.TokenDuration != 0 && (s.TokenDuration < 1 || s.TokenDuration > 8760) {
		return fmt.Errorf("tokenDuration must be between 1 and 8760 hours, or 0 for default")
	}

	// Chat upload backend
	validBackends := map[string]bool{uploadBackendDisk: true, uploadBackendInline: true, uploadBackendS3: true, "": true}
	if !validBackends[s.ChatUploadBackend] {
		return fmt.Errorf("chatUploadBackend must be disk, inline, or s3")
	}

	// Chat upload sizes
	if s.ChatUploadMaxBytes < 0 {
		return fmt.Errorf("chatUploadMaxBytes cannot be negative")
	}
	if s.ChatUploadMaxDimension < 0 {
		return fmt.Errorf("chatUploadMaxDimension cannot be negative")
	}
	if s.ChatUploadInlineMax < 0 {
		return fmt.Errorf("chatUploadInlineMax cannot be negative")
	}

	// Log level
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true, "trace": true, "": true}
	if !validLevels[s.LogLevel] {
		return fmt.Errorf("invalid logLevel")
	}

	// Room limits
	if s.MaxParticipantsLimit < 0 || s.MaxParticipantsLimit > 100000 {
		return fmt.Errorf("maxParticipantsLimit must be between 0 and 100000")
	}
	if s.MaxRoomsPerUser < 0 || s.MaxRoomsPerUser > 100000 {
		return fmt.Errorf("maxRoomsPerUser must be between 0 and 100000")
	}

	// Upload quotas
	if s.MaxUploadBytesPerUser < 0 {
		return fmt.Errorf("maxUploadBytesPerUser cannot be negative")
	}
	if s.GlobalDiskThresholdBytes < 0 {
		return fmt.Errorf("globalDiskThresholdBytes cannot be negative")
	}

	// Chat message retention
	if s.ChatMaxMessageCount < 0 {
		return fmt.Errorf("chatMaxMessageCount cannot be negative")
	}
	if s.ChatMessageTTLHours < 0 {
		return fmt.Errorf("chatMessageTTLHours cannot be negative")
	}

	// JWT secret
	if s.JWTSecret != "" && len(s.JWTSecret) < 32 {
		return fmt.Errorf("jwtSecret must be at least 32 characters")
	}

	// Server port
	if s.ServerPort != "" {
		port, err := strconv.Atoi(s.ServerPort)
		if err != nil || port < 1 || port > 65535 {
			return fmt.Errorf("serverPort must be a valid port number between 1 and 65535")
		}
	}

	// URL format checks — parse but don't connect
	type urlCheck struct {
		val  string
		name string
	}
	urlFields := []urlCheck{
		{s.FrontendURL, settingFrontendURL},
		{s.LiveKitHost, settingLiveKitHost},
		{s.GoogleRedirectURL, settingGoogleRedirectURL},
		{s.GithubRedirectURL, "githubRedirectUrl"},
		{s.TwitterRedirectURL, "twitterRedirectUrl"},
		{s.ChatUploadS3Endpoint, "chatUploadS3Endpoint"},
		{s.ChatUploadS3PublicURL, "chatUploadS3PublicUrl"},
	}
	for _, f := range urlFields {
		if f.val != "" {
			parsed, err := url.Parse(f.val)
			if err != nil {
				return fmt.Errorf("%s: invalid URL", f.name)
			}
			// Must have a scheme and host (absolute URL)
			if parsed.Scheme == "" || parsed.Host == "" {
				return fmt.Errorf("%s: must be an absolute URL (scheme + host required)", f.name)
			}
			// Reject non-http/https/wss schemes (javascript:, file:, data:, etc.)
			if parsed.Scheme != schemeHTTP && parsed.Scheme != schemeHTTPS && parsed.Scheme != schemeWS && parsed.Scheme != schemeWSS {
				return fmt.Errorf("%s: unsupported URL scheme %q, must be http/https/ws/wss", f.name, parsed.Scheme)
			}
		}
	}

	// Email format
	if s.ServerEmail != "" {
		if _, err := mail.ParseAddress(s.ServerEmail); err != nil {
			return fmt.Errorf("serverEmail: invalid email format")
		}
	}

	// CORS — disallow credentials when any origin is wildcard
	if s.CORSAllowCredentials {
		if s.CORSAllowedOrigins == "" || s.CORSAllowedOrigins == "*" {
			return fmt.Errorf("corsAllowCredentials cannot be true when corsAllowedOrigins is '*' or empty")
		}
		origins := strings.Split(s.CORSAllowedOrigins, ",")
		for _, o := range origins {
			if strings.TrimSpace(o) == "*" {
				return fmt.Errorf("corsAllowCredentials cannot be true when corsAllowedOrigins contains '*'")
			}
		}
	}
	if s.CORSMaxAge < 0 {
		return fmt.Errorf("corsMaxAge cannot be negative")
	}
	if s.CORSMaxAge > 86400 {
		return fmt.Errorf("corsMaxAge cannot exceed 86400 (24 hours)")
	}

	// Cross-field: TLS + !ACME → cert + key required
	if s.ServerEnableTLS && !s.ServerUseACME {
		if s.ServerCertFile == "" {
			return fmt.Errorf("serverCertFile is required when TLS enabled without ACME")
		}
		if s.ServerKeyFile == "" {
			return fmt.Errorf("serverKeyFile is required when TLS enabled without ACME")
		}
	}

	// Cross-field: LiveKit external → key + secret required
	if s.LiveKitExternal {
		if s.LiveKitAPIKey == "" {
			return fmt.Errorf("livekitApiKey is required for external LiveKit server")
		}
		if s.LiveKitAPISecret == "" {
			return fmt.Errorf("livekitApiSecret is required for external LiveKit server")
		}
	}

	// Cross-field: ACME → email required
	if s.ServerUseACME {
		if !s.ServerEnableTLS {
			return fmt.Errorf("serverUseAcme requires serverEnableTls to be true")
		}
		if s.ServerEmail == "" {
			return fmt.Errorf("serverEmail is required when using ACME")
		}
	}

	// Email branding — validate hex colors
	if s.EmailHeaderBg != "" && !isValidHexColor(s.EmailHeaderBg) {
		return fmt.Errorf("emailHeaderBg must be a valid hex color (#rrggbb)")
	}
	if s.EmailButtonBg != "" && !isValidHexColor(s.EmailButtonBg) {
		return fmt.Errorf("emailButtonBg must be a valid hex color (#rrggbb)")
	}

	// Email branding — validate instance URL if set
	if s.EmailInstanceURL != "" {
		parsed, err := url.Parse(s.EmailInstanceURL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return fmt.Errorf("emailInstanceUrl: must be a valid absolute URL")
		}
		if parsed.Scheme != schemeHTTP && parsed.Scheme != schemeHTTPS {
			return fmt.Errorf("emailInstanceUrl: unsupported URL scheme %q, must be http/https", parsed.Scheme)
		}
	}

	// Email branding — validate support email if set
	if s.EmailSupportEmail != "" {
		if _, err := mail.ParseAddress(s.EmailSupportEmail); err != nil {
			return fmt.Errorf("emailSupportEmail: invalid email format")
		}
	}

	// Email password minimum length
	if s.EmailPassword != "" && len(s.EmailPassword) < 4 {
		return fmt.Errorf("emailPassword must be at least 4 characters")
	}

	// SMTP port validation
	if s.EmailSMTPPort != 0 && (s.EmailSMTPPort < 1 || s.EmailSMTPPort > 65535) {
		return fmt.Errorf("emailSmtpPort must be between 1 and 65535, or 0 for config default")
	}

	return nil
}

// isValidHexColor checks that a string is a valid 6-character hex color with # prefix.
func isValidHexColor(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

const maskedSecret = "••••••••"

type AdminHandler struct {
	settingsRepo    *repository.SettingsRepository
	inviteTokenRepo *repository.InviteTokenRepository
	webhookRepo     *repository.WebhookRepository
	// TODO oncoming feature
	recordingRepo *repository.RecordingRepository
}

func NewAdminHandler(
	sr *repository.SettingsRepository,
	itr *repository.InviteTokenRepository,
	wr *repository.WebhookRepository,
	rr *repository.RecordingRepository,
) *AdminHandler {
	return &AdminHandler{settingsRepo: sr, inviteTokenRepo: itr, webhookRepo: wr, recordingRepo: rr}
}

// GetSettings returns effective system settings with secrets masked.
// GET /api/admin/settings
//
// @Summary Get system settings
// @Description Get effective system settings. Superadmin access required. Secret fields are masked.
// @Tags admin
// @Accept json
// @Produce json
// @Success 200 {object} models.SystemSettings "Settings with masked secrets"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 500 {object} ErrorResponse "Failed to fetch settings"
// @Router /admin/settings [get]
func (h *AdminHandler) GetSettings(c *fiber.Ctx) error {
	s, err := h.settingsRepo.GetEffectiveSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch settings")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch settings"})
	}
	return c.JSON(maskSettings(s))
}

// GetPublicSettings returns only the fields relevant to anonymous visitors (no auth required).
// GET /api/auth/settings
//
// @Summary Get public settings
// @Description Get public settings visible to unauthenticated visitors (registration, OAuth providers, etc.)
// @Tags system
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Public settings"
// @Failure 500 {object} ErrorResponse "Failed to fetch settings"
// @Router /auth/settings [get]
func (h *AdminHandler) GetPublicSettings(c *fiber.Ctx) error {
	s, err := h.settingsRepo.GetEffectiveSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch public settings")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch settings"})
	}
	requireEmailVerify := false
	if cfg := config.GetSafe(); cfg != nil {
		requireEmailVerify = cfg.Auth.RequireEmailVerification
	}
	maxBytes := s.ChatUploadMaxBytes
	if maxBytes <= 0 {
		maxBytes = defaultChatUploadMaxBytes
	}
	maxDim := s.ChatUploadMaxDimension
	if maxDim <= 0 {
		maxDim = defaultChatUploadMaxDimension
	}
	webxdcEnabled := false
	webxdcBase := ""
	if cfg := config.GetSafe(); cfg != nil && cfg.Webxdc.Active(cfg.Server.Domain) {
		webxdcEnabled = true
		webxdcBase = cfg.Webxdc.BaseDomain
	}
	return c.JSON(fiber.Map{
		"serverName":               s.ServerName,
		"registrationEnabled":      s.RegistrationEnabled,
		"tokenRegistrationOnly":    s.TokenRegistrationOnly,
		"guestLoginEnabled":        s.GuestLoginEnabled,
		"passkeysEnabled":          s.PasskeysEnabled,
		"oauthProviders":           auth.ConfiguredProviders(),
		"requireEmailVerification": requireEmailVerify,
		"chatMaxMessageCount":      s.ChatMaxMessageCount,
		"chatMessageTTLHours":      s.ChatMessageTTLHours,
		"chatUploadMaxBytes":       maxBytes,
		"chatUploadMaxDimension":   maxDim,
		"recordingsEnabled": s.RecordingsEnabled,
		"rnnoiseEnabled":    s.RNNoiseEnabled,
		"krispEnabled":      s.KrispEnabled,
		"webxdc": fiber.Map{
			"enabled":      webxdcEnabled,
			"experimental": true,
			"baseDomain":   webxdcBase,
		},
	})
}

// UpdateSettings applies partial updates to system settings.
// PUT /api/admin/settings
//
// @Summary Update system settings
// @Description Update system settings via partial JSON merge. Superadmin access required. Secrets sent as "••••••••" are preserved.
// @Tags admin
// @Accept json
// @Produce json
// @Param request body object true "Partial settings JSON"
// @Success 200 {object} models.SystemSettings "Updated settings with masked secrets"
// @Failure 400 {object} ErrorResponse "Validation error"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 500 {object} ErrorResponse "Failed to save settings"
// @Router /admin/settings [put]
func (h *AdminHandler) UpdateSettings(c *fiber.Ctx) error {
	// Get existing settings first (to use as base for partial updates)
	existing, err := h.settingsRepo.GetSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch current settings")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch current settings"})
	}

	// Parse raw JSON body to detect which fields the client actually sent
	// (rather than zeroing unset fields via direct struct unmarshal)
	var raw map[string]json.RawMessage
	if err := c.BodyParser(&raw); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Apply only the fields present in the request onto the existing settings
	if err := applySettingsFields(existing, raw); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	// Validate merged settings
	if err := validateSettings(existing); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	existing.ID = 1
	if err := h.settingsRepo.SaveSettings(existing); err != nil {
		log.Error().Err(err).Msg("Failed to save settings")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save settings"})
	}

	// Reload runtime-configurable subsystems
	effective, err := h.settingsRepo.GetEffectiveSettings()
	if err != nil {
		log.Error().Err(err).Msg("Settings saved but failed to reload")
		return c.Status(500).JSON(fiber.Map{"error": "Settings saved but failed to reload"})
	}
	auth.ReloadProviders(effective)

	log.Info().Msg("Admin settings updated and providers reloaded")
	return c.JSON(maskSettings(effective))
}

// applySettingsFields selectively applies fields from raw JSON onto existing settings.
// Only fields present in the JSON body are applied; others retain their current value.
// Returns an error if a field value has the wrong type for its expected Go type.
func applySettingsFields(existing *models.SystemSettings, raw map[string]json.RawMessage) error {
	for key, val := range raw {
		switch key {
		// Secrets — handle masked placeholder
		case "googleClientSecret", "githubClientSecret", "twitterClientSecret",
			"jwtSecret", "sessionSecret", "livekitApiSecret",
			"chatUploadS3AccessKey", "chatUploadS3SecretKey",
			"emailPassword":
			var s string
			if err := json.Unmarshal(val, &s); err != nil {
				return fmt.Errorf("%s: expected a string, got %s", key, describeJSONType(val))
			}
			if strings.TrimSpace(s) == maskedSecret {
				// keep existing value
			} else {
				switch key {
				case "googleClientSecret":
					existing.GoogleClientSecret = s
				case "githubClientSecret":
					existing.GithubClientSecret = s
				case "twitterClientSecret":
					existing.TwitterClientSecret = s
				case "jwtSecret":
					existing.JWTSecret = s
				case "sessionSecret":
					existing.SessionSecret = s
				case "livekitApiSecret":
					existing.LiveKitAPISecret = s
				case "chatUploadS3AccessKey":
					existing.ChatUploadS3AccessKey = s
				case "chatUploadS3SecretKey":
					existing.ChatUploadS3SecretKey = s
				case "emailPassword":
					existing.EmailPassword = s
				}
			}

		// String fields
		case "googleClientId", settingGoogleRedirectURL,
			"githubClientId", "githubRedirectUrl",
			"twitterClientId", "twitterRedirectUrl",
			settingFrontendURL, "serverPort", "serverHost", "serverDomain",
			"serverCertFile", "serverKeyFile", "serverEmail",
			settingLiveKitHost, "livekitApiKey",
			"corsAllowedOrigins", "corsAllowedHeaders", "corsAllowedMethods",
			"chatUploadBackend", "chatUploadDiskDir",
			"chatUploadS3Endpoint", "chatUploadS3Bucket", "chatUploadS3Region",
			"chatUploadS3PublicUrl", "logLevel",
			"serverName",
			"emailInstanceName", "emailSupportEmail", "emailInstanceUrl",
			"emailHeaderBg", "emailButtonBg",
			"emailSubjectVerify", "emailSubjectWelcome",
			"emailSubjectReset", "emailSubjectChanged", "emailSubjectInvite",
			"emailPreheaderVerify", "emailPreheaderWelcome",
			"emailPreheaderReset", "emailPreheaderChanged", "emailPreheaderInvite",
			"emailSmtpHost", "emailUsername", "emailFromAddress", "emailFromName",
			"webxdcGalleryRemoteCatalogUrl", "webxdcGallerySource":
			var s string
			if err := json.Unmarshal(val, &s); err != nil {
				return fmt.Errorf("%s: expected a string, got %s", key, describeJSONType(val))
			}
			switch key {
			case "googleClientId":
				existing.GoogleClientID = s
			case settingGoogleRedirectURL:
				existing.GoogleRedirectURL = s
			case "githubClientId":
				existing.GithubClientID = s
			case "githubRedirectUrl":
				existing.GithubRedirectURL = s
			case "twitterClientId":
				existing.TwitterClientID = s
			case "twitterRedirectUrl":
				existing.TwitterRedirectURL = s
			case settingFrontendURL:
				existing.FrontendURL = s
			case "serverPort":
				existing.ServerPort = s
			case "serverHost":
				existing.ServerHost = s
			case "serverDomain":
				existing.ServerDomain = s
			case "serverCertFile":
				existing.ServerCertFile = s
			case "serverKeyFile":
				existing.ServerKeyFile = s
			case "serverEmail":
				existing.ServerEmail = s
			case settingLiveKitHost:
				existing.LiveKitHost = s
			case "livekitApiKey":
				existing.LiveKitAPIKey = s
			case "corsAllowedOrigins":
				existing.CORSAllowedOrigins = s
			case "corsAllowedHeaders":
				existing.CORSAllowedHeaders = s
			case "corsAllowedMethods":
				existing.CORSAllowedMethods = s
			case "chatUploadBackend":
				existing.ChatUploadBackend = s
			case "chatUploadDiskDir":
				existing.ChatUploadDiskDir = s
			case "chatUploadS3Endpoint":
				existing.ChatUploadS3Endpoint = s
			case "chatUploadS3Bucket":
				existing.ChatUploadS3Bucket = s
			case "chatUploadS3Region":
				existing.ChatUploadS3Region = s
			case "chatUploadS3PublicUrl":
				existing.ChatUploadS3PublicURL = s
			case "logLevel":
				existing.LogLevel = s
			case "serverName":
				existing.ServerName = s
			case "emailInstanceName":
				existing.EmailInstanceName = s
			case "emailSupportEmail":
				existing.EmailSupportEmail = s
			case "emailInstanceUrl":
				existing.EmailInstanceURL = s
			case "emailHeaderBg":
				existing.EmailHeaderBg = s
			case "emailButtonBg":
				existing.EmailButtonBg = s
			case "emailSubjectVerify":
				existing.EmailSubjectVerify = s
			case "emailSubjectWelcome":
				existing.EmailSubjectWelcome = s
			case "emailSubjectReset":
				existing.EmailSubjectReset = s
			case "emailSubjectChanged":
				existing.EmailSubjectChanged = s
			case "emailSubjectInvite":
				existing.EmailSubjectInvite = s
			case "emailPreheaderVerify":
				existing.EmailPreheaderVerify = s
			case "emailPreheaderWelcome":
				existing.EmailPreheaderWelcome = s
			case "emailPreheaderReset":
				existing.EmailPreheaderReset = s
			case "emailPreheaderChanged":
				existing.EmailPreheaderChanged = s
			case "emailPreheaderInvite":
				existing.EmailPreheaderInvite = s
			case "emailSmtpHost":
				existing.EmailSMTPHost = s
			case "emailUsername":
				existing.EmailUsername = s
			case "emailFromAddress":
				existing.EmailFromAddress = s
			case "emailFromName":
				existing.EmailFromName = s
			case "webxdcGalleryRemoteCatalogUrl":
				existing.WebxdcGalleryRemoteCatalogURL = strings.TrimSpace(s)
			case "webxdcGallerySource":
				s = strings.TrimSpace(s)
				switch s {
				case "", "local", "remote", "both", "semi-remote":
					existing.WebxdcGallerySource = s
				default:
					return fmt.Errorf("webxdcGallerySource: must be local, semi-remote, remote, or both")
				}
			}

		// Bool fields
		case "registrationEnabled", "tokenRegistrationOnly", "passkeysEnabled",
			"serverEnableTls", "serverUseAcme", "behindProxy",
			"livekitExternal", "corsAllowCredentials", "guestLoginEnabled",
			"recordingsEnabled",
			"webxdcGalleryEnabled", "webxdcGalleryAllowRemoteDownload", "webxdcGalleryInstanceCatalogEnabled",
			"rnnoiseEnabled", "krispEnabled",
			"emailTlsSkipVerify", "emailSmtpsMode":
			var b bool
			if err := json.Unmarshal(val, &b); err != nil {
				return fmt.Errorf("%s: expected a boolean, got %s", key, describeJSONType(val))
			}
			switch key {
			case "registrationEnabled":
				existing.RegistrationEnabled = b
			case "tokenRegistrationOnly":
				existing.TokenRegistrationOnly = b
			case "passkeysEnabled":
				existing.PasskeysEnabled = b
			case "serverEnableTls":
				existing.ServerEnableTLS = b
			case "serverUseAcme":
				existing.ServerUseACME = b
			case "behindProxy":
				existing.BehindProxy = b
			case "livekitExternal":
				existing.LiveKitExternal = b
			case "corsAllowCredentials":
				existing.CORSAllowCredentials = b
			case "guestLoginEnabled":
				existing.GuestLoginEnabled = b
			case "recordingsEnabled":
				existing.RecordingsEnabled = b
			case "webxdcGalleryEnabled":
				existing.WebxdcGalleryEnabled = b
			case "webxdcGalleryAllowRemoteDownload":
				existing.WebxdcGalleryAllowRemoteDownload = b
			case "webxdcGalleryInstanceCatalogEnabled":
				existing.WebxdcGalleryInstanceCatalogEnabled = b
			case "rnnoiseEnabled":
				existing.RNNoiseEnabled = b
			case "krispEnabled":
				existing.KrispEnabled = b
			case "emailTlsSkipVerify":
				existing.EmailTLSSkipVerify = b
			case "emailSmtpsMode":
				existing.EmailSMTPSMode = b
			}

		// Int fields
		case "corsMaxAge", "tokenDuration",
			"maxParticipantsLimit", "maxRoomsPerUser",
			"chatMaxMessageCount", "chatMessageTTLHours",
			"chatUploadMaxDimension",
			"recordingMaxDurationMins", "recordingMaxFileSizeMB",
			"emailSmtpPort",
			"webxdcMaxArchiveMB", "webxdcMaxUncompressedMB", "webxdcMaxSingleFileMB", "webxdcMaxEntries":
			var i int
			if err := json.Unmarshal(val, &i); err != nil {
				return fmt.Errorf("%s: expected an integer, got %s", key, describeJSONType(val))
			}
			switch key {
			case "corsMaxAge":
				existing.CORSMaxAge = i
			case "tokenDuration":
				existing.TokenDuration = i
			case "maxParticipantsLimit":
				existing.MaxParticipantsLimit = i
			case "maxRoomsPerUser":
				existing.MaxRoomsPerUser = i
			case "chatMaxMessageCount":
				existing.ChatMaxMessageCount = i
			case "chatMessageTTLHours":
				existing.ChatMessageTTLHours = i
			case "chatUploadMaxDimension":
				existing.ChatUploadMaxDimension = i
			case "recordingMaxDurationMins":
				existing.RecordingMaxDurationMins = i
			case "recordingMaxFileSizeMB":
				existing.RecordingMaxFileSizeMB = i
			case "emailSmtpPort":
				existing.EmailSMTPPort = i
			case "webxdcMaxArchiveMB":
				if i < 0 || i > 512 {
					return fmt.Errorf("webxdcMaxArchiveMB: must be 0–512 (0 = config default)")
				}
				existing.WebxdcMaxArchiveMB = i
			case "webxdcMaxUncompressedMB":
				if i < 0 || i > 1024 {
					return fmt.Errorf("webxdcMaxUncompressedMB: must be 0–1024 (0 = config default)")
				}
				existing.WebxdcMaxUncompressedMB = i
			case "webxdcMaxSingleFileMB":
				if i < 0 || i > 256 {
					return fmt.Errorf("webxdcMaxSingleFileMB: must be 0–256 (0 = config default)")
				}
				existing.WebxdcMaxSingleFileMB = i
			case "webxdcMaxEntries":
				if i < 0 || i > 10000 {
					return fmt.Errorf("webxdcMaxEntries: must be 0–10000 (0 = config default)")
				}
				existing.WebxdcMaxEntries = i
			}

		// Int64 fields
		case "chatUploadMaxBytes", "chatUploadInlineMax",
			"maxUploadBytesPerUser", "globalDiskThresholdBytes":
			var i int64
			if err := json.Unmarshal(val, &i); err != nil {
				return fmt.Errorf("%s: expected an integer, got %s", key, describeJSONType(val))
			}
			switch key {
			case "chatUploadMaxBytes":
				existing.ChatUploadMaxBytes = i
			case "chatUploadInlineMax":
				existing.ChatUploadInlineMax = i
			case "maxUploadBytesPerUser":
				existing.MaxUploadBytesPerUser = i
			case "globalDiskThresholdBytes":
				existing.GlobalDiskThresholdBytes = i
			}
		}
	}
	return nil
}

// describeJSONType returns a human-readable type name for a JSON raw message.
func describeJSONType(raw json.RawMessage) string {
	raw = bytes.TrimSpace(raw)
	if len(raw) == 0 {
		return "null"
	}
	switch raw[0] {
	case '"':
		return "a string"
	case '{':
		return "an object"
	case '[':
		return "an array"
	case 't', 'f':
		return "a boolean"
	case 'n':
		return "null"
	default:
		if bytes.ContainsAny(raw, ".eE") {
			return "a float (expected integer)"
		}
		return "a number"
	}
}

// maskSettings returns a copy with secret fields replaced by a placeholder.
func maskSettings(s *models.SystemSettings) *models.SystemSettings {
	cp := *s
	if cp.GoogleClientSecret != "" {
		cp.GoogleClientSecret = maskedSecret
	}
	if cp.GithubClientSecret != "" {
		cp.GithubClientSecret = maskedSecret
	}
	if cp.TwitterClientSecret != "" {
		cp.TwitterClientSecret = maskedSecret
	}
	if cp.JWTSecret != "" {
		cp.JWTSecret = maskedSecret
	}
	if cp.SessionSecret != "" {
		cp.SessionSecret = maskedSecret
	}
	if cp.LiveKitAPISecret != "" {
		cp.LiveKitAPISecret = maskedSecret
	}
	if cp.ChatUploadS3AccessKey != "" {
		cp.ChatUploadS3AccessKey = maskedSecret
	}
	if cp.ChatUploadS3SecretKey != "" {
		cp.ChatUploadS3SecretKey = maskedSecret
	}
	if cp.EmailPassword != "" {
		cp.EmailPassword = maskedSecret
	}
	return &cp
}

// ListInviteTokens returns paginated invite tokens.
// GET /api/admin/invite-tokens
//
// @Summary List invite tokens
// @Description Get paginated list of registration invite tokens. Superadmin access required.
// @Tags admin
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(50)
// @Success 200 {object} map[string]interface{} "{tokens, total}"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 500 {object} ErrorResponse "Failed to fetch tokens"
// @Router /admin/invite-tokens [get]
func (h *AdminHandler) ListInviteTokens(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	tokens, total, err := h.inviteTokenRepo.List(repository.PaginationParams{Page: page, Limit: limit})
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch invite tokens")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch tokens"})
	}
	if tokens == nil {
		tokens = []models.InviteToken{}
	}
	type tokenResponse struct {
		models.InviteToken
		Used bool `json:"used"`
	}
	out := make([]tokenResponse, len(tokens))
	for i := range tokens {
		out[i] = tokenResponse{InviteToken: tokens[i], Used: tokens[i].UsedAt != nil}
	}
	return c.JSON(fiber.Map{"tokens": out, "total": total})
}

// CreateInviteToken creates a new registration invite token.
// POST /api/admin/invite-tokens
//
// @Summary Create invite token
// @Description Create a new registration invite token. Superadmin access required.
// @Tags admin
// @Accept json
// @Produce json
// @Param request body object true "{email?: string, expiresInHours?: int}"
// @Success 201 {object} models.InviteToken "Created token"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 500 {object} ErrorResponse "Failed to create token"
// @Router /admin/invite-tokens [post]
func (h *AdminHandler) CreateInviteToken(c *fiber.Ctx) error {
	claims := c.Locals("user").(*auth.Claims)
	var input struct {
		Email     string `json:"email"`
		ExpiresIn int    `json:"expiresInHours"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if input.ExpiresIn <= 0 {
		input.ExpiresIn = 72
	}
	if input.ExpiresIn > 720 {
		return c.Status(400).JSON(fiber.Map{"error": "expiresInHours cannot exceed 720 (30 days)"})
	}

	if input.Email != "" {
		if _, err := mail.ParseAddress(input.Email); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid email format"})
		}
	}

	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate secure token"})
	}
	token := &models.InviteToken{
		ID:        uuid.NewString(),
		Token:     hex.EncodeToString(b),
		Email:     input.Email,
		CreatedBy: claims.UserID,
		ExpiresAt: time.Now().Add(time.Duration(input.ExpiresIn) * time.Hour),
	}
	if err := h.inviteTokenRepo.Create(token); err != nil {
		log.Error().Err(err).Msg("Failed to create invite token")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create token"})
	}
	return c.Status(201).JSON(token)
}

// SendTestEmail sends a synchronous test email to verify SMTP configuration end-to-end.
// POST /api/admin/settings/send-test-email
//
// @Summary Send test email
// @Description Send a synchronous test email to verify SMTP configuration. Superadmin access required.
// @Tags admin
// @Accept json
// @Produce json
// @Param request body object true "{to: string} - recipient email"
// @Success 200 {object} map[string]interface{} "Test email sent"
// @Failure 400 {object} ErrorResponse "Invalid request or SMTP error"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 408 {object} ErrorResponse "SMTP connection timed out"
// @Failure 500 {object} ErrorResponse "Failed to load settings"
// @Router /admin/settings/send-test-email [post]
func (h *AdminHandler) SendTestEmail(c *fiber.Ctx) error {
	var input struct {
		To string `json:"to"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if input.To == "" {
		return c.Status(400).JSON(fiber.Map{"error": "to field is required"})
	}
	if _, err := mail.ParseAddress(input.To); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid email format"})
	}

	effective, err := h.settingsRepo.GetEffectiveSettings()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to load settings"})
	}

	// Build SMTP config from effective settings with fallback to config.yaml
	smtpHost := effective.EmailSMTPHost
	smtpPort := effective.EmailSMTPPort
	username := effective.EmailUsername
	password := effective.EmailPassword
	fromAddr := effective.EmailFromAddress
	fromName := effective.EmailFromName
	tlsSkip := effective.EmailTLSSkipVerify
	smtpsMode := effective.EmailSMTPSMode

	// Double-check: if still empty, try config.yaml directly
	if smtpHost == "" || smtpPort == 0 {
		if cfg := config.GetSafe(); cfg != nil {
			if smtpHost == "" {
				smtpHost = cfg.Email.SMTPHost
			}
			if smtpPort == 0 {
				smtpPort = cfg.Email.SMTPPort
			}
			if username == "" {
				username = cfg.Email.Username
			}
			if password == "" {
				password = cfg.Email.Password
			}
			if fromAddr == "" {
				fromAddr = cfg.Email.FromAddress
			}
			if fromName == "" {
				fromName = cfg.Email.FromName
			}
			if !tlsSkip && cfg.Email.TLSSkipVerify {
				tlsSkip = cfg.Email.TLSSkipVerify
			}
			if !smtpsMode && cfg.Email.SMTPSMode {
				smtpsMode = cfg.Email.SMTPSMode
			}
		}
	}

	if smtpHost == "" || smtpPort == 0 {
		return c.Status(400).JSON(fiber.Map{"error": "SMTP not configured — set host and port in settings"})
	}

	if fromAddr == "" {
		fromAddr = "noreply@bedrud"
	}
	if fromName == "" {
		fromName = effective.EmailInstanceName
		if fromName == "" {
			fromName = "Bedrud"
		}
	}

	instanceName := effective.EmailInstanceName
	if instanceName == "" {
		instanceName = "Bedrud"
	}
	now := time.Now().UTC().Format(time.RFC1123Z)

	subject := fmt.Sprintf("Test Email from %s", instanceName)
	bodyHTML := buildTestEmailHTML(instanceName, effective.EmailButtonBg, effective.EmailHeaderBg)
	bodyPlain := fmt.Sprintf(
		"This is a test email from %s.\nSent at: %s\nIf you see this, SMTP is working.\n",
		instanceName, now,
	)

	addr := net.JoinHostPort(smtpHost, fmt.Sprint(smtpPort))
	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, smtpHost)
	}

	msg := utils.BuildMessage(fromName, fromAddr, input.To, subject, bodyHTML, bodyPlain)

	ctx, cancel := context.WithTimeout(c.Context(), 30*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				errCh <- fmt.Errorf("panic in SendSMTP: %v", r)
			}
		}()
		errCh <- utils.SendSMTP(addr, auth, fromAddr, []string{input.To}, []byte(msg), smtpHost, tlsSkip, smtpsMode)
	}()

	select {
	case err := <-errCh:
		if err != nil {
			log.Error().Err(err).Str("to", input.To).Msg("Test email send failed")
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		log.Info().Str("to", input.To).Msg("Test email sent successfully")
		return c.JSON(fiber.Map{"status": "ok", "message": fmt.Sprintf("Test email sent to %s", input.To)})
	case <-ctx.Done():
		return c.Status(408).JSON(fiber.Map{"error": "SMTP connection timed out after 30s"})
	}
}

func buildTestEmailHTML(instanceName, buttonBg, headerBg string) string {
	if headerBg == "" {
		headerBg = "#1a1a2e"
	}
	if buttonBg == "" {
		buttonBg = "#e11d48"
	}
	now := time.Now().UTC().Format(time.RFC1123Z)
	return fmt.Sprintf(`
<!DOCTYPE html><html><body style="margin:0;padding:0;font-family:sans-serif;">
<div style="background:%s;padding:24px;text-align:center;">
  <h1 style="color:#fff;margin:0;">%s — Test Email</h1>
</div>
<div style="padding:32px;color:#333;line-height:1.7;">
  <p>If you see this, your SMTP configuration is working correctly.</p>
  <p style="color:#666;font-size:14px;">Sent at %s</p>
  <p style="margin-top:24px;"><a href="#" style="display:inline-block;background:%s;color:#fff;padding:12px 24px;text-decoration:none;">Test Button</a></p>
</div>
<div style="background:#f5f5f5;padding:16px;text-align:center;font-size:12px;color:#666;">
  <p>This is an automated test email from %s. No action required.</p>
</div>
</body></html>`, headerBg, instanceName, now, buttonBg, instanceName)
}

// ValidateSettingsConnectivity runs runtime checks against external services
// using the provided settings subset. Returns per-check status without saving.
// POST /api/admin/settings/validate
//
// @Summary Validate settings connectivity
// @Description Run connectivity checks against external services (LiveKit, S3, email, TLS) without saving. Superadmin access required.
// @Tags admin
// @Accept json
// @Produce json
// @Param request body object true "Partial settings to validate"
// @Success 200 {object} map[string]interface{} "{checks: {livekit, s3, tls, email}}"
// @Failure 400 {object} ErrorResponse "Invalid input"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Router /admin/settings/validate [post]
func (h *AdminHandler) ValidateSettingsConnectivity(c *fiber.Ctx) error {
	var raw map[string]json.RawMessage
	if err := c.BodyParser(&raw); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
	}

	results := make(map[string]interface{})

	// Build a partial SystemSettings from the request
	var s models.SystemSettings
	buf, err := json.Marshal(raw)
	if err != nil {
		results["marshal"] = failResult("failed to marshal settings: " + err.Error())
		return c.JSON(fiber.Map{"checks": results})
	}
	if err := json.Unmarshal(buf, &s); err != nil {
		results["unmarshal"] = failResult("invalid settings format: " + err.Error())
		return c.JSON(fiber.Map{"checks": results})
	}

	// LiveKit connectivity check
	if s.LiveKitHost != "" || s.LiveKitAPIKey != "" || s.LiveKitAPISecret != "" {
		results["livekit"] = checkLiveKitConnectivity(s.LiveKitHost, s.LiveKitAPIKey, s.LiveKitAPISecret)
	}

	// TLS certificate validation
	if s.ServerCertFile != "" || s.ServerKeyFile != "" {
		results["tls"] = checkTLSCerts(s.ServerCertFile, s.ServerKeyFile)
	}

	// S3 connectivity check
	if s.ChatUploadBackend == "s3" || s.ChatUploadS3Endpoint != "" || s.ChatUploadS3Bucket != "" {
		results["s3"] = checkS3Connectivity(&s)
	}

	// Email connectivity check (DNS MX lookup)
	if s.ServerEmail != "" {
		results["email"] = checkEmailDelivery(s.ServerEmail)
	}

	return c.JSON(fiber.Map{"checks": results})
}

type checkResult struct {
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

func okResult() checkResult {
	return checkResult{Status: "ok"}
}

func failResult(msg string) checkResult {
	return checkResult{Status: healthStatusError, Message: msg}
}

func skipResult(msg string) checkResult {
	return checkResult{Status: "skipped", Message: msg}
}

const checkTimeout = 10 * time.Second

func checkLiveKitConnectivity(host, apiKey, apiSecret string) checkResult {
	if host == "" {
		return skipResult("no host provided")
	}
	if apiKey == "" || apiSecret == "" {
		return skipResult("apiKey or apiSecret empty")
	}

	lkCfg := &config.LiveKitConfig{
		Host: host,
	}
	client := lkutil.NewClient(lkCfg)

	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()

	authCtx, err := lkutil.AuthContext(ctx, apiKey, apiSecret)
	if err != nil {
		return failResult("failed to create auth token: " + err.Error())
	}

	// Ping by listing rooms (empty filter)
	_, err = client.ListRooms(authCtx, &livekit.ListRoomsRequest{})
	if err != nil {
		if twirpErr, ok := err.(twirp.Error); ok {
			return failResult(twirpErr.Msg())
		}
		return failResult("connection failed: " + err.Error())
	}

	return okResult()
}

func checkTLSCerts(certFile, keyFile string) checkResult {
	if certFile == "" && keyFile == "" {
		return skipResult("no cert or key file specified")
	}
	if certFile == "" {
		return failResult("certFile is empty")
	}
	if keyFile == "" {
		return failResult("keyFile is empty")
	}

	info, err := utils.ValidateTLSCertPair(certFile, keyFile)
	if err != nil {
		return failResult(err.Error())
	}

	if info.Status == "expiring" {
		return checkResult{
			Status:  "warning",
			Message: fmt.Sprintf("Certificate expires in %d days (%s)", info.DaysRemaining, info.NotAfter.Format(time.RFC3339)),
		}
	}

	return okResult()
}

func checkS3Connectivity(s *models.SystemSettings) checkResult {
	if s.ChatUploadBackend != "" && s.ChatUploadBackend != "s3" {
		return skipResult(fmt.Sprintf("backend is %q, not \"s3\"", s.ChatUploadBackend))
	}
	if s.ChatUploadS3Endpoint == "" {
		return skipResult("endpoint not set")
	}
	if s.ChatUploadS3Bucket == "" {
		return failResult("bucket name is empty")
	}
	if s.ChatUploadS3AccessKey == "" || s.ChatUploadS3SecretKey == "" {
		return failResult("S3 access key or secret key is empty")
	}

	// Minimal connectivity: HEAD request to bucket endpoint
	endpoint := strings.TrimRight(s.ChatUploadS3Endpoint, "/")

	// Warn if S3 endpoint uses plain HTTP (credentials exposed)
	parsedURL, err := url.Parse(endpoint)
	if err == nil && parsedURL.Scheme != schemeHTTPS {
		return checkResult{
			Status:  "warning",
			Message: "S3 endpoint uses non-HTTPS — credentials sent in plaintext",
		}
	}

	url := fmt.Sprintf("%s/%s", endpoint, s.ChatUploadS3Bucket)

	req, err := http.NewRequest(http.MethodHead, url, http.NoBody)
	if err != nil {
		return failResult("failed to create request: " + err.Error())
	}

	ctx, cancel := context.WithTimeout(context.Background(), checkTimeout)
	defer cancel()
	req = req.WithContext(ctx)

	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := noRedirectClient.Do(req)
	if err != nil {
		return failResult("connection failed: " + err.Error())
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return failResult(fmt.Sprintf("bucket returned HTTP %d: %s", resp.StatusCode, resp.Status))
	}

	return okResult()
}

func checkEmailDelivery(email string) checkResult {
	_, err := mail.ParseAddress(email)
	if err != nil {
		return failResult("invalid email format: " + err.Error())
	}

	// Extract domain for MX lookup
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return failResult("invalid email: missing @")
	}
	domain := parts[1]

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var resolver net.Resolver
	mxRecords, err := resolver.LookupMX(ctx, domain)
	if err != nil {
		return checkResult{
			Status:  "warning",
			Message: fmt.Sprintf("domain %q has no MX records: %v — email delivery may fail", domain, err),
		}
	}
	if len(mxRecords) == 0 {
		return checkResult{
			Status:  "warning",
			Message: fmt.Sprintf("domain %q has no MX records — email delivery may fail", domain),
		}
	}

	return okResult()
}

// DeleteInviteToken deletes an invite token.
// DELETE /api/admin/invite-tokens/:id
//
// @Summary Delete invite token
// @Description Delete a registration invite token. Superadmin access required.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "Token ID"
// @Success 200 {object} map[string]string "{status: success}"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 500 {object} ErrorResponse "Failed to delete token"
// @Router /admin/invite-tokens/{id} [delete]
func (h *AdminHandler) DeleteInviteToken(c *fiber.Ctx) error {
	tokenID := c.Params("id")
	if err := h.inviteTokenRepo.Delete(tokenID); err != nil {
		log.Error().Err(err).Str("tokenID", tokenID).Msg("Failed to delete invite token")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete token"})
	}
	return c.JSON(fiber.Map{"status": "success"})
}

// ---------- Webhook CRUD ----------

// webhookDTO is the public representation of a webhook (secret masked).
type webhookDTO struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	URL       string     `json:"url"`
	Secret    string     `json:"secret,omitempty"` // only returned once on create/rotate
	Events    []string   `json:"events"`
	IsActive  bool       `json:"isActive"`
	LastSeen  *time.Time `json:"lastSeen,omitempty"`
	CreatedBy string     `json:"createdBy"`
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// validWebhookEvents is the set of event types subscribers can listen for.
var validWebhookEvents = map[string]bool{
	models.EventRoomCreated:        true,
	models.EventRoomEnded:          true,
	models.EventParticipantJoined:  true,
	models.EventRecordingCompleted: true,
	models.EventWebhookTest:        true,
}

// validateEvents checks that the provided event list is non-empty and contains
// only known event types. Returns an error message suitable for API response.
func validateEvents(events []string) error {
	if len(events) == 0 {
		return fmt.Errorf("at least one event must be specified")
	}
	for _, e := range events {
		if !validWebhookEvents[e] {
			return fmt.Errorf("unknown event type: %q", e)
		}
	}
	return nil
}

// ListWebhooks returns paginated webhook endpoints.
// GET /api/admin/webhooks
//
// @Summary List webhooks
// @Description Get paginated list of webhook endpoints. Superadmin access required.
// @Tags admin
// @Accept json
// @Produce json
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(50)
// @Success 200 {object} map[string]interface{} "{webhooks, total, page, limit}"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 500 {object} ErrorResponse "Failed to list webhooks"
// @Router /admin/webhooks [get]
func (h *AdminHandler) ListWebhooks(c *fiber.Ctx) error {
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	webhooks, total, err := h.webhookRepo.ListPaginated(repository.PaginationParams{Page: page, Limit: limit})
	if err != nil {
		log.Error().Err(err).Msg("Failed to list webhooks")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list webhooks"})
	}
	result := make([]webhookDTO, len(webhooks))
	for i := range webhooks {
		w := &webhooks[i]
		result[i] = webhookDTO{
			ID:        w.ID,
			Name:      w.Name,
			URL:       w.URL,
			Secret:    w.MaskedSecret(),
			Events:    w.Events,
			IsActive:  w.IsActive,
			LastSeen:  w.LastSeen,
			CreatedBy: w.CreatedBy,
			CreatedAt: w.CreatedAt,
			UpdatedAt: w.UpdatedAt,
		}
	}
	return c.JSON(fiber.Map{"webhooks": result, "total": total, "page": page, "limit": limit})
}

type createWebhookRequest struct {
	Name   string   `json:"name"`
	URL    string   `json:"url"`
	Events []string `json:"events"`
}

// CreateWebhook creates a new webhook endpoint.
// POST /api/admin/webhooks
//
// @Summary Create webhook
// @Description Create a new webhook endpoint with a generated secret. Superadmin access required.
// @Tags admin
// @Accept json
// @Produce json
// @Param request body createWebhookRequest true "Webhook configuration"
// @Success 201 {object} webhookDTO "Created webhook with plaintext secret"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 500 {object} ErrorResponse "Failed to create webhook"
// @Router /admin/webhooks [post]
func (h *AdminHandler) CreateWebhook(c *fiber.Ctx) error {
	var req createWebhookRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}
	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{"error": "name is required"})
	}
	if req.URL == "" {
		return c.Status(400).JSON(fiber.Map{"error": "url is required"})
	}

	if err := validateEvents(req.Events); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	// Validate URL
	parsed, err := url.Parse(req.URL)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid URL: must be absolute http:// or https:// URL"})
	}
	if parsed.Scheme != schemeHTTP && parsed.Scheme != schemeHTTPS {
		return c.Status(400).JSON(fiber.Map{"error": "URL scheme must be http:// or https://"})
	}

	// Generate a 32-char crypto-random secret
	secretBytes := make([]byte, 16)
	if _, err := rand.Read(secretBytes); err != nil {
		log.Error().Err(err).Msg("Failed to generate webhook secret")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate secret"})
	}
	secret := hex.EncodeToString(secretBytes)

	userID := c.Locals("userID").(string)
	w := models.Webhook{
		ID:        uuid.New().String(),
		Name:      req.Name,
		URL:       req.URL,
		Secret:    secret,
		Events:    req.Events,
		IsActive:  true,
		CreatedBy: userID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := h.webhookRepo.Create(&w); err != nil {
		log.Error().Err(err).Msg("Failed to create webhook")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create webhook"})
	}

	// Return secret plaintext once
	return c.Status(201).JSON(webhookDTO{
		ID:        w.ID,
		Name:      w.Name,
		URL:       w.URL,
		Secret:    secret,
		Events:    w.Events,
		IsActive:  w.IsActive,
		CreatedBy: w.CreatedBy,
		CreatedAt: w.CreatedAt,
		UpdatedAt: w.UpdatedAt,
	})
}

type updateWebhookRequest struct {
	Name     *string   `json:"name"`
	URL      *string   `json:"url"`
	Events   *[]string `json:"events"`
	IsActive *bool     `json:"isActive"`
}

// UpdateWebhook updates an existing webhook endpoint.
// PUT /api/admin/webhooks/:id
//
// @Summary Update webhook
// @Description Update webhook name, URL, events, or active status. Superadmin access required.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "Webhook ID"
// @Param request body updateWebhookRequest true "Webhook fields to update"
// @Success 200 {object} webhookDTO "Updated webhook"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "Webhook not found"
// @Failure 500 {object} ErrorResponse "Failed to update webhook"
// @Router /admin/webhooks/{id} [put]
func (h *AdminHandler) UpdateWebhook(c *fiber.Ctx) error {
	id := c.Params("id")
	w, err := h.webhookRepo.GetByID(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Webhook not found"})
	}

	var req updateWebhookRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	if req.Name != nil {
		if *req.Name == "" {
			return c.Status(400).JSON(fiber.Map{"error": "name cannot be empty"})
		}
		w.Name = *req.Name
	}
	if req.URL != nil {
		if *req.URL == "" {
			return c.Status(400).JSON(fiber.Map{"error": "url cannot be empty"})
		}
		parsed, err := url.Parse(*req.URL)
		if err != nil || parsed.Scheme == "" || parsed.Host == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid URL"})
		}
		if parsed.Scheme != schemeHTTP && parsed.Scheme != schemeHTTPS {
			return c.Status(400).JSON(fiber.Map{"error": "URL scheme must be http:// or https://"})
		}
		w.URL = *req.URL
	}
	if req.Events != nil {
		if err := validateEvents(*req.Events); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		w.Events = *req.Events
	}
	if req.IsActive != nil {
		w.IsActive = *req.IsActive
	}

	if err := h.webhookRepo.Update(w); err != nil {
		log.Error().Err(err).Str("webhookID", id).Msg("Failed to update webhook")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update webhook"})
	}

	return c.JSON(webhookDTO{
		ID:        w.ID,
		Name:      w.Name,
		URL:       w.URL,
		Secret:    w.MaskedSecret(),
		Events:    w.Events,
		IsActive:  w.IsActive,
		LastSeen:  w.LastSeen,
		CreatedBy: w.CreatedBy,
		CreatedAt: w.CreatedAt,
		UpdatedAt: w.UpdatedAt,
	})
}

// DeleteWebhook deletes a webhook endpoint.
// DELETE /api/admin/webhooks/:id
//
// @Summary Delete webhook
// @Description Delete a webhook endpoint. Superadmin access required.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "Webhook ID"
// @Success 200 {object} map[string]string "{status: deleted}"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "Webhook not found"
// @Failure 500 {object} ErrorResponse "Failed to delete webhook"
// @Router /admin/webhooks/{id} [delete]
func (h *AdminHandler) DeleteWebhook(c *fiber.Ctx) error {
	id := c.Params("id")
	if err := h.webhookRepo.Delete(id); err != nil {
		if err == repository.ErrWebhookNotFound {
			return c.Status(404).JSON(fiber.Map{"error": "Webhook not found"})
		}
		log.Error().Err(err).Str("webhookID", id).Msg("Failed to delete webhook")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete webhook"})
	}
	return c.JSON(fiber.Map{"status": "deleted"})
}

// RotateWebhookSecret generates a new secret for a webhook endpoint.
// POST /api/admin/webhooks/:id/rotate-secret
//
// @Summary Rotate webhook secret
// @Description Generate a new crypto-random secret for a webhook endpoint. Superadmin access required.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "Webhook ID"
// @Success 200 {object} map[string]string "{secret: ...}"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "Webhook not found"
// @Failure 500 {object} ErrorResponse "Failed to rotate secret"
// @Router /admin/webhooks/{id}/rotate-secret [post]
func (h *AdminHandler) RotateWebhookSecret(c *fiber.Ctx) error {
	id := c.Params("id")

	// Generate new 32-char crypto-random secret
	secretBytes := make([]byte, 16)
	if _, err := rand.Read(secretBytes); err != nil {
		log.Error().Err(err).Msg("Failed to generate webhook secret")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate secret"})
	}
	newSecret := hex.EncodeToString(secretBytes)

	if err := h.webhookRepo.UpdateSecret(id, newSecret); err != nil {
		if err == repository.ErrWebhookNotFound {
			return c.Status(404).JSON(fiber.Map{"error": "Webhook not found"})
		}
		log.Error().Err(err).Str("webhookID", id).Msg("Failed to rotate webhook secret")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to rotate secret"})
	}

	// Return new secret plaintext once
	return c.JSON(fiber.Map{"secret": newSecret})
}

// TestWebhook sends a ping event to a webhook endpoint and returns the result.
// POST /api/admin/webhooks/:id/test
//
// @Summary Test webhook
// @Description Send a ping event to test webhook connectivity and latency. Superadmin access required.
// @Tags admin
// @Accept json
// @Produce json
// @Param id path string true "Webhook ID"
// @Success 200 {object} map[string]interface{} "{status, httpStatus, latencyMs, error?}"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 404 {object} ErrorResponse "Webhook not found"
// @Router /admin/webhooks/{id}/test [post]
func (h *AdminHandler) TestWebhook(c *fiber.Ctx) error {
	id := c.Params("id")
	w, err := h.webhookRepo.GetByID(id)
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Webhook not found"})
	}

	start := time.Now()

	// Build ping payload
	envelope := map[string]any{
		"event":     "ping",
		"timestamp": time.Now().UTC().Format(time.RFC3339),
		"data":      map[string]any{},
	}
	body, _ := json.Marshal(envelope)

	// HMAC-SHA256 signature
	mac := hmac.New(sha256.New, []byte(w.Secret))
	mac.Write(body)
	sig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequest(http.MethodPost, w.URL, bytes.NewReader(body))
	if err != nil {
		return c.JSON(fiber.Map{
			"status": "error",
			"error":  fmt.Sprintf("Invalid URL: %v", err),
		})
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Bedrud-Signature", sig)
	req.Header.Set("X-Bedrud-Event", "ping")
	req.Header.Set("X-Bedrud-Timestamp", envelope["timestamp"].(string))

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()

	if err != nil {
		return c.JSON(fiber.Map{
			"status":    "delivery_failed",
			"error":     err.Error(),
			"latencyMs": latency,
		})
	}
	resp.Body.Close()

	success := resp.StatusCode >= 200 && resp.StatusCode < 300
	return c.JSON(fiber.Map{
		"status":     map[bool]string{true: "success", false: "non_2xx"}[success],
		"httpStatus": resp.StatusCode,
		"latencyMs":  latency,
	})
}
