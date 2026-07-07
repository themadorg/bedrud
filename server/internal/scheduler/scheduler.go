package scheduler

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/queue"
	"bedrud/internal/repository"
	"bedrud/internal/storage"
	"bedrud/internal/utils"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/go-co-op/gocron"
	lkauth "github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"github.com/rs/zerolog/log"
	"github.com/twitchtv/twirp"
	"gorm.io/gorm"
)

var scheduler *gocron.Scheduler

var (
	certFile  string
	keyFile   string
	certHosts []string
	certMu    sync.Mutex
)

func Initialize(db *gorm.DB, roomRepo *repository.RoomRepository, userRepo *repository.UserRepository, recordingRepo *repository.RecordingRepository, lkCfg *config.LiveKitConfig, serverCfg *config.ServerConfig, recStore storage.RecordingStore, recCfg *config.RecordingConfig) {
	scheduler = gocron.NewScheduler(time.Local)

	certFile = ""
	keyFile = ""
	certHosts = nil
	if serverCfg.EnableTLS && !serverCfg.DisableTLS && !serverCfg.UseACME {
		certFile = serverCfg.CertFile
		keyFile = serverCfg.KeyFile
		if certFile == "" {
			certFile = "/etc/bedrud/cert.pem"
		}
		if keyFile == "" {
			keyFile = "/etc/bedrud/key.pem"
		}
		var hosts []string
		if serverCfg.Domain != "" {
			hosts = append(hosts, serverCfg.Domain)
		}
		if ip := net.ParseIP(serverCfg.Host); ip != nil && !ip.IsLoopback() && !ip.IsUnspecified() {
			hosts = append(hosts, serverCfg.Host)
		}
		if outIP := utils.OutboundIP(); outIP != nil && !outIP.IsLoopback() && !outIP.IsUnspecified() {
			found := false
			for _, h := range hosts {
				if h == outIP.String() {
					found = true
					break
				}
			}
			if !found {
				hosts = append(hosts, outIP.String())
			}
		}
		hosts = append(hosts, "localhost", "127.0.0.1", "::1")
		certHosts = hosts
	}

	apiHost := lkCfg.InternalHost
	if apiHost == "" {
		apiHost = lkCfg.Host
	}
	var httpClient *http.Client
	if lkCfg.SkipTLSVerify && strings.HasPrefix(apiHost, "https") {
		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	} else {
		httpClient = http.DefaultClient
	}
	lkClient := livekit.NewRoomServiceProtobufClient(apiHost, httpClient)

	_, _ = scheduler.Every(1).Minute().Do(func() {
		if roomRepo != nil {
			if err := roomRepo.CleanupExpiredRooms(); err != nil {
				log.Error().Err(err).Msg("Scheduler: failed to clean up expired rooms")
			}
		}
		checkIdleRooms(roomRepo, lkCfg, lkClient)
	})

	// Weekly cleanup of stale guest users (older than 7 days, no active rooms)
	if userRepo != nil {
		_, _ = scheduler.Every(1).Week().At("03:00").Do(func() {
			cutoff := time.Now().Add(-7 * 24 * time.Hour)
			deleted, err := userRepo.DeleteGuestUsers(cutoff)
			if err != nil {
				log.Error().Err(err).Msg("Scheduler: failed to clean up stale guest users")
			} else if deleted > 0 {
				log.Info().Int64("deleted", deleted).Msg("Scheduler: cleaned up stale guest users")
			}
		})
	}

	// Daily cleanup of unverified local/passkey accounts (configurable TTL, default 48h)
	if userRepo != nil {
		_, _ = scheduler.Every(1).Day().At("03:30").Do(func() {
			cfg := config.Get()
			ttl := cfg.Auth.UnverifiedAccountTTLHours
			if ttl <= 0 {
				ttl = 48 // default
			}
			cutoff := time.Now().Add(-time.Duration(ttl) * time.Hour)
			deleted, err := userRepo.DeleteUnverifiedUsers(cutoff)
			if err != nil {
				log.Error().Err(err).Msg("Scheduler: failed to clean up unverified accounts")
			} else if deleted > 0 {
				log.Info().Int64("deleted", deleted).Msg("Scheduler: cleaned up unverified accounts")
			}
		})
	}

	// Periodic cleanup of expired blocked refresh tokens
	_, _ = scheduler.Every(1).Hour().Do(func() {
		if userRepo != nil {
			if err := userRepo.CleanupBlockedTokens(); err != nil {
				log.Error().Err(err).Msg("Scheduler: failed to clean up blocked tokens")
			} else {
				log.Info().Msg("Scheduler: cleaned up expired blocked tokens")
			}
		}
	})

	// Periodic pruning of in-memory revoked access token set
	_, _ = scheduler.Every(1).Hour().Do(func() {
		auth.PruneRevokedTokens()
	})

	// Queue cleanup — done jobs older than 7 days, failed jobs older than 30 days
	_, _ = scheduler.Every(1).Day().At("03:00").Do(func() {
		queue.CleanupJobs(db, 7*24*time.Hour)
		queue.CleanupFailedJobs(db, 30*24*time.Hour)
	})

	// TODO oncoming feature: stale recording cleanup
	// Daily cleanup of stale (failed + pending + started) recordings older than 7 days
	if recordingRepo != nil {
		_, _ = scheduler.Every(1).Day().At("03:00").Do(func() {
			cutoff := time.Now().Add(-7 * 24 * time.Hour)
			if err := recordingRepo.DeleteStaleRecordings(cutoff); err != nil {
				log.Error().Err(err).Msg("Scheduler: failed to clean up stale recordings")
			}
		})
	}

	if certFile != "" {
		_, _ = scheduler.Every(1).Day().At("09:00").Do(func() {
			checkCertExpiry()
		})
	}

	// TODO oncoming feature: recording retention cleanup
	if recCfg != nil && recCfg.RetentionHours > 0 && recStore != nil {
		interval := recCfg.CleanupIntervalHours
		if interval <= 0 {
			interval = 24
		}
		_, _ = scheduler.Every(interval).Hour().Do(func() {
			cutoff := time.Now().Add(-time.Duration(recCfg.RetentionHours) * time.Hour)

			recordings, err := recordingRepo.FindExpiredOnArchivedRooms(cutoff)
			if err != nil {
				log.Error().Err(err).Msg("Recording retention: failed to query expired")
				return
			}
			if len(recordings) == 0 {
				return
			}

			deleted := 0
			for _, rec := range recordings {
				if rec.FileURL != "" {
					key := storage.ExtractStorageKey(rec.FileURL)
					if delErr := recStore.Delete(context.Background(), key); delErr != nil {
						log.Warn().Err(delErr).Str("recordingID", rec.ID).
							Msg("Recording retention: file delete failed")
					}
				}
				if err := recordingRepo.DeleteRecording(rec.ID); err != nil {
					log.Warn().Err(err).Str("recordingID", rec.ID).
						Msg("Recording retention: DB delete failed")
					continue
				}
				deleted++
			}

			// Purge empty archived rooms
			emptyRooms, err := roomRepo.FindArchivedRoomsNoRecordings()
			if err == nil {
				for _, room := range emptyRooms {
					if err := roomRepo.HardDeleteRoom(room.ID); err != nil {
						log.Warn().Err(err).Str("roomID", room.ID).
							Msg("Recording retention: failed to purge empty archived room")
					}
				}
				if len(emptyRooms) > 0 {
					log.Info().Int("count", len(emptyRooms)).
						Msg("Recording retention: purged empty archived rooms")
				}
			}

			log.Info().Int("deleted", deleted).
				Msg("Recording retention cleanup complete")
		})
	} else {
		log.Info().Msg("Recording retention disabled (retentionHours=0 or no store)")
	}

	scheduler.StartAsync()
}

