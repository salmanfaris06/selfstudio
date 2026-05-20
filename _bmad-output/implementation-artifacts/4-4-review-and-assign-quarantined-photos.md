# Story 4.4: Review and Assign Quarantined Photos

Status: done

## Story

Sebagai operator, saya ingin meninjau foto yang dikarantina dan meng-assign secara manual ke session yang eligible, sehingga foto yang masih bisa diselamatkan dapat dikirim ke session/customer yang benar tanpa salah routing otomatis.

## Acceptance Criteria

1. Given quarantined photos exist, when operator opens quarantine view, then UI shows each item with safe preview/path metadata, station, detected time, quarantined time, reason, status, and related/last session reference when available.
2. Given a quarantined photo is still `status: "quarantined"`, when operator selects an eligible target session and confirms assignment, then the Go service creates/returns a routed photo/session assignment trace and changes the quarantine item to an assigned/resolved state.
3. Eligible target sessions are determined server-side only: same station is the default/safest eligibility rule; locked sessions may be eligible for late-photo recovery only through explicit operator confirmation; active sessions for another station are not eligible unless a deliberate cross-station recovery rule is added and tested in the backend.
4. Assignment preserves full traceability from quarantine source to final session: `quarantine_id`, resulting `photo_id` or assignment id, `station_id`, `source_path`, `source_size_bytes`, `detected_at`, `stable_at`, `quarantined_at`, `reason`, `assigned_at`, `assigned_session_id`, and assignment actor/action context in activity log.
5. Duplicate assignment is blocked/idempotent: repeated assign calls for the same `quarantine_id` must not create duplicate routed photos, duplicate processing jobs, or conflicting assigned session state.
6. Manual assignment is logged with safe operator-readable message and station/session refs; logs must not expose unnecessary sensitive path data.
7. SSE publishes a safe event such as `quarantine.assigned` or `photo.routed` using safe entity ids and the existing event wrapper so live cards/session detail/quarantine view refresh without manual reload.
8. UI requires explicit confirmation before assignment, uses text labels in addition to color, shows actionable errors, and keeps the quarantine list/session counts current after assignment.
9. OpenAPI documents quarantine list, eligible sessions, assignment request/response, new quarantine status values, and related summary changes.
10. Tests/build pass for Go agent and relevant web type/build checks.

## Tasks / Subtasks

- [x] Extend quarantine domain/store for review + assignment lifecycle. (AC: 2, 4, 5)
  - [x] Keep existing `apps/agent/internal/quarantine.Store` duplicate identity behavior from Story 4.3 intact.
  - [x] Add durable status values with `snake_case`, e.g. `quarantined` and `assigned`; do not remove existing `quarantined` semantics.
  - [x] Add assignment fields to quarantine record or assignment result: `assigned_session_id`, `assigned_photo_id` or `photo_id`, `assigned_at`.
  - [x] Add lookup/update methods such as `List(status, station_id, limit)`, `Get(quarantine_id)`, and `Assign(quarantine_id, session_id, photo_id, assigned_at)`.
  - [x] Make `Assign` idempotent: if already assigned to the same session, return existing assigned result; if assigned to a different session, return an operator-actionable conflict.
- [x] Add server-side eligible session and assignment service/API. (AC: 2, 3, 4, 5, 9)
  - [x] Implement all business rules in Go service/router layer, not API handlers and not frontend.
  - [x] Add list endpoint for quarantine review, recommended `GET /api/quarantine?status=quarantined&station_id=...`.
  - [x] Add eligible sessions endpoint or include eligible targets in list/detail response, recommended `GET /api/quarantine/{quarantine_id}/eligible-sessions`.
  - [x] Add assignment endpoint, recommended `POST /api/quarantine/{quarantine_id}/assign` with body `{ "session_id": "..." }`.
  - [x] Validate target session exists and is eligible using `sessions.Store`; frontend must not decide eligibility.
  - [x] Use existing `photos.Store.Route(...)` or a dedicated assignment-aware wrapper to create/return one routed photo for the quarantine source identity.
  - [x] Prevent assignment if item is not found, not currently assignable, target session is invalid/ineligible, or identity conflicts with an existing photo for another session.
