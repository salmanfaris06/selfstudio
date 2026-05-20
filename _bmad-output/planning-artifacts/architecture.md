---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
lastStep: 8
status: 'complete'
completedAt: '2026-05-18'
inputDocuments:
  - "D:/_Project/selfstudio/_bmad-output/planning-artifacts/prd.md"
workflowType: 'architecture'
project_name: 'selfstudio'
user_name: 'alpharize'
date: '2026-05-18'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**
PRD mendefinisikan 63 functional requirements yang membentuk local-network operational control room untuk event photo booth multi-station. Requirement utama terbagi menjadi Camera Station Management, Session Management, Photo Ingestion and Routing, Image Processing and Local Storage, Dashboard and Operator Controls, Readiness/Health/Recovery, dan Google Drive Fulfillment.

Secara arsitektural, sistem harus memiliki backend lokal sebagai pusat koordinasi, bukan frontend-only app. Backend perlu mengelola konfigurasi 3 camera station, active session per station, folder watcher per input folder, stable JPG ingestion, metadata routing, image processing queue, quarantine workflow, session summary, activity logs, dan Google Drive upload queue.

**Non-Functional Requirements:**
PRD mendefinisikan 36 NFR yang sangat memengaruhi arsitektur:
- Processing dan upload tidak boleh memblokir dashboard/session controls.
- Original JPG harus disimpan sebelum graded/LUT processing.
- State session, photo, processing, quarantine, dan upload harus recoverable setelah restart.
- Folder watcher harus duplicate-safe dan tidak boleh memproses partial JPG.
- Local save status harus independen dari Google Drive upload status.
- Operator harus mendapat status actionable, bukan raw logs saja.
- Access MVP dibatasi ke local operational network.
- Dashboard harus memakai label teks untuk critical states, bukan warna saja.

**Scale & Complexity:**
- Primary domain: local-network full-stack web application with filesystem workers, image processing, and cloud fulfillment.
- Complexity level: medium.
- Estimated architectural components: web dashboard, local API/server, Supabase Postgres persistence, station config service, session state service, file watcher/scanner, stable file detector, ingest router, processing worker, local storage service, quarantine manager, Google Drive upload worker, health/readiness service, activity log service, recovery service.

### Technical Constraints & Dependencies

- App berjalan di PC admin yang terhubung ke 3 Sony A6000 via USB.
- Camera integration dilakukan via folder-based JPG output, bukan direct camera control sebagai source utama.
- Sistem harus memantau 3 input folder unik.
- JPG-only MVP.
- Local storage adalah source of safety; Google Cloud Storage adalah post-session cloud storage/fulfillment layer.
- Supabase Postgres digunakan sebagai database metadata operasional.
- File operations perlu temp file, atomic rename, collision-safe naming, dan transactional metadata update.
- Google Cloud Storage integration harus tahan token failure, network failure, partial upload, retry, duplicate prevention.
- Dashboard target utama Chrome/Chromium dan Edge modern di local network.
- Mobile/tablet hanya monitoring ringan, bukan prioritas MVP.

### Cross-Cutting Concerns Identified

- Deterministic routing: setiap JPG harus map ke station + active session snapshot yang benar.
- Idempotency: watcher duplicate event, retry processing, retry upload, restart recovery tidak boleh menghasilkan duplikasi.
- State machines: station/session/photo/quarantine/upload perlu status eksplisit dan transisi aman.
- Recovery: pending session/photo/upload job harus dipulihkan saat app restart.
- Operator UX: status harus jelas, action-specific, dan aman untuk event pressure.
- Data integrity: original-first save, traceability source→session→original→graded→cloud asset, no dangling metadata to missing files.
- Queue isolation: capture/local processing tidak boleh terganggu oleh cloud upload.
- Local security/privacy: dashboard local-only, customer data dan foto diperlakukan sebagai data customer.

## Starter Template Evaluation

### Primary Technology Domain

Primary domain adalah local-network full-stack web application dengan Go local service/worker, Next.js dashboard, Supabase Postgres sebagai database, dan Google Cloud Storage sebagai image storage/cloud asset destination.

### Starter Options Considered

