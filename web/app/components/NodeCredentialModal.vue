<script setup lang="ts">
const open = defineModel<boolean>('open', { default: false })
const props = defineProps<{ nodeName: string; credential: string }>()

const { ok } = useApiToast()

async function copy() {
  try {
    await navigator.clipboard.writeText(props.credential)
    ok('已复制到剪贴板')
  } catch {
    ok('复制失败', '请手动选择并复制')
  }
}
</script>

<template>
  <UModal v-model:open="open" title="节点凭据">
    <template #body>
      <div class="space-y-4">
        <UAlert
          color="warning"
          variant="subtle"
          icon="i-lucide-shield-alert"
          title="凭据只显示这一次"
          description="关闭后无法再次查看。请立即把它配置到评测节点；遗失时只能轮换新凭据。"
        />

        <div class="space-y-1">
          <p class="text-sm text-muted">节点：{{ nodeName }}</p>
          <div class="flex items-center gap-2">
            <UInput :model-value="credential" readonly class="w-full font-mono text-xs" />
            <UButton icon="i-lucide-copy" color="neutral" variant="outline" @click="copy">
              复制
            </UButton>
          </div>
        </div>
      </div>
    </template>
    <template #footer>
      <div class="flex w-full justify-end">
        <UButton color="primary" @click="open = false">我已保存</UButton>
      </div>
    </template>
  </UModal>
</template>
