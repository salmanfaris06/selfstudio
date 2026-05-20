# Story 7.1: Detect and Assign gPhoto2 Cameras to Stations

Status: done

## Story

Sebagai admin, saya ingin mendeteksi kamera yang tersedia lewat gPhoto2 dan meng-assign tiap kamera/device/port ke station logical, sehingga tiap kamera fisik reliably menjatuhkan JPG ke input folder station yang benar.

## Acceptance Criteria

1. Given gPhoto2 tersedia native atau via WSL/usbipd, when admin menjalankan discovery, then sistem mengembalikan daftar kamera normalized berisi camera name, device/port identity, transport mode, source runtime, connected status, dan safe diagnostics.
2. Given gPhoto2/WSL/usbipd/kamera tidak tersedia, when discovery dijalankan, then API mengembalikan safe error/action seperti `INSTALL_GPHOTO2`, `CHECK_WSL`, `CHECK_USBIPD`, atau `CONNECT_CAMERA` tanpa crash.
3. Given kamera terdeteksi, when admin assign kamera ke `station_1`/`station_2`/`station_3`, then assignment tersimpan dan dipakai ulang setelah restart bila identity yang sama masih detectable.
4. Given dua station mencoba memakai kamera/port yang sama, when settings disimpan, then sistem menolak konfigurasi duplicate assignment.
5. Given operator meminta action yang butuh privilege/install/driver/usbipd bind, then sistem hanya menampilkan next action dan tidak menjalankan perubahan privileged tanpa explicit approval.
6. Tests/build pass dan docs/API diperbarui.

## Acceptance Criteria Context / BDD Detail

### AC1 — Discovery native/WSL menghasilkan kamera normalized

- Discovery adalah fitur backend Go di `apps/agent`, bukan prototype TS lama di `src/server`.
- Endpoint yang disarankan: `GET /api/cameras/gphoto2/discovery` atau `POST /api/cameras/gphoto2/discover` untuk forced scan. Jika mutation/scan dianggap command, gunakan `POST` dan wajib auth + trusted origin; jika read-only cached result, `GET` boleh auth-only.
- Response sukses tetap memakai wrapper `{ "data": { ... } }` dan field `snake_case`.
- DTO minimum yang harus tersedia untuk setiap kamera:
  - `camera_id` atau `identity_key`: stable internal key hasil normalisasi.
  - `name`: nama kamera dari gPhoto2, misalnya `Sony Alpha-A6000` bila tersedia.
  - `model`: optional, jika bisa dipisahkan dari name.
  - `port`: port gPhoto2 seperti `usb:001,004` bila tersedia.
  - `device_path`/`bus_id`: optional safe hardware identity jika tersedia dari diagnostic, jangan wajib.
  - `transport`: `usb`/`ptp`/`unknown`.
  - `runtime`: `native_windows`, `wsl`, atau `unknown`.
  - `connected`: boolean.
  - `diagnostics`: daftar safe diagnostic item, bukan raw stderr lengkap.
  - `detected_at`: ISO UTC.
- Discovery harus mencoba strategi yang aman dan deterministic:
  1. Native `gphoto2 --auto-detect` jika binary tersedia dan runtime mendukung.
  2. WSL `wsl.exe -- gphoto2 --auto-detect` atau distro eksplisit dari config jika native tidak tersedia/Windows path membutuhkan WSL.
  3. Optional usbipd diagnostic read-only (`usbipd list`) hanya untuk status/instruksi, bukan bind/attach otomatis.
- Parser harus tahan output kosong, header berbeda, localized noise, stderr warning, dan multi-camera rows. Jangan treat warning sebagai crash bila exit code masih memberi data yang usable.
- Discovery result harus mengembalikan status aggregate, misalnya `ready`, `no_cameras`, `gphoto2_missing`, `wsl_missing`, `usbipd_check_needed`, `error`, plus `action` operator.

### AC2 — Safe unavailable states dan error/action

- Tidak boleh panic/crash bila binary tidak ada, WSL tidak aktif, usbipd tidak terinstall, kamera tidak tersambung, kamera busy, atau command timeout.
- Error response gunakan standard API shape:
  ```json
  {
    "error": {
      "code": "GPHOTO2_UNAVAILABLE",
      "message": "gPhoto2 belum tersedia untuk camera discovery.",
      "action": "INSTALL_GPHOTO2",
      "details": { "runtime": "wsl" }
    }
  }
  ```
- Gunakan action allowlist yang jelas:
  - `INSTALL_GPHOTO2`: gPhoto2 tidak ditemukan di native/WSL runtime.
  - `CHECK_WSL`: Windows host tidak bisa menjalankan WSL atau distro default tidak tersedia.
  - `CHECK_USBIPD`: usbipd tidak ditemukan atau kamera belum attached ke WSL.
  - `CONNECT_CAMERA`: tidak ada kamera terdeteksi oleh gPhoto2.
  - `RETRY_CAMERA_DISCOVERY`: transient command/timeout/busy.
  - `CHECK_CAMERA_USB_MODE`: kamera terlihat OS-level tetapi gPhoto2 tidak bisa akses; operator perlu cek USB mode/PTP/PC Remote.
