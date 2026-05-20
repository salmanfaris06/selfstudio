package processing

import (
	"fmt"
	"time"

	"selfstudio/agent/internal/activity"
	"selfstudio/agent/internal/events"
	"selfstudio/agent/internal/photos"
)

// RecoveryOutcome documents the startup reconciliation contract for Epic 5 only.
// Outcomes are metadata/filesystem verification results; graded processing itself
// is scheduled asynchronously by the caller so dashboard/API startup is not blocked.
type RecoveryOutcome string

const (
	RecoveryVerified          RecoveryOutcome = "verified_success"
	RecoveryResumed           RecoveryOutcome = "resumed_processing"
	RecoveryFailed            RecoveryOutcome = "failed_actionable"
	RecoverySkippedRetryLimit RecoveryOutcome = "skipped_retry_limit"
)

type StartupRecoverySummary struct {
	Verified          int `json:"verified"`
	Resumed           int `json:"resumed"`
	Failed            int `json:"failed"`
	SkippedRetryLimit int `json:"skipped_retry_limit"`
	Errors            int `json:"errors"`
}

type StartupRecoveryResult struct {
	Summary    StartupRecoverySummary
	EnqueueIDs []string
}

type StartupRecovery struct {
	OriginalSaver *OriginalSaver
	Processor     *GradedProcessor
	Activity      *activity.Store
	Broker        *events.Broker
	Now           func() time.Time
}

func (r StartupRecovery) Recover() StartupRecoveryResult {
	result := StartupRecoveryResult{EnqueueIDs: []string{}}
	if r.OriginalSaver != nil {
		for _, saveResult := range r.OriginalSaver.ReconcilePending() {
			if saveResult.Err != nil {
				result.Summary.Failed++
				result.Summary.Errors++
			} else if saveResult.Photo.OriginalSaveStatus == photos.OriginalStatusSaved {
				result.Summary.Verified++
			}
		}
	}
	if r.Processor == nil || r.Processor.Photos == nil {
		r.publish(result.Summary)
		return result
	}
	for _, photo := range r.Processor.Photos.ListAll() {
		switch photo.GradedProcessingStatus {
		case photos.GradedStatusProcessed:
			if err := validGraded(photo); err != nil {
				r.Processor.Photos.MarkGradedFailed(photo.PhotoID, actionableGradedReason(err), r.now())
				if persistErr := r.Processor.persist(); persistErr != nil {
					result.Summary.Errors++
				}
				result.Summary.Failed++
				result.Summary.Errors++
			} else {
				result.Summary.Verified++
			}
		case "", photos.GradedStatusPending, photos.GradedStatusProcessing:
			if canResumeGraded(photo) {
				result.EnqueueIDs = append(result.EnqueueIDs, photo.PhotoID)
				result.Summary.Resumed++
			}
		case photos.GradedStatusFailed:
			if canResumeGraded(photo) {
				if IsAutomaticRetryEligible(photo) {
					result.EnqueueIDs = append(result.EnqueueIDs, photo.PhotoID)
					result.Summary.Resumed++
				} else if photo.GradedAttemptCount >= MaxGradedAttempts {
					result.Summary.SkippedRetryLimit++
				}
			}
		}
	}
	r.publish(result.Summary)
	return result
}

func canResumeGraded(photo photos.Photo) bool {
	return photo.OriginalSaveStatus == photos.OriginalStatusSaved && photo.ProcessingStatus == photos.ProcessingStatusEligible && validOriginal(photo) == nil
}

func actionableGradedReason(err error) string {
	if err == nil {
		return "graded output verification failed after restart"
	}
	return "graded output missing or invalid after restart: " + err.Error() + "; regenerate via manual retry after confirming original and LUT are available"
}

func (r StartupRecovery) publish(summary StartupRecoverySummary) {
	if summary == (StartupRecoverySummary{}) {
		return
	}
	payload := map[string]any{"verified": summary.Verified, "resumed": summary.Resumed, "failed": summary.Failed, "skipped_retry_limit": summary.SkippedRetryLimit, "errors": summary.Errors}
	if r.Broker != nil {
		if event, err := events.New("processing.recovered", "startup", "startup", payload); err == nil {
			r.Broker.Publish(event)
		}
		if event, err := events.New("queue.updated", "startup", "startup", payload); err == nil {
			r.Broker.Publish(event)
		}
	}
	if r.Activity != nil {
		message := fmt.Sprintf("Startup processing recovery: verified=%d resumed=%d failed=%d skipped_retry_limit=%d errors=%d", summary.Verified, summary.Resumed, summary.Failed, summary.SkippedRetryLimit, summary.Errors)
		result := activity.ResultSuccess
		if summary.Errors > 0 || summary.Failed > 0 {
			result = activity.ResultFailure
		}
		r.Activity.Record("processing.recovered", result, message)
	}
}

func (r StartupRecovery) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	if r.Processor != nil {
		return r.Processor.now()
	}
	return time.Now().UTC()
}
