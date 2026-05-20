# Story 4.3: Quarantine Unassigned and Late Photos

Status: done

## Story

As an operator, I want photos with no eligible session quarantined with reason, so that wrong customer routing is prevented.

## Acceptance Criteria

1. Given a stable JPG is detected for a station with no active eligible session, when routing is evaluated, then the backend records the photo as quarantined/unassigned and does not assign it to any session.
2. Given a stable JPG is detected after the station session is locked or timer-expired, when routing is evaluated, then the backend records the photo as quarantined/unassigned with a late-photo reason and does not assign it to the locked session.
3. Each quarantined photo record preserves traceability: `quarantine_id`, optional `photo_id` or stable source identity, `station_id`, `source_path`, `source_size_bytes`, `detected_at`, `stable_at`, `quarantined_at`, `reason`, `status`, and an optional related/last session reference when determinable.
4. Quarantine reasons are explicit, operator-readable categories such as `no_active_session` and `late_photo`; do not use ambiguous free text as the durable reason value.
5. Duplicate detections for the same source identity do not create duplicate quarantine records, including duplicate watcher/scan events or repeated scan calls.
6. Quarantined photos are never silently assigned to a session, never moved into session output folders, and never enter Epic 5 processing queues in this story.
7. Operator alert/status surfaces quarantine count and reason category in station/session detail or dashboard summary enough for event troubleshooting.
8. SSE publishes quarantine events such as `photo.quarantined` or `quarantine.created` with safe IDs and the existing event wrapper; activity log records safe quarantine entries with station/session references where available.
9. OpenAPI documents new/changed quarantine fields, scan response shape, and any detail/summary additions.
10. Tests/build pass for Go agent and relevant web type/build checks.

## Tasks / Subtasks

- [x] Add quarantine domain model/store in Go. (AC: 1, 2, 3, 4, 5)
  - [x] Create an in-memory store consistent with current project state; do not introduce a partial Supabase migration unless the broader persistence layer is explicitly in scope.
  - [x] Use a duplicate-safe identity based on `station_id + normalized source_path + source_size_bytes`, matching Story 4.2 routed-photo duplicate behavior.
  - [x] Return the existing quarantine record on duplicate quarantine attempts instead of creating another record.
  - [x] Define durable statuses/reasons with `snake_case`: suggested `status: "quarantined"`, reasons `no_active_session` and `late_photo`.
- [x] Extend server-side ingestion routing to quarantine non-routable stable JPGs. (AC: 1, 2, 4, 6)
  - [x] Keep `apps/agent/internal/ingestion/scanner.go` focused on detection/stability only.
  - [x] Extend `apps/agent/internal/ingestion/router.go` or add a quarantine service called from it; do not put business rules in API handlers or frontend.
  - [x] If `sessions.ActiveForStation(station_id, now)` returns no eligible active session, create a quarantine record instead of returning only `unassigned_pending_quarantine`.
  - [x] Determine reason using server-owned session state: no active/known session => `no_active_session`; locked/timer-expired or last station session ended before detection/routing boundary => `late_photo`.
  - [x] Do not move, copy, delete, or process source JPGs in this story; quarantine is metadata-only unless existing project conventions already provide a safe quarantine folder move API (none currently exists).
- [x] Update ingestion scan API response. (AC: 1, 3, 7, 9)
  - [x] Preserve existing `{data:{photos,routed_photos,errors}}` semantics for Story 4.1/4.2 compatibility.
  - [x] Add `quarantined_photos` or enrich non-routed route results with quarantine fields; choose the clearer OpenAPI-documented shape and update web runtime validation.
  - [x] Keep API fields `snake_case` and response wrapped in `{data}`.
- [x] Surface quarantine status to operators. (AC: 7)
  - [x] Update session detail summary `quarantine_count` from real quarantine data, not the current hard-coded `0`.
  - [x] Update live station cards or existing session/detail UI to show quarantine count and latest reason category with text labels, not color only.
  - [x] Show no-active-session quarantine in station-level context when there is no active session; do not require a session detail panel that cannot be opened.