// Stop gracefully shuts down the scheduler.
func Stop() {
	if scheduler != nil {
		scheduler.Stop()
	}
}

// checkIdleRooms marks active DB rooms as idle when they have 0 participants in LiveKit.
// Rooms created within the last 5 minutes are skipped to avoid false positives.
func checkIdleRooms(roomRepo *repository.RoomRepository, cfg *config.LiveKitConfig, client livekit.RoomService) {
	if roomRepo == nil {
		return
	}
	rooms, err := roomRepo.GetAllActiveRoomsWithLimit(1000)
	if err != nil || len(rooms) == 0 {
		return
	}

	at := lkauth.NewAccessToken(cfg.APIKey, cfg.APISecret)
	at.AddGrant(&lkauth.VideoGrant{RoomList: true}) //nolint:staticcheck // AddGrant is deprecated but VideoGrant field is not available in this version of the protocol SDK
	token, err := at.ToJWT()
	if err != nil {
		log.Error().Err(err).Msg("Scheduler: failed to generate LiveKit token")
		return
	}
	ctx, err := twirp.WithHTTPRequestHeaders(context.Background(), http.Header{
		"Authorization": []string{"Bearer " + token},
	})
	if err != nil {
		log.Error().Err(err).Msg("Scheduler: failed to set LiveKit auth headers")
		return
	}

	resp, err := client.ListRooms(ctx, &livekit.ListRoomsRequest{})
	if err != nil {
		log.Warn().Err(err).Msg("Scheduler: failed to list LiveKit rooms")
		return
	}

	// Build map of room name -> participant count from LiveKit
	lkRooms := make(map[string]uint32, len(resp.Rooms))
	for _, r := range resp.Rooms {
		lkRooms[r.Name] = r.NumParticipants
	}

	grace := 5 * time.Minute
	for i := range rooms {
		room := &rooms[i]
		if time.Since(room.CreatedAt) < grace {
			continue
		}
		if room.Settings.IsPersistent {
			log.Debug().Str("room", room.Name).Msg("Skipping idle check for persistent room")
			continue
		}
		count, exists := lkRooms[room.Name]
		if !exists || count == 0 {
			if err := roomRepo.SetRoomIdle(room.ID); err != nil {
				log.Error().Err(err).Str("room", room.Name).Msg("Scheduler: failed to set room idle")
			} else {
				// Re-check LK: someone may have joined between our ListRooms snapshot and now
				recheck, recheckErr := client.ListParticipants(ctx, &livekit.ListParticipantsRequest{Room: room.Name})
				if recheckErr == nil && len(recheck.Participants) > 0 {
					// Someone joined — reactivate the room
					if err := roomRepo.UpdateRoom(&models.Room{ID: room.ID, IsActive: true}); err != nil {
						log.Warn().Err(err).Str("room", room.Name).Msg("Scheduler: failed to reactivate room")
					} else {
						log.Info().Int("participants", len(recheck.Participants)).Str("room", room.Name).Msg("Room reactivated (participant joined during idle check)")
						continue
					}
				}
				log.Info().Str("room", room.Name).Msg("Room set to idle (no participants)")
				if err := roomRepo.DeactivateRoomParticipants(room.ID); err != nil {
					log.Warn().Err(err).Str("room", room.Name).Msg("Scheduler: failed to deactivate participants")
				}
			}
		}
	}
}

