# Story 6.8: Upload Original and Graded JPGs to Google Drive

Status: done

## Story

Sebagai operator, saya ingin original dan graded JPG di-upload ke Google Drive setelah local completion, sehingga hasil session siap dibuka/dibagikan dari folder Drive tanpa mengganggu capture dan processing lokal.

## Acceptance Criteria

1. Given session sudah terkunci dan local original/graded state aman, when upload worker berjalan, then original JPG yang tersimpan lokal di-upload ke Google Drive session folder.
2. Given graded JPG berhasil dibuat, then graded JPG di-upload ke Drive subfolder atau naming yang jelas untuk `graded`.
3. Given graded processing gagal tetapi original tersimpan, then original tetap eligible upload dan graded dicatat `not_eligible`/failed tanpa menghapus local file.
4. Given upload sedang berjalan/gagal, then operator masih bisa menjalankan capture/session baru dan local save/LUT processing tidak diblokir.
5. Given upload berhasil, then sistem menyimpan per-file Drive identity: `drive_file_id`, `drive_folder_id`, `photo_id`, `asset_kind`, file name, status, attempt count, timestamps.
6. Given upload gagal karena Drive/network/credential error, then local file tetap dipertahankan dan per-file status menjadi failed dengan safe error/action.
7. Given API/SSE/activity/UI menampilkan status, then tidak ada credential/token/private key/raw Drive error/full local path yang bocor.
8. Tests/build pass dan OpenAPI/docs diperbarui.

## Acceptance Criteria Context / BDD Detail

### AC1 — Upload original hanya setelah local state aman

- Trigger utama adalah endpoint existing `POST /api/sessions/{session_id}/uploads/start` melalui `SessionUploadsHandler.Start` dan `upload.Worker.StartSession`.
- Handler/worker wajib mempertahankan guard session: upload hanya boleh dimulai untuk session `locked` atau state setara local-complete yang sudah dipakai sistem. Session `active` harus tetap ditolak dengan `UPLOAD_PENDING_LOCAL_COMPLETION` + `WAIT_FOR_LOCAL_COMPLETION`.
- Target Drive folder harus sudah resolved oleh Story 6.7: `SessionCloudTarget.Status == ready`, `drive_session_folder_id` terisi, dan `remote_identity == drive_session_folder_id`. Jika belum, return safe error `CLOUD_TARGET_NOT_READY` + action `RESOLVE_CLOUD_TARGET` / copy UI `Resolve Drive Folder`.
- Original eligible jika photo record punya `OriginalSaveStatus == saved_original` dan `LocalOriginalPath` readable oleh Go service. Jangan upload dari source path kamera; upload harus dari final local original output.
- Jangan menunggu semua upload selesai secara synchronous dalam HTTP request. Endpoint boleh discover/create jobs, persist state, lalu menjalankan worker async seperti pola existing.

### AC2 — Upload graded dengan lokasi/naming jelas

- Graded eligible jika `GradedProcessingStatus == processed` dan `LocalGradedPath` readable.
- Struktur Drive yang direkomendasikan untuk Story 6.8:
  - Original: di subfolder `original` di bawah final `drive_session_folder_id`.
  - Graded: di subfolder `graded` di bawah final `drive_session_folder_id`.
- Jika implementasi memilih naming berbasis path virtual, tetap harus menyimpan `drive_folder_id` aktual untuk parent upload dan `drive_file_id` hasil upload. Karena Drive tidak punya object key seperti GCS, jangan jadikan path string sebagai remote identity utama.
- Subfolder `original` dan `graded` harus dibuat/resolve idempotent dengan lookup parent+name sebelum create, memakai Drive folder semantics yang sama dengan Story 6.7.

### AC3 — Graded gagal tidak menghalangi original

- Untuk setiap photo yang original-nya saved dan readable, buat/pertahankan job `asset_kind = original` meskipun graded gagal.
- Jika `GradedProcessingStatus` adalah `failed` atau `not_eligible`, buat/pertahankan job deterministic `asset_kind = graded` dengan `status = not_eligible`, bukan mencoba upload file kosong/missing.
- Jika graded status masih `pending`/`processing`, upload session harus melaporkan `pending_local_completion` untuk bagian graded yang belum final, tetapi original yang sudah eligible boleh tetap dibuat job-nya. Jangan hapus atau ubah local processing state.
- Jika graded status `processed` tetapi local graded file missing/unreadable, catat graded job `failed` dengan `UPLOAD_LOCAL_FILE_MISSING` + `CHECK_LOCAL_OUTPUT`.

### AC4 — Upload tidak memblokir capture/session/processing

- Drive upload harus tetap di package Go `apps/agent/internal/upload` dan berjalan async/background. Jangan memindahkan upload ke Next.js/browser.
- Jangan memakai lock global yang menghalangi folder watcher, ingest router, session start/end, original save, LUT processing, quarantine, atau dashboard API.
- Worker boleh melakukan upload per job serial untuk MVP, tetapi harus isolated dari queue processing. Jika memakai concurrency, batasi aman dan test duplicate guard.
- Error Drive/network tidak boleh mengubah local session/photo/processing/quarantine state dan tidak boleh menghapus local files.

