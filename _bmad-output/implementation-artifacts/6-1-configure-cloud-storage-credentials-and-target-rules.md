# Story 6.1: Configure Cloud Storage Credentials and Target Rules

Status: done

## Story

Sebagai admin, saya ingin mengonfigurasi satu akun cloud storage dan aturan target upload, sehingga asset session yang sudah selesai secara lokal dapat di-upload ke cloud setelah local delivery tanpa mengekspos credential ke browser dan tanpa mencampur status cloud dengan status local save.

## Acceptance Criteria

1. Given admin membuka cloud settings, when UI memuat konfigurasi, then UI menampilkan status cloud storage, bucket/target root, aturan object prefix yang tervalidasi, dan status connection check terpisah dari local save status.
2. Given admin mengisi konfigurasi GCS, when admin menyimpan credential dan target rules, then Go service menyimpan credential server-side only di lokasi runtime lokal yang tidak dikirim ke browser/API response/log/activity/SSE.
3. Given browser melakukan request cloud settings, when response dikirim, then response hanya berisi metadata aman seperti `provider`, `bucket_name`, `target_root_prefix`, `object_naming_template`, `credentials_configured`, `connection_status`, `last_checked_at`, dan `last_error` yang sudah disanitasi; tidak pernah berisi service account JSON, private key, token, atau path credential mentah.
4. Given admin menjalankan connection check, when credential dan bucket valid, then service memverifikasi authorization ke GCS dan akses bucket/prefix, menyimpan `authorized` status, mengirim SSE `cloud.status_updated`, dan mencatat activity log aman.
5. Given connection check gagal karena credential invalid, bucket tidak ada/tidak authorized, network error, atau target rule invalid, when service merespons, then API memakai `{error:{code,message,action,details}}` dengan action spesifik seperti `FIX_CLOUD_CREDENTIALS`, `FIX_CLOUD_BUCKET`, `RETRY_CLOUD_CHECK`, atau `FIX_CLOUD_TARGET_RULES`, tanpa membocorkan secret.
6. Given admin menyimpan target root/folder rules, when service memvalidasi GCS object naming convention, then aturan menghasilkan object key deterministik dan aman untuk future upload dengan pola terdokumentasi: `{target_root_prefix}/{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}/{asset_kind}/{safe_file_name}`.
7. Given customer/order/file name mengandung karakter tidak aman, path traversal, empty segment, atau Unicode/space, when preview object key dibuat, then service melakukan sanitasi deterministik, menolak `./`, `../`, leading slash, control characters, empty final object name, dan object name yang terlalu panjang.
8. Given cloud config tersimpan, when health/dashboard/session detail membutuhkan status, then cloud status muncul sebagai status terpisah dari local save/processing status dan tidak mengubah session menjadi uploaded atau upload_pending; upload lifecycle baru diimplementasikan pada Story 6.2+.
9. Given existing station/session/processing/readiness/activity flows berjalan, then story ini tidak meregresi local capture, routing, original save, LUT processing, processing queue, quarantine, session summary, atau startup processing recovery.
10. Tests/build pass untuk Go agent, web typecheck/build yang relevan, dan OpenAPI/API contract diperbarui.

## Tasks / Subtasks

- [x] Definisikan scope dan boundary cloud config Epic 6. (AC: 1, 8, 9)
  - [x] Story ini hanya membuat konfigurasi credential, target rules, validation, connection status, API/UI settings, health/status surfacing, dan dokumentasi object naming.
  - [x] Jangan implement upload queue, create remote per-session prefix/folder, per-file upload, retry upload, dedup upload, atau upload recovery; itu milik Story 6.2-6.5.
  - [x] Gunakan istilah cloud/GCS sesuai architecture; PRD menyebut Google Drive tetapi architecture sudah memutuskan Google Cloud Storage sebagai Drive-equivalent storage flow.
