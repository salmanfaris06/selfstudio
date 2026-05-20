# Story 5.3: Track Processing Queue and Photo Status

Status: done

## Story

Sebagai operator, saya ingin melihat status processing queue dan status per-foto secara jelas, sehingga saya bisa cepat mengetahui backlog, job yang sedang berjalan, kegagalan, retry count, dan hambatan processing selama event tanpa menghentikan dashboard atau station lain.

## Acceptance Criteria

1. Given photos are pending, processing, processed/succeeded, failed, or retrying, when operator opens queue/status view, then UI shows queue counts grouped by status, current job state, failure reason, retry count, and last update.
2. Given graded processing state changes after original save, quarantine assignment, success, failure, or future retry transition, when state changes, then safe SSE events update queue/status views using `photo_id`/safe entity IDs and not raw filesystem paths.
3. Given image processing is running, slow, failed, or unavailable, then dashboard/session controls remain interactive and processing does not block other stations.
4. Given multiple stations have routed photos, queue/status summary separates station/session/photo identity enough for operator troubleshooting while preserving Go service as source of truth.
5. Given a photo has original-save and graded-processing metadata, when exposed through API, then response uses `{data}` wrapper, `snake_case` JSON, existing status fields, timestamps, failure reason, attempt count, local-original/local-graded paths where already exposed, and no frontend-invented state machine.
6. Given queue/status UI is loading, empty, disconnected, or receives malformed data, then it shows actionable text labels and does not rely on color only.
7. Given existing Epic 4 ingestion/quarantine and Stories 5.1/5.2 original/graded processing exist, then this story does not regress stable JPG detection, duplicate-safe routing, original-first save, LUT processing, quarantine assignment, restart reconciliation, or existing API/web contract validators.
8. Tests/build pass for Go agent and relevant web/API contract checks.

## Tasks / Subtasks

- [x] Define processing queue/status read model in the Go service. (AC: 1, 4, 5)
  - [x] Add a server-owned summary DTO derived from `photos.Store`, not from frontend cache, with at minimum: `total`, `not_eligible`, `pending`, `processing`, `processed`, `failed`, `retrying` (future-compatible, may be `0` until Story 5.4), `last_updated_at`, and optional `current_job`.
  - [x] Add per-photo queue item DTO with: `photo_id`, `station_id`, `session_id`, `source_path`, `local_original_path`, `local_graded_path`, `original_save_status`, `processing_status`, `graded_processing_status`, `graded_last_error`, `graded_attempt_count`, `graded_processing_started_at`, `graded_processed_at`, and computed `last_updated_at`.
  - [x] Treat Story 5.2 `graded_processing_status = processed` as the succeeded state for UI labels; do not rename existing persisted status values.
  - [x] Add a computed `retrying` bucket only as future-compatible API field; do not implement Story 5.4 retry policy here.
- [x] Add processing queue/status API endpoint(s). (AC: 1, 4, 5)
  - [x] Recommended endpoint: `GET /api/processing/queue` returning `{data:{summary, items}}`.
  - [x] Support safe filters if simple: `station_id`, `session_id`, `status`, `limit`; validate station IDs and positive limits.
  - [x] Preserve existing API error shape `{error:{code,message,action,details}}`.
  - [x] Keep all filesystem/path inspection server-side; frontend must not read local files.
  - [x] Update `docs/api/openapi.yaml` for endpoint, response schemas, status enum, and queue SSE events.
- [x] Publish consistent queue/status SSE updates. (AC: 2, 3, 5)
  - [x] Reuse existing events from Story 5.2: `photo.processing_started`, `photo.processed`, `photo.processing_failed`, and `queue.updated`.
  - [x] Ensure every queue-impacting transition publishes `queue.updated` with safe IDs and enough fields for query invalidation (`photo_id`, `station_id`, `session_id`, `graded_processing_status` if available).
  - [x] Confirm ingestion and quarantine assignment paths both publish the same queue events.
  - [x] Do not use `source_path`, `local_original_path`, `local_graded_path`, or LUT path as SSE `entity_id`.
