# Story 6.4: Track, Retry, and De-Duplicate Cloud Uploads

Status: done

## Story

Sebagai operator, saya ingin upload cloud yang gagal dapat di-retry secara aman, sehingga delivery cloud bisa pulih tanpa membuat asset duplikat, tanpa mengganggu workflow lokal, dan tanpa mengekspos credential.

## Acceptance Criteria

1. Given upload file gagal karena token, network, partial upload, timeout, conflict/precondition, atau remote error, when sistem menyimpan status gagal, then job menyimpan safe error code/action, attempt count, timestamp, dan status per-file/per-session yang benar tanpa menghapus atau mengubah local original/graded file.
2. Given job upload berstatus failed atau pending retryable, when retry otomatis berjalan, then sistem retry secara background sampai batas maksimum 3 attempt total per job menggunakan identity deterministik `(session_id, photo_id, asset_kind)` dan object key yang sama.
3. Given operator menekan Retry Cloud Upload untuk session atau file gagal, when request diterima dari authenticated trusted origin, then retry manual berjalan asynchronous, tercatat di activity log, dan tidak menjalankan upload besar di request path.
4. Given retry otomatis/manual terjadi bersamaan, restart ringan, double-click UI, atau SSE/API trigger ganda, when job yang sama sedang pending/uploading/retrying/uploaded, then sistem tidak membuat duplicate job dan tidak meng-upload object duplikat.
5. Given remote object sudah ada atau upload sebelumnya mungkin partial/unknown, when retry berjalan, then sistem menggunakan tracked `remote_identity`/object metadata/status dan GCS idempotency guard yang aman untuk mencegah duplicate assets atau overwrite tidak sengaja.
6. Given retry gagal lagi, when batas retry otomatis belum tercapai, then job tetap retryable dengan actionable error; when batas retry otomatis tercapai, then job tetap failed dan membutuhkan Retry Cloud Upload manual.
7. Given sebagian file uploaded dan sebagian failed, when status agregat dihitung, then session cloud status menunjukkan `partial_failed`/`failed`/`uploading`/`uploaded` secara jujur dan tetap terpisah dari local save/processing status.
8. Given retry status berubah, when API/SSE/UI/activity log diperbarui, then operator melihat per-file retry count, last error action, next action, dan label teks; payload tidak berisi credential, token, service account JSON/private key, credential path, raw privileged cloud error, atau full local customer filesystem path.
9. Tests/build pass untuk Go agent, web typecheck/build yang relevan, dan OpenAPI/API contract diperbarui.

## Tasks / Subtasks

- [x] Perluas model upload job untuk retry/de-dup metadata. (AC: 1, 2, 4, 5, 6, 8)
  - [x] Update `apps/agent/internal/upload/jobs.go` tanpa mengganti identity yang sudah ada: `JobID(session_id, photo_id, asset_kind)` tetap authoritative.
  - [x] Tambahkan status retry-compatible: gunakan enum yang konsisten seperti `retrying` dan/atau `retry_scheduled`; jangan membuat status ambigu yang bercampur dengan processing job.
  - [x] Tambahkan field minimal bila belum ada: `next_retry_at`, `last_attempt_at`, `retry_after`, `max_attempts` atau konstanta max 3, `remote_generation`/`remote_etag` atau metadata remote aman yang tersedia dari GCS, dan `dedupe_key` bila dibutuhkan untuk API/UI.
  - [x] Pertahankan `local_path` hanya internal/API aman yang sudah ada; jangan menambahkan credential/token/service account/raw cloud error ke persisted job.
  - [x] Pastikan migrasi/load backward-compatible dari state Story 6.3 (`upload_jobs.json` version 1) atau bump version dengan migrasi eksplisit yang aman.
