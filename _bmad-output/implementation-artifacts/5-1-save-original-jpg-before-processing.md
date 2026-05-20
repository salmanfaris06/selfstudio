# Story 5.1: Save Original JPG Before Processing

Status: done

## Story

Sebagai operator, saya ingin setiap original JPG foto session yang valid disimpan terlebih dahulu, sehingga foto customer tetap aman walaupun grading/LUT processing gagal.

## Acceptance Criteria

1. Given routed session photo exists, when local delivery starts, then system copies/saves original JPG to deterministic customer/order/station folder.
2. Given a photo has not completed original save, when LUT/graded processing would otherwise begin, then LUT/graded processing does not start and no graded job is treated as eligible before original save success.
3. Given original save succeeds, when metadata is persisted, then final persisted photo/processing record never points to a missing local original file after success.
4. Given two source JPGs would produce the same target filename, when originals are saved, then filename collisions are prevented deterministically without overwriting an existing customer file.
5. Given a saved original is inspected through API/session detail state, then traceability includes `source_path` and `local_original_path` plus station/session/photo identity.
6. Given original save fails because source is missing/unreadable, output folder is not writable, disk/path operation fails, or copy verification fails, then the job/photo is marked failed or retryable with an actionable safe error and the UI/API must not report local save success.
7. Given the app restarts after originals were saved or while jobs were pending, when the service starts again, then saved-original state is reloaded/reconciled from durable state and does not create duplicate output files for the same photo.
8. Existing Epic 4 behavior is not regressed: stable JPG detection, active-session routing, quarantine, assignment, persisted photo/quarantine recovery, station summaries, and duplicate identity continue to pass.
9. Tests/build pass for Go agent and relevant web/API contract checks.

## Tasks / Subtasks

- [x] Model original-save/local-delivery state in the Go service. (AC: 1, 2, 3, 5, 6, 7)
  - [x] Extend photo/local-delivery metadata so routed photos can track at minimum: `local_original_path`, original-save status (`pending`/`saving`/`saved_original`/`failed` or equivalent), `last_error`, `attempt_count`, and timestamps.
  - [x] Keep existing `photos.Identity(station_id, source_path, source_size_bytes)` semantics unchanged; do not add timestamps/session/status to duplicate identity.
  - [x] Decide whether to store original-save fields directly on `photos.Photo` or a new `processing_jobs`/local delivery store; document the chosen boundary in code comments and OpenAPI if exposed.
  - [x] Persist any new state under `local-data/state` using the same versioned JSON + atomic temp-write/rename pattern already used by sessions/photos/quarantine.
- [x] Implement deterministic original output path generation. (AC: 1, 4, 5)
  - [x] Use session snapshot data, not live station config, for historical output rules: `session.StationSnapshot.OutputFolder`, `CustomerName`, `OrderNumber`, station identity/name/background.
  - [x] Sanitize customer/order/station path segments for Windows-invalid characters, reserved names, trimming, and length safety; never allow path traversal outside the configured output root.
  - [x] Use a deterministic folder convention, recommended: `<output_root>/<safe_customer>_<safe_order>/<safe_station>/originals/` for original JPGs. If existing output rule code already defines a structure, extend it rather than inventing a conflicting second convention.
  - [x] Preserve source extension as `.jpg`/`.jpeg` normalized consistently; do not convert image data in this story.
  - [x] Implement deterministic collision handling that never overwrites an existing different file: e.g., base source filename, then suffix using short `photo_id` or `-001` style. Prefer `photo_id` suffix for restart idempotency.
- [x] Implement original-first copy service/worker in `apps/agent/internal/processing`. (AC: 1, 2, 3, 6, 7)
  - [x] Add a processing/local-delivery service that consumes routed photos from `photos.Store`; keep all filesystem work in Go, never in Next.js.
  - [x] Copy source JPG to a temp file in the destination directory, flush/close it, then atomically rename to final `local_original_path` so partial originals are never observed as successful outputs.
  - [x] Verify after save that final file exists, is non-zero, and expected size matches source size before marking `saved_original`.
  - [x] Do not move/delete source JPGs; Epic 5 saves originals, later stories handle LUT and retries.
  - [x] If source and target are the same path or already saved, handle idempotently without duplicate copies or overwrites.
  - [x] Ensure original save completes before any future LUT/graded job can run; if adding a queue, graded steps must be gated on `saved_original`.
