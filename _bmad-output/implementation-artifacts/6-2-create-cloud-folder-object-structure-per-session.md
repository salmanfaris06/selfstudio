# Story 6.2: Create Cloud Folder/Object Structure Per Session

Status: done

## Story

Sebagai operator, saya ingin asset cloud diorganisasi per customer dan order number, sehingga fulfillment pasca-event mudah ditemukan dan struktur remote session bisa dibuat/di-resolve secara idempotent tanpa meng-upload file foto dulu.

## Acceptance Criteria

1. Given session mencapai local complete, when cloud fulfillment dimulai, then sistem membuat atau me-resolve remote path/object prefix berdasarkan customer name dan order number.
2. Given customer/order/session/file metadata mengandung karakter tidak aman, when remote prefix dibuat, then sanitized names mencegah invalid path, traversal, leading slash, control characters, empty segment, dan key terlalu panjang.
3. Given remote structure berhasil dibuat/di-resolve, when operasi selesai, then remote identity disimpan untuk session agar future upload memakai identitas yang sama.
4. Given worker/API dipanggil ulang karena retry atau restart, when remote identity sudah ada atau prefix sudah pernah dibuat, then operasi idempotent dan tidak membuat duplicate folder/job/identity.
5. Given cloud config belum ada, unauthorized, atau target rule invalid, when cloud fulfillment dimulai, then sistem menyimpan status/actionable failure yang aman tanpa mengubah local output, local processing, atau session routing.
6. Given Story 6.3+ belum diimplementasikan, when Story 6.2 selesai, then sistem belum meng-upload original/graded JPG dan belum menjalankan retry upload per-file; scope hanya session-level remote prefix/folder/object-structure resolution.
7. Given dashboard/session detail menampilkan status, when remote identity pending/ready/failed, then cloud/upload status tetap terpisah dari local save/processing status dengan label teks dan action yang jelas.
8. Tests/build pass untuk Go agent, web typecheck/build yang relevan, dan OpenAPI/API contract diperbarui.

## Tasks / Subtasks

- [x] Definisikan model session cloud fulfillment dan batas scope Story 6.2. (AC: 1, 3, 4, 6)
  - [x] Buat package baru `apps/agent/internal/upload` untuk domain upload/session cloud structure; jangan campur dengan `internal/cloud` yang khusus config/connection/object-key helper.
  - [x] Definisikan record session-level, misalnya `SessionCloudTarget` / `UploadSessionTarget`, dengan field aman: `session_id`, `station_id`, `bucket_name`, `target_root_prefix`, `object_prefix`, `remote_identity`, `status`, `attempt_count`, `last_error_code`, `last_error_action`, `last_checked_at`/`resolved_at`, timestamps.
  - [x] Status minimal: `pending`, `resolving`, `ready`, `failed`; gunakan snake_case JSON dan lowercase status.
  - [x] Tegaskan tidak ada per-file upload job di story ini; per-file upload original/graded adalah Story 6.3, retry/dedup per-file adalah Story 6.4, recovery queue adalah Story 6.5.
- [x] Implement persistence idempotent untuk session cloud target. (AC: 3, 4, 5)
  - [x] Simpan state runtime di `SELFSTUDIO_LOCAL_DATA_DIR/state/upload_targets.json` atau nama setara di bawah `local-data/state`; jangan simpan di `apps/web/public` atau source tree.
  - [x] Ikuti atomic JSON persistence pattern dari `sessions.Persistence`, `photos.Persistence`, `cloud.Persistence`: versioned state file, temp file + sync + rename, validation saat load.
  - [x] Unique identity wajib berbasis session, bukan freeform request: satu `session_id` hanya punya satu target record aktif.
  - [x] `ResolveForSession` harus return existing `ready` target tanpa membuat ulang prefix; jika previous `failed`, retry boleh update record yang sama dan increment `attempt_count`.
  - [x] Persistence failure tidak boleh dipalsukan sebagai success; jangan publish SSE/activity success kalau save gagal.
