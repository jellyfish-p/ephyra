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
)

type Server struct {
	judgev1.UnimplementedJudgeControlServer
	Judging *judging.Module
	Clock   clock.Clock
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
	s.Judging.Hub.Register(conn)
	defer func() {
		s.Judging.Hub.Unregister(node.ID, conn)
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
			inv, err := s.Judging.Heartbeat(stream.Context(), node.ID, b.Heartbeat.GetActiveAttemptIds())
			if err != nil {
				log.Printf("heartbeat: %v", err)
				continue
			}
			_ = stream.Send(&judgev1.ServerMessage{Body: &judgev1.ServerMessage_HeartbeatAck{
				HeartbeatAck: &judgev1.HeartbeatAck{UnixMs: s.Clock.Now().UnixMilli(), InvalidateAttemptIds: inv},
			}})
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
			_, _ = s.Judging.Heartbeat(stream.Context(), node.ID, b.Slots.GetActiveAttemptIds())
		case *judgev1.NodeMessage_Progress:
			_ = b
		default:
			_ = domain.ProtocolVersion
			_ = uuid.Nil
		}
	}
}