- [x] Implement retry policy otomatis yang konservatif dan testable. (AC: 1, 2, 6)
  - [x] Letakkan policy di `apps/agent/internal/upload`, misalnya `retry_policy.go`; jangan reuse processing retry code secara langsung karena lifecycle cloud berbeda.
  - [x] Retry otomatis hanya untuk failure retryable: network/timeout/token transient/remote 5xx/unknown partial yang aman; non-retryable config/target/local-file-missing harus menunggu operator fix.
  - [x] Batasi auto retry sampai 3 attempt total per job; manual retry boleh melewati limit hanya setelah operator action eksplisit.
  - [x] Gunakan backoff sederhana dan deterministic/testable (contoh 5s/30s/2m atau configurable) tanpa loop tak terbatas.
  - [x] Persist setiap transisi retry sebelum publish SSE/activity; tidak boleh mengklaim retry scheduled/started bila save gagal.
- [x] Bangun duplicate-safe retry runner/guard. (AC: 2, 3, 4, 5)
  - [x] Tambahkan in-memory guard keyed by `job_id` atau `session_id:photo_id:asset_kind` agar double trigger tidak menjalankan dua upload paralel untuk asset yang sama.
  - [x] Jika job sedang `uploading`/`retrying`, retry request harus return accepted/no-op yang jujur, bukan membuat job baru.
  - [x] Jika job sudah `uploaded`, retry request harus no-op idempotent dan tidak re-upload kecuali ada explicit future force mode (bukan scope story ini).
  - [x] Jangan scan output folder bebas untuk retry; retry wajib berasal dari persisted `FileUploadJob` dan verified local file.
  - [x] Preserve queue isolation: retry upload tidak boleh memanggil processing queue, ingestion, watcher, session routing, atau local save path.
- [x] Perkuat GCS de-duplication/idempotency behavior. (AC: 4, 5)
  - [x] Update `apps/agent/internal/upload/gcs_uploader.go` dan `Uploader` interface bila perlu agar upload result menyimpan safe remote generation/metageneration/etag/identity.
  - [x] Gunakan deterministic object key dari Story 6.2/6.3; jangan generate object key baru saat retry.
  - [x] Gunakan GCS precondition yang aman untuk create-only/overwrite-safe behavior, misalnya generation precondition (`ifGenerationMatch`/`DoesNotExist`) atau equivalent Go client condition, dan dokumentasikan pilihan.
  - [x] Untuk object sudah ada dengan same tracked identity/status, reconcile sebagai uploaded bila aman; untuk conflict yang tidak bisa dipastikan sama, mark failed dengan safe action `RETRY_CLOUD_UPLOAD` atau `CHECK_CLOUD_OBJECT` (pilih action konsisten dan dokumentasikan di OpenAPI).
  - [x] Jangan overwrite remote object tanpa identity check; jangan membuat suffix copy seperti `(1)` untuk menghindari duplicate asset.
- [x] Tambahkan API retry manual. (AC: 3, 4, 7, 8)
  - [x] Endpoint disarankan: `POST /api/uploads/{job_id}/retry` untuk per-file retry dan `POST /api/sessions/{session_id}/uploads/retry` untuk semua failed/retryable jobs pada session.
  - [x] Semua endpoint wajib `RequireAuth`; mutation wajib `RequireTrustedOrigin`.
  - [x] Response success wajib `{data}`; error wajib `{error:{code,message,action,details}}`.
  - [x] Error/action minimal: `UPLOAD_JOB_NOT_FOUND`, `UPLOAD_NOT_RETRYABLE`, `UPLOAD_ALREADY_RUNNING`, `UPLOAD_ALREADY_UPLOADED`, `UPLOAD_LOCAL_FILE_MISSING`/`CHECK_LOCAL_OUTPUT`, `UPLOAD_RETRY_STATE_SAVE_FAILED`/`RETRY_CLOUD_UPLOAD`, `UPLOAD_FAILED`/`RETRY_CLOUD_UPLOAD`.
  - [x] Retry request path hanya enqueue/start background worker; jangan upload file besar secara blocking.
