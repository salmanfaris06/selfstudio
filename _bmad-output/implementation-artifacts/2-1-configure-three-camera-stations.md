# Story 2.1: Configure Three Camera Stations

Status: done

<!-- Note: Validation is optional. Run validate-create-story for quality check before dev-story. -->

## Story

As an admin,
I want to create and edit three logical camera stations,
so that each physical photo source has a named operational slot.

## Acceptance Criteria

1. Given authenticated admin opens station settings, when station configuration loads, then dashboard shows exactly three logical camera stations.
2. Given admin edits station name, physical camera/device identifier, input folder, background name, default LUT, and output rule, when admin saves, then Go service persists the station configuration and returns the updated values with `{data}` wrapper.
3. Given admin submits a station config with missing required fields, when Go service validates the request, then it rejects the request with `{error:{code,message,action,details}}` and an operator-actionable message.
4. Given admin submits duplicate input folders across stations, when Go service validates the request, then duplicate input folders are blocked with a clear actionable error.
5. Given station configuration changes, when the save succeeds or fails, then activity log records a safe `station.config_updated` success or failure entry without secrets.
6. Given station configuration changes, when save succeeds, then dashboard refreshes via TanStack Query and can receive an SSE `station.updated` event.
7. Given frontend and OpenAPI are inspected, when contracts are reviewed, then `docs/api/openapi.yaml` documents station endpoints, request/response schemas, and relevant error responses.

## Tasks / Subtasks

- [x] Add station domain model and in-memory/config store foundation (AC: 1, 2, 3, 4)
  - [x] Create Go package under `apps/agent/internal/stations` or `apps/agent/internal/service` for station model and validation.
  - [x] Represent exactly three logical stations with stable IDs such as `station_1`, `station_2`, `station_3`.
  - [x] Define station config fields: `station_id`, `name`, `device_identifier`, `input_folder`, `background_name`, `default_lut_path`, `output_rule`.
  - [x] Add validation for required fields and duplicate normalized input folders.
  - [x] Keep persistence intentionally scoped for this story: in-memory is acceptable only if clearly documented as temporary; prefer a small local JSON config file under configured local data dir if non-invasive.
- [x] Add protected Go station API endpoints (AC: 1, 2, 3, 4, 5, 6)
  - [x] Add `GET /api/stations` returning all three station configs.
  - [x] Add `PUT /api/stations/{station_id}` or equivalent route for editing a station.
  - [x] Protect endpoints with local session auth.
  - [x] Apply state-changing route guard pattern: auth + trusted Origin + consistent API errors.
  - [x] Record safe activity log entries for station config update success/failure.
  - [x] Publish `station.updated` SSE event after successful save.
- [x] Add frontend station settings UI (AC: 1, 2, 3, 4, 6)
  - [x] Add API types/helpers in `apps/web/src/lib/api/client.ts` or feature-local module.
  - [x] Add TanStack Query hook under `apps/web/src/features/stations`.
  - [x] Render three station settings cards or form sections under authenticated dashboard.
  - [x] Allow editing required fields and saving one station at a time.
  - [x] Show loading/error/success states with text labels and actionable messages.
  - [x] Invalidate stations query after save and on `station.updated` SSE event.
- [x] Update dashboard composition (AC: 1, 6)
  - [x] Integrate station settings into existing authenticated dashboard shell without breaking health and activity log.
  - [x] Preserve local PIN gate behavior and session re-check behavior from Epic 1.
  - [x] Do not implement readiness checks, camera detection, watcher validation, backup/restore, or event readiness checklist in this story.
- [x] Update OpenAPI documentation (AC: 7)
  - [x] Document `GET /api/stations`.
  - [x] Document station update endpoint.
  - [x] Document `Station`, `StationListResponse`, `UpdateStationRequest`, validation errors, unauthorized, forbidden/untrusted origin, and activity-unavailable where applicable.
- [x] Add tests and validation (AC: 1-7)
  - [x] Add Go tests for default three stations, required-field validation, duplicate input folder blocking, protected route auth, untrusted Origin rejection, activity logging, and `station.updated` publication.
  - [x] Run `cd apps/agent && go test ./...`.
  - [x] Run `cd apps/web && npm run typecheck && npm run build`.
  - [x] Manual smoke: login, view three station configs, save valid edit, see activity entry, confirm duplicate folder is rejected.

### Review Findings

