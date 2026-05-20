# Story 1.5: Record and View Operator Activity Log Foundation

Status: done

## Story

As an admin,
I want core operator actions recorded and visible,
So that troubleshooting has an audit trail from the start.

## Acceptance Criteria

1. Given authenticated operator performs login, logout, health recheck, or config placeholder action, when action completes or fails, then Go service records a timestamped activity log entry.
2. Given a log entry is recorded, then it includes action type, result, safe message, and optional station/session references when available.
3. Given the dashboard is open, then it exposes an activity log view with timestamp and action filter.
4. Given logs are displayed or fetched, then logs avoid unnecessary sensitive data.
5. Given frontend requests activity logs, then the API supports fetching recent logs.

## Tasks / Subtasks

- [x] Add Go activity log model/store (AC: 1, 2, 4)
  - [x] Create `apps/agent/internal/activity` package with safe in-memory log store.
  - [x] Define log fields: `id`, `occurred_at`, `action_type`, `result`, `message`, optional `station_id`, optional `session_id`.
  - [x] Keep entries bounded to avoid unbounded memory growth.
- [x] Record core actions (AC: 1, 4)
  - [x] Record login success/failure without logging PIN/password.
  - [x] Record logout success/failure where applicable.
  - [x] Record health recheck via `/api/health`.
  - [x] Add minimal authenticated config placeholder action endpoint and record result.
- [x] Add activity API (AC: 2, 4, 5)
  - [x] Add `GET /api/activity` for recent entries.
  - [x] Add optional `action_type` filter.
  - [x] Protect activity endpoints with local session auth.
  - [x] Return `{data}` wrapper and existing `{error}` contract.
- [x] Add dashboard activity log view (AC: 3, 4, 5)
  - [x] Add frontend API helper/types for recent activity logs.
  - [x] Add TanStack Query hook for activity logs.
  - [x] Render timestamp, action type, result, safe message, and action filter.
  - [x] Ensure loading/error/retry states are visible and actionable.
- [x] Update docs and validation (AC: 1-5)
  - [x] Update `docs/api/openapi.yaml` for activity endpoints and schemas.
  - [x] Run Go tests.
  - [x] Run frontend typecheck/build.

### Review Findings

- [x] [Review][Patch] Unauthenticated public health requests are recorded as operator activity and can spam/evict real audit entries [apps/agent/internal/api/health.go:32]
- [x] [Review][Patch] Login rate limiting trusts spoofable or malformed `X-Forwarded-For` values [apps/agent/internal/api/auth.go:127]
- [x] [Review][Patch] State-changing cookie-authenticated endpoints lack Origin/CSRF guard [apps/agent/internal/api/health.go:27]
- [x] [Review][Patch] Logout always records `logout.success` even when no valid session/logout occurred [apps/agent/internal/api/auth.go:55]
- [x] [Review][Patch] Empty PIN is treated as auth failure instead of invalid request [apps/agent/internal/api/auth.go:87]
- [x] [Review][Patch] Activity fallback IDs can collide because random failure returns constant `act_fallback` [apps/agent/internal/activity/store.go:81]
- [x] [Review][Patch] Activity store limit handling resets high limits to 50 instead of clamping [apps/agent/internal/activity/store.go:55]
- [x] [Review][Patch] Activity `action_type` filter accepts arbitrary unbounded strings [apps/agent/internal/api/activity.go:30]
- [x] [Review][Patch] Activity store cannot record optional station/session references even when available [apps/agent/internal/activity/store.go:31]
- [x] [Review][Patch] Config placeholder route protection is not covered by mux-level tests [apps/agent/internal/api/activity_test.go:83]
- [x] [Review][Patch] Activity UI can crash on malformed `entries` or invalid timestamps and can duplicate config placeholder records on rapid clicks [apps/web/src/features/activity/activity-log.tsx:22]
- [x] [Review][Patch] OpenAPI omits `500 ACTIVITY_UNAVAILABLE` and nullable field requiredness is inconsistent [docs/api/openapi.yaml:102]

## Dev Notes

### Scope Boundary

Foundation only. Use in-memory log storage for MVP. Do not add Supabase persistence, full audit compliance, export, pagination beyond a simple recent limit, station/session real references, or advanced filters yet.

### Previous Story Intelligence

- Story 1.2 added local auth and session cookie.
- Story 1.3 added shared API response and protected `/events`.
- Story 1.4 added TanStack Query provider, health dashboard shell, local UI primitives, and health query.

### Suggested API

- `GET /api/activity?limit=50&action_type=login.success`
- `POST /api/config/placeholder-action`

Suggested response shape:

```json
{
  "data": {
    "entries": [
      {
        "id": "act_...",
        "occurred_at": "2026-05-18T10:30:00Z",
        "action_type": "login.success",
        "result": "success",
        "message": "Operator login berhasil.",
        "station_id": null,
        "session_id": null
      }
    ]
  }
}
```

### Security Guidance

- Never log PIN/password, session cookie/token, request bodies, Supabase service role, or Google credentials.
- Keep log messages operator-safe.
- Protect log fetch and config placeholder action with local auth.

### Testing Requirements

Run and record:

- `cd apps/agent && go test ./...`
- `cd apps/web && npm run typecheck`
- `cd apps/web && npm run build`

## Dev Agent Record

### Agent Model Used

GPT-5.1 Codex Max

### Debug Log References

- Initial `cd apps/agent && go test ./...` hit Windows transient `Access is denied` while executing activity test binary.
- Retried with `cd apps/agent && go test ./internal/activity -count=1 && go test ./...` -> passed.
- `cd apps/web && npm run typecheck && npm run build` -> passed.
- Manual smoke: login recorded `login.success`, authenticated `GET /api/activity` returned recent entries, `POST /api/config/placeholder-action` recorded `config.placeholder`, and `action_type` filter returned matching entry.
- Review patch validation:
  - `cd apps/agent && go test ./...` -> passed.
  - `cd apps/web && npm run typecheck && npm run build` -> passed.
  - Manual smoke confirmed public health requests no longer create activity, login entries include safe session reference, and untrusted Origin is rejected for config placeholder action.

### Completion Notes List

- Story created and started from sprint workflow.
- Added bounded in-memory activity store.
- Recorded safe login success/failure, logout success, health recheck, and config placeholder actions.
- Added protected `/api/activity` and `/api/config/placeholder-action` endpoints.
- Added dashboard activity log view with action filter, refresh, loading/error/empty states, and safe text messages.
- Updated OpenAPI for activity schemas/endpoints.

### Change Log

- 2026-05-18: Created Story 1.5 activity log foundation and started development.
- 2026-05-18: Implemented activity log foundation and marked ready for review.
- 2026-05-18: Applied Story 1.5 code review patches and marked done.

### File List

- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/internal/activity/store.go`
- `apps/agent/internal/activity/store_test.go`
- `apps/agent/internal/api/activity.go`
- `apps/agent/internal/api/activity_test.go`
- `apps/agent/internal/api/auth.go`
- `apps/agent/internal/api/auth_extra_test.go`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/security.go`
- `apps/agent/internal/api/events_test.go`
- `apps/agent/internal/auth/session.go`
- `apps/web/src/app/globals.css`
- `apps/web/src/features/activity/activity-log.tsx`
- `apps/web/src/features/activity/use-activity-query.ts`
- `apps/web/src/features/health/health-dashboard.tsx`
- `apps/web/src/lib/api/client.ts`
- `docs/api/openapi.yaml`
