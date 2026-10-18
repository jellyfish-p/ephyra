<script setup lang="ts">
import type { FormError, FormSubmitEvent } from '#ui/types'

const props = defineProps<{ organizationId: string }>()
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ imported: [] }>()

const api = useApi()
const { fail, ok } = useApiToast()

const state = reactive({
  slug: '',
  title: '',
  file: null as File | null,
})
const loading = ref(false)

function validate(input: typeof state): FormError[] {
  const errors: FormError[] = []
  if (!input.slug.trim()) errors.push({ name: 'slug', message: '请输入题目标识' })
  if (!input.title.trim()) errors.push({ name: 'title', message: '请输入题目标题' })
  if (!input.file) errors.push({ name: 'file', message: '请选择 Hydro 题包 zip' })
  return errors
}

async function onSubmit(event: FormSubmitEvent<typeof state>) {
  const input = event.data
  if (!input.file) return
  loading.value = true
  try {
    const form = new FormData()
    form.append('zip', input.file)
    form.append('slug', input.slug.trim().toLowerCase())
    form.append('title', input.title.trim())
    const res = await api.postForm<{ problem_id: string; revision_id: string; revision_n: number }>(
      `/api/organizations/${props.organizationId}/problems/import`,
      form,
    )
    ok('题包已导入', `第 ${res.revision_n} 版评测数据`)
    emit('imported')
    open.value = false
    state.slug = ''
    state.title = ''
    state.file = null
  } catch (e) {
    fail(e, '导入题包失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <UModal
    v-model:open="open"
    title="导入题包"
    description="支持基础 Hydro 题包结构的明确子集；不支持的配置会明确报错，不会静默改变语义。"
  >
    <template #body>
      <UForm
        id="import-problem-form"
        :state="state"
        :validate="validate"
        class="space-y-4"
        @submit="onSubmit"
      >
        <UFormField label="题目标识" name="slug" required>
          <UInput v-model="state.slug" placeholder="two-sum" class="w-full" />
        </UFormField>
        <UFormField label="题目标题" name="title" required>
          <UInput v-model="state.title" placeholder="两数之和" class="w-full" />
        </UFormField>
        <UFormField label="题包 zip" name="file" required>
          <UFileUpload
            v-model="state.file"
            accept=".zip,application/zip"
            label="拖拽或点击选择 zip 题包"
            class="w-full"
          />
        </UFormField>
      </UForm>
    </template>
    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <UButton color="neutral" variant="ghost" @click="open = false">取消</UButton>
        <UButton type="submit" form="import-problem-form" :loading="loading" color="primary">
          导入
        </UButton>
      </div>
    </template>
  </UModal>
</template>