- [x] Wire original-save lifecycle into ingestion/session flow. (AC: 1, 2, 5, 6)
  - [x] After a non-duplicate route to an active session, enqueue/start original save for that `photo_id`; do not enqueue quarantine records.
  - [x] For manual quarantine assignment that routes a quarantined item to a session, enqueue/start original save for the assigned `photo_id` too.
  - [x] Suppress duplicate route/re-scan events from creating duplicate original-save jobs for the same photo.
  - [x] Publish safe SSE events such as `photo.original_saved` and `queue.updated` using `photo_id`, `station_id`, and `session_id`; never use raw filesystem path as an entity id.
  - [x] Record activity log entries for original-save success/failure using safe operator-readable messages that do not expose unnecessary sensitive path detail.
- [x] Expose status through API/OpenAPI and minimal UI/summary integration only as needed. (AC: 3, 5, 6, 8)
  - [x] Update `docs/api/openapi.yaml` for new photo/local-save fields/events while preserving `{data}` wrapper, `{error:{code,message,action,details}}`, `snake_case` JSON, and dot-notation SSE.
  - [x] Update session detail/live-card API responses so local save/original status can be inspected without breaking existing fields.
  - [x] Update `apps/web/src/lib/api/client.ts` validators/types for any exposed fields/events.
  - [x] If UI changes are needed, keep them minimal and consistent with existing station/session detail patterns; show text labels, not color-only state.
- [x] Implement restart reconciliation for original-save state. (AC: 3, 7)
  - [x] On startup, reload original-save metadata before accepting new scans/jobs.
  - [x] For `saved_original`, verify `local_original_path` still exists and matches expected non-zero/source size; if missing, mark actionable failed/retryable rather than claiming success.
  - [x] For `pending`/`saving` records from a previous process, requeue or mark retryable deterministically; never create a second final original path for the same photo.
  - [x] Recovery must use persisted session/photo state and filesystem checks server-side only.
- [x] Add focused tests and run validation. (AC: 1-9)
  - [x] Go unit tests for path sanitization, output path generation, collision/idempotency behavior, and path traversal prevention.
  - [x] Go tests for copy success: temp file + final rename, size verification, `local_original_path` persisted, source remains untouched.
  - [x] Go tests for failure paths: missing source, unwritable output, verification mismatch if injectable, persistence save failure should not report success.
  - [x] Go restart tests: saved original reload/reconcile, pending job requeue, duplicate route after restart does not create duplicate output.
  - [x] Regression tests proving Epic 4 photo/quarantine persistence, duplicate routing, and assignment still pass.
  - [x] Run `cd apps/agent && go test ./...`; run `cd apps/web && npm run typecheck`; run `cd apps/web && npm run build` if environment allows.

### Review Follow-ups (AI)

- [x] [High] Persisted/API response can report assignment success with stale original-save fields — `apps/agent/internal/api/quarantine.go:154-159` calls `saveAssignedOriginal(photo)` but then returns the pre-save `photo` value in `AssignQuarantineData`, so the immediate assignment API response can omit `local_original_path` and still show `original_save_status: pending` / `processing_status: not_eligible` after the original has been saved. This violates AC5 traceability and can mislead operators/clients.
- [x] [Medium] Ingestion routed response is stale after original save — `apps/agent/internal/ingestion/router.go:13-28` `RouteResult` has no local-original/status fields, and `apps/agent/internal/api/ingestion.go:74-80` appends the pre-save `routeResult` after `saveOriginal`. The scan API therefore cannot inspect `local_original_path` / original-save status for newly routed photos even though the OpenAPI contract marks these fields on routed results. This violates AC5/API contract consistency.
- [x] [High] Existing target with matching size is accepted as saved without content identity verification — `apps/agent/internal/processing/original_saver.go:174-177` treats any pre-existing target file with the expected size as idempotent success. If an operator/external process leaves a different file at the deterministic path with the same byte size, the system will mark `saved_original` and persist `local_original_path` pointing to the wrong original. This violates AC3 and AC4 collision/no-overwrite safety. Fix by verifying content identity (for example hash/byte compare against source) before accepting an existing target as idempotent success; otherwise fail rather than claim success.

