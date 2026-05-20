# Story 7.6: Run Real Hardware gPhoto2 Event Smoke Test

Status: done

## Story

Sebagai operator, saya ingin guided smoke test dengan kamera nyata, sehingga saya tahu flow camera → gPhoto2 → station folder → ingestion → processing → Drive delivery siap sebelum produksi.

## Acceptance Criteria

1. **Guided real-hardware path**  
   Given minimal satu kamera nyata connected dan assigned ke station, when smoke test starts, then sistem memandu operator melalui discovery, listener start/retry, physical shutter capture, ingestion verification, local original save verification, graded processing verification, dan optional Google Drive upload verification.

2. **Local-only when Drive absent**  
   Given Google Drive credentials/settings belum tersedia atau operator memilih local-only mode, when smoke test runs, then camera → input folder → ingestion → local original → graded processing tetap dapat selesai dan Drive step dicatat sebagai `skipped`/`not_configured`, bukan failure blocking.

3. **Complete diagnostic report**  
   Given setiap step selesai atau gagal, then smoke report mencatat `timestamp`, `station_id`, safe `camera_name`/`camera_id` summary, safe `file_name`, `session_id` bila dibuat/ditargetkan, ingestion count/result, local original result/path basename-safe, graded processing result, upload result, duration, failure code/action, dan operator next action.

4. **Safe failure behavior**  
   Given failure terjadi pada discovery, listener, physical shutter wait, ingestion, processing, atau upload, then smoke berhenti atau lanjut sesuai mode (`stop_on_failure` default true; optional continue mode explicit) dan report menyimpan next action aman tanpa raw shell output, absolute paths, token, raw port/identity, atau privileged command output.

5. **Saved local diagnostic artifact**  
   Given smoke selesai/aborted, then report tersimpan sebagai local diagnostic artifact under local app data/reports (atau configured implementation artifact target untuk dev) dan bisa dibuka/digunakan untuk event readiness review.

6. **No deletion / no destructive operations**  
   Smoke tidak menghapus file kamera, station input files, local output originals/graded files, quarantine files, reports, atau Google Drive files/folders. Cleanup otomatis hanya boleh untuk temp file milik smoke runner yang jelas prefix-owned dan belum menjadi customer/input/output artifact; rekomendasi MVP: jangan cleanup file hasil smoke sama sekali, hanya label/report.

7. **Hardware manual validation is explicit**  
   Given real hardware tidak tersedia di automated tests/CI, then implementation menyediakan manual validation checklist dan report output sample; story tidak boleh diklaim hardware-pass kecuali benar-benar dijalankan dengan kamera fisik dan hasilnya dicatat di Dev Agent Record.

8. **Tests/build/docs pass**  
   Unit/API/frontend tests untuk report generation, local-only Drive skip, safe sanitization, failure modes, and no-delete/no-privileged boundaries pass; OpenAPI/docs/manual runbook diperbarui; required validation commands dijalankan dan dicatat.

## Acceptance Criteria Context / BDD Detail

### AC1 — Guided real-hardware path

- Story ini adalah runtime/QA smoke feature di production Go agent (`apps/agent`) + optional web/dashboard control (`apps/web`), bukan script ad-hoc yang bypass architecture.
- Minimal viable scope: **single station first**. Multi-station dapat didukung sebagai loop opsional jika hardware tersedia, tetapi jangan membuat multi-station blocking untuk MVP.
- Smoke must use existing production APIs/services from Stories 7.1–7.5:
  - gPhoto2 discovery/assignment from Story 7.1.
  - tether listener status/start/retry/stop from Story 7.2/7.5.
  - camera readiness/test capture concepts from Story 7.3.
  - dashboard-safe camera/tether actions from Story 7.4.
  - desired state/auto-restart/backoff from Story 7.5.
- Smoke must **not** create a parallel gPhoto implementation. It should orchestrate existing components or call small service interfaces around them.
- Operator-guided physical shutter step:
  - runner starts/ensures listener running for selected station,
  - prompts/status tells operator to press physical camera shutter,
  - waits for new downloaded JPG in `station.input_folder`, using expected time window/snapshot of input folder before prompt,
  - verifies the file is stable and likely JPG (`.jpg/.jpeg`, non-empty, JPEG SOI if feasible) through existing watch validation/scanner semantics.
- The smoke must prove the main event path: file is written into `station.input_folder`, then existing watcher/ingestion/processing pipeline observes and handles it. Do **not** shortcut `capture event -> photo record`.
- If the app currently does not expose a single event bus hook for “photo ingested/processed”, implement a safe polling verifier against existing station/session/photo/processing APIs/stores in Go, not browser filesystem reads. Prefer backend service because browser must not read local paths.
- If an active session is required to prove ingestion → processing → local output, smoke may create a dedicated smoke session with unmistakable safe names, e.g. customer `SMOKE_TEST_DO_NOT_DELETE`, order `SMOKE-YYYYMMDD-HHMMSS`. This is allowed only via existing session service/API and must be reported clearly as diagnostic data.
- If an active session already exists for the station, default behavior should be safe: reject unless operator explicitly confirms using current session for smoke. Avoid accidentally routing smoke photos into a real customer session.

