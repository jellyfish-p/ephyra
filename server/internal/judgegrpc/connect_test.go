package judgegrpc

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/google/uuid"
	judgev1 "github.com/jellyfish-p/ephyra/protocol/judge/v1"
	"github.com/jellyfish-p/ephyra/server/internal/clock"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/judging"
	"google.golang.org/grpc/metadata"
)

type stubJudging struct {
	nodeID      uuid.UUID
	invalidated []string
}

func (s *stubJudging) Handshake(context.Context, *judgev1.HandshakeRequest) (uuid.UUID, *judgev1.HandshakeResponse, error) {
	return s.nodeID, &judgev1.HandshakeResponse{Accepted: true, ProtocolVersion: domain.ProtocolVersion}, nil
}

func (s *stubJudging) Heartbeat(_ context.Context, _ uuid.UUID, active []string) ([]string, error) {
	return judging.HeartbeatInvalidations(active, s.invalidated), nil
}

func (*stubJudging) Disconnect(context.Context, uuid.UUID) {}

func (*stubJudging) AcceptResult(context.Context, judging.Report) error { return nil }

type stubStream struct {
	ctx  context.Context
	recv []*judgev1.NodeMessage
	sent []*judgev1.ServerMessage
	i    int
}

func (s *stubStream) Context() context.Context { return s.ctx }
func (s *stubStream) SetHeader(metadata.MD) error {
	return nil
}
func (s *stubStream) SendHeader(metadata.MD) error { return nil }
func (s *stubStream) SetTrailer(metadata.MD)       {}
func (s *stubStream) SendMsg(any) error            { return nil }
func (s *stubStream) RecvMsg(any) error            { return io.EOF }

func (s *stubStream) Send(m *judgev1.ServerMessage) error {
	s.sent = append(s.sent, m)
	return nil
}

func (s *stubStream) Recv() (*judgev1.NodeMessage, error) {
	if s.i >= len(s.recv) {
		return nil, io.EOF
	}
	m := s.recv[s.i]
	s.i++
	return m, nil
}

func TestConnectSlotReportSendsHeartbeatAck(t *testing.T) {
	nodeID := uuid.New()
	dead := uuid.New().String()
	live := uuid.New().String()
	now := time.UnixMilli(1_700_000_000_000)
	srv := &Server{
		Judging: &stubJudging{nodeID: nodeID, invalidated: []string{dead}},
		Hub:     judging.NewHub(),
		Clock:   clock.Fixed{T: now},
	}
	stream := &stubStream{
		ctx: context.Background(),
		recv: []*judgev1.NodeMessage{
			{Body: &judgev1.NodeMessage_Handshake{Handshake: &judgev1.HandshakeRequest{
				ProtocolVersion: domain.ProtocolVersion, NodeId: nodeID.String(), MaxSlots: 1,
			}}},
			{Body: &judgev1.NodeMessage_Slots{Slots: &judgev1.SlotReport{
				BusySlots: 1, FreeSlots: 0, ActiveAttemptIds: []string{dead, live},
			}}},
		},
	}
	if err := srv.Connect(stream); err != io.EOF {
		t.Fatalf("connect: %v", err)
	}
	var ack *judgev1.HeartbeatAck
	for _, msg := range stream.sent {
		if got := msg.GetHeartbeatAck(); got != nil {
			ack = got
		}
	}
	if ack == nil {
		t.Fatal("SlotReport must send HeartbeatAck")
	}
	if ack.GetUnixMs() != now.UnixMilli() {
		t.Fatalf("unix_ms %d", ack.GetUnixMs())
	}
	if len(ack.GetInvalidateAttemptIds()) != 1 || ack.GetInvalidateAttemptIds()[0] != dead {
		t.Fatalf("invalidate %v", ack.GetInvalidateAttemptIds())
	}
}