- [x] Publish events and activity logs. (AC: 8)
  - [x] Publish `photo.quarantined` or `quarantine.created` with `entity_type: "quarantine"`, safe `quarantine_id` as `entity_id`, and `data` containing safe IDs/status/reason.
  - [x] Record activity action with station reference and related session reference when known; do not leak unnecessary sensitive path data in activity message.
  - [x] Do not use raw `source_path` as SSE `entity_id`.
- [x] Update OpenAPI and frontend contract. (AC: 7, 9)
  - [x] Update `docs/api/openapi.yaml` for quarantine schemas and scan/session/station summary additions.
  - [x] Update `apps/web/src/lib/api/client.ts` types and runtime validators for quarantine response data.
  - [x] Update SSE invalidation if current event client filters by event type.
- [x] Add focused tests. (AC: 1, 2, 5, 6, 8, 10)
  - [x] Go tests for no-active-session quarantine.
  - [x] Go tests for locked/timer-expired late-photo quarantine and no assignment to locked session.
  - [x] Go tests for duplicate quarantine identity returning one record.
  - [x] API tests for scan response containing quarantine data while preserving photos/routed/errors.
  - [x] Event/activity tests for quarantine side effects with safe IDs.
  - [x] Web type/runtime validation tests or typecheck coverage for new response shape.

### Review Follow-ups (AI)

- [x] [HIGH] Active station cards do not refresh quarantine/photo summary after `photo.quarantined` / `photo.routed` events — `apps/web/src/features/health/health-dashboard.tsx` only invalidates `sessionsQueryKey` and activity for photo events, but `LiveStationCards` reads counts/reasons from per-session detail queries keyed as `["sessions", sessionId]`. React Query exact invalidation of `["sessions"]` will not refresh those detail queries, so AC7 operator alert/status can remain stale after quarantine/routing until manual refresh or remount. Fix should invalidate affected session-detail queries or use non-exact/predicate invalidation for session detail data on photo events.
- [x] [MEDIUM] No-active-session quarantine is not surfaced from real station-level quarantine data — `apps/web/src/features/sessions/live-station-cards.tsx` renders only static explanatory text when there is no active session, and no endpoint/query provides station quarantine counts for inactive stations. This does not satisfy AC7's requirement to show no-active-session quarantine in station-level context without requiring a session detail panel. Add a real station/dashboard summary source and render count/latest reason for inactive stations.

## Dev Notes

### Source Requirements

- Epic 4 goal: monitor input folders, wait for stable JPGs, prevent duplicate processing, route photos to active session, quarantine unassigned/late photos, and later provide review/assign UI.
- This story implements FR25, FR26, FR27 and supports FR30, FR43, FR44, FR48, FR54.
- PRD/NFR guardrails: timer/admin end and photo-ingestion boundary cases must resolve deterministically; each photo must maintain traceability; dashboard must surface actionable alert states with text labels; duplicate filesystem events must not create duplicate processing.
- Acceptance criteria from epics: stable JPG with no active session or after lock is moved/recorded as quarantined/unassigned; quarantine reason is stored; operator alert shows quarantine count and reason category; quarantined photos are never silently assigned.

### Current Code Context To Read Before Editing

- `apps/agent/internal/ingestion/scanner.go`
  - Current state: scans configured station input folders, accepts `.jpg/.jpeg`, ignores dirs/zero-byte/unstable files, waits 500ms, suppresses duplicate paths in process lifetime, and returns `DetectedPhoto{station_id, source_path, size_bytes, detected_at, stable_at, status:"detected"}`.
  - Preserve: stable detection behavior, case-insensitive JPG handling, duplicate reservation, per-station scanning, per-station `errors`, and no file mutation.
