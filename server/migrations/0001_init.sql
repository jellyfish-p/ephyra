CREATE TABLE accounts (
    id UUID PRIMARY KEY,
    email TEXT NOT NULL,
    email_normalized TEXT NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    display_name TEXT NOT NULL,
    email_verified_at TIMESTAMPTZ,
    site_admin BOOLEAN NOT NULL DEFAULT FALSE,
    disabled_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE sessions (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts (id),
    token_hash TEXT NOT NULL UNIQUE,
    csrf_secret TEXT NOT NULL,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    user_agent TEXT NOT NULL DEFAULT '',
    ip TEXT NOT NULL DEFAULT ''
);

CREATE TABLE email_challenges (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts (id),
    kind TEXT NOT NULL,
    token_hash TEXT NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE organizations (
    id UUID PRIMARY KEY,
    slug TEXT NOT NULL UNIQUE,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE organization_memberships (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations (id),
    account_id UUID NOT NULL REFERENCES accounts (id),
    role TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (organization_id, account_id)
);

CREATE TABLE organization_join_requests (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations (id),
    account_id UUID NOT NULL REFERENCES accounts (id),
    status TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    decided_at TIMESTAMPTZ,
    decided_by UUID REFERENCES accounts (id)
);

CREATE UNIQUE INDEX organization_join_requests_pending
    ON organization_join_requests (organization_id, account_id)
    WHERE status = 'pending';

CREATE TABLE objects (
    id UUID PRIMARY KEY,
    sha256 TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    storage_key TEXT NOT NULL UNIQUE,
    content_type TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX objects_sha256_size ON objects (sha256, size_bytes);

CREATE TABLE problems (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations (id),
    slug TEXT NOT NULL,
    title TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (organization_id, slug)
);

CREATE TABLE judge_data_revisions (
    id UUID PRIMARY KEY,
    problem_id UUID NOT NULL REFERENCES problems (id),
    revision_n INT NOT NULL,
    normalized_config JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    created_by UUID NOT NULL REFERENCES accounts (id),
    UNIQUE (problem_id, revision_n)
);

CREATE TABLE revision_files (
    id UUID PRIMARY KEY,
    revision_id UUID NOT NULL REFERENCES judge_data_revisions (id),
    role TEXT NOT NULL,
    name TEXT NOT NULL,
    object_id UUID NOT NULL REFERENCES objects (id),
    UNIQUE (revision_id, name)
);

CREATE TABLE contests (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations (id),
    slug TEXT NOT NULL,
    title TEXT NOT NULL,
    rule TEXT NOT NULL,
    entry_mode TEXT NOT NULL,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at TIMESTAMPTZ NOT NULL,
    register_starts_at TIMESTAMPTZ NOT NULL,
    allow_external BOOLEAN NOT NULL DEFAULT FALSE,
    registration_requires_review BOOLEAN NOT NULL DEFAULT FALSE,
    max_team_size INT NOT NULL DEFAULT 1,
    acm_penalty_minutes INT NOT NULL DEFAULT 20,
    freeze_minutes_before_end INT,
    ioi_public_scoreboard BOOLEAN NOT NULL DEFAULT FALSE,
    case_feedback TEXT NOT NULL DEFAULT 'subtask',
    source_public_after BOOLEAN NOT NULL DEFAULT FALSE,
    source_visibility_locked BOOLEAN NOT NULL DEFAULT FALSE,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (organization_id, slug),
    CHECK (rule IN ('acm', 'oi', 'ioi')),
    CHECK (entry_mode IN ('individual', 'team')),
    CHECK (max_team_size >= 1)
);

CREATE TABLE contest_problems (
    id UUID PRIMARY KEY,
    contest_id UUID NOT NULL REFERENCES contests (id),
    problem_id UUID NOT NULL REFERENCES problems (id),
    letter TEXT NOT NULL,
    scoring_revision_id UUID NOT NULL REFERENCES judge_data_revisions (id),
    target_revision_id UUID NOT NULL REFERENCES judge_data_revisions (id),
    UNIQUE (contest_id, letter),
    UNIQUE (contest_id, problem_id)
);

CREATE TABLE entrants (
    id UUID PRIMARY KEY,
    contest_id UUID NOT NULL REFERENCES contests (id),
    kind TEXT NOT NULL,
    display_name TEXT NOT NULL,
    status TEXT NOT NULL,
    registered_at TIMESTAMPTZ NOT NULL,
    accepted_at TIMESTAMPTZ,
    roster_frozen_at TIMESTAMPTZ
);

CREATE TABLE entrant_members (
    id UUID PRIMARY KEY,
    contest_id UUID NOT NULL REFERENCES contests (id),
    entrant_id UUID NOT NULL REFERENCES entrants (id),
    account_id UUID NOT NULL REFERENCES accounts (id),
    role TEXT NOT NULL,
    confirmed_at TIMESTAMPTZ,
    UNIQUE (contest_id, account_id)
);

CREATE TABLE submissions (
    id UUID PRIMARY KEY,
    contest_id UUID NOT NULL REFERENCES contests (id),
    contest_problem_id UUID NOT NULL REFERENCES contest_problems (id),
    problem_id UUID NOT NULL REFERENCES problems (id),
    entrant_id UUID NOT NULL REFERENCES entrants (id),
    submitter_account_id UUID NOT NULL REFERENCES accounts (id),
    language_id TEXT NOT NULL,
    execution_profile_id TEXT NOT NULL,
    source_object_id UUID NOT NULL REFERENCES objects (id),
    judge_data_revision_id UUID NOT NULL REFERENCES judge_data_revisions (id),
    receive_seq BIGSERIAL UNIQUE,
    received_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX submissions_entrant_problem ON submissions (entrant_id, problem_id, receive_seq);
CREATE INDEX submissions_contest ON submissions (contest_id, receive_seq);

CREATE TABLE rejudge_batches (
    id UUID PRIMARY KEY,
    contest_problem_id UUID NOT NULL REFERENCES contest_problems (id),
    from_revision_id UUID NOT NULL REFERENCES judge_data_revisions (id),
    to_revision_id UUID NOT NULL REFERENCES judge_data_revisions (id),
    state TEXT NOT NULL,
    created_by UUID NOT NULL REFERENCES accounts (id),
    created_at TIMESTAMPTZ NOT NULL,
    switched_at TIMESTAMPTZ
);

CREATE UNIQUE INDEX rejudge_batches_open
    ON rejudge_batches (contest_problem_id)
    WHERE state IN ('running', 'ready_to_switch');

CREATE TABLE judge_tasks (
    id UUID PRIMARY KEY,
    submission_id UUID NOT NULL REFERENCES submissions (id),
    contest_id UUID NOT NULL REFERENCES contests (id),
    judge_data_revision_id UUID NOT NULL REFERENCES judge_data_revisions (id),
    execution_profile_id TEXT NOT NULL,
    purpose TEXT NOT NULL,
    rejudge_batch_id UUID REFERENCES rejudge_batches (id),
    state TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX judge_tasks_state ON judge_tasks (state, created_at);

CREATE TABLE rejudge_batch_items (
    batch_id UUID NOT NULL REFERENCES rejudge_batches (id),
    submission_id UUID NOT NULL REFERENCES submissions (id),
    task_id UUID NOT NULL REFERENCES judge_tasks (id),
    PRIMARY KEY (batch_id, submission_id)
);

CREATE TABLE judge_pools (
    id UUID PRIMARY KEY,
    kind TEXT NOT NULL,
    organization_id UUID REFERENCES organizations (id),
    name TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE judge_nodes (
    id UUID PRIMARY KEY,
    pool_id UUID NOT NULL REFERENCES judge_pools (id),
    name TEXT NOT NULL,
    credential_hash TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT TRUE,
    max_slots INT NOT NULL DEFAULT 1,
    last_heartbeat_at TIMESTAMPTZ,
    connected BOOLEAN NOT NULL DEFAULT FALSE,
    advertised_profiles JSONB NOT NULL DEFAULT '[]',
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE routing_policy_versions (
    id UUID PRIMARY KEY,
    organization_id UUID NOT NULL REFERENCES organizations (id),
    version_n INT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (organization_id, version_n)
);

CREATE TABLE routing_policy_entries (
    policy_version_id UUID NOT NULL REFERENCES routing_policy_versions (id),
    pool_id UUID NOT NULL REFERENCES judge_pools (id),
    priority INT NOT NULL,
    PRIMARY KEY (policy_version_id, pool_id)
);

CREATE TABLE judging_attempts (
    id UUID PRIMARY KEY,
    task_id UUID NOT NULL REFERENCES judge_tasks (id),
    generation INT NOT NULL,
    node_id UUID NOT NULL REFERENCES judge_nodes (id),
    pool_id UUID NOT NULL REFERENCES judge_pools (id),
    routing_policy_version_id UUID,
    judge_data_revision_id UUID NOT NULL REFERENCES judge_data_revisions (id),
    execution_profile_id TEXT NOT NULL,
    state TEXT NOT NULL,
    dispatched_at TIMESTAMPTZ NOT NULL,
    lease_until TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ,
    system_fault BOOLEAN NOT NULL DEFAULT FALSE,
    system_fault_kind TEXT NOT NULL DEFAULT '',
    verdict TEXT NOT NULL DEFAULT '',
    score_hundredths BIGINT,
    result JSONB,
    UNIQUE (task_id, generation)
);

CREATE INDEX judging_attempts_node_state ON judging_attempts (node_id, state);

CREATE TABLE submission_results (
    submission_id UUID NOT NULL REFERENCES submissions (id),
    revision_id UUID NOT NULL REFERENCES judge_data_revisions (id),
    attempt_id UUID NOT NULL REFERENCES judging_attempts (id),
    verdict TEXT NOT NULL,
    score_hundredths BIGINT NOT NULL,
    subtask_scores JSONB NOT NULL,
    system_fault BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (submission_id, revision_id)
);

CREATE TABLE outbox_events (
    id UUID PRIMARY KEY,
    topic TEXT NOT NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ
);

CREATE INDEX outbox_unpublished ON outbox_events (created_at) WHERE published_at IS NULL;

CREATE TABLE stream_consumptions (
    event_id UUID PRIMARY KEY,
    processed_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE download_grants (
    id UUID PRIMARY KEY,
    token_hash TEXT NOT NULL UNIQUE,
    object_id UUID NOT NULL REFERENCES objects (id),
    attempt_id UUID REFERENCES judging_attempts (id),
    account_id UUID REFERENCES accounts (id),
    expires_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE audit_events (
    id UUID PRIMARY KEY,
    actor_id UUID REFERENCES accounts (id),
    action TEXT NOT NULL,
    organization_id UUID,
    contest_id UUID,
    submission_id UUID,
    task_id UUID,
    attempt_id UUID,
    payload JSONB NOT NULL DEFAULT '{}',
    created_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE csrf_anonymous (
    token_hash TEXT PRIMARY KEY,
    expires_at TIMESTAMPTZ NOT NULL
);
