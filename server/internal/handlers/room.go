package handlers

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/database"
	"bedrud/internal/lkutil"
	"bedrud/internal/models"
	"bedrud/internal/queue"
	"bedrud/internal/repository"
	"bedrud/internal/services"
	"bedrud/internal/storage"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gofiber/fiber/v2"
	lkauth "github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"github.com/rs/zerolog/log"
)

// livekitTokenTTL is the validity duration for LiveKit access tokens.
// LK server auto-refreshes tokens for connected clients (10-min TTL),
// so this TTL primarily controls:
//
//	(a) initial connection gating,
//	(b) reconnection window after network interruption when auto-refresh is unavailable,
//	(c) self-hosted token revocation window (kicked users can't rejoin after TTL).
const livekitTokenTTL = 2 * time.Hour

func boolPtr(b bool) *bool { return &b }

type AdminUpdateRoomSettingsInput struct {
	AllowChat       *bool `json:"allowChat"`
	AllowVideo      *bool `json:"allowVideo"`
	AllowAudio      *bool `json:"allowAudio"`
	RequireApproval *bool `json:"requireApproval"`
	E2EE            *bool `json:"e2ee"`
	IsPersistent    *bool `json:"isPersistent"`
}

type CreateRoomRequest struct {
	Name            string              `json:"name"`
	MaxParticipants int                 `json:"maxParticipants"`
	IsPublic        bool                `json:"isPublic"`
	Mode            string              `json:"mode"`
	Settings        models.RoomSettings `json:"settings"`
}

type JoinRoomRequest struct {
	RoomName string `json:"roomName"`
}

type RoomHandler struct {
	roomRepo         *repository.RoomRepository
	userRepo         *repository.UserRepository
	livekitHost      string
	apiKey           string
	apiSecret        string
	client           livekit.RoomService
	uploadStore      storage.ChatUploadStore
	uploadMax        int64
	uploadTracker    *storage.ChatUploadTracker
	cleanupSvc       *services.RoomCleanupService
	settingsRepo     *repository.SettingsRepository
	deletionInFlight sync.Map
	uploadBackend    string
	inlineMaxBytes   int64
}

func NewRoomHandler(lkCfg *config.LiveKitConfig, chatCfg *config.ChatConfig, roomRepo *repository.RoomRepository, userRepo *repository.UserRepository, settingsRepo *repository.SettingsRepository, uploadTracker *storage.ChatUploadTracker, cleanupSvc *services.RoomCleanupService) *RoomHandler {
	client := lkutil.NewClient(lkCfg)

	uploadMax := chatCfg.Uploads.MaxBytes.Int64()
	if uploadMax == 0 {
		uploadMax = 10 * 1024 * 1024
	}

	inlineMaxBytes := chatCfg.Uploads.InlineMaxBytes.Int64()
	if inlineMaxBytes == 0 {
		inlineMaxBytes = 512_000 // 500 KB default
	}

	return &RoomHandler{
		roomRepo:       roomRepo,
		userRepo:       userRepo,
		livekitHost:    lkCfg.Host,
		apiKey:         lkCfg.APIKey,
		apiSecret:      lkCfg.APISecret,
		client:         client,
		uploadStore:    storage.NewChatUploadStore(&chatCfg.Uploads),
		uploadMax:      uploadMax,
		uploadTracker:  uploadTracker,
		cleanupSvc:     cleanupSvc,
		settingsRepo:   settingsRepo,
		uploadBackend:  chatCfg.Uploads.Backend,
		inlineMaxBytes: inlineMaxBytes,
	}
}

func (h *RoomHandler) maxParticipantsLimit() int {
	if h.settingsRepo == nil {
		return 1000
	}
	s, err := h.settingsRepo.GetEffectiveSettings()
	if err != nil || s.MaxParticipantsLimit <= 0 {
		return 1000
	}
	return s.MaxParticipantsLimit
}

// lkSessionStartedAt returns the LiveKit room creation timestamp (epoch ms) for
// roomName, or 0 if no active session exists yet. The first user to join sees 0
// because the LiveKit room is created when their client connects via WebSocket;
// subsequent joiners see the session start time.
func (h *RoomHandler) lkSessionStartedAt(ctx context.Context, roomName string) int64 {
	authedCtx := h.withAuth(ctx, &lkauth.VideoGrant{RoomList: true})
	resp, err := h.client.ListRooms(authedCtx, &livekit.ListRoomsRequest{Names: []string{roomName}})
	if err != nil || len(resp.Rooms) == 0 {
		return 0
	}
	return resp.Rooms[0].CreationTime * 1000
}

func (h *RoomHandler) withAuth(ctx context.Context, grants ...*lkauth.VideoGrant) context.Context {
	ctx, err := lkutil.AuthContext(ctx, h.apiKey, h.apiSecret, grants...)
	if err != nil {
		log.Error().Err(err).Msg("Failed to create LK auth context")
	}
	return ctx
}

func (h *RoomHandler) CreateRoom(c *fiber.Ctx) error {
	var req CreateRoomRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	// Normalize: trim whitespace, lowercase
	req.Name = strings.TrimSpace(strings.ToLower(req.Name))

	// Auto-generate name if not provided
	if req.Name == "" {
		generated, err := models.GenerateRandomRoomName()
		if err != nil {
			log.Error().Err(err).Msg("Failed to generate random room name")
			return c.Status(500).JSON(fiber.Map{"error": "Failed to generate room name"})
		}
		req.Name = generated
	}

	// Validate the room name (URL-safe check)
	if err := models.ValidateRoomName(req.Name); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	// Validate MaxParticipants
	if req.MaxParticipants < 0 {
		req.MaxParticipants = 0
	}
	limit := h.maxParticipantsLimit()
	if req.MaxParticipants > limit {
		return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("maxParticipants must be at most %d", limit)})
	}

	// Validate mode
	validModes := map[string]bool{"standard": true, "webinar": true, "broadcast": true}
	if req.Mode == "" {
		req.Mode = "standard"
	} else if !validModes[req.Mode] {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid room mode"})
	}

	claims := c.Locals("user").(*auth.Claims)
	isSuperAdmin := false
	for _, a := range claims.Accesses {
		if a == string(models.AccessSuperAdmin) {
			isSuperAdmin = true
			break
		}
	}
	if !isSuperAdmin {
		req.Settings.IsPersistent = false

		// Enforce per-user room creation limit
		settings, err := h.settingsRepo.GetEffectiveSettings()
		if err != nil {
			log.Error().Err(err).Msg("Failed to get settings for room limit check")
			return c.Status(500).JSON(fiber.Map{"error": "Failed to verify room limit"})
		}
		if settings.MaxRoomsPerUser > 0 {
			activeCount, err := h.roomRepo.CountActiveRoomsByUser(claims.UserID)
			if err != nil {
				log.Error().Err(err).Msg("Failed to count user rooms")
				return c.Status(500).JSON(fiber.Map{"error": "Failed to verify room limit"})
			}
			if activeCount >= int64(settings.MaxRoomsPerUser) {
				return c.Status(403).JSON(fiber.Map{
					"error": fmt.Sprintf("Room limit reached (max %d active rooms)", settings.MaxRoomsPerUser),
				})
			}
		}
	}
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomCreate: true})
	_, err := h.client.CreateRoom(ctx, &livekit.CreateRoomRequest{Name: req.Name, MaxParticipants: uint32(req.MaxParticipants)})
	if err != nil {
		log.Error().Err(err).Str("room", req.Name).Msg("LiveKit CreateRoom failed")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create media room"})
	}
	room, err := h.roomRepo.CreateRoom(claims.UserID, req.Name, req.IsPublic, req.Mode, req.MaxParticipants, &req.Settings)
	if err != nil {
		// Clean up orphaned LiveKit room on DB failure
		if _, delErr := h.client.DeleteRoom(ctx, &livekit.DeleteRoomRequest{Room: req.Name}); delErr != nil {
			log.Warn().Err(delErr).Str("room", req.Name).Msg("Failed to clean up orphaned LK room")
		}
		// Map specific errors to appropriate HTTP status codes
		if errors.Is(err, models.ErrRoomNameTaken) {
			return c.Status(409).JSON(fiber.Map{"error": err.Error()})
		}
		if errors.Is(err, models.ErrRoomNameInvalid) || errors.Is(err, models.ErrRoomNameTooShort) || errors.Is(err, models.ErrRoomNameTooLong) {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}
		log.Error().Err(err).Msg("Database CreateRoom failed")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create room"})
	}
	return c.JSON(fiber.Map{
		"id": room.ID, "name": room.Name, "createdBy": room.CreatedBy, "isActive": room.IsActive,
		"isPublic": room.IsPublic, "maxParticipants": room.MaxParticipants, "settings": room.Settings,
		"livekitHost": h.livekitHost, "mode": room.Mode,
	})
}

