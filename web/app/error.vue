<script setup lang="ts">
const props = defineProps<{
  error: { statusCode: number; statusMessage?: string; message?: string }
}>()

const isNotFound = computed(() => props.error.statusCode === 404)
const title = computed(() => (isNotFound.value ? '页面不存在' : '出错了'))
const description = computed(() => {
  if (isNotFound.value) return '你访问的页面可能已被移除，或链接有误。'
  return props.error.statusMessage || props.error.message || '发生了未预期的错误。'
})
</script>

<template>
  <div class="flex min-h-screen items-center justify-center px-4">
    <div class="w-full max-w-md space-y-6 text-center">
      <p class="font-mono text-5xl font-semibold text-muted">{{ error.statusCode }}</p>
      <div class="space-y-2">
        <h1 class="text-xl font-semibold text-highlighted">{{ title }}</h1>
        <p class="text-sm text-muted">{{ description }}</p>
      </div>
      <div class="flex justify-center gap-2">
        <UButton to="/" color="primary" icon="i-lucide-house">返回首页</UButton>
        <UButton
          color="neutral"
          variant="outline"
          icon="i-lucide-rotate-ccw"
          @click="$router.back()"
        >
          返回上一页
        </UButton>
      </div>
    </div>
  </div>
</template>
