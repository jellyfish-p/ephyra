/**
 * Gate a page behind a signed-in account. Use as `definePageMeta({ middleware: 'auth' })`.
 */
export default defineNuxtRouteMiddleware(async (to) => {
  const { refresh, isLoggedIn } = useSession()
  try {
    await refresh()
  } catch {
    // Fall through to the signed-out branch.
  }
  if (!isLoggedIn.value) {
    return navigateTo({ path: '/login', query: { redirect: to.fullPath } })
  }
})
