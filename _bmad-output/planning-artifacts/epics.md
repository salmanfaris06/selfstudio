---
stepsCompleted: [1, 2, 3, 4]
lastStep: 4
status: 'complete'
completedAt: '2026-05-18'
inputDocuments:
  - "D:/_Project/selfstudio/_bmad-output/planning-artifacts/prd.md"
  - "D:/_Project/selfstudio/_bmad-output/planning-artifacts/architecture.md"
workflowType: 'epics-and-stories'
project_name: 'selfstudio'
user_name: 'alpharize'
date: '2026-05-18'
---

# selfstudio - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for selfstudio, decomposing the requirements from the PRD and Architecture requirements into implementable stories.

## Requirements Inventory

### Functional Requirements

FR1: Admin can create and manage three logical camera stations.
FR2: Admin can assign a physical camera/device identifier to each camera station.
FR3: Admin can assign a unique input folder to each camera station.
FR4: Admin can assign a background name to each camera station.
FR5: Admin can assign a default LUT to each camera station.
FR6: Admin can configure deterministic output folder rules for session results.
FR7: Admin can view connection/readiness status for each camera station.
FR8: Admin can run test capture/test watch validation for each camera station.
FR9: Admin can refresh/reconnect/recheck a camera station when it has a device or folder issue.
FR10: System prevents invalid station configuration such as duplicate input folders or missing required paths.
FR11: System can backup and restore camera station configuration.
FR12: Admin/operator can create a session for a specific camera station.
FR13: Admin/operator can enter customer name, order number, and timer duration when creating a session.
FR14: System allows only one active session per camera station.
FR15: System tracks session state across active, ending, locked, local complete, upload pending, uploaded, and failed states.
FR16: Operator can end a session manually from the admin UI.
FR17: System can end a session automatically when the timer expires.
FR18: System locks a session after it ends so new photos no longer enter that session.
FR19: System records session summary including photo counts, failures, quarantine count, duration, local output path, and Drive upload status.
FR20: System can recover session state after application restart.
FR21: System can monitor each camera station input folder for new JPG files.
FR22: System can identify which camera station produced a detected JPG.
FR23: System can wait until a detected JPG is stable before processing it.
FR24: System can route a detected JPG to the active session for its camera station.
FR25: System sends JPGs with no eligible active session to quarantine/unassigned.
FR26: System sends JPGs detected after session lock to quarantine/unassigned.
FR27: System records a quarantine reason for each quarantined photo.
FR28: Admin/operator can review quarantined photos.
FR29: Admin/operator can manually assign a quarantined photo to an appropriate session.
FR30: System prevents duplicate processing of the same detected JPG.
FR31: System can save the original JPG for each valid session photo.
FR32: System can apply the station/session LUT to create a graded JPG.
FR33: System can save original and graded JPGs to deterministic customer/order/station folders.
FR34: System can preserve the original JPG even when LUT processing fails.
FR35: System can track processing status for each photo.
FR36: System can automatically retry failed photo processing up to three times.
FR37: Admin/operator can manually retry failed photo processing.
FR38: System can prevent filename collisions in local output folders.
FR39: System can identify missing or invalid LUT conditions and mark affected processing jobs as failed/retryable.
FR40: Operator can view a dashboard with three camera station cards.
FR41: Operator can see the active customer, order number, timer, station status, LUT, photo count, and last photo preview for each station.
FR42: Operator can see primary station states such as READY, LIVE, ATTENTION, LOCKED, and UPLOAD status.
FR43: Operator can see actionable alerts for camera, folder, disk, processing, quarantine, and upload problems.
FR44: Operator can access station/session detail views for deeper troubleshooting.
FR45: Operator can open the local result folder for a completed session.
FR46: Operator can view local save status separately from Google Drive upload status.
FR47: Operator can view processing queue and upload queue status.
FR48: Operator can view activity logs filtered by station/session.
FR49: System can run an event readiness checklist before sessions start, covering input folders readable, output folders writable, LUT files present/valid, disk threshold met, watcher running, processor ready, application data/state health, quarantine folder writable, and Drive status known.
FR50: System can block session start when required readiness checks fail.
FR51: System can show disk space warnings and prevent unsafe operation when storage falls below configurable warning/block thresholds.
FR52: System can show application health indicators for watcher, processor, application data/state health, disk, and Drive connectivity.
FR53: System can record operator actions such as start session, end session, reconnect, retry, config change, and Drive upload.
FR54: System can recover pending photo processing jobs after application restart.
FR55: System can recover pending Google Drive upload jobs after application restart.
FR56: Admin can connect one Google Drive admin account for cloud upload.
FR57: System can create Google Drive folders based on customer name and order number.
FR58: System can upload original and graded JPGs to Google Drive after local session completion.
FR59: System can track Google Drive upload status per session and per file.
FR60: System can retry failed Google Drive uploads.
FR61: System preserves local files when Google Drive upload fails.
FR62: System ensures Google Drive upload does not block new capture sessions.
FR63: System can prevent duplicate Drive uploads for the same session asset during retry or restart using tracked upload identity and status.

