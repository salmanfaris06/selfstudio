# Story 4.2: Route Valid JPGs to Active Sessions

Status: done

## Story
As an operator, I want detected photos routed to the station active session, so that each customer receives correct photos.

## Acceptance Criteria
1. Given a station has an active session, when a stable JPG is detected before session lock, then the backend creates a persistent photo record tied to that session and station.
2. The photo record preserves at minimum `photo_id`, `station_id`, `session_id`, `source_path`, `source_size_bytes`, `detected_at`, `routed_at`, and routing/status value.
3. Routing uses server-owned Go session state only; the frontend must not decide whether a photo belongs to a session.
4. Duplicate detected JPG identities do not create duplicate routed photo records, including duplicate watcher/scan events in the same process lifetime.
5. Routing status appears in station/session detail through API/UI enough for operator troubleshooting.
6. SSE publishes a routing event such as `photo.routed` using the existing event wrapper and safe identifiers.
7. Activity log records safe routing entries with station/session references and without leaking unnecessary sensitive data.
8. OpenAPI documents new/changed photo routing fields/endpoints/responses.
9. Tests/build pass for Go agent and relevant web type/build checks.

## Tasks / Subtasks
- [x] Add routed photo domain model/store in Go.
  - [x] Implement an in-memory store consistent with current project state, because current session/station/activity stores are in-memory and no `photos` migration exists yet.
  - [x] Use a duplicate-safe identity derived from station ID + normalized source path + source size + detected timestamp or file modified timestamp when available.
  - [x] Return existing photo record on duplicate route attempts instead of creating another record.
- [x] Connect Story 4.1 stable detection to server-side routing.
  - [x] Keep stable detection in `apps/agent/internal/ingestion/scanner.go` responsible only for detecting files.
  - [x] Add routing logic in `internal/ingestion` or `internal/service`, not in API handler and not in frontend.
  - [x] Find active eligible session by `station_id` from `apps/agent/internal/sessions.Store` at route time.
  - [x] Treat `sessions.StatusActive` as the only eligible status for this story; locked/no-session quarantine is Story 4.3 and must not be silently assigned.
- [x] Update `POST /api/ingestion/scan` response to include routed photos.
  - [x] Preserve existing `photos` detected data and `errors` shape unless intentionally migrated with web updates.
  - [x] Add routed record list/status so scan UI/session detail can show routing result.
  - [x] Do not move/copy/delete source files in this story.
- [x] Add route visibility to session detail.
  - [x] Update `GET /api/sessions/{session_id}` summary/detail to include routed photo count and recent routed photos or a minimal `photos` list.
  - [x] Update frontend API types/runtime validation in `apps/web/src/lib/api/client.ts`.
  - [x] Update session/detail UI or existing dashboard panel to show routing status and source metadata safely.
- [x] Publish events and activity logs.
  - [x] Publish `photo.routed` with `entity_type: "photo"`, `entity_id: photo_id`, `event_type: "photo.routed"`, and `data` containing safe IDs/status.
  - [x] Record activity action `photo.routed` with `station_id` and `session_id` references.
  - [x] Avoid using raw `source_path` as SSE `entity_id`; this fixes/de-risks a Story 4.1 deferred review item.
- [x] Update contracts and tests.
  - [x] Update `docs/api/openapi.yaml` for scan routing response and session detail photo routing fields.
  - [x] Add Go tests for active-session routing, no frontend-owned routing, duplicate safety, locked session non-routing, SSE/activity side effects, and session detail photo visibility.
  - [x] Add/update web type checks for new response shape.

## Dev Notes

### Source Requirements
- Epic 4 goal: monitor input folders, wait for stable JPGs, prevent duplicate processing, route photos to active session, quarantine unassigned/late photos, and provide review/assign UI later.
- This story covers FR24 and supports FR22/FR30/FR43/FR44/FR48/FR54 foundations.
- Acceptance criteria from epics: active station session + stable JPG before lock creates photo record tied to session/station; preserves source path and detected timestamp; routing uses server-owned session state, not frontend state; routing status appears in station/session detail.

### Current Code Context To Read Before Editing
- `apps/agent/internal/ingestion/scanner.go`
  - Current state: scans all configured station input folders, accepts `.jpg/.jpeg`, ignores dirs/zero-byte/unstable files, and suppresses duplicate paths in process lifetime using `seen` map keyed by lowercased cleaned source path.
  - It returns `DetectedPhoto{station_id, source_path, size_bytes, detected_at, stable_at, status:"detected"}`.
  - Preserve: do not regress stable detection, case-insensitive JPG handling, duplicate reservation, per-station scan order, and no file mutation.