**Option 1: Official Next.js starter + custom Go service monorepo**
- Next.js dibuat dengan official `create-next-app@latest`.
- Go service dibuat minimal dengan `go mod init`.
- Struktur monorepo: `apps/web` dan `apps/agent`.
- Cocok karena kebutuhan utama adalah local backend worker yang stabil, bukan public SaaS full-stack template.

**Option 2: Existing Next.js + Go monorepo starters**
- Beberapa starter tersedia, tetapi banyak masih baru, low adoption, atau membawa keputusan yang tidak relevan seperti generic auth/RBAC/dashboard SaaS.
- Risiko template lock-in lebih tinggi dibanding official starter + clean custom service.

**Option 3: Electron/Tauri desktop shell**
- Memberi desktop app feel, tetapi menambah complexity packaging dan runtime.
- Tidak diperlukan untuk MVP karena PRD menargetkan local-network web dashboard.
- Bisa dipertimbangkan post-MVP sebagai shell untuk operator convenience.

### Selected Starter: Official Next.js + Custom Go Service Monorepo

**Rationale for Selection:**
Starter ini paling sesuai dengan risiko utama produk: reliability pipeline lokal. Go service dapat mengelola filesystem watcher, stable JPG detection, image processing queue, upload queue, recovery, dan Windows-local operations. Next.js dashboard tetap fokus pada UI operator, realtime station status, forms, session controls, dan troubleshooting views.

Supabase Postgres digunakan sebagai database terpusat untuk metadata operasional: stations, sessions, photos, processing jobs, quarantine, upload jobs, activity logs, and config snapshots. Google Cloud Storage digunakan untuk cloud image storage/delivery assets, sedangkan local filesystem tetap menjadi safety source saat event berjalan.

**Initialization Command:**

```bash
mkdir selfstudio
cd selfstudio
mkdir apps
npx create-next-app@latest apps/web --ts --tailwind --eslint --app --src-dir --import-alias "@/*"
mkdir apps/agent
cd apps/agent
go mod init selfstudio/agent
```

**Architectural Decisions Provided by Starter:**

**Language & Runtime:**
- Frontend: TypeScript + React + Next.js App Router.
- Backend/local worker: Go.
- Target runtime: Windows admin PC.
- Browser target: Chrome/Chromium and Edge on local network.

**Styling Solution:**
- Tailwind CSS from official Next.js setup.
- shadcn/ui recommended for dashboard primitives: cards, dialogs, badges, tables, forms, toasts.

**Build Tooling:**
- Next.js build/dev server for web dashboard.
- Go build for Windows local service executable.
- Separate processes for UI and service to isolate worker reliability from frontend runtime.

**Testing Framework:**
- Next.js: add Playwright for operator workflow E2E later.
- Go: built-in `testing` package for state machines, routing rules, watcher idempotency, and queue logic.

**Code Organization:**
- `apps/web`: dashboard, station UI, session forms, status views.
- `apps/agent`: local API, watcher, scanner, processing workers, Google Drive upload worker, Supabase DB adapter.
- Shared API contract documented via OpenAPI or typed endpoint definitions.

**Development Experience:**
- Next.js hot reload for dashboard UI.
- Go fast compile/test loop for local worker.
- Supabase migrations for database schema.
- Windows-first local scripts for starting both processes.

**Note:** Project initialization using this command should be first implementation story.

## Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions (Block Implementation):**
- Data source of record: Supabase Postgres for metadata; local filesystem for event-time photo safety.
- MVP auth: local PIN/password gate in Go service.
- API pattern: REST commands + SSE realtime updates.
- Frontend architecture: Next.js App Router + TanStack Query + shadcn/ui + Tailwind.
- MVP packaging: separate Node/Next.js process and Go process launched via Windows scripts.

**Important Decisions (Shape Architecture):**
- Go service owns all filesystem access, watcher/scanner, ingest routing, processing queue, upload queue, and recovery.
- Next.js never reads camera folders directly and never receives service credentials.
- Supabase migrations manage database schema.
- Google Cloud Storage stores cloud image assets after local safety write.
- OpenAPI documents Go API contract.

**Deferred Decisions (Post-MVP):**
- Electron/Tauri desktop shell deferred until local web MVP proves reliable.
- Single-port bundled packaging deferred until MVP process boundaries stabilize.
- Advanced RBAC deferred; MVP uses simple local operator gate.
- WebSocket deferred; SSE is enough for dashboard push status unless bidirectional realtime becomes necessary.

