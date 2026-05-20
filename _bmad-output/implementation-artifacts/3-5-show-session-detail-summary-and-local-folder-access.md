# Story 3.5: Show Session Detail Summary and Local Folder Access

Status: done

## Story
As an operator, I want to view session detail summary and the local output folder path, so that I can troubleshoot and access results quickly after or during a session.

## Acceptance Criteria
1. Authenticated operator can request session detail by `session_id`.
2. Session detail shows customer name, order number, station, status, timer/start/end timestamps, end reason, station snapshot, local output folder, photo count placeholder, failures placeholder, quarantine count placeholder, and upload status placeholder.
3. Dashboard live cards expose a detail summary/action for active or locked sessions.
4. Local output folder is shown as local path text with operator-safe copy; no OS shell execution is required in this story.
5. Unknown session returns structured `SESSION_NOT_FOUND` error.
6. API remains local-auth protected and documented in OpenAPI.
7. Activity/SSE behavior is not required for read-only detail fetch, and detail fetch must not spam activity log.
8. Tests/build pass.

## Tasks / Subtasks
- [x] Add `GET /api/sessions/{session_id}`.
- [x] Add `SessionDetailResponse` with summary placeholder fields.
- [x] Add frontend session detail fetch/action in live cards.
- [x] Update OpenAPI.
- [x] Add tests and run validation.

## Dev Notes
This story is read-only. Do not launch Explorer or execute shell commands. Photo/failure/quarantine/upload counts stay placeholders until later epics.

## Dev Agent Record

### Debug Log References

- `cd apps/agent && go test ./...` -> passed.
- `cd apps/web && npm run typecheck && npm run build` -> passed.

### Completion Notes List

- Added read-only `GET /api/sessions/{session_id}` detail endpoint.
- Added local output folder and placeholder counts/upload status summary.
- Added frontend session detail query and live-card summary display.
- Updated OpenAPI and tests.

### File List

- `apps/agent/internal/sessions/store.go`
- `apps/agent/internal/api/sessions.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/sessions_test.go`
- `apps/web/src/lib/api/client.ts`
- `apps/web/src/features/sessions/use-session-detail-query.ts`
- `apps/web/src/features/sessions/live-station-cards.tsx`
- `docs/api/openapi.yaml`
