<script setup lang="ts">
import type { Contest, Submission } from '~/types/api'

definePageMeta({ middleware: 'auth' })

const route = useRoute()
const api = useApi()

const submissionId = computed(() => String(route.params.id))
const contestId = ref('')

const submission = ref<Submission | null>(null)
const contest = ref<Contest | null>(null)
const source = ref<string | null>(null)
const sourceError = ref<string | null>(null)
const loading = ref(true)
const loadingSource = ref(false)
const loadError = ref<string | null>(null)

const events = useContestEvents(contestId)
const connected = computed(() => events.connected.value)

const isPending = computed(
  () =>
    submission.value !== null &&
    submission.value.verdict === undefined &&
    submission.value.compile_failed === undefined,
)

async function load() {
  loading.value = true
  loadError.value = null
  try {
    const view = await api.get<Submission>(`/api/submissions/${submissionId.value}`)
    submission.value = view
    if (view.contest_id) {
      void api
        .get<Contest>(`/api/contests/${view.contest_id}`)
        .then((c) => {
          contest.value = c
        })
        .catch(() => {
          contest.value = null
        })
      // The contest stream carries the same projection, filtered per viewer.
      contestId.value = view.contest_id
      events.start()
    }
  } catch (e) {
    loadError.value = asApiError(e).message
  } finally {
    loading.value = false
  }
}

// A live frame for this submission supersedes the fetched snapshot.
watch(
  () => events.submissions.value.get(submissionId.value),
  (update) => {
    if (update) submission.value = update
  },
)

async function loadSource() {
  loadingSource.value = true
  sourceError.value = null
  try {
    source.value = await api.getText(`/api/submissions/${submissionId.value}/source`)
  } catch (e) {
    source.value = null
    sourceError.value =
      asApiError(e).status === 403
        ? '按本场比赛的源码公开设置，你无权查看这份源码。'
        : asApiError(e).message
  } finally {
    loadingSource.value = false
  }
}

onMounted(load)
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-start justify-between gap-3">
      <div class="space-y-1">
        <h1 class="text-2xl font-semibold text-highlighted">提交详情</h1>
        <div class="flex flex-wrap items-center gap-2">
          <span class="font-mono text-xs text-muted">{{ submissionId }}</span>
          <UBadge v-if="connected" color="success" variant="subtle" size="sm" icon="i-lucide-radio">
            实时更新
          </UBadge>
        </div>
      </div>
      <div class="flex gap-2">
        <UButton
          v-if="contest"
          :to="`/contests/${contest.id}`"
          color="neutral"
          variant="ghost"
          icon="i-lucide-arrow-left"
        >
          {{ contest.title }}
        </UButton>
        <UButton
          color="neutral"
          variant="outline"
          icon="i-lucide-refresh-cw"
          :loading="loading"
          @click="load"
        >
          刷新
        </UButton>
      </div>
    </div>

    <UAlert
      v-if="loadError"
      color="error"
      variant="subtle"
      icon="i-lucide-triangle-alert"
      title="无法加载提交"
      :description="loadError"
    />

    <USkeleton v-else-if="loading" class="h-48 w-full" />

    <template v-else-if="submission">
      <UCard>
        <dl class="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <div>
            <dt class="text-sm text-muted">参赛主体</dt>
            <dd class="font-medium">
              {{ submission.entrant_display_name || submission.entrant_id.slice(0, 8) }}
            </dd>
          </div>
          <div>
            <dt class="text-sm text-muted">语言</dt>
            <dd>{{ languageLabel(submission.language_id) }}</dd>
          </div>
          <div>
            <dt class="text-sm text-muted">提交序号</dt>
            <dd class="font-mono">{{ submission.receive_seq }}</dd>
          </div>
          <div>
            <dt class="text-sm text-muted">提交时间</dt>
            <dd :title="formatDateTime(submission.received_at)">
              {{ formatRelative(submission.received_at) }}
            </dd>
          </div>
        </dl>
      </UCard>

      <UCard>
        <template #header>
          <div class="flex items-center justify-between gap-3">
            <span class="font-medium">评测结果</span>
            <UBadge
              v-if="isPending"
              color="neutral"
              variant="subtle"
              size="sm"
              icon="i-lucide-loader-circle"
            >
              评测中
            </UBadge>
          </div>
        </template>
        <SubmissionResult :submission="submission" />
      </UCard>

      <UCard>
        <template #header>
          <div class="flex items-center justify-between gap-3">
            <span class="font-medium">源码</span>
            <UButton
              v-if="source === null"
              size="xs"
              color="neutral"
              variant="outline"
              icon="i-lucide-code"
              :loading="loadingSource"
              @click="loadSource"
            >
              查看源码
            </UButton>
          </div>
        </template>

        <UAlert
          v-if="sourceError"
          color="neutral"
          variant="subtle"
          icon="i-lucide-lock"
          title="源码不可见"
          :description="sourceError"
        />
        <pre
          v-else-if="source !== null"
          class="max-h-96 overflow-auto rounded-md bg-elevated p-4 text-xs leading-relaxed"
        ><code>{{ source }}</code></pre>
        <p v-else class="text-sm text-muted">
          源码默认保密；比赛结束后按本场比赛的公开设置决定是否可查看。
        </p>
      </UCard>
    </template>
  </div>
</template>
