# Story 7.4: Show Live Camera/Tether Status and Controls in the Dashboard

Status: done

## Story

Sebagai operator, saya ingin setiap station card menampilkan health kamera dan tether secara live, sehingga saya bisa cepat bereaksi saat event tanpa membuka log teknis atau berpindah ke halaman settings.

## Acceptance Criteria

1. Given dashboard terbuka, then tiap station card menampilkan camera assigned/unassigned, connected/disconnected, listener running/stopped/error, last capture time, last downloaded filename, dan next action.
2. Given operator klik start/stop/retry listener atau test capture, then request memakai authenticated trusted-origin endpoint dan update UI secara aman.
3. Given status berubah, then UI update via polling/SSE tanpa merusak live station/session cards existing.
4. Given error terjadi, then UI menampilkan safe action label tanpa raw shell output, secret, raw camera port/identity, command output, atau path sensitif.
5. Given Drive upload/local processing punya status berbeda, then UI tetap membedakan camera/tether, local processing, dan Google Drive upload.
6. Typecheck/build pass.

## Acceptance Criteria Context / BDD Detail

### AC1 — Station card menampilkan status kamera/tether live

- Target utama adalah `apps/web/src/features/sessions/live-station-cards.tsx`, bukan redesign besar dashboard.
- Untuk setiap station card, tambahkan area/section eksplisit seperti **Camera/Tether** yang terpisah dari section session, local processing, quarantine, dan Google Drive.
- Data authoritative harus berasal dari Go agent APIs yang sudah ada dari Stories 7.1-7.3:
  - `GET /api/stations` untuk persisted `station.camera_assignment`.
  - `GET /api/stations/{station_id}/readiness` untuk checks `camera_assignment`, `gphoto2_availability`, `camera_connected`, `tether_listener`, `input_folder_writable`, `camera_test_capture`.
  - `GET /api/stations/{station_id}/tether-listener` untuk `listener.status`, `runtime`, `camera_name`, `started_at`, `stopped_at`, `last_capture_at`, `last_downloaded_file_name`, `last_error_code`, `last_error_action`, `message`.
  - `POST /api/stations/{station_id}/camera-test-capture` untuk explicit test capture action.
- Minimum UI per station card:
  - Assignment: `Assigned` / `Unassigned` + safe camera display name only.
  - Connection: `Connected` / `Disconnected` / `Unknown` dari readiness `camera_connected`, bukan OS-level USB raw data.
  - Listener: `Running` / `Stopped` / `Error` / `Starting` / `Stopping` dari tether listener endpoint.
  - Last capture time: local formatted time from `last_capture_at`, or `Belum ada capture`.
  - Last downloaded filename: basename-only from `last_downloaded_file_name`, or `-`.
  - Next action: safe actionable label derived from readiness/listener `action` fields, e.g. `ASSIGN_CAMERA`, `CONNECT_CAMERA`, `START_TETHER_LISTENER`, `RETRY_TETHER_LISTENER`, `CHECK_STATION_INPUT_FOLDER`, `RECHECK_CAMERA_READINESS`.
- Do not show raw `camera_assignment.port`, `identity_key`, raw `device_path`, full `input_folder`, WSL path, gPhoto command, or stdout/stderr.
- Keep text labels; do not rely on color only.

### AC2 — Start/stop/retry listener dan test capture memakai endpoint aman

- Buttons on station card should call existing client functions/endpoints:
  - Start: `POST /api/stations/{station_id}/tether-listener/start`.
  - Stop: `POST /api/stations/{station_id}/tether-listener/stop`.
  - Retry listener: reuse start endpoint when current listener status is `error` or `stopped`, with button copy `Retry listener` or `Start listener` depending state.
  - Test capture: `POST /api/stations/{station_id}/camera-test-capture`.
- Frontend does not add auth headers manually beyond existing API client behavior; Go agent enforces auth/session and trusted origin on POST endpoints.
- Buttons must have pending/disabled states to prevent accidental rapid double-click UX, but backend remains source of safety for duplicate start.
- If test capture is blocked because listener is running or active session exists, display safe server error action. Do not invent client workaround.
- Button enablement guidance:
  - Start/Retry enabled when station exists and listener status is `stopped` or `error`; disabled while `starting`/`running`/mutation pending.
  - Stop enabled only when listener status is `running` or `starting`; disabled otherwise.
  - Test capture enabled when station exists and no mutation is pending; if server rejects due listener conflict or missing assignment, show server error/action.
