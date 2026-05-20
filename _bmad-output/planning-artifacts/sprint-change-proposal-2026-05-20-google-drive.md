# Sprint Change Proposal — Replace GCS with Google Drive Upload

**Project:** selfstudio  
**Date:** 2026-05-20  
**Requested by:** alpharize  
**Mode:** Batch proposal  
**Status:** Approved

## 1. Issue Summary

### Trigger

Saat release readiness/event-scale smoke, cloud fulfillment diarahkan ke Google Cloud Storage (GCS). User mengklarifikasi bahwa kebutuhan sebenarnya adalah **Google Drive**, bukan GCS.

### Core Problem

Ini adalah **misunderstanding / implementation drift dari requirement asli**. PRD berulang kali menyebut Google Drive sebagai deliverable cloud untuk operator/customer, tetapi architecture dan Epic 6 mengubahnya menjadi Google Cloud Storage (`gcs`). Implementasi mengikuti architecture yang drift tersebut, sehingga hasilnya tidak cocok dengan workflow manusia/operator yang diinginkan.

### Evidence

PRD menyebut Google Drive sebagai MVP scope dan success criteria:

- “Google Drive upload setelah session selesai”
- “Google Drive folder otomatis berdasarkan customer + order”
- “Upload original + graded ke Google Drive”
- “Retry failed Drive upload”
- FR56–FR63 semuanya menyebut Google Drive/Drive upload.

Architecture dan current implementation menyebut GCS:

- `Cloud storage: Google Cloud Storage stores post-session cloud image assets`
- `provider: gcs`
- endpoint cloud settings memakai `bucket_name`, `target_root_prefix`, service account JSON/path.

User confirmation:

> “bukankah saya mintanya google drive saja?”

## 2. Change Navigation Checklist Summary

### Section 1 — Understand Trigger and Context

- [x] 1.1 Triggering story: Epic 6, terutama Story 6.1–6.5 cloud fulfillment.
- [x] 1.2 Core problem: PRD says Google Drive, architecture/implementation says GCS.
- [x] 1.3 Evidence: PRD FR56–FR63 + user clarification + GCS implementation/docs.

### Section 2 — Epic Impact Assessment

- [x] 2.1 Current epic can still be completed, but provider scope must change from GCS to Google Drive.
- [x] 2.2 Modify Epic 6 scope and acceptance criteria.
- [x] 2.3 No impact to Epics 1–5 except labels/status copy where “cloud/GCS” appears.
- [x] 2.4 No new epic required; this is a replacement within Epic 6.
- [x] 2.5 Epic order does not change.

### Section 3 — Artifact Conflict and Impact Analysis

- [x] 3.1 PRD remains correct and should be treated as source of truth.
- [x] 3.2 Architecture conflicts and must be updated from GCS to Google Drive API.
- [x] 3.3 UI/UX copy must say Google Drive, not GCS/bucket/cloud storage.
- [x] 3.4 API docs, OpenAPI, config, runtime state docs, and smoke checklist need updates.

### Section 4 — Path Forward Evaluation

#### Option 1: Direct Adjustment

**Viable.** Replace provider implementation and docs while preserving upload queue/retry/recovery architecture.

- Effort: Medium/High
- Risk: Medium
- Rationale: Existing upload job model, session completion flow, retry, and local-first safety can be reused.

#### Option 2: Rollback

**Not recommended.** Full rollback of Epic 6 is unnecessary. GCS implementation can be refactored/replaced at provider boundary.

- Effort: High
- Risk: High

#### Option 3: PRD MVP Review

**Not needed.** PRD already says Google Drive. MVP is still achievable after correcting implementation.

- Effort: Medium
- Risk: Low/Medium

#### Recommended Path

**Direct Adjustment:** Replace GCS-specific cloud fulfillment with Google Drive provider while keeping the core upload workflow:

local complete → resolve remote folder → create upload jobs → upload originals/graded → retry/dedupe → restart recovery.

## 3. Impact Analysis

### Epic Impact

#### Epic 6

Epic 6 must be redefined from “Google Cloud/Drive-equivalent storage flow sesuai architecture GCS” to “Google Drive fulfillment.”

Affected stories:

- 6.1 Configure Google Drive credentials and folder rules
- 6.2 Create Google Drive folder structure per session
- 6.3 Upload original and graded JPGs after local completion
- 6.4 Track, retry, and de-duplicate Drive uploads
- 6.5 Recover pending Drive upload jobs after restart

### PRD Impact

PRD mostly remains valid. Minor cleanup only if any “Google Cloud Storage” wording appears in PRD additional requirements.