- `apps/agent/internal/api/ingestion.go`
  - Current state: `POST /api/ingestion/scan` calls scanner, records `photo.detected`, publishes `photo.detected`, and returns `{photos, errors}`.
  - Story change: after detection, route each detected photo to an active session server-side and include routing result. Move substantial routing rules out of handler.
  - Preserve: authenticated endpoint behavior, `{data}` wrapper, per-station `errors`, existing operator-safe API errors.
- `apps/agent/internal/sessions/store.go`
  - Current state: in-memory `Store` supports `Start`, `End`, `LockExpired`, `List`, `Get`; only `active` and `locked` statuses currently exist. Timer expiry is applied by `LockExpired`/`End` paths, not automatically on every read unless caller invokes it.
  - Story change: routing must consult this store and should call/use a method that respects expiration boundaries before assigning. If no helper exists, add a store method such as `ActiveForStation(stationID, now)` that locks expired sessions before returning an active session.
  - Preserve: one active session per station, locked sessions never receive photos, manual/timer end semantics.
- `apps/agent/internal/api/sessions.go` and `apps/agent/internal/sessions/persistence.go`
  - Current state: session list/detail and summary exist; summary currently has placeholder counts.
  - Story change: detail/summary should surface routed photo status/count. Keep output folder/local/cloud placeholders compatible with Epic 5/6 later.
- `apps/web/src/lib/api/client.ts`
  - Current state: has `DetectedPhoto`, `IngestionScanData`, `SessionDetailData`, runtime validation, and `runIngestionScan()`.
  - Story change: update types/validators for routed photos and session detail visibility. Keep `snake_case` API fields.
- `docs/api/openapi.yaml`
  - Current state: documents ingestion scan endpoint and response schemas.
  - Story change: document routed photo schema, routing status, and session detail additions.

### Architecture Guardrails
- Go service owns all filesystem access, worker logic, session/photo state, and routing decisions.
- Next.js/browser must never read camera folders, decide active session eligibility, or hold service credentials.
- API responses must use `{data}` wrapper; errors must use `{error:{code,message,action,details}}`.
- DB/API JSON/status fields use `snake_case`.
- SSE event names use dot notation and event wrapper with `event_id`, `event_type`, `occurred_at`, and `data`.
- Operator-facing errors must be actionable and safe; technical detail belongs in local logs/tests.
- Runtime files stay under `local-data`; source JPGs must not be moved/copied/deleted in this story.

### Recommended Implementation Shape
- Add `apps/agent/internal/ingestion/router.go` or `apps/agent/internal/service/photo_router.go`.
- Add `apps/agent/internal/ingestion/photo_store.go` or `apps/agent/internal/photos/store.go` for routed photo records. A dedicated `internal/photos` package is cleaner if future Epic 5 processing will expand photo state.
- Suggested statuses for this story: `detected` for scanner output and `routed` for successful session assignment. Reserve `quarantined` for Story 4.3.
- Suggested route result shape:
  - `photo_id`
  - `station_id`
  - `session_id`
  - `source_path`
  - `source_size_bytes`
  - `detected_at`
  - `stable_at`
  - `routed_at`
  - `status: "routed"`
  - `duplicate: boolean` if useful for scan diagnostics.
- For no active/locked session in this story, return a non-routed result such as `status:"unassigned_pending_quarantine"` or omit routed photo and include a safe station-level routing message. Do not quarantine yet unless Story 4.3 is also explicitly implemented later.
- Ensure timer boundary determinism: routing time should be `time.Now().UTC()` in Go, and session eligibility should compare against server `EndsAt`; do not trust browser time.

### Previous Story Intelligence (4.1)
- 4.1 added scanner, scan endpoint, dashboard scan panel, OpenAPI update, SSE/activity on stable detections, and duplicate suppression.
- Review patches already fixed missing OpenAPI, concurrent duplicate reservation, swallowed station scan errors, frontend response validation, SSE invalidation, and scan panel error count.
- Deferred from 4.1 and relevant here: avoid raw source path in SSE `entity_id`; use generated photo IDs for persistent model in this story.
- Deferred but not required unless easy: stronger JPEG header validation and multi-observation stability.

### Testing Requirements
- Go unit/API tests must cover:
  - Stable detected photo routes to current active session for same station.
  - Active session on another station is not used.
  - Locked/expired session is not assigned.
  - Duplicate scan/event does not create duplicate routed photo record.
  - Scan response includes routing data and still includes per-station errors.
  - `photo.routed` SSE and `photo.routed` activity entries are emitted on successful route.
  - Session detail/summary exposes routed photo count/list.
- Run at minimum:
  - `cd apps/agent && go test ./...`
  - `cd apps/web && npm run typecheck`
  - `cd apps/web && npm run build` if dependencies/environment allow.

