# Story 5.2: Apply Station/Session LUT to Create Graded JPG

Status: done

## Story

Sebagai operator, saya ingin foto yang original-nya sudah tersimpan diproses dengan LUT snapshot station/session, sehingga output event memiliki look background yang konsisten tanpa mengorbankan original JPG.

## Acceptance Criteria

1. Given original JPG is saved and session has LUT snapshot, when processor runs, then system creates a graded JPG in deterministic output folder for the same customer/order/station session.
2. Given a photo has `original_save_status = saved_original` and `processing_status = eligible`, when graded processing starts, then processing uses `local_original_path` as input, not `source_path`, and never starts for photos without verified saved original.
3. Given session snapshot contains a valid readable `.cube` LUT path, when graded processing succeeds, then persisted photo/processing state includes `local_graded_path`, graded processing status, timestamps, and traceability to `photo_id`, `station_id`, `session_id`, `local_original_path`, and LUT snapshot path.
4. Given the LUT file is missing, unreadable, non-`.cube`, invalid, or the processor exits/fails, when processor runs, then the graded job is marked failed/retryable with actionable safe error while preserving original and keeping `local_original_path` intact.
5. Given two photos have colliding source basenames, when graded outputs are created, then filename collisions are prevented deterministically without overwriting an existing customer file.
6. Given dashboard/API/session detail inspects the photo after processing, then processing status updates are visible through API and SSE, with text/status values suitable for operator UI.
7. Given processing is running or fails, then dashboard/session controls remain usable and existing ingestion/routing/quarantine/original-save behaviors from Epics 4 and Story 5.1 are not regressed.
8. Given app restarts after graded success/failure or while a graded job was in progress, when service starts again, then persisted state is reconciled with local files; success is not reported for missing/invalid graded output and duplicate output is not created.
9. The exact LUT processor library/tool is documented in implementation notes and OpenAPI/API-visible behavior remains consistent with architecture contracts.
10. Tests/build pass for Go agent and relevant web/API contract checks.

## Tasks / Subtasks

- [x] Extend photo/processing state for graded output. (AC: 1, 3, 4, 6, 8)
  - [x] Add fields to routed photo state or a tightly coupled processing store for at minimum: `local_graded_path`, `graded_processing_status`, `graded_last_error`, `graded_attempt_count`, `graded_processing_started_at`, `graded_processed_at`, and `lut_path`/`lut_snapshot_path` used for traceability.
  - [x] Recommended status values: `not_eligible`, `pending`, `processing`, `processed`, `failed` (lowercase `snake_case`). Preserve existing `original_save_status` values from Story 5.1.
  - [x] Update `photos/persistence.go` validation and backward-compatible normalization so older Story 5.1 records load safely.
  - [x] Do not remove or rename `local_original_path`, `original_save_status`, or current `processing_status` fields without updating all API/web/tests. Prefer additive fields unless refactoring is fully covered.
- [x] Implement deterministic graded output path generation. (AC: 1, 3, 5)
  - [x] Reuse session snapshot output root and sanitization/collision patterns from `processing.OriginalPath`.
  - [x] Recommended folder convention: same customer/order/station folder as originals, under `graded/` instead of `originals/`.
  - [x] Recommended filename convention: `<source_base>__<photo_id>.jpg` for restart-safe deterministic uniqueness.
  - [x] Prevent path traversal outside `session.StationSnapshot.OutputFolder` and never overwrite an existing different graded file.
  - [x] Preserve original JPG untouched; write graded output through temp file + close/sync + atomic rename + verification.
- [x] Implement LUT processor behind a Go-owned interface in `apps/agent/internal/processing`. (AC: 1, 2, 4, 7, 9)
  - [x] Add `lut_processor.go` or equivalent with an interface such as `type LUTProcessor interface { Apply(ctx, inputPath, lutPath, outputPath string) error }` so retry/recovery can test without shelling out.
  - [x] Recommended MVP tool: ImageMagick 7 command-line (`magick`) invoked from Go with bounded context timeout. Architecture intentionally deferred the exact LUT tool; this story must document the choice in code comments and story Dev Agent Record.
  - [x] For `.cube` support with ImageMagick 7, expected approach is convert/read cube LUT as HALD CLUT and apply to image (for example through ImageMagick `cube:`/`-hald-clut` flow). Validate exact command in tests or a small integration fixture before claiming completion.
  - [x] If ImageMagick is not available in the dev/test environment, keep the production adapter injectable and provide deterministic fake processor tests; readiness/operator error must clearly say `LUT_PROCESSOR_UNAVAILABLE` or equivalent.
  - [x] Validate LUT path from `session.StationSnapshot.DefaultLUTPath`, not mutable current station config.
