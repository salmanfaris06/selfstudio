# Story 7.5: Recover and Reconnect gPhoto2 Tethering During Event

Status: done

## Story

Sebagai operator, saya ingin camera tethering pulih aman dari disconnect, gPhoto2 process exit, USB/device change, dan app restart, sehingga capture event dapat lanjut tanpa duplicate ingestion, tanpa manual folder repair, dan tanpa menjalankan privileged USB/install/driver actions otomatis.

## Acceptance Criteria

1. **Disconnect/process-exit attention state**  
   Given kamera disconnect, gPhoto2 process exits unexpectedly, atau USB device/port berubah, when recovery observes the condition, then affected station enters camera/tether attention state with safe next action while existing session APIs, folder watcher, ingestion, processing, quarantine, and Drive upload continue where possible.

2. **Startup desired-state recovery**  
   Given app/agent restarts, when startup recovery runs, then persisted listener desired state is restored and listener restart is attempted only for stations whose listener was previously desired running and whose auto-restart setting is enabled.

3. **No privileged reconnect/install/driver actions**  
   Given reconnect would require `usbipd bind`, `usbipd attach`, driver/Zadig change, package install (`winget`, `choco`, `apt install`), WSL setup, or elevated action, when recovery evaluates next steps, then system never runs those commands automatically and instead returns/logs manual next action such as `CHECK_USBIPD`, `INSTALL_GPHOTO2`, `CHECK_WSL`, `CONNECT_CAMERA`, or `CHECK_CAMERA_USB_MODE`.

4. **Same input folder and duplicate-safe ingestion**  
   Given reconnect succeeds, when listener resumes, then downloaded JPGs continue to be written only to the same configured `station.input_folder`, and existing folder watcher/stable-file/idempotency pipeline remains the only ingestion source of truth so duplicate filesystem events or restarted listener output do not create duplicate photo/session/processing/upload records.

5. **Backoff and safe failure summary**  
   Given reconnect fails repeatedly, when auto-recovery continues, then exponential or bounded backoff prevents tight restart loops, max attempts/window are enforced, station remains actionable, and activity log records sanitized reconnect attempt/result/failure summary without raw shell output, raw paths, raw port/identity diagnostics, tokens, or privileged command output.

6. **Manual retry from dashboard/API**  
   Given operator clicks retry reconnect/listener from dashboard, when station is in stopped/error/attention state, then authenticated trusted-origin API attempts only safe discovery/listener restart behavior, respects backoff/idempotency/duplicate-start constraints, and surfaces safe message/action in station card.

7. **Tests/build/docs pass**  
   Tests/build pass and API/OpenAPI/docs are updated for desired-state persistence, auto-restart setting, recovery status, manual retry semantics, safe actions, and no-privileged-action guarantee.

## Acceptance Criteria Context / BDD Detail

### AC1 — Disconnect/process-exit moves station to attention without blocking event flow

- Story 7.2 already detects unexpected listener process exit in `TetherSupervisor.monitor` and marks `TetherStatusError` with `LastErrorCode = TETHER_LISTENER_EXITED` and `LastErrorAction = RETRY_TETHER_LISTENER`.
- Story 7.5 must build on that instead of creating a parallel process manager.
- A gPhoto2 listener failure is a **camera/tether attention** condition only. It must not mark active sessions, processing jobs, ingestion scanner, local save, quarantine, or Drive upload as failed by itself.
- Existing Go APIs for session start/end, ingestion scan, processing retry, quarantine, and Drive upload must remain responsive while recovery/backoff is running.
- Station card already has a separate Camera/Tether section from Story 7.4. Keep this separation: do not merge camera attention with local processing or Drive upload statuses.
- Expected visible state examples:
  - listener `error`, last error action `RETRY_TETHER_LISTENER` or `CONNECT_CAMERA`;
  - readiness `tether_listener` warning/failed depending `SELFSTUDIO_CAMERA_READINESS_REQUIRED`;
  - dashboard next action in Indonesian operator copy.
- Safe activity examples:
  - `station.tether_listener_exited`
  - `station.tether_reconnect_scheduled`
  - `station.tether_reconnect_attempted`
  - `station.tether_reconnect_failed`
  - `station.tether_reconnect_succeeded`
- Public messages must be sanitized with a stronger policy than raw gPhoto stdout/stderr. Prefer allowlisted code/message/action over passing diagnostics through.

### AC2 — Persist listener desired state and auto-restart setting across restart

- Current listener state in `TetherSupervisor` is in-memory only. Story 7.5 must persist **desired listener state**, not OS process handles.
- Add durable local config under Go agent/local-data, preferably one of:
  - extend station config with non-secret camera/tether operational settings, or
  - new persisted file such as `local-data/config/tether_listeners.json` using temp file + atomic rename pattern similar to `apps/agent/internal/stations/persistence.go`.
- Persisted data minimum per station:
  - `station_id`
  - `desired_state`: `running` or `stopped`
  - `auto_restart_enabled`: boolean
  - `last_started_at`, `last_stopped_at` optional
  - `last_recovery_attempt_at` optional
  - `recovery_attempt_count` or window metadata optional
  - `updated_at`
- Desired state rules:
  - Successful Start Listener sets `desired_state=running`.
  - Successful Stop Listener sets `desired_state=stopped` and should cancel pending auto-recovery for that station.
  - Duplicate start no-op keeps `desired_state=running`.
  - Unexpected process exit must not immediately set desired state to stopped; it should remain running so auto-recovery may decide whether to restart.
  - If auto-restart is disabled, unexpected exit remains attention/error but no automatic restart is attempted.