- [x] Tambahkan domain dan persistence server-side untuk cloud settings. (AC: 2, 3, 8)
  - [x] Buat package baru yang disarankan: `apps/agent/internal/cloud` untuk domain config, sanitasi target rules, connection checker interface, dan persistence.
  - [x] Simpan config runtime di bawah `SELFSTUDIO_LOCAL_DATA_DIR/state/cloud_config.json` atau lokasi local-data setara; pastikan file tidak berada di `apps/web/public` dan tidak dikomit.
  - [x] Pisahkan secret material dari safe public metadata. Jika menyimpan service account JSON/file path untuk MVP, field tersebut hanya boleh dibaca oleh Go service dan tidak pernah masuk response/event/activity.
  - [x] Gunakan atomic write pattern seperti persistence existing (`photos`, `stations`, `sessions`) dan validasi backward-compatible saat file belum ada.
  - [x] Pertimbangkan file permission best effort di Windows; minimal jangan log path/isi secret dan dokumentasikan bahwa credential tetap server-side di admin PC.
- [x] Implement validasi credential input dan target rules. (AC: 2, 5, 6, 7)
  - [x] Dukung satu provider awal: `gcs`.
  - [x] Validasi `bucket_name` non-empty dan mengikuti batas aman GCS bucket name; jangan hardcode credential di frontend.
  - [x] Validasi `target_root_prefix` opsional tetapi jika ada harus disanitasi: trim slash, reject `./`, `../`, backslash path traversal, control chars, dan segment kosong.
  - [x] Implement helper object key preview untuk future uploads dengan template final: `{target_root_prefix}/{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}/{asset_kind}/{safe_file_name}`.
  - [x] Implement sanitasi segment deterministik: lower/trim, space menjadi `-`, buang/control unsafe chars, collapse repeated separators, fallback safe token saat customer/order kosong, dan preserve extension JPG secara aman.
  - [x] Tambahkan tests untuk path traversal, Unicode/space, Windows backslash, empty segments, collision-ish normalization, very long names, dan object key tidak diawali slash.
- [x] Implement GCS connection checker di Go service. (AC: 4, 5, 10)
  - [x] Tambahkan dependency resmi `cloud.google.com/go/storage` hanya di `apps/agent`; jangan menambahkan Google credential package ke `apps/web`.
  - [x] Gunakan Application Default Credentials atau service account JSON server-side sesuai input config; browser tidak menerima credential.
  - [x] Connection check minimal harus membuat client, memverifikasi bucket metadata/access, dan jika prefix diperlukan memverifikasi kemampuan yang relevan tanpa membuat object customer nyata kecuali memakai probe aman yang langsung dihapus atau opsi dry-run yang terdokumentasi.
  - [x] Map error ke safe status: `not_configured`, `checking`, `authorized`, `failed`; simpan `last_checked_at` ISO UTC dan `last_error` aman.
  - [x] Buat interface/fake checker agar tests tidak butuh network/GCS nyata.
- [x] Tambahkan REST API dan wiring auth/trusted origin. (AC: 1, 2, 3, 4, 5, 8)
  - [x] Tambah handler yang disarankan: `apps/agent/internal/api/cloud_settings.go`.
  - [x] Endpoint minimal: `GET /api/cloud/settings`, `PUT /api/cloud/settings`, `POST /api/cloud/settings/check`, dan opsional `POST /api/cloud/settings/object-key-preview` jika preview dipisah dari GET/PUT.
  - [x] Semua endpoint wajib `RequireAuth`; mutations/check wajib `RequireTrustedOrigin`.
  - [x] Success response wajib `{data}` wrapper dengan `snake_case` fields; error wajib `{error:{code,message,action,details}}`.
  - [x] Register routes di mux tanpa merusak chain existing `NewMuxWith...`; pertimbangkan menambah `NewMuxWithCloudSettings` agar testable.
  - [x] Pastikan tests membuktikan secret tidak muncul di JSON response, logs, activity, atau SSE payload.
- [x] Tambahkan SSE dan activity log aman untuk cloud config/status. (AC: 4, 5, 8)
  - [x] Publish `cloud.status_updated` saat settings disimpan dan saat connection check selesai.
  - [x] Payload hanya berisi metadata aman: provider, bucket configured/name jika dianggap aman, target root, `credentials_configured`, connection status, `last_checked_at`, safe error code/action.
  - [x] Record activity seperti `cloud.settings_updated` dan `cloud.connection_checked` dengan result/status saja; jangan catat private key, service account email jika sensitif, token, credential path mentah, atau raw GCS error yang mengandung secret.
  - [x] Jangan publish event upload per-file/session di story ini.
