# Story 6.10: Recover Pending Google Drive Upload Jobs After Restart

Status: done

## Story

Sebagai operator, saya ingin pending Google Drive upload jobs dipulihkan setelah aplikasi restart, sehingga fulfillment dapat lanjut aman tanpa duplicate upload dan tanpa mencampur status local completion dengan status Drive.

## Acceptance Criteria

1. Given app restart saat upload job `pending`, `uploading`, `retrying`, `retry_scheduled`, `failed`, atau partial complete, when Go agent startup, then sistem memuat persisted upload jobs dan menormalkan status in-flight lama secara aman.
2. Given job sebelumnya `uploading`/`retrying` saat proses mati, then job tidak dianggap uploaded otomatis; job dikembalikan ke recoverable state berdasarkan local file dan Drive identity.
3. Given job sudah `uploaded` dengan `drive_file_id`, then recovery menjaga status uploaded hanya bila metadata cukup aman atau menandai perlu check bila verifikasi tidak bisa dilakukan.
4. Given job belum uploaded dan local file masih ada, then upload dapat lanjut asynchronous memakai job/file/folder identity yang sama tanpa mengubah file lokal atau membuat duplicate Drive file.
5. Given local file hilang, then job menjadi failed dengan `UPLOAD_LOCAL_FILE_MISSING` dan action `CHECK_LOCAL_OUTPUT`.
6. Given sebagian jobs uploaded dan sebagian pending/failed, then API/UI menunjukkan aggregate status jujur dan local complete tetap terpisah.
7. Given recovery memperbarui state, then event/activity `upload.recovered`/session updated hanya dipublish setelah durable save berhasil.
8. Given recovery berjalan, then capture/session controls/ingestion/local save/LUT processing tetap tersedia dan tidak diblokir upload recovery.
9. Tests/build pass dan OpenAPI/docs diperbarui.

## Acceptance Criteria Context / BDD Detail

### AC1 — Startup load dan normalisasi status lama

- Recovery entry point saat ini berada di `apps/agent/internal/upload/recovery.go` melalui `StartupRecovery.Recover`.
- Jobs dipersist di `local-data/state/upload_jobs.json` melalui `JobsPersistence`; `LoadOrDefault` harus tetap menjadi sumber state upload setelah restart.
- Status yang wajib ditangani:
  - `pending`: tetap recoverable dan eligible resume jika local file ada.
  - `uploading` dan `retrying`: proses lama mati; jangan anggap sukses. Normalisasi ke `retry_scheduled` atau `failed` sesuai retryability/attempt limit/local file.
  - `retry_scheduled`: resume hanya jika `next_retry_at` sudah due atau nil; jika belum due, jangan upload langsung.
  - `failed`: jangan semua failed di-resume; hanya retryable dan belum exhausted yang boleh dijadwalkan/di-resume sesuai policy.
  - `uploaded`: verify/mark safe sesuai AC3.
  - `not_eligible`: abaikan sebagai terminal non-error.
- Recovery harus idempotent: running recovery dua kali setelah startup tidak boleh menggandakan enqueue, menaikkan attempt count tanpa upload nyata, atau membuat Drive file baru.

### AC2 — In-flight lama tidak dianggap uploaded

- Job `uploading`/`retrying` pada crash harus dianggap ambiguous/incomplete.
- Jika `attempt_count < MaxAutoUploadAttempts` dan error/action retryable, set `retry_scheduled` dengan `next_retry_at = now` agar bisa resume segera.
- Jika sudah exhausted atau non-retryable, set `failed` dengan safe metadata.
- Gunakan Drive-specific action saat mengisi default: prefer `RETRY_DRIVE_UPLOAD` untuk Drive upload failure, bukan fallback user-facing `RETRY_CLOUD_UPLOAD` kecuali compatibility internal diperlukan.
- Jangan mengisi `drive_file_id` atau `remote_identity` baru hanya karena status sebelumnya in-flight.

### AC3 — Uploaded Drive file verification/metadata safety

