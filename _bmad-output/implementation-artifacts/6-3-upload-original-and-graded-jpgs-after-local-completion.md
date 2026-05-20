# Story 6.3: Upload Original and Graded JPGs After Local Completion

Status: done

## Story

Sebagai operator, saya ingin original dan graded JPG di-upload ke cloud hanya setelah local completion, sehingga fulfillment cloud berjalan aman tanpa mengganggu capture, routing session, local save, atau LUT processing saat event.

## Acceptance Criteria

1. Given session sudah terkunci dan semua photo session sudah mencapai local completion yang aman, when upload worker berjalan, then setiap JPG original yang tersimpan lokal di-upload ke target GCS yang sudah dikonfigurasi.
2. Given graded JPG berhasil dibuat untuk photo session, when upload worker berjalan, then graded JPG tersebut di-upload ke target GCS yang sama menggunakan object key `.../graded/{safe_file_name}`.
3. Given graded processing gagal tetapi original tersimpan, when upload worker berjalan, then original tetap eligible untuk upload dan graded dicatat sebagai tidak siap/failed/not eligible tanpa menghapus atau mengubah file lokal.
4. Given upload sedang berjalan atau gagal, when operator memulai session baru, capture folder watcher menerima JPG baru, local original save berjalan, atau LUT processing berjalan, then cloud upload tidak memblokir flow tersebut.
5. Given upload gagal karena config/cloud/network/remote error, when error terjadi, then local file tetap dipertahankan, per-file status menjadi failed dengan error/action aman, dan session cloud status menunjukkan failed/pending tanpa menandai local delivery failed.
6. Given upload berhasil untuk file original atau graded, when status disimpan, then sistem menyimpan per-file upload identity/status termasuk `photo_id`, `asset_kind`, `local_path`, `bucket_name`, `object_key`, `remote_identity`, status, attempt count, dan timestamp.
7. Given session detail/dashboard ditampilkan, when upload file pending/uploading/uploaded/failed/not eligible, then UI/API menampilkan per-session dan per-file upload status terpisah dari local save/processing status dengan label teks dan action yang jelas.
8. Given API/SSE/activity log menerima update upload, when event/log dipublikasikan, then payload tidak berisi credential, token, service account JSON, raw private key, atau raw error cloud yang berisiko sensitif.
9. Tests/build pass untuk Go agent, web typecheck/build yang relevan, dan OpenAPI/API contract diperbarui.

## Tasks / Subtasks

- [x] Definisikan model per-file cloud upload job untuk Story 6.3. (AC: 1, 2, 3, 5, 6)
  - [x] Extend package `apps/agent/internal/upload`; jangan membuat package cloud baru dan jangan mencampur lifecycle job dengan `internal/cloud` yang khusus config/credential/object-key helper.
  - [x] Tambahkan model, misalnya `FileUploadJob`, dengan field minimal: `job_id`, `session_id`, `station_id`, `photo_id`, `asset_kind` (`original`/`graded`), `local_path`, `bucket_name`, `object_key`, `remote_identity`, `status`, `attempt_count`, `last_error_code`, `last_error_action`, `created_at`, `updated_at`, `uploaded_at`.
  - [x] Status minimal Story 6.3: `pending`, `uploading`, `uploaded`, `failed`, `not_eligible`. Jangan implement automatic retry/backoff/manual retry penuh di story ini; itu Story 6.4.
  - [x] Job identity harus deterministic: satu `(photo_id, asset_kind)` hanya punya satu job aktif untuk session target tertentu.
  - [x] Jangan menambahkan credential/token/path credential ke job, API response, SSE, activity, atau frontend state.
