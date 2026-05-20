# Story 5.5: Recover Pending Processing Jobs After Restart

Status: done

## Story

Sebagai operator, saya ingin processing jobs yang pending/processing/retrying/failed dipulihkan setelah aplikasi restart, sehingga local delivery dapat lanjut dengan aman tanpa kehilangan original JPG, tanpa membuat graded output duplikat, dan tanpa menampilkan queue state yang menyesatkan.

## Acceptance Criteria

1. Given aplikasi restart saat photo/job berada pada `original_save_status` pending/saving/failed atau `graded_processing_status` pending/processing/failed, when Go service start kembali, then service reload persisted photo records dan menjalankan recovery/reconciliation untuk original save dan graded processing yang aman.
2. Given original JPG sudah ditandai `saved_original`, when startup recovery berjalan, then service memverifikasi `local_original_path` benar-benar ada, readable, bukan directory, dan size sesuai sebelum graded processing/retry dimulai.
3. Given photo/job ditandai `processed`, when startup recovery berjalan, then service memverifikasi `local_graded_path` benar-benar ada dan valid; jika missing/invalid, status menjadi failed dengan actionable reason, bukan dianggap sukses.
4. Given graded job berada pada `pending` atau `processing` sebelum restart dan original valid, when recovery berjalan, then job dikembalikan ke processing queue dan diproses secara asynchronous/non-blocking dengan event/status queue yang aman.
5. Given graded job berada pada `failed` dengan `graded_attempt_count < 3`, when recovery berjalan, then automatic retry boleh dilanjutkan sampai batas total automatic attempt 3; if `graded_attempt_count >= 3`, then recovery tidak menjalankan automatic retry dan dashboard tetap menunjukkan failed/manual action.
6. Given recovery dipanggil lebih dari sekali atau service restart berulang, then duplicate-safe job identity (`photo_id` + deterministic graded path) mencegah duplicate photo records, duplicate final outputs, dan concurrent double-processing untuk photo yang sama.
7. Given recovery mengubah atau memverifikasi status job, then system publishes safe SSE events (`queue.updated` plus relevant `photo.processing_started`, `photo.processed`, or `photo.processing_failed`) using safe IDs only and records concise recovery activity log without raw local file paths.
8. Given operator membuka processing queue setelah restart, then UI/API shows recovered queue state with text labels, attempt count, last failure reason, current job if any, and manual Retry processing action for failed retryable jobs.
9. Given existing ingestion, quarantine assignment, original-first save, LUT processing, retry, session detail, and queue/status behavior exist, then this story must not regress those flows or implement Epic 6 cloud upload recovery.
10. Tests/build pass for Go agent and relevant web/API contract checks.

## Tasks / Subtasks

- [x] Define explicit processing startup recovery contract and boundaries. (AC: 1, 4, 5, 6, 9)
  - [x] Treat this story as recovery hardening for Epic 5 only: photo original save + graded processing + queue/status after restart.
  - [x] Do not implement Google Cloud/upload recovery, remote reconciliation, or Drive/GCS state; those belong to Epic 6.
  - [x] Preserve current persisted statuses: original `pending|saving|saved_original|failed`, processing `not_eligible|eligible`, graded `not_eligible|pending|processing|processed|failed`.
  - [x] Define recovery outcomes in code comments/tests: verified success, resumed processing, failed with actionable reason, skipped because automatic retry limit reached.
- [x] Refactor startup reconciliation so pending processing is queued/asynchronous instead of blocking startup with direct ImageMagick calls. (AC: 1, 4, 8, 10)
  - [x] Current `cmd/selfstudio-agent/main.go` calls `originalSaver.ReconcilePending()` and `gradedProcessor.Reconcile(context.Background())` synchronously before HTTP server starts; evaluate and adjust so long-running graded processing does not block dashboard availability.
  - [x] Keep fast verification/metadata reconciliation on startup, but schedule processing attempts through the same shared `processingRunner`/`ProcessingGuard` path used by ingestion, quarantine assignment, and manual retry.
  - [x] Ensure queue API is available quickly after restart and shows jobs as pending/processing/failed based on persisted/recovered state.
  - [x] If retaining any synchronous startup work, tests must prove it is metadata/filesystem verification only and not a long LUT processor call.