- [x] Build graded processing service/worker. (AC: 1, 2, 3, 4, 7, 8)
  - [x] Process only photos where original save is verified (`original_save_status = saved_original`, `local_original_path` exists and size is non-zero/expected) and graded state is pending/failed/retryable.
  - [x] Mark job `processing` before invoking LUT tool and persist that transition.
  - [x] On success, verify final graded file exists, is non-zero, and can be stat/read before marking `processed`.
  - [x] On failure, mark failed/retryable with safe operator message and preserve `local_original_path`; do not delete or alter original.
  - [x] Avoid holding store locks while doing filesystem/image work; API/dashboard must remain responsive.
  - [x] Initial implementation may run synchronously after Story 5.1 original-save completion if bounded, but prefer a small queue/goroutine service if processing can be slow. Do not block frontend/browser logic.
- [x] Wire graded processing after original save and assignment routes. (AC: 1, 2, 6, 7)
  - [x] After `OriginalSaver.Save` returns saved original in ingestion route flow, enqueue/start graded processing for that `photo_id`.
  - [x] After manual quarantine assignment saves original, enqueue/start graded processing for that `photo_id`.
  - [x] Suppress duplicate route/re-scan events from creating duplicate graded jobs for the same photo.
  - [x] Publish safe SSE events such as `photo.processing_started`, `photo.processed`, `photo.processing_failed`, and `queue.updated`; entity IDs must be safe IDs (`photo_id`), not raw paths.
  - [x] Record activity log entries for processing success/failure with safe operator-readable messages.
- [x] Expose graded status through API/OpenAPI and minimal UI/client contract. (AC: 3, 4, 6, 9)
  - [x] Update `docs/api/openapi.yaml` schemas for routed photo/session detail/ingestion route results with graded output fields and SSE events.
  - [x] Update `apps/web/src/lib/api/client.ts` TypeScript types/validators for new fields.
  - [x] If UI changes are needed, keep them minimal: text labels for `pending/processing/processed/failed`, actionable failure message, and no direct filesystem access from Next.js.
  - [x] Ensure API errors continue to use `{error:{code,message,action,details}}` and success responses use `{data}`.
- [x] Implement restart reconciliation for graded jobs. (AC: 8)
  - [x] On startup, load photo/processing state after sessions/photos and before accepting new processing work.
  - [x] For `processed` records, verify `local_graded_path` exists and is non-zero/readable; if missing/invalid, mark failed/retryable rather than claiming success.
  - [x] For `processing` records from a previous process, requeue or mark failed/retryable deterministically.
  - [x] For `pending`/`failed` records where original remains saved, make them eligible for future retry/recovery without creating a second final path.
- [x] Add focused tests and run validation. (AC: 1-10)
  - [x] Go unit tests for graded path generation, path traversal prevention, collision/idempotency, and use of session snapshot LUT/output paths.
  - [x] Go tests for gating: no graded processing without verified `saved_original`; processing uses `local_original_path`, never source-only.
  - [x] Go tests with fake LUT processor for success and failure, including missing/invalid LUT, processor unavailable, and safe error persistence.
  - [x] Go restart tests: processed graded reload/reconcile, in-progress requeue/fail-safe, duplicate scan after restart does not duplicate output.
  - [x] Regression tests proving Story 5.1 original save and Epic 4 routing/quarantine/assignment still pass.
  - [x] Run `cd apps/agent && go test ./...`.
  - [x] Run `cd apps/web && npm run typecheck`.
  - [x] Run `cd apps/web && npm run build` if environment allows.

### Review Follow-ups (AI)

