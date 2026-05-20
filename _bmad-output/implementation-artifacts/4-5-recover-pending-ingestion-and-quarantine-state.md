# Story 4.5: Recover Pending Ingestion and Quarantine State

Status: done

## Story

Sebagai operator, saya ingin state ingestion/quarantine yang pending dipulihkan setelah aplikasi restart, sehingga foto yang sudah terdeteksi, belum selesai routing, atau masih dikarantina tidak hilang dan tidak diproses ganda.

## Acceptance Criteria

1. Given app restarts while stable JPGs already exist in station input folders, when the Go service starts and operator runs/observes ingestion recovery, then the scanner/router reconciles filesystem candidates with persisted photo/quarantine state instead of treating restart as a reason to lose or duplicate records.
2. Given routed photos and quarantine items existed before restart, when service starts again, then photo records, quarantine records, assignment state, and duplicate-safe identities are reloaded from durable local state under `local-data/state`.
3. Given the same source JPG is encountered again after restart, when scan/recovery runs, then identity `station_id + normalized source_path + source_size_bytes` prevents duplicate photo records, duplicate quarantine records, duplicate assignment records, and duplicate operator alerts.
4. Given a photo was detected but routing outcome is unresolved because restart occurred mid-flow, when recovery runs, then the system resolves it deterministically to active/eligible session or quarantine using server-owned session state and records an actionable unresolved/recovered status when it cannot resolve safely.
5. Given quarantined photos remain open after restart, when operator opens live cards/quarantine review, then open quarantine counts, latest reasons, eligible-session assignment behavior, and assigned/resolved records remain consistent with pre-restart state.
6. Given recovery performs work or finds conflicts, when recovery completes, then activity logs record safe operator-readable recovery entries and SSE publishes safe events (`ingestion.recovered`, `photo.routed`, `photo.quarantined`, `quarantine.recovered`, or equivalent) without raw filesystem paths as entity ids.
7. API responses and OpenAPI document any recovery status fields/endpoints/events using existing `{data}` wrapper, `{error:{code,message,action,details}}` errors, `snake_case` JSON, and dot-notation SSE.
8. Existing Story 4.1-4.4 behavior is not regressed: stable JPG detection, active-session routing, quarantine reason categories, manual assignment rules, station summaries, and UI invalidation continue to pass tests.
9. Tests/build pass for Go agent and relevant web type/build checks.

## Tasks / Subtasks

- [x] Add durable persistence for photo and quarantine stores. (AC: 2, 3, 5)
  - [x] Create persistence modules mirroring `sessions.Persistence` style under `apps/agent/internal/photos` and `apps/agent/internal/quarantine`, or a shared small state persistence helper if simpler.
  - [x] Persist to `local-data/state/photos.json` and `local-data/state/quarantine.json` with versioned envelopes, `saved_at`, atomic temp-write + rename, and validation before replace.
  - [x] Preserve exact identity semantics from current stores: `station_id + lower(filepath.Clean(source_path)) + source_size_bytes`; do not include detection timestamps, routed timestamps, quarantine timestamps, session ids, or status in identity.
  - [x] Load persisted records on service startup before building `ingestion.Router`; rebuild `identityTo` maps and ordering from records.
  - [x] Save after every mutation that changes photo/quarantine state: route, quarantine, assignment. Do not save only on clean shutdown.
  - [x] Reject/repair malformed state safely: service should fail fast for corrupt state if data integrity is uncertain, or return an actionable health/recovery error if a partial recovery mode is implemented.
- [x] Extend stores without breaking current API callers. (AC: 2, 3, 5, 8)
  - [x] Keep existing `photos.Store.Route(...)` and `quarantine.Store.Quarantine(...)` duplicate behavior for compatibility with Stories 4.2-4.4.
  - [x] Add constructors/helpers such as `NewStoreFromRecords` / `ReplaceAll` with validation for required ids, source identity uniqueness, allowed statuses, and assignment consistency.
  - [x] If persistence is injected into stores, keep APIs deterministic and testable; avoid filesystem writes inside tests unless explicitly using `t.TempDir()`.
  - [x] Ensure `quarantine.Store.Assign` persists assigned state and remains idempotent for same target session and conflicting for different target session.