- [x] Harden original-save reconciliation semantics. (AC: 1, 2, 6, 7)
  - [x] Read `apps/agent/internal/processing/original_saver.go` completely before editing.
  - [x] Preserve `OriginalSaver.Save` original-first behavior: copy from source to deterministic original path, byte-compare existing originals, atomic temp/rename behavior.
  - [x] For `saved_original`, verify `local_original_path` exists, is readable, non-empty, non-directory, and size matches `source_size_bytes`; mark failed if invalid.
  - [x] For `pending|saving|failed`, decide whether recovery can safely retry original save from `source_path`; if source missing/invalid, mark failed/actionable instead of creating dangling metadata.
  - [x] Persist recovered original status atomically with photo state.
- [x] Harden graded processing reconciliation semantics. (AC: 1, 3, 4, 5, 6)
  - [x] Read `apps/agent/internal/processing/graded_processor.go` and `retry_policy.go` completely before editing.
  - [x] For `processed`, keep verifying `local_graded_path`; mark failed with safe reason if missing/invalid.
  - [x] For `pending|processing`, only enqueue if original is verified saved and session snapshot/LUT/output path can be resolved by existing processor rules.
  - [x] For `failed`, enqueue automatic recovery retry only when `processing.IsAutomaticRetryEligible(photo)` returns true.
  - [x] Do not reset `graded_attempt_count` during recovery. Automatic retry limit must remain total attempts across restart.
  - [x] Do not accept/overwrite an existing deterministic graded target unless existing Story 5.2 verification rules explicitly allow it; preserve stale-output safety.
- [x] Reuse duplicate-safe processing guard across all recovery-triggered work. (AC: 4, 5, 6)
  - [x] Use the same `processingGuard := processing.NewProcessingGuard()` shared with ingestion/quarantine/manual retry wiring in `main.go`.
  - [x] Recovery-scheduled jobs must call the same guard path as manual/automatic processing so restart recovery cannot race with an operator manual retry after dashboard loads.
  - [x] Add concurrency tests where recovery enqueue and manual retry target the same `photo_id`; expected result: at most one processor run starts.
- [x] Publish safe SSE and activity for recovery. (AC: 7, 8)
  - [x] For resumed processing, reuse existing safe events from `processing_runner.go`: `photo.processing_started`, terminal `photo.processed`/`photo.processing_failed`, and `queue.updated`.
  - [x] Add a concise recovery summary event if useful, e.g. `processing.recovered` with counts only (`verified`, `resumed`, `failed`, `skipped_retry_limit`, `errors`) and entity ID `startup`; avoid file paths.
  - [x] Record operator-safe activity such as `processing.recovered` with counts; do not log `source_path`, `local_original_path`, `local_graded_path`, or LUT path.
  - [x] Avoid noisy per-attempt activity logs that flood operator log during large restart recovery; terminal repeated failures may log one concise action message.
- [x] Ensure queue/status API and UI accurately reflect recovered state. (AC: 8)
  - [x] Read `apps/agent/internal/api/processing_queue.go`, `apps/agent/internal/processing/queue_status.go`, and `apps/web/src/features/processing/processing-queue-panel.tsx` before editing.
  - [x] Preserve Story 5.3 fix: queue summary counts must be calculated before applying `limit`.
  - [x] Failed items after recovery limit reached must show retry count and actionable reason/manual retry CTA.
  - [x] Do not invent frontend-only `retrying` state unless backend actually reports it; frontend must render server state from TanStack Query.
  - [x] SSE invalidation should make queue refresh after recovery events; extend event subscriptions only if current handler misses new event names.
- [x] Update API/OpenAPI/docs contract for processing recovery behavior. (AC: 7, 8, 10)
  - [x] Update `docs/api/openapi.yaml` with any new `processing.recovered` SSE event and recovery count payload if implemented.
  - [x] Document startup recovery semantics near processing queue/retry docs: pending/processing resume, failed auto retry under limit, failed at limit stays manual.
  - [x] Keep API response and event payload conventions: `{data}` wrapper for REST, `{error:{code,message,action,details}}`, `snake_case` JSON fields, dot-notation SSE names.
