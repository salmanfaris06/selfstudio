# Google Drive Cloud Delivery API

Cloud delivery uses Google Drive (`google_drive`) and is configured only through the Go agent. Credential fields are write-only: service account JSON/private key/token and credential file paths are never returned by GET responses, SSE payloads, activity logs, or frontend rendering.

## Endpoints

- `GET /api/cloud/settings` returns `{data}` with safe metadata: `provider`, `drive_root_folder_id`, `drive_root_folder_name`, `folder_naming_template`, `credentials_configured`, `connection_status`, `last_checked_at`, `last_error`, `last_error_code`, `last_error_action`.
- `PUT /api/cloud/settings` stores `provider: "google_drive"`, `drive_root_folder_id`, optional safe display name `drive_root_folder_name`, and optional write-only `service_account_json` or `credential_file_path` server-side under local runtime state.
- `POST /api/cloud/settings/check` verifies Google Drive authorization and root folder access with server-side credentials and updates `not_configured`, `authorized`, or `failed` status.
- `POST /api/cloud/settings/folder-preview` returns a deterministic sanitized Drive folder path preview using `{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}`.

## Safe Error Actions

- `FIX_DRIVE_CREDENTIALS`: configure/replace Google Drive credential server-side.
- `FIX_DRIVE_FOLDER`: verify the root folder ID exists and is shared/writable by the configured credential.
- `RETRY_DRIVE_CHECK`: retry after temporary network/API failure.
- `RETRY_DRIVE_UPLOAD`: retry a failed Google Drive file upload.
- `CHECK_LOCAL_OUTPUT`: verify local original/graded output still exists.
- `CHECK_DRIVE_FILE`: verify an uploaded Google Drive file identity when recovery cannot safely confirm it.
- `FIX_DRIVE_FOLDER_RULES`: fix unsafe folder naming input or provider mismatch.

## Security Rules

- Browser receives only safe metadata, never raw secrets or local credential file paths.
- Google Drive connection and upload work run only in the Go agent.
- Drive status is separate from local save, LUT processing, local delivery, and session status.
- Upload APIs expose `local_file_name` only; full `local_path` remains internal to the Go worker.

## Story 6.7 — Google Drive Folder Target Resolution

`POST /api/sessions/{session_id}/cloud-target/resolve` now resolves or creates the Google Drive folder chain under the configured `drive_root_folder_id` using the deterministic template `{yyyy}/{mm}/{dd}/{safe_customer_name}/{safe_order_number}/{station_id}/{session_id}`. The resolver looks up each folder by `(parent_id, exact sanitized name, folder mime type, not trashed)` before creating it, so retries/restarts reuse the same final Drive folder when it already exists.

Successful responses keep local-first behavior and do not start file upload jobs. The public target includes safe Drive metadata only:

- `drive_root_folder_id`, `drive_root_folder_name`
- `drive_folder_path` for display/debug
- `drive_session_folder_id` as the final session folder ID
- `drive_folder_chain[]` entries with `level`, `name`, `folder_id`, and `parent_id`
- `remote_identity`, equal to `drive_session_folder_id`

If Google Drive returns duplicate folders for the same parent/name, the resolver reuses a deterministic existing folder (oldest `createdTime`, then lowest ID) rather than creating another duplicate. Folder lookup drains all Drive result pages before selecting a match, so duplicates beyond the first page are still considered.

Failed responses persist a failed target with safe `last_error_code`/`last_error_action` such as `FIX_DRIVE_CREDENTIALS`, `FIX_DRIVE_FOLDER_RULES`, `RETRY_DRIVE_CHECK`, or `RETRY_DRIVE_FOLDER`. Session state, local output, photo records, processing queue, and quarantine are not modified by Drive folder failures. If a previously ready target already has a known `drive_session_folder_id`, transient Drive lookup/create failures preserve that ready identity instead of overwriting it with `failed`; explicit invalid configuration/authorization failures remain actionable. SSE events `cloud.target_resolved` and `cloud.target_failed` include only safe metadata and never include service account JSON, OAuth tokens, refresh tokens, private keys, or credential file paths.