- [x] Bangun object prefix resolver yang memakai helper Story 6.1. (AC: 1, 2, 4)
  - [x] Reuse `apps/agent/internal/cloud.SanitizeTargetRoot`, `SafeSegment`, dan `BuildObjectKey` / `ObjectNamingTemplate`; jangan membuat sanitizer baru yang berbeda.
  - [x] Prefix session harus kompatibel dengan final template Story 6.1: `{target_root_prefix}/{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}`; `asset_kind` dan `safe_file_name` ditambahkan nanti oleh Story 6.3.
  - [x] Tanggal gunakan timestamp session yang deterministik (`StartedAt` direkomendasikan; jika product memilih local completion timestamp, dokumentasikan dan gunakan konsisten). Jangan gunakan `time.Now()` untuk session real karena retry/restart akan menghasilkan prefix berbeda.
  - [x] Validasi `station_id` dan `session_id` dari backend/session store, bukan display name; safe customer/order berasal dari `sessions.Session.CustomerName` dan `OrderNumber`.
  - [x] Reject/return actionable error untuk prefix kosong akhir, leading slash, `./`, `../`, backslash, control chars, dan length berlebihan; jangan silent truncate bagian yang menjadi identity.
- [x] Implement remote GCS folder/object-structure resolution tanpa upload foto customer. (AC: 1, 3, 4, 5, 6)
  - [x] Gunakan config dari `internal/cloud.Persistence`; hanya lanjut jika provider `gcs`, bucket valid, credential configured, dan `connection_status == authorized` atau check/resolution dapat membuktikan akses aman.
  - [x] Karena GCS object namespace flat, "folder" adalah prefix. Untuk bukti resolusi, pilih salah satu pola aman: (a) simpan prefix-only identity tanpa membuat object, atau (b) buat marker object kecil seperti `{object_prefix}/.selfstudio_session.json` lalu overwrite/idempotent dengan precondition yang aman. Dokumentasikan pilihan di code/OpenAPI/story comments.
  - [x] Jika membuat marker object, marker tidak boleh berisi secret atau full local path; cukup session_id/station_id/created_at/schema_version. Local customer/order sudah tercermin dalam prefix dan merupakan customer data.
  - [x] Jangan upload original/graded JPG di story ini. Jangan enumerate local photo files kecuali hanya untuk status summary non-mutating.
  - [x] Map error aman: `CLOUD_NOT_CONFIGURED`, `CLOUD_NOT_AUTHORIZED`, `CLOUD_TARGET_RULE_INVALID`, `CLOUD_PREFIX_RESOLVE_FAILED`, `CLOUD_TARGET_SAVE_FAILED` dengan actions seperti `FIX_CLOUD_CREDENTIALS`, `RETRY_CLOUD_CHECK`, `FIX_CLOUD_TARGET_RULES`, `RETRY_CLOUD_TARGET`.
- [x] Tambahkan API dan wiring Go service. (AC: 1, 3, 4, 5, 7)
  - [x] Endpoint disarankan: `POST /api/sessions/{session_id}/cloud-target/resolve` untuk trigger/resolve target; `GET /api/sessions/{session_id}/cloud-target` untuk status session target.
  - [x] Semua endpoint wajib `RequireAuth`; POST wajib `RequireTrustedOrigin`.
  - [x] Success response wajib `{data}` wrapper; errors wajib `{error:{code,message,action,details}}` memakai helper `response.go`.
  - [x] Wire handler di chain mux `apps/agent/internal/api/health.go` atau split file setara tanpa merusak route existing.
  - [x] Handler harus membaca session dari `sessions.Store`; jika session belum local complete karena status model saat ini hanya `active/locked`, gunakan guard minimal yang aman: hanya allow locked session dengan local processing summary complete jika dapat diverifikasi. Jika local_complete state belum tersedia di code, dokumentasikan gap dan gunakan explicit status `pending_local_completion`/action `WAIT_FOR_LOCAL_COMPLETION`, jangan menandai ready palsu.
- [x] Integrasikan status di session summary/dashboard tanpa mencampur local status. (AC: 5, 7)
  - [x] Update `SessionSummary.UploadStatus` yang saat ini `placeholder` di `apps/agent/internal/api/sessions.go` agar membaca session cloud target: `not_configured`, `pending`, `resolving`, `ready`, `failed`, atau `placeholder` hanya bila service belum wired.
  - [x] Tambahkan safe `cloud_target` detail bila diperlukan: `object_prefix`, `bucket_name`, `status`, `last_error_code`, `last_error_action`, timestamps; jangan expose credential path/json/token.
  - [x] Update frontend session/detail/dashboard area sesuai pola existing agar label teks cloud target terlihat dan action `Resolve Cloud Target` / `Retry Cloud Target` tersedia ketika aman.
  - [x] Pastikan cloud status tetap terpisah dari processing queue/local save status; local session tetap usable meski cloud failed.
