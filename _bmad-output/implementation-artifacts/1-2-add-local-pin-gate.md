# Story 1.2: Add Local PIN Gate

Status: done

## Story

As an operator,
I want dashboard access protected by a local PIN/password gate,
so that only authorized event staff can use operational controls.

## Acceptance Criteria

1. Given app is running without active authenticated session, when operator opens dashboard, then operator sees PIN/password gate before operational UI.
2. Given operator enters the valid PIN/password, when login is submitted, then Go service creates a local authenticated session and dashboard shows the operational UI.
3. Given operator enters an invalid PIN/password, when login is submitted, then UI shows an actionable error without leaking the configured PIN/password.
4. Given operator is authenticated, when logout is submitted, then local authenticated session is cleared and dashboard returns to the PIN/password gate.
5. Given browser/frontend code is inspected, when credentials and config are reviewed, then browser never receives Supabase service role key, Google Cloud credentials, or the configured PIN/password.
6. Given protected operational endpoints are called without a valid local auth session, when Go service handles the request, then it denies the request with an operator-actionable unauthorized error shape.

## Tasks / Subtasks

- [x] Add Go local auth configuration (AC: 2, 3, 5)
  - [x] Add `SELFSTUDIO_AUTH_PIN` / password-style env support in `apps/agent/internal/config` without printing or returning the value.
  - [x] Fail startup with a clear actionable config error when auth secret is missing for non-test runtime.
  - [x] Update root and agent `.env.example` with placeholder auth secret only; never commit real PIN/password.
  - [x] Add config tests for missing auth secret, whitespace secret, and valid secret.
- [x] Add Go auth/session package and handlers (AC: 2, 3, 4, 6)
  - [x] Create `apps/agent/internal/auth` for local session creation, validation, and clearing.
  - [x] Store sessions server-side in memory for MVP with cryptographically random session tokens.
  - [x] Set session token via `HttpOnly`, `SameSite=Lax` cookie scoped to the local Go service.
  - [x] Add `POST /api/auth/login`, `POST /api/auth/logout`, and `GET /api/auth/session` handlers.
  - [x] Keep `/health` public.
  - [x] Add reusable middleware/guard for future protected `/api/*` commands.
  - [x] Return API responses in the architecture shape: success `{ "data": ... }`, failures `{ "error": { "code", "message", "action", "details" } }`.
- [x] Add frontend PIN/password gate (AC: 1, 2, 3, 4, 5)
  - [x] Convert the dashboard entry to a Client Component where needed for form state and auth calls.
  - [x] Create a login gate UI before operational dashboard content is visible.
  - [x] Add login/logout flows that call only the Go API; do not embed the configured PIN/password in frontend env or code.
  - [x] Show invalid-login error text that is actionable and does not reveal the configured secret.
  - [x] Ensure unauthenticated initial render does not show operational controls before session check completes.
- [x] Update API docs and contracts (AC: 2, 3, 4, 6)
  - [x] Extend `docs/api/openapi.yaml` with auth endpoints and unauthorized error response schema.
  - [x] Use `snake_case` JSON fields for API payloads and examples.
  - [x] Document that credentials/secrets are server-side only.
- [x] Update Windows scripts/env flow (AC: 2, 5)
  - [x] Ensure `scripts/dev.ps1` and `scripts/start.ps1` pass `SELFSTUDIO_AUTH_PIN` to the Go process when present.
  - [x] Ensure missing auth secret fails before launching services with a clear message.
- [x] Add tests and verification (AC: 1, 2, 3, 4, 5, 6)
  - [x] Add Go tests for login success, invalid login, session check, logout, and protected middleware.
  - [x] Add frontend typecheck/build validation.
  - [x] Add manual smoke notes for unauthenticated gate, valid login, invalid login, logout, and `/health` remaining public.

### Review Findings