- [x] Update aggregate status, SSE, activity log, dan UI. (AC: 1, 3, 6, 7, 8)
  - [x] Update `AggregateUploadStatus` agar menghitung `retrying`/`retry_scheduled` tanpa menyembunyikan failed/pending/uploaded states.
  - [x] SSE event dot-notation disarankan: `upload.retry_started`, `upload.retry_scheduled`, `upload.retry_failed`, `upload.retry_exhausted`, plus existing `upload.file_uploaded` dan `upload.session_updated`.
  - [x] Activity log actions: `cloud.upload_retry_started`, `cloud.upload_retry_scheduled`, `cloud.upload_retry_failed`, `cloud.upload_retry_exhausted`; include safe station/session refs bila tersedia.
  - [x] Update `apps/web/src/lib/api/client.ts` untuk types/functions retry dan status enum baru.
  - [x] Update `apps/web/src/features/sessions/live-station-cards.tsx` dan/atau component detail upload agar tombol `Retry Cloud Upload` hanya muncul untuk failed/partial_failed/retryable job/session, memakai loading state, dan tidak menyembunyikan locked session.
  - [x] Update `apps/web/src/features/health/health-dashboard.tsx` SSE invalidation untuk upload retry events; jangan invalidate processing queue kecuali local processing berubah.
- [x] Update OpenAPI dan dokumentasi. (AC: 3, 7, 8, 9)
  - [x] Update `docs/api/openapi.yaml` dengan endpoint retry, request/response schemas, status enum, safe error/action, dan SSE event schemas.
  - [x] Dokumentasikan bahwa retry/de-dup menggunakan deterministic object key: `{target_root_prefix}/{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}/{asset_kind}/{safe_file_name}`.
  - [x] Dokumentasikan batas Story 6.4: retry/de-dup runtime untuk persisted jobs; recovery setelah application restart penuh tetap Story 6.5 kecuali state sudah diload oleh proses berjalan.
  - [x] Tegaskan provider tetap Google Cloud Storage, bukan Google Drive API.
- [x] Tambahkan tests dan validasi. (AC: 1-9)
  - [x] Go unit tests untuk retryable vs non-retryable error mapping, max 3 automatic attempts, manual retry beyond automatic limit, dan backoff/next_retry_at.
  - [x] Go tests untuk duplicate guard: double manual retry/double auto tick tidak menjalankan dua uploader call untuk job yang sama.
  - [x] Go tests untuk uploaded job retry no-op dan failed job retry reuses same `object_key`/`job_id`.
  - [x] GCS uploader tests dengan fake/adapter untuk precondition conflict, already-exists reconcile, remote identity capture, dan no overwrite/no duplicate suffix.
  - [x] API tests untuk auth/trusted-origin, job not found, not retryable, already running, already uploaded, state save failure, per-file retry success, session retry success.
  - [x] Web type/schema tests jika API client/UI disentuh.
  - [x] Run `cd apps/agent && go test ./...`.
  - [x] Run `cd apps/web && npm run typecheck`.
  - [x] Run `cd apps/web && npm run build` jika environment/dependencies memungkinkan.

## Dev Notes

### Source Requirements

- Epic 6 objective: post-session cloud fulfillment dengan GCS sesuai architecture, per-file/per-session upload tracking, retry failure, duplicate prevention, dan upload tidak memblokir session baru.
- Story 6.4 dari epics: upload gagal karena token/network/partial/remote error harus retry otomatis/manual secara aman; sistem menggunakan tracked upload identity/status untuk mencegah duplicate upload; status update per file/per session; failure menampilkan action Retry Drive/Cloud Upload; retry action logged.
- FR59-FR63: track upload status per session/file, retry failed uploads, preserve local files on failure, ensure upload does not block capture sessions, prevent duplicate cloud uploads using tracked identity/status.
- NFR4/NFR11/NFR16/NFR25/NFR26/NFR32: upload tidak degrade local workflow, local save independent dari cloud success, retries duplicate-safe, token/network/partial upload handled safely, remote folder/file status tracked enough to reduce duplicate uploads, local-vs-cloud status clearly separated.
- PRD journey: Google Drive/Cloud upload failure must be pending/failed while local result remains safe; operator can retry after connectivity returns.

### Current Code Context To Read Before Editing