- [x] Add web API client types/validators for processing queue. (AC: 1, 5, 6)
  - [x] Update `apps/web/src/lib/api/client.ts` with `ProcessingQueueSummary`, `ProcessingQueueItem`, `ProcessingQueueData`, and `getProcessingQueue(...)`.
  - [x] Add runtime validators matching existing client patterns (`isRoutedPhoto`, session detail validators, etc.).
  - [x] Keep status labels derived from backend status fields; do not create a separate frontend state machine.
- [x] Build minimal operator queue/status UI. (AC: 1, 4, 6)
  - [x] Add a feature component under `apps/web/src/features/processing/`, e.g. `processing-queue-panel.tsx` plus hook `use-processing-queue-query.ts`.
  - [x] Show text labels for status buckets: pending, processing, processed, failed, retrying.
  - [x] Show current/recent items with station, session, photo id, status, retry/attempt count, failure reason, and last update.
  - [x] Include loading, empty, error, and disconnected/SSE-lag-friendly copy.
  - [x] Integrate the panel into the existing dashboard/page shell without disrupting live station cards.
  - [x] Optional but recommended: include processing summary on live station cards using existing session detail/failure counts, but avoid large UI refactors.
- [x] Ensure SSE-driven refresh/invalidation. (AC: 2, 3, 6)
  - [x] Locate existing SSE client/query invalidation code in `apps/web/src/lib/events` or dashboard providers.
  - [x] Invalidate/refetch processing queue query on `queue.updated`, `photo.processing_started`, `photo.processed`, and `photo.processing_failed`.
  - [x] Keep TanStack Query as server-state cache; use `invalidateQueries` or targeted cache updates, not bespoke polling-only state.
- [x] Preserve non-blocking processing behavior. (AC: 3, 7)
  - [x] Do not move ImageMagick or graded processing back into blocking HTTP request handlers.
  - [x] Do not hold `photos.Store` locks while rendering summaries for long operations; copy store records then compute in memory.
  - [x] Keep queue endpoint read-only and fast; no processing side effects.
- [x] Add focused tests and validation. (AC: 1-8)
  - [x] Go tests for queue summary buckets and per-photo item fields from mixed photo states: not eligible, pending, processing, processed, failed.
  - [x] Go API tests for `/api/processing/queue`, filters, invalid filters, `{data}` wrapper, and error shape.
  - [x] Go regression tests that existing ingestion/quarantine processing events still publish `queue.updated` and do not use raw paths as entity IDs.
  - [x] Web type/validator tests if project pattern exists; otherwise at minimum `npm run typecheck` must cover new types/components.
  - [x] Run `cd apps/agent && go test ./...`.
  - [x] Run `cd apps/web && npm run typecheck`.
  - [x] Run `cd apps/web && npm run build` if environment allows.

### Review Follow-ups (AI)

- [x] [HIGH][Review][Patch] Fix processing queue summary so `limit` only limits returned `items`, not status bucket counts. Current `BuildQueueStatus` filters, sorts, truncates by `filter.Limit`, then calls `summarizeQueue(items)`, so `/api/processing/queue?limit=20` and the dashboard panel can under-report `total`, `pending`, `processing`, `processed`, `failed`, `retrying`, and `current_job` when more than 20 matching photos exist. This violates AC1/AC4 read-model accuracy and makes the dashboard summary misleading during event backlog troubleshooting. Evidence: `apps/agent/internal/processing/queue_status.go:52-71`; dashboard uses `useProcessingQueueQuery({ limit: 20 })` in `apps/web/src/features/processing/processing-queue-panel.tsx`.

## Dev Notes

### Source Requirements

- Epic 5 objective: original-first local delivery, LUT processing, processing status, retry, collision safety, queue/status visibility, and recovery.
- Story 5.3 acceptance criteria from epics: queue/status view shows queue counts, current job state, failure reason, retry count, and last update; updates via SSE; dashboard remains interactive; processing does not block other stations.
- FR35: system can track processing status for each photo.
- FR47: operator can view processing queue and upload queue status. This story covers processing queue only; upload queue remains Epic 6.
- FR43/FR44/FR48: processing problems must be actionable and visible in dashboard/troubleshooting views/logs.
- NFR2/NFR3: image processing must not block dashboard/session controls and must not block other stations.
- NFR12/NFR14/NFR16/NFR18/NFR28/NFR29/NFR31: surface failed processing/missing LUT, preserve traceability, duplicate-safe retries, no dangling success state, understandable text labels, specific actions.