- Auto-restart setting:
  - Must default safe. Recommended default: `false` unless product/user explicitly wants previous starts auto-recovered. Because AC says restart only when enabled, do not infer enabled from assignment alone.
  - Expose setting through API/UI if needed. Minimal viable: API accepts `auto_restart_enabled` update and dashboard/settings show/control it.
  - Do not store secrets.
- Startup recovery:
  - On mux/service startup (or a startup hook in `cmd/selfstudio-agent/main.go`), load persisted desired states.
  - For each station with `desired_state=running` and `auto_restart_enabled=true`, validate station config/assignment/input folder and schedule safe restart through the same supervisor/recovery path.
  - Recovery should not block server boot for long-running gPhoto attempts. Schedule asynchronously and publish safe activity/SSE.

### AC3 — No privileged reconnect/install/driver/usbipd actions

- Allowed automatic actions:
  - read persisted station/tether settings;
  - read-only gPhoto2 discovery (`gphoto2 --auto-detect`, WSL equivalent);
  - read-only `usbipd list` diagnostic from Story 7.1 path;
  - start the assigned station listener using existing safe allowlisted command builder;
  - stop/kill only the process owned by this station supervisor;
  - update local desired/recovery metadata, SSE, and activity log.
- Forbidden automatic actions:
  - `usbipd bind`, `usbipd attach`, `usbipd detach`;
  - `winget`, `choco`, `apt install`, dependency install/setup;
  - driver/Zadig/registry changes;
  - `taskkill /IM gphoto2.exe`, `pkill gphoto2`, global WSL/gPhoto process kill;
  - deleting camera files;
  - deleting station input/output/quarantine/local result files;
  - direct Drive upload or Drive credential check as part of camera reconnect.
- If discovery or start failure suggests manual privileged setup, response/activity should store action only, e.g. `CHECK_USBIPD`, and safe Indonesian label. It may include a manual command hint only if generated from a strict allowlist and sanitized; for this story, prefer no command hints on dashboard.
- Add regression/static tests that recovery package/supervisor does not build command specs containing forbidden tokens (`usbipd bind`, `attach`, `winget`, `choco`, `apt`, `powershell`, `cmd /C`, `taskkill`, `pkill`).

### AC4 — Reconnect resumes same input folder and watcher dedupe remains authoritative

- Listener restart must use current persisted `station.input_folder` and `station.camera_assignment`; do not accept path/runtime/port override from request body.
- The existing `BuildTetherCommand` path confinement must remain in force. If changed, preserve tests for:
  - filename pattern inside input folder;
  - WSL path conversion;
  - invalid port rejection;
  - no shell metacharacters.
- Recovery must not import or call photo/session/processing/upload stores directly. It may only start listener that writes JPG to input folder.
- Existing ingestion idempotency is based on station/source path/size/mtime patterns from Epic 4. Story 7.5 should add or preserve tests proving restart/recovery does not directly create duplicate photo records.
- If a listener restarts and gPhoto2 re-downloads a camera file with same or similar filename, filename pattern still must be collision-safe (`%Y%m%d_%H%M%S_%03n.%C`) and ingestion dedupe should handle duplicate filesystem events. Do not add overwrite flags.
- Do not repair folders by moving/deleting files. If input folder missing/unwritable, return `CHECK_STATION_INPUT_FOLDER` and attention state.

### AC5 — Repeated failures use backoff and safe failure summaries

- Implement per-station recovery/backoff guard in Go. Suggested package/file:
  - `apps/agent/internal/cameras/tether_recovery.go`
  - `apps/agent/internal/cameras/tether_recovery_test.go`
- Suggested policy:
  - immediate or short first attempt after unexpected exit if auto-restart enabled;
  - exponential backoff with caps, e.g. 1s, 2s, 5s, 10s, 30s max (tests may use injected clock/timer);
  - max attempts per window, e.g. 5 attempts in 5 minutes before pausing to operator attention;
  - manual retry can bypass or reset delay only if safe and authenticated, but must still not duplicate process.
- Make policy configurable in code constants or non-secret config, but keep MVP simple and testable.
- Tight loop prevention is mandatory: no goroutine loop that immediately calls `Start` repeatedly while camera remains disconnected.
- Safe summary fields:
  - `station_id`
  - `status`: `scheduled|attempting|succeeded|failed|paused`
  - `attempt_count`
  - `next_attempt_at` optional
  - `last_error_code`
  - `last_error_action`
  - `message` safe only
- SSE event suggestion: `camera.tether_recovery_updated` with `entity_type=station`.
- Activity log must not include raw stdout/stderr. Use summary counts and safe actions only.

### AC6 — Manual retry from dashboard/API

- Existing Story 7.4 Start/Retry button already calls `POST /api/stations/{station_id}/tether-listener/start`. For Story 7.5, this may remain the manual retry endpoint if it updates desired state/backoff correctly.
- If adding explicit retry endpoint, suggested route:
  - `POST /api/stations/{station_id}/tether-listener/retry`
  - protected by `RequireAuth` + `RequireTrustedOrigin`.
- Manual retry behavior:
  - validates station scope;
  - reads persisted station assignment/input folder only;
  - cancels/clears paused backoff for that station if safe;
  - no-op if already running;
  - attempts safe listener start through supervisor;
  - updates desired state only according to user intent (retry/start = running desired; stop = stopped desired);
  - records safe activity/SSE.
- Dashboard should show auto-restart enabled/disabled and recovery/backoff state if available. Minimal UI: status text and action copy in Camera/Tether section or Station Settings.