Required correction:

```text
OLD: Cloud storage: Google Cloud Storage stores post-session cloud image assets; local files must never be deleted due to cloud upload failure.
NEW: Cloud fulfillment: Google Drive stores post-session original and graded image assets in operator/customer-friendly folders; local files must never be deleted due to Drive upload failure.
```

### Architecture Impact

Architecture needs substantive update:

- Replace GCS upload worker with Google Drive upload worker.
- Replace bucket/object-key model with folder/file model.
- Replace `bucket_name` and `target_root_prefix` with `drive_root_folder_id` or admin-selected Drive parent folder plus folder naming template.
- Replace object metadata/dedupe with Drive file identity tracking.
- Replace GCS checker with Drive authorization/folder access checker.
- Update security rules to keep Drive credentials/token server-side only.

### API/Docs Impact

Affected files likely include:

- `docs/api/cloud-storage-story-6-1.md` → replace or supersede with Google Drive settings API docs.
- `docs/api/openapi.yaml` if cloud endpoints are documented there.
- Current cloud endpoints can remain `/api/cloud/*` generically, but response fields must change away from bucket/object naming.

Recommended API naming:

```http
GET  /api/cloud/settings
PUT  /api/cloud/settings
POST /api/cloud/settings/check
POST /api/cloud/settings/folder-preview
POST /api/sessions/{session_id}/cloud-target/resolve
POST /api/sessions/{session_id}/uploads/start
GET  /api/sessions/{session_id}/uploads
```

Provider response should show:

```json
{
  "provider": "google_drive",
  "drive_root_folder_id": "...",
  "folder_naming_template": "...",
  "credentials_configured": true,
  "connection_status": "authorized"
}
```

### Code Impact

Likely affected areas:

- `apps/agent/internal/cloud/*`
- `apps/agent/internal/upload/*`
- `apps/agent/internal/api/cloud_settings.go`
- `apps/agent/internal/api/cloud_targets.go`
- readiness/checklist labels/actions
- frontend dashboard cloud/settings/status copy
- tests for settings/check/target/upload/recovery

Provider model change:

```text
GCS:
  bucket_name + target_root_prefix + object_key

Google Drive:
  root_folder_id + nested folder names + file IDs
```

### Data/State Impact

Existing runtime/state created for GCS should be treated as incompatible or migrated best-effort.

Recommended MVP approach:

- Add provider field versioning.
- If existing provider is `gcs`, show `DRIVE_CONFIG_REQUIRED` and require new Drive settings.
- Do not try to upload old GCS target records as Drive records automatically.

## 4. Detailed Change Proposals

### PRD Additional Requirement

**OLD:**

```text
Cloud storage: Google Cloud Storage stores post-session cloud image assets; local files must never be deleted due to cloud upload failure.
```

**NEW:**

```text
Cloud fulfillment: Google Drive stores post-session original and graded image assets in operator/customer-friendly folders; local files must never be deleted due to Drive upload failure.
```

**Rationale:** Aligns architecture with product requirement and operator delivery workflow.

---

### Architecture — Technical Constraints & Dependencies

**OLD:**

```text
Local storage adalah source of safety; Google Cloud Storage adalah post-session cloud storage/fulfillment layer.
Google Cloud Storage integration harus tahan token failure, network failure, partial upload, retry, duplicate prevention.
```

**NEW:**

```text
Local storage adalah source of safety; Google Drive adalah post-session delivery/fulfillment layer for operator/customer-accessible folders.
Google Drive integration must tolerate credential/token failure, network failure, partial upload, folder creation conflicts, file upload conflicts, retry, and duplicate prevention.
```

---

### Architecture — Infrastructure & Deployment

**OLD:**

```text
GCS upload worker runs independently from capture/processing flow.
```

**NEW:**

```text
Google Drive upload worker runs independently from capture/processing flow.
```

---

### Epic 6 Description

**OLD:**

```text
Admin bisa connect Google Cloud/Drive-equivalent storage flow sesuai architecture GCS, upload original+graded setelah local completion, track per-file/per-session upload, retry failure, prevent duplicate uploads, dan ensure upload tidak blokir session baru.
```

**NEW:**

```text
Admin bisa menghubungkan Google Drive sebagai post-session delivery target, membuat folder Drive per customer/order/session, upload original+graded setelah local completion, track per-file/per-session upload, retry failure, prevent duplicate Drive files, dan ensure upload tidak blokir session baru.
```

---

### Story 6.1

**OLD TITLE:** Configure Cloud Storage Credentials and Target Rules