- API details tidak boleh mengandung command raw lengkap, absolute credential path, token, raw privileged stderr, atau data device yang tidak perlu.
- Command execution wajib context timeout pendek dan kill process tree semampunya; jangan membiarkan gPhoto2/WSL child process menggantung request.

### AC3 — Assignment tersimpan dan reusable setelah restart

- Assignment adalah mapping station logical ke identity gPhoto2 normalized. Mapping harus persisted bersama station config atau file config baru di `local-data/config` dengan atomic temp+rename.
- Prefer integrasi ke model station existing agar UI/settings tetap satu sumber:
  - Existing `stations.Station` punya `device_identifier`; Story 7.1 boleh menambah field baru seperti `camera_assignment`/`gphoto2_camera` tanpa menghilangkan backward compatibility.
  - Jika hanya memakai `device_identifier`, pastikan maknanya tidak ambigu: simpan stable identity, bukan label bebas saja.
- Data minimum assignment per station:
  - `station_id`.
  - `camera_identity_key` stable dari discovery (`runtime|port|name` atau format lebih baik).
  - `camera_name` display.
  - `port`.
  - `runtime`.
  - `assigned_at`.
  - optional `last_seen_at`/`connected` dari scan terakhir.
- Setelah restart, `stations.Persistence.LoadOrDefault` atau persistence baru harus memuat assignment. Jika kamera yang sama terdeteksi ulang, UI/API harus dapat menunjukkan assigned+connected; jika tidak, assigned+disconnected dengan action `CONNECT_CAMERA`/`CHECK_USBIPD`, bukan menghapus assignment diam-diam.
- Assignment persistence tidak boleh merusak backup/restore station config. Jika `stations.json` version diubah, migration/backward-read untuk version lama wajib ditangani.

### AC4 — Duplicate assignment ditolak

- Sistem harus mencegah dua station memakai identity/port kamera yang sama.
- Validasi duplicate harus berjalan di service/store backend, bukan hanya UI.
- Normalize key sebelum compare: trim, lower-case untuk Windows-like values, canonical separator, dan prefer canonical `identity_key` dari discovery.
- Duplicate empty/unassigned tidak dianggap conflict; duplicate non-empty assignment harus menghasilkan `409 Conflict` atau `400 Bad Request` dengan code seperti `DUPLICATE_CAMERA_ASSIGNMENT` dan action `CHOOSE_DIFFERENT_CAMERA`.
- Existing duplicate input folder validation di `apps/agent/internal/stations/store.go` harus tetap berfungsi dan tidak regress.

### AC5 — Privileged/install/driver/usbipd actions hanya instruksi eksplisit

- Story ini hanya discovery dan assignment. Tidak boleh otomatis:
  - install gPhoto2/apt/choco/winget,
  - menjalankan `usbipd bind`, `usbipd attach`, driver switch, Zadig, registry/driver changes,
  - menghapus file kamera,
  - start tether listener long-running (itu Story 7.2).
- Jika diagnostic menemukan tindakan privileged/manual, response harus berisi next action dan optional `manual_command_hint` yang safe, bukan menjalankannya.
- Jika menampilkan command hint, jangan echo input user; generate dari allowlist dan sanitize bus id/distro dengan regex ketat. Hindari full raw shell script.
- UI harus memberi copy operator: “Manual approval required / Jalankan di terminal admin jika Anda setuju”, bukan tombol auto-run privileged.

### AC6 — Tests/build/docs/API

- Update `docs/api/openapi.yaml` dengan endpoint discovery/assignment, schemas, response codes, dan safe error actions.
- Update `apps/web/src/lib/api/client.ts` types/functions dan UI station settings untuk discovery + assignment.
- Required validation setelah implementasi:
  - `cd apps/agent && GOTMPDIR=../../.gotmp go test ./...`
  - `cd apps/web && npm run typecheck`
  - `cd apps/web && npm run build`
  - `npm run typecheck`
- Catat hasil command di Dev Agent Record; jangan klaim pass tanpa benar-benar menjalankan.

## Tasks / Subtasks

- [x] Audit kode dan kontrak existing sebelum coding
  - [x] Baca lengkap `apps/agent/internal/stations/store.go`, `persistence.go`, `readiness.go`, `watch_validation.go`.
  - [x] Baca lengkap `apps/agent/internal/api/stations.go`, `readiness.go`, `health.go` router registration, `response.go`, `security.go`.
  - [x] Baca lengkap `apps/web/src/lib/api/client.ts`, `apps/web/src/features/stations/station-settings.tsx`, hooks station existing.
  - [x] Baca spike specs: `spec-gphoto-helper-diagnostic-spike.md`, `spec-direct-camera-capture-spike.md`, `spec-usb-camera-detection-spike.md` untuk safety lessons; jangan copy prototype TS ke production.
