package upload

import "time"

const MaxAutoUploadAttempts = 3

const (
	JobStatusRetrying       = "retrying"
	JobStatusRetryScheduled = "retry_scheduled"

	ErrorUploadJobNotFound       = "UPLOAD_JOB_NOT_FOUND"
	ErrorUploadNotRetryable      = "UPLOAD_NOT_RETRYABLE"
	ErrorUploadAlreadyRunning    = "UPLOAD_ALREADY_RUNNING"
	ErrorUploadAlreadyUploaded   = "UPLOAD_ALREADY_UPLOADED"
	ErrorUploadRetryStateSave    = "UPLOAD_RETRY_STATE_SAVE_FAILED"
	ErrorUploadConflict          = "UPLOAD_CONFLICT"
	ErrorUploadObjectCheckNeeded = "UPLOAD_OBJECT_CHECK_NEEDED"

	ActionCheckCloudObject = "CHECK_CLOUD_OBJECT"
)

var uploadBackoff = []time.Duration{5 * time.Second, 30 * time.Second, 2 * time.Minute}

func NextRetryDelay(attemptCount int) time.Duration {
	if attemptCount < 0 {
		attemptCount = 0
	}
	if attemptCount >= len(uploadBackoff) {
		return uploadBackoff[len(uploadBackoff)-1]
	}
	return uploadBackoff[attemptCount]
}

func IsRetryableUploadError(code, action string) bool {
	switch code {
	case ErrorUploadFailed, ErrorDriveUploadFailed:
		return true
	case ErrorUploadConflict, ErrorUploadObjectCheckNeeded, ErrorUploadRemoteCheckNeeded:
		return false
	case ErrorUploadLocalFileMissing, ErrorCloudTargetNotReady, ErrorUploadPendingLocalCompletion, ErrorDriveUploadUnauthorized, ErrorDriveFolderUnavailable:
		return false
	}
	return action == ActionRetryCloudUpload || action == ActionRetryDriveUpload || action == ActionCheckCloudObject
}

func CanAutoRetry(j FileUploadJob) bool {
	return (j.Status == JobStatusFailed || j.Status == JobStatusRetryScheduled) && j.AttemptCount < MaxAutoUploadAttempts && IsRetryableUploadError(j.LastErrorCode, j.LastErrorAction)
}

func CanManualRetry(j FileUploadJob) bool {
	return (j.Status == JobStatusFailed || j.Status == JobStatusRetryScheduled || j.Status == JobStatusPending) && j.AttemptCount < MaxAutoUploadAttempts && IsRetryableUploadError(j.LastErrorCode, j.LastErrorAction)
}

func ScheduleRetry(j FileUploadJob, now time.Time) FileUploadJob {
	delay := NextRetryDelay(j.AttemptCount)
	next := now.Add(delay).UTC()
	j.Status = JobStatusRetryScheduled
	j.NextRetryAt = &next
	j.RetryAfterSeconds = int(delay.Seconds())
	j.MaxAttempts = MaxAutoUploadAttempts
	j.UpdatedAt = now.UTC()
	return j
}