- [x] Update frontend cloud settings UI. (AC: 1, 3, 4, 5, 8)
  - [x] Buat feature disarankan: `apps/web/src/features/cloud/cloud-settings.tsx` dan API client methods di `apps/web/src/lib/api/client.ts` jika pola existing memakai client helper.
  - [x] Tambahkan akses dari dashboard/settings area existing tanpa merombak layout besar; jika belum ada navigation khusus, tampilkan panel cloud settings di health/dashboard/settings page yang paling sesuai dengan pola current UI.
  - [x] Form harus menerima bucket/target root/rule dan credential input dengan warning bahwa credential disimpan server-side only; setelah save, field secret harus dikosongkan/masked dan tidak re-render secret.
  - [x] Tampilkan connection status dengan label teks: `NOT CONFIGURED`, `AUTHORIZED`, `FAILED`, `CHECKING`, serta action button `Check Connection`.
  - [x] Tampilkan object key convention dan preview sanitized key agar admin paham struktur cloud target.
  - [x] Pastikan cloud status terpisah dari local save/processing status pada dashboard/session detail jika area status sudah ada.
- [x] Update OpenAPI dan docs contract. (AC: 3, 4, 5, 6, 10)
  - [x] Tambahkan schemas untuk CloudSettings, update request, connection check response, object key preview, dan SSE `cloud.status_updated`.
  - [x] Dokumentasikan bahwa credential fields write-only dan tidak pernah dikembalikan.
  - [x] Dokumentasikan object naming convention final dan sanitasi segment.
  - [x] Dokumentasikan error codes/actions cloud config: `CLOUD_NOT_CONFIGURED`, `CLOUD_CREDENTIALS_INVALID`, `CLOUD_BUCKET_UNAUTHORIZED`, `CLOUD_TARGET_RULE_INVALID`, `CLOUD_CHECK_FAILED`.
- [x] Tambahkan tests dan validasi. (AC: 1-10)
  - [x] Go unit tests untuk cloud config persistence atomic/default/migration/no-secret public DTO.
  - [x] Go unit tests untuk target rule/object key sanitization dan rejection cases.
  - [x] Go API tests untuk auth/trusted origin, safe responses, PUT validation, connection check success/failure dengan fake checker, safe activity, dan SSE event payload.
  - [x] Web typecheck/build untuk cloud settings panel dan API types.
  - [x] Run `cd apps/agent && go test ./...`.
  - [x] Run `cd apps/web && npm run typecheck`.
  - [x] Run `cd apps/web && npm run build` jika environment/dependencies memungkinkan.

### Review Follow-ups (AI)

- [x] [High] Fail connection-check responses when persisting checked status fails: `apps/agent/internal/api/cloud_settings.go` ignores `h.Store.Save(s)` in `Check`, then publishes `cloud.status_updated` and can return 200 `authorized` even though `last_checked_at` / `authorized` / safe error state were not persisted. This violates AC4 persistence of authorized status and AC5 failure-state persistence; return a `{error:{code,message,action,details}}` save failure and avoid publishing misleading status when persistence fails.
- [x] [Medium] Do not discard load errors on settings update: `CloudSettingsHandler.Put` uses `current, _ := h.Store.LoadOrDefault()`. If the runtime config exists but is unreadable/corrupt, PUT silently starts from defaults and may overwrite prior credential/status state. This violates the persistence/atomic-backward-compatible guardrail; handle load errors with a safe `CLOUD_CONFIG_READ_FAILED` response before applying updates.
- [x] [Medium] Add focused tests for persistence failure paths and SSE/activity safety on failed persistence: current API tests cover happy persistence but not save/load failures, so the misleading authorized response/event above is not caught. Add tests asserting no success response and no `cloud.status_updated` when check-state persistence fails, plus PUT returns safe read failure on corrupt state.

### Review Validation

- 2026-05-19: `cd apps/agent && go test ./...` passed.
- 2026-05-19: `cd apps/web && npm run typecheck` passed.
- 2026-05-19: `cd apps/web && npm run build` passed.
- 2026-05-19: Review follow-up validation `cd apps/agent && go test ./...` passed after one transient Windows `Access is denied` retry.

## Dev Notes

### Source Requirements

