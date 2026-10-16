package judging

import (
	"context"
	"encoding/json"

	"github.com/google/uuid"
	judgev1 "github.com/jellyfish-p/ephyra/protocol/judge/v1"
	"github.com/jellyfish-p/ephyra/server/internal/domain"
	"github.com/jellyfish-p/ephyra/server/internal/persist"
	"github.com/jellyfish-p/ephyra/server/internal/problem"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (m *Module) Schedule(ctx context.Context) error {
	var rows []struct {
		ID             uuid.UUID `gorm:"column:id"`
		OrganizationID uuid.UUID `gorm:"column:organization_id"`
		ReceiveSeq     int64     `gorm:"column:receive_seq"`
	}
	if err := m.DB.WithContext(ctx).
		Table("judge_tasks").
		Select("judge_tasks.id, contests.organization_id, submissions.receive_seq").
		Joins("JOIN submissions ON submissions.id = judge_tasks.submission_id").
		Joins("JOIN contests ON contests.id = judge_tasks.contest_id").
		Where("judge_tasks.state = ?", domain.TaskPendingSchedule).
		Find(&rows).Error; err != nil {
		return err
	}
	pending := make([]PendingWork, 0, len(rows))
	for _, r := range rows {
		pending = append(pending, PendingWork{TaskID: r.ID, OrgID: r.OrganizationID, ReceiveSeq: r.ReceiveSeq})
	}
	for _, t := range OrderPendingWork(pending) {
		if err := m.dispatchOne(ctx, t.TaskID); err != nil {
			continue
		}
	}
	return nil
}

func (m *Module) ReapLost(ctx context.Context) error {
	cut := m.now().Add(-domain.NodeLostAfter)
	var nodes []persist.JudgeNode
	if err := m.DB.WithContext(ctx).
		Where("enabled = TRUE AND (last_heartbeat_at IS NULL OR last_heartbeat_at < ?)", cut).
		Find(&nodes).Error; err != nil {
		return err
	}
	return m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, n := range nodes {
			if err := tx.Model(&n).Update("connected", false).Error; err != nil {
				return err
			}
			if err := m.failNodeAttempts(tx, n.ID, "lost"); err != nil {
				return err
			}
			_ = persist.Audit(tx, persist.AuditEvent{Action: "node.lost", CreatedAt: m.now()})
		}
		return nil
	})
}

func (m *Module) dispatchOne(ctx context.Context, taskID uuid.UUID) error {
	var send *judgev1.ServerMessage
	var nodeID uuid.UUID
	var attemptID uuid.UUID
	err := m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var task persist.JudgeTask
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&task, "id = ?", taskID).Error; err != nil {
			return err
		}
		if task.State != string(domain.TaskPendingSchedule) {
			return nil
		}
		var sub persist.Submission
		if err := tx.First(&sub, "id = ?", task.SubmissionID).Error; err != nil {
			return err
		}
		var c persist.Contest
		if err := tx.First(&c, "id = ?", task.ContestID).Error; err != nil {
			return err
		}
		pools, policyID, err := m.poolSnapshots(tx, c.OrganizationID, task.ExecutionProfileID)
		if err != nil {
			return tx.Model(&task).Update("state", string(domain.TaskSchedulingError)).Error
		}
		dec, err := SelectPool(pools)
		if err != nil {
			return tx.Model(&task).Update("state", string(domain.TaskSchedulingError)).Error
		}
		if dec.Wait {
			return nil
		}
		node, occupied, err := m.pickNode(tx, dec.PoolID, task.ExecutionProfileID)
		if err != nil {
			return tx.Model(&task).Update("state", string(domain.TaskSchedulingError)).Error
		}
		if !SlotAvailable(node.MaxSlots, occupied) {
			return nil
		}
		var gen int
		if err := tx.Model(&persist.JudgingAttempt{}).Where("task_id = ?", task.ID).
			Select("COALESCE(MAX(generation), 0)").Scan(&gen).Error; err != nil {
			return err
		}
		now := m.now()
		att := persist.JudgingAttempt{
			ID: uuid.New(), TaskID: task.ID, Generation: gen + 1, NodeID: node.ID, PoolID: node.PoolID,
			RoutingPolicyVersionID: policyID, JudgeDataRevisionID: task.JudgeDataRevisionID,
			ExecutionProfileID: task.ExecutionProfileID, State: string(domain.AttemptDispatched),
			DispatchedAt: now, LeaseUntil: now.Add(domain.NodeLostAfter),
		}
		if err := tx.Create(&att).Error; err != nil {
			return err
		}
		if err := tx.Model(&task).Update("state", string(domain.TaskDispatched)).Error; err != nil {
			return err
		}
		msg, err := m.buildDispatch(ctx, tx, task, sub, att)
		if err != nil {
			return err
		}
		send = msg
		nodeID = node.ID
		attemptID = att.ID
		return nil
	})
	if err != nil || send == nil {
		return err
	}
	if !m.Hub.Dispatch(nodeID, send) {
		return m.markDispatchSendFailed(ctx, taskID, attemptID)
	}
	return nil
}