- [x] Publish SSE dan activity log aman. (AC: 3, 4, 5, 7)
  - [x] Publish event `upload.target_resolved` atau `cloud.target_resolved` saat target ready, dan `upload.target_failed`/`cloud.target_failed` saat gagal. Pilih dot-notation konsisten dan dokumentasikan di OpenAPI.
  - [x] SSE payload hanya berisi safe metadata: `session_id`, `station_id`, `status`, `bucket_name`, `object_prefix`, `last_error_code`, `last_error_action`; tidak ada credential, local full source paths, raw GCS errors, atau service account email/private key.
  - [x] Activity log: `cloud.target_resolved` / `cloud.target_failed` dengan result/status dan safe message. Jangan log raw GCS response jika berpotensi berisi secret/request metadata.
  - [x] Dashboard SSE invalidation perlu refresh session detail/summary dan activity, bukan processing queue kecuali ada alasan eksplisit.
- [x] Update OpenAPI dan docs. (AC: 1, 3, 5, 6, 7, 8)
  - [x] Tambahkan schemas `SessionCloudTarget`, `SessionCloudTargetResponse`, resolve response, SSE events, status enum, dan error codes/actions.
  - [x] Dokumentasikan prefix convention session-level dan hubungannya dengan Story 6.1 object naming template.
  - [x] Dokumentasikan bahwa Story 6.2 belum upload file JPG; Story 6.3 akan menambahkan `{asset_kind}/{safe_file_name}` dan per-file upload status.
- [x] Tambahkan tests dan validasi. (AC: 1-8)
  - [x] Go unit tests untuk prefix generation deterministic: same session/config menghasilkan prefix sama across retry; customer/order unsafe disanitasi.
  - [x] Go persistence tests untuk default load, validation, atomic save, unique session target, failed→retry update, no duplicate records.
  - [x] Go API tests untuk auth/trusted-origin, session not found, not local complete/locked guard, cloud not configured/unauthorized, success idempotency, safe errors, no SSE/activity success on persistence failure.
  - [x] Fake GCS/prefix resolver tests agar tidak butuh network; real GCS client harus behind interface.
  - [x] Web tests/typecheck untuk API client/status UI bila frontend disentuh.
  - [x] Run `cd apps/agent && go test ./...`.
  - [x] Run `cd apps/web && npm run typecheck`.
  - [x] Run `cd apps/web && npm run build` jika environment/dependencies memungkinkan.

### Review Follow-ups (AI)

- [x] [HIGH] Cloud target resolve action is unreachable from the dashboard because `LiveStationCards` only passes sessions with `status === "active"` into each card, while the backend correctly allows resolve only for `locked` sessions. After a session is ended/locked it disappears from the card, so operators cannot see pending/failed/ready cloud target status or click `Resolve Cloud Target` / `Retry Cloud Target` from this UI. Evidence: `apps/web/src/features/sessions/live-station-cards.tsx:37` filters active sessions only, and the resolve button at `apps/web/src/features/sessions/live-station-cards.tsx:74` is gated on `activeSession.status !== "locked"`; backend guard requires locked at `apps/agent/internal/api/cloud_targets.go:48`. Violates AC7 and local completion/session completion gating intent.

## Dev Notes

### Source Requirements

- Epic 6 objective: post-session cloud fulfillment dengan GCS sesuai architecture, upload original+graded setelah local completion, per-file/per-session upload tracking, retry failure, duplicate prevention, dan upload tidak memblokir session baru.
- Story 6.2 dari epics: saat session reaches local complete dan cloud fulfillment begins, sistem create/resolve remote path/object prefix berdasarkan customer/order; sanitized names prevent invalid paths; remote identity stored for session; operation idempotent across retry/restart.
- FR57: system can create cloud folders based on customer name and order number. Architecture resolves storage target as Google Cloud Storage (GCS), bukan Google Drive API.
- FR19/FR46/FR47/FR48: session summary/dashboard harus memisahkan local save status dari cloud upload status dan menyediakan log/status troubleshooting.
- FR58-FR63 adalah cerita berikutnya; Story 6.2 harus menyiapkan remote identity yang future upload/retry/dedup bisa pakai, tetapi belum menjalankan upload original/graded.
- NFR4/NFR11/NFR16/NFR21/NFR25/NFR26/NFR32: cloud fulfillment tidak boleh mengganggu capture/local save/processing, local file tetap aman saat cloud gagal, retry harus duplicate-safe, customer data diproteksi, dan status local-vs-cloud jelas terpisah.