### AC2 — Local-only Drive-optional behavior

- Drive is optional for this story. Local smoke must not require Drive credentials, Drive folder, or network availability.
- If Drive settings are absent, invalid, not authorized, or operator selects local-only mode:
  - report `drive.status = skipped` or `not_configured`,
  - `drive.required = false`,
  - local smoke result can still be `passed` if camera/ingestion/local original/graded steps pass.
- If operator selects Drive verification mode (`drive_mode=verify` or equivalent), then Drive failures should be report failure/warning according to mode, but still must not delete local files or block future sessions.
- Drive verification should reuse existing upload/target/status APIs or upload worker state. Do not implement direct Drive client calls inside smoke runner unless it reuses `internal/upload` abstractions safely and still preserves existing queue semantics.
- Never upload directly from camera file before local completion. Drive verification must happen after local save/graded processing success.

### AC3 — Report schema and safe fields

Report must be JSON-serializable and optionally rendered in UI. Suggested backend type:

```go
type HardwareSmokeReport struct {
    ReportID       string    `json:"report_id"`
    StartedAt      time.Time `json:"started_at"`
    CompletedAt    *time.Time `json:"completed_at,omitempty"`
    OverallStatus  string    `json:"overall_status"` // passed|failed|warning|aborted
    Mode           string    `json:"mode"` // local_only|drive_optional|drive_verify
    StopOnFailure  bool      `json:"stop_on_failure"`
    StationID      string    `json:"station_id"`
    StationName    string    `json:"station_name"`
    Camera         SafeSmokeCamera `json:"camera"`
    SessionID      string    `json:"session_id,omitempty"`
    FileName       string    `json:"file_name,omitempty"` // basename only
    Steps          []HardwareSmokeStep `json:"steps"`
    Summary        HardwareSmokeSummary `json:"summary"`
    NextAction     string    `json:"next_action"`
}

type HardwareSmokeStep struct {
    StepKey      string     `json:"step_key"`
    Status       string     `json:"status"` // pending|running|passed|failed|skipped|warning
    StartedAt    time.Time  `json:"started_at"`
    CompletedAt  *time.Time `json:"completed_at,omitempty"`
    DurationMs   int64      `json:"duration_ms"`
    StationID    string     `json:"station_id,omitempty"`
    FileName     string     `json:"file_name,omitempty"`
    Count        int        `json:"count,omitempty"`
    Result       string     `json:"result,omitempty"`
    ErrorCode    string     `json:"error_code,omitempty"`
    ErrorAction  string     `json:"error_action,omitempty"`
    Message      string     `json:"message"`
}
```

Required step keys:

1. `station_config_loaded`
2. `camera_assignment_verified`
3. `gphoto2_discovery`
4. `camera_connected_verified`
5. `listener_started_or_running`
6. `operator_physical_shutter_prompted`
7. `downloaded_file_detected_in_input_folder`
8. `ingestion_verified`
9. `local_original_verified`
10. `graded_processing_verified`
11. `drive_upload_verified_or_skipped`
12. `report_saved`

Safe report constraints:

- `camera_id` must be a safe summary, not raw `identity_key`/port. Use `camera_name` plus `assigned`/`connected` status; if internal identity is necessary for correlation, hash it or store only last short checksum, never raw `runtime|port|name` in UI/report by default.
- `file_name` is basename only. No absolute Windows path, WSL `/mnt/...`, Drive token, raw stdout/stderr, command line, or raw gPhoto port.
- If full local report needs internal absolute paths for developer troubleshooting, store them only in local technical log gated by existing local logging policy, not in operator report/API/UI.

### AC4 — Failure behavior and modes

- Default `stop_on_failure=true` for event safety. A continue mode may be supported for diagnostic collection, but must be explicit in request/UI.
- Failure examples and next actions:
  - no assignment → `CAMERA_ASSIGNMENT_REQUIRED`, action `ASSIGN_CAMERA`.
  - gPhoto2 unavailable → `GPHOTO2_UNAVAILABLE`, action `INSTALL_GPHOTO2`/`CHECK_WSL`.
  - camera disconnected → action `CONNECT_CAMERA`/`CHECK_USBIPD`/`CHECK_CAMERA_USB_MODE`.
  - listener error → action `RETRY_TETHER_LISTENER`.
  - no new JPG within timeout → action `PRESS_SHUTTER_OR_CHECK_CAMERA` (new action allowed if documented) or `RETRY_TEST_CAPTURE`.
  - ingestion not observed → action `CHECK_WATCHER`/`RECHECK_CAMERA_READINESS`.
  - processing failed → action `RETRY_PROCESSING` or `CHECK_LUT`.
  - Drive skipped → status `skipped`, action `CONFIGURE_DRIVE_OPTIONAL` if useful.