- Do not add any browser-side command execution, WSL/usbipd operation, path probing, direct filesystem access, or Google Drive call.

### AC3 — Polling/SSE update tanpa merusak existing cards

- Existing dashboard already uses TanStack Query and SSE in `apps/web/src/features/health/health-dashboard.tsx`.
- Existing SSE invalidation currently does **not** listen for `camera.tether_listener_updated`. Add it.
- On `camera.tether_listener_updated` event (`entity_type: station`), invalidate or patch:
  - `['tether-listener', station_id]`,
  - `stationReadinessQueryKey(station_id)`,
  - `stationsQueryKey` if needed,
  - `activityQueryKey("")`.
- Continue existing invalidations for `station.camera_test_capture_completed`, `station.readiness_checked`, `station.health_refreshed`, `station.updated`, session/photo/upload events.
- `useTetherListenerQuery` should be used in each station card or a child component. If adding polling, keep it modest (e.g. 5-10s or disabled if SSE enough) and avoid heavy dashboard churn.
- Do not regress current live card features:
  - session active/locked display,
  - customer/order/timer,
  - LUT/background,
  - photo count,
  - quarantine summary,
  - Drive folder/upload controls,
  - End Session confirmation.

### AC4 — Safe error display and no sensitive/raw output

- Display `ApiError.message`, `ApiError.action`, `listener.message`, and readiness `check.action` only after safe formatting.
- Never render:
  - raw stdout/stderr,
  - full shell command,
  - `gphoto2 --...`, `wsl.exe --...`, `usbipd ...`,
  - absolute Windows paths (`D:\...`) or WSL paths (`/mnt/...`),
  - raw camera port (`usb:001,004`) unless backend already provides a safe display label; for this story, avoid showing raw port entirely,
  - raw `identity_key`, device path, bus id, token, credential, environment variable, or Drive secret.
- Use React escaped text only; do not use `dangerouslySetInnerHTML`.
- Add small helper(s) in frontend if needed:
  - `safeCameraLabel(assignment)` returns `camera_name` or `Assigned camera`.
  - `safeFileName(name)` returns basename and strips separators.
  - `cameraActionLabel(action)` maps action codes to Indonesian operator copy.
- Error copy should be actionable under event pressure, e.g. `Hubungkan kamera lalu Recheck`, `Assign kamera di Station Settings`, `Start tether listener`, `Cek input folder station`.

### AC5 — Separate camera/tether vs local processing vs Drive upload

- Current `LiveStationCard` already displays local/session summary and Google Drive status. Do not merge camera/tether state into those labels.
- Add separate labels such as:
  - `Camera/Tether: Running — last file ...`
  - `Local processing: ...` existing photo/processing summary remains separate.
  - `Google Drive upload: ...` existing Drive status remains separate.
- Do not show station as globally `READY` just because Drive upload is ready; camera readiness and local/Drive status are independent.
- Do not let Drive status or Drive credentials absence block camera/tether controls.

### AC6 — Validation

- Required validation commands after implementation:
  - `cd apps/web && npm run typecheck`
  - `cd apps/web && npm run build`
  - `npm run typecheck`
- If backend code is touched, also run:
  - `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...` (use alternate `.gotmp2/.gotmp-review*` and document if Windows transient `Access is denied` occurs).
- Record actual results in Dev Agent Record. Do not claim pass without running.

## Tasks / Subtasks

- [x] Audit current implementation before coding
  - [x] Read `apps/web/src/features/sessions/live-station-cards.tsx` completely and preserve current session/quarantine/Drive behavior.
  - [x] Read `apps/web/src/features/health/health-dashboard.tsx` SSE invalidation logic.
  - [x] Read `apps/web/src/features/stations/use-tether-listener.ts` and existing mutation/query invalidation.
  - [x] Read `apps/web/src/features/stations/use-camera-test-capture-mutation.ts`.
  - [x] Read `apps/web/src/lib/api/client.ts` types/functions for `Station`, `ReadinessCheck`, `TetherListener`, `CameraTestCaptureResult`, `getTetherListener`, `startTetherListener`, `stopTetherListener`, `runCameraTestCapture`.
  - [x] Read `apps/web/src/features/stations/station-settings.tsx` to reuse safe display patterns and avoid reintroducing raw port/path display.
  - [x] Skim backend API handlers `apps/agent/internal/api/camera_tether.go`, `camera_test_capture.go`, `readiness.go` for response shapes and safe actions.
