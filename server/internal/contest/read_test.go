package contest

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
	"github.com/jellyfish-p/ephyra/server/internal/objectstore"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/jellyfish-p/ephyra/server/internal/problem"
)

func TestProblemDetailExcludesJudgeData(t *testing.T) {
	cfg := problem.NormalizedConfig{
		Kind: domain.KindBatch, Checker: problem.CheckerSpec{Kind: domain.CheckerTestlib, SourceName: "SECRET_CHECKER.cpp"},
		Interactor:  &problem.InteractorSpec{SourceName: "SECRET_INTERACTOR.cpp"},
		TimeLimitMS: 1000, MemoryLimitBytes: 268435456,
		Subtasks: []problem.NormalizedSubtask{{ID: "1", Score: exact.FromInt(50), CaseIDs: []string{"SECRET_CASE"}}},
		Cases:    []problem.NormalizedCase{{ID: "SECRET_CASE", InputName: "SECRET_INPUT.in", OutputName: "SECRET_OUTPUT.out"}},
	}
	cp := persist.ContestProblem{Letter: "A", ProblemID: uuid.New(), ScoringRevisionID: uuid.New(), TargetRevisionID: uuid.New()}
	raw, err := json.Marshal(problemDetailOf(cp, "Two Sum", cfg))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "SECRET") {
		t.Fatalf("judge data leaked: %s", raw)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	keys := make([]string, 0, len(got))
	for key := range got {
		keys = append(keys, key)
	}
	slices.Sort(keys)
	want := []string{"checker_kind", "kind", "letter", "memory_limit_bytes", "problem_id", "scoring_revision_id", "statement_markdown", "subtasks", "target_revision_id", "time_limit_ms", "title"}
	if !reflect.DeepEqual(keys, want) {
		t.Fatalf("keys %v, want %v (no samples or file metadata allowed)", keys, want)
	}
	if got["statement_markdown"] != nil {
		t.Fatalf("missing statement must be null: %s", raw)
	}
	subtask := got["subtasks"].([]any)[0].(map[string]any)
	if len(subtask) != 2 || subtask["id"] != "1" || subtask["score"] != "50.00" {
		t.Fatalf("subtask must contain only id and decimal-string score: %v", subtask)
	}
	if got["kind"] != "batch" || got["checker_kind"] != string(domain.CheckerTestlib) || got["time_limit_ms"] != float64(1000) || got["memory_limit_bytes"] != float64(268435456) {
		t.Fatalf("public config: %s", raw)
	}
}

func TestReadStatementIsOptionalAndBounded(t *testing.T) {
	blobs := objectstore.NewMemory()
	for _, tc := range []struct {
		name string
		body string
	}{
		{"markdown", "# Two Sum\n"},
		{"empty", ""},
		{"bounded", strings.Repeat("x", (1<<20)+100)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := blobs.Put(t.Context(), tc.name, strings.NewReader(tc.body), int64(len(tc.body)), "text/markdown"); err != nil {
				t.Fatal(err)
			}
			got := readStatement(t.Context(), blobs, tc.name)
			if got == nil || *got != tc.body[:min(len(tc.body), 1<<20)] {
				t.Fatal("statement absent, corrupted, or exceeded 1 MiB")
			}
		})
	}
	if readStatement(t.Context(), blobs, "absent") != nil || readStatement(t.Context(), nil, "absent") != nil {
		t.Fatal("unavailable object must produce null")
	}
	r := &failingStatementReader{}
	if readStatement(t.Context(), failingStatementStore{reader: r}, "broken") != nil {
		t.Fatal("partial object read must produce null")
	}
	if !r.closed {
		t.Fatal("reader must be closed on failure")
	}
}

type failingStatementStore struct {
	objectstore.Store
	reader *failingStatementReader
}

func (s failingStatementStore) Get(context.Context, string) (io.ReadCloser, int64, string, error) {
	return s.reader, 0, "text/markdown", nil
}

type failingStatementReader struct{ closed bool }

func (r *failingStatementReader) Read(p []byte) (int, error) {
	return copy(p, "partial"), errors.New("object read failed")
}

func (r *failingStatementReader) Close() error {
	r.closed = true
	return nil
}

func TestMyEntrantNullableJSON(t *testing.T) {
	raw, err := json.Marshal(MyEntrantView{Members: []EntrantMemberView{{Role: "member"}}})
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"accepted_at", "roster_frozen_at"} {
		value, ok := got[key]
		if !ok || value != nil {
			t.Fatalf("%s must be present and null: %s", key, raw)
		}
	}
	if member := got["members"].([]any)[0].(map[string]any); len(member) != 4 || member["confirmed"] != false {
		t.Fatalf("member shape: %v", member)
	}
}
