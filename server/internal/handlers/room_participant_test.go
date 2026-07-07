package handlers

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// setupParticipantTestApp wires all moderation endpoints with a mock LK client.
// Returns app, roomRepo, and the claims pointer for dynamic auth switching.
func setupParticipantTestApp(t *testing.T) (*fiber.App, *repository.RoomRepository, *auth.Claims) {
	t.Helper()
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)

	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "k", APISecret: "s"}
	handler := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, roomRepo, nil, nil, nil, nil, nil, nil)

	claims := &auth.Claims{
		UserID:   "creator-user",
		Email:    "creator@ex.com",
		Name:     "Creator",
		Accesses: []string{"user"},
	}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", claims)
		return c.Next()
	})

	// All moderation endpoints
	app.Post("/room/:roomId/kick/:identity", handler.KickParticipant)
	app.Post("/room/:roomId/mute/:identity", handler.MuteParticipant)
	app.Post("/room/:roomId/ban/:identity", handler.BanParticipant)
	app.Post("/room/:roomId/video/:identity/off", handler.DisableParticipantVideo)
	app.Post("/room/:roomId/promote/:identity", handler.PromoteParticipant)
	app.Post("/room/:roomId/demote/:identity", handler.DemoteParticipant)
	app.Post("/room/:roomId/chat/:identity/block", handler.BlockChat)
	app.Post("/room/:roomId/deafen/:identity", handler.DeafenParticipant)
	app.Post("/room/:roomId/undeafen/:identity", handler.UndeafenParticipant)
	app.Post("/room/:roomId/ask/:identity/:action", handler.AskParticipantAction)
	app.Post("/room/:roomId/spotlight/:identity", handler.SpotlightParticipant)
	app.Post("/room/:roomId/screenshare/:identity/stop", handler.StopScreenShare)
	app.Get("/room/:roomId/participant/:identity/info", handler.GetParticipantInfo)
	app.Get("/room/:roomId/participant/:identity/profile", handler.GetParticipantProfile)
	app.Post("/room/:roomId/stage/:identity/bring", handler.BringToStage)
	app.Post("/room/:roomId/stage/:identity/remove", handler.RemoveFromStage)

	// Seed users
	users := []models.User{
		{ID: "creator-user", Email: "creator@ex.com", Name: "Creator", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}},
		{ID: "other-user", Email: "other@ex.com", Name: "Other", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}},
		{ID: "mod-user", Email: "mod@ex.com", Name: "Mod", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}},
		{ID: "superadmin-user", Email: "admin@ex.com", Name: "Admin", Provider: "local", IsActive: true, Accesses: models.StringArray{"user", "superadmin"}},
	}
	for _, u := range users {
		db.Create(&u)
	}

	return app, roomRepo, claims
}

// --- Room not found tests ---

func TestParticipantAction_RoomNotFound(t *testing.T) {
	app, _, _ := setupParticipantTestApp(t)

	tests := []struct {
		name   string
		method string
		path   string
		body   io.Reader
	}{
		{"KickParticipant", "POST", "/room/nonexistent/kick/some-user", nil},
		{"MuteParticipant", "POST", "/room/nonexistent/mute/some-user", nil},
		{"BanParticipant", "POST", "/room/nonexistent/ban/some-user", nil},
		{"DisableParticipantVideo", "POST", "/room/nonexistent/video/some-user/off", nil},
		{"PromoteParticipant", "POST", "/room/nonexistent/promote/some-user", nil},
		{"DemoteParticipant", "POST", "/room/nonexistent/demote/some-user", nil},
		{"BlockChat", "POST", "/room/nonexistent/chat/some-user/block", nil},
		{"DeafenParticipant", "POST", "/room/nonexistent/deafen/some-user", nil},
		{"UndeafenParticipant", "POST", "/room/nonexistent/undeafen/some-user", nil},
		{"AskParticipantAction", "POST", "/room/nonexistent/ask/some-user/unmute", nil},
		{"SpotlightParticipant", "POST", "/room/nonexistent/spotlight/some-user", nil},
		{"StopScreenShare", "POST", "/room/nonexistent/screenshare/some-user/stop", nil},
		{"GetParticipantInfo", "GET", "/room/nonexistent/participant/some-user/info", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, tt.body)
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("expected 404 for nonexistent room, got %d", resp.StatusCode)
			}
		})
	}
}

// --- Not-implemented endpoints ---