- [x] Add camera/tether status component for station cards
  - [x] Create a focused child component (recommended) inside `apps/web/src/features/sessions/live-station-cards.tsx` or new file `camera-tether-status.tsx`.
  - [x] Use `useTetherListenerQuery(station.station_id)` and existing station readiness data.
  - [x] Render assignment, connection, listener status, last capture time, last downloaded filename, next action.
  - [x] Use text labels and badges; ensure status remains understandable without color.
  - [x] Keep layout compact; no major redesign.
- [x] Add station card controls
  - [x] Wire Start Listener button to `useStartTetherListenerMutation`.
  - [x] Wire Stop Listener button to `useStopTetherListenerMutation`.
  - [x] Wire Retry Listener as start mutation when listener status is `error`/`stopped`.
  - [x] Wire Run Test Capture button to `useCameraTestCaptureMutation`.
  - [x] Show pending states and safe error/action messages per station.
  - [x] Invalidate relevant station readiness/tether/stations/activity queries after mutations if hooks do not already cover all needed keys.
- [x] Extend SSE invalidation
  - [x] Add listener for SSE event `camera.tether_listener_updated` in `health-dashboard.tsx`.
  - [x] On event, invalidate `['tether-listener', station_id]`, station readiness, stations if needed, and activity.
  - [x] Ensure cleanup removes the added listener.
  - [x] Verify existing photo/session/upload/readiness event invalidations remain intact.
- [x] Implement safe display helpers
  - [x] Add frontend helper(s) for basename-only filename rendering.
  - [x] Add action code to Indonesian copy mapping for camera/tether actions.
  - [x] Ensure raw `port`, `identity_key`, `source_path`, command strings, WSL paths, and local absolute paths are not rendered in station card.
  - [x] Add fallback copy for unknown status/action without exposing raw data.
- [x] Preserve local processing and Drive separation
  - [x] Keep existing session detail, local output summary, photo count, processing/queue references, quarantine, Drive target/upload controls.
  - [x] Label Camera/Tether section separately from Google Drive upload section.
  - [x] Do not alter backend session/photo/upload state machines.
- [x] Tests/validation
  - [x] If test infrastructure exists for frontend components, add or update tests for safe rendering and action button state; otherwise document manual UI checks in Dev Agent Record.
  - [x] Run `cd apps/web && npm run typecheck`.
  - [x] Run `cd apps/web && npm run build`.
  - [x] Run `npm run typecheck`.
  - [x] If any Go/backend file changed, run `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`. (Tidak berlaku: tidak ada file Go/backend diubah.)

## Dev Notes

### Review Findings

- [x] [Review][Patch] Test capture failure omits server next action [apps/web/src/features/sessions/camera-tether-status.tsx:66] — AC2/AC4 requires blocked/rejected test capture to display the safe server error/action and not invent a client workaround. The success path renders only `data.test_capture.label` plus optional file name; if the backend returns a non-success validation result with `action` such as `ASSIGN_CAMERA`, `STOP_TETHER_LISTENER`, or `RECHECK_CAMERA_READINESS`, the station card drops that action and gives the operator less guidance.
- [x] [Review][Patch] Raw listener message is rendered with incomplete sanitization [apps/web/src/features/sessions/camera-tether-status.tsx:107] — AC4 says never render raw camera port/identity/device path/command/stdout/stderr/path-sensitive output. `listener.message` is rendered through `safeText`, but `safeText` does not scrub raw gPhoto port strings like `usb:001,004`, Unix paths outside `/mnt/...`, identity keys, device paths, bus IDs, or stdout/stderr labels. Because frontend runtime guards accept any listener `message` string, a backend regression or unexpected API payload could leak forbidden details into the dashboard.
- [x] [Re-review][Patch] `listener.message` sanitization still misses some absolute Unix paths and command-like diagnostics [apps/web/src/features/sessions/camera-tether-status.tsx:145] — AC4/prior follow-up requires redacting Unix/Windows paths and command-like diagnostics. Current `safeText` only redacts Unix paths under selected roots (`/mnt`, `/home`, `/tmp`, `/var`, `/usr`, `/etc`, `/opt`, `/media`, `/run`, `/dev`) and commands named `gphoto2`, `wsl`, `usbipd`, `powershell`, or `cmd`. It can still render absolute paths such as `/Users/operator/event/photo.jpg`, `/Volumes/card/DCIM/IMG_0001.JPG`, or `/workspace/selfstudio/local-data/...`, and shell-like diagnostics such as `bash -lc ...`, `sh -c ...`, or `/bin/sh: ...`. Broaden `safeText` to redact generic absolute Unix paths and common shell command diagnostics before marking done.

