package testutil

import (
	"context"
	"sync/atomic"

	"github.com/livekit/protocol/livekit"
)

// MockRoomService implements livekit.RoomService for testing.
// Each method returns zero-value responses unless overridden.
// Use RoomServiceHooks to control specific method behavior.
type MockRoomService struct {
	// Hooks — set before test to control behavior per-method
	OnCreateRoom          func(ctx context.Context, req *livekit.CreateRoomRequest) (*livekit.Room, error)
	OnListRooms           func(ctx context.Context, req *livekit.ListRoomsRequest) (*livekit.ListRoomsResponse, error)
	OnDeleteRoom          func(ctx context.Context, req *livekit.DeleteRoomRequest) (*livekit.DeleteRoomResponse, error)
	OnListParticipants    func(ctx context.Context, req *livekit.ListParticipantsRequest) (*livekit.ListParticipantsResponse, error)
	OnGetParticipant      func(ctx context.Context, req *livekit.RoomParticipantIdentity) (*livekit.ParticipantInfo, error)
	OnRemoveParticipant   func(ctx context.Context, req *livekit.RoomParticipantIdentity) (*livekit.RemoveParticipantResponse, error)
	OnMutePublishedTrack  func(ctx context.Context, req *livekit.MuteRoomTrackRequest) (*livekit.MuteRoomTrackResponse, error)
	OnUpdateParticipant   func(ctx context.Context, req *livekit.UpdateParticipantRequest) (*livekit.ParticipantInfo, error)
	OnUpdateSubscriptions func(ctx context.Context, req *livekit.UpdateSubscriptionsRequest) (*livekit.UpdateSubscriptionsResponse, error)
	OnSendData            func(ctx context.Context, req *livekit.SendDataRequest) (*livekit.SendDataResponse, error)
	OnUpdateRoomMetadata  func(ctx context.Context, req *livekit.UpdateRoomMetadataRequest) (*livekit.Room, error)
	OnForwardParticipant  func(ctx context.Context, req *livekit.ForwardParticipantRequest) (*livekit.ForwardParticipantResponse, error)
	OnMoveParticipant     func(ctx context.Context, req *livekit.MoveParticipantRequest) (*livekit.MoveParticipantResponse, error)
	OnPerformRpc          func(ctx context.Context, req *livekit.PerformRpcRequest) (*livekit.PerformRpcResponse, error)

	// CallCounts — incremented each time a method is called (useful for assertions)
	CreateRoomCalls          atomic.Int64
	ListRoomsCalls           atomic.Int64
	DeleteRoomCalls          atomic.Int64
	ListParticipantsCalls    atomic.Int64
	GetParticipantCalls      atomic.Int64
	RemoveParticipantCalls   atomic.Int64
	MutePublishedTrackCalls  atomic.Int64
	UpdateParticipantCalls   atomic.Int64
	UpdateSubscriptionsCalls atomic.Int64
	SendDataCalls            atomic.Int64
	UpdateRoomMetadataCalls  atomic.Int64
	ForwardParticipantCalls  atomic.Int64
	MoveParticipantCalls     atomic.Int64
	PerformRpcCalls          atomic.Int64
}

// MockEgress implements livekit.Egress for testing.
// Each method returns zero-value responses unless overridden.
// Use hooks to control specific method behavior.
type MockEgress struct {
	// Hooks — set before test to control behavior per-method
	OnStartRoomCompositeEgress  func(ctx context.Context, req *livekit.RoomCompositeEgressRequest) (*livekit.EgressInfo, error)
	OnStartWebEgress            func(ctx context.Context, req *livekit.WebEgressRequest) (*livekit.EgressInfo, error)
	OnStartParticipantEgress    func(ctx context.Context, req *livekit.ParticipantEgressRequest) (*livekit.EgressInfo, error)
	OnStartTrackCompositeEgress func(ctx context.Context, req *livekit.TrackCompositeEgressRequest) (*livekit.EgressInfo, error)
	OnStartTrackEgress          func(ctx context.Context, req *livekit.TrackEgressRequest) (*livekit.EgressInfo, error)
	OnUpdateLayout              func(ctx context.Context, req *livekit.UpdateLayoutRequest) (*livekit.EgressInfo, error)
	OnUpdateStream              func(ctx context.Context, req *livekit.UpdateStreamRequest) (*livekit.EgressInfo, error)
	OnListEgress                func(ctx context.Context, req *livekit.ListEgressRequest) (*livekit.ListEgressResponse, error)
	OnStopEgress                func(ctx context.Context, req *livekit.StopEgressRequest) (*livekit.EgressInfo, error)

	// CallCounts — incremented each time a method is called
	StartRoomCompositeEgressCalls  atomic.Int64
	StartWebEgressCalls            atomic.Int64
	StartParticipantEgressCalls    atomic.Int64
	StartTrackCompositeEgressCalls atomic.Int64
	StartTrackEgressCalls          atomic.Int64
	UpdateLayoutCalls              atomic.Int64
	UpdateStreamCalls              atomic.Int64
	ListEgressCalls                atomic.Int64
	StopEgressCalls                atomic.Int64
}