- [x] Add focused tests and validation. (AC: 1-10)
  - [x] Go tests for original recovery: saved original missing/invalid becomes failed; pending/saving original recovers when source exists; missing source fails actionably.
  - [x] Go tests for graded recovery: processed missing graded output becomes failed; pending/processing with valid original enqueues/resumes; failed below attempt limit retries; failed at limit does not auto retry.
  - [x] Go tests for duplicate safety: repeated recovery run and recovery+manual retry cannot start two processors for same photo.
  - [x] Go tests for safe SSE/activity recovery payloads: IDs only, no raw paths/LUT in event entity/activity messages.
  - [x] Web/type tests or typecheck for queue rendering if UI/API types change.
  - [x] Run `cd apps/agent && go test ./...`.
  - [x] Run `cd apps/web && npm run typecheck`.
  - [x] Run `cd apps/web && npm run build` if environment allows.

### Review Follow-ups (AI)

- [x] [High] Make startup recovery truly non-blocking: `main.go` still calls `processingRecovery.Recover()` before constructing/starting the HTTP server, and `Recover()` calls `OriginalSaver.ReconcilePending()`, whose pending/saving/failed path calls `Save()` and performs filesystem copy/sync work synchronously before dashboard/API availability. This violates AC1/AC4 and the startup non-blocking guardrail; recovery should avoid long original copy/processing work on the pre-listen path or start the server before background recovery.
- [x] [High] Prevent recovery enqueue from racing ahead of HTTP server and SSE clients: `main.go` loops over `recoveryResult.EnqueueIDs` and calls `api.EnqueueProcessing(...)` before `server.ListenAndServe()`. The runner goroutines can start ImageMagick and publish `photo.processing_started` / terminal events before the queue API and browser SSE connection are available, weakening AC4/AC7/AC8 startup safety. Enqueue should happen after server startup or via a lifecycle-safe background scheduler.
- [x] [High] Preserve retry max semantics for failed jobs recovered below the limit: startup enqueues failed jobs with `manual=false`, and `processingRunner.runAttempts` loops automatic retries until ineligible. A failed photo with `graded_attempt_count == 2` can consume attempt 3 and then immediately publish failure; this matches the total max. However, a failed photo with `graded_attempt_count < 3` is counted as `resumed` without exposing skipped-after-terminal state during recovery, and the pre-listen enqueue makes those terminal events/activity easy to miss. Add focused tests for counts/events at attempt 2 and ensure the queue remains failed/manual at max without hidden additional attempts or missed queue refresh.
- [x] [Medium] Make recovery failure reasons actionable and specific: `OriginalSaver.ReconcilePending()` maps any invalid persisted original to `saved original missing or invalid after restart`, and `StartupRecovery` maps invalid processed graded output to a generic `graded output missing or invalid after restart; retry processing manually after verifying local output`. AC2/AC3 require verification of existence/readability/non-directory/size and actionable reason; preserving a safe category such as missing/unreadable/directory/size-mismatch would improve operator remediation without leaking raw paths.
- [x] [Medium] Strengthen transactionality around recovery state persistence: `StartupRecovery.Recover()` ignores `r.Processor.persist()` errors after marking graded processed records failed, while `OriginalSaver.ReconcilePending()` also ignores final `s.persist()` errors. AC7 and persistence guardrails require recovery status changes to be persisted atomically/truthfully; failed persistence should increment errors and avoid publishing success-like recovery summaries that may not match disk state.

## Dev Notes

### Source Requirements

- Epic 5 objective: original-first local delivery, LUT processing, processing status, retry, collision safety, queue/status visibility, and processing recovery.
- Story 5.5 from epics: reload job records after restart, reconcile local original/graded files with persisted state, prevent double output through duplicate-safe identity, and show recovered queue state.
- FR54: system can recover pending photo processing jobs after application restart.
- FR31-FR35: original and graded JPGs must be saved locally, original first, with processing status per photo.
- FR36-FR39: retry up to three automatic attempts, manual retry available, filename collisions prevented, missing/invalid LUT is failed/retryable.
- FR43/FR47/FR48: operator must see actionable processing problems, processing queue status, and activity logs.
- NFR3/NFR8/NFR9/NFR14/NFR16/NFR18/NFR31: processing must not block dashboard controls, original must be preserved before graded output, state must recover after restart, traceability must remain complete, retries must be duplicate-safe, persisted records must not point to missing final files, and errors must expose specific actions.

