# Story 7.2: Supervise gPhoto2 Tether Listener per Station

Status: done

## Story

Sebagai operator, saya ingin start/stop gPhoto2 tether listener per station, sehingga shutter fisik kamera otomatis ter-download sebagai JPG ke input folder station.

## Acceptance Criteria

1. Given station punya camera assignment dan input folder writable, when operator starts tether listener, then sistem menjalankan supervised gPhoto2 wait-event/download process untuk station itu.
2. Given listener menerima capture event, when JPG didownload, then file ditulis hanya di input folder station dengan filename collision-safe.
3. Given duplicate start dikirim untuk station yang sama, then sistem return no-op/safe conflict dan tidak membuat process kedua.
4. Given operator stop listener, then hanya process listener station tersebut yang dihentikan dan status menjadi stopped.
5. Given gPhoto2 stderr/stdout berisi path/raw diagnostics, then API/SSE/activity hanya expose sanitized message/action.
6. Given file sudah masuk folder input, then existing folder watcher melakukan ingestion; listener tidak memanggil ingestion langsung.
7. Tests/build pass dan docs/API diperbarui.

## Acceptance Criteria Context / BDD Detail

### AC1 — Start listener supervised per station

- Implementasi production wajib di Go agent `apps/agent`, bukan prototype TypeScript lama di `src/server/*`.
- Start endpoint yang disarankan: `POST /api/stations/{station_id}/tether-listener/start`.
- Endpoint wajib `RequireAuth` + `RequireTrustedOrigin` karena menjalankan process lokal jangka panjang.
- Prasyarat sebelum start:
  - station id valid (`station_1`, `station_2`, `station_3`),
  - station memiliki `camera_assignment` dari Story 7.1,
  - `camera_assignment.identity_key`, `port`, dan `runtime` non-empty,
  - station `input_folder` ada/writable atau dapat divalidasi writable tanpa membuat file permanen,
  - gPhoto2 runtime tersedia sesuai assignment (`native_windows` atau `wsl`),
  - tidak ada listener existing untuk station yang sama.
- Command harus dibuat dari allowlist, tanpa shell string:
  - native: `gphoto2 --port <assigned_port> --filename <safe_pattern> --wait-event-and-download` atau varian batch non-interactive yang terbukti oleh tests/manual validation,
  - WSL: `wsl.exe -- gphoto2 --port <assigned_port> --filename <safe_pattern> --wait-event-and-download`,
  - jika perlu interval/timeout loop, supervisor boleh menjalankan loop process pendek, tetapi tetap satu logical listener per station.
- Process harus supervised:
  - `exec.CommandContext`/context cancellation,
  - status in-memory per station,
  - capture stdout/stderr bounded,
  - process exit terdeteksi dan status menjadi `error`/`stopped` sesuai penyebab,
  - cleanup saat agent shutdown bila hook shutdown sudah tersedia; minimal expose `StopAll()` di supervisor untuk dipanggil dari main saat wiring memungkinkan.
- Response sukses tetap `{data:{listener:{...}}}` dengan JSON `snake_case`.
- Listener state terpisah dari session state, processing status, dan Drive upload status.

### AC2 — Download JPG hanya ke input folder station dan filename collision-safe

- `--filename` harus resolve ke path di dalam `station.input_folder` saja.
- Jangan menerima output path dari request body. Semua path berasal dari persisted station config.
- Gunakan absolute cleaned path untuk input folder, lalu verifikasi setiap generated target path tetap berada di bawah folder tersebut.
- Filename pattern harus collision-safe dan tidak bergantung pada nama kamera mentah. Rekomendasi:
  - `station_1_%Y%m%d_%H%M%S_%n.%C` bila gPhoto2 token `%n/%C` tersedia, atau
  - supervisor-generated prefix + gPhoto2 numbering yang diuji, misalnya `station_1_capture_%Y%m%d_%H%M%S_%03n.%C`.
- Jika gPhoto2 menghasilkan RAW selain JPG, story ini hanya menjamin JPG pipeline. Jangan route RAW ke ingestion; raw files boleh dicegah melalui config/camera mode guidance atau didownload dengan ekstensi aman tetapi watcher existing hanya JPG. Dokumentasikan jika gPhoto2/camera mode perlu diset JPEG.
- Setelah download, listener boleh memperbarui `last_capture_at`/`last_downloaded_file_name` berdasarkan sanitized output jika dapat diparse aman. Jangan expose absolute path.