func (h *RoomHandler) JoinRoom(c *fiber.Ctx) error {
	var req JoinRoomRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	req.RoomName = strings.ToLower(strings.TrimSpace(req.RoomName))
	claims := c.Locals("user").(*auth.Claims)
	room, err := h.roomRepo.GetRoomByName(req.RoomName)
	if err != nil {
		log.Error().Err(err).Str("room", req.RoomName).Msg("Failed to look up room")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to look up room"})
	}
	if room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}

	// Enforce room active state — suspended rooms cannot be rejoined
	if !room.IsActive {
		return c.Status(fiber.StatusGone).JSON(fiber.Map{"error": "room is no longer active"})
	}

	// Enforce private room access — non-creators must be approved participants
	if !room.IsPublic && room.CreatedBy != claims.UserID {
		isParticipant, err := h.roomRepo.IsParticipant(room.ID, claims.UserID)
		if err != nil {
			log.Error().Err(err).Str("roomID", room.ID).Str("userID", claims.UserID).Msg("Failed to check room access")
			return c.Status(500).JSON(fiber.Map{"error": "Failed to check room access"})
		}
		if !isParticipant {
			if room.Settings.RequireApproval {
				return c.Status(403).JSON(fiber.Map{"error": "This room requires approval to join"})
			}
			return c.Status(403).JSON(fiber.Map{"error": "This room is private"})
		}
	}

	// Enforce participant limit (atomic capacity check inside transaction)
	if err := h.roomRepo.AddParticipantWithCapacityCheck(room.ID, claims.UserID, room.MaxParticipants); err != nil {
		if err.Error() == "room is full" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "room is full"})
		}
		if err.Error() == "user is banned from this room" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "you are banned from this room"})
		}
		log.Error().Err(err).Str("roomID", room.ID).Str("userID", claims.UserID).Msg("AddParticipant failed")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to join room"})
	}

	at := lkauth.NewAccessToken(h.apiKey, h.apiSecret)
	at.AddGrant(&lkauth.VideoGrant{RoomJoin: true, Room: req.RoomName, CanUpdateOwnMetadata: boolPtr(false)}).SetIdentity(claims.UserID).SetName(claims.Name).SetValidFor(time.Hour) //nolint:staticcheck // AddGrant is deprecated but VideoGrant field is not available in this version of the protocol SDK
	if meta, err := json.Marshal(map[string]interface{}{"accesses": claims.Accesses}); err == nil {
		at.SetMetadata(string(meta))
	}
	token, err := at.ToJWT()
	if err != nil {
		log.Error().Err(err).Msg("Failed to sign LiveKit join token")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate room token"})
	}

	adminId := room.AdminID
	if adminId == "" {
		adminId = room.CreatedBy
	}

	sessionStartedAt := h.lkSessionStartedAt(c.Context(), req.RoomName)

	return c.JSON(fiber.Map{
		"id": room.ID, "name": room.Name, "token": token, "createdBy": room.CreatedBy, "adminId": adminId, "isActive": room.IsActive,
		"isPublic": room.IsPublic, "maxParticipants": room.MaxParticipants, "expiresAt": room.ExpiresAt,
		"settings": room.Settings, "livekitHost": h.livekitHost, "mode": room.Mode,
		"sessionStartedAt": sessionStartedAt,
	})
}

type GuestJoinRoomRequest struct {
	RoomName  string `json:"roomName"`
	GuestName string `json:"guestName"`
}

type RefreshLiveKitTokenRequest struct {
	RoomName string `json:"roomName"`
}

func (h *RoomHandler) GuestJoinRoom(c *fiber.Ctx) error {
	var req GuestJoinRoomRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	req.RoomName = strings.ToLower(strings.TrimSpace(req.RoomName))
	req.GuestName = strings.TrimSpace(req.GuestName)

	// Sanitize guest name: strip control characters and HTML special chars
	// Must run before validation to prevent null byte and control char bypass
	req.GuestName = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '<' || r == '>' || r == '&' || r == '"' || r == '\'' {
			return -1
		}
		return r
	}, req.GuestName)

	if req.GuestName == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Guest name is required"})
	}
	if len(req.GuestName) > 50 {
		return c.Status(400).JSON(fiber.Map{"error": "Guest name too long (max 50 characters)"})
	}

	room, err := h.roomRepo.GetRoomByName(req.RoomName)
	if err != nil {
		log.Error().Err(err).Str("room", req.RoomName).Msg("Failed to look up room")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to look up room"})
	}
	if room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}
	if !room.IsPublic {
		return c.Status(403).JSON(fiber.Map{"error": "This room is private"})
	}

	// Enforce room active state
	if !room.IsActive {
		return c.Status(fiber.StatusGone).JSON(fiber.Map{"error": "room is no longer active"})
	}

	guestID := "guest-" + generateShortID()
	// Note: Guest ban checking with random IDs is inherently ineffective —
	// each guest visit generates a new ID. IP-based tracking or signed
	// guest cookies would be needed for real guest ban enforcement.

	// Atomic capacity check + participant tracking
	if err := h.roomRepo.AddParticipantWithCapacityCheck(room.ID, guestID, room.MaxParticipants); err != nil {
		if err.Error() == "room is full" {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "room is full"})
		}
		log.Warn().Err(err).Str("roomID", room.ID).Str("guestID", guestID).Msg("Failed to track guest participant")
	}

	at := lkauth.NewAccessToken(h.apiKey, h.apiSecret)
	at.AddGrant(&lkauth.VideoGrant{ //nolint:staticcheck // AddGrant is deprecated but VideoGrant field is not available in this version of the protocol SDK
		RoomJoin:             true,
		Room:                 req.RoomName,
		CanUpdateOwnMetadata: boolPtr(false),
	}).SetIdentity(guestID).SetName(req.GuestName).SetValidFor(time.Hour)
	token, err := at.ToJWT()
	if err != nil {
		log.Error().Err(err).Msg("Failed to sign LiveKit guest token")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate room token"})
	}

	adminId := room.AdminID
	if adminId == "" {
		adminId = room.CreatedBy
	}

	sessionStartedAt := h.lkSessionStartedAt(c.Context(), req.RoomName)

	return c.JSON(fiber.Map{
		"id": room.ID, "name": room.Name, "token": token, "adminId": adminId,
		"livekitHost": h.livekitHost, "sessionStartedAt": sessionStartedAt,
	})
}

func (h *RoomHandler) RefreshLiveKitToken(c *fiber.Ctx) error {
	var req RefreshLiveKitTokenRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	req.RoomName = strings.ToLower(strings.TrimSpace(req.RoomName))
	if req.RoomName == "" {
		return c.Status(400).JSON(fiber.Map{"error": "roomName required"})
	}

	claims := c.Locals("user").(*auth.Claims)

	room, err := h.roomRepo.GetRoomByName(req.RoomName)
	if err != nil {
		log.Error().Err(err).Str("room", req.RoomName).Msg("Failed to look up room")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to look up room"})
	}
	if room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}
	if !room.IsActive {
		return c.Status(fiber.StatusGone).JSON(fiber.Map{"error": "room is no longer active"})
	}

	// Enforce private room access — only creator or approved participants can refresh
	if !room.IsPublic && room.CreatedBy != claims.UserID {
		isParticipant, err := h.roomRepo.IsParticipant(room.ID, claims.UserID)
		if err != nil {
			log.Error().Err(err).Str("roomID", room.ID).Str("userID", claims.UserID).Msg("Failed to check room access")
			return c.Status(500).JSON(fiber.Map{"error": "Failed to check room access"})
		}
		if !isParticipant {
			banned, err := h.roomRepo.IsParticipantBanned(room.ID, claims.UserID)
			if err == nil && banned {
				return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "you are banned from this room"})
			}
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "This room is private"})
		}
	}

	at := lkauth.NewAccessToken(h.apiKey, h.apiSecret)
	at.AddGrant(&lkauth.VideoGrant{RoomJoin: true, Room: req.RoomName, CanUpdateOwnMetadata: boolPtr(false)}).SetIdentity(claims.UserID).SetName(claims.Name).SetValidFor(livekitTokenTTL) //nolint:staticcheck
	if meta, err := json.Marshal(map[string]interface{}{"accesses": claims.Accesses}); err == nil {
		at.SetMetadata(string(meta))
	}
	token, err := at.ToJWT()
	if err != nil {
		log.Error().Err(err).Msg("Failed to sign LiveKit refresh token")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate token"})
	}

	return c.JSON(fiber.Map{"token": token})
}

func generateShortID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	const charLen = len(chars)             // 36
	const maxValid = 256 - (256 % charLen) // 252, rejection threshold

	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%x", time.Now().UnixNano())[:8]
	}
	for i, v := range b {
		// Rejection sampling: retry biased values (4 of 256)
		for int(v) >= maxValid {
			if _, err := rand.Read(b[i : i+1]); err != nil {
				v = byte(int(v) % charLen)
				break
			}
			v = b[i]
		}
		b[i] = chars[int(v)%charLen]
	}
	return string(b)
}

