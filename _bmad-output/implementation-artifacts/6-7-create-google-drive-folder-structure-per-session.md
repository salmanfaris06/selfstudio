# Story 6.7: Create Google Drive Folder Structure Per Session

Status: done

## Story

Sebagai operator, saya ingin sistem membuat atau me-resolve folder Google Drive per customer/order/session, sehingga hasil event mudah ditemukan dan upload dapat retry/restart tanpa membuat folder duplikat.

## Acceptance Criteria

1. Given session sudah terkunci/local complete enough, when Drive fulfillment dimulai, then sistem membuat atau resolve nested folder Drive berdasarkan template `{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}` di bawah configured root folder.
2. Given folder dengan nama yang sama sudah ada di parent yang sama, when resolver berjalan ulang, then sistem reuse folder ID yang aman/idempotent, bukan membuat duplikat.
3. Given folder berhasil dibuat/resolved, then sistem menyimpan Drive folder IDs per level atau minimal final `drive_session_folder_id` untuk session.
4. Given retry/restart menjalankan resolver lagi, then operasi idempotent dan menghasilkan remote identity yang sama untuk session.
5. Given Drive belum configured/authorized atau root folder tidak writable, then sistem menyimpan status failure aman dan local output/session/processing tidak berubah.
6. Given folder name input tidak aman, then sanitized names mencegah slash/backslash, traversal, control chars, empty segment, dan panjang berlebihan.
7. Given dashboard/session detail ditampilkan, then Drive target status terlihat terpisah dari local save/processing dengan label teks dan action `Resolve Drive Folder`/`Retry Drive Folder`.
8. Tests/build pass dan OpenAPI/docs diperbarui.

## Acceptance Criteria Context / BDD Detail

### AC1 — Resolve/create nested Drive folder

- Trigger utama tetap endpoint existing `POST /api/sessions/{session_id}/cloud-target/resolve`.
- Session harus minimal `sessions.StatusLocked`; handler saat ini sudah menolak active session dengan `CLOUD_PENDING_LOCAL_COMPLETION` + `WAIT_FOR_LOCAL_COMPLETION` dan perilaku ini harus dipertahankan.
- Template folder default berasal dari Story 6.6 dan harus tetap: `{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}`.
- Tanggal harus deterministik. Gunakan timestamp session yang stabil (prefer `StartedAt` atau field yang saat ini dipakai `BuildSessionDriveFolderPath`; jangan memakai `time.Now()` untuk target session jika itu membuat path berubah antar retry/restart).
- Folder dibuat di bawah configured `drive_root_folder_id` dari cloud settings, bukan di My Drive root implicit.

### AC2 — Idempotency by parent + name

- Google Drive tidak menjamin nama folder unik secara global maupun per parent jika dibuat berulang. Resolver wajib melakukan lookup folder dengan kombinasi parent folder ID + exact sanitized folder name + mime type folder + not trashed sebelum create.
- Jika lookup menemukan folder existing yang valid, reuse ID tersebut.
- Jika tidak ada, create folder baru dengan `mimeType = application/vnd.google-apps.folder` dan `parents = [parent_id]`.
- Jika ada lebih dari satu folder existing dengan nama sama di parent sama, pilih deterministik dan aman (mis. oldest/first by createdTime asc) dan catat warning/activity aman; jangan membuat folder ketiga.
- Re-run resolver setelah success tidak boleh menaikkan `attempt_count` terus-menerus untuk ready target yang sama, mengikuti pola existing idempotent Story 6.6 test.

### AC3 — Persist Drive folder identity

- Minimal wajib menyimpan final session folder ID sebagai `drive_session_folder_id` atau field setara di `SessionCloudTarget` JSON/state.
- Sangat direkomendasikan menyimpan chain per level agar restart/retry tidak perlu lookup semua level jika sudah ada: contoh `drive_folder_ids` atau `drive_folder_chain` berisi `{name, folder_id, parent_id}` untuk yyyy/mm/dd/customer/order/station/session.
- `remote_identity` untuk story ini harus menjadi final Drive session folder ID, bukan path string. `drive_folder_path` tetap boleh dipertahankan sebagai display/debug metadata.
- Field legacy GCS (`bucket_name`, `object_prefix`, `target_root_prefix`) jangan menjadi sumber kebenaran baru. Jika masih ada demi kompatibilitas internal, jangan tampil sebagai identitas Drive utama.

