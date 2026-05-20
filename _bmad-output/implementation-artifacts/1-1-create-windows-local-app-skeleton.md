# Story 1.1: Create Windows Local App Skeleton

Status: done

## Story

As an admin,
I want to start the local dashboard and Go service from Windows scripts,
so that event operation can run from one admin PC.

## Acceptance Criteria

1. **Given** clean project checkout on Windows, **When** admin runs local start script, **Then** Go service starts on configured localhost port.
2. **Given** clean project checkout on Windows, **When** admin runs local start script, **Then** Next.js dashboard starts on configured localhost port.
3. **Given** script starts services, **When** startup output is printed, **Then** it prints local dashboard URL and local network URL placeholder or detection result.
4. **Given** required env values are missing, **When** admin runs start script, **Then** script fails with clear actionable message.
5. **Given** story implementation is complete, **When** repository is inspected, **Then** monorepo contains `apps/web`, `apps/agent`, `docs/api`, and database migration location.

## Tasks / Subtasks

- [x] Create target monorepo directories (AC: 5)
  - [x] Create `apps/web` for Next.js dashboard.
  - [x] Create `apps/agent` for Go local service.
  - [x] Create `docs/api` for OpenAPI contract.
  - [x] Create `supabase/migrations` for database migrations.
  - [x] Create `scripts` for Windows PowerShell launchers.
  - [x] Create runtime directories under `local-data` as needed: `input/station-1`, `input/station-2`, `input/station-3`, `output`, `quarantine`, `logs`, `tmp`.
- [x] Initialize or align Next.js dashboard skeleton (AC: 2, 5)
  - [x] Use official Next.js starter pattern from architecture: `create-next-app@latest apps/web --ts --tailwind --eslint --app --src-dir --import-alias "@/*"` when folder is empty.
  - [x] If existing root spike app conflicts, do not delete it silently; isolate new app under `apps/web` and document root spike as legacy/prototype.
  - [x] Add minimal dashboard home page that can render while backend health is separate.
- [x] Initialize Go service skeleton (AC: 1, 5)
  - [x] Run `go mod init selfstudio/agent` in `apps/agent` when `go.mod` does not exist.
  - [x] Add entrypoint `apps/agent/cmd/selfstudio-agent/main.go`.
  - [x] Add `/health` endpoint returning JSON using architecture response pattern: `{ "data": { ... } }` with `snake_case` fields.
  - [x] Read service host/port from env with safe defaults for local dev.
- [x] Add environment templates and config validation (AC: 4)
  - [x] Add root `.env.example` documenting ports and local paths.
  - [x] Add `apps/agent/.env.example` for Go service env.
  - [x] Add `apps/web/.env.example` or `.env.local.example` for public local API URL only.
  - [x] Startup scripts must fail clearly if required values are absent; optional values may use safe defaults.
- [x] Add Windows launcher scripts (AC: 1, 2, 3, 4)
  - [x] Add `scripts/dev.ps1` to start Go service and Next.js dev server in separate processes.
  - [x] Add `scripts/start.ps1` for production-like local start placeholder.
  - [x] Add `scripts/build.ps1` to build Next.js and Go executable placeholder/implementation.
  - [x] Print localhost URL and detected LAN URL or explicit “LAN URL detection unavailable” message.
- [x] Add initial API docs placeholder (AC: 5)
  - [x] Add `docs/api/openapi.yaml` with `/health` endpoint and response wrapper.
- [x] Add smoke tests/checks (AC: 1, 2, 4)
  - [x] Add Go test for health response or config validation where practical.
  - [x] Add minimal frontend build/typecheck path where practical.
  - [x] Document exact commands in story Dev Agent Record.

### Review Findings

