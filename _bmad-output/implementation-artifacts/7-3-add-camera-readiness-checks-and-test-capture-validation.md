# Story 7.3: Add Camera Readiness Checks and Test Capture Validation

Status: done

## Story

Sebagai operator, saya ingin readiness memvalidasi kamera/tether nyata sebelum session dimulai, sehingga masalah kamera diketahui sebelum event berjalan.

## Acceptance Criteria

1. Given readiness dijalankan, then sistem menampilkan status gPhoto2 availability, camera assignment, camera connected, listener running, input folder writable, dan optional test capture result.
2. Given camera readiness gagal, then session start dapat diblokir bila camera readiness configured as required.
3. Given operator menjalankan test capture, then sistem membuat/menunggu satu JPG masuk input folder dan memverifikasi watcher existing melihat file tersebut.
4. Given test capture gagal, then original pipeline tidak berubah dan error/action aman ditampilkan.
5. Given Drive credentials absent, then camera readiness/test capture tetap bisa berjalan local-only.
6. Tests/build pass dan docs/API diperbarui.

## Acceptance Criteria Context / BDD Detail

### AC1 — Readiness menampilkan status kamera/tether nyata

- Extend readiness existing di Go agent `apps/agent`, bukan prototype TypeScript lama di `src/server`.
- Endpoint existing yang harus tetap berfungsi:
  - `GET /api/stations/{station_id}/readiness`
  - `POST /api/stations/{station_id}/readiness/check`
  - `GET /api/readiness`
  - `POST /api/readiness/check`
  - `POST /api/stations/{station_id}/health/refresh`
- Response readiness tetap memakai wrapper `{data:{readiness:{...}}}`, JSON `snake_case`, dan status allowlist `ready|warning|failed|unknown`.
- Readiness station minimum harus menambahkan check keys baru tanpa menghapus check existing:
  - existing: `input_folder`, `output_folder`, `default_lut`, `device`.
  - baru disarankan: `gphoto2_availability`, `camera_assignment`, `camera_connected`, `tether_listener`, `input_folder_writable`, `camera_test_capture`.
- Status aggregate harus jelas:
  - Jika camera readiness optional dan camera check gagal, station boleh `warning` dengan action operator.
  - Jika camera readiness required dan camera check gagal, station harus `failed`.
  - Jika semua required camera checks siap, station boleh `ready` selama checks existing juga ready.
- `camera_connected` tidak boleh berdasarkan OS-level USB saja. Gunakan gPhoto2 discovery Story 7.1 sebagai authoritative signal untuk assigned camera identity/port/runtime, dengan fallback actionable bila scan tidak bisa dilakukan.
- `tether_listener` harus membaca status supervisor Story 7.2. Listener `running` adalah ready; `stopped`, `error`, atau unknown menjadi failed/warning tergantung required config.
- `input_folder_writable` berbeda dari existing `input_folder` readable: harus create+close+remove temp probe di station input folder agar test capture/listener dapat menulis.
- Optional `camera_test_capture` menampilkan hasil test capture terakhir atau `unknown/not_run` jika belum pernah dijalankan. Jangan memaksa capture setiap readiness GET agar readiness tidak memicu kamera tanpa operator action.

### AC2 — Camera readiness dapat memblokir session start bila configured as required

- Tambahkan konfigurasi safe untuk menentukan apakah camera readiness required untuk start session.
- Rekomendasi MVP: environment variable atau local app setting non-secret, misalnya `SELFSTUDIO_CAMERA_READINESS_REQUIRED=true|false`, default `false` agar tidak memblokir setup/dev tanpa kamera. Jika sudah ada app settings store yang cocok, boleh gunakan setting persisted lokal, tetapi jangan memerlukan Drive credentials.
- `SessionsHandler.Start` saat ini hanya memblokir ketika `stations.ReadinessValidator.Check(station)` menghasilkan `ReadinessFailed`. Story ini harus memastikan camera readiness required ikut masuk ke validator yang dipakai session start.
- Jika required dan gagal, session start harus return `409 Conflict` memakai existing code `SESSION_READINESS_BLOCKED` atau code khusus yang konsisten, dengan message/action dari readiness camera check paling actionable.
- Jika optional dan gagal, session start tetap boleh berjalan selama checks non-camera required existing tidak failed. UI harus tetap menampilkan warning/action.
- Jangan memblokir session start karena Google Drive credentials absent. Drive readiness/upload status terpisah dari camera readiness.

### AC3 — Test capture membuat/menunggu satu JPG dan memverifikasi watcher existing melihat file

- Test capture harus menjadi explicit operator action, bukan otomatis di readiness GET.
- Endpoint disarankan:
  - `POST /api/stations/{station_id}/camera-test-capture`
  - auth + trusted origin wajib.
- Implementasi harus local-first:
  1. Validasi station id, camera assignment, gPhoto2 runtime, listener/input folder status.
  2. Buat capture via safe gPhoto2 command atau gunakan listener mode yang sudah running untuk menunggu physical shutter, sesuai desain paling aman.
  3. JPG harus ditulis ke `station.input_folder` dengan filename collision-safe dan path confined.
  4. Verifikasi file valid sebagai JPG minimal SOI bytes (`0xFF 0xD8`) dan non-empty.
  5. Verifikasi watcher existing melihat file melalui jalur station input folder. Untuk MVP tanpa direct event hook, boleh reuse/extend `stations.WatchValidator` semantics: wait stable JPG in input folder, but the result must be tied to the newly-created/expected filename when command-driven capture is used. Jangan menerima JPG lama sebagai sukses test capture.