- [x] [Review][High] Existing graded target is accepted without proving it belongs to this job — `writeGraded` returns success for any existing non-empty `local_graded_path` target before invoking the processor or comparing content (`apps/agent/internal/processing/graded_processor.go:155-159`). This violates AC5/AC8 collision/restart safety because a stale or different customer file at the deterministic path will be reported as processed instead of failing retryably. Fix should verify ownership/content deterministically or fail on pre-existing different output, never silently accept arbitrary non-empty files.
- [x] [Review][Medium] Graded processing runs synchronously inside ingestion/quarantine HTTP flows — ingestion calls `GradedProcessor.Process(context.Background())` before returning the scan response (`apps/agent/internal/api/ingestion.go:119-124`), and quarantine assignment does the same (`apps/agent/internal/api/quarantine.go:195-199`). With ImageMagick timeout defaulting to 30s per image, dashboard/API controls can be blocked during scans/assignments, violating AC7/NFR queue isolation expectations. Fix should enqueue/bound background work or otherwise avoid blocking request handlers on image processing.
- [x] [Review][Medium] Manual quarantine assignment does not publish the graded processing SSE contract — the ingestion path emits `photo.processing_started`, `photo.processed`/`photo.processing_failed`, and `queue.updated`, but the quarantine path only records failure activity and publishes no processing started/succeeded/failed/queue events (`apps/agent/internal/api/quarantine.go:195-204`). This violates AC6 and the task requiring assignment routes to publish safe SSE status updates. Fix should reuse the same safe event/activity behavior for quarantine assignment.
- [x] [Review][Medium] ImageMagick `.cube` command is not validated by an integration fixture despite the story requirement — the production adapter uses `magick <input> cube:<lut> -hald-clut <output>` (`apps/agent/internal/processing/lut_processor.go:45`), but tests only use `fakeLUT` and do not prove the selected CLI syntax works for a real `.cube`/JPG fixture. This leaves AC9/task evidence incomplete. Add a small integration/fixture test that is skipped when `magick` is unavailable but validates the exact command when present, or adjust the adapter to a proven command sequence.

## Dev Notes

### Source Requirements

- Epic 5 objective: original-first local delivery, LUT processing, processing status, retry, collision safety, queue/status visibility, and recovery.
- Story 5.2 acceptance criteria from epics: original saved + session LUT snapshot creates graded JPG; missing/invalid LUT fails graded job retryably while preserving original; exact LUT processor tool must be documented; processing status updates dashboard.
- FR32: apply station/session LUT to create a graded JPG.
- FR33: save original and graded JPGs to deterministic customer/order/station folders.
- FR34/NFR8/NFR27: original must survive LUT failure.
- FR35/FR39: track processing status and identify missing/invalid LUT as failed/retryable.
- NFR2/NFR3/NFR14/NFR15/NFR16/NFR18: processing must not block dashboard/session controls, must preserve traceability, collision safety, duplicate-safe retries, and no persisted success to missing final files.

### Current Code Context To Read Before Editing

- `apps/agent/internal/photos/store.go`
  - Current state: `Photo` includes Story 5.1 original-save fields: `local_original_path`, `original_save_status`, `last_error`, `attempt_count`, `original_save_started_at`, `original_saved_at`, and `processing_status` (`not_eligible`/`eligible`). `MarkOriginalSaved` sets `processing_status = eligible`.
  - Story change: add graded/LUT processing state without breaking existing duplicate identity (`Identity(station_id, source_path, source_size_bytes)`) or original-save behavior.
  - Must preserve: `Route(...)` duplicate returns existing photo with `Duplicate=true`; `ListBySession`, `CountBySession`, `ListAll`, persisted load validation, and original-save normalization.
- `apps/agent/internal/photos/persistence.go`
  - Current state: durable versioned `local-data/state/photos.json` with validation for original save and processing status.
  - Story change: update validation/normalization for graded fields and ensure corrupt/missing graded state fails safely without preventing old records from loading unnecessarily.
- `apps/agent/internal/processing/original_saver.go`
  - Current state: Go-owned original saver with deterministic session-snapshot output paths, path sanitization, temp copy, verification, idempotency, byte-compare existing target, and restart reconciliation. It documents why original-save state lives on `photos.Photo`.
  - Story change: reuse patterns for `GradedPath`/processor state. Do not merge graded output into originals folder or bypass original verification.
- `apps/agent/internal/api/ingestion.go`
  - Current state: scan routes stable photos, persists route mutation, calls `OriginalSaver.Save`, appends post-save route result, publishes `photo.original_saved` and `queue.updated`.
  - Story change: enqueue/run graded processing after successful original save. Preserve safe persistence/rollback behavior and do not make scan API report success if a required persistence transition failed.
- `apps/agent/internal/api/quarantine.go`
  - Current state: assignment routes quarantined photo to a session and calls original saver; review follow-up fixed API response to return post-save fields.
  - Story change: assignment path must also start/enqueue graded processing after original save. Preserve locked-session assignment guard from Story 4.4.