- Untuk Google Drive, metadata uploaded yang cukup aman minimal adalah `drive_file_id` dan/atau `remote_identity` yang sama dengan Drive file ID, plus `drive_folder_id` untuk parent target jika tersedia.
- Existing helper `hasRemoteMetadata` masih GCS-oriented (`remote_generation`, `remote_etag`) dan harus diupgrade agar Drive-first:
  - `DriveFileID != ""` atau `RemoteIdentity != ""` cukup sebagai identity persisted.
  - Jika `RemoteIdentity` kosong tetapi `DriveFileID` ada, recovery boleh memperbaiki compatibility field `RemoteIdentity = DriveFileID` setelah durable save.
  - Jika `RemoteIdentity` ada tetapi `DriveFileID` kosong dan value tampak Drive identity dari Story 6.8, recovery boleh mengisi `DriveFileID = RemoteIdentity`.
- Jika verifier tersedia, gunakan Drive-aware verifier, bukan `RemoteVerifier.Stat(bucketName, objectKey)` GCS-only. Story ini boleh mengganti/menambah interface, misalnya:
  - `VerifyUploaded(ctx, job) (VerifiedUploadInfo, error)` yang bisa memakai `drive_file_id`/`drive_folder_id`.
  - Backward compatibility untuk legacy `RemoteVerifier` boleh dipertahankan untuk tests lama, tetapi Drive path harus utama.
- Jika verifier tidak tersedia namun `drive_file_id` ada, jangan downgrade uploaded menjadi failed hanya karena tidak bisa network check. Catat sebagai `unverified_uploaded` dan pertahankan status uploaded.
- Jika metadata uploaded tidak cukup aman, tandai `failed` dengan safe code `UPLOAD_REMOTE_CHECK_NEEDED`/existing `UPLOAD_OBJECT_CHECK_NEEDED` dan action Drive-specific `CHECK_DRIVE_FILE` atau compatible safe action. Jangan klaim uploaded.

### AC4 — Resume pending tanpa duplicate Drive file

- Resume harus memakai `Worker.ResumeRecoveredJob` / retry path existing agar tetap melewati `uploadGuard`, `DriveUploader.UploadFile`, safe filename, lookup exact parent+safe-name, dan Drive duplicate prevention dari Story 6.8/6.9.
- Job identity wajib tetap deterministic: `job_id = session_id:photo_id:asset_kind`, `dedupe_key` sama.
- Jangan menjalankan discovery ulang yang membuat job baru untuk pending persisted job kecuali operator memulai upload/session flow eksplisit. Recovery harus memulihkan job persisted, bukan scan arbitrary output folder dan mengarang upload jobs.
- Resume pending/retry_due harus asynchronous setelah state normalisasi tersimpan durable.
- Jika `drive_folder_id` kosong pada job Drive yang belum uploaded, recovery tidak boleh upload ke path/legacy object key sebagai pengganti. Tandai failed dengan action `RESOLVE_CLOUD_TARGET`/`RETRY_DRIVE_FOLDER` sesuai konteks, atau repair dari target/session only jika identity aman dan sudah persisted.

### AC5 — Local file missing

- Untuk semua job non-uploaded/non-not_eligible yang butuh local file, recovery harus cek readability local path internal.
- Jika local file hilang/unreadable:
  - `status = failed`
  - `last_error_code = UPLOAD_LOCAL_FILE_MISSING`
  - `last_error_action = CHECK_LOCAL_OUTPUT`
  - clear `next_retry_at` dan `retry_after`
  - jangan hapus/ubah metadata Drive yang sudah ada.
- API/UI tidak boleh mengekspos full local path; DTO tetap hanya `local_file_name`.

### AC6 — Aggregate status jujur dan local status terpisah

- Gunakan `AggregateUploadStatus(target, jobs)` existing, tapi pastikan recovery state normalization tidak menyebabkan aggregate salah:
  - `uploaded + failed` → `partial_failed`
  - semua eligible uploaded + graded `not_eligible` → `uploaded`
  - ada `retrying`/`uploading` setelah resume → `uploading`
  - ada `pending`/`retry_scheduled` → `pending`
  - failed tanpa uploaded → `failed`
- Local completion/session locked/local save status jangan diubah oleh recovery upload.
- Session detail/live card tetap harus menampilkan Drive target/upload terpisah dari local save/processing.

### AC7 — Publish hanya setelah durable save

- `StartupRecovery.Recover` saat ini publish `upload.recovered` setelah `Persistence.Save` jika `changed`; pertahankan prinsip ini dan perluas agar session update events aman juga dipublish setelah save.
- Jika tidak ada perubahan state tetapi ada uploaded unverified count, boleh publish summary event as informational setelah load selesai.
- Activity log message harus Google Drive-specific dan safe: “Google Drive upload recovery completed”, bukan “Cloud”/GCS copy.
- SSE payload `upload.recovered` harus safe dan useful: summary counts, affected session IDs jika feasible, no `local_path`, no credential, no raw Drive error.