### Current Code Context To Read Before Editing

- `cmd/selfstudio-agent/main.go`
  - Current state: loads station/session/photo/quarantine stores, runs ingestion recovery, creates `OriginalSaver`, `GradedProcessor`, and shared `ProcessingGuard`, then calls `originalSaver.ReconcilePending()` and `gradedProcessor.Reconcile(context.Background())` before constructing/starting the HTTP server.
  - What this story changes: startup processing recovery should avoid long blocking LUT processing before dashboard/API are available; recovery work should be scheduled through the shared guarded runner where practical.
  - Must preserve: persistence setup, transactional `saveRuntimeState`, rollback behavior for API handlers, and the single shared `processingGuard` used by ingestion/quarantine/manual retry.
- `apps/agent/internal/processing/original_saver.go`
  - Current state: `Save` creates deterministic original path, copies original from `source_path`, marks original statuses, and `ReconcilePending` verifies saved originals or retries pending/saving/failed saves.
  - What this story changes: make recovery semantics explicit/tested and ensure invalid saved originals or missing source files become actionable failed states, not silent success/dangling paths.
  - Must preserve: original-first gating, collision-safe deterministic path, byte-compare existing original behavior, and no graded processing before verified original.
- `apps/agent/internal/processing/graded_processor.go`
  - Current state: `Process` verifies saved original, session snapshot, `.cube` LUT, deterministic graded path, marks processing, calls ImageMagick adapter, marks processed/failed. `Reconcile` verifies processed graded files, processes pending/processing saved originals, and retries failed saved-original photos only when `IsAutomaticRetryEligible`.
  - What this story changes: separate fast reconcile decisions from asynchronous job execution if needed; ensure recovery respects attempt limit and emits queue/activity events.
  - Must preserve: no retry from `source_path`, no processed status unless graded output exists, and no accepting stale existing deterministic graded targets.
- `apps/agent/internal/processing/retry_policy.go`
  - Current state: defines `MaxGradedAttempts = 3`, automatic/manual retry eligibility, and `ProcessingGuard` keyed by `photo_id`.
  - What this story changes: recovery must reuse the same policy/guard; do not duplicate attempt-limit logic elsewhere.
- `apps/agent/internal/api/processing_runner.go`
  - Current state: shared asynchronous runner publishes safe processing/queue SSE events, records safe activity, runs automatic retry loop with backoff, and uses `ProcessingGuard`.
  - What this story changes: recovery-scheduled processing should reuse this runner or an equivalent shared path.
  - Must preserve: safe payloads with `photo_id`, `station_id`, `session_id`; no raw paths as event entity IDs or activity identifiers.
- `apps/agent/internal/photos/store.go` and `persistence.go`
  - Current state: photo record contains `local_original_path`, `original_save_status`, `processing_status`, `local_graded_path`, `graded_processing_status`, `graded_last_error`, `graded_attempt_count`, timestamps, and LUT snapshot path. Persistence validates required fields and status enums, writes atomically to `local-data/state/photos.json`.
  - What this story changes: likely no schema migration needed; if fields are added, update validation/backward compatibility carefully.
  - Must preserve: `Identity(station_id, source_path, source_size_bytes)` duplicate safety, `photo_id` stability, and atomic state writes.
- `apps/agent/internal/api/processing_queue.go` and `apps/agent/internal/processing/queue_status.go`
  - Current state: read-only queue endpoint derives summary/items from photo store. Story 5.3 review fixed summary to count before `limit`.
  - What this story changes: ensure recovered queue state is accurately visible; avoid side effects in GET endpoint.
- `apps/web/src/features/processing/processing-queue-panel.tsx`
  - Current state: renders queue summary/items, failure reason, attempt count, and manual Retry processing action from Story 5.4.
  - What this story changes: ideally minimal; only adjust if recovery introduces new safe event/status display fields.
