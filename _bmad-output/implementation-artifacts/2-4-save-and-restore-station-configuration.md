# Story 2.4: Save and Restore Station Configuration

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an admin,
I want station configuration to survive app restarts and be restorable from a safe backup,
so that event setup is not lost and I can recover from mistaken station edits.

## Acceptance Criteria

1. Given admin saves any station configuration, when the Go agent restarts, then the three station configurations are loaded from durable local storage instead of reverting to defaults.
2. Given station configuration is saved, when a backup is requested, then Go service writes a timestamped backup file containing all three station configs without secrets.
3. Given a backup file exists, when admin restores it, then Go service validates it before replacing current station configs.
4. Given a restore file is malformed, missing required stations, contains duplicate input folders, unsafe output rules, or invalid fields, when restore is attempted, then current config remains unchanged and an actionable error is returned.
5. Given save, backup, or restore succeeds/fails, when operator checks activity log, then safe activity entries exist without PIN/session/cloud credentials or request bodies.
6. Given station config changes due to restore, when dashboard is open, then station config, station readiness, and event readiness views refresh via SSE invalidation.
7. Given API docs are inspected, when docs are opened, then `docs/api/openapi.yaml` documents persistence/backup/restore endpoint(s), schemas, and relevant errors.

## Tasks / Subtasks

- [x] Add durable station config storage (AC: 1, 4, 5)
  - [x] Define local JSON config file path under local data/config, e.g. `local-data/config/stations.json`.
  - [x] Persist exactly three station configs after successful `PUT /api/stations/{station_id}`.
  - [x] Load station configs at Go agent startup.
  - [x] If config file is missing, initialize defaults and persist or keep defaults with clear behavior.
  - [x] If config file is malformed/invalid at startup, fail safe with clear startup error or use defaults with explicit activity/log message; choose and document behavior.
  - [x] Ensure writes are atomic: write temp file, fsync/close, rename.
  - [x] Do not store secrets; station config should only contain station fields.
- [x] Add backup endpoint (AC: 2, 5, 7)
  - [x] Add protected mutation endpoint, e.g. `POST /api/stations/backup`.
  - [x] Apply trusted Origin guard.
  - [x] Write timestamped backup file under local data/config/backups.
  - [x] Return backup metadata such as filename/path safe display, created_at, and station count.
  - [x] Record safe activity for success/failure.
- [x] Add restore endpoint (AC: 3, 4, 5, 6, 7)
  - [x] Add protected mutation endpoint, e.g. `POST /api/stations/restore`.
  - [x] Accept a local backup filename or path constrained to backup directory; do not accept arbitrary filesystem paths without containment validation.
  - [x] Load backup file, validate exactly three known station IDs and existing station validation rules.
  - [x] Reject duplicate input folders, unsafe output rules, malformed JSON, unknown station IDs, missing station IDs, invalid field lengths, and empty required fields.
  - [x] Apply restore atomically so current in-memory and durable config remain unchanged on failure.
  - [x] Record safe activity for success/failure.
  - [x] Publish `station.updated` and/or `stations.restored` SSE event after successful restore.
- [x] Extend frontend station settings UI (AC: 2, 3, 4, 6)
  - [x] Add backup action button with loading/success/error state.
  - [x] Add restore action with filename/path input or list latest backups if simple to support.
  - [x] Show clear warning that restore replaces all three station configs.
  - [x] Invalidate station config, station readiness, and event readiness queries on restore/backup related SSE events.
- [x] Update OpenAPI documentation (AC: 7)
  - [x] Document backup/restore endpoints, request/response schemas, examples, and error responses.
- [x] Add tests and validation (AC: 1-7)
  - [x] Go unit tests for load missing file, save/load roundtrip, atomic write behavior, malformed file handling, backup creation, restore validation, and restore rollback on failure.
  - [x] API tests for auth, trusted Origin, backup success/failure, restore success/failure, activity, and SSE publication.
  - [x] Frontend typecheck/build validation.
  - [x] Manual smoke: save station config, restart Go agent, verify config persists; create backup; alter station; restore backup; verify dashboard/API restored values.