### AC8 — Non-blocking startup/capture/processing

- Recovery tidak boleh melakukan network Drive verification atau resume upload secara synchronous/blocking lama di critical startup path jika itu menunda API readiness/capture/session controls.
- Jika startup wiring sekarang memanggil recovery sebelum server ready, pertimbangkan menjalankan recovery worker goroutine setelah stores loaded dan API server/other workers initialized. Minimal: recovery loops harus non-blocking terhadap watcher/processing dan tidak memegang lock global lama.
- Resume upload boleh serial untuk MVP tetapi harus memakai upload worker path terisolasi; jangan mengambil lock processing/watcher/session.
- Failure Drive/network saat recovery tidak boleh menurunkan local readiness atau mengubah local files.

### AC9 — Tests/build/docs

- Required validation setelah implementasi:
  - `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`
  - `cd apps/web && npm run typecheck`
  - `cd apps/web && npm run build`
  - `npm run typecheck`
- Update `docs/api/cloud-storage-story-6-1.md` dan `docs/api/openapi.yaml` bila event/recovery fields/status/actions berubah.

## Tasks / Subtasks

- [x] Audit recovery wiring dan startup path sebelum coding
  - [x] Baca `apps/agent/internal/upload/recovery.go` lengkap dan tests terkait.
  - [x] Baca `apps/agent/cmd/selfstudio-agent/main.go` untuk melihat kapan `JobsPersistence.LoadOrDefault`, `StartupRecovery.Recover`, worker startup, router, SSE broker, watcher/processing dijalankan.
  - [x] Pastikan recovery tidak memblokir API/server/watcher/processing lebih lama dari operasi local state normalization yang perlu.
- [x] Upgrade recovery dari GCS/object metadata ke Drive-first metadata
  - [x] Update `hasRemoteMetadata` agar `drive_file_id`/`remote_identity` Drive dihitung sebagai uploaded metadata aman.
  - [x] Update `remoteMatches` atau ganti interface verifier agar Drive file ID/folder ID bisa diverifikasi tanpa bucket/object key.
  - [x] Tambahkan repair compatibility: `DriveFileID` ↔ `RemoteIdentity` bila salah satu kosong dan status uploaded.
  - [x] Tambahkan safe code/action untuk “Drive file check needed” jika belum ada; bila memakai legacy code, UI/docs harus tetap Google Drive-safe.
- [x] Normalisasi status in-flight/retry secara aman
  - [x] `uploading`/`retrying` lama → `retry_scheduled` due now jika retryable dan attempts belum exhausted.
  - [x] Default error/action untuk in-flight crash harus Drive-specific (`DRIVE_UPLOAD_FAILED`/`RETRY_DRIVE_UPLOAD` atau compatible safe alias).
  - [x] `retry_scheduled` due → enqueue resume; not due → tetap scheduled.
  - [x] `failed` retryable dan due/existing policy → schedule/resume hanya jika aman; non-retryable tetap failed.
  - [x] `pending` dengan local file dan Drive folder identity valid → enqueue resume.
- [x] Guard local file dan Drive target identity
  - [x] Untuk non-uploaded jobs, cek local file readable; missing → `UPLOAD_LOCAL_FILE_MISSING` + `CHECK_LOCAL_OUTPUT`.
  - [x] Untuk Drive jobs yang belum uploaded, require `drive_folder_id`; jika missing, jangan fallback ke legacy GCS object upload diam-diam.
  - [x] Jangan scan arbitrary output folders; hanya gunakan persisted jobs/targets.
- [x] Durable save dan event/activity ordering
  - [x] Pastikan semua perubahan job disimpan via `JobsPersistence.Save` sebelum `upload.recovered` dan session/upload update events dipublish.
  - [x] Jika save gagal, jangan publish success; return error dengan summary errors.
  - [x] Activity log gunakan Google Drive-safe action/message.
  - [x] Publish session/update events untuk session terdampak setelah save agar UI aggregate refresh jujur.
- [x] Resume asynchronous dan duplicate-safe
  - [x] Resume harus memanggil `Worker.ResumeRecoveredJob` atau path setara yang melewati `uploadGuard`.
  - [x] Pastikan resume tidak accepted dua kali untuk job sama pada recovery rerun/double invocation.
  - [x] Pastikan retry path memakai `DriveUploader.UploadFile` sehingga lookup parent+safe-name dari Story 6.8/6.9 mencegah duplicate remote file.