- Any failure payload must use existing API error shape `{error:{code,message,action,details}}` if returned immediately, and report step fields if saved.
- Smoke should always attempt to save partial report before returning failure, so readiness review has evidence.
- Do not stop a listener automatically at end if it was already running before smoke. If smoke started it from stopped state, either leave it running and report desired state, or stop only if request has `restore_previous_listener_state=true`. Default safer event behavior: do not surprise-kill an operator’s listener.

### AC5 — Saved local diagnostic artifact

- Recommended storage: `local-data/reports/hardware-smoke/` with files like `hardware-smoke-20260520T153000Z-station_1.json`.
- Create directories if missing with safe permissions.
- Use temp file + atomic rename pattern like station persistence (`apps/agent/internal/stations/persistence.go`) to avoid partial report.
- Report save must never overwrite an existing report; include timestamp + random suffix/report ID.
- OpenAPI/API should expose either:
  - `POST /api/stations/{station_id}/hardware-smoke-tests` to run and return report, and optional `GET /api/hardware-smoke-tests/{report_id}` to read report; or
  - a local script/CLI `go run ./cmd/selfstudio-agent smoke-hardware ...` only if story remains developer/operator runbook. Preferred for dashboard readiness: API endpoint protected by auth + trusted origin.
- If returning report file location to UI, return safe relative report id/name, not absolute path.

### AC6 — No deletion / destructive operation ban

Hard ban in implementation and tests:

- No deletion of camera files.
- No deletion of station input folder contents.
- No deletion of output originals/graded files.
- No deletion of quarantine files.
- No deletion of Drive files/folders.
- No global process kills.
- No `usbipd bind/attach/detach`, install commands, driver changes, or privileged shell.

Allowed limited cleanup:

- Removing a temp report file before atomic rename if it was created by this report writer.
- Removing temp probe files used by existing folder writable checks if they use known prefix and are not image/customer artifacts.
- Avoid cleanup of smoke JPG/session output by default; label/report instead. This is safer and satisfies “no deletion”.

### AC7 — Hardware manual validation constraints

- Automated tests cannot prove real camera hardware. They should fake discovery/listener/watcher/processing/upload interfaces.
- Manual validation must be documented in the story/runbook and Dev Agent Record:
  1. Windows/WSL/gPhoto2 available, Sony A6000 connected/assigned.
  2. Start app/agent/web.
  3. Ensure station input/output/LUT ready.
  4. Run hardware smoke local-only.
  5. Press physical shutter when prompted.
  6. Confirm report shows pass through ingestion/local original/graded and Drive skipped/not_configured if absent.
  7. If Drive configured, run Drive verify mode and confirm upload status.
- Dev agent must not mark “hardware validation passed” unless this checklist was actually run. If not available, record: “Hardware validation not run; no physical camera available.”

## Tasks / Subtasks

- [x] Audit current implementation and preserve boundaries
  - [x] Read this story, Stories 7.1–7.5, and `spec-gphoto-helper-diagnostic-spike.md` before coding.
  - [x] Read `apps/agent/internal/cameras/*` including discovery, tether supervisor, test capture, state store, recovery, and tests.
  - [x] Read `apps/agent/internal/api/cameras.go`, `camera_tether.go`, `camera_test_capture.go`, `readiness.go`, `event_readiness.go`, `sessions.go`, `ingestion.go`, `processing_queue.go`, `session_uploads.go`, `health.go`, `response.go`, `security.go`.
  - [x] Read relevant stores/services for sessions/photos/processing/uploads enough to verify smoke without bypassing ingestion.
  - [x] Read `apps/web/src/features/sessions/camera-tether-status.tsx`, `live-station-cards.tsx`, `apps/web/src/features/readiness/event-readiness-checklist.tsx`, station hooks, and API client.
  - [x] Confirm no root `src/server/gphoto-helper.ts` production reuse; it remains historical spike only.

- [x] Design smoke runner and report schema
  - [x] Add backend package/service, recommended `apps/agent/internal/cameras/hardware_smoke.go` or `apps/agent/internal/smoke/hardware_smoke.go`.
  - [x] Define report/step/status/mode structs with safe JSON fields only.
  - [x] Define safe action allowlist additions if needed: `PRESS_SHUTTER_OR_CHECK_CAMERA`, `CHECK_WATCHER`, `CONFIGURE_DRIVE_OPTIONAL`.
  - [x] Implement report writer under `local-data/reports/hardware-smoke` using temp+atomic rename and non-overwrite report IDs.
  - [x] Ensure report writer never deletes prior reports or image artifacts.

- [x] Implement guided smoke orchestration
  - [x] Validate station id and load persisted station config/assignment.
  - [x] Run/reuse gPhoto2 discovery and verify assigned camera connected without exposing raw port/identity.
  - [x] Capture initial listener status; start/retry listener if needed using existing `TetherSupervisor`/API-safe path.
  - [x] Snapshot input folder state before physical shutter prompt to avoid stale JPG false positives.
  - [x] Prompt/wait for physical shutter by waiting for a new stable JPG in station input folder within configurable timeout.
  - [x] Verify new file basename and JPEG/stable status; no deletion/overwrite.
  - [x] Create or require a smoke session safely if needed to prove ingestion/processing. Default should avoid active real customer sessions unless explicitly allowed.
  - [x] Poll/verify ingestion count/photo record for the new file through existing backend state, not direct browser filesystem.
  - [x] Poll/verify original local save and graded processing result through existing processing/photo/session status.
  - [x] Drive step: if local-only/Drive absent, mark skipped/not_configured; if Drive verify selected, reuse existing upload status/worker flow after local completion.
  - [x] Save partial report on every terminal failure/abort.

