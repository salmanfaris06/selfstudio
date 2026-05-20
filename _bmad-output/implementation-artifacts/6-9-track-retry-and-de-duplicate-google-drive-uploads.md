# Story 6.9: Track, Retry, and De-duplicate Google Drive Uploads

Status: done

## Story

Sebagai operator, saya ingin upload Google Drive yang gagal dapat di-retry aman, sehingga delivery dapat pulih tanpa membuat file duplikat dan tanpa mengganggu workflow lokal.

## Acceptance Criteria

1. Given upload file gagal karena credential/token/network/Drive API/partial upload, then job menyimpan safe error code/action, attempt count, timestamp, dan status per-file/per-session.
2. Given job failed atau retryable, when retry otomatis berjalan, then sistem retry background sampai batas maksimum 3 attempt total per job memakai identity deterministik `(session_id, photo_id, asset_kind)`.
3. Given operator menekan Retry Drive Upload, then retry manual berjalan asynchronous dari authenticated trusted origin dan tercatat di activity log.
4. Given retry otomatis/manual/double click/restart ringan terjadi bersamaan, then sistem tidak membuat duplicate job dan tidak meng-upload duplicate file.
5. Given remote Drive file mungkin sudah ada dari attempt sebelumnya, then sistem menggunakan tracked `drive_file_id`, app properties, atau lookup parent+name+metadata untuk mencegah duplicate Drive files.
6. Given sebagian file uploaded dan sebagian failed, then aggregate status menunjukkan `partial_failed`/`failed`/`uploading`/`uploaded` secara jujur dan terpisah dari local status.
7. Given retry status berubah, then API/SSE/UI/activity menampilkan retry count, last error action, next action, dan label teks tanpa secret/raw privileged error.
8. Tests/build pass dan OpenAPI/docs diperbarui.

## Acceptance Criteria Context / BDD Detail

### AC1 — Persist safe failure metadata per file and aggregate session

- Source of truth upload job adalah `apps/agent/internal/upload.FileUploadJob` yang dipersist ke `local-data/state/upload_jobs.json` via `JobsPersistence`.
- Failure credential/token/network/Drive API/partial upload harus masuk sebagai safe code/action, bukan raw Google error. Gunakan/pertahankan mapping Story 6.8:
  - credential/token/unauthorized → `DRIVE_UPLOAD_UNAUTHORIZED` + `FIX_DRIVE_CREDENTIALS`.
  - folder missing/not writable/permission → `DRIVE_FOLDER_UNAVAILABLE` + `FIX_DRIVE_FOLDER` atau `RETRY_DRIVE_FOLDER` sesuai konteks.
  - network/rate/transient/partial response → `DRIVE_UPLOAD_FAILED` atau `UPLOAD_FAILED` + `RETRY_DRIVE_UPLOAD`.
  - local file missing → `UPLOAD_LOCAL_FILE_MISSING` + `CHECK_LOCAL_OUTPUT` dan tidak retry otomatis.
- Saat upload gagal, job wajib menyimpan: `status`, `attempt_count`, `last_attempt_at`, `updated_at`, `last_error_code`, `last_error_action`, `next_retry_at`/`retry_after` bila dijadwalkan.
- DTO publik tidak boleh mengirim `local_path`; hanya `local_file_name`.
- Per-session aggregate memakai `AggregateUploadStatus` dan tetap terpisah dari status local save/processing serta Drive target status.

### AC2 — Automatic retry, max 3 total attempts, deterministic identity

- Existing `MaxAutoUploadAttempts = 3` adalah batas attempt total per job, bukan 3 retry tambahan setelah attempt pertama.
- Deterministic identity wajib tetap `JobID(sessionID, photoID, assetKind)` dan `dedupe_key` sama dengan job ID.
- `AutoRetryDue` harus menjadwalkan failed retryable job ke `retry_scheduled`, lalu menjalankan retry saat `next_retry_at` due.
- Retry tidak boleh membuat job baru jika job dengan `(session_id, photo_id, asset_kind)` sudah ada. `ensureJob` harus return existing job.
- Manual retry boleh memakai endpoint existing, tetapi tidak boleh bypass max attempt untuk auto retry. Jika produk mengizinkan manual setelah exhausted, behavior harus eksplisit dan aman; untuk story ini default aman: manual hanya menerima job retryable sesuai `CanManualRetry`.