- `docs/api/openapi.yaml`
  - Current state: documents queue endpoint, manual retry endpoint, and processing SSE events.
  - What this story changes: document any processing recovery event/semantics added by this story.

### Architecture Guardrails

- Go service owns all filesystem, processing, retry, recovery, activity logs, SSE publication, and persisted state transitions. Next.js must not read local photo folders or invent recovery state.
- Supabase/GCS credentials and local filesystem authority must never move to browser/Next.js.
- REST responses use `{data}` wrapper; errors use `{error:{code,message,action,details}}`; DB/API JSON/status fields use `snake_case`; SSE event names use dot notation.
- Recovery must be idempotent. Re-running recovery after restart must not duplicate photo records, original paths, graded paths, attempts beyond policy, or activity spam.
- Recovery must not weaken Story 5.1/5.2 safety: original saved first; graded output only after verified original; stale existing graded target cannot be accepted casually.
- Startup recovery must leave the system usable end-to-end: dashboard loads, queue tells the truth, operator can retry/fix issues, and future ingestion/quarantine/manual retry still works.

### Recommended Implementation Shape

- Consider adding a focused recovery orchestrator rather than growing `main.go`:
  - `apps/agent/internal/processing/recovery.go` for `RecoveryService`/`RecoverOnStartup` that verifies records, returns counts, and optionally enqueues safe processing via callback.
  - `apps/agent/internal/processing/recovery_test.go` for table-driven startup recovery scenarios.
- If `processing_runner` is currently in package `api`, either:
  - expose a small recovery enqueue method in API wiring after server setup, or
  - move reusable runner logic into `internal/processing`/`internal/service` so recovery, API manual retry, ingestion, and quarantine can share it without import cycles.
- Keep `OriginalSaver.ReconcilePending` and `GradedProcessor.Reconcile` behavior covered by tests. If changing return semantics to “plan/enqueue” instead of “process now,” update tests and names to avoid misleading future agents.
- Prefer recovery summary counts over verbose per-file activity logs.

### Previous Story Intelligence

- Story 5.4 implemented retry policy and shared manual/automatic retry runner. Review follow-ups fixed two high-risk issues: automatic and manual processing now share a guard, and `GradedProcessor.Reconcile` no longer automatically retries failed photos after max attempts. Do not regress either fix.
- Story 5.4 also added manual retry API `POST /api/photos/{photo_id}/retry-processing`, safe retry SSE/activity behavior, and UI retry action in processing queue panel. Story 5.5 should reuse these instead of creating a second retry mechanism.
- Story 5.3 created queue/status read model, API, SSE invalidation, OpenAPI docs, and operator queue UI. A review fix made summary counts independent of returned item limit; preserve this.
- Story 5.2 implemented graded LUT processing with ImageMagick CLI adapter, deterministic graded path, safe `.cube` LUT validation, background processing, and refusal to accept/overwrite unverified existing graded output. Recovery must not bypass those checks.
- Story 5.1 implemented original saver with collision-safe/idempotent behavior and byte-compare for existing originals. Recovery must trust only verified saved originals and must not process graded output from camera source path alone.
- Story 4.5 already handled ingestion/quarantine recovery. Do not reimplement routing scan in this story except where needed to understand pending routed photo records.
- Git history remains sparse (`Add from-scratch setup guide`, `Initial Selfstudio camera capture spike`); current workspace code and story artifacts are authoritative.

### Latest Technical Notes

- No new external library is required for Story 5.5. Use existing Go `context.Context`, goroutine, timer/backoff, filesystem, and persistence patterns.
- Architecture lists observed tools: Go `1.26.3`, Next.js `16.2.6`, TanStack Query `5.100.10`, Supabase CLI `v2.98.1`, shadcn CLI v4. Actual workspace may use older checked-in frontend dependencies; follow `apps/web/package.json` unless a separate upgrade story exists.
- ImageMagick interaction must remain inside `apps/agent/internal/processing/lut_processor.go`/Go service. Do not call ImageMagick from frontend or during queue GET.
- Use atomic persistence already present in `photos.Persistence.Save`; do not write ad-hoc JSON state files for recovery.