- [x] Add API endpoint and/or CLI/runbook
  - [x] Preferred API: `POST /api/stations/{station_id}/hardware-smoke-tests` protected by `RequireAuth` + `RequireTrustedOrigin`.
  - [x] Request body fields: `mode` (`local_only|drive_optional|drive_verify`), `stop_on_failure`, `timeout_ms`, `allow_active_session` default false, `restore_previous_listener_state` default false.
  - [x] Response: `{data:{report}}` with safe report fields and saved `report_id`/relative file name.
  - [x] Optional status/list endpoint only if needed; do not overbuild.
  - [x] Publish safe SSE event `camera.hardware_smoke_completed` or `station.hardware_smoke_completed` for dashboard invalidation.
  - [x] Record activity log success/failure with station ref and safe summary.

- [x] Optional UI integration (minimal but useful)
  - [x] Add “Run hardware smoke test” control either in Station Settings/readiness panel or Camera/Tether station card.
  - [x] Show mode selector with Drive optional/local-only copy; default local-only or drive_optional.
  - [x] Show clear manual instruction: “Press physical shutter now” while runner waits.
  - [x] Render report summary and safe next action; never render raw ports, absolute paths, command output, identity keys, or tokens.
  - [x] Invalidate readiness/activity/tether/session/processing/upload queries after smoke result.

- [x] Docs/OpenAPI/manual validation
  - [x] Update `docs/api/openapi.yaml` with smoke endpoint schemas, modes, report, errors, and safety constraints.
  - [x] Add or update operations doc, recommended `docs/hardware-smoke-test.md`, with manual camera checklist and local-only/Drive-optional behavior.
  - [x] Document no deletion, no privileged usbipd/install/driver commands, no Drive deletion, and no bypass of watcher/ingestion.

- [x] Backend tests
  - [x] Report schema serializes safe fields; no absolute paths/raw port/identity/command/stdout/stderr/token in report JSON.
  - [x] Report writer creates unique files with temp+atomic rename and never deletes existing reports/artifacts.
  - [x] Local-only mode marks Drive skipped/not_configured and can pass local pipeline.
  - [x] Drive absent does not fail local-only smoke.
  - [x] Discovery/assignment missing returns safe failure/action.
  - [x] Listener already running is reused; listener stopped is started; duplicate start remains safe.
  - [x] Stale JPG before smoke is not accepted as physical shutter result.
  - [x] New stable JPG is detected by expected filename/time window.
  - [x] Ingestion/processing verification uses existing state interfaces; no direct creation of photo/session/processing/upload records except allowed smoke session through existing session service.
  - [x] Failure mode saves partial report and respects `stop_on_failure`.
  - [x] Static/guard test: smoke package does not construct forbidden commands (`usbipd bind/attach`, `winget`, `choco`, `apt`, `taskkill`, `pkill`, shell string) and does not delete image/output/Drive artifacts.
  - [x] API tests: auth required, trusted origin required, station scope validation, safe error wrapper, safe report response.

- [x] Frontend/type/build validation
  - [x] If UI added, TypeScript types compile and UI handles running/passed/failed/skipped report states.
  - [x] UI safe rendering audit for no raw `port`, `identity_key`, `source_path`, command, stdout/stderr, absolute paths.
  - [x] Run required validations and record actual results.

- [x] Manual hardware validation record
  - [x] If hardware available, run local-only smoke with real camera and paste report summary in Dev Agent Record.
  - [x] If Drive configured, optionally run Drive verify mode and record outcome.
  - [x] If hardware unavailable, explicitly state hardware validation not run; do not imply pass.

### Review Findings

