package persist

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type Account struct {
	ID              uuid.UUID `gorm:"type:uuid;primaryKey"`
	Email           string
	EmailNormalized string
	PasswordHash    string
	DisplayName     string
	EmailVerifiedAt *time.Time
	SiteAdmin       bool
	DisabledAt      *time.Time
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

func (Account) TableName() string { return "accounts" }

type Session struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	AccountID  uuid.UUID `gorm:"type:uuid"`
	TokenHash  string
	CSRFSecret string `gorm:"column:csrf_secret"`
	ExpiresAt  time.Time
	RevokedAt  *time.Time
	CreatedAt  time.Time
	LastSeenAt time.Time
	UserAgent  string
	IP         string
}

func (Session) TableName() string { return "sessions" }

type EmailChallenge struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	AccountID  uuid.UUID `gorm:"type:uuid"`
	Kind       string
	TokenHash  string
	ExpiresAt  time.Time
	ConsumedAt *time.Time
	CreatedAt  time.Time
}

func (EmailChallenge) TableName() string { return "email_challenges" }

type Organization struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Slug      string
	Name      string
	CreatedAt time.Time
}

func (Organization) TableName() string { return "organizations" }

type OrganizationMembership struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrganizationID uuid.UUID `gorm:"type:uuid"`
	AccountID      uuid.UUID `gorm:"type:uuid"`
	Role           string
	CreatedAt      time.Time
}

func (OrganizationMembership) TableName() string { return "organization_memberships" }

type OrganizationJoinRequest struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrganizationID uuid.UUID `gorm:"type:uuid"`
	AccountID      uuid.UUID `gorm:"type:uuid"`
	Status         string
	CreatedAt      time.Time
	DecidedAt      *time.Time
	DecidedBy      *uuid.UUID `gorm:"type:uuid"`
}

func (OrganizationJoinRequest) TableName() string { return "organization_join_requests" }

type Object struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	SHA256      string    `gorm:"column:sha256"`
	SizeBytes   int64
	StorageKey  string
	ContentType string
	CreatedAt   time.Time
}

func (Object) TableName() string { return "objects" }

type Problem struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrganizationID uuid.UUID `gorm:"type:uuid"`
	Slug           string
	Title          string
	CreatedAt      time.Time
}

func (Problem) TableName() string { return "problems" }

type JudgeDataRevision struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProblemID        uuid.UUID `gorm:"type:uuid"`
	RevisionN        int
	NormalizedConfig json.RawMessage `gorm:"type:jsonb"`
	CreatedAt        time.Time
	CreatedBy        uuid.UUID `gorm:"type:uuid"`
}

func (JudgeDataRevision) TableName() string { return "judge_data_revisions" }

type RevisionFile struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	RevisionID uuid.UUID `gorm:"type:uuid"`
	Role       string
	Name       string
	ObjectID   uuid.UUID `gorm:"type:uuid"`
}

func (RevisionFile) TableName() string { return "revision_files" }

type Contest struct {
	ID                         uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrganizationID             uuid.UUID `gorm:"type:uuid"`
	Slug                       string
	Title                      string
	Rule                       string
	EntryMode                  string
	StartsAt                   time.Time
	EndsAt                     time.Time
	RegisterStartsAt           time.Time
	AllowExternal              bool
	RegistrationRequiresReview bool
	MaxTeamSize                int
	ACMPenaltyMinutes          int `gorm:"column:acm_penalty_minutes"`
	FreezeMinutesBeforeEnd     *int
	IOIPublicScoreboard        bool `gorm:"column:ioi_public_scoreboard"`
	CaseFeedback               string
	SourcePublicAfter          bool
	SourceVisibilityLocked     bool
	PublishedAt                *time.Time
	CreatedAt                  time.Time
}

func (Contest) TableName() string { return "contests" }

