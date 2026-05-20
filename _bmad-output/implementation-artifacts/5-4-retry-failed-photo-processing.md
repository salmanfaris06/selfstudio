# Story 5.4: Retry Failed Photo Processing

Status: done

## Story

Sebagai operator, saya ingin processing foto yang gagal dapat di-retry otomatis dan manual secara aman, sehingga kegagalan sementara seperti LUT/processor belum siap dapat dipulihkan tanpa kehilangan original JPG dan tanpa membuat output/record duplikat.

## Acceptance Criteria

1. Given graded photo processing fails, when retry policy runs, then the system retries failed graded processing up to three total attempts automatically and never retries after the third failed attempt without operator action.
2. Given a failed processing job is visible in queue/status UI, when an authenticated operator triggers manual retry, then Go service retries from the verified `local_original_path`, increments attempt count through existing processing state, and returns an actionable `{data}` response.
3. Given manual or automatic retry is requested for a photo that is not retryable, then API rejects it with `{error:{code,message,action,details}}` explaining the required action and does not mutate photo state.
4. Given retries run repeatedly, concurrently, or after duplicate operator clicks/SSE refreshes, then retry behavior is duplicate-safe: one photo/job identity produces at most one final graded output path and no duplicate photo records.
5. Given retry starts, succeeds, or fails again, then system publishes safe SSE events (`photo.processing_started`, `photo.processed` or `photo.processing_failed`, and `queue.updated`) using `photo_id`/safe IDs only, and logs manual retry actions in activity log.
6. Given repeated failure remains after retry attempts, then queue/status UI shows retry/attempt count and an actionable reason such as fix LUT, install ImageMagick, or retry later; critical state uses text labels, not color only.
7. Given existing original-save, graded-processing, ingestion, quarantine assignment, and queue/status features exist, then this story does not regress original-first save, stable JPG detection, quarantine assignment, non-blocking dashboard behavior, or current API/web validators.
8. Tests/build pass for Go agent and relevant web/API contract checks.

## Tasks / Subtasks

- [x] Define retry eligibility and attempt policy in Go processing layer. (AC: 1, 3, 4, 7)
  - [x] Add constants/helpers near `apps/agent/internal/processing` for `MaxGradedAttempts = 3`, retryable statuses, and safe retry rejection reasons.
  - [x] Eligible for retry: existing photo with `original_save_status = saved_original`, non-empty verified `local_original_path`, `processing_status = eligible`, `graded_processing_status = failed`, and `graded_attempt_count < 3` for automatic retry.
  - [x] Manual retry may be allowed for `graded_processing_status = failed` even when `graded_attempt_count >= 3`, but must be explicit operator action and still require verified original. Document and test this distinction.
  - [x] Never retry from `source_path` only. Retry must use the saved original path created by Story 5.1.
  - [x] Do not add new durable state machine unless necessary; extend existing `photos.Photo` fields and `GradedProcessor.Process` behavior first.
- [x] Implement automatic retry scheduling without blocking HTTP requests/dashboard. (AC: 1, 4, 5, 7)
  - [x] Reuse current background goroutine pattern from ingestion/quarantine assignment; do not run ImageMagick synchronously in request handlers.
  - [x] On processing failure from ingestion/quarantine, automatically schedule retry until `graded_attempt_count` reaches 3 total attempts.
  - [x] Use existing deterministic `GradedPath(session, photo)` so retries target the same final path and remain collision-safe.
  - [x] Ensure retry does not accept/overwrite an unverified existing graded target; preserve current Story 5.2 safety behavior.
  - [x] Avoid tight retry loops that starve other stations; use a small backoff/timer or queue worker pattern that keeps station/session controls responsive.
- [x] Add manual retry API endpoint. (AC: 2, 3, 4, 5)
  - [x] Recommended endpoint: `POST /api/photos/{photo_id}/retry-processing`.
  - [x] Require auth and trusted origin, following existing mutation route patterns in `apps/agent/internal/api/health.go`/mux wiring.
  - [x] Response should use `{data:{photo}}` or `{data:{photo, retry_started:true}}` with existing `photos.Photo` JSON fields; keep `snake_case`.
  - [x] Invalid cases return actionable errors: photo not found, original missing/invalid, already processing, already processed, not eligible, automatic limit reached without manual action context, processor/session unavailable.
  - [x] Protect concurrent manual retry clicks with store/processing guard so only one processing attempt starts for a photo at a time.