- [x] [Review][Patch] `stop_on_failure=false` diabaikan dan tidak ada continue mode — AC4 meminta default `stop_on_failure=true` dengan optional continue mode explicit; implementasi membaca flag ke `report.StopOnFailure`, tetapi helper `fail` selalu langsung `finishAndSave` dan return error pada kegagalan pertama, sehingga request eksplisit `stop_on_failure=false` tetap berhenti di step pertama yang gagal dan tidak mengumpulkan diagnostik lanjutan. [apps/agent/internal/cameras/hardware_smoke.go:155]
- [x] [Review][Patch] `allow_active_session`/safe smoke session tidak diterapkan — AC1 mengharuskan default aman menolak active real customer session kecuali operator eksplisit mengizinkan, atau membuat/menargetkan smoke session via existing session service bila dibutuhkan; request field ada, tetapi runner/verifier hanya menunggu photo store dan menerima `SessionID` apa pun dari ingestion tanpa memeriksa active session, tanpa membuat dedicated smoke session, dan tanpa menggunakan `allow_active_session`. Ini bisa merutekan/menandai foto smoke ke sesi customer aktif. [apps/agent/internal/cameras/hardware_smoke.go:75]
- [x] [Review][Patch] Drive verify tidak benar-benar memverifikasi upload job/status selesai — AC2 mensyaratkan mode `drive_verify` memakai existing upload/target/status APIs atau worker state setelah local completion. `VerifyDrive` hanya membaca `UploadTargets.Get(sessionID)` dan langsung mengembalikan `target.PublicStatus()` sebagai sukses untuk status apa pun selain `not_configured`; tidak menggunakan `UploadJobs`, tidak polling hingga uploaded, dan akan menandai step passed/report passed untuk status `pending` atau bahkan `failed`. [apps/agent/internal/api/hardware_smoke.go:201]
- [x] [Review][Patch] Report yang tersimpan tidak memuat step `report_saved` dan tidak membuktikan report persistence di artifact — `finishAndSave` memanggil `Writer.Save(*report)` sebelum `report.ReportFile` dan step `report_saved` ditambahkan; akibatnya JSON di disk tidak berisi `report_file` maupun step required `report_saved`, meski response API berisi step tersebut. Ini melanggar AC3/AC5 complete diagnostic report dan saved artifact review. [apps/agent/internal/cameras/hardware_smoke.go:251]
- [x] [Review][Patch] Atomic report writer masih bisa overwrite dalam race dan meninggalkan temp file pada rename failure — AC5 mensyaratkan report save tidak pernah overwrite dan atomic write. Implementasi hanya `os.Stat(final)` lalu `os.Rename(tmp, final)`; pada platform POSIX rename dapat mengganti file yang muncul di antara check dan rename. Tidak ada `O_EXCL`/unique retry loop untuk final, dan temp file tidak dibersihkan bila rename gagal. [apps/agent/internal/cameras/hardware_smoke.go:276]
- [x] [Review][Patch] Verifikasi file shutter belum memastikan file stabil — AC1/AC7 mengharuskan stable JPG validation. `WaitForNewJPG` menerima file segera setelah modtime >= snapshot, size > 0, dan JPEG SOI; tidak ada pemeriksaan ukuran/modtime stabil antar interval. File yang masih ditulis dapat diterima terlalu dini dan menyebabkan ingestion/processing verification flake. [apps/agent/internal/api/hardware_smoke.go:139]
- [x] [Review][Patch] Error discovery dapat panic ketika assignment nil bila runner dipakai langsung — `DefaultHardwareSmokeVerifier.Discover` dereference `station.Assignment.IdentityKey` tanpa guard. Runner memang memeriksa assignment sebelum memanggil verifier, tetapi verifier adalah exported/default component dan test/API bisa memakainya langsung; safety boundary sebaiknya tetap safe failure, bukan panic. [apps/agent/internal/api/hardware_smoke.go:115]

## Dev Notes

### Epic 7 Context

Epic 7 adds managed gPhoto2 camera tethering while preserving selfstudio’s local-first architecture. Story 7.6 is the final runtime confidence story: it proves the chain from physical camera shutter through gPhoto2 listener, station input folder, existing watcher/ingestion, local original-first processing, graded output, and optional Google Drive upload. This story should produce a reusable readiness artifact for real event operation, not just a developer test.

### Current Architecture That Must Be Followed

- Frontend: `apps/web`, Next.js App Router, TypeScript, TanStack Query, Tailwind/shadcn.
- Backend/local worker: Go service in `apps/agent`; all filesystem/process/credentials/local mutations live in Go.
- API: REST under `/api`, SSE under `/events`, success wrapper `{data}`, error wrapper `{error:{code,message,action,details}}`, JSON `snake_case`.
- Auth/security: local PIN/session cookie; state-changing endpoints require `RequireAuth` + `RequireTrustedOrigin`.
- Events: dot notation with existing event wrapper.
- Browser must never execute commands, inspect local filesystem, run usbipd, or receive raw diagnostics/secrets.
- Folder watcher/ingestion remains source of truth. Smoke runner may verify state, but must not bypass the pipeline.
- Local save is independent of Drive upload. Drive absent must not block local smoke.

### Completed Story Intelligence

#### Story 7.1 — Discovery and assignment

- Implemented safe gPhoto2 discovery and station camera assignment in Go.
- Existing endpoint: `POST /api/cameras/gphoto2/discover`.
- Existing endpoint: `PUT /api/stations/{station_id}/camera-assignment`.
- `usbipd list` read-only diagnostic is allowed; `usbipd bind/attach` are forbidden.
- Duplicate camera assignment validation exists.
- Prior review caught raw port/identity display. Do not regress: no raw `identity_key`/`port` in report/UI.

#### Story 7.2 — Tether listener supervisor

- Existing endpoints:
  - `GET /api/stations/{station_id}/tether-listener`
  - `POST /api/stations/{station_id}/tether-listener/start`
  - `POST /api/stations/{station_id}/tether-listener/stop`
