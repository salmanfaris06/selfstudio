package upload

import (
	"context"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
)

const (
	EventUploadRecovered = "upload.recovered"
)

type RemoteObjectInfo struct {
	RemoteIdentity       string
	RemoteGeneration     int64
	RemoteMetageneration int64
	RemoteETag           string
}

type RemoteVerifier interface {
	Stat(ctx context.Context, bucketName, objectKey string) (RemoteObjectInfo, error)
}

type UploadedVerifier interface {
	VerifyUploaded(ctx context.Context, job FileUploadJob) (RemoteObjectInfo, error)
}

type RecoverySummary struct {
	RecoveredPending   int `json:"recovered_pending"`
	Resumed            int `json:"resumed"`
	FailedMissingLocal int `json:"failed_missing_local"`
	VerifiedUploaded   int `json:"verified_uploaded"`
	UnverifiedUploaded int `json:"unverified_uploaded"`
	RequiresCloudCheck int `json:"requires_cloud_check"`
	Errors             int `json:"errors"`
}

type RecoveryResult struct {
	Summary    RecoverySummary
	EnqueueIDs []string
}

type StartupRecovery struct {
	Worker           *Worker
	Verifier         RemoteVerifier
	UploadedVerifier UploadedVerifier
	Activity         *activity.Store
	Broker           *events.Broker
	Now              func() time.Time
}

func (r StartupRecovery) Recover(ctx context.Context) (RecoveryResult, error) {
	res := RecoveryResult{}
	if r.Worker == nil || r.Worker.Jobs == nil {
		return res, nil
	}
	r.Worker.ensureGuard()
	if r.Worker.RecoveryMu != nil {
		r.Worker.RecoveryMu.Lock()
		defer r.Worker.RecoveryMu.Unlock()
	}
	now := r.now()
	changed := false
	seenEnqueue := map[string]bool{}
	for _, j := range r.Worker.Jobs.List() {
		if j.Status == JobStatusNotEligible {
			continue
		}
		original := j
		switch j.Status {
		case JobStatusUploading, JobStatusRetrying:
			j.Status = JobStatusRetryScheduled
			j.MaxAttempts = MaxAutoUploadAttempts
			if j.LastErrorCode == "" {
				j.LastErrorCode = ErrorDriveUploadFailed
			}
			if j.LastErrorAction == "" || j.LastErrorAction == ActionRetryCloudUpload {
				j.LastErrorAction = ActionRetryDriveUpload
			}
			if j.AttemptCount < MaxAutoUploadAttempts && IsRetryableUploadError(j.LastErrorCode, j.LastErrorAction) {
				next := now
				j.NextRetryAt = &next
				j.RetryAfterSeconds = 0
			} else {
				j.Status = JobStatusFailed
				j.NextRetryAt = nil
				j.RetryAfterSeconds = 0
			}
			res.Summary.RecoveredPending++
		case JobStatusUploaded:
			j = repairDriveUploadedIdentity(j)
			if !hasRemoteMetadata(j) {
				j.Status = JobStatusFailed
				j.LastErrorCode = ErrorUploadRemoteCheckNeeded
				j.LastErrorAction = ActionCheckDriveFile
				j.NextRetryAt = nil
				j.RetryAfterSeconds = 0
				res.Summary.RequiresCloudCheck++
				break
			}
			if r.UploadedVerifier != nil {
				info, err := r.UploadedVerifier.VerifyUploaded(ctx, j)
				if err == nil && remoteMatches(j, info) {
					res.Summary.VerifiedUploaded++
				} else if j.DriveFileID != "" {
					res.Summary.UnverifiedUploaded++
				} else {
					j.Status = JobStatusFailed
					j.LastErrorCode = ErrorUploadRemoteCheckNeeded
					j.LastErrorAction = ActionCheckDriveFile
					j.NextRetryAt = nil
					j.RetryAfterSeconds = 0
					res.Summary.RequiresCloudCheck++
				}
			} else if r.Verifier != nil && j.DriveFileID == "" {
				info, err := r.Verifier.Stat(ctx, j.BucketName, j.ObjectKey)
				if err != nil || !remoteMatches(j, info) {
					j.Status = JobStatusFailed
					j.LastErrorCode = ErrorUploadObjectCheckNeeded
					j.LastErrorAction = ActionCheckCloudObject
					j.NextRetryAt = nil
					j.RetryAfterSeconds = 0
					res.Summary.RequiresCloudCheck++
				} else {
					res.Summary.VerifiedUploaded++
				}
			} else {
				res.Summary.UnverifiedUploaded++
			}
		}
		if original.Status != JobStatusUploaded && j.Status != JobStatusUploaded && j.Status != JobStatusNotEligible && !readable(j.LocalPath) {
			j.Status = JobStatusFailed
			j.LastErrorCode = ErrorUploadLocalFileMissing
			j.LastErrorAction = ActionCheckLocalOutput
			j.NextRetryAt = nil
			j.RetryAfterSeconds = 0
			res.Summary.FailedMissingLocal++
		}
		if j.Status != JobStatusUploaded && j.Status != JobStatusNotEligible && readable(j.LocalPath) && j.DriveFolderID == "" && j.BucketName == "" && j.ObjectKey == "" {
			j.Status = JobStatusFailed
			j.LastErrorCode = ErrorCloudTargetNotReady
			j.LastErrorAction = ActionResolveCloudTarget
			j.NextRetryAt = nil
			j.RetryAfterSeconds = 0
		}
		if shouldResumeAfterRecovery(j, now) && !seenEnqueue[j.JobID] {
			seenEnqueue[j.JobID] = true
			res.EnqueueIDs = append(res.EnqueueIDs, j.JobID)
			res.Summary.Resumed++
		}
		if j != original {
			j.UpdatedAt = now
			if err := r.Worker.Jobs.Upsert(j); err != nil {
				res.Summary.Errors++
				return res, err
			}
			changed = true
		}
	}
	if changed {
		if err := r.Worker.Persistence.Save(r.Worker.Jobs); err != nil {
			res.Summary.Errors++
			return res, err
		}
	}
	r.publishSuccess(res.Summary)
	for _, id := range res.EnqueueIDs {
		if _, err := r.Worker.ResumeRecoveredJob(ctx, id); err != nil {
			// ResumeRecoveredJob may no-op if another guarded path accepted it first; keep recovery non-blocking.
			continue
		}
	}
	return res, nil
}