- [x] Implement startup recovery/reconciliation service. (AC: 1, 3, 4, 6)
  - [x] Add a small recovery service/package, e.g. `apps/agent/internal/recovery`, invoked during `cmd/selfstudio-agent/main.go` startup after station/session/photo/quarantine state loads.
  - [x] Reconcile persisted routed photos/quarantine items against current filesystem enough to identify missing source files or duplicate candidates; do not move/copy/delete image files in this story.
  - [x] Re-scan station input folders after startup using existing `ingestion.Scanner`/router path or a dedicated recovery scan that respects stable-file rules.
  - [x] Ensure restart does not reset the scanner's duplicate memory in a way that creates duplicates; persisted stores must be the cross-restart duplicate source of truth.
  - [x] For unresolved cases, prefer quarantine with actionable reason/status over unsafe routing. If adding status values, keep them explicit (`recovered`, `unresolved`, or `quarantined`) and documented.
  - [x] Recovery must use server-side session state only. Frontend must not decide whether a recovered photo routes to an active/locked session.
- [x] Wire recovery activity logging and SSE. (AC: 5, 6, 8)
  - [x] Log startup recovery summary with counts: recovered routed photos, recovered/open quarantine items, skipped duplicates, unresolved/conflicts.
  - [x] Publish safe events only with ids such as `photo_id`, `quarantine_id`, `station_id`, `session_id`; never use raw `source_path` as `entity_id`.
  - [x] Extend web SSE invalidation if new event types are introduced so station cards, session detail, quarantine review, and activity log refresh.
- [x] Update API contract and frontend validators only where behavior is exposed. (AC: 5, 7)
  - [x] Update `docs/api/openapi.yaml` for new persisted/recovery fields, statuses, summaries, or recovery endpoint if one is added.
  - [x] Update `apps/web/src/lib/api/client.ts` runtime validators without weakening existing validation for photos/quarantine/session summaries.
  - [x] Preserve existing endpoints and response shapes for `/api/ingestion/scan`, `/api/quarantine`, `/api/quarantine/{id}/eligible-sessions`, `/api/quarantine/{id}/assign`, `/api/sessions`, and station quarantine summary.
- [x] Add focused regression and recovery tests. (AC: 1-9)
  - [x] Go persistence round-trip tests for photos and quarantine, including assigned quarantine state.
  - [x] Go tests that restart simulation (`save -> load -> scan same files`) does not create duplicate routed/quarantine records.
  - [x] Go tests that duplicate identity ignores timestamps and preserves current case-insensitive clean-path behavior.
  - [x] Go tests that persisted assigned quarantine does not return as open station alert after reload.
  - [x] Go recovery tests for no-active-session and late-photo quarantine after reload.
  - [x] API tests for safe recovery/activity/SSE ids if recovery emits events through API/startup seams.
  - [x] Web typecheck/build and validator coverage for any changed JSON shape.

## Dev Notes

### Source Requirements

- Epic 4 objective: monitor station input folders, wait for stable JPGs, prevent duplicate processing, route photos to active sessions, quarantine unassigned/late photos, and provide operator review/assignment. This story closes the Epic 4 reliability gap by making pending ingestion/quarantine state survive restart.
- FR54 explicitly requires recovery of pending photo processing jobs after restart; within Epic 4 scope this story covers pending ingestion/quarantine recovery that precedes Epic 5 processing.
- NFR9 requires recoverable session, photo, processing, quarantine, and upload state after restart. NFR14 requires traceability from source folder to session/quarantine and later output/upload state. NFR16 requires duplicate-safe retries/recovery.
- PRD Journey 2: late photos after session lock must remain visible in quarantine and manually assignable after restart; they must not silently enter old/new sessions.

### Current Code Context To Read Before Editing

