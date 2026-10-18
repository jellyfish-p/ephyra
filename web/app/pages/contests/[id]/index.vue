<script setup lang="ts">
import type { FormError, FormSubmitEvent } from '#ui/types'
import type { Contest, ContestProblem, Entrant, StandingRow } from '~/types/api'

definePageMeta({ middleware: 'auth' })

const route = useRoute()
const api = useApi()
const { fail, ok } = useApiToast()
const { me } = useSession()

const contestId = computed(() => String(route.params.id))

const contest = ref<Contest | null>(null)
const problems = ref<ContestProblem[]>([])
const entrant = ref<Entrant | null>(null)
const standings = ref<StandingRow[]>([])
const standingsHidden = ref(false)
const loading = ref(true)
const loadError = ref<string | null>(null)
const tab = ref('problems')

const registering = ref(false)
const teamOpen = ref(false)
const teamState = reactive({ name: '' })
const inviteOpen = ref(false)
const inviteState = reactive({ accountId: '' })
const busy = ref(false)

const letters = computed(() => problems.value.map((p) => p.letter))
const phase = computed(() => (contest.value ? contestPhase(contest.value) : 'before'))
const registrationAvailable = computed(() => {
  if (!contest.value) return false
  return registrationOpen(contest.value) && phase.value === 'before'
})

const myMember = computed(
  () => entrant.value?.members.find((m) => m.account_id === me.value?.account_id) ?? null,
)
const isCaptain = computed(() => myMember.value?.role === 'captain')
const needsConfirm = computed(() => myMember.value !== null && !myMember.value.confirmed)

async function load() {
  loading.value = true
  loadError.value = null
  try {
    contest.value = await api.get<Contest>(`/api/contests/${contestId.value}`)
    await Promise.all([loadProblems(), loadEntrant(), loadStandings()])
  } catch (e) {
    loadError.value = asApiError(e).message
  } finally {
    loading.value = false
  }
}

async function loadProblems() {
  try {
    const res = await api.get<{ problems: ContestProblem[] }>(
      `/api/contests/${contestId.value}/problems`,
    )
    problems.value = res.problems ?? []
  } catch {
    problems.value = []
  }
}

async function loadEntrant() {
  try {
    const res = await api.get<{ entrant: Entrant | null }>(
      `/api/contests/${contestId.value}/my-entrant`,
    )
    entrant.value = res.entrant
  } catch {
    entrant.value = null
  }
}

async function loadStandings() {
  try {
    const res = await api.get<{ standings: StandingRow[] }>(
      `/api/contests/${contestId.value}/standings`,
    )
    standings.value = res.standings ?? []
    standingsHidden.value = false
  } catch (e) {
    // A non-participant may be denied the board entirely.
    standings.value = []
    standingsHidden.value = asApiError(e).status === 403
  }
}

onMounted(load)

async function registerIndividual() {
  registering.value = true
  try {
    await api.post<{ entrant_id: string; status: string }>(
      `/api/contests/${contestId.value}/register`,
    )
    ok('报名已提交')
    await loadEntrant()
  } catch (e) {
    fail(e, '报名失败')
  } finally {
    registering.value = false
  }
}

function validateTeam(input: typeof teamState): FormError[] {
  const errors: FormError[] = []
  if (!input.name.trim()) errors.push({ name: 'name', message: '请输入队伍名称' })
  return errors
}

async function createTeam(event: FormSubmitEvent<typeof teamState>) {
  registering.value = true
  try {
    await api.post<{ entrant_id: string; status: string }>(
      `/api/contests/${contestId.value}/teams`,
      { name: event.data.name.trim() },
    )
    ok('队伍已创建', '邀请队员后提交报名')
    teamOpen.value = false
    teamState.name = ''
    await loadEntrant()
  } catch (e) {
    fail(e, '创建队伍失败')
  } finally {
    registering.value = false
  }
}

async function inviteMember() {
  if (!entrant.value || !inviteState.accountId.trim()) return
  busy.value = true
  try {
    await api.post<{ ok: boolean }>(`/api/entrants/${entrant.value.id}/invite`, {
      account_id: inviteState.accountId.trim(),
    })
    ok('邀请已发出')
    inviteOpen.value = false
    inviteState.accountId = ''
    await loadEntrant()
  } catch (e) {
    fail(e, '邀请失败')
  } finally {
    busy.value = false
  }
}

async function confirmInvite() {
  if (!entrant.value) return
  busy.value = true
  try {
    await api.post<{ ok: boolean }>(`/api/entrants/${entrant.value.id}/confirm`)
    ok('已确认加入队伍')
    await loadEntrant()
  } catch (e) {
    fail(e, '确认失败')
  } finally {
    busy.value = false
  }
}

