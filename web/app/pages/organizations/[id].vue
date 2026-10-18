<script setup lang="ts">
/**
 * Route shell for a single organization.
 *
 * Same reason as `contests/[id].vue`: `members` and `judge` are child pages, so
 * the parent has to render `<NuxtPage />` or they never appear. The overview
 * lives in `[id]/index.vue`, so `/organizations/:id` is unchanged.
 *
 * The auth middleware is declared here so it covers the whole subtree.
 */
definePageMeta({ middleware: 'auth' })
</script>

<template>
  <NuxtPage />
</template>