- [x] Publish retry-safe SSE and activity events. (AC: 5, 6)
  - [x] Reuse current safe event pattern from `ingestion.go` and `quarantine.go`: `entity_type = photo`, `entity_id = photo_id`, data includes `photo_id`, `station_id`, `session_id`, and `graded_processing_status`.
  - [x] Publish `queue.updated` for retry start and terminal retry result.
  - [x] Manual retry records activity action such as `photo.processing_retry` with success/failure and station/session refs; do not log raw source/original/graded paths in activity messages.
  - [x] Automatic retry may record concise technical/operator-safe activity only for terminal repeated failure; avoid noisy logs per internal backoff if that would overwhelm activity view.
- [x] Update processing queue/status web UI for manual retry action. (AC: 2, 3, 5, 6)
  - [x] Extend `apps/web/src/lib/api/client.ts` with manual retry fetcher/types/validator.
  - [x] Add mutation hook under `apps/web/src/features/processing/` or colocate with existing queue hook.
  - [x] In `processing-queue-panel.tsx`, show a `Retry processing` button only for failed queue items; disable while mutation is pending.
  - [x] After retry mutation or relevant SSE event, invalidate/refetch processing queue query using TanStack Query; do not create frontend-only processing state transitions.
  - [x] Display attempt count and failure reason already exposed by Story 5.3; add copy that tells operator to fix LUT/ImageMagick when safe error code indicates that cause.
- [x] Update OpenAPI/API contract docs. (AC: 2, 3, 5, 8)
  - [x] Add `POST /api/photos/{photo_id}/retry-processing` to `docs/api/openapi.yaml`.
  - [x] Document success response, error shape, auth requirement, retry events, and attempt-count semantics.
  - [x] Keep status enums compatible with current `GradedProcessingStatus`; do not introduce frontend-only persisted `retrying` unless backend actually returns it.
- [x] Preserve existing behavior and non-goals. (AC: 4, 7)
  - [x] Do not implement Story 5.5 restart recovery beyond preserving current `GradedProcessor.Reconcile` semantics.
  - [x] Do not implement upload retry/cloud upload; that belongs to Epic 6.
  - [x] Do not move processing, filesystem checks, LUT execution, or credentials into Next.js/browser.
  - [x] Do not rename existing persisted statuses (`processed` remains succeeded state).
- [x] Add focused tests and validation. (AC: 1-8)
  - [x] Go unit tests for retry eligibility, max automatic attempts, manual retry after max attempts, invalid original/session/LUT cases, and duplicate/concurrent retry guard.
  - [x] Go API tests for `POST /api/photos/{photo_id}/retry-processing`: auth/trusted origin, success `{data}`, not found, not retryable, already processing/processed, and error shape.
  - [x] Go SSE/activity tests verifying safe IDs and manual retry activity logging without raw paths.
  - [x] Web type/validator/mutation coverage if project pattern exists; otherwise `npm run typecheck` must cover new client/hook/component changes.
  - [x] Run `cd apps/agent && go test ./...`.
  - [x] Run `cd apps/web && npm run typecheck`.
  - [x] Run `cd apps/web && npm run build` if environment allows.

### Review Follow-ups (AI)

- [x] [HIGH] Automatic processing uses per-call guards, allowing duplicate concurrent attempts. Evidence: ingestion and quarantine call `newProcessingRunner(..., nil, ...)`, which creates a new `ProcessingGuard` each time (`apps/agent/internal/api/ingestion.go:122`, `apps/agent/internal/api/quarantine.go:205`, `apps/agent/internal/api/processing_runner.go:22-31`). A manual retry uses the shared guard, but it cannot see an already-running automatic attempt. This violates AC4 duplicate-safe concurrency and can increment attempts twice or race the deterministic graded output.
- [x] [HIGH] Restart reconciliation still retries failed photos after the automatic limit. Evidence: `GradedProcessor.Reconcile` calls `p.Process` for every saved-original photo with `GradedStatusFailed` regardless of `graded_attempt_count` (`apps/agent/internal/processing/graded_processor.go:88-90`). A failed photo at 3 attempts is retried automatically on restart without operator action, violating AC1 and the Story 5.5 non-goal boundary.
- [x] [MEDIUM] Required API/SSE/activity test coverage for manual retry is missing. Evidence: no `apps/agent/internal/api/photo_retry_test.go` or equivalent tests cover `POST /api/photos/{photo_id}/retry-processing`, auth/trusted origin, `{data}` response/error shape, safe retry SSE/activity, or duplicate manual clicks, despite explicit Testing Requirements and Task checklist. Existing retry coverage is limited to `apps/agent/internal/processing/retry_policy_test.go:20-70`.