### AC3 — Duplicate start safe no-op/conflict

- Duplicate start untuk station yang sama tidak boleh membuat process kedua.
- Pilih salah satu perilaku konsisten:
  - `409 Conflict` dengan code `TETHER_LISTENER_ALREADY_RUNNING`, action `STOP_TETHER_LISTENER`, atau
  - `200 OK` no-op mengembalikan listener existing dengan `already_running: true`.
- Rekomendasi untuk operator UX: `200 OK` no-op lebih aman bila tombol diklik dua kali; tetap catat activity `station.tether_listener_start_noop`.
- Concurrency guard wajib ada di backend supervisor dengan mutex/map per station; UI disabled state saja tidak cukup.

### AC4 — Stop hanya process listener station tersebut

- Stop endpoint yang disarankan: `POST /api/stations/{station_id}/tether-listener/stop`.
- Stop wajib hanya membatalkan context/process milik station tersebut, tidak membunuh semua `gphoto2` process global.
- Jangan menggunakan `taskkill /IM gphoto2.exe` atau `pkill gphoto2` tanpa process group/id tracking karena bisa menghentikan station lain atau proses operator manual.
- Jika process sudah stopped, endpoint boleh no-op `200 OK` dengan status `stopped`.
- Stop harus menunggu grace period pendek, lalu force kill child yang diluncurkan oleh supervisor bila masih hidup. Untuk WSL, pastikan tidak memakai command global yang membunuh seluruh WSL/gPhoto process.
- Activity log harus mencatat success/failure dengan station ref.

### AC5 — Sanitized stdout/stderr/API/SSE/activity

- Raw stdout/stderr gPhoto2 sering memuat absolute path, camera path, libusb diagnostic, device id, atau OS-level detail. Public API/SSE/activity hanya boleh berisi safe message/action.
- Sanitization minimum:
  - absolute Windows path seperti `D:\...` dan `/mnt/c/...` diganti `[path]` atau hanya basename,
  - command line lengkap tidak diexpose,
  - stderr multiline dibatasi dan diringkas,
  - token/credential/env tidak diexpose,
  - bus id/port boleh diexpose hanya jika sudah disimpan sebagai safe assignment display; hindari raw diagnostic panjang.
- Technical raw logs jika benar-benar diperlukan hanya di local structured log, bukan API/SSE/activity. Namun story ini cukup menyimpan sanitized state in-memory.
- Error action allowlist disarankan:
  - `ASSIGN_CAMERA`,
  - `CHECK_STATION_INPUT_FOLDER`,
  - `INSTALL_GPHOTO2`,
  - `CHECK_WSL`,
  - `CHECK_USBIPD`,
  - `CONNECT_CAMERA`,
  - `CHECK_CAMERA_USB_MODE`,
  - `START_TETHER_LISTENER`,
  - `STOP_TETHER_LISTENER`,
  - `RETRY_TETHER_LISTENER`.

### AC6 — Folder watcher tetap source of truth ingestion

- Listener hanya men-download file ke station input folder.
- Listener tidak boleh memanggil `internal/ingestion.Router`, `Scanner`, `photos.Store`, session service, processing queue, quarantine, atau upload worker secara langsung.
- Existing watcher/scanner/stable JPG detection tetap satu-satunya jalur masuk ke photo/session pipeline.
- Kalau listener mengetahui filename dari stdout, data itu hanya untuk tether status (`last_downloaded_file_name`) dan operator diagnostics; bukan record photo authoritative.
- Jangan menambahkan shortcut “capture event -> photo record” karena akan merusak idempotency, session boundary, quarantine, original-first save, dan Drive upload flow yang sudah dibangun Epic 4-6.

### AC7 — Tests/build/docs/API

- Update `docs/api/openapi.yaml` dengan endpoints start/stop/status, schemas listener, error responses/actions, dan catatan sanitized diagnostics.
- Required validation setelah implementasi:
  - `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...` (boleh pakai `.gotmp2/.gotmp3` bila Windows transient Access Denied seperti Story 7.1),
  - `cd apps/web && npm run typecheck`,
  - `cd apps/web && npm run build`,
  - `npm run typecheck`.