type ContestProblem struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey"`
	ContestID         uuid.UUID `gorm:"type:uuid"`
	ProblemID         uuid.UUID `gorm:"type:uuid"`
	Letter            string
	ScoringRevisionID uuid.UUID `gorm:"type:uuid"`
	TargetRevisionID  uuid.UUID `gorm:"type:uuid"`
}

func (ContestProblem) TableName() string { return "contest_problems" }

type Entrant struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	ContestID      uuid.UUID `gorm:"type:uuid"`
	Kind           string
	DisplayName    string
	Status         string
	RegisteredAt   time.Time
	AcceptedAt     *time.Time
	RosterFrozenAt *time.Time
}

func (Entrant) TableName() string { return "entrants" }

type EntrantMember struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	ContestID   uuid.UUID `gorm:"type:uuid"`
	EntrantID   uuid.UUID `gorm:"type:uuid"`
	AccountID   uuid.UUID `gorm:"type:uuid"`
	Role        string
	ConfirmedAt *time.Time
}

func (EntrantMember) TableName() string { return "entrant_members" }

type Submission struct {
	ID                  uuid.UUID `gorm:"type:uuid;primaryKey"`
	ContestID           uuid.UUID `gorm:"type:uuid"`
	ContestProblemID    uuid.UUID `gorm:"type:uuid"`
	ProblemID           uuid.UUID `gorm:"type:uuid"`
	EntrantID           uuid.UUID `gorm:"type:uuid"`
	SubmitterAccountID  uuid.UUID `gorm:"type:uuid"`
	LanguageID          string
	ExecutionProfileID  string
	SourceObjectID      uuid.UUID `gorm:"type:uuid"`
	JudgeDataRevisionID uuid.UUID `gorm:"type:uuid"`
	ReceiveSeq          int64     `gorm:"autoIncrement"`
	ReceivedAt          time.Time
}

func (Submission) TableName() string { return "submissions" }

type RejudgeBatch struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey"`
	ContestProblemID uuid.UUID `gorm:"type:uuid"`
	FromRevisionID   uuid.UUID `gorm:"type:uuid"`
	ToRevisionID     uuid.UUID `gorm:"type:uuid"`
	State            string
	CreatedBy        uuid.UUID `gorm:"type:uuid"`
	CreatedAt        time.Time
	SwitchedAt       *time.Time
}

func (RejudgeBatch) TableName() string { return "rejudge_batches" }

type JudgeTask struct {
	ID                  uuid.UUID `gorm:"type:uuid;primaryKey"`
	SubmissionID        uuid.UUID `gorm:"type:uuid"`
	ContestID           uuid.UUID `gorm:"type:uuid"`
	JudgeDataRevisionID uuid.UUID `gorm:"type:uuid"`
	ExecutionProfileID  string
	Purpose             string
	RejudgeBatchID      *uuid.UUID `gorm:"type:uuid"`
	State               string
	CreatedAt           time.Time
}

func (JudgeTask) TableName() string { return "judge_tasks" }

type RejudgeBatchItem struct {
	BatchID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	SubmissionID uuid.UUID `gorm:"type:uuid;primaryKey"`
	TaskID       uuid.UUID `gorm:"type:uuid"`
}

func (RejudgeBatchItem) TableName() string { return "rejudge_batch_items" }

type JudgePool struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	Kind           string
	OrganizationID *uuid.UUID `gorm:"type:uuid"`
	Name           string
	Enabled        bool
	CreatedAt      time.Time
}

func (JudgePool) TableName() string { return "judge_pools" }

type JudgeNode struct {
	ID                 uuid.UUID `gorm:"type:uuid;primaryKey"`
	PoolID             uuid.UUID `gorm:"type:uuid"`
	Name               string
	CredentialHash     string
	Enabled            bool
	MaxSlots           int
	LastHeartbeatAt    *time.Time
	Connected          bool
	AdvertisedProfiles JSONList `gorm:"type:jsonb"`
	RevokedAt          *time.Time
	CreatedAt          time.Time
}