- [x] Implement persistence atomic untuk upload jobs. (AC: 5, 6)
  - [x] Simpan runtime state di `SELFSTUDIO_LOCAL_DATA_DIR/state/upload_jobs.json` atau nama setara di bawah `local-data/state`; jangan simpan di `apps/web/public`, source tree, localStorage, cookies, atau OpenAPI examples dengan data nyata.
  - [x] Ikuti pattern `upload.Persistence`, `sessions.Persistence`, `photos.Persistence`, `cloud.Persistence`: versioned JSON, temp file + sync + rename, validasi saat load.
  - [x] Persistence failure tidak boleh dipalsukan sebagai success; jangan publish success SSE/activity jika status job gagal disimpan.
  - [x] Load corrupt/invalid state harus fail aman dan tidak membuat worker meng-upload file tanpa state.
- [x] Bangun discovery job dari session local output yang benar-benar complete. (AC: 1, 2, 3, 6)
  - [x] Gunakan `sessions.Store`, `photos.Store`, dan `upload.Store` session target Story 6.2; jangan scan folder output bebas tanpa metadata photo.
  - [x] Session minimal harus `locked`; jika belum ada status `local_complete`, hitung local completion secara eksplisit dari photo metadata: original upload eligible hanya jika `OriginalSaveStatus == saved_original` dan `LocalOriginalPath` ada/verified; graded upload eligible hanya jika `GradedProcessingStatus == processed` dan `LocalGradedPath` ada/verified.
  - [x] Jika masih ada photo pending/saving/processing, return/persist status session upload `pending_local_completion` atau setara; jangan upload sebagian sambil mengklaim session complete kecuali status per-file jelas.
  - [x] Jika graded failed/missing tapi original saved, buat original job dan tandai graded `not_eligible` atau failed dengan action aman; jangan memblokir original upload.
  - [x] Verifikasi file lokal ada dan readable sebelum membuat/upload job; final persisted record tidak boleh menunjuk missing local final file sebagai uploaded.
- [x] Generate GCS object keys dengan helper Story 6.1/6.2. (AC: 1, 2, 6)
  - [x] Reuse `cloud.SafeFileName`, `cloud.BuildObjectKey` semantics, `upload.BuildSessionObjectPrefix`, dan session target `ObjectPrefix`; jangan membuat sanitizer baru yang berbeda.
  - [x] Object key final wajib memakai prefix Story 6.2 dan append `/{asset_kind}/{safe_file_name}`.
  - [x] `asset_kind` hanya `original` atau `graded`; invalid value reject, jangan fallback diam-diam untuk persisted job.
  - [x] File name berasal dari `filepath.Base(LocalOriginalPath/LocalGradedPath)` lalu disanitasi; jangan expose full local path di object key.
  - [x] Object key harus deterministic across retry/restart untuk photo/job yang sama.
- [x] Implement uploader worker/service non-blocking. (AC: 1, 2, 4, 5, 6)
  - [x] Buat interface GCS uploader agar tests memakai fake uploader; real implementation boleh memakai existing dependency `cloud.google.com/go/storage` di Go agent saja.
  - [x] Worker harus asynchronous/background; endpoint trigger tidak boleh menjalankan upload besar secara blocking di request path.
  - [x] Batasi concurrency upload agar tidak mengganggu capture/local processing; default konservatif 1-2 worker sudah cukup untuk MVP.
  - [x] Gunakan context timeout per upload dan safe error mapping; jangan retry loop tak terbatas di Story 6.3.
  - [x] Upload failure hanya mengubah cloud upload job/session status, tidak menghapus/memindah/mengubah local original/graded files.
  - [x] Jangan mengubah processing queue/guard untuk cloud upload; cloud upload punya guard/runner sendiri agar local processing tetap independen.