**NEW TITLE:** Configure Google Drive Credentials and Folder Rules

**OLD AC snippets:**

```text
GCS object naming convention is documented and validated
cloud status appears separately from local save status
```

**NEW AC:**

```text
Given admin opens Drive settings
When admin configures Google Drive credentials and root folder/folder rules
Then service stores credentials/tokens server-side only
And browser never receives credentials, private keys, refresh tokens, or access tokens
And connection check reports authorized/failed status
And Drive root folder access is verified
And Drive folder naming convention is documented and validated
And Drive status appears separately from local save status.
```

---

### Story 6.2

**OLD TITLE:** Create Cloud Folder/Object Structure Per Session

**NEW TITLE:** Create Google Drive Folder Structure Per Session

**NEW AC:**

```text
Given session reaches local complete
When Drive fulfillment begins
Then system creates or resolves nested Drive folders based on event/customer/order/session rules
And sanitized folder names prevent invalid or confusing names
And Drive folder IDs are stored for session
And operation is idempotent across retry/restart.
```

---

### Story 6.3

**OLD:** upload to configured cloud target

**NEW:** upload to configured Google Drive session folder

Add AC:

```text
And uploaded Drive file IDs are stored per asset
And original and graded files are uploaded into clearly named Drive subfolders or with asset-kind metadata.
```

---

### Story 6.4

**OLD:** duplicate assets in cloud

**NEW:** duplicate files in Drive

Add AC:

```text
And retry checks tracked Drive file ID or deterministic app property before creating another file
And failure shows actionable Retry Drive Upload action.
```

---

### Story 6.5

**OLD:** reconciles remote identity/status enough to reduce duplicate uploads

**NEW:**

```text
And reconciles Drive folder IDs/file IDs/status enough to reduce duplicate uploads
```

## 5. Recommended Implementation Plan

### Phase 1 — Provider Contract Refactor

- Introduce generic upload provider interface if not already cleanly separated.
- Rename user-facing `gcs` provider to `google_drive`.
- Keep internal upload job/session target concepts reusable.

### Phase 2 — Drive Settings & Authorization

- Replace bucket settings with:
  - `provider: google_drive`
  - `drive_root_folder_id`
  - `folder_naming_template`
  - `credential_file_path` or OAuth/admin account config
- Implement connection check:
  - credentials valid
  - Drive API reachable
  - root folder exists and writable

### Phase 3 — Drive Folder Resolver

- Create/resolve nested folder path for each completed session.
- Store folder IDs, not only names.
- Idempotent folder creation by parent+name lookup.

### Phase 4 — Drive Upload Worker

- Upload original + graded files.
- Store Drive file IDs per upload job.
- Ensure retry does not duplicate files.
- Preserve local files on every remote failure.

### Phase 5 — UI/Docs/Readiness

- Replace GCS/bucket/object copy with Google Drive/folder/file copy.
- Update readiness checklist Drive status.
- Update OpenAPI/docs.
- Rerun event-scale smoke with Drive credentials.

## 6. Risks and Mitigations

### Risk: Google Drive auth complexity

Drive can use OAuth user consent or service account/domain delegation. For MVP local operator workflow, simplest options are:

1. Service account with access to a shared Drive folder, or
2. OAuth installed-app flow storing token server-side.

Recommendation: start with **service account + shared folder access** if acceptable, because it is easier for local automation and CI/smoke.

### Risk: Duplicate folder/file creation

Mitigation: store Drive folder IDs and file IDs; lookup by parent+name before create; optionally use Drive appProperties for session/photo/upload identity.

### Risk: Existing GCS work was completed

Mitigation: reuse upload queue, retry, state persistence, and recovery patterns; replace provider adapter and user-facing schema.

## 7. Handoff Plan

### Product/PM

- Approve this correction as restoring original PRD intent, not adding new scope.
- Decide auth mode: service-account shared folder vs OAuth admin account.

### Architect

- Update architecture references from GCS to Google Drive.
- Define Drive folder/file identity model and dedupe strategy.

### Developer

- Implement Drive provider replacement.
- Update API/docs/tests/readiness/UI copy.
- Run event-scale smoke with real Drive credentials/folder.

### QA/Review

- Verify no GCS wording remains in operator UI/docs except migration notes.
- Verify Drive upload failure does not block local pipeline.
- Verify retry/restart does not duplicate Drive files.

## 8. Approval

Approved by user on 2026-05-20.

```text
Approve replacing GCS with Google Drive upload provider for Epic 6, preserving local-first upload queue/retry/recovery behavior and updating architecture/docs/stories accordingly.
```