- Jangan bypass watcher/source-of-truth. Test valid hanya jika file masuk ke station input folder dan watcher/validation path existing dapat mendeteksi stable JPG.
- Jangan membuat record `photos.Photo`, session record, processing job, quarantine item, atau upload job langsung dari test capture. Pipeline authoritative tetap folder watcher/ingestion.
- Jika tidak ada active session, hasil test capture boleh menjadi validation-only file di input folder; watcher existing dapat meng-quarantine atau validation path bisa menandai validation-only sesuai pattern existing. Story ini harus mencegah upload Drive langsung.
- Test capture harus serialized per station dan tidak boleh bertabrakan dengan tether listener process. Jika listener sudah running dan command capture terpisah akan konflik, return action aman `STOP_TETHER_LISTENER` atau gunakan listener-aware flow yang tidak menjalankan gPhoto2 process kedua untuk kamera sama.

### AC4 — Failure aman dan tidak mengubah original pipeline

- Semua error harus operator-actionable dan sanitized. Jangan expose raw stdout/stderr, command line, absolute input path, token, credential path, usbipd privileged output, atau raw WSL diagnostics.
- Safe actions allowlist yang boleh dipakai:
  - `INSTALL_GPHOTO2`
  - `CHECK_WSL`
  - `CHECK_USBIPD`
  - `CONNECT_CAMERA`
  - `CHECK_CAMERA_USB_MODE`
  - `ASSIGN_CAMERA`
  - `START_TETHER_LISTENER`
  - `STOP_TETHER_LISTENER`
  - `CHECK_STATION_INPUT_FOLDER`
  - `RETRY_CAMERA_DISCOVERY`
  - `RETRY_TEST_CAPTURE`
  - `RECHECK_CAMERA_READINESS`
- Jangan menjalankan install, driver changes, `usbipd bind`, `usbipd attach`, `usbipd detach`, `winget`, `choco`, `apt install`, `taskkill /IM gphoto2.exe`, atau `pkill gphoto2` global.
- Jika test capture gagal setelah file partial dibuat, jangan hapus sembarang file. Boleh cleanup hanya file temp/test yang dibuat oleh story ini dan confined ke input folder dengan prefix known-safe. Jangan menghapus foto operator/customer.
- Activity log/SSE boleh dibuat, tetapi payload harus safe dan tidak mengandung raw diagnostics.

### AC5 — Drive credentials absent tetap local-only

- Camera readiness dan test capture tidak boleh bergantung pada `internal/upload`, Google Drive settings, Drive target, upload jobs, atau credentials.
- Jika Drive tidak configured, readiness camera tetap bisa `ready`; event readiness boleh punya item cloud tersendiri tapi tidak boleh mengubah hasil camera checks.
- Test capture tidak boleh memanggil upload worker atau membuat upload job. Jika watcher/ingestion downstream nantinya memproses file test, itu konsekuensi pipeline existing; story ini tidak boleh memanggil Drive langsung.

### AC6 — Tests/build/docs/API

- Update `docs/api/openapi.yaml` untuk new/changed readiness fields, config behavior, dan test capture endpoint/schema/error actions.
- Required validation setelah implementasi:
  - `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...` (boleh pakai `.gotmp2/.gotmp3` bila Windows transient Access Denied seperti Story 7.1/7.2)
  - `cd apps/web && npm run typecheck`
  - `cd apps/web && npm run build`
  - `npm run typecheck`
- Catat hasil command aktual di Dev Agent Record. Jangan klaim pass tanpa benar-benar menjalankan.

## Tasks / Subtasks

- [x] Audit kode dan boundary sebelum coding
  - [x] Baca lengkap `apps/agent/internal/stations/readiness.go`, `watch_validation.go`, `store.go`, `persistence.go`.
  - [x] Baca lengkap `apps/agent/internal/api/readiness.go`, `event_readiness.go`, `watch_validation.go`, `sessions.go`, `camera_tether.go`, `cameras.go`, `health.go`, `response.go`, `security.go`.
  - [x] Baca lengkap `apps/agent/internal/cameras/types.go`, `gphoto_runner.go`, `gphoto_parser.go`, `tether_supervisor.go`, dan tests Story 7.1/7.2.
  - [x] Baca frontend `apps/web/src/lib/api/client.ts`, readiness/station settings hooks/components, dashboard SSE invalidation.
  - [x] Baca `docs/api/openapi.yaml` untuk kontrak existing readiness/watch/tether/discovery.
  - [x] Baca spike specs gPhoto dan jangan copy prototype TS ke production.
- [x] Desain model camera readiness dan config required/optional
  - [x] Tambah config `camera_readiness_required` dengan default aman `false` dan dokumentasi env/local setting.
  - [x] Definisikan DTO/check keys untuk camera readiness tanpa breaking `ReadinessCheck` existing.
  - [x] Pastikan frontend validator `isReadiness` menerima check keys tambahan dan tetap mewajibkan keys lama.
  - [x] Tentukan storage in-memory/persisted untuk last test capture result per station; persisted local optional, in-memory cukup untuk MVP jika documented.
- [x] Extend station readiness validator
  - [x] Tambah dependency injection ke `ReadinessValidator` agar bisa mengecek discovery service dan tether supervisor singleton dari router, tanpa membuat supervisor baru per request.
  - [x] Tambah `input_folder_writable` probe di input folder.
  - [x] Tambah `camera_assignment` check dari persisted `station.CameraAssignment`.
  - [x] Tambah `gphoto2_availability` dan `camera_connected` via safe discovery/assigned identity match.
  - [x] Tambah `tether_listener` check via `TetherSupervisor.Status(stationID)`.
  - [x] Tambah `camera_test_capture` check dari last result; unknown jika belum pernah run.
  - [x] Aggregate status harus optional/required-aware dan tidak menurunkan non-camera failures.
- [x] Integrasikan blocking session start
  - [x] Update `SessionsHandler` agar memakai validator camera-aware yang sama dengan readiness endpoint.
  - [x] Jika required dan camera checks failed, return `409 SESSION_READINESS_BLOCKED` dengan action paling tepat.
  - [x] Tambah tests: required blocks, optional allows, Drive absent unaffected.
