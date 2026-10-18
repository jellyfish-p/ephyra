package scoring

import (
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
)

// SubmissionMeta is the non-result identity of a submission. It is a DTO, not a GORM model.
type SubmissionMeta struct {
	ID                 uuid.UUID
	ContestID          uuid.UUID
	EntrantID          uuid.UUID
	SubmitterAccountID uuid.UUID
	LanguageID         string
	ReceivedAt         time.Time
	ReceiveSeq         int64
	RevisionID         uuid.UUID
}

// ResultDetail is the sanitized judging summary used to project a public view.
// It must never carry checker logs, expected output, or a raw JudgeResult.
type ResultDetail struct {
	HasResult     bool
	RevisionID    uuid.UUID
	CompileFailed bool
	Verdict       domain.Verdict
	Score         exact.Score
	Subtasks      []SubtaskOutcome
	Cases         []CaseOutcome
	SystemFault   bool
}

type CasePublic struct {
	CaseID  string `json:"case_id"`
	Verdict string `json:"verdict"`
}

type SubtaskPublic struct {
	ID      string      `json:"id"`
	Score   exact.Score `json:"score"`
	Verdict string      `json:"verdict"`
}

// SubmissionPublic is the HTTP/SSE payload. GORM models must not be serialized.
type SubmissionPublic struct {
	ID                 uuid.UUID `json:"id"`
	ContestID          uuid.UUID `json:"contest_id"`
	EntrantID          uuid.UUID `json:"entrant_id"`
	SubmitterAccountID uuid.UUID `json:"submitter_account_id"`
	LanguageID         string    `json:"language_id"`
	ReceivedAt         time.Time `json:"received_at"`
	ReceiveSeq         int64     `json:"receive_seq"`
	RevisionID         uuid.UUID `json:"revision_id"`
	Feedback           string    `json:"feedback"`
	// EntrantDisplayName is populated by the contest submission list only, so a
	// scoreboard-style list can label rows without a second lookup. The
	// single-submission and SSE payloads leave it empty.
	EntrantDisplayName string          `json:"entrant_display_name,omitempty"`
	NotEffective       bool            `json:"not_effective,omitempty"`
	CompileFailed      *bool           `json:"compile_failed,omitempty"`
	Verdict            string          `json:"verdict,omitempty"`
	Score              *exact.Score    `json:"score,omitempty"`
	Subtasks           []SubtaskPublic `json:"subtasks,omitempty"`
	Cases              []CasePublic    `json:"cases,omitempty"`
}

// EmitMask is the set of result fields Visible() allows a given audience to receive.
type EmitMask struct {
	CompileFailed bool
	Verdict       bool
	Score         bool
	Subtasks      bool
	Cases         bool
}

func EmitFields(v View) EmitMask {
	var m EmitMask
	switch v.Feedback {
	case FeedbackCompileOnly:
		m.CompileFailed = true
	case FeedbackScore:
		m.CompileFailed = true
		m.Verdict = true
		m.Score = true
	case FeedbackFull:
		m.CompileFailed = true
		m.Verdict = true
		m.Score = true
		m.Subtasks = true
		if v.Granularity == domain.FeedbackCases {
			m.Cases = true
		}
	}
	return m
}

func ShouldEmit(v View) bool {
	return v.Feedback != FeedbackNone
}

func ProjectSubmission(meta SubmissionMeta, view View, detail *ResultDetail) SubmissionPublic {
	out := SubmissionPublic{
		ID:                 meta.ID,
		ContestID:          meta.ContestID,
		EntrantID:          meta.EntrantID,
		SubmitterAccountID: meta.SubmitterAccountID,
		LanguageID:         meta.LanguageID,
		ReceivedAt:         meta.ReceivedAt,
		ReceiveSeq:         meta.ReceiveSeq,
		RevisionID:         meta.RevisionID,
		Feedback:           view.Feedback.Name(),
		NotEffective:       view.NotEffective,
	}
	if detail == nil || !detail.HasResult {
		return out
	}
	if detail.SystemFault && view.Feedback != FeedbackFull {
		return out
	}
	mask := EmitFields(view)
	if mask.CompileFailed && !detail.SystemFault {
		cf := detail.CompileFailed || detail.Verdict == domain.VerdictCompilationError
		out.CompileFailed = &cf
	}
	if mask.Verdict {
		out.Verdict = string(detail.Verdict)
	}
	if mask.Score && !detail.SystemFault {
		sc := detail.Score
		out.Score = &sc
	}
	if mask.Subtasks && !detail.SystemFault {
		out.Subtasks = projectSubtasks(detail.Subtasks)
	}
	if mask.Cases && !detail.SystemFault {
		out.Cases = projectCases(detail.Cases)
	}
	return out
}

func projectCases(in []CaseOutcome) []CasePublic {
	if len(in) == 0 {
		return nil
	}
	out := make([]CasePublic, 0, len(in))
	for _, c := range in {
		out = append(out, CasePublic{CaseID: c.CaseID, Verdict: string(c.Verdict)})
	}
	return out
}

func projectSubtasks(in []SubtaskOutcome) []SubtaskPublic {
	if len(in) == 0 {
		return nil
	}
	out := make([]SubtaskPublic, 0, len(in))
	for _, s := range in {
		out = append(out, SubtaskPublic{ID: s.ID, Score: s.Score, Verdict: string(s.Verdict)})
	}
	return out
}