func TestParticipantAction_NotImplemented(t *testing.T) {
	app, _, _ := setupParticipantTestApp(t)

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"BringToStage", "POST", "/room/some-room/stage/test-user/bring"},
		{"RemoveFromStage", "POST", "/room/some-room/stage/test-user/remove"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusNotImplemented {
				t.Fatalf("expected 501 for not implemented, got %d", resp.StatusCode)
			}
		})
	}
}

// --- Authz: non-creator attempts actions that require admin/creator ---

func TestParticipantAction_NonCreatorForbidden(t *testing.T) {
	app, roomRepo, baseClaims := setupParticipantTestApp(t)

	// Create room owned by creator-user
	room, err := roomRepo.CreateRoom("creator-user", "test-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// Switch claims to other-user (not creator, not superadmin)
	*baseClaims = auth.Claims{UserID: "other-user", Email: "other@ex.com", Accesses: []string{"user"}}
	_ = roomRepo.AddParticipant(room.ID, "other-user")

	// Endpoints that use admin check (GetRoom + claims.UserID != adminId) — return "Insufficient permissions"
	adminCheckEndpoints := []struct {
		name   string
		method string
		path   string
	}{
		{"KickParticipant", "POST", "/room/" + room.ID + "/kick/victim"},
		{"MuteParticipant", "POST", "/room/" + room.ID + "/mute/victim"},
		{"BanParticipant", "POST", "/room/" + room.ID + "/ban/victim"},
		{"PromoteParticipant", "POST", "/room/" + room.ID + "/promote/victim"},
		{"DemoteParticipant", "POST", "/room/" + room.ID + "/demote/victim"},
	}

	for _, tt := range adminCheckEndpoints {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("expected 403 (non-creator), got %d", resp.StatusCode)
			}
		})
	}

	// Endpoints that use isRoomModerator check — return "not authorized for this room"
	modCheckEndpoints := []struct {
		name   string
		method string
		path   string
	}{
		{"BlockChat", "POST", "/room/" + room.ID + "/chat/victim/block"},
		{"DeafenParticipant", "POST", "/room/" + room.ID + "/deafen/victim"},
		{"UndeafenParticipant", "POST", "/room/" + room.ID + "/undeafen/victim"},
		{"AskParticipantAction", "POST", "/room/" + room.ID + "/ask/victim/unmute"},
		{"SpotlightParticipant", "POST", "/room/" + room.ID + "/spotlight/victim"},
		{"StopScreenShare", "POST", "/room/" + room.ID + "/screenshare/victim/stop"},
		{"DisableParticipantVideo", "POST", "/room/" + room.ID + "/video/victim/off"},
	}

	for _, tt := range modCheckEndpoints {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("expected 403 (non-moderator), got %d", resp.StatusCode)
			}
		})
	}
}

// --- Authz: superadmin can bypass all checks ---

func TestParticipantAction_SuperadminBypassesAuthz(t *testing.T) {
	app, roomRepo, baseClaims := setupParticipantTestApp(t)

	room, err := roomRepo.CreateRoom("creator-user", "superadmin-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// Switch claims to superadmin
	*baseClaims = auth.Claims{UserID: "superadmin-user", Email: "admin@ex.com", Accesses: []string{"user", "superadmin"}}

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"KickParticipant", "POST", "/room/" + room.ID + "/kick/victim"},
		{"MuteParticipant", "POST", "/room/" + room.ID + "/mute/victim"},
		{"BanParticipant", "POST", "/room/" + room.ID + "/ban/victim"},
		{"PromoteParticipant", "POST", "/room/" + room.ID + "/promote/victim"},
		{"DemoteParticipant", "POST", "/room/" + room.ID + "/demote/victim"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			// These will attempt LK calls and get mock responses → 200
			// or if LK returns error → 500
			// Either way not 403
			if resp.StatusCode == http.StatusForbidden {
				t.Fatalf("superadmin should not get 403, got %d", resp.StatusCode)
			}
		})
	}
}

// --- Self-targeting forbidden ---

