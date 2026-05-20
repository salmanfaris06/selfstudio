# Story 6.5: Recover Pending Cloud Upload Jobs After Restart

Status: done

## Story

Sebagai operator, saya ingin pending cloud upload jobs dipulihkan setelah aplikasi restart, sehingga fulfillment cloud dapat berlanjut dengan aman tanpa duplicate upload, tanpa mengganggu capture/local processing, dan tanpa mencampur status local complete dengan status cloud.

## Acceptance Criteria

1. Given aplikasi restart saat upload job berada di status `pending`, `uploading`, `retrying`, `retry_scheduled`, `failed`, atau partial complete, when Go agent startup, then sistem memuat `upload_jobs.json`, menormalkan status in-flight lama, dan menampilkan queue/session upload state yang jujur tanpa menghapus local files.
2. Given persisted job sebelumnya `uploading` atau `retrying` saat proses mati, when startup recovery berjalan, then job tidak dianggap uploaded; job dikembalikan ke status recoverable (`pending` atau `retry_scheduled`/`failed` dengan safe action) berdasarkan local file dan remote identity yang tersedia.
3. Given persisted job sudah `uploaded` dengan `remote_identity`/generation metadata, when startup recovery berjalan, then job tetap `uploaded` hanya bila metadata cukup aman; jika remote verification tidak bisa dilakukan atau konflik ditemukan, status menjadi actionable safe state (`failed`/`CHECK_CLOUD_OBJECT` atau retryable sesuai policy) tanpa re-upload otomatis berbahaya.
4. Given job belum uploaded dan local original/graded file masih ada, when startup recovery selesai, then upload dapat lanjut asynchronous memakai `job_id`, `object_key`, `bucket_name`, `local_path`, `dedupe_key`, dan retry policy yang sudah ada dari Story 6.4; object key tidak berubah dan tidak membuat duplicate remote object.
5. Given local file untuk pending/failed job hilang setelah restart, when recovery merekonsiliasi job, then job menjadi `failed` dengan `UPLOAD_LOCAL_FILE_MISSING` dan action `CHECK_LOCAL_OUTPUT`; session upload status tetap terpisah dari local save/processing status dan tidak menghapus metadata lain.
6. Given sebagian jobs uploaded dan sebagian pending/failed setelah restart, when API/UI/SSE menampilkan status, then session cloud status menunjukkan `pending`/`uploading`/`partial_failed`/`failed`/`uploaded` secara jujur dan dashboard tetap menampilkan local complete separately.
7. Given recovery memperbarui job state, when durable save berhasil, then sistem publish safe SSE/activity seperti `upload.recovered` dan/atau `upload.session_updated`; if save gagal, then jangan publish success/recovered event dan jangan klaim recovery selesai.
8. Given recovery berjalan saat startup, when upload worker melanjutkan pending jobs, then capture, station/session controls, ingestion, local original save, LUT processing, quarantine, dan processing retry tetap tersedia; upload recovery tidak boleh blocking main request path.
9. Tests/build pass untuk Go agent, web typecheck/build yang relevan, dan OpenAPI/API contract diperbarui untuk startup recovery event/status.

## Tasks / Subtasks

- [x] Bangun service startup recovery khusus cloud upload. (AC: 1, 2, 4, 5, 7, 8)
  - [x] Tambahkan `apps/agent/internal/upload/recovery.go` atau file setara; jangan gabungkan dengan `internal/processing/recovery.go` karena lifecycle remote identity berbeda.
  - [x] Input service: `JobsStore`, `JobsPersistence`, `Uploader`/remote verifier capability, `events.Broker`, `activity.Store`, clock/test hooks.
  - [x] Recovery harus idempotent: running twice after same state must not create duplicate uploads or duplicate job records.
  - [x] Persist all state normalization before publishing SSE/activity or enqueueing resumed upload.
- [x] Normalisasi persisted in-flight upload statuses saat startup. (AC: 1, 2, 6)
  - [x] Treat stale `uploading` and `retrying` as interrupted, not successful.
  - [x] Convert interrupted jobs to safe retryable state using existing retry policy: likely `retry_scheduled` with `next_retry_at` now/soon if retryable and attempts remain; otherwise `failed` with `RETRY_CLOUD_UPLOAD`.
  - [x] Preserve `attempt_count`, `last_attempt_at`, `max_attempts`, `last_error_code`, `last_error_action`, and remote metadata; do not reset history to hide failures.
  - [x] Do not mutate `not_eligible` jobs except to validate they remain harmless.