### AC7 — Docs/API/tests/build

- Update `docs/api/openapi.yaml` for any new/changed fields/endpoints:
  - tether listener desired state / auto restart status;
  - optional recovery status response;
  - retry/start/stop semantics;
  - error actions and no privileged actions.
- Required validations after implementation:
  - `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...` (use fresh alternate GOTMPDIR and document Windows transient failures honestly if needed)
  - `cd apps/web && npm run typecheck`
  - `cd apps/web && npm run build`
  - `npm run typecheck`
- Do not mark pass without executing. If real camera hardware is not available, state hardware validation not run.

## Tasks / Subtasks

- [x] Audit current implementation and preserve boundaries
  - [x] Read `apps/agent/internal/cameras/tether_supervisor.go`, `tether_supervisor_test.go`, `gphoto_runner.go`, `types.go`.
  - [x] Read `apps/agent/internal/api/camera_tether.go`, `camera_tether_test.go`, `health.go`, `response.go`, `security.go`.
  - [x] Read `apps/agent/internal/stations/store.go`, `persistence.go`, `readiness.go`.
  - [x] Read `apps/agent/internal/recovery/recovery.go` and `apps/agent/internal/processing/recovery.go` for existing recovery patterns but keep camera recovery separate.
  - [x] Read `apps/web/src/features/sessions/camera-tether-status.tsx`, `apps/web/src/features/stations/use-tether-listener.ts`, `apps/web/src/features/health/health-dashboard.tsx`, `apps/web/src/lib/api/client.ts`.
  - [x] Read previous Stories 7.1-7.4 and spike specs listed in File References.
- [x] Design/persist tether desired state and auto-restart setting
  - [x] Add durable non-secret store for per-station listener desired state and `auto_restart_enabled` using atomic temp+rename.
  - [x] Backward/default load handles missing file by treating all stations as `desired_state=stopped`, `auto_restart_enabled=false`.
  - [x] Start/stop/manual retry update desired state consistently.
  - [x] Add unit tests for load/save/backward missing file/corrupt file handling if implemented.
- [x] Implement recovery coordinator/backoff
  - [x] Add per-station recovery coordinator with injected clock/timer/test hooks.
  - [x] Subscribe or callback from `TetherSupervisor` unexpected exit, or expose a safe hook that API/main can wire without polling tight loops.
  - [x] On unexpected exit: mark attention/error, record safe activity, schedule restart only if desired running + auto-restart enabled.
  - [x] On startup: load desired states and schedule safe restart only for enabled stations.
  - [x] Enforce max attempts/backoff cap and paused attention state.
  - [x] Ensure Stop cancels pending recovery and sets desired stopped.
- [x] Extend API contracts safely
  - [x] Add or extend status endpoint to include `desired_state`, `auto_restart_enabled`, and recovery summary/backoff state.
  - [x] Add endpoint to update auto-restart setting if needed, e.g. `PUT /api/stations/{station_id}/tether-listener/settings`.
  - [x] Optionally add explicit `POST /api/stations/{station_id}/tether-listener/retry`; otherwise make start endpoint document manual retry semantics.
  - [x] Protect all mutations with auth + trusted origin.
  - [x] Return `{data}` wrapper and `{error:{code,message,action,details}}` errors only.
  - [x] Update activity log and SSE with safe messages.
- [x] Preserve no-privileged-actions and no direct ingestion
  - [x] Recovery uses existing safe discovery/`usbipd list` read-only and `TetherSupervisor.Start` only.
  - [x] Add tests/static guard that camera recovery does not execute or construct forbidden privileged commands.
  - [x] Add tests/static guard that recovery/tether packages do not import `internal/photos`, `internal/sessions`, `internal/processing`, `internal/upload`, or create photo/upload/session records.
  - [x] Ensure station input folder path comes from persisted station config only.
- [x] Frontend/dashboard integration
  - [x] Update API client types/functions for desired state, auto-restart setting, recovery status, and retry/settings endpoint if added.
  - [x] Update Camera/Tether section to show auto-restart and recovery/backoff status without raw diagnostics.
  - [x] Add toggle/control for auto-restart if endpoint added; pending/disabled states required.
  - [x] Update SSE invalidation for `camera.tether_recovery_updated` if added.
  - [x] Preserve sanitized `safeText`, `safeFileName`, and no raw port/identity/path display.
- [x] OpenAPI/docs
  - [x] Update `docs/api/openapi.yaml` for settings/recovery/desired state schemas and endpoints.
  - [x] Document startup recovery, backoff, manual retry, and no privileged usbipd/install/driver actions.
  - [x] Document that watcher remains ingestion source of truth and reconnect never directly creates photo/session/upload records.
- [x] Backend tests
  - [x] Desired-state persistence: start sets running desired; stop sets stopped; startup loads previous values; missing file defaults safe.
  - [x] Auto-restart: enabled+desired running schedules restart; disabled does not; desired stopped does not.
  - [x] Unexpected exit: station status error/attention, activity safe, recovery scheduled only when eligible.
  - [x] Backoff: repeated failures do not tight-loop; next attempt delay increases/caps; max/window pauses.
  - [x] Manual retry: authenticated trusted-origin, duplicate start no-op, resets/overrides backoff safely.
  - [x] No privileged command specs/tokens constructed.
  - [x] No direct ingestion/photo/session/processing/upload imports or calls.
  - [x] API tests for auth, trusted origin, response wrapper, station scope, safe details, settings persistence.
  - [x] Regression: existing Story 7.1 discovery, Story 7.2 tether start/stop, Story 7.3 test capture/readiness, and Story 7.4 dashboard API assumptions still pass.
