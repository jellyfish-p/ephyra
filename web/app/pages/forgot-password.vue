<script setup lang="ts">
import type { FormError, FormSubmitEvent } from '#ui/types'

definePageMeta({ layout: 'auth' })

const api = useApi()
const { fail, ok } = useApiToast()

const state = reactive({ email: '' })
const loading = ref(false)
const sent = ref(false)

function validate(input: typeof state): FormError[] {
  const errors: FormError[] = []
  if (!input.email.trim()) errors.push({ name: 'email', message: '请输入邮箱' })
  return errors
}

async function onSubmit(event: FormSubmitEvent<typeof state>) {
  loading.value = true
  try {
    // The endpoint always answers 200 so it cannot be used to probe accounts.
    await api.post<{ ok: boolean }>('/api/auth/forgot-password', {
      email: event.data.email.trim(),
    })
    sent.value = true
    ok('请求已提交')
  } catch (e) {
    fail(e, '提交失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <UCard>
    <template #header>
      <div class="space-y-1">
        <h1 class="text-xl font-semibold text-highlighted">重置密码</h1>
        <p class="text-sm text-muted">
          提交邮箱后，系统会发送重置令牌。出于安全考虑，无论邮箱是否存在都会返回成功。
        </p>
      </div>
    </template>

    <UAlert v-if="sent" color="info" variant="subtle" icon="i-lucide-mail" title="请求已提交">
      <template #description>
        <div class="space-y-2">
          <p>如果该邮箱存在，你会收到重置令牌。开发环境下令牌会打印在后端日志中。</p>
          <UButton to="/reset-password" size="xs" color="info" variant="soft">
            我已拿到令牌
          </UButton>
        </div>
      </template>
    </UAlert>

    <UForm v-else :state="state" :validate="validate" class="space-y-4" @submit="onSubmit">
      <UFormField label="邮箱" name="email" required>
        <UInput v-model="state.email" type="email" autocomplete="email" class="w-full" autofocus />
      </UFormField>

      <UButton type="submit" block :loading="loading" color="primary">发送重置令牌</UButton>
    </UForm>

    <template #footer>
      <NuxtLink to="/login" class="text-sm text-primary hover:underline">返回登录</NuxtLink>
    </template>
  </UCard>
</template>
