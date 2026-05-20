# Story 3.4: Recover Live Session State After Restart

Status: done

## Story
As an operator, I want live session state recovered after the local app restarts, so that event operation can continue safely without losing active/locked session context.

## Acceptance Criteria
1. On Go agent startup, persisted sessions are loaded and invalid/malformed session state fails startup safely with an actionable error.
2. After restart, `GET /api/sessions` returns active and locked sessions from durable local state.
3. Expired active sessions loaded after restart are locked with `end_reason: timer`, persisted, and surfaced through the session list.
4. Dashboard clearly indicates recovered active/locked sessions after refresh/restart instead of showing empty live cards.
5. Recovery behavior does not create duplicate sessions and still enforces one active session per station.
6. Activity/SSE behavior remains safe: recovery should not invent operator actions, but timer-lock persistence during recovery/list can record/publish `session.ended` once.
7. OpenAPI documents recovered session fields and restart-safe behavior where applicable.
8. Tests/build pass.

## Tasks / Subtasks
- [x] Harden session persistence load and recovery behavior.
- [x] Ensure expired persisted active sessions are locked/persisted on first session list after restart.
- [x] Add recovery metadata or UI copy so operator can distinguish persisted state from empty state.
- [x] Add tests for persistence load, malformed file, duplicate active sessions, expired loaded sessions, and session list after restart.
- [x] Update OpenAPI descriptions/examples as needed.
- [x] Run validation.

## Dev Notes
Session persistence already exists from Story 3.1. This story focuses on proving and surfacing restart recovery behavior, not adding new session lifecycle states beyond active/locked.

## Dev Agent Record

### Debug Log References

- `cd apps/agent && go test ./...` -> passed.
- `cd apps/web && npm run typecheck && npm run build` -> passed.

### Completion Notes List

- Added recovered flag to session list response and frontend UI copy.
- Added tests for expired persisted session recovery and duplicate active enforcement.
- Expired sessions loaded after restart are locked/persisted/published when session list is requested.
- Updated OpenAPI session list response recovery semantics.

### File List

- `apps/agent/internal/api/sessions.go`
- `apps/agent/internal/api/sessions_test.go`
- `apps/web/src/lib/api/client.ts`
- `apps/web/src/features/sessions/live-station-cards.tsx`
- `docs/api/openapi.yaml`