func (JudgeNode) TableName() string { return "judge_nodes" }

type RoutingPolicyVersion struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey"`
	OrganizationID uuid.UUID `gorm:"type:uuid"`
	VersionN       int
	CreatedAt      time.Time
}

func (RoutingPolicyVersion) TableName() string { return "routing_policy_versions" }

type RoutingPolicyEntry struct {
	PolicyVersionID uuid.UUID `gorm:"type:uuid;primaryKey"`
	PoolID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Priority        int
}

func (RoutingPolicyEntry) TableName() string { return "routing_policy_entries" }

type JudgingAttempt struct {
	ID                     uuid.UUID `gorm:"type:uuid;primaryKey"`
	TaskID                 uuid.UUID `gorm:"type:uuid"`
	Generation             int
	NodeID                 uuid.UUID  `gorm:"type:uuid"`
	PoolID                 uuid.UUID  `gorm:"type:uuid"`
	RoutingPolicyVersionID *uuid.UUID `gorm:"type:uuid"`
	JudgeDataRevisionID    uuid.UUID  `gorm:"type:uuid"`
	ExecutionProfileID     string
	State                  string
	DispatchedAt           time.Time
	LeaseUntil             time.Time
	FinishedAt             *time.Time
	SystemFault            bool
	SystemFaultKind        string
	Verdict                string
	ScoreHundredths        *int64
	Result                 json.RawMessage `gorm:"type:jsonb"`
}

func (JudgingAttempt) TableName() string { return "judging_attempts" }

type SubmissionResult struct {
	SubmissionID    uuid.UUID `gorm:"type:uuid;primaryKey"`
	RevisionID      uuid.UUID `gorm:"type:uuid;primaryKey"`
	AttemptID       uuid.UUID `gorm:"type:uuid"`
	Verdict         string
	ScoreHundredths int64
	SubtaskScores   json.RawMessage `gorm:"type:jsonb"`
	SystemFault     bool
}

func (SubmissionResult) TableName() string { return "submission_results" }

type OutboxEvent struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey"`
	Topic       string
	Payload     json.RawMessage `gorm:"type:jsonb"`
	CreatedAt   time.Time
	PublishedAt *time.Time
}

func (OutboxEvent) TableName() string { return "outbox_events" }

type StreamConsumption struct {
	EventID     uuid.UUID `gorm:"type:uuid;primaryKey"`
	ProcessedAt time.Time
}

func (StreamConsumption) TableName() string { return "stream_consumptions" }

type DownloadGrant struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	TokenHash string
	ObjectID  uuid.UUID  `gorm:"type:uuid"`
	AttemptID *uuid.UUID `gorm:"type:uuid"`
	AccountID *uuid.UUID `gorm:"type:uuid"`
	ExpiresAt time.Time
}

func (DownloadGrant) TableName() string { return "download_grants" }

type AuditEvent struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey"`
	ActorID        *uuid.UUID `gorm:"type:uuid"`
	Action         string
	OrganizationID *uuid.UUID      `gorm:"type:uuid"`
	ContestID      *uuid.UUID      `gorm:"type:uuid"`
	SubmissionID   *uuid.UUID      `gorm:"type:uuid"`
	TaskID         *uuid.UUID      `gorm:"type:uuid"`
	AttemptID      *uuid.UUID      `gorm:"type:uuid"`
	Payload        json.RawMessage `gorm:"type:jsonb"`
	CreatedAt      time.Time
}

func (AuditEvent) TableName() string { return "audit_events" }

type JSONList []string

func (j JSONList) Value() (driver.Value, error) {
	b, err := json.Marshal([]string(j))
	return b, err
}

func (j *JSONList) Scan(value any) error {
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, j)
	case string:
		return json.Unmarshal([]byte(v), j)
	case nil:
		*j = JSONList{}
		return nil
	default:
		return fmt.Errorf("unsupported JSONList type %T", value)
	}
}
