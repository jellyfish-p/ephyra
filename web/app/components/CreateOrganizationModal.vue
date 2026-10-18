<script setup lang="ts">
import type { FormError, FormSubmitEvent } from '#ui/types'
import type { Organization } from '~/types/api'

const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ created: [Organization] }>()

const api = useApi()
const { fail, ok } = useApiToast()

const state = reactive({ slug: '', name: '' })
const loading = ref(false)

function validate(input: typeof state): FormError[] {
  const errors: FormError[] = []
  if (!input.slug.trim()) errors.push({ name: 'slug', message: '请输入组织标识' })
  if (!input.name.trim()) errors.push({ name: 'name', message: '请输入组织名称' })
  return errors
}

async function onSubmit(event: FormSubmitEvent<typeof state>) {
  loading.value = true
  try {
    const org = await api.post<Organization>('/api/organizations', {
      slug: event.data.slug.trim().toLowerCase(),
      name: event.data.name.trim(),
    })
    ok('组织已创建', org.name)
    emit('created', org)
    open.value = false
    state.slug = ''
    state.name = ''
  } catch (e) {
    fail(e, '创建组织失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <UModal v-model:open="open" title="创建组织" description="只有站点管理员可以创建组织。">
    <template #body>
      <UForm
        id="create-org-form"
        :state="state"
        :validate="validate"
        class="space-y-4"
        @submit="onSubmit"
      >
        <UFormField label="组织标识" name="slug" required help="用于 URL，例如 ephyra-team">
          <UInput v-model="state.slug" placeholder="ephyra-team" class="w-full" autofocus />
        </UFormField>
        <UFormField label="组织名称" name="name" required>
          <UInput v-model="state.name" placeholder="Ephyra Team" class="w-full" />
        </UFormField>
      </UForm>
    </template>
    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <UButton color="neutral" variant="ghost" @click="open = false">取消</UButton>
        <UButton type="submit" form="create-org-form" :loading="loading" color="primary"
          >创建</UButton
        >
      </div>
    </template>
  </UModal>
</template>