- [x] Desain domain model camera discovery + assignment di Go
  - [x] Tambah package yang disarankan `apps/agent/internal/cameras` atau `internal/gphoto` berisi domain types `CameraDiscoveryResult`, `DetectedCamera`, `CameraAssignment`.
  - [x] Definisikan enum/status/action allowlist: runtime, transport, discovery status, safe next actions.
  - [x] Implement canonical identity key builder yang stable dan documented; prefer `runtime + port + normalized_name`, dengan fallback yang eksplisit.
- [x] Implement safe command runner gPhoto2/WSL/usbipd diagnostic
  - [x] Gunakan `exec.CommandContext`, bukan shell string.
  - [x] Allowlist binary dan args: `gphoto2 --auto-detect`, `gphoto2 --summary` optional, `wsl.exe -- gphoto2 --auto-detect`, `usbipd list` read-only optional.
  - [x] Tambah timeout dan output byte limit; sanitize stdout/stderr menjadi diagnostics yang aman.
  - [x] Parser table `gphoto2 --auto-detect` dengan unit tests untuk no camera, single camera, multi-camera, warning/noise, WSL unavailable.
- [x] Implement API discovery
  - [x] Tambah handler misalnya `apps/agent/internal/api/cameras.go`.
  - [x] Register route di `apps/agent/internal/api/health.go` mux.
  - [x] Auth wajib; trusted origin wajib untuk route yang memicu fresh scan jika memakai POST.
  - [x] Response sukses `{data:{status, action, runtime, cameras, diagnostics, scanned_at}}`; error memakai standard error wrapper.
  - [x] Publish activity log safe untuk discovery success/failure; optional SSE `camera.discovery_completed` hanya jika ada broker dan payload safe.
- [x] Implement persistence assignment dan duplicate validation
  - [x] Pilih integrasi: extend `stations.Station`/`UpdateStation` atau persistence terpisah. Jika extend station, jaga backward compatibility JSON lama.
  - [x] Tambah validation duplicate assignment di store, mirip duplicate input folder.
  - [x] Pastikan `Backup`/`Restore` menyertakan assignment tanpa secrets dan tetap valid untuk tiga station.
  - [x] Tambah endpoint assignment bila tidak menyatu dengan `PUT /api/stations/{station_id}`; rekomendasi: `PUT /api/stations/{station_id}/camera-assignment` agar assignment tidak tercampur form station lama.
- [x] Update UI station settings
  - [x] Tambah panel “gPhoto2 camera discovery” di `StationSettings`.
  - [x] Tombol discovery menampilkan loading, result table, status text labels, action hints.
  - [x] Per station dapat memilih kamera terdeteksi dan assign; duplicate assignment error ditampilkan actionable.
  - [x] UI tidak menampilkan raw stderr/command penuh; render teks dengan React escaping, jangan `dangerouslySetInnerHTML`.
  - [x] Tampilkan assigned camera persisted: name, port, runtime, connected/last seen bila tersedia.
- [x] Update OpenAPI/docs
  - [x] Tambah paths discovery dan assignment di `docs/api/openapi.yaml`.
  - [x] Tambah schemas `GPhoto2DiscoveryResponse`, `DetectedGPhoto2Camera`, `CameraAssignment`, `CameraAssignmentRequest`.
  - [x] Tambah error responses/actions `GPHOTO2_UNAVAILABLE`, `WSL_UNAVAILABLE`, `USBIPD_CHECK_REQUIRED`, `DUPLICATE_CAMERA_ASSIGNMENT`.
- [x] Backend tests
  - [x] Parser: native output single/multi, no cameras, warnings, malformed rows.
  - [x] Runner: unavailable binary maps to `INSTALL_GPHOTO2`; WSL failure maps `CHECK_WSL`; usbipd missing/check maps safe diagnostic; timeout maps retryable safe error.
  - [x] Store/persistence: assignment saved/reloaded; older `stations.json` without assignment still loads; backup/restore includes assignment.
  - [x] Duplicate validation: same camera identity rejected across station_1/station_2; empty assignments allowed; input folder duplicate regression still rejected.
  - [x] API: auth required, trusted origin for mutation, safe error shape, no raw command leak, assignment response wrapper.
- [x] Frontend/type tests/build validation
  - [x] TypeScript types compile; no implicit `any` for new DTOs.
  - [x] UI handles discovery loading/error/empty/multi-camera states.
  - [x] Run all validation commands and record results.

### Review Findings