- [x] Reconcile local file availability for all non-uploaded eligible jobs. (AC: 4, 5)
  - [x] Use existing `readable(local_path)` semantics or extracted helper; do not expose full path to UI/API/SSE.
  - [x] If local file missing, mark job `failed`, `last_error_code=UPLOAD_LOCAL_FILE_MISSING`, `last_error_action=CHECK_LOCAL_OUTPUT`, clear `next_retry_at` unless manual fix needed.
  - [x] If local file present and job is pending/retryable, keep same deterministic `job_id` and `object_key`; never rebuild object key from current customer/order/station config.
- [x] Add remote identity/status reconciliation guard. (AC: 3, 4)
  - [x] Extend `Uploader` interface or add small verifier interface only if needed, e.g. `Stat(ctx,bucket,object_key)` returning safe remote generation/metageneration/etag/identity.
  - [x] For uploaded jobs with persisted `remote_generation`/`remote_identity`, verify remote object when possible; if verifier unavailable, keep persisted uploaded state but log/recover with “unverified persisted uploaded” safe metadata, not a success re-upload.
  - [x] For precondition conflict/already-exists during resumed upload, reuse Story 6.4 mapping (`UPLOAD_OBJECT_CHECK_NEEDED`/`CHECK_CLOUD_OBJECT`) instead of infinite retry or overwrite.
  - [x] Do not overwrite, delete, rename, or suffix remote objects during recovery.
- [x] Resume eligible jobs asynchronously using existing Worker guard. (AC: 4, 8)
  - [x] Add method such as `Worker.RecoverAndResume(ctx, now)` or `Recovery.Recover(...).EnqueueIDs` similar to processing recovery, but ensure upload `guard` is shared with manual/API retry and auto retry scheduler.
  - [x] Recovery startup may enqueue pending/retryable jobs after durable save, but must not block HTTP server startup for long uploads.
  - [x] If a job is already `uploaded`, no-op; if `uploading/retrying` was interrupted, only one goroutine may resume per `job_id`.
  - [x] Preserve queue isolation: do not call watcher, ingestion router, processing runner, local save, or session mutation logic.
- [x] Publish safe recovery SSE/activity and dashboard invalidation. (AC: 6, 7)
  - [x] Suggested event: `upload.recovered` with `entity_type: "upload"` or `"startup"`, safe IDs/counts only: recovered_pending, resumed, failed_missing_local, verified_uploaded, requires_cloud_check, errors.
  - [x] Continue publishing `upload.session_updated`/per-job events only after durable save.
  - [x] Activity actions: `cloud.upload_recovery_completed`, `cloud.upload_recovery_failed`, optional `cloud.upload_recovery_job_failed`; include safe session/station refs only.
  - [x] Update `apps/web/src/features/health/health-dashboard.tsx` only if current `upload.*` event handler does not already invalidate session/upload/activity for `upload.recovered`.
- [x] Update API/OpenAPI and safe DTO docs. (AC: 6, 7, 9)
  - [x] Update `docs/api/openapi.yaml` SSE examples/schema to include `upload.recovered` and recovery summary payload.
  - [x] Ensure `SessionUploadsResponse` still excludes `local_path`; expose only `local_file_name` already established in Story 6.4.
  - [x] Document status normalization semantics for restart: stale `uploading/retrying` are interrupted and recoverable, never assumed uploaded.
- [x] Add tests and run validation. (AC: 1-9)
  - [x] Go unit tests for recovery normalization: pending remains pending/resumable; uploading/retrying becomes retryable; failed stays actionable; uploaded preserved/verified; not_eligible ignored.
  - [x] Go tests for local missing file mapping to `UPLOAD_LOCAL_FILE_MISSING`/`CHECK_LOCAL_OUTPUT` and aggregate `partial_failed` behavior.
  - [x] Go tests for no duplicate resume: recovery + auto retry/manual retry concurrent path calls uploader at most once for same `job_id`.
  - [x] Go tests for durable save failure: no `upload.recovered` success event/activity and no resumed goroutine.
  - [x] API/SSE tests if handler/event schema changes.
  - [x] Run `cd apps/agent && go test ./...`.
  - [x] Run `cd apps/web && npm run typecheck`.
  - [x] Run `cd apps/web && npm run build` if dependencies/environment allow.