### Current Code Context To Read Before Editing

- `apps/agent/internal/cloud/config.go`
  - Current state: domain cloud settings dari Story 6.1 (`Settings`, `PublicSettings`, `UpdateRequest`, status `not_configured/checking/authorized/failed`, actions `FIX_CLOUD_CREDENTIALS`, `FIX_CLOUD_BUCKET`, `RETRY_CLOUD_CHECK`, `FIX_CLOUD_TARGET_RULES`).
  - What this story changes: reuse settings and status; do not add upload lifecycle fields into cloud settings config.
  - Must preserve: public DTO excludes service account JSON/private key/token/path.
- `apps/agent/internal/cloud/object_keys.go`
  - Current state: GCS bucket/root validation, `SafeSegment`, `SafeFileName`, and `BuildObjectKey` final file object key template.
  - What this story changes: build session-level prefix with the same sanitizer/template semantics; avoid a second incompatible sanitizer.
  - Must preserve: rejection of traversal, backslash, leading slash, control chars, unsafe file names, and too-long keys.
- `apps/agent/internal/cloud/persistence.go`
  - Current state: atomic persistence for `local-data/state/cloud_config.json`.
  - What this story changes: read cloud settings to determine bucket/root/credentials/auth status; do not write credentials from upload target logic.
- `apps/agent/internal/api/cloud_settings.go`
  - Current state: endpoints `GET/PUT /api/cloud/settings`, `POST /api/cloud/settings/check`, `POST /api/cloud/settings/object-key-preview`; safe SSE `cloud.status_updated`; safe activity.
  - What this story changes: follow its auth/trusted-origin/error/SSE safety patterns for cloud target APIs.
  - Must preserve: if persistence fails, do not publish misleading success event/activity.
- `apps/agent/internal/sessions/store.go` and `persistence.go`
  - Current state: session statuses currently `active` and `locked`; PRD/architecture mention future `local_complete`, `upload_pending`, `uploaded`, `failed`, but code may not yet model all statuses.
  - What this story changes: cloud target resolution must be tied to an existing session and preferably local completion. If local_complete is not modeled, do not fake completion; introduce a safe explicit guard/status or document necessary status extension.
  - Must preserve: one active session per station, deterministic timer lock, session snapshot output folder.
- `apps/agent/internal/api/sessions.go`
  - Current state: session summary includes `UploadStatus: "placeholder"`; `Get` returns session, summary, recent photos; `End`/`Start` persist and publish session events.
  - What this story changes: expose session cloud target/upload status separately in summary/detail.
  - Must preserve: existing session start/end behavior, quarantine counts, local output folder, photo counts, failure counts.
- `apps/agent/internal/photos/store.go`
  - Current state: per-photo local original/graded paths and statuses exist; original/graded local state is independent from cloud.
  - What this story changes: likely none directly. Do not add cloud fields to photos yet unless strictly needed for future references; per-file upload status belongs Story 6.3/6.4.
- `apps/agent/cmd/selfstudio-agent/main.go`
  - Current state: wires station/session/photo/quarantine/processing recovery and cloud settings handler; no `internal/upload` yet.
  - What this story changes: create upload target persistence/store/resolver and wire handler with fake/testable resolver interface.
  - Must preserve: startup recovery order; do not block server start because cloud is unconfigured unless target persistence file is corrupt and truly unsafe.
- `apps/web/src/features/cloud/cloud-settings.tsx`
  - Current state: cloud config UI; explicitly states Story 6.1 does not upload files; shows object naming preview.
  - What this story changes: may add a separate session cloud target panel/action; do not put per-session resolve controls inside credential text area.
- `apps/web/src/features/health/health-dashboard.tsx`
  - Current state: listens to `cloud.status_updated` and invalidates broad photo/session/activity queries. Existing cloud status update invalidation is grouped with photo affected queries.
  - What this story changes: add event listener/invalidation for chosen cloud target event so session detail/status refreshes; avoid unnecessary processing queue invalidations if event is only cloud target.