func checkCertExpiry() {
	if certFile == "" || keyFile == "" {
		return
	}

	data, err := os.ReadFile(certFile)
	if err != nil {
		log.Error().Err(err).Str("path", certFile).Msg("Scheduler: cannot read TLS certificate")
		return
	}

	block, _ := pem.Decode(data)
	if block == nil {
		log.Error().Str("path", certFile).Msg("Scheduler: failed to decode TLS certificate PEM")
		return
	}

	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		log.Error().Err(err).Str("path", certFile).Msg("Scheduler: failed to parse TLS certificate")
		return
	}

	daysRemaining := int((time.Until(cert.NotAfter).Hours() + 23) / 24)
	if daysRemaining <= 0 {
		log.Error().Int("daysRemaining", daysRemaining).Str("expires", cert.NotAfter.Format(time.RFC3339)).Msg("TLS certificate has EXPIRED — attempting renewal")
		renewSelfSignedCert()
	} else if daysRemaining <= utils.CertWarnDays {
		log.Warn().Int("daysRemaining", daysRemaining).Str("expires", cert.NotAfter.Format(time.RFC3339)).Msg("TLS certificate is expiring soon — attempting renewal")
		renewSelfSignedCert()
	}
}

func renewSelfSignedCert() {
	if certFile == "" || keyFile == "" || len(certHosts) == 0 {
		return
	}

	certMu.Lock()
	defer certMu.Unlock()

	hosts := make([]string, len(certHosts))
	copy(hosts, certHosts)
	if outIP := utils.OutboundIP(); outIP != nil && !outIP.IsLoopback() && !outIP.IsUnspecified() {
		found := false
		for _, h := range hosts {
			if h == outIP.String() {
				found = true
				break
			}
		}
		if !found {
			hosts = append(hosts, outIP.String())
		}
	}

	if err := utils.RenewSelfSignedCert(certFile, keyFile, hosts...); err != nil {
		log.Error().Err(err).Str("certFile", certFile).Msg("Scheduler: failed to renew self-signed TLS certificate")
		return
	}
	certHosts = hosts
	log.Info().Strs("hosts", certHosts).Msg("Scheduler: self-signed TLS certificate renewed successfully")
}