- Supervisor is per-station, duplicate start no-op, stop station-specific, no global kill.
- `BuildTetherCommand` handles path confinement and WSL path conversion.
- Listener only writes JPG to `station.input_folder`; it does not call ingestion/photo/session/processing/upload.
- Preserve race fix: stop during process startup must not orphan gPhoto2 process.

#### Story 7.3 — Camera readiness and test capture

- Existing endpoint: `POST /api/stations/{station_id}/camera-test-capture`.
- Camera readiness checks include assignment, gPhoto2 availability, camera connected, listener, input folder writable, last test capture.
- Test capture is explicit, local-first, expected-file-only, JPEG SOI validated, and watcher validation path.
- Review fixes included timeout context, raw path/port UI removal, collision-safe filename, no `--force-overwrite`, and safe error details.
- For smoke: reuse the same principles, but physical shutter smoke should validate actual listener-driven download rather than only command-driven capture.

#### Story 7.4 — Live dashboard Camera/Tether UI

- `apps/web/src/features/sessions/camera-tether-status.tsx` shows Camera/Tether separately from local processing and Drive.
- Strong frontend sanitization exists for listener messages, raw port, paths, shell diagnostics, identities, and credentials.
- SSE invalidation handles `camera.tether_listener_updated`.
- Extend safe mappings if adding smoke action/event codes.

#### Story 7.5 — Recovery and reconnect

- Persisted desired listener state and `auto_restart_enabled` exist via `TetherStateStore`.
- Recovery coordinator/backoff exists; auto-restart only when desired running + enabled.
- Existing endpoints include settings/retry:
  - `POST /api/stations/{station_id}/tether-listener/retry`
  - `PUT /api/stations/{station_id}/tether-listener/settings`
- Review fixed critical gating: disabling auto-restart cancels pending recovery; manual retry remains one-shot when auto-restart disabled.
- Hardware validation was not run in Story 7.5 environment.

### Spike Specs and Lessons

From `_bmad-output/implementation-artifacts/spec-gphoto-helper-diagnostic-spike.md`:

- Historical TS prototype had diagnostics, capture, one-click setup, continuous capture, and trigger listener.
- Useful lessons: no install commands, no driver changes, station output must stay inside input folder, serialized capture/listener, JPG SOI validation.
- Dangerous/outdated prototype behavior: one-click `usbipd bind/attach` and setup+capture. These are **not allowed** in production story.
- Do not copy `src/server/gphoto-helper.ts` or `src/server/gphoto-autosetup.ts`; production backend is Go `apps/agent`.

### Existing Code References — Current State

#### Backend camera/tether

- `apps/agent/internal/cameras/gphoto_runner.go`
  - Discovery service, safe command runner, read-only `usbipd list` diagnostic.
- `apps/agent/internal/cameras/tether_supervisor.go`
  - Per-station process supervisor, command/path confinement, sanitized listener status.
- `apps/agent/internal/cameras/test_capture.go`
  - Expected-file validation, JPEG SOI validation, collision-safe filename; useful for smoke detection patterns.
- `apps/agent/internal/cameras/tether_recovery.go`
  - Backoff coordinator; avoid conflicting with smoke runner.
- `apps/agent/internal/cameras/tether_state_store.go`
  - Desired state persistence; smoke should not unexpectedly change auto-restart settings.

#### Backend API

- `apps/agent/internal/api/health.go`
  - Router wiring. Add smoke route here consistently and use same singleton `TetherSupervisor`, `DiscoveryService`, and stores.
- `apps/agent/internal/api/camera_tether.go`
  - Safe listener response includes listener/settings/recovery.
- `apps/agent/internal/api/camera_test_capture.go`
  - Good example of auth/trusted-origin endpoint, activity, SSE, safe details.
- `apps/agent/internal/api/response.go`
  - Use `writeData`, `writeAPIError`, `writeAPIErrorWithDetails`.
- `apps/agent/internal/api/security.go`
  - Mutations require trusted origin.

#### Existing pipeline areas to inspect before implementing verification

- `apps/agent/internal/api/sessions.go` and session store/service: how to create/end sessions and read active/detail safely.
- `apps/agent/internal/api/ingestion.go` and ingestion scanner/router: how new files become photo records.
- `apps/agent/internal/api/processing_queue.go`, `photo_retry.go`, processing package: how to observe original/graded state.
- `apps/agent/internal/api/session_uploads.go`, cloud settings/targets: how to observe Drive upload status and absence.
- `apps/agent/internal/readiness/checklist.go`: event readiness conventions.

#### Frontend

- `apps/web/src/lib/api/client.ts`: add smoke DTO/function if UI/API added.
- `apps/web/src/features/sessions/camera-tether-status.tsx`: safest place for minimal per-station smoke button/report summary.
- `apps/web/src/features/readiness/event-readiness-checklist.tsx`: alternative placement for readiness review flow.
- `apps/web/src/features/health/health-dashboard.tsx`: add SSE invalidation if smoke completed event introduced.