### AC4 — Retry/restart idempotent

- Persist target state melalui existing `upload.Store` + `upload.Persistence` mechanism.
- Setelah restart, resolver harus load existing target dan jika `drive_session_folder_id`/`remote_identity` ada serta status ready, return target yang sama tanpa create ulang.
- Jika state local hilang tapi Drive folder sudah ada, lookup parent+name harus menemukan chain existing dan menghasilkan final ID yang sama.

### AC5 — Safe failure tanpa regresi local flow

- Jika Drive belum configured, credential belum ada, connection status bukan `authorized`, root folder invalid, atau API create/list gagal: simpan target `failed` dengan `last_error_code` dan `last_error_action` aman.
- Gunakan action Drive-specific bila tersedia/ditambahkan: `FIX_DRIVE_CREDENTIALS`, `FIX_DRIVE_FOLDER`, `RETRY_DRIVE_CHECK`, `FIX_DRIVE_FOLDER_RULES`, `RETRY_DRIVE_FOLDER`. Jika mempertahankan konstanta generic existing, UI copy tetap harus menyebut Google Drive, bukan GCS/cloud bucket.
- Local output, session status, photo records, processing jobs, quarantine, dan processing queue tidak boleh dimodifikasi oleh kegagalan folder Drive.
- Jangan mulai upload file original/graded di story ini; upload konten file milik Story 6.8.

### AC6 — Sanitasi folder name

- Gunakan fungsi Story 6.6 `cloud.SafeDriveSegment` / `BuildDriveFolderPreview` sebagai dasar agar preview dan resolver identik.
- Harus mencegah slash `/`, backslash `\\`, traversal `..`, control chars, segment kosong, `.`/`..`, dan segment terlalu panjang.
- Segment yang terlalu panjang dipotong deterministik sesuai batas existing (saat ini 120 char) tanpa menghasilkan string kosong.
- Jangan pakai input mentah sebagai query Drive atau response operator jika mengandung secret/control char.

### AC7 — Dashboard/session detail status

- Session detail harus menampilkan status Drive target terpisah dari local save/processing.
- Label/action yang terlihat operator: `Resolve Drive Folder` untuk pending/not resolved, `Retry Drive Folder` untuk failed, status teks seperti `Drive folder ready`, `Drive folder failed`, `Drive not configured`, bukan hanya warna/icon.
- Button harus memanggil existing mutation `resolveSessionCloudTarget(sessionId)` atau endpoint yang sama; update TanStack Query invalidation tetap menyegarkan session detail, sessions list, dan activity.
- SSE event `cloud.target_resolved` / `cloud.target_failed` harus membawa metadata aman: `session_id`, `station_id`, `status`, `drive_root_folder_id`, `drive_root_folder_name`, `drive_folder_path`, final `drive_session_folder_id`/`remote_identity`, `last_error_code`, `last_error_action`. Jangan kirim credential/token/private key/path credential.

### AC8 — Tests/build/docs

- Tambahkan/ubah Go tests untuk resolver, Drive folder client abstraction, API handler, persistence/restart, sanitasi, dan failure mapping.
- Tambahkan/ubah web tests/typecheck jika komponen session detail/cloud target DTO berubah.
- Jalankan minimal:
  - `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`
  - `cd apps/web && npm run typecheck`
  - `cd apps/web && npm run build`
  - `npm run typecheck`
- Update docs/API contract: minimal `docs/api/cloud-storage-story-6-1.md` dan OpenAPI jika endpoint cloud target terdokumentasi.

## Tasks / Subtasks

- [x] Backend Drive folder client abstraction
  - [x] Tambahkan interface di `apps/agent/internal/upload` atau `apps/agent/internal/cloud` untuk operasi Drive folder: lookup folder by parent+name, create folder, optional get root folder metadata.
  - [x] Implementasi real Google Drive API menggunakan credential server-side dari Story 6.6; jangan akses credential dari browser.
  - [x] Tambahkan fake/in-memory implementation untuk tests agar tidak bergantung network.
- [x] Upgrade target model dari path identity ke folder ID identity
  - [x] Update `apps/agent/internal/upload/target.go` dengan final session folder ID (`DriveSessionFolderID` json `drive_session_folder_id`) dan optional folder chain (`drive_folder_ids`/`drive_folder_chain`).
  - [x] Pastikan `RemoteIdentity` diisi final folder ID setelah resolve sukses.
  - [x] Pertahankan `DriveFolderPath` sebagai display path; jangan lagi memakai `ObjectPrefix` sebagai identitas utama.
  - [x] Update persistence JSON agar backward compatible terhadap target Story 6.6 yang mungkin hanya punya path.
