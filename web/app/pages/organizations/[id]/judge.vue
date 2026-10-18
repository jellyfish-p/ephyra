<script setup lang="ts">
import type { FormError, FormSubmitEvent } from '#ui/types'
import type { JudgeNode, Membership, Pool, RoutingEntry } from '~/types/api'

definePageMeta({ middleware: 'auth' })

const route = useRoute()
const api = useApi()
const { fail, ok } = useApiToast()
const { isSiteAdmin } = useSession()

const orgId = computed(() => String(route.params.id))

const pools = ref<Pool[]>([])
const nodesByPool = ref<Map<string, JudgeNode[]>>(new Map())
const membership = ref<Membership | null>(null)
const routing = ref<RoutingEntry[]>([])
const loading = ref(true)
const loadError = ref<string | null>(null)
const savingRouting = ref(false)

const credentialOpen = ref(false)
const credentialNodeName = ref('')
const credentialValue = ref('')

const poolOpen = ref(false)
const poolState = reactive({ kind: 'organization', name: '' })
const creatingPool = ref(false)

const nodeOpen = ref(false)
const nodeState = reactive({ poolId: '', name: '' })
const registeringNode = ref(false)
const busyNodeId = ref<string | null>(null)

const canManage = computed(
  () =>
    membership.value?.member === true &&
    (membership.value.role === 'owner' || membership.value.role === 'admin'),
)

const poolOptions = computed(() => pools.value.map((p) => ({ label: p.name, value: p.id })))

async function load() {
  loading.value = true
  loadError.value = null
  try {
    membership.value = await api.get<Membership>(`/api/organizations/${orgId.value}/membership`)
    const res = await api.get<{ pools: Pool[] }>(`/api/organizations/${orgId.value}/pools`)
    pools.value = res.pools ?? []
    await Promise.all(pools.value.map((pool) => loadNodes(pool.id)))
    await loadRouting()
  } catch (e) {
    loadError.value = asApiError(e).message
  } finally {
    loading.value = false
  }
}

async function loadNodes(poolId: string) {
  try {
    const res = await api.get<{ nodes: JudgeNode[] }>(`/api/pools/${poolId}/nodes`)
    nodesByPool.value = new Map(nodesByPool.value).set(poolId, res.nodes ?? [])
  } catch {
    nodesByPool.value = new Map(nodesByPool.value).set(poolId, [])
  }
}

async function loadRouting() {
  try {
    const res = await api.get<{ entries: RoutingEntry[] }>(
      `/api/organizations/${orgId.value}/routing`,
    )
    routing.value = res.entries ?? []
  } catch {
    routing.value = []
  }
}

onMounted(load)

function validatePool(input: typeof poolState): FormError[] {
  const errors: FormError[] = []
  if (!input.name.trim()) errors.push({ name: 'name', message: '请输入资源池名称' })
  return errors
}

async function createPool(event: FormSubmitEvent<typeof poolState>) {
  creatingPool.value = true
  try {
    await api.post<Pool>('/api/pools', {
      kind: event.data.kind,
      organization_id: event.data.kind === 'organization' ? orgId.value : null,
      name: event.data.name.trim(),
    })
    ok('资源池已创建')
    poolOpen.value = false
    poolState.name = ''
    poolState.kind = 'organization'
    await load()
  } catch (e) {
    fail(e, '创建资源池失败')
  } finally {
    creatingPool.value = false
  }
}

function openNodeDialog(poolId: string) {
  nodeState.poolId = poolId
  nodeState.name = ''
  nodeOpen.value = true
}

async function registerNode() {
  if (!nodeState.name.trim()) {
    fail(new Error('请输入节点名称'), '注册节点失败')
    return
  }
  registeringNode.value = true
  try {
    const res = await api.post<{ id: string; credential: string }>(
      `/api/pools/${nodeState.poolId}/nodes`,
      { name: nodeState.name.trim() },
    )
    credentialNodeName.value = nodeState.name.trim()
    credentialValue.value = res.credential
    credentialOpen.value = true
    nodeOpen.value = false
    await loadNodes(nodeState.poolId)
  } catch (e) {
    fail(e, '注册节点失败')
  } finally {
    registeringNode.value = false
  }
}

async function rotateNode(node: JudgeNode) {
  busyNodeId.value = node.id
  try {
    const res = await api.post<{ credential: string }>(`/api/nodes/${node.id}/rotate`)
    credentialNodeName.value = node.name
    credentialValue.value = res.credential
    credentialOpen.value = true
  } catch (e) {
    fail(e, '轮换凭据失败')
  } finally {
    busyNodeId.value = null
  }
}