- Catat hasil command aktual di Dev Agent Record. Jangan klaim pass tanpa menjalankan.

## Tasks / Subtasks

- [x] Audit kode existing dan pastikan boundary sebelum coding
  - [x] Baca lengkap `apps/agent/internal/cameras/types.go`, `gphoto_runner.go`, `gphoto_parser.go`, dan tests Story 7.1.
  - [x] Baca lengkap `apps/agent/internal/api/cameras.go`, `health.go`, `response.go`, `security.go`, `events.go`.
  - [x] Baca lengkap `apps/agent/internal/stations/store.go`, `persistence.go`, readiness/watch validation terkait input folder.
  - [x] Baca ingestion/watcher files yang relevan (`internal/ingestion/*`, `internal/photos/*`, API scan) hanya untuk memahami apa yang tidak boleh dipanggil langsung.
  - [x] Baca frontend `apps/web/src/lib/api/client.ts`, station settings/hooks existing, dan dashboard station feature bila status listener ditampilkan minimal.
  - [x] Baca spike `_bmad-output/implementation-artifacts/spec-gphoto-helper-diagnostic-spike.md`; ambil lesson listener, jangan copy production TS.
- [x] Desain domain model dan supervisor tether listener di Go
  - [x] Tambah package/file disarankan `apps/agent/internal/cameras/tether_supervisor.go` atau `internal/tether` dengan types `TetherListener`, `TetherStatus`, `TetherSupervisor`.
  - [x] Definisikan status allowlist: `stopped`, `starting`, `running`, `stopping`, `error`.
  - [x] Definisikan action/error allowlist sesuai AC5.
  - [x] Implement map per station + mutex; duplicate start safe.
  - [x] Implement process lifecycle dengan `exec.CommandContext`, cancellation, bounded stdout/stderr readers, exit monitoring, dan station-specific stop.
- [x] Implement safe command builder native/WSL
  - [x] Reuse atau extend allowlist pattern dari `cameras.ExecRunner`; jangan pakai shell concat.
  - [x] Validate runtime hanya `native_windows`/`wsl`; unknown harus error actionable.
  - [x] Validate port dari assignment dengan regex ketat untuk gPhoto port (`usb:001,004`, `ptpip:*`, dll) sebelum menjadi arg.
  - [x] Build filename pattern dari sanitized station id + absolute confined input folder.
  - [x] Unit test command spec tidak mengandung request/user shell input dan menolak path traversal.
- [x] Implement path confinement dan filename collision safety
  - [x] Resolve `station.input_folder` menjadi absolute clean path.
  - [x] Check writable dengan temp file create+remove atau existing helper jika ada.
  - [x] Ensure generated filename/pattern parent adalah input folder; reject jika clean/rel keluar folder.
  - [x] Sanitized public status hanya expose basename untuk downloaded file.
- [x] Implement API endpoints start/stop/status
  - [x] Tambah handler misalnya di `apps/agent/internal/api/camera_tether.go` atau extend `cameras.go`.
  - [x] Routes disarankan:
    - `GET /api/stations/{station_id}/tether-listener`,
    - `POST /api/stations/{station_id}/tether-listener/start`,
    - `POST /api/stations/{station_id}/tether-listener/stop`.
  - [x] Register di `apps/agent/internal/api/health.go` atau router wiring existing.
  - [x] Auth wajib untuk GET; auth + trusted origin untuk POST.
  - [x] Response `{data:{listener:{station_id,status,runtime,camera_name,started_at,stopped_at,last_capture_at,last_downloaded_file_name,last_error_code,last_error_action,message}}}`.
  - [x] Publish safe SSE events `camera.tether_listener_updated` atau `station.updated` hint, dengan wrapper existing.
  - [x] Record activity `station.tether_listener_started`, `station.tether_listener_stopped`, `station.tether_listener_failed`, `station.tether_listener_start_noop`.
- [x] Integrasi frontend minimal / API client
  - [x] Tambah types dan functions di `apps/web/src/lib/api/client.ts` untuk get/start/stop listener.
  - [x] Tambah hooks feature station bila pola TanStack Query existing digunakan.
  - [x] Minimal UI boleh berada di Station Settings atau station card control placeholder: status text, Start, Stop, error action. Jangan menunggu Story 7.4 untuk API client/docs; Story 7.4 akan memperluas dashboard UX.
  - [x] UI tidak menampilkan raw stderr/command/path; hanya message/action/basename.
