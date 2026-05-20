# Story 1.4: Create Health Dashboard Shell

Status: done

## Story

As an operator,
I want to see core app health indicators on dashboard,
So that I know whether local service is ready for event operation.

## Acceptance Criteria

1. Given authenticated operator opens dashboard, when dashboard loads, then it shows service health, database reachability, worker placeholder status, disk placeholder status, and event stream status.
2. Given health indicators render, then statuses use text labels and accessible descriptions, not color only.
3. Given dashboard health data is loading or fails, then loading and error states are visible and actionable.
4. Given health data is available, then it refreshes via TanStack Query.
5. Given SSE emits `health.updated`, then dashboard can receive the event and invalidate or update health state.
6. Given UI renders, then it uses shadcn/ui-style local components and existing Next.js App Router conventions.

## Tasks / Subtasks

- [x] Add agent health readiness API (AC: 1, 3, 4)
  - [x] Extend `/api/health` response with `database`, `worker`, and `disk` placeholder status fields using `snake_case`.
  - [x] Preserve existing `{data}` response wrapper and `/health` compatibility alias.
  - [x] Keep placeholder status explicit, operator-actionable, and non-secret.
  - [x] Add Go tests for extended health response shape.
- [x] Add frontend query foundation (AC: 3, 4)
  - [x] Install and configure `@tanstack/react-query` in `apps/web` if not already present.
  - [x] Add a client-side QueryProvider under the App Router layout.
  - [x] Add health query helper/hook that uses existing API response contract helpers.
  - [x] Ensure loading, success, and error branches are visible.
- [x] Add SSE health event integration (AC: 5)
  - [x] Use existing `createEventStream()` and `parseEventEnvelope()` from Story 1.3.
  - [x] Listen for `health.updated` and invalidate the health query.
  - [x] Show event stream status as text: connecting/open/error/closed or equivalent.
  - [x] Clean up EventSource listeners on unmount.
- [x] Add shadcn/ui-style local dashboard components (AC: 1, 2, 6)
  - [x] Create minimal local UI primitives if the shadcn CLI registry is not initialized yet, e.g. Card/Button/Badge patterns under `apps/web/src/components/ui`.
  - [x] Render service health, database reachability, worker placeholder status, disk placeholder status, and event stream status as separate cards or rows.
  - [x] Ensure each status includes text label and actionable helper text.
  - [x] Do not rely on color alone for status meaning.
- [x] Update docs and validation (AC: 1-6)
  - [x] Update `docs/api/openapi.yaml` health schema for added fields.
  - [x] Run `cd apps/agent && go test ./...`.
  - [x] Run `cd apps/web && npm run typecheck && npm run build`.
  - [x] Smoke dashboard behavior after local login if feasible.

### Review Findings

- [x] [Review][Patch] Dashboard shell/page wiring was not included in review diff, making root rendering unverifiable [apps/web/src/app/page.tsx:1]
- [x] [Review][Patch] CORS only allows `localhost:3000`; dashboard opened from `127.0.0.1:3000` can fail auth, health, and SSE [apps/agent/internal/api/cors.go:5]
- [x] [Review][Patch] API client accepts malformed 2xx success payloads without `data`, causing downstream runtime crashes [apps/web/src/lib/api/client.ts:89]
- [x] [Review][Patch] Login response with `authenticated:false` produces no visible feedback [apps/web/src/features/auth/local-pin-gate.tsx:35]
- [x] [Review][Patch] Session check failure is treated as unauthenticated instead of a retryable session-check error [apps/web/src/features/auth/local-pin-gate.tsx:17]
- [x] [Review][Patch] SSE error after session expiry leaves authenticated dashboard visible without rechecking auth [apps/web/src/features/health/health-dashboard.tsx:43]
- [x] [Review][Patch] Health event listener parses envelope but does not verify `event_type`/entity before invalidating query [apps/web/src/features/health/health-dashboard.tsx:58]
- [x] [Review][Patch] OpenAPI shared `DataResponse` uses ambiguous `data: true` schema [docs/api/openapi.yaml:154]
- [x] [Review][Patch] Health API defines unused `HealthResponse` type [apps/agent/internal/api/health.go:5]

## Dev Notes

### Scope Boundary

This story creates the dashboard health shell only. Do not implement real Supabase database probing, real disk space checks, real worker queue processing, station readiness, camera watcher, photo ingestion, image processing, or cloud upload. Use explicit placeholders so operators can distinguish “not implemented yet” from “healthy.”

### Previous Story Intelligence

Story 1.3 is complete and introduced:

- `/api/health` canonical health endpoint and `/health` compatibility alias.
- Shared Go API response shapes: `{ "data": ... }` and `{ "error": { "code", "message", "action", "details" } }`.
- Protected `/events` SSE endpoint.
- Event envelope shape: `event_id`, `event_type`, `entity_type`, `entity_id`, `occurred_at`, `data`.
- Event type dot-notation validation.
- `apps/web/src/lib/events/client.ts` with `createEventStream()` and `parseEventEnvelope()`.
- `apps/web/src/lib/api/client.ts` exports `getHealth()`, `HealthData`, `DataResponse`, and `ErrorResponse`.
- Auth gate already protects dashboard UI via local PIN from Story 1.2.

Story 1.2 introduced:

- Local auth session cookie `selfstudio_session`.
- API client requests use `credentials: "include"`.
- Browser must never receive `SELFSTUDIO_AUTH_PIN` or service credentials.

### Expected Health Data Shape