async function submitRegistration() {
  if (!entrant.value) return
  busy.value = true
  try {
    await api.post<{ ok: boolean }>(`/api/entrants/${entrant.value.id}/submit-registration`)
    ok('报名已提交', '等待比赛管理员审核')
    await loadEntrant()
  } catch (e) {
    fail(e, '提交报名失败')
  } finally {
    busy.value = false
  }
}

async function withdraw() {
  if (!entrant.value) return
  busy.value = true
  try {
    await api.post<{ ok: boolean }>(`/api/entrants/${entrant.value.id}/withdraw`)
    ok('已撤回报名')
    await loadEntrant()
  } catch (e) {
    fail(e, '撤回失败')
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="space-y-6">
    <UAlert
      v-if="loadError"
      color="error"
      variant="subtle"
      icon="i-lucide-triangle-alert"
      title="无法加载比赛"
      :description="loadError"
    />

    <USkeleton v-else-if="loading" class="h-32 w-full" />

    <template v-else-if="contest">
      <div class="space-y-4">
        <div class="flex flex-wrap items-start justify-between gap-3">
          <div class="space-y-2">
            <h1 class="text-2xl font-semibold text-highlighted">{{ contest.title }}</h1>
            <ContestBadges :contest="contest" />
            <p class="text-sm text-muted">
              {{ formatDateTime(contest.starts_at) }} — {{ formatDateTime(contest.ends_at) }} （{{
                formatRelative(contest.ends_at)
              }}）
            </p>
          </div>
          <div class="flex flex-wrap gap-2">
            <UButton
              :to="`/contests/${contestId}/submissions`"
              color="neutral"
              variant="outline"
              icon="i-lucide-list"
            >
              提交记录
            </UButton>
            <UButton
              :to="`/contests/${contestId}/admin`"
              color="neutral"
              variant="outline"
              icon="i-lucide-settings"
            >
              比赛管理
            </UButton>
          </div>
        </div>

        <UCard>
          <template #header>
            <div class="flex items-center gap-2">
              <UIcon name="i-lucide-user-check" class="size-5 text-primary" />
              <span class="font-medium">参赛状态</span>
            </div>
          </template>

          <div v-if="entrant" class="space-y-4">
            <div class="flex flex-wrap items-center gap-2">
              <UBadge :color="ENTRANT_STATUS_COLORS[entrant.status]" variant="subtle">
                {{ ENTRANT_STATUS_LABELS[entrant.status] }}
              </UBadge>
              <span class="font-medium text-highlighted">{{ entrant.display_name }}</span>
              <UBadge color="neutral" variant="subtle" size="sm">
                {{ ENTRY_MODE_LABELS[entrant.kind] }}
              </UBadge>
              <UBadge v-if="entrant.roster_frozen_at" color="info" variant="subtle" size="sm">
                名单已冻结
              </UBadge>
            </div>

            <div v-if="entrant.members.length" class="space-y-1">
              <p class="text-sm font-medium text-highlighted">队员</p>
              <ul class="space-y-1">
                <li
                  v-for="member in entrant.members"
                  :key="member.account_id"
                  class="flex items-center gap-2 text-sm"
                >
                  <UBadge
                    :color="member.confirmed ? 'success' : 'warning'"
                    variant="subtle"
                    size="sm"
                  >
                    {{ member.confirmed ? '已确认' : '待确认' }}
                  </UBadge>
                  <span>{{ member.display_name }}</span>
                  <span class="text-muted">
                    {{ member.role === 'captain' ? '队长' : '队员' }}
                  </span>
                </li>
              </ul>
            </div>

            <div class="flex flex-wrap gap-2">
              <UButton
                v-if="needsConfirm"
                color="primary"
                icon="i-lucide-check"
                :loading="busy"
                @click="confirmInvite"
              >
                确认加入队伍
              </UButton>
              <UButton
                v-if="isCaptain && entrant.kind === 'team' && entrant.status === 'pending'"
                color="neutral"
                variant="outline"
                icon="i-lucide-user-plus"
                @click="inviteOpen = true"
              >
                邀请队员
              </UButton>
              <UButton
                v-if="isCaptain && entrant.kind === 'team' && entrant.status === 'pending'"
                color="primary"
                icon="i-lucide-send"
                :loading="busy"
                @click="submitRegistration"
              >
                提交报名
              </UButton>
              <UButton
                v-if="isCaptain && !entrant.roster_frozen_at"
                color="error"
                variant="outline"
                icon="i-lucide-undo-2"
                :loading="busy"
                @click="withdraw"
              >
                撤回报名
              </UButton>
            </div>

            <p v-if="entrant.kind === 'team'" class="text-xs text-muted">
              队伍报名生效时冻结名单；开赛后已报名队伍不换人。
            </p>
          </div>

          <div v-else class="space-y-3">
            <p class="text-sm text-muted">
              <template v-if="registrationAvailable"> 你还没有报名这场比赛。 </template>
              <template v-else>
                当前不在报名时间内（报名开始于 {{ formatDateTime(contest.register_starts_at) }}）。
              </template>
            </p>
            <div class="flex flex-wrap gap-2">
              <UButton
                v-if="registrationAvailable && contest.entry_mode === 'individual'"
                color="primary"
                icon="i-lucide-user-plus"
                :loading="registering"
                @click="registerIndividual"
              >
                个人报名
              </UButton>
              <UButton
                v-if="registrationAvailable && contest.entry_mode === 'team'"
                color="primary"
                icon="i-lucide-users"
                @click="teamOpen = true"
              >
                创建队伍
              </UButton>
            </div>
          </div>
        </UCard>

        <UTabs
          v-model="tab"
          :items="[
            { label: '题目', icon: 'i-lucide-list', slot: 'problems', value: 'problems' },
            { label: '榜单', icon: 'i-lucide-trophy', slot: 'standings', value: 'standings' },
          ]"
        >
          <template #problems>
            <div class="space-y-3 pt-4">
              <UEmpty
                v-if="!problems.length"
                icon="i-lucide-list"
                title="暂无题目"
                description="比赛管理员还没有添加题目。"
                variant="subtle"
              />
              <div v-else class="divide-y divide-default rounded-lg border border-default">
                <NuxtLink
                  v-for="problem in problems"
                  :key="problem.letter"
                  :to="`/contests/${contestId}/problems/${problem.letter}`"
                  class="flex items-center gap-4 px-4 py-3 transition hover:bg-elevated/50"
                >
                  <span
                    class="flex size-8 shrink-0 items-center justify-center rounded-md bg-elevated font-mono font-semibold"
                  >
                    {{ problem.letter }}
                  </span>
                  <span class="font-medium text-highlighted">
                    {{ problem.title || '题目' }}
                  </span>
                  <UIcon name="i-lucide-arrow-right" class="ml-auto size-4 text-muted" />
                </NuxtLink>
              </div>
            </div>
          </template>

          <template #standings>
            <div class="space-y-4 pt-4">
              <div class="flex justify-end">
                <UButton
                  :to="`/contests/${contestId}/standings`"
                  size="sm"
                  color="neutral"
                  variant="outline"
                  icon="i-lucide-trophy"
                >
                  打开完整榜单
                </UButton>
              </div>
              <UAlert
                v-if="standingsHidden"
                color="neutral"
                variant="subtle"
                icon="i-lucide-eye-off"
                title="榜单当前不可见"
                description="按本场赛制与阶段，公开榜尚未开放。"
              />
              <StandingsTable
                v-else
                :standings="standings"
                :letters="letters"
                :rule="contest.rule"
              />
            </div>
          </template>
        </UTabs>
      </div>

      <UModal v-model:open="teamOpen" title="创建队伍">
        <template #body>
          <UForm
            id="create-team-form"
            :state="teamState"
            :validate="validateTeam"
            class="space-y-4"
            @submit="createTeam"
          >
            <UFormField label="队伍名称" name="name" required>
              <UInput v-model="teamState.name" placeholder="Team Ephyra" class="w-full" autofocus />
            </UFormField>
          </UForm>
        </template>
        <template #footer>
          <div class="flex w-full justify-end gap-2">
            <UButton color="neutral" variant="ghost" @click="teamOpen = false">取消</UButton>
            <UButton type="submit" form="create-team-form" :loading="registering" color="primary">
              创建队伍
            </UButton>
          </div>
        </template>
      </UModal>

      <UModal v-model:open="inviteOpen" title="邀请队员">
        <template #body>
          <div class="space-y-4">
            <UAlert
              color="info"
              variant="subtle"
              icon="i-lucide-info"
              title="需要队员的账号 ID"
              description="后端暂未提供按邮箱或名称检索账号的接口，因此需要直接填写队员的账号 UUID。"
            />
            <UFormField label="账号 ID" required>
              <UInput
                v-model="inviteState.accountId"
                placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
                class="w-full font-mono text-sm"
                autofocus
              />
            </UFormField>
          </div>
        </template>
        <template #footer>
          <div class="flex w-full justify-end gap-2">
            <UButton color="neutral" variant="ghost" @click="inviteOpen = false">取消</UButton>
            <UButton :loading="busy" color="primary" @click="inviteMember">发送邀请</UButton>
          </div>
        </template>
      </UModal>
    </template>
  </div>
</template>
