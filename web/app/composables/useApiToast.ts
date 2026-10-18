import type { ApiError } from './useApi'

/** Thin wrapper around Nuxt UI's toaster with Ephyra-shaped defaults. */
export function useApiToast() {
  const toast = useToast()

  function fail(e: unknown, title = '操作失败') {
    const err = asApiError(e)
    toast.add({
      title,
      description: err.message,
      color: 'error',
      icon: 'i-lucide-circle-alert',
    })
    return err
  }

  function ok(title: string, description?: string) {
    toast.add({
      title,
      description,
      color: 'success',
      icon: 'i-lucide-circle-check',
    })
  }

  function info(title: string, description?: string) {
    toast.add({
      title,
      description,
      color: 'info',
      icon: 'i-lucide-info',
    })
  }

  return { fail, ok, info }
}

/**
 * Show a failure toast for any error that is not an auth/visibility error,
 * returning the error so callers can still branch on it.
 */
export function reportError(e: unknown, title?: string): ApiError {
  return useApiToast().fail(e, title)
}
