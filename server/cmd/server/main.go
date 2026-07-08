/*
 * Copyright 2026 Bedrud Contributors
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package main

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/database"
	"bedrud/internal/handlers"
	"bedrud/internal/lkutil"
	"bedrud/internal/middleware"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/scheduler"
	"bedrud/internal/services"
	"bedrud/internal/storage"
	"bedrud/internal/utils"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	_ "bedrud/docs"

	root "bedrud"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/gofiber/swagger"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// @title           Bedrud Backend API
// @version         1.0
// @description     This is a Bedrud Backend API server.
// @termsOfService  https://bedrud.org/en/terms/

// @contact.name   Bedrud API Support
// @contact.url    https://bedrud.org/en/contact
// @contact.email  support@bedrud.org

// @license.name  Apache 2.0
// @license.url   https://github.com/themadorg/bedrud/blob/master/LICENSE

// @host      localhost:8090
// @BasePath  /api

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter the token with the `Bearer ` prefix, e.g. "Bearer abcde12345"

func init() {
	// Load configuration
	configPath := os.Getenv("CONFIG_PATH")
	if configPath == "" {
		configPath = "config.yaml"
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to load configuration")
	}

	// Configure zerolog based on config
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logLevel, err := zerolog.ParseLevel(cfg.Logger.Level)
	if err != nil {
		logLevel = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(logLevel)

	output := os.Stdout
	if cfg.Logger.OutputPath != "" {
		file, err := utils.SafeOpenAppend(cfg.Logger.OutputPath, 0o644)
		if err == nil {
			output = file
		} else {
			log.Warn().Err(err).Str("path", cfg.Logger.OutputPath).Msg("Failed to create log file, falling back to stdout")
		}
	}

	log.Logger = log.Output(zerolog.ConsoleWriter{
		Out:        output,
		TimeFormat: time.RFC3339,
	})

	if cfg.Auth.JWTSecret == "" {
		log.Fatal().Msg("jwtSecret is required: set AUTH_JWT_SECRET env or auth.jwtSecret in config.yaml")
	}
	if len(cfg.Auth.JWTSecret) < 32 {
		log.Warn().Int("length", len(cfg.Auth.JWTSecret)).Msg("jwtSecret is shorter than recommended 32 characters")
	}
	if cfg.Auth.SessionSecret == "" {
		log.Fatal().Msg("sessionSecret is required: set AUTH_SESSION_SECRET env or auth.sessionSecret in config.yaml")
	}
}

func main() {
	if err := run(); err != nil {
		log.Error().Err(err).Msg("Application failed")
		os.Exit(1)
	}
}

func run() error {
	cfg := config.Get()

	// Initialize session store first
	auth.InitializeSessionStore(cfg.Auth.SessionSecret, cfg.Server.EnableTLS)

	// Initialize database connection
	if err := database.Initialize(&cfg.Database); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	defer func() { _ = database.Close() }()

	// Run database migrations after database initialization
	if err := database.RunMigrations(); err != nil {
		return fmt.Errorf("failed to run database migrations: %w", err)
	}

	// Scheduler is initialized after repositories are set up (see below)

	// Initialize Goth providers (after session store is initialized)
	auth.Init(cfg)

	// Create new Fiber instance
	app := fiber.New(fiber.Config{
		AppName:      "Bedrud API",
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout.Int()) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout.Int()) * time.Second,
		BodyLimit:    2 * 1024 * 1024,
		// Enable custom error handling
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			log.Error().Err(err).
				Str("path", c.Path()).
				Str("ip", c.IP()).
				Msg("Error handling request")

			// Default 500 status code
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}

			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	// Proxy LiveKit traffic if we are using internal host
	if strings.Contains(strings.ToLower(cfg.LiveKit.InternalHost), "127.0.0.1") ||
		strings.Contains(strings.ToLower(cfg.LiveKit.InternalHost), "localhost") {
		lkProxy := cfg.LiveKit.InternalHost
		if lkProxy == "" {
			lkProxy = "http://127.0.0.1:7880"
		}
		target, _ := url.Parse(lkProxy)
		rp := httputil.NewSingleHostReverseProxy(target)

		// Custom director to handle path stripping and logging
		oldDirector := rp.Director
		rp.Director = func(req *http.Request) {
			oldDirector(req)
			originalPath := req.URL.Path
			req.URL.Path = strings.TrimPrefix(originalPath, "/livekit")
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
			req.Host = target.Host
			log.Debug().Str("original", originalPath).Str("proxied", req.URL.Path).Msg("Proxying LiveKit request (WS supported)")
		}

		app.Use("/livekit", adaptor.HTTPHandler(rp))
	}

	// ===============================
	// Repositories
	// ===============================
	userRepo := repository.NewUserRepository(database.GetDB())
	passkeyRepo := repository.NewPasskeyRepository(database.GetDB())
	roomRepo := repository.NewRoomRepository(database.GetDB())
	settingsRepo := repository.NewSettingsRepository(database.GetDB())
	settingsRepo.SetConfig(cfg)
	if effective, err := settingsRepo.GetEffectiveSettings(); err == nil {
		auth.ReloadProviders(effective)
	}
	inviteTokenRepo := repository.NewInviteTokenRepository(database.GetDB())
	webhookRepo := repository.NewWebhookRepository(database.GetDB())
	recordingRepo := repository.NewRecordingRepository(database.GetDB())
	// TODO oncoming feature: recordingStore, scheduler recording cleanup
	// recordingStore := storage.NewRecordingStore(&cfg.Recording, cfg.Chat.Uploads.S3)

	scheduler.Initialize(database.GetDB(), roomRepo, userRepo, recordingRepo, &cfg.LiveKit, &cfg.Server, nil, nil)
	defer scheduler.Stop()

	// ===============================
	// Services
	// ===============================
	authService := auth.NewAuthService(userRepo, passkeyRepo)

	// ===============================
	// Middleware
	// ===============================
	app.Use(recover.New())
	app.Use(helmet.New(helmet.Config{
		XSSProtection:      "1; mode=block",
		ContentTypeNosniff: "nosniff",
		XFrameOptions:      "DENY",
		ReferrerPolicy:     "strict-origin-when-cross-origin",
	}))
	app.Use(cors.New(cors.Config{
		AllowOrigins:     cfg.Cors.AllowedOrigins,
		AllowHeaders:     cfg.Cors.AllowedHeaders,
		AllowMethods:     cfg.Cors.AllowedMethods,
		AllowCredentials: cfg.Cors.AllowCredentials,
		ExposeHeaders:    cfg.Cors.ExposeHeaders,
		MaxAge:           cfg.Cors.MaxAge.Int(),
	}))

	// ===============================
	// Group all API routes under /api
	// ===============================
	api := app.Group("/api")

	api.Get("/health", healthCheck)
	api.Get("/ready", readinessCheck)

	if os.Getenv("DISABLE_API_DOCS") == "" {
		api.Get("/swagger/*", swagger.New(swagger.Config{
			URL:          "/api/swagger/doc.json",
			DeepLinking:  true,
			DocExpansion: "list",
		}))

		api.Get("/scalar", func(c *fiber.Ctx) error {
			c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
			return c.SendString(`<!doctype html>
<html>
<head>
  <title>Bedrud API — Scalar</title>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width, initial-scale=1" />
</head>
<body>
  <script id="api-reference" data-url="/api/swagger/doc.json"></script>
  <script src="https://cdn.jsdelivr.net/npm/@scalar/api-reference"></script>
</body>
</html>`)
		})
	}

	// ------------------------------
	// Cooldown cache for verification email resends
	cooldownTTL := 2 * time.Minute
	if cfg.Auth.VerificationEmailCooldownMins > 0 {
		cooldownTTL = time.Duration(cfg.Auth.VerificationEmailCooldownMins) * time.Minute
	}
	emailCooldown := handlers.NewCooldownCache(cooldownTTL)
	verifEventRepo := repository.NewVerificationEventRepository(database.GetDB())

	challengeStore := auth.NewChallengeStore(cfg.Auth.PasskeyChallengeTTL)
	authHandler := handlers.NewAuthHandler(authService, cfg, settingsRepo, inviteTokenRepo, challengeStore, emailCooldown, verifEventRepo)
	api.Post("/auth/register", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.Register)
	api.Post("/auth/login", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.Login)
	api.Post("/auth/guest-login", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.GuestLogin)
	api.Post("/auth/refresh", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.RefreshToken)
	api.Post("/auth/logout", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), authHandler.Logout)
	api.Get("/auth/me", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), authHandler.GetMe)
	api.Put("/auth/me", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), authHandler.UpdateProfile)
	api.Post("/auth/me/avatar", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), authHandler.UploadAvatar)
	api.Delete("/auth/me/avatar", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), authHandler.DeleteAvatar)
	api.Put("/auth/password", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), authHandler.ChangePassword)
	api.Post("/auth/verify", authHandler.VerifyEmail)
	api.Get("/auth/verify/status", middleware.Protected(), authHandler.CheckVerificationStatus)
	api.Post("/auth/verify/resend", middleware.ResendRateLimiter(cfg.RateLimit), authHandler.ResendVerification)
	api.Post("/auth/forgot-password", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.ForgotPassword)
	api.Post("/auth/reset-password", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.ResetPassword)
	api.Get("/auth/:provider/login", middleware.AuthRateLimiter(cfg.RateLimit), handlers.BeginAuthHandler)
	api.Get("/auth/:provider/callback", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.CallbackHandler)

	prefsRepo := repository.NewUserPreferencesRepository(database.GetDB())
	preferencesHandler := handlers.NewPreferencesHandler(prefsRepo)
	api.Get("/auth/preferences", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), preferencesHandler.GetPreferences)
	api.Put("/auth/preferences", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), preferencesHandler.UpdatePreferences)

	// Passkey routes
	api.Post("/auth/passkey/register/begin", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), authHandler.PasskeyRegisterBegin)
	api.Post("/auth/passkey/register/finish", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), authHandler.PasskeyRegisterFinish)
	api.Post("/auth/passkey/login/begin", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.PasskeyLoginBegin)
	api.Post("/auth/passkey/login/finish", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.PasskeyLoginFinish)
	api.Post("/auth/passkey/signup/begin", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.PasskeySignupBegin)
	api.Post("/auth/passkey/signup/finish", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.PasskeySignupFinish)

	// Initialize handlers
	uploadDir := cfg.Chat.Uploads.DiskDir
	if uploadDir == "" {
		uploadDir = "./data/uploads/chat"
	}
	var s3Deleter storage.ObjectDeleter
	if cfg.Chat.Uploads.Backend == "s3" &&
		cfg.Chat.Uploads.S3.Endpoint != "" &&
		cfg.Chat.Uploads.S3.Bucket != "" &&
		cfg.Chat.Uploads.S3.AccessKey != "" {
		s3Deleter = storage.NewS3Deleter(cfg.Chat.Uploads.S3)
	}
	uploadTracker := storage.NewChatUploadTracker(database.GetDB(), uploadDir, s3Deleter)
	lkClient := lkutil.NewClient(&cfg.LiveKit)
	// TODO oncoming feature: egress client, recording service, recording handler
	// egressClient, err := lkutil.NewEgressClient(&cfg.LiveKit)
	// if err != nil {
	// 	log.Warn().Err(err).Msg("Failed to create LiveKit egress client — recording disabled")
	// }
	// recordingService := services.NewRecordingService(settingsRepo, recordingRepo, roomRepo, egressClient, cfg.LiveKit.APIKey, cfg.LiveKit.APISecret)
	// recordingHandler := handlers.NewRecordingHandler(roomRepo, recordingService, recordingRepo, recordingStore)
	cleanupSvc := services.NewRoomCleanupService(roomRepo, nil, lkClient, nil, cfg.LiveKit.APIKey, cfg.LiveKit.APISecret, uploadTracker)
	roomHandler := handlers.NewRoomHandler(lkClient, &cfg.LiveKit, &cfg.Chat, roomRepo, userRepo, recordingRepo, settingsRepo, webhookRepo, uploadTracker, cleanupSvc)

	// Room routes
	api.Post("/room/create", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), middleware.APIRateLimiter(cfg.RateLimit), roomHandler.CreateRoom)
	api.Post("/room/join", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.JoinRoom)
	api.Post("/room/guest-join", middleware.GuestRateLimiter(cfg.RateLimit), roomHandler.GuestJoinRoom)
	api.Post("/room/refresh-token", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), middleware.APIRateLimiter(cfg.RateLimit), roomHandler.RefreshLiveKitToken)
	api.Get("/room/list", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.ListRooms)
	api.Get("/room/archived", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.ListArchivedRooms)
	api.Post("/room/:roomId/kick/:identity", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.KickParticipant)
	api.Post("/room/:roomId/mute/:identity", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.MuteParticipant)
	api.Post("/room/:roomId/ban/:identity", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.BanParticipant)
	api.Post("/room/:roomId/video/:identity/off", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.DisableParticipantVideo)
	api.Post("/room/:roomId/promote/:identity", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.PromoteParticipant)
	api.Post("/room/:roomId/demote/:identity", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.DemoteParticipant)
	api.Post("/room/:roomId/chat/:identity/block", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.BlockChat)
	api.Post("/room/:roomId/deafen/:identity", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.DeafenParticipant)
	api.Post("/room/:roomId/undeafen/:identity", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.UndeafenParticipant)
	api.Post("/room/:roomId/ask/:identity/:action", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.AskParticipantAction)
	api.Post("/room/:roomId/spotlight/:identity", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.SpotlightParticipant)
	api.Post("/room/:roomId/screenshare/:identity/stop", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.StopScreenShare)
	api.Get("/room/:roomId/presence", middleware.APIRateLimiter(cfg.RateLimit), roomHandler.GetRoomPresence)
	api.Get("/room/:roomId/participant/:identity/info", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.GetParticipantInfo)
	api.Get("/room/:roomId/participant/:identity/profile", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.GetParticipantProfile)
	api.Post("/room/:roomId/stage/:identity/bring", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.BringToStage)
	api.Post("/room/:roomId/stage/:identity/remove", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.RemoveFromStage)
	api.Put("/room/:roomId/settings", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.UpdateSettings)
	api.Delete("/room/:roomId", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), roomHandler.DeleteRoom)
	api.Post("/room/:roomId/chat/upload", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), middleware.APIRateLimiter(cfg.RateLimit), roomHandler.UploadChatImage)

	// TODO oncoming feature: recording routes
	// api.Post("/rooms/:id/recording/start", ...)
	// api.Post("/rooms/:id/recording/stop", ...)
	// api.Get("/rooms/:id/recordings", ...)
	// api.Get("/rooms/:id/recordings/:rid", ...)

	// Serve disk-backed chat image uploads.
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		log.Warn().Err(err).Str("dir", uploadDir).Msg("Could not create chat upload dir")
	}
	app.Get("/uploads/chat/*", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), func(c *fiber.Ctx) error {
		path := c.Params("*")
		if strings.Contains(path, "..") {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid path"})
		}
		return c.SendFile(filepath.Join(uploadDir, path))
	})

	avatarDir := storage.AvatarDir()
	if err := os.MkdirAll(avatarDir, 0o755); err != nil {
		log.Warn().Err(err).Str("dir", avatarDir).Msg("Could not create avatar upload dir")
	}
	app.Get("/uploads/avatars/*", func(c *fiber.Ctx) error {
		path := c.Params("*")
		if path == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Missing file path"})
		}
		resolved, err := storage.ResolveAvatarFile(path)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid path"})
		}
		return c.SendFile(resolved)
	})

	// Initialize handlers
	usersHandler := handlers.NewUsersHandler(userRepo, roomRepo, passkeyRepo, prefsRepo, cleanupSvc, verifEventRepo)
	adminHandler := handlers.NewAdminHandler(settingsRepo, inviteTokenRepo, webhookRepo, recordingRepo)
	certHandler := handlers.NewCertHandler(cfg)
	overviewHandler := handlers.NewAdminOverviewHandler(roomRepo, userRepo, settingsRepo, &cfg.LiveKit, lkClient, database.GetDB(), time.Now(), "dev")

	// Admin routes
	adminGroup := api.Group("/admin",
		middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo),
		middleware.RequireEmailVerified(cfg, userRepo),
		middleware.RequireAccess(models.AccessSuperAdmin),
	)
	adminGroup.Get("/users", usersHandler.ListUsers)
	adminGroup.Get("/users/recent", usersHandler.ListRecentSignups)
	adminGroup.Post("/users/bulk/ban", usersHandler.BulkBanUsers)
	adminGroup.Post("/users/bulk/promote", usersHandler.BulkPromoteUsers)
	adminGroup.Post("/users/bulk/delete", usersHandler.BulkDeleteUsers)
	adminGroup.Put("/users/:id/status", usersHandler.UpdateUserStatus)
	adminGroup.Put("/users/:id/accesses", usersHandler.UpdateUserAccesses)
	adminGroup.Post("/users/:id/force-logout", usersHandler.ForceLogout)
	adminGroup.Put("/users/:id/password", usersHandler.SetUserPassword)
	adminGroup.Get("/stats", roomHandler.GetAdminStats)
	adminGroup.Get("/overview", overviewHandler.GetOverview)
	adminGroup.Get("/rooms", roomHandler.AdminListRooms)
	adminGroup.Get("/rooms/events", roomHandler.ListRoomEvents)
	adminGroup.Post("/rooms/bulk/suspend", roomHandler.BulkSuspendRooms)
	adminGroup.Post("/rooms/bulk/close", roomHandler.BulkCloseRooms)
	adminGroup.Post("/rooms/:roomId/token", roomHandler.AdminGenerateToken)
	adminGroup.Delete("/rooms/:roomId", roomHandler.AdminCloseRoom)
	adminGroup.Post("/rooms/:roomId/suspend", roomHandler.AdminSuspendRoom)
	adminGroup.Post("/rooms/:roomId/reactivate", roomHandler.AdminReactivateRoom)
	adminGroup.Put("/rooms/:roomId", roomHandler.AdminUpdateRoom)
	adminGroup.Get("/online-count", roomHandler.GetOnlineCount)
	adminGroup.Get("/livekit/stats", roomHandler.AdminLiveKitStats)
	adminGroup.Get("/users/:id", usersHandler.GetUserDetail)
	adminGroup.Post("/users/:id/verify", usersHandler.AdminVerifyEmail)
	adminGroup.Post("/users/:id/verify/resend", usersHandler.AdminResendVerification)
	adminGroup.Delete("/users/:id", usersHandler.DeleteUser)
	adminGroup.Get("/rooms/:roomId/participants", roomHandler.AdminGetRoomParticipants)
	adminGroup.Post("/rooms/:roomId/participants/:identity/kick", roomHandler.AdminKickParticipant)
	adminGroup.Post("/rooms/:roomId/participants/:identity/mute", roomHandler.AdminMuteParticipant)
	api.Get("/auth/settings", adminHandler.GetPublicSettings)
	api.Get("/cert", certHandler.GetCert)
	adminGroup.Get("/settings", adminHandler.GetSettings)
	adminGroup.Put("/settings", adminHandler.UpdateSettings)
	adminGroup.Post("/settings/validate", adminHandler.ValidateSettingsConnectivity)
	adminGroup.Get("/invite-tokens", adminHandler.ListInviteTokens)
	adminGroup.Post("/invite-tokens", adminHandler.CreateInviteToken)
	adminGroup.Delete("/invite-tokens/:id", adminHandler.DeleteInviteToken)
	adminGroup.Get("/webhooks", adminHandler.ListWebhooks)
	adminGroup.Post("/webhooks", adminHandler.CreateWebhook)
	adminGroup.Put("/webhooks/:id", adminHandler.UpdateWebhook)
	adminGroup.Delete("/webhooks/:id", adminHandler.DeleteWebhook)
	adminGroup.Post("/webhooks/:id/rotate-secret", adminHandler.RotateWebhookSecret)
	adminGroup.Post("/webhooks/:id/test", adminHandler.TestWebhook)
	// TODO oncoming feature: admin recording routes
	// adminGroup.Get("/recordings", ...)
	// adminGroup.Post("/recordings/bulk/delete", ...)
	adminGroup.Get("/cert-info", certHandler.GetCertInfo)

	queueHandler := handlers.NewAdminQueueHandler(database.GetDB())
	adminGroup.Get("/queue", queueHandler.GetQueueStats)

	// LiveKit webhook (no app auth middleware — uses LiveKit's own JWT signature)
	livekitWebhookHandler := handlers.NewLiveKitWebhookHandler(&cfg.LiveKit, roomRepo, recordingRepo, webhookRepo, database.GetDB())
	api.Post("/livekit/webhook", livekitWebhookHandler.Handle)

	// ------------------------------
	// Serve static files
	app.Static("/static", "./static")

	// ------------------------------
	// Serve frontend application using embedded files
	app.Use("/", filesystem.New(filesystem.Config{
		Root:       http.FS(root.UI),
		PathPrefix: "frontend",
		Browse:     false,
	}))

	// ------------------------------
	// For backward compatibility - these will be removed later
	app.Get("/health", func(c *fiber.Ctx) error { return c.Redirect("/api/health") })
	app.Get("/ready", func(c *fiber.Ctx) error { return c.Redirect("/api/ready") })

	// Serve SPA - handle routes for SPA by serving index.html for all non-API routes
	app.Get("*", func(c *fiber.Ctx) error {
		// Skip if path starts with /api or /static
		path := c.Path()
		if strings.HasPrefix(path, "/api") || strings.HasPrefix(path, "/static") {
			return c.Next()
		}
		// Serve index.html from embedded files
		file, err := root.UI.ReadFile("frontend/index.html")
		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("index.html not found")
		}
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		return c.Send(file)
	})

	// Start server in a goroutine
	serverAddr := cfg.Server.Host + ":" + cfg.Server.Port
	go func() {
		log.Info().Msgf("➜ Bedrud is running on HTTP %s (bound %s)", utils.DisplayAddr(cfg.Server.Host, cfg.Server.Port), serverAddr)
		if err := app.Listen(serverAddr); err != nil {
			log.Error().Err(err).Msgf("Failed to start server on %s (bound %s)", utils.DisplayAddr(cfg.Server.Host, cfg.Server.Port), serverAddr)
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info().Msg("Shutting down server...")
	return app.Shutdown()
}

// @Summary Health check endpoint
// @Description Get the health status of the service
// @Tags health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /health [get]
// @Router /api/health [get]
// Health check handler
func healthCheck(c *fiber.Ctx) error {
	log.Info().
		Str("path", c.Path()).
		Str("ip", c.IP()).
		Msg("Health check request received")

	return c.JSON(fiber.Map{
		"status": "healthy",
		"time":   time.Now().Unix(),
	})
}

// @Summary Readiness check endpoint
// @Description Get the readiness status of the service
// @Tags health
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /ready [get]
// @Router /api/ready [get]
// Readiness check handler
func readinessCheck(c *fiber.Ctx) error {
	log.Info().
		Str("path", c.Path()).
		Str("ip", c.IP()).
		Msg("Readiness check request received")

	return c.JSON(fiber.Map{
		"status": "ready",
		"time":   time.Now().Unix(),
	})
}