### AC3 — Manual retry endpoint, async, auth/trusted-origin safe, activity logged

- Endpoint existing yang harus dipakai/ditingkatkan:
  - `POST /api/uploads/{job_id}/retry`
  - `POST /api/sessions/{session_id}/uploads/retry`
- Handler saat ini berada di `apps/agent/internal/api/session_uploads.go` (`RetryJob`, `RetrySession`). Pertahankan response `{data}` dan error `{error:{code,message,action,details}}`.
- Manual retry harus asynchronous: endpoint menerima request, set status `retrying`, persist, publish event, start goroutine, return `202 Accepted` dengan safe job DTO.
- Pastikan routing endpoint tetap melewati middleware auth/PIN/trusted-origin yang sudah ada di main/router; jangan menambahkan public unauthenticated retry path.
- Activity log harus mencatat request manual retry dan hasil worker event dengan message aman seperti “Google Drive upload retry requested/started/failed/uploaded”. Jangan mencatat credential/path/raw error.

### AC4 — Concurrency, double click, auto/manual/restart race safety

- Existing `uploadGuard` mencegah job sama berjalan paralel dalam satu process. Story ini harus memperkuat dan menutup race berikut:
  - manual double click untuk job/session yang sama.
  - auto retry due bersamaan dengan manual retry.
  - `StartSession` dipanggil lagi saat retry sedang berjalan.
  - startup recovery/resume bersamaan dengan retry manual ringan.
- Jika job sedang `uploading`/`retrying`, retry kedua harus no-op/accepted false dengan safe reason `UPLOAD_ALREADY_RUNNING`, bukan menaikkan `attempt_count` atau menjalankan upload kedua.
- Jika job sudah `uploaded` dengan `drive_file_id`, retry harus no-op reason `UPLOAD_ALREADY_UPLOADED` dan tidak memanggil Drive API.
- Persist update status sebelum goroutine upload dimulai sudah benar; pastikan semua branch release guard dan tidak meninggalkan job stuck.
- Karena `JobsStore.Get` lalu `guard.begin` bukan transaksi lintas process, story ini cukup untuk single local agent process; Story 6.10 akan memperkuat restart recovery.

### AC5 — Drive dedupe remote file identity

- Primary dedupe hierarchy:
  1. Jika job sudah punya `drive_file_id`/`remote_identity` dan status `uploaded`, jangan upload ulang.
  2. Jika job retry setelah partial create/persist failure dan `drive_file_id` belum tersimpan, lakukan lookup existing Drive file sebelum create.
  3. Lookup minimal saat ini: exact parent folder (`drive_folder_id`) + safe file name + MIME `image/jpeg` + `trashed=false`, deterministic oldest/first.
  4. Tambahkan/siapkan Drive `appProperties` bila feasible agar dedupe identity lebih kuat: `selfstudio_session_id`, `selfstudio_photo_id`, `selfstudio_asset_kind`, `selfstudio_dedupe_key`.
- Story 6.8 sudah menambahkan `DriveUploader.UploadFile` yang sanitizes name, `FindFile` by parent+name, lalu reuse existing ID sebelum create. Story 6.9 harus menambah tests yang membuktikan jalur retry memakai lookup ini dan tidak create duplicate setelah partial create.
- Jika menambahkan `appProperties`, update production `GoogleDriveFileClient.FindFile` query dan `UploadFile` metadata; fake tests harus mencakup parent+name+appProperties match. Jangan expose appProperties berisi data sensitif; session/photo IDs OK sebagai operational metadata.
- Jangan memakai Drive path string sebagai identity utama. `drive_file_id` adalah identity file; `drive_folder_id` adalah parent target.
- Jangan overwrite remote file kecuali identity terbukti aman. MVP recommendation: reuse existing matching file ID; jangan update/replace content di story ini.

### AC6 — Honest aggregate status

- `AggregateUploadStatus` harus tetap menghasilkan:
  - `uploading` bila ada `uploading`/`retrying`.
  - `partial_failed` bila ada failed dan setidaknya satu uploaded.
  - `failed` bila ada failed dan belum ada uploaded eligible.
  - `pending` bila masih ada pending/retry_scheduled.
  - `uploaded` hanya bila semua eligible jobs uploaded.