- [x] Update OpenAPI/docs
  - [x] `docs/api/openapi.yaml`: paths start/stop/status, schemas `TetherListenerStatus`, `TetherListenerData`, response codes.
  - [x] Document duplicate start behavior, sanitized diagnostics, no direct ingestion guarantee, and safe manual actions for WSL/usbipd.
- [x] Backend tests
  - [x] Supervisor duplicate start under concurrency creates one process only.
  - [x] Stop station_1 does not stop station_2 fake process.
  - [x] Process exit updates status/error and safe action.
  - [x] Missing assignment -> error `CAMERA_ASSIGNMENT_REQUIRED` action `ASSIGN_CAMERA`.
  - [x] Unwritable/missing input folder -> `STATION_INPUT_FOLDER_UNWRITABLE` action `CHECK_STATION_INPUT_FOLDER`.
  - [x] Command builder native and WSL args match allowlist; rejects unknown runtime, invalid port, traversal path.
  - [x] Sanitizer strips Windows/WSL paths and command details from public status.
  - [x] No test imports/calls ingestion/photo/session APIs from listener package.
  - [x] API tests for auth, trusted origin, success wrapper, duplicate start behavior, stop no-op, no raw path leak.
- [x] Frontend/type/build validation
  - [x] TypeScript types compile.
  - [x] UI handles stopped/running/error/loading states.
  - [x] Run required validations and record actual results.

### Review Findings

- [x] [Review][Patch] Start/stop race can orphan a gPhoto2 process if stop lands while process startup is in-flight [apps/agent/internal/cameras/tether_supervisor.go:169]
- [x] [Review][Patch] Tether status/stop endpoints return synthetic success for invalid or unknown station ids instead of validating station scope [apps/agent/internal/api/camera_tether.go:30]

## Dev Notes

### Epic 7 Context

Epic 7 menambahkan managed gPhoto2 camera tethering di atas pipeline yang sudah ada. Prinsip utamanya: kamera/tether hanya bertugas menulis JPG ke `station.input_folder`; folder watcher existing dari Epic 4 tetap source of truth untuk ingestion dan routing. Story 7.2 adalah jembatan real camera shutter -> file input folder. Story ini tidak mengubah session lifecycle, routing, processing, quarantine, atau Google Drive upload.

Story 7.1 sudah selesai dan menyediakan:

- package `apps/agent/internal/cameras` untuk discovery parser/runner,
- persisted `station.camera_assignment`,
- duplicate camera assignment validation,
- API `POST /api/cameras/gphoto2/discover`,
- API `PUT /api/stations/{station_id}/camera-assignment`,
- Station Settings UI untuk discovery/assignment,
- OpenAPI docs untuk discovery/assignment,
- safe diagnostic policy: no install/driver/usbipd bind/attach otomatis.

Story 7.2 harus membangun di atas assignment tersebut. Jangan mengulang discovery sebagai prasyarat blocking kecuali untuk safe validation/status; gunakan assignment persisted sebagai target command.

### Current Architecture yang Wajib Diikuti

- Frontend: Next.js App Router + TypeScript + TanStack Query + Tailwind/shadcn di `apps/web`.
- Backend/local worker: Go service di `apps/agent`; semua filesystem, command invocation, worker, credentials, dan mutations berada di Go.
- API: REST under `/api`, SSE under `/events`, success wrapper `{data}`, error wrapper `{error:{code,message,action,details}}`, JSON `snake_case`.
- Auth/security: local session cookie/PIN gate; state-changing endpoints harus `RequireAuth` + `RequireTrustedOrigin`.
- Events: dot notation dan wrapper existing `events.New(...)`.
- Browser tidak boleh menjalankan command, membaca local filesystem langsung, atau menerima raw diagnostic/credential.
- Go service authoritative; frontend hanya TanStack Query cache + SSE invalidation/patch hints.

### Existing Code Intelligence / Current State

#### `apps/agent/internal/cameras/gphoto_runner.go`

