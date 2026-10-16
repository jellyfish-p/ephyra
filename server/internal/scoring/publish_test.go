package scoring

import "testing"

func TestPublicationBarrier(t *testing.T) {
	if PublicationBarrier(false, nil).Ready {
		t.Fatal("must not publish before end")
	}
	if !PublicationBarrier(true, nil).Ready {
		t.Fatal("empty contest may publish")
	}
	if PublicationBarrier(true, []WorkItem{{HasNormalResultForScoringRev: true, OpenRejudgeBatch: true}}).Ready {
		t.Fatal("open rejudge")
	}
	if PublicationBarrier(true, []WorkItem{{HasNormalResultForScoringRev: true, SystemFault: true}}).Ready {
		t.Fatal("system fault")
	}
	if PublicationBarrier(true, []WorkItem{{HasNormalResultForScoringRev: false}}).Ready {
		t.Fatal("incomplete")
	}
	if !PublicationBarrier(true, []WorkItem{{HasNormalResultForScoringRev: true}}).Ready {
		t.Fatal("normal CE/WA/AC should allow publish")
	}
}