## Dev Notes

### Scope Boundary

This story persists and backs up station configuration only. Do **not** implement photo ingestion, folder watchers, session start, Supabase persistence, GCS upload, cloud sync, image processing, customer/order workflows, or real device discovery.

### Business Context

Stories 2.1-2.3 made station setup configurable and safe for preflight. However, station configs are still in-memory. For real event prep, the operator must not lose folder/LUT/output setup on app restart, and must be able to recover from accidental edits.

### Previous Story Intelligence

Story 2.1 completed:

- exactly three logical stations with stable IDs:
  - `station_1`
  - `station_2`
  - `station_3`
- station config API:
  - `GET /api/stations`
  - `PUT /api/stations/{station_id}`
- validation for required fields, duplicate input folder, unsafe output rule, field lengths
- safe activity logging and `station.updated` SSE

Story 2.2 completed:

- station readiness API:
  - `GET /api/stations/{station_id}/readiness`
  - `POST /api/stations/{station_id}/readiness/check`
- output rule probing derives safe path from `output_root + output_rule`
- station readiness UI and SSE invalidation

Story 2.3 completed:

- event readiness checklist API:
  - `GET /api/readiness`
  - `POST /api/readiness/check`
- checklist includes per-station required checks and root storage item
- checklist SSE `readiness.checked`
- event readiness UI and disabled session-start placeholder CTA

Important carry-forward review lessons:

- API docs must match actual error codes.
- State-changing cookie-auth endpoints require trusted Origin guard.
- Restore/backup endpoints must never accept arbitrary path traversal.
- Query invalidation must cover station config, station readiness, and event readiness.
- Activity logs must not include secrets or raw request bodies.
- Atomic write/rollback behavior matters for operator trust.

### Architecture Requirements

From `architecture.md`:

- Go local service owns local filesystem and station state.
- Next.js dashboard uses REST commands and SSE invalidation.
- API success responses use `{data}` wrapper.
- API errors use `{error:{code,message,action,details}}`.
- JSON uses `snake_case`.
- SSE event names use dot notation.
- Dashboard status/action text must not rely on color only.
- Windows is the target OS; path handling must be Windows-safe.

### Suggested Storage Format

Recommended durable file path:

```text
local-data/config/stations.json
```

Recommended backup directory:

```text
local-data/config/backups/
```

Recommended JSON shape:

```json
{
  "version": 1,
  "saved_at": "2026-05-18T10:30:00Z",
  "stations": [
    {
      "station_id": "station_1",
      "name": "Station 1",
      "device_identifier": "Sony A6000 Main",
      "input_folder": "D:/Selfstudio/input/main",
      "background_name": "White",
      "default_lut_path": "D:/Selfstudio/luts/default.cube",
      "output_rule": "{customer_name}/{order_number}/{station_id}"
    }
  ]
}
```

Do not include:

- PIN/password
- session tokens
- Supabase service role
- Google credentials
- request bodies

### Suggested API Contract

```http
POST /api/stations/backup
POST /api/stations/restore
```

Backup response:

```json
{
  "data": {
    "backup": {
      "filename": "stations-20260518-103000.json",
      "created_at": "2026-05-18T10:30:00Z",
      "station_count": 3
    }
  }
}
```

Restore request:

```json
{
  "filename": "stations-20260518-103000.json"
}
```

Restore response:

```json
{
  "data": {
    "restored": true,
    "station_count": 3
  }
}
```

Recommended error codes:

- `STATION_CONFIG_UNAVAILABLE`
- `STATION_BACKUP_FAILED`
- `STATION_RESTORE_FAILED`
- `INVALID_STATION_BACKUP`
- `INVALID_REQUEST`
- `UNAUTHORIZED`
- `UNTRUSTED_ORIGIN`

### Persistence Design Guidance

