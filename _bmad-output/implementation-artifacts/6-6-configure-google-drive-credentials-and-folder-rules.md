# Story 6.6: Configure Google Drive Credentials and Folder Rules

Status: done

## Story

Sebagai admin, saya ingin mengonfigurasi Google Drive sebagai target delivery dan aturan folder upload, sehingga asset session yang sudah selesai secara lokal dapat di-upload ke folder Drive yang mudah dibuka operator/customer tanpa mengekspos credential ke browser.

## Acceptance Criteria

1. Given admin membuka Drive settings, when UI memuat konfigurasi, then UI menampilkan provider `google_drive`, root folder status, folder naming template, credential configured status, dan connection status terpisah dari local save status.
2. Given admin menyimpan credential Drive, when service menerima credential file path atau OAuth/service-account config server-side, then Go service menyimpannya server-side only dan browser/API response/SSE/activity tidak pernah menerima private key, token, refresh token, atau credential file path mentah.
3. Given browser melakukan GET settings, when response dikirim, then response hanya berisi safe metadata: `provider`, `drive_root_folder_id`, `drive_root_folder_name` jika aman, `folder_naming_template`, `credentials_configured`, `connection_status`, `last_checked_at`, `last_error_code`, dan `last_error_action`.
4. Given admin menjalankan connection check, when credential valid dan root folder writable, then service memverifikasi Drive API authorization, akses root folder, kemampuan create folder/file probe aman, menyimpan `authorized`, mengirim SSE aman, dan mencatat activity log aman.
5. Given connection check gagal karena credential invalid, folder tidak ada/unauthorized, Drive API disabled, network error, atau folder rule invalid, then API memakai `{error:{code,message,action,details}}` dengan action seperti `FIX_DRIVE_CREDENTIALS`, `FIX_DRIVE_FOLDER`, `RETRY_DRIVE_CHECK`, atau `FIX_DRIVE_FOLDER_RULES`.
6. Given admin menyimpan folder rules, when preview folder path dibuat, then service menghasilkan folder path deterministik dan aman untuk future upload dengan pola default `{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}`.
7. Given customer/order/folder name mengandung karakter tidak aman, traversal, slash/backslash, empty segment, control characters, atau terlalu panjang, when preview dibuat, then service melakukan sanitasi deterministik atau reject dengan error aman.
8. Given existing local capture/session/processing flows berjalan, then story ini tidak meregresi local capture, routing, original save, LUT processing, queue, quarantine, session summary, atau startup recovery.
9. Tests/build pass untuk Go agent dan web typecheck/build; API/OpenAPI/docs diperbarui dari GCS ke Google Drive.

## Tasks / Subtasks

- [x] Ubah model cloud settings dari GCS ke Google Drive (`provider: google_drive`, root folder, folder naming template).
- [x] Tambahkan validasi credential/root folder/folder rule dengan response metadata aman tanpa secret leak.
- [x] Tambahkan Drive folder preview endpoint dan sanitasi folder path deterministik.
- [x] Update Google Drive checker untuk memverifikasi Drive API + root folder access server-side.
- [x] Update dashboard cloud settings panel dari bucket/GCS ke Google Drive/folder.
- [x] Update readiness/API docs/planning copy terkait GCS ke Google Drive.
- [x] Jalankan validasi Go/web/root typecheck/build.

### Review Findings

- [x] [Review][Patch] Connection check belum memverifikasi kemampuan write/probe di root folder Drive [apps/agent/internal/cloud/checker.go:70] — AC4 mensyaratkan service memverifikasi authorization, akses root folder, dan kemampuan create folder/file probe aman. Implementasi `GoogleDriveChecker.Check` hanya melakukan `Files.Get(...).Fields("id,name,mimeType,trashed")` lalu menganggap `authorized`; ini dapat meloloskan folder yang dapat dibaca tetapi tidak writable, sehingga upload/story berikutnya gagal terlambat.
- [x] [Review][Patch] Public settings masih mengekspos field di luar kontrak Drive-safe metadata [apps/agent/internal/cloud/config.go:38] — AC3 membatasi response GET settings ke `provider`, `drive_root_folder_id`, `drive_root_folder_name` jika aman, `folder_naming_template`, `credentials_configured`, `connection_status`, `last_checked_at`, `last_error_code`, dan `last_error_action`. `PublicSettings` masih memiliki/menulis `bucket_name`, `target_root_prefix`, `object_naming_template`, dan `last_error`; ini memperlebar kontrak API setelah migrasi GCS dan berisiko mengembalikan pesan error yang tidak termasuk allowlist.
- [x] [Review][Patch] Folder rule invalid pada connection check tidak memakai action Drive-specific [apps/agent/internal/cloud/object_keys.go:135] — AC5 meminta action seperti `FIX_DRIVE_FOLDER_RULES` untuk folder rule invalid. `BuildDriveFolderPreview` dan `SafeDriveSegment` menghasilkan `DRIVE_FOLDER_RULE_INVALID`, tetapi memakai `ActionFixRules` yang saat ini bernilai `FIX_DRIVE_FOLDER_RULES`; pastikan semua jalur validasi Drive tetap memakai action ini dan tambahkan test kontrak untuk error preview/check agar tidak regresi ke action GCS/generic.

## Implementation Notes

- Replace user-facing GCS settings with Google Drive settings; keep generic `/api/cloud/settings` endpoints if useful, but provider must be `google_drive`.
- Recommended MVP credential mode: service account JSON file shared to a Drive folder, or OAuth installed-app flow if service account sharing is unacceptable.
- Browser must never handle OAuth tokens/private keys.
- Update docs that previously referenced `bucket_name`, `target_root_prefix`, and GCS object keys.