### Data Architecture

**Decision:** Use Supabase Postgres as metadata system of record; use local filesystem as event-time photo safety source.

**Verified Versions/Tools:**
- Supabase CLI: stable `v2.98.1` observed; beta `v2.99.0-beta.*` avoided for MVP.
- Supabase local development uses CLI migrations and Docker-compatible local stack.
- Go runtime: `1.26.3`.
- Go database driver: `pgx` recommended for direct Postgres access.

**Rationale:**
Supabase Postgres gives durable metadata, migration workflow, and future remote visibility. Local filesystem remains source of safety because capture events cannot depend on internet/cloud availability. Every photo must be saved locally before processing/upload. Go service performs transactional DB writes for stations, sessions, photos, processing jobs, quarantine, upload jobs, logs, and config snapshots.

**Primary Tables:**
- `stations`
- `station_configs`
- `sessions`
- `photos`
- `processing_jobs`
- `quarantine_items`
- `upload_jobs`
- `activity_logs`
- `app_settings`

**Migration Approach:**
Use Supabase CLI migrations:
```bash
supabase init
supabase migration new create_core_schema
supabase db reset
supabase db push
```

### Authentication & Security

**Decision:** MVP uses local PIN/password gate implemented in Go service.

**Rationale:**
PRD requires local-network restriction, not complex RBAC. Local PIN/password gate protects operator dashboard without adding Supabase Auth friction during event operations. This keeps system usable even if Supabase Auth/cloud session is unavailable. Supabase Auth can be added later if remote/admin user management becomes required.

**Security Rules:**
- Dashboard exposed only on local network or localhost by default.
- Go service validates local auth token/session for API commands.
- Service credentials for Supabase and Google Cloud stay server-side in Go config/environment.
- Browser never receives Supabase service role keys or Google Cloud credentials.
- Activity logs track sensitive operator actions: start session, end session, config change, retry, upload action.

### API & Communication Patterns

**Decision:** Go service exposes REST API for commands and SSE for dashboard realtime updates.

**Rationale:**
REST is simple and reliable for state-changing commands like create session, end session, recheck station, retry processing, assign quarantine, and retry upload. SSE is sufficient for one-way dashboard updates: station status, timer ticks, photo processed, queue status, upload status, alerts. WebSocket remains deferred unless bidirectional realtime control becomes necessary.

**API Standards:**
- REST endpoints under `/api`.
- SSE stream under `/events`.
- OpenAPI contract generated/maintained for Next.js integration.
- All mutations handled by Go service.
- API errors use operator-actionable shape:

```json
{
  "error": {
    "code": "STATION_NOT_READY",
    "message": "Station input folder is not readable",
    "action": "RECHECK_STATION"
  }
}
```

### Frontend Architecture

**Decision:** Next.js App Router with TanStack Query, SSE-driven updates, shadcn/ui, and Tailwind.

**Verified Versions/Tools:**
- Next.js latest npm observed: `16.2.6`.
- TanStack Query latest observed: `5.100.10`.
- shadcn CLI v4 current docs via `npx shadcn@latest`.

**Rationale:**
Next.js provides a modern React foundation for dashboard UI. TanStack Query handles server-state caching and mutation lifecycles without turning backend state into fragile global frontend state. SSE pushes authoritative Go service updates to dashboard. shadcn/ui and Tailwind provide fast, accessible dashboard components for cards, dialogs, badges, tables, forms, alerts, and toasts.

**Frontend Rules:**
- Use Client Components for realtime station dashboard areas.
- Use Server Components where data is static or initial-load only.
- TanStack Query stores server state.
- Zustand allowed only for UI-only state such as selected station, open panel, local filters.
- No direct filesystem access from Next.js.
- No direct privileged cloud/database credentials in browser.

### Infrastructure & Deployment

**Decision:** MVP uses separate Node/Next.js process and Go process launched via Windows scripts.

**Rationale:**
Separate processes keep implementation simple and make worker reliability easier to debug. Go owns local operations and long-running workers. Next.js owns dashboard development/build runtime. One-click/single-port packaging can be added later once MVP behavior stabilizes.