- [x] Implement create-or-resolve folder chain
  - [x] Refactor `apps/agent/internal/upload/resolver.go` agar setelah settings valid, resolver membuat/resolve setiap segment di bawah `DriveRootFolderID`.
  - [x] Pakai sanitized segments yang sama dengan preview (`BuildSessionDriveFolderPath`/`BuildDriveFolderPreview`) agar admin preview = actual folder.
  - [x] Lookup existing folder sebelum create pada setiap level.
  - [x] Jika target ready existing sudah punya final folder ID, return idempotent tanpa API call create.
  - [x] On failure, persist safe failure status tanpa mengubah local session/photo/processing.
- [x] API/SSE contract update
  - [x] Update `apps/agent/internal/api/cloud_targets.go` response/event payload agar menyertakan final `drive_session_folder_id`/folder chain dan tetap safe.
  - [x] Update error code/action mapping ke Drive-specific copy bila memungkinkan.
  - [x] Pastikan endpoint GET `cloud-target` mengembalikan target pending/failed/ready dengan field Drive baru.
- [x] UI session detail/dashboard update
  - [x] Cari komponen session detail/status yang menampilkan upload/cloud status; update copy dari generic cloud/GCS ke Google Drive folder target.
  - [x] Pastikan action `Resolve Drive Folder` / `Retry Drive Folder` tersedia dan invalidation TanStack Query tetap bekerja.
  - [x] Tampilkan Drive target status terpisah dari local save/processing dan dari upload file (Story 6.8).
- [x] Tests
  - [x] RED tests: `resolver` membuat folder chain sesuai template dan menyimpan final folder ID.
  - [x] RED tests: existing folder by parent+name direuse dan tidak create duplikat.
  - [x] RED tests: retry/restart dengan persisted target ready menghasilkan identity sama.
  - [x] RED tests: missing config/unauthorized/root unwritable/API failure menyimpan failed target dan tidak mengubah local state.
  - [x] RED tests: unsafe names ditolak/disanitasi sesuai Story 6.6 preview.
  - [x] RED tests: API response/SSE tidak mengandung credential, private key, token, service account JSON, credential file path.
  - [x] GREEN/refactor sesuai pola existing.
- [x] Docs/build validation
  - [x] Update `docs/api/cloud-storage-story-6-1.md` dan `docs/api/openapi.yaml` jika ada.
  - [x] Jalankan semua command validasi di AC8 dan catat hasil di Dev Agent Record.

## Dev Notes

### Review Findings

- [x] [Review][Patch] Drive folder retry button keys off upload status, not target status [apps/web/src/features/sessions/live-station-cards.tsx:121]
- [x] [Review][Patch] Session detail summary lacks separate Drive target status/identity fields [apps/agent/internal/api/sessions.go:64]
- [x] [Review][Patch] Resolver overwrites a known ready Drive identity on transient failures [apps/agent/internal/upload/resolver.go:143]
- [x] [Review][Patch] Drive folder lookup may miss duplicate folders beyond first result page [apps/agent/internal/upload/drive_folders.go:57]


### Konteks Epic / Corrective Change

- Epic 6 sedang dalam corrective path: requirement asli adalah Google Drive fulfillment, bukan Google Cloud Storage. File proposal: `_bmad-output/planning-artifacts/sprint-change-proposal-2026-05-20-google-drive.md`.
- Flow target yang disetujui: `local complete → resolve remote folder → create upload jobs → upload originals/graded → retry/dedupe → restart recovery`.
- Story 6.7 hanya mencakup tahap `resolve remote folder`. Jangan upload file di story ini.
- PRD FR terkait: FR56 admin connect Google Drive, FR57 create Drive folders by customer/order, FR58 upload original/graded, FR59 track status, FR60 retry, FR61 preserve local files on Drive failure, FR62 upload non-blocking, FR63 duplicate prevention.
- NFR penting: local save independent dari Drive, upload/Drive work tidak boleh mengganggu capture/session/processing, state upload recoverable after restart, status operator actionable.

### Prior Story 6.6 Learnings (wajib dipakai)