## Dev Agent Record

### Debug Log

- RED: added failing Drive contract tests in `apps/agent/internal/cloud/drive_test.go` and `apps/agent/internal/api/cloud_settings_drive_test.go`; initial run failed because Drive provider/settings/preview fields were missing.
- GREEN: implemented `ProviderGoogleDrive`, Drive public DTO, Drive folder preview, Drive settings PUT/GET/check flow, and Drive-safe SSE metadata.
- Refactor/compatibility: updated upload target resolver to use Drive folder path identity while preserving local-first queue semantics for later stories.
- Fixed flaky Windows concurrent upload state persistence by serializing upload jobs state saves with `jobsStateSaveMu`.
- RED: added review regression tests for Drive write/probe checking, public settings allowlist, and Drive-specific folder-rule error actions; initial run failed because the probe helper did not exist and public DTO still had legacy fields.
- GREEN: refactored `GoogleDriveChecker` through an injectable Drive probe client that validates root folder metadata, creates a safe hidden probe folder under the root, and deletes it before authorizing.
- GREEN: tightened `PublicSettings` and web `CloudSettings` contract to the Drive-safe allowlist only; UI now shows `last_error_code`/`last_error_action` instead of legacy `last_error`.
- REFACTOR: kept validation errors mapped to `FIX_DRIVE_FOLDER_RULES` for Drive preview and settings validation paths and covered them with API contract tests.

### Completion Notes

- Google Drive is now the default cloud provider in safe public settings.
- Cloud settings API stores Drive root folder ID/name and write-only credential material server-side only.
- `GET /api/cloud/settings` returns safe Drive metadata only; no service account JSON, token, private key, or credential file path is exposed.
- `POST /api/cloud/settings/check` uses Google Drive API scopes to verify credential and root folder access, mapping failures to Drive-specific actions.
- `POST /api/cloud/settings/folder-preview` returns deterministic sanitized Drive folder paths using `{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}`.
- Dashboard copy/fields now show Google Drive root folder and Drive folder naming instead of GCS bucket/object naming.
- Readiness item changed from `gcs` to `google_drive`.
- API doc `docs/api/cloud-storage-story-6-1.md` now documents Google Drive metadata/actions.
- ✅ Resolved review finding [Patch]: Google Drive connection check now verifies root folder write/probe capability, not only metadata read access.
- ✅ Resolved review finding [Patch]: Public cloud settings now obey the Drive-safe metadata allowlist and no longer include legacy GCS fields or `last_error`.
- ✅ Resolved review finding [Patch]: Added contract tests ensuring Drive folder-rule invalid preview/settings paths return `FIX_DRIVE_FOLDER_RULES`.

### Validation

- `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...` — passed.
- `cd apps/web && npm run typecheck` — passed.
- `cd apps/web && npm run build` — passed.
- `npm run typecheck` — passed.
- `cd apps/agent && GOTMPDIR=../../.gotmp go test ./internal/cloud ./internal/api` — failed during RED phase as expected before implementation (`CheckGoogleDriveWithClient`/probe abstraction missing and public DTO legacy field references still present).
- `cd apps/agent && GOTMPDIR=../../.gotmp go test ./internal/cloud ./internal/api` — passed after implementation.
- `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...` — passed.
- `cd apps/web && npm run typecheck` — passed.
- `cd apps/web && npm run build` — passed.
- `npm run typecheck` — passed.

## File List

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/cloud_settings.go`
- `apps/agent/internal/api/cloud_settings_contract_test.go`
- `apps/agent/internal/api/cloud_settings_drive_test.go`
- `apps/agent/internal/api/cloud_settings_failure_test.go`
- `apps/agent/internal/api/cloud_settings_test.go`
- `apps/agent/internal/api/cloud_targets.go`
- `apps/agent/internal/api/cloud_targets_test.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/session_uploads.go`
- `apps/agent/internal/cloud/checker.go`
- `apps/agent/internal/cloud/cloud_test.go`
- `apps/agent/internal/cloud/config.go`
- `apps/agent/internal/cloud/drive_checker_test.go`
- `apps/agent/internal/cloud/drive_test.go`
- `apps/agent/internal/cloud/object_keys.go`
- `apps/agent/internal/cloud/persistence.go`
- `apps/agent/internal/readiness/checklist.go`
- `apps/agent/internal/readiness/checklist_test.go`
- `apps/agent/internal/upload/jobs_persistence.go`
- `apps/agent/internal/upload/persistence.go`
- `apps/agent/internal/upload/prefix.go`
- `apps/agent/internal/upload/resolver.go`
- `apps/agent/internal/upload/resolver_test.go`
- `apps/agent/internal/upload/target.go`
- `apps/agent/internal/upload/uploader.go`
- `apps/web/src/features/cloud/cloud-settings.tsx`
- `apps/web/src/lib/api/client.ts`
- `docs/api/cloud-storage-story-6-1.md`
- `_bmad-output/planning-artifacts/architecture.md`
- `_bmad-output/planning-artifacts/epics.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`

## Change Log

- 2026-05-20 — Replaced Story 6.6 cloud settings surface from GCS to Google Drive and moved story to review.
- 2026-05-20 — Addressed code review findings: Drive write/probe check, public settings allowlist, and Drive-specific folder-rule action regression tests; story remains ready for review.