### Epic 7 Context

Epic 7 menambahkan managed gPhoto2 camera tethering di atas pipeline lokal yang sudah ada. Story 7.4 adalah UI dashboard story: operator harus bisa melihat dan mengendalikan status camera/tether langsung dari live station cards. Story ini tidak mengubah cara foto masuk pipeline: gPhoto2/tether hanya menulis JPG ke `station.input_folder`; folder watcher/ingestion tetap source of truth untuk routing ke session, original-first save, LUT processing, quarantine, dan Google Drive upload.

### Current Architecture yang Wajib Diikuti

- Frontend: `apps/web`, Next.js App Router, TypeScript, TanStack Query, Tailwind/shadcn.
- Backend/local worker: `apps/agent` Go service. Browser tidak boleh menjalankan command, membaca filesystem, atau memegang credential.
- API: REST under `/api`, SSE under `/events`, success wrapper `{data}`, error wrapper `{error:{code,message,action,details}}`, JSON `snake_case`.
- Auth/security: local PIN/session cookie; POST commands already protected in Go by auth + trusted origin.
- Events: dot notation; Story 7.2 publishes `camera.tether_listener_updated` with `entity_type: station`.
- Go service authoritative; frontend hanya server-state cache via TanStack Query + SSE invalidation.

### Completed Story Intelligence

#### Story 7.1 — Discovery/Assignment

- Implemented safe gPhoto2 discovery and station camera assignment in Go.
- Persisted `station.camera_assignment` includes `identity_key`, `camera_name`, `port`, `runtime`, timestamps/connected.
- Important UI lesson: do not show raw port or identity key. Prior review caught raw `port`/`identity_key` display and it was fixed in settings UI.
- Discovery/assignment does not install gPhoto2, does not run privileged usbipd bind/attach, and does not start listeners.

#### Story 7.2 — Tether Listener Supervisor

- Existing APIs:
  - `GET /api/stations/{station_id}/tether-listener`
  - `POST /api/stations/{station_id}/tether-listener/start`
  - `POST /api/stations/{station_id}/tether-listener/stop`
- Existing web hook: `apps/web/src/features/stations/use-tether-listener.ts` with query key `['tether-listener', stationId]`.
- Backend publishes `camera.tether_listener_updated` SSE event with safe `listener` payload.
- Supervisor protects duplicate starts, station-scoped stop, command allowlist, path confinement, WSL path conversion, bounded diagnostics, and sanitized public messages.
- Do not add global `taskkill`, `pkill`, usbipd bind/attach, install commands, or browser-side commands.

#### Story 7.3 — Camera Readiness/Test Capture

- Existing endpoint: `POST /api/stations/{station_id}/camera-test-capture`.
- Existing web hook: `apps/web/src/features/stations/use-camera-test-capture-mutation.ts`.
- Readiness checks now include camera-related keys and can be required/optional via backend config.
- Test capture is explicit operator action, local-first, expected-file-only, JPEG SOI validated, watcher validation path, no Drive upload direct, no photo/session/processing/upload store bypass.
- Review learnings:
  - UI must not render absolute `source_path`.
  - UI must not render raw camera port/identity.
  - Test capture errors should show allowlisted safe details only.

### Existing Code References — Current State

#### `apps/web/src/features/sessions/live-station-cards.tsx`

- Renders `LiveStationCards` and per-station `LiveStationCard`.
- Current data sources:
  - `useStationsQuery()` for station config.
  - `useSessionsQuery()` for active/locked sessions.
  - `useStationReadinessQuery(station_id)` for readiness.
  - `useSessionDetailQuery(session_id)` for summary/photo counts/upload/local output.
  - `useStationQuarantineSummaryQuery(station_id)`.
  - Drive mutations: resolve/start/retry.
- Current UI already includes:
  - status/customer/order/timer/LUT/background/photo count/quarantine.
  - session detail summary.
  - Drive folder and upload controls.