## Dev Notes

### Source Requirements

- Epic 5 objective: original-first local delivery, LUT processing, processing status, retry, collision safety, queue/status visibility, and recovery. This story is the foundation: preserve the original before any grading.
- FR31: save original JPG for each valid session photo.
- FR33: save outputs to deterministic customer/order/station folders.
- FR34/NFR8/NFR27: original must be preserved even when LUT processing later fails.
- FR35/FR38/FR54 and NFR14/NFR15/NFR18: track status, prevent filename collisions, recover state after restart, and never leave persisted success pointing at missing final files.
- PRD technical success requires 100% original JPGs saved before graded processing and zero lost files after local save success.

### Current Code Context To Read Before Editing

- `apps/agent/internal/photos/store.go`
  - Current state: routed-photo store only. `Photo` has `photo_id`, `station_id`, `session_id`, `source_path`, `source_size_bytes`, timestamps, `status = routed`, and duplicate flag. No `local_original_path` or processing status exists yet.
  - Story change: add local original-save state or introduce an adjacent processing/local-delivery store. Preserve route duplicate semantics and `Identity(...)` exactly.
  - Must preserve: `Route(...)` duplicate returns existing photo with `Duplicate=true`; `ListBySession`, `CountBySession`, `ListAll`, and persisted load validation must remain compatible.
- `apps/agent/internal/photos/persistence.go`
  - Current state: durable versioned `local-data/state/photos.json` with atomic save/load pattern. Use this pattern for any photo schema change or new processing persistence.
  - Story change: if `Photo` schema changes, handle old records safely; if new store is added, use the same envelope style and fail safely on corrupt state.
- `apps/agent/internal/ingestion/router.go`
  - Current state: routes stable detected photos to active sessions or quarantine. It creates photo records but does not save/copy original files.
  - Story change: original-save enqueue should happen after successful non-duplicate routed results, not inside frontend and not for quarantine. Avoid burying filesystem copy inside pure routing if that makes testing/rollback difficult.
- `apps/agent/internal/api/ingestion.go`
  - Current state: scan endpoint records/publishes detected/routed/quarantined events, suppresses duplicate alerts after Story 4.5, and uses runtime persistence hooks.
  - Story change: hook original-save queue/service after successful route and ensure persistence failure produces actionable errors rather than UI-success-with-undurable-state.
- `apps/agent/internal/api/quarantine.go`
  - Current state: assignment routes quarantined photo to a session and persists photo/quarantine stores transactionally.
  - Story change: manual assignment must also start original save for the newly assigned/routed photo. Preserve Story 4.4 rule: locked-session assignment is allowed only for related `late_photo`; `no_active_session` must not be assignable to arbitrary locked sessions.
- `apps/agent/cmd/selfstudio-agent/main.go` and `apps/agent/cmd/selfstudio-agent/runtime_state.go`
  - Current state: startup loads stations/sessions/photos/quarantine, runs ingestion recovery, and wires transactional photo/quarantine persistence hooks.
  - Story change: initialize original-save processing state before handlers, run original-save recovery/reconciliation after photo/session state loads, and include new state in runtime persistence/rollback if coupled to photo mutations.
- `apps/agent/internal/sessions/store.go`
  - Current state: session snapshots include `OutputFolder`, `OutputRule`, station name/background/default LUT/input folder. Use snapshots for output path generation.
  - Story change: do not use mutable current station config when computing historical output paths for already-routed session photos.
- `apps/agent/internal/api/sessions.go`
  - Current state: session detail/live-card summaries count photos and open quarantine using stores.
  - Story change: expose original/local save status in summaries if required; do not break existing response validators.
