<script setup lang="ts">
import type { Contest } from '~/types/api'

const props = defineProps<{
  contest: Pick<Contest, 'rule' | 'entry_mode' | 'starts_at' | 'ends_at' | 'published_at'>
}>()

const phase = computed(() => contestPhase(props.contest))
</script>

<template>
  <div class="flex flex-wrap items-center gap-1.5">
    <UBadge color="secondary" variant="subtle">{{ RULE_LABELS[contest.rule] }}</UBadge>
    <UBadge color="neutral" variant="subtle">
      {{ ENTRY_MODE_LABELS[contest.entry_mode] }}
    </UBadge>
    <UBadge :color="PHASE_COLORS[phase]" variant="subtle">{{ PHASE_LABELS[phase] }}</UBadge>
  </div>
</template>
