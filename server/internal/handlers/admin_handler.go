package handlers

import (
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/utils"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"strconv"
	"strings"
	"time"

	"bedrud/config"
	"bedrud/internal/lkutil"
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
	validBackends := map[string]bool{"disk": true, "inline": true, "s3": true, "": true}
	if !validBackends[s.ChatUploadBackend] {
		return fmt.Errorf("chatUploadBackend must be disk, inline, or s3")
	}

	// Chat upload sizes
	if s.ChatUploadMaxBytes < 0 {
		return fmt.Errorf("chatUploadMaxBytes cannot be negative")
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
		{s.FrontendURL, "frontendUrl"},
		{s.LiveKitHost, "livekitHost"},
		{s.GoogleRedirectURL, "googleRedirectUrl"},
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
			if parsed.Scheme != "http" && parsed.Scheme != "https" && parsed.Scheme != "ws" && parsed.Scheme != "wss" {
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

	return nil
}

const maskedSecret = "••••••••"

type AdminHandler struct {
	settingsRepo    *repository.SettingsRepository
	inviteTokenRepo *repository.InviteTokenRepository
}

func NewAdminHandler(sr *repository.SettingsRepository, itr *repository.InviteTokenRepository) *AdminHandler {
	return &AdminHandler{settingsRepo: sr, inviteTokenRepo: itr}
}

func (h *AdminHandler) GetSettings(c *fiber.Ctx) error {
	s, err := h.settingsRepo.GetEffectiveSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch settings")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch settings"})
	}
	return c.JSON(maskSettings(s))
}

// GetPublicSettings returns only the fields relevant to anonymous visitors (no auth required).
func (h *AdminHandler) GetPublicSettings(c *fiber.Ctx) error {
	s, err := h.settingsRepo.GetEffectiveSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch public settings")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch settings"})
	}
	return c.JSON(fiber.Map{
		"registrationEnabled":   s.RegistrationEnabled,
		"tokenRegistrationOnly": s.TokenRegistrationOnly,
		"passkeysEnabled":       s.PasskeysEnabled,
		"oauthProviders":        auth.ConfiguredProviders(),
		"chatMaxMessageCount":   s.ChatMaxMessageCount,
		"chatMessageTTLHours":   s.ChatMessageTTLHours,
	})
}

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
			"chatUploadS3AccessKey", "chatUploadS3SecretKey":
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
				}
			}

		// String fields
		case "googleClientId", "googleRedirectUrl",
			"githubClientId", "githubRedirectUrl",
			"twitterClientId", "twitterRedirectUrl",
			"frontendUrl", "serverPort", "serverHost", "serverDomain",
			"serverCertFile", "serverKeyFile", "serverEmail",
			"livekitHost", "livekitApiKey",
			"corsAllowedOrigins", "corsAllowedHeaders", "corsAllowedMethods",
			"chatUploadBackend", "chatUploadDiskDir",
			"chatUploadS3Endpoint", "chatUploadS3Bucket", "chatUploadS3Region",
			"chatUploadS3PublicUrl", "logLevel":
			var s string
			if err := json.Unmarshal(val, &s); err != nil {
				return fmt.Errorf("%s: expected a string, got %s", key, describeJSONType(val))
			}
			switch key {
			case "googleClientId":
				existing.GoogleClientID = s
			case "googleRedirectUrl":
				existing.GoogleRedirectURL = s
			case "githubClientId":
				existing.GithubClientID = s
			case "githubRedirectUrl":
				existing.GithubRedirectURL = s
			case "twitterClientId":
				existing.TwitterClientID = s
			case "twitterRedirectUrl":
				existing.TwitterRedirectURL = s
			case "frontendUrl":
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
			case "livekitHost":
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
			}

		// Bool fields
		case "registrationEnabled", "tokenRegistrationOnly", "passkeysEnabled",
			"serverEnableTls", "serverUseAcme", "behindProxy",
			"livekitExternal", "corsAllowCredentials":
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
			}

		// Int fields
		case "corsMaxAge", "tokenDuration",
			"maxParticipantsLimit", "maxRoomsPerUser",
			"chatMaxMessageCount", "chatMessageTTLHours":
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
	return &cp
}

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

// ValidateSettingsConnectivity runs runtime checks against external services
// using the provided settings subset. Returns per-check status without saving.
func (h *AdminHandler) ValidateSettingsConnectivity(c *fiber.Ctx) error {
	var raw map[string]json.RawMessage
	if err := c.BodyParser(&raw); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Build a partial SystemSettings from the request
	var s models.SystemSettings
	buf, _ := json.Marshal(raw)
	json.Unmarshal(buf, &s) // best-effort; missing fields stay zero

	results := make(map[string]interface{})

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
		results["s3"] = checkS3Connectivity(s)
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
	return checkResult{Status: "error", Message: msg}
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

func checkS3Connectivity(s models.SystemSettings) checkResult {
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
	url := fmt.Sprintf("%s/%s", endpoint, s.ChatUploadS3Bucket)

	req, err := http.NewRequest("HEAD", url, nil)
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

func (h *AdminHandler) DeleteInviteToken(c *fiber.Ctx) error {
	tokenID := c.Params("id")
	if err := h.inviteTokenRepo.Delete(tokenID); err != nil {
		log.Error().Err(err).Str("tokenID", tokenID).Msg("Failed to delete invite token")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete token"})
	}
	return c.JSON(fiber.Map{"status": "success"})
}