- [x] Tambahkan API Go untuk trigger dan status upload. (AC: 4, 5, 6, 7, 8)
  - [x] Endpoint disarankan: `POST /api/sessions/{session_id}/uploads/start` untuk enqueue/start upload session assets; `GET /api/sessions/{session_id}/uploads` untuk list per-file upload status.
  - [x] Semua endpoint wajib `RequireAuth`; mutation wajib `RequireTrustedOrigin`.
  - [x] Response success wajib `{data}`; error wajib `{error:{code,message,action,details}}` dari helper `response.go`.
  - [x] Error/action minimal: `UPLOAD_PENDING_LOCAL_COMPLETION`/`WAIT_FOR_LOCAL_COMPLETION`, `CLOUD_TARGET_NOT_READY`/`RESOLVE_CLOUD_TARGET`, `UPLOAD_LOCAL_FILE_MISSING`/`CHECK_LOCAL_OUTPUT`, `UPLOAD_FAILED`/`RETRY_CLOUD_UPLOAD`, `UPLOAD_STATE_SAVE_FAILED`/`RETRY_CLOUD_UPLOAD`.
  - [x] Handler harus menggunakan session target Story 6.2; jika target belum ready, jangan buat object key/upload palsu.
- [x] Integrasikan status di session summary/dashboard tanpa mencampur local status. (AC: 5, 7)
  - [x] Update `SessionSummary.UploadStatus` agar merefleksikan aggregate job: `not_configured`, `target_pending`, `pending`, `uploading`, `uploaded`, `partial_failed`, `failed`, atau `pending_local_completion`; tetap terpisah dari `LocalOutputFolder`, processing queue, dan photo processing failures.
  - [x] Tambahkan detail per-file di API/UI: original/graded status per photo, attempt count, safe error action, uploaded timestamp, object key/bucket jika aman.
  - [x] Update `apps/web/src/features/sessions/live-station-cards.tsx` atau session detail component dengan label teks dan tombol `Start Cloud Upload` hanya untuk locked/local-complete-enough session dengan ready cloud target.
  - [x] Jangan mengembalikan locked session visibility bug Story 6.2; locked/completed session harus tetap reachable untuk cloud status/action.
  - [x] Dashboard SSE invalidation harus refresh session detail/upload status/activity; jangan invalidate processing queue kecuali local processing berubah.
- [x] Publish SSE dan activity log aman. (AC: 5, 7, 8)
  - [x] Event dot-notation disarankan: `upload.started`, `upload.file_uploaded`, `upload.file_failed`, `upload.session_updated`.
  - [x] SSE payload hanya berisi safe metadata: `session_id`, `station_id`, `photo_id`, `asset_kind`, `status`, `bucket_name`, `object_key`, `last_error_code`, `last_error_action`.
  - [x] Activity log actions: `cloud.upload_started`, `cloud.upload_file_uploaded`, `cloud.upload_file_failed`, `cloud.upload_session_failed` dengan refs station/session bila tersedia.
  - [x] Jangan log raw GCS response/error jika mengandung request metadata, credential path, token, private key, service account JSON, atau full local customer filesystem paths.
- [x] Update OpenAPI dan docs. (AC: 6, 7, 8, 9)
  - [x] Tambahkan schemas `SessionUploadStatus`, `FileUploadJob`, `SessionUploadsResponse`, trigger response, status enum, error codes/actions, dan SSE event schemas.
  - [x] Dokumentasikan object key final: `{target_root_prefix}/{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}/{asset_kind}/{safe_file_name}`.
  - [x] Tegaskan Story 6.3 menambahkan upload file original/graded dan per-file status, tetapi Story 6.4 masih bertanggung jawab untuk retry/de-duplicate lanjutan dan Story 6.5 untuk recovery pending upload jobs after restart.
- [x] Tambahkan tests dan validasi. (AC: 1-9)
  - [x] Go unit tests untuk job identity deterministic, one `(photo_id, asset_kind)` job, object key generation original/graded, and safe filename behavior.
  - [x] Persistence tests untuk default load, atomic save, corrupt state fail, duplicate job validation, save failure no success event/activity.
  - [x] Worker tests dengan fake uploader untuk success original+graded, original-only when graded failed, missing local file, GCS failure, non-blocking trigger, and no local file deletion.
  - [x] API tests untuk auth/trusted-origin, session not found, not locked/local completion pending, target not ready, start success, get status, safe errors.
  - [x] Web type/schema tests jika API client/UI disentuh.
  - [x] Run `cd apps/agent && go test ./...`.
  - [x] Run `cd apps/web && npm run typecheck`.
  - [x] Run `cd apps/web && npm run build` jika environment/dependencies memungkinkan.