- [x] Frontend/type/build validation
  - [x] TypeScript compile for new DTOs/actions/status.
  - [x] Dashboard handles recovery scheduled/paused/failed/succeeded and auto-restart enabled/disabled.
  - [x] Run required validation commands and record actual results.

## Dev Notes

### Epic 7 Context

Epic 7 adds managed gPhoto2 tethering but keeps the original selfstudio safety model: camera/tether writes JPG files into a station input folder, and the existing folder watcher/stable detection/ingestion pipeline remains authoritative. Story 7.5 is a recovery hardening story for live events. It must recover safely from listener exits, disconnects, USB changes, and app restart without expanding privilege, bypassing ingestion, or endangering customer files.

### Current Architecture That Must Be Followed

- Frontend: `apps/web`, Next.js App Router, TypeScript, TanStack Query, Tailwind/shadcn.
- Backend/local worker: Go service in `apps/agent`; all filesystem/process/credentials/local mutations live in Go.
- API: REST under `/api`, SSE under `/events`, success wrapper `{data}`, error wrapper `{error:{code,message,action,details}}`, JSON `snake_case`.
- Auth/security: local PIN/session cookie; state-changing endpoints require `RequireAuth` + `RequireTrustedOrigin`.
- Events: dot notation with existing event wrapper.
- Browser must never execute commands, inspect local filesystem, run usbipd, or receive raw diagnostics/secrets.
- Local processing and Google Drive upload remain separate from camera/tether recovery.

### Current Code Intelligence / Current State

#### `apps/agent/internal/cameras/tether_supervisor.go`

- Defines `TetherStatus`: `stopped`, `starting`, `running`, `stopping`, `error`.
- `TetherSupervisor` is in-memory with `listeners map[string]*tetherRuntime`.
- `Start` validates station id, assignment, input folder writable, builds command, prevents duplicate process, and starts one process per station.
- `Stop` cancels only station-owned process and deletes runtime.
- `monitor` marks unexpected process exit as `TetherStatusError`, `LastErrorCode=TETHER_LISTENER_EXITED`, `LastErrorAction=RETRY_TETHER_LISTENER`, message `Tether listener exited unexpectedly.`
- There is no persisted desired state, no auto-restart setting, and no backoff coordinator yet.
- Existing review fix: stop during startup race kills late-started process. Preserve that regression.
- `BuildTetherCommand` uses assigned port/runtime and `BuildConfinedFilenamePattern`; do not let recovery bypass this.
- Current `SanitizeTetherDiagnostic` redacts Windows paths and `/mnt` paths but frontend Story 7.4 adds stronger redaction. If backend recovery logs new messages, prefer safe allowlisted messages rather than relying only on sanitizer.

#### `apps/agent/internal/api/camera_tether.go`

- Existing routes:
  - `GET /api/stations/{station_id}/tether-listener`
  - `POST /api/stations/{station_id}/tether-listener/start`
  - `POST /api/stations/{station_id}/tether-listener/stop`
- Start/stop publish `camera.tether_listener_updated` and record activity.
- `writeStartError` currently exposes `map[string]any{"listener": listener}` details. Listener should be sanitized; if adding fields, ensure no raw diagnostics. Consider allowlisting details in this story if touching error behavior.
- Unknown station ids are validated against store (Story 7.2 review fix). Preserve.

#### `apps/agent/internal/api/health.go`

- `NewMuxWithUploadsAndCamera` wires singleton `tetherSupervisor` and `testCaptureService` when injected.
- Startup recovery will likely need wiring here or in `cmd/selfstudio-agent/main.go` so it uses the same supervisor instance as API/readiness.
- Avoid creating a second `TetherSupervisor`; recovery must share the singleton used by dashboard/readiness.

#### `apps/agent/internal/stations/readiness.go`

- Camera checks include `camera_assignment`, `gphoto2_availability`, `camera_connected`, `tether_listener`, `input_folder_writable`, `camera_test_capture`.
- `tether_listener` currently reads `TetherStatusProvider.Status(stationID)` and expects `running` for ready; stopped/error becomes warning/failed depending `SELFSTUDIO_CAMERA_READINESS_REQUIRED`.
- Story 7.5 may add recovery status but should not break readiness check keys required by web type guards.

#### `apps/web/src/features/sessions/camera-tether-status.tsx`

- Station card shows assignment, connection, listener status, last capture, last downloaded filename, next action, and controls.
- It includes strong frontend redaction for listener messages and safe action mapping.
- If new action codes are added, extend `cameraActionLabel`.
- Do not render raw `port`, `identity_key`, `device_path`, `bus_id`, absolute paths, commands, stdout/stderr.

#### `apps/web/src/features/stations/use-tether-listener.ts`

- Query key: `['tether-listener', stationId]`.
- Mutations update tether query and invalidate station readiness, stations, and activity.
- If response schema changes, update this hook and API client together.

#### Existing recovery packages

- `apps/agent/internal/recovery/recovery.go` handles ingestion startup reconciliation.
- `apps/agent/internal/processing/recovery.go` handles Epic 5 processing recovery.
- Use their style for safe summaries/activity/SSE inspiration only. Camera/tether recovery must remain separate and must not recover processing/upload jobs.

### Completed Story Intelligence

#### Story 7.1 — Discovery and assignment

- Safe gPhoto2 discovery and station camera assignment are implemented in Go.
- `usbipd list` read-only diagnostic is allowed; `bind`/`attach` are not.
- Duplicate camera assignment validation exists.
- UI and API review previously caught raw port/identity display. Do not regress.

