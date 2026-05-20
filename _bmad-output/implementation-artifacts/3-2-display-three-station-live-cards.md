# Story 3.2: Display Three Station Live Cards

Status: done

## Story
As an operator, I want a dashboard with three live camera station cards, so that I can monitor the event at a glance.

## Acceptance Criteria
1. Authenticated operator sees exactly three live station cards for `station_1`, `station_2`, and `station_3`.
2. Each card shows station name, status text, readiness summary, active customer/order/timer when a session is active, LUT/background, and photo count placeholder.
3. Backend exposes authenticated session list data so dashboard does not infer active sessions from local UI only.
4. `session.started` SSE refreshes live cards, sessions, readiness, and activity views.
5. Loading/error/empty active-session states are visible and actionable.
6. OpenAPI documents session list endpoint and response schema.
7. Tests/build pass.

## Tasks / Subtasks
- [x] Add `GET /api/sessions` returning active/recent sessions.
- [x] Add frontend session query hook.
- [x] Add live station cards component to dashboard.
- [x] Reuse existing station/readiness/session state; no photo ingestion yet.
- [x] Update OpenAPI.
- [x] Add Go API tests and run validation.

## Dev Notes
Scope: live monitoring shell only. Photo count and last photo preview remain placeholders until ingestion stories.

## Dev Agent Record

### Debug Log References

- `cd apps/agent && go test ./...` -> passed.
- `cd apps/web && npm run typecheck && npm run build` -> passed.

### Completion Notes List

- Added authenticated `GET /api/sessions`.
- Added frontend sessions query and live station cards for exactly three stations.
- Live cards show station status, session customer/order/timer, LUT/background, and photo count placeholder.
- Updated OpenAPI and API tests.

### File List

- `apps/agent/internal/api/sessions.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/sessions_test.go`
- `apps/web/src/features/sessions/use-sessions-query.ts`
- `apps/web/src/features/sessions/use-start-session-mutation.ts`
- `apps/web/src/features/sessions/live-station-cards.tsx`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`