- `apps/agent/internal/upload/jobs.go`
  - Current state: `FileUploadJob` persisted identity/status from Story 6.3 with deterministic `JobID(sessionID, photoID, assetKind)`, statuses `pending/uploading/uploaded/failed/not_eligible`, `attempt_count`, safe error fields, `remote_identity`, timestamps.
  - What this story changes: add retry/de-dup metadata/statuses without breaking deterministic identity.
  - Must preserve: one `(session_id, photo_id, asset_kind)` active job; no credential fields.
- `apps/agent/internal/upload/jobs_persistence.go`
  - Current state: versioned atomic JSON persistence under `local-data/state/upload_jobs.json`, validates duplicate jobs and deterministic IDs.
  - What this story changes: add version migration or backward-compatible optional fields.
  - Must preserve: corrupt/invalid state fails safe; save failure prevents success/SSE/activity.
- `apps/agent/internal/upload/worker.go`
  - Current state: `StartSession` discovers jobs from session/photo metadata, persists, then async uploads pending jobs; `uploadOne` increments attempt_count, saves before publish, preserves local file, and publishes worker events after durable save.
  - What this story changes: retry scheduler/runner, manual retry entrypoints, duplicate guard, retry status transitions.
  - Must preserve: non-blocking request path; upload failure only changes cloud upload job/session status; local files untouched.
- `apps/agent/internal/upload/uploader.go`
  - Current state: `Uploader` interface returns `RemoteIdentity`; `LocalCopyUploader` is test-only; `TimeoutUploader` wraps uploads.
  - What this story changes: interface may need remote generation/etag/precondition result; keep fakes simple and testable.
  - Must preserve: production wiring must not use `LocalCopyUploader`.
- `apps/agent/internal/upload/gcs_uploader.go`
  - Current state: production GCS uploader backed by `cloud.google.com/go/storage` from Story 6.3.
  - What this story changes: add safe precondition/idempotency handling and safe error mapping for already exists/unknown/remote failure.
  - Must preserve: credentials server-side only; no raw privileged errors in API/SSE/activity.
- `apps/agent/internal/api/session_uploads.go`
  - Current state: `GET /api/sessions/{session_id}/uploads`, `POST /api/sessions/{session_id}/uploads/start`, safe SSE/activity consumer for worker events.
  - What this story changes: add manual retry endpoints and safe event/action mapping.
  - Must preserve: `RequireAuth`/`RequireTrustedOrigin` wiring pattern from mux; no success event if persistence fails.
- `apps/agent/cmd/selfstudio-agent/main.go`
  - Current state: wires upload target store, jobs store, GCS uploader, worker, API, events.
  - What this story changes: wire retry scheduler/guard/ticker if implemented.
  - Must preserve: startup must continue to fail safe if upload job persistence is corrupt; no product code implementation in story creation phase.
- `apps/agent/internal/api/sessions.go`
  - Current state: session summary upload status aggregates target + jobs.
  - What this story changes: include retry statuses and next action in summaries if needed.
  - Must preserve: local output/processing/quarantine/session status behavior.
- `apps/web/src/lib/api/client.ts`
  - Current state: typed API client for session upload start/status from Story 6.3.
  - What this story changes: add retry endpoint functions/types and new status enum.
- `apps/web/src/features/sessions/live-station-cards.tsx`
  - Current state: locked sessions remain visible; shows cloud status and Start Cloud Upload button.
  - What this story changes: add Retry Cloud Upload affordance without active-only filtering regression.
- `apps/web/src/features/health/health-dashboard.tsx`
  - Current state: listens for upload events and invalidates session/upload/activity.
  - What this story changes: include retry event names.
- `docs/api/openapi.yaml`
  - Current state: documents upload start/status endpoints and notes Story 6.4/6.5 future scope.
  - What this story changes: document retry/de-dup endpoints, statuses, errors, SSE.

### Architecture Guardrails