`GET /api/sessions/{session_id}` exposes Drive folder target summary fields separately from aggregate upload status: `drive_target_status`, `drive_target_identity`, `drive_session_folder_id`, `drive_folder_path`, `drive_root_folder_id`, `drive_root_folder_name`, `drive_last_error_code`, and `drive_last_error_action`.

## Story 6.8 — Original/Graded JPG Uploads to Google Drive

`POST /api/sessions/{session_id}/uploads/start` discovers deterministic per-file upload jobs after the session is locked and the Story 6.7 Drive target is ready. The HTTP request persists job state and starts background upload work; it does not wait for every file to finish and does not block capture, session start/end, ingest, local original save, LUT processing, quarantine, or dashboard reads.

Eligibility rules:

- Original JPG is uploadable only from the saved local original output (`saved_original` + readable local file), never directly from the camera/source path.
- Graded JPG is uploadable only when graded processing is `processed` and the final graded file is readable.
- Graded `failed`/`not_eligible` creates a deterministic `asset_kind = graded` job with status `not_eligible`; original upload remains eligible.
- Missing final local original/graded files become safe `UPLOAD_LOCAL_FILE_MISSING` + `CHECK_LOCAL_OUTPUT` failures and local files/state are not deleted or mutated.
- Active or otherwise locally incomplete sessions are rejected with `UPLOAD_PENDING_LOCAL_COMPLETION` + `WAIT_FOR_LOCAL_COMPLETION`.
- Missing/unready Drive target is rejected with `CLOUD_TARGET_NOT_READY` + `RESOLVE_CLOUD_TARGET`.

Drive layout and identity:

- The worker resolves or creates `original` and `graded` subfolders below `drive_session_folder_id` using Drive folder lookup semantics before file upload.
- Each uploaded job stores `drive_folder_id` for the selected subfolder, `drive_file_id` for the Drive file, `remote_identity` mirroring/containing the Drive file ID, `photo_id`, `session_id`, `station_id`, `asset_kind`, `dedupe_key`, `status`, `attempt_count`, timestamps, and retry/error fields.
- Legacy `bucket_name`, `object_key`, generation, and metageneration fields are compatibility-only and are not the primary public mental model for Google Drive.

Public API/SSE safety:

- `GET /api/sessions/{session_id}/uploads` and `POST /uploads/start` return `{data}` with safe job DTOs. They include `local_file_name`, `drive_folder_id`, and `drive_file_id`, but never full local paths, credential material, tokens, service account JSON, private keys, credential paths, or raw Google error bodies.
- Upload SSE events (`upload.started`, `upload.file_uploaded`, `upload.retry_failed`, `upload.session_updated`, etc.) include only safe metadata: `session_id`, `station_id`, `photo_id`, `asset_kind`, `status`, `drive_folder_id`, `drive_file_id`, `last_error_code`, `last_error_action`, `attempt_count`, and retry timestamps/seconds.
- Activity messages use operator-safe Google Drive copy such as “Google Drive upload started”, “Google Drive file uploaded”, and “Google Drive upload failed”.

## Story 6.9 — Retry and De-duplicate Google Drive Uploads

Retry uses the existing authenticated/trusted-origin endpoints:

- `POST /api/uploads/{job_id}/retry`
- `POST /api/sessions/{session_id}/uploads/retry`

Both endpoints return `202 Accepted` for an accepted asynchronous retry or a duplicate-safe no-op. Public retry responses include `session_id`, `upload_status`, `session_upload_status`, `jobs[]`, `accepted`, and optional `noop_reason`. The response remains safe: no `local_path`, credential path, service account JSON, token/private key, or raw Google API body is returned.

Retry policy and status semantics:

- Retryable Drive failures use safe metadata such as `DRIVE_UPLOAD_FAILED` + `RETRY_DRIVE_UPLOAD`.
- Credential, authorization, folder, target-not-ready, and missing-local-file failures are not auto/manual retryable until the operator fixes the safe action.
- `MaxAutoUploadAttempts = 3` is the total attempt limit per job, not three extra retries. Manual retry follows the same safe default limit.
- `retrying` means a background worker accepted the job. `retry_scheduled` means automatic retry is planned with `next_retry_at`/`retry_after`.
- Running/uploaded retry requests are no-ops with `UPLOAD_ALREADY_RUNNING` or `UPLOAD_ALREADY_UPLOADED` and do not increment `attempt_count`.

Deduplication rules:

- Job identity remains deterministic: `job_id = dedupe_key = (session_id, photo_id, asset_kind)`.
- Creating/retrying upload jobs never creates a second local job for the same file identity.
- If a job already has `drive_file_id` and is uploaded, retry does not call Drive again.
- If a previous attempt created a Drive file before the local state was fully persisted, retry first looks up an existing Drive file by exact parent `drive_folder_id`, sanitized file name, MIME `image/jpeg`, and `trashed=false`; when found, the worker stores that `drive_file_id`/`remote_identity` instead of creating a duplicate Drive file.
- App-properties identity can be added later for stronger Drive-side lookup, but current Story 6.9 contract relies on persisted Drive IDs plus deterministic parent/safe-name fallback.

Aggregate upload status remains separate from local save/processing and Drive folder target status: `uploading` for uploading/retrying jobs, `partial_failed` for mixed uploaded+failed eligible jobs, `failed` for all eligible failures, `pending` for pending/retry_scheduled jobs, and `uploaded` only when all eligible jobs are uploaded (not_eligible graded jobs do not block success).

## Story 6.10 — Startup Recovery for Pending Google Drive Upload Jobs

On agent startup, persisted upload jobs are loaded from local state and normalized before any recovered upload is resumed. Recovery is local-first and non-destructive: it never scans arbitrary output folders, never mutates local original/graded JPG files, and never treats local completion as equivalent to Google Drive upload completion.

Recovery status semantics:

- `pending` jobs with readable local files and a persisted `drive_folder_id` are eligible for asynchronous resume using the same deterministic `job_id`/`dedupe_key`.
- stale `uploading`/`retrying` jobs from a crashed process are not considered uploaded. They are normalized to due `retry_scheduled` with `DRIVE_UPLOAD_FAILED` + `RETRY_DRIVE_UPLOAD` when retryable and under `MaxAutoUploadAttempts`; exhausted/non-retryable jobs remain `failed`.
- `retry_scheduled` jobs resume only when `next_retry_at` is due or absent. Future schedules are preserved.
- uploaded jobs keep `uploaded` when Drive identity is safe (`drive_file_id` and/or `remote_identity`). If only one compatibility field is present, recovery repairs `drive_file_id`/`remote_identity` mirror fields after durable save. Without a verifier, uploaded Drive jobs with `drive_file_id` are counted as `unverified_uploaded`, not downgraded.
- uploaded jobs without safe Drive identity are marked failed with `UPLOAD_REMOTE_CHECK_NEEDED` + `CHECK_DRIVE_FILE`; legacy object check fields remain compatibility-only.
- non-uploaded jobs with missing/unreadable local files become `UPLOAD_LOCAL_FILE_MISSING` + `CHECK_LOCAL_OUTPUT` and are not resumed.

`upload.recovered` is published only after the normalized job state is durably saved. Its payload contains summary counts (`recovered_pending`, `resumed`, `failed_missing_local`, `verified_uploaded`, `unverified_uploaded`, `requires_cloud_check`, `errors`) and excludes full local paths, credentials, tokens, private keys, and raw Google API error bodies. Recovery activity copy is Google Drive-specific: “Google Drive upload recovery completed”. Recovered resume uses the existing upload worker path, so Drive duplicate prevention still relies on exact parent folder + safe filename lookup before create.
