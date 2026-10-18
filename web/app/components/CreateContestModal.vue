<script setup lang="ts">
import type { FormError, FormSubmitEvent } from '#ui/types'
import type { Contest, ContestRule, EntryMode, FeedbackGranularity } from '~/types/api'

const props = defineProps<{ organizationId: string }>()
const open = defineModel<boolean>('open', { default: false })
const emit = defineEmits<{ created: [Contest] }>()

const api = useApi()
const { fail, ok } = useApiToast()

const now = new Date()
const defaults = () => ({
  slug: '',
  title: '',
  rule: 'acm' as ContestRule,
  entry_mode: 'individual' as EntryMode,
  register_starts_at: toLocalInput(now),
  starts_at: toLocalInput(new Date(now.getTime() + 60 * 60 * 1000)),
  ends_at: toLocalInput(new Date(now.getTime() + 4 * 60 * 60 * 1000)),
  allow_external: false,
  registration_requires_review: false,
  max_team_size: 3,
  acm_penalty_minutes: 20,
  freeze_minutes_before_end: null as number | null,
  ioi_public_scoreboard: false,
  case_feedback: 'subtask' as FeedbackGranularity,
  source_public_after: false,
})

const state = reactive(defaults())
const loading = ref(false)

const ruleItems = [
  { label: 'ACM', value: 'acm' },
  { label: 'OI', value: 'oi' },
  { label: 'IOI', value: 'ioi' },
]
const entryItems = [
  { label: '个人', value: 'individual' },
  { label: '队伍', value: 'team' },
]
const feedbackItems = [
  { label: '按子任务', value: 'subtask' },
  { label: '按测试点', value: 'case' },
]

function validate(input: typeof state): FormError[] {
  const errors: FormError[] = []
  if (!input.slug.trim()) errors.push({ name: 'slug', message: '请输入比赛标识' })
  if (!input.title.trim()) errors.push({ name: 'title', message: '请输入比赛名称' })
  const start = new Date(input.starts_at).getTime()
  const end = new Date(input.ends_at).getTime()
  const register = new Date(input.register_starts_at).getTime()
  if (Number.isNaN(start) || Number.isNaN(end)) {
    errors.push({ name: 'starts_at', message: '请选择开始与结束时间' })
  } else {
    if (end <= start) errors.push({ name: 'ends_at', message: '结束时间必须晚于开始时间' })
    if (register > end) {
      errors.push({ name: 'register_starts_at', message: '报名开始时间不能晚于比赛结束时间' })
    }
  }
  if (input.entry_mode === 'team' && input.max_team_size < 1) {
    errors.push({ name: 'max_team_size', message: '队伍人数上限至少为 1' })
  }
  return errors
}

async function onSubmit(event: FormSubmitEvent<typeof state>) {
  const input = event.data
  loading.value = true
  try {
    const contest = await api.post<Contest>(`/api/organizations/${props.organizationId}/contests`, {
      slug: input.slug.trim().toLowerCase(),
      title: input.title.trim(),
      rule: input.rule,
      entry_mode: input.entry_mode,
      register_starts_at: localInputToIso(input.register_starts_at),
      starts_at: localInputToIso(input.starts_at),
      ends_at: localInputToIso(input.ends_at),
      allow_external: input.allow_external,
      registration_requires_review: input.registration_requires_review,
      max_team_size: input.entry_mode === 'team' ? input.max_team_size : 0,
      acm_penalty_minutes: input.rule === 'acm' ? input.acm_penalty_minutes : 0,
      freeze_minutes_before_end: input.rule === 'acm' ? input.freeze_minutes_before_end : null,
      ioi_public_scoreboard: input.rule === 'ioi' ? input.ioi_public_scoreboard : false,
      case_feedback: input.case_feedback,
      source_public_after: input.source_public_after,
    })
    ok('比赛已创建', contest.title)
    emit('created', contest)
    open.value = false
    Object.assign(state, defaults())
  } catch (e) {
    fail(e, '创建比赛失败')
  } finally {
    loading.value = false
  }
}
</script>

