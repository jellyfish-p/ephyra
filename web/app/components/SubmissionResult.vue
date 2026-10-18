<script setup lang="ts">
import type { Submission } from '~/types/api'

const props = defineProps<{ submission: Submission }>()

/** The backend omits fields the viewer may not see, so render by presence. */
const hidden = computed(() => props.submission.feedback === 'none')
const compileFailed = computed(() => props.submission.compile_failed === true)
const judging = computed(
  () =>
    !hidden.value &&
    props.submission.verdict === undefined &&
    props.submission.compile_failed === undefined,
)
const subtasks = computed(() => props.submission.subtasks ?? [])
const cases = computed(() => props.submission.cases ?? [])
</script>

<template>
  <div class="space-y-4">
    <UAlert
      v-if="hidden"
      color="neutral"
      variant="subtle"
      icon="i-lucide-eye-off"
      title="结果不可见"
      description="按本场赛制与当前阶段，你无权查看这份提交的评测结果。"
    />

    <template v-else>
      <div class="flex flex-wrap items-center gap-2">
        <UBadge
          v-if="submission.not_effective"
          color="warning"
          variant="subtle"
          icon="i-lucide-history"
        >
          非计分版本
        </UBadge>

        <UBadge v-if="judging" color="neutral" variant="subtle" icon="i-lucide-loader-circle">
          评测中
        </UBadge>

        <UBadge v-else-if="compileFailed" color="neutral" variant="subtle" icon="i-lucide-file-x">
          编译失败
        </UBadge>

        <UBadge
          v-else-if="submission.verdict"
          :color="verdictColor(submission.verdict)"
          variant="subtle"
        >
          {{ verdictLabel(submission.verdict) }}
        </UBadge>

        <span v-if="submission.score !== undefined" class="font-mono text-lg font-semibold">
          {{ submission.score }}
        </span>
      </div>

      <div v-if="subtasks.length" class="space-y-2">
        <h3 class="text-sm font-medium text-highlighted">子任务</h3>
        <div class="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
          <div
            v-for="subtask in subtasks"
            :key="subtask.id"
            class="flex items-center justify-between rounded-md border border-default px-3 py-2"
          >
            <div class="flex items-center gap-2">
              <span class="text-sm text-muted">子任务 {{ subtask.id }}</span>
              <UBadge :color="verdictColor(subtask.verdict)" variant="subtle" size="sm">
                {{ verdictShort(subtask.verdict) }}
              </UBadge>
            </div>
            <span class="font-mono text-sm">{{ subtask.score }}</span>
          </div>
        </div>
      </div>

      <div v-if="cases.length" class="space-y-2">
        <h3 class="text-sm font-medium text-highlighted">
          测试点 <span class="text-muted">({{ cases.length }})</span>
        </h3>
        <div class="flex flex-wrap gap-1.5">
          <UBadge
            v-for="testCase in cases"
            :key="testCase.case_id"
            :color="verdictColor(testCase.verdict)"
            variant="subtle"
            size="sm"
            class="font-mono"
          >
            {{ testCase.case_id }} · {{ verdictShort(testCase.verdict) }}
          </UBadge>
        </div>
      </div>
    </template>
  </div>
</template>