Story 6.6 selesai dan menetapkan fondasi berikut:

- Provider default sekarang `google_drive`.
- `GET /api/cloud/settings` harus mengembalikan allowlist metadata aman saja: `provider`, `drive_root_folder_id`, `drive_root_folder_name`, `folder_naming_template`, `credentials_configured`, `connection_status`, `last_checked_at`, `last_error_code`, `last_error_action`.
- Browser tidak boleh menerima service account JSON, OAuth token, refresh token, private key, atau credential file path mentah.
- `GoogleDriveChecker` sudah diverifikasi untuk root folder metadata + write/probe folder capability; gunakan pola ini/abstraction yang sama agar Story 6.7 tidak mengulang credential handling secara tidak aman.
- Folder preview endpoint sudah menghasilkan path deterministik dengan template default dan sanitasi Drive-safe.
- Review Story 6.6 menemukan dan memperbaiki risiko: checker sebelumnya hanya read metadata, public settings masih expose field legacy, dan error action folder rule harus Drive-specific. Jangan regresi.
- Files yang disentuh Story 6.6 dan sangat relevan: `apps/agent/internal/cloud/checker.go`, `config.go`, `object_keys.go`, `persistence.go`, `apps/agent/internal/upload/resolver.go`, `target.go`, `persistence.go`, `apps/agent/internal/api/cloud_targets.go`, `cloud_settings.go`, `apps/web/src/features/cloud/cloud-settings.tsx`, `apps/web/src/lib/api/client.ts`, `docs/api/cloud-storage-story-6-1.md`.

### Current Code State yang Harus Dibaca Developer

Developer wajib membaca file UPDATE berikut sebelum coding:

- `apps/agent/internal/upload/resolver.go`
  - Saat ini `Resolver.ResolveForSession` hanya validasi settings + membangun `folderPath`/`identity` lokal via `BuildSessionDriveFolderPath`.
  - Komentar eksplisit menyatakan actual Drive folder creation/upload ditunda ke later Drive stories; Story 6.7 menghapus gap ini.
  - Existing ready target langsung direturn idempotent; preserve behavior ini, tetapi pastikan ready target punya final Drive folder ID.
  - Saat ini `RemoteIdentity` kemungkinan path identity, bukan folder ID; ubah sesuai AC3.
- `apps/agent/internal/upload/target.go`
  - `SessionCloudTarget` saat ini punya field legacy `BucketName`, `TargetRootPrefix`, `ObjectPrefix`, serta Drive fields `DriveRootFolderID`, `DriveRootFolderName`, `DriveFolderPath`, `RemoteIdentity`.
  - Tambahkan field Drive folder ID final dan optional chain; jangan hapus field legacy tanpa mengecek tests/story berikutnya karena upload/retry code mungkin masih compile terhadapnya.
  - Error constants masih generic `CLOUD_*` dan action `FIX_CLOUD_*`; boleh dipertahankan jika API/UI copy tetap Drive-safe, tetapi prefer alias/action Drive-specific agar corrective stories jelas.
- `apps/agent/internal/cloud/object_keys.go`
  - `BuildDriveFolderPreview` dan `SafeDriveSegment` adalah sumber sanitasi dari Story 6.6.
  - `SafeDriveSegment` saat ini menolak `..` dan control chars, memakai `SafeSegment`, truncate 120 char, dan reject empty/dot/dotdot.
  - Jangan membuat sanitasi baru yang berbeda dari preview.
- `apps/agent/internal/cloud/checker.go`
  - Sudah punya abstraction/probe pattern untuk Google Drive checker. Reuse credential/client creation pattern, jangan duplikasi unsafe parsing credential.
- `apps/agent/internal/api/cloud_targets.go`
  - Endpoint `GET /api/sessions/{session_id}/cloud-target` dan `POST /api/sessions/{session_id}/cloud-target/resolve` sudah ada.
  - Handler sudah guard `session.Status == locked`; preserve.
  - SSE payload saat ini belum menyertakan final session folder ID karena belum ada.
- `apps/agent/internal/api/cloud_targets_test.go`
  - Test existing `TestCloudTargetResolveSuccessIdempotentAndSafe` memastikan idempotency dan no secret leak. Update expectation dari path identity ke folder ID identity.
- `apps/web/src/features/sessions/use-resolve-cloud-target-mutation.ts`
  - Mutation existing sudah invalidasi session detail, sessions list, activity. Pakai kembali.
