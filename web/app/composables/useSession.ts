import type { Account } from '~/types/api'

/**
 * Site-wide session state.
 *
 * The backend owns the session: an HttpOnly cookie plus a readable CSRF
 * cookie. This composable only caches the resolved `GET /api/me` payload so
 * navigation does not re-fetch it on every page.
 */
export function useSession() {
  const api = useApi()
  const me = useState<Account | null>('session.account', () => null)
  const resolved = useState<boolean>('session.resolved', () => false)
  const pending = useState<boolean>('session.pending', () => false)

  const isLoggedIn = computed(() => me.value !== null)
  const isVerified = computed(() => me.value?.email_verified === true)
  const isSiteAdmin = computed(() => me.value?.site_admin === true)

  /** Resolve the session once. Pass `true` to force a re-fetch. */
  async function refresh(force = false): Promise<Account | null> {
    if (resolved.value && !force) return me.value
    pending.value = true
    try {
      me.value = await api.get<Account>('/api/me')
    } catch (e) {
      const err = asApiError(e)
      if (err.status !== 401) throw err
      me.value = null
    } finally {
      pending.value = false
      resolved.value = true
    }
    return me.value
  }

  async function login(email: string, password: string): Promise<Account> {
    const account = await api.post<Account>('/api/auth/login', { email, password })
    me.value = account
    resolved.value = true
    return account
  }

  async function register(email: string, password: string, displayName: string): Promise<void> {
    await api.post<{ ok: boolean }>('/api/auth/register', {
      email,
      password,
      display_name: displayName,
    })
  }

  async function logout(): Promise<void> {
    try {
      await api.post<{ ok: boolean }>('/api/auth/logout')
    } finally {
      me.value = null
      resolved.value = true
    }
  }

  return {
    me,
    resolved,
    pending,
    isLoggedIn,
    isVerified,
    isSiteAdmin,
    refresh,
    login,
    register,
    logout,
  }
}