- [x] [Review][Patch] Station update should reject non-JSON `Content-Type` to match OpenAPI JSON contract [apps/agent/internal/api/stations.go:63]
- [x] [Review][Patch] Duplicate input-folder normalization misses slash/backslash variants and equivalent path forms [apps/agent/internal/stations/store.go:137]
- [x] [Review][Patch] Output rule accepts unsafe/ambiguous values, including missing `{station_id}` and traversal segments [apps/agent/internal/stations/store.go:81]
- [x] [Review][Patch] Station fields accept excessively long strings without bounds [apps/agent/internal/stations/store.go:81]
- [x] [Review][Patch] Station update nil request body path can panic in internal tests/handlers [apps/agent/internal/api/stations.go:63]
- [x] [Review][Patch] Generic SSE error still triggers auth-expired handling and can log out operators on transient reconnects [apps/web/src/features/health/health-dashboard.tsx:58]
- [x] [Review][Patch] Station form refetch can overwrite unsaved edits while operator is typing [apps/web/src/features/stations/station-settings.tsx:54]
- [x] [Review][Patch] Station mutation message styling can show success text as error after a previous failed mutation [apps/web/src/features/stations/station-settings.tsx:103]
- [x] [Review][Patch] API client should reject payloads containing both `data` and `error` fields [apps/web/src/lib/api/client.ts:176]
- [x] [Review][Patch] Frontend station update response is not runtime-validated before treating save as successful [apps/web/src/lib/api/client.ts:70]
- [x] [Review][Patch] `ApiError` drops `details`, preventing field-level station validation display [apps/web/src/lib/api/client.ts:38]
- [x] [Review][Patch] Activity records can attach unsanitized station IDs from failed station update requests [apps/agent/internal/api/stations.go:110]
- [x] [Review][Patch] OpenAPI omits `403 UNTRUSTED_ORIGIN` for logout and config placeholder actions [docs/api/openapi.yaml:70]
- [x] [Review][Patch] OpenAPI does not explicitly document `DUPLICATE_INPUT_FOLDER` for station updates [docs/api/openapi.yaml:1913]
- [x] [Review][Patch] Station update store-unavailable failure path is not activity-logged [apps/agent/internal/api/stations.go:42]
- [x] [Review][Patch] Missing route/decoder tests for unauthenticated station update, unknown station, malformed/multiple JSON, and path normalization variants [apps/agent/internal/api/stations_test.go:1]

## Dev Notes

### Scope Boundary

This story is station configuration only. Do **not** implement station readiness checks, device probing, LUT validation, folder watcher, test capture validation, backup/restore, event readiness checklist, session start, photo ingestion, processing, quarantine, or cloud upload.

### Business Context

Epic 2 starts the operational setup layer. Story 2.1 gives admin a place to configure exactly three logical camera stations before any readiness/watch/session behavior exists. The goal is not to prove a station is usable yet; the goal is to create and edit the canonical station configuration safely.

### Previous Epic Intelligence

Epic 1 established these foundations:

- `apps/agent` Go service with API response helpers and route mux.
- Local PIN auth with `selfstudio_session` cookie.
- `RequireAuth` middleware for protected endpoints.
- `RequireTrustedOrigin` guard for state-changing cookie-authenticated endpoints.
- Credentialed CORS for local dashboard origins.
- Protected `/events` SSE stream with event envelope and dot-notation event validation.
- TanStack Query provider and dashboard shell in `apps/web`.
- Health dashboard and activity log UI under authenticated local PIN gate.
- Bounded in-memory activity store with safe action messages and optional session references.

Important review lessons from Epic 1:

- Route-level tests are required for protected endpoints; handler-only tests miss middleware behavior.
- Public endpoints should not create important audit/activity side effects.
- State-changing cookie-auth endpoints must use trusted Origin/CSRF guard.
- Do not trust `X-Forwarded-For` for local rate limiting/security decisions.
- Frontend must handle malformed API responses and invalid timestamps safely.
- OpenAPI must stay aligned with implemented responses.

### Architecture Requirements

From `architecture.md`:

- Browser talks only to Go service API.
- Go service owns all mutations and authoritative operational state.
- Next.js never reads camera folders directly.
- Service credentials stay server-side; browser must never receive Supabase service role or Google Cloud credentials.
- REST endpoints live under `/api`.
- SSE stream lives under `/events`.
- OpenAPI contract is maintained at `docs/api/openapi.yaml`.
- API success responses use `{ "data": ... }`.
- API errors use `{ "error": { "code", "message", "action", "details" } }`.
- JSON fields use `snake_case`.
- SSE event names use dot notation such as `station.updated`.
- Go packages use short lowercase names.
- Go files use snake_case.
- React feature components live under `apps/web/src/features/{feature}`.
- API clients live under `apps/web/src/lib/api`.
- Server state in frontend uses TanStack Query.

