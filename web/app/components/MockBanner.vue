<script setup lang="ts">
/**
 * Mock-mode control strip.
 *
 * Renders only when the API is being served by `app/mock`. Two things the real
 * backend does are invisible in a browser-only mock — it sends email, and it
 * judges asynchronously — so this surfaces the tokens it "sent" and offers to
 * land a pending verdict immediately.
 */
interface MockAccountInfo {
  id: string
  email: string
  display_name: string
  email_verified: boolean
  site_admin: boolean
  password: string
}

interface MockContestInfo {
  id: string
  slug: string
  title: string
  rule: string
}

interface MockStateInfo {
  now: string
  accounts: MockAccountInfo[]
  contests: MockContestInfo[]
  pending_tasks: number
  pending_submissions: string[]
}

interface MockTokenInfo {
  token: string
  kind: 'verify' | 'reset'
  email: string
  expires_at: string
}

const enabled = useMockEnabled()
const api = useApi()
const { fail, ok } = useApiToast()
const { me, refresh } = useSession()

const open = ref(false)
const info = ref<MockStateInfo | null>(null)
const tokens = ref<MockTokenInfo[]>([])
const busy = ref(false)

async function load() {
  if (!enabled) return
  try {
    const [state, tokenList] = await Promise.all([
      api.get<MockStateInfo>('/api/_mock/state'),
      api.get<{ tokens: MockTokenInfo[] }>('/api/_mock/tokens'),
    ])
    info.value = state
    tokens.value = tokenList.tokens ?? []
  } catch (e) {
    fail(e, '无法读取 mock 状态')
  }
}

onMounted(load)

async function signInAs(account: MockAccountInfo) {
  busy.value = true
  try {
    await api.post('/api/auth/login', { email: account.email, password: account.password })
    await refresh()
    ok('已切换账号', account.email)
    await load()
  } catch (e) {
    fail(e, '切换账号失败')
  } finally {
    busy.value = false
  }
}

async function finishJudging() {
  const pending = info.value?.pending_submissions ?? []
  if (!pending.length) return
  busy.value = true
  try {
    for (const id of pending) {
      await api.post(`/api/_mock/judge/${id}/complete`)
    }
    ok('已立即完成评测', `${pending.length} 份提交`)
    await load()
  } catch (e) {
    fail(e, '完成评测失败')
  } finally {
    busy.value = false
  }
}

async function reset() {
  busy.value = true
  try {
    await api.post('/api/_mock/reset')
  } catch {
    // The state is gone either way; a failure here is not worth reporting.
  } finally {
    busy.value = false
    window.location.reload()
  }
}

async function copyToken(token: string) {
  try {
    await navigator.clipboard.writeText(token)
    ok('令牌已复制', '粘贴到验证页即可')
  } catch (e) {
    fail(e, '复制失败')
  }
}
</script>