- [x] API/UI contract verification
  - [x] Pastikan `GET /api/sessions/{session_id}/uploads` setelah recovery menampilkan aggregate status jujur.
  - [x] Pastikan DTO tidak expose `local_path`; only `local_file_name`.
  - [x] Jika ada UI copy “Cloud upload recovery”, ubah ke “Google Drive upload recovery”.
  - [x] Pastikan frontend runtime guard menerima recovered statuses (`pending`, `retry_scheduled`, `retrying`, `failed`, `uploaded`) dengan Drive-only fields.
- [x] Tests backend
  - [x] RED: load persisted `pending` job dengan local file dan `drive_folder_id` → recovery saves if needed, enqueues/resumes async, uses same `job_id`/`dedupe_key`, no duplicate job.
  - [x] RED: persisted `uploading`/`retrying` crash state → normalized to `retry_scheduled` due now or failed if exhausted; not uploaded automatically.
  - [x] RED: uploaded with `drive_file_id` and no verifier → remains uploaded, counted `unverified_uploaded`, aggregate uploaded.
  - [x] RED: uploaded missing Drive identity → `failed` check-needed, not claimed uploaded.
  - [x] RED: local file missing → failed `UPLOAD_LOCAL_FILE_MISSING` + `CHECK_LOCAL_OUTPUT`, no resume.
  - [x] RED: retry_scheduled future → not resumed early; due → resumed once.
  - [x] RED: partial session aggregate after recovery: uploaded + failed = `partial_failed`; original uploaded + graded not_eligible = `uploaded`.
  - [x] RED: event/activity published only after successful `JobsPersistence.Save`; simulate save failure and assert no success publish.
  - [x] RED: recovery resume path reuses existing Drive file from fake Drive uploader and does not create duplicate.
- [x] Docs/build validation
  - [x] Update docs/API for recovery event/action/summary if changed.
  - [x] Run all required validation commands and record results in Dev Agent Record.

### Review Findings

- [x] [Review][Patch] Recovery can duplicate auto-resume with the 5-second auto-retry ticker [apps/agent/cmd/selfstudio-agent/main.go:124]
- [x] [Review][Patch] Drive verifier mismatch can downgrade a valid uploaded Drive job [apps/agent/internal/upload/recovery.go:99]

## Dev Notes

### Konteks Epic / Corrective Change

- Epic 6 adalah corrective path dari GCS menuju Google Drive provider.
- Flow yang sudah dibangun:
  1. Story 6.6 — konfigurasi Google Drive, safe public settings, root write probe, folder preview/sanitasi.
  2. Story 6.7 — create/resolve nested Drive folder per session, final `drive_session_folder_id` sebagai identity target.
  3. Story 6.8 — upload original/graded JPG ke Drive, per-file `drive_folder_id`/`drive_file_id`, subfolder `original`/`graded`, safe API/SSE/activity.
  4. Story 6.9 — retry/dedupe Drive upload, Drive-specific retry action, no duplicate file on retry, aggregate status jujur.
  5. Story 6.10 — recovery after restart untuk persisted upload jobs.
- FR utama: FR55 recover pending Google Drive upload jobs after restart, FR59 track upload status, FR60 retry failed Drive uploads, FR61 preserve local files, FR62 upload non-blocking, FR63 duplicate prevention.
- NFR utama: NFR9 recoverable state after restart, NFR11 local save independent dari Drive, NFR16 retries duplicate-safe, NFR26 remote status cukup untuk reduce duplicate, NFR32 local-vs-Drive status separated.

### Prior Story 6.6 Learnings yang Wajib Dipakai

- Provider publik adalah `google_drive`; jangan reintroduce GCS/bucket/object sebagai operator-facing mental model.
- Public cloud settings allowlist Drive-safe: `provider`, `drive_root_folder_id`, `drive_root_folder_name`, `folder_naming_template`, `credentials_configured`, `connection_status`, `last_checked_at`, `last_error_code`, `last_error_action`.
- Browser/API/SSE/activity tidak boleh menerima credential file path, service account JSON, OAuth token, refresh token, private key, atau raw privileged error.
- Root folder authorization harus termasuk write/probe capability.
- Folder rule errors harus Drive-specific (`FIX_DRIVE_FOLDER_RULES`).