async function revokeNode(node: JudgeNode) {
  busyNodeId.value = node.id
  try {
    await api.post<{ ok: boolean }>(`/api/nodes/${node.id}/revoke`)
    ok('节点已撤销', node.name)
    await loadNodes(node.pool_id)
  } catch (e) {
    fail(e, '撤销节点失败')
  } finally {
    busyNodeId.value = null
  }
}

function addRoutingRow() {
  routing.value = [...routing.value, { pool_id: '', priority: routing.value.length + 1 }]
}

function removeRoutingRow(index: number) {
  routing.value = routing.value.filter((_, i) => i !== index)
}

async function saveRouting() {
  const entries = routing.value.filter((e) => e.pool_id)
  savingRouting.value = true
  try {
    await api.post<{ ok: boolean }>(`/api/organizations/${orgId.value}/routing`, { entries })
    ok('路由策略已保存')
    await loadRouting()
  } catch (e) {
    fail(e, '保存路由策略失败')
  } finally {
    savingRouting.value = false
  }
}

function nodesOf(poolId: string): JudgeNode[] {
  return nodesByPool.value.get(poolId) ?? []
}

function nodeStateColor(node: JudgeNode) {
  if (node.revoked) return 'error' as const
  if (!node.enabled) return 'neutral' as const
  return node.connected ? ('success' as const) : ('warning' as const)
}

function nodeStateLabel(node: JudgeNode) {
  if (node.revoked) return '已撤销'
  if (!node.enabled) return '已禁用'
  return node.connected ? '在线' : '离线'
}
</script>