func (h *RoomHandler) ListRooms(c *fiber.Ctx) error {
	claims := c.Locals("user").(*auth.Claims)
	rooms, err := h.roomRepo.GetRoomsCreatedByUser(claims.UserID)
	if err != nil {
		log.Error().Err(err).Str("userID", claims.UserID).Msg("Failed to list rooms")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to list rooms"})
	}
	if rooms == nil {
		rooms = []models.Room{}
	}
	return c.JSON(rooms)
}

func (h *RoomHandler) sendSystemMessage(ctx context.Context, roomName, event, actor, target string) {
	type sysMsg struct {
		Type   string `json:"type"`
		Event  string `json:"event"`
		Actor  string `json:"actor"`
		Target string `json:"target"`
	}
	b, _ := json.Marshal(sysMsg{Type: "system", Event: event, Actor: actor, Target: target})
	topic := "system"
	_, _ = h.client.SendData(ctx, &livekit.SendDataRequest{
		Room:  roomName,
		Data:  b,
		Kind:  livekit.DataPacket_RELIABLE,
		Topic: &topic,
	})
}

// sendTargetedSystemMessage sends a system data message to only the target identity.
func (h *RoomHandler) sendTargetedSystemMessage(ctx context.Context, roomName, event, actor, target string) {
	type sysMsg struct {
		Type   string `json:"type"`
		Event  string `json:"event"`
		Actor  string `json:"actor"`
		Target string `json:"target"`
	}
	b, _ := json.Marshal(sysMsg{Type: "system", Event: event, Actor: actor, Target: target})
	topic := "system"
	_, _ = h.client.SendData(ctx, &livekit.SendDataRequest{
		Room:                  roomName,
		Data:                  b,
		Kind:                  livekit.DataPacket_RELIABLE,
		Topic:                 &topic,
		DestinationIdentities: []string{target},
	})
}

// resolveRoom loads a room by ID and returns (room, adminId, error response). Returns nil room + wrote response on error.
func (h *RoomHandler) resolveRoom(c *fiber.Ctx, roomID string) (*models.Room, string, error) {
	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil {
		log.Error().Err(err).Str("roomId", roomID).Msg("Failed to look up room")
		_ = c.Status(500).JSON(fiber.Map{"error": "Failed to look up room"})
		return nil, "", err
	}
	if room == nil {
		_ = c.Status(404).JSON(fiber.Map{"error": "Room not found"})
		return nil, "", fmt.Errorf("room not found")
	}
	adminId := room.AdminID
	if adminId == "" {
		adminId = room.CreatedBy
	}
	return room, adminId, nil
}

func (h *RoomHandler) PromoteParticipant(c *fiber.Ctx) error {
	roomID, identity := c.Params("roomId"), c.Params("identity")
	claims := c.Locals("user").(*auth.Claims)
	room, adminId, err := h.resolveRoom(c, roomID)
	if err != nil {
		return nil
	}
	if claims.UserID != adminId && !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(403).JSON(fiber.Map{"error": "Insufficient permissions"})
	}
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
	p, err := h.client.GetParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room.Name, Identity: identity})
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Participant not found"})
	}
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(p.Metadata), &meta); err != nil || meta == nil {
		meta = map[string]interface{}{}
	}
	accesses, _ := meta["accesses"].([]interface{})
	for _, a := range accesses {
		if a == "moderator" {
			return c.JSON(fiber.Map{"status": "already_moderator"})
		}
	}
	meta["accesses"] = append(accesses, "moderator")
	newMeta, _ := json.Marshal(meta)
	_, err = h.client.UpdateParticipant(ctx, &livekit.UpdateParticipantRequest{
		Room: room.Name, Identity: identity, Metadata: string(newMeta),
	})
	if err != nil {
		log.Error().Err(err).Msg("LK UpdateParticipant failed (promote)")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update participant"})
	}
	// Persist room-scoped moderator flag in DB so isRoomModerator checks work.
	if err := h.roomRepo.SetRoomModerator(room.ID, identity, true); err != nil {
		log.Error().Err(err).Msg("Failed to persist moderator flag")
	}
	return c.JSON(fiber.Map{"status": "success"})
}

func (h *RoomHandler) DemoteParticipant(c *fiber.Ctx) error {
	roomID, identity := c.Params("roomId"), c.Params("identity")
	claims := c.Locals("user").(*auth.Claims)
	room, adminId, err := h.resolveRoom(c, roomID)
	if err != nil {
		return nil
	}
	if claims.UserID != adminId && !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(403).JSON(fiber.Map{"error": "Insufficient permissions"})
	}
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
	p, err := h.client.GetParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room.Name, Identity: identity})
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Participant not found"})
	}
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(p.Metadata), &meta); err != nil || meta == nil {
		meta = map[string]interface{}{}
	}
	accesses, _ := meta["accesses"].([]interface{})
	filtered := make([]interface{}, 0, len(accesses))
	for _, a := range accesses {
		if a != "moderator" {
			filtered = append(filtered, a)
		}
	}
	meta["accesses"] = filtered
	newMeta, _ := json.Marshal(meta)
	_, err = h.client.UpdateParticipant(ctx, &livekit.UpdateParticipantRequest{
		Room: room.Name, Identity: identity, Metadata: string(newMeta),
	})
	if err != nil {
		log.Error().Err(err).Msg("LK UpdateParticipant failed (demote)")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update participant"})
	}
	// Clear room-scoped moderator flag in DB.
	if err := h.roomRepo.SetRoomModerator(room.ID, identity, false); err != nil {
		log.Error().Err(err).Msg("Failed to clear moderator flag")
	}
	return c.JSON(fiber.Map{"status": "success"})
}

func (h *RoomHandler) BlockChat(c *fiber.Ctx) error {
	roomID, identity := c.Params("roomId"), c.Params("identity")
	claims := c.Locals("user").(*auth.Claims)
	room, adminId, err := h.resolveRoom(c, roomID)
	if err != nil {
		return nil
	}
	if !isRoomModerator(claims, adminId, room.ID, h.roomRepo) {
		return c.Status(403).JSON(fiber.Map{"error": "not authorized for this room"})
	}
	if identity == claims.UserID {
		return c.Status(400).JSON(fiber.Map{"error": "Cannot perform this action on yourself"})
	}
	if identity == adminId {
		return c.Status(403).JSON(fiber.Map{"error": "Cannot target the room admin"})
	}
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
	p, err := h.client.GetParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room.Name, Identity: identity})
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Participant not found"})
	}
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(p.Metadata), &meta); err != nil || meta == nil {
		meta = map[string]interface{}{}
	}
	meta["chatBlocked"] = true
	newMeta, _ := json.Marshal(meta)
	_, err = h.client.UpdateParticipant(ctx, &livekit.UpdateParticipantRequest{
		Room: room.Name, Identity: identity, Metadata: string(newMeta),
	})
	if err != nil {
		log.Error().Err(err).Msg("LK UpdateParticipant failed (block chat)")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update participant"})
	}
	return c.JSON(fiber.Map{"status": "success"})
}

func (h *RoomHandler) DeafenParticipant(c *fiber.Ctx) error {
	roomID, identity := c.Params("roomId"), c.Params("identity")
	claims := c.Locals("user").(*auth.Claims)
	room, adminId, err := h.resolveRoom(c, roomID)
	if err != nil {
		return nil
	}
	if !isRoomModerator(claims, adminId, room.ID, h.roomRepo) {
		return c.Status(403).JSON(fiber.Map{"error": "not authorized for this room"})
	}
	if identity == claims.UserID {
		return c.Status(400).JSON(fiber.Map{"error": "Cannot perform this action on yourself"})
	}
	if identity == adminId {
		return c.Status(403).JSON(fiber.Map{"error": "Cannot target the room admin"})
	}
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
	h.sendTargetedSystemMessage(ctx, room.Name, "deafen", claims.UserID, identity)
	return c.JSON(fiber.Map{"status": "success"})
}

func (h *RoomHandler) UndeafenParticipant(c *fiber.Ctx) error {
	roomID, identity := c.Params("roomId"), c.Params("identity")
	claims := c.Locals("user").(*auth.Claims)
	room, adminId, err := h.resolveRoom(c, roomID)
	if err != nil {
		return nil
	}
	if !isRoomModerator(claims, adminId, room.ID, h.roomRepo) {
		return c.Status(403).JSON(fiber.Map{"error": "not authorized for this room"})
	}
	if identity == claims.UserID {
		return c.Status(400).JSON(fiber.Map{"error": "Cannot perform this action on yourself"})
	}
	if identity == adminId {
		return c.Status(403).JSON(fiber.Map{"error": "Cannot target the room admin"})
	}
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
	h.sendTargetedSystemMessage(ctx, room.Name, "undeafen", claims.UserID, identity)
	return c.JSON(fiber.Map{"status": "success"})
}