### Prior Story 6.7 Learnings yang Wajib Dipakai

- `SessionCloudTarget` punya `DriveSessionFolderID` dan `DriveFolderChain`; `remote_identity` target adalah final Drive session folder ID, bukan path.
- Drive folder resolver lookup/create parent+name idempotent dan memilih duplicate deterministic.
- Ready target identity harus dipertahankan saat transient failure; upload recovery tidak boleh overwrite target ready karena Drive/network error.
- Session detail punya Drive target fields terpisah dari upload status.

### Prior Story 6.8 Learnings yang Wajib Dipakai

- `FileUploadJob` punya `DriveFolderID`, `DriveFileID`; `RemoteIdentity` mirror `DriveFileID` setelah upload success.
- Original/graded asset subfolders dibuat/resolve idempotent di bawah `drive_session_folder_id`.
- Production `DriveUploader` memakai settings-backed client factory; credential changes tidak perlu restart.
- `DriveUploader.UploadFile` sanitizes file name via `cloud.SafeFileName`, lookup exact parent+safe-name+not-trashed, reuse existing file before create.
- Frontend runtime guard tidak boleh mewajibkan legacy `bucket_name`/`object_key`.

### Prior Story 6.9 Learnings yang Wajib Dipakai

- `RETRY_DRIVE_UPLOAD` sudah menjadi action retryable untuk auto/manual retry; credential/folder/local-file failures tetap non-retryable.
- Manual/auto retry honor `MaxAutoUploadAttempts = 3` total attempts.
- Double-click/running/uploaded retries no-op aman tanpa attempt increment.
- Drive dedupe retry path sudah testable: existing remote file by `drive_folder_id` + safe filename direuse dan `drive_file_id`/`remote_identity` dipersist tanpa create kedua.
- API retry DTO mengandung retry metadata dan Drive IDs; no `local_path`.

### Current Code State yang Harus Dibaca Developer

- `apps/agent/internal/upload/recovery.go`
  - Saat ini masih GCS-oriented: `RemoteVerifier.Stat(ctx, bucketName, objectKey)`, `RemoteObjectInfo.RemoteGeneration`, `RemoteMetageneration`, `RemoteETag`.
  - `hasRemoteMetadata` belum Drive-first; uploaded job dengan hanya `drive_file_id` dapat salah dianggap perlu check/fail jika `remote_identity` kosong.
  - `uploading`/`retrying` dinormalisasi ke `retry_scheduled`, tetapi default action masih `RETRY_CLOUD_UPLOAD`; update ke Drive-specific/correct compatibility.
  - `publishSuccess` memakai action `cloud.upload_recovery_completed` dan message “Cloud upload recovery completed”; ubah copy/event payload ke Google Drive-safe.
  - Recovery memanggil `ResumeRecoveredJob` setelah publish success; pastikan durable save sudah selesai sebelum publish/resume.
- `apps/agent/internal/upload/jobs.go`
  - Status valid termasuk `retrying`, `retry_scheduled` dari retry policy.
  - `validateJob` mengizinkan Drive-only identity jika `DriveFolderID` ada; legacy bucket/object fallback masih ada compatibility.
  - `BuildFileObjectKey` masih ada untuk compatibility/display, bukan Drive remote identity utama.
- `apps/agent/internal/upload/jobs_persistence.go`
  - `JobsPersistence.Save` atomic temp+rename dan sudah diserialisasi dengan `jobsStateSaveMu` untuk Windows.
  - Tests harus memakai temp local data dir dan `.gotmp` command sesuai project pattern.
- `apps/agent/internal/upload/retry_runner.go`
  - `ResumeRecoveredJob` memanggil `retryJobs(... manual=false, allowPending=true)`.
  - `retryJobs` set status `retrying`, save, publish, lalu goroutine `uploadOneGuarded`.
  - `uploadGuard` hanya in-memory single process; sufficient for MVP local agent. Recovery rerun harus tetap no-op/guarded.
- `apps/agent/internal/upload/worker.go`
  - `uploadOneGuarded` increments attempt count only when actual run starts.
  - `uploadDriveOrLegacy` uses DriveUploader if `DriveUploader != nil || DriveFolderID != ""`.
  - If `DriveUploader == nil` but job has DriveFolderID, safe error `CLOUD_TARGET_NOT_READY`/`RESOLVE_CLOUD_TARGET`.
