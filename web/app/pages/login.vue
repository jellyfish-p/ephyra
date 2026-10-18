<script setup lang="ts">
import type { FormError, FormSubmitEvent } from '#ui/types'

definePageMeta({ layout: 'auth', middleware: 'guest' })

const { login } = useSession()
const { fail, ok } = useApiToast()
const route = useRoute()
const router = useRouter()

const state = reactive({ email: '', password: '' })
const loading = ref(false)

function validate(input: typeof state): FormError[] {
  const errors: FormError[] = []
  if (!input.email.trim()) errors.push({ name: 'email', message: '请输入邮箱' })
  if (!input.password) errors.push({ name: 'password', message: '请输入密码' })
  return errors
}

async function onSubmit(event: FormSubmitEvent<typeof state>) {
  loading.value = true
  try {
    const account = await login(event.data.email.trim(), event.data.password)
    ok('登录成功', `欢迎回来，${account.display_name}`)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/'
    await router.push(redirect)
  } catch (e) {
    fail(e, '登录失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <UCard>
    <template #header>
      <div class="space-y-1">
        <h1 class="text-xl font-semibold text-highlighted">登录 Ephyra</h1>
        <p class="text-sm text-muted">使用站点账号登录，组织与比赛权限另行管理。</p>
      </div>
    </template>

    <UForm :state="state" :validate="validate" class="space-y-4" @submit="onSubmit">
      <UFormField label="邮箱" name="email" required>
        <UInput
          v-model="state.email"
          type="email"
          autocomplete="email"
          placeholder="you@example.com"
          class="w-full"
          autofocus
        />
      </UFormField>

      <UFormField label="密码" name="password" required>
        <UInput
          v-model="state.password"
          type="password"
          autocomplete="current-password"
          placeholder="••••••••"
          class="w-full"
        />
      </UFormField>

      <UButton type="submit" block :loading="loading" color="primary">登录</UButton>
    </UForm>

    <template #footer>
      <div class="flex flex-col gap-2 text-sm">
        <div class="flex items-center justify-between">
          <NuxtLink to="/forgot-password" class="text-primary hover:underline">
            忘记密码？
          </NuxtLink>
          <NuxtLink to="/register" class="text-primary hover:underline"> 注册新账号 </NuxtLink>
        </div>
      </div>
    </template>
  </UCard>
</template>
