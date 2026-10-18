import { defineConfig } from 'vitest/config'
import { fileURLToPath } from 'node:url'

export default defineConfig({
  resolve: {
    alias: {
      '~': fileURLToPath(new URL('./app', import.meta.url)),
    },
  },
  test: {
    // No DOM: the mock's cookie layer falls back to an in-memory jar, which is
    // what makes the whole auth flow testable in plain Node.
    environment: 'node',
    include: ['app/**/*.test.ts'],
  },
})