### Suggested API Contract

Recommended endpoints:

```http
GET /api/stations
PUT /api/stations/{station_id}
```

Recommended station shape:

```json
{
  "station_id": "station_1",
  "name": "Station 1",
  "device_identifier": "Sony A6000 #1",
  "input_folder": "D:\\Selfstudio\\input\\station-1",
  "background_name": "Default Background",
  "default_lut_path": "D:\\Selfstudio\\luts\\default.cube",
  "output_rule": "{customer_name}/{order_number}/{station_id}"
}
```

Recommended list response:

```json
{
  "data": {
    "stations": []
  }
}
```

Recommended update request fields should match editable station config fields. Do not allow changing `station_id` through request body unless intentionally required.

Recommended errors:

- `STATION_NOT_FOUND`
- `INVALID_STATION_CONFIG`
- `DUPLICATE_INPUT_FOLDER`
- `UNAUTHORIZED`
- `UNTRUSTED_ORIGIN`
- `ACTIVITY_UNAVAILABLE` only if activity logging remains hard dependency for this endpoint

### Station IDs and Count

Use exactly three logical stations for MVP, matching PRD/epics. Prefer stable IDs rather than generated UUIDs for this local setup foundation:

- `station_1`
- `station_2`
- `station_3`

Do not let UI create a fourth station in this story. If future requirements need dynamic station count, that should be a later product/architecture decision.

### Persistence Guidance

Architecture eventually expects Supabase Postgres as metadata system of record with tables like `stations` and `station_configs`. However, Epic 1 has not introduced Supabase migrations or pgx access yet.

For this story, acceptable implementation paths:

1. **Preferred if small:** local JSON station config file under `SELFSTUDIO_LOCAL_DATA_DIR` with atomic write and tests.
2. **Acceptable foundation:** in-memory store with explicit TODO/debt note in story completion, if implementing durable local config would expand scope too much.
3. **Do not do unless explicitly scoped:** full Supabase migration and pgx store, because that may require broader schema/bootstrap work.

If using a local JSON file, preserve Windows path strings exactly in API responses, but normalize for duplicate detection.

### Duplicate Folder Validation

Duplicate input folders should be detected after normalization:

- trim surrounding whitespace
- clean path separators using Go path/filepath functions
- case-insensitive comparison on Windows
- reject duplicates across different station IDs

Do not require that the folders actually exist yet; readiness/file accessibility belongs to Story 2.2.

### Activity Logging Requirements

Record config update attempts with safe messages only:

- Success action type: `station.config_updated`
- Failure action type: `station.config_update_failed`
- Result: `success` or `failure`
- Include `station_id` reference when available.
- Do not log full request bodies, local auth tokens, PIN/password, Supabase credentials, Google credentials, or overly sensitive path details beyond what is operationally necessary.

### SSE Requirements

After successful station update, publish a `station.updated` event through existing broker.

Recommended event envelope fields:

- `event_id`: generated by events package
- `event_type`: `station.updated`
- `entity_type`: `station`
- `entity_id`: station ID
- `occurred_at`: UTC timestamp
- `data`: updated station object or minimal `{ "station_id": "station_1" }`

Frontend should invalidate the stations query on this event. It should not invent station workflow state locally.

### Existing Files Likely to Update

Read these files before editing:

- `apps/agent/cmd/selfstudio-agent/main.go` — current dependency wiring for auth, events, activity, mux.
- `apps/agent/internal/api/health.go` — current `NewMux` route registration pattern.
- `apps/agent/internal/api/security.go` — trusted Origin guard for state-changing endpoints.
- `apps/agent/internal/api/middleware.go` — auth middleware.
- `apps/agent/internal/activity/store.go` — `RecordWithRefs` for station activity references.
- `apps/agent/internal/events/broker.go` and `event.go` — event publish/validation patterns.
- `apps/web/src/features/health/health-dashboard.tsx` — current dashboard composition and SSE auth-expiry behavior.
- `apps/web/src/features/activity/activity-log.tsx` — existing dashboard activity section.
- `apps/web/src/lib/api/client.ts` — API helper and shared types.
- `docs/api/openapi.yaml` — contract file to extend.

### Frontend Guidance

Suggested files:

```text
apps/web/src/features/stations/station-settings.tsx
apps/web/src/features/stations/use-stations-query.ts
apps/web/src/features/stations/use-update-station-mutation.ts
```