func (h *RoomHandler) AskParticipantAction(c *fiber.Ctx) error {
	roomID, identity, action := c.Params("roomId"), c.Params("identity"), c.Params("action")
	if action != "unmute" && action != "camera" {
		return c.Status(400).JSON(fiber.Map{"error": "Unknown action"})
	}
	claims := c.Locals("user").(*auth.Claims)
	room, adminId, err := h.resolveRoom(c, roomID)
	if err != nil {
		return nil
	}
	if !isRoomModerator(claims, adminId, room.ID, h.roomRepo) {
		return c.Status(403).JSON(fiber.Map{"error": "not authorized for this room"})
	}
	if identity == claims.UserID {
		return c.Status(400).JSON(fiber.Map{"error": "Cannot perform this action on yourself"})
	}
	if identity == adminId {
		return c.Status(403).JSON(fiber.Map{"error": "Cannot target the room admin"})
	}
	event := "ask_" + action
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
	h.sendTargetedSystemMessage(ctx, room.Name, event, claims.UserID, identity)
	return c.JSON(fiber.Map{"status": "success"})
}

func (h *RoomHandler) SpotlightParticipant(c *fiber.Ctx) error {
	roomID, identity := c.Params("roomId"), c.Params("identity")
	claims := c.Locals("user").(*auth.Claims)
	room, adminId, err := h.resolveRoom(c, roomID)
	if err != nil {
		return nil
	}
	if !isRoomModerator(claims, adminId, room.ID, h.roomRepo) {
		return c.Status(403).JSON(fiber.Map{"error": "not authorized for this room"})
	}
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
	// Broadcast to entire room so all clients pin this participant
	h.sendSystemMessage(ctx, room.Name, "spotlight", claims.UserID, identity)
	return c.JSON(fiber.Map{"status": "success"})
}

func (h *RoomHandler) StopScreenShare(c *fiber.Ctx) error {
	roomID, identity := c.Params("roomId"), c.Params("identity")
	claims := c.Locals("user").(*auth.Claims)
	room, adminId, err := h.resolveRoom(c, roomID)
	if err != nil {
		return nil
	}
	if !isRoomModerator(claims, adminId, room.ID, h.roomRepo) {
		return c.Status(403).JSON(fiber.Map{"error": "not authorized for this room"})
	}
	if identity == claims.UserID {
		return c.Status(400).JSON(fiber.Map{"error": "Cannot perform this action on yourself"})
	}
	if identity == adminId {
		return c.Status(403).JSON(fiber.Map{"error": "Cannot target the room admin"})
	}
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
	p, err := h.client.GetParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room.Name, Identity: identity})
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Participant not found"})
	}
	for _, track := range p.Tracks {
		if track.Source == livekit.TrackSource_SCREEN_SHARE || track.Source == livekit.TrackSource_SCREEN_SHARE_AUDIO {
			if _, err := h.client.MutePublishedTrack(ctx, &livekit.MuteRoomTrackRequest{
				Room: room.Name, Identity: identity, TrackSid: track.Sid, Muted: true,
			}); err != nil {
				log.Warn().Err(err).Str("room", room.Name).Str("identity", identity).Str("track", track.Sid).Msg("Failed to mute screen share track")
			}
		}
	}
	return c.JSON(fiber.Map{"status": "success"})
}

func (h *RoomHandler) GetParticipantInfo(c *fiber.Ctx) error {
	roomID, identity := c.Params("roomId"), c.Params("identity")
	claims := c.Locals("user").(*auth.Claims)
	room, adminId, err := h.resolveRoom(c, roomID)
	if err != nil {
		return nil
	}
	// Self-access always allowed; room moderator/owner/superadmin can view anyone
	if claims.UserID != identity && !isRoomModerator(claims, adminId, room.ID, h.roomRepo) {
		return c.Status(403).JSON(fiber.Map{"error": "not authorized for this room"})
	}
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
	p, err := h.client.GetParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room.Name, Identity: identity})
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Participant not found"})
	}
	type TrackInfo struct {
		Sid    string `json:"sid"`
		Type   string `json:"type"`
		Source string `json:"source"`
		Muted  bool   `json:"muted"`
	}
	tracks := make([]TrackInfo, 0, len(p.Tracks))
	for _, t := range p.Tracks {
		tracks = append(tracks, TrackInfo{
			Sid:    t.Sid,
			Type:   t.Type.String(),
			Source: t.Source.String(),
			Muted:  t.Muted,
		})
	}
	return c.JSON(fiber.Map{
		"identity": p.Identity,
		"name":     p.Name,
		"state":    p.State.String(),
		"joinedAt": p.JoinedAt,
		"tracks":   tracks,
	})
}

func (h *RoomHandler) KickParticipant(c *fiber.Ctx) error {
	roomID, identity := c.Params("roomId"), c.Params("identity")
	claims := c.Locals("user").(*auth.Claims)
	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil {
		log.Error().Err(err).Str("roomId", roomID).Msg("Failed to look up room")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to look up room"})
	}
	if room == nil {
		return c.SendStatus(404)
	}
	adminId := room.AdminID
	if adminId == "" {
		adminId = room.CreatedBy
	}
	if claims.UserID != adminId && !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(403).JSON(fiber.Map{"error": "Insufficient permissions"})
	}
	// Prevent self-targeting
	if identity == claims.UserID {
		return c.Status(400).JSON(fiber.Map{"error": "Cannot perform this action on yourself"})
	}
	// Prevent targeting the room admin
	if identity == adminId {
		return c.Status(403).JSON(fiber.Map{"error": "Cannot kick the room admin"})
	}
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
	_, err = h.client.RemoveParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room.Name, Identity: identity})
	if err != nil {
		log.Error().Err(err).Msg("LK RemoveParticipant failed (kick)")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to remove participant"})
	}
	h.sendSystemMessage(ctx, room.Name, "kick", claims.UserID, identity)
	_ = h.roomRepo.RemoveParticipant(room.ID, identity)
	return c.JSON(fiber.Map{"status": "success"})
}

// @Summary Delete room
// @Description Delete a room you created. Enqueues room deletion and returns immediately.
// @Tags rooms
// @Accept json
// @Produce json
// @Param roomId path string true "Room ID"
// @Security BearerAuth
// @Success 202 {object} map[string]interface{} "Deletion queued"
// @Failure 403 {object} ErrorResponse "Not the room creator"
// @Failure 404 {object} ErrorResponse "Room not found"
// @Failure 409 {object} ErrorResponse "Deletion already in progress"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /room/{roomId} [delete]
func (h *RoomHandler) DeleteRoom(c *fiber.Ctx) error {
	roomID := c.Params("roomId")
	claims := c.Locals("user").(*auth.Claims)

	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil {
		log.Error().Err(err).Str("roomId", roomID).Msg("Failed to look up room")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to look up room"})
	}
	if room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}
	if room.CreatedBy != claims.UserID && !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(403).JSON(fiber.Map{"error": "Only the room creator can delete this room"})
	}

	if _, loaded := h.deletionInFlight.LoadOrStore(roomID, true); loaded {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Deletion already in progress"})
	}

	payload := queue.RoomDeletePayload{
		RoomID:        roomID,
		SystemEvent:   "room_ended",
		SystemMessage: "The meeting has been ended by the creator",
	}
	if err := queue.Enqueue(context.Background(), database.GetDB(), "room_delete", payload,
		queue.WithPriority(1), queue.WithMaxAttempts(3)); err != nil {
		h.deletionInFlight.Delete(roomID)
		log.Error().Err(err).Str("roomId", roomID).Msg("Failed to enqueue room deletion")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to queue deletion"})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"message": "Room deletion queued"})
}

func (h *RoomHandler) MuteParticipant(c *fiber.Ctx) error {
	roomID, identity := c.Params("roomId"), c.Params("identity")
	claims := c.Locals("user").(*auth.Claims)
	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil {
		log.Error().Err(err).Str("roomId", roomID).Msg("Failed to look up room")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to look up room"})
	}
	if room == nil {
		return c.SendStatus(404)
	}
	adminId := room.AdminID
	if adminId == "" {
		adminId = room.CreatedBy
	}
	if claims.UserID != adminId && !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(403).JSON(fiber.Map{"error": "Insufficient permissions"})
	}
	// Prevent self-targeting
	if identity == claims.UserID {
		return c.Status(400).JSON(fiber.Map{"error": "Cannot perform this action on yourself"})
	}
	// Prevent targeting the room admin
	if identity == adminId {
		return c.Status(403).JSON(fiber.Map{"error": "Cannot mute the room admin"})
	}
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
	p, err := h.client.GetParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room.Name, Identity: identity})
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Participant not found"})
	}
	for _, track := range p.Tracks {
		if track.Type == livekit.TrackType_AUDIO {
			if _, err := h.client.MutePublishedTrack(ctx, &livekit.MuteRoomTrackRequest{
				Room: room.Name, Identity: identity, TrackSid: track.Sid, Muted: true,
			}); err != nil {
				log.Warn().Err(err).Str("room", room.Name).Str("identity", identity).Str("track", track.Sid).Msg("Failed to mute audio track")
			}
		}
	}
	return c.JSON(fiber.Map{"status": "success"})
}

