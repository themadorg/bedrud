package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"bedrud/config"
	"bedrud/internal/auth"
	"bedrud/internal/models"
	"bedrud/internal/repository"
	"bedrud/internal/testutil"

	"github.com/gofiber/fiber/v2"
	"github.com/livekit/protocol/livekit"
)

const (
	ensureRoomName            = "ensure-lk-room"
	ensureRoomMaxParticipants = 7
)

// postJoin sends a JSON body to a join route and returns the response status code.
func postJoin(t *testing.T, app *fiber.App, path string, body map[string]string) int {
	t.Helper()
	raw, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	return resp.StatusCode
}

// setupEnsureRoomTest wires the join routes to the given LiveKit mock and seeds a
// public room, so LiveKit closing it is the only thing standing in the way of a join.
func setupEnsureRoomTest(t *testing.T, lkMock *testutil.MockRoomService) (*fiber.App, *repository.RoomRepository, *models.Room) {
	t.Helper()
	// Guest join issues API tokens, which needs a JWT secret.
	config.SetForTest(&config.Config{
		Auth: config.AuthConfig{JWTSecret: "ensure-room-test-secret-32bytes!!", TokenDuration: 1},
	})
	claims := &auth.Claims{UserID: "ensure-owner", Email: "ensure@ex.com", Name: "Ensure", Accesses: []string{"user"}}
	app, roomRepo := setupJoinTestAppWithLiveKit(t, claims, lkMock)
	room, err := roomRepo.CreateRoom(claims.UserID, ensureRoomName, true, "standard", ensureRoomMaxParticipants, &models.RoomSettings{})
	if err != nil {
		t.Fatal(err)
	}
	// The repository enrolls the creator on creation; release it so every join starts from an empty room.
	if err := roomRepo.RemoveAllParticipants(room.ID); err != nil {
		t.Fatal(err)
	}
	return app, roomRepo, room
}

func TestJoinHandlers_EnsureLiveKitRoom(t *testing.T) {
	joinRoutes := []struct {
		name string
		path string
		body map[string]string
	}{
		{name: "JoinRoom", path: "/rooms/join", body: map[string]string{"roomName": ensureRoomName}},
		{name: "GuestJoinRoom", path: "/rooms/guest-join", body: map[string]string{"roomName": ensureRoomName, "guestName": "Guest"}},
	}

	for _, route := range joinRoutes {
		t.Run(route.name, func(t *testing.T) {
			t.Run("should create the LiveKit room with its max participants before issuing a token", func(t *testing.T) {
				lkMock := testutil.NewMockRoomService()
				var created *livekit.CreateRoomRequest
				lkMock.OnCreateRoom = func(_ context.Context, request *livekit.CreateRoomRequest) (*livekit.Room, error) {
					created = request
					return &livekit.Room{Name: request.Name}, nil
				}
				app, _, _ := setupEnsureRoomTest(t, lkMock)

				if status := postJoin(t, app, route.path, route.body); status != http.StatusOK {
					t.Fatalf("expected 200, got %d", status)
				}
				if created == nil {
					t.Fatal("expected the join to create the LiveKit room")
				}
				if created.Name != ensureRoomName || created.MaxParticipants != ensureRoomMaxParticipants {
					t.Fatalf("expected CreateRoom(%q, %d), got CreateRoom(%q, %d)",
						ensureRoomName, ensureRoomMaxParticipants, created.Name, created.MaxParticipants)
				}
			})

			t.Run("should fail the join without enrolling a participant when LiveKit is unavailable", func(t *testing.T) {
				lkMock := testutil.NewMockRoomService()
				lkMock.OnCreateRoom = func(context.Context, *livekit.CreateRoomRequest) (*livekit.Room, error) {
					return nil, errors.New("connection refused")
				}
				app, roomRepo, room := setupEnsureRoomTest(t, lkMock)

				if status := postJoin(t, app, route.path, route.body); status != http.StatusServiceUnavailable {
					t.Fatalf("expected 503, got %d", status)
				}
				count, err := roomRepo.GetParticipantCount(room.ID)
				if err != nil {
					t.Fatal(err)
				}
				if count != 0 {
					t.Fatalf("expected no participant enrolled, got %d", count)
				}
			})
		})
	}
}
