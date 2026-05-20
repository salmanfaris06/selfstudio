# Story 2.2: Validate Station Paths, LUTs, and Device Readiness

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an admin,
I want to validate station folders, LUTs, and device readiness,
so that invalid station setup cannot appear ready.

## Acceptance Criteria

1. Given stations have saved configuration, when admin runs readiness check or opens station status, then Go service checks each station input folder is readable.
2. Given stations have saved configuration, when readiness check runs, then Go service checks each station output folder/root is writable according to the configured output rule context.
3. Given stations have saved configuration, when readiness check runs, then Go service checks each station default LUT file is present and has an allowed LUT file format.
4. Given station readiness has required failures, when dashboard renders station status, then station cannot show `READY` and instead shows actionable failure text such as Fix Folder Path, Fix LUT Path, or Recheck Camera.
5. Given camera/device identifier cannot be verified by a reliable local API yet, when readiness check runs, then system reports device readiness as an explicit `unknown` or placeholder status, not `ready`.
6. Given readiness result changes, when check completes, then Go service records a safe activity entry and publishes `station.updated` or `station.readiness_checked` SSE event.
7. Given API docs are inspected, when docs are opened, then `docs/api/openapi.yaml` documents station readiness endpoint(s), schemas, status values, and relevant error responses.

## Tasks / Subtasks

- [x] Add station readiness model and validator (AC: 1, 2, 3, 4, 5)
  - [x] Add Go readiness types for station-level readiness and individual checks.
  - [x] Define check keys: `input_folder`, `output_folder`, `default_lut`, `device`.
  - [x] Define status values: `ready`, `warning`, `failed`, `unknown`.
  - [x] Validate input folder readability with Windows-safe filesystem checks.
  - [x] Validate output base/root writability without creating customer/session output folders permanently.
  - [x] Validate LUT path presence and allowed extension, e.g. `.cube` for MVP.
  - [x] Mark device check as `unknown` with actionable text unless a reliable local verification exists.
  - [x] Compute aggregate station readiness so any required failure prevents `ready`.
- [x] Add protected readiness API endpoints (AC: 1-7)
  - [x] Add `GET /api/stations/{station_id}/readiness` for one station.
  - [x] Add `POST /api/stations/{station_id}/readiness/check` or equivalent mutation to rerun check.
  - [x] Protect endpoints with local session auth.
  - [x] Apply trusted Origin guard to readiness mutation endpoint.
  - [x] Return `{data}` wrapper and existing `{error}` shape.
  - [x] Record safe activity entry on readiness check success/failure.
  - [x] Publish `station.readiness_checked` or `station.updated` SSE event after check completes.
- [x] Extend frontend station settings/status UI (AC: 4, 5, 6)
  - [x] Add API helper/types for readiness data.
  - [x] Add TanStack Query hook/mutation for readiness.
  - [x] Display readiness status per station with text labels, not color only.
  - [x] Add “Run readiness check” action per station.
  - [x] Show actionable check result text for folder/LUT/device statuses.
  - [x] Invalidate readiness/stations query on relevant SSE event.
- [x] Update OpenAPI documentation (AC: 7)
  - [x] Document readiness endpoint(s), request/response schemas, status enum, and examples.
  - [x] Document unauthorized, untrusted origin, station not found, and validation/system error responses.
- [x] Add tests and validation (AC: 1-7)
  - [x] Add Go tests for readable input folder, missing input folder, writable output location, missing/invalid LUT path, device unknown status, and aggregate not-ready behavior.
  - [x] Add API route tests for auth, untrusted origin, station not found, activity logging, and SSE event publication.
  - [x] Run `cd apps/agent && go test ./...`.
  - [x] Run `cd apps/web && npm run typecheck && npm run build`.
  - [x] Manual smoke: configure station paths to temp folders/LUT, run readiness check, verify dashboard/API statuses and activity entry.

## Dev Notes

### Scope Boundary

This story validates configured station folders, LUT path, and device readiness status. Do **not** implement folder watching/test capture validation, backup/restore station config, event readiness checklist, session start, photo ingestion, image processing, quarantine, or cloud upload.

### Business Context

Story 2.1 lets admin configure the three logical camera stations. Story 2.2 makes that configuration operationally safer by proving that station setup cannot look ready when required paths or LUT settings are invalid. This is the foundation for later event readiness and session-start blocking.

### Previous Story Intelligence

Story 2.1 completed:

- Go `stations.Store` with exactly three logical stations.
- Protected `GET /api/stations` and `PUT /api/stations/{station_id}`.
- Required-field, duplicate input folder, field length, and output-rule validation.
- Activity logging for station config update success/failure.
- `station.updated` SSE publication.
- Frontend station settings UI with TanStack Query.
- Station update route hardening: trusted Origin, JSON content type, malformed JSON handling, safe station ID activity refs.