<template>
  <div v-if="enabled" class="border-b border-default bg-elevated/60">
    <UContainer>
      <div class="flex flex-wrap items-center gap-2 py-2 text-sm">
        <UBadge color="info" variant="subtle" icon="i-lucide-flask-conical" size="sm">
          Mock 模式
        </UBadge>
        <span class="text-muted"> 接口由浏览器内的 mock 提供，未连接后端 </span>
        <UBadge v-if="info?.pending_tasks" color="warning" variant="subtle" size="sm">
          {{ info.pending_tasks }} 个评测任务未完成
        </UBadge>

        <div class="ml-auto flex items-center gap-2">
          <UButton
            v-if="info?.pending_submissions.length"
            size="xs"
            color="neutral"
            variant="outline"
            icon="i-lucide-fast-forward"
            :loading="busy"
            @click="finishJudging"
          >
            立即完成评测
          </UButton>
          <UButton
            size="xs"
            color="neutral"
            variant="ghost"
            icon="i-lucide-chevron-down"
            @click="open = !open"
          >
            控制台
          </UButton>
        </div>
      </div>

      <div v-if="open" class="space-y-4 border-t border-default py-4">
        <div class="grid gap-4 lg:grid-cols-3">
          <div class="space-y-2">
            <h3 class="text-sm font-medium text-highlighted">切换账号</h3>
            <p class="text-xs text-muted">
              所有种子账号密码均为 <code class="font-mono">password123</code>。
              <span v-if="me">当前：{{ me.email }}</span>
            </p>
            <div class="flex flex-wrap gap-2">
              <UButton
                v-for="account in info?.accounts ?? []"
                :key="account.id"
                size="xs"
                :color="me?.email === account.email ? 'primary' : 'neutral'"
                :variant="me?.email === account.email ? 'solid' : 'outline'"
                :disabled="busy"
                @click="signInAs(account)"
              >
                {{ account.display_name }}
                <UBadge
                  v-if="!account.email_verified"
                  color="warning"
                  variant="subtle"
                  size="sm"
                  class="ml-1"
                >
                  未验证
                </UBadge>
                <UBadge
                  v-if="account.site_admin"
                  color="info"
                  variant="subtle"
                  size="sm"
                  class="ml-1"
                >
                  站点管理员
                </UBadge>
              </UButton>
            </div>
          </div>

          <div class="space-y-2">
            <h3 class="text-sm font-medium text-highlighted">邮箱令牌</h3>
            <p class="text-xs text-muted">
              后端只发送裸令牌，不发送链接，因此验证与重置页面需要手动粘贴。
            </p>
            <div v-if="!tokens.length" class="text-xs text-muted">暂无待用令牌</div>
            <ul v-else class="space-y-1">
              <li
                v-for="token in tokens"
                :key="token.token"
                class="flex flex-wrap items-center gap-2 rounded-md border border-default px-2 py-1.5"
              >
                <UBadge color="neutral" variant="subtle" size="sm">
                  {{ token.kind === 'verify' ? '验证邮箱' : '重置密码' }}
                </UBadge>
                <span class="text-xs text-muted">{{ token.email }}</span>
                <code class="font-mono text-xs">{{ token.token }}</code>
                <UButton
                  size="xs"
                  color="neutral"
                  variant="ghost"
                  icon="i-lucide-copy"
                  class="ml-auto"
                  @click="copyToken(token.token)"
                >
                  复制
                </UButton>
              </li>
            </ul>
          </div>
          <div class="space-y-2">
            <h3 class="text-sm font-medium text-highlighted">示例比赛</h3>
            <p class="text-xs text-muted">种子数据覆盖未开始、进行中、封榜中与已发布四种阶段。</p>
            <div class="flex flex-wrap gap-2">
              <UButton
                v-for="contest in info?.contests ?? []"
                :key="contest.id"
                size="xs"
                color="neutral"
                variant="outline"
                :to="`/contests/${contest.id}`"
                @click="open = false"
              >
                {{ contest.title }}
              </UButton>
            </div>
          </div>
        </div>

        <div class="flex flex-wrap items-center gap-2 border-t border-default pt-3">
          <p class="text-xs text-muted">
            在源码中写 <code class="font-mono">// @verdict=tle</code>
            （或 ac/wa/mle/rte/ce/sys）可强制评测结果。
          </p>
          <div class="ml-auto flex gap-2">
            <UButton
              size="xs"
              color="neutral"
              variant="outline"
              icon="i-lucide-refresh-cw"
              :loading="busy"
              @click="load"
            >
              刷新状态
            </UButton>
            <UButton
              size="xs"
              color="error"
              variant="outline"
              icon="i-lucide-rotate-ccw"
              :disabled="busy"
              @click="reset"
            >
              重置数据
            </UButton>
            <UButton
              size="xs"
              color="neutral"
              variant="ghost"
              icon="i-lucide-plug"
              @click="setMockEnabled(false)"
            >
              退出 Mock
            </UButton>
          </div>
        </div>
      </div>
    </UContainer>
  </div>
</template>