**Windows MVP Layout:**
- `scripts/dev.ps1`: starts Next.js dev server and Go service.
- `scripts/start.ps1`: starts production-like Next.js and Go service.
- Go service exposes `/health`.
- Local structured JSON logs written to disk.
- Operational activity logs written to Supabase Postgres.
- Google Drive upload worker runs independently from capture/processing flow.

### Decision Impact Analysis

**Implementation Sequence:**
1. Initialize monorepo and toolchain.
2. Create Supabase schema migrations.
3. Build Go service skeleton with config, health, local auth, REST, SSE.
4. Build station config and readiness services.
5. Build watcher/scanner/stable file detection.
6. Build session state and routing.
7. Build local original-first storage and processing job model.
8. Build Next.js dashboard with TanStack Query and SSE updates.
9. Build quarantine and retry workflows.
10. Build Google Cloud Storage upload queue.
11. Build Windows scripts and pilot packaging.

**Cross-Component Dependencies:**
- Next.js depends on Go API contract and SSE event schema.
- Go service depends on Supabase schema and environment config.
- Processing and upload jobs depend on photo/session metadata and local file paths.
- Google Drive upload depends on completed local session assets.
- Operator dashboard depends on activity logs, station state, and queue summaries.

## Implementation Patterns & Consistency Rules

### Pattern Categories Defined

**Critical Conflict Points Identified:**
15+ area rawan konflik: database naming, API naming, JSON field format, error shape, SSE events, Go package structure, Next.js component layout, test placement, validation, logging, retry semantics, idempotency keys, date/time format, local path handling, and auth/session handling.

### Naming Patterns

**Database Naming Conventions:**
- Tables use plural `snake_case`: `stations`, `station_configs`, `processing_jobs`.
- Columns use `snake_case`: `station_id`, `created_at`, `local_original_path`.
- Primary keys use `id uuid primary key`.
- Foreign keys use `{table_singular}_id`: `station_id`, `session_id`, `photo_id`.
- Indexes use `idx_{table}_{columns}`: `idx_photos_session_id`, `idx_upload_jobs_status`.
- Unique constraints use `uq_{table}_{columns}`.
- Check constraints use `chk_{table}_{rule}`.

**API Naming Conventions:**
- REST endpoints use plural kebab-case nouns:
  - `GET /api/stations`
  - `POST /api/stations/{station_id}/recheck`
  - `POST /api/sessions`
  - `POST /api/photos/{photo_id}/retry-processing`
- Path params use snake_case inside OpenAPI: `{station_id}`.
- Query params use snake_case: `station_id`, `session_id`, `status`.
- Headers use standard form: `Authorization`, `Content-Type`, `X-Request-ID`.

**Code Naming Conventions:**
- Go packages use short lowercase names: `api`, `config`, `watcher`, `ingest`, `processing`, `upload`, `store`.
- Go exported identifiers use PascalCase: `CreateSession`, `ProcessingJob`.
- Go internal variables use camelCase: `stationID`, `sessionID`.
- TypeScript variables/functions use camelCase: `stationId`, `createSession`.
- React components use PascalCase: `StationCard`.
- React component files use kebab-case: `station-card.tsx`.
- Go files use snake_case: `session_service.go`.

### Structure Patterns

**Project Organization:**
- `apps/web` contains all Next.js code.
- `apps/agent` contains all Go service code.
- `supabase/migrations` contains all schema migrations.
- `scripts` contains Windows PowerShell launcher scripts.
- `docs/api` contains OpenAPI specs.
- `docs/architecture` contains supplementary architecture diagrams if needed.

**Next.js Structure:**
- Route groups under `apps/web/src/app`.
- Shared UI components under `apps/web/src/components/ui`.
- Feature components under `apps/web/src/features/{feature}`.
- API clients under `apps/web/src/lib/api`.
- SSE client under `apps/web/src/lib/events`.
- Types generated or shared from OpenAPI under `apps/web/src/types/api.ts`.