- `not_eligible` graded jobs tidak boleh dihitung sebagai failure dan tidak boleh mencegah `uploaded` jika semua eligible original/graded yang ada sudah uploaded.
- UI/session summary harus tidak menyamakan local complete dengan Drive uploaded. Local status, Drive target status, dan Drive upload status tetap terpisah.

### AC7 — API/SSE/UI/activity safe retry visibility

- Upload job DTO harus menampilkan safe fields: `attempt_count`, `max_attempts`, `last_attempt_at`, `next_retry_at`, `retry_after`, `last_error_code`, `last_error_action`, `drive_folder_id`, `drive_file_id`, `dedupe_key`, `status`.
- SSE payload existing di `SessionUploadsHandler.publish` sudah membawa retry fields; pertahankan dan pastikan event dikirim pada status `retrying`, `retry_scheduled`, `failed`, `uploaded`, session aggregate update.
- UI copy harus text-based: “Retry Drive Upload”, “Retrying Drive Upload…”, “retry_scheduled - retry otomatis dijadwalkan”, “failed - cek action aman lalu retry upload”. Jangan bergantung pada warna saja.
- Activity log message aman dan tidak berisi full local path, raw Google API body, credential file path, service account JSON, token/private_key/refresh_token.

### AC8 — Tests/build/docs

- Required validation setelah implementasi:
  - `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`
  - `cd apps/web && npm run typecheck`
  - `cd apps/web && npm run build`
  - `npm run typecheck`
- Update docs/API:
  - `docs/api/cloud-storage-story-6-1.md` dengan kontrak retry/dedupe Story 6.9.
  - `docs/api/openapi.yaml` untuk endpoint retry, job fields retry, status `retrying`/`retry_scheduled`, Drive IDs, safe error actions bila belum lengkap.

## Tasks / Subtasks

- [x] Audit current retry/dedupe implementation before editing
  - [x] Baca lengkap `apps/agent/internal/upload/jobs.go`, `jobs_persistence.go`, `worker.go`, `retry_policy.go`, `retry_runner.go`, `drive_uploader.go`, `recovery.go`, dan tests terkait.
  - [x] Baca `apps/agent/internal/api/session_uploads.go` dan router wiring di `apps/agent/cmd/selfstudio-agent/main.go` untuk auth/trusted-origin assumptions.
  - [x] Baca UI upload controls di `apps/web/src/features/sessions/live-station-cards.tsx`, mutation hooks upload/retry, dan `apps/web/src/lib/api/client.ts` runtime guards.
- [x] Tighten retry policy and error action semantics
  - [x] Pastikan `IsRetryableUploadError` menerima `RETRY_DRIVE_UPLOAD` selain `RETRY_CLOUD_UPLOAD` agar Drive-specific failures auto/manual retryable.
  - [x] Pertimbangkan alias compatibility: keep `RETRY_CLOUD_UPLOAD` internal lama bila tests masih ada, tetapi public/UI docs harus prefer `RETRY_DRIVE_UPLOAD`.
  - [x] Pastikan credential/folder/local-file errors tidak auto retry; network/Drive transient retryable.
  - [x] Tambahkan tests untuk `CanAutoRetry`, `CanManualRetry`, `ScheduleRetry`, dan max 3 attempt total.
- [x] Strengthen duplicate job and concurrent retry no-op behavior
  - [x] Tambahkan tests double manual retry/double click pada job failed yang sama; expect hanya satu accepted/running dan satu upload invocation.
  - [x] Tambahkan tests manual retry saat job `uploading`/`retrying`; expect no-op `UPLOAD_ALREADY_RUNNING` tanpa attempt increment.
  - [x] Tambahkan tests retry uploaded job; expect no-op `UPLOAD_ALREADY_UPLOADED` tanpa Drive call.
  - [x] Pastikan `RetrySession` tidak menerima uploaded/running jobs sebagai candidates yang membingungkan response; jika tetap dimasukkan untuk no-op reason, jangan return Accepted true.