## Dev Notes

### Source Requirements

- Epic 5 objective: original-first local delivery, LUT processing, processing status, retry, collision safety, queue/status visibility, and processing recovery.
- Story 5.4 from epics: retry failed photo processing automatically and manually; up to three automatic attempts; duplicate-safe; retry actions logged; repeated failure shows actionable reason.
- FR36: system automatically retries failed photo processing up to three times.
- FR37: admin/operator can manually retry failed photo processing.
- FR34/FR39: original JPG must be preserved and missing/invalid LUT must mark graded job failed/retryable.
- FR43/FR47/FR48: dashboard must show actionable processing problems, queue status, and activity logs.
- NFR3/NFR16/NFR18/NFR31: retries must not block dashboard/session controls, must be duplicate-safe, must not leave persisted records pointing to missing final files, and must expose a specific action such as Retry Photo Processing.

### Current Code Context To Read Before Editing

- `apps/agent/internal/photos/store.go`
  - Current state: `Photo` already has original and graded processing fields: `original_save_status`, `processing_status`, `local_original_path`, `local_graded_path`, `graded_processing_status`, `graded_last_error`, `graded_attempt_count`, `graded_processing_started_at`, `graded_processed_at`, and `lut_snapshot_path`.
  - Important behavior: `MarkGradedProcessing` increments `graded_attempt_count`; `MarkGradedFailed` stores safe failure reason; `MarkGradedProcessed` marks `processed`.
  - Must preserve: `Identity(station_id, source_path, source_size_bytes)` duplicate safety, original-save normalization, and current persisted status values.
- `apps/agent/internal/processing/graded_processor.go`
  - Current state: `GradedProcessor.Process` verifies saved original, gets session snapshot, validates `.cube` LUT, computes deterministic graded path, marks processing, runs `LUTProcessor`, then marks processed/failed. `Reconcile` currently processes pending/processing/failed saved-original photos on startup.
  - Story change: add retry policy/manual retry orchestration around this processor, not a separate incompatible processor. Preserve original-first gating and safe failure strings.
  - High-risk detail: `writeGraded` currently refuses an existing deterministic graded target unless verified through `processed` state; do not weaken this because it prevents accepting stale/unverified outputs.
- `apps/agent/internal/processing/lut_processor.go`
  - Current state: ImageMagick 7 CLI adapter through `exec.CommandContext`; failures are persisted as retryable safe messages such as `LUT_PROCESSOR_UNAVAILABLE` and `LUT_PROCESSING_FAILED`.
  - Story change: retry should surface these existing safe messages; do not expose raw stderr or call ImageMagick from web.
- `apps/agent/internal/api/ingestion.go`
  - Current state: after original save, `enqueueGraded` publishes `photo.processing_started`, runs `GradedProcessor.Process` in a goroutine, then publishes `photo.processed`/`photo.processing_failed` and `queue.updated`; failures log `photo.processing_failed` activity.
  - Story change: hook automatic retry here or in reusable processing runner. Avoid duplicating retry/event code separately for ingestion and quarantine.
- `apps/agent/internal/api/quarantine.go`
  - Current state: manual quarantine assignment saves original then enqueues graded processing with parallel event behavior.
  - Story change: ensure assigned quarantined photos get the same automatic retry policy and manual retry endpoint works for them too.
- `apps/agent/internal/api/processing_queue.go`
  - Current state: `GET /api/processing/queue` provides read-only queue status and validates filters. Do not add retry side effects to this GET endpoint.
