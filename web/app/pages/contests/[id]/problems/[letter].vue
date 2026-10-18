<script setup lang="ts">
import type { FormError, FormSubmitEvent } from '#ui/types'
import type {
  Contest,
  ContestProblemDetail,
  Entrant,
  LanguageID,
  Submission,
  SubmitAccepted,
} from '~/types/api'

definePageMeta({ middleware: 'auth' })

const route = useRoute()
const api = useApi()
const { fail, ok } = useApiToast()

const contestId = computed(() => String(route.params.id))
const letter = computed(() => String(route.params.letter).toUpperCase())

const detail = ref<ContestProblemDetail | null>(null)
const contest = ref<Contest | null>(null)
const entrant = ref<Entrant | null>(null)
const mySubmissions = ref<Submission[]>([])
const loading = ref(true)
const loadError = ref<string | null>(null)
const submissionsError = ref<string | null>(null)

const { submissions: liveSubmissions, connected, start: startEvents } = useContestEvents(contestId)

const state = reactive({
  language: 'cpp23' as LanguageID,
  mode: 'paste' as 'paste' | 'upload',
  code: '',
  file: null as File | null,
})
const submitting = ref(false)
const lastAccepted = ref<SubmitAccepted | null>(null)

const languageItems = [
  { label: 'C++23', value: 'cpp23' },
  { label: 'C17', value: 'c17' },
  { label: 'C23', value: 'c23' },
]
const modeItems = [
  { label: '粘贴代码', value: 'paste' },
  { label: '上传文件', value: 'upload' },
]

const sourceFileName = computed(() => (state.language === 'cpp23' ? 'main.cpp' : 'main.c'))

async function load() {
  loading.value = true
  loadError.value = null
  try {
    const [problem, contestView] = await Promise.all([
      api.get<ContestProblemDetail>(`/api/contests/${contestId.value}/problems/${letter.value}`),
      api.get<Contest>(`/api/contests/${contestId.value}`),
    ])
    detail.value = problem
    contest.value = contestView
    await Promise.all([loadEntrant(), loadMySubmissions()])
    startEvents()
  } catch (e) {
    loadError.value = asApiError(e).message
  } finally {
    loading.value = false
  }
}

async function loadEntrant() {
  try {
    const res = await api.get<{ entrant: Entrant | null }>(
      `/api/contests/${contestId.value}/my-entrant`,
    )
    entrant.value = res.entrant
  } catch {
    entrant.value = null
  }
}

async function loadMySubmissions() {
  submissionsError.value = null
  try {
    const res = await api.get<{ submissions: Submission[] }>(
      `/api/contests/${contestId.value}/submissions`,
      {
        letter: letter.value,
        limit: 20,
      },
    )
    mySubmissions.value = res.submissions ?? []
  } catch (e) {
    submissionsError.value = asApiError(e).message
  }
}

onMounted(load)

/** Live frames are merged in, so a submission shows up without a refetch. */
const mergedSubmissions = computed(() => {
  const map = new Map(mySubmissions.value.map((s) => [s.id, s]))
  for (const [id, live] of liveSubmissions.value) {
    if (map.has(id)) map.set(id, live)
  }
  return [...map.values()].sort((a, b) => b.receive_seq - a.receive_seq)
})

function validate(input: typeof state): FormError[] {
  const errors: FormError[] = []
  if (input.mode === 'paste' && !input.code.trim()) {
    errors.push({ name: 'code', message: '请输入代码' })
  }
  if (input.mode === 'upload' && !input.file) {
    errors.push({ name: 'file', message: '请选择源文件' })
  }
  return errors
}