var _ livekit.Egress = (*MockEgress)(nil)

func NewMockEgress() *MockEgress {
	return &MockEgress{}
}

func (m *MockEgress) StartRoomCompositeEgress(ctx context.Context, req *livekit.RoomCompositeEgressRequest) (*livekit.EgressInfo, error) {
	m.StartRoomCompositeEgressCalls.Add(1)
	if m.OnStartRoomCompositeEgress != nil {
		return m.OnStartRoomCompositeEgress(ctx, req)
	}
	return &livekit.EgressInfo{EgressId: "mock-egress-id", Status: livekit.EgressStatus_EGRESS_ACTIVE}, nil
}

func (m *MockEgress) StartWebEgress(ctx context.Context, req *livekit.WebEgressRequest) (*livekit.EgressInfo, error) {
	m.StartWebEgressCalls.Add(1)
	if m.OnStartWebEgress != nil {
		return m.OnStartWebEgress(ctx, req)
	}
	return &livekit.EgressInfo{EgressId: "mock-egress-id"}, nil
}

func (m *MockEgress) StartParticipantEgress(ctx context.Context, req *livekit.ParticipantEgressRequest) (*livekit.EgressInfo, error) {
	m.StartParticipantEgressCalls.Add(1)
	if m.OnStartParticipantEgress != nil {
		return m.OnStartParticipantEgress(ctx, req)
	}
	return &livekit.EgressInfo{EgressId: "mock-egress-id"}, nil
}

func (m *MockEgress) StartTrackCompositeEgress(ctx context.Context, req *livekit.TrackCompositeEgressRequest) (*livekit.EgressInfo, error) {
	m.StartTrackCompositeEgressCalls.Add(1)
	if m.OnStartTrackCompositeEgress != nil {
		return m.OnStartTrackCompositeEgress(ctx, req)
	}
	return &livekit.EgressInfo{EgressId: "mock-egress-id"}, nil
}

func (m *MockEgress) StartTrackEgress(ctx context.Context, req *livekit.TrackEgressRequest) (*livekit.EgressInfo, error) {
	m.StartTrackEgressCalls.Add(1)
	if m.OnStartTrackEgress != nil {
		return m.OnStartTrackEgress(ctx, req)
	}
	return &livekit.EgressInfo{EgressId: "mock-egress-id"}, nil
}

func (m *MockEgress) UpdateLayout(ctx context.Context, req *livekit.UpdateLayoutRequest) (*livekit.EgressInfo, error) {
	m.UpdateLayoutCalls.Add(1)
	if m.OnUpdateLayout != nil {
		return m.OnUpdateLayout(ctx, req)
	}
	return &livekit.EgressInfo{}, nil
}

func (m *MockEgress) UpdateStream(ctx context.Context, req *livekit.UpdateStreamRequest) (*livekit.EgressInfo, error) {
	m.UpdateStreamCalls.Add(1)
	if m.OnUpdateStream != nil {
		return m.OnUpdateStream(ctx, req)
	}
	return &livekit.EgressInfo{}, nil
}

func (m *MockEgress) ListEgress(ctx context.Context, req *livekit.ListEgressRequest) (*livekit.ListEgressResponse, error) {
	m.ListEgressCalls.Add(1)
	if m.OnListEgress != nil {
		return m.OnListEgress(ctx, req)
	}
	return &livekit.ListEgressResponse{}, nil
}

func (m *MockEgress) StopEgress(ctx context.Context, req *livekit.StopEgressRequest) (*livekit.EgressInfo, error) {
	m.StopEgressCalls.Add(1)
	if m.OnStopEgress != nil {
		return m.OnStopEgress(ctx, req)
	}
	return &livekit.EgressInfo{EgressId: req.EgressId, Status: livekit.EgressStatus_EGRESS_ABORTED}, nil
}

var _ livekit.RoomService = (*MockRoomService)(nil)

func NewMockRoomService() *MockRoomService {
	return &MockRoomService{}
}

func (m *MockRoomService) CreateRoom(ctx context.Context, req *livekit.CreateRoomRequest) (*livekit.Room, error) {
	m.CreateRoomCalls.Add(1)
	if m.OnCreateRoom != nil {
		return m.OnCreateRoom(ctx, req)
	}
	return &livekit.Room{Name: req.Name, Sid: "mock-room-sid"}, nil
}

func (m *MockRoomService) ListRooms(ctx context.Context, req *livekit.ListRoomsRequest) (*livekit.ListRoomsResponse, error) {
	m.ListRoomsCalls.Add(1)
	if m.OnListRooms != nil {
		return m.OnListRooms(ctx, req)
	}
	return &livekit.ListRoomsResponse{}, nil
}

