<script setup lang="ts">
import { h } from 'vue'
import { reportError } from '~/composables/useApiToast'
import type { TableColumn } from '#ui/types'
import type {
  BankProblem,
  Contest,
  ContestProblem,
  Entrant,
  JudgeTask,
  Membership,
  ProblemRevision,
  RejudgeBatch,
  TaskState,
} from '~/types/api'

definePageMeta({ middleware: 'auth' })

const route = useRoute()
const api = useApi()
const { fail, ok } = useApiToast()

const contestId = computed(() => String(route.params.id))

const contest = ref<Contest | null>(null)
const problems = ref<ContestProblem[]>([])
const bank = ref<BankProblem[]>([])
const entrants = ref<Entrant[]>([])
const tasks = ref<JudgeTask[]>([])
const membership = ref<Membership | null>(null)

const loading = ref(true)
const loadError = ref<string | null>(null)
const tab = ref('problems')

const entrantStatus = ref<string>('pending')
const taskState = ref<string>('awaiting_admin')
const sourcePublic = ref(false)
const savingSource = ref(false)

// Add-problem form
const addOpen = ref(false)
const addState = reactive({ problemId: '', revisionId: '', letter: '' })
const revisions = ref<ProblemRevision[]>([])
const loadingRevisions = ref(false)
const adding = ref(false)

// Rejudge form
const rejudgeOpen = ref(false)
const rejudgeTarget = ref<ContestProblem | null>(null)
const rejudgeRevisionId = ref('')
const rejudgeBatch = ref<RejudgeBatch | null>(null)
const rejudging = ref(false)
const switching = ref(false)
const busyEntrant = ref<string | null>(null)
const busyTask = ref<string | null>(null)

const canManage = computed(
  () =>
    membership.value?.member === true &&
    (membership.value.role === 'owner' || membership.value.role === 'admin'),
)

const bankItems = computed(() =>
  bank.value.map((p) => ({ label: `${p.slug} · ${p.title}`, value: p.id })),
)
const revisionItems = computed(() =>
  revisions.value.map((r) => ({
    label: `第 ${r.revision_n} 版 · ${formatDateTime(r.created_at)}`,
    value: r.revision_id,
  })),
)

// Reka UI reserves the empty string for clearing a select, so "all" is spelled
// out rather than left blank.
const ALL_STATUSES = 'all'

const entrantStatusItems = [
  { label: '待审核', value: 'pending' },
  { label: '已通过', value: 'accepted' },
  { label: '已拒绝', value: 'rejected' },
  { label: '已撤回', value: 'withdrawn' },
  { label: '全部', value: ALL_STATUSES },
]
const taskStateItems = [
  { label: '等待管理员处理', value: 'awaiting_admin' },
  { label: '调度错误', value: 'scheduling_error' },
  { label: '已派发', value: 'dispatched' },
  { label: '等待调度', value: 'pending_schedule' },
  { label: '已完成', value: 'completed' },
  { label: '全部', value: ALL_STATUSES },
]

async function load() {
  loading.value = true
  loadError.value = null
  try {
    contest.value = await api.get<Contest>(`/api/contests/${contestId.value}`)
    sourcePublic.value = contest.value.source_public_after
    const orgId = contest.value.organization_id
    const [membershipView, problemList, bankList] = await Promise.all([
      api.get<Membership>(`/api/organizations/${orgId}/membership`),
      api.get<{ problems: ContestProblem[] }>(`/api/contests/${contestId.value}/problems`),
      api
        .get<{ problems: BankProblem[] }>(`/api/organizations/${orgId}/problems`)
        .catch(() => ({ problems: [] })),
    ])
    membership.value = membershipView
    problems.value = problemList.problems ?? []
    bank.value = bankList.problems ?? []
    if (canManage.value) await Promise.all([loadEntrants(), loadTasks()])
  } catch (e) {
    loadError.value = asApiError(e).message
  } finally {
    loading.value = false
  }
}

async function loadEntrants() {
  try {
    const res = await api.get<{ entrants: Entrant[] }>(
      `/api/contests/${contestId.value}/entrants`,
      entrantStatus.value === ALL_STATUSES ? undefined : { status: entrantStatus.value },
    )
    entrants.value = res.entrants ?? []
  } catch (e) {
    entrants.value = []
    if (asApiError(e).status !== 403) reportError(e, '加载报名列表失败')
  }
}