**Go Structure:**
- Entrypoint: `apps/agent/cmd/selfstudio-agent/main.go`.
- Internal packages under `apps/agent/internal`.
- HTTP API handlers: `internal/api`.
- Database access: `internal/store`.
- Business services: `internal/service`.
- Watcher/scanner: `internal/watcher`.
- Stable file detection: `internal/ingest`.
- Image processing queue: `internal/processing`.
- Google Drive upload queue: `internal/upload`.
- Config loading: `internal/config`.
- Structured logging: `internal/logging`.

**Test Placement:**
- Go tests co-located as `*_test.go`.
- Next.js component tests co-located as `*.test.tsx` only when needed.
- Playwright E2E tests under `apps/web/e2e`.
- State machine/idempotency tests must be in Go package tests near logic.

### Format Patterns

**API Response Formats:**
- Success responses use wrapper:
```json
{
  "data": {}
}
```
- List responses use:
```json
{
  "data": [],
  "meta": {
    "total": 0
  }
}
```
- Error responses use:
```json
{
  "error": {
    "code": "STATION_NOT_READY",
    "message": "Station input folder is not readable",
    "action": "RECHECK_STATION",
    "details": {}
  }
}
```

**Data Exchange Formats:**
- JSON fields use `snake_case` to match DB/API: `station_id`, `created_at`.
- TypeScript API client maps API fields explicitly; UI may use generated types directly.
- Dates use ISO 8601 UTC strings: `2026-05-18T10:30:00Z`.
- Status values use lowercase kebab-case or snake_case consistently:
  - Session: `active`, `ending`, `locked`, `local_complete`, `upload_pending`, `uploaded`, `failed`
  - Photo: `detected`, `stabilizing`, `saved_original`, `processing`, `processed`, `failed`, `quarantined`
  - Upload: `pending`, `uploading`, `uploaded`, `failed`

### Communication Patterns

**Event System Patterns:**
- SSE event names use dot notation:
  - `station.updated`
  - `session.updated`
  - `photo.created`
  - `photo.processed`
  - `queue.updated`
  - `upload.updated`
  - `alert.created`
- SSE payloads use wrapper:
```json
{
  "event_id": "uuid",
  "event_type": "station.updated",
  "occurred_at": "2026-05-18T10:30:00Z",
  "data": {}
}
```
- Every event has `event_id`, `event_type`, `occurred_at`, `data`.
- Events are notification/update hints; Go service + DB remain source of truth.

**State Management Patterns:**
- Go service owns authoritative station/session/photo/upload state.
- TanStack Query owns server state in frontend.
- SSE updates either patch query cache or invalidate relevant queries.
- Zustand allowed only for UI-only state.
- No duplicated workflow state machines in frontend.

### Process Patterns

**Error Handling Patterns:**
- Go service logs technical detail; API returns operator-safe message and action.
- UI shows `message` and renders action CTA from `action`.
- Retriable errors must include retry action: `RETRY_PROCESSING`, `RETRY_UPLOAD`, `RECHECK_STATION`.
- Non-retriable errors must explain required operator action.

**Loading State Patterns:**
- Mutations show button-level loading.
- Dashboard cards keep last known state while refresh/SSE reconnect occurs.
- Global loading only for first page load.
- Offline/disconnected state shown as banner, not full-screen blocker.

**Retry and Idempotency Patterns:**
- Ingest idempotency key: station ID + source file path + file size + modified timestamp.
- Processing retry uses original saved path, never source path only.
- Upload retry uses local asset ID + target Google Drive folder/file identity.
- All retry jobs increment `attempt_count` and store `last_error`.

**Logging Patterns:**
- Technical logs: structured JSON local files.
- Operator activity logs: persisted in `activity_logs`.
- Log levels: `debug`, `info`, `warn`, `error`.
- Every command request includes or generates `request_id`.

### Enforcement Guidelines

**All AI Agents MUST:**
- Use `snake_case` for DB, API JSON, and status fields.
- Keep all filesystem, processing, upload, and credential logic inside Go service.
- Never expose Supabase service role or Google Cloud credentials to Next.js/browser.
- Use REST mutations and SSE events, not ad-hoc polling-only dashboard logic.
- Preserve original JPG before any processing.
- Use idempotency keys for watcher, processing retry, and upload retry.
- Return operator-actionable error responses.
- Add/update tests for state transitions and idempotency logic.