### Regression Risks To Avoid
- Do not break Story 4.1 scan semantics or tests.
- Do not create frontend session-routing logic.
- Do not assign photos to locked/expired sessions because timer/admin boundary cases are core NFR10.
- Do not use raw filesystem paths as durable public IDs or SSE entity IDs.
- Do not introduce Supabase/service credential use in frontend.
- Do not implement product-code behavior for quarantine assignment/review yet; that belongs to Stories 4.3 and 4.4.

## Project Context Reference
- Planning artifacts: `_bmad-output/planning-artifacts/epics.md`, `prd.md`, `architecture.md`.
- Previous story: `_bmad-output/implementation-artifacts/4-1-watch-station-input-folders-for-stable-jpgs.md`.
- Current project is a Windows-local Next.js + Go monorepo with Go agent authoritative for filesystem/session/photo state.
- Current git commits are sparse (`Add from-scratch setup guide`, `Initial Selfstudio camera capture spike`), and most implementation artifacts/code are uncommitted in the working tree; rely on files in the workspace, not git history alone.

### Review Follow-ups (AI)
- [x] [HIGH][Review][Patch] Duplicate identity can create a second routed photo when the same file is detected in a later scan/process with a different detected timestamp. Evidence: `photos.Store.Route` builds duplicate identity from `stationID + normalized source path + source size + detectedAt` (`apps/agent/internal/photos/store.go:42-50`, `apps/agent/internal/photos/store.go:88-90`), and `Router.Route` passes scanner `DetectedAt` into that identity (`apps/agent/internal/ingestion/router.go:44`). The story requires duplicate detected JPG identities not to create duplicate routed photo records, including duplicate watcher/scan events. For the same stable JPG, a later detector invocation can produce a new `detected_at`, changing the identity despite identical station/path/size. Use a stable file identity component such as normalized source path + size + file modified/stable timestamp when available, or otherwise avoid using per-detection time as the duplicate discriminator.

## Dev Agent Record

### Debug Log References
- 2026-05-19: `cd apps/agent && go test ./...` — pass.
- 2026-05-19: `cd apps/agent && go test ./... && cd ../web && npm run typecheck && npm run build` — pass.
- 2026-05-19: `cd apps/agent && go test ./...` — initially failed during RED phase for duplicate detectedAt router regression test, then passed after patch; one Windows `fork/exec ... api.test.exe: Access is denied` retry was resolved with `go clean -testcache`.
- 2026-05-19: `cd apps/web && npm run typecheck` — pass.

### Completion Notes List
- ✅ Resolved review finding [HIGH]: duplicate routed photo identity no longer uses per-detection `detected_at`; identity is based on station ID + normalized source path + source size so the same file with a different `detected_at` returns the existing routed photo.
- Menambahkan regression tests di photo store dan ingestion router untuk membuktikan file yang sama dengan `detected_at` berbeda tidak membuat routed photo kedua.
- Menambahkan `internal/photos` in-memory routed photo store dengan ID stabil `photo_*`, identity duplicate-safe, dan pengembalian record existing untuk duplicate route attempts.
- Menambahkan server-side ingestion router yang mengambil session aktif via Go session store (`ActiveForStation`) dan tidak meroute session locked/expired/no-session.
- Memperluas `POST /api/ingestion/scan` dengan `routed_photos` tanpa mengubah shape `photos`/`errors` dan tanpa memindah/menyalin/menghapus source JPG.
- Memperluas session detail dengan `summary.photo_count` aktual dan daftar `photos` terbaru untuk troubleshooting operator.
- Menambahkan event/activity `photo.routed` dengan safe `photo_id` sebagai SSE entity ID; event `photo.detected` juga tidak lagi memakai raw source path sebagai entity ID.
- Memperbarui OpenAPI, frontend API types/runtime validators, dan live station UI untuk status routed photo.

### File List
- apps/agent/cmd/selfstudio-agent/main.go
- apps/agent/internal/api/ingestion.go
- apps/agent/internal/api/photo_routing_test.go
- apps/agent/internal/api/sessions.go
- apps/agent/internal/ingestion/router.go
- apps/agent/internal/ingestion/router_test.go
- apps/agent/internal/photos/store.go
- apps/agent/internal/photos/store_test.go
- apps/agent/internal/sessions/store.go
- apps/web/src/features/sessions/live-station-cards.tsx
- apps/web/src/lib/api/client.ts
- docs/api/openapi.yaml
- _bmad-output/implementation-artifacts/4-2-route-valid-jpgs-to-active-sessions.md
- _bmad-output/implementation-artifacts/sprint-status.yaml

## Change Log
- 2026-05-19: Ultimate context engine analysis completed - comprehensive developer guide created.
- 2026-05-19: Implemented server-owned routed photo model/store, ingestion routing, API/UI visibility, SSE/activity logging, OpenAPI updates, and test coverage.
- 2026-05-19: Addressed code review finding - 1 HIGH duplicate routed photo identity issue resolved.