- Go service owns upload retry, duplicate guard, GCS interaction, persistence, SSE, activity logging, and credential use.
- Next.js/browser must never receive service account JSON, private key, OAuth/ADC token, credential file path, Supabase service role, or raw privileged cloud error.
- API JSON/status fields use `snake_case`; success uses `{data}`; errors use `{error:{code,message,action,details}}`; SSE event names use dot notation.
- Cloud upload retry must not call or block watcher, ingestion, session routing, local original save, LUT processing, or processing retry queue.
- Local files are safety source: retry must not delete, move, rewrite, rename, or reprocess local original/graded files.
- Do not introduce Google Drive API; architecture-selected provider is Google Cloud Storage while UI text may say Cloud Upload.
- Do not implement full restart recovery scanner/reconciler in this story; Story 6.5 owns recovery after application restart. This story can load existing jobs as part of normal startup wiring but must not claim complete recovery.
- Do not create duplicate remote objects by changing object keys, appending suffixes, or overwriting without identity/precondition checks.

### Previous Story Intelligence

- Story 6.3 implemented per-file upload jobs, `upload_jobs.json`, deterministic object key builder, GCS uploader, non-blocking upload worker, session upload API, dashboard Start Cloud Upload action, safe SSE/activity, and OpenAPI docs.
- Story 6.3 review fixed critical issues: production now uses real server-side `GCSUploader`, worker events are consumed/published to SSE/activity, final status is published only after durable persistence save, and API handler tests exist.
- Story 6.2 created session-level prefix-only `SessionCloudTarget` and fixed locked-session dashboard visibility; do not hide locked/completed sessions again.
- Story 6.1 established credential safety, GCS settings, object key sanitizer, authenticated/trusted-origin endpoints, and persistence safety; reuse those patterns.
- Story 5.4 has processing retry patterns, but upload retry must remain separate because upload has remote identity/precondition concerns and must not interact with processing retry guard.
- Git history currently has only sparse setup commits (`Add from-scratch setup guide`, `Initial Selfstudio camera capture spike`); implementation artifacts and working tree are authoritative.

### Latest Technical Notes

- Current GCS Go client docs indicate storage operations may retry transient failures and that operations such as object insert/copy/rewrite are conditionally idempotent when generation preconditions are used. Use preconditions (for example `ifGenerationMatch` / `DoesNotExist` through `cloud.google.com/go/storage` conditions) to make upload retry safer and prevent race/duplicate overwrite. [Source: Google Cloud Storage Go client docs and Cloud Storage retry/precondition docs, accessed 2026-05-19]
- GCS object namespace is flat; `/` is prefix convention only. The app intentionally uses strict sanitized object keys.
- Do not upgrade `cloud.google.com/go/storage` unless necessary; if changed, run full Go tests and document why.

### Testing Requirements

Run at minimum:

- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build` if dependencies/environment allow.

Required coverage:

- Retry policy max 3 automatic attempts and manual retry explicit behavior.
- Retryable/non-retryable safe error classification.
- Duplicate guard prevents parallel upload for same `job_id` from double trigger.
- Retry uses same `job_id`, `object_key`, `bucket_name`, and `local_path`; no suffix/renamed object key.
- Already uploaded job retry is no-op and returns truthful status.
- GCS precondition/already-exists/unknown result maps to safe status/action without raw errors.
- Persistence save failure prevents retry-started/upload-success SSE/activity/API success.
- API endpoints require auth and trusted origin for mutations.
- UI typecheck covers retry status enum and retry endpoint functions.
- Existing Story 6.1-6.3 cloud target/upload tests still pass.

### Regression Risks To Avoid

- Do not reintroduce production `LocalCopyUploader` or fake success without GCS upload.
- Do not publish `upload.file_uploaded`/retry success before durable `upload_jobs.json` save.
- Do not create a second job row for the same `(session_id, photo_id, asset_kind)`.
- Do not start two upload goroutines for one job because of double click, auto tick, or concurrent API calls.
- Do not overwrite remote objects or create duplicate `(1)`/suffix objects as a de-dup strategy.
- Do not mark session `uploaded` while eligible jobs are failed/retrying/pending.
- Do not expose full local paths or credential material in SSE/activity/API.
- Do not break locked session visibility in dashboard.
- Do not block local event operation while retry is waiting/backing off.

## Project Structure Notes

Expected new files:

- `apps/agent/internal/upload/retry_policy.go`
- `apps/agent/internal/upload/retry_runner.go` or equivalent guard/scheduler file
- `apps/agent/internal/upload/retry_policy_test.go`
- `apps/agent/internal/upload/retry_runner_test.go`
- `apps/agent/internal/api/session_upload_retries_test.go`
- Optional frontend hook/component: `apps/web/src/features/sessions/use-retry-cloud-upload-mutation.ts`, `apps/web/src/features/sessions/session-upload-status.tsx`

Expected modified files:

- `apps/agent/internal/upload/jobs.go`
- `apps/agent/internal/upload/jobs_persistence.go`
- `apps/agent/internal/upload/worker.go`
- `apps/agent/internal/upload/uploader.go`
- `apps/agent/internal/upload/gcs_uploader.go`
- `apps/agent/internal/upload/worker_test.go`
- `apps/agent/internal/api/session_uploads.go`
- `apps/agent/internal/api/health.go` or mux wiring file
- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/sessions.go`
- `apps/web/src/lib/api/client.ts`
- `apps/web/src/features/sessions/live-station-cards.tsx`
- `apps/web/src/features/health/health-dashboard.tsx`
- `docs/api/openapi.yaml`