### Review Follow-ups (AI)

- [x] [High] Pending jobs are counted for recovery resume but never actually resume — `StartupRecovery.shouldResumeAfterRecovery` adds `pending` jobs to `EnqueueIDs`, but `Worker.RetryJob(ctx, id, false)` rejects pending jobs because `CanAutoRetry` only allows `failed` or `retry_scheduled` with retryable error metadata. This violates AC4/AC8 for persisted `pending` jobs after restart; normalize pending into an accepted resume path or add a dedicated guarded recovery resume that can start pending jobs after durable save without duplicate uploads. Evidence: `apps/agent/internal/upload/recovery.go` enqueues `JobStatusPending`; `apps/agent/internal/upload/retry_runner.go` rejects non-manual pending jobs via `CanAutoRetry`.
- [x] [Medium] Uploaded job remote verification is not wired in production — `StartupRecovery.Verifier` is left nil in `main.go`, and neither `GCSUploader` nor `TimeoutUploader` exposes `Stat`, so uploaded jobs with metadata are always treated as `VerifiedUploaded` without remote verification or explicit unverified recovery metadata/logging. This weakens AC3 and the remote identity reconciliation task. Add a production GCS stat/verifier path with timeout, or rename/report the summary as unverified when verifier is unavailable.

## Dev Notes

### Source Requirements

- Epic 6 objective: post-session cloud fulfillment using architecture-selected GCS, upload tracking, retry failure, duplicate prevention, restart recovery, and non-blocking local operation.
- Story 6.5 from epics: reload upload queue after restart; reconcile remote identity/status enough to reduce duplicates; dashboard shows upload pending/uploaded/failed separately from local complete; capture/local processing remain available while upload recovers.
- FR55: system can recover pending Google Drive/cloud upload jobs after application restart.
- FR59-FR63: track cloud upload per session/file, retry failed uploads, preserve local files, prevent cloud upload from blocking capture, prevent duplicate uploads using tracked identity/status.
- NFR4/NFR9/NFR11/NFR16/NFR25/NFR26/NFR32: upload must not degrade capture/local processing, restart state recoverable, local save independent from cloud success, retries duplicate-safe, cloud failure scenarios handled, remote status tracked enough to reduce duplicates, local-vs-cloud status clearly separated.

### Current Code Context To Read Before Editing

- `apps/agent/internal/upload/jobs.go`
  - Current state: `FileUploadJob` has deterministic `JobID(session_id, photo_id, asset_kind)`, `local_path` internal, `object_key`, `remote_identity`, `remote_generation`, retry fields, statuses `pending/uploading/uploaded/failed/not_eligible/retrying/retry_scheduled`.
  - Change: recovery must interpret stale persisted statuses safely.
  - Preserve: deterministic identity, no credential fields, no duplicate job for same asset.
- `apps/agent/internal/upload/jobs_persistence.go`
  - Current state: atomic JSON persistence at `local-data/state/upload_jobs.json`, version 1, rejects invalid/duplicate jobs, `AggregateUploadStatus` already understands retry statuses.
  - Change: recovery should use this persistence and fail safe on save/load errors.
  - Preserve: corrupt invalid state must stop startup/fail safe; save before event/success.
- `apps/agent/internal/upload/worker.go`
  - Current state: `StartSession` discovers upload jobs after locked local completion; `uploadOne` persists uploading before publish, preserves local files, maps GCS errors, schedules retry, publishes only after durable save; worker has shared pointer `guard`.
  - Change: add recovery resume entrypoint or call retry/resume through existing methods.
  - Preserve: non-blocking upload; no local processing/session mutation; durable publish guard.
- `apps/agent/internal/upload/retry_runner.go`
  - Current state: shared `uploadGuard`, `RetryJob`, `RetrySession`, and `AutoRetryDue`; manual/API and scheduler should use one shared `*Worker`.
  - Change: startup recovery must use same guard, not a copied worker.
  - Preserve: double trigger must not produce parallel upload for same job.