Extend the existing `HealthData` concept. Suggested shape:

```json
{
  "data": {
    "service": "selfstudio-agent",
    "status": "ok",
    "database": {
      "status": "placeholder",
      "label": "Database belum dikonfigurasi",
      "action": "Konfigurasi Supabase pada story berikutnya."
    },
    "worker": {
      "status": "placeholder",
      "label": "Worker siap sebagai placeholder",
      "action": "Queue worker akan aktif saat pipeline foto dibuat."
    },
    "disk": {
      "status": "placeholder",
      "label": "Disk check belum aktif",
      "action": "Disk usage check akan ditambahkan sebelum event readiness."
    }
  }
}
```

Keep statuses text-friendly. Recommended status vocabulary for now: `ok`, `warning`, `error`, `placeholder`, `unknown`.

### Frontend Guidance

- Story 1.4 owns replacing the minimal authenticated dashboard content in `apps/web/src/app/page.tsx` with a health dashboard shell.
- Keep page component lean; put client behavior in a feature component such as:
  - `apps/web/src/features/health/health-dashboard.tsx`
  - `apps/web/src/features/health/use-health-query.ts`
- Query key can be `['health']`.
- SSE integration should invalidate `['health']` on `health.updated`.
- EventSource code must run only in client components/effects.
- Follow Vercel React performance guidance: avoid unnecessary waterfalls, clean up event listeners, keep dependencies stable, and do not place browser-only code in Server Components.

### shadcn/ui Guidance

The project may not have a shadcn registry initialized yet. If `components.json` does not exist, create small local shadcn-style primitives with Tailwind class names instead of running broad generators. Keep them simple and reusable:

```text
apps/web/src/components/ui/card.tsx
apps/web/src/components/ui/button.tsx
apps/web/src/components/ui/badge.tsx
```

Use semantic HTML, accessible labels, and visible text statuses.

### SSE Guidance

- `/events` is protected by auth cookie; EventSource must use `{ withCredentials: true }` via `createEventStream()`.
- Display event stream status as visible text.
- For this story, receiving a real `health.updated` event may only be possible in tests/manual mocks because no publisher exists yet. Implement the listener and invalidation path; do not add fake production event publishers unless necessary.

### Testing Requirements

Run and record:

- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build`

Manual smoke target:

- Start Go agent with `SELFSTUDIO_AUTH_PIN`.
- Start web dashboard.
- Login with local PIN.
- Confirm health dashboard shows all required indicators with text labels and actionable loading/error states.

### Anti-Patterns to Avoid

- Do not expose secrets to browser env.
- Do not implement real DB/disk/worker probes in this story unless already trivial and non-invasive.
- Do not use color-only status indicators.
- Do not leave health UI hidden behind dev-only text.
- Do not make SSE a hard dependency for initial dashboard rendering; health query should work if stream is unavailable.
- Do not add station/session/photo UI yet.

### References

- [Source: `_bmad-output/planning-artifacts/epics.md` -> Story 1.4]
- [Source: `_bmad-output/implementation-artifacts/1-3-establish-api-error-and-sse-contract-foundation.md` -> Previous Story Intelligence]
- [Source: `_bmad-output/planning-artifacts/architecture.md` -> Frontend State and Realtime Patterns]

## Project Structure Notes

- Go health endpoint lives in `apps/agent/internal/api/health.go`.
- Frontend app lives in `apps/web/src/app`.
- Shared API client lives in `apps/web/src/lib/api/client.ts`.
- Event client lives in `apps/web/src/lib/events/client.ts`.
- Prefer feature code under `apps/web/src/features/health`.

## Dev Agent Record

### Agent Model Used

GPT-5.1 Codex Max

### Debug Log References

- `cd apps/agent && go test ./...` -> passed.
- `cd apps/web && npm run typecheck && npm run build` -> passed.
- Manual smoke: `GET /api/health` returned extended `{data}` payload with service, database, worker, and disk text statuses.
- Review patch validation:
  - `cd apps/agent && go test ./...` -> passed.
  - `cd apps/web && npm run typecheck && npm run build` -> passed.
  - Manual CORS smoke confirmed `http://127.0.0.1:3000` origin receives credentialed CORS headers for `/api/health` and preflight.

### Completion Notes List

- Story created by BMad create-story workflow.
- Extended Go health contract with database/worker/disk placeholder statuses.
- Added TanStack Query provider and health query hook.
- Added authenticated health dashboard shell with loading, error, retry, and text status cards.
- Added SSE `health.updated` listener that invalidates health query and shows event stream status.
- Added local shadcn/ui-style Card, Button, and Badge primitives.
- Updated OpenAPI health schema for extended component statuses.

### Change Log

- 2026-05-18: Created Story 1.4 health dashboard shell and marked ready for development.
- 2026-05-18: Implemented health dashboard shell and marked ready for review.
- 2026-05-18: Applied Story 1.4 code review patches and marked done.

### File List

- `apps/agent/internal/api/cors.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/health_test.go`
- `apps/web/package.json`
- `apps/web/package-lock.json`
- `apps/web/src/app/globals.css`
- `apps/web/src/app/layout.tsx`
- `apps/web/src/app/page.test-anchor.ts`
- `apps/web/src/app/providers.tsx`
- `apps/web/src/components/ui/badge.tsx`
- `apps/web/src/components/ui/button.tsx`
- `apps/web/src/components/ui/card.tsx`
- `apps/web/src/features/auth/local-pin-gate.tsx`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/features/health/use-health-query.ts`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`