### AC5 — Persist per-file Drive identity

- Setelah upload success, job harus menyimpan minimal:
  - `drive_file_id` = ID file Drive hasil upload.
  - `drive_folder_id` = ID folder Drive tempat file di-upload (`original`/`graded` subfolder atau final session folder jika tidak membuat subfolder).
  - `photo_id`, `session_id`, `station_id`, `asset_kind`.
  - `local_file_name` pada DTO publik; jangan expose full `local_path` ke browser.
  - `status`, `attempt_count`, `created_at`, `updated_at`, `last_attempt_at`, `uploaded_at`.
  - `dedupe_key` deterministic (`session_id:photo_id:asset_kind` existing sudah benar).
- `remote_identity` untuk upload file harus sama dengan atau berisi `drive_file_id`; jangan pakai `gs://...`, bucket, object key, generation sebagai identity utama.
- Field legacy `bucket_name`, `object_key`, `remote_generation`, `remote_metageneration` boleh dipertahankan internal/backward-compatible jika masih dibutuhkan compile/tests, tetapi public DTO/API/OpenAPI harus mengutamakan Drive fields dan tidak menampilkan GCS sebagai model mental utama.

### AC6 — Failure mapping aman

- Credential invalid/unauthorized/token error: safe code/action Drive-specific seperti `DRIVE_UPLOAD_UNAUTHORIZED`/`FIX_DRIVE_CREDENTIALS` atau reuse existing safe code dengan action `FIX_DRIVE_CREDENTIALS`.
- Drive folder missing/not writable: action `RETRY_DRIVE_FOLDER` atau `FIX_DRIVE_FOLDER`.
- Network/rate limit/transient API: action `RETRY_DRIVE_UPLOAD` atau existing `RETRY_CLOUD_UPLOAD` dengan UI copy Google Drive.
- Local file missing: `UPLOAD_LOCAL_FILE_MISSING` + `CHECK_LOCAL_OUTPUT`.
- Raw Google API error body, credential file path, service account email/private key/token/refresh token tidak boleh masuk API/SSE/activity. Technical detail boleh ada di local structured logs setelah sanitasi.

### AC7 — API/SSE/activity/UI aman

- `GET /api/sessions/{session_id}/uploads` dan `POST /uploads/start` harus return `{data}` dengan jobs DTO safe. Jangan return `local_path`; return `local_file_name` saja.
- SSE upload events harus membawa safe metadata: `session_id`, `station_id`, `photo_id`, `asset_kind`, `status`, `drive_folder_id`, `drive_file_id`, `last_error_code`, `last_error_action`, `attempt_count`, timestamps/retry fields jika perlu. Jangan kirim full path lokal atau raw Drive error.
- Activity log messages harus operator-safe: “Google Drive upload started/failed/uploaded”, bukan raw exception.
- UI copy harus konsisten Google Drive: “Start Drive Upload”, “Retry Drive Upload”, “Drive upload failed”, “Original uploaded”, “Graded not eligible”. Hindari “bucket”, “GCS”, “object generation” pada UI.

### AC8 — Tests/build/docs

- Minimal validasi setelah implementasi:
  - `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`
  - `cd apps/web && npm run typecheck`
  - `cd apps/web && npm run build`
  - `npm run typecheck`
- Update `docs/api/cloud-storage-story-6-1.md` dan `docs/api/openapi.yaml` untuk kontrak Drive upload per-file.
- Tambahkan tests untuk Drive uploader real abstraction dengan fake client; jangan membutuhkan network/credential Google nyata.

## Tasks / Subtasks

- [x] Audit state upload existing dan ganti asumsi GCS menjadi Drive file upload
  - [x] Baca lengkap `apps/agent/internal/upload/jobs.go`, `uploader.go`, `worker.go`, `retry_runner.go`, `recovery.go`, `jobs_persistence.go`, `persistence.go`, dan tests terkait.
  - [x] Identifikasi field legacy `bucket_name/object_key/remote_generation` yang masih dibutuhkan internal vs yang harus diganti/ditambah dengan `drive_folder_id/drive_file_id`.
  - [x] Pastikan migration perubahan model tetap backward-compatible terhadap JSON state lama.
- [x] Tambahkan model Drive upload identity per file
  - [x] Tambahkan field `DriveFolderID` dan `DriveFileID` ke `upload.FileUploadJob` dengan JSON `drive_folder_id`, `drive_file_id`.
  - [x] Update `validateJob`, persistence, aggregate status, retry code, DTO API, TypeScript type `FileUploadJob`, dan OpenAPI.
  - [x] Pastikan public DTO tidak expose `local_path`; `local_file_name` tetap safe.
- [x] Implement Drive file uploader server-side
  - [x] Tambahkan interface Drive file client/uploader di `apps/agent/internal/upload` untuk create/resolve folder dan upload file bytes ke Drive.
  - [x] Implement production client menggunakan Google Drive API credential server-side dari Story 6.6/6.7; browser tidak pernah menerima credential.
  - [x] Gunakan upload metadata: file name safe, parent folder ID, mime type `image/jpeg`.
  - [x] Map Google/network/credential errors ke safe `SafeUploadError` code/action.
  - [x] Tambahkan fake/in-memory Drive uploader untuk tests.