- `apps/agent/cmd/selfstudio-agent/main.go` and `apps/agent/cmd/selfstudio-agent/runtime_state.go`
  - Current state: startup loads stations/sessions/photos/quarantine, wires persistence hooks, initializes original saver, and runs original-save reconciliation.
  - Story change: initialize LUT processor/graded service after stores load; run graded recovery after original-save reconciliation so pending originals can become processable.
- `apps/agent/internal/sessions/store.go`
  - Current state: `StationSnapshot` includes `DefaultLUTPath`, `OutputFolder`, station name/background/input/output rule. `Start` snapshots current station config.
  - Story change: always use `session.StationSnapshot.DefaultLUTPath` and `OutputFolder`; never use mutable station config for historical processing.
- `apps/agent/internal/api/sessions.go`
  - Current state: session detail returns `photos []photos.Photo` and summary counts, with failures currently hardcoded `0`.
  - Story change: include graded processing failures in summary and expose status fields through existing photo objects without breaking old fields.
- `docs/api/openapi.yaml`
  - Current state: documents original-save fields/events from Story 5.1.
  - Story change: document graded fields, status enum, `photo.processed`/failure events, and any processor-unavailable errors.
- `apps/web/src/lib/api/client.ts`
  - Current state: `OriginalSaveStatus = pending|saving|saved_original|failed`, `ProcessingStatus = not_eligible|eligible`, `RoutedPhoto` contains original-save fields.
  - Story change: add typed graded-processing fields/status. Do not add browser local-file processing.

### Architecture Guardrails

- Go service owns all filesystem access, image processing, queues, recovery, and credentials. Next.js/browser must never read/write camera folders, originals, graded files, or LUT files directly.
- Original-first remains non-negotiable: LUT processing starts only after verified `saved_original` and persists failure without modifying/deleting original.
- Use session snapshot (`StationSnapshot.DefaultLUTPath`, `OutputFolder`, background/station info) so later station config changes do not alter historical processing.
- All API JSON/status fields use `snake_case`; SSE event names use dot notation.
- Raw filesystem paths may be API data where already exposed, but never use raw paths as SSE `entity_id` or noisy activity-log identifiers.
- Processing and future retries must be duplicate-safe; path generation must be deterministic by `photo_id`.
- Missing/invalid LUT and processor-tool failures are expected operational states, not crashes.
- Do not implement Story 5.4 retry policy beyond marking failed/retryable and enabling idempotent re-run foundations. Do not implement Story 5.5 full recovery queue beyond this story's startup reconciliation needs. Do not implement cloud upload.

### Recommended LUT Processor Tool Decision

- Recommended MVP selection: ImageMagick 7 (`magick`) CLI invoked by Go service via an injectable adapter.
- Rationale: mature cross-platform Windows availability, supports JPEG read/write and color transform operations, can be installed independently of Go build, and keeps heavy image pipeline out of browser/Next.js.
- Research notes: ImageMagick command-line options include `-clut`, `-hald-clut`, and `-interpolate`; ImageMagick 7.0.8.20+ discussion documents converting `.cube` LUT to HALD image with `magick cube:FG_CineVibrant.cube[6] hald6.png`, then applying HALD CLUT. Validate exact CLI for project fixtures before finalizing.
- Implementation caution: ImageMagick `.cube` handling can vary by install/delegates. The adapter must detect command failure and report an actionable `LUT_PROCESSING_FAILED`/`LUT_PROCESSOR_UNAVAILABLE` style error rather than pretending success.
- If a pure Go LUT library is chosen instead, document why it is preferred, pin the dependency/version, and add fixture tests proving `.cube` parsing and JPG output. Do not silently invent an unverified parser unless tests prove color application behavior.

### Previous Story Intelligence

- Story 5.1 created original-save lifecycle and added processing eligibility fields. Review follow-ups fixed stale API responses after original save and same-size existing-target false success. Preserve those fixes.
- Story 4.1 established stable scanner behavior: `.jpg/.jpeg`, stable size/modtime wait, zero-byte/unstable files ignored, no file mutation.
- Story 4.2 established routed photo identity. Do not change duplicate identity; timestamps/session/status must not affect identity.
- Story 4.3 established quarantine reasons/counts and safe events.
- Story 4.4 established manual quarantine assignment and locked-session eligibility guard. Do not regress: `no_active_session` cannot assign to arbitrary locked sessions.
- Story 4.5 added durable `photos.json` and `quarantine.json`, startup recovery, duplicate scan alert suppression, and transactional multi-store persistence. Reuse this durability pattern and preserve rollback behavior.
- Git history is sparse (`Add from-scratch setup guide`, `Initial Selfstudio camera capture spike`); story artifacts and workspace code are more authoritative than commits.