- Sudah ada `CommandSpec`, `CommandResult`, `CommandRunner`, `ExecRunner` untuk command discovery.
- `ExecRunner` saat ini hanya allowlist:
  - `gphoto2 --auto-detect`,
  - `wsl.exe -- gphoto2 --auto-detect`,
  - `usbipd list`.
- Untuk listener, developer bisa:
  - menambah allowlist baru khusus tether command, atau
  - membuat runner/supervisor terpisah yang tetap command-array allowlisted.
- Jangan memaksakan listener melalui `DiscoveryService.Discover`; itu request/scan oriented, bukan long-running process supervisor.

#### `apps/agent/internal/cameras/types.go`

- Runtime enum: `native_windows`, `wsl`, `unknown`.
- `CameraAssignment` ada di cameras package, tetapi stations package juga punya mirrored `CameraAssignment`. Hati-hati import cycle; API saat ini memakai `stations.CameraAssignment` untuk persisted config.
- Safe actions existing termasuk `INSTALL_GPHOTO2`, `CHECK_WSL`, `CHECK_USBIPD`, `CONNECT_CAMERA`, `RETRY_CAMERA_DISCOVERY`, `CHECK_CAMERA_USB_MODE`, `CHOOSE_DIFFERENT_CAMERA`. Tambah tether actions tanpa merusak existing values.

#### `apps/agent/internal/stations/store.go`

- `Station` fields: `station_id`, `name`, `device_identifier`, `input_folder`, `background_name`, `default_lut_path`, `output_rule`, optional `camera_assignment`.
- `UpdateCameraAssignment` persists assignment and duplicate validation.
- `ReplaceAll`/backup/restore already normalize assignment.
- `ValidateStation` only validates string lengths and required station fields; input folder writability is handled elsewhere, so listener must validate writability at start.

#### `apps/agent/internal/api/cameras.go`

- Current handler owns discovery + assignment. It has access to station store/persistence/activity/broker.
- Good place to add tether handler only if not making file too large; otherwise create `camera_tether.go` with same dependencies plus supervisor.
- Existing pattern:
  - `writeData` success,
  - `writeAPIErrorWithDetails` errors,
  - `activityStore.RecordWithRefs`,
  - `events.New("station.updated", ...)`.

#### Router wiring in `apps/agent/internal/api/health.go`

- `NewMuxWithUploads(...)` currently creates `camerasHandler := NewCamerasHandler(...)` internally with `cameras.NewDiscoveryService(nil)`.
- If tether supervisor needs shared singleton, do not instantiate a new supervisor per request. Instantiate once during mux creation and capture in handler.
- For tests, consider adding constructor variant or allowing handler injection so fake supervisor can be used.

#### Existing ingestion/processing boundary

- `apps/agent/internal/ingestion/router.go`, `scanner.go`, `photos/store.go`, `processing/*`, `upload/*` already implement photo pipeline concepts.
- Story 7.2 must not import these from tether supervisor. The only integration is filesystem: gPhoto2 writes file into station input folder, then watcher/scanner sees it.

### Spike Specs and Lessons to Reuse Carefully

From `_bmad-output/implementation-artifacts/spec-gphoto-helper-diagnostic-spike.md`:

- Spike implemented old TS endpoints including trigger listener:
  - `GET /api/gphoto/trigger-listener/status`,
  - `POST /api/gphoto/trigger-listener/start`,
  - `POST /api/gphoto/trigger-listener/stop`.
- It used `gphoto2 --wait-event-and-download=<seconds>` in a loop and let operator press physical shutter repeatedly.
- Useful lessons:
  - no install commands,
  - no driver changes,
  - output must stay inside station input folder,
  - capture/listener must be serialized per relevant scope,
  - setup/usbipd reality matters on Windows.
- Do not copy TS files `src/server/gphoto-helper.ts` / `gphoto-autosetup.ts` into production. Current production backend is Go `apps/agent`.
- Spike one-click setup did `usbipd bind/attach`; Story 7.2 must NOT do that automatically. Only return safe actions/instructions.

### Windows / WSL / usbipd Constraints

- Native Windows gPhoto2 may not exist or may be unreliable. WSL runtime is expected/common.
- WSL gPhoto2 can only see USB cameras attached to WSL. Attachment often requires manual `usbipd bind`/`attach`; `bind` may require elevated terminal.
- Story 7.2 must never run:
  - `winget`, `choco`, `apt install`,
  - `usbipd bind`, `usbipd attach`, `usbipd detach`,
  - driver switch/Zadig/registry commands,
  - global `pkill gphoto2`, `taskkill /IM gphoto2.exe`.