### NonFunctional Requirements

NFR1: Dashboard station status should update near real-time enough for event operation.
NFR2: New photos should appear in the relevant station flow after file stabilization and processing without blocking other stations.
NFR3: Image processing must not block dashboard interaction or session controls during normal event operation.
NFR4: Google Drive upload must not degrade capture, local save, session routing, or LUT processing performance.
NFR5: System must support 3 active camera stations during at least a 2-hour simulation with at least 300 total JPG files.
NFR6: Folder watcher must handle duplicate file system events without duplicate processing.
NFR7: The system must never treat a partial/incomplete JPG as successfully processed.
NFR8: The system must preserve original JPG before attempting graded/LUT output.
NFR9: The system must maintain recoverable session, photo, processing, quarantine, and upload state after application restart.
NFR10: Timer end, admin end, and photo ingestion boundary cases must resolve deterministically.
NFR11: Local save success must be independent of Google Drive upload success.
NFR12: System must surface failed watcher, failed processing, missing LUT, low disk, missing folder, and Drive upload failure states to the operator.
NFR13: System must prevent invalid readiness state from being shown as READY.
NFR14: Each photo record must maintain complete traceability from source folder to session, local original, graded output, quarantine state if any, and Drive upload status.
NFR15: Local output filenames must be collision-safe.
NFR16: Processing and upload retries must be duplicate-safe.
NFR17: Session config must be snapshotted so later config changes do not alter historical routing interpretation.
NFR18: Persisted application records must not point to missing final local files after save, processing, retry, or restart scenarios.
NFR19: Dashboard access should be limited to the local operational network for MVP.
NFR20: Google Drive authentication must use an authorized admin account.
NFR21: Stored customer names, order numbers, and photo files must be treated as customer data.
NFR22: The system should avoid exposing dashboard access to public internet in MVP.
NFR23: Activity logs should not expose unnecessary sensitive data beyond operational troubleshooting needs.
NFR24: Camera integration depends on reliable JPG output to configured input folders.
NFR25: Google Drive integration must support token failure, network failure, partial upload, and retry scenarios.
NFR26: Google Drive upload must track remote folder/file status enough to reduce duplicate uploads.
NFR27: LUT processing must tolerate missing/invalid LUT by failing the graded job while preserving original.
NFR28: Station status must be understandable by operators under event pressure.
NFR29: Critical states must use text labels in addition to color.
NFR30: Critical actions such as End Session must require confirmation.
NFR31: Error actions must be specific, such as Retry Photo Processing, Retry Drive Upload, Restart Folder Watcher, or Recheck Camera.
NFR32: Dashboard must clearly separate local save status from Google Drive upload status.
NFR33: The admin PC should remain awake and connected during event operation.
NFR34: The system should show the local network URL needed to access the dashboard from other devices.
NFR35: Disk space must be checked before allowing new sessions when below safe thresholds.
NFR36: Logs should be timestamped and filterable by station/session for troubleshooting.

### Additional Requirements

- Starter/greenfield foundation must use official Next.js starter plus custom Go service monorepo: `apps/web` and `apps/agent`.
- Frontend stack: Next.js App Router, TypeScript, Tailwind, shadcn/ui, TanStack Query, SSE-driven dashboard updates.
- Backend/local worker stack: Go service on Windows admin PC, owning filesystem watcher/scanner, stable JPG detection, ingest routing, processing queue, upload queue, local PIN auth, REST API, SSE, recovery, and credential handling.
- Database: Supabase Postgres is metadata system of record using Supabase CLI migrations; local filesystem remains event-time photo safety source.
- Cloud storage: Google Cloud Storage stores post-session cloud image assets; local files must never be deleted due to cloud upload failure.
- API pattern: REST command endpoints under `/api`, SSE updates under `/events`, OpenAPI contract in `docs/api/openapi.yaml`.
- Security: MVP local PIN/password gate in Go service; browser must never receive Supabase service role key or Google Cloud credentials.
- Naming/pattern rules: DB/API JSON/status use `snake_case`; API responses use `{data}` wrapper and `{error:{code,message,action,details}}` errors; SSE events use dot notation and event wrapper.
- State ownership: Go service owns authoritative station/session/photo/quarantine/upload state; frontend may cache via TanStack Query but must not invent workflow state machines.
- Project structure and boundaries from `architecture.md` must be followed, especially Go-only filesystem/worker/credential boundaries.
- Important implementation gaps to resolve in stories: exact LUT processor library/tool, Google Drive folder naming convention, and explicit behavior when Supabase is unavailable.

### UX Design Requirements

Tidak ada UX Design Specification terpisah ditemukan. UI/UX requirements berasal dari PRD dan Architecture: 3 station cards, actionable alerts, status text labels, confirmation for critical actions, readiness checklist, quarantine review, session summary, logs, loading/error handling, and local-vs-cloud status separation.

### FR Coverage Map

### FR Coverage Map