<template>
  <UModal
    v-model:open="open"
    title="创建比赛"
    description="赛制在开赛后冻结，报名类型创建后不可修改。"
    :ui="{ content: 'max-w-3xl' }"
  >
    <template #body>
      <UForm
        id="create-contest-form"
        :state="state"
        :validate="validate"
        class="space-y-6"
        @submit="onSubmit"
      >
        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField label="比赛标识" name="slug" required>
            <UInput v-model="state.slug" placeholder="spring-2026" class="w-full" />
          </UFormField>
          <UFormField label="比赛名称" name="title" required>
            <UInput v-model="state.title" placeholder="2026 春季赛" class="w-full" />
          </UFormField>
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField label="赛制" name="rule" required :help="RULE_DESCRIPTIONS[state.rule]">
            <USelect v-model="state.rule" :items="ruleItems" class="w-full" />
          </UFormField>
          <UFormField label="报名类型" name="entry_mode" required>
            <USelect v-model="state.entry_mode" :items="entryItems" class="w-full" />
          </UFormField>
        </div>

        <div class="grid gap-4 sm:grid-cols-3">
          <UFormField label="报名开始" name="register_starts_at" required>
            <UInput v-model="state.register_starts_at" type="datetime-local" class="w-full" />
          </UFormField>
          <UFormField label="比赛开始" name="starts_at" required>
            <UInput v-model="state.starts_at" type="datetime-local" class="w-full" />
          </UFormField>
          <UFormField label="比赛结束" name="ends_at" required>
            <UInput v-model="state.ends_at" type="datetime-local" class="w-full" />
          </UFormField>
        </div>

        <div class="space-y-3">
          <USwitch v-model="state.allow_external" label="允许组织外的站点用户参赛" />
          <USwitch v-model="state.registration_requires_review" label="报名需要比赛管理员审核" />
          <USwitch v-model="state.source_public_after" label="赛后公开源码（已登录账号可查看）" />
        </div>

        <div class="grid gap-4 sm:grid-cols-2">
          <UFormField
            v-if="state.entry_mode === 'team'"
            label="队伍人数上限"
            name="max_team_size"
            required
            help="允许 1～N 人，单人队伍同样进入队伍榜"
          >
            <UInputNumber v-model="state.max_team_size" :min="1" class="w-full" />
          </UFormField>

          <UFormField
            v-if="state.rule === 'acm'"
            label="每次错误提交的罚时（分钟）"
            name="acm_penalty_minutes"
          >
            <UInputNumber v-model="state.acm_penalty_minutes" :min="0" class="w-full" />
          </UFormField>

          <UFormField
            v-if="state.rule === 'acm'"
            label="封榜时长（分钟，留空不封榜）"
            name="freeze_minutes_before_end"
          >
            <UInputNumber v-model="state.freeze_minutes_before_end" :min="0" class="w-full" />
          </UFormField>

          <UFormField
            v-if="state.rule === 'ioi'"
            label="赛中公开实时榜单"
            name="ioi_public_scoreboard"
          >
            <USwitch v-model="state.ioi_public_scoreboard" label="公开榜单" />
          </UFormField>

          <UFormField
            label="反馈粒度"
            name="case_feedback"
            help="OI 赛中只反馈编译状态，该开关在赛后公开阶段生效"
          >
            <USelect v-model="state.case_feedback" :items="feedbackItems" class="w-full" />
          </UFormField>
        </div>
      </UForm>
    </template>
    <template #footer>
      <div class="flex w-full justify-end gap-2">
        <UButton color="neutral" variant="ghost" @click="open = false">取消</UButton>
        <UButton type="submit" form="create-contest-form" :loading="loading" color="primary">
          创建比赛
        </UButton>
      </div>
    </template>
  </UModal>
</template>