#### Story 7.2 — Tether listener supervisor

- Supervised per-station listener exists.
- Duplicate start is safe no-op.
- Stop is station-specific; no global kill.
- Review found and fixed startup/stop race and unknown station validation. Preserve tests.
- Listener does not call ingestion/photo/session/processing/upload packages.

#### Story 7.3 — Camera readiness and test capture

- Camera readiness can be optional/required via `SELFSTUDIO_CAMERA_READINESS_REQUIRED`.
- Test capture is explicit operator action, writes expected JPG to station input folder, validates JPEG SOI and watcher path, and does not call Drive/upload/photo/session directly.
- Review fixed timeout, raw port/path UI, collision-safe filename, and command allowlist.

#### Story 7.4 — Live dashboard Camera/Tether UI

- Station cards now show Camera/Tether as separate section.
- SSE invalidates on `camera.tether_listener_updated`.
- Strong frontend message redaction was added after review/re-review.
- No backend files changed in Story 7.4.

### Spike Specs and Lessons to Reuse Carefully

- `_bmad-output/implementation-artifacts/spec-gphoto-helper-diagnostic-spike.md`
  - Historical TS prototype had one-click setup and trigger listener. Production backend is now Go; do not copy `src/server/*` into production.
  - Useful lesson: physical shutter listener can use gPhoto wait-event/download, but setup/usbipd actions must be human/manual.
  - Spike ran `usbipd bind/attach`; Story 7.5 must not.
- `_bmad-output/implementation-artifacts/spec-direct-camera-capture-spike.md`
  - Do not claim camera control/reconnect success until capability/discovery proves it.
  - Keep folder watcher as source of truth.
  - Reject concurrent unsafe operations.
- `_bmad-output/implementation-artifacts/spec-usb-camera-detection-spike.md`
  - OS-level USB detection is not proof of gPhoto2 tether readiness.
  - Render device data safely.

### Suggested Production Design

Recommended new backend components:

```text
apps/agent/internal/cameras/tether_state_store.go       # desired state + auto-restart persistence
apps/agent/internal/cameras/tether_state_store_test.go
apps/agent/internal/cameras/tether_recovery.go          # coordinator/backoff/startup recovery
apps/agent/internal/cameras/tether_recovery_test.go
apps/agent/internal/api/camera_tether_settings.go       # optional settings/retry endpoints or extend camera_tether.go
apps/agent/internal/api/camera_tether_settings_test.go
```

Suggested DTOs:

```go
type TetherDesiredState string
const (
    TetherDesiredStopped TetherDesiredState = "stopped"
    TetherDesiredRunning TetherDesiredState = "running"
)

type TetherListenerSettings struct {
    StationID           string             `json:"station_id"`
    DesiredState        TetherDesiredState `json:"desired_state"`
    AutoRestartEnabled  bool               `json:"auto_restart_enabled"`
    LastStartedAt       *time.Time         `json:"last_started_at,omitempty"`
    LastStoppedAt       *time.Time         `json:"last_stopped_at,omitempty"`
    UpdatedAt           time.Time          `json:"updated_at"`
}

type TetherRecoveryStatus struct {
    StationID        string     `json:"station_id"`
    Status           string     `json:"status"` // idle|scheduled|attempting|succeeded|failed|paused
    AttemptCount     int        `json:"attempt_count"`
    NextAttemptAt    *time.Time `json:"next_attempt_at,omitempty"`
    LastErrorCode    string     `json:"last_error_code,omitempty"`
    LastErrorAction  SafeAction `json:"last_error_action,omitempty"`
    Message          string     `json:"message"`
    UpdatedAt        time.Time  `json:"updated_at"`
}
```

Suggested API options:

- Extend `GET /api/stations/{station_id}/tether-listener` response:
  ```json
  {
    "data": {
      "listener": { ... },
      "settings": {
        "desired_state": "running",
        "auto_restart_enabled": true
      },
      "recovery": {
        "status": "scheduled",
        "attempt_count": 2,
        "next_attempt_at": "2026-05-20T...Z",
        "last_error_action": "CHECK_USBIPD",
        "message": "Reconnect dijadwalkan dengan backoff."
      }
    }
  }
  ```
- Add `PUT /api/stations/{station_id}/tether-listener/settings` body `{ "auto_restart_enabled": true }`.
- Add `POST /api/stations/{station_id}/tether-listener/retry` only if start endpoint cannot cleanly represent manual retry.

### File References

#### Must read before implementation