- [x] [Review][Patch] Discovery service tidak pernah menjalankan diagnostic `usbipd list`, sehingga AC1/AC2/AC5 tidak terpenuhi untuk kondisi WSL/usbipd. Enum/status/action `usbipd_check_needed` dan `CHECK_USBIPD` ada, tetapi alur `DiscoveryService.Discover` hanya mencoba native `gphoto2` lalu `wsl.exe -- gphoto2 --auto-detect`; tidak ada pemanggilan read-only `usbipd list` atau mapping kondisi kamera belum attached ke `CHECK_USBIPD`/`CHECK_CAMERA_USB_MODE`. [apps/agent/internal/cameras/gphoto_runner.go:111]
- [x] [Review][Patch] Parser `gphoto2 --auto-detect` hanya mengenali baris yang mengandung `usb:`, sehingga kamera/transport non-USB/PTP atau variasi port gPhoto2 lain akan diabaikan dan AC1 “transport usb/ptp/unknown” serta parser tahan header/row berbeda belum terpenuhi. `ParseAutoDetect` mencari `strings.LastIndex(trimmed, "usb:")`; akibatnya baris valid tanpa prefix `usb:` jatuh menjadi diagnostic, bukan `DetectedCamera` dengan `transport: unknown/ptp`. [apps/agent/internal/cameras/gphoto_parser.go:42]
- [x] [Review][Patch] Coverage API untuk discovery/assignment yang diklaim di story belum ada. Tidak ditemukan `apps/agent/internal/api/cameras_test.go` atau test lain untuk auth required, trusted origin, safe error shape, duplicate assignment response, dan no raw command leak, padahal AC6 secara eksplisit mensyaratkan API tests. [apps/agent/internal/api]

## Dev Notes

### Epic 7 Context

Epic 7 memperkenalkan managed gPhoto2 camera tethering, tetapi tetap mempertahankan arsitektur lokal yang sudah dibangun: kamera/tether hanya menghasilkan JPG ke station input folder, lalu folder watcher menjadi source of truth untuk ingestion. Story 7.1 hanya mencakup discovery dan assignment. Story berikutnya akan mengurus supervised tether listener, readiness/test capture, dashboard status/control, reconnect/recovery, dan real hardware smoke test.

Implikasi penting: developer tidak boleh membuat shortcut dari gPhoto2 langsung ke session/photo/processing/upload. Output dari kamera tetap harus masuk ke `station.input_folder`, kemudian pipeline existing `watcher/ingestion -> sessions -> photos -> processing -> upload` yang menangani routing, stable file detection, original-first save, LUT, quarantine, dan Google Drive.

### Current Architecture yang Wajib Diikuti

- Frontend: `apps/web`, Next.js App Router, TypeScript, TanStack Query, Tailwind, shadcn/ui primitives.
- Backend/local worker: `apps/agent` Go service. Semua filesystem, command invocation, worker, credential, dan state mutation berada di Go.
- API: REST under `/api`, SSE under `/events`, success wrapper `{data}`, error wrapper `{error:{code,message,action,details}}`, JSON `snake_case`.
- Auth/security: local session cookie/PIN gate; state-changing endpoints harus `RequireAuth` + `RequireTrustedOrigin`.
- Event names dot notation; if adding event use `camera.discovery_completed` or `station.updated`, never ad-hoc event shape.
- Browser tidak boleh menjalankan command, membaca local filesystem, atau memegang credential/service keys.

### File References — Existing Code to Read/Modify

- `apps/agent/internal/stations/store.go`
  - Current station model: `StationID`, `Name`, `DeviceIdentifier`, `InputFolder`, `BackgroundName`, `DefaultLUTPath`, `OutputRule`.
  - Existing validation includes required fields, output rule safety, duplicate input folder normalization.
  - Story 7.1 likely extends this with camera assignment or adds adjacent store.
- `apps/agent/internal/stations/persistence.go`
  - Persists `local-data/config/stations.json` with `ConfigFile{Version,SavedAt,Stations}` using temp file + atomic rename.
  - Current `configVersion = 1`; if schema changes, add backward compatibility rather than breaking existing config.
  - Backup/restore validation requires exactly three station IDs.
- `apps/agent/internal/api/stations.go`
  - `GET /api/stations` and `PUT /api/stations/{station_id}` pattern, activity log, `station.updated` event.
  - Reuse error response helpers and trusted-origin patterns.
- `apps/agent/internal/api/health.go`
  - Router registration lives here. Add new camera routes here consistently.
- `apps/agent/internal/api/response.go`
  - Use existing helpers (`writeData`, `writeAPIError`, `writeAPIErrorWithDetails`) for contract consistency.
- `apps/agent/internal/activity/store.go`
  - Record operator actions with safe messages. Good action names: `camera.discovery_completed`, `camera.discovery_failed`, `station.camera_assignment_updated`, `station.camera_assignment_failed`.