- `apps/agent/internal/ingestion/router.go`
  - Current state: `Router.Route` asks `sessions.ActiveForStation(station_id, now)`; if active, it creates a routed photo; otherwise it returns status `unassigned_pending_quarantine` without persistence.
  - Story change: replace/extend `unassigned_pending_quarantine` with durable quarantine records and reason classification.
  - Preserve: server-owned routing only; frontend must not decide active session eligibility.
- `apps/agent/internal/photos/store.go`
  - Current state: in-memory routed-photo store with stable `PhotoID`, `StatusRouted`, identity based on station ID + normalized source path + source size, `ListBySession`, `CountBySession`, and duplicate return behavior.
  - Story change: reuse the identity pattern for quarantine. Do not regress the Story 4.2 duplicate fix by reintroducing per-detection timestamps into identity.
- `apps/agent/internal/sessions/store.go`
  - Current state: in-memory session store supports `active` and `locked`; `ActiveForStation` locks expired sessions using server time before returning an eligible active session.
  - Story change: quarantine reason needs enough context to distinguish `no_active_session` vs `late_photo`. Add a server-side helper if needed, e.g. `LastSessionForStation(stationID)` or a method returning locked/expired boundary context.
  - Preserve: one active session per station, locked sessions never receive photos, timer expiry based on server timestamps.
- `apps/agent/internal/api/ingestion.go`
  - Current state: `POST /api/ingestion/scan` records/publishes `photo.detected`, calls router, returns `routed_photos`, and records/publishes `photo.routed` for successful routes.
  - Story change: include quarantine results and publish/activity quarantine side effects. Keep handler thin; substantial rules belong to service/router/store.
- `apps/agent/internal/api/sessions.go`
  - Current state: `SessionSummary.QuarantineCount` is hard-coded to `0`; `PhotoCount` comes from `photos.Store` when available.
  - Story change: inject/use quarantine store to compute real `quarantine_count` for session-related late photos and provide station-level count where appropriate.
- `apps/web/src/lib/api/client.ts`
  - Current state: contains `RouteResult` union including `unassigned_pending_quarantine`, `IngestionScanData`, `SessionDetailData`, validators, and API functions.
  - Story change: update types/validators for quarantine records and summary counts. Keep API JSON field names `snake_case`.
- `apps/web/src/features/sessions/live-station-cards.tsx`
  - Current state: shows active session details, routed photo count, summary quarantine count (currently always 0), and text labels.
  - Story change: display real quarantine count/latest reason category without relying on color only; handle no-active-session quarantine visibility.
- `docs/api/openapi.yaml`
  - Current state: documents ingestion scan and routed photo/session detail shapes from Stories 4.1/4.2.
  - Story change: document quarantine schema and response additions.

### Architecture Guardrails

- Go service owns all filesystem, worker, session/photo/quarantine state, and routing decisions.
- Next.js/browser must never read camera folders, classify routing eligibility, or hold Supabase/service/cloud credentials.
- API responses must use `{data}` wrapper; errors must use `{error:{code,message,action,details}}`.
- DB/API JSON/status fields use `snake_case`; SSE names use dot notation.
- SSE payload wrapper must include `event_id`, `event_type`, `occurred_at`, and `data`.
- Operator-facing errors/messages must be actionable and safe; technical details belong in Go logs/tests.
- Runtime photo files remain under `local-data`; do not put photo assets in `apps/web/public`.
- Do not implement quarantine review/manual assignment in this story; that is Story 4.4. This story only records and surfaces quarantine/unassigned/late state.

### Recommended Implementation Shape

- Add `apps/agent/internal/quarantine/store.go` for quarantine records rather than mixing quarantine into `internal/photos`.
- Suggested record:
  - `quarantine_id`
  - `station_id`
  - `related_session_id` nullable/empty
  - `source_path`
  - `source_size_bytes`
  - `detected_at`
  - `stable_at`
  - `quarantined_at`
  - `reason: "no_active_session" | "late_photo"`
  - `status: "quarantined"`
  - `duplicate: boolean` for diagnostics only