- `docs/api/openapi.yaml`
  - Current state: documents Story 6.1 cloud settings schemas/events and session summary `upload_status` placeholder.
  - What this story changes: add session cloud target endpoints/schemas/events and update session summary schema/status descriptions.

### Architecture Guardrails

- Go service owns credential handling, cloud target resolution, filesystem/local-data persistence, GCS client creation, activity logs, and SSE publication.
- Browser/Next.js must never receive service account JSON, private key, OAuth token, ADC token, Supabase service role, raw credential file contents, or raw credential file path.
- `internal/cloud` remains config/credential/target-rule helper package; create `internal/upload` or similar for session target/upload domain to avoid coupling config with queue state.
- API JSON/status fields use `snake_case`; REST success uses `{data}`; REST errors use `{error:{code,message,action,details}}`; SSE event names use dot notation.
- GCS has flat object namespace. "Folder" means prefix (and optional marker object) rather than Google Drive folder resource.
- Cloud target status is not local save/processing status. Cloud failure must not mark local delivery failed and must not block new capture/session routing.
- Existing local pipeline remains authoritative and safe: station/session/photo/quarantine/processing recovery behavior from Epics 1-5 and Story 6.1 credential safety must continue unchanged.
- Remote identity must be deterministic and stable across retry/restart; same session + same cloud settings should not generate multiple prefixes.

### Session-Level Prefix Convention

Use this session-level prefix, derived from Story 6.1 final object naming template:

```text
{target_root_prefix}/{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}
```

Rules:

- `target_root_prefix` optional and sanitized; if empty, prefix starts with date segment and never `/`.
- Date must come from deterministic session timestamp (`StartedAt` unless implementation explicitly selects local completion timestamp and persists it before use).
- `safe_customer_name` and `safe_order_number` use existing deterministic sanitizer; fallback only for previews/tests, not to hide invalid real session data unless product explicitly allows it.
- `station_id` and `session_id` are backend IDs from session record.
- Future Story 6.3 appends `/{asset_kind}/{safe_file_name}` for each uploaded original/graded JPG.
- Reject unsafe prefixes instead of silently truncating identity-bearing segments.

### Previous Story Intelligence

- Story 6.1 implemented server-side GCS settings, safe public DTOs, object key sanitization, fake checker tests, authenticated/trusted-origin endpoints, safe `cloud.status_updated`, activity logging, and OpenAPI docs.
- Story 6.1 review fixes are mandatory pattern: connection check no longer returns success or publishes SSE/activity when persisting checked status fails; settings update no longer ignores load/corrupt-state errors. Apply the same persistence safety to Story 6.2.
- Story 5.5 completed startup processing recovery and safe `processing.recovered` events/activity; do not merge cloud target recovery into processing recovery.
- Story 5.4 created processing retry runner/guard. Upload retry gets its own future upload guard; Story 6.2 should not reuse `ProcessingGuard` for cloud target state unless intentionally abstracted.
- Story 5.3 established processing queue/status UI and OpenAPI patterns. Mirror operator-actionable status patterns but keep cloud target status separate from local processing queue.
- Epics 1-4 established auth, activity logs, station settings/readiness, sessions, ingestion, quarantine, and dashboard patterns. Reuse existing mux/UI/API conventions.
- Git history is sparse (`Add from-scratch setup guide`, `Initial Selfstudio camera capture spike`); current workspace files and story artifacts are authoritative.

### Latest Technical Notes

- Google Cloud Storage Go client latest docs observed `cloud.google.com/go/storage` v1.62.1; current repo has `cloud.google.com/go/storage v1.61.3` indirect from Story 6.1. Do not upgrade dependencies unless necessary; if upgrading, run full Go tests and note reason.
- GCS object names are flat strings; `/` simulates hierarchy. Google docs allow many characters but object names cannot contain CR/LF and problematic names can leak information or break tooling. This project intentionally applies stricter sanitizer to avoid traversal-like `./`/`../`, control chars, backslash, and leading slash.
- Bucket/object names can leak customer/order information through request behavior. Only include customer/order because product requires findable fulfillment; do not log full customer names beyond operator-facing activity/status that is necessary.
- No new frontend cloud/GCS dependency is needed. Keep Google Cloud client libraries in Go agent only.

### Testing Requirements

Run at minimum:

- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build` if dependencies/environment allow.

Required coverage:

- Deterministic prefix generation from session/config; retry/restart produces same `object_prefix`.
- Sanitization rejects/normalizes unsafe customer/order/root/session/file-like inputs consistently with Story 6.1.
- Default target state for session with no resolved target returns safe `not_configured`/`pending` status and no secret fields.
- Cloud unconfigured/unauthorized/invalid target returns actionable API error and safe persisted failed status if appropriate.
- Success resolve persists remote identity and returns existing identity on repeated calls without duplicate records.
- Persistence save/load failure returns safe error and does not publish success SSE/activity.
- API endpoints require auth; resolve mutation requires trusted origin.
- SSE/activity payloads exclude credentials, raw local full source paths, raw GCS errors, and service account details.
- Existing processing/session/quarantine/cloud settings tests still pass.

### Regression Risks To Avoid

- Do not upload original/graded JPGs in this story.
- Do not create duplicate cloud target records for same session on retry/restart.
- Do not generate prefixes using current time for real sessions; that breaks idempotency.
- Do not expose credentials through API responses, OpenAPI examples, frontend state, SSE, activity logs, console logs, or errors.
- Do not store cloud target state in frontend/localStorage/cookies.
- Do not block local capture/session start/local processing solely because cloud target resolution failed.
- Do not overwrite or reinterpret existing `cloud_config.json` credential config.
- Do not add Google Drive API; architecture-selected provider is GCS.
- Do not mark session `uploaded` or create per-file upload status; that is Story 6.3+.
- Do not hide local completion gap by lying in status. If local_complete is not represented, expose a pending/waiting status and document exactly what remains for Story 6.3.

## Project Structure Notes

Expected new files:

- `apps/agent/internal/upload/target.go`
- `apps/agent/internal/upload/persistence.go`
- `apps/agent/internal/upload/prefix.go`
- `apps/agent/internal/upload/resolver.go`
- `apps/agent/internal/upload/*_test.go`
- `apps/agent/internal/api/cloud_targets.go`
- `apps/agent/internal/api/cloud_targets_test.go`
- Optional frontend: `apps/web/src/features/cloud/session-cloud-target.tsx` or integration in existing session detail/card component.

Expected modified files:

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/health.go` or mux wiring file if split later
- `apps/agent/internal/api/sessions.go`
- `apps/web/src/lib/api/client.ts`
- Session dashboard/detail components under `apps/web/src/features/sessions/` if UI status/action is added
- `apps/web/src/features/health/health-dashboard.tsx` for SSE invalidation if new event is used
- `docs/api/openapi.yaml`

Runtime target state remains under `local-data/state` and must not be committed.

## References

- `_bmad-output/planning-artifacts/epics.md` — Epic 6 and Story 6.2 acceptance criteria; FR57-FR63 mapping.
- `_bmad-output/planning-artifacts/prd.md` — Google Drive Fulfillment requirements, local-vs-cloud status separation, cloud failure/retry NFRs.
- `_bmad-output/planning-artifacts/architecture.md` — GCS selected over Drive API, Go service credential boundary, upload package location, API/SSE/error patterns, object naming gap.
- `_bmad-output/implementation-artifacts/6-1-configure-cloud-storage-credentials-and-target-rules.md` — previous story, cloud config/object key sanitizer/safe persistence/SSE/activity patterns.
- `apps/agent/internal/cloud/config.go` — cloud settings/status/action constants and safe public DTO.
- `apps/agent/internal/cloud/object_keys.go` — sanitizer and object key builder to reuse.
- `apps/agent/internal/api/cloud_settings.go` — API/auth/trusted-origin/SSE/activity safety pattern.
- `apps/agent/internal/sessions/store.go` — session model and current active/locked status limitation.
- `apps/agent/internal/api/sessions.go` — session detail summary and current `UploadStatus` placeholder.
- `apps/agent/internal/photos/store.go` — local original/graded status fields; do not add per-file cloud upload here yet unless explicitly needed.
- `apps/agent/cmd/selfstudio-agent/main.go` — current service wiring.
- `apps/web/src/features/cloud/cloud-settings.tsx` — existing cloud settings UI and object naming display.
- `apps/web/src/features/health/health-dashboard.tsx` — SSE subscription/invalidation pattern.
- `docs/api/openapi.yaml` — API/SSE contract source.
- Google Cloud docs: Cloud Storage Go client latest observed v1.62.1; object names are flat namespace and project applies stricter naming safety.

## Dev Agent Record

### Agent Model Used

GPT-5.1 Codex

### Debug Log References

- 2026-05-19: Implemented `internal/upload` domain with prefix-only GCS session target identity; no original/graded JPG upload and no per-file upload jobs were added.
- 2026-05-19: First `go test ./...` exposed a prefix sanitizer expectation mismatch; test was corrected to Story 6.1 `SafeSegment` behavior and rerun successfully.
- 2026-05-19: One Windows test execution reported transient `upload.test.exe: Access is denied`; rerunning `go test ./internal/upload -count=1` and then full `go test ./...` passed.
- 2026-05-19: Final validations passed: `cd apps/agent && go test ./...`, `cd apps/web && npm run typecheck`, `cd apps/web && npm run build`.
- 2026-05-19: Review follow-up HIGH fixed by allowing `LiveStationCards` to select a station's active session first, otherwise its locked session, so completed/locked sessions remain visible for cloud target status and Resolve/Retry actions while active sessions keep only End Session action. Validations passed: `cd apps/web && npm run typecheck`, `cd apps/web && npm run build`.

### Completion Notes List

- Ultimate context engine analysis completed - comprehensive developer guide created.
- Created session-level cloud target model/persistence under `apps/agent/internal/upload`, stored at `SELFSTUDIO_LOCAL_DATA_DIR/state/upload_targets.json` with versioned atomic JSON save/load and one active record per `session_id`.
- Implemented deterministic prefix-only GCS remote identity using session `StartedAt`, cloud settings bucket/root, and Story 6.1 sanitizers; selected prefix-only resolution (no marker object), documented in code/OpenAPI.
- Added cloud target resolver with safe failure states/actions for not configured, unauthorized, invalid target rules, prefix resolution failure, target save failure, and pending local completion; persistence failure is not reported as success.
- Added authenticated/trusted-origin API endpoints `GET /api/sessions/{session_id}/cloud-target` and `POST /api/sessions/{session_id}/cloud-target/resolve` with `{data}` responses and standard `{error}` responses.
- Wired runtime store/resolver in Go agent startup and session summaries so upload/cloud status is separate from local save/processing status.
- Updated dashboard session cards with clear cloud target label and Resolve/Retry action, plus SSE invalidation for `cloud.target_resolved` and `cloud.target_failed` without invalidating processing queue.
- Updated OpenAPI docs with session cloud target endpoints, schemas, prefix convention, safe SSE events, statuses, and Story 6.2 no-JPG-upload scope.
- ✅ Resolved review finding [HIGH]: Dashboard now keeps locked/completed station sessions reachable after End Session, shows their separate cloud target status, and enables Resolve/Retry only for locked sessions; backend locked-session gating and local/cloud status separation remain unchanged.

### File List

- apps/agent/cmd/selfstudio-agent/main.go
- apps/agent/internal/api/cloud_targets.go
- apps/agent/internal/api/cloud_targets_test.go
- apps/agent/internal/api/health.go
- apps/agent/internal/api/sessions.go
- apps/agent/internal/upload/persistence.go
- apps/agent/internal/upload/persistence_test.go
- apps/agent/internal/upload/prefix.go
- apps/agent/internal/upload/prefix_test.go
- apps/agent/internal/upload/resolver.go
- apps/agent/internal/upload/resolver_test.go
- apps/agent/internal/upload/target.go
- apps/web/src/features/health/health-dashboard.tsx
- apps/web/src/features/sessions/live-station-cards.tsx
- apps/web/src/features/sessions/use-resolve-cloud-target-mutation.ts
- apps/web/src/lib/api/client.ts
- docs/api/openapi.yaml
- _bmad-output/implementation-artifacts/6-2-create-cloud-folder-object-structure-per-session.md
- _bmad-output/implementation-artifacts/sprint-status.yaml

## Change Log

- 2026-05-19: Ultimate context engine analysis completed - comprehensive developer guide created.
- 2026-05-19: Implemented Story 6.2 session cloud target prefix planning/resolution, persistence, API/UI integration, OpenAPI documentation, and validations; status moved to review.
- 2026-05-19: Addressed code review HIGH finding for locked-session cloud target reachability in dashboard; status moved to review.
