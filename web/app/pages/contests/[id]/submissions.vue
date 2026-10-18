<script setup lang="ts">
import { reportError } from '~/composables/useApiToast'
import type { Contest, ContestProblem, Entrant, Submission } from '~/types/api'

definePageMeta({ middleware: 'auth' })

const route = useRoute()
const api = useApi()

const contestId = computed(() => String(route.params.id))

const contest = ref<Contest | null>(null)
const problems = ref<ContestProblem[]>([])
const entrant = ref<Entrant | null>(null)
const submissions = ref<Submission[]>([])
const loading = ref(true)
const loadingMore = ref(false)
const loadError = ref<string | null>(null)
const exhausted = ref(false)

/**
 * Reka UI reserves the empty string for clearing a select and throws on a
 * `<SelectItem value="">`, so "all" needs a real value of its own.
 */
const ALL_LETTERS = 'all'

const letterFilter = ref<string>(ALL_LETTERS)
const onlyMine = ref(false)

const { submissions: live, connected, start: startEvents } = useContestEvents(contestId)

const letterItems = computed(() => [
  { label: '全部题目', value: ALL_LETTERS },
  ...problems.value.map((p) => ({ label: `${p.letter} · ${p.title ?? '题目'}`, value: p.letter })),
])

/** The `letter` query parameter, empty when every letter is selected. */
const letterQuery = computed(() => (letterFilter.value === ALL_LETTERS ? '' : letterFilter.value))

const rows = computed(() => {
  const map = new Map(submissions.value.map((s) => [s.id, s]))
  // Live frames carry fresher results for rows already on screen.
  for (const [id, update] of live.value) {
    if (map.has(id)) map.set(id, update)
  }
  return [...map.values()].sort((a, b) => b.receive_seq - a.receive_seq)
})

const newestSeq = computed(() =>
  submissions.value.reduce((min, s) => Math.min(min, s.receive_seq), Number.POSITIVE_INFINITY),
)

async function loadContext() {
  try {
    const [contestView, problemList, mine] = await Promise.all([
      api.get<Contest>(`/api/contests/${contestId.value}`),
      api.get<{ problems: ContestProblem[] }>(`/api/contests/${contestId.value}/problems`),
      api.get<{ entrant: Entrant | null }>(`/api/contests/${contestId.value}/my-entrant`),
    ])
    contest.value = contestView
    problems.value = problemList.problems ?? []
    entrant.value = mine.entrant
  } catch {
    // Filters and the title degrade gracefully; the list below reports errors.
  }
}

async function loadSubmissions() {
  loading.value = true
  loadError.value = null
  exhausted.value = false
  try {
    const query: Record<string, unknown> = { limit: 50 }
    if (letterQuery.value) query.letter = letterQuery.value
    if (onlyMine.value && entrant.value) query.entrant_id = entrant.value.id
    const res = await api.get<{ submissions: Submission[] }>(
      `/api/contests/${contestId.value}/submissions`,
      query,
    )
    submissions.value = res.submissions ?? []
    exhausted.value = submissions.value.length < 50
  } catch (e) {
    loadError.value = asApiError(e).message
    submissions.value = []
  } finally {
    loading.value = false
  }
}

async function loadMore() {
  if (loadingMore.value || exhausted.value) return
  loadingMore.value = true
  try {
    const query: Record<string, unknown> = { limit: 50, before_seq: newestSeq.value }
    if (letterQuery.value) query.letter = letterQuery.value
    if (onlyMine.value && entrant.value) query.entrant_id = entrant.value.id
    const res = await api.get<{ submissions: Submission[] }>(
      `/api/contests/${contestId.value}/submissions`,
      query,
    )
    const page = res.submissions ?? []
    submissions.value = [...submissions.value, ...page]
    exhausted.value = page.length < 50
  } catch (e) {
    reportError(e, '加载更多失败')
  } finally {
    loadingMore.value = false
  }
}

onMounted(async () => {
  await loadContext()
  await loadSubmissions()
  startEvents()
})

watch([letterFilter, onlyMine], () => {
  void loadSubmissions()
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="space-y-1">
        <h1 class="text-2xl font-semibold text-highlighted">
          提交记录
          <span v-if="contest" class="text-muted">· {{ contest.title }}</span>
        </h1>
        <div class="flex items-center gap-2">
          <UBadge v-if="connected" color="success" variant="subtle" size="sm" icon="i-lucide-radio">
            实时更新
          </UBadge>
          <span class="text-xs text-muted">仅显示你有权查看的提交</span>
        </div>
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

    <div class="flex flex-wrap items-end gap-4">
      <UFormField label="题目" class="w-56">
        <USelect v-model="letterFilter" :items="letterItems" class="w-full" />
      </UFormField>
      <UFormField label="范围">
        <USwitch v-model="onlyMine" label="只看我的提交" :disabled="!entrant" />
      </UFormField>
      <UButton
        color="neutral"
        variant="outline"
        icon="i-lucide-refresh-cw"
        class="ml-auto"
        @click="loadSubmissions"
      >
        刷新
      </UButton>
    </div>

    <UAlert
      v-if="loadError"
      color="error"
      variant="subtle"
      icon="i-lucide-triangle-alert"
      title="无法加载提交记录"
      :description="loadError"
    />

    <USkeleton v-else-if="loading" class="h-64 w-full" />

    <UEmpty
      v-else-if="!rows.length"
      icon="i-lucide-inbox"
      title="暂无提交"
      description="还没有符合当前筛选条件的提交。"
      variant="subtle"
    />

    <template v-else>
      <ul class="divide-y divide-default rounded-lg border border-default">
        <li
          v-for="submission in rows"
          :key="submission.id"
          class="flex flex-wrap items-center gap-3 px-4 py-3"
        >
          <span class="font-mono text-sm text-muted">#{{ submission.receive_seq }}</span>
          <UBadge
            v-if="submission.verdict"
            :color="verdictColor(submission.verdict)"
            variant="subtle"
            size="sm"
          >
            {{ verdictShort(submission.verdict) }}
          </UBadge>
          <UBadge v-else-if="submission.compile_failed" color="neutral" variant="subtle" size="sm">
            CE
          </UBadge>
          <UBadge v-else color="neutral" variant="subtle" size="sm">评测中</UBadge>

          <span class="text-sm font-medium">
            {{ submission.entrant_display_name || submission.entrant_id.slice(0, 8) }}
          </span>
          <span class="text-sm text-muted">{{ languageLabel(submission.language_id) }}</span>
          <span v-if="submission.score !== undefined" class="font-mono text-sm">
            {{ submission.score }}
          </span>
          <span class="text-xs text-muted" :title="formatDateTime(submission.received_at)">
            {{ formatRelative(submission.received_at) }}
          </span>
          <UButton
            :to="`/submissions/${submission.id}`"
            size="xs"
            color="neutral"
            variant="ghost"
            icon="i-lucide-arrow-right"
            trailing
            class="ml-auto"
          >
            详情
          </UButton>
        </li>
      </ul>

      <div class="flex justify-center">
        <UButton
          v-if="!exhausted"
          color="neutral"
          variant="outline"
          :loading="loadingMore"
          @click="loadMore"
        >
          加载更多
        </UButton>
        <p v-else class="text-sm text-muted">已到达列表末尾</p>
      </div>
    </template>
  </div>
</template>
