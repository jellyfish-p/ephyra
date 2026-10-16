package judging

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jellyfish-p/ephyra/server/internal/access"
	"github.com/jellyfish-p/ephyra/server/internal/apperr"
	"github.com/jellyfish-p/ephyra/server/internal/clock"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/exact"
	"github.com/jellyfish-p/ephyra/server/internal/identity"
	"github.com/jellyfish-p/ephyra/server/internal/objectstore"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/jellyfish-p/ephyra/server/internal/problem"
	"github.com/jellyfish-p/ephyra/server/internal/sec"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Module struct {
	DB        *gorm.DB
	Clock     clock.Clock
	Hub       *Hub
	Blobs     objectstore.Store
	PublicURL string
}

func New(db *gorm.DB, clk clock.Clock, blobs objectstore.Store, publicURL string) *Module {
	return &Module{DB: db, Clock: clk, Hub: NewHub(), Blobs: blobs, PublicURL: strings.TrimRight(publicURL, "/")}
}

func (m *Module) now() time.Time { return m.Clock.Now().UTC() }

func (m *Module) CreatePool(ctx context.Context, p access.Principal, kind domain.PoolKind, orgID *uuid.UUID, name string) (PoolView, error) {
	if kind == domain.PoolSystem && !p.SiteAdmin {
		return PoolView{}, apperr.New(apperr.Forbidden, "only site admins can create system pools")
	}
	if kind == domain.PoolOrganization {
		if orgID == nil {
			return PoolView{}, apperr.New(apperr.Invalid, "organization pool requires organization id")
		}
		if err := m.requireManage(ctx, p, *orgID); err != nil {
			return PoolView{}, err
		}
	}
	pool := persist.JudgePool{
		ID: uuid.New(), Kind: string(kind), OrganizationID: orgID, Name: name, Enabled: true, CreatedAt: m.now(),
	}
	if err := m.DB.WithContext(ctx).Create(&pool).Error; err != nil {
		return PoolView{}, err
	}
	return PoolView{ID: pool.ID, Kind: pool.Kind, Name: pool.Name}, nil
}

func (m *Module) RegisterNode(ctx context.Context, p access.Principal, poolID uuid.UUID, name string) (NodeRegisteredView, error) {
	var pool persist.JudgePool
	if err := m.DB.WithContext(ctx).First(&pool, "id = ?", poolID).Error; err != nil {
		return NodeRegisteredView{}, apperr.New(apperr.NotFound, "pool not found")
	}
	if err := m.authorizePool(ctx, p, pool); err != nil {
		return NodeRegisteredView{}, err
	}
	raw, err := sec.RandomToken(32)
	if err != nil {
		return NodeRegisteredView{}, err
	}
	node := persist.JudgeNode{
		ID: uuid.New(), PoolID: poolID, Name: name, CredentialHash: sec.HashToken(raw),
		Enabled: true, MaxSlots: domain.DefaultMaxSlots, AdvertisedProfiles: persist.JSONList{}, CreatedAt: m.now(),
	}
	if err := m.DB.WithContext(ctx).Create(&node).Error; err != nil {
		return NodeRegisteredView{}, err
	}
	if err := persist.Audit(m.DB.WithContext(ctx), persist.AuditEvent{
		ActorID: &p.AccountID, Action: "node.register", OrganizationID: pool.OrganizationID, CreatedAt: m.now(),
	}); err != nil {
		return NodeRegisteredView{}, err
	}
	return NodeRegisteredView{ID: node.ID, Credential: raw}, nil
}

func (m *Module) RotateCredential(ctx context.Context, p access.Principal, nodeID uuid.UUID) (CredentialView, error) {
	raw, err := sec.RandomToken(32)
	if err != nil {
		return CredentialView{}, err
	}
	err = m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var node persist.JudgeNode
		if err := tx.First(&node, "id = ?", nodeID).Error; err != nil {
			return apperr.New(apperr.NotFound, "node not found")
		}
		var pool persist.JudgePool
		if err := tx.First(&pool, "id = ?", node.PoolID).Error; err != nil {
			return err
		}
		if err := m.authorizePool(ctx, p, pool); err != nil {
			return err
		}
		return tx.Model(&node).Update("credential_hash", sec.HashToken(raw)).Error
	})
	if err != nil {
		return CredentialView{}, err
	}
	return CredentialView{Credential: raw}, nil
}

