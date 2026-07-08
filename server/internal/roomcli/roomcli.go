package roomcli

import (
	"bedrud/config"
	"bedrud/internal/clioutput"
	"bedrud/internal/database"
	"bedrud/internal/lkutil"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/services"
	"bedrud/internal/storage"
	"context"
	"fmt"
	"time"

	lkauth "github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
)

func ListRooms(configPath string, page, pageSize int, activeOnly bool) error {
	return withRepo(configPath, func(cfg *config.Config, roomRepo *repository.RoomRepository) error {
		if page < 1 {
			page = 1
		}
		if pageSize <= 0 {
			pageSize = 50
		}
		var (
			rooms []models.Room
			total int64
			err   error
		)
		if activeOnly {
			rooms, err = roomRepo.GetAllActiveRooms()
			total = int64(len(rooms))
		} else {
			rooms, total, err = roomRepo.GetAllRoomsPaginated(repository.PaginationParams{Page: page, Limit: pageSize})
		}
		if err != nil {
			return fmt.Errorf("list rooms: %w", err)
		}

		if clioutput.JSON() {
			summaries := make([]map[string]any, 0, len(rooms))
			for i := range rooms {
				summaries = append(summaries, roomSummary(&rooms[i]))
			}
			data := map[string]any{
				"rooms":      summaries,
				"activeOnly": activeOnly,
				"total":      total,
			}
			if !activeOnly {
				data["page"] = page
				data["pageSize"] = pageSize
			}
			return clioutput.Success("", data)
		}

		clioutput.Printf("%-36s  %-30s  %-36s  %-7s  %-7s  %-9s  %s\n",
			"ID", "NAME", "CREATED_BY", "MODE", "ACTIVE", "MAX_PART", "EXPIRES_AT")
		for _, r := range rooms {
			active := "no"
			if r.IsActive {
				active = "yes"
			}
			clioutput.Printf("%-36s  %-30s  %-36s  %-7s  %-7s  %-9d  %s\n",
				r.ID, truncate(r.Name, 30), r.CreatedBy, r.Mode, active, r.MaxParticipants, r.ExpiresAt.Format(time.RFC3339))
		}
		if !activeOnly {
			clioutput.Printf("\nshowing page %d (%d per page) of %d total room(s)\n", page, pageSize, total)
		} else {
			clioutput.Printf("\n%d active room(s)\n", total)
		}
		return nil
	})
}

func ShowRoom(configPath, roomID string) error {
	return withRepo(configPath, func(cfg *config.Config, roomRepo *repository.RoomRepository) error {
		room, err := getRoomByIDOrName(roomRepo, roomID)
		if err != nil {
			return err
		}
		participants, err := roomRepo.GetActiveParticipants(room.ID)
		if err != nil {
			return fmt.Errorf("get participants: %w", err)
		}
		if clioutput.JSON() {
			return clioutput.Success("", map[string]any{
				"room":                   roomDetail(room),
				"activeParticipantCount": len(participants),
			})
		}
		clioutput.Println("Room:")
		clioutput.Printf("  ID:               %s\n", room.ID)
		clioutput.Printf("  Name:             %s\n", room.Name)
		clioutput.Printf("  CreatedBy:        %s\n", room.CreatedBy)
		clioutput.Printf("  Admin:            %s\n", room.AdminID)
		clioutput.Printf("  Mode:             %s\n", room.Mode)
		clioutput.Printf("  Public:           %t\n", room.IsPublic)
		clioutput.Printf("  Active:           %t\n", room.IsActive)
		clioutput.Printf("  MaxParticipants:  %d\n", room.MaxParticipants)
		clioutput.Printf("  CreatedAt:        %s\n", room.CreatedAt.Format(time.RFC3339))
		clioutput.Printf("  ExpiresAt:        %s\n", room.ExpiresAt.Format(time.RFC3339))
		clioutput.Println("  Settings:")
		clioutput.Printf("    AllowChat:      %t\n", room.Settings.AllowChat)
		clioutput.Printf("    AllowVideo:     %t\n", room.Settings.AllowVideo)
		clioutput.Printf("    AllowAudio:     %t\n", room.Settings.AllowAudio)
		clioutput.Printf("    RequireApproval:%t\n", room.Settings.RequireApproval)
		clioutput.Printf("    E2EE:           %t\n", room.Settings.E2EE)
		clioutput.Printf("    Persistent:     %t\n", room.Settings.IsPersistent)
		clioutput.Printf("  ActiveParticipants: %d\n", len(participants))
		return nil
	})
}

func CloseRoom(configPath, roomID string) error {
	return withRepo(configPath, func(cfg *config.Config, roomRepo *repository.RoomRepository) error {
		room, err := getRoomByIDOrName(roomRepo, roomID)
		if err != nil {
			return err
		}
		svc := buildCleanupService(cfg, roomRepo)
		opts := services.CascadeDeleteOptions{
			SystemEvent:   "room_deleted",
			SystemMessage: "This room has been closed by an administrator",
		}
		if err := svc.CascadeDeleteRoom(context.Background(), room, opts); err != nil {
			return fmt.Errorf("close room: %w", err)
		}
		return clioutput.Success(
			fmt.Sprintf("✓ Closed room %s (%s)", room.Name, room.ID),
			map[string]any{"roomId": room.ID, "name": room.Name},
		)
	})
}