FR1: Epic 2 - Camera station setup, validation, readiness, and config safety
FR2: Epic 2 - Camera station setup, validation, readiness, and config safety
FR3: Epic 2 - Camera station setup, validation, readiness, and config safety
FR4: Epic 2 - Camera station setup, validation, readiness, and config safety
FR5: Epic 2 - Camera station setup, validation, readiness, and config safety
FR6: Epic 2 - Camera station setup, validation, readiness, and config safety
FR7: Epic 2 - Camera station setup, validation, readiness, and config safety
FR8: Epic 2 - Camera station setup, validation, readiness, and config safety
FR9: Epic 2 - Camera station setup, validation, readiness, and config safety
FR10: Epic 2 - Camera station setup, validation, readiness, and config safety
FR11: Epic 2 - Camera station setup, validation, readiness, and config safety
FR12: Epic 3 - Live session lifecycle, dashboard, summary, and recovery
FR13: Epic 3 - Live session lifecycle, dashboard, summary, and recovery
FR14: Epic 3 - Live session lifecycle, dashboard, summary, and recovery
FR15: Epic 3 - Live session lifecycle, dashboard, summary, and recovery
FR16: Epic 3 - Live session lifecycle, dashboard, summary, and recovery
FR17: Epic 3 - Live session lifecycle, dashboard, summary, and recovery
FR18: Epic 3 - Live session lifecycle, dashboard, summary, and recovery
FR19: Epic 3 - Live session lifecycle, dashboard, summary, and recovery
FR20: Epic 3 - Live session lifecycle, dashboard, summary, and recovery
FR21: Epic 4 - Folder watching, stable JPG ingestion, routing, duplicate safety, and quarantine
FR22: Epic 4 - Folder watching, stable JPG ingestion, routing, duplicate safety, and quarantine
FR23: Epic 4 - Folder watching, stable JPG ingestion, routing, duplicate safety, and quarantine
FR24: Epic 4 - Folder watching, stable JPG ingestion, routing, duplicate safety, and quarantine
FR25: Epic 4 - Folder watching, stable JPG ingestion, routing, duplicate safety, and quarantine
FR26: Epic 4 - Folder watching, stable JPG ingestion, routing, duplicate safety, and quarantine
FR27: Epic 4 - Folder watching, stable JPG ingestion, routing, duplicate safety, and quarantine
FR28: Epic 4 - Folder watching, stable JPG ingestion, routing, duplicate safety, and quarantine
FR29: Epic 4 - Folder watching, stable JPG ingestion, routing, duplicate safety, and quarantine
FR30: Epic 4 - Folder watching, stable JPG ingestion, routing, duplicate safety, and quarantine
FR31: Epic 5 - Original-first local save, LUT processing, retry, collision safety, and processing status
FR32: Epic 5 - Original-first local save, LUT processing, retry, collision safety, and processing status
FR33: Epic 5 - Original-first local save, LUT processing, retry, collision safety, and processing status
FR34: Epic 5 - Original-first local save, LUT processing, retry, collision safety, and processing status
FR35: Epic 5 - Original-first local save, LUT processing, retry, collision safety, and processing status
FR36: Epic 5 - Original-first local save, LUT processing, retry, collision safety, and processing status
FR37: Epic 5 - Original-first local save, LUT processing, retry, collision safety, and processing status
FR38: Epic 5 - Original-first local save, LUT processing, retry, collision safety, and processing status
FR39: Epic 5 - Original-first local save, LUT processing, retry, collision safety, and processing status
FR40: Epic 3 - Operator dashboard and troubleshooting views (feature-specific status extended by Epics 4-6)
FR41: Epic 3 - Operator dashboard and troubleshooting views (feature-specific status extended by Epics 4-6)
FR42: Epic 3 - Operator dashboard and troubleshooting views (feature-specific status extended by Epics 4-6)
FR43: Epic 3 - Operator dashboard and troubleshooting views (feature-specific status extended by Epics 4-6)
FR44: Epic 3 - Operator dashboard and troubleshooting views (feature-specific status extended by Epics 4-6)
FR45: Epic 3 - Operator dashboard and troubleshooting views (feature-specific status extended by Epics 4-6)
FR46: Epic 3 - Operator dashboard and troubleshooting views (feature-specific status extended by Epics 4-6)
FR47: Epic 3 - Operator dashboard and troubleshooting views (feature-specific status extended by Epics 4-6)
FR48: Epic 3 - Operator dashboard and troubleshooting views (feature-specific status extended by Epics 4-6)
FR49: Epic 2 - Event readiness, health, disk safety, and operator action records
FR50: Epic 2 - Event readiness, health, disk safety, and operator action records
FR51: Epic 2 - Event readiness, health, disk safety, and operator action records
FR52: Epic 1/Epic 2 - Foundation health endpoint plus readiness/health indicators
FR53: Epic 1/Epic 2/Epic 3 - Activity logging foundation and operator action records
FR54: Epic 4/Epic 5 - Restart recovery for pending ingestion and processing jobs
FR55: Epic 6 - Restart recovery for pending cloud upload jobs
FR56: Epic 6 - Post-session cloud fulfillment, upload tracking, retry, and duplicate safety
FR57: Epic 6 - Post-session cloud fulfillment, upload tracking, retry, and duplicate safety
FR58: Epic 6 - Post-session cloud fulfillment, upload tracking, retry, and duplicate safety
FR59: Epic 6 - Post-session cloud fulfillment, upload tracking, retry, and duplicate safety
FR60: Epic 6 - Post-session cloud fulfillment, upload tracking, retry, and duplicate safety
FR61: Epic 6 - Post-session cloud fulfillment, upload tracking, retry, and duplicate safety
FR62: Epic 6 - Post-session cloud fulfillment, upload tracking, retry, and duplicate safety
FR63: Epic 6 - Post-session cloud fulfillment, upload tracking, retry, and duplicate safety

