# Story 3.3: End Sessions Manually or by Timer

Status: done

## Story
As an operator, I want to end a session manually or automatically when the timer expires, so that the session locks before late photos can enter.

## Acceptance Criteria
1. Operator can end an active session manually with confirmation.
2. Backend marks ended session as `locked` and records `ended_at` plus `end_reason` (`manual` or `timer`).
3. Timer-expired active sessions are treated as locked by backend list/start logic without requiring browser state.
4. Starting a new session for the same station is allowed only after the previous active session is locked.
5. Activity records `session.ended` and `session.end_failed` safely.
6. SSE publishes `session.ended`; dashboard refreshes sessions/live cards/activity/readiness.
7. End endpoint uses local auth and trusted Origin guard.
8. OpenAPI documents endpoint, schemas, and errors.
9. Tests/build pass.

## Tasks / Subtasks
- [x] Extend session status/lifecycle with locked ended sessions.
- [x] Add `POST /api/sessions/{session_id}/end`.
- [x] Auto-lock expired sessions in backend list/start paths.
- [x] Add frontend end session button with confirmation.
- [x] Add SSE invalidation and activity.
- [x] Update OpenAPI/tests.

## Dev Notes
No ingestion/photo routing yet. Locking here prepares routing boundaries for Epic 4.

### Review Findings

- [x] [Review][Patch] OpenAPI missing end-session endpoint/schema/errors.
- [x] [Review][Patch] Timer auto-lock did not persist or emit activity/SSE; handler now persists/publishes expired locks from list/start paths.
- [x] [Review][Patch] End request accepted trailing JSON; added second decode guard.
- [x] [Review][Patch] Frontend accepted arbitrary session status and malformed ended_at; validator tightened.
- [x] [Review][Defer] Full rollback of manual end persistence failure; broader transactional persistence hardening tracked as deferred.
- [x] [Review][Defer] Live countdown interval; current polling remains acceptable shell behavior.

## Dev Agent Record

### Debug Log References

- `cd apps/agent && go test ./...` -> passed.
- `cd apps/web && npm run typecheck && npm run build` -> passed.

### Completion Notes List

- Applied Story 3.3 review patches: OpenAPI endpoint/schema, timer-lock persistence/SSE/activity, trailing JSON guard, stricter frontend session validation.
- Validation after review patches passed: `cd apps/agent && go test ./...`; `cd apps/web && npm run typecheck && npm run build`.
- Added locked session lifecycle with manual/timer end reasons.
- Added `POST /api/sessions/{session_id}/end` with auth/origin guard.
- Auto-locks expired active sessions in list/start paths.
- Added frontend End session button with confirmation.
- Added `session.ended` activity/SSE invalidation.

### File List

- `apps/agent/internal/sessions/store.go`
- `apps/agent/internal/api/sessions.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/sessions_test.go`
- `apps/web/src/lib/api/client.ts`
- `apps/web/src/features/sessions/use-end-session-mutation.ts`
- `apps/web/src/features/sessions/live-station-cards.tsx`
- `apps/web/src/features/health/health-dashboard.tsx`
- `docs/api/openapi.yaml`
