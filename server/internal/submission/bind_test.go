package submission

import "testing"

func TestNewSubmissionDoesNotJoinOpenBatch(t *testing.T) {
	if OpenBatchIDForNewSubmission() != nil {
		t.Fatal("later submissions must not be inserted into an open rejudge batch")
	}
}