## Epic List

### Epic 1: Local App Foundation & Operator Access
Operator bisa menjalankan aplikasi lokal di Windows, login dengan PIN, melihat shell dashboard dasar, dan sistem punya schema/config/API/event foundation untuk semua fitur berikutnya.
**FRs covered:** FR52, FR53

### Epic 2: Camera Station Setup & Readiness
Admin bisa mengonfigurasi 3 camera station, memvalidasi folder/device/LUT/output readiness, mencegah konfigurasi tidak aman, backup/restore config, dan memperbaiki station bermasalah sebelum event.
**FRs covered:** FR1, FR2, FR3, FR4, FR5, FR6, FR7, FR8, FR9, FR10, FR11, FR49, FR50, FR51, FR52, FR53

### Epic 3: Live Session Control Room
Operator bisa membuat dan mengelola session per station dengan customer/order/timer, melihat 3 station card realtime, mengakhiri session manual/otomatis, dan melihat session summary awal.
**FRs covered:** FR12, FR13, FR14, FR15, FR16, FR17, FR18, FR19, FR20, FR40, FR41, FR42, FR43, FR44, FR45, FR46, FR48, FR53

### Epic 4: Photo Ingestion, Routing & Quarantine Safety
Sistem bisa memantau folder input, menunggu JPG stabil, mencegah duplicate processing, route foto ke session aktif, quarantine foto tanpa session/late photo, dan memberi operator UI review/assign.
**FRs covered:** FR21, FR22, FR23, FR24, FR25, FR26, FR27, FR28, FR29, FR30, FR43, FR44, FR48, FR54

### Epic 5: Original-First Processing & Local Delivery
Sistem menyimpan original dulu, apply LUT, simpan graded output, track processing status, handle missing/invalid LUT, retry otomatis/manual, cegah collision, tampilkan queue/status, dan recovery processing jobs.
**FRs covered:** FR31, FR32, FR33, FR34, FR35, FR36, FR37, FR38, FR39, FR43, FR45, FR46, FR47, FR48, FR54

### Epic 6: Post-Session Cloud Fulfillment
Admin bisa connect Google Drive fulfillment flow, upload original+graded setelah local completion, track per-file/per-session upload, retry failure, prevent duplicate uploads, dan ensure upload tidak blokir session baru.
**FRs covered:** FR56, FR57, FR58, FR59, FR60, FR61, FR62, FR63, FR19, FR43, FR46, FR47, FR48, FR55
## Epics and Stories

## Epic 1: Local App Foundation & Operator Access
Operator bisa menjalankan aplikasi lokal di Windows, login dengan PIN, melihat shell dashboard dasar, dan sistem punya schema/config/API/event foundation untuk semua fitur berikutnya.

### Story 1.1: Create Windows Local App Skeleton

As an admin,
I want to start the local dashboard and Go service from Windows scripts,
So that event operation can run from one admin PC.

**Acceptance Criteria:**

**Given** clean project checkout on Windows
**When** admin runs local start script
**Then** Go service starts on configured localhost port
**And** Next.js dashboard starts on configured localhost port
**And** script prints local dashboard URL and local network URL placeholder/detection result
**And** script fails with clear message if required env values are missing
**And** monorepo contains `apps/web`, `apps/agent`, `docs/api`, and database migration location.

### Story 1.2: Add Local PIN Gate

As an operator,
I want dashboard access protected by a local PIN/password gate,
So that only authorized event staff can use operational controls.

**Acceptance Criteria:**

**Given** app is running without active authenticated session
**When** operator opens dashboard
**Then** operator sees PIN/password gate before operational UI
**And** valid PIN creates local authenticated session
**And** invalid PIN shows actionable error without leaking configured PIN
**And** logout clears local authenticated session
**And** browser never receives Supabase service role key or Google Cloud credentials.

### Story 1.3: Establish API, Error, and SSE Contract Foundation

As a frontend developer,
I want stable REST, error, and SSE response contracts,
So that dashboard features can integrate consistently.

**Acceptance Criteria:**