- [x] Resolve/create asset subfolders idempotent
  - [x] Di bawah `drive_session_folder_id`, lookup/create folder `original` dan `graded` dengan parent+name+mimetype folder+not trashed.
  - [x] Simpan `drive_folder_id` pada job sesuai `asset_kind`.
  - [x] Reuse existing Story 6.7 folder client jika bisa; jangan duplikasi query escaping/credential logic secara unsafe.
- [x] Update job discovery dan object key logic menjadi Drive-safe
  - [x] Refactor `BuildFileObjectKey` atau tambahkan fungsi baru `BuildDriveUploadPlan` yang menghasilkan safe file name + target folder ID, bukan GCS object key sebagai identity.
  - [x] Original saved/readable → job pending/uploadable.
  - [x] Graded processed/readable → job pending/uploadable.
  - [x] Graded failed/not_eligible → job `not_eligible` deterministic.
  - [x] Missing local original/graded final path → failed safe, local file tidak dihapus.
- [x] Update upload worker execution
  - [x] `uploadOneGuarded` harus memanggil Drive uploader dengan `drive_folder_id`, safe file name, local path.
  - [x] Success mengisi `drive_file_id`, `remote_identity`, `uploaded_at`, clear errors, status `uploaded`.
  - [x] Failure menyimpan status failed/retry scheduled sesuai retry policy existing tanpa raw error leak.
  - [x] Pertahankan `uploadGuard` agar duplicate concurrent upload job tidak berjalan ganda.
- [x] Update API/SSE/activity dan frontend UI
  - [x] `SessionUploadsHandler` DTO mengisi `drive_folder_id` dan `drive_file_id`; legacy fields tidak menjadi tampilan utama.
  - [x] SSE payload upload menyertakan safe Drive IDs dan tidak menyertakan full path lokal.
  - [x] Update `apps/web/src/lib/api/client.ts` type `FileUploadJob` untuk Drive fields.
  - [x] Update komponen upload/session detail/live cards agar copy Google Drive dan status original/graded jelas.
- [x] Tests backend
  - [x] RED: original saved uploads to Drive original folder and stores `drive_file_id/drive_folder_id`.
  - [x] RED: graded processed uploads to Drive graded folder and stores identity.
  - [x] RED: graded failed creates `not_eligible` while original still uploads.
  - [x] RED: target not ready returns `CLOUD_TARGET_NOT_READY` + resolve action; no file upload attempted.
  - [x] RED: local file missing fails safe and preserves local path/file state.
  - [x] RED: credential/network/Drive API error maps safe action, no secret/raw error in API/SSE/activity.
  - [x] RED: retry/duplicate guard does not create duplicate Drive file upload for same deterministic job.
- [x] Docs/build validation
  - [x] Update `docs/api/cloud-storage-story-6-1.md` with Story 6.8 Drive upload contract.
  - [x] Update `docs/api/openapi.yaml` if session upload schemas are documented there.
  - [x] Run all validation commands and record results in Dev Agent Record.


### Review Follow-ups (AI)

- [x] [AI-Review][HIGH] Production uploader now uses a settings-backed Drive client factory/provider so uploads and subfolder resolution read the latest Drive settings instead of a startup credential snapshot.
- [x] [AI-Review][MEDIUM] Drive upload file names are sanitized through `cloud.SafeFileName` before lookup/create/upload, with regression coverage for unsafe characters.
- [x] [AI-Review][MEDIUM] Drive uploader now performs exact parent+safe-name+not-trashed lookup and reuses the deterministic existing file before create to avoid duplicate files on retry after partial create.
- [x] [AI-Review][LOW] OpenAPI public cloud settings/upload copy was updated away from GCS bucket/object terminology toward Google Drive folder/file identity.
- [x] [AI-ReReview][MEDIUM] Frontend upload-job runtime guard still requires legacy `bucket_name` and `object_key` strings in `apps/web/src/lib/api/client.ts:isFileUploadJob`, even though the TypeScript type marks them optional and Drive identity is `drive_folder_id`/`drive_file_id`. This keeps a public GCS-shaped contract alive and can reject Drive-only API responses. Make these fields optional in the guard and prefer/validate Drive IDs for uploaded/Drive jobs.

## Dev Notes

### Konteks Epic / Corrective Change

- Epic 6 sedang di corrective path: mengganti implementasi lama Google Cloud Storage/GCS menjadi Google Drive provider.
- Story 6.6 selesai: konfigurasi Google Drive, safe settings DTO, credential server-side only, root folder writable probe, folder preview sanitasi.
- Story 6.7 selesai: create/resolve nested Drive folder per session dan menyimpan `drive_session_folder_id` sebagai remote identity target.
- Story 6.8 adalah tahap upload file konten: `local complete → resolved Drive session folder → create original/graded upload jobs → upload JPGs → track per-file Drive identity`.
- Story 6.9 akan memperkuat retry/dedupe; Story 6.8 tetap harus tidak memperburuk duplicate safety existing dan harus menyimpan identity cukup untuk 6.9.
- Story 6.10 akan recovery pending upload jobs after restart; Story 6.8 harus mempertahankan persistence JSON yang recoverable.