- [x] Build quarantine review UI. (AC: 1, 8)
  - [x] Add a feature under `apps/web/src/features/quarantine/` rather than mixing complex review UI into `live-station-cards.tsx`.
  - [x] Show list/table/cards of quarantine items with reason text (`no_active_session`, `late_photo`), station, safe source metadata, detected/quarantined timestamps, and current status.
  - [x] Provide session picker from backend-provided eligible sessions or backend-validated session list.
  - [x] Require confirmation before assignment; confirmation text must clearly identify target session/customer/order and warn that assignment is manual recovery.
  - [x] Show loading/error/success states with operator-actionable text; do not rely on color only.
  - [x] After assignment, invalidate quarantine list, sessions list/detail, station quarantine summary, activity log, and affected live station card data.
- [x] Wire events and activity logging. (AC: 6, 7, 8)
  - [x] Record activity action such as `quarantine.assigned` with station and assigned session refs.
  - [x] Avoid raw `source_path` in activity message and SSE entity id; use `quarantine_id`/`photo_id` safe IDs.
  - [x] Publish SSE using wrapper with `event_id`, `event_type`, `occurred_at`, and `data`.
  - [x] Update web SSE invalidation so `quarantine.assigned`, `photo.routed`, and existing `photo.quarantined` refresh affected queries.
- [x] Update API contract and frontend validators. (AC: 1, 2, 4, 9)
  - [x] Update `docs/api/openapi.yaml` with schemas for `QuarantineItem`, `EligibleSession`, `AssignQuarantineRequest`, and assignment response.
  - [x] Update `apps/web/src/lib/api/client.ts` types/runtime guards for list, eligible sessions, and assignment response.
  - [x] Preserve `{data}` success wrapper and `{error:{code,message,action,details}}` error shape.
  - [x] Keep all API JSON/status fields `snake_case`.
- [x] Add focused tests. (AC: 2, 3, 4, 5, 6, 7, 10)
  - [x] Go tests for listing quarantined items and hiding/marking assigned ones according to query/status.
  - [x] Go tests for same-station eligible session assignment.
  - [x] Go tests for late-photo assignment to related locked session with explicit assign endpoint.
  - [x] Go tests for ineligible target session rejection and duplicate/different-session conflict.
  - [x] Go tests that duplicate assignment does not create duplicate photo records.
  - [x] API tests for assignment response, operator-actionable errors, activity log, and SSE safe IDs.
  - [x] Web type/runtime validation coverage through typecheck/build and any existing component test pattern if available.

### Review Follow-ups (AI)

- [x] [HIGH] Locked-session eligibility is too broad for non-late quarantines — `apps/agent/internal/api/quarantine.go` validates assignment with same-station only and `eligibleSessions` lists every same-station session, so a `no_active_session` quarantine can be manually assigned to an arbitrary locked same-station session. AC3 only allows locked-session recovery for late-photo recovery through explicit operator confirmation; tighten backend eligibility so locked targets are accepted only for valid late-photo/related recovery (or add a deliberately tested broader backend rule), and add API tests that `no_active_session` → locked session is rejected while related locked `late_photo` remains allowed.

## Dev Notes

### Source Requirements

- Epic 4 goal: monitor input folders, wait for stable JPGs, prevent duplicate processing, route to active session, quarantine unassigned/late photos, and give operator UI review/assign.
- This story implements FR28 and FR29 and supports FR30, FR43, FR44, FR48, FR54.
- PRD journey 2: a late photo after timer/session lock must go to quarantine; operator opens review quarantine, sees thumbnail/metadata, and can assign it back to previous session if it belongs there.
- Reliability/data-integrity guardrails: deterministic server-owned routing, traceability from source to session, duplicate-safe processing, no partial/incomplete JPG considered processed, and operator-visible actionable states.

### Current Code Context To Read Before Editing

- `apps/agent/internal/quarantine/store.go`
  - Current state: in-memory quarantine records from Story 4.3 with fields `quarantine_id`, optional `photo_id`, `station_id`, `related_session_id`, `source_path`, `source_size_bytes`, `detected_at`, `stable_at`, `quarantined_at`, `reason`, `status`, `duplicate`.
  - Current statuses/reasons: `status: "quarantined"`; reasons `no_active_session`, `late_photo`.
  - Duplicate identity: `station_id + normalized source_path + source_size_bytes`, hashed to `quar_*` ID. Preserve this exactly; do not add timestamps to identity.
  - Current list methods only support station/related-session/all and reset `Duplicate=false` for read results.
  - Story change: add assignment lifecycle without breaking quarantine counts and duplicate semantics.
- `apps/agent/internal/ingestion/router.go`
  - Current state: routes stable detected photos to active sessions using `sessions.ActiveForStation`; otherwise creates quarantine records through quarantine store.
  - Preserve: server-owned session eligibility, metadata-only quarantine for unassigned/late photos, and no file movement/copy/delete in ingestion.
  - Story change: assignment should reuse or coordinate with routed-photo creation but must not silently bypass the quarantine lifecycle.