- If camera is not reachable, return action `CHECK_USBIPD`, `CONNECT_CAMERA`, or `CHECK_CAMERA_USB_MODE` depending on safe diagnostic.
- WSL path conversion is critical:
  - gPhoto2 running inside WSL needs a Linux path, e.g. Windows `D:\_Project\selfstudio\local-data\input\station-1` may be `/mnt/d/_Project/selfstudio/local-data/input/station-1`.
  - Implement deterministic Windows-to-WSL path conversion only for local drive paths; reject UNC/network paths for WSL listener unless explicitly supported.
  - Do not expose converted absolute path in API/SSE; public status uses station id and basename only.
- usbipd bus IDs/ports can change after reconnect. Assignment from Story 7.1 may be name+port MVP identity. If start fails due stale port, surface `RETRY_TETHER_LISTENER`/`CONNECT_CAMERA` and let Story 7.5 handle reconnect refinement.

### Safe Command / Process Supervision Constraints

- Use `exec.CommandContext`, not `cmd /C`, `powershell -Command`, or shell-concatenated strings.
- Command args must be constructed from:
  - constant binary names (`gphoto2`, `wsl.exe`),
  - constant flags (`--port`, `--filename`, `--wait-event-and-download`),
  - validated assignment port,
  - internally generated filename pattern.
- Request body should not include command args, filename, folder, runtime override, or port override.
- Output readers must be bounded to avoid memory growth during long event operation. Keep rolling last N sanitized lines or last safe message.
- Consider process state:
  - `starting`: command created but not confirmed running,
  - `running`: process started,
  - `stopping`: stop requested,
  - `stopped`: no process,
  - `error`: process exited unexpectedly or prerequisites failed.
- Because gPhoto2 may run continuously, status endpoint should not block waiting for process completion.
- If implementing loop with short `--wait-event-and-download=<seconds>`, stop must break loop quickly and not spawn a new iteration after stop.

### File References

#### Must read before implementation

- `_bmad-output/planning-artifacts/epics.md` — Epic 7 and Story 7.2 context.
- `_bmad-output/planning-artifacts/architecture.md` — API/SSE/error/security/project boundaries.
- `_bmad-output/planning-artifacts/prd.md` — station readiness, local safety, operator UX requirements.
- `_bmad-output/implementation-artifacts/7-1-detect-and-assign-gphoto2-cameras-to-stations.md` — completed assignment/discovery context and validation learnings.
- `_bmad-output/implementation-artifacts/spec-gphoto-helper-diagnostic-spike.md` — prior listener spike lessons and anti-patterns.
- `apps/agent/internal/cameras/types.go`
- `apps/agent/internal/cameras/gphoto_runner.go`
- `apps/agent/internal/cameras/gphoto_parser.go`
- `apps/agent/internal/cameras/*_test.go`
- `apps/agent/internal/stations/store.go`
- `apps/agent/internal/stations/persistence.go`
- `apps/agent/internal/api/cameras.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/response.go`
- `apps/agent/internal/api/security.go`
- `apps/agent/internal/events/event.go`
- `apps/agent/internal/activity/store.go`
- `apps/web/src/lib/api/client.ts`
- `apps/web/src/features/stations/station-settings.tsx`
- `docs/api/openapi.yaml`

#### Likely files to create/update

- `apps/agent/internal/cameras/tether_supervisor.go` (NEW)
- `apps/agent/internal/cameras/tether_command.go` (NEW or combined)
- `apps/agent/internal/cameras/tether_supervisor_test.go` (NEW)
- `apps/agent/internal/cameras/tether_command_test.go` (NEW)
- `apps/agent/internal/api/camera_tether.go` (NEW) or `cameras.go` (UPDATE)
- `apps/agent/internal/api/camera_tether_test.go` (NEW)
- `apps/agent/internal/api/health.go` (UPDATE route registration and singleton supervisor wiring)
- `apps/web/src/lib/api/client.ts` (UPDATE types/functions)
- `apps/web/src/features/stations/use-tether-listener-mutation.ts` (NEW optional)
- `apps/web/src/features/stations/station-settings.tsx` or station card component (UPDATE minimal controls/status)
- `docs/api/openapi.yaml` (UPDATE)

