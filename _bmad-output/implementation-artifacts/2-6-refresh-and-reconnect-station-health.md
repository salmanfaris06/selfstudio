# Story 2.6: Refresh and Reconnect Station Health

Status: done

## Story

As an operator,
I want to refresh or reconnect a station with camera/folder issues,
so that I can recover event readiness without restarting everything.

## Acceptance Criteria

1. Given an authenticated operator sees a station with attention/warning/failed/unknown readiness, when they click refresh/reconnect/recheck, then the Go service reruns station-specific readiness checks for that station.
2. Given station-specific checks complete, when dashboard is open, then station readiness, station config, event readiness, and activity views refresh via SSE-driven invalidation.
3. Given refresh/reconnect succeeds, when operator checks the activity log, then a safe `station.health_refreshed` success entry exists.
4. Given refresh/reconnect fails due to unknown station, unavailable store, invalid folder, or readiness error, then API returns structured error or failed readiness result with specific operator action.
5. Given refresh/reconnect action is requested, then endpoint is protected by local auth and trusted Origin guard.
6. Given API docs are inspected, then `docs/api/openapi.yaml` documents refresh/reconnect endpoint, response schema, activity/SSE event, and errors.

## Tasks / Subtasks

- [x] Add station health refresh API (AC: 1, 4, 5, 6)
  - [x] Add `POST /api/stations/{station_id}/health/refresh` or equivalent.
  - [x] Reuse station readiness validator rather than creating a fake status.
  - [x] Return `{data:{readiness:{...}}}` using existing station readiness schema.
  - [x] Map unknown station and unavailable store to structured errors.
  - [x] Apply trusted Origin guard.
- [x] Add activity and SSE (AC: 2, 3)
  - [x] Record `station.health_refreshed` on success/warning result.
  - [x] Record `station.health_refresh_failed` on hard API failure or failed readiness aggregate.
  - [x] Publish `station.health_refreshed` SSE with station_id and readiness payload.
  - [x] Ensure messages contain no secrets or request bodies.
- [x] Extend frontend station UI (AC: 1, 2, 4)
  - [x] Add “Refresh station health” or “Reconnect/Recheck” button per station.
  - [x] Disable while form has unsaved edits or readiness mutation is pending.
  - [x] Show success/error state with actionable text.
  - [x] Invalidate station readiness, station list, event readiness, and activity on SSE.
- [x] Update OpenAPI (AC: 6)
  - [x] Document endpoint, schemas, examples, and errors.
- [x] Add tests and validation (AC: 1-6)
  - [x] Go API tests for auth, trusted Origin, unknown station, success activity, failed activity, and SSE.
  - [x] Frontend typecheck/build validation.
  - [x] Manual smoke: configure valid station folders/LUT, run refresh, verify readiness and activity/SSE.

## Dev Notes

### Scope Boundary

This story provides an operator-facing recovery/recheck action for station health. It does not implement real camera reconnect SDK integration, long-running watcher daemon restart, session start, photo routing, or ingestion. Device state may remain `unknown` until real camera integration exists.

### Previous Story Intelligence

- Story 2.2 already implements station readiness checks and `POST /api/stations/{station_id}/readiness/check`.
- Story 2.3 event readiness invalidates based on station/readiness SSE.
- Story 2.5 added `station.validation_completed` SSE and validation-only watch test.
- Mutation endpoints must use local auth + trusted Origin.
- Do not show fake ready for unknown device state.

### Suggested Contract

Endpoint:

```http
POST /api/stations/{station_id}/health/refresh
```

Response:

```json
{
  "data": {
    "readiness": {
      "station_id": "station_1",
      "status": "warning",
      "label": "Station perlu verifikasi operator",
      "action": "Periksa device/tether lalu refresh lagi.",
      "checked_at": "2026-05-18T10:30:00Z",
      "checks": []
    }
  }
}
```

SSE:

```json
{
  "event_type": "station.health_refreshed",
  "entity_type": "station",
  "entity_id": "station_1",
  "payload": { "readiness": { } }
}
```

Activity:

- `station.health_refreshed`
- `station.health_refresh_failed`

### Existing Files Likely to Update

- `apps/agent/internal/api/readiness.go`
- `apps/agent/internal/api/readiness_test.go`
- `apps/agent/internal/api/health.go`
- `apps/web/src/features/stations/station-settings.tsx`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`


### Review Findings

- [x] [Review][Patch] Activity log was not refreshed from `station.health_refreshed` SSE [apps/web/src/features/health/health-dashboard.tsx]
- [x] [Review][Patch] OpenAPI had unresolved `ReadinessCheckFailed` response reference [docs/api/openapi.yaml]
- [x] [Review][Patch] UI reported refresh success even when readiness result was failed [apps/web/src/features/stations/station-settings.tsx]
- [x] [Review][Patch] No API tests existed for the new health refresh behavior [apps/agent/internal/api/health_refresh_test.go]
- [x] [Review][Patch] Malformed SSE payload catch paths left stale queries [apps/web/src/features/health/health-dashboard.tsx]
- [x] [Review][Defer] Wire `onAuthExpired` into SSE/API auth expiry handling — deferred, pre-existing auth UX issue outside Story 2.6
- [x] [Review][Defer] Revisit login/logout trusted-Origin/auth contract and public health endpoint — deferred, pre-existing security contract outside Story 2.6
- [x] [Review][Defer] Loosen placeholder-era event readiness runtime validation for future session-start implementation — deferred, future Epic 3 compatibility issue
- [x] [Review][Defer] Split refresh success/failure SSE event names further — deferred, payload carries readiness status and current event means refresh completed

## Dev Agent Record

### Agent Model Used

{{agent_model_name_version}}

### Debug Log References

- `cd apps/agent && go test ./...` -> passed.
- `cd apps/web && npm run typecheck && npm run build` -> passed.
### Completion Notes List

- Applied Story 2.6 review patches: activity invalidation, OpenAPI missing response, UI message based on readiness status, API tests, and stale-query invalidation on malformed SSE.
- Review patch validation: `cd apps/agent && go test ./...` -> passed after retry; first run hit transient Windows `Access is denied`.
- Review patch validation: `cd apps/web && npm run typecheck && npm run build` -> passed.
- Ultimate context engine analysis completed - comprehensive developer guide created.
- Implemented station health refresh endpoint reusing station readiness validator.
- Added activity logging and `station.health_refreshed` SSE.
- Added frontend refresh station health button and SSE invalidation.
- Updated OpenAPI with station health refresh endpoint.

### File List

- `apps/agent/internal/api/readiness.go`
- `apps/agent/internal/api/health.go`
- `apps/web/src/features/stations/use-refresh-station-health-mutation.ts`
- `apps/web/src/features/stations/station-settings.tsx`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`
