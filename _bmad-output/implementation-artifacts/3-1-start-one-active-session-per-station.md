# Story 3.1: Start One Active Session Per Station

Status: done

## Story

As an operator,
I want to create a session for a station with customer, order number, and timer,
so that incoming photos route to the correct customer/order.

## Acceptance Criteria

1. Given an authenticated operator starts a session for a station with customer name, order number, and timer duration, then backend creates an active session for that station.
2. Given a station already has an active session, when operator tries to start another session on that same station, then request is blocked with structured `SESSION_ALREADY_ACTIVE` error and actionable text.
3. Given session starts successfully, then session stores a snapshot of station config at start time including station name, background name, default LUT, output rule, input folder, output folder, and device identifier.
4. Given session starts successfully, then activity log records `session.started` with safe station/session references and no customer PII beyond operational identifiers required by product.
5. Given session starts successfully, then SSE publishes `session.started` and dashboard invalidates sessions, stations/live cards, event readiness, and activity.
6. Given readiness for the station has required failed checks, then session start is blocked with structured readiness error and actionable text.
7. Given session start endpoint is called, then it is protected by local auth and trusted Origin guard.
8. Given app restarts after a session was started, then active session state can be restored from local durable storage.
9. Given API docs are inspected, then OpenAPI documents session start endpoint, session response, structured errors, activity, and SSE event.

## Tasks / Subtasks

- [x] Create session domain and persistence (AC: 1, 2, 3, 8)
  - [x] Add `apps/agent/internal/sessions` package.
  - [x] Define `Session`, `SessionStatus`, `StationSnapshot`, `StartSessionRequest`.
  - [x] Implement store enforcing one active session per station.
  - [x] Implement atomic local persistence under `local-data/state/sessions.json`.
  - [x] Load sessions on Go agent startup; malformed persisted state fails safe with clear startup error.
- [x] Implement start session API (AC: 1, 2, 6, 7, 9)
  - [x] Add `POST /api/stations/{station_id}/sessions`.
  - [x] Validate customer name, order number, and timer duration.
  - [x] Reuse station readiness validator and block required failed status.
  - [x] Return `{data:{session:{...}}}`.
  - [x] Map errors: `STATION_NOT_FOUND`, `SESSION_ALREADY_ACTIVE`, `SESSION_READINESS_BLOCKED`, `INVALID_SESSION_REQUEST`, `SESSIONS_UNAVAILABLE`.
  - [x] Apply local auth and trusted Origin guard.
- [x] Add activity and SSE (AC: 4, 5)
  - [x] Record `session.started` on success.
  - [x] Record `session.start_failed` on blocked/failure cases when safe.
  - [x] Publish `session.started` with session summary payload.
- [x] Frontend integration (AC: 1, 2, 5, 6)
  - [x] Add start session form/control per station card or settings area.
  - [x] Collect customer name, order number, timer duration.
  - [x] Show blocked active-session/readiness errors with action text.
  - [x] Add session query/mutation hooks.
  - [x] Listen for `session.started` SSE and invalidate sessions/stations/event readiness/activity.
- [x] OpenAPI and validation (AC: 7, 9)
  - [x] Document start session endpoint and schemas.
  - [x] Add Go tests for success, duplicate active session, readiness blocked, unknown station, auth, trusted Origin, persistence restore, activity, and SSE.
  - [x] Run `cd apps/agent && go test ./...`.
  - [x] Run `cd apps/web && npm run typecheck && npm run build`.

## Dev Notes

### Scope Boundary

This story creates session state and start-session behavior only. It does not implement photo ingestion, folder watcher production daemon, photo routing, session ending, timer auto-end, local result folder opening, or upload status. Those are later Epic 3/Epic 4 stories.

### Architecture Guidance

- Go service owns authoritative session state.
- Frontend must not invent session workflow state machines.
- API responses use `{data}` wrapper and structured `{error}` failures.
- Status/JSON fields use `snake_case`.
- SSE event names use dot notation.
- Customer name/order number are operational data and must not be logged unnecessarily beyond session record and safe references.