async function loadTasks() {
  try {
    const res = await api.get<{ tasks: JudgeTask[] }>(
      `/api/contests/${contestId.value}/tasks`,
      taskState.value === ALL_STATUSES ? undefined : { state: taskState.value },
    )
    tasks.value = res.tasks ?? []
  } catch (e) {
    tasks.value = []
    if (asApiError(e).status !== 403) reportError(e, '加载评测任务失败')
  }
}

onMounted(load)
watch(entrantStatus, () => {
  if (canManage.value) void loadEntrants()
})
watch(taskState, () => {
  if (canManage.value) void loadTasks()
})

async function onBankProblemChange() {
  addState.revisionId = ''
  revisions.value = []
  if (!addState.problemId) return
  loadingRevisions.value = true
  try {
    const res = await api.get<{ revisions: ProblemRevision[] }>(
      `/api/problems/${addState.problemId}/revisions`,
    )
    revisions.value = res.revisions ?? []
    // Newest first, which is what the backend returns and what an admin wants.
    if (revisions.value[0]) addState.revisionId = revisions.value[0].revision_id
  } catch (e) {
    fail(e, '加载题目修订失败')
  } finally {
    loadingRevisions.value = false
  }
}

async function addProblem() {
  if (!addState.problemId || !addState.revisionId || !addState.letter.trim()) {
    fail(new Error('请选择题库题目、修订版本并填写题号'), '添加题目失败')
    return
  }
  adding.value = true
  try {
    await api.post(`/api/contests/${contestId.value}/problems`, {
      problem_id: addState.problemId,
      revision_id: addState.revisionId,
      letter: addState.letter.trim().toUpperCase(),
    })
    ok('题目已添加到比赛')
    addOpen.value = false
    addState.problemId = ''
    addState.revisionId = ''
    addState.letter = ''
    const res = await api.get<{ problems: ContestProblem[] }>(
      `/api/contests/${contestId.value}/problems`,
    )
    problems.value = res.problems ?? []
  } catch (e) {
    fail(e, '添加题目失败')
  } finally {
    adding.value = false
  }
}

async function openRejudge(problem: ContestProblem) {
  rejudgeTarget.value = problem
  rejudgeRevisionId.value = ''
  rejudgeBatch.value = null
  rejudgeOpen.value = true
  revisions.value = []
  if (!problem.problem_id) return
  loadingRevisions.value = true
  try {
    const res = await api.get<{ revisions: ProblemRevision[] }>(
      `/api/problems/${problem.problem_id}/revisions`,
    )
    revisions.value = res.revisions ?? []
  } catch (e) {
    fail(e, '加载题目修订失败')
  } finally {
    loadingRevisions.value = false
  }
}

async function createRejudge() {
  if (!rejudgeTarget.value || !rejudgeRevisionId.value) {
    fail(new Error('请选择重测使用的评测数据版本'), '创建重测失败')
    return
  }
  rejudging.value = true
  try {
    rejudgeBatch.value = await api.post<RejudgeBatch>(
      `/api/contest-problems/${rejudgeTarget.value.id}/rejudge`,
      { revision_id: rejudgeRevisionId.value },
    )
    ok('重测批次已创建', '整批完成后需显式切换计分结果')
    await loadTasks()
  } catch (e) {
    fail(e, '创建重测失败')
  } finally {
    rejudging.value = false
  }
}

async function switchBatch() {
  if (!rejudgeBatch.value) return
  switching.value = true
  try {
    await api.post(`/api/rejudge-batches/${rejudgeBatch.value.batch_id}/switch`)
    ok('已切换到新的计分结果')
    rejudgeOpen.value = false
  } catch (e) {
    fail(e, '切换计分结果失败')
  } finally {
    switching.value = false
  }
}

async function decideEntrant(entrant: Entrant, approve: boolean) {
  busyEntrant.value = entrant.id
  try {
    await api.post(`/api/entrants/${entrant.id}/decision`, { approve })
    ok(approve ? '报名已通过' : '报名已拒绝', entrant.display_name)
    await loadEntrants()
  } catch (e) {
    fail(e, '处理报名失败')
  } finally {
    busyEntrant.value = null
  }
}

