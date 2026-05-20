# Story 1.3: Establish API, Error, and SSE Contract Foundation

Status: done

## Story

As a frontend developer,
I want stable REST, error, and SSE response contracts,
so that dashboard features can integrate consistently.

## Acceptance Criteria

1. Given Go service is running, when client calls `/api/health`, then response uses `{data}` wrapper with `snake_case` fields.
2. Given an API request fails, when Go service returns an error, then response uses `{ "error": { "code", "message", "action", "details" } }` with operator-actionable message/action.
3. Given authenticated client connects to `/events`, when the connection is accepted, then Go service exposes an SSE stream with dot-notation event names.
4. Given an SSE message is emitted, when client receives it, then payload uses standard event wrapper: `event_id`, `event_type`, `entity_type`, `entity_id`, `occurred_at`, and `data`.
5. Given API docs are inspected, when docs are opened, then `docs/api/openapi.yaml` documents initial health/auth/event endpoints and shared response schemas.
6. Given frontend code integrates with API, when it consumes health/errors/events, then shared TypeScript helpers/types make the response contracts consistent and do not expose secrets.

## Tasks / Subtasks

- [x] Align REST health endpoint and response helpers (AC: 1, 2)
  - [x] Add or preserve `/api/health` while keeping existing `/health` compatibility if needed for scripts/smoke tests.
  - [x] Ensure health response uses `{ "data": ... }` and `snake_case` fields only.
  - [x] Centralize Go success/error response helpers for reuse across auth and future handlers.
  - [x] Ensure errors use `code`, `message`, `action`, and `details` consistently.
- [x] Add SSE event model and broker foundation (AC: 3, 4)
  - [x] Create Go event types under `apps/agent/internal/events`.
  - [x] Define event wrapper fields: `event_id`, `event_type`, `entity_type`, `entity_id`, `occurred_at`, `data`.
  - [x] Enforce dot-notation event names such as `health.updated`.
  - [x] Add a minimal in-memory broker suitable for MVP foundation.
  - [x] Add `/events` handler with `text/event-stream`, no-cache headers, keepalive/comment behavior, and disconnect handling.
  - [x] Require valid local auth session for `/events` unless a deliberate health-only exception is documented.
- [x] Add frontend API/event contract helpers (AC: 6)
  - [x] Extend `apps/web/src/lib/api/client.ts` or create adjacent modules for shared response/error types.
  - [x] Add a browser-safe SSE helper that connects to the configured Go API `/events` using credentials.
  - [x] Ensure frontend helpers never include service credentials, auth PIN, Supabase service role, or Google credentials.
- [x] Update OpenAPI documentation (AC: 1, 2, 3, 4, 5)
  - [x] Document `/api/health` and retain `/health` if implemented as compatibility alias.
  - [x] Document shared `DataResponse`, `ErrorResponse`, health response, auth responses, and event wrapper schema.
  - [x] Document `/events` as `text/event-stream` with event wrapper example.
  - [x] Include implemented auth error responses from Story 1.2 (`INVALID_REQUEST`, `INVALID_PIN`, `TOO_MANY_ATTEMPTS`, `AUTH_SESSION_FAILED`, `UNAUTHORIZED`).
- [x] Add tests and verification (AC: 1, 2, 3, 4, 5, 6)
  - [x] Add Go tests for `/api/health`, error helper shape, SSE event serialization, and `/events` headers/auth behavior.
  - [x] Add frontend typecheck/build validation.
  - [x] Add manual smoke notes for health contract and SSE connection behavior.

### Review Findings

- [x] [Review][Patch] SSE stream can be terminated by server WriteTimeout before first keepalive [apps/agent/cmd/selfstudio-agent/main.go:27]
- [x] [Review][Patch] SSE event type validation is too permissive and not enforced at publish/serialization boundary [apps/agent/internal/events/event.go:39]
- [x] [Review][Patch] SSE initial and keepalive writes ignore write failures and can leave broken clients subscribed until context closes [apps/agent/internal/api/events.go:32]
- [x] [Review][Patch] Frontend EventSource helper can throw in non-browser contexts and lacks typed event parsing helper [apps/web/src/lib/events/client.ts:3]
- [x] [Review][Patch] OpenAPI lacks cookie security scheme for protected `/events` and does not clearly link SSE frames to EventEnvelope [docs/api/openapi.yaml:28]
- [x] [Review][Patch] Event ID implementation and docs/examples disagree on format [apps/agent/internal/events/event.go:70]
- [x] [Review][Patch] Frontend shared API response types are not exported for consistent health/error contract consumption [apps/web/src/lib/api/client.ts:1]