**Pattern Enforcement:**
- OpenAPI schema is source of truth for API contract.
- Supabase migrations are source of truth for DB schema.
- Go tests validate state transitions, idempotency, and retry behavior.
- Playwright validates critical operator workflows after UI exists.
- Pattern changes must update this architecture document before implementation.

### Pattern Examples

**Good Examples:**
- `POST /api/sessions/{session_id}/end`
- `processing_jobs.attempt_count`
- SSE event `photo.processed`
- Error action `RETRY_UPLOAD`
- React file `station-card.tsx`
- Go service file `session_service.go`

**Anti-Patterns:**
- Browser reads local camera folder directly.
- Next.js stores Supabase service role key.
- API returns raw Go/internal errors to operator.
- Upload job starts before local session completion.
- Retry creates duplicate photo/upload records.
- Frontend invents its own session state separate from Go service.

## Project Structure & Boundaries

### Complete Project Directory Structure

```text
selfstudio/
├── README.md
├── .gitignore
├── .env.example
├── package.json
├── pnpm-workspace.yaml
├── turbo.json
├── scripts/
│   ├── dev.ps1
│   ├── start.ps1
│   ├── build.ps1
│   └── reset-local.ps1
├── docs/
│   ├── api/openapi.yaml
│   ├── architecture/state-machines.md
│   ├── architecture/data-flow.md
│   ├── architecture/event-contracts.md
│   └── operations/windows-setup.md
├── supabase/
│   ├── config.toml
│   ├── seed.sql
│   └── migrations/000001_create_core_schema.sql
├── apps/
│   ├── web/
│   │   ├── package.json
│   │   ├── next.config.ts
│   │   ├── tsconfig.json
│   │   ├── eslint.config.mjs
│   │   ├── components.json
│   │   ├── postcss.config.mjs
│   │   ├── public/app-icon.svg
│   │   ├── e2e/dashboard.spec.ts
│   │   ├── e2e/session-flow.spec.ts
│   │   ├── e2e/quarantine-flow.spec.ts
│   │   └── src/
│   │       ├── app/
│   │       ├── components/
│   │       ├── features/
│   │       ├── lib/
│   │       └── types/
│   └── agent/
│       ├── go.mod
│       ├── go.sum
│       ├── .env.example
│       ├── cmd/selfstudio-agent/main.go
│       ├── internal/
│       │   ├── api/
│       │   ├── auth/
│       │   ├── config/
│       │   ├── domain/
│       │   ├── events/
│       │   ├── ingest/
│       │   ├── logging/
│       │   ├── processing/
│       │   ├── service/
│       │   ├── store/
│       │   ├── upload/
│       │   └── watcher/
│       └── testdata/
└── local-data/
    ├── input/station-1/
    ├── input/station-2/
    ├── input/station-3/
    ├── output/
    ├── quarantine/
    ├── logs/
    └── tmp/
```

### Architectural Boundaries

**API Boundaries:**
- Browser talks only to Go service API.
- Go service owns all mutations.
- Next.js never talks directly to Supabase for operational metadata in MVP.
- Go service is only process allowed to use Supabase service credentials and Google Cloud credentials.
- External boundaries: Supabase Postgres via `pgx`, Google Cloud Storage via Go client, Windows filesystem via Go service only.

**Component Boundaries:**
- `apps/web/src/features/*` owns UI feature components only.
- `apps/web/src/lib/api` owns REST calls.
- `apps/web/src/lib/events` owns SSE connection and event routing.
- `apps/agent/internal/service` owns business rules.
- `apps/agent/internal/store` owns DB read/write.
- `apps/agent/internal/watcher` detects filesystem changes but does not route business logic.
- `apps/agent/internal/ingest` owns stable file and session routing decisions.
- `apps/agent/internal/processing` owns original save and LUT processing.
- `apps/agent/internal/upload` owns Google Drive upload lifecycle.

### Requirements to Structure Mapping

