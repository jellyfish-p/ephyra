package judgegrpc

import (
	"context"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
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

func (s *stubJudging) Handshake(context.Context, judging.Hello) (judging.Admission, error) {
	return judging.Admission{
		NodeID: s.nodeID, Accepted: true, ProtocolVersion: domain.ProtocolVersion,
		HeartbeatIntervalMs: int32(domain.HeartbeatInterval / time.Millisecond),
		LeaseMs:             int32(domain.NodeLostAfter / time.Millisecond),
	}, nil
}

func (s *stubJudging) Heartbeat(_ context.Context, _ uuid.UUID, active []string) ([]string, error) {
	return judging.HeartbeatInvalidations(active, s.invalidated), nil
}

func (*stubJudging) Disconnect(context.Context, uuid.UUID) {}

func (*stubJudging) AcceptResult(context.Context, judging.Report) error { return nil }

type stubStream struct {
	ctx     context.Context
	recv    []*judgev1.NodeMessage
	sent    []*judgev1.ServerMessage
	i       int
	sending int32
	mu      sync.Mutex
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
	if !atomic.CompareAndSwapInt32(&s.sending, 0, 1) {
		return fmt.Errorf("concurrent Send")
	}
	defer atomic.StoreInt32(&s.sending, 0)
	s.mu.Lock()
	s.sent = append(s.sent, m)
	s.mu.Unlock()
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
	stream.mu.Lock()
	sent := append([]*judgev1.ServerMessage(nil), stream.sent...)
	stream.mu.Unlock()
	for _, msg := range sent {
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

type liveStream struct {
	ctx     context.Context
	recv    chan *judgev1.NodeMessage
	sending int32
	mu      sync.Mutex
	sent    []*judgev1.ServerMessage
}

func (s *liveStream) Context() context.Context { return s.ctx }
func (s *liveStream) SetHeader(metadata.MD) error {
	return nil
}
func (s *liveStream) SendHeader(metadata.MD) error { return nil }
func (s *liveStream) SetTrailer(metadata.MD)       {}
func (s *liveStream) SendMsg(any) error            { return nil }
func (s *liveStream) RecvMsg(any) error            { return io.EOF }

func (s *liveStream) Send(m *judgev1.ServerMessage) error {
	if !atomic.CompareAndSwapInt32(&s.sending, 0, 1) {
		return fmt.Errorf("concurrent Send")
	}
	time.Sleep(time.Millisecond)
	defer atomic.StoreInt32(&s.sending, 0)
	s.mu.Lock()
	s.sent = append(s.sent, m)
	s.mu.Unlock()
	return nil
}

func (s *liveStream) Recv() (*judgev1.NodeMessage, error) {
	select {
	case m, ok := <-s.recv:
		if !ok {
			return nil, io.EOF
		}
		return m, nil
	case <-s.ctx.Done():
		return nil, s.ctx.Err()
	}
}

func TestConnectSingleWriterUnderDispatchAndHeartbeat(t *testing.T) {
	nodeID := uuid.New()
	hub := judging.NewHub()
	srv := &Server{
		Judging: &stubJudging{nodeID: nodeID},
		Hub:     hub,
		Clock:   clock.System{},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stream := &liveStream{ctx: ctx, recv: make(chan *judgev1.NodeMessage, 8)}
	errCh := make(chan error, 1)
	go func() { errCh <- srv.Connect(stream) }()

	stream.recv <- &judgev1.NodeMessage{Body: &judgev1.NodeMessage_Handshake{Handshake: &judgev1.HandshakeRequest{
		ProtocolVersion: domain.ProtocolVersion, NodeId: nodeID.String(), MaxSlots: 1,
	}}}

	deadline := time.Now().Add(2 * time.Second)
	for !hub.Connected(nodeID) {
		if time.Now().After(deadline) {
			t.Fatal("node never registered")
		}
		time.Sleep(time.Millisecond)
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 40; i++ {
			hub.Dispatch(nodeID, &judgev1.ServerMessage{Body: &judgev1.ServerMessage_Dispatch{Dispatch: &judgev1.DispatchAttempt{
				AttemptId: uuid.NewString(),
			}}})
			time.Sleep(time.Millisecond)
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < 40; i++ {
			stream.recv <- &judgev1.NodeMessage{Body: &judgev1.NodeMessage_Heartbeat{Heartbeat: &judgev1.Heartbeat{
				ActiveAttemptIds: []string{uuid.NewString()},
			}}}
			time.Sleep(time.Millisecond)
		}
	}()
	wg.Wait()
	close(stream.recv)
	select {
	case err := <-errCh:
		if err != nil && err != io.EOF {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("connect hung")
	}
}