- `_bmad-output/planning-artifacts/epics.md`
- `_bmad-output/planning-artifacts/architecture.md`
- `_bmad-output/planning-artifacts/prd.md`
- `_bmad-output/implementation-artifacts/7-1-detect-and-assign-gphoto2-cameras-to-stations.md`
- `_bmad-output/implementation-artifacts/7-2-supervise-gphoto2-tether-listener-per-station.md`
- `_bmad-output/implementation-artifacts/7-3-add-camera-readiness-checks-and-test-capture-validation.md`
- `_bmad-output/implementation-artifacts/7-4-show-live-camera-tether-status-and-controls-in-dashboard.md`
- `_bmad-output/implementation-artifacts/spec-gphoto-helper-diagnostic-spike.md`
- `_bmad-output/implementation-artifacts/spec-direct-camera-capture-spike.md`
- `_bmad-output/implementation-artifacts/spec-usb-camera-detection-spike.md`
- `apps/agent/internal/cameras/tether_supervisor.go`
- `apps/agent/internal/cameras/tether_supervisor_test.go`
- `apps/agent/internal/cameras/gphoto_runner.go`
- `apps/agent/internal/cameras/types.go`
- `apps/agent/internal/cameras/readiness_adapter.go`
- `apps/agent/internal/api/camera_tether.go`
- `apps/agent/internal/api/camera_tether_test.go`
- `apps/agent/internal/api/camera_test_capture.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/stations/store.go`
- `apps/agent/internal/stations/persistence.go`
- `apps/agent/internal/stations/readiness.go`
- `apps/agent/internal/recovery/recovery.go`
- `apps/agent/internal/processing/recovery.go`
- `apps/web/src/features/sessions/camera-tether-status.tsx`
- `apps/web/src/features/stations/use-tether-listener.ts`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`

#### Likely files to create/update

- `apps/agent/internal/cameras/tether_state_store.go` (NEW)
- `apps/agent/internal/cameras/tether_state_store_test.go` (NEW)
- `apps/agent/internal/cameras/tether_recovery.go` (NEW)
- `apps/agent/internal/cameras/tether_recovery_test.go` (NEW)
- `apps/agent/internal/cameras/tether_supervisor.go` (UPDATE: exit callback/hook, desired-state integration if needed)
- `apps/agent/internal/cameras/tether_supervisor_test.go` (UPDATE)
- `apps/agent/internal/api/camera_tether.go` (UPDATE response/settings/retry/recovery publish)
- `apps/agent/internal/api/camera_tether_test.go` (UPDATE)
- `apps/agent/internal/api/health.go` (UPDATE wiring singleton recovery/state store)
- `apps/agent/cmd/selfstudio-agent/main.go` (UPDATE startup recovery/shutdown hook if needed)
- `apps/web/src/lib/api/client.ts` (UPDATE DTOs/functions)
- `apps/web/src/features/stations/use-tether-listener.ts` (UPDATE query/mutations/invalidation)
- `apps/web/src/features/sessions/camera-tether-status.tsx` (UPDATE UI for auto-restart/recovery)
- `apps/web/src/features/health/health-dashboard.tsx` (UPDATE SSE invalidation if `camera.tether_recovery_updated` added)
- `docs/api/openapi.yaml` (UPDATE)

### Testing Strategy

#### Backend unit/API tests

1. Desired state persistence:
   - missing file loads safe defaults;
   - start/manual retry sets `desired_state=running`;
   - stop sets `desired_state=stopped` and cancels pending recovery;
   - `auto_restart_enabled` persists and reloads;
   - write uses temp+atomic rename and does not store secrets.
2. Startup recovery:
   - desired running + auto restart enabled -> safe restart scheduled/attempted;
   - desired running + auto restart disabled -> no restart, status attention/stopped;
   - desired stopped -> no restart;
   - missing assignment/input folder -> no privileged action, safe failure action.
3. Unexpected exit recovery:
   - process exit triggers error/attention and recovery schedule only when eligible;
   - existing active sessions remain untouched;
   - activity/SSE summary safe.
4. Backoff:
   - repeated failures increase delay;
   - max delay cap enforced;
   - max attempts/window pauses;
   - no tight loop under immediate failures;
   - manual retry is safe and does not spawn duplicate listener.
5. Security/no privileged actions:
   - recovery never constructs forbidden command args/tokens;
   - only read-only `usbipd list` may appear through discovery;
   - no `bind`, `attach`, `install`, `taskkill`, `pkill`, shell concatenation.
6. Boundary/static guard:
   - camera recovery/tether package does not import `internal/photos`, `internal/sessions`, `internal/processing`, `internal/upload`;
   - listener/recovery does not call ingestion router/scanner directly.
7. API tests:
   - auth required for status/settings/retry;
   - trusted origin required for mutations;
   - invalid station returns safe `STATION_NOT_FOUND`;
   - response wrapper includes settings/recovery if added;
   - errors include safe code/message/action/details only;
   - no raw command/path/port/identity leaks.

#### Frontend validation

- Typecheck/build:
  - `cd apps/web && npm run typecheck`
  - `cd apps/web && npm run build`
  - `npm run typecheck`
- UI states:
  - auto-restart disabled and stopped;
  - auto-restart enabled desired running;
  - recovery scheduled with next attempt;
  - recovery paused after failures;
  - manual retry pending/success/error;
  - no raw diagnostic rendering.

#### Manual/hardware validation after implementation

- No real camera: listener start/recovery fails safely with `INSTALL_GPHOTO2`/`CHECK_WSL`/`CONNECT_CAMERA`, no crash, no loop.
- WSL gPhoto2 but camera not attached: auto recovery pauses/backoffs with `CHECK_USBIPD`, no `usbipd bind/attach` executed.
- Camera connected: start listener, unplug camera/process exits, station shows attention, reconnect camera, manual retry or auto-restart resumes listener.
- App restart with auto-restart enabled: listener desired running is restored and safe restart attempted.
- App restart with auto-restart disabled: no automatic listener restart.
- Capture during recovery: existing session/ingestion/processing remains available; no duplicate photo records from listener recovery.

### Regression Risks to Avoid

- Auto-running privileged USB/setup/install/driver commands.
- Tight infinite restart loop while camera disconnected.
- Creating a second `TetherSupervisor` instance so dashboard/readiness/recovery disagree.
- Setting desired state to stopped on unexpected exit, which would prevent intended auto-recovery.
- Restarting listeners for every assigned station on startup regardless of `auto_restart_enabled`.
- Starting duplicate listener processes under auto-recovery + manual retry race.
- Killing global gPhoto2/WSL processes.
- Bypassing folder watcher by creating photo/session/upload records directly from recovery.
- Deleting camera files or local input/output/quarantine/result files during recovery.
- Exposing raw port/identity/path/stdout/stderr/command details in API/SSE/activity/UI.
- Regressing Story 7.4 message sanitization.
- Blocking dashboard/session APIs during recovery attempts.

### Recent Git / Project Intelligence

Recent git history is sparse and does not represent all current implementation detail:

- `c5ab05b Add from-scratch setup guide`
- `1dbc6cb Initial Selfstudio camera capture spike`

Most relevant code exists in working tree files from Stories 7.1-7.4, so developer must inspect current files directly rather than infer from commits.

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
- `apps/agent/internal/api/camera_tether.go`
- `apps/agent/internal/api/camera_tether_test.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/tether_recovery_notifier.go`
- `apps/agent/internal/cameras/tether_recovery.go`
- `apps/agent/internal/cameras/tether_recovery_test.go`
- `apps/agent/internal/cameras/tether_state_store.go`
- `apps/agent/internal/cameras/tether_state_store_test.go`
- `apps/agent/internal/cameras/tether_supervisor.go`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/features/sessions/camera-tether-status.tsx`
- `apps/web/src/features/stations/use-tether-listener.ts`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`
- `docs/tether-recovery.md`
- `_bmad-output/implementation-artifacts/7-5-recover-and-reconnect-gphoto2-tethering-during-event.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`

### Debug Log

- 2026-05-20: Loaded workflow/config/story context; sprint status moved to `in-progress`.
- 2026-05-20: Resumed Story 7.5 after AI code review with 2 blocking findings around auto-restart cancellation/gating and manual retry one-shot semantics.
- 2026-05-20: Added failing regression tests proving scheduled automatic attempts must re-read persisted desired state/auto-restart before `Supervisor.Start`, and manual retry failure must not schedule automatic backoff when auto-restart is disabled.
- 2026-05-20: Updated recovery coordinator so scheduled automatic attempts re-check persisted settings immediately before start, failed attempts reschedule only when persisted `desired_state=running` and `auto_restart_enabled=true`, and manual retry failure is tracked as actionable failed state without automatic continuation when disabled.
- 2026-05-20: Updated settings API so disabling `auto_restart_enabled` cancels pending recovery timer/state for that station.
- 2026-05-20: Audited current camera/tether supervisor/API/readiness/recovery/frontend files and prior Story 7.1-7.4/spike context.
- 2026-05-20: Added persisted tether listener settings store with safe missing-file defaults and corrupt-file handling.
- 2026-05-20: Added per-station recovery coordinator with desired-state/auto-restart gating, injected timers, bounded backoff, max attempt pause, startup scheduling, manual retry, and stop cancellation.
- 2026-05-20: Added supervisor unexpected-exit hook without creating a second process manager.
- 2026-05-20: Extended API response with settings/recovery, added settings and retry endpoints, sanitized error details, and wired activity/SSE notifier.
- 2026-05-20: Wired main startup recovery using the same singleton supervisor as API/readiness.
- 2026-05-20: Updated frontend DTOs/hooks/dashboard Camera/Tether UI for auto-restart, recovery status, retry endpoint, and SSE invalidation.
- 2026-05-20: Updated OpenAPI and recovery docs with no-privileged-action and ingestion-boundary guarantees.
- 2026-05-20: Initial focused `go test ./internal/api` hit a Windows temp cleanup transient in unrelated upload test; reran required full suite successfully with fresh GOTMPDIR.
- 2026-05-20: Hardware validation with real gPhoto2 camera was not run in this environment.

### Change Log

- 2026-05-20: Story context prepared and expanded for development. Status set to `ready-for-dev`.
- 2026-05-20: Implemented safe gPhoto2 tether desired-state persistence, auto-restart setting, bounded recovery/backoff, manual retry/settings API, dashboard recovery controls, OpenAPI/docs, and validation tests. Status set to `review`.
- 2026-05-20: Addressed AI code review blocking findings: auto-restart disable now cancels pending recovery and scheduled automatic attempts are gated by freshly persisted desired settings; manual retry remains one-shot while auto-restart is disabled.
- 2026-05-20: Re-review confirmed follow-up fixes and no new recovery/privilege/ingestion regressions. Status set to `done`.

### Validation Results

- PASS: `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`
- PASS: `cd apps/web && npm run typecheck`
- PASS: `cd apps/web && npm run build`
- PASS: `npm run typecheck`
- PASS: Focused TDD checks during development: `cd apps/agent && GOTMPDIR=../../.gotmp-red go test ./internal/cameras`, `cd apps/agent && GOTMPDIR=../../.gotmp-red go test ./internal/api -run 'TestTetherAPI'`.
- PASS: Review-fix RED check failed as expected before implementation: `cd apps/agent && GOTMPDIR=../../.gotmp-tdd go test ./internal/cameras -run 'TestRecoveryScheduledAttemptRechecksSettingsBeforeStart|TestRecoveryManualRetryFailureIsOneShotWhenAutoRestartDisabled'`.
- PASS: Review-fix focused backend validation after implementation: `cd apps/agent && GOTMPDIR=../../.gotmp-tdd2 go test ./internal/cameras` and `cd apps/agent && GOTMPDIR=../../.gotmp-api-story75 go test ./internal/api -run 'TestTetherAPI'`.
- PASS: Review-fix full backend validation: `cd apps/agent && GOTMPDIR=../../.gotmp-full75 go test ./...`.
- PASS: Review-fix frontend validation: `cd apps/web && npm run typecheck`.
- PASS: Review-fix frontend build: `cd apps/web && npm run build`.
- PASS: Review-fix root typecheck: `npm run typecheck`.
- NOTE: One combined focused run hit Windows temp/Access transient with `.gotmp-tdd`; rerun with fresh GOTMPDIRs passed.
- NOTE: Real hardware validation was not run; no physical gPhoto2 camera was available in this environment.

## Completion Note

Ultimate context engine analysis completed - comprehensive developer guide created. Story 7.5 is ready for development with explicit guardrails for safe reconnect/recovery, persisted desired listener state, auto-restart only when enabled, bounded backoff, no privileged usbipd/install/driver actions, no duplicate ingestion, no direct pipeline bypass, and current apps/agent + apps/web architecture references.

Review follow-up complete: both blocking findings are resolved. Disabling `auto_restart_enabled` now cancels pending recovery and scheduled automatic attempts re-read persisted desired state/settings immediately before `Supervisor.Start`. Manual retry remains a one-shot operator action when auto-restart is disabled; failure records safe actionable state and only continues automatic backoff if persisted settings still allow `desired_state=running` plus `auto_restart_enabled=true`.

## Code Review — 2026-05-20

Reviewer: BMAD Code Review (AI)
Outcome: **Changes requested**

### Summary

Implementation covers the broad shape of Story 7.5: persisted desired state, auto-restart setting, recovery coordinator/backoff, API/settings/retry endpoints, frontend controls, SSE invalidation, docs/OpenAPI, and focused tests. No privileged reconnect/install/driver actions or direct photo/session/processing/upload imports were found in the new camera recovery path.

However, there is a blocking correctness issue in the recovery coordinator/API interaction: scheduled/manual recovery attempts can continue after `auto_restart_enabled` is disabled, violating the central AC2/AC6 guarantee that automatic restart happens only when enabled.

### Blocking Findings

1. **Auto-restart disabled does not cancel or gate already scheduled recovery**
   - Files: `apps/agent/internal/api/camera_tether.go`, `apps/agent/internal/cameras/tether_recovery.go`
   - `PUT /api/stations/{station_id}/tether-listener/settings` persists `auto_restart_enabled=false` but does not cancel an existing timer/state in `TetherRecoveryCoordinator`.
   - `TetherRecoveryCoordinator.attempt` does not re-read desired state / auto-restart before starting the listener for scheduled automatic attempts.
   - Result: if an unexpected exit schedules recovery while auto-restart is enabled, then the operator disables auto-restart before `next_attempt_at`, the timer can still call `Supervisor.Start` and restart the listener automatically.
   - Required fix: when disabling auto-restart, cancel pending automatic recovery for that station and/or make every non-manual scheduled attempt re-check `desired_state == running && auto_restart_enabled == true` immediately before calling `Supervisor.Start`.

2. **Manual retry failure can create continued automatic retries even when auto-restart is disabled**
   - Files: `apps/agent/internal/api/camera_tether.go`, `apps/agent/internal/cameras/tether_recovery.go`
   - `Retry` calls `markDesiredRunning`, then `recovery.ManualRetry`, then directly calls `supervisor.Start`.
   - `ManualRetry` schedules a recovery attempt with `manual=true`; if that attempt fails, `failAndMaybeReschedule` unconditionally calls `Schedule(stationID, false)` without checking `auto_restart_enabled`.
   - Because `markDesiredRunning` does not enable auto-restart, this can turn a single manual retry into continued automatic retry/backoff while `auto_restart_enabled=false`.
   - Required fix: distinguish manual one-shot retry from automatic recovery. After a manual attempt fails, only continue automatic rescheduling when persisted settings still have `desired_state=running` and `auto_restart_enabled=true`; otherwise leave actionable failed/attention state without scheduling.

### Validation Run During Review

- PASS: `cd apps/agent && GOTMPDIR=../../.gotmp-review-agent go test ./internal/cameras ./internal/api` after creating the GOTMPDIR directory.
- PASS: `cd apps/web && npm run typecheck`.
- Initial Windows temp-dir setup note: `GOTMPDIR=../../.gotmp-review-agent go test ...` failed before the directory existed; rerun passed after `mkdir -p .gotmp-review-agent`.

### Additional Notes

- Full required validation commands from the story were reported by the dev agent as passing, but this review only reran focused backend API/camera tests and web typecheck.
- Hardware gPhoto2 validation remains not run, consistent with the Dev Agent Record.

## Re-Review — 2026-05-20

Reviewer: BMAD Code Review (AI)
Outcome: **Approved — Done**

### Summary

Prior blockers are resolved:

- Disabling `auto_restart_enabled` cancels pending recovery in the settings endpoint and prevents scheduled automatic recovery from starting.
- Scheduled automatic attempts re-read persisted settings immediately before `Supervisor.Start` and only proceed when `desired_state=running` and `auto_restart_enabled=true`.
- Manual retry remains one-shot while `auto_restart_enabled=false`; failed manual attempts record an actionable failed state and do not enter automatic retry/backoff.

No new regressions found in reviewed paths for safe recovery boundaries: no privileged reconnect/install/driver commands, no folder watcher bypass, no direct photo/session/processing/upload ingestion calls, and duplicate process start remains no-op.

### Validation Run During Re-Review

- PASS: `cd apps/agent && GOTMPDIR=../../.gotmp-review-rereview go test ./internal/cameras ./internal/api`.
- Hardware gPhoto2 validation was not run in this environment.