## Dev Notes

### Scope Boundary

This story is contract foundation only. Do not implement station health dashboard UI, TanStack Query dashboard refresh, real station/session/photo events, camera watcher, Supabase schema, processing, or cloud upload here. Provide reusable contracts and a minimal event stream foundation for later stories.

### Previous Story Intelligence

Story 1.2 is complete and introduced:

- `SELFSTUDIO_AUTH_PIN` required by Go config.
- In-memory local auth sessions with `selfstudio_session` cookie.
- Auth endpoints: `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/session`.
- `api.RequireAuth(manager, next)` middleware for protected endpoints.
- Credentialed CORS for `http://localhost:3000` and `OPTIONS` preflight.
- API client in `apps/web/src/lib/api/client.ts` with `ApiError` handling and `credentials: "include"`.
- `docs/api/openapi.yaml` already documents auth and error schemas; extend rather than replace carelessly.

Story 1.1 established:

- Go entrypoint: `apps/agent/cmd/selfstudio-agent/main.go`.
- API package location: `apps/agent/internal/api`.
- Config package location: `apps/agent/internal/config`.
- Web app location: `apps/web/src/app` and `apps/web/src/lib`.
- Validation commands: `cd apps/agent && go test ./...`, `cd apps/web && npm run typecheck && npm run build`.

### Architecture Requirements

From architecture and epics:

- REST command endpoints live under `/api`.
- SSE stream lives under `/events`.
- API success responses use `{ "data": ... }`.
- API errors use `{ "error": { "code", "message", "action", "details" } }`.
- JSON fields use `snake_case` for API payloads and status fields.
- SSE event names use dot notation, examples: `station.updated`, `session.updated`, `photo.created`, `photo.processed`, `queue.updated`, `upload.updated`.
- SSE payload wrapper must include `event_id`, `event_type`, `occurred_at`, and `data`; this story also requires `entity_type` and `entity_id`.
- Dates/times use ISO 8601 UTC strings.
- Go service remains source of truth; frontend only consumes contracts.
- Browser never receives Supabase service role, Google credentials, or local auth secret.

### Recommended Go Design

Suggested files:

```text
apps/agent/internal/events/event.go
apps/agent/internal/events/broker.go
apps/agent/internal/events/broker_test.go
apps/agent/internal/api/events.go
apps/agent/internal/api/events_test.go
apps/agent/internal/api/health.go
apps/agent/internal/api/health_test.go
apps/agent/internal/api/response.go
apps/agent/internal/api/response_test.go
```

Recommended event shape:

```go
type Event struct {
    EventID    string         `json:"event_id"`
    EventType  string         `json:"event_type"`
    EntityType string         `json:"entity_type"`
    EntityID   string         `json:"entity_id"`
    OccurredAt time.Time      `json:"occurred_at"`
    Data       map[string]any `json:"data"`
}
```

For SSE wire format, emit an SSE `event:` line using the dot-notation `event_type` and a `data:` line containing the JSON event wrapper.

Minimal broker is acceptable: subscribe/unsubscribe clients, publish events to subscribers, drop events for slow clients rather than blocking critical workflows. Do not overbuild persistence/replay in this story.

### API Compatibility Guidance

Existing Story 1.1 smoke used `/health`. Story 1.3 AC says `/api/health`. Implement `/api/health` as canonical contract endpoint and keep `/health` as compatibility alias unless there is a strong reason to remove it. Update OpenAPI to document both if both exist.

### Frontend Guidance

Suggested frontend files:

