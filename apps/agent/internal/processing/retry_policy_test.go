package processing

import (
	"os"
	"path/filepath"
	"testing"

	"selfstudio/agent/internal/photos"
)

func retryPolicyPhoto(t *testing.T) photos.Photo {
	t.Helper()
	path := filepath.Join(t.TempDir(), "original.jpg")
	if err := os.WriteFile(path, []byte("jpg"), 0o644); err != nil {
		t.Fatal(err)
	}
	return photos.Photo{PhotoID: "photo_1", SourceSizeBytes: 3, OriginalSaveStatus: photos.OriginalStatusSaved, LocalOriginalPath: path, ProcessingStatus: photos.ProcessingStatusEligible, GradedProcessingStatus: photos.GradedStatusFailed}
}

func TestRetryEligibilityAutomaticStopsAtMaxAttempts(t *testing.T) {
	photo := retryPolicyPhoto(t)
	photo.GradedAttemptCount = MaxGradedAttempts
	if err := RetryEligibility(photo, RetryModeAutomatic); err == nil {
		t.Fatal("expected automatic retry limit rejection")
	} else if rejection, ok := err.(RetryRejection); !ok || rejection.Code != "AUTOMATIC_RETRY_LIMIT_REACHED" {
		t.Fatalf("unexpected rejection: %#v", err)
	}
}

func TestRetryEligibilityManualAllowedAfterMaxAttempts(t *testing.T) {
	photo := retryPolicyPhoto(t)
	photo.GradedAttemptCount = MaxGradedAttempts
	if err := RetryEligibility(photo, RetryModeManual); err != nil {
		t.Fatalf("manual retry after max attempts should be allowed: %v", err)
	}
}

func TestRetryEligibilityRejectsMissingOriginal(t *testing.T) {
	photo := retryPolicyPhoto(t)
	photo.LocalOriginalPath = filepath.Join(t.TempDir(), "missing.jpg")
	if err := RetryEligibility(photo, RetryModeManual); err == nil {
		t.Fatal("expected missing original rejection")
	} else if rejection, ok := err.(RetryRejection); !ok || rejection.Code != "ORIGINAL_INVALID" {
		t.Fatalf("unexpected rejection: %#v", err)
	}
}

func TestRetryEligibilityRejectsProcessingAndProcessed(t *testing.T) {
	for _, status := range []string{photos.GradedStatusProcessing, photos.GradedStatusProcessed} {
		photo := retryPolicyPhoto(t)
		photo.GradedProcessingStatus = status
		if err := RetryEligibility(photo, RetryModeManual); err == nil {
			t.Fatalf("expected rejection for %s", status)
		}
	}
}

func TestProcessingGuardRejectsConcurrentDuplicate(t *testing.T) {
	guard := NewProcessingGuard()
	if !guard.TryStart("photo_1") {
		t.Fatal("first start should succeed")
	}
	if guard.TryStart("photo_1") {
		t.Fatal("duplicate start should be rejected")
	}
	guard.Done("photo_1")
	if !guard.TryStart("photo_1") {
		t.Fatal("start after done should succeed")
	}
}
