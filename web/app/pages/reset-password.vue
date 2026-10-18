<script setup lang="ts">
import type { FormError, FormSubmitEvent } from '#ui/types'

definePageMeta({ layout: 'auth' })

const api = useApi()
const { fail, ok } = useApiToast()
const route = useRoute()
const router = useRouter()

const state = reactive({ token: '', password: '', confirm: '' })
const loading = ref(false)

const queryToken = route.query.token
if (typeof queryToken === 'string') state.token = queryToken

function validate(input: typeof state): FormError[] {
  const errors: FormError[] = []
  if (!input.token.trim()) errors.push({ name: 'token', message: '请输入邮件中的重置令牌' })
  if (input.password.length < 8) {
    errors.push({ name: 'password', message: '密码至少需要 8 个字符' })
  }
  if (input.confirm !== input.password) {
    errors.push({ name: 'confirm', message: '两次输入的密码不一致' })
  }
  return errors
}

async function onSubmit(event: FormSubmitEvent<typeof state>) {
  loading.value = true
  try {
    await api.post<{ ok: boolean }>('/api/auth/reset-password', {
      token: event.data.token.trim(),
      password: event.data.password,
    })
    ok('密码已重置', '请使用新密码登录')
    await router.push('/login')
  } catch (e) {
    fail(e, '重置失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <UCard>
    <template #header>
      <div class="space-y-1">
        <h1 class="text-xl font-semibold text-highlighted">设置新密码</h1>
        <p class="text-sm text-muted">输入重置令牌和新密码。令牌只能使用一次。</p>
      </div>
    </template>

    <UForm :state="state" :validate="validate" class="space-y-4" @submit="onSubmit">
      <UFormField label="重置令牌" name="token" required>
        <UInput v-model="state.token" class="w-full font-mono text-sm" autofocus />
      </UFormField>

      <UFormField label="新密码" name="password" required help="至少 8 个字符">
        <UInput
          v-model="state.password"
          type="password"
          autocomplete="new-password"
          class="w-full"
        />
      </UFormField>

      <UFormField label="确认新密码" name="confirm" required>
        <UInput
          v-model="state.confirm"
          type="password"
          autocomplete="new-password"
          class="w-full"
        />
      </UFormField>

      <UButton type="submit" block :loading="loading" color="primary">重置密码</UButton>
    </UForm>

    <template #footer>
      <NuxtLink to="/login" class="text-sm text-primary hover:underline">返回登录</NuxtLink>
    </template>
  </UCard>
</template>
