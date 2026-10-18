<script setup lang="ts">
const props = defineProps<{ markdown: string | null }>()

const hasStatement = computed(() => Boolean(props.markdown && props.markdown.trim()))
</script>

<template>
  <div v-if="hasStatement" class="statement">
    <MDC :value="markdown!" />
  </div>
  <UAlert
    v-else
    color="neutral"
    variant="subtle"
    icon="i-lucide-file-question"
    title="暂无题面"
    description="该题目的评测数据包中没有包含 Markdown 题面文件（如 statement.md）。"
  />
</template>