- `apps/agent/internal/photos/store.go`
  - Current state: in-memory routed-photo store with `PhotoID`, `status:"routed"`, source identity based on station/source path/source size, duplicate return behavior, and list/count by session.
  - Critical risk: `photos.Store.Route` currently returns an existing photo for duplicate source identity regardless of target session. Assignment code must detect if an existing photo belongs to a different session and avoid falsely reporting successful assignment to another session.
- `apps/agent/internal/sessions/store.go`
  - Current state: session store supports `active` and `locked`, one active per station, server-time expiry lock, `LastSessionForStation` used by Story 4.3 late-photo reasoning.
  - Story change: provide/consume safe target sessions for assignment; same-station sessions are the primary eligible set. Locked sessions are valid recovery targets only through explicit manual assignment.
  - Preserve: frontend never owns session eligibility/state machine.
- `apps/agent/internal/api/ingestion.go`
  - Current state: `POST /api/ingestion/scan` returns `photos`, `routed_photos`, `quarantined_photos`, `errors`; publishes `photo.quarantined` and activity logs.
  - Preserve: scan semantics and response compatibility.
  - Story change: do not overload scan endpoint for review/assignment; create dedicated quarantine API handlers.
- `apps/agent/internal/api/sessions.go`
  - Current state: session detail returns real `photo_count`, `quarantine_count`, `station_quarantine_count`, `latest_quarantine_reason`; station endpoint returns quarantine summary.
  - Story change: after assignment, counts should reflect assigned/resolved quarantine correctly. Define whether `station_quarantine_count` means open quarantines or total historical quarantines; for operator alert UX it should represent open/unassigned quarantines.
- `apps/web/src/lib/api/client.ts`
  - Current state: has `QuarantinedPhoto`, `RouteResult`, `IngestionScanData`, `StationQuarantineSummary`, runtime guards, and API functions for station quarantine summary.
  - Story change: add quarantine review/assignment API functions and validators; avoid weakening runtime validation.
- `apps/web/src/features/sessions/live-station-cards.tsx`
  - Current state: station cards show routed photo count and station quarantine count/latest reason.
  - Preserve: live-card summary is not the full review UI; keep it as alert/summary.
  - Story change: can add navigation/entry point to quarantine review but should not embed complex assignment workflow directly in cards.
- `apps/web/src/features/health/health-dashboard.tsx`
  - Current state from Story 4.3 review follow-up should invalidate relevant session/quarantine queries on photo events.
  - Story change: add invalidation for assignment events.
- `docs/api/openapi.yaml`
  - Current state: documents quarantine summary and `quarantined_photos` from ingestion scan.
  - Story change: add review/list/eligible/assign endpoints and schemas.

### Architecture Guardrails

- Go service owns all filesystem, worker, session/photo/quarantine state, routing decisions, and credential boundaries.
- Next.js/browser must never read camera folders, decide routing eligibility, or mutate authoritative workflow state locally.
- API responses must use `{data}` wrapper; errors must use `{error:{code,message,action,details}}`.
- DB/API JSON/status fields use `snake_case`; SSE event names use dot notation.
- SSE payload wrapper must include `event_id`, `event_type`, `occurred_at`, and `data`.
- Operator messages must be safe and actionable; technical detail belongs in Go logs/tests.
- Runtime photo files remain in `local-data`; do not expose photos via `apps/web/public` or implement unsafe browser filesystem access.
- Do not implement Epic 5 original-save/LUT processing in this story. Assignment may create a routed photo record/trace only; Epic 5 will consume valid routed session photos for original-first processing.
- Do not introduce Supabase migrations unless the broader persistence layer is deliberately in scope; current stories use in-memory stores and session JSON persistence patterns.

### Recommended Implementation Shape

- New Go API file: `apps/agent/internal/api/quarantine.go`.
- Extend Go package: `apps/agent/internal/quarantine/store.go` with assignment-safe methods.
- Consider a small service package or methods near ingestion/quarantine to keep API handlers thin, e.g. `internal/service/quarantine_assignment.go` if complexity grows.
- Suggested endpoint shapes:
  - `GET /api/quarantine?status=quarantined&station_id=station_1`
  - `GET /api/quarantine/{quarantine_id}/eligible-sessions`
  - `POST /api/quarantine/{quarantine_id}/assign` with `{ "session_id": "sess_..." }`