func (h *RoomHandler) BanParticipant(c *fiber.Ctx) error {
	roomID, identity := c.Params("roomId"), c.Params("identity")
	claims := c.Locals("user").(*auth.Claims)
	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil {
		log.Error().Err(err).Str("roomId", roomID).Msg("Failed to look up room")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to look up room"})
	}
	if room == nil {
		return c.SendStatus(404)
	}
	adminId := room.AdminID
	if adminId == "" {
		adminId = room.CreatedBy
	}
	if claims.UserID != adminId && !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(403).JSON(fiber.Map{"error": "Insufficient permissions"})
	}
	// Prevent self-targeting
	if identity == claims.UserID {
		return c.Status(400).JSON(fiber.Map{"error": "Cannot perform this action on yourself"})
	}
	// Prevent targeting the room admin
	if identity == adminId {
		return c.Status(403).JSON(fiber.Map{"error": "Cannot ban the room admin"})
	}
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
	var removeErr error
	_, removeErr = h.client.RemoveParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room.Name, Identity: identity})
	if removeErr != nil {
		log.Error().Err(removeErr).Msg("LK RemoveParticipant failed (ban)")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to remove participant"})
	}
	h.sendSystemMessage(ctx, room.Name, "ban", claims.UserID, identity)
	if err := h.roomRepo.KickParticipant(room.ID, identity); err != nil {
		log.Warn().Err(err).Str("roomID", room.ID).Str("identity", identity).Msg("Failed to update ban in DB")
	}
	return c.JSON(fiber.Map{"status": "success"})
}

func containsAccess(accesses []string, target string) bool {
	for _, a := range accesses {
		if a == target {
			return true
		}
	}
	return false
}

func (h *RoomHandler) DisableParticipantVideo(c *fiber.Ctx) error {
	roomID, identity := c.Params("roomId"), c.Params("identity")
	claims := c.Locals("user").(*auth.Claims)
	room, adminId, err := h.resolveRoom(c, roomID)
	if err != nil {
		return nil
	}
	if !isRoomModerator(claims, adminId, room.ID, h.roomRepo) {
		return c.Status(403).JSON(fiber.Map{"error": "not authorized for this room"})
	}
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
	p, err := h.client.GetParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room.Name, Identity: identity})
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Participant not found"})
	}
	for _, track := range p.Tracks {
		if track.Type == livekit.TrackType_VIDEO && track.Source == livekit.TrackSource_CAMERA {
			if _, err := h.client.MutePublishedTrack(ctx, &livekit.MuteRoomTrackRequest{
				Room: room.Name, Identity: identity, TrackSid: track.Sid, Muted: true,
			}); err != nil {
				log.Warn().Err(err).Str("room", room.Name).Str("identity", identity).Str("track", track.Sid).Msg("Failed to mute camera track")
			}
		}
	}
	return c.JSON(fiber.Map{"status": "success"})
}
func (h *RoomHandler) BringToStage(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "not yet implemented"})
}
func (h *RoomHandler) RemoveFromStage(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "not yet implemented"})
}

