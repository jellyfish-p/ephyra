package judgegrpc

import (
	"context"
	"io"
	"log/slog"

	"github.com/google/uuid"
	judgev1 "github.com/jellyfish-p/ephyra/protocol/judge/v1"
	"github.com/jellyfish-p/ephyra/server/internal/clock"
	"github.com/jellyfish-p/ephyra/server/internal/judging"
)

// judgingAPI is the judging surface Connect uses. *judging.Module implements it;
// tests may substitute a stub so Connect can be covered without GORM.
type judgingAPI interface {
	Handshake(ctx context.Context, hello judging.Hello) (judging.Admission, error)
	Heartbeat(ctx context.Context, nodeID uuid.UUID, active []string) ([]string, error)
	Disconnect(ctx context.Context, nodeID uuid.UUID)
	AcceptResult(ctx context.Context, report judging.Report) error
}

var _ judgingAPI = (*judging.Module)(nil)

type Server struct {
	judgev1.UnimplementedJudgeControlServer
	Judging judgingAPI
	Hub     *judging.Hub
	Clock   clock.Clock
}

func ackInvalidations(inv []string) *judgev1.HeartbeatAck {
	return &judgev1.HeartbeatAck{InvalidateAttemptIds: inv}
}

func (s *Server) nodeHub() *judging.Hub {
	if s.Hub != nil {
		return s.Hub
	}
	if m, ok := s.Judging.(*judging.Module); ok && m != nil && m.Hub != nil {
		s.Hub = m.Hub
		return s.Hub
	}
	panic("judgegrpc: Hub is required so Connect and Schedule share one connection table")
}

func (s *Server) ackActive(ctx context.Context, conn *judging.Conn, nodeID uuid.UUID, active []string) {
	inv, err := s.Judging.Heartbeat(ctx, nodeID, active)
	if err != nil {
		slog.Error("heartbeat", "node_id", nodeID, "err", err)
		return
	}
	ack := ackInvalidations(inv)
	ack.UnixMs = s.Clock.Now().UnixMilli()
	msg := &judgev1.ServerMessage{Body: &judgev1.ServerMessage_HeartbeatAck{HeartbeatAck: ack}}
	if !conn.Send(ctx, msg) {
		slog.Warn("heartbeat ack dropped", "node_id", nodeID)
	}
}

func writeLoop(stream judgev1.JudgeControl_ConnectServer, conn *judging.Conn) error {
	for {
		select {
		case msg := <-conn.Messages():
			if err := stream.Send(msg); err != nil {
				return err
			}
		case <-conn.Done():
			for {
				select {
				case msg := <-conn.Messages():
					if err := stream.Send(msg); err != nil {
						return err
					}
				default:
					return nil
				}
			}
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}
}

func (s *Server) Connect(stream judgev1.JudgeControl_ConnectServer) error {
	first, err := stream.Recv()
	if err != nil {
		return err
	}
	hs := first.GetHandshake()
	if hs == nil {
		return io.EOF
	}
	adm, err := s.Judging.Handshake(stream.Context(), helloFromProto(hs))
	if err != nil {
		return err
	}
	if err := stream.Send(&judgev1.ServerMessage{Body: &judgev1.ServerMessage_Handshake{Handshake: admissionToProto(adm)}}); err != nil {
		return err
	}
	if !adm.Accepted {
		return nil
	}
	conn := judging.NewConn(adm.NodeID, hs.GetExecutionProfileIds(), hs.GetMaxSlots())
	s.nodeHub().Register(conn)
	defer s.Judging.Disconnect(context.Background(), adm.NodeID)

	writeDone := make(chan error, 1)
	go func() { writeDone <- writeLoop(stream, conn) }()

	readErr := s.readLoop(stream, conn, adm.NodeID)
	s.nodeHub().Unregister(adm.NodeID, conn)
	writeErr := <-writeDone
	if readErr != nil {
		return readErr
	}
	return writeErr
}

func (s *Server) readLoop(stream judgev1.JudgeControl_ConnectServer, conn *judging.Conn, nodeID uuid.UUID) error {
	for {
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		switch b := msg.Body.(type) {
		case *judgev1.NodeMessage_Heartbeat:
			s.ackActive(stream.Context(), conn, nodeID, b.Heartbeat.GetActiveAttemptIds())
		case *judgev1.NodeMessage_Result:
			rep, err := reportFromProto(b.Result)
			if err != nil {
				slog.Error("result parse", "node_id", nodeID, "err", err)
				continue
			}
			if err := s.Judging.AcceptResult(stream.Context(), rep); err != nil {
				slog.Error("result", "node_id", nodeID, "attempt_id", rep.AttemptID, "err", err)
			}
		case *judgev1.NodeMessage_Slots:
			// SlotReport is the same "here is what I am running" signal as
			// Heartbeat. The node must stop invalidated attempts before it
			// advertises free slots, so we reuse HeartbeatAck.
			s.ackActive(stream.Context(), conn, nodeID, b.Slots.GetActiveAttemptIds())
		case *judgev1.NodeMessage_Progress:
			_ = b
		}
	}
}