**Given** Go service is running
**When** client calls `/api/health`
**Then** response uses `{data}` wrapper with `snake_case` fields
**And** failed responses use `{error:{code,message,action,details}}`
**And** `/events` exposes SSE connection with dot-notation event names
**And** SSE payload uses standard event wrapper with `event_type`, `entity_type`, `entity_id`, `occurred_at`, and `data`
**And** `docs/api/openapi.yaml` documents initial health/auth/event endpoints.

### Story 1.4: Create Health Dashboard Shell

As an operator,
I want to see core app health indicators on dashboard,
So that I know whether local service is ready for event operation.

**Acceptance Criteria:**

**Given** authenticated operator opens dashboard
**When** dashboard loads
**Then** it shows service health, database reachability, worker placeholder status, disk placeholder status, and event stream status
**And** statuses use text labels, not color only
**And** loading and error states are visible and actionable
**And** data refreshes via TanStack Query and can receive SSE health update events
**And** UI uses shadcn/ui components.

### Story 1.5: Record and View Operator Activity Log Foundation

As an admin,
I want core operator actions recorded and visible,
So that troubleshooting has an audit trail from the start.

**Acceptance Criteria:**

**Given** authenticated operator performs login, logout, health recheck, or config placeholder action
**When** action completes or fails
**Then** Go service records timestamped activity log entry
**And** log entry includes action type, result, safe message, and optional station/session references when available
**And** dashboard exposes activity log view with timestamp and action filter
**And** logs avoid unnecessary sensitive data
**And** API supports fetching recent logs.

## Epic 2: Camera Station Setup & Readiness
Admin bisa mengonfigurasi 3 camera station, memvalidasi folder/device/LUT/output readiness, mencegah konfigurasi tidak aman, backup/restore config, dan memperbaiki station bermasalah sebelum event.

### Story 2.1: Configure Three Camera Stations

As an admin,
I want to create and edit three logical camera stations,
So that each physical photo source has a named operational slot.

**Acceptance Criteria:**

**Given** authenticated admin opens station settings
**When** admin creates or edits station name, physical camera/device identifier, input folder, background name, default LUT, and output rule
**Then** system saves station configuration
**And** each station displays its configured values
**And** required fields cannot be empty
**And** duplicate input folders are blocked with actionable error
**And** activity log records config changes.

### Story 2.2: Validate Station Paths, LUTs, and Device Readiness

As an admin,
I want to validate station folders, LUTs, and device readiness,
So that invalid station setup cannot appear ready.

**Acceptance Criteria:**

**Given** stations have saved configuration
**When** admin runs readiness check or opens station status
**Then** system checks input folder readable, output folder writable, LUT file present/valid, and camera/device identifier status if available
**And** station cannot show READY when required check fails
**And** failure shows specific action such as Recheck Camera or Fix Folder Path
**And** statuses include text labels.

### Story 2.3: Run Test Capture Watch Validation

As an admin,
I want to run test watch validation for each station,
So that folder watcher behavior is verified before an event.

**Acceptance Criteria:**

**Given** station has valid folders
**When** admin starts test validation and places or selects a test JPG in input folder
**Then** system detects stable JPG without assigning it to a real session
**And** result shows detected station, stable detection success/failure, and validation timestamp
**And** duplicate filesystem events do not create duplicate validation records
**And** test output is clearly labeled as validation-only.

### Story 2.4: Refresh and Reconnect Station Health

As an operator,
I want to refresh or reconnect a station with camera/folder issues,
So that I can recover event readiness without restarting everything.

**Acceptance Criteria:**

**Given** station has ATTENTION status
**When** operator clicks refresh/reconnect/recheck
**Then** system reruns station-specific checks
**And** station status updates via API and SSE
**And** action result is logged
**And** errors include specific operator action.

### Story 2.5: Backup and Restore Station Configuration

As an admin,
I want to backup and restore station configuration,
So that event setup can recover from accidental changes.

**Acceptance Criteria:**

**Given** station configuration exists
**When** admin exports backup
**Then** system creates backup containing station config without secrets
**And** backup can be imported later
**And** restore validates duplicate folders and required paths before applying
**And** restore action is logged with success or failure.

### Story 2.6: Run Event Readiness Checklist

As an operator,
I want a pre-event readiness checklist,
So that unsafe operation is blocked before sessions start.

**Acceptance Criteria:**

**Given** operator opens readiness checklist
**When** checklist runs
**Then** system checks input folders readable, output folders writable, LUTs valid, disk thresholds, watcher running, processor ready, app data/state health, quarantine folder writable, and cloud status known
**And** required failures block session start
**And** disk warning/block thresholds are configurable
**And** checklist displays actionable pass/warn/fail text labels.

## Epic 3: Live Session Control Room
Operator bisa membuat dan mengelola session per station dengan customer/order/timer, melihat 3 station card realtime, mengakhiri session manual/otomatis, dan melihat session summary awal.

### Story 3.1: Start One Active Session Per Station

As an operator,
I want to create a session for a station with customer, order number, and timer,
So that incoming photos route to the correct customer/order.