- `apps/agent/internal/upload/retry_policy.go`
  - Current state: retry classification/max 3 auto attempts/backoff and manual retry rules from Story 6.4.
  - Change: recovery should reuse these policies; do not invent a second cloud retry policy.
- `apps/agent/internal/upload/gcs_uploader.go` and `uploader.go`
  - Current state: production GCS uploader uses server-side settings, deterministic object key, precondition/idempotency behavior, safe error mapping; `LocalCopyUploader` is test-only.
  - Change: add stat/verify capability only if necessary and keep fakes simple.
  - Preserve: no raw privileged errors or credential leakage; no production fake uploader.
- `apps/agent/internal/api/session_uploads.go`
  - Current state: safe upload DTO excludes full `local_path` and exposes only `local_file_name`; consumes worker events and publishes `upload.*` SSE/activity.
  - Change: add/reuse recovery events and safe activity logging.
  - Preserve: `RequireAuth`/`RequireTrustedOrigin` for mutations; safe response wrapper and error shape.
- `apps/agent/cmd/selfstudio-agent/main.go`
  - Current state: loads sessions/photos/quarantine/upload targets/upload jobs/cloud settings; creates shared `*upload.Worker`; starts auto retry scheduler; processing recovery runs after server start.
  - Change: wire upload startup recovery after `jobsStore`/`uploadWorker` initialization and before/around scheduler/server start with clear non-blocking behavior.
  - Preserve: one shared worker instance across scheduler/API/recovery; startup should fail safe on corrupt upload job persistence.
- `apps/web/src/features/health/health-dashboard.tsx`
  - Current state: `handleUploadUpdated` invalidates sessions/session-detail/session-uploads/activity for any `upload.*` event.
  - Change: likely no change required for `upload.recovered`; verify and avoid unnecessary processing invalidations.
- `docs/api/openapi.yaml`
  - Current state: documents upload status/start/retry endpoints and SSE examples; notes restart recovery was Story 6.5.
  - Change: document recovery event/status semantics and remove outdated “future scope” wording where applicable.

### Architecture Guardrails

- Go service owns upload recovery, remote reconciliation, persistence, worker resume, SSE/activity, and all credential use.
- Next.js/browser must never receive service account JSON, private key, OAuth/ADC token, credential path, Supabase service role, raw GCS error, or full local customer filesystem path.
- API JSON/status fields use `snake_case`; success responses use `{data}`; errors use `{error:{code,message,action,details}}`; SSE events use dot notation.
- Local filesystem remains safety source. Recovery must never delete, move, rewrite, or reprocess local original/graded files.
- Provider remains Google Cloud Storage, not Google Drive API, despite product wording “Drive/Cloud”.
- Upload recovery must not block or call capture/session routing/processing/quarantine flows.
- Do not create duplicate remote objects by changing object keys, appending suffixes, or overwriting without identity/precondition checks.
- Do not mark session `uploaded` while any eligible job is pending/retrying/failed/requires cloud check.

### Previous Story Intelligence

- Story 6.4 completed retry/de-dup, manual retry API/UI, GCS preconditions, safe DTOs, and full tests. Reuse it rather than building another queue/retry system.
- Critical Story 6.4 review lesson: never copy `upload.Worker` by value because guard copy permits duplicate parallel uploads. Use one shared `*upload.Worker` end-to-end for recovery, auto retry scheduler, and API/manual retry.
- Story 6.4 safe payload fix: API/SSE/UI must not expose `local_path`; keep `local_file_name` only.
- Story 6.4 GCS lesson: precondition/already-exists conflict must map to `UPLOAD_OBJECT_CHECK_NEEDED`/`CHECK_CLOUD_OBJECT`, not endless `RETRY_CLOUD_UPLOAD`.
- Story 6.3 established upload jobs discovery and non-blocking background upload after locked local completion. Recovery should continue persisted jobs, not rediscover by scanning output folders freely.
- Story 6.2 fixed locked session visibility; do not hide locked/completed sessions when adding recovery state.
- Story 5.5/processing recovery pattern can inspire structure, but upload recovery must remain separate because remote identity and GCS object safety differ.