func (m *MockRoomService) DeleteRoom(ctx context.Context, req *livekit.DeleteRoomRequest) (*livekit.DeleteRoomResponse, error) {
	m.DeleteRoomCalls.Add(1)
	if m.OnDeleteRoom != nil {
		return m.OnDeleteRoom(ctx, req)
	}
	return &livekit.DeleteRoomResponse{}, nil
}

func (m *MockRoomService) ListParticipants(ctx context.Context, req *livekit.ListParticipantsRequest) (*livekit.ListParticipantsResponse, error) {
	m.ListParticipantsCalls.Add(1)
	if m.OnListParticipants != nil {
		return m.OnListParticipants(ctx, req)
	}
	return &livekit.ListParticipantsResponse{}, nil
}

func (m *MockRoomService) GetParticipant(ctx context.Context, req *livekit.RoomParticipantIdentity) (*livekit.ParticipantInfo, error) {
	m.GetParticipantCalls.Add(1)
	if m.OnGetParticipant != nil {
		return m.OnGetParticipant(ctx, req)
	}
	return &livekit.ParticipantInfo{
		Identity: req.Identity,
		Sid:      "mock-participant-sid",
		Name:     req.Identity,
	}, nil
}

func (m *MockRoomService) RemoveParticipant(ctx context.Context, req *livekit.RoomParticipantIdentity) (*livekit.RemoveParticipantResponse, error) {
	m.RemoveParticipantCalls.Add(1)
	if m.OnRemoveParticipant != nil {
		return m.OnRemoveParticipant(ctx, req)
	}
	return &livekit.RemoveParticipantResponse{}, nil
}

func (m *MockRoomService) MutePublishedTrack(ctx context.Context, req *livekit.MuteRoomTrackRequest) (*livekit.MuteRoomTrackResponse, error) {
	m.MutePublishedTrackCalls.Add(1)
	if m.OnMutePublishedTrack != nil {
		return m.OnMutePublishedTrack(ctx, req)
	}
	return &livekit.MuteRoomTrackResponse{}, nil
}

func (m *MockRoomService) UpdateParticipant(ctx context.Context, req *livekit.UpdateParticipantRequest) (*livekit.ParticipantInfo, error) {
	m.UpdateParticipantCalls.Add(1)
	if m.OnUpdateParticipant != nil {
		return m.OnUpdateParticipant(ctx, req)
	}
	return &livekit.ParticipantInfo{
		Identity: req.Identity,
		Name:     req.Name,
		Metadata: req.Metadata,
	}, nil
}

func (m *MockRoomService) UpdateSubscriptions(ctx context.Context, req *livekit.UpdateSubscriptionsRequest) (*livekit.UpdateSubscriptionsResponse, error) {
	m.UpdateSubscriptionsCalls.Add(1)
	if m.OnUpdateSubscriptions != nil {
		return m.OnUpdateSubscriptions(ctx, req)
	}
	return &livekit.UpdateSubscriptionsResponse{}, nil
}

func (m *MockRoomService) SendData(ctx context.Context, req *livekit.SendDataRequest) (*livekit.SendDataResponse, error) {
	m.SendDataCalls.Add(1)
	if m.OnSendData != nil {
		return m.OnSendData(ctx, req)
	}
	return &livekit.SendDataResponse{}, nil
}

func (m *MockRoomService) UpdateRoomMetadata(ctx context.Context, req *livekit.UpdateRoomMetadataRequest) (*livekit.Room, error) {
	m.UpdateRoomMetadataCalls.Add(1)
	if m.OnUpdateRoomMetadata != nil {
		return m.OnUpdateRoomMetadata(ctx, req)
	}
	return &livekit.Room{Name: req.Room}, nil
}

func (m *MockRoomService) ForwardParticipant(ctx context.Context, req *livekit.ForwardParticipantRequest) (*livekit.ForwardParticipantResponse, error) {
	m.ForwardParticipantCalls.Add(1)
	if m.OnForwardParticipant != nil {
		return m.OnForwardParticipant(ctx, req)
	}
	return &livekit.ForwardParticipantResponse{}, nil
}

func (m *MockRoomService) MoveParticipant(ctx context.Context, req *livekit.MoveParticipantRequest) (*livekit.MoveParticipantResponse, error) {
	m.MoveParticipantCalls.Add(1)
	if m.OnMoveParticipant != nil {
		return m.OnMoveParticipant(ctx, req)
	}
	return &livekit.MoveParticipantResponse{}, nil
}

func (m *MockRoomService) PerformRpc(ctx context.Context, req *livekit.PerformRpcRequest) (*livekit.PerformRpcResponse, error) {
	m.PerformRpcCalls.Add(1)
	if m.OnPerformRpc != nil {
		return m.OnPerformRpc(ctx, req)
	}
	return &livekit.PerformRpcResponse{}, nil
}