- `apps/web/src/lib/api/client.ts`
  - DTO cloud target perlu field baru `drive_session_folder_id` / folder chain jika ditambahkan.
- `docs/api/cloud-storage-story-6-1.md`
  - Story 6.6 sudah memperbarui Google Drive settings; tambahkan bagian cloud target/folder resolution.

### Arsitektur dan Guardrails

- Go service owns all filesystem, worker, credential, upload, and Drive API logic. Next.js tidak boleh membaca folder lokal atau memakai credential Drive.
- API pattern: REST commands under `/api`, SSE under `/events`, response `{data: ...}`, error `{error:{code,message,action,details}}`.
- JSON/status use `snake_case`; SSE events use dot notation.
- Google Drive upload worker/target logic berada di `apps/agent/internal/upload`; cloud settings/checker di `apps/agent/internal/cloud`; API handlers di `apps/agent/internal/api`; frontend feature UI di `apps/web/src/features/*`.
- Local filesystem adalah safety source; Drive failure tidak boleh menghapus/mengubah local files.
- Capture/local processing harus tetap non-blocking terhadap Drive work. Folder resolve boleh gagal/di-retry tanpa menahan session baru.
- Browser/API/SSE/activity logs harus secret-safe. Jangan log raw credential, token, private key, service account JSON, OAuth refresh token, atau credential file path.

### Google Drive API Implementation Notes

- Folder MIME type: `application/vnd.google-apps.folder`.
- Lookup query pattern harus aman dan escaped, secara konseptual: `name = '<escaped name>' and '<parent_id>' in parents and mimeType = 'application/vnd.google-apps.folder' and trashed = false`.
- Gunakan field minimal untuk lookup/create: `id`, `name`, `parents`, optionally `createdTime`.
- Buat folder dengan metadata `Name`, `MimeType`, `Parents`.
- Permission/share behavior bukan bagian story ini kecuali diperlukan oleh configured service account access; jangan membuka public sharing tanpa requirement.
- Simpan ID, bukan nama/path, karena Drive names are not globally unique.
- Network/API errors harus dimap ke safe code/action; raw Google API error boleh masuk details hanya jika sudah disanitasi dan tidak mengandung credential.

### Data Model Recommendation

Disarankan update `SessionCloudTarget`:

```go
type DriveFolderRef struct {
    Level    string `json:"level"`
    Name     string `json:"name"`
    FolderID string `json:"folder_id"`
    ParentID string `json:"parent_id,omitempty"`
}

type SessionCloudTarget struct {
    // existing fields...
    DriveSessionFolderID string           `json:"drive_session_folder_id,omitempty"`
    DriveFolderChain     []DriveFolderRef `json:"drive_folder_chain,omitempty"`
}
```

Status success:

- `status = ready`
- `drive_root_folder_id = settings.DriveRootFolderID`
- `drive_folder_path = yyyy/mm/dd/.../session`
- `drive_session_folder_id = <final folder id>`
- `remote_identity = <same final folder id>`
- `resolved_at` set
- `last_error_code/action` cleared

Failure:

- `status = failed`
- `last_error_code/action` set
- no local session/photo/processing mutation
- preserve previous ready target unless explicitly invalid? Prefer: if existing ready target has final folder ID, do not overwrite with failed due to transient recheck unless user intentionally retries and Drive confirms invalid. Be conservative to avoid losing known identity.

### UX Copy Guidance

- Use Google Drive words consistently: “Drive folder”, “Google Drive target”, “Resolve Drive Folder”, “Retry Drive Folder”.
- Avoid “bucket”, “object prefix”, “GCS”, “cloud object” in user-facing UI for this story.
- Show local save/processing separately from Drive folder status and from file upload status.
- Error action must tell operator what to do: configure Drive, recheck Drive, fix folder rules, retry Drive folder.

### Testing Strategy

Backend priority tests:

1. Resolver creates chain under root with fake Drive client:
   - input session customer/order unsafe but valid after sanitization
   - expect folders created in order and target ready with final ID.
2. Existing folders reused:
   - pre-seed fake Drive with same parent+name chain
   - run resolver twice
   - expect no duplicate folder creation and same final ID.
3. Restart persistence:
   - resolve once, save store
   - load new store/resolver
   - resolve same session
   - expect same `drive_session_folder_id` and no create.