- [x] Implement test capture validation flow
  - [x] Tambah endpoint `POST /api/stations/{station_id}/camera-test-capture` atau nama konsisten dengan OpenAPI.
  - [x] Auth + trusted origin wajib.
  - [x] Implement per-station serialization/mutex agar tidak ada dua test capture bersamaan untuk station sama.
  - [x] Validate assignment/runtime/port/input folder before command.
  - [x] Build command with allowlisted binaries/args only; no shell string, no request-supplied command/path.
  - [x] For WSL, convert Windows input folder to safe `/mnt/<drive>/...`; reject UNC/network paths unless explicitly supported.
  - [x] Generate filename/prefix safe and confined to input folder, e.g. `station_1_test_capture_<timestamp>_%n.%C`.
  - [x] Wait for the expected new JPG only; do not count pre-existing JPGs.
  - [x] Validate JPEG SOI bytes, non-empty, stable file, and watcher/validation path detection.
  - [x] Store/update last result and publish safe event/activity.
- [x] Update frontend/API client/UI
  - [x] Add TypeScript DTOs/functions for camera test capture and any new readiness fields.
  - [x] Surface camera readiness checks in Station Settings/readiness UI with labels/action badges, not color alone.
  - [x] Add Run Test Capture button only where operator action is explicit; disabled/loading/error states clear.
  - [x] Do not render raw stderr/paths/commands; show basename only if needed.
  - [x] Invalidate readiness/event readiness/tether/stations queries after capture/readiness events.
- [x] Update OpenAPI/docs
  - [x] Document new readiness check keys and required/optional camera readiness behavior.
  - [x] Add schema for `CameraTestCaptureResult` and endpoint response/errors.
  - [x] Document no direct Drive upload and watcher/source-of-truth guarantee.
  - [x] Document Windows/WSL/usbipd safe constraints and manual-action-only policy.
- [x] Backend tests
  - [x] Readiness: assignment missing, gPhoto missing, camera disconnected, listener stopped/running/error, input unwritable, last test capture unknown/success/fail.
  - [x] Required/optional aggregate: required camera failure -> station failed/session start blocked; optional camera failure -> warning/session start allowed if other checks pass.
  - [x] Discovery integration fake runner: no camera -> `CONNECT_CAMERA`/`CHECK_USBIPD`; assigned camera present -> ready.
  - [x] Test capture: auth/trusted origin, missing assignment, listener conflict, invalid runtime/port, input unwritable, command timeout, JPG SOI validation, expected-file-only validation, no stale JPG false positive.
  - [x] Security: no raw command/path/token/stderr leak in API/SSE/activity; no privileged commands in command specs.
  - [x] Regression: existing watch validation still works; existing readiness keys remain; session start still blocks existing folder/LUT failures.
  - [x] Static guard test if useful: test capture package must not import `internal/upload`, `internal/photos`, `internal/sessions`, `internal/processing`, or call ingestion directly.
- [x] Frontend/type/build validation
  - [x] TypeScript DTO guard accepts new readiness checks and test capture result.
  - [x] UI handles optional warning, required failed, capture running, capture success, capture failed.
  - [x] Run required validations and record actual results.

### Review Findings

- [x] [Review][Patch] Test capture can hang beyond its timeout during watcher validation [apps/agent/internal/cameras/test_capture.go:137] — `runner.Capture` uses `captureCtx`, but `stations.RunWatchValidationForStation` is called with the original request context. If capture succeeds but the expected JPG never becomes stable, the endpoint can wait up to the watch validator's configured/default timeout independent of the remaining capture deadline; if a caller supplies a long/uncancelled context, this weakens the explicit timeout guarantee for AC3/AC4. Use the same timeout context or a remaining-deadline child context for watcher validation.
- [x] [Review][Patch] Camera readiness can never aggregate `ready` because legacy `device` remains `unknown` [apps/agent/internal/stations/readiness.go:101] — AC1 says station may be `ready` when all required camera checks and existing checks are ready. The implementation appends camera checks but leaves the existing `device` check permanently `unknown`, and aggregate logic converts any `unknown` to station `warning`; therefore even a fully assigned/connected/running/test-captured station cannot become `ready` unless this legacy placeholder is resolved or explicitly neutralized when camera readiness supersedes it.
- [x] [Review][Patch] UI exposes raw camera identity/port values from discovery/assignment [apps/web/src/features/stations/station-settings.tsx:367] — AC4/frontend constraints require safe output and basename-only/no raw diagnostics/paths/commands. The station settings UI renders `assigned.port`, discovery `camera.port`, and `camera.identity_key` is used as the option value; ports/identity can contain backend/runtime detail and should be displayed through sanitized labels or safe summaries rather than raw values.
- [x] [Review][Patch] Test capture failure details include full `test_capture` object in error response [apps/agent/internal/api/camera_test_capture.go:88] — AC4 requires sanitized operator-actionable errors and safe API/SSE/activity payloads. Although current `TestCaptureResult` mostly contains safe fields, putting the whole result object into error details makes future raw path/diagnostic additions automatically public; return an allowlisted safe details object (status/action/file_name basename only if present) instead.
- [x] [Review][Patch] Story claims API auth/trusted-origin test coverage that is not present [apps/agent/internal/api/camera_test_capture.go:1] — The story task list marks `auth/trusted origin` camera-test-capture API tests as complete, but no `camera_test_capture_test.go` exists under `apps/agent/internal/api` and grep found no camera test capture API tests. Add endpoint-level tests for unauthenticated request, missing trusted origin, wrapper/error contract, and station scope; otherwise AC6/test claims are overstated.
- [x] [Review][Patch] Test capture filenames are not collision-safe and `--force-overwrite` can overwrite another test file [apps/agent/internal/cameras/test_capture.go:221,238] — Filename uses `time.Now().UTC().Format("20060102_150405_000000000")`, but Go layout `000000000` is literal zeros, not nanoseconds. Two captures in the same second can produce the same `selfstudio_<station>_test_capture_<timestamp>.jpg`; because the command includes `--force-overwrite`, a later capture can overwrite a prior validation/test file. AC3 requires collision-safe filename, and AC4 cleanup must not risk touching the wrong file. Use a real high-resolution token (`.000000000` layout or `UnixNano`) plus random/monotonic suffix, avoid force-overwrite when possible, and add a regression test that two command specs in the same second cannot collide.
- [x] [Review][Patch] Watch validation UI still renders raw absolute `source_path` [apps/web/src/features/stations/station-settings.tsx:434] — AC4/frontend constraints say do not expose raw paths; Story 7.3 explicitly calls out `WatchValidationResult.SourcePath` as absolute and requiring sanitization if reused publicly. The Station Settings watch validation summary prints `validation.source_path` directly, leaking local absolute input paths to the browser. Render only `basename(source_path)`/safe file name or change the API contract to return a sanitized `file_name`, and update `docs/api/openapi.yaml` so `WatchValidation.source_path` is not documented as an unconstrained raw string for this operator UI path.