### Testing Requirements

Run at minimum:

- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build` if dependencies/environment allow.

Required coverage:

- Startup recovery verifies `saved_original` local original and fails it if missing/invalid.
- Startup recovery handles original pending/saving/failed with existing source present vs source missing.
- Startup recovery verifies `processed` graded output and marks failed if missing/invalid.
- Startup recovery resumes/enqueues `pending` and stale `processing` graded jobs only after verified original.
- Startup recovery retries failed graded jobs below `MaxGradedAttempts` but skips failed jobs at/above limit.
- Repeated recovery and recovery+manual retry are duplicate-safe via shared guard.
- Recovery events/activity contain safe IDs/counts only and do not leak raw filesystem/LUT paths.
- Queue API/UI after recovery shows accurate status, attempt count, failure reason, and Retry processing CTA for failed eligible/manual retry cases.

### Regression Risks To Avoid

- Do not run long ImageMagick processing synchronously before the HTTP server starts if it delays operator dashboard recovery.
- Do not reset `graded_attempt_count` on restart.
- Do not auto retry failed jobs after `MaxGradedAttempts` without manual operator action.
- Do not mark missing original/graded files as successful just because metadata says saved/processed.
- Do not process from `source_path` as a substitute for verified `local_original_path` during graded recovery.
- Do not accept or overwrite stale graded output at deterministic path without existing verification rules.
- Do not create duplicate photo records during recovery scan/reconcile.
- Do not leak customer/file paths in SSE entity IDs or activity messages.
- Do not implement Epic 6 cloud upload recovery in this story.

## Project Structure Notes

Expected new files:

- `apps/agent/internal/processing/recovery.go` (recommended, if extracting recovery orchestration)
- `apps/agent/internal/processing/recovery_test.go` (recommended)

Expected modified files:

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/processing/original_saver.go`
- `apps/agent/internal/processing/graded_processor.go`
- `apps/agent/internal/processing/retry_policy.go` only if recovery policy needs explicit helpers
- `apps/agent/internal/api/processing_runner.go` or a moved/shared processing runner if needed for recovery reuse
- `apps/agent/internal/processing/queue_status.go` only if new backend-reported status/counts are added
- `apps/web/src/features/processing/processing-queue-panel.tsx` only if display fields/events change
- `apps/web/src/lib/api/client.ts` only if API response/types change
- `docs/api/openapi.yaml`

Runtime photo assets remain under configured local output folders/`local-data`; never commit generated JPGs or put them under `apps/web/public`.

## References

- `_bmad-output/planning-artifacts/epics.md` — Epic 5 and Story 5.5 acceptance criteria; FR54 mapping.
- `_bmad-output/planning-artifacts/prd.md` — Image Processing and Local Storage; Readiness, Health, and Recovery; NFR9/NFR16/NFR18.
- `_bmad-output/planning-artifacts/architecture.md` — Go service ownership, retry/idempotency, recovery, REST/SSE/event contracts, file structure.
- `_bmad-output/implementation-artifacts/5-4-retry-failed-photo-processing.md` — immediate previous story, retry policy/runner/manual retry API/UI and review follow-ups.
- `_bmad-output/implementation-artifacts/5-3-track-processing-queue-and-photo-status.md` — processing queue API/UI/SSE patterns and summary-count review fix.
- `_bmad-output/implementation-artifacts/5-2-apply-station-session-lut-to-create-graded-jpg.md` — graded processing, deterministic output, ImageMagick/LUT safety.
- `_bmad-output/implementation-artifacts/5-1-save-original-jpg-before-processing.md` — original-first save and collision-safe local original foundation.
- `_bmad-output/implementation-artifacts/4-5-recover-pending-ingestion-and-quarantine-state.md` — existing ingestion/quarantine recovery patterns.
- `apps/agent/cmd/selfstudio-agent/main.go` — startup recovery and service wiring.
- `apps/agent/internal/processing/original_saver.go` — original save and original reconciliation.
- `apps/agent/internal/processing/graded_processor.go` — graded processing and current reconcile behavior.
- `apps/agent/internal/processing/retry_policy.go` — max attempts and processing guard.
- `apps/agent/internal/api/processing_runner.go` — async processing runner, SSE, activity, automatic retry loop.
- `apps/agent/internal/photos/store.go` and `apps/agent/internal/photos/persistence.go` — persisted photo state and validation.
- `apps/agent/internal/processing/queue_status.go` — queue status derivation.
- `apps/web/src/features/processing/processing-queue-panel.tsx` — operator queue UI.
- `docs/api/openapi.yaml` — API/SSE contract source.