- [x] [Review][Patch] Browser auth calls will fail cross-origin because Go API lacks credentialed CORS and OPTIONS preflight support. [`apps/agent/internal/api`, `apps/web/src/lib/api/client.ts`]
- [x] [Review][Patch] Sessions never expire server-side despite 12-hour cookie expiry; abandoned/stolen tokens remain valid until process restart/logout. [`apps/agent/internal/auth/session.go`]
- [x] [Review][Patch] Login endpoint has no brute-force throttling or temporary lockout for repeated invalid PIN attempts. [`apps/agent/internal/auth/session.go`, `apps/agent/internal/api/auth.go`]
- [x] [Review][Patch] Placeholder auth PIN `change-this-local-pin` is accepted as a real runtime secret. [`apps/agent/internal/config/config.go`, `.env.example`]
- [x] [Review][Patch] Login request body is unbounded and accepts malformed/trailing JSON input. [`apps/agent/internal/api/auth.go`]
- [x] [Review][Patch] Frontend API client assumes every response is JSON and can mask connection/CORS/non-JSON failures with weak diagnostics. [`apps/web/src/lib/api/client.ts`]
- [x] [Review][Patch] LAN dashboard URL is misleading because default API URL remains `localhost` and agent binds loopback by default. [`scripts/dev.ps1`, `scripts/start.ps1`]
- [x] [Review][Patch] OpenAPI omits implemented `400 INVALID_REQUEST` and `500 AUTH_SESSION_FAILED` login responses. [`docs/api/openapi.yaml`]

## Dev Notes

### Scope Boundary

This story implements only the local MVP access gate. Do not implement Supabase Auth, RBAC, user management, roles, remote admin login, station configuration, SSE, camera watcher, session management, or cloud credentials UI here.

### Architecture Requirements

- MVP auth is a local PIN/password gate implemented in Go service.
- Go service validates local auth token/session for API commands.
- Browser talks only to Go service API.
- Browser must never receive Supabase service role keys, Google Cloud credentials, or the configured PIN/password.
- Service credentials and local auth secrets stay server-side in Go config/environment.
- API endpoints under `/api`; `/health` remains public operational health.
- JSON fields use `snake_case`.
- API success responses use `{ "data": ... }`; API errors use `{ "error": { "code", "message", "action", "details" } }`.
- Go packages use short lowercase names; auth package under `apps/agent/internal/auth`.

### Previous Story Intelligence

Story 1.1 established the monorepo skeleton, Go health/config patterns, Next.js App Router app, tracked env examples, ignored build/runtime outputs, and validation commands.

## Dev Agent Record

### Agent Model Used

GPT-5.1 Codex Max

### Debug Log References

- `cd apps/agent && go test ./...` -> passed.
- `cd apps/web && npm run typecheck && npm run build` -> passed.
- `biome_check` on web paths -> failed with harness error `spawn EINVAL`.
- Manual smoke with `SELFSTUDIO_AUTH_PIN=123456 go run ./cmd/selfstudio-agent`:
  - `GET /health` returned `200` without auth.
  - `GET /api/auth/session` returned `{ "data": { "authenticated": false } }` before login.
  - Invalid `POST /api/auth/login` returned `401 INVALID_PIN` without leaking the configured PIN.
  - Valid `POST /api/auth/login` returned `200`, set `HttpOnly; SameSite=Lax` `selfstudio_session` cookie, and returned authenticated true.
  - Authenticated `GET /api/auth/session` returned authenticated true with cookie.
  - `POST /api/auth/logout` cleared session cookie and returned authenticated false.

### Completion Notes List

- Implemented server-side local PIN/password configuration via `SELFSTUDIO_AUTH_PIN`.
- Added in-memory local auth session manager with cryptographically random tokens and constant-time PIN comparison.
- Added auth API endpoints for login, logout, and session check with `{data}` and `{error}` response wrappers.
- Added reusable Go auth middleware for future protected operational endpoints while keeping `/health` public.
- Added frontend local PIN gate that hides operational dashboard until session check/login succeeds.
- Updated Windows scripts to require/pass `SELFSTUDIO_AUTH_PIN` before launching the Go process.
- Updated OpenAPI contract and env examples for local auth.
- Added Go auth/config/API tests and validated frontend typecheck/build.

### Change Log

- 2026-05-18: Created Story 1.2 local PIN/password gate context and marked ready for development.
- 2026-05-18: Implemented local PIN/password gate and marked ready for review.

### File List

- `.env.example`
- `apps/agent/.env.example`
- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/auth.go`
- `apps/agent/internal/api/auth_test.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/health_test.go`
- `apps/agent/internal/api/middleware.go`
- `apps/agent/internal/api/response.go`
- `apps/agent/internal/auth/session.go`
- `apps/agent/internal/auth/session_test.go`
- `apps/agent/internal/config/config.go`
- `apps/agent/internal/config/config_test.go`
- `apps/web/src/app/globals.css`
- `apps/web/src/app/page.tsx`
- `apps/web/src/features/auth/local-pin-gate.tsx`
- `apps/web/src/lib/api/client.ts`
- `apps/web/tsconfig.json`
- `docs/api/openapi.yaml`
- `scripts/dev.ps1`
- `scripts/start.ps1`
