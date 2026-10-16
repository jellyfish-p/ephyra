package submission

import "github.com/google/uuid"

// OpenBatchIDForNewSubmission is always nil. A submission received after a
// rejudge snapshot binds contest_problem.target_revision_id on its own
// initial task and is not inserted into the open batch.
func OpenBatchIDForNewSubmission() *uuid.UUID {
	return nil
}
