package domain

import "time"

type Rule string

const (
	RuleACM Rule = "acm"
	RuleOI  Rule = "oi"
	RuleIOI Rule = "ioi"
)

func ParseRule(s string) (Rule, bool) {
	switch Rule(s) {
	case RuleACM, RuleOI, RuleIOI:
		return Rule(s), true
	default:
		return "", false
	}
}

type EntryMode string

const (
	EntryIndividual EntryMode = "individual"
	EntryTeam       EntryMode = "team"
)

func ParseEntryMode(s string) (EntryMode, bool) {
	switch EntryMode(s) {
	case EntryIndividual, EntryTeam:
		return EntryMode(s), true
	default:
		return "", false
	}
}

type Verdict string

const (
	VerdictAccepted            Verdict = "accepted"
	VerdictWrongAnswer         Verdict = "wrong_answer"
	VerdictTimeLimitExceeded   Verdict = "time_limit_exceeded"
	VerdictMemoryLimitExceeded Verdict = "memory_limit_exceeded"
	VerdictRuntimeError        Verdict = "runtime_error"
	VerdictCompilationError    Verdict = "compilation_error"
	VerdictSystemError         Verdict = "system_error"
)

func ParseVerdict(s string) (Verdict, bool) {
	switch Verdict(s) {
	case VerdictAccepted, VerdictWrongAnswer, VerdictTimeLimitExceeded,
		VerdictMemoryLimitExceeded, VerdictRuntimeError, VerdictCompilationError,
		VerdictSystemError:
		return Verdict(s), true
	default:
		return "", false
	}
}

func (v Verdict) IsPlayerError() bool {
	switch v {
	case VerdictWrongAnswer, VerdictTimeLimitExceeded, VerdictMemoryLimitExceeded, VerdictRuntimeError:
		return true
	default:
		return false
	}
}

func (v Verdict) IsNormalTerminal() bool {
	switch v {
	case VerdictAccepted, VerdictWrongAnswer, VerdictTimeLimitExceeded,
		VerdictMemoryLimitExceeded, VerdictRuntimeError, VerdictCompilationError:
		return true
	default:
		return false
	}
}

func (v Verdict) CountsTowardACMPenalty() bool {
	return v.IsPlayerError()
}

type LanguageID string

const (
	LangCPP23 LanguageID = "cpp23"
	LangC17   LanguageID = "c17"
	LangC23   LanguageID = "c23"
)

func ParseLanguage(s string) (LanguageID, bool) {
	switch LanguageID(s) {
	case LangCPP23, LangC17, LangC23:
		return LanguageID(s), true
	default:
		return "", false
	}
}

type ProblemKind string

const (
	KindBatch       ProblemKind = "batch"
	KindInteractive ProblemKind = "interactive"
)

type CheckerKind string

const (
	CheckerExact   CheckerKind = "exact"
	CheckerToken   CheckerKind = "token"
	CheckerTestlib CheckerKind = "testlib"
)

type FeedbackGranularity string

const (
	FeedbackCases    FeedbackGranularity = "case"
	FeedbackSubtasks FeedbackGranularity = "subtask"
)

type ContestPhase string

const (
	PhaseBefore            ContestPhase = "before"
	PhaseRunning           ContestPhase = "running"
	PhaseEndedUnpublished  ContestPhase = "ended_unpublished"
	PhasePublished         ContestPhase = "published"
)

func Phase(now, start, end time.Time, published bool) ContestPhase {
	switch {
	case published:
		return PhasePublished
	case now.Before(start):
		return PhaseBefore
	case now.Before(end):
		return PhaseRunning
	default:
		return PhaseEndedUnpublished
	}
}

type OrgRole string

const (
	OrgRoleOwner  OrgRole = "owner"
	OrgRoleAdmin  OrgRole = "admin"
	OrgRoleMember OrgRole = "member"
)

func (r OrgRole) CanManageOrg() bool {
	return r == OrgRoleOwner || r == OrgRoleAdmin
}

type PoolKind string

const (
	PoolSystem       PoolKind = "system"
	PoolOrganization PoolKind = "organization"
)

type TaskState string

const (
	TaskPendingSchedule TaskState = "pending_schedule"
	TaskDispatched      TaskState = "dispatched"
	TaskCompleted       TaskState = "completed"
	TaskAwaitingAdmin   TaskState = "awaiting_admin"
	TaskSchedulingError TaskState = "scheduling_error"
)

type AttemptState string

const (
	AttemptDispatched  AttemptState = "dispatched"
	AttemptSucceeded   AttemptState = "succeeded"
	AttemptFailed      AttemptState = "failed"
	AttemptInvalidated AttemptState = "invalidated"
)

type EntrantStatus string

const (
	EntrantPending   EntrantStatus = "pending"
	EntrantAccepted  EntrantStatus = "accepted"
	EntrantWithdrawn EntrantStatus = "withdrawn"
	EntrantRejected  EntrantStatus = "rejected"
)

const ProtocolVersion = "1"

const DefaultACMPenaltyMinutes = 20
const DefaultMaxSlots = 1
const HeartbeatInterval = 10 * time.Second
const NodeLostAfter = 60 * time.Second
const MaxSourceBytes = 256 * 1024
const MaxUncompressedZip = 64 << 20
const MaxZipFiles = 512
const MaxZipFileBytes = 32 << 20
