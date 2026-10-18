<script setup lang="ts">
import type { FormError, FormSubmitEvent } from '#ui/types'

definePageMeta({ layout: 'auth' })

const api = useApi()
const { fail, ok } = useApiToast()
const { refresh, isLoggedIn } = useSession()
const route = useRoute()

const state = reactive({ token: '' })
const loading = ref(false)
const done = ref(false)

// The backend emails a bare token (no clickable link), so a token may also
// arrive as a query parameter from a custom mail template.
const queryToken = route.query.token
if (typeof queryToken === 'string') state.token = queryToken

function validate(input: typeof state): FormError[] {
  const errors: FormError[] = []
  if (!input.token.trim()) errors.push({ name: 'token', message: '请输入邮件中的验证令牌' })
  return errors
}

async function onSubmit(event: FormSubmitEvent<typeof state>) {
  loading.value = true
  try {
    await api.post<{ ok: boolean }>('/api/auth/verify-email', { token: event.data.token.trim() })
    done.value = true
    ok('邮箱验证成功')
    if (isLoggedIn.value) await refresh(true)
  } catch (e) {
    fail(e, '邮箱验证失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <UCard>
    <template #header>
      <div class="space-y-1">
        <h1 class="text-xl font-semibold text-highlighted">验证邮箱</h1>
        <p class="text-sm text-muted">
          把验证邮件中的令牌粘贴到下方。开发环境下邮件不会真正发出，令牌会打印在后端日志中。
        </p>
      </div>
    </template>

    <UAlert
      v-if="done"
      color="success"
      variant="subtle"
      icon="i-lucide-mail-check"
      title="邮箱已验证"
      description="现在可以报名比赛并提交代码了。"
    />

    <UForm v-else :state="state" :validate="validate" class="space-y-4" @submit="onSubmit">
      <UFormField label="验证令牌" name="token" required>
        <UInput
          v-model="state.token"
          placeholder="粘贴邮件中的令牌"
          class="w-full font-mono text-sm"
          autofocus
        />
      </UFormField>

      <UButton type="submit" block :loading="loading" color="primary">验证邮箱</UButton>
    </UForm>

    <template #footer>
      <div class="flex items-center justify-between text-sm">
        <NuxtLink v-if="isLoggedIn" to="/" class="text-primary hover:underline">返回首页</NuxtLink>
        <NuxtLink v-else to="/login" class="text-primary hover:underline">去登录</NuxtLink>
        <NuxtLink to="/register" class="text-muted hover:underline">重新注册</NuxtLink>
      </div>
    </template>
  </UCard>
</template>