### Current Code Context To Read Before Editing

- `apps/agent/internal/photos/store.go`
  - Current state: `Photo` already includes Story 5.1/5.2 processing fields: `local_original_path`, `original_save_status`, `processing_status`, `local_graded_path`, `graded_processing_status`, `graded_last_error`, `graded_attempt_count`, `graded_processing_started_at`, `graded_processed_at`, `lut_snapshot_path`.
  - Story change: derive queue summary/items from these fields. Prefer additive read-model code rather than changing `Photo` persistence shape.
  - Must preserve: duplicate identity `Identity(station_id, source_path, source_size_bytes)`, `Route(...)` duplicate behavior, original-save state normalization, graded state values.
- `apps/agent/internal/processing/graded_processor.go`
  - Current state: `GradedProcessor.Process` verifies saved original, uses session snapshot LUT/output folder, persists `processing`, invokes `LUTProcessor`, persists `processed` or `failed`, and `Reconcile` processes pending/failed states at startup.
  - Story change: expose its state through queue API/UI. Do not add blocking behavior inside request handlers. Be careful that existing `Reconcile` may process pending/failed jobs; this story should not broaden recovery/retry semantics beyond status visibility.
  - Must preserve: original-first gating, safe failure messages, deterministic graded path, refusal to accept/overwrite pre-existing unverified output.
- `apps/agent/internal/processing/lut_processor.go`
  - Current state: ImageMagick 7 adapter using `exec.CommandContext`, timeout, safe processor-unavailable/failure errors.
  - Story change: no need to alter LUT command unless queue status needs clearer safe error strings. Do not implement frontend image processing.
- `apps/agent/internal/api/ingestion.go`
  - Current state: after original save, `enqueueGraded` publishes `photo.processing_started`, runs `GradedProcessor.Process` in a goroutine, then publishes `photo.processed`/`photo.processing_failed` and `queue.updated`.
  - Story change: ensure queue UI/API uses these events. If event payload is extended, keep safe IDs only.
  - Must preserve: HTTP scan must not block on ImageMagick; no raw paths as SSE entity IDs.
- `apps/agent/internal/api/quarantine.go`
  - Current state: manual assignment saves original then `enqueueAssignedGraded` publishes started/terminal processing events and activity entries in background.
  - Story change: verify parity with ingestion and include queue invalidation needs. Preserve locked-session eligibility guard.
- `apps/agent/internal/api/sessions.go`
  - Current state: session detail returns photos and summary; failures count `GradedStatusFailed`. Live station cards rely on this summary.
  - Story change: can remain as-is or be lightly extended, but the queue endpoint should become the primary queue/status view. Do not break existing `SessionDetailData` validator.
- `docs/api/openapi.yaml`
  - Current state: documents photo graded fields/events from Story 5.2 and SSE wrapper examples.
  - Story change: document processing queue endpoint/schema and queue-related event payloads.
- `apps/web/src/lib/api/client.ts`
  - Current state: contains `RoutedPhoto`, `OriginalSaveStatus`, `ProcessingStatus`, `GradedProcessingStatus`, session/quarantine types and validators.
  - Story change: add queue types/fetcher/validators. Reuse `GradedProcessingStatus`; do not invent incompatible status strings.
- `apps/web/src/features/sessions/live-station-cards.tsx`
  - Current state: displays station cards, active session detail, photo count, failure count, quarantine count, and latest photo status.
  - Story change: add queue panel elsewhere or minimal summary here; avoid turning station cards into a complex processing table.
- `apps/web/src/lib/events` or equivalent SSE integration
  - Current state: locate existing event subscription/query invalidation behavior before editing.
  - Story change: invalidate processing queue query on queue/photo processing events.

### Architecture Guardrails