Runtime state remains under `local-data/state` and must not be committed.

## References

- `_bmad-output/planning-artifacts/epics.md` — Epic 6 and Story 6.4 acceptance criteria; FR59-FR63 mapping.
- `_bmad-output/planning-artifacts/prd.md` — Google Drive/Cloud Fulfillment retry/failure requirements; NFR16/NFR25/NFR26/NFR32.
- `_bmad-output/planning-artifacts/architecture.md` — GCS selected over Drive API, Go credential boundary, upload package location, API/SSE/error/retry/idempotency patterns.
- `_bmad-output/implementation-artifacts/6-1-configure-cloud-storage-credentials-and-target-rules.md` — cloud config, credential safety, GCS object key sanitizer, API/SSE/activity patterns.
- `_bmad-output/implementation-artifacts/6-2-create-cloud-folder-object-structure-per-session.md` — session target prefix, idempotent remote identity, locked-session UI fix.
- `_bmad-output/implementation-artifacts/6-3-upload-original-and-graded-jpgs-after-local-completion.md` — current upload job/worker/API/UI implementation and review follow-up lessons.
- `apps/agent/internal/upload/jobs.go` — per-file upload job identity/status model.
- `apps/agent/internal/upload/jobs_persistence.go` — atomic upload job persistence.
- `apps/agent/internal/upload/worker.go` — upload discovery/run flow and durable publish guard.
- `apps/agent/internal/upload/gcs_uploader.go` — production GCS upload implementation to harden with preconditions.
- `apps/agent/internal/api/session_uploads.go` — upload status/start API and worker event publishing.
- `apps/web/src/features/sessions/live-station-cards.tsx` — dashboard cloud status/action UI.
- `apps/web/src/lib/api/client.ts` — frontend API client/types.
- `apps/web/src/features/health/health-dashboard.tsx` — SSE invalidation pattern.
- `docs/api/openapi.yaml` — API/SSE contract source.
- Google Cloud Storage Go client and retry/precondition docs — use generation preconditions for conditionally idempotent upload retry behavior.

## Review Follow-ups (AI)