- `apps/agent/internal/processing/queue_status.go`
  - Current state: queue summary/items derive from `photos.Store`; `limit` only limits returned items, not summary counts. Preserve this review fix.
  - Story change: if a backend `retrying` status is introduced, update queue bucket logic and tests; otherwise keep retry visibility through `processing` plus `graded_attempt_count`.
- `apps/web/src/features/processing/processing-queue-panel.tsx`
  - Current state: shows queue summary, current/recent items, status labels, failure reason, and attempt count. This is the correct place for manual retry button.
- `apps/web/src/features/processing/use-processing-queue-query.ts`
  - Current state: query hook for `GET /api/processing/queue`. Add mutation separately; keep TanStack Query server-state ownership.
- `apps/web/src/lib/api/client.ts`
  - Current state: contains `ProcessingQueueItem`, `ProcessingQueueData`, validators, and `getProcessingQueue`. Extend it rather than creating a second API client.
- `docs/api/openapi.yaml`
  - Current state: documents processing queue endpoint and queue-related SSE events. Update the same contract for retry endpoint/events.

### Architecture Guardrails

- Go service owns all filesystem, image processing, retry orchestration, state transitions, activity logs, and SSE publication. Next.js only invokes API and renders server state.
- API responses must keep `{data}` wrapper; errors must keep `{error:{code,message,action,details}}`.
- DB/API JSON/status use `snake_case`; SSE event names use dot notation; status values must remain compatible with current code (`not_eligible`, `pending`, `processing`, `processed`, `failed`).
- Retry must be idempotent and duplicate-safe. The same `photo_id` must not create duplicate photo rows or multiple final graded paths.
- Retry must process from verified `local_original_path`, never from camera `source_path` as the only source.
- Manual retry is an operator action and must be logged. Activity log messages must be safe and avoid unnecessary customer/file path details.
- Processing/retry must remain asynchronous relative to dashboard/API commands; no long-running ImageMagick call in a request handler.
- Browser must never receive Supabase service role keys, Google credentials, or direct local filesystem authority.

### Recommended Implementation Shape

- Add or extend Go files:
  - `apps/agent/internal/processing/retry_policy.go` for retry eligibility, automatic-vs-manual rules, max attempts, and reusable error codes/messages.
  - `apps/agent/internal/processing/retry_policy_test.go`.
  - `apps/agent/internal/api/photo_retry.go` for `POST /api/photos/{photo_id}/retry-processing`.
  - `apps/agent/internal/api/photo_retry_test.go`.
- Prefer a reusable API helper/runner for processing events so ingestion, quarantine assignment, and manual retry all publish identical safe events.
- If adding an in-memory processing guard, key it by `photo_id`; release it on terminal success/failure and ensure tests cover concurrent duplicate manual retries.
- Web changes should be minimal and centered on existing processing feature files:
  - `apps/web/src/lib/api/client.ts`
  - `apps/web/src/features/processing/use-retry-processing-mutation.ts` or colocated mutation hook
  - `apps/web/src/features/processing/processing-queue-panel.tsx`
  - Existing dashboard SSE invalidation should already listen to processing/queue events from Story 5.3; extend only if new event names are added.

### Previous Story Intelligence

- Story 5.3 created the queue/status read model, API, SSE invalidation, OpenAPI docs, and operator UI. A high-priority review fix changed summary counts to be computed before `limit`; do not regress this.
- Story 5.2 implemented graded LUT processing and key safety rules: original-first gating, deterministic graded path, background processing goroutines, ImageMagick safe errors, and refusal to accept/overwrite pre-existing unverified graded outputs.
- Story 5.1 implemented original saver with collision-safe/idempotent behavior and byte-compare for same-size existing originals. Retry must rely on the saved original path, not redo source ingest.
- Story 4.4/4.5 established quarantine assignment/recovery and duplicate-safe persisted photo/quarantine state. Manual retry must work for photos assigned from quarantine without bypassing eligibility rules.
- Git history remains sparse (`Add from-scratch setup guide`, `Initial Selfstudio camera capture spike`); story artifacts and current workspace code are the authoritative implementation context.

### Latest Technical Notes