async function redispatch(task: JudgeTask) {
  busyTask.value = task.id
  try {
    await api.post(`/api/tasks/${task.id}/redispatch`)
    ok('已重新派发评测任务')
    await loadTasks()
  } catch (e) {
    fail(e, '重新派发失败')
  } finally {
    busyTask.value = null
  }
}

async function saveSourcePublic() {
  savingSource.value = true
  try {
    await api.patch(`/api/contests/${contestId.value}/source-public`, {
      source_public_after: sourcePublic.value,
    })
    ok('源码公开设置已保存')
  } catch (e) {
    fail(e, '保存失败')
  } finally {
    savingSource.value = false
  }
}

const problemColumns: TableColumn<ContestProblem>[] = [
  {
    accessorKey: 'letter',
    header: '题号',
    cell: ({ row }) => h('span', { class: 'font-mono font-semibold' }, row.original.letter),
  },
  { accessorKey: 'title', header: '标题', cell: ({ row }) => row.original.title ?? '—' },
  {
    id: 'revision',
    header: '计分版本',
    cell: ({ row }) =>
      h(
        'span',
        { class: 'font-mono text-xs text-muted' },
        row.original.scoring_revision_id?.slice(0, 8) ?? '—',
      ),
  },
  // `UTable` only renders the `<columnId>-cell` slot for a column that exists,
  // so the rejudge button needs this column to be declared here.
  { id: 'actions', header: '操作' },
]

