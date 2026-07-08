package handlers

import (
	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/livekit/protocol/livekit"
)

// TestCreateRoom_DBFailure_CompensatesLK verifies that when DB CreateRoom fails
// after LK CreateRoom succeeds, the handler calls LK DeleteRoom as compensating action.
func TestCreateRoom_DBFailure_CompensatesLK(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)

	lkMock := testutil.NewMockRoomService()
	lkCfg := config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "k", APISecret: "s"}
	handler := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, roomRepo, nil, nil, settingsRepo, nil, nil, nil)

	claims := &auth.Claims{UserID: "creator-user", Email: "creator@ex.com", Name: "Creator", Accesses: []string{"user"}}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", claims)
		return c.Next()
	})
	app.Post("/room/create", handler.CreateRoom)

	db.Create(&models.User{ID: "creator-user", Email: "creator@ex.com", Name: "Creator", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})

	// First creation succeeds
	body1 := `{"name":"dup-room"}`
	req1 := httptest.NewRequest(http.MethodPost, "/room/create", strings.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	resp1, err := app.Test(req1, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp1.Body.Close() }()
	if resp1.StatusCode != http.StatusOK {
		t.Fatalf("first create should succeed, got %d", resp1.StatusCode)
	}

	// Second creation with same name triggers DB unique constraint error
	// LK CreateRoom succeeds (mock), but DB CreateRoom fails (duplicate)
	// Handler should compensate by calling LK DeleteRoom
	beforeCalls := lkMock.DeleteRoomCalls.Load()
	body2 := `{"name":"dup-room"}`
	req2 := httptest.NewRequest(http.MethodPost, "/room/create", strings.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	resp2, err := app.Test(req2, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp2.Body.Close() }()
	if resp2.StatusCode != http.StatusConflict {
		bodyBytes, _ := io.ReadAll(resp2.Body)
		t.Fatalf("expected 409 for duplicate, got %d: %s", resp2.StatusCode, string(bodyBytes))
	}

	afterCalls := lkMock.DeleteRoomCalls.Load()
	if afterCalls <= beforeCalls {
		t.Fatal("expected LK DeleteRoom to be called as compensating action on DB failure")
	}
}

// TestCreateRoom_LKFailure_NoCompensatingActionNeeded verifies that when LK CreateRoom
// fails, the handler returns 500 and does NOT call DeleteRoom (nothing to clean).
func TestCreateRoom_LKFailure_NoCompensatingActionNeeded(t *testing.T) {
	db := testutil.SetupTestDB(t)
	roomRepo := repository.NewRoomRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)

	lkMock := testutil.NewMockRoomService()
	lkMock.OnCreateRoom = func(ctx context.Context, req *livekit.CreateRoomRequest) (*livekit.Room, error) {
		return nil, io.ErrUnexpectedEOF // simulate LK failure
	}
	lkCfg := config.LiveKitConfig{Host: "http://localhost:9999", APIKey: "k", APISecret: "s"}
	handler := NewRoomHandler(lkMock, &lkCfg, &config.ChatConfig{}, roomRepo, nil, nil, settingsRepo, nil, nil, nil)

	claims := &auth.Claims{UserID: "creator-user", Email: "creator@ex.com", Name: "Creator", Accesses: []string{"user"}}

	app := fiber.New()
	app.Use(func(c *fiber.Ctx) error {
		c.Locals("user", claims)
		return c.Next()
	})
	app.Post("/room/create", handler.CreateRoom)

	db.Create(&models.User{ID: "creator-user", Email: "creator@ex.com", Name: "Creator", Provider: "local", IsActive: true, Accesses: models.StringArray{"user"}})

	body := `{"name":"lk-fail-room"}`
	req := httptest.NewRequest(http.MethodPost, "/room/create", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusInternalServerError {
		t.Fatalf("expected 500 for LK failure, got %d", resp.StatusCode)
	}

	// DeleteRoom should NOT be called since LK CreateRoom never succeeded
	if calls := lkMock.DeleteRoomCalls.Load(); calls > 0 {
		t.Fatalf("expected 0 DeleteRoom calls (no LK room to clean), got %d", calls)
	}
}