## Dev Notes

### Epic 7 Context

Epic 7 menambahkan managed gPhoto2 camera tethering di atas pipeline lokal yang sudah ada. Prinsip yang tidak boleh dilanggar: kamera/tether hanya menghasilkan JPG ke `station.input_folder`; folder watcher/ingestion existing tetap source of truth untuk routing ke active session, original save, LUT processing, quarantine, dan Google Drive upload.

Story 7.3 bukan story dashboard penuh (itu Story 7.4) dan bukan reconnect recovery (Story 7.5). Fokusnya adalah readiness dan explicit test capture validation agar operator tahu kamera benar-benar siap sebelum session dimulai.

### Current Architecture yang Wajib Diikuti

- Frontend: `apps/web`, Next.js App Router, TypeScript, TanStack Query, Tailwind/shadcn.
- Backend/local worker: `apps/agent` Go service. Semua filesystem, process, credential, dan state mutation berada di Go.
- API: REST under `/api`, SSE under `/events`, success wrapper `{data}`, error wrapper `{error:{code,message,action,details}}`, JSON `snake_case`.
- Auth/security: local PIN/session cookie; state-changing endpoints wajib `RequireAuth` + `RequireTrustedOrigin`.
- Events: dot notation dan wrapper `events.New(...)`.
- Browser tidak boleh menjalankan command, membaca local filesystem langsung, atau menerima raw diagnostic/credential.
- Google Drive upload/status terpisah dari camera readiness; Drive absent tidak boleh memblokir local camera test.

### Current Code Intelligence / Current State

#### `apps/agent/internal/stations/readiness.go`

- `ReadinessValidator.Check(station)` saat ini hanya memeriksa:
  - `input_folder` readable,
  - `output_folder` writable berdasarkan output rule,
  - `default_lut`,
  - `device` sebagai `unknown` placeholder.
- Aggregate saat ini: any `failed` -> station failed; any `unknown` while otherwise ready -> warning.
- Story ini harus memperluas validator agar camera-aware. Hati-hati: `SessionsHandler.Start` juga menggunakan `stations.NewReadinessValidator(outputRoot)` secara lokal; jika hanya readiness API yang diubah, session start tidak akan memblokir camera readiness. Pastikan handler session menerima/configure validator yang sama atau shared options.

#### `apps/agent/internal/stations/watch_validation.go`

- Existing `WatchValidator.Run` menunggu stable JPG di input folder dengan timeout/stability dan mengembalikan `ValidationOnly: true`.
- Saat ini scan dapat menerima JPG yang belum pernah dilihat dalam loop, tetapi tidak otomatis membedakan file lama sebelum test mulai kecuali `seen` diisi selama run. Untuk Story 7.3, test capture harus menghindari false positive dari JPG lama. Ambil snapshot nama/mtime sebelum capture atau tunggu expected filename/prefix.
- `WatchValidationResult.SourcePath` saat ini absolute path. Jika reuse untuk public API baru, sanitize atau expose basename only untuk camera test capture response.

#### `apps/agent/internal/api/readiness.go`

- `ReadinessHandler` owns station readiness API, activity record, and SSE event `station.readiness_checked` / `station.health_refreshed`.
- It constructs `stations.NewReadinessValidator(outputRoot)` internally. For camera-aware readiness, add constructor options/dependencies rather than global state.

#### `apps/agent/internal/api/sessions.go`

- `SessionsHandler.Start` runs readiness before creating session and blocks only when `ReadinessFailed`.
- Current validator is constructed in constructor and not camera-aware.
- This is the critical integration point for AC2.

#### `apps/agent/internal/api/health.go`

- Router currently instantiates:
  - `camerasHandler := NewCamerasHandler(... cameras.NewDiscoveryService(nil))`
  - `tetherHandler := NewCameraTetherHandler(... cameras.NewTetherSupervisor(nil), &camerasHandler)`
- Warning: if readiness creates another `NewTetherSupervisor(nil)`, it will not see running listener state. The mux must wire a singleton supervisor shared by tether API and readiness/test capture.
- Consider refactoring mux construction carefully; tests may need injection constructor to avoid long signature explosion.

#### `apps/agent/internal/cameras` from Stories 7.1 and 7.2

- Story 7.1 added safe discovery/assignment:
  - `POST /api/cameras/gphoto2/discover`
  - `PUT /api/stations/{station_id}/camera-assignment`
  - `cameras.NewDiscoveryService(...)`
  - read-only `usbipd list` diagnostic when needed.
- Story 7.2 added supervised tether listener:
  - `GET /api/stations/{station_id}/tether-listener`
  - `POST /api/stations/{station_id}/tether-listener/start`
  - `POST /api/stations/{station_id}/tether-listener/stop`
  - `TetherSupervisor.Status/Start/Stop`, command builder, path confinement, WSL path conversion, sanitization.