- [x] Strengthen Drive remote de-duplication
  - [x] Tambahkan fake Drive client test: upload attempt creates file remotely lalu returns transient error / persist missing simulation; retry should find existing parent+safe-name file and reuse ID, not create second file.
  - [x] Jika feasible, tambahkan Drive `appProperties` identity pada create dan lookup: session_id/photo_id/asset_kind/dedupe_key.
  - [x] Jika appProperties ditunda, dokumentasikan parent+safe-name fallback dan pastikan tests membuktikan deterministic reuse.
  - [x] Pastikan `drive_file_id` yang ditemukan tersimpan di `DriveFileID` dan `RemoteIdentity`, `AlreadyExisted` tidak menjadi public leak yang tidak perlu.
- [x] Ensure failure metadata and aggregate statuses are truthful
  - [x] Tambahkan tests mixed jobs: original uploaded + graded failed → `partial_failed`; all eligible uploaded + graded not_eligible → `uploaded`; all failed → `failed`; retrying → `uploading`; retry_scheduled/pending → `pending`.
  - [x] Pastikan failed retryable job memiliki `next_retry_at` dan `retry_after` setelah `ScheduleRetry`.
  - [x] Pastikan exhausted job (`attempt_count >= 3`) tetap `failed` dan tidak dijadwalkan lagi.
- [x] API/SSE/activity safety and docs
  - [x] Tambahkan/ubah API tests untuk `RetryJob` dan `RetrySession`: `202 Accepted`, safe DTO, no `local_path`, no credential/raw token/private_key, activity recorded.
  - [x] Tambahkan SSE tests atau handler-level assertion payload membawa retry fields dan Drive IDs aman.
  - [x] Update OpenAPI/docs dengan retry endpoints dan Drive-specific actions.
- [x] Frontend UI/type guard updates
  - [x] Pastikan `FileUploadJob` runtime guard menerima `retrying`/`retry_scheduled`, Drive-only identity, optional legacy `bucket_name/object_key`, dan uploaded job wajib punya `drive_file_id`.
  - [x] Pastikan tombol `Retry Drive Upload` hanya aktif untuk `failed`/`partial_failed`, disabled saat retry mutation pending, dan label status retry count/next action terlihat.
  - [x] Jika upload job list belum menampilkan retry metadata, tambahkan minimal copy di session card/detail yang ada tanpa membuat state machine frontend baru.
- [x] Run validation and update story record
  - [x] Jalankan semua required validation command.
  - [x] Catat hasil di Dev Agent Record saat implementasi.

## Dev Notes

### Konteks Epic / Corrective Change

- Epic 6 adalah corrective path dari GCS ke Google Drive sesuai approved change proposal `_bmad-output/planning-artifacts/sprint-change-proposal-2026-05-20-google-drive.md`.
- Flow Epic 6 yang disetujui: `local complete → resolve remote folder → create upload jobs → upload originals/graded → retry/dedupe → restart recovery`.
- Story 6.9 fokus pada tahap retry/dedupe setelah Story 6.8 sudah bisa upload original/graded ke Drive.
- Story 6.10 berikutnya akan fokus restart recovery pending upload jobs. Namun Story 6.9 tetap harus tidak memperburuk recovery ringan: persisted statuses, next retry timestamps, and no duplicate jobs are prerequisites for 6.10.

### Prior Story 6.6 Learnings yang Wajib Dipakai

- Provider publik sekarang `google_drive`; jangan reintroduce GCS/bucket/object sebagai model mental operator.
- Public cloud settings allowlist Drive-safe saja: `provider`, `drive_root_folder_id`, `drive_root_folder_name`, `folder_naming_template`, `credentials_configured`, `connection_status`, `last_checked_at`, `last_error_code`, `last_error_action`.
- Browser/API/SSE/activity tidak boleh menerima credential file path, service account JSON, OAuth access/refresh token, private key, atau raw privileged error.
- Root folder validation harus mencakup write/probe capability; jangan percaya read metadata saja.
- Folder rule errors harus Drive-specific (`FIX_DRIVE_FOLDER_RULES`).

### Prior Story 6.7 Learnings yang Wajib Dipakai