- Camera Station Management (FR1-FR11): `features/stations`, `station_handler.go`, `station_service.go`, `stations`, `station_configs`.
- Session Management (FR12-FR20): `features/sessions`, `session_handler.go`, `session_service.go`, `sessions`.
- Photo Ingestion and Routing (FR21-FR30): `internal/watcher`, `internal/ingest`, `quarantine_service.go`, `photos`, `quarantine_items`.
- Image Processing and Local Storage (FR31-FR39): `internal/processing`, `features/photos`, `photos`, `processing_jobs`.
- Dashboard and Operator Controls (FR40-FR48): `features/dashboard`, `features/uploads`, `features/logs`, `internal/events`.
- Readiness, Health, and Recovery (FR49-FR55): `readiness_service.go`, `recovery_service.go`, `health_handler.go`, `activity_logs`.
- Google Cloud Storage Fulfillment (FR56-FR63): `internal/upload`, `upload_handler.go`, `features/uploads`, `upload_jobs`.

### Integration Points

**Internal Communication:**
- Go service publishes domain events to in-memory event broker.
- SSE streams event broker updates to browser.
- Frontend reacts by patching/invalidating TanStack Query cache.
- Go workers coordinate through DB job rows and service layer.

**External Integrations:**
- Supabase Postgres: metadata/state system of record.
- Google Cloud Storage: post-session original/graded cloud objects.
- Windows filesystem: camera input folders, local output, quarantine, tmp, logs.
- Optional LUT tool/library: called only from `internal/processing`.

**Data Flow:**
1. Camera/tether software writes JPG to station input folder.
2. Watcher/scanner detects candidate file.
3. Stable file detector waits until file complete.
4. Ingest router creates idempotent photo record.
5. Session service determines active session or quarantine.
6. Processing saves original locally first.
7. LUT processor creates graded output.
8. Events update dashboard.
9. Session ends and local completion is verified.
10. Upload worker uploads original/graded assets to Google Drive.
11. Upload status updates DB and dashboard.

### File Organization Patterns

**Configuration Files:**
- Root `.env.example` documents shared environment variables.
- `apps/agent/.env.example` documents Go service credentials/config.
- `apps/web/.env.local` stores frontend public local API URL only.
- `supabase/config.toml` manages local Supabase config.
- Runtime machine config can live under local app data path later; not committed.

**Source Organization:**
- Frontend organized by feature.
- Go organized by responsibility package.
- Domain types separated in `internal/domain`.
- Business logic must not live inside API handlers.

**Test Organization:**
- Go unit tests co-located.
- Go integration tests may use `testdata`.
- Playwright E2E tests under `apps/web/e2e`.
- Sample JPG/LUT fixtures under `apps/agent/testdata`.

**Asset Organization:**
- Static UI assets in `apps/web/public`.
- Runtime photo assets in `local-data`, never `public`.
- Quarantined assets in `local-data/quarantine`.
- Temp processing files in `local-data/tmp`.

### Development Workflow Integration

**Development Server Structure:**
- `scripts/dev.ps1` starts Supabase local stack if needed, Go service, and Next.js dev server.
- Next.js dev server proxies or points API calls to Go service.
- Go service loads `.env` and exposes `/health`, `/api`, `/events`.

**Build Process Structure:**
- `scripts/build.ps1` builds Next.js app and Go Windows executable.
- Go build output goes to `dist/agent/selfstudio-agent.exe`.
- Next.js build output remains under `apps/web/.next`.

**Deployment Structure:**
- MVP pilot runs separate processes via `scripts/start.ps1`.
- Local data directories created/validated on startup.
- Google Drive/Supabase credentials configured on admin PC, not committed.

## Architecture Validation Results

### Coherence Validation ✅

**Decision Compatibility:**
Semua keputusan utama kompatibel: Go service owning local operations, Next.js as dashboard-only UI, Supabase Postgres as metadata system of record, local filesystem as event safety layer, Google Drive as post-session cloud delivery, REST + SSE as command/realtime split, and Windows-first scripts/processes.

**Pattern Consistency:**
Patterns align across database, API, events, frontend, and Go service. `snake_case` is used for DB/API/status; Go and TypeScript follow language conventions internally; error, SSE, retry, idempotency, and logging patterns all support reliability goals.

**Structure Alignment:**
Project structure supports all boundaries: `apps/agent` owns filesystem/workers/credentials, `apps/web` owns UI, `supabase/migrations` owns schema, `docs/api/openapi.yaml` owns contract, and `local-data` separates runtime files from source/public assets.

### Requirements Coverage Validation ✅

