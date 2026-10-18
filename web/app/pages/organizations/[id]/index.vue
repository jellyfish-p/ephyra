<script setup lang="ts">
import { h } from 'vue'
import type { TableColumn } from '#ui/types'
import type { BankProblem, Contest, Membership, Organization } from '~/types/api'

definePageMeta({ middleware: 'auth' })

const route = useRoute()
const api = useApi()
const { fail, ok } = useApiToast()

const orgId = computed(() => String(route.params.id))

const org = ref<Organization | null>(null)
const membership = ref<Membership | null>(null)
const contests = ref<Contest[]>([])
const problems = ref<BankProblem[]>([])
const loading = ref(true)
const loadError = ref<string | null>(null)
const joining = ref(false)
const tab = ref('contests')

const createContestOpen = ref(false)
const importProblemOpen = ref(false)

const canManage = computed(
  () =>
    membership.value?.member === true &&
    (membership.value.role === 'owner' || membership.value.role === 'admin'),
)

async function load() {
  loading.value = true
  loadError.value = null
  try {
    org.value = await api.get<Organization>(`/api/organizations/${orgId.value}`)
    membership.value = await api.get<Membership>(`/api/organizations/${orgId.value}/membership`)
    await Promise.all([loadContests(), loadProblems()])
  } catch (e) {
    loadError.value = asApiError(e).message
  } finally {
    loading.value = false
  }
}

async function loadContests() {
  try {
    const res = await api.get<{ contests: Contest[] }>(`/api/organizations/${orgId.value}/contests`)
    contests.value = res.contests ?? []
  } catch {
    contests.value = []
  }
}

async function loadProblems() {
  try {
    const res = await api.get<{ problems: BankProblem[] }>(
      `/api/organizations/${orgId.value}/problems`,
    )
    problems.value = res.problems ?? []
  } catch {
    problems.value = []
  }
}

onMounted(load)

async function join() {
  joining.value = true
  try {
    await api.post<{ ok: boolean }>(`/api/organizations/${orgId.value}/join`)
    ok('申请已提交', '等待组织管理员审核')
    membership.value = await api.get<Membership>(`/api/organizations/${orgId.value}/membership`)
  } catch (e) {
    fail(e, '申请加入失败')
  } finally {
    joining.value = false
  }
}

const problemColumns: TableColumn<BankProblem>[] = [
  {
    accessorKey: 'slug',
    header: '标识',
    cell: ({ row }) => h('span', { class: 'font-mono text-sm' }, row.original.slug),
  },
  { accessorKey: 'title', header: '标题' },
]
</script>

<template>
  <div class="space-y-6">
    <UAlert
      v-if="loadError"
      color="error"
      variant="subtle"
      icon="i-lucide-triangle-alert"
      title="无法加载组织"
      :description="loadError"
    />

    <template v-else>
      <USkeleton v-if="loading" class="h-24 w-full" />

      <div v-else-if="org" class="space-y-4">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="space-y-1">
            <div class="flex flex-wrap items-center gap-2">
              <h1 class="text-2xl font-semibold text-highlighted">{{ org.name }}</h1>
              <UBadge v-if="membership?.member" color="primary" variant="subtle">
                {{ roleLabel(membership.role) }}
              </UBadge>
              <UBadge v-else color="neutral" variant="subtle">非成员</UBadge>
            </div>
            <p class="font-mono text-sm text-muted">{{ org.slug }}</p>
          </div>

          <div class="flex flex-wrap gap-2">
            <UButton
              v-if="!membership?.member"
              color="primary"
              icon="i-lucide-user-plus"
              :loading="joining"
              @click="join"
            >
              申请加入
            </UButton>
            <UButton
              v-if="canManage"
              :to="`/organizations/${orgId}/members`"
              color="neutral"
              variant="outline"
              icon="i-lucide-users"
            >
              成员与申请
            </UButton>
            <UButton
              v-if="canManage"
              :to="`/organizations/${orgId}/judge`"
              color="neutral"
              variant="outline"
              icon="i-lucide-server"
            >
              评测资源
            </UButton>
          </div>
        </div>

        <UTabs
          v-model="tab"
          :items="[
            { label: '比赛', icon: 'i-lucide-trophy', slot: 'contests', value: 'contests' },
            { label: '题库', icon: 'i-lucide-library', slot: 'problems', value: 'problems' },
          ]"
        >
          <template #contests>
            <div class="space-y-4 pt-4">
              <div v-if="canManage" class="flex justify-end">
                <UButton
                  icon="i-lucide-plus"
                  color="primary"
                  size="sm"
                  @click="createContestOpen = true"
                >
                  创建比赛
                </UButton>
              </div>

              <UEmpty
                v-if="!contests.length"
                icon="i-lucide-trophy"
                title="暂无比赛"
                :description="canManage ? '创建第一场比赛。' : '该组织还没有比赛。'"
                variant="subtle"
              />

              <div v-else class="grid gap-4 md:grid-cols-2">
                <UCard v-for="contest in contests" :key="contest.id">
                  <NuxtLink :to="`/contests/${contest.id}`" class="block space-y-3">
                    <div class="flex items-start justify-between gap-2">
                      <span class="font-medium text-highlighted">{{ contest.title }}</span>
                    </div>
                    <ContestBadges :contest="contest" />
                    <dl class="space-y-1 text-sm text-muted">
                      <div class="flex justify-between gap-4">
                        <dt>报名开始</dt>
                        <dd>{{ formatDateTime(contest.register_starts_at) }}</dd>
                      </div>
                      <div class="flex justify-between gap-4">
                        <dt>比赛时间</dt>
                        <dd>
                          {{ formatDateTime(contest.starts_at) }} —
                          {{ formatDateTime(contest.ends_at) }}
                        </dd>
                      </div>
                    </dl>
                  </NuxtLink>
                </UCard>
              </div>
            </div>
          </template>

          <template #problems>
            <div class="space-y-4 pt-4">
              <div v-if="canManage" class="flex justify-end">
                <UButton
                  icon="i-lucide-upload"
                  color="primary"
                  size="sm"
                  @click="importProblemOpen = true"
                >
                  导入题包
                </UButton>
              </div>

              <UTable :data="problems" :columns="problemColumns" empty="题库为空" />
            </div>
          </template>
        </UTabs>
      </div>

      <CreateContestModal
        v-model:open="createContestOpen"
        :organization-id="orgId"
        @created="loadContests"
      />
      <ImportProblemModal
        v-model:open="importProblemOpen"
        :organization-id="orgId"
        @imported="loadProblems"
      />
    </template>
  </div>
</template>
