package upload

import (
	"testing"
	"time"
)

func TestRetryPolicyClassifiesRetryableAndBackoff(t *testing.T) {
	if !IsRetryableUploadError(ErrorUploadFailed, ActionRetryCloudUpload) {
		t.Fatal("generic upload failure should be retryable")
	}
	if IsRetryableUploadError(ErrorUploadLocalFileMissing, ActionCheckLocalOutput) {
		t.Fatal("missing local output should wait for operator fix")
	}
	if NextRetryDelay(0) != 5*time.Second || NextRetryDelay(1) != 30*time.Second || NextRetryDelay(9) != 2*time.Minute {
		t.Fatal("unexpected deterministic backoff")
	}
}

func TestRetryPolicyMaxAutomaticAndManualBeyondLimit(t *testing.T) {
	job := FileUploadJob{Status: JobStatusFailed, AttemptCount: MaxAutoUploadAttempts - 1, LastErrorCode: ErrorUploadFailed, LastErrorAction: ActionRetryCloudUpload}
	if !CanAutoRetry(job) {
		t.Fatal("job should auto retry before limit")
	}
	job.AttemptCount = MaxAutoUploadAttempts
	if CanAutoRetry(job) {
		t.Fatal("job should not auto retry after max attempts")
	}
	if CanManualRetry(job) {
		t.Fatal("manual retry should respect max attempt limit by default")
	}
}

func TestScheduleRetryPersistsNextActionMetadata(t *testing.T) {
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	job := ScheduleRetry(FileUploadJob{Status: JobStatusFailed, AttemptCount: 1, LastErrorCode: ErrorUploadFailed, LastErrorAction: ActionRetryCloudUpload}, now)
	if job.Status != JobStatusRetryScheduled || job.NextRetryAt == nil || job.RetryAfterSeconds != 30 || job.MaxAttempts != MaxAutoUploadAttempts {
		t.Fatalf("retry metadata not set: %+v", job)
	}
}

func TestRetryPolicyDriveSpecificErrorsAndMaxAttempts(t *testing.T) {
	job := FileUploadJob{Status: JobStatusFailed, AttemptCount: 1, LastErrorCode: ErrorDriveUploadFailed, LastErrorAction: ActionRetryDriveUpload}
	if !IsRetryableUploadError(ErrorDriveUploadFailed, ActionRetryDriveUpload) || !CanAutoRetry(job) || !CanManualRetry(job) {
		t.Fatal("Drive transient upload failure should be auto/manual retryable before max attempts")
	}
	for _, j := range []FileUploadJob{
		{Status: JobStatusFailed, AttemptCount: 1, LastErrorCode: ErrorDriveUploadUnauthorized, LastErrorAction: ActionFixCredentials},
		{Status: JobStatusFailed, AttemptCount: 1, LastErrorCode: ErrorDriveFolderUnavailable, LastErrorAction: ActionFixDriveFolder},
		{Status: JobStatusFailed, AttemptCount: 1, LastErrorCode: ErrorUploadLocalFileMissing, LastErrorAction: ActionCheckLocalOutput},
	} {
		if CanAutoRetry(j) || CanManualRetry(j) {
			t.Fatalf("non-retryable safe error should not retry: %+v", j)
		}
	}
	job.AttemptCount = MaxAutoUploadAttempts
	if CanAutoRetry(job) || CanManualRetry(job) {
		t.Fatal("exhausted Drive job should not be accepted for retry")
	}
}
