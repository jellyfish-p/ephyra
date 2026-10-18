# Ephyra Web

Nuxt 4 + `@nuxt/ui` frontend for the Ephyra online judge (`../server`).

## Running

```bash
npm install
npm run dev        # http://localhost:3000 — API served by the in-browser mock
npm run check      # format:check + typecheck, i.e. what CI runs
npm test
npm run build      # .output/
```

`npm run dev` needs **no backend**: it serves the API from the in-browser mock
(see "Mock mode" below). To develop against the real Go server instead:

```bash
cd ../server && go run ./cmd/ephyra     # :8080 HTTP, :9090 gRPC
cd ../web && NUXT_PUBLIC_API_MOCK=false npm run dev
```

Either way the frontend calls `/api` on its own origin and Nitro proxies it to
the backend (see "Why the frontend must be same-origin"). The backend origin
defaults to `http://localhost:8080`; set `NUXT_BACKEND_ORIGIN` to change it.

## Formatting

Prettier owns the style, and `npm run format` applies it:

```bash
npm run format         # rewrite in place
npm run format:check   # what CI runs; fails on any drift
```

`.prettierrc.json` keeps the style this codebase already used — 100 columns,
single quotes, no semicolons — rather than imposing a new one. `endOfLine` is
pinned to `lf`, which matters here: the Go tree is checked out CRLF on Windows
(`core.autocrlf=true`), and `.gitattributes` pins `web/**` to LF so the frontend
does not inherit that. Without the pin, a Windows edit could silently add CRLF
and the format check would fail for a reason that has nothing to do with style.

`.github/workflows/quality.yml` runs `gofmt` and `go vet` for the Go tree and
`format:check` plus `nuxt typecheck` for the frontend. The Go tests stay in
`test.yml`, and the frontend's 54 vitest tests are not part of either — run them
with `npm test`.

CI pins Node to the version in `.nvmrc` (22, the LTS line Nuxt supports); local
development on 24 is fine.

## Mock mode

The whole API can be served from inside the browser instead of the Go server.
`npm run dev` does this by default, so the frontend runs with nothing else
installed. To turn it off for a dev session:

```bash
NUXT_PUBLIC_API_MOCK=false npm run dev
```

A production build does the opposite: it talks to the real backend, and
`NUXT_PUBLIC_API_MOCK=true` opts a build back into the mock (handy for a demo).

The flag is read by `useMockEnabled()`, and a `localStorage` entry
(`ephyra.mock` = `on`/`off`) overrides it, so the mode can be toggled from the
mock console in the app header without a restart.

This is **not** a set of hand-written fixtures. It is a port of the Go handlers,
kept deliberately faithful in the four places where a plausible-looking fake
would teach the frontend the wrong thing:

- **Status codes and error shape.** Errors are `{"error":{"code","message"}}`
  with the status from `statusFor`, so `403` really means "not visible" and the
  UI's branching on status is exercised rather than assumed.
- **Visibility.** `app/mock/scoring.ts` and `app/mock/access.ts` are ports of
  `scoring.Visible`, `scoring.EmitFields`, `scoring.ProjectSubmission` and
  `access.Audience`. Absence still means hidden: a field the viewer may not see is
  omitted, never nulled. The audience ladder (owner → admin → self → rival →
  signed-in → anonymous) is what the tests pin hardest.
- **Scoring.** ACM penalty arithmetic, the freeze window, OI/IOI partial credit
  and `exact.Score`'s decimal-string form all come from a port of
  `scoring.ScoreContest`, so a scoreboard in mock mode is arithmetically the same
  board the backend would produce.
- **No invented fields.** Anything the backend does not send does not exist here
  either. Where something genuinely cannot happen in a browser — email, and
  judging that takes time — it is exposed through a separate reserved prefix
  (`/api/_mock/...`) rather than by bending a real response shape.

Ids are readable and deliberately invalid as UUIDs (`subm1000-0000-4000-...`), so
a leaked mock id is obvious in a bug report.

### Mock-only endpoints

| Endpoint                             | Purpose                                                          |
| ------------------------------------ | ---------------------------------------------------------------- |
| `GET /api/_mock/state`               | Accounts, contests, organizations, problems, pools, pending work |
| `GET /api/_mock/tokens`              | The verify/reset tokens the backend would have emailed           |
| `POST /api/_mock/judge/:id/complete` | Land a pending verdict immediately                               |
| `POST /api/_mock/reset`              | Rebuild the world and sign out                                   |

### Driving the judge

A submission is judged asynchronously, exactly as the real one is, so a fresh
submission sits pending until the scheduler lands it. To force a verdict, put a
directive in the source:

```cpp
// @verdict=wa
```

Accepted values are `ac`, `wa`, `tle`, `mle`, `rte`, `ce`, `sys`. Without a
directive the verdict is derived from a hash of the source, so the same code
always produces the same result.

### Tests

```bash
npm test        # vitest run
```

