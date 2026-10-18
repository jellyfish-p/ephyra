<script setup lang="ts">
import { h } from 'vue'
import type { TableColumn } from '#ui/types'
import type { ContestRule, StandingRow } from '~/types/api'

const props = withDefaults(
  defineProps<{
    standings: StandingRow[]
    /** Contest problem letters in display order. */
    letters: string[]
    rule: ContestRule
    loading?: boolean
  }>(),
  { loading: false },
)

function renderProblem(row: StandingRow, letter: string) {
  const score = problemScoresByLetter(row).get(letter)
  if (!score) return h('span', { class: 'text-dimmed' }, '·')

  const failed = score.failed_attempts_before_ac

  if (score.pending) {
    return h('span', { class: 'text-warning' }, '…')
  }

  if (props.rule === 'acm') {
    if (score.solved) {
      return h('span', { class: 'inline-flex items-baseline gap-1 font-mono text-success' }, [
        String(score.penalty_minutes),
        failed > 0 ? h('span', { class: 'text-xs text-error' }, `(-${failed})`) : null,
      ])
    }
    if (failed > 0) {
      return h('span', { class: 'font-mono text-error' }, `-${failed}`)
    }
    return h('span', { class: 'text-dimmed' }, '·')
  }

  const value = Number.parseFloat(score.score)
  const cls =
    value <= 0
      ? 'font-mono text-dimmed'
      : score.solved
        ? 'font-mono text-success'
        : 'font-mono text-warning'
  return h('span', { class: cls }, score.score)
}

const columns = computed<TableColumn<StandingRow>[]>(() => {
  const base: TableColumn<StandingRow>[] = [
    { accessorKey: 'rank', header: '#', meta: { class: { th: 'w-14', td: 'w-14' } } },
    {
      accessorKey: 'display_name',
      header: '参赛主体',
      cell: ({ row }) => h('span', { class: 'font-medium' }, row.original.display_name),
    },
  ]

  if (props.rule === 'acm') {
    base.push({
      id: 'solved',
      header: '通过',
      cell: ({ row }) =>
        h('span', { class: 'font-mono font-semibold' }, String(row.original.solved)),
    })
    base.push({
      id: 'penalty',
      header: '罚时',
      cell: ({ row }) => h('span', { class: 'font-mono' }, `${row.original.penalty_minutes}`),
    })
  } else {
    base.push({
      id: 'total_score',
      header: '总分',
      cell: ({ row }) => h('span', { class: 'font-mono font-semibold' }, row.original.total_score),
    })
  }

  for (const letter of props.letters) {
    base.push({
      id: `problem-${letter}`,
      header: letter,
      cell: ({ row }) => renderProblem(row.original, letter),
    })
  }

  return base
})
</script>

<template>
  <div class="space-y-3">
    <UAlert
      v-if="standings.some((s) => s.unsettled)"
      color="warning"
      variant="subtle"
      icon="i-lucide-hourglass"
      title="榜单尚未结算"
      description="仍有提交未完成评测或存在重测批次，当前名次为临时结果。"
    />

    <UTable
      :data="standings"
      :columns="columns"
      :loading="loading"
      sticky
      class="w-full"
      empty="暂无参赛者或榜单尚未公开"
    />
  </div>
</template>