func (m *Module) markDispatchSendFailed(ctx context.Context, taskID, attemptID uuid.UUID) error {
	attState, taskState, kind := DispatchSendFailure()
	now := m.now()
	return m.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&persist.JudgingAttempt{}).
			Where("id = ? AND task_id = ?", attemptID, taskID).
			Updates(map[string]any{
				"state": string(attState), "system_fault": true, "system_fault_kind": kind, "finished_at": now,
			}).Error; err != nil {
			return err
		}
		return tx.Model(&persist.JudgeTask{}).Where("id = ?", taskID).
			Update("state", string(taskState)).Error
	})
}

func (m *Module) poolSnapshots(tx *gorm.DB, orgID uuid.UUID, profile string) ([]PoolSnapshot, *uuid.UUID, error) {
	var ver persist.RoutingPolicyVersion
	err := tx.Where("organization_id = ?", orgID).Order("version_n DESC").First(&ver).Error
	if err != nil {
		return nil, nil, err
	}
	var entries []persist.RoutingPolicyEntry
	if err := tx.Where("policy_version_id = ?", ver.ID).Find(&entries).Error; err != nil {
		return nil, nil, err
	}
	out := make([]PoolSnapshot, 0, len(entries))
	for _, e := range entries {
		var pool persist.JudgePool
		if err := tx.First(&pool, "id = ?", e.PoolID).Error; err != nil {
			continue
		}
		if pool.Kind == string(domain.PoolOrganization) && (pool.OrganizationID == nil || *pool.OrganizationID != orgID) {
			continue
		}
		snap := PoolSnapshot{ID: pool.ID, Priority: e.Priority, Enabled: pool.Enabled, HasProfile: true}
		if !pool.Enabled {
			out = append(out, snap)
			continue
		}
		var nodes []persist.JudgeNode
		if err := tx.Where("pool_id = ? AND enabled = TRUE AND connected = TRUE AND revoked_at IS NULL AND jsonb_exists(advertised_profiles, ?)", pool.ID, profile).
			Find(&nodes).Error; err != nil {
			return nil, nil, err
		}
		snap.HealthyNodes = len(nodes)
		if len(nodes) == 0 {
			snap.Offline = true
			snap.HasProfile = false
			var anyCapable int64
			if err := tx.Model(&persist.JudgeNode{}).
				Where("pool_id = ? AND enabled = TRUE AND revoked_at IS NULL AND jsonb_exists(advertised_profiles, ?)", pool.ID, profile).
				Count(&anyCapable).Error; err != nil {
				return nil, nil, err
			}
			snap.HasProfile = anyCapable > 0
			out = append(out, snap)
			continue
		}
		free := 0
		for _, n := range nodes {
			var occ int64
			if err := tx.Model(&persist.JudgingAttempt{}).
				Where("node_id = ? AND state = ?", n.ID, domain.AttemptDispatched).
				Count(&occ).Error; err != nil {
				return nil, nil, err
			}
			slots := n.MaxSlots - int(occ)
			if slots > 0 {
				free += slots
			}
		}
		snap.FreeSlots = free
		snap.Offline = false
		out = append(out, snap)
	}
	id := ver.ID
	return out, &id, nil
}