**Acceptance Criteria:**

**Given** station passes required readiness checks
**When** operator starts session with customer name, order number, and timer duration
**Then** system creates active session for that station
**And** starting a second active session on same station is blocked
**And** session config snapshot stores station background, LUT, and output rules
**And** session start is logged.

### Story 3.2: Display Three Station Live Cards

As an operator,
I want a dashboard with three live camera station cards,
So that I can monitor the event at a glance.

**Acceptance Criteria:**

**Given** dashboard is open
**When** station/session state changes
**Then** each card shows customer, order number, timer, station status, LUT, photo count, last photo preview placeholder, and upload/local status
**And** states include READY, LIVE, ATTENTION, LOCKED, and UPLOAD status with text labels
**And** updates arrive near real-time via SSE and TanStack Query invalidation
**And** critical alerts are visible without relying on color only.

### Story 3.3: End Sessions Manually or by Timer

As an operator,
I want sessions to end manually or automatically when timer expires,
So that photos stop entering finished sessions.

**Acceptance Criteria:**

**Given** station has active session
**When** operator confirms End Session or timer expires
**Then** session transitions to ending then locked
**And** new photos after lock cannot enter that session
**And** manual End Session requires confirmation
**And** boundary cases resolve deterministically using server-side timestamps
**And** session end is logged.

### Story 3.4: Recover Live Session State After Restart

As an operator,
I want session state to recover after app restart,
So that event operation can continue safely.

**Acceptance Criteria:**

**Given** app restarts during active, ending, locked, local complete, upload pending, uploaded, or failed session state
**When** service starts again
**Then** system reloads persisted session state
**And** dashboard displays recovered state clearly
**And** active/locked boundaries remain deterministic
**And** recovery activity is logged.

### Story 3.5: Show Session Detail, Summary, and Local Folder Access

As an operator,
I want session detail and summary views,
So that I can troubleshoot and deliver completed local results.

**Acceptance Criteria:**

**Given** session exists
**When** operator opens session detail
**Then** UI shows duration, photo counts, failures, quarantine count, local output path, local save status, and cloud upload status separately
**And** operator can open local result folder for completed session on admin PC
**And** logs can be filtered by station/session
**And** failed state shows specific next action.

## Epic 4: Photo Ingestion, Routing & Quarantine Safety
Sistem bisa memantau folder input, menunggu JPG stabil, mencegah duplicate processing, route foto ke session aktif, quarantine foto tanpa session/late photo, dan memberi operator UI review/assign.

### Story 4.1: Watch Station Input Folders for Stable JPGs

As an operator,
I want the system to detect new stable JPGs in each station folder,
So that photos can enter the correct station flow safely.

**Acceptance Criteria:**

**Given** station watcher is running
**When** JPG appears in station input folder
**Then** system identifies producing station
**And** waits until file is stable before marking detected
**And** partial/incomplete JPG is never treated as processed
**And** duplicate filesystem events do not create duplicate photo records
**And** watcher status updates dashboard.

### Story 4.2: Route Valid JPGs to Active Sessions

As an operator,
I want detected photos routed to the station active session,
So that each customer receives correct photos.

**Acceptance Criteria:**

**Given** station has active session
**When** stable JPG is detected before session lock
**Then** system creates photo record tied to that session and station
**And** record preserves source path and detected timestamp
**And** routing uses server-owned session state, not frontend state
**And** routing status appears in station/session detail.

### Story 4.3: Quarantine Unassigned and Late Photos

As an operator,
I want photos with no eligible session quarantined with reason,
So that wrong customer routing is prevented.

**Acceptance Criteria:**

**Given** stable JPG is detected with no active session or after session lock
**When** routing is evaluated
**Then** system moves/records photo as quarantined/unassigned
**And** quarantine reason is stored
**And** operator alert shows quarantine count and reason category
**And** quarantined photos are never silently assigned.

### Story 4.4: Review and Assign Quarantined Photos

As an operator,
I want to review quarantined photos and assign them manually,
So that recoverable photos can be delivered to the correct session.

**Acceptance Criteria:**

**Given** quarantined photos exist
**When** operator opens quarantine view
**Then** UI shows photo preview/path metadata, station, detected time, and reason
**And** operator can assign photo to an eligible session with confirmation
**And** assignment creates traceability from source to final session
**And** assignment is logged
**And** duplicate assignment is blocked.

### Story 4.5: Recover Pending Ingestion and Quarantine State

As an operator,
I want pending ingestion/quarantine state recovered after restart,
So that photos are not lost or duplicated.

**Acceptance Criteria:**

**Given** app restarts while files are detected, pending routing, or quarantined
**When** service starts
**Then** system reconciles persisted records and filesystem state
**And** duplicate-safe identities prevent double processing
**And** unresolved records appear with actionable status
**And** recovery is logged.

## Epic 5: Original-First Processing & Local Delivery
Sistem menyimpan original dulu, apply LUT, simpan graded output, track processing status, handle missing/invalid LUT, retry otomatis/manual, cegah collision, tampilkan queue/status, dan recovery processing jobs.