### Prior Story 6.6 Learnings yang Wajib Dipakai

- Public cloud settings allowlist Drive-safe saja. Jangan reintroduce `bucket_name`, credential path, raw error, token, refresh token, private key, atau service account JSON ke browser/API/SSE/activity.
- `GoogleDriveChecker` sudah punya abstraction/probe pattern untuk server-side credential dan Drive root write validation. Reuse pola credential/client creation yang aman.
- Folder path preview/sanitasi memakai `cloud.SafeDriveSegment` / `BuildDriveFolderPreview`; untuk file name gunakan safe filename helper existing (`cloud.SafeFileName`) atau fungsi setara yang deterministic.
- Review Story 6.6 memperbaiki tiga risiko yang tidak boleh regresi: root folder harus writable, public settings tidak boleh punya legacy/GCS fields, dan action Drive folder rules harus Drive-specific.

### Prior Story 6.7 Learnings yang Wajib Dipakai

- `SessionCloudTarget` sekarang memiliki `DriveSessionFolderID` dan `DriveFolderChain`; `remote_identity` untuk target adalah final Drive session folder ID.
- Resolver sudah lookup/create folder chain idempotent berdasarkan parent+name dan memilih duplicate deterministik (oldest `createdTime`, lalu ID).
- Resolver mempertahankan known ready identity saat transient failure, sehingga Story 6.8 jangan overwrite target ready hanya karena upload gagal.
- UI/session summary sudah punya field Drive target terpisah dari aggregate upload: `drive_target_status`, `drive_session_folder_id`, `drive_folder_path`, root, last error/action.
- Story 6.7 sengaja tidak upload file; semua TODO upload file sekarang berada di Story 6.8.

### Current Code State yang Harus Dibaca Developer

Developer wajib membaca file UPDATE berikut sebelum coding:

- `apps/agent/internal/upload/jobs.go`
  - Saat ini masih memodelkan upload sebagai `bucket_name`, `object_key`, `remote_generation`, `remote_metageneration`, dan helper `BuildFileObjectKey` membuat path `<object_prefix>/<asset_kind>/<filename>`.
  - Untuk Drive, ini harus diganti/dilapisi dengan folder/file ID. `object_key` boleh tetap sebagai display/compat virtual path, tetapi bukan remote identity utama.
  - `JobID(sessionID, photoID, assetKind)` sudah deterministic dan cocok sebagai dedupe key.
- `apps/agent/internal/upload/uploader.go`
  - Interface `Uploader.Upload(ctx, bucketName, objectKey, localPath)` masih GCS/object style.
  - `LocalCopyUploader` test-only mengembalikan `gs://...`; jangan pakai ini sebagai production Drive behavior.
  - Refactor hati-hati agar tests lama tetap bisa dikompilasi atau migrasi ke interface Drive upload baru.
- `apps/agent/internal/upload/worker.go`
  - `StartSession` sudah menjaga session locked dan target ready; preserve guard ini.
  - `discover` sudah membuat original job jika original saved/readable, graded job jika processed/readable, graded failed/not_eligible menjadi `not_eligible`, dan pending processing membuat `complete=false`. Ini sesuai AC3 dan perlu dipertahankan saat menambah Drive fields.
  - `uploadOneGuarded` saat ini memanggil `w.Uploader.Upload(j.BucketName, j.ObjectKey, j.LocalPath)` lalu menyimpan `RemoteIdentity`; ubah agar menyimpan `DriveFileID`/`DriveFolderID` dan tetap async.
- `apps/agent/internal/upload/drive_folders.go`
  - Story 6.7 folder client abstraction sudah ada untuk Drive folder lookup/create. Reuse untuk subfolder `original`/`graded` jika desainnya cocok.
- `apps/agent/internal/api/session_uploads.go`
  - DTO sudah mulai punya `DriveFolderID` dan `DriveFileID`, tetapi `toUploadJobDTOs` saat ini mengisi `DriveFileID` dari `RemoteIdentity` dan belum mengisi `DriveFolderID`.
  - SSE publish mengirim `drive_file_id` dari `RemoteIdentity`; tambahkan folder ID dan pastikan full local path tidak dikirim.
  - Error/activity copy masih beberapa memakai “Cloud upload”; untuk UI/API operator, ubah ke Google Drive jika menyentuh copy.
- `apps/web/src/lib/api/client.ts`
  - Type `FileUploadJob` masih memuat GCS fields dan belum memiliki `drive_folder_id`. Tambahkan Drive fields dan update UI consumption.
- `docs/api/cloud-storage-story-6-1.md`
  - Sudah mendokumentasikan Story 6.6/6.7; tambahkan Story 6.8 upload contract.

### Arsitektur dan Guardrails