Important review lessons from Story 2.1:

- Test route-level auth and trusted-Origin behavior, not just handlers.
- Runtime-validate frontend API responses before treating mutations as successful.
- Do not let query refetch overwrite unsaved form edits.
- OpenAPI must document every implemented error code that matters to operator/client behavior.
- Path normalization and Windows path variants matter.

### Architecture Requirements

From `architecture.md`:

- Go service owns station state and all local filesystem checks.
- Next.js must never read local camera folders directly.
- API endpoints live under `/api` and use `{data}` success wrapper.
- API errors use `{error:{code,message,action,details}}` with operator-actionable messages.
- JSON fields use `snake_case`.
- SSE event names use dot notation.
- Dashboard status must use text labels, not color only.
- Critical errors must include specific actions such as Recheck Camera or Fix Folder Path.
- Readiness must prevent invalid state from being shown as READY.

### Suggested API Contract

Recommended endpoints:

```http
GET /api/stations/{station_id}/readiness
POST /api/stations/{station_id}/readiness/check
```

Recommended readiness shape:

```json
{
  "data": {
    "readiness": {
      "station_id": "station_1",
      "status": "failed",
      "label": "Station belum siap",
      "action": "Perbaiki input folder dan LUT lalu jalankan recheck.",
      "checked_at": "2026-05-18T10:30:00Z",
      "checks": [
        {
          "check_key": "input_folder",
          "status": "ready",
          "label": "Input folder bisa dibaca",
          "action": "Tidak ada aksi diperlukan."
        },
        {
          "check_key": "device",
          "status": "unknown",
          "label": "Device identifier belum diverifikasi otomatis",
          "action": "Pastikan kamera/tether software menulis JPG ke input folder."
        }
      ]
    }
  }
}
```

Recommended errors:

- `STATION_NOT_FOUND`
- `READINESS_CHECK_FAILED`
- `UNAUTHORIZED`
- `UNTRUSTED_ORIGIN`
- `STATIONS_UNAVAILABLE`

### Filesystem Validation Guidance

For this story, prefer direct local filesystem checks in Go:

- `input_folder`: path exists, is directory, and can be opened/read.
- `output_folder`: derive the stable base/root from station config and verify it exists or can be created/written safely. If deriving from `output_rule` is ambiguous, validate a configured local output base from config/default local-data output path and document the limitation.
- `default_lut_path`: path exists, is a file, and extension is allowed (`.cube` MVP).
- `device_identifier`: report `unknown`/placeholder; do not fake readiness.

Do not create persistent test artifacts without cleanup. If write probing is needed, create a temp file and remove it immediately.

### Activity Logging Requirements

Recommended action types:

- `station.readiness_checked`
- `station.readiness_check_failed`

Include `station_id` when valid. Do not log secrets or full request bodies. Local paths may be shown only as operational troubleshooting text if needed, but avoid excessive sensitive detail in activity messages.

### SSE Requirements

After readiness check completes, publish one of:

- `station.readiness_checked` with `entity_type: "station"`
- or `station.updated` if dashboard already listens for station updates

Frontend should invalidate readiness/stations queries on this event.

### Existing Files Likely to Update

Read before editing:

- `apps/agent/internal/stations/store.go`
- `apps/agent/internal/api/stations.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/security.go`
- `apps/agent/internal/activity/store.go`
- `apps/agent/internal/events/event.go`
- `apps/agent/internal/events/broker.go`
- `apps/web/src/features/stations/station-settings.tsx`
- `apps/web/src/features/stations/use-stations-query.ts`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`

### Frontend Guidance

Suggested files:

```text
apps/web/src/features/stations/use-station-readiness-query.ts
apps/web/src/features/stations/use-run-readiness-check-mutation.ts
```

It is acceptable to render readiness inside `StationSettings` for this story if the component remains maintainable. If it becomes crowded, extract a `StationReadinessCard` component.

### Testing Standards Summary

Go tests:

- Use temporary directories/files from `t.TempDir()` for filesystem checks.
- Test missing directory/file paths.
- Test invalid LUT extension.
- Test required failure prevents aggregate `ready`.
- Test device status is `unknown`, not `ready`.
- Test protected readiness routes and trusted Origin on mutation.
- Test activity log and SSE publication.

Frontend validation:

- `npm run typecheck`
- `npm run build`
- Manual smoke where feasible.

### Project Structure Notes

- Keep filesystem readiness checks in Go, never in Next.js.
- Keep station readiness logic near station domain/service code.
- Keep UI under `apps/web/src/features/stations`.
- Preserve Story 2.1 station config behavior and Epic 1 auth/health/activity behavior.

### References

- [Source: `_bmad-output/planning-artifacts/epics.md` → Epic 2 / Story 2.2]
- [Source: `_bmad-output/planning-artifacts/architecture.md` → Requirements to Structure Mapping]
- [Source: `_bmad-output/planning-artifacts/architecture.md` → Error Handling Patterns]
- [Source: `_bmad-output/planning-artifacts/architecture.md` → Project Structure & Boundaries]
- [Source: `_bmad-output/implementation-artifacts/2-1-configure-three-camera-stations.md` → Previous Story Intelligence]


### Review Findings

Applied Story 2.2 code review patches:

- [x] [Review][Patch] `station.readiness_checked` SSE now invalidates station and readiness queries.
- [x] [Review][Patch] Output readiness now derives a safe probe path from `output_root + output_rule` using sample placeholder values.
- [x] [Review][Patch] Output readiness no longer creates persistent directories; it requires the derived output directory to already exist and cleans up probe files.
- [x] [Review][Patch] Output write probe now checks close/remove errors.
- [x] [Review][Patch] LUT extension check is case-insensitive for Windows-first usage.
- [x] [Review][Patch] LUT check now opens the file to validate readability.
- [x] [Review][Patch] Failed aggregate readiness records failure activity instead of success.
- [x] [Review][Patch] Readiness button is disabled while station form has unsaved edits.
- [x] [Review][Patch] Saving station invalidates readiness query.
- [x] [Review][Patch] Readiness polling interval removed to avoid repeated filesystem probes.
- [x] [Review][Patch] Frontend runtime readiness validation now checks status enum, timestamps, check shape, and required check keys.
- [x] [Review][Patch] API tests now use `filepath.Join`, check `store.Update` errors, and removed mutable `osWriteFile` test global.
- [x] [Review][Patch] OpenAPI now documents `READINESS_CHECK_FAILED` examples and readiness response example.

Deferred/noise:

- [ ] [Review][Defer] GET `/readiness` still computes current filesystem readiness rather than returning cached last-known readiness. Validator is now non-persistent except temp file probe in existing derived folder; cached readiness persistence belongs with durable station config/state follow-up.
- [ ] [Review][Defer] Deeper Windows reparse-point/symlink hardening is deferred until real watcher/session filesystem safety hardening.

## Dev Agent Record

### Agent Model Used

{{agent_model_name_version}}

### Debug Log References

- `cd apps/agent && go test ./...` -> passed.
- `cd apps/web && npm run typecheck && npm run build` -> passed.
- Review patch validation `cd apps/agent && go test ./...` -> passed.
- Review patch validation `cd apps/web && npm run typecheck && npm run build` -> passed.
- Manual smoke exercised login, station update, readiness check endpoint, and readiness activity fetch. Initial ad-hoc bash path smoke exposed path/env setup issue; patched smoke with `SELFSTUDIO_LOCAL_DATA_DIR=D:/_Project/selfstudio/local-data` and pre-created output rule folder returned aggregate `warning` with input/output/LUT ready and device unknown.
### Completion Notes List

- Ultimate context engine analysis completed - comprehensive developer guide created.
- Added station readiness domain model and validator for input folder, output folder, default LUT, and device placeholder checks.
- Added protected readiness API endpoints for `GET /api/stations/{station_id}/readiness` and `POST /api/stations/{station_id}/readiness/check`.
- Readiness mutation records safe activity and publishes `station.readiness_checked` SSE events.
- Added frontend readiness query/mutation hooks and station readiness UI inside station settings.
- Updated OpenAPI with readiness endpoints and schemas.
- Applied Story 2.2 code review patches for SSE invalidation, output-rule probing, non-persistent output checks, LUT readability/case handling, activity semantics, frontend stale-state guards, runtime validation, tests, and OpenAPI examples.


### Change Log

- 2026-05-18: Created Story 2.2 station readiness context and marked ready for development.
- 2026-05-18: Implemented station readiness validation and marked ready for review.
- 2026-05-18: Applied Story 2.2 code review patches.

### File List

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/readiness.go`
- `apps/agent/internal/api/readiness_test.go`
- `apps/agent/internal/stations/readiness.go`
- `apps/agent/internal/stations/readiness_test.go`
- `apps/agent/internal/stations/store.go`
- `apps/web/src/app/globals.css`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/features/stations/station-settings.tsx`
- `apps/web/src/features/stations/use-run-readiness-check-mutation.ts`
- `apps/web/src/features/stations/use-station-readiness-query.ts`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`