func (r StartupRecovery) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

func hasRemoteMetadata(j FileUploadJob) bool {
	return j.DriveFileID != "" || j.RemoteIdentity != "" || j.RemoteGeneration > 0 || j.RemoteETag != ""
}

func repairDriveUploadedIdentity(j FileUploadJob) FileUploadJob {
	if j.Status != JobStatusUploaded {
		return j
	}
	if j.RemoteIdentity == "" && j.DriveFileID != "" {
		j.RemoteIdentity = j.DriveFileID
	}
	if j.DriveFileID == "" && j.RemoteIdentity != "" && j.RemoteGeneration == 0 {
		j.DriveFileID = j.RemoteIdentity
	}
	return j
}

func remoteMatches(j FileUploadJob, info RemoteObjectInfo) bool {
	if j.DriveFileID != "" {
		if info.RemoteIdentity == "" {
			return true
		}
		return info.RemoteIdentity == j.DriveFileID
	}
	if j.RemoteIdentity != "" && info.RemoteIdentity != "" && j.RemoteIdentity != info.RemoteIdentity {
		return false
	}
	if j.RemoteGeneration > 0 && info.RemoteGeneration > 0 && j.RemoteGeneration != info.RemoteGeneration {
		return false
	}
	if j.RemoteMetageneration > 0 && info.RemoteMetageneration > 0 && j.RemoteMetageneration != info.RemoteMetageneration {
		return false
	}
	if j.RemoteETag != "" && info.RemoteETag != "" && j.RemoteETag != info.RemoteETag {
		return false
	}
	return true
}

func shouldResumeAfterRecovery(j FileUploadJob, now time.Time) bool {
	if j.Status == JobStatusPending {
		return j.DriveFolderID != "" || (j.BucketName != "" && j.ObjectKey != "")
	}
	if j.Status == JobStatusRetryScheduled && CanAutoRetry(j) && (j.NextRetryAt == nil || !j.NextRetryAt.After(now)) {
		return true
	}
	return false
}

func (r StartupRecovery) publishSuccess(summary RecoverySummary) {
	if r.Activity != nil {
		r.Activity.Record("cloud.upload_recovery_completed", activity.ResultSuccess, "Google Drive upload recovery completed")
	}
	if r.Broker == nil {
		return
	}
	payload := map[string]any{
		"recovered_pending":    summary.RecoveredPending,
		"resumed":              summary.Resumed,
		"failed_missing_local": summary.FailedMissingLocal,
		"verified_uploaded":    summary.VerifiedUploaded,
		"unverified_uploaded":  summary.UnverifiedUploaded,
		"requires_cloud_check": summary.RequiresCloudCheck,
		"errors":               summary.Errors,
	}
	if ev, err := events.New(EventUploadRecovered, "upload", "startup", payload); err == nil {
		r.Broker.Publish(ev)
	}
}