- `docs/api/openapi.yaml`
  - Current state: documents ingestion, photos/quarantine/session summary, SSE wrapper, and recovery events.
  - Story change: document any new `local_original_path`, original-save status, queue summary, or SSE event fields.
- `apps/web/src/lib/api/client.ts` and relevant `apps/web/src/features/*`
  - Current state: frontend consumes Go API state and SSE invalidation. It must not read local files directly.
  - Story change: add validators/display for local original save status only through API fields.

### Architecture Guardrails

- Go service owns all filesystem access, workers, processing, recovery, and persistence. Next.js/browser must never copy/read camera folders or local-data files directly.
- Original-first means no LUT/graded processing may start until original save is verified and persisted as successful.
- Do not implement LUT application in this story. Create only the gates/interfaces/status needed so Story 5.2 can safely run after `saved_original`.
- Do not implement cloud/GCS upload in this story. Local original save is independent from cloud upload.
- Do not move/delete source JPGs from station input folders; existing ingestion and recovery rely on source traceability.
- Use temp file + atomic rename for final originals. A partially copied file must never be represented as successful `local_original_path`.
- API/status fields use `snake_case`; status values should be lowercase `snake_case` such as `pending`, `saving`, `saved_original`, `failed`.
- SSE event names use dot notation and safe IDs only. Raw `source_path`/`local_original_path` may be response data where needed, but never `entity_id`.
- Runtime files stay under configured local output/local-data paths, never under `apps/web/public`.
- Treat customer names, order numbers, and photo files as customer data; avoid unnecessary path exposure in logs/activity entries.

### Recommended Implementation Shape

- Add package/files:
  - `apps/agent/internal/processing/original_saver.go` for path generation and copy/verify logic.
  - `apps/agent/internal/processing/original_saver_test.go`.
  - Optional `apps/agent/internal/processing/store.go` + `persistence.go` if not extending `photos.Photo` directly.
- Modify:
  - `apps/agent/internal/photos/store.go` and `persistence.go` if local original fields live on photo records.
  - `apps/agent/internal/api/ingestion.go` and `quarantine.go` to enqueue/start original save after non-duplicate route/assignment.
  - `apps/agent/cmd/selfstudio-agent/main.go` for processing service initialization and restart reconciliation.
  - `apps/agent/internal/api/sessions.go`, `docs/api/openapi.yaml`, and `apps/web/src/lib/api/client.ts` only for exposed state.
- Prefer a small synchronous worker/service first if queue infrastructure is not present; keep dashboard non-blocking by not doing long copies on UI thread (there is no UI thread in Go, but avoid holding API store locks while copying).
- Make persistence failure explicit. If photo state says `saved_original` in memory but cannot persist, return/record failure or rollback; do not let the operator see success that will vanish after restart.
- For collision-safe filenames, a stable name like `<base>__<photo_id>.jpg` is simpler and restart-safe than incrementing suffixes, unless an existing deterministic output rule already exists.

### Previous Story Intelligence

- Story 4.1 established stable scanner behavior: `.jpg/.jpeg`, stable size/modtime wait, zero-byte/unstable files ignored, no file mutation.
- Story 4.2 established routed photo identity. Do not change duplicate identity; timestamps must not affect identity.
- Story 4.3 established quarantine reasons/counts and safe events.
- Story 4.4 established manual quarantine assignment and locked-session eligibility guard. Do not regress: `no_active_session` cannot assign to arbitrary locked sessions.
- Story 4.5 added durable `photos.json` and `quarantine.json`, startup recovery, duplicate scan alert suppression, and transactional multi-store persistence. Reuse this durability pattern and preserve rollback behavior.
- Story 4.5 review fixed a high-risk partial persistence issue; any new processing/original-save persistence must avoid the same class of bug.
- Git history is sparse (`Add from-scratch setup guide`, `Initial Selfstudio camera capture spike`); current story artifacts and workspace code are more authoritative than commit history.

### Latest Technical Notes

