import type {
  AttemptState,
  CheckerKind,
  ContestPhase,
  ContestRule,
  EntrantStatus,
  EntryMode,
  FeedbackLevel,
  LanguageID,
  OrgRole,
  PoolKind,
  ProblemKind,
  TaskState,
  Verdict,
} from '~/types/api'

export type BadgeColor =
  'primary' | 'secondary' | 'success' | 'info' | 'warning' | 'error' | 'neutral'

export const VERDICT_LABELS: Record<Verdict, string> = {
  accepted: '通过',
  wrong_answer: '答案错误',
  time_limit_exceeded: '超时',
  memory_limit_exceeded: '内存超限',
  runtime_error: '运行错误',
  compilation_error: '编译错误',
  system_error: '系统错误',
}

export const VERDICT_SHORT: Record<Verdict, string> = {
  accepted: 'AC',
  wrong_answer: 'WA',
  time_limit_exceeded: 'TLE',
  memory_limit_exceeded: 'MLE',
  runtime_error: 'RE',
  compilation_error: 'CE',
  system_error: 'SE',
}

export const VERDICT_COLORS: Record<Verdict, BadgeColor> = {
  accepted: 'success',
  wrong_answer: 'error',
  time_limit_exceeded: 'warning',
  memory_limit_exceeded: 'warning',
  runtime_error: 'error',
  compilation_error: 'neutral',
  system_error: 'error',
}

export function verdictLabel(verdict: string | undefined): string {
  if (!verdict) return '等待评测'
  return VERDICT_LABELS[verdict as Verdict] ?? verdict
}

export function verdictShort(verdict: string | undefined): string {
  if (!verdict) return '…'
  return VERDICT_SHORT[verdict as Verdict] ?? verdict
}

export function verdictColor(verdict: string | undefined): BadgeColor {
  if (!verdict) return 'neutral'
  return VERDICT_COLORS[verdict as Verdict] ?? 'neutral'
}

export const RULE_LABELS: Record<ContestRule, string> = {
  acm: 'ACM',
  oi: 'OI',
  ioi: 'IOI',
}

export const RULE_DESCRIPTIONS: Record<ContestRule, string> = {
  acm: '按通过题数与总罚时排名',
  oi: '按总分排名，取同题最后一次提交',
  ioi: '按总分排名，取同题最高分提交',
}

export const ENTRY_MODE_LABELS: Record<EntryMode, string> = {
  individual: '个人',
  team: '队伍',
}

export const ENTRANT_STATUS_LABELS: Record<EntrantStatus, string> = {
  pending: '待审核',
  accepted: '已通过',
  withdrawn: '已撤回',
  rejected: '已拒绝',
}

export const ENTRANT_STATUS_COLORS: Record<EntrantStatus, BadgeColor> = {
  pending: 'warning',
  accepted: 'success',
  withdrawn: 'neutral',
  rejected: 'error',
}

export const PHASE_LABELS: Record<ContestPhase, string> = {
  before: '未开始',
  running: '进行中',
  ended_unpublished: '已结束 · 待公布',
  published: '已公布',
}

export const PHASE_COLORS: Record<ContestPhase, BadgeColor> = {
  before: 'info',
  running: 'success',
  ended_unpublished: 'warning',
  published: 'neutral',
}

export const ROLE_LABELS: Record<OrgRole, string> = {
  owner: '所有者',
  admin: '管理员',
  member: '成员',
}

export function roleLabel(role: string | undefined): string {
  if (!role) return '—'
  if (role === 'site_admin') return '站点管理员'
  return ROLE_LABELS[role as OrgRole] ?? role
}

export const POOL_KIND_LABELS: Record<PoolKind, string> = {
  system: '系统资源池',
  organization: '组织独享池',
}

export const PROBLEM_KIND_LABELS: Record<ProblemKind, string> = {
  batch: '批处理',
  interactive: '交互题',
}

export const CHECKER_LABELS: Record<CheckerKind, string> = {
  exact: '全文比对',
  token: '逐词比对',
  testlib: '自定义校验器 (testlib)',
}

export const LANGUAGE_LABELS: Record<LanguageID, string> = {
  cpp23: 'C++23',
  c17: 'C17',
  c23: 'C23',
}

export function languageLabel(id: string | undefined): string {
  if (!id) return '—'
  return LANGUAGE_LABELS[id as LanguageID] ?? id
}

export const FEEDBACK_LABELS: Record<FeedbackLevel, string> = {
  none: '不可见',
  compile: '仅编译状态',
  score: '分数',
  full: '完整反馈',
}

export const TASK_STATE_LABELS: Record<TaskState, string> = {
  pending_schedule: '等待调度',
  dispatched: '已派发',
  completed: '已完成',
  awaiting_admin: '等待管理员处理',
  scheduling_error: '调度错误',
}

export const TASK_STATE_COLORS: Record<TaskState, BadgeColor> = {
  pending_schedule: 'info',
  dispatched: 'primary',
  completed: 'success',
  awaiting_admin: 'warning',
  scheduling_error: 'error',
}

export function taskStateLabel(state: string | undefined): string {
  if (!state) return '—'
  return TASK_STATE_LABELS[state as TaskState] ?? state
}

export function taskStateColor(state: string | undefined): BadgeColor {
  if (!state) return 'neutral'
  return TASK_STATE_COLORS[state as TaskState] ?? 'neutral'
}

export const ATTEMPT_STATE_LABELS: Record<AttemptState, string> = {
  dispatched: '执行中',
  succeeded: '成功',
  failed: '失败',
  invalidated: '已失效',
}

export function attemptStateLabel(state: string | undefined): string {
  if (!state) return '—'
  return ATTEMPT_STATE_LABELS[state as AttemptState] ?? state
}