- [x] **Critical** — Shared worker/guard fix belum lengkap untuk concurrency auto retry vs API/manual retry. `upload.Worker` masih bertipe value yang mengandung `guard uploadGuard` (mutex + map), `main.go` menjalankan scheduler pada variabel `uploadWorker` (`uploadWorker.AutoRetryDue(...)`), sementara `NewSessionUploadsHandler(..., worker upload.Worker, ...)` menyalin worker itu lalu menyimpan pointer ke copy (`workerPtr := &worker`). Akibatnya auto retry dan API retry memiliki guard berbeda; trigger bersamaan dari scheduler dan double-click/API masih bisa meng-upload `job_id` yang sama paralel. Gunakan satu shared `*upload.Worker` end-to-end atau guard eksternal/pointer yang tidak bisa tercopy, dan tambahkan test concurrent auto tick + manual/API retry yang memastikan uploader hanya dipanggil sekali. (AC4, duplicate guard concurrency; bukti: `apps/agent/cmd/selfstudio-agent/main.go:114-121,141`, `apps/agent/internal/api/session_uploads.go:56-58`, `apps/agent/internal/upload/worker.go:13-21`)
- [x] **Critical** — Perbaiki duplicate guard concurrency agar satu `job_id` benar-benar tidak bisa di-upload paralel dari double-click/API/SSE/auto trigger. `SessionUploadsHandler` menyimpan `upload.Worker` by value dan `RetryJob`/`RetrySession` menyalin lagi `worker := h.Worker`; `uploadGuard` (mutex + map) ikut tercopy sehingga concurrent request dapat memiliki guard berbeda dan sama-sama memulai goroutine upload untuk job yang sama. Gunakan shared worker/guard pointer atau guard eksternal yang tidak tercopy, lalu tambah test concurrent API/manual retry yang memastikan uploader hanya dipanggil sekali. (AC4, duplicate guard concurrency)
- [x] **High** — Jangan kirim `local_path` penuh di API response retry/status/session upload atau OpenAPI. `SessionUploadsData` dan `RetryResult` saat ini mengembalikan `[]upload.FileUploadJob` langsung, sehingga `local_path` persisted internal dapat mengekspos full local customer filesystem path ke browser/API, bertentangan dengan AC8 dan guardrail safe payload. Buat DTO aman tanpa full local path (mis. basename/safe local ref bila perlu) dan update client/OpenAPI/tests. (AC8, remote metadata/API safety)
- [x] **High** — Tangani GCS create-only precondition conflict secara eksplisit sebagai safe conflict/object-check path, bukan generic retry. `GCSUploader` memakai `DoesNotExist`, tetapi `writer.Close()` tidak mendeteksi Google API 412/precondition failure dan hampir semua error dimap ke `UPLOAD_FAILED/RETRY_CLOUD_UPLOAD`; ini dapat membuat retry otomatis berulang terhadap object yang sudah ada/unknown alih-alih mark `CHECK_CLOUD_OBJECT` atau reconcile aman sesuai tracked identity. Tambah error mapping/test fake untuk precondition failed/already exists dan pastikan tidak overwrite/suffix. (AC5)
- [x] **Medium** — Jadikan retry manual untuk job `uploaded`/sedang running benar-benar no-op idempotent yang truthful, bukan 409 error. Saat ini `RetryJob` pada uploaded/running jatuh ke `SafeUploadError` (`UPLOAD_ALREADY_UPLOADED`/`UPLOAD_ALREADY_RUNNING`), sedangkan task meminta accepted/no-op jujur dan uploaded no-op idempotent tanpa re-upload. Selaraskan API contract/UI/test dengan semantics no-op yang disepakati. (AC3, AC4)

## Dev Agent Record

### Agent Model Used

OpenAI GPT-5 (Codex CLI)

### Debug Log References

- 2026-05-19: Final critical review follow-up fixed; validasi akhir: `cd apps/agent && go test ./...`, `cd apps/web && npm run typecheck`, `cd apps/web && npm run build`.
- 2026-05-19: Review follow-up fixes completed; validasi akhir: `cd apps/agent && go test ./...`, `cd apps/web && npm run typecheck`, `cd apps/web && npm run build`.
- 2026-05-19: Implementasi retry/de-dup cloud upload mengikuti red-green-refactor; validasi akhir: `cd apps/agent && go test ./...`, `cd apps/web && npm run typecheck`, `cd apps/web && npm run build`.
- 2026-05-19: Catatan transient: satu run paket terbatas `go test ./internal/api ./internal/upload` sempat gagal saat cleanup TempDir Windows karena file state masih aktif; rerun paket API dan full suite agent berhasil.

### Completion Notes List