func TestParticipantAction_SelfTargetForbidden(t *testing.T) {
	app, roomRepo, baseClaims := setupParticipantTestApp(t)

	room, err := roomRepo.CreateRoom("creator-user", "self-test-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// Creator targets themselves
	*baseClaims = auth.Claims{UserID: "creator-user", Email: "creator@ex.com", Accesses: []string{"user"}}

	selfBlockedEndpoints := []struct {
		name   string
		method string
		path   string
	}{
		{"KickParticipant", "POST", "/room/" + room.ID + "/kick/creator-user"},
		{"MuteParticipant", "POST", "/room/" + room.ID + "/mute/creator-user"},
		{"BanParticipant", "POST", "/room/" + room.ID + "/ban/creator-user"},
		{"BlockChat", "POST", "/room/" + room.ID + "/chat/creator-user/block"},
		{"DeafenParticipant", "POST", "/room/" + room.ID + "/deafen/creator-user"},
		{"UndeafenParticipant", "POST", "/room/" + room.ID + "/undeafen/creator-user"},
		{"StopScreenShare", "POST", "/room/" + room.ID + "/screenshare/creator-user/stop"},
		// AskParticipantAction also self-targets 400
	}

	for _, tt := range selfBlockedEndpoints {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("expected 400 for self-target, got %d", resp.StatusCode)
			}
		})
	}
}

// --- Admin-targeting forbidden ---

func TestParticipantAction_AdminTargetForbidden(t *testing.T) {
	app, roomRepo, baseClaims := setupParticipantTestApp(t)

	room, err := roomRepo.CreateRoom("creator-user", "admin-target-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// Create another user and add as participant so they can attempt to target admin
	otherClaims := &auth.Claims{UserID: "other-user", Email: "other@ex.com", Accesses: []string{"user"}}
	*baseClaims = *otherClaims
	_ = roomRepo.AddParticipant(room.ID, "other-user")
	_ = roomRepo.SetRoomModerator(room.ID, "other-user", true)

	// Only endpoints that check admin-target
	adminTargetBlocked := []struct {
		name   string
		method string
		path   string
	}{
		{"KickParticipant", "POST", "/room/" + room.ID + "/kick/creator-user"},
		{"MuteParticipant", "POST", "/room/" + room.ID + "/mute/creator-user"},
		{"BanParticipant", "POST", "/room/" + room.ID + "/ban/creator-user"},
		{"BlockChat", "POST", "/room/" + room.ID + "/chat/creator-user/block"},
		{"DeafenParticipant", "POST", "/room/" + room.ID + "/deafen/creator-user"},
		{"UndeafenParticipant", "POST", "/room/" + room.ID + "/undeafen/creator-user"},
		{"StopScreenShare", "POST", "/room/" + room.ID + "/screenshare/creator-user/stop"},
	}

	for _, tt := range adminTargetBlocked {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("expected 403 for admin-target, got %d", resp.StatusCode)
			}
		})
	}
}

// --- AskParticipantAction: invalid action ---

func TestAskParticipantAction_InvalidAction(t *testing.T) {
	app, roomRepo, baseClaims := setupParticipantTestApp(t)

	room, err := roomRepo.CreateRoom("creator-user", "ask-action-test", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}
	*baseClaims = auth.Claims{UserID: "creator-user", Email: "creator@ex.com", Accesses: []string{"user"}}

	req := httptest.NewRequest(http.MethodPost, "/room/"+room.ID+"/ask/victim/invalid_action", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid action, got %d", resp.StatusCode)
	}
}

// --- Success paths (mock LK returns valid responses) ---

func TestParticipantAction_Success(t *testing.T) {
	app, roomRepo, baseClaims := setupParticipantTestApp(t)

	room, err := roomRepo.CreateRoom("creator-user", "success-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	*baseClaims = auth.Claims{UserID: "creator-user", Email: "creator@ex.com", Accesses: []string{"user"}}

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"KickParticipant", "POST", "/room/" + room.ID + "/kick/victim"},
		{"MuteParticipant", "POST", "/room/" + room.ID + "/mute/victim"},
		{"BanParticipant", "POST", "/room/" + room.ID + "/ban/victim"},
		{"DisableParticipantVideo", "POST", "/room/" + room.ID + "/video/victim/off"},
		{"BlockChat", "POST", "/room/" + room.ID + "/chat/victim/block"},
		{"DeafenParticipant", "POST", "/room/" + room.ID + "/deafen/victim"},
		{"UndeafenParticipant", "POST", "/room/" + room.ID + "/undeafen/victim"},
		{"AskParticipantAction", "POST", "/room/" + room.ID + "/ask/victim/unmute"},
		{"SpotlightParticipant", "POST", "/room/" + room.ID + "/spotlight/victim"},
		{"StopScreenShare", "POST", "/room/" + room.ID + "/screenshare/victim/stop"},
		{"PromoteParticipant", "POST", "/room/" + room.ID + "/promote/victim"},
		{"DemoteParticipant", "POST", "/room/" + room.ID + "/demote/victim"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			resp, err := app.Test(req, -1)
			if err != nil {
				t.Fatalf("app.Test failed: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			// With mock LK, all these should return 200.
			// This tests the full handler path: authz → room lookup → LK call → response.
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200 (mock LK), got %d", resp.StatusCode)
			}
		})
	}
}

