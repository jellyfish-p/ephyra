<script setup lang="ts">
import { h } from 'vue'
import type { TableColumn } from '#ui/types'
import type { JoinRequest, Membership, OrganizationMember } from '~/types/api'

definePageMeta({ middleware: 'auth' })

const route = useRoute()
const api = useApi()
const { fail, ok } = useApiToast()

const orgId = computed(() => String(route.params.id))

const members = ref<OrganizationMember[]>([])
const requests = ref<JoinRequest[]>([])
const membership = ref<Membership | null>(null)
const loading = ref(true)
const membersError = ref<string | null>(null)
const requestsError = ref<string | null>(null)
const deciding = ref<string | null>(null)

const canManage = computed(
  () =>
    membership.value?.member === true &&
    (membership.value.role === 'owner' || membership.value.role === 'admin'),
)

async function load() {
  loading.value = true
  membersError.value = null
  requestsError.value = null
  try {
    membership.value = await api.get<Membership>(`/api/organizations/${orgId.value}/membership`)
  } catch {
    membership.value = null
  }

  try {
    const res = await api.get<{ members: OrganizationMember[] }>(
      `/api/organizations/${orgId.value}/members`,
    )
    members.value = res.members ?? []
  } catch (e) {
    membersError.value = asApiError(e).message
  }

  try {
    const res = await api.get<{ join_requests: JoinRequest[] }>(
      `/api/organizations/${orgId.value}/join-requests`,
    )
    requests.value = res.join_requests ?? []
  } catch (e) {
    requestsError.value = asApiError(e).message
  }

  loading.value = false
}

onMounted(load)

async function decide(request: JoinRequest, approve: boolean) {
  deciding.value = request.id
  try {
    await api.post<{ ok: boolean }>(
      `/api/organizations/${orgId.value}/join-requests/${request.id}/decision`,
      { approve },
    )
    ok(approve ? '已通过申请' : '已拒绝申请', request.display_name)
    await load()
  } catch (e) {
    fail(e, '处理申请失败')
  } finally {
    deciding.value = null
  }
}

const memberColumns: TableColumn<OrganizationMember>[] = [
  {
    accessorKey: 'display_name',
    header: '成员',
    cell: ({ row }) => h('span', { class: 'font-medium' }, row.original.display_name),
  },
  {
    accessorKey: 'role',
    header: '角色',
    cell: ({ row }) => h('span', { class: 'text-sm' }, roleLabel(row.original.role)),
  },
  {
    accessorKey: 'account_id',
    header: '账号 ID',
    cell: ({ row }) =>
      h('span', { class: 'font-mono text-xs text-muted' }, row.original.account_id),
  },
]
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-semibold text-highlighted">成员与申请</h1>
        <p class="text-sm text-muted">组织成员身份只在本组织内有效。</p>
      </div>
      <UButton
        :to="`/organizations/${orgId}`"
        color="neutral"
        variant="ghost"
        icon="i-lucide-arrow-left"
      >
        返回组织
      </UButton>
    </div>

    <USkeleton v-if="loading" class="h-40 w-full" />

    <template v-else>
      <UCard>
        <template #header>
          <div class="flex items-center gap-2">
            <UIcon name="i-lucide-users" class="size-5 text-primary" />
            <span class="font-medium">成员（{{ members.length }}）</span>
          </div>
        </template>

        <UAlert
          v-if="membersError"
          color="error"
          variant="subtle"
          icon="i-lucide-lock"
          title="无法查看成员名单"
          :description="membersError"
        />
        <UTable v-else :data="members" :columns="memberColumns" empty="暂无成员" />
      </UCard>

      <UCard v-if="canManage">
        <template #header>
          <div class="flex items-center gap-2">
            <UIcon name="i-lucide-user-check" class="size-5 text-primary" />
            <span class="font-medium">待处理申请（{{ requests.length }}）</span>
          </div>
        </template>

        <UAlert
          v-if="requestsError"
          color="error"
          variant="subtle"
          icon="i-lucide-triangle-alert"
          title="无法加载加入申请"
          :description="requestsError"
        />

        <UEmpty
          v-else-if="!requests.length"
          icon="i-lucide-inbox"
          title="没有待处理申请"
          variant="subtle"
        />

        <ul v-else class="divide-y divide-default">
          <li
            v-for="request in requests"
            :key="request.id"
            class="flex flex-wrap items-center justify-between gap-3 py-3 first:pt-0 last:pb-0"
          >
            <div>
              <p class="font-medium text-highlighted">{{ request.display_name }}</p>
              <p class="text-xs text-muted">
                申请于 {{ formatDateTime(request.created_at) }} ·
                {{ formatRelative(request.created_at) }}
              </p>
            </div>
            <div class="flex gap-2">
              <UButton
                color="primary"
                size="sm"
                icon="i-lucide-check"
                :loading="deciding === request.id"
                @click="decide(request, true)"
              >
                通过
              </UButton>
              <UButton
                color="error"
                variant="outline"
                size="sm"
                icon="i-lucide-x"
                :loading="deciding === request.id"
                @click="decide(request, false)"
              >
                拒绝
              </UButton>
            </div>
          </li>
        </ul>
      </UCard>
    </template>
  </div>
</template>
