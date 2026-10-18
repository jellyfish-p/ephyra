<script setup lang="ts">
import type { FormError, FormSubmitEvent } from '#ui/types'

definePageMeta({ layout: 'auth', middleware: 'guest' })

const { register } = useSession()
const { fail, ok } = useApiToast()
const router = useRouter()

const state = reactive({ displayName: '', email: '', password: '', confirm: '' })
const loading = ref(false)

function validate(input: typeof state): FormError[] {
  const errors: FormError[] = []
  if (!input.displayName.trim()) errors.push({ name: 'displayName', message: '请输入显示名称' })
  if (!input.email.trim()) errors.push({ name: 'email', message: '请输入邮箱' })
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
    await register(event.data.email.trim(), event.data.password, event.data.displayName.trim())
    ok('注册成功', '请查收验证邮件并完成邮箱验证')
    await router.push({ path: '/verify-email', query: { email: event.data.email.trim() } })
  } catch (e) {
    fail(e, '注册失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <UCard>
    <template #header>
      <div class="space-y-1">
        <h1 class="text-xl font-semibold text-highlighted">注册账号</h1>
        <p class="text-sm text-muted">站点账号全站统一。验证邮箱后才能报名比赛和提交代码。</p>
      </div>
    </template>

    <UForm :state="state" :validate="validate" class="space-y-4" @submit="onSubmit">
      <UFormField label="显示名称" name="displayName" required>
        <UInput
          v-model="state.displayName"
          placeholder="在榜单和提交中展示的名称"
          class="w-full"
          autofocus
        />
      </UFormField>

      <UFormField label="邮箱" name="email" required>
        <UInput
          v-model="state.email"
          type="email"
          autocomplete="email"
          placeholder="you@example.com"
          class="w-full"
        />
      </UFormField>

      <UFormField label="密码" name="password" required help="至少 8 个字符">
        <UInput
          v-model="state.password"
          type="password"
          autocomplete="new-password"
          class="w-full"
        />
      </UFormField>

      <UFormField label="确认密码" name="confirm" required>
        <UInput
          v-model="state.confirm"
          type="password"
          autocomplete="new-password"
          class="w-full"
        />
      </UFormField>

      <UButton type="submit" block :loading="loading" color="primary">注册</UButton>
    </UForm>

    <template #footer>
      <p class="text-sm text-muted">
        已有账号？
        <NuxtLink to="/login" class="text-primary hover:underline">直接登录</NuxtLink>
      </p>
    </template>
  </UCard>
</template>