## Dev Notes

### Source Requirements

- Epic 6 objective: post-session cloud fulfillment dengan GCS sesuai architecture, upload original+graded setelah local completion, per-file/per-session upload tracking, retry failure, duplicate prevention, dan upload tidak memblokir session baru.
- Story 6.3 dari epics: saat session local output complete dan upload worker berjalan, original dan graded JPG di-upload ke configured cloud target, upload tidak memblokir capture/local save/session routing/LUT processing, local files preserved saat upload fails, dan per-file upload status tracked.
- FR58-FR59-FR61-FR62: upload original/graded ke cloud setelah local completion, track status per session/per file, preserve local files on upload failure, and ensure cloud upload does not block new capture sessions.
- FR19/FR43/FR46/FR47/FR48: session summary/dashboard harus menampilkan upload status terpisah, actionable alerts, queue/status, dan activity logs.
- NFR4/NFR11/NFR16/NFR21/NFR25/NFR26/NFR32: upload tidak boleh degrade local workflow, local save independent dari cloud success, upload duplicate-safe, customer data/photo data protected, token/network/partial upload handled safely, remote identity/status tracked, local-vs-cloud status clear.

### Current Code Context To Read Before Editing

- `apps/agent/internal/upload/target.go`
  - Current state: session-level cloud target from Story 6.2 with statuses `pending/resolving/ready/failed` and safe error/action constants.
  - What this story changes: add separate per-file upload job/status model; do not overload `SessionCloudTarget` as a file job.
  - Must preserve: `SessionCloudTarget` remains prefix-only identity and contains no credential data.
- `apps/agent/internal/upload/persistence.go`
  - Current state: atomic JSON persistence for `local-data/state/upload_targets.json` and in-memory `Store` keyed by `session_id`.
  - What this story changes: create a second persistence/store for upload jobs or carefully extend with separate state section; avoid breaking existing target load/save compatibility.
  - Must preserve: one target per session and ready target validation.
- `apps/agent/internal/upload/prefix.go`
  - Current state: deterministic session-level prefix from cloud settings + session; validates prefix safety.
  - What this story changes: build final file object key by appending `original/{safe_file_name}` or `graded/{safe_file_name}` to target prefix.
  - Must preserve: same session/config produces same prefix across retry/restart.
- `apps/agent/internal/upload/resolver.go`
  - Current state: resolves prefix-only identity; does not upload marker or JPG; maps cloud settings/config errors safely.
  - What this story changes: uploader should depend on ready target produced here; do not add JPG upload into resolver.
- `apps/agent/internal/cloud/config.go` and `object_keys.go`
  - Current state: GCS settings, credentials configured check, `ValidateSettings`, `SafeSegment`, `SafeFileName`, and final object key preview template.
  - What this story changes: reuse sanitizer and settings; real upload may instantiate GCS client server-side only.
  - Must preserve: browser never receives credentials; no Google client library in frontend.
- `apps/agent/internal/photos/store.go`
  - Current state: photo records include `LocalOriginalPath`, `OriginalSaveStatus`, `LocalGradedPath`, `GradedProcessingStatus`, attempt counts, and timestamps; `ListBySession` provides session photo records.
  - What this story changes: read these records to create upload jobs; avoid adding cloud fields to photo model unless absolutely necessary because upload state belongs in `internal/upload`.
  - Must preserve: original-first save/graded processing status semantics and no dangling local file references after successful local save.
- `apps/agent/internal/api/cloud_targets.go`
  - Current state: authenticated/trusted-origin session cloud target GET/resolve; locked-session guard; safe SSE/activity.
  - What this story changes: mirror auth/error/SSE/activity pattern for upload start/status endpoints.
  - Must preserve: no success event/activity when persistence/save fails.