```text
apps/web/src/lib/api/client.ts
apps/web/src/lib/events/client.ts
apps/web/src/lib/events/types.ts
```

The event helper should be small and browser-safe. It may expose a function that creates `EventSource` for `${apiBaseUrl}/events`. Remember that native `EventSource` supports credentials using `{ withCredentials: true }` in modern browsers.

Do not wire dashboard UI to real health cards yet; Story 1.4 owns health dashboard shell.

### Security and Auth Guidance

- Keep `/api/health` public unless product explicitly changes readiness/health visibility.
- Protect `/events` with existing local session middleware because event streams will later carry operational state.
- Keep CORS behavior compatible with Story 1.2 browser auth flow.
- Do not add secrets to frontend env.

### Testing Requirements

Run and record:

- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build`
- Manual smoke or equivalent:
  - `GET /api/health` returns `200` with `{data}` wrapper.
  - Unauthenticated `GET /events` returns `401` error response or equivalent denied behavior.
  - Authenticated `GET /events` returns `text/event-stream` headers.
  - SSE event payload follows wrapper shape.

### Anti-Patterns to Avoid

- Do not implement dashboard health UI in this story.
- Do not introduce WebSockets; architecture chose SSE.
- Do not return raw Go errors.
- Do not make frontend own workflow state machines.
- Do not remove existing auth endpoints from Story 1.2.
- Do not expose service credentials or auth PIN to browser.

### References

- [Source: `_bmad-output/planning-artifacts/epics.md` -> Story 1.3]
- [Source: `_bmad-output/planning-artifacts/architecture.md` -> API & Communication Patterns]
- [Source: `_bmad-output/planning-artifacts/architecture.md` -> Communication Patterns]
- [Source: `_bmad-output/planning-artifacts/architecture.md` -> Project Structure & Boundaries]
- [Source: `_bmad-output/implementation-artifacts/1-2-add-local-pin-gate.md` -> Previous Story Intelligence]

## Project Structure Notes

- Continue extending `apps/agent/internal/api` and add `apps/agent/internal/events`.
- Continue extending `apps/web/src/lib`; avoid UI-heavy work until Story 1.4.
- OpenAPI remains the source contract file at `docs/api/openapi.yaml`.

## Dev Agent Record

### Agent Model Used

GPT-5.1 Codex Max

### Debug Log References

- `cd apps/agent && go test ./...` -> passed.
- `cd apps/web && npm run typecheck && npm run build` -> passed.
- Manual smoke with `SELFSTUDIO_AUTH_PIN=123456 go run ./cmd/selfstudio-agent`:
  - `GET /api/health` returned `200` with `{data}` wrapper.
  - unauthenticated `GET /events` returned `401` with `{error}` wrapper.
  - valid login set local auth cookie.
  - authenticated `GET /events` returned `200`, `Content-Type: text/event-stream`, `Cache-Control: no-cache`, and initial `: connected` SSE comment.
- Review patch validation:
  - `cd apps/agent && go test ./...` -> passed.
  - `cd apps/web && npm run typecheck && npm run build` -> passed.
  - Manual smoke after patches confirmed `/api/health`, unauthenticated `/events`, login, and authenticated `/events` stream headers.

### Completion Notes List

- Story created by BMad create-story workflow.
- Ultimate context engine analysis completed - comprehensive developer guide created.

### Change Log

- 2026-05-18: Created Story 1.3 API/error/SSE contract foundation and marked ready for development.
- 2026-05-18: Implemented API/error/SSE contract foundation and marked ready for review.
- 2026-05-18: Applied Story 1.3 code review patches and marked done.

### File List

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/api/events.go`
- `apps/agent/internal/api/events_test.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/health_test.go`
- `apps/agent/internal/api/response.go`
- `apps/agent/internal/api/response_test.go`
- `apps/agent/internal/events/broker.go`
- `apps/agent/internal/events/broker_test.go`
- `apps/agent/internal/events/event.go`
- `apps/web/src/lib/api/client.ts`
- `apps/web/src/lib/events/client.ts`
- `apps/web/src/lib/events/types.ts`
- `docs/api/openapi.yaml`
