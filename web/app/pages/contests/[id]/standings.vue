<script setup lang="ts">
import type { Contest, ContestProblem, StandingRow } from '~/types/api'

definePageMeta({ middleware: 'auth' })

const route = useRoute()
const api = useApi()

const contestId = computed(() => String(route.params.id))

const contest = ref<Contest | null>(null)
const problems = ref<ContestProblem[]>([])
const standings = ref<StandingRow[]>([])
const hidden = ref(false)
const loading = ref(true)
const loadError = ref<string | null>(null)
const autoRefresh = ref(true)
const lastLoadedAt = ref<Date | null>(null)

let timer: ReturnType<typeof setInterval> | null = null

const letters = computed(() => problems.value.map((p) => p.letter))

async function load(showSpinner = true) {
  if (showSpinner) loading.value = true
  loadError.value = null
  try {
    const [contestView, problemList] = await Promise.all([
      api.get<Contest>(`/api/contests/${contestId.value}`),
      api.get<{ problems: ContestProblem[] }>(`/api/contests/${contestId.value}/problems`),
    ])
    contest.value = contestView
    problems.value = problemList.problems ?? []
    await loadStandings()
    lastLoadedAt.value = new Date()
  } catch (e) {
    loadError.value = asApiError(e).message
  } finally {
    loading.value = false
  }
}

async function loadStandings() {
  try {
    const res = await api.get<{ standings: StandingRow[] }>(
      `/api/contests/${contestId.value}/standings`,
    )
    standings.value = res.standings ?? []
    hidden.value = false
  } catch (e) {
    standings.value = []
    hidden.value = asApiError(e).status === 403
  }
}

onMounted(() => {
  void load()
  timer = setInterval(() => {
    if (autoRefresh.value) void loadStandings()
  }, 15_000)
})

onScopeDispose(() => {
  if (timer) clearInterval(timer)
})
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div class="space-y-1">
        <h1 class="text-2xl font-semibold text-highlighted">
          榜单
          <span v-if="contest" class="text-muted">· {{ contest.title }}</span>
        </h1>
        <p v-if="lastLoadedAt" class="text-xs text-muted">
          最后更新 {{ lastLoadedAt.toLocaleTimeString('zh-CN') }}
        </p>
      </div>
      <div class="flex items-center gap-3">
        <USwitch v-model="autoRefresh" label="自动刷新" />
        <UButton
          :to="`/contests/${contestId}`"
          color="neutral"
          variant="ghost"
          icon="i-lucide-arrow-left"
        >
          返回比赛
        </UButton>
      </div>
    </div>

    <UAlert
      v-if="loadError"
      color="error"
      variant="subtle"
      icon="i-lucide-triangle-alert"
      title="无法加载榜单"
      :description="loadError"
    />

    <USkeleton v-else-if="loading" class="h-64 w-full" />

    <template v-else>
      <UAlert
        v-if="contest"
        color="neutral"
        variant="subtle"
        icon="i-lucide-info"
        :title="`${RULE_LABELS[contest.rule]} 排名规则`"
        :description="RULE_DESCRIPTIONS[contest.rule]"
      />

      <UAlert
        v-if="hidden"
        color="neutral"
        variant="subtle"
        icon="i-lucide-eye-off"
        title="榜单当前不可见"
        description="按本场赛制与阶段，公开榜尚未开放。"
      />

      <StandingsTable
        v-else-if="contest"
        :standings="standings"
        :letters="letters"
        :rule="contest.rule"
        :loading="loading"
      />
    </template>
  </div>
</template>