<template>
  <div class="space-y-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <div>
        <h1 class="text-2xl font-semibold text-highlighted">评测资源</h1>
        <p class="text-sm text-muted">
          路由仅在组织级配置，所有比赛统一使用。满载但可排队仍属可用，不会静默回退到其他池。
        </p>
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

    <UAlert
      v-if="loadError"
      color="error"
      variant="subtle"
      icon="i-lucide-triangle-alert"
      title="无法加载评测资源"
      :description="loadError"
    />

    <UAlert
      v-else-if="!loading && !canManage"
      color="warning"
      variant="subtle"
      icon="i-lucide-lock"
      title="权限不足"
      description="只有组织管理员可以管理评测资源池与路由策略。"
    />

    <USkeleton v-else-if="loading" class="h-40 w-full" />

    <template v-else>
      <UCard>
        <template #header>
          <div class="flex flex-wrap items-center justify-between gap-2">
            <div class="flex items-center gap-2">
              <UIcon name="i-lucide-server" class="size-5 text-primary" />
              <span class="font-medium">评测资源池（{{ pools.length }}）</span>
            </div>
            <UButton size="sm" color="primary" icon="i-lucide-plus" @click="poolOpen = true">
              新建资源池
            </UButton>
          </div>
        </template>

        <UEmpty
          v-if="!pools.length"
          icon="i-lucide-server"
          title="暂无资源池"
          description="创建资源池后即可注册评测节点。"
          variant="subtle"
        />

        <div v-else class="space-y-4">
          <div v-for="pool in pools" :key="pool.id" class="rounded-lg border border-default p-4">
            <div class="flex flex-wrap items-center justify-between gap-2">
              <div class="flex flex-wrap items-center gap-2">
                <span class="font-medium text-highlighted">{{ pool.name }}</span>
                <UBadge
                  :color="pool.kind === 'system' ? 'secondary' : 'primary'"
                  variant="subtle"
                  size="sm"
                >
                  {{ POOL_KIND_LABELS[pool.kind] }}
                </UBadge>
                <UBadge v-if="!pool.enabled" color="neutral" variant="subtle" size="sm"
                  >已禁用</UBadge
                >
              </div>
              <UButton
                size="sm"
                color="neutral"
                variant="outline"
                icon="i-lucide-plus"
                @click="openNodeDialog(pool.id)"
              >
                注册节点
              </UButton>
            </div>

            <div class="mt-3">
              <UEmpty
                v-if="!nodesOf(pool.id).length"
                icon="i-lucide-hard-drive"
                title="暂无节点"
                variant="subtle"
                size="sm"
              />
              <ul v-else class="divide-y divide-default">
                <li
                  v-for="node in nodesOf(pool.id)"
                  :key="node.id"
                  class="flex flex-wrap items-center justify-between gap-3 py-2"
                >
                  <div class="flex flex-wrap items-center gap-2">
                    <UBadge :color="nodeStateColor(node)" variant="subtle" size="sm">
                      {{ nodeStateLabel(node) }}
                    </UBadge>
                    <span class="font-medium">{{ node.name }}</span>
                    <span class="text-xs text-muted">
                      槽位 {{ node.max_slots }} ·
                      {{
                        node.last_heartbeat_at
                          ? `心跳 ${formatRelative(node.last_heartbeat_at)}`
                          : '无心跳'
                      }}
                    </span>
                  </div>
                  <div class="flex items-center gap-2">
                    <div v-if="node.advertised_profiles?.length" class="hidden gap-1 lg:flex">
                      <UBadge
                        v-for="profile in node.advertised_profiles"
                        :key="profile"
                        color="neutral"
                        variant="subtle"
                        size="sm"
                        class="font-mono"
                      >
                        {{ profile }}
                      </UBadge>
                    </div>
                    <UButton
                      size="xs"
                      color="neutral"
                      variant="outline"
                      icon="i-lucide-refresh-cw"
                      :loading="busyNodeId === node.id"
                      @click="rotateNode(node)"
                    >
                      轮换凭据
                    </UButton>
                    <UButton
                      size="xs"
                      color="error"
                      variant="outline"
                      icon="i-lucide-ban"
                      :disabled="node.revoked"
                      :loading="busyNodeId === node.id"
                      @click="revokeNode(node)"
                    >
                      撤销
                    </UButton>
                  </div>
                </li>
              </ul>
            </div>
          </div>
        </div>
      </UCard>

      <UCard>
        <template #header>
          <div class="flex flex-wrap items-center justify-between gap-2">
            <div class="flex items-center gap-2">
              <UIcon name="i-lucide-route" class="size-5 text-primary" />
              <span class="font-medium">路由策略</span>
            </div>
            <div class="flex gap-2">
              <UButton
                size="sm"
                color="neutral"
                variant="outline"
                icon="i-lucide-plus"
                @click="addRoutingRow"
              >
                添加级别
              </UButton>
              <UButton size="sm" color="primary" :loading="savingRouting" @click="saveRouting">
                保存
              </UButton>
            </div>
          </div>
        </template>

        <p class="mb-3 text-sm text-muted">
          数值越大优先级越高。同一优先级内按空闲槽位负载均衡；该级别全部不可用才使用下一级。
        </p>

        <UEmpty
          v-if="!routing.length"
          icon="i-lucide-route"
          title="未配置路由"
          description="所有级别都不可用时评测会报调度错误，等待管理员重新分配。"
          variant="subtle"
        />

        <div v-else class="space-y-2">
          <div v-for="(entry, index) in routing" :key="index" class="flex items-end gap-3">
            <UFormField label="资源池" class="flex-1">
              <USelect
                v-model="entry.pool_id"
                :items="poolOptions"
                class="w-full"
                placeholder="选择资源池"
              />
            </UFormField>
            <UFormField label="优先级" class="w-32">
              <UInputNumber v-model="entry.priority" :min="0" class="w-full" />
            </UFormField>
            <UButton
              color="error"
              variant="ghost"
              icon="i-lucide-trash-2"
              aria-label="删除该级别"
              @click="removeRoutingRow(index)"
            />
          </div>
        </div>
      </UCard>
    </template>

    <UModal v-model:open="poolOpen" title="新建资源池">
      <template #body>
        <UForm
          id="create-pool-form"
          :state="poolState"
          :validate="validatePool"
          class="space-y-4"
          @submit="createPool"
        >
          <UFormField label="资源池名称" name="name" required>
            <UInput v-model="poolState.name" placeholder="组织独享池" class="w-full" autofocus />
          </UFormField>
          <UFormField v-if="isSiteAdmin" label="类型" name="kind">
            <USelect
              v-model="poolState.kind"
              :items="[
                { label: '组织独享池', value: 'organization' },
                { label: '系统评测资源池', value: 'system' },
              ]"
              class="w-full"
            />
          </UFormField>
        </UForm>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" @click="poolOpen = false">取消</UButton>
          <UButton type="submit" form="create-pool-form" :loading="creatingPool" color="primary"
            >创建</UButton
          >
        </div>
      </template>
    </UModal>

    <UModal v-model:open="nodeOpen" title="注册评测节点">
      <template #body>
        <div class="space-y-4">
          <UAlert
            color="info"
            variant="subtle"
            icon="i-lucide-key-round"
            title="注册后会一次性返回节点凭据"
            description="凭据用于节点主动建立 gRPC 流，可随时轮换或撤销。"
          />
          <UFormField label="节点名称" required>
            <UInput v-model="nodeState.name" placeholder="judge-01" class="w-full" autofocus />
          </UFormField>
        </div>
      </template>
      <template #footer>
        <div class="flex w-full justify-end gap-2">
          <UButton color="neutral" variant="ghost" @click="nodeOpen = false">取消</UButton>
          <UButton :loading="registeringNode" color="primary" @click="registerNode">注册</UButton>
        </div>
      </template>
    </UModal>

    <NodeCredentialModal
      v-model:open="credentialOpen"
      :node-name="credentialNodeName"
      :credential="credentialValue"
    />
  </div>
</template>
