package judgegrpc

import (
	"context"
	"io"
	"log"

	"github.com/google/uuid"
	judgev1 "github.com/jellyfish-p/ephyra/protocol/judge/v1"
	"github.com/jellyfish-p/ephyra/server/internal/clock"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/judging"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
)

// judgingAPI is the judging surface Connect uses. *judging.Module implements it;
// tests may substitute a stub so Connect can be covered without GORM.
type judgingAPI interface {
	Handshake(ctx context.Context, req *judgev1.HandshakeRequest) (persist.JudgeNode, *judgev1.HandshakeResponse, error)
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
	s.Hub = judging.NewHub()
	return s.Hub
}

func (s *Server) ackActive(stream judgev1.JudgeControl_ConnectServer, nodeID uuid.UUID, active []string) {
	inv, err := s.Judging.Heartbeat(stream.Context(), nodeID, active)
	if err != nil {
		log.Printf("heartbeat: %v", err)
		return
	}
	ack := ackInvalidations(inv)
	ack.UnixMs = s.Clock.Now().UnixMilli()
	_ = stream.Send(&judgev1.ServerMessage{Body: &judgev1.ServerMessage_HeartbeatAck{HeartbeatAck: ack}})
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
	node, resp, err := s.Judging.Handshake(stream.Context(), hs)
	if err != nil {
		return err
	}
	if err := stream.Send(&judgev1.ServerMessage{Body: &judgev1.ServerMessage_Handshake{Handshake: resp}}); err != nil {
		return err
	}
	if !resp.Accepted {
		return nil
	}
	conn := &judging.Conn{
		NodeID: node.ID, Send: make(chan *judgev1.ServerMessage, 8),
		Profiles: hs.GetExecutionProfileIds(), MaxSlots: hs.GetMaxSlots(),
	}
	s.nodeHub().Register(conn)
	defer func() {
		s.nodeHub().Unregister(node.ID, conn)
		s.Judging.Disconnect(context.Background(), node.ID)
	}()

	errCh := make(chan error, 1)
	go func() {
		for msg := range conn.Send {
			if err := stream.Send(msg); err != nil {
				errCh <- err
				return
			}
		}
	}()

	for {
		select {
		case err := <-errCh:
			return err
		default:
		}
		msg, err := stream.Recv()
		if err != nil {
			return err
		}
		switch b := msg.Body.(type) {
		case *judgev1.NodeMessage_Heartbeat:
			s.ackActive(stream, node.ID, b.Heartbeat.GetActiveAttemptIds())
		case *judgev1.NodeMessage_Result:
			rep, err := judging.ReportFromProto(b.Result)
			if err != nil {
				log.Printf("result parse: %v", err)
				continue
			}
			if err := s.Judging.AcceptResult(stream.Context(), rep); err != nil {
				log.Printf("result: %v", err)
			}
		case *judgev1.NodeMessage_Slots:
			// SlotReport is the same "here is what I am running" signal as
			// Heartbeat. The node must stop invalidated attempts before it
			// advertises free slots, so we reuse HeartbeatAck.
			s.ackActive(stream, node.ID, b.Slots.GetActiveAttemptIds())
		case *judgev1.NodeMessage_Progress:
			_ = b
		default:
			_ = domain.ProtocolVersion
			_ = uuid.Nil
		}
	}
}