func (m *Module) RevokeNode(ctx context.Context, p access.Principal, nodeID uuid.UUID) error {
	return m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var node persist.JudgeNode
		if err := tx.First(&node, "id = ?", nodeID).Error; err != nil {
			return apperr.New(apperr.NotFound, "node not found")
		}
		var pool persist.JudgePool
		if err := tx.First(&pool, "id = ?", node.PoolID).Error; err != nil {
			return err
		}
		if err := m.authorizePool(ctx, p, pool); err != nil {
			return err
		}
		now := m.now()
		if err := tx.Model(&node).Updates(map[string]any{"revoked_at": now, "enabled": false, "connected": false}).Error; err != nil {
			return err
		}
		return m.failNodeAttempts(tx, node.ID, "revoked")
	})
}

func (m *Module) SetRouting(ctx context.Context, p access.Principal, orgID uuid.UUID, entries []RoutingEntry) error {
	if err := m.requireManage(ctx, p, orgID); err != nil {
		return err
	}
	now := m.now()
	return m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var maxN int
		if err := tx.Model(&persist.RoutingPolicyVersion{}).Where("organization_id = ?", orgID).
			Select("COALESCE(MAX(version_n), 0)").Scan(&maxN).Error; err != nil {
			return err
		}
		ver := persist.RoutingPolicyVersion{ID: uuid.New(), OrganizationID: orgID, VersionN: maxN + 1, CreatedAt: now}
		if err := tx.Create(&ver).Error; err != nil {
			return err
		}
		for _, e := range entries {
			row := persist.RoutingPolicyEntry{PolicyVersionID: ver.ID, PoolID: e.PoolID, Priority: e.Priority}
			if err := tx.Create(&row).Error; err != nil {
				return err
			}
		}
		return persist.Audit(tx, persist.AuditEvent{
			ActorID: &p.AccountID, Action: "routing.set", OrganizationID: &orgID, CreatedAt: now,
		})
	})
}

type RoutingEntry struct {
	PoolID   uuid.UUID
	Priority int
}

func rejectHello(reason string) Admission {
	return Admission{Accepted: false, RejectReason: reason, ProtocolVersion: domain.ProtocolVersion}
}

func (m *Module) Handshake(ctx context.Context, hello Hello) (Admission, error) {
	if hello.ProtocolVersion != domain.ProtocolVersion {
		return rejectHello("unsupported protocol version"), nil
	}
	nodeID, err := uuid.Parse(hello.NodeID)
	if err != nil {
		return rejectHello("invalid node id"), nil
	}
	var node persist.JudgeNode
	if err := m.DB.WithContext(ctx).First(&node, "id = ?", nodeID).Error; err != nil {
		return rejectHello("unknown node"), nil
	}
	if node.RevokedAt != nil || !node.Enabled || node.CredentialHash != sec.HashToken(hello.Credential) {
		return rejectHello("invalid credentials"), nil
	}
	now := m.now()
	if err := m.DB.WithContext(ctx).Model(&node).Updates(map[string]any{
		"connected": true, "last_heartbeat_at": now, "advertised_profiles": persist.JSONList(hello.Profiles),
		"max_slots": max(int(hello.MaxSlots), 1),
	}).Error; err != nil {
		return Admission{}, err
	}
	return Admission{
		NodeID: node.ID, Accepted: true, ProtocolVersion: domain.ProtocolVersion,
		HeartbeatIntervalMs: int32(domain.HeartbeatInterval / time.Millisecond),
		LeaseMs:             int32(domain.NodeLostAfter / time.Millisecond),
	}, nil
}