func SuspendRoom(configPath, roomID string) error {
	return withRepo(configPath, func(cfg *config.Config, roomRepo *repository.RoomRepository) error {
		room, err := getRoomByIDOrName(roomRepo, roomID)
		if err != nil {
			return err
		}
		svc := buildCleanupService(cfg, roomRepo)
		if err := svc.SuspendRoom(context.Background(), room); err != nil {
			return fmt.Errorf("suspend: %w", err)
		}
		return clioutput.Success(
			fmt.Sprintf("✓ Suspended room %s (%s)", room.Name, room.ID),
			map[string]any{"roomId": room.ID, "name": room.Name},
		)
	})
}

func ReactivateRoom(configPath, roomID string) error {
	return withRepo(configPath, func(cfg *config.Config, roomRepo *repository.RoomRepository) error {
		room, err := getRoomByIDOrName(roomRepo, roomID)
		if err != nil {
			return err
		}
		room.IsActive = true
		room.ExpiresAt = time.Now().Add(24 * time.Hour)
		if err := roomRepo.UpdateRoom(room); err != nil {
			return fmt.Errorf("reactivate: %w", err)
		}
		return clioutput.Success(
			fmt.Sprintf("✓ Reactivated room %s (%s)", room.Name, room.ID),
			map[string]any{"roomId": room.ID, "name": room.Name, "expiresAt": room.ExpiresAt.Format(time.RFC3339)},
		)
	})
}

func KickParticipant(configPath, roomID, identity string) error {
	return withRepo(configPath, func(cfg *config.Config, roomRepo *repository.RoomRepository) error {
		room, err := getRoomByIDOrName(roomRepo, roomID)
		if err != nil {
			return err
		}
		client := lkutil.NewClient(&cfg.LiveKit)
		ctx, err := lkutil.AuthContext(context.Background(), cfg.LiveKit.APIKey, cfg.LiveKit.APISecret, &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
		if err != nil {
			return fmt.Errorf("livekit auth: %w", err)
		}
		if _, err := client.RemoveParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room.Name, Identity: identity}); err != nil {
			return fmt.Errorf("livekit kick: %w", err)
		}
		dbWarn := ""
		if err := roomRepo.KickParticipant(room.ID, identity); err != nil {
			dbWarn = err.Error()
			clioutput.Printf("⚠ Kicked from LiveKit but DB update failed: %v\n", err)
		}
		data := map[string]any{
			"roomId":   room.ID,
			"roomName": room.Name,
			"identity": identity,
		}
		if dbWarn != "" {
			data["dbWarning"] = dbWarn
		}
		return clioutput.Success(fmt.Sprintf("✓ Kicked %s from room %s", identity, room.Name), data)
	})
}

func roomSummary(r *models.Room) map[string]any {
	if r == nil {
		return nil
	}
	return map[string]any{
		"id":              r.ID,
		"name":            r.Name,
		"createdBy":       r.CreatedBy,
		"mode":            r.Mode,
		"active":          r.IsActive,
		"maxParticipants": r.MaxParticipants,
		"expiresAt":       r.ExpiresAt.Format(time.RFC3339),
	}
}

func roomDetail(r *models.Room) map[string]any {
	if r == nil {
		return nil
	}
	return map[string]any{
		"id":              r.ID,
		"name":            r.Name,
		"createdBy":       r.CreatedBy,
		"adminId":         r.AdminID,
		"mode":            r.Mode,
		"public":          r.IsPublic,
		"active":          r.IsActive,
		"maxParticipants": r.MaxParticipants,
		"createdAt":       r.CreatedAt.Format(time.RFC3339),
		"expiresAt":       r.ExpiresAt.Format(time.RFC3339),
		"settings": map[string]any{
			"allowChat":       r.Settings.AllowChat,
			"allowVideo":      r.Settings.AllowVideo,
			"allowAudio":      r.Settings.AllowAudio,
			"requireApproval": r.Settings.RequireApproval,
			"e2ee":            r.Settings.E2EE,
			"isPersistent":    r.Settings.IsPersistent,
		},
	}
}

func withRepo(configPath string, fn func(*config.Config, *repository.RoomRepository) error) error {
	cfg, err := config.Load(configPath)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := database.Initialize(&cfg.Database); err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	defer func() { _ = database.Close() }()
	if err := database.RunMigrations(); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	return fn(cfg, repository.NewRoomRepository(database.GetDB()))
}

func getRoomByIDOrName(repo *repository.RoomRepository, ref string) (*models.Room, error) {
	if r, err := repo.GetRoom(ref); err == nil && r != nil {
		return r, nil
	}
	r, err := repo.GetRoomByName(ref)
	if err != nil {
		return nil, fmt.Errorf("look up room: %w", err)
	}
	if r == nil {
		return nil, fmt.Errorf("room not found: %s", ref)
	}
	return r, nil
}

func buildCleanupService(cfg *config.Config, roomRepo *repository.RoomRepository) *services.RoomCleanupService {
	client := lkutil.NewClient(&cfg.LiveKit)
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
	tracker := storage.NewChatUploadTracker(database.GetDB(), uploadDir, s3Deleter)
	return services.NewRoomCleanupService(roomRepo, nil, client, nil, cfg.LiveKit.APIKey, cfg.LiveKit.APISecret, tracker)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 1 {
		return ""
	}
	return s[:n-1] + "…"
}
