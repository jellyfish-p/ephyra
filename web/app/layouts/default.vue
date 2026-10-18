<script setup lang="ts">
const { me, isLoggedIn, isVerified, isSiteAdmin, logout, refresh } = useSession()
const { fail, ok } = useApiToast()
const router = useRouter()

onMounted(() => {
  refresh().catch(() => {})
})

const navItems = [
  { label: '首页', to: '/', icon: 'i-lucide-house' },
  { label: '组织', to: '/organizations', icon: 'i-lucide-building-2' },
]

const colorMode = useColorMode()
const colorModeIcon = computed(() => {
  switch (colorMode.preference) {
    case 'light':
      return 'i-lucide-sun'
    case 'dark':
      return 'i-lucide-moon'
    default:
      return 'i-lucide-monitor'
  }
})

function cycleColorMode() {
  const order = ['light', 'dark', 'system'] as const
  const index = order.indexOf(colorMode.preference as (typeof order)[number])
  colorMode.preference = order[(index + 1) % order.length]!
}

async function onLogout() {
  try {
    await logout()
    ok('已退出登录')
    await router.push('/')
  } catch (e) {
    fail(e, '退出登录失败')
  }
}

const userMenuItems = computed(() => {
  const account = me.value
  if (!account) return []
  return [
    [
      {
        type: 'label' as const,
        label: account.display_name,
        avatar: { icon: 'i-lucide-user' },
      },
    ],
    [
      {
        label: account.email_verified ? '邮箱已验证' : '邮箱未验证',
        icon: account.email_verified ? 'i-lucide-mail-check' : 'i-lucide-mail-warning',
        to: account.email_verified ? undefined : '/verify-email',
        color: account.email_verified ? undefined : ('warning' as const),
      },
    ],
    ...(account.site_admin
      ? [
          [
            {
              label: '站点管理员',
              icon: 'i-lucide-shield-check',
            },
          ],
        ]
      : []),
    [
      {
        label: '退出登录',
        icon: 'i-lucide-log-out',
        onSelect: () => {
          onLogout()
        },
      },
    ],
  ]
})
</script>

<template>
  <div class="flex min-h-screen flex-col bg-default">
    <header class="sticky top-0 z-50 border-b border-default bg-default/80 backdrop-blur">
      <UContainer>
        <div class="flex h-16 items-center gap-4">
          <NuxtLink to="/" class="flex shrink-0 items-center gap-2 font-semibold text-highlighted">
            <UIcon name="i-lucide-swords" class="size-6 text-primary" />
            <span class="text-lg tracking-tight">Ephyra</span>
          </NuxtLink>

          <UNavigationMenu
            :items="navItems"
            variant="link"
            class="hidden sm:flex"
            :ui="{ root: 'gap-1' }"
          />

          <div class="ml-auto flex items-center gap-2">
            <UButton
              :icon="colorModeIcon"
              color="neutral"
              variant="ghost"
              square
              aria-label="切换主题"
              @click="cycleColorMode"
            />

            <template v-if="isLoggedIn">
              <UBadge
                v-if="!isVerified"
                color="warning"
                variant="subtle"
                icon="i-lucide-mail-warning"
                class="hidden md:inline-flex"
              >
                邮箱未验证
              </UBadge>
              <UDropdownMenu :items="userMenuItems" :content="{ align: 'end' }">
                <UButton color="neutral" variant="ghost" class="gap-2">
                  <UAvatar :alt="me?.display_name" size="2xs" />
                  <span class="hidden sm:inline">{{ me?.display_name }}</span>
                  <UIcon name="i-lucide-chevron-down" class="size-4" />
                </UButton>
              </UDropdownMenu>
            </template>

            <template v-else>
              <UButton to="/login" color="neutral" variant="ghost">登录</UButton>
              <UButton to="/register" color="primary">注册</UButton>
            </template>
          </div>
        </div>
      </UContainer>
    </header>

    <MockBanner />

    <main class="flex-1">
      <UContainer class="py-8">
        <slot />
      </UContainer>
    </main>

    <footer class="border-t border-default">
      <UContainer>
        <div class="flex flex-col gap-2 py-6 text-sm text-muted sm:flex-row sm:items-center">
          <p>Ephyra — 开源在线评测系统</p>
          <p class="sm:ml-auto">前端 Nuxt 4 + Nuxt UI · 后端 Go + PostgreSQL</p>
        </div>
      </UContainer>
    </footer>
  </div>
</template>
