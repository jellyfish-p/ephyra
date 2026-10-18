/**
 * Keep signed-in users away from login/register. Use as
 * `definePageMeta({ middleware: 'guest' })`.
 */
export default defineNuxtRouteMiddleware(async () => {
  const { refresh, isLoggedIn } = useSession()
  try {
    await refresh()
  } catch {
    return
  }
  if (isLoggedIn.value) {
    return navigateTo('/')
  }
})