const entrantColumns: TableColumn<Entrant>[] = [
  {
    accessorKey: 'display_name',
    header: '参赛主体',
    cell: ({ row }) => h('span', { class: 'font-medium' }, row.original.display_name),
  },
  { accessorKey: 'kind', header: '类型', cell: ({ row }) => ENTRY_MODE_LABELS[row.original.kind] },
  {
    accessorKey: 'status',
    header: '状态',
    cell: ({ row }) => h('span', { class: 'text-sm' }, ENTRANT_STATUS_LABELS[row.original.status]),
  },
  {
    id: 'roster',
    header: '队员',
    cell: ({ row }) =>
      h(
        'span',
        { class: 'text-sm text-muted' },
        row.original.members.map((m) => m.display_name).join('、') || '—',
      ),
  },
  // Same as the problems table: the approve/reject buttons live in this column's
  // `actions-cell` slot, which does not exist unless the column is declared.
  { id: 'actions', header: '操作' },
]
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="space-y-1">
        <h1 class="text-2xl font-semibold text-highlighted">
          比赛管理
          <span v-if="contest" class="text-muted">· {{ contest.title }}</span>
        </h1>
        <p class="text-sm text-muted">重测会保留旧榜并标注重测中，整批完成后统一切换计分结果。</p>
      </div>
      <UButton
        :to="`/contests/${contestId}`"
        color="neutral"
        variant="ghost"
        icon="i-lucide-arrow-left"
      >
        返回比赛
      </UButton>
    </div>

    <UAlert
      v-if="loadError"
      color="error"
      variant="subtle"
      icon="i-lucide-triangle-alert"
      title="无法加载比赛"
      :description="loadError"
    />

    <USkeleton v-else-if="loading" class="h-64 w-full" />

    <UAlert
      v-else-if="!canManage"
      color="warning"
      variant="subtle"
      icon="i-lucide-lock"
      title="权限不足"
      description="只有本场比赛所属组织的管理员可以执行这些操作。"
    />

    <UTabs
      v-else
      v-model="tab"
      :items="[
        { label: '题目', icon: 'i-lucide-list', slot: 'problems', value: 'problems' },
        { label: '报名审核', icon: 'i-lucide-user-check', slot: 'entrants', value: 'entrants' },
        { label: '评测任务', icon: 'i-lucide-server-cog', slot: 'tasks', value: 'tasks' },
        { label: '设置', icon: 'i-lucide-settings', slot: 'settings', value: 'settings' },
      ]"
    >
      <template #problems>
        <div class="space-y-4 pt-4">
          <div class="flex justify-end">
            <UButton icon="i-lucide-plus" color="primary" size="sm" @click="addOpen = true">
              添加比赛题目
            </UButton>
          </div>

          <UTable :data="problems" :columns="problemColumns" empty="比赛还没有题目">
            <template #actions-cell="{ row }">
              <div class="flex justify-end">
                <UButton
                  size="xs"
                  color="neutral"
                  variant="outline"
                  icon="i-lucide-refresh-cw"
                  @click="openRejudge(row.original)"
                >
                  重测
                </UButton>
              </div>
            </template>
          </UTable>
        </div>
      </template>

      <template #entrants>
        <div class="space-y-4 pt-4">
          <UFormField label="状态筛选" class="w-48">
            <USelect v-model="entrantStatus" :items="entrantStatusItems" class="w-full" />
          </UFormField>
          <UTable :data="entrants" :columns="entrantColumns" empty="没有符合条件的报名">
            <template #actions-cell="{ row }">
              <div v-if="row.original.status === 'pending'" class="flex justify-end gap-2">
                <UButton
                  size="xs"
                  color="primary"
                  icon="i-lucide-check"
                  :loading="busyEntrant === row.original.id"
                  @click="decideEntrant(row.original, true)"
                >
                  通过
                </UButton>
                <UButton
                  size="xs"
                  color="error"
                  variant="outline"
                  icon="i-lucide-x"
                  :loading="busyEntrant === row.original.id"
                  @click="decideEntrant(row.original, false)"
                >
                  拒绝
                </UButton>
              </div>
            </template>
          </UTable>
        </div>
      </template>

      <template #tasks>
        <div class="space-y-4 pt-4">
          <UAlert
            color="info"
            variant="subtle"
            icon="i-lucide-info"
            title="节点领取任务后失联不会自动重派"
            description="请核对节点状态后手动重新派发；旧尝试的迟到结果不会覆盖新结果。"
          />
          <UFormField label="状态筛选" class="w-56">
            <USelect v-model="taskState" :items="taskStateItems" class="w-full" />
          </UFormField>

          <UEmpty
            v-if="!tasks.length"
            icon="i-lucide-inbox"
            title="没有符合条件的评测任务"
            variant="subtle"
          />

          <ul v-else class="divide-y divide-default rounded-lg border border-default">
            <li v-for="task in tasks" :key="task.id" class="space-y-2 px-4 py-3">
              <div class="flex flex-wrap items-center gap-2">
                <UBadge :color="taskStateColor(task.state)" variant="subtle" size="sm">
                  {{ taskStateLabel(task.state) }}
                </UBadge>
                <span class="font-mono text-xs text-muted">任务 {{ task.id.slice(0, 8) }}</span>
                <UButton
                  :to="`/submissions/${task.submission_id}`"
                  size="xs"
                  color="neutral"
                  variant="ghost"
                  icon="i-lucide-arrow-right"
                  trailing
                >
                  提交 {{ task.submission_id.slice(0, 8) }}
                </UButton>
                <span class="text-xs text-muted">{{ formatRelative(task.created_at) }}</span>
                <UButton
                  size="xs"
                  color="primary"
                  variant="outline"
                  icon="i-lucide-send"
                  class="ml-auto"
                  :loading="busyTask === task.id"
                  @click="redispatch(task)"
                >
                  重新派发
                </UButton>
              </div>
              <div v-if="task.attempt" class="flex flex-wrap gap-3 text-xs text-muted">
                <span
                  >尝试 #{{ task.attempt.generation }} ·
                  {{ attemptStateLabel(task.attempt.state) }}</span
                >
                <span>节点 {{ task.attempt.node_id.slice(0, 8) }}</span>
                <span>租约至 {{ formatDateTime(task.attempt.lease_until) }}</span>
                <span v-if="task.attempt.system_fault" class="text-error">系统故障</span>
              </div>
              <p v-else class="text-xs text-muted">尚未派发到任何节点</p>
            </li>
          </ul>
        </div>
      </template>

      <template #settings>
        <div class="space-y-4 pt-4">
          <UCard>
            <template #header>
              <span class="font-medium">源码公开</span>
            </template>
            <div class="space-y-4">
              <UAlert
                v-if="contest?.source_visibility_locked"
                color="warning"
                variant="subtle"
                icon="i-lucide-lock"
                title="该设置已锁定"
                description="比赛已开赛，不能临时扩大原本保密代码的公开范围。"
              />
              <USwitch
                v-model="sourcePublic"
                label="赛后公开源码"
                :disabled="contest?.source_visibility_locked"
              />
              <p class="text-sm text-muted">
                公开后，比赛结束且全部评测完成时，源码可由已登录站点账号查看；匿名访客不可查看。
              </p>
              <UButton
                color="primary"
                :loading="savingSource"
                :disabled="contest?.source_visibility_locked"
                @click="saveSourcePublic"
              >
                保存设置
              </UButton>
            </div>
          </UCard>

          <UCard>
            <template #header>
              <span class="font-medium">比赛参数</span>
            </template>
            <dl class="grid gap-3 sm:grid-cols-2">
              <div>
                <dt class="text-sm text-muted">赛制</dt>
                <dd>{{ contest ? RULE_LABELS[contest.rule] : '—' }}</dd>
              </div>
              <div>
                <dt class="text-sm text-muted">报名类型</dt>
                <dd>{{ contest ? ENTRY_MODE_LABELS[contest.entry_mode] : '—' }}</dd>
              </div>
              <div>
                <dt class="text-sm text-muted">允许组织外参赛</dt>
                <dd>{{ contest?.allow_external ? '是' : '否' }}</dd>
              </div>
              <div>
                <dt class="text-sm text-muted">报名开始</dt>
                <dd>{{ formatDateTime(contest?.register_starts_at) }}</dd>
              </div>
              <div>
                <dt class="text-sm text-muted">比赛时间</dt>
                <dd>
                  {{ formatDateTime(contest?.starts_at) }} — {{ formatDateTime(contest?.ends_at) }}
                </dd>
              </div>
            </dl>
            <p class="mt-3 text-xs text-muted">
              赛制仅开赛前可改，报名类型创建后不可修改；当前后端未提供修改赛制的接口。
            </p>
          </UCard>
        </div>
      </template>
    </UTabs>

    <UModal v-model:open="addOpen" title="添加比赛题目">
      <template #body>
        <div class="space-y-4">
          <UFormField label="题库题目" required>
            <USelect
              v-model="addState.problemId"
              :items="bankItems"
              class="w-full"
              placeholder="选择题目"
              @update:model-value="onBankProblemChange"
            />
          </UFormField>
          <UFormField
            label="评测数据版本"
            required
            help="比赛题目绑定明确的评测数据版本，题库更新不会自动改变本场计分依据"
          >
            <USelect
              v-model="addState.revisionId"
              :items="revisionItems"
              :loading="loadingRevisions"
              :disabled="!addState.problemId"
              class="w-full"
              placeholder="选择版本"
            />
          </UFormField>
          <UFormField label="题号" required help="比赛内展示的字母，例如 A">
            <UInput v-model="addState.letter" placeholder="A" class="w-full font-mono" />
          </UFormField>
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" @click="addOpen = false">取消</UButton>
          <UButton color="primary" :loading="adding" @click="addProblem">添加</UButton>
        </div>
      </template>
    </UModal>

    <UModal v-model:open="rejudgeOpen" title="重测比赛题目">
      <template #body>
        <div class="space-y-4">
          <UAlert
            color="warning"
            variant="subtle"
            icon="i-lucide-history"
            title="重测不会产生新的提交"
            description="整批完成后统一切换计分结果；期间保留旧榜并标注重测中，不逐条混用版本。"
          />

          <p class="text-sm">
            题目
            <span class="font-mono font-semibold">{{ rejudgeTarget?.letter }}</span>
            <span v-if="rejudgeTarget?.title" class="text-muted">· {{ rejudgeTarget.title }}</span>
          </p>

          <UFormField label="使用评测数据版本" required>
            <USelect
              v-model="rejudgeRevisionId"
              :items="revisionItems"
              :loading="loadingRevisions"
              class="w-full"
              placeholder="选择版本"
            />
          </UFormField>

          <UAlert
            v-if="rejudgeBatch"
            color="info"
            variant="subtle"
            icon="i-lucide-hourglass"
            title="重测批次已创建"
            :description="`批次 ${rejudgeBatch.batch_id.slice(0, 8)} · 状态 ${rejudgeBatch.state}`"
          />
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" @click="rejudgeOpen = false">关闭</UButton>
          <UButton v-if="!rejudgeBatch" color="primary" :loading="rejudging" @click="createRejudge">
            创建重测批次
          </UButton>
          <UButton v-else color="primary" :loading="switching" @click="switchBatch">
            切换计分结果
          </UButton>
        </div>
      </template>
    </UModal>
  </div>
</template>