4. Failure paths:
   - no credentials/root/authorized false/root not writable/API list/create error
   - expect failed status, safe code/action, no local state changes.
5. Sanitization parity:
   - compare resolver path segments with `BuildDriveFolderPreview`; slash/backslash/traversal/control/empty/long cases covered.
6. API/SSE safety:
   - response body/SSE/activity must not contain `private_key`, `service_account`, `refresh_token`, raw credential path.

Frontend/type tests:

- TypeScript DTO includes `drive_session_folder_id` optional.
- Session detail/action rendering handles pending/ready/failed/not configured.

### Regression Risks to Avoid

- Reintroducing GCS terminology/fields into public API/UI.
- Treating `drive_folder_path` as identity; path is not enough for Drive due duplicate names.
- Creating duplicate folders on every retry because lookup is missing or query does not filter parent.
- Using `time.Now()` for session path and changing date between retries.
- Exposing credential metadata through error details, activity, SSE, or browser DTO.
- Starting file upload jobs in Story 6.7; keep that for Story 6.8.
- Breaking existing local session/processing/quarantine tests while changing upload target model.

## File References

### Likely backend UPDATE files

- `apps/agent/internal/upload/resolver.go` — implement actual Drive folder create/resolve chain.
- `apps/agent/internal/upload/target.go` — add Drive folder identity fields and safe DTO JSON.
- `apps/agent/internal/upload/persistence.go` — ensure target persistence handles new fields/backward compatibility.
- `apps/agent/internal/upload/resolver_test.go` — update/add resolver tests.
- `apps/agent/internal/cloud/object_keys.go` — reuse/possibly expose path segment builder; keep preview parity.
- `apps/agent/internal/cloud/checker.go` — reuse Drive client/credential patterns.
- `apps/agent/internal/api/cloud_targets.go` — response/SSE payload and error mapping.
- `apps/agent/internal/api/cloud_targets_test.go` — API safety/idempotency tests.
- `apps/agent/cmd/selfstudio-agent/main.go` — wire real Drive folder resolver/client if dependency injection required.

### Likely frontend UPDATE files

- `apps/web/src/lib/api/client.ts` — update CloudTarget type fields.
- `apps/web/src/features/sessions/use-resolve-cloud-target-mutation.ts` — likely no logic change; verify query invalidation still sufficient.
- Session detail/dashboard components under `apps/web/src/features/sessions` / `apps/web/src/features/dashboard` — update labels/actions/status rendering.

### Docs UPDATE files

- `docs/api/cloud-storage-story-6-1.md` — add Drive folder target resolution contract.
- `docs/api/openapi.yaml` — update if present/used for cloud target endpoints.

## Previous Story Intelligence

- Story 6.6 changed broad surfaces from GCS to Drive and passed validations:
  - `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`
  - `cd apps/web && npm run typecheck`
  - `cd apps/web && npm run build`
  - `npm run typecheck`
- Review fixes in Story 6.6 are direct guardrails for 6.7:
  - Always verify writable Drive root for config check.
  - Keep public settings allowlist Drive-safe.
  - Keep Drive folder rule errors mapped to Drive-specific action.
- Story 6.6 left a deliberate placeholder in `upload.Resolver`: folder path identity only, no actual Drive folder creation. Story 6.7 must close this gap.

## Git Intelligence Summary

- Recent git history is sparse (`c5ab05b Add from-scratch setup guide`, `1dbc6cb Initial Selfstudio camera capture spike`) and does not reflect most BMad story work because many project files are currently untracked.
- Treat current working tree files and prior story documents as source of truth over git history.
- Do not assume absence from git means feature absent; inspect actual files before editing.

## Latest Technical Notes

- Use Google Drive API folder semantics: folders are files with `mimeType = application/vnd.google-apps.folder` and `parents`.
- Idempotency must be application-level by lookup parent+name before create; Drive itself permits duplicate names.
- Store Drive IDs. Names/path are display hints, not stable unique remote identity.
- Keep scopes as narrow as practical for service account/OAuth mode already established in Story 6.6; no credential should reach browser.

## Project Context Reference

- Project: `selfstudio`.
- Stack: Go local service on Windows admin PC + Next.js App Router dashboard + Supabase Postgres metadata.
- Communication/API conventions: snake_case JSON, REST command endpoints, SSE realtime events, safe `{error:{code,message,action,details}}` responses.
- Critical product invariant: local capture/save/processing remains source of safety and must continue even when Drive is unavailable.