- ✅ Resolved review finding [Critical]: scheduler auto retry, API/manual retry, dan handler sekarang memakai satu shared `*upload.Worker`; guard disimpan sebagai pointer agar copy worker tidak membawa mutex/map terpisah; test concurrent auto retry + manual retry memastikan uploader hanya dipanggil sekali untuk `job_id` yang sama.
- ✅ Resolved review finding [Critical]: `SessionUploadsHandler` sekarang memakai shared `*upload.Worker` dan retry API tidak menyalin guard; test concurrent API memastikan hanya satu upload call untuk `job_id` yang sama.
- ✅ Resolved review finding [High]: API retry/status/session upload memakai DTO aman tanpa `local_path`; frontend type dan OpenAPI diganti ke `local_file_name` basename-only.
- ✅ Resolved review finding [High]: GCS precondition/already-exists conflict dipetakan eksplisit ke `UPLOAD_OBJECT_CHECK_NEEDED`/`CHECK_CLOUD_OBJECT`, dan auto retry tidak mengulang conflict object-check.
- ✅ Resolved review finding [Medium]: retry manual untuk `uploaded`/`uploading`/`retrying` menjadi 202 no-op idempotent dengan `noop_reason`, bukan 409 error.
- Menambahkan metadata retry/de-dup pada `FileUploadJob` tanpa mengubah deterministic `JobID(session_id, photo_id, asset_kind)` dan tetap backward-compatible dengan `upload_jobs.json` version 1 melalui field optional.
- Menambahkan retry policy cloud upload terpisah dari processing retry: retryable classification, batas 3 automatic attempts, manual retry eksplisit beyond limit, backoff 5s/30s/2m, dan persisted transition sebelum event.
- Menambahkan duplicate-safe retry runner dengan in-memory guard per job_id; retry running/uploaded menjadi no-op/error jujur dan retry hanya memakai persisted job serta local file terverifikasi.
- Memperkuat GCS uploader dengan create-only generation precondition `DoesNotExist`, deterministic object key reuse, dan capture remote generation/metageneration/etag tanpa overwrite/suffix duplicate.
- Menambahkan endpoint retry manual per job dan per session dengan auth + trusted origin, response `{data}`/`{error}`, safe error/action, event/activity retry, serta background worker path.
- Memperbarui aggregate status, SSE payload aman, UI `Retry Cloud Upload`, API client types/functions, health invalidation existing `upload.*`, dan OpenAPI contract.
- Tests ditambahkan/diupdate untuk retry policy, duplicate guard, identity reuse, API retry auth/not-found/success, dan full regression lulus.

### File List

- apps/agent/cmd/selfstudio-agent/main.go
- apps/agent/internal/api/health.go
- apps/agent/internal/api/session_upload_retries_test.go
- apps/agent/internal/api/session_uploads.go
- apps/agent/internal/upload/gcs_uploader.go
- apps/agent/internal/upload/jobs.go
- apps/agent/internal/upload/jobs_persistence.go
- apps/agent/internal/upload/retry_policy.go
- apps/agent/internal/upload/retry_policy_test.go
- apps/agent/internal/upload/retry_runner.go
- apps/agent/internal/upload/retry_runner_test.go
- apps/agent/internal/upload/uploader.go
- apps/agent/internal/upload/worker.go
- apps/agent/internal/upload/worker_test.go
- apps/web/src/features/sessions/live-station-cards.tsx
- apps/web/src/features/sessions/use-retry-cloud-upload-mutation.ts
- apps/web/src/lib/api/client.ts
- docs/api/openapi.yaml
- _bmad-output/implementation-artifacts/6-4-track-retry-and-de-duplicate-cloud-uploads.md
- _bmad-output/implementation-artifacts/sprint-status.yaml

## Change Log

- 2026-05-19: Addressed final critical code review finding - shared worker/guard fixed for scheduler and API/manual retry.
- 2026-05-19: Addressed code review findings - 4 items resolved.
- 2026-05-19: Ultimate context engine analysis completed - comprehensive developer guide created.
- 2026-05-19: Implemented retry/de-dup cloud uploads, manual retry API/UI, GCS idempotency guard, docs, and tests; story moved to review.