func (m *Module) Heartbeat(ctx context.Context, nodeID uuid.UUID, active []string) ([]string, error) {
	now := m.now()
	if err := m.DB.WithContext(ctx).Model(&persist.JudgeNode{}).Where("id = ?", nodeID).
		Updates(map[string]any{"last_heartbeat_at": now, "connected": true}).Error; err != nil {
		return nil, err
	}
	ids := make([]uuid.UUID, 0, len(active))
	for _, s := range active {
		id, err := uuid.Parse(s)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	var dbInvalidated []string
	if len(ids) > 0 {
		var invalid []persist.JudgingAttempt
		if err := m.DB.WithContext(ctx).
			Where("node_id = ? AND state = ? AND id IN ?", nodeID, domain.AttemptInvalidated, ids).
			Find(&invalid).Error; err != nil {
			return nil, err
		}
		for _, a := range invalid {
			dbInvalidated = append(dbInvalidated, a.ID.String())
		}
	}
	return HeartbeatInvalidations(active, dbInvalidated), nil
}

func (m *Module) Disconnect(ctx context.Context, nodeID uuid.UUID) {
	_ = m.DB.WithContext(ctx).Model(&persist.JudgeNode{}).Where("id = ?", nodeID).Update("connected", false).Error
}

func (m *Module) AcceptResult(ctx context.Context, report Report) error {
	return m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var att persist.JudgingAttempt
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&att, "id = ?", report.AttemptID).Error; err != nil {
			return apperr.New(apperr.NotFound, "attempt not found")
		}
		var task persist.JudgeTask
		if err := tx.First(&task, "id = ?", att.TaskID).Error; err != nil {
			return err
		}
		var rev persist.JudgeDataRevision
		if err := tx.First(&rev, "id = ?", att.JudgeDataRevisionID).Error; err != nil {
			return err
		}
		cfg, err := unmarshalConfig(rev)
		if err != nil {
			return err
		}
		domainAtt := Attempt{
			ID: att.ID, TaskID: att.TaskID, Generation: int32(att.Generation), NodeID: att.NodeID,
			RevisionID: att.JudgeDataRevisionID, ProfileID: att.ExecutionProfileID,
			State: domain.AttemptState(att.State), HasStored: att.Result != nil,
			StoredFault: att.SystemFault, StoredVerdict: domain.Verdict(att.Verdict),
		}
		if att.ScoreHundredths != nil {
			domainAtt.StoredScore = exact.FromHundredths(*att.ScoreHundredths)
		}
		res := AcceptReport(domainAtt, cfg, report)
		if res.Action == AcceptIdempotent {
			return nil
		}
		if res.Action == AcceptReject {
			return res.Err
		}
		now := m.now()
		raw, _ := json.Marshal(report)
		updates := map[string]any{
			"finished_at": now, "result": raw, "system_fault": report.SystemFault,
			"system_fault_kind": report.FaultKind, "verdict": string(report.Verdict),
		}
		if report.SystemFault {
			updates["state"] = string(domain.AttemptFailed)
			updates["verdict"] = ""
			updates["score_hundredths"] = nil
			if err := tx.Model(&att).Updates(updates).Error; err != nil {
				return err
			}
			return tx.Model(&task).Update("state", string(domain.TaskAwaitingAdmin)).Error
		}
		h := res.Computed.Total.Hundredths()
		updates["state"] = string(domain.AttemptSucceeded)
		updates["score_hundredths"] = h
		updates["verdict"] = string(res.Computed.Verdict)
		if err := tx.Model(&att).Updates(updates).Error; err != nil {
			return err
		}
		subScores, _ := json.Marshal(res.Computed.Subtasks)
		sr := persist.SubmissionResult{
			SubmissionID: task.SubmissionID, RevisionID: att.JudgeDataRevisionID, AttemptID: att.ID,
			Verdict: string(res.Computed.Verdict), ScoreHundredths: h, SubtaskScores: subScores,
		}
		if err := tx.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "submission_id"}, {Name: "revision_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"attempt_id", "verdict", "score_hundredths", "subtask_scores", "system_fault"}),
		}).Create(&sr).Error; err != nil {
			return err
		}
		return tx.Model(&task).Update("state", string(domain.TaskCompleted)).Error
	})
}

func (m *Module) Redispatch(ctx context.Context, p access.Principal, taskID uuid.UUID) error {
	now := m.now()
	plan := PlanRedispatch()
	return m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task persist.JudgeTask
		if err := tx.First(&task, "id = ?", taskID).Error; err != nil {
			return apperr.New(apperr.NotFound, "task not found")
		}
		var c persist.Contest
		if err := tx.First(&c, "id = ?", task.ContestID).Error; err != nil {
			return err
		}
		if err := m.requireManage(ctx, p, c.OrganizationID); err != nil {
			return err
		}
		if plan.InvalidateDispatched {
			if err := tx.Model(&persist.JudgingAttempt{}).
				Where("task_id = ? AND state = ?", task.ID, domain.AttemptDispatched).
				Updates(map[string]any{"state": string(domain.AttemptInvalidated)}).Error; err != nil {
				return err
			}
		}
		if err := tx.Model(&task).Update("state", string(plan.TaskState)).Error; err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{"task_id": task.ID.String(), "reason": "admin_redispatch"})
		ev := persist.OutboxEvent{ID: uuid.New(), Topic: "judge.task.ready", Payload: payload, CreatedAt: now}
		if err := tx.Create(&ev).Error; err != nil {
			return err
		}
		return persist.Audit(tx, persist.AuditEvent{
			ActorID: &p.AccountID, Action: "task.redispatch", ContestID: &task.ContestID, TaskID: &task.ID, CreatedAt: now,
		})
	})
}