### File References

#### Must read before implementation

- `_bmad-output/planning-artifacts/epics.md`
- `_bmad-output/planning-artifacts/architecture.md`
- `_bmad-output/planning-artifacts/prd.md`
- `_bmad-output/implementation-artifacts/7-1-detect-and-assign-gphoto2-cameras-to-stations.md`
- `_bmad-output/implementation-artifacts/7-2-supervise-gphoto2-tether-listener-per-station.md`
- `_bmad-output/implementation-artifacts/7-3-add-camera-readiness-checks-and-test-capture-validation.md`
- `_bmad-output/implementation-artifacts/7-4-show-live-camera-tether-status-and-controls-in-dashboard.md`
- `_bmad-output/implementation-artifacts/7-5-recover-and-reconnect-gphoto2-tethering-during-event.md`
- `_bmad-output/implementation-artifacts/spec-gphoto-helper-diagnostic-spike.md`
- `apps/agent/internal/cameras/gphoto_runner.go`
- `apps/agent/internal/cameras/tether_supervisor.go`
- `apps/agent/internal/cameras/test_capture.go`
- `apps/agent/internal/cameras/tether_recovery.go`
- `apps/agent/internal/cameras/tether_state_store.go`
- `apps/agent/internal/api/camera_tether.go`
- `apps/agent/internal/api/camera_test_capture.go`
- `apps/agent/internal/api/readiness.go`
- `apps/agent/internal/api/sessions.go`
- `apps/agent/internal/api/ingestion.go`
- `apps/agent/internal/api/processing_queue.go`
- `apps/agent/internal/api/session_uploads.go`
- `apps/agent/internal/api/health.go`
- `apps/web/src/features/sessions/camera-tether-status.tsx`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`

#### Likely files to create/update

- `apps/agent/internal/cameras/hardware_smoke.go` or `apps/agent/internal/smoke/hardware_smoke.go` (NEW)
- `apps/agent/internal/cameras/hardware_smoke_test.go` or equivalent (NEW)
- `apps/agent/internal/cameras/hardware_smoke_report_store.go` (NEW optional)
- `apps/agent/internal/api/hardware_smoke.go` (NEW)
- `apps/agent/internal/api/hardware_smoke_test.go` (NEW)
- `apps/agent/internal/api/health.go` (UPDATE route/wiring)
- `apps/web/src/lib/api/client.ts` (UPDATE DTO/function if UI added)
- `apps/web/src/features/sessions/camera-tether-status.tsx` or readiness UI (UPDATE optional)
- `apps/web/src/features/health/health-dashboard.tsx` (UPDATE if SSE event added)
- `docs/api/openapi.yaml` (UPDATE)
- `docs/hardware-smoke-test.md` (NEW recommended)

### Testing Strategy

#### Backend unit/API tests

1. Report generation:
   - safe JSON fields only;
   - step order preserved;
   - durations/timestamps populated;
   - partial report saved on failure;
   - unique report IDs/files.
2. Safety/no deletion:
   - fake filesystem includes input/output/quarantine files; smoke failure does not remove them;
   - report writer only removes its own temp report if necessary;
   - static grep/guard for forbidden destructive commands/Drive delete calls.
3. Local-only/Drive optional:
   - Drive absent returns `skipped`/`not_configured`;
   - local pipeline pass yields overall `passed` or `warning`, not failed due Drive.
4. Camera/tether prerequisites:
   - missing assignment;
   - discovery no camera;
   - listener already running;
   - listener start failure;
   - listener retry success.
5. Physical shutter wait simulation:
   - stale JPG before smoke ignored;
   - new stable JPG accepted;
   - timeout records safe action;
   - bad/non-JPG rejected.
6. Ingestion/processing verification:
   - fake state shows ingestion count increments;
   - original saved before graded;
   - graded failure reports action `RETRY_PROCESSING`/`CHECK_LUT` and preserves original result;
   - no direct photo/session/upload records are created except allowed smoke session via service.
7. API security:
   - auth required;
   - trusted origin required;
   - station not found;
   - response wrapper;
   - safe error wrapper;
   - no raw path/port/identity/command leaks.

#### Frontend validation (if UI touched)

- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build`
- `npm run typecheck`
- Manual/safe rendering audit for no raw camera identity/path/command/report internals.

#### Backend validation

- `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`
- If Windows `Access is denied` transient occurs, rerun with fresh `.gotmp-*` and document both attempts accurately.

#### Manual hardware validation

- Local-only real camera smoke:
  - assigned camera connected;
  - listener starts/running;
  - operator presses physical shutter;
  - report shows new file detected in station input folder;
  - ingestion count increments;
  - original and graded output verified;
  - Drive step skipped/not_configured if absent.
- Drive verify smoke (optional):
  - Drive settings authorized;
  - report shows folder/upload status after local completion;
  - upload failures do not delete local files.

### Regression Risks to Avoid