- Story 7.4 should add camera/tether status here without deleting existing behavior.
- Current code renders some sensitive local-ish values such as `default_lut_path`, `local_output_folder`, `drive_folder_path`; this story’s hard constraint specifically applies to camera/tether port/path/command output. Do not introduce new raw camera/path leaks.

#### `apps/web/src/features/health/health-dashboard.tsx`

- Owns global SSE connection and query invalidation.
- Already listens for many events including `station.camera_test_capture_completed`, but not `camera.tether_listener_updated` yet.
- Add both `stream.addEventListener(...)` and cleanup `removeEventListener(...)`.

#### `apps/web/src/features/stations/use-tether-listener.ts`

- Query key: `['tether-listener', stationId]`.
- Mutations currently set query data and invalidate `['activity']`; activity query key elsewhere is `activityQueryKey("")`, so consider adding proper invalidation at call site or in hook if needed.

#### `apps/web/src/lib/api/client.ts`

- Relevant types already exist:
  - `CameraAssignment`
  - `Station`
  - `ReadinessCheck`
  - `StationReadiness`
  - `TetherListenerStatus`
  - `TetherListener`
  - `CameraTestCaptureResult`
- Relevant functions already exist around lines ~1120 and ~1250:
  - `runCameraTestCapture(stationId)`
  - `getTetherListener(stationId)`
  - `startTetherListener(stationId)`
  - `stopTetherListener(stationId)`

#### Backend API shape references

- `apps/agent/internal/api/camera_tether.go`
  - Error details for tether start currently include `listener`, which should already be sanitized by backend. UI should still avoid displaying any field not allowlisted.
  - Publish event: `camera.tether_listener_updated`.
- `apps/agent/internal/cameras/tether_supervisor.go`
  - Public listener fields are safe, but `Runtime` should not be over-emphasized in operator card; do not show raw port.
  - `LastDownloadedFileName` is extracted basename-only; still pass through a frontend basename helper.
- `apps/agent/internal/api/camera_test_capture.go`
  - Error details are allowlisted as status/action/file_name only.

### Safe UI Copy Suggestions

Use Indonesian operator copy:

- Assignment:
  - Assigned: `Kamera assigned: {camera_name}`
  - Unassigned: `Kamera belum di-assign`
- Connected:
  - Ready check success: `Camera connected`
  - Failed/warning: `Camera disconnected/needs attention`
- Listener:
  - `running`: `Listener running — shutter fisik akan didownload ke input folder station`
  - `stopped`: `Listener stopped`
  - `error`: `Listener error — gunakan Retry Listener atau ikuti next action`
  - `starting`: `Listener starting...`
  - `stopping`: `Listener stopping...`
- Actions:
  - `ASSIGN_CAMERA`: `Assign kamera di Station Settings`
  - `CONNECT_CAMERA`: `Hubungkan kamera / cek USB mode`
  - `CHECK_USBIPD`: `Cek USB attach ke WSL secara manual`
  - `CHECK_WSL`: `Cek WSL runtime`
  - `INSTALL_GPHOTO2`: `Install/aktifkan gPhoto2 secara manual`
  - `START_TETHER_LISTENER`: `Start tether listener`
  - `STOP_TETHER_LISTENER`: `Stop tether listener`
  - `RETRY_TETHER_LISTENER`: `Retry tether listener`
  - `CHECK_STATION_INPUT_FOLDER`: `Cek input folder station`
  - `RETRY_TEST_CAPTURE`: `Retry test capture`
  - `RECHECK_CAMERA_READINESS`: `Recheck camera readiness`

### Guardrails / Anti-Patterns

- Do not reanimate or copy old root `src/server/gphoto-helper.ts` spike implementation.
- Do not add browser-side calls to `gphoto2`, WSL, PowerShell, usbipd, filesystem APIs, or local path probes.
- Do not display raw camera port/path/identity/command/stdout/stderr.
- Do not run or expose privileged actions: `usbipd bind`, `usbipd attach`, install commands, driver/Zadig changes, global kill.
- Do not create/modify photo/session/processing/upload records from dashboard controls.
- Do not merge camera/tether status with Drive upload status.
- Do not break existing End Session confirmation.
- Do not claim real hardware validation unless actually performed.

## File References

### Must read before implementation

