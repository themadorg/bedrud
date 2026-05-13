package lkutil

import (
	"bedrud/config"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	lkauth "github.com/livekit/protocol/auth"
	"github.com/livekit/protocol/livekit"
	"github.com/twitchtv/twirp"
)

func NewClient(lkCfg *config.LiveKitConfig) livekit.RoomService {
	apiHost := lkCfg.InternalHost
	if apiHost == "" {
		apiHost = lkCfg.Host
	}
	httpClient := http.DefaultClient
	if lkCfg.SkipTLSVerify && strings.HasPrefix(apiHost, "https") {
		httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		}
	}
	return livekit.NewRoomServiceProtobufClient(apiHost, httpClient)
}

func AuthContext(ctx context.Context, apiKey, apiSecret string, grants ...*lkauth.VideoGrant) (context.Context, error) {
	at := lkauth.NewAccessToken(apiKey, apiSecret)
	for _, g := range grants {
		at.AddGrant(g) //nolint:staticcheck // AddGrant is deprecated but VideoGrant field is not available in this version of the protocol SDK
	}
	token, err := at.ToJWT()
	if err != nil {
		return ctx, fmt.Errorf("failed to generate LiveKit auth token: %w", err)
	}
	ctx, err = twirp.WithHTTPRequestHeaders(ctx, http.Header{
		"Authorization": []string{"Bearer " + token},
	})
	if err != nil {
		return ctx, fmt.Errorf("failed to set LiveKit auth headers: %w", err)
	}
	return ctx, nil
}

type SystemMessage struct {
	Type            string `json:"type"`
	Event           string `json:"event"`
	Message         string `json:"message"`
	DeletedIdentity string `json:"deletedIdentity,omitempty"`
}

func SendSystemMessage(ctx context.Context, client livekit.RoomService, roomName, event, message string) {
	b, _ := json.Marshal(SystemMessage{Type: "system", Event: event, Message: message})
	topic := "system"
	_, _ = client.SendData(ctx, &livekit.SendDataRequest{
		Room:  roomName,
		Data:  b,
		Kind:  livekit.DataPacket_RELIABLE,
		Topic: &topic,
	})
}

func SendSystemMessageWithDeletedIdentity(ctx context.Context, client livekit.RoomService, roomName, event, message, deletedIdentity string) {
	b, _ := json.Marshal(SystemMessage{
		Type:            "system",
		Event:           event,
		Message:         message,
		DeletedIdentity: deletedIdentity,
	})
	topic := "system"
	_, _ = client.SendData(ctx, &livekit.SendDataRequest{
		Room:  roomName,
		Data:  b,
		Kind:  livekit.DataPacket_RELIABLE,
		Topic: &topic,
	})
}
