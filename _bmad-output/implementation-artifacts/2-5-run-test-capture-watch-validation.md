# Story 2.5: Run Test Capture Watch Validation

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an admin,
I want to run test watch validation for each station,
so that folder watcher behavior is verified before an event without assigning test photos to a real customer session.

## Acceptance Criteria

1. Given an authenticated admin opens station settings, when a station has valid saved folders, then the admin can run a validation-only watch test for that station.
2. Given admin starts test validation, when a JPG file is already present or newly placed in the configured station input folder, then the Go service detects a stable JPG for that station and records validation result with station ID, source path, detected timestamp, stable status, and validation timestamp.
3. Given duplicate filesystem events or repeated scan observations occur for the same test JPG during one validation run, when validation completes, then only one validation result is recorded for that source file.
4. Given no stable JPG is detected before timeout, when validation ends, then the result is `failed` or `warning` with actionable text explaining that no stable JPG was detected.
5. Given test validation detects a JPG, when validation completes, then the file is never routed to a real session, never copied to customer output, and result is clearly labeled validation-only.
6. Given validation succeeds or fails, when operator checks activity log, then safe activity entries are recorded without PIN/session/cloud credentials or request bodies.
7. Given dashboard is open, when validation result changes, then station validation/readiness UI refreshes via SSE-driven invalidation.
8. Given API docs are inspected, when docs are opened, then `docs/api/openapi.yaml` documents test watch validation endpoint(s), schemas, statuses, timeout behavior, and errors.

## Tasks / Subtasks

- [x] Add test watch validation domain model (AC: 1-5)
  - [x] Add validation status enum: `ready`, `running`, `success`, `warning`, `failed`.
  - [x] Add result fields: station_id, status, label, action, source_path, detected_at, stable_at, validated_at, validation_only.
  - [x] Add per-run dedupe identity for source path so duplicate events/observations produce one result.
  - [x] Keep validation-only state separate from session/photo ingestion state.
- [x] Implement Go validation service (AC: 2-5)
  - [x] Read station config from `stations.Store`.
  - [x] Validate station exists.
  - [x] Verify input folder exists/readable before scanning.
  - [x] Scan for `.jpg` / `.jpeg` files case-insensitively in the station input folder.
  - [x] Detect file stability by observing unchanged size and modified timestamp across a short interval.
  - [x] Use bounded timeout so request cannot hang indefinitely.
  - [x] Avoid moving, copying, routing, or mutating the detected JPG.
  - [x] Return warning/failed result if no stable JPG is found.
- [x] Add API endpoint(s) (AC: 1-8)
  - [x] Add protected mutation endpoint, e.g. `POST /api/stations/{station_id}/validation/watch-test`.
  - [x] Apply trusted Origin guard.
  - [x] Optionally accept JSON request with timeout/stability interval within safe bounds; otherwise use safe defaults.
  - [x] Return `{data:{validation:{...}}}`.
  - [x] Return structured errors for unknown station, invalid request, unreadable input folder, and validation unavailable.
- [x] Add activity logging and SSE (AC: 6-7)
  - [x] Record `station.validation_succeeded` on success.
  - [x] Record `station.validation_failed` on warning/failure/error.
  - [x] Publish `station.validation_completed` SSE with station_id and validation summary.
  - [x] Ensure activity messages contain safe operational info only, no raw request body or secrets.
- [x] Extend frontend station UI (AC: 1, 4, 5, 7)
  - [x] Add “Run test watch validation” button per station.
  - [x] Disable while station form has unsaved edits.
  - [x] Show validation-only result with status, label, action, detected source path if available, and timestamps.
  - [x] Make no-session/no-routing wording explicit.
  - [x] Invalidate validation/readiness/stations/event readiness queries on `station.validation_completed` SSE.
- [x] Update OpenAPI (AC: 8)
  - [x] Document validation endpoint, request/response schemas, examples, statuses, and error responses.
- [x] Add tests and validation (AC: 1-8)
  - [x] Go unit tests for stable JPG detection, no JPG timeout, duplicate observation dedupe, uppercase extensions, unreadable/missing folder, and no file mutation.
  - [x] API tests for auth, trusted Origin, unknown station, success, timeout/no file, activity, and SSE publication.
  - [x] Frontend typecheck/build validation.
  - [x] Manual smoke: configure station input temp folder, place stable JPG, run validation, verify validation-only success, activity, and SSE/UI refresh.

## Dev Notes

### Scope Boundary

This story implements a **validation-only watch test**. Do **not** implement real folder watcher daemon, active session routing, photo records, quarantine, original/graded output copying, image processing, or customer delivery. Those belong to Epics 3-5.

The validation may use a bounded scan/poll approach for MVP instead of a long-running watcher if that is simpler and safer. The user-facing behavior must still validate that the configured input folder can produce stable JPG detection before event operation.

### Business Context

Epic 2 needs confidence that each camera station’s input folder can be observed before the event starts. A test JPG should prove “the system can see a stable JPG from this station” without accidentally sending the file to a customer session. This reduces setup risk before live capture begins.