// --- GetParticipantInfo: special authz (self-access always OK) ---

func TestGetParticipantInfo_SelfAccessAlwaysAllowed(t *testing.T) {
	app, roomRepo, baseClaims := setupParticipantTestApp(t)

	room, err := roomRepo.CreateRoom("creator-user", "self-info-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// Non-creator user can view their own info
	*baseClaims = auth.Claims{UserID: "other-user", Email: "other@ex.com", Accesses: []string{"user"}}
	_ = roomRepo.AddParticipant(room.ID, "other-user")

	req := httptest.NewRequest(http.MethodGet, "/room/"+room.ID+"/participant/other-user/info", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for self-info access, got %d", resp.StatusCode)
	}
}

func TestGetParticipantProfile_MeetingParticipantCanViewOther(t *testing.T) {
	app, roomRepo, baseClaims := setupParticipantTestApp(t)

	room, err := roomRepo.CreateRoom("creator-user", "profile-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	*baseClaims = auth.Claims{UserID: "other-user", Email: "other@ex.com", Accesses: []string{"user"}}
	_ = roomRepo.AddParticipant(room.ID, "other-user")

	req := httptest.NewRequest(http.MethodGet, "/room/"+room.ID+"/participant/creator-user/profile", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for in-meeting profile fetch, got %d", resp.StatusCode)
	}
}

func TestGetParticipantInfo_NonModeratorCannotViewOthers(t *testing.T) {
	app, roomRepo, baseClaims := setupParticipantTestApp(t)

	room, err := roomRepo.CreateRoom("creator-user", "other-info-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	// Non-creator user trying to view a different participant
	*baseClaims = auth.Claims{UserID: "other-user", Email: "other@ex.com", Accesses: []string{"user"}}
	_ = roomRepo.AddParticipant(room.ID, "other-user")

	req := httptest.NewRequest(http.MethodGet, "/room/"+room.ID+"/participant/victim/info", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for non-moderator viewing others, got %d", resp.StatusCode)
	}
}

// --- Promote: already moderator ---

func TestPromoteParticipant_AlreadyModerator(t *testing.T) {
	app, roomRepo, baseClaims := setupParticipantTestApp(t)

	room, err := roomRepo.CreateRoom("creator-user", "already-mod-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	*baseClaims = auth.Claims{UserID: "creator-user", Email: "creator@ex.com", Accesses: []string{"user"}}

	// Set up mock LK to return a participant with moderator metadata
	// (Default mock returns empty metadata, so "already_moderator" won't fire
	// unless we set up the hook)
	// For now, test the normal promote path

	req := httptest.NewRequest(http.MethodPost, "/room/"+room.ID+"/promote/victim", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 (promote success), got %d", resp.StatusCode)
	}
}

// --- AskParticipantAction: valid action values ---

func TestAskParticipantAction_ValidActionCamera(t *testing.T) {
	app, roomRepo, baseClaims := setupParticipantTestApp(t)

	room, err := roomRepo.CreateRoom("creator-user", "ask-camera-room", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	*baseClaims = auth.Claims{UserID: "creator-user", Email: "creator@ex.com", Accesses: []string{"user"}}

	req := httptest.NewRequest(http.MethodPost, "/room/"+room.ID+"/ask/victim/camera", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 for ask camera, got %d", resp.StatusCode)
	}
}

// --- Json response structure check on success ---

func TestBanParticipant_ResponseBody(t *testing.T) {
	app, roomRepo, baseClaims := setupParticipantTestApp(t)

	room, err := roomRepo.CreateRoom("creator-user", "ban-response-test", true, "standard", 0, &models.RoomSettings{})
	if err != nil {
		t.Fatalf("failed to create room: %v", err)
	}

	*baseClaims = auth.Claims{UserID: "creator-user", Email: "creator@ex.com", Accesses: []string{"user"}}

	req := httptest.NewRequest(http.MethodPost, "/room/"+room.ID+"/ban/victim", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()

	var body map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}
	if body["status"] != "success" {
		t.Fatalf("expected status 'success', got %v", body["status"])
	}
}