### Latest Technical Notes

- Current Google Cloud Storage docs state preconditions make requests proceed only when the resource matches expected conditions and help prevent race conditions in uploads/deletes/metadata updates. Use generation preconditions/`DoesNotExist` or equivalent for retry-safe uploads.
- GCS retry docs classify some operations as conditionally idempotent only when preconditions or ETags are used. Do not rely on blind retry without precondition.
- GCS objects expose generation and metageneration; generation changes on content changes and metageneration changes on metadata changes. Persisted `remote_generation`/`remote_metageneration`/ETag are useful for safe reconciliation.
- `cloud.google.com/go/storage` latest docs observed v1.61.3; do not upgrade dependency unless necessary. If dependency changes, document reason and run full Go suite.

### Testing Requirements

Minimum commands:

- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build` if environment/dependencies allow

Required coverage:

- Stale `uploading`/`retrying` after restart never becomes uploaded automatically.
- Pending/retry_scheduled jobs resume with same deterministic `job_id`, `object_key`, `bucket_name`, and `local_path`.
- Uploaded jobs with remote metadata are preserved/verified without re-upload; conflict maps to safe check action.
- Missing local file maps to `UPLOAD_LOCAL_FILE_MISSING`/`CHECK_LOCAL_OUTPUT`.
- Save failure prevents success SSE/activity and prevents background resume.
- Recovery + scheduler + manual retry concurrency uses one guard and at most one uploader call per `job_id`.
- Session aggregate status remains truthful for partial uploaded/failed/pending states.
- API/SSE payloads do not expose full `local_path` or credentials.

### Regression Risks To Avoid

- Do not reintroduce production `LocalCopyUploader` or fake cloud success.
- Do not republish `upload.file_uploaded` before durable save.
- Do not create a new job row for existing `(session_id, photo_id, asset_kind)`.
- Do not start recovery uploads before normalizing and saving interrupted state.
- Do not overwrite remote objects or generate duplicate suffix objects.
- Do not reset attempt/error history to make recovery look cleaner.
- Do not block agent startup indefinitely on remote verification or large uploads; use timeouts/context.
- Do not invalidate processing queue in frontend for upload-only recovery unless processing actually changes.

## Project Structure Notes

Expected new files:

- `apps/agent/internal/upload/recovery.go`
- `apps/agent/internal/upload/recovery_test.go`

Expected modified files:

- `apps/agent/internal/upload/worker.go`
- `apps/agent/internal/upload/uploader.go` and/or `gcs_uploader.go` if remote verification interface is added
- `apps/agent/internal/upload/retry_runner.go` only if recovery must call shared guard helpers
- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/session_uploads.go` if new recovery event/activity mapping is needed
- `apps/web/src/features/health/health-dashboard.tsx` only if `upload.*` invalidation is insufficient
- `docs/api/openapi.yaml`

Runtime state remains under `local-data/state` and must not be committed.

## References

- `_bmad-output/planning-artifacts/epics.md` — Epic 6 and Story 6.5 AC; FR55/FR59-FR63 mapping.
- `_bmad-output/planning-artifacts/prd.md` — restart recovery, cloud fulfillment, local-vs-cloud separation, GCS/Drive failure requirements; NFR4/NFR9/NFR16/NFR25/NFR26/NFR32.
- `_bmad-output/planning-artifacts/architecture.md` — Go owns workers/credentials/filesystem, GCS selected, API/SSE/error patterns, retry/idempotency, project structure.
- `_bmad-output/implementation-artifacts/6-1-configure-cloud-storage-credentials-and-target-rules.md` — credential safety, GCS settings, object key sanitizer.
- `_bmad-output/implementation-artifacts/6-2-create-cloud-folder-object-structure-per-session.md` — session cloud target prefix and locked session visibility lesson.
- `_bmad-output/implementation-artifacts/6-3-upload-original-and-graded-jpgs-after-local-completion.md` — upload job discovery, non-blocking worker, persistence and SSE patterns.
- `_bmad-output/implementation-artifacts/6-4-track-retry-and-de-duplicate-cloud-uploads.md` — retry/de-dup implementation, review follow-ups, guard concurrency, safe DTO, GCS precondition conflict mapping.
- `apps/agent/internal/upload/jobs.go` — upload job identity/status model.
- `apps/agent/internal/upload/jobs_persistence.go` — upload job persistence and aggregate status.
- `apps/agent/internal/upload/worker.go` — upload start/run flow.
- `apps/agent/internal/upload/retry_runner.go` — shared duplicate guard and retry scheduler.
- `apps/agent/internal/api/session_uploads.go` — safe upload API DTOs and SSE/activity publisher.
- `apps/agent/cmd/selfstudio-agent/main.go` — startup wiring and shared worker instance.
- Google Cloud Storage docs: request preconditions, retry strategy, Go storage package docs (accessed 2026-05-19).