- `apps/agent/internal/events/event.go` and `events/broker.go`
  - Use existing event wrapper if publishing discovery/assignment updates.
- `apps/web/src/lib/api/client.ts`
  - Add DTO types and API calls for discovery/assignment. Keep snake_case response types.
- `apps/web/src/features/stations/station-settings.tsx`
  - Existing station config panel; extend with camera discovery/assignment while preserving readiness/watch/session controls.
- `docs/api/openapi.yaml`
  - Update with endpoints/schemas/errors.

### Spike Specs and Lessons to Reuse (Do Not Reanimate as Production)

- `_bmad-output/implementation-artifacts/spec-gphoto-helper-diagnostic-spike.md`
  - Prior TS prototype added `GET /api/gphoto/diagnostics`, `POST /api/gphoto/capture`, setup, continuous capture, trigger listener.
  - Useful lessons: no install/driver changes; station output must stay inside input folder; global serialization; JPEG SOI validation for capture; WSL/usbipd reality.
  - Production change: do not copy `src/server/gphoto-helper.ts` into production; Go `apps/agent` is production backend now. Also, one-click setup/bind/attach from spike is NOT allowed in Story 7.1 without explicit approval.
- `_bmad-output/implementation-artifacts/spec-direct-camera-capture-spike.md`
  - Lesson: never claim camera control is supported until capability probe proves it; separate USB detected/storage import/direct capture/gPhoto states; invalid station and concurrent operations must be rejected.
  - Story 7.1 only discovery/assignment, not direct capture.
- `_bmad-output/implementation-artifacts/spec-usb-camera-detection-spike.md`
  - Lesson: OS-level device detection does not mean camera ready for capture/tether; render device data safely; unsupported platform returns clear status.
  - Optional: usbipd diagnostic can enrich action hints, but gPhoto2 detection is authoritative for this story.

### Suggested Production Design

#### Go packages

Recommended package layout:

```text
apps/agent/internal/cameras/
  gphoto_discovery.go
  gphoto_parser.go
  gphoto_runner.go
  assignment.go        # if not embedded in stations
  assignment_test.go
  gphoto_parser_test.go
  gphoto_runner_test.go
apps/agent/internal/api/
  cameras.go
  cameras_test.go
```

If assignment is embedded in `stations`, keep duplicate validation near `stations.Store` so all station config persistence/backup/restore paths enforce it.

#### DTO sketch

```go
type DetectedGPhotoCamera struct {
    IdentityKey string    `json:"identity_key"`
    Name        string    `json:"name"`
    Model       string    `json:"model,omitempty"`
    Port        string    `json:"port"`
    Transport   string    `json:"transport"`
    Runtime     string    `json:"runtime"`
    Connected   bool      `json:"connected"`
    Diagnostics []string  `json:"diagnostics,omitempty"`
    DetectedAt  time.Time `json:"detected_at"`
}

type CameraAssignment struct {
    IdentityKey string    `json:"identity_key"`
    CameraName  string    `json:"camera_name"`
    Port        string    `json:"port"`
    Runtime     string    `json:"runtime"`
    AssignedAt  time.Time `json:"assigned_at"`
    LastSeenAt  *time.Time `json:"last_seen_at,omitempty"`
}
```

Use actual field names that fit code style, but keep public JSON `snake_case`.

#### Identity key guidance

- Preferred key: canonical runtime + canonical gPhoto2 port + normalized camera name.
- Example: `wsl|usb:001,004|sony_alpha_a6000`.
- If port can change after unplug/replug, identity may not be perfectly stable. Document limitation in diagnostics and use name+port for MVP. Do not claim serial number stability unless `gphoto2 --summary` or config can safely extract serial per camera.
- If serial extraction is added, it must use allowlisted args and timeout, and failure should not break discovery list.

#### Command safety guidance

- `exec.CommandContext(ctx, "gphoto2", "--auto-detect")`.
- `exec.CommandContext(ctx, "wsl.exe", "--", "gphoto2", "--auto-detect")` or `wsl.exe -d <safeDistro> -- gphoto2 --auto-detect` if config exists.
- `exec.CommandContext(ctx, "usbipd", "list")` read-only only.
- Never use `cmd /C`, `powershell -Command` with concatenated user input, or shell string from request.
- Limit output with pipe reader or post-read truncation to safe max; record local technical log if needed, but public response sanitized.

### Windows / WSL / usbipd Constraints

- On Windows, gPhoto2 commonly runs inside WSL because native Windows gPhoto2/libusb support may be absent or unreliable.
- A USB camera must often be attached to WSL using usbipd. `usbipd bind` may require elevated/admin terminal; `usbipd attach` changes USB availability between host and WSL. Story 7.1 must not execute either automatically.
- Some Sony A6000 modes expose mass storage/MTP but not PTP remote/tether. Discovery may show no camera even though Windows Device Manager sees a USB device. Return `CHECK_CAMERA_USB_MODE`/`CHECK_USBIPD` guidance, not a false READY.
- usbipd/device bus IDs can change after reconnect. Treat them as diagnostic or part of MVP identity with known limitation; future story/hardware smoke test may refine.
- Never change drivers, install packages, or detach host devices without explicit human-run manual action.