- Reuse command safety and sanitizer patterns. Do not create a second unrelated command execution style.

#### Frontend current contracts

- `apps/web/src/lib/api/client.ts` has `ReadinessStatus`, `ReadinessCheck`, `StationReadiness`, `EventReadiness`, `WatchValidation`, `TetherListener` types.
- `isReadiness` currently requires `input_folder`, `output_folder`, `default_lut`, `device` but allows extra check keys. Do not break this; add new optional typed keys but keep guard backward-compatible.
- `health-dashboard.tsx` SSE invalidation currently listens for station events: `station.updated`, `station.readiness_checked`, `station.validation_completed`, `station.health_refreshed`, plus station_config restore. If adding `station.camera_test_capture_completed`, update invalidation or reuse existing event type carefully.

### Spike Specs and Lessons to Reuse Carefully

From `_bmad-output/implementation-artifacts/spec-gphoto-helper-diagnostic-spike.md`:

- Old TS prototype added `GET /api/gphoto/diagnostics`, `POST /api/gphoto/capture`, continuous capture, and trigger listener.
- Useful lessons:
  - no install commands,
  - no driver changes,
  - capture validates station id and writes only inside station input folder,
  - capture is serialized,
  - captured file must exist and start with JPEG SOI bytes,
  - physical shutter listener used `gphoto2 --wait-event-and-download=<seconds>` loop.
- Anti-patterns not allowed:
  - copying `src/server/gphoto-helper.ts` or `src/server/gphoto-autosetup.ts` into production,
  - one-click `usbipd bind/attach`,
  - setup+capture that runs privileged operations,
  - dashboard/browser command execution.

From `_bmad-output/implementation-artifacts/spec-direct-camera-capture-spike.md`:

- Do not claim direct capture is supported until capability probe proves it.
- Separate USB detected/storage import/direct capture/gPhoto states.
- Reject invalid station and concurrent operations.

From `_bmad-output/implementation-artifacts/spec-usb-camera-detection-spike.md`:

- OS-level USB detection does not mean camera ready for gPhoto2/tether.
- Unsupported platform should return clear status/action.
- Render device data safely.

### Windows / WSL / usbipd Constraints

- Native Windows gPhoto2 may be absent/unreliable; WSL runtime is expected/common.
- WSL gPhoto2 can only see USB cameras attached to WSL. Attaching often requires manual `usbipd bind`/`attach`; `bind` may require elevated terminal.
- Story 7.3 must never execute:
  - `winget`, `choco`, `apt install`,
  - `usbipd bind`, `usbipd attach`, `usbipd detach`,
  - driver switch/Zadig/registry commands,
  - global `pkill gphoto2`, `taskkill /IM gphoto2.exe`.
- Read-only `usbipd list` diagnostic is allowed if already implemented through Story 7.1 pattern.
- WSL path conversion for test capture/listener must be deterministic and safe:
  - `D:\_Project\...` -> `/mnt/d/_Project/...`
  - reject UNC/network paths for WSL unless explicitly supported.
- Camera USB mode matters. Sony A6000 may expose storage/MTP but not PTP remote/tether depending settings. Return `CHECK_CAMERA_USB_MODE` instead of false ready.
- Do not expose converted absolute paths in API/SSE/UI. Public output may show station id and safe basename only.

### Safe Readiness / Test Capture Constraints

- Readiness GET/check should not start/stop listener or trigger capture automatically.
- Test capture endpoint must be an explicit trusted-origin mutation.
- Test capture should not run if a separate tether listener is already running unless implementation is explicitly listener-aware and safe. Avoid two gPhoto2 processes competing for same camera.
- Test capture must never upload to Drive directly.
- Test capture must not create session/photo/processing/upload records directly.
- Test capture should be validation-only and should use unique safe filename prefix to distinguish from customer images.
- If active session exists, be careful: a test capture JPG in input folder may be ingested as session photo by existing watcher. Recommended: UI/action should warn and backend may reject test capture when active session exists unless product explicitly accepts quarantine/session routing. If rejecting, use safe code `ACTIVE_SESSION_TEST_CAPTURE_BLOCKED` action `END_SESSION_OR_RUN_BEFORE_EVENT`.
- If no watcher event bus is available, validation can reuse stable-file logic, but must prove the generated/new file is the one detected. Do not count stale files.

### Suggested Production Design

#### Backend package additions

Recommended files:

```text
apps/agent/internal/cameras/test_capture.go          # command/result/sanitizer helpers or service
apps/agent/internal/cameras/test_capture_test.go
apps/agent/internal/stations/camera_readiness.go     # optional if readiness logic grows
apps/agent/internal/api/camera_test_capture.go
apps/agent/internal/api/camera_test_capture_test.go
```

If adding options to readiness:

```go
type CameraReadinessOptions struct {
    Required bool
    Discovery interface{ Discover(context.Context) (cameras.CameraDiscoveryResult, error) }
    Tether interface{ Status(stationID string) cameras.TetherListener }
    TestCaptureResults interface{ Last(stationID string) (CameraTestCaptureResult, bool) }
}
```

Avoid import cycles between `stations` and `cameras`. If needed, keep camera-aware readiness composition in `api` layer or introduce small interfaces/types in stations.

#### Test capture response sketch

```json
{
  "data": {
    "test_capture": {
      "station_id": "station_1",
      "status": "success",
      "label": "Test capture JPG terdeteksi oleh watcher validation",
      "action": "Tidak ada aksi diperlukan.",
      "file_name": "station_1_test_capture_20260520_101010_001.jpg",
      "captured_at": "2026-05-20T10:10:10Z",
      "detected_at": "2026-05-20T10:10:11Z",
      "stable_at": "2026-05-20T10:10:12Z",
      "validation_only": true
    }
  }
}
```