`app/mock/mock.test.ts` drives `mockFetch` end to end — CSRF, cookies, status
codes, error bodies — with a frozen clock, so contest phases never drift with the
wall clock. It pins the things most likely to rot: the route table versus the
paths the frontend actually calls, the audience ladder, the ACM penalty
arithmetic, the freeze filter, the two-phase rejudge, and the rule that a node
listing never carries a credential.

`app/structure.test.ts` guards the failure mode the test suite cannot otherwise
see: mistakes that typecheck, build and pass review, then show up only as a blank
page or a dead button. It checks that every page with child pages renders
`<NuxtPage />`, that `UTable` cell slots use the name Nuxt UI v4 actually renders,
and that no select item uses the empty-string value Reka UI reserves.

## Why the frontend must be same-origin

The backend sets its session cookie `ephyra_session` with `SameSite=Lax` and ships
no CORS middleware. A browser will therefore not send the session on a
cross-origin `fetch`, and the request would be unauthenticated no matter what the
frontend does.

So the app calls the API **relative** (`/api/...`) and Nitro proxies it to the
backend — in development and in the built server alike:

```ts
nitro: {
  routeRules: { '/api/**': { proxy: `${backendOrigin}/api/**` } },
}
```

That is a route rule rather than `devProxy`, so `npm run build` produces a server
that is already same-origin with the API and needs no separate reverse proxy. The
proxy forwards request cookies upstream and `Set-Cookie` back down untouched,
which is what makes session auth survive the hop.

`runtimeConfig.public.apiBase` is empty by default; `useApi().apiUrl()` exists
only for the cases where `$fetch` cannot be used (the SSE `EventSource` and
direct source downloads).

Rendering is `ssr: false` on purpose: session and CSRF handling then live
entirely in the browser, and there is no server-side cookie forwarding to get
wrong.

## Auth and CSRF

1. `GET /api/auth/csrf` sets a **readable** `ephyra_csrf` cookie.
2. Every unsafe request (`POST`/`PATCH`/`PUT`/`DELETE`) must echo that value in
   the `X-CSRF-Token` header.

`useApi()` does both automatically. Errors come back as
`{"error":{"code":"...","message":"..."}}` and are normalized into an `ApiError`
with `status` and `code` (`useApi.ts`).

Email delivery is out of scope for this app: the backend mails a bare token, not
a link. `/verify-email` and `/reset-password` therefore accept the token by hand
(they also honour `?token=`).

## Endpoint contract used by this app

### Auth / identity

| Method | Path                        | Notes                    |
| ------ | --------------------------- | ------------------------ |
| GET    | `/api/auth/csrf`            | issues the CSRF cookie   |
| POST   | `/api/auth/register`        | password ≥ 8 chars       |
| POST   | `/api/auth/login`           |                          |
| POST   | `/api/auth/logout`          |                          |
| POST   | `/api/auth/verify-email`    | body `{token}`           |
| POST   | `/api/auth/forgot-password` | body `{email}`           |
| POST   | `/api/auth/reset-password`  | body `{token, password}` |
| GET    | `/api/me`                   |                          |
| GET    | `/api/me/organizations`     | memberships with `role`  |

### Organizations

| Method | Path                                                       | Notes                              |
| ------ | ---------------------------------------------------------- | ---------------------------------- |
| GET    | `/api/organizations`                                       | directory, any signed-in account   |
| POST   | `/api/organizations`                                       | site admin only                    |
| GET    | `/api/organizations/:id`                                   |                                    |
| GET    | `/api/organizations/:id/membership`                        | `{organization_id, member, role?}` |
| GET    | `/api/organizations/:id/members`                           |                                    |
| POST   | `/api/organizations/:id/join`                              |                                    |
| GET    | `/api/organizations/:id/join-requests`                     | managers only                      |
| POST   | `/api/organizations/:id/join-requests/:requestId/decision` | `{approve}`                        |
| GET    | `/api/organizations/:id/problems`                          | problem bank                       |
| POST   | `/api/organizations/:id/problems/import`                   |                                    |
| GET    | `/api/problems/:id/revisions`                              | newest first, managers only        |
| POST   | `/api/problems/:id/revisions`                              |                                    |
| GET    | `/api/organizations/:id/contests`                          |                                    |
| POST   | `/api/organizations/:id/contests`                          |                                    |

### Judge infrastructure

| Method | Path                             | Notes                                     |
| ------ | -------------------------------- | ----------------------------------------- |
| GET    | `/api/organizations/:id/pools`   |                                           |
| POST   | `/api/pools`                     |                                           |
| GET    | `/api/pools/:id/nodes`           | never returns credentials                 |
| POST   | `/api/pools/:id/nodes`           | **returns** the plaintext credential once |
| POST   | `/api/nodes/:id/rotate`          | returns the new credential once           |
| POST   | `/api/nodes/:id/revoke`          |                                           |
| GET    | `/api/organizations/:id/routing` | ordered `{pool_id, priority}`             |
| POST   | `/api/organizations/:id/routing` |                                           |

### Contests

| Method | Path                                                                                         | Notes                                            |
| ------ | -------------------------------------------------------------------------------------------- | ------------------------------------------------ |
| GET    | `/api/contests/:id`                                                                          |                                                  |
| PATCH  | `/api/contests/:id/source-public`                                                            | `{source_public_after}`; locked once running     |
| GET    | `/api/contests/:id/problems`                                                                 | letters; `id` is the contest-problem id          |
| GET    | `/api/contests/:id/problems/:letter`                                                         | statement markdown only, never judge data        |
| POST   | `/api/contests/:id/problems`                                                                 | `{problem_id, revision_id, letter}`              |
| GET    | `/api/contests/:id/standings`                                                                | snake_case, `display_name`, per-problem `letter` |
| GET    | `/api/contests/:id/submissions`                                                              | `?entrant_id&letter&limit&before_seq`            |
| GET    | `/api/contests/:id/events`                                                                   | SSE `submission` frames, viewer-filtered         |
| GET    | `/api/contests/:id/my-entrant`                                                               | `{entrant: ... \| null}`                         |
| GET    | `/api/contests/:id/entrants`                                                                 | `?status`, managers only                         |
| POST   | `/api/contests/:id/register`                                                                 |                                                  |
| POST   | `/api/contests/:id/teams`                                                                    |                                                  |
| POST   | `/api/entrants/:id/invite` \| `confirm` \| `submit-registration` \| `decision` \| `withdraw` |                                                  |

### Submissions and judging

| Method | Path                                             | Notes                                     |
| ------ | ------------------------------------------------ | ----------------------------------------- |
| POST   | `/api/contests/:id/problems/:letter/submissions` | multipart `language` + `source`           |
| GET    | `/api/submissions/:id`                           | omitted fields mean "not visible to you"  |
| GET    | `/api/submissions/:id/source`                    | raw text; 403 when not public yet         |
| GET    | `/api/contests/:id/tasks`                        | `?state&limit`, managers only             |
| POST   | `/api/tasks/:id/redispatch`                      | manual; a lost node is never auto-retried |
| POST   | `/api/contest-problems/:id/rejudge`              | `{revision_id}` → `{batch_id, state}`     |
| POST   | `/api/rejudge-batches/:id/switch`                | publish the batch's scores                |

## Contract rules the UI depends on

- **Decimal strings.** `exact.Score` marshals as a string (`"50.00"`), never a
  binary float. Parse with `Number.parseFloat` only for display math.
- **Absence means hidden.** `GET /api/submissions/:id` drops fields the viewer may
  not see; the UI branches on presence (`verdict === undefined` ⇒ still judging),
  not on nulls.
- **Credentials are shown once.** `POST /api/pools/:id/nodes` and
  `POST /api/nodes/:id/rotate` are the only responses carrying plaintext node
  credentials. The UI forces a copy step before dismissing the dialog.
- **Statement text only.** `GET /api/contests/:id/problems/:letter` returns the
  scoring revision's markdown (≤ 1 MiB) and never samples, tests, checker or
  interactor.
- **Rejudge is two-phase.** Creating a batch keeps the old board (marked
  rejudging); `switch` publishes the new scores. The UI never mixes versions.
- **Entrant-member roles** are `captain` and `member`; organization roles are
  `owner`, `admin` and `member`.

## Layout

```
app/
  components/   StandingsTable, SubmissionResult, StatementView, badges, modals, MockBanner
  composables/  useApi (fetch+CSRF), useApiToast, useSession, useContestEvents (SSE), useMock
  layouts/      default (app shell), auth (centered card)
  middleware/   auth (requires session), guest (redirects signed-in users)
  mock/         the in-browser API: router, handlers/, scoring, access, seed
  pages/        index, login/register/verify/reset, organizations/*, contests/*, submissions/*
  types/        api.ts — every wire DTO
  utils/        format, labels, standings
  structure.test.ts   silent-failure guards (see Tests)
```

### Page structure

`contests/[id].vue` and `organizations/[id].vue` are **route shells**: they declare
the auth middleware and render `<NuxtPage />`, while the overview itself lives in
`[id]/index.vue`. This is not a stylistic choice. A page with children must render
`<NuxtPage />` or Nuxt refuses to display any of them — it emits only a console
warning (`NUXT_E4016`), so `/contests/:id/standings` and friends appear to navigate
and then render nothing.

When adding a page under `contests/[id]/` or `organizations/[id]/`, keep that
shape. `app/structure.test.ts` enforces it.

## Known gaps

- **Statement attachments.** `revision_files` rows other than the markdown
  statement are not exposed, so sample files and images cannot be rendered yet.
- **Email links.** See above — token paste is the only supported flow.
- **Contest editing.** `rule` and `entry_mode` cannot be changed after creation
  because the backend exposes no update route; the admin page says so.
- **Live tests.** The backend's DB-backed tests need `EPHYRA_TEST_DATABASE_URL`
  (see `docker-compose.yml`); without it they skip.
- **Mock is single-tab.** State lives in the page, so two tabs are two separate
  worlds and a reload starts over. It also has no transactions, no row locking and
  no gRPC judge node, so it cannot reproduce a race — only a rule.