- Epic 6 objective: post-session cloud fulfillment dengan GCS sesuai architecture, upload original+graded setelah local completion, per-file/per-session upload tracking, retry failure, duplicate prevention, dan upload tidak memblokir session baru.
- Story 6.1 dari epics: admin configure one cloud storage account and target rules; service stores credentials server-side only; browser never receives credentials; connection check reports authorized/failed; GCS object naming convention documented and validated; cloud status appears separately from local save status.
- FR56: Admin can connect one Google Drive admin account for cloud upload. Architecture resolves this as Google Cloud Storage provider for MVP cloud image assets.
- FR57-FR63 are future Epic 6 stories but shape this config: object path must support customer/order organization, original+graded uploads, upload status, retry, local preservation, non-blocking capture, and duplicate-safe upload identity.
- FR19/FR46/FR47/FR48: session summary/dashboard must keep local save status separate from cloud/upload status, expose queue/status, and activity logs.
- NFR4/NFR11/NFR16/NFR20/NFR21/NFR25/NFR26/NFR32: cloud upload must not degrade local pipeline, local save independent of cloud, upload retry duplicate-safe, auth uses authorized admin account, customer data protected, token/network/partial failures supported later, local-vs-cloud status separated.

### Current Code Context To Read Before Editing

- `apps/agent/internal/config/config.go`
  - Current state: loads `SELFSTUDIO_AGENT_HOST`, `SELFSTUDIO_AGENT_PORT`, `SELFSTUDIO_LOCAL_DATA_DIR`, and required `SELFSTUDIO_AUTH_PIN` only.
  - What this story changes: may add cloud-related env defaults only if needed, but prefer runtime settings via API/persistence for admin-configurable cloud settings.
  - Must preserve: missing/placeholder PIN startup failure and no secret exposure in errors.
- `apps/agent/internal/api/health.go`
  - Current state: central mux chain registers auth, activity, stations, readiness, sessions, ingestion, processing queue, photo retry, quarantine, and events. `HealthData` currently has service/database/worker/disk only.
  - What this story changes: add cloud settings routes and optionally cloud health component/status.
  - Must preserve: auth/trusted-origin wrappers and existing route behavior.
- `apps/agent/internal/api/response.go`
  - Current state: response helpers enforce `{data}` and `{error}` patterns.
  - What this story changes: reuse these helpers; do not hand-roll inconsistent JSON.
- `apps/agent/internal/events/event.go` and `apps/agent/internal/events/broker.go`
  - Current state: SSE envelope supports dot-notation event names and safe `entity_type`/`entity_id`.
  - What this story changes: add/use `cloud.status_updated` with safe metadata only.
- `apps/agent/internal/activity/store.go` plus `apps/agent/internal/api/activity.go`
  - Current state: safe activity entries already used for operator actions and recovery; avoid unnecessary sensitive data.
  - What this story changes: add cloud settings/check activity entries with sanitized messages/details.
- `apps/agent/internal/stations/persistence.go`, `apps/agent/internal/photos/persistence.go`, `apps/agent/internal/sessions/persistence.go`
  - Current state: JSON persistence under local-data/state with validation and atomic save patterns.
  - What this story changes: mirror these patterns for cloud config; do not create ad-hoc persistence that can corrupt state.
- `apps/web/src/features/health/health-dashboard.tsx`
  - Current state: dashboard shell/status and SSE handling exist; Story 5.5 added processing recovery/queue invalidation behavior.
  - What this story changes: likely place to surface cloud status if no dedicated settings route exists.
  - Must preserve: health indicators text labels and existing SSE-driven refresh behavior.
- `apps/web/src/features/stations/station-settings.tsx`
  - Current state: established settings form patterns, validation display, save activity, and operator-friendly errors for station config.
  - What this story changes: use as UI pattern reference for cloud settings form; do not mix station config with credential secrets.
- `apps/web/src/lib/api/client.ts`
  - Current state: API client methods/types for auth, health, stations, readiness, sessions, ingestion, processing, quarantine likely exist here.
  - What this story changes: add cloud settings methods/types following existing fetch/error handling conventions.
- `docs/api/openapi.yaml`
  - Current state: source of API/SSE contract, includes `processing.recovered` and existing REST/SSE patterns.
  - What this story changes: add cloud endpoints/schemas/events and object naming docs.

### Architecture Guardrails