- `apps/agent/internal/api/sessions.go`
  - Current state: `SessionSummary.UploadStatus` currently maps session cloud target state, not per-file upload aggregate. Session statuses are still mainly `active` and `locked`.
  - What this story changes: aggregate target + file jobs into truthful upload status. If local_complete state still not modeled, compute readiness from photo local/graded metadata and expose pending status honestly.
  - Must preserve: session start/end behavior, quarantine counts, local output folder, photo counts, processing failure counts.
- `apps/web/src/features/sessions/live-station-cards.tsx`
  - Current state: Story 6.2 review fixed locked-session visibility for cloud target actions/status.
  - What this story changes: add upload start/status affordance without hiding locked/completed sessions again.
- `apps/web/src/lib/api/client.ts`
  - Current state: typed API client and validators for session/cloud target/photo state.
  - What this story changes: add types and functions for session uploads start/status.
- `apps/web/src/features/health/health-dashboard.tsx`
  - Current state: listens for cloud target events and invalidates relevant session/activity data.
  - What this story changes: add upload event invalidation for session/upload status only; avoid unnecessary processing queue invalidation.
- `docs/api/openapi.yaml`
  - Current state: documents cloud settings and session cloud target endpoints; notes Story 6.3 appends `/{asset_kind}/{safe_file_name}` and uploads JPGs.
  - What this story changes: document per-file upload jobs, endpoints, events, and aggregate statuses.

### Architecture Guardrails

- Go service owns all cloud upload, filesystem checks, GCS credentials, upload job persistence, SSE publication, and activity logging.
- Next.js/browser must never receive service account JSON, private key, OAuth token, ADC token, Supabase service role, credential file contents, credential file path, or raw privileged cloud errors.
- `internal/cloud` remains config/credential/object-key helper; `internal/upload` owns session target and upload lifecycle.
- API JSON/status fields use `snake_case`; REST success uses `{data}`; REST errors use `{error:{code,message,action,details}}`; SSE event names use dot notation.
- GCS object namespace is flat; folder means prefix. Story 6.2 uses prefix-only identity; Story 6.3 uploads actual JPG objects under that prefix.
- Cloud upload failure must not change local save/processing status, must not delete local files, and must not block session start, folder watcher, routing, original save, or LUT processing.
- Do not introduce Google Drive API. Architecture-selected provider is Google Cloud Storage.
- Do not implement full automatic retry/dedup/restart recovery scope beyond safe idempotent job identity required for this story; Story 6.4/6.5 cover advanced retry/recovery.

### Object Key Convention

Use the Story 6.2 session target prefix and append asset kind + sanitized file name:

```text
{target_root_prefix}/{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}/{asset_kind}/{safe_file_name}
```

Rules:

- `object_prefix` comes from ready `SessionCloudTarget` and must not be regenerated with `time.Now()`.
- `asset_kind` is exactly `original` or `graded`.
- `safe_file_name` comes from `cloud.SafeFileName(filepath.Base(local_path))`.
- Same photo/job across retry/restart must produce same object key.
- Reject unsafe keys; do not silently truncate identity-bearing segments.

### Previous Story Intelligence

- Story 6.2 created `internal/upload` with prefix-only session target, atomic `upload_targets.json`, APIs `GET/POST /api/sessions/{session_id}/cloud-target`, safe SSE `cloud.target_resolved`/`cloud.target_failed`, dashboard cloud target action, and OpenAPI docs.
- Story 6.2 explicitly did not upload JPGs and selected prefix-only GCS identity, not marker object. Story 6.3 must add file upload without moving that responsibility back into cloud target resolver.
- Story 6.2 review fixed a HIGH UI bug: locked sessions disappeared from dashboard, making cloud target action unreachable. Do not reintroduce active-only session filtering.
- Story 6.1 implemented safe GCS settings, credential server-side only, object key sanitizer, authenticated/trusted-origin endpoints, fake checker tests, and persistence safety. Apply the same “no success if persistence fails” rule here.
- Story 5.5/5.4/5.3 established processing recovery/retry/queue patterns. Use their operator-actionable status style, but keep cloud upload worker/guard separate from processing worker/guard.
- Current git history is sparse (`Add from-scratch setup guide`, `Initial Selfstudio camera capture spike`); current workspace files and implementation artifacts are authoritative.