func (h *RoomHandler) UpdateSettings(c *fiber.Ctx) error {
	roomID := c.Params("roomId")
	claims := c.Locals("user").(*auth.Claims)

	var input struct {
		IsPublic        *bool                `json:"isPublic"`
		MaxParticipants *int                 `json:"maxParticipants"`
		Settings        *models.RoomSettings `json:"settings"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}

	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil {
		log.Error().Err(err).Str("roomId", roomID).Msg("Failed to look up room")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to look up room"})
	}
	if room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}

	adminID := room.AdminID
	if adminID == "" {
		adminID = room.CreatedBy
	}
	if claims.UserID != adminID && !containsAccess(claims.Accesses, "superadmin") {
		return c.Status(403).JSON(fiber.Map{"error": "Insufficient permissions"})
	}

	if input.IsPublic != nil {
		room.IsPublic = *input.IsPublic
	}
	if input.MaxParticipants != nil {
		limit := h.maxParticipantsLimit()
		if *input.MaxParticipants < 0 || *input.MaxParticipants > limit {
			return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("maxParticipants must be between 0 and %d", limit)})
		}
		room.MaxParticipants = *input.MaxParticipants
	}
	if input.Settings != nil {
		// Preserve admin-only field — persistence is only changeable via AdminUpdateRoom.
		input.Settings.IsPersistent = room.Settings.IsPersistent
		room.Settings = *input.Settings
	}

	if err := h.roomRepo.UpdateRoom(room); err != nil {
		log.Error().Err(err).Str("roomId", roomID).Msg("Failed to update room settings")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to update room settings"})
	}

	// Sync MaxParticipants to LiveKit
	if input.MaxParticipants != nil {
		ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
		if _, err := h.client.CreateRoom(ctx, &livekit.CreateRoomRequest{
			Name: room.Name, MaxParticipants: uint32(*input.MaxParticipants),
		}); err != nil {
			log.Warn().Err(err).Msg("Failed to sync MaxParticipants to LiveKit")
		}
	}

	return c.JSON(room)
}

func (h *RoomHandler) AdminListRooms(c *fiber.Ctx) error {
	p := repository.RoomFilterParams{
		Page:   c.QueryInt("page", 1),
		Limit:  c.QueryInt("limit", 50),
		Search: c.Query("q"),
	}

	// Parse multi-value visibility
	if vis := c.Query("visibility"); vis != "" {
		p.Visibility = strings.Split(vis, ",")
		for _, v := range p.Visibility {
			if v != "public" && v != "private" {
				return c.Status(400).JSON(fiber.Map{"error": "Invalid visibility: " + v})
			}
		}
	}

	// Parse status
	if st := c.Query("status"); st != "" {
		p.Status = strings.Split(st, ",")
		for _, v := range p.Status {
			if v != "active" && v != "suspended" {
				return c.Status(400).JSON(fiber.Map{"error": "Invalid status: " + v})
			}
		}
	}

	// Parse occupancy (filters on actual participant count)
	p.Occupancy = c.Query("occupancy")
	if p.Occupancy != "" {
		validOcc := map[string]bool{"empty": true, "1-5": true, "6-20": true, "20+": true}
		if !validOcc[p.Occupancy] {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid occupancy filter"})
		}
	}

	// Parse capacity (legacy, on max_participants)
	if p.Occupancy == "" {
		p.Capacity = c.Query("capacity")
		if p.Capacity != "" {
			validCaps := map[string]bool{"empty": true, "1-5": true, "6-20": true, "20+": true}
			if !validCaps[p.Capacity] {
				return c.Status(400).JSON(fiber.Map{"error": "Invalid capacity filter"})
			}
		}
	}

	// Parse created
	p.Created = c.Query("created")
	if p.Created != "" {
		validDurations := map[string]bool{"today": true, "7d": true, "30d": true}
		if !validDurations[p.Created] {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid created filter"})
		}
	}

	// Parse new filters
	p.Owner = c.Query("owner")
	p.DateFrom = c.Query("dateFrom")
	p.DateTo = c.Query("dateTo")
	p.LastActivityFrom = c.Query("lastActivityFrom")
	p.LastActivityTo = c.Query("lastActivityTo")

	// Parse sort/order
	p.Sort = c.Query("sort", "createdAt")
	p.Order = c.Query("order", "desc")
	validSorts := map[string]bool{
		"name": true, "createdAt": true, "maxParticipants": true,
		"participantsCount": true, "lastActivityAt": true, "createdBy": true,
	}
	if !validSorts[p.Sort] {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid sort field"})
	}
	if p.Order != "asc" && p.Order != "desc" {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid order, must be asc or desc"})
	}

	// Clamp limit
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 50
	}

	rooms, total, err := h.roomRepo.GetAllRoomsFiltered(p)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch rooms"})
	}

	// Enrich with computed fields (participants count, owner names, last activity)
	enriched, err := h.roomRepo.EnrichAdminRoomDetails(rooms)
	if err != nil {
		// Non-fatal — log and return basic rooms
		return c.JSON(fiber.Map{"rooms": rooms, "total": total, "page": p.Page, "limit": p.Limit})
	}

	return c.JSON(fiber.Map{"rooms": enriched, "total": total, "page": p.Page, "limit": p.Limit})
}

// GetAdminStats returns aggregate system statistics for the admin dashboard KPI strip.
func (h *RoomHandler) GetAdminStats(c *fiber.Ctx) error {
	now := time.Now()
	dayAgo := now.Add(-24 * time.Hour)
	weekAgo := now.Add(-7 * 24 * time.Hour)

	totalRooms, err := h.roomRepo.CountRooms()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count rooms"})
	}

	activeRooms, err := h.roomRepo.CountActiveRooms()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count active rooms"})
	}

	privateRooms, err := h.roomRepo.CountPrivateRooms()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count private rooms"})
	}

	publicRooms, err := h.roomRepo.CountPublicRooms()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count public rooms"})
	}

	emptyRooms, err := h.roomRepo.CountEmptyRooms()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count empty rooms"})
	}

	roomsLast24h, err := h.roomRepo.CountRoomsSince(dayAgo)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count rooms since"})
	}

	roomsLast7d, err := h.roomRepo.CountRoomsSince(weekAgo)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count rooms since"})
	}

	avgUsersPerRoom, err := h.roomRepo.AvgParticipantsPerRoom()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to compute avg participants"})
	}

	onlineUsers, err := h.roomRepo.CountActiveParticipants()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count active participants"})
	}

	totalUsers, err := h.userRepo.CountUsers()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count users"})
	}

	staleRooms, err := h.roomRepo.CountStaleRooms(48)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count stale rooms"})
	}

	return c.JSON(fiber.Map{
		"totalRooms":        totalRooms,
		"activeRooms":       activeRooms,
		"privateRooms":      privateRooms,
		"publicRooms":       publicRooms,
		"emptyRooms":        emptyRooms,
		"flaggedRooms":      0,
		"pendingActions":    0,
		"roomsLast24h":      roomsLast24h,
		"roomsLast7d":       roomsLast7d,
		"avgUsersPerRoom":   avgUsersPerRoom,
		"onlineUsers":       onlineUsers,
		"totalUsers":        totalUsers,
		"staleRooms":        staleRooms,
		"moderationFlags":   0,
	})
}

func (h *RoomHandler) AdminGenerateToken(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "not yet implemented"})
}

// @Summary Close room (admin)
// @Description Permanently delete a room by ID. Enqueues room deletion and returns immediately.
// @Tags admin
// @Accept json
// @Produce json
// @Param roomId path string true "Room ID"
// @Security BearerAuth
// @Success 202 {object} map[string]interface{} "Deletion queued"
// @Failure 404 {object} ErrorResponse "Room not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /admin/rooms/{roomId}/close [post]
func (h *RoomHandler) AdminCloseRoom(c *fiber.Ctx) error {
	roomID := c.Params("roomId")
	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch room"})
	}
	if room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}

	if _, loaded := h.deletionInFlight.LoadOrStore(roomID, true); loaded {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "Deletion already in progress"})
	}

	payload := queue.RoomDeletePayload{
		RoomID:        roomID,
		SystemEvent:   "room_closed",
		SystemMessage: "This room has been closed by an administrator",
	}
	if err := queue.Enqueue(context.Background(), database.GetDB(), "room_delete", payload,
		queue.WithPriority(1), queue.WithMaxAttempts(3)); err != nil {
		h.deletionInFlight.Delete(roomID)
		log.Error().Err(err).Str("roomId", roomID).Msg("Failed to enqueue room deletion")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to queue deletion"})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"message": "Room close queued"})
}

// @Summary Suspend room (admin)
// @Description Suspend a room, ending all active calls but preserving room data. Enqueues suspension and returns immediately.
// @Tags admin
// @Accept json
// @Produce json
// @Param roomId path string true "Room ID"
// @Security BearerAuth
// @Success 202 {object} map[string]interface{} "Suspension queued"
// @Failure 400 {object} ErrorResponse "Room already inactive"
// @Failure 404 {object} ErrorResponse "Room not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /admin/rooms/{roomId}/suspend [post]
func (h *RoomHandler) AdminSuspendRoom(c *fiber.Ctx) error {
	roomID := c.Params("roomId")
	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch room"})
	}
	if room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}
	if !room.IsActive {
		return c.Status(400).JSON(fiber.Map{"error": "Room is not active"})
	}

	payload := queue.RoomSuspendPayload{
		RoomID: roomID,
	}
	if err := queue.Enqueue(context.Background(), database.GetDB(), "room_suspend", payload,
		queue.WithPriority(2), queue.WithMaxAttempts(3)); err != nil {
		log.Error().Err(err).Str("roomId", roomID).Msg("Failed to enqueue room suspension")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to queue suspension"})
	}

	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"message": "Room suspension queued"})
}

func (h *RoomHandler) AdminReactivateRoom(c *fiber.Ctx) error {
	roomID := c.Params("roomId")
	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch room"})
	}
	if room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}
	if room.IsActive {
		return c.Status(400).JSON(fiber.Map{"error": "Room is already active"})
	}
	room.IsActive = true
	room.ExpiresAt = time.Now().Add(24 * time.Hour)
	if err := h.roomRepo.UpdateRoom(room); err != nil {
		log.Error().Err(err).Str("roomId", roomID).Msg("Failed to reactivate room")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to reactivate room"})
	}
	// Also recreate the LiveKit room
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomCreate: true})
	if _, err := h.client.CreateRoom(ctx, &livekit.CreateRoomRequest{
		Name: room.Name, MaxParticipants: uint32(room.MaxParticipants),
	}); err != nil {
		log.Warn().Err(err).Str("room", room.Name).Msg("Failed to recreate LiveKit room during reactivation")
	}
	return c.JSON(room)
}

func (h *RoomHandler) AdminUpdateRoom(c *fiber.Ctx) error {
	roomID := c.Params("roomId")
	var input struct {
		MaxParticipants *int                          `json:"maxParticipants"`
		Settings        *AdminUpdateRoomSettingsInput `json:"settings"`
	}
	if err := c.BodyParser(&input); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Input validation
	limit := h.maxParticipantsLimit()
	if input.MaxParticipants != nil && (*input.MaxParticipants < 1 || *input.MaxParticipants > limit) {
		return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("maxParticipants must be between 1 and %d", limit)})
	}

	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch room"})
	}
	if room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}

	// Apply all changes in-memory, then single atomic write
	if input.Settings != nil {
		merged := room.Settings
		if input.Settings.AllowChat != nil {
			merged.AllowChat = *input.Settings.AllowChat
		}
		if input.Settings.AllowVideo != nil {
			merged.AllowVideo = *input.Settings.AllowVideo
		}
		if input.Settings.AllowAudio != nil {
			merged.AllowAudio = *input.Settings.AllowAudio
		}
		if input.Settings.RequireApproval != nil {
			merged.RequireApproval = *input.Settings.RequireApproval
		}
		if input.Settings.E2EE != nil {
			merged.E2EE = *input.Settings.E2EE
		}
		if input.Settings.IsPersistent != nil {
			merged.IsPersistent = *input.Settings.IsPersistent
		}
		room.Settings = merged
	}
	if input.MaxParticipants != nil {
		room.MaxParticipants = *input.MaxParticipants
	}
	// Single atomic write — eliminates two-write race
	if input.Settings != nil || input.MaxParticipants != nil {
		if err := h.roomRepo.UpdateRoom(room); err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to update room"})
		}
	}

	// Sync MaxParticipants to LiveKit
	if input.MaxParticipants != nil {
		ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
		if _, err := h.client.CreateRoom(ctx, &livekit.CreateRoomRequest{
			Name: room.Name, MaxParticipants: uint32(*input.MaxParticipants),
		}); err != nil {
			log.Warn().Err(err).Msg("Failed to sync MaxParticipants to LiveKit (admin)")
		}
	}

	updated, err := h.roomRepo.GetRoom(roomID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch updated room"})
	}
	return c.JSON(updated)
}

func (h *RoomHandler) GetOnlineCount(c *fiber.Ctx) error {
	count, err := h.roomRepo.CountActiveParticipants()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to count participants"})
	}
	return c.JSON(fiber.Map{"count": count})
}

// AdminLiveKitStats returns aggregate stats from the LiveKit server.
func (h *RoomHandler) AdminLiveKitStats(c *fiber.Ctx) error {
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomList: true})
	resp, err := h.client.ListRooms(ctx, &livekit.ListRoomsRequest{})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch LiveKit stats"})
	}

	type RoomStat struct {
		Name            string `json:"name"`
		NumParticipants uint32 `json:"numParticipants"`
		NumPublishers   uint32 `json:"numPublishers"`
		CreationTime    int64  `json:"creationTime"`
	}
	var totalParticipants, totalPublishers uint32
	rooms := make([]RoomStat, 0, len(resp.Rooms))
	for _, r := range resp.Rooms {
		totalParticipants += r.NumParticipants
		totalPublishers += r.NumPublishers
		rooms = append(rooms, RoomStat{
			Name:            r.Name,
			NumParticipants: r.NumParticipants,
			NumPublishers:   r.NumPublishers,
			CreationTime:    r.CreationTime,
		})
	}
	return c.JSON(fiber.Map{
		"totalParticipants": totalParticipants,
		"totalPublishers":   totalPublishers,
		"activeRooms":       len(resp.Rooms),
		"rooms":             rooms,
	})
}

// AdminGetRoomParticipants fetches live participant data from LiveKit for a specific room.
func (h *RoomHandler) AdminGetRoomParticipants(c *fiber.Ctx) error {
	roomID := c.Params("roomId")
	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch room"})
	}
	if room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}

	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
	resp, err := h.client.ListParticipants(ctx, &livekit.ListParticipantsRequest{Room: room.Name})
	if err != nil {
		// Room may not be active in LiveKit — return empty list
		return c.JSON(fiber.Map{"participants": []struct{}{}, "room": room})
	}

	type TrackStat struct {
		Sid     string `json:"sid"`
		Type    string `json:"type"`
		Source  string `json:"source"`
		Muted   bool   `json:"muted"`
		Bitrate uint32 `json:"bitrate"` // target bitrate from highest quality layer
	}
	type ParticipantStat struct {
		Sid         string      `json:"sid"`
		Identity    string      `json:"identity"`
		Name        string      `json:"name"`
		State       string      `json:"state"`
		JoinedAt    int64       `json:"joinedAt"`
		IsPublisher bool        `json:"isPublisher"`
		Tracks      []TrackStat `json:"tracks"`
	}

	participants := make([]ParticipantStat, 0, len(resp.Participants))
	for _, p := range resp.Participants {
		tracks := make([]TrackStat, 0, len(p.Tracks))
		for _, t := range p.Tracks {
			var bitrate uint32
			for _, layer := range t.Layers { //nolint:staticcheck // Layers is deprecated but VideoLayers field is not available in this version of the protocol SDK
				if layer.Bitrate > bitrate {
					bitrate = layer.Bitrate
				}
			}
			tracks = append(tracks, TrackStat{
				Sid:     t.Sid,
				Type:    t.Type.String(),
				Source:  t.Source.String(),
				Muted:   t.Muted,
				Bitrate: bitrate,
			})
		}
		participants = append(participants, ParticipantStat{
			Sid:         p.Sid,
			Identity:    p.Identity,
			Name:        p.Name,
			State:       p.State.String(),
			JoinedAt:    p.JoinedAt,
			IsPublisher: p.IsPublisher,
			Tracks:      tracks,
		})
	}
	return c.JSON(fiber.Map{"participants": participants, "room": room})
}

// UploadChatImage accepts a multipart image upload for in-room chat.
// Any authenticated participant may upload. The server selects the storage backend
// based on config (disk / inline base64 / s3) and returns the attachment metadata.
func (h *RoomHandler) UploadChatImage(c *fiber.Ctx) error {
	roomID := c.Params("roomId")
	claims := c.Locals("user").(*auth.Claims)

	// Verify the room exists.
	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil || room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}

	// Check participant status
	isParticipant, err := h.roomRepo.IsParticipant(room.ID, claims.UserID)
	if err != nil {
		log.Error().Err(err).Msg("Failed to check participant status")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to verify access"})
	}
	if !isParticipant {
		return c.Status(403).JSON(fiber.Map{"error": "Not a participant in this room"})
	}

	// Check if chat is allowed
	if !room.Settings.AllowChat {
		return c.Status(403).JSON(fiber.Map{"error": "Chat is disabled for this room"})
	}

	// Enforce per-user upload quota and global disk threshold
	isSuperAdmin := false
	for _, a := range claims.Accesses {
		if a == string(models.AccessSuperAdmin) {
			isSuperAdmin = true
			break
		}
	}
	settings, err := h.settingsRepo.GetEffectiveSettings()
	if err != nil {
		log.Error().Err(err).Msg("Failed to get settings for upload quota check")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to verify upload quota"})
	}

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Missing file field"})
	}
	if file.Size > h.uploadMax {
		return c.Status(413).JSON(fiber.Map{"error": "File too large"})
	}

	if !isSuperAdmin && settings.MaxUploadBytesPerUser > 0 {
		userBytes, err := h.uploadTracker.GetUserUploadBytes(claims.UserID)
		if err != nil {
			log.Error().Err(err).Msg("Failed to query user upload quota")
			return c.Status(500).JSON(fiber.Map{"error": "Failed to verify upload quota"})
		}
		if userBytes+file.Size > settings.MaxUploadBytesPerUser {
			return c.Status(507).JSON(fiber.Map{
				"error": fmt.Sprintf("Upload quota exceeded (max %d bytes per user)", settings.MaxUploadBytesPerUser),
			})
		}
	}

	if settings.GlobalDiskThresholdBytes > 0 {
		totalBytes, err := h.uploadTracker.GetTotalUploadBytes()
		if err != nil {
			log.Error().Err(err).Msg("Failed to query global upload threshold")
		} else if totalBytes+file.Size > settings.GlobalDiskThresholdBytes {
			return c.Status(507).JSON(fiber.Map{
				"error": "Server storage limit reached, please try again later",
			})
		}
	}

	f, err := file.Open()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to read upload"})
	}
	defer f.Close()

	// Read into memory (limited to uploadMax, already checked above).
	data := make([]byte, file.Size)
	if _, err := f.Read(data); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to read upload"})
	}

	// Validate image dimensions
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(data)); err == nil {
		const maxDim = 8192
		if cfg.Width > maxDim || cfg.Height > maxDim {
			return c.Status(400).JSON(fiber.Map{"error": fmt.Sprintf("Image dimensions too large (max %dx%d)", maxDim, maxDim)})
		}
	}
	// If DecodeConfig fails, it's potentially not an image; let the storage backend decide

	// For S3 backend with files above the inline threshold, enqueue async upload
	// instead of blocking the HTTP request on an S3 PUT.
	if h.uploadBackend == "s3" && int64(len(data)) > h.inlineMaxBytes {
		// Determine MIME type for the payload
		mime, err := storage.SniffMime(data)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"error": err.Error()})
		}

		payload := queue.ChatUploadS3Payload{
			Data:     base64.StdEncoding.EncodeToString(data),
			RoomID:   roomID,
			MimeType: mime,
			UserID:   claims.UserID,
		}
		if err := queue.Enqueue(context.Background(), database.GetDB(), "chat_upload_s3", payload); err != nil {
			log.Error().Err(err).Str("roomID", roomID).Msg("Failed to enqueue chat upload")
			return c.Status(500).JSON(fiber.Map{"error": "Failed to queue upload"})
		}

		log.Info().Str("roomID", roomID).Str("userID", claims.UserID).
			Int("bytes", len(data)).Str("mime", mime).
			Msg("chat upload enqueued for async S3 upload")
		return c.Status(fiber.StatusAccepted).JSON(fiber.Map{
			"status":   "upload_queued",
			"job_type": "chat_upload_s3",
			"size":     len(data),
			"mime":     mime,
		})
	}

	// Sync path for disk/hybrid/inline or small S3 files
	attachment, err := h.uploadStore.Store(data)
	if err != nil {
		log.Warn().Err(err).Str("roomId", roomID).Msg("Chat image upload failed")
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	if h.uploadTracker != nil {
		hash, ext, backend := parseUploadMeta(attachment.URL, attachment.Mime, data)
		if err := h.uploadTracker.Record(roomID, claims.UserID, hash, ext, attachment.Size, backend); err != nil {
			log.Warn().Err(err).Str("roomID", roomID).Str("hash", hash).Msg("failed to track chat upload")
		}
	}

	return c.JSON(attachment)
}

// AdminKickParticipant removes any participant from a room (no creator check).
func (h *RoomHandler) AdminKickParticipant(c *fiber.Ctx) error {
	roomID := c.Params("roomId")
	identity := c.Params("identity")
	claims := c.Locals("user").(*auth.Claims)
	room, _ := h.roomRepo.GetRoom(roomID)
	if room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})
	_, err := h.client.RemoveParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room.Name, Identity: identity})
	if err != nil {
		log.Error().Err(err).Msg("LK RemoveParticipant failed (admin kick)")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to remove participant"})
	}
	h.sendSystemMessage(ctx, room.Name, "kick", claims.UserID, identity)
	_ = h.roomRepo.RemoveParticipant(room.ID, identity)
	return c.JSON(fiber.Map{"status": "success"})
}

// AdminMuteParticipant mutes all audio tracks for a participant.
func (h *RoomHandler) AdminMuteParticipant(c *fiber.Ctx) error {
	roomID := c.Params("roomId")
	identity := c.Params("identity")
	room, _ := h.roomRepo.GetRoom(roomID)
	if room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}
	ctx := h.withAuth(c.Context(), &lkauth.VideoGrant{RoomAdmin: true, Room: room.Name})

	// Get participant to find their audio track SIDs
	p, err := h.client.GetParticipant(ctx, &livekit.RoomParticipantIdentity{Room: room.Name, Identity: identity})
	if err != nil {
		return c.Status(404).JSON(fiber.Map{"error": "Participant not found"})
	}
	for _, track := range p.Tracks {
		if track.Type == livekit.TrackType_AUDIO {
			if _, err := h.client.MutePublishedTrack(ctx, &livekit.MuteRoomTrackRequest{
				Room:     room.Name,
				Identity: identity,
				TrackSid: track.Sid,
				Muted:    true,
			}); err != nil {
				log.Warn().Err(err).Str("room", room.Name).Str("identity", identity).Str("track", track.Sid).Msg("Failed to mute audio track (admin)")
			}
		}
	}
	return c.JSON(fiber.Map{"status": "success"})
}

// ListRoomEvents returns paginated room events.
// @Summary List room events
// @Description Get a paginated list of room activity events (room created, user joined) (requires superadmin access)
// @Tags admin
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param page query int false "Page number" default(1)
// @Param limit query int false "Items per page" default(50)
// @Param q query string false "Search by room name or user name"
// @Param type query string false "Comma-separated event types: room_created,room_joined"
// @Param dateFrom query string false "Start date (YYYY-MM-DD)"
// @Param dateTo query string false "End date (YYYY-MM-DD)"
// @Param order query string false "Sort direction: asc, desc" default(desc)
// @Success 200 {object} map[string]interface{} "{events, total, page, limit}"
// @Failure 400 {object} ErrorResponse "Invalid parameters"
// @Failure 401 {object} ErrorResponse "Unauthorized"
// @Failure 403 {object} ErrorResponse "Forbidden"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /admin/rooms/events [get]
func (h *RoomHandler) ListRoomEvents(c *fiber.Ctx) error {
	p := repository.RoomEventsFilterParams{
		Page:   c.QueryInt("page", 1),
		Limit:  c.QueryInt("limit", 50),
		Search: c.Query("q"),
	}

	// Parse event types
	if types := c.Query("type"); types != "" {
		p.Types = strings.Split(types, ",")
		validTypes := map[string]bool{"room_created": true, "room_joined": true}
		for _, v := range p.Types {
			if !validTypes[v] {
				return c.Status(400).JSON(fiber.Map{"error": "Invalid event type: " + v})
			}
		}
	}

	// Date range — validate format
	p.DateFrom = c.Query("dateFrom")
	p.DateTo = c.Query("dateTo")
	if p.DateFrom != "" {
		if _, err := time.Parse("2006-01-02", p.DateFrom); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid dateFrom format, expected YYYY-MM-DD"})
		}
	}
	if p.DateTo != "" {
		if _, err := time.Parse("2006-01-02", p.DateTo); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid dateTo format, expected YYYY-MM-DD"})
		}
	}

	// Order
	p.Order = c.Query("order", "desc")
	if p.Order != "asc" && p.Order != "desc" {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid order, must be asc or desc"})
	}

	// Clamp limit
	if p.Limit <= 0 || p.Limit > 100 {
		p.Limit = 50
	}

	// Clamp page
	if p.Page <= 0 {
		p.Page = 1
	}

	events, total, err := h.roomRepo.GetRoomEventsFiltered(p)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch room events"})
	}

	return c.JSON(fiber.Map{
		"events": events,
		"total":  total,
		"page":   p.Page,
		"limit":  p.Limit,
	})
}

// detectUploadBackend classifies a chat upload URL into a storage backend type.
// Used to populate ChatUpload.StorageBackend for cleanup routing.
func detectUploadBackend(url string) string {
	if strings.HasPrefix(url, "data:") {
		return "inline"
	}
	if strings.HasPrefix(url, "/uploads/chat/") {
		return "disk"
	}
	return "s3"
}

// mimeExtension returns the file extension for an allowed image MIME type.
func mimeExtension(mime string) string {
	switch mime {
	case "image/png":
		return ".png"
	case "image/jpeg":
		return ".jpg"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	}
	return ""
}

// parseUploadMeta extracts the content hash, file extension, and storage backend
// from a chat upload result. For inline uploads, hash is computed from data
// since the URL doesn't contain a filename.
// BulkSuspendRooms enqueues suspension for multiple rooms.
// Reports per-ID errors: "room not found", "already suspended", or queue failures.
// @Summary Bulk suspend rooms
// @Tags admin
// @Accept json
// @Produce json
// @Param request body BulkIDsRequest true "Room IDs to suspend"
// @Success 202 {object} BulkResult
// @Router /admin/rooms/bulk/suspend [post]
func (h *RoomHandler) BulkSuspendRooms(c *fiber.Ctx) error {
	var req BulkIDsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	if len(req.IDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No room IDs provided"})
	}
	if len(req.IDs) > 500 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Maximum 500 rooms per request"})
	}

	rooms, err := h.roomRepo.GetRoomsByIDs(req.IDs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch rooms"})
	}

	roomByID := make(map[string]*models.Room, len(rooms))
	for _, r := range rooms {
		roomByID[r.ID] = &r
	}

	results := make(map[string]BulkItemResult, len(req.IDs))
	processed := 0
	failed := 0

	for _, id := range req.IDs {
		r, found := roomByID[id]
		if !found {
			results[id] = BulkItemResult{Success: false, Error: "room not found"}
			failed++
			continue
		}
		if !r.IsActive {
			results[id] = BulkItemResult{Success: true, Name: r.Name}
			processed++
			continue
		}
		payload := queue.RoomSuspendPayload{RoomID: r.ID}
		if err := queue.Enqueue(context.Background(), database.GetDB(), "room_suspend", payload,
			queue.WithPriority(2), queue.WithMaxAttempts(3)); err != nil {
			results[id] = BulkItemResult{Success: false, Name: r.Name, Error: err.Error()}
			failed++
		} else {
			results[id] = BulkItemResult{Success: true, Name: r.Name}
			processed++
		}
	}

	return c.Status(fiber.StatusAccepted).JSON(BulkResult{
		Results:        results,
		TotalProcessed: processed,
		TotalFailed:    failed,
	})
}

// BulkCloseRooms enqueues deletion for multiple rooms.
// Reports per-ID errors: "room not found" or queue failures.
// @Summary Bulk close rooms
// @Tags admin
// @Accept json
// @Produce json
// @Param request body BulkIDsRequest true "Room IDs to close"
// @Success 202 {object} BulkResult
// @Router /admin/rooms/bulk/close [post]
func (h *RoomHandler) BulkCloseRooms(c *fiber.Ctx) error {
	var req BulkIDsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}
	if len(req.IDs) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "No room IDs provided"})
	}
	if len(req.IDs) > 500 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Maximum 500 rooms per request"})
	}

	rooms, err := h.roomRepo.GetRoomsByIDs(req.IDs)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch rooms"})
	}

	roomByID := make(map[string]*models.Room, len(rooms))
	for _, r := range rooms {
		roomByID[r.ID] = &r
	}

	results := make(map[string]BulkItemResult, len(req.IDs))
	processed := 0
	failed := 0

	for _, id := range req.IDs {
		r, found := roomByID[id]
		if !found {
			results[id] = BulkItemResult{Success: false, Error: "room not found"}
			failed++
			continue
		}
		payload := queue.RoomDeletePayload{
			RoomID:        r.ID,
			SystemEvent:   "room_ended",
			SystemMessage: "Room closed by administrator",
		}
		if err := queue.Enqueue(context.Background(), database.GetDB(), "room_delete", payload,
			queue.WithPriority(1), queue.WithMaxAttempts(3)); err != nil {
			results[id] = BulkItemResult{Success: false, Name: r.Name, Error: err.Error()}
			failed++
		} else {
			results[id] = BulkItemResult{Success: true, Name: r.Name}
			processed++
		}
	}

	return c.Status(fiber.StatusAccepted).JSON(BulkResult{
		Results:        results,
		TotalProcessed: processed,
		TotalFailed:    failed,
	})
}

func parseUploadMeta(url, mime string, data []byte) (hash, ext, backend string) {
	backend = detectUploadBackend(url)
	switch backend {
	case "disk":
		filename := strings.TrimPrefix(url, "/uploads/chat/")
		if dotIdx := strings.LastIndex(filename, "."); dotIdx > 0 {
			return filename[:dotIdx], filename[dotIdx:], backend
		}
		return filename, "", backend
	case "s3":
		if idx := strings.LastIndex(url, "/"); idx >= 0 {
			filename := url[idx+1:]
			if dotIdx := strings.LastIndex(filename, "."); dotIdx > 0 {
				return filename[:dotIdx], filename[dotIdx:], backend
			}
			return filename, "", backend
		}
		return "", "", backend
	default: // inline
		if len(data) > 0 {
			h := sha256.Sum256(data)
			return hex.EncodeToString(h[:]), mimeExtension(mime), backend
		}
		return "", mimeExtension(mime), backend
	}
}