- Go service owns all credential handling, filesystem/local-data persistence, GCS client creation, connection checks, cloud state, future upload workers, activity logs, and SSE publication.
- Browser/Next.js must never receive service account JSON, private key, OAuth token, ADC token, Supabase service role, raw credential file contents, or credential file path if it could reveal machine-sensitive info.
- Do not put credential logic in `apps/web`, `NEXT_PUBLIC_*`, localStorage, cookies, or OpenAPI examples containing real-looking private keys.
- API JSON/status fields use `snake_case`; REST success uses `{data}`; REST errors use `{error:{code,message,action,details}}`; SSE event names use dot notation.
- Cloud status is not local save status. Story 6.1 must not mark sessions uploaded, start upload jobs, or block capture/processing because cloud is unconfigured/failed.
- Existing local pipeline remains authoritative and safe: station/session/photo/quarantine/processing recovery behavior from Epics 1-5 must continue unchanged.
- Object key generation must be deterministic and future-upload-friendly: same session/photo asset + same config should produce same target key; this will become part of duplicate-safe upload identity in Story 6.4.

### GCS Object Naming Convention To Implement/Document

Recommended final template for future upload objects:

```text
{target_root_prefix}/{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}/{asset_kind}/{safe_file_name}
```

Rules:

- `target_root_prefix` is optional and sanitized. If empty, object key starts at date segment; never with `/`.
- Date should come from session start/end/local completion timestamp when future upload stories integrate; for Story 6.1 preview can use current date or sample session date.
- `safe_customer_name` and `safe_order_number` are deterministic sanitized segments; fallback to `customer-unknown` / `order-unknown` if missing in preview.
- `station_id` and `session_id` are safe IDs from backend, not freeform display names.
- `asset_kind` should be `original` or `graded` for future Story 6.3.
- `safe_file_name` is sanitized basename only; reject path separators and control chars; preserve `.jpg`/`.jpeg` extension safely.
- Reject or normalize segments that produce `.` or `..`; never allow `./`, `../`, backslash traversal, leading slash, empty object name, or raw Windows paths.
- Keep object names reasonably below GCS limits; fail validation if generated key is too long rather than silently truncating identifiers needed for uniqueness.

### Previous Story Intelligence

- Story 5.5 completed startup processing recovery and added safe `processing.recovered` events/activity. Do not duplicate that recovery path or route cloud upload recovery into it.
- Story 5.5 review fixes moved recovery/enqueue after server startup and made events/activity safe IDs/counts only. Follow the same safety model for cloud connection check events.
- Story 5.4 created shared processing retry runner and `ProcessingGuard`; future upload retry should get its own upload guard later, but Story 6.1 should not create processing/upload retry mechanisms.
- Story 5.3 established processing queue/status UI and OpenAPI patterns. Cloud status should be similarly operator-actionable and text-labeled, but separate from local processing queue.
- Story 5.1/5.2 implemented original-first and graded output safety. Cloud config must not alter local output paths or process from source path.
- Epics 1-4 established auth, activity logs, station settings/readiness, sessions, ingestion, quarantine, and dashboard patterns. Reuse existing API/mux/UI patterns instead of reinventing a new settings architecture.
- Git history is sparse (`Add from-scratch setup guide`, `Initial Selfstudio camera capture spike`); current workspace code and story artifacts are authoritative.

### Latest Technical Notes

- Google Cloud official Go package currently observed as `cloud.google.com/go/storage` v1.61.3 in Google Cloud docs. Add it only to `apps/agent/go.mod` when implementing connection checks.
- Google client libraries support Application Default Credentials (ADC). For local admin PC MVP, service account JSON or ADC must be configured server-side; browser must never handle credentials.
- GCS object names are flat strings; `/` only simulates folders. Google docs recommend avoiding control characters and problematic relative path sequences like `./` and `../` in object names.
- Bucket/object names can leak information via request behavior; avoid embedding unnecessary sensitive data beyond customer/order requirement. Sanitized customer/order remains customer data and should not appear in logs beyond operationally necessary UI.
- No external web dependency is needed in frontend. Use existing React/Next/TanStack/shadcn patterns and checked-in `apps/web/package.json` versions.

### Testing Requirements

Run at minimum:

- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build` if dependencies/environment allow.

Required coverage:

- Cloud config default state when no config file exists returns `not_configured` and no secret fields.
- Saving config persists server-side and public DTO excludes service account JSON/private key/token/path.
- Invalid target rules reject traversal (`../`, `./`), leading slash, backslash, control chars, empty segments, and excessive length.
- Object key preview sanitizes customer/order/file names deterministically and preserves safe `.jpg` extension.
- Connection check success with fake checker sets `authorized`, `last_checked_at`, publishes `cloud.status_updated`, and records safe activity.
- Connection check failures map to actionable errors/status without leaking raw credential material.
- API endpoints require auth and mutations/check require trusted origin.
- Cloud settings UI masks/clears credential input after save and displays text status/actionable error.
- Existing processing/session/quarantine tests still pass.

### Regression Risks To Avoid

- Do not expose credential contents through GET settings, OpenAPI examples, frontend state, SSE payloads, activity logs, browser devtools-visible error details, or console logs.
- Do not store cloud credentials in `apps/web`, `public`, `NEXT_PUBLIC_*`, localStorage, or cookies.
- Do not start upload jobs in Story 6.1 or mutate session upload status beyond a separate cloud connectivity/config status.
- Do not block session start/local processing solely because cloud is unconfigured unless a future readiness story explicitly changes required checks; Story 2.6 only required cloud status known.
- Do not use Google Drive API if implementing architecture-selected GCS; keep names user-facing as cloud storage where appropriate.
- Do not overwrite existing mux routes or break auth/trusted origin protections while adding cloud routes.
- Do not log raw GCS errors if they include request/credential details; sanitize to code/action/operator message.
- Do not hardcode real bucket names, credentials, project IDs, or private keys in tests/docs.

## Project Structure Notes

Expected new files:

- `apps/agent/internal/cloud/config.go`
- `apps/agent/internal/cloud/persistence.go`
- `apps/agent/internal/cloud/object_keys.go`
- `apps/agent/internal/cloud/checker.go`
- `apps/agent/internal/cloud/*_test.go`
- `apps/agent/internal/api/cloud_settings.go`
- `apps/agent/internal/api/cloud_settings_test.go`
- `apps/web/src/features/cloud/cloud-settings.tsx`

Expected modified files:

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/health.go` or mux wiring file if split later
- `apps/agent/go.mod` / `apps/agent/go.sum` for `cloud.google.com/go/storage`
- `apps/web/src/lib/api/client.ts`
- `apps/web/src/features/health/health-dashboard.tsx` or app/dashboard route mounting cloud settings
- `docs/api/openapi.yaml`
- `.env.example` and/or `apps/agent/.env.example` only if documenting optional ADC/service account env behavior; never include real secrets.

Runtime cloud config/credential files remain under `local-data/state` or admin PC credential path and must not be committed.

## References

- `_bmad-output/planning-artifacts/epics.md` — Epic 6 and Story 6.1 acceptance criteria; FR56-FR63 mapping.
- `_bmad-output/planning-artifacts/prd.md` — Google Drive Fulfillment requirements, local-vs-cloud status separation, cloud failure/retry NFRs.
- `_bmad-output/planning-artifacts/architecture.md` — GCS selected over Drive API, Go service credential boundary, upload package location, API/SSE/error patterns, object naming gap.
- `_bmad-output/implementation-artifacts/5-5-recover-pending-processing-jobs-after-restart.md` — immediate previous story, safe recovery events/activity and non-regression constraints.
- `_bmad-output/implementation-artifacts/5-4-retry-failed-photo-processing.md` — retry/actionable error patterns to mirror later for upload.
- `_bmad-output/implementation-artifacts/5-3-track-processing-queue-and-photo-status.md` — queue/status UI and OpenAPI patterns.
- `apps/agent/internal/config/config.go` — current runtime config loading.
- `apps/agent/internal/api/health.go` — mux routing and health component pattern.
- `apps/agent/internal/events/event.go` — SSE envelope pattern.
- `apps/agent/internal/activity/store.go` — safe operator activity model.
- `apps/agent/internal/stations/persistence.go` / `apps/agent/internal/photos/persistence.go` — atomic JSON persistence patterns.
- `apps/web/src/features/stations/station-settings.tsx` — settings form reference.
- `apps/web/src/features/health/health-dashboard.tsx` — dashboard/status mounting point.
- `docs/api/openapi.yaml` — API/SSE contract source.
- Google Cloud docs: Cloud Storage Go client `cloud.google.com/go/storage` observed latest v1.61.3; ADC and service account auth remain server-side.

## Dev Agent Record

### Agent Model Used

OpenAI GPT-5 via API developer subagent.

### Debug Log References

- 2026-05-19: Implemented cloud domain/persistence/object key validation with fake-checker tests before real GCS checker wiring.
- 2026-05-19: Added REST endpoints, mux wiring, safe SSE/activity publishing, and fake checker API tests.
- 2026-05-19: Added dashboard Cloud Settings panel and API client methods; credential textarea is write-only and cleared after save.
- 2026-05-19: First `go test ./...` failed due Windows `Access is denied` on generated test binaries and one trusted-origin test expectation; fixed test and reran successfully.
- 2026-05-19: Validations passed: `cd apps/agent && go test ./...`, `cd apps/web && npm run typecheck`, `cd apps/web && npm run build`.
- 2026-05-19: Addressed review follow-ups for cloud persistence failure handling: `Check` now returns safe `CLOUD_CONFIG_SAVE_FAILED` without SSE/activity when checked status cannot persist; `Put` now returns safe `CLOUD_CONFIG_READ_FAILED` when existing state cannot be loaded.
- 2026-05-19: Added focused API tests for check save failure, no misleading `cloud.status_updated`/activity on failed persistence, and PUT load/corrupt state safe read failure. First full Go test run hit transient Windows test binary `Access is denied`; targeted tests and rerun of `cd apps/agent && go test ./...` passed.

### Completion Notes List

- Ultimate context engine analysis completed - comprehensive developer guide created.
- Implemented Story 6.1 cloud config boundary only: no upload queue, per-file upload, remote folder creation, retry, dedup, or upload lifecycle mutations were added.
- Added server-side `internal/cloud` package for GCS settings, atomic local persistence under `local-data/state/cloud_config.json`, write-only credential storage, safe public DTOs, target root validation, deterministic object key preview, and GCS/fake connection checkers.
- Added authenticated/trusted-origin cloud settings REST endpoints with `{data}` and `{error:{code,message,action,details}}` responses, safe `cloud.status_updated` SSE payloads, and safe activity entries.
- Added dashboard Cloud Settings UI showing separate cloud connection status, bucket/target root, object naming convention/preview, write-only credential input, and Check Connection action without exposing secrets.
- Updated OpenAPI/API documentation for cloud settings schemas, write-only credentials, SSE event, error codes/actions, and object naming sanitization.
- Tests cover default/not_configured state, no-secret public DTO, target rule rejection, object key sanitization/length, auth/trusted-origin, PUT validation, fake checker success/failure, safe activity, and SSE payload behavior.
- Required validations passed: Go agent tests, web typecheck, and web build.
- ✅ Resolved review finding [High]: connection check no longer returns authorized/success or publishes misleading SSE/activity if persisting checked status fails.
- ✅ Resolved review finding [Medium]: settings update no longer discards LoadOrDefault/read failures or overwrites defaults on unreadable/corrupt runtime state.
- ✅ Resolved review finding [Medium]: added focused persistence-failure tests for check save failure, PUT read failure, and no SSE/activity on failed persistence.

### File List

- apps/agent/cmd/selfstudio-agent/main.go
- apps/agent/go.mod
- apps/agent/go.sum
- apps/agent/internal/api/cloud_settings.go
- apps/agent/internal/api/cloud_settings_failure_test.go
- apps/agent/internal/api/cloud_settings_test.go
- apps/agent/internal/api/health.go
- apps/agent/internal/cloud/checker.go
- apps/agent/internal/cloud/cloud_test.go
- apps/agent/internal/cloud/config.go
- apps/agent/internal/cloud/object_keys.go
- apps/agent/internal/cloud/persistence.go
- apps/web/src/features/cloud/cloud-settings.tsx
- apps/web/src/features/health/health-dashboard.tsx
- apps/web/src/lib/api/client.ts
- docs/api/cloud-storage-story-6-1.md
- docs/api/openapi.yaml
- _bmad-output/implementation-artifacts/6-1-configure-cloud-storage-credentials-and-target-rules.md
- _bmad-output/implementation-artifacts/sprint-status.yaml

## Change Log

- 2026-05-19: Ultimate context engine analysis completed - comprehensive developer guide created.

- 2026-05-19: Implemented cloud storage credentials and target rules; status moved to review.
- 2026-05-19: Addressed code review findings - 3 items resolved; added safe persistence-failure handling and tests; status moved to review.