- `apps/agent/internal/api/session_uploads.go`
  - DTO excludes `local_path`, includes `drive_folder_id`, `drive_file_id`, retry fields.
  - SSE publish includes Drive IDs and retry fields; no full path.
  - Activity copy mostly Google Drive-safe for upload/retry; recovery copy in `recovery.go` still needs Drive-specific wording.
- `apps/web/src/lib/api/client.ts`
  - Runtime guard was fixed in Story 6.8/6.9; verify it still accepts recovered Drive-only jobs and statuses.
- `apps/web/src/features/sessions/live-station-cards.tsx`
  - Upload retry button/status uses server aggregate; do not add frontend-owned recovery state machine.

### Architecture and Guardrails

- Go service owns filesystem, recovery, worker, Drive API, credential, retry, and upload state.
- Next.js/browser only renders API/SSE state and triggers trusted authenticated commands.
- API convention: REST `/api`, SSE `/events`, success `{data}`, error `{error:{code,message,action,details}}`, JSON `snake_case`.
- SSE event names use dot notation. `upload.recovered` is explicitly required.
- Local files are source of safety. Recovery/upload failure must never delete, move, or mutate local original/graded files.
- Recovery must not conflate local completion with Drive uploaded. Local status, Drive target status, and Drive upload status remain separate.
- Never expose full local path, credential path, token, private key, service account JSON, raw Google API body, or raw privileged error to API/SSE/activity/UI.

### Google Drive Recovery Technical Notes

- Drive file IDs are durable identities; Drive file/folder names are not unique.
- For uploaded jobs, prefer persisted `drive_file_id`. `remote_identity` is compatibility mirror.
- For pending/retry jobs, dedupe on resume is handled by `DriveUploader.UploadFile` lookup parent folder + safe filename before create. Ensure recovered jobs keep `drive_folder_id` and `local_path` intact.
- If implementing Drive verifier, verify by file ID, optionally parent contains `drive_folder_id`, not trashed, MIME image/jpeg if available. Do not require public permissions.
- If no verifier exists at startup, preserving uploaded with `drive_file_id` as `unverified_uploaded` is safer than downgrading and causing reupload/duplicate.

### Testing Strategy

Backend priority tests:

1. `StartupRecovery` loads pending persisted Drive job:
   - local file exists, `drive_folder_id` set.
   - expect `EnqueueIDs` contains same job ID and job not duplicated.
2. Crash in-flight normalization:
   - `uploading`/`retrying` job with attempts below max → `retry_scheduled` due now and resumed.
   - exhausted job → `failed`, no enqueue.
3. Uploaded metadata:
   - `uploaded` with `drive_file_id` and no verifier stays uploaded, `unverified_uploaded++`.
   - `uploaded` with `drive_file_id` and Drive verifier success → `verified_uploaded++`.
   - `uploaded` without Drive identity → failed/check-needed.
4. Local missing:
   - pending/retry_scheduled failed with `UPLOAD_LOCAL_FILE_MISSING` + `CHECK_LOCAL_OUTPUT`; no enqueue.
5. Future schedule:
   - `retry_scheduled` with `next_retry_at` in future stays scheduled and not resumed.
6. Aggregate:
   - after recovery, call API/status helper and verify partial/failed/uploaded cases.
7. Durable save/event ordering:
   - simulate persistence save failure (may need injectable persistence or wrapper) and assert no success event/activity.
8. Duplicate-safe resume:
   - fake Drive uploader with existing file by parent+safe-name; recovery resume uploads/reuses same `drive_file_id` and create count remains 0/unchanged.

Frontend/type validation:

- Ensure `FileUploadJob` guard accepts all recovered statuses and Drive-only identity.
- UI labels should remain Google Drive wording and text labels should show aggregate status/actions.

### Regression Risks to Avoid

- Downgrading uploaded Drive jobs because recovery only understands GCS generation/etag.
- Re-uploading already uploaded jobs because `drive_file_id` not treated as remote identity.
- Resuming jobs with missing `drive_folder_id` via legacy object key and creating wrong remote assets.
- Publishing `upload.recovered` before failed save, causing UI to believe recovery happened when state is not durable.
- Blocking startup/capture/processing while Drive network verifier is slow.
- Reintroducing GCS copy/actions in operator-facing messages.
- Exposing `local_path` or credentials through recovery events/activity/details.

## File References

### Backend likely UPDATE files