### Latest Technical Notes

- Architecture observed Google Cloud Storage Go client `cloud.google.com/go/storage`; repo currently has it indirectly from Story 6.1. Do not upgrade dependencies unless needed; if upgrading, run full Go tests and document reason.
- GCS object names are flat strings; `/` simulates hierarchy. Project intentionally uses strict object key sanitation despite GCS allowing many characters.
- For overwrite/duplicate behavior in this story, safest MVP is deterministic object key + persisted job status. If using GCS generation preconditions, keep behavior testable behind interface. Advanced duplicate reconciliation/retry is Story 6.4/6.5.
- No new frontend cloud/GCS dependency is needed.

### Testing Requirements

Run at minimum:

- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build` if dependencies/environment allow.

Required coverage:

- Deterministic object key generation for original and graded assets from ready session target.
- Upload job identity prevents duplicate `(photo_id, asset_kind)` job records.
- Original-only upload when graded processing failed; graded upload when processed.
- Pending local completion prevents premature upload while local save/graded processing is still pending/processing.
- Missing/unreadable local file produces safe failed/not eligible status and does not mark uploaded.
- Fake GCS upload success updates job status to uploaded and persists remote identity.
- Fake GCS upload failure preserves local file and persists safe failed status/action.
- API endpoints require auth; start mutation requires trusted origin.
- Persistence save failure prevents success API/SSE/activity.
- SSE/activity/API payloads exclude credentials, raw tokens, raw GCS errors, and credential paths.
- Existing session/processing/quarantine/cloud target tests still pass.

### Regression Risks To Avoid

- Do not upload before a ready Story 6.2 cloud target exists.
- Do not upload before local original exists and is readable.
- Do not claim session uploaded if some eligible file jobs are pending/uploading/failed.
- Do not delete, move, rewrite, or reprocess local original/graded files as part of cloud upload.
- Do not block new capture sessions, ingestion, routing, original save, or LUT processing while upload runs.
- Do not expose credentials or full sensitive local paths in browser/SSE/activity logs.
- Do not implement Google Drive API or put upload logic in Next.js.
- Do not reintroduce the locked-session dashboard visibility bug from Story 6.2.
- Do not build retry loops that conflict with Story 6.4; one trigger + safe status is enough for Story 6.3.
- Do not store upload job state in `upload_targets.json` in a way that breaks existing Story 6.2 target state compatibility.

## Project Structure Notes

Expected new files:

- `apps/agent/internal/upload/jobs.go`
- `apps/agent/internal/upload/jobs_persistence.go`
- `apps/agent/internal/upload/object_keys.go` or equivalent file-key helper
- `apps/agent/internal/upload/worker.go`
- `apps/agent/internal/upload/uploader.go`
- `apps/agent/internal/upload/*_test.go`
- `apps/agent/internal/api/session_uploads.go`
- `apps/agent/internal/api/session_uploads_test.go`
- Optional frontend component/hook: `apps/web/src/features/sessions/use-start-cloud-upload-mutation.ts`, `apps/web/src/features/sessions/session-upload-status.tsx`

Expected modified files:

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/health.go` or mux wiring file if split later
- `apps/agent/internal/api/sessions.go`
- `apps/web/src/lib/api/client.ts`
- `apps/web/src/features/sessions/live-station-cards.tsx` and/or session detail component
- `apps/web/src/features/health/health-dashboard.tsx`
- `docs/api/openapi.yaml`

Runtime upload job state remains under `local-data/state` and must not be committed.

## References

- `_bmad-output/planning-artifacts/epics.md` — Epic 6 and Story 6.3 acceptance criteria; FR58-FR63 mapping.
- `_bmad-output/planning-artifacts/prd.md` — Google Drive/Cloud Fulfillment requirements, local-vs-cloud separation, upload failure/retry NFRs.
- `_bmad-output/planning-artifacts/architecture.md` — GCS selected over Drive API, Go credential boundary, upload package location, API/SSE/error/state patterns.
- `_bmad-output/implementation-artifacts/6-1-configure-cloud-storage-credentials-and-target-rules.md` — cloud config, credential safety, GCS object key sanitizer, API/SSE/activity patterns.
- `_bmad-output/implementation-artifacts/6-2-create-cloud-folder-object-structure-per-session.md` — session target prefix, idempotent remote identity, current upload package patterns, locked-session UI fix.
- `apps/agent/internal/upload/target.go` — current session cloud target model and statuses.
- `apps/agent/internal/upload/persistence.go` — current atomic upload target persistence/store pattern.
- `apps/agent/internal/upload/prefix.go` — deterministic session prefix generation.
- `apps/agent/internal/upload/resolver.go` — prefix-only resolver; do not place JPG upload here.
- `apps/agent/internal/cloud/object_keys.go` — `SafeFileName`, object key convention, strict sanitizer.
- `apps/agent/internal/cloud/config.go` — safe cloud settings and credential configured status.
- `apps/agent/internal/photos/store.go` — photo local original/graded paths and statuses.
- `apps/agent/internal/api/cloud_targets.go` — auth/trusted-origin/error/SSE/activity pattern for Story 6.2 cloud target APIs.
- `apps/agent/internal/api/sessions.go` — session summary upload status aggregation point.
- `apps/web/src/features/sessions/live-station-cards.tsx` — dashboard session/cloud action UI, including locked session visibility fix.
- `apps/web/src/lib/api/client.ts` — frontend API client/types.
- `apps/web/src/features/health/health-dashboard.tsx` — SSE invalidation pattern.
- `docs/api/openapi.yaml` — API/SSE contract source.

## Dev Agent Record

### Agent Model Used

GPT-5.1 Codex CLI

### Debug Log References

- 2026-05-19: Implemented `internal/upload` per-file job model, atomic `upload_jobs.json` persistence, deterministic object key builder, uploader interface, and background worker with fake-uploader tests.
- 2026-05-19: Added authenticated/trusted-origin upload status/start endpoints and wired runtime stores/worker in agent main.
- 2026-05-19: Updated web API types/client, dashboard labels/action, SSE invalidation, and OpenAPI contract.
- 2026-05-19: Validation passed: `cd apps/agent && go test ./...`, `cd apps/web && npm run typecheck`, `cd apps/web && npm run build`.
- 2026-05-19: Review follow-up fix: replaced production local-copy uploader with server-side GCS uploader using Story 6.1 cloud settings, wired worker upload events to safe SSE/activity, made final job persistence failure suppress publish, and added session upload API handler tests.
- 2026-05-19: Review follow-up validation passed: `cd apps/agent && go test ./...`, `cd apps/web && npm run typecheck`, `cd apps/web && npm run build`.

### Completion Notes List

- Implemented Story 6.3 cloud upload lifecycle as a separate `internal/upload` job system, preserving Story 6.2 cloud target as prefix-only identity.
- Upload jobs are deterministic by `(session_id, photo_id, asset_kind)`, persisted atomically under local-data state, and never include credentials/tokens/service account/private key material.
- Worker discovers eligible original/graded JPGs from session/photo metadata only after locked/local-complete-enough state, uploads asynchronously via interface/fake-testable uploader, and preserves local files on failure.
- API/UI now expose aggregate session upload status and per-file job details separately from local save/processing status, with safe error codes/actions.
- OpenAPI documents new upload endpoints, schemas, status enums, and object-key convention; advanced retry/dedup/restart recovery remains future Story 6.4/6.5 scope.
- ✅ Resolved review finding [HIGH]: production runtime now uses real server-side `upload.GCSUploader` backed by `cloud.google.com/go/storage` and authorized Story 6.1 cloud settings; `LocalCopyUploader` is fail-safe and test-only.
- ✅ Resolved review finding [HIGH]: worker file-status events now publish safe `upload.file_uploaded`, `upload.file_failed`, and `upload.session_updated` SSE/activity updates.
- ✅ Resolved review finding [HIGH]: final upload status is published only after `Jobs.Upsert` and durable persistence save succeed.
- ✅ Resolved review finding [MEDIUM]: added API handler tests for auth, trusted-origin, target/local-completion errors, state-save failure, safe responses, start success, and GET status.

### File List

- apps/agent/cmd/selfstudio-agent/main.go
- apps/agent/internal/api/health.go
- apps/agent/internal/api/session_uploads.go
- apps/agent/internal/api/session_uploads_test.go
- apps/agent/internal/api/sessions.go
- apps/agent/internal/upload/jobs.go
- apps/agent/internal/upload/jobs_persistence.go
- apps/agent/internal/upload/jobs_test.go
- apps/agent/internal/upload/gcs_uploader.go
- apps/agent/internal/upload/uploader.go
- apps/agent/internal/upload/worker.go
- apps/agent/internal/upload/worker_test.go
- apps/web/src/features/health/health-dashboard.tsx
- apps/web/src/features/sessions/live-station-cards.tsx
- apps/web/src/features/sessions/use-start-cloud-upload-mutation.ts
- apps/web/src/lib/api/client.ts
- docs/api/openapi.yaml
- _bmad-output/implementation-artifacts/6-3-upload-original-and-graded-jpgs-after-local-completion.md
- _bmad-output/implementation-artifacts/sprint-status.yaml

## Review Follow-ups

- [x] HIGH: Replace runtime `upload.LocalCopyUploader{}` wiring in `apps/agent/cmd/selfstudio-agent/main.go` with a real server-side GCS uploader backed by `cloud.google.com/go/storage` and credential settings. Current production worker returns `gs://...` success without writing to GCS when `DestinationRoot` is empty, violating AC1/AC2 and the fake-uploader boundary.
- [x] HIGH: Wire worker file-status events to safe SSE/activity publication. `Worker.uploadOne` only sends to an internal `Events` channel, but `main.go` does not provide/consume it and `SessionUploadsHandler.publish` is only called for `upload.started`; `upload.file_uploaded`, `upload.file_failed`, and `upload.session_updated` are never emitted, violating AC7/AC8.
- [x] HIGH: Do not publish/upload-success state when final job persistence fails. `Worker.uploadOne` ignores `Jobs.Upsert`/`Persistence.Save` errors before publishing final status, so an upload can succeed remotely while durable per-file status is not saved, violating the persistence guardrail and AC5/AC6.
- [x] MEDIUM: Add API handler tests for `GET /api/sessions/{session_id}/uploads` and `POST /api/sessions/{session_id}/uploads/start` covering auth, trusted-origin mutation guard, target-not-ready/local-completion errors, state-save failure, safe responses, and start success. No `session_uploads_test.go` exists despite story requirements.

## Change Log

- 2026-05-19: Ultimate context engine analysis completed - comprehensive developer guide created.
- 2026-05-19: Implemented Story 6.3 upload original/graded JPG jobs, persistence, worker/API/UI/OpenAPI updates, and validations; status moved to review.
- 2026-05-19: Code review found blocking upload/runtime/event/persistence issues; status moved back to in-progress.
- 2026-05-19: Addressed code review findings - 4 items resolved; real GCS uploader, upload SSE/activity, persistence guard, and API tests added; status moved to review.