Use existing local UI primitives from:

```text
apps/web/src/components/ui/card.tsx
apps/web/src/components/ui/button.tsx
apps/web/src/components/ui/badge.tsx
```

If form fields need styling, use plain semantic inputs with existing global CSS or add minimal reusable styles. Avoid broad UI framework setup in this story.

### Testing Standards Summary

Go tests:

- Validate exact three default stations.
- Validate required fields.
- Validate duplicate folder detection with case/path separator normalization.
- Validate successful update returns `{data}` wrapper.
- Validate unauthenticated station endpoints return `401`.
- Validate state-changing station update rejects untrusted `Origin`.
- Validate activity logging on success and failure.
- Validate `station.updated` is published on successful update.

Frontend validation:

- `npm run typecheck`
- `npm run build`
- Manual browser/API smoke where feasible.

### Project Structure Notes

- Keep backend station logic in Go under `apps/agent/internal/*`.
- Keep frontend station UI under `apps/web/src/features/stations`.
- Keep API contract in `docs/api/openapi.yaml`.
- Do not add station logic to the existing root TypeScript prototype.
- Continue additive architecture; preserve Epic 1 behavior end-to-end.

### References

- [Source: `_bmad-output/planning-artifacts/epics.md` → Epic 2 / Story 2.1]
- [Source: `_bmad-output/planning-artifacts/architecture.md` → API & Communication Patterns]
- [Source: `_bmad-output/planning-artifacts/architecture.md` → Frontend Architecture]
- [Source: `_bmad-output/planning-artifacts/architecture.md` → Implementation Patterns & Consistency Rules]
- [Source: `_bmad-output/planning-artifacts/architecture.md` → Project Structure & Boundaries]
- [Source: `_bmad-output/implementation-artifacts/epic-1-retro-2026-05-18.md` → Action Items / Next Epic Preparation]
- [Source: `_bmad-output/implementation-artifacts/1-5-record-and-view-operator-activity-log-foundation.md` → Previous Story Intelligence]

## Dev Agent Record

### Agent Model Used

{{agent_model_name_version}}

### Debug Log References

- `cd apps/agent && go test ./...` -> passed.
- Initial station API test run failed because test helper built malformed JSON for duplicate path case; fixed test helper to marshal JSON safely.
- `cd apps/web && npm run typecheck && npm run build` -> passed.
- Review patch validation `cd apps/agent && go test ./...` -> passed.
- Review patch validation `cd apps/web && npm run typecheck && npm run build` -> passed.
- Manual review patch smoke confirmed non-JSON station update returns `415 UNSUPPORTED_MEDIA_TYPE` and unsafe output rule returns `400 INVALID_STATION_CONFIG`.
- Full validation `cd apps/agent && go test ./... && cd ../web && npm run typecheck && npm run build` -> passed.
- Manual smoke with local agent: login, `GET /api/stations`, valid `PUT /api/stations/station_1`, duplicate input folder rejection, and `station.config_updated` activity fetch all worked.
### Completion Notes List

- Ultimate context engine analysis completed - comprehensive developer guide created.
- Implemented in-memory station configuration foundation with exactly three stable stations (`station_1`, `station_2`, `station_3`).
- Added protected `GET /api/stations` and `PUT /api/stations/{station_id}` endpoints with trusted Origin guard on mutation.
- Added required-field and duplicate input folder validation.
- Added safe activity logging for station config update success/failure.
- Added `station.updated` SSE event publication and frontend query invalidation.
- Added station settings UI under authenticated dashboard with TanStack Query hooks.
- Updated OpenAPI contract for station endpoints and schemas.
- Persistence is in-memory for this story; Supabase/local durable config remains explicit follow-up for later station/data stories.
- Applied 16 Story 2.1 code review patches covering station validation, request hardening, frontend query/edit edge cases, OpenAPI gaps, and additional tests.



### Change Log

- 2026-05-18: Created Story 2.1 station configuration context and marked ready for development.
- 2026-05-18: Implemented station configuration foundation and marked ready for review.
- 2026-05-18: Applied Story 2.1 code review patches.

### File List

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/response.go`
- `apps/agent/internal/api/stations.go`
- `apps/agent/internal/api/stations_test.go`
- `apps/agent/internal/stations/store.go`
- `apps/agent/internal/stations/store_test.go`
- `apps/web/src/app/globals.css`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/features/stations/station-settings.tsx`
- `apps/web/src/features/stations/use-stations-query.ts`
- `apps/web/src/features/stations/use-update-station-mutation.ts`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`