- [x] [Review][Patch] Environment example files are ignored/not present in reviewable source — `.env.example`, `apps/agent/.env.example`, and `apps/web/.env.example` are required by the story but do not appear in git status/diff, likely due `.gitignore`; ensure templates are tracked without exposing real credentials. [`.gitignore` / `.env.example`]
- [x] [Review][Patch] Next.js scaffold does not satisfy Tailwind starter requirement — story/architecture requires official Next.js + Tailwind starter pattern, but web app lacks Tailwind/PostCSS config and dependencies. [`apps/web/package.json`]
- [x] [Review][Patch] `start.ps1` can fail on a clean checkout because it runs `next start` without ensuring dependencies are installed and production build exists. [`scripts/start.ps1`]
- [x] [Review][Patch] `start.ps1` ignores configured agent host/port propagation in spawned PowerShell; displayed URL can diverge from actual Go process config. [`scripts/start.ps1`]
- [x] [Review][Patch] Go HTTP server lacks basic timeouts, making even local service vulnerable to hung/slow connections. [`apps/agent/cmd/selfstudio-agent/main.go`]
- [x] [Review][Patch] `SELFSTUDIO_LOCAL_DATA_DIR=./local-data` resolves relative to `apps/agent` when scripts run from that directory, not repository-root `local-data`. [`scripts/dev.ps1`, `scripts/start.ps1`, `apps/agent/internal/config/config.go`]
- [x] [Review][Patch] Generated/runtime artifacts are unignored and show up as untracked review noise (`apps/web/.next`, `apps/web/tsconfig.tsbuildinfo`, `local-data/tmp/agent-smoke.pid`); add ignore rules while keeping `.gitkeep` placeholders. [`.gitignore`]
- [x] [Review][Patch] PowerShell scripts interpolate paths/env values into command strings; paths containing apostrophes or special characters can break process launch. [`scripts/dev.ps1`, `scripts/start.ps1`]
- [x] [Review][Patch] TypeScript and type packages are production dependencies instead of devDependencies, increasing install/runtime surface. [`apps/web/package.json`]

## Dev Notes

### Scope Boundary

This story is foundation only. Do **not** implement station config, auth PIN gate, SSE, Supabase schema, camera watcher, image processing, quarantine, or cloud upload here. Create skeleton paths and startup flow that later stories extend.

### Current Repository Reality

Existing repository is a TypeScript camera-input spike, not final architecture:

- Root `package.json` currently names project `selfstudio-camera-input-spike` and runs `tsx src/server/index.ts`.
- Existing `src/server/*` contains prototype camera/storage/watcher/diagnostic code.
- Existing root scripts include `1. INSTALL.bat`, `2. RUN.bat`, `setup-selfstudio-from-scratch.ps1`, `start-selfstudio-admin.ps1`.

Do not delete existing spike/prototype files unless user explicitly approves. Treat them as research/prototype assets. New architecture must live under `apps/web` and `apps/agent`.

### Architecture Requirements

Follow `architecture.md` exactly:

- Frontend: Next.js App Router + TypeScript + Tailwind; shadcn/ui comes later when UI primitives needed.
- Backend/local worker: Go service on Windows admin PC.
- Monorepo target structure:
  - `apps/web`
  - `apps/agent`
  - `docs/api/openapi.yaml`
  - `supabase/migrations`
  - `scripts/*.ps1`
  - `local-data/*`
- Go entrypoint: `apps/agent/cmd/selfstudio-agent/main.go`.
- Go packages later go under `apps/agent/internal/*`; for this story only create package skeletons when needed by health/config.
- Browser talks to Go service only. No direct browser Supabase or Google credential access.
- MVP runs separate Node/Next.js process and Go process via Windows PowerShell scripts.

### API Contract Requirements

Initial `/health` endpoint must already follow final contract style:

```json
{
  "data": {
    "service": "selfstudio-agent",
    "status": "ok"
  }
}
```

Rules:
- JSON fields use `snake_case`.
- Error responses, if implemented, use `{ "error": { "code", "message", "action", "details" } }`.
- Dates/times use ISO 8601 UTC when present.

### Windows Script Requirements

PowerShell scripts must be Windows-first and operator-readable:

- Use clear `Write-Host` sections.
- Print commands being started.
- Print dashboard URLs.
- Fail before launching if required tool missing: `node`, package manager, `go`.
- Prefer non-destructive behavior.
- If LAN IP detection is implemented, use Windows-safe approach; otherwise print placeholder message clearly.

Suggested URL output:

```text
Dashboard local: http://localhost:3000
Dashboard LAN: http://<detected-lan-ip>:3000
Agent health: http://localhost:8080/health
```

### Environment Requirements

Minimum suggested env names:

- `SELFSTUDIO_AGENT_HOST=127.0.0.1`
- `SELFSTUDIO_AGENT_PORT=8080`
- `NEXT_PUBLIC_SELFSTUDIO_API_URL=http://localhost:8080`
- `SELFSTUDIO_LOCAL_DATA_DIR=./local-data`

Do not add real credentials. `.env.example` only.

### File Structure Requirements

Expected created/updated files include, but are not limited to:

```text
apps/web/
apps/agent/go.mod
apps/agent/cmd/selfstudio-agent/main.go
apps/agent/.env.example
docs/api/openapi.yaml
supabase/migrations/.gitkeep or initial README
scripts/dev.ps1
scripts/start.ps1
scripts/build.ps1
.env.example
local-data/input/station-1/.gitkeep
local-data/input/station-2/.gitkeep
local-data/input/station-3/.gitkeep
local-data/output/.gitkeep
local-data/quarantine/.gitkeep
local-data/logs/.gitkeep
local-data/tmp/.gitkeep
```

If create-next-app generates additional files under `apps/web`, keep them scoped there.

### Testing Requirements

Run and record outcomes where available:

- `go test ./...` from `apps/agent`.
- Next.js lint/typecheck/build command from `apps/web` if scaffold supports it.
- Manual smoke: start scripts or equivalent commands and request `GET /health`.

Do not claim success without command output. If environment lacks required tool, record exact blocker.

### Anti-Patterns to Avoid

- Do not keep building final MVP in root `src/server`; that is prototype layout.
- Do not move filesystem/camera worker logic into Next.js.
- Do not expose service role keys or cloud credentials to frontend env.
- Do not implement future stories early.
- Do not delete prototype camera spike work without approval.
- Do not make scripts depend on WSL/bash; target Windows PowerShell.

### References

- [Source: `_bmad-output/planning-artifacts/epics.md` → Story 1.1]
- [Source: `_bmad-output/planning-artifacts/architecture.md` → Starter Template Evaluation]
- [Source: `_bmad-output/planning-artifacts/architecture.md` → Project Structure & Boundaries]
- [Source: `_bmad-output/planning-artifacts/architecture.md` → API & Communication Patterns]
- [Source: `_bmad-output/planning-artifacts/architecture.md` → Infrastructure & Deployment]
- [Source: `_bmad-output/planning-artifacts/prd.md` → MVP Scope, Technical Success, Operational Constraints]

## Project Structure Notes

- Target architecture differs from existing repo spike. Implement additive monorepo structure first.
- Existing root TypeScript watcher code may later inform Go watcher behavior, but this story should not port it.
- Use `docs/api/openapi.yaml` as contract seed for next stories.

## Dev Agent Record

### Agent Model Used

GPT-5.1 Codex Max

### Debug Log References

- `cd apps/agent && go test ./...` → passed.
- `cd apps/web && npm install` → completed; npm audit reports 1 moderate and 1 high vulnerability in scaffold dependencies.
- `cd apps/web && npm run typecheck && npm run build` → initially failed because `next.config.ts` is unsupported by installed Next.js 14; fixed by using `next.config.mjs`; rerun passed.
- Manual smoke: `GET http://127.0.0.1:8080/health` returned `200` and `{ "data": { "service": "selfstudio-agent", "status": "ok" } }`.

### Completion Notes List

- Story created by BMad create-story workflow.
- Ultimate context engine analysis completed - comprehensive developer guide created.
- Created additive target monorepo skeleton without deleting existing root TypeScript camera-input spike.
- Added minimal Go agent with env-backed config, `/health` endpoint, and tests.
- Added minimal Next.js App Router dashboard under `apps/web` with isolated package config.
- Added Windows-first PowerShell scripts for dev, start, and build flows with command checks and URL output.
- Added env examples, OpenAPI health contract, Supabase migration location, and local runtime directory placeholders.

### Change Log

- 2026-05-18: Implemented Story 1.1 local app skeleton and marked ready for review.

### File List

- `.env.example`
- `apps/agent/.env.example`
- `apps/agent/cmd/selfstudio-agent/main.go`
- `apps/agent/go.mod`
- `apps/agent/internal/api/health.go`
- `apps/agent/internal/api/health_test.go`
- `apps/agent/internal/config/config.go`
- `apps/agent/internal/config/config_test.go`
- `apps/web/.env.example`
- `apps/web/next-env.d.ts`
- `apps/web/next.config.mjs`
- `apps/web/package-lock.json`
- `apps/web/package.json`
- `apps/web/src/app/globals.css`
- `apps/web/src/app/layout.tsx`
- `apps/web/src/app/page.tsx`
- `apps/web/tsconfig.json`
- `docs/api/openapi.yaml`
- `local-data/input/station-1/.gitkeep`
- `local-data/input/station-2/.gitkeep`
- `local-data/input/station-3/.gitkeep`
- `local-data/logs/.gitkeep`
- `local-data/output/.gitkeep`
- `local-data/quarantine/.gitkeep`
- `local-data/tmp/.gitkeep`
- `scripts/build.ps1`
- `scripts/dev.ps1`
- `scripts/start.ps1`
- `supabase/migrations/.gitkeep`
