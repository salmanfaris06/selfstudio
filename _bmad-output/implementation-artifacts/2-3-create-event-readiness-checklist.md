# Story 2.3: Create Event Readiness Checklist

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an operator,
I want a single event readiness checklist that summarizes station, storage, connectivity, and operator setup,
so that I can confidently start a photo session only when required preflight items are safe.

## Acceptance Criteria

1. Given station readiness checks exist, when operator opens dashboard, then a readiness checklist shows each required station item with status and actionable text.
2. Given all required station path/LUT checks pass and device checks are explicitly unknown/warning, when checklist is computed, then checklist can show `warning`/`needs operator confirmation` but not a false `ready` for device automation.
3. Given any required station folder/LUT check fails, when checklist is computed, then overall event readiness is `failed` and session start remains unavailable/placeholder-disabled.
4. Given storage/local data output root is missing or unwritable, when checklist is computed, then checklist shows failed storage item with clear action.
5. Given Supabase/GCS connectivity is not implemented yet, when checklist is computed, then cloud connectivity items are shown as explicit `placeholder` or `unknown`, not `ready`.
6. Given operator reruns checklist, when check completes, then Go service records safe activity and publishes an SSE event so the dashboard refreshes.
7. Given API docs are inspected, when docs are opened, then `docs/api/openapi.yaml` documents readiness checklist endpoint(s), schema, status values, and error responses.

## Tasks / Subtasks

- [x] Add event readiness domain/service model (AC: 1-5)
  - [x] Define checklist aggregate model with overall status, label, action, checked_at, and grouped checklist items.
  - [x] Define item categories: `stations`, `storage`, `cloud`, `operator`.
  - [x] Define status values aligned with existing health/readiness language: `ready`, `warning`, `failed`, `unknown`, `placeholder`.
  - [x] Include station readiness summary for exactly three stations.
  - [x] Treat folder/LUT failures as required blockers.
  - [x] Treat device readiness as `unknown`/warning requiring operator confirmation, not automated ready.
  - [x] Add storage/local output root readiness item.
  - [x] Add cloud/Supabase/GCS items as explicit placeholder/unknown until real integrations exist.
  - [x] Add session start availability flag or placeholder-disabled reason.
- [x] Add protected readiness checklist API (AC: 1-7)
  - [x] Add `GET /api/readiness` for current event readiness checklist.
  - [x] Add `POST /api/readiness/check` to recompute checklist.
  - [x] Protect endpoints with local session auth.
  - [x] Apply trusted Origin guard to checklist mutation endpoint.
  - [x] Return standard `{data}` and `{error}` wrappers.
  - [x] Record safe activity on checklist check success/failure.
  - [x] Publish `readiness.checked` SSE event after mutation completes.
- [x] Extend frontend dashboard UI (AC: 1-6)
  - [x] Add API types/helpers for event readiness checklist.
  - [x] Add TanStack Query hook/mutation for checklist.
  - [x] Render checklist above station configuration or near health dashboard summary.
  - [x] Show overall readiness using text labels, not color only.
  - [x] Show grouped checklist items with action text.
  - [x] Add “Run event readiness check” action.
  - [x] Keep start session CTA unavailable/placeholder-disabled when readiness failed or warning/placeholder remains.
  - [x] Invalidate checklist query on `readiness.checked`, `station.updated`, and `station.readiness_checked` SSE events.
- [x] Update OpenAPI documentation (AC: 7)
  - [x] Document checklist endpoint(s), response schemas, status enum, item categories, examples, and relevant errors.
- [x] Add tests and validation (AC: 1-7)
  - [x] Go unit tests for aggregate ready/warning/failed behavior.
  - [x] Go tests for storage missing/unwritable behavior.
  - [x] API tests for auth, untrusted origin, activity logging, and SSE publication.
  - [x] Frontend typecheck/build validation.
  - [x] Manual smoke: login, configure valid station paths/LUT/output folders, run station readiness, run event checklist, verify warning state due to device/cloud placeholders and disabled start-session placeholder.

## Dev Notes

### Scope Boundary

This story creates a consolidated readiness checklist for event/session preflight. Do **not** implement real session start, photo ingestion, folder watchers, live camera probing, Supabase connectivity checks, GCS upload checks, image processing, queue workers, or cloud fulfillment. Those belong to later stories/epics.

### Business Context

Stories 2.1 and 2.2 let the operator configure stations and validate per-station folders/LUT readiness. Story 2.3 turns those individual checks into a single operator-facing preflight surface. The operator should not have to infer event readiness from separate panels.

### Previous Story Intelligence

Story 2.1 completed:

- exactly three logical stations with stable IDs `station_1`, `station_2`, `station_3`
- protected station config API
- station config UI
- activity and SSE events for station updates

Story 2.2 completed:

- station readiness model and validator
- protected station readiness endpoints:
  - `GET /api/stations/{station_id}/readiness`
  - `POST /api/stations/{station_id}/readiness/check`
- station readiness statuses: `ready`, `warning`, `failed`, `unknown`
- station checks: `input_folder`, `output_folder`, `default_lut`, `device`
- device readiness explicitly `unknown`
- output folder readiness derives safe probe path from `outputRoot + output_rule`
- readiness mutation records activity and publishes `station.readiness_checked`
- frontend station readiness panel and hooks
- OpenAPI readiness schemas

Story 2.2 review lessons to carry forward:

- SSE handlers must accept the actual event type they register for.
- Checklist/station readiness queries must be invalidated on related station/readiness SSE events.
- Avoid filesystem probes that create persistent directories.
- Runtime-validate frontend API responses before trusting readiness state.
- Do not show stale readiness for unsaved station form edits.
- OpenAPI must mirror actual error codes.

### Architecture Requirements

From `architecture.md`:

- Go service owns local filesystem checks and session/readiness orchestration.
- Next.js dashboard displays readiness and commands checks via REST.
- API success response uses `{data}` wrapper.
- API error response uses `{error:{code,message,action,details}}`.
- SSE uses dot-notation event names and refreshes dashboard state.
- Dashboard status must use text labels, not color only.
- Critical operator errors must include action text.
- Session start must not be enabled when required readiness is failed.

### Suggested API Contract

Recommended endpoints:

```http
GET /api/readiness
POST /api/readiness/check
```

Recommended response shape:

```json
{
  "data": {
    "readiness": {
      "status": "warning",
      "label": "Event belum sepenuhnya terverifikasi otomatis",
      "action": "Konfirmasi device dan cloud placeholder sebelum event.",
      "checked_at": "2026-05-18T10:30:00Z",
      "session_start_available": false,
      "session_start_action": "Session start belum tersedia sampai stories session control selesai dan required readiness aman.",
      "items": [
        {
          "category": "stations",
          "item_key": "station_1",
          "status": "warning",
          "label": "Station 1 butuh verifikasi device",
          "action": "Pastikan kamera/tether software menulis JPG ke input folder."
        },
        {
          "category": "cloud",
          "item_key": "gcs",
          "status": "placeholder",
          "label": "Google Cloud Storage belum dicek otomatis",
          "action": "Cloud upload akan divalidasi di Epic 6."
        }
      ]
    }
  }
}
```

Recommended error codes:

- `READINESS_UNAVAILABLE`
- `READINESS_CHECK_FAILED`
- `UNAUTHORIZED`
- `UNTRUSTED_ORIGIN`

### Aggregation Guidance

Recommended aggregate semantics:

- Any required station folder/LUT failure => overall `failed`.
- Missing/unwritable local output root => overall `failed`.
- Device unknown only => overall `warning`, not `ready`.
- Cloud/Supabase/GCS placeholders => overall `warning` or `placeholder`, not `ready`.
- Session start remains unavailable/disabled in this story regardless of aggregate state because actual session control is not implemented yet. The reason must be visible.

### Storage Check Guidance

For storage item, reuse the same output root concept as Story 2.2. Do not create persistent folders during checklist. It is acceptable to require root/folders to exist and be writable via temp probe cleanup. If using derived output-rule folders per station, reuse existing station readiness output result rather than duplicating logic.

### Activity Logging Requirements

Recommended action types:

- `readiness.checked`
- `readiness.check_failed`

Activity messages must be operator-safe and not include secrets or full request bodies.

### SSE Requirements

After checklist mutation completes, publish:

- `readiness.checked`
- `entity_type: "readiness"`
- `entity_id: "event"` or equivalent stable identifier

Frontend should invalidate checklist query when receiving:

- `readiness.checked`
- `station.updated`
- `station.readiness_checked`

### Existing Files Likely to Update

Read before editing:

- `apps/agent/internal/stations/readiness.go`
- `apps/agent/internal/api/readiness.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/features/stations/station-settings.tsx`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`

Suggested new files:

```text
apps/agent/internal/readiness/checklist.go
apps/agent/internal/readiness/checklist_test.go
apps/agent/internal/api/event_readiness.go
apps/agent/internal/api/event_readiness_test.go
apps/web/src/features/readiness/event-readiness-checklist.tsx
apps/web/src/features/readiness/use-event-readiness-query.ts
apps/web/src/features/readiness/use-run-event-readiness-check-mutation.ts
```

### Testing Standards Summary

Go tests:

- Use `t.TempDir()` for storage checks.
- Test aggregate failed when a required station folder/LUT check fails.
- Test aggregate warning when only device/cloud placeholders remain.
- Test session start availability is false with clear action.
- Test protected API routes and trusted Origin mutation.
- Test activity and SSE on checklist mutation.

Frontend validation:

- `npm run typecheck`
- `npm run build`

Manual smoke:

- Login.
- Configure station paths/LUT/output folders.
- Run station readiness.
- Run event readiness checklist.
- Verify checklist shows grouped items and session start placeholder disabled.

### References

- [Source: `_bmad-output/planning-artifacts/epics.md` → Epic 2 / Story 2.3]
- [Source: `_bmad-output/planning-artifacts/architecture.md` → Error Handling Patterns]
- [Source: `_bmad-output/planning-artifacts/architecture.md` → API and SSE requirements]
- [Source: `_bmad-output/implementation-artifacts/2-1-configure-three-camera-stations.md` → station setup implementation]
- [Source: `_bmad-output/implementation-artifacts/2-2-validate-station-paths-luts-and-device-readiness.md` → station readiness implementation]


### Review Findings

Applied Story 2.3 code review patches:

- [x] [Review][Patch] Event checklist now exposes every required station check item: input folder, output folder, default LUT, and device per station.
- [x] [Review][Patch] Added root-level storage item `local_output_root` with exists/writable temp-probe cleanup validation.
- [x] [Review][Patch] Builder now returns `ErrUnavailable` for nil station store; API maps it to `500 READINESS_UNAVAILABLE`, matching OpenAPI.
- [x] [Review][Patch] Event readiness mutation returns `READINESS_UNAVAILABLE` when activity store or SSE broker is unavailable.
- [x] [Review][Patch] Checklist validates exactly three stations and fails if count is wrong.
- [x] [Review][Patch] Missing station output check now creates explicit failed storage item instead of disappearing silently.
- [x] [Review][Patch] Aggregate status now preserves `placeholder` when placeholders remain and no failed item exists.
- [x] [Review][Patch] Frontend event readiness validation now enforces required categories/items, three station groups, `session_start_available === false`, valid category enum, and required cloud/operator/storage items.
- [x] [Review][Patch] Event readiness UI uses structured success/error message state instead of text-prefix styling.
- [x] [Review][Patch] `readiness.checked` SSE now invalidates event readiness, stations, and station readiness query prefix.
- [x] [Review][Patch] Removed unused `onAuthExpired` effect dependency from dashboard SSE effect.
- [x] [Review][Patch] Added disabled “Start session belum tersedia” CTA.
- [x] [Review][Patch] Added API test for unavailable readiness error and success/warning `readiness.checked` activity path.

## Dev Agent Record

### Agent Model Used

{{agent_model_name_version}}

### Debug Log References

- `cd apps/agent && go test ./...` -> passed.
- `cd apps/web && npm run typecheck && npm run build` -> passed.
- Review patch validation `cd apps/agent && go test ./...` -> passed.
- Review patch validation `cd apps/web && npm run typecheck && npm run build` -> passed.
- Manual smoke: login, configure three valid stations with temp input/LUT and output folders, run `POST /api/readiness/check` -> aggregate `warning`; station output items `ready`; cloud/operator placeholders present; session start unavailable.
- Review patch manual smoke: `POST /api/readiness/check` returned aggregate `placeholder`, root storage `ready`, per-station input/output/LUT `ready`, per-station device `unknown`, cloud/operator placeholders, and session start unavailable.
### Completion Notes List

- Ultimate context engine analysis completed - comprehensive developer guide created.
- Added event readiness checklist domain/service with grouped station, storage, cloud, and operator items.
- Expanded checklist review patches to include per-station check items, root storage validation, unavailable error paths, placeholder aggregate status, stronger frontend response validation, disabled start-session CTA, and additional API tests.
- Added protected `GET /api/readiness` and trusted-origin `POST /api/readiness/check`.
- Checklist mutation records safe activity and publishes `readiness.checked` SSE.
- Added frontend event readiness checklist UI, query hook, and mutation hook.
- Dashboard invalidates event checklist on `readiness.checked`, `station.updated`, and `station.readiness_checked`.
- Updated OpenAPI with event readiness endpoints, schemas, enums, and examples.


### Change Log

- 2026-05-18: Created Story 2.3 event readiness checklist context and marked ready for development.
- 2026-05-18: Implemented event readiness checklist and marked ready for review.
- 2026-05-18: Applied Story 2.3 code review patches.

### File List

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/event_readiness.go`
- `apps/agent/internal/api/event_readiness_test.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/readiness/checklist.go`
- `apps/agent/internal/readiness/checklist_test.go`
- `apps/web/src/app/globals.css`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/features/readiness/event-readiness-checklist.tsx`
- `apps/web/src/features/readiness/use-event-readiness-query.ts`
- `apps/web/src/features/readiness/use-run-event-readiness-check-mutation.ts`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`