- Go service owns all filesystem, worker, credential, upload, Drive API logic. Next.js/browser hanya memanggil API dan render status.
- API convention: REST under `/api`, SSE under `/events`, success `{data}`, error `{error:{code,message,action,details}}`, JSON `snake_case`.
- SSE event names dot notation; upload events existing seperti `upload.started`, `upload.file_uploaded`, `upload.retry_failed`, `upload.session_updated` boleh dipertahankan.
- Local filesystem adalah safety source. Drive upload failure tidak boleh menghapus/memindahkan/memodifikasi local original/graded.
- Upload status harus terpisah dari local save/processing dan Drive folder target status.
- Jangan expose full local path ke browser untuk upload job. `local_file_name` cukup untuk operator; full path boleh tetap internal karena worker perlu membaca file.
- Jangan menambahkan Supabase/Drive credential logic di frontend.

### Google Drive API Implementation Notes

- Drive folders/files adalah resource `files`.
- Folder MIME type: `application/vnd.google-apps.folder`.
- JPG MIME type: `image/jpeg`.
- Upload file metadata minimal: `Name`, `Parents: []string{drive_folder_id}`. Jangan set public permission/share tanpa requirement.
- Untuk duplicate-safety minimal Story 6.8, sebelum create upload boleh mencari existing file by parent+name+not trashed dan reuse/overwrite policy harus jelas. Rekomendasi MVP: jika job belum punya `drive_file_id`, lookup existing exact file di target folder dan pilih deterministik sebelum upload untuk mengurangi duplicate; jika perlu upload baru karena content unknown, Story 6.9 akan memperkuat dedupe. Jangan menciptakan duplicate pada retry setelah success karena existing job uploaded harus no-op.
- Google API errors harus disanitasi. Jangan mengirim raw response yang mungkin mengandung token/path/credential.

### Data Model Recommendation

Disarankan update `FileUploadJob`:

```go
type FileUploadJob struct {
    // existing identity fields...
    DriveFolderID string `json:"drive_folder_id,omitempty"`
    DriveFileID   string `json:"drive_file_id,omitempty"`
    LocalFileName string `json:"-"` // DTO only; do not persist if derivable
    // keep legacy BucketName/ObjectKey only for compatibility/display if needed
}
```

Disarankan interface baru/lapisan adapter:

```go
type DriveFileUploader interface {
    UploadFile(ctx context.Context, folderID string, fileName string, localPath string) (DriveUploadResult, error)
}

type DriveUploadResult struct {
    DriveFileID string
    DriveFolderID string
    RemoteETag string
    AlreadyExisted bool
}
```

Jika ingin mempertahankan `Uploader` lama agar refactor kecil, buat adapter `GoogleDriveUploader` yang menginterpretasi parameter dengan aman hanya sementara, tetapi tetap simpan Drive fields eksplisit di job. Jangan biarkan `bucketName = google-drive` dan `objectKey` menjadi satu-satunya sumber kebenaran.

### UX Copy Guidance

- Gunakan “Google Drive upload”, “Drive file”, “Drive folder”, “Original”, “Graded”.
- Status label harus text-based, bukan warna saja: `Pending`, `Uploading`, `Uploaded`, `Failed`, `Not eligible`.
- Error action: `Resolve Drive Folder`, `Retry Drive Upload`, `Check Local Output`, `Fix Drive Credentials`.
- Jangan tampilkan path lokal penuh; jika operator perlu akses folder lokal, gunakan fitur existing “open local result folder”, bukan upload job row.

### Testing Strategy

Backend priority tests:

1. `Worker.StartSession` dengan target Drive ready dan original saved/readable:
   - expect original job pending/uploaded.
   - expect `drive_folder_id` = original folder ID, `drive_file_id` set after upload.
2. Graded processed/readable:
   - expect graded job upload ke graded folder/name yang safe.
3. Graded failed/not eligible:
   - expect original tetap upload.
   - expect graded job `not_eligible`, no uploader call for graded.
4. Target pending/not ready:
   - expect `SessionUploadTargetPending`, safe error, no uploader calls.
5. Local missing:
   - expect failed `UPLOAD_LOCAL_FILE_MISSING`, local file not deleted, API action `CHECK_LOCAL_OUTPUT`.
6. Safe leak regression:
   - API/SSE/activity bodies must not contain `private_key`, `refresh_token`, `token`, `service_account`, `credential_file_path`, raw local full path, or raw Google error.
7. Duplicate/no-op:
   - uploaded job should not be uploaded again on repeated start/retry.
   - concurrent `uploadOne` on same job should be guarded by existing `uploadGuard`.

Frontend/type tests:

- `FileUploadJob` type supports `drive_folder_id`, `drive_file_id`.
- Upload table/status renders Drive IDs/status and not GCS terminology.
- Existing invalidation after start/retry still refreshes session detail/uploads/activity.

### Regression Risks to Avoid

