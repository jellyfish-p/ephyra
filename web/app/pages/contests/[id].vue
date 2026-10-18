<script setup lang="ts">
/**
 * Route shell for a single contest.
 *
 * This route has child pages (`standings`, `submissions`, `admin`,
 * `problems/:letter`), and Nuxt will not display a child unless its parent
 * renders `<NuxtPage />` — without it the child is silently dropped with
 * NUXT_E4016 and the navigation appears to do nothing. The overview itself lives
 * in `[id]/index.vue`, so `/contests/:id` is unchanged.
 *
 * The auth middleware is declared here so it covers the whole subtree.
 */
definePageMeta({ middleware: 'auth' })
</script>

<template>
  <NuxtPage />
</template>
