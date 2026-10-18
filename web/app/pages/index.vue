<script setup lang="ts">
import type { MyOrganization } from '~/types/api'

const api = useApi()
const { me, isLoggedIn, isVerified, isSiteAdmin, refresh } = useSession()

const organizations = ref<MyOrganization[]>([])
const loading = ref(false)
const loadError = ref<string | null>(null)
const createOpen = ref(false)

async function loadOrganizations() {
  if (!isLoggedIn.value) return
  loading.value = true
  loadError.value = null
  try {
    const res = await api.get<{ organizations: MyOrganization[] }>('/api/me/organizations')
    organizations.value = res.organizations ?? []
  } catch (e) {
    loadError.value = asApiError(e).message
  } finally {
    loading.value = false
  }
}

onMounted(async () => {
  await refresh().catch(() => {})
  await loadOrganizations()
})

watch(isLoggedIn, () => {
  void loadOrganizations()
})
</script>

<template>
  <div class="space-y-8">
    <template v-if="!isLoggedIn">
      <section class="space-y-4 py-12 text-center">
        <UIcon name="i-lucide-swords" class="mx-auto size-12 text-primary" />
        <h1 class="text-3xl font-semibold text-highlighted">Ephyra 在线评测系统</h1>
        <p class="mx-auto max-w-2xl text-muted">
          面向正式竞赛的开源评测平台。组织拥有独立题库、成员范围与管理权限， 比赛支持 ACM、OI、IOI
          三种赛制与个人／队伍两种报名类型。
        </p>
        <div class="flex justify-center gap-3 pt-2">
          <UButton to="/login" color="primary" size="lg">登录</UButton>
          <UButton to="/register" color="neutral" variant="outline" size="lg">注册账号</UButton>
        </div>
      </section>

      <section class="grid gap-4 sm:grid-cols-3">
        <UCard>
          <template #header>
            <div class="flex items-center gap-2">
              <UIcon name="i-lucide-building-2" class="size-5 text-primary" />
              <span class="font-medium">多组织隔离</span>
            </div>
          </template>
          <p class="text-sm text-muted">
            单实例承载多个组织，各自拥有题库、成员与管理员；一个组织的权限不延伸到其他组织。
          </p>
        </UCard>
        <UCard>
          <template #header>
            <div class="flex items-center gap-2">
              <UIcon name="i-lucide-trophy" class="size-5 text-primary" />
              <span class="font-medium">三种赛制</span>
            </div>
          </template>
          <p class="text-sm text-muted">
            ACM 按通过题数与罚时排名；OI 取同题最后一次提交；IOI 取同题最高分提交。
          </p>
        </UCard>
        <UCard>
          <template #header>
            <div class="flex items-center gap-2">
              <UIcon name="i-lucide-server" class="size-5 text-primary" />
              <span class="font-medium">可运营的评测资源</span>
            </div>
          </template>
          <p class="text-sm text-muted">
            支持系统评测资源池与组织独享池，按优先级路由，满载排队而不静默降级。
          </p>
        </UCard>
      </section>
    </template>

    <template v-else>
      <div class="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 class="text-2xl font-semibold text-highlighted">你好，{{ me?.display_name }}</h1>
          <p class="text-sm text-muted">{{ me?.email }}</p>
        </div>
        <UButton v-if="isSiteAdmin" icon="i-lucide-plus" color="primary" @click="createOpen = true">
          创建组织
        </UButton>
      </div>

      <UAlert
        v-if="!isVerified"
        color="warning"
        variant="subtle"
        icon="i-lucide-mail-warning"
        title="邮箱尚未验证"
        description="验证邮箱后才能报名比赛和提交代码。"
      >
        <template #description>
          <div class="space-y-2">
            <p>验证邮箱后才能报名比赛和提交代码。</p>
            <UButton to="/verify-email" size="xs" color="warning" variant="soft">
              去验证邮箱
            </UButton>
          </div>
        </template>
      </UAlert>

      <section class="space-y-4">
        <div class="flex items-center justify-between">
          <h2 class="text-lg font-semibold text-highlighted">我的组织</h2>
          <UButton
            to="/organizations"
            color="neutral"
            variant="ghost"
            size="sm"
            trailing-icon="i-lucide-arrow-right"
          >
            浏览全部组织
          </UButton>
        </div>

        <UAlert
          v-if="loadError"
          color="error"
          variant="subtle"
          icon="i-lucide-triangle-alert"
          title="无法加载我的组织"
          :description="loadError"
        />

        <div v-else-if="loading" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <USkeleton v-for="n in 3" :key="n" class="h-28 w-full" />
        </div>

        <UEmpty
          v-else-if="!organizations.length"
          icon="i-lucide-building-2"
          title="尚未加入任何组织"
          description="加入组织后即可参加该组织的比赛。"
          variant="subtle"
        >
          <template #actions>
            <UButton to="/organizations" color="primary">浏览组织</UButton>
          </template>
        </UEmpty>

        <div v-else class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          <UCard
            v-for="org in organizations"
            :key="org.id"
            class="transition hover:ring-primary/50"
          >
            <NuxtLink :to="`/organizations/${org.id}`" class="block space-y-2">
              <div class="flex items-start justify-between gap-2">
                <span class="font-medium text-highlighted">{{ org.name }}</span>
                <UBadge :color="org.member ? 'primary' : 'warning'" variant="subtle" size="sm">
                  {{ roleLabel(org.role) }}
                </UBadge>
              </div>
              <p class="font-mono text-xs text-muted">{{ org.slug }}</p>
            </NuxtLink>
          </UCard>
        </div>
      </section>
    </template>

    <CreateOrganizationModal v-model:open="createOpen" @created="loadOrganizations" />
  </div>
</template>