### UX Requirements

- UI must keep status understandable under event pressure. Use text labels plus badges, not color alone.
- Discovery panel should distinguish:
  - gPhoto2 runtime available/unavailable.
  - no cameras detected.
  - cameras detected but unassigned.
  - station assigned to connected camera.
  - station assigned to currently missing camera.
  - duplicate assignment rejected.
- Show action hints as operator-friendly messages, e.g. “Install gPhoto2 in WSL”, “Check WSL distro”, “Attach USB device to WSL manually with approval”, “Connect camera / check USB mode”.
- Do not expose raw stderr blocks; show short sanitized diagnostics.
- Do not remove existing station fields or controls. Readiness, watch validation, start session, backup/restore must continue to work.

### Testing Strategy

Backend tests are highest value because hardware may not be present in CI/local dev:

1. Parser tests for `gphoto2 --auto-detect` fixtures:
   - no camera table.
   - one Sony camera on `usb:001,004`.
   - multiple cameras.
   - warnings before/after table.
   - malformed rows ignored with diagnostic.
2. Runner tests with fake command runner:
   - native binary missing -> `INSTALL_GPHOTO2`.
   - WSL missing -> `CHECK_WSL`.
   - WSL gPhoto2 missing -> `INSTALL_GPHOTO2` with runtime `wsl`.
   - no cameras -> success/empty with `CONNECT_CAMERA`.
   - timeout -> safe retryable error/action.
3. Store/persistence tests:
   - assignment saved and loaded.
   - older config without assignment loads with nil/empty assignment.
   - duplicate assignment across stations rejected.
   - backup/restore includes assignment and still rejects duplicate input folders.
4. API tests:
   - unauthenticated discovery/assignment rejected.
   - untrusted origin rejected for assignment/POST discovery.
   - success wrapper uses `data`.
   - error wrapper uses code/message/action/details and contains no raw shell command.
   - duplicate assignment returns expected code/action.
5. Frontend validation:
   - TypeScript compile and build.
   - UI handles empty/no runtime/no camera/multi-camera/duplicate assignment states.

Manual/hardware validation for developer/operator after code lands:

- Windows host with no WSL/gPhoto2: discovery returns `CHECK_WSL` or `INSTALL_GPHOTO2`, no crash.
- WSL with gPhoto2 but camera not attached: discovery returns `CONNECT_CAMERA` or `CHECK_USBIPD`.
- Camera attached to WSL: discovery shows camera name+port; assign to station_1; restart app; assignment persists.
- Attempt assign same camera to station_2; API rejects duplicate and UI shows actionable error.

### Regression Risks to Avoid

- Reintroducing old TypeScript `src/server` prototype as production path while app now uses Go `apps/agent`.
- Running `usbipd bind/attach`, install commands, or driver changes automatically.
- Letting user-supplied station/camera values become shell args without allowlist/sanitization.
- Breaking existing `PUT /api/stations/{station_id}` required fields or station backup/restore.
- Breaking duplicate input folder validation while adding camera duplicate validation.
- Treating OS-level USB detection as proof gPhoto2/tether is ready.
- Exposing raw local paths, raw stderr, privileged command output, tokens, or credentials in API/SSE/UI.
- Bypassing folder watcher/session routing by creating photo/session records directly from gPhoto2 discovery.
- Claiming camera serial persistence if only name+port was available.

### Recent Git / Project Intelligence

- Recent commits are sparse:
  - `c5ab05b Add from-scratch setup guide`
  - `1dbc6cb Initial Selfstudio camera capture spike`
- Most current implementation exists as working tree files rather than many commits. Developer must inspect current files directly rather than infer from git history.
- Current architecture has already moved production backend to Go (`apps/agent`) while old spike code remains under root `src/`; do not confuse the two.

## File References

### Must read before implementation

- `_bmad-output/planning-artifacts/epics.md` — Epic 7 and Story 7.1-7.6 context.
- `_bmad-output/planning-artifacts/architecture.md` — stack, boundaries, API/SSE/error conventions.
- `_bmad-output/planning-artifacts/prd.md` — camera station, readiness, operational safety requirements.
- `_bmad-output/implementation-artifacts/spec-gphoto-helper-diagnostic-spike.md`
- `_bmad-output/implementation-artifacts/spec-direct-camera-capture-spike.md`
- `_bmad-output/implementation-artifacts/spec-usb-camera-detection-spike.md`
- `apps/agent/internal/stations/store.go`
- `apps/agent/internal/stations/persistence.go`
- `apps/agent/internal/api/stations.go`
- `apps/agent/internal/api/health.go`
- `apps/web/src/lib/api/client.ts`
- `apps/web/src/features/stations/station-settings.tsx`
- `docs/api/openapi.yaml`

