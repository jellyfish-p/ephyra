package scoring

import "github.com/google/uuid"

type WorkItem struct {
	SubmissionID                   uuid.UUID
	HasNormalResultForScoringRev   bool
	AwaitingAdmin                  bool
	SystemFault                    bool
	OpenRejudgeBatch               bool
}

type Barrier struct {
	Ready  bool
	Reason string
}

func PublicationBarrier(contestEnded bool, items []WorkItem) Barrier {
	if !contestEnded {
		return Barrier{Reason: "contest_not_ended"}
	}
	if len(items) == 0 {
		return Barrier{Ready: true}
	}
	for _, it := range items {
		if it.OpenRejudgeBatch {
			return Barrier{Reason: "open_rejudge_batch"}
		}
		if it.AwaitingAdmin || it.SystemFault {
			return Barrier{Reason: "pending_system_error"}
		}
		if !it.HasNormalResultForScoringRev {
			return Barrier{Reason: "incomplete_judging"}
		}
	}
	return Barrier{Ready: true}
}