- `apps/agent/cmd/selfstudio-agent/main.go`
  - Current state: loads station persistence and session persistence from `cfg.LocalDataDir`, then creates fresh in-memory `photoStore := photos.NewStore()` and `quarantineStore := quarantine.NewStore()` on every process start.
  - Story change: photo/quarantine stores must load from durable state before router/API construction; recovery should run during startup after stores are loaded.
  - Must preserve: auth, activity, event broker, station/session handlers, ingestion scanner/router wiring.
- `apps/agent/internal/sessions/persistence.go`
  - Current state: versioned `local-data/state/sessions.json`, validates sessions, writes via temp file + sync + rename.
  - Reuse this pattern for photo/quarantine persistence. Avoid inventing a different unsafe write method.
- `apps/agent/internal/photos/store.go`
  - Current state: in-memory routed-photo store. `Route` creates `photo_id` from identity; duplicate identity returns existing photo with `Duplicate=true`; `GetBySourceIdentity`, `ListBySession`, `CountBySession`, `ListAll` exist.
  - Story change: add durable load/save and validation. Preserve `StatusRouted = "routed"` and identity calculation exactly.
  - Critical risk: changing identity will break duplicate safety and Story 4.4 assignment conflict handling.
- `apps/agent/internal/quarantine/store.go`
  - Current state: in-memory quarantine store with statuses `quarantined`, `assigned`; reasons `no_active_session`, `late_photo`; assignment fields `assigned_session_id`, `assigned_photo_id`, `assigned_at`; duplicate identity maps station/path/size to one `quarantine_id`.
  - Story change: persist and reload all fields including assigned state. Open counts must include only `status == "quarantined"`.
  - Preserve: `Assign` idempotency for same session, conflict for different session, and `List(status, station_id, limit)` semantics.
- `apps/agent/internal/ingestion/scanner.go`
  - Current state: scans configured station input folders, accepts `.jpg/.jpeg`, ignores directories/zero-byte/unstable files, waits 500ms, uses process-local `seen` map keyed by lowercased path to suppress duplicate scan results.
  - Story change: process-local `seen` is not enough after restart. Recovery must rely on persisted photo/quarantine identity stores to suppress cross-restart duplicates.
  - Preserve: no file mutation, stable-file checks, duplicate suppression within one process.
- `apps/agent/internal/ingestion/router.go`
  - Current state: routes stable detected photos to active sessions via `sessions.ActiveForStation`; otherwise creates quarantine record. Late-photo reasoning uses `LastSessionForStation`, locked session `EndedAt`/`EndsAt`, and sets `related_session_id`.
  - Story change: recovery should use this server-owned routing logic or match it exactly; do not duplicate divergent routing rules in frontend or API handlers.
- `apps/agent/internal/api/ingestion.go`
  - Current state: scan endpoint records/publishes `photo.detected`, `photo.routed`, `photo.quarantined` and returns `photos`, `routed_photos`, `quarantined_photos`, `errors`.
  - Story change: after persistence is added, scan/recovery side effects must not emit misleading duplicate operator alerts for already-known records after restart.
- `apps/agent/internal/api/quarantine.go`
  - Current state: manual assignment endpoints, eligible sessions, safe activity logging, and `quarantine.assigned`/`photo.routed` SSE. Locked sessions are eligible only for related `late_photo` recovery.
  - Story change: assignment behavior and open/assigned state must survive restart unchanged.
- `apps/agent/internal/api/sessions.go`
  - Current state: session detail/live card summaries count photos via `photoStore.CountBySession` and open quarantine via `quarantineStore.CountByStation` / `CountByRelatedSession`.
  - Story change: after reload, these summaries must reflect persisted photo/quarantine state without extra UI changes.
- `apps/web/src/features/quarantine/*` and `apps/web/src/lib/api/client.ts`
  - Current state: quarantine review UI and API validators rely on `QuarantineItem`, `EligibleSession`, assignment response, and query invalidation from Story 4.4.
  - Story change: keep UI compatible; add new recovery status labels only if backend exposes them.