- Go service owns authoritative station/session/photo/processing state. Frontend may cache via TanStack Query but must not invent workflow state machines.
- Filesystem, image processing, local paths, recovery, and credentials remain Go-only. Next.js/browser must never read/write camera folders, originals, graded outputs, LUT files, or local-data files directly.
- API responses must keep `{data}` wrapper; failures must keep `{error:{code,message,action,details}}`.
- DB/API JSON/status fields use `snake_case`; SSE event names use dot notation.
- Queue/status is a read model over existing photo/processing state. Do not duplicate durable state unless there is a strong reason.
- Raw paths may remain API data where already exposed for operator troubleshooting, but never use raw paths as SSE `entity_id` or noisy activity-log identifiers.
- Processing visibility must not regress non-blocking behavior. Do not call ImageMagick synchronously from queue/status endpoints.
- Do not implement Story 5.4 retry policy in this story. Show `retry count` using `graded_attempt_count`; show `retrying` as future-compatible only if needed.
- Do not implement Story 5.5 pending-job recovery beyond what Story 5.2 already provides.
- Do not implement upload queue from Epic 6; if UI labels mention upload, keep existing placeholder separation.

### Recommended Implementation Shape

- Add Go package or files:
  - `apps/agent/internal/processing/queue_status.go` for summary/item DTOs and `BuildQueueStatus(photoStore, filter)` helper; or keep in `internal/api` only if logic is trivial. Prefer processing package for testability.
  - `apps/agent/internal/processing/queue_status_test.go`.
  - `apps/agent/internal/api/processing_queue.go` and `processing_queue_test.go` for HTTP endpoint.
- Modify Go wiring:
  - `apps/agent/cmd/selfstudio-agent/main.go` to register the endpoint on the existing mux/router.
  - Reuse existing auth/origin middleware patterns from other `/api/*` endpoints.
- Modify web:
  - `apps/web/src/lib/api/client.ts` for fetcher/types/validators.
  - `apps/web/src/features/processing/use-processing-queue-query.ts`.
  - `apps/web/src/features/processing/processing-queue-panel.tsx`.
  - Existing dashboard/page component to render the panel.
  - Existing SSE provider/client to invalidate queue query.
- Keep implementation simple and operator-centered: counts + latest/current items are enough for this story. Avoid a large table framework dependency.

### Previous Story Intelligence

- Story 5.2 implemented graded LUT processing and resolved key review follow-ups:
  - Existing deterministic graded targets must not be silently accepted; pre-existing outputs are rejected as unverified.
  - Ingestion and quarantine assignment enqueue graded processing in background goroutines, preventing HTTP responses from blocking on ImageMagick.
  - Quarantine assignment publishes the same processing SSE contract as ingestion.
  - ImageMagick `.cube` command has fixture coverage that skips when `magick` is unavailable.
- Story 5.1 implemented original-save lifecycle and resolved review findings:
  - API responses now return post-save fields.
  - Original saver byte-compares same-size existing targets before accepting idempotent success.
  - Original-save fields are already exposed in API/web types.
- Story 4.5 added durable photo/quarantine persistence and transactional rollback. Any new queue endpoint should read from this durable in-memory state and not introduce partial persistence risk.
- Story 4.4 established manual quarantine assignment eligibility: `late_photo` can target related locked session; `no_active_session` cannot be assigned to arbitrary locked sessions. Do not regress.
- Git history remains sparse (`Add from-scratch setup guide`, `Initial Selfstudio camera capture spike`); story artifacts and current workspace code are the authoritative context.

### Latest Technical Notes

- Architecture currently records observed tools: Go `1.26.3`, Next.js `16.2.6`, TanStack Query `5.100.10`, Supabase CLI `v2.98.1`, shadcn CLI v4. Workspace `apps/web/package.json` currently uses Next `^14.2.0` and React `^18.2.0`; follow the actual workspace unless a deliberate upgrade story exists.
- TanStack Query v5 docs emphasize invalidating related queries after mutations/events; use query invalidation for `queue.updated` and processing events rather than maintaining a separate frontend workflow state.
- Go `exec.CommandContext` is already used in `lut_processor.go` for timeout/cancellation; this story should observe/report state only and not change process management unless necessary.
- ImageMagick remains an operational dependency for Story 5.2. Queue UI should surface `LUT_PROCESSOR_UNAVAILABLE` / `LUT_PROCESSING_FAILED` safe messages from `graded_last_error` without exposing raw stderr.

