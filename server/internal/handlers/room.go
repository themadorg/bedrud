package handlers

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/lkutil"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/services"
	"bedrud/internal/storage"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	lkauth "github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"github.com/rs/zerolog/log"
)

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
	roomRepo      *repository.RoomRepository
	livekitHost   string
	apiKey        string
	apiSecret     string
	client        livekit.RoomService
	uploadStore   storage.ChatUploadStore
	uploadMax     int64
	uploadTracker *storage.ChatUploadTracker
	cleanupSvc    *services.RoomCleanupService
	settingsRepo  *repository.SettingsRepository
}

func NewRoomHandler(lkCfg *config.LiveKitConfig, chatCfg *config.ChatConfig, roomRepo *repository.RoomRepository, settingsRepo *repository.SettingsRepository, uploadTracker *storage.ChatUploadTracker, cleanupSvc *services.RoomCleanupService) *RoomHandler {
	client := lkutil.NewClient(lkCfg)

	uploadMax := chatCfg.Uploads.MaxBytes.Int64()
	if uploadMax == 0 {
		uploadMax = 10 * 1024 * 1024
	}

	return &RoomHandler{
		roomRepo:      roomRepo,
		livekitHost:   lkCfg.Host,
		apiKey:        lkCfg.APIKey,
		apiSecret:     lkCfg.APISecret,
		client:        client,
		uploadStore:   storage.NewChatUploadStore(&chatCfg.Uploads),
		uploadMax:     uploadMax,
		uploadTracker: uploadTracker,
		cleanupSvc:    cleanupSvc,
		settingsRepo:  settingsRepo,
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
	at.AddGrant(&lkauth.VideoGrant{RoomJoin: true, Room: req.RoomName, CanUpdateOwnMetadata: boolPtr(true)}).SetIdentity(claims.UserID).SetName(claims.Name).SetValidFor(time.Hour) //nolint:staticcheck // AddGrant is deprecated but VideoGrant field is not available in this version of the protocol SDK
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

	return c.JSON(fiber.Map{
		"id": room.ID, "name": room.Name, "token": token, "createdBy": room.CreatedBy, "adminId": adminId, "isActive": room.IsActive,
		"isPublic": room.IsPublic, "maxParticipants": room.MaxParticipants, "expiresAt": room.ExpiresAt,
		"settings": room.Settings, "livekitHost": h.livekitHost, "mode": room.Mode,
	})
}

type GuestJoinRoomRequest struct {
	RoomName  string `json:"roomName"`
	GuestName string `json:"guestName"`
}

func (h *RoomHandler) GuestJoinRoom(c *fiber.Ctx) error {
	var req GuestJoinRoomRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid body"})
	}
	req.RoomName = strings.ToLower(strings.TrimSpace(req.RoomName))
	req.GuestName = strings.TrimSpace(req.GuestName)
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

	return c.JSON(fiber.Map{
		"id": room.ID, "name": room.Name, "token": token, "adminId": adminId,
		"livekitHost": h.livekitHost,
	})
}

func generateShortID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	const charLen = len(chars) // 36
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

	opts := services.CascadeDeleteOptions{
		SystemEvent:   "room_ended",
		SystemMessage: "The meeting has been ended by the creator",
	}
	if err := h.cleanupSvc.CascadeDeleteRoom(c.Context(), room, opts); err != nil {
		log.Error().Err(err).Str("roomId", roomID).Msg("Failed to delete room")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to delete room"})
	}

	return c.JSON(fiber.Map{"status": "success"})
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
	page := c.QueryInt("page", 1)
	limit := c.QueryInt("limit", 50)
	rooms, total, err := h.roomRepo.GetAllRoomsPaginated(repository.PaginationParams{Page: page, Limit: limit})
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch rooms"})
	}
	return c.JSON(fiber.Map{"rooms": rooms, "total": total, "page": page, "limit": limit})
}

func (h *RoomHandler) AdminGenerateToken(c *fiber.Ctx) error {
	return c.Status(fiber.StatusNotImplemented).JSON(fiber.Map{"error": "not yet implemented"})
}

func (h *RoomHandler) AdminCloseRoom(c *fiber.Ctx) error {
	roomID := c.Params("roomId")
	room, err := h.roomRepo.GetRoom(roomID)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to fetch room"})
	}
	if room == nil {
		return c.Status(404).JSON(fiber.Map{"error": "Room not found"})
	}
	opts := services.CascadeDeleteOptions{
		SystemEvent:   "room_closed",
		SystemMessage: "This room has been closed by an administrator",
	}
	if err := h.cleanupSvc.CascadeDeleteRoom(c.Context(), room, opts); err != nil {
		log.Error().Err(err).Str("roomId", roomID).Msg("Failed to close room")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to close room"})
	}
	return c.JSON(fiber.Map{"status": "success"})
}

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
	if err := h.cleanupSvc.SuspendRoom(c.Context(), room); err != nil {
		log.Error().Err(err).Str("roomId", roomID).Msg("Failed to suspend room")
		return c.Status(500).JSON(fiber.Map{"error": "Failed to suspend room"})
	}
	return c.JSON(fiber.Map{"status": "success"})
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

	file, err := c.FormFile("file")
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Missing file field"})
	}
	if file.Size > h.uploadMax {
		return c.Status(413).JSON(fiber.Map{"error": "File too large"})
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

	attachment, err := h.uploadStore.Store(data)
	if err != nil {
		log.Warn().Err(err).Str("roomId", roomID).Msg("Chat image upload failed")
		return c.Status(400).JSON(fiber.Map{"error": err.Error()})
	}

	if h.uploadTracker != nil && strings.HasPrefix(attachment.URL, "/uploads/chat/") {
		filename := strings.TrimPrefix(attachment.URL, "/uploads/chat/")
		dotIdx := strings.LastIndex(filename, ".")
		hash := filename
		ext := ""
		if dotIdx > 0 {
			hash = filename[:dotIdx]
			ext = filename[dotIdx:]
		}
		if hash != "" {
			if err := h.uploadTracker.Record(roomID, hash, ext); err != nil {
				log.Warn().Err(err).Str("roomID", roomID).Str("hash", hash).Msg("failed to track chat upload")
			}
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