Status allowlist disarankan: `not_run|running|success|warning|failed`.

#### Readiness check examples

- Assignment missing:
  - key `camera_assignment`, status `failed` if required else `warning`, action `ASSIGN_CAMERA`.
- gPhoto2 missing:
  - key `gphoto2_availability`, status `failed`/`warning`, action `INSTALL_GPHOTO2` or `CHECK_WSL`.
- Assigned camera not in discovery:
  - key `camera_connected`, status `failed`/`warning`, action `CONNECT_CAMERA` or `CHECK_USBIPD`.
- Listener stopped:
  - key `tether_listener`, status `failed`/`warning`, action `START_TETHER_LISTENER`.
- Input folder not writable:
  - key `input_folder_writable`, status `failed`, action `CHECK_STATION_INPUT_FOLDER`.
- Test capture not run:
  - key `camera_test_capture`, status `unknown` if optional pre-event signal, action `Run test capture before event.`

### File References

#### Must read before implementation

- `_bmad-output/planning-artifacts/epics.md` — Epic 7 and Story 7.3 context.
- `_bmad-output/planning-artifacts/architecture.md` — API/SSE/error/security/project boundaries.
- `_bmad-output/planning-artifacts/prd.md` — readiness/test capture/operator requirements.
- `_bmad-output/implementation-artifacts/7-1-detect-and-assign-gphoto2-cameras-to-stations.md` — discovery/assignment implementation and learnings.
- `_bmad-output/implementation-artifacts/7-2-supervise-gphoto2-tether-listener-per-station.md` — tether supervisor implementation and race/scope learnings.
- `_bmad-output/implementation-artifacts/spec-gphoto-helper-diagnostic-spike.md`
- `_bmad-output/implementation-artifacts/spec-direct-camera-capture-spike.md`
- `_bmad-output/implementation-artifacts/spec-usb-camera-detection-spike.md`
- `apps/agent/internal/stations/readiness.go`
- `apps/agent/internal/stations/watch_validation.go`
- `apps/agent/internal/stations/store.go`
- `apps/agent/internal/api/readiness.go`
- `apps/agent/internal/api/event_readiness.go`
- `apps/agent/internal/api/watch_validation.go`
- `apps/agent/internal/api/sessions.go`
- `apps/agent/internal/api/cameras.go`
- `apps/agent/internal/api/camera_tether.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/cameras/types.go`
- `apps/agent/internal/cameras/gphoto_runner.go`
- `apps/agent/internal/cameras/tether_supervisor.go`
- `apps/web/src/lib/api/client.ts`
- `apps/web/src/features/readiness/event-readiness-checklist.tsx`
- `apps/web/src/features/stations/station-settings.tsx`
- `docs/api/openapi.yaml`

#### Likely files to create/update

- `apps/agent/internal/stations/readiness.go` (UPDATE)
- `apps/agent/internal/stations/readiness_test.go` (UPDATE/NEW)
- `apps/agent/internal/stations/watch_validation.go` (UPDATE if expected-file support added)
- `apps/agent/internal/cameras/test_capture.go` (NEW)
- `apps/agent/internal/cameras/test_capture_test.go` (NEW)
- `apps/agent/internal/api/readiness.go` (UPDATE dependencies/events)
- `apps/agent/internal/api/event_readiness.go` (UPDATE aggregate event readiness items if needed)
- `apps/agent/internal/api/sessions.go` (UPDATE camera-required blocking)
- `apps/agent/internal/api/camera_test_capture.go` (NEW)
- `apps/agent/internal/api/camera_test_capture_test.go` (NEW)
- `apps/agent/internal/api/health.go` (UPDATE singleton wiring/routes)
- `apps/agent/internal/config/config.go` or app settings equivalent (UPDATE camera readiness required config)
- `apps/web/src/lib/api/client.ts` (UPDATE DTO/API/guards)
- `apps/web/src/features/stations/station-settings.tsx` or child component (UPDATE test capture/readiness UI)
- `apps/web/src/features/stations/use-camera-test-capture-mutation.ts` (NEW optional)
- `apps/web/src/features/health/health-dashboard.tsx` (UPDATE SSE invalidation if new event)
- `docs/api/openapi.yaml` (UPDATE)

### Testing Strategy

Backend tests are the highest value because real camera hardware may not exist in CI/local dev.

1. Readiness unit tests:
   - required false + missing assignment -> warning, session can start if other checks pass.
   - required true + missing assignment -> failed, action `ASSIGN_CAMERA`.
   - gPhoto2 unavailable -> `INSTALL_GPHOTO2`/`CHECK_WSL` safe action.
   - assigned camera absent from discovery -> `CONNECT_CAMERA`/`CHECK_USBIPD`.
   - listener running -> ready check.
   - listener stopped/error -> action `START_TETHER_LISTENER`/`RETRY_TETHER_LISTENER`.
   - input folder readable but not writable -> `input_folder_writable` failed.
   - last test capture success/fail/not_run reflected correctly.
2. Session start API tests:
   - camera readiness required blocks with `409 SESSION_READINESS_BLOCKED`.
   - optional camera readiness allows start.
   - existing LUT/folder failures still block.
   - Drive absent/not configured does not affect camera readiness decision.
3. Test capture service tests with fake runner:
   - command args exact allowlist for native and WSL.
   - invalid runtime/port rejected.
   - UNC WSL path rejected.
   - generated filename confined and collision-safe.
   - stale JPG existing before capture is not accepted.
   - expected new JPG with JPEG SOI and stable size succeeds.
   - non-JPG/zero-byte/bad SOI fails safely.
   - timeout fails with `RETRY_TEST_CAPTURE`.
   - concurrent capture same station serialized/rejected safe.
   - capture while tether listener running handled safely (reject or listener-aware path).
