import { resetMock } from '~/mock'

/**
 * Whether to serve the API from the in-browser mock.
 *
 * The default comes from `runtimeConfig.public.apiMock`, which is on for
 * `npm run dev` and off for a production build (see the `$development` block in
 * `nuxt.config.ts`). A `localStorage` override wins over it, so the mode can be
 * flipped from the mock console without restarting the dev server.
 */
const STORAGE_KEY = 'ephyra.mock'

function readOverride(): boolean | null {
  if (typeof localStorage === 'undefined') return null
  const value = localStorage.getItem(STORAGE_KEY)
  if (value === 'on') return true
  if (value === 'off') return false
  return null
}

export function useMockEnabled(): boolean {
  const override = readOverride()
  if (override !== null) return override
  return useRuntimeConfig().public.apiMock === true
}

/** `null` clears the override and falls back to the build-time flag. */
export function setMockEnabled(enabled: boolean | null): void {
  if (typeof localStorage === 'undefined') return
  if (enabled === null) localStorage.removeItem(STORAGE_KEY)
  else localStorage.setItem(STORAGE_KEY, enabled ? 'on' : 'off')
  // Switching modes must not leave mock state or a mock session behind.
  if (enabled === false) resetMock()
  if (typeof window !== 'undefined') window.location.reload()
}