func (m *Module) CreateRejudge(ctx context.Context, p access.Principal, contestProblemID, toRevisionID uuid.UUID) (RejudgeBatchView, error) {
	var batch persist.RejudgeBatch
	err := m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var cp persist.ContestProblem
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&cp, "id = ?", contestProblemID).Error; err != nil {
			return apperr.New(apperr.NotFound, "contest problem not found")
		}
		var c persist.Contest
		if err := tx.First(&c, "id = ?", cp.ContestID).Error; err != nil {
			return err
		}
		if err := m.requireManage(ctx, p, c.OrganizationID); err != nil {
			return err
		}
		var rev persist.JudgeDataRevision
		if err := tx.First(&rev, "id = ? AND problem_id = ?", toRevisionID, cp.ProblemID).Error; err != nil {
			return apperr.New(apperr.NotFound, "revision not found")
		}
		var snapshot []persist.Submission
		if err := tx.Where("contest_problem_id = ?", cp.ID).Order("receive_seq").Find(&snapshot).Error; err != nil {
			return err
		}
		var snapshotMax int64
		if len(snapshot) > 0 {
			snapshotMax = snapshot[len(snapshot)-1].ReceiveSeq
		}
		now := m.now()
		batch = persist.RejudgeBatch{
			ID: uuid.New(), ContestProblemID: cp.ID, FromRevisionID: cp.ScoringRevisionID,
			ToRevisionID: toRevisionID, State: "running", CreatedBy: p.AccountID, CreatedAt: now,
		}
		if err := tx.Create(&batch).Error; err != nil {
			return apperr.New(apperr.Conflict, "an open rejudge batch already exists for this problem")
		}
		if err := tx.Model(&cp).Update("target_revision_id", toRevisionID).Error; err != nil {
			return err
		}
		for _, s := range snapshot {
			if !InRejudgeSnapshot(s.ReceiveSeq, snapshotMax) {
				continue
			}
			task := persist.JudgeTask{
				ID: uuid.New(), SubmissionID: s.ID, ContestID: s.ContestID,
				JudgeDataRevisionID: toRevisionID, ExecutionProfileID: s.ExecutionProfileID,
				Purpose: "rejudge", RejudgeBatchID: &batch.ID, State: string(domain.TaskPendingSchedule), CreatedAt: now,
			}
			if err := tx.Create(&task).Error; err != nil {
				return err
			}
			item := persist.RejudgeBatchItem{BatchID: batch.ID, SubmissionID: s.ID, TaskID: task.ID}
			if err := tx.Create(&item).Error; err != nil {
				return err
			}
			payload, _ := json.Marshal(map[string]any{"task_id": task.ID.String(), "batch_id": batch.ID.String()})
			ev := persist.OutboxEvent{ID: uuid.New(), Topic: "judge.task.ready", Payload: payload, CreatedAt: now}
			if err := tx.Create(&ev).Error; err != nil {
				return err
			}
		}
		return persist.Audit(tx, persist.AuditEvent{
			ActorID: &p.AccountID, Action: "rejudge.create", ContestID: &c.ID, CreatedAt: now,
		})
	})
	if err != nil {
		return RejudgeBatchView{}, err
	}
	return RejudgeBatchView{ID: batch.ID, State: batch.State}, nil
}