## Dev Agent Record

### Agent Model Used

OpenAI GPT-5.1 Codex Max

### Debug Log References

- 2026-05-19: Implemented `upload.StartupRecovery` with durable normalization before recovery event/activity and resume.
- 2026-05-19: Reworked upload retry guard path so recovery/manual/auto retry share one worker guard without copying worker state.
- 2026-05-19: Added upload recovery tests for interrupted jobs, missing local files, uploaded metadata guard, save failure, and duplicate prevention.
- 2026-05-19: Validation passed: `cd apps/agent && go test ./...`; `cd apps/web && npm run typecheck`; `cd apps/web && npm run build`.
- 2026-05-19: Addressed review follow-up: pending startup recovery now uses `Worker.ResumeRecoveredJob` so durable pending jobs can start through the shared upload guard without weakening manual/auto retry semantics.
- 2026-05-19: Addressed review follow-up: production GCS uploader and timeout wrapper now expose `Stat`, startup recovery wires the verifier, and unavailable verifier counts as `unverified_uploaded` rather than verified.
- 2026-05-19: Validation passed after follow-up fixes: `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`; `cd apps/web && npm run typecheck`; `cd apps/web && npm run build`.

### Completion Notes List

- Added dedicated cloud upload startup recovery service separate from processing recovery.
- Recovery normalizes stale `uploading`/`retrying` jobs as interrupted, preserves history/identity, checks local file availability, and marks missing local files with `UPLOAD_LOCAL_FILE_MISSING` / `CHECK_LOCAL_OUTPUT`.
- Uploaded jobs are preserved when persisted remote metadata is sufficient; jobs without safe remote identity metadata become actionable `UPLOAD_OBJECT_CHECK_NEEDED` / `CHECK_CLOUD_OBJECT` instead of being re-uploaded blindly.
- Eligible retry-scheduled jobs resume through existing retry semantics; eligible pending recovery jobs resume through `Worker.ResumeRecoveredJob`, a dedicated startup path that shares the same upload guard and preserves stricter manual/auto retry rules.
- Startup wiring runs upload recovery after HTTP server startup and before the long-running select loop, keeping request paths and capture/local processing separate.
- Added safe `upload.recovered` SSE/activity summary and dashboard invalidation; OpenAPI documents recovery event and restart status normalization semantics.
- ✅ Resolved review finding [High]: pending jobs persisted across restart now actually resume via dedicated guarded recovery resume after durable save.
- ✅ Resolved review finding [Medium]: production remote verification is wired through GCS `Stat` with timeout wrapper; if a verifier is unavailable, recovery reports uploaded jobs as unverified instead of verified.

### File List

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/upload/gcs_uploader.go`
- `apps/agent/internal/upload/recovery.go`
- `apps/agent/internal/upload/recovery_test.go`
- `apps/agent/internal/upload/retry_runner.go`
- `apps/agent/internal/upload/uploader.go`
- `apps/agent/internal/upload/worker.go`
- `apps/web/src/features/health/health-dashboard.tsx`
- `docs/api/openapi.yaml`
- `_bmad-output/implementation-artifacts/6-5-recover-pending-cloud-upload-jobs-after-restart.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`

## Change Log

- 2026-05-19: Ultimate context engine analysis completed - comprehensive developer guide created.
- 2026-05-19: Implemented cloud upload startup recovery with safe normalization, resume, SSE/activity, docs, and tests.
- 2026-05-19: Addressed code review findings - 2 items resolved; added guarded pending recovery resume, production GCS stat verifier wiring, unverified uploaded summary, and follow-up tests/docs.