### Readiness Rule for This Story

Session start may proceed only if required station readiness checks do not fail. Device check can still be `unknown` from Epic 2 MVP, but failed input/output/LUT checks must block. If aggregate status is `failed`, block with `SESSION_READINESS_BLOCKED`.

### Persistence Pattern

Use the station config persistence lessons:

- write temp file
- sync file
- rename/replace atomically
- skip parent directory sync on Windows if access denied
- validate loaded state before accepting it

### Suggested Contract

```http
POST /api/stations/{station_id}/sessions
Content-Type: application/json

{
  "customer_name": "Customer A",
  "order_number": "ORD-001",
  "timer_seconds": 900
}
```

Response:

```json
{
  "data": {
    "session": {
      "session_id": "sess_...",
      "station_id": "station_1",
      "status": "active",
      "customer_name": "Customer A",
      "order_number": "ORD-001",
      "timer_seconds": 900,
      "started_at": "2026-05-19T10:00:00Z",
      "ends_at": "2026-05-19T10:15:00Z",
      "station_snapshot": {
        "station_name": "Station 1",
        "background_name": "White",
        "default_lut_path": "D:/LUTs/default.cube",
        "input_folder": "D:/Capture/Station1",
        "output_rule": "station_1/{order_number}",
        "device_identifier": "CAM-1"
      }
    }
  }
}
```

SSE:

```json
{
  "event_type": "session.started",
  "entity_type": "session",
  "entity_id": "sess_...",
  "payload": { "session": {} }
}
```


### Review Findings

- [x] [Review][Patch] Session creation was not atomic with persistence failure; rollback added on save failure.
- [x] [Review][Patch] AC3 snapshot missed `output_folder`; added derived output folder to backend, frontend type, and OpenAPI.
- [x] [Review][Patch] Missing readiness-blocked API test; added `SESSION_READINESS_BLOCKED` test.
- [x] [Review][Patch] `session.started` SSE did not invalidate station readiness for remote dashboards; added station-specific invalidation.
- [x] [Review][Patch] Event readiness validator rejected `session_start_available: true`; changed to boolean validation.
- [x] [Review][Patch] OpenAPI omitted timer max; added `maximum: 86400`.
- [x] [Review][Patch] Frontend start form sent predictable invalid requests; added disabled validation and timer max.
- [x] [Review][Patch] Request body accepted trailing JSON junk; added second decode guard.
- [x] [Review][Patch] Persisted session validation allowed invalid lifecycle/status/timer; tightened validation.
- [x] [Review][Defer] Full concurrent start+save serialization and Windows atomic replace primitive — deferred as broader persistence hardening.
- [x] [Review][Defer] End-session/timer-expiry release behavior — deferred to Story 3.3.
- [x] [Review][Defer] Dedicated session list/read endpoint/live-card query — deferred to Story 3.2.

## Dev Agent Record

### Agent Model Used

{{agent_model_name_version}}

### Debug Log References

- `cd apps/agent && go test ./...` -> passed.
- `cd apps/web && npm run typecheck && npm run build` -> passed.
### Completion Notes List

- Applied Story 3.1 review patches for output folder snapshot, rollback on persistence failure, readiness-blocked tests, SSE invalidation, OpenAPI bounds, frontend validation, trailing JSON rejection, and persisted state validation.
- Review patch validation passed: `cd apps/agent && go test ./...`; `cd apps/web && npm run typecheck && npm run build`.
- Implemented session domain/store/persistence with one-active-session-per-station enforcement.
- Added `POST /api/stations/{station_id}/sessions` with local auth, trusted Origin, readiness blocking, persistence, activity, and SSE.
- Added frontend start session controls and mutation with query invalidation.
- Updated OpenAPI with session start schemas/errors.
### File List

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/sessions.go`
- `apps/agent/internal/api/sessions_test.go`
- `apps/agent/internal/sessions/store.go`
- `apps/agent/internal/sessions/persistence.go`
- `apps/web/src/features/sessions/use-start-session-mutation.ts`
- `apps/web/src/features/stations/station-settings.tsx`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`