### Testing Requirements

Run at minimum:

- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build` if dependencies/environment allow.

If Windows blocks Go test execution from temp paths with `Access is denied`, use the known workaround from prior stories: `go test -c ./...` or package-specific `go test -c`, then execute generated `*.test.exe` via `cmd.exe /c`.

Required coverage:

- Queue summary correctly counts mixed statuses: no original/not eligible, saved original pending graded, processing, processed, failed.
- `last_updated_at` is deterministic from available timestamps (`graded_processed_at`, `graded_processing_started_at`, `original_saved_at`, `routed_at`, etc.).
- Queue item includes failure reason and `graded_attempt_count` for failed processing.
- Filters by station/session/status return only matching items and reject invalid values safely.
- Endpoint response uses `{data}` and OpenAPI-described `snake_case` fields.
- SSE event payloads use safe IDs and trigger web query invalidation.
- Web queue panel renders loading, empty, failed, processing, and processed states with text labels.
- Existing `cd apps/agent && go test ./...` regression remains green for ingestion/quarantine/original/graded processing.

### Regression Risks To Avoid

- Do not mark a failed photo as processed just for UI convenience.
- Do not collapse `processing_status` and `graded_processing_status` into a new persisted field without full migration and test coverage.
- Do not block `/api/processing/queue` on image processing or filesystem verification.
- Do not reintroduce synchronous graded processing in ingestion/quarantine HTTP handlers.
- Do not expose raw filesystem paths as SSE entity IDs.
- Do not add frontend-only state transitions such as changing pending to processed before Go service reports it.
- Do not implement automatic/manual retry behavior in this story; preserve that for Story 5.4.
- Do not implement upload queue/status beyond existing placeholders; cloud upload is Epic 6.
- Do not break existing `RoutedPhoto` validators or session detail API consumers.

## Project Structure Notes

- Expected new files:
  - `apps/agent/internal/processing/queue_status.go`
  - `apps/agent/internal/processing/queue_status_test.go`
  - `apps/agent/internal/api/processing_queue.go`
  - `apps/agent/internal/api/processing_queue_test.go`
  - `apps/web/src/features/processing/use-processing-queue-query.ts`
  - `apps/web/src/features/processing/processing-queue-panel.tsx`
- Expected modified files:
  - `apps/agent/cmd/selfstudio-agent/main.go`
  - `apps/agent/internal/api/ingestion.go` and/or `quarantine.go` only if event payload parity needs minor extension
  - `docs/api/openapi.yaml`
  - `apps/web/src/lib/api/client.ts`
  - Existing dashboard/page/SSE provider files under `apps/web/src/app`, `apps/web/src/features`, or `apps/web/src/lib/events` after locating current structure
- Runtime/generated photo assets remain under configured session output folders and `local-data`; never under `apps/web/public` or committed source.

## References

- `_bmad-output/planning-artifacts/epics.md` — Epic 5 and Story 5.3 acceptance criteria; FR35 and FR47 mapping.
- `_bmad-output/planning-artifacts/prd.md` — Image Processing and Local Storage, Dashboard and Operator Controls, NFR2/NFR3/NFR12/NFR14/NFR28/NFR29/NFR31.
- `_bmad-output/planning-artifacts/architecture.md` — Go service worker ownership, REST/SSE patterns, TanStack Query server-state ownership, queue isolation, API/event naming.
- `_bmad-output/implementation-artifacts/5-2-apply-station-session-lut-to-create-graded-jpg.md` — immediate prior story implementation, current fields/events, review follow-ups.
- `_bmad-output/implementation-artifacts/5-1-save-original-jpg-before-processing.md` — original-save lifecycle and processing eligibility foundation.
- `_bmad-output/implementation-artifacts/4-5-recover-pending-ingestion-and-quarantine-state.md` — durable persistence and rollback lessons.
- `apps/agent/internal/photos/store.go` — photo/original/graded processing state fields.
- `apps/agent/internal/processing/graded_processor.go` — graded processing transitions and reconciliation.
- `apps/agent/internal/api/ingestion.go` — queue-impacting SSE events after ingestion route.
- `apps/agent/internal/api/quarantine.go` — queue-impacting SSE events after quarantine assignment.
- `apps/agent/internal/api/sessions.go` — current session detail failure counts and photo list.
- `apps/web/src/lib/api/client.ts` — API client types/validators to extend.
- `apps/web/src/features/sessions/live-station-cards.tsx` — current operator dashboard station UI.
- `docs/api/openapi.yaml` — API/SSE contract source.

## Dev Agent Record

### Agent Model Used

OpenAI GPT-5 Codex

### Debug Log References

- `cd apps/agent && go test ./internal/processing` initially failed as expected during RED phase because the queue read model tests had no passing implementation/test fixture normalization yet.
- `cd apps/agent && go test ./internal/api ./internal/processing` passed after endpoint/read-model implementation.
- `cd apps/web && npm run typecheck` passed after web client, hook, and panel implementation.
- `cd apps/web && npm run typecheck && npm run build` passed.
- `cd apps/agent && go test ./...` passed.
- `cd apps/agent && go test ./internal/processing` passed after adding limit-vs-summary regression coverage.
- `cd apps/agent && go test ./...` passed after fixing the HIGH review follow-up.

### Completion Notes List

- Ultimate context engine analysis completed - comprehensive developer guide created.
- Implemented server-owned processing queue read model over `photos.Store` with summary buckets, current job, per-photo status metadata, computed `last_updated_at`, and future-compatible `retrying` bucket without adding retry policy.
- Added authenticated `GET /api/processing/queue` endpoint with `{data:{summary,items}}`, safe filters, positive limit validation, and preserved API error shape.
- Extended ingestion and quarantine processing SSE payloads so queue-impacting transitions publish safe `queue.updated` events with photo/station/session IDs and graded status; no raw filesystem paths are used as SSE entity IDs.
- Added web API types, fetcher, runtime validators, TanStack Query hook, and operator queue/status panel with loading, empty, error, disconnected/SSE-lag-friendly copy, status buckets, current job, recent items, failure reason, retry/attempt count, and last update.
- Integrated queue invalidation into the existing dashboard SSE flow for `queue.updated`, `photo.processing_started`, `photo.processed`, and `photo.processing_failed` while preserving existing live station cards and non-blocking background processing.
- Updated OpenAPI with processing queue endpoint, schemas, status enum, and queue-related SSE examples.
- ✅ Resolved review finding [HIGH]: processing queue summary is now computed from all matching filtered photos before `limit` is applied, while returned `items` still honor `limit`; added Go regression tests for read model and API endpoint.

### File List

- apps/agent/cmd/selfstudio-agent/main.go
- apps/agent/internal/api/health.go
- apps/agent/internal/api/ingestion.go
- apps/agent/internal/api/processing_queue.go
- apps/agent/internal/api/processing_queue_test.go
- apps/agent/internal/api/quarantine.go
- apps/agent/internal/processing/queue_status.go
- apps/agent/internal/processing/queue_status_test.go
- apps/web/src/features/health/health-dashboard.tsx
- apps/web/src/features/processing/processing-queue-panel.tsx
- apps/web/src/features/processing/use-processing-queue-query.ts
- apps/web/src/lib/api/client.ts
- docs/api/openapi.yaml
- _bmad-output/implementation-artifacts/5-3-track-processing-queue-and-photo-status.md
- _bmad-output/implementation-artifacts/sprint-status.yaml

## Change Log

- 2026-05-19: Ultimate context engine analysis completed - comprehensive developer guide created.
- 2026-05-19: Implemented processing queue/status read model, API endpoint, SSE invalidation, operator UI, OpenAPI docs, and focused validations for Story 5.3.
- 2026-05-19: Addressed HIGH code review finding for queue summary accuracy before item limit/pagination and returned story to review.
