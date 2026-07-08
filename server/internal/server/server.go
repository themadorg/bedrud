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

package server

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/database"
	"bedrud/internal/handlers"
	"bedrud/internal/livekit"
	"bedrud/internal/lkutil"
	"bedrud/internal/middleware"
	"bedrud/internal/models"
	"bedrud/internal/queue"
	"bedrud/internal/repository"
	"bedrud/internal/scheduler"
	"bedrud/internal/services"
	"bedrud/internal/storage"
	"bedrud/internal/utils"
	"context"
	"crypto/tls"
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

	root "bedrud"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/helmet"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"golang.org/x/crypto/acme/autocert"
)

func Run(configPath, version string) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	logLevel, _ := zerolog.ParseLevel(cfg.Logger.Level)
	zerolog.SetGlobalLevel(logLevel)
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stdout, TimeFormat: time.RFC3339})

	if cfg.Auth.JWTSecret == "" {
		return fmt.Errorf("jwtSecret is required: set AUTH_JWT_SECRET env or auth.jwtSecret in config.yaml")
	}
	if len(cfg.Auth.JWTSecret) < 32 {
		log.Warn().Int("length", len(cfg.Auth.JWTSecret)).Msg("jwtSecret is shorter than recommended 32 characters")
	}
	if cfg.Auth.SessionSecret == "" {
		return fmt.Errorf("sessionSecret is required: set AUTH_SESSION_SECRET env or auth.sessionSecret in config.yaml")
	}

	if cfg.Auth.RequireEmailVerification && (cfg.Email.SMTPHost == "" || cfg.Email.SMTPPort == 0) {
		log.Warn().Msg("email verification is enabled but SMTP is not configured — verification emails will be logged, not sent")
	}

	tlsEnabled := cfg.Server.EnableTLS && !cfg.Server.DisableTLS
	if tlsEnabled && !cfg.Server.UseACME {
		certFile := cfg.Server.CertFile
		keyFile := cfg.Server.KeyFile
		if certFile == "" {
			certFile = "/etc/bedrud/cert.pem"
		}
		if keyFile == "" {
			keyFile = "/etc/bedrud/key.pem"
		}
		certInfo, err := utils.ValidateTLSCertPair(certFile, keyFile)
		if err != nil {
			return fmt.Errorf("TLS certificate validation failed: %w", err)
		}
		if certInfo.DaysRemaining <= utils.CertWarnDays {
			log.Warn().Int("daysRemaining", certInfo.DaysRemaining).Str("expires", certInfo.NotAfter.Format(time.RFC3339)).Msg("TLS certificate is expiring soon")
		} else {
			log.Info().Str("subject", certInfo.Subject).Int("daysRemaining", certInfo.DaysRemaining).Str("expires", certInfo.NotAfter.Format(time.RFC3339)).Msg("TLS certificate validated")
		}
	}

	// Start embedded LiveKit unless an external deployment is configured.
	internalHost := strings.ToLower(cfg.LiveKit.InternalHost)
	useInternalLK := !cfg.LiveKit.External &&
		(strings.Contains(internalHost, "localhost") || strings.Contains(internalHost, "127.0.0.1"))
	if useInternalLK {
		log.Info().Msg("➜ Starting internal managed LiveKit server...")
		if len(cfg.LiveKit.APISecret) < 32 {
			return fmt.Errorf(
				"LiveKit API secret is too short (%d chars, need at least 32).\n\n"+
					"Generate a secret:\n"+
					"  openssl rand -hex 32\n\n"+
					"Then set it in config.yaml:\n"+
					"  livekit:\n"+
					"    apiSecret: <generated-secret>\n"+
					"Or via environment:\n"+
					"  LIVEKIT_API_SECRET=<secret> bedrud run",
				len(cfg.LiveKit.APISecret),
			)
		}
		certFile, keyFile := "", ""
		if cfg.Server.EnableTLS && !cfg.Server.DisableTLS {
			certFile = cfg.Server.CertFile
			keyFile = cfg.Server.KeyFile
			if certFile == "" {
				certFile = "/etc/bedrud/cert.pem"
			} else if abs, err := filepath.Abs(certFile); err == nil {
				certFile = abs
			}
			if keyFile == "" {
				keyFile = "/etc/bedrud/key.pem"
			} else if abs, err := filepath.Abs(keyFile); err == nil {
				keyFile = abs
			}
		}
		// Generate LiveKit API key/secret if not set (needed for webhook signing)
		if cfg.LiveKit.APIKey == "" {
			genKey, genSecret, err := utils.GenerateLiveKitKeypair()
			if err != nil {
				log.Error().Err(err).Msg("Failed to generate LiveKit keypair")
			} else {
				cfg.LiveKit.APIKey = genKey
				cfg.LiveKit.APISecret = genSecret
				log.Info().Msg("Generated LiveKit API key/secret (not set in config)")
			}
		}
		nodeIP := livekit.ResolveNodeIP(cfg.LiveKit.NodeIP, cfg.Server.Host)
		if err := livekit.StartInternalServer(context.Background(), cfg.LiveKit.APIKey, cfg.LiveKit.APISecret, 7880, certFile, keyFile, cfg.LiveKit.ConfigPath, nodeIP, cfg.Server.Host, cfg.Server.HTTPPort); err != nil {
			log.Error().Err(err).Msg("Failed to start internal LiveKit server")
		}
	} else if cfg.LiveKit.External {
		log.Info().Str("host", cfg.LiveKit.Host).Msg("➜ Using external LiveKit server")
	}

	auth.InitializeSessionStore(cfg.Auth.SessionSecret, cfg.Server.EnableTLS && !cfg.Server.DisableTLS)
	if err := database.Initialize(&cfg.Database); err != nil {
		return err
	}
	defer func() { _ = database.Close() }()
	if err := database.RunMigrations(); err != nil {
		log.Error().Err(err).Msg("Failed to run database migrations")
	}
	roomRepo := repository.NewRoomRepository(database.GetDB())
	userRepo := repository.NewUserRepository(database.GetDB())
	recordingRepo := repository.NewRecordingRepository(database.GetDB())
	// TODO oncoming feature: recordingStore, scheduler recording cleanup
	// recordingStore := storage.NewRecordingStore(&cfg.Recording, cfg.Chat.Uploads.S3)
	scheduler.Initialize(database.GetDB(), roomRepo, userRepo, recordingRepo, &cfg.LiveKit, &cfg.Server, nil, nil)
	defer scheduler.Stop()
	auth.Init(cfg)

	// Load deactivated users into in-memory ban set for fast middleware checks
	inactiveUsers, _ := userRepo.GetInactiveUserIDs()
	if len(inactiveUsers) > 0 {
		auth.LoadBannedUsersFromDB(inactiveUsers)
	}

	fiberCfg := fiber.Config{
		AppName:   "Bedrud API",
		BodyLimit: 2 * 1024 * 1024,
	}
	// Enable trusted-proxy mode when: explicit trustedProxies list is set,
	// OR behindProxy=true (CDN / nginx in front), OR DisableTLS with a domain.
	if len(cfg.Server.TrustedProxies) > 0 || cfg.Server.BehindProxy {
		fiberCfg.EnableTrustedProxyCheck = true
		if len(cfg.Server.TrustedProxies) > 0 {
			fiberCfg.TrustedProxies = cfg.Server.TrustedProxies
		} else {
			// Trust all proxies when behindProxy=true and no explicit list.
			fiberCfg.TrustedProxies = []string{"0.0.0.0/0"}
		}
		if cfg.Server.ProxyHeader != "" {
			fiberCfg.ProxyHeader = cfg.Server.ProxyHeader
		} else {
			fiberCfg.ProxyHeader = "X-Forwarded-For"
		}
	}
	app := fiber.New(fiberCfg)

	// Warn if rate limiting is active but client IP detection is misconfigured
	// (behind proxy without trusted proxy check). If rate limiter keys on proxy IP,
	// all users behind that proxy share the same rate limit bucket.
	hasRateLimiting := cfg.RateLimit.AuthMaxRequests != nil ||
		cfg.RateLimit.GuestMaxRequests != nil ||
		cfg.RateLimit.APIMaxRequests != nil ||
		cfg.RateLimit.AuthResendMaxRequests != nil
	if hasRateLimiting && !cfg.Server.BehindProxy && len(cfg.Server.TrustedProxies) == 0 {
		log.Warn().Msg(
			"Rate limiting is active but BehindProxy=false and no TrustedProxies set. " +
				"If running behind nginx/Cloudflare, all rate limiters will see the proxy IP " +
				"as the client IP. Set behindProxy: true in config.yaml.",
		)
	}

	// Proxy LiveKit traffic only when using the internal (embedded) server.
	// When livekit.external=true the client connects directly to livekit.host.
	if useInternalLK {
		target, _ := url.Parse("http://127.0.0.1:7880")
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

	app.Use(recover.New())
	app.Use(helmet.New(helmet.Config{
		XSSProtection:      "1; mode=block",
		ContentTypeNosniff: "nosniff",
		XFrameOptions:      "DENY",
		ReferrerPolicy:     "strict-origin-when-cross-origin",
	}))
	corsConfig := cors.Config{
		AllowHeaders:     cfg.Cors.AllowedHeaders,
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowCredentials: cfg.Cors.AllowCredentials,
	}
	if cfg.Cors.AllowCredentials {
		if cfg.Cors.AllowedOrigins == "*" || cfg.Cors.AllowedOrigins == "" {
			log.Error().Msg("CORS: AllowCredentials=true requires explicit AllowedOrigins (not '*'). Refusing to start.")
			return fmt.Errorf("CORS misconfiguration: allowCredentials=true with wildcard origins is insecure")
		}
		corsConfig.AllowOrigins = cfg.Cors.AllowedOrigins
	} else {
		if cfg.Cors.AllowedOrigins == "" || cfg.Cors.AllowedOrigins == "*" {
			corsConfig.AllowOrigins = "*"
		} else {
			corsConfig.AllowOrigins = cfg.Cors.AllowedOrigins
		}
	}
	app.Use(cors.New(corsConfig))

	api := app.Group("/api")

	// Health & readiness
	api.Get("/health", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{"status": "healthy", "time": time.Now().Unix()})
	})
	api.Get("/ready", func(c *fiber.Ctx) error {
		if sqlDB, err := database.GetDB().DB(); err != nil || sqlDB.Ping() != nil {
			return c.Status(503).JSON(fiber.Map{"status": "not_ready", "error": "database unavailable"})
		}
		return c.JSON(fiber.Map{"status": "ready", "time": time.Now().Unix()})
	})
	app.Get("/health", func(c *fiber.Ctx) error { return c.Redirect("/api/health") })
	app.Get("/ready", func(c *fiber.Ctx) error { return c.Redirect("/api/ready") })

	passkeyRepo := repository.NewPasskeyRepository(database.GetDB())
	settingsRepo := repository.NewSettingsRepository(database.GetDB())
	settingsRepo.SetConfig(cfg)
	inviteTokenRepo := repository.NewInviteTokenRepository(database.GetDB())
	prefsRepo := repository.NewUserPreferencesRepository(database.GetDB())
	webhookRepo := repository.NewWebhookRepository(database.GetDB())
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
	uploadStore := storage.NewChatUploadStore(&cfg.Chat.Uploads)
	// TODO oncoming feature: recordingStore
	// recordingStore = storage.NewRecordingStore(&cfg.Recording, cfg.Chat.Uploads.S3)
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

	// Queue worker — runs async jobs for deletion, uploads, etc.
	queueWorker := queue.NewWorker(database.GetDB(), map[string]queue.Handler{
		"user_delete":      queue.NewUserDeleteHandler(cleanupSvc, userRepo, passkeyRepo, prefsRepo, roomRepo),
		"room_delete":      queue.NewRoomDeleteHandler(cleanupSvc, roomRepo),
		"room_suspend":     queue.NewRoomSuspendHandler(cleanupSvc, roomRepo),
		"chat_upload_s3":   queue.NewChatUploadS3Handler(uploadStore, uploadTracker),
		"send_email":       queue.NewSendEmailHandler(&cfg.Email),
		"dispatch_webhook": queue.NewDispatchWebhookHandler(),
		// TODO oncoming feature: process_recording, recording_delete queue handlers
	}, queue.WorkerOptions{
		Interval:    time.Duration(cfg.Queue.PollInterval.Int64()) * time.Millisecond,
		Concurrency: cfg.Queue.Concurrency,
	})
	queueWorker.Start(context.Background())
	defer queueWorker.Stop()

	// Cooldown cache for verification email resends
	cooldownTTL := 2 * time.Minute
	if cfg.Auth.VerificationEmailCooldownMins > 0 {
		cooldownTTL = time.Duration(cfg.Auth.VerificationEmailCooldownMins) * time.Minute
	}
	emailCooldown := handlers.NewCooldownCache(cooldownTTL)

	authService := auth.NewAuthService(userRepo, passkeyRepo)
	verifEventRepo := repository.NewVerificationEventRepository(database.GetDB())
	challengeStore := auth.NewChallengeStore(cfg.Auth.PasskeyChallengeTTL)
	authHandler := handlers.NewAuthHandler(authService, cfg, settingsRepo, inviteTokenRepo, challengeStore, emailCooldown, verifEventRepo)
	roomHandler := handlers.NewRoomHandler(lkClient, &cfg.LiveKit, &cfg.Chat, roomRepo, userRepo, recordingRepo, settingsRepo, webhookRepo, uploadTracker, cleanupSvc)

	api.Post("/auth/register", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.Register)
	api.Post("/auth/login", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.Login)
	api.Post("/auth/guest-login", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.GuestLogin)
	api.Post("/auth/refresh", middleware.AuthRateLimiter(cfg.RateLimit), authHandler.RefreshToken)
	api.Post("/auth/logout", middleware.Protected(), authHandler.Logout)
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
	// api.Get("/rooms/:id/recordings/:rid/wait", ...)
	// api.Get("/rooms/:id/recordings/:rid", ...)
	// api.Delete("/rooms/:id/recordings", ...)
	// api.Delete("/rooms/:id/recordings/:recordingId", ...)

	// Serve disk-backed chat image uploads as static files.
	// Inline (base64) and S3-hosted images are not served from here.
	if err := os.MkdirAll(uploadDir, 0o755); err != nil {
		log.Warn().Err(err).Str("dir", uploadDir).Msg("Could not create chat upload dir")
	}
	// Protected chat upload endpoint with path traversal prevention
	app.Get("/uploads/chat/*", middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo), func(c *fiber.Ctx) error {
		path := c.Params("*")
		if path == "" {
			return c.Status(400).JSON(fiber.Map{"error": "Missing file path"})
		}
		resolved := filepath.Join(uploadDir, path)
		if !strings.HasPrefix(resolved, filepath.Clean(uploadDir)+string(os.PathSeparator)) && resolved != filepath.Clean(uploadDir) {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid path"})
		}
		return c.SendFile(resolved)
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

	// TODO oncoming feature: recording static file serving

	// Admin routes
	usersHandler := handlers.NewUsersHandler(userRepo, roomRepo, passkeyRepo, prefsRepo, cleanupSvc, verifEventRepo)
	adminHandler := handlers.NewAdminHandler(settingsRepo, inviteTokenRepo, webhookRepo, recordingRepo)
	certHandler := handlers.NewCertHandler(cfg)
	overviewHandler := handlers.NewAdminOverviewHandler(roomRepo, userRepo, settingsRepo, &cfg.LiveKit, lkClient, database.GetDB(), time.Now(), version)
	adminGroup := api.Group("/admin",
		middleware.Protected(), middleware.RequireEmailVerified(cfg, userRepo),
		middleware.RequireAccess(models.AccessSuperAdmin),
	)
	adminGroup.Get("/users", usersHandler.ListUsers)
	adminGroup.Get("/users/recent", usersHandler.ListRecentSignups)
	adminGroup.Put("/users/:id/status", usersHandler.UpdateUserStatus)
	adminGroup.Put("/users/:id/accesses", usersHandler.UpdateUserAccesses)
	adminGroup.Post("/users/:id/force-logout", usersHandler.ForceLogout)
	adminGroup.Get("/rooms", roomHandler.AdminListRooms)
	adminGroup.Get("/rooms/events", roomHandler.ListRoomEvents)
	adminGroup.Post("/rooms/:roomId/token", roomHandler.AdminGenerateToken)
	adminGroup.Delete("/rooms/:roomId", roomHandler.AdminCloseRoom)
	adminGroup.Post("/rooms/:roomId/suspend", roomHandler.AdminSuspendRoom)
	adminGroup.Post("/rooms/:roomId/reactivate", roomHandler.AdminReactivateRoom)
	adminGroup.Put("/rooms/:roomId", roomHandler.AdminUpdateRoom)
	adminGroup.Get("/online-count", roomHandler.GetOnlineCount)
	adminGroup.Get("/livekit/stats", roomHandler.AdminLiveKitStats)
	adminGroup.Get("/overview", overviewHandler.GetOverview)
	adminGroup.Get("/users/:id", usersHandler.GetUserDetail)
	adminGroup.Post("/users/:id/verify", usersHandler.AdminVerifyEmail)
	adminGroup.Post("/users/:id/verify/resend", usersHandler.AdminResendVerification)
	adminGroup.Get("/users/:id/sessions", usersHandler.ListUserSessions)
	adminGroup.Delete("/users/:id", usersHandler.DeleteUser)
	adminGroup.Get("/rooms/:roomId/participants", roomHandler.AdminGetRoomParticipants)
	adminGroup.Post("/rooms/:roomId/participants/:identity/kick", roomHandler.AdminKickParticipant)
	adminGroup.Post("/rooms/:roomId/participants/:identity/mute", roomHandler.AdminMuteParticipant)
	adminGroup.Post("/rooms/bulk/suspend", roomHandler.BulkSuspendRooms)
	adminGroup.Post("/rooms/bulk/close", roomHandler.BulkCloseRooms)
	api.Get("/auth/settings", adminHandler.GetPublicSettings)
	api.Get("/cert", certHandler.GetCert)
	adminGroup.Get("/settings", adminHandler.GetSettings)
	adminGroup.Put("/settings", adminHandler.UpdateSettings)
	adminGroup.Post("/settings/send-test-email", adminHandler.SendTestEmail)
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

	app.Use("/", filesystem.New(filesystem.Config{Root: http.FS(root.UI), PathPrefix: "frontend"}))

	// Pre-read both HTML files: index.html has SSR'd homepage content (for SEO),
	// shell.html has the same <head>/scripts but no pre-rendered route markup
	// (avoids flashing the homepage when loading /m/*, /dashboard/*, etc.).
	indexHTML, _ := root.UI.ReadFile("frontend/index.html")
	shellHTML, _ := root.UI.ReadFile("frontend/shell.html")
	if len(shellHTML) == 0 {
		shellHTML = indexHTML // fallback if shell.html hasn't been generated yet
	}

	app.Get("*", func(c *fiber.Ctx) error {
		if strings.HasPrefix(c.Path(), "/api") {
			return c.Next()
		}
		c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
		if c.Path() == "/" {
			return c.Status(http.StatusOK).Send(indexHTML)
		}
		return c.Status(http.StatusOK).Send(shellHTML)
	})

	go func() {
		if cfg.Server.UseACME && cfg.Server.Domain != "" {
			log.Info().Msgf("➜ Enabling Let's Encrypt for domain: %s", cfg.Server.Domain)

			certManager := &autocert.Manager{
				Prompt:     autocert.AcceptTOS,
				HostPolicy: autocert.HostWhitelist(cfg.Server.Domain),
				Cache:      autocert.DirCache("/var/lib/bedrud/certs"),
			}

			// Manager for HTTP-01 challenge on port 80
			go func() {
				log.Info().Msgf("➜ Starting ACME challenge server on %s (bound 0.0.0.0:80)", utils.DisplayAddr("0.0.0.0", "80"))
				if err := http.ListenAndServe(":80", certManager.HTTPHandler(nil)); err != nil {
					log.Error().Err(err).Msg("ACME challenge server failed")
				}
			}()

			tlsConfig := &tls.Config{
				GetCertificate: certManager.GetCertificate,
				MinVersion:     tls.VersionTLS12,
			}

			ln, err := tls.Listen("tcp", ":443", tlsConfig)
			if err != nil {
				log.Error().Err(err).Msg("Failed to listen on :443 for ACME — falling back to plain HTTP")
				// fall through to the plain-HTTP / manual-TLS block below
			} else {
				log.Info().Msgf("➜ Bedrud is running on HTTPS %s (bound 0.0.0.0:443)", utils.DisplayAddr("0.0.0.0", "443"))
				if err := app.Listener(ln); err != nil {
					log.Error().Err(err).Msg("ACME TLS listener failed")
				}
				return
			}
		}

		addr := cfg.Server.Host + ":" + cfg.Server.Port
		tlsEnabled := cfg.Server.EnableTLS && !cfg.Server.DisableTLS
		if tlsEnabled {
			// Start a tiny HTTP app that only redirects to HTTPS. Do not expose API/auth on plain HTTP.
			httpPort := cfg.Server.HTTPPort
			if httpPort == "" {
				httpPort = "80"
			}
			go func() {
				httpAddr := cfg.Server.Host + ":" + httpPort
				redirectApp := fiber.New()
				redirectApp.All("*", func(c *fiber.Ctx) error {
					host := cfg.Server.Domain
					if host == "" {
						host = c.Hostname()
					}
					if cfg.Server.Port != "" && cfg.Server.Port != "443" {
						host += ":" + cfg.Server.Port
					}
					return c.Redirect("https://"+host+c.OriginalURL(), http.StatusPermanentRedirect)
				})
				log.Info().Msgf("➜ Redirecting HTTP %s to HTTPS (bound %s)", utils.DisplayAddr(cfg.Server.Host, httpPort), httpAddr)
				if err := redirectApp.Listen(httpAddr); err != nil {
					log.Debug().Err(err).Msg("HTTP redirect server failed")
				}
			}()
			// Start HTTPS on primary port
			log.Info().Msgf("➜ Bedrud is running on HTTPS %s (bound %s)", utils.DisplayAddr(cfg.Server.Host, cfg.Server.Port), addr)
			if err := app.ListenTLS(addr, cfg.Server.CertFile, cfg.Server.KeyFile); err != nil {
				log.Error().Err(err).Str("addr", addr).Msg("TLS listener failed")
			}
		} else {
			log.Info().Msgf("➜ Bedrud is running on HTTP %s (bound %s)", utils.DisplayAddr(cfg.Server.Host, cfg.Server.Port), addr)
			if err := app.Listen(addr); err != nil {
				log.Error().Err(err).Str("addr", addr).Msg("HTTP listener failed")
			}
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	return app.Shutdown()
}
