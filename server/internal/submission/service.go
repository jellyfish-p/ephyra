package submission

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/clock"
	"github.com/jellyfish-p/ephyra/server/internal/contest"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/execution"
	"github.com/jellyfish-p/ephyra/server/internal/objectstore"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Module struct {
	DB    *gorm.DB
	Clock clock.Clock
	Blobs objectstore.Store
}

type AcceptResult struct {
	Submission persist.Submission
	Task       persist.JudgeTask
}

func (m *Module) Submit(ctx context.Context, p access.Principal, contestID uuid.UUID, letter, language string, source []byte) (AcceptResult, error) {
	if len(source) == 0 || int64(len(source)) > domain.MaxSourceBytes {
		return AcceptResult{}, apperr.New(apperr.Invalid, "source is empty or too large")
	}
	lang, ok := domain.ParseLanguage(language)
	if !ok {
		return AcceptResult{}, apperr.New(apperr.Invalid, "unsupported language")
	}
	profile, ok := execution.DefaultFor(lang)
	if !ok {
		return AcceptResult{}, apperr.New(apperr.Invalid, "no execution profile for language")
	}
	if !p.EmailVerified {
		return AcceptResult{}, apperr.New(apperr.Unverified, "verify email before submitting")
	}

	sum := sha256.Sum256(source)
	hexSum := hex.EncodeToString(sum[:])
	key := "sha256/" + hexSum
	if err := m.Blobs.Put(ctx, key, bytes.NewReader(source), int64(len(source)), "text/plain"); err != nil {
		return AcceptResult{}, err
	}

	var out AcceptResult
	err := m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var c persist.Contest
		if err := tx.First(&c, "id = ?", contestID).Error; err != nil {
			return apperr.New(apperr.NotFound, "contest not found")
		}
		var mem persist.EntrantMember
		if err := tx.Where("contest_id = ? AND account_id = ?", contestID, p.AccountID).First(&mem).Error; err != nil {
			return apperr.New(apperr.Forbidden, "not registered")
		}
		var en persist.Entrant
		if err := tx.First(&en, "id = ?", mem.EntrantID).Error; err != nil {
			return err
		}
		pol := contest.Snapshot{
			Rule: domain.Rule(c.Rule), EntryMode: domain.EntryMode(c.EntryMode),
			StartsAt: c.StartsAt, EndsAt: c.EndsAt, RegisterStartsAt: c.RegisterStartsAt,
			AllowExternal: c.AllowExternal, MaxTeamSize: c.MaxTeamSize,
		}
		if err := contest.CanSubmit(pol, m.Clock.Now().UTC(), en.Status == string(domain.EntrantAccepted), p.EmailVerified); err != nil {
			return err
		}
		var cp persist.ContestProblem
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Where("contest_id = ? AND letter = ?", contestID, letter).First(&cp).Error; err != nil {
			return apperr.New(apperr.NotFound, "contest problem not found")
		}
		now := m.Clock.Now().UTC()
		var obj persist.Object
		if err := tx.Where("sha256 = ? AND size_bytes = ?", hexSum, int64(len(source))).First(&obj).Error; err != nil {
			obj = persist.Object{
				ID: uuid.New(), SHA256: hexSum, SizeBytes: int64(len(source)),
				StorageKey: key, ContentType: "text/plain", CreatedAt: now,
			}
			if err := tx.Create(&obj).Error; err != nil {
				if err := tx.Where("sha256 = ? AND size_bytes = ?", hexSum, int64(len(source))).First(&obj).Error; err != nil {
					return err
				}
			}
		}
		sub := persist.Submission{
			ID: uuid.New(), ContestID: contestID, ContestProblemID: cp.ID, ProblemID: cp.ProblemID,
			EntrantID: en.ID, SubmitterAccountID: p.AccountID, LanguageID: string(lang),
			ExecutionProfileID: profile.ID, SourceObjectID: obj.ID,
			JudgeDataRevisionID: cp.TargetRevisionID, ReceivedAt: now,
		}
		if err := tx.Create(&sub).Error; err != nil {
			return err
		}
		if err := tx.First(&sub, "id = ?", sub.ID).Error; err != nil {
			return err
		}
		task := persist.JudgeTask{
			ID: uuid.New(), SubmissionID: sub.ID, ContestID: contestID,
			JudgeDataRevisionID: cp.TargetRevisionID, ExecutionProfileID: profile.ID,
			Purpose: "initial", RejudgeBatchID: OpenBatchIDForNewSubmission(),
			State: string(domain.TaskPendingSchedule), CreatedAt: now,
		}
		if err := tx.Create(&task).Error; err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{
			"task_id": task.ID.String(), "submission_id": sub.ID.String(), "contest_id": contestID.String(),
		})
		ev := persist.OutboxEvent{ID: uuid.New(), Topic: "judge.task.ready", Payload: payload, CreatedAt: now}
		if err := tx.Create(&ev).Error; err != nil {
			return err
		}
		out = AcceptResult{Submission: sub, Task: task}
		return nil
	})
	return out, err
}

func (m *Module) Get(ctx context.Context, id uuid.UUID) (persist.Submission, error) {
	var s persist.Submission
	if err := m.DB.WithContext(ctx).First(&s, "id = ?", id).Error; err != nil {
		return persist.Submission{}, apperr.New(apperr.NotFound, "submission not found")
	}
	return s, nil
}