- `apps/agent/internal/upload/recovery.go` — core recovery normalization, Drive metadata handling, event/activity copy, resume sequencing.
- `apps/agent/internal/upload/recovery_test.go` — add comprehensive Drive recovery tests.
- `apps/agent/internal/upload/jobs.go` — add/adjust safe Drive check-needed constants/actions if needed.
- `apps/agent/internal/upload/retry_policy.go` — verify Drive retryability remains correct for recovered jobs.
- `apps/agent/internal/upload/retry_runner.go` — only adjust if recovery tests reveal no-op/resume issues.
- `apps/agent/internal/upload/worker.go` — only adjust if recovered Drive jobs missing identity need safe failure/repair handling.
- `apps/agent/cmd/selfstudio-agent/main.go` — startup wiring/non-blocking recovery and Drive verifier wiring if required.
- `apps/agent/internal/api/session_uploads.go` — ensure API/SSE safe fields/status after recovery; likely minimal change.

### Frontend likely UPDATE files

- `apps/web/src/lib/api/client.ts` — verify runtime guard/status enum Drive-only recovered jobs.
- `apps/web/src/features/sessions/live-station-cards.tsx` — only if copy/status labels need recovery/Drive wording tweaks.

### Docs UPDATE files

- `docs/api/cloud-storage-story-6-1.md` — add Story 6.10 recovery contract.
- `docs/api/openapi.yaml` — update `upload.recovered` event/schema/status/action docs if documented.

## Previous Story Intelligence

- Story 6.6 validations passed after Drive settings/root write probe/public allowlist/action mapping fixes.
- Story 6.7 validations passed after target status UI fix, separate Drive target summary, ready identity preservation, paginated deterministic folder lookup.
- Story 6.8 validations passed after settings-backed Drive client factory, safe filename sanitization, existing file reuse, OpenAPI GCS cleanup, frontend guard optional legacy fields.
- Story 6.9 validations passed after Drive retryability, no-op double retry, remote dedupe retry tests, aggregate status tests, docs/OpenAPI updates.
- Current implementation state shows recovery is the remaining area still materially GCS-shaped; Story 6.10 should focus there, not broad refactors of upload worker/discovery unless tests require.

## Git Intelligence Summary

- Git history is sparse and not representative of BMad story work; current working tree and story records are source of truth.
- Do not infer feature absence from git commits; inspect actual files before edits.

## Latest Technical Notes

- Google Drive file verification can use `files.get(fileId).Fields("id,name,mimeType,parents,trashed,modifiedTime")`; if implemented, keep server-side only and sanitize errors.
- Drive duplicate prevention on resume should rely on persisted `drive_file_id`, then `DriveUploader.UploadFile` parent+safe-name lookup. AppProperties are a future strengthening but not mandatory if Story 6.9 parent+name fallback remains tested.
- No public sharing/permission changes are required for recovery.

## Project Context Reference

- Project: `selfstudio`.
- Stack: Go local service on Windows admin PC + Next.js App Router dashboard + Supabase/Postgres-style metadata/state + local filesystem safety layer.
- API conventions: `{data}` success wrapper, `{error:{code,message,action,details}}` errors, `snake_case` JSON, SSE dot notation.
- Critical invariant: local capture/save/processing remains source of safety and must continue even when Google Drive upload/recovery fails.

## Dev Agent Record

### Debug Log

- 2026-05-20 — Story prepared by context engine from planning artifacts, draft Story 6.10, completed Stories 6.6–6.9, and current upload recovery/job/worker/API code.
- 2026-05-20 — Audit recovery.go, recovery_test.go, main.go, worker/retry/API/client code sebelum implementasi; confirmed upload recovery was GCS-oriented and startup server already starts before recovery.
- 2026-05-20 — Added RED backend tests for Drive pending/in-flight recovery, uploaded Drive metadata, missing local file, due/future retry scheduling, aggregate status, durable save failure ordering, and guarded duplicate-safe resume.
- 2026-05-20 — Implemented Drive-first recovery metadata handling, Drive-specific crash defaults, local/target guards, durable-save-before-publish behavior, Google Drive-safe activity copy, and asynchronous startup recovery invocation.
- 2026-05-20 — First full Go validation with .gotmp2 hit transient Windows temp cleanup failure in internal/api; reran with .gotmp3 and passed.
- 2026-05-20 — Addressed review patch findings with RED tests for recovery/auto-retry double resume and Drive verifier mismatch preservation; coordinated recovery and auto retry via shared worker recovery mutex, and kept uploaded Drive jobs with `drive_file_id` uploaded when verifier mismatch/transient response cannot safely disprove persisted identity.

