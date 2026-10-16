package scoring

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
)

func TestEmitFieldsCompileOnlyOmitsScoreAndCases(t *testing.T) {
	mask := EmitFields(View{Feedback: FeedbackCompileOnly, Granularity: domain.FeedbackCases})
	if !mask.CompileFailed {
		t.Fatal("compile status must be emitted")
	}
	if mask.Score || mask.Verdict || mask.Cases || mask.Subtasks {
		t.Fatalf("OI contest compile-only must hide cases/subtasks/score, got %+v", mask)
	}
}

func TestProjectSubmissionHidesForbiddenFields(t *testing.T) {
	meta := SubmissionMeta{
		ID: uuid.New(), ContestID: uuid.New(), EntrantID: uuid.New(),
		SubmitterAccountID: uuid.New(), LanguageID: "cpp23",
		ReceivedAt: time.Unix(50, 0).UTC(), ReceiveSeq: 3, RevisionID: uuid.New(),
	}
	detail := &ResultDetail{
		HasResult: true, CompileFailed: true, Verdict: domain.VerdictCompilationError,
		Score:    exact.FromInt(80),
		Subtasks: []SubtaskOutcome{{ID: "s1", Score: exact.FromInt(80), Verdict: domain.VerdictAccepted}},
		Cases:    []CaseOutcome{{CaseID: "1", Verdict: domain.VerdictAccepted}},
	}
	dto := ProjectSubmission(meta, View{Feedback: FeedbackCompileOnly, NotEffective: true}, detail)
	if dto.Score != nil || dto.Verdict != "" || len(dto.Cases) > 0 || len(dto.Subtasks) > 0 {
		t.Fatalf("compile-only leaked result fields: %+v", dto)
	}
	if dto.CompileFailed == nil || !*dto.CompileFailed {
		t.Fatal("compile-only must include compile status")
	}
	if !dto.NotEffective {
		t.Fatal("not_effective must survive projection")
	}
	raw, err := json.Marshal(dto)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, needle := range []string{"checker", "expected", "JudgeResult", "system_fault_kind"} {
		if strings.Contains(strings.ToLower(s), strings.ToLower(needle)) {
			t.Fatalf("payload must not include %q: %s", needle, s)
		}
	}
}

func TestProjectSubmissionFullEmitsScoreNotRawResult(t *testing.T) {
	meta := SubmissionMeta{ID: uuid.New(), RevisionID: uuid.New()}
	sc := exact.FromInt(50)
	detail := &ResultDetail{
		HasResult: true, Verdict: domain.VerdictWrongAnswer, Score: sc,
		Cases:    []CaseOutcome{{CaseID: "1", Verdict: domain.VerdictWrongAnswer}},
		Subtasks: []SubtaskOutcome{{ID: "s1", Score: sc, Verdict: domain.VerdictWrongAnswer}},
	}
	dto := ProjectSubmission(meta, View{Feedback: FeedbackFull, Granularity: domain.FeedbackCases}, detail)
	if dto.Score == nil || dto.Score.String() != "50.00" || dto.Verdict != string(domain.VerdictWrongAnswer) {
		t.Fatalf("full view must include score and verdict, got %+v", dto)
	}
	if len(dto.Cases) != 1 || dto.Cases[0].CaseID != "1" {
		t.Fatalf("cases %+v", dto.Cases)
	}
	raw, _ := json.Marshal(dto)
	if strings.Contains(string(raw), "expected") || strings.Contains(string(raw), "checker") {
		t.Fatalf("must not leak checker text: %s", raw)
	}
}

func TestShouldEmitSkipsHiddenAudience(t *testing.T) {
	if ShouldEmit(View{Feedback: FeedbackNone}) {
		t.Fatal("must not push FeedbackNone payloads")
	}
	if !ShouldEmit(View{Feedback: FeedbackCompileOnly}) {
		t.Fatal("compile-only is visible to the audience")
	}
}