async function onSubmit(event: FormSubmitEvent<typeof state>) {
  const input = event.data
  const file =
    input.mode === 'upload'
      ? input.file
      : new File([input.code], sourceFileName.value, { type: 'text/plain' })
  if (!file) return

  submitting.value = true
  try {
    const form = new FormData()
    form.append('language', input.language)
    form.append('source', file)
    const res = await api.postForm<SubmitAccepted>(
      `/api/contests/${contestId.value}/problems/${letter.value}/submissions`,
      form,
    )
    lastAccepted.value = res
    ok('提交成功', `序号 #${res.receive_seq}`)
    await loadMySubmissions()
  } catch (e) {
    fail(e, '提交失败')
  } finally {
    submitting.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <UAlert
      v-if="loadError"
      color="error"
      variant="subtle"
      icon="i-lucide-triangle-alert"
      title="无法加载题目"
      :description="loadError"
    />

    <USkeleton v-else-if="loading" class="h-64 w-full" />

    <template v-else-if="detail">
      <div class="flex flex-wrap items-center gap-3">
        <UButton
          :to="`/contests/${contestId}`"
          color="neutral"
          variant="ghost"
          icon="i-lucide-arrow-left"
          size="sm"
        >
          返回比赛
        </UButton>
        <span
          class="flex size-8 items-center justify-center rounded-md bg-elevated font-mono font-semibold"
        >
          {{ detail.letter }}
        </span>
        <h1 class="text-xl font-semibold text-highlighted">{{ detail.title || '题目' }}</h1>
        <ContestBadges v-if="contest" :contest="contest" class="ml-auto" />
      </div>

      <div class="grid gap-6 lg:grid-cols-3">
        <div class="space-y-4 lg:col-span-2">
          <UCard>
            <template #header>
              <div class="flex flex-wrap items-center gap-2 text-sm">
                <UBadge color="neutral" variant="subtle">
                  {{ PROBLEM_KIND_LABELS[detail.kind] }}
                </UBadge>
                <UBadge color="neutral" variant="subtle">
                  {{ CHECKER_LABELS[detail.checker_kind] }}
                </UBadge>
                <span class="text-muted">时限 {{ formatDuration(detail.time_limit_ms) }}</span>
                <span class="text-muted">内存 {{ formatBytes(detail.memory_limit_bytes) }}</span>
              </div>
            </template>
            <StatementView :markdown="detail.statement_markdown" />
          </UCard>

          <UCard v-if="detail.subtasks.length">
            <template #header>
              <span class="font-medium">子任务</span>
            </template>
            <div class="flex flex-wrap gap-2">
              <UBadge
                v-for="subtask in detail.subtasks"
                :key="subtask.id"
                color="neutral"
                variant="subtle"
                class="font-mono"
              >
                子任务 {{ subtask.id }} · {{ subtask.score }}
              </UBadge>
            </div>
            <p class="mt-3 text-xs text-muted">
              子任务的全部测试点通过才获得该组全部分数，否则得 0。
            </p>
          </UCard>
        </div>

        <div class="space-y-4">
          <UCard class="lg:sticky lg:top-20">
            <template #header>
              <span class="font-medium">提交代码</span>
            </template>

            <UAlert
              v-if="!entrant || entrant.status !== 'accepted'"
              color="warning"
              variant="subtle"
              icon="i-lucide-circle-alert"
              title="无法提交"
              :description="
                entrant
                  ? `报名状态为「${ENTRANT_STATUS_LABELS[entrant.status]}」，通过后才能提交。`
                  : '你还没有报名这场比赛。'
              "
              class="mb-4"
            />

            <UForm :state="state" :validate="validate" class="space-y-4" @submit="onSubmit">
              <UFormField label="语言" name="language">
                <USelect v-model="state.language" :items="languageItems" class="w-full" />
              </UFormField>

              <UFormField label="提交方式" name="mode">
                <USelect v-model="state.mode" :items="modeItems" class="w-full" />
              </UFormField>

              <UFormField v-if="state.mode === 'paste'" label="代码" name="code">
                <UTextarea
                  v-model="state.code"
                  :rows="12"
                  placeholder="在此粘贴源代码"
                  class="w-full font-mono text-sm"
                />
              </UFormField>

              <UFormField v-else label="源文件" name="file">
                <UFileUpload
                  v-model="state.file"
                  :label="`选择 ${sourceFileName}`"
                  class="w-full"
                />
              </UFormField>

              <UButton
                type="submit"
                block
                color="primary"
                :loading="submitting"
                :disabled="!entrant || entrant.status !== 'accepted'"
              >
                提交评测
              </UButton>
            </UForm>

            <UAlert
              v-if="lastAccepted"
              color="success"
              variant="subtle"
              icon="i-lucide-check"
              class="mt-4"
            >
              <template #description>
                <div class="space-y-2">
                  <p>提交成功，序号 #{{ lastAccepted.receive_seq }}</p>
                  <UButton
                    :to="`/submissions/${lastAccepted.submission_id}`"
                    size="xs"
                    color="success"
                    variant="soft"
                  >
                    查看评测详情
                  </UButton>
                </div>
              </template>
            </UAlert>
          </UCard>
        </div>
      </div>

      <UCard>
        <template #header>
          <div class="flex flex-wrap items-center gap-2">
            <span class="font-medium">提交记录</span>
            <UBadge
              v-if="connected"
              color="success"
              variant="subtle"
              size="sm"
              icon="i-lucide-radio"
            >
              实时
            </UBadge>
            <UButton
              size="xs"
              color="neutral"
              variant="ghost"
              icon="i-lucide-refresh-cw"
              class="ml-auto"
              @click="loadMySubmissions"
            >
              刷新
            </UButton>
          </div>
        </template>

        <UAlert
          v-if="submissionsError"
          color="error"
          variant="subtle"
          icon="i-lucide-triangle-alert"
          title="无法加载提交记录"
          :description="submissionsError"
        />

        <UEmpty
          v-else-if="!mergedSubmissions.length"
          icon="i-lucide-inbox"
          title="暂无提交"
          variant="subtle"
        />

        <ul v-else class="divide-y divide-default">
          <li
            v-for="submission in mergedSubmissions"
            :key="submission.id"
            class="flex flex-wrap items-center gap-3 py-3 first:pt-0 last:pb-0"
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
            <UBadge v-else color="neutral" variant="subtle" size="sm">评测中</UBadge>
            <span v-if="submission.entrant_display_name" class="text-sm">
              {{ submission.entrant_display_name }}
            </span>
            <span class="text-sm text-muted">{{ languageLabel(submission.language_id) }}</span>
            <span v-if="submission.score !== undefined" class="font-mono text-sm">
              {{ submission.score }}
            </span>
            <span class="text-xs text-muted">{{ formatRelative(submission.received_at) }}</span>
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
      </UCard>
    </template>
  </div>
</template>