- Current architecture document lists Go `1.26.3`, Next.js `16.2.6`, TanStack Query `5.100.10`, Supabase CLI `v2.98.1`, and shadcn CLI v4 as observed tool versions.
- Go `os` file operations are safe for concurrent use but OS limits still apply; keep copy concurrency bounded if a worker queue is introduced.
- For atomic output, a temp-file-then-rename approach is required. `github.com/google/renameio/v2` documents atomic replace semantics, but do not add a dependency unless needed; existing project already uses custom temp-write + rename for JSON persistence, so prefer consistent standard-library implementation for binary copy.

### Testing Requirements

Run at minimum:

- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build` if dependencies/environment allow.

If Windows blocks Go test execution from temp paths with `Access is denied`, use the known workaround from Story 4.4/4.5: `go test -c ./...` then execute generated `*.test.exe` via `cmd.exe /c`.

Required coverage:

- Original save succeeds only after final file exists and matches expected source size.
- Source JPG remains untouched after original save.
- `local_original_path` is persisted and recovered after restart.
- Duplicate scan/route of the same photo does not produce a second final original.
- Collision handling prevents overwrite when two routed photos share the same source basename.
- Missing source/unwritable target/corrupt state returns actionable failure and does not claim success.
- Original-save state gates future graded processing; no graded-eligible state before `saved_original`.
- Existing Epic 4 tests continue to pass.

### Regression Risks To Avoid

- Do not start LUT/graded work before verified original save.
- Do not overwrite existing customer files on filename collision.
- Do not mark `saved_original` before the copy is fully flushed/closed/renamed and verified.
- Do not persist success pointing to a missing/non-zero-invalid final file.
- Do not change route/quarantine duplicate identity semantics.
- Do not copy quarantine/unassigned photos until they are manually assigned/routed to a session.
- Do not make frontend responsible for local file operations or routing decisions.
- Do not expose raw filesystem paths as SSE `entity_id` or noisy activity-log details.
- Do not implement Story 5.2 LUT processing, Story 5.4 retry policy, or Story 6 upload scope beyond minimal states/interfaces required for original-first gating.

## Project Structure Notes

- Expected new files: `apps/agent/internal/processing/original_saver.go`, `apps/agent/internal/processing/original_saver_test.go`, optional processing store/persistence files.
- Expected modified files: `apps/agent/internal/photos/store.go`, `apps/agent/internal/photos/persistence.go`, `apps/agent/internal/api/ingestion.go`, `apps/agent/internal/api/quarantine.go`, `apps/agent/cmd/selfstudio-agent/main.go`, `apps/agent/cmd/selfstudio-agent/runtime_state.go`, `apps/agent/internal/api/sessions.go`, `docs/api/openapi.yaml`, and `apps/web/src/lib/api/client.ts` if fields are exposed.
- Follow existing Go package naming: short lowercase packages, snake_case filenames, business logic outside HTTP handlers.
- Runtime output belongs under configured output/session folders and `local-data/state`; never commit generated photos/state.

## References

- `_bmad-output/planning-artifacts/epics.md` — Epic 5 and Story 5.1 acceptance criteria; FR31-FR39 mapping.
- `_bmad-output/planning-artifacts/prd.md` — Image Processing and Local Storage; technical success; NFR8, NFR14, NFR15, NFR18.
- `_bmad-output/planning-artifacts/architecture.md` — Go service filesystem ownership, local filesystem safety, temp file/atomic rename, state ownership, project boundaries.
- `_bmad-output/implementation-artifacts/4-1-watch-station-input-folders-for-stable-jpgs.md` — stable scanner behavior.
- `_bmad-output/implementation-artifacts/4-2-route-valid-jpgs-to-active-sessions.md` — routed photo identity and duplicate safety.
- `_bmad-output/implementation-artifacts/4-3-quarantine-unassigned-and-late-photos.md` — quarantine behavior and safe events.
- `_bmad-output/implementation-artifacts/4-4-review-and-assign-quarantined-photos.md` — assignment lifecycle and locked-session guard.
- `_bmad-output/implementation-artifacts/4-5-recover-pending-ingestion-and-quarantine-state.md` — durable photo/quarantine persistence and runtime rollback lessons.
- `apps/agent/internal/photos/store.go` — current routed-photo model and identity.
- `apps/agent/internal/photos/persistence.go` — durable photo state pattern.
- `apps/agent/internal/ingestion/router.go` — server-owned routing decisions.
- `apps/agent/internal/api/ingestion.go` — scan/route side effects and persistence hook.
- `apps/agent/internal/api/quarantine.go` — assignment route side effects.
- `apps/agent/internal/sessions/store.go` — session snapshot fields for deterministic output paths.
- `docs/api/openapi.yaml` — API/SSE contract source.

## Dev Agent Record

### Agent Model Used

GPT-5 Codex

### Debug Log References

- 2026-05-19: Review follow-up validation passed with `cd apps/agent && go test ./...`. Web typecheck/build not run because no web files were touched by this follow-up.

- 2026-05-19: `cd apps/agent && go test ./internal/processing ./internal/photos` initially failed due to `OriginalSaver.Save` field/method name collision; renamed persistence callback to `Persist`.
- 2026-05-19: `cd apps/agent && go test ./internal/processing ./internal/photos` initially failed because test session fixtures did not satisfy session validation; fixed fixtures with active status, timer, and timestamps.
- 2026-05-19: `cd apps/agent && go test ./...` initially hit Windows temp execution `Access is denied` for `internal/api`; verified workaround with `go test -c ./internal/api && cmd.exe /c api.test.exe`, then reran full suite successfully.

### Completion Notes List

- ✅ Resolved review finding [High]: assignment API response now returns post-save `Photo` fields from `OriginalSaver.Save`, so `local_original_path`, `original_save_status`, and `processing_status` reflect the persisted saved-original state.
- ✅ Resolved review finding [Medium]: ingestion routed scan responses now hydrate routed results from the post-save photo record and include local-original/status fields defined by the API contract.
- ✅ Resolved review finding [High]: original saver now byte-compares same-size existing targets against the source before accepting idempotent success; mismatched content fails safely and is not overwritten.

- Added original-save state directly to routed `photos.Photo` records (`local_original_path`, `original_save_status`, `last_error`, `attempt_count`, timestamps, and `processing_status`) while preserving existing duplicate identity semantics.
- Added Go-owned original saver under `apps/agent/internal/processing` with deterministic session-snapshot output paths, safe Windows/path sanitization, photo-id collision suffixing, temp-file copy, flush/close, atomic rename, verification, idempotency, and restart reconciliation.
- Wired original save after non-duplicate ingestion routes and manual quarantine assignment; quarantine-only items are not copied until routed/assigned.
- Published safe `photo.original_saved` and `queue.updated` events with safe IDs, and activity log entries for success/failure without using raw paths as entity IDs.
- Exposed original-save fields through existing session detail/photo API shape, OpenAPI, and web TypeScript client types.
- Validations passed: `cd apps/agent && go test ./...`; `cd apps/web && npm run typecheck`; `cd apps/web && npm run build`.

### File List

- apps/agent/cmd/selfstudio-agent/main.go
- apps/agent/internal/api/ingestion.go
- apps/agent/internal/api/quarantine.go
- apps/agent/internal/ingestion/router.go
- apps/agent/internal/photos/persistence.go
- apps/agent/internal/photos/store.go
- apps/agent/internal/processing/original_saver.go
- apps/agent/internal/processing/original_saver_test.go
- apps/web/src/lib/api/client.ts
- docs/api/openapi.yaml
- _bmad-output/implementation-artifacts/5-1-save-original-jpg-before-processing.md
- _bmad-output/implementation-artifacts/sprint-status.yaml

## Change Log

- 2026-05-19: Ultimate context engine analysis completed - comprehensive developer guide created.
- 2026-05-19: Implemented original-first local JPG save lifecycle, API status exposure, restart reconciliation, and validation coverage.
- 2026-05-19: Addressed code review findings - 3 items resolved; refreshed post-save API response state and added same-size existing-target byte identity verification.
