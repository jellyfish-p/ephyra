/**
 * Origin of the Go backend.
 *
 * Nitro proxies `/api` to it in development and in the built server alike, so the
 * frontend is always same-origin with the API. That is not a convenience: the
 * backend sets its session cookie with `SameSite=Lax` and ships no CORS
 * middleware, so a browser will not send the session cross-origin.
 */
const backendOrigin = process.env.NUXT_BACKEND_ORIGIN || 'http://localhost:8080'

export default defineNuxtConfig({
  compatibilityDate: '2025-07-15',

  // The Ephyra API authenticates with HttpOnly session cookies plus a CSRF
  // cookie. Rendering on the client keeps cookies and CSRF handling entirely in
  // the browser, so no session forwarding is needed.
  ssr: false,

  devtools: { enabled: true },

  modules: ['@nuxt/ui'],

  css: ['~/assets/css/main.css'],

  runtimeConfig: {
    public: {
      /**
       * Base URL of the Ephyra Go backend.
       *
       * Empty by default so every request is same-origin and goes through the
       * Nitro `/api` proxy below. Override with NUXT_PUBLIC_API_BASE only when
       * serving from the backend's own origin directly.
       */
      apiBase: '',

      /**
       * Serve the whole API from an in-browser mock instead of the Go backend.
       *
       * Off in a production build, which goes through the `/api` proxy to the
       * real backend; the `$development` block below turns it on for
       * `npm run dev`, so the frontend runs with no backend at all. Either
       * default can be overridden with `NUXT_PUBLIC_API_MOCK`, and the mock
       * banner toggles it per browser without a restart.
       *
       * See `app/mock/` and the "Mock mode" section of the README.
       */
      apiMock: false,
    },
  },

  /**
   * `npm run dev` defaults to the mock.
   *
   * Keyed on the Nuxt environment rather than `import.meta.dev`, which is not
   * populated inside `nuxt.config.ts`.
   */
  $development: {
    runtimeConfig: {
      public: {
        apiMock: true,
      },
    },
  },

  nitro: {
    routeRules: {
      /**
       * Same-origin passthrough to the backend.
       *
       * A route rule rather than `devProxy`, so the built server proxies too and
       * a production deployment does not need a separate reverse proxy. The
       * `/**` target suffix is what makes Nitro strip the `/api` prefix and
       * re-join it onto the backend origin.
       */
      '/api/**': { proxy: `${backendOrigin}/api/**` },
    },
  },

  ui: {
    // Fonts and the MDC pipeline are bundled locally so builds stay hermetic.
    fonts: false,
    mdc: true,
  },

  icon: {
    mode: 'css',
  },

  app: {
    head: {
      title: 'Ephyra',
      htmlAttrs: { lang: 'zh-CN' },
      meta: [
        { charset: 'utf-8' },
        { name: 'viewport', content: 'width=device-width, initial-scale=1' },
        { name: 'description', content: 'Ephyra — 开源在线评测系统' },
      ],
    },
  },
})