### Testing Strategy

Backend tests are highest value because hardware may not be present in CI/dev:

1. Supervisor lifecycle tests with fake process runner:
   - start transitions `stopped -> starting/running`,
   - duplicate start returns existing/no-op and start count remains 1,
   - stop transitions station-specific process to `stopped`,
   - unexpected process exit becomes `error` with safe action,
   - stop after already stopped is safe no-op.
2. Command/path tests:
   - native command args exact allowlist,
   - WSL command args exact allowlist,
   - WSL path conversion for drive-letter path,
   - UNC/network path rejected for WSL unless explicitly supported,
   - invalid port rejected,
   - filename pattern parent confined to input folder,
   - path traversal in station config cannot escape.
3. Sanitization tests:
   - Windows paths redacted,
   - WSL `/mnt/c/...` paths redacted,
   - command strings redacted,
   - multiline stderr bounded,
   - public status contains basename only.
4. API tests:
   - auth required for status/start/stop,
   - trusted origin required for start/stop,
   - missing assignment error shape,
   - unwritable folder error shape,
   - duplicate start behavior,
   - stop only station target,
   - response wrapper and `snake_case`,
   - no raw path/command leak in JSON.
5. Frontend validation:
   - typecheck/build,
   - controls render stopped/running/error,
   - duplicate/no-op error displayed actionable,
   - raw diagnostics not rendered.
6. Manual hardware validation after implementation:
   - No gPhoto2: start returns `INSTALL_GPHOTO2` or `CHECK_WSL` safely.
   - WSL gPhoto2 but camera not attached: start returns `CHECK_USBIPD`/`CONNECT_CAMERA` safely.
   - Camera assigned and attached: start listener, press physical shutter, JPG appears in station input folder.
   - Existing watcher ingests JPG without listener calling ingestion.
   - Duplicate start does not create second gPhoto2 process.
   - Stop station_1 does not affect station_2 listener.

### Regression Risks to Avoid

- Reusing old TS `src/server/gphoto-helper.ts` as production implementation.
- Running privileged setup (`usbipd bind/attach`, install, driver changes) automatically.
- Creating shell command strings from station/camera/request values.
- Killing global gPhoto2/WSL processes instead of station-owned process.
- Bypassing folder watcher by creating photo/session records directly.
- Exposing raw stdout/stderr, absolute paths, command line, bus diagnostics, tokens, or credential paths in API/SSE/activity.
- Storing listener state as session/photo/processing/upload state.
- Blocking dashboard/API request while listener is running.
- Allowing duplicate listener process for one station under rapid double-click/concurrent requests.
- Breaking Story 7.1 discovery/assignment APIs or station backup/restore.
- Assuming Sony camera supports tether mode without safe failure/action.

### Previous Story Intelligence (Story 7.1)

Story 7.1 completed after review patches. Key learnings for Story 7.2:

- Production path is Go `apps/agent`; old spike under root `src/` is historical only.
- Safe command execution must use allowlisted binaries/args and `exec.CommandContext`.
- Windows transient `go test` failures can occur with `.gotmp` Access Denied; rerun with alternate GOTMPDIR and document accurately.
- API tests were required by review when missing; include them upfront for tether endpoints.
- Parser/runner must handle WSL/usbipd states gracefully and never crash.
- `usbipd list` read-only diagnostic is allowed; bind/attach are not.
- UI must render diagnostics safely; do not use raw stderr or `dangerouslySetInnerHTML`.
- OpenAPI must be updated in the same story as API additions.

### Project Context Reference

- Project: `selfstudio`.
- User: `alpharize`.
- Current date: 2026-05-20.
- Planning artifacts folder: `_bmad-output/planning-artifacts`.
- Implementation artifacts folder: `_bmad-output/implementation-artifacts`.
- Local production architecture: Go agent (`apps/agent`) + Next.js web (`apps/web`) + local data under `local-data`.

## Dev Agent Record

### Debug Log