## Dev Agent Record

### Debug Log

- 2026-05-20 — Memuat Story 6.7, sprint status, dan konteks Dev Notes.
- 2026-05-20 — Menambahkan Drive folder client abstraction, resolver chain idempotent, model target Drive folder ID, API/SSE metadata aman, UI copy Drive, docs/OpenAPI, dan tests.
- 2026-05-20 — Catatan validasi: percobaan awal `go test ./...` sempat gagal karena transient Windows `Access is denied`; rerun berhasil.
- 2026-05-20 — Melanjutkan setelah BMad code review; menambahkan RED tests untuk preserve ready identity transient failure, duplicate selection deterministik, dan session summary field Drive.
- 2026-05-20 — Catatan validasi patch: dua run Go test awal sempat gagal pada cleanup TempDir Windows (`directory is not empty`), lalu rerun full suite berhasil.
- 2026-05-20 — Re-review follow-up fixes: memverifikasi 4 temuan patch sudah resolved dan menjalankan ulang validasi backend/frontend.

### Completion Notes

- Implementasi membuat/resolve chain folder Google Drive berdasarkan template session stabil dan lookup parent+name sebelum create.
- `remote_identity` sekarang final `drive_session_folder_id`; `drive_folder_path` tetap sebagai metadata display/debug.
- Failure Drive tetap tersimpan aman sebagai target failed dan tidak menyentuh local session/photo/processing.
- UI operator memakai copy Drive folder dan action `Resolve Drive Folder` / `Retry Drive Folder`.
- ✅ Resolved review finding [Patch]: Drive folder retry button sekarang memakai `drive_target_status`, bukan aggregate `upload_status`.
- ✅ Resolved review finding [Patch]: Session detail summary sekarang mengekspos field Drive target terpisah: status, identity, folder ID, path, root, dan last error/action.
- ✅ Resolved review finding [Patch]: Resolver mempertahankan/repair ready Drive identity yang sudah diketahui saat lookup/create mengalami transient failure, bukan menimpa menjadi failed.
- ✅ Resolved review finding [Patch]: Lookup Google Drive folder sekarang memproses semua page dan resolver memilih duplicate secara deterministik (oldest createdTime, lalu ID).

### Validation

- PASS — `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`
- PASS — `cd apps/web && npm run typecheck`
- PASS — `cd apps/web && npm run build`
- PASS — `npm run typecheck`
- PASS — Patch rerun `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`
- PASS — Patch rerun `cd apps/web && npm run typecheck`
- PASS — Patch rerun `cd apps/web && npm run build`
- PASS — Patch rerun `npm run typecheck`
- PASS — Re-review `cd apps/agent && GOTMPDIR=../../.gotmp go test ./internal/upload ./internal/api`
- PASS — Re-review `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`
- PASS — Re-review `cd apps/web && npm run typecheck`
- PASS — Re-review `cd apps/web && npm run build`
- PASS — Re-review `npm run typecheck`

## File List

- apps/agent/internal/upload/drive_folders.go
- apps/agent/internal/upload/resolver.go
- apps/agent/internal/upload/target.go
- apps/agent/internal/upload/resolver_test.go
- apps/agent/internal/api/cloud_targets.go
- apps/agent/internal/api/cloud_targets_test.go
- apps/agent/internal/api/sessions.go
- apps/agent/internal/api/sessions_test.go
- apps/web/src/lib/api/client.ts
- apps/web/src/features/sessions/live-station-cards.tsx
- docs/api/cloud-storage-story-6-1.md
- docs/api/openapi.yaml
- _bmad-output/implementation-artifacts/sprint-status.yaml

## Change Log

- 2026-05-20 — Story prepared for development with comprehensive Google Drive folder resolver context; status set to ready-for-dev.
- 2026-05-20 — Implemented Google Drive per-session folder resolve/create with idempotent Drive folder ID identity; status set to review.
- 2026-05-20 — Addressed BMad code review patch findings: target-status retry button, separate Drive summary fields, transient ready identity preservation, paginated deterministic duplicate lookup; status remains review.
- 2026-05-20 — Re-review clean; all prior patch findings verified resolved; status set to done.

## Completion Note

Ultimate context engine analysis completed - comprehensive developer guide created.