- Drive target identity per session adalah final `drive_session_folder_id`; `remote_identity` target juga berisi folder ID, bukan path.
- Folder resolver lookup/create parent+name idempotent dan memilih duplicate deterministic (oldest `createdTime`, lalu ID).
- Ready target identity harus dipertahankan pada transient failure; jangan overwrite known-good target saat upload/retry gagal.
- Session detail sudah punya Drive target fields terpisah: `drive_target_status`, `drive_session_folder_id`, `drive_folder_path`, root, last error/action.

### Prior Story 6.8 Learnings yang Wajib Dipakai

- `FileUploadJob` sekarang punya `drive_folder_id` dan `drive_file_id`; `remote_identity` mirror `drive_file_id` setelah upload success.
- Original/graded asset subfolders (`original`, `graded`) dibuat/resolve idempotent di bawah `drive_session_folder_id`.
- Production `DriveUploader` sudah memakai settings-backed client factory agar credential changes tidak perlu restart.
- `DriveUploader.UploadFile` sudah melakukan `cloud.SafeFileName`, lookup exact parent+safe-name+not-trashed, reuse existing file, lalu create jika tidak ada.
- Review Story 6.8 menemukan dan memperbaiki risiko: stale credential snapshot, unsafe filename, duplicate file after partial create, OpenAPI GCS copy, dan frontend guard yang mewajibkan legacy `bucket_name/object_key`. Jangan regresi.
- Remaining gap yang sangat relevan untuk 6.9: `IsRetryableUploadError` saat ini perlu diverifikasi menerima Drive action `RETRY_DRIVE_UPLOAD`; bila belum, retry otomatis/manual untuk `DRIVE_UPLOAD_FAILED` dapat tidak berjalan.

### Current Code State yang Harus Dibaca Developer

- `apps/agent/internal/upload/jobs.go`
  - `JobID(sessionID, photoID, assetKind)` sudah deterministic; pertahankan sebagai source dedupe key.
  - `FileUploadJob` menyimpan `DriveFolderID`, `DriveFileID`, `RemoteIdentity`, `DedupeKey`, `AttemptCount`, `MaxAttempts`, `LastAttemptAt`, `NextRetryAt`, `RetryAfterSeconds`.
  - `validateJob` masih mengizinkan legacy `BucketName/ObjectKey` fallback; public contract harus tetap Drive-first.
- `apps/agent/internal/upload/worker.go`
  - `StartSession` menjaga session locked dan target ready; jangan ubah guard ini.
  - `ensureJob` return existing job sehingga duplicate job prevention sudah ada.
  - `uploadOneGuarded` increment `AttemptCount`, persist `uploading/retrying`, publish, upload, lalu set `uploaded` atau schedule retry via `CanAutoRetry`.
  - `uploadDriveOrLegacy` memakai `DriveUploader.UploadFile(ctx, j.DriveFolderID, fileName, j.LocalPath)` bila Drive path tersedia.
- `apps/agent/internal/upload/retry_policy.go`
  - `MaxAutoUploadAttempts = 3`.
  - Status tambahan: `retrying`, `retry_scheduled`.
  - Risk: `IsRetryableUploadError` saat dibaca hanya menerima `UPLOAD_FAILED` dan action `RETRY_CLOUD_UPLOAD`/`CHECK_CLOUD_OBJECT`; pastikan Drive errors (`DRIVE_UPLOAD_FAILED`, `RETRY_DRIVE_UPLOAD`) termasuk retryable sesuai AC.
- `apps/agent/internal/upload/retry_runner.go`
  - `uploadGuard` in-memory mencegah concurrent run per job.
  - `RetryJob`, `RetrySession`, `AutoRetryDue`, `ResumeRecoveredJob` sudah ada.
  - Manual retry skips uploaded/running jobs; response reason handling perlu tests agar double-click jelas dan tidak Accepted true palsu.
- `apps/agent/internal/upload/drive_uploader.go`
  - `DriveUploader.UploadFile` sanitizes file name, calls `FindFile`, reuses existing, then creates.
  - `GoogleDriveFileClient.FindFile` query currently parent+name+mimeType+trashed and pages all results. Consider appProperties enhancement.
  - `MapDriveUploadError` maps secrets safely; tests must prevent raw leak.