- `_bmad-output/planning-artifacts/epics.md`
- `_bmad-output/planning-artifacts/architecture.md`
- `_bmad-output/planning-artifacts/prd.md`
- `_bmad-output/implementation-artifacts/7-1-detect-and-assign-gphoto2-cameras-to-stations.md`
- `_bmad-output/implementation-artifacts/7-2-supervise-gphoto2-tether-listener-per-station.md`
- `_bmad-output/implementation-artifacts/7-3-add-camera-readiness-checks-and-test-capture-validation.md`
- `_bmad-output/implementation-artifacts/spec-gphoto-helper-diagnostic-spike.md`
- `apps/web/src/features/sessions/live-station-cards.tsx`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/features/stations/use-tether-listener.ts`
- `apps/web/src/features/stations/use-camera-test-capture-mutation.ts`
- `apps/web/src/features/stations/use-station-readiness-query.ts`
- `apps/web/src/features/stations/station-settings.tsx`
- `apps/web/src/lib/api/client.ts`
- `apps/agent/internal/api/camera_tether.go`
- `apps/agent/internal/api/camera_test_capture.go`
- `apps/agent/internal/cameras/tether_supervisor.go`
- `docs/api/openapi.yaml`

### Likely files to update/create

- `apps/web/src/features/sessions/live-station-cards.tsx` (UPDATE)
- `apps/web/src/features/sessions/camera-tether-status.tsx` (NEW optional/recommended)
- `apps/web/src/features/health/health-dashboard.tsx` (UPDATE SSE invalidation)
- `apps/web/src/features/stations/use-tether-listener.ts` (UPDATE invalidation if needed)
- `apps/web/src/lib/api/client.ts` (UPDATE only if helper/type gaps found)
- `apps/web/src/features/sessions/*.test.tsx` or equivalent (NEW optional if test setup exists)
- No backend file should be necessary unless an API/SSE bug is found.

## Testing Strategy

### Frontend validation

- Typecheck must catch DTO/action/status mistakes:
  - `cd apps/web && npm run typecheck`
  - `npm run typecheck`
- Build must pass:
  - `cd apps/web && npm run build`
- Manual UI checks in dev browser:
  - Station with no assignment shows `Unassigned` and action `ASSIGN_CAMERA`.
  - Station with assignment but disconnected readiness shows disconnected/attention safe action.
  - Listener stopped shows Start/Retry enabled and Stop disabled.
  - Listener running shows Stop enabled, Start disabled, last capture/file when available.
  - Listener error shows error label and Retry Listener.
  - Test capture mutation shows loading, success, and safe failure action.
  - Camera/tether status updates after SSE `camera.tether_listener_updated` or query refetch.
  - Existing session/customer/order/timer/quarantine/Drive controls still render.

### Security/safety checks

- Search rendered station card code to ensure it does not display:
  - `.port`
  - `.identity_key`
  - `.device_path`
  - `.bus_id`
  - raw `.input_folder` in camera/tether section
  - command strings or diagnostics.
- If displaying filenames, assert basename-only helper strips `\` and `/`.
- Errors should use safe message/action; do not stringify full `ApiError.details` object into UI.

### Backend validation (only if backend changed)

- `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`
- If Windows transient `Access is denied` occurs, rerun with a fresh `GOTMPDIR` and document both attempts.

## Project Context Reference

- Project: `selfstudio`.
- User: `alpharize`.
- Current date: 2026-05-20.
- Planning artifacts folder: `_bmad-output/planning-artifacts`.
- Implementation artifacts folder: `_bmad-output/implementation-artifacts`.
- Production architecture: Go agent (`apps/agent`) + Next.js web (`apps/web`) + local data under `local-data`.

## Dev Agent Record

### File List

- `apps/web/src/features/sessions/camera-tether-status.tsx` (new)
- `apps/web/src/features/sessions/live-station-cards.tsx`
- `apps/web/src/features/stations/use-tether-listener.ts`
- `apps/web/src/features/health/health-dashboard.tsx`
- `_bmad-output/implementation-artifacts/7-4-show-live-camera-tether-status-and-controls-in-dashboard.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`

### Debug Log

- 2026-05-20: Audit file wajib selesai: live station cards, health dashboard SSE, station hooks, API client, station settings, dan backend camera/readiness handlers.
- 2026-05-20: Menambahkan komponen Camera/Tether terpisah di station card dengan helper safe label, safe filename, safe action mapping, dan sanitasi error/message.
- 2026-05-20: Menambahkan controls Start/Retry/Stop listener dan Run test capture dengan pending/disabled states.
- 2026-05-20: Memperluas invalidasi mutation dan SSE untuk `camera.tether_listener_updated`.
- 2026-05-20: Frontend tidak memiliki test runner/component test script; manual coverage didokumentasikan lewat validasi typecheck/build dan audit grep safe rendering.
- 2026-05-20: Tidak ada file Go/backend diubah; Go test tidak dijalankan karena tidak berlaku.
- 2026-05-20: Review follow-up selesai: hasil test capture non-success kini menampilkan safe server action (`ASSIGN_CAMERA`, `STOP_TETHER_LISTENER`, `RECHECK_CAMERA_READINESS`, dll.) lewat mapping operator copy.
- 2026-05-20: Review follow-up selesai: sanitasi `listener.message` diperkuat untuk meredaksi raw camera port (`usb:001,004`), path Unix sensitif, identity/device/bus identifiers, stdout/stderr, credential labels, dan command-like diagnostics.
- 2026-05-20: Re-review patch selesai: sanitasi `listener.message` kini meredaksi `/Users`, `/Volumes`, `/workspace`, path Unix absolut generik, serta diagnostik shell seperti `bash -lc`, `sh -c`, `/bin/sh:`, dan `/usr/bin/env` tanpa memperluas redaksi ke copy UI Indonesia biasa.

### Change Log

- 2026-05-20: Story context prepared and expanded for development. Status set to `ready-for-dev`.
- 2026-05-20: Implemented live Camera/Tether station card status, safe controls, SSE invalidation, and validation updates. Status set to `review`.
- 2026-05-20: Addressed code review patch findings for safe test capture action display and stronger listener message sanitization. Status remains `review`.
- 2026-05-20: Addressed re-review patch finding for broader Unix path and shell diagnostic redaction in listener messages. Status set to `done` after final re-review.

### Validation Results

- `cd apps/web && npm run typecheck` — PASS.
- `cd apps/web && npm run build` — PASS.
- `npm run typecheck` — PASS.
- Safety audit: searched `apps/web/src/features/sessions/*.tsx` for raw camera port/identity/device/source path/command tokens and `dangerouslySetInnerHTML`; no rendered raw camera identifiers or dangerous HTML usage introduced. Matches are imports/helper sanitizers/action labels only.
- Review patch validation: `camera-tether-status.tsx` now appends safe mapped action copy for non-success test capture results and strengthens `safeText` redaction for `usb:001,004`, Unix paths, identity/device/bus labels, stdout/stderr, credentials, and command-like diagnostics.
- Final re-review validation: `camera-tether-status.tsx` broadens `safeText` redaction to cover `/Users/...`, `/Volumes/...`, `/workspace/...`, generic absolute Unix path-like tokens, and shell diagnostics including `bash -lc`, `sh -c`, `/bin/sh:`, `/usr/bin/env`, `/usr/bin/bash`, and related shell paths; normal Indonesian UI copy such as `Hubungkan kamera lalu Recheck` and `Assign kamera di Station Settings` remains intact.
- `cd apps/web && npm run typecheck` — PASS (re-run after re-review patch).
- `cd apps/web && npm run build` — PASS (re-run after re-review patch).
- `npm run typecheck` — PASS (re-run after re-review patch).
- Backend validation — not run; no Go/backend files changed.

## Completion Note

Implementation complete after re-review follow-up. Dashboard station cards now show a separate Camera/Tether section with safe assignment, connection, listener, last capture, last downloaded filename, and next action labels. Start/Retry/Stop listener and Run test capture controls use existing authenticated API client behavior, show pending states, and avoid raw port/path/identity/command/stdout/stderr display. Non-success test capture responses include the backend safe action mapped to Indonesian operator copy, so actions like `ASSIGN_CAMERA`, `STOP_TETHER_LISTENER`, and `RECHECK_CAMERA_READINESS` are not omitted. Listener message sanitization now redacts raw camera ports, Windows paths, selected and generic absolute Unix paths including `/Users`, `/Volumes`, and `/workspace`, identity/device/bus identifiers, stdout/stderr labels, credentials, and command-like shell diagnostics such as `bash -lc`, `sh -c`, `/bin/sh:`, and `/usr/bin/env`, while normal Indonesian UI copy remains intact. SSE invalidation handles `camera.tether_listener_updated`. Local processing/session details and Google Drive upload controls remain separate. Required frontend validations passed after the re-review patch.
