package judging

import "github.com/google/uuid"

// Hello is a node's identity and advertised capacity. The gRPC adapter
// translates the wire handshake into this type so judging does not import
// protocol messages.
type Hello struct {
	ProtocolVersion string
	NodeID          string
	Credential      string
	Profiles        []string
	MaxSlots        int32
}

// Admission is judging's decision for a Hello. The adapter maps it back
// onto the versioned handshake response.
type Admission struct {
	NodeID              uuid.UUID
	Accepted            bool
	RejectReason        string
	ProtocolVersion     string
	HeartbeatIntervalMs int32
	LeaseMs             int32
}