- `docs/api/openapi.yaml`
  - Current state: documents ingestion scan, quarantine list/eligible/assign schemas, station quarantine summary, sessions, and SSE wrapper.
  - Story change: document persistence-visible status fields/events/endpoints if added; keep existing contract stable.

### Architecture Guardrails

- Go service owns all filesystem access, watcher/scanner, ingestion routing, recovery, photo/quarantine state, and credential boundaries.
- Next.js/browser must never read camera folders, local-data files, or decide recovery/routing eligibility.
- Do not implement Epic 5 original-save/LUT processing, processing queue recovery, GCS upload recovery, or local file copying/moving in this story.
- Runtime photo files remain outside `apps/web/public`; do not expose raw filesystem paths as browser-routable assets.
- Use REST/SSE contracts already established: `{data}` response wrapper, actionable error shape, `snake_case` JSON, dot-notation SSE event names, ISO 8601 UTC times.
- Existing in-memory stores are acceptable implementation substrate only if backed by durable JSON persistence for restart recovery in this story. Do not introduce Supabase migrations unless intentionally converting this area to DB-backed persistence across all related stores.
- Prefer safe quarantine/unresolved status over unsafe automatic routing whenever persisted state and filesystem state disagree.

### Recommended Implementation Shape

- Add:
  - `apps/agent/internal/photos/persistence.go` + tests.
  - `apps/agent/internal/quarantine/persistence.go` + tests.
  - `apps/agent/internal/recovery/recovery.go` + tests if reconciliation has more than startup wiring.
- Modify:
  - `apps/agent/cmd/selfstudio-agent/main.go` to load photo/quarantine persistence before router construction and run recovery summary.
  - `apps/agent/internal/photos/store.go` and `apps/agent/internal/quarantine/store.go` for `ReplaceAll`/constructor validation and optional persistence hooks.
  - `apps/agent/internal/api/ingestion.go` only if duplicate/recovered events need suppression or new recovery summaries are exposed.
  - `apps/agent/internal/api/quarantine.go` only if assignment persistence must be triggered from handler/service layer.
  - `docs/api/openapi.yaml` and `apps/web/src/lib/api/client.ts` only for exposed contract changes.
- Persistence file schemas should be boring and explicit:
  - Photos: `{ "version": 1, "saved_at": "...", "photos": [Photo...] }`
  - Quarantine: `{ "version": 1, "saved_at": "...", "items": [Record...] }`
- If store methods save internally, return errors or expose mutation wrappers. Do not silently ignore disk write failures after creating in-memory state; that would make UI lie about recoverability.
- If handler/service layer saves after mutation, ensure all mutation paths are covered: ingestion route, quarantine creation, quarantine assignment, future tests.

### Previous Story Intelligence

- Story 4.1 established stable JPG scanning: case-insensitive `.jpg/.jpeg`, waits for stable file size/modtime, ignores zero-byte/unstable files, handles duplicate paths in process lifetime, and mutates no files. Preserve these semantics.
- Story 4.2 established routed-photo records and duplicate identity. Review learning: duplicate identity must not include per-detection timestamps. This is the most important anti-regression rule for this story.
- Story 4.3 established quarantine records, `no_active_session` and `late_photo` reasons, open quarantine summary counts, SSE/activity side effects, and station-level summary for no-active-session context.
- Story 4.4 established manual assignment lifecycle, `assigned` quarantine status, server-side eligible sessions, conflict-safe duplicate assignment, safe activity/SSE IDs, OpenAPI schemas, and UI invalidation. Persist all of it; do not rebuild assignment from scratch.
- Story 4.4 review follow-up fixed a HIGH issue: locked-session assignment is allowed only for related `late_photo`; `no_active_session` must not be assignable to arbitrary locked sessions. Do not regress this rule in recovery or persisted eligible-session behavior.
- Git history is sparse (`Add from-scratch setup guide`, `Initial Selfstudio camera capture spike`); current workspace files and story artifacts are the authoritative implementation pattern source.

