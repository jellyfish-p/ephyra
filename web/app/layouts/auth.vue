<script setup lang="ts">
const { isSiteAdmin } = useSession()
</script>

<template>
  <div class="flex min-h-screen flex-col bg-default">
    <header class="border-b border-default">
      <UContainer>
        <div class="flex h-16 items-center gap-4">
          <NuxtLink to="/" class="flex items-center gap-2 font-semibold text-highlighted">
            <UIcon name="i-lucide-swords" class="size-6 text-primary" />
            <span class="text-lg tracking-tight">Ephyra</span>
          </NuxtLink>
          <UButton
            v-if="isSiteAdmin"
            to="/organizations"
            color="neutral"
            variant="ghost"
            size="sm"
            class="ml-auto"
            icon="i-lucide-building-2"
          >
            组织管理
          </UButton>
        </div>
      </UContainer>
    </header>

    <main class="flex flex-1 items-center justify-center px-4 py-12">
      <div class="w-full max-w-md">
        <slot />
      </div>
    </main>

    <footer class="py-6 text-center text-sm text-muted">Ephyra — 开源在线评测系统</footer>
  </div>
</template>