func (m *Module) SwitchRejudge(ctx context.Context, p access.Principal, batchID uuid.UUID) error {
	return m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var batch persist.RejudgeBatch
		if err := tx.First(&batch, "id = ?", batchID).Error; err != nil {
			return apperr.New(apperr.NotFound, "batch not found")
		}
		var cp persist.ContestProblem
		if err := tx.First(&cp, "id = ?", batch.ContestProblemID).Error; err != nil {
			return err
		}
		var c persist.Contest
		if err := tx.First(&c, "id = ?", cp.ContestID).Error; err != nil {
			return err
		}
		if err := m.requireManage(ctx, p, c.OrganizationID); err != nil {
			return err
		}
		var items []persist.RejudgeBatchItem
		if err := tx.Where("batch_id = ?", batch.ID).Find(&items).Error; err != nil {
			return err
		}
		for _, it := range items {
			var sr persist.SubmissionResult
			err := tx.Where("submission_id = ? AND revision_id = ?", it.SubmissionID, batch.ToRevisionID).First(&sr).Error
			if err != nil || sr.SystemFault || !domain.Verdict(sr.Verdict).IsNormalTerminal() {
				return apperr.New(apperr.Conflict, "batch is not ready to switch")
			}
		}
		now := m.now()
		if err := tx.Model(&cp).Update("scoring_revision_id", batch.ToRevisionID).Error; err != nil {
			return err
		}
		return tx.Model(&batch).Updates(map[string]any{"state": "switched", "switched_at": now}).Error
	})
}

func (m *Module) OpenFile(ctx context.Context, rawToken string) (BlobRef, error) {
	var g persist.DownloadGrant
	if err := m.DB.WithContext(ctx).Where("token_hash = ?", sec.HashToken(rawToken)).First(&g).Error; err != nil {
		return BlobRef{}, apperr.New(apperr.Unauthorized, "invalid download token")
	}
	if m.now().After(g.ExpiresAt) {
		return BlobRef{}, apperr.New(apperr.Unauthorized, "download token expired")
	}
	var obj persist.Object
	if err := m.DB.WithContext(ctx).First(&obj, "id = ?", g.ObjectID).Error; err != nil {
		return BlobRef{}, apperr.New(apperr.NotFound, "object not found")
	}
	return blobRef(obj), nil
}

func blobRef(obj persist.Object) BlobRef {
	return BlobRef{StorageKey: obj.StorageKey, SHA256: obj.SHA256, ContentType: obj.ContentType}
}

func (m *Module) authorizePool(ctx context.Context, p access.Principal, pool persist.JudgePool) error {
	if pool.Kind == string(domain.PoolSystem) {
		if !p.SiteAdmin {
			return apperr.New(apperr.Forbidden, "system pool is site-admin only")
		}
		return nil
	}
	if pool.OrganizationID == nil {
		return apperr.New(apperr.Forbidden, "invalid pool")
	}
	return m.requireManage(ctx, p, *pool.OrganizationID)
}

func (m *Module) requireManage(ctx context.Context, p access.Principal, orgID uuid.UUID) error {
	if p.SiteAdmin {
		return identity.AllowManageOrg(p, false, "")
	}
	var mem persist.OrganizationMembership
	err := m.DB.WithContext(ctx).Where("organization_id = ? AND account_id = ?", orgID, p.AccountID).First(&mem).Error
	return identity.AllowManageOrg(p, err == nil, domain.OrgRole(mem.Role))
}

func (m *Module) failNodeAttempts(tx *gorm.DB, nodeID uuid.UUID, reason string) error {
	var attempts []persist.JudgingAttempt
	if err := tx.Where("node_id = ? AND state = ?", nodeID, domain.AttemptDispatched).Find(&attempts).Error; err != nil {
		return err
	}
	now := m.now()
	for _, a := range attempts {
		if err := tx.Model(&a).Updates(map[string]any{
			"state": string(domain.AttemptInvalidated), "finished_at": now, "system_fault": true, "system_fault_kind": reason,
		}).Error; err != nil {
			return err
		}
		if err := tx.Model(&persist.JudgeTask{}).Where("id = ?", a.TaskID).
			Update("state", string(domain.TaskAwaitingAdmin)).Error; err != nil {
			return err
		}
	}
	return nil
}

func unmarshalConfig(rev persist.JudgeDataRevision) (problem.NormalizedConfig, error) {
	var cfg problem.NormalizedConfig
	if err := json.Unmarshal(rev.NormalizedConfig, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m *Module) grantURL(ctx context.Context, tx *gorm.DB, objectID, attemptID uuid.UUID) (string, error) {
	raw, err := sec.RandomToken(24)
	if err != nil {
		return "", err
	}
	g := persist.DownloadGrant{
		ID: uuid.New(), TokenHash: sec.HashToken(raw), ObjectID: objectID, AttemptID: &attemptID,
		ExpiresAt: m.now().Add(15 * time.Minute),
	}
	if err := tx.Create(&g).Error; err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/api/judge/files/%s", m.PublicURL, raw), nil
}