### Testing Requirements

Run at minimum:

- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build` if environment/dependencies allow.

If Windows blocks `go test ./...` execution from temp paths with `Access is denied`, use the known workaround from Story 4.4: `go test -c ./...` then execute generated `*.test.exe` via `cmd.exe /c`.

Required coverage:

- Photo persistence save/load round trip preserves ids, identity maps, order/list behavior, and duplicate detection after reload.
- Quarantine persistence save/load round trip preserves open and assigned records, related session, assignment fields, reason/status, and open-count behavior.
- Simulated restart with same stable files in input folders does not duplicate routed photos or quarantined items.
- Restart after assignment keeps quarantine assigned, keeps photo routed to same session, and does not count assigned item as open alert.
- Recovery logs safe summaries and emits safe SSE events if events are part of implementation.
- Corrupt/malformed state files fail safely with actionable error and do not silently start with empty stores.
- Existing Story 4.1-4.4 tests still pass.

### Regression Risks To Avoid

- Do not reset photo/quarantine state to empty on startup; that is the defect this story fixes.
- Do not make the scanner's `seen` map the cross-restart source of truth; it is process-local only.
- Do not change `photos.Identity` or `quarantine.Identity` semantics.
- Do not route locked-session/no-active photos automatically during recovery unless current router rules would route them; use quarantine for ambiguity.
- Do not emit duplicate `photo.routed`/`photo.quarantined` activity alerts for already-known records on every restart.
- Do not break Story 4.4 assignment idempotency and conflict behavior.
- Do not count `assigned` quarantine records as open live-card alerts.
- Do not move/copy/delete source JPGs; Epic 5 will handle original-first local save.
- Do not introduce frontend filesystem access, Supabase service role exposure, or Google credentials exposure.

## Project Structure Notes

- Expected new files: `apps/agent/internal/photos/persistence.go`, `apps/agent/internal/quarantine/persistence.go`, optional `apps/agent/internal/recovery/recovery.go`, and co-located `*_test.go` files.
- Expected modified files: `apps/agent/cmd/selfstudio-agent/main.go`, `apps/agent/internal/photos/store.go`, `apps/agent/internal/quarantine/store.go`, possibly `apps/agent/internal/api/ingestion.go`, `apps/agent/internal/api/quarantine.go`, `apps/web/src/lib/api/client.ts`, `apps/web/src/features/health/health-dashboard.tsx`, `docs/api/openapi.yaml`.
- Follow existing Go package naming: short lowercase packages, snake_case Go filenames, business rules outside thin HTTP handlers when complexity grows.
- Runtime state belongs under `local-data/state`; never commit generated state files.

## References

- `_bmad-output/planning-artifacts/epics.md` — Epic 4 and Story 4.5 acceptance criteria; FR21-FR30 and FR54 mapping.
- `_bmad-output/planning-artifacts/prd.md` — Photo Ingestion and Routing; Readiness/Health/Recovery; NFR9, NFR14, NFR16; Journey 2 late-photo quarantine recovery.
- `_bmad-output/planning-artifacts/architecture.md` — Go service ownership, REST/SSE contracts, idempotency, project boundaries, recovery requirements, local filesystem safety.
- `_bmad-output/implementation-artifacts/4-1-watch-station-input-folders-for-stable-jpgs.md` — stable scanner behavior and duplicate path handling.
- `_bmad-output/implementation-artifacts/4-2-route-valid-jpgs-to-active-sessions.md` — routed photo identity and duplicate safety.
- `_bmad-output/implementation-artifacts/4-3-quarantine-unassigned-and-late-photos.md` — quarantine reason/count/event behavior.
- `_bmad-output/implementation-artifacts/4-4-review-and-assign-quarantined-photos.md` — assignment lifecycle, HIGH review follow-up, file list, and test workaround.
- `apps/agent/cmd/selfstudio-agent/main.go` — startup wiring currently resets photo/quarantine stores.
- `apps/agent/internal/sessions/persistence.go` — persistence pattern to reuse.
- `apps/agent/internal/photos/store.go` — current routed-photo store and identity.
- `apps/agent/internal/quarantine/store.go` — current quarantine/assignment store and identity.
- `apps/agent/internal/ingestion/scanner.go` — stable JPG scan and process-local seen map.
- `apps/agent/internal/ingestion/router.go` — server-owned routing/quarantine decisions.
- `apps/agent/internal/api/quarantine.go` — manual assignment API and eligibility rules.
- `docs/api/openapi.yaml` — API/SSE contract.

### Review Follow-ups (AI)

- [x] [HIGH] Persist photo/quarantine mutations after runtime ingestion and assignment — AC2/AC5/AC9 require saving after every route, quarantine, and assignment mutation, but persistence is only saved during startup recovery in `cmd/selfstudio-agent/main.go`. `api.IngestionHandler.Scan` calls `router.Route(...)` without saving `photos.json`/`quarantine.json`, and `api.QuarantineHandler.Assign` calls `photoStore.Route(...)` / `quarantineStore.Assign(...)` without saving either store. Any photos detected, quarantined, or assigned after startup can be lost on restart, directly regressing durable JSON persistence and recovery.
- [x] [HIGH] Make runtime persistence failure transactional/actionable — The story requires disk write failures not be silently ignored after in-memory mutation. Current API mutation paths have no persistence hook/error handling, so even if persistence is added later, handlers must avoid UI-success-with-undurable-state by rolling back or returning an actionable `{error}` when saving `photos.json` or `quarantine.json` fails.
- [x] [MEDIUM] Suppress duplicate scan activity/SSE for already-known persisted identities — `api.IngestionHandler.Scan` records `photo.detected` and publishes `photo.routed`/`photo.quarantined` even when `router.Route(...)` returns `Duplicate=true`. After restart or repeated scans of known files, this can create duplicate operator alerts/events despite identity de-duplication, violating AC3/AC6 event safety expectations.
- [x] [HIGH] Make multi-store persistence rollback truly transactional — `saveRuntimeState` in `apps/agent/cmd/selfstudio-agent/main.go` saves `photos.json` first and `quarantine.json` second. If the second save fails after the first succeeds, `rollbackRuntimeState` reloads the already-written photo state, so API returns `INGESTION_PERSIST_FAILED`/`QUARANTINE_PERSIST_FAILED` while durable state may still contain the new routed photo without the matching quarantine assignment. This violates the resolved follow-up intent (“Tidak ada perubahan runtime yang dianggap berhasil”), AC2/AC5 durable consistency, and AC9 transactional failure coverage. Evidence: `QuarantineHandler.Assign` creates/routes a photo, assigns quarantine, then calls `persistAssignment`; on partial save failure, rollback can preserve the photo but lose the assignment, causing inconsistent restart recovery and potential duplicate/conflict behavior.

## Dev Agent Record

### Agent Model Used

GPT-5.1 Codex

### Debug Log References

- 2026-05-19: Implemented photo/quarantine JSON persistence with versioned envelopes, atomic temp-write/rename, validation, and `NewStoreFromRecords`/`ReplaceAll` reload paths.
- 2026-05-19: Added startup recovery service that scans stable JPGs, routes through server-owned router logic, skips persisted duplicates, emits safe recovery/SSE summaries, and logs operator-readable recovery activity.
- 2026-05-19: Wired startup load/recovery/save flow in `cmd/selfstudio-agent/main.go`; corrupt photo/quarantine state fails fast instead of silently starting empty.
- 2026-05-19: Updated web SSE invalidation for `ingestion.recovered` and documented recovery SSE payload in OpenAPI.
- 2026-05-19: Validation: `cd apps/agent && go test -c ./...` then executed all generated Windows test binaries via `cmd.exe`; `cd apps/web && npm run typecheck`; `cd apps/web && npm run build`.
- 2026-05-19: Addressed review follow-ups by wiring runtime persistence hooks for ingestion/assignment, adding rollback/actionable API errors on persistence failure, and suppressing duplicate scan activity/SSE for duplicate routed/quarantine identities.
- 2026-05-19: Validation: `cd apps/agent && go test ./...`; `cd apps/web && npm run typecheck`; `cd apps/web && npm run build`.
- 2026-05-19: Addressed remaining HIGH review follow-up by replacing sequential runtime persistence with a transactional save wrapper that snapshots both durable files before writing and restores both if the second store save fails.
- 2026-05-19: Added regression coverage for second-save-fails scenario proving the new routed photo is not durably visible after quarantine persistence fails.
- 2026-05-19: Validation: `cd apps/agent && go test ./...`; `cd apps/web && npm run typecheck`; `cd apps/web && npm run build`.

### Completion Notes List

- Durable photo state now persists to `local-data/state/photos.json`; durable quarantine state persists to `local-data/state/quarantine.json` with validation and restart reload.
- Existing duplicate identity semantics remain unchanged: `station_id + lower(filepath.Clean(source_path)) + source_size_bytes`; timestamp/session/status fields do not participate in identity.
- Reloaded stores rebuild identity maps/order and preserve routed photos, open quarantine, assigned quarantine, assignment metadata, and open-count behavior.
- Startup recovery scans current station input folders with existing stable-file scanner and routes through backend router/session logic; known source identities are skipped as duplicates and do not create duplicate records.
- Recovery emits safe `ingestion.recovered`, `photo.routed`, and `photo.quarantined` events using durable ids, not raw filesystem paths as entity ids.
- Web listens for `ingestion.recovered` to invalidate activity/session/quarantine/station summary views; existing API response shapes remain preserved.
- ✅ Resolved review finding [HIGH]: runtime ingestion and assignment now call injected persistence hooks after each non-duplicate route/quarantine/assign mutation.
- ✅ Resolved review finding [HIGH]: persistence failures now return actionable API errors and reload stores from durable state so the UI does not falsely report undurable success.
- ✅ Resolved review finding [MEDIUM]: duplicate scan route results no longer record duplicate activity entries or publish duplicate SSE events for already-known identities.
- ✅ Resolved review finding [HIGH]: multi-store runtime persistence now snapshots and restores both `photos.json` and `quarantine.json` so a second-save failure does not leave partial durable state visible.

### File List

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/cmd/selfstudio-agent/runtime_state.go`
- `apps/agent/cmd/selfstudio-agent/runtime_state_test.go`
- `apps/agent/internal/api/ingestion.go`
- `apps/agent/internal/api/quarantine.go`
- `apps/agent/internal/api/quarantine_assignment_test.go`
- `apps/agent/internal/photos/store.go`
- `apps/agent/internal/photos/persistence.go`
- `apps/agent/internal/photos/persistence_test.go`
- `apps/agent/internal/quarantine/store.go`
- `apps/agent/internal/quarantine/persistence.go`
- `apps/agent/internal/quarantine/persistence_test.go`
- `apps/agent/internal/recovery/recovery.go`
- `apps/agent/internal/recovery/recovery_test.go`
- `apps/web/src/features/health/health-dashboard.tsx`
- `docs/api/openapi.yaml`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `_bmad-output/implementation-artifacts/4-5-recover-pending-ingestion-and-quarantine-state.md`

## Change Log

- 2026-05-19: Ultimate context engine analysis completed - comprehensive developer guide created.
- 2026-05-19: Implemented durable photo/quarantine recovery, startup reconciliation, safe recovery SSE/activity updates, focused tests, and moved story to review.
- 2026-05-19: Addressed review follow-ups: runtime persistence hooks, actionable rollback on persistence failure, duplicate scan alert/SSE suppression, and validation rerun.
- 2026-05-19: Addressed HIGH transactional persistence follow-up: runtime photo/quarantine saves now rollback durable files as a unit if quarantine save fails after photo save.