- Reintroducing GCS as user-facing provider (`bucket`, `object`, `gs://`, generation) after corrective change.
- Treating Drive path or file name as unique identity; Drive requires file IDs.
- Uploading from camera source path instead of local original/graded output.
- Blocking capture/session/processing while Drive upload is slow.
- Marking graded failure as session upload failure that blocks original delivery.
- Exposing raw local paths or credential/error detail through API/SSE/activity/UI.
- Overwriting ready Drive target identity when upload fails.
- Deleting local files after upload success/failure; deletion is not in scope.

## File References

### Likely backend UPDATE files

- `apps/agent/internal/upload/jobs.go` — add Drive file/folder identity fields; refactor object key planning.
- `apps/agent/internal/upload/uploader.go` — replace/add Google Drive file uploader abstraction and safe error mapping.
- `apps/agent/internal/upload/worker.go` — discover jobs, resolve asset subfolders, run Drive upload, persist Drive IDs.
- `apps/agent/internal/upload/drive_folders.go` — reuse folder lookup/create for `original`/`graded` subfolders.
- `apps/agent/internal/upload/jobs_persistence.go` — ensure new fields persist/reload.
- `apps/agent/internal/upload/retry_runner.go` / `retry_policy.go` — verify retry still works with Drive IDs.
- `apps/agent/internal/upload/recovery.go` — ensure recovery sees Drive fields and does not duplicate uploaded jobs.
- `apps/agent/internal/api/session_uploads.go` — safe DTO/SSE/activity for Drive upload jobs.
- `apps/agent/cmd/selfstudio-agent/main.go` — wire production Google Drive uploader/client.
- `apps/agent/internal/upload/*_test.go` and `apps/agent/internal/api/*upload*_test.go` — add/update tests.

### Likely frontend UPDATE files

- `apps/web/src/lib/api/client.ts` — add `drive_folder_id`, `drive_file_id`; reduce reliance on GCS fields.
- Session/upload UI under `apps/web/src/features/sessions` — update upload status rows/buttons/copy if present.
- Live dashboard/session detail components that show upload status — ensure Drive wording and safe fields.

### Docs UPDATE files

- `docs/api/cloud-storage-story-6-1.md` — add Drive upload contract for Story 6.8.
- `docs/api/openapi.yaml` — update schemas/endpoints for session upload job Drive fields.

## Previous Story Intelligence

- Story 6.6 validation passed after fixes:
  - `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`
  - `cd apps/web && npm run typecheck`
  - `cd apps/web && npm run build`
  - `npm run typecheck`
- Story 6.7 validation passed after review fixes:
  - `cd apps/agent && GOTMPDIR=../../.gotmp go test ./internal/upload ./internal/api`
  - `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`
  - `cd apps/web && npm run typecheck`
  - `cd apps/web && npm run build`
  - `npm run typecheck`
- Story 6.7 review findings relevant to 6.8:
  - UI retry button must key off correct Drive/upload status, not an unrelated aggregate.
  - Session detail summary must expose separate Drive target fields; keep upload status separate too.
  - Do not overwrite known ready Drive identity on transient failures.
  - Drive folder lookup must handle pagination/duplicates deterministically.

## Git Intelligence Summary

- Git history is sparse and does not reflect most BMad story work because many files may be untracked or not committed.
- Treat current working tree plus completed story documents 6.6/6.7 as source of truth.
- Developer must inspect actual code before editing; do not infer absence/presence from git commits alone.

## Latest Technical Notes

- Google Drive API file upload should use server-side Go client with Drive scopes already established by settings/checker. Keep scopes as narrow as practical.
- Drive file/folder names are not unique; IDs are the durable identity.
- Parent folder controls organization; store the parent `drive_folder_id` per upload job.
- No public sharing/permissions changes are required in this story unless existing credential mode requires service account folder sharing already configured by admin.

## Project Context Reference

- Project: `selfstudio`.
- Stack: Go local service on Windows admin PC + Next.js App Router dashboard + Supabase/Postgres-style metadata/state + local filesystem safety layer.
- API conventions: `{data}` success wrapper, `{error:{code,message,action,details}}` errors, `snake_case` JSON, SSE dot notation.
- Critical invariant: local capture/save/processing remains source of safety and must continue even when Drive upload fails.

## Dev Agent Record

### Debug Log

- 2026-05-20 — Review follow-up RED: added failing uploader tests for safe filename sanitization and exact parent+name reuse before create; confirmed failures before implementation.
- 2026-05-20 — Review follow-up GREEN/REFACTOR: added refresh-safe Drive client factory/provider wiring, Drive file lookup reuse, `cloud.SafeFileName` upload sanitization, and OpenAPI Drive schema/copy cleanup.
- 2026-05-20 — Loaded story, sprint status, upload worker/job/persistence/retry/recovery/API/frontend/docs context.
- 2026-05-20 — RED: added Drive uploader tests; initial run failed because `DriveUploadResult`, `DriveUploader`, and Drive-safe error constants did not exist.
- 2026-05-20 — GREEN/REFACTOR: added Drive file uploader abstraction/client, Drive per-file identity fields, asset subfolder resolution, worker execution path, safe API/SSE/activity DTOs, frontend type/copy updates, and docs/OpenAPI updates.
- 2026-05-20 — First exact required Go validation with `.gotmp` hit transient Windows `Access is denied` for test binary; reran successfully after formatting and again with required `.gotmp` path.
- 2026-05-20 — Re-review follow-up: updated `isFileUploadJob` runtime guard to make legacy `bucket_name`/`object_key` optional, accept Drive-only identities, and require `drive_file_id` for uploaded jobs.