- `apps/agent/internal/api/session_uploads.go`
  - DTO excludes `local_path` and includes retry metadata/Drive IDs.
  - `publish` SSE payload includes retry fields and Drive IDs.
  - Activity messages are Google Drive-safe; keep safe.
- `apps/web/src/lib/api/client.ts`
  - `FileUploadJob` type includes Drive fields; runtime guard was fixed in Story 6.8 to accept Drive-only identity. Re-check before adding new statuses/fields.
- `apps/web/src/features/sessions/live-station-cards.tsx`
  - Retry button exists and is keyed off aggregate upload `failed`/`partial_failed`.
  - Ensure UX shows retry count/next action enough for AC7; do not create frontend-owned upload state machine.

### Architecture and Guardrails

- Go service owns all filesystem, upload worker, Drive API, credential, retry, and recovery logic.
- Next.js/browser only calls API and renders server state. No Drive credential or filesystem access in frontend.
- API convention: REST `/api`, SSE `/events`, success `{data}`, error `{error:{code,message,action,details}}`, JSON `snake_case`.
- SSE names dot notation; existing upload events (`upload.retry_started`, `upload.retry_scheduled`, `upload.retry_failed`, `upload.retry_exhausted`, `upload.file_uploaded`, `upload.session_updated`) are acceptable.
- Local filesystem remains safety source. Drive failure/retry must never delete local original/graded files or block capture/session/processing.
- Retry/dedupe must not affect photo routing, processing queue, quarantine, session start/end, or local output folder access.

### Google Drive Dedupe Technical Notes

- Drive allows duplicate names. Parent+name lookup is not mathematically perfect, but is MVP-safe fallback when combined with deterministic local file names and per-job persisted IDs.
- Best improvement for Story 6.9 is appProperties:
  - On create: set `AppProperties: map[string]string{"selfstudio_session_id": ..., "selfstudio_photo_id": ..., "selfstudio_asset_kind": ..., "selfstudio_dedupe_key": ...}`.
  - On lookup: query by parent + safe file name + trashed=false and/or appProperties. Google Drive query supports `appProperties has { key='...' and value='...' }`.
  - If current interface lacks session/photo/asset context, either extend interface to accept a small `DriveUploadPlan` or keep parent+name and add TODO/docs. Do not hack session IDs into filename.
- If multiple matching files exist, choose deterministically and log/activity safe warning if appropriate; do not create another duplicate.

### Data Model Notes

- Keep public `FileUploadJob` Drive-first:
  - `drive_folder_id`: parent folder for asset upload.
  - `drive_file_id`: remote file identity.
  - `remote_identity`: mirror `drive_file_id` for compatibility.
  - `dedupe_key`: deterministic `session_id:photo_id:asset_kind`.
- Do not expose `LocalPath`. `local_file_name` in DTO is enough.
- Legacy `bucket_name`, `object_key`, generation fields may remain optional compatibility fields but should not be required by frontend guard or docs.

### Testing Strategy

Backend priority tests:

1. Retry policy:
   - `DRIVE_UPLOAD_FAILED` + `RETRY_DRIVE_UPLOAD` is auto/manual retryable.
   - `DRIVE_UPLOAD_UNAUTHORIZED` + `FIX_DRIVE_CREDENTIALS` is not auto retryable.
   - `UPLOAD_LOCAL_FILE_MISSING` + `CHECK_LOCAL_OUTPUT` is not auto retryable.
   - attempt count 3 is exhausted.
2. Auto retry scheduling:
   - failed retryable job with attempt 1 schedules `retry_scheduled`, `next_retry_at`, `retry_after`.
   - due retry transitions to `retrying` then upload; not due does nothing.
3. Manual retry concurrency:
   - double `RetryJob` for same failed job only accepts one; second no-op already running or not retryable.
   - retry uploaded job returns no-op and no Drive call.
4. Drive dedupe:
   - failed job retry where remote file exists by parent+safe-name reuses `drive_file_id` and does not call create.
   - optional appProperties query/create if implemented.
5. Aggregate status:
   - uploaded + failed = `partial_failed`.
   - all failed = `failed`.
   - retrying = `uploading`.
   - retry_scheduled/pending = `pending`.
   - original uploaded + graded not_eligible = `uploaded`.