func (m *Module) pickNode(tx *gorm.DB, poolID uuid.UUID, profile string) (persist.JudgeNode, int, error) {
	var nodes []persist.JudgeNode
	if err := tx.Where("pool_id = ? AND enabled = TRUE AND connected = TRUE AND revoked_at IS NULL AND jsonb_exists(advertised_profiles, ?)", poolID, profile).
		Find(&nodes).Error; err != nil {
		return persist.JudgeNode{}, 0, err
	}
	var best persist.JudgeNode
	bestFree := -1
	bestOcc := 0
	for _, n := range nodes {
		var occ int64
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
			Model(&persist.JudgingAttempt{}).
			Where("node_id = ? AND state = ?", n.ID, domain.AttemptDispatched).
			Count(&occ).Error; err != nil {
			return persist.JudgeNode{}, 0, err
		}
		free := n.MaxSlots - int(occ)
		if free > bestFree {
			bestFree = free
			best = n
			bestOcc = int(occ)
		}
	}
	if best.ID == uuid.Nil {
		return persist.JudgeNode{}, 0, gorm.ErrRecordNotFound
	}
	return best, bestOcc, nil
}

func (m *Module) buildDispatch(ctx context.Context, tx *gorm.DB, task persist.JudgeTask, sub persist.Submission, att persist.JudgingAttempt) (*judgev1.ServerMessage, error) {
	var src persist.Object
	if err := tx.First(&src, "id = ?", sub.SourceObjectID).Error; err != nil {
		return nil, err
	}
	srcURL, err := m.grantURL(ctx, tx, src.ID, att.ID)
	if err != nil {
		return nil, err
	}
	var files []persist.RevisionFile
	if err := tx.Where("revision_id = ?", task.JudgeDataRevisionID).Find(&files).Error; err != nil {
		return nil, err
	}
	var dataRefs []*judgev1.FileRef
	for _, f := range files {
		var obj persist.Object
		if err := tx.First(&obj, "id = ?", f.ObjectID).Error; err != nil {
			return nil, err
		}
		url, err := m.grantURL(ctx, tx, obj.ID, att.ID)
		if err != nil {
			return nil, err
		}
		dataRefs = append(dataRefs, &judgev1.FileRef{
			ObjectId: obj.ID.String(), Sha256Hex: obj.SHA256, SizeBytes: obj.SizeBytes, DownloadUrl: url,
		})
	}
	var rev persist.JudgeDataRevision
	if err := tx.First(&rev, "id = ?", task.JudgeDataRevisionID).Error; err != nil {
		return nil, err
	}
	var cfg problem.NormalizedConfig
	if err := json.Unmarshal(rev.NormalizedConfig, &cfg); err != nil {
		return nil, err
	}
	return &judgev1.ServerMessage{Body: &judgev1.ServerMessage_Dispatch{Dispatch: &judgev1.DispatchAttempt{
		AttemptId: att.ID.String(), TaskId: task.ID.String(), Generation: int32(att.Generation),
		JudgeDataRevisionId: task.JudgeDataRevisionID.String(), ExecutionProfileId: task.ExecutionProfileID,
		LanguageId: sub.LanguageID,
		Source:     &judgev1.FileRef{ObjectId: src.ID.String(), Sha256Hex: src.SHA256, SizeBytes: src.SizeBytes, DownloadUrl: srcURL},
		DataFiles:  dataRefs, NormalizedConfigJson: string(rev.NormalizedConfig),
		TimeLimitMs: cfg.TimeLimitMS, MemoryLimitBytes: cfg.MemoryLimitBytes,
	}}}, nil
}

func (m *Module) EnsureSystemPool(ctx context.Context) (persist.JudgePool, error) {
	var pool persist.JudgePool
	err := m.DB.WithContext(ctx).Where("kind = ?", domain.PoolSystem).First(&pool).Error
	if err == nil {
		return pool, nil
	}
	pool = persist.JudgePool{ID: uuid.New(), Kind: string(domain.PoolSystem), Name: "system", Enabled: true, CreatedAt: m.now()}
	if err := m.DB.WithContext(ctx).Create(&pool).Error; err != nil {
		return persist.JudgePool{}, err
	}
	return pool, nil
}