### Completion Notes

- ✅ Resolved review finding [HIGH]: production uploader no longer holds a stale startup credential snapshot; `main.go` now wires a Drive client factory backed by persisted cloud settings for upload and folder resolution.
- ✅ Resolved review finding [MEDIUM]: Drive filenames are sanitized before upload using `cloud.SafeFileName`, with a regression test proving unsafe characters are normalized.
- ✅ Resolved review finding [MEDIUM]: Drive uploader now looks up exact parent+name existing JPEG files and reuses the deterministic existing ID before creating, preventing duplicates on partial-create retry.
- ✅ Resolved review finding [LOW]: OpenAPI public cloud settings/copy now documents Google Drive provider/root folder fields instead of lingering GCS bucket/object schemas.
- ✅ Resolved re-review finding [MEDIUM]: frontend upload job runtime guard now treats legacy `bucket_name`/`object_key` as optional compatibility fields, accepts Drive-only `drive_folder_id`/`drive_file_id` identity, and validates uploaded jobs with a Drive file ID.
- Implemented Google Drive file upload path for original and graded JPG jobs while keeping local files as safety source.
- Added per-file `drive_folder_id` and `drive_file_id` persistence/API metadata; `remote_identity` now mirrors Drive file ID after upload.
- Added idempotent `original`/`graded` subfolder lookup/create under Story 6.7 `drive_session_folder_id` using existing Drive folder client semantics.
- Added server-side Google Drive file upload client with `image/jpeg` metadata and safe error mapping for credential, folder, network, and local-file failures.
- Kept legacy bucket/object fields compatibility-only so older JSON state can still load while public Drive fields are preferred.
- Updated SSE/activity/API/frontend wording to Google Drive and kept full local paths/raw Google errors/secrets out of public payloads.

### Validation

- PASS — `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`
- PASS — `cd apps/web && npm run typecheck`
- PASS — `cd apps/web && npm run build`
- PASS — `npm run typecheck`
- PASS — Review follow-up regression tests: `cd apps/agent && GOTMPDIR=../../.gotmp go test ./internal/upload -run 'TestDriveUploader(UploadsWithFolderIDAndSafeFileName|ReusesExisting)'`
- PASS — Re-review follow-up focused web typecheck: `cd apps/web && npm run typecheck`
- PASS — Re-review follow-up focused web build: `cd apps/web && npm run build`
- PASS — Re-review follow-up root typecheck: `npm run typecheck`