**Feature Coverage:**
All FR categories are mapped to modules: camera station management, session management, photo ingestion/routing, image processing/local storage, dashboard controls, readiness/health/recovery, and Google Drive fulfillment.

**Functional Requirements Coverage:**
All 63 FRs are architecturally supported by defined UI features, Go services, API handlers, schema groups, and event/worker patterns.

**Non-Functional Requirements Coverage:**
NFRs are supported by queue isolation, Go workers, SSE updates, original-first save, stable detection, idempotency, recovery, traceability, transaction-safe metadata, credential boundaries, operator-actionable errors, readiness checks, local logs, and health endpoints.

### Implementation Readiness Validation ✅

**Decision Completeness:**
Critical decisions are documented with versions where relevant: Next.js `16.2.6`, Go `1.26.3`, TanStack Query `5.100.10`, Supabase CLI stable observed `v2.98.1`, and shadcn CLI v4 via `npx shadcn@latest`.

**Structure Completeness:**
Structure is implementation-ready for first stories: monorepo, web/agent boundaries, data/API/event/worker/test locations, and requirements-to-directory mapping are defined.

**Pattern Completeness:**
Naming, API shape, SSE shape, error shape, retry/idempotency, logging, state ownership, test placement, and credential boundaries are specified.

### Gap Analysis Results

**Critical Gaps:** None.

**Important Gaps:**
- Exact LUT processing library/tool not selected yet; architecture isolates it behind `internal/processing/lut_processor.go`.
- Exact Google Drive folder naming convention should be finalized before upload story; architecture isolates it in `internal/upload/drive_file_ids.go`.
- Full offline behavior with Supabase unavailable needs explicit product decision if required. Current architecture uses Supabase Postgres as metadata source of record and local filesystem as photo safety layer.

**Nice-to-Have Gaps:**
- Add architecture diagrams in `docs/architecture/data-flow.md`.
- Add state machine docs in `docs/architecture/state-machines.md`.
- Add OpenAPI generation tooling decision.
- Add Windows service installer later.

### Validation Issues Addressed

No critical issues found. Important gaps are intentionally deferred because they do not block architecture. LUT processor is hidden behind interface, Google Drive folder/file identity format is isolated, and offline DB fallback is out unless full offline operation becomes a requirement.

### Architecture Completeness Checklist

**Requirements Analysis**
- [x] Project context thoroughly analyzed
- [x] Scale and complexity assessed
- [x] Technical constraints identified
- [x] Cross-cutting concerns mapped

**Architectural Decisions**
- [x] Critical decisions documented with versions
- [x] Technology stack fully specified
- [x] Integration patterns defined
- [x] Performance considerations addressed

**Implementation Patterns**
- [x] Naming conventions established
- [x] Structure patterns defined
- [x] Communication patterns specified
- [x] Process patterns documented

**Project Structure**
- [x] Complete directory structure defined
- [x] Component boundaries established
- [x] Integration points mapped
- [x] Requirements to structure mapping complete

### Architecture Readiness Assessment

**Overall Status:** READY FOR IMPLEMENTATION

**Confidence Level:** high

**Key Strengths:**
- Strong separation between local operational worker and UI.
- Reliability risks directly addressed: stable detection, idempotency, original-first save, retry, recovery.
- Credentials remain server-side.
- Dashboard receives realtime updates without owning workflow state.
- Project structure maps cleanly to all FR categories.

**Areas for Future Enhancement:**
- Electron/Tauri shell for desktop convenience.
- Single-port bundled runtime.
- Supabase Auth/RBAC if remote admin access becomes needed.
- Offline local DB fallback if event must run without internet/Supabase.
- Detailed Google Drive file naming and retention policies.

### Implementation Handoff

**AI Agent Guidelines:**
- Follow all architectural decisions exactly as documented.
- Use implementation patterns consistently across all components.
- Respect project structure and boundaries.
- Refer to this document for all architectural questions.
- Never move filesystem/credential/worker logic into Next.js.
- Preserve original JPG before processing or upload.

**First Implementation Priority:**
Initialize monorepo and toolchain:

```bash
mkdir apps
npx create-next-app@latest apps/web --ts --tailwind --eslint --app --src-dir --import-alias "@/*"
mkdir apps/agent
cd apps/agent
go mod init selfstudio/agent
```