6. API/SSE safety:
   - retry endpoint DTO includes retry metadata and Drive IDs, excludes `local_path`.
   - SSE payload excludes secrets/full path/raw Drive error and includes `attempt_count`, `next_retry_at`, `last_error_action`, `dedupe_key`.

Frontend/type validation:

- Runtime guard accepts `retrying` and `retry_scheduled`.
- Retry button state remains derived from server aggregate status.
- Text labels expose retry count/next action enough for operator under event pressure.

### Regression Risks to Avoid

- Auto retry silently not working because Drive-specific action is not considered retryable.
- Manual retry double click increments `attempt_count` twice or uploads twice.
- Retry creates duplicate Drive file after partial create because it ignores existing parent+name/metadata.
- Marking uploaded jobs failed during light restart because verifier only understands legacy bucket/object fields; leave deeper restart reconciliation to Story 6.10 but do not worsen current Drive metadata handling.
- Reintroducing GCS copy or requiring `bucket_name/object_key` in public frontend contract.
- Exposing `local_path`, raw Google API body, token/private_key, credential path, or service account JSON.

## File References

### Backend likely UPDATE files

- `apps/agent/internal/upload/retry_policy.go` — add Drive retryable actions/codes and tests.
- `apps/agent/internal/upload/retry_runner.go` — strengthen no-op/double-click behavior and response reasons if tests reveal gaps.
- `apps/agent/internal/upload/worker.go` — ensure retry path preserves dedupe identity and does not upload duplicate while status running/uploaded.
- `apps/agent/internal/upload/drive_uploader.go` — enhance dedupe lookup/appProperties if implemented.
- `apps/agent/internal/upload/jobs.go` — verify deterministic dedupe key and validation for Drive-only jobs.
- `apps/agent/internal/upload/jobs_persistence.go` — ensure retry metadata persists/reloads.
- `apps/agent/internal/api/session_uploads.go` — retry endpoint response/SSE/activity safe metadata.
- `apps/agent/internal/upload/*_test.go`, `apps/agent/internal/api/*upload*_test.go` — add regression tests.

### Frontend likely UPDATE files

- `apps/web/src/lib/api/client.ts` — ensure runtime guard/type supports retry metadata, Drive-only identity, optional legacy fields.
- `apps/web/src/features/sessions/live-station-cards.tsx` — upload/retry labels and retry count/next action visibility.
- `apps/web/src/features/sessions/use-retry-cloud-upload-mutation.ts` — likely no logic change; verify endpoint/invalidation.

### Docs UPDATE files

- `docs/api/cloud-storage-story-6-1.md` — add Story 6.9 retry/dedupe contract.
- `docs/api/openapi.yaml` — retry schemas, Drive IDs, status enum, safe actions.

## Previous Story Intelligence

- Story 6.6 validations passed after fixing Drive settings/root write probe/public allowlist/action mapping.
- Story 6.7 validations passed after fixing retry button target status, separate Drive target summary, preserving ready target identity on transient failure, paginated deterministic folder duplicate lookup.
- Story 6.8 validations passed after fixing settings-backed Drive client factory, safe filename sanitization, existing Drive file reuse before create, OpenAPI GCS cleanup, frontend guard optional legacy fields.
- Current Story 6.9 should build directly on those fixes and should not refactor broad upload architecture unless tests prove a specific bug.

## Git Intelligence Summary

- Git history is sparse and does not reflect most BMad story work; many project files may be untracked.
- Treat current working tree, completed story files 6.6/6.7/6.8, and current planning artifacts as source of truth.
- Developer must inspect actual code before editing; do not infer from git commits.

## Latest Technical Notes

- Google Drive API supports `appProperties` on files for application-private-ish metadata useful for dedupe. Query syntax supports `appProperties has { key='k' and value='v' }`.
- Drive names are not unique; IDs are durable. Dedupe should prefer persisted `drive_file_id`, then appProperties, then parent+safe-name fallback.
- Do not change Drive sharing/permissions in this story.
- Keep scopes narrow and server-side; existing Drive client uses Drive file/metadata scopes from Story 6.8.

## Project Context Reference