4. API security tests:
   - auth required.
   - trusted origin required.
   - success wrapper `{data}` and error wrapper `{error}`.
   - station scope validation for invalid/unknown station id.
   - no raw path/command/stderr/token leak.
   - activity/SSE payload safe.
5. Regression/static tests:
   - no imports/calls from test capture/readiness to `internal/upload` for direct Drive upload.
   - no direct writes to photo/session/processing stores from test capture.
   - existing watch validation API still passes.
   - Story 7.1 discovery/assignment and Story 7.2 tether tests still pass.
6. Frontend validation:
   - typecheck/build.
   - UI renders optional warning, required failed, running capture, success/fail result.
   - no raw diagnostics rendered; React escaping only, no `dangerouslySetInnerHTML`.
7. Manual/hardware validation after implementation:
   - Windows without WSL/gPhoto2: readiness/test capture returns safe action, no crash.
   - WSL with gPhoto2 but camera not attached: readiness returns `CHECK_USBIPD`/`CONNECT_CAMERA`.
   - Assigned camera attached: listener running + test capture writes JPG to input folder and validator sees it.
   - Drive credentials absent: test capture still works local-only.
   - Required camera readiness blocks session start when camera disconnected; optional allows with warning.

### Regression Risks to Avoid

- Creating a new tether supervisor instance for readiness, causing listener status always stopped/unknown.
- Bypassing folder watcher by creating photo/session/processing/upload records directly.
- Counting stale JPGs as a successful test capture.
- Running two gPhoto2 processes against the same station/camera.
- Running privileged `usbipd bind/attach`, install, driver, or global kill commands.
- Exposing raw stdout/stderr, command lines, local absolute paths, token, credential path, or bus diagnostic details in API/SSE/activity/UI.
- Blocking session start unconditionally in dev environments where camera readiness should be optional.
- Making Drive configuration required for local camera readiness.
- Breaking existing readiness keys required by frontend guard.
- Breaking existing station settings, discovery/assignment, tether listener, watch validation, session start, or OpenAPI contracts.
- Claiming real hardware pass without running hardware validation.

### Previous Story Intelligence

#### Story 7.1 learnings

- Production path is Go `apps/agent`; root `src/server` is historical spike only.
- Safe command execution must use allowlisted binaries/args and `exec.CommandContext`.
- `usbipd list` read-only diagnostic is allowed; bind/attach are not.
- API tests for auth/trusted-origin/safe errors are expected and prevented review findings.
- UI must render diagnostics safely and avoid raw stderr/HTML injection.
- OpenAPI must be updated with API changes.

#### Story 7.2 learnings

- Tether supervisor must be a singleton shared across router/handlers.
- Start/stop race can orphan gPhoto2 process; preserve the race regression tests.
- Status/stop endpoints must validate station scope, not return synthetic success for unknown station ids.
- WSL path conversion and path confinement are critical.
- Stop must not kill global gPhoto2/WSL processes.
- Tether package includes static guard to avoid importing ingestion/photo/session APIs; similar guard is useful for test capture.
- Windows Go tests may hit transient `Access is denied`; rerun with alternate `GOTMPDIR` and record honestly.

## Project Context Reference

- Project: `selfstudio`.
- User: `alpharize`.
- Current date: 2026-05-20.
- Planning artifacts folder: `_bmad-output/planning-artifacts`.
- Implementation artifacts folder: `_bmad-output/implementation-artifacts`.
- Production architecture: Go agent (`apps/agent`) + Next.js web (`apps/web`) + local data under `local-data`.

## Dev Agent Record

### File List

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/camera_test_capture.go`
- `apps/agent/internal/api/camera_test_capture_test.go`
- `apps/agent/internal/api/event_readiness.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/readiness.go`
- `apps/agent/internal/api/sessions.go`
- `apps/agent/internal/cameras/gphoto_runner.go`
- `apps/agent/internal/cameras/readiness_adapter.go`
- `apps/agent/internal/cameras/test_capture.go`
- `apps/agent/internal/cameras/test_capture_test.go`
- `apps/agent/internal/config/config.go`
- `apps/agent/internal/stations/readiness.go`
- `apps/agent/internal/stations/readiness_camera_test.go`
- `apps/agent/internal/stations/watch_validation.go`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/features/stations/station-settings.tsx`
- `apps/web/src/features/stations/use-camera-test-capture-mutation.ts`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `_bmad-output/implementation-artifacts/7-3-add-camera-readiness-checks-and-test-capture-validation.md`

### Debug Log

- 2026-05-20: Audit kode Go agent, frontend readiness/station settings, OpenAPI, dan story/spike guardrails selesai; prototype TypeScript lama tidak disalin.
- 2026-05-20: Menambahkan `SELFSTUDIO_CAMERA_READINESS_REQUIRED` default `false`, dependency injection discovery/tether/test-capture ke `ReadinessValidator`, dan adapter agar singleton tether supervisor dipakai readiness/session/API.
- 2026-05-20: Menambahkan readiness checks `gphoto2_availability`, `camera_assignment`, `camera_connected`, `tether_listener`, `input_folder_writable`, dan `camera_test_capture`; optional camera failure menjadi warning, required menjadi failed.
- 2026-05-20: Menambahkan endpoint trusted mutation `POST /api/stations/{station_id}/camera-test-capture`; flow membuat JPG expected filename di input folder, memvalidasi stable expected file via watcher validation path, mengecek JPEG SOI, menyimpan last result in-memory, dan publish activity/SSE aman.
- 2026-05-20: Menjaga local-first constraint: tidak import/call upload/photos/sessions/processing dari test capture service, tidak membuat record pipeline langsung, dan tidak menjalankan install/driver/usbipd privileged/global kill command.
- 2026-05-20: Run awal `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...` sempat gagal transient Windows: temp cleanup directory not empty dan `config.test.exe: Access is denied`; rerun dengan `.gotmp2` pass, kemudian rerun required `.gotmp` pass.
- 2026-05-20: Hardware camera nyata tidak tersedia di sesi ini; validasi hardware manual belum dijalankan. Implementasi diuji dengan fake runner/service dan tetap aman saat gPhoto2/camera absent.
- 2026-05-20: Review patch diterapkan: watcher validation kini memakai remaining capture timeout; legacy `device` menjadi ready saat assigned camera sudah supersede; UI kamera menampilkan label sanitized tanpa raw port; error details test capture hanya allowlist status/action/file_name; API test capture auth/trusted-origin/wrapper/station-scope ditambahkan.
- 2026-05-20: Re-review patch diterapkan: filename test capture memakai `UnixNano` + random suffix, output file di-reserve dengan `O_CREATE|O_EXCL`, argumen `--force-overwrite` dihapus, regression test collision/overwrite ditambahkan, dan UI watch validation kini hanya menampilkan safe basename dari `source_path`.
- 2026-05-20: Final blocker patch diterapkan: production allowlist `gphoto_runner` kini menerima command test capture aman `--port <port> --filename <file> --capture-image-and-download --quiet` tanpa `--force-overwrite`; regresi memastikan output `BuildTestCaptureCommand` native/WSL diterima allowlist, `--force-overwrite` tetap ditolak, dan filename injection tetap ditolak.