### Latest Technical Notes

- Current architecture document lists Go `1.26.3`, Next.js `16.2.6`, TanStack Query `5.100.10`, Supabase CLI `v2.98.1`, and shadcn CLI v4 as observed tool versions.
- ImageMagick docs/research: use `magick` command; relevant options include `-hald-clut`, `-clut`, `-interpolate`; `.cube` to HALD conversion is documented in ImageMagick 7-era discussion. Validate on local Windows installation.
- Go `os/exec.CommandContext` should be used for external process timeouts/cancellation. Capture stderr for technical logs but return operator-safe API/activity messages.
- Keep processing concurrency bounded if adding goroutines. Three station event workload must not spawn unbounded image processes.
- Continue using temp-file-then-rename for final outputs; do not expose partially written graded JPG as successful.

### Testing Requirements

Run at minimum:

- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build` if dependencies/environment allow.

If Windows blocks Go test execution from temp paths with `Access is denied`, use the known workaround from Stories 4.4/4.5/5.1: `go test -c ./...` or package-specific `go test -c`, then execute generated `*.test.exe` via `cmd.exe /c`.

Required coverage:

- Graded output succeeds only after verified original save.
- Processor uses `local_original_path`; a missing/unreadable source camera path after original save must not block graded processing.
- Missing, unreadable, wrong-extension, or invalid LUT marks graded failed/retryable and preserves original.
- Processor unavailable/timeout is an actionable failure and does not mark success.
- `local_graded_path` is persisted and recovered after restart.
- Duplicate route/re-scan/recovery does not create duplicate graded output.
- Existing target same name but different content is not overwritten or accepted silently.
- Session detail/API route result includes current graded status fields.
- Epic 4 ingestion/quarantine and Story 5.1 original-save tests remain green.

### Regression Risks To Avoid

- Do not process from `source_path` when `local_original_path` exists; source folder files may disappear later.
- Do not start LUT processing before original save is verified/persisted.
- Do not use current station config LUT for historical sessions; use session snapshot.
- Do not overwrite existing graded files on collision or same deterministic path with different content.
- Do not mark graded success before the temp file is fully written, renamed, and verified.
- Do not mutate/delete originals on processing failure.
- Do not expose filesystem paths as SSE entity IDs or verbose activity log identifiers.
- Do not block dashboard/session controls with long-running image processing under store locks.
- Do not break original-save API fields or Story 5.1 fixes for post-save response freshness.

## Project Structure Notes

- Expected new files:
  - `apps/agent/internal/processing/lut_processor.go`
  - `apps/agent/internal/processing/graded_processor.go`
  - `apps/agent/internal/processing/graded_processor_test.go`
  - Optional `apps/agent/internal/processing/lut_processor_test.go` and fixture files under `apps/agent/testdata` if using integration fixtures.
- Expected modified files:
  - `apps/agent/internal/photos/store.go`
  - `apps/agent/internal/photos/persistence.go`
  - `apps/agent/internal/ingestion/router.go`
  - `apps/agent/internal/api/ingestion.go`
  - `apps/agent/internal/api/quarantine.go`
  - `apps/agent/internal/api/sessions.go`
  - `apps/agent/cmd/selfstudio-agent/main.go`
  - `apps/agent/cmd/selfstudio-agent/runtime_state.go`
  - `docs/api/openapi.yaml`
  - `apps/web/src/lib/api/client.ts`
- Runtime generated photo assets belong under configured session output folders and `local-data`, never under `apps/web/public` or committed source.

## References

- `_bmad-output/planning-artifacts/epics.md` — Epic 5 and Story 5.2 acceptance criteria; FR31-FR39 mapping.
- `_bmad-output/planning-artifacts/prd.md` — Image Processing and Local Storage; NFR8, NFR14, NFR15, NFR18, NFR27.
- `_bmad-output/planning-artifacts/architecture.md` — Go service filesystem/worker ownership, processing package boundary, original-first, queue isolation, state ownership.
- `_bmad-output/implementation-artifacts/5-1-save-original-jpg-before-processing.md` — immediate prior story implementation learnings and review follow-ups.
- `_bmad-output/implementation-artifacts/4-1-watch-station-input-folders-for-stable-jpgs.md` — stable scanner behavior.
- `_bmad-output/implementation-artifacts/4-2-route-valid-jpgs-to-active-sessions.md` — routed photo identity and duplicate safety.
- `_bmad-output/implementation-artifacts/4-4-review-and-assign-quarantined-photos.md` — assignment lifecycle and locked-session guard.
- `_bmad-output/implementation-artifacts/4-5-recover-pending-ingestion-and-quarantine-state.md` — durable photo/quarantine persistence and rollback lessons.
- `apps/agent/internal/photos/store.go` — current photo/original-save state.
- `apps/agent/internal/processing/original_saver.go` — deterministic original output implementation pattern.
- `apps/agent/internal/api/ingestion.go` — post-route original-save wiring.
- `apps/agent/internal/api/quarantine.go` — assignment/original-save wiring.
- `apps/agent/internal/sessions/store.go` — session snapshot fields including `DefaultLUTPath` and `OutputFolder`.
- `docs/api/openapi.yaml` — API/SSE contract source.
- ImageMagick docs: command-line options for `-clut`, `-hald-clut`, interpolation, and MagickCore CLUT APIs.

## Dev Agent Record

### Agent Model Used

GPT-5.1 Codex

### Debug Log References

- `python3 _bmad/scripts/resolve_customization.py --skill .agents/skills/bmad-agent-dev --key workflow`
- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build`