### Story 5.1: Save Original JPG Before Processing

As an operator,
I want every valid session photo original saved first,
So that customer photos are preserved even if grading fails.

**Acceptance Criteria:**

**Given** routed session photo exists
**When** local delivery starts
**Then** system copies/saves original JPG to deterministic customer/order/station folder
**And** original save completes before LUT processing begins
**And** final persisted record never points to missing local original after success
**And** filename collisions are prevented
**And** traceability includes source path and local original path.

### Story 5.2: Apply Station/Session LUT to Create Graded JPG

As an operator,
I want photos graded with the station/session LUT,
So that event output has consistent background-specific look.

**Acceptance Criteria:**

**Given** original JPG is saved and session has LUT snapshot
**When** processor runs
**Then** system creates graded JPG in deterministic output folder
**And** missing/invalid LUT marks graded job failed/retryable while preserving original
**And** exact LUT processor library/tool is documented in implementation notes
**And** processing status updates dashboard.

### Story 5.3: Track Processing Queue and Photo Status

As an operator,
I want to see processing queue and per-photo status,
So that I can spot delays or failures during event work.

**Acceptance Criteria:**

**Given** photos are pending, processing, succeeded, failed, or retrying
**When** operator opens queue/status view
**Then** UI shows queue counts, current job state, failure reason, retry count, and last update
**And** status updates via SSE
**And** dashboard remains interactive while processing runs
**And** processing does not block other stations.

### Story 5.4: Retry Failed Photo Processing

As an operator,
I want failed processing retried automatically and manually,
So that transient failures can recover without losing originals.

**Acceptance Criteria:**

**Given** processing job fails
**When** retry policy runs
**Then** system retries up to three times automatically
**And** operator can manually retry failed processing
**And** retries are duplicate-safe
**And** retry actions are logged
**And** repeated failure shows actionable reason.

### Story 5.5: Recover Pending Processing Jobs After Restart

As an operator,
I want pending processing jobs recovered after restart,
So that local delivery continues safely.

**Acceptance Criteria:**

**Given** app restarts during pending, processing, retrying, or failed processing state
**When** service starts
**Then** system reloads job records
**And** reconciles local original/graded files with persisted state
**And** duplicate-safe job identity prevents double output
**And** dashboard shows recovered queue state.

## Epic 6: Post-Session Cloud Fulfillment
Admin bisa connect Google Drive fulfillment flow, upload original+graded setelah local completion, track per-file/per-session upload, retry failure, prevent duplicate uploads, dan ensure upload tidak blokir session baru.

### Story 6.1: Configure Cloud Storage Credentials and Target Rules

As an admin,
I want to configure one cloud storage account and target rules,
So that completed session assets can upload after local delivery.

**Acceptance Criteria:**

**Given** admin opens cloud settings
**When** admin configures credentials and target root/folder rules
**Then** service stores credentials server-side only
**And** browser never receives credentials
**And** connection check reports authorized/failed status
**And** Google Drive folder naming convention is documented and validated
**And** cloud status appears separately from local save status.

### Story 6.2: Create Cloud Folder/Object Structure Per Session

As an operator,
I want cloud assets organized by customer and order number,
So that post-event fulfillment is easy to find.

**Acceptance Criteria:**

**Given** session reaches local complete
**When** cloud fulfillment begins
**Then** system creates or resolves remote path/object prefix based on customer name and order number
**And** sanitized names prevent invalid paths
**And** remote identity is stored for session
**And** operation is idempotent across retry/restart.

### Story 6.3: Upload Original and Graded JPGs After Local Completion

As an operator,
I want original and graded JPGs uploaded only after local completion,
So that cloud fulfillment never risks local event capture.

**Acceptance Criteria:**

**Given** session local output is complete
**When** upload worker runs
**Then** original and graded JPGs upload to configured cloud target
**And** upload does not block new capture sessions, local save, session routing, or LUT processing
**And** local files are preserved when upload fails
**And** per-file upload status is tracked.

### Story 6.4: Track, Retry, and De-Duplicate Cloud Uploads

As an operator,
I want failed uploads retried safely,
So that cloud delivery recovers without duplicate assets.

**Acceptance Criteria:**

**Given** upload fails due to token, network, partial upload, or remote error
**When** retry runs automatically or operator retries manually
**Then** system uses tracked upload identity/status to prevent duplicate uploads
**And** upload status updates per file and per session
**And** failure shows actionable Retry Drive/Cloud Upload action
**And** retry action is logged.

### Story 6.5: Recover Pending Cloud Upload Jobs After Restart

As an operator,
I want pending cloud upload jobs recovered after restart,
So that fulfillment can continue later.

**Acceptance Criteria:**

**Given** app restarts while upload jobs are pending, uploading, failed, or partially complete
**When** service starts
**Then** system reloads upload queue
**And** reconciles remote identity/status enough to reduce duplicate uploads
**And** dashboard shows upload pending/uploaded/failed state separately from local complete
**And** capture and local processing remain available while upload recovers.

