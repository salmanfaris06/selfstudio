# Deferred Work


## Deferred from: code review of 2-4-save-and-restore-station-configuration (2026-05-19)

- Add optimistic concurrency/versioning for multi-tab station edits. Reason: beyond Story 2.4 MVP scope.
- Add restore preview/confirmation token/checksum flow. Reason: UX hardening beyond current acceptance criteria.
- Tighten broader station path containment policy for input/LUT paths. Reason: broader station safety policy beyond persistence story.
- Revisit logout/login OpenAPI/security behavior. Reason: pre-existing auth contract outside Story 2.4.
- Theme CSS consistency cleanup. Reason: pre-existing/global UI polish.


## Deferred from: code review of 2-5-run-test-capture-watch-validation (2026-05-19)

- Async/rate-limited validation requests: deferred because current story is bounded synchronous local-operator validation.
- Tokenized/new-file-only validation: deferred because story explicitly accepts already-present JPGs.
- JPEG magic/header validation: deferred because current scope validates stable JPG candidate by extension.
- Source-path redaction: deferred because AC asks to show source path in local admin context.
- Store invariant hardening for ReplaceAll/List: deferred as pre-existing broader store hardening.
- Server-wide WriteTimeout split: deferred as pre-existing SSE architecture tradeoff.
- onAuthExpired wiring: deferred as pre-existing auth UX issue.


## Deferred from: code review of 2-6-refresh-and-reconnect-station-health (2026-05-19)

- Wire `onAuthExpired` into SSE/API auth expiry handling. Reason: pre-existing auth UX issue outside Story 2.6.
- Revisit login/logout trusted-Origin/auth contract and public health endpoint. Reason: pre-existing security contract outside Story 2.6.
- Loosen placeholder-era event readiness runtime validation for future session-start implementation. Reason: future Epic 3 compatibility issue.
- Split refresh success/failure SSE event names further. Reason: current event means refresh completed and payload carries readiness status.


## Deferred from: code review of 3-1-start-one-active-session-per-station (2026-05-19)

- Full concurrent session start + persistence serialization and Windows atomic replace primitive. Reason: broader persistence hardening; current rollback covers single save failure.
- End-session/timer-expiry release behavior. Reason: belongs to Story 3.3 session ending/timer lifecycle.
- Dedicated session list/read endpoint and live-card query. Reason: belongs to Story 3.2 live station cards.


## Deferred from: code review of 3-3-end-sessions-manually-or-by-timer (2026-05-19)

- Full rollback/transactional persistence for manual end failure. Reason: broader persistence hardening; timer lock persistence is now handled.
- Live countdown interval/re-render every second. Reason: current query polling is acceptable shell behavior; can be improved in UI polish.
- Strong sanitization for customer/order filesystem path segments and true `{session_id}` output rule derivation. Reason: broader output path hardening before ingestion/local delivery.


## Deferred from: code review of 4-1-watch-station-input-folders-for-stable-jpgs (2026-05-19)

- Strong JPEG header/content validation and multi-observation stability. Reason: ingestion hardening before high-volume event simulation.
- Replace raw source path in `photo.detected` SSE entity/data with generated photo IDs. Reason: persistent photo model starts in Story 4.2/4.3.
- Bounded scan duration/max files for large folders. Reason: performance hardening for later load testing.