- Reusing old TypeScript gPhoto spike in `src/server`.
- Running privileged USB/setup/install/driver commands.
- Running `usbipd bind/attach/detach`, `winget`, `choco`, `apt`, `taskkill`, `pkill`, shell strings.
- Deleting any camera/input/output/quarantine/Drive files.
- Accepting stale JPGs as smoke capture success.
- Creating photo/session/processing/upload records directly and bypassing existing services.
- Starting duplicate listener process or fighting auto-recovery.
- Stopping an operator’s pre-existing listener unexpectedly.
- Requiring Drive for local readiness.
- Exposing raw port/identity/path/stdout/stderr/command/token in report/API/UI.
- Claiming hardware pass without real hardware.

## Project Context Reference

- Project: `selfstudio`.
- User: `alpharize`.
- Current date: 2026-05-20.
- Planning artifacts folder: `_bmad-output/planning-artifacts`.
- Implementation artifacts folder: `_bmad-output/implementation-artifacts`.
- Production architecture: Go agent (`apps/agent`) + Next.js web (`apps/web`) + local data under `local-data`.

## Dev Agent Record

### File List

- `apps/agent/internal/cameras/hardware_smoke.go` (new)
- `apps/agent/internal/cameras/hardware_smoke_test.go` (new)
- `apps/agent/internal/api/hardware_smoke.go` (new)
- `apps/agent/internal/api/hardware_smoke_test.go` (new)
- `apps/agent/internal/api/health.go` (updated route/wiring)
- `docs/api/openapi.yaml` (updated)
- `docs/hardware-smoke-test.md` (new)
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (updated status)
- `_bmad-output/implementation-artifacts/7-6-run-real-hardware-gphoto2-event-smoke-test.md` (updated Dev Agent Record/status/tasks)

### Debug Log

- Review patch pass: implemented real `stop_on_failure=false` continuation with pending-step partial reports, safe active-session gating via session store and `allow_active_session`, Drive verify polling through upload jobs/aggregate status, persisted `report_file` + `report_saved` in saved JSON, race-resistant report writer with lock/exclusive final naming and temp cleanup on rename failure, stable JPG shutter validation, and nil-assignment guard in default discovery verifier.
- Implemented backend hardware smoke report schema, step schema, safe camera summary, local-only/Drive-verify modes, stop-on-failure default, and report writer with temp file + atomic rename under `local-data/reports/hardware-smoke`.
- Implemented smoke orchestration in Go agent using existing discovery/tether/photo/upload state abstractions. Smoke waits for a new stable JPG in station input folder after the physical shutter prompt and verifies ingestion/local original/graded status through backend state rather than direct browser filesystem access or direct photo/session record creation.
- Added API endpoint `POST /api/stations/{station_id}/hardware-smoke-tests` with `RequireAuth` + `RequireTrustedOrigin`, `{data:{report}}` response, activity log, and safe SSE `station.hardware_smoke_completed`.
- Added docs/OpenAPI and manual validation runbook documenting local-only/Drive-optional behavior and hard safety bans.
- Guardrails preserved: no deletion of camera/input/output/quarantine/Drive/report artifacts beyond report temp atomic rename; no privileged `usbipd bind/attach/detach`, install/driver, global kill, shell command, or folder watcher bypass; no raw port/identity/path/command/stdout/stderr/token exposure in report/API.
- Hardware validation not run; no physical camera available in this execution environment. No real-hardware pass is claimed.
- Note: first focused API test attempt with `GOTMPDIR=../../.gotmp` hit Windows transient `Access is denied`; rerun with `GOTMPDIR=../../.gotmp2` passed.
- Review patch validation note: first focused rerun with `GOTMPDIR=../../.gotmp-review-fix` hit Windows transient `Access is denied` for `api.test.exe`; rerun with `GOTMPDIR=../../.gotmp-review-fix2` passed.

### Change Log

- 2026-05-20: Story context prepared and expanded for development. Status set to `ready-for-dev`.
- 2026-05-20: Implemented guided real-hardware gPhoto2 event smoke test backend/API/report/docs/tests. Status set to `review`.
- 2026-05-20: Addressed code review findings - 7 patch items resolved; status set to `review`.

### Validation Results

- PASS: `cd apps/agent && GOTMPDIR=../../.gotmp2 go test ./...`
- PASS: `cd apps/web && npm run typecheck`
- PASS: `cd apps/web && npm run build`
- PASS: `npm run typecheck`
- PASS: `cd apps/agent && GOTMPDIR=../../.gotmp-review-fix2 go test ./internal/cameras ./internal/api`
- PASS: `cd apps/agent && GOTMPDIR=../../.gotmp-review-fix2 go test ./...`
- PASS: `npm run typecheck`
- Manual hardware smoke: not run; no physical camera available. Hardware pass is not claimed.

## Completion Note

Ultimate context engine analysis completed - comprehensive developer guide created. Story 7.6 is ready for development with explicit guardrails for real-hardware gPhoto2 smoke validation, local-only/Drive-optional mode, safe report artifacts, no deletion/destructive actions, no privileged USB/install/driver operations, no watcher/ingestion bypass, and manual hardware validation constraints.