### Completion Notes

- Implemented Story 6.10 Google Drive upload recovery: persisted pending/retry jobs are normalized and resumed via `Worker.ResumeRecoveredJob`, while stale in-flight jobs are not marked uploaded automatically.
- Uploaded Drive jobs now treat `drive_file_id`/`remote_identity` as safe identity, repair compatibility mirror fields, and remain `uploaded` as `unverified_uploaded` when no verifier is available. Jobs without safe uploaded identity are marked `UPLOAD_REMOTE_CHECK_NEEDED` + `CHECK_DRIVE_FILE`.
- Recovery now preserves local-first behavior: missing local files fail with `UPLOAD_LOCAL_FILE_MISSING` + `CHECK_LOCAL_OUTPUT`; pending Drive jobs require persisted Drive target identity and no output-folder rescanning was introduced.
- `upload.recovered` and Google Drive recovery activity are published only after durable save succeeds; save failure tests assert no success publish and no resume.
- Startup upload recovery now runs in a goroutine after server startup and processing recovery enqueue, so API/capture/processing readiness is not blocked by upload resume work.
- API/UI contracts remain safe: DTO still exposes `local_file_name` only, frontend guard already accepts recovered statuses and Drive-only fields, and docs/OpenAPI were updated for recovery/action semantics.
- ✅ Resolved review finding [Patch]: Recovery resume and the 5-second `AutoRetryDue` ticker now coordinate through `Worker.RecoveryMu`, preventing recovery rerun/ticker overlap from double-enqueueing or double-attempting the same due job.
- ✅ Resolved review finding [Patch]: Uploaded Google Drive jobs with persisted `drive_file_id` are no longer downgraded on verifier mismatch/transient verifier uncertainty; Drive-aware matching prioritizes `drive_file_id`/Drive identity and records such cases as unverified uploaded rather than failed.

### Validation

- `cd apps/agent && GOTMPDIR=../../.gotmp2 go test ./internal/upload` — PASS.
- `cd apps/agent && GOTMPDIR=../../.gotmp2 go test ./...` — initial run had transient Windows TempDir cleanup failure in `internal/api`; no code assertion failure.
- `cd apps/agent && GOTMPDIR=../../.gotmp3 go test ./...` — PASS.
- `cd apps/web && npm run typecheck` — PASS.
- `cd apps/web && npm run build` — PASS.
- `npm run typecheck` — PASS.
- `cd apps/agent && GOTMPDIR=../../.gotmp go test ./internal/upload` — RED: failed expected new Drive verifier mismatch test before fix.
- `cd apps/agent && GOTMPDIR=../../.gotmp3 go test ./internal/upload` — PASS after patch fixes.
- `cd apps/agent && GOTMPDIR=../../.gotmp4 go test ./...` — initial run hit transient Windows `compile.exe: Access is denied` in `internal/api`; no code assertion failure.
- `cd apps/agent && GOTMPDIR=../../.gotmp5 go test ./...` — PASS.
- `cd apps/web && npm run typecheck` — PASS.
- `cd apps/web && npm run build` — PASS.
- `npm run typecheck` — PASS.

## File List

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/upload/jobs.go`
- `apps/agent/internal/upload/recovery.go`
- `apps/agent/internal/upload/recovery_test.go`
- `apps/agent/internal/upload/retry_policy.go`
- `apps/agent/internal/upload/worker.go`
- `docs/api/cloud-storage-story-6-1.md`
- `docs/api/openapi.yaml`
- `_bmad-output/implementation-artifacts/6-10-recover-pending-google-drive-upload-jobs-after-restart.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`

## Change Log

- 2026-05-20 — Story prepared for development with comprehensive Google Drive upload recovery context; status set to ready-for-dev.
- 2026-05-20 — Implemented Drive-first pending upload job recovery after restart, async resume, safe metadata normalization, durable recovery events, docs/OpenAPI updates, and validations; status set to review.
- 2026-05-20 — Addressed code review patch findings: coordinated recovery with auto-retry ticker to prevent duplicate resume/attempts, and made Drive verifier mismatch handling preserve valid uploaded jobs with `drive_file_id`; status remains review.

## Completion Note

Ultimate context engine analysis completed - comprehensive developer guide created.
