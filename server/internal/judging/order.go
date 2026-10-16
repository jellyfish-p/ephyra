package judging

import (
	"sort"

	"github.com/google/uuid"
)

// PendingWork is a pending_schedule task joined to its submission and contest.
// ReceiveSeq is the server-assigned submission order, not task created_at.
type PendingWork struct {
	TaskID     uuid.UUID
	OrgID      uuid.UUID
	ReceiveSeq int64
}

// OrderPendingWork dispatches earlier-received submissions first. Within an
// organization that is receive_seq order; receive_seq is site-unique so the
// same key also orders mixed-org work without using created_at.
func OrderPendingWork(items []PendingWork) []PendingWork {
	out := make([]PendingWork, len(items))
	copy(out, items)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].ReceiveSeq != out[j].ReceiveSeq {
			return out[i].ReceiveSeq < out[j].ReceiveSeq
		}
		if out[i].OrgID != out[j].OrgID {
			return out[i].OrgID.String() < out[j].OrgID.String()
		}
		return out[i].TaskID.String() < out[j].TaskID.String()
	})
	return out
}
