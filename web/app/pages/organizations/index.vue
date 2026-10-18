<script setup lang="ts">
import type { MyOrganization, Organization } from '~/types/api'

definePageMeta({ middleware: 'auth' })

const api = useApi()
const { isSiteAdmin } = useSession()
const { fail, ok } = useApiToast()

const organizations = ref<Organization[]>([])
const mine = ref<Map<string, MyOrganization>>(new Map())
const loading = ref(true)
const loadError = ref<string | null>(null)
const createOpen = ref(false)
const joiningId = ref<string | null>(null)
/** Orgs joined from this page, before a reload confirms membership. */
const requested = ref<Set<string>>(new Set())

async function load() {
  loading.value = true
  loadError.value = null
  try {
    const [all, my] = await Promise.all([
      api.get<{ organizations: Organization[] }>('/api/organizations'),
      api.get<{ organizations: MyOrganization[] }>('/api/me/organizations'),
    ])
    organizations.value = all.organizations ?? []
    mine.value = new Map((my.organizations ?? []).map((o) => [o.id, o]))
  } catch (e) {
    loadError.value = asApiError(e).message
  } finally {
    loading.value = false
  }
}

onMounted(load)

function membershipOf(id: string) {
  return mine.value.get(id)
}

async function join(org: Organization) {
  joiningId.value = org.id
  try {
    await api.post<{ ok: boolean }>(`/api/organizations/${org.id}/join`)
    requested.value = new Set(requested.value).add(org.id)
    ok('申请已提交', '等待组织管理员审核')
  } catch (e) {
    fail(e, '申请加入失败')
  } finally {
    joiningId.value = null
  }
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-semibold text-highlighted">组织</h1>
        <p class="text-sm text-muted">
          每个组织拥有独立题库、成员范围与管理权限，权限不延伸到其他组织。
        </p>
      </div>
      <UButton v-if="isSiteAdmin" icon="i-lucide-plus" color="primary" @click="createOpen = true">
        创建组织
      </UButton>
    </div>

    <UAlert
      v-if="loadError"
      color="error"
      variant="subtle"
      icon="i-lucide-triangle-alert"
      title="无法加载组织列表"
      :description="loadError"
    />

    <div v-else-if="loading" class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <USkeleton v-for="n in 6" :key="n" class="h-32 w-full" />
    </div>

    <UEmpty
      v-else-if="!organizations.length"
      icon="i-lucide-building-2"
      title="暂无组织"
      description="组织仅由站点管理员创建。"
      variant="subtle"
    />

    <div v-else class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
      <UCard v-for="org in organizations" :key="org.id">
        <div class="space-y-3">
          <div class="flex items-start justify-between gap-2">
            <NuxtLink
              :to="`/organizations/${org.id}`"
              class="font-medium text-highlighted hover:text-primary"
            >
              {{ org.name }}
            </NuxtLink>
            <UBadge v-if="membershipOf(org.id)" color="primary" variant="subtle" size="sm">
              {{ roleLabel(membershipOf(org.id)?.role) }}
            </UBadge>
            <UBadge v-else-if="requested.has(org.id)" color="warning" variant="subtle" size="sm">
              待审核
            </UBadge>
          </div>

          <p class="font-mono text-xs text-muted">{{ org.slug }}</p>

          <div class="flex items-center gap-2">
            <UButton
              :to="`/organizations/${org.id}`"
              color="neutral"
              variant="outline"
              size="sm"
              icon="i-lucide-arrow-right"
              trailing
            >
              查看
            </UButton>
            <UButton
              v-if="!membershipOf(org.id) && !requested.has(org.id)"
              color="primary"
              size="sm"
              :loading="joiningId === org.id"
              icon="i-lucide-user-plus"
              @click="join(org)"
            >
              申请加入
            </UButton>
          </div>
        </div>
      </UCard>
    </div>

    <CreateOrganizationModal v-model:open="createOpen" @created="load" />
  </div>
</template>
