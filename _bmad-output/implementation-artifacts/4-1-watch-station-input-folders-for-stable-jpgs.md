# Story 4.1: Watch Station Input Folders for Stable JPGs

Status: done

## Story
As the system, I want to watch each station input folder for stable JPG files, so that new event photos can be detected safely before routing.

## Acceptance Criteria
1. Backend exposes authenticated ingestion scan/run endpoint that checks all configured station input folders.
2. The scan detects `.jpg` and `.jpeg` case-insensitively and waits for file stability before accepting a photo.
3. The scan ignores directories, zero-byte files, non-JPG files, and unstable files.
4. Duplicate file paths are not detected twice in one process lifetime.
5. Detection preserves source file and returns station id, source path, size, detected/stable timestamps, and status.
6. SSE publishes `photo.detected` for detected stable JPGs and activity records safe `photo.detected` entries.
7. OpenAPI documents scan endpoint and response schema.
8. Tests/build pass.

## Tasks / Subtasks
- [x] Add ingestion domain scanner/store.
- [x] Add `POST /api/ingestion/scan`.
- [x] Add SSE/activity on stable detections.
- [x] Add frontend basic scan action/status.
- [x] Update OpenAPI.
- [x] Add Go tests and run validation.

## Dev Notes
This story detects only; routing to sessions happens in Story 4.2. Do not move/copy/delete source files.

### Review Findings

- [x] [Review][Patch] OpenAPI missing ingestion scan endpoint/schema/errors.
- [x] [Review][Patch] Concurrent scans could detect same file twice; reserve identity before stability wait and release on invalid/unstable.
- [x] [Review][Patch] Station scan errors were swallowed; response now includes per-station `errors`.
- [x] [Review][Patch] Frontend did not validate ingestion scan response; added runtime validation.
- [x] [Review][Patch] Dashboard did not react to `photo.detected`; added SSE invalidation for activity/sessions.
- [x] [Review][Patch] Scan panel did not invalidate activity or show station error count; patched.
- [x] [Review][Defer] Strong JPEG header validation and multi-observation stability; deferred to ingestion hardening.
- [x] [Review][Defer] Avoid raw source path in SSE entity/data with generated photo IDs; deferred to persistent photo model in Story 4.2/4.3.

## Dev Agent Record

### Debug Log References

- `cd apps/agent && go test ./...` -> passed.
- `cd apps/web && npm run typecheck && npm run build` -> passed.

### Completion Notes List

- Applied Story 4.1 review patches: OpenAPI, atomic duplicate reservation, per-station scan errors, frontend validation, `photo.detected` SSE invalidation, activity query invalidation.
- Validation after review patches passed: `cd apps/agent && go test ./...`; `cd apps/web && npm run typecheck && npm run build`.
- Added ingestion scanner for stable JPG detection across station input folders.
- Added `POST /api/ingestion/scan`, activity `photo.detected`, and SSE `photo.detected`.
- Added dashboard scan panel.
- Added duplicate suppression in scanner process lifetime.

### File List

- `apps/agent/internal/ingestion/scanner.go`
- `apps/agent/internal/api/ingestion.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/ingestion_test.go`
- `apps/web/src/lib/api/client.ts`
- `apps/web/src/features/ingestion/ingestion-scan-panel.tsx`
- `apps/web/src/features/health/health-dashboard.tsx`
- `docs/api/openapi.yaml`