### Likely files to create/update

- `apps/agent/internal/cameras/gphoto_discovery.go` (NEW)
- `apps/agent/internal/cameras/gphoto_parser.go` (NEW)
- `apps/agent/internal/cameras/gphoto_runner.go` (NEW)
- `apps/agent/internal/cameras/*_test.go` (NEW)
- `apps/agent/internal/api/cameras.go` (NEW)
- `apps/agent/internal/api/cameras_test.go` (NEW)
- `apps/agent/internal/stations/store.go` (UPDATE, assignment fields/validation if embedded)
- `apps/agent/internal/stations/persistence.go` (UPDATE, schema/backward compatibility if station config extended)
- `apps/agent/internal/stations/store_test.go` / `persistence_test.go` (UPDATE)
- `apps/agent/internal/api/health.go` (UPDATE route registration)
- `apps/web/src/lib/api/client.ts` (UPDATE types/functions)
- `apps/web/src/features/stations/station-settings.tsx` or new child component (UPDATE/NEW)
- `docs/api/openapi.yaml` (UPDATE)

## Project Context Reference

- Project: `selfstudio`.
- User: `alpharize`.
- Current date: 2026-05-20.
- Planning artifacts folder: `_bmad-output/planning-artifacts`.
- Implementation artifacts folder: `_bmad-output/implementation-artifacts`.
- Local production architecture: Go agent (`apps/agent`) + Next.js web (`apps/web`) + local data under `local-data`.

## Dev Agent Record

### Debug Log

- 2026-05-20: Audit kode existing station store/persistence/readiness/watch validation, API routing/security/response, frontend client/settings/hooks, dan spike specs. Keputusan: production path hanya Go `apps/agent`; tidak menghidupkan prototype TS lama.
- 2026-05-20: TDD RED untuk parser/runner gPhoto2 menghasilkan expected compile failures pada package baru `internal/cameras`, lalu GREEN dengan domain types, parser, identity key builder, safe command runner, dan discovery service.
- 2026-05-20: Integrasi assignment ke `stations.Station` dengan field optional `camera_assignment` agar backward-compatible dengan `stations.json` version 1 lama; duplicate non-empty identity ditolak di store.
- 2026-05-20: Tambah API `POST /api/cameras/gphoto2/discover` dan `PUT /api/stations/{station_id}/camera-assignment` dengan auth + trusted origin, error wrapper standar, activity log safe, dan SSE safe payload.
- 2026-05-20: Tambah UI discovery/assignment di Station Settings tanpa raw stderr/HTML injection dan tetap mempertahankan readiness/watch/session controls.
- 2026-05-20: Biome check tidak bisa dijalankan di environment ini (`spawn EINVAL`); validasi utama repo tetap pass via Go tests, web typecheck/build, dan root typecheck.
- 2026-05-20: TDD RED untuk review follow-up: tambah test parser port non-USB/PTP, discovery WSL no-camera yang harus memanggil `usbipd list` read-only, dan API tests discovery/assignment auth/trusted origin/safe error/duplicate/no raw command leak; test awal gagal sesuai temuan review.
- 2026-05-20: GREEN review follow-up: `DiscoveryService.Discover` kini menjalankan diagnostic read-only `usbipd list` saat WSL gPhoto2 tidak menemukan kamera, memetakan indikasi kamera/usbipd missing ke `usbipd_check_needed` + `CHECK_USBIPD`, tanpa bind/attach/install/driver command.
- 2026-05-20: GREEN review follow-up: `ParseAutoDetect` kini parsing row berdasarkan token port gPhoto2 yang aman (`scheme:value`) sehingga port `ptpip:*` menjadi transport `ptp`, port valid lain menjadi transport `unknown`, dan warning/noise tetap diagnostic.
- 2026-05-20: GREEN review follow-up: tambah `apps/agent/internal/api/cameras_test.go` untuk discovery/assignment auth, trusted origin, safe error shape, duplicate assignment response, dan guard no raw command leak.

### File List