- Project: `selfstudio`.
- Stack: Go local service on Windows admin PC + Next.js App Router dashboard + Supabase/Postgres-style metadata/state + local filesystem safety layer.
- API conventions: `{data}` success wrapper, `{error:{code,message,action,details}}` errors, `snake_case` JSON, SSE dot notation.
- Critical invariant: local capture/save/processing remains source of safety and must continue even when Drive upload fails or retries.

## Dev Agent Record

### Debug Log

- 2026-05-20 — Loaded story/context and marked sprint status in-progress.
- 2026-05-20 — Audited upload jobs persistence, worker, retry policy/runner, Drive uploader, recovery, API handler/router wiring, frontend retry controls/runtime guards.
- 2026-05-20 — RED: added/updated retry policy tests for Drive-specific retryability and max 3 total attempts; observed expected failures before code change.
- 2026-05-20 — GREEN/REFACTOR: updated retry policy so `DRIVE_UPLOAD_FAILED` + `RETRY_DRIVE_UPLOAD` is retryable and manual retry respects max attempt limit.
- 2026-05-20 — Added concurrency/no-op tests for uploading/retrying/uploaded retry requests without attempt increments.
- 2026-05-20 — Added Drive retry dedupe test proving existing parent+safe-name Drive file is reused and no duplicate create occurs.
- 2026-05-20 — Added aggregate status tests for partial_failed/failed/uploading/pending/uploaded with not_eligible graded jobs.
- 2026-05-20 — Updated retry DTO compatibility fields (`session_id`, `upload_status`) so web runtime guards accept retry endpoint responses.
- 2026-05-20 — Updated docs/OpenAPI for Story 6.9 retry/dedupe contract and safe no-op semantics.

### Completion Notes

- Implemented Drive-specific retry semantics while preserving legacy cloud retry compatibility. `RETRY_DRIVE_UPLOAD` now participates in auto/manual retry, credential/folder/local-file failures remain non-retryable, and both auto/manual retry honor the 3 total attempts limit.
- Strengthened regression coverage for duplicate-safe retries: double-running/uploaded requests no-op safely without attempt increments, and existing guard behavior prevents concurrent duplicate upload calls.
- Verified Drive dedupe retry path reuses an already-created Drive file found by `drive_folder_id` + sanitized file name and persists `drive_file_id`/`remote_identity` without creating a second remote file. AppProperties are documented as future strengthening; current implementation uses persisted IDs plus parent/safe-name fallback.
- Kept local-first invariant: Drive retry failures do not delete/mutate local outputs or conflate local processing with Drive upload status.
- Updated API retry response shape to remain compatible with the web `SessionUploadsData` guard while still returning retry metadata (`accepted`, `noop_reason`, retry fields, Drive IDs).
- Added minimal UI copy explaining retry count/next action visibility and button pending label; existing server-derived state machine remains authoritative.
- Documentation/OpenAPI now describe retry endpoints, safe fields, no-op reasons, max attempts, statuses, Drive IDs, and dedupe behavior.

### Validation

- PASS — `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`
- PASS — `cd apps/web && npm run typecheck`
- PASS — `cd apps/web && npm run build`
- PASS — `npm run typecheck`

## File List

- `apps/agent/internal/upload/retry_policy.go`
- `apps/agent/internal/upload/retry_policy_test.go`
- `apps/agent/internal/upload/retry_runner_test.go`
- `apps/agent/internal/upload/drive_uploader_test.go`
- `apps/agent/internal/upload/jobs_test.go`
- `apps/agent/internal/api/session_uploads.go`
- `apps/agent/internal/api/session_upload_retries_test.go`
- `apps/web/src/features/sessions/live-station-cards.tsx`
- `docs/api/cloud-storage-story-6-1.md`
- `docs/api/openapi.yaml`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `_bmad-output/implementation-artifacts/6-9-track-retry-and-de-duplicate-google-drive-uploads.md`

## Change Log

- 2026-05-20 — Story prepared for development with comprehensive Google Drive retry/dedupe context; status set to ready-for-dev.
- 2026-05-20 — Implemented Story 6.9 Drive upload retry/dedupe behavior, tests, docs/OpenAPI, and validation; status set to review.

## Completion Note

Ultimate context engine analysis completed - comprehensive developer guide created.