### Change Log

- 2026-05-20: Story context prepared and expanded for development. Status set to `ready-for-dev`.
- 2026-05-20: Implemented camera-aware readiness, required/optional session blocking, safe test capture validation endpoint/service, frontend button/DTO/invalidation, and OpenAPI docs. Status set to `review`.
- 2026-05-20: Addressed code review findings - 5 patch items resolved. Status remains `review`.
- 2026-05-20: Addressed re-review findings - 2 patch items resolved. Status remains `review`.
- 2026-05-20: Fixed final Story 7.3 blocker by aligning `gphoto_runner` production allowlist with safe no-overwrite test capture command and adding regression coverage. Status remains `review`.

### Validation Results

- `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...` — PASS setelah rerun; semua package Go pass.
- `cd apps/agent && GOTMPDIR=../../.gotmp2 go test ./...` — PASS, dipakai untuk mengatasi transient Windows Access Denied/temp cleanup pada run pertama.
- `cd apps/agent && GOTMPDIR=../../.gotmp2 go test ./internal/cameras ./internal/stations ./internal/api` — PASS untuk patch review setelah TDD red/green.
- `cd apps/agent && GOTMPDIR=../../.gotmp2 go test ./...` — FAIL transient Windows `Access is denied` saat menjalankan test binary `selfstudio-agent.test.exe` dan `readiness.test.exe`; package lain pass/cached.
- `cd apps/agent && GOTMPDIR=../../.gotmp-review5 go test ./...` — PASS setelah membuat direktori temp baru `.gotmp-review5`.
- `cd apps/web && npm run typecheck` — PASS.
- `cd apps/web && npm run build` — PASS; Next.js build compiled, lint/type validity, static page generation berhasil.
- `npm run typecheck` — PASS.
- Re-review 2026-05-20: `cd apps/agent && GOTMPDIR=../../.gotmp go test ./internal/cameras` — PASS.
- Re-review 2026-05-20: `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...` — PASS.
- Re-review 2026-05-20: `cd apps/web && npm run typecheck` — PASS.
- Re-review 2026-05-20: `cd apps/web && npm run build` — PASS.
- Re-review 2026-05-20: `npm run typecheck` — PASS.
- Final blocker 2026-05-20: `cd apps/agent && GOTMPDIR=../../.gotmp go test ./internal/cameras -run 'TestBuildTestCaptureCommandSafeNative|TestBuildTestCaptureCommandSafeWSLAcceptedByProductionAllowlist|TestProductionAllowlistRejectsUnsafeTestCaptureArgs'` — PASS.
- Final blocker 2026-05-20: `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...` — PASS.
- Final blocker 2026-05-20: `cd apps/web && npm run typecheck` — PASS.
- Final blocker 2026-05-20: `cd apps/web && npm run build` — PASS.
- Final blocker 2026-05-20: `npm run typecheck` — PASS.

## Completion Note

Story 7.3 selesai diimplementasikan dan siap review. Camera readiness kini memvalidasi assignment, gPhoto2 discovery, assigned camera connected, tether listener singleton, input folder writable, dan last camera test capture tanpa memicu capture otomatis. Session start memakai validator camera-aware yang sama dan dapat diblokir saat `SELFSTUDIO_CAMERA_READINESS_REQUIRED=true`. Camera test capture adalah explicit operator action, auth + trusted-origin, local-first ke station input folder, expected-file-only, JPEG SOI validation, watcher validation path, tanpa upload Drive langsung dan tanpa bypass pipeline.

Review follow-up 2026-05-20 selesai: timeout watcher validation tidak melewati capture timeout, aggregate readiness bisa ready saat camera checks supersede legacy device, UI tidak menampilkan raw port/identity, error details test capture sudah allowlisted, dan endpoint-level API tests untuk auth/trusted-origin/safe wrapper/station scope sudah ditambahkan.

Re-review follow-up 2026-05-20 selesai: filename test capture sekarang collision-safe dan tidak memakai `--force-overwrite`; UI watch validation tidak lagi merender absolute `source_path`, hanya nama file aman/basename; OpenAPI menegaskan raw `source_path` tidak boleh dirender di operator UI tanpa sanitasi.

Final blocker 2026-05-20 selesai: `gphoto_runner` production allowlist sudah selaras dengan command test capture aman dari `BuildTestCaptureCommand` tanpa `--force-overwrite`; regresi native/WSL memastikan command diterima allowlist, opsi overwrite tidak kembali, dan filename injection tetap ditolak. Final re-review bersih; status diset ke `done`.