Recommended approach:

- Add persistence responsibility near `apps/agent/internal/stations`, e.g. `persistence.go`.
- Keep `stations.Store` responsible for validation and in-memory state.
- Add constructor/helper that loads persisted state then initializes store.
- Add store method to replace all configs atomically after validation, e.g. `ReplaceAll([]Station) error`.
- Persist after successful single-station update and after successful restore.
- On backup, snapshot current store state and write a backup JSON file.
- On restore, load file into candidate stations, validate all candidates in a separate temporary store, then swap/persist only if valid.

### Windows-Safe Path Guidance

- Use `filepath.Clean`, `filepath.Abs`, and containment checks for backup paths.
- Restore should accept filename inside configured backup directory, not arbitrary paths.
- Reject separators or traversal in filename unless explicitly supporting a contained relative path.
- Use timestamped filenames without `:` because Windows filenames cannot contain colon.

### Activity Logging Requirements

Recommended action types:

- `station.config_saved`
- `station.config_save_failed`
- `station.config_backup_created`
- `station.config_backup_failed`
- `station.config_restored`
- `station.config_restore_failed`

Use safe messages only. Backup filename is acceptable; avoid full sensitive paths if possible.

### SSE Requirements

On successful restore, publish:

- `stations.restored`
- `entity_type: "station_config"`
- `entity_id: "all"`

Frontend should invalidate:

- station list query
- station readiness queries
- event readiness query
- activity query if relevant

### Existing Files Likely to Update

Read before editing:

- `apps/agent/internal/stations/store.go`
- `apps/agent/internal/stations/readiness.go`
- `apps/agent/internal/api/stations.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/web/src/features/stations/station-settings.tsx`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`

Suggested new files:

```text
apps/agent/internal/stations/persistence.go
apps/agent/internal/stations/persistence_test.go
apps/agent/internal/api/station_config_backup.go
apps/agent/internal/api/station_config_backup_test.go
apps/web/src/features/stations/use-station-backup-mutation.ts
apps/web/src/features/stations/use-station-restore-mutation.ts
```

### Testing Standards Summary

Go tests:

- Use `t.TempDir()` for config/backups.
- Verify save/load roundtrip persists all three stations.
- Verify malformed config/backup rejected.
- Verify missing/unknown stations rejected.
- Verify duplicate input folders rejected.
- Verify unsafe output rules rejected.
- Verify restore rollback: current store unchanged and durable file unchanged after invalid restore.
- Verify backup filename is Windows-safe and contained in backup directory.
- Verify protected API routes and trusted Origin.

Frontend validation:

- `npm run typecheck`
- `npm run build`

Manual smoke:

- Save station config.
- Restart Go agent.
- Verify station config persists via `GET /api/stations`.
- Create backup.
- Change station config.
- Restore backup.
- Verify restored values and SSE-driven dashboard refresh.

### References

- [Source: `_bmad-output/planning-artifacts/epics.md` → Epic 2 / Story 2.4]
- [Source: `_bmad-output/planning-artifacts/architecture.md` → API/SSE/Error Handling]
- [Source: `_bmad-output/implementation-artifacts/2-1-configure-three-camera-stations.md` → station config foundation]
- [Source: `_bmad-output/implementation-artifacts/2-2-validate-station-paths-luts-and-device-readiness.md` → station readiness]
- [Source: `_bmad-output/implementation-artifacts/2-3-create-event-readiness-checklist.md` → event readiness checklist]


### Review Findings

- [x] [Review][Patch] Restore can leave in-memory station config changed when durable save fails [apps/agent/internal/stations/persistence.go]
- [x] [Review][Patch] Station update mutates in-memory state before persistence and can show failed update after 500 response [apps/agent/internal/api/stations.go]
- [x] [Review][Patch] `stations.restored` SSE invalidates readiness for entity_id `all` instead of all stations [apps/web/src/features/health/health-dashboard.tsx]
- [x] [Review][Patch] Restore mutation success path omits per-station readiness invalidation [apps/web/src/features/stations/use-station-restore-mutation.ts]
- [x] [Review][Patch] Backup filenames can collide within the same second and overwrite backups [apps/agent/internal/stations/persistence.go]
- [x] [Review][Patch] Backup/save atomic write does not handle existing Windows target robustly and does not fsync parent directory [apps/agent/internal/stations/persistence.go]
- [x] [Review][Patch] Restore SSE payload hard-codes station_count instead of publishing actual restored count [apps/agent/internal/api/station_config_backup.go]
- [x] [Review][Patch] `LoadOrDefault` ignores stat errors other than missing file [apps/agent/internal/stations/persistence.go]
- [x] [Review][Patch] Frontend backup/restore responses lack runtime validation [apps/web/src/lib/api/client.ts]
- [x] [Review][Patch] OpenAPI omits restore `415 UNSUPPORTED_MEDIA_TYPE` response [docs/api/openapi.yaml]
- [x] [Review][Patch] Mojibake appears in user-visible station readiness text [apps/web/src/features/stations/station-settings.tsx]
- [x] [Review][Defer] Add optimistic concurrency/versioning for multi-tab station edits — deferred, beyond Story 2.4 MVP scope
- [x] [Review][Defer] Add restore preview/confirmation token/checksum flow — deferred, UX hardening beyond current acceptance criteria
- [x] [Review][Defer] Tighten broader station path containment policy for input/LUT paths — deferred, broader station safety policy beyond persistence story
- [x] [Review][Defer] Revisit logout/login OpenAPI/security behavior — deferred, pre-existing auth contract outside Story 2.4
- [x] [Review][Defer] Theme CSS consistency cleanup — deferred, pre-existing/global UI polish

## Dev Agent Record

### Agent Model Used

{{agent_model_name_version}}

### Debug Log References

- `cd apps/agent && go test ./...` -> passed.
- `cd apps/web && npm run typecheck && npm run build` -> passed.
- Manual smoke: saved station config with `SELFSTUDIO_LOCAL_DATA_DIR=D:/_Project/selfstudio/local-data/tmp/persist-smoke`, created backup, restarted Go agent, and verified `GET /api/stations` returned persisted `station_1` name/input folder.
### Completion Notes List

- Applied all Story 2.4 review patch findings: rollback on persistence failure, restore/station readiness invalidation, nanosecond backup filenames, Windows replace handling, parent directory sync where supported, actual restore event count, stat error handling, backup/restore response validation, OpenAPI 415 documentation, and mojibake cleanup.
- Review patch validation: `cd apps/agent && go test ./...` -> passed.
- Review patch validation: `cd apps/web && npm run typecheck && npm run build` -> passed.
- Ultimate context engine analysis completed - comprehensive developer guide created.
- Added durable station config persistence under local data config with atomic JSON writes.
- Go agent now loads station config from local storage on startup.
- Station updates persist after successful validation.
- Added station backup and restore APIs with trusted Origin guard, activity logging, restore validation, rollback-safe behavior, and `stations.restored` SSE.
- Added frontend backup/restore controls in station settings.
- Updated OpenAPI with station backup/restore endpoints and schemas.


### Change Log

- 2026-05-18: Created Story 2.4 station persistence/backup/restore context and marked ready for development.
- 2026-05-18: Implemented station persistence/backup/restore and marked ready for review.

### File List

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/station_config_backup.go`
- `apps/agent/internal/api/station_config_backup_test.go`
- `apps/agent/internal/api/stations.go`
- `apps/agent/internal/stations/persistence.go`
- `apps/agent/internal/stations/persistence_test.go`
- `apps/agent/internal/stations/store.go`
- `apps/web/src/app/globals.css`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/features/stations/station-settings.tsx`
- `apps/web/src/features/stations/use-station-backup-mutation.ts`
- `apps/web/src/features/stations/use-station-restore-mutation.ts`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`