- Suggested store methods:
  - `Quarantine(stationID, relatedSessionID, sourcePath, sourceSizeBytes, detectedAt, stableAt, quarantinedAt, reason) Record`
  - `CountByStation(stationID) int`
  - `CountByRelatedSession(sessionID) int`
  - `ListByStation(stationID, limit int) []Record`
  - `ListByRelatedSession(sessionID, limit int) []Record`
- Reason classification should be deterministic and server-time based:
  - If a station has a locked session whose `ended_at`/`ends_at` boundary is before or equal to detection/routing time and it is the most recent session for that station, use `late_photo` with `related_session_id`.
  - If there is no station session context, use `no_active_session`.
  - If implementation cannot safely infer related session, prefer `no_active_session` over guessing; never assign silently.
- API shape recommendation:
  - Keep `routed_photos` for successful routes.
  - Add `quarantined_photos` for durable quarantine records.
  - Optionally keep route result `unassigned_pending_quarantine` only as backward-compatible diagnostic, but do not make it the only persisted state.

### Previous Story Intelligence (4.2)

- 4.2 introduced `internal/photos` in-memory store, server-side ingestion router, scan response `routed_photos`, session detail photo visibility, `photo.routed` events/activity, OpenAPI updates, and frontend validators.
- Critical review fix from 4.2: duplicate routed-photo identity must not include per-detection `detected_at`; current identity is station ID + normalized source path + source size. Apply the same anti-regression rule to quarantine records.
- 4.2 intentionally did not quarantine locked/no-session photos; it returned `unassigned_pending_quarantine`. Story 4.3 must now make that durable and visible.
- Current git history is sparse; most useful implementation intelligence is in workspace files and story artifacts, not commits.

### Testing Requirements

Run at minimum:

- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build` if dependencies/environment allow.

Required coverage:

- No active session creates one quarantine record with reason `no_active_session`.
- Locked session creates quarantine with reason `late_photo` and related session reference when determinable.
- Timer-expired active session is locked before routing/quarantine decision and does not receive the photo.
- Duplicate source identity returns the existing quarantine record and does not increment counts.
- Routed photos still route normally to active sessions after quarantine additions.
- Scan response preserves `photos`, `routed_photos`, and `errors`, and includes quarantine data.
- SSE/activity use safe IDs and include reason category.
- Session/station UI shows quarantine count/reason text.

### Regression Risks To Avoid

- Do not break Story 4.1 scan semantics, duplicate suppression, or per-station scan errors.
- Do not break Story 4.2 active-session routing or routed photo visibility.
- Do not assign late/no-session photos to a locked or unrelated session.
- Do not use raw filesystem paths as durable public IDs or SSE entity IDs.
- Do not start Epic 5 processing for quarantined photos.
- Do not implement Story 4.4 manual assignment/review workflow beyond minimal visibility required by this story.
- Do not introduce frontend-owned workflow state machines.

## Project Structure Notes

- Expected new Go package: `apps/agent/internal/quarantine`.
- Expected modified Go files: `apps/agent/internal/ingestion/router.go`, `apps/agent/internal/api/ingestion.go`, `apps/agent/internal/api/sessions.go`, potentially `apps/agent/cmd/selfstudio-agent/main.go` for wiring and `apps/agent/internal/sessions/store.go` for session boundary helper.
- Expected modified web files: `apps/web/src/lib/api/client.ts`, `apps/web/src/features/sessions/live-station-cards.tsx`, and possibly query/event invalidation hooks if quarantine event names need cache refresh.
- Expected contract file: `docs/api/openapi.yaml`.
- Keep tests co-located as Go `*_test.go`; keep frontend changes type-safe through existing TypeScript API client validators.

## References

- `_bmad-output/planning-artifacts/epics.md` — Epic 4 and Story 4.3 acceptance criteria; FR25-FR27 mapping.
- `_bmad-output/planning-artifacts/prd.md` — Journey 2 late photo quarantine; Functional Requirements Photo Ingestion and Routing; NFR10, NFR14, NFR29, NFR31.
- `_bmad-output/planning-artifacts/architecture.md` — Go service ownership, REST/SSE contracts, naming/status patterns, retry/idempotency, project boundaries.
- `_bmad-output/implementation-artifacts/4-2-route-valid-jpgs-to-active-sessions.md` — previous story implementation learnings and duplicate identity review fix.
- `apps/agent/internal/ingestion/router.go` — current non-routed result placeholder to replace with durable quarantine.
- `apps/agent/internal/photos/store.go` — existing duplicate-safe identity pattern to reuse.
- `apps/agent/internal/api/sessions.go` — current hard-coded `quarantine_count: 0` to make real.

## Dev Agent Record

### Agent Model Used

{{agent_model_name_version}}

### Debug Log References

- 2026-05-19: `cd apps/agent && go test ./...` — pass.
- 2026-05-19: `cd apps/web && npm run typecheck` — pass.
- 2026-05-19: `cd apps/web && npm run build` — pass.
- 2026-05-19: `cd apps/agent && go test ./...` — pass after review follow-up fixes.
- 2026-05-19: `cd apps/web && npm run typecheck` — pass after review follow-up fixes.
- 2026-05-19: `cd apps/web && npm run build` — pass after review follow-up fixes.

### Completion Notes List

- Implemented in-memory quarantine store with duplicate-safe identity, durable `quarantined` status, and `no_active_session` / `late_photo` reasons.
- Extended ingestion router to create metadata-only quarantine records for no active eligible session and expired/locked late-photo cases, without moving/copying/deleting JPG files.
- Added `quarantined_photos` to scan response while preserving `photos`, `routed_photos`, and `errors` under `{data}`.
- Wired quarantine store through agent main, ingestion API, and session detail summary; session cards now show quarantine count/reason text and no-active-session context.
- Added `photo.quarantined` SSE/activity side effects using safe `quarantine_id` entity IDs and no raw path as SSE entity ID.
- Updated OpenAPI and web API runtime validation/types for quarantine records and summary additions.
- Added Go tests for no-active-session quarantine, late-photo quarantine, duplicate quarantine identity, scan response shape, and event/activity side effects; web contract is covered by typecheck/build.
- ✅ Resolved review finding [HIGH]: Active station cards now invalidate session detail queries and station quarantine summaries on `photo.quarantined` / `photo.routed` events, preventing stale quarantine/photo counts.
- ✅ Resolved review finding [MEDIUM]: Added a real station-level quarantine summary endpoint/client query and rendered count/latest reason for inactive station cards, including no-active-session quarantine context.

### File List

- apps/agent/cmd/selfstudio-agent/main.go
- apps/agent/internal/api/health.go
- apps/agent/internal/api/ingestion.go
- apps/agent/internal/api/photo_routing_test.go
- apps/agent/internal/api/quarantine_test.go
- apps/agent/internal/api/sessions.go
- apps/agent/internal/ingestion/router.go
- apps/agent/internal/ingestion/router_test.go
- apps/agent/internal/quarantine/store.go
- apps/agent/internal/quarantine/store_test.go
- apps/agent/internal/sessions/store.go
- apps/web/src/features/health/health-dashboard.tsx
- apps/web/src/features/sessions/live-station-cards.tsx
- apps/web/src/features/sessions/use-station-quarantine-summary-query.ts
- apps/web/src/lib/api/client.ts
- docs/api/openapi.yaml
- _bmad-output/implementation-artifacts/4-3-quarantine-unassigned-and-late-photos.md
- _bmad-output/implementation-artifacts/sprint-status.yaml

## Change Log

- 2026-05-19: Ultimate context engine analysis completed - comprehensive developer guide created.
- 2026-05-19: Implemented quarantine store/routing/API/UI/events/contracts/tests; story moved to review.
- 2026-05-19: Addressed code review findings - 2 items resolved; story remains ready for review.