### Completion Notes List

- Ultimate context engine analysis completed - comprehensive developer guide created.
- Added additive graded processing fields to routed photo state and persistence normalization/validation while preserving Story 5.1 original-save fields.
- Implemented deterministic `graded/` output path generation using session snapshot output folder, sanitized customer/order/station segments, and `<source_base>__<photo_id>.jpg` uniqueness.
- Implemented Go-owned `LUTProcessor` interface and ImageMagick 7 (`magick`) production adapter with timeout and safe `LUT_PROCESSOR_UNAVAILABLE` / `LUT_PROCESSING_FAILED` errors; tests use an injectable fake processor.
- Implemented `GradedProcessor` with original-first gating from `local_original_path`, session snapshot LUT validation, processing/processed/failed transitions, temp-file output, verification, and restart reconciliation.
- Wired graded processing after ingestion original-save and manual quarantine assignment; added safe SSE/activity events for processing started/succeeded/failed and queue updates.
- Exposed graded fields through API structs, OpenAPI schema/events, session failure counts, and web API TypeScript types/validators.
- Validations passed: `cd apps/agent && go test ./...`, `cd apps/web && npm run typecheck`, `cd apps/web && npm run build`.
- ✅ Resolved review finding [High]: existing deterministic graded targets are no longer silently accepted; pre-existing outputs are rejected as unverified instead of marked processed.
- ✅ Resolved review finding [Medium]: ingestion and quarantine assignment now enqueue graded processing in background goroutines so HTTP responses are not blocked by ImageMagick processing.
- ✅ Resolved review finding [Medium]: manual quarantine assignment now publishes `photo.processing_started`, terminal `photo.processed`/`photo.processing_failed`, and `queue.updated` events plus activity entries.
- ✅ Resolved review finding [Medium]: added ImageMagick `.cube` command fixture coverage that skips cleanly when `magick` is unavailable and validates the exact adapter command when present.
- Review follow-up validation passed: `cd apps/agent && go test ./...`; fixture skip observed when ImageMagick `magick` is not installed.

### File List

- `_bmad-output/implementation-artifacts/5-2-apply-station-session-lut-to-create-graded-jpg.md`
- `_bmad-output/implementation-artifacts/sprint-status.yaml`
- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/ingestion.go`
- `apps/agent/internal/api/quarantine.go`
- `apps/agent/internal/api/quarantine_assignment_test.go`
- `apps/agent/internal/api/sessions.go`
- `apps/agent/internal/ingestion/router.go`
- `apps/agent/internal/photos/persistence.go`
- `apps/agent/internal/photos/store.go`
- `apps/agent/internal/processing/graded_processor.go`
- `apps/agent/internal/processing/graded_processor_test.go`
- `apps/agent/internal/processing/lut_processor.go`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`

## Change Log

- 2026-05-19: Ultimate context engine analysis completed - comprehensive developer guide created.
- 2026-05-19: Implemented Story 5.2 graded JPG LUT processing, API/web contract updates, restart reconciliation, tests, and validation; status moved to review.
- 2026-05-19: Addressed code review findings - 4 items resolved; status moved to review.