- `apps/agent/internal/cameras/types.go` (new)
- `apps/agent/internal/cameras/gphoto_parser.go` (new)
- `apps/agent/internal/cameras/gphoto_runner.go` (new)
- `apps/agent/internal/cameras/gphoto_parser_test.go` (new)
- `apps/agent/internal/cameras/gphoto_runner_test.go` (new)
- `apps/agent/internal/api/cameras.go` (new)
- `apps/agent/internal/api/cameras_test.go` (new)
- `apps/agent/internal/api/health.go` (modified)
- `apps/agent/internal/stations/store.go` (modified)
- `apps/agent/internal/stations/persistence.go` (modified)
- `apps/agent/internal/stations/store_test.go` (modified)
- `apps/agent/internal/stations/persistence_test.go` (modified)
- `apps/web/src/lib/api/client.ts` (modified)
- `apps/web/src/features/stations/station-settings.tsx` (modified)
- `apps/web/src/features/stations/use-gphoto2-discovery-mutation.ts` (new)
- `apps/web/src/features/stations/use-camera-assignment-mutation.ts` (new)
- `docs/api/openapi.yaml` (modified)
- `_bmad-output/implementation-artifacts/sprint-status.yaml` (modified)
- `_bmad-output/implementation-artifacts/7-1-detect-and-assign-gphoto2-cameras-to-stations.md` (modified)

### Change Log

- 2026-05-20: Implemented safe gPhoto2 camera discovery with normalized camera DTOs, runtime/status/action allowlists, no shell command construction, command timeout, output limiting, and safe diagnostics.
- 2026-05-20: Added station camera assignment persistence, duplicate assignment validation, and assignment API endpoint while preserving folder watcher as ingestion source of truth.
- 2026-05-20: Added Station Settings UI panel for gPhoto2 discovery, action hints, camera selection per station, and persisted assignment display.
- 2026-05-20: Updated OpenAPI with discovery/assignment endpoints, schemas, and safe error/action contracts.
- 2026-05-20: Addressed code review findings: added read-only `usbipd list` diagnostic path, expanded gPhoto2 parser for non-USB/PTP/unknown ports, and added backend API tests for discovery/assignment security/error contracts.

### Validation Results

- `cd apps/agent && GOTMPDIR=../../.gotmp go test ./internal/cameras` — FAILED as expected during TDD RED because package/types/functions did not exist yet.
- `cd apps/agent && GOTMPDIR=../../.gotmp2 go test ./internal/stations` — PASS after using alternate GOTMPDIR; initial `.gotmp` run failed with Windows `Access is denied` executing test binary.
- `cd apps/agent && GOTMPDIR=../../.gotmp2 go test ./internal/api ./internal/cameras ./internal/stations` — PASS.
- `cd apps/agent && GOTMPDIR=../../.gotmp2 go test ./...` — first full run had transient cleanup failure in `internal/upload` temp dir; rerun passed.
- `cd apps/agent && GOTMPDIR=../../.gotmp2 go test ./...` — PASS.
- `cd apps/web && npm run typecheck` — PASS.
- `cd apps/web && npm run build` — PASS.
- `npm run typecheck` — PASS.
- Biome check attempted for changed frontend files — unable to run in environment (`spawn EINVAL`).
- `cd apps/agent && GOTMPDIR=../../.gotmp2 go test ./internal/cameras ./internal/api` — FAILED during review TDD/validation: new USBIPD API test exposed missing diagnostic path; cameras package also hit Windows temp executable `Access is denied` in `.gotmp2`.
- `cd apps/agent && GOTMPDIR=../../.gotmp3 go test ./internal/cameras ./internal/api` — cameras PASS; api hit transient Windows temp cleanup issue in unrelated session upload temp directory.
- `cd apps/agent && GOTMPDIR=../../.gotmp3 go test ./internal/api -run 'TestCamera'` — PASS.
- `cd apps/agent && GOTMPDIR=../../.gotmp3 go test ./...` — FAILED once due Windows `compile.exe: Access is denied` transient during api build; unaffected packages passed/cached.
- `cd apps/agent && GOTMPDIR=../../.gotmp4 go test ./...` — PASS.
- `cd apps/web && npm run typecheck` — PASS.
- `cd apps/web && npm run build` — PASS.
- `npm run typecheck` — PASS.
- Biome check attempted for review-changed Go/test files — unable to run in environment (`spawn EINVAL`).
- 2026-05-20 re-review: `cd apps/agent && GOTMPDIR=../../.gotmp5 go test ./...` — PASS.
- 2026-05-20 re-review: `cd apps/web && npm run typecheck` — PASS.
- 2026-05-20 re-review: `cd apps/web && npm run build` — PASS.
- 2026-05-20 re-review: `npm run typecheck` — PASS.

## Completion Note

Story 7.1 implementation complete and marked done after re-review. All prior review patch findings were addressed and validation passed. Discovery/assignment is implemented local-first in Go agent, safely exposes normalized gPhoto2 camera data, persists station camera assignments, blocks duplicate non-empty camera identities, updates Station Settings UI, documents API changes, includes API security/error contract tests, runs read-only `usbipd list` diagnostics only, and parses valid non-USB/PTP/unknown gPhoto2 ports as cameras. No install, driver, privileged usbipd bind/attach, direct capture, tether listener, session/photo routing shortcut, or bypass of folder watcher was added.
