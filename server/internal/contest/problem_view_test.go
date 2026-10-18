package contest

import (
	"encoding/json"
	"testing"

	"github.com/google/uuid"
)

// The contest admin UI rejudges by contest-problem id, so the problem list has
// to expose that id even though it stays hidden for ordinary contestants.
func TestProblemViewAlwaysExposesContestProblemID(t *testing.T) {
	id := uuid.New()
	body, err := json.Marshal(ProblemView{ID: id, Letter: "A"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded["id"] != id.String() {
		t.Fatalf("contest problem id missing: %s", body)
	}
	if decoded["letter"] != "A" {
		t.Fatalf("letter missing: %s", body)
	}
	// A contestant sees letters only: the bank problem and revisions stay hidden.
	for _, hidden := range []string{"problem_id", "scoring_revision_id", "target_revision_id", "title"} {
		if _, ok := decoded[hidden]; ok {
			t.Fatalf("%q must be omitted when unset: %s", hidden, body)
		}
	}
}

// The rejudge flow targets `contest-problems/:id`, so the list envelope has to
// carry the row id. This is the shape the HTTP handler emits.
func TestContestProblemListEnvelopeCarriesID(t *testing.T) {
	cpID, problemID := uuid.New(), uuid.New()
	body, err := json.Marshal(map[string]any{
		"problems": []ProblemView{{ID: cpID, Letter: "A", ProblemID: &problemID}},
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded struct {
		Problems []struct {
			ID        string `json:"id"`
			Letter    string `json:"letter"`
			ProblemID string `json:"problem_id"`
		} `json:"problems"`
	}
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Problems) != 1 {
		t.Fatalf("problems: %s", body)
	}
	if decoded.Problems[0].ID != cpID.String() {
		t.Fatalf("contest problem id missing from the list envelope: %s", body)
	}
	if decoded.Problems[0].ProblemID != problemID.String() {
		t.Fatalf("problem_id: %s", body)
	}
}

func TestProblemViewIncludesAdminRevisionFields(t *testing.T) {
	problemID, scoring, target := uuid.New(), uuid.New(), uuid.New()
	body, err := json.Marshal(ProblemView{
		ID: uuid.New(), Letter: "B", Title: "Two Sum",
		ProblemID: &problemID, ScoringRevisionID: &scoring, TargetRevisionID: &target,
	})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(body, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for key, want := range map[string]string{
		"problem_id":          problemID.String(),
		"scoring_revision_id": scoring.String(),
		"target_revision_id":  target.String(),
	} {
		if decoded[key] != want {
			t.Fatalf("%s = %#v, want %s", key, decoded[key], want)
		}
	}
}