- Architecture lists observed tools: Go `1.26.3`, Next.js `16.2.6`, TanStack Query `5.100.10`, Supabase CLI `v2.98.1`, shadcn CLI v4. Actual workspace may use Next `^14.2.0`/React `^18.2.0`; follow the checked-in workspace unless an explicit upgrade story exists.
- Use Go `context.Context` and existing `exec.CommandContext`-based LUT adapter behavior; retries should preserve cancellation/timeouts rather than creating unmanaged processes.
- TanStack Query should invalidate/refetch queue data after retry mutation and SSE events; do not maintain a separate frontend state machine.

### Testing Requirements

Run at minimum:

- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build` if dependencies/environment allow.

Required coverage:

- Automatic retry attempts exactly up to 3 total graded attempts and then stops with failed status/actionable reason.
- Manual retry can restart a failed job from verified original and increments `graded_attempt_count` through existing store methods.
- Manual retry rejects non-existing, already processed, currently processing, not eligible, missing-original, invalid-original, or missing-session photos with correct error shape.
- Concurrent/duplicate manual retry calls for same `photo_id` do not start duplicate processor runs or create duplicate outputs.
- Retry events use `photo_id` as entity ID and do not leak `source_path`, `local_original_path`, `local_graded_path`, or LUT path as SSE entity IDs/activity identifiers.
- Web queue panel displays failed item retry action, pending mutation state, API error action text, and updated attempt count/failure reason after refetch.
- Existing ingestion/quarantine/original/graded/queue tests remain green.

### Regression Risks To Avoid

- Do not reset `graded_attempt_count` on retry unless there is a deliberate, tested manual retry semantics; automatic limit depends on this count.
- Do not mark failed photo as processed before verified graded output exists.
- Do not accept an existing graded file just because the path exists; keep verification and stale-output safety.
- Do not block `/api/photos/{photo_id}/retry-processing` until ImageMagick completes unless tests and UX explicitly prove it is fast; preferred response starts/queues retry and UI follows via SSE/refetch.
- Do not expose raw filesystem paths as SSE entity IDs or activity messages.
- Do not make the frontend invent `retrying` if backend does not persist/report it.
- Do not implement upload retry or Story 5.5 restart recovery in this story.

## Project Structure Notes

- Expected new files:
  - `apps/agent/internal/processing/retry_policy.go`
  - `apps/agent/internal/processing/retry_policy_test.go`
  - `apps/agent/internal/api/photo_retry.go`
  - `apps/agent/internal/api/photo_retry_test.go`
  - optional `apps/web/src/features/processing/use-retry-processing-mutation.ts`
- Expected modified files:
  - `apps/agent/cmd/selfstudio-agent/main.go` or current mux wiring file `apps/agent/internal/api/health.go`
  - `apps/agent/internal/api/ingestion.go`
  - `apps/agent/internal/api/quarantine.go`
  - `apps/agent/internal/processing/graded_processor.go` only if retry hooks/guards require it
  - `apps/agent/internal/processing/queue_status.go` only if backend exposes `retrying`
  - `apps/web/src/lib/api/client.ts`
  - `apps/web/src/features/processing/processing-queue-panel.tsx`
  - `docs/api/openapi.yaml`
- Runtime photo assets remain under configured local output folders/`local-data`; never commit generated JPGs or put them under `apps/web/public`.

## References

- `_bmad-output/planning-artifacts/epics.md` — Epic 5 and Story 5.4 acceptance criteria; FR36/FR37 mapping.
- `_bmad-output/planning-artifacts/prd.md` — Image Processing and Local Storage requirements; NFR3/NFR16/NFR18/NFR31.
- `_bmad-output/planning-artifacts/architecture.md` — Go service ownership, REST/SSE patterns, retry/idempotency, activity logging, queue isolation.
- `_bmad-output/implementation-artifacts/5-3-track-processing-queue-and-photo-status.md` — immediate previous story, queue API/UI/SSE patterns and review follow-up.
- `_bmad-output/implementation-artifacts/5-2-apply-station-session-lut-to-create-graded-jpg.md` — graded processing behavior and ImageMagick safety rules.
- `_bmad-output/implementation-artifacts/5-1-save-original-jpg-before-processing.md` — original-first save and collision-safe local original foundation.
- `apps/agent/internal/photos/store.go` — current photo processing fields and attempt-count mutation methods.
- `apps/agent/internal/processing/graded_processor.go` — current graded processing state transitions and `Reconcile` behavior.
- `apps/agent/internal/api/ingestion.go` — current background enqueue and safe processing SSE events.
- `apps/agent/internal/api/quarantine.go` — current assigned-photo processing path.
- `apps/agent/internal/api/processing_queue.go` — current read-only queue endpoint.
- `apps/web/src/features/processing/processing-queue-panel.tsx` — current operator queue UI to extend.
- `apps/web/src/lib/api/client.ts` — API client/types/validators to extend.
- `docs/api/openapi.yaml` — API/SSE contract source.

## Dev Agent Record

### Agent Model Used

GPT-5.1 Codex

### Debug Log References

- 2026-05-19: `cd apps/agent && go test ./...` failed once because `validOriginal` helper was redeclared; fixed by reusing existing processing helper.
- 2026-05-19: `cd apps/agent && go test ./...` failed once because test original size did not match `SourceSizeBytes`; fixed test fixture.
- 2026-05-19: `cd apps/agent && go test ./...` passed.
- 2026-05-19: `cd apps/web && npm run typecheck` passed.
- 2026-05-19: `cd apps/web && npm run build` passed.
- 2026-05-19: `cd apps/agent && go test ./...` passed after review follow-up fixes for shared automatic/manual guard, restart retry limit, and manual retry API/SSE/activity coverage.

### Completion Notes List

- Ultimate context engine analysis completed - comprehensive developer guide created.
- Added retry policy with `MaxGradedAttempts = 3`, automatic/manual eligibility distinction, actionable rejection errors, and in-memory per-photo processing guard.
- Added shared processing runner so ingestion, quarantine assignment, and manual retry use safe asynchronous processing events and automatic retry up to the total attempt limit.
- Added authenticated/trusted-origin manual retry endpoint `POST /api/photos/{photo_id}/retry-processing` returning `{data:{photo,retry_started:true}}` and logging safe operator activity.
- Updated queue UI with failed-photo Retry processing action, TanStack Query invalidation, pending/error display, attempt count, and actionable LUT/ImageMagick guidance.
- Updated OpenAPI contract with retry endpoint, response schema, auth/origin/error responses, retry event note, and attempt-count semantics.
- ✅ Resolved review finding [HIGH]: automatic/manual processing now share the same per-photo guard through ingestion, quarantine assignment, and manual retry wiring.
- ✅ Resolved review finding [HIGH]: restart reconciliation no longer automatically retries failed photos once `graded_attempt_count` reaches `MaxGradedAttempts`.
- ✅ Resolved review finding [MEDIUM]: added manual retry API coverage for auth/origin, `{data}` response and error shape, safe SSE IDs, activity logging without raw paths, and duplicate click concurrency.

### File List

- apps/agent/cmd/selfstudio-agent/main.go
- apps/agent/internal/api/health.go
- apps/agent/internal/api/ingestion.go
- apps/agent/internal/api/photo_retry.go
- apps/agent/internal/api/photo_retry_test.go
- apps/agent/internal/api/processing_runner.go
- apps/agent/internal/api/quarantine.go
- apps/agent/internal/processing/graded_processor.go
- apps/agent/internal/processing/graded_processor_test.go
- apps/agent/internal/processing/retry_policy.go
- apps/agent/internal/processing/retry_policy_test.go
- apps/web/src/features/processing/processing-queue-panel.tsx
- apps/web/src/features/processing/use-retry-processing-mutation.ts
- apps/web/src/lib/api/client.ts
- docs/api/openapi.yaml
- _bmad-output/implementation-artifacts/5-4-retry-failed-photo-processing.md
- _bmad-output/implementation-artifacts/sprint-status.yaml

## Change Log

- 2026-05-19: Ultimate context engine analysis completed - comprehensive developer guide created.
- 2026-05-19: Implemented Story 5.4 retry failed photo processing, added retry API/UI/docs/tests, and moved story to review.
- 2026-05-19: Addressed code review findings - 3 items resolved: shared processing guard, restart reconciliation automatic retry limit, and manual retry API/SSE/activity/concurrency test coverage.