- 2026-05-20: Loaded story 7.2, sprint status, planning artifacts, Story 7.1 context, camera/station/API/security/event/activity/frontend/OpenAPI code, and spike constraints before implementation.
- 2026-05-20: Implemented Go tether supervisor with per-station mutex/map, duplicate start no-op, station-specific stop, bounded stdout/stderr readers, command-array execution, status transitions, sanitized public state, and StopAll hook.
- 2026-05-20: Implemented native/WSL command builder using constant binaries/flags, strict runtime/port validation, confined filename pattern, WSL drive path conversion, UNC rejection for WSL, and input folder writable check via temp create/remove.
- 2026-05-20: Added REST endpoints for listener status/start/stop with auth/trusted-origin, activity records, safe SSE event, and no ingestion/photo/session/processing/upload shortcut.
- 2026-05-20: Added minimal web API client, TanStack Query hooks, and Station Settings controls for sanitized listener status/start/stop.
- 2026-05-20: Updated OpenAPI contract with tether listener paths, schemas, duplicate start behavior, sanitized diagnostics, and folder watcher source-of-truth note.
- 2026-05-20: First required Go validation with `.gotmp` hit Windows transient `Access is denied` for `cmd/selfstudio-agent`; reran with `.gotmp2` successfully per story guidance.
- 2026-05-20: Added failing regression tests for review patches: Stop during in-flight process startup must kill any late-started process, and GET/STOP tether endpoints must reject unknown station ids instead of returning synthetic stopped success.
- 2026-05-20: Fixed tether supervisor startup/stop race by marking runtime stopped under lock, detecting canceled/stopped runtime immediately after process start returns, canceling context, killing late process, and preserving stopped state/map cleanup.
- 2026-05-20: Fixed tether status/stop API scope validation by looking up station ids through stations store before calling supervisor Status/Stop; invalid or unknown station ids now return safe API errors.

### File List

- apps/agent/internal/cameras/tether_supervisor.go
- apps/agent/internal/cameras/tether_supervisor_test.go
- apps/agent/internal/api/camera_tether.go
- apps/agent/internal/api/camera_tether_test.go
- apps/agent/internal/api/health.go
- apps/web/src/lib/api/client.ts
- apps/web/src/features/stations/use-tether-listener.ts
- apps/web/src/features/stations/station-settings.tsx
- docs/api/openapi.yaml
- _bmad-output/implementation-artifacts/7-2-supervise-gphoto2-tether-listener-per-station.md
- _bmad-output/implementation-artifacts/sprint-status.yaml

### Change Log

- 2026-05-20: Added supervised per-station gPhoto2 tether listener implementation, safe command/path handling, status/start/stop API, frontend controls, OpenAPI docs, and tests.
- 2026-05-20: Addressed code review patch findings: startup stop race now kills/aborts late-started processes, and tether status/stop endpoints validate station scope before supervisor calls.
- 2026-05-20: Moved Story 7.2 to review after validations passed.

### Validation Results

- `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...` — FAILED due Windows transient `fork/exec D:\_Project\selfstudio\.gotmp\...\selfstudio-agent.test.exe: Access is denied` only for `cmd/selfstudio-agent`; other packages passed/cached.
- `cd apps/agent && GOTMPDIR=../../.gotmp2 go test ./...` — PASS.
- `cd apps/agent && GOTMPDIR=../../.gotmp-review go test ./internal/cameras ./internal/api` — FAILED as expected for new review regression coverage; `internal/api` exposed synthetic success for unknown station id, while `internal/cameras` hit Windows transient `Access is denied` for test binary.
- `cd apps/agent && GOTMPDIR=../../.gotmp-review2 go test ./internal/cameras ./internal/api` — FAILED due Windows transient `Access is denied` for `internal/api`; `internal/cameras` passed including startup/stop race regression.
- `cd apps/agent && GOTMPDIR=../../.gotmp-review3 go test ./internal/api` — PASS after station-scope validation fix and test path normalization adjustment.
- `cd apps/agent && GOTMPDIR=../../.gotmp-review4 go test ./...` — PASS.
- `cd apps/web && npm run typecheck` — PASS.
- `cd apps/web && npm run build` — PASS.
- `npm run typecheck` — PASS.

## Completion Note

Ultimate context engine analysis completed - comprehensive developer guide created. Story 7.2 siap untuk development dengan guardrails process supervision, command safety, path confinement, Windows/WSL/usbipd constraints, API/UI/docs expectations, dan testing strategy lengkap.