## Epic 7: Managed gPhoto2 Camera Tethering
Operator bisa menghubungkan kamera fisik ke station melalui gPhoto2/tether helper, memonitor status kamera, menjalankan listener capture, memulihkan disconnect saat event, dan tetap mempertahankan folder input station sebagai source of truth untuk pipeline ingestion → local save → LUT → Google Drive.

### Story 7.1: Detect and Assign gPhoto2 Cameras to Stations

As an admin,
I want to detect available gPhoto2 cameras and assign each camera/device/port to a logical station,
So that each physical camera reliably drops JPGs into the correct station input folder.

**Acceptance Criteria:**

**Given** gPhoto2 is available natively or through WSL/usbipd
**When** admin runs camera discovery
**Then** system returns normalized camera/device/port status per detected camera
**And** admin can assign a detected camera identity to station_1/station_2/station_3
**And** assignments persist across restart when the same camera identity is detectable
**And** unavailable gPhoto2/WSL/usbipd/camera states return safe actionable errors
**And** no install, driver change, usbipd bind, or privileged command is executed without explicit operator action/approval.

### Story 7.2: Supervise gPhoto2 Tether Listener per Station

As an operator,
I want to start and stop a gPhoto2 tether listener for each station,
So that physical shutter captures are downloaded automatically into the station input folder.

**Acceptance Criteria:**

**Given** a station has camera assignment and writable input folder
**When** operator starts tether listener
**Then** system starts a supervised gPhoto2 wait-event/download process for that station
**And** downloaded JPGs are written only inside that station input folder with collision-safe filenames
**And** folder watcher remains the only ingestion source of truth
**And** duplicate start is rejected/no-op safely
**And** stop terminates only the station listener process
**And** listener stdout/stderr are sanitized before logs/API/SSE/activity.

### Story 7.3: Add Camera Readiness Checks and Test Capture Validation

As an operator,
I want readiness to validate real camera/tether health before sessions start,
So that camera problems are caught before guests arrive.

**Acceptance Criteria:**

**Given** station readiness is requested
**When** camera checks run
**Then** readiness reports gPhoto2 availability, camera assignment, camera connected, listener running, input folder writable, and optional test capture result
**And** failed camera readiness can block session start only when configured as required
**And** test capture writes a JPG to station input folder and verifies the existing ingestion watcher sees it
**And** test capture never uploads to Drive directly and never bypasses local-first pipeline
**And** unsafe states show clear next actions such as INSTALL_GPHOTO2, CONNECT_CAMERA, START_TETHER_LISTENER, CHECK_USBIPD, or CHECK_STATION_INPUT_FOLDER.

### Story 7.4: Show Live Camera/Tether Status and Controls in the Dashboard

As an operator,
I want each station card to show camera and tether health,
So that I can react quickly during an event.

**Acceptance Criteria:**

**Given** dashboard is open
**When** camera/tether status changes
**Then** each station card shows connected/disconnected, assigned camera, listener running/stopped/error, last capture time, last downloaded file, and actionable next step
**And** operator can start/stop/retry listener and run test capture from the UI using authenticated trusted-origin requests
**And** UI distinguishes camera/tether status from local processing and Google Drive upload status
**And** errors do not expose raw shell commands, local paths beyond safe filenames, tokens, or privileged diagnostic output
**And** status updates are available through API polling and/or SSE without breaking existing station/session cards.

### Story 7.5: Recover and Reconnect gPhoto2 Tethering During Event

As an operator,
I want camera tethering to recover safely from disconnects and app restarts,
So that event capture can continue without duplicate ingestion or manual folder repair.

**Acceptance Criteria:**

**Given** a camera disconnect, gPhoto2 process exits, USB device changes, or the app restarts
**When** recovery runs
**Then** system marks affected station attention state and attempts only safe configured reconnect/restart behavior
**And** it never changes drivers, binds USB devices, or runs privileged commands without explicit approval
**And** it restarts listener only for stations previously configured for auto-restart
**And** downloaded files are still deduplicated by existing ingestion pipeline
**And** activity log records reconnect attempts/results with safe messages
**And** operator can manually retry reconnect from the dashboard.

### Story 7.6: Run Real Hardware gPhoto2 Event Smoke Test

As an operator,
I want a guided real-camera smoke test across one or more stations,
So that I know camera → gPhoto2 → station folder → ingestion → processing → Drive delivery works before production.

**Acceptance Criteria:**

**Given** at least one real camera is connected and assigned to a station
**When** smoke test starts
**Then** system guides operator through discovery, listener start, physical shutter capture, ingestion verification, local original save, graded processing, and optional Drive upload verification
**And** the smoke report records timestamps, station IDs, camera IDs, file names, ingestion counts, processing result, upload result, and failures
**And** smoke test never deletes camera files or local outputs
**And** smoke can run in local-only mode when Drive credentials are absent
**And** report is saved as an implementation artifact or local diagnostic file for event readiness review.