- Suggested assignment response:
  - `{data:{quarantine:{...status:"assigned", assigned_session_id, assigned_photo_id, assigned_at}, photo:{...status:"routed"}}}`
- Eligibility guidance:
  - Primary allowed target: sessions from same `station_id`.
  - For `late_photo`, prefer related locked session when present, but still require operator confirmation.
  - Reject cross-station by default with error code like `QUARANTINE_SESSION_INELIGIBLE` and action `Pilih session dari station yang sama atau cek ulang foto.`
- Count semantics recommendation:
  - Operator alert counts should count open `status:"quarantined"` only.
  - Historical assigned records may remain visible through a filter but should not keep station in alert state.
- Duplicate handling recommendation:
  - If `photos.Store.Route` returns duplicate with a `SessionID` different from requested target, assignment must not mark quarantine assigned to requested target. Return conflict.
  - If duplicate photo is already routed to the same target session, mark/return assignment idempotently.

### Previous Story Intelligence

- Story 4.1 established stable JPG scanning: case-insensitive `.jpg/.jpeg`, waits for stability, ignores zero-byte/unstable files, handles duplicate paths in process lifetime, and mutates no files. Do not regress scan behavior.
- Story 4.2 established routed-photo records and duplicate identity. Critical review learning: duplicate identity must not include per-detection timestamps. Reuse this anti-regression rule.
- Story 4.3 established quarantine records, reason categories, quarantine count summary, SSE/activity side effects, and station-level summary for no-active-session context.
- Story 4.3 review follow-ups fixed stale UI invalidation for photo/quarantine events and added real station quarantine summaries. Assignment must extend the same invalidation model.
- Git history is sparse (`Add from-scratch setup guide`, `Initial Selfstudio camera capture spike`); current workspace files and story artifacts are more reliable than commit history for implementation patterns.

### Testing Requirements

Run at minimum:

- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build` if environment/dependencies allow.

Required coverage:

- Listing quarantine review items returns expected open records with safe fields and no duplicate rows.
- Assigning an open quarantine item to an eligible same-station session marks it assigned and creates/returns one routed photo.
- Assigning a late-photo quarantine item to its related locked session succeeds only via explicit assignment endpoint.
- Assigning to missing/ineligible/cross-station session returns actionable API error and leaves quarantine open.
- Repeating the same assignment is idempotent and does not create duplicate photo records or duplicate activity side effects beyond acceptable idempotent response behavior.
- Attempting a second assignment to a different session is blocked.
- SSE/activity for assignment use `quarantine_id`/`photo_id`, not raw source path.
- UI updates list, station quarantine count, session photo count, and activity log after assignment.
- OpenAPI and TypeScript validators match actual JSON responses.

### Regression Risks To Avoid

- Do not break Story 4.1 scan semantics, stable detection, duplicate suppression, or station scan errors.
- Do not break Story 4.2 active-session routing or routed photo visibility.
- Do not break Story 4.3 quarantine creation, reason categories, station quarantine summary, or `photo.quarantined` events.
- Do not assign late/no-session photos automatically; assignment must be operator-confirmed.
- Do not let frontend determine eligible sessions or mutate workflow state optimistically without backend confirmation.
- Do not count assigned/resolved quarantine items as open operator alerts unless the UI clearly labels historical counts separately.
- Do not use raw filesystem paths as public IDs or SSE entity IDs.
- Do not start Epic 5 processing queue or local file copy/move in this story.
- Do not introduce browser access to `local-data` or camera input folders.

## Project Structure Notes

- Expected new Go file: `apps/agent/internal/api/quarantine.go`.
- Expected modified Go files: `apps/agent/internal/quarantine/store.go`, `apps/agent/internal/api/sessions.go`, `apps/agent/cmd/selfstudio-agent/main.go`, potentially `apps/agent/internal/sessions/store.go` and `apps/agent/internal/photos/store.go` if safe helpers are needed.
- Expected new/modified web files: `apps/web/src/features/quarantine/*`, `apps/web/src/lib/api/client.ts`, `apps/web/src/features/health/health-dashboard.tsx`, possibly a dashboard entry component where quarantine review is rendered.
- Expected contract file: `docs/api/openapi.yaml`.
- Tests should be co-located: Go `*_test.go`; frontend must at least pass current typecheck/build workflow.

## References

- `_bmad-output/planning-artifacts/epics.md` — Epic 4 and Story 4.4 acceptance criteria; FR28-FR29 mapping.
- `_bmad-output/planning-artifacts/prd.md` — Journey 2 late photo quarantine/recovery; Photo Ingestion and Routing; NFR10, NFR14, NFR29, NFR31.
- `_bmad-output/planning-artifacts/architecture.md` — Go service ownership, REST/SSE contracts, naming/status patterns, idempotency, project boundaries.
- `_bmad-output/implementation-artifacts/4-3-quarantine-unassigned-and-late-photos.md` — previous story implementation details and review follow-ups.
- `apps/agent/internal/quarantine/store.go` — current quarantine store to extend.
- `apps/agent/internal/photos/store.go` — routed-photo duplicate identity and conflict risk.
- `apps/agent/internal/api/sessions.go` — current quarantine summary/count behavior.
- `apps/web/src/lib/api/client.ts` — existing quarantine types/validators.
- `docs/api/openapi.yaml` — API contract to update.

## Dev Agent Record

### Agent Model Used

GPT-5.1 Codex Max

### Debug Log References

- `cd apps/agent && go test ./internal/quarantine` initially failed as expected in RED phase because `Assign`, `ListFilter`, `StatusAssigned`, and related methods did not exist.
- Direct `go test ./...` execution on this Windows workspace hit `Access is denied` when Go tried to execute temp test binaries from the default temp build path.
- Workaround validation used `go test -c ./...` followed by executing generated `*.test.exe` via `cmd.exe /c`; all generated test binaries passed.
- `cd apps/web && npm run typecheck` passed.
- `cd apps/web && npm run build` passed.
- Review follow-up RED/GREEN validation: `cd apps/agent && go test ./...` passed after adding locked-session eligibility restrictions and API regression tests.

### Completion Notes List

- Extended quarantine store with `assigned` lifecycle, assignment trace fields, filtered list/get/assign methods, idempotent same-session assignment, and different-session conflict handling while preserving duplicate identity based on station/path/size.
- Added quarantine review/eligible/assign API endpoints with `{data}` responses, existing API error shape, server-side same-station eligibility, locked related late-photo support via explicit assignment endpoint, photo-route conflict protection, safe activity logging, and `quarantine.assigned`/`photo.routed` SSE publishing.
- Added UI under `apps/web/src/features/quarantine/` showing quarantine metadata, eligible session picker, explicit confirmation, actionable loading/error/success states, and cache invalidation for quarantine/session/station summary/activity data.
- Updated web SSE handling so `quarantine.assigned`, `photo.routed`, and `photo.quarantined` refresh affected live dashboard queries.
- Updated OpenAPI contract for quarantine list, eligible sessions, assignment request/response, assigned status fields, and open/unassigned quarantine count semantics.
- Added focused Go store/API tests for listing, same-station assignment, late-photo locked-session eligibility, cross-station rejection, conflict handling, duplicate idempotency, safe activity, and safe SSE IDs.
- ✅ Resolved review finding [HIGH]: locked-session eligibility now rejects `no_active_session` quarantine assignment to locked same-station sessions and only allows locked targets for related `late_photo` recovery; added API tests for rejected no-active locked target and accepted related locked late-photo assignment.

### File List

- apps/agent/cmd/selfstudio-agent/main.go
- apps/agent/internal/api/health.go
- apps/agent/internal/api/quarantine.go
- apps/agent/internal/api/quarantine_assignment_test.go
- apps/agent/internal/photos/store.go
- apps/agent/internal/quarantine/store.go
- apps/agent/internal/quarantine/store_test.go
- apps/web/src/app/globals.css
- apps/web/src/features/health/health-dashboard.tsx
- apps/web/src/features/quarantine/quarantine-review.tsx
- apps/web/src/features/quarantine/use-assign-quarantine-mutation.ts
- apps/web/src/features/quarantine/use-eligible-sessions-query.ts
- apps/web/src/features/quarantine/use-quarantine-query.ts
- apps/web/src/lib/api/client.ts
- docs/api/openapi.yaml
- _bmad-output/implementation-artifacts/4-4-review-and-assign-quarantined-photos.md
- _bmad-output/implementation-artifacts/sprint-status.yaml

## Change Log

- 2026-05-19: Ultimate context engine analysis completed - comprehensive developer guide created.
- 2026-05-19: Implemented quarantine review and manual assignment lifecycle; added Go API/store/tests, web review UI, SSE invalidation, and OpenAPI updates.
- 2026-05-19: Addressed code review finding - 1 HIGH item resolved; tightened locked-session eligibility and added API regressions.