### Previous Story Intelligence

Completed Story 2.1:

- exactly three logical stations:
  - `station_1`
  - `station_2`
  - `station_3`
- station config API:
  - `GET /api/stations`
  - `PUT /api/stations/{station_id}`
- duplicate input folder validation
- `station.updated` SSE

Completed Story 2.2:

- station readiness API:
  - `GET /api/stations/{station_id}/readiness`
  - `POST /api/stations/{station_id}/readiness/check`
- station readiness checks input folder, output folder, default LUT, and device unknown/ready state
- readiness UI already exists inside station settings

Completed Story 2.3/2.4 implementation history:

- Event readiness checklist exists with `GET /api/readiness` and `POST /api/readiness/check`.
- Station config persistence/backup/restore exists.
- Dashboard uses TanStack Query and SSE invalidation.
- Activity logging exists and must remain safe.

Important carry-forward review lessons:

- Mutation endpoints require local auth + trusted Origin.
- API docs must match actual error behavior.
- Runtime validation on frontend should reject malformed API payloads.
- SSE invalidation must update station-specific and event readiness views.
- Do not log request bodies or secret-like values.
- Windows path behavior matters.

### Architecture Requirements

From `architecture.md`:

- Go local service owns filesystem access, watchers/scanners, and future photo ingestion.
- Frontend must not invent workflow state machines.
- REST API commands use `{data}` wrapper.
- API errors use `{error:{code,message,action,details}}`.
- JSON fields use `snake_case`.
- SSE event names use dot notation.
- Critical status must use text labels, not color only.
- Windows is target OS.

### Suggested API Contract

Endpoint:

```http
POST /api/stations/{station_id}/validation/watch-test
```

Optional request:

```json
{
  "timeout_ms": 5000,
  "stability_ms": 500
}
```

Recommended bounds:

- timeout minimum: 1000ms
- timeout maximum: 15000ms
- stability minimum: 200ms
- stability maximum: 3000ms

Response:

```json
{
  "data": {
    "validation": {
      "station_id": "station_1",
      "status": "success",
      "label": "Stable JPG terdeteksi",
      "action": "Validation-only: file tidak diroute ke session.",
      "source_path": "D:/Selfstudio/input/main/test.jpg",
      "detected_at": "2026-05-18T10:30:00Z",
      "stable_at": "2026-05-18T10:30:01Z",
      "validated_at": "2026-05-18T10:30:01Z",
      "validation_only": true
    }
  }
}
```

Failure/no file response can still be `200` with failed validation result:

```json
{
  "data": {
    "validation": {
      "station_id": "station_1",
      "status": "failed",
      "label": "Belum ada stable JPG terdeteksi",
      "action": "Tempatkan test JPG di input folder station lalu jalankan validation lagi.",
      "source_path": null,
      "detected_at": null,
      "stable_at": null,
      "validated_at": "2026-05-18T10:30:05Z",
      "validation_only": true
    }
  }
}
```

Recommended error codes:

- `STATION_NOT_FOUND`
- `INVALID_VALIDATION_REQUEST`
- `VALIDATION_UNAVAILABLE`
- `INPUT_FOLDER_UNAVAILABLE`
- `UNAUTHORIZED`
- `UNTRUSTED_ORIGIN`
- `UNSUPPORTED_MEDIA_TYPE`

Recommended SSE:

```json
{
  "event_type": "station.validation_completed",
  "entity_type": "station",
  "entity_id": "station_1",
  "payload": {
    "validation": { ... }
  }
}
```

Recommended activity action types:

- `station.validation_succeeded`
- `station.validation_failed`

### Stability Detection Guidance

Minimum acceptable algorithm for this story:

1. List files in the configured station input folder.
2. Filter regular files with `.jpg` / `.jpeg`, case-insensitive.
3. Sort by modified time descending or name for deterministic behavior.
4. For each candidate, stat once, wait `stability_ms`, stat again.
5. Stable if size and modified time are unchanged and size > 0.
6. Dedupe by cleaned absolute path during the validation run.
7. Return first stable candidate.
8. If none stable before timeout, return failed validation-only result.

Do not open/process/decode JPG bytes in this story unless needed for readability; file stability is sufficient.

### Frontend Guidance

Existing likely files:

- `apps/web/src/features/stations/station-settings.tsx`
- `apps/web/src/features/stations/use-station-readiness-query.ts`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`

Suggested new file:

```text
apps/web/src/features/stations/use-run-watch-validation-mutation.ts
```

UI should:

- place button near station readiness controls
- disable button while station form is dirty
- show result with status text, label, action, source path, detected/stable/validated timestamps
- explicitly say “Validation-only: tidak membuat session/photo/customer output”

### Testing Standards Summary

Go tests:

- Use `t.TempDir()`.
- Create small `.jpg` file bytes; no real camera required.
- Use short stability/timeout durations in tests.
- Assert source file still exists and content unchanged after validation.
- Assert uppercase `.JPG` accepted.
- Assert duplicate observations do not create multiple results in one run.
- Assert no file gives failed validation result, not panic/hang.
- API tests cover auth, trusted Origin, station not found, success, failure/no file, activity, SSE.

Frontend validation:

- `npm run typecheck`
- `npm run build`

Manual smoke:

- Configure station input folder to temp folder.
- Put `test.jpg` inside.
- Run watch validation from UI or API.
- Verify result is success and validation-only.
- Verify file remains in place and no session/output file is created.
- Verify activity and SSE/UI refresh.

### References

- [Source: `_bmad-output/planning-artifacts/epics.md` → Epic 2 / Story 2.3 Run Test Capture Watch Validation]
- [Source: `_bmad-output/planning-artifacts/architecture.md` → Go filesystem ownership, REST/SSE, error contract]
- [Source: `_bmad-output/implementation-artifacts/2-1-configure-three-camera-stations.md` → station config foundation]
- [Source: `_bmad-output/implementation-artifacts/2-2-validate-station-paths-luts-and-device-readiness.md` → station readiness]
- [Source: `_bmad-output/implementation-artifacts/2-3-create-event-readiness-checklist.md` → event readiness/SSE invalidation]
- [Source: `_bmad-output/implementation-artifacts/2-4-save-and-restore-station-configuration.md` → persistence/backup/restore patterns]


### Review Findings

- [x] [Review][Patch] OpenAPI documentation was missing for watch validation endpoint [docs/api/openapi.yaml]
- [x] [Review][Patch] Unstable JPG could be permanently skipped during one validation run [apps/agent/internal/stations/watch_validation.go]
- [x] [Review][Patch] Unreadable input folder during scan returned 200 failed validation instead of structured `INPUT_FOLDER_UNAVAILABLE` error [apps/agent/internal/stations/watch_validation.go]
- [x] [Review][Patch] Request cancellation was ignored during bounded validation sleeps [apps/agent/internal/stations/watch_validation.go]
- [x] [Review][Patch] Validation could exceed configured timeout by entering stability sleep near deadline [apps/agent/internal/stations/watch_validation.go]
- [x] [Review][Patch] Frontend success validation accepted missing source/detected/stable fields [apps/web/src/lib/api/client.ts]
- [x] [Review][Patch] Frontend sent watch validation POST without explicit JSON body/content type [apps/web/src/lib/api/client.ts]
- [x] [Review][Patch] Output rule validation accepted Windows absolute paths on non-Windows runtime [apps/agent/internal/stations/store.go]
- [x] [Review][Defer] Make validation asynchronous or rate-limited for many repeated validation requests — deferred, current story uses bounded synchronous validation and local authenticated operator scope
- [x] [Review][Defer] Require newly created/modified file or tokenized filename instead of accepting existing JPGs — deferred, story explicitly allows already-present JPGs
- [x] [Review][Defer] Validate JPEG magic/header bytes instead of extension-only detection — deferred, story stability guidance says extension-based stable file detection is sufficient
- [x] [Review][Defer] Redact local source path in validation response/UI — deferred, AC requires source path result and local admin context accepts path visibility
- [x] [Review][Defer] Broader store invariant changes for ReplaceAll/List ordering — deferred, pre-existing store hardening outside current validation story
- [x] [Review][Defer] Revisit server-wide WriteTimeout 0 for SSE vs normal API endpoints — deferred, pre-existing SSE architecture tradeoff from Story 1.3
- [x] [Review][Defer] onAuthExpired prop still unused — deferred, pre-existing auth UX issue outside watch validation

## Dev Agent Record

### Agent Model Used

{{agent_model_name_version}}

### Debug Log References

- `cd apps/agent && go test ./...` -> passed.
- `cd apps/web && npm run typecheck && npm run build` -> passed.
### Completion Notes List

- Applied Story 2.5 review patches: OpenAPI docs, context cancellation, timeout guard before stability sleep, unstable-file recheck, structured unreadable-folder errors, stricter watch validation runtime validation, explicit JSON request body, and cross-runtime Windows absolute output rule rejection.
- Review patch validation: `cd apps/agent && go test ./...` -> passed.
- Review patch validation: `cd apps/web && npm run typecheck && npm run build` -> passed.
- Ultimate context engine analysis completed - comprehensive developer guide created.
- Implemented validation-only watch test domain/service with stable JPG detection, timeout handling, case-insensitive `.jpg/.jpeg`, and source-file preservation.
- Added `POST /api/stations/{station_id}/validation/watch-test` with auth, trusted Origin, activity logging, and `station.validation_completed` SSE.
- Added frontend watch validation button/result panel and SSE invalidation.
- Added Go domain/API tests for success, no file, missing folder, auth, trusted Origin, activity, and SSE.

### File List

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/watch_validation.go`
- `apps/agent/internal/api/watch_validation_test.go`
- `apps/agent/internal/stations/store.go`
- `apps/agent/internal/stations/watch_validation.go`
- `apps/agent/internal/stations/watch_validation_test.go`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/features/stations/station-settings.tsx`
- `apps/web/src/features/stations/use-run-watch-validation-mutation.ts`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`