## File List

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/session_uploads.go`
- `apps/agent/internal/upload/drive_uploader.go`
- `apps/agent/internal/upload/drive_uploader_test.go`
- `apps/agent/internal/upload/jobs.go`
- `apps/agent/internal/upload/uploader.go`
- `apps/agent/internal/upload/worker.go`
- `apps/agent/internal/upload/worker_test.go`
- `apps/web/src/features/sessions/live-station-cards.tsx`
- `apps/web/src/lib/api/client.ts`
- `docs/api/cloud-storage-story-6-1.md`
- `docs/api/openapi.yaml`
- `_bmad-output/implementation-artifacts/6-8-upload-original-and-graded-jpgs-to-google-drive.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`

## Change Log

- 2026-05-20 — Story prepared for development with comprehensive Google Drive upload context; status set to ready-for-dev.
- 2026-05-20 — Implemented Story 6.8 Google Drive original/graded JPG upload with Drive file/folder identity, safe API/SSE/activity metadata, frontend Drive copy, docs/OpenAPI updates, and validation passing.
- 2026-05-20 — Addressed code review findings: refresh-safe Drive client factory, safe filename sanitization, retry idempotent Drive file reuse, and OpenAPI GCS cleanup; validations passing.
- 2026-05-20 — Addressed re-review finding: frontend upload-job guard now prefers Drive IDs and no longer requires legacy bucket/object fields; focused web validations passing.
- 2026-05-20 — Final re-review passed: verified `isFileUploadJob` accepts Drive-only upload jobs with `drive_folder_id`/`drive_file_id`, keeps legacy bucket/object optional, and preserves uploaded-job safety by requiring `drive_file_id`; story marked done.

## Completion Note

Ultimate context engine analysis completed - comprehensive developer guide created.

## Code Review (2026-05-20)

Reviewer: AI Code Review

### Verdict

Changes requested. Implementasi sudah menambahkan jalur upload Google Drive, field `drive_folder_id`/`drive_file_id`, subfolder `original`/`graded`, API/SSE safe metadata, UI copy, dan validasi Go utama lulus. Namun ada beberapa risiko correctness yang perlu diperbaiki sebelum story bisa ditandai `done`.

### Findings

1. **HIGH — Production uploader memakai credential snapshot saat startup dan tidak mengikuti konfigurasi Drive terbaru**
   - Lokasi: `apps/agent/cmd/selfstudio-agent/main.go:110-118`, `apps/agent/internal/upload/drive_uploader.go:64-75`
   - Dampak: `GoogleDriveFileClient` dibuat sekali dari `cloudSettings` saat agent start. Jika operator mengisi/mengubah credential Google Drive lewat API Story 6.6 setelah aplikasi berjalan, upload worker tetap memakai client lama sampai restart. Ini membuat upload gagal walaupun UI/settings sudah menunjukkan credential baru/valid, melanggar ekspektasi operator dan safe local-first flow.
   - Rekomendasi: buat client factory/provider yang membaca settings saat upload/resolve subfolder atau refresh worker client setelah settings berhasil disimpan; map kondisi belum terkonfigurasi ke safe `DRIVE_UPLOAD_UNAUTHORIZED`/`FIX_DRIVE_CREDENTIALS`, bukan membutuhkan restart diam-diam.

2. **MEDIUM — Nama file Drive tidak disanitasi sebelum upload**
   - Lokasi: `apps/agent/internal/upload/worker.go:165`, `apps/agent/internal/upload/drive_uploader.go:87-97`
   - Dampak: worker memakai `filepathBase(j.LocalPath)` langsung sebagai `fileName` untuk Drive. Acceptance notes meminta safe filename helper (`cloud.SafeFileName` atau setara) agar deterministic dan aman. Test `TestDriveUploaderUploadsWithFolderIDAndSafeFileName` tidak benar-benar memverifikasi sanitasi.
   - Rekomendasi: jalankan basename melalui `cloud.SafeFileName`, persist/return nama aman yang sama dengan upload plan, dan tambahkan test nama berisi karakter/path edge-case.

3. **MEDIUM — Retry setelah partial Drive create dapat membuat duplicate file**
   - Lokasi: `apps/agent/internal/upload/drive_uploader.go:87-97`, `apps/agent/internal/upload/worker.go:221-228`
   - Dampak: uploader selalu `Files.Create` tanpa lookup existing exact `parent+name+not trashed`. Jika Drive berhasil membuat file tetapi response/persist gagal, retry akan membuat file kedua dengan nama sama. Story 6.8 meminta duplicate safety existing tidak memburuk dan notes merekomendasikan lookup deterministik sebelum create.
   - Rekomendasi: sebelum create, query file by parent+name+mimeType/not trashed dan reuse deterministic existing file ID atau dokumentasikan overwrite/reuse policy; tambahkan fake-client test untuk retry/idempotency.

4. **LOW — OpenAPI masih menyisakan schema/copy GCS lama di cloud settings area**
   - Lokasi: `docs/api/openapi.yaml:1058-1138`, `docs/api/openapi.yaml:1344-1400`
   - Dampak: walaupun `FileUploadJob` sudah Drive-oriented, dokumen API masih punya schema lama `provider: gcs`, `bucket_name`, `object_key` pada cloud settings/preview. Ini berisiko mengembalikan model mental GCS setelah corrective change.
   - Rekomendasi: bersihkan/replace schema lama menjadi Google Drive fields atau tandai legacy compatibility internal dengan jelas; pastikan public docs tidak menjadikan bucket/object sebagai kontrak utama.

### Validation Run By Reviewer

- PASS — `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`

### Notes

- API/SSE handler yang direview tidak mengirim full `local_path` dan activity message memakai copy Google Drive yang aman.
- Subfolder `original`/`graded` dibuat/resolve idempotent via `FindFolder` lalu `CreateFolder`, dan folder lookup existing sudah pagination/deterministic mengikuti Story 6.7 client.
- Sprint status tetap `review` sampai findings di atas diperbaiki dan validasi penuh (`apps/web` typecheck/build serta root typecheck) dicatat ulang.

## Final Re-Review (2026-05-20)

Reviewer: AI Code Review

### Verdict

Approved. Story 6.8 clean untuk scope re-review guard frontend dan siap ditandai `done`.

### Checks

- `apps/web/src/lib/api/client.ts:isFileUploadJob` tidak lagi mewajibkan legacy `bucket_name`/`object_key`; keduanya divalidasi sebagai optional string compatibility fields.
- Guard menerima job Drive-only selama memiliki `drive_folder_id` atau `drive_file_id`, sehingga respons API Drive tanpa field GCS legacy tidak ditolak.
- Safety tetap dipertahankan: job berstatus `uploaded` wajib memiliki `drive_file_id`; `not_eligible` dan `failed` tetap boleh tanpa remote identity agar status aman/terminal tidak dibuang oleh runtime guard.
- Kontrak TypeScript `FileUploadJob` sudah memuat `drive_folder_id?: string` dan `drive_file_id?: string` dengan legacy `bucket_name`/`object_key` optional.

### Validation Run By Reviewer

- PASS — `cd apps/web && npm run typecheck`
- PASS — `cd apps/web && npm run build`
- PASS — `npm run typecheck`

### Status

No open findings. Story marked `done`; sprint status updated to `done`.