## Dev Agent Record

### Agent Model Used

OpenAI GPT-5.1 Codex CLI

### Debug Log References

- `cd apps/agent && go test ./...` — PASS
- `cd apps/web && npm run typecheck` — PASS
- `cd apps/web && npm run build` — PASS
- `cd apps/agent && go test ./...` — PASS (review follow-ups)
- `cd apps/web && npm run typecheck` — PASS (review follow-ups)
- `cd apps/web && npm run build` — PASS (review follow-ups)

### Completion Notes List

- Ultimate context engine analysis completed - comprehensive developer guide created.
- Menambahkan startup processing recovery khusus Epic 5 dengan outcome eksplisit: verified, resumed, failed actionable, dan skipped retry limit.
- Startup recovery sekarang hanya melakukan verifikasi/metadata cepat lalu menjadwalkan graded jobs lewat shared async processing runner dan shared `ProcessingGuard`, bukan menjalankan LUT/ImageMagick langsung saat startup.
- Original recovery memperketat validasi `saved_original` agar file harus ada, readable, non-directory, non-empty, dan size sesuai `source_size_bytes`; pending/saving/failed original dicoba ulang dari source dan gagal actionably jika source invalid.
- Graded recovery memverifikasi output `processed`, enqueue pending/processing/failed-under-limit hanya bila original valid, tidak reset attempt count, dan tetap memakai stale-output safety di processor existing.
- Menambahkan event/activity aman `processing.recovered` berisi count saja, serta invalidasi frontend untuk recovery/queue startup events.
- OpenAPI diperbarui untuk event recovery dan semantik queue setelah restart.
- ✅ Resolved review finding [High]: Startup recovery sekarang berjalan setelah listener/server aktif, sehingga copy/reconcile original tidak memblok dashboard/API availability.
- ✅ Resolved review finding [High]: Recovery enqueue dipindahkan setelah server startup path aktif agar runner/SSE events tidak race sebelum API tersedia.
- ✅ Resolved review finding [High]: Menambahkan test attempt-count recovery pada `graded_attempt_count == 2`; automatic recovery hanya memakai satu attempt tersisa, publish queue refresh, lalu state tetap failed/manual at max.
- ✅ Resolved review finding [Medium]: Reason recovery original/graded kini menyebut kategori aman dan actionable seperti missing/directory/size mismatch/unreadable tanpa raw path di event/activity summary.
- ✅ Resolved review finding [Medium]: Persist error saat recovery failure sekarang dihitung sebagai error dan tidak disembunyikan oleh summary sukses-like.

### File List

- apps/agent/cmd/selfstudio-agent/main.go
- apps/agent/internal/api/processing_runner.go
- apps/agent/internal/api/processing_runner_recovery_test.go
- apps/agent/internal/processing/original_saver.go
- apps/agent/internal/processing/recovery.go
- apps/agent/internal/processing/recovery_test.go
- apps/web/src/features/health/health-dashboard.tsx
- docs/api/openapi.yaml
- _bmad-output/implementation-artifacts/5-5-recover-pending-processing-jobs-after-restart.md
- _bmad-output/implementation-artifacts/sprint-status.yaml

## Change Log

- 2026-05-19: Ultimate context engine analysis completed - comprehensive developer guide created.
- 2026-05-19: Implemented Epic 5 startup processing recovery, async guarded resume/retry scheduling, safe recovery events/activity, queue UI invalidation, OpenAPI docs, and validation tests.
- 2026-05-19: Addressed code review findings - 5 items resolved; startup recovery/enqueue now post-listen, retry max semantics covered, failure reasons are specific/actionable, and persistence errors are reported truthfully.
